package agents

import (
	"encoding/json"
	"fmt"
	"strings"
)

const forensicPresentationContract = "forensics.agent-presentation/v1"

// buildForensicPresentationMetadata turns sidecar output into a bounded display
// contract. The narrative remains available for compatibility, while clients can
// render exact findings without scraping markdown or receiving the full raw payload.
func buildForensicPresentationMetadata(result forensicDirectResult) map[string]any {
	root, _ := result.Raw.(map[string]any)
	enterprise := mapValue(root, "enterprise")
	if len(enterprise) == 0 {
		enterprise = root
	}
	factPacket := mapValue(enterprise, "fact_packet")

	executive := normalizeExecutiveAnswer(firstString(enterprise, "executive_answer", "summary", "answer"))
	metrics := presentationMetrics(root, enterprise)
	findings := boundedAnySlice(valueAt(enterprise, "deterministic_findings"), 20)
	if len(findings) == 0 {
		findings = boundedAnySlice(valueAt(enterprise, "findings"), 20)
	}
	table := presentationTable(root, enterprise)
	if len(table) == 0 {
		if tables := boundedAnySlice(valueAt(enterprise, "tables"), 1); len(tables) > 0 {
			table, _ = tables[0].(map[string]any)
			table = boundedTable(table, 20)
		}
	}
	if len(table) > 0 {
		if table["columns"] == nil {
			table["columns"] = []string{}
		}
		if table["priority_columns"] == nil {
			table["priority_columns"] = []string{}
		}
	}
	rawCitations := boundedAnySlice(valueAt(factPacket, "citations"), 20)
	if len(rawCitations) == 0 {
		rawCitations = boundedAnySlice(valueAt(enterprise, "citations"), 20)
	}
	if len(rawCitations) == 0 {
		rawCitations = boundedAnySlice(valueAt(enterprise, "semantic_evidence"), 20)
	}
	citations := rawCitations
	if len(citations) == 0 {
		rawCitations = boundedAnySlice(valueAt(enterprise, "provenance"), 20)
		citations = rawCitations
	}
	citations = presentationCitations(citations)
	visualizations := boundedAnySlice(valueAt(enterprise, "visualizations"), 8)
	limitations := presentationLimitations(valueAt(enterprise, "limitations"))
	nextActions := boundedAnySlice(valueAt(enterprise, "next_actions"), 8)
	if len(nextActions) == 0 {
		nextActions = boundedAnySlice(valueAt(enterprise, "recommended_actions"), 8)
	}

	modelStatus := "not_used"
	modelInterpretation := valueAt(enterprise, "model_interpretation")
	narrative := mapValue(enterprise, "narrative")
	if len(narrative) > 0 {
		modelInterpretation = narrative
		if firstString(narrative, "status") == "validated_model" {
			modelStatus = "succeeded"
		} else if fallback, _ := valueAt(narrative, "fallback").(bool); fallback {
			modelStatus = "fallback"
		}
		if narrativeFindings := boundedAnySlice(valueAt(narrative, "key_findings"), 20); len(narrativeFindings) > 0 {
			findings = narrativeFindings
		}
	}
	answer := mapValue(root, "answer")
	if modelInterpretation == nil && firstString(answer, "llm_summary") != "" {
		modelInterpretation = map[string]any{"status": "succeeded", "summary": firstString(answer, "llm_summary")}
	}
	if modelMap, ok := modelInterpretation.(map[string]any); ok && len(modelMap) > 0 && len(narrative) == 0 {
		modelStatus = firstString(modelMap, "status")
		if modelStatus == "" {
			modelStatus = "succeeded"
		}
	}
	if status := firstString(enterprise, "model_status", "synthesis_status"); status != "" {
		modelStatus = status
	}
	if firstString(answer, "llm_fallback_reason") != "" {
		modelStatus = "fallback"
	}
	elapsedMS := result.Elapsed.Milliseconds()
	if elapsedMS < 1 {
		elapsedMS = 1
	}

	operation := mapValue(enterprise, "operation")
	if mode := firstString(operation, "submode"); mode == "image_similarity" || mode == "face_similarity" {
		findings = []any{}
		for _, row := range mapSlice(table["rows"]) {
			name := firstString(row, "source_file")
			if name == "" {
				name = "Source name unavailable"
			}
			findings = append(findings, map[string]any{"claim_type": "candidate_similarity", "text": fmt.Sprintf("Rank %v: %s — similarity score %v (not a probability).", row["rank"], name, row["similarity_score"])})
		}
	}
	familyID := firstString(operation, "family_id")
	operationID := firstString(operation, "operation_id")
	if operationID == "" {
		operationID = firstString(enterprise, "operation_id")
	}
	sourceAccess := firstString(operation, "source_access")
	if sourceAccess == "" {
		sourceAccess = "records"
	}
	presentation := map[string]any{
		"contract_version":    forensicPresentationContract,
		"status":              firstString(enterprise, "status"),
		"result_state":        firstString(enterprise, "result_state"),
		"processing_state":    firstString(enterprise, "processing_state"),
		"row_count":           boundedValue(valueAt(enterprise, "row_count"), 0),
		"clarification":       firstString(enterprise, "clarification"),
		"execution_authority": "deterministic_records",
		"model_role":          "explanation_after_exact_analysis",
		"model_status":        modelStatus,
		"elapsed_ms":          elapsedMS,
		"tool_id":             result.ToolName,
		"operation_id":        operationID,
		"family_id":           familyID,
		"specialist":          forensicSpecialistForFamily(familyID),
		"source_access":       sourceAccess,
		"executive_answer":    executive,
		"metrics":             metrics,
		"findings":            findings,
		"table":               table,
		"citations":           citations,
		"visualizations":      visualizations,
		"limitations":         limitations,
		"next_actions":        nextActions,
		"fact_packet":         boundedValue(factPacket, 0),
		"narrative":           boundedValue(narrative, 0),
		"result_kind":         firstString(enterprise, "result_kind"),
		"proof_state":         boundedValue(valueAt(enterprise, "proof_state"), 0),
		"relationships":       boundedAnySlice(valueAt(factPacket, "relationships"), 12),
		"language":            firstString(mapValue(factPacket, "language"), "tag"),
		"text_direction":      firstString(mapValue(factPacket, "presentation_hints"), "direction"),
		"context":             boundedValue(valueAt(enterprise, "conversation_context"), 0),
		"trace": map[string]any{
			"tool_id":        result.ToolName,
			"elapsed_ms":     elapsedMS,
			"authority":      "deterministic_records",
			"operation":      boundedValue(operation, 0),
			"coverage":       boundedValue(valueAt(enterprise, "coverage"), 0),
			"telemetry":      boundedValue(valueAt(enterprise, "telemetry"), 0),
			"raw_metrics":    boundedValue(valueAt(enterprise, "metrics"), 0),
			"raw_provenance": boundedValue(rawCitations, 0),
		},
	}
	if modelInterpretation != nil {
		presentation["model_interpretation"] = boundedValue(modelInterpretation, 0)
	}
	return map[string]any{"presentation": presentation}
}

func presentationTable(root, enterprise map[string]any) map[string]any {
	table := boundedTable(firstMap(enterprise, "data_grid", "table"), 20)
	rows := mapSlice(table["rows"])
	if len(rows) == 0 {
		return table
	}
	mode := firstString(mapValue(enterprise, "operation"), "submode")
	if mode == "image_similarity" || mode == "face_similarity" {
		columns := presentColumns(rows, "rank", "similarity_score", "source_file", "query_source_file")
		table["columns"], table["priority_columns"] = columns, columns
		table["title"] = "Similar image candidates"
		if mode == "face_similarity" {
			table["title"] = "Similar face candidates — not identity confirmation"
		}
		return table
	}
	template := firstString(root, "template")
	if template == "video_anpr_grouped_timeline" {
		columns := presentColumns(rows, "normalized_plate_text", "group_selected_plate_text", "match_kind", "source_time", "frame_number", "source_file", "sightings_count", "manual_review_required")
		if columns == nil {
			columns = []string{}
		}
		table["columns"], table["priority_columns"] = columns, columns
		table["title"] = "Retained plate readings and selected group candidates"
		return table
	}
	columns := recordColumnsForSection(template, rows)
	if template == "source_file_audit" {
		columns = presentColumns(rows, "source_file", "record_types", "accepted_rows", "rejected_rows", "duplicate_rows", "processing_states", "last_observed_at")
	}
	table["columns"] = columns
	priority := boundedStringSlice(valueAt(mapValue(mapValue(enterprise, "fact_packet"), "presentation_hints"), "priority_columns"), 8)
	if len(priority) == 0 && len(columns) > 0 {
		priority = append([]string(nil), columns[:min(len(columns), 5)]...)
	}
	table["priority_columns"] = priority
	table["column_visibility"] = "progressive"
	switch template {
	case "source_file_audit":
		table["title"] = "Registered source files"
	case "frequent_contacts":
		table["title"] = "Most frequent phone contacts"
	case "case_readiness":
		table["title"] = "Evidence processing readiness"
	case "temporal_activity":
		table["title"] = "Temporal calculation components"
		table["columns"] = presentColumns(rows, "section", "hour_of_day", "day_start", "event_count", "nocturnal_events", "nonzero_duration_events", "average_nonzero_duration", "metric", "duration_seconds", "observed_at", "source_file", "row_number")
	default:
		if firstString(table, "title") == "" {
			table["title"] = forensicPresentationTitle(template)
		}
	}
	return table
}

func presentationMetrics(root, enterprise map[string]any) []any {
	template := firstString(root, "template")
	internal := map[string]bool{
		"template": true, "route": true, "planner confidence": true,
		"display rows": true, "provenance items": true,
	}
	metrics := boundedAnySlice(valueAt(enterprise, "metrics"), 20)
	out := make([]any, 0, 6)
	for _, value := range metrics {
		metric, ok := value.(map[string]any)
		if !ok {
			continue
		}
		label := firstString(metric, "label", "name")
		metricValue := valueAt(metric, "value")
		if label == "" || internal[strings.ToLower(label)] || isEmptyPresentationValue(metricValue) {
			continue
		}
		if strings.EqualFold(label, "Evidence Count") && firstString(metric, "source") == "kb_rag" && metricValue == nil {
			continue
		}
		if strings.EqualFold(label, "Records Row Count") {
			switch template {
			case "source_file_audit":
				label = "Source files"
			case "frequent_contacts":
				label = "Ranked contacts"
			case "temporal_activity":
				label = "Matched CDR events"
			default:
				label = "Results"
			}
		}
		out = append(out, map[string]any{"label": label, "value": boundedValue(metricValue, 0)})
		if len(out) == 6 {
			break
		}
	}
	return out
}

func isEmptyPresentationValue(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}

func presentationCitations(values []any) []any {
	out := make([]any, 0, len(values))
	for index, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
				out = append(out, map[string]any{"label": fmt.Sprintf("Source %d", index+1), "detail": text})
			}
			continue
		}
		if firstString(item, "source") == "records_aggregate" {
			count := strings.TrimSpace(fmt.Sprint(valueAt(item, "contribution_count")))
			groups := strings.TrimSpace(fmt.Sprint(valueAt(item, "source_group_count")))
			resultKey := firstString(item, "result_key")
			status := "lineage incomplete"
			if complete, _ := valueAt(item, "lineage_complete").(bool); complete {
				status = "complete lineage"
			}
			parts := []string{count + " contributing rows", groups + " source groups", status}
			if resultKey != "" {
				parts = append(parts, "result "+resultKey)
			}
			out = append(out, map[string]any{
				"label":          "Aggregate contribution lineage",
				"detail":         strings.Join(parts, " · "),
				"proof_role":     "aggregate_contribution_lineage",
				"completeness":   status,
				"source_file":    firstString(item, "source_file"),
				"evidence_id":    valueAt(item, "evidence_id"),
				"version_id":     valueAt(item, "version_id"),
				"lineage_digest": valueAt(item, "lineage_digest"),
			})
			continue
		}
		file := firstString(item, "source_file", "source_entry", "source_name")
		recordType := firstString(item, "record_type")
		row := strings.TrimSpace(fmt.Sprint(valueAt(item, "row_number")))
		if row == "<nil>" {
			row = ""
		}
		label := file
		if label == "" {
			label = firstString(item, "citation", "source")
		}
		if label == "" {
			label = fmt.Sprintf("Source %d", index+1)
		}
		parts := make([]string, 0, 3)
		if recordType != "" {
			parts = append(parts, strings.ToUpper(recordType)+" evidence")
		}
		if row != "" {
			parts = append(parts, "row "+row)
		}
		if timestamp := firstString(item, "timestamp"); timestamp != "" {
			parts = append(parts, timestamp)
		}
		citation := map[string]any{
			"label":       label,
			"detail":      strings.Join(parts, " · "),
			"source_file": file,
			"evidence_id": valueAt(item, "evidence_id"),
		}
		if version := valueAt(item, "version_id"); version != nil {
			citation["version_id"] = version
		}
		if artifact := valueAt(item, "artifact_id"); artifact != nil {
			citation["artifact_id"] = artifact
		}
		if role := firstString(item, "proof_role"); role != "" {
			citation["proof_role"] = role
		}
		if citationID := firstString(item, "citation_id", "id"); citationID != "" {
			citation["citation_id"] = citationID
			citation["proof_role"] = firstString(item, "proof_role")
			citation["completeness"] = firstString(item, "proof_completeness")
			citation["version_id"] = valueAt(item, "version_id")
		}
		if locatorValue := firstPresentValue(item, "locator", "source_locator", "citation_locator"); locatorValue != nil {
			switch locator := locatorValue.(type) {
			case map[string]any:
				citation["locator"] = locator
			case string:
				var structuredLocator map[string]any
				if json.Unmarshal([]byte(locator), &structuredLocator) == nil && len(structuredLocator) > 0 {
					citation["locator"] = structuredLocator
				} else if strings.TrimSpace(locator) != "" {
					citation["locator"] = locator
				}
			}
			if citation["detail"] == "" {
				if locator, ok := citation["locator"].(string); ok {
					citation["detail"] = locator
				}
			}
		}
		out = append(out, citation)
	}
	return out
}

func presentationLimitations(value any) []string {
	items := boundedStringSlice(value, 20)
	out := make([]string, 0, 6)
	for _, item := range items {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized == "" || strings.Contains(normalized, "no raw full tables were sent to an llm") || strings.HasPrefix(normalized, "no records query was run") || strings.HasPrefix(normalized, "which ") {
			continue
		}
		out = append(out, item)
		if len(out) == 6 {
			break
		}
	}
	return out
}

func forensicSpecialistForFamily(familyID string) string {
	switch familyID {
	case "communications_cdr":
		return "Communications_CDR_Analyst"
	case "network_ipdr":
		return "Network_IPDR_Capture_Analyst"
	case "anpr_vehicles":
		return "Vehicle_ANPR_Geospatial_Analyst"
	case "subscriber_identity":
		return "Subscriber_Identity_Analyst"
	case "tower_location":
		return "Tower_Location_Reference_Analyst"
	default:
		return "Forensic_Records_Analyst"
	}
}

func valueAt(values map[string]any, key string) any {
	if values == nil {
		return nil
	}
	return values[key]
}

func firstPresentValue(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func mapValue(values map[string]any, key string) map[string]any {
	result, _ := valueAt(values, key).(map[string]any)
	return result
}

func firstMap(values map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if result := mapValue(values, key); len(result) > 0 {
			return result
		}
	}
	return nil
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeExecutiveAnswer(value string) string {
	value = strings.TrimSpace(value)
	for _, marker := range []string{"**Deterministic Findings**", "**Model Interpretation**", "**Limitations**", "### Answer", "### Plain-language interpretation", "### What the result does not establish"} {
		value = strings.ReplaceAll(value, marker, "")
	}
	value = strings.ReplaceAll(value, "**", "")
	value = strings.TrimSpace(value)
	if len(value) > 1200 {
		return value[:1200] + "…"
	}
	return value
}

func boundedTable(table map[string]any, limit int) map[string]any {
	if len(table) == 0 {
		return map[string]any{}
	}
	result := map[string]any{}
	if columns := boundedColumnKeys(table["columns"], 20); len(columns) > 0 {
		result["columns"] = columns
	}
	if rows := boundedAnySlice(table["rows"], limit); len(rows) > 0 {
		result["rows"] = rows
	}
	if count := table["count"]; count != nil {
		result["count"] = boundedValue(count, 0)
	}
	if countLabel := firstString(table, "count_label"); countLabel != "" {
		result["count_label"] = countLabel
	}
	if title := firstString(table, "title", "name"); title != "" {
		result["title"] = title
	}
	return result
}

func boundedColumnKeys(value any, limit int) []string {
	items := boundedAnySlice(value, limit)
	result := make([]string, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case string:
			if typed = strings.TrimSpace(typed); typed != "" {
				result = append(result, typed)
			}
		case map[string]any:
			if key := firstString(typed, "key", "field", "name"); key != "" {
				result = append(result, key)
			}
		}
	}
	return result
}

func boundedStringSlice(value any, limit int) []string {
	items := boundedAnySlice(value, limit)
	result := make([]string, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case string:
			if typed = strings.TrimSpace(typed); typed != "" {
				result = append(result, typed)
			}
		case map[string]any:
			if text := firstString(typed, "text", "label", "message", "action"); text != "" {
				result = append(result, text)
			}
		}
	}
	return result
}

func boundedAnySlice(value any, limit int) []any {
	items, ok := value.([]any)
	if !ok || limit <= 0 {
		return nil
	}
	if len(items) > limit {
		items = items[:limit]
	}
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, boundedValue(item, 0))
	}
	return result
}

func boundedValue(value any, depth int) any {
	// Visualization descriptors nest source rows under visualization -> spec ->
	// rows. Preserve their scalar values while retaining a hard depth bound.
	if depth > 4 {
		return "[nested value omitted]"
	}
	switch typed := value.(type) {
	case nil, bool, float64, float32, int, int32, int64, uint, uint32, uint64:
		return typed
	case string:
		if len(typed) > 2000 {
			return typed[:2000] + "…"
		}
		return typed
	case []any:
		if len(typed) > 20 {
			typed = typed[:20]
		}
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, boundedValue(item, depth+1))
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		count := 0
		for key, item := range typed {
			if count >= 30 {
				break
			}
			out[key] = boundedValue(item, depth+1)
			count++
		}
		return out
	default:
		return fmt.Sprint(typed)
	}
}
