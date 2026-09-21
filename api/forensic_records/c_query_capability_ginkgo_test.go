package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func cReference(id string) QueryCapabilityReferenceV1 {
	refs, err := loadQueryCapabilityReferences()
	Expect(err).NotTo(HaveOccurred())
	for _, c := range refs.Capabilities {
		if c.ID == id {
			return c
		}
	}
	Fail("missing capability " + id)
	return QueryCapabilityReferenceV1{}
}
func cContext() SemanticReadinessContextV1 {
	return SemanticReadinessContextV1{Authorized: true, ExecutorAvailable: true, Origin: "SOURCE_CANDIDATE", ScopeMode: "authorized_workspace", SemanticContracts: map[string]bool{"derived-text-search/v1": true}}
}

var _ = Describe("C query capability references", func() {
	It("preserves independent operation descriptor variant and certification counts", func() {
		refs, err := loadQueryCapabilityReferences()
		Expect(err).NotTo(HaveOccurred())
		Expect(refs.Capabilities).To(HaveLen(22))
		catalog, err := loadForensicPlatformCatalog()
		Expect(err).NotTo(HaveOccurred())
		Expect(catalog.Operations).To(HaveLen(104))
		Expect(catalog.Adapters).To(HaveLen(9))
		Expect(catalog.Agents).To(HaveLen(16))
		Expect(supportedQueryTemplates()).To(HaveLen(79))
		ledger, err := loadQueryVariantLedger()
		Expect(err).NotTo(HaveOccurred())
		Expect(ledger.Entries).To(HaveLen(214))
		levels := map[string]int{}
		for _, c := range operationCertificationByID() {
			levels[c.CertificationLevel]++
		}
		Expect(levels).To(Equal(map[string]int{"REGISTERED": 62, "SOURCE_VALIDATED": 12, "FIXTURE_CERTIFIED": 1, "PRODUCT_CERTIFIED": 4}))
		Expect(cReference("image.similarity").ReferenceKind).To(Equal("DIRECT_DESCRIPTOR"))
		Expect(cReference("face.candidates").ReferenceKind).To(Equal("DIRECT_DESCRIPTOR"))
	})
	DescribeTable("rejects invalid reference metadata", func(mutate func(*QueryCapabilityReferencesV1)) {
		refs, err := loadQueryCapabilityReferences()
		Expect(err).NotTo(HaveOccurred())
		mutate(&refs)
		Expect(validateQueryCapabilityReferences(refs)).To(HaveOccurred())
	},
		Entry("duplicate id", func(r *QueryCapabilityReferencesV1) { r.Capabilities[1].ID = r.Capabilities[0].ID }),
		Entry("deleted operation", func(r *QueryCapabilityReferencesV1) { r.Capabilities[0].OperationRef = "deleted.operation" }),
		Entry("wrong family", func(r *QueryCapabilityReferencesV1) { r.Capabilities[0].Family = "audio_intelligence" }),
		Entry("unknown semantic", func(r *QueryCapabilityReferencesV1) { r.Capabilities[0].Semantics = []string{"MAGIC"} }),
		Entry("duplicate semantic alias", func(r *QueryCapabilityReferencesV1) {
			c := r.Capabilities[0]
			c.ID = "other.alias"
			r.Capabilities = append(r.Capabilities, c)
		}),
		Entry("wrong direct descriptor", func(r *QueryCapabilityReferencesV1) { r.Capabilities[0].ReferenceKind = "DIRECT_DESCRIPTOR" }),
		Entry("unknown authority", func(r *QueryCapabilityReferencesV1) { r.Capabilities[0].Authority = "AI" }),
		Entry("unqualified input", func(r *QueryCapabilityReferencesV1) { r.Capabilities[0].InputTypes = []string{"evtx"} }),
		Entry("unknown language", func(r *QueryCapabilityReferencesV1) { r.Capabilities[0].LanguageCoverage["urdu"] = "PASS" }),
		Entry("unknown scope", func(r *QueryCapabilityReferencesV1) { r.Capabilities[0].ScopeModes = []string{"all_tenants"} }),
		Entry("unknown locator", func(r *QueryCapabilityReferencesV1) { r.Capabilities[0].LocatorTypes = []string{"invented"} }),
		Entry("unknown role", func(r *QueryCapabilityReferencesV1) { r.Capabilities[0].ProcessorRole = "magic" }),
		Entry("proof traversal", func(r *QueryCapabilityReferencesV1) { r.Capabilities[0].ProofRefs = []string{"reports/../../private"} }),
		Entry("missing B1 semantic reference", func(r *QueryCapabilityReferencesV1) {
			for i := range r.Capabilities {
				if r.Capabilities[i].OperationRef == "image.ocr_search" {
					r.Capabilities[i].SemanticContract = ""
				}
			}
		}),
	)
	DescribeTable("separates retained results from processor readiness", func(s CapabilityEvidenceStateV1, want string, query bool) {
		r := projectSemanticReadiness(cReference("transcript.phrase"), s, cContext())
		Expect(r.ResultState).To(Equal(want))
		Expect(r.QueryExistingResults).To(Equal(query))
		Expect(r.ProcessNewInput).To(BeFalse())
		Expect(r.SuggestionEligible).To(BeFalse())
		Expect(r.SearchCompleteness).To(Equal("REQUIRES_EXECUTION"))
	},
		Entry("current result", CapabilityEvidenceStateV1{Eligible: true, Results: true}, "COMPLETE_RESULTS", true),
		Entry("not run", CapabilityEvidenceStateV1{Eligible: true}, "NOT_RUN", false),
		Entry("complete zero", CapabilityEvidenceStateV1{Eligible: true, CompleteZero: true}, "COMPLETE_ZERO_RESULTS", true),
		Entry("failed", CapabilityEvidenceStateV1{Eligible: true, Failed: true}, "FAILED", false),
		Entry("failed after valid artifact", CapabilityEvidenceStateV1{Eligible: true, Failed: true, Results: true}, "COMPLETE_RESULTS", true),
		Entry("failed invalidates zero", CapabilityEvidenceStateV1{Eligible: true, Failed: true, CompleteZero: true}, "FAILED", false),
		Entry("partial", CapabilityEvidenceStateV1{Eligible: true, Incomplete: true}, "INCOMPLETE", false),
		Entry("unsupported input", CapabilityEvidenceStateV1{}, "UNAVAILABLE", false),
	)
	It("does not claim undeployed B1 semantics in a live baseline context", func() {
		ctx := cContext()
		ctx.Origin = "LIVE_BASELINE"
		ctx.SemanticContracts = nil
		r := projectSemanticReadiness(cReference("transcript.phrase"), CapabilityEvidenceStateV1{Eligible: true, Results: true}, ctx)
		Expect(r.QueryExistingResults).To(BeFalse())
		Expect(r.Reason).To(Equal("SEMANTIC_CONTRACT_UNDEPLOYED"))
	})
	It("accounts for every catalog-only operation without widening suggestions", func() {
		ledger, err := loadQueryVariantLedger()
		Expect(err).NotTo(HaveOccurred())
		noncatalog := map[string]bool{}
		for _, v := range ledger.Entries {
			if v.SourceCorpus != "operation_catalog" {
				noncatalog[v.CanonicalOperation] = true
			}
		}
		payload, err := os.ReadFile(filepath.Join("..", "..", "reports", "nxb21", "c-catalog-only-disposition-v1.json"))
		Expect(err).NotTo(HaveOccurred())
		var report struct {
			Records []struct {
				OperationID   string `json:"operation_id"`
				Disposition   string `json:"disposition"`
				NewSuggestion bool   `json:"new_suggestion_eligibility"`
			} `json:"records"`
		}
		Expect(json.Unmarshal(payload, &report)).To(Succeed())
		found := map[string]bool{}
		for _, v := range report.Records {
			Expect(found[v.OperationID]).To(BeFalse())
			found[v.OperationID] = true
			Expect(noncatalog[v.OperationID]).To(BeFalse())
			Expect(v.NewSuggestion).To(BeFalse())
			if semanticCertifications[v.OperationID].ExposureStatus == "engineering_only" {
				Expect(v.Disposition).To(Equal("INTERNAL_ONLY"))
			}
		}
		for _, t := range supportedQueryTemplates() {
			Expect(found[t.OperationID]).To(Equal(!noncatalog[t.OperationID]))
		}
		Expect(found).To(HaveLen(29))
	})
	It("requires all existing worker receipt predicates for fresh processing", func() {
		ctx := cContext()
		receipt := CapabilityProcessorReceiptV1{ContractVersion: "forensics.processor-readiness/v1", Role: "asr", State: "READY", CodePresent: true, RoleEnabled: true, RoleAdmitted: true, WorkerHealthy: true, ResourcePolicySatisfied: true, BackendAvailable: true, ModelAssetsVerified: true}
		ctx.ProcessorReceipts = map[string]CapabilityProcessorReceiptV1{"asr": receipt}
		project := func() SemanticCapabilityReadinessV1 {
			return projectSemanticReadiness(cReference("transcript.phrase"), CapabilityEvidenceStateV1{Eligible: true, Results: true}, ctx)
		}
		Expect(project().ProcessNewInput).To(BeTrue())
		receipt.RoleEnabled = false
		ctx.ProcessorReceipts["asr"] = receipt
		r := project()
		Expect(r.ProcessingState).To(Equal("RUNTIME_ROLE_DISABLED"))
		Expect(r.ProcessNewInput).To(BeFalse())
		Expect(r.QueryExistingResults).To(BeTrue())
		receipt.RoleEnabled = true
		receipt.ModelAssetsVerified = false
		ctx.ProcessorReceipts["asr"] = receipt
		Expect(project().ProcessingState).To(Equal("MODEL_REQUIRED"))
		receipt.ModelAssetsVerified = true
		receipt.BackendAvailable = false
		ctx.ProcessorReceipts["asr"] = receipt
		Expect(project().ProcessNewInput).To(BeFalse())
	})
	It("does not make TTS ready from ordinary normalized audio while ASR independently queries retained text", func() {
		families := materializeForensicCapabilities(CoverageSummary{}, []evidenceCapabilityCount{{Modality: "audio", Extension: "wav", Total: 7, Normalized: 7, Derived: 12, ArtifactTypes: []string{"forensics.audio-timestamp-segment/v1"}}})
		for _, f := range families {
			if f.ID == "tts_artifacts" {
				Expect(f.Availability).To(Equal("unavailable"))
				Expect(f.SuggestedQueries).To(BeEmpty())
			}
		}
		Expect(projectSemanticReadiness(cReference("transcript.phrase"), CapabilityEvidenceStateV1{Eligible: true, Results: true}, cContext()).QueryExistingResults).To(BeTrue())
	})
	It("fails closed for engineering descriptors and unauthorized contexts", func() {
		c := cReference("image.similarity")
		c.OperationRef = "image.compare"
		r := projectSemanticReadiness(c, CapabilityEvidenceStateV1{Eligible: true, Results: true}, cContext())
		Expect(r.Reason).To(Equal("ENGINEERING_ONLY"))
		Expect(r.QueryExistingResults).To(BeFalse())
		ctx := cContext()
		ctx.Authorized = false
		r = projectSemanticReadiness(cReference("transcript.phrase"), CapabilityEvidenceStateV1{Eligible: true, Results: true}, ctx)
		Expect(r.Reason).To(Equal("UNAUTHORIZED"))
	})
	It("requires the governed query and compatible candidate set for image and face similarity", func() {
		for _, id := range []string{"image.similarity", "face.candidates"} {
			ctx := cContext()
			c := cReference(id)
			s := CapabilityEvidenceStateV1{Eligible: true, Results: true}
			Expect(projectSemanticReadiness(c, s, ctx).QueryExistingResults).To(BeFalse())
			ctx.ValidatedSimilarity = map[string]bool{id: true}
			Expect(projectSemanticReadiness(c, s, ctx).QueryExistingResults).To(BeTrue())
		}
	})
	It("projects OCR image ANPR and video ANPR independently including completed zero", func() {
		for _, id := range []string{"ocr.phrase", "image.plate", "video.group_plate"} {
			c := cReference(id)
			ctx := cContext()
			if id == "video.group_plate" || id == "image.plate" {
				ctx.ScopeMode = "selected_evidence"
			}
			Expect(projectSemanticReadiness(c, CapabilityEvidenceStateV1{Eligible: true, Results: true}, ctx).QueryExistingResults).To(BeTrue())
			r := projectSemanticReadiness(c, CapabilityEvidenceStateV1{Eligible: true, CompleteZero: true}, ctx)
			Expect(r.ResultState).To(Equal("COMPLETE_ZERO_RESULTS"))
			Expect(r.QueryExistingResults).To(BeTrue())
			ctx.ProcessorReceipts = map[string]CapabilityProcessorReceiptV1{"image_anpr": {ContractVersion: "forensics.processor-readiness/v1", Role: "image_anpr", State: "READY", CodePresent: true, RoleEnabled: true, RoleAdmitted: true, WorkerHealthy: true, ResourcePolicySatisfied: true, BackendAvailable: true, ModelAssetsVerified: true}}
			r = projectSemanticReadiness(c, CapabilityEvidenceStateV1{Eligible: true}, ctx)
			Expect(r.ProcessNewInput).To(Equal(id == "image.plate"))
			Expect(r.QueryExistingResults).To(BeFalse())
		}
	})
	It("does not use selected NOT_RUN for workspace readiness", func() {
		c := cReference("transcript.phrase")
		ctx := cContext()
		ctx.ScopeMode = "selected_evidence"
		Expect(projectSemanticReadiness(c, CapabilityEvidenceStateV1{Eligible: true}, ctx).QueryExistingResults).To(BeFalse())
		ctx.ScopeMode = "authorized_workspace"
		Expect(projectSemanticReadiness(c, CapabilityEvidenceStateV1{Eligible: true, Results: true}, ctx).QueryExistingResults).To(BeTrue())
	})
	It("does not use unrelated artifacts in the existing planner projection", func() {
		template, _ := queryTemplateByName("audio_transcript_search")
		ctx := CapabilityProjectionContextV1{Authorized: true, RuntimeAvailable: true, FamilyStates: map[string]WorkspaceCapabilityStateV1{"audio_and_stt": {DerivedArtifacts: 1, ArtifactTypes: []string{"forensics.audio-observation/v1"}}}}
		Expect(capabilityFromTemplate(template, ctx).Availability.Executable).To(BeFalse())
		ctx.FamilyStates["audio_and_stt"] = WorkspaceCapabilityStateV1{DerivedArtifacts: 1, ArtifactTypes: []string{"forensics.audio-timestamp-segment/v1"}}
		Expect(capabilityFromTemplate(template, ctx).Availability.Executable).To(BeTrue())
	})
	It("serializes additive source metadata without claiming live deployment or writing Activity", func() {
		req := httptest.NewRequest(http.MethodGet, "/query/capabilities?tenant_id=test&collection_id=case", nil)
		w := httptest.NewRecorder()
		forensicCapabilitiesHandler(nil).ServeHTTP(w, req)
		Expect(w.Code).To(Equal(200))
		var response map[string]any
		Expect(json.Unmarshal(w.Body.Bytes(), &response)).To(Succeed())
		Expect(response).To(HaveKey("families"))
		Expect(response).To(HaveKey("query_capability_readiness"))
		Expect(response["deployment_attestation"]).To(ContainSubstring("NOT_ATTESTED"))
		if dir := os.Getenv("NXB21_C_EXPORT_DIR"); dir != "" {
			Expect(os.WriteFile(filepath.Join(dir, "source-snapshot.json"), w.Body.Bytes(), 0600)).To(Succeed())
		}
	})
	It("rejects malformed selected scope before database access", func() {
		w := httptest.NewRecorder()
		forensicCapabilitiesHandler(nil).ServeHTTP(w, httptest.NewRequest("GET", "/query/capabilities?collection_id=case&evidence_id=invalid", nil))
		Expect(w.Code).To(Equal(400))
	})
	It("measures metadata projection without inference", func() {
		refs, err := loadQueryCapabilityReferences()
		Expect(err).NotTo(HaveOccurred())
		start := time.Now()
		for _, c := range refs.Capabilities {
			projectSemanticReadiness(c, CapabilityEvidenceStateV1{}, cContext())
		}
		GinkgoWriter.Printf("C projection %d capabilities: %s (diagnostic, no SLA)\n", len(refs.Capabilities), time.Since(start))
	})
	DescribeTable("classifies advertised formats without parser promotion", func(ext, want string) {
		refs, err := loadQueryCapabilityReferences()
		Expect(err).NotTo(HaveOccurred())
		found := false
		for _, b := range refs.InputBoundaries {
			if b.InputType == ext && b.Category != "tts_artifacts" {
				found = true
				Expect(b.State).To(Equal(want))
			}
		}
		Expect(found).To(BeTrue())
	},
		Entry("TXT", "txt", "SUPPORTED_INGEST_AND_ANALYSIS"), Entry("PDF", "pdf", "SUPPORTED_INGEST_AND_ANALYSIS"), Entry("DOCX", "docx", "SUPPORTED_INGEST_AND_ANALYSIS"),
		Entry("MD", "md", "NOT_QUALIFIED"), Entry("YAML", "yaml", "NOT_QUALIFIED"), Entry("CSV", "csv", "SUPPORTED_INGEST_AND_ANALYSIS"), Entry("TSV", "tsv", "SUPPORTED_INGEST_AND_ANALYSIS"), Entry("XLSX", "xlsx", "SUPPORTED_INGEST_AND_ANALYSIS"), Entry("Parquet", "parquet", "SUPPORTED_INGEST_AND_ANALYSIS"),
		Entry("EVTX", "evtx", "NOT_QUALIFIED"), Entry("OFX", "ofx", "NOT_QUALIFIED"), Entry("MT940", "mt940", "NOT_QUALIFIED"), Entry("SRT", "srt", "NOT_QUALIFIED"), Entry("VTT", "vtt", "NOT_QUALIFIED"), Entry("ASS", "ass", "NOT_QUALIFIED"),
		Entry("PCAP", "pcap", "SUPPORTED_INGEST_INVENTORY_ONLY"), Entry("SQLite", "sqlite", "SUPPORTED_INGEST_INVENTORY_ONLY"), Entry("ZIP", "zip", "SUPPORTED_INGEST_INVENTORY_ONLY"), Entry("TAR", "tar", "SUPPORTED_INGEST_INVENTORY_ONLY"),
		Entry("HEIC", "heic", "NOT_QUALIFIED"), Entry("SVG", "svg", "NOT_QUALIFIED"), Entry("raw", "raw", "NOT_QUALIFIED"), Entry("PNG", "png", "SUPPORTED_INGEST_AND_ANALYSIS"), Entry("WAV", "wav", "SUPPORTED_INGEST_AND_ANALYSIS"), Entry("MP4", "mp4", "SUPPORTED_INGEST_AND_ANALYSIS"),
	)
})
