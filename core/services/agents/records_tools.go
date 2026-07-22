package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mudler/LocalAI/core/services/records"
	"github.com/mudler/LocalAI/pkg/httpclient"
)

const RecordsToolsPolicy = `Structured Records Intelligence is available for exact analytics.
Use records_query, records_aggregate, records_schema, records_explain_batch, cdr_query, anpr_query, or correlate_records for exact counts, filters, rankings, shortest/longest/min/max, distinct values, and time-window correlations over ingested CSV/JSON/JSONL/TSV/log records.
Use Knowledge Base search for evidence text, previews, summaries, and explanation context.
Never infer exact structured values from semantic retrieval alone; if the needed batch or field is missing, say what data or tool input is missing.
Cite batch_id, source_file, source_entry, and row_number when the tool returns them.`

const ForensicRecordsToolsPolicy = `Enterprise Forensic Records Intelligence is available through forensic_hybrid_query, forensic_query_templates, forensic_list_evidence, forensic_get_evidence, and forensic_generate_report.
Use forensic_hybrid_query for natural-language analyst questions over a Knowledge Base collection, including CDR, ANPR, IPDR, subscriber, tower, transaction, access-log, policy, document, and generic evidence collections.
Use forensic_list_evidence to audit what evidence exists in a collection and forensic_get_evidence to inspect one evidence item, linked ingest jobs, KB assets, canonical row previews, and entity rollups.
Use canonical_records through forensic_hybrid_query for source-of-truth filters over any record family, entity, source_file, batch_id, raw_payload field, or field-exists/not-exists question.
The forensic API routes exact analytics to read-only SQL templates and evidence/context lookup to the Knowledge Base. Do not invent counts, timelines, relationships, attributes, or quality statistics from retrieved text alone.
Use forensic_query_templates when you are unsure which deterministic template exists. Use forensic_generate_report for a cited Markdown case brief after the exact query results are available.
Always include tenant_id and collection_id when known, and preserve warnings such as KB vector fallback in the final answer.`

type ForensicCanonicalPayloadFilter struct {
	Field  string   `json:"field" jsonschema:"description=Top-level raw_payload field name. Matching is exact or case-insensitive for heterogeneous source headers."`
	Op     string   `json:"op" jsonschema:"description=Operator: eq, contains, in, exists, or not_exists"`
	Value  string   `json:"value,omitempty" jsonschema:"description=Single comparison value as text"`
	Values []string `json:"values,omitempty" jsonschema:"description=Multiple values for the in operator"`
}

type ForensicCanonicalFieldFilter struct {
	Field string `json:"field" jsonschema:"description=Top-level raw_payload field name. Matching is exact or case-insensitive for heterogeneous source headers."`
	Op    string `json:"op" jsonschema:"description=Operator: exists or not_exists"`
}

type ForensicHybridQueryArgs struct {
	TenantID          string                           `json:"tenant_id,omitempty" jsonschema:"description=Tenant ID, defaults to the agent forensic_tenant_id or default"`
	UserID            string                           `json:"user_id,omitempty" jsonschema:"description=Optional user ID for scoped KB evidence lookup"`
	CollectionID      string                           `json:"collection_id,omitempty" jsonschema:"description=Knowledge Base collection/case ID"`
	Query             string                           `json:"query" jsonschema:"description=Natural-language analyst question"`
	Target            string                           `json:"target,omitempty" jsonschema:"description=Optional extracted target such as phone, plate, IP, IMSI, IMEI, location, or call type"`
	Targets           []string                         `json:"targets,omitempty" jsonschema:"description=Optional multiple targets to match against primary_target and secondary_target"`
	Template          string                           `json:"template,omitempty" jsonschema:"description=Optional deterministic template name, for example canonical_records, entity_activity, relationship_network, entity_timeline, source_records, schema_profile, data_quality, evidence"`
	RecordType        string                           `json:"record_type,omitempty" jsonschema:"description=Optional canonical record type such as cdr, anpr, ipdr, subscriber, tower_location, transaction, access_log, or generic"`
	DateFrom          string                           `json:"date_from,omitempty" jsonschema:"description=Optional inclusive timestamp lower bound in RFC3339 format"`
	DateTo            string                           `json:"date_to,omitempty" jsonschema:"description=Optional exclusive timestamp upper bound in RFC3339 format"`
	SourceFile        string                           `json:"source_file,omitempty" jsonschema:"description=Optional exact source file filter"`
	BatchID           string                           `json:"batch_id,omitempty" jsonschema:"description=Optional exact batch UUID/text filter"`
	RawPayloadFilters []ForensicCanonicalPayloadFilter `json:"raw_payload_filters,omitempty" jsonschema:"description=Canonical raw_payload filters for source-specific attributes/properties"`
	FieldFilters      []ForensicCanonicalFieldFilter   `json:"field_filters,omitempty" jsonschema:"description=Canonical raw_payload field exists/not_exists filters"`
	FieldExists       []string                         `json:"field_exists,omitempty" jsonschema:"description=Raw payload fields that must exist, exact or case-insensitive"`
	FieldNotExists    []string                         `json:"field_not_exists,omitempty" jsonschema:"description=Raw payload fields that must not exist, exact or case-insensitive"`
	Offset            int                              `json:"offset,omitempty" jsonschema:"description=Pagination offset for canonical_records"`
	SortBy            string                           `json:"sort_by,omitempty" jsonschema:"description=Sort column for canonical_records: timestamp, record_type, primary_target, secondary_target, source_file, row_number, or ingested_at"`
	SortDirection     string                           `json:"sort_direction,omitempty" jsonschema:"description=Sort direction asc or desc"`
	Limit             int                              `json:"limit,omitempty" jsonschema:"description=Maximum structured rows or aggregate rows to return"`
	MaxKBResults      int                              `json:"max_kb_results,omitempty" jsonschema:"description=Maximum KB evidence results to include"`
	SynthesisModel    string                           `json:"synthesis_model,omitempty" jsonschema:"description=Optional LocalAI chat model to use for bounded synthesis. If omitted, the sidecar uses FORENSIC_SYNTHESIS_MODEL or deterministic fallback."`
}

type ForensicReportArgs struct {
	TenantID        string `json:"tenant_id,omitempty" jsonschema:"description=Tenant ID, defaults to the agent forensic_tenant_id or default"`
	UserID          string `json:"user_id,omitempty" jsonschema:"description=Optional user ID for scoped KB evidence lookup"`
	CollectionID    string `json:"collection_id,omitempty" jsonschema:"description=Knowledge Base collection/case ID"`
	Target          string `json:"target,omitempty" jsonschema:"description=Optional report target such as phone, plate, IP, IMSI, IMEI, or location"`
	IncludeEvidence bool   `json:"include_evidence,omitempty" jsonschema:"description=Whether to include KB evidence previews and citations"`
}

type ForensicListEvidenceArgs struct {
	TenantID         string `json:"tenant_id,omitempty" jsonschema:"description=Tenant ID, defaults to the agent forensic_tenant_id or default"`
	CollectionID     string `json:"collection_id,omitempty" jsonschema:"description=Knowledge Base collection/case ID"`
	CaseID           string `json:"case_id,omitempty" jsonschema:"description=Optional case/workspace identifier"`
	Modality         string `json:"modality,omitempty" jsonschema:"description=Optional modality filter such as structured_records, document, image, audio, video, text, or unknown"`
	DetectedType     string `json:"detected_type,omitempty" jsonschema:"description=Optional detected type such as cdr, anpr, ipdr, subscriber, pdf, image, or generic"`
	ProcessingStatus string `json:"processing_status,omitempty" jsonschema:"description=Optional status filter such as queued, processing, completed, failed, skipped, duplicate, or registered"`
	Query            string `json:"q,omitempty" jsonschema:"description=Optional filename, hash, KB entry, metadata, or source search text"`
	Limit            int    `json:"limit,omitempty" jsonschema:"description=Maximum evidence items to return"`
	Offset           int    `json:"offset,omitempty" jsonschema:"description=Pagination offset"`
}

type ForensicGetEvidenceArgs struct {
	TenantID   string `json:"tenant_id,omitempty" jsonschema:"description=Tenant ID, defaults to the agent forensic_tenant_id or default"`
	EvidenceID string `json:"evidence_id" jsonschema:"description=Evidence UUID to inspect"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Maximum canonical row previews to return"`
}

type ForensicQueryTemplatesArgs struct{}

type ForensicRecordsToolConfig struct {
	APIURL       string
	APIKey       string
	UserID       string
	TenantID     string
	CollectionID string
	Model        string
}

type ForensicHybridQueryTool struct {
	ForensicRecordsToolConfig
}

func (t ForensicHybridQueryTool) Run(args ForensicHybridQueryArgs) (string, any, error) {
	args = t.defaultsHybridArgs(args)
	var resp map[string]any
	if err := forensicPost(t.APIURL, t.APIKey, "/query/hybrid", args, &resp); err != nil {
		return "", nil, err
	}
	return marshalToolResult(resp), resp, nil
}

type ForensicQueryTemplatesTool struct {
	ForensicRecordsToolConfig
}

func (t ForensicQueryTemplatesTool) Run(_ ForensicQueryTemplatesArgs) (string, any, error) {
	var resp map[string]any
	if err := forensicGet(t.APIURL, t.APIKey, "/query/templates", &resp); err != nil {
		return "", nil, err
	}
	return marshalToolResult(resp), resp, nil
}

type ForensicGenerateReportTool struct {
	ForensicRecordsToolConfig
}

func (t ForensicGenerateReportTool) Run(args ForensicReportArgs) (string, any, error) {
	args = t.defaultsReportArgs(args)
	var resp map[string]any
	if err := forensicPost(t.APIURL, t.APIKey, "/reports/generate", args, &resp); err != nil {
		return "", nil, err
	}
	return marshalToolResult(resp), resp, nil
}

type ForensicListEvidenceTool struct {
	ForensicRecordsToolConfig
}

func (t ForensicListEvidenceTool) Run(args ForensicListEvidenceArgs) (string, any, error) {
	args = t.defaultsListEvidenceArgs(args)
	values := url.Values{}
	values.Set("tenant_id", args.TenantID)
	values.Set("collection_id", args.CollectionID)
	for _, pair := range []struct {
		key   string
		value string
	}{
		{"case_id", args.CaseID},
		{"modality", args.Modality},
		{"detected_type", args.DetectedType},
		{"processing_status", args.ProcessingStatus},
		{"q", args.Query},
	} {
		if strings.TrimSpace(pair.value) != "" {
			values.Set(pair.key, pair.value)
		}
	}
	if args.Limit > 0 {
		values.Set("limit", fmt.Sprint(args.Limit))
	}
	if args.Offset > 0 {
		values.Set("offset", fmt.Sprint(args.Offset))
	}
	var resp map[string]any
	if err := forensicGetQuery(t.APIURL, t.APIKey, "/evidence", values, &resp); err != nil {
		return "", nil, err
	}
	return marshalToolResult(resp), resp, nil
}

type ForensicGetEvidenceTool struct {
	ForensicRecordsToolConfig
}

func (t ForensicGetEvidenceTool) Run(args ForensicGetEvidenceArgs) (string, any, error) {
	args = t.defaultsGetEvidenceArgs(args)
	values := url.Values{}
	values.Set("tenant_id", args.TenantID)
	if args.Limit > 0 {
		values.Set("limit", fmt.Sprint(args.Limit))
	}
	var resp map[string]any
	if err := forensicGetQuery(t.APIURL, t.APIKey, "/evidence/"+url.PathEscape(args.EvidenceID), values, &resp); err != nil {
		return "", nil, err
	}
	return marshalToolResult(resp), resp, nil
}

func (t ForensicRecordsToolConfig) defaultsHybridArgs(args ForensicHybridQueryArgs) ForensicHybridQueryArgs {
	if strings.TrimSpace(args.TenantID) == "" {
		args.TenantID = defaultForensicTenant(t.TenantID)
	}
	if strings.TrimSpace(args.UserID) == "" {
		args.UserID = t.UserID
	}
	if strings.TrimSpace(args.CollectionID) == "" {
		args.CollectionID = t.CollectionID
	}
	if args.Limit <= 0 {
		args.Limit = 20
	}
	if args.MaxKBResults <= 0 {
		args.MaxKBResults = 3
	}
	if strings.TrimSpace(args.SynthesisModel) == "" {
		args.SynthesisModel = strings.TrimSpace(t.Model)
	}
	return args
}

func (t ForensicRecordsToolConfig) defaultsListEvidenceArgs(args ForensicListEvidenceArgs) ForensicListEvidenceArgs {
	if strings.TrimSpace(args.TenantID) == "" {
		args.TenantID = defaultForensicTenant(t.TenantID)
	}
	if strings.TrimSpace(args.CollectionID) == "" {
		args.CollectionID = t.CollectionID
	}
	if args.Limit <= 0 {
		args.Limit = 50
	}
	return args
}

func (t ForensicRecordsToolConfig) defaultsGetEvidenceArgs(args ForensicGetEvidenceArgs) ForensicGetEvidenceArgs {
	if strings.TrimSpace(args.TenantID) == "" {
		args.TenantID = defaultForensicTenant(t.TenantID)
	}
	if args.Limit <= 0 {
		args.Limit = 25
	}
	return args
}

func (t ForensicRecordsToolConfig) defaultsReportArgs(args ForensicReportArgs) ForensicReportArgs {
	if strings.TrimSpace(args.TenantID) == "" {
		args.TenantID = defaultForensicTenant(t.TenantID)
	}
	if strings.TrimSpace(args.UserID) == "" {
		args.UserID = t.UserID
	}
	if strings.TrimSpace(args.CollectionID) == "" {
		args.CollectionID = t.CollectionID
	}
	return args
}

func defaultForensicTenant(tenantID string) string {
	if strings.TrimSpace(tenantID) == "" {
		return "default"
	}
	return tenantID
}

type AgentKeyValue struct {
	Key   string `json:"key" jsonschema:"description=Argument name"`
	Value string `json:"value" jsonschema:"description=Argument value as a string; the records service will compare and parse as needed"`
}

type AgentRecordFilter struct {
	Field  string   `json:"field" jsonschema:"description=Field name or alias"`
	Op     string   `json:"op,omitempty" jsonschema:"description=Operator: eq, ne, contains, gt, gte, lt, lte, in, exists, or not_exists"`
	Value  string   `json:"value,omitempty" jsonschema:"description=Single comparison value as text"`
	Values []string `json:"values,omitempty" jsonschema:"description=Multiple values for the in operator"`
}

type AgentRecordsQueryArgs struct {
	BatchIDs       []string            `json:"batch_ids,omitempty" jsonschema:"description=Restrict query to these batch IDs"`
	CollectionName string              `json:"collection_name,omitempty" jsonschema:"description=Restrict query to a collection"`
	RecordType     string              `json:"record_type,omitempty" jsonschema:"description=Record type such as cdr, anpr, ipdr, subscriber, tower_location, transaction, access_log, or generic"`
	Filters        []AgentRecordFilter `json:"filters,omitempty" jsonschema:"description=Additional exact filters"`
	TimeRange      *records.TimeRange  `json:"time_range,omitempty" jsonschema:"description=Optional timestamp range"`
	Sort           []records.SortField `json:"sort,omitempty" jsonschema:"description=Sort fields"`
	Fields         []string            `json:"fields,omitempty" jsonschema:"description=Selected output fields"`
	Limit          int                 `json:"limit,omitempty" jsonschema:"description=Maximum records to return"`
	Offset         int                 `json:"offset,omitempty" jsonschema:"description=Pagination offset"`
	Helper         string              `json:"helper,omitempty" jsonschema:"description=Helper query name, for example shortest_call, longest_call, calls_between_numbers, plate_search, sightings_by_time_range"`
	HelperArgs     []AgentKeyValue     `json:"helper_args,omitempty" jsonschema:"description=Helper arguments as key/value strings"`
}

type AgentRecordsAggregateArgs struct {
	AgentRecordsQueryArgs
	Operation string   `json:"operation" jsonschema:"description=Aggregate operation: count, distinct, min, max, top, min_by_field, or max_by_field"`
	Field     string   `json:"field,omitempty" jsonschema:"description=Field to aggregate"`
	GroupBy   []string `json:"group_by,omitempty" jsonschema:"description=Fields to group by"`
	TopN      int      `json:"top_n,omitempty" jsonschema:"description=Maximum groups or values to return"`
}

type AgentCorrelateRecordsArgs struct {
	Left              AgentRecordsQueryArgs `json:"left" jsonschema:"description=Left records query"`
	Right             AgentRecordsQueryArgs `json:"right" jsonschema:"description=Right records query"`
	EntityFields      []string              `json:"entity_fields,omitempty" jsonschema:"description=Fields used as shared entity keys"`
	TimeField         string                `json:"time_field,omitempty" jsonschema:"description=Timestamp field used for time-window matching"`
	TimeWindowSeconds int                   `json:"time_window_seconds,omitempty" jsonschema:"description=Maximum time delta in seconds"`
	Limit             int                   `json:"limit,omitempty" jsonschema:"description=Maximum correlation matches"`
}

func (a AgentRecordsQueryArgs) ToRecordsQueryRequest() records.QueryRequest {
	return records.QueryRequest{
		BatchIDs:       a.BatchIDs,
		CollectionName: a.CollectionName,
		RecordType:     a.RecordType,
		Filters:        agentFiltersToRecords(a.Filters),
		TimeRange:      a.TimeRange,
		Sort:           a.Sort,
		Fields:         a.Fields,
		Limit:          a.Limit,
		Offset:         a.Offset,
		Helper:         a.Helper,
		HelperArgs:     keyValuesToMap(a.HelperArgs),
	}
}

func (a AgentRecordsAggregateArgs) ToRecordsAggregateRequest() records.AggregateRequest {
	return records.AggregateRequest{
		QueryRequest: a.AgentRecordsQueryArgs.ToRecordsQueryRequest(),
		Operation:    a.Operation,
		Field:        a.Field,
		GroupBy:      a.GroupBy,
		TopN:         a.TopN,
	}
}

func (a AgentCorrelateRecordsArgs) ToRecordsCorrelateRequest() records.CorrelateRequest {
	return records.CorrelateRequest{
		Left:              a.Left.ToRecordsQueryRequest(),
		Right:             a.Right.ToRecordsQueryRequest(),
		EntityFields:      a.EntityFields,
		TimeField:         a.TimeField,
		TimeWindowSeconds: a.TimeWindowSeconds,
		Limit:             a.Limit,
	}
}

func agentFiltersToRecords(filters []AgentRecordFilter) []records.Filter {
	out := make([]records.Filter, 0, len(filters))
	for _, filter := range filters {
		values := make([]any, 0, len(filter.Values))
		for _, value := range filter.Values {
			values = append(values, value)
		}
		out = append(out, records.Filter{
			Field:  filter.Field,
			Op:     filter.Op,
			Value:  filter.Value,
			Values: values,
		})
	}
	return out
}

func keyValuesToMap(values []AgentKeyValue) map[string]any {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]any, len(values))
	for _, value := range values {
		if strings.TrimSpace(value.Key) == "" {
			continue
		}
		out[value.Key] = value.Value
	}
	return out
}

type RecordsQueryTool struct {
	APIURL string
	APIKey string
	UserID string
}

func (t RecordsQueryTool) Run(args AgentRecordsQueryArgs) (string, any, error) {
	var resp records.QueryResponse
	if err := recordsPost(t.APIURL, t.APIKey, t.UserID, "/api/records/query", args.ToRecordsQueryRequest(), &resp); err != nil {
		return "", nil, err
	}
	return marshalToolResult(resp), resp, nil
}

type RecordsAggregateTool struct {
	APIURL string
	APIKey string
	UserID string
}

func (t RecordsAggregateTool) Run(args AgentRecordsAggregateArgs) (string, any, error) {
	var resp records.AggregateResponse
	if err := recordsPost(t.APIURL, t.APIKey, t.UserID, "/api/records/aggregate", args.ToRecordsAggregateRequest(), &resp); err != nil {
		return "", nil, err
	}
	return marshalToolResult(resp), resp, nil
}

type RecordsSchemaArgs struct {
	RecordType string `json:"record_type" jsonschema:"description=Record type such as cdr, anpr, ipdr, subscriber, tower_location, transaction, access_log, or generic"`
}

type RecordsSchemaTool struct {
	APIURL string
	APIKey string
	UserID string
}

func (t RecordsSchemaTool) Run(args RecordsSchemaArgs) (string, any, error) {
	var resp records.SchemaResponse
	if err := recordsGet(t.APIURL, t.APIKey, t.UserID, "/api/records/schema/"+url.PathEscape(args.RecordType), &resp); err != nil {
		return "", nil, err
	}
	return marshalToolResult(resp), resp, nil
}

type RecordsExplainBatchArgs struct {
	BatchID string `json:"batch_id" jsonschema:"description=Records batch ID to explain"`
}

type RecordsExplainBatchTool struct {
	APIURL string
	APIKey string
	UserID string
}

func (t RecordsExplainBatchTool) Run(args RecordsExplainBatchArgs) (string, any, error) {
	var resp records.Batch
	if err := recordsGet(t.APIURL, t.APIKey, t.UserID, "/api/records/batches/"+url.PathEscape(args.BatchID), &resp); err != nil {
		return "", nil, err
	}
	return marshalToolResult(resp), resp, nil
}

type TypedRecordsQueryArgs struct {
	AgentRecordsQueryArgs
}

type CDRQueryTool struct {
	APIURL string
	APIKey string
	UserID string
}

func (t CDRQueryTool) Run(args TypedRecordsQueryArgs) (string, any, error) {
	req := typedRecordsRequest(records.RecordTypeCDR, args)
	var resp records.QueryResponse
	if err := recordsPost(t.APIURL, t.APIKey, t.UserID, "/api/records/query", req, &resp); err != nil {
		return "", nil, err
	}
	return marshalToolResult(resp), resp, nil
}

type ANPRQueryTool struct {
	APIURL string
	APIKey string
	UserID string
}

func (t ANPRQueryTool) Run(args TypedRecordsQueryArgs) (string, any, error) {
	req := typedRecordsRequest(records.RecordTypeANPR, args)
	var resp records.QueryResponse
	if err := recordsPost(t.APIURL, t.APIKey, t.UserID, "/api/records/query", req, &resp); err != nil {
		return "", nil, err
	}
	return marshalToolResult(resp), resp, nil
}

type CorrelateRecordsTool struct {
	APIURL string
	APIKey string
	UserID string
}

func (t CorrelateRecordsTool) Run(args AgentCorrelateRecordsArgs) (string, any, error) {
	var resp records.CorrelateResponse
	if err := recordsPost(t.APIURL, t.APIKey, t.UserID, "/api/records/correlate", args.ToRecordsCorrelateRequest(), &resp); err != nil {
		return "", nil, err
	}
	return marshalToolResult(resp), resp, nil
}

func typedRecordsRequest(recordType string, args TypedRecordsQueryArgs) records.QueryRequest {
	req := args.ToRecordsQueryRequest()
	req.RecordType = recordType
	return req
}

func recordsPost(apiURL, apiKey, userID, path string, body any, dst any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, recordsURL(apiURL, path, userID), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return doRecordsRequest(req, dst)
}

func recordsGet(apiURL, apiKey, userID, path string, dst any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, recordsURL(apiURL, path, userID), nil)
	if err != nil {
		return err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return doRecordsRequest(req, dst)
}

func forensicPost(apiURL, apiKey, path string, body any, dst any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, forensicURL(apiURL, path), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return doRecordsRequest(req, dst)
}

func forensicGet(apiURL, apiKey, path string, dst any) error {
	return forensicGetQuery(apiURL, apiKey, path, nil, dst)
}

func forensicGetQuery(apiURL, apiKey, path string, query url.Values, dst any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	target := forensicURL(apiURL, path)
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return doRecordsRequest(req, dst)
}

func recordsURL(apiURL, path, userID string) string {
	u := strings.TrimRight(apiURL, "/") + path
	if userID != "" {
		values := url.Values{}
		values.Set("user_id", userID)
		u += "?" + values.Encode()
	}
	return u
}

func forensicURL(apiURL, path string) string {
	return strings.TrimRight(apiURL, "/") + path
}

func doRecordsRequest(req *http.Request, dst any) error {
	resp, err := httpclient.New().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("records API failed (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func marshalToolResult(value any) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}
