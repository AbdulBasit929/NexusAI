package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/httpclient"
)

const (
	defaultHybridLimit = 20
	maxHybridLimit     = 100
)

var targetPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`),
	regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
	regexp.MustCompile(`(?i)\b[A-Z]{2,6}(?:[-_][A-Z0-9]{2,8}){1,4}\b`),
	regexp.MustCompile(`(?i)\b[A-Z]{1,4}[- ]?\d{1,6}[A-Z]?\b`),
	regexp.MustCompile(`\b\+?\d(?:[\s\-]?\d){7,18}\b`),
}

type queryIntent string

const (
	intentRecords  queryIntent = "records"
	intentSemantic queryIntent = "semantic"
	intentHybrid   queryIntent = "hybrid"
	intentClarify  queryIntent = "clarification"
)

type hybridQueryRequest struct {
	TenantID          string                   `json:"tenant_id"`
	UserID            string                   `json:"user_id"`
	CollectionID      string                   `json:"collection_id"`
	Query             string                   `json:"query"`
	Target            string                   `json:"target"`
	Targets           []string                 `json:"targets,omitempty"`
	Template          string                   `json:"template"`
	RecordType        string                   `json:"record_type,omitempty"`
	DateFrom          string                   `json:"date_from,omitempty"`
	DateTo            string                   `json:"date_to,omitempty"`
	SourceFile        string                   `json:"source_file,omitempty"`
	BatchID           string                   `json:"batch_id,omitempty"`
	RawPayloadFilters []CanonicalPayloadFilter `json:"raw_payload_filters,omitempty"`
	FieldFilters      []CanonicalFieldFilter   `json:"field_filters,omitempty"`
	FieldExists       []string                 `json:"field_exists,omitempty"`
	FieldNotExists    []string                 `json:"field_not_exists,omitempty"`
	Offset            int                      `json:"offset,omitempty"`
	SortBy            string                   `json:"sort_by,omitempty"`
	SortDirection     string                   `json:"sort_direction,omitempty"`
	Limit             int                      `json:"limit"`
	MaxKBResults      int                      `json:"max_kb_results"`
	SynthesisModel    string                   `json:"synthesis_model,omitempty"`
}

type CanonicalPayloadFilter struct {
	Field  string `json:"field"`
	Op     string `json:"op"`
	Value  any    `json:"value,omitempty"`
	Values []any  `json:"values,omitempty"`
}

type CanonicalFieldFilter struct {
	Field string `json:"field"`
	Op    string `json:"op"`
}

type hybridQueryResponse struct {
	CollectionID string          `json:"collection_id"`
	Intent       queryIntent     `json:"intent"`
	Template     string          `json:"template"`
	Target       string          `json:"target,omitempty"`
	Route        []string        `json:"route"`
	Policy       string          `json:"policy"`
	Planner      map[string]any  `json:"planner"`
	QueryPlan    QueryPlan       `json:"query_plan"`
	Coverage     CoverageSummary `json:"coverage,omitempty"`
	Records      map[string]any  `json:"records,omitempty"`
	Evidence     map[string]any  `json:"evidence,omitempty"`
	Answer       map[string]any  `json:"answer"`
	Enterprise   map[string]any  `json:"enterprise,omitempty"`
	Telemetry    QueryTelemetry  `json:"telemetry"`
	Warnings     []string        `json:"warnings,omitempty"`
	GeneratedAt  time.Time       `json:"generated_at"`
}

type queryTemplate struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	RecordTypes []string `json:"record_types"`
	Route       string   `json:"route"`
}

func queryTemplatesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"templates": supportedQueryTemplates(),
		})
	}
}

func hybridQueryHandler(cfg config, db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		requestID := requestIDFromHTTP(r)
		w.Header().Set("X-Request-ID", requestID)
		ctx := contextWithRequestID(r.Context(), requestID)
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		var req hybridQueryRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("decode query request: %w", err))
			return
		}
		req.TenantID = defaultString(req.TenantID, "default")
		req.CollectionID = normalizeCollectionID(req.CollectionID)
		req.Query = strings.TrimSpace(req.Query)
		req.Target = strings.TrimSpace(req.Target)
		req.Template = normalize(req.Template)
		req.RecordType = normalize(req.RecordType)
		req.SourceFile = strings.TrimSpace(req.SourceFile)
		req.BatchID = strings.TrimSpace(req.BatchID)
		req.SortBy = normalize(req.SortBy)
		req.SortDirection = normalize(req.SortDirection)
		req.SynthesisModel = strings.TrimSpace(req.SynthesisModel)
		req.Limit = clampLimit(req.Limit)
		req.Offset = clampOffset(req.Offset)
		req.MaxKBResults = clampKBResults(req.MaxKBResults)
		explicitTargetProvided := req.Target != "" || len(req.Targets) > 0
		if req.CollectionID == "" {
			writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
			return
		}
		if req.Query == "" && req.Template == "" {
			writeError(w, http.StatusBadRequest, errors.New("query or template is required"))
			return
		}

		planner := planRuntimeQuery(req)
		intent := planner.Intent
		template := planner.Template
		req = applyCanonicalQueryHints(req, template)
		plannerTargets := planner.Targets
		if shouldPromotePlannerTargets(req, template, explicitTargetProvided) {
			req.Target = planner.Target
		} else {
			plannerTargets = nil
		}
		if req.DateFrom == "" {
			req.DateFrom = planner.DateFrom
		}
		if req.DateTo == "" {
			req.DateTo = planner.DateTo
		}
		req.Targets = canonicalTargetSet(req.Target, req.Targets, plannerTargets)
		if req.Target == "" && len(req.Targets) > 0 {
			req.Target = req.Targets[0]
		}
		queryPlan := applyCanonicalRequestToQueryPlan(planner.QueryPlan, req, template)
		resp := hybridQueryResponse{
			CollectionID: req.CollectionID,
			Intent:       intent,
			Template:     template,
			Target:       req.Target,
			Policy:       "Exact analytics are computed from parameterized SQL templates. KB content is used only for evidence, source previews, and contextual grounding.",
			Planner: map[string]any{
				"source":           planner.Source,
				"confidence":       planner.Confidence,
				"extracted_target": planner.Target,
				"targets":          req.Targets,
				"target_type":      planner.TargetType,
				"field_hints":      planner.FieldHints,
				"date_from":        req.DateFrom,
				"date_to":          req.DateTo,
				"reason":           planner.Reason,
				"query_plan":       queryPlan,
			},
			QueryPlan:   queryPlan,
			Answer:      map[string]any{},
			GeneratedAt: time.Now().UTC(),
		}
		var dbLatencyMS int64
		var kbLatencyMS int64
		var llmLatencyMS int64

		if needsClarification(req.Query, template, req.Target) {
			resp.Intent = intentClarify
			resp.Route = []string{"clarification"}
			resp.Answer["clarification_required"] = true
			resp.Answer["clarification"] = clarificationQuestion(req.Query, template)
			resp.Answer["limitations"] = []string{"A target-specific query was detected, but no target identifier was provided or extractable from the request."}
			resp.Answer["guardrail"] = "No records query was run because the request needs one missing identifier."
			resp.Telemetry = buildQueryTelemetry(requestID, dbLatencyMS, kbLatencyMS, llmLatencyMS, startedAt, resp)
			resp.Enterprise = buildEnterprisePayload(req, resp)
			writeJSON(w, http.StatusOK, resp)
			return
		}

		if intent == intentRecords || intent == intentHybrid {
			if db == nil {
				writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
				return
			}
			dbStart := time.Now()
			records, err := runAnalyticalTemplate(ctx, db, req, template)
			dbLatencyMS += time.Since(dbStart).Milliseconds()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			resp.Records = records
			resp.Route = append(resp.Route, "records_sql")
			resp.Answer["records_summary"] = summarizeRecords(template, records)
			recordsRowCount := canonicalAnswerRowCount(template, records)
			resp.Answer["records_row_count"] = recordsRowCount
			if total, ok := canonicalTotalCount(records); ok {
				resp.Answer["records_returned_row_count"] = countResultRows(records["canonical_records"])
				resp.Answer["records_total_count"] = total
			}
			if recordsRowCount == 0 {
				resp.Answer["records_status"] = "no_matching_records"
				resp.Answer["records_limitation"] = "No matching structured records were returned for the selected collection, template, and target."
			} else {
				resp.Answer["records_status"] = "matched"
			}
		}

		if intent == intentSemantic || intent == intentHybrid {
			kbStart := time.Now()
			evidence, warning := queryKnowledgeBaseEvidence(ctx, cfg, req)
			kbLatencyMS += time.Since(kbStart).Milliseconds()
			if len(evidenceResults(evidence)) == 0 && db != nil {
				dbStart := time.Now()
				if fallback := collectionAssetEvidence(ctx, db, req); len(evidenceResults(fallback)) > 0 {
					evidence = fallback
					if warning != "" {
						warning += "; using structured KB asset catalog fallback"
					}
				}
				dbLatencyMS += time.Since(dbStart).Milliseconds()
			}
			resp.Evidence = evidence
			resp.Route = append(resp.Route, "kb_rag")
			if warning != "" {
				resp.Warnings = append(resp.Warnings, warning)
			}
			resp.Answer["evidence_summary"] = summarizeEvidence(evidence)
			resp.Answer["evidence_count"] = len(evidenceResults(evidence))
			if len(evidenceResults(evidence)) == 0 {
				resp.Answer["evidence_status"] = "no_kb_evidence"
				resp.Answer["evidence_limitation"] = "No Knowledge Base evidence previews were returned for this query."
			} else {
				resp.Answer["evidence_status"] = "matched"
			}
		}

		if len(resp.Route) == 0 {
			resp.Route = []string{"records_sql"}
		}
		if db != nil && shouldLoadCoverage(req, resp) {
			dbStart := time.Now()
			resp.Coverage = loadCoverageSummary(ctx, db, req)
			dbLatencyMS += time.Since(dbStart).Milliseconds()
			resp.QueryPlan.CoverageCheck = resp.Coverage
			resp.Planner["query_plan"] = resp.QueryPlan
		}
		if intent == intentHybrid {
			llmStart := time.Now()
			if summary, fallbackReason := synthesizeWithBoundedLLM(ctx, cfg, req, resp); summary != "" {
				resp.Answer["llm_summary"] = summary
			} else if fallbackReason != "" {
				resp.Answer["llm_fallback_reason"] = fallbackReason
				resp.QueryPlan.FallbackReason = fallbackReason
				resp.Planner["query_plan"] = resp.QueryPlan
			}
			llmLatencyMS = time.Since(llmStart).Milliseconds()
		}
		resp.Answer["guardrail"] = "No raw full tables were sent to an LLM. Response contains only computed aggregates, selected source previews, and citations."
		resp.Telemetry = buildQueryTelemetry(requestID, dbLatencyMS, kbLatencyMS, llmLatencyMS, startedAt, resp)
		resp.Enterprise = buildEnterprisePayload(req, resp)
		writeJSON(w, http.StatusOK, resp)
	}
}

func needsClarification(query, template, target string) bool {
	if strings.TrimSpace(target) != "" {
		return false
	}
	q := strings.ToLower(query)
	targetCount := len(extractTargets(query))
	if isComparisonQuery(q) && targetCount < 2 {
		return template == "relationship_network" || template == "source_records" || template == "co_travel_or_co_presence"
	}
	targetSpecific := containsAny(q, []string{
		"this number",
		"that number",
		"these numbers",
		"both numbers",
		"this phone",
		"that phone",
		"these phones",
		"this plate",
		"that plate",
		"these plates",
		"this ip",
		"that ip",
		"these ips",
		"this entity",
		"that entity",
		"these entities",
		"this subscriber",
		"that subscriber",
	})
	if !targetSpecific {
		return false
	}
	switch template {
	case "top_locations", "geospatial_movement", "entity_timeline", "relationship_network", "source_records", "first_seen_last_seen", "subscriber_profile", "imei_imsi_usage", "tower_activity", "anpr_sightings":
		return true
	default:
		return false
	}
}

func clarificationQuestion(query, template string) string {
	if isComparisonQuery(strings.ToLower(query)) {
		return "Which two or more target identifiers should I compare?"
	}
	switch template {
	case "top_locations", "geospatial_movement", "tower_activity":
		return "Which number, plate, IP, IMSI/IMEI, or cell-site ID should I analyze for location activity?"
	case "relationship_network":
		return "Which target entity should I use to build the relationship network?"
	case "entity_timeline", "first_seen_last_seen":
		return "Which entity should I build the timeline or first/last-seen view for?"
	case "source_records":
		return "Which identifier or source file should I retrieve auditable source rows for?"
	case "anpr_sightings":
		return "Which plate number, camera, or location should I search ANPR sightings for?"
	default:
		return "Which target identifier should I analyze?"
	}
}

func supportedQueryTemplates() []queryTemplate {
	return []queryTemplate{
		{"collection_overview", "Collection-level ingest, KB asset, and record-family summary.", []string{"all"}, "records"},
		{"frequent_contacts", "CDR frequent contacts matrix with incoming/outgoing counts and first/last contact.", []string{"cdr"}, "records"},
		{"call_type_breakdown", "CDR event counts by call type and direction.", []string{"cdr"}, "records"},
		{"temporal_activity", "Hourly baselines, nocturnal activity, non-zero duration statistics, and shortest/longest audited duration rows.", []string{"cdr"}, "records"},
		{"top_locations", "Most frequent CDR locations and cells.", []string{"cdr"}, "records"},
		{"geospatial_movement", "Chronological CDR movement and primary off-peak base location.", []string{"cdr"}, "records"},
		{"anpr_sightings", "ANPR sightings by plate, camera, and location.", []string{"anpr"}, "records"},
		{"entity_activity", "Cross-record entity observation summary across CDR, ANPR, IPDR, and generic records.", []string{"cdr", "anpr", "ipdr", "generic"}, "records"},
		{"relationship_network", "Entities observed in the same source rows as a target entity.", []string{"cdr", "anpr", "ipdr", "generic"}, "records"},
		{"entity_timeline", "Chronological evidence timeline across CDR and generic record families.", []string{"cdr", "anpr", "ipdr", "generic"}, "records"},
		{"source_records", "Small capped set of source rows for audit and manual review.", []string{"cdr", "anpr", "ipdr", "generic"}, "records"},
		{"canonical_records", "Parameterized source-of-truth query over forensic.records with JSONB filters, provenance, paging, totals, and sort controls.", []string{"all"}, "records"},
		{"schema_profile", "Detected headers, normalized schemas, and structured/RAG asset status.", []string{"all"}, "records"},
		{"data_quality", "Ingest quality, duplicate counts, rejected rows, and parser error samples.", []string{"all"}, "records"},
		{"evidence", "Knowledge Base evidence search with raw-entry fallback.", []string{"all"}, "kb"},
		{"shortest_call", "Shortest non-zero CDR call duration with source row provenance.", []string{"cdr"}, "records"},
		{"longest_call", "Longest CDR call duration with source row provenance.", []string{"cdr"}, "records"},
		{"duration_extremes", "Shortest, longest, and aggregate non-zero CDR duration statistics.", []string{"cdr"}, "records"},
		{"first_seen_last_seen", "First and last observation per entity across indexed records.", []string{"cdr", "anpr", "ipdr", "generic"}, "records"},
		{"activity_by_day", "Daily activity counts by record family.", []string{"cdr", "anpr", "ipdr", "generic"}, "records"},
		{"activity_by_hour", "Hourly CDR activity counts.", []string{"cdr"}, "records"},
		{"night_activity", "Nocturnal CDR activity and duration outliers.", []string{"cdr"}, "records"},
		{"repeated_location_visits", "Repeated location and cell-site visits.", []string{"cdr"}, "records"},
		{"co_travel_or_co_presence", "Entities co-present in the same source rows as a target.", []string{"cdr", "anpr", "ipdr", "generic"}, "records"},
		{"subscriber_profile", "Subscriber, IMSI, IMEI, and source-row profile for a target.", []string{"subscriber", "cdr", "generic"}, "records"},
		{"imei_imsi_usage", "IMEI/IMSI usage activity for a target.", []string{"cdr"}, "records"},
		{"tower_activity", "Tower, cell-site, and location activity for a target.", []string{"cdr", "tower_location"}, "records"},
		{"suspicious_patterns", "Rule-based anomaly summary covering night activity, duration extremes, and quality warnings.", []string{"all"}, "records"},
		{"anomaly_summary", "Rule-based anomaly summary covering night activity, duration extremes, and quality warnings.", []string{"all"}, "records"},
		{"cross_dataset_entity_summary", "Cross-dataset entity observation summary.", []string{"all"}, "records"},
		{"source_file_audit", "Ingested source files, KB asset state, and record-family counts.", []string{"all"}, "records"},
		{"duplicate_upload_audit", "Duplicate upload counts and ingest status.", []string{"all"}, "records"},
		{"case_readiness", "Operational readiness score across ingest completion, duplicates, rejected rows, record families, and KB asset coverage.", []string{"all"}, "records"},
		{"evidence_package_summary", "Collection overview, source files, data quality, and KB evidence route.", []string{"all"}, "hybrid"},
		{"executive_case_brief", "Collection-level executive brief inputs from deterministic analytics.", []string{"all"}, "hybrid"},
		{"court_ready_source_summary", "Auditable source rows, source files, and data-quality limitations.", []string{"all"}, "records"},
		{"limitations_and_data_quality", "Known limitations, rejected rows, duplicate counts, and parser errors.", []string{"all"}, "records"},
	}
}

type runtimePlan struct {
	Intent     queryIntent
	Template   string
	Target     string
	Targets    []string
	TargetType string
	FieldHints []string
	DateFrom   string
	DateTo     string
	Source     string
	Confidence float64
	Reason     string
	QueryPlan  QueryPlan
}

func planRuntimeQuery(req hybridQueryRequest) runtimePlan {
	targets := extractTargets(req.Query)
	target := req.Target
	if target == "" {
		target = firstTarget(targets)
	} else {
		target = normalizeExtractedTarget(target)
	}
	if target != "" && !containsString(targets, target) {
		targets = append([]string{target}, targets...)
	}
	dateFrom, dateTo := extractDateRange(req.Query)
	if req.DateFrom != "" {
		dateFrom = req.DateFrom
	}
	if req.DateTo != "" {
		dateTo = req.DateTo
	}
	template := chooseTemplate(req.Query, req.Template)
	intent := classifyIntent(req.Query, req.Template)
	targetType := classifyTargetType(target)
	fieldHints := extractFieldHints(req.Query)
	if (dateFrom != "" || dateTo != "") && !containsString(fieldHints, "time") {
		fieldHints = append(fieldHints, "time")
	}
	targets = nonNilStrings(targets)
	fieldHints = nonNilStrings(fieldHints)
	if req.Template != "" {
		plan := runtimePlan{Intent: intent, Template: template, Target: target, Targets: targets, TargetType: targetType, FieldHints: fieldHints, DateFrom: dateFrom, DateTo: dateTo, Source: "explicit_template", Confidence: 1, Reason: "template was provided by caller"}
		plan.QueryPlan = plan.toQueryPlan()
		return plan
	}
	confidence := 0.72
	reason := "rule-based runtime query planner"
	if target != "" {
		confidence += 0.12
		reason += "; target extracted"
	}
	if dateFrom != "" || dateTo != "" {
		confidence += 0.06
		reason += "; date range extracted"
	}
	if len(targets) > 1 {
		confidence += 0.04
		reason += "; multiple targets extracted"
	}
	if len(fieldHints) > 0 {
		confidence += 0.02
		reason += "; field hints extracted"
	}
	if intent == intentHybrid {
		confidence += 0.08
	}
	if confidence > 0.95 {
		confidence = 0.95
	}
	plan := runtimePlan{Intent: intent, Template: template, Target: target, Targets: targets, TargetType: targetType, FieldHints: fieldHints, DateFrom: dateFrom, DateTo: dateTo, Source: "runtime_query", Confidence: confidence, Reason: reason}
	plan.QueryPlan = plan.toQueryPlan()
	return plan
}

func (p runtimePlan) toQueryPlan() QueryPlan {
	strategy := "sql_deterministic"
	if p.Intent == intentHybrid {
		strategy = "hybrid_llm"
	}
	filters := map[string]any{
		"target_type": p.TargetType,
		"field_hints": p.FieldHints,
	}
	if p.Target != "" {
		filters["target"] = p.Target
	}
	if p.DateFrom != "" || p.DateTo != "" {
		filters["date_bounds"] = TimeRange{From: p.DateFrom, To: p.DateTo}
	}
	return QueryPlan{
		InterpretedIntent: string(p.Intent),
		SelectedTemplate:  p.Template,
		TargetIdentifiers: nonNilStrings(p.Targets),
		DateBounds:        TimeRange{From: p.DateFrom, To: p.DateTo},
		AppliedFilters:    filters,
		Confidence:        p.Confidence,
		ExecutionStrategy: strategy,
	}
}

func classifyIntent(query, explicitTemplate string) queryIntent {
	template := normalize(explicitTemplate)
	if template == "evidence" || template == "semantic" || template == "policy" {
		return intentSemantic
	}
	if template == "evidence_package_summary" || template == "executive_case_brief" {
		return intentHybrid
	}
	if template != "" {
		return intentRecords
	}
	q := strings.ToLower(query)
	plannedTemplate := chooseTemplate(query, "")
	if plannedTemplate == "evidence_package_summary" || plannedTemplate == "executive_case_brief" {
		return intentHybrid
	}
	if plannedTemplate == "source_records" && containsAny(q, []string{"summarize", "explain", "evidence", "cite", "why", "report", "brief", "context"}) {
		return intentHybrid
	}
	if plannedTemplate == "source_records" || plannedTemplate == "schema_profile" || plannedTemplate == "data_quality" || plannedTemplate == "case_readiness" {
		return intentRecords
	}
	recordsKeywords := []string{
		"count", "total", "most", "frequent", "longest", "shortest", "duration", "timeline",
		"hourly", "nocturnal", "anomaly", "movement", "location", "base", "contact",
		"ratio", "incoming", "outgoing", "plate", "camera", "entity",
		"entities", "related", "overview", "status", "rows", "duplicates", "call", "type", "gprs",
		"sms", "volte", "sighting", "sightings", "seen", "where", "schema", "headers",
		"quality", "errors", "rejected", "timeline", "chronology", "network", "link", "links",
		"relationship", "relationships", "raw", "sample", "samples", "source row",
		"readiness", "ready", "court ready", "production ready", "trust", "reliable", "coverage",
		"compare", "between", "limitations", "missing", "investigate", "findings", "important",
		"canonical", "forensic.records", "record type", "raw payload", "jsonb", "batch id", "field exists",
	}
	semanticKeywords := []string{
		"policy", "explain", "summarize", "source", "evidence", "cite", "why", "report",
		"brief", "context", "document", "case note",
	}
	hasRecords := containsAny(q, recordsKeywords)
	hasSemantic := containsAny(q, semanticKeywords)
	switch {
	case hasRecords && hasSemantic:
		return intentHybrid
	case hasSemantic:
		return intentSemantic
	default:
		return intentRecords
	}
}

func chooseTemplate(query, explicitTemplate string) string {
	if normalized := normalize(explicitTemplate); normalized != "" {
		return canonicalTemplate(normalized)
	}
	q := strings.ToLower(query)
	target := extractTarget(query)
	dateFrom, _ := extractDateRange(query)
	switch {
	case isReportRequest(q) || containsAny(q, []string{"executive brief", "case brief", "case summary", "summarize this case"}):
		return "executive_case_brief"
	case containsCanonicalQueryHint(q):
		return "canonical_records"
	case dateFrom != "" && containsAny(q, []string{"what happened", "happened", "events", "timeline", "activity on", "on this date", "on that date"}):
		return "entity_timeline"
	case containsAny(q, []string{"court ready", "court-ready"}):
		return "court_ready_source_summary"
	case containsAny(q, []string{"which files", "source file", "source files", "file audit", "ingested files", "files ingested"}):
		return "source_file_audit"
	case containsAny(q, []string{"duplicate upload", "duplicate uploads", "duplicate file", "duplicate files"}):
		return "duplicate_upload_audit"
	case containsAny(q, []string{"limitations", "known limitations", "missing data", "data gaps", "parser limitations"}):
		return "limitations_and_data_quality"
	case containsAny(q, []string{"case readiness", "readiness", "ready for court", "ready for production", "production ready", "can we trust", "reliable enough", "evidence health", "case health"}):
		return "case_readiness"
	case containsAny(q, []string{"evidence package"}):
		return "evidence_package_summary"
	case containsAny(q, []string{"schema", "headers", "columns", "fields", "adapter", "detected"}):
		return "schema_profile"
	case isComparisonQuery(q) || containsAny(q, []string{"relationship", "relationships", "network", "link", "links", "connection", "connections"}):
		return "relationship_network"
	case containsAny(q, []string{"timeline", "chronology", "sequence", "events", "history"}):
		return "entity_timeline"
	case containsAny(q, []string{"raw row", "raw rows", "source row", "source rows", "sample rows", "samples", "show records", "source records"}):
		return "source_records"
	case containsAny(q, []string{"collection status", "collection overview", "ingest status"}):
		return "collection_overview"
	case containsAny(q, []string{"quality", "error", "errors", "rejected", "duplicate", "duplicates", "parser", "failed"}):
		return "data_quality"
	case containsAny(q, []string{"overview", "status", "ingest", "batch", "asset", "assets", "rows"}):
		return "collection_overview"
	case containsAny(q, []string{"call type", "call types", "gprs", "sms", "volte", "data", "breakdown"}):
		return "call_type_breakdown"
	case containsAny(q, []string{"contact", "contacts", "caller", "callers", "called", "dialed", "dialled", "communicated", "communication"}):
		return "frequent_contacts"
	case containsAny(q, []string{"shortest call"}):
		return "shortest_call"
	case containsAny(q, []string{"longest call"}):
		return "longest_call"
	case containsAny(q, []string{"first seen", "last seen"}):
		return "first_seen_last_seen"
	case containsAny(q, []string{"suspicious", "anomaly", "anomalies", "investigate next", "what should i investigate", "key findings", "important findings"}):
		return "suspicious_patterns"
	case containsAny(q, []string{"activity by day", "daily activity"}):
		return "activity_by_day"
	case containsAny(q, []string{"activity by hour", "hourly activity"}):
		return "activity_by_hour"
	case containsAny(q, []string{"night activity", "nocturnal"}):
		return "night_activity"
	case containsAny(q, []string{"hourly", "nocturnal", "duration", "longest", "shortest", "temporal", "night"}):
		return "duration_extremes"
	case containsAny(q, []string{"subscriber"}):
		return "subscriber_profile"
	case containsAny(q, []string{"imei", "imsi"}):
		return "imei_imsi_usage"
	case containsAny(q, []string{"entity", "entities", "related", "ip", "imei", "imsi"}):
		return "entity_activity"
	case containsAny(q, []string{"sighting", "sightings", "plate", "camera"}) || (containsAny(q, []string{"seen"}) && !isNumericTarget(target)):
		return "anpr_sightings"
	case containsAny(q, []string{"tower activity", "tower", "cell site", "cell sites"}):
		return "tower_activity"
	case containsAny(q, []string{"top location", "top locations", "common location", "common locations", "cell", "cells", "where", "visited", "tower", "site", "seen"}):
		return "top_locations"
	case containsAny(q, []string{"movement", "geo", "location", "base", "lat", "long", "tower"}):
		return "geospatial_movement"
	case containsAny(q, []string{"policy", "source", "evidence", "document", "case note"}):
		return "evidence"
	default:
		return "frequent_contacts"
	}
}

func isReportRequest(normalized string) bool {
	return (containsAny(normalized, []string{"generate", "create", "write", "produce"}) &&
		containsAny(normalized, []string{"report", "brief", "briefing", "summary"})) ||
		containsAny(normalized, []string{"forensic intelligence report", "intelligence report", "case report"})
}

func containsCanonicalQueryHint(q string) bool {
	for _, pattern := range []string{
		"canonical records",
		"canonical query",
		"canonical sql",
		"forensic.records",
		"raw_payload",
		"record_type",
		"batch_id",
	} {
		if strings.Contains(q, pattern) {
			return true
		}
	}
	if looksLikeCanonicalFilterQuery(q) {
		return true
	}
	return containsAny(q, []string{
		"canonical records",
		"canonical query",
		"canonical sql",
		"raw payload",
		"jsonb",
		"record type",
		"batch id",
		"field exists",
		"field not exists",
	})
}

func looksLikeCanonicalFilterQuery(q string) bool {
	if !containsAny(q, []string{"records", "record", "rows", "row", "cdr", "anpr", "ipdr", "subscriber", "tower", "transaction", "access log", "generic"}) {
		return false
	}
	if containsAny(q, []string{"field exists", "field not exists", "exists", "missing", "source file", "batch id"}) {
		return true
	}
	return regexp.MustCompile(`(?i)\bwhere\b.+(?:\b(?:is|is not|equals?|equal to|not equal|contains|like|in|greater than|less than|at least|at most)\b|>=|<=|>|<|!=|<>)(?:\s|$)`).MatchString(q) ||
		regexp.MustCompile(`(?i)\b(?:raw payload|raw_payload|attribute|property|field)\b.+(?:\b(?:is|is not|equals?|equal to|not equal|contains|like|in|greater than|less than|at least|at most|exists|missing)\b|>=|<=|>|<|!=|<>)(?:\s|$)`).MatchString(q)
}

func canonicalTemplate(template string) string {
	switch normalize(template) {
	case "policy", "semantic", "kb":
		return "evidence"
	case "canonical_sql_query", "canonical_query", "records_query", "records_sql_query", "dynamic_records":
		return "canonical_records"
	case "files_ingested", "ingested_files":
		return "source_file_audit"
	case "duplicate_uploads":
		return "duplicate_upload_audit"
	default:
		return normalize(template)
	}
}

func normalizeCollectionID(value string) string {
	collectionID := strings.TrimSpace(value)
	switch strings.ToLower(strings.ReplaceAll(collectionID, "-", "_")) {
	case "forensic_records_analyst", "forensicrecordsanalyst":
		return "records-demo"
	default:
		return collectionID
	}
}

func isNumericTarget(target string) bool {
	if target == "" {
		return false
	}
	return regexp.MustCompile(`^\d{10,15}$`).MatchString(target)
}

func runAnalyticalTemplate(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, template string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	switch template {
	case "collection_overview":
		return collectionOverview(ctx, db, req)
	case "frequent_contacts":
		return frequentContacts(ctx, db, req)
	case "call_type_breakdown":
		return callTypeBreakdown(ctx, db, req)
	case "temporal_activity":
		return temporalActivity(ctx, db, req)
	case "top_locations":
		return topLocations(ctx, db, req)
	case "geospatial_movement":
		return geospatialMovement(ctx, db, req)
	case "anpr_sightings":
		return anprSightings(ctx, db, req)
	case "relationship_network":
		return relationshipNetwork(ctx, db, req)
	case "entity_timeline":
		return entityTimeline(ctx, db, req)
	case "source_records":
		return sourceRecords(ctx, db, req)
	case "canonical_records":
		return canonicalRecords(ctx, db, req)
	case "schema_profile":
		return schemaProfile(ctx, db, req)
	case "data_quality":
		return dataQuality(ctx, db, req)
	case "case_readiness":
		return caseReadiness(ctx, db, req)
	case "shortest_call":
		return durationExtremes(ctx, db, req, "shortest")
	case "longest_call":
		return durationExtremes(ctx, db, req, "longest")
	case "duration_extremes":
		return durationExtremes(ctx, db, req, "")
	case "first_seen_last_seen":
		return firstSeenLastSeen(ctx, db, req)
	case "activity_by_day":
		return activityByDay(ctx, db, req)
	case "activity_by_hour", "night_activity":
		return temporalActivity(ctx, db, req)
	case "repeated_location_visits", "tower_activity":
		return topLocations(ctx, db, req)
	case "co_travel_or_co_presence":
		return relationshipNetwork(ctx, db, req)
	case "subscriber_profile", "imei_imsi_usage", "cross_dataset_entity_summary":
		return entityActivity(ctx, db, req)
	case "source_file_audit", "duplicate_upload_audit", "evidence_package_summary", "executive_case_brief":
		return collectionOverview(ctx, db, req)
	case "court_ready_source_summary":
		return sourceRecords(ctx, db, req)
	case "suspicious_patterns", "anomaly_summary":
		return suspiciousPatterns(ctx, db, req)
	case "limitations_and_data_quality":
		return dataQuality(ctx, db, req)
	case "entity_activity", "evidence":
		return entityActivity(ctx, db, req)
	default:
		return nil, fmt.Errorf("unsupported query template %q", template)
	}
}

func collectionOverview(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	jobs, err := queryRows(ctx, db, req.TenantID, `
SELECT source_file, record_type::text AS record_type, status::text AS status,
       total_rows, accepted_rows, duplicate_rows, rejected_rows, queued_at, completed_at
FROM forensic.records_ingest_jobs
WHERE tenant_id = $1 AND collection_id = $2
ORDER BY queued_at DESC
LIMIT $3`, req.TenantID, req.CollectionID, req.Limit)
	if err != nil {
		return nil, err
	}
	assets, err := queryRows(ctx, db, req.TenantID, `
SELECT source_file, detected_record_type::text AS detected_record_type,
       structured_status, rag_status, source_entry, size_bytes
FROM forensic.kb_collection_assets
WHERE tenant_id = $1 AND collection_id = $2
ORDER BY created_at DESC
LIMIT $3`, req.TenantID, req.CollectionID, req.Limit)
	if err != nil {
		return nil, err
	}
	families, err := queryRows(ctx, db, req.TenantID, `
SELECT record_type::text AS record_type, count(*) AS batch_count,
       sum(total_rows) AS total_rows, sum(inserted_rows) AS inserted_rows,
       sum(duplicate_rows) AS duplicate_rows, sum(rejected_rows) AS rejected_rows
FROM forensic.kb_active_metadata
WHERE tenant_id = $1 AND collection_id = $2
GROUP BY record_type
ORDER BY total_rows DESC`, req.TenantID, req.CollectionID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"jobs": jobs, "assets": assets, "record_families": families}, nil
}

func frequentContacts(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	rows, err := queryRows(ctx, db, req.TenantID, `
SELECT dialed_number, total_interactions, incoming_count, outgoing_count, first_contact, last_contact,
       CASE WHEN outgoing_count = 0 THEN NULL ELSE round(incoming_count::numeric / outgoing_count::numeric, 4) END AS incoming_outgoing_ratio
FROM forensic.cdr_frequent_contacts
WHERE tenant_id = $1 AND collection_id = $2
  AND (
    $3 = ''
    OR dialed_number = $3
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(dialed_number, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
  )
ORDER BY total_interactions DESC, last_contact DESC
LIMIT $4`, req.TenantID, req.CollectionID, req.Target, req.Limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{"frequent_contacts": rows, "row_count": len(rows)}, nil
}

func callTypeBreakdown(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	rows, err := queryRows(ctx, db, req.TenantID, `
SELECT call_type, direction, count(*) AS event_count,
       min(call_start_ts) AS first_seen, max(call_start_ts) AS last_seen
FROM forensic.cdr_records
WHERE tenant_id = $1 AND collection_id = $2
  AND (
    $3 = ''
    OR msisdn = $3 OR call_org_num = $3 OR call_dialed_num = $3 OR call_type ILIKE $3
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(msisdn, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_org_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_dialed_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
  )
GROUP BY call_type, direction
ORDER BY event_count DESC, call_type, direction
LIMIT $4`, req.TenantID, req.CollectionID, req.Target, req.Limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{"call_type_breakdown": rows, "row_count": len(rows)}, nil
}

func temporalActivity(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	hourly, err := queryRows(ctx, db, req.TenantID, `
SELECT extract(hour from call_start_ts)::int AS hour_of_day, count(*) AS event_count
FROM forensic.cdr_records
WHERE tenant_id = $1 AND collection_id = $2
  AND (
    $3 = ''
    OR msisdn = $3 OR call_org_num = $3 OR call_dialed_num = $3
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(msisdn, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_org_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_dialed_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
  )
GROUP BY hour_of_day
ORDER BY hour_of_day`, req.TenantID, req.CollectionID, req.Target)
	if err != nil {
		return nil, err
	}
	nocturnal, err := queryRows(ctx, db, req.TenantID, `
SELECT count(*) AS nocturnal_events
FROM forensic.cdr_records
WHERE tenant_id = $1 AND collection_id = $2
  AND extract(hour from call_start_ts)::int BETWEEN 1 AND 4
  AND (
    $3 = ''
    OR msisdn = $3 OR call_org_num = $3 OR call_dialed_num = $3
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(msisdn, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_org_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_dialed_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
  )`, req.TenantID, req.CollectionID, req.Target)
	if err != nil {
		return nil, err
	}
	durations, err := queryRows(ctx, db, req.TenantID, `
SELECT count(*) FILTER (WHERE duration_seconds > 0) AS nonzero_duration_events,
       min(duration_seconds) FILTER (WHERE duration_seconds > 0) AS shortest_nonzero_duration,
       max(duration_seconds) FILTER (WHERE duration_seconds > 0) AS longest_duration,
       round(avg(duration_seconds) FILTER (WHERE duration_seconds > 0), 2) AS average_nonzero_duration
FROM forensic.cdr_records
WHERE tenant_id = $1 AND collection_id = $2
  AND (
    $3 = ''
    OR msisdn = $3 OR call_org_num = $3 OR call_dialed_num = $3
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(msisdn, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_org_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_dialed_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
  )`, req.TenantID, req.CollectionID, req.Target)
	if err != nil {
		return nil, err
	}
	extremes, err := queryRows(ctx, db, req.TenantID, `
WITH filtered AS (
	SELECT duration_seconds, call_start_ts, msisdn, call_org_num, call_dialed_num,
	       call_type, direction, location, source_file, row_number
	FROM forensic.cdr_records
	WHERE tenant_id = $1 AND collection_id = $2
	  AND duration_seconds > 0
	  AND (
	    $3 = ''
	    OR msisdn = $3 OR call_org_num = $3 OR call_dialed_num = $3
	    OR (
	      length(regexp_replace($3, '\D', '', 'g')) >= 8
	      AND regexp_replace(coalesce(msisdn, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
	    )
	    OR (
	      length(regexp_replace($3, '\D', '', 'g')) >= 8
	      AND regexp_replace(coalesce(call_org_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
	    )
	    OR (
	      length(regexp_replace($3, '\D', '', 'g')) >= 8
	      AND regexp_replace(coalesce(call_dialed_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
	    )
	  )
),
shortest AS (
	SELECT * FROM filtered ORDER BY duration_seconds ASC, call_start_ts ASC LIMIT 1
),
longest AS (
	SELECT * FROM filtered ORDER BY duration_seconds DESC, call_start_ts ASC LIMIT 1
)
SELECT 'shortest_nonzero_call'::text AS metric, duration_seconds, call_start_ts AS observed_at,
       msisdn, call_org_num, call_dialed_num, call_type, direction, location, source_file, row_number
FROM shortest
UNION ALL
SELECT 'longest_call'::text AS metric, duration_seconds, call_start_ts AS observed_at,
       msisdn, call_org_num, call_dialed_num, call_type, direction, location, source_file, row_number
FROM longest`, req.TenantID, req.CollectionID, req.Target)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"hourly_activity":   hourly,
		"nocturnal":         firstRow(nocturnal),
		"duration_stats":    firstRow(durations),
		"duration_extremes": extremes,
	}, nil
}

func durationExtremes(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, mode string) (map[string]any, error) {
	result, err := temporalActivity(ctx, db, req)
	if err != nil {
		return nil, err
	}
	rows, _ := result["duration_extremes"].([]map[string]any)
	if mode != "" && len(rows) > 0 {
		filtered := make([]map[string]any, 0, 1)
		for _, row := range rows {
			metric := strings.ToLower(fmt.Sprint(row["metric"]))
			if strings.Contains(metric, mode) {
				filtered = append(filtered, row)
			}
		}
		result["duration_extremes"] = filtered
		result["row_count"] = len(filtered)
		return result, nil
	}
	result["row_count"] = len(rows)
	return result, nil
}

func firstSeenLastSeen(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	rows, err := queryRows(ctx, db, req.TenantID, `
SELECT entity_type, entity_value, observation_count, first_seen, last_seen, record_types
FROM forensic.entity_activity_summary
WHERE tenant_id = $1 AND collection_id = $2
  AND ($3 = '' OR entity_value ILIKE '%' || $3 || '%')
ORDER BY last_seen DESC NULLS LAST, observation_count DESC
LIMIT $4`, req.TenantID, req.CollectionID, req.Target, req.Limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{"first_seen_last_seen": rows, "target": req.Target, "row_count": len(rows)}, nil
}

func activityByDay(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	rows, err := queryRows(ctx, db, req.TenantID, `
WITH events AS (
  SELECT call_start_ts::date AS activity_date, 'cdr'::text AS record_type
  FROM forensic.cdr_records
  WHERE tenant_id = $1 AND collection_id = $2
    AND (
      $3 = ''
      OR msisdn = $3 OR call_org_num = $3 OR call_dialed_num = $3 OR imei = $3 OR imsi = $3 OR location ILIKE '%' || $3 || '%'
      OR (
        length(regexp_replace($3, '\D', '', 'g')) >= 8
        AND regexp_replace(coalesce(msisdn, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
      )
      OR (
        length(regexp_replace($3, '\D', '', 'g')) >= 8
        AND regexp_replace(coalesce(call_org_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
      )
      OR (
        length(regexp_replace($3, '\D', '', 'g')) >= 8
        AND regexp_replace(coalesce(call_dialed_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
      )
    )
    AND ($4::timestamptz IS NULL OR call_start_ts >= $4::timestamptz)
    AND ($5::timestamptz IS NULL OR call_start_ts < $5::timestamptz)
  UNION ALL
  SELECT observed_at::date AS activity_date, record_type::text AS record_type
  FROM forensic.generic_records
  WHERE tenant_id = $1 AND collection_id = $2
    AND observed_at IS NOT NULL
    AND ($3 = '' OR primary_entity ILIKE '%' || $3 || '%' OR secondary_entity ILIKE '%' || $3 || '%' OR location ILIKE '%' || $3 || '%')
    AND ($4::timestamptz IS NULL OR observed_at >= $4::timestamptz)
    AND ($5::timestamptz IS NULL OR observed_at < $5::timestamptz)
)
SELECT activity_date, record_type, count(*) AS event_count
FROM events
WHERE activity_date IS NOT NULL
GROUP BY activity_date, record_type
ORDER BY activity_date ASC, record_type
LIMIT $6`, req.TenantID, req.CollectionID, req.Target, dateBoundArg(req.DateFrom), dateBoundArg(req.DateTo), req.Limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{"activity_by_day": rows, "target": req.Target, "date_from": req.DateFrom, "date_to": req.DateTo, "row_count": len(rows)}, nil
}

func suspiciousPatterns(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	temporal, err := temporalActivity(ctx, db, req)
	if err != nil {
		return nil, err
	}
	quality, err := dataQuality(ctx, db, req)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"night_activity":      temporal["nocturnal"],
		"duration_extremes":   temporal["duration_extremes"],
		"duration_stats":      temporal["duration_stats"],
		"data_quality":        quality["summary"],
		"recent_quality_jobs": quality["jobs"],
	}, nil
}

func topLocations(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	rows, err := queryRows(ctx, db, req.TenantID, `
SELECT location, cell_site_id, round(avg(latitude)::numeric, 6) AS latitude,
       round(avg(longitude)::numeric, 6) AS longitude, count(*) AS observation_count,
       min(call_start_ts) AS first_seen, max(call_start_ts) AS last_seen
FROM forensic.cdr_records
WHERE tenant_id = $1 AND collection_id = $2
  AND location IS NOT NULL AND location <> ''
  AND (
    $3 = ''
    OR msisdn = $3 OR call_org_num = $3 OR call_dialed_num = $3 OR location ILIKE '%' || $3 || '%'
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(msisdn, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_org_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_dialed_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
  )
GROUP BY location, cell_site_id
ORDER BY observation_count DESC, last_seen DESC
LIMIT $4`, req.TenantID, req.CollectionID, req.Target, req.Limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{"top_locations": rows, "row_count": len(rows)}, nil
}

func geospatialMovement(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	timeline, err := queryRows(ctx, db, req.TenantID, `
SELECT call_start_ts, msisdn, call_org_num, call_dialed_num, cell_site_id, location, latitude, longitude
FROM forensic.cdr_records
WHERE tenant_id = $1 AND collection_id = $2
  AND latitude IS NOT NULL AND longitude IS NOT NULL
  AND (
    $3 = ''
    OR msisdn = $3 OR call_org_num = $3 OR call_dialed_num = $3
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(msisdn, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_org_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_dialed_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
  )
ORDER BY call_start_ts ASC
LIMIT $4`, req.TenantID, req.CollectionID, req.Target, req.Limit)
	if err != nil {
		return nil, err
	}
	base, err := queryRows(ctx, db, req.TenantID, `
SELECT location, cell_site_id, round(avg(latitude)::numeric, 6) AS latitude, round(avg(longitude)::numeric, 6) AS longitude,
       count(*) AS off_peak_observations
FROM forensic.cdr_records
WHERE tenant_id = $1 AND collection_id = $2
  AND latitude IS NOT NULL AND longitude IS NOT NULL
  AND (extract(hour from call_start_ts)::int >= 22 OR extract(hour from call_start_ts)::int <= 6)
  AND (
    $3 = ''
    OR msisdn = $3 OR call_org_num = $3 OR call_dialed_num = $3
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(msisdn, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_org_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_dialed_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
  )
GROUP BY location, cell_site_id
ORDER BY off_peak_observations DESC, location
LIMIT 5`, req.TenantID, req.CollectionID, req.Target)
	if err != nil {
		return nil, err
	}
	return map[string]any{"movement_timeline": timeline, "primary_base_candidates": base}, nil
}

func anprSightings(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	rows, err := queryRows(ctx, db, req.TenantID, `
SELECT observed_at, primary_entity AS plate_number, secondary_entity AS camera_id,
       location, latitude, longitude, source_file, row_number
FROM forensic.generic_records
WHERE tenant_id = $1 AND collection_id = $2
  AND record_type = 'anpr'
  AND ($3 = '' OR primary_entity ILIKE '%' || $3 || '%' OR secondary_entity ILIKE '%' || $3 || '%' OR location ILIKE '%' || $3 || '%')
ORDER BY observed_at ASC NULLS LAST, row_number ASC
LIMIT $4`, req.TenantID, req.CollectionID, req.Target, req.Limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{"anpr_sightings": rows, "row_count": len(rows)}, nil
}

func entityActivity(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	rows, err := queryRows(ctx, db, req.TenantID, `
SELECT entity_type, entity_value, observation_count, record_type_count, first_seen, last_seen, record_types
FROM forensic.entity_activity_summary
WHERE tenant_id = $1 AND collection_id = $2
  AND ($3 = '' OR entity_value ILIKE '%' || $3 || '%')
ORDER BY observation_count DESC, last_seen DESC NULLS LAST
LIMIT $4`, req.TenantID, req.CollectionID, req.Target, req.Limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{"entity_activity": rows, "row_count": len(rows)}, nil
}

func relationshipNetwork(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	if req.Target == "" {
		return entityActivity(ctx, db, req)
	}
	rows, err := queryRows(ctx, db, req.TenantID, `
WITH target_rows AS (
  SELECT DISTINCT tenant_id, collection_id, batch_id, row_hash
  FROM forensic.record_entities
  WHERE tenant_id = $1 AND collection_id = $2
    AND entity_value ILIKE '%' || $3 || '%'
)
SELECT re.entity_type, re.entity_value, re.source_field,
       count(*) AS co_observation_count,
       min(re.observed_at) AS first_seen,
       max(re.observed_at) AS last_seen,
       array_agg(DISTINCT re.record_type::text ORDER BY re.record_type::text) AS record_types,
       array_agg(DISTINCT re.source_file ORDER BY re.source_file) AS source_files
FROM forensic.record_entities re
JOIN target_rows tr
  ON tr.tenant_id = re.tenant_id
 AND tr.collection_id = re.collection_id
 AND tr.batch_id = re.batch_id
 AND tr.row_hash = re.row_hash
WHERE re.tenant_id = $1
  AND re.collection_id = $2
  AND re.entity_value NOT ILIKE '%' || $3 || '%'
GROUP BY re.entity_type, re.entity_value, re.source_field
ORDER BY co_observation_count DESC, last_seen DESC NULLS LAST
LIMIT $4`, req.TenantID, req.CollectionID, req.Target, req.Limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{"relationship_network": rows, "target": req.Target, "row_count": len(rows)}, nil
}

func entityTimeline(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	rows, err := queryRows(ctx, db, req.TenantID, `
SELECT event_time, record_type, primary_entity, secondary_entity, location, event_label, source_file, row_number
FROM (
  SELECT call_start_ts AS event_time, 'cdr'::text AS record_type,
         coalesce(nullif(msisdn, ''), call_org_num) AS primary_entity,
         call_dialed_num AS secondary_entity,
         location,
         concat_ws(' ', direction, call_type) AS event_label,
         source_file,
         row_number
  FROM forensic.cdr_records
  WHERE tenant_id = $1 AND collection_id = $2
    AND (
      $3 = ''
      OR msisdn = $3 OR call_org_num = $3 OR call_dialed_num = $3 OR imei = $3 OR imsi = $3 OR cell_site_id = $3 OR location ILIKE '%' || $3 || '%'
      OR (
        length(regexp_replace($3, '\D', '', 'g')) >= 8
        AND regexp_replace(coalesce(msisdn, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
      )
      OR (
        length(regexp_replace($3, '\D', '', 'g')) >= 8
        AND regexp_replace(coalesce(call_org_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
      )
      OR (
        length(regexp_replace($3, '\D', '', 'g')) >= 8
        AND regexp_replace(coalesce(call_dialed_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
      )
    )
    AND ($4::timestamptz IS NULL OR call_start_ts >= $4::timestamptz)
    AND ($5::timestamptz IS NULL OR call_start_ts < $5::timestamptz)
  UNION ALL
  SELECT observed_at AS event_time, record_type::text AS record_type,
         primary_entity, secondary_entity, location,
         record_type::text AS event_label,
         source_file,
         row_number
  FROM forensic.generic_records
  WHERE tenant_id = $1 AND collection_id = $2
    AND ($3 = '' OR primary_entity ILIKE '%' || $3 || '%' OR secondary_entity ILIKE '%' || $3 || '%' OR location ILIKE '%' || $3 || '%')
    AND ($4::timestamptz IS NULL OR observed_at >= $4::timestamptz)
    AND ($5::timestamptz IS NULL OR observed_at < $5::timestamptz)
) timeline
ORDER BY event_time ASC NULLS LAST, source_file, row_number
LIMIT $6`, req.TenantID, req.CollectionID, req.Target, dateBoundArg(req.DateFrom), dateBoundArg(req.DateTo), req.Limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{"entity_timeline": rows, "target": req.Target, "date_from": req.DateFrom, "date_to": req.DateTo, "row_count": len(rows)}, nil
}

func sourceRecords(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	cdrRows, err := queryRows(ctx, db, req.TenantID, `
SELECT 'cdr'::text AS record_type, source_file, row_number, call_start_ts AS observed_at,
       msisdn, call_org_num, call_dialed_num, imsi, imei, call_type, direction, location
FROM forensic.cdr_records
WHERE tenant_id = $1 AND collection_id = $2
  AND (
    $3 = ''
    OR msisdn = $3 OR call_org_num = $3 OR call_dialed_num = $3 OR imei = $3 OR imsi = $3 OR call_type ILIKE $3 OR location ILIKE '%' || $3 || '%'
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(msisdn, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_org_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
    OR (
      length(regexp_replace($3, '\D', '', 'g')) >= 8
      AND regexp_replace(coalesce(call_dialed_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
    )
  )
  AND ($4::timestamptz IS NULL OR call_start_ts >= $4::timestamptz)
  AND ($5::timestamptz IS NULL OR call_start_ts < $5::timestamptz)
ORDER BY call_start_ts ASC
LIMIT $6`, req.TenantID, req.CollectionID, req.Target, dateBoundArg(req.DateFrom), dateBoundArg(req.DateTo), req.Limit)
	if err != nil {
		return nil, err
	}
	genericRows, err := queryRows(ctx, db, req.TenantID, `
SELECT record_type::text AS record_type, source_file, row_number, observed_at,
       primary_entity, secondary_entity, location
FROM forensic.generic_records
WHERE tenant_id = $1 AND collection_id = $2
  AND ($3 = '' OR primary_entity ILIKE '%' || $3 || '%' OR secondary_entity ILIKE '%' || $3 || '%' OR location ILIKE '%' || $3 || '%')
  AND ($4::timestamptz IS NULL OR observed_at >= $4::timestamptz)
  AND ($5::timestamptz IS NULL OR observed_at < $5::timestamptz)
ORDER BY observed_at ASC NULLS LAST, source_file, row_number
LIMIT $6`, req.TenantID, req.CollectionID, req.Target, dateBoundArg(req.DateFrom), dateBoundArg(req.DateTo), req.Limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{"cdr_source_records": cdrRows, "generic_source_records": genericRows, "target": req.Target, "date_from": req.DateFrom, "date_to": req.DateTo, "row_count": len(cdrRows) + len(genericRows)}, nil
}

type canonicalRecordsQuery struct {
	WhereSQL string
	Args     []any
	OrderBy  string
	Filters  map[string]any
}

type canonicalSQLBuilder struct {
	args    []any
	where   []string
	filters map[string]any
}

func canonicalRecords(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	built, err := buildCanonicalRecordsQuery(req)
	if err != nil {
		return nil, err
	}
	countRows, err := queryRows(ctx, db, req.TenantID, `
SELECT count(*) AS total_count
FROM forensic.records
`+built.WhereSQL, built.Args...)
	if err != nil {
		return nil, err
	}
	total := int64(0)
	if len(countRows) > 0 {
		total = int64FromAny(countRows[0]["total_count"])
	}
	rowArgs := append([]any{}, built.Args...)
	limitParam := fmt.Sprintf("$%d", len(rowArgs)+1)
	rowArgs = append(rowArgs, req.Limit)
	offsetParam := fmt.Sprintf("$%d", len(rowArgs)+1)
	rowArgs = append(rowArgs, req.Offset)
	rows, err := queryRows(ctx, db, req.TenantID, `
SELECT record_id::text AS record_id,
       tenant_id,
       collection_id,
       file_id,
       batch_id::text AS batch_id,
       record_type::text AS record_type,
       timestamp,
       primary_target,
       secondary_target,
       source_file,
       row_number,
       row_hash,
       raw_payload,
       metadata,
       ingested_at,
       'forensic.records'::text AS source_table
FROM forensic.records
`+built.WhereSQL+`
ORDER BY `+built.OrderBy+`
LIMIT `+limitParam+` OFFSET `+offsetParam, rowArgs...)
	if err != nil {
		return nil, err
	}
	provenance := canonicalProvenance(rows, req.CollectionID)
	return map[string]any{
		"canonical_records": rows,
		"row_count":         len(rows),
		"total_count":       total,
		"limit":             req.Limit,
		"offset":            req.Offset,
		"query_route":       "forensic.records",
		"canonical_table":   "forensic.records",
		"sort":              map[string]any{"by": canonicalSortColumn(req.SortBy), "direction": canonicalSortDirection(req.SortDirection), "order_by": built.OrderBy},
		"filters":           built.Filters,
		"provenance":        provenance,
	}, nil
}

func buildCanonicalRecordsQuery(req hybridQueryRequest) (canonicalRecordsQuery, error) {
	builder := canonicalSQLBuilder{filters: map[string]any{}}
	builder.addPredicate("tenant_id = " + builder.addArg(req.TenantID))
	builder.addPredicate("collection_id = " + builder.addArg(req.CollectionID))
	builder.filters["tenant_id"] = req.TenantID
	builder.filters["collection_id"] = req.CollectionID
	if req.RecordType != "" {
		builder.addPredicate("record_type = " + builder.addArg(req.RecordType))
		builder.filters["record_type"] = req.RecordType
	}
	if req.SourceFile != "" {
		builder.addPredicate("source_file = " + builder.addArg(req.SourceFile))
		builder.filters["source_file"] = req.SourceFile
	}
	if req.BatchID != "" {
		builder.addPredicate("batch_id::text = " + builder.addArg(req.BatchID))
		builder.filters["batch_id"] = req.BatchID
	}
	if req.DateFrom != "" {
		builder.addPredicate("timestamp >= " + builder.addArg(req.DateFrom) + "::timestamptz")
		builder.filters["date_from"] = req.DateFrom
	}
	if req.DateTo != "" {
		builder.addPredicate("timestamp < " + builder.addArg(req.DateTo) + "::timestamptz")
		builder.filters["date_to"] = req.DateTo
	}
	if targets := canonicalTargetSet(req.Target, req.Targets, nil); len(targets) > 0 {
		targetPredicates := make([]string, 0, len(targets))
		for _, target := range targets {
			targetPredicates = append(targetPredicates, builder.targetPredicate(target))
		}
		builder.addPredicate("(" + strings.Join(targetPredicates, " OR ") + ")")
		builder.filters["targets"] = targets
	}
	if err := builder.addPayloadFilters(req.RawPayloadFilters); err != nil {
		return canonicalRecordsQuery{}, err
	}
	if err := builder.addFieldFilters(req.FieldFilters, req.FieldExists, req.FieldNotExists); err != nil {
		return canonicalRecordsQuery{}, err
	}
	return canonicalRecordsQuery{
		WhereSQL: "WHERE " + strings.Join(builder.where, "\n  AND "),
		Args:     builder.args,
		OrderBy:  canonicalOrderBy(req),
		Filters:  builder.filters,
	}, nil
}

func (b *canonicalSQLBuilder) addArg(value any) string {
	b.args = append(b.args, value)
	return fmt.Sprintf("$%d", len(b.args))
}

func (b *canonicalSQLBuilder) addPredicate(predicate string) {
	b.where = append(b.where, predicate)
}

func (b *canonicalSQLBuilder) targetPredicate(target string) string {
	targetParam := b.addArg(target)
	clauses := []string{
		"primary_target = " + targetParam,
		"secondary_target = " + targetParam,
		"primary_target ILIKE '%' || " + targetParam + " || '%'",
		"secondary_target ILIKE '%' || " + targetParam + " || '%'",
	}
	digits := regexp.MustCompile(`\D`).ReplaceAllString(target, "")
	if len(digits) >= 8 {
		digitParam := b.addArg(digits)
		clauses = append(clauses,
			"regexp_replace(coalesce(primary_target, ''), '\\D', '', 'g') = "+digitParam,
			"regexp_replace(coalesce(secondary_target, ''), '\\D', '', 'g') = "+digitParam,
		)
	}
	return "(" + strings.Join(clauses, " OR ") + ")"
}

func (b *canonicalSQLBuilder) addPayloadFilters(filters []CanonicalPayloadFilter) error {
	if len(filters) == 0 {
		return nil
	}
	applied := make([]map[string]any, 0, len(filters))
	for _, filter := range filters {
		field := strings.TrimSpace(filter.Field)
		if field == "" {
			return errors.New("raw_payload filter field is required")
		}
		op := normalize(filter.Op)
		if op == "" {
			op = "eq"
		}
		fieldParam := b.addArg(field)
		fieldValue := canonicalPayloadValueSQL(fieldParam)
		appliedFilter := map[string]any{"field": field, "op": op}
		switch op {
		case "eq", "equals", "equal":
			value := canonicalFilterValue(filter.Value)
			valueParam := b.addArg(value)
			b.addPredicate(fieldValue + " = " + valueParam)
			appliedFilter["value"] = value
		case "ne", "not_eq", "not_equal":
			value := canonicalFilterValue(filter.Value)
			valueParam := b.addArg(value)
			b.addPredicate("(" + fieldValue + " IS NULL OR " + fieldValue + " <> " + valueParam + ")")
			appliedFilter["value"] = value
		case "contains", "ilike":
			value := canonicalFilterValue(filter.Value)
			valueParam := b.addArg(value)
			b.addPredicate(fieldValue + " ILIKE '%' || " + valueParam + " || '%'")
			appliedFilter["value"] = value
		case "in":
			values := canonicalFilterValues(filter.Values)
			if len(values) == 0 && filter.Value != nil {
				values = canonicalFilterValues([]any{filter.Value})
			}
			if len(values) == 0 {
				return fmt.Errorf("raw_payload filter %q requires at least one value", field)
			}
			valuesParam := b.addArg(values)
			b.addPredicate(fieldValue + " = ANY(" + valuesParam + "::text[])")
			appliedFilter["values"] = values
		case "gt", "gte", "lt", "lte":
			value := canonicalFilterValue(filter.Value)
			if !isCanonicalNumericLiteral(value) {
				return fmt.Errorf("raw_payload numeric filter %q requires a numeric value", field)
			}
			valueParam := b.addArg(value)
			comparator := map[string]string{"gt": ">", "gte": ">=", "lt": "<", "lte": "<="}[op]
			numericValue := "CASE WHEN " + fieldValue + " ~ '^-?[0-9]+(\\.[0-9]+)?$' THEN " + fieldValue + "::numeric END"
			b.addPredicate(numericValue + " " + comparator + " " + valueParam + "::numeric")
			appliedFilter["value"] = value
		case "exists":
			b.addPredicate(canonicalPayloadExistsSQL(fieldParam))
		case "not_exists", "missing":
			b.addPredicate("NOT " + canonicalPayloadExistsSQL(fieldParam))
		default:
			return fmt.Errorf("unsupported raw_payload filter op %q", filter.Op)
		}
		applied = append(applied, appliedFilter)
	}
	b.filters["raw_payload_filters"] = applied
	return nil
}

func canonicalPayloadValueSQL(fieldParam string) string {
	return "coalesce(raw_payload ->> " + fieldParam + ", (SELECT payload.v FROM jsonb_each_text(raw_payload) AS payload(k, v) WHERE lower(payload.k) = lower(" + fieldParam + ") LIMIT 1), metadata -> 'normalized_fields' ->> " + fieldParam + ", (SELECT meta.v FROM jsonb_each_text(coalesce(metadata -> 'normalized_fields', '{}'::jsonb)) AS meta(k, v) WHERE lower(meta.k) = lower(" + fieldParam + ") LIMIT 1))"
}

func canonicalPayloadExistsSQL(fieldParam string) string {
	return "(raw_payload ? " + fieldParam + " OR EXISTS (SELECT 1 FROM jsonb_object_keys(raw_payload) AS payload(k) WHERE lower(payload.k) = lower(" + fieldParam + ")) OR metadata -> 'normalized_fields' ? " + fieldParam + " OR EXISTS (SELECT 1 FROM jsonb_object_keys(coalesce(metadata -> 'normalized_fields', '{}'::jsonb)) AS meta(k) WHERE lower(meta.k) = lower(" + fieldParam + ")))"
}

func (b *canonicalSQLBuilder) addFieldFilters(filters []CanonicalFieldFilter, existsFields, notExistsFields []string) error {
	applied := make([]map[string]any, 0, len(filters)+len(existsFields)+len(notExistsFields))
	add := func(field, op string) error {
		field = strings.TrimSpace(field)
		if field == "" {
			return errors.New("field filter field is required")
		}
		op = normalize(op)
		fieldParam := b.addArg(field)
		switch op {
		case "exists", "":
			b.addPredicate(canonicalPayloadExistsSQL(fieldParam))
			applied = append(applied, map[string]any{"field": field, "op": "exists"})
		case "not_exists", "missing":
			b.addPredicate("NOT " + canonicalPayloadExistsSQL(fieldParam))
			applied = append(applied, map[string]any{"field": field, "op": "not_exists"})
		default:
			return fmt.Errorf("unsupported field filter op %q", op)
		}
		return nil
	}
	for _, field := range existsFields {
		if err := add(field, "exists"); err != nil {
			return err
		}
	}
	for _, field := range notExistsFields {
		if err := add(field, "not_exists"); err != nil {
			return err
		}
	}
	for _, filter := range filters {
		if err := add(filter.Field, filter.Op); err != nil {
			return err
		}
	}
	if len(applied) > 0 {
		b.filters["field_filters"] = applied
	}
	return nil
}

func canonicalOrderBy(req hybridQueryRequest) string {
	column := canonicalSortColumn(req.SortBy)
	direction := canonicalSortDirection(req.SortDirection)
	if column == "timestamp" {
		return column + " " + direction + " NULLS LAST, source_file ASC NULLS LAST, row_number ASC NULLS LAST"
	}
	fallbacks := []string{"timestamp DESC NULLS LAST", "source_file ASC NULLS LAST", "row_number ASC NULLS LAST"}
	order := []string{column + " " + direction + " NULLS LAST"}
	for _, fallback := range fallbacks {
		if strings.HasPrefix(fallback, column+" ") {
			continue
		}
		order = append(order, fallback)
	}
	return strings.Join(order, ", ")
}

func canonicalSortColumn(value string) string {
	switch normalize(value) {
	case "record_type", "primary_target", "secondary_target", "source_file", "row_number", "ingested_at":
		return normalize(value)
	case "timestamp", "observed_at", "":
		return "timestamp"
	default:
		return "timestamp"
	}
}

func canonicalSortDirection(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "ASC":
		return "ASC"
	default:
		return "DESC"
	}
}

func canonicalFilterValue(value any) string {
	return strings.TrimSpace(fmt.Sprint(value))
}

func canonicalFilterValues(values []any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		text := canonicalFilterValue(value)
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func canonicalProvenance(rows []map[string]any, collectionID string) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"source":        "records_sql",
			"source_table":  "forensic.records",
			"collection_id": collectionID,
			"record_id":     row["record_id"],
			"record_type":   row["record_type"],
			"source_file":   row["source_file"],
			"row_number":    row["row_number"],
			"batch_id":      row["batch_id"],
			"file_id":       row["file_id"],
			"row_hash":      row["row_hash"],
			"timestamp":     row["timestamp"],
		})
	}
	return out
}

func schemaProfile(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	assets, err := queryRows(ctx, db, req.TenantID, `
SELECT source_file, detected_record_type::text AS detected_record_type,
       requested_record_type::text AS requested_record_type,
       structured_status, rag_status, storage_mode,
       jsonb_array_length(headers) AS header_count,
       nullif(routing_decision->>'routing_reason', '') AS routing_reason,
       nullif(quality_report->>'total_rows', '')::bigint AS total_rows,
       nullif(quality_report->>'inserted_rows', '')::bigint AS inserted_rows,
       nullif(quality_report->>'indexed_entities', '')::bigint AS indexed_entities,
       nullif(quality_report->>'rejected_rows', '')::bigint AS rejected_rows,
       created_at, updated_at
FROM forensic.kb_collection_assets
WHERE tenant_id = $1 AND collection_id = $2
ORDER BY created_at DESC
LIMIT $3`, req.TenantID, req.CollectionID, req.Limit)
	if err != nil {
		return nil, err
	}
	metadata, err := queryRows(ctx, db, req.TenantID, `
SELECT source_file, record_type::text AS record_type, total_rows, inserted_rows, duplicate_rows,
       rejected_rows, min_timestamp, max_timestamp, unique_targets_count,
       unique_originators_count, unique_locations_count,
       (SELECT count(*) FROM jsonb_object_keys(coalesce(normalized_schema, '{}'::jsonb))) AS normalized_field_count
FROM forensic.kb_active_metadata
WHERE tenant_id = $1 AND collection_id = $2
ORDER BY created_at DESC
LIMIT $3`, req.TenantID, req.CollectionID, req.Limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{"assets": assets, "metadata": metadata, "row_count": len(assets) + len(metadata)}, nil
}

func dataQuality(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	jobs, err := queryRows(ctx, db, req.TenantID, `
SELECT source_file, record_type::text AS record_type, status::text AS status,
       total_rows, accepted_rows, duplicate_rows, rejected_rows, error_message,
       queued_at, started_at, completed_at
FROM forensic.records_ingest_jobs
WHERE tenant_id = $1 AND collection_id = $2
ORDER BY queued_at DESC
LIMIT $3`, req.TenantID, req.CollectionID, req.Limit)
	if err != nil {
		return nil, err
	}
	errors, err := queryRows(ctx, db, req.TenantID, `
SELECT coalesce(j.source_file, e.file_id) AS source_file,
       e.row_number, e.error_code, e.error_message, e.created_at
FROM forensic.records_ingest_errors e
LEFT JOIN forensic.records_ingest_jobs j
  ON j.tenant_id = e.tenant_id
 AND j.collection_id = e.collection_id
 AND j.file_id = e.file_id
WHERE e.tenant_id = $1 AND e.collection_id = $2
ORDER BY e.created_at DESC
LIMIT $3`, req.TenantID, req.CollectionID, req.Limit)
	if err != nil {
		return nil, err
	}
	summary, err := queryRows(ctx, db, req.TenantID, `
SELECT coalesce(sum(total_rows), 0) AS total_rows,
       coalesce(sum(accepted_rows), 0) AS accepted_rows,
       coalesce(sum(duplicate_rows), 0) AS duplicate_rows,
       coalesce(sum(rejected_rows), 0) AS rejected_rows,
       count(*) FILTER (WHERE status = 'failed') AS failed_jobs,
       count(*) FILTER (WHERE status = 'completed') AS completed_jobs
FROM forensic.records_ingest_jobs
WHERE tenant_id = $1 AND collection_id = $2`, req.TenantID, req.CollectionID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"summary": firstRow(summary), "jobs": jobs, "errors": errors}, nil
}

func caseReadiness(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	overview, err := collectionOverview(ctx, db, req)
	if err != nil {
		return nil, err
	}
	quality, err := dataQuality(ctx, db, req)
	if err != nil {
		return nil, err
	}
	schema, err := schemaProfile(ctx, db, req)
	if err != nil {
		return nil, err
	}
	readiness := scoreCaseReadiness(overview, quality, schema)
	return map[string]any{
		"readiness":        readiness,
		"readiness_checks": readinessChecks(readiness),
		"quality_summary":  quality["summary"],
		"record_families":  overview["record_families"],
		"assets":           schema["assets"],
		"jobs":             overview["jobs"],
		"row_count":        countResultRows(overview) + countResultRows(quality) + countResultRows(schema),
	}, nil
}

func scoreCaseReadiness(overview, quality, schema map[string]any) map[string]any {
	jobs := typedRows(overview["jobs"])
	families := typedRows(overview["record_families"])
	assets := typedRows(schema["assets"])
	summary, _ := quality["summary"].(map[string]any)

	totalRows := firstPositiveInt(summary, "total_rows", "accepted_rows")
	if totalRows == 0 {
		totalRows = sumRowsInt(families, "total_rows", "inserted_rows")
	}
	acceptedRows := firstPositiveInt(summary, "accepted_rows")
	if acceptedRows == 0 {
		acceptedRows = sumRowsInt(families, "inserted_rows", "total_rows")
	}
	duplicateRows := firstPositiveInt(summary, "duplicate_rows")
	if duplicateRows == 0 {
		duplicateRows = sumRowsInt(families, "duplicate_rows")
	}
	rejectedRows := firstPositiveInt(summary, "rejected_rows")
	if rejectedRows == 0 {
		rejectedRows = sumRowsInt(families, "rejected_rows")
	}
	failedJobs := firstPositiveInt(summary, "failed_jobs")
	completedJobs := firstPositiveInt(summary, "completed_jobs")
	if completedJobs == 0 {
		for _, job := range jobs {
			if strings.EqualFold(fmt.Sprint(job["status"]), "completed") {
				completedJobs++
			}
		}
	}

	score := 100
	var issues []string
	var actions []string
	if len(jobs) == 0 {
		score -= 35
		issues = append(issues, "No ingest jobs were found for this collection.")
		actions = append(actions, "Ingest at least one structured source file before relying on case analytics.")
	}
	if len(families) == 0 || acceptedRows == 0 {
		score -= 30
		issues = append(issues, "No accepted structured record families were found.")
		actions = append(actions, "Check parser routing and rerun ingestion for source files with expected CDR, ANPR, IPDR, subscriber, tower, or transaction records.")
	}
	if failedJobs > 0 {
		score -= minInt(25, int(failedJobs)*10)
		issues = append(issues, fmt.Sprintf("%d ingest job(s) failed.", failedJobs))
		actions = append(actions, "Review failed ingest jobs and parser errors before operational use.")
	}
	if rejectedRows > 0 {
		penalty := 8
		if totalRows > 0 {
			penalty += minInt(22, int((rejectedRows*100)/totalRows))
		}
		score -= penalty
		issues = append(issues, fmt.Sprintf("%d rejected row(s) may reduce completeness.", rejectedRows))
		actions = append(actions, "Repair rejected rows or document why they are out of scope.")
	}
	if duplicateRows > 0 {
		penalty := 4
		if totalRows > 0 {
			penalty += minInt(16, int((duplicateRows*100)/totalRows))
		}
		score -= penalty
		issues = append(issues, fmt.Sprintf("%d duplicate row(s) were detected.", duplicateRows))
		actions = append(actions, "Confirm duplicate rows are expected or deduplicate affected source batches.")
	}
	if len(assets) == 0 {
		score -= 15
		issues = append(issues, "No KB asset catalog entries were found for source traceability.")
		actions = append(actions, "Repair collection assets or mirror source files to the Knowledge Base for citation coverage.")
	} else if countRowsWithStatus(assets, "rag_status", "ready", "indexed", "mirrored") == 0 {
		score -= 10
		issues = append(issues, "No KB assets report ready RAG/source preview status.")
		actions = append(actions, "Reindex or repair KB source previews so reports can cite source evidence.")
	}
	if completedJobs == 0 && len(jobs) > 0 {
		score -= 20
		issues = append(issues, "No completed ingest jobs were found.")
		actions = append(actions, "Wait for queued/running jobs or investigate stalled ingestion workers.")
	}
	if score < 0 {
		score = 0
	}
	level := "ready"
	switch {
	case score < 50:
		level = "blocked"
	case score < 75:
		level = "needs_review"
	case score < 90:
		level = "usable_with_limitations"
	}
	if len(issues) == 0 {
		issues = append(issues, "No blocking ingest, duplicate, rejected-row, or KB asset issues were detected by deterministic checks.")
	}
	if len(actions) == 0 {
		actions = append(actions, "Proceed with target-specific source-row review before external reporting.")
	}
	return map[string]any{
		"score":            score,
		"level":            level,
		"total_rows":       totalRows,
		"accepted_rows":    acceptedRows,
		"duplicate_rows":   duplicateRows,
		"rejected_rows":    rejectedRows,
		"failed_jobs":      failedJobs,
		"completed_jobs":   completedJobs,
		"source_files":     len(jobs),
		"record_families":  len(families),
		"kb_assets":        len(assets),
		"issues":           issues,
		"recommended_next": actions,
	}
}

func readinessChecks(readiness map[string]any) []map[string]any {
	return []map[string]any{
		{"check": "overall", "status": readiness["level"], "score": readiness["score"]},
		{"check": "structured_rows", "status": statusFromCount(valueAsInt(readiness["accepted_rows"])), "count": readiness["accepted_rows"]},
		{"check": "record_families", "status": statusFromCount(valueAsInt(readiness["record_families"])), "count": readiness["record_families"]},
		{"check": "failed_jobs", "status": statusFromZero(valueAsInt(readiness["failed_jobs"])), "count": readiness["failed_jobs"]},
		{"check": "rejected_rows", "status": statusFromZero(valueAsInt(readiness["rejected_rows"])), "count": readiness["rejected_rows"]},
		{"check": "duplicate_rows", "status": statusFromZero(valueAsInt(readiness["duplicate_rows"])), "count": readiness["duplicate_rows"]},
		{"check": "kb_assets", "status": statusFromCount(valueAsInt(readiness["kb_assets"])), "count": readiness["kb_assets"]},
	}
}

func typedRows(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, item := range rows {
			if row, ok := item.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func firstPositiveInt(row map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if value := valueAsInt(row[key]); value > 0 {
			return value
		}
	}
	return 0
}

func countRowsWithStatus(rows []map[string]any, key string, accepted ...string) int {
	allowed := map[string]struct{}{}
	for _, value := range accepted {
		allowed[strings.ToLower(value)] = struct{}{}
	}
	count := 0
	for _, row := range rows {
		status := strings.ToLower(strings.TrimSpace(fmt.Sprint(row[key])))
		if _, ok := allowed[status]; ok {
			count++
		}
	}
	return count
}

func statusFromCount(count int64) string {
	if count > 0 {
		return "ok"
	}
	return "missing"
}

func statusFromZero(count int64) string {
	if count == 0 {
		return "ok"
	}
	return "review"
}

func queryRows(ctx context.Context, db *pgxpool.Pool, tenantID, query string, args ...any) ([]map[string]any, error) {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin read-only query: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, fmt.Errorf("set tenant context: %w", err)
	}
	rows, err := tx.Query(ctx, sqlWithRequestID(ctx, query), args...)
	if err != nil {
		return nil, fmt.Errorf("execute analytical template: %w", err)
	}
	defer rows.Close()
	values, err := rowsToMaps(rows)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit read-only query: %w", err)
	}
	return values, nil
}

func rowsToMaps(rows pgx.Rows) ([]map[string]any, error) {
	fields := rows.FieldDescriptions()
	out := make([]map[string]any, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("scan analytical row: %w", err)
		}
		row := make(map[string]any, len(values))
		for i, value := range values {
			row[string(fields[i].Name)] = normalizeDBValue(value)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate analytical rows: %w", err)
	}
	return out, nil
}

func queryKnowledgeBaseEvidence(ctx context.Context, cfg config, req hybridQueryRequest) (map[string]any, string) {
	if cfg.LocalAIURL == "" {
		return map[string]any{"results": []any{}}, "LOCALAI_KB_URL is not configured; KB evidence lookup skipped"
	}
	searchResults, err := kbSearch(ctx, cfg, req)
	if err == nil && len(searchResults) > 0 {
		return map[string]any{"mode": "vector_search", "results": searchResults}, ""
	}
	entries, previewErr := kbEntryFallback(ctx, cfg, req)
	warning := ""
	if err != nil {
		warning = "KB vector search unavailable; used raw source-entry fallback: " + userFacingKBSearchError(err)
	} else if len(entries) > 0 {
		warning = "KB vector search returned no results; used raw source-entry fallback"
	}
	if previewErr != nil {
		warning = strings.TrimSpace(warning + "; raw entry fallback warning: " + previewErr.Error())
	}
	return map[string]any{"mode": "raw_entry_fallback", "results": entries}, warning
}

func collectionAssetEvidence(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) map[string]any {
	rows, err := queryRows(ctx, db, req.TenantID, `
SELECT evidence_id::text, source_file, source_entry, detected_record_type::text AS detected_record_type,
       structured_status, rag_status, storage_mode, size_bytes, updated_at
FROM forensic.kb_collection_assets
WHERE tenant_id = $1 AND collection_id = $2
  AND (
    $3 = ''
    OR source_file ILIKE '%' || $3 || '%'
    OR coalesce(source_entry, '') ILIKE '%' || $3 || '%'
    OR detected_record_type::text ILIKE '%' || $3 || '%'
  )
ORDER BY
  CASE WHEN rag_status = 'mirrored' THEN 0 ELSE 1 END,
  updated_at DESC
LIMIT $4`, req.TenantID, req.CollectionID, req.Target, req.MaxKBResults)
	if err != nil || len(rows) == 0 {
		return map[string]any{"mode": "structured_asset_fallback", "results": []any{}}
	}
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		sourceFile := fmt.Sprint(row["source_file"])
		sourceEntry := strings.TrimSpace(fmt.Sprint(row["source_entry"]))
		if sourceEntry == "<nil>" {
			sourceEntry = ""
		}
		recordType := fmt.Sprint(row["detected_record_type"])
		status := fmt.Sprintf("structured=%s, rag=%s, storage=%s", row["structured_status"], row["rag_status"], row["storage_mode"])
		preview := fmt.Sprintf("Structured KB asset %s is registered in collection %s as %s. Status: %s.", sourceFile, req.CollectionID, recordType, status)
		source := sourceEntry
		if source == "" {
			source = sourceFile
		}
		results = append(results, map[string]any{
			"content": preview,
			"preview": preview,
			"entry":   source,
			"metadata": map[string]any{
				"evidence_id":          row["evidence_id"],
				"file_name":            sourceFile,
				"source":               source,
				"detected_record_type": recordType,
				"rag_status":           row["rag_status"],
				"structured_status":    row["structured_status"],
			},
			"similarity": 0,
		})
	}
	return map[string]any{"mode": "structured_asset_fallback", "results": results}
}

func evidenceResults(evidence map[string]any) []map[string]any {
	switch typed := evidence["results"].(type) {
	case []map[string]any:
		return typed
	case []any:
		results := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			row, ok := item.(map[string]any)
			if ok {
				results = append(results, row)
			}
		}
		return results
	default:
		return nil
	}
}

func countResultRows(value any) int {
	switch typed := value.(type) {
	case nil:
		return 0
	case []map[string]any:
		return len(typed)
	case []any:
		count := 0
		for _, item := range typed {
			if _, ok := item.(map[string]any); ok {
				count++
				continue
			}
			count += countResultRows(item)
		}
		return count
	case map[string]any:
		count := 0
		for key, item := range typed {
			if isResultMetadataKey(key) {
				continue
			}
			count += countResultRows(item)
		}
		return count
	default:
		return 0
	}
}

func canonicalAnswerRowCount(template string, records map[string]any) int {
	if template == "canonical_records" {
		if total, ok := canonicalTotalCount(records); ok {
			return int(total)
		}
	}
	return countResultRows(records)
}

func canonicalTotalCount(records map[string]any) (int64, bool) {
	value, ok := records["total_count"]
	if !ok {
		return 0, false
	}
	return int64FromAny(value), true
}

func isResultMetadataKey(key string) bool {
	switch key {
	case "row_count", "total_count", "limit", "offset", "target", "targets", "date_from", "date_to",
		"query_route", "canonical_table", "filters", "sort", "provenance":
		return true
	default:
		return false
	}
}

func kbSearch(ctx context.Context, cfg config, req hybridQueryRequest) ([]map[string]any, error) {
	payload, _ := json.Marshal(map[string]any{"query": req.Query, "max_results": req.MaxKBResults})
	searchURL := cfg.LocalAIURL + "/api/agents/collections/" + url.PathEscape(req.CollectionID) + "/search"
	if req.UserID != "" {
		values := url.Values{}
		values.Set("user_id", req.UserID)
		searchURL += "?" + values.Encode()
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.LocalAIAPIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.LocalAIAPIKey)
	}
	resp, err := httpclient.NewWithTimeout(30 * time.Second).Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("kb search status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	return decoded.Results, nil
}

func kbEntryFallback(ctx context.Context, cfg config, req hybridQueryRequest) ([]map[string]any, error) {
	entries, err := kbEntries(ctx, cfg, req)
	if err != nil {
		return nil, err
	}
	terms := queryTerms(req.Query + " " + req.Target)
	var results []map[string]any
	for _, entry := range entries {
		content, chunks, err := kbEntryContent(ctx, cfg, req, entry)
		if err != nil {
			continue
		}
		score := lexicalScore(strings.ToLower(content), terms)
		if score == 0 && len(terms) > 0 {
			continue
		}
		results = append(results, map[string]any{
			"entry":       entry,
			"chunk_count": chunks,
			"score":       score,
			"preview":     truncate(content, 2000),
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i]["score"].(int) > results[j]["score"].(int)
	})
	if len(results) > req.MaxKBResults {
		results = results[:req.MaxKBResults]
	}
	return results, nil
}

func userFacingKBSearchError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.Contains(msg, "nResults must be") {
		return "KB vector index has fewer searchable chunks than requested for this collection"
	}
	if strings.Contains(msg, "collection not found") || strings.Contains(msg, "status=404") {
		return "LocalAI Knowledge Base collection is not currently indexed or reachable; exact records analytics still ran, and structured KB asset metadata can be used as a fallback"
	}
	return msg
}

func kbEntries(ctx context.Context, cfg config, req hybridQueryRequest) ([]string, error) {
	entriesURL := cfg.LocalAIURL + "/api/agents/collections/" + url.PathEscape(req.CollectionID) + "/entries"
	if req.UserID != "" {
		values := url.Values{}
		values.Set("user_id", req.UserID)
		entriesURL += "?" + values.Encode()
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, entriesURL, nil)
	if err != nil {
		return nil, err
	}
	if cfg.LocalAIAPIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.LocalAIAPIKey)
	}
	resp, err := httpclient.NewWithTimeout(30 * time.Second).Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("kb entries status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded struct {
		Entries []string `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	return decoded.Entries, nil
}

func kbEntryContent(ctx context.Context, cfg config, req hybridQueryRequest, entry string) (string, int, error) {
	contentURL := cfg.LocalAIURL + "/api/agents/collections/" + url.PathEscape(req.CollectionID) + "/entries/" + escapeEntryPath(entry)
	if req.UserID != "" {
		values := url.Values{}
		values.Set("user_id", req.UserID)
		contentURL += "?" + values.Encode()
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, contentURL, nil)
	if err != nil {
		return "", 0, err
	}
	if cfg.LocalAIAPIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.LocalAIAPIKey)
	}
	resp, err := httpclient.NewWithTimeout(30 * time.Second).Do(httpReq)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", 0, fmt.Errorf("kb entry content status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded struct {
		Content    string `json:"content"`
		ChunkCount int    `json:"chunk_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", 0, err
	}
	return decoded.Content, decoded.ChunkCount, nil
}

func summarizeRecords(template string, records map[string]any) string {
	switch template {
	case "collection_overview":
		return "Computed collection ingest, KB asset, and record-family overview."
	case "frequent_contacts":
		return "Computed frequent contact matrix from cdr_frequent_contacts."
	case "call_type_breakdown":
		return "Computed CDR event counts by call type and direction."
	case "temporal_activity":
		return "Computed hourly baseline, nocturnal count, non-zero duration statistics, and shortest/longest audited duration rows."
	case "top_locations":
		return "Computed most frequent CDR locations and cell sites."
	case "geospatial_movement":
		return "Computed movement timeline and off-peak base-location candidates."
	case "anpr_sightings":
		return "Computed ANPR sightings from generic structured records."
	case "relationship_network":
		return "Computed co-observed related entities from the normalized entity index."
	case "entity_timeline":
		return "Computed a chronological cross-record timeline from CDR and generic records."
	case "source_records":
		return "Retrieved a capped, auditable set of matching source rows."
	case "canonical_records":
		return "Queried forensic.records with parameterized canonical filters, paging, total count, and source provenance."
	case "schema_profile":
		return "Retrieved detected headers, schemas, routing status, and quality reports."
	case "data_quality":
		return "Computed ingest quality, duplicate, rejection, and parser-error status."
	case "case_readiness":
		return "Computed deterministic case readiness from ingest status, source traceability, duplicates, rejected rows, and record-family coverage."
	case "entity_activity", "evidence":
		return "Computed normalized entity activity across ingested record families."
	default:
		return "Computed deterministic records result."
	}
}

func summarizeEvidence(evidence map[string]any) string {
	mode, _ := evidence["mode"].(string)
	return "Retrieved Knowledge Base evidence using " + defaultString(mode, "unknown")
}

func shouldLoadCoverage(req hybridQueryRequest, resp hybridQueryResponse) bool {
	if resp.Intent == intentClarify {
		return true
	}
	if stringValueAny(resp.Answer["records_status"]) == "no_matching_records" || stringValueAny(resp.Answer["records_row_count"]) == "0" {
		return true
	}
	return req.DateFrom != "" || req.DateTo != ""
}

func synthesizeWithBoundedLLM(ctx context.Context, cfg config, req hybridQueryRequest, resp hybridQueryResponse) (string, string) {
	model := strings.TrimSpace(req.SynthesisModel)
	if model == "" {
		model = strings.TrimSpace(cfg.SynthesisModel)
	}
	if cfg.LocalAIURL == "" || model == "" {
		return "", "LLM synthesis skipped - no synthesis_model was requested and FORENSIC_SYNTHESIS_MODEL is not configured"
	}
	llmCtx, cancel := context.WithTimeout(ctx, 3500*time.Millisecond)
	defer cancel()
	payload, _ := json.Marshal(map[string]any{
		"model":       model,
		"temperature": 0,
		"max_tokens":  180,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You summarize forensic records results. Use only supplied JSON facts. Do not invent counts, dates, entities, or sources. Keep the answer under 120 words.",
			},
			{
				"role": "user",
				"content": truncate(marshalJSONString(map[string]any{
					"query":      req.Query,
					"template":   resp.Template,
					"route":      resp.Route,
					"answer":     resp.Answer,
					"coverage":   resp.Coverage,
					"row_sample": flattenEnterpriseRows(resp.Records, 8),
				}), 8000),
			},
		},
	})
	httpReq, err := http.NewRequestWithContext(llmCtx, http.MethodPost, cfg.LocalAIURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", "LLM synthesis request build failed - defaulted to deterministic engine"
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.LocalAIAPIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.LocalAIAPIKey)
	}
	httpResp, err := httpclient.NewWithTimeout(3500 * time.Millisecond).Do(httpReq)
	if err != nil {
		return "", "LLM synthesis timeout - defaulted to deterministic engine"
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Sprintf("LLM synthesis failed with status %d - defaulted to deterministic engine", httpResp.StatusCode)
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(httpResp.Body, 1<<20)).Decode(&decoded); err != nil {
		return "", "LLM synthesis response decode failed - defaulted to deterministic engine"
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return "", "LLM synthesis returned no content - defaulted to deterministic engine"
	}
	return strings.TrimSpace(decoded.Choices[0].Message.Content), ""
}

func marshalJSONString(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func buildQueryTelemetry(requestID string, dbLatencyMS, kbLatencyMS, llmLatencyMS int64, startedAt time.Time, resp hybridQueryResponse) QueryTelemetry {
	return QueryTelemetry{
		RequestID:         requestID,
		DBLatencyMS:       dbLatencyMS,
		KBLatencyMS:       kbLatencyMS,
		LLMLatencyMS:      llmLatencyMS,
		TotalLatencyMS:    time.Since(startedAt).Milliseconds(),
		PlannerConfidence: numericFloat(resp.Planner["confidence"]),
		ExecutionPath:     strings.Join(resp.Route, "+"),
	}
}

func numericFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func int64FromAny(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case float64:
		return int64(typed)
	case pgtype.Numeric:
		text := formatPGNumeric(typed)
		var out int64
		fmt.Sscan(text, &out)
		return out
	default:
		return 0
	}
}

func buildEnterprisePayload(req hybridQueryRequest, resp hybridQueryResponse) map[string]any {
	rows := flattenEnterpriseRows(resp.Records, 100)
	provenance := enterpriseProvenance(req, rows, resp.Evidence)
	limitations := enterpriseLimitations(resp)
	if count := numericFloat(resp.Answer["records_row_count"]); count > float64(len(rows)) && len(rows) >= 100 {
		limitations = append(limitations, fmt.Sprintf("Data grid is capped at %d displayed rows from %.0f matching records.", len(rows), count))
	}
	metrics := enterpriseMetrics(req, resp, rows, provenance)
	claims := enterpriseClaims(resp)
	return map[string]any{
		"status":              enterpriseOutcomeStatus(resp, rows),
		"summary":             enterpriseSummary(req, resp, metrics, limitations),
		"metrics":             metrics,
		"data_grid":           enterpriseDataGrid(rows),
		"provenance":          provenance,
		"limitations":         limitations,
		"recommended_actions": enterpriseRecommendedActions(req, resp, rows),
		"coverage": map[string]any{
			"tenant_id":                req.TenantID,
			"collection_id":            req.CollectionID,
			"template":                 resp.Template,
			"route":                    resp.Route,
			"queried_record_sql":       containsString(resp.Route, "records_sql"),
			"queried_kb":               containsString(resp.Route, "kb_rag"),
			"date_from":                req.DateFrom,
			"date_to":                  req.DateTo,
			"target":                   req.Target,
			"collection_min_timestamp": resp.Coverage.CollectionMinTimestamp,
			"collection_max_timestamp": resp.Coverage.CollectionMaxTimestamp,
			"total_indexed_records":    resp.Coverage.TotalIndexedRecords,
			"record_families_present":  resp.Coverage.RecordFamiliesPresent,
			"nearest_activity":         resp.Coverage.NearestActivity,
			"valid_target_examples":    resp.Coverage.ValidTargetExamples,
		},
		"telemetry": resp.Telemetry,
		"synthesis": map[string]any{
			"claims": claims,
			"policy": "Claims are labeled by source: deterministic SQL facts are computed from records templates; semantic context comes from Knowledge Base retrieval.",
		},
	}
}

func enterpriseSummary(req hybridQueryRequest, resp hybridQueryResponse, metrics []map[string]any, limitations []string) string {
	title := "Executive Intelligence Brief"
	if resp.Intent == intentClarify {
		return title + "\n\nClarification is required before running this forensic records query. No SQL template was executed."
	}
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")
	if stringValueAny(resp.Answer["records_status"]) == "no_matching_records" || stringValueAny(resp.Answer["records_row_count"]) == "0" {
		b.WriteString("No matching structured records were found for this query.")
		if req.DateFrom != "" || req.DateTo != "" {
			b.WriteString(fmt.Sprintf(" The selected date bounds were `%s` to `%s`.", defaultString(req.DateFrom, "open"), defaultString(req.DateTo, "open")))
		}
		if req.Target != "" {
			b.WriteString(fmt.Sprintf(" Target `%s` was applied.", req.Target))
		}
		b.WriteString(" This is a negative result, not a generated narrative: broaden the time window, remove the target filter, or audit available source files before drawing a case conclusion.")
		if len(limitations) > 0 {
			b.WriteString(fmt.Sprintf("\n\nCoverage note: %d limitation or caveat was detected.", len(limitations)))
		}
		if fallback := strings.TrimSpace(stringValueAny(resp.Answer["llm_fallback_reason"])); fallback != "" {
			b.WriteString("\n\nSynthesis fallback: ")
			b.WriteString(fallback)
		}
		_ = metrics
		return b.String()
	}
	if summary := strings.TrimSpace(stringValueAny(resp.Answer["llm_summary"])); summary != "" {
		return title + "\n\n" + summary
	}
	b.WriteString(fmt.Sprintf("Collection `%s` was analyzed with `%s` using route `%s`.", req.CollectionID, resp.Template, strings.Join(resp.Route, " + ")))
	if req.Target != "" {
		b.WriteString(fmt.Sprintf(" Target `%s` was applied.", req.Target))
	}
	if req.DateFrom != "" || req.DateTo != "" {
		b.WriteString(fmt.Sprintf(" Date bounds: `%s` to `%s`.", defaultString(req.DateFrom, "open"), defaultString(req.DateTo, "open")))
	}
	if summary, ok := resp.Answer["records_summary"].(string); ok && summary != "" {
		b.WriteString("\n\n")
		b.WriteString("[Deterministic Fact (SQL)] ")
		b.WriteString(summary)
	}
	if summary, ok := resp.Answer["evidence_summary"].(string); ok && summary != "" {
		b.WriteString("\n\n")
		b.WriteString("[Semantic Context (KB)] ")
		b.WriteString(summary)
	}
	if len(limitations) > 0 {
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Coverage note: %d limitation or caveat was detected.", len(limitations)))
	}
	if fallback := strings.TrimSpace(stringValueAny(resp.Answer["llm_fallback_reason"])); fallback != "" {
		b.WriteString("\n\n")
		b.WriteString("Synthesis fallback: ")
		b.WriteString(fallback)
	}
	_ = metrics
	return b.String()
}

func enterpriseOutcomeStatus(resp hybridQueryResponse, rows []map[string]any) string {
	if resp.Intent == intentClarify || stringValueAny(resp.Answer["clarification_required"]) == "true" {
		return "needs_input"
	}
	if len(rows) == 0 || stringValueAny(resp.Answer["records_status"]) == "no_matching_records" || stringValueAny(resp.Answer["records_row_count"]) == "0" {
		return "no_results"
	}
	if len(resp.Warnings) > 0 || len(enterpriseLimitations(resp)) > 0 {
		return "answered_with_limitations"
	}
	return "answered"
}

func enterpriseRecommendedActions(req hybridQueryRequest, resp hybridQueryResponse, rows []map[string]any) []map[string]any {
	added := map[string]struct{}{}
	var actions []map[string]any
	add := func(label, query, template, reason string) {
		query = strings.TrimSpace(query)
		template = strings.TrimSpace(template)
		if query == "" && template == "" {
			return
		}
		key := query + "|" + template
		if _, ok := added[key]; ok {
			return
		}
		added[key] = struct{}{}
		actions = append(actions, map[string]any{
			"label":    label,
			"query":    query,
			"template": template,
			"reason":   reason,
		})
	}

	if resp.Intent == intentClarify || stringValueAny(resp.Answer["clarification_required"]) == "true" {
		add("Show available entities", "show available entities", "entity_activity", "Find valid targets before running a target-specific workflow.")
		add("Audit source files", "which files were ingested?", "source_file_audit", "Confirm the collection has the expected source evidence.")
		return actions
	}

	noResults := len(rows) == 0 || stringValueAny(resp.Answer["records_status"]) == "no_matching_records" || stringValueAny(resp.Answer["records_row_count"]) == "0"
	if noResults {
		if resp.Coverage.CollectionMinTimestamp != "" || resp.Coverage.CollectionMaxTimestamp != "" {
			add(
				"Expand date bounds",
				"build timeline",
				"entity_timeline",
				fmt.Sprintf("Available collection range is %s to %s.", defaultString(resp.Coverage.CollectionMinTimestamp, "unknown"), defaultString(resp.Coverage.CollectionMaxTimestamp, "unknown")),
			)
		}
		add("Audit source files", "which files were ingested?", "source_file_audit", "Confirm what evidence exists in this collection.")
		add("Show entity coverage", "show available entities", "entity_activity", "Find targets and date coverage that are actually indexed.")
		if req.DateFrom != "" || req.DateTo != "" {
			add("Remove date filter", "build timeline", "entity_timeline", "Run the same workflow without the current date bounds.")
		}
		if req.Target != "" {
			add("Broaden target search", "show source records", "source_records", "Inspect nearby rows without relying on one exact template match.")
		}
		add("Check readiness", "is this case ready for production?", "case_readiness", "Review ingest quality, duplicate rows, rejected rows, and KB coverage.")
		return actions
	}

	switch resp.Template {
	case "source_file_audit", "collection_overview":
		add("Assess readiness", "is this case ready for production?", "case_readiness", "Turn inventory into an operational quality decision.")
		add("Find entities", "show available entities", "entity_activity", "Identify usable numbers, plates, IPs, and related identifiers.")
	case "entity_activity", "frequent_contacts", "relationship_network":
		add("Build timeline", "build timeline", "entity_timeline", "Move from entity coverage to chronological review.")
		add("Show source rows", "show source records", "source_records", "Inspect row-level evidence behind the entity summary.")
	case "entity_timeline", "geospatial_movement", "anpr_sightings":
		add("Show source rows", "show source records", "source_records", "Audit the records behind the timeline or movement view.")
		add("Check relationships", "show relationship network", "relationship_network", "Look for co-observed entities and links.")
	default:
		add("Audit source files", "which files were ingested?", "source_file_audit", "Review traceability and evidence coverage.")
		add("Check readiness", "is this case ready for production?", "case_readiness", "Confirm quality before reporting.")
	}
	return actions
}

func enterpriseMetrics(req hybridQueryRequest, resp hybridQueryResponse, rows []map[string]any, provenance []map[string]any) []map[string]any {
	metrics := []map[string]any{
		{"label": "Template", "value": resp.Template, "source": "planner"},
		{"label": "Route", "value": strings.Join(resp.Route, " + "), "source": "planner"},
		{"label": "Planner Confidence", "value": resp.Planner["confidence"], "source": "planner"},
		{"label": "Display Rows", "value": len(rows), "source": "records_sql"},
		{"label": "Records Row Count", "value": resp.Answer["records_row_count"], "source": "records_sql"},
		{"label": "Evidence Count", "value": resp.Answer["evidence_count"], "source": "kb_rag"},
		{"label": "Provenance Items", "value": len(provenance), "source": "records_sql+kb_rag"},
	}
	if req.Target != "" {
		metrics = append(metrics, map[string]any{"label": "Target", "value": req.Target, "source": "planner"})
	}
	if req.DateFrom != "" || req.DateTo != "" {
		metrics = append(metrics, map[string]any{"label": "Date Range", "value": strings.TrimSpace(defaultString(req.DateFrom, "open") + " - " + defaultString(req.DateTo, "open")), "source": "planner"})
	}
	if summary, ok := resp.Records["summary"].(map[string]any); ok {
		for _, key := range []string{"total_rows", "accepted_rows", "duplicate_rows", "rejected_rows", "jobs_total", "kb_assets_total", "unique_targets_count"} {
			if value, ok := summary[key]; ok {
				metrics = append(metrics, map[string]any{"label": humanizeField(key), "value": value, "source": "records_sql"})
			}
		}
	}
	return metrics
}

func enterpriseDataGrid(rows []map[string]any) map[string]any {
	return map[string]any{
		"columns": inferEnterpriseColumns(rows),
		"rows":    rows,
		"count":   len(rows),
	}
}

func enterpriseProvenance(req hybridQueryRequest, rows []map[string]any, evidence map[string]any) []map[string]any {
	out := make([]map[string]any, 0)
	seen := map[string]struct{}{}
	add := func(item map[string]any) {
		if len(out) >= 50 {
			return
		}
		key := fmt.Sprint(item["source_file"], "|", item["source_entry"], "|", item["row_number"], "|", item["citation"])
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	for _, row := range rows {
		if firstPresent(row, "source_file", "source_entry", "row_number", "timestamp") == nil {
			continue
		}
		add(map[string]any{
			"source":        "records_sql",
			"collection_id": req.CollectionID,
			"record_type":   firstPresent(row, "record_type", "detected_record_type"),
			"source_file":   firstPresent(row, "source_file", "file_name"),
			"source_entry":  row["source_entry"],
			"row_number":    row["row_number"],
			"timestamp":     firstPresent(row, "timestamp", "call_start_ts", "observed_at", "queued_at", "completed_at"),
		})
	}
	for _, item := range evidenceResults(evidence) {
		metadata, _ := item["metadata"].(map[string]any)
		add(map[string]any{
			"source":        "kb_rag",
			"collection_id": req.CollectionID,
			"source_file":   firstPresent(metadata, "file_name", "source"),
			"source_entry":  firstPresent(item, "entry", "source"),
			"citation":      firstPresent(item, "citation", "id"),
			"score":         firstPresent(item, "similarity", "score"),
			"preview":       firstPresent(item, "preview", "content"),
		})
	}
	return out
}

func enterpriseLimitations(resp hybridQueryResponse) []string {
	var out []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" && !containsString(out, value) {
			out = append(out, value)
		}
	}
	for _, key := range []string{"records_limitation", "evidence_limitation", "llm_fallback_reason", "clarification", "guardrail"} {
		add(stringValueAny(resp.Answer[key]))
	}
	for _, limitation := range stringSliceAny(resp.Answer["limitations"]) {
		add(limitation)
	}
	for _, warning := range resp.Warnings {
		add(warning)
	}
	if stringValueAny(resp.Answer["records_status"]) == "no_matching_records" {
		add("No matching structured records were returned for the selected collection, template, and filters.")
	}
	return out
}

func enterpriseClaims(resp hybridQueryResponse) []map[string]any {
	var claims []map[string]any
	if summary := stringValueAny(resp.Answer["records_summary"]); summary != "" {
		claims = append(claims, map[string]any{"source": "Deterministic Fact (SQL)", "claim": summary, "template": resp.Template})
	}
	if count := resp.Answer["records_row_count"]; count != nil {
		claims = append(claims, map[string]any{"source": "Deterministic Fact (SQL)", "claim": "Structured records row count computed.", "value": count, "template": resp.Template})
	}
	if summary := stringValueAny(resp.Answer["evidence_summary"]); summary != "" {
		claims = append(claims, map[string]any{"source": "Semantic Context (KB)", "claim": summary})
	}
	if clarification := stringValueAny(resp.Answer["clarification"]); clarification != "" {
		claims = append(claims, map[string]any{"source": "Planner Guidance", "claim": clarification})
	}
	return claims
}

func flattenEnterpriseRows(value any, limit int) []map[string]any {
	var out []map[string]any
	var visit func(any, string)
	visit = func(current any, section string) {
		if len(out) >= limit || current == nil {
			return
		}
		switch typed := current.(type) {
		case []map[string]any:
			for _, row := range typed {
				visit(row, section)
			}
		case []any:
			for _, item := range typed {
				visit(item, section)
			}
		case map[string]any:
			if enterpriseIsRow(typed) {
				row := map[string]any{"section": section}
				for key, value := range typed {
					row[key] = value
				}
				out = append(out, row)
				return
			}
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if key == "raw_row" || isResultMetadataKey(key) {
					continue
				}
				visit(typed[key], key)
			}
		}
	}
	visit(value, "records")
	return out
}

func enterpriseIsRow(row map[string]any) bool {
	if len(row) == 0 {
		return false
	}
	hasScalar := false
	for key, value := range row {
		switch value.(type) {
		case map[string]any:
			if key != "raw_row" && key != "metadata" {
				return false
			}
		case []map[string]any:
			return false
		case []any:
			for _, item := range value.([]any) {
				if _, ok := item.(map[string]any); ok {
					return false
				}
			}
		case nil, string, bool, int, int32, int64, float32, float64, time.Time:
			hasScalar = true
		}
	}
	return hasScalar
}

func inferEnterpriseColumns(rows []map[string]any) []map[string]any {
	seen := map[string]struct{}{}
	var columns []map[string]any
	for _, row := range rows {
		keys := make([]string, 0, len(row))
		for key := range row {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			columns = append(columns, map[string]any{
				"key":    key,
				"header": humanizeField(key),
			})
		}
	}
	return columns
}

func humanizeField(key string) string {
	key = strings.TrimSpace(strings.ReplaceAll(key, "_", " "))
	if key == "" {
		return ""
	}
	parts := strings.Fields(key)
	for i, part := range parts {
		if len(part) <= 3 {
			parts[i] = strings.ToUpper(part)
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func stringValueAny(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

func stringSliceAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := stringValueAny(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func firstPresent(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok && strings.TrimSpace(stringValueAny(value)) != "" {
			return value
		}
	}
	return nil
}

func firstRow(rows []map[string]any) map[string]any {
	if len(rows) == 0 {
		return map[string]any{}
	}
	return rows[0]
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultHybridLimit
	}
	if limit > maxHybridLimit {
		return maxHybridLimit
	}
	return limit
}

func clampOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	if offset > 100000 {
		return 100000
	}
	return offset
}

func clampKBResults(limit int) int {
	if limit <= 0 {
		return 1
	}
	if limit > 20 {
		return 20
	}
	return limit
}

func applyCanonicalQueryHints(req hybridQueryRequest, template string) hybridQueryRequest {
	if template != "canonical_records" || strings.TrimSpace(req.Query) == "" {
		return req
	}
	if req.RecordType == "" {
		req.RecordType = extractCanonicalRecordType(req.Query)
	}
	if req.SourceFile == "" {
		req.SourceFile = extractCanonicalSourceFile(req.Query)
	}
	if req.BatchID == "" {
		req.BatchID = extractCanonicalBatchID(req.Query)
	}
	if req.SortBy == "" {
		req.SortBy, req.SortDirection = extractCanonicalSort(req.Query)
	} else if req.SortDirection == "" {
		_, req.SortDirection = extractCanonicalSort(req.Query)
	}
	if limit, ok := extractCanonicalLimit(req.Query); ok {
		req.Limit = clampLimit(limit)
	}
	if offset, ok := extractCanonicalOffset(req.Query); ok {
		req.Offset = clampOffset(offset)
	}
	req.RawPayloadFilters = mergeCanonicalPayloadFilters(req.RawPayloadFilters, extractCanonicalPayloadFilters(req.Query))
	exists, notExists := extractCanonicalFieldExistence(req.Query)
	req.FieldExists = mergeStringSet(req.FieldExists, exists)
	req.FieldNotExists = mergeStringSet(req.FieldNotExists, notExists)
	return req
}

func extractCanonicalRecordType(query string) string {
	q := strings.ToLower(query)
	types := []struct {
		terms []string
		value string
	}{
		{[]string{"access log", "access-log", "access_log"}, "access_log"},
		{[]string{"tower location", "tower-location", "tower_location"}, "tower_location"},
		{[]string{"subscriber"}, "subscriber"},
		{[]string{"transaction"}, "transaction"},
		{[]string{"generic"}, "generic"},
		{[]string{"anpr"}, "anpr"},
		{[]string{"ipdr"}, "ipdr"},
		{[]string{"cdr"}, "cdr"},
	}
	for _, candidate := range types {
		for _, term := range candidate.terms {
			if regexp.MustCompile(`\b` + regexp.QuoteMeta(term) + `\b`).MatchString(q) {
				return candidate.value
			}
		}
	}
	if match := regexp.MustCompile(`(?i)\brecord[_\s-]*type\s*(?:is|=|equals?|equal to)?\s*["']?([a-z][a-z0-9_-]*)`).FindStringSubmatch(query); len(match) == 2 {
		return normalize(match[1])
	}
	return ""
}

func extractCanonicalSourceFile(query string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bsource[_\s-]*file\s*(?:is|=|named|name|:)?\s*("[^"]+"|'[^']+'|[^\s,;]+)`),
		regexp.MustCompile(`(?i)\bfrom\s+file\s*("[^"]+"|'[^']+'|[^\s,;]+)`),
	}
	for _, pattern := range patterns {
		if match := pattern.FindStringSubmatch(query); len(match) == 2 {
			return trimCanonicalToken(match[1])
		}
	}
	return ""
}

func extractCanonicalBatchID(query string) string {
	if match := regexp.MustCompile(`(?i)\bbatch(?:[_\s-]*id)?\s*(?:is|=|:)?\s*([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|[A-Za-z0-9_.:-]+)`).FindStringSubmatch(query); len(match) == 2 {
		value := trimCanonicalToken(match[1])
		if strings.Trim(value, "-_:") != "" {
			return value
		}
	}
	return ""
}

func extractCanonicalPayloadFilters(query string) []CanonicalPayloadFilter {
	var out []CanonicalPayloadFilter
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:raw[_\s-]*payload|attribute|property|field)\s+([A-Za-z][A-Za-z0-9_.-]*(?:\s+[A-Za-z][A-Za-z0-9_.-]*)?)\s+(is not|!=|<>|not equals?|not equal to|>=|=>|<=|=<|>|<|greater than or equal to|at least|greater than|more than|less than or equal to|at most|less than|is|=|equals?|equal to|contains|like|in)\s+("[^"]+"|'[^']+'|[A-Za-z0-9_.:+@/-]+(?:\s*,\s*[A-Za-z0-9_.:+@/-]+)*)`),
		regexp.MustCompile(`(?i)\bwhere\s+([A-Za-z][A-Za-z0-9_.-]*(?:\s+[A-Za-z][A-Za-z0-9_.-]*)?)\s+(is not|!=|<>|not equals?|not equal to|>=|=>|<=|=<|>|<|greater than or equal to|at least|greater than|more than|less than or equal to|at most|less than|is|=|equals?|equal to|contains|like|in)\s+("[^"]+"|'[^']+'|[A-Za-z0-9_.:+@/-]+(?:\s*,\s*[A-Za-z0-9_.:+@/-]+)*)`),
	}
	seen := map[string]struct{}{}
	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatch(query, -1) {
			if len(match) != 4 {
				continue
			}
			field := normalizeCanonicalFieldName(match[1])
			if !isUsefulCanonicalPayloadField(field) {
				continue
			}
			op := normalizeCanonicalPayloadOp(match[2])
			value := trimCanonicalToken(match[3])
			if value == "" {
				continue
			}
			key := field + "|" + op + "|" + strings.ToLower(value)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			filter := CanonicalPayloadFilter{Field: field, Op: op}
			if op == "in" {
				for _, part := range strings.Split(value, ",") {
					if item := trimCanonicalToken(part); item != "" {
						filter.Values = append(filter.Values, item)
					}
				}
				if len(filter.Values) == 0 {
					filter.Values = []any{value}
				}
			} else {
				filter.Value = value
			}
			out = append(out, filter)
		}
	}
	return out
}

func extractCanonicalFieldExistence(query string) ([]string, []string) {
	var exists []string
	var notExists []string
	patterns := []struct {
		re        *regexp.Regexp
		notExists bool
	}{
		{regexp.MustCompile(`(?i)(?:raw[_\s-]*payload\s+)?(?:field|attribute|property)?\s*([A-Za-z][A-Za-z0-9_.-]*(?:\s+[A-Za-z][A-Za-z0-9_.-]*)?)\s+(?:exists|is present|present)`), false},
		{regexp.MustCompile(`(?i)(?:raw[_\s-]*payload\s+)?(?:field|attribute|property)?\s*([A-Za-z][A-Za-z0-9_.-]*(?:\s+[A-Za-z][A-Za-z0-9_.-]*)?)\s+(?:does not exist|not exists|is missing|missing|absent)`), true},
	}
	for _, pattern := range patterns {
		for _, match := range pattern.re.FindAllStringSubmatch(query, -1) {
			if len(match) != 2 {
				continue
			}
			field := normalizeCanonicalFieldName(match[1])
			if !isUsefulCanonicalPayloadField(field) {
				continue
			}
			if pattern.notExists {
				notExists = append(notExists, field)
			} else {
				exists = append(exists, field)
			}
		}
	}
	return uniqueStrings(exists), uniqueStrings(notExists)
}

func extractCanonicalSort(query string) (string, string) {
	q := strings.ToLower(query)
	if match := regexp.MustCompile(`(?i)\bsort\s+by\s+(timestamp|record_type|primary_target|secondary_target|source_file|row_number|ingested_at)(?:\s+(asc|ascending|desc|descending))?`).FindStringSubmatch(query); len(match) >= 2 {
		direction := "desc"
		if len(match) >= 3 && strings.TrimSpace(match[2]) != "" {
			direction = match[2]
		}
		return normalize(match[1]), canonicalDirectionAlias(direction)
	}
	switch {
	case containsAny(q, []string{"oldest first", "earliest first", "ascending time"}):
		return "timestamp", "asc"
	case containsAny(q, []string{"latest first", "newest first", "most recent first", "descending time"}):
		return "timestamp", "desc"
	default:
		return "", ""
	}
}

func extractCanonicalLimit(query string) (int, bool) {
	return extractPositiveInt(query, `(?i)\b(?:limit|top|first)\s+(\d{1,4})\b`)
}

func extractCanonicalOffset(query string) (int, bool) {
	return extractPositiveInt(query, `(?i)\boffset\s+(\d{1,6})\b`)
}

func extractPositiveInt(query, pattern string) (int, bool) {
	if match := regexp.MustCompile(pattern).FindStringSubmatch(query); len(match) == 2 {
		var out int
		if _, err := fmt.Sscan(match[1], &out); err == nil && out >= 0 {
			return out, true
		}
	}
	return 0, false
}

func normalizeCanonicalPayloadOp(op string) string {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "!=", "<>", "is not":
		return "ne"
	case ">", "greater than", "more than":
		return "gt"
	case ">=", "=>", "greater than or equal to", "at least":
		return "gte"
	case "<", "less than":
		return "lt"
	case "<=", "=<", "less than or equal to", "at most":
		return "lte"
	}
	switch normalize(op) {
	case "not_equal", "not_equals", "not_equal_to":
		return "ne"
	case "contains", "like":
		return "contains"
	case "in":
		return "in"
	default:
		return "eq"
	}
}

func isCanonicalNumericLiteral(value string) bool {
	return regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]+)?$`).MatchString(strings.TrimSpace(value))
}

func canonicalDirectionAlias(value string) string {
	switch normalize(value) {
	case "asc", "ascending":
		return "asc"
	default:
		return "desc"
	}
}

func normalizeCanonicalFieldName(field string) string {
	field = strings.TrimSpace(strings.Trim(field, "\"'`:,.;"))
	field = regexp.MustCompile(`(?i)^(?:raw[_\s-]*payload|field|attribute|property)\s+`).ReplaceAllString(field, "")
	field = regexp.MustCompile(`(?i)^(?:where|and|with|having)\s+`).ReplaceAllString(field, "")
	field = strings.TrimSpace(field)
	field = strings.ReplaceAll(field, "-", "_")
	field = strings.ReplaceAll(field, " ", "_")
	return strings.ToLower(strings.Trim(field, "_"))
}

func trimCanonicalToken(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"'`")
	return strings.Trim(value, ",.;")
}

func isUsefulCanonicalPayloadField(field string) bool {
	field = normalize(field)
	if field == "" || len(field) < 2 {
		return false
	}
	if strings.Contains(field, "__") {
		return false
	}
	if strings.HasSuffix(field, "_not") || strings.Contains(field, "_not_") {
		return false
	}
	switch field {
	case "record_type", "source_file", "batch_id", "tenant_id", "collection_id", "target", "targets",
		"primary_target", "secondary_target", "timestamp", "rows", "records", "record", "where":
		return false
	default:
		return true
	}
}

func mergeCanonicalPayloadFilters(existing, extracted []CanonicalPayloadFilter) []CanonicalPayloadFilter {
	out := append([]CanonicalPayloadFilter{}, existing...)
	seen := map[string]struct{}{}
	for _, filter := range out {
		seen[canonicalPayloadFilterKey(filter)] = struct{}{}
	}
	for _, filter := range extracted {
		key := canonicalPayloadFilterKey(filter)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, filter)
	}
	return out
}

func canonicalPayloadFilterKey(filter CanonicalPayloadFilter) string {
	var values []string
	for _, value := range filter.Values {
		values = append(values, canonicalFilterValue(value))
	}
	return strings.ToLower(filter.Field + "|" + filter.Op + "|" + canonicalFilterValue(filter.Value) + "|" + strings.Join(values, ","))
}

func mergeStringSet(existing, extracted []string) []string {
	out := append([]string{}, existing...)
	seen := map[string]struct{}{}
	for _, value := range out {
		seen[strings.ToLower(value)] = struct{}{}
	}
	for _, value := range extracted {
		value = normalizeCanonicalFieldName(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func uniqueStrings(values []string) []string {
	return mergeStringSet(nil, values)
}

func shouldPromotePlannerTargets(req hybridQueryRequest, template string, explicitTargetProvided bool) bool {
	if explicitTargetProvided {
		return true
	}
	if template != "canonical_records" {
		return true
	}
	return !hasCanonicalFilters(req)
}

func hasCanonicalFilters(req hybridQueryRequest) bool {
	return req.RecordType != "" ||
		req.SourceFile != "" ||
		req.BatchID != "" ||
		len(req.RawPayloadFilters) > 0 ||
		len(req.FieldFilters) > 0 ||
		len(req.FieldExists) > 0 ||
		len(req.FieldNotExists) > 0
}

func canonicalTargetSet(primary string, groups ...[]string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		normalized := normalizeExtractedTarget(value)
		if normalized == "" {
			normalized = value
		}
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	add(primary)
	for _, group := range groups {
		for _, value := range group {
			add(value)
		}
	}
	return out
}

func applyCanonicalRequestToQueryPlan(plan QueryPlan, req hybridQueryRequest, template string) QueryPlan {
	if plan.AppliedFilters == nil {
		plan.AppliedFilters = map[string]any{}
	}
	if len(req.Targets) > 0 {
		plan.TargetIdentifiers = nonNilStrings(req.Targets)
		plan.AppliedFilters["targets"] = req.Targets
	}
	if req.RecordType != "" {
		plan.AppliedFilters["record_type"] = req.RecordType
	}
	if req.SourceFile != "" {
		plan.AppliedFilters["source_file"] = req.SourceFile
	}
	if req.BatchID != "" {
		plan.AppliedFilters["batch_id"] = req.BatchID
	}
	if len(req.RawPayloadFilters) > 0 {
		plan.AppliedFilters["raw_payload_filters"] = req.RawPayloadFilters
	}
	if len(req.FieldFilters) > 0 || len(req.FieldExists) > 0 || len(req.FieldNotExists) > 0 {
		plan.AppliedFilters["field_filters"] = map[string]any{
			"filters":    req.FieldFilters,
			"exists":     req.FieldExists,
			"not_exists": req.FieldNotExists,
		}
	}
	if req.Offset > 0 || req.Limit != defaultHybridLimit {
		plan.AppliedFilters["pagination"] = map[string]any{"limit": req.Limit, "offset": req.Offset}
	}
	if req.SortBy != "" || req.SortDirection != "" {
		plan.AppliedFilters["sort"] = map[string]any{"by": canonicalSortColumn(req.SortBy), "direction": canonicalSortDirection(req.SortDirection)}
	}
	if template == "canonical_records" {
		plan.AppliedFilters["canonical_table"] = "forensic.records"
		plan.ExecutionStrategy = "canonical_sql_query"
	}
	return plan
}

func containsAny(value string, needles []string) bool {
	terms := map[string]struct{}{}
	for _, term := range rawQueryTerms(value) {
		terms[term] = struct{}{}
	}
	for _, needle := range needles {
		if strings.Contains(needle, " ") {
			if strings.Contains(value, needle) {
				return true
			}
			continue
		}
		if _, ok := terms[needle]; ok {
			return true
		}
	}
	return false
}

func rawQueryTerms(query string) []string {
	re := regexp.MustCompile(`[a-zA-Z0-9]+`)
	return re.FindAllString(strings.ToLower(query), -1)
}

func extractTarget(query string) string {
	return firstTarget(extractTargets(query))
}

func extractTargets(query string) []string {
	type targetMatch struct {
		value string
		start int
	}
	var matches []targetMatch
	for _, pattern := range targetPatterns {
		for _, loc := range pattern.FindAllStringIndex(query, -1) {
			target := normalizeExtractedTarget(query[loc[0]:loc[1]])
			if target == "" {
				continue
			}
			matches = append(matches, targetMatch{value: target, start: loc[0]})
		}
	}
	upper := strings.ToUpper(query)
	for _, loc := range regexp.MustCompile(`\b(?:GPRS|SMS|VOLTE)\b`).FindAllStringIndex(upper, -1) {
		matches = append(matches, targetMatch{value: upper[loc[0]:loc[1]], start: loc[0]})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].start < matches[j].start
	})
	seen := map[string]struct{}{}
	var targets []string
	for _, match := range matches {
		if _, ok := seen[match.value]; ok {
			continue
		}
		seen[match.value] = struct{}{}
		targets = append(targets, match.value)
	}
	return targets
}

func firstTarget(targets []string) string {
	if len(targets) == 0 {
		return ""
	}
	return targets[0]
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func classifyTargetType(target string) string {
	if target == "" {
		return ""
	}
	switch {
	case strings.Contains(target, "@"):
		return "email"
	case regexp.MustCompile(`^(?:\d{1,3}\.){3}\d{1,3}$`).MatchString(target):
		return "ip"
	case regexp.MustCompile(`^\d{8,19}$`).MatchString(target):
		return "phone"
	case target == "GPRS" || target == "SMS" || target == "VOLTE":
		return "call_type"
	case strings.Contains(target, "-"):
		return "identifier"
	default:
		return "entity"
	}
}

func extractFieldHints(query string) []string {
	q := strings.ToLower(query)
	hints := []struct {
		name  string
		terms []string
	}{
		{"duration", []string{"duration", "shortest", "longest"}},
		{"call_type", []string{"call type", "call types", "gprs", "sms", "volte"}},
		{"direction", []string{"incoming", "outgoing", "inbound", "outbound"}},
		{"source_file", []string{"source file", "source files", "which files", "ingested files", "files ingested"}},
		{"source_row", []string{"source row", "source rows", "raw row", "raw rows", "sample rows"}},
		{"location", []string{"location", "locations", "where", "visited", "tower", "cell site", "cell sites", "camera"}},
		{"identity", []string{"subscriber", "imei", "imsi", "msisdn", "entity", "entities"}},
		{"quality", []string{"quality", "duplicate", "duplicates", "rejected", "error", "errors", "failed", "limitations", "missing data"}},
		{"time", []string{"timeline", "chronology", "sequence", "events", "date", "hourly", "daily", "night", "nocturnal"}},
		{"evidence", []string{"evidence", "citation", "citations", "cite", "report", "brief"}},
	}
	var out []string
	for _, hint := range hints {
		if containsAny(q, hint.terms) {
			out = append(out, hint.name)
		}
	}
	return out
}

func isComparisonQuery(q string) bool {
	return containsAny(q, []string{
		"compare",
		"comparison",
		"between these",
		"between those",
		"between the",
		"link these",
		"link those",
		"connect these",
		"connect those",
		"relationship between",
		"relationships between",
		"co presence",
		"co-presence",
		"co travel",
		"co-travel",
	})
}

func normalizeExtractedTarget(match string) string {
	target := strings.TrimSpace(match)
	if target == "" || isIgnoredTarget(target) || isDateLikeTarget(target) {
		return ""
	}
	if regexp.MustCompile(`^(?:\d{1,3}\.){3}\d{1,3}$`).MatchString(target) {
		return target
	}
	digits := regexp.MustCompile(`\D`).ReplaceAllString(target, "")
	if len(digits) >= 8 && len(digits) <= 19 {
		return digits
	}
	if strings.Contains(target, "@") {
		return strings.ToLower(target)
	}
	return strings.ToUpper(strings.ReplaceAll(target, " ", "-"))
}

func isIgnoredTarget(target string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(target, "_", "-"), " ", "-"))
	if regexp.MustCompile(`^(?:on|in|at|by|from|to)-?\d{1,6}$`).MatchString(normalized) {
		return true
	}
	switch normalized {
	case "records-demo", "forensic-records", "local-ai", "localai", "call", "calls", "data", "record", "records":
		return true
	default:
		return false
	}
}

func isDateLikeTarget(target string) bool {
	value := strings.TrimSpace(target)
	if value == "" {
		return false
	}
	if _, _, ok := parseDateExpression(value); ok {
		return true
	}
	digits := regexp.MustCompile(`\D`).ReplaceAllString(value, "")
	if len(digits) == 8 {
		_, _, ok := parseDateExpression(digits)
		return ok
	}
	return false
}

func extractDateRange(query string) (string, string) {
	matches := regexp.MustCompile(`\b\d{4}[-/]\d{1,2}[-/]\d{1,2}\b|\b\d{1,2}[-/]\d{1,2}[-/]\d{4}\b|\b\d{8}\b`).FindAllString(query, -1)
	if len(matches) == 0 {
		return "", ""
	}
	var starts []time.Time
	var ends []time.Time
	for _, match := range matches {
		start, end, ok := parseDateExpression(match)
		if !ok {
			continue
		}
		starts = append(starts, start)
		ends = append(ends, end)
	}
	if len(starts) == 0 {
		return "", ""
	}
	from := starts[0]
	to := ends[0]
	for i := range starts {
		if starts[i].Before(from) {
			from = starts[i]
		}
		if ends[i].After(to) {
			to = ends[i]
		}
	}
	return from.Format(time.RFC3339), to.Format(time.RFC3339)
}

func parseDateExpression(value string) (time.Time, time.Time, bool) {
	value = strings.TrimSpace(value)
	layouts := []string{"2006-01-02", "2006/01/02", "02-01-2006", "02/01/2006", "20060102"}
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, value, time.UTC)
		if err == nil {
			start := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
			return start, start.AddDate(0, 0, 1), true
		}
	}
	return time.Time{}, time.Time{}, false
}

func dateBoundArg(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func normalizeDBValue(value any) any {
	switch v := value.(type) {
	case time.Time:
		return v.UTC().Format(time.RFC3339)
	case []byte:
		return string(v)
	case pgtype.Numeric:
		return formatPGNumeric(v)
	default:
		return v
	}
}

func formatPGNumeric(v pgtype.Numeric) string {
	if !v.Valid {
		return ""
	}
	if v.NaN {
		return "NaN"
	}
	switch v.InfinityModifier {
	case pgtype.Infinity:
		return "Infinity"
	case pgtype.NegativeInfinity:
		return "-Infinity"
	}
	if v.Int == nil {
		return "0"
	}
	rat := new(big.Rat).SetInt(v.Int)
	switch {
	case v.Exp > 0:
		scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(v.Exp)), nil)
		rat.Mul(rat, new(big.Rat).SetInt(scale))
	case v.Exp < 0:
		scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-v.Exp)), nil)
		rat.Quo(rat, new(big.Rat).SetInt(scale))
	}
	decimals := 0
	if v.Exp < 0 {
		decimals = int(-v.Exp)
		if decimals > 6 {
			decimals = 6
		}
	}
	return trimNumericString(rat.FloatString(decimals))
}

func trimNumericString(value string) string {
	if !strings.Contains(value, ".") {
		return value
	}
	value = strings.TrimRight(value, "0")
	value = strings.TrimRight(value, ".")
	if value == "" || value == "-" {
		return "0"
	}
	return value
}

func queryTerms(query string) []string {
	raw := rawQueryTerms(query)
	stopwords := map[string]struct{}{
		"and": {}, "are": {}, "for": {}, "from": {}, "how": {}, "show": {},
		"the": {}, "this": {}, "that": {}, "there": {}, "what": {}, "where": {},
		"with": {}, "evidence": {}, "summarize": {}, "summary": {}, "records": {},
		"record": {}, "source": {}, "sources": {}, "related": {}, "entities": {},
		"entity": {}, "collection": {},
	}
	seen := map[string]struct{}{}
	var terms []string
	for _, term := range raw {
		if len(term) < 2 {
			continue
		}
		if _, skip := stopwords[term]; skip {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}
	return terms
}

func lexicalScore(content string, terms []string) int {
	score := 0
	for _, term := range terms {
		score += strings.Count(content, term)
	}
	return score
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "\n[preview truncated]"
}

func escapeEntryPath(entry string) string {
	parts := strings.Split(entry, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
