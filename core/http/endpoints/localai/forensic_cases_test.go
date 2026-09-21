package localai

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/mudler/LocalAI/core/http/auth"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Phase 5 forensic case governance", func() {
	It("selects the verified pilot without merging legacy aliases", func() {
		item := classifyForensicCase("records-demo-verified")
		Expect(item.Purpose).To(Equal("canonical_pilot"))
		Expect(item.Visibility).To(Equal("analyst"))
		Expect(item.Selectable).To(BeTrue())
		Expect(item.Aliases).To(ConsistOf("records-demo", "nexusai-structured-demo-20260730", "nexusai-structured-demo-v2-20260730"))
		Expect(item.CleanupDisposition).To(Equal("preserve"))
	})

	DescribeTable("classifies retained collections for capability-aware selection without authorizing deletion",
		func(collection, purpose, disposition string) {
			item := classifyForensicCase(collection)
			Expect(item.Purpose).To(Equal(purpose))
			Expect(item.Visibility).To(Equal("system"))
			Expect(item.Selectable).To(BeTrue())
			Expect(item.CleanupDisposition).To(Equal(disposition))
			Expect(item.Warnings).NotTo(BeEmpty())
		},
		Entry("acceptance", "forensic-phase2-complete-acceptance-20260727", "acceptance_or_system", "hide_and_retain"),
		Entry("validation", "forensic-pakistan-cdr-validation-20260724", "acceptance_or_system", "hide_and_retain"),
		Entry("accidental analyst-named collection", "Forensic_Records_Analyst", "accidental_name_candidate", "verify_unreferenced_then_request_approval"),
		Entry("CDR specialist-name collection", "Communications_CDR_Analyst", "accidental_name_candidate", "verify_unreferenced_then_request_approval"),
		Entry("IPDR specialist-name collection", "network_ipdr_capture_analyst", "accidental_name_candidate", "verify_unreferenced_then_request_approval"),
		Entry("ANPR specialist-name collection", "Vehicle_ANPR_Geospatial_Analyst", "accidental_name_candidate", "verify_unreferenced_then_request_approval"),
		Entry("subscriber specialist-name collection", "Subscriber_Identity_Analyst", "accidental_name_candidate", "verify_unreferenced_then_request_approval"),
		Entry("tower specialist-name collection", "Tower_Location_Reference_Analyst", "accidental_name_candidate", "verify_unreferenced_then_request_approval"),
	)

	It("never promotes a system cleanup collection to the implicit analyst case", func() {
		cases, hidden, defaultCaseID := governedForensicCases([]string{
			"communications_cdr_analyst",
			"nexusai-forensic-demo",
			"forensic-phase2-complete-acceptance-20260727",
		}, true)
		Expect(cases).To(HaveLen(3))
		Expect(hidden).To(Equal(0))
		Expect(defaultCaseID).To(Equal("nexusai-forensic-demo"))
	})

	It("hides system collections from the default analyst discovery surface", func() {
		cases, hidden, defaultCaseID := governedForensicCases([]string{
			"communications_cdr_analyst",
			"nexusai-forensic-demo",
			"tower_location_reference_analyst",
		}, false)
		Expect(cases).To(HaveLen(1))
		Expect(hidden).To(Equal(2))
		Expect(cases[0].CaseID).To(Equal("nexusai-forensic-demo"))
		Expect(defaultCaseID).To(Equal("nexusai-forensic-demo"))
	})

	It("returns no implicit case when only system collections are accessible", func() {
		cases, hidden, defaultCaseID := governedForensicCases([]string{
			"communications_cdr_analyst",
			"forensic-phase2-complete-acceptance-20260727",
		}, true)
		Expect(cases).To(HaveLen(2))
		Expect(hidden).To(Equal(0))
		Expect(defaultCaseID).To(BeEmpty())
	})

	It("binds report and query bodies to the URL case only after validating both identifiers", func() {
		body := map[string]any{"target": "subscriber-42"}
		Expect(bindForensicCaseBodyScope(body, "case-authorized")).To(Succeed())
		Expect(body).To(HaveKeyWithValue("case_id", "case-authorized"))
		Expect(body).To(HaveKeyWithValue("collection_id", "case-authorized"))

		matching := map[string]any{"case_id": "case-authorized", "collection_id": "case-authorized"}
		Expect(bindForensicCaseBodyScope(matching, "case-authorized")).To(Succeed())

		caseMismatch := map[string]any{"case_id": "case-other", "collection_id": "case-authorized"}
		Expect(bindForensicCaseBodyScope(caseMismatch, "case-authorized")).To(MatchError("case_id does not match URL case"))
		Expect(caseMismatch).To(HaveKeyWithValue("case_id", "case-other"))
		Expect(caseMismatch).To(HaveKeyWithValue("collection_id", "case-authorized"))

		collectionMismatch := map[string]any{"case_id": "case-authorized", "collection_id": "collection-other"}
		Expect(bindForensicCaseBodyScope(collectionMismatch, "case-authorized")).To(MatchError("collection_id does not match URL case"))
		Expect(collectionMismatch).To(HaveKeyWithValue("collection_id", "collection-other"))
	})

	It("keeps legacy demos selectable for governed reconciliation", func() {
		item := classifyForensicCase("records-demo")
		Expect(item.CaseStatus).To(Equal("reconciliation_required"))
		Expect(item.Visibility).To(Equal("system"))
		Expect(item.Selectable).To(BeTrue())
		Expect(item.RetentionClass).To(Equal("hold_for_manifest_review"))
	})

	It("augments external manifest resources and preserves unavailable states", func() {
		manifest := map[string]any{"resources": []any{
			map[string]any{"resource": "kb_entries", "status": "external", "count": nil},
		}}
		augmentManifestResource(manifest, "kb_entries", 7, nil, "localai_kb")
		augmentManifestResource(manifest, "sources", 0, errors.New("source backend unavailable"), "localai_kb")
		resources := manifest["resources"].([]any)
		Expect(resources).To(HaveLen(2))
		Expect(resources[0].(map[string]any)).To(HaveKeyWithValue("count", int64(7)))
		Expect(resources[0].(map[string]any)).To(HaveKeyWithValue("status", "counted"))
		Expect(resources[1].(map[string]any)).To(HaveKeyWithValue("status", "unavailable"))
		Expect(resources[1].(map[string]any)["note"]).To(ContainSubstring("unavailable"))
	})

	It("fails closed before case-scoped evidence inspection when collection authority is unavailable", func() {
		recorder := httptest.NewRecorder()
		ctx := echo.New().NewContext(
			httptest.NewRequest(http.MethodGet, "/api/v1/forensics/cases/case-a/evidence/evidence-1", nil),
			recorder,
		)
		ctx.SetPath("/api/v1/forensics/cases/:case_id/evidence/:evidence_id")
		ctx.SetParamNames("case_id", "evidence_id")
		ctx.SetParamValues("case-a", "evidence-1")

		err := GetForensicCaseEvidenceV1Endpoint(nil)(ctx)
		var httpError *echo.HTTPError
		Expect(errors.As(err, &httpError)).To(BeTrue())
		Expect(httpError.Code).To(Equal(http.StatusServiceUnavailable))
	})

	It("binds media candidate reads to the URL case and preserves explicit candidates", func() {
		input := url.Values{
			"query_face_observation_id": {"face-1"},
			"candidate_evidence_id":     {"evidence-a", "evidence-b"},
			"authorization":             {"caller-supplied-value"},
			"untrusted":                 {"must-not-pass"},
		}
		values := forensicCaseMediaQuery(input, "case-a", []string{"query_face_observation_id", "candidate_evidence_id"}, true)
		Expect(values.Get("case_id")).To(Equal("case-a"))
		Expect(values.Get("collection_id")).To(Equal("case-a"))
		Expect(values["candidate_evidence_id"]).To(Equal([]string{"evidence-a", "evidence-b"}))
		Expect(values.Get("authorization")).To(Equal("explicit_case_evidence_scope"))
		Expect(values).NotTo(HaveKey("untrusted"))
	})

	It("proxies every authenticated platform discovery surface with trusted identity", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.Header.Get("Authorization")).To(Equal("Bearer discovery-token"))
			Expect(r.Header.Get("X-Forensic-Tenant-ID")).To(Equal("tenant-a"))
			Expect(r.Header.Get("X-Forensic-Actor-ID")).To(Equal("analyst-a"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"path":"` + r.URL.Path + `"}`))
		}))
		defer server.Close()

		previousURL, hadURL := os.LookupEnv("FORENSIC_RECORDS_API_URL")
		previousKey, hadKey := os.LookupEnv("FORENSIC_RECORDS_API_KEY")
		previousTenant, hadTenant := os.LookupEnv("FORENSIC_RECORDS_TENANT_ID")
		DeferCleanup(func() {
			if hadURL {
				_ = os.Setenv("FORENSIC_RECORDS_API_URL", previousURL)
			} else {
				_ = os.Unsetenv("FORENSIC_RECORDS_API_URL")
			}
			if hadKey {
				_ = os.Setenv("FORENSIC_RECORDS_API_KEY", previousKey)
			} else {
				_ = os.Unsetenv("FORENSIC_RECORDS_API_KEY")
			}
			if hadTenant {
				_ = os.Setenv("FORENSIC_RECORDS_TENANT_ID", previousTenant)
			} else {
				_ = os.Unsetenv("FORENSIC_RECORDS_TENANT_ID")
			}
		})
		Expect(os.Setenv("FORENSIC_RECORDS_API_URL", server.URL)).To(Succeed())
		Expect(os.Setenv("FORENSIC_RECORDS_API_KEY", "discovery-token")).To(Succeed())
		Expect(os.Setenv("FORENSIC_RECORDS_TENANT_ID", "tenant-a")).To(Succeed())

		handlers := []struct {
			section string
			handler echo.HandlerFunc
		}{
			{"adapters", ListForensicAdaptersV1Endpoint()},
			{"operations", ListForensicOperationsV1Endpoint()},
			{"agents", ListForensicSpecialistsV1Endpoint()},
			{"contracts", ListForensicContractsV1Endpoint()},
		}
		for _, item := range handlers {
			recorder := httptest.NewRecorder()
			ctx := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/api/v1/forensics/"+item.section, nil), recorder)
			ctx.Set("auth_user", &auth.User{ID: "analyst-a", Role: auth.RoleUser})
			Expect(item.handler(ctx)).To(Succeed())
			Expect(recorder.Code).To(Equal(http.StatusOK))
			Expect(recorder.Body.String()).To(MatchJSON(`{"path":"/api/v1/forensics/` + item.section + `"}`))
		}
	})
})
