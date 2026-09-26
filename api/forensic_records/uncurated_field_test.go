package main

import (
	"strings"
	"testing"
)

func descriptor(id string, curated bool) FieldDescriptorV1 {
	return FieldDescriptorV1{
		FieldID: id, NormalizedName: strings.ReplaceAll(id, ".", "_"),
		EffectiveType: "STRING", Curated: curated,
		AllowedFilters: []string{"EQ"}, AllowedAggregates: []string{"COUNT"},
		Projectable: true, Groupable: true,
	}
}

// MEASURED AND REVERTED 2026-09-25. Withholding uncurated columns closed H12's
// field substitution and cost TWR-02 a FALSE NEGATIVE ("there are no tower
// records in this case") -- changing the issued enum changed plans for
// questions that already worked. This asserts the RESTORED behaviour so the
// revert cannot be silently undone; the route to closing H12 is curating the
// columns (WI-LAYER-1), not withholding them.
func TestUncuratedFieldsFillSpareSlots(t *testing.T) {
	catalog := []FieldDescriptorV1{
		descriptor("tower.site_code", true),
		descriptor("tower.latitude", true),
		descriptor("fld_0355ecbda8251b2bdc2c3462", false),
		descriptor("fld_deadbeefdeadbeefdeadbeef", false),
	}
	issued, _ := retrieveSourceNativeFields("What is the average beam width of the towers?", catalog)
	curated, uncurated := 0, 0
	for _, field := range issued {
		if field.Curated {
			curated++
		} else {
			uncurated++
		}
	}
	if curated != 2 {
		t.Fatalf("both curated fields must be issued, got %d", curated)
	}
	if uncurated == 0 {
		t.Fatal("uncurated columns fill the spare slots; withholding them was measured " +
			"at one confident-wrong (TWR-02) and reverted")
	}
}

// THE EMPTY-ENUM KILLER. A family with nothing curated must still be issued
// something: an empty enum compiles to an alternation with no alternatives and
// llama.cpp answers the whole request with "failed to parse grammar" -- HTTP
// 500 before any inference.
func TestUncuratedFieldsAreTheFallbackWhenNothingIsCurated(t *testing.T) {
	catalog := []FieldDescriptorV1{
		descriptor("fld_0355ecbda8251b2bdc2c3462", false),
		descriptor("fld_deadbeefdeadbeefdeadbeef", false),
	}
	issued, _ := retrieveSourceNativeFields("anything at all", catalog)
	if len(issued) == 0 {
		t.Fatal("with nothing curated the uncurated columns MUST be issued; an empty enum " +
			"is an unparseable grammar, not a narrow choice")
	}
}

// Curated fields keep being issued in full -- curation is the contract.
func TestCuratedFieldsAreStillAllIssued(t *testing.T) {
	catalog := []FieldDescriptorV1{
		descriptor("cdr.msisdn", true),
		descriptor("cdr.call_type", true),
		descriptor("cdr.event_time", true),
	}
	issued, _ := retrieveSourceNativeFields("how many calls of each type", catalog)
	if len(issued) != 3 {
		t.Fatalf("issued %d of 3 curated fields; a curated column must never be dropped", len(issued))
	}
}
