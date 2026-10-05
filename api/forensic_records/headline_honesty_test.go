package main

import "testing"

// Built from the 2026-09-29 laptop probe (reports/laptop-speed-20260929/RESULT.md):
// S5 and S7 stated a filtered count as the case total; S8 stated missing data as 0.
func honestyResponse(result map[string]any, catalog []FieldDescriptorV1) hybridQueryResponse {
	return hybridQueryResponse{Records: map[string]any{
		"source_native_results": []map[string]any{result},
		"field_catalog":         catalog,
	}}
}

var honestyCatalog = []FieldDescriptorV1{
	{FieldID: "anpr.plate_number", NormalizedName: "plate_number", DisplayName: "Plate number", Sensitivity: "IDENTIFIER"},
	{FieldID: "ipdr.protocol", NormalizedName: "protocol", DisplayName: "Protocol", Sensitivity: "NONE"},
	{FieldID: "ipdr.bytes_up", NormalizedName: "bytes_up", DisplayName: "Bytes uploaded", Sensitivity: "NONE"},
	{FieldID: "subscriber.cnic", NormalizedName: "cnic", DisplayName: "CNIC", Sensitivity: "PII"},
	{FieldID: "cdr.call_type", NormalizedName: "call_type", DisplayName: "Call type", Sensitivity: "NONE"},
	{FieldID: "anpr_model_observation.plate_text", NormalizedName: "plate_text", Sensitivity: "IDENTIFIER", RedactionState: "FILTER_ONLY"},
}

func honestyCountPlan(filters ...SourceNativeFilterV1) *SourceNativePlanV1 {
	return &SourceNativePlanV1{ContractVersion: "forensics.source-native-plan/v1", Filters: filters,
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}}
}

func TestHeadlineStatesPlanFilters(t *testing.T) {
	anpr := hybridQueryRequest{RecordType: "anpr"}
	ipdr := hybridQueryRequest{RecordType: "ipdr"}
	headline := func(req hybridQueryRequest, value any, plan *SourceNativePlanV1) string {
		t.Helper()
		answer, ok := sourceNativeResultAnswer(req, honestyResponse(map[string]any{"m1": value}, honestyCatalog), plan)
		if !ok {
			t.Fatalf("no answer") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		return answer.Headline
	}
	plate := SourceNativeFilterV1{FieldID: "anpr.plate_number", Op: "EQ", Value: "LHR-2026"}
	https := SourceNativeFilterV1{FieldID: "ipdr.protocol", Op: "EQ", Value: "HTTPS"}

	t.Run("switch off keeps today's sentence", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(headlineStatesFiltersEnv, "")
		if got := headline(anpr, int64(87), honestyCountPlan(plate)); got != "There are 87 ANPR sightings in this case." {
			t.Fatalf("got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Setenv(headlineStatesFiltersEnv, "true")
	for _, tc := range []struct {
		name string
		req  hybridQueryRequest
		n    int64
		plan *SourceNativePlanV1
		want string
	}{
		{"S5: the plate filter is stated", anpr, 87, honestyCountPlan(plate), "There are 87 ANPR sightings with plate number LHR-2026 in this case."},
		{"S7: the protocol filter is stated", ipdr, 644, honestyCountPlan(https), "There are 644 IPDR sessions with protocol HTTPS in this case."},
		{"a filter equal to the target is not said twice", hybridQueryRequest{RecordType: "anpr", Target: "lhr 2026"}, 87, honestyCountPlan(plate), "There are 87 ANPR sightings involving lhr 2026 in this case."},
		{"a PII value is never repeated", hybridQueryRequest{RecordType: "subscriber"}, 1, honestyCountPlan(SourceNativeFilterV1{FieldID: "subscriber.cnic", Op: "EQ", Value: "3520212345671"}), "There is 1 subscriber record with CNIC matching the value you gave in this case."},
		{"IN lists its values", hybridQueryRequest{RecordType: "cdr"}, 12, honestyCountPlan(SourceNativeFilterV1{FieldID: "cdr.call_type", Op: "IN", Values: []string{"SMS", "CALL"}}), "There are 12 CDR records with call type SMS, CALL in this case."},
		{"a null test is stated", ipdr, 0, honestyCountPlan(SourceNativeFilterV1{FieldID: "ipdr.bytes_up", Op: "IS_NULL"}), "There are no IPDR sessions with no bytes uploaded in this case."},
		{"a range bound is left to the range wording", ipdr, 5, honestyCountPlan(SourceNativeFilterV1{FieldID: "ipdr.bytes_up", Op: "GT", Value: "10"}), "There are 5 IPDR sessions in this case."},
	} {
		t.Run(tc.name, func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			if got := headline(tc.req, tc.n, tc.plan); got != tc.want {
				t.Fatalf("got  %q\nwant %q", got, tc.want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		})
	}

	// Measured on the first arm: each of these said the same constraint twice.
	t.Run("said once, not twice", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		resp := honestyResponse(nil, append(honestyCatalog,
			FieldDescriptorV1{FieldID: "cdr.event_time", NormalizedName: "event_time", DisplayName: "Event time", EffectiveType: "TIMESTAMP"},
			FieldDescriptorV1{FieldID: "subscriber.status", NormalizedName: "status", DisplayName: "Account status"}))
		dated := hybridQueryRequest{RecordType: "cdr", DateFrom: "2026-08-01T00:00:00Z", DateTo: "2026-09-01T00:00:00Z"}
		between := SourceNativeFilterV1{FieldID: "cdr.event_time", Op: "BETWEEN", Values: []string{"2026-08-01T00:00:00Z", "2026-09-01T00:00:00Z"}}
		if got := planFilterQualifiers(dated, resp, honestyCountPlan(between)); got != "" {
			t.Fatalf("CDR-14: the date range is already stated, got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if got := planFilterQualifiers(hybridQueryRequest{RecordType: "cdr"}, resp, honestyCountPlan(between)); got == "" {
			t.Fatal("without a stated date range the BETWEEN must still be named") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		contains := SourceNativeFilterV1{FieldID: "anpr.plate_number", Op: "CONTAINS", Value: "QQQ-9999"}
		if got := planFilterQualifiers(hybridQueryRequest{RecordType: "anpr", Target: "QQQ-9999"}, resp, honestyCountPlan(contains)); got != "" {
			t.Fatalf("H13: the target is already stated, got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		groupedPlan := honestyCountPlan(SourceNativeFilterV1{FieldID: "subscriber.status", Op: "EQ", Value: "ACTIVE"})
		groupedPlan.GroupFields = []string{"subscriber.status"}
		if got := planFilterQualifiers(hybridQueryRequest{RecordType: "subscriber"}, resp, groupedPlan); got != "" {
			t.Fatalf("H8: the breakdown already lists the grouped field, got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("a filter-only field is left to its own sentence", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		got := planFilterQualifiers(anpr, honestyResponse(nil, honestyCatalog), honestyCountPlan(SourceNativeFilterV1{FieldID: "anpr_model_observation.plate_text", Op: "EQ", Value: "MN1367"}))
		if got != "" {
			t.Fatalf("got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}

func TestEmptyAggregateIsNotZero(t *testing.T) {
	ipdr := hybridQueryRequest{RecordType: "ipdr"}
	sumPlan := &SourceNativePlanV1{ContractVersion: "forensics.source-native-plan/v1",
		Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: "SUM", FieldID: "ipdr.bytes_up"}}}
	headline := func(result map[string]any, plan *SourceNativePlanV1) string {
		t.Helper()
		answer, ok := sourceNativeResultAnswer(ipdr, honestyResponse(result, honestyCatalog), plan)
		if !ok {
			t.Fatalf("no answer") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		return answer.Headline
	}
	s8 := map[string]any{"m1": nil, measureDenominatorKey("m1"): 0}

	t.Run("switch off keeps today's sentence", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(emptyAggregateHonestEnv, "")
		if got := headline(s8, sumPlan); got != "The total bytes uploaded across IPDR sessions is 0." {
			t.Fatalf("got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Setenv(emptyAggregateHonestEnv, "true")
	t.Run("S8: no values means no total", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		if got := headline(s8, sumPlan); got != "No bytes uploaded values are recorded in these IPDR sessions, so there is no total to state." {
			t.Fatalf("got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
	t.Run("a NULL aggregate without a count is still empty", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		if got := headline(map[string]any{"m1": nil}, sumPlan); got != "No bytes uploaded values are recorded in these IPDR sessions, so there is no total to state." {
			t.Fatalf("got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
	t.Run("a real total is unchanged", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		if got := headline(map[string]any{"m1": int64(1200), measureDenominatorKey("m1"): 5}, sumPlan); got != "The total bytes uploaded across IPDR sessions is 1,200." {
			t.Fatalf("got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
	t.Run("a real zero with counted values is still zero", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		if got := headline(map[string]any{"m1": int64(0), measureDenominatorKey("m1"): 3}, sumPlan); got != "The total bytes uploaded across IPDR sessions is 0." {
			t.Fatalf("got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
	t.Run("every aggregate that needs values", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		for op, word := range map[string]string{"AVG": "average", "MIN": "minimum", "MAX": "maximum"} {
			plan := &SourceNativePlanV1{ContractVersion: "forensics.source-native-plan/v1",
				Measures: []SourceNativeMeasureV1{{MeasureID: "m1", Op: op, FieldID: "ipdr.bytes_up"}}}
			want := "No bytes uploaded values are recorded in these IPDR sessions, so there is no " + word + " to state."
			if got := headline(s8, plan); got != want {
				t.Fatalf("%s: got %q", op, got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	})
}
