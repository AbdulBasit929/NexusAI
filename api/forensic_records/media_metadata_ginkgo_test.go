package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deterministic media metadata", func() {
	It("extracts bounded PNG dimensions without a model", func() {
		path := filepath.Join(GinkgoT().TempDir(), "scene.png")
		canvas := image.NewRGBA(image.Rect(0, 0, 3, 2))
		canvas.Set(1, 1, color.RGBA{R: 255, A: 255})
		file, err := os.Create(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(png.Encode(file, canvas)).To(Succeed())
		Expect(file.Close()).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "scene.png", evidenceClassification{Modality: "image"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("format", "png"))
		Expect(metadata).To(HaveKeyWithValue("width_pixels", 3))
		Expect(metadata).To(HaveKeyWithValue("height_pixels", 2))
		Expect(metadata).To(HaveKeyWithValue("deterministic", true))
		Expect(metadata).To(HaveKeyWithValue("color_model", "rgb"))
		Expect(metadata).To(HaveKeyWithValue("intake_contract_version", imageIntakeContractVersion))
		Expect(metadata).To(HaveKeyWithValue("validation_state", "accepted_metadata_only"))
		Expect(metadata).To(HaveKeyWithValue("downstream_decode_allowed", true))
	})

	It("extracts JPEG and GIF dimensions through the same bounded contract", func() {
		canvas := image.NewRGBA(image.Rect(0, 0, 4, 3))
		canvas.Set(2, 1, color.RGBA{G: 255, A: 255})
		for _, fixture := range []struct {
			name   string
			format string
			encode func(*bytes.Buffer) error
		}{
			{name: "scene.jpg", format: "jpeg", encode: func(buffer *bytes.Buffer) error { return jpeg.Encode(buffer, canvas, nil) }},
			{name: "scene.gif", format: "gif", encode: func(buffer *bytes.Buffer) error { return gif.Encode(buffer, canvas, nil) }},
		} {
			buffer := &bytes.Buffer{}
			Expect(fixture.encode(buffer)).To(Succeed())
			path := filepath.Join(GinkgoT().TempDir(), fixture.name)
			Expect(os.WriteFile(path, buffer.Bytes(), 0o600)).To(Succeed())
			metadata, warnings := extractDeterministicMediaMetadata(path, fixture.name, evidenceClassification{Modality: "image"})
			Expect(warnings).To(BeEmpty())
			Expect(metadata).To(HaveKeyWithValue("format", fixture.format))
			Expect(metadata).To(HaveKeyWithValue("width_pixels", 4))
			Expect(metadata).To(HaveKeyWithValue("height_pixels", 3))
		}
	})

	It("extracts BMP, TIFF orientation, and WebP canvas metadata without decoding pixels", func() {
		fixtures := []struct {
			name       string
			payload    []byte
			width      int
			height     int
			format     string
			assertions func(map[string]any)
		}{
			{name: "scan.bmp", payload: bmpFixture(7, 5), width: 7, height: 5, format: "bmp"},
			{
				name: "scan.tiff", payload: tiffFixture(9, 6, 6), width: 9, height: 6, format: "tiff",
				assertions: func(metadata map[string]any) {
					Expect(metadata).To(HaveKeyWithValue("exif_orientation", uint16(6)))
					Expect(metadata).To(HaveKeyWithValue("display_width_pixels", 6))
					Expect(metadata).To(HaveKeyWithValue("display_height_pixels", 9))
				},
			},
			{
				name: "scan.webp", payload: webPFixture(11, 8), width: 11, height: 8, format: "webp",
				assertions: func(metadata map[string]any) {
					Expect(metadata).To(HaveKeyWithValue("animated", false))
				},
			},
		}
		for _, fixture := range fixtures {
			path := filepath.Join(GinkgoT().TempDir(), fixture.name)
			Expect(os.WriteFile(path, fixture.payload, 0o600)).To(Succeed())
			metadata, warnings := extractDeterministicMediaMetadata(path, fixture.name, evidenceClassification{Modality: "image"})
			Expect(warnings).To(BeEmpty(), fixture.name)
			Expect(metadata).To(HaveKeyWithValue("format", fixture.format))
			Expect(metadata).To(HaveKeyWithValue("width_pixels", fixture.width))
			Expect(metadata).To(HaveKeyWithValue("height_pixels", fixture.height))
			Expect(metadata).To(HaveKeyWithValue("pixel_count", uint64(fixture.width*fixture.height)))
			if fixture.assertions != nil {
				fixture.assertions(metadata)
			}
		}
	})

	It("extracts JPEG EXIF orientation without retaining sensitive free-text or GPS tags", func() {
		canvas := image.NewRGBA(image.Rect(0, 0, 12, 7))
		encoded := &bytes.Buffer{}
		Expect(jpeg.Encode(encoded, canvas, nil)).To(Succeed())
		payload := jpegWithEXIFOrientation(encoded.Bytes(), 8)
		path := filepath.Join(GinkgoT().TempDir(), "rotated.jpg")
		Expect(os.WriteFile(path, payload, 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "rotated.jpg", evidenceClassification{Modality: "image"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("exif_orientation", uint16(8)))
		Expect(metadata).To(HaveKeyWithValue("exif_orientation_label", "left_bottom"))
		Expect(metadata).To(HaveKeyWithValue("display_width_pixels", 7))
		Expect(metadata).To(HaveKeyWithValue("display_height_pixels", 12))
		Expect(metadata).NotTo(HaveKey("gps"))
		Expect(metadata).NotTo(HaveKey("camera_model"))
	})

	It("extracts WAV duration, channel, rate, and bit depth", func() {
		path := filepath.Join(GinkgoT().TempDir(), "interview.wav")
		payload := pcmWAVFixture(8000, 1, 16, 8000)
		Expect(os.WriteFile(path, payload, 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "interview.wav", evidenceClassification{Modality: "audio"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("format", "wav"))
		Expect(metadata).To(HaveKeyWithValue("channels", uint16(1)))
		Expect(metadata).To(HaveKeyWithValue("sample_rate_hz", uint32(8000)))
		Expect(metadata).To(HaveKeyWithValue("bits_per_sample", uint16(16)))
		Expect(metadata).To(HaveKeyWithValue("duration_ms", int64(1000)))
	})

	It("extracts bounded FLAC STREAMINFO duration and technical fields", func() {
		path := filepath.Join(GinkgoT().TempDir(), "interview.flac")
		Expect(os.WriteFile(path, flacFixture(48000, 2, 24, 96000), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "interview.flac", evidenceClassification{Modality: "audio"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("format", "flac"))
		Expect(metadata).To(HaveKeyWithValue("channels", uint8(2)))
		Expect(metadata).To(HaveKeyWithValue("sample_rate_hz", uint32(48000)))
		Expect(metadata).To(HaveKeyWithValue("bits_per_sample", uint8(24)))
		Expect(metadata).To(HaveKeyWithValue("total_samples", uint64(96000)))
		Expect(metadata).To(HaveKeyWithValue("duration_ms", int64(2000)))
		Expect(metadata).To(HaveKeyWithValue("duration_known", true))
	})

	It("flags extension-content disagreement while preserving detected header metadata", func() {
		path := filepath.Join(GinkgoT().TempDir(), "mislabelled.jpg")
		canvas := image.NewRGBA(image.Rect(0, 0, 2, 2))
		file, err := os.Create(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(png.Encode(file, canvas)).To(Succeed())
		Expect(file.Close()).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "mislabelled.jpg", evidenceClassification{Modality: "image"})
		Expect(metadata).To(HaveKeyWithValue("format", "png"))
		Expect(metadata).To(HaveKeyWithValue("extension_content_match", false))
		Expect(metadata).To(HaveKeyWithValue("validation_state", "manual_review_required"))
		Expect(metadata).To(HaveKeyWithValue("downstream_decode_allowed", false))
		classification := applyImageIntakeClassification(evidenceClassification{Modality: "image", ProcessingRoute: "image_ocr_vision_pending"}, metadata)
		Expect(classification.ProcessingRoute).To(Equal("image_intake_manual_review"))
		Expect(warnings).To(ConsistOf(ContainSubstring("does not match detected png content")))
	})

	DescribeTable("rejects malformed or resource-amplifying headers with bounded warnings",
		func(name string, payload []byte, modality, warning string) {
			path := filepath.Join(GinkgoT().TempDir(), name)
			Expect(os.WriteFile(path, payload, 0o600)).To(Succeed())
			metadata, warnings := extractDeterministicMediaMetadata(path, name, evidenceClassification{Modality: modality})
			if modality == "image" {
				Expect(metadata).To(HaveKeyWithValue("validation_state", "manual_review_required"))
				Expect(metadata).To(HaveKeyWithValue("downstream_decode_allowed", false))
				Expect(warnings).To(ConsistOf(And(ContainSubstring("image intake requires manual review"), ContainSubstring(warning))))
			} else {
				Expect(metadata).To(BeEmpty())
				Expect(warnings).To(ConsistOf(ContainSubstring(warning)))
			}
		},
		Entry("BMP size beyond file", "oversized.bmp", corruptBMPFixture(), "image", "declared size does not match"),
		Entry("TIFF IFD entry bomb", "ifd-bomb.tiff", tiffEntryBombFixture(), "image", "limit is 512"),
		Entry("WebP chunk beyond RIFF", "truncated.webp", corruptWebPFixture(), "image", "chunk exceeds the RIFF boundary"),
		Entry("FLAC zero sample rate", "zero-rate.flac", flacFixture(0, 1, 16, 100), "audio", "zero sample rate"),
		Entry("WAV chunk beyond RIFF", "truncated.wav", corruptWAVFixture(), "audio", "chunk exceeds the declared RIFF boundary"),
	)

	It("blocks automatic decode when dimensions, pixels, or bytes exceed policy", func() {
		metadata := mediaMetadataBase("png")
		metadata["width_pixels"] = maxImageDimensionPixels + 1
		metadata["height_pixels"] = 4
		metadata["extension_content_match"] = true
		warnings := []string{}
		addImageIntakeMetadata(metadata, maxImageEvidenceBytes+1, &warnings)

		Expect(metadata).To(HaveKeyWithValue("validation_state", "manual_review_required"))
		Expect(metadata).To(HaveKeyWithValue("downstream_decode_allowed", false))
		Expect(metadata).To(HaveKeyWithValue("decode_review_required", true))
		Expect(warnings).To(ContainElement(ContainSubstring("byte automatic-processing limit")))
		Expect(warnings).To(ContainElement(ContainSubstring("per-axis automatic-processing limit")))
		Expect(imageUploadExceedsLimit("camera.jpg", maxImageEvidenceBytes+1)).To(BeTrue())
		Expect(imageUploadExceedsLimit("camera.jpg", maxImageEvidenceBytes)).To(BeFalse())
		Expect(imageUploadExceedsLimit("records.csv", maxImageEvidenceBytes+1)).To(BeFalse())
	})

	It("preserves unsupported image formats without enabling decode", func() {
		path := filepath.Join(GinkgoT().TempDir(), "drawing.svg")
		Expect(os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o600)).To(Succeed())
		metadata, warnings := extractDeterministicMediaMetadata(path, "drawing.svg", evidenceClassification{Modality: "image"})
		Expect(metadata).To(HaveKeyWithValue("validation_state", "manual_review_required"))
		Expect(metadata).To(HaveKeyWithValue("downstream_decode_allowed", false))
		Expect(warnings).To(ContainElement(ContainSubstring("not enabled for bounded raster inspection")))
	})

	It("routes an image with appended polyglot payload to manual review", func() {
		path := filepath.Join(GinkgoT().TempDir(), "appended.png")
		canvas := image.NewRGBA(image.Rect(0, 0, 2, 2))
		encoded := &bytes.Buffer{}
		Expect(png.Encode(encoded, canvas)).To(Succeed())
		encoded.Write([]byte("PK\x03\x04appended-container"))
		Expect(os.WriteFile(path, encoded.Bytes(), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "appended.png", evidenceClassification{Modality: "image"})
		Expect(metadata).To(HaveKeyWithValue("validation_state", "manual_review_required"))
		Expect(metadata).To(HaveKeyWithValue("downstream_decode_allowed", false))
		Expect(warnings).To(ContainElement(ContainSubstring("trailing or invalid content")))
	})

	It("keeps corrupt media registered with a bounded warning", func() {
		path := filepath.Join(GinkgoT().TempDir(), "broken.wav")
		Expect(os.WriteFile(path, []byte("not-wave"), 0o600)).To(Succeed())
		metadata, warnings := extractDeterministicMediaMetadata(path, "broken.wav", evidenceClassification{Modality: "audio"})
		Expect(metadata).To(BeEmpty())
		Expect(warnings).To(ConsistOf(ContainSubstring("deterministic WAV metadata unavailable")))
	})
})

func pcmWAVFixture(sampleRate uint32, channels, bitsPerSample uint16, samples uint32) []byte {
	blockAlign := channels * bitsPerSample / 8
	byteRate := sampleRate * uint32(blockAlign)
	dataSize := samples * uint32(blockAlign)
	buffer := bytes.NewBuffer(make([]byte, 0, 44+dataSize))
	buffer.WriteString("RIFF")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(36)+dataSize)
	buffer.WriteString("WAVEfmt ")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(16))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(1))
	_ = binary.Write(buffer, binary.LittleEndian, channels)
	_ = binary.Write(buffer, binary.LittleEndian, sampleRate)
	_ = binary.Write(buffer, binary.LittleEndian, byteRate)
	_ = binary.Write(buffer, binary.LittleEndian, blockAlign)
	_ = binary.Write(buffer, binary.LittleEndian, bitsPerSample)
	buffer.WriteString("data")
	_ = binary.Write(buffer, binary.LittleEndian, dataSize)
	buffer.Write(make([]byte, dataSize))
	return buffer.Bytes()
}

func bmpFixture(width, height int32) []byte {
	rowBytes := ((width*3 + 3) / 4) * 4
	pixelBytes := rowBytes * height
	buffer := bytes.NewBuffer(make([]byte, 0, 54+pixelBytes))
	buffer.WriteString("BM")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(54+pixelBytes))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(0))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(54))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(40))
	_ = binary.Write(buffer, binary.LittleEndian, width)
	_ = binary.Write(buffer, binary.LittleEndian, height)
	_ = binary.Write(buffer, binary.LittleEndian, uint16(1))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(24))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(0))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(pixelBytes))
	_ = binary.Write(buffer, binary.LittleEndian, int32(2835))
	_ = binary.Write(buffer, binary.LittleEndian, int32(2835))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(0))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(0))
	buffer.Write(make([]byte, pixelBytes))
	return buffer.Bytes()
}

func tiffFixture(width, height uint32, orientation uint16) []byte {
	buffer := &bytes.Buffer{}
	buffer.WriteString("II")
	_ = binary.Write(buffer, binary.LittleEndian, uint16(42))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(8))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(3))
	writeTIFFEntry(buffer, 256, 4, width)
	writeTIFFEntry(buffer, 257, 4, height)
	writeTIFFEntry(buffer, 274, 3, uint32(orientation))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(0))
	return buffer.Bytes()
}

func writeTIFFEntry(buffer *bytes.Buffer, tag, fieldType uint16, value uint32) {
	_ = binary.Write(buffer, binary.LittleEndian, tag)
	_ = binary.Write(buffer, binary.LittleEndian, fieldType)
	_ = binary.Write(buffer, binary.LittleEndian, uint32(1))
	if fieldType == 3 {
		_ = binary.Write(buffer, binary.LittleEndian, uint16(value))
		_ = binary.Write(buffer, binary.LittleEndian, uint16(0))
		return
	}
	_ = binary.Write(buffer, binary.LittleEndian, value)
}

func jpegWithEXIFOrientation(jpegPayload []byte, orientation uint16) []byte {
	tiff := &bytes.Buffer{}
	tiff.WriteString("II")
	_ = binary.Write(tiff, binary.LittleEndian, uint16(42))
	_ = binary.Write(tiff, binary.LittleEndian, uint32(8))
	_ = binary.Write(tiff, binary.LittleEndian, uint16(1))
	writeTIFFEntry(tiff, 274, 3, uint32(orientation))
	_ = binary.Write(tiff, binary.LittleEndian, uint32(0))
	appPayload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	segment := &bytes.Buffer{}
	segment.Write([]byte{0xff, 0xe1})
	_ = binary.Write(segment, binary.BigEndian, uint16(len(appPayload)+2))
	segment.Write(appPayload)
	return append(append(append([]byte{}, jpegPayload[:2]...), segment.Bytes()...), jpegPayload[2:]...)
}

func webPFixture(width, height uint32) []byte {
	chunks := &bytes.Buffer{}
	chunks.WriteString("VP8X")
	_ = binary.Write(chunks, binary.LittleEndian, uint32(10))
	chunks.Write([]byte{0, 0, 0, 0})
	writeLittleEndianUint24(chunks, width-1)
	writeLittleEndianUint24(chunks, height-1)
	chunks.WriteString("VP8L")
	_ = binary.Write(chunks, binary.LittleEndian, uint32(6))
	bits := (width - 1) | (height-1)<<14
	chunks.WriteByte(0x2f)
	_ = binary.Write(chunks, binary.LittleEndian, bits)
	chunks.WriteByte(0)
	buffer := &bytes.Buffer{}
	buffer.WriteString("RIFF")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(chunks.Len()+4))
	buffer.WriteString("WEBP")
	buffer.Write(chunks.Bytes())
	return buffer.Bytes()
}

func writeLittleEndianUint24(buffer *bytes.Buffer, value uint32) {
	buffer.Write([]byte{byte(value), byte(value >> 8), byte(value >> 16)})
}

func flacFixture(sampleRate uint32, channels, bitsPerSample uint8, totalSamples uint64) []byte {
	buffer := bytes.NewBuffer(make([]byte, 0, 42))
	buffer.WriteString("fLaC")
	buffer.Write([]byte{0x80, 0, 0, 34})
	streamInfo := make([]byte, 34)
	binary.BigEndian.PutUint16(streamInfo[0:2], 4096)
	binary.BigEndian.PutUint16(streamInfo[2:4], 4096)
	packed := uint64(sampleRate)<<44 | uint64(channels-1)<<41 | uint64(bitsPerSample-1)<<36 | totalSamples
	binary.BigEndian.PutUint64(streamInfo[10:18], packed)
	buffer.Write(streamInfo)
	return buffer.Bytes()
}

func corruptBMPFixture() []byte {
	payload := bmpFixture(1, 1)
	binary.LittleEndian.PutUint32(payload[2:6], uint32(len(payload)+100))
	return payload
}

func tiffEntryBombFixture() []byte {
	buffer := &bytes.Buffer{}
	buffer.WriteString("II")
	_ = binary.Write(buffer, binary.LittleEndian, uint16(42))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(8))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(maxTIFFIFDEntries+1))
	return buffer.Bytes()
}

func corruptWebPFixture() []byte {
	buffer := &bytes.Buffer{}
	buffer.WriteString("RIFF")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(14))
	buffer.WriteString("WEBPVP8X")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(10))
	buffer.Write([]byte{0, 0})
	return buffer.Bytes()
}

func corruptWAVFixture() []byte {
	buffer := &bytes.Buffer{}
	buffer.WriteString("RIFF")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(12))
	buffer.WriteString("WAVEfmt ")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(100))
	return buffer.Bytes()
}
