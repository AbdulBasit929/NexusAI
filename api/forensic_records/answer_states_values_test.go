package main

import (
	"strings"
	"testing"
)

func rangePlan(lowField, highField string) *SourceNativePlanV1 {
	return &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures: []SourceNativeMeasureV1{
			{MeasureID: "m1", Op: "MIN", FieldID: lowField},
			{MeasureID: "m2", Op: "MAX", FieldID: highField},
		},
	}
}

func rangeResponse(plan *SourceNativePlanV1, low, high any) hybridQueryResponse {
	return hybridQueryResponse{Records: map[string]any{
		"plan": plan, "complete": true,
		"source_native_results": []map[string]any{{"m1": low, "m2": high}},
	}}
}

// OFF, the narration is byte-identical to today: a range still declines and the
// caller keeps its existing text. Every switch gating new behaviour owes this.
func TestAnswerStatesValuesOffDeclinesARange(t *testing.T) {
	t.Setenv(answerStatesValuesEnv, "")
	plan := rangePlan("cdr.event_time", "cdr.event_time")
	resp := rangeResponse(plan, "2026-04-01T19:00:02Z", "2026-08-10T05:33:00Z")
	if _, ok := buildResultAnswer(hybridQueryRequest{RecordType: "cdr"}, resp, nil); ok {
		t.Fatal("switch off must not produce a range answer")
	}
}

// CDR-09. "What date range do the CDR records cover?" answered "1 CDR record
// matched this question" -- a ROW COUNT to a question about two dates.
func TestAnswerStatesValuesStatesBothEndsOfARange(t *testing.T) {
	t.Setenv(answerStatesValuesEnv, "true")
	plan := rangePlan("cdr.event_time", "cdr.event_time")
	resp := rangeResponse(plan, "2026-04-01T19:00:02Z", "2026-08-10T05:33:00Z")
	answer, ok := buildResultAnswer(hybridQueryRequest{RecordType: "cdr"}, resp, nil)
	if !ok {
		t.Fatal("a range plan produced no answer")
	}
	for _, want := range []string{"2026-04-01", "2026-08-10"} {
		if !strings.Contains(answer.Headline, want) {
			t.Fatalf("headline %q omits %s", answer.Headline, want)
		}
	}
	if strings.Contains(answer.Headline, "T19:00:02Z") {
		t.Fatalf("timestamp not shortened for reading: %q", answer.Headline)
	}
}

// THE TRAP. shortDate truncates ANY string of ten characters or more, so a
// large number handed to it silently loses its tail and is stated with full
// confidence. A numeric range must survive intact.
func TestAnswerStatesValuesDoesNotTruncateANumericRange(t *testing.T) {
	t.Setenv(answerStatesValuesEnv, "true")
	plan := rangePlan("cdr.network_volume", "cdr.network_volume")
	resp := rangeResponse(plan, "1730127963000", "9730127963999")
	answer, ok := buildResultAnswer(hybridQueryRequest{RecordType: "cdr"}, resp, nil)
	if !ok {
		t.Fatal("a numeric range produced no answer")
	}
	for _, want := range []string{"1730127963000", "9730127963999"} {
		if !strings.Contains(answer.Headline, want) {
			t.Fatalf("headline %q truncated %s", answer.Headline, want)
		}
	}
}

// MIN and MAX of DIFFERENT fields is not a range. Describing it as one would
// assert a relationship between two quantities that were never compared.
func TestAnswerStatesValuesRejectsAMixedFieldRange(t *testing.T) {
	t.Setenv(answerStatesValuesEnv, "true")
	plan := rangePlan("cdr.event_time", "cdr.duration_seconds")
	resp := rangeResponse(plan, "2026-04-01T19:00:02Z", "1798")
	if _, ok := buildResultAnswer(hybridQueryRequest{RecordType: "cdr"}, resp, nil); ok {
		t.Fatal("MIN and MAX of different fields was described as a range")
	}
}

// ACC-02. "Show the breakdown of HTTP status codes" stated the top THREE of six
// and the analyst was never told about status 500. A complete breakdown of a
// handful of groups is stated in full.
func TestAnswerStatesValuesStatesEveryGroupOfASmallBreakdown(t *testing.T) {
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		GroupFields:     []string{"access_log.status"},
	}
	resp := hybridQueryResponse{Records: map[string]any{
		"plan": plan, "complete": true,
		"field_catalog": []FieldDescriptorV1{{FieldID: "access_log.status", NormalizedName: "status", DisplayName: "Status"}},
		"source_native_results": []map[string]any{
			{"status": "200", "m1": 697}, {"status": "201", "m1": 89},
			{"status": "400", "m1": 64}, {"status": "404", "m1": 58},
			{"status": "302", "m1": 46}, {"status": "500", "m1": 46},
		},
	}}

	t.Setenv(answerStatesValuesEnv, "")
	before, ok := buildResultAnswer(hybridQueryRequest{RecordType: "access_log"}, resp, nil)
	if !ok {
		t.Fatal("breakdown produced no answer with the switch off")
	}
	if strings.Contains(before.Headline, "500") {
		t.Fatalf("switch off should still truncate; got %q", before.Headline)
	}

	t.Setenv(answerStatesValuesEnv, "true")
	after, ok := buildResultAnswer(hybridQueryRequest{RecordType: "access_log"}, resp, nil)
	if !ok {
		t.Fatal("breakdown produced no answer with the switch on")
	}
	for _, want := range []string{"200", "201", "400", "404", "302", "500", "697", "46"} {
		if !strings.Contains(after.Headline, want) {
			t.Fatalf("headline %q omits %s", after.Headline, want)
		}
	}
}

// A breakdown too large to read is still summarised. A sentence listing a
// hundred values is not an answer; it is the table.
func TestAnswerStatesValuesSummarisesALargeBreakdown(t *testing.T) {
	t.Setenv(answerStatesValuesEnv, "true")
	results := make([]map[string]any, 0, answerBreakdownFullLimit+4)
	for i := 0; i < answerBreakdownFullLimit+4; i++ {
		results = append(results, map[string]any{"status": string(rune('A'+i)) + "-code", "m1": 100 - i})
	}
	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		GroupFields:     []string{"access_log.status"},
	}
	resp := hybridQueryResponse{Records: map[string]any{
		"plan": plan, "complete": true,
		"field_catalog":         []FieldDescriptorV1{{FieldID: "access_log.status", NormalizedName: "status", DisplayName: "Status"}},
		"source_native_results": results,
	}}
	answer, ok := buildResultAnswer(hybridQueryRequest{RecordType: "access_log"}, resp, nil)
	if !ok {
		t.Fatal("large breakdown produced no answer")
	}
	if strings.Contains(answer.Headline, "In full") {
		t.Fatalf("a breakdown beyond the limit must not claim to be full: %q", answer.Headline)
	}
}
