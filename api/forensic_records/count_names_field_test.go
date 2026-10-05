package main

import "testing"

func countFieldResponse(value any, measure SourceNativeMeasureV1) (hybridQueryResponse, *SourceNativePlanV1) {
	plan := &SourceNativePlanV1{ContractVersion: "forensics.source-native-plan/v1", Measures: []SourceNativeMeasureV1{measure}}
	resp := hybridQueryResponse{Records: map[string]any{
		"source_native_results": []map[string]any{{measure.MeasureID: value}},
		"field_catalog": []FieldDescriptorV1{
			{FieldID: "cdr.msisdn", NormalizedName: "msisdn", DisplayName: "Subscriber number"},
			{FieldID: "cdr.imei", NormalizedName: "imei", DisplayName: "Device IMEI"},
			{FieldID: "access_log.status", NormalizedName: "status", DisplayName: "HTTP status code"},
			{FieldID: "cdr.cell_id", NormalizedName: "cell_site_id"},
		},
	}}
	return resp, plan
}

func headlineFor(t *testing.T, req hybridQueryRequest, value any, measure SourceNativeMeasureV1) string {
	t.Helper()
	resp, plan := countFieldResponse(value, measure)
	answer, ok := sourceNativeResultAnswer(req, resp, plan)
	if !ok {
		t.Fatalf("no answer for %+v", measure) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	return answer.Headline
}

func TestCountNamesField(t *testing.T) {
	cdr := hybridQueryRequest{RecordType: "cdr"}
	distinctMSISDN := SourceNativeMeasureV1{MeasureID: "m1", Op: "COUNT_DISTINCT", FieldID: "cdr.msisdn"}
	countIMEI := SourceNativeMeasureV1{MeasureID: "m1", Op: "COUNT", FieldID: "cdr.imei"}
	countAll := SourceNativeMeasureV1{MeasureID: "m1", Op: "COUNT"}

	t.Run("switch off keeps today's sentences", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(countNamesFieldEnv, "")
		for measure, want := range map[SourceNativeMeasureV1]string{
			distinctMSISDN: "The count across CDR records is 10.",
			countIMEI:      "There are 10 CDR records in this case.",
			countAll:       "There are 10 CDR records in this case.",
		} {
			if got := headlineFor(t, cdr, int64(10), measure); got != want {
				t.Fatalf("%+v: got %q want %q", measure, got, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	})

	t.Setenv(countNamesFieldEnv, "true")

	cases := []struct {
		name    string
		req     hybridQueryRequest
		value   any
		measure SourceNativeMeasureV1
		want    string
	}{
		// CDR-08, H1-CDR-HANDSETS and the acronym rule.
		{"distinct names the field", cdr, int64(10), distinctMSISDN, "There are 10 distinct subscriber number values across CDR records."},
		{"an acronym inside the name survives", cdr, int64(4997), SourceNativeMeasureV1{MeasureID: "m1", Op: "COUNT_DISTINCT", FieldID: "cdr.imei"}, "There are 4,997 distinct device IMEI values across CDR records."},
		{"a leading acronym survives", hybridQueryRequest{RecordType: "access_log"}, int64(6), SourceNativeMeasureV1{MeasureID: "m1", Op: "COUNT_DISTINCT", FieldID: "access_log.status"}, "There are 6 distinct HTTP status code values across access log entries."},
		{"one distinct value", cdr, int64(1), distinctMSISDN, "There is 1 distinct subscriber number value across CDR records."},
		{"no distinct values", cdr, int64(0), distinctMSISDN, "There are no subscriber number values across CDR records."},
		{"no display name uses the source name", cdr, int64(3), SourceNativeMeasureV1{MeasureID: "m1", Op: "COUNT_DISTINCT", FieldID: "cdr.cell_id"}, "There are 3 distinct cell site id values across CDR records."},
		{"scope is kept", hybridQueryRequest{RecordType: "cdr", Target: "923001110001"}, int64(2), distinctMSISDN, "There are 2 distinct subscriber number values across CDR records involving 923001110001."},
		// COUNT(field) counts rows where the field is present, not every record.
		{"a field count says the field was present", cdr, int64(8000), countIMEI, "8,000 CDR records have a device IMEI value."},
		{"one record", cdr, int64(1), countIMEI, "1 CDR record has a device IMEI value."},
		{"no records", cdr, int64(0), countIMEI, "No CDR records have a device IMEI value."},
		// COUNT(*) is every record and is unchanged.
		{"a record count is unchanged", cdr, int64(8642), countAll, "There are 8,642 CDR records in this case."},
		// A field the catalogue cannot name is not described as all records either.
		{"an unnamed field claims no record total", cdr, int64(7), SourceNativeMeasureV1{MeasureID: "m1", Op: "COUNT", FieldID: "cdr.unknown"}, "The count across CDR records is 7."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			if got := headlineFor(t, tc.req, tc.value, tc.measure); got != tc.want {
				t.Fatalf("got  %q\nwant %q", got, tc.want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		})
	}

	// Measured on the A4 arm: "How many transactions have an amount above 50000?"
	// counts the amount field AND filters on it. The filter guarantees the field is
	// present, so the count is the record count and the record sentence stays.
	t.Run("a filter on the counted field keeps the record sentence", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		measure := SourceNativeMeasureV1{MeasureID: "m1", Op: "COUNT", FieldID: "transaction.amount"}
		resp, plan := countFieldResponse(int64(1), measure)
		plan.Filters = []SourceNativeFilterV1{{FieldID: "transaction.amount", Op: "GT", Value: "50000"}}
		answer, ok := sourceNativeResultAnswer(hybridQueryRequest{RecordType: "transaction"}, resp, plan)
		if !ok || answer.Headline != "There is 1 transaction in this case." {
			t.Fatalf("got ok=%v %q", ok, answer.Headline) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		// A filter on a DIFFERENT field does not guarantee this one is present.
		resp, plan = countFieldResponse(int64(8000), countIMEI)
		plan.Filters = []SourceNativeFilterV1{{FieldID: "cdr.msisdn", Op: "EQ", Value: "923001110001"}}
		if answer, _ := sourceNativeResultAnswer(cdr, resp, plan); answer.Headline != "8,000 CDR records have a device IMEI value." {
			t.Fatalf("got %q", answer.Headline) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}
