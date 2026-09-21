package localai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mudler/LocalAI/core/http/auth"
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
	DescribeTable("registers every uploaded evidence format",
		func(filename string, expected bool) {
			Expect(shouldForwardKBUploadToForensicRecords(filename)).To(Equal(expected))
		},
		Entry("csv", "cdr.csv", true),
		Entry("jsonl", "sessions.jsonl", true),
		Entry("log", "access.log", true),
		Entry("financial workbook", "transactions.xlsx", true),
		Entry("pdf", "policy.pdf", true),
		Entry("image", "photo.png", true),
		Entry("audio", "interview.wav", true),
		Entry("video", "camera.mp4", true),
		Entry("packet capture", "traffic.pcapng", true),
		Entry("unknown evidence", "device.dump", true),
		Entry("blank filename", "", false),
	)

	DescribeTable("honors the sidecar mirror marker",
		func(filename, skipValue string, expected bool) {
			Expect(shouldForwardKBUploadRequestToForensicRecords(filename, skipValue)).To(Equal(expected))
		},
		Entry("normal structured upload", "cdr.csv", "", true),
		Entry("sidecar mirror upload", "cdr.csv", "true", false),
		Entry("non-structured evidence upload", "policy.pdf", "", true),
		Entry("sidecar non-structured mirror", "interview.wav", "true", false),
	)

	It("reads opt-in sidecar config from environment", func() {
		DeferCleanup(func() {
			_ = os.Unsetenv("FORENSIC_RECORDS_API_URL")
			_ = os.Unsetenv("LOCALAI_FORENSIC_RECORDS_API_URL")
			_ = os.Unsetenv("FORENSIC_RECORDS_KB_UPLOAD_ENABLED")
			_ = os.Unsetenv("FORENSIC_RECORDS_TENANT_ID")
			_ = os.Unsetenv("FORENSIC_RECORDS_API_KEY")
			_ = os.Unsetenv("FORENSIC_RECORDS_UPLOAD_TIMEOUT")
		})
		Expect(os.Setenv("FORENSIC_RECORDS_API_URL", "http://records-api:8091/")).To(Succeed())
		Expect(os.Setenv("FORENSIC_RECORDS_TENANT_ID", "tenant-a")).To(Succeed())
		Expect(os.Setenv("FORENSIC_RECORDS_API_KEY", "service-token")).To(Succeed())
		Expect(os.Unsetenv("FORENSIC_RECORDS_UPLOAD_TIMEOUT")).To(Succeed())

		cfg := forensicRecordsKBUploadConfigFromEnv()
		Expect(cfg.Enabled).To(BeTrue())
		Expect(cfg.APIURL).To(Equal("http://records-api:8091"))
		Expect(cfg.TenantID).To(Equal("tenant-a"))
		Expect(cfg.APIKey).To(Equal("service-token"))
		Expect(cfg.UploadTimeout).To(Equal(5 * time.Minute))
		Expect(os.Setenv("FORENSIC_RECORDS_UPLOAD_TIMEOUT", "12m")).To(Succeed())
		Expect(forensicRecordsKBUploadConfigFromEnv().UploadTimeout).To(Equal(12 * time.Minute))

		Expect(os.Setenv("FORENSIC_RECORDS_KB_UPLOAD_ENABLED", "false")).To(Succeed())
		Expect(forensicRecordsKBUploadConfigFromEnv().Enabled).To(BeFalse())
	})

	It("preserves precise forensic sidecar statuses", func() {
		Expect(forensicRecordsResultStatus(map[string]any{"status": "duplicate"})).To(Equal("duplicate"))
		Expect(forensicRecordsResultStatus(map[string]any{"status": " queued "})).To(Equal("queued"))
		Expect(forensicRecordsResultStatus(map[string]any{"job_id": "job-1"})).To(Equal("queued"))
	})

	It("requires exact authenticated collection ownership for forensic proxy access", func() {
		collections := []string{"case-a", "case-b"}
		Expect(forensicCollectionAllowed(collections, " case-a ")).To(BeTrue())
		Expect(forensicCollectionAllowed(collections, "case-c")).To(BeFalse())
		Expect(forensicCollectionAllowed(collections, "")).To(BeFalse())
	})

	It("forwards an existing KB upload to the forensic sidecar without remirroring", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Path).To(Equal("/webhooks/records/upload"))
			Expect(r.Header.Get("Authorization")).To(Equal("Bearer token"))
			Expect(r.Header.Get("X-Forensic-Tenant-ID")).To(Equal("default"))
			Expect(r.Header.Get("X-Forensic-Actor-ID")).To(Equal("actor-1"))
			Expect(r.Header.Get("X-Forensic-Subject-ID")).To(Equal("user-1"))
			Expect(r.Header.Get("X-Forensic-Actor-Role")).To(Equal("admin"))
			Expect(r.Header.Get("X-Forensic-Collection-ID")).To(Equal("records-demo"))
			Expect(r.Header.Get("X-Forensic-Case-ID")).To(Equal("case-7"))
			Expect(r.ParseMultipartForm(1 << 20)).To(Succeed())
			Expect(r.FormValue("tenant_id")).To(Equal("default"))
			Expect(r.FormValue("collection_id")).To(Equal("records-demo"))
			Expect(r.FormValue("user_id")).To(Equal("user-1"))
			Expect(r.FormValue("record_type")).To(Equal("cdr"))
			Expect(r.FormValue("skip_kb_mirror")).To(Equal("true"))
			Expect(r.FormValue("source_entry")).To(Equal("uuid/cdr.csv"))
			Expect(r.FormValue("declared_modality")).To(Equal("structured_records"))
			Expect(r.FormValue("evidence_role")).To(Equal("source"))
			Expect(r.FormValue("case_id")).To(Equal("case-7"))
			Expect(r.FormValue("jurisdiction")).To(Equal("PK"))
			Expect(r.FormValue("source_timezone")).To(Equal("Asia/Karachi"))
			Expect(r.FormValue("source_timezone_state")).To(Equal("analyst_confirmed"))
			Expect(r.FormValue("source_date_order")).To(Equal("DMY"))
			Expect(r.FormValue("source_date_order_state")).To(Equal("analyst_confirmed"))
			Expect(r.FormValue("asr_language")).To(Equal("ur"))
			file, header, err := r.FormFile("file")
			Expect(err).ToNot(HaveOccurred())
			defer file.Close()
			Expect(header.Filename).To(Equal("cdr.csv"))
			Expect(header.Header.Get("Content-Type")).To(Equal("text/csv"))
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
			"actor-1",
			"admin",
			"cdr.csv",
			"text/csv",
			"uuid/cdr.csv",
			strings.NewReader("a,b\n1,2\n"),
			map[string]string{
				"record_type":             "cdr",
				"declared_modality":       "structured_records",
				"evidence_role":           "source",
				"case_id":                 "case-7",
				"jurisdiction":            "PK",
				"source_timezone":         "Asia/Karachi",
				"source_timezone_state":   "analyst_confirmed",
				"source_date_order":       "DMY",
				"source_date_order_state": "analyst_confirmed",
				"asr_language":            "ur",
			},
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(HaveKeyWithValue("job_id", "job-1"))
	})

	It("binds authenticated LocalAI identity to every forensic sidecar proxy request", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Header.Get("Authorization")).To(Equal("Bearer proxy-token"))
			Expect(r.Header.Get("X-Forensic-Tenant-ID")).To(Equal("tenant-a"))
			Expect(r.Header.Get("X-Forensic-Actor-ID")).To(Equal("actor-1"))
			Expect(r.Header.Get("X-Forensic-Subject-ID")).To(Equal("actor-1"))
			Expect(r.Header.Get("X-Forensic-Actor-Role")).To(Equal("admin"))
			Expect(r.Header.Get("X-Forensic-Collection-ID")).To(Equal("collection-a"))
			Expect(r.Header.Get("X-Forensic-Case-ID")).To(Equal("case-a"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}))
		defer server.Close()
		DeferCleanup(func() {
			_ = os.Unsetenv("FORENSIC_RECORDS_API_URL")
			_ = os.Unsetenv("FORENSIC_RECORDS_API_KEY")
			_ = os.Unsetenv("FORENSIC_RECORDS_TENANT_ID")
		})
		Expect(os.Setenv("FORENSIC_RECORDS_API_URL", server.URL)).To(Succeed())
		Expect(os.Setenv("FORENSIC_RECORDS_API_KEY", "proxy-token")).To(Succeed())
		Expect(os.Setenv("FORENSIC_RECORDS_TENANT_ID", "tenant-a")).To(Succeed())

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/records/forensic/status", nil)
		recorder := httptest.NewRecorder()
		ctx := e.NewContext(req, recorder)
		ctx.Set("auth_user", &auth.User{ID: "actor-1", Role: auth.RoleAdmin})
		Expect(proxyForensicRecords(ctx, http.MethodGet, "/collections/status", nil, nil, "collection-a", "case-a")).To(Succeed())
		Expect(recorder.Code).To(Equal(http.StatusOK))
	})

	It("requires an explicit fallback identity when LocalAI auth is disabled", func() {
		DeferCleanup(func() {
			_ = os.Unsetenv("FORENSIC_RECORDS_PROXY_ACTOR_ID")
			_ = os.Unsetenv("FORENSIC_RECORDS_PROXY_ACTOR_ROLE")
		})
		Expect(os.Unsetenv("FORENSIC_RECORDS_PROXY_ACTOR_ID")).To(Succeed())
		Expect(os.Unsetenv("FORENSIC_RECORDS_PROXY_ACTOR_ROLE")).To(Succeed())
		e := echo.New()
		ctx := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
		_, _, _, err := forensicForwardIdentity(ctx)
		Expect(err).To(MatchError("forensic proxy identity is not configured for unauthenticated LocalAI"))
	})

	It("uses the configured accountable identity for a local no-auth UI", func() {
		DeferCleanup(func() {
			_ = os.Unsetenv("FORENSIC_RECORDS_PROXY_ACTOR_ID")
			_ = os.Unsetenv("FORENSIC_RECORDS_PROXY_ACTOR_ROLE")
		})
		Expect(os.Setenv("FORENSIC_RECORDS_PROXY_ACTOR_ID", "local-operator")).To(Succeed())
		Expect(os.Setenv("FORENSIC_RECORDS_PROXY_ACTOR_ROLE", "admin")).To(Succeed())
		e := echo.New()
		ctx := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
		actorID, subjectID, actorRole, err := forensicForwardIdentity(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(actorID).To(Equal("local-operator"))
		Expect(subjectID).To(Equal("local-operator"))
		Expect(actorRole).To(Equal("admin"))
	})

	It("forwards stable reprocessing idempotency and trusted evidence scope", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Path).To(Equal("/evidence/evidence-1/reprocess"))
			Expect(r.Header.Get("Idempotency-Key")).To(Equal("case-a:reprocess-001"))
			Expect(r.Header.Get("X-Forensic-Collection-ID")).To(Equal("case-a"))
			Expect(r.Header.Get("X-Forensic-Actor-ID")).To(Equal("user-a"))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"queued","job_id":"job-2"}`))
		}))
		defer server.Close()
		DeferCleanup(func() {
			_ = os.Unsetenv("FORENSIC_RECORDS_API_URL")
			_ = os.Unsetenv("FORENSIC_RECORDS_API_KEY")
			_ = os.Unsetenv("FORENSIC_RECORDS_TENANT_ID")
		})
		Expect(os.Setenv("FORENSIC_RECORDS_API_URL", server.URL)).To(Succeed())
		Expect(os.Setenv("FORENSIC_RECORDS_API_KEY", "proxy-token")).To(Succeed())
		Expect(os.Setenv("FORENSIC_RECORDS_TENANT_ID", "tenant-a")).To(Succeed())

		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/api/records/forensic/evidence/evidence-1/reprocess", strings.NewReader(`{"collection_id":"case-a","reason":"operator-approved retry"}`))
		req.Header.Set("Idempotency-Key", "case-a:reprocess-001")
		recorder := httptest.NewRecorder()
		ctx := e.NewContext(req, recorder)
		ctx.Set("auth_user", &auth.User{ID: "user-a", Role: auth.RoleUser})
		Expect(proxyForensicRecords(
			ctx, http.MethodPost, "/evidence/evidence-1/reprocess", nil,
			strings.NewReader(`{"collection_id":"case-a","reason":"operator-approved retry"}`),
			"case-a", "",
		)).To(Succeed())
		Expect(recorder.Code).To(Equal(http.StatusAccepted))
	})

	It("preserves delegated agent-worker provenance", func() {
		e := echo.New()
		ctx := e.NewContext(
			httptest.NewRequest(http.MethodGet, "/api/records/forensic/status?user_id=subject-1", nil),
			httptest.NewRecorder(),
		)
		ctx.Set("auth_user", &auth.User{
			ID:       "worker-1",
			Role:     auth.RoleUser,
			Provider: auth.ProviderAgentWorker,
		})
		Expect(effectiveUserID(ctx)).To(Equal("subject-1"))
		Expect(forensicActorRole(ctx)).To(Equal("agent-worker"))
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
