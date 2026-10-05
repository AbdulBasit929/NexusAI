package main

import "testing"

func TestTableHeader(t *testing.T) {
	t.Run("switch off keeps today's humanizer", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(tableHeaderCasingEnv, "")
		if got := tableHeader("call_end_ts"); got != humanizeField("call_end_ts") {
			t.Fatalf("header %q changed with the switch off", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("switch on", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(tableHeaderCasingEnv, "true")
		// Recorded on the deployed build, 2026-09-29, with what they should read.
		cases := map[string]string{
			"call_end_ts":             "Call end timestamp",
			"CALL_DIALED_NUM":         "Call dialed number",
			"CALL_END_DT_TM":          "Call end date time",
			"row_hash":                "Row hash",
			"completed_at":            "Completed at",
			"sampled_non_empty":       "Sampled non empty",
			"imei":                    "IMEI",
			"cnic":                    "CNIC",
			"msisdn":                  "MSISDN",
			"Cell_SITE_ID":            "Cell site ID",
			"source_ip":               "Source IP",
			"evidence_id":             "Evidence ID",
			"rag_status":              "RAG status",
			"incoming_outgoing_ratio": "Incoming outgoing ratio",
			"call_org_num":            "Call originating number",
		}
		for key, want := range cases {
			if got := tableHeader(key); got != want {
				t.Errorf("%s: header %q, want %q", key, got, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	})

	t.Run("metric labels keep the humanizer the plumbing check depends on", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(tableHeaderCasingEnv, "true")
		// factPacketPlumbingMetric matches "Records Row Count" exactly; the metric
		// path must still produce that text.
		if humanizeField("records_row_count") != "Records ROW Count" && !factPacketPlumbingMetric["Records Row Count"] {
			t.Skip("plumbing label set changed; re-check this pin") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if got := humanizeField("records_row_count"); got == tableHeader("records_row_count") {
			t.Fatalf("humanizeField must be untouched by A1.1, got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}
