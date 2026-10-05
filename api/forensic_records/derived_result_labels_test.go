package main

import (
	"strings"
	"testing"
)

func derivedResponse(artifactTypes ...string) hybridQueryResponse {
	provenance := []map[string]any{}
	for _, artifactType := range artifactTypes {
		entry := map[string]any{"artifact_id": "a"}
		if artifactType != "" {
			entry["artifact_type"] = artifactType
		}
		provenance = append(provenance, entry)
	}
	return hybridQueryResponse{Records: map[string]any{
		"provenance":            provenance,
		"source_native_results": []map[string]any{{"m1": 307}},
	}}
}

func TestDerivedResultNoun(t *testing.T) {
	plateReads := derivedResponse("forensics.anpr-observation/v1", "forensics.anpr-observation/v1")

	t.Run("switch off keeps the record-type noun", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(derivedResultLabelsEnv, "")
		if got := resultNoun(hybridQueryRequest{RecordType: "anpr"}, plateReads, 307); got != "ANPR sightings" {
			t.Fatalf("noun = %q, want today's %q", got, "ANPR sightings") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("switch on names what was counted", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(derivedResultLabelsEnv, "true")
		cases := map[string]struct {
			resp hybridQueryResponse
			n    float64
			want string
		}{
			"plate reads (M1)":     {plateReads, 307, "plate reads"},
			"one plate read":       {plateReads, 1, "plate read"},
			"video plate groups":   {derivedResponse("forensics.video-anpr-plate-group/v1"), 24, "video plate groups"},
			"faces (M16)":          {derivedResponse("forensics.face-observation/v1"), 20, "detected faces"},
			"image text regions":   {derivedResponse("forensics.image-ocr-observation/v1"), 309, "image text regions"},
			"audio segments (M13)": {derivedResponse("forensics.audio-timestamp-segment/v1"), 11, "transcribed audio segments"},
		}
		for name, c := range cases {
			if got := resultNoun(hybridQueryRequest{RecordType: "anpr"}, c.resp, c.n); got != c.want {
				t.Errorf("%s: noun = %q, want %q", name, got, c.want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		// Camera sightings (P1) cite source rows, not artifacts, and keep their noun.
		kept := map[string]hybridQueryResponse{
			"structured provenance": derivedResponse(""),
			"mixed artifact types":  derivedResponse("forensics.anpr-observation/v1", "forensics.face-observation/v1"),
			"partly structured":     derivedResponse("forensics.anpr-observation/v1", ""),
			"unknown artifact type": derivedResponse("forensics.something-new/v9"),
			"no provenance":         {},
		}
		for name, resp := range kept {
			if got := resultNoun(hybridQueryRequest{RecordType: "anpr"}, resp, 1057); got != "ANPR sightings" {
				t.Errorf("%s: noun = %q, must stay %q", name, got, "ANPR sightings") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	})

	t.Run("the count sentence uses it", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(derivedResultLabelsEnv, "true")
		plan := &SourceNativePlanV1{Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}}
		answer, ok := sourceNativeResultAnswer(hybridQueryRequest{RecordType: "anpr"}, plateReads, plan)
		if !ok || answer.Headline != "There are 307 plate reads in this case." {
			t.Fatalf("headline = %q (ok=%v), want %q", answer.Headline, ok, "There are 307 plate reads in this case.") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if strings.Contains(answer.Headline, "sighting") {
			t.Fatal("a model's plate read must not be called a sighting") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}
