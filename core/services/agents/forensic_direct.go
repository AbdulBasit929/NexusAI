package agents

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type forensicDirectResult struct {
	ToolName string
	ArgsJSON string
	Text     string
}

var forensicTargetPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`),
	regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
	regexp.MustCompile(`(?i)\b[A-Z]{2,6}(?:[-_][A-Z0-9]{2,8}){1,4}\b`),
	regexp.MustCompile(`(?i)\b[A-Z]{1,4}[- ]?\d{1,6}[A-Z]?\b`),
	regexp.MustCompile(`\b\+?\d(?:[\s\-]?\d){7,18}\b`),
}

// CanRunDeterministicForensicRoute reports whether a message should bypass the
// LLM and go directly to the records API. It intentionally checks only routing
// eligibility, so callers can decide before enqueueing work.
func CanRunDeterministicForensicRoute(cfg *AgentConfig, message, userID string) bool {
	toolConfig := forensicRecordsToolConfig(cfg, userID)
	if toolConfig.APIURL == "" {
		return false
	}
	return deterministicForensicRouteRequested(message)
}

// TryDeterministicForensicRoute executes deterministic records analytics for
// forensic prompts and emits the same callbacks as the normal agent executor.
func TryDeterministicForensicRoute(cfg *AgentConfig, message, userID string, cb Callbacks, opts ...ExecuteChatOpts) (string, bool, error) {
	toolConfig := forensicRecordsToolConfig(cfg, userID)
	if toolConfig.APIURL == "" {
		return "", false, nil
	}
	direct, ok, err := runDeterministicForensicRoute(message, toolConfig)
	if !ok {
		return "", false, nil
	}
	text, finishErr := finishDeterministicForensicRoute(cfg, direct, err, cb, opts)
	return text, true, finishErr
}

func runDeterministicForensicRoute(message string, toolConfig ForensicRecordsToolConfig) (forensicDirectResult, bool, error) {
	normalized := strings.ToLower(message)
	switch {
	case strings.Contains(normalized, "forensic_query_templates") ||
		(strings.Contains(normalized, "template") && strings.Contains(normalized, "forensic")):
		tool := ForensicQueryTemplatesTool{ForensicRecordsToolConfig: toolConfig}
		text, raw, err := tool.Run(ForensicQueryTemplatesArgs{})
		if err != nil {
			return forensicDirectResult{}, true, err
		}
		return forensicDirectResult{
			ToolName: "forensic_query_templates",
			ArgsJSON: "{}",
			Text:     formatDirectForensicResult("forensic_query_templates", text, raw),
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
		argsJSON, _ := json.Marshal(args)
		return forensicDirectResult{
			ToolName: "forensic_get_evidence",
			ArgsJSON: string(argsJSON),
			Text:     formatDirectForensicResult("forensic_get_evidence", text, raw),
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
		argsJSON, _ := json.Marshal(args)
		return forensicDirectResult{
			ToolName: "forensic_list_evidence",
			ArgsJSON: string(argsJSON),
			Text:     formatDirectForensicResult("forensic_list_evidence", text, raw),
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
		argsJSON, _ := json.Marshal(args)
		return forensicDirectResult{
			ToolName: "forensic_generate_report",
			ArgsJSON: string(argsJSON),
			Text:     formatDirectForensicResult("forensic_generate_report", text, raw),
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
		args := ForensicHybridQueryArgs{
			Query:          message,
			Target:         extractForensicTarget(message),
			Template:       forensicTemplateHint(message),
			Limit:          20,
			MaxKBResults:   3,
			SynthesisModel: toolConfig.Model,
		}
		tool := ForensicHybridQueryTool{ForensicRecordsToolConfig: toolConfig}
		text, raw, err := tool.Run(args)
		if err != nil {
			return forensicDirectResult{}, true, err
		}
		argsJSON, _ := json.Marshal(args)
		return forensicDirectResult{
			ToolName: "forensic_hybrid_query",
			ArgsJSON: string(argsJSON),
			Text:     formatDirectForensicResult("forensic_hybrid_query", text, raw),
		}, true, nil
	}
	return forensicDirectResult{}, false, nil
}

func deterministicForensicRouteRequested(message string) bool {
	normalized := strings.ToLower(message)
	return strings.Contains(normalized, "forensic_query_templates") ||
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
	return regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`).FindString(message)
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
		"cell", "site", "plate", "camera", "call", "sms", "gprs", "volte",
		"ingested", "ingest job", "source file", "source files", "canonical", "raw payload",
		"raw_payload", "jsonb", "batch id", "batch_id", "record type", "record_type",
	})
	hasAnalyticIntent := containsForensicTerm(normalized, []string{
		"shortest", "longest", "duration", "frequent", "contact", "contacts", "breakdown",
		"incoming", "outgoing", "timeline", "chronology", "sequence", "history", "relationship",
		"network", "related", "entity", "entities", "location", "locations", "where", "visited",
		"movement", "geospatial", "tower", "site", "cell", "hourly", "nocturnal", "night",
		"quality", "duplicate", "duplicates", "rejected", "error", "errors", "failed", "parser",
		"schema", "headers", "columns", "fields", "overview", "batch", "ingest", "status",
		"source row", "source rows", "source record", "source records", "raw row", "raw rows",
		"sighting", "sightings", "evidence", "policy", "citation", "citations", "cite",
		"file", "files", "audit", "summary", "anomaly", "suspicious", "first seen", "last seen",
		"what happened", "happened", "date",
		"compare", "comparison", "between", "limitations", "missing data", "data gaps",
		"investigate", "key findings", "important findings",
		"canonical", "raw payload", "raw_payload", "jsonb", "field exists", "field not exists",
		"batch id", "batch_id", "record type", "record_type", "primary target", "secondary target",
	})

	return hasAnalyticIntent && (hasTarget || hasRecordContext || hasCollectionAnalyticIntent || hasDateExpression(message))
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
		"case summary",
		"case readiness",
		"evidence health",
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
	})
}

func forensicTemplateHint(message string) string {
	normalized := strings.ToLower(message)
	switch {
	case isForensicCanonicalFilterQuery(normalized):
		return "canonical_records"
	case containsForensicTerm(normalized, []string{"canonical records", "canonical query", "canonical sql", "forensic.records", "raw payload", "raw_payload", "jsonb", "record type", "record_type", "batch id", "batch_id", "field exists", "field not exists"}):
		return "canonical_records"
	case containsForensicTerm(normalized, []string{"which files", "source file", "source files", "file audit", "ingested files", "files ingested"}):
		return "source_file_audit"
	case containsForensicTerm(normalized, []string{"duplicate upload", "duplicate uploads", "duplicate file", "duplicate files"}):
		return "duplicate_upload_audit"
	case containsForensicTerm(normalized, []string{"limitations", "known limitations", "missing data", "data gaps", "parser limitations"}):
		return "limitations_and_data_quality"
	case containsForensicTerm(normalized, []string{"available entities", "entity inventory", "known entities"}):
		return "entity_activity"
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
	case containsForensicTerm(normalized, []string{"data quality", "quality issue", "quality issues", "duplicate", "duplicates", "rejected"}):
		return "data_quality"
	case containsForensicTerm(normalized, []string{"case readiness", "readiness", "evidence health", "ready for court", "ready for production", "production ready", "can we trust", "reliable enough"}):
		return "case_readiness"
	case containsForensicTerm(normalized, []string{"first seen", "last seen", "first_seen", "last_seen"}):
		return "first_seen_last_seen"
	case hasDateExpression(message) && containsForensicTerm(normalized, []string{"what happened", "happened", "timeline", "events", "activity on", "date"}):
		return "entity_timeline"
	case containsForensicTerm(normalized, []string{"activity by day", "daily activity"}):
		return "activity_by_day"
	case containsForensicTerm(normalized, []string{"activity by hour", "hourly activity"}):
		return "activity_by_hour"
	case containsForensicTerm(normalized, []string{"night activity", "nocturnal"}):
		return "night_activity"
	case containsForensicTerm(normalized, []string{"top location", "top locations", "most often observed", "where was"}):
		return "top_locations"
	case containsForensicTerm(normalized, []string{"tower activity", "cell site", "cell sites"}):
		return "tower_activity"
	case containsForensicTerm(normalized, []string{"evidence package"}):
		return "evidence_package_summary"
	case containsForensicTerm(normalized, []string{"executive brief", "case brief"}):
		return "executive_case_brief"
	case containsForensicTerm(normalized, []string{"court ready", "court-ready"}):
		return "court_ready_source_summary"
	case containsForensicTerm(normalized, []string{"suspicious", "anomaly", "anomalies", "investigate next", "what should i investigate", "key findings", "important findings"}):
		return "suspicious_patterns"
	default:
		return ""
	}
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
	hasReportIntent := containsForensicTerm(normalized, []string{
		"report", "brief", "briefing", "case package", "findings summary", "intelligence summary",
	})
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

func extractForensicTarget(message string) string {
	for _, pattern := range forensicTargetPatterns {
		match := pattern.FindString(message)
		if target := normalizeExtractedForensicTarget(match); target != "" {
			return target
		}
	}
	return ""
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

func hasDateExpression(message string) bool {
	return regexp.MustCompile(`\b\d{4}[-/]\d{1,2}[-/]\d{1,2}\b|\b\d{1,2}[-/]\d{1,2}[-/]\d{4}\b|\b\d{8}\b`).FindString(message) != ""
}

func isForensicDateLikeTarget(target string) bool {
	value := strings.TrimSpace(target)
	if value == "" {
		return false
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
		if markdown, ok := report["markdown"].(string); ok && strings.TrimSpace(markdown) != "" {
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
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("**Forensic evidence catalog**\n\n")
	if summary, ok := resp["summary"].(map[string]any); ok {
		writeFact(&b, "Evidence total", stringValue(summary["evidence_total"]))
		writeFact(&b, "Completed", stringValue(firstPresent(summary, "completed", "evidence_completed")))
		writeFact(&b, "Failed", stringValue(firstPresent(summary, "failed", "evidence_failed")))
	}
	b.WriteString("\n| Evidence ID | Source File | Type | Status | Route |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, item := range items {
		b.WriteString(fmt.Sprintf(
			"| %s | %s | %s | %s | %s |\n",
			markdownCell(stringValue(item["evidence_id"])),
			markdownCell(stringValue(firstPresent(item, "source_file", "original_filename"))),
			markdownCell(stringValue(firstPresent(item, "detected_type", "modality"))),
			markdownCell(stringValue(item["processing_status"])),
			markdownCell(stringValue(item["processing_route"])),
		))
	}
	return b.String()
}

func formatForensicEvidenceDetailSummary(resp map[string]any) string {
	item, ok := resp["item"].(map[string]any)
	if !ok || len(item) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("**Forensic evidence detail**\n\n")
	writeFact(&b, "Evidence ID", stringValue(item["evidence_id"]))
	writeFact(&b, "Source file", stringValue(firstPresent(item, "source_file", "original_filename")))
	writeFact(&b, "Detected type", stringValue(item["detected_type"]))
	writeFact(&b, "Status", stringValue(item["processing_status"]))
	writeFact(&b, "Route", stringValue(item["processing_route"]))
	writeFact(&b, "KB entry", stringValue(item["kb_entry_ref"]))
	writeFact(&b, "Records batch", stringValue(item["records_batch_id"]))
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
	summary := strings.TrimSpace(stringValue(enterprise["summary"]))
	if summary != "" {
		b.WriteString(summary)
		b.WriteString("\n\n")
	} else {
		b.WriteString("**Executive Intelligence Brief**\n\n")
	}

	writeFact(&b, "Collection", stringValue(resp["collection_id"]))
	writeFact(&b, "Template", stringValue(resp["template"]))
	writeFact(&b, "Target", stringValue(resp["target"]))
	writeFact(&b, "Route", strings.Join(stringSlice(resp["route"]), " + "))
	writeFact(&b, "Intent", stringValue(resp["intent"]))

	if metrics := mapSlice(enterprise["metrics"]); len(metrics) > 0 {
		b.WriteString("\n**Operating Metrics**\n\n")
		for _, metric := range metrics {
			label := stringValue(metric["label"])
			value := stringValue(metric["value"])
			if label == "" || value == "" {
				continue
			}
			writeFact(&b, label, value)
		}
	}

	if synthesis, ok := enterprise["synthesis"].(map[string]any); ok {
		if claims := mapSlice(synthesis["claims"]); len(claims) > 0 {
			b.WriteString("\n**Sourced Claims**\n\n")
			for _, claim := range claims {
				source := stringValue(claim["source"])
				text := stringValue(claim["claim"])
				if text == "" {
					continue
				}
				if source != "" {
					b.WriteString("- **")
					b.WriteString(source)
					b.WriteString(":** ")
				} else {
					b.WriteString("- ")
				}
				b.WriteString(text)
				if value := stringValue(claim["value"]); value != "" {
					b.WriteString(" (")
					b.WriteString(value)
					b.WriteString(")")
				}
				b.WriteString("\n")
			}
		}
	}

	if grid, ok := enterprise["data_grid"].(map[string]any); ok {
		if rows := mapSlice(grid["rows"]); len(rows) > 0 {
			b.WriteString("\n**Data Grid Preview**\n\n")
			writeMarkdownTable(&b, "enterprise_data_grid", rows, 8)
		}
	}

	if provenance := mapSlice(enterprise["provenance"]); len(provenance) > 0 {
		b.WriteString("\n**Provenance**\n\n")
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

	if limitations := stringSlice(enterprise["limitations"]); len(limitations) > 0 {
		b.WriteString("\n**Coverage Limitations**\n\n")
		for _, limitation := range limitations {
			b.WriteString("- ")
			b.WriteString(limitation)
			b.WriteString("\n")
		}
	}

	writeWarnings(&b, resp["warnings"])
	return strings.TrimSpace(b.String())
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
	if markdown, ok := resp["report"].(string); ok && strings.TrimSpace(markdown) != "" {
		return markdown
	}
	if markdown, ok := resp["summary"].(string); ok && strings.TrimSpace(markdown) != "" {
		return markdown
	}
	return ""
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
		writeCompactRows(b, columns, rows, limit)
		return
	}
	b.WriteString("| ")
	b.WriteString(strings.Join(columns, " | "))
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
	if len(columns) > 6 {
		return columns[:6]
	}
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
