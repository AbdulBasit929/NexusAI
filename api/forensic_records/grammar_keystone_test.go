package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/mudler/LocalAI/pkg/functions"
)

// THE KEYSTONE, ASSERTED ON THE PATH THE PRODUCT ACTUALLY USES.
//
// The architecture's central safety claim is that a field outside the issued
// set is structurally impossible to emit: the enum compiles to a GBNF
// alternation and the decoder cannot produce anything else. Every other test in
// this repository checks the SCHEMA -- that the enum is present and non-empty
// -- and none checked the grammar that is handed to the sampler.
//
// The grammar must be built the way the SERVER builds it. The schema travels as
// JSON, is unmarshalled into `functions.Item`, wrapped in a
// JSONFunctionStructure and converted there (core/http/endpoints/openai/chat.go).
// Converting the in-memory Go map directly instead produces a DIFFERENT and
// misleading result -- a []string enum fails the converter's `.([]any)`
// assertion and every slot degrades to the generic string rule -- which is how
// this investigation briefly concluded the keystone was broken when it was not.
// The JSON round-trip below is not incidental; it is the point.
func TestIssuedGrammarConstrainsEveryFieldSlot(t *testing.T) {
	fields := []FieldDescriptorV1{
		{FieldID: "cdr.msisdn", EffectiveType: "STRING",
			AllowedAggregates: []string{"COUNT", "COUNT_DISTINCT"}},
		{FieldID: "cdr.event_time", EffectiveType: fieldTypeTimestamp,
			AllowedAggregates: []string{"COUNT", "MIN", "MAX"}},
	}
	rules := grammarRules(servingPathGrammar(t, fields))

	// Every slot that names a FIELD must be an alternation of the issued IDs,
	// never the generic string rule.
	// Both filter variants are listed, not one. Splitting the filter object
	// into a null test and a valued comparison renamed these rules, and a test
	// that named only the pre-split rule would have gone quiet about the
	// variant it no longer covered instead of failing.
	for _, rule := range []string{
		"root-0-project-item",
		"root-0-group-fields-item",
		"root-0-filters-item-0-field-id",
		"root-0-filters-item-1-field-id",
		"root-0-measures-item-1-field-id",
	} {
		body, ok := rules[rule]
		if !ok {
			t.Errorf("%s is absent; the field slot is unconstrained", rule)
			continue
		}
		if isGenericString(body) {
			t.Errorf("%s compiled to the GENERIC STRING rule -- any field name would be "+
				"accepted, which is the keystone failing silently: %s", rule, body)
			continue
		}
		if !containsString(grammarLiterals(body), "cdr.msisdn") {
			t.Errorf("%s does not enumerate the issued fields: %s", rule, body)
		}
	}

	// And the operator slots, which bound what may be done to a field.
	for _, rule := range []string{
		"root-0-filters-item-0-op",
		"root-0-filters-item-1-op",
		"root-0-measures-item-1-op",
	} {
		body, ok := rules[rule]
		if !ok || isGenericString(body) {
			t.Errorf("%s is unconstrained: %q", rule, body)
		}
	}
}

// CDR-08's property, asserted where it actually takes effect. The row-count
// variant must permit COUNT and an empty field and nothing else, so
// COUNT_DISTINCT of nothing cannot be written down at all.
func TestGrammarCannotWriteCountDistinctOfNothing(t *testing.T) {
	fields := []FieldDescriptorV1{
		{FieldID: "cdr.msisdn", EffectiveType: "STRING",
			AllowedAggregates: []string{"COUNT", "COUNT_DISTINCT"}},
	}
	rules := grammarRules(servingPathGrammar(t, fields))

	op, okOp := rules["root-0-measures-item-0-op"]
	fieldID, okField := rules["root-0-measures-item-0-field-id"]
	if !okOp || !okField {
		t.Fatal("the row-count measure variant must exist as its own rule")
	}
	if ops := grammarLiterals(op); len(ops) != 1 || ops[0] != "COUNT" {
		t.Errorf("the empty-field variant must permit COUNT alone, got: %v", ops)
	}
	if ids := grammarLiterals(fieldID); len(ids) != 1 || ids[0] != "" {
		t.Errorf("the row-count variant must take the empty field only, got: %v", ids)
	}

	// The aggregate variant carries COUNT_DISTINCT, over a real field.
	aggregateOps := grammarLiterals(rules["root-0-measures-item-1-op"])
	aggregateFields := grammarLiterals(rules["root-0-measures-item-1-field-id"])
	if !containsString(aggregateOps, "COUNT_DISTINCT") {
		t.Errorf("COUNT_DISTINCT must remain reachable: %v", aggregateOps)
	}
	if containsString(aggregateFields, "") {
		t.Errorf("the aggregate variant must not accept an empty field: %v", aggregateFields)
	}
}

// The correction, encoded so it cannot be re-derived wrongly: the two enum
// representations are INDISTINGUISHABLE on the wire, so the client-side Go type
// cannot affect what the server builds.
func TestEnumRepresentationsAreIdenticalOnTheWire(t *testing.T) {
	asStrings, err := json.Marshal(map[string]any{"enum": []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	asAny, err := json.Marshal(map[string]any{"enum": grammarEnum([]string{"a", "b"})})
	if err != nil {
		t.Fatal(err)
	}
	if string(asStrings) != string(asAny) {
		t.Fatalf("the wire forms differ: %s vs %s", asStrings, asAny)
	}
	t.Logf("both marshal to %s -- the server cannot tell them apart, which is why "+
		"changing the Go type changed no measured outcome", asAny)
}

// servingPathGrammar builds the grammar exactly as the server does.
func servingPathGrammar(t *testing.T, fields []FieldDescriptorV1) string {
	t.Helper()
	wire, err := json.Marshal(semanticSourceNativeSchema(fields))
	if err != nil {
		t.Fatalf("the issued schema must marshal: %v", err)
	}
	var item functions.Item
	if err := json.Unmarshal(wire, &item); err != nil {
		t.Fatalf("the server unmarshals the schema into functions.Item: %v", err)
	}
	structure := &functions.JSONFunctionStructure{AnyOf: []functions.Item{item}}
	grammar, err := structure.Grammar()
	if err != nil {
		t.Fatalf("grammar generation failed; the server logs this and proceeds with NO "+
			"grammar at all, so a failure here is a silent loss of every constraint: %v", err)
	}
	return grammar
}

var grammarRulePattern = regexp.MustCompile(`^([a-zA-Z0-9_-]+)\s*::=\s*(.*)$`)

func grammarRules(grammar string) map[string]string {
	rules := map[string]string{}
	for _, line := range strings.Split(grammar, "\n") {
		if match := grammarRulePattern.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
			rules[match[1]] = match[2]
		}
	}
	return rules
}

// isGenericString reports a rule body that is just the shared `string` rule,
// which accepts any quoted text at all.
func isGenericString(body string) bool {
	return strings.TrimSpace(body) == "string"
}

// grammarLiterals extracts the values a GBNF alternation accepts.
//
// A rule body reads `"\"cdr.msisdn\"" | "\"cdr.event_time\""` -- GBNF
// literals wrapping JSON strings, so each alternative carries two layers of
// quoting. Comparing against a hand-written quoted form is how the first
// version of this test reported a correct grammar as broken; stripping the
// quoting and comparing values is unambiguous.
func grammarLiterals(body string) []string {
	out := []string{}
	for _, alternative := range strings.Split(body, "|") {
		trimmed := strings.TrimSpace(alternative)
		if trimmed == "" {
			continue
		}
		out = append(out, strings.NewReplacer(`\`, "", `"`, "").Replace(trimmed))
	}
	return out
}
