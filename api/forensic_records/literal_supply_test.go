package main

import (
	"strings"
	"testing"
)

// Every case here is reconstructed from a CACHED MODEL COMPLETION read on
// 2026-09-24, not from a guess about what the generator emits. The four
// questions all failed with one error string and turned out to have two
// unrelated causes; the controls are what keep them apart.

func supplyRequest(query string) hybridQueryRequest {
	return hybridQueryRequest{Query: query, TenantID: "t", CollectionID: "c",
		QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
}

// ---------------------------------------------------------------------------
// FIX A1 -- an identifier is a shape, not a digit.
// ---------------------------------------------------------------------------

func TestAllAlphabeticIdentifierIsAnalystSupplied(t *testing.T) {
	// SUB-03. The generator produced ONE filter and it was exactly right:
	// subscriber.subscriber_id EQ "PK-SUB-SYN-ALPHA". The allowlist returned an
	// EMPTY list, because the token holds no digit.
	values := semanticSourceNativeLiteralValues(
		supplyRequest("Who is the registered subscriber for PK-SUB-SYN-ALPHA?"))
	if !containsString(values, "PK-SUB-SYN-ALPHA") {
		t.Fatalf("the analyst typed this identifier; it must be supplied: %v", values)
	}
}

// The control that matters: widening this allowlist must not let the question's
// own vocabulary become a filterable value. If it did, every hallucinated
// CONTAINS filter on a question word would be waved through.
func TestOrdinaryQuestionWordsAreNeverAnalystSuppliedValues(t *testing.T) {
	values := semanticSourceNativeLiteralValues(
		supplyRequest("Where is tower PK-LHR-SYN-001 located in the area?"))
	for _, word := range []string{"where", "Where", "located", "area", "tower", "is", "in"} {
		if containsString(values, word) {
			t.Errorf("%q is a question word, not a value the analyst supplied: %v", word, values)
		}
	}
	// ...while the identifier in the same sentence still is.
	if !containsString(values, "PK-LHR-SYN-001") {
		t.Error("the identifier must remain supplied")
	}
}

func TestStructuredIdentifierTokenRequiresTwoSeparatedSegments(t *testing.T) {
	for _, token := range []string{"PK-SUB-SYN-ALPHA", "LHR-2026", "a/b", "x_y"} {
		if !structuredIdentifierToken(token) {
			t.Errorf("%q is separator-bearing with two segments; it is an identifier shape", token)
		}
	}
	for _, token := range []string{"outbound", "where", "area", "", "-", "call"} {
		if structuredIdentifierToken(token) {
			t.Errorf("%q is not an identifier shape", token)
		}
	}
}

// ---------------------------------------------------------------------------
// FIX A2 -- a date bound is legitimately absent from the question's text.
// ---------------------------------------------------------------------------

func temporalCatalog() []FieldDescriptorV1 {
	return []FieldDescriptorV1{
		{FieldID: "cdr.event_time", EffectiveType: fieldTypeTimestamp},
		{FieldID: "cdr.msisdn", EffectiveType: "STRING"},
	}
}

func TestDateBoundDerivedFromAMonthNameIsAnalystSupplied(t *testing.T) {
	// CDR-14. The generator's plan was exactly right and the allowlist held
	// only "2026", so string matching refused it.
	req := supplyRequest("How many call records are from August 2026?")
	req.DateFrom, req.DateTo = "2026-08-01", "2026-08-31"
	catalog := temporalCatalog()

	// Both renderings observed from the SAME question on different runs.
	for _, value := range []string{
		"2026-08-01T00:00:00Z", "2026-08-31T23:59:59Z", "2026-09-01T00:00:00Z",
	} {
		if !temporalFilterWithinRequestedRange(req, "cdr.event_time", value, catalog) {
			t.Errorf("%q is inside the month the analyst asked for", value)
		}
	}
}

func TestDateBoundOutsideTheRequestedRangeIsRefused(t *testing.T) {
	req := supplyRequest("How many call records are from August 2026?")
	req.DateFrom, req.DateTo = "2026-08-01", "2026-08-31"
	catalog := temporalCatalog()
	for _, value := range []string{"2026-06-01T00:00:00Z", "2027-01-01T00:00:00Z", "2026-07-31T23:59:59Z"} {
		if temporalFilterWithinRequestedRange(req, "cdr.event_time", value, catalog) {
			t.Errorf("%q is outside the analyst's range and must not be waved through", value)
		}
	}
}

// The three conditions are all required, or this becomes a general escape from
// the allowlist rather than a date rule.
func TestTemporalSupplyRequiresRangeFieldTypeAndParse(t *testing.T) {
	catalog := temporalCatalog()

	// No analyst-supplied range at all.
	noRange := supplyRequest("How many call records are there?")
	if temporalFilterWithinRequestedRange(noRange, "cdr.event_time", "2026-08-01T00:00:00Z", catalog) {
		t.Error("with no requested range there is nothing to validate against")
	}

	req := supplyRequest("How many call records are from August 2026?")
	req.DateFrom, req.DateTo = "2026-08-01", "2026-08-31"

	// A STRING column must never be validated as a date, whatever it holds.
	if temporalFilterWithinRequestedRange(req, "cdr.msisdn", "2026-08-01T00:00:00Z", catalog) {
		t.Error("only a TIMESTAMP/DATE field may carry a date bound")
	}
	// A field absent from the issued catalog is not temporal either.
	if temporalFilterWithinRequestedRange(req, "cdr.not_issued", "2026-08-01T00:00:00Z", catalog) {
		t.Error("a field that was never issued cannot be validated")
	}
	// An unparseable value is not a date.
	if temporalFilterWithinRequestedRange(req, "cdr.event_time", "outbound", catalog) {
		t.Error("a value that is not an instant is not a date bound")
	}
}

// ---------------------------------------------------------------------------
// THE CONTROL FOR BOTH FIXES -- an INVENTED value must still be refused.
// ---------------------------------------------------------------------------

// CDR-11 and TWR-02 fail with the same error string as SUB-03 and CDR-14 and
// are NOT the same defect. The generator appended filters the analyst never
// asked for -- `cdr.direction EQ "outbound"`, `tower.site_location CONTAINS
// "where"`, `tower.district CONTAINS "area"` -- and the allowlist is CORRECT to
// refuse them. Fix A must not move these.
func TestInventedFilterValuesRemainRefusedAfterWidening(t *testing.T) {
	cases := []struct {
		query    string
		invented []string
	}{
		{"How many calls did 923001110001 make?", []string{"outbound"}},
		{"Where is tower PK-LHR-SYN-001 located?", []string{"where", "area"}},
	}
	for _, tc := range cases {
		values := semanticSourceNativeLiteralValues(supplyRequest(tc.query))
		for _, invented := range tc.invented {
			if containsString(values, invented) {
				t.Errorf("%q was invented by the generator, not supplied by the analyst: %v",
					invented, values)
			}
			if containsString(values, strings.ToLower(invented)) && invented != strings.ToLower(invented) {
				t.Errorf("%q must not enter in any case form", invented)
			}
		}
	}
}
