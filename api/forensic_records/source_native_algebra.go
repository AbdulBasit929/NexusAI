package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	fieldDescriptorContractV1  = "forensics.field-descriptor/v1"
	sourceNativePlanContractV1 = "forensics.source-native-plan/v1"
	sourceNativeResultV1       = "forensics.source-native-result/v1"
	sourceNativeFieldTopK      = 12
	sourceNativeInputRowLimit  = 1000
	sourceNativeGroupLimit     = 100
	sourceNativeProjectLimit   = 12
	sourceNativeFilterLimit    = 8
	sourceNativeMeasureLimit   = 4
	sourceNativeComplexityMax  = 16

	// The GRAMMAR the model fills is bounded far more tightly than the API
	// contract above. The GBNF converter emits every property unconditionally,
	// so the model is always handed these arrays to fill and it does: measured
	// 2026-09-22 it emitted 599B, 1696B and 1791B against a ~292B correct plan
	// and never terminated, which surfaced as `truncated_token_budget`. Raising
	// max_tokens would only buy more room to pad. No gold plan in the corpus
	// uses more than 3 projections or 2 filters, so these leave headroom while
	// bounding the worst case. The executor's limits are unchanged: a typed API
	// caller may still send the full contract.
	sourceNativeSchemaProjectLimit = 4
	sourceNativeSchemaFilterLimit  = 3
	sourceNativeSchemaHavingLimit  = 2
)

const (
	fieldTypeString    = "STRING"
	fieldTypeInteger   = "INTEGER"
	fieldTypeDecimal   = "DECIMAL"
	fieldTypeBoolean   = "BOOLEAN"
	fieldTypeTimestamp = "TIMESTAMP"
	fieldTypeDate      = "DATE"
	fieldTypeUnknown   = "UNKNOWN"
)

// FieldDescriptorV1 is derived from rows inside one authenticated evidence
// scope. FieldID is the only field reference accepted from a model or typed
// plan; SourceName is never interpreted as SQL.
type FieldDescriptorV1 struct {
	ContractVersion string `json:"contract_version"`
	FieldID         string `json:"field_id"`
	// DisplayName, Description and Synonyms are empty for an inferred field and
	// populated only from the curated semantic layer. Their absence is what made
	// the catalogue unusable to a model: nothing said that `call_org_num` means
	// "originating number", so a field could only be picked by similarity.
	DisplayName            string   `json:"display_name,omitempty"`
	Description            string   `json:"description,omitempty"`
	Synonyms               []string `json:"synonyms,omitempty"`
	Curated                bool     `json:"curated,omitempty"`
	SourceName             string   `json:"source_name"`
	SourceNames            []string `json:"source_names"`
	NormalizedName         string   `json:"normalized_name"`
	EffectiveType          string   `json:"effective_type"`
	PresenceCount          int      `json:"presence_count"`
	NullCount              int      `json:"null_count"`
	IncompatibleValueCount int      `json:"incompatible_value_count"`
	Sensitivity            string   `json:"sensitivity"`
	RedactionState         string   `json:"redaction_state"`
	AllowedFilters         []string `json:"allowed_filters"`
	Projectable            bool     `json:"projectable"`
	Groupable              bool     `json:"groupable"`
	AllowedAggregates      []string `json:"allowed_aggregates"`
	Sortable               bool     `json:"sortable"`
	FamilyProvenance       []string `json:"family_provenance"`
	SourceProvenance       []string `json:"source_provenance"`
	// DerivedOp/DerivedFields carry a curated metric that is COMPUTED rather
	// than stored. There is no duration column in CDR data, only a start and an
	// end, so "what was the longest call" could not be expressed at all: the
	// layer declares cdr.call_duration_seconds with an allowlisted expression,
	// but the typed plan had no way to name it. Publishing the metric as a
	// catalogue field lets the generator select it from the same enum as any
	// stored column, with the expression resolved server-side at SQL time.
	DerivedOp         string   `json:"derived_op,omitempty"`
	DerivedFields     []string `json:"derived_fields,omitempty"`
	EvidenceID        string   `json:"evidence_id,omitempty"`
	EvidenceVersionID string   `json:"evidence_version_id,omitempty"`
}

type SourceNativeFieldScoreV1 struct {
	FieldID string  `json:"field_id"`
	Rank    int     `json:"rank"`
	Score   float64 `json:"score"`
}

type SourceNativeFilterV1 struct {
	FieldID string   `json:"field_id"`
	Op      string   `json:"op"`
	Value   string   `json:"value"`
	Values  []string `json:"values"`
}

type SourceNativeMeasureV1 struct {
	MeasureID string `json:"measure_id"`
	Op        string `json:"op"`
	FieldID   string `json:"field_id"`
}

type SourceNativeTimeBucketV1 struct {
	FieldID  string `json:"field_id"`
	Bucket   string `json:"bucket"`
	Timezone string `json:"timezone"`
}

type SourceNativeHavingV1 struct {
	MeasureID string `json:"measure_id"`
	Op        string `json:"op"`
	Value     string `json:"value"`
}

type SourceNativeSortV1 struct {
	Target    string `json:"target"`
	Direction string `json:"direction"`
}

type SourceNativePlanV1 struct {
	ContractVersion string                    `json:"contract_version"`
	Project         []string                  `json:"project"`
	Filters         []SourceNativeFilterV1    `json:"filters"`
	GroupFields     []string                  `json:"group_fields"`
	Measures        []SourceNativeMeasureV1   `json:"measures"`
	TimeBucket      *SourceNativeTimeBucketV1 `json:"time_bucket"`
	Having          []SourceNativeHavingV1    `json:"having"`
	Sort            []SourceNativeSortV1      `json:"sort"`
	Limit           int                       `json:"limit"`
}

func (p *SourceNativePlanV1) UnmarshalJSON(raw []byte) error {
	type plain SourceNativePlanV1
	var decoded plain
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if err := d.Decode(&decoded); err != nil {
		return err
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return errors.New("source-native plan contains trailing content")
	}
	*p = SourceNativePlanV1(decoded)
	return validateSourceNativePlanShape(p)
}

var sourceNativeNameParts = regexp.MustCompile(`[^a-z0-9]+`)

func normalizeSourceNativeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = sourceNativeNameParts.ReplaceAllString(value, "_")
	return strings.Trim(value, "_")
}

func sourceNativeSensitive(name string) bool {
	name = normalizeSourceNativeName(name)
	for _, token := range []string{"cnic", "national_id", "password", "passwd", "secret", "token", "credential", "subscriber_name", "customer_name", "full_name"} {
		if name == token || strings.Contains(name, "_"+token) || strings.Contains(name, token+"_") {
			return true
		}
	}
	return false
}

func sourceNativeFieldID(req hybridQueryRequest, normalized string) string {
	scope := strings.Join([]string{fieldDescriptorContractV1, req.TenantID, req.CollectionID, req.EvidenceID, req.EvidenceVersionID, normalized}, "\x00")
	digest := sha256.Sum256([]byte(scope))
	return "fld_" + hex.EncodeToString(digest[:12])
}

type sourceNativeObservedField struct {
	sourceNames map[string]bool
	families    map[string]bool
	sources     map[string]bool
	presence    int
	nulls       int
	types       map[string]int
}

func sourceNativeMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func sourceNativeRowValues(row map[string]any) map[string]struct {
	source string
	value  any
} {
	values := map[string]struct {
		source string
		value  any
	}{}
	add := func(source string, value any) {
		normalized := normalizeSourceNativeName(source)
		if normalized == "" || len(normalized) > 128 {
			return
		}
		if _, exists := values[normalized]; !exists {
			values[normalized] = struct {
				source string
				value  any
			}{source: source, value: value}
		}
	}
	for _, name := range []string{"record_type", "source_file", "timestamp", "row_number"} {
		add(name, row[name])
	}
	rawPayload := sourceNativeMap(row["raw_payload"])
	for _, key := range sortedSourceNativeMapKeys(rawPayload) {
		add(key, rawPayload[key])
	}
	metadata := sourceNativeMap(row["metadata"])
	normalizedFields := sourceNativeMap(metadata["normalized_fields"])
	for _, key := range sortedSourceNativeMapKeys(normalizedFields) {
		add(key, normalizedFields[key])
	}
	return values
}

func sortedSourceNativeMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func classifySourceNativeScalar(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case bool:
		return fieldTypeBoolean
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fieldTypeInteger
	case float32:
		if math.Trunc(float64(typed)) == float64(typed) {
			return fieldTypeInteger
		}
		return fieldTypeDecimal
	case float64:
		if math.Trunc(typed) == typed {
			return fieldTypeInteger
		}
		return fieldTypeDecimal
	case json.Number:
		if _, err := typed.Int64(); err == nil {
			return fieldTypeInteger
		}
		if _, err := typed.Float64(); err == nil {
			return fieldTypeDecimal
		}
		return fieldTypeString
	case time.Time:
		return fieldTypeTimestamp
	case string:
		value := strings.TrimSpace(typed)
		if value == "" {
			return ""
		}
		if _, err := time.Parse(time.RFC3339, value); err == nil {
			return fieldTypeTimestamp
		}
		if _, err := time.Parse("2006-01-02", value); err == nil {
			return fieldTypeDate
		}
		if strings.EqualFold(value, "true") || strings.EqualFold(value, "false") {
			return fieldTypeBoolean
		}
		if regexp.MustCompile(`^-?[0-9]+$`).MatchString(value) {
			return fieldTypeInteger
		}
		if regexp.MustCompile(`^-?(?:[0-9]+\.[0-9]+|[0-9]+(?:e[+-]?[0-9]+))$`).MatchString(strings.ToLower(value)) {
			return fieldTypeDecimal
		}
		return fieldTypeString
	default:
		return fieldTypeUnknown
	}
}

func effectiveSourceNativeType(counts map[string]int) (string, int) {
	if len(counts) == 0 {
		return fieldTypeUnknown, 0
	}
	numeric := counts[fieldTypeInteger] + counts[fieldTypeDecimal]
	bestType, bestCount := "", 0
	for kind, count := range counts {
		if count > bestCount || count == bestCount && kind < bestType {
			bestType, bestCount = kind, count
		}
	}
	if numeric >= bestCount {
		bestType, bestCount = fieldTypeInteger, numeric
		if counts[fieldTypeDecimal] > 0 {
			bestType = fieldTypeDecimal
		}
	}
	total := 0
	for _, count := range counts {
		total += count
	}
	return bestType, total - bestCount
}

func sourceNativeTypePolicy(kind string, incompatible int) (filters, aggregates []string, projectable, groupable, sortable bool) {
	if kind == fieldTypeUnknown {
		return []string{}, []string{}, false, false, false
	}
	projectable = true
	if incompatible > 0 {
		return []string{"IS_NULL", "IS_NOT_NULL"}, []string{}, true, false, false
	}
	groupable, sortable = true, true
	aggregates = []string{"COUNT"}
	switch kind {
	case fieldTypeString:
		filters = []string{"EQ", "NEQ", "IN", "CONTAINS", "IS_NULL", "IS_NOT_NULL"}
	case fieldTypeBoolean:
		filters = []string{"EQ", "NEQ", "IS_NULL", "IS_NOT_NULL"}
	case fieldTypeInteger, fieldTypeDecimal:
		filters = []string{"EQ", "NEQ", "GT", "GTE", "LT", "LTE", "BETWEEN", "IN", "IS_NULL", "IS_NOT_NULL"}
		aggregates = []string{"COUNT", "SUM", "AVG", "MIN", "MAX"}
	case fieldTypeTimestamp, fieldTypeDate:
		filters = []string{"EQ", "NEQ", "GT", "GTE", "LT", "LTE", "BETWEEN", "IS_NULL", "IS_NOT_NULL"}
		aggregates = []string{"COUNT", "MIN", "MAX"}
	}
	return filters, aggregates, projectable, groupable, sortable
}

// sourceNativeScopeWhere builds the authorized-scope WHERE clause shared by
// the bounded in-memory sample (sourceNativeScopeRows) and the scalable SQL
// executor (executeSourceNativePlanSQL). Every value is bound as a parameter;
// no request-derived string is ever concatenated into the SQL text itself.
func sourceNativeScopeWhere(req hybridQueryRequest, args []any) ([]string, []any, error) {
	if req.TenantID == "" || req.CollectionID == "" {
		return nil, nil, errors.New("source-native catalog requires an authorized database scope")
	}
	args = append(args, req.TenantID, req.CollectionID)
	where := []string{"r.tenant_id=$1", "r.collection_id=$2"}
	add := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if req.RecordType != "" {
		where = append(where, "r.record_type="+add(req.RecordType))
	}
	if req.SourceFile != "" {
		where = append(where, "r.source_file="+add(req.SourceFile))
	}
	if req.BatchID != "" {
		where = append(where, "r.batch_id::text="+add(req.BatchID))
	}
	if req.EvidenceID != "" {
		evidenceParam := add(req.EvidenceID)
		clause := "EXISTS (SELECT 1 FROM forensic.evidence_items e WHERE e.tenant_id=r.tenant_id AND e.collection_id=r.collection_id AND e.records_batch_id=r.batch_id AND e.evidence_id::text=" + evidenceParam
		if req.EvidenceVersionID != "" {
			clause += " AND e.current_version_id::text=" + add(req.EvidenceVersionID)
		}
		where = append(where, clause+")")
	}
	return where, args, nil
}

func sourceNativeScopeRows(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) ([]map[string]any, error) {
	if db == nil {
		return nil, errors.New("source-native catalog requires an authorized database scope")
	}
	where, args, err := sourceNativeScopeWhere(req, nil)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(ctx, db, req.TenantID, `SELECT r.record_id::text AS record_id,r.tenant_id,r.collection_id,r.file_id,r.batch_id::text AS batch_id,r.record_type::text AS record_type,r.timestamp,r.source_file,r.row_number,r.row_hash,r.raw_payload,r.metadata,'forensic.records'::text AS source_table,e.evidence_id,e.evidence_version_id FROM forensic.records r LEFT JOIN LATERAL (SELECT item.evidence_id::text AS evidence_id,item.current_version_id::text AS evidence_version_id FROM forensic.evidence_items item WHERE item.tenant_id=r.tenant_id AND item.collection_id=r.collection_id AND item.records_batch_id=r.batch_id ORDER BY item.evidence_id LIMIT 1) e ON true WHERE `+strings.Join(where, " AND ")+` ORDER BY r.record_id LIMIT 1001`, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) > sourceNativeInputRowLimit {
		return nil, fmt.Errorf("source-native input exceeds %d rows; narrow the authorized scope", sourceNativeInputRowLimit)
	}
	return rows, nil
}

// sourceNativeCatalogSampleRows returns up to sourceNativeInputRowLimit rows
// for field-shape/type discovery ONLY. Unlike sourceNativeScopeRows it never
// errors when the authorized scope has more rows than the sample size — a
// truncated sample is still enough to learn which fields exist and their
// type, since the actual computation runs as real SQL over the full scope
// (see executeSourceNativePlanSQL), never over this sample.
func sourceNativeCatalogSampleRows(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) ([]map[string]any, error) {
	if db == nil {
		return nil, errors.New("source-native catalog requires an authorized database scope")
	}
	where, args, err := sourceNativeScopeWhere(req, nil)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(ctx, db, req.TenantID, `SELECT r.record_id::text AS record_id,r.tenant_id,r.collection_id,r.file_id,r.batch_id::text AS batch_id,r.record_type::text AS record_type,r.timestamp,r.source_file,r.row_number,r.row_hash,r.raw_payload,r.metadata,'forensic.records'::text AS source_table,e.evidence_id,e.evidence_version_id FROM forensic.records r LEFT JOIN LATERAL (SELECT item.evidence_id::text AS evidence_id,item.current_version_id::text AS evidence_version_id FROM forensic.evidence_items item WHERE item.tenant_id=r.tenant_id AND item.collection_id=r.collection_id AND item.records_batch_id=r.batch_id ORDER BY item.evidence_id LIMIT 1) e ON true WHERE `+strings.Join(where, " AND ")+fmt.Sprintf(` ORDER BY r.record_id LIMIT %d`, sourceNativeInputRowLimit), args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func buildSourceNativeFieldCatalog(req hybridQueryRequest, rows []map[string]any) []FieldDescriptorV1 {
	observed := map[string]*sourceNativeObservedField{}
	for _, row := range rows {
		seen := map[string]bool{}
		for normalized, value := range sourceNativeRowValues(row) {
			if sourceNativeSensitive(normalized) {
				continue
			}
			item := observed[normalized]
			if item == nil {
				item = &sourceNativeObservedField{sourceNames: map[string]bool{}, families: map[string]bool{}, sources: map[string]bool{}, types: map[string]int{}}
				observed[normalized] = item
			}
			item.sourceNames[value.source] = true
			item.families[stringValueAny(row["record_type"])] = true
			item.sources[stringValueAny(row["source_file"])] = true
			if seen[normalized] {
				continue
			}
			seen[normalized] = true
			item.presence++
			kind := classifySourceNativeScalar(value.value)
			if kind == "" {
				item.nulls++
			} else {
				item.types[kind]++
			}
		}
	}
	catalog := make([]FieldDescriptorV1, 0, len(observed))
	for normalized, item := range observed {
		sourceNames := sortedSourceNativeSet(item.sourceNames)
		families := sortedSourceNativeSet(item.families)
		sources := sortedSourceNativeSet(item.sources)
		kind, incompatible := effectiveSourceNativeType(item.types)
		filters, aggregates, projectable, groupable, sortable := sourceNativeTypePolicy(kind, incompatible)
		catalog = append(catalog, FieldDescriptorV1{
			ContractVersion: fieldDescriptorContractV1, FieldID: sourceNativeFieldID(req, normalized),
			SourceName: sourceNames[0], SourceNames: sourceNames, NormalizedName: normalized,
			EffectiveType: kind, PresenceCount: item.presence, NullCount: item.nulls,
			IncompatibleValueCount: incompatible, Sensitivity: "STANDARD", RedactionState: "VISIBLE",
			AllowedFilters: filters, Projectable: projectable, Groupable: groupable,
			AllowedAggregates: aggregates, Sortable: sortable, FamilyProvenance: families,
			SourceProvenance: sources, EvidenceID: req.EvidenceID, EvidenceVersionID: req.EvidenceVersionID,
		})
	}
	sort.Slice(catalog, func(i, j int) bool { return catalog[i].FieldID < catalog[j].FieldID })
	return catalog
}

func sortedSourceNativeSet(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func discoverSourceNativeFieldCatalog(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) ([]FieldDescriptorV1, []map[string]any, error) {
	rows, err := sourceNativeScopeRows(ctx, db, req)
	if err != nil {
		return nil, nil, err
	}
	return buildSourceNativeFieldCatalog(req, rows), rows, nil
}

func retrieveSourceNativeFields(question string, catalog []FieldDescriptorV1) ([]FieldDescriptorV1, []SourceNativeFieldScoreV1) {
	type scored struct {
		field FieldDescriptorV1
		score float64
	}
	query := descriptorTerms(question)
	terms := make([]string, 0, len(query))
	for term := range query {
		terms = append(terms, term)
	}
	sort.Strings(terms)
	all := make([]scored, 0, len(catalog))
	for _, field := range catalog {
		parts := append([]string{field.NormalizedName, field.SourceName, field.EffectiveType}, field.SourceNames...)
		// A curated field carries a display name, a description and synonyms.
		// Those were written precisely so an analyst's wording finds the field,
		// but retrieval ignored them and matched only the raw column name, so
		// "protocol" could not find a field whose synonyms list "protocol".
		if field.Curated {
			parts = append(parts, field.DisplayName, field.Description)
			parts = append(parts, field.Synonyms...)
		}
		document := descriptorTerms(strings.Join(parts, " "))
		score := 0.0
		for _, term := range terms {
			if count := document[term]; count > 0 {
				score += float64(count)
				if term == field.NormalizedName || strings.Contains(field.NormalizedName, term) {
					score += 2
				}
			}
		}
		all = append(all, scored{field: field, score: score})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].score != all[j].score {
			return all[i].score > all[j].score
		}
		// Curated fields are the CONTRACT; uncurated observed columns exist only
		// as coverage fallback. Ties used to break alphabetically on field ID,
		// which silently decided this by PREFIX: `fld_00...` outranks
		// `ipdr.protocol` because "f" < "i". Every family whose prefix sorts
		// after "fld_" was starved of its own curated fields by the ~13
		// uncurated envelope columns (record_id, batch_id, row_hash, ...) that
		// all tie at score 0 — ipdr, subscriber, tower_location and transaction,
		// while cdr/anpr/access_log were untouched and hid the defect.
		// Measured 2026-09-22: "How many IPDR sessions are there?" was offered
		// 11 hashed fields and 1 curated one, with `ipdr.bytes` absent, so the
		// question was unanswerable at any model quality.
		if all[i].field.Curated != all[j].field.Curated {
			return all[i].field.Curated
		}
		return all[i].field.FieldID < all[j].field.FieldID
	})
	// Issue EVERY curated field for the family, then fill any remaining slots
	// with the best-ranked uncurated columns. The layer keeps curated sets small
	// (8-17 per family), so this stays bounded.
	//
	// A fixed top-12 cut curated fields on the alphabetical tie-break instead:
	// communications_cdr has 17 curated entries, so "How many calls did
	// 923001110001 make?" lost `cdr.msisdn` — 10th alphabetically — and the
	// question became unanswerable at any model quality. Measured 2026-09-22.
	// Curation is the contract; a column the analyst curated must never be
	// dropped in favour of `row_hash`.
	ordered := make([]scored, 0, len(all))
	for _, candidate := range all {
		if candidate.field.Curated {
			ordered = append(ordered, candidate)
		}
	}
	// UNCURATED COLUMNS FILL THE SPARE SLOTS -- restored 2026-09-25 after the
	// alternative was MEASURED AND REVERTED.
	//
	// They are the field-substitution vector: an uncurated column reaches the
	// generator as a hashed id with no description, so H12 ("average BEAM
	// WIDTH") compiled AVG over one and answered "the average AZIMUTH is
	// 209.97". Withholding them DID close that -- H12 stopped substituting --
	// and zero correct answers in either corpus referenced an uncurated field,
	// so the change looked free.
	//
	// It was not. Changing the issued enum changed what the generator produced
	// for OTHER questions: TWR-02 went CLARIFIED -> "There are no tower records
	// in this case", a FALSE NEGATIVE, because the new enum led it to invent a
	// `tower.district IS_NULL` filter alongside the correct site_code one.
	// Trading a fabricated field for a fabricated absence is not a trade this
	// product makes.
	//
	// The lesson generalises and is now twice-proven: the issued enum is not a
	// menu the generator picks from independently -- changing ANY part of it
	// changes plans everywhere. Curating the columns (WI-LAYER-1) removes the
	// substitution vector without changing what is offered for questions that
	// already work; that is the route, not withholding.
	for _, candidate := range all {
		if len(ordered) >= sourceNativeFieldTopK {
			break
		}
		if !candidate.field.Curated {
			ordered = append(ordered, candidate)
		}
	}
	// A computed metric is useless without the columns it is computed from, and
	// the executor resolves them through the SAME issued catalogue. Pull them in
	// so selecting the metric can never produce an unexecutable plan.
	byID := map[string]scored{}
	for _, candidate := range all {
		byID[candidate.field.FieldID] = candidate
	}
	present := map[string]bool{}
	for _, candidate := range ordered {
		present[candidate.field.FieldID] = true
	}
	for _, candidate := range append([]scored{}, ordered...) {
		for _, id := range candidate.field.DerivedFields {
			if dependency, ok := byID[id]; ok && !present[id] {
				present[id] = true
				ordered = append(ordered, dependency)
			}
		}
	}
	fields := make([]FieldDescriptorV1, len(ordered))
	scores := make([]SourceNativeFieldScoreV1, len(ordered))
	for index := range fields {
		fields[index] = ordered[index].field
		scores[index] = SourceNativeFieldScoreV1{FieldID: ordered[index].field.FieldID, Rank: index + 1, Score: ordered[index].score}
	}
	return fields, scores
}

func validateSourceNativePlanShape(plan *SourceNativePlanV1) error {
	if plan == nil {
		return nil
	}
	if plan.ContractVersion != sourceNativePlanContractV1 {
		return errors.New("invalid source-native plan contract")
	}
	if len(plan.Project) > sourceNativeProjectLimit || len(plan.Filters) > sourceNativeFilterLimit || len(plan.GroupFields) > 2 || len(plan.Measures) > sourceNativeMeasureLimit || len(plan.Having) > sourceNativeMeasureLimit || len(plan.Sort) > 2 {
		return errors.New("source-native plan exceeds a bounded collection limit")
	}
	complexity := len(plan.Project) + len(plan.Filters) + len(plan.GroupFields) + len(plan.Measures) + len(plan.Having) + len(plan.Sort)
	if plan.TimeBucket != nil {
		complexity++
	}
	if complexity == 0 || complexity > sourceNativeComplexityMax {
		return fmt.Errorf("source-native plan complexity must be between 1 and %d", sourceNativeComplexityMax)
	}
	if plan.Limit < 0 || plan.Limit > maxHybridLimit {
		return fmt.Errorf("source-native limit must be between 0 and %d", maxHybridLimit)
	}
	if len(plan.Project) > 0 && (len(plan.GroupFields) > 0 || len(plan.Measures) > 0 || plan.TimeBucket != nil || len(plan.Having) > 0) {
		return errors.New("source-native projection cannot combine aggregate controls")
	}
	if len(plan.Project) == 0 && len(plan.Measures) == 0 {
		return errors.New("source-native plan requires projection or measures")
	}
	for index, measure := range plan.Measures {
		if measure.MeasureID != fmt.Sprintf("m%d", index+1) {
			return errors.New("source-native measures require ordered issued IDs m1..m4")
		}
		if !containsString([]string{"COUNT", "COUNT_DISTINCT", "SUM", "AVG", "MIN", "MAX"}, strings.ToUpper(measure.Op)) {
			return errors.New("source-native aggregate is not allowlisted")
		}
		if strings.ToUpper(measure.Op) == "COUNT_DISTINCT" && measure.FieldID == "" {
			return errors.New("COUNT_DISTINCT requires the field whose values are counted")
		}
	}
	for _, sortSpec := range plan.Sort {
		if sortSpec.Target == "" || !containsString([]string{"ASC", "DESC"}, strings.ToUpper(sortSpec.Direction)) {
			return errors.New("source-native sort requires an issued target and direction")
		}
	}
	return nil
}

func validateSourceNativePlan(plan *SourceNativePlanV1, catalog []FieldDescriptorV1) error {
	if err := validateSourceNativePlanShape(plan); err != nil {
		return err
	}
	issued := map[string]FieldDescriptorV1{}
	for _, field := range catalog {
		issued[field.FieldID] = field
	}
	field := func(id string) (FieldDescriptorV1, error) {
		value, ok := issued[id]
		if !ok {
			return FieldDescriptorV1{}, fmt.Errorf("field_id %q was not issued for the authorized scope", id)
		}
		return value, nil
	}
	seen := map[string]bool{}
	for _, id := range plan.Project {
		value, err := field(id)
		if err != nil || !value.Projectable || seen[id] {
			return fmt.Errorf("source-native projection field is unavailable, repeated, or restricted: %s", id)
		}
		seen[id] = true
	}
	seen = map[string]bool{}
	for _, id := range plan.GroupFields {
		value, err := field(id)
		if err != nil || !value.Groupable || seen[id] {
			return fmt.Errorf("source-native group field is unavailable, repeated, or restricted: %s", id)
		}
		seen[id] = true
	}
	measures := map[string]SourceNativeMeasureV1{}
	for _, measure := range plan.Measures {
		op := strings.ToUpper(measure.Op)
		if measure.FieldID == "" {
			if op != "COUNT" {
				return errors.New("only COUNT may omit field_id")
			}
		} else {
			value, err := field(measure.FieldID)
			if err != nil || !containsString(value.AllowedAggregates, op) {
				return fmt.Errorf("aggregate %s is not allowed for field_id %q", op, measure.FieldID)
			}
		}
		measures[measure.MeasureID] = measure
	}
	if plan.TimeBucket != nil {
		value, err := field(plan.TimeBucket.FieldID)
		if err != nil || !containsString([]string{fieldTypeTimestamp, fieldTypeDate}, value.EffectiveType) {
			return errors.New("time bucket requires an issued timestamp/date field")
		}
		if !containsString([]string{"hour", "day", "week", "month"}, plan.TimeBucket.Bucket) {
			return errors.New("time bucket must be hour, day, week or month")
		}
		if _, err := time.LoadLocation(plan.TimeBucket.Timezone); err != nil {
			return errors.New("time bucket requires an explicit IANA timezone")
		}
	}
	for _, filter := range plan.Filters {
		value, err := field(filter.FieldID)
		op := strings.ToUpper(filter.Op)
		if err != nil || !containsString(value.AllowedFilters, op) {
			return fmt.Errorf("filter %s is not allowed for field_id %q", op, filter.FieldID)
		}
		if op == "IN" && (len(filter.Values) == 0 || len(filter.Values) > 20) {
			return errors.New("IN requires one to twenty exact values")
		}
		if op == "BETWEEN" && len(filter.Values) != 2 {
			return errors.New("BETWEEN requires exactly two values")
		}
		if (op == "IS_NULL" || op == "IS_NOT_NULL") && (filter.Value != "" || len(filter.Values) > 0) {
			return errors.New("null filters cannot carry values")
		}
	}
	for _, having := range plan.Having {
		measure, ok := measures[having.MeasureID]
		if !ok || !containsString([]string{"COUNT", "SUM", "AVG"}, strings.ToUpper(measure.Op)) || !containsString([]string{"GT", "GTE", "LT", "LTE", "EQ", "NEQ"}, strings.ToUpper(having.Op)) || !isCanonicalNumericLiteral(having.Value) {
			return errors.New("HAVING must reference an issued numeric aggregate and numeric threshold")
		}
	}
	for _, sortSpec := range plan.Sort {
		if _, ok := measures[sortSpec.Target]; ok {
			continue
		}
		value, err := field(sortSpec.Target)
		if err != nil || !value.Sortable || (!containsString(plan.GroupFields, sortSpec.Target) && !containsString(plan.Project, sortSpec.Target) && (plan.TimeBucket == nil || plan.TimeBucket.FieldID != sortSpec.Target)) {
			return fmt.Errorf("sort target %q is not an issued output", sortSpec.Target)
		}
	}
	return nil
}

func sourceNativeFieldMap(catalog []FieldDescriptorV1) map[string]FieldDescriptorV1 {
	out := make(map[string]FieldDescriptorV1, len(catalog))
	for _, field := range catalog {
		out[field.FieldID] = field
	}
	return out
}

func sourceNativeValue(row map[string]any, field FieldDescriptorV1) (any, bool) {
	for _, name := range field.SourceNames {
		if value, ok := row[name]; ok && normalizeSourceNativeName(name) == field.NormalizedName {
			return value, true
		}
	}
	for normalized, value := range sourceNativeRowValues(row) {
		if normalized == field.NormalizedName {
			return value.value, true
		}
	}
	return nil, false
}

func sourceNativeLocator(row map[string]any) map[string]any {
	locator := map[string]any{}
	for _, field := range canonicalProjectionProvenance {
		locator[field] = row[field]
	}
	locator["evidence_id"] = row["evidence_id"]
	locator["version_id"] = row["evidence_version_id"]
	return locator
}

func sourceNativeRat(value any) (*big.Rat, bool) {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" {
		return nil, false
	}
	rat := new(big.Rat)
	if _, ok := rat.SetString(text); !ok {
		return nil, false
	}
	return rat, true
}

func sourceNativeTime(value any, kind string) (time.Time, bool) {
	if typed, ok := value.(time.Time); ok {
		return typed, true
	}
	layout := time.RFC3339
	if kind == fieldTypeDate {
		layout = "2006-01-02"
	}
	parsed, err := time.Parse(layout, strings.TrimSpace(fmt.Sprint(value)))
	return parsed, err == nil
}

func sourceNativeCompare(value any, literal, kind, op string) bool {
	op = strings.ToUpper(op)
	if value == nil {
		return op == "IS_NULL"
	}
	if op == "IS_NOT_NULL" {
		return true
	}
	if kind == fieldTypeInteger || kind == fieldTypeDecimal {
		left, ok1 := sourceNativeRat(value)
		right, ok2 := sourceNativeRat(literal)
		if !ok1 || !ok2 {
			return false
		}
		return sourceNativeComparison(left.Cmp(right), op)
	}
	if kind == fieldTypeTimestamp || kind == fieldTypeDate {
		left, ok1 := sourceNativeTime(value, kind)
		right, ok2 := sourceNativeTime(literal, kind)
		if !ok1 || !ok2 {
			return false
		}
		cmp := 0
		if left.Before(right) {
			cmp = -1
		} else if left.After(right) {
			cmp = 1
		}
		return sourceNativeComparison(cmp, op)
	}
	left := fmt.Sprint(value)
	cmp := strings.Compare(left, literal)
	if op == "CONTAINS" {
		return strings.Contains(strings.ToLower(left), strings.ToLower(literal))
	}
	return sourceNativeComparison(cmp, op)
}

func sourceNativeComparison(cmp int, op string) bool {
	switch op {
	case "EQ":
		return cmp == 0
	case "NEQ":
		return cmp != 0
	case "GT":
		return cmp > 0
	case "GTE":
		return cmp >= 0
	case "LT":
		return cmp < 0
	case "LTE":
		return cmp <= 0
	}
	return false
}

func sourceNativeRowMatches(row map[string]any, plan *SourceNativePlanV1, fields map[string]FieldDescriptorV1) bool {
	for _, filter := range plan.Filters {
		field := fields[filter.FieldID]
		value, present := sourceNativeValue(row, field)
		if !present {
			value = nil
		}
		op := strings.ToUpper(filter.Op)
		matched := false
		switch op {
		case "IN":
			for _, candidate := range filter.Values {
				matched = matched || sourceNativeCompare(value, candidate, field.EffectiveType, "EQ")
			}
		case "BETWEEN":
			matched = sourceNativeCompare(value, filter.Values[0], field.EffectiveType, "GTE") && sourceNativeCompare(value, filter.Values[1], field.EffectiveType, "LTE")
		default:
			matched = sourceNativeCompare(value, filter.Value, field.EffectiveType, op)
		}
		if !matched {
			return false
		}
	}
	return true
}

func sourceNativeNumberResult(value *big.Rat) any {
	if value == nil {
		return nil
	}
	if value.IsInt() && value.Num().IsInt64() {
		return value.Num().Int64()
	}
	parsed, _ := strconv.ParseFloat(value.FloatString(10), 64)
	return parsed
}

func sourceNativeAggregate(rows []map[string]any, measure SourceNativeMeasureV1, fields map[string]FieldDescriptorV1) any {
	op := strings.ToUpper(measure.Op)
	if op == "COUNT" && measure.FieldID == "" {
		return len(rows)
	}
	field := fields[measure.FieldID]
	values := []any{}
	for _, row := range rows {
		if value, present := sourceNativeValue(row, field); present && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" {
			values = append(values, value)
		}
	}
	if op == "COUNT" {
		return len(values)
	}
	if len(values) == 0 {
		return nil
	}
	if op == "SUM" || op == "AVG" {
		total := new(big.Rat)
		for _, value := range values {
			rat, ok := sourceNativeRat(value)
			if !ok {
				return nil
			}
			total.Add(total, rat)
		}
		if op == "AVG" {
			total.Quo(total, big.NewRat(int64(len(values)), 1))
		}
		return sourceNativeNumberResult(total)
	}
	best := values[0]
	for _, candidate := range values[1:] {
		cmp := 0
		if field.EffectiveType == fieldTypeInteger || field.EffectiveType == fieldTypeDecimal {
			left, _ := sourceNativeRat(candidate)
			right, _ := sourceNativeRat(best)
			cmp = left.Cmp(right)
		} else if field.EffectiveType == fieldTypeTimestamp || field.EffectiveType == fieldTypeDate {
			left, _ := sourceNativeTime(candidate, field.EffectiveType)
			right, _ := sourceNativeTime(best, field.EffectiveType)
			if left.Before(right) {
				cmp = -1
			} else if left.After(right) {
				cmp = 1
			}
		}
		if op == "MIN" && cmp < 0 || op == "MAX" && cmp > 0 {
			best = candidate
		}
	}
	return best
}

func sourceNativeBucket(value any, field FieldDescriptorV1, spec *SourceNativeTimeBucketV1) (any, any, error) {
	if value == nil {
		return nil, nil, nil
	}
	stamp, ok := sourceNativeTime(value, field.EffectiveType)
	if !ok {
		return nil, nil, errors.New("source-native time value violates issued type")
	}
	start, end, err := canonicalTimeBucket(stamp, &CanonicalGroupV1{Field: "timestamp", Measure: "count", Bucket: spec.Bucket, Timezone: spec.Timezone})
	if err != nil {
		return nil, nil, err
	}
	return start.Format(time.RFC3339), end.Format(time.RFC3339), nil
}

func executeSourceNativePlan(req hybridQueryRequest, plan *SourceNativePlanV1, catalog []FieldDescriptorV1, rows []map[string]any) (map[string]any, error) {
	if err := validateSourceNativePlan(plan, catalog); err != nil {
		return nil, err
	}
	fields := sourceNativeFieldMap(catalog)
	filtered := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row["tenant_id"] != req.TenantID || row["collection_id"] != req.CollectionID {
			return nil, errors.New("source-native row escaped authorized scope")
		}
		if sourceNativeRowMatches(row, plan, fields) {
			filtered = append(filtered, row)
		}
	}
	limit := plan.Limit
	if limit == 0 {
		limit = defaultHybridLimit
	}
	results := []map[string]any{}
	if len(plan.Project) > 0 {
		for _, row := range filtered {
			out := map[string]any{}
			for _, id := range plan.Project {
				field := fields[id]
				value, _ := sourceNativeValue(row, field)
				out[field.NormalizedName] = value
			}
			out["metadata"] = map[string]any{"source_rows": []map[string]any{sourceNativeLocator(row)}}
			out["__tie"] = stringValueAny(row["record_id"])
			results = append(results, out)
		}
	} else {
		type group struct {
			values map[string]any
			end    any
			rows   []map[string]any
		}
		groups := map[string]*group{}
		if len(filtered) == 0 && len(plan.GroupFields) == 0 && plan.TimeBucket == nil {
			groups["all"] = &group{values: map[string]any{}, rows: []map[string]any{}}
		}
		for _, row := range filtered {
			values := map[string]any{}
			parts := []string{}
			for _, id := range plan.GroupFields {
				field := fields[id]
				value, _ := sourceNativeValue(row, field)
				values[field.NormalizedName] = value
				encoded, _ := json.Marshal(value)
				parts = append(parts, field.FieldID+":"+string(encoded))
			}
			var bucketEnd any
			if plan.TimeBucket != nil {
				field := fields[plan.TimeBucket.FieldID]
				value, _ := sourceNativeValue(row, field)
				start, end, err := sourceNativeBucket(value, field, plan.TimeBucket)
				if err != nil {
					return nil, err
				}
				values[field.NormalizedName] = start
				bucketEnd = end
				encoded, _ := json.Marshal(start)
				parts = append(parts, field.FieldID+":"+string(encoded))
			}
			key := strings.Join(parts, "|")
			bucket := groups[key]
			if bucket == nil {
				if len(groups) >= sourceNativeGroupLimit {
					return nil, fmt.Errorf("source-native group cardinality exceeds %d", sourceNativeGroupLimit)
				}
				bucket = &group{values: values, end: bucketEnd}
				groups[key] = bucket
			}
			bucket.rows = append(bucket.rows, row)
		}
		keys := make([]string, 0, len(groups))
		for key := range groups {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			bucket := groups[key]
			out := map[string]any{}
			for name, value := range bucket.values {
				out[name] = value
			}
			if plan.TimeBucket != nil {
				name := fields[plan.TimeBucket.FieldID].NormalizedName
				out[name+"_bucket_end_exclusive"] = bucket.end
				out["timezone"] = plan.TimeBucket.Timezone
			}
			for _, measure := range plan.Measures {
				out[measure.MeasureID] = sourceNativeAggregate(bucket.rows, measure, fields)
			}
			lineage := make([]map[string]any, 0, len(bucket.rows))
			for _, row := range bucket.rows {
				lineage = append(lineage, sourceNativeLocator(row))
			}
			sort.Slice(lineage, func(i, j int) bool {
				return stringValueAny(lineage[i]["record_id"]) < stringValueAny(lineage[j]["record_id"])
			})
			out["metadata"] = map[string]any{"source_rows": lineage, "contributing_row_count": len(lineage)}
			out["__tie"] = key
			keep := true
			for _, having := range plan.Having {
				keep = keep && sourceNativeCompare(out[having.MeasureID], having.Value, fieldTypeDecimal, having.Op)
			}
			if keep {
				results = append(results, out)
			}
		}
	}
	if len(plan.Sort) > 0 {
		sort.SliceStable(results, func(i, j int) bool {
			for _, spec := range plan.Sort {
				key := spec.Target
				if field, ok := fields[key]; ok {
					key = field.NormalizedName
				}
				cmp := strings.Compare(fmt.Sprint(results[i][key]), fmt.Sprint(results[j][key]))
				if left, ok1 := sourceNativeRat(results[i][key]); ok1 {
					if right, ok2 := sourceNativeRat(results[j][key]); ok2 {
						cmp = left.Cmp(right)
					}
				}
				if cmp != 0 {
					if strings.EqualFold(spec.Direction, "DESC") {
						return cmp > 0
					}
					return cmp < 0
				}
			}
			return stringValueAny(results[i]["__tie"]) < stringValueAny(results[j]["__tie"])
		})
	}
	totalResults := len(results)
	if len(results) > limit {
		results = results[:limit]
	}
	for _, row := range results {
		delete(row, "__tie")
	}
	return map[string]any{
		"contract_version": sourceNativeResultV1, "source_native_results": results,
		"row_count": len(results), "total_result_count": totalResults, "total_count": len(filtered),
		"scanned_source_rows": len(rows), "limit": limit, "complete": true,
		"field_catalog_contract": fieldDescriptorContractV1, "field_catalog": catalog,
		"plan": plan, "input_row_limit": sourceNativeInputRowLimit, "group_limit": sourceNativeGroupLimit,
		"provenance": canonicalProvenance(filtered, req.CollectionID),
	}, nil
}

// sourceNativeRecords is the sole live entry point for executing a compiled,
// already-validated SourceNativePlanV1 (see canonicalRecords). It samples up
// to sourceNativeInputRowLimit rows to learn the field catalog (shape/type
// only — never used to compute a result), then executes the plan as real
// parameterized SQL over the FULL authorized scope so filters/aggregates/
// groups are correct regardless of how many rows the collection actually
// has. This replaces the previous behavior of computing the answer over the
// same capped sample, which failed closed (safe, but useless) for any
// collection larger than 1000 rows — i.e. every real forensic collection.
func sourceNativeRecords(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	catalog := req.SourceNativeCatalog
	if len(catalog) == 0 {
		// No compile-time catalog was carried on the request (e.g. a stored/
		// replayed plan from a follow-up mutation) — sample fresh. When a
		// compile-time catalog IS present, it is always used instead of a new
		// sample, since a fresh sample can legitimately disagree with the one
		// the plan was validated against and wrongly reject a valid plan.
		rows, err := sourceNativeCatalogSampleRows(ctx, db, req)
		if err != nil {
			return nil, err
		}
		catalog = buildSourceNativeFieldCatalog(req, rows)
	}
	return executeSourceNativePlanSQL(ctx, db, req, req.SourceNative, catalog)
}
