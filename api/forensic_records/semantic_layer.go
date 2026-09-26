package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// The curated semantic layer. The runtime catalogue is inferred per request from
// sampled rows and carries no descriptions, synonyms, value domains, metrics or
// joins — `grep -c Description source_native_algebra.go` returns 0. Every element
// modelled here closes a measured failure:
//
//	source_names      SUB-02 answered 11 instead of 6, because `status` and
//	                  `account_status` hold the same concept under two headers.
//	values            "active" is not an identifier pattern, so literal extraction
//	                  never made it an obligation and the CONSTRAINT_APPLIED guard
//	                  had nothing to check.
//	metrics+expression CDR-15: there is NO duration column, only start and end.
//	default_measure   TXN-02: with nothing declared, a magnitude question can only
//	                  abstain.
//	distinct_capable  CDR-07/08 and ANPR-05 need a distinct count the IR lacks.
//
// The layer is DATA: YAML on disk, reviewable in a diff. This file only loads and
// validates it, and validation FAILS CLOSED.
const semanticLayerContractV1 = "forensics.semantic-layer/v1"

// Allowlisted derived-metric expressions. A metric never carries raw SQL and never
// names a table; anything outside this vocabulary is rejected.
var semanticLayerExpressionPattern = regexp.MustCompile(
	`^timestamp_diff_seconds\(\s*([a-z0-9_.]+)\s*,\s*([a-z0-9_.]+)\s*\)$`)

var (
	semanticLayerTypes        = map[string]bool{"STRING": true, "NUMBER": true, "TIMESTAMP": true, "DATE": true, "BOOLEAN": true}
	semanticLayerFilters      = map[string]bool{"EQ": true, "NEQ": true, "IN": true, "CONTAINS": true, "GT": true, "GTE": true, "LT": true, "LTE": true, "BETWEEN": true, "IS_NULL": true, "IS_NOT_NULL": true}
	semanticLayerAggregates   = map[string]bool{"COUNT": true, "SUM": true, "AVG": true, "MIN": true, "MAX": true, "COUNT_DISTINCT": true}
	semanticLayerSensitivity  = map[string]bool{"NONE": true, "IDENTIFIER": true, "PII": true, "RESTRICTED": true}
	semanticLayerRedaction    = map[string]bool{"": true, "NONE": true, "MASKED": true, "WITHHELD": true}
	semanticLayerCardinality  = map[string]bool{"one_to_one": true, "many_to_one": true, "one_to_many": true}
	semanticLayerNumericTypes = map[string]bool{"NUMBER": true}
)

type SemanticLayerValueV1 struct {
	Value       string   `yaml:"value" json:"value"`
	DisplayName string   `yaml:"display_name" json:"display_name"`
	Synonyms    []string `yaml:"synonyms" json:"synonyms"`
}

type SemanticLayerFieldV1 struct {
	ID                string                 `yaml:"id" json:"id"`
	DisplayName       string                 `yaml:"display_name" json:"display_name"`
	Description       string                 `yaml:"description" json:"description"`
	Synonyms          []string               `yaml:"synonyms" json:"synonyms"`
	SourceNames       []string               `yaml:"source_names" json:"source_names"`
	Type              string                 `yaml:"type" json:"type"`
	Sensitivity       string                 `yaml:"sensitivity" json:"sensitivity"`
	Redaction         string                 `yaml:"redaction" json:"redaction,omitempty"`
	AllowedFilters    []string               `yaml:"allowed_filters" json:"allowed_filters"`
	AllowedAggregates []string               `yaml:"allowed_aggregates" json:"allowed_aggregates"`
	Projectable       bool                   `yaml:"projectable" json:"projectable"`
	Groupable         bool                   `yaml:"groupable" json:"groupable"`
	Sortable          bool                   `yaml:"sortable" json:"sortable"`
	DistinctCapable   bool                   `yaml:"distinct_capable" json:"distinct_capable"`
	Values            []SemanticLayerValueV1 `yaml:"values" json:"values,omitempty"`
}

type SemanticLayerMetricV1 struct {
	ID          string   `yaml:"id" json:"id"`
	DisplayName string   `yaml:"display_name" json:"display_name"`
	Description string   `yaml:"description" json:"description"`
	Synonyms    []string `yaml:"synonyms" json:"synonyms"`
	Aggregate   string   `yaml:"aggregate" json:"aggregate"`
	FieldID     string   `yaml:"field_id" json:"field_id,omitempty"`
	Expression  string   `yaml:"expression" json:"expression,omitempty"`
	Unit        string   `yaml:"unit" json:"unit,omitempty"`
	Type        string   `yaml:"type" json:"type"`
}

type SemanticLayerJoinV1 struct {
	ID           string `yaml:"id" json:"id"`
	Description  string `yaml:"description" json:"description"`
	LeftFieldID  string `yaml:"left_field_id" json:"left_field_id"`
	RightFieldID string `yaml:"right_field_id" json:"right_field_id"`
	Cardinality  string `yaml:"cardinality" json:"cardinality"`
}

type SemanticLayerEntityV1 struct {
	ContractVersion string                  `yaml:"contract_version" json:"contract_version"`
	Family          string                  `yaml:"family" json:"family"`
	RecordType      string                  `yaml:"record_type" json:"record_type"`
	DisplayName     string                  `yaml:"display_name" json:"display_name"`
	Description     string                  `yaml:"description" json:"description"`
	Synonyms        []string                `yaml:"synonyms" json:"synonyms"`
	DefaultMeasure  string                  `yaml:"default_measure" json:"default_measure"`
	Source          SemanticLayerSourceV1   `yaml:"source" json:"source,omitzero"`
	Fields          []SemanticLayerFieldV1  `yaml:"fields" json:"fields"`
	Metrics         []SemanticLayerMetricV1 `yaml:"metrics" json:"metrics"`
	Joins           []SemanticLayerJoinV1   `yaml:"joins" json:"joins,omitempty"`
}

type SemanticLayerV1 struct {
	ContractVersion string                  `json:"contract_version"`
	Entities        []SemanticLayerEntityV1 `json:"entities"`

	fieldByID  map[string]SemanticLayerFieldV1
	metricByID map[string]SemanticLayerMetricV1
	entityByID map[string]SemanticLayerEntityV1
	// fieldOwner maps a field ID to the family that declares it. A field ID's
	// prefix is NOT its family ("cdr.call_type" belongs to
	// "communications_cdr"), so the owner has to be recorded rather than
	// parsed. The executor needs it to learn WHICH TABLE a plan reads.
	fieldOwner map[string]string
}

// EntityByFieldID resolves the entity that declares a field. The second result
// is false for an unknown field ID.
func (l *SemanticLayerV1) EntityByFieldID(id string) (SemanticLayerEntityV1, bool) {
	if l == nil {
		return SemanticLayerEntityV1{}, false
	}
	family, ok := l.fieldOwner[id]
	if !ok {
		return SemanticLayerEntityV1{}, false
	}
	entity, ok := l.entityByID[family]
	return entity, ok
}

// FieldByID resolves a curated field. The second result is false for an unknown ID,
// which callers must treat as a refusal, never as a reason to guess.
func (l *SemanticLayerV1) FieldByID(id string) (SemanticLayerFieldV1, bool) {
	field, ok := l.fieldByID[id]
	return field, ok
}

func (l *SemanticLayerV1) MetricByID(id string) (SemanticLayerMetricV1, bool) {
	metric, ok := l.metricByID[id]
	return metric, ok
}

func (l *SemanticLayerV1) EntityByFamily(family string) (SemanticLayerEntityV1, bool) {
	entity, ok := l.entityByID[family]
	return entity, ok
}

// ValueLiterals returns every enumerated value of every field, lowercased, mapped to
// the field that declares it. This is what lets literal extraction treat "active" as
// an obligation: without it, a value literal is invisible and the filter is silently
// dropped (the SUB-02 failure).
func (l *SemanticLayerV1) ValueLiterals(family string) map[string]string {
	out := map[string]string{}
	entity, ok := l.entityByID[family]
	if !ok {
		return out
	}
	for _, field := range entity.Fields {
		for _, value := range field.Values {
			out[strings.ToLower(strings.TrimSpace(value.Value))] = field.ID
			for _, synonym := range value.Synonyms {
				if trimmed := strings.ToLower(strings.TrimSpace(synonym)); trimmed != "" {
					out[trimmed] = field.ID
				}
			}
		}
	}
	return out
}

// LoadSemanticLayer reads and validates every *.yaml in dir. It fails closed: a layer
// that does not validate is never partially returned, because a half-loaded catalogue
// would silently narrow what an analyst can ask about.
func LoadSemanticLayer(dir string) (*SemanticLayerV1, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("semantic layer: %w", err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("semantic layer: no entity files in %s", dir)
	}
	layer := &SemanticLayerV1{ContractVersion: semanticLayerContractV1}
	for _, path := range paths {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("semantic layer %s: %w", filepath.Base(path), readErr)
		}
		var entity SemanticLayerEntityV1
		decoder := yaml.NewDecoder(strings.NewReader(string(raw)))
		decoder.KnownFields(true)
		if decodeErr := decoder.Decode(&entity); decodeErr != nil {
			return nil, fmt.Errorf("semantic layer %s: %w", filepath.Base(path), decodeErr)
		}
		layer.Entities = append(layer.Entities, entity)
	}
	if err := layer.validate(); err != nil {
		return nil, err
	}
	return layer, nil
}

func (l *SemanticLayerV1) validate() error {
	l.fieldByID = map[string]SemanticLayerFieldV1{}
	l.metricByID = map[string]SemanticLayerMetricV1{}
	l.entityByID = map[string]SemanticLayerEntityV1{}
	l.fieldOwner = map[string]string{}
	fieldOwner := l.fieldOwner

	for _, entity := range l.Entities {
		where := "entity " + entity.Family
		if entity.ContractVersion != semanticLayerContractV1 {
			return fmt.Errorf("%s: contract_version must be %s", where, semanticLayerContractV1)
		}
		if entity.Family == "" || entity.RecordType == "" || entity.DisplayName == "" {
			return fmt.Errorf("%s: family, record_type and display_name are required", where)
		}
		if strings.TrimSpace(entity.Description) == "" {
			return fmt.Errorf("%s: a description is required — an undescribed entity is why the inferred catalogue fails", where)
		}
		// WHERE THE ROWS LIVE. Absent, this defaults to forensic.records and
		// every pre-existing entity is unchanged.
		if err := validateSemanticSource(where, entity.Source); err != nil {
			return err
		}
		if _, clash := l.entityByID[entity.Family]; clash {
			return fmt.Errorf("%s: duplicate family", where)
		}
		l.entityByID[entity.Family] = entity
		if len(entity.Fields) == 0 {
			return fmt.Errorf("%s: no fields", where)
		}

		// A synonym that resolves to two fields in the same entity is ambiguous, and
		// an ambiguous synonym is exactly how a filter binds to the wrong field.
		synonymOwner := map[string]string{}
		for _, field := range entity.Fields {
			if err := validateSemanticField(where, field, entity.Source.IsDerived()); err != nil {
				return err
			}
			if owner, clash := fieldOwner[field.ID]; clash {
				return fmt.Errorf("%s: field_id %s already declared by %s", where, field.ID, owner)
			}
			fieldOwner[field.ID] = entity.Family
			l.fieldByID[field.ID] = field
			for _, synonym := range append([]string{field.DisplayName}, field.Synonyms...) {
				key := strings.ToLower(strings.TrimSpace(synonym))
				if key == "" {
					continue
				}
				if owner, clash := synonymOwner[key]; clash && owner != field.ID {
					return fmt.Errorf("%s: synonym %q maps to both %s and %s; an ambiguous synonym binds filters to the wrong field", where, key, owner, field.ID)
				}
				synonymOwner[key] = field.ID
			}
		}

		for _, metric := range entity.Metrics {
			if err := l.validateSemanticMetric(where, metric); err != nil {
				return err
			}
			if _, clash := l.metricByID[metric.ID]; clash {
				return fmt.Errorf("%s: duplicate metric %s", where, metric.ID)
			}
			l.metricByID[metric.ID] = metric
		}

		if entity.DefaultMeasure == "" {
			return fmt.Errorf("%s: default_measure is required; without one a magnitude question can only abstain", where)
		}
	}

	// Resolved last: a default measure or join may reference another entity's fields.
	for _, entity := range l.Entities {
		where := "entity " + entity.Family
		if _, ok := l.metricByID[entity.DefaultMeasure]; !ok {
			if _, isField := l.fieldByID[entity.DefaultMeasure]; !isField {
				return fmt.Errorf("%s: default_measure %s resolves to no metric or field", where, entity.DefaultMeasure)
			}
		}
		for _, join := range entity.Joins {
			left, leftOK := l.fieldByID[join.LeftFieldID]
			right, rightOK := l.fieldByID[join.RightFieldID]
			if !leftOK || !rightOK {
				return fmt.Errorf("%s: join %s references an unknown field", where, join.ID)
			}
			if fieldOwner[left.ID] == fieldOwner[right.ID] {
				return fmt.Errorf("%s: join %s has both sides in the same entity", where, join.ID)
			}
			if left.Type != right.Type {
				return fmt.Errorf("%s: join %s joins %s to %s across differing types", where, join.ID, left.Type, right.Type)
			}
			if !semanticLayerCardinality[join.Cardinality] {
				return fmt.Errorf("%s: join %s has an unknown cardinality %q", where, join.ID, join.Cardinality)
			}
		}
	}
	return nil
}

func validateSemanticField(where string, field SemanticLayerFieldV1, derived bool) error {
	if field.ID == "" || field.DisplayName == "" {
		return fmt.Errorf("%s: every field needs an id and a display_name", where)
	}
	if strings.TrimSpace(field.Description) == "" {
		return fmt.Errorf("%s: field %s has no description; that omission is the defect this layer exists to fix", where, field.ID)
	}
	if len(field.SourceNames) == 0 {
		return fmt.Errorf("%s: field %s declares no source_names, so it can never bind to a row", where, field.ID)
	}
	// A derived entity may name a NESTED path ("observation.start_seconds"),
	// because an observation keeps its analytical fields under one object. A
	// records entity may not: a dotted name there resolves to a key that does
	// not exist, and the field reads NULL for every row -- an entity that
	// validates, loads, and answers every question about it with "no value".
	for _, name := range field.SourceNames {
		if err := validateSemanticSourceFieldName(where+" field "+field.ID, name, derived); err != nil {
			return err
		}
	}
	if !semanticLayerTypes[field.Type] {
		return fmt.Errorf("%s: field %s has unknown type %q", where, field.ID, field.Type)
	}
	if !semanticLayerSensitivity[field.Sensitivity] {
		return fmt.Errorf("%s: field %s has unknown sensitivity %q", where, field.ID, field.Sensitivity)
	}
	if !semanticLayerRedaction[field.Redaction] {
		return fmt.Errorf("%s: field %s has unknown redaction %q", where, field.ID, field.Redaction)
	}
	if field.Sensitivity == "PII" && field.Redaction == "" {
		return fmt.Errorf("%s: field %s is PII and must declare a redaction", where, field.ID)
	}
	for _, filter := range field.AllowedFilters {
		if !semanticLayerFilters[filter] {
			return fmt.Errorf("%s: field %s allows unknown filter %q", where, field.ID, filter)
		}
	}
	for _, aggregate := range field.AllowedAggregates {
		if !semanticLayerAggregates[aggregate] {
			return fmt.Errorf("%s: field %s allows unknown aggregate %q", where, field.ID, aggregate)
		}
		// SUM or AVG over text is meaningless and would execute as a silent zero.
		if (aggregate == "SUM" || aggregate == "AVG") && !semanticLayerNumericTypes[field.Type] {
			return fmt.Errorf("%s: field %s is %s and cannot allow %s", where, field.ID, field.Type, aggregate)
		}
	}
	seen := map[string]bool{}
	for _, value := range field.Values {
		if strings.TrimSpace(value.Value) == "" {
			return fmt.Errorf("%s: field %s declares an empty enumerated value", where, field.ID)
		}
		if seen[value.Value] {
			return fmt.Errorf("%s: field %s declares duplicate value %q", where, field.ID, value.Value)
		}
		seen[value.Value] = true
	}
	return nil
}

func (l *SemanticLayerV1) validateSemanticMetric(where string, metric SemanticLayerMetricV1) error {
	if metric.ID == "" || metric.DisplayName == "" {
		return fmt.Errorf("%s: every metric needs an id and a display_name", where)
	}
	if strings.TrimSpace(metric.Description) == "" {
		return fmt.Errorf("%s: metric %s has no description", where, metric.ID)
	}
	if !semanticLayerAggregates[metric.Aggregate] {
		return fmt.Errorf("%s: metric %s has unknown aggregate %q", where, metric.ID, metric.Aggregate)
	}
	if metric.Expression != "" {
		match := semanticLayerExpressionPattern.FindStringSubmatch(metric.Expression)
		if match == nil {
			return fmt.Errorf("%s: metric %s uses an expression outside the allowlist: %q", where, metric.ID, metric.Expression)
		}
		for _, ref := range match[1:] {
			field, ok := l.fieldByID[ref]
			if !ok {
				return fmt.Errorf("%s: metric %s references unknown field %s", where, metric.ID, ref)
			}
			if field.Type != "TIMESTAMP" && field.Type != "DATE" {
				return fmt.Errorf("%s: metric %s takes a time difference of non-time field %s", where, metric.ID, ref)
			}
		}
		if metric.FieldID != "" {
			return fmt.Errorf("%s: metric %s declares both an expression and a field_id", where, metric.ID)
		}
		return nil
	}
	if metric.Aggregate == "COUNT" {
		// A plain row count takes no field.
		return nil
	}
	if metric.FieldID == "" {
		return fmt.Errorf("%s: metric %s needs a field_id or an expression", where, metric.ID)
	}
	field, ok := l.fieldByID[metric.FieldID]
	if !ok {
		return fmt.Errorf("%s: metric %s references unknown field %s", where, metric.ID, metric.FieldID)
	}
	if metric.Aggregate == "COUNT_DISTINCT" {
		if !field.DistinctCapable {
			return fmt.Errorf("%s: metric %s counts distinct values of %s, which is not marked distinct_capable", where, metric.ID, field.ID)
		}
		return nil
	}
	if !containsString(field.AllowedAggregates, metric.Aggregate) {
		return fmt.Errorf("%s: metric %s applies %s to %s, which does not allow it", where, metric.ID, metric.Aggregate, field.ID)
	}
	return nil
}

// --- wiring the layer into the compiler -------------------------------------

// semanticLayerDirEnv lets a deployment point at the curated YAML. The layer is
// OPTIONAL by construction: when it is absent the compiler falls back to the
// inferred catalogue and the product behaves exactly as before. A missing layer
// must never take the product down.
const semanticLayerDirEnv = "FORENSIC_SEMANTIC_LAYER_DIR"

var semanticLayerSearchPaths = []string{
	"/semantic_layer", // container
	"semantic_layer",  // repo root
	filepath.Join("..", "..", "semantic_layer"), // package tests
}

// CatalogFields converts one curated entity into the FieldDescriptorV1 shape the
// executor and the typed plan already speak.
//
// Two deliberate choices:
//
//   - SourceNames carries EVERY declared alias, and sourceNativeValue already
//     walks that list. This is what makes "how many subscribers are active"
//     return 6 instead of 5: `status` and `account_status` become one field.
//   - PII fields are EXCLUDED. The inferred catalogue drops them by name
//     (sourceNativeSensitive), and widening dynamic exposure to masked personal
//     data is not something this work item can verify end to end. Projection of
//     PII stays on the curated template path.
func (e SemanticLayerEntityV1) CatalogFields(req hybridQueryRequest) []FieldDescriptorV1 {
	out := make([]FieldDescriptorV1, 0, len(e.Fields))
	for _, field := range e.Fields {
		if field.Sensitivity == "PII" || field.Sensitivity == "RESTRICTED" {
			continue
		}
		names := append([]string(nil), field.SourceNames...)
		sensitivity, redaction := "STANDARD", "VISIBLE"
		if field.Sensitivity == "IDENTIFIER" {
			sensitivity = "IDENTIFIER"
		}
		out = append(out, FieldDescriptorV1{
			ContractVersion:   fieldDescriptorContractV1,
			FieldID:           field.ID,
			DisplayName:       field.DisplayName,
			Description:       field.Description,
			Synonyms:          append([]string(nil), field.Synonyms...),
			Curated:           true,
			SourceName:        names[0],
			SourceNames:       names,
			NormalizedName:    normalizeSourceNativeName(names[0]),
			EffectiveType:     semanticLayerEffectiveType(field.Type),
			Sensitivity:       sensitivity,
			RedactionState:    redaction,
			AllowedFilters:    append([]string(nil), field.AllowedFilters...),
			AllowedAggregates: semanticLayerPlanAggregates(field.AllowedAggregates, field.DistinctCapable),
			Projectable:       field.Projectable,
			Groupable:         field.Groupable,
			Sortable:          field.Sortable,
			FamilyProvenance:  []string{e.RecordType},
			EvidenceID:        req.EvidenceID,
			EvidenceVersionID: req.EvidenceVersionID,
		})
	}
	// Curated metrics that are COMPUTED become catalogue fields of their own, so
	// the generator can select them exactly like a stored column. Only
	// measurable: a derived value has no stored column to filter or group on.
	for _, metric := range e.Metrics {
		parts := semanticLayerExpressionPattern.FindStringSubmatch(metric.Expression)
		if metric.Expression == "" || parts == nil {
			continue
		}
		out = append(out, FieldDescriptorV1{
			ContractVersion:   fieldDescriptorContractV1,
			FieldID:           metric.ID,
			DisplayName:       metric.DisplayName,
			Description:       metric.Description,
			Synonyms:          append([]string(nil), metric.Synonyms...),
			Curated:           true,
			SourceName:        metric.ID,
			SourceNames:       []string{metric.ID},
			NormalizedName:    normalizeSourceNativeName(metric.ID),
			EffectiveType:     semanticLayerEffectiveType(metric.Type),
			Sensitivity:       "STANDARD",
			RedactionState:    "VISIBLE",
			AllowedFilters:    nil,
			AllowedAggregates: []string{"MIN", "MAX", "AVG", "SUM", "COUNT"},
			Projectable:       false,
			Groupable:         false,
			Sortable:          true,
			DerivedOp:         "timestamp_diff_seconds",
			DerivedFields:     []string{parts[1], parts[2]},
			FamilyProvenance:  []string{e.RecordType},
			EvidenceID:        req.EvidenceID,
			EvidenceVersionID: req.EvidenceVersionID,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FieldID < out[j].FieldID })
	return out
}

// semanticLayerPlanAggregates advertises what the TYPED PLAN can actually
// execute. COUNT_DISTINCT used to be stripped here because the plan had no
// distinct aggregate; it has one now, so a field the layer marks
// distinct_capable offers it.
// semanticLayerEffectiveType translates a LAYER type to the type the executor
// reasons about. The layer declares NUMBER; the SQL builder switches on
// INTEGER/DECIMAL and sends anything else down the raw-text path.
//
// So every curated numeric column was compared AS TEXT: `MAX(transaction.amount)`
// returned the lexicographically largest string, which is why "what was the
// largest transaction" answered 9,200 rather than 75,000 -- "9" sorts after "7".
// That was recorded as the generator being unstable across builds and is the
// stated reason the IR fallback shipped disabled. It was a type mapping.
// Measured and fixed 2026-09-22.
func semanticLayerEffectiveType(declared string) string {
	if declared == "NUMBER" {
		return fieldTypeDecimal
	}
	return declared
}

func semanticLayerPlanAggregates(declared []string, distinctCapable bool) []string {
	out := make([]string, 0, len(declared)+1)
	seen := map[string]bool{}
	for _, aggregate := range declared {
		if aggregate == "COUNT_DISTINCT" && !distinctCapable {
			continue
		}
		if !seen[aggregate] {
			seen[aggregate] = true
			out = append(out, aggregate)
		}
	}
	if distinctCapable && !seen["COUNT_DISTINCT"] {
		out = append(out, "COUNT_DISTINCT")
	}
	return out
}

// mergeSemanticLayerCatalog puts curated fields first and keeps any inferred
// field the layer does not already describe, matched on normalized source name.
// Curation must add meaning without removing reach: a field present in the data
// but absent from the YAML stays queryable, just undescribed.
func mergeSemanticLayerCatalog(curated, inferred []FieldDescriptorV1) []FieldDescriptorV1 {
	covered := map[string]bool{}
	for _, field := range curated {
		for _, name := range field.SourceNames {
			covered[normalizeSourceNativeName(name)] = true
		}
	}
	out := append([]FieldDescriptorV1(nil), curated...)
	for _, field := range inferred {
		if covered[field.NormalizedName] {
			continue
		}
		out = append(out, field)
	}
	return out
}

// semanticLayerValueFilters turns enumerated value literals present in the
// question into typed filters.
//
// This is the SUB-02 class. "How many subscribers are active" carries no
// identifier pattern, so literal extraction never saw "active", the
// CONSTRAINT_APPLIED guard had nothing to check, and the answer widened to every
// subscriber. A value is only bound when the layer declares it, so nothing is
// ever invented.
// semanticLayerValueGrouping reports fields whose question mentions MORE THAN ONE
// of their declared values. "Incoming versus outgoing" names two values of
// cdr.direction, and that is a request to break down BY direction, not to filter
// to one of them. Filtering there answers a narrower question than was asked —
// it returned the incoming count alone where a breakdown was wanted.

// _semanticLayerValueEchoesFieldName reports whether a declared value is
// indistinguishable from its own field's name. CALL is a value of cdr.call_type,
// whose name tokenises to "call" + "type", so the phrase "where call_type is
// SMS" appears to name two values and was misread as a breakdown. A value that
// merely echoes its field's name is no evidence the analyst named that value.
func _semanticLayerValueEchoesFieldName(field SemanticLayerFieldV1, phrase string) bool {
	name := map[string]bool{}
	for _, part := range append([]string{field.DisplayName, field.ID}, field.Synonyms...) {
		for _, token := range semanticWords.FindAllString(strings.ToLower(part), -1) {
			name[token] = true
		}
	}
	tokens := semanticWords.FindAllString(strings.ToLower(phrase), -1)
	if len(tokens) == 0 {
		return false
	}
	for _, token := range tokens {
		if !name[token] {
			return false
		}
	}
	return true
}

func semanticLayerValueGrouping(question string, entity SemanticLayerEntityV1) []string {
	stems := _semanticLayerQuestionTokens(question)
	grouped := []string{}
	for _, field := range entity.Fields {
		matched := 0
		for _, value := range field.Values {
			for _, phrase := range append([]string{value.Value}, value.Synonyms...) {
				if _semanticLayerValueEchoesFieldName(field, phrase) {
					continue
				}
				if _semanticLayerPhraseInQuestion(phrase, stems) {
					matched++
					break
				}
			}
		}
		if matched > 1 {
			grouped = append(grouped, field.ID)
		}
	}
	return grouped
}

func semanticLayerValueFilters(question string, entity SemanticLayerEntityV1, catalog []FieldDescriptorV1) []SourceNativeFilterV1 {
	multi := map[string]bool{}
	for _, id := range semanticLayerValueGrouping(question, entity) {
		multi[id] = true
	}
	fields := map[string]FieldDescriptorV1{}
	for _, field := range catalog {
		fields[field.FieldID] = field
	}
	stems := _semanticLayerQuestionTokens(question)
	filters := []SourceNativeFilterV1{}
	bound := map[string]bool{}
	for _, field := range entity.Fields {
		descriptor, ok := fields[field.ID]
		if !ok || bound[field.ID] || multi[field.ID] {
			continue
		}
		for _, value := range field.Values {
			matched := ""
			for _, phrase := range append([]string{value.Value}, value.Synonyms...) {
				if _semanticLayerValueEchoesFieldName(field, phrase) {
					continue
				}
				if _semanticLayerPhraseInQuestion(phrase, stems) {
					matched = value.Value
					break
				}
			}
			if matched == "" {
				continue
			}
			if !containsString(descriptor.AllowedFilters, "EQ") {
				continue
			}
			filters = append(filters, SourceNativeFilterV1{FieldID: field.ID, Op: "EQ", Value: matched, Values: []string{}})
			bound[field.ID] = true
			break
		}
	}
	return filters
}

func _semanticLayerQuestionTokens(question string) map[string]bool {
	out := map[string]bool{}
	for _, token := range semanticWords.FindAllString(strings.ToLower(question), -1) {
		out[token] = true
	}
	return out
}

// A phrase matches only when every one of its tokens appears. Single-token
// phrases shorter than three characters are ignored: "in" or "no" would
// otherwise bind a filter from ordinary English.
func _semanticLayerPhraseInQuestion(phrase string, stems map[string]bool) bool {
	tokens := semanticWords.FindAllString(strings.ToLower(strings.TrimSpace(phrase)), -1)
	if len(tokens) == 0 {
		return false
	}
	if len(tokens) == 1 && len(tokens[0]) < 3 {
		return false
	}
	for _, token := range tokens {
		if !stems[token] {
			return false
		}
	}
	return true
}

var (
	semanticLayerOnce   sync.Once
	semanticLayerCached *SemanticLayerV1
	semanticLayerLoaded string
)

// defaultSemanticLayer loads the curated layer once per process. It returns nil
// when no layer is deployed, and the caller then uses the inferred catalogue —
// the layer improves answers, it is never required to serve a request.
//
// A layer that is PRESENT but INVALID is a different matter: that is a curation
// error and it is reported, not silently ignored, because a half-trusted
// catalogue is worse than none.
func defaultSemanticLayer() (*SemanticLayerV1, string) {
	semanticLayerOnce.Do(func() {
		candidates := semanticLayerSearchPaths
		if configured := strings.TrimSpace(os.Getenv(semanticLayerDirEnv)); configured != "" {
			candidates = append([]string{configured}, candidates...)
		}
		for _, dir := range candidates {
			if entries, err := filepath.Glob(filepath.Join(dir, "*.yaml")); err != nil || len(entries) == 0 {
				continue
			}
			layer, err := LoadSemanticLayer(dir)
			if err != nil {
				semanticLayerLoaded = "semantic_layer_invalid:" + err.Error()
				return
			}
			semanticLayerCached, semanticLayerLoaded = layer, "semantic_layer:"+dir
			return
		}
		semanticLayerLoaded = "semantic_layer_absent"
	})
	return semanticLayerCached, semanticLayerLoaded
}

// semanticLayerCatalogForRequest returns the catalogue the compiler should use:
// curated fields for the question's family, merged with any inferred field the
// layer does not describe. The second result reports which path was taken so the
// decision is auditable rather than invisible.
func semanticLayerCatalogForRequest(req hybridQueryRequest, question string, inferred []FieldDescriptorV1) ([]FieldDescriptorV1, string) {
	family := semanticQuestionFamily(question)
	if family == "" {
		family = capabilityFamilyForRecordType(req.RecordType)
	}
	return semanticLayerCatalogForFamily(req, family, inferred)
}

// semanticLayerCatalogForFamily is the same selection with the family already
// resolved. Split out so the derived-isolation boundary can be asserted for
// EVERY curated family, not only the ones some question happens to route to --
// the next collision will be with a different column name.
func semanticLayerCatalogForFamily(req hybridQueryRequest, family string, inferred []FieldDescriptorV1) ([]FieldDescriptorV1, string) {
	layer, state := defaultSemanticLayer()
	if layer == nil {
		return inferred, state
	}
	entity, ok := layer.EntityByFamily(family)
	if !ok {
		return inferred, "semantic_layer_no_entity_for_family"
	}
	curated := entity.CatalogFields(req)
	if len(curated) == 0 {
		return inferred, "semantic_layer_entity_empty"
	}
	// A DERIVED FAMILY IS ISSUED ITS CURATED FIELDS AND NOTHING ELSE.
	//
	// `inferred` is discovered from forensic.records. For a STRUCTURED family
	// those are the right table's columns and merging them is what keeps an
	// uncurated column answerable -- withholding them was measured and rejected
	// (it cost TWR-02 a false negative, "there are no tower records in this
	// case"). For a DERIVED family they are the WRONG TABLE, and no plan can
	// ever use one correctly.
	//
	// MEASURED LIVE, Step 4, 2026-09-26. "What is the average OCR confidence of
	// the PLATE READS?" resolved `anpr_model_observation` -- correctly -- and
	// the catalogue then issued 6 curated derived fields PLUS the records column
	// `ocr_confidence` as `fld_22a22183f7a9e0024f970ca6`. The model chose the
	// records field, `sourceNativeBindingForFieldIDs` followed the plan's fields
	// to forensic.records exactly as designed, and a question about 307 MODEL
	// plate reads was answered 0.89 from the INGESTED camera sightings. Truth
	// was 0.955573.
	//
	// Nothing refused it, because the plan did not MIX sources -- it was purely
	// records. The mixed-plan refusal guards the wrong step; the mixture has to
	// not be OFFERED. The citation was honest and named records, which is what
	// made it diagnosable: a provenance label cannot catch a mis-bound plan, it
	// can only stop one from lying about where it read.
	//
	// This is inert unless a derived family resolves, which needs
	// FORENSIC_MEDIA_FAMILY_ROUTING. Structured families keep the merge exactly.
	if entity.Source.IsDerived() {
		return curated, "semantic_layer_catalog_derived_only:" + family
	}
	return mergeSemanticLayerCatalog(curated, inferred), "semantic_layer_catalog:" + family
}

// appendSourceNativeValueFilters adds a curated value-literal filter for any
// field the question names by value and that no extracted filter already binds.
// It is additive and conservative: an already-bound field is never overridden.
// semanticLayerGroupFieldForQuestion returns the curated field a question asks to
// be broken down by, when it names several of that field's values.
func semanticLayerGroupFieldForQuestion(question string, catalog []FieldDescriptorV1) (string, bool) {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return "", false
	}
	entity, ok := layer.EntityByFamily(semanticQuestionFamily(question))
	if !ok {
		return "", false
	}
	known := map[string]bool{}
	for _, field := range catalog {
		known[field.FieldID] = field.Groupable
	}
	for _, id := range semanticLayerValueGrouping(question, entity) {
		if known[id] {
			return id, true
		}
	}
	return "", false
}

func appendSourceNativeValueFilters(existing []SourceNativeFilterV1, question string, catalog []FieldDescriptorV1) []SourceNativeFilterV1 {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return existing
	}
	family := semanticQuestionFamily(question)
	entity, ok := layer.EntityByFamily(family)
	if !ok {
		return existing
	}
	alreadyBound := map[string]bool{}
	for _, filter := range existing {
		alreadyBound[filter.FieldID] = true
	}
	out := append([]SourceNativeFilterV1(nil), existing...)
	for _, filter := range semanticLayerValueFilters(question, entity, catalog) {
		if alreadyBound[filter.FieldID] || len(out) >= sourceNativeFilterLimit {
			continue
		}
		out = append(out, filter)
		alreadyBound[filter.FieldID] = true
	}
	return out
}

// semanticLayerQuestionNamesValue reports whether the question names a value the
// layer declares. It consults the layer alone — no catalogue, no database — so it
// is cheap enough to widen the decision about whether to build a catalogue at all.
//
// Without this the catalogue is skipped whenever a registered operation matches,
// the layer is therefore never consulted, and "how many subscribers are active"
// keeps returning the template's breakdown of all 11.
func semanticLayerQuestionNamesValue(question string) bool {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return false
	}
	entity, ok := layer.EntityByFamily(semanticQuestionFamily(question))
	if !ok {
		return false
	}
	stems := _semanticLayerQuestionTokens(question)
	for _, field := range entity.Fields {
		for _, value := range field.Values {
			for _, phrase := range append([]string{value.Value}, value.Synonyms...) {
				if _semanticLayerPhraseInQuestion(phrase, stems) {
					return true
				}
			}
		}
	}
	return false
}

// sourceNativeFieldByCuratedName resolves a hint against the curated display name
// and synonyms by EXACT normalized match, and nothing else.
//
// Without it the curated vocabulary is dead weight for field resolution:
// sourceNativeFieldByHint scores against NormalizedName only, so the hint
// "phone number" scored against "msisdn" and resolved to cdr.call_type — which
// is how "which phone number made the most calls" lost its grouping entirely.
//
// Exact match only, deliberately. A near-match here would be a similarity
// judgement on a structural role, which is the defect D2 and the measure-hint
// defect both came from.
func sourceNativeFieldByCuratedName(hint string, catalog []FieldDescriptorV1, allow func(FieldDescriptorV1) bool) (FieldDescriptorV1, bool) {
	want := normalizeSourceNativeName(hint)
	if want == "" {
		return FieldDescriptorV1{}, false
	}
	for _, field := range catalog {
		if !field.Curated || (allow != nil && !allow(field)) {
			continue
		}
		if normalizeSourceNativeName(field.DisplayName) == want {
			return field, true
		}
		for _, synonym := range field.Synonyms {
			if normalizeSourceNativeName(synonym) == want {
				return field, true
			}
		}
	}
	return FieldDescriptorV1{}, false
}

// semanticQuestionNamesMultipleFamilies reports whether a question mentions more
// than one curated evidence family.
//
// "Show tower activity for 923001110001" names the tower family by word and the
// CDR family by identifier: it is a CDR question with a tower dimension, exactly
// like "which cell site handled the most calls". Refusing a template on the
// record type alone breaks those, so the family guard must stand down whenever
// the question is genuinely cross-family. Precision over recall.
var semanticTargetScopedPattern = regexp.MustCompile(
	`\b(?:92|03)\d{7,13}\b|\{\s*(?:target|msisdn|number|identifier)\s*\}`)

func semanticQuestionNamesMultipleFamilies(question string) bool {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return false
	}
	stems := _semanticLayerQuestionTokens(question)
	seen := map[string]bool{}
	for _, entity := range layer.Entities {
		for _, phrase := range append([]string{entity.DisplayName}, entity.Synonyms...) {
			if _semanticLayerPhraseInQuestion(phrase, stems) {
				seen[entity.Family] = true
				break
			}
		}
	}
	// An identifier implies telephony even when no CDR word appears, which is what
	// makes "tower activity for <number>" a CDR question with a tower dimension.
	// A parameterised target ("… for {target}") is the same statement with the
	// value not yet substituted: a target-scoped question follows one identifier
	// across families, so it is cross-family by construction.
	if semanticTargetScopedPattern.MatchString(question) {
		seen["communications_cdr"] = true
	}
	return len(seen) > 1
}

// semanticQuestionNamesMultipleCuratedValues reports whether the question names
// more than one declared value of a single field, which makes it a breakdown.
func semanticQuestionNamesMultipleCuratedValues(question string) bool {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return false
	}
	entity, ok := layer.EntityByFamily(semanticQuestionFamily(question))
	if !ok {
		return false
	}
	return len(semanticLayerValueGrouping(question, entity)) > 0
}

// semanticLayerDeclaredLiterals returns the CANONICAL values the curated layer
// declares for the question's family, for each value or synonym the question
// actually names.
//
// The layer has carried these enumerations since WI-4 and nothing read them:
// `ValueLiterals` was written for exactly this and never called. The cost was
// measurable. "How many subscribers are active?" names ACTIVE, which
// `subscriber.status` declares with synonym "active", but literal extraction
// only recognised quoted strings and bare numbers. The generator's correct
// `subscriber.status = ACTIVE` filter was then rejected as a value the analyst
// never supplied -- SUB-02, and the same shape on every enumerated field.
//
// Scoped to ONE family: accepting another family's vocabulary would widen the
// CONSTRAINT_APPLIED allowlist into a place a wrong value could hide.
func semanticLayerDeclaredLiterals(question string) []string {
	family := semanticQuestionFamily(question)
	if family == "" {
		return nil
	}
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return nil
	}
	entity, ok := layer.EntityByFamily(family)
	if !ok {
		return nil
	}
	// Token set, so "active" does not match inside "inactive".
	//
	// STEMMED, using the SAME stemmer the semantic frame uses. Measured
	// 2026-09-25: `access_log.status` declares the value "500" with the synonym
	// "server error" and a description that says so outright, yet "How many
	// server errorS are in the access log?" bound nothing -- the question says
	// "errors" and the synonym says "error", and this matcher compared raw
	// words while the rest of the system stems. The answer was 1,000, the whole
	// family, where the truth is 46.
	//
	// Two vocabularies disagreeing about one sentence is the defect pattern
	// this codebase keeps paying for; the fix is always to share the one
	// vocabulary rather than to add a second rule.
	words := map[string]bool{}
	stems := map[string]bool{}
	for _, word := range strings.FieldsFunc(strings.ToLower(question), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		words[word] = true
		stems[semanticStem(word)] = true
	}
	// EXACT words for naming a value. Stemming here was measured and rejected
	// on 2026-09-25: "calls" stems to "call", which IS the value `CALL` of
	// cdr.call_type, so every ordinary question about calls -- CDR-05, CDR-06,
	// CDR-11, CDR-12, NEG-01 -- would have started supplying a call-type filter
	// the analyst never asked for. Five currently-correct questions, to fix one.
	named := func(phrase string) bool {
		parts := strings.Fields(strings.ToLower(strings.TrimSpace(phrase)))
		if len(parts) == 0 {
			return false
		}
		for _, part := range parts {
			if !words[part] {
				return false
			}
		}
		return true
	}
	// STEMMED matching for MULTI-WORD synonyms only. A two-word phrase is
	// specific enough that plural agreement is noise, not signal: the layer
	// declares "server error" and the analyst writes "server errors".
	namedStemmed := func(phrase string) bool {
		parts := strings.Fields(strings.ToLower(strings.TrimSpace(phrase)))
		if len(parts) == 0 {
			return false
		}
		for _, part := range parts {
			if !words[part] && !stems[semanticStem(part)] {
				return false
			}
		}
		return true
	}
	// A SINGLE word only supplies a value when it names that value. Loose
	// one-word synonyms must not: `cdr.direction` declares OUTGOING with the
	// synonym "made", so "Which phone number made the most calls?" would have
	// supplied OUTGOING and let a filter the analyst never asked for pass the
	// CONSTRAINT_APPLIED guard. A multi-word synonym ("text message") is
	// specific enough to count.
	supplies := func(value SemanticLayerValueV1) bool {
		if named(value.Value) {
			return true
		}
		for _, synonym := range value.Synonyms {
			if len(strings.Fields(strings.TrimSpace(synonym))) > 1 && namedStemmed(synonym) {
				return true
			}
		}
		return false
	}
	out := []string{}
	for _, field := range entity.Fields {
		for _, value := range field.Values {
			if supplies(value) {
				out = append(out, value.Value)
			}
		}
	}
	return out
}

// semanticLayerPhraseLiterals returns only the declared values a question named
// through a MULTI-WORD synonym.
//
// The distinction is the whole point. A single-word match is the value naming
// itself ("volte", "active") and is common but weak -- "how many CDR records
// are there?" names the value CALL simply by containing "call", and filtering
// on it would be wrong. A multi-word phrase ("server error" -> 500) is specific
// enough that the analyst meant it, so an unused one is a RESTRICTION THE PLAN
// DROPPED rather than noise.
//
// Measured 2026-09-25 across both corpora before use.
func semanticLayerPhraseLiterals(question string) []string {
	family := semanticQuestionFamily(question)
	if family == "" {
		return nil
	}
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return nil
	}
	entity, ok := layer.EntityByFamily(family)
	if !ok {
		return nil
	}
	words := map[string]bool{}
	stems := map[string]bool{}
	for _, word := range strings.FieldsFunc(strings.ToLower(question), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		words[word] = true
		stems[semanticStem(word)] = true
	}
	phraseNamed := func(phrase string) bool {
		parts := strings.Fields(strings.ToLower(strings.TrimSpace(phrase)))
		if len(parts) < 2 {
			return false
		}
		for _, part := range parts {
			if !words[part] && !stems[semanticStem(part)] {
				return false
			}
		}
		return true
	}
	out := []string{}
	for _, field := range entity.Fields {
		for _, value := range field.Values {
			for _, synonym := range value.Synonyms {
				if phraseNamed(synonym) {
					out = append(out, value.Value)
					break
				}
			}
		}
	}
	return out
}
