package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deterministic MPEG media inventory", func() {
	It("inventories bounded constant-bitrate MP3 frames without reading tags or audio payloads", func() {
		path := filepath.Join(GinkgoT().TempDir(), "pakistan_interview.mp3")
		payload := mp3Fixture([]byte{9, 9, 9}, true, true)
		Expect(os.WriteFile(path, payload, 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "pakistan_interview.mp3", evidenceClassification{Modality: "audio"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("extractor_version", "1.5.0"))
		Expect(metadata).To(HaveKeyWithValue("format", "mp3"))
		Expect(metadata).To(HaveKeyWithValue("mpeg_version", "1"))
		Expect(metadata).To(HaveKeyWithValue("mpeg_layer", 3))
		Expect(metadata).To(HaveKeyWithValue("sample_rate_hz", 44100))
		Expect(metadata).To(HaveKeyWithValue("channels", 2))
		Expect(metadata).To(HaveKeyWithValue("frame_count", uint64(3)))
		Expect(metadata).To(HaveKeyWithValue("duration_ms", int64(78)))
		Expect(metadata).To(HaveKeyWithValue("bitrate_mode", "constant"))
		Expect(metadata).To(HaveKeyWithValue("minimum_bitrate_kbps", 128))
		Expect(metadata).To(HaveKeyWithValue("maximum_bitrate_kbps", 128))
		Expect(metadata).To(HaveKeyWithValue("id3v2_present", true))
		Expect(metadata).To(HaveKeyWithValue("id3v1_present", true))
		Expect(metadata).To(HaveKeyWithValue("id3_tag_values_read", false))
		Expect(metadata).To(HaveKeyWithValue("compressed_audio_payload_bytes_read", 0))
		Expect(metadata).To(HaveKeyWithValue("audio_decoded", false))
		Expect(fmt.Sprint(metadata)).NotTo(ContainSubstring("PRIVATE-TAG-VALUE"))
	})

	It("accounts for variable-bitrate MP3 frames exactly", func() {
		path := filepath.Join(GinkgoT().TempDir(), "variable.mp3")
		Expect(os.WriteFile(path, mp3Fixture([]byte{9, 10, 9}, false, false), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "variable.mp3", evidenceClassification{Modality: "audio"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("frame_count", uint64(3)))
		Expect(metadata).To(HaveKeyWithValue("bitrate_mode", "variable"))
		Expect(metadata).To(HaveKeyWithValue("minimum_bitrate_kbps", 128))
		Expect(metadata).To(HaveKeyWithValue("maximum_bitrate_kbps", 160))
		Expect(metadata).To(HaveKeyWithValue("frame_inventory_complete", true))
	})

	It("inventories MP4 video and audio tracks, codecs, duration, dimensions, and rotation", func() {
		path := filepath.Join(GinkgoT().TempDir(), "cctv.mp4")
		Expect(os.WriteFile(path, isoMediaFixture("isom", true, true, 90), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "cctv.mp4", evidenceClassification{Modality: "video"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("extractor_version", "1.5.0"))
		Expect(metadata).To(HaveKeyWithValue("format", "iso_bmff"))
		Expect(metadata).To(HaveKeyWithValue("major_brand", "isom"))
		Expect(metadata).To(HaveKeyWithValue("duration_ms", int64(5000)))
		Expect(metadata).To(HaveKeyWithValue("track_count", 2))
		Expect(metadata).To(HaveKeyWithValue("video_track_count", 1))
		Expect(metadata).To(HaveKeyWithValue("audio_track_count", 1))
		Expect(metadata).To(HaveKeyWithValue("media_data_payload_bytes", uint64(32)))
		Expect(metadata).To(HaveKeyWithValue("media_payload_bytes_read", 0))
		Expect(metadata).To(HaveKeyWithValue("video_decoded", false))
		Expect(metadata).To(HaveKeyWithValue("frames_extracted", 0))

		tracks, ok := metadata["tracks"].([]map[string]any)
		Expect(ok).To(BeTrue())
		Expect(tracks).To(HaveLen(2))
		Expect(tracks[0]).To(HaveKeyWithValue("handler_type", "vide"))
		Expect(tracks[0]).To(HaveKeyWithValue("width_pixels", uint32(1920)))
		Expect(tracks[0]).To(HaveKeyWithValue("height_pixels", uint32(1080)))
		Expect(tracks[0]).To(HaveKeyWithValue("rotation_degrees", 90))
		Expect(tracks[0]).To(HaveKeyWithValue("display_width_pixels", uint32(1080)))
		Expect(tracks[0]).To(HaveKeyWithValue("display_height_pixels", uint32(1920)))
		Expect(tracks[0]).To(HaveKeyWithValue("codecs", []string{"avc1"}))
		Expect(tracks[1]).To(HaveKeyWithValue("handler_type", "soun"))
		Expect(tracks[1]).To(HaveKeyWithValue("channels", uint16(2)))
		Expect(tracks[1]).To(HaveKeyWithValue("sample_rate_hz", uint32(48000)))
		Expect(tracks[1]).To(HaveKeyWithValue("codecs", []string{"mp4a"}))
	})

	It("supports audio-only M4A inventory through the same ISO Base Media parser", func() {
		path := filepath.Join(GinkgoT().TempDir(), "call.m4a")
		Expect(os.WriteFile(path, isoMediaFixture("M4A ", false, true, 0), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "call.m4a", evidenceClassification{Modality: "audio"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("track_count", 1))
		Expect(metadata).To(HaveKeyWithValue("audio_track_count", 1))
		Expect(metadata).To(HaveKeyWithValue("video_track_count", 0))
	})

	It("flags QuickTime content carried under an MP4 extension", func() {
		path := filepath.Join(GinkgoT().TempDir(), "mislabelled.mp4")
		Expect(os.WriteFile(path, isoMediaFixture("qt  ", true, false, 0), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "mislabelled.mp4", evidenceClassification{Modality: "video"})
		Expect(metadata).To(HaveKeyWithValue("format", "quicktime"))
		Expect(warnings).To(ConsistOf(ContainSubstring("extension contains a QuickTime-major-brand file")))
	})

	DescribeTable("rejects malformed or misleading MPEG containers with bounded warnings",
		func(name string, payload []byte, modality, warning string) {
			path := filepath.Join(GinkgoT().TempDir(), name)
			Expect(os.WriteFile(path, payload, 0o600)).To(Succeed())
			metadata, warnings := extractDeterministicMediaMetadata(path, name, evidenceClassification{Modality: modality})
			Expect(metadata).To(BeEmpty())
			Expect(warnings).To(ConsistOf(ContainSubstring(warning)))
		},
		Entry("free-format MP3", "free-format.mp3", mp3InvalidFrameFixture(0), "audio", "free-format bitrate"),
		Entry("truncated MP3 frame", "truncated.mp3", mp3InvalidFrameFixture(9), "audio", "exceeds the available file bytes"),
		Entry("oversized ID3 tag", "bad-id3.mp3", oversizedID3Fixture(), "audio", "ID3v2 tag exceeds"),
		Entry("MP4 box beyond file", "truncated.mp4", oversizedISOBoxFixture(), "video", "exceeds its parent boundary"),
		Entry("sample-entry amplification", "many-entries.mp4", excessiveSampleEntriesFixture(), "video", "sample-entry count 257 exceeds limit 256"),
		Entry("MP3 disguised as MP4", "disguised.mp4", mp3Fixture([]byte{9}, false, false), "video", "deterministic MP4/MOV metadata unavailable"),
	)
})

func mp3Fixture(bitrateIndices []byte, withID3v2, withID3v1 bool) []byte {
	buffer := &bytes.Buffer{}
	if withID3v2 {
		value := []byte("PRIVATE-TAG-VALUE")
		buffer.Write([]byte{'I', 'D', '3', 4, 0, 0})
		buffer.Write(syncSafeInteger(len(value)))
		buffer.Write(value)
	}
	for _, bitrateIndex := range bitrateIndices {
		header := []byte{0xff, 0xfb, bitrateIndex << 4, 0x00}
		parsed, err := parseMP3FrameHeader(header)
		Expect(err).NotTo(HaveOccurred())
		buffer.Write(header)
		buffer.Write(make([]byte, parsed.frameBytes-4))
	}
	if withID3v1 {
		tag := make([]byte, 128)
		copy(tag[:], []byte("TAGPRIVATE-TAG-VALUE"))
		buffer.Write(tag)
	}
	return buffer.Bytes()
}

func mp3InvalidFrameFixture(bitrateIndex byte) []byte {
	header := []byte{0xff, 0xfb, bitrateIndex << 4, 0x00}
	if bitrateIndex == 0 {
		return header
	}
	return append(header, make([]byte, 8)...)
}

func oversizedID3Fixture() []byte {
	return []byte{'I', 'D', '3', 4, 0, 0, 0x7f, 0x7f, 0x7f, 0x7f, 0xff, 0xfb, 0x90, 0x00}
}

func syncSafeInteger(value int) []byte {
	return []byte{byte(value >> 21 & 0x7f), byte(value >> 14 & 0x7f), byte(value >> 7 & 0x7f), byte(value & 0x7f)}
}

func isoMediaFixture(majorBrand string, video, audio bool, rotation int) []byte {
	ftypPayload := &bytes.Buffer{}
	ftypPayload.WriteString(majorBrand)
	_ = binary.Write(ftypPayload, binary.BigEndian, uint32(0))
	ftypPayload.WriteString(majorBrand)
	ftypPayload.WriteString("isom")

	mvhd := make([]byte, 100)
	binary.BigEndian.PutUint32(mvhd[12:16], 1000)
	binary.BigEndian.PutUint32(mvhd[16:20], 5000)
	moovPayload := bytes.NewBuffer(isoBoxFixture("mvhd", mvhd))
	trackID := uint32(1)
	if video {
		moovPayload.Write(isoTrackFixture(trackID, "vide", "avc1", 90000, 450000, 1920, 1080, rotation))
		trackID++
	}
	if audio {
		moovPayload.Write(isoTrackFixture(trackID, "soun", "mp4a", 48000, 240000, 0, 0, 0))
	}
	return bytes.Join([][]byte{
		isoBoxFixture("ftyp", ftypPayload.Bytes()),
		isoBoxFixture("moov", moovPayload.Bytes()),
		isoBoxFixture("mdat", make([]byte, 32)),
	}, nil)
}

func isoTrackFixture(trackID uint32, handler, codec string, timescale, duration uint32, width, height uint32, rotation int) []byte {
	tkhd := make([]byte, 84)
	tkhd[3] = 3
	binary.BigEndian.PutUint32(tkhd[12:16], trackID)
	writeRotationMatrix(tkhd[40:76], rotation)
	binary.BigEndian.PutUint32(tkhd[76:80], width<<16)
	binary.BigEndian.PutUint32(tkhd[80:84], height<<16)

	mdhd := make([]byte, 24)
	binary.BigEndian.PutUint32(mdhd[12:16], timescale)
	binary.BigEndian.PutUint32(mdhd[16:20], duration)
	hdlr := make([]byte, 24)
	copy(hdlr[8:12], []byte(handler))

	sampleEntry := make([]byte, 36)
	binary.BigEndian.PutUint32(sampleEntry[:4], uint32(len(sampleEntry)))
	copy(sampleEntry[4:8], []byte(codec))
	if handler == "soun" {
		binary.BigEndian.PutUint16(sampleEntry[24:26], 2)
		binary.BigEndian.PutUint16(sampleEntry[26:28], 16)
		binary.BigEndian.PutUint32(sampleEntry[32:36], 48000<<16)
	}
	stsd := make([]byte, 8)
	binary.BigEndian.PutUint32(stsd[4:8], 1)
	stsd = append(stsd, sampleEntry...)
	stbl := isoBoxFixture("stbl", isoBoxFixture("stsd", stsd))
	minf := isoBoxFixture("minf", stbl)
	mdia := isoBoxFixture("mdia", bytes.Join([][]byte{isoBoxFixture("mdhd", mdhd), isoBoxFixture("hdlr", hdlr), minf}, nil))
	return isoBoxFixture("trak", append(isoBoxFixture("tkhd", tkhd), mdia...))
}

func isoBoxFixture(boxType string, payload []byte) []byte {
	buffer := bytes.NewBuffer(make([]byte, 0, 8+len(payload)))
	_ = binary.Write(buffer, binary.BigEndian, uint32(8+len(payload)))
	buffer.WriteString(boxType)
	buffer.Write(payload)
	return buffer.Bytes()
}

func writeRotationMatrix(target []byte, rotation int) {
	values := [9]int32{0x10000, 0, 0, 0, 0x10000, 0, 0, 0, 0x40000000}
	switch rotation {
	case 90:
		values[0], values[1], values[3], values[4] = 0, 0x10000, -0x10000, 0
	case 180:
		values[0], values[1], values[3], values[4] = -0x10000, 0, 0, -0x10000
	case 270:
		values[0], values[1], values[3], values[4] = 0, -0x10000, 0x10000, 0
	}
	for index, value := range values {
		binary.BigEndian.PutUint32(target[index*4:(index+1)*4], uint32(value))
	}
}

func oversizedISOBoxFixture() []byte {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[:4], 1000)
	copy(payload[4:8], []byte("ftyp"))
	return payload
}

func excessiveSampleEntriesFixture() []byte {
	mvhd := make([]byte, 100)
	binary.BigEndian.PutUint32(mvhd[12:16], 1000)
	binary.BigEndian.PutUint32(mvhd[16:20], 1000)
	tkhd := make([]byte, 84)
	tkhd[3] = 3
	binary.BigEndian.PutUint32(tkhd[12:16], 1)
	writeRotationMatrix(tkhd[40:76], 0)
	binary.BigEndian.PutUint32(tkhd[76:80], 640<<16)
	binary.BigEndian.PutUint32(tkhd[80:84], 480<<16)
	mdhd := make([]byte, 24)
	binary.BigEndian.PutUint32(mdhd[12:16], 1000)
	binary.BigEndian.PutUint32(mdhd[16:20], 1000)
	hdlr := make([]byte, 24)
	copy(hdlr[8:12], []byte("vide"))
	stsd := make([]byte, 8)
	binary.BigEndian.PutUint32(stsd[4:8], 257)
	stbl := isoBoxFixture("stbl", isoBoxFixture("stsd", stsd))
	minf := isoBoxFixture("minf", stbl)
	mdia := isoBoxFixture("mdia", bytes.Join([][]byte{isoBoxFixture("mdhd", mdhd), isoBoxFixture("hdlr", hdlr), minf}, nil))
	trak := isoBoxFixture("trak", append(isoBoxFixture("tkhd", tkhd), mdia...))
	moov := isoBoxFixture("moov", append(isoBoxFixture("mvhd", mvhd), trak...))
	return append(isoBoxFixture("ftyp", []byte("isom\x00\x00\x00\x00isom")), moov...)
}
