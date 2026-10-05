package main

import (
	"strings"
	"testing"
)

// PII MASKING AT PROJECTION. The WI-LAYER-7 ruling: a plate candidate projects
// as one opaque, case-scoped alias shared across both plate families, and
// "revealing a prefix, suffix or length is not approved".

const testAliasSecret = "test-only-alias-secret-not-a-deployment-value"

// THE ALIAS DISCLOSES NOTHING ABOUT THE PLATE. Not a prefix, not a suffix, not
// the length -- the ruling names all three.
func TestPlateAliasDisclosesNothing(t *testing.T) {
	t.Setenv(piiAliasSecretEnv, testAliasSecret)
	for _, plate := range []string{"ABC-123", "MN1367", "LEA-09-1234", "Z-1"} {
		alias, ok := piiPlateAlias("case-alpha", plate)
		if !ok {
			t.Fatalf("%s produced no alias", plate) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		bare := strings.TrimPrefix(alias, "Plate candidate ")
		compact := normalizePlateForAlias(plate)
		if strings.Contains(strings.ToUpper(alias), compact) {
			t.Errorf("alias for %s contains the plate itself: %s", plate, alias) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		for n := 2; n <= len(compact); n++ {
			if strings.Contains(bare, compact[:n]) {
				t.Errorf("alias for %s leaks the %d-char prefix %q: %s", plate, n, compact[:n], alias) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
			if strings.Contains(bare, compact[len(compact)-n:]) {
				t.Errorf("alias for %s leaks the %d-char suffix %q: %s", plate, n, compact[len(compact)-n:], alias) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		if len(bare) != piiAliasChars {
			t.Errorf("alias for %s is %d chars; a varying width discloses plate length", plate, len(bare)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

// FIXED WIDTH ACROSS WILDLY DIFFERENT LENGTHS -- the length-disclosure clause,
// asserted directly rather than as a side effect of the loop above.
func TestPlateAliasWidthIsConstant(t *testing.T) {
	t.Setenv(piiAliasSecretEnv, testAliasSecret)
	widths := map[int]bool{}
	for _, plate := range []string{"A1", "ABC-123", "ABCDEFGHIJKLMNOP-1234567890"} {
		alias, ok := piiPlateAlias("case-alpha", plate)
		if !ok {
			t.Fatalf("%s produced no alias", plate) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		widths[len(alias)] = true
	}
	if len(widths) != 1 {
		t.Errorf("alias width varies with plate length: %v", widths) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

// STABLE WITHIN A CASE, AND SHARED ACROSS THE TWO PLATE FAMILIES. Both curate
// observation.normalized_plate_text, and the same vehicle must read as the same
// alias whether it came from a still or a video group -- that is the whole
// analytical value of masking rather than withholding.
func TestPlateAliasIsStableAndSharedWithinACase(t *testing.T) {
	t.Setenv(piiAliasSecretEnv, testAliasSecret)
	first, _ := piiPlateAlias("case-alpha", "ABC-123")
	again, _ := piiPlateAlias("case-alpha", "ABC-123")
	if first != again {
		t.Error("the alias is not stable for the same plate in the same case") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	// Separator and case differences are the SAME vehicle. Two aliases would
	// split one subject in two, which fabricates a distinction.
	for _, variant := range []string{"abc-123", "ABC 123", "ABC123", " abc123 "} {
		got, _ := piiPlateAlias("case-alpha", variant)
		if got != first {
			t.Errorf("%q produced a different alias than ABC-123; one vehicle would read as two subjects", variant) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

// CASE-SCOPED. The same plate in another case must not be correlatable.
func TestPlateAliasIsCaseScoped(t *testing.T) {
	t.Setenv(piiAliasSecretEnv, testAliasSecret)
	alpha, _ := piiPlateAlias("case-alpha", "ABC-123")
	beta, _ := piiPlateAlias("case-beta", "ABC-123")
	if alpha == beta {
		t.Error("the same plate yields the same alias in two cases -- cross-case correlation is possible") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

// A DIFFERENT SECRET YIELDS A DIFFERENT ALIAS. This is what makes the alias a
// MAC rather than a digest: without it, plate strings are low enough entropy to
// enumerate and the alias is just the plate re-encoded.
func TestPlateAliasDependsOnTheSecret(t *testing.T) {
	t.Setenv(piiAliasSecretEnv, testAliasSecret)
	withFirst, _ := piiPlateAlias("case-alpha", "ABC-123")
	t.Setenv(piiAliasSecretEnv, "a-completely-different-secret")
	withSecond, _ := piiPlateAlias("case-alpha", "ABC-123")
	if withFirst == withSecond {
		t.Fatal("the alias does not depend on the secret -- it is an unkeyed digest, reversible by enumeration") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

// FAILS CLOSED WITHOUT A SECRET. It must refuse, not degrade to an unkeyed
// digest and not emit the raw value. A scheme that silently weakens while still
// labelled MASKED is worse than no masking.
func TestMaskingFailsClosedWithoutASecret(t *testing.T) {
	t.Setenv(piiAliasSecretEnv, "")
	if piiMaskingAvailable() {
		t.Error("masking reports available with no secret") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	alias, ok := piiPlateAlias("case-alpha", "ABC-123")
	if ok {
		t.Errorf("an alias was produced with no secret: %q", alias) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if strings.Contains(alias, "ABC") {
		t.Fatal("the raw plate leaked when masking was unavailable") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	// And the field must not be admitted to the catalogue at all.
	t.Setenv(piiMaskedProjectionEnv, "true")
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	entity, present := layer.EntityByFamily("anpr_model_observation")
	if !present {
		t.Skip("family missing") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	for _, field := range entity.CatalogFields(hybridQueryRequest{CollectionID: "case-alpha"}) {
		if strings.Contains(field.FieldID, "plate_text") {
			t.Error("a MASKED field was issued while masking was unavailable -- it would project raw") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

// WITHHELD IS NOT MASKED. The five OCR and transcript fields must stay out of
// the catalogue even with masking fully enabled: WI-LAYER-7 ruled token masking
// insufficient for free text, because names and addresses are not recognisable
// by token syntax.
func TestWithheldTextIsNeverAdmittedEvenWithMaskingOn(t *testing.T) {
	t.Setenv(piiAliasSecretEnv, testAliasSecret)
	t.Setenv(piiMaskedProjectionEnv, "true")
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	withheld := map[string][]string{
		"image_ocr_observation":    {"raw_text", "normalized_text"},
		"audio_timestamp_segment":  {"text"},
		"audio_roman_urdu_segment": {"raw_urdu_text", "roman_urdu_text"},
	}
	req := hybridQueryRequest{CollectionID: "nexusai-multimodal-product-acceptance"}
	for family, names := range withheld {
		entity, ok := layer.EntityByFamily(family)
		if !ok {
			t.Errorf("%s is not curated", family) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			continue
		}
		issued := map[string]bool{}
		for _, field := range entity.CatalogFields(req) {
			issued[bareFieldName(field.FieldID)] = true
		}
		for _, name := range names {
			if issued[name] {
				t.Errorf("%s.%s is WITHHELD but was issued with masking on", family, name) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	}
}

// THE MASKED FIELD IS ADMITTED WHEN EVERY CONDITION HOLDS, and arrives marked so
// downstream code can tell it apart.
func TestMaskedPlateIsAdmittedAndMarked(t *testing.T) {
	t.Setenv(piiAliasSecretEnv, testAliasSecret)
	t.Setenv(piiMaskedProjectionEnv, "true")
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	req := hybridQueryRequest{CollectionID: "nexusai-multimodal-product-acceptance"}
	for _, family := range []string{"anpr_model_observation", "video_anpr_plate_group"} {
		entity, ok := layer.EntityByFamily(family)
		if !ok {
			t.Errorf("%s is not curated", family) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			continue
		}
		found := false
		for _, field := range entity.CatalogFields(req) {
			if !strings.Contains(field.FieldID, "plate_text") {
				continue
			}
			found = true
			if field.RedactionState != "MASKED" {
				t.Errorf("%s: plate_text admitted with redaction %q", family, field.RedactionState) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
			if field.Sensitivity != "PII" {
				t.Errorf("%s: plate_text admitted with sensitivity %q", family, field.Sensitivity) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
			// SORTING IS THE SUBTLE LEAK: SQL orders by the RAW value while the
			// analyst sees aliases, so the ORDER reconstructs the plates.
			if field.Sortable {
				t.Errorf("%s: a MASKED field is sortable -- the ordering discloses the raw values", family) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		if !found {
			t.Errorf("%s: plate_text was not admitted although every condition holds", family) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

// THE SWITCH DEFAULTS OFF, and off the field stays withheld exactly as it ships.
func TestMaskedProjectionDefaultsOff(t *testing.T) {
	t.Setenv(piiAliasSecretEnv, testAliasSecret)
	t.Setenv(piiMaskedProjectionEnv, "")
	if piiMaskedProjectionEnabled() {
		t.Fatal("masked projection defaults on") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	entity, ok := layer.EntityByFamily("anpr_model_observation")
	if !ok {
		t.Skip("family missing") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	for _, field := range entity.CatalogFields(hybridQueryRequest{CollectionID: "c"}) {
		if strings.Contains(field.FieldID, "plate_text") {
			t.Error("plate_text is issued with the switch off") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

// AN UNRECOGNISED REDACTION WITHHOLDS. A new redaction value must be
// implemented deliberately, never default to disclosure.
func TestUnknownRedactionIsNotMasked(t *testing.T) {
	t.Setenv(piiAliasSecretEnv, testAliasSecret)
	for _, state := range []string{"", "VISIBLE", "WITHHELD", "SOMETHING_NEW"} {
		if _, handled := maskCuratedValue(FieldDescriptorV1{RedactionState: state}, "c", "ABC-123"); handled {
			t.Errorf("redaction %q was treated as maskable", state) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	value, handled := maskCuratedValue(FieldDescriptorV1{RedactionState: "MASKED"}, "c", "ABC-123")
	if !handled {
		t.Fatal("a MASKED field was not handled") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if strings.Contains(stringValueAny(value), "ABC") {
		t.Fatalf("the raw plate survived masking: %v", value) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

// THE SUBSCRIBER FIELDS ARE MASKED IN CURATION BUT HAVE NO SCHEME, so they must
// never be admitted. This is the defect the first version of this change shipped
// into the catalogue: subscriber.cnic is a national identity number, and it was
// admitted and would have been run through the PLATE aliaser.
func TestUnimplementedMaskSchemesAreNeverAdmitted(t *testing.T) {
	t.Setenv(piiAliasSecretEnv, testAliasSecret)
	t.Setenv(piiMaskedProjectionEnv, "true")
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	entity, ok := layer.EntityByFamily("subscriber_identity")
	if !ok {
		t.Skip("subscriber_identity missing") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	declaredMasked := 0
	for _, field := range entity.Fields {
		if field.Sensitivity == "PII" && field.Redaction == "MASKED" {
			declaredMasked++
		}
	}
	if declaredMasked == 0 {
		t.Fatal("no PII/MASKED subscriber field found, so this test guards nothing") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	for _, field := range entity.CatalogFields(hybridQueryRequest{CollectionID: "case-alpha"}) {
		for _, banned := range []string{"cnic", "full_name"} {
			if strings.Contains(field.FieldID, banned) {
				t.Errorf("%s was admitted with no implemented masking scheme", field.FieldID) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	}
	t.Logf("%d PII/MASKED subscriber fields declared, none admitted", declaredMasked) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
}

// AND IF ONE EVER REACHES THE MASKER ANYWAY, it yields nothing rather than the
// raw value. Defence in depth: admission and masking must both refuse.
func TestAFieldWithNoSchemeMasksToNothing(t *testing.T) {
	t.Setenv(piiAliasSecretEnv, testAliasSecret)
	field := FieldDescriptorV1{
		FieldID: "subscriber.cnic", RedactionState: "MASKED",
		SourceName: "cnic", SourceNames: []string{"cnic"},
	}
	value, handled := maskCuratedValue(field, "case-alpha", "3520112345671")
	if !handled {
		t.Fatal("a MASKED field was passed through unhandled") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if strings.Contains(stringValueAny(value), "3520") {
		t.Fatalf("the raw CNIC survived: %v", value) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if strings.Contains(stringValueAny(value), "Plate candidate") {
		t.Fatalf("a CNIC was labelled as a plate alias: %v", value) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}
