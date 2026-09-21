package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type stim4MultiCDRGolden struct {
	ContractVersion string                `json:"contract_version"`
	Target          string                `json:"target"`
	SourceSet       StructuredSourceSetV1 `json:"source_set"`
	Observations    []stim4GoldenCDRRow   `json:"observations"`
	Expected        stim4MultiCDRExpected `json:"expected"`
}

type stim4GoldenCDRRow struct {
	ObservationID   string `json:"observation_id"`
	SourceID        string `json:"source_id"`
	SourceFile      string `json:"source_file"`
	EvidenceID      string `json:"evidence_id"`
	VersionID       string `json:"version_id"`
	RowNumber       int64  `json:"row_number"`
	RowHash         string `json:"row_hash"`
	Caller          string `json:"caller"`
	Callee          string `json:"callee"`
	Direction       string `json:"direction"`
	EventType       string `json:"event_type"`
	IMEI            string `json:"imei"`
	IMSI            string `json:"imsi"`
	Tower           string `json:"tower"`
	StartedAt       string `json:"started_at"`
	DurationSeconds *int64 `json:"duration_seconds"`
}

type stim4MultiCDRExpected struct {
	MatchedEventCount       int                 `json:"matched_event_count"`
	ContactComparisonRows   int                 `json:"contact_comparison_rows"`
	CommonContacts          []string            `json:"common_contacts"`
	UniqueContacts          map[string][]string `json:"unique_contacts"`
	SharedIMEICount         int                 `json:"shared_imei_count"`
	SharedIMSICount         int                 `json:"shared_imsi_count"`
	SharedTowerCount        int                 `json:"shared_tower_count"`
	TemporalOverlapCount    int                 `json:"temporal_overlap_count"`
	DuplicateCandidateCount int                 `json:"duplicate_candidate_count"`
	ConflictCount           int                 `json:"conflict_count"`
}

type stim4CrossFamilyGolden struct {
	ContractVersion string `json:"contract_version"`
	Observations    []struct {
		ObservationID string `json:"observation_id"`
		Family        string `json:"family"`
		SourceID      string `json:"source_id"`
		EvidenceID    string `json:"evidence_id"`
		VersionID     string `json:"version_id"`
		EntityType    string `json:"entity_type"`
		EntityValue   string `json:"entity_value"`
		ObservedAt    string `json:"observed_at"`
		ValidFrom     string `json:"valid_from"`
		ValidTo       string `json:"valid_to"`
	} `json:"observations"`
	ExpectedRelationships         []map[string]string `json:"expected_relationships"`
	AntiCorrelationObservationIDs []string            `json:"anti_correlation_observation_ids"`
}

func loadSTIM4MultiCDRGolden() stim4MultiCDRGolden {
	payload, err := os.ReadFile("contracts/stim4-multi-cdr-golden-v1.json")
	Expect(err).NotTo(HaveOccurred())
	var golden stim4MultiCDRGolden
	Expect(json.Unmarshal(payload, &golden)).To(Succeed())
	return golden
}

func goldenCDRObservations(rows []stim4GoldenCDRRow) []multiCDRObservation {
	observations := make([]multiCDRObservation, 0, len(rows))
	for _, row := range rows {
		startedAt, err := time.Parse(time.RFC3339, row.StartedAt)
		Expect(err).NotTo(HaveOccurred())
		observations = append(observations, multiCDRObservation{
			ObservationID: row.ObservationID, SourceID: row.SourceID, SourceFile: row.SourceFile,
			EvidenceID: row.EvidenceID, VersionID: row.VersionID, RowNumber: row.RowNumber, RowHash: row.RowHash,
			Caller: row.Caller, Callee: row.Callee, Direction: row.Direction, EventType: row.EventType,
			IMEI: row.IMEI, IMSI: row.IMSI, Tower: row.Tower, StartedAt: startedAt, Duration: row.DurationSeconds,
		})
	}
	return observations
}

func loadSTIM4CrossFamilyGolden() (stim4CrossFamilyGolden, []typedStructuredObservation) {
	payload, err := os.ReadFile("contracts/stim4-cross-family-golden-v1.json")
	Expect(err).NotTo(HaveOccurred())
	var golden stim4CrossFamilyGolden
	Expect(json.Unmarshal(payload, &golden)).To(Succeed())
	observations := make([]typedStructuredObservation, 0, len(golden.Observations))
	for _, row := range golden.Observations {
		parse := func(value string) *time.Time {
			if value == "" {
				return nil
			}
			parsed, parseErr := time.Parse(time.RFC3339, value)
			Expect(parseErr).NotTo(HaveOccurred())
			return &parsed
		}
		var observedAt time.Time
		if row.ObservedAt != "" {
			parsed, parseErr := time.Parse(time.RFC3339, row.ObservedAt)
			Expect(parseErr).NotTo(HaveOccurred())
			observedAt = parsed
		}
		observations = append(observations, typedStructuredObservation{ObservationID: row.ObservationID, Family: row.Family, SourceID: row.SourceID, EvidenceID: row.EvidenceID, VersionID: row.VersionID, EntityType: row.EntityType, EntityValue: row.EntityValue, ObservedAt: observedAt, ValidFrom: parse(row.ValidFrom), ValidTo: parse(row.ValidTo)})
	}
	return golden, observations
}

var _ = Describe("STIM-4 multi-source structured correlation", func() {
	It("closes the STIM-3 registry with no silently uncertified ordinary-user operation", func() {
		matrix, err := loadOperationCertificationMatrix()
		Expect(err).NotTo(HaveOccurred())
		counts := map[string]int{}
		silent := []string{}
		for _, entry := range matrix.Entries {
			counts[entry.ExposureStatus]++
			switch entry.ExposureStatus {
			case "queryable":
				if entry.OverallStatus != "certified" && entry.OverallStatus != "certified_with_limitation" {
					silent = append(silent, entry.OperationID)
				}
			case "limited":
				Expect(entry.DebtDisposition).NotTo(BeEmpty(), entry.OperationID)
				Expect(entry.SuggestionEligible).To(BeFalse(), entry.OperationID)
			case "engineering_only":
				Expect(entry.DebtDisposition).NotTo(BeEmpty(), entry.OperationID)
				Expect(entry.SuggestionEligible).To(BeFalse(), entry.OperationID)
			default:
				silent = append(silent, entry.OperationID)
			}
		}
		Expect(matrix.Entries).To(HaveLen(79))
		Expect(counts).To(Equal(map[string]int{"queryable": 5, "limited": 63, "engineering_only": 11}))
		Expect(silent).To(BeEmpty())
	})

	It("matches the independent three-source multi-CDR golden exactly", func() {
		golden := loadSTIM4MultiCDRGolden()
		actual, err := buildMultiCDRComparison(golden.SourceSet, golden.Target, goldenCDRObservations(golden.Observations), 100)
		Expect(err).NotTo(HaveOccurred())
		Expect(actual["contract_version"]).To(Equal(multiCDRComparisonContractV1))
		Expect(actual["matched_event_count"]).To(Equal(golden.Expected.MatchedEventCount))
		Expect(actual["common_contacts"]).To(Equal(golden.Expected.CommonContacts))
		Expect(actual["unique_contacts"]).To(Equal(golden.Expected.UniqueContacts))
		Expect(actual["contact_comparison"]).To(HaveLen(golden.Expected.ContactComparisonRows))
		Expect(actual["shared_imei"]).To(HaveLen(golden.Expected.SharedIMEICount))
		Expect(actual["shared_imsi"]).To(HaveLen(golden.Expected.SharedIMSICount))
		Expect(actual["shared_tower"]).To(HaveLen(golden.Expected.SharedTowerCount))
		Expect(actual["temporal_overlaps"]).To(HaveLen(golden.Expected.TemporalOverlapCount))
		Expect(actual["duplicate_candidates"]).To(HaveLen(golden.Expected.DuplicateCandidateCount))
		Expect(actual["conflicts"]).To(HaveLen(golden.Expected.ConflictCount))
		for _, row := range append(actual["duplicate_candidates"].([]map[string]any), actual["conflicts"].([]map[string]any)...) {
			Expect(row["observation_ids"]).NotTo(BeEmpty())
			Expect(row["citations"]).NotTo(BeEmpty())
			for _, citation := range row["citations"].([]map[string]any) {
				Expect(citation["evidence_id"]).NotTo(BeEmpty())
				Expect(citation["version_id"]).NotTo(BeEmpty())
				Expect(citation["row_hash"]).NotTo(BeEmpty())
			}
		}
	})

	It("keeps zero duration distinct from missing duration and preserves direction roles", func() {
		golden := loadSTIM4MultiCDRGolden()
		actual, err := buildMultiCDRComparison(golden.SourceSet, golden.Target, goldenCDRObservations(golden.Observations), 100)
		Expect(err).NotTo(HaveOccurred())
		rows := actual["contact_comparison"].([]map[string]any)
		var reciprocal, zeroDuration map[string]any
		for _, row := range rows {
			if row["source_id"] == "cdr-a" && row["counterparty"] == "923009000001" {
				reciprocal = row
			}
			if row["source_id"] == "cdr-b" && row["counterparty"] == "923009000001" {
				zeroDuration = row
			}
		}
		Expect(reciprocal).To(SatisfyAll(
			HaveKeyWithValue("interaction_count", int64(2)),
			HaveKeyWithValue("incoming_count", int64(1)),
			HaveKeyWithValue("outgoing_count", int64(1)),
			HaveKeyWithValue("duration_sum_seconds", int64(60)),
			HaveKeyWithValue("duration_value_count", int64(1)),
			HaveKeyWithValue("missing_duration_count", int64(1)),
			HaveKeyWithValue("direction_class", "reciprocal_observation"),
		))
		Expect(zeroDuration).To(SatisfyAll(
			HaveKeyWithValue("duration_sum_seconds", int64(0)),
			HaveKeyWithValue("duration_value_count", int64(1)),
			HaveKeyWithValue("missing_duration_count", int64(0)),
		))
	})

	It("rejects unbounded, duplicate, and contradictory source identities", func() {
		golden := loadSTIM4MultiCDRGolden()
		tooSmall := golden.SourceSet
		tooSmall.Sources = tooSmall.Sources[:1]
		Expect(validateStructuredSourceSet(tooSmall)).To(MatchError(ContainSubstring("between 2 and 8")))
		tooLarge := golden.SourceSet
		tooLarge.Sources = make([]StructuredSourceRefV1, 9)
		for index := range tooLarge.Sources {
			tooLarge.Sources[index] = StructuredSourceRefV1{SourceID: fmt.Sprintf("source-%d", index), SourceFile: fmt.Sprintf("source-%d.csv", index)}
		}
		Expect(validateStructuredSourceSet(tooLarge)).To(MatchError(ContainSubstring("between 2 and 8")))

		unsupported := golden.SourceSet
		unsupported.ContractVersion = "forensics.structured-source-set/v2"
		Expect(validateStructuredSourceSet(unsupported)).To(MatchError(ContainSubstring("unsupported source-set contract")))

		evidenceOnly := golden.SourceSet
		evidenceOnly.Sources = append([]StructuredSourceRefV1(nil), golden.SourceSet.Sources...)
		evidenceOnly.Sources[0].VersionID = ""
		Expect(validateStructuredSourceSet(evidenceOnly)).To(MatchError(ContainSubstring("provide evidence_id and version_id together")))
		versionOnly := golden.SourceSet
		versionOnly.Sources = append([]StructuredSourceRefV1(nil), golden.SourceSet.Sources...)
		versionOnly.Sources[0].EvidenceID = ""
		Expect(validateStructuredSourceSet(versionOnly)).To(MatchError(ContainSubstring("provide evidence_id and version_id together")))

		duplicate := golden.SourceSet
		duplicate.Sources = append([]StructuredSourceRefV1(nil), duplicate.Sources...)
		duplicate.Sources[1].SourceFile = duplicate.Sources[0].SourceFile
		Expect(validateStructuredSourceSet(duplicate)).To(MatchError(ContainSubstring("duplicate source_file")))

		observations := goldenCDRObservations(golden.Observations)
		observations[0].SourceID = "another-source"
		_, err := buildMultiCDRComparison(golden.SourceSet, golden.Target, observations, 100)
		Expect(err).To(MatchError(ContainSubstring("source identity conflicts")))
	})

	It("rejects unresolved and mismatched retained source membership before analytical execution", func() {
		golden := loadSTIM4MultiCDRGolden()
		validRows := make([]map[string]any, 0, len(golden.SourceSet.Sources))
		for _, source := range golden.SourceSet.Sources {
			validRows = append(validRows, map[string]any{
				"source_id": source.SourceID, "source_file": source.SourceFile,
				"cdr_record_count": 2, "unsupported_record_count": 0,
				"evidence_version_record_count": 2,
			})
		}
		cases := []struct {
			name   string
			mutate func([]map[string]any)
		}{
			{name: "unknown source", mutate: func(rows []map[string]any) { rows[0]["cdr_record_count"] = 0 }},
			{name: "wrong filename", mutate: func(rows []map[string]any) { rows[0]["source_file"] = "wrong.csv" }},
			{name: "wrong evidence", mutate: func(rows []map[string]any) { rows[0]["evidence_version_record_count"] = 0 }},
			{name: "wrong version", mutate: func(rows []map[string]any) { rows[1]["evidence_version_record_count"] = 0 }},
			{name: "evidence from another source", mutate: func(rows []map[string]any) { rows[2]["evidence_version_record_count"] = 0 }},
			{name: "wrong authorized scope", mutate: func(rows []map[string]any) {
				rows[0]["cdr_record_count"] = 0
				rows[1]["cdr_record_count"] = 0
				rows[2]["cdr_record_count"] = 0
			}},
			{name: "unsupported family", mutate: func(rows []map[string]any) { rows[0]["unsupported_record_count"] = 1 }},
		}
		for _, testCase := range cases {
			rows := make([]map[string]any, len(validRows))
			for index, row := range validRows {
				rows[index] = map[string]any{}
				for key, value := range row {
					rows[index][key] = value
				}
			}
			testCase.mutate(rows)
			queryCount := 0
			_, err := multiSourceCDRComparisonWithQuery(context.Background(), hybridQueryRequest{
				TenantID: "tenant-a", CollectionID: "case-a", Target: golden.Target,
				SourceSet: &golden.SourceSet, Limit: 20,
			}, func(_ context.Context, _, query string, _ ...any) ([]map[string]any, error) {
				queryCount++
				Expect(query).To(Equal(structuredSourceMembershipSQL), testCase.name)
				return rows, nil
			})
			var membershipErr *structuredSourceMembershipError
			Expect(errors.As(err, &membershipErr)).To(BeTrue(), testCase.name)
			Expect(membershipErr.Error()).To(Equal("selected source set is invalid for the authorized collection"), testCase.name)
			Expect(queryCount).To(Equal(1), testCase.name)
		}
	})

	It("accepts exact retained membership and only then executes the analytical query", func() {
		golden := loadSTIM4MultiCDRGolden()
		membership := make([]map[string]any, 0, len(golden.SourceSet.Sources))
		for _, source := range golden.SourceSet.Sources {
			membership = append(membership, map[string]any{
				"source_id": source.SourceID, "source_file": source.SourceFile,
				"cdr_record_count": 1, "unsupported_record_count": 0, "evidence_version_record_count": 1,
			})
		}
		queries := []string{}
		result, err := multiSourceCDRComparisonWithQuery(context.Background(), hybridQueryRequest{
			TenantID: "tenant-a", CollectionID: "case-a", Target: golden.Target,
			SourceSet: &golden.SourceSet, Limit: 20,
		}, func(_ context.Context, _, query string, _ ...any) ([]map[string]any, error) {
			queries = append(queries, query)
			if query == structuredSourceMembershipSQL {
				return membership, nil
			}
			return []map[string]any{}, nil
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result["matched_event_count"]).To(Equal(0))
		Expect(queries).To(Equal([]string{structuredSourceMembershipSQL, multiSourceCDRRowsSQL}))
	})

	It("returns a stable non-enumerating client error for rejected membership", func() {
		recorder := httptest.NewRecorder()
		writeRecordsExecutionError(recorder, fmt.Errorf("governed execution failed: %w", &structuredSourceMembershipError{reason: "wrong tenant"}))
		Expect(recorder.Code).To(Equal(http.StatusBadRequest))
		var response map[string]string
		Expect(json.Unmarshal(recorder.Body.Bytes(), &response)).To(Succeed())
		Expect(response).To(Equal(map[string]string{
			"error":      "selected source set is invalid for the authorized collection",
			"error_code": structuredSourceMembershipErrorCode,
		}))
		Expect(recorder.Body.String()).NotTo(ContainSubstring("wrong tenant"))
	})

	It("binds the governed plan and SQL to the exact case and source set", func() {
		golden := loadSTIM4MultiCDRGolden()
		template, found := queryTemplateByName("multi_cdr_comparison")
		Expect(found).To(BeTrue())
		capability := capabilityFromTemplate(template, CapabilityProjectionContextV1{Authorized: true, RuntimeAvailable: true, FamilyStates: map[string]WorkspaceCapabilityStateV1{"cdr": {IndexedRecords: 12, RegisteredEvidence: 3, NormalizedEvidence: 3}}})
		Expect(capability.SourceScoped).To(BeTrue())
		Expect(capability.RequiredParameters).To(ConsistOf("target", "source_set"))
		req := hybridQueryRequest{TenantID: "tenant-a", UserID: "analyst-a", CollectionID: "case-a", Query: template.ExampleQuery, Target: golden.Target, Targets: []string{golden.Target}, SourceSet: &golden.SourceSet, Limit: 20, MaxKBResults: 3}
		planner := planRuntimeQuery(req)
		planner.Template, planner.Target, planner.Targets, planner.Confidence = template.Name, golden.Target, []string{golden.Target}, 1
		understanding := adaptLegacyRuntimePlan(req, planner, template)
		plan, err := buildSingleCapabilityPlan("request-stim4", req, understanding, capability)
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.Workspace.CollectionID).To(Equal("case-a"))
		Expect(plan.Steps[0].EvidenceScope.SourceFiles).To(ConsistOf("cdr-a.csv", "cdr-b.csv", "cdr-c.csv"))
		normalizedSQL := strings.Join(strings.Fields(multiSourceCDRRowsSQL), " ")
		Expect(normalizedSQL).To(ContainSubstring("c.tenant_id = $1 AND c.collection_id = $2"))
		Expect(normalizedSQL).To(ContainSubstring("unnest($3::text[], $4::text[], $5::text[], $6::text[])"))
		Expect(normalizedSQL).To(ContainSubstring("c.file_id::text = selected.source_id AND c.source_file = selected.source_file"))
		Expect(normalizedSQL).To(ContainSubstring("r.evidence_id::text = selected.evidence_id"))
		Expect(normalizedSQL).To(ContainSubstring("LIKE '0092%'"))
		Expect(normalizedSQL).To(ContainSubstring("~ '^03[0-9]{9}$'"))
		Expect(normalizedSQL).To(ContainSubstring("version_id"))
		Expect(normalizedSQL).NotTo(ContainSubstring("ILIKE"))
	})

	It("renders only typed evidence-backed graph edges", func() {
		golden := loadSTIM4MultiCDRGolden()
		actual, err := buildMultiCDRComparison(golden.SourceSet, golden.Target, goldenCDRObservations(golden.Observations), 100)
		Expect(err).NotTo(HaveOccurred())
		graph := actual["graph"].(map[string]any)
		Expect(graph["nodes"]).NotTo(BeEmpty())
		Expect(graph["edges"]).NotTo(BeEmpty())
		for _, edge := range graph["edges"].([]map[string]any) {
			Expect(edge["relationship"]).NotTo(BeEmpty())
			Expect(edge["relationship_strength"]).NotTo(BeEmpty())
		}
		Expect(graph["semantics"]).To(ContainSubstring("prominence is not analytical strength"))
	})

	It("uses canonical CDR service classes for cross-provider conflict keys", func() {
		durationA, durationB := int64(60), int64(120)
		sourceSet := StructuredSourceSetV1{ContractVersion: structuredSourceSetContractV1, Sources: []StructuredSourceRefV1{
			{SourceID: "source-a", SourceFile: "a.csv"},
			{SourceID: "source-b", SourceFile: "b.csv"},
		}}
		startedAt := time.Date(2026, time.August, 18, 5, 0, 0, 0, time.UTC)
		actual, err := buildMultiCDRComparison(sourceSet, "+923001234567", []multiCDRObservation{
			{ObservationID: "a1", SourceID: "source-a", SourceFile: "a.csv", Caller: "03001234567", Callee: "03110000001", EventType: "VOICE", StartedAt: startedAt, Duration: &durationA},
			{ObservationID: "b1", SourceID: "source-b", SourceFile: "b.csv", Caller: "923001234567", Callee: "923110000001", EventType: "CALL", StartedAt: startedAt, Duration: &durationB},
		}, 20)
		Expect(err).NotTo(HaveOccurred())
		Expect(actual["conflicts"]).To(HaveLen(1))
		Expect(actual["duplicate_candidates"]).To(BeEmpty())
	})

	It("matches only typed time-valid cross-family relationships and resists tempting coincidences", func() {
		golden, observations := loadSTIM4CrossFamilyGolden()
		actual := buildTypedCrossFamilyRelationships(observations)
		Expect(actual).To(HaveLen(len(golden.ExpectedRelationships)))
		for index, expected := range golden.ExpectedRelationships {
			Expect(actual[index]).To(SatisfyAll(
				HaveKeyWithValue("left_observation_id", expected["left_observation_id"]),
				HaveKeyWithValue("right_observation_id", expected["right_observation_id"]),
				HaveKeyWithValue("relationship", expected["relationship"]),
				HaveKeyWithValue("relationship_strength", expected["relationship_strength"]),
			))
		}
		for _, forbiddenID := range golden.AntiCorrelationObservationIDs {
			for _, relation := range actual {
				Expect([]string{stringValueAny(relation["left_observation_id"]), stringValueAny(relation["right_observation_id"])}).NotTo(ContainElement(forbiddenID))
			}
		}
	})

	It("publishes comparison, relationship, graph, timeline, conflict, limitation, and provenance views", func() {
		golden := loadSTIM4MultiCDRGolden()
		records, err := buildMultiCDRComparison(golden.SourceSet, golden.Target, goldenCDRObservations(golden.Observations), 100)
		Expect(err).NotTo(HaveOccurred())
		response := hybridQueryResponse{Template: "multi_cdr_comparison", Route: []string{"records_sql"}, Records: records, Answer: map[string]any{"records_row_count": records["matched_event_count"], "records_summary": "Source-aware multi-CDR comparison computed."}}
		presentation := buildMultiCDREnterprisePayload(hybridQueryRequest{TenantID: "tenant-a", CollectionID: "case-a", Target: golden.Target}, response)
		Expect(presentation).To(HaveKey("comparison"))
		Expect(presentation).To(HaveKey("relationship_table"))
		Expect(presentation["visualizations"]).To(HaveLen(2))
		Expect(presentation).To(HaveKey("conflicts"))
		Expect(presentation["limitations"]).NotTo(BeEmpty())
		Expect(presentation["limitations"]).To(ContainElement(ContainSubstring("not complete aggregate contribution lineage")))
		Expect(presentation["provenance"]).To(HaveLen(golden.Expected.MatchedEventCount))
	})

	It("keeps repeated three-source result compilation bounded", func() {
		golden := loadSTIM4MultiCDRGolden()
		observations := goldenCDRObservations(golden.Observations)
		started := time.Now()
		for iteration := 0; iteration < 1000; iteration++ {
			_, err := buildMultiCDRComparison(golden.SourceSet, golden.Target, observations, 100)
			Expect(err).NotTo(HaveOccurred())
		}
		elapsed := time.Since(started)
		GinkgoWriter.Printf("STIM4MultiCDRCompileIterations=1000 Elapsed=%s\n", elapsed)
		Expect(elapsed).To(BeNumerically("<", 2*time.Second))
	})

	It("uses one bounded record-entity candidate query and preserves exact cross-family fields", func() {
		normalizedSQL := strings.Join(strings.Fields(crossFamilyCandidateRowsSQL), " ")
		Expect(normalizedSQL).To(ContainSubstring("JOIN forensic.record_entities re"))
		Expect(normalizedSQL).To(ContainSubstring("re.tenant_id = $1 AND re.collection_id = $2"))
		Expect(normalizedSQL).To(ContainSubstring("r.record_type <> 'cdr'"))
		Expect(strings.Count(normalizedSQL, "jsonb_each_text")).To(Equal(1))
		Expect(normalizedSQL).To(ContainSubstring("re.entity_type IN"))
		Expect(normalizedSQL).To(ContainSubstring("re.record_type::text = $6"))
		Expect(normalizedSQL).To(ContainSubstring("r.record_type::text = $6"))
		Expect(normalizedSQL).To(ContainSubstring("re.record_type::text AS record_type"))
		Expect(normalizedSQL).To(ContainSubstring("r.record_type::text AS record_type"))
		Expect(normalizedSQL).To(ContainSubstring("r.record_type::text = matched.record_type"))
		Expect(normalizedSQL).To(ContainSubstring("re.source_file = $7"))
		Expect(normalizedSQL).To(ContainSubstring("LIMIT $9"))

		row := map[string]any{
			"requested_target": "490154203237518", "record_type": "subscriber", "observed_at": "2026-08-01T10:00:00Z",
			"primary_target": "923001234567", "secondary_target": "PK-SUB-001",
			"normalized_fields": map[string]any{"imei_raw": "490154203237518", "imsi_raw": "410010123456789", "location": "Lahore"},
			"evidence_id":       "evidence-1", "version_id": "version-1", "record_id": "record-1", "file_id": "file-1",
			"batch_id": "batch-1", "source_file": "subscriber.jsonl", "row_number": 1, "row_hash": "hash-1",
			"citation_locator": map[string]any{"record_id": "record-1", "row_hash": "hash-1", "source_file": "subscriber.jsonl", "row_number": 1},
		}
		compiled := compileCrossFamilyCandidateRows(hybridQueryRequest{Limit: 20}, []string{"490154203237518"}, []map[string]any{row})
		Expect(compiled["matched_record_count"]).To(Equal(1))
		Expect(compiled["target_matches"].([]map[string]any)[0]["matched_fields"]).To(ContainElement("imei_raw"))
		relations := compiled["related_entities"].([]map[string]any)
		Expect(relations).To(ContainElement(SatisfyAll(HaveKeyWithValue("related_entity_type", "imsi"), HaveKeyWithValue("related_entity_value", "410010123456789"))))
	})

	It("compiles the maximum cross-family candidate set within the bounded source budget", func() {
		rows := make([]map[string]any, 0, maxCrossFamilyMaterializedRows)
		for index := 0; index < maxCrossFamilyMaterializedRows; index++ {
			rows = append(rows, map[string]any{
				"requested_target": "923001234567", "record_type": "ipdr", "observed_at": "2026-08-01T10:00:00Z",
				"primary_target": "10.0.0.1", "secondary_target": "923001234567",
				"normalized_fields": map[string]any{"subscriber_reference": "923001234567", "source_ip_canonical": "10.0.0.1"},
				"record_id":         fmt.Sprintf("record-%d", index), "source_file": "ipdr.csv", "row_number": index + 1,
			})
		}
		started := time.Now()
		compiled := compileCrossFamilyCandidateRows(hybridQueryRequest{Limit: 20}, []string{"923001234567"}, rows)
		elapsed := time.Since(started)
		GinkgoWriter.Printf("STIM4CrossFamilyCandidateRows=%d Elapsed=%s\n", len(rows), elapsed)
		Expect(compiled["matched_record_count"]).To(Equal(maxCrossFamilyMaterializedRows))
		Expect(elapsed).To(BeNumerically("<", 3*time.Second))
	})
})
