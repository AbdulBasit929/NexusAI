package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxMP3Frames               = 1_000_000
	maxMP3FrameBytes           = 4096
	maxISOBMFFBoxes            = 100_000
	maxISOBMFFDepth            = 12
	maxISOBMFFTracks           = 4096
	maxISOBMFFSampleEntries    = 256
	maxISOBMFFCompatibleBrands = 64
	maxMPEGHeaderBytes         = 8 << 20
)

type boundedMPEGReader struct {
	file      *os.File
	fileSize  int64
	bytesRead int64
}

func (reader *boundedMPEGReader) readAt(buffer []byte, offset int64) error {
	if offset < 0 || int64(len(buffer)) > reader.fileSize-offset {
		return io.ErrUnexpectedEOF
	}
	if int64(len(buffer)) > maxMPEGHeaderBytes-reader.bytesRead {
		return fmt.Errorf("technical header reads exceed the %d-byte limit", maxMPEGHeaderBytes)
	}
	if _, err := reader.file.ReadAt(buffer, offset); err != nil {
		return err
	}
	reader.bytesRead += int64(len(buffer))
	return nil
}

type mp3FrameHeader struct {
	version      string
	layer        int
	bitrateKbps  int
	sampleRateHz int
	frameBytes   int
	samples      int
	channels     int
	channelMode  string
	crcProtected bool
}

func extractMP3Metadata(path string) (map[string]any, []string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, nil, fmt.Errorf("open MP3: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("stat MP3: %w", err)
	}
	if info.Size() < 4 {
		return nil, nil, errors.New("file is shorter than an MPEG audio frame header")
	}
	reader := &boundedMPEGReader{file: file, fileSize: info.Size()}
	position := int64(0)
	metadata := mediaMetadataBase("mp3")
	warnings := []string{}

	var id3Marker [3]byte
	if err := reader.readAt(id3Marker[:], 0); err != nil {
		return nil, nil, err
	}
	if string(id3Marker[:]) == "ID3" {
		var prefix [10]byte
		copy(prefix[:3], id3Marker[:])
		if err := reader.readAt(prefix[3:], 3); err != nil {
			return nil, nil, err
		}
		major, revision := prefix[3], prefix[4]
		if major < 2 || major > 4 || revision == 0xff {
			return nil, nil, fmt.Errorf("unsupported ID3v2 version %d.%d", major, revision)
		}
		if prefix[6]&0x80 != 0 || prefix[7]&0x80 != 0 || prefix[8]&0x80 != 0 || prefix[9]&0x80 != 0 {
			return nil, nil, errors.New("ID3v2 tag size is not sync-safe")
		}
		reservedMask := byte(0x3f)
		if major == 3 {
			reservedMask = 0x1f
		} else if major == 4 {
			reservedMask = 0x0f
		}
		if prefix[5]&reservedMask != 0 {
			return nil, nil, errors.New("ID3v2 header has reserved flags set")
		}
		tagPayloadBytes := int64(prefix[6])<<21 | int64(prefix[7])<<14 | int64(prefix[8])<<7 | int64(prefix[9])
		footerBytes := int64(0)
		if major == 4 && prefix[5]&0x10 != 0 {
			footerBytes = 10
		}
		position = 10 + tagPayloadBytes + footerBytes
		if position > info.Size()-4 {
			return nil, nil, errors.New("ID3v2 tag exceeds the available file bytes")
		}
		metadata["id3v2_present"] = true
		metadata["id3v2_version"] = fmt.Sprintf("2.%d.%d", major, revision)
		metadata["id3v2_total_bytes"] = position
		metadata["id3_tag_values_read"] = false
		metadata["id3_tag_values_retained"] = false
	}
	if _, found := metadata["id3v2_present"]; !found {
		metadata["id3v2_present"] = false
		metadata["id3_tag_values_read"] = false
		metadata["id3_tag_values_retained"] = false
	}

	var first mp3FrameHeader
	var frameCount, crcProtectedCount uint64
	var totalFrameBytes, totalSamples uint64
	minBitrate, maxBitrate := 0, 0
	minChannels, maxChannels := 0, 0
	channelModes := map[string]struct{}{}
	id3v1Present := false

	for position < info.Size() {
		remaining := info.Size() - position
		if remaining == 128 {
			var marker [3]byte
			if err := reader.readAt(marker[:], position); err != nil {
				return nil, nil, err
			}
			if string(marker[:]) == "TAG" {
				id3v1Present = true
				position = info.Size()
				break
			}
		}
		if frameCount >= maxMP3Frames {
			return nil, nil, fmt.Errorf("MP3 frame count exceeds the inventory limit of %d", maxMP3Frames)
		}
		if remaining < 4 {
			return nil, nil, fmt.Errorf("trailing %d bytes do not form an MPEG audio frame", remaining)
		}
		var raw [4]byte
		if err := reader.readAt(raw[:], position); err != nil {
			return nil, nil, fmt.Errorf("read MPEG audio frame %d: %w", frameCount+1, err)
		}
		header, err := parseMP3FrameHeader(raw[:])
		if err != nil {
			return nil, nil, fmt.Errorf("invalid MPEG audio frame %d at byte %d: %w", frameCount+1, position, err)
		}
		if int64(header.frameBytes) > remaining {
			return nil, nil, fmt.Errorf("MPEG audio frame %d exceeds the available file bytes", frameCount+1)
		}
		if frameCount == 0 {
			first = header
			minBitrate, maxBitrate = header.bitrateKbps, header.bitrateKbps
			minChannels, maxChannels = header.channels, header.channels
		} else if header.version != first.version || header.layer != first.layer || header.sampleRateHz != first.sampleRateHz {
			return nil, nil, fmt.Errorf("MPEG audio stream parameters change at frame %d", frameCount+1)
		}
		minBitrate = min(minBitrate, header.bitrateKbps)
		maxBitrate = max(maxBitrate, header.bitrateKbps)
		minChannels = min(minChannels, header.channels)
		maxChannels = max(maxChannels, header.channels)
		channelModes[header.channelMode] = struct{}{}
		if header.crcProtected {
			crcProtectedCount++
		}
		frameCount++
		totalFrameBytes += uint64(header.frameBytes)
		totalSamples += uint64(header.samples)
		position += int64(header.frameBytes)
	}
	if frameCount == 0 {
		return nil, nil, errors.New("no MPEG audio frames were found")
	}

	durationMillis, known := mediaDurationMillis(totalSamples, uint64(first.sampleRateHz))
	if !known {
		return nil, nil, errors.New("MPEG audio duration exceeds the supported integer range")
	}
	modes := make([]string, 0, len(channelModes))
	for mode := range channelModes {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	metadata["mpeg_version"] = first.version
	metadata["mpeg_layer"] = first.layer
	metadata["sample_rate_hz"] = first.sampleRateHz
	metadata["frame_count"] = frameCount
	metadata["encoded_audio_bytes"] = totalFrameBytes
	metadata["total_samples_per_channel"] = totalSamples
	metadata["duration_ms"] = durationMillis
	metadata["minimum_bitrate_kbps"] = minBitrate
	metadata["maximum_bitrate_kbps"] = maxBitrate
	metadata["bitrate_mode"] = map[bool]string{true: "constant", false: "variable"}[minBitrate == maxBitrate]
	metadata["minimum_channels"] = minChannels
	metadata["maximum_channels"] = maxChannels
	metadata["channel_modes"] = modes
	if minChannels == maxChannels {
		metadata["channels"] = minChannels
	} else {
		warnings = append(warnings, "MP3 channel mode changes within the stream")
	}
	metadata["crc_protected_frame_count"] = crcProtectedCount
	metadata["id3v1_present"] = id3v1Present
	metadata["frame_inventory_complete"] = true
	metadata["file_size_bytes"] = info.Size()
	metadata["header_bytes_inspected"] = reader.bytesRead
	metadata["compressed_audio_payload_bytes_read"] = 0
	metadata["audio_decoded"] = false
	metadata["transcript_generated"] = false
	return metadata, warnings, nil
}

func parseMP3FrameHeader(raw []byte) (mp3FrameHeader, error) {
	value := binary.BigEndian.Uint32(raw)
	if value&0xffe00000 != 0xffe00000 {
		return mp3FrameHeader{}, errors.New("frame sync is absent")
	}
	versionBits := (value >> 19) & 0x3
	version := ""
	sampleRateDivisor := 1
	switch versionBits {
	case 0:
		version = "2.5"
		sampleRateDivisor = 4
	case 2:
		version = "2"
		sampleRateDivisor = 2
	case 3:
		version = "1"
	default:
		return mp3FrameHeader{}, errors.New("reserved MPEG version")
	}
	layerBits := (value >> 17) & 0x3
	layer := 0
	switch layerBits {
	case 1:
		layer = 3
	case 2:
		layer = 2
	case 3:
		layer = 1
	default:
		return mp3FrameHeader{}, errors.New("reserved MPEG layer")
	}
	bitrateIndex := int((value >> 12) & 0xf)
	if bitrateIndex == 0 {
		return mp3FrameHeader{}, errors.New("free-format bitrate is not supported by bounded inventory")
	}
	if bitrateIndex == 15 {
		return mp3FrameHeader{}, errors.New("invalid bitrate index")
	}
	bitrate := mp3Bitrate(version, layer, bitrateIndex)
	sampleRateIndex := int((value >> 10) & 0x3)
	if sampleRateIndex == 3 {
		return mp3FrameHeader{}, errors.New("invalid sample-rate index")
	}
	sampleRate := []int{44100, 48000, 32000}[sampleRateIndex] / sampleRateDivisor
	padding := int((value >> 9) & 1)
	frameBytes, samples := 0, 0
	switch layer {
	case 1:
		frameBytes = (12*bitrate*1000/sampleRate + padding) * 4
		samples = 384
	case 2:
		frameBytes = 144*bitrate*1000/sampleRate + padding
		samples = 1152
	case 3:
		coefficient := 144
		samples = 1152
		if version != "1" {
			coefficient = 72
			samples = 576
		}
		frameBytes = coefficient*bitrate*1000/sampleRate + padding
	}
	if frameBytes < 4 || frameBytes > maxMP3FrameBytes {
		return mp3FrameHeader{}, fmt.Errorf("computed frame length %d is outside the supported range", frameBytes)
	}
	channelModeBits := int((value >> 6) & 0x3)
	channelModes := []string{"stereo", "joint_stereo", "dual_channel", "mono"}
	channels := 2
	if channelModeBits == 3 {
		channels = 1
	}
	return mp3FrameHeader{
		version: version, layer: layer, bitrateKbps: bitrate, sampleRateHz: sampleRate,
		frameBytes: frameBytes, samples: samples, channels: channels,
		channelMode: channelModes[channelModeBits], crcProtected: value&(1<<16) == 0,
	}, nil
}

func mp3Bitrate(version string, layer, index int) int {
	if version == "1" {
		switch layer {
		case 1:
			return []int{0, 32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448}[index]
		case 2:
			return []int{0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384}[index]
		default:
			return []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}[index]
		}
	}
	if layer == 1 {
		return []int{0, 32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256}[index]
	}
	return []int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160}[index]
}

type isoBox struct {
	offset       int64
	size         int64
	headerBytes  int64
	payloadStart int64
	payloadEnd   int64
	typ          string
}

type isoSampleEntry struct {
	offset int64
	size   int64
	codec  string
}

type isoTrack struct {
	id              uint32
	handler         string
	timescale       uint32
	duration        uint64
	durationKnown   bool
	widthFixed      uint32
	heightFixed     uint32
	rotationDegrees int
	rotationKnown   bool
	enabled         bool
	sampleEntries   []isoSampleEntry
}

type isoInventory struct {
	reader             *boundedMPEGReader
	boxCount           int
	topLevelTypes      map[string]struct{}
	compatibleBrands   []string
	majorBrand         string
	minorVersion       uint32
	ftypCount          int
	moovCount          int
	mdatCount          int
	mdatPayloadBytes   uint64
	movieTimescale     uint32
	movieDuration      uint64
	movieDurationKnown bool
	mvhdCount          int
	tracks             []*isoTrack
	warnings           []string
}

func isISOBMFFExtension(extension string) bool {
	switch extension {
	case ".mp4", ".m4a", ".m4v", ".mov", ".3gp":
		return true
	default:
		return false
	}
}

func extractISOBMFFMetadata(path, extension string) (map[string]any, []string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, nil, fmt.Errorf("open ISO Base Media file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("stat ISO Base Media file: %w", err)
	}
	if info.Size() < 8 {
		return nil, nil, errors.New("file is shorter than an ISO Base Media box header")
	}
	inventory := &isoInventory{
		reader:        &boundedMPEGReader{file: file, fileSize: info.Size()},
		topLevelTypes: map[string]struct{}{},
	}
	if err := inventory.scanBoxes(0, info.Size(), 0, nil); err != nil {
		return nil, nil, err
	}
	if inventory.moovCount != 1 {
		return nil, nil, fmt.Errorf("expected exactly one movie box, found %d", inventory.moovCount)
	}
	if inventory.mvhdCount != 1 {
		return nil, nil, fmt.Errorf("expected exactly one movie header, found %d", inventory.mvhdCount)
	}
	if inventory.ftypCount == 0 {
		inventory.warnings = append(inventory.warnings, "ISO Base Media file has no file-type box; accepted as legacy QuickTime-compatible structure")
	} else if inventory.ftypCount != 1 {
		return nil, nil, fmt.Errorf("expected at most one file-type box, found %d", inventory.ftypCount)
	}
	if inventory.mdatCount == 0 {
		inventory.warnings = append(inventory.warnings, "ISO Base Media file contains no media-data box")
	}

	trackMetadata := make([]map[string]any, 0, len(inventory.tracks))
	handlerCounts := map[string]int{}
	seenTrackIDs := map[uint32]struct{}{}
	for index, track := range inventory.tracks {
		if track.id == 0 {
			return nil, nil, fmt.Errorf("track %d has a zero track ID", index+1)
		}
		if _, duplicate := seenTrackIDs[track.id]; duplicate {
			return nil, nil, fmt.Errorf("duplicate track ID %d", track.id)
		}
		seenTrackIDs[track.id] = struct{}{}
		entryMetadata, warnings, err := inventory.describeTrack(track)
		if err != nil {
			return nil, nil, fmt.Errorf("track %d: %w", track.id, err)
		}
		inventory.warnings = append(inventory.warnings, warnings...)
		trackMetadata = append(trackMetadata, entryMetadata)
		handlerCounts[track.handler]++
	}

	format := "iso_bmff"
	if inventory.majorBrand == "qt  " {
		format = "quicktime"
		if extension != ".mov" {
			inventory.warnings = append(inventory.warnings, fmt.Sprintf("%s extension contains a QuickTime-major-brand file", extension))
		}
	}
	if extension == ".m4a" && handlerCounts["vide"] > 0 {
		inventory.warnings = append(inventory.warnings, "M4A-labelled file contains a video track")
	}
	if extension == ".m4v" && handlerCounts["vide"] == 0 {
		inventory.warnings = append(inventory.warnings, "M4V-labelled file contains no video track")
	}

	metadata := mediaMetadataBase(format)
	metadata["container_family"] = "iso_base_media"
	metadata["file_size_bytes"] = info.Size()
	metadata["major_brand"] = inventory.majorBrand
	metadata["minor_version"] = inventory.minorVersion
	metadata["compatible_brands"] = inventory.compatibleBrands
	metadata["top_level_box_types"] = sortedSet(inventory.topLevelTypes)
	metadata["box_count"] = inventory.boxCount
	metadata["track_count"] = len(inventory.tracks)
	metadata["video_track_count"] = handlerCounts["vide"]
	metadata["audio_track_count"] = handlerCounts["soun"]
	metadata["subtitle_track_count"] = handlerCounts["subt"] + handlerCounts["text"] + handlerCounts["sbtl"]
	metadata["metadata_track_count"] = handlerCounts["meta"]
	metadata["media_data_box_count"] = inventory.mdatCount
	metadata["media_data_payload_bytes"] = inventory.mdatPayloadBytes
	metadata["movie_timescale"] = inventory.movieTimescale
	metadata["movie_duration_known"] = inventory.movieDurationKnown
	if inventory.movieDurationKnown {
		durationMillis, known := mediaDurationMillis(inventory.movieDuration, uint64(inventory.movieTimescale))
		if !known {
			return nil, nil, errors.New("movie duration exceeds the supported integer range")
		}
		metadata["duration_ms"] = durationMillis
	}
	metadata["tracks"] = trackMetadata
	metadata["header_bytes_inspected"] = inventory.reader.bytesRead
	metadata["media_payload_bytes_read"] = 0
	metadata["audio_decoded"] = false
	metadata["video_decoded"] = false
	metadata["frames_extracted"] = 0
	metadata["transcript_generated"] = false
	return metadata, appendUniqueWarnings(nil, inventory.warnings...), nil
}

func (inventory *isoInventory) scanBoxes(start, end int64, depth int, track *isoTrack) error {
	if depth > maxISOBMFFDepth {
		return fmt.Errorf("ISO Base Media box nesting exceeds depth limit %d", maxISOBMFFDepth)
	}
	for position := start; position < end; {
		if inventory.boxCount >= maxISOBMFFBoxes {
			return fmt.Errorf("ISO Base Media box count exceeds limit %d", maxISOBMFFBoxes)
		}
		box, err := inventory.readBox(position, end)
		if err != nil {
			return err
		}
		inventory.boxCount++
		if depth == 0 {
			inventory.topLevelTypes[box.typ] = struct{}{}
		}
		switch box.typ {
		case "ftyp":
			if depth != 0 {
				return errors.New("file-type box is not top-level")
			}
			inventory.ftypCount++
			if err := inventory.parseFileType(box); err != nil {
				return err
			}
		case "moov":
			if depth != 0 {
				return errors.New("movie box is not top-level")
			}
			inventory.moovCount++
			if err := inventory.scanBoxes(box.payloadStart, box.payloadEnd, depth+1, nil); err != nil {
				return err
			}
		case "mvhd":
			if depth != 1 || track != nil {
				return errors.New("movie header is outside the movie box")
			}
			inventory.mvhdCount++
			if err := inventory.parseMovieHeader(box); err != nil {
				return err
			}
		case "trak":
			if depth != 1 || track != nil {
				return errors.New("track box is outside the movie box")
			}
			if len(inventory.tracks) >= maxISOBMFFTracks {
				return fmt.Errorf("track count exceeds limit %d", maxISOBMFFTracks)
			}
			newTrack := &isoTrack{}
			inventory.tracks = append(inventory.tracks, newTrack)
			if err := inventory.scanBoxes(box.payloadStart, box.payloadEnd, depth+1, newTrack); err != nil {
				return err
			}
		case "tkhd":
			if track == nil {
				return errors.New("track header is outside a track box")
			}
			if err := inventory.parseTrackHeader(box, track); err != nil {
				return err
			}
		case "mdia", "minf", "stbl":
			if track == nil {
				return fmt.Errorf("%s box is outside a track", box.typ)
			}
			if err := inventory.scanBoxes(box.payloadStart, box.payloadEnd, depth+1, track); err != nil {
				return err
			}
		case "mdhd":
			if track == nil {
				return errors.New("media header is outside a track")
			}
			if err := inventory.parseMediaHeader(box, track); err != nil {
				return err
			}
		case "hdlr":
			if track != nil {
				if err := inventory.parseHandler(box, track); err != nil {
					return err
				}
			}
		case "stsd":
			if track == nil {
				return errors.New("sample-description box is outside a track")
			}
			if err := inventory.parseSampleDescriptions(box, track); err != nil {
				return err
			}
		case "mdat":
			if depth != 0 {
				return errors.New("media-data box is not top-level")
			}
			inventory.mdatCount++
			payloadBytes := uint64(box.payloadEnd - box.payloadStart)
			if math.MaxUint64-inventory.mdatPayloadBytes < payloadBytes {
				return errors.New("media-data byte total overflows uint64")
			}
			inventory.mdatPayloadBytes += payloadBytes
		}
		position += box.size
	}
	return nil
}

func (inventory *isoInventory) readBox(offset, parentEnd int64) (isoBox, error) {
	if parentEnd-offset < 8 {
		return isoBox{}, fmt.Errorf("truncated ISO Base Media box header at byte %d", offset)
	}
	var header [32]byte
	if err := inventory.reader.readAt(header[:8], offset); err != nil {
		return isoBox{}, fmt.Errorf("read box header at byte %d: %w", offset, err)
	}
	size := int64(binary.BigEndian.Uint32(header[:4]))
	headerBytes := int64(8)
	typ := fourCC(header[4:8])
	if size == 1 {
		if parentEnd-offset < 16 {
			return isoBox{}, fmt.Errorf("truncated extended-size box at byte %d", offset)
		}
		if err := inventory.reader.readAt(header[8:16], offset+8); err != nil {
			return isoBox{}, err
		}
		extended := binary.BigEndian.Uint64(header[8:16])
		if extended > math.MaxInt64 {
			return isoBox{}, fmt.Errorf("box at byte %d exceeds supported size", offset)
		}
		size = int64(extended)
		headerBytes = 16
	} else if size == 0 {
		size = parentEnd - offset
	}
	if typ == "uuid" {
		headerBytes += 16
		if size < headerBytes || parentEnd-offset < headerBytes {
			return isoBox{}, fmt.Errorf("truncated UUID box at byte %d", offset)
		}
		if err := inventory.reader.readAt(header[16:32], offset+headerBytes-16); err != nil {
			return isoBox{}, err
		}
	}
	if size < headerBytes {
		return isoBox{}, fmt.Errorf("box %s at byte %d is shorter than its header", typ, offset)
	}
	if size > parentEnd-offset {
		return isoBox{}, fmt.Errorf("box %s at byte %d exceeds its parent boundary", typ, offset)
	}
	return isoBox{offset: offset, size: size, headerBytes: headerBytes, payloadStart: offset + headerBytes, payloadEnd: offset + size, typ: typ}, nil
}

func (inventory *isoInventory) parseFileType(box isoBox) error {
	payloadBytes := box.payloadEnd - box.payloadStart
	if payloadBytes < 8 || (payloadBytes-8)%4 != 0 {
		return errors.New("file-type box has an invalid length")
	}
	brandCount := int((payloadBytes - 8) / 4)
	if brandCount > maxISOBMFFCompatibleBrands {
		return fmt.Errorf("compatible-brand count %d exceeds limit %d", brandCount, maxISOBMFFCompatibleBrands)
	}
	payload := make([]byte, payloadBytes)
	if err := inventory.reader.readAt(payload, box.payloadStart); err != nil {
		return err
	}
	inventory.majorBrand = fourCC(payload[:4])
	inventory.minorVersion = binary.BigEndian.Uint32(payload[4:8])
	inventory.compatibleBrands = make([]string, 0, brandCount)
	for offset := 8; offset < len(payload); offset += 4 {
		inventory.compatibleBrands = append(inventory.compatibleBrands, fourCC(payload[offset:offset+4]))
	}
	return nil
}

func (inventory *isoInventory) parseMovieHeader(box isoBox) error {
	if inventory.mvhdCount > 1 {
		return errors.New("multiple movie-header boxes are not supported")
	}
	version, payload, err := inventory.readFullBoxPrefix(box, 32)
	if err != nil {
		return err
	}
	switch version {
	case 0:
		if len(payload) < 20 {
			return errors.New("version 0 movie header is truncated")
		}
		inventory.movieTimescale = binary.BigEndian.Uint32(payload[12:16])
		inventory.movieDuration = uint64(binary.BigEndian.Uint32(payload[16:20]))
		inventory.movieDurationKnown = inventory.movieDuration != math.MaxUint32
	case 1:
		if len(payload) < 32 {
			return errors.New("version 1 movie header is truncated")
		}
		inventory.movieTimescale = binary.BigEndian.Uint32(payload[20:24])
		inventory.movieDuration = binary.BigEndian.Uint64(payload[24:32])
		inventory.movieDurationKnown = inventory.movieDuration != math.MaxUint64
	default:
		return fmt.Errorf("unsupported movie-header version %d", version)
	}
	if inventory.movieTimescale == 0 {
		return errors.New("movie timescale is zero")
	}
	return nil
}

func (inventory *isoInventory) parseTrackHeader(box isoBox, track *isoTrack) error {
	if track.id != 0 {
		return errors.New("track contains multiple track-header boxes")
	}
	version, payload, err := inventory.readFullBoxPrefix(box, 96)
	if err != nil {
		return err
	}
	flags := uint32(payload[1])<<16 | uint32(payload[2])<<8 | uint32(payload[3])
	track.enabled = flags&1 != 0
	var matrixOffset, widthOffset int
	switch version {
	case 0:
		if len(payload) < 84 {
			return errors.New("version 0 track header is truncated")
		}
		track.id = binary.BigEndian.Uint32(payload[12:16])
		matrixOffset, widthOffset = 40, 76
	case 1:
		if len(payload) < 96 {
			return errors.New("version 1 track header is truncated")
		}
		track.id = binary.BigEndian.Uint32(payload[20:24])
		matrixOffset, widthOffset = 52, 88
	default:
		return fmt.Errorf("unsupported track-header version %d", version)
	}
	track.widthFixed = binary.BigEndian.Uint32(payload[widthOffset : widthOffset+4])
	track.heightFixed = binary.BigEndian.Uint32(payload[widthOffset+4 : widthOffset+8])
	track.rotationDegrees, track.rotationKnown = isoRotation(payload[matrixOffset : matrixOffset+36])
	return nil
}

func (inventory *isoInventory) parseMediaHeader(box isoBox, track *isoTrack) error {
	if track.timescale != 0 {
		return errors.New("track contains multiple media-header boxes")
	}
	version, payload, err := inventory.readFullBoxPrefix(box, 32)
	if err != nil {
		return err
	}
	switch version {
	case 0:
		if len(payload) < 20 {
			return errors.New("version 0 media header is truncated")
		}
		track.timescale = binary.BigEndian.Uint32(payload[12:16])
		track.duration = uint64(binary.BigEndian.Uint32(payload[16:20]))
		track.durationKnown = track.duration != math.MaxUint32
	case 1:
		if len(payload) < 32 {
			return errors.New("version 1 media header is truncated")
		}
		track.timescale = binary.BigEndian.Uint32(payload[20:24])
		track.duration = binary.BigEndian.Uint64(payload[24:32])
		track.durationKnown = track.duration != math.MaxUint64
	default:
		return fmt.Errorf("unsupported media-header version %d", version)
	}
	if track.timescale == 0 {
		return errors.New("media timescale is zero")
	}
	return nil
}

func (inventory *isoInventory) parseHandler(box isoBox, track *isoTrack) error {
	if track.handler != "" {
		return errors.New("track contains multiple handler boxes")
	}
	_, payload, err := inventory.readFullBoxPrefix(box, 12)
	if err != nil {
		return err
	}
	if len(payload) < 12 {
		return errors.New("handler box is truncated")
	}
	track.handler = fourCC(payload[8:12])
	return nil
}

func (inventory *isoInventory) parseSampleDescriptions(box isoBox, track *isoTrack) error {
	if len(track.sampleEntries) != 0 {
		return errors.New("track contains multiple sample-description boxes")
	}
	_, payload, err := inventory.readFullBoxPrefix(box, 8)
	if err != nil {
		return err
	}
	if len(payload) < 8 {
		return errors.New("sample-description box is truncated")
	}
	entryCount := int(binary.BigEndian.Uint32(payload[4:8]))
	if entryCount > maxISOBMFFSampleEntries {
		return fmt.Errorf("sample-entry count %d exceeds limit %d", entryCount, maxISOBMFFSampleEntries)
	}
	position := box.payloadStart + 8
	for index := 0; index < entryCount; index++ {
		if box.payloadEnd-position < 8 {
			return fmt.Errorf("sample entry %d header is truncated", index+1)
		}
		var header [8]byte
		if err := inventory.reader.readAt(header[:], position); err != nil {
			return err
		}
		size := int64(binary.BigEndian.Uint32(header[:4]))
		if size < 8 || size > box.payloadEnd-position {
			return fmt.Errorf("sample entry %d exceeds the sample-description boundary", index+1)
		}
		track.sampleEntries = append(track.sampleEntries, isoSampleEntry{offset: position, size: size, codec: fourCC(header[4:8])})
		position += size
	}
	if position != box.payloadEnd {
		return errors.New("sample-description box has unaccounted trailing bytes")
	}
	return nil
}

func (inventory *isoInventory) describeTrack(track *isoTrack) (map[string]any, []string, error) {
	if track.handler == "" {
		return nil, nil, errors.New("handler type is missing")
	}
	if track.timescale == 0 {
		return nil, nil, errors.New("media header is missing")
	}
	metadata := map[string]any{
		"track_id": track.id, "handler_type": track.handler, "enabled": track.enabled,
		"timescale": track.timescale, "duration_known": track.durationKnown,
	}
	if track.durationKnown {
		durationMillis, known := mediaDurationMillis(track.duration, uint64(track.timescale))
		if !known {
			return nil, nil, errors.New("duration exceeds the supported integer range")
		}
		metadata["duration_ms"] = durationMillis
	}
	warnings := []string{}
	if track.handler == "vide" {
		width, widthFraction := track.widthFixed>>16, track.widthFixed&0xffff
		height, heightFraction := track.heightFixed>>16, track.heightFixed&0xffff
		if width == 0 || height == 0 {
			return nil, nil, errors.New("video track has zero display dimensions")
		}
		metadata["width_pixels"] = width
		metadata["height_pixels"] = height
		if widthFraction != 0 || heightFraction != 0 {
			warnings = append(warnings, fmt.Sprintf("track %d display dimensions contain fractional fixed-point values", track.id))
		}
		if track.rotationKnown {
			metadata["rotation_degrees"] = track.rotationDegrees
			displayWidth, displayHeight := width, height
			if track.rotationDegrees == 90 || track.rotationDegrees == 270 {
				displayWidth, displayHeight = height, width
			}
			metadata["display_width_pixels"] = displayWidth
			metadata["display_height_pixels"] = displayHeight
		} else {
			metadata["rotation_known"] = false
			warnings = append(warnings, fmt.Sprintf("track %d uses a non-standard transformation matrix", track.id))
		}
	}
	codecs := make([]string, 0, len(track.sampleEntries))
	for _, entry := range track.sampleEntries {
		codecs = append(codecs, entry.codec)
		var prefix [36]byte
		readBytes := min(int64(len(prefix)), entry.size)
		if readBytes < 8 {
			return nil, nil, errors.New("sample entry is shorter than its header")
		}
		if err := inventory.reader.readAt(prefix[:readBytes], entry.offset); err != nil {
			return nil, nil, err
		}
		if track.handler == "soun" {
			if entry.size < 36 {
				return nil, nil, fmt.Errorf("audio sample entry %s is truncated", entry.codec)
			}
			version := binary.BigEndian.Uint16(prefix[16:18])
			if version > 1 {
				warnings = append(warnings, fmt.Sprintf("track %d audio sample-entry version %d needs deeper codec-specific inspection", track.id, version))
				continue
			}
			channels := binary.BigEndian.Uint16(prefix[24:26])
			sampleSize := binary.BigEndian.Uint16(prefix[26:28])
			sampleRateFixed := binary.BigEndian.Uint32(prefix[32:36])
			if channels == 0 || sampleRateFixed>>16 == 0 {
				return nil, nil, fmt.Errorf("audio sample entry %s has zero channels or sample rate", entry.codec)
			}
			metadata["channels"] = channels
			metadata["sample_size_bits"] = sampleSize
			metadata["sample_rate_hz"] = sampleRateFixed >> 16
			if sampleRateFixed&0xffff != 0 {
				warnings = append(warnings, fmt.Sprintf("track %d sample rate contains a fractional fixed-point value", track.id))
			}
		}
	}
	metadata["sample_entry_count"] = len(track.sampleEntries)
	metadata["codecs"] = uniqueSortedStrings(codecs)
	return metadata, warnings, nil
}

func (inventory *isoInventory) readFullBoxPrefix(box isoBox, maximum int64) (byte, []byte, error) {
	payloadBytes := box.payloadEnd - box.payloadStart
	readBytes := min(payloadBytes, maximum)
	if readBytes < 4 {
		return 0, nil, fmt.Errorf("box %s is shorter than a full-box header", box.typ)
	}
	payload := make([]byte, readBytes)
	if err := inventory.reader.readAt(payload, box.payloadStart); err != nil {
		return 0, nil, err
	}
	return payload[0], payload, nil
}

func isoRotation(matrix []byte) (int, bool) {
	if len(matrix) < 36 {
		return 0, false
	}
	a := int32(binary.BigEndian.Uint32(matrix[0:4]))
	b := int32(binary.BigEndian.Uint32(matrix[4:8]))
	c := int32(binary.BigEndian.Uint32(matrix[12:16]))
	d := int32(binary.BigEndian.Uint32(matrix[16:20]))
	switch {
	case a == 0x10000 && b == 0 && c == 0 && d == 0x10000:
		return 0, true
	case a == 0 && b == 0x10000 && c == -0x10000 && d == 0:
		return 90, true
	case a == -0x10000 && b == 0 && c == 0 && d == -0x10000:
		return 180, true
	case a == 0 && b == -0x10000 && c == 0x10000 && d == 0:
		return 270, true
	default:
		return 0, false
	}
}

func mediaDurationMillis(units, timescale uint64) (int64, bool) {
	if timescale == 0 {
		return 0, false
	}
	seconds := units / timescale
	remainder := units % timescale
	if seconds > math.MaxInt64/1000 {
		return 0, false
	}
	milliseconds := seconds*1000 + remainder*1000/timescale
	if milliseconds > math.MaxInt64 {
		return 0, false
	}
	return int64(milliseconds), true
}

func fourCC(value []byte) string {
	printable := true
	for _, character := range value {
		if character < 0x20 || character > 0x7e {
			printable = false
			break
		}
	}
	if printable {
		return string(value)
	}
	return fmt.Sprintf("0x%x", value)
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func uniqueSortedStrings(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			set[value] = struct{}{}
		}
	}
	return sortedSet(set)
}
