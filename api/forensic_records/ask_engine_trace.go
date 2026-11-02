package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ASK ENGINE V2: THE RECORDS BEHIND A SUMMARY (docs/data/answer-evidence-and-interpretation-20261102.md, part A2).
//
// Most of what an analyst asks is not a list of records but a summary of them: who a number talks to most (v_contact_pairs), who two numbers both contact (a join of two of those), how two numbers are
// connected (v_connection_edges), what an account paid (v_account_counterparties), where two numbers met (v_colocation_windows), where a number sleeps (v_weekly_anchors). A line of such an answer
// stands for records, and the analyst must be able to see them: the calls between the two numbers, the transactions of the account, the sightings of the plate.
//
// This table says, for every summary view of the catalogue, which records stand behind one of its lines (the raw view they are read from and the condition that selects them), and which numbers of the
// line can be recomputed from those records (the events of a pair are the count of the calls between them; the first and the last time are their earliest and latest). It is written next to the
// definitions of the views (migrations 017 to 031) and a spec holds the two together: every view of the catalogue is either a raw view (askEvidenceViews), traced here, or declared untraceable with
// its reason (askUntraceable); a view that is none of the three fails the build.
//
// The condition reads two relations only: r, a record of the raw view, and l, the line of the summary view (its grain and the columns the condition or the checks read). Nothing here is taken from a
// question: the line is selected by the answer's own conditions, inside the database, so that no value is ever written back as a literal.

// askTraceCheck says that a column of the line must equal an aggregate of the records behind it. COUNT stands for the count of the records.
type askTraceCheck struct {
	Column string
	Expr   string
}

// askTraceStep is the records behind one line: where they are read from and which of them.
type askTraceStep struct {
	Raw    string          // the raw view the records are read from (a key of askEvidenceViews); "" when the lines of the view are files
	Cond   string          // a condition over r (a record) and l (the line)
	Order  string          // the order the records are listed in, over r
	Checks []askTraceCheck // the numbers of the line that the records must give back
	Views  []string        // other views the condition reads
	Say    string          // what the records are, in a sentence
}

// askTraceComputed is a column the line gets in its CTE, computed once from the summary row (a is the alias of the view): the condition then reads it instead of computing it per record.
type askTraceComputed struct {
	Name string
	Expr string
}

type askTrace struct {
	Computed []askTraceComputed
	Grain    []string // the columns that identify a line
	Extra    []string // other columns of the view that the condition or the checks read
	Label    func(line map[string]string) string
	Pick     func(line map[string]string) (askTraceStep, bool)
}

func askStatic(step askTraceStep) func(map[string]string) (askTraceStep, bool) {
	return func(map[string]string) (askTraceStep, bool) { return step, true }
}

func askLabelJoin(cols ...string) func(map[string]string) string {
	return func(line map[string]string) string {
		var parts []string
		for _, c := range cols {
			if v := line[c]; v != "" {
				parts = append(parts, v)
			}
		}
		return strings.Join(parts, " · ")
	}
}

func askLabelArrow(from, to string) func(map[string]string) string {
	return func(line map[string]string) string { return line[from] + " → " + line[to] }
}

// the slot of a weekly anchor: the night is 22:00 to 07:00, the day 09:00 to 17:00 on weekdays, on the case clock (the session's time zone)
const askSlotCond = `((l.slot = 'night' AND (extract(hour FROM r.start_utc) >= 22 OR extract(hour FROM r.start_utc) < 7)) OR (l.slot = 'day' AND extract(hour FROM r.start_utc) BETWEEN 9 AND 16 AND extract(isodow FROM r.start_utc) <= 5))`

// the span of the weeks of an anchor, read from the weekly anchors of the same number and slot (a week is a block of 7 days that starts on week_start)
var askAnchorSpanColumns = []askTraceComputed{
	{"span_from", "(SELECT min(w.week_start) FROM v_weekly_anchors w WHERE w.number = {a}.number AND w.slot = {a}.slot AND w.week = {a}.first_week)::timestamptz"},
	{"span_to", "((SELECT min(w.week_start) FROM v_weekly_anchors w WHERE w.number = {a}.number AND w.slot = {a}.slot AND w.week = {a}.last_week) + 7)::timestamptz"},
}

const askAnchorSpan = `r.start_utc >= l.span_from AND r.start_utc < l.span_to`

var askPairChecks = []askTraceCheck{
	{"events", "COUNT"},
	{"outgoing", "COUNT FILTER (WHERE r.direction = 'outgoing')"},
	{"incoming", "COUNT FILTER (WHERE r.direction = 'incoming')"},
	{"unknown", "COUNT FILTER (WHERE r.direction NOT IN ('outgoing', 'incoming'))"},
	{"total_duration_s", "sum(r.duration_s)"},
	{"first_seen", "min(r.start_utc)"},
	{"last_seen", "max(r.start_utc)"},
}

// askTraces: one entry for every summary view of the catalogue.
var askTraces = map[string]askTrace{
	// ---- calls and messages
	"v_contact_pairs": {Grain: []string{"src", "dst"}, Extra: []string{"events", "outgoing", "incoming", "unknown", "total_duration_s", "first_seen", "last_seen"}, Label: askLabelArrow("src", "dst"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Cond: "r.src = l.src AND r.dst = l.dst", Order: "r.start_utc", Checks: askPairChecks, Say: "the calls and messages of the first number to or from the second, in its own records"})},
	"v_contact_links": {Grain: []string{"a", "b"}, Extra: []string{"events"}, Label: askLabelArrow("a", "b"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Cond: "(r.src = l.a AND r.dst = l.b) OR (r.src = l.b AND r.dst = l.a)", Order: "r.start_utc", Checks: []askTraceCheck{{"events", "COUNT"}},
			Say: "the calls and messages between the two numbers, in either direction"})},
	"v_connection_edges": {Grain: []string{"src", "dst"}, Label: askLabelArrow("src", "dst"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Cond: "(r.src = l.src AND r.dst = l.dst) OR (r.src = l.dst AND r.dst = l.src)", Order: "r.start_utc", Say: "the calls and messages that make this link, in either direction"})},
	"v_cell_visits": {Grain: []string{"src", "cell_key", "hour_utc"}, Extra: []string{"events"}, Label: askLabelJoin("src", "cell_key", "hour_utc"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Cond: "r.src = l.src AND r.cell_key = l.cell_key AND date_trunc('hour', r.start_utc) = l.hour_utc", Order: "r.start_utc", Checks: []askTraceCheck{{"events", "COUNT"}},
			Say: "the events of the number at this cell site in this hour"})},
	"v_cells": {Grain: []string{"cell_key"}, Label: askLabelJoin("cell_key", "site_name"), Extra: []string{"site_name"},
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Cond: "r.cell_key = l.cell_key", Order: "r.start_utc DESC", Say: "the events that were recorded at this cell site (the latest first)"})},
	"v_colocation_windows": {Grain: []string{"a", "b", "cell_key", "start_utc", "end_utc"}, Label: askLabelJoin("a", "b", "cell_key", "start_utc"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Cond: "r.cell_key = l.cell_key AND r.src IN (l.a, l.b) AND r.start_utc >= l.start_utc AND r.start_utc <= l.end_utc", Order: "r.start_utc",
			Say: "the events of the two numbers at this cell site inside the window"})},
	"v_meeting_candidates": {Grain: []string{"a", "b", "cell_key", "start_utc", "end_utc", "last_contact_utc"}, Label: askLabelJoin("a", "b", "cell_key", "start_utc"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Order: "r.start_utc",
			Cond: "(r.cell_key = l.cell_key AND r.src IN (l.a, l.b) AND r.start_utc >= l.start_utc AND r.start_utc <= l.end_utc) OR ((r.src = l.a AND r.dst = l.b OR r.src = l.b AND r.dst = l.a) AND r.start_utc = l.last_contact_utc)",
			Say:  "the events of the two numbers at the cell site inside the window, and the call or message between them shortly before"})},
	"v_silent_numbers": {Grain: []string{"number"}, Extra: []string{"events", "first_event_utc", "last_event_utc"}, Label: askLabelJoin("number"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Cond: "r.src = l.number", Order: "r.start_utc DESC", Say: "the events of the number, the latest first",
			Checks: []askTraceCheck{{"events", "COUNT"}, {"first_event_utc", "min(r.start_utc)"}, {"last_event_utc", "max(r.start_utc)"}}})},
	"v_successor_candidates": {Grain: []string{"old_number", "new_number"}, Extra: []string{"old_last_event_utc", "new_first_event_utc"}, Label: askLabelArrow("old_number", "new_number"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Order: "r.start_utc",
			Cond: "(r.src = l.old_number AND r.start_utc = l.old_last_event_utc) OR (r.src = l.new_number AND r.start_utc = l.new_first_event_utc)",
			Say:  "the two events that mark the hand-over: the old number's last event and the new number's first (the score rests on contacts, cell sites and handsets they share)"})},
	"v_rhythm_changes": {Grain: []string{"number", "first_day"}, Label: askLabelJoin("number", "first_day"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Cond: "r.src = l.number AND r.start_utc::date >= l.first_day - 1 AND r.start_utc::date <= l.first_day + 1", Order: "r.start_utc",
			Say: "the events of the number on the day its rhythm changed, the day before and the day after"})},
	"v_speed_alerts": {Grain: []string{"number", "from_time_utc", "to_time_utc", "from_cell_key", "to_cell_key"}, Label: askLabelJoin("number", "from_time_utc"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Order: "r.start_utc",
			Cond: "r.src = l.number AND ((r.start_utc = l.from_time_utc AND r.cell_key = l.from_cell_key) OR (r.start_utc = l.to_time_utc AND r.cell_key = l.to_cell_key))",
			Say:  "the two consecutive events whose distance and time give the speed"})},
	"v_handsets": {Grain: []string{"src", "imei", "imsi"}, Extra: []string{"events", "first_seen", "last_seen"}, Label: askLabelJoin("src", "imei"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Cond: "r.src = l.src AND r.imei IS NOT DISTINCT FROM l.imei AND r.imsi IS NOT DISTINCT FROM l.imsi", Order: "r.start_utc",
			Checks: []askTraceCheck{{"events", "COUNT"}, {"first_seen", "min(r.start_utc)"}, {"last_seen", "max(r.start_utc)"}}, Say: "the events of the number with this handset and SIM"})},
	"v_device_pairings": {Grain: []string{"src", "imei", "imsi"}, Extra: []string{"events", "first_seen", "last_seen"}, Label: askLabelJoin("src", "imei"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Cond: "r.src = l.src AND r.imei IS NOT DISTINCT FROM l.imei AND r.imsi IS NOT DISTINCT FROM l.imsi", Order: "r.start_utc",
			Checks: []askTraceCheck{{"events", "COUNT"}, {"first_seen", "min(r.start_utc)"}, {"last_seen", "max(r.start_utc)"}}, Say: "the events of the number with this handset and SIM"})},
	// ---- where a number sleeps and works
	"v_weekly_anchors": {Grain: []string{"number", "slot", "week", "week_start", "cell_key"}, Extra: []string{"events_at_anchor"}, Label: askLabelJoin("number", "slot", "week_start", "cell_key"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Order: "r.start_utc", Checks: []askTraceCheck{{"events_at_anchor", "COUNT"}},
			Cond: "r.src = l.number AND r.cell_key = l.cell_key AND r.start_utc >= l.week_start::timestamptz AND r.start_utc < (l.week_start + 7)::timestamptz AND " + askSlotCond,
			Say:  "the events of the number at this cell site in this week, in the hours of the slot"})},
	"v_movement_anchors": {Grain: []string{"number", "slot", "cell_key", "first_week", "last_week", "first_week_start"}, Label: askLabelJoin("number", "slot", "first_week_start", "cell_key"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Order: "r.start_utc",
			Cond: "r.src = l.number AND r.cell_key = l.cell_key AND r.start_utc >= l.first_week_start::timestamptz AND r.start_utc < (l.first_week_start + ((l.last_week - l.first_week + 1) * 7))::timestamptz AND " + askSlotCond,
			Say:  "the events of the number at this cell site during the weeks it was its anchor, in the hours of the slot"})},
	"v_anchor_changes": {Grain: []string{"number", "slot", "from_cell_key", "to_cell_key", "first_week_start"}, Label: askLabelJoin("number", "slot", "first_week_start", "to_cell_key"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Order: "r.start_utc",
			Cond: "r.src = l.number AND r.cell_key = l.to_cell_key AND r.start_utc >= l.first_week_start::timestamptz AND " + askSlotCond,
			Say:  "the first events of the number at its new anchor, in the hours of the slot"})},
	"v_anchor_runs": {Computed: askAnchorSpanColumns, Grain: []string{"number", "slot", "cell_key", "first_week", "last_week"}, Label: askLabelJoin("number", "slot", "cell_key", "first_week"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Order: "r.start_utc", Views: []string{"v_weekly_anchors"},
			Cond: "r.src = l.number AND r.cell_key = l.cell_key AND " + askAnchorSpan + " AND " + askSlotCond, Say: "the events of the number at this cell site during these weeks, in the hours of the slot"})},
	"v_anchor_deviations": {Computed: askAnchorSpanColumns, Grain: []string{"number", "slot", "cell_key", "first_week", "last_week"}, Label: askLabelJoin("number", "slot", "cell_key", "first_week"),
		Pick: askStatic(askTraceStep{Raw: "v_comm_events", Order: "r.start_utc", Views: []string{"v_weekly_anchors"},
			Cond: "r.src = l.number AND r.cell_key = l.cell_key AND " + askAnchorSpan + " AND " + askSlotCond, Say: "the events of the number at this cell site during the short stretch, in the hours of the slot"})},
	// ---- bank and wallet
	"v_account_counterparties": {Grain: []string{"account", "counterparty", "direction"}, Extra: []string{"transactions", "total_amount", "first_utc", "last_utc"}, Label: askLabelJoin("account", "direction", "counterparty"),
		Pick: askStatic(askTraceStep{Raw: "v_bank_transactions", Order: "r.txn_utc",
			Cond:   "r.account = l.account AND r.direction = l.direction AND (r.counterparty_account = l.counterparty OR r.counterparty_msisdn = l.counterparty)",
			Checks: []askTraceCheck{{"transactions", "COUNT"}, {"total_amount", "sum(r.amount)"}, {"first_utc", "min(r.txn_utc)"}, {"last_utc", "max(r.txn_utc)"}}, Say: "the transactions of the account with this counterparty, in this direction"})},
	"v_account_activity": {Grain: []string{"account", "day"}, Extra: []string{"transactions", "credits", "debits"}, Label: askLabelJoin("account", "day"),
		Pick: askStatic(askTraceStep{Raw: "v_bank_transactions", Order: "r.txn_utc", Cond: "r.account = l.account AND r.txn_utc::date = l.day",
			Checks: []askTraceCheck{{"transactions", "COUNT"}, {"credits", "coalesce(sum(r.amount) FILTER (WHERE r.direction = 'credit'), 0)"}, {"debits", "coalesce(sum(r.amount) FILTER (WHERE r.direction = 'debit'), 0)"}},
			Say:    "the transactions of the account on this day (the case clock)"})},
	"v_account_flows": {Grain: []string{"credit_id", "debit_id"}, Extra: []string{"account"}, Label: askLabelJoin("account", "credit_id", "debit_id"),
		Pick: askStatic(askTraceStep{Raw: "v_bank_transactions", Cond: "r.txn_id IN (l.credit_id, l.debit_id)", Order: "r.txn_utc", Say: "the credit and the debit that make the pass-through"})},
	"v_account_limit_bands": {Grain: []string{"account"}, Extra: []string{"transactions", "total_amount", "first_utc", "last_utc", "limit_amount", "share_percent"}, Label: askLabelJoin("account"),
		Pick: askStatic(askTraceStep{Raw: "v_bank_transactions", Order: "r.txn_utc",
			Cond:   "r.account = l.account AND r.direction = 'debit' AND r.amount >= l.limit_amount * l.share_percent / 100.0 AND r.amount < l.limit_amount",
			Checks: []askTraceCheck{{"transactions", "COUNT"}, {"total_amount", "sum(r.amount)"}, {"first_utc", "min(r.txn_utc)"}, {"last_utc", "max(r.txn_utc)"}}, Say: "the debits of the account just under the limit"})},
	// ---- plates, registrations, documents
	"v_plate_visits": {Grain: []string{"plate_norm", "camera_id"}, Extra: []string{"sightings", "first_utc", "last_utc"}, Label: askLabelJoin("plate_norm", "camera_id"),
		Pick: askStatic(askTraceStep{Raw: "v_plate_sightings", Cond: "r.plate_norm = l.plate_norm AND r.camera_id = l.camera_id", Order: "r.observed_utc",
			Checks: []askTraceCheck{{"sightings", "COUNT"}, {"first_utc", "min(r.observed_utc)"}, {"last_utc", "max(r.observed_utc)"}}, Say: "the sightings of the plate at this camera"})},
	"v_number_registration_activity": {Grain: []string{"msisdn"}, Extra: []string{"registrations", "sources"}, Label: askLabelJoin("msisdn"),
		Pick: askStatic(askTraceStep{Raw: "v_subscriber_registrations", Cond: "r.msisdn = l.msisdn", Order: "r.valid_from",
			Checks: []askTraceCheck{{"registrations", "COUNT"}, {"sources", "count(DISTINCT r.evidence_id)"}}, Say: "the registrations of the number, one per source file entry"})},
	"v_document_values": {Grain: []string{"evidence_id", "document", "kind", "value"}, Extra: []string{"mentions"}, Label: askLabelJoin("document", "kind", "value"),
		Pick: askStatic(askTraceStep{Raw: "v_document_mentions", Cond: "r.evidence_id = l.evidence_id AND r.kind = l.kind AND r.value = l.value", Order: "r.char_start",
			Checks: []askTraceCheck{{"mentions", "COUNT"}}, Say: "the mentions of this key in this document"})},
	"v_document_links": {Grain: []string{"kind", "value", "family"}, Extra: []string{"records"}, Label: askLabelJoin("kind", "value", "family"), Pick: askDocumentLinkStep},
	// ---- the files themselves
	"v_evidence_inventory": {Grain: []string{"evidence_id"}, Extra: []string{"original_filename", "modality"}, Label: askLabelJoin("original_filename"),
		Pick: askStatic(askTraceStep{Say: "the files themselves: this answer is about the evidence files of the case, not about records inside them"})},
}

// askDocumentLinkStep: the records of another family that carry the key a document names. The family tells which records.
func askDocumentLinkStep(line map[string]string) (askTraceStep, bool) {
	count := []askTraceCheck{{"records", "COUNT"}}
	switch line["kind"] + "/" + line["family"] {
	case "msisdn/calls":
		return askTraceStep{Raw: "v_comm_events", Cond: "r.src = l.value OR r.dst = l.value", Order: "r.start_utc", Checks: count, Say: "the calls and messages that name this number as the subscriber or as the other party"}, true
	case "msisdn/own_calls":
		return askTraceStep{Raw: "v_comm_events", Cond: "r.src = l.value", Order: "r.start_utc", Checks: count, Say: "the calls and messages in this number's own records"}, true
	case "msisdn/registrations":
		return askTraceStep{Raw: "v_subscriber_registrations", Cond: "r.msisdn = l.value", Order: "r.valid_from", Checks: count, Say: "the registrations of this number"}, true
	case "msisdn/wallet":
		return askTraceStep{Raw: "v_bank_transactions", Cond: "r.counterparty_msisdn = l.value", Order: "r.txn_utc", Checks: count, Say: "the wallet transactions with this number"}, true
	case "account/statements":
		return askTraceStep{Raw: "v_bank_transactions", Cond: "r.account = l.value OR r.counterparty_account = l.value", Order: "r.txn_utc", Checks: count, Say: "the transactions of this account and with it"}, true
	case "plate/sightings":
		return askTraceStep{Raw: "v_plate_sightings", Cond: "r.plate_norm = l.value", Order: "r.observed_utc", Checks: count, Say: "the sightings of this plate"}, true
	case "iccid/registrations":
		return askTraceStep{Raw: "v_subscriber_registrations", Cond: "r.iccid = l.value", Order: "r.valid_from", Checks: count, Say: "the registrations of this SIM card"}, true
	case "imsi/registrations":
		return askTraceStep{Raw: "v_subscriber_registrations", Cond: "r.imsi = l.value", Order: "r.valid_from", Checks: count, Say: "the registrations of this SIM identity"}, true
	case "imei/calls":
		return askTraceStep{Raw: "v_comm_events", Cond: "r.imei = l.value", Order: "r.start_utc", Checks: count, Say: "the events recorded with this handset"}, true
	}
	return askTraceStep{}, false
}

// askUntraceable are the views of the catalogue that have no records behind a line, with the reason said to the analyst.
var askUntraceable = map[string]string{}

// askTraceViewNames lists the views that have a trace, sorted (for the spec and the messages).
func askTraceViewNames() []string {
	names := make([]string, 0, len(askTraces))
	for name := range askTraces {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

var askLineColumn = regexp.MustCompile(`(^|[^A-Za-z0-9_.])l\.([a-z_]+)`)

// askTraceCond writes a condition for one alias of the query: the column of the line l.x becomes l.<alias>__x, the name the line CTE gives it.
func askTraceCond(cond, alias string) string {
	return askLineColumn.ReplaceAllString(cond, "${1}l."+alias+"__${2}")
}

// askTraceChecksSQL is the aggregates that give the checks back, over the records r of the raw view whose key is key.
func askTraceExpr(expr, key string) string {
	return strings.ReplaceAll(expr, "COUNT", "count(r."+key+")")
}

func askTraceUses(step askTraceStep) []string {
	views := []string{}
	if step.Raw != "" {
		views = append(views, step.Raw)
	}
	return append(views, step.Views...)
}

// askTraceDescribe is for the messages: the views that have no trace.
func askTraceMissing(names []string) []string {
	var missing []string
	for _, name := range names {
		if _, raw := askEvidenceViews[name]; raw {
			continue
		}
		if _, ok := askTraces[name]; ok {
			continue
		}
		if _, ok := askUntraceable[name]; ok {
			continue
		}
		missing = append(missing, name)
	}
	return missing
}

var _ = fmt.Sprintf
