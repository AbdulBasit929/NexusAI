package main

import (
	"strings"
	"testing"
	"time"
)

// Rows from the deployed build's CDR-10 response (reports/column-labels-20260929/on):
// the top two contacts are tied at 121 CDR records.
func cdr10Contacts() []map[string]any {
	contact := func(number string, total int64) map[string]any {
		return map[string]any{"counterparty": number, "total_interactions": total}
	}
	return []map[string]any{
		contact("923009998887", 121), contact("923009998883", 121), contact("923009998882", 117),
		contact("923009998885", 109), contact("923009998886", 105), contact("923009998881", 101),
		contact("923009998884", 100), contact("923009998888", 98),
	}
}

func contactsResponse(rows []map[string]any) hybridQueryResponse {
	return hybridQueryResponse{Template: "frequent_contacts", Answer: map[string]any{}, Records: map[string]any{"frequent_contacts": rows}}
}

func TestFrequentContactLeaders(t *testing.T) {
	req := hybridQueryRequest{Query: "Who did 923001110001 contact most frequently?", Target: "923001110001", Limit: 20}

	t.Run("switch off says nothing new", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(templateStatesResultEnv, "")
		if got := frequentContactLeaders(req, contactsResponse(cdr10Contacts())); got != "" {
			t.Fatalf("switch off produced %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if got := enterpriseExecutiveAnswerBody(req, contactsResponse(cdr10Contacts()), cdr10Contacts()); got != "8 phone contacts ranked for 923001110001." {
			t.Fatalf("switch off changed the sentence: %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Setenv(templateStatesResultEnv, "true")

	t.Run("a tie names every tied contact, not one of them", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		want := "923009998887 and 923009998883 are tied as the most frequent contacts of 923001110001, with 121 CDR records each."
		if got := frequentContactLeaders(req, contactsResponse(cdr10Contacts())); got != want {
			t.Fatalf("got  %q\nwant %q", got, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		got := enterpriseExecutiveAnswerBody(req, contactsResponse(cdr10Contacts()), cdr10Contacts())
		if got != want+" 8 phone contacts ranked for 923001110001." {
			t.Fatalf("the ranking size must follow the leaders: %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("a single leader is named with its count", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		rows := cdr10Contacts()[1:]
		want := "The most frequent contact of 923001110001 is 923009998883, with 121 CDR records between them."
		if got := frequentContactLeaders(req, contactsResponse(rows)); got != want {
			t.Fatalf("got  %q\nwant %q", got, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("row order does not decide the leader", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		rows := cdr10Contacts()
		rows[0], rows[7] = rows[7], rows[0]
		if got := frequentContactLeaders(req, contactsResponse(rows)); !strings.HasPrefix(got, "923009998883 and 923009998887 are tied") {
			t.Fatalf("leaders must come from the counts: %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("a full page that is all one tie may continue past the page", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		rows := cdr10Contacts()[:2]
		if got := frequentContactLeaders(hybridQueryRequest{Target: "923001110001", Limit: 2}, contactsResponse(rows)); got != "" {
			t.Fatalf("the tie set is not known, but it claimed %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if got := frequentContactLeaders(hybridQueryRequest{Target: "923001110001", Limit: 20}, contactsResponse(rows)); !strings.Contains(got, "are tied") {
			t.Fatalf("a short page holds every tied row: %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("an unreadable row claims nothing", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		for _, bad := range []map[string]any{
			{"counterparty": "923009998887", "total_interactions": "121"},
			{"counterparty": "923009998887", "total_interactions": 12.5},
			{"counterparty": "", "total_interactions": int64(200)},
			{"counterparty": "923009998887"},
		} {
			rows := append(cdr10Contacts(), bad)
			if got := frequentContactLeaders(req, contactsResponse(rows)); got != "" {
				t.Fatalf("row %v produced %q", bad, got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		if got := frequentContactLeaders(req, contactsResponse(nil)); got != "" {
			t.Fatalf("no rows produced %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("filters are named, not restated", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		scoped := req
		scoped.DateFrom, scoped.DateTo, scoped.Direction = "2026-04-01T00:00:00Z", "2026-04-10T00:00:00Z", "OUTGOING"
		got := frequentContactLeaders(scoped, contactsResponse(cdr10Contacts()))
		if !strings.Contains(got, "of 923001110001 in the requested date range, counting only outgoing records, with 121") {
			t.Fatalf("scope missing: %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("many tied leaders are bounded", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		rows := []map[string]any{}
		for _, number := range []string{"1", "2", "3", "4", "5", "6", "7"} {
			rows = append(rows, map[string]any{"counterparty": "92300999000" + number, "total_interactions": int64(9)})
		}
		rows = append(rows, map[string]any{"counterparty": "923009990009", "total_interactions": int64(1)})
		got := frequentContactLeaders(req, contactsResponse(rows))
		if !strings.Contains(got, "923009990005 and 2 others are tied") || strings.Contains(got, "923009990006") {
			t.Fatalf("got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("only the frequent-contacts template", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		resp := contactsResponse(cdr10Contacts())
		resp.Template = "call_type_breakdown"
		if got := frequentContactLeaders(req, resp); got != "" {
			t.Fatalf("got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}

// Coverage from the deployed build's ANPR-03 response: 87 exact matches, all
// ANPR, first 2026-07-14T06:00:00Z and last 2026-07-19T07:22:00Z. The capped page
// of 20 displayed matches starts later (07:50), so the page must not be used.
func anpr03Response(coverage []map[string]any) hybridQueryResponse {
	return hybridQueryResponse{Template: "cross_family_correlation", Answer: map[string]any{}, Records: map[string]any{
		"family_coverage": coverage,
		"target_matches":  []map[string]any{{"requested_target": "LHR-2026", "observed_at": "2026-07-14T07:50:00+00:00"}},
	}}
}

func lhrCoverage() []map[string]any {
	return []map[string]any{{
		"requested_target": "LHR-2026", "record_type": "anpr", "observation_count": 87,
		"first_seen": time.Date(2026, 7, 14, 6, 0, 0, 0, time.UTC), "last_seen": time.Date(2026, 7, 19, 7, 22, 0, 0, time.UTC),
	}}
}

func TestTargetFirstLastSeen(t *testing.T) {
	req := hybridQueryRequest{Query: "When was LHR-2026 first and last seen?", Target: "LHR-2026", Limit: 20}

	t.Run("switch off says nothing new", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(templateStatesResultEnv, "")
		if got := targetFirstLastSeen(req, anpr03Response(lhrCoverage())); got != "" {
			t.Fatalf("switch off produced %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Setenv(templateStatesResultEnv, "true")

	t.Run("both times come from the complete coverage", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		want := "LHR-2026 was first seen at 2026-07-14 06:00:00 UTC and last seen at 2026-07-19 07:22:00 UTC, across 87 ANPR sightings."
		if got := targetFirstLastSeen(req, anpr03Response(lhrCoverage())); got != want {
			t.Fatalf("got  %q\nwant %q", got, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if got := enterpriseExecutiveAnswerBody(req, anpr03Response(lhrCoverage()), []map[string]any{{"x": 1}}); got != want {
			t.Fatalf("executive answer: %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("string timestamps parse the same way", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		coverage := lhrCoverage()
		coverage[0]["first_seen"], coverage[0]["last_seen"] = "2026-07-14T06:00:00Z", "2026-07-19T12:22:00+05:00"
		if got := targetFirstLastSeen(req, anpr03Response(coverage)); !strings.Contains(got, "last seen at 2026-07-19 07:22:00 UTC") {
			t.Fatalf("got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("several families name where each end was seen", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		coverage := append(lhrCoverage(), map[string]any{
			"requested_target": "LHR-2026", "record_type": "cdr", "observation_count": int64(3),
			"first_seen": "2026-07-10T09:00:00Z", "last_seen": "2026-07-12T09:00:00Z",
		})
		want := "LHR-2026 was first seen at 2026-07-10 09:00:00 UTC (CDR record) and last seen at 2026-07-19 07:22:00 UTC (ANPR sighting), across 90 matching records in 2 record families."
		if got := targetFirstLastSeen(req, anpr03Response(coverage)); got != want {
			t.Fatalf("got  %q\nwant %q", got, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("one sighting and one shared time", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		coverage := lhrCoverage()
		coverage[0]["observation_count"], coverage[0]["last_seen"] = 1, coverage[0]["first_seen"]
		if got := targetFirstLastSeen(req, anpr03Response(coverage)); got != "LHR-2026 was seen once, at 2026-07-14 06:00:00 UTC (1 ANPR sighting)." {
			t.Fatalf("got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		coverage[0]["observation_count"] = 3
		if got := targetFirstLastSeen(req, anpr03Response(coverage)); got != "All 3 ANPR sightings of LHR-2026 share one time, 2026-07-14 06:00:00 UTC." {
			t.Fatalf("got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("only a question about first or last", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		for _, question := range []string{"Correlate LHR-2026 across all record families", "Show the last 5 records seen for LHR-2026", "What else appears with LHR-2026?"} {
			if got := targetFirstLastSeen(hybridQueryRequest{Query: question}, anpr03Response(lhrCoverage())); got != "" {
				t.Fatalf("%q produced %q", question, got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		for _, question := range []string{"When was LHR-2026 last seen?", "When did LHR-2026 first appear?", "earliest sighting of LHR-2026"} {
			if got := targetFirstLastSeen(hybridQueryRequest{Query: question}, anpr03Response(lhrCoverage())); got == "" {
				t.Fatalf("%q was not recognised", question) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	})

	t.Run("unreadable coverage claims nothing", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		for _, mutate := range []func(map[string]any){
			func(row map[string]any) { row["first_seen"] = "yesterday" },
			func(row map[string]any) { delete(row, "last_seen") },
			func(row map[string]any) { row["observation_count"] = 0 },
			func(row map[string]any) { row["requested_target"] = "" },
			func(row map[string]any) { row["last_seen"] = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC) },
		} {
			coverage := lhrCoverage()
			mutate(coverage[0])
			if got := targetFirstLastSeen(req, anpr03Response(coverage)); got != "" {
				t.Fatalf("coverage %v produced %q", coverage[0], got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
		if got := targetFirstLastSeen(req, anpr03Response(nil)); got != "" {
			t.Fatalf("no coverage produced %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("a dated question names its date range", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		scoped := req
		scoped.DateFrom = "2026-07-15T00:00:00Z"
		if got := targetFirstLastSeen(scoped, anpr03Response(lhrCoverage())); !strings.HasSuffix(got, "across 87 ANPR sightings in the requested date range.") {
			t.Fatalf("got %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}
