package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/mudler/LocalAI/pkg/functions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deterministic first hybrid planner", func() {
	It("keeps every wire enum choice valid through LocalAI grammar conversion and decoding", func() {
		ts := []HybridTupleV1{{ID: "generic.alpha/FILTER/authorized_workspace"}, {ID: "generic.beta/AGGREGATE/authorized_workspace"}}
		schema := hybridResidualSchema(ts)
		Expect(schema["required"]).To(Equal([]string{"decision"}))
		props := schema["properties"].(map[string]any)
		Expect(props).To(HaveLen(1))
		raw, err := json.Marshal(schema)
		Expect(err).NotTo(HaveOccurred())
		var item functions.Item
		Expect(json.Unmarshal(raw, &item)).To(Succeed())
		grammar, err := (functions.JSONFunctionStructure{AnyOf: []functions.Item{item}}).Grammar()
		Expect(err).NotTo(HaveOccurred())
		for _, decision := range props["decision"].(map[string]any)["enum"].([]string) {
			Expect(grammar).To(ContainSubstring(decision))
			wire, _ := json.Marshal(map[string]string{"decision": decision})
			p, err := decodeHybridResidual(wire, ts)
			Expect(err).NotTo(HaveOccurred())
			if strings.HasPrefix(decision, "CLARIFY:") {
				Expect(p.SelectedTupleID).To(BeEmpty())
				Expect(p.AmbiguityState).To(Equal("NEEDS_CLARIFICATION"))
				Expect(p.ClarificationCode).NotTo(BeEmpty())
			} else {
				Expect(string(p.SelectedTupleID)).To(Equal(decision))
				Expect(p.AmbiguityState).To(Equal("RESOLVED"))
				Expect(p.ClarificationCode).To(BeEmpty())
			}
		}
		for _, invalid := range []string{`{"decision":null}`, `{"decision":""}`, `{"decision":"unknown"}`, `{"decision":"CLARIFY:AMBIGUOUS_INTENT","decision":"CLARIFY:INSUFFICIENT_FACTS"}`, `{"decision":"CLARIFY:AMBIGUOUS_INTENT","ambiguity_state":"RESOLVED"}`, `{"ambiguity_state":"NEEDS_CLARIFICATION","clarification_code":"","confidence":0,"contract_version":"forensics.hybrid-residual/v1","selected_tuple_id":"generic.alpha/FILTER/authorized_workspace"}`} {
			_, err := decodeHybridResidual([]byte(invalid), ts)
			Expect(err).To(HaveOccurred(), invalid)
		}
	})
	workspace := func(q string) hybridQueryRequest {
		return hybridQueryRequest{Query: q, TenantID: "t", UserID: "u", CollectionID: "c", QueryScope: queryEvidenceScope{Kind: "authorized_workspace"}}
	}
	DescribeTable("preserves complete source identifiers and offsets", func(value, want string) {
		q := "Find events for " + value
		x := hybridIdentifierSpans(q)
		Expect(x).To(HaveLen(1))
		Expect(x[0].Raw).To(Equal(value))
		Expect(x[0].Canonical).To(Equal(want))
		Expect(q[x[0].Start:x[0].End]).To(Equal(value))
		Expect(extractTargets(q)).To(Equal([]string{want}))
	}, Entry("analyst", "analyst-k29", "analyst-k29"), Entry("user", "user-j77", "user-j77"), Entry("account", "account-f18", "account-f18"), Entry("principal", "principal-z92", "principal-z92"), Entry("phone", "03184567123", "03184567123"), Entry("ip", "198.51.100.47", "198.51.100.47"), Entry("plate", "LX-8432", "LX-8432"), Entry("alphanumeric", "ACCT_8BX4", "ACCT_8BX4"))
	It("does not capture ordinary prose", func() {
		Expect(hybridIdentifierSpans("analyst reviewed principal account user evidence")).To(BeEmpty())
	})
	DescribeTable("binds literal representation", func(query, capability, representation string) {
		req, f := extractHybridFacts(workspace(query))
		ts := buildHybridTuples(req, f)
		Expect(ts).To(HaveLen(1))
		Expect(ts[0].Capability).To(Equal(capability))
		Expect(ts[0].Representation).To(Equal(representation))
		out, err := assembleHybridPlan(req, f, ts[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(out.TextQuery.LiteralText).To(Equal(f.TextQuery.LiteralText))
		Expect(out.TextQuery.Representation).To(Equal(representation))
	}, Entry("Urdu document", `تمام دستاویزات میں "نئی رسید" تلاش کریں۔`, "document.phrase", "raw"), Entry("English document", `Search documents for "dispatch folio"`, "document.phrase", "raw"), Entry("Urdu transcript", `تمام ریکارڈنگز میں "نئی رسید" تلاش کریں۔`, "transcript.phrase", "raw"), Entry("Roman Urdu transcript", `Roman Urdu transcript mein "rasta khol do" dhoondo`, "roman_urdu.phrase", "roman_derivative"), Entry("mixed transcript", `Roman Urdu transcript میں "اصل رسید" تلاش کریں`, "transcript.phrase", "raw"))
	It("rejects representation incompatible with document capability", func() {
		req, f := extractHybridFacts(workspace(`Search document for "closed lane"`))
		ts := buildHybridTuples(req, f)
		ts[0].Representation = "roman_derivative"
		_, err := assembleHybridPlan(req, f, ts[0])
		Expect(err).To(HaveOccurred())
	})
	It("resolves explicit Urdu endpoint summary to a compatible tuple", func() {
		req, f := extractHybridFacts(workspace("اینڈ پوائنٹ 198.51.100.93 کے سیشنز کا خلاصہ دیں۔"))
		ts := buildHybridTuples(req, f)
		Expect(ts).To(HaveLen(1))
		Expect(ts[0].Capability).To(Equal("ipdr.endpoint"))
		Expect(ts[0].Semantic).To(Equal("AGGREGATE"))
	})
	It("keeps capabilities paired with their actual semantics", func() {
		req, f := extractHybridFacts(workspace("Inspect 198.51.100.94"))
		ts := buildHybridTuples(req, f)
		Expect(len(ts)).To(BeNumerically(">", 1))
		for _, t := range ts {
			r, ok := dynamicCapabilityReference(t.Capability)
			Expect(ok).To(BeTrue())
			Expect(r.Semantics).To(ContainElement(t.Semantic))
			Expect(t.Capability + "/" + t.Semantic).NotTo(Equal("ipdr.sessions/AGGREGATE"))
		}
	})
	It("retains capability while replacing the target and dates", func() {
		req := workspace("Keep the contact ranking but replace the subject with 03185461237.")
		req.ConversationContext = queryConversationContext{ContractVersion: followUpContextContractV1, TenantID: "t", UserID: "u", CollectionID: "c", Template: "frequent_contacts", Target: "03185461238", DateFrom: "2026-01-01T00:00:00Z", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}
		out, status := resolveHybridPlanner(context.Background(), config{}, req)
		Expect(status).To(Equal("hybrid_deterministic_plan"))
		Expect(out.HybridAudit.Facts.Transition).To(Equal("TARGET_REPLACED"))
		Expect(out.DynamicProposal.QueryCapabilityID).To(Equal("cdr.frequent_contacts"))
		Expect(out.Target).To(Equal("03185461237"))
		Expect(out.DateFrom).To(Equal(req.ConversationContext.DateFrom))
		Expect(out.HybridAudit.ModelDecision).To(BeNil())
	})
	It("rejects foreign and expired context inheritance", func() {
		req := workspace("same analysis for 03185461239")
		req.ConversationContext = queryConversationContext{TenantID: "foreign", Template: "frequent_contacts", Target: "03185461238"}
		_, f := extractHybridFacts(req)
		Expect(f.PriorCapability).To(BeEmpty())
		Expect(f.ExplicitCapability).To(BeEmpty())
		req.ConversationContext.TenantID = "t"
		req.ConversationContext.ExpiresAt = "2000-01-01T00:00:00Z"
		_, f = extractHybridFacts(req)
		Expect(f.PriorCapability).To(BeEmpty())
	})
	DescribeTable("owns explicit multilingual counts", func(q string) { _, f := extractHybridFacts(workspace(q)); Expect(f.TopK).To(Equal(5)) }, Entry("English", "five most frequent contacts for 03185461230"), Entry("Roman Urdu", "paanch sab se zyada rabtay 03185461230"), Entry("Urdu", "پانچ زیادہ رابطے 03185461230"))
	It("assembles explicit dates and direction without model roundtrip", func() {
		req := workspace("top 5 frequent contacts for 03185461231 outgoing between 2026-11-03 and 2026-11-05")
		out, status := resolveHybridPlanner(context.Background(), config{}, req)
		Expect(status).To(Equal("hybrid_deterministic_plan"))
		Expect(out.DateFrom).To(Equal("2026-11-03T00:00:00Z"))
		Expect(out.DateTo).To(Equal("2026-11-06T00:00:00Z"))
		Expect(out.Direction).To(Equal("OUTGOING"))
		Expect(out.Limit).To(Equal(5))
	})
	It("keeps principal intact through final execution request", func() {
		out, status := resolveHybridPlanner(context.Background(), config{}, workspace("Filter failed login events for analyst-m36"))
		Expect(status).To(Equal("hybrid_deterministic_plan"))
		Expect(out.Target).To(Equal("analyst-m36"))
		Expect(out.HybridAudit.FinalPlan.Target).To(Equal(out.Target))
	})
	DescribeTable("refuses unsupported and conflicting requests without inference", func(q string) { _, f := extractHybridFacts(workspace(q)); Expect(f.State).NotTo(BeEmpty()) }, Entry("SQL", "Run SELECT * FROM forensic.records"), Entry("scope", "Read a different case"), Entry("budget", "top 99999 contacts"), Entry("source missing", `Search this document for "closed lane"`), Entry("ambiguous literal", `Find "alpha" or "beta" in a document`), Entry("unsupported format", "Analyze this EVTX file"))
	It("uses one exclusive residual decision and excludes source facts", func() {
		req, f := extractHybridFacts(workspace("Inspect 198.51.100.95"))
		raw, err := json.Marshal(hybridResidualSchema(buildHybridTuples(req, f)))
		Expect(err).NotTo(HaveOccurred())
		for _, s := range []string{"\"target\"", "literal_text", "scope_intent", "date_from", "text_representation"} {
			Expect(string(raw)).NotTo(ContainSubstring(s))
		}
	})
	It("rejects malformed, unknown, contradictory and fact-overriding residuals", func() {
		req, f := extractHybridFacts(workspace("Inspect 198.51.100.96"))
		ts := buildHybridTuples(req, f)
		p := map[string]string{"decision": string(ts[0].ID)}
		raw, _ := json.Marshal(p)
		_, err := decodeHybridResidual(raw, ts)
		Expect(err).NotTo(HaveOccurred())
		for _, bad := range []string{`{}`, string(raw) + `{}`, strings.Replace(string(raw), string(ts[0].ID), "unknown", 1), strings.TrimSuffix(string(raw), "}") + `,"target":"evil"}`, strings.TrimSuffix(string(raw), "}") + `,"scope":"other_case"}`} {
			_, err := decodeHybridResidual([]byte(bad), ts)
			Expect(err).To(HaveOccurred(), bad)
		}
	})
	It("selects a residual tuple and records all three layers", func() {
		req := workspace("Inspect 198.51.100.97")
		req.SynthesisModel = "qwen3-4b-instruct-2507-q4km-nxb21d-dev"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]any
			Expect(json.NewDecoder(r.Body).Decode(&payload)).To(Succeed())
			Expect(payload["max_tokens"]).To(Equal(float64(512)))
			Expect(payload["temperature"]).To(Equal(float64(0)))
			p := map[string]string{"decision": "ipdr.endpoint/AGGREGATE/authorized_workspace"}
			raw, _ := json.Marshal(p)
			json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": string(raw)}}}})
		}))
		defer server.Close()
		out, status := resolveHybridPlanner(context.Background(), config{LocalAIURL: server.URL}, req)
		Expect(status).To(Equal("hybrid_model_plan"))
		Expect(out.Target).To(Equal("198.51.100.97"))
		Expect(out.HybridAudit.DecisionSource).To(Equal("MODEL_DECISION"))
		Expect(out.HybridAudit.RawResidual).NotTo(BeEmpty())
		Expect(out.HybridAudit.Facts.Identifiers).To(HaveLen(1))
		Expect(out.HybridAudit.FinalPlan).NotTo(BeNil())
		Expect(out.HybridAudit.Reconciliation).To(BeEmpty())
	})
	It("accepts bounded ambiguity without constructing a plan", func() {
		req, f := extractHybridFacts(workspace("Inspect 198.51.100.98"))
		p := map[string]string{"decision": "CLARIFY:AMBIGUOUS_INTENT"}
		raw, _ := json.Marshal(p)
		out, err := decodeHybridResidual(raw, buildHybridTuples(req, f))
		Expect(err).NotTo(HaveOccurred())
		Expect(out.SelectedTupleID).To(BeEmpty())
	})
	It("binds selected source-time separately from calendar dates", func() {
		req := workspace("Show this recording from 00:12 to 00:28")
		req.EvidenceID = "10000000-0000-4000-8000-000000000001"
		req.QueryScope = queryEvidenceScope{Kind: "selected_evidence", SourceFamily: "audio"}
		out, status := resolveHybridPlanner(context.Background(), config{}, req)
		Expect(status).To(Equal("hybrid_deterministic_plan"))
		Expect(*out.StartSeconds).To(Equal(float64(12)))
		Expect(*out.EndSeconds).To(Equal(float64(28)))
		Expect(out.DateFrom).To(BeEmpty())
		Expect(out.DynamicProposal.ScopeIntent).To(Equal("selected_evidence"))
	})
	It("does not offer a target-required operation without a target", func() {
		req, f := extractHybridFacts(workspace("subscriber lookup"))
		Expect(buildHybridTuples(req, f)).To(BeEmpty())
	})
	It("rejects forged authorization and operation tuples", func() {
		req, f := extractHybridFacts(workspace("network endpoint 198.51.100.49"))
		tuple := buildHybridTuples(req, f)[0]
		forged := f
		forged.CollectionID = "foreign"
		_, err := assembleHybridPlan(req, forged, tuple)
		Expect(err).To(HaveOccurred())
		tuple.Operation = "shell.exec"
		_, err = assembleHybridPlan(req, f, tuple)
		Expect(err).To(HaveOccurred())
	})
	DescribeTable("rejects incomplete and corrupt model transport", func(body, want string) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		defer server.Close()
		req := workspace("Inspect 198.51.100.51")
		req.SynthesisModel = "qwen3-4b-instruct-2507-q4km-nxb21d-dev"
		out, status := resolveHybridPlanner(context.Background(), config{LocalAIURL: server.URL}, req)
		Expect(status).To(Equal(want))
		Expect(out.HybridAudit.FinalPlan).To(BeNil())
	}, Entry("invalid UTF8", string([]byte{0xff}), "malformed_completion"), Entry("invalid JSON", "not json", "malformed_completion"), Entry("length", `{"choices":[{"finish_reason":"length","message":{"content":"{}"}}]}`, "incomplete_completion"), Entry("extra model fields", `{"choices":[{"finish_reason":"stop","message":{"content":"{\"scope\":\"foreign\"}"}}]}`, "malformed_residual"))
})
