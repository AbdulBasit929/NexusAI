package main

import (
	"strings"
	"testing"
)

// The measured case: record_type=tower answered "There are no tower records in
// this case" about a case holding five. Every row here is a value a client can
// send, and what the service must do with it.
func TestCanonicalizeRequestedRecordType(t *testing.T) {
	cases := []struct {
		value     string
		want      string
		rewritten bool
		refused   bool
	}{
		{"", "", false, false},
		// Known families pass through untouched.
		{"cdr", "cdr", false, false},
		{"tower_location", "tower_location", false, false},
		{"ACCESS_LOG", "access_log", false, false},
		// Kept known on purpose: an absent family scopes to zero rather than
		// counting every record in the case (NEG-04).
		{"email", "email", false, false},
		// Derived families declared by the curated layer.
		{"anpr_model_observation", "anpr_model_observation", false, false},
		// THE MEASURED DEFECT: the alias the workspace sent.
		{"tower", "tower_location", true, false},
		{"towers", "tower_location", true, false},
		// Names no family: refused, never narrated as absence.
		{"xyz", "", false, true},
		{"towerz", "", false, true},
		// A phrase must not pick up a family from an incidental word.
		{"call status", "", false, true},
	}
	for _, tc := range cases {
		got, rewritten, err := canonicalizeRequestedRecordType(tc.value)
		if tc.refused {
			if err == nil {
				t.Errorf("%q must be refused, got %q", tc.value, got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
			continue
		}
		if err != nil {
			t.Errorf("%q must be accepted, got error %v", tc.value, err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			continue
		}
		if got != tc.want || rewritten != tc.rewritten {
			t.Errorf("%q -> %q rewritten=%v, want %q rewritten=%v", tc.value, got, rewritten, tc.want, tc.rewritten) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

// The refusal must say that nothing was counted. A refusal that reads like an
// empty result would be the same defect in different words.
func TestRefusalStatesNothingWasCounted(t *testing.T) {
	_, _, err := canonicalizeRequestedRecordType("xyz")
	if err == nil || !strings.Contains(err.Error(), "nothing was counted") {
		t.Fatalf("the refusal must state that no scope was applied, got %v", err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

// Default ON: the off-state is the unsafe one.
func TestRecordTypeValidationDefaultsOn(t *testing.T) {
	t.Setenv(recordTypeValidationEnv, "")
	if !recordTypeValidationEnabled() {
		t.Fatal("must default on") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	t.Setenv(recordTypeValidationEnv, "false")
	if recordTypeValidationEnabled() {
		t.Fatal("must be switchable off explicitly") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}
