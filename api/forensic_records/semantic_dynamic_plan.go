package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/mudler/LocalAI/pkg/httpclient"
)

const semanticDynamicOperation = "dynamic.typed_algebra"

type semanticDynamicFilter struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
}
type semanticDynamicProposal struct {
	Mode          string                  `json:"mode"`
	Projection    []string                `json:"projection"`
	Filters       []semanticDynamicFilter `json:"filters"`
	Group         *CanonicalGroupV1       `json:"group"`
	Compare       *CanonicalCompareV1     `json:"compare"`
	SortBy        string                  `json:"sort_by"`
	SortDirection string                  `json:"sort_direction"`
	SourceNative  *SourceNativePlanV1     `json:"source_native"`
}

func semanticDynamicFamilyAllowed(req hybridQueryRequest, family string) bool {
	// The same canonical executor used by typed API plans owns scope and row
	// policy. Media observations must use their specialized truth contracts.
	switch family {
	case "communications_cdr", "network_ipdr", "anpr_vehicles", "subscriber_identity", "tower_location", "financial_transactions", "access_security_logs", "generic_tabular", "case_cross_family":
		t, ok := queryTemplateByName("canonical_records")
		return ok && t.ExposureStatus != "engineering_only" && semanticTemplateMatchesScope(t, req)
	}
	return false
}

var semanticQuotedValue = regexp.MustCompile(`"([^"\r\n]{1,512})"|'([^'\r\n]{1,512})'`)

func semanticLiteralValues(req hybridQueryRequest) []string {
	values := []string{}
	for _, m := range semanticQuotedValue.FindAllStringSubmatch(req.Query, -1) {
		v := m[1]
		if v == "" {
			v = m[2]
		}
		values = append(values, v)
	}
	return capabilityUniqueSortedStrings(values)
}

func applySemanticDynamicPlan(req hybridQueryRequest, family string, p semanticDynamicProposal) (hybridQueryRequest, error) {
	if p.Mode == "source_native" {
		return req, fmt.Errorf("source-native algebra requires an issued field catalog")
	}
	if p.SourceNative != nil {
		return req, fmt.Errorf("legacy dynamic mode cannot carry source-native algebra")
	}
	if !semanticDynamicFamilyAllowed(req, family) {
		return req, fmt.Errorf("dynamic algebra unavailable in family/scope")
	}
	bound, facts := extractHybridFacts(req)
	if facts.State != "" {
		return req, fmt.Errorf("%s", facts.State)
	}
	if req.Group != nil || req.Compare != nil || len(req.Projection) > 0 || req.SortBy != "" || req.Offset != 0 {
		return req, fmt.Errorf("explicit algebra must use the typed path")
	}
	bound.Template = "canonical_records"
	bound.Target = defaultString(req.Target, facts.Target)
	bound.Targets = canonicalTargetSet(bound.Target, req.Targets, facts.Targets)
	if p.Mode != "compare" {
		bound.DateFrom = defaultString(req.DateFrom, facts.DateFrom)
		bound.DateTo = defaultString(req.DateTo, facts.DateTo)
	}
	if facts.Direction != "" || req.SourceSet != nil || req.TextQuery != nil || facts.TextQuery != nil {
		return req, fmt.Errorf("specialized direction, source-set and text semantics require their governed executor")
	}
	// Family classification constrains canonical rows; it cannot widen a bound
	// source family or turn generic records into specialized analytical facts.
	for _, c := range semanticOperationCandidates(req) {
		if c.FamilyID == family {
			t, _ := queryTemplateByName(queryTemplateNameByOperationID(c.OperationID))
			if len(t.RecordTypes) == 1 && t.RecordTypes[0] != "all" {
				if bound.RecordType != "" && bound.RecordType != t.RecordTypes[0] {
					return req, fmt.Errorf("family conflicts with bound record type")
				}
				bound.RecordType = t.RecordTypes[0]
				break
			}
		}
	}
	plan := map[string]any{"contract_version": "forensics.query-plan/v1", "intent": queryTemplateOperationID("canonical_records")}
	filters := []map[string]any{}
	if len(p.Filters) > 2 {
		return req, fmt.Errorf("too many canonical scalar filters")
	}
	for _, f := range p.Filters {
		if (f.Field != "source_file" && f.Field != "record_type") || f.Op != "eq" || !containsString(semanticLiteralValues(req), f.Value) {
			return req, fmt.Errorf("filter must reference an exact supplied literal on an allowed field")
		}
		if f.Field == "source_file" {
			if bound.SourceFile != "" && bound.SourceFile != f.Value {
				return req, fmt.Errorf("source filter conflict")
			}
			bound.SourceFile = f.Value
		} else {
			if bound.RecordType != "" && bound.RecordType != f.Value {
				return req, fmt.Errorf("record type conflict")
			}
			bound.RecordType = f.Value
		}
		filters = append(filters, map[string]any{"field": f.Field, "op": f.Op, "value": f.Value})
	}
	if len(filters) > 0 {
		plan["filters"] = filters
	}
	switch p.Mode {
	case "rows":
		if p.Group != nil || p.Compare != nil {
			return req, fmt.Errorf("rows cannot carry aggregates")
		}
		if len(p.Projection) > 0 {
			plan["projection"] = p.Projection
		}
		bound.Projection = p.Projection
		if p.SortBy != "" {
			plan["sort"] = []map[string]any{{"field": p.SortBy, "direction": p.SortDirection}}
			bound.SortBy = p.SortBy
			bound.SortDirection = p.SortDirection
		} else if p.SortDirection != "" {
			return req, fmt.Errorf("sort direction without field")
		}
		bound.Limit = clampLimit(req.Limit)
		if facts.TopK > 0 {
			bound.Limit = clampLimit(facts.TopK)
		}
	case "group":
		if p.Group == nil || p.Compare != nil || len(p.Projection) > 0 || p.SortBy != "" || p.SortDirection != "" {
			return req, fmt.Errorf("invalid group composition")
		}
		g := *p.Group
		if g.TopK != 0 {
			return req, fmt.Errorf("top-k is server-bound")
		}
		g.TopK = facts.TopK
		if g.Timezone != "" && !containsString(semanticLiteralValues(req), g.Timezone) {
			return req, fmt.Errorf("timezone requires an exact supplied literal")
		}
		bound.Group = &g
		bound.Limit = 0
		bound.Offset = 0
		plan["group"] = &g
		if err := validateCanonicalGroupRequest(bound); err != nil {
			return req, err
		}
	case "compare":
		if p.Compare == nil || p.Group != nil || len(p.Projection) > 0 || p.SortBy != "" || p.SortDirection != "" {
			return req, fmt.Errorf("invalid compare composition")
		}
		for _, operand := range []CanonicalComparisonOperandV1{p.Compare.A, p.Compare.B} {
			if operand.Value != nil && !containsString(semanticLiteralValues(req), *operand.Value) {
				return req, fmt.Errorf("comparison value is not supplied")
			}
			for _, v := range []string{operand.From, operand.To} {
				if v != "" && !containsString(semanticLiteralValues(req), v) {
					return req, fmt.Errorf("comparison date is not supplied")
				}
			}
		}
		bound.Compare = p.Compare
		bound.Limit = 0
		bound.Offset = 0
		plan["compare"] = p.Compare
		if err := validateCanonicalCompareRequest(bound); err != nil {
			return req, err
		}
	default:
		return req, fmt.Errorf("unknown dynamic mode")
	}
	if err := validateTypedQueryPlanV1(queryTemplateOperationID("canonical_records"), "canonical_records", plan); err != nil {
		return req, err
	}
	if bound.SemanticPlannerAudit != nil {
		bound.SemanticPlannerAudit.DynamicPlan = plan
	}
	return bound, nil
}

func applySemanticSourceNativePlan(req hybridQueryRequest, family string, p semanticDynamicProposal, catalog []FieldDescriptorV1) (hybridQueryRequest, error) {
	if !semanticDynamicFamilyAllowed(req, family) || p.Mode != "source_native" || p.SourceNative == nil {
		return req, fmt.Errorf("source-native dynamic algebra unavailable in family/scope")
	}
	if len(p.Projection) > 0 || len(p.Filters) > 0 || p.Group != nil || p.Compare != nil || p.SortBy != "" || p.SortDirection != "" {
		return req, fmt.Errorf("source-native algebra cannot carry legacy dynamic controls")
	}
	if err := validateSourceNativePlan(p.SourceNative, catalog); err != nil {
		return req, err
	}
	literals := semanticSourceNativeLiteralValues(req)
	for _, filter := range p.SourceNative.Filters {
		for _, value := range append([]string{filter.Value}, filter.Values...) {
			if value != "" && !containsString(literals, value) {
				return req, fmt.Errorf("source-native filter value was not supplied by the analyst")
			}
		}
	}
	for _, having := range p.SourceNative.Having {
		if !containsString(literals, having.Value) {
			return req, fmt.Errorf("source-native HAVING threshold was not supplied by the analyst")
		}
	}
	bound := req
	bound.Template = "canonical_records"
	bound.SourceNative = p.SourceNative
	bound.Group, bound.Compare, bound.Projection = nil, nil, nil
	bound.Limit, bound.Offset, bound.SortBy, bound.SortDirection = 0, 0, "", ""
	if bound.SemanticPlannerAudit != nil {
		bound.SemanticPlannerAudit.DynamicPlan = p.SourceNative
	}
	return bound, nil
}

func queryTemplateOperationID(name string) string {
	t, _ := queryTemplateByName(name)
	return t.OperationID
}

func semanticDynamicSchema() map[string]any {
	str := func(v ...string) map[string]any { return map[string]any{"type": "string", "enum": v} }
	obj := func(props map[string]any) map[string]any {
		keys := []string{}
		for k := range props {
			keys = append(keys, k)
		}
		return map[string]any{"type": "object", "additionalProperties": false, "properties": props, "required": capabilityUniqueSortedStrings(keys)}
	}
	nullable := func(s map[string]any) map[string]any {
		return map[string]any{"anyOf": []any{s, map[string]any{"type": "null"}}}
	}
	text := map[string]any{"type": "string", "maxLength": 512}
	operand := obj(map[string]any{"value": nullable(text), "from": text, "to": text})
	fields := []string{}
	for f := range canonicalProjectionTypes {
		fields = append(fields, f)
	}
	fields = capabilityUniqueSortedStrings(fields)
	return obj(map[string]any{
		"mode": str("rows", "group", "compare"), "projection": map[string]any{"type": "array", "items": str(fields...), "maxItems": 7},
		"filters": map[string]any{"type": "array", "maxItems": 2, "items": obj(map[string]any{"field": str("source_file", "record_type"), "op": str("eq"), "value": text})},
		"sort_by": str(append([]string{""}, fields...)...), "sort_direction": str("", "asc", "desc"),
		"group":   nullable(obj(map[string]any{"field": str("source_file", "record_type", "timestamp"), "measure": str("count"), "bucket": str("", "hour", "day", "week", "month"), "timezone": text, "top_k": map[string]any{"type": "integer", "const": 0}})),
		"compare": nullable(obj(map[string]any{"field": str("source_file", "record_type", "timestamp"), "measure": str("count"), "a": operand, "b": operand})),
	})
}

func semanticSourceNativeLiteralValues(req hybridQueryRequest) []string {
	values := semanticLiteralValues(req)
	values = append(values, regexp.MustCompile(`\b-?[0-9]+(?:\.[0-9]+)?\b`).FindAllString(req.Query, -1)...)
	return capabilityUniqueSortedStrings(values)
}

func semanticSourceNativeSchema(fields []FieldDescriptorV1) map[string]any {
	str := func(v ...string) map[string]any { return map[string]any{"type": "string", "enum": v} }
	obj := func(props map[string]any) map[string]any {
		keys := make([]string, 0, len(props))
		for key := range props {
			keys = append(keys, key)
		}
		return map[string]any{"type": "object", "additionalProperties": false, "properties": props, "required": capabilityUniqueSortedStrings(keys)}
	}
	nullable := func(value map[string]any) map[string]any {
		return map[string]any{"anyOf": []any{value, map[string]any{"type": "null"}}}
	}
	ids, timeIDs := []string{}, []string{}
	for _, field := range fields {
		ids = append(ids, field.FieldID)
		if field.EffectiveType == fieldTypeTimestamp || field.EffectiveType == fieldTypeDate {
			timeIDs = append(timeIDs, field.FieldID)
		}
	}
	measureIDs := []string{"m1", "m2", "m3", "m4"}
	text := map[string]any{"type": "string", "maxLength": 512}
	fieldID := str(ids...)
	return obj(map[string]any{
		"contract_version": map[string]any{"type": "string", "const": sourceNativePlanContractV1},
		"project":          map[string]any{"type": "array", "items": fieldID, "maxItems": sourceNativeProjectLimit},
		"filters": map[string]any{"type": "array", "maxItems": sourceNativeFilterLimit, "items": obj(map[string]any{
			"field_id": fieldID, "op": str("EQ", "NEQ", "IN", "CONTAINS", "GT", "GTE", "LT", "LTE", "BETWEEN", "IS_NULL", "IS_NOT_NULL"), "value": text,
			"values": map[string]any{"type": "array", "items": text, "maxItems": 20},
		})},
		"group_fields": map[string]any{"type": "array", "items": fieldID, "maxItems": 2},
		"measures": map[string]any{"type": "array", "maxItems": sourceNativeMeasureLimit, "items": obj(map[string]any{
			"measure_id": str(measureIDs...), "op": str("COUNT", "SUM", "AVG", "MIN", "MAX"), "field_id": str(append([]string{""}, ids...)...),
		})},
		"time_bucket": nullable(obj(map[string]any{"field_id": str(timeIDs...), "bucket": str("hour", "day", "week", "month"), "timezone": text})),
		"having":      map[string]any{"type": "array", "maxItems": sourceNativeMeasureLimit, "items": obj(map[string]any{"measure_id": str(measureIDs...), "op": str("EQ", "NEQ", "GT", "GTE", "LT", "LTE"), "value": text})},
		"sort":        map[string]any{"type": "array", "maxItems": 2, "items": obj(map[string]any{"target": str(append(ids, measureIDs...)...), "direction": str("ASC", "DESC")})},
		"limit":       map[string]any{"type": "integer", "minimum": 0, "maximum": maxHybridLimit},
	})
}

func semanticDynamicSchemaWithFields(fields []FieldDescriptorV1) map[string]any {
	schema := semanticDynamicSchema()
	properties := schema["properties"].(map[string]any)
	properties["mode"] = map[string]any{"type": "string", "enum": []string{"rows", "group", "compare", "source_native"}}
	properties["source_native"] = map[string]any{"anyOf": []any{semanticSourceNativeSchema(fields), map[string]any{"type": "null"}}}
	required := append([]string{}, schema["required"].([]string)...)
	required = append(required, "source_native")
	schema["required"] = capabilityUniqueSortedStrings(required)
	return schema
}

func resolveSemanticDynamicPlan(ctx context.Context, cfg config, req hybridQueryRequest, family string) (hybridQueryRequest, string) {
	if !semanticDynamicFamilyAllowed(req, family) {
		return req, "UNSUPPORTED_REQUEST_CLASS"
	}
	catalog, retrieved := []FieldDescriptorV1{}, []FieldDescriptorV1{}
	fieldScores := []SourceNativeFieldScoreV1{}
	if cfg.QueryDB != nil {
		rows, err := sourceNativeCatalogSampleRows(ctx, cfg.QueryDB, req)
		if err != nil {
			return req, "field_catalog_unavailable"
		}
		catalog = buildSourceNativeFieldCatalog(req, rows)
		retrieved, fieldScores = retrieveSourceNativeFields(req.Query, catalog)
		if req.SemanticPlannerAudit != nil {
			req.SemanticPlannerAudit.FieldCatalogCount = len(catalog)
			req.SemanticPlannerAudit.RetrievedFields = retrieved
			req.SemanticPlannerAudit.FieldRetrievalScores = fieldScores
		}
	}
	contextJSON, _ := json.Marshal(map[string]any{"question": req.Query, "family": family, "literal_values": semanticSourceNativeLiteralValues(req), "bound_from": req.DateFrom, "bound_to": req.DateTo, "issued_fields": retrieved})
	schema := semanticDynamicSchema()
	system := "Propose only the bounded canonical source-row algebra in the schema. Count means source rows, never distinct entities. Use rows for filter/project/sort/top-k, group for count/group/time-bucket/top-k, compare for two source-row counts. Unused arrays empty, objects null, strings empty. Values/timezones/comparison bounds must be exact supplied literal_values. Never infer dates or identities. Server supplies scope, time range and top-k; group top_k must be 0. No SQL, free steps or source payload fields."
	if len(retrieved) > 0 {
		schema = semanticDynamicSchemaWithFields(retrieved)
		system = "Build one bounded source-native typed plan using only issued field_id values, issued operators, m1..m4 aggregate IDs, and exact analyst literals. Prefer source_native when the requested project/group/aggregate is not a registered operation. SUM/AVG require fields whose issued allowed_aggregates include them. Unknown and incompatible values never become zero. Use at most two group fields and deterministic sort. No SQL, table names, JSON paths, scope identifiers, or invented literals. All unused legacy controls must be empty/null."
	}
	payload, _ := json.Marshal(map[string]any{"model": req.SynthesisModel, "temperature": 0, "max_tokens": 512, "response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "bounded_canonical_algebra", "strict": true, "schema": schema}}, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": string(contextJSON)}}})
	call, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cfg.LocalAIURL, "/")+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return req, "request_build_failed"
	}
	call.Header.Set("Content-Type", "application/json")
	if cfg.LocalAIAPIKey != "" {
		call.Header.Set("Authorization", "Bearer "+cfg.LocalAIAPIKey)
	}
	response, err := httpclient.NewWithTimeout(semanticPlannerRequestTimeout(cfg)).Do(call)
	if err != nil {
		return req, "timeout_or_unavailable"
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return req, "runtime_rejected"
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return req, "malformed_completion"
	}
	var e struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Finish string `json:"finish_reason"`
		} `json:"choices"`
	}
	if json.Unmarshal(raw, &e) != nil || len(e.Choices) != 1 || e.Choices[0].Finish != "stop" {
		return req, "malformed_completion"
	}
	var p semanticDynamicProposal
	d := json.NewDecoder(strings.NewReader(e.Choices[0].Message.Content))
	d.DisallowUnknownFields()
	if d.Decode(&p) != nil || d.Decode(&struct{}{}) != io.EOF {
		return req, "malformed_dynamic_proposal"
	}
	if p.Mode == "source_native" {
		resolved, err := applySemanticSourceNativePlan(req, family, p, retrieved)
		if err != nil {
			return req, "dynamic_validation_rejected:" + err.Error()
		}
		return resolved, "semantic_dynamic_plan"
	}
	resolved, err := applySemanticDynamicPlan(req, family, p)
	if err != nil {
		return req, "dynamic_validation_rejected:" + err.Error()
	}
	return resolved, "semantic_dynamic_plan"
}
