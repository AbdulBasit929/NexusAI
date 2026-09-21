package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestClassifyIntent(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		template string
		want     queryIntent
	}{
		{"overview query", "show collection status and duplicate rows", "", intentRecords},
		{"records default", "show frequent contacts", "", intentRecords},
		{"semantic", "summarize the policy evidence", "", intentSemantic},
		{"hybrid", "summarize evidence and show most frequent contacts", "", intentHybrid},
		{"hybrid plural entities", "summarize evidence for ABC-123 and show related entities", "", intentHybrid},
		{"hybrid policy and exact records", "Using both policy context and deterministic records, explain why exact counts must not be inferred from Knowledge Base chunks.", "", intentHybrid},
		{"readiness", "is this case ready for production", "", intentRecords},
		{"explicit records", "anything", "temporal_activity", intentRecords},
		{"explicit evidence", "anything", "evidence", intentSemantic},
		{"explicit hybrid evidence package", "anything", "evidence_package_summary", intentHybrid},
		{"explicit hybrid executive brief", "anything", "executive_case_brief", intentHybrid},
		{"natural deterministic relationship", "What evidence-backed relationships exist for 923001110001?", "", intentRecords},
		{"natural deterministic package", "What is included in the evidence package?", "", intentRecords},
		{"natural deterministic executive brief", "Prepare an executive case brief from deterministic findings", "", intentRecords},
		{"natural deterministic court summary", "Prepare a court-ready source and provenance summary", "", intentRecords},
		{"natural model assistance", "Explain the frequent contacts using only returned counts", "", intentHybrid},
		{"deterministic anomaly summary", "Summarize the deterministic anomalies", "", intentRecords},
		{"deterministic cross-dataset summary", "Summarize 923001110001 across datasets", "", intentRecords},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyIntent(tt.query, tt.template); got != tt.want {
				t.Fatalf("classifyIntent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChooseTemplate(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"show collection status and duplicate rows", "collection_overview"},
		{"how many GPRS records are there", "service_usage"},
		{"show IMEI changes for 923001234567", "device_identity_changes"},
		{"show IPDR endpoint summary", "ipdr_endpoint_summary"},
		{"show DNS domain summary", "ipdr_domain_summary"},
		{"For 923001234567 show me calls activity around 10 July 2026.", "temporal_activity"},
		{"923001234567 cdr actvty 10 july 2026 pls", "temporal_activity"},
		{"10 جولائی 2026 کو 923001234567 کی CDR سرگرمی دکھائیں", "temporal_activity"},
		{"show network protocol breakdown", "ipdr_protocol_breakdown"},
		{"show hourly session volume", "ipdr_session_volume"},
		{"show subscriber sessions for 923001234567", "ipdr_subscriber_sessions"},
		{"show concurrent sessions for 923001234567", "ipdr_concurrent_sessions"},
		{"show IPDR timeline for 10.20.1.7", "ipdr_timeline"},
		{"show camera sequence for ABC-123", "anpr_camera_sequence"},
		{"show ANPR camera activity", "anpr_camera_activity"},
		{"show co-travel for ABC-123", "anpr_co_travel"},
		{"show route timing for ABC-123", "anpr_route_timing"},
		{"show plate variants for ABC-123", "anpr_plate_variants"},
		{"show ANPR timeline for ABC-123", "anpr_timeline"},
		{"show hourly nocturnal anomalies", "suspicious_patterns"},
		{"shortest call duration of 923461678183", "shortest_call"},
		{"longest call duration for 923461678183", "longest_call"},
		{"show duration extremes for 923461678183", "duration_extremes"},
		{"show first seen last seen for 923461678183", "first_seen_last_seen"},
		{"when was 923001110001 first and last observed", "first_seen_last_seen"},
		{"show daily activity for ABC-123", "activity_by_day"},
		{"which files were ingested", "source_file_audit"},
		{"summarize this case", "executive_case_brief"},
		{"generate a report", "executive_case_brief"},
		{"generate a forensic intelligence report", "executive_case_brief"},
		{"what happened on 2026-07-10", "entity_timeline"},
		{"show top locations", "top_locations"},
		{"where was ABC-123 seen", "anpr_sightings"},
		{"show movement and base location", "geospatial_movement"},
		{"show plate and camera entities", "entity_activity"},
		{"summarize evidence for ABC-123 and show related entities", "entity_activity"},
		{"show relationship network for ABC-123", "relationship_network"},
		{"build timeline for ABC-123", "entity_timeline"},
		{"show source rows for ABC-123", "source_records"},
		{"show detected headers and schema", "schema_profile"},
		{"show duplicate and rejected row quality", "data_quality"},
		{"show duplicate uploads", "duplicate_upload_audit"},
		{"what limitations and missing data exist", "limitations_and_data_quality"},
		{"show available entities", "entity_activity"},
		{"what should I investigate next", "suspicious_patterns"},
		{"compare these numbers", "relationship_network"},
		{"is this case ready for production", "case_readiness"},
		{"show evidence health and readiness", "case_readiness"},
		{"find source evidence", "evidence"},
		{"Using both policy context and deterministic records, explain why exact counts must not be inferred from Knowledge Base chunks.", "evidence"},
		{"top contacts", "frequent_contacts"},
		{"please discover the hidden criminal intent", ""},
	}
	for _, tt := range tests {
		if got := chooseTemplate(tt.query, ""); got != tt.want {
			t.Fatalf("chooseTemplate(%q) = %q, want %q", tt.query, got, tt.want)
		}
	}
}

func TestCanonicalTemplateAliases(t *testing.T) {
	tests := map[string]string{
		"semantic":            "evidence",
		"policy":              "evidence",
		"canonical_sql_query": "canonical_records",
		"records_query":       "canonical_records",
		"files_ingested":      "source_file_audit",
		"duplicate_uploads":   "duplicate_upload_audit",
		"duration-extremes":   "duration_extremes",
	}
	for input, want := range tests {
		if got := canonicalTemplate(input); got != want {
			t.Fatalf("canonicalTemplate(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSourceFileAuditGroupsCanonicalSources(t *testing.T) {
	for _, fragment := range []string{
		"UNION ALL",
		"FROM forensic.records_ingest_jobs",
		"FROM forensic.kb_collection_assets",
		"FROM forensic.evidence_items",
		"GROUP BY source_file",
		"string_agg(DISTINCT evidence_id",
	} {
		if !strings.Contains(sourceFileAuditSQL, fragment) {
			t.Fatalf("source file audit must reconcile one canonical row per source; missing %q", fragment)
		}
	}
}

func TestChooseTemplateCanonicalRecords(t *testing.T) {
	tests := map[string]string{
		"query canonical records for record type cdr":       "canonical_records",
		"filter forensic.records by raw_payload status":     "canonical_records",
		"show rows where field exists in raw payload":       "canonical_records",
		"find records for batch id 7597b681-62c4-4a1f":      "canonical_records",
		"show CDR records where call type is GPRS":          "canonical_records",
		"how many rows where IMEI exists":                   "canonical_records",
		"show CDR records where duration seconds > 60":      "canonical_records",
		"How many records do we have for each record type?": "canonical_records",
		"show source rows for ABC-123":                      "source_records",
		"show detected headers and schema":                  "schema_profile",
	}
	for query, want := range tests {
		if got := chooseTemplate(query, ""); got != want {
			t.Fatalf("chooseTemplate(%q) = %q, want %q", query, got, want)
		}
	}
}

func TestApplyCanonicalQueryHintsFromNaturalLanguage(t *testing.T) {
	req := applyCanonicalQueryHints(hybridQueryRequest{
		Query: "show CDR records where call type is GPRS limit 3 oldest first",
		Limit: defaultHybridLimit,
	}, "canonical_records")
	if req.RecordType != "cdr" {
		t.Fatalf("record_type = %q, want cdr", req.RecordType)
	}
	if req.Limit != 3 {
		t.Fatalf("limit = %d, want 3", req.Limit)
	}
	if req.SortBy != "timestamp" || req.SortDirection != "asc" {
		t.Fatalf("sort = %q/%q, want timestamp/asc", req.SortBy, req.SortDirection)
	}
	if len(req.RawPayloadFilters) != 1 {
		t.Fatalf("raw filters = %#v, want one", req.RawPayloadFilters)
	}
	filter := req.RawPayloadFilters[0]
	if filter.Field != "call_type" || filter.Op != "eq" || filter.Value != "GPRS" {
		t.Fatalf("raw filter = %#v, want call_type eq GPRS", filter)
	}
}

func TestApplyCanonicalQueryHintsForSourceBatchAndFieldExistence(t *testing.T) {
	req := applyCanonicalQueryHints(hybridQueryRequest{
		Query: "show rows from source file seed_cdr_large.csv where IMEI exists and deleted_at missing offset 10 batch id 7597b681-62c4-4a1f-9b6f-97b438da5f97",
		Limit: defaultHybridLimit,
	}, "canonical_records")
	if req.SourceFile != "seed_cdr_large.csv" {
		t.Fatalf("source_file = %q", req.SourceFile)
	}
	if req.BatchID != "7597b681-62c4-4a1f-9b6f-97b438da5f97" {
		t.Fatalf("batch_id = %q", req.BatchID)
	}
	if req.Offset != 10 {
		t.Fatalf("offset = %d, want 10", req.Offset)
	}
	if len(req.FieldExists) != 1 || req.FieldExists[0] != "imei" {
		t.Fatalf("field_exists = %#v", req.FieldExists)
	}
	if len(req.FieldNotExists) != 1 || req.FieldNotExists[0] != "deleted_at" {
		t.Fatalf("field_not_exists = %#v", req.FieldNotExists)
	}
}

func TestApplyCanonicalQueryHintsDoesNotOverrideExplicitFilters(t *testing.T) {
	req := applyCanonicalQueryHints(hybridQueryRequest{
		Query:      "show CDR records where call type is GPRS",
		RecordType: "anpr",
		RawPayloadFilters: []CanonicalPayloadFilter{
			{Field: "status", Op: "eq", Value: "ok"},
		},
	}, "canonical_records")
	if req.RecordType != "anpr" {
		t.Fatalf("record_type = %q, want explicit anpr", req.RecordType)
	}
	if len(req.RawPayloadFilters) != 2 {
		t.Fatalf("raw filters = %#v, want explicit plus extracted", req.RawPayloadFilters)
	}
	if req.RawPayloadFilters[0].Field != "status" {
		t.Fatalf("explicit filter was not preserved first: %#v", req.RawPayloadFilters)
	}
}

func TestApplyCanonicalQueryHintsForComparisons(t *testing.T) {
	req := applyCanonicalQueryHints(hybridQueryRequest{
		Query: "show CDR records where duration seconds greater than 60 and raw payload call type is not SMS",
		Limit: defaultHybridLimit,
	}, "canonical_records")
	if req.RecordType != "cdr" {
		t.Fatalf("record_type = %q, want cdr", req.RecordType)
	}
	if len(req.RawPayloadFilters) != 2 {
		t.Fatalf("raw filters = %#v, want two", req.RawPayloadFilters)
	}
	byField := map[string]CanonicalPayloadFilter{}
	for _, filter := range req.RawPayloadFilters {
		byField[filter.Field] = filter
	}
	if filter := byField["duration_seconds"]; filter.Op != "gt" || filter.Value != "60" {
		t.Fatalf("duration filter = %#v, want duration_seconds gt 60", filter)
	}
	if filter := byField["call_type"]; filter.Op != "ne" || filter.Value != "SMS" {
		t.Fatalf("call_type filter = %#v, want call_type ne SMS", filter)
	}
}

func TestCanonicalRecordsQueryBuilderAppliesFilters(t *testing.T) {
	req := hybridQueryRequest{
		TenantID:       "default",
		CollectionID:   "case-alpha",
		Target:         "+92 346 167 8183",
		Targets:        []string{"ABC-123"},
		RecordType:     "cdr",
		DateFrom:       "2026-07-10T00:00:00Z",
		DateTo:         "2026-07-11T00:00:00Z",
		SourceFile:     "cdr.csv",
		BatchID:        "7597b681-62c4-4a1f-9b6f-97b438da5f97",
		Limit:          25,
		Offset:         50,
		SortBy:         "source_file",
		SortDirection:  "asc",
		FieldExists:    []string{"imei"},
		FieldNotExists: []string{"deleted_at"},
		FieldFilters:   []CanonicalFieldFilter{{Field: "imsi", Op: "exists"}},
		RawPayloadFilters: []CanonicalPayloadFilter{
			{Field: "direction", Op: "eq", Value: "outgoing"},
			{Field: "location", Op: "contains", Value: "LHR"},
			{Field: "call_type", Op: "in", Values: []any{"SMS", "GPRS"}},
		},
	}
	built, err := buildCanonicalRecordsQuery(req)
	if err != nil {
		t.Fatalf("buildCanonicalRecordsQuery() error = %v", err)
	}
	for _, want := range []string{
		"tenant_id = $1",
		"collection_id = $2",
		"record_type = $3",
		"source_file = $4",
		"batch_id::text = $5",
		"timestamp >= $6::timestamptz",
		"timestamp < $7::timestamptz",
		"primary_target",
		"secondary_target",
		"raw_payload ->>",
		"raw_payload ?",
		"jsonb_each_text(raw_payload)",
		"jsonb_object_keys(raw_payload)",
		"metadata -> 'normalized_fields'",
		"AND NOT (raw_payload ?",
	} {
		if !strings.Contains(built.WhereSQL, want) {
			t.Fatalf("WHERE SQL missing %q:\n%s", want, built.WhereSQL)
		}
	}
	if len(built.Args) != 19 {
		t.Fatalf("args len = %d, want 19: %#v", len(built.Args), built.Args)
	}
	if built.OrderBy != "source_file ASC NULLS LAST, timestamp DESC NULLS LAST, row_number ASC NULLS LAST" {
		t.Fatalf("order by = %q", built.OrderBy)
	}
	targets := built.Filters["targets"].([]string)
	if len(targets) != 2 || targets[0] != "923461678183" || targets[1] != "ABC-123" {
		t.Fatalf("targets filter = %#v", targets)
	}
	if built.Filters["canonical_table"] != nil {
		t.Fatalf("builder filters should not include query-plan-only keys: %#v", built.Filters)
	}
}

func TestCanonicalRecordsQueryBuilderSortDefaultsAndRejectsBadPayloadOp(t *testing.T) {
	req := hybridQueryRequest{
		TenantID:          "default",
		CollectionID:      "case-alpha",
		SortBy:            "raw_payload;drop",
		SortDirection:     "sideways",
		RawPayloadFilters: []CanonicalPayloadFilter{{Field: "direction", Op: "regex", Value: "outgoing"}},
	}
	if _, err := buildCanonicalRecordsQuery(req); err == nil {
		t.Fatalf("expected unsupported payload op error")
	}
	req.RawPayloadFilters = nil
	built, err := buildCanonicalRecordsQuery(req)
	if err != nil {
		t.Fatalf("buildCanonicalRecordsQuery() error = %v", err)
	}
	if built.OrderBy != "timestamp DESC NULLS LAST, source_file ASC NULLS LAST, row_number ASC NULLS LAST" {
		t.Fatalf("order by = %q", built.OrderBy)
	}
}

func TestIPDRSessionScopePreservesRetainedLegacyRowsWithoutInference(t *testing.T) {
	for _, want := range []string{
		"raw_payload->>'source_ip'",
		"raw_payload->>'destination_ip'",
		"raw_payload->>'subscriber_id'",
		"raw_payload->>'session_id'",
		"raw_payload->>'domain'",
		"raw_payload->>'bytes'",
		"byte_count_text ~ '^[0-9]{1,19}$'",
		"source_port_text::int BETWEEN 1 AND 65535",
	} {
		if !strings.Contains(ipdrSessionScopeSQL, want) {
			t.Fatalf("IPDR compatibility scope missing %q:\n%s", want, ipdrSessionScopeSQL)
		}
	}
	for _, forbidden := range []string{"geoip", "dns_lookup", "subscriber_owner", "route_inference"} {
		if strings.Contains(strings.ToLower(ipdrSessionScopeSQL), forbidden) {
			t.Fatalf("IPDR compatibility scope contains forbidden inference %q", forbidden)
		}
	}
}

func TestIPDREndpointSummaryCarriesRepresentativeAggregateProvenance(t *testing.T) {
	for _, field := range []string{"source_file", "row_number", "row_hash", "evidence_id"} {
		fragment := "array_agg(" + field + " ORDER BY session_start, record_id"
		if !strings.Contains(ipdrEndpointSummarySQL, fragment) {
			t.Fatalf("IPDR endpoint aggregate is missing representative %s provenance: %s", field, ipdrEndpointSummarySQL)
		}
	}
	for _, forbidden := range []string{"geoip", "subscriber_owner", "route_inference"} {
		if strings.Contains(strings.ToLower(ipdrEndpointSummarySQL), forbidden) {
			t.Fatalf("IPDR endpoint summary contains forbidden inference %q", forbidden)
		}
	}
}

func TestNormalizeDBValueFormatsUUIDsForJSONAndCitations(t *testing.T) {
	value := [16]byte{0x64, 0x35, 0xf6, 0x35, 0xb4, 0xfb, 0x4a, 0xed, 0xa5, 0xf5, 0x77, 0xc4, 0x52, 0x80, 0xf8, 0x81}
	want := "6435f635-b4fb-4aed-a5f5-77c45280f881"
	if got := normalizeDBValue(value); got != want {
		t.Fatalf("normalizeDBValue(UUID) = %#v, want %q", got, want)
	}
	if got := normalizeDBValue(pgtype.UUID{Bytes: value, Valid: true}); got != want {
		t.Fatalf("normalizeDBValue(pgtype.UUID) = %#v, want %q", got, want)
	}
}

func TestEnterpriseProvenancePreservesEvidenceAndRowHash(t *testing.T) {
	items := enterpriseProvenance(hybridQueryRequest{CollectionID: "case-alpha"}, []map[string]any{{
		"source_file": "seed_ipdr.csv", "row_number": int64(4),
		"evidence_id": "6435f635-b4fb-4aed-a5f5-77c45280f881",
		"row_hash":    "881c502a9b349d29f4336c192e9b339b74bb88af58ea49ca798322342833d157",
	}}, nil, nil)
	if len(items) != 1 {
		t.Fatalf("provenance len = %d, want 1", len(items))
	}
	if items[0]["evidence_id"] != "6435f635-b4fb-4aed-a5f5-77c45280f881" {
		t.Fatalf("evidence ID not preserved: %#v", items[0])
	}
	if items[0]["row_hash"] != "881c502a9b349d29f4336c192e9b339b74bb88af58ea49ca798322342833d157" {
		t.Fatalf("row hash not preserved: %#v", items[0])
	}
}

func TestCanonicalRecordsQueryBuilderNumericComparison(t *testing.T) {
	built, err := buildCanonicalRecordsQuery(hybridQueryRequest{
		TenantID:     "default",
		CollectionID: "case-alpha",
		RawPayloadFilters: []CanonicalPayloadFilter{
			{Field: "duration_seconds", Op: "gt", Value: "60"},
			{Field: "call_type", Op: "ne", Value: "SMS"},
		},
	})
	if err != nil {
		t.Fatalf("buildCanonicalRecordsQuery() error = %v", err)
	}
	if !strings.Contains(built.WhereSQL, "::numeric END >") {
		t.Fatalf("WHERE SQL missing numeric comparison:\n%s", built.WhereSQL)
	}
	if !strings.Contains(built.WhereSQL, "<>") {
		t.Fatalf("WHERE SQL missing not-equal comparison:\n%s", built.WhereSQL)
	}
	if _, err := buildCanonicalRecordsQuery(hybridQueryRequest{
		TenantID:     "default",
		CollectionID: "case-alpha",
		RawPayloadFilters: []CanonicalPayloadFilter{
			{Field: "duration_seconds", Op: "gt", Value: "sixty"},
		},
	}); err == nil {
		t.Fatalf("expected nonnumeric comparison value to fail")
	}
}

func TestApplyCanonicalRequestToQueryPlan(t *testing.T) {
	plan := applyCanonicalRequestToQueryPlan(QueryPlan{AppliedFilters: map[string]any{}}, hybridQueryRequest{
		Target:        "923461678183",
		Targets:       []string{"923461678183", "ABC-123"},
		RecordType:    "cdr",
		SourceFile:    "cdr.csv",
		BatchID:       "batch-1",
		Limit:         10,
		Offset:        20,
		SortBy:        "ingested_at",
		SortDirection: "asc",
		FieldExists:   []string{"imei"},
		RawPayloadFilters: []CanonicalPayloadFilter{
			{Field: "direction", Op: "eq", Value: "outgoing"},
		},
	}, "canonical_records")
	if plan.ExecutionStrategy != "canonical_sql_query" {
		t.Fatalf("execution strategy = %q", plan.ExecutionStrategy)
	}
	if plan.AppliedFilters["canonical_table"] != "forensic.records" {
		t.Fatalf("applied filters = %#v", plan.AppliedFilters)
	}
	if len(plan.TargetIdentifiers) != 2 {
		t.Fatalf("target identifiers = %#v", plan.TargetIdentifiers)
	}
}

func TestCanonicalPlannerDoesNotPromoteAttributeWordsAsTargets(t *testing.T) {
	req := hybridQueryRequest{
		Query:      "show canonical records where raw_payload call_type is GPRS",
		RecordType: "cdr",
		RawPayloadFilters: []CanonicalPayloadFilter{
			{Field: "call_type", Op: "eq", Value: "GPRS"},
		},
	}
	if shouldPromotePlannerTargets(req, "canonical_records", false) {
		t.Fatalf("canonical filtered requests should not promote query words as entity targets")
	}
	if !shouldPromotePlannerTargets(req, "canonical_records", true) {
		t.Fatalf("explicit target requests should preserve caller-provided target filters")
	}
}

func TestNormalizeCollectionID(t *testing.T) {
	tests := map[string]string{
		"Forensic_Records_Analyst": "records-demo",
		"forensic_records_analyst": "records-demo",
		"records-demo":             "records-demo",
		"case-42":                  "case-42",
	}
	for input, want := range tests {
		if got := normalizeCollectionID(input); got != want {
			t.Fatalf("normalizeCollectionID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPlanRuntimeQueryExtractsTarget(t *testing.T) {
	plan := planRuntimeQuery(hybridQueryRequest{Query: "where was ABC-123 seen", CollectionID: "records-demo"})
	if plan.Template != "anpr_sightings" {
		t.Fatalf("template = %q, want anpr_sightings", plan.Template)
	}
	if plan.Target != "ABC-123" {
		t.Fatalf("target = %q, want ABC-123", plan.Target)
	}
	if plan.TargetType != "identifier" {
		t.Fatalf("target_type = %q, want identifier", plan.TargetType)
	}
	if len(plan.Targets) != 1 || plan.Targets[0] != "ABC-123" {
		t.Fatalf("targets = %#v, want [ABC-123]", plan.Targets)
	}
	if plan.Source != "runtime_query" {
		t.Fatalf("source = %q, want runtime_query", plan.Source)
	}
}

func TestPlanRuntimeQueryExtractsDateWithoutFalseTarget(t *testing.T) {
	plan := planRuntimeQuery(hybridQueryRequest{Query: "what happened on 2026-07-10?", CollectionID: "records-demo"})
	if plan.Template != "entity_timeline" {
		t.Fatalf("template = %q, want entity_timeline", plan.Template)
	}
	if plan.Target != "" {
		t.Fatalf("target = %q, want empty", plan.Target)
	}
	if plan.DateFrom != "2026-07-10T00:00:00Z" {
		t.Fatalf("date_from = %q, want 2026-07-10T00:00:00Z", plan.DateFrom)
	}
	if plan.DateTo != "2026-07-11T00:00:00Z" {
		t.Fatalf("date_to = %q, want 2026-07-11T00:00:00Z", plan.DateTo)
	}
	if len(plan.FieldHints) == 0 || plan.FieldHints[0] != "time" {
		t.Fatalf("field_hints = %#v, want time hint", plan.FieldHints)
	}
}

func TestExtractDateRange(t *testing.T) {
	from, to := extractDateRange("compare events from 2026-07-10 to 2026-07-12")
	if from != "2026-07-10T00:00:00Z" {
		t.Fatalf("from = %q, want 2026-07-10T00:00:00Z", from)
	}
	if to != "2026-07-13T00:00:00Z" {
		t.Fatalf("to = %q, want 2026-07-13T00:00:00Z", to)
	}
}

func TestExtractNamedMonthDateRange(t *testing.T) {
	for _, query := range []string{
		"923001234567 ki 10 July 2026 ki CDR activity dikhao.",
		"10 جولائی 2026 کو 923001234567 کی CDR سرگرمی دکھائیں",
	} {
		from, to := extractDateRange(query)
		if from != "2026-07-10T00:00:00Z" || to != "2026-07-11T00:00:00Z" {
			t.Fatalf("named-month range for %q = %q to %q", query, from, to)
		}
	}
}

func TestNeedsClarificationForTargetSpecificQueries(t *testing.T) {
	if !needsClarification("where was this number most often observed", "top_locations", "") {
		t.Fatalf("expected target-specific location query to require clarification")
	}
	if needsClarification("show top locations", "top_locations", "") {
		t.Fatalf("collection-level top locations should not require clarification")
	}
	if needsClarification("where was this number most often observed", "top_locations", "923461678183") {
		t.Fatalf("provided target should not require clarification")
	}
	if !needsClarification("compare these numbers", "relationship_network", "") {
		t.Fatalf("expected comparison query without targets to require clarification")
	}
	if needsClarification("compare 923461678183 and 923461678184", "relationship_network", "") {
		t.Fatalf("comparison with two targets should not require clarification")
	}
	for _, template := range []string{"ipdr_subscriber_sessions", "ipdr_concurrent_sessions", "ipdr_timeline"} {
		if !needsClarification("show bounded IPDR details", template, "") {
			t.Fatalf("expected %s to require an explicit target", template)
		}
		if needsClarification("show bounded IPDR details for 10.20.1.7", template, "10.20.1.7") {
			t.Fatalf("did not expect %s to require clarification with an explicit target", template)
		}
	}
}

func TestCountResultRows(t *testing.T) {
	got := countResultRows(map[string]any{
		"row_count": 99,
		"records": []map[string]any{
			{"id": 1},
			{"id": 2},
		},
		"nested": map[string]any{
			"more": []any{map[string]any{"id": 3}},
		},
	})
	if got != 3 {
		t.Fatalf("countResultRows() = %d, want 3", got)
	}

	canonical := countResultRows(map[string]any{
		"canonical_records": []map[string]any{{"record_id": "1"}, {"record_id": "2"}},
		"filters": map[string]any{
			"raw_payload_filters": []map[string]any{{"field": "call_type", "op": "eq", "value": "GPRS"}},
		},
		"sort":        map[string]any{"by": "timestamp"},
		"provenance":  []map[string]any{{"record_id": "1"}},
		"total_count": int64(500),
	})
	if canonical != 2 {
		t.Fatalf("canonical countResultRows() = %d, want 2", canonical)
	}
	total := canonicalAnswerRowCount("canonical_records", map[string]any{
		"canonical_records": []map[string]any{{"record_id": "1"}, {"record_id": "2"}},
		"total_count":       int64(5861),
	})
	if total != 5861 {
		t.Fatalf("canonicalAnswerRowCount() = %d, want total count 5861", total)
	}
}

func TestScoreCaseReadinessReady(t *testing.T) {
	readiness := scoreCaseReadiness(
		map[string]any{
			"jobs": []map[string]any{{"status": "completed"}},
			"record_families": []map[string]any{
				{"record_type": "cdr", "total_rows": int64(1000), "inserted_rows": int64(1000)},
			},
		},
		map[string]any{
			"summary": map[string]any{
				"total_rows":     int64(1000),
				"accepted_rows":  int64(1000),
				"duplicate_rows": int64(0),
				"rejected_rows":  int64(0),
				"failed_jobs":    int64(0),
				"completed_jobs": int64(1),
			},
		},
		map[string]any{
			"assets": []map[string]any{{"rag_status": "ready"}},
		},
	)
	if readiness["level"] != "ready" {
		t.Fatalf("level = %v, want ready: %#v", readiness["level"], readiness)
	}
	if readiness["score"].(int) < 90 {
		t.Fatalf("score = %v, want >= 90", readiness["score"])
	}
}

func TestScoreCaseReadinessBlocked(t *testing.T) {
	readiness := scoreCaseReadiness(
		map[string]any{
			"jobs":            []map[string]any{},
			"record_families": []map[string]any{},
		},
		map[string]any{
			"summary": map[string]any{
				"failed_jobs":   int64(1),
				"rejected_rows": int64(12),
			},
		},
		map[string]any{
			"assets": []map[string]any{},
		},
	)
	if readiness["level"] != "blocked" {
		t.Fatalf("level = %v, want blocked: %#v", readiness["level"], readiness)
	}
	issues := readiness["issues"].([]string)
	if len(issues) == 0 {
		t.Fatalf("expected readiness issues")
	}
}

func TestReadinessChecks(t *testing.T) {
	checks := readinessChecks(map[string]any{
		"level":           "needs_review",
		"score":           70,
		"accepted_rows":   int64(10),
		"record_families": int64(1),
		"failed_jobs":     int64(0),
		"rejected_rows":   int64(2),
		"duplicate_rows":  int64(0),
		"kb_assets":       int64(1),
	})
	if len(checks) != 7 {
		t.Fatalf("checks len = %d, want 7", len(checks))
	}
	if checks[4]["status"] != "review" {
		t.Fatalf("rejected row check status = %v, want review", checks[4]["status"])
	}
}

func TestBuildEnterprisePayloadForHybridResult(t *testing.T) {
	req := hybridQueryRequest{
		TenantID:     "default",
		CollectionID: "case-alpha",
		Query:        "summarize evidence and show source records for 923461678183",
		Template:     "source_records",
		Target:       "923461678183",
		DateFrom:     "2026-07-10T00:00:00Z",
		DateTo:       "2026-07-11T00:00:00Z",
	}
	resp := hybridQueryResponse{
		Intent: intentHybrid,
		Route:  []string{"records_sql", "kb_rag"},
		Planner: map[string]any{
			"confidence": 0.93,
		},
		Template: "source_records",
		Records: map[string]any{
			"row_count": 1,
			"summary": map[string]any{
				"total_rows":     int64(20),
				"duplicate_rows": int64(2),
			},
			"source_records": []map[string]any{
				{
					"record_type":   "cdr",
					"timestamp":     "2026-07-10T12:15:00Z",
					"source_file":   "cdr.csv",
					"row_number":    int64(7),
					"source_number": "923461678183",
				},
			},
		},
		Evidence: map[string]any{
			"mode": "vector_search",
			"results": []any{
				map[string]any{
					"content":    "Policy evidence preview",
					"similarity": 0.82,
					"metadata": map[string]any{
						"file_name": "case-notes.md",
					},
				},
			},
		},
		Answer: map[string]any{
			"records_summary":   "Retrieved a capped, auditable set of matching source rows.",
			"records_row_count": 1,
			"evidence_summary":  "Retrieved Knowledge Base evidence using vector_search",
			"evidence_count":    1,
		},
		Warnings: []string{"A capped preview was returned."},
	}

	enterprise := buildEnterprisePayload(req, resp)
	if enterprise["summary"] == "" {
		t.Fatalf("expected enterprise summary")
	}
	grid := enterprise["data_grid"].(map[string]any)
	if grid["count"] != 2 {
		t.Fatalf("data grid count = %v, want 2", grid["count"])
	}
	provenance := enterprise["provenance"].([]map[string]any)
	if len(provenance) < 2 {
		t.Fatalf("provenance len = %d, want records and KB provenance", len(provenance))
	}
	coverage := enterprise["coverage"].(map[string]any)
	if coverage["queried_record_sql"] != true || coverage["queried_kb"] != true {
		t.Fatalf("coverage = %#v, want SQL and KB flags", coverage)
	}
	claims := enterprise["synthesis"].(map[string]any)["claims"].([]map[string]any)
	if len(claims) < 3 {
		t.Fatalf("claims len = %d, want deterministic and semantic claims", len(claims))
	}
	limitations := enterprise["limitations"].([]string)
	if len(limitations) != 1 || limitations[0] != "A capped preview was returned." {
		t.Fatalf("limitations = %#v", limitations)
	}
}

func TestBuildEnterprisePayloadForClarification(t *testing.T) {
	resp := hybridQueryResponse{
		Intent:   intentClarify,
		Route:    []string{"clarification"},
		Template: "top_locations",
		Answer: map[string]any{
			"clarification_required": true,
			"clarification":          "Which target should I analyze?",
			"limitations":            []any{"A target-specific query needs an identifier."},
		},
	}

	enterprise := buildEnterprisePayload(hybridQueryRequest{CollectionID: "case-alpha"}, resp)
	if got := enterprise["summary"].(string); !strings.Contains(got, "More information is required") {
		t.Fatalf("summary = %q", got)
	}
	grid := enterprise["data_grid"].(map[string]any)
	if grid["count"] != 0 {
		t.Fatalf("data grid count = %v, want 0", grid["count"])
	}
	limitations := enterprise["limitations"].([]string)
	if len(limitations) != 2 {
		t.Fatalf("limitations = %#v, want clarification and limitation", limitations)
	}
}

func TestTemporalActivityCountsMatchedEventsAndLabelsCalculationComponents(t *testing.T) {
	records := map[string]any{
		"hourly_activity":   []map[string]any{{"hour_of_day": int64(9), "event_count": int64(3)}, {"hour_of_day": int64(10), "event_count": int64(1)}},
		"daily_activity":    []map[string]any{{"day_start": "2026-07-10T00:00:00Z", "event_count": int64(4)}},
		"nocturnal":         map[string]any{"nocturnal_events": int64(0)},
		"duration_stats":    map[string]any{"nonzero_duration_events": int64(2), "average_nonzero_duration": 27.5},
		"duration_extremes": []map[string]any{{"metric": "shortest_nonzero_call"}, {"metric": "longest_call"}},
	}
	if got := canonicalAnswerRowCount("temporal_activity", records); got != 4 {
		t.Fatalf("matched event count = %d, want 4", got)
	}
	enterprise := buildEnterprisePayload(hybridQueryRequest{CollectionID: "case-alpha"}, hybridQueryResponse{
		Intent: intentRecords, Template: "temporal_activity", Route: []string{"records_sql"}, Records: records,
		Answer: map[string]any{"records_row_count": 4, "records_summary": "Computed temporal activity."},
	})
	grid := enterprise["data_grid"].(map[string]any)
	if grid["title"] != "Temporal calculation components" || grid["count_label"] != "calculation components" || grid["count"] != 7 {
		t.Fatalf("temporal grid semantics are ambiguous: %#v", grid)
	}
}

func TestBuildEnterprisePayloadIncludesCoverageAndTelemetry(t *testing.T) {
	req := hybridQueryRequest{
		TenantID:     "default",
		CollectionID: "case-alpha",
		Query:        "what happened on 2026-07-10",
		DateFrom:     "2026-07-10T00:00:00Z",
		DateTo:       "2026-07-11T00:00:00Z",
	}
	resp := hybridQueryResponse{
		Intent:   intentRecords,
		Route:    []string{"records_sql"},
		Template: "entity_timeline",
		Planner:  map[string]any{"confidence": 0.8},
		QueryPlan: QueryPlan{
			InterpretedIntent: "records",
			SelectedTemplate:  "entity_timeline",
			DateBounds:        TimeRange{From: req.DateFrom, To: req.DateTo},
			Confidence:        0.8,
			ExecutionStrategy: "sql_deterministic",
		},
		Coverage: CoverageSummary{
			CollectionMinTimestamp: "2026-04-02T01:22:34Z",
			CollectionMaxTimestamp: "2026-06-20T19:08:01Z",
			TotalIndexedRecords:    14648,
			RecordFamiliesPresent: []RecordFamilyCoverage{
				{RecordType: "cdr", Count: 8634},
			},
			NearestActivity:     NearestActivity{Before: "2026-06-20T19:08:01Z"},
			ValidTargetExamples: []string{"9234******83"},
		},
		Answer: map[string]any{
			"records_summary":    "Computed a chronological cross-record timeline from CDR and generic records.",
			"records_row_count":  0,
			"records_status":     "no_matching_records",
			"records_limitation": "No matching structured records were returned for the selected collection, template, and target.",
		},
		Telemetry: QueryTelemetry{
			RequestID:         "req-123",
			DBLatencyMS:       7,
			TotalLatencyMS:    9,
			PlannerConfidence: 0.8,
			ExecutionPath:     "records_sql",
		},
	}

	enterprise := buildEnterprisePayload(req, resp)
	if enterprise["status"] != "no_results" {
		t.Fatalf("status = %v, want no_results", enterprise["status"])
	}
	coverage := enterprise["coverage"].(map[string]any)
	if coverage["collection_min_timestamp"] != "2026-04-02T01:22:34Z" {
		t.Fatalf("coverage = %#v", coverage)
	}
	if coverage["total_indexed_records"] != int64(14648) {
		t.Fatalf("coverage total = %v, want 14648", coverage["total_indexed_records"])
	}
	telemetry := enterprise["telemetry"].(QueryTelemetry)
	if telemetry.RequestID != "req-123" || telemetry.DBLatencyMS != 7 {
		t.Fatalf("telemetry = %#v", telemetry)
	}
	actions := enterprise["recommended_actions"].([]map[string]any)
	if len(actions) == 0 || actions[0]["label"] != "Expand date bounds" {
		t.Fatalf("actions = %#v", actions)
	}
}

func TestBuildEnterprisePayloadCapsLargeResultGrid(t *testing.T) {
	rows := make([]map[string]any, 150)
	for i := range rows {
		rows[i] = map[string]any{
			"record_type": "cdr",
			"row_number":  i + 1,
			"source_file": "large.csv",
		}
	}
	resp := hybridQueryResponse{
		Intent:   intentRecords,
		Route:    []string{"records_sql"},
		Template: "source_records",
		Planner:  map[string]any{"confidence": 0.91},
		Records:  map[string]any{"source_records": rows, "row_count": 150},
		Answer: map[string]any{
			"records_summary":   "Retrieved a capped, auditable set of matching source rows.",
			"records_row_count": 150,
		},
	}

	enterprise := buildEnterprisePayload(hybridQueryRequest{CollectionID: "case-alpha"}, resp)
	grid := enterprise["data_grid"].(map[string]any)
	if grid["count"] != 100 {
		t.Fatalf("data grid count = %v, want 100", grid["count"])
	}
	limitations := strings.Join(enterprise["limitations"].([]string), "\n")
	if !strings.Contains(limitations, "capped at 100 displayed rows from 150 matching records") {
		t.Fatalf("limitations = %q", limitations)
	}
}

func TestRuntimePlanBuildsTypedQueryPlan(t *testing.T) {
	plan := planRuntimeQuery(hybridQueryRequest{Query: "summarize evidence and show source records for 923461678183"})
	if plan.QueryPlan.SelectedTemplate != "source_records" {
		t.Fatalf("selected template = %q", plan.QueryPlan.SelectedTemplate)
	}
	if plan.QueryPlan.ExecutionStrategy != "hybrid_llm" {
		t.Fatalf("execution strategy = %q, want hybrid_llm", plan.QueryPlan.ExecutionStrategy)
	}
	if len(plan.QueryPlan.TargetIdentifiers) != 1 || plan.QueryPlan.TargetIdentifiers[0] != "923461678183" {
		t.Fatalf("targets = %#v", plan.QueryPlan.TargetIdentifiers)
	}
}

func TestBoundedLLMFallbackReasonOnServiceFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "offline", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	summary, fallback := synthesizeWithBoundedLLM(
		context.Background(),
		config{LocalAIURL: server.URL, SynthesisModel: "qwen3-0.6b"},
		hybridQueryRequest{Query: "summarize records"},
		hybridQueryResponse{Template: "source_records", Answer: map[string]any{"records_row_count": 1}},
	)
	if summary != "" {
		t.Fatalf("summary = %q, want empty on fallback", summary)
	}
	if !strings.Contains(fallback, "defaulted to deterministic engine") {
		t.Fatalf("fallback = %q", fallback)
	}
}

func TestSQLWithRequestID(t *testing.T) {
	ctx := contextWithRequestID(context.Background(), "req-123 */ bad")
	got := sqlWithRequestID(ctx, "SELECT 1")
	if !strings.Contains(got, "request_id: req-123bad") {
		t.Fatalf("sql = %q", got)
	}
	if strings.Contains(got, "*/ bad") {
		t.Fatalf("request id was not sanitized: %q", got)
	}
}

func TestExtractTarget(t *testing.T) {
	tests := map[string]string{
		"show calls for 923461678183":           "923461678183",
		"show calls for +92 346 167 8183":       "923461678183",
		"where was ABC 123 seen":                "ABC-123",
		"tower activity for LHR-GUL-014":        "LHR-GUL-014",
		"show traffic from 192.168.100.45":      "192.168.100.45",
		"show evidence for analyst@example.org": "analyst@example.org",
		"how many gprs records":                 "GPRS",
		"what happened on 2026-07-10":           "",
		"what happened on 20260710":             "",
		"show call type breakdown":              "",
		"give me data quality summary":          "",
		"summarize records-demo":                "",
	}
	for query, want := range tests {
		if got := extractTarget(query); got != want {
			t.Fatalf("extractTarget(%q) = %q, want %q", query, got, want)
		}
	}
}

func TestExtractTargets(t *testing.T) {
	targets := extractTargets("compare 923461678183 with 923461678184 and analyst@example.org")
	want := []string{"923461678183", "923461678184", "analyst@example.org"}
	if len(targets) != len(want) {
		t.Fatalf("targets = %#v, want %#v", targets, want)
	}
	for i := range want {
		if targets[i] != want[i] {
			t.Fatalf("targets = %#v, want %#v", targets, want)
		}
	}
}

func TestClassifyTargetType(t *testing.T) {
	tests := map[string]string{
		"923461678183":        "phone",
		"analyst@example.org": "email",
		"192.168.100.45":      "ip",
		"GPRS":                "call_type",
		"LHR-GUL-014":         "identifier",
		"UNKNOWN":             "entity",
		"":                    "",
	}
	for target, want := range tests {
		if got := classifyTargetType(target); got != want {
			t.Fatalf("classifyTargetType(%q) = %q, want %q", target, got, want)
		}
	}
}

func TestExtractFieldHints(t *testing.T) {
	hints := extractFieldHints("show source files, longest duration, and missing data")
	want := []string{"duration", "source_file", "quality"}
	if len(hints) != len(want) {
		t.Fatalf("hints = %#v, want %#v", hints, want)
	}
	for i := range want {
		if hints[i] != want[i] {
			t.Fatalf("hints = %#v, want %#v", hints, want)
		}
	}
}

func TestQueryTermsAndLexicalScore(t *testing.T) {
	terms := queryTerms("ABC-123 Gate 4")
	if len(terms) != 3 {
		t.Fatalf("terms len = %d, want 3: %#v", len(terms), terms)
	}
	score := lexicalScore("abc-123 appeared at gate 4. gate 4 repeated.", terms)
	if score != 4 {
		t.Fatalf("score = %d, want 4", score)
	}
}

func TestQueryTermsDropsRoutingStopwords(t *testing.T) {
	terms := queryTerms("summarize evidence for ABC-123 and show related entities")
	want := []string{"abc", "123"}
	if len(terms) != len(want) {
		t.Fatalf("terms = %#v, want %#v", terms, want)
	}
	for i := range want {
		if terms[i] != want[i] {
			t.Fatalf("terms = %#v, want %#v", terms, want)
		}
	}
}

func TestUserFacingKBSearchError(t *testing.T) {
	err := userFacingKBSearchError(fakeError("kb search status=500 body={\"error\":\"nResults must be <= the number of documents in the collection\"}"))
	want := "KB vector index has fewer searchable chunks than requested for this collection"
	if err != want {
		t.Fatalf("userFacingKBSearchError() = %q, want %q", err, want)
	}
}

type fakeError string

func (e fakeError) Error() string {
	return string(e)
}

func TestEscapeEntryPath(t *testing.T) {
	got := escapeEntryPath("uuid/source [draft].csv")
	want := "uuid/source%20%5Bdraft%5D.csv"
	if got != want {
		t.Fatalf("escapeEntryPath() = %q, want %q", got, want)
	}
}
