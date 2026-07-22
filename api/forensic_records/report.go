package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type reportGenerateRequest struct {
	TenantID        string `json:"tenant_id"`
	UserID          string `json:"user_id"`
	CollectionID    string `json:"collection_id"`
	Target          string `json:"target"`
	IncludeEvidence bool   `json:"include_evidence"`
}

type reportGenerateResponse struct {
	CollectionID string         `json:"collection_id"`
	Target       string         `json:"target,omitempty"`
	Markdown     string         `json:"markdown"`
	Sections     map[string]any `json:"sections"`
	Warnings     []string       `json:"warnings,omitempty"`
	GeneratedAt  time.Time      `json:"generated_at"`
}

func reportGenerateHandler(cfg config, db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		if db == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
			return
		}
		var req reportGenerateRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("decode report request: %w", err))
			return
		}
		req.TenantID = defaultString(req.TenantID, "default")
		req.CollectionID = normalizeCollectionID(req.CollectionID)
		req.Target = strings.TrimSpace(req.Target)
		if req.CollectionID == "" {
			writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		base := hybridQueryRequest{
			TenantID:     req.TenantID,
			UserID:       req.UserID,
			CollectionID: req.CollectionID,
			Target:       req.Target,
			Limit:        10,
			MaxKBResults: 1,
		}

		sections := map[string]any{}
		overview, err := collectionOverview(ctx, db, base)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		sections["overview"] = overview
		frequent, _ := frequentContacts(ctx, db, base)
		temporal, _ := temporalActivity(ctx, db, base)
		locations, _ := topLocations(ctx, db, base)
		entities, _ := entityActivity(ctx, db, base)
		anpr, _ := anprSightings(ctx, db, base)
		relationships, _ := relationshipNetwork(ctx, db, base)
		timeline, _ := entityTimeline(ctx, db, base)
		sourceRows, _ := sourceRecords(ctx, db, base)
		schema, _ := schemaProfile(ctx, db, base)
		quality, _ := dataQuality(ctx, db, base)
		readiness, _ := caseReadiness(ctx, db, base)
		sections["frequent_contacts"] = frequent
		sections["temporal_activity"] = temporal
		sections["top_locations"] = locations
		sections["entity_activity"] = entities
		sections["anpr_sightings"] = anpr
		sections["relationship_network"] = relationships
		sections["entity_timeline"] = timeline
		sections["source_records"] = sourceRows
		sections["schema_profile"] = schema
		sections["data_quality"] = quality
		sections["case_readiness"] = readiness

		var warnings []string
		if req.IncludeEvidence {
			evidenceReq := base
			if req.Target != "" {
				evidenceReq.Query = "evidence for " + req.Target
			} else {
				evidenceReq.Query = "collection evidence"
			}
			evidence, warning := queryKnowledgeBaseEvidence(ctx, cfg, evidenceReq)
			if len(evidenceResults(evidence)) == 0 {
				if fallback := collectionAssetEvidence(ctx, db, evidenceReq); len(evidenceResults(fallback)) > 0 {
					evidence = fallback
					if warning != "" {
						warning += "; using structured KB asset catalog fallback"
					}
				}
			}
			sections["evidence"] = evidence
			if warning != "" {
				warnings = append(warnings, warning)
			}
		}

		generatedAt := time.Now().UTC()
		resp := reportGenerateResponse{
			CollectionID: req.CollectionID,
			Target:       req.Target,
			Sections:     sections,
			Warnings:     warnings,
			GeneratedAt:  generatedAt,
		}
		resp.Markdown = buildForensicMarkdown(resp)
		writeJSON(w, http.StatusOK, resp)
	}
}

func buildForensicMarkdown(report reportGenerateResponse) string {
	var b strings.Builder
	b.WriteString("## Forensic Intelligence Report\n\n")
	b.WriteString("- Generated At: " + report.GeneratedAt.Format(time.RFC3339) + "\n")
	b.WriteString("- Collection ID: " + markdownSafe(report.CollectionID) + "\n")
	if report.Target != "" {
		b.WriteString("- Target: " + markdownSafe(report.Target) + "\n")
	}
	b.WriteString("- Accuracy Guardrail: This report uses only computed records aggregates and retrieved KB source previews. No unsupported facts are inferred.\n\n")

	b.WriteString("### Executive Brief\n\n")
	b.WriteString("The collection was analyzed through deterministic records templates. Exact counts, rankings, timelines, and entity observations come from PostgreSQL/TimescaleDB. Evidence previews, when present, come from the Knowledge Base source entries.\n\n")
	writeExecutiveFindings(&b, report)
	writeAnalyticConfidence(&b, report)

	writeMapSection(&b, "Case Readiness", report.Sections, "case_readiness", "readiness")
	writeRowsSection(&b, "Case Readiness - Checks", report.Sections, "case_readiness", "readiness_checks", 12)
	writeRowsSection(&b, "Collection Overview - Jobs", report.Sections, "overview", "jobs", 8)
	writeRowsSection(&b, "Record Families", report.Sections, "overview", "record_families", 8)
	writeRowsSection(&b, "Schema Profile - Assets", report.Sections, "schema_profile", "assets", 8)
	writeMapSection(&b, "Data Quality - Summary", report.Sections, "data_quality", "summary")
	writeRowsSection(&b, "Data Quality - Recent Errors", report.Sections, "data_quality", "errors", 10)
	writeRowsSection(&b, "Frequent Contacts", report.Sections, "frequent_contacts", "frequent_contacts", 10)
	writeRowsSection(&b, "Temporal Activity - Hourly Baseline", report.Sections, "temporal_activity", "hourly_activity", 24)
	writeMapSection(&b, "Temporal Activity - Duration Stats", report.Sections, "temporal_activity", "duration_stats")
	writeRowsSection(&b, "Temporal Activity - Duration Extremes", report.Sections, "temporal_activity", "duration_extremes", 2)
	writeMapSection(&b, "Temporal Activity - Nocturnal Events", report.Sections, "temporal_activity", "nocturnal")
	writeRowsSection(&b, "Top Locations", report.Sections, "top_locations", "top_locations", 10)
	writeRowsSection(&b, "Entity Activity", report.Sections, "entity_activity", "entity_activity", 10)
	writeRowsSection(&b, "Relationship Network", report.Sections, "relationship_network", "relationship_network", 10)
	writeRowsSection(&b, "Entity Timeline", report.Sections, "entity_timeline", "entity_timeline", 15)
	writeRowsSection(&b, "ANPR Sightings", report.Sections, "anpr_sightings", "anpr_sightings", 10)
	writeRowsSection(&b, "Source Records - CDR", report.Sections, "source_records", "cdr_source_records", 8)
	writeRowsSection(&b, "Source Records - Generic", report.Sections, "source_records", "generic_source_records", 8)
	writeEvidenceSection(&b, report.Sections)
	writeLimitationsSection(&b, report)
	writeNextStepsSection(&b, report)

	if len(report.Warnings) > 0 {
		b.WriteString("### Warnings\n\n")
		for _, warning := range report.Warnings {
			b.WriteString("- " + markdownSafe(warning) + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func writeExecutiveFindings(b *strings.Builder, report reportGenerateResponse) {
	findings := deterministicFindings(report)
	if len(findings) == 0 {
		return
	}
	b.WriteString("### Key Findings\n\n")
	for _, finding := range findings {
		b.WriteString("- " + markdownSafe(finding) + "\n")
	}
	b.WriteString("\n")
}

func deterministicFindings(report reportGenerateResponse) []string {
	var findings []string
	if families := nestedRows(report.Sections, "overview", "record_families"); len(families) > 0 {
		findings = append(findings, fmt.Sprintf("Structured records are present across %d record family/families.", len(families)))
		if total := sumRowsInt(families, "total_rows", "inserted_rows"); total > 0 {
			findings = append(findings, fmt.Sprintf("The structured record index contains at least %d row(s) across available metadata.", total))
		}
	}
	if jobs := nestedRows(report.Sections, "overview", "jobs"); len(jobs) > 0 {
		findings = append(findings, fmt.Sprintf("%d source ingest job(s) are represented in the collection overview.", len(jobs)))
	}
	if contacts := nestedRows(report.Sections, "frequent_contacts", "frequent_contacts"); len(contacts) > 0 {
		top := contacts[0]
		findings = append(findings, fmt.Sprintf("Top CDR contact candidate: %s with %s interaction(s).", formatReportValue(top["dialed_number"]), formatReportValue(top["total_interactions"])))
	}
	if locations := nestedRows(report.Sections, "top_locations", "top_locations"); len(locations) > 0 {
		top := locations[0]
		findings = append(findings, fmt.Sprintf("Most observed location candidate: %s with %s observation(s).", formatReportValue(top["location"]), formatReportValue(top["observation_count"])))
	}
	if quality := nestedMap(report.Sections, "data_quality", "summary"); len(quality) > 0 {
		rejected := formatReportValue(quality["rejected_rows"])
		duplicates := formatReportValue(quality["duplicate_rows"])
		if rejected != "" || duplicates != "" {
			findings = append(findings, fmt.Sprintf("Data quality summary reports %s rejected row(s) and %s duplicate row(s).", defaultReportValue(rejected, "0"), defaultReportValue(duplicates, "0")))
		}
	}
	if readiness := nestedMap(report.Sections, "case_readiness", "readiness"); len(readiness) > 0 {
		findings = append(findings, fmt.Sprintf("Case readiness is %s with score %s.", defaultReportValue(formatReportValue(readiness["level"]), "unknown"), defaultReportValue(formatReportValue(readiness["score"]), "0")))
	}
	if evidence := nestedEvidenceRows(report.Sections); len(evidence) > 0 {
		findings = append(findings, fmt.Sprintf("%d Knowledge Base evidence preview(s) are attached to this report.", len(evidence)))
	}
	return findings
}

func writeAnalyticConfidence(b *strings.Builder, report reportGenerateResponse) {
	var statements []string
	if countReportRows(report.Sections) > 0 {
		statements = append(statements, "High for exact values shown in tables because they are computed by fixed SQL templates.")
	}
	if readiness := nestedMap(report.Sections, "case_readiness", "readiness"); len(readiness) > 0 {
		statements = append(statements, fmt.Sprintf("Operational readiness is %s based on deterministic ingest, quality, and source-traceability checks.", defaultReportValue(formatReportValue(readiness["level"]), "unknown")))
	}
	if len(nestedEvidenceRows(report.Sections)) > 0 {
		statements = append(statements, "Medium to high for contextual evidence previews, subject to KB indexing coverage and source document quality.")
	}
	if len(statements) == 0 {
		statements = append(statements, "Limited because no structured rows or KB evidence previews were returned.")
	}
	b.WriteString("### Confidence\n\n")
	for _, statement := range statements {
		b.WriteString("- " + statement + "\n")
	}
	b.WriteString("\n")
}

func writeLimitationsSection(b *strings.Builder, report reportGenerateResponse) {
	var limitations []string
	if countReportRows(report.Sections) == 0 {
		limitations = append(limitations, "No structured rows were available in the report sections.")
	}
	if len(nestedEvidenceRows(report.Sections)) == 0 {
		limitations = append(limitations, "No Knowledge Base evidence previews were returned or evidence inclusion was disabled.")
	}
	if quality := nestedMap(report.Sections, "data_quality", "summary"); len(quality) > 0 {
		if valueAsInt(quality["rejected_rows"]) > 0 {
			limitations = append(limitations, "Rejected rows exist and may affect completeness until corrected and reingested.")
		}
		if valueAsInt(quality["failed_jobs"]) > 0 {
			limitations = append(limitations, "At least one ingest job failed; affected source files should be reviewed before operational use.")
		}
	}
	if readiness := nestedMap(report.Sections, "case_readiness", "readiness"); len(readiness) > 0 {
		if level := formatReportValue(readiness["level"]); level == "blocked" || level == "needs_review" {
			limitations = append(limitations, "Case readiness checks indicate the collection needs remediation before high-confidence external use.")
		}
	}
	limitations = append(limitations, "The report does not infer facts outside returned records, source metadata, and KB evidence previews.")
	b.WriteString("### Limitations\n\n")
	for _, limitation := range limitations {
		b.WriteString("- " + markdownSafe(limitation) + "\n")
	}
	b.WriteString("\n")
}

func writeNextStepsSection(b *strings.Builder, report reportGenerateResponse) {
	steps := []string{
		"Review source rows and source files for every high-impact finding before external use.",
		"Resolve rejected rows, duplicate uploads, and failed ingest jobs, then rerun the affected deterministic templates.",
		"Use target-specific timeline, relationship-network, and source-record queries for entities that require deeper investigation.",
	}
	if len(nestedEvidenceRows(report.Sections)) == 0 {
		steps = append(steps, "Upload or reindex supporting policy/case-note documents if narrative evidence is required.")
	}
	b.WriteString("### Recommended Next Steps\n\n")
	for _, step := range steps {
		b.WriteString("- " + markdownSafe(step) + "\n")
	}
	b.WriteString("\n")
}

func writeRowsSection(b *strings.Builder, title string, sections map[string]any, sectionKey, rowsKey string, limit int) {
	rows := nestedRows(sections, sectionKey, rowsKey)
	if len(rows) == 0 {
		return
	}
	total := len(rows)
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	b.WriteString("### " + title + "\n\n")
	writeReportTable(b, reportColumns(rowsKey, rows), rows)
	if total > len(rows) {
		b.WriteString(fmt.Sprintf("\nShowing %d of %d rows. Use the raw API response for the complete result.\n", len(rows), total))
	}
	b.WriteString("\n")
}

func writeMapSection(b *strings.Builder, title string, sections map[string]any, sectionKey, mapKey string) {
	row := nestedMap(sections, sectionKey, mapKey)
	if len(row) == 0 {
		return
	}
	b.WriteString("### " + title + "\n\n")
	writeReportFacts(b, row)
	b.WriteString("\n")
}

func writeEvidenceSection(b *strings.Builder, sections map[string]any) {
	evidence, ok := sections["evidence"].(map[string]any)
	if !ok {
		return
	}
	rows, _ := evidence["results"].([]map[string]any)
	if len(rows) == 0 {
		return
	}
	b.WriteString("### Knowledge Base Evidence\n\n")
	for i, row := range rows {
		entry := reportEvidenceSource(row)
		preview := formatReportValue(firstReportValue(row, "preview", "content"))
		if entry == "" {
			entry = "Knowledge Base source"
		}
		b.WriteString(fmt.Sprintf("%d. Source: %s", i+1, markdownSafe(entry)))
		if similarity := formatReportValue(row["similarity"]); similarity != "" && similarity != "0" {
			b.WriteString(" (similarity " + markdownSafe(similarity) + ")")
		}
		b.WriteString("\n\n")
		if preview != "" {
			b.WriteString(markdownSafeEvidencePreview(preview, 900) + "\n\n")
		}
	}
}

func reportEvidenceSource(row map[string]any) string {
	if source := formatReportValue(firstReportValue(row, "entry", "source", "id")); source != "" {
		return source
	}
	metadata, _ := row["metadata"].(map[string]any)
	return formatReportValue(firstReportValue(metadata, "file_name", "source", "source_entry"))
}

func firstReportValue(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value
		}
	}
	return nil
}

func writeReportTable(b *strings.Builder, columns []string, rows []map[string]any) {
	if len(columns) == 0 || len(rows) == 0 {
		return
	}
	b.WriteString("| ")
	b.WriteString(strings.Join(columns, " | "))
	b.WriteString(" |\n| ")
	b.WriteString(strings.Join(repeatMarkdown("---", len(columns)), " | "))
	b.WriteString(" |\n")
	for _, row := range rows {
		cells := make([]string, 0, len(columns))
		for _, column := range columns {
			cells = append(cells, markdownTableCell(formatReportValue(row[column])))
		}
		b.WriteString("| ")
		b.WriteString(strings.Join(cells, " | "))
		b.WriteString(" |\n")
	}
}

func writeReportFacts(b *strings.Builder, row map[string]any) {
	keys := make([]string, 0, len(row))
	for key, value := range row {
		if value != nil && !skipVerboseReportField(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		b.WriteString("- **" + markdownSafe(key) + ":** " + markdownSafe(formatReportValue(row[key])) + "\n")
	}
}

func reportColumns(rowsKey string, rows []map[string]any) []string {
	preferred := map[string][]string{
		"readiness_checks":       {"check", "status", "score", "count"},
		"jobs":                   {"source_file", "record_type", "status", "total_rows", "accepted_rows", "duplicate_rows", "rejected_rows", "completed_at"},
		"record_families":        {"record_type", "batch_count", "total_rows", "inserted_rows", "duplicate_rows", "rejected_rows"},
		"assets":                 {"source_file", "detected_record_type", "structured_status", "rag_status", "storage_mode", "updated_at"},
		"errors":                 {"source_file", "record_type", "row_number", "error_message", "created_at"},
		"frequent_contacts":      {"dialed_number", "incoming_count", "outgoing_count", "total_interactions", "first_contact", "last_contact"},
		"hourly_activity":        {"hour_of_day", "event_count"},
		"duration_extremes":      {"metric", "duration_seconds", "observed_at", "call_type", "direction", "call_org_num", "call_dialed_num", "location", "source_file", "row_number"},
		"top_locations":          {"location", "cell_site_id", "observation_count", "first_seen", "last_seen"},
		"entity_activity":        {"entity_type", "entity_value", "observation_count", "first_seen", "last_seen", "record_types"},
		"relationship_network":   {"entity_type", "entity_value", "co_observation_count", "source_field", "first_seen", "last_seen"},
		"entity_timeline":        {"event_time", "event_label", "location", "primary_entity", "secondary_entity", "source_file"},
		"anpr_sightings":         {"observed_at", "plate_number", "camera_id", "location", "latitude", "longitude", "source_file"},
		"cdr_source_records":     {"observed_at", "msisdn", "call_type", "direction", "call_org_num", "call_dialed_num", "location"},
		"generic_source_records": {"observed_at", "record_type", "primary_entity", "secondary_entity", "location", "source_file"},
	}
	if columns := presentReportColumns(rows, preferred[rowsKey]...); len(columns) > 0 {
		return columns
	}
	return fallbackReportColumns(rows, 6)
}

func presentReportColumns(rows []map[string]any, candidates ...string) []string {
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

func fallbackReportColumns(rows []map[string]any, max int) []string {
	seen := map[string]bool{}
	var columns []string
	for _, row := range rows {
		keys := make([]string, 0, len(row))
		for key := range row {
			if !skipVerboseReportField(key) {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			if !seen[key] {
				columns = append(columns, key)
				seen[key] = true
				if len(columns) >= max {
					return columns
				}
			}
		}
	}
	return columns
}

func skipVerboseReportField(key string) bool {
	switch key {
	case "headers", "quality_report", "metadata_quality_report", "routing_decision", "normalized_schema", "sample_rows":
		return true
	default:
		return false
	}
}

func nestedRows(sections map[string]any, sectionKey, rowsKey string) []map[string]any {
	section, ok := sections[sectionKey].(map[string]any)
	if !ok {
		return nil
	}
	rows, _ := section[rowsKey].([]map[string]any)
	return rows
}

func nestedMap(sections map[string]any, sectionKey, mapKey string) map[string]any {
	section, ok := sections[sectionKey].(map[string]any)
	if !ok {
		return nil
	}
	row, _ := section[mapKey].(map[string]any)
	return row
}

func nestedEvidenceRows(sections map[string]any) []map[string]any {
	evidence, ok := sections["evidence"].(map[string]any)
	if !ok {
		return nil
	}
	return evidenceResults(evidence)
}

func countReportRows(value any) int {
	switch typed := value.(type) {
	case nil:
		return 0
	case []map[string]any:
		return len(typed)
	case []any:
		count := 0
		for _, item := range typed {
			count += countReportRows(item)
		}
		return count
	case map[string]any:
		count := 0
		for key, item := range typed {
			if key == "evidence" {
				continue
			}
			count += countReportRows(item)
		}
		return count
	default:
		return 0
	}
}

func sumRowsInt(rows []map[string]any, keys ...string) int64 {
	var total int64
	for _, row := range rows {
		for _, key := range keys {
			value := valueAsInt(row[key])
			if value > 0 {
				total += value
				break
			}
		}
	}
	return total
}

func valueAsInt(value any) int64 {
	switch v := value.(type) {
	case nil:
		return 0
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		if v > uint64(^uint64(0)>>1) {
			return 0
		}
		return int64(v)
	case float32:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil {
			return parsed
		}
		return 0
	case pgtype.Numeric:
		parsed, err := strconv.ParseInt(formatPGNumeric(v), 10, 64)
		if err == nil {
			return parsed
		}
		return 0
	default:
		return 0
	}
}

func defaultReportValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func compactRow(row map[string]any) string {
	var parts []string
	for key, value := range row {
		if value == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, formatReportValue(value)))
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func formatReportValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case time.Time:
		return v.UTC().Format(time.RFC3339)
	case pgtype.Numeric:
		return formatPGNumeric(v)
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case []string:
		return joinReportValues(v)
	case []any:
		values := make([]string, 0, len(v))
		for _, item := range v {
			if formatted := formatReportValue(item); formatted != "" {
				values = append(values, formatted)
			}
		}
		return joinReportValues(values)
	case map[string]any:
		return summarizeReportMap(v)
	default:
		return fmt.Sprint(v)
	}
}

func markdownSafe(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func markdownTableCell(value string) string {
	value = markdownSafe(value)
	value = strings.ReplaceAll(value, "|", "\\|")
	if value == "" {
		return "-"
	}
	return value
}

func markdownSafeEvidencePreview(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	value = strings.TrimLeft(value, "#> \t")
	value = strings.ReplaceAll(value, "`", "'")
	value = truncate(value, limit)
	if strings.TrimSpace(value) == "" {
		return "Preview unavailable."
	}
	return value
}

func joinReportValues(values []string) string {
	const maxValues = 5
	if len(values) == 0 {
		return ""
	}
	if len(values) <= maxValues {
		return strings.Join(values, ", ")
	}
	return strings.Join(values[:maxValues], ", ") + fmt.Sprintf(" +%d more", len(values)-maxValues)
}

func summarizeReportMap(values map[string]any) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	const maxPairs = 5
	parts := make([]string, 0, minInt(len(keys), maxPairs))
	for _, key := range keys {
		formatted := formatReportValue(values[key])
		if formatted == "" {
			continue
		}
		parts = append(parts, key+"="+formatted)
		if len(parts) >= maxPairs {
			break
		}
	}
	if len(keys) > maxPairs {
		parts = append(parts, fmt.Sprintf("+%d more", len(keys)-maxPairs))
	}
	return strings.Join(parts, "; ")
}

func repeatMarkdown(value string, count int) []string {
	out := make([]string, count)
	for i := range out {
		out[i] = value
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
