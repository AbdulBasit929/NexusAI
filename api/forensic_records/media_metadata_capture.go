package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const (
	maxCapturePackets           = 1_000_000
	maxCaptureBlocks            = 1_250_000
	maxCaptureSections          = 1024
	maxCaptureInterfaces        = 10_000
	maxCaptureInterfacesSection = 4096
	maxCaptureHeaderBytes       = 64 << 20
	maxCapturedPacketBytes      = 16 << 20
	maxCaptureBlockBytes        = 32 << 20
	maxCaptureOptionsBytes      = 64 << 10
)

const (
	pcapngSectionHeaderBlock       = uint32(0x0a0d0d0a)
	pcapngInterfaceDescription     = uint32(0x00000001)
	pcapngObsoletePacketBlock      = uint32(0x00000002)
	pcapngSimplePacketBlock        = uint32(0x00000003)
	pcapngNameResolutionBlock      = uint32(0x00000004)
	pcapngInterfaceStatisticsBlock = uint32(0x00000005)
	pcapngEnhancedPacketBlock      = uint32(0x00000006)
	pcapngDecryptionSecretsBlock   = uint32(0x0000000a)
	pcapngCustomBlockCopyable      = uint32(0x00000bad)
	pcapngCustomBlockNonCopyable   = uint32(0x40000bad)
)

type captureReader struct {
	file      *os.File
	fileSize  int64
	bytesRead int64
}

type pcapngInterface struct {
	sectionIndex            int
	interfaceID             uint32
	linkType                uint16
	snapLength              uint32
	timestampBase2          bool
	timestampExponent       uint8
	timestampOffsetSeconds  int64
	timestampConfigComplete bool
	packetCount             uint64
	optionCount             int
	nameOptionPresent       bool
	addressOptionCount      int
	filterOptionPresent     bool
}

type pcapngSection struct {
	index       int
	order       binary.ByteOrder
	endianName  string
	major       uint16
	minor       uint16
	expectedEnd int64
	interfaces  []*pcapngInterface
}

type capturePacketTotals struct {
	packetCount                 uint64
	totalCapturedBytes          uint64
	totalOriginalBytes          uint64
	maximumCapturedBytes        uint32
	maximumOriginalBytes        uint32
	truncatedPacketCount        uint64
	originalSmallerCount        uint64
	timestampOrderRegression    uint64
	timestampUnavailableCount   uint64
	firstTimestamp              time.Time
	lastTimestamp               time.Time
	previousTimestamp           time.Time
	firstTimestampSet           bool
	previousTimestampSet        bool
	packetInventoryComplete     bool
	packetInventoryLimitReached bool
}

func newCapturePacketTotals() capturePacketTotals {
	return capturePacketTotals{packetInventoryComplete: true}
}

func extractCaptureMetadata(path, extension string) (map[string]any, []string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, nil, fmt.Errorf("open capture: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("stat capture: %w", err)
	}
	if info.Size() < 4 {
		return nil, nil, errors.New("file is shorter than a capture magic value")
	}
	reader := &captureReader{file: file, fileSize: info.Size()}
	var magic [4]byte
	if err := reader.readAt(magic[:], 0); err != nil {
		return nil, nil, err
	}

	var metadata map[string]any
	var warnings []string
	switch {
	case bytes.Equal(magic[:], []byte{0x0a, 0x0d, 0x0d, 0x0a}):
		metadata, warnings, err = extractPCAPNGMetadata(reader)
		if err == nil && extension == ".pcap" {
			warnings = appendUniqueWarnings(warnings, "capture extension .pcap contains PCAPNG structure")
		}
	case classicPCAPMagic(magic[:]):
		metadata, warnings, err = extractClassicPCAPMetadata(reader, magic[:])
		if err == nil && extension == ".pcapng" {
			warnings = appendUniqueWarnings(warnings, "capture extension .pcapng contains classic PCAP structure")
		}
	default:
		return nil, nil, fmt.Errorf("unrecognized PCAP/PCAPNG magic %x", magic)
	}
	if err != nil {
		return nil, nil, err
	}
	metadata["file_size_bytes"] = info.Size()
	metadata["header_bytes_inspected"] = reader.bytesRead
	metadata["packet_payload_bytes_read"] = 0
	metadata["packet_payloads_retained"] = false
	metadata["protocols_decoded"] = false
	metadata["sessions_reconstructed"] = false
	metadata["endpoint_addresses_retained"] = false
	return metadata, warnings, nil
}

func extractClassicPCAPMetadata(reader *captureReader, magic []byte) (map[string]any, []string, error) {
	if reader.fileSize < 24 {
		return nil, nil, errors.New("file is shorter than the 24-byte PCAP header")
	}
	order, endianName, precision, err := classicPCAPFormat(magic)
	if err != nil {
		return nil, nil, err
	}
	var header [24]byte
	if err := reader.readAt(header[:], 0); err != nil {
		return nil, nil, err
	}
	major := order.Uint16(header[4:6])
	minor := order.Uint16(header[6:8])
	if major != 2 {
		return nil, nil, fmt.Errorf("unsupported PCAP major version %d", major)
	}
	reserved1 := order.Uint32(header[8:12])
	reserved2 := order.Uint32(header[12:16])
	snapLength := order.Uint32(header[16:20])
	if snapLength == 0 {
		return nil, nil, errors.New("PCAP snapshot length is zero")
	}
	if snapLength > maxCapturedPacketBytes {
		return nil, nil, fmt.Errorf("PCAP snapshot length %d exceeds inventory packet limit %d", snapLength, maxCapturedPacketBytes)
	}
	linkInfo := order.Uint32(header[20:24])
	linkType := uint16(linkInfo & 0xffff)
	reserved3 := (linkInfo >> 16) & 0x03ff
	packagePresent := linkInfo&0x04000000 != 0
	reservedBit := linkInfo&0x08000000 != 0
	fcsWords := uint8(linkInfo >> 28)

	warnings := []string{}
	inventoryComplete := true
	if minor != 4 {
		warnings = append(warnings, fmt.Sprintf("PCAP minor version is %d; current format version is 2.4", minor))
	}
	if reserved1 != 0 || reserved2 != 0 {
		warnings = append(warnings, "PCAP legacy reserved header fields are non-zero")
	}
	if reserved3 != 0 || reservedBit {
		warnings = append(warnings, "PCAP link-type reserved bits are non-zero")
		inventoryComplete = false
	}
	if !packagePresent && fcsWords != 0 {
		warnings = append(warnings, "PCAP FCS length bits are set while the presence bit is clear")
		inventoryComplete = false
	}

	totals := newCapturePacketTotals()
	position := int64(24)
	for position < reader.fileSize {
		if totals.packetCount >= maxCapturePackets {
			totals.packetInventoryComplete = false
			totals.packetInventoryLimitReached = true
			warnings = appendUniqueWarnings(warnings, fmt.Sprintf("PCAP packet inventory stopped at limit %d", maxCapturePackets))
			break
		}
		if reader.fileSize-position < 16 {
			return nil, nil, fmt.Errorf("truncated PCAP packet header at byte %d", position)
		}
		var packetHeader [16]byte
		if err := reader.readAt(packetHeader[:], position); err != nil {
			return nil, nil, err
		}
		seconds := order.Uint32(packetHeader[0:4])
		fraction := order.Uint32(packetHeader[4:8])
		capturedLength := order.Uint32(packetHeader[8:12])
		originalLength := order.Uint32(packetHeader[12:16])
		fractionLimit := uint32(1_000_000)
		nanosecondsPerUnit := int64(1000)
		if precision == "nanoseconds" {
			fractionLimit = 1_000_000_000
			nanosecondsPerUnit = 1
		}
		if fraction >= fractionLimit {
			return nil, nil, fmt.Errorf("PCAP packet %d timestamp fraction %d exceeds %s range", totals.packetCount, fraction, precision)
		}
		if capturedLength > snapLength {
			return nil, nil, fmt.Errorf("PCAP packet %d captured length %d exceeds snapshot length %d", totals.packetCount, capturedLength, snapLength)
		}
		if capturedLength > maxCapturedPacketBytes {
			return nil, nil, fmt.Errorf("PCAP packet %d captured length %d exceeds inventory limit %d", totals.packetCount, capturedLength, maxCapturedPacketBytes)
		}
		dataStart := position + 16
		if int64(capturedLength) > reader.fileSize-dataStart {
			return nil, nil, fmt.Errorf("PCAP packet %d data exceeds available file bytes", totals.packetCount)
		}
		timestamp := time.Unix(int64(seconds), int64(fraction)*nanosecondsPerUnit).UTC()
		totals.observePacket(capturedLength, originalLength, timestamp, true)
		position = dataStart + int64(capturedLength)
	}
	if totals.originalSmallerCount > 0 {
		warnings = appendUniqueWarnings(warnings, fmt.Sprintf("PCAP has %d packets whose original length is smaller than captured length", totals.originalSmallerCount))
		inventoryComplete = false
	}
	if totals.timestampOrderRegression > 0 {
		warnings = appendUniqueWarnings(warnings, fmt.Sprintf("PCAP has %d timestamp order regressions", totals.timestampOrderRegression))
	}

	metadata := mediaMetadataBase("pcap")
	metadata["byte_order"] = endianName
	metadata["timestamp_precision"] = precision
	metadata["version_major"] = major
	metadata["version_minor"] = minor
	metadata["reserved1"] = reserved1
	metadata["reserved2"] = reserved2
	metadata["snapshot_length_bytes"] = snapLength
	metadata["link_type"] = linkType
	metadata["link_type_name"] = captureLinkTypeName(linkType)
	metadata["fcs_length_present"] = packagePresent
	if packagePresent {
		metadata["fcs_length_bytes"] = int(fcsWords) * 2
	}
	metadata["inventory_complete"] = inventoryComplete && totals.packetInventoryComplete
	metadata["content_validation"] = "file_header_and_packet_record_boundaries"
	applyCaptureTotals(metadata, totals)
	return metadata, warnings, nil
}

func extractPCAPNGMetadata(reader *captureReader) (map[string]any, []string, error) {
	metadata := mediaMetadataBase("pcapng")
	warnings := []string{}
	totals := newCapturePacketTotals()
	position := int64(0)
	blockCount := 0
	sectionCount := 0
	interfaceCount := 0
	enhancedPacketBlocks := 0
	simplePacketBlocks := 0
	obsoletePacketBlocks := 0
	nameResolutionBlocks := 0
	interfaceStatisticsBlocks := 0
	customBlocks := 0
	unknownBlocks := 0
	decryptionSecretsBlocks := 0
	optionsComplete := true
	inventoryComplete := true
	sectionSummaries := []map[string]any{}
	interfaceSummaries := []*pcapngInterface{}
	var current *pcapngSection

	for position < reader.fileSize {
		if blockCount >= maxCaptureBlocks {
			warnings = appendUniqueWarnings(warnings, fmt.Sprintf("PCAPNG block inventory stopped at limit %d", maxCaptureBlocks))
			inventoryComplete = false
			break
		}
		if reader.fileSize-position < 12 {
			return nil, nil, fmt.Errorf("truncated PCAPNG block header at byte %d", position)
		}
		var prefix [12]byte
		if err := reader.readAt(prefix[:], position); err != nil {
			return nil, nil, err
		}
		if bytes.Equal(prefix[:4], []byte{0x0a, 0x0d, 0x0d, 0x0a}) {
			if current != nil && current.expectedEnd >= 0 && current.expectedEnd != position {
				return nil, nil, fmt.Errorf("PCAPNG section %d declared end %d but next section starts at %d", current.index, current.expectedEnd, position)
			}
			section, summary, blockLength, err := readPCAPNGSection(reader, position, sectionCount)
			if err != nil {
				return nil, nil, err
			}
			sectionCount++
			if sectionCount > maxCaptureSections {
				return nil, nil, fmt.Errorf("PCAPNG exceeds section limit %d", maxCaptureSections)
			}
			current = section
			sectionSummaries = append(sectionSummaries, summary)
			position += int64(blockLength)
			blockCount++
			continue
		}
		if current == nil {
			return nil, nil, errors.New("PCAPNG file does not begin with a Section Header Block")
		}
		blockType := current.order.Uint32(prefix[0:4])
		blockLength := current.order.Uint32(prefix[4:8])
		if err := validatePCAPNGBlock(reader, current.order, position, blockLength, 12); err != nil {
			return nil, nil, fmt.Errorf("PCAPNG block %d at byte %d: %w", blockCount, position, err)
		}

		switch blockType {
		case pcapngInterfaceDescription:
			iface, complete, optionWarnings, err := readPCAPNGInterface(reader, current, position, blockLength)
			if err != nil {
				return nil, nil, err
			}
			warnings = appendUniqueWarnings(warnings, optionWarnings...)
			if !complete {
				optionsComplete = false
				inventoryComplete = false
			}
			current.interfaces = append(current.interfaces, iface)
			interfaceSummaries = append(interfaceSummaries, iface)
			interfaceCount++
			if interfaceCount > maxCaptureInterfaces || len(current.interfaces) > maxCaptureInterfacesSection {
				return nil, nil, fmt.Errorf("PCAPNG exceeds interface limits total=%d per_section=%d", maxCaptureInterfaces, maxCaptureInterfacesSection)
			}
		case pcapngEnhancedPacketBlock:
			if totals.packetCount >= maxCapturePackets {
				totals.packetInventoryComplete = false
				totals.packetInventoryLimitReached = true
				warnings = appendUniqueWarnings(warnings, fmt.Sprintf("PCAPNG packet inventory stopped at limit %d", maxCapturePackets))
				inventoryComplete = false
				position = reader.fileSize
				continue
			}
			if err := readPCAPNGEnhancedPacket(reader, current, position, blockLength, &totals); err != nil {
				return nil, nil, err
			}
			enhancedPacketBlocks++
		case pcapngSimplePacketBlock:
			if totals.packetCount >= maxCapturePackets {
				totals.packetInventoryComplete = false
				totals.packetInventoryLimitReached = true
				warnings = appendUniqueWarnings(warnings, fmt.Sprintf("PCAPNG packet inventory stopped at limit %d", maxCapturePackets))
				inventoryComplete = false
				position = reader.fileSize
				continue
			}
			if err := readPCAPNGSimplePacket(reader, current, position, blockLength, &totals); err != nil {
				return nil, nil, err
			}
			simplePacketBlocks++
		case pcapngObsoletePacketBlock:
			if totals.packetCount >= maxCapturePackets {
				totals.packetInventoryComplete = false
				totals.packetInventoryLimitReached = true
				warnings = appendUniqueWarnings(warnings, fmt.Sprintf("PCAPNG packet inventory stopped at limit %d", maxCapturePackets))
				inventoryComplete = false
				position = reader.fileSize
				continue
			}
			if err := readPCAPNGObsoletePacket(reader, current, position, blockLength, &totals); err != nil {
				return nil, nil, err
			}
			obsoletePacketBlocks++
			warnings = appendUniqueWarnings(warnings, "PCAPNG contains obsolete Packet Blocks")
		case pcapngNameResolutionBlock:
			nameResolutionBlocks++
		case pcapngInterfaceStatisticsBlock:
			if blockLength < 24 {
				return nil, nil, errors.New("PCAPNG Interface Statistics Block is shorter than 24 bytes")
			}
			var fixed [12]byte
			if err := reader.readAt(fixed[:], position+8); err != nil {
				return nil, nil, err
			}
			interfaceID := current.order.Uint32(fixed[0:4])
			if int(interfaceID) >= len(current.interfaces) {
				return nil, nil, fmt.Errorf("PCAPNG Interface Statistics Block references missing interface %d", interfaceID)
			}
			interfaceStatisticsBlocks++
		case pcapngDecryptionSecretsBlock:
			decryptionSecretsBlocks++
			warnings = appendUniqueWarnings(warnings, "PCAPNG contains decryption-secrets blocks; secret payloads were not read or retained")
		case pcapngCustomBlockCopyable, pcapngCustomBlockNonCopyable:
			customBlocks++
		default:
			unknownBlocks++
		}
		position += int64(blockLength)
		blockCount++
	}
	if current != nil && current.expectedEnd >= 0 && current.expectedEnd != reader.fileSize {
		return nil, nil, fmt.Errorf("PCAPNG section %d declared end %d but file ends at %d", current.index, current.expectedEnd, reader.fileSize)
	}
	if sectionCount == 0 {
		return nil, nil, errors.New("PCAPNG contains no Section Header Block")
	}
	if totals.originalSmallerCount > 0 {
		warnings = appendUniqueWarnings(warnings, fmt.Sprintf("PCAPNG has %d packets whose original length is smaller than captured length", totals.originalSmallerCount))
		inventoryComplete = false
	}
	if totals.timestampOrderRegression > 0 {
		warnings = appendUniqueWarnings(warnings, fmt.Sprintf("PCAPNG has %d timestamp order regressions", totals.timestampOrderRegression))
	}

	interfaceOutput := make([]map[string]any, 0, len(interfaceSummaries))
	for _, iface := range interfaceSummaries {
		resolutionBase := 10
		if iface.timestampBase2 {
			resolutionBase = 2
		}
		interfaceOutput = append(interfaceOutput, map[string]any{
			"section_index":              iface.sectionIndex,
			"interface_id":               iface.interfaceID,
			"link_type":                  iface.linkType,
			"link_type_name":             captureLinkTypeName(iface.linkType),
			"snapshot_length_bytes":      iface.snapLength,
			"timestamp_resolution_base":  resolutionBase,
			"timestamp_resolution_power": iface.timestampExponent,
			"timestamp_offset_seconds":   iface.timestampOffsetSeconds,
			"timestamp_config_complete":  iface.timestampConfigComplete,
			"packet_count":               iface.packetCount,
			"option_count":               iface.optionCount,
			"name_option_present":        iface.nameOptionPresent,
			"address_option_count":       iface.addressOptionCount,
			"filter_option_present":      iface.filterOptionPresent,
		})
	}
	metadata["section_count"] = sectionCount
	metadata["sections"] = sectionSummaries
	metadata["interface_count"] = interfaceCount
	metadata["interfaces"] = interfaceOutput
	metadata["block_count"] = blockCount
	metadata["enhanced_packet_block_count"] = enhancedPacketBlocks
	metadata["simple_packet_block_count"] = simplePacketBlocks
	metadata["obsolete_packet_block_count"] = obsoletePacketBlocks
	metadata["name_resolution_block_count"] = nameResolutionBlocks
	metadata["interface_statistics_block_count"] = interfaceStatisticsBlocks
	metadata["custom_block_count"] = customBlocks
	metadata["unknown_block_count"] = unknownBlocks
	metadata["decryption_secrets_block_count"] = decryptionSecretsBlocks
	metadata["interface_options_complete"] = optionsComplete
	metadata["interface_names_retained"] = false
	metadata["interface_addresses_retained"] = false
	metadata["capture_filters_retained"] = false
	metadata["name_resolution_values_retained"] = false
	metadata["decryption_secrets_retained"] = false
	metadata["inventory_complete"] = inventoryComplete && totals.packetInventoryComplete
	metadata["content_validation"] = "section_block_interface_and_packet_boundaries"
	applyCaptureTotals(metadata, totals)
	return metadata, warnings, nil
}

func readPCAPNGSection(reader *captureReader, position int64, index int) (*pcapngSection, map[string]any, uint32, error) {
	var fixed [24]byte
	if err := reader.readAt(fixed[:], position); err != nil {
		return nil, nil, 0, err
	}
	var order binary.ByteOrder
	endianName := ""
	switch {
	case bytes.Equal(fixed[8:12], []byte{0x1a, 0x2b, 0x3c, 0x4d}):
		order = binary.BigEndian
		endianName = "big"
	case bytes.Equal(fixed[8:12], []byte{0x4d, 0x3c, 0x2b, 0x1a}):
		order = binary.LittleEndian
		endianName = "little"
	default:
		return nil, nil, 0, fmt.Errorf("PCAPNG section %d has invalid byte-order magic", index)
	}
	blockLength := order.Uint32(fixed[4:8])
	if err := validatePCAPNGBlock(reader, order, position, blockLength, 28); err != nil {
		return nil, nil, 0, fmt.Errorf("PCAPNG section %d: %w", index, err)
	}
	major := order.Uint16(fixed[12:14])
	minor := order.Uint16(fixed[14:16])
	if major != 1 {
		return nil, nil, 0, fmt.Errorf("unsupported PCAPNG section major version %d", major)
	}
	if minor != 0 && minor != 2 {
		return nil, nil, 0, fmt.Errorf("unsupported PCAPNG section minor version %d", minor)
	}
	declaredLength := int64(order.Uint64(fixed[16:24]))
	expectedEnd := int64(-1)
	if declaredLength != -1 {
		if declaredLength < 0 || declaredLength > reader.fileSize-position-int64(blockLength) {
			return nil, nil, 0, fmt.Errorf("PCAPNG section %d declared length %d exceeds available bytes", index, declaredLength)
		}
		expectedEnd = position + int64(blockLength) + declaredLength
	}
	section := &pcapngSection{
		index:       index,
		order:       order,
		endianName:  endianName,
		major:       major,
		minor:       minor,
		expectedEnd: expectedEnd,
	}
	summary := map[string]any{
		"section_index":         index,
		"byte_order":            endianName,
		"version_major":         major,
		"version_minor":         minor,
		"declared_length_bytes": declaredLength,
	}
	return section, summary, blockLength, nil
}

func readPCAPNGInterface(reader *captureReader, section *pcapngSection, position int64, blockLength uint32) (*pcapngInterface, bool, []string, error) {
	if blockLength < 20 {
		return nil, false, nil, errors.New("PCAPNG Interface Description Block is shorter than 20 bytes")
	}
	var fixed [8]byte
	if err := reader.readAt(fixed[:], position+8); err != nil {
		return nil, false, nil, err
	}
	iface := &pcapngInterface{
		sectionIndex:            section.index,
		interfaceID:             uint32(len(section.interfaces)),
		linkType:                section.order.Uint16(fixed[0:2]),
		snapLength:              section.order.Uint32(fixed[4:8]),
		timestampExponent:       6,
		timestampConfigComplete: true,
	}
	warnings := []string{}
	complete := true
	if section.order.Uint16(fixed[2:4]) != 0 {
		warnings = append(warnings, fmt.Sprintf("PCAPNG section %d interface %d has non-zero reserved field", section.index, iface.interfaceID))
		complete = false
	}
	if iface.snapLength > maxCapturedPacketBytes {
		return nil, false, nil, fmt.Errorf("PCAPNG interface %d snapshot length %d exceeds inventory limit %d", iface.interfaceID, iface.snapLength, maxCapturedPacketBytes)
	}
	optionsLength := int(blockLength) - 20
	if optionsLength == 0 {
		return iface, complete, warnings, nil
	}
	if optionsLength > maxCaptureOptionsBytes {
		iface.timestampConfigComplete = false
		warnings = append(warnings, fmt.Sprintf("PCAPNG section %d interface %d options exceed inspection limit %d bytes", section.index, iface.interfaceID, maxCaptureOptionsBytes))
		return iface, false, warnings, nil
	}
	options := make([]byte, optionsLength)
	if err := reader.readAt(options, position+16); err != nil {
		return nil, false, nil, err
	}
	offset := 0
	for offset < len(options) {
		if len(options)-offset < 4 {
			return nil, false, nil, errors.New("PCAPNG interface option header is truncated")
		}
		optionType := section.order.Uint16(options[offset : offset+2])
		optionLength := int(section.order.Uint16(options[offset+2 : offset+4]))
		offset += 4
		paddedLength := alignCapture32(optionLength)
		if optionLength > len(options)-offset || paddedLength > len(options)-offset {
			return nil, false, nil, errors.New("PCAPNG interface option exceeds block boundary")
		}
		value := options[offset : offset+optionLength]
		padding := options[offset+optionLength : offset+paddedLength]
		if !allZero(padding) {
			warnings = appendUniqueWarnings(warnings, fmt.Sprintf("PCAPNG section %d interface %d has non-zero option padding", section.index, iface.interfaceID))
			complete = false
		}
		offset += paddedLength
		if optionType == 0 {
			if optionLength != 0 {
				return nil, false, nil, errors.New("PCAPNG end-of-options marker has non-zero length")
			}
			break
		}
		iface.optionCount++
		switch optionType {
		case 2, 3:
			iface.nameOptionPresent = true
		case 4, 5, 6, 7:
			iface.addressOptionCount++
		case 9:
			if optionLength != 1 {
				return nil, false, nil, errors.New("PCAPNG if_tsresol option length is not 1")
			}
			iface.timestampBase2 = value[0]&0x80 != 0
			iface.timestampExponent = value[0] & 0x7f
		case 11:
			iface.filterOptionPresent = true
		case 14:
			if optionLength != 8 {
				return nil, false, nil, errors.New("PCAPNG if_tsoffset option length is not 8")
			}
			iface.timestampOffsetSeconds = int64(section.order.Uint64(value))
		}
	}
	return iface, complete, warnings, nil
}

func readPCAPNGEnhancedPacket(reader *captureReader, section *pcapngSection, position int64, blockLength uint32, totals *capturePacketTotals) error {
	if blockLength < 32 {
		return errors.New("PCAPNG Enhanced Packet Block is shorter than 32 bytes")
	}
	var fixed [20]byte
	if err := reader.readAt(fixed[:], position+8); err != nil {
		return err
	}
	interfaceID := section.order.Uint32(fixed[0:4])
	if int(interfaceID) >= len(section.interfaces) {
		return fmt.Errorf("PCAPNG Enhanced Packet Block references missing interface %d", interfaceID)
	}
	iface := section.interfaces[interfaceID]
	timestampRaw := uint64(section.order.Uint32(fixed[4:8]))<<32 | uint64(section.order.Uint32(fixed[8:12]))
	capturedLength := section.order.Uint32(fixed[12:16])
	originalLength := section.order.Uint32(fixed[16:20])
	if err := validatePCAPNGPacketLengths(iface, capturedLength, originalLength, blockLength, 32); err != nil {
		return err
	}
	timestamp, timestampKnown := pcapngTimestamp(timestampRaw, iface)
	totals.observePacket(capturedLength, originalLength, timestamp, timestampKnown)
	iface.packetCount++
	return nil
}

func readPCAPNGSimplePacket(reader *captureReader, section *pcapngSection, position int64, blockLength uint32, totals *capturePacketTotals) error {
	if blockLength < 16 {
		return errors.New("PCAPNG Simple Packet Block is shorter than 16 bytes")
	}
	if len(section.interfaces) == 0 {
		return errors.New("PCAPNG Simple Packet Block has no interface 0")
	}
	var originalBytes [4]byte
	if err := reader.readAt(originalBytes[:], position+8); err != nil {
		return err
	}
	iface := section.interfaces[0]
	originalLength := section.order.Uint32(originalBytes[:])
	capturedLength := originalLength
	if iface.snapLength != 0 && capturedLength > iface.snapLength {
		capturedLength = iface.snapLength
	}
	if capturedLength > maxCapturedPacketBytes {
		return fmt.Errorf("PCAPNG Simple Packet captured length %d exceeds inventory limit %d", capturedLength, maxCapturedPacketBytes)
	}
	if uint32(alignCapture32(int(capturedLength))) != blockLength-16 {
		return errors.New("PCAPNG Simple Packet data length does not match block boundary")
	}
	totals.observePacket(capturedLength, originalLength, time.Time{}, false)
	iface.packetCount++
	return nil
}

func readPCAPNGObsoletePacket(reader *captureReader, section *pcapngSection, position int64, blockLength uint32, totals *capturePacketTotals) error {
	if blockLength < 32 {
		return errors.New("PCAPNG obsolete Packet Block is shorter than 32 bytes")
	}
	var fixed [20]byte
	if err := reader.readAt(fixed[:], position+8); err != nil {
		return err
	}
	interfaceID := uint32(section.order.Uint16(fixed[0:2]))
	if int(interfaceID) >= len(section.interfaces) {
		return fmt.Errorf("PCAPNG obsolete Packet Block references missing interface %d", interfaceID)
	}
	iface := section.interfaces[interfaceID]
	timestampRaw := uint64(section.order.Uint32(fixed[4:8]))<<32 | uint64(section.order.Uint32(fixed[8:12]))
	capturedLength := section.order.Uint32(fixed[12:16])
	originalLength := section.order.Uint32(fixed[16:20])
	if err := validatePCAPNGPacketLengths(iface, capturedLength, originalLength, blockLength, 32); err != nil {
		return err
	}
	timestamp, timestampKnown := pcapngTimestamp(timestampRaw, iface)
	totals.observePacket(capturedLength, originalLength, timestamp, timestampKnown)
	iface.packetCount++
	return nil
}

func validatePCAPNGPacketLengths(iface *pcapngInterface, capturedLength, _ uint32, blockLength uint32, minimumLength uint32) error {
	if capturedLength > maxCapturedPacketBytes {
		return fmt.Errorf("PCAPNG captured length %d exceeds inventory limit %d", capturedLength, maxCapturedPacketBytes)
	}
	if iface.snapLength != 0 && capturedLength > iface.snapLength {
		return fmt.Errorf("PCAPNG captured length %d exceeds interface snapshot length %d", capturedLength, iface.snapLength)
	}
	paddedLength := uint32(alignCapture32(int(capturedLength)))
	if paddedLength > blockLength-minimumLength {
		return errors.New("PCAPNG packet data exceeds block boundary")
	}
	return nil
}

func validatePCAPNGBlock(reader *captureReader, order binary.ByteOrder, position int64, blockLength uint32, minimumLength uint32) error {
	if blockLength < minimumLength || blockLength%4 != 0 {
		return fmt.Errorf("block length %d is below minimum %d or not 32-bit aligned", blockLength, minimumLength)
	}
	if blockLength > maxCaptureBlockBytes {
		return fmt.Errorf("block length %d exceeds inventory limit %d", blockLength, maxCaptureBlockBytes)
	}
	if int64(blockLength) > reader.fileSize-position {
		return fmt.Errorf("block length %d exceeds available file bytes", blockLength)
	}
	var trailer [4]byte
	if err := reader.readAt(trailer[:], position+int64(blockLength)-4); err != nil {
		return err
	}
	if order.Uint32(trailer[:]) != blockLength {
		return fmt.Errorf("leading and trailing block lengths disagree (%d != %d)", blockLength, order.Uint32(trailer[:]))
	}
	return nil
}

func (reader *captureReader) readAt(destination []byte, offset int64) error {
	if offset < 0 || int64(len(destination)) > reader.fileSize-offset {
		return io.ErrUnexpectedEOF
	}
	if reader.bytesRead+int64(len(destination)) > maxCaptureHeaderBytes {
		return fmt.Errorf("capture header inspection exceeds byte limit %d", maxCaptureHeaderBytes)
	}
	if _, err := reader.file.ReadAt(destination, offset); err != nil {
		return fmt.Errorf("read capture at byte %d: %w", offset, err)
	}
	reader.bytesRead += int64(len(destination))
	return nil
}

func (totals *capturePacketTotals) observePacket(capturedLength, originalLength uint32, timestamp time.Time, timestampKnown bool) {
	totals.packetCount++
	totals.totalCapturedBytes += uint64(capturedLength)
	totals.totalOriginalBytes += uint64(originalLength)
	if capturedLength > totals.maximumCapturedBytes {
		totals.maximumCapturedBytes = capturedLength
	}
	if originalLength > totals.maximumOriginalBytes {
		totals.maximumOriginalBytes = originalLength
	}
	if capturedLength < originalLength {
		totals.truncatedPacketCount++
	}
	if originalLength < capturedLength {
		totals.originalSmallerCount++
	}
	if !timestampKnown {
		totals.timestampUnavailableCount++
		return
	}
	if !totals.firstTimestampSet || timestamp.Before(totals.firstTimestamp) {
		totals.firstTimestamp = timestamp
	}
	if !totals.firstTimestampSet || timestamp.After(totals.lastTimestamp) {
		totals.lastTimestamp = timestamp
	}
	totals.firstTimestampSet = true
	if totals.previousTimestampSet && timestamp.Before(totals.previousTimestamp) {
		totals.timestampOrderRegression++
	}
	totals.previousTimestamp = timestamp
	totals.previousTimestampSet = true
}

func applyCaptureTotals(metadata map[string]any, totals capturePacketTotals) {
	metadata["packet_count"] = totals.packetCount
	metadata["total_captured_bytes"] = totals.totalCapturedBytes
	metadata["total_original_bytes"] = totals.totalOriginalBytes
	metadata["maximum_captured_packet_bytes"] = totals.maximumCapturedBytes
	metadata["maximum_original_packet_bytes"] = totals.maximumOriginalBytes
	metadata["truncated_packet_count"] = totals.truncatedPacketCount
	metadata["original_smaller_than_captured_count"] = totals.originalSmallerCount
	metadata["timestamp_order_regression_count"] = totals.timestampOrderRegression
	metadata["timestamp_unavailable_packet_count"] = totals.timestampUnavailableCount
	metadata["packet_inventory_complete"] = totals.packetInventoryComplete
	metadata["packet_inventory_limit_reached"] = totals.packetInventoryLimitReached
	metadata["timestamp_bounds_available"] = totals.firstTimestampSet
	if totals.firstTimestampSet {
		metadata["first_timestamp_utc"] = totals.firstTimestamp.Format(time.RFC3339Nano)
		metadata["last_timestamp_utc"] = totals.lastTimestamp.Format(time.RFC3339Nano)
	}
}

func pcapngTimestamp(raw uint64, iface *pcapngInterface) (time.Time, bool) {
	if !iface.timestampConfigComplete {
		return time.Time{}, false
	}
	denominator := new(big.Int)
	if iface.timestampBase2 {
		denominator.Lsh(big.NewInt(1), uint(iface.timestampExponent))
	} else {
		denominator.Exp(big.NewInt(10), big.NewInt(int64(iface.timestampExponent)), nil)
	}
	value := new(big.Int).SetUint64(raw)
	seconds := new(big.Int)
	remainder := new(big.Int)
	seconds.QuoRem(value, denominator, remainder)
	if !seconds.IsInt64() {
		return time.Time{}, false
	}
	secondValue := seconds.Int64()
	if (iface.timestampOffsetSeconds > 0 && secondValue > math.MaxInt64-iface.timestampOffsetSeconds) ||
		(iface.timestampOffsetSeconds < 0 && secondValue < math.MinInt64-iface.timestampOffsetSeconds) {
		return time.Time{}, false
	}
	secondValue += iface.timestampOffsetSeconds
	nanoseconds := new(big.Int).Mul(remainder, big.NewInt(1_000_000_000))
	nanoseconds.Quo(nanoseconds, denominator)
	if !nanoseconds.IsInt64() {
		return time.Time{}, false
	}
	return time.Unix(secondValue, nanoseconds.Int64()).UTC(), true
}

func classicPCAPMagic(value []byte) bool {
	_, _, _, err := classicPCAPFormat(value)
	return err == nil
}

func classicPCAPFormat(value []byte) (binary.ByteOrder, string, string, error) {
	switch {
	case bytes.Equal(value, []byte{0xa1, 0xb2, 0xc3, 0xd4}):
		return binary.BigEndian, "big", "microseconds", nil
	case bytes.Equal(value, []byte{0xd4, 0xc3, 0xb2, 0xa1}):
		return binary.LittleEndian, "little", "microseconds", nil
	case bytes.Equal(value, []byte{0xa1, 0xb2, 0x3c, 0x4d}):
		return binary.BigEndian, "big", "nanoseconds", nil
	case bytes.Equal(value, []byte{0x4d, 0x3c, 0xb2, 0xa1}):
		return binary.LittleEndian, "little", "nanoseconds", nil
	default:
		return nil, "", "", errors.New("not a classic PCAP magic value")
	}
}

func captureLinkTypeName(value uint16) string {
	switch value {
	case 0:
		return "null_loopback"
	case 1:
		return "ethernet"
	case 101:
		return "raw_ip"
	case 113:
		return "linux_cooked_v1"
	case 127:
		return "ieee802_11_radiotap"
	case 228:
		return "ipv4"
	case 229:
		return "ipv6"
	case 276:
		return "linux_cooked_v2"
	default:
		return "unmapped"
	}
}

func alignCapture32(value int) int {
	return (value + 3) &^ 3
}
