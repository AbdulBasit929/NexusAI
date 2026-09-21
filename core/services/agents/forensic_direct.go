package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mudler/LocalAI/pkg/forensicrequest"
	"github.com/mudler/LocalAI/pkg/forensictext"
)

type forensicDirectResult struct {
	ToolName string
	ArgsJSON string
	Text     string
	Raw      any
	Elapsed  time.Duration
}

var forensicTargetPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`),
	regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
	regexp.MustCompile(`(?i)\b[A-Z]{2,6}(?:[-_][A-Z0-9]{2,127})+\b`),
	regexp.MustCompile(`(?i)\b[A-Z]{1,4}[- ]?\d{1,6}[A-Z]{0,3}\b`),
	regexp.MustCompile(`\b\+?\d(?:[\s\-]?\d){7,18}\b`),
}

var forensicUUIDPattern = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)

var forensicSourceSecondRangePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:between\s+)?([0-9]+(?:\.[0-9]+)?)\s*(?:and|aur|to|-)\s*([0-9]+(?:\.[0-9]+)?)\s*(?:seconds?|secs?|s)?\b`),
	regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*سے\s*([0-9]+(?:\.[0-9]+)?)\s*(?:سیکنڈ)?`),
}

var forensicTemplateAssignmentPattern = regexp.MustCompile(`(?i)\btemplate\s*=\s*([a-z][a-z0-9_]*)\b`)

type ForensicRequestClassV1 string

const (
	ForensicRequestDeterministicFastPath ForensicRequestClassV1 = "deterministic_forensic_fast_path"
	ForensicRequestAssistantRouter       ForensicRequestClassV1 = "assistant_request_router"
)

// ClassifyForensicRequestV1 separates only the provable deterministic fast
// path. Everything else goes to the configured assistant, where unrestricted
// language can be classified and the governed forensic tool can be selected.
// This avoids forcing greetings, product help, and relevant general knowledge
// through case analytics while keeping exact known operations low latency.
func ClassifyForensicRequestV1(message string) ForensicRequestClassV1 {
	terminal := ClassifyTerminalForensicRequestV1(message, false, false)
	if deterministicForensicRouteRequested(message) || terminal == forensicrequest.ProductHelp {
		return ForensicRequestDeterministicFastPath
	}
	return ForensicRequestAssistantRouter
}

// ClassifyTerminalForensicRequestV1 exposes the same authoritative terminal
// contract used by the query API. Routing mode remains a separate transport
// choice and must not invent another semantic request class.
func ClassifyTerminalForensicRequestV1(message string, hasEvidenceContext, hasConversationContext bool) forensicrequest.Class {
	return forensicrequest.Classify(forensicrequest.Input{Text: message, HasEvidenceContext: hasEvidenceContext, HasConversationContext: hasConversationContext})
}

// CanRunDeterministicForensicRoute reports whether a message has a provable
// deterministic forensic fast path. Unknown wording is intentionally handled
// by the assistant request router rather than coerced into a case query.
func CanRunDeterministicForensicRoute(cfg *AgentConfig, message, userID string) bool {
	toolConfig := forensicRecordsToolConfig(cfg, userID)
	if toolConfig.APIURL == "" {
		return false
	}
	if terminal := ClassifyTerminalForensicRequestV1(strings.TrimSpace(message), false, toolConfig.ConversationContext != nil); terminal == forensicrequest.ContextualFollowUp || terminal == forensicrequest.ProductHelp || terminal == forensicrequest.Clarify || terminal == forensicrequest.Unsupported {
		return true
	}
	return ClassifyForensicRequestV1(strings.TrimSpace(message)) == ForensicRequestDeterministicFastPath
}

// TryDeterministicForensicRoute executes deterministic records analytics for
// forensic prompts and emits the same callbacks as the normal agent executor.
func TryDeterministicForensicRoute(cfg *AgentConfig, message, userID string, cb Callbacks, opts ...ExecuteChatOpts) (string, bool, error) {
	return TryDeterministicForensicRouteContext(context.Background(), cfg, message, userID, cb, opts...)
}

// TryDeterministicForensicRouteContext binds downstream forensic HTTP work to
// the owning chat request so cancellation stops both execution and delivery.
func TryDeterministicForensicRouteContext(ctx context.Context, cfg *AgentConfig, message, userID string, cb Callbacks, opts ...ExecuteChatOpts) (string, bool, error) {
	toolConfig := forensicRecordsToolConfig(cfg, userID)
	toolConfig.Context = ctx
	if toolConfig.APIURL == "" {
		return "", false, nil
	}
	started := time.Now()
	direct, ok, err := runDeterministicForensicRoute(message, toolConfig)
	if !ok {
		args := buildForensicHybridQueryArgs(message, toolConfig)
		queryToolConfig := toolConfig
		queryToolConfig.Model = args.SynthesisModel
		text, raw, runErr := (ForensicHybridQueryTool{ForensicRecordsToolConfig: queryToolConfig}).Run(args)
		argsJSON, _ := json.Marshal(args)
		direct = forensicDirectResult{
			ToolName: "forensic_hybrid_query",
			ArgsJSON: string(argsJSON),
			Text:     formatDirectForensicResult("forensic_hybrid_query", text, raw),
			Raw:      raw,
		}
		err = runErr
	}
	direct.Elapsed = time.Since(started)
	text, finishErr := finishDeterministicForensicRoute(cfg, direct, err, cb, opts)
	return text, true, finishErr
}

func runDeterministicForensicRoute(message string, toolConfig ForensicRecordsToolConfig) (forensicDirectResult, bool, error) {
	normalized := normalizeForensicRoutingSemantics(message)
	switch {
	case strings.Contains(normalized, "forensic_query_templates") ||
		(strings.Contains(normalized, "template") && strings.Contains(normalized, "forensic") && !strings.Contains(normalized, "template=")):
		tool := ForensicQueryTemplatesTool{ForensicRecordsToolConfig: toolConfig}
		text, raw, err := tool.Run(ForensicQueryTemplatesArgs{})
		if err != nil {
			return forensicDirectResult{}, true, err
		}
		return forensicDirectResult{
			ToolName: "forensic_query_templates",
			ArgsJSON: "{}",
			Text:     formatDirectForensicResult("forensic_query_templates", text, raw),
			Raw:      raw,
		}, true, nil

	case strings.Contains(normalized, "forensic_get_evidence") ||
		(strings.Contains(normalized, "evidence") && strings.Contains(normalized, "detail") && extractForensicEvidenceID(message) != "") ||
		(strings.Contains(normalized, "inspect evidence") && extractForensicEvidenceID(message) != ""):
		args := ForensicGetEvidenceArgs{
			EvidenceID: extractForensicEvidenceID(message),
			Limit:      25,
		}
		tool := ForensicGetEvidenceTool{ForensicRecordsToolConfig: toolConfig}
		text, raw, err := tool.Run(args)
		if err != nil {
			return forensicDirectResult{}, true, err
		}
		if report, ok := raw.(map[string]any); ok {
			report["collection_id"] = toolConfig.CollectionID
			report["template"] = forensicTemplateHint(message)
		}
		argsJSON, _ := json.Marshal(args)
		return forensicDirectResult{
			ToolName: "forensic_get_evidence",
			ArgsJSON: string(argsJSON),
			Text:     formatDirectForensicResult("forensic_get_evidence", text, raw),
			Raw:      raw,
		}, true, nil

	case strings.Contains(normalized, "forensic_list_evidence") ||
		isEvidenceCatalogQuery(normalized):
		args := ForensicListEvidenceArgs{
			Query:  evidenceCatalogSearchHint(message),
			Limit:  50,
			Offset: 0,
		}
		tool := ForensicListEvidenceTool{ForensicRecordsToolConfig: toolConfig}
		text, raw, err := tool.Run(args)
		if err != nil {
			return forensicDirectResult{}, true, err
		}
		if report, ok := raw.(map[string]any); ok {
			report["collection_id"] = toolConfig.CollectionID
			report["template"] = "evidence"
		}
		argsJSON, _ := json.Marshal(args)
		return forensicDirectResult{
			ToolName: "forensic_list_evidence",
			ArgsJSON: string(argsJSON),
			Text:     formatDirectForensicResult("forensic_list_evidence", text, raw),
			Raw:      raw,
		}, true, nil

	case strings.Contains(normalized, "forensic_generate_report") ||
		(strings.Contains(normalized, "report") && strings.Contains(normalized, "forensic")) ||
		isNaturalForensicReportQuery(message, normalized) ||
		strings.Contains(normalized, "case brief") ||
		strings.Contains(normalized, "findings summary"):
		args := ForensicReportArgs{
			Target:          extractForensicTarget(message),
			IncludeEvidence: true,
		}
		tool := ForensicGenerateReportTool{ForensicRecordsToolConfig: toolConfig}
		text, raw, err := tool.Run(args)
		if err != nil {
			return forensicDirectResult{}, true, err
		}
		if report, ok := raw.(map[string]any); ok {
			report["collection_id"] = toolConfig.CollectionID
			report["template"] = forensicTemplateHint(message)
		}
		argsJSON, _ := json.Marshal(args)
		return forensicDirectResult{
			ToolName: "forensic_generate_report",
			ArgsJSON: string(argsJSON),
			Text:     formatDirectForensicResult("forensic_generate_report", text, raw),
			Raw:      raw,
		}, true, nil

	case strings.Contains(normalized, "forensic_hybrid_query") ||
		strings.Contains(normalized, "canonical records") ||
		strings.Contains(normalized, "canonical query") ||
		strings.Contains(normalized, "raw payload") ||
		strings.Contains(normalized, "raw_payload") ||
		strings.Contains(normalized, "call type breakdown") ||
		strings.Contains(normalized, "relationship network") ||
		strings.Contains(normalized, "entity timeline") ||
		strings.Contains(normalized, "entity activity") ||
		strings.Contains(normalized, "data quality") ||
		strings.Contains(normalized, "source records") ||
		strings.Contains(normalized, "exact records analytics") ||
		isNaturalForensicQuery(message, normalized):
		args := buildForensicHybridQueryArgs(message, toolConfig)
		queryToolConfig := toolConfig
		// The generic tool defaults an omitted synthesis model from its config.
		// For the direct route, an empty model is intentional: it is the explicit
		// low-latency exact-answer mode selected above.
		queryToolConfig.Model = args.SynthesisModel
		tool := ForensicHybridQueryTool{ForensicRecordsToolConfig: queryToolConfig}
		text, raw, err := tool.Run(args)
		if err != nil {
			return forensicDirectResult{}, true, err
		}
		argsJSON, _ := json.Marshal(args)
		return forensicDirectResult{
			ToolName: "forensic_hybrid_query",
			ArgsJSON: string(argsJSON),
			Text:     formatDirectForensicResult("forensic_hybrid_query", text, raw),
			Raw:      raw,
		}, true, nil
	}
	return forensicDirectResult{}, false, nil
}

// forensicSynthesisModel adds bounded qualitative explanation to ordinary
// analyst questions after the deterministic result has been computed. Exact
// SQL facts and citations remain independently rendered and authoritative.
// Power users and acceptance tests can explicitly request the low-latency
// deterministic-only path without exposing that implementation detail in the
// normal Agent Chat workflow.
func forensicSynthesisModel(message, model string) string {
	normalized := normalizeForensicRoutingSemantics(message)
	if containsForensicTerm(normalized, []string{"run deterministic", "deterministic only", "model off", "without model"}) {
		return ""
	}
	if containsForensicTerm(normalized, []string{"forensic_query_templates"}) {
		return ""
	}
	return strings.TrimSpace(model)
}

func deterministicForensicRouteRequested(message string) bool {
	normalized := normalizeForensicRoutingSemantics(message)
	// Explanations containing forensic nouns are not proof of a case query.
	// Explicit tool/template requests retain their existing deterministic path.
	if forensictext.IsConceptExplanation(message, explicitForensicTemplate(message) != "" || strings.Contains(normalized, "forensic_") || extractForensicTarget(message) != "") {
		return false
	}
	return explicitForensicTemplate(message) != "" ||
		forensicTemplateHint(message) != "" ||
		strings.Contains(normalized, "forensic_query_templates") ||
		(strings.Contains(normalized, "template") && strings.Contains(normalized, "forensic")) ||
		strings.Contains(normalized, "forensic_generate_report") ||
		(strings.Contains(normalized, "report") && strings.Contains(normalized, "forensic")) ||
		isNaturalForensicReportQuery(message, normalized) ||
		strings.Contains(normalized, "case brief") ||
		strings.Contains(normalized, "findings summary") ||
		strings.Contains(normalized, "forensic_list_evidence") ||
		strings.Contains(normalized, "forensic_get_evidence") ||
		isEvidenceCatalogQuery(normalized) ||
		strings.Contains(normalized, "forensic_hybrid_query") ||
		strings.Contains(normalized, "canonical records") ||
		strings.Contains(normalized, "canonical query") ||
		strings.Contains(normalized, "raw payload") ||
		strings.Contains(normalized, "raw_payload") ||
		strings.Contains(normalized, "call type breakdown") ||
		strings.Contains(normalized, "relationship network") ||
		strings.Contains(normalized, "entity timeline") ||
		strings.Contains(normalized, "entity activity") ||
		strings.Contains(normalized, "data quality") ||
		strings.Contains(normalized, "source records") ||
		strings.Contains(normalized, "exact records analytics") ||
		isNaturalForensicQuery(message, normalized)
}

func isEvidenceCatalogQuery(normalized string) bool {
	return containsForensicTerm(normalized, []string{
		"evidence catalog",
		"evidence inventory",
		"list evidence",
		"show evidence items",
		"what evidence exists",
		"what evidence do we have",
		"uploaded evidence",
		"evidence registry",
		"evidence items",
	})
}

func extractForensicEvidenceID(message string) string {
	return forensicUUIDPattern.FindString(message)
}

func evidenceCatalogSearchHint(message string) string {
	if id := extractForensicEvidenceID(message); id != "" {
		return id
	}
	if target := extractForensicTarget(message); target != "" {
		return target
	}
	return ""
}

func isNaturalForensicQuery(message, normalized string) bool {
	hasTarget := extractForensicTarget(message) != ""
	hasCollectionAnalyticIntent := hasCollectionLevelForensicIntent(normalized)
	hasRecordContext := containsForensicTerm(normalized, []string{
		"forensic", "records", "record", "cdr", "anpr", "ipdr", "subscriber", "tower",
		"transaction", "access log", "access-log", "records-demo", "msisdn", "imei", "imsi",
		"cell", "site", "plate", "camera", "call", "sms", "gprs", "volte", "network", "protocol",
		"ingested", "ingest job", "source file", "source files", "canonical", "raw payload",
		"raw_payload", "jsonb", "batch id", "batch_id", "record type", "record_type",
		"سبسکرائبر", "صارف", "سم", "شناخت",
	}) || (looksLikeForensicANPRPlateTarget(extractForensicTarget(message)) && containsForensicTerm(normalized, []string{"sighting", "where", "found", "when"}))
	hasAnalyticIntent := containsForensicTerm(normalized, []string{
		"shortest", "longest", "duration", "frequent", "contact", "contacts", "breakdown",
		"incoming", "outgoing", "timeline", "chronology", "sequence", "history", "relationship",
		"network", "related", "entity", "entities", "location", "locations", "where", "visited",
		"movement", "geospatial", "tower", "site", "cell", "hourly", "nocturnal", "night",
		"sighting", "where", "found", "when",
		"quality", "duplicate", "duplicates", "rejected", "error", "errors", "failed", "parser",
		"schema", "headers", "columns", "fields", "overview", "batch", "ingest", "status",
		"source row", "source rows", "source record", "source records", "raw row", "raw rows",
		"sighting", "sightings", "evidence", "policy", "citation", "citations", "cite",
		"file", "files", "audit", "summary", "anomaly", "suspicious", "first seen", "last seen",
		"what happened", "happened", "date", "activity", "readiness",
		"show",
		"compare", "comparison", "between", "limitations", "missing data", "data gaps",
		"investigate", "key findings", "important findings",
		"canonical", "raw payload", "raw_payload", "jsonb", "field exists", "field not exists",
		"batch id", "batch_id", "record type", "record_type", "primary target", "secondary target",
		"activation", "deactivation", "validity", "identity lookup", "subscriber status", "identifier reuse", "identity conflict",
		"tafseel", "maloomat", "ikhtilaf", "dobara istemal", "active hua", "band hua", "تفصیل", "حیثیت", "فعالیت", "تضاد", "دوبارہ استعمال",
	})

	return hasCollectionAnalyticIntent || (hasAnalyticIntent && (hasTarget || hasRecordContext || hasDateExpression(message)))
}

func hasCollectionLevelForensicIntent(normalized string) bool {
	return containsForensicTerm(normalized, []string{
		"frequent contacts",
		"top contacts",
		"call type breakdown",
		"data quality",
		"quality issues",
		"which files",
		"source files",
		"ingested files",
		"collection overview",
		"case overview",
		"overview of this case",
		"overview of the case",
		"case summary",
		"case readiness",
		"evidence health",
		"ready for analysis",
		"analysis ready",
		"ready for court",
		"ready for production",
		"summarize this case",
		"suspicious patterns",
		"anomaly summary",
		"generate report",
		"duplicate uploads",
		"known limitations",
		"missing data",
		"available entities",
		"what should i investigate",
		"key findings",
		"service usage",
		"device changes",
		"imei changes",
		"ipdr endpoints",
		"network protocols",
		"camera activity",
		"camera sequence",
		"plate variants",
		"same-camera co-travel",
		"route timing",
		"concurrent sessions",
		"cross-family correlation",
		"cross-family co-presence",
		"imei imsi usage",
		"evidence package",
		"subscriber identity",
		"subscriber validity",
		"subscriber sim and device links",
		"subscriber device links",
		"subscriber status",
		"subscriber conflicts",
		"identifier reuse",
		"tower site lookup",
		"tower reference history",
		"tower coordinate audit",
		"tower status summary",
		"tower alias conflicts",
		"tower cdr join",
		"subscriber ki maloomat",
		"subscriber ki tafseel",
		"subscriber status ka khulasa",
		"سبسکرائبر کی تفصیل",
		"سبسکرائبر حیثیت",
		"سبسکرائبر ریکارڈ میں تضاد",
		"schema profile",
		"detected schema",
		"detected fields",
		"detected headers",
	})
}

func forensicTemplateHint(message string) string {
	if explicit := explicitForensicTemplate(message); explicit != "" {
		return explicit
	}
	normalized := normalizeForensicRoutingSemantics(message)
	if isBoundedForensicComposition(normalized) {
		return ""
	}
	switch {
	case isForensicCanonicalFilterQuery(normalized):
		return "canonical_records"
	case containsForensicTerm(normalized, []string{"canonical records", "canonical query", "canonical sql", "forensic.records", "raw payload", "raw_payload", "jsonb", "record type", "record_type", "batch id", "batch_id", "field exists", "field not exists"}):
		return "canonical_records"
	case containsForensicTerm(normalized, []string{"prepare an executive", "executive case brief", "executive findings brief"}):
		return "executive_case_brief"
	case containsForensicTerm(normalized, []string{"court-ready source", "court ready source", "source and provenance summary"}):
		return "court_ready_source_summary"
	case containsForensicTerm(normalized, []string{"included in the evidence package", "evidence package contents"}):
		return "evidence_package_summary"
	case containsForensicTerm(normalized, []string{"collection overview", "case overview", "overview of this case", "overview of the case", "summarize this case"}):
		return "collection_overview"
	case containsForensicTerm(normalized, []string{"divided by call type", "cdr events by call type"}):
		return "call_type_breakdown"
	case containsForensicTerm(normalized, []string{"services were used", "services used by"}):
		return "service_usage"
	case containsForensicTerm(normalized, []string{"change imei or imsi", "changed imei or imsi"}):
		return "device_identity_changes"
	case containsForensicTerm(normalized, []string{"domains appear in the network", "domains in the network records"}):
		return "ipdr_domain_summary"
	case containsForensicTerm(normalized, []string{"network traffic divided by protocol", "traffic by protocol"}):
		return "ipdr_protocol_breakdown"
	case containsForensicTerm(normalized, []string{"active in the cdr data", "cdr activity over time"}):
		return "temporal_activity"
	case containsForensicTerm(normalized, []string{"supplied locations appear most often", "most frequent supplied locations"}):
		return "top_locations"
	case containsForensicTerm(normalized, []string{"chronological supplied locations", "supplied location chronology"}):
		return "geospatial_movement"
	case containsForensicTerm(normalized, []string{"cameras observed", "cameras saw"}) && containsForensicTerm(normalized, []string{"time order", "chronological order"}):
		return "anpr_camera_sequence"
	case containsForensicTerm(normalized, []string{"anpr cameras have the most", "busiest anpr cameras"}):
		return "anpr_camera_activity"
	case containsForensicTerm(normalized, []string{"time gaps between consecutive sightings", "gaps between consecutive sightings"}):
		return "anpr_route_timing"
	case containsForensicTerm(normalized, []string{"subscriber observations exist", "subscriber observations for"}):
		return "subscriber_identity_lookup"
	case containsForensicTerm(normalized, []string{"when was subscriber", "subscriber valid", "subscriber validity period"}):
		return "subscriber_validity_timeline"
	case containsForensicTerm(normalized, []string{"sim and device identifiers are linked", "sim and device identifiers linked"}):
		return "subscriber_device_links"
	case containsForensicTerm(normalized, []string{"subscriber records are active", "active, inactive or suspended", "active inactive or suspended"}):
		return "subscriber_status_summary"
	case containsForensicTerm(normalized, []string{"subscriber identifiers appear with multiple", "subscriber identifiers with multiple phone"}):
		return "subscriber_reuse_candidates"
	case containsForensicTerm(normalized, []string{"what activity exists for", "activity exists for"}):
		return "entity_activity"
	case containsForensicTerm(normalized, []string{"evidence-backed relationships", "evidence backed relationships"}):
		return "relationship_network"
	case containsForensicTerm(normalized, []string{"available evidence families", "all evidence families"}) && containsForensicTerm(normalized, []string{"correlate", "correlation"}):
		return "cross_family_correlation"
	case containsForensicTerm(normalized, []string{"happened involving", "involving"}) && containsForensicTerm(normalized, []string{"over time", "timeline"}):
		return "entity_timeline"
	case containsForensicTerm(normalized, []string{"normalized cdr records", "normalized records"}):
		return "canonical_records"
	case containsForensicTerm(normalized, []string{"both the shortest and longest", "shortest and longest calls"}):
		return "duration_extremes"
	case containsForensicTerm(normalized, []string{"night-time activity", "nighttime activity"}):
		return "night_activity"
	case containsForensicTerm(normalized, []string{"supplied locations recur", "recurring supplied locations"}):
		return "repeated_location_visits"
	case containsForensicTerm(normalized, []string{"cross-family co-presence", "cross family co-presence"}):
		return "co_travel_or_co_presence"
	case containsForensicTerm(normalized, []string{"identifiers does the cdr show", "cdr identifiers for"}):
		return "subscriber_profile"
	case containsForensicTerm(normalized, []string{"cdr cells or sites observed", "cdr cell or site activity"}):
		return "tower_activity"
	case containsForensicTerm(normalized, []string{"deterministic patterns", "patterns should an analyst review"}):
		return "suspicious_patterns"
	case containsForensicTerm(normalized, []string{"deterministic anomalies", "anomaly summary"}):
		return "anomaly_summary"
	case containsForensicTerm(normalized, []string{"across datasets", "across data sets"}) && containsForensicTerm(normalized, []string{"summarize", "summary"}):
		return "cross_dataset_entity_summary"
	case containsForensicTerm(normalized, []string{"uploaded more than once", "same source files more than once"}):
		return "duplicate_upload_audit"
	case containsForensicTerm(normalized, []string{"ready for analyst demonstration", "ready for demonstration"}):
		return "case_readiness"
	case containsForensicTerm(normalized, []string{"reference facts exist for tower", "reference facts for tower", "reference facts for site"}):
		return "tower_site_lookup"
	case containsForensicTerm(normalized, []string{"reference history for", "tower history for", "site history for"}):
		return "tower_reference_timeline"
	case containsForensicTerm(normalized, []string{"tower coordinates, datums", "tower coordinates datums", "coordinates, datums or uncertainty"}):
		return "tower_coordinate_audit"
	case containsForensicTerm(normalized, []string{"tower references divided by status", "tower references by status and technology"}):
		return "tower_status_summary"
	case containsForensicTerm(normalized, []string{"tower aliases associated with conflicting", "tower aliases with conflicting"}):
		return "tower_alias_conflicts"
	case containsForensicTerm(normalized, []string{"join cdr observations to the valid tower", "join cdr observations to tower reference"}):
		return "tower_cdr_join"
	case containsForensicTerm(normalized, []string{"transaction summary", "financial summary", "transactions by currency", "transaction totals", "len den ka khulasa", "لین دین کا خلاصہ", "کرنسی کے حساب سے لین دین"}):
		return "financial_transaction_summary"
	case containsForensicTerm(normalized, []string{"failed access", "failed login", "denied access", "access failures", "nakam rasai", "login fail", "ناکام رسائی", "ناکام لاگ ان"}):
		return "access_failed_events"
	case containsForensicTerm(normalized, []string{"generic records", "generic structured rows", "generic data rows", "aam structured records", "عمومی ریکارڈز"}):
		return "generic_filter_records"
	case containsForensicTerm(normalized, []string{"document metadata", "document processing status", "registered documents", "document ki tafseel", "دستاویز کی تفصیل", "رجسٹرڈ دستاویزات"}):
		return "document_metadata"
	case containsForensicTerm(normalized, []string{"search documents", "find in documents", "document passages", "documents mein dhoondo", "dastavez mein dhoondo", "دستاویز میں تلاش", "دستاویزات میں ڈھونڈو"}):
		return "document_search"
	case containsForensicTerm(normalized, []string{"image metadata", "image processing status", "registered images", "tasveer ki tafseel", "تصویر کی تفصیل", "رجسٹرڈ تصاویر"}):
		return "image_metadata"
	case containsForensicTerm(normalized, []string{"search image ocr", "find ocr text", "ocr observations", "image text search", "exact recognized text", "recognized text in retained derived observations", "tasveer ka matn", "تصویر کا متن", "او سی آر تلاش"}):
		return "image_ocr_search"
	case containsForensicTerm(normalized, []string{"compare these two face candidates", "find candidate faces similar", "candidate faces visually similar", "faces visually similar", "closest face candidate", "similar face candidates", "face similarity"}):
		return "face_candidate_observations"
	case containsForensicTerm(normalized, []string{"find images similar", "similar images", "visually similar images", "images look most similar", "visual similarity"}):
		return "image_metadata"
	case containsForensicTerm(normalized, []string{"face candidate observations", "face candidates", "chehray ke candidates", "chehre ke candidates", "چہرے کے امیدوار", "تصویر میں چہرے"}):
		return "face_candidate_observations"
	case containsForensicTerm(normalized, []string{"audio metadata", "audio processing status", "registered audio", "audio ki tafseel", "آڈیو کی تفصیل", "رجسٹرڈ آڈیو"}):
		return "audio_metadata"
	case containsForensicTerm(normalized, []string{"search transcript", "find in transcript", "audio transcript", "roman urdu transcript", "transcript mein dhoondo", "آڈیو متن میں تلاش", "ٹرانسکرپٹ میں تلاش"}):
		return "audio_transcript_search"
	case containsForensicTerm(normalized, []string{"video metadata", "video processing status", "registered videos", "video ki tafseel", "ویڈیو کی تفصیل", "رجسٹرڈ ویڈیوز"}):
		return "video_metadata"
	case containsForensicTerm(normalized, []string{"video observation timeline", "video timeline", "sampled video observations", "video ka timeline", "ویڈیو ٹائم لائن", "ویڈیو مشاہدات"}):
		return "video_timeline"
	case containsForensicTerm(normalized, []string{"which files", "source file", "source files", "file audit", "ingested files", "files ingested"}):
		return "source_file_audit"
	case containsForensicTerm(normalized, []string{"duplicate upload", "duplicate uploads", "duplicate file", "duplicate files"}):
		return "duplicate_upload_audit"
	case containsForensicTerm(normalized, []string{"cross-family correlation", "cross family correlation", "correlate across families", "correlate across record families"}):
		return "cross_family_correlation"
	case containsForensicTerm(normalized, []string{"cross-dataset entity summary", "cross dataset entity summary", "entity across datasets", "entity across data sets"}):
		return "cross_dataset_entity_summary"
	case containsForensicTerm(normalized, []string{"cross-family co-presence", "cross family co-presence", "co-presence across families", "co presence across families"}):
		return "co_travel_or_co_presence"
	case containsForensicTerm(normalized, []string{"limitations", "known limitations", "missing data", "data gaps", "parser limitations"}):
		return "limitations_and_data_quality"
	case containsForensicTerm(normalized, []string{"source records", "source rows", "raw source rows"}):
		return "source_records"
	case containsForensicTerm(normalized, []string{"evidence lineage", "evidence provenance", "evidence details"}):
		return "evidence"
	case containsForensicTerm(normalized, []string{"ipdr concurrent", "concurrent session", "overlapping session", "session overlap"}):
		return "ipdr_concurrent_sessions"
	case containsForensicTerm(normalized, []string{"ipdr timeline", "network timeline", "session timeline"}):
		return "ipdr_timeline"
	case containsForensicTerm(normalized, []string{"subscriber session", "subscriber network session"}):
		return "ipdr_subscriber_sessions"
	case containsForensicTerm(normalized, []string{"session volume", "network volume", "byte volume", "traffic volume"}):
		return "ipdr_session_volume"
	case containsForensicTerm(normalized, []string{"protocol breakdown", "network protocol"}):
		return "ipdr_protocol_breakdown"
	case containsForensicTerm(normalized, []string{"domain summary", "network domains", "ipdr domains"}):
		return "ipdr_domain_summary"
	case containsForensicTerm(normalized, []string{"endpoint summary", "endpoint activity", "ipdr endpoint activity", "network endpoints", "ipdr endpoints"}):
		return "ipdr_endpoint_summary"
	case containsForensicTerm(normalized, []string{"tower cdr join", "cdr tower join", "cell reference join", "site reference join", "time-aware tower join", "time aware tower join"}):
		return "tower_cdr_join"
	case containsForensicTerm(normalized, []string{"tower alias conflict", "tower alias conflicts", "site alias conflict", "conflicting tower reference", "conflicting site reference"}):
		return "tower_alias_conflicts"
	case containsForensicTerm(normalized, []string{"tower status", "site status summary", "tower technology summary", "site technology summary"}):
		return "tower_status_summary"
	case containsForensicTerm(normalized, []string{"tower coordinate audit", "site coordinate audit", "audit tower coordinate", "audit site coordinate", "tower datum", "site datum", "tower uncertainty", "site uncertainty"}):
		return "tower_coordinate_audit"
	case containsForensicTerm(normalized, []string{"tower reference timeline", "site reference timeline", "tower reference history", "site reference history"}):
		return "tower_reference_timeline"
	case containsForensicTerm(normalized, []string{"tower site lookup", "look up tower site", "look up tower/site", "lookup tower site", "tower/site reference", "tower lookup", "site lookup", "cell site lookup", "tower reference lookup"}):
		return "tower_site_lookup"
	case containsForensicTerm(normalized, []string{"co-travel", "co travel", "same-camera co-observation", "same camera co-observation"}):
		return "anpr_co_travel"
	case containsForensicTerm(normalized, []string{"route timing", "consecutive sighting timing"}):
		return "anpr_route_timing"
	case containsForensicTerm(normalized, []string{"plate variant", "plate variants", "ocr variant"}):
		return "anpr_plate_variants"
	case containsForensicTerm(normalized, []string{"camera activity", "activity by camera"}):
		return "anpr_camera_activity"
	case containsForensicTerm(normalized, []string{"camera sequence"}):
		return "anpr_camera_sequence"
	case containsForensicTerm(normalized, []string{"grouped video anpr", "video anpr groups", "plates appear in this video", "plates in this video", "plate groups in video"}):
		return "video_anpr_grouped_timeline"
	case containsForensicTerm(normalized, []string{"video"}) && containsForensicTerm(normalized, []string{"plate", "plates", "anpr"}):
		return "video_anpr_grouped_timeline"
	case containsForensicTerm(normalized, []string{"anpr timeline", "plate timeline"}):
		return "anpr_timeline"
	case containsForensicTerm(normalized, []string{"image"}) && containsForensicTerm(normalized, []string{"plate", "plates", "anpr"}):
		return "anpr_sightings"
	case looksLikeForensicANPRPlateTarget(extractForensicTarget(message)) && containsForensicTerm(normalized, []string{"sighting", "seen", "where", "found", "when"}):
		return "anpr_sightings"
	case containsForensicTerm(normalized, []string{"plate sighting", "plate sightings", "anpr sighting", "anpr sightings"}):
		return "anpr_sightings"
	case containsForensicTerm(normalized, []string{"plate", "anpr"}) && containsForensicTerm(normalized, []string{"sighting", "seen", "show", "when"}):
		return "anpr_sightings"
	case containsForensicTerm(normalized, []string{"cdr subscriber profile", "cdr-observed subscriber profile", "cdr observed subscriber profile"}):
		return "subscriber_profile"
	case containsForensicTerm(normalized, []string{"imei imsi usage", "imei/imsi usage", "device and sim usage", "sim and device usage"}):
		return "imei_imsi_usage"
	case containsForensicTerm(normalized, []string{"subscriber reuse", "identifier reuse", "sim reuse", "reused imsi", "reused imei", "reassigned number", "sim dobara istemal", "شناختی نمبر کا دوبارہ استعمال"}):
		return "subscriber_reuse_candidates"
	case containsForensicTerm(normalized, []string{"subscriber conflict", "subscriber conflicts", "identity conflict", "identity conflicts", "conflicting subscriber", "alias conflict", "subscriber record mein ikhtilaf", "سبسکرائبر ریکارڈ میں تضاد"}):
		return "subscriber_conflict_audit"
	case containsForensicTerm(normalized, []string{"subscriber status", "subscriber statuses", "active subscribers", "inactive subscribers", "suspended subscribers", "subscriber status ka khulasa", "سبسکرائبر حیثیت", "فعال سبسکرائبرز", "معطل سبسکرائبرز"}):
		return "subscriber_status_summary"
	case containsForensicTerm(normalized, []string{"subscriber device links", "subscriber device link", "subscriber sim links", "subscriber sim and device links", "explicit sim device", "subscriber imsi imei", "sim aur device links", "imei imsi rabta", "سم اور ڈیوائس لنکس"}):
		return "subscriber_device_links"
	case containsForensicTerm(normalized, []string{"subscriber validity", "subscriber activation", "subscriber deactivation", "subscription validity", "subscriber timeline", "activation aur deactivation", "kab active hua", "kab band hua", "فعالیت کی مدت"}):
		return "subscriber_validity_timeline"
	case containsForensicTerm(normalized, []string{"subscriber identity", "subscriber lookup", "subscriber profile", "look up subscriber", "lookup subscriber", "subscriber ki maloomat", "subscriber ki tafseel", "سبسکرائبر کی تفصیل", "سبسکرائبر شناخت", "صارف کی تفصیل"}):
		return "subscriber_identity_lookup"
	case containsForensicTerm(normalized, []string{"device change", "device changes", "imei change", "imei changes", "imsi change", "imsi changes", "sim swap", "handset change"}):
		return "device_identity_changes"
	case containsForensicTerm(normalized, []string{"service usage", "ussd", "packet data", "service classification"}):
		return "service_usage"
	case containsForensicTerm(normalized, []string{"available entities", "entity inventory", "known entities"}):
		return "entity_activity"
	case containsForensicTerm(normalized, []string{"entity activity", "activity for entity", "activity for this entity"}):
		return "entity_activity"
	case containsForensicTerm(normalized, []string{"relationship network", "relationship graph", "connections for entity"}):
		return "relationship_network"
	case isForensicComparisonQuery(normalized):
		return "relationship_network"
	case containsForensicTerm(normalized, []string{"shortest call"}):
		return "shortest_call"
	case containsForensicTerm(normalized, []string{"longest call"}):
		return "longest_call"
	case containsForensicTerm(normalized, []string{"duration", "shortest", "longest"}):
		return "duration_extremes"
	case containsForensicTerm(normalized, []string{"call type", "call types", "breakdown"}):
		return "call_type_breakdown"
	case containsForensicTerm(normalized, []string{"frequent contact", "frequent contacts", "top contacts"}):
		return "frequent_contacts"
	case containsForensicTerm(normalized, []string{"temporal cdr activity", "cdr temporal activity", "cdr actvty", "communications activity over time"}):
		return "temporal_activity"
	case containsForensicTerm(normalized, []string{"data quality", "quality issue", "quality issues", "duplicate", "duplicates", "rejected"}):
		return "data_quality"
	case containsForensicTerm(normalized, []string{"case readiness", "readiness", "evidence health", "ready for analysis", "analysis ready", "ready to analyze", "ready for court", "ready for production", "production ready", "can we trust", "reliable enough"}):
		return "case_readiness"
	case containsForensicTerm(normalized, []string{"first seen", "last seen", "first_seen", "last_seen", "first and last observed", "first and last observation"}):
		return "first_seen_last_seen"
	case hasDateExpression(message) && containsForensicTerm(normalized, []string{"what happened", "happened", "timeline", "events", "activity on", "date"}):
		return "entity_timeline"
	case containsForensicTerm(normalized, []string{"activity by day", "daily activity"}):
		return "activity_by_day"
	case containsForensicTerm(normalized, []string{"activity by hour", "hourly activity"}):
		return "activity_by_hour"
	case containsForensicTerm(normalized, []string{"night activity", "nocturnal"}):
		return "night_activity"
	case containsForensicTerm(normalized, []string{"repeated location visits", "repeat location visits", "locations visited repeatedly"}):
		return "repeated_location_visits"
	case containsForensicTerm(normalized, []string{"top location", "top locations", "most often observed", "where was"}):
		return "top_locations"
	case containsForensicTerm(normalized, []string{"movement", "geospatial", "base location", "location timeline"}):
		return "geospatial_movement"
	case containsForensicTerm(normalized, []string{"tower activity", "cell site", "cell sites"}):
		return "tower_activity"
	case containsForensicTerm(normalized, []string{"collection status", "collection overview", "ingest status"}):
		return "collection_overview"
	case containsForensicTerm(normalized, []string{"schema", "headers", "columns", "detected fields"}):
		return "schema_profile"
	case containsForensicTerm(normalized, []string{"evidence package"}):
		return "evidence_package_summary"
	case containsForensicTerm(normalized, []string{"executive brief", "case brief"}):
		return "executive_case_brief"
	case containsForensicTerm(normalized, []string{"court ready", "court-ready"}):
		return "court_ready_source_summary"
	case containsForensicTerm(normalized, []string{"anomaly summary", "summarize anomalies"}):
		return "anomaly_summary"
	case containsForensicTerm(normalized, []string{"suspicious", "anomalies", "investigate next", "what should i investigate", "key findings", "important findings"}):
		return "suspicious_patterns"
	default:
		return ""
	}
}

func explicitForensicTemplate(message string) string {
	match := forensicTemplateAssignmentPattern.FindStringSubmatch(message)
	if len(match) != 2 {
		return ""
	}
	return strings.ToLower(match[1])
}

func isForensicCanonicalFilterQuery(normalized string) bool {
	hasRecordContext := containsForensicTerm(normalized, []string{
		"records", "record", "rows", "row", "cdr", "anpr", "ipdr", "subscriber",
		"tower", "transaction", "access log", "generic",
	})
	if !hasRecordContext {
		return false
	}
	if containsForensicTerm(normalized, []string{"field exists", "field not exists", "source file", "batch id", "raw payload", "raw_payload"}) {
		return true
	}
	return regexp.MustCompile(`\bwhere\b.+(?:\b(?:is|is not|equals?|equal to|not equal|contains|like|in|exists|missing|greater than|less than|at least|at most)\b|>=|<=|>|<|!=|<>)(?:\s|$)`).MatchString(normalized)
}

func isForensicComparisonQuery(normalized string) bool {
	return containsForensicTerm(normalized, []string{
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

func isNaturalForensicReportQuery(message, normalized string) bool {
	hasReportIntent := regexp.MustCompile(`\b(?:report|brief|briefing)\b`).MatchString(normalized) ||
		containsForensicTerm(normalized, []string{"case package", "findings summary", "intelligence summary"})
	if !hasReportIntent {
		return false
	}
	if containsForensicTerm(normalized, []string{"generate report", "generate a report", "case report", "executive brief", "case brief"}) {
		return true
	}
	hasTarget := extractForensicTarget(message) != ""
	hasRecordContext := containsForensicTerm(normalized, []string{
		"forensic", "records", "record", "cdr", "anpr", "ipdr", "subscriber", "tower",
		"transaction", "access log", "access-log", "records-demo", "msisdn", "imei", "imsi",
		"cell", "site", "plate", "camera", "call", "sms", "gprs", "volte", "evidence",
	})
	return hasTarget || hasRecordContext
}

func containsForensicTerm(value string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

// normalizeForensicRoutingSemantics translates only generic analyst-intent
// vocabulary. Target extraction continues to run on the original message so
// ASCII plates, UUIDs, phone numbers, and dates remain byte-accurate.
func normalizeForensicRoutingSemantics(message string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(message)), " "))
	replacements := []struct {
		pattern *regexp.Regexp
		value   string
	}{
		{regexp.MustCompile(`(?:ویڈیو|وڈیو)`), "video"},
		{regexp.MustCompile(`نمبر\s+پلیٹ(?:یں)?`), "plate"},
		{regexp.MustCompile(`گاڑی\s+کی\s+پلیٹ(?:یں)?`), "plate"},
		{regexp.MustCompile(`نظر\s+آ(?:ئی|یا|ئیں|ئے)`), "sighting"},
		{regexp.MustCompile(`کہاں`), "where"},
		{regexp.MustCompile(`(?:ملی|ملا|ملیں|ملے)`), "found"},
		{regexp.MustCompile(`تصویر`), "image"},
		{regexp.MustCompile(`کب`), "when"},
		{regexp.MustCompile(`(?:دکھائیں|دکھاؤ|بتائیں|بتاؤ)`), "show"},
	}
	for _, replacement := range replacements {
		normalized = replacement.pattern.ReplaceAllString(normalized, replacement.value)
	}
	return strings.Join(strings.Fields(normalized), " ")
}

func looksLikeForensicANPRPlateTarget(value string) bool {
	compact := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(value)))
	return regexp.MustCompile(`^[A-Z]{1,4}[0-9]{1,6}[A-Z]{0,3}$`).MatchString(compact)
}

func forensicVideoEvidenceID(message string) string {
	if forensicTemplateHint(message) != "video_anpr_grouped_timeline" {
		return ""
	}
	return extractForensicEvidenceID(message)
}

func extractForensicTarget(message string) string {
	// Evidence, case, batch, and document UUIDs are typed identifiers. Remove
	// their complete spans and typed source-second ranges before scanning for
	// phone, plate, IP, email, or other analytical targets so neither an
	// internal UUID group nor a range connector can become a plate.
	message = forensicUUIDPattern.ReplaceAllString(message, " ")
	for _, pattern := range forensicSourceSecondRangePatterns {
		message = pattern.ReplaceAllString(message, " ")
	}
	for _, pattern := range forensicTargetPatterns {
		for _, match := range pattern.FindAllString(message, -1) {
			if target := normalizeExtractedForensicTarget(match); target != "" {
				return target
			}
		}
	}
	return ""
}

func buildForensicHybridQueryArgs(message string, toolConfig ForensicRecordsToolConfig) ForensicHybridQueryArgs {
	literalIntent, literalFamily := forensictext.Extract(message)
	startSeconds, endSeconds := extractForensicSourceSecondRange(message)
	template := forensicTemplateHint(message)
	if literalIntent != nil {
		template = forensicTemplateHint(forensictext.PlanningText(message))
	}
	if literalIntent != nil && (template == "" || template == "audio_transcript_search" || template == "image_ocr_search" || template == "document_search") {
		if candidate := forensictext.Template(literalFamily); candidate != "" {
			template = candidate
		}
	} else {
		literalIntent = nil
	}
	target := extractForensicTarget(message)
	if template == "" && looksLikeForensicANPRPlateTarget(target) && containsForensicTerm(normalizeForensicRoutingSemantics(message), []string{"plate", "anpr"}) {
		template = "anpr_sightings"
	}
	evidenceID, versionID := "", ""
	queryScope := toolConfig.QueryScope
	if queryScope != nil && forensicExplicitWorkspaceScope(message) {
		copy := *queryScope
		copy.Kind, copy.EvidenceID, copy.EvidenceVersionID = "current_workspace", "", ""
		copy.SourceFamily, copy.AvailableResultFamilies = "", nil
		queryScope = &copy
	}
	if scope := queryScope; scope != nil && strings.TrimSpace(scope.Kind) == "selected_evidence" {
		evidenceID = strings.ToLower(strings.TrimSpace(scope.EvidenceID))
		versionID = strings.ToLower(strings.TrimSpace(scope.EvidenceVersionID))
		if (strings.EqualFold(strings.TrimSpace(scope.SourceFamily), "audio") || containsTranscriptResultFamily(scope.AvailableResultFamilies)) &&
			(template == "" || template == "audio_transcript_search") {
			template = "audio_transcript_search"
		}
		if strings.EqualFold(strings.TrimSpace(scope.SourceFamily), "video") &&
			containsForensicResultFamily(scope.AvailableResultFamilies, "forensics.video-anpr-plate-group/v1") &&
			template == "anpr_sightings" {
			template = "video_anpr_grouped_timeline"
		}
	}
	if literalIntent != nil && template == "" && queryScope != nil && queryScope.Kind == "selected_evidence" {
		template = forensictext.Template(queryScope.SourceFamily)
	}
	transcriptMode, exactTerm := "", ""
	if template == "audio_transcript_search" {
		transcriptMode, exactTerm = forensicTranscriptQuery(message, startSeconds, endSeconds)
	} else if template == "image_ocr_search" {
		_, exactTerm = forensicTranscriptQuery(message, nil, nil)
	}
	args := ForensicHybridQueryArgs{
		TextQuery:           literalIntent,
		Query:               message,
		Target:              forensicFirstNonEmpty(exactTerm, target),
		Template:            template,
		Limit:               20,
		MaxKBResults:        3,
		SynthesisModel:      forensicSynthesisModel(message, toolConfig.Model),
		ConversationContext: toolConfig.ConversationContext,
		EvidenceID:          forensicFirstNonEmpty(evidenceID, forensicVideoEvidenceID(message)),
		EvidenceVersionID:   versionID,
		QueryScope:          queryScope,
		TranscriptMode:      transcriptMode,
		ExactTerm:           exactTerm,
		QueryLanguage:       forensicQueryLanguage(message),
		StartSeconds:        startSeconds,
		EndSeconds:          endSeconds,
	}
	if args.Template == "video_anpr_grouped_timeline" {
		args.Plate = args.Target
		args.Target = args.EvidenceID
	}
	return args
}

func forensicFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func forensicExplicitWorkspaceScope(message string) bool {
	return forensictext.ExplicitScope(message) == "authorized_workspace"
}

func containsTranscriptResultFamily(families []string) bool {
	for _, family := range families {
		if family == "forensics.audio-timestamp-segment/v1" || family == "forensics.audio-roman-urdu-segment/v1" {
			return true
		}
	}
	return false
}

func containsForensicResultFamily(families []string, wanted string) bool {
	for _, family := range families {
		if strings.EqualFold(strings.TrimSpace(family), wanted) {
			return true
		}
	}
	return false
}

var forensicQuotedPhrase = regexp.MustCompile(`["“”'‘’]([^"“”'‘’]{1,256})["“”'‘’]`)
var forensicExactTranscriptPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:find|search)(?:\s+(?:this|the))?(?:\s+(?:transcript|recording|audio))?(?:\s+(?:phrase|term))?\s*[:=\-]\s*(.{1,256}?)\s*[?.!]*$`),
	regexp.MustCompile(`(?i)(?:did\s+(?:they|the\s+speaker|he|she)|does\s+(?:the\s+recording|this\s+audio))\s+(?:say|mention)\s+(.{1,256}?)\s*[?.!]*$`),
	regexp.MustCompile(`(?i)was\s+(.{1,256}?)\s+(?:said|mentioned)(?:\s+in\s+(?:this\s+)?(?:audio|recording))?\s*[?.!]*$`),
	regexp.MustCompile(`کیا\s+(.{1,256}?)\s+کا\s+ذکر\s+ہوا`),
	regexp.MustCompile(`(?i)^(.{1,256}?)\s+ka\s+zikr\s+hua`),
}

func forensicTranscriptQuery(message string, startSeconds, endSeconds *float64) (string, string) {
	if startSeconds != nil || endSeconds != nil {
		return "time_range", ""
	}
	if forensicTranscriptTimeAnaphor(message) {
		return "source_time", ""
	}
	if match := forensicQuotedPhrase.FindStringSubmatch(message); len(match) == 2 {
		return "exact", strings.TrimSpace(match[1])
	}
	for _, pattern := range forensicExactTranscriptPatterns {
		if match := pattern.FindStringSubmatch(strings.TrimSpace(message)); len(match) == 2 {
			return "exact", strings.TrimSpace(match[1])
		}
	}
	return "source", ""
}

func forensicTranscriptTimeAnaphor(message string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(message)), " "))
	return regexp.MustCompile(`\bwhen\s+did\s+(?:they|he|she|the\s+speaker)\s+say\s+(?:that|it)\b`).MatchString(normalized) ||
		regexp.MustCompile(`(?:انہوں\s+نے|اس\s+نے).*?(?:یہ|وہ).*?کب.*?کہا`).MatchString(normalized)
}

func forensicQueryLanguage(message string) string {
	hasUrdu := regexp.MustCompile(`[\x{0600}-\x{06ff}]`).MatchString(message)
	hasLatin := regexp.MustCompile(`[A-Za-z]`).MatchString(message)
	switch {
	case hasUrdu && hasLatin:
		return "mixed"
	case hasUrdu:
		return "ur"
	case hasLatin && containsForensicTerm(strings.ToLower(message), []string{" kya ", " ka ", " mein ", " dikhao", " batao", " hua"}):
		return "roman_urdu"
	default:
		return "en"
	}
}

func extractForensicSourceSecondRange(message string) (*float64, *float64) {
	for _, pattern := range forensicSourceSecondRangePatterns {
		match := pattern.FindStringSubmatch(message)
		if len(match) != 3 {
			continue
		}
		start, startErr := strconv.ParseFloat(match[1], 64)
		end, endErr := strconv.ParseFloat(match[2], 64)
		if startErr == nil && endErr == nil && start >= 0 && end >= start {
			return &start, &end
		}
	}
	return nil, nil
}

func isBoundedForensicComposition(normalized string) bool {
	hasSubscriber := containsForensicTerm(normalized, []string{"subscriber", "sim", "imsi", "iccid", "device association"})
	hasCDR := containsForensicTerm(normalized, []string{"cdr activity", "call activity", "temporal activity", "hourly calls", "frequent contacts", "contact most", "top contacts"})
	return hasSubscriber && hasCDR
}

func normalizeExtractedForensicTarget(match string) string {
	target := strings.TrimSpace(match)
	if target == "" || isIgnoredForensicTarget(target) || isForensicDateLikeTarget(target) {
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
	target = strings.ToUpper(strings.ReplaceAll(target, " ", "-"))
	return target
}

func isIgnoredForensicTarget(target string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(target, "_", "-"), " ", "-"))
	if regexp.MustCompile(`^(?:on|in|at|by|from|to|for)-?\d{1,6}$`).MatchString(normalized) {
		return true
	}
	switch normalized {
	case "records-demo", "forensic-records", "local-ai", "localai", "call", "calls", "data", "record", "records":
		return true
	default:
		return false
	}
}

func hasDateExpression(message string) bool {
	return regexp.MustCompile(`\b\d{4}[-/]\d{1,2}[-/]\d{1,2}\b|\b\d{1,2}[-/]\d{1,2}[-/]\d{4}\b|\b\d{8}\b`).FindString(message) != ""
}

func isForensicDateLikeTarget(target string) bool {
	value := strings.TrimSpace(target)
	if value == "" {
		return false
	}
	if regexp.MustCompile(`(?i)^(?:jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)[-\s]+\d{4}$`).MatchString(value) {
		return true
	}
	if parseForensicDateExpression(value) {
		return true
	}
	digits := regexp.MustCompile(`\D`).ReplaceAllString(value, "")
	return len(digits) == 8 && parseForensicDateExpression(digits)
}

func parseForensicDateExpression(value string) bool {
	for _, layout := range []string{"2006-01-02", "2006/01/02", "02-01-2006", "02/01/2006", "20060102"} {
		if _, err := time.ParseInLocation(layout, strings.TrimSpace(value), time.UTC); err == nil {
			return true
		}
	}
	return false
}

func formatDirectForensicResult(toolName, text string, raw any) string {
	if report, ok := raw.(map[string]any); ok {
		if markdown, ok := report["markdown"].(string); ok && strings.TrimSpace(markdown) != "" && toolName != "forensic_generate_report" {
			return markdown
		}
		switch toolName {
		case "forensic_hybrid_query":
			if summary := formatForensicHybridSummary(report); summary != "" {
				return summary
			}
		case "forensic_query_templates":
			if summary := formatForensicTemplatesSummary(report); summary != "" {
				return summary
			}
		case "forensic_list_evidence":
			if summary := formatForensicEvidenceListSummary(report); summary != "" {
				return summary
			}
		case "forensic_get_evidence":
			if summary := formatForensicEvidenceDetailSummary(report); summary != "" {
				return summary
			}
		case "forensic_generate_report":
			if summary := formatForensicReportSummary(report); summary != "" {
				return summary
			}
		}
	}
	if strings.TrimSpace(text) == "" {
		text = fmt.Sprint(raw)
	}
	return fmt.Sprintf("Deterministic forensic result from `%s` completed, but the response did not include an analyst summary.", toolName)
}

func formatForensicEvidenceListSummary(resp map[string]any) string {
	items := mapSlice(resp["items"])
	var b strings.Builder
	b.WriteString("## Evidence inventory\n\n")
	if summary, ok := resp["summary"].(map[string]any); ok {
		b.WriteString("### At a glance\n\n")
		writeFact(&b, "Evidence items", stringValue(summary["evidence_total"]))
		writeFact(&b, "Ready", stringValue(firstPresent(summary, "completed", "evidence_completed")))
		writeFact(&b, "Failed", stringValue(firstPresent(summary, "failed", "evidence_failed")))
	}
	if len(items) == 0 {
		b.WriteString("\nNo evidence items matched this governed collection and filter. No source, count, or provenance claim was inferred.\n")
		writeWarnings(&b, resp["warnings"])
		return strings.TrimSpace(b.String())
	}
	b.WriteString("\n### Sources\n\n| Source | Evidence type | Processing status |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, item := range items {
		b.WriteString(fmt.Sprintf(
			"| %s | %s | %s |\n",
			markdownCell(stringValue(firstPresent(item, "source_file", "original_filename"))),
			markdownCell(stringValue(firstPresent(item, "detected_type", "modality"))),
			markdownCell(stringValue(item["processing_status"])),
		))
	}
	b.WriteString("\n<details>\n<summary>Technical identifiers and processing routes</summary>\n\n")
	writeFact(&b, "Case", stringValue(resp["collection_id"]))
	for _, item := range items {
		b.WriteString("- `")
		b.WriteString(stringValue(item["evidence_id"]))
		b.WriteString("` — ")
		b.WriteString(stringValue(firstPresent(item, "source_file", "original_filename")))
		if route := stringValue(item["processing_route"]); route != "" {
			b.WriteString(" (")
			b.WriteString(route)
			b.WriteString(")")
		}
		b.WriteString("\n")
	}
	b.WriteString("\n</details>")
	return b.String()
}

func formatForensicEvidenceDetailSummary(resp map[string]any) string {
	item, ok := resp["item"].(map[string]any)
	if !ok || len(item) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Evidence details\n\n")
	writeFact(&b, "Source", stringValue(firstPresent(item, "source_file", "original_filename")))
	writeFact(&b, "Evidence type", stringValue(item["detected_type"]))
	writeFact(&b, "Processing status", stringValue(item["processing_status"]))
	if jobs := mapSlice(resp["ingest_jobs"]); len(jobs) > 0 {
		b.WriteString("\n**Linked ingest jobs**\n\n")
		writeMarkdownTable(&b, "jobs", jobs, 5)
	}
	if rows := mapSlice(resp["records_preview"]); len(rows) > 0 {
		b.WriteString("\n**Canonical row preview**\n\n")
		writeMarkdownTable(&b, "canonical_records", rows, 5)
	}
	if rollup := mapSlice(resp["entity_rollup"]); len(rollup) > 0 {
		b.WriteString("\n**Entity rollup**\n\n")
		writeMarkdownTable(&b, "entity_activity", rollup, 5)
	}
	b.WriteString("\n<details>\n<summary>Technical identifiers</summary>\n\n")
	writeFact(&b, "Evidence ID", stringValue(item["evidence_id"]))
	writeFact(&b, "Processing route", stringValue(item["processing_route"]))
	writeFact(&b, "Knowledge Base entry", stringValue(item["kb_entry_ref"]))
	writeFact(&b, "Records batch", stringValue(item["records_batch_id"]))
	b.WriteString("\n</details>")
	return b.String()
}

func formatForensicHybridSummary(resp map[string]any) string {
	if enterprise, ok := resp["enterprise"].(map[string]any); ok {
		if summary := formatEnterpriseForensicSummary(resp, enterprise); summary != "" {
			return summary
		}
	}
	var b strings.Builder
	if answer, ok := resp["answer"].(map[string]any); ok {
		if stringValue(answer["clarification_required"]) == "true" {
			b.WriteString("**Clarification needed**\n\n")
			if question := stringValue(answer["clarification"]); question != "" {
				b.WriteString(question)
				b.WriteString("\n\n")
			}
			writeFact(&b, "Collection", stringValue(resp["collection_id"]))
			writeFact(&b, "Route", strings.Join(stringSlice(resp["route"]), " + "))
			writeFact(&b, "Template", stringValue(resp["template"]))
			if limitations := stringSlice(answer["limitations"]); len(limitations) > 0 {
				b.WriteString("\n**Why I paused**\n\n")
				for _, limitation := range limitations {
					b.WriteString("- ")
					b.WriteString(limitation)
					b.WriteString("\n")
				}
			}
			return b.String()
		}
	}

	b.WriteString("**Forensic analysis result**\n\n")
	writeFact(&b, "Collection", stringValue(resp["collection_id"]))
	writeFact(&b, "Template", stringValue(resp["template"]))
	writeFact(&b, "Target", stringValue(resp["target"]))
	writeFact(&b, "Route", strings.Join(stringSlice(resp["route"]), " + "))
	writeFact(&b, "Intent", stringValue(resp["intent"]))

	if answer, ok := resp["answer"].(map[string]any); ok {
		writeFact(&b, "Records status", stringValue(answer["records_status"]))
		writeFact(&b, "Records rows", stringValue(answer["records_row_count"]))
		writeFact(&b, "Evidence status", stringValue(answer["evidence_status"]))
		writeFact(&b, "Evidence count", stringValue(answer["evidence_count"]))
		writeFact(&b, "Records summary", stringValue(answer["records_summary"]))
		writeFact(&b, "Evidence summary", stringValue(answer["evidence_summary"]))
		writeFact(&b, "Records limitation", stringValue(answer["records_limitation"]))
		writeFact(&b, "Evidence limitation", stringValue(answer["evidence_limitation"]))
		writeFact(&b, "Guardrail", stringValue(answer["guardrail"]))
	}

	if planner, ok := resp["planner"].(map[string]any); ok {
		if confidence := confidenceValue(planner["confidence"]); confidence != "" {
			writeFact(&b, "Planner confidence", confidence)
		}
	}

	if records, ok := resp["records"].(map[string]any); ok {
		writeReadinessSummary(&b, records)
		writeRecordsSummary(&b, records)
	}
	if evidence, ok := resp["evidence"].(map[string]any); ok {
		writeEvidenceSummary(&b, evidence)
	}
	writeWarnings(&b, resp["warnings"])
	return b.String()
}

func formatEnterpriseForensicSummary(resp, enterprise map[string]any) string {
	var b strings.Builder
	template := stringValue(resp["template"])
	b.WriteString("## ")
	b.WriteString(forensicPresentationTitle(template))
	b.WriteString("\n\n")
	summary := strings.TrimSpace(stringValue(enterprise["summary"]))
	modelNarrative := ""
	if answer, ok := resp["answer"].(map[string]any); ok && strings.TrimSpace(stringValue(answer["llm_fallback_reason"])) == "" {
		modelNarrative = strings.TrimSpace(stringValue(answer["llm_summary"]))
	}
	switch {
	case modelNarrative != "":
		// Role A already passed the grounding validator (query.go only sets
		// llm_summary without llm_fallback_reason on VALIDATED_MODEL); prefer
		// its reasoning-rich text over the deterministic executive summary.
		b.WriteString(polishForensicSummary(modelNarrative))
		b.WriteString("\n\n")
	case summary != "":
		b.WriteString(polishForensicSummary(summary))
		b.WriteString("\n\n")
	default:
		b.WriteString("The analysis completed, but no plain-language summary was returned.\n\n")
	}

	if metrics := mapSlice(enterprise["metrics"]); len(metrics) > 0 {
		visible := userFacingForensicMetrics(metrics)
		if len(visible) > 0 {
			b.WriteString("### At a glance\n\n")
		}
		for _, metric := range visible {
			label := stringValue(metric["label"])
			value := stringValue(metric["value"])
			if label == "" || value == "" {
				continue
			}
			writeFact(&b, label, value)
		}
	}

	if grid, ok := enterprise["data_grid"].(map[string]any); ok {
		if rows := mapSlice(grid["rows"]); len(rows) > 0 {
			b.WriteString("\n### Key results\n\n")
			writeMarkdownTable(&b, template, rows, 5)
		}
	}

	limitations := stringSlice(enterprise["limitations"])
	visibleLimitations := make([]string, 0, len(limitations))
	for _, limitation := range limitations {
		if !isInternalForensicLimitation(limitation) {
			visibleLimitations = append(visibleLimitations, limitation)
		}
	}
	if len(visibleLimitations) > 0 {
		b.WriteString("\n### Important context\n\n")
		limit := len(visibleLimitations)
		if limit > 2 {
			limit = 2
		}
		for _, limitation := range visibleLimitations[:limit] {
			b.WriteString("- ")
			b.WriteString(limitation)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n<details>\n<summary>Technical details and source traceability</summary>\n\n")
	writeFact(&b, "Case", stringValue(resp["collection_id"]))
	writeFact(&b, "Analysis", forensicPresentationTitle(template))
	writeFact(&b, "Target", stringValue(resp["target"]))
	writeFact(&b, "Execution route", strings.Join(stringSlice(resp["route"]), " + "))

	if provenance := mapSlice(enterprise["provenance"]); len(provenance) > 0 {
		b.WriteString("\n**Source references**\n\n")
		limit := len(provenance)
		if limit > 5 {
			limit = 5
		}
		for i, item := range provenance[:limit] {
			source := stringValue(item["source"])
			file := stringValue(firstPresent(item, "source_file", "source_entry", "citation"))
			row := stringValue(item["row_number"])
			timestamp := stringValue(item["timestamp"])
			parts := []string{}
			if file != "" {
				parts = append(parts, fmt.Sprintf("file/source `%s`", file))
			}
			if row != "" {
				parts = append(parts, fmt.Sprintf("row `%s`", row))
			}
			if timestamp != "" {
				parts = append(parts, fmt.Sprintf("time `%s`", timestamp))
			}
			if source != "" {
				b.WriteString(fmt.Sprintf("%d. **%s**", i+1, source))
			} else {
				b.WriteString(fmt.Sprintf("%d.", i+1))
			}
			if len(parts) > 0 {
				b.WriteString(" - ")
				b.WriteString(strings.Join(parts, "; "))
			}
			b.WriteString("\n")
		}
	}

	if len(limitations) > 0 {
		b.WriteString("\n**All limitations**\n\n")
		for _, limitation := range limitations {
			b.WriteString("- ")
			b.WriteString(limitation)
			b.WriteString("\n")
		}
	}

	writeWarnings(&b, resp["warnings"])
	b.WriteString("\n</details>")
	return strings.TrimSpace(b.String())
}

func forensicPresentationTitle(template string) string {
	if title, ok := map[string]string{
		"collection_overview": "Case overview", "frequent_contacts": "Frequent contacts",
		"call_type_breakdown": "Call activity by type", "service_usage": "Communication service usage",
		"device_identity_changes": "Device and SIM identifier changes", "ipdr_endpoint_summary": "Network endpoints",
		"ipdr_domain_summary": "Observed network domains", "ipdr_protocol_breakdown": "Network traffic by protocol",
		"ipdr_session_volume": "Network session volume", "ipdr_subscriber_sessions": "Subscriber network sessions",
		"ipdr_concurrent_sessions": "Overlapping network sessions", "ipdr_timeline": "Network activity timeline",
		"temporal_activity": "Call activity over time", "top_locations": "Most frequently supplied locations",
		"geospatial_movement": "Chronological supplied-location activity", "anpr_sightings": "Vehicle sightings",
		"anpr_camera_sequence": "Camera observation sequence", "anpr_camera_activity": "Camera activity",
		"anpr_co_travel": "Nearby same-camera observations", "anpr_route_timing": "Time between vehicle sightings",
		"anpr_plate_variants": "Observed plate variants", "anpr_timeline": "Vehicle observation timeline",
		"subscriber_identity_lookup": "Subscriber observations", "subscriber_validity_timeline": "Subscriber validity history",
		"subscriber_device_links": "Subscriber, SIM and device links", "subscriber_status_summary": "Subscriber status summary",
		"subscriber_conflict_audit": "Subscriber data conflicts", "subscriber_reuse_candidates": "Reused subscriber identifiers",
		"entity_activity": "Entity activity", "relationship_network": "Evidence-backed related entities",
		"cross_family_correlation": "Cross-evidence correlation", "entity_timeline": "Cross-record timeline",
		"source_records": "Source records", "canonical_records": "Normalized records",
		"schema_profile": "Detected schemas and fields", "data_quality": "Data quality review",
		"evidence": "Evidence inventory and lineage", "shortest_call": "Shortest call",
		"longest_call": "Longest call", "duration_extremes": "Call-duration extremes",
		"first_seen_last_seen": "First and last observed activity", "activity_by_day": "Daily activity",
		"activity_by_hour": "Hourly activity", "night_activity": "Night-time activity",
		"repeated_location_visits": "Recurring supplied locations", "co_travel_or_co_presence": "Cross-evidence co-presence",
		"subscriber_profile": "CDR identifier profile", "imei_imsi_usage": "IMEI and IMSI usage",
		"tower_activity": "CDR cell and site activity", "suspicious_patterns": "Patterns requiring analyst review",
		"anomaly_summary": "Deterministic anomaly summary", "cross_dataset_entity_summary": "Entity summary across datasets",
		"source_file_audit": "Source-file reconciliation", "duplicate_upload_audit": "Duplicate-upload review",
		"case_readiness": "Case readiness", "evidence_package_summary": "Evidence package summary",
		"executive_case_brief": "Executive case brief", "court_ready_source_summary": "Source and provenance summary",
		"limitations_and_data_quality": "Limitations and missing-data review", "tower_site_lookup": "Tower/site reference",
		"tower_reference_timeline": "Tower reference history", "tower_coordinate_audit": "Tower coordinate review",
		"tower_status_summary": "Tower reference status", "tower_alias_conflicts": "Tower alias conflicts",
		"tower_cdr_join":                "CDR-to-tower reference match",
		"financial_transaction_summary": "Financial transaction summary", "access_failed_events": "Failed access events",
		"generic_filter_records": "Generic structured records", "document_metadata": "Document processing inventory",
		"document_search": "Document passage search", "image_metadata": "Image processing inventory",
		"image_ocr_search": "Image OCR observations", "face_candidate_observations": "Face candidate observations",
		"audio_metadata": "Audio processing inventory", "audio_transcript_search": "Audio transcript search",
		"video_metadata": "Video processing inventory", "video_timeline": "Video observation timeline",
	}[template]; ok {
		return title
	}
	return "Forensic analysis"
}

func polishForensicSummary(summary string) string {
	replacer := strings.NewReplacer(
		"**Deterministic Findings**", "### Answer",
		"**Model Interpretation**", "### Plain-language interpretation",
		"**Limitations**", "### What the result does not establish",
		"Synthesis fallback:", "Model note:",
	)
	return strings.TrimSpace(replacer.Replace(summary))
}

func userFacingForensicMetrics(metrics []map[string]any) []map[string]any {
	internal := map[string]bool{
		"template": true, "route": true, "planner confidence": true,
		"display rows": true, "provenance items": true,
	}
	out := make([]map[string]any, 0, 6)
	for _, metric := range metrics {
		label := strings.ToLower(stringValue(metric["label"]))
		if internal[label] || stringValue(metric["value"]) == "" {
			continue
		}
		out = append(out, metric)
		if len(out) == 6 {
			break
		}
	}
	return out
}

func isInternalForensicLimitation(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(normalized, "no raw full tables were sent to an llm")
}

func formatForensicTemplatesSummary(resp map[string]any) string {
	templates, ok := resp["templates"].([]any)
	if !ok || len(templates) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("**Deterministic forensic query templates**\n\n")
	b.WriteString("| Template | Route | Description |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, item := range templates {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := markdownCell(stringValue(firstPresent(row, "name", "template")))
		route := markdownCell(stringValue(firstPresent(row, "route", "intent")))
		description := markdownCell(stringValue(firstPresent(row, "description", "summary")))
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", name, route, description))
	}
	return b.String()
}

func formatForensicReportSummary(resp map[string]any) string {
	markdown := stringValue(resp["report"])
	if markdown == "" {
		markdown = stringValue(resp["summary"])
	}
	if markdown == "" {
		markdown = stringValue(resp["markdown"])
	}
	if markdown == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Forensic report\n\n")
	b.WriteString(markdown)
	b.WriteString("\n\n<details>\n<summary>Report scope and technical details</summary>\n\n")
	writeFact(&b, "Case", stringValue(resp["collection_id"]))
	writeFact(&b, "Analysis", forensicPresentationTitle(stringValue(resp["template"])))
	b.WriteString("\n</details>")
	return strings.TrimSpace(b.String())
}

func writeRecordsSummary(b *strings.Builder, records map[string]any) {
	sectionName, rows := firstRecordRows(records)
	if len(rows) == 0 {
		if rowCount := stringValue(records["row_count"]); rowCount != "" {
			b.WriteString("\n**Structured Records**\n\n")
			writeFact(b, "Rows", rowCount)
		}
		return
	}

	b.WriteString("\n**Structured Records**\n\n")
	writeFact(b, "Result set", sectionName)
	writeFact(b, "Rows returned", stringValue(records["row_count"]))
	writeMarkdownTable(b, sectionName, rows, 12)
}

func writeReadinessSummary(b *strings.Builder, records map[string]any) {
	readiness, ok := records["readiness"].(map[string]any)
	if !ok || len(readiness) == 0 {
		return
	}
	b.WriteString("\n**Case Readiness**\n\n")
	writeFact(b, "Level", stringValue(readiness["level"]))
	writeFact(b, "Score", stringValue(readiness["score"]))
	writeFact(b, "Accepted rows", stringValue(readiness["accepted_rows"]))
	writeFact(b, "Source files", stringValue(readiness["source_files"]))
	writeFact(b, "Record families", stringValue(readiness["record_families"]))
	writeFact(b, "KB assets", stringValue(readiness["kb_assets"]))
	if issues := stringSlice(readiness["issues"]); len(issues) > 0 {
		b.WriteString("\n**Readiness Issues**\n\n")
		for _, issue := range issues {
			b.WriteString("- ")
			b.WriteString(issue)
			b.WriteString("\n")
		}
	}
	if actions := stringSlice(readiness["recommended_next"]); len(actions) > 0 {
		b.WriteString("\n**Recommended Next**\n\n")
		for _, action := range actions {
			b.WriteString("- ")
			b.WriteString(action)
			b.WriteString("\n")
		}
	}
}

func writeEvidenceSummary(b *strings.Builder, evidence map[string]any) {
	results, _ := evidence["results"].([]any)
	b.WriteString("\n**Knowledge Base Evidence**\n\n")
	writeFact(b, "Retrieval mode", stringValue(evidence["mode"]))
	if len(results) == 0 {
		b.WriteString("No KB evidence previews were returned.\n")
		return
	}
	for i, item := range results {
		if i >= 3 {
			break
		}
		result, ok := item.(map[string]any)
		if !ok {
			continue
		}
		source := ""
		if metadata, ok := result["metadata"].(map[string]any); ok {
			source = stringValue(firstPresent(metadata, "file_name", "source", "source_entry"))
		}
		if source == "" {
			source = stringValue(firstPresent(result, "entry", "source", "id"))
		}
		content := evidencePreviewText(stringValue(firstPresent(result, "content", "preview")), 260)
		similarity := stringValue(result["similarity"])
		b.WriteString(fmt.Sprintf("%d. %s", i+1, content))
		if source != "" || similarity != "" {
			b.WriteString("\n   ")
			writeInlineFact(b, "Source", source)
			writeInlineFact(b, "Similarity", similarity)
		}
		b.WriteString("\n")
	}
}

func writeWarnings(b *strings.Builder, raw any) {
	warnings := stringSlice(raw)
	if len(warnings) == 0 {
		return
	}
	b.WriteString("\n**Warnings**\n\n")
	for _, warning := range warnings {
		b.WriteString("- ")
		b.WriteString(warning)
		b.WriteString("\n")
	}
}

func writeRawToolOutput(b *strings.Builder, raw any) {
	sanitized := sanitizeForensicRaw(raw, 0)
	encoded, err := json.MarshalIndent(sanitized, "", "  ")
	if err != nil || len(encoded) == 0 {
		return
	}
	b.WriteString("\n<details>\n<summary>Compact deterministic tool output</summary>\n\n```json\n")
	b.Write(encoded)
	b.WriteString("\n```\n</details>")
}

func sanitizeForensicRaw(value any, depth int) any {
	if depth > 6 {
		return stringValue(value)
	}
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if skipForensicRawField(key) {
				continue
			}
			out[key] = sanitizeForensicRaw(item, depth+1)
		}
		return out
	case []any:
		limit := len(typed)
		if limit > 12 {
			limit = 12
		}
		out := make([]any, 0, limit+1)
		for _, item := range typed[:limit] {
			out = append(out, sanitizeForensicRaw(item, depth+1))
		}
		if len(typed) > limit {
			out = append(out, fmt.Sprintf("... %d more rows omitted from chat preview", len(typed)-limit))
		}
		return out
	case []map[string]any:
		limit := len(typed)
		if limit > 12 {
			limit = 12
		}
		out := make([]any, 0, limit+1)
		for _, item := range typed[:limit] {
			out = append(out, sanitizeForensicRaw(item, depth+1))
		}
		if len(typed) > limit {
			out = append(out, fmt.Sprintf("... %d more rows omitted from chat preview", len(typed)-limit))
		}
		return out
	case string:
		return compactPreview(typed, 1200)
	default:
		return typed
	}
}

func skipForensicRawField(key string) bool {
	switch strings.ToLower(key) {
	case "raw_record", "headers", "quality_report", "metadata_quality_report", "routing_decision", "normalized_schema", "sample_rows":
		return true
	default:
		return false
	}
}

func firstRecordRows(records map[string]any) (string, []map[string]any) {
	preferred := []string{
		"readiness_checks",
		"canonical_records",
		"call_type_breakdown",
		"duration_extremes",
		"first_seen_last_seen",
		"activity_by_day",
		"hourly_activity",
		"top_locations",
		"entity_activity",
		"entity_timeline",
		"relationship_network",
		"cdr_source_records",
		"generic_source_records",
		"source_records",
		"record_families",
		"jobs",
		"assets",
		"data_quality",
		"recent_quality_jobs",
		"schema_profile",
	}
	for _, key := range preferred {
		if rows := mapRows(records[key]); len(rows) > 0 {
			return key, rows
		}
	}
	keys := make([]string, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if rows := mapRows(records[key]); len(rows) > 0 {
			return key, rows
		}
	}
	return "", nil
}

func mapRows(value any) []map[string]any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func writeMarkdownTable(b *strings.Builder, sectionName string, rows []map[string]any, limit int) {
	if len(rows) == 0 {
		return
	}
	columns := recordColumnsForSection(sectionName, rows)
	if len(columns) > 5 {
		columns = columns[:5]
	}
	b.WriteString("| ")
	headings := make([]string, 0, len(columns))
	for _, column := range columns {
		headings = append(headings, forensicFieldLabel(column))
	}
	b.WriteString(strings.Join(headings, " | "))
	b.WriteString(" |\n| ")
	b.WriteString(strings.Join(repeatString("---", len(columns)), " | "))
	b.WriteString(" |\n")
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	for _, row := range rows[:limit] {
		cells := make([]string, 0, len(columns))
		for _, column := range columns {
			cells = append(cells, markdownCell(stringValue(row[column])))
		}
		b.WriteString("| ")
		b.WriteString(strings.Join(cells, " | "))
		b.WriteString(" |\n")
	}
	if len(rows) > limit {
		b.WriteString(fmt.Sprintf("\nShowing %d of %d rows. Open the Records Intelligence data grid for the complete result.\n", limit, len(rows)))
	}
}

func writeCompactRows(b *strings.Builder, columns []string, rows []map[string]any, limit int) {
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	for idx, row := range rows[:limit] {
		values := make([]string, 0, len(columns))
		for _, column := range columns {
			value := stringValue(row[column])
			if value == "" {
				continue
			}
			values = append(values, fmt.Sprintf("**%s:** %s", column, compactPreview(value, 96)))
		}
		if len(values) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("%d. %s\n", idx+1, strings.Join(values, "; ")))
	}
	if len(rows) > limit {
		b.WriteString(fmt.Sprintf("\nShowing %d of %d rows. Open the Records Intelligence data grid for the complete result.\n", limit, len(rows)))
	}
}

func recordColumnsForSection(sectionName string, rows []map[string]any) []string {
	switch sectionName {
	case "frequent_contacts":
		if columns := presentColumns(rows, "counterparty", "dialed_number", "contact", "total_interactions", "incoming_count", "outgoing_count", "first_contact", "last_contact"); len(columns) > 0 {
			return columns
		}
	case "source_file_audit":
		if columns := presentColumns(rows, "source_file", "record_types", "accepted_rows", "rejected_rows", "duplicate_rows", "processing_states", "last_observed_at"); len(columns) > 0 {
			return columns
		}
	case "service_usage":
		if columns := presentColumns(rows, "service_group", "service_type", "event_count", "first_seen", "last_seen"); len(columns) > 0 {
			return columns
		}
	case "device_identity_changes", "imei_imsi_usage", "subscriber_profile":
		if columns := presentColumns(rows, "observed_at", "msisdn", "imei", "imsi", "change_type", "source_file"); len(columns) > 0 {
			return columns
		}
	case "ipdr_endpoint_summary":
		if columns := presentColumns(rows, "source_ip", "destination_ip", "nat_ip", "destination_port", "protocol", "session_count", "total_bytes"); len(columns) > 0 {
			return columns
		}
	case "ipdr_domain_summary":
		if columns := presentColumns(rows, "domain", "session_count", "total_bytes", "first_seen", "last_seen"); len(columns) > 0 {
			return columns
		}
	case "ipdr_protocol_breakdown":
		if columns := presentColumns(rows, "protocol", "session_count", "total_bytes", "first_seen", "last_seen"); len(columns) > 0 {
			return columns
		}
	case "ipdr_session_volume":
		if columns := presentColumns(rows, "activity_hour", "session_count", "total_bytes", "first_seen", "last_seen"); len(columns) > 0 {
			return columns
		}
	case "ipdr_subscriber_sessions", "ipdr_timeline", "ipdr_concurrent_sessions":
		if columns := presentColumns(rows, "start_time", "end_time", "subscriber_id", "source_ip", "destination_ip", "protocol", "total_bytes"); len(columns) > 0 {
			return columns
		}
	case "anpr_sightings", "anpr_camera_sequence", "anpr_timeline":
		if columns := presentColumns(rows, "observed_at", "plate", "camera_id", "location", "confidence", "source_file"); len(columns) > 0 {
			return columns
		}
	case "anpr_camera_activity":
		if columns := presentColumns(rows, "camera_id", "location", "sighting_count", "distinct_plate_count", "average_supplied_confidence", "first_seen", "last_seen"); len(columns) > 0 {
			return columns
		}
	case "anpr_co_travel":
		if columns := presentColumns(rows, "observed_at", "camera_id", "location", "target_plate", "other_plate", "time_difference_seconds"); len(columns) > 0 {
			return columns
		}
	case "anpr_route_timing":
		if columns := presentColumns(rows, "from_time", "to_time", "from_location", "to_location", "elapsed_seconds", "straight_line_distance_km"); len(columns) > 0 {
			return columns
		}
	case "anpr_plate_variants":
		if columns := presentColumns(rows, "normalized_plate", "observed_plate", "observation_count", "average_supplied_confidence", "first_seen", "last_seen"); len(columns) > 0 {
			return columns
		}
	case "subscriber_identity_lookup", "subscriber_validity_timeline", "subscriber_device_links":
		if columns := presentColumns(rows, "subscriber_reference", "msisdn", "imsi", "imei", "subscriber_status", "valid_from", "valid_to"); len(columns) > 0 {
			return columns
		}
	case "subscriber_status_summary":
		if columns := presentColumns(rows, "subscriber_status", "row_count", "manual_review_required", "first_validity_start", "last_validity_end"); len(columns) > 0 {
			return columns
		}
	case "subscriber_conflict_audit", "subscriber_reuse_candidates":
		if columns := presentColumns(rows, "identifier_type", "identifier_value", "distinct_value_count", "msisdn_count", "conflict_fields", "manual_review_required"); len(columns) > 0 {
			return columns
		}
	case "tower_site_lookup", "tower_reference_timeline", "tower_coordinate_audit":
		if columns := presentColumns(rows, "site_id", "sector_id", "site_name", "latitude", "longitude", "operational_status", "valid_from", "valid_to"); len(columns) > 0 {
			return columns
		}
	case "tower_status_summary":
		if columns := presentColumns(rows, "operational_status", "technology", "reference_row_count", "manual_review_required", "first_reference", "last_reference"); len(columns) > 0 {
			return columns
		}
	case "tower_alias_conflicts":
		if columns := presentColumns(rows, "site_alias", "reference_count", "conflicting_fields", "first_reference", "last_reference"); len(columns) > 0 {
			return columns
		}
	case "tower_cdr_join", "tower_activity":
		if columns := presentColumns(rows, "observed_at", "cell_site_id", "site_id", "sector_id", "location", "match_status", "source_file"); len(columns) > 0 {
			return columns
		}
	case "readiness_checks":
		if columns := presentColumns(rows, "check", "status", "score", "count"); len(columns) > 0 {
			return columns
		}
	case "call_type_breakdown":
		if columns := presentColumns(rows, "call_type", "direction", "event_count", "first_seen", "last_seen"); len(columns) > 0 {
			return columns
		}
	case "duration_extremes":
		if columns := presentColumns(rows, "metric", "duration_seconds", "observed_at", "call_type", "direction", "call_org_num", "call_dialed_num", "location", "source_file", "row_number"); len(columns) > 0 {
			return columns
		}
	case "first_seen_last_seen":
		if columns := presentColumns(rows, "entity_type", "entity_value", "observation_count", "first_seen", "last_seen", "record_types"); len(columns) > 0 {
			return columns
		}
	case "activity_by_day":
		if columns := presentColumns(rows, "activity_date", "record_type", "event_count"); len(columns) > 0 {
			return columns
		}
	case "hourly_activity":
		if columns := presentColumns(rows, "hour_of_day", "event_count"); len(columns) > 0 {
			return columns
		}
	case "top_locations":
		if columns := presentColumns(rows, "location", "cell_site_id", "observation_count", "first_seen", "last_seen"); len(columns) > 0 {
			return columns
		}
	case "entity_activity":
		if columns := presentColumns(rows, "entity_type", "entity_value", "observation_count", "first_seen", "last_seen", "record_types"); len(columns) > 0 {
			return columns
		}
	case "entity_timeline":
		if columns := presentColumns(rows, "event_time", "event_label", "location", "primary_entity", "secondary_entity", "source_file"); len(columns) > 0 {
			return columns
		}
	case "relationship_network":
		if columns := presentColumns(rows, "entity_type", "entity_value", "co_observation_count", "source_field", "first_seen", "last_seen"); len(columns) > 0 {
			return columns
		}
	case "source_records", "cdr_source_records":
		if columns := presentColumns(rows, "observed_at", "record_type", "call_type", "direction", "call_org_num", "call_dialed_num", "location", "source_file"); len(columns) > 0 {
			return columns
		}
	case "canonical_records":
		if columns := presentColumns(rows, "timestamp", "record_type", "primary_target", "secondary_target", "source_file", "row_number", "batch_id"); len(columns) > 0 {
			return columns
		}
	case "generic_source_records":
		if columns := presentColumns(rows, "observed_at", "record_type", "primary_entity", "secondary_entity", "location", "source_file"); len(columns) > 0 {
			return columns
		}
	case "record_families":
		if columns := presentColumns(rows, "record_type", "batch_count", "total_rows", "inserted_rows", "duplicate_rows", "rejected_rows"); len(columns) > 0 {
			return columns
		}
	case "data_quality", "jobs", "recent_quality_jobs":
		if columns := presentColumns(rows, "source_file", "record_type", "status", "total_rows", "accepted_rows", "duplicate_rows", "rejected_rows", "completed_at"); len(columns) > 0 {
			return columns
		}
	case "schema_profile", "assets":
		if columns := presentColumns(rows, "source_file", "detected_record_type", "structured_status", "rag_status", "storage_mode", "updated_at"); len(columns) > 0 {
			return columns
		}
	}
	return recordColumns(rows)
}

func forensicFieldLabel(field string) string {
	words := strings.Fields(strings.ReplaceAll(field, "_", " "))
	acronyms := map[string]string{
		"anpr": "ANPR", "cdr": "CDR", "cnic": "CNIC", "dns": "DNS",
		"imei": "IMEI", "imsi": "IMSI", "ip": "IP", "ipdr": "IPDR",
		"kb": "KB", "msisdn": "MSISDN", "nat": "NAT", "rf": "RF",
		"sms": "SMS", "sim": "SIM", "tcp": "TCP", "udp": "UDP",
	}
	for i, word := range words {
		if acronym, ok := acronyms[strings.ToLower(word)]; ok {
			words[i] = acronym
			continue
		}
		if word != "" {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

func presentColumns(rows []map[string]any, candidates ...string) []string {
	var columns []string
	for _, column := range candidates {
		for _, row := range rows {
			if _, ok := row[column]; ok {
				columns = append(columns, column)
				break
			}
		}
	}
	return columns
}

func recordColumns(rows []map[string]any) []string {
	priority := []string{
		"call_type",
		"direction",
		"event_count",
		"first_seen",
		"last_seen",
		"entity_type",
		"entity_value",
		"observation_count",
		"record_types",
		"source_file",
		"record_type",
		"row_count",
		"quality_dimension",
		"issue_count",
	}
	seen := map[string]bool{}
	var columns []string
	for _, column := range priority {
		for _, row := range rows {
			if _, ok := row[column]; ok {
				columns = append(columns, column)
				seen[column] = true
				break
			}
		}
	}
	var rest []string
	for _, row := range rows {
		for column := range row {
			if !seen[column] {
				rest = append(rest, column)
				seen[column] = true
			}
		}
	}
	sort.Strings(rest)
	columns = append(columns, rest...)
	return columns
}

func writeFact(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	b.WriteString("- **")
	b.WriteString(label)
	b.WriteString(":** ")
	b.WriteString(value)
	b.WriteString("\n")
}

func writeInlineFact(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	b.WriteString("**")
	b.WriteString(label)
	b.WriteString(":** ")
	b.WriteString(value)
	b.WriteString("  ")
}

func firstPresent(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value
		}
	}
	return nil
}

func mapSlice(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		rows := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			rows = append(rows, row)
		}
		return rows
	default:
		return nil
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if value := stringValue(item); value != "" {
				values = append(values, value)
			}
		}
		return strings.Join(values, ", ")
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func confidenceValue(value any) string {
	switch typed := value.(type) {
	case float64:
		return strconv.FormatFloat(typed, 'f', 2, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', 2, 64)
	default:
		return stringValue(value)
	}
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case nil:
		return nil
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if value := stringValue(item); value != "" {
				out = append(out, value)
			}
		}
		return out
	default:
		if value := stringValue(typed); value != "" {
			return []string{value}
		}
		return nil
	}
}

func repeatString(value string, count int) []string {
	out := make([]string, count)
	for i := range out {
		out[i] = value
	}
	return out
}

func compactPreview(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[:limit]) + "..."
}

func evidencePreviewText(value string, limit int) string {
	value = compactPreview(value, limit)
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, "#> \t")
	value = strings.ReplaceAll(value, "`", "'")
	value = strings.ReplaceAll(value, "|", "\\|")
	if value == "" {
		return "Evidence preview unavailable."
	}
	return value
}

func markdownCell(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
