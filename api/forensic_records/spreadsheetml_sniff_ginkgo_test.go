package main

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const spreadsheetMLSample = `<?xml version="1.0"?>
<Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet" xmlns:ss="urn:schemas-microsoft-com:office:spreadsheet">
 <Worksheet ss:Name="TITLE"><Table><Row><Cell><Data ss:Type="String">No.</Data></Cell></Row></Table></Worksheet>
</Workbook>
`

// capturingSpreadsheetPublisher keeps the jobs the upload handler would put on the queue.
type capturingSpreadsheetPublisher struct{ jobs []ingestJob }

func (p *capturingSpreadsheetPublisher) Publish(_ context.Context, job ingestJob) error {
	p.jobs = append(p.jobs, job)
	return nil
}

func oleWorkbookBytes(size int) []byte {
	return append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, size)...)
}

var _ = Describe("SpreadsheetML 2003 workbook intake", func() {
	DescribeTable("recognises the workbook from the first bytes of the file",
		func(head []byte, want bool) {
			Expect(looksLikeSpreadsheetML2003(head)).To(Equal(want))
		},
		Entry("a declared workbook", []byte(spreadsheetMLSample), true),
		Entry("with a byte order mark", append([]byte{0xEF, 0xBB, 0xBF}, spreadsheetMLSample...), true),
		Entry("with leading blank lines", []byte("\r\n\r\n  "+spreadsheetMLSample), true),
		Entry("a different XML document", []byte(`<?xml version="1.0"?><note><to>x</to></note>`), false),
		Entry("the namespace without an XML declaration", []byte(`<Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet"/>`), false),
		Entry("a legacy binary workbook (OLE container)", oleWorkbookBytes(600), false),
		Entry("a delimited text export", []byte("No.\tStart Time\tEnd Time\n1\t2026-01-01\t2026-01-01\n"), false),
		Entry("an empty file", nil, false),
	)

	DescribeTable("judges the stored file by its content, and only for workbook names",
		func(content []byte, sourceName string, want bool) {
			path := filepath.Join(GinkgoT().TempDir(), "stored-object")
			Expect(os.WriteFile(path, content, 0o600)).To(Succeed())
			Expect(isSpreadsheetML2003File(path, sourceName)).To(Equal(want))
		},
		Entry("a workbook under .xls", []byte(spreadsheetMLSample), "ufone.xls", true),
		Entry("a workbook under .XLS", []byte(spreadsheetMLSample), "UFONE.XLS", true),
		Entry("a workbook under .xml", []byte(spreadsheetMLSample), "export.xml", true),
		Entry("a workbook under a name that is not a workbook name", []byte(spreadsheetMLSample), "export.csv", false),
		Entry("a workbook under .xlsx (the zip route decides those)", []byte(spreadsheetMLSample), "export.xlsx", false),
		Entry("a binary workbook under .xls", oleWorkbookBytes(600), "legacy.xls", false),
		Entry("a declaration beyond the bytes that are read", append(bytes.Repeat([]byte(" "), spreadsheetMLSniffBytes), spreadsheetMLSample...), "late.xls", false),
	)

	It("treats a file that cannot be opened as not a workbook", func() {
		Expect(isSpreadsheetML2003File(filepath.Join(GinkgoT().TempDir(), "missing"), "gone.xls")).To(BeFalse())
	})

	Context("through the upload handler", func() {
		upload := func(name string, content []byte) (*httptest.ResponseRecorder, *capturingSpreadsheetPublisher) {
			store, err := newContentAddressedStore(GinkgoT().TempDir())
			Expect(err).NotTo(HaveOccurred())
			publisher := &capturingSpreadsheetPublisher{}
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			for key, value := range map[string]string{"tenant_id": "tenant-1", "collection_id": "case-1", "case_id": "case-1", "skip_kb_mirror": "true", "source_timezone": "Asia/Karachi"} {
				Expect(form.WriteField(key, value)).To(Succeed())
			}
			part, err := form.CreateFormFile("file", name)
			Expect(err).NotTo(HaveOccurred())
			_, err = part.Write(content)
			Expect(err).NotTo(HaveOccurred())
			Expect(form.Close()).To(Succeed())
			request := httptest.NewRequest(http.MethodPost, "/webhooks/records/upload", &body)
			request.Header.Set("Content-Type", form.FormDataContentType())
			recorder := httptest.NewRecorder()
			uploadHandler(config{}, store, publisher, nil)(recorder, request)
			return recorder, publisher
		}

		It("queues a workbook named .xls for the records worker, which reads it through the typed ingest", func() {
			recorder, publisher := upload("ufone-export.xls", []byte(spreadsheetMLSample))
			Expect(recorder.Code).To(Equal(http.StatusAccepted), recorder.Body.String())
			Expect(publisher.jobs).To(HaveLen(1))
			job := publisher.jobs[0]
			Expect(job.SourceFile).To(Equal("ufone-export.xls"))
			Expect(job.Metadata).To(HaveKeyWithValue(workbookFormatHint, workbookFormatSpreadsheetML2003))
			Expect(job.Metadata).To(HaveKeyWithValue("evidence_modality", "tabular"))
			Expect(job.Metadata).To(HaveKeyWithValue("evidence_detected_type", workbookFormatSpreadsheetML2003))
			Expect(job.Metadata).To(HaveKeyWithValue("evidence_processing_route", "forensic_records_worker"))
			Expect(job.Metadata).To(HaveKeyWithValue("source_timezone", "Asia/Karachi"))
			Expect(job.Metadata).To(HaveKeyWithValue("source_timezone_state", "analyst_confirmed"))
		})

		It("queues a workbook named .xml as well", func() {
			recorder, publisher := upload("export.xml", []byte(spreadsheetMLSample))
			Expect(recorder.Code).To(Equal(http.StatusAccepted))
			Expect(publisher.jobs).To(HaveLen(1))
		})

		It("keeps a binary .xls registered, without a job", func() {
			recorder, publisher := upload("legacy.xls", oleWorkbookBytes(2048))
			Expect(recorder.Code).To(Equal(http.StatusOK))
			Expect(publisher.jobs).To(BeEmpty())
			var response map[string]any
			Expect(json.Unmarshal(recorder.Body.Bytes(), &response)).To(Succeed())
			Expect(response).To(HaveKeyWithValue("status", "registered"))
			Expect(response).To(HaveKeyWithValue("processing_route", "tabular_adapter_pending"))
		})

		It("keeps another XML document registered, without a job", func() {
			recorder, publisher := upload("note.xml", []byte(`<?xml version="1.0"?><note><to>x</to></note>`))
			Expect(recorder.Code).To(Equal(http.StatusOK))
			Expect(publisher.jobs).To(BeEmpty())
			Expect(recorder.Body.String()).To(ContainSubstring("tabular_adapter_pending"))
		})
	})
})
