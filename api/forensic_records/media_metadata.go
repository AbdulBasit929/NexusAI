package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	deterministicMediaExtractorID      = "nexusai_deterministic_media_metadata"
	deterministicMediaExtractorVersion = "1.5.0"
	maxImageConfigBytes                = 4 << 20
	maxMediaHeaderChunks               = 512
	imageIntakeContractVersion         = "forensics.image-intake/v1"
	maxImageEvidenceBytes              = int64(64 << 20)
	maxImageDimensionPixels            = 32_768
)

func extractDeterministicMediaMetadata(path, sourceFile string, classification evidenceClassification) (map[string]any, []string) {
	extension := strings.ToLower(filepath.Ext(sourceFile))
	switch {
	case classification.Modality == "image" && isDeterministicImageExtension(extension):
		metadata, warnings, err := extractImageMetadata(path, extension)
		if err != nil {
			return imageIntakeFailureMetadata(path, extension, err), []string{"image intake requires manual review: " + err.Error()}
		}
		return metadata, warnings
	case classification.Modality == "image":
		return imageIntakeFailureMetadata(path, extension, fmt.Errorf("format is not enabled for bounded raster inspection")), []string{"image intake requires manual review: the supplied format is preserved but is not enabled for bounded raster inspection"}
	case classification.Modality == "audio" && extension == ".wav":
		metadata, err := extractWAVMetadata(path)
		if err != nil {
			return map[string]any{}, []string{"deterministic WAV metadata unavailable: " + err.Error()}
		}
		return metadata, nil
	case classification.Modality == "audio" && extension == ".flac":
		metadata, err := extractFLACMetadata(path)
		if err != nil {
			return map[string]any{}, []string{"deterministic FLAC metadata unavailable: " + err.Error()}
		}
		return metadata, nil
	case classification.Modality == "audio" && extension == ".mp3":
		metadata, warnings, err := extractMP3Metadata(path)
		if err != nil {
			return map[string]any{}, []string{"deterministic MP3 metadata unavailable: " + err.Error()}
		}
		return metadata, warnings
	case (classification.Modality == "audio" || classification.Modality == "video") && isISOBMFFExtension(extension):
		metadata, warnings, err := extractISOBMFFMetadata(path, extension)
		if err != nil {
			return map[string]any{}, []string{"deterministic MP4/MOV metadata unavailable: " + err.Error()}
		}
		return metadata, warnings
	case classification.Modality == "document" && extension == ".pdf":
		metadata, warnings, err := extractPDFMetadata(path)
		if err != nil {
			return map[string]any{}, []string{"deterministic PDF inventory unavailable: " + err.Error()}
		}
		return metadata, warnings
	case classification.Modality == "container" && (extension == ".zip" || extension == ".tar"):
		metadata, warnings, err := extractArchiveMetadata(path, extension)
		if err != nil {
			return map[string]any{}, []string{"deterministic archive inventory unavailable: " + err.Error()}
		}
		return metadata, warnings
	case classification.Modality == "database" && (extension == ".sqlite" || extension == ".sqlite3" || extension == ".db"):
		metadata, warnings, err := extractSQLiteMetadata(path)
		if err != nil {
			return map[string]any{}, []string{"deterministic SQLite inventory unavailable: " + err.Error()}
		}
		return metadata, warnings
	case classification.Modality == "network_or_system_capture" && (extension == ".pcap" || extension == ".pcapng" || extension == ".cap"):
		metadata, warnings, err := extractCaptureMetadata(path, extension)
		if err != nil {
			return map[string]any{}, []string{"deterministic packet-capture inventory unavailable: " + err.Error()}
		}
		return metadata, warnings
	default:
		return map[string]any{}, nil
	}
}

func mediaMetadataBase(format string) map[string]any {
	return map[string]any{
		"extractor_id":      deterministicMediaExtractorID,
		"extractor_version": deterministicMediaExtractorVersion,
		"format":            format,
		"deterministic":     true,
	}
}

func extractStandardImageMetadata(path string) (map[string]any, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open image: %w", err)
	}
	defer file.Close()

	config, format, err := image.DecodeConfig(io.LimitReader(file, maxImageConfigBytes))
	if err != nil {
		return nil, fmt.Errorf("decode bounded image header: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return nil, fmt.Errorf("invalid image dimensions %dx%d", config.Width, config.Height)
	}
	metadata := mediaMetadataBase(format)
	metadata["width_pixels"] = config.Width
	metadata["height_pixels"] = config.Height
	metadata["color_model"] = deterministicColorModel(config.ColorModel)
	return metadata, nil
}

func deterministicColorModel(model color.Model) string {
	switch model {
	case color.GrayModel, color.Gray16Model:
		return "grayscale"
	case color.RGBAModel, color.RGBA64Model, color.NRGBAModel, color.NRGBA64Model:
		return "rgb"
	case color.YCbCrModel:
		return "ycbcr"
	case color.CMYKModel:
		return "cmyk"
	default:
		if _, ok := model.(color.Palette); ok {
			return "indexed_palette"
		}
		return "unspecified"
	}
}

func extractWAVMetadata(path string) (map[string]any, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open WAV: %w", err)
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat WAV: %w", err)
	}

	var header [12]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return nil, fmt.Errorf("read RIFF header: %w", err)
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a RIFF/WAVE file")
	}
	declaredEnd := int64(binary.LittleEndian.Uint32(header[4:8])) + 8
	if declaredEnd < int64(len(header)) || declaredEnd > fileInfo.Size() {
		return nil, fmt.Errorf("RIFF size exceeds the available file bytes")
	}

	var (
		audioFormat   uint16
		channels      uint16
		sampleRate    uint32
		byteRate      uint32
		bitsPerSample uint16
		dataBytes     uint32
		foundFormat   bool
		foundData     bool
	)
	for chunkCount := 0; chunkCount < maxMediaHeaderChunks; chunkCount++ {
		position, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("inspect WAV position: %w", err)
		}
		if position == declaredEnd {
			break
		}
		if declaredEnd-position < 8 {
			return nil, fmt.Errorf("truncated WAV chunk header")
		}
		var chunkHeader [8]byte
		if _, err := io.ReadFull(file, chunkHeader[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, fmt.Errorf("read WAV chunk header: %w", err)
		}
		chunkID := string(chunkHeader[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])
		paddedChunkSize := int64(chunkSize) + (int64(chunkSize) & 1)
		position, err = file.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("inspect WAV chunk position: %w", err)
		}
		if paddedChunkSize > declaredEnd-position {
			return nil, fmt.Errorf("WAV %q chunk exceeds the declared RIFF boundary", chunkID)
		}
		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return nil, fmt.Errorf("fmt chunk is shorter than 16 bytes")
			}
			var format [16]byte
			if _, err := io.ReadFull(file, format[:]); err != nil {
				return nil, fmt.Errorf("read fmt chunk: %w", err)
			}
			audioFormat = binary.LittleEndian.Uint16(format[0:2])
			channels = binary.LittleEndian.Uint16(format[2:4])
			sampleRate = binary.LittleEndian.Uint32(format[4:8])
			byteRate = binary.LittleEndian.Uint32(format[8:12])
			bitsPerSample = binary.LittleEndian.Uint16(format[14:16])
			if err := skipWAVChunkRemainder(file, chunkSize-16); err != nil {
				return nil, err
			}
			foundFormat = true
		case "data":
			dataBytes = chunkSize
			foundData = true
			if _, err := file.Seek(int64(chunkSize)+(int64(chunkSize)&1), io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("skip WAV data chunk: %w", err)
			}
		default:
			if _, err := file.Seek(int64(chunkSize)+(int64(chunkSize)&1), io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("skip WAV %q chunk: %w", chunkID, err)
			}
		}
		if foundFormat && foundData {
			break
		}
	}
	if !foundFormat || !foundData {
		return nil, fmt.Errorf("required fmt or data chunk is missing")
	}
	if channels == 0 || sampleRate == 0 || byteRate == 0 || bitsPerSample == 0 {
		return nil, fmt.Errorf("invalid zero-valued WAV format field")
	}

	metadata := mediaMetadataBase("wav")
	metadata["audio_format_code"] = audioFormat
	metadata["channels"] = channels
	metadata["sample_rate_hz"] = sampleRate
	metadata["byte_rate"] = byteRate
	metadata["bits_per_sample"] = bitsPerSample
	metadata["data_bytes"] = dataBytes
	metadata["duration_ms"] = int64(dataBytes) * 1000 / int64(byteRate)
	return metadata, nil
}

func extractFLACMetadata(path string) (map[string]any, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open FLAC: %w", err)
	}
	defer file.Close()

	var header [42]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return nil, fmt.Errorf("read FLAC STREAMINFO header: %w", err)
	}
	if string(header[0:4]) != "fLaC" {
		return nil, fmt.Errorf("not a native FLAC stream")
	}
	if header[4]&0x7f != 0 {
		return nil, fmt.Errorf("first FLAC metadata block is not STREAMINFO")
	}
	blockLength := uint32(header[5])<<16 | uint32(header[6])<<8 | uint32(header[7])
	if blockLength != 34 {
		return nil, fmt.Errorf("FLAC STREAMINFO length is %d, expected 34", blockLength)
	}

	streamInfo := header[8:]
	minimumBlockSamples := binary.BigEndian.Uint16(streamInfo[0:2])
	maximumBlockSamples := binary.BigEndian.Uint16(streamInfo[2:4])
	minimumFrameBytes := readBigEndianUint24(streamInfo[4:7])
	maximumFrameBytes := readBigEndianUint24(streamInfo[7:10])
	packed := binary.BigEndian.Uint64(streamInfo[10:18])
	sampleRate := uint32(packed >> 44)
	channels := uint8((packed>>41)&0x7) + 1
	bitsPerSample := uint8((packed>>36)&0x1f) + 1
	totalSamples := packed & ((uint64(1) << 36) - 1)
	if sampleRate == 0 {
		return nil, fmt.Errorf("zero sample rate cannot produce audio duration metadata")
	}
	if bitsPerSample < 4 || bitsPerSample > 32 {
		return nil, fmt.Errorf("invalid FLAC bit depth %d", bitsPerSample)
	}
	if minimumBlockSamples < 16 || maximumBlockSamples < minimumBlockSamples {
		return nil, fmt.Errorf("invalid FLAC block-size range %d-%d", minimumBlockSamples, maximumBlockSamples)
	}
	if minimumFrameBytes != 0 && maximumFrameBytes != 0 && maximumFrameBytes < minimumFrameBytes {
		return nil, fmt.Errorf("invalid FLAC frame-size range %d-%d", minimumFrameBytes, maximumFrameBytes)
	}

	metadata := mediaMetadataBase("flac")
	metadata["minimum_block_samples"] = minimumBlockSamples
	metadata["maximum_block_samples"] = maximumBlockSamples
	metadata["minimum_frame_bytes"] = minimumFrameBytes
	metadata["maximum_frame_bytes"] = maximumFrameBytes
	metadata["sample_rate_hz"] = sampleRate
	metadata["channels"] = channels
	metadata["bits_per_sample"] = bitsPerSample
	metadata["total_samples"] = totalSamples
	metadata["duration_known"] = totalSamples != 0
	if totalSamples != 0 {
		metadata["duration_ms"] = int64(totalSamples * 1000 / uint64(sampleRate))
	}
	return metadata, nil
}

func readBigEndianUint24(value []byte) uint32 {
	return uint32(value[0])<<16 | uint32(value[1])<<8 | uint32(value[2])
}

func skipWAVChunkRemainder(file *os.File, remaining uint32) error {
	skip := int64(remaining) + (int64(remaining) & 1)
	if _, err := file.Seek(skip, io.SeekCurrent); err != nil {
		return fmt.Errorf("skip WAV fmt remainder: %w", err)
	}
	return nil
}
