package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	maxTIFFIFDEntries           = 512
	decodeReviewThresholdPixels = uint64(100_000_000)
)

var deterministicImageFormats = map[string]string{
	".png":  "png",
	".jpg":  "jpeg",
	".jpeg": "jpeg",
	".gif":  "gif",
	".bmp":  "bmp",
	".tif":  "tiff",
	".tiff": "tiff",
	".webp": "webp",
}

func isDeterministicImageExtension(extension string) bool {
	_, supported := deterministicImageFormats[extension]
	return supported
}

func extractImageMetadata(path, extension string) (map[string]any, []string, error) {
	info, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return nil, nil, fmt.Errorf("stat image: %w", err)
	}
	format, err := detectImageFormat(path)
	if err != nil {
		return nil, nil, err
	}
	var metadata map[string]any
	var warnings []string
	switch format {
	case "png", "jpeg", "gif":
		metadata, err = extractStandardImageMetadata(path)
	case "bmp":
		metadata, err = extractBMPMetadata(path)
	case "tiff":
		metadata, err = extractTIFFMetadata(path)
	case "webp":
		metadata, err = extractWebPMetadata(path)
	default:
		err = fmt.Errorf("unsupported image signature")
	}
	if err != nil {
		return nil, nil, err
	}
	boundaryValidated, boundaryErr := validateImageContainerBoundary(path, format)
	if boundaryErr != nil {
		return nil, nil, boundaryErr
	}
	metadata["container_boundary_validated"] = boundaryValidated

	if expected := deterministicImageFormats[extension]; expected != format {
		warnings = append(warnings, fmt.Sprintf("image extension %q does not match detected %s content", extension, format))
		metadata["extension_content_match"] = false
	} else {
		metadata["extension_content_match"] = true
	}
	if format == "jpeg" {
		orientation, found, orientationErr := extractJPEGEXIFOrientation(path)
		if orientationErr != nil {
			warnings = append(warnings, "bounded JPEG EXIF orientation unavailable: "+orientationErr.Error())
		} else if found {
			addOrientationMetadata(metadata, orientation)
		}
	}
	addImageIntakeMetadata(metadata, info.Size(), &warnings)
	return metadata, warnings, nil
}

func validateImageContainerBoundary(path, format string) (bool, error) {
	switch format {
	case "png":
		file, err := os.Open(filepath.Clean(path))
		if err != nil {
			return false, fmt.Errorf("open PNG boundary: %w", err)
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return false, fmt.Errorf("stat PNG boundary: %w", err)
		}
		if info.Size() < 12 {
			return false, fmt.Errorf("PNG is shorter than its terminal chunk")
		}
		var terminal [12]byte
		if _, err := file.ReadAt(terminal[:], info.Size()-12); err != nil {
			return false, fmt.Errorf("read PNG terminal chunk: %w", err)
		}
		expected := []byte{0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82}
		if !bytes.Equal(terminal[:], expected) {
			return false, fmt.Errorf("PNG has trailing or invalid content after its terminal IEND chunk")
		}
		return true, nil
	case "bmp", "webp":
		// Their parsers verify the declared file/container size exactly.
		return true, nil
	default:
		return false, nil
	}
}

func imageIntakeFailureMetadata(path, extension string, cause error) map[string]any {
	metadata := mediaMetadataBase("unverified")
	metadata["intake_contract_version"] = imageIntakeContractVersion
	metadata["validation_state"] = "manual_review_required"
	metadata["downstream_decode_allowed"] = false
	metadata["inspection_mode"] = "bounded_header_only"
	metadata["supplied_extension"] = extension
	metadata["metadata_privacy_policy"] = "orientation_only_no_exif_free_text_or_gps"
	metadata["validation_reason"] = cause.Error()
	if info, err := os.Stat(filepath.Clean(path)); err == nil {
		metadata["source_size_bytes"] = info.Size()
	}
	return metadata
}

func detectImageFormat(path string) (string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("open image: %w", err)
	}
	defer file.Close()

	var header [12]byte
	read, err := io.ReadFull(file, header[:])
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", fmt.Errorf("read image signature: %w", err)
	}
	data := header[:read]
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}):
		return "png", nil
	case len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8:
		return "jpeg", nil
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return "gif", nil
	case len(data) >= 2 && string(data[:2]) == "BM":
		return "bmp", nil
	case len(data) >= 4 && (bytes.Equal(data[:4], []byte{'I', 'I', 42, 0}) || bytes.Equal(data[:4], []byte{'M', 'M', 0, 42})):
		return "tiff", nil
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "webp", nil
	default:
		return "", fmt.Errorf("unrecognized image signature")
	}
}

func extractBMPMetadata(path string) (map[string]any, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open BMP: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat BMP: %w", err)
	}

	header := make([]byte, 26)
	if _, err := io.ReadFull(file, header); err != nil {
		return nil, fmt.Errorf("read BMP headers: %w", err)
	}
	if string(header[:2]) != "BM" {
		return nil, fmt.Errorf("invalid BMP signature")
	}
	declaredSize := binary.LittleEndian.Uint32(header[2:6])
	pixelOffset := binary.LittleEndian.Uint32(header[10:14])
	dibSize := binary.LittleEndian.Uint32(header[14:18])
	if int64(dibSize) > info.Size()-14 {
		return nil, fmt.Errorf("BMP DIB header exceeds available file bytes")
	}
	if declaredSize != 0 && (declaredSize < pixelOffset || int64(declaredSize) != info.Size()) {
		return nil, fmt.Errorf("BMP declared size does not match available file bytes or precedes pixel data")
	}
	if uint64(pixelOffset) < uint64(14)+uint64(dibSize) || int64(pixelOffset) > info.Size() {
		return nil, fmt.Errorf("BMP pixel offset is outside the valid file header boundary")
	}
	if dibSize >= 40 {
		extra := make([]byte, 28)
		if _, err := io.ReadFull(file, extra); err != nil {
			return nil, fmt.Errorf("read BMP info header: %w", err)
		}
		header = append(header, extra...)
	}

	var width, height int64
	var bitsPerPixel, planes uint16
	var compression uint32
	var topDown bool
	switch {
	case dibSize == 12:
		width = int64(binary.LittleEndian.Uint16(header[18:20]))
		height = int64(binary.LittleEndian.Uint16(header[20:22]))
		planes = binary.LittleEndian.Uint16(header[22:24])
		bitsPerPixel = binary.LittleEndian.Uint16(header[24:26])
	case dibSize >= 40:
		rawWidth := int32(binary.LittleEndian.Uint32(header[18:22]))
		rawHeight := int32(binary.LittleEndian.Uint32(header[22:26]))
		if rawWidth <= 0 || rawHeight == 0 || rawHeight == -1<<31 {
			return nil, fmt.Errorf("invalid BMP dimensions %dx%d", rawWidth, rawHeight)
		}
		width = int64(rawWidth)
		height = int64(rawHeight)
		if height < 0 {
			height = -height
			topDown = true
		}
		planes = binary.LittleEndian.Uint16(header[26:28])
		bitsPerPixel = binary.LittleEndian.Uint16(header[28:30])
		compression = binary.LittleEndian.Uint32(header[30:34])
	default:
		return nil, fmt.Errorf("unsupported BMP DIB header size %d", dibSize)
	}
	if width <= 0 || height <= 0 || planes != 1 || bitsPerPixel == 0 {
		return nil, fmt.Errorf("invalid BMP geometry or pixel format")
	}

	metadata := mediaMetadataBase("bmp")
	metadata["width_pixels"] = int(width)
	metadata["height_pixels"] = int(height)
	metadata["bits_per_pixel"] = bitsPerPixel
	metadata["compression_code"] = compression
	metadata["top_down_rows"] = topDown
	return metadata, nil
}

func extractTIFFMetadata(path string) (map[string]any, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open TIFF: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat TIFF: %w", err)
	}

	tags, err := readTIFFPrimaryIFD(file, info.Size())
	if err != nil {
		return nil, err
	}
	width, widthFound := tags[256]
	height, heightFound := tags[257]
	if !widthFound || !heightFound || width == 0 || height == 0 {
		return nil, fmt.Errorf("TIFF primary IFD is missing valid image dimensions")
	}

	metadata := mediaMetadataBase("tiff")
	metadata["width_pixels"] = int(width)
	metadata["height_pixels"] = int(height)
	if orientation, found := tags[274]; found {
		if orientation < 1 || orientation > 8 {
			return nil, fmt.Errorf("invalid TIFF orientation %d", orientation)
		}
		addOrientationMetadata(metadata, uint16(orientation))
	}
	return metadata, nil
}

func readTIFFPrimaryIFD(reader io.ReaderAt, size int64) (map[uint16]uint32, error) {
	var header [8]byte
	if _, err := reader.ReadAt(header[:], 0); err != nil {
		return nil, fmt.Errorf("read TIFF header: %w", err)
	}
	var order binary.ByteOrder
	switch string(header[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return nil, fmt.Errorf("invalid TIFF byte order")
	}
	if order.Uint16(header[2:4]) != 42 {
		return nil, fmt.Errorf("invalid TIFF magic")
	}
	ifdOffset := int64(order.Uint32(header[4:8]))
	if ifdOffset < 8 || ifdOffset > size-2 {
		return nil, fmt.Errorf("TIFF primary IFD offset is outside the file")
	}

	var countBytes [2]byte
	if _, err := reader.ReadAt(countBytes[:], ifdOffset); err != nil {
		return nil, fmt.Errorf("read TIFF IFD count: %w", err)
	}
	entryCount := int(order.Uint16(countBytes[:]))
	if entryCount > maxTIFFIFDEntries {
		return nil, fmt.Errorf("TIFF primary IFD has %d entries; limit is %d", entryCount, maxTIFFIFDEntries)
	}
	entriesSize := int64(entryCount * 12)
	if ifdOffset+2+entriesSize+4 > size {
		return nil, fmt.Errorf("TIFF primary IFD exceeds available file bytes")
	}
	entries := make([]byte, entriesSize)
	if _, err := reader.ReadAt(entries, ifdOffset+2); err != nil {
		return nil, fmt.Errorf("read TIFF IFD entries: %w", err)
	}

	tags := make(map[uint16]uint32, 3)
	for index := 0; index < entryCount; index++ {
		entry := entries[index*12 : (index+1)*12]
		tag := order.Uint16(entry[0:2])
		if tag != 256 && tag != 257 && tag != 274 {
			continue
		}
		fieldType := order.Uint16(entry[2:4])
		count := order.Uint32(entry[4:8])
		if count != 1 {
			return nil, fmt.Errorf("TIFF tag %d has unsupported value count %d", tag, count)
		}
		switch fieldType {
		case 3:
			tags[tag] = uint32(order.Uint16(entry[8:10]))
		case 4:
			tags[tag] = order.Uint32(entry[8:12])
		default:
			return nil, fmt.Errorf("TIFF tag %d has unsupported field type %d", tag, fieldType)
		}
	}
	return tags, nil
}

func extractJPEGEXIFOrientation(path string) (uint16, bool, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return 0, false, fmt.Errorf("open JPEG: %w", err)
	}
	defer file.Close()
	prefix, err := io.ReadAll(io.LimitReader(file, maxImageConfigBytes))
	if err != nil {
		return 0, false, fmt.Errorf("read bounded JPEG header: %w", err)
	}
	if len(prefix) < 2 || prefix[0] != 0xff || prefix[1] != 0xd8 {
		return 0, false, fmt.Errorf("invalid JPEG signature")
	}

	for offset, segmentCount := 2, 0; offset < len(prefix) && segmentCount < maxMediaHeaderChunks; segmentCount++ {
		for offset < len(prefix) && prefix[offset] == 0xff {
			offset++
		}
		if offset >= len(prefix) {
			break
		}
		marker := prefix[offset]
		offset++
		if marker == 0xd9 || marker == 0xda {
			break
		}
		if marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
			continue
		}
		if offset+2 > len(prefix) {
			return 0, false, fmt.Errorf("truncated JPEG segment length")
		}
		segmentLength := int(binary.BigEndian.Uint16(prefix[offset : offset+2]))
		if segmentLength < 2 || offset+segmentLength > len(prefix) {
			return 0, false, fmt.Errorf("JPEG segment exceeds the bounded header")
		}
		payload := prefix[offset+2 : offset+segmentLength]
		if marker == 0xe1 && len(payload) >= 6 && string(payload[:6]) == "Exif\x00\x00" {
			tiff := payload[6:]
			tags, err := readTIFFPrimaryIFD(bytes.NewReader(tiff), int64(len(tiff)))
			if err != nil {
				return 0, false, err
			}
			orientation, found := tags[274]
			if !found {
				return 0, false, nil
			}
			if orientation < 1 || orientation > 8 {
				return 0, false, fmt.Errorf("invalid EXIF orientation %d", orientation)
			}
			return uint16(orientation), true, nil
		}
		offset += segmentLength
	}
	return 0, false, nil
}

func extractWebPMetadata(path string) (map[string]any, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open WebP: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat WebP: %w", err)
	}

	var header [12]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return nil, fmt.Errorf("read WebP RIFF header: %w", err)
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WEBP" {
		return nil, fmt.Errorf("not a RIFF/WEBP file")
	}
	declaredEnd := int64(binary.LittleEndian.Uint32(header[4:8])) + 8
	if declaredEnd < 20 || declaredEnd != info.Size() {
		return nil, fmt.Errorf("WebP RIFF size does not match the available file bytes")
	}

	var canvasWidth, canvasHeight uint32
	var foundCanvas, foundBitstream, foundAnimationFrame, animated bool
	var featureFlags byte
	completedChunkScan := false
	for chunkCount := 0; chunkCount < maxMediaHeaderChunks; chunkCount++ {
		position, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("inspect WebP position: %w", err)
		}
		if position == declaredEnd {
			completedChunkScan = true
			break
		}
		if position >= maxImageConfigBytes {
			return nil, fmt.Errorf("WebP image header exceeds the %d-byte inspection limit", maxImageConfigBytes)
		}
		if declaredEnd-position < 8 {
			return nil, fmt.Errorf("truncated WebP chunk header")
		}
		var chunkHeader [8]byte
		if _, err := io.ReadFull(file, chunkHeader[:]); err != nil {
			return nil, fmt.Errorf("read WebP chunk header: %w", err)
		}
		chunkID := string(chunkHeader[:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])
		paddedSize := int64(chunkSize) + int64(chunkSize&1)
		position, _ = file.Seek(0, io.SeekCurrent)
		if paddedSize > declaredEnd-position {
			return nil, fmt.Errorf("WebP %q chunk exceeds the RIFF boundary", chunkID)
		}

		switch chunkID {
		case "VP8X":
			if chunkCount != 0 {
				return nil, fmt.Errorf("VP8X is not the first WebP chunk")
			}
			if chunkSize != 10 {
				return nil, fmt.Errorf("VP8X chunk length is %d, expected 10", chunkSize)
			}
			var payload [10]byte
			if _, err := io.ReadFull(file, payload[:]); err != nil {
				return nil, fmt.Errorf("read VP8X payload: %w", err)
			}
			if payload[0]&0xc1 != 0 || payload[1] != 0 || payload[2] != 0 || payload[3] != 0 {
				return nil, fmt.Errorf("VP8X reserved bits are not zero")
			}
			featureFlags = payload[0]
			animated = featureFlags&0x02 != 0
			canvasWidth = readLittleEndianUint24(payload[4:7]) + 1
			canvasHeight = readLittleEndianUint24(payload[7:10]) + 1
			foundCanvas = true
		case "VP8 ":
			if chunkSize < 10 {
				return nil, fmt.Errorf("VP8 chunk is shorter than its frame header")
			}
			var payload [10]byte
			if _, err := io.ReadFull(file, payload[:]); err != nil {
				return nil, fmt.Errorf("read VP8 frame header: %w", err)
			}
			if payload[0]&1 != 0 || !bytes.Equal(payload[3:6], []byte{0x9d, 0x01, 0x2a}) {
				return nil, fmt.Errorf("invalid VP8 key-frame header")
			}
			width := uint32(binary.LittleEndian.Uint16(payload[6:8]) & 0x3fff)
			height := uint32(binary.LittleEndian.Uint16(payload[8:10]) & 0x3fff)
			if !foundCanvas {
				canvasWidth, canvasHeight, foundCanvas = width, height, true
			}
			foundBitstream = true
			if _, err := file.Seek(int64(chunkSize)-10, io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("skip VP8 payload: %w", err)
			}
		case "VP8L":
			if chunkSize < 5 {
				return nil, fmt.Errorf("VP8L chunk is shorter than its image header")
			}
			var payload [5]byte
			if _, err := io.ReadFull(file, payload[:]); err != nil {
				return nil, fmt.Errorf("read VP8L image header: %w", err)
			}
			if payload[0] != 0x2f {
				return nil, fmt.Errorf("invalid VP8L signature")
			}
			bits := binary.LittleEndian.Uint32(payload[1:5])
			if bits>>29 != 0 {
				return nil, fmt.Errorf("unsupported VP8L version")
			}
			width := bits&0x3fff + 1
			height := (bits>>14)&0x3fff + 1
			if !foundCanvas {
				canvasWidth, canvasHeight, foundCanvas = width, height, true
			}
			foundBitstream = true
			if _, err := file.Seek(int64(chunkSize)-5, io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("skip VP8L payload: %w", err)
			}
		case "ANMF":
			if chunkSize < 16 {
				return nil, fmt.Errorf("ANMF chunk is shorter than its frame header")
			}
			foundBitstream = true
			foundAnimationFrame = true
			if _, err := file.Seek(int64(chunkSize), io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("skip ANMF payload: %w", err)
			}
		default:
			if _, err := file.Seek(int64(chunkSize), io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("skip WebP %q payload: %w", chunkID, err)
			}
		}
		if chunkSize&1 != 0 {
			if _, err := file.Seek(1, io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("skip WebP padding: %w", err)
			}
		}
	}
	if !completedChunkScan {
		position, err := file.Seek(0, io.SeekCurrent)
		if err != nil || position != declaredEnd {
			return nil, fmt.Errorf("WebP chunk count exceeds the inspection limit of %d", maxMediaHeaderChunks)
		}
	}
	if !foundCanvas || canvasWidth == 0 || canvasHeight == 0 || !foundBitstream {
		return nil, fmt.Errorf("WebP canvas or image bitstream is missing")
	}
	if animated != foundAnimationFrame {
		return nil, fmt.Errorf("WebP animation metadata is inconsistent")
	}

	metadata := mediaMetadataBase("webp")
	metadata["width_pixels"] = int(canvasWidth)
	metadata["height_pixels"] = int(canvasHeight)
	metadata["animated"] = animated
	metadata["has_alpha"] = featureFlags&0x10 != 0
	metadata["has_exif"] = featureFlags&0x08 != 0
	metadata["has_xmp"] = featureFlags&0x04 != 0
	metadata["has_icc_profile"] = featureFlags&0x20 != 0
	return metadata, nil
}

func readLittleEndianUint24(value []byte) uint32 {
	return uint32(value[0]) | uint32(value[1])<<8 | uint32(value[2])<<16
}

func addOrientationMetadata(metadata map[string]any, orientation uint16) {
	labels := map[uint16]string{
		1: "top_left",
		2: "top_right",
		3: "bottom_right",
		4: "bottom_left",
		5: "left_top",
		6: "right_top",
		7: "right_bottom",
		8: "left_bottom",
	}
	metadata["exif_orientation"] = orientation
	metadata["exif_orientation_label"] = labels[orientation]
	width, widthOK := metadata["width_pixels"].(int)
	height, heightOK := metadata["height_pixels"].(int)
	if widthOK && heightOK {
		if orientation >= 5 {
			metadata["display_width_pixels"] = height
			metadata["display_height_pixels"] = width
		} else {
			metadata["display_width_pixels"] = width
			metadata["display_height_pixels"] = height
		}
	}
}

func addImageIntakeMetadata(metadata map[string]any, sizeBytes int64, warnings *[]string) {
	metadata["intake_contract_version"] = imageIntakeContractVersion
	metadata["inspection_mode"] = "bounded_header_only"
	metadata["source_size_bytes"] = sizeBytes
	metadata["max_source_size_bytes"] = maxImageEvidenceBytes
	metadata["max_dimension_pixels"] = maxImageDimensionPixels
	metadata["max_pixel_count"] = decodeReviewThresholdPixels
	metadata["metadata_privacy_policy"] = "orientation_only_no_exif_free_text_or_gps"
	metadata["validation_state"] = "accepted_metadata_only"
	metadata["downstream_decode_allowed"] = true
	width, widthOK := metadata["width_pixels"].(int)
	height, heightOK := metadata["height_pixels"].(int)
	if !widthOK || !heightOK || width <= 0 || height <= 0 {
		metadata["validation_state"] = "manual_review_required"
		metadata["downstream_decode_allowed"] = false
		return
	}
	pixels := uint64(width) * uint64(height)
	metadata["pixel_count"] = pixels
	reviewRequired := sizeBytes > maxImageEvidenceBytes || width > maxImageDimensionPixels || height > maxImageDimensionPixels || pixels > decodeReviewThresholdPixels || metadata["extension_content_match"] == false
	metadata["decode_review_required"] = reviewRequired
	if reviewRequired {
		metadata["validation_state"] = "manual_review_required"
		metadata["downstream_decode_allowed"] = false
	}
	if sizeBytes > maxImageEvidenceBytes {
		*warnings = append(*warnings, fmt.Sprintf("image size %d bytes exceeds the %d-byte automatic-processing limit", sizeBytes, maxImageEvidenceBytes))
	}
	if width > maxImageDimensionPixels || height > maxImageDimensionPixels {
		*warnings = append(*warnings, fmt.Sprintf("image dimensions %dx%d exceed the %d-pixel per-axis automatic-processing limit", width, height, maxImageDimensionPixels))
	}
	if pixels > decodeReviewThresholdPixels {
		*warnings = append(*warnings, fmt.Sprintf("image pixel count %d exceeds the %d-pixel automatic-processing limit", pixels, decodeReviewThresholdPixels))
	}
}
