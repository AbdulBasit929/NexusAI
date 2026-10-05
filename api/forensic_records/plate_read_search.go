package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// U2b. SEARCH-ONLY PLATE LOOKUP OVER THE PLATE READER'S OUTPUT.
//
// "Which image shows plate MN1367?" could not be answered: the plate text a
// model read from an image or a video frame is curated `sensitivity: PII`, and
// the query engine is never issued a PII field (CatalogFields), by design.
//
// PRODUCT-OWNER DECISION, 2026-09-28: SEARCH-ONLY. An analyst who supplies a
// plate may use it as an exact filter over the model's plate reads, to learn
// which images or video frames it was read in. The plate text is never
// projected, grouped, sorted or aggregated, so a question that names no plate
// ("Which plates were read from the videos?") stays refused exactly as today.
// This is the rule audio already follows: a supplied number can be found.
//
// What an answer can disclose: the count of matching reads, the files they
// were read in (from the provenance citation locator) and that they are
// unreviewed model observations. Provenance carries no plate text. The only
// plate in the answer is the one the analyst typed.
//
// See reports/plate-read-search-20260928/.
const plateSearchOnlyEnv = "FORENSIC_PLATE_SEARCH_ONLY"

const (
	plateReadFamily    = "anpr_model_observation"
	plateReadTextField = "anpr_model_observation.plate_text"
)

func plateSearchOnlyEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(plateSearchOnlyEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

var (
	plateSearchSubject = regexp.MustCompile(`(?i)\b(?:plates?|number\s+plates?|registrations?|licen[cs]e\s+plates?|vehicles?|cars?)\b`)
	plateSearchMedia   = regexp.MustCompile(`(?i)\b(?:images?|photos?|pictures?|videos?|frames?|footage)\b`)
	// A question asking for image TEXT belongs to the image-text search (U2a),
	// which reads plates in fragments ("BCX-567") and answers them today.
	plateSearchTextAsk = regexp.MustCompile(`(?i)\b(?:ocr|text|phrase|written|wording)\b`)
	plateKeyStrip      = regexp.MustCompile(`[^A-Z0-9]`)
)

// analystPlateLiterals returns the plate keys the analyst typed, normalised the
// way the plate reader stores them: upper case, letters and digits only
// ("BCX-567" -> "BCX567"). A key must mix letters and digits, so a phone number
// is never mistaken for a plate.
func analystPlateLiterals(query string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, target := range extractTargets(query) {
		// A file name ("DSC_1105.JPG") mixes letters and digits too; plates never
		// carry a dot or an underscore.
		if strings.ContainsAny(target, "._") {
			continue
		}
		key := plateKeyStrip.ReplaceAllString(strings.ToUpper(target), "")
		if len(key) < 4 || len(key) > 10 || !strings.ContainsAny(key, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") ||
			!strings.ContainsAny(key, "0123456789") || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

// plateReadSearchApplies: the question names images or video, is about a plate,
// supplies at least one plate, and does not ask for image text.
func plateReadSearchApplies(query string) bool {
	if !plateSearchOnlyEnabled() {
		return false
	}
	if !plateSearchSubject.MatchString(query) || !plateSearchMedia.MatchString(query) || plateSearchTextAsk.MatchString(query) {
		return false
	}
	literals := analystPlateLiterals(query)
	return len(literals) > 0 && len(literals) <= 20
}

// plateReadFilterOnlyField reports the one PII field admitted to a catalogue,
// filter-only, and only for a question that supplies a plate.
func plateReadFilterOnlyField(req hybridQueryRequest, entity SemanticLayerEntityV1, field SemanticLayerFieldV1) bool {
	return entity.Family == plateReadFamily && field.ID == plateReadTextField && plateReadSearchApplies(req.Query)
}

// filterOnlyDescriptor issues a PII field that may appear in a filter and
// nowhere else. validateSourceNativePlan enforces Projectable, Groupable,
// Sortable and AllowedAggregates, so a plan that tried to list, group, sort or
// count the plates is rejected before it runs.
func filterOnlyDescriptor(entity SemanticLayerEntityV1, field SemanticLayerFieldV1, req hybridQueryRequest) FieldDescriptorV1 {
	names := append([]string(nil), field.SourceNames...)
	filters := []string{}
	for _, op := range field.AllowedFilters {
		if op == "EQ" || op == "IN" {
			filters = append(filters, op)
		}
	}
	return FieldDescriptorV1{
		ContractVersion: fieldDescriptorContractV1, FieldID: field.ID, DisplayName: field.DisplayName,
		Description: field.Description, Synonyms: append([]string(nil), field.Synonyms...), Curated: true,
		SourceName: names[0], SourceNames: names, NormalizedName: normalizeSourceNativeName(names[0]),
		EffectiveType: semanticLayerEffectiveType(field.Type), Sensitivity: field.Sensitivity,
		RedactionState: "FILTER_ONLY", AllowedFilters: filters,
		Projectable: false, Groupable: false, Sortable: false, AllowedAggregates: nil,
		FamilyProvenance: []string{entity.RecordType},
		EvidenceID:       req.EvidenceID, EvidenceVersionID: req.EvidenceVersionID,
	}
}

// plateReadSearchRequest turns a qualifying question into a verified typed
// plan: COUNT of plate reads whose plate text equals (or is in) the supplied
// keys. It is validated against the issued catalogue before it is adopted.
func plateReadSearchRequest(req hybridQueryRequest) (hybridQueryRequest, bool) {
	if !plateReadSearchApplies(req.Query) {
		return req, false
	}
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return req, false
	}
	entity, ok := layer.EntityByFamily(plateReadFamily)
	if !ok {
		return req, false
	}
	catalog := entity.CatalogFields(req)
	plates := analystPlateLiterals(req.Query)
	filter := SourceNativeFilterV1{FieldID: plateReadTextField, Op: "EQ", Value: plates[0], Values: []string{plates[0]}}
	if len(plates) > 1 {
		filter = SourceNativeFilterV1{FieldID: plateReadTextField, Op: "IN", Values: plates}
	}
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Filters:         []SourceNativeFilterV1{filter},
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
	}
	if err := validateSourceNativePlan(plan, catalog); err != nil {
		return req, false
	}
	bound := req
	bound.Template = "canonical_records"
	bound.SourceNative = plan
	bound.SourceNativeCatalog = catalog
	bound.Group, bound.Compare, bound.Projection = nil, nil, nil
	bound.Target, bound.Targets = plates[0], plates
	return bound, true
}

// plateReadSearchPlan reports a plan built by plateReadSearchRequest.
func plateReadSearchPlan(plan *SourceNativePlanV1) bool {
	if !plateSearchOnlyEnabled() || plan == nil {
		return false
	}
	for _, filter := range plan.Filters {
		if filter.FieldID == plateReadTextField {
			return true
		}
	}
	return false
}

// plateReadSearchNote completes the count sentence: the files the reads came
// from, and what a model read is and is not. With no match it says the reader
// may have misread the plate, because an empty exact search over model reads
// does not show the plate is absent from the images.
func plateReadSearchNote(resp hybridQueryResponse, plan *SourceNativePlanV1, count float64) string {
	if !plateReadSearchPlan(plan) {
		return ""
	}
	if count == 0 {
		return "The plate reader may have misread it, so this does not show the plate is absent from the images or video."
	}
	files := map[string]bool{}
	entries := mapsFromAny(resp.Records["provenance"])
	for _, entry := range entries {
		locator, _ := entry["citation_locator"].(map[string]any)
		if name := strings.TrimSpace(stringValueAny(locator["source_file"])); name != "" {
			files[name] = true
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	listed := ""
	switch {
	case len(names) == 0:
	case len(entries) >= int(count):
		listed = fmt.Sprintf("Read in: %s. ", strings.Join(names, ", "))
	default:
		shown := names
		if len(shown) > 3 {
			shown = shown[:3]
		}
		listed = fmt.Sprintf("Read in files including: %s. ", strings.Join(shown, ", "))
	}
	return listed + "These are unreviewed model reads, not camera sightings."
}

// plateReadNounOr names what a plate-read search counted. With zero matches
// there is no provenance to take the label from, and the fallback record-type
// noun would say "no ANPR sightings" -- camera records that were never
// searched. Measured 2026-09-28 on the U2b arm and fixed before shipping.
func plateReadNounOr(plan *SourceNativePlanV1, fallback string, n float64) string {
	if !plateReadSearchPlan(plan) {
		return fallback
	}
	nouns := derivedArtifactNouns["forensics.anpr-observation/v1"]
	if n == 1 {
		return nouns[0]
	}
	return nouns[1]
}

// withPlateReadNote appends plateReadSearchNote to a count sentence.
func withPlateReadNote(headline string, resp hybridQueryResponse, plan *SourceNativePlanV1, count float64) string {
	if note := plateReadSearchNote(resp, plan, count); note != "" {
		return headline + " " + note
	}
	return headline
}
