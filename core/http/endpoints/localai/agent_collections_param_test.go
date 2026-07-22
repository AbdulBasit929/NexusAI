package localai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Regression for #10443: agent/collection names carry a "legacy-api-key:"
// prefix, so the ':' is percent-encoded as %3A in the request path. Echo routes
// such paths via URL.RawPath and stores the path-param value still escaped, so
// handlers must URL-decode it before looking the collection up in the store -
// otherwise the lookup sees "legacy-api-key%3ALiteraryResearch" and 404s.
var _ = Describe("decodedParam", func() {
	var e *echo.Echo

	BeforeEach(func() {
		e = echo.New()
	})

	// route runs a request through Echo's real router so the path param is
	// populated exactly as it would be in production, then returns the decoded
	// value the handler would observe.
	route := func(rawPath string) string {
		var got string
		e.GET("/api/agents/collections/:name/upload", func(c echo.Context) error {
			got = decodedParam(c, "name")
			return c.NoContent(http.StatusOK)
		})
		req := httptest.NewRequest(http.MethodGet, rawPath, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusOK))
		return got
	}

	It("decodes a percent-encoded colon in the collection name", func() {
		got := route("/api/agents/collections/legacy-api-key%3ALiteraryResearch/upload")
		Expect(got).To(Equal("legacy-api-key:LiteraryResearch"))
	})

	It("leaves an unencoded name untouched", func() {
		got := route("/api/agents/collections/PlainCollection/upload")
		Expect(got).To(Equal("PlainCollection"))
	})
})

var _ = Describe("forensic records KB upload forwarding", func() {
	DescribeTable("detects structured-looking uploads",
		func(filename string, expected bool) {
			Expect(shouldForwardKBUploadToForensicRecords(filename)).To(Equal(expected))
		},
		Entry("csv", "cdr.csv", true),
		Entry("jsonl", "sessions.jsonl", true),
		Entry("log", "access.log", true),
		Entry("pdf", "policy.pdf", false),
		Entry("image", "photo.png", false),
	)

	It("reads opt-in sidecar config from environment", func() {
		DeferCleanup(func() {
			_ = os.Unsetenv("FORENSIC_RECORDS_API_URL")
			_ = os.Unsetenv("LOCALAI_FORENSIC_RECORDS_API_URL")
			_ = os.Unsetenv("FORENSIC_RECORDS_KB_UPLOAD_ENABLED")
			_ = os.Unsetenv("FORENSIC_RECORDS_TENANT_ID")
		})
		Expect(os.Setenv("FORENSIC_RECORDS_API_URL", "http://records-api:8091/")).To(Succeed())
		Expect(os.Setenv("FORENSIC_RECORDS_TENANT_ID", "tenant-a")).To(Succeed())

		cfg := forensicRecordsKBUploadConfigFromEnv()
		Expect(cfg.Enabled).To(BeTrue())
		Expect(cfg.APIURL).To(Equal("http://records-api:8091"))
		Expect(cfg.TenantID).To(Equal("tenant-a"))

		Expect(os.Setenv("FORENSIC_RECORDS_KB_UPLOAD_ENABLED", "false")).To(Succeed())
		Expect(forensicRecordsKBUploadConfigFromEnv().Enabled).To(BeFalse())
	})

	It("preserves precise forensic sidecar statuses", func() {
		Expect(forensicRecordsResultStatus(map[string]any{"status": "duplicate"})).To(Equal("duplicate"))
		Expect(forensicRecordsResultStatus(map[string]any{"status": " queued "})).To(Equal("queued"))
		Expect(forensicRecordsResultStatus(map[string]any{"job_id": "job-1"})).To(Equal("queued"))
	})

	It("forwards an existing KB upload to the forensic sidecar without remirroring", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Path).To(Equal("/webhooks/records/upload"))
			Expect(r.Header.Get("Authorization")).To(Equal("Bearer token"))
			Expect(r.ParseMultipartForm(1 << 20)).To(Succeed())
			Expect(r.FormValue("tenant_id")).To(Equal("default"))
			Expect(r.FormValue("collection_id")).To(Equal("records-demo"))
			Expect(r.FormValue("user_id")).To(Equal("user-1"))
			Expect(r.FormValue("record_type")).To(Equal("auto"))
			Expect(r.FormValue("skip_kb_mirror")).To(Equal("true"))
			Expect(r.FormValue("source_entry")).To(Equal("uuid/cdr.csv"))
			file, header, err := r.FormFile("file")
			Expect(err).ToNot(HaveOccurred())
			defer file.Close()
			Expect(header.Filename).To(Equal("cdr.csv"))
			body, err := io.ReadAll(file)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(body)).To(Equal("a,b\n1,2\n"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"job_id":"job-1","status":"queued"}`))
		}))
		defer server.Close()

		result, err := forwardKBUploadToForensicRecords(
			context.Background(),
			forensicRecordsKBUploadConfig{Enabled: true, APIURL: server.URL, APIKey: "token", TenantID: "default"},
			"records-demo",
			"user-1",
			"cdr.csv",
			"uuid/cdr.csv",
			strings.NewReader("a,b\n1,2\n"),
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(HaveKeyWithValue("job_id", "job-1"))
	})
})

var _ = Describe("collection request normalization", func() {
	DescribeTable("normalizes collection search requests",
		func(query string, maxResults int, expectedQuery string, expectedMaxResults int) {
			gotQuery, gotMaxResults, err := normalizeCollectionSearchRequest(query, maxResults)
			Expect(err).NotTo(HaveOccurred())
			Expect(gotQuery).To(Equal(expectedQuery))
			Expect(gotMaxResults).To(Equal(expectedMaxResults))
		},
		Entry("trims a raw query", "  shortest duration call on 2026-07-10  ", 5, "shortest duration call on 2026-07-10", 5),
		Entry("defaults a missing max results value", "target numbers in Gulberg", 0, "target numbers in Gulberg", defaultCollectionSearchMaxResults),
		Entry("caps excessive max results", "all calls from area code 042", 500, "all calls from area code 042", maxCollectionSearchMaxResults),
	)

	It("rejects a blank collection search query", func() {
		_, _, err := normalizeCollectionSearchRequest("   ", 10)
		Expect(err).To(MatchError("query is required"))
	})

	It("caps requested search results to the indexed chunk count", func() {
		svc := fakeCollectionContentService{
			chunks: map[string]int{
				"one.txt": 1,
				"two.md":  2,
			},
		}
		Expect(estimateCollectionSearchDocumentCount(svc, "user-1", "records-demo", []string{"one.txt", "two.md"}, 10)).To(Equal(3))
		Expect(estimateCollectionSearchDocumentCount(svc, "user-1", "records-demo", []string{"one.txt", "two.md"}, 2)).To(Equal(3))
	})

	DescribeTable("extracts a safe retry cap from Chroma collection-size errors",
		func(message string, requested int, expected int) {
			Expect(capSearchResultsFromCollectionSizeError(fakeSearchError(message), requested)).To(Equal(expected))
		},
		Entry("exact nResults message with count", "nResults must be <= number of documents in the collection (2)", 10, 2),
		Entry("collection contains wording", "nResults must be <= the number of documents in the collection; collection contains 4 documents", 10, 4),
		Entry("no count available", "nResults must be <= the number of documents in the collection", 10, 0),
		Entry("larger than requested", "collection contains 99 documents", 10, 0),
	)

	DescribeTable("parses source update intervals",
		func(raw json.RawMessage, expected int) {
			got, err := parseCollectionSourceIntervalMinutes(raw)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(expected))
		},
		Entry("defaults a missing interval", json.RawMessage(nil), defaultCollectionSourceIntervalMinutes),
		Entry("defaults null", json.RawMessage("null"), defaultCollectionSourceIntervalMinutes),
		Entry("defaults an empty string", json.RawMessage(`""`), defaultCollectionSourceIntervalMinutes),
		Entry("accepts numeric minutes", json.RawMessage("15"), 15),
		Entry("accepts numeric strings as minutes", json.RawMessage(`"45"`), 45),
		Entry("accepts minute durations", json.RawMessage(`"30m"`), 30),
		Entry("accepts hour durations", json.RawMessage(`"1h"`), 60),
		Entry("rounds sub-minute precision up to a full minute", json.RawMessage(`"90s"`), 2),
	)

	It("rejects invalid source update intervals", func() {
		_, err := parseCollectionSourceIntervalMinutes(json.RawMessage(`"daily"`))
		Expect(err).To(MatchError("update_interval must be minutes or a duration like 30m or 1h"))
	})
})

type fakeCollectionContentService struct {
	chunks map[string]int
}

func (f fakeCollectionContentService) GetCollectionEntryContentForUser(_, _, entry string) (string, int, error) {
	return "", f.chunks[entry], nil
}

type fakeSearchError string

func (e fakeSearchError) Error() string {
	return string(e)
}
