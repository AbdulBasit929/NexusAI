package main

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/mudler/LocalAI/pkg/forensicrequest"
)

// ASK ENGINE V2: THE ANSWER, BUILT BY CODE FROM THE RESULT (roadmap 6.8.5, 6.8.7).
//
// No model writes a word of it. The headline is built from the result's own column names and values; the derivation is the parse tree restated in plain words
// (ask_engine_describe.go); the result's SHAPE (a scalar, a ranked list, a graph, a map, a series, lanes or a table, from the column-name convention of the prompt)
// is sent so that the interface can draw the view that fits and always offers the table. The composition of a sentence by the model, under the number lock, is
// milestone M5 and is not here.

// askCell formats the value of column i for the headline. A text column is shown exactly as stored (an identifier is not a quantity).
func askCell(r *govSQLResult, i int, value any) string {
	if text, ok := value.(string); ok && i < len(r.Texts) && r.Texts[i] {
		return text
	}
	return govSQLFormatValue(value)
}

func askLabel(r *govSQLResult, i int) string {
	if i < len(r.Labels) && r.Labels[i] != "" {
		return r.Labels[i]
	}
	if i < len(r.Columns) {
		return govSQLHumanize(r.Columns[i])
	}
	return "Result"
}

func askColumnIndex(r *govSQLResult, name string) int {
	for i, c := range r.Columns {
		if strings.EqualFold(c, name) {
			return i
		}
	}
	return -1
}

// askCount is a count with its noun in the right number: "1 link", "2 links".
func askCount(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// askHeadline is the one sentence that states the result.
func askHeadline(r *govSQLResult, shape, absence string) string {
	n := len(r.Rows)
	if absence != "" {
		return absence
	}
	more := ""
	if r.Truncated {
		more = fmt.Sprintf(" (the first %s; there are more)", govSQLGroup(fmt.Sprint(n)))
	}
	if n == 0 {
		if absence != "" {
			return absence
		}
		return "No row matches what you asked."
	}
	switch shape {
	case "scalar":
		return askLabel(r, 0) + ": " + askCell(r, 0, r.Rows[0][0]) + "."
	case "ranked":
		l, v := askColumnIndex(r, "label"), askColumnIndex(r, "value")
		items := make([]string, 0, askHeadlineItems)
		for i := 0; i < n && i < askHeadlineItems; i++ {
			items = append(items, fmt.Sprintf("%s (%s)", askCell(r, l, r.Rows[i][l]), askCell(r, v, r.Rows[i][v])))
		}
		tail := ""
		if n > askHeadlineItems {
			tail = fmt.Sprintf(", and %d more", n-askHeadlineItems)
		}
		return fmt.Sprintf("%s%s: %s%s.", askCount(n, "result", "results"), more, strings.Join(items, ", "), tail)
	case "graph":
		s, d := askColumnIndex(r, "src"), askColumnIndex(r, "dst")
		nodes := map[string]bool{}
		for _, row := range r.Rows {
			nodes[fmt.Sprint(row[s])], nodes[fmt.Sprint(row[d])] = true, true
		}
		return fmt.Sprintf("%s between %s%s.", askCount(n, "link", "links"), askCount(len(nodes), "number", "numbers"), more)
	case "map":
		// Rows of (cell, hour) visits are positions on the map, but the question was about cell sites: say how many sites the positions are at.
		head := askCount(n, "position", "positions")
		if c := askColumnIndex(r, "cell_key"); c >= 0 {
			sites := map[string]bool{}
			for _, row := range r.Rows {
				sites[fmt.Sprint(row[c])] = true
			}
			head += " at " + askCount(len(sites), "cell site", "cell sites")
		}
		return head + more + "."
	case "series":
		return fmt.Sprintf("%s%s.", askCount(n, "point", "points"), more)
	case "lanes":
		return fmt.Sprintf("%s%s.", askCount(n, "interval", "intervals"), more)
	}
	if n == 1 {
		parts := make([]string, 0, len(r.Columns))
		for i := range r.Columns {
			parts = append(parts, askLabel(r, i)+" "+askCell(r, i, r.Rows[0][i]))
		}
		return strings.Join(parts, "; ") + "."
	}
	return fmt.Sprintf("%d rows%s.", n, more)
}

// askPathHeadline is the sentence for a connection question (decision D22, 2026-10-16: two numbers are connected when they are in contact with each other or through
// other numbers, in either direction; two subscribers who both contact the same number are connected through that shared contact). It names the numbers that connect
// the two the question names, read from the result's own edges: the shortest routes between them are found in the returned graph, and every number of the sentence
// is a number of the question or of the result. "" when the question is not a connection question of two numbers or the result does not join them (the generic
// sentence is then used).
func askPathHeadline(r *govSQLResult, f askFrame, maxHops int) string {
	if !f.Cues["path"] || len(f.Numbers) < 2 {
		return ""
	}
	a, b := f.Numbers[0], f.Numbers[1]
	hops := f.Hops
	if hops == 0 {
		hops = maxHops
	}
	if len(r.Rows) == 0 {
		if hops > 0 {
			return fmt.Sprintf("No connection between %s and %s was found within %d hops through the calls and messages of this case.", a, b, hops)
		}
		return fmt.Sprintf("No connection between %s and %s was found through the calls and messages of this case.", a, b)
	}
	s, d := askColumnIndex(r, "src"), askColumnIndex(r, "dst")
	if s < 0 || d < 0 {
		return ""
	}
	next := map[string][]string{}
	for _, row := range r.Rows {
		x, y := fmt.Sprint(row[s]), fmt.Sprint(row[d])
		next[x] = append(next[x], y)
		next[y] = append(next[y], x)
	}
	distances := func(from string) map[string]int {
		dist := map[string]int{from: 0}
		queue := []string{from}
		for len(queue) > 0 {
			at := queue[0]
			queue = queue[1:]
			for _, to := range next[at] {
				if _, seen := dist[to]; !seen {
					dist[to] = dist[at] + 1
					queue = append(queue, to)
				}
			}
		}
		return dist
	}
	fromA, fromB := distances(a), distances(b)
	total, ok := fromA[b]
	if !ok || total < 1 {
		return ""
	}
	var between []string
	for node, da := range fromA {
		if db, reached := fromB[node]; reached && node != a && node != b && da+db == total {
			between = append(between, node)
		}
	}
	sort.Strings(between)
	list := func() string {
		shown := between
		if len(shown) > 5 {
			shown = shown[:5]
		}
		text := strings.Join(shown, ", ")
		if len(between) > len(shown) {
			text += fmt.Sprintf(" and %d more", len(between)-len(shown))
		}
		return text
	}
	switch {
	case total == 1:
		return fmt.Sprintf("%s and %s are in direct contact.", a, b)
	case total == 2 && len(between) == 1:
		return fmt.Sprintf("%s and %s are connected through a shared contact, %s.", a, b, between[0])
	case total == 2:
		return fmt.Sprintf("%s and %s are connected through %d shared contacts: %s.", a, b, len(between), list())
	case len(between) == 0:
		return ""
	}
	return fmt.Sprintf("%s and %s are connected in %d hops; the numbers between them on the shortest routes are %s.", a, b, total, list())
}

func askRowMaps(r *govSQLResult) []map[string]any { return govSQLRowMaps(r) }

// askAnswerResponse is the response of an answered question.
// askReadsRecords is whether the query reads a view over the typed call or internet records (the views that leave exact repeats out).
func askReadsRecords(v *askValidated) bool {
	for _, view := range v.Views {
		switch view {
		case "v_comm_events", "v_contact_pairs", "v_contact_links", "v_connection_edges", "v_cell_visits", "v_colocation_windows", "v_meeting_candidates", "v_device_pairings", "v_data_sessions":
			return true
		}
	}
	return false
}

func askAnswerResponse(req hybridQueryRequest, cat *askCatalogue, v *askValidated, r *govSQLResult, f askFrame, absence string, audit AskAuditV2) *hybridQueryResponse {
	shape := askShape(r.Columns, len(r.Rows))
	headline := askHeadline(r, shape, absence)
	if absence == "" && shape == "graph" {
		if text := askPathHeadline(r, f, v.MaxHops); text != "" {
			headline = text
		}
	}
	derivation := askDescribe(v, cat)
	sentences := derivation.Sentences()
	limitations := []string{
		"Computed by a model-written query over this case's analyst views, checked against your question and run read-only; the model saw no case data and wrote none of the numbers. The steps are listed under derivation.",
	}
	if len(f.Months)+len(f.Dates) > 0 || derivation.CaseClock {
		zone, loc := govSQLTimeZone()
		limitations = append(limitations, "Times are read and shown on the case clock ("+zone+", "+govSQLZoneOffset(loc, time.Now())+").")
	}
	if r.Truncated {
		limitations = append(limitations, fmt.Sprintf("Only the first %s rows are shown; the query matched more.", govSQLGroup(fmt.Sprint(len(r.Rows)))))
	}
	if audit.RepeatsLeftOut > 0 && askReadsRecords(v) {
		limitations = append(limitations, fmt.Sprintf("%s rows of this case's call and internet records are exact repeats of another row and are left out of these counts.", govSQLGroup(fmt.Sprint(audit.RepeatsLeftOut))))
	}
	if shape == "graph" {
		limitations = append(limitations, "A link is a record of contact between two numbers; it is not a finding about the people. Services and numbers that everyone contacts are left out where the query says so.")
	}
	if shape == "map" {
		limitations = append(limitations, "A cell site is a coverage area, not the position of a person.")
	}
	limitations = append(limitations, askViewCaveats(v)...)
	limitations = append(limitations, askParamNotes(f.ParamUses)...)
	rows := askRowMaps(r)
	answer := map[string]any{
		"result_state": "COMPLETED", "answer": headline, "grounding": askAnswerGround,
		"ask_engine_v2": map[string]any{"shape": shape, "sql": v.SQL, "columns": r.Columns, "row_count": len(r.Rows), "views": v.Views},
	}
	enterprise := map[string]any{
		"status": "completed", "result_state": "COMPLETED", "executive_answer": headline,
		"summary": limitations[0], "limitations": limitations,
		"data_grid": enterpriseDataGrid(rows), "row_count": len(rows),
		"result_shape": shape,
		"provenance": []map[string]any{{
			"source": "analyst_views_sql", "kind": "ask_engine_v2", "views": v.Views, "sql": v.SQL, "row_count": len(rows),
		}},
		"metrics": askMetrics(r, shape),
		"operation": map[string]any{
			"template": "ask_engine_v2", "operation_id": "forensics.ask_engine_v2", "source_access": "analyst_views_sql",
		},
		"coverage": map[string]any{
			"tenant_id": req.TenantID, "collection_id": req.CollectionID,
			"route": []string{"ask_engine_v2"}, "queried_record_sql": true, "queried_kb": false,
		},
		"derivation": map[string]any{
			"sql": v.SQL, "steps": sentences, "conditions_checked": audit.Checks, "views": v.Views,
			"model": audit.Model, "prompt_id": audit.PromptID, "attempts": audit.Attempts, "repairs": audit.Repairs,
		},
	}
	return &hybridQueryResponse{
		CollectionID: req.CollectionID, RequestClass: forensicrequest.GovernedAnalysis, Intent: intentRecords,
		Route: []string{"ask_engine_v2"}, Policy: askAnswerPolicy,
		Planner:   map[string]any{"contract_version": forensicrequest.ContractV1, "request_class": forensicrequest.GovernedAnalysis},
		QueryPlan: QueryPlan{}, Capability: queryCapabilityAssessment{Status: "supported"},
		Answer: answer, GeneratedAt: time.Now().UTC(), Enterprise: enterprise,
		Telemetry: QueryTelemetry{TotalLatencyMS: audit.TotalMS},
	}
}

func askMetrics(r *govSQLResult, shape string) []map[string]any {
	if shape != "scalar" && !(len(r.Rows) == 1 && len(r.Columns) <= 6) {
		return []map[string]any{}
	}
	metrics := make([]map[string]any, 0, len(r.Columns))
	for i := range r.Columns {
		metrics = append(metrics, map[string]any{"label": askLabel(r, i), "value": askCell(r, i, r.Rows[0][i]), "source": "ask_engine_v2"})
	}
	return metrics
}

// askClarification is a response that says what was not done and why. No number is shown: a number that ignores part of the question would be a wrong answer.
func askClarification(req hybridQueryRequest, text, limitation string, audit AskAuditV2, steps []string) *hybridQueryResponse {
	answer := map[string]any{"result_state": "CLARIFICATION_REQUIRED", "clarification": text, "grounding": askAnswerGround}
	enterprise := terminalEnterprisePayload(req, forensicrequest.Clarify, answer)
	enterprise["limitations"] = []string{limitation}
	enterprise["summary"] = limitation
	enterprise["derivation"] = map[string]any{"views": audit.Views, "steps": steps, "attempts": audit.Attempts, "repairs": audit.Repairs}
	return &hybridQueryResponse{
		CollectionID: req.CollectionID, RequestClass: forensicrequest.GovernedAnalysis, Intent: intentClarify,
		Route: []string{"ask_engine_v2", "clarification"}, Policy: askAnswerPolicy,
		Planner:   map[string]any{"contract_version": forensicrequest.ContractV1, "request_class": forensicrequest.GovernedAnalysis},
		QueryPlan: QueryPlan{}, Capability: queryCapabilityAssessment{Status: "clarification"},
		Answer: answer, GeneratedAt: time.Now().UTC(), Enterprise: enterprise,
		Telemetry: QueryTelemetry{TotalLatencyMS: audit.TotalMS},
	}
}

// askAbstainResponse is the answer "not answered, and why" when a query was written but could not be shown to answer this question.
func askAbstainResponse(req hybridQueryRequest, cat *askCatalogue, f askFrame, v *askValidated, unmet []askObligation, audit AskAuditV2) *hybridQueryResponse {
	reasons := make([]string, 0, len(unmet))
	for _, item := range unmet {
		if item.Reason != "" {
			reasons = append(reasons, item.Reason)
		}
	}
	text := "I did not give an answer, because I could not show that the query I wrote asks everything you asked: " + strings.Join(reasons, "; ") + ". Rephrase the question with the exact number, date or condition."
	var steps []string
	if v != nil {
		steps = askDescribe(v, cat).Sentences()
	}
	return askClarification(req, text, "No result is shown, because a number that ignores part of the question would be a wrong answer. Nothing was inferred.", audit, steps)
}

// askTooBroadResponse says that the query would read too much, or took too long, and what to do: "too broad" is a normal outcome of free SQL.
func askTooBroadResponse(req hybridQueryRequest, run *govSQLRunError, audit AskAuditV2) *hybridQueryResponse {
	what := "took longer than the limit"
	if run != nil && run.Code == "TOO_EXPENSIVE" {
		what = "would read too much of the case"
	}
	text := "This question is too broad: the query " + what + ". Narrow it by a number, a time period or a cell, and ask again."
	return askClarification(req, text, "Nothing was run to completion, so no result is shown.", audit, nil)
}

// askLogAudit writes one line for each question the engine touched: what it did, how many attempts, which views, how long. Never the question, the query or a
// value; the reason is one of the engine's own fixed phrases or codes.
func askLogAudit(a AskAuditV2) {
	slog.Info("ask engine v2", "state", a.State, "attempts", a.Attempts, "views", strings.Join(a.Views, ","), "examples", strings.Join(a.Examples, ","),
		"repairs", strings.Join(a.Repairs, ","), "reason", govSQLClip(a.Reason, 120), "rows", a.Rows, "shape", a.Shape, "agreement", a.Agreement,
		"model_ms", a.ModelMS, "exec_ms", a.ExecMS, "total_ms", a.TotalMS)
}
