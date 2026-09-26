package main

import (
	"strings"
	"testing"
)

// A curated numeric column must reach SQL as a NUMBER. The layer declares
// NUMBER; the SQL builder switches on INTEGER/DECIMAL and sends anything else
// down the raw-text path, so every curated numeric field was compared as text:
// MAX() returned the lexicographically largest string and "the largest
// transaction" answered 9,200 instead of 75,000. That was recorded as generator
// instability and is the stated reason the IR fallback shipped disabled.
func TestCuratedTypesReachTheExecutorAsExecutableTypes(t *testing.T) {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	checked := 0
	for _, entity := range layer.Entities {
		for _, field := range entity.CatalogFields(hybridQueryRequest{}) {
			switch field.EffectiveType {
			case fieldTypeString, fieldTypeInteger, fieldTypeDecimal,
				fieldTypeBoolean, fieldTypeTimestamp, fieldTypeDate, fieldTypeUnknown:
			default:
				t.Errorf("%s has effective type %q, which the SQL builder does not "+
					"recognise — it will be compared as raw text",
					field.FieldID, field.EffectiveType)
			}
			if field.EffectiveType == fieldTypeDecimal || field.EffectiveType == fieldTypeInteger {
				args := []any{}
				if !strings.Contains(sourceNativeFieldTypedExpr(sourceNativeRecordsBinding(), field, &args), "numeric") {
					t.Errorf("%s is numeric but its SQL carries no numeric cast", field.FieldID)
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no curated numeric field was exercised; the guard proves nothing")
	}
	t.Logf("verified %d curated numeric fields cast to numeric", checked)
}
