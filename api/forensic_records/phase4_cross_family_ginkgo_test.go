package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type crossFamilyGoldenPack struct {
	SchemaVersion      string                   `json:"schema_version"`
	PlannerCases       []crossFamilyPlannerCase `json:"planner_cases"`
	RelatedOccurrences []map[string]any         `json:"related_occurrences"`
	ExpectedRelations  []crossFamilyExpected    `json:"expected_relations"`
}

type crossFamilyPlannerCase struct {
	Query                 string      `json:"query"`
	Template              string      `json:"template"`
	Intent                queryIntent `json:"intent"`
	Target                string      `json:"target"`
	ClarificationRequired bool        `json:"clarification_required"`
}

type crossFamilyExpected struct {
	RequestedTarget    string   `json:"requested_target"`
	RelatedEntityType  string   `json:"related_entity_type"`
	RelatedEntityValue string   `json:"related_entity_value"`
	CoObservationCount int      `json:"co_observation_count"`
	RecordTypes        []string `json:"record_types"`
	SourceFields       []string `json:"source_fields"`
	SourceFiles        []string `json:"source_files"`
	CitationCount      int      `json:"citation_count"`
}

var _ = Describe("Phase 4C exact cross-family correlation", func() {
	loadGoldens := func() crossFamilyGoldenPack {
		path := filepath.Join("..", "..", "tests", "fixtures", "forensic_modalities", "pakistan_cross_family_query_goldens_v1.json")
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		var pack crossFamilyGoldenPack
		Expect(json.Unmarshal(data, &pack)).To(Succeed())
		Expect(pack.SchemaVersion).To(Equal("1.0.0"))
		return pack
	}

	It("routes Pakistan-oriented correlation questions without replacing relationship-network behavior", func() {
		for _, golden := range loadGoldens().PlannerCases {
			plan := planRuntimeQuery(hybridQueryRequest{Query: golden.Query})
			Expect(plan.Template).To(Equal(golden.Template), golden.Query)
			Expect(plan.Intent).To(Equal(golden.Intent), golden.Query)
			Expect(plan.Target).To(Equal(golden.Target), golden.Query)
			Expect(needsClarification(golden.Query, plan.Template, plan.Target, plan.Targets)).To(Equal(golden.ClarificationRequired), golden.Query)
		}
	})

	It("uses exact bounded canonical matching and returns stable source locators", func() {
		normalized := strings.Join(strings.Fields(crossFamilyCandidateRowsSQL), " ")
		Expect(normalized).To(ContainSubstring("JOIN forensic.record_entities re"))
		Expect(normalized).To(ContainSubstring("FROM forensic.records r"))
		Expect(normalized).To(ContainSubstring("r.tenant_id = $1 AND r.collection_id = $2"))
		Expect(normalized).To(ContainSubstring("r.evidence_id"))
		Expect(normalized).To(ContainSubstring("version_id"))
		Expect(normalized).To(ContainSubstring("record_id"))
		Expect(normalized).To(ContainSubstring("row_hash"))
		Expect(normalized).To(ContainSubstring("xlsx_locator"))
		Expect(normalized).To(ContainSubstring("length(requested.digits_key) >= 8"))
		Expect(normalized).To(ContainSubstring("requested.compact_key"))
		Expect(normalized).NotTo(ContainSubstring("ILIKE"))
		Expect(strings.Count(normalized, "jsonb_each_text")).To(Equal(1))
		Expect(normalized).To(ContainSubstring("$4::timestamptz"))
		Expect(normalized).To(ContainSubstring("$6 = '' OR r.record_type::text = $6"))
		Expect(normalized).To(ContainSubstring("re.record_type::text AS record_type"))
		Expect(normalized).To(ContainSubstring("r.record_type::text AS record_type"))
		Expect(normalized).To(ContainSubstring("r.record_type::text = matched.record_type"))
		Expect(normalized).To(ContainSubstring("LIMIT $9"))
	})

	It("aggregates cited co-observations exactly and deterministically", func() {
		pack := loadGoldens()
		actual := aggregateCrossFamilyRelations(pack.RelatedOccurrences, 5)
		Expect(actual).To(HaveLen(len(pack.ExpectedRelations)))
		for index, expected := range pack.ExpectedRelations {
			relation := actual[index]
			Expect(relation["requested_target"]).To(Equal(expected.RequestedTarget))
			Expect(relation["related_entity_type"]).To(Equal(expected.RelatedEntityType))
			Expect(relation["related_entity_value"]).To(Equal(expected.RelatedEntityValue))
			Expect(relation["co_observation_count"]).To(Equal(expected.CoObservationCount))
			Expect(relation["relationship"]).To(Equal("observed_in_same_record"))
			Expect(relation["relationship_strength"]).To(Equal("observed_association"))
			Expect(relation["record_types"]).To(Equal(expected.RecordTypes))
			Expect(relation["source_fields"]).To(Equal(expected.SourceFields))
			Expect(relation["source_files"]).To(Equal(expected.SourceFiles))
			Expect(relation["citations"]).To(HaveLen(expected.CitationCount))
		}
		xlsxCitation := actual[0]["citations"].([]map[string]any)[0]
		Expect(xlsxCitation).To(HaveKey("evidence_id"))
		Expect(xlsxCitation).To(HaveKey("row_hash"))
		Expect(xlsxCitation).To(HaveKey("xlsx_locator"))
	})

	It("keeps answer counts tied to exact matched records rather than nested display rows", func() {
		records := map[string]any{
			"matched_record_count": 7,
			"target_matches":       []map[string]any{{"record_id": "one"}},
			"related_entities":     []map[string]any{{"related_entity_value": "two"}},
		}
		Expect(canonicalAnswerRowCount("cross_family_correlation", records)).To(Equal(7))
		limitations := crossFamilyQueryLimitations(map[string]any{
			"matched_record_count":       7,
			"returned_match_count":       2,
			"relations_may_be_truncated": true,
			"relation_scan_limit":        50,
		})
		Expect(limitations).To(HaveLen(2))
		Expect(limitations[0]).To(ContainSubstring("2 displayed source records from 7"))
		Expect(limitations[1]).To(ContainSubstring("must not be treated as exhaustive"))
	})

	It("bounds caller-supplied target expansion before database execution", func() {
		payload := []byte(`{"tenant_id":"tenant-a","collection_id":"case-a","query":"correlate these targets across record families","targets":["target-1","target-2","target-3","target-4","target-5","target-6","target-7","target-8","target-9"]}`)
		req := httptest.NewRequest(http.MethodPost, "/query/hybrid", bytes.NewReader(payload))
		recorder := httptest.NewRecorder()

		hybridQueryHandler(config{}, nil).ServeHTTP(recorder, req)

		Expect(recorder.Code).To(Equal(http.StatusBadRequest))
		Expect(recorder.Body.String()).To(ContainSubstring("at most 8 target identifiers"))
	})

	It("accepts explicit comparison targets without asking for redundant clarification", func() {
		Expect(needsClarification(
			"compare these accounts across record families",
			"cross_family_correlation",
			"",
			[]string{"PK36SCBL0000001123456702", "PK97MEZN0001234567890123"},
		)).To(BeFalse())
		plan := applyCanonicalRequestToQueryPlan(
			QueryPlan{AppliedFilters: map[string]any{}},
			hybridQueryRequest{Targets: []string{"923461678183"}},
			"cross_family_correlation",
		)
		Expect(plan.ExecutionStrategy).To(Equal("cross_family_exact_sql"))
		Expect(plan.AppliedFilters).To(HaveKeyWithValue("canonical_table", "forensic.records"))
		matching := plan.AppliedFilters["entity_match"].(map[string]any)
		Expect(matching).To(HaveKeyWithValue("substring_match", false))
	})
})
