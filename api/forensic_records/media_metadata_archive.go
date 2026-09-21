package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	maxArchiveMembers          = 10_000
	maxArchiveCentralBytes     = 64 << 20
	maxArchiveMemberBytes      = uint64(2 << 30)
	maxArchiveExpandedBytes    = uint64(10 << 30)
	maxArchiveCompressionRatio = 100.0
	maxArchiveExtensionKinds   = 64
	maxArchiveLinkTargetBytes  = 4096
	maxZIPTailBytes            = 65_557
)

var nestedArchiveExtensions = map[string]struct{}{
	".7z": {}, ".bz2": {}, ".gz": {}, ".rar": {}, ".tar": {}, ".tgz": {}, ".xz": {}, ".zip": {},
}

type archiveInventory struct {
	memberCount             int
	fileCount               int
	directoryCount          int
	symlinkCount            int
	hardlinkCount           int
	specialFileCount        int
	unsafePathCount         int
	unsafeLinkTargetCount   int
	duplicateNameCount      int
	encryptedMemberCount    int
	nonUTF8NameCount        int
	unsupportedMethodCount  int
	nestedArchiveCount      int
	zeroCompressedSizeCount int
	totalCompressedBytes    uint64
	totalUncompressedBytes  uint64
	maximumMemberBytes      uint64
	maximumCompressionRatio float64
	extensionCounts         map[string]int
	seenNames               map[string]struct{}
	inventoryComplete       bool
	extendedMetadataHeaders int
}

func newArchiveInventory() *archiveInventory {
	return &archiveInventory{
		extensionCounts:   map[string]int{},
		seenNames:         map[string]struct{}{},
		inventoryComplete: true,
	}
}

func extractArchiveMetadata(path, extension string) (map[string]any, []string, error) {
	switch extension {
	case ".zip":
		return extractZIPMetadata(path)
	case ".tar":
		return extractTARMetadata(path)
	default:
		return nil, nil, fmt.Errorf("unsupported deterministic archive extension %q", extension)
	}
}

func extractZIPMetadata(path string) (map[string]any, []string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, nil, fmt.Errorf("open ZIP: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("stat ZIP: %w", err)
	}

	entryCount, centralOffset, centralSize, zip64, trailingBytes, err := readZIPDirectoryBounds(file, info.Size())
	if err != nil {
		return nil, nil, err
	}
	if entryCount > maxArchiveMembers {
		return nil, nil, fmt.Errorf("ZIP declares %d members; inventory limit is %d", entryCount, maxArchiveMembers)
	}
	if centralSize > maxArchiveCentralBytes {
		return nil, nil, fmt.Errorf("ZIP central directory is %d bytes; inventory limit is %d", centralSize, maxArchiveCentralBytes)
	}
	if centralOffset > uint64(info.Size()) || centralSize > uint64(info.Size())-centralOffset {
		return nil, nil, fmt.Errorf("ZIP central directory exceeds available file bytes")
	}

	reader, err := zip.NewReader(file, info.Size())
	if err != nil {
		return nil, nil, fmt.Errorf("read ZIP central directory: %w", err)
	}
	if len(reader.File) != entryCount {
		return nil, nil, fmt.Errorf("ZIP member count changed between preflight (%d) and parser (%d)", entryCount, len(reader.File))
	}

	inventory := newArchiveInventory()
	for _, member := range reader.File {
		inventory.memberCount++
		inventory.observeName(member.Name, member.NonUTF8)
		mode := member.Mode()
		switch {
		case member.FileInfo().IsDir():
			inventory.directoryCount++
		case mode&os.ModeSymlink != 0:
			inventory.symlinkCount++
			if !zipLinkTargetSafe(member) {
				inventory.unsafeLinkTargetCount++
			}
		case !mode.IsRegular():
			inventory.specialFileCount++
		default:
			inventory.fileCount++
		}
		if member.Flags&0x1 != 0 {
			inventory.encryptedMemberCount++
		}
		if member.Method != zip.Store && member.Method != zip.Deflate {
			inventory.unsupportedMethodCount++
		}
		inventory.observeSizes(member.CompressedSize64, member.UncompressedSize64)
	}

	metadata, warnings := inventory.metadata("zip")
	metadata["zip64"] = zip64
	metadata["central_directory_bytes"] = centralSize
	metadata["trailing_bytes"] = trailingBytes
	metadata["content_validation"] = "central_directory_only"
	if trailingBytes > 0 {
		warnings = appendUniqueWarnings(warnings, "ZIP has bytes after the end-of-central-directory record")
		metadata["extraction_safe"] = false
	}
	return metadata, warnings, nil
}

func zipLinkTargetSafe(member *zip.File) bool {
	if member.Flags&0x1 != 0 || (member.Method != zip.Store && member.Method != zip.Deflate) || member.UncompressedSize64 > maxArchiveLinkTargetBytes {
		return false
	}
	reader, err := member.Open()
	if err != nil {
		return false
	}
	payload, readErr := io.ReadAll(io.LimitReader(reader, maxArchiveLinkTargetBytes+1))
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil || len(payload) > maxArchiveLinkTargetBytes {
		return false
	}
	return isArchivePathSafe(string(payload))
}

func readZIPDirectoryBounds(reader io.ReaderAt, size int64) (int, uint64, uint64, bool, int64, error) {
	if size < 22 {
		return 0, 0, 0, false, 0, fmt.Errorf("file is too short for a ZIP end record")
	}
	tailSize := size
	if tailSize > maxZIPTailBytes {
		tailSize = maxZIPTailBytes
	}
	tail := make([]byte, tailSize)
	if _, err := reader.ReadAt(tail, size-tailSize); err != nil && err != io.EOF {
		return 0, 0, 0, false, 0, fmt.Errorf("read ZIP tail: %w", err)
	}
	signature := []byte{'P', 'K', 0x05, 0x06}
	relativeOffset := bytes.LastIndex(tail, signature)
	if relativeOffset < 0 || relativeOffset+22 > len(tail) {
		return 0, 0, 0, false, 0, fmt.Errorf("ZIP end-of-central-directory record was not found")
	}
	eocd := tail[relativeOffset:]
	commentLength := int(binary.LittleEndian.Uint16(eocd[20:22]))
	if relativeOffset+22+commentLength > len(tail) {
		return 0, 0, 0, false, 0, fmt.Errorf("ZIP end record comment exceeds available bytes")
	}
	absEOCDOffset := size - tailSize + int64(relativeOffset)
	trailingBytes := size - (absEOCDOffset + 22 + int64(commentLength))
	if binary.LittleEndian.Uint16(eocd[4:6]) != 0 || binary.LittleEndian.Uint16(eocd[6:8]) != 0 {
		return 0, 0, 0, false, 0, fmt.Errorf("multi-disk ZIP archives are not supported")
	}
	entriesOnDisk := binary.LittleEndian.Uint16(eocd[8:10])
	totalEntries := binary.LittleEndian.Uint16(eocd[10:12])
	if entriesOnDisk != totalEntries && totalEntries != 0xffff {
		return 0, 0, 0, false, 0, fmt.Errorf("ZIP entry counts are inconsistent")
	}
	centralSize32 := binary.LittleEndian.Uint32(eocd[12:16])
	centralOffset32 := binary.LittleEndian.Uint32(eocd[16:20])
	zip64 := totalEntries == 0xffff || centralSize32 == 0xffffffff || centralOffset32 == 0xffffffff
	if !zip64 {
		return int(totalEntries), uint64(centralOffset32), uint64(centralSize32), false, trailingBytes, nil
	}
	if absEOCDOffset < 20 {
		return 0, 0, 0, true, 0, fmt.Errorf("ZIP64 locator is missing")
	}
	var locator [20]byte
	if _, err := reader.ReadAt(locator[:], absEOCDOffset-20); err != nil {
		return 0, 0, 0, true, 0, fmt.Errorf("read ZIP64 locator: %w", err)
	}
	if !bytes.Equal(locator[:4], []byte{'P', 'K', 0x06, 0x07}) || binary.LittleEndian.Uint32(locator[4:8]) != 0 || binary.LittleEndian.Uint32(locator[16:20]) != 1 {
		return 0, 0, 0, true, 0, fmt.Errorf("invalid or multi-disk ZIP64 locator")
	}
	zip64Offset := binary.LittleEndian.Uint64(locator[8:16])
	if zip64Offset > uint64(size)-56 {
		return 0, 0, 0, true, 0, fmt.Errorf("ZIP64 end record offset exceeds available file bytes")
	}
	var record [56]byte
	if _, err := reader.ReadAt(record[:], int64(zip64Offset)); err != nil {
		return 0, 0, 0, true, 0, fmt.Errorf("read ZIP64 end record: %w", err)
	}
	if !bytes.Equal(record[:4], []byte{'P', 'K', 0x06, 0x06}) || binary.LittleEndian.Uint64(record[4:12]) < 44 {
		return 0, 0, 0, true, 0, fmt.Errorf("invalid ZIP64 end record")
	}
	if binary.LittleEndian.Uint32(record[16:20]) != 0 || binary.LittleEndian.Uint32(record[20:24]) != 0 {
		return 0, 0, 0, true, 0, fmt.Errorf("multi-disk ZIP64 archives are not supported")
	}
	entriesOnDisk64 := binary.LittleEndian.Uint64(record[24:32])
	totalEntries64 := binary.LittleEndian.Uint64(record[32:40])
	if entriesOnDisk64 != totalEntries64 || totalEntries64 > uint64(maxArchiveMembers) {
		return 0, 0, 0, true, 0, fmt.Errorf("ZIP64 declares %d members; inventory limit is %d", totalEntries64, maxArchiveMembers)
	}
	return int(totalEntries64), binary.LittleEndian.Uint64(record[48:56]), binary.LittleEndian.Uint64(record[40:48]), true, trailingBytes, nil
}

func extractTARMetadata(path string) (map[string]any, []string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, nil, fmt.Errorf("open TAR: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("stat TAR: %w", err)
	}
	if info.Size() < 1024 {
		return nil, nil, fmt.Errorf("file is too short for a complete TAR archive")
	}

	inventory := newArchiveInventory()
	var warnings []string
	position := int64(0)
	zeroBlocks := 0
	terminated := false
	for position+512 <= info.Size() {
		var header [512]byte
		if _, err := file.ReadAt(header[:], position); err != nil {
			return nil, nil, fmt.Errorf("read TAR header at %d: %w", position, err)
		}
		position += 512
		if isZeroBlock(header[:]) {
			zeroBlocks++
			if zeroBlocks == 2 {
				terminated = true
				break
			}
			continue
		}
		if zeroBlocks != 0 {
			return nil, nil, fmt.Errorf("non-zero TAR header follows a zero terminator block")
		}
		if err := validateTARChecksum(header[:]); err != nil {
			return nil, nil, fmt.Errorf("TAR header at %d: %w", position-512, err)
		}
		size, err := parseTARNumber(header[124:136])
		if err != nil {
			return nil, nil, fmt.Errorf("TAR member size at %d: %w", position-512, err)
		}
		paddedSize := (size + 511) &^ 511
		if paddedSize < size || uint64(position) > uint64(info.Size()) || paddedSize > uint64(info.Size()-position) {
			return nil, nil, fmt.Errorf("TAR member data exceeds available file bytes")
		}
		typeFlag := header[156]
		if typeFlag == 'x' || typeFlag == 'g' || typeFlag == 'L' || typeFlag == 'K' {
			inventory.extendedMetadataHeaders++
			inventory.inventoryComplete = false
			warnings = appendUniqueWarnings(warnings, "TAR uses PAX/GNU extended names or metadata; bounded inventory does not retain their payloads")
			position += int64(paddedSize)
			continue
		}
		if inventory.memberCount >= maxArchiveMembers {
			inventory.inventoryComplete = false
			warnings = appendUniqueWarnings(warnings, fmt.Sprintf("TAR member inventory stopped at the %d-member limit", maxArchiveMembers))
			break
		}

		name := tarHeaderPath(header[:])
		linkTarget := tarString(header[157:257])
		inventory.memberCount++
		inventory.observeName(name, !utf8.ValidString(name))
		switch typeFlag {
		case 0, '0':
			inventory.fileCount++
			inventory.observeSizes(size, size)
		case '5':
			inventory.directoryCount++
		case '1':
			inventory.hardlinkCount++
			if !isArchivePathSafe(linkTarget) {
				inventory.unsafeLinkTargetCount++
			}
		case '2':
			inventory.symlinkCount++
			if !isArchivePathSafe(linkTarget) {
				inventory.unsafeLinkTargetCount++
			}
		default:
			inventory.specialFileCount++
		}
		position += int64(paddedSize)
	}
	if !terminated {
		inventory.inventoryComplete = false
		warnings = appendUniqueWarnings(warnings, "TAR did not end with two consecutive zero blocks inside the file")
	}
	if info.Size()%512 != 0 {
		inventory.inventoryComplete = false
		warnings = appendUniqueWarnings(warnings, "TAR file size is not aligned to a 512-byte record")
	}

	metadata, inventoryWarnings := inventory.metadata("tar")
	warnings = appendUniqueWarnings(warnings, inventoryWarnings...)
	metadata["terminal_zero_blocks"] = zeroBlocks
	metadata["extended_metadata_headers"] = inventory.extendedMetadataHeaders
	metadata["content_validation"] = "header_and_checksum_only"
	if !inventory.inventoryComplete {
		metadata["extraction_safe"] = false
	}
	return metadata, warnings, nil
}

func (inventory *archiveInventory) observeName(name string, nonUTF8 bool) {
	if nonUTF8 || !utf8.ValidString(name) {
		inventory.nonUTF8NameCount++
	}
	if !isArchivePathSafe(name) {
		inventory.unsafePathCount++
	}
	if _, exists := inventory.seenNames[name]; exists {
		inventory.duplicateNameCount++
	} else {
		inventory.seenNames[name] = struct{}{}
	}
	extension := archiveExtensionCategory(name)
	if _, nested := nestedArchiveExtensions[extension]; nested {
		inventory.nestedArchiveCount++
	}
	if len(inventory.extensionCounts) >= maxArchiveExtensionKinds {
		if _, exists := inventory.extensionCounts[extension]; !exists {
			extension = "<other>"
		}
	}
	inventory.extensionCounts[extension]++
}

func (inventory *archiveInventory) observeSizes(compressed, uncompressed uint64) {
	inventory.totalCompressedBytes = saturatingAdd(inventory.totalCompressedBytes, compressed)
	inventory.totalUncompressedBytes = saturatingAdd(inventory.totalUncompressedBytes, uncompressed)
	if uncompressed > inventory.maximumMemberBytes {
		inventory.maximumMemberBytes = uncompressed
	}
	if compressed == 0 {
		if uncompressed > 0 {
			inventory.zeroCompressedSizeCount++
			inventory.maximumCompressionRatio = maxArchiveCompressionRatio + 1
		}
		return
	}
	ratio := float64(uncompressed) / float64(compressed)
	if ratio > inventory.maximumCompressionRatio {
		inventory.maximumCompressionRatio = ratio
	}
}

func (inventory *archiveInventory) metadata(format string) (map[string]any, []string) {
	extractionSafe := inventory.inventoryComplete && inventory.unsafePathCount == 0 && inventory.unsafeLinkTargetCount == 0 &&
		inventory.symlinkCount == 0 && inventory.hardlinkCount == 0 && inventory.specialFileCount == 0 &&
		inventory.encryptedMemberCount == 0 && inventory.unsupportedMethodCount == 0 && inventory.nestedArchiveCount == 0 &&
		inventory.duplicateNameCount == 0 && inventory.nonUTF8NameCount == 0 && inventory.zeroCompressedSizeCount == 0 &&
		inventory.totalUncompressedBytes <= maxArchiveExpandedBytes && inventory.maximumMemberBytes <= maxArchiveMemberBytes &&
		inventory.maximumCompressionRatio <= maxArchiveCompressionRatio
	metadata := mediaMetadataBase(format)
	metadata["member_count"] = inventory.memberCount
	metadata["file_count"] = inventory.fileCount
	metadata["directory_count"] = inventory.directoryCount
	metadata["symlink_count"] = inventory.symlinkCount
	metadata["hardlink_count"] = inventory.hardlinkCount
	metadata["special_file_count"] = inventory.specialFileCount
	metadata["unsafe_path_count"] = inventory.unsafePathCount
	metadata["unsafe_link_target_count"] = inventory.unsafeLinkTargetCount
	metadata["duplicate_name_count"] = inventory.duplicateNameCount
	metadata["encrypted_member_count"] = inventory.encryptedMemberCount
	metadata["non_utf8_name_count"] = inventory.nonUTF8NameCount
	metadata["unsupported_compression_method_count"] = inventory.unsupportedMethodCount
	metadata["nested_archive_count"] = inventory.nestedArchiveCount
	metadata["zero_compressed_size_member_count"] = inventory.zeroCompressedSizeCount
	metadata["total_compressed_bytes"] = inventory.totalCompressedBytes
	metadata["total_uncompressed_bytes"] = inventory.totalUncompressedBytes
	metadata["maximum_member_uncompressed_bytes"] = inventory.maximumMemberBytes
	metadata["maximum_compression_ratio"] = inventory.maximumCompressionRatio
	metadata["extension_counts"] = inventory.extensionCounts
	metadata["inventory_complete"] = inventory.inventoryComplete
	metadata["extraction_safe"] = extractionSafe
	metadata["member_names_retained"] = false

	var warnings []string
	if inventory.unsafePathCount > 0 || inventory.unsafeLinkTargetCount > 0 {
		warnings = append(warnings, "archive contains traversal, absolute, drive-qualified, backslash, NUL, or unsafe link paths")
	}
	if inventory.symlinkCount > 0 || inventory.hardlinkCount > 0 || inventory.specialFileCount > 0 {
		warnings = append(warnings, "archive contains links or special files and is blocked from automatic extraction")
	}
	if inventory.encryptedMemberCount > 0 {
		warnings = append(warnings, "archive contains encrypted members and cannot be fully validated without separate authorization")
	}
	if inventory.unsupportedMethodCount > 0 {
		warnings = append(warnings, "archive uses compression methods outside the bounded Store/Deflate contract")
	}
	if inventory.nestedArchiveCount > 0 {
		warnings = append(warnings, "archive contains nested archive members; recursive extraction remains disabled")
	}
	if inventory.totalUncompressedBytes > maxArchiveExpandedBytes || inventory.maximumMemberBytes > maxArchiveMemberBytes {
		warnings = append(warnings, "archive expansion exceeds the configured total or per-member review limit")
	}
	if inventory.maximumCompressionRatio > maxArchiveCompressionRatio {
		warnings = append(warnings, "archive compression ratio exceeds the configured review threshold")
	}
	if inventory.duplicateNameCount > 0 {
		warnings = append(warnings, "archive contains duplicate member names")
	}
	if inventory.nonUTF8NameCount > 0 {
		warnings = append(warnings, "archive contains non-UTF-8 member names")
	}
	return metadata, warnings
}

func isArchivePathSafe(name string) bool {
	if name == "" || !utf8.ValidString(name) || strings.ContainsRune(name, 0) || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") {
		return false
	}
	if len(name) >= 2 && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z')) && name[1] == ':' {
		return false
	}
	cleaned := pathpkg.Clean(name)
	return cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

func archiveExtensionCategory(name string) string {
	extension := strings.ToLower(pathpkg.Ext(strings.TrimSuffix(name, "/")))
	if extension == "" {
		return "<none>"
	}
	if len(extension) > 16 {
		return "<other>"
	}
	for _, value := range extension[1:] {
		if (value < 'a' || value > 'z') && (value < '0' || value > '9') {
			return "<other>"
		}
	}
	return extension
}

func saturatingAdd(left, right uint64) uint64 {
	if ^uint64(0)-left < right {
		return ^uint64(0)
	}
	return left + right
}

func isZeroBlock(block []byte) bool {
	for _, value := range block {
		if value != 0 {
			return false
		}
	}
	return true
}

func validateTARChecksum(header []byte) error {
	declared, err := parseTARNumber(header[148:156])
	if err != nil {
		return fmt.Errorf("invalid checksum field: %w", err)
	}
	var sum uint64
	for index, value := range header {
		if index >= 148 && index < 156 {
			sum += uint64(' ')
		} else {
			sum += uint64(value)
		}
	}
	if sum != declared {
		return fmt.Errorf("checksum mismatch: declared %d, calculated %d", declared, sum)
	}
	return nil
}

func parseTARNumber(field []byte) (uint64, error) {
	if len(field) == 0 {
		return 0, fmt.Errorf("empty numeric field")
	}
	if field[0]&0x80 != 0 {
		if field[0]&0x40 != 0 {
			return 0, fmt.Errorf("negative base-256 value")
		}
		value := uint64(field[0] & 0x3f)
		for _, current := range field[1:] {
			if value > (^uint64(0)-uint64(current))/256 {
				return 0, fmt.Errorf("base-256 value overflows uint64")
			}
			value = value*256 + uint64(current)
		}
		return value, nil
	}
	trimmed := strings.Trim(string(field), " \x00")
	if trimmed == "" {
		return 0, nil
	}
	var value uint64
	for _, current := range trimmed {
		if current < '0' || current > '7' {
			return 0, fmt.Errorf("non-octal digit %q", current)
		}
		if value > (^uint64(0)-uint64(current-'0'))/8 {
			return 0, fmt.Errorf("octal value overflows uint64")
		}
		value = value*8 + uint64(current-'0')
	}
	return value, nil
}

func tarHeaderPath(header []byte) string {
	name := tarString(header[0:100])
	prefix := tarString(header[345:500])
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}

func tarString(field []byte) string {
	if index := bytes.IndexByte(field, 0); index >= 0 {
		field = field[:index]
	}
	return strings.TrimRight(string(field), " ")
}
