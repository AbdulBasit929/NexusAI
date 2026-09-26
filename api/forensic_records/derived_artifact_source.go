package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// WHERE AN ENTITY'S ROWS LIVE.
//
// Until now every curated entity bound to `forensic.records` through
// `record_type`, and the executor hard-coded that table in five places. So no
// typed plan could reference a single OCR line, plate read, face detection or
// transcript segment, and every media question that worked did so because
// somebody had written a template for it. That is the dependency the product is
// removing.
//
// The evidence those questions need is already structured. On
// `nexusai-multimodal-product-acceptance`, `forensic.derived_artifacts` holds
// 865 rows across 11 contracts, with normalized text, numeric confidences,
// review states and time offsets -- typed columns, not prose.
//
// An entity therefore declares its source:
//
//	source:
//	  table: derived_artifacts          # omitted means records, so every
//	  artifact_type: forensics.anpr-observation/v1
//	  payload: metadata                 # existing entity is unchanged
//
// OMISSION IS THE DEFAULT AND MEANS "records". That is deliberate: the seven
// structured entities carry no source block and must keep behaving exactly as
// they do, because changing the issued field set changes plans for questions
// that already work -- proven twice, and it cost a false negative both times.
const (
	semanticSourceTableRecords = "records"
	semanticSourceTableDerived = "derived_artifacts"
)

// semanticSourcePayloadColumns are the jsonb columns an entity may read its
// fields from, per table. An allowlist rather than a free string: the payload
// name reaches SQL, and a column named by a YAML file is a column named by
// whoever can write one.
var semanticSourcePayloadColumns = map[string]map[string]bool{
	semanticSourceTableRecords: {"raw_payload": true},
	semanticSourceTableDerived: {"metadata": true},
}

// semanticSourceTables maps the declared table name to its real, qualified
// name. Nothing else may reach SQL.
var semanticSourceTables = map[string]string{
	semanticSourceTableRecords: "forensic.records",
	semanticSourceTableDerived: "forensic.derived_artifacts",
}

type SemanticLayerSourceV1 struct {
	Table        string `yaml:"table" json:"table,omitempty"`
	ArtifactType string `yaml:"artifact_type" json:"artifact_type,omitempty"`
	Payload      string `yaml:"payload" json:"payload,omitempty"`
	// ObservationType narrows a contract that carries MORE THAN ONE KIND OF ROW.
	//
	// Measured 2026-09-26: `forensics.image-observation/v1` holds 20
	// `image_technical_observation` rows and 6 `video_sampled_frame_observation`
	// rows, and their FIELDS DIFFER -- only the sampled frames carry a frame
	// number and sampling interval. One entity over both would describe fields
	// that two thirds of its rows do not have, and every aggregate would silently
	// mix two populations.
	//
	// Three other contracts carry two kinds whose fields are IDENTICAL -- a
	// standalone audio segment and the same segment extracted from a video's
	// audio track. Those do not need splitting, but an analyst counting "audio
	// segments" is entitled to know both are in the total, so the discriminator
	// is available to express either choice.
	//
	// Matched EXACTLY against metadata->>'observation_type', as a bound
	// parameter. Never a pattern: a prefix match would silently pull
	// `video_embedded_audio_transcript_segment` into `audio_transcript_segment`.
	ObservationType string `yaml:"observation_type" json:"observation_type,omitempty"`
}

// resolvedTable is the declared table with its default applied.
func (s SemanticLayerSourceV1) resolvedTable() string {
	if table := strings.TrimSpace(s.Table); table != "" {
		return table
	}
	return semanticSourceTableRecords
}

// resolvedPayload is the declared payload column with its per-table default.
func (s SemanticLayerSourceV1) resolvedPayload() string {
	if payload := strings.TrimSpace(s.Payload); payload != "" {
		return payload
	}
	if s.resolvedTable() == semanticSourceTableDerived {
		return "metadata"
	}
	return "raw_payload"
}

// IsDerived reports an entity whose rows are MODEL OBSERVATIONS rather than
// ingested source records. Callers use it to keep the two apart: a model's
// plate read is not a camera's sighting, and presenting one as the other is a
// fabricated sighting.
func (s SemanticLayerSourceV1) IsDerived() bool {
	return s.resolvedTable() == semanticSourceTableDerived
}

// validateSemanticSource checks a declared source block.
//
// A derived entity MUST name its artifact_type. Without one the entity would
// match every row in the table regardless of contract -- an OCR entity counting
// face detections -- and the count would look entirely authoritative.
func validateSemanticSource(where string, source SemanticLayerSourceV1) error {
	table := source.resolvedTable()
	if _, ok := semanticSourceTables[table]; !ok {
		known := []string{semanticSourceTableRecords, semanticSourceTableDerived}
		return fmt.Errorf("%s: source.table %q is not one of %s", where, table, strings.Join(known, ", "))
	}
	payload := source.resolvedPayload()
	if !semanticSourcePayloadColumns[table][payload] {
		return fmt.Errorf("%s: source.payload %q is not a readable column of %s", where, payload, table)
	}
	artifactType := strings.TrimSpace(source.ArtifactType)
	switch table {
	case semanticSourceTableDerived:
		if artifactType == "" {
			return fmt.Errorf("%s: source.artifact_type is required for %s — without one the entity matches every artifact contract in the table", where, table)
		}
		// The contracts carry version suffixes. A typo yields an entity that
		// silently matches ZERO rows, which reads as "there is no such
		// evidence" -- the confident-empty-answer failure.
		if !strings.HasPrefix(artifactType, "forensics.") || !strings.Contains(artifactType, "/v") {
			return fmt.Errorf("%s: source.artifact_type %q is not a versioned forensics contract (expected forensics.<name>/v<N>)", where, artifactType)
		}
	default:
		if artifactType != "" {
			return fmt.Errorf("%s: source.artifact_type is meaningless for table %s", where, table)
		}
		if strings.TrimSpace(source.ObservationType) != "" {
			return fmt.Errorf("%s: source.observation_type is meaningless for table %s", where, table)
		}
	}
	// A discriminator that is declared but blank would match rows whose
	// observation_type is the empty string, which is not what anyone writing it
	// meant. Omit it to match the whole contract; set it to narrow.
	if source.ObservationType != "" && strings.TrimSpace(source.ObservationType) == "" {
		return fmt.Errorf("%s: source.observation_type is blank; omit it to match the whole contract", where)
	}
	return nil
}

// sourceNativeBinding is what the SQL executor needs to know about WHERE a
// plan's rows live. It is resolved ONCE per plan, from the curated layer, and
// passed explicitly rather than carried on FieldDescriptorV1 -- that struct is
// serialized into the model payload, and adding to it changes the issued field
// set, which changes plans for questions that already work.
type sourceNativeBinding struct {
	Table           string
	Payload         string
	ArtifactType    string
	ObservationType string
	Derived         bool
}

// sourceNativeRecordsBinding is the default: ingested source records, exactly
// as every plan read them before derived artifacts existed.
func sourceNativeRecordsBinding() sourceNativeBinding {
	return sourceNativeBinding{
		Table:   semanticSourceTables[semanticSourceTableRecords],
		Payload: "raw_payload",
	}
}

// sourceNativeBindingForFieldIDs resolves the binding for every field a plan
// references, and REFUSES A PLAN THAT MIXES SOURCES.
//
// Refusing is the safety property. A plan naming both a CDR column and an OCR
// observation cannot be answered by one table, and the alternatives are both
// unacceptable: silently reading one table would answer about evidence the
// analyst did not ask about, and silently joining them would assert a
// relationship between an ingested record and a model's guess that nothing in
// the data establishes. Neither may be presented as an answer.
//
// A field the layer does not own resolves to the records default, because that
// is what an INFERRED (uncurated) catalogue field is: a key in
// forensic.records.raw_payload.
// sourceNativeBindingForPlan resolves the binding for a plan, falling back to
// the ISSUED CATALOGUE when the plan names no field at all.
//
// THE FALLBACK IS THE WHOLE POINT, and it was found by measurement. A plain
// `COUNT(*)` carries NO field ID -- and "how many plate groups were read from
// the videos?" is exactly that plan. Resolving from the plan's fields alone left
// it with nothing to resolve, so it defaulted to forensic.records and answered
// **9,580** (every structured row in the collection) where the truth is **24**.
// A count that large over forensic evidence reads exactly like a right one.
//
// The catalogue is the correct fallback because it is what BOUNDED the plan: the
// generator could only reference fields it was issued, so the issued set names
// the evidence the question was compiled against.
func sourceNativeBindingForPlan(question string, plan *SourceNativePlanV1, catalog []FieldDescriptorV1) (sourceNativeBinding, error) {
	// 1. THE PLAN'S OWN FIELDS are authoritative: they name the evidence being read.
	if ids := sourceNativePlanFieldIDs(plan); len(ids) > 0 {
		return sourceNativeBindingForFieldIDs(ids)
	}

	// A plain COUNT(*) names NO field, and "how many plate groups were read from
	// the videos?" is exactly that plan. Resolving from the plan alone left
	// nothing to resolve, so it defaulted to forensic.records and answered
	// **9,580** -- every structured row in the collection -- where the truth is
	// **24**. A count that large over forensic evidence reads like a right one.

	// 2. THE ISSUED CATALOGUE, because it is what BOUNDED the plan: the generator
	//    could only reference fields it was issued, so the issued set names the
	//    evidence the question was compiled against.
	ids := make([]string, 0, len(catalog))
	for _, field := range catalog {
		ids = append(ids, field.FieldID)
	}
	if len(ids) > 0 {
		if binding, err := sourceNativeBindingForFieldIDs(ids); err == nil {
			return binding, nil
		}
	}

	// 3. THE QUESTION'S FAMILY, only when the catalogue is MIXED.
	//
	// Measured 2026-09-26: "how many faces were detected in the evidence?" is a
	// COUNT(*), and the issued catalogue legitimately mixes CURATED media fields
	// with INFERRED forensic.records fields -- so requiring the catalogue to agree
	// refused a good question and surfaced as an HTTP 500. The Phase 1 gate is
	// ZERO HTTP 500s.
	//
	// This is a TIEBREAKER and not the first test, deliberately. The question's
	// family comes from the STRUCTURED record-type extractor, so "across the video
	// frames" resolves a structured family and would override a media catalogue --
	// which it did, on the first attempt at this fix.
	if layer, _ := defaultSemanticLayer(); layer != nil {
		if entity, ok := layer.EntityByFamily(semanticQuestionFamily(question)); ok {
			return sourceNativeBinding{
				Table:           semanticSourceTables[entity.Source.resolvedTable()],
				Payload:         entity.Source.resolvedPayload(),
				ArtifactType:    strings.TrimSpace(entity.Source.ArtifactType),
				ObservationType: strings.TrimSpace(entity.Source.ObservationType),
				Derived:         entity.Source.IsDerived(),
			}, nil
		}
	}

	// 4. Nothing establishes the source. Records is what an UNCURATED catalogue
	//    field is, and a question this ambiguous is declined downstream by
	//    verification. Erroring here would be an HTTP 500 for a question the
	//    product should simply decline.
	return sourceNativeRecordsBinding(), nil
}

func sourceNativeBindingForFieldIDs(ids []string) (sourceNativeBinding, error) {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		// No curated layer means no derived entity can exist, so every field is
		// an inferred key in forensic.records.
		return sourceNativeRecordsBinding(), nil
	}
	binding := sourceNativeRecordsBinding()
	owner, found := "", false
	for _, id := range ids {
		entity, ok := layer.EntityByFieldID(id)
		if !ok {
			// An inferred field: forensic.records by definition.
			entity = SemanticLayerEntityV1{Family: semanticSourceTableRecords}
		}
		candidate := sourceNativeBinding{
			Table:           semanticSourceTables[entity.Source.resolvedTable()],
			Payload:         entity.Source.resolvedPayload(),
			ArtifactType:    strings.TrimSpace(entity.Source.ArtifactType),
			ObservationType: strings.TrimSpace(entity.Source.ObservationType),
			Derived:         entity.Source.IsDerived(),
		}
		if !found {
			binding, owner, found = candidate, entity.Family, true
			continue
		}
		if candidate != binding {
			return sourceNativeBinding{}, fmt.Errorf(
				"plan mixes evidence sources: %s reads %s and %s reads %s — "+
					"an ingested record and a model observation cannot be counted together",
				owner, binding.Table, entity.Family, candidate.Table)
		}
	}
	return binding, nil
}

// FORENSIC_DERIVED_ARTIFACT_EXECUTION gates reading derived artifacts at all.
// Default OFF: with it off a plan over a derived entity is refused, so the
// records path -- which serves all 62 golden questions -- is untouched.
const derivedArtifactExecutionEnv = "FORENSIC_DERIVED_ARTIFACT_EXECUTION"

func derivedArtifactExecutionEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(derivedArtifactExecutionEnv)), "true")
}

// sourceNativeDerivedScopeWhere is the authorized scope for derived artifacts.
//
// It is SEPARATE from sourceNativeScopeWhere rather than a parameterisation of
// it, because that function constrains on `r.record_type`, `r.source_file` and
// `r.batch_id` -- and forensic.derived_artifacts HAS NONE OF THEM. Reusing it
// would not produce a wrong answer; it would fail to compile the statement.
//
// Tenant and collection are non-negotiable and come first, exactly as they do
// for records: an unscoped read of evidence is not a bug, it is a breach.
func sourceNativeDerivedScopeWhere(req hybridQueryRequest, args []any) ([]string, []any, error) {
	if req.TenantID == "" || req.CollectionID == "" {
		return nil, nil, errors.New("source-native execution requires an authorized database scope")
	}
	args = append(args, req.TenantID, req.CollectionID)
	where := []string{
		fmt.Sprintf("d.tenant_id=$%d", len(args)-1),
		fmt.Sprintf("d.collection_id=$%d", len(args)),
	}
	// Only COMPLETED artifacts are evidence. A row still processing, or one that
	// failed, is not an observation the analyst may count -- and counting it
	// would inflate every total with work that never finished.
	args = append(args, "completed")
	where = append(where, fmt.Sprintf("d.processing_status=$%d", len(args)))
	return where, args, nil
}

// sourceNativePlanFieldIDs lists every field a plan references.
//
// Sort targets are included only when they name a field: a sort may target a
// measure alias instead, and a measure alias is not a field ID.
func sourceNativePlanFieldIDs(plan *SourceNativePlanV1) []string {
	if plan == nil {
		return nil
	}
	ids := make([]string, 0, len(plan.Project)+len(plan.GroupFields)+len(plan.Filters)+len(plan.Measures))
	ids = append(ids, plan.Project...)
	ids = append(ids, plan.GroupFields...)
	for _, filter := range plan.Filters {
		ids = append(ids, filter.FieldID)
	}
	for _, measure := range plan.Measures {
		if strings.TrimSpace(measure.FieldID) != "" {
			ids = append(ids, measure.FieldID)
		}
	}
	if plan.TimeBucket != nil {
		ids = append(ids, plan.TimeBucket.FieldID)
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// sourceNativePayloadTextExpr builds the SQL that extracts one source name as
// text from the bound payload column.
//
// Every path segment is a BOUND PARAMETER, never interpolated, so a key
// containing quotes or punctuation cannot alter the statement. Intermediate
// segments use `->` (jsonb) and the last uses `->>` (text), which is what makes
// "observation.start_seconds" resolve to
// metadata->'observation'->>'start_seconds'.
func sourceNativePayloadTextExpr(binding sourceNativeBinding, alias, sourceName string, args *[]any) string {
	path := semanticSourceFieldPath(sourceName, binding.Derived)
	if len(path) == 0 {
		return "NULL"
	}
	expr := alias + "." + binding.Payload
	for i, segment := range path {
		*args = append(*args, segment)
		placeholder := fmt.Sprintf("$%d", len(*args))
		if i == len(path)-1 {
			expr += "->>" + placeholder
		} else {
			expr += "->" + placeholder
		}
	}
	return expr
}

// semanticSourceFieldPath resolves a source name to its jsonb path.
//
// A RECORDS field names ONE LITERAL KEY and is never split. This is not a
// nicety: `anpr.plate_number` binds to the CSV header "Registration No.",
// which ENDS IN A PERIOD. Treating a dot as a path separator there turned a
// working field into an empty path and broke the shipped layer -- caught by
// TestWI4 the first time this file ran.
//
// A DERIVED field may name a nested path, because an observation keeps its
// analytical fields under one object: "observation.normalized_plate_text".
// Splitting is the declared convention for those contracts, and only those.
//
// Segments are used as BOUND PARAMETERS, never interpolated, so a key
// containing punctuation cannot alter the statement.
func semanticSourceFieldPath(sourceName string, derived bool) []string {
	trimmed := strings.TrimSpace(sourceName)
	if trimmed == "" {
		return nil
	}
	if !derived {
		return []string{trimmed}
	}
	parts := strings.Split(trimmed, ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil
		}
		out = append(out, part)
	}
	return out
}

// validateSemanticSourceFieldName rejects a source name that cannot be resolved
// to a jsonb path.
func validateSemanticSourceFieldName(where, sourceName string, derived bool) error {
	path := semanticSourceFieldPath(sourceName, derived)
	if len(path) == 0 {
		return fmt.Errorf("%s: source name %q is empty or has an empty path segment", where, sourceName)
	}
	if len(path) > 4 {
		return fmt.Errorf("%s: source name %q nests deeper than the payload contracts do", where, sourceName)
	}
	return nil
}
