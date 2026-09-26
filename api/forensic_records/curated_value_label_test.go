package main

import "testing"

// One screen showed the answer saying "GPRS: 5,863 ; SMS: 1,108 ; CALL: 930"
// above a table saying "Data session", "SMS", "Call" -- the same values in two
// vocabularies. The dimension was curated; its values were not.
func TestCuratedValueLabelUsesTheLayersValueDisplayNames(t *testing.T) {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated semantic layer not loadable from this working directory")
	}
	// Drive the assertion from the layer itself rather than hard-coding a
	// value: the corpus must not leak into the compiler.
	var fieldID, raw, want string
	for _, entity := range layer.Entities {
		for _, field := range entity.Fields {
			for _, value := range field.Values {
				if value.DisplayName != "" && value.DisplayName != value.Value {
					fieldID, raw, want = field.ID, value.Value, value.DisplayName
					break
				}
			}
		}
	}
	if fieldID == "" {
		t.Skip("no curated value carries a display name distinct from its raw value")
	}
	if got := curatedValueLabel(fieldID, raw); got != want {
		t.Errorf("curatedValueLabel(%q, %q) = %q, want %q", fieldID, raw, got, want)
	}
	// Case-insensitively too: the executor returns whatever the column holds.
	if got := curatedValueLabel(fieldID, lowerASCII(raw)); got != want {
		t.Errorf("lowercase %q resolved to %q, want %q", raw, got, want)
	}
}

func lowerASCII(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			out[i] = r + 32
		}
	}
	return string(out)
}

// An uncurated value returns unchanged. A raw identifier is a truthful
// fallback; inventing a label for a value the layer does not define would be
// worse than showing the value itself.
func TestCuratedValueLabelLeavesUnknownValuesAlone(t *testing.T) {
	for _, tc := range []struct{ field, value string }{
		{"cdr.call_type", "NOT_A_REAL_VALUE"},
		{"no.such.field", "GPRS"},
		{"", "GPRS"},
		{"cdr.call_type", ""},
	} {
		if got := curatedValueLabel(tc.field, tc.value); got != tc.value {
			t.Errorf("curatedValueLabel(%q, %q) = %q, want it unchanged", tc.field, tc.value, got)
		}
	}
}
