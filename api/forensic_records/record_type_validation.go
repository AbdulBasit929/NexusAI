package main

import (
	"fmt"
	"os"
	"strings"
)

// AN EXPLICIT record_type THAT NAMES NO KNOWN FAMILY MUST NEVER BECOME A
// STATEMENT OF ABSENCE.
//
// Measured 2026-09-27 against the running service, collection
// nexusai-forensic-demo, "How many cell towers are in this case?":
//
//	record_type=tower            -> "There are no tower records in this case."
//	record_type=tower_location   -> "There are 5 tower records in this case."
//
// The analyst workspace's Tower scope sent `tower`. The request field was
// normalised and then used as a scope filter with no validation, so it matched
// zero rows and the zero was narrated as a finding: a CONFIDENT FALSE NEGATIVE,
// the worst answer this product can give. The client has been corrected, but
// the service must not depend on every client spelling a family right.
//
// ONE DEFINITION of what a family name means. A question that says "tower" is
// already scoped to `tower_location` by `extractCanonicalRecordType`; the
// explicit field now goes through the same mapping rather than a second,
// divergent one:
//
//   - a record type the service knows              -> kept as is
//   - a documented alias ("tower", "towers")        -> canonicalised to its family
//   - anything else                                 -> REFUSED, never narrated
//
// "Known" is the union of the canonical structured families and every
// record_type the curated semantic layer declares. `email` stays known on
// purpose: it is how a family absent from the case correctly scopes to zero
// rows instead of counting everything (see extractCanonicalRecordType).
const recordTypeValidationEnv = "FORENSIC_RECORD_TYPE_VALIDATION"

// Default ON. This is a safety fix whose off-state is the unsafe one -- off is
// what turns a misspelt family into "there are no records" -- so, like
// FORENSIC_CONSTRAINT_OBLIGATIONS, it ships on and can be turned off explicitly.
func recordTypeValidationEnabled() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv(recordTypeValidationEnv)), "false") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// canonicalStructuredRecordTypes mirrors the values `extractCanonicalRecordType`
// can return. Kept as data here because that function returns a mapping from
// free text, and a request field needs membership, not extraction.
var canonicalStructuredRecordTypes = []string{
	"email", "access_log", "ipdr", "anpr", "transaction", "cdr",
	"tower_location", "subscriber", "generic",
}

func knownRecordTypes() map[string]bool {
	known := map[string]bool{}
	for _, value := range canonicalStructuredRecordTypes {
		known[value] = true
	}
	if layer, _ := defaultSemanticLayer(); layer != nil {
		for _, entity := range layer.Entities {
			if value := strings.ToLower(strings.TrimSpace(entity.RecordType)); value != "" {
				known[value] = true
			}
		}
	}
	return known
}

// canonicalizeRequestedRecordType returns the record type to scope by, whether
// it was rewritten from an alias, and an error when the value names no family.
func canonicalizeRequestedRecordType(value string) (string, bool, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return "", false, nil
	}
	known := knownRecordTypes()
	if known[trimmed] {
		return trimmed, false, nil
	}
	// The alias must resolve to exactly the family the question path would
	// choose, and that family must itself be known. A single-token value is
	// required so a phrase cannot pick up a family from an incidental word.
	if !strings.ContainsAny(trimmed, " \t") {
		spelled := strings.ReplaceAll(trimmed, "_", " ")
		if canonical := extractCanonicalRecordType(spelled); canonical != "" && known[canonical] {
			return canonical, true, nil
		}
	}
	return "", false, fmt.Errorf("record_type %q is not an evidence family this service knows; "+
		"no scope was applied and nothing was counted", value)
}
