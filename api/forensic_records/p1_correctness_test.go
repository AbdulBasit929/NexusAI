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

// ---------------------------------------------------------------------------
// WI-1 — every extracted literal is an obligation (closes D1).
//
// D1 was: the compiler extracted a target identifier, a date range and payload
// filters, then compiled a plan that referenced none of them, so "how many
// calls did <number> make" and even a NONEXISTENT number returned a count of
// every row in the case. The request-level half (target and dates reaching the
// SQL) was closed in source_native_sql_executor.go; these cover the remaining
// half — frame.Filters reaching the plan, and a refusal when a literal fails
// to bind.
//
// Expected values are written by hand from the question text.
// ---------------------------------------------------------------------------

func p1SubscriberCatalog(t *testing.T) []FieldDescriptorV1 {
	t.Helper()
	rows := []map[string]any{}
	for i, status := range []string{"ACTIVE", "SUSPENDED", "ACTIVE"} {
		rows = append(rows, map[string]any{
			"record_type": "subscriber", "source_file": "subs.csv", "row_number": i + 1,
			"raw_payload": map[string]any{
				"subscriber_id": "PK-SUB-SYN-ALPHA", "status": status, "city": "Lahore",
			},
		})
	}
	return buildSourceNativeFieldCatalog(hybridQueryRequest{TenantID: "t", CollectionID: "c"}, rows)
}

// An extracted payload filter must reach the plan. Before WI-1 nothing consumed
// frame.Filters, so this compiled to an unfiltered count of every CDR row.
func TestWI1ExtractedPayloadFilterReachesThePlan(t *testing.T) {
	catalog := p1CDRCatalog(t)
	question := "How many CDR records where call_type is SMS?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	if len(frame.Filters) != 1 {
		t.Fatalf("precondition: expected one extracted filter, got %+v", frame.Filters)
	}
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
	if plan == nil {
		t.Fatalf("expected a plan, got nil (%s)", reason)
	}
	if len(plan.Filters) != 1 {
		t.Fatalf("extracted filter was dropped: plan.Filters=%+v (%s)", plan.Filters, reason)
	}
	got := plan.Filters[0]
	if got.Op != "EQ" || got.Value != "SMS" {
		t.Fatalf("filter = %+v; want EQ SMS", got)
	}
	if name := sourceNativeFieldMap(catalog)[got.FieldID].NormalizedName; name != "call_type" {
		t.Fatalf("filter bound to %q, want call_type", name)
	}
}

// A literal naming a field that is not in the authorized catalog must refuse.
// Counting the whole case instead is the D1 failure and is never acceptable.
func TestWI1UnboundFilterRefusesInsteadOfCounting(t *testing.T) {
	catalog := p1CDRCatalog(t)
	question := "How many CDR records where blood_type is ONegative?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	if len(frame.Filters) != 1 {
		t.Fatalf("precondition: expected one extracted filter, got %+v", frame.Filters)
	}
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
	if plan != nil {
		t.Fatalf("unbound literal must not compile a plan; got %+v", plan)
	}
	if !strings.HasPrefix(reason, semanticConstraintUnboundPrefix) {
		t.Fatalf("reason = %q; want a %s reason", reason, semanticConstraintUnboundPrefix)
	}
	// The refusal must name the specific literal so the analyst can act on it.
	if !strings.Contains(reason, "blood_type") || !strings.Contains(reason, "ONegative") {
		t.Fatalf("reason %q does not name the unbound field and literal", reason)
	}
}

// An identifier in the question is an obligation. If nothing carries it, the
// compiler must refuse rather than answer a broader question than was asked.
func TestWI1IdentifierIsAnObligation(t *testing.T) {
	question := "How many calls did 923001110001 make?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	if len(frame.Identifiers) != 1 {
		t.Fatalf("precondition: expected one identifier, got %+v", frame.Identifiers)
	}
	plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1,
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}}

	// Nothing binds the number: refuse, and name it.
	unbound := deterministicUnboundConstraints(hybridQueryRequest{Query: question}, frame, plan)
	if len(unbound) != 1 || !strings.Contains(unbound[0], "923001110001") {
		t.Fatalf("unbound = %v; want the identifier named", unbound)
	}
	// The request-level target constraint binds it, including when the analyst
	// wrote it in a different format than the records store.
	for _, target := range []string{"923001110001", "+92 300 1110001"} {
		bound := deterministicUnboundConstraints(
			hybridQueryRequest{Query: question, Target: target}, frame, plan)
		if len(bound) != 0 {
			t.Fatalf("target %q must satisfy the obligation, got %v", target, bound)
		}
	}
}

// NEG-01. A nonexistent number is still an obligation: the answer is 0, never
// a count of the 12,912 rows in the case.
func TestWI1NonexistentIdentifierNeverWidensToTheWholeCase(t *testing.T) {
	question := "How many calls did 03999999999 make?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1,
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}}
	if unbound := deterministicUnboundConstraints(hybridQueryRequest{Query: question}, frame, plan); len(unbound) == 0 {
		t.Fatal("an unbound nonexistent identifier must refuse, not count the case")
	}
	// Once bound, the plan stands and the executor returns 0 rows.
	if unbound := deterministicUnboundConstraints(
		hybridQueryRequest{Query: question, Target: "03999999999"}, frame, plan); len(unbound) != 0 {
		t.Fatalf("bound target must satisfy the obligation, got %v", unbound)
	}
}

// CDR-14. "from August 2026" is an obligation exactly like an identifier.
func TestWI1DateRangeIsAnObligation(t *testing.T) {
	question := "How many call records are from August 2026?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	if frame.TimeIntent.From != "2026-08-01T00:00:00Z" || frame.TimeIntent.To != "2026-09-01T00:00:00Z" {
		t.Fatalf("precondition: time intent = %+v", frame.TimeIntent)
	}
	plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1,
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}}
	if unbound := deterministicUnboundConstraints(hybridQueryRequest{Query: question}, frame, plan); len(unbound) == 0 {
		t.Fatal("an unbound date range must refuse, not count every month")
	}
	bound := deterministicUnboundConstraints(hybridQueryRequest{
		Query: question, DateFrom: "2026-08-01T00:00:00Z", DateTo: "2026-09-01T00:00:00Z",
	}, frame, plan)
	if len(bound) != 0 {
		t.Fatalf("bound date range must satisfy the obligation, got %v", bound)
	}
}

// A question with no literals has no obligations and must still compile.
func TestWI1UnconstrainedQuestionStillCompiles(t *testing.T) {
	catalog := p1CDRCatalog(t)
	question := "How many CDR records do we have in this case?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
	if plan == nil {
		t.Fatalf("a plain count must compile; reason=%s", reason)
	}
	if len(plan.Filters) != 0 {
		t.Fatalf("a plain count must not invent filters: %+v", plan.Filters)
	}
	if unbound := deterministicUnboundConstraints(hybridQueryRequest{Query: question}, frame, plan); len(unbound) != 0 {
		t.Fatalf("no literals means no obligations, got %v", unbound)
	}
}

// A filter must never be bound to a field by embedding similarity. Binding the
// wrong field silently answers a different question, which is the failure mode
// this work item exists to remove, and the cause of D2. The catalog here holds
// several plausible fields and every one of them is scored maximally similar.
func TestWI1FilterBindingIgnoresEmbeddingSignal(t *testing.T) {
	catalog := p1CDRCatalog(t)
	question := "How many CDR records where call_type is SMS?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	similar := map[string]float64{}
	for _, field := range catalog {
		similar[field.FieldID] = 0.99 // every field looks maximally similar
	}
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, similar)
	if plan == nil || len(plan.Filters) != 1 {
		t.Fatalf("expected one bound filter; reason=%s plan=%+v", reason, plan)
	}
	if name := sourceNativeFieldMap(catalog)[plan.Filters[0].FieldID].NormalizedName; name != "call_type" {
		t.Fatalf("filter bound to %q, want call_type; embedding similarity must not decide", name)
	}
	// Saturated similarity must not manufacture a grouping either (D2).
	if len(plan.GroupFields) != 0 {
		t.Fatalf("embedding similarity produced a grouping: %v", plan.GroupFields)
	}
}

// Filters extracted from a question are discarded before the compiler ever
// sees them unless the request routed to the canonical_records template:
// applyCanonicalQueryHints (query.go) returns early for every other template,
// so facts.Filters is empty for families that route elsewhere. That is the
// same "extracted then silently dropped" defect class as D1, one layer up, and
// it is why "how many subscribers are active" counts every subscriber.
//
// Fixing it means changing template routing in query.go, which WI-1 does not
// own and the anti-template rule forbids extending. This test pins the current
// behaviour so the gap is visible and cannot regress unnoticed; it must be
// inverted when the routing is fixed.
func TestWI1KnownGapPayloadFiltersLostOutsideCanonicalRecords(t *testing.T) {
	question := "How many subscribers where status is ACTIVE?"
	if raw := extractCanonicalPayloadFilters(question); len(raw) != 1 || raw[0].Field != "status" {
		t.Fatalf("precondition: the filter is extractable, got %+v", raw)
	}
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	if len(frame.Filters) != 0 {
		t.Fatalf("KNOWN GAP CLOSED — filters now survive routing (%+v). "+
			"Invert this test and bind them.", frame.Filters)
	}
}

// ---------------------------------------------------------------------------
// WI-2 — Fact Packet v2: facts are answers, not plumbing.
//
// The 2026-09-18 baseline found that a packet's facts were "Template:",
// "Route:", "Planner Confidence" and "Display Rows". Neither the deterministic
// text nor the LLM could state an answer from that, validateNarrative
// correctly rejected the paraphrase, and the analyst was left with nothing.
// The answer was stated in the analyst's text for only 5 of 20 correct or
// partial factual answers.
//
// Expected values are written by hand from the question and the result values.
// ---------------------------------------------------------------------------

// The exact plumbing metrics the baseline observed being presented as facts.
func wi2PlumbingMetrics() []map[string]any {
	return []map[string]any{
		{"label": "Template", "value": "canonical_records"},
		{"label": "Route", "value": "records_sql"},
		{"label": "Planner Confidence", "value": 1},
		{"label": "Display Rows", "value": 1},
		{"label": "Records Row Count", "value": 1},
	}
}

// The executive answer the product actually produced for a count question.
const wi2AlgebraJargon = "Executed bounded source-native typed algebra over 8642 " +
	"authorized source rows and returned 1 deterministic results with contribution lineage."

func wi2CountPacket(t *testing.T) FactPacketV1 {
	t.Helper()
	plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1,
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}}
	req := hybridQueryRequest{Query: "How many CDR records do we have in this case?", RecordType: "cdr"}
	resp := hybridQueryResponse{
		Template: "canonical_records",
		Records:  map[string]any{"plan": plan, "complete": true, "source_native_results": []map[string]any{{"m1": 8642}}},
		Answer:   map[string]any{},
	}
	return buildFactPacket(req, resp, map[string]any{
		"executive_answer": wi2AlgebraJargon,
		"metrics":          wi2PlumbingMetrics(),
	})
}

func wi2BreakdownPacket(t *testing.T) FactPacketV1 {
	t.Helper()
	catalog := p1CDRCatalog(t)
	callType := ""
	for _, field := range catalog {
		if field.NormalizedName == "call_type" {
			callType = field.FieldID
		}
	}
	if callType == "" {
		t.Fatal("precondition: the fixture catalog has no call_type field")
	}
	plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1,
		GroupFields: []string{callType},
		Measures:    []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		Sort:        []SourceNativeSortV1{{Target: "m1", Direction: "DESC"}}}
	req := hybridQueryRequest{Query: "Show the call type breakdown", RecordType: "cdr"}
	resp := hybridQueryResponse{
		Template: "canonical_records",
		Records: map[string]any{"plan": plan, "complete": true, "field_catalog": catalog,
			"source_native_results": []map[string]any{
				{"m1": 5863, "call_type": "GPRS"},
				{"m1": 1108, "call_type": "SMS"},
				{"m1": 739, "call_type": "VOLTE"},
			}},
		Answer: map[string]any{},
	}
	return buildFactPacket(req, resp, map[string]any{
		"executive_answer": wi2AlgebraJargon,
		"metrics":          wi2PlumbingMetrics(),
	})
}

func wi2DocumentPacket(t *testing.T) FactPacketV1 {
	t.Helper()
	req := hybridQueryRequest{Query: "What does the case notes document say about plate ABC-123?"}
	resp := hybridQueryResponse{
		Template: "document_search",
		Records:  map[string]any{},
		Answer:   map[string]any{},
		Evidence: map[string]any{"results": []map[string]any{{
			"preview": "Vehicle ABC-123 was logged at the Gulberg checkpoint on 14 July 2026.",
			"metadata": map[string]any{
				"source_file": "case-notes.pdf", "evidence_id": "ev-1", "version_id": "v-1",
				"citation_locator": map[string]any{"page": 3, "char_start": 120, "char_end": 188},
			},
		}}},
	}
	return buildFactPacket(req, resp, map[string]any{
		"executive_answer": wi2AlgebraJargon,
		"metrics":          wi2PlumbingMetrics(),
		"provenance": []map[string]any{{
			"evidence_id": "ev-1", "version_id": "v-1", "source_file": "case-notes.pdf",
			// The citation must resolve to the SAME locator as the passage;
			// documentFactCitationRefs deliberately refuses to attach a
			// citation that points somewhere else in the document.
			"citation_locator": map[string]any{"page": 3, "char_start": 120, "char_end": 188},
		}},
	})
}

// No fact in any packet may describe the machinery. This is the defect that
// affected every question in the baseline.
func TestWI2NoPacketPresentsPlumbingAsAFact(t *testing.T) {
	packets := map[string]FactPacketV1{
		"count":     wi2CountPacket(t),
		"breakdown": wi2BreakdownPacket(t),
		"document":  wi2DocumentPacket(t),
	}
	for name, packet := range packets {
		if len(packet.Facts) == 0 {
			t.Fatalf("%s: packet has no facts at all", name)
		}
		for _, fact := range packet.Facts {
			if factPacketPlumbingText(fact.Text) {
				t.Errorf("%s: %s is plumbing presented as a fact: %q", name, fact.FactID, fact.Text)
			}
			for label := range factPacketPlumbingMetric {
				if strings.HasPrefix(fact.Text, label+":") {
					t.Errorf("%s: %s carries the plumbing metric %q: %q", name, fact.FactID, label, fact.Text)
				}
			}
		}
	}
}

// The count packet must state 8,642, and must carry it as a value and not only
// inside a sentence.
func TestWI2CountPacketStatesTheAnswer(t *testing.T) {
	packet := wi2CountPacket(t)
	first := packet.Facts[0]
	if first.Kind != "direct_answer" {
		t.Fatalf("first fact is %q, want direct_answer: %+v", first.Kind, packet.Facts)
	}
	if want := "There are 8,642 CDR records in this case."; first.Text != want {
		t.Fatalf("direct answer = %q, want %q", first.Text, want)
	}
	if strings.Contains(strings.ToLower(first.Text), "algebra") {
		t.Fatal("the algebra jargon reached the direct answer")
	}
	value, ok := answerNumber(first.Value)
	if !ok || value != 8642 {
		t.Fatalf("direct answer value = %v (ok=%v), want 8642 carried as a value", first.Value, ok)
	}
}

// A breakdown states the total and the largest categories, hand-computed:
// 5863 + 1108 + 739 = 7710.
func TestWI2BreakdownPacketStatesTotalsAndCategories(t *testing.T) {
	packet := wi2BreakdownPacket(t)
	first := packet.Facts[0]
	if first.Kind != "direct_answer" {
		t.Fatalf("first fact is %q, want direct_answer", first.Kind)
	}
	for _, want := range []string{"7,710", "GPRS", "5,863"} {
		if !strings.Contains(first.Text, want) {
			t.Fatalf("direct answer %q omits %q", first.Text, want)
		}
	}
	findings := 0
	for _, fact := range packet.Facts {
		if fact.Kind == "result_finding" {
			findings++
		}
	}
	if findings != 3 {
		t.Fatalf("got %d result findings, want one per call type", findings)
	}
}

// A document answer's facts are the cited passages themselves.
func TestWI2DocumentPacketCarriesThePassageAsAFact(t *testing.T) {
	packet := wi2DocumentPacket(t)
	found := false
	for _, fact := range packet.Facts {
		if fact.Kind == "source_passage" && strings.Contains(fact.Text, "ABC-123") {
			found = true
			if len(fact.CitationIDs) == 0 {
				t.Errorf("passage fact %s carries no citation", fact.FactID)
			}
		}
	}
	if !found {
		t.Fatalf("no cited passage among the facts: %+v", packet.Facts)
	}
}

// Plumbing still exists for engineers, but only in the trace, and the trace is
// never shown to the narrator.
func TestWI2PlumbingLivesInTheTraceAndNeverReachesNarration(t *testing.T) {
	packet := wi2CountPacket(t)
	traceMetrics := mapsFromAny(packet.Trace["metrics"])
	if len(traceMetrics) != len(wi2PlumbingMetrics()) {
		t.Fatalf("trace carries %d metrics, want %d", len(traceMetrics), len(wi2PlumbingMetrics()))
	}
	for _, metric := range packet.Metrics {
		if factPacketPlumbingMetric[fpFirstString(metric, "label", "name")] {
			t.Errorf("plumbing metric left on the packet: %+v", metric)
		}
	}
	if packet.narrationPacket().Trace != nil {
		t.Fatal("the trace must be stripped before the packet is shown to a model")
	}
	// Stripping the trace must not remove the answer.
	if len(packet.narrationPacket().Facts) != len(packet.Facts) {
		t.Fatal("stripping the trace changed the facts")
	}
}

// If narration is rejected or never runs, the analyst must still get the
// answer. This is the property that makes narration genuinely optional.
func TestWI2NarrationRejectionStillLeavesAUsableAnswer(t *testing.T) {
	for name, packet := range map[string]FactPacketV1{
		"count":     wi2CountPacket(t),
		"breakdown": wi2BreakdownPacket(t),
	} {
		fallback := deterministicNarrativeFallback(packet, "GROUNDING_REJECTED")
		if fallback.DirectAnswer != packet.Facts[0].Text {
			t.Errorf("%s: fallback answer %q is not the packet's direct answer %q",
				name, fallback.DirectAnswer, packet.Facts[0].Text)
		}
		if factPacketPlumbingText(fallback.DirectAnswer) {
			t.Errorf("%s: fallback answer is plumbing: %q", name, fallback.DirectAnswer)
		}
		if len(fallback.KeyFindings) == 0 {
			t.Errorf("%s: fallback carries no findings", name)
		}
	}
	// The count case states the number with no model involved at all.
	if answer := deterministicNarrativeFallback(wi2CountPacket(t), "GROUNDING_REJECTED"); !strings.Contains(answer.DirectAnswer, "8,642") {
		t.Fatalf("deterministic answer omits the count: %q", answer.DirectAnswer)
	}
}

// A zero result must say so plainly rather than falling back to jargon.
func TestWI2ZeroResultStatesTheAbsence(t *testing.T) {
	plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1,
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}}
	req := hybridQueryRequest{Query: "How many calls did 03999999999 make?", RecordType: "cdr", Target: "03999999999"}
	resp := hybridQueryResponse{
		Template: "canonical_records",
		Records:  map[string]any{"plan": plan, "complete": true, "source_native_results": []map[string]any{{"m1": 0}}},
		Answer:   map[string]any{},
	}
	packet := buildFactPacket(req, resp, map[string]any{
		"executive_answer": wi2AlgebraJargon, "metrics": wi2PlumbingMetrics(),
	})
	want := "There are no CDR records involving 03999999999 in this case."
	if packet.Facts[0].Text != want {
		t.Fatalf("zero-result answer = %q, want %q", packet.Facts[0].Text, want)
	}
}

// Rejecting the plumbing sentence must never turn a non-empty result set into
// "No matching evidence was found." A plan with no single computed headline
// still has to state something true about the evidence.
func TestWI2ResultsWithoutAHeadlineStillStateSomethingTrue(t *testing.T) {
	req := hybridQueryRequest{Query: "List the CDR records for this case", RecordType: "cdr"}
	resp := hybridQueryResponse{Template: "canonical_records", Records: map[string]any{}, Answer: map[string]any{}}
	rows := []map[string]any{
		{"msisdn": "923001110001", "call_type": "SMS"},
		{"msisdn": "923001110002", "call_type": "VOICE"},
	}
	packet := buildFactPacket(req, resp, map[string]any{
		"executive_answer": wi2AlgebraJargon,
		"metrics":          wi2PlumbingMetrics(),
		"data_grid":        map[string]any{"rows": rows, "count": len(rows)},
	})
	if len(packet.Facts) == 0 {
		t.Fatal("a non-empty result set produced no facts at all")
	}
	answer := deterministicNarrativeFallback(packet, "GROUNDING_REJECTED").DirectAnswer
	if strings.Contains(strings.ToLower(answer), "no matching evidence") {
		t.Fatalf("two returned rows were reported as no evidence: %q", answer)
	}
	if !strings.Contains(answer, "2") {
		t.Fatalf("answer %q does not state how many records matched", answer)
	}
	if factPacketPlumbingText(answer) {
		t.Fatalf("answer is plumbing: %q", answer)
	}
}

// ---------------------------------------------------------------------------
// WI-3 — family guard (D4) and the measure-hint defect.
//
// D4: IPDR, tower and access-log questions were answered from CDR and ingest
// operations. The guard removes foreign-family operations BEFORE ranking,
// because ranking cannot repair a pool that contains the wrong families.
//
// It is built on extractCanonicalRecordType, not frame.FamilyHint. Measured
// 2026-09-21: wherever the two disagree the record type is right. "Which
// domain was accessed most often" hints access_security_logs because of the
// word "accessed" (it is an IPDR question) and "which IP address made the most
// requests" hints nothing at all. A guard built on the weaker signal would
// reject correct candidates and lock in wrong ones.
//
// Expected values are written by hand from the question text.
// ---------------------------------------------------------------------------

func wi3TransactionCatalog(t *testing.T) []FieldDescriptorV1 {
	t.Helper()
	rows := []map[string]any{}
	for i, amount := range []string{"75000", "31000", "12300", "11400"} {
		rows = append(rows, map[string]any{
			"record_type": "transaction", "source_file": "txn.csv", "row_number": i + 1,
			"raw_payload": map[string]any{
				"account": "ACCT-778812", "transaction_amount": amount, "currency": "PKR",
			},
		})
	}
	return buildSourceNativeFieldCatalog(hybridQueryRequest{TenantID: "t", CollectionID: "c"}, rows)
}

// No question may be served by an operation from a family it did not ask about.
func TestWI3FamilyGuardRemovesForeignFamilyOperations(t *testing.T) {
	cases := map[string]string{
		"Which domain was accessed most often?":                      "network_ipdr",
		"Which subscriber has the most internet sessions?":           "network_ipdr",
		"How many cell towers are in the case?":                      "tower_location",
		"Show the breakdown of HTTP status codes in the access logs": "access_security_logs",
		"Which IP address made the most requests?":                   "access_security_logs",
		"How many requests failed with a server error (status 500)?": "access_security_logs",
		"What was the largest transaction?":                          "financial_transactions",
		"How many CDR records do we have in this case?":              "communications_cdr",
	}
	for question, wantFamily := range cases {
		req := hybridQueryRequest{Query: question, TenantID: "t", CollectionID: "c"}
		kept, family := filterOperationsByQuestionFamily(question, semanticOperationCandidates(req))
		if family != wantFamily {
			t.Errorf("%q: guard family = %q, want %q", question, family, wantFamily)
			continue
		}
		for _, candidate := range kept {
			if candidate.FamilyID != wantFamily {
				t.Errorf("%q: %s survived from family %q, want only %q",
					question, candidate.OperationID, candidate.FamilyID, wantFamily)
			}
		}
	}
}

// ACC-03 is the clearest D4 case: it previously ranked a CDR location
// operation first for an access-log question.
func TestWI3AccessLogQuestionNeverReachesACDROperation(t *testing.T) {
	question := "Which IP address made the most requests?"
	req := hybridQueryRequest{Query: question, TenantID: "t", CollectionID: "c"}
	unfiltered := semanticOperationCandidates(req)
	frame := extractSemanticFrame(req)
	before, _ := rankSemanticOperationsForFrame(question, frame, unfiltered)
	if len(before) == 0 || before[0].FamilyID != "communications_cdr" {
		t.Skipf("precondition changed: unfiltered top is %+v", before[:min(1, len(before))])
	}
	rankedAll, scoresAll := rankSemanticOperationsForFrame(question, frame, unfiltered)
	after, _, _ := filterRankedByQuestionFamily(question, rankedAll, scoresAll)
	for _, candidate := range after {
		if candidate.FamilyID == "communications_cdr" {
			t.Fatalf("a CDR operation survived an access-log question: %s", candidate.OperationID)
		}
	}
}

// A question that names no structured record type must not be filtered at all.
// Precision over recall: an unknown family is not a reason to discard the pool.
func TestWI3GuardDoesNotFilterWhenNoRecordTypeIsNamed(t *testing.T) {
	question := "Give me an overview of this case"
	req := hybridQueryRequest{Query: question, TenantID: "t", CollectionID: "c"}
	all := semanticOperationCandidates(req)
	kept, family := filterOperationsByQuestionFamily(question, all)
	if family != "" {
		t.Fatalf("family = %q, want empty for a question naming no record type", family)
	}
	if len(kept) != len(all) {
		t.Fatalf("pool was filtered from %d to %d with no family named", len(all), len(kept))
	}
}

// TXN-03. "The most transactions" counts rows. Ranking accounts by amount
// answers a different question.
func TestWI3MostRecordNounCountsRowsNotAmounts(t *testing.T) {
	catalog := wi3TransactionCatalog(t)
	question := "Which account has the most transactions?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	if frame.Measure != "max" {
		t.Fatalf("precondition: frame.Measure = %q, expected the 'most' -> max mapping", frame.Measure)
	}
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
	if plan == nil {
		t.Fatalf("expected a plan, got nil (%s)", reason)
	}
	if len(plan.Measures) != 1 || plan.Measures[0].Op != "COUNT" {
		t.Fatalf("measure = %+v, want a single COUNT", plan.Measures)
	}
	if plan.Measures[0].FieldID != "" {
		name := sourceNativeFieldMap(catalog)[plan.Measures[0].FieldID].NormalizedName
		t.Fatalf("COUNT was bound to field %q; counting rows takes no field", name)
	}
}

// The same rule must not break the CDR ranking questions that were correct.
func TestWI3CountingSuperlativesStayCounts(t *testing.T) {
	// The claim under test is about the AGGREGATE, not about whether a plan
	// compiles: "the most calls" counts rows and must never become MAX.
	// Whether the ranking itself resolves depends on the catalogue, and S9
	// SHAPE now correctly refuses a ranking whose dimension did not resolve
	// rather than answering it with a plain count.
	for _, question := range []string{
		"Which phone number made the most calls?",
		"Which cell site handled the most calls?",
		"Who did 923001110001 contact most frequently?",
	} {
		frame := extractSemanticFrame(hybridQueryRequest{Query: question})
		if got := sourceNativeMeasureAggregate(frame, question); got != "COUNT" {
			t.Errorf("%q: aggregate = %s, want COUNT", question, got)
		}
	}
	// With the curated catalogue the dimension resolves and the plan compiles.
	catalog := wi5CuratedCatalog(t, "communications_cdr")
	question := "Which phone number made the most calls?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
	if plan == nil {
		t.Fatalf("curated catalogue must resolve the ranking; got nil (%s)", reason)
	}
	if plan.Measures[0].Op != "COUNT" || len(plan.GroupFields) != 1 {
		t.Fatalf("plan = %+v; want COUNT grouped by one field", plan)
	}
}

// S9 SHAPE must refuse a ranking whose dimension did not resolve, rather than
// silently answering it with a plain count — that returns a real number for a
// different question.
func TestWI3UnresolvableRankingIsRefusedNotCounted(t *testing.T) {
	catalog := p1CDRCatalog(t) // inferred catalogue: no curated synonyms
	question := "Which cell site handled the most calls?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
	if plan != nil {
		t.Fatalf("an unresolvable ranking must refuse, got %+v", plan)
	}
	if !strings.HasPrefix(reason, s9ShapeViolation) {
		t.Fatalf("reason = %q; want an S9 shape violation", reason)
	}
}

// TXN-02. "The largest transaction" is a maximum, never a count. With no
// explicit measure field and no declared default measure, refusing is correct;
// answering with a count is the defect.
func TestWI3MagnitudeSuperlativeIsNeverACount(t *testing.T) {
	catalog := wi3TransactionCatalog(t)
	question := "What was the largest transaction?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
	if plan != nil && len(plan.Measures) > 0 && plan.Measures[0].Op == "COUNT" {
		t.Fatalf("'largest' was answered with a COUNT: %+v", plan.Measures)
	}
	if plan == nil && reason != "MEASURE_FIELD_UNRESOLVED" {
		t.Fatalf("refused with %q, want MEASURE_FIELD_UNRESOLVED", reason)
	}
	if aggregate, ok := sourceNativeMagnitudeSuperlative(question); !ok || aggregate != "MAX" {
		t.Fatalf("magnitude superlative = %q ok=%v, want MAX", aggregate, ok)
	}
	if aggregate, _ := sourceNativeMagnitudeSuperlative("What was the shortest call?"); aggregate != "MIN" {
		t.Fatalf("'shortest' = %q, want MIN", aggregate)
	}
}

// The whole-question fallback is gone. Saturated similarity must not be able
// to supply a measure field that the analyst never named — the same defect D2
// was fixed for, one slot along.
func TestWI3MeasureFieldNeverResolvedFromTheWholeQuestion(t *testing.T) {
	catalog := wi3TransactionCatalog(t)
	similar := map[string]float64{}
	for _, field := range catalog {
		similar[field.FieldID] = 0.99
	}
	question := "Which account has the highest value?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	if frame.MeasureFieldHint != "" {
		t.Skipf("precondition changed: a measure hint was extracted (%q)", frame.MeasureFieldHint)
	}
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, similar)
	if plan != nil {
		for _, measure := range plan.Measures {
			if measure.Op != "COUNT" && measure.FieldID != "" {
				name := sourceNativeFieldMap(catalog)[measure.FieldID].NormalizedName
				t.Fatalf("embedding similarity supplied the measure field %q (%s)", name, reason)
			}
		}
	}
}

// An explicit measure hint still resolves: the fix removes the whole-question
// fallback, not the feature. TXN-01 has a real hint ("transaction amount").
func TestWI3ExplicitMeasureHintStillResolves(t *testing.T) {
	catalog := wi3TransactionCatalog(t)
	question := "What is the total transaction amount?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	if frame.MeasureFieldHint == "" {
		t.Fatalf("precondition: expected a measure field hint for %q", question)
	}
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
	if plan == nil {
		t.Fatalf("an explicit hint must still resolve; got nil (%s)", reason)
	}
	measure := plan.Measures[0]
	if measure.Op != "SUM" {
		t.Fatalf("measure op = %s, want SUM", measure.Op)
	}
	if name := sourceNativeFieldMap(catalog)[measure.FieldID].NormalizedName; name != "transaction_amount" {
		t.Fatalf("SUM bound to %q, want transaction_amount", name)
	}
}

// A magnitude superlative must not override an aggregate the question names.
// "The highest average invoice total" ranks BY the average; the superlative is
// the direction, not the aggregate. Caught by an existing ginkgo expectation
// when the magnitude rule was first added unconditionally.
func TestWI3MagnitudeSuperlativeYieldsToANamedAggregate(t *testing.T) {
	cases := map[string]string{
		"Which department has the highest average invoice total?": "AVG",
		"What is the total transaction amount?":                   "SUM",
		"What was the largest transaction?":                       "MAX",
		"Which account has the most transactions?":                "COUNT",
	}
	for question, want := range cases {
		frame := extractSemanticFrame(hybridQueryRequest{Query: question})
		if got := sourceNativeMeasureAggregate(frame, question); got != want {
			t.Errorf("%q: aggregate = %s, want %s (frame.Measure=%q)", question, got, want, frame.Measure)
		}
	}
}

// The guard must not disturb how operations are scored. Filtering the pool
// before ranking changes the BM25 corpus statistics: measured 2026-09-21, the
// same winning operation scored 12.41 instead of 22.19 and its top-two margin
// collapsed from 0.760 to 0.028, which is enough to fail the confidence gate
// and turn a correct confident match into a clarification. The guard is
// therefore applied to the ranked result, and this pins that.
func TestWI3FamilyGuardDoesNotDistortRankingScores(t *testing.T) {
	question := "List camera activity from plate observations."
	req := hybridQueryRequest{TenantID: "t", CollectionID: "c", Query: question}
	frame := extractSemanticFrame(req)
	all := semanticOperationCandidates(req)

	rankedAll, scoresAll := rankSemanticOperationsForFrame(question, frame, all)
	keptRanked, keptScores, family := filterRankedByQuestionFamily(question, rankedAll, scoresAll)
	if family != "anpr_vehicles" || len(keptScores) == 0 {
		t.Fatalf("guard family = %q with %d scores", family, len(keptScores))
	}
	if keptScores[0].TotalScore != scoresAll[0].TotalScore {
		t.Fatalf("post-filter top score %.3f != full-pool top score %.3f",
			keptScores[0].TotalScore, scoresAll[0].TotalScore)
	}
	if _, matched, _ := deterministicRegisteredMatch(frame, keptRanked, keptScores, nil); !matched {
		t.Fatal("a confident in-family match was lost after filtering")
	}
	// Filtering the pool first would have produced a materially lower score.
	preFiltered, _ := filterOperationsByQuestionFamily(question, all)
	_, preScores := rankSemanticOperationsForFrame(question, frame, preFiltered)
	if len(preScores) > 0 && preScores[0].TotalScore >= scoresAll[0].TotalScore {
		t.Skip("BM25 corpus sensitivity no longer reproduces; the guard order no longer matters")
	}
}
