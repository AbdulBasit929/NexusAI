package main

import (
	"strings"
	"testing"
)

// Regressions for defects found by the 2026-09-18 live golden baseline
// (reports/nexusai-tl-audit-20260918/P0-baseline-report.md). Expected values
// are written by hand from the question text, never produced by the helpers.

func p1CDRCatalog(t *testing.T) []FieldDescriptorV1 {
	t.Helper()
	rows := []map[string]any{}
	for i, callType := range []string{"GPRS", "SMS", "CALL", "GPRS", "VOLTE", "SMS"} {
		review := "false"
		if i%2 == 0 {
			review = "true"
		}
		rows = append(rows, map[string]any{
			"record_type": "cdr", "source_file": "cdr-a.csv", "timestamp": "2026-04-0" + string(rune('1'+i)) + "T10:00:00Z", "row_number": i + 1,
			"raw_payload": map[string]any{"MSISDN": "923001110001", "CALL_TYPE": callType, "CALL_DIALED_NUM": "923009998887"},
			"metadata":    map[string]any{"normalized_fields": map[string]any{"manual_review_required": review}},
		})
	}
	catalog := buildSourceNativeFieldCatalog(hybridQueryRequest{TenantID: "t", CollectionID: "c"}, rows)
	if len(catalog) == 0 {
		t.Fatal("expected a non-empty catalog")
	}
	return catalog
}

func TestP1PlainCountNeverGroupsFromQuestionEmbedding(t *testing.T) {
	catalog := p1CDRCatalog(t)
	// Simulate the embedding runtime being warm and scoring every field as
	// highly similar to the whole question — the condition that produced a
	// spurious GROUP BY manual_review_required live.
	similar := map[string]float64{}
	for _, field := range catalog {
		similar[field.FieldID] = 0.99
	}
	for _, question := range []string{
		"How many CDR records do we have in this case?",
		"How many emails are in this case?",
		"count the cdrs",
	} {
		frame := extractSemanticFrame(hybridQueryRequest{Query: question})
		plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, similar)
		if plan == nil {
			continue // declining to compile is acceptable; grouping is not
		}
		if len(plan.GroupFields) != 0 {
			t.Fatalf("%q (%s): plain count must not group, got %v", question, reason, plan.GroupFields)
		}
	}
}

func TestP1ExplicitGroupingStillResolves(t *testing.T) {
	catalog := p1CDRCatalog(t)
	question := "How many records by call_type?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
	if plan == nil || len(plan.GroupFields) != 1 {
		t.Fatalf("explicit 'by call_type' must group; reason=%s plan=%+v", reason, plan)
	}
	if got := sourceNativeFieldMap(catalog)[plan.GroupFields[0]].NormalizedName; got != "call_type" {
		t.Fatalf("grouped by %q, want call_type", got)
	}
}

func TestP1MonthYearDateRange(t *testing.T) {
	cases := []struct{ question, from, to string }{
		{"How many call records are from August 2026?", "2026-08-01T00:00:00Z", "2026-09-01T00:00:00Z"},
		{"calls in dec 2025", "2025-12-01T00:00:00Z", "2026-01-01T00:00:00Z"},
		// A full day expression must stay a single day, not widen to the month.
		{"calls on 10 August 2026", "2026-08-10T00:00:00Z", "2026-08-11T00:00:00Z"},
		{"How many CDR records do we have?", "", ""},
	}
	for _, c := range cases {
		from, to := extractDateRange(c.question)
		if from != c.from || to != c.to {
			t.Errorf("%q: got [%s, %s), want [%s, %s)", c.question, from, to, c.from, c.to)
		}
	}
}

func TestP1RecordTypeSynonymsAndPlurals(t *testing.T) {
	cases := map[string]string{
		"count the cdrs":                                   "cdr",
		"How many calls did 923001110001 make?":            "cdr",
		"Which cell site handled the most calls?":          "cdr",
		"How many subscribers are there?":                  "subscriber",
		"Which subscriber has the most internet sessions?": "ipdr",
		"How many cell towers are in the case?":            "tower_location",
		"How many times was plate LHR-2026 seen?":          "anpr",
		"Show the breakdown of HTTP status codes":          "access_log",
		"What was the largest transaction?":                "transaction",
		"How many emails are in this case?":                "email",
		"How many records do we have?":                     "",
	}
	for question, want := range cases {
		if got := extractCanonicalRecordType(question); got != want {
			t.Errorf("%q: got %q, want %q", question, got, want)
		}
	}
}

func TestP1RequestConstraintsBindTargetAndDates(t *testing.T) {
	where, args := sourceNativeRequestConstraintsSQL(hybridQueryRequest{
		Target: "+92 300 1110001", DateFrom: "2026-08-01T00:00:00Z", DateTo: "2026-09-01T00:00:00Z",
	}, []string{"r.tenant_id=$1", "r.collection_id=$2"}, []any{"t", "c"})
	sql := strings.Join(where, " AND ")
	for _, want := range []string{"r.primary_target=$3", "r.secondary_target=$3", "=$4", "r.timestamp>=$5::timestamptz", "r.timestamp<$6::timestamptz"} {
		if !strings.Contains(sql, want) {
			t.Errorf("missing %q in %s", want, sql)
		}
	}
	if len(args) != 6 || args[3] != "923001110001" {
		t.Fatalf("args = %v; want digits-normalized target as $4", args)
	}
	// Values are always bound, never concatenated into SQL text.
	if strings.Contains(sql, "923001110001") || strings.Contains(sql, "2026-08") {
		t.Fatalf("literal leaked into SQL: %s", sql)
	}
	none, noneArgs := sourceNativeRequestConstraintsSQL(hybridQueryRequest{}, []string{"x"}, []any{"a"})
	if len(none) != 1 || len(noneArgs) != 1 {
		t.Fatalf("no constraints must add nothing, got %v %v", none, noneArgs)
	}
}

func TestP1ResultAnswerStatesTheCount(t *testing.T) {
	plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1, Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}}
	resp := hybridQueryResponse{Records: map[string]any{"plan": plan, "complete": true, "source_native_results": []map[string]any{{"m1": 8642}}}}
	answer, ok := buildResultAnswer(hybridQueryRequest{RecordType: "cdr"}, resp, nil)
	if !ok || answer.Headline != "There are 8,642 CDR records in this case." {
		t.Fatalf("got %q ok=%v", answer.Headline, ok)
	}
	resp.Records["source_native_results"] = []map[string]any{{"m1": 0}}
	answer, _ = buildResultAnswer(hybridQueryRequest{RecordType: "cdr", Target: "03999999999"}, resp, nil)
	if answer.Headline != "There are no CDR records involving 03999999999 in this case." {
		t.Fatalf("zero-with-target headline: %q", answer.Headline)
	}
	answer, _ = buildResultAnswer(hybridQueryRequest{RecordType: "cdr", DateFrom: "2026-08-01T00:00:00Z", DateTo: "2026-09-01T00:00:00Z"}, hybridQueryResponse{Records: map[string]any{"plan": plan, "source_native_results": []map[string]any{{"m1": 2}}}}, nil)
	if answer.Headline != "There are 2 CDR records from 2026-08-01 up to (not including) 2026-09-01 in this case." {
		t.Fatalf("date-scoped headline: %q", answer.Headline)
	}
}

func TestP1ResultAnswerTotalsATemplateBreakdownByCategory(t *testing.T) {
	rows := []map[string]any{
		{"call_type": "GPRS", "direction": "DATA", "event_count": 3142, "first_seen": "2026-04-02T08:22:38Z", "section": "call_type_breakdown"},
		{"call_type": "GPRS", "direction": "INCOMING", "event_count": 1379, "first_seen": "2026-04-01T19:03:36Z", "section": "call_type_breakdown"},
		{"call_type": "GPRS", "direction": "OUTGOING", "event_count": 1342, "first_seen": "2026-04-01T19:00:02Z", "section": "call_type_breakdown"},
		{"call_type": "SMS", "direction": "INCOMING", "event_count": 694, "first_seen": "2026-04-01T19:30:04Z", "section": "call_type_breakdown"},
		{"call_type": "SMS", "direction": "OUTGOING", "event_count": 414, "first_seen": "2026-04-01T19:30:04Z", "section": "call_type_breakdown"},
	}
	answer, ok := buildResultAnswer(hybridQueryRequest{}, hybridQueryResponse{Template: "call_type_breakdown", Answer: map[string]any{}}, rows)
	if !ok {
		t.Fatal("expected an answer")
	}
	// 3142+1379+1342+694+414 = 6971; GPRS 5863, SMS 1108 (hand-computed).
	want := "6,971 CDR records across 2 call type values. Largest: GPRS: 5,863; SMS: 1,108."
	if answer.Headline != want {
		t.Fatalf("got  %q\nwant %q", answer.Headline, want)
	}
}

func TestP1ResultAnswerNeverSumsAcrossCurrencies(t *testing.T) {
	rows := []map[string]any{
		{"currency": "PKR", "transaction_status": "approved", "transaction_count": 3, "total_amount_decimal": "98700.00"},
		{"currency": "USD", "transaction_status": "approved", "transaction_count": 1, "total_amount_decimal": "100.00"},
	}
	answer, _ := buildResultAnswer(hybridQueryRequest{}, hybridQueryResponse{Template: "financial_transaction_summary", Answer: map[string]any{}}, rows)
	for _, finding := range answer.Findings {
		if strings.Contains(finding, "Combined") {
			t.Fatalf("summed across currencies: %q", finding)
		}
	}
	rows[1]["currency"] = "PKR"
	answer, _ = buildResultAnswer(hybridQueryRequest{}, hybridQueryResponse{Template: "financial_transaction_summary", Answer: map[string]any{}}, rows)
	if len(answer.Findings) == 0 || answer.Findings[0] != "Combined total amount decimal: PKR 98,800" {
		t.Fatalf("single-currency total: %v", answer.Findings)
	}
}

func TestP1DerivedTextRelevanceRejectsFunctionWordMatches(t *testing.T) {
	japanese := "Now widely available throughout the archipelago, Japanese cuisines features an array of simply"
	coconut := "especially Japanese coconut sugar, and various aromatic spices."
	identifier := "کال 03001234567 at 1035"
	cases := []struct {
		text, question string
		want           bool
	}{
		{japanese, "Is the number 03001234567 mentioned in any audio?", false},
		{identifier, "Is the number 03001234567 mentioned in any audio?", true},
		{identifier, "Is +92 300 1234567 in any audio?", false}, // different digits
		{coconut, "Which recording mentions coconut sugar and at what time?", true},
		{japanese, "Which recording mentions coconut sugar and at what time?", false},
		{japanese, "Search the audio transcripts for Japanese cuisine", true},
		{"Investigation Workspace", "Find OCR text mentioning Investigation Workspace", true},
		{"Stay Positive. Work Hard. Make it Happen.", "Find OCR text mentioning Investigation Workspace", false},
		{"NexusAl Islamabad Reference REF-2026-02", "Which image contains the text REF-2026-02?", true},
		{"Investigation Workspace Reference REF-2026-01", "Which image contains the text REF-2026-02?", false},
	}
	for _, c := range cases {
		if _, got := derivedTextRelevance(c.text, c.question); got != c.want {
			t.Errorf("%q vs %q: got %v, want %v", c.question, c.text, got, c.want)
		}
	}
}
