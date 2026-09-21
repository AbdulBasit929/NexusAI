package main

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/mudler/LocalAI/pkg/forensictext"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("D dynamic capability planner", func() {
	proposalFor := func(req hybridQueryRequest, ref QueryCapabilityReferenceV1) DynamicPlannerProposalV1 {
		semantic := ref.Semantics[0]
		proposal := DynamicPlannerProposalV1{ContractVersion: dynamicPlannerProposalV1, QueryCapabilityID: ref.ID, Semantic: semantic, ScopeIntent: plannerScopeIntent(req), Confidence: .9}
		if containsString([]string{"EXACT_VALUE", "EXACT_PHRASE", "PHRASE_CONTAINS", "TOKEN_SEARCH"}, semantic) && strings.Contains(ref.ID, "phrase") {
			req.Query = "Find \"alpha beta\" in this evidence"
			proposal.LiteralText = "alpha beta"
			proposal.Parameters.TextQuery = &forensictext.Query{LiteralText: "alpha beta", MatchSemantic: forensictext.Semantic(semantic)}
		}
		return proposal
	}

	It("validates every C semantic capability and derives its registered reference", func() {
		refs, err := loadQueryCapabilityReferences()
		Expect(err).NotTo(HaveOccurred())
		Expect(refs.Capabilities).To(HaveLen(22))
		for _, ref := range refs.Capabilities {
			req := hybridQueryRequest{Query: "Inspect current authorized evidence", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
			if containsString(ref.ScopeModes, "selected_evidence") && !containsString(ref.ScopeModes, "authorized_workspace") {
				req.QueryScope.Kind, req.EvidenceID = string(EvidenceScopeSelected), "10000000-0000-4000-8000-000000000001"
			}
			proposal := proposalFor(req, ref)
			if ref.ID == "image.plate" {
				req.Query = "Find plate ZX-615 in this image"
				proposal.LiteralText = "ZX-615"
				proposal.Target = "ZX-615"
			}
			if proposal.LiteralText != "" && ref.ID != "image.plate" {
				req.Query = "Find \"alpha beta\" in this evidence"
			}
			resolved, err := validateDynamicPlannerProposal(req, proposal)
			Expect(err).NotTo(HaveOccurred(), ref.ID)
			Expect(resolved.OperationRef).To(Equal(ref.OperationRef))
		}
	})

	It("never exposes catalog-only operations to the model", func() {
		req := hybridQueryRequest{Query: "please inspect this", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		candidates := dynamicCapabilityCandidates(req)
		Expect(candidates).NotTo(BeEmpty())
		Expect(candidates).To(HaveLen(20))
		for _, candidate := range candidates {
			Expect(candidate.ID).NotTo(Equal("forensics.raw_sql"))
		}
		Expect(languageAssistanceCandidates(req.Query)).To(HaveLen(79)) // legacy API remains separate
	})

	It("rejects unknown capabilities, rewritten literals, invented identifiers and huge top-k", func() {
		req := hybridQueryRequest{Query: `Find "اصل عبارت" for plate QR71ABC and top 5`, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		base := DynamicPlannerProposalV1{ContractVersion: dynamicPlannerProposalV1, QueryCapabilityID: "cross_family.identifiers", Semantic: "COMPOSE", ScopeIntent: "authorized_workspace", Target: "QR71ABC", Confidence: .8}
		cases := []DynamicPlannerProposalV1{base, base, base, base}
		cases[0].QueryCapabilityID = "invented.operation"
		cases[1].LiteralText = "بدلی عبارت"
		cases[2].Target = "QR71ABD"
		cases[3].TopK = 1000000
		for _, proposal := range cases {
			_, err := validateDynamicPlannerProposal(req, proposal)
			Expect(err).To(HaveOccurred())
		}
	})

	It("rejects unknown JSON fields and contradictory typed targets", func() {
		_, err := decodeDynamicPlannerProposal([]byte(`{"contract_version":"forensics.dynamic-capability-proposal/v1","query_capability_id":"cdr.identifier_lookup","semantic":"EXACT_VALUE","scope_intent":"authorized_workspace","parameters":{},"confidence":0.9,"sql":"select 1"}`))
		Expect(err).To(HaveOccurred())
		req := hybridQueryRequest{Query: "Find 07123456789", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		p := DynamicPlannerProposalV1{ContractVersion: dynamicPlannerProposalV1, QueryCapabilityID: "cdr.identifier_lookup", Semantic: "EXACT_VALUE", ScopeIntent: "authorized_workspace", Target: "07123456789", Parameters: LanguageAssistanceParametersV1{Target: "07999999999"}, Confidence: .9}
		_, err = validateDynamicPlannerProposal(req, p)
		Expect(err).To(HaveOccurred())
	})

	DescribeTable("preserves one authoritative plate span across languages",
		func(question, plate string) {
			authority, err := reconcileSingleExactIdentifier(question, "image.plate", "EXACT_VALUE", plate, []string{plate})
			Expect(err).NotTo(HaveOccurred())
			Expect(authority.ModelProposalStatus).To(Equal("PASS"))
			Expect(authority.Reconciled).To(BeFalse())
			Expect(authority.FinalLiteralText).To(Equal(plate))
			Expect(authority.FinalEntities).To(Equal([]string{plate}))
		},
		Entry("English", "Find plate KQ-731 in this image", "KQ-731"),
		Entry("Urdu", "اس تصویر میں نمبر پلیٹ MN-4829 تلاش کریں", "MN-4829"),
		Entry("Roman Urdu", "is tasveer mein R-9042 number dhoondo", "R-9042"),
		Entry("mixed", "Selected image میں plate GH-305K check کریں", "GH-305K"),
	)

	It("reconciles split entities while preserving the failed model proposal", func() {
		authority, err := reconcileSingleExactIdentifier("Look across this image for plate ISB-5907.", "image.plate", "EXACT_VALUE", "ISB-5907", []string{"ISB-590", "5907"})
		Expect(err).NotTo(HaveOccurred())
		Expect(authority.ModelProposalStatus).To(Equal("FAIL_ATOMIC_IDENTIFIER"))
		Expect(authority.MismatchReasons).To(ContainElements("entity_count_not_one", "atomic_entity_not_exact_source_span"))
		Expect(authority.ModelEntities).To(Equal([]string{"ISB-590", "5907"}))
		Expect(authority.FinalEntities).To(Equal([]string{"ISB-5907"}))
		Expect(authority.Reconciled).To(BeTrue())
	})

	DescribeTable("reconciles exact identifier corruption without hiding it",
		func(literal string, entities []string, reason string) {
			authority, err := reconcileSingleExactIdentifier("Find exact plate ABC-1234 in this image", "image.plate", "EXACT_VALUE", literal, entities)
			Expect(err).NotTo(HaveOccurred())
			Expect(authority.ModelProposalStatus).To(Equal("FAIL_ATOMIC_IDENTIFIER"))
			Expect(authority.MismatchReasons).To(ContainElement(reason))
			Expect(authority.FinalLiteralText).To(Equal("ABC-1234"))
			Expect(authority.FinalEntities).To(Equal([]string{"ABC-1234"}))
		},
		Entry("prefix suffix split", "ABC-1234", []string{"ABC-123", "1234"}, "entity_count_not_one"),
		Entry("correct entity wrong literal", "ABC1234", []string{"ABC-1234"}, "literal_not_exact_source_span"),
		Entry("drops identifier suffix", "ABC-123", []string{"ABC-123"}, "atomic_entity_not_exact_source_span"),
		Entry("removes punctuation", "ABC1234", []string{"ABC1234"}, "atomic_entity_not_exact_source_span"),
		Entry("invents a second identifier", "ABC-1234", []string{"ABC-1234", "ZZ-999"}, "entity_count_not_one"),
	)

	It("fails closed on multiple image.plate candidates and ignores ordinary prose", func() {
		_, err := reconcileSingleExactIdentifier("Find plates ABC-1234 and QZ-88 in this image", "image.plate", "EXACT_VALUE", "ABC-1234", []string{"ABC-1234"})
		Expect(err).To(MatchError(ContainSubstring("multiple candidate spans require clarification")))
		Expect(authoritativeExactIdentifierSpans("find the ordinary phrase in this selected image", "image.plate")).To(BeEmpty())
	})

	It("leaves document and transcript phrase planning outside exact identifier authority", func() {
		for _, capability := range []string{"document.phrase", "transcript.phrase"} {
			authority, err := reconcileSingleExactIdentifier("Find alpha-beta in this source", capability, "PHRASE_CONTAINS", "alpha-beta", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(authority).To(BeNil())
		}
	})

	DescribeTable("preserves one explicit phrase span across languages",
		func(question, capability, literal string) {
			proposal := DynamicPlannerProposalV1{QueryCapabilityID: capability, Semantic: "PHRASE_CONTAINS", LiteralText: literal, Parameters: LanguageAssistanceParametersV1{TextQuery: &forensictext.Query{LiteralText: literal, MatchSemantic: forensictext.PhraseContains}}}
			authority, err := reconcileExplicitPhraseLiteral(question, proposal)
			Expect(err).NotTo(HaveOccurred())
			Expect(authority.ModelProposalStatus).To(Equal("PASS"))
			Expect(authority.SourceLiteral).To(Equal(literal))
			Expect(authority.FinalLiteral).To(Equal(literal))
			Expect(authority.SourceCodepoints).To(Equal(authority.FinalCodepoints))
			Expect(authority.Reconciled).To(BeFalse())
		},
		Entry("English document", `Search this document for "sealed loading register".`, "document.phrase", "sealed loading register"),
		Entry("Urdu document", `اس دستاویز میں "سامان دوبارہ شمار کرو" تلاش کریں۔`, "document.phrase", "سامان دوبارہ شمار کرو"),
		Entry("Roman Urdu transcript", `recording mein "darwaza foran kholo" dhoondo`, "transcript.phrase", "darwaza foran kholo"),
		Entry("mixed Urdu literal", `Search this document for "ادائیگی روک دو" ابھی۔`, "document.phrase", "ادائیگی روک دو"),
		Entry("mixed English literal", `اس recording میں "checkpoint moved west" تلاش کریں۔`, "transcript.phrase", "checkpoint moved west"),
	)

	It("uses the source-grounded Urdu phrase instead of model homoglyphs", func() {
		question := `اس دستاویز میں "سامان دوبارہ شمار کرو" تلاش کریں۔`
		corrupted := "س\u0430\u043cان دوبارہ شمار کرو"
		proposal := DynamicPlannerProposalV1{QueryCapabilityID: "document.phrase", Semantic: "PHRASE_CONTAINS", LiteralText: corrupted, Parameters: LanguageAssistanceParametersV1{TextQuery: &forensictext.Query{LiteralText: corrupted, MatchSemantic: forensictext.PhraseContains}}}
		resolved, authority, err := applyPhraseLiteralAuthority(hybridQueryRequest{Query: question}, proposal)
		Expect(err).NotTo(HaveOccurred())
		Expect(authority.ModelLiteral).To(Equal(corrupted))
		Expect(authority.ScriptFamilyMismatch).To(BeTrue())
		Expect(authority.MismatchReasons).To(ContainElements("literal_not_exact_source_span", "model_script_family_mismatch"))
		Expect(resolved.LiteralText).To(Equal("سامان دوبارہ شمار کرو"))
		Expect(resolved.Parameters.TextQuery.LiteralText).To(Equal("سامان دوبارہ شمار کرو"))
		Expect(resolved.LiteralText).NotTo(Equal(corrupted))
	})

	It("reconciles an omitted phrase and unjustified clarification when typed context is sufficient", func() {
		question := `Across every recording, locate "rear entrance remained locked".`
		proposal := DynamicPlannerProposalV1{ContractVersion: dynamicPlannerProposalV1, QueryCapabilityID: "transcript.phrase", Semantic: "PHRASE_CONTAINS", ScopeIntent: "authorized_workspace", Clarification: "Which phrase?", Confidence: .8}
		resolved, authority, err := applyPhraseLiteralAuthority(hybridQueryRequest{Query: question, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}, proposal)
		Expect(err).NotTo(HaveOccurred())
		Expect(authority.ModelLiteral).To(BeEmpty())
		Expect(authority.ModelClarification).NotTo(BeEmpty())
		Expect(authority.MismatchReasons).To(ContainElements("literal_not_exact_source_span", "unjustified_model_clarification", "typed_text_query_missing"))
		Expect(resolved.LiteralText).To(Equal("rear entrance remained locked"))
		Expect(resolved.Clarification).To(BeEmpty())
		_, err = validateDynamicPlannerProposal(hybridQueryRequest{Query: question, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}, resolved)
		Expect(err).NotTo(HaveOccurred())
	})

	It("preserves selected document and workspace transcript scope", func() {
		selected := hybridQueryRequest{Query: `Search this document for "sealed register".`, EvidenceID: "10000000-0000-4000-8000-000000000001", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeSelected), SourceFamily: "document"}}
		document := DynamicPlannerProposalV1{ContractVersion: dynamicPlannerProposalV1, QueryCapabilityID: "document.phrase", Semantic: "PHRASE_CONTAINS", ScopeIntent: "selected_evidence", LiteralText: "", Clarification: "Which phrase?", Confidence: .8}
		document, _, err := applyPhraseLiteralAuthority(selected, document)
		Expect(err).NotTo(HaveOccurred())
		_, err = validateDynamicPlannerProposal(selected, document)
		Expect(err).NotTo(HaveOccurred())

		workspace := hybridQueryRequest{Query: `Across every transcript, find "loading lane closed".`, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		transcript := DynamicPlannerProposalV1{ContractVersion: dynamicPlannerProposalV1, QueryCapabilityID: "transcript.phrase", Semantic: "PHRASE_CONTAINS", ScopeIntent: "authorized_workspace", Confidence: .8}
		transcript, _, err = applyPhraseLiteralAuthority(workspace, transcript)
		Expect(err).NotTo(HaveOccurred())
		_, err = validateDynamicPlannerProposal(workspace, transcript)
		Expect(err).NotTo(HaveOccurred())
	})

	It("fails closed on ambiguous phrases and leaves unquoted prose outside deterministic phrase authority", func() {
		proposal := DynamicPlannerProposalV1{QueryCapabilityID: "document.phrase", Semantic: "PHRASE_CONTAINS", LiteralText: "alpha"}
		_, err := reconcileExplicitPhraseLiteral(`Search this document for "alpha" or "beta".`, proposal)
		Expect(err).To(MatchError(ContainSubstring("multiple explicit phrase spans")))
		authority, err := reconcileExplicitPhraseLiteral("Search this document for alpha beta", proposal)
		Expect(err).NotTo(HaveOccurred())
		Expect(authority).To(BeNil())
	})

	It("excludes quote punctuation and keeps exact identifier authority unchanged", func() {
		proposal := DynamicPlannerProposalV1{QueryCapabilityID: "document.phrase", Semantic: "PHRASE_CONTAINS", LiteralText: "wrong"}
		resolved, authority, err := applyPhraseLiteralAuthority(hybridQueryRequest{Query: `Search this document for "alpha, beta!".`}, proposal)
		Expect(err).NotTo(HaveOccurred())
		Expect(authority.SourceLiteral).To(Equal("alpha, beta!"))
		Expect(resolved.LiteralText).To(Equal("alpha, beta!"))
		plate, err := reconcileSingleExactIdentifier("Find plate ISB-5907 in this image", "image.plate", "EXACT_VALUE", "ISB-5907", []string{"ISB-590", "5907"})
		Expect(err).NotTo(HaveOccurred())
		Expect(plate.FinalEntities).To(Equal([]string{"ISB-5907"}))
	})

	It("accepts selected image scope and rejects workspace scope for image.plate", func() {
		selected := hybridQueryRequest{Query: "Find plate ZX-615 in this image", EvidenceID: "10000000-0000-4000-8000-000000000001", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeSelected), SourceFamily: "image"}}
		proposal := DynamicPlannerProposalV1{ContractVersion: dynamicPlannerProposalV1, QueryCapabilityID: "image.plate", Semantic: "EXACT_VALUE", LiteralText: "ZX-615", Target: "ZX-615", ScopeIntent: "selected_evidence", Confidence: .9}
		_, err := validateDynamicPlannerProposal(selected, proposal)
		Expect(err).NotTo(HaveOccurred())
		workspace := selected
		workspace.EvidenceID = ""
		workspace.QueryScope.Kind = string(EvidenceScopeWorkspace)
		proposal.ScopeIntent = "authorized_workspace"
		_, err = validateDynamicPlannerProposal(workspace, proposal)
		Expect(err).To(MatchError(ContainSubstring("unsupported scope")))
	})

	It("binds explicit scope without changing the authorized case identity", func() {
		id := "10000000-0000-4000-8000-000000000001"
		req := hybridQueryRequest{TenantID: "tenant-a", UserID: "analyst-a", CollectionID: "case-a", EvidenceID: id, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeSelected), EvidenceID: id, SourceFamily: "audio"}, Query: "Find plate QR71ABC across the whole investigation"}
		resolved, status := applyExplicitQueryScopeIntent(req)
		Expect(status).To(Equal("authorized_workspace"))
		Expect(resolved.EvidenceID).To(BeEmpty())
		Expect(resolved.QueryScope.Kind).To(Equal(string(EvidenceScopeWorkspace)))
		Expect(resolved.TenantID).To(Equal(req.TenantID))
		Expect(resolved.CollectionID).To(Equal(req.CollectionID))
		Expect(resolved.UserID).To(Equal(req.UserID))
		ids := []string{}
		for _, ref := range dynamicCapabilityCandidates(resolved) {
			ids = append(ids, ref.ID)
		}
		Expect(ids).To(ContainElement("cross_family.identifiers"))
	})

	It("requires selected evidence and rejects contradictory scope language", func() {
		_, status := applyExplicitQueryScopeIntent(hybridQueryRequest{Query: "Search this recording"})
		Expect(status).To(Equal("SELECTED_EVIDENCE_REQUIRED"))
		_, status = applyExplicitQueryScopeIntent(hybridQueryRequest{Query: "Search this recording across the whole investigation"})
		Expect(status).To(Equal("AMBIGUOUS_SCOPE"))
	})

	DescribeTable("binds explicit multilingual workspace scope outside the quoted literal",
		func(query string) {
			id := "10000000-0000-4000-8000-000000000001"
			req := hybridQueryRequest{Query: query, EvidenceID: id, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeSelected), EvidenceID: id, SourceFamily: "audio"}}
			resolved, status := applyExplicitQueryScopeIntent(req)
			Expect(status).To(Equal("authorized_workspace"))
			Expect(resolved.QueryScope.Kind).To(Equal(string(EvidenceScopeWorkspace)))
			Expect(resolved.EvidenceID).To(BeEmpty())
		},
		Entry("Roman Urdu transcript files", `tamam transcript files mein "pichla rasta band rakho" search karo`),
		Entry("English every transcript", `Across every transcript, find "loading lane closed".`),
		Entry("Urdu all recordings", `تمام ریکارڈنگز میں "دروازہ بند رکھو" تلاش کریں۔`),
		Entry("Roman Urdu all documents", `tamam document files mein "naya record kholo" dhoondo`),
	)

	It("ignores workspace words inside the authoritative literal", func() {
		id := "10000000-0000-4000-8000-000000000001"
		req := hybridQueryRequest{Query: `Search this document for "all transcript files".`, EvidenceID: id, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeSelected), EvidenceID: id, SourceFamily: "document"}}
		_, status := applyExplicitQueryScopeIntent(req)
		Expect(status).To(Equal("selected_evidence"))
	})

	It("emits a strict schema over capability identifiers rather than operation identifiers", func() {
		req := hybridQueryRequest{Query: "subscriber details for 07123456789", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		raw, err := json.Marshal(dynamicPlannerJSONSchema(req))
		Expect(err).NotTo(HaveOccurred())
		text := string(raw)
		Expect(text).To(ContainSubstring("subscriber.lookup"))
		Expect(text).NotTo(ContainSubstring("subscriber.identity_lookup"))
		Expect(text).NotTo(ContainSubstring("sql"))
	})

	It("derives direct-descriptor operation references without letting the model select them", func() {
		ref, ok := dynamicCapabilityReference("image.similarity")
		Expect(ok).To(BeTrue())
		Expect(ref.OperationRef).To(Equal("image.visual_similarity"))
		Expect(ref.ReferenceKind).To(Equal("DIRECT_DESCRIPTOR"))
	})

	It("keeps calendar time separate from recording source time", func() {
		req := hybridQueryRequest{Query: "Search this recording from 00:10 to 00:20", EvidenceID: "10000000-0000-4000-8000-000000000001", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeSelected)}}
		start, end, ok := extractPlannerSourceTime(req.Query)
		Expect(ok).To(BeTrue())
		Expect(*start).To(Equal(10.0))
		Expect(*end).To(Equal(20.0))
		p := DynamicPlannerProposalV1{ContractVersion: dynamicPlannerProposalV1, QueryCapabilityID: "transcript.phrase", Semantic: "TIME_RANGE", ScopeIntent: "selected_evidence", StartSeconds: start, EndSeconds: end, Confidence: .9}
		_, err := validateDynamicPlannerProposal(req, p)
		Expect(err).NotTo(HaveOccurred())
		changed := 25.0
		p.EndSeconds = &changed
		_, err = validateDynamicPlannerProposal(req, p)
		Expect(err).To(HaveOccurred())
		calendar := hybridQueryRequest{Query: "Show calls from 2026-04-01 to 2026-04-03", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		_, _, ok = extractPlannerSourceTime(calendar.Query)
		Expect(ok).To(BeFalse())
	})

	It("fails closed for TTS, identity, arbitrary execution, and unsupported formats", func() {
		for _, query := range []string{"Generate speech from this text", "Who is this person in the image?", "Run SELECT * FROM forensic.records", "Analyze this EVTX file"} {
			assessment := assessDynamicQuery(query, "")
			Expect(assessment.Status).To(Equal("unavailable"), query)
		}
	})

	It("does not inherit stale context after a new family and target are explicit", func() {
		req := hybridQueryRequest{Query: "Find plate QR71ABC", TenantID: "tenant-a", UserID: "analyst-a", CollectionID: "case-a", ConversationContext: queryConversationContext{ContractVersion: followUpContextContractV1, TenantID: "tenant-a", UserID: "analyst-a", CollectionID: "case-a", Template: "audio_transcript_search", Target: "07123456789"}}
		plan := planRuntimeQuery(req)
		resolved, after := applyAuditableConversationContext(req, plan, time.Now().UTC())
		Expect(after.Template).To(Equal("anpr_sightings"))
		Expect(resolved.Target).NotTo(Equal("07123456789"))
		Expect(after.Source).NotTo(ContainSubstring("conversation_context"))
	})

	It("rejects conversation context from another tenant or case", func() {
		req := hybridQueryRequest{Query: "Only outgoing", TenantID: "tenant-a", UserID: "analyst-a", CollectionID: "case-a", ConversationContext: queryConversationContext{ContractVersion: followUpContextContractV1, TenantID: "tenant-b", UserID: "analyst-a", CollectionID: "case-b", Template: "frequent_contacts", Target: "07123456789"}}
		resolved, plan := applyAuditableConversationContext(req, planRuntimeQuery(req), time.Now().UTC())
		Expect(resolved.Target).To(BeEmpty())
		Expect(plan.Source).NotTo(ContainSubstring("conversation_context"))
		Expect(plan.Reason).To(ContainSubstring("scope_changed"))
	})
})
