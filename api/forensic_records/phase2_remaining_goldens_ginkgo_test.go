package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type phase2RemainingGoldenPack struct {
	SchemaVersion string                      `json:"schema_version"`
	Cases         []phase2RemainingGoldenCase `json:"cases"`
}

type phase2RemainingGoldenCase struct {
	ID                       string                     `json:"id"`
	Family                   string                     `json:"family"`
	Generator                string                     `json:"generator"`
	Filename                 string                     `json:"filename"`
	ContentType              string                     `json:"content_type"`
	Headers                  []string                   `json:"headers"`
	Hints                    map[string]string          `json:"hints"`
	ExpectedSHA256           string                     `json:"expected_sha256"`
	ExpectedClassification   modalityClassifierContract `json:"expected_classification"`
	ExpectedMetadata         map[string]any             `json:"expected_metadata"`
	ExpectedWarningFragments []string                   `json:"expected_warning_fragments"`
	Golden                   string                     `json:"golden"`
}

var _ = Describe("Phase 2 remaining modality goldens", func() {
	It("reconstructs every fixed payload and verifies classification, bounded metadata, and abstention routes", func() {
		manifestPath := filepath.Join("..", "..", "tests", "fixtures", "forensic_modalities", "phase2_remaining_goldens_v1.json")
		data, err := os.ReadFile(manifestPath)
		Expect(err).NotTo(HaveOccurred())
		var pack phase2RemainingGoldenPack
		Expect(json.Unmarshal(data, &pack)).To(Succeed())
		Expect(pack.SchemaVersion).To(Equal("1.0.0"))
		Expect(pack.Cases).To(HaveLen(12))

		seenFamilies := map[string]struct{}{}
		exportDir := strings.TrimSpace(os.Getenv("FORENSIC_PHASE2_FIXTURE_OUTPUT_DIR"))
		if exportDir != "" {
			Expect(os.MkdirAll(exportDir, 0o700)).To(Succeed())
		}
		for _, fixture := range pack.Cases {
			fixture := fixture
			By(fixture.ID)
			Expect(fixture.ID).NotTo(BeEmpty())
			Expect(fixture.Golden).NotTo(BeEmpty())
			Expect(seenFamilies).NotTo(HaveKey(fixture.Family))
			seenFamilies[fixture.Family] = struct{}{}

			payload := phase2GoldenPayload(fixture.Generator)
			Expect(payload).NotTo(BeEmpty())
			digest := fmt.Sprintf("%x", sha256.Sum256(payload))
			Expect(fixture.ExpectedSHA256).To(MatchRegexp(`^[0-9a-f]{64}$`), fixture.ID)
			Expect(digest).To(Equal(fixture.ExpectedSHA256), fixture.ID)

			path := filepath.Join(GinkgoT().TempDir(), fixture.Filename)
			Expect(os.WriteFile(path, payload, 0o600)).To(Succeed())
			if exportDir != "" {
				Expect(os.WriteFile(filepath.Join(exportDir, fixture.Filename), payload, 0o600)).To(Succeed())
			}
			classification := classifyEvidenceItem(
				fixture.Filename,
				fixture.ContentType,
				detectRecordType(fixture.Headers),
				fixture.Headers,
				fixture.Hints,
			)
			Expect(classification.Modality).To(Equal(fixture.ExpectedClassification.Modality), fixture.ID)
			Expect(classification.DetectedType).To(Equal(fixture.ExpectedClassification.DetectedType), fixture.ID)
			Expect(classification.ProcessingRoute).To(Equal(fixture.ExpectedClassification.Route), fixture.ID)
			Expect(classification.QueueRecords).To(Equal(fixture.ExpectedClassification.QueueRecords), fixture.ID)

			metadata, warnings := extractDeterministicMediaMetadata(path, fixture.Filename, classification)
			normalizedMetadata := normalizeGoldenJSON(metadata)
			for key, expected := range fixture.ExpectedMetadata {
				Expect(normalizedMetadata).To(HaveKeyWithValue(key, expected), fixture.ID+":"+key)
			}
			for _, fragment := range fixture.ExpectedWarningFragments {
				Expect(warnings).To(ContainElement(ContainSubstring(fragment)), fixture.ID)
			}
			if len(fixture.ExpectedWarningFragments) == 0 {
				Expect(warnings).To(BeEmpty(), fixture.ID)
			}
		}
	})
})

func normalizeGoldenJSON(value any) map[string]any {
	payload, err := json.Marshal(value)
	Expect(err).NotTo(HaveOccurred())
	result := map[string]any{}
	Expect(json.Unmarshal(payload, &result)).To(Succeed())
	return result
}

func phase2GoldenPayload(generator string) []byte {
	switch generator {
	case "generic_schema_drift_csv":
		return []byte("event_time,subject_id,custom_value,vendor_extra\n2026-07-20 08:00:00,SYN-001,alpha,\n2026-07-20T03:01:00Z,SYN-002,اردو تجرباتی,new-column\n2026-07-20 08:00:00,SYN-001,alpha,\n")
	case "minimal_xlsx":
		return minimalPhase2XLSX()
	case "minimal_pdf":
		return minimalPDF(false)
	case "oriented_jpeg":
		canvas := image.NewRGBA(image.Rect(0, 0, 12, 7))
		canvas.Set(5, 3, color.RGBA{R: 30, G: 80, B: 140, A: 255})
		buffer := &bytes.Buffer{}
		Expect(jpeg.Encode(buffer, canvas, &jpeg.Options{Quality: 90})).To(Succeed())
		return jpegWithEXIFOrientation(buffer.Bytes(), 8)
	case "pcm_wav":
		return pcmWAVFixture(8000, 1, 16, 8000)
	case "tts_pcm_wav":
		return pcmWAVFixture(16000, 1, 16, 8000)
	case "multispeaker_vtt":
		return []byte("WEBVTT\n\n00:00:00.000 --> 00:00:01.500\n<v Analyst>Assalam-o-alaikum.\n\n00:00:01.250 --> 00:00:03.000\n<v Witness>وعلیکم السلام، یہ تجرباتی متن ہے۔\n\n00:00:03.100 --> 00:00:04.000\n<v Analyst>Thank you.\n")
	case "iso_bmff_video_audio":
		return isoMediaFixture("isom", true, true, 90)
	case "minimal_pcapng":
		order := binary.LittleEndian
		payload := append(pcapngTestSection(order, -1), pcapngTestInterface(order, 1, 64, nil)...)
		return append(payload, pcapngTestEnhancedPacket(order, 0, 1_700_000_000_250_000, []byte{1, 2, 3, 4}, 4)...)
	case "sqlite_database":
		return sqliteTestDatabase(false)
	case "safe_zip":
		return zipFixture([]zipFixtureMember{
			{name: "records/", directory: true},
			{name: "records/events.csv", payload: []byte("id,value\n1,synthetic\n")},
			{name: "manifest.json", payload: []byte(`{"case":"synthetic"}`)},
		})
	case "unknown_binary":
		return []byte{0x13, 0x37, 0x00, 0xff, 0x4e, 0x58, 0x53, 0x59, 0x4e, 0x54, 0x48, 0x00, 0x01, 0x02, 0x03}
	default:
		Fail("unknown Phase 2 fixture generator: " + generator)
		return nil
	}
}

func minimalPhase2XLSX() []byte {
	entries := []struct {
		name string
		body string
	}{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Transactions" sheetId="1" r:id="rId1"/><sheet name="Review" sheetId="2" r:id="rId2"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/></Relationships>`},
		{"xl/worksheets/sheet1.xml", `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>account_number</t></is></c><c r="B1" t="inlineStr"><is><t>amount</t></is></c></row><row r="2"><c r="A2" t="inlineStr"><is><t>SYN-001</t></is></c><c r="B2"><v>1234.50</v></c></row><row r="3"><c r="A3" t="inlineStr"><is><t>SYN-TOTAL</t></is></c><c r="B3"><f>SUM(B2:B2)</f><v>1234.50</v></c></row></sheetData></worksheet>`},
		{"xl/worksheets/sheet2.xml", `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>review_status</t></is></c></row><row r="2"><c r="A2" t="inlineStr"><is><t>synthetic</t></is></c></row></sheetData></worksheet>`},
	}
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Store}
		file, err := writer.CreateHeader(header)
		Expect(err).NotTo(HaveOccurred())
		_, err = file.Write([]byte(entry.body))
		Expect(err).NotTo(HaveOccurred())
	}
	Expect(writer.Close()).To(Succeed())
	return buffer.Bytes()
}
