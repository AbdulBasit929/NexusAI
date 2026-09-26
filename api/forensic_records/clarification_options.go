package main

import (
	"fmt"
	"sort"
	"strings"
)

// CLARIFICATION OPTIONS.
//
// A clarification with no options is a dead end. The analyst is told the
// question cannot be answered and handed nothing to do about it, which reads as
// a failure even though abstaining was the correct behaviour. 18 of the 62
// golden questions now clarify -- 14 from verified-only and arbitration, 4 more
// from WI-10 -- so this is nearly a third of all traffic ending in a shrug.
//
// UX contract §7 requires 2-4 concrete options labelled with semantic-layer
// display names and re-runnable in one click.
//
// EVERY option is derived from the CURATED semantic layer, never invented:
//
//   - labels are `display_name` values from the layer, which are SQL-verified
//     against real source columns (83 source_names, WI-4);
//   - only `sensitivity: NONE` fields are offered, so an option can never
//     surface a field the sensitive-field guard withholds by design (PII, and
//     IDENTIFIER fields whose cardinality makes them useless as a grouping);
//   - only `groupable: true` fields are offered as breakdowns, because that
//     flag is the layer's own statement that grouping by it is meaningful.
//
// An option is a SUGGESTION, not a claim about evidence. If one of them also
// fails to resolve, the system clarifies again -- that is the product working,
// not a lie. What it must never do is name a field that does not exist, or one
// the analyst is not entitled to see.

// ClarificationOptionV1 is one re-runnable choice. `Label` is analyst language;
// `Query` is the exact question to send back, so the UI needs no phrasing logic
// of its own and cannot drift from what the compiler actually accepts.
type ClarificationOptionV1 struct {
	Label string `json:"label"`
	Query string `json:"query"`
}

// UX §7 asks for 2-4. Fewer than 2 is not a choice; more than 4 is a menu.
const (
	clarificationOptionLimit = 4
	clarificationOptionFloor = 2
)

// clarificationOptions builds the choices offered alongside a withheld answer.
// It returns nil rather than a single option: offering exactly one choice is
// not a clarification, it is a guess wearing a question mark.
func clarificationOptions(req hybridQueryRequest, missingFields []string) []ClarificationOptionV1 {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return nil
	}
	var options []ClarificationOptionV1
	entity, resolved := layer.EntityByFamily(semanticQuestionFamily(req.Query))
	// A question that NAMES a document, an image or a recording is not answered
	// by breaking down CDR rows, whatever family the ladder matched. DOC-04 is
	// the case: it resolved to `anpr` because it says "plate", and offering
	// "Break down by Camera" would push the analyst further toward exactly the
	// evidence WI-10 just established it should not be reading. When the
	// question names an artifact, the only useful choice is WHERE to look.
	if resolved && !questionNamesMediaArtifact(req.Query) {
		options = curatedEntityOptions(entity, missingFields)
	} else {
		// No family resolved -- the question never reached a curated entity, so
		// the useful choice is WHICH evidence to search. This is the case for
		// CASE-01, IMG-04 and X-01: all three sit in the audit's `no_family`
		// set, where no typed plan is possible until a scope is named.
		options = evidenceScopeOptions(layer, req)
	}
	// An option that re-asks the question just withheld is a loop, not a
	// choice. Measured live 2026-09-23: IMG-04 ("How many images are in this
	// case?") was offered "Images -> How many images are in this case?" --
	// clicking it returns the analyst to the same clarification.
	asked := normalizedQuestion(req.Query)
	kept := options[:0]
	for _, option := range options {
		if normalizedQuestion(option.Query) == asked {
			continue
		}
		kept = append(kept, option)
	}
	options = kept
	if len(options) < clarificationOptionFloor {
		return nil
	}
	if len(options) > clarificationOptionLimit {
		options = options[:clarificationOptionLimit]
	}
	return options
}

// normalizedQuestion compares two questions as an analyst would read them,
// ignoring case, surrounding space and a trailing question mark.
func normalizedQuestion(question string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(question)), "?")
}

// curatedEntityOptions offers ways to narrow a question the layer DOES
// recognise: the concrete values of a field the question failed to bind, then
// breakdowns by the fields the layer marks groupable.
func curatedEntityOptions(entity SemanticLayerEntityV1, missingFields []string) []ClarificationOptionV1 {
	noun := entityNoun(entity)
	var options []ClarificationOptionV1

	// A missing parameter the analyst cannot guess at is answerable when the
	// layer enumerates that field's values: "Voice call", "SMS". Offering the
	// real values is strictly better than asking them to phrase it again.
	missing := map[string]bool{}
	for _, name := range missingFields {
		missing[strings.ToLower(strings.TrimSpace(name))] = true
	}
	for _, field := range entity.Fields {
		if !fieldOfferable(field) || len(field.Values) == 0 || !fieldNamed(field, missing) {
			continue
		}
		for _, value := range field.Values {
			label := strings.TrimSpace(value.DisplayName)
			if label == "" {
				label = value.Value // truthful raw fallback, never an invention
			}
			options = append(options, ClarificationOptionV1{
				Label: label,
				Query: fmt.Sprintf("How many %s have %s %s?",
					noun, strings.ToLower(fieldLabel(field)), label),
			})
			if len(options) >= clarificationOptionLimit {
				return options
			}
		}
	}

	// Otherwise the honest offer is a breakdown: the question asked for a
	// measurement the compiler could not shape, and the layer knows exactly
	// which fields it is meaningful to group by.
	var groupable []SemanticLayerFieldV1
	for _, field := range entity.Fields {
		if fieldOfferable(field) && field.Groupable {
			groupable = append(groupable, field)
		}
	}
	sort.SliceStable(groupable, func(i, j int) bool {
		return fieldLabel(groupable[i]) < fieldLabel(groupable[j])
	})
	for _, field := range groupable {
		options = append(options, ClarificationOptionV1{
			Label: fmt.Sprintf("Break down by %s", fieldLabel(field)),
			// Mirrors CDR-16, which this shape answers correctly today.
			Query: fmt.Sprintf("How many %s are there for each %s?",
				noun, strings.ToLower(fieldLabel(field))),
		})
		if len(options) >= clarificationOptionLimit {
			break
		}
	}
	return options
}

// questionNamesMediaArtifact reuses the artifact nouns the WI-10 misroute guard
// is keyed on, so "what counts as a media question" has ONE definition.
func questionNamesMediaArtifact(question string) bool {
	lowered := strings.ToLower(question)
	for _, noun := range mediaArtifactNouns {
		if strings.Contains(lowered, noun) {
			return true
		}
	}
	return false
}

// mediaEvidenceScopes are the non-tabular evidence types. The curated semantic
// layer covers only the 7 STRUCTURED entities, so without these a question
// about a document or a recording could only ever be offered CDR counts --
// measured 2026-09-23, where X-01's answer lives in a PDF and a WAV and every
// offered option pointed at structured families instead.
//
// The labels are NOT invented: UX contract §5 specifies this exact chip
// vocabulary ("CDR · IPDR · ANPR · Subscriber · Tower · Financial · Access log
// · Documents · Images · Audio · Video"). The query shapes are the ones the
// corpus proves answer correctly today -- DOC-02/DOC-06, IMG-01, AUD-01/AUD-03.
// TargetQuery is the phrasing the CORPUS PROVES answers correctly for that
// evidence type, not a phrasing invented here. Measured live 2026-09-23: the
// generic "Which audio recordings mention 03001234567?" returned zero rows
// while AUD-03's "Is 03001234567 mentioned in any audio?" is scored CORRECT on
// the same identifier. An option is only worth offering if clicking it works.
var mediaEvidenceScopes = []struct {
	Label       string
	Noun        string
	TargetQuery string
	Keywords    []string
}{
	{Label: "Documents", Noun: "documents",
		TargetQuery: "Which document mentions %s?", // DOC-02
		Keywords:    []string{"document", "documents", "pdf", "notes", "guide", "email"}},
	{Label: "Images", Noun: "images",
		TargetQuery: "Find OCR text mentioning %s", // IMG-01
		Keywords:    []string{"image", "images", "photo", "photos", "picture", "ocr"}},
	{Label: "Audio", Noun: "audio recordings",
		TargetQuery: "Is %s mentioned in any audio?", // AUD-03
		Keywords:    []string{"audio", "recording", "recordings", "transcript", "transcripts"}},
	{Label: "Video", Noun: "videos",
		TargetQuery: "Which videos mention %s?", // no proven shape; generic
		Keywords:    []string{"video", "videos", "footage"}},
}

// evidenceScopeOptions offers WHERE to look. Used when no curated family
// resolved, or when the question names an artifact the structured families do
// not hold.
//
// Scopes whose own vocabulary appears in the question come first: asking about
// images and being offered "Call detail records" first is not a choice an
// analyst will click.
func evidenceScopeOptions(layer *SemanticLayerV1, req hybridQueryRequest) []ClarificationOptionV1 {
	target := strings.TrimSpace(req.Target)
	lowered := strings.ToLower(req.Query)

	type scope struct {
		label       string
		noun        string
		targetQuery string
		relevant    bool
	}
	var scopes []scope
	for _, media := range mediaEvidenceScopes {
		relevant := false
		for _, keyword := range media.Keywords {
			if strings.Contains(lowered, keyword) {
				relevant = true
				break
			}
		}
		// Without something to search FOR, a media scope can only be offered as
		// a count -- which is IMG-04's own failing question, so offering it
		// would loop the analyst straight back here.
		if target == "" && !relevant {
			continue
		}
		scopes = append(scopes, scope{label: media.Label, noun: media.Noun,
			targetQuery: media.TargetQuery, relevant: relevant})
	}
	entities := append([]SemanticLayerEntityV1(nil), layer.Entities...)
	sort.SliceStable(entities, func(i, j int) bool {
		return entityLabel(entities[i]) < entityLabel(entities[j])
	})
	for _, entity := range entities {
		if label := entityLabel(entity); label != "" {
			scopes = append(scopes, scope{label: label, noun: entityNoun(entity)})
		}
	}
	sort.SliceStable(scopes, func(i, j int) bool { return scopes[i].relevant && !scopes[j].relevant })

	var options []ClarificationOptionV1
	relevantLead := 0
	for _, s := range scopes {
		if !s.relevant {
			break
		}
		relevantLead++
	}
	for _, s := range scopes {
		option := ClarificationOptionV1{
			Label: s.label,
			Query: fmt.Sprintf("How many %s are in this case?", s.noun),
		}
		if target != "" {
			// The question names something specific; carrying it into the
			// option is what makes the choice worth clicking. This is what
			// turns X-01 from "no matching records" into four real searches.
			option.Label = fmt.Sprintf("%s mentioning %s", s.label, target)
			if s.targetQuery != "" {
				option.Query = fmt.Sprintf(s.targetQuery, target)
			} else {
				option.Query = fmt.Sprintf("Which %s mention %s?", s.noun, target)
			}
		}
		options = append(options, option)
		if len(options) >= clarificationOptionLimit {
			break
		}
	}
	if target == "" {
		// Asking what evidence a case HOLDS is answered by the source-file
		// audit, not by counting one family. `source_file_audit` is a real
		// template and this exact phrasing is already offered as a suggested
		// question elsewhere in the records path, so it is grounded, not
		// invented. It also gives IMG-04 somewhere to go once its own question
		// is removed as a self-reference.
		//
		// It sits AFTER the evidence type the question actually named: an
		// analyst asking about images wants images first, not a file listing.
		inventory := ClarificationOptionV1{Label: "Ingested files", Query: "Which files were ingested?"}
		at := relevantLead
		if at > len(options) {
			at = len(options)
		}
		options = append(options, ClarificationOptionV1{})
		copy(options[at+1:], options[at:])
		options[at] = inventory
	}
	return options
}

// fieldOfferable gates on the layer's OWN sensitivity declaration. PII is
// withheld by design and must never be suggested; IDENTIFIER fields are
// per-subject values whose cardinality makes them meaningless as a choice.
func fieldOfferable(field SemanticLayerFieldV1) bool {
	return strings.EqualFold(strings.TrimSpace(field.Sensitivity), "NONE")
}

func fieldNamed(field SemanticLayerFieldV1, names map[string]bool) bool {
	if len(names) == 0 {
		return false
	}
	if names[strings.ToLower(field.ID)] || names[strings.ToLower(field.DisplayName)] {
		return true
	}
	for _, source := range field.SourceNames {
		if names[strings.ToLower(source)] {
			return true
		}
	}
	return false
}

// fieldLabel and entityLabel prefer the curated display name and fall back to
// the raw identifier VISIBLY. A fabricated label in a forensic product is a
// truthfulness defect, not a cosmetic one.
func fieldLabel(field SemanticLayerFieldV1) string {
	if name := strings.TrimSpace(field.DisplayName); name != "" {
		return name
	}
	return field.ID
}

func entityLabel(entity SemanticLayerEntityV1) string {
	if name := strings.TrimSpace(entity.DisplayName); name != "" {
		return name
	}
	return entity.Family
}

// entityNoun renders what the analyst is counting, in their words.
func entityNoun(entity SemanticLayerEntityV1) string {
	label := strings.TrimSpace(entity.DisplayName)
	if label == "" {
		label = entity.RecordType
	}
	if label == "" {
		return "records"
	}
	lowered := strings.ToLower(label)
	if strings.HasSuffix(lowered, "records") || strings.HasSuffix(lowered, "s") {
		return lowered
	}
	return lowered + " records"
}
