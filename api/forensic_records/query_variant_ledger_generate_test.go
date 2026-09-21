package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

type queryVariantSeed struct {
	Text                  string
	Template              string
	Language              string
	Source                string
	Target                string
	Direction             string
	ClarificationRequired bool
	FollowUp              bool
}

var reconciledFollowUpVariantSeeds = []queryVariantSeed{
	{Text: "Only outgoing.", Template: "frequent_contacts", Language: "english", Source: "apf34_follow_up_contracts", Target: "923001110001", Direction: "OUTGOING", FollowUp: true},
	{Text: "Now show only incoming for 923009999999.", Template: "frequent_contacts", Language: "english", Source: "apf34_follow_up_contracts", Target: "923009999999", Direction: "INCOMING", FollowUp: true},
	{Text: "For 923001234567 instead.", Template: "frequent_contacts", Language: "english", Source: "apf34_follow_up_contracts", Target: "923001234567", Direction: "OUTGOING", FollowUp: true},
	{Text: "Switch to 923001234567.", Template: "frequent_contacts", Language: "english", Source: "apf34_follow_up_contracts", Target: "923001234567", Direction: "OUTGOING", FollowUp: true},
	{Text: "Use 923001234567 instead.", Template: "frequent_contacts", Language: "english", Source: "apf34_follow_up_contracts", Target: "923001234567", Direction: "OUTGOING", FollowUp: true},
	{Text: "Same analysis for 923001234567.", Template: "frequent_contacts", Language: "english", Source: "apf34_follow_up_contracts", Target: "923001234567", Direction: "OUTGOING", FollowUp: true},
	{Text: "Now do it for 923001234567.", Template: "frequent_contacts", Language: "english", Source: "apf34_follow_up_contracts", Target: "923001234567", Direction: "OUTGOING", FollowUp: true},
	{Text: "Do that for 923001234567 instead.", Template: "frequent_contacts", Language: "english", Source: "apf34_follow_up_contracts", Target: "923001234567", Direction: "OUTGOING", FollowUp: true},
	{Text: "Show temporal CDR activity for 923001234567 instead.", Template: "temporal_activity", Language: "english", Source: "apf34_follow_up_contracts", Target: "923001234567"},
	{Text: "show frequent contacts for this number", Template: "frequent_contacts", Language: "english", Source: "apf34_follow_up_contracts", ClarificationRequired: true},
}

var apf35LiveLanguageVariantSeeds = []queryVariantSeed{
	{Text: "For 923001234567 show me calls activity around 10 July 2026.", Template: "temporal_activity", Language: "english", Source: "apf35_live_language_equivalence", Target: "923001234567"},
	{Text: "923001234567 ki 10 July 2026 ki CDR activity dikhao.", Template: "temporal_activity", Language: "roman_urdu", Source: "apf35_live_language_equivalence", Target: "923001234567"},
	{Text: "10 جولائی 2026 کو 923001234567 کی CDR سرگرمی دکھائیں", Template: "temporal_activity", Language: "urdu", Source: "apf35_live_language_equivalence", Target: "923001234567"},
}

var stim5LanguageVariantSeeds = []queryVariantSeed{
	{Text: "cdr actvty 923001234567", Template: "temporal_activity", Language: "english", Source: "stim5_query_quality", Target: "923001234567"},
	{Text: "frequent cntcts 923001234567", Template: "frequent_contacts", Language: "english", Source: "stim5_query_quality", Target: "923001234567"},
	{Text: "who he called most 923001234567", Template: "frequent_contacts", Language: "english", Source: "stim5_query_quality", Target: "923001234567"},
	{Text: "show outgng only", Template: "frequent_contacts", Language: "english", Source: "stim5_query_quality", Target: "923001110001", Direction: "OUTGOING", FollowUp: true},
	{Text: "compare both src", Template: "multi_cdr_comparison", Language: "english", Source: "stim5_query_quality"},
	{Text: "any common imei", Template: "multi_cdr_comparison", Language: "english", Source: "stim5_query_quality"},
	{Text: "sab se zyada kis number se baat hui 923001234567", Template: "frequent_contacts", Language: "roman_urdu", Source: "stim5_query_quality", Target: "923001234567"},
	{Text: "10 July 2026 ko 923001234567 ki activity dikhao", Template: "temporal_activity", Language: "roman_urdu", Source: "stim5_query_quality", Target: "923001234567"},
	{Text: "common contacts batao", Template: "multi_cdr_comparison", Language: "roman_urdu", Source: "stim5_query_quality"},
	{Text: "same IMEI observation hai?", Template: "multi_cdr_comparison", Language: "roman_urdu", Source: "stim5_query_quality"},
	{Text: "سرگرمی دکھائیں 923001234567", Template: "temporal_activity", Language: "urdu", Source: "stim5_query_quality", Target: "923001234567"},
	{Text: "سب سے زیادہ رابطہ 923001234567", Template: "frequent_contacts", Language: "urdu", Source: "stim5_query_quality", Target: "923001234567"},
	{Text: "صرف آؤٹ گوئنگ", Template: "frequent_contacts", Language: "urdu", Source: "stim5_query_quality", Target: "923001110001", Direction: "OUTGOING", FollowUp: true},
	{Text: "مشترکہ رابطے", Template: "multi_cdr_comparison", Language: "urdu", Source: "stim5_query_quality"},
	{Text: "اس نمبر کی outgoing activity show karo 923001234567", Template: "temporal_activity", Language: "mixed", Source: "stim5_query_quality", Target: "923001234567", Direction: "OUTGOING"},
	{Text: "evidence citations دکھائیں", Template: "evidence", Language: "mixed", Source: "stim5_query_quality"},
}

var nxb1LanguageVariantSeeds = []queryVariantSeed{
	{Text: "Show transaction totals by currency and status.", Template: "financial_transaction_summary", Language: "english", Source: "nxb1_family_breadth"},
	{Text: "transaction totals currncy wise", Template: "financial_transaction_summary", Language: "english", Source: "nxb1_family_breadth"},
	{Text: "len den ka khulasa currency ke hisab se dikhao", Template: "financial_transaction_summary", Language: "roman_urdu", Source: "nxb1_family_breadth"},
	{Text: "کرنسی کے حساب سے لین دین کا خلاصہ دکھائیں", Template: "financial_transaction_summary", Language: "urdu", Source: "nxb1_family_breadth"},
	{Text: "transaction ka خلاصہ currency wise", Template: "financial_transaction_summary", Language: "mixed", Source: "nxb1_family_breadth"},
	{Text: "Show failed access events.", Template: "access_failed_events", Language: "english", Source: "nxb1_family_breadth"},
	{Text: "failed login evnts dikhao", Template: "access_failed_events", Language: "roman_urdu", Source: "nxb1_family_breadth"},
	{Text: "ناکام رسائی کے واقعات دکھائیں", Template: "access_failed_events", Language: "urdu", Source: "nxb1_family_breadth"},
	{Text: "failed access واقعات show karo", Template: "access_failed_events", Language: "mixed", Source: "nxb1_family_breadth"},
	{Text: "Show generic structured rows.", Template: "generic_filter_records", Language: "english", Source: "nxb1_family_breadth"},
	{Text: "aam structured records dikhao", Template: "generic_filter_records", Language: "roman_urdu", Source: "nxb1_family_breadth"},
	{Text: "عمومی ریکارڈز دکھائیں", Template: "generic_filter_records", Language: "urdu", Source: "nxb1_family_breadth"},
	{Text: "Show registered document metadata.", Template: "document_metadata", Language: "english", Source: "nxb1_family_breadth"},
	{Text: "documents mein dhoondo agreement", Template: "document_search", Language: "roman_urdu", Source: "nxb1_family_breadth"},
	{Text: "دستاویز میں تلاش کریں agreement", Template: "document_search", Language: "mixed", Source: "nxb1_family_breadth"},
	{Text: "Show image processing status.", Template: "image_metadata", Language: "english", Source: "nxb1_family_breadth"},
	{Text: "tasveer ka matn dhoondo ABC123", Template: "image_ocr_search", Language: "roman_urdu", Source: "nxb1_family_breadth"},
	{Text: "تصویر کا متن تلاش کریں ABC123", Template: "image_ocr_search", Language: "mixed", Source: "nxb1_family_breadth"},
	{Text: "Show face candidate observations in this image.", Template: "face_candidate_observations", Language: "english", Source: "nxb1_family_breadth"},
	{Text: "tasveer mein chehray ke candidates dikhao", Template: "face_candidate_observations", Language: "roman_urdu", Source: "nxb1_family_breadth"},
	{Text: "تصویر میں چہرے کے امیدوار دکھائیں", Template: "face_candidate_observations", Language: "urdu", Source: "nxb1_family_breadth"},
	{Text: "image میں face candidates show karo", Template: "face_candidate_observations", Language: "mixed", Source: "nxb1_family_breadth"},
	{Text: "Show registered audio metadata.", Template: "audio_metadata", Language: "english", Source: "nxb1_family_breadth"},
	{Text: "transcript mein dhoondo meeting", Template: "audio_transcript_search", Language: "roman_urdu", Source: "nxb1_family_breadth"},
	{Text: "ٹرانسکرپٹ میں تلاش کریں meeting", Template: "audio_transcript_search", Language: "mixed", Source: "nxb1_family_breadth"},
	{Text: "Show registered video metadata.", Template: "video_metadata", Language: "english", Source: "nxb1_family_breadth"},
	{Text: "video ka timeline dikhao", Template: "video_timeline", Language: "roman_urdu", Source: "nxb1_family_breadth"},
	{Text: "ویڈیو ٹائم لائن دکھائیں", Template: "video_timeline", Language: "urdu", Source: "nxb1_family_breadth"},
}

func TestGenerateQueryVariantLedger(t *testing.T) {
	if os.Getenv("GENERATE_QUERY_VARIANT_LEDGER") != "1" {
		t.Skip("generation not requested")
	}
	ledger := buildQueryVariantLedgerForTest(t)
	payload, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile("contracts/query-variant-ledger-v1.json", payload, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestQueryVariantLedgerReconcilesAcceptedSources(t *testing.T) {
	want := buildQueryVariantLedgerForTest(t)
	got, err := loadQueryVariantLedger()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateQueryVariantLedger(got, supportedQueryTemplates()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("derived query-variant ledger is stale; run GENERATE_QUERY_VARIANT_LEDGER=1 go test ./api/forensic_records -run TestGenerateQueryVariantLedger")
	}
}

func buildQueryVariantLedgerForTest(t *testing.T) QueryVariantLedgerV1 {
	t.Helper()
	seeds := acceptedQueryVariantSeedsForTest(t)
	ledger := QueryVariantLedgerV1{
		ContractVersion: queryVariantLedgerContractV1,
		GeneratedFrom: []string{
			"supportedQueryTemplates()",
			"r6_6_real_world_corpus_ginkgo_test.go",
			"tests/fixtures/forensic_modalities/pakistan_subscriber_query_goldens_v1.json",
			"contracts/forensic-family-query-answer-corpus-v1.json",
			"query_intelligence_test.go",
			"subscriber_phase7_ginkgo_test.go",
			"query_followup_contracts_test.go",
			"apf35_live_language_equivalence",
			"stim5_query_quality",
			"nxb1_family_breadth",
		},
		Entries: make([]QueryVariantLedgerEntryV1, 0, len(seeds)),
	}
	firstByText := map[string]string{}
	for index, seed := range seeds {
		template, ok := queryTemplateByName(seed.Template)
		if !ok {
			t.Fatalf("variant source references unknown template %q", seed.Template)
		}
		variantID := fmt.Sprintf("qv-%03d", index+1)
		entry := QueryVariantLedgerEntryV1{
			VariantID: variantID, Text: seed.Text, Language: seed.Language, SourceCorpus: seed.Source,
			CanonicalOperation: template.OperationID, ExpectedIntent: string(durableIntentForTemplate(template)), ExpectedFamily: template.FamilyID,
			ExpectedTarget: seed.Target, ExpectedEntities: []string{}, ExpectedTime: map[string]string{}, ExpectedDirection: seed.Direction,
			ExpectedFilters: map[string]string{}, FollowUpRequirements: map[string]string{}, ClarificationRequired: seed.ClarificationRequired,
		}
		if seed.Target != "" {
			entry.ExpectedEntities = []string{seed.Target}
		}
		if seed.Direction != "" {
			entry.ExpectedFilters["direction"] = seed.Direction
		}
		if seed.FollowUp {
			entry.FollowUpRequirements = map[string]string{
				"contract_version": followUpContextContractV1,
				"prior_operation":  "cdr.frequent_contacts",
				"prior_target":     "923001110001",
				"prior_direction":  "OUTGOING",
			}
		}
		entry.RoutingPass, entry.ParameterPass = evaluateVariantSeed(t, seed, template)
		entry.SemanticEquivalent = entry.RoutingPass && entry.ParameterPass
		entry.Status = "fail"
		if entry.SemanticEquivalent {
			entry.Status = "pass"
		}
		if first, duplicate := firstByText[seed.Text]; duplicate {
			entry.DuplicateOf = first
		} else {
			firstByText[seed.Text] = variantID
		}
		ledger.Entries = append(ledger.Entries, entry)
	}
	return ledger
}

func evaluateVariantSeed(t *testing.T, seed queryVariantSeed, template queryTemplateCatalogEntry) (bool, bool) {
	t.Helper()
	req := hybridQueryRequest{TenantID: "default", UserID: "variant-ledger", CollectionID: "case-a", Query: seed.Text}
	req.Direction = canonicalEventDirection(extractEventDirection(req.Query))
	plan := planRuntimeQuery(req)
	if seed.FollowUp {
		now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
		req.ConversationContext = queryConversationContext{
			ContractVersion: followUpContextContractV1, AnalysisID: "ledger-analysis", TurnID: "ledger-turn",
			TenantID: req.TenantID, UserID: req.UserID, CollectionID: req.CollectionID,
			ExpiresAt: now.Add(30 * time.Minute).Format(time.RFC3339), Target: "923001110001", Template: "frequent_contacts", Direction: "OUTGOING",
		}
		resolved, resolvedPlan := applyAuditableConversationContext(req, plan, now)
		req, plan = resolved, resolvedPlan
	}
	routingPass := plan.Template == template.Name
	if seed.ClarificationRequired {
		return routingPass, needsClarification(seed.Text, template.Name, "")
	}
	actualTarget := req.Target
	if actualTarget == "" {
		actualTarget = plan.Target
	}
	actualDirection := req.Direction
	parameterPass := (seed.Target == "" || actualTarget == seed.Target) && (seed.Direction == "" || actualDirection == seed.Direction)
	return routingPass, parameterPass
}

func acceptedQueryVariantSeedsForTest(t *testing.T) []queryVariantSeed {
	t.Helper()
	seeds := make([]queryVariantSeed, 0, 160)
	for _, template := range supportedQueryTemplates() {
		seeds = append(seeds, queryVariantSeed{Text: template.ExampleQuery, Template: template.Name, Language: queryVariantLanguage(template.ExampleQuery, "en"), Source: "operation_catalog"})
	}

	r6Source := readVariantSource(t, "r6_6_real_world_corpus_ginkgo_test.go")
	for _, match := range regexp.MustCompile(`\{"([^"]+)",\s*"([^"]+)"\}`).FindAllStringSubmatch(r6Source, -1) {
		seeds = append(seeds, queryVariantSeed{Text: match[1], Template: match[2], Language: queryVariantLanguage(match[1], "en"), Source: "r6_6_real_world_corpus"})
	}

	var subscriberFixture struct {
		QueryCases []struct {
			Language              string `json:"language"`
			Query                 string `json:"query"`
			Template              string `json:"template"`
			Target                string `json:"target"`
			ClarificationRequired bool   `json:"clarification_required"`
		} `json:"query_cases"`
	}
	readVariantJSON(t, "../../tests/fixtures/forensic_modalities/pakistan_subscriber_query_goldens_v1.json", &subscriberFixture)
	for _, item := range subscriberFixture.QueryCases {
		seeds = append(seeds, queryVariantSeed{Text: item.Query, Template: item.Template, Language: queryVariantLanguage(item.Query, item.Language), Source: "pakistan_subscriber_query_goldens_v1", Target: item.Target, ClarificationRequired: item.ClarificationRequired})
	}

	var familyCorpus forensicQueryCorpus
	if err := json.Unmarshal(forensicQueryAnswerCorpusJSON, &familyCorpus); err != nil {
		t.Fatal(err)
	}
	for _, item := range familyCorpus.Entries {
		seeds = append(seeds, queryVariantSeed{Text: item.Query, Template: item.ExpectedTemplate, Language: queryVariantLanguage(item.Query, item.Locale), Source: "forensic_family_query_answer_corpus_v1"})
	}

	intelligenceSource := readVariantSource(t, "query_intelligence_test.go")
	intelligencePattern := regexp.MustCompile(`\{"(English|Roman Urdu|Urdu)",\s*"([^"]+)",\s*"([^"]+)",\s*"[^"]+"\}`)
	for _, match := range intelligencePattern.FindAllStringSubmatch(intelligenceSource, -1) {
		seeds = append(seeds, queryVariantSeed{Text: match[2], Template: match[3], Language: queryVariantLanguage(match[2], match[1]), Source: "apf31_language_equivalence"})
	}

	subscriberSource := readVariantSource(t, "subscriber_phase7_ginkgo_test.go")
	subscriberPattern := regexp.MustCompile(`Entry\("[^"]+",\s*"([^"]+)",\s*"([^"]+)"\)`)
	subscriberMatches := subscriberPattern.FindAllStringSubmatch(subscriberSource, -1)
	if len(subscriberMatches) < 6 {
		t.Fatalf("subscriber operation variant source has %d entries, want at least 6", len(subscriberMatches))
	}
	for _, match := range subscriberMatches[:6] {
		seeds = append(seeds, queryVariantSeed{Text: match[1], Template: match[2], Language: queryVariantLanguage(match[1], "en"), Source: "subscriber_phase7_operation_variants"})
	}

	seeds = append(seeds, reconciledFollowUpVariantSeeds...)
	seeds = append(seeds, apf35LiveLanguageVariantSeeds...)
	seeds = append(seeds, stim5LanguageVariantSeeds...)
	seeds = append(seeds, nxb1LanguageVariantSeeds...)
	return seeds
}

func queryVariantLanguage(text, declared string) string {
	if queryHasMixedScript(text) || strings.EqualFold(declared, "mixed") {
		return "mixed"
	}
	if regexp.MustCompile(`[\x{0600}-\x{06ff}]`).MatchString(text) || strings.EqualFold(declared, "ur") || strings.EqualFold(declared, "Urdu") {
		return "urdu"
	}
	if strings.EqualFold(declared, "ur-Latn") || strings.EqualFold(declared, "roman_urdu") || strings.EqualFold(declared, "Roman Urdu") {
		return "roman_urdu"
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, " ke ") || strings.Contains(lower, " dikhao") || strings.Contains(lower, " ka khulasa") {
		return "mixed"
	}
	return "english"
}

func readVariantSource(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func readVariantJSON(t *testing.T, path string, target any) {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		t.Fatal(err)
	}
}
