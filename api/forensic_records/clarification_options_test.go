package main

import (
	"strings"
	"testing"
)

// Options are the difference between "I cannot answer that" and "I cannot
// answer that -- did you mean one of these?". 18 of 62 golden questions now
// clarify, so this governs nearly a third of all traffic.
//
// The invariants that matter are about TRUTHFULNESS, not presentation: an
// option may never name a field that does not exist, and may never surface one
// the sensitive-field guard withholds by design.

func layerOrSkip(t *testing.T) *SemanticLayerV1 {
	t.Helper()
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated semantic layer not loadable from this working directory")
	}
	return layer
}

// UX §7 asks for 2-4. One option is not a choice -- it is a guess with a
// question mark on it -- so the floor is enforced by returning nothing.
func TestClarificationOptionsRespectTheContractRange(t *testing.T) {
	layerOrSkip(t)
	for _, question := range []string{
		"How many records do we have for each record type?",
		"How many CDR records are there for each call type?",
		"Where does 03001234567 appear across all evidence?",
		"How many images are in this case?",
	} {
		options := clarificationOptions(hybridQueryRequest{Query: question}, nil)
		if n := len(options); n != 0 && (n < clarificationOptionFloor || n > clarificationOptionLimit) {
			t.Errorf("%q: %d options, want 0 or %d-%d", question, n, clarificationOptionFloor, clarificationOptionLimit)
		}
		for _, option := range options {
			if strings.TrimSpace(option.Label) == "" {
				t.Errorf("%q: an option with no label tells the analyst nothing", question)
			}
			if strings.TrimSpace(option.Query) == "" {
				t.Errorf("%q: option %q carries no query, so it cannot be clicked", question, option.Label)
			}
		}
	}
}

// The whole point of sourcing from the curated layer: every label is a
// display_name that was SQL-verified against a real source column.
func TestClarificationOptionsUseCuratedDisplayNames(t *testing.T) {
	layer := layerOrSkip(t)
	known := map[string]bool{}
	for _, entity := range layer.Entities {
		known[strings.ToLower(entityLabel(entity))] = true
		for _, field := range entity.Fields {
			known[strings.ToLower(fieldLabel(field))] = true
			for _, value := range field.Values {
				if value.DisplayName != "" {
					known[strings.ToLower(value.DisplayName)] = true
				}
			}
		}
	}
	options := clarificationOptions(hybridQueryRequest{Query: "How many CDR records are there for each call type?"}, nil)
	if len(options) == 0 {
		t.Skip("no options offered for this question; covered by the range test")
	}
	for _, option := range options {
		label := strings.ToLower(strings.TrimPrefix(option.Label, "Break down by "))
		if idx := strings.Index(label, " mentioning "); idx >= 0 {
			label = label[:idx]
		}
		if !known[label] {
			t.Errorf("option label %q is not a curated display name -- it was invented", option.Label)
		}
	}
}

// A PII or IDENTIFIER field must never be offered. The sensitive-field guard
// withholds those by design, so suggesting one would advertise a capability the
// product deliberately refuses -- and SUB-03's gold already demands fields the
// guard correctly withholds.
func TestClarificationOptionsNeverOfferSensitiveFields(t *testing.T) {
	layer := layerOrSkip(t)
	forbidden := map[string]bool{}
	for _, entity := range layer.Entities {
		for _, field := range entity.Fields {
			if !fieldOfferable(field) {
				forbidden[strings.ToLower(fieldLabel(field))] = true
			}
		}
	}
	if len(forbidden) == 0 {
		t.Fatal("expected the curated layer to declare sensitive fields (PII/IDENTIFIER)")
	}
	for _, question := range []string{
		"How many subscribers are there for each field?",
		"How many CDR records are there?",
		"Where does 03001234567 appear across all evidence?",
	} {
		for _, option := range clarificationOptions(hybridQueryRequest{Query: question}, nil) {
			label := strings.ToLower(strings.TrimPrefix(option.Label, "Break down by "))
			if forbidden[label] {
				t.Errorf("%q offered sensitive field %q", question, option.Label)
			}
		}
	}
}

// Only fields the layer marks groupable are offered as breakdowns -- that flag
// is the layer's own statement that grouping by the field is meaningful.
func TestClarificationOptionsOnlyBreakDownByGroupableFields(t *testing.T) {
	layer := layerOrSkip(t)
	groupable := map[string]bool{}
	for _, entity := range layer.Entities {
		for _, field := range entity.Fields {
			if field.Groupable && fieldOfferable(field) {
				groupable[strings.ToLower(fieldLabel(field))] = true
			}
		}
	}
	for _, option := range clarificationOptions(hybridQueryRequest{Query: "How many CDR records are there for each call type?"}, nil) {
		if !strings.HasPrefix(option.Label, "Break down by ") {
			continue
		}
		field := strings.ToLower(strings.TrimPrefix(option.Label, "Break down by "))
		if !groupable[field] {
			t.Errorf("offered a breakdown by %q, which the layer does not mark groupable", field)
		}
	}
}

// A question that reached no family (CASE-01, IMG-04, X-01 are all in the
// audit's no_family set) can only be rescued by naming the evidence to search,
// so the options become scope choices.
func TestClarificationOptionsOfferScopeWhenNoFamilyResolves(t *testing.T) {
	layerOrSkip(t)
	options := clarificationOptions(hybridQueryRequest{Query: "How many images are in this case?"}, nil)
	if len(options) < clarificationOptionFloor {
		t.Fatalf("a no-family question must still offer a scope choice; got %d", len(options))
	}
	for _, option := range options {
		if strings.HasPrefix(option.Label, "Break down by ") {
			t.Errorf("no family resolved, so %q is not a choice the analyst can act on", option.Label)
		}
	}
}

// When the question names something specific, carrying it into the option is
// what makes the choice worth clicking.
func TestClarificationOptionsCarryTheTargetIntoScopeChoices(t *testing.T) {
	layerOrSkip(t)
	req := hybridQueryRequest{
		Query:  "Where does 03001234567 appear across all evidence?",
		Target: "03001234567",
	}
	options := clarificationOptions(req, nil)
	if len(options) < clarificationOptionFloor {
		t.Fatalf("X-01 must offer somewhere to look; got %d", len(options))
	}
	for _, option := range options {
		if !strings.Contains(option.Query, "03001234567") {
			t.Errorf("option %q dropped the identifier the analyst asked about", option.Label)
		}
	}
}

// Deterministic output: the same question must not reshuffle its choices
// between calls, or the UI appears to change its mind.
func TestClarificationOptionsAreDeterministic(t *testing.T) {
	layerOrSkip(t)
	req := hybridQueryRequest{Query: "How many CDR records are there for each call type?"}
	first := clarificationOptions(req, nil)
	for i := 0; i < 3; i++ {
		again := clarificationOptions(req, nil)
		if len(again) != len(first) {
			t.Fatalf("option count changed between calls: %d then %d", len(first), len(again))
		}
		for j := range first {
			if first[j] != again[j] {
				t.Fatalf("option %d changed between calls: %+v then %+v", j, first[j], again[j])
			}
		}
	}
}

// DOC-04 resolved to the ANPR family because it says "plate", but it asks what
// a DOCUMENT says. Offering "Break down by Camera" would push the analyst
// further toward the evidence WI-10 established it should not be reading.
func TestClarificationOptionsNeverBreakDownAMediaQuestion(t *testing.T) {
	layerOrSkip(t)
	req := hybridQueryRequest{
		Query:  "What does the case notes document say about plate ABC-123?",
		Target: "ABC-123",
	}
	options := clarificationOptions(req, nil)
	if len(options) < clarificationOptionFloor {
		t.Fatalf("a media question must still be offered somewhere to look; got %d", len(options))
	}
	for _, option := range options {
		if strings.HasPrefix(option.Label, "Break down by ") {
			t.Errorf("offered %q for a question about a document", option.Label)
		}
	}
}

// Asking about images and being offered "Call detail records" first is not a
// choice an analyst will click.
func TestClarificationOptionsRankTheNamedEvidenceTypeFirst(t *testing.T) {
	layerOrSkip(t)
	for _, tc := range []struct{ question, want string }{
		{"Which recording mentions coconut sugar and at what time?", "Audio"},
		{"What does the case notes document say about plate ABC-123?", "Documents"},
		// IMG-04 is the interaction between the two rules: "Images" IS the
		// named evidence type, but its option would re-ask the question
		// verbatim, so the self-reference guard removes it and the inventory
		// option -- which genuinely answers "what evidence is in this case" --
		// leads instead.
		{"How many images are in this case?", "Ingested files"},
	} {
		options := clarificationOptions(hybridQueryRequest{Query: tc.question}, nil)
		if len(options) == 0 {
			t.Errorf("%q offered nothing", tc.question)
			continue
		}
		if !strings.HasPrefix(options[0].Label, tc.want) {
			t.Errorf("%q: first option %q, want it to lead with %q",
				tc.question, options[0].Label, tc.want)
		}
	}
}

// X-01's answer lives in a PDF and a WAV. Before this, every option pointed at
// a structured family, so the choices led away from the answer.
func TestClarificationOptionsOfferMediaScopesForACrossEvidenceSearch(t *testing.T) {
	layerOrSkip(t)
	req := hybridQueryRequest{
		Query:  "Where does 03001234567 appear across all evidence?",
		Target: "03001234567",
	}
	options := clarificationOptions(req, nil)
	var media int
	for _, option := range options {
		for _, scope := range mediaEvidenceScopes {
			if strings.HasPrefix(option.Label, scope.Label) {
				media++
			}
		}
	}
	if media == 0 {
		t.Fatalf("a cross-evidence search must be able to offer document/image/audio scopes; got %+v", options)
	}
}

// An option that re-asks the question just withheld is a loop, not a choice.
// Measured live: IMG-04 was offered "Images -> How many images are in this
// case?", which is the question it had just declined to answer.
func TestClarificationOptionsNeverEchoTheQuestionBack(t *testing.T) {
	layerOrSkip(t)
	for _, question := range []string{
		"How many images are in this case?",
		"How many records do we have for each record type?",
		"Where does 03001234567 appear across all evidence?",
	} {
		for _, option := range clarificationOptions(hybridQueryRequest{Query: question}, nil) {
			if normalizedQuestion(option.Query) == normalizedQuestion(question) {
				t.Errorf("%q was offered itself back as %q", question, option.Label)
			}
		}
	}
}
