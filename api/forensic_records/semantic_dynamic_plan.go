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
	"time"
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
	kept := make([]SourceNativeFilterV1, 0, len(p.SourceNative.Filters))
	invented := []string{}
	for _, filter := range p.SourceNative.Filters {
		supplied := true
		for _, value := range append([]string{filter.Value}, filter.Values...) {
			if value == "" || containsString(literals, value) {
				continue
			}
			// A DATE BOUND is the one filter value that is legitimately not a
			// substring of the question. CDR-14 asks "how many call records are
			// from August 2026?" and the generator produced exactly the right
			// plan -- `cdr.event_time BETWEEN 2026-08-01T00:00:00Z ..
			// 2026-08-31T23:59:59Z` -- which string matching refused, because
			// the analyst typed a month NAME. Verified 2026-09-24 from the
			// cached completion.
			//
			// Comparing the parsed INSTANT against the range the deterministic
			// extractor already derived is also robust to a detail that broke
			// enumeration: the generator renders the same month as an inclusive
			// `2026-08-31T23:59:59Z` on one run and an exclusive
			// `2026-09-01T00:00:00Z` on another. Both are inside the analyst's
			// month; neither is a string the analyst typed.
			if temporalFilterWithinRequestedRange(req, filter.FieldID, value, catalog) {
				continue
			}
			supplied = false
			break
		}
		if supplied {
			kept = append(kept, filter)
			continue
		}
		if !semanticDropInventedFiltersEnabled() {
			return req, fmt.Errorf("source-native filter value was not supplied by the analyst")
		}
		invented = append(invented, filter.FieldID)
	}
	if len(invented) > 0 {
		pruned, err := planWithoutInventedFilters(req, p.SourceNative, kept, catalog)
		if err != nil {
			return req, err
		}
		p.SourceNative = pruned
	}
	for _, having := range p.SourceNative.Having {
		if !containsString(literals, having.Value) {
			return req, fmt.Errorf("source-native HAVING threshold was not supplied by the analyst")
		}
	}
	bound := req
	bound.Template = "canonical_records"
	bound.SourceNative = p.SourceNative
	if len(invented) > 0 && bound.SemanticPlannerAudit != nil {
		bound.SemanticPlannerAudit.DroppedInventedFilters = invented
	}
	// The catalogue must travel with the plan. The plan references field IDs and
	// the executor resolves them through this catalogue; without it a perfectly
	// valid plan reached execution and came back as "the requested analysis
	// needs a valid required parameter". The deterministic dynamic branch always
	// set it; this path did not.
	bound.SourceNativeCatalog = catalog
	// Scope the plan to the family it was generated for. Without this the typed
	// algebra runs across EVERY record type in the collection -- measured
	// 2026-09-22, "which cell site handled the most calls" grouped
	// cdr.cell_site_id over 12,912 rows of CDR, IPDR, ANPR, subscriber, tower
	// and transaction data instead of the 8,642 CDR rows it asked about. The
	// legacy dynamic binder has always done this; the source-native one never
	// did, so every plan this path produced was silently unscoped.
	//
	// An explicitly requested record type always wins: the analyst's scope is
	// not something a generated plan may widen or narrow.
	if bound.RecordType == "" {
		if layer, _ := defaultSemanticLayer(); layer != nil {
			if entity, ok := layer.EntityByFamily(family); ok {
				bound.RecordType = entity.RecordType
			}
		}
	}
	bound.Group, bound.Compare, bound.Projection = nil, nil, nil
	// The plan carries its own bounds, so the legacy request-level sort/offset
	// are cleared — but the execution contract requires Parameters.Limit >= 1
	// (query_intelligence_contracts.go: "execution parameters exceed the
	// governed row budget"). Zeroing it made every IR-produced plan fail the
	// budget check after passing every other guard.
	bound.Offset, bound.SortBy, bound.SortDirection = 0, "", ""
	if bound.Limit < 1 {
		bound.Limit = defaultHybridLimit
	}
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
	str := func(v ...string) map[string]any { return map[string]any{"type": "string", "enum": grammarEnum(v)} }
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

// sourceNativeIdentifierToken matches an analyst-supplied identifier: a token
// carrying at least one digit, allowing the separators real identifiers use.
// The previous rule saw only BARE NUMBERS, so "plate LHR-2026" yielded
// `-2026` -- reading the hyphen as a minus sign. A correct plan filtering
// `anpr.plate_number = "LHR-2026"` was therefore rejected as "not supplied by
// the analyst" on EVERY plate question, at any model quality. Measured 2026-09-22.
var sourceNativeIdentifierToken = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9_./:+-]*[A-Za-z0-9]|[0-9]+`)

// semanticSourceNativeLiteralValues is the CONSTRAINT_APPLIED allowlist: a filter
// may only carry a value the analyst actually supplied. It has to see every shape
// of literal a question can carry, or it silently turns correct plans into refusals.
func semanticSourceNativeLiteralValues(req hybridQueryRequest) []string {
	values := semanticLiteralValues(req)
	separators := func(r rune) bool {
		return r == '-' || r == '_' || r == '/' || r == ':' || r == '.'
	}
	for _, token := range sourceNativeIdentifierToken.FindAllString(req.Query, -1) {
		if len(token) > 128 {
			continue
		}
		// A digit is not what makes a token an identifier -- a SEPARATOR is.
		// The digit gate was the third false-rejection this allowlist has
		// produced, after the `-2026` minus-sign read and the invisible curated
		// enum literals. SUB-03 asks "who is the registered subscriber for
		// PK-SUB-SYN-ALPHA?", the generator produced the single exactly-correct
		// filter `subscriber.subscriber_id EQ "PK-SUB-SYN-ALPHA"`, and this
		// loop skipped the token because it holds no digit -- so a perfect plan
		// was refused as "not supplied by the analyst". Verified 2026-09-24
		// from the cached completion, not inferred.
		//
		// An ordinary word still cannot enter: it has no separator, and a
		// separator-bearing token must have at least two non-empty segments.
		if !strings.ContainsAny(token, "0123456789") && !structuredIdentifierToken(token) {
			continue
		}
		values = append(values, token)
		// A hyphenated identifier also supplies its parts, which is how
		// "records from 2026-08" supplies the year on its own.
		for _, part := range strings.FieldsFunc(token, separators) {
			if part != token && part != "" && strings.ContainsAny(part, "0123456789") {
				values = append(values, part)
			}
		}
	}
	// Values the CURATED LAYER declares are supplied by the analyst too: "active"
	// is not an identifier pattern, so it was invisible here and SUB-02's correct
	// `subscriber.status = ACTIVE` filter was rejected. The layer has carried these
	// enumerations since WI-4 and nothing ever read them.
	values = append(values, semanticLayerDeclaredLiterals(req.Query)...)
	return capabilityUniqueSortedStrings(values)
}

// semanticSourceNativeSchema issues the plan schema with no narrowing.
func semanticSourceNativeSchema(fields []FieldDescriptorV1) map[string]any {
	return semanticSourceNativeSchemaForGoal(fields, "")
}

// semanticSourceNativeSchemaForGoal issues the plan schema NARROWED by what the
// question deterministically asks for. S4: cut what the generator is offered so
// the correct plan is the only expressible one.
func semanticSourceNativeSchemaForGoal(fields []FieldDescriptorV1, goal string) map[string]any {
	str := func(v ...string) map[string]any { return map[string]any{"type": "string", "enum": grammarEnum(v)} }
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
	// An empty enum is not a narrow choice, it is an IMPOSSIBLE one: the GBNF
	// converter turns `enum: []` into an alternation with no alternatives, and
	// llama.cpp answers the whole request with
	// "Failed to initialize samplers: failed to parse grammar" -- HTTP 500,
	// before any inference. Retrieval returns the top-K fields for the
	// question, so a question that mentions no time ("total bytes
	// transferred", "records per source file") legitimately keeps no TIMESTAMP
	// field and empties timeIDs. Reproduced controlled on 2026-09-22: the same
	// schema is 200 with the enum populated and 500 with it emptied.
	// So when nothing can be bucketed, time_bucket is null-only.
	timeBucket := map[string]any{"type": "null"}
	if len(timeIDs) > 0 {
		timeBucket = nullable(obj(map[string]any{
			"field_id": str(timeIDs...), "bucket": str("hour", "day", "week", "month"), "timezone": text}))
	}
	return obj(map[string]any{
		"contract_version": map[string]any{"type": "string", "const": sourceNativePlanContractV1},
		"project":          map[string]any{"type": "array", "items": fieldID, "maxItems": sourceNativeSchemaProjectLimit},
		"filters":          map[string]any{"type": "array", "maxItems": sourceNativeSchemaFilterLimit, "items": filterItemSchema(obj, str, text, fieldID)},
		"group_fields":     map[string]any{"type": "array", "items": fieldID, "maxItems": 2},
		"measures":         map[string]any{"type": "array", "maxItems": sourceNativeMeasureLimit, "items": measureItemSchema(obj, str, measureIDs, ids, distinctOnlyFieldIDs(fields, goal))},
		"time_bucket":      timeBucket,
		"having":           map[string]any{"type": "array", "maxItems": sourceNativeSchemaHavingLimit, "items": obj(map[string]any{"measure_id": str(measureIDs...), "op": str("EQ", "NEQ", "GT", "GTE", "LT", "LTE"), "value": text})},
		"sort":             map[string]any{"type": "array", "maxItems": 2, "items": obj(map[string]any{"target": str(append(ids, measureIDs...)...), "direction": str("ASC", "DESC")})},
		"limit":            map[string]any{"type": "integer", "minimum": 0, "maximum": maxHybridLimit},
	})
}

func semanticDynamicSchemaWithFields(fields []FieldDescriptorV1) map[string]any {
	schema := semanticDynamicSchema()
	properties := schema["properties"].(map[string]any)
	properties["mode"] = map[string]any{"type": "string", "enum": grammarEnum([]string{"rows", "group", "compare", "source_native"})}
	properties["source_native"] = map[string]any{"anyOf": []any{semanticSourceNativeSchema(fields), map[string]any{"type": "null"}}}
	required := append([]string{}, schema["required"].([]string)...)
	required = append(required, "source_native")
	schema["required"] = capabilityUniqueSortedStrings(required)
	return schema
}

// sourceNativeBriefDescription keeps the first sentence of a curated
// description. The full YAML text is written for a human reading a diff; the
// generator only needs enough to tell two fields apart, and on a ~4 tok/s CPU
// every prompt token is latency the request pays twice -- prefill, then a
// slower decode.
//
// Measured 2026-09-22: once retrieval correctly issued all 13 curated CDR
// fields, their full descriptions pushed "How many calls did <number> make?"
// past the 180s MaximumModelTimeout policy ceiling and a valid plan was thrown
// away as a timeout. Trimming the prose keeps every FIELD available, which is
// what the accuracy depends on.
func sourceNativeBriefDescription(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if cut := strings.Index(text, ". "); cut > 0 && cut < 180 {
		return text[:cut+1]
	}
	if len(text) > 180 {
		if space := strings.LastIndex(text[:180], " "); space > 0 {
			return text[:space]
		}
		return text[:180]
	}
	return text
}

func resolveSemanticDynamicPlan(ctx context.Context, cfg config, req hybridQueryRequest, family string) (hybridQueryRequest, string) {
	if !semanticDynamicFamilyAllowed(req, family) {
		return req, "UNSUPPORTED_REQUEST_CLASS"
	}
	// Generation is the dominant cost of a structured answer and it was
	// recorded NOWHERE: `llm_latency_ms` is assigned only inside bounded
	// synthesis, so CDR-02 reported total=119,835 ms · db=988 ms · llm=0 --
	// 119 seconds attributed to nothing, on a question whose SQL took under a
	// second. Timing it here rather than in a caller covers all three entry
	// points (fallback, shadow probe, and the open-ended planner).
	//
	// ACCUMULATE: with shadow AND fallback on, one question generates twice,
	// and the audit must show the total actually spent, not the last leg.
	if req.SemanticPlannerAudit != nil {
		generationStart := time.Now()
		defer func() {
			req.SemanticPlannerAudit.IRGenerationMS += time.Since(generationStart).Milliseconds()
		}()
	}
	catalog, retrieved := []FieldDescriptorV1{}, []FieldDescriptorV1{}
	fieldScores := []SourceNativeFieldScoreV1{}
	if cfg.QueryDB != nil {
		// Scope the catalogue sample to the family that was already resolved.
		// Without this the sample spans every record type in the collection —
		// 82 distinct columns in the demo case against 8 for IPDR — and BM25's
		// top-K fills with uncurated hashes from unrelated families. Measured
		// 2026-09-22: an IPDR breakdown was offered 10 hashed fields and only 2
		// curated ones, and "How many IPDR sessions are there?" was never
		// offered `ipdr.protocol` or `ipdr.bytes` at all, so no model of any
		// quality could have answered it. CDR hid this because its curated
		// synonyms win the ranking on their own.
		//
		// Only the CATALOGUE sample is narrowed; req is untouched, so execution
		// scope and field IDs are exactly what they were.
		sampleReq := req
		if sampleReq.RecordType == "" {
			if layer, _ := defaultSemanticLayer(); layer != nil {
				if entity, ok := layer.EntityByFamily(family); ok {
					sampleReq.RecordType = entity.RecordType
				}
			}
		}
		rows, err := sourceNativeCatalogSampleRows(ctx, cfg.QueryDB, sampleReq)
		if err != nil {
			return req, "field_catalog_unavailable"
		}
		catalog = buildSourceNativeFieldCatalog(req, rows)
		// Hand the generator the CURATED catalogue. WI-0 §2.3 predicted that
		// selecting `fld_89f1331b28bdd124373ca5d8` from an enum of hashes is a
		// materially harder task than selecting `cdr.msisdn`, and it was: with
		// hashed IDs the generator filtered "how many calls did <number> make"
		// on the wrong field and answered "no CDR records" — a clarification
		// turned into a confident wrong answer. Readable IDs carry descriptions
		// and synonyms with them.
		catalog, _ = semanticLayerCatalogForRequest(req, req.Query, catalog)
		retrieved, fieldScores = retrieveSourceNativeFields(req.Query, catalog)
		if req.SemanticPlannerAudit != nil {
			req.SemanticPlannerAudit.FieldCatalogCount = len(catalog)
			req.SemanticPlannerAudit.RetrievedFields = retrieved
			req.SemanticPlannerAudit.FieldRetrievalScores = fieldScores
		}
	}
	// Curated fields carry display names, descriptions and synonyms. Handing the
	// model the bare field_id list is what WI-0 measured as materially harder:
	// nothing tells it that `call_org_num` means "originating number".
	issued := make([]map[string]any, 0, len(retrieved))
	for _, field := range retrieved {
		entry := map[string]any{"field_id": field.FieldID, "type": field.EffectiveType,
			"groupable": field.Groupable, "aggregates": field.AllowedAggregates,
			"filters": field.AllowedFilters}
		if field.DisplayName != "" {
			entry["name"] = field.DisplayName
		}
		if field.Description != "" {
			entry["description"] = sourceNativeBriefDescription(field.Description)
		}
		if len(field.Synonyms) > 0 {
			entry["also_called"] = field.Synonyms
		}
		issued = append(issued, entry)
	}
	contextJSON, _ := json.Marshal(map[string]any{"question": req.Query, "family": family, "literal_values": semanticSourceNativeLiteralValues(req), "bound_from": req.DateFrom, "bound_to": req.DateTo, "issued_fields": issued})
	schema := semanticDynamicSchema()
	system := "Propose only the bounded canonical source-row algebra in the schema. Count means source rows, never distinct entities. Use rows for filter/project/sort/top-k, group for count/group/time-bucket/top-k, compare for two source-row counts. Unused arrays empty, objects null, strings empty. Values/timezones/comparison bounds must be exact supplied literal_values. Never infer dates or identities. Server supplies scope, time range and top-k; group top_k must be 0. No SQL, free steps or source payload fields."
	cleanPlanSchema := false
	if len(retrieved) > 0 {
		// The CLEAN IR schema, not the legacy wrapper. The wrapper additionally
		// makes the model fill mode/projection/sort_by/group/compare, which is
		// accumulated surface rather than the plan being asked for: it lengthens
		// generation on a ~4 tok/s CPU past the planner timeout and gives the
		// model four more ways to be wrong. WI-0's 74.3% was measured on the
		// clean schema.
		// S4 NARROWING. The frame's goal is deterministic and was measured across
		// the full 62 BEFORE being used here: `distinct` fires on exactly two
		// questions, CDR-08 and ANPR-05, with zero false positives on the other
		// sixty. It is also a signal the system already treats as authoritative --
		// S9 refuses a plain COUNT whenever the goal is distinct, which is why
		// both questions clarify today. Narrowing makes the schema AGREE WITH THE
		// VERIFIER rather than spending a 60-150 s generation on a plan S9 is
		// going to reject.
		schema = semanticSourceNativeSchemaForGoal(retrieved, semanticRequestFrameGoal(req))
		cleanPlanSchema = true
		// TWO SENTENCES ADDED 2026-09-24, each from a CAPTURED completion, not
		// from a guess about what the model needs told.
		//
		// COUNT_DISTINCT: every other aggregate had an instruction here and
		// this one had none, so the model answered "how many unique phone
		// numbers appear as callers" with a plain COUNT of rows -- 8,642 where
		// the truth is 10. S9's distinct obligation caught it, correctly. The
		// question was framed as a model-tier limit; the model had simply never
		// been told the aggregate existed.
		//
		// INVENTED FILTERS: "never invent one" governed only the case where the
		// question names NO literal. CDR-11 names one and the model appended
		// `cdr.direction = "outbound"` beside it; TWR-02 appended
		// `site_location CONTAINS "where"` -- the interrogative itself. The
		// rule now binds every filter, not just the empty case. Dropping such
		// filters after the fact was measured and REJECTED: it created two
		// confident-wrong answers, because an invented filter marks a plan the
		// model did not understand rather than a blemish on a sound one.
		system = irCleanPlanSystemPrompt()
	}
	payload, _ := json.Marshal(map[string]any{"model": req.SynthesisModel, "temperature": 0, "max_tokens": 512, "response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "bounded_canonical_algebra", "strict": true, "schema": schema}}, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": string(contextJSON)}}})
	// The completion is requested at temperature 0, so it is a deterministic
	// function of this payload: a byte-identical payload yields byte-identical
	// output. Memoizing on the payload hash therefore returns exactly what the
	// model would have returned, and the cached bytes re-enter the SAME decode,
	// bind and re-verification path below. The cache removes an HTTP call,
	// never a check. See semantic_plan_cache.go.
	planCacheKey := semanticPlanCacheKey(req, payload)
	raw, planCacheHit := cachedSemanticPlanCompletion(planCacheKey)
	if req.SemanticPlannerAudit != nil {
		req.SemanticPlannerAudit.IRPlanCacheHit = planCacheHit
	}
	if !planCacheHit {
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
			// Record WHY. A bare "runtime_rejected" cannot tell a grammar the
			// backend refused to compile from a slot that was still busy, and those
			// have opposite fixes. LocalAI's access log only prints the status.
			body, _ := io.ReadAll(io.LimitReader(response.Body, 400))
			detail := strings.Join(strings.Fields(string(body)), " ")
			return req, fmt.Sprintf("runtime_rejected:%d:%s", response.StatusCode, detail)
		}
		raw, err = io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
		if err != nil || len(raw) > 1<<20 {
			return req, "malformed_completion"
		}
	}
	var e struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Finish string `json:"finish_reason"`
		} `json:"choices"`
	}
	if json.Unmarshal(raw, &e) != nil || len(e.Choices) != 1 {
		return req, "malformed_completion"
	}
	if finish := e.Choices[0].Finish; finish != "stop" {
		// LocalAI reports "length" when generation hit the max_tokens ceiling
		// (core/http/endpoints/openai/chat.go). Folding that into
		// malformed_completion blames the model for a budget WE set, and hides
		// the one failure here that has a one-line fix. The plan is still
		// discarded either way; only the recorded reason differs.
		if finish == "length" {
			// Record how much came back. A plan that truncates at roughly the
			// size of a valid one means the budget is simply too small; one that
			// truncates far past it means the model is padding the grammar's
			// arrays (project maxItems 12, filters 8) and the fix is the schema,
			// not the budget. Without this the two are indistinguishable.
			return req, fmt.Sprintf("truncated_token_budget:%dB", len(e.Choices[0].Message.Content))
		}
		return req, "malformed_completion:finish_" + finish
	}
	// Memoize only a completion that decoded and finished cleanly. Caching a
	// transient malformation or a truncated generation would make a momentary
	// model hiccup permanent for that question, and the failures above are
	// exactly the ones worth retrying. Everything past this point is
	// deterministic given the same bytes and the same request scope, which the
	// key already covers -- so a plan the validator later REJECTS is still
	// worth memoizing: re-generating to earn the identical rejection costs the
	// analyst two minutes for nothing.
	if !planCacheHit {
		storeSemanticPlanCompletion(planCacheKey, raw)
	}
	var p semanticDynamicProposal
	if cleanPlanSchema {
		// The clean schema returns the plan itself, so it is parsed directly and
		// wrapped into the proposal the validator already speaks.
		var plan SourceNativePlanV1
		decoder := json.NewDecoder(strings.NewReader(e.Choices[0].Message.Content))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&plan) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			return req, "malformed_dynamic_proposal"
		}
		p = semanticDynamicProposal{Mode: "source_native", SourceNative: &plan}
	} else {
		d := json.NewDecoder(strings.NewReader(e.Choices[0].Message.Content))
		d.DisallowUnknownFields()
		if d.Decode(&p) != nil || d.Decode(&struct{}{}) != io.EOF {
			return req, "malformed_dynamic_proposal"
		}
	}
	if p.Mode == "source_native" {
		// Keep the REJECTED plan. Discarding it and returning only the error
		// string is what made this class un-diagnosable: "filter value was not
		// supplied by the analyst" cannot tell an invented value from a real
		// one our extractor could not see, and those have opposite fixes. The
		// plan is evidence on the audit, never an answer.
		if req.SemanticPlannerAudit != nil {
			req.SemanticPlannerAudit.DynamicPlan = p.SourceNative
		}
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

// measureItemSchema issues a measure as TWO VARIANTS so that an aggregate with
// nothing to aggregate cannot be emitted at all.
//
// The flat object listed "" among `field_id`'s enum members, because a plain
// COUNT legitimately omits a field. That also made `COUNT_DISTINCT` with an
// empty field_id a grammatically valid sentence, and the generator emitted
// exactly that for CDR-08 -- "how many unique phone numbers appear as callers"
// -- which the strict parser then refused at decode with "COUNT_DISTINCT
// requires the field whose values are counted". Read from the cached
// completion 2026-09-24.
//
// That inverted the keystone of this architecture. The enum exists so an
// invalid plan is IMPOSSIBLE to produce, not so it is caught afterwards; a
// schema that permits what the validator forbids turns a near-miss into a
// refusal instead of an answer. The two variants restore the property:
//
//	row count        op is COUNT, field_id is ""      -- count rows
//	field aggregate  any allowed op over an issued id -- aggregate a column
//
// EMPTY ENUMS ARE THE KNOWN KILLER on this path: the GBNF converter turns
// `enum: []` into an alternation with no alternatives and llama.cpp answers the
// whole request with "failed to parse grammar" -- HTTP 500 before any
// inference. Retrieval can legitimately issue no fields, so the aggregate
// variant is offered only when there is something to aggregate.
func measureItemSchema(
	obj func(map[string]any) map[string]any,
	str func(...string) map[string]any,
	measureIDs, ids, distinctOnly []string,
) map[string]any {
	// S4: a DISTINCT question may not be answered by counting rows, so the
	// row-count variant is withdrawn entirely and the only measure on offer is
	// COUNT_DISTINCT over a distinct-capable field. S9 refuses everything this
	// removes, so the narrowing makes the schema agree with the verifier rather
	// than introducing a new judgement of its own.
	if len(distinctOnly) > 0 {
		return obj(map[string]any{
			"measure_id": str(measureIDs...),
			"op":         str("COUNT_DISTINCT"),
			"field_id":   str(distinctOnly...),
		})
	}
	rowCount := obj(map[string]any{
		"measure_id": str(measureIDs...),
		"op":         str("COUNT"),
		"field_id":   str(""),
	})
	if len(ids) == 0 {
		return rowCount
	}
	fieldAggregate := obj(map[string]any{
		"measure_id": str(measureIDs...),
		"op":         str("COUNT", "COUNT_DISTINCT", "SUM", "AVG", "MIN", "MAX"),
		"field_id":   str(ids...),
	})
	return map[string]any{"anyOf": []any{rowCount, fieldAggregate}}
}

// grammarEnum renders an enum as []any.
//
// THIS CHANGES NOTHING ON THE SERVING PATH, and the reason is worth recording
// because the investigation that produced it reached a confident wrong
// conclusion first.
//
// LocalAI's converter reads an enum with `schema["enum"].([]any)`
// (pkg/functions/grammars/json_schema.go). Calling that converter directly on
// the in-memory Go map -- which is what a local probe does -- fails the
// assertion when the enum is a []string, skips the enum branch in silence, and
// emits the GENERIC STRING rule for every field_id and op. Dumped that way on
// 2026-09-24 it looks exactly like the keystone silently not holding.
//
// IT WAS HOLDING. The product never converts locally: the schema is marshalled
// into the request body, and the SERVER unmarshals it into `functions.Item`
// before converting. []string and []any marshal to the same JSON, so the
// server has always seen []any and has always produced the alternation:
//
//	root-0-project-item         ::= "\"cdr.msisdn\"" | "\"cdr.event_time\""
//	root-0-filters-item-field-id ::= "\"cdr.msisdn\"" | "\"cdr.event_time\""
//
// So this is kept for one narrow reason only: a future local conversion, or a
// probe like the one above, gets the same answer the server does instead of a
// misleading one. Any test that means to assert the keystone must go through
// the JSON round-trip -- see grammar_keystone_test.go.
func grammarEnum(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

// irCleanPlanSystemPrompt is the instruction set handed to the generator with
// the clean IR schema. Named so it can be asserted on: it is the only place the
// model is told what the issued aggregates MEAN, and a missing instruction here
// is indistinguishable from a model that cannot do the task. CDR-08 was nearly
// escalated to a larger model tier over an aggregate this prompt never
// mentioned.
func irCleanPlanSystemPrompt() string {
	return "Fill one bounded analytical plan using ONLY the issued field_id values, the issued operators, and measure IDs m1..m4. Count rows with {\"measure_id\":\"m1\",\"op\":\"COUNT\",\"field_id\":\"\"} — COUNT is the only aggregate allowed an empty field_id. A total or sum uses SUM, the largest MAX, the smallest MIN; never answer \"how much\" or \"largest\" with COUNT. A date range, time period or span covered needs TWO measures on the time field: MIN as m1 and MAX as m2 - one value cannot express a range. A breakdown or \"by X\" puts X in group_fields. \"Which X has the most\" groups by X, counts rows, sorts m1 DESC, limit 1. Every literal in the question MUST become a filter. If the question names NO literal, filters MUST be [] - never invent one. Leave every array you do not need empty. SUM/AVG require a field whose issued aggregates include them. Never write SQL, name a table, or invent a field or literal."
}

// filterItemSchema issues a filter as TWO VARIANTS so that a null test carrying
// a value cannot be written down.
//
// Identical in kind to the measure split, and found the same way. The filter
// was issued as ONE flat object -- `op` spanning all eleven operators beside a
// free `value` and a `values` array -- so `IS_NULL` with a value was a
// grammatically valid sentence. The generator wrote it for CDR-11
// (`cdr.call_type IS_NULL` carrying a value) and `validateSourceNativePlan`
// refused the plan with "null filters cannot carry values", turning a question
// the system had understood into a clarification.
//
// The keystone is that an invalid plan is IMPOSSIBLE to emit, not caught
// afterwards. A flat object cannot express "value must be empty when op is a
// null test"; two variants can, and GBNF alternation supports it.
//
//	null test   op ∈ {IS_NULL, IS_NOT_NULL}, value "", and NO values key at all
//	valued      every other operator, with value and values as before
//
// The null variant omits `values` rather than bounding it to zero items: an
// array with maxItems 0 compiles to an alternation with no alternatives, which
// is the empty-enum killer wearing a different hat. Omitted from `properties`
// it is also omitted from `required`, the decoder never emits the key, and
// `SourceNativeFilterV1.Values` unmarshals to nil -- which is what the
// validator wants.
//
// Measured before building: across every hand-written gold plan the operators
// used are EQ 9, NEQ 1, BETWEEN 1, and NO gold plan uses a null test. The null
// variant is a capability the curated layer declares and the corpus never
// needs; it stays expressible because withdrawing a declared capability is a
// separate decision.
func filterItemSchema(
	obj func(map[string]any) map[string]any,
	str func(...string) map[string]any,
	text map[string]any,
	fieldID map[string]any,
) map[string]any {
	nullTest := obj(map[string]any{
		"field_id": fieldID,
		"op":       str("IS_NULL", "IS_NOT_NULL"),
		"value":    str(""),
	})
	valued := obj(map[string]any{
		"field_id": fieldID,
		"op":       str("EQ", "NEQ", "IN", "CONTAINS", "GT", "GTE", "LT", "LTE", "BETWEEN"),
		"value":    text,
		"values":   map[string]any{"type": "array", "items": text, "maxItems": 20},
	})
	return map[string]any{"anyOf": []any{nullTest, valued}}
}
