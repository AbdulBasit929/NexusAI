package main

import (
	"strings"
	"testing"
)

func TestPlateConfusionCostCountsCommonMixUpsAsOneAndOtherEditsAsTwo(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"SYNT9150", "SYNT9150", 0},
		{"SYNT4O24", "SYNT4024", 1}, // O and 0
		{"SYNT2905", "SYNI2905", 1}, // T and I
		{"LEA1234", "LEA1284", 2},   // 3 and 8 are not a listed pair
		{"LEA1234", "LEA123", 2},    // a dropped character
		{"LEA1234", "LEA12345", 2},  // an added character
		{"LEA1234", "LEB1234", 2},
		{"LEA1234", "MNS9999", 14},
	} {
		if got := plateConfusionCost(c.a, c.b); got != c.want {
			t.Fatalf("%s vs %s: cost %d, want %d", c.a, c.b, got, c.want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestSearchPlateReadsKeepsExactAndSimilarApartAndNeverUsesAFormat(t *testing.T) {
	rows := []plateReadRow{
		{EvidenceID: "e1", File: "a.jpg", Key: "SYNT9150", Observation: "o1"},
		{EvidenceID: "e2", File: "b.jpg", Key: "SYNI9150", Observation: "o2"}, // similar: T read as I
		{EvidenceID: "e3", File: "c.jpg", Key: "LEA1234", Observation: "o3"},  // a different plate
		{EvidenceID: "e1", File: "a.jpg", Key: "SYNT9150", Observation: "o1"}, // the same observation twice
		{EvidenceID: "e4", File: "d.jpg", Key: "SYNT915", Observation: "o4"},  // a dropped character
		{EvidenceID: "e5", File: "e.jpg", Key: "", Observation: "o5"},         // an empty reading
	}
	r := searchPlateReads("SYNT9150", rows)
	if got := distinctPlateFiles(r.Exact); len(got) != 1 || got[0] != "a.jpg" {
		t.Fatalf("exact %v", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if got := distinctPlateFiles(r.Similar); len(got) != 2 || got[0] != "b.jpg" || got[1] != "d.jpg" {
		t.Fatalf("similar %v (ranked by cost, then name)", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	// a plate in no registered format is matched like any other: no format is consulted
	odd := searchPlateReads("ZQ9X77KK", []plateReadRow{{EvidenceID: "e9", File: "odd.jpg", Key: "ZQ9X77KK", Observation: "o9"}})
	if len(odd.Exact) != 1 {
		t.Fatal("an unusual plate must be found exactly") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	// too short for a similarity search: exact only
	short := searchPlateReads("AB12", []plateReadRow{{EvidenceID: "e1", File: "s.jpg", Key: "A812", Observation: "o1"}})
	if len(short.Similar) != 0 || len(short.Exact) != 0 {
		t.Fatal("a plate shorter than five characters is searched exactly only") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestPlateSearchSentenceNamesTheTypedPlateAndNeverAStoredReading(t *testing.T) {
	rows := []plateReadRow{{EvidenceID: "e2", File: "b.jpg", Key: "SYNI2905", Observation: "o2"}, {EvidenceID: "e4", File: "d.jpg", Key: "SYNT4024", Observation: "o4"}}
	text := plateSearchSentence(searchPlateReads("SYNT2905", rows))
	if !strings.Contains(text, "No image or video reading matches the plate SYNT2905 exactly.") || !strings.Contains(text, "b.jpg") || !strings.Contains(text, "check it by eye") {
		t.Fatalf("sentence %q", text) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if strings.Contains(text, "SYNI2905") || strings.Contains(text, "SYNT4024") {
		t.Fatalf("a stored reading must never be printed: %q", text) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	exact := plateSearchSentence(searchPlateReads("SYNT4024", rows))
	if !strings.HasPrefix(exact, "The plate SYNT4024 was read in 1 image or video file: d.jpg.") {
		t.Fatalf("exact sentence %q", exact) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	none := plateSearchSentence(searchPlateReads("LEA1234", rows))
	if !strings.Contains(none, "may have misread it") || !strings.Contains(none, "does not show the plate is absent") {
		t.Fatalf("empty sentence %q", none) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestPlateSearchSentenceShortensALongFileList(t *testing.T) {
	var rows []plateReadRow
	for i := 0; i < 8; i++ {
		rows = append(rows, plateReadRow{EvidenceID: string(rune('a' + i)), File: "f" + string(rune('a'+i)) + ".jpg", Key: "SYNT9150", Observation: "o" + string(rune('a'+i))})
	}
	text := plateSearchSentence(searchPlateReads("SYNT9150", rows))
	if !strings.Contains(text, "read in 8 image or video files") || !strings.Contains(text, "and 3 more") {
		t.Fatalf("sentence %q", text) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestAnalystPlateLiteralsAreFormatFree(t *testing.T) {
	for _, c := range []struct {
		question string
		want     []string
	}{
		{"Which images show the plate SYNT4O24?", []string{"SYNT4O24"}},     // letters, digits, a letter, digits
		{"Which images show the plate AB12CD3456?", []string{"AB12CD3456"}}, // two letter groups
		{"Which photos show the plate 9ABC123?", []string{"9ABC123"}},       // digit first
		{"Which images show the plate LEA-1234?", []string{"LEA1234"}},      // the usual shape still works
		{"Which images show the plate lea 1234?", []string{"LEA1234"}},
		{"Which images show the plates SYNT2905 and SYNT9150?", []string{"SYNT2905", "SYNT9150"}},
		{"Which images show the plate with the file DSC_1105.JPG?", nil}, // a file name is not a plate
		{"Which images show the plate 03001234567?", nil},                // a phone number is not a plate
		{"Which images show the plate on MP4 footage?", nil},             // too short
	} {
		got := analystPlateLiterals(c.question)
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Fatalf("%q: got %v, want %v", c.question, got, c.want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}
