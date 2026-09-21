package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deterministic PDF and archive inventory", func() {
	It("inventories PDF page-tree structure without extracting content", func() {
		path := filepath.Join(GinkgoT().TempDir(), "case.pdf")
		Expect(os.WriteFile(path, minimalPDF(false), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "case.pdf", evidenceClassification{Modality: "document"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("format", "pdf"))
		Expect(metadata).To(HaveKeyWithValue("extractor_version", "1.5.0"))
		Expect(metadata).To(HaveKeyWithValue("pdf_version", "1.7"))
		Expect(metadata).To(HaveKeyWithValue("page_count", 2))
		Expect(metadata).To(HaveKeyWithValue("declared_page_count", 2))
		Expect(metadata).To(HaveKeyWithValue("observed_page_objects", 2))
		Expect(metadata).To(HaveKeyWithValue("page_count_reliable", true))
		Expect(metadata).To(HaveKeyWithValue("has_eof_marker", true))
		Expect(metadata).To(HaveKeyWithValue("content_extracted", false))
		Expect(metadata).To(HaveKeyWithValue("inventory_complete", true))
	})

	It("flags PDF encryption, attachments, and active-content indicators", func() {
		path := filepath.Join(GinkgoT().TempDir(), "active.pdf")
		Expect(os.WriteFile(path, minimalPDF(true), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "active.pdf", evidenceClassification{Modality: "document"})
		Expect(metadata).To(HaveKeyWithValue("encrypted", true))
		Expect(metadata).To(HaveKeyWithValue("has_embedded_files", true))
		Expect(metadata).To(HaveKeyWithValue("has_javascript", true))
		Expect(metadata).To(HaveKeyWithValue("has_open_action", true))
		Expect(metadata).To(HaveKeyWithValue("has_launch_action", true))
		Expect(warnings).To(ContainElement(ContainSubstring("active-content indicators")))
		Expect(warnings).To(ContainElement(ContainSubstring("embedded-file content")))
	})

	It("does not count Page-like tokens inside PDF strings, comments, or streams", func() {
		payload := strings.Replace(string(minimalPDF(false)), "xref\n", "5 0 obj\n<< /Length 42 >>\nstream\n/Type /Page (% /Type /Page) % /Type /Page\nendstream\nendobj\nxref\n", 1)
		path := filepath.Join(GinkgoT().TempDir(), "stream.pdf")
		Expect(os.WriteFile(path, []byte(payload), 0o600)).To(Succeed())

		metadata, _ := extractDeterministicMediaMetadata(path, "stream.pdf", evidenceClassification{Modality: "document"})
		Expect(metadata).To(HaveKeyWithValue("observed_page_objects", 2))
	})

	It("preserves a bounded PDF inventory with a warning when EOF is missing", func() {
		payload := bytes.TrimSuffix(minimalPDF(false), []byte("%%EOF\n"))
		path := filepath.Join(GinkgoT().TempDir(), "truncated.pdf")
		Expect(os.WriteFile(path, payload, 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "truncated.pdf", evidenceClassification{Modality: "document"})
		Expect(metadata).To(HaveKeyWithValue("inventory_complete", false))
		Expect(metadata).To(HaveKeyWithValue("has_eof_marker", false))
		Expect(warnings).To(ContainElement(ContainSubstring("EOF marker")))
	})

	It("caps PDF structural tokens", func() {
		_, warnings := scanPDFStructuralTokens(bytes.Repeat([]byte("/Name "), 20), 8)
		Expect(warnings).To(ContainElement(ContainSubstring("token limit of 8")))
	})

	It("inventories a safe ZIP without decompressing or retaining member names", func() {
		path := filepath.Join(GinkgoT().TempDir(), "evidence.zip")
		Expect(os.WriteFile(path, zipFixture([]zipFixtureMember{
			{name: "records/", directory: true},
			{name: "records/calls.csv", payload: []byte("a,b\n1,2\n")},
			{name: "manifest.json", payload: []byte(`{"case":"synthetic"}`)},
		}), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "evidence.zip", evidenceClassification{Modality: "container"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("format", "zip"))
		Expect(metadata).To(HaveKeyWithValue("member_count", 3))
		Expect(metadata).To(HaveKeyWithValue("file_count", 2))
		Expect(metadata).To(HaveKeyWithValue("directory_count", 1))
		Expect(metadata).To(HaveKeyWithValue("unsafe_path_count", 0))
		Expect(metadata).To(HaveKeyWithValue("nested_archive_count", 0))
		Expect(metadata).To(HaveKeyWithValue("inventory_complete", true))
		Expect(metadata).To(HaveKeyWithValue("extraction_safe", true))
		Expect(metadata).To(HaveKeyWithValue("member_names_retained", false))
	})

	It("blocks ZIP traversal, links, nested archives, duplicate names, and extreme ratios", func() {
		path := filepath.Join(GinkgoT().TempDir(), "hostile.zip")
		payload := zipFixture([]zipFixtureMember{
			{name: "../escape.txt", payload: []byte("escape")},
			{name: "link", payload: []byte("../../target"), symlink: true},
			{name: "nested.zip", payload: []byte("PK")},
			{name: "duplicate.txt", payload: []byte("first")},
			{name: "duplicate.txt", payload: []byte("second")},
			{name: "ratio.txt", payload: bytes.Repeat([]byte("A"), 256<<10)},
		})
		Expect(os.WriteFile(path, payload, 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "hostile.zip", evidenceClassification{Modality: "container"})
		Expect(metadata).To(HaveKeyWithValue("unsafe_path_count", 1))
		Expect(metadata).To(HaveKeyWithValue("symlink_count", 1))
		Expect(metadata).To(HaveKeyWithValue("nested_archive_count", 1))
		Expect(metadata).To(HaveKeyWithValue("duplicate_name_count", 1))
		Expect(metadata).To(HaveKeyWithValue("extraction_safe", false))
		Expect(metadata["maximum_compression_ratio"].(float64)).To(BeNumerically(">", maxArchiveCompressionRatio))
		Expect(warnings).To(ContainElement(ContainSubstring("unsafe link paths")))
		Expect(warnings).To(ContainElement(ContainSubstring("recursive extraction remains disabled")))
		Expect(warnings).To(ContainElement(ContainSubstring("compression ratio")))
	})

	It("rejects a ZIP member-count bomb before central-directory parsing", func() {
		path := filepath.Join(GinkgoT().TempDir(), "member-bomb.zip")
		Expect(os.WriteFile(path, zipEOCDMemberBomb(), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "member-bomb.zip", evidenceClassification{Modality: "container"})
		Expect(metadata).To(BeEmpty())
		Expect(warnings).To(ConsistOf(ContainSubstring("inventory limit is 10000")))
	})

	It("inventories a checksum-valid TAR without reading member bodies", func() {
		path := filepath.Join(GinkgoT().TempDir(), "evidence.tar")
		Expect(os.WriteFile(path, tarFixture([]tarFixtureMember{
			{name: "records/", typeFlag: tar.TypeDir},
			{name: "records/calls.csv", payload: []byte("a,b\n1,2\n")},
			{name: "notes/readme.txt", payload: []byte("synthetic")},
		}), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "evidence.tar", evidenceClassification{Modality: "container"})
		Expect(warnings).To(BeEmpty())
		Expect(metadata).To(HaveKeyWithValue("format", "tar"))
		Expect(metadata).To(HaveKeyWithValue("member_count", 3))
		Expect(metadata).To(HaveKeyWithValue("file_count", 2))
		Expect(metadata).To(HaveKeyWithValue("directory_count", 1))
		Expect(metadata).To(HaveKeyWithValue("terminal_zero_blocks", 2))
		Expect(metadata).To(HaveKeyWithValue("extraction_safe", true))
	})

	It("blocks unsafe TAR paths and link targets", func() {
		path := filepath.Join(GinkgoT().TempDir(), "hostile.tar")
		Expect(os.WriteFile(path, tarFixture([]tarFixtureMember{
			{name: "/absolute.txt", payload: []byte("bad")},
			{name: "safe-link", typeFlag: tar.TypeSymlink, linkName: "../../escape"},
			{name: "nested.tar", payload: []byte("nested")},
		}), 0o600)).To(Succeed())

		metadata, warnings := extractDeterministicMediaMetadata(path, "hostile.tar", evidenceClassification{Modality: "container"})
		Expect(metadata).To(HaveKeyWithValue("unsafe_path_count", 1))
		Expect(metadata).To(HaveKeyWithValue("unsafe_link_target_count", 1))
		Expect(metadata).To(HaveKeyWithValue("symlink_count", 1))
		Expect(metadata).To(HaveKeyWithValue("nested_archive_count", 1))
		Expect(metadata).To(HaveKeyWithValue("extraction_safe", false))
		Expect(warnings).To(ContainElement(ContainSubstring("blocked from automatic extraction")))
	})

	It("rejects TAR checksum corruption and declared members beyond the file", func() {
		valid := tarFixture([]tarFixtureMember{{name: "record.txt", payload: []byte("data")}})
		corruptChecksum := append([]byte{}, valid...)
		corruptChecksum[10] ^= 0xff
		oversized := append([]byte{}, valid...)
		writeTAROctal(oversized[124:136], uint64(len(oversized)+4096))
		writeTARChecksum(oversized[:512])
		for _, fixture := range []struct {
			name    string
			payload []byte
			warning string
		}{
			{name: "bad-checksum.tar", payload: corruptChecksum, warning: "checksum mismatch"},
			{name: "oversized.tar", payload: oversized, warning: "member data exceeds"},
		} {
			path := filepath.Join(GinkgoT().TempDir(), fixture.name)
			Expect(os.WriteFile(path, fixture.payload, 0o600)).To(Succeed())
			metadata, warnings := extractDeterministicMediaMetadata(path, fixture.name, evidenceClassification{Modality: "container"})
			Expect(metadata).To(BeEmpty())
			Expect(warnings).To(ConsistOf(ContainSubstring(fixture.warning)))
		}
	})
})

func minimalPDF(active bool) []byte {
	var extraCatalog, extraObjects, trailer string
	if active {
		extraCatalog = " /OpenAction 5 0 R /Names << /EmbeddedFiles 6 0 R >>"
		extraObjects = "5 0 obj\n<< /S /JavaScript /JS (app.alert\\(synthetic\\)) /Next << /S /Launch >> >>\nendobj\n6 0 obj\n<< /Type /EmbeddedFile /Length 0 >>\nstream\n\nendstream\nendobj\n"
		trailer = " /Encrypt 7 0 R"
	}
	return []byte("%PDF-1.7\n" +
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R" + extraCatalog + " >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R >>\nendobj\n" +
		"4 0 obj\n<< /Type /Page /Parent 2 0 R >>\nendobj\n" + extraObjects +
		"xref\n0 1\n0000000000 65535 f\n" +
		"trailer\n<< /Size 8 /Root 1 0 R" + trailer + " >>\nstartxref\n0\n%%EOF\n")
}

type zipFixtureMember struct {
	name      string
	payload   []byte
	directory bool
	symlink   bool
}

func zipFixture(members []zipFixtureMember) []byte {
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)
	for _, member := range members {
		header := &zip.FileHeader{Name: member.name, Method: zip.Deflate}
		if member.directory {
			header.Method = zip.Store
			header.SetMode(os.ModeDir | 0o755)
		} else if member.symlink {
			header.SetMode(os.ModeSymlink | 0o777)
		} else {
			header.SetMode(0o600)
		}
		entry, err := writer.CreateHeader(header)
		Expect(err).NotTo(HaveOccurred())
		_, err = entry.Write(member.payload)
		Expect(err).NotTo(HaveOccurred())
	}
	Expect(writer.Close()).To(Succeed())
	return buffer.Bytes()
}

func zipEOCDMemberBomb() []byte {
	buffer := bytes.NewBuffer(make([]byte, 0, 22))
	buffer.Write([]byte{'P', 'K', 0x05, 0x06})
	_ = binary.Write(buffer, binary.LittleEndian, uint16(0))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(0))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(maxArchiveMembers+1))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(maxArchiveMembers+1))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(0))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(0))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(0))
	return buffer.Bytes()
}

type tarFixtureMember struct {
	name     string
	payload  []byte
	typeFlag byte
	linkName string
}

func tarFixture(members []tarFixtureMember) []byte {
	buffer := &bytes.Buffer{}
	writer := tar.NewWriter(buffer)
	for _, member := range members {
		typeFlag := member.typeFlag
		if typeFlag == 0 {
			typeFlag = tar.TypeReg
		}
		size := int64(len(member.payload))
		if typeFlag != tar.TypeReg {
			size = 0
		}
		header := &tar.Header{Name: member.name, Typeflag: typeFlag, Linkname: member.linkName, Mode: 0o600, Size: size, Format: tar.FormatUSTAR}
		Expect(writer.WriteHeader(header)).To(Succeed())
		if size > 0 {
			_, err := writer.Write(member.payload)
			Expect(err).NotTo(HaveOccurred())
		}
	}
	Expect(writer.Close()).To(Succeed())
	return buffer.Bytes()
}

func writeTAROctal(field []byte, value uint64) {
	for index := 0; index < len(field)-1; index++ {
		field[index] = '0'
	}
	valueText := []byte(formatOctal(value))
	copy(field[len(field)-1-len(valueText):len(field)-1], valueText)
	field[len(field)-1] = 0
}

func formatOctal(value uint64) string {
	if value == 0 {
		return "0"
	}
	var digits [24]byte
	position := len(digits)
	for value > 0 {
		position--
		digits[position] = byte('0' + value%8)
		value /= 8
	}
	return string(digits[position:])
}

func writeTARChecksum(header []byte) {
	for index := 148; index < 156; index++ {
		header[index] = ' '
	}
	var sum uint64
	for _, value := range header {
		sum += uint64(value)
	}
	text := formatOctal(sum)
	for index := 148; index < 156; index++ {
		header[index] = 0
	}
	copy(header[154-len(text):154], text)
	header[154] = 0
	header[155] = ' '
}
