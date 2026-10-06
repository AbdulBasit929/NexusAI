package main

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensicrequest"
)

// GOVERNED SQL LANE: THE LOOP.
//
//	question
//	  -> shortlist the views it is about (curated vocabulary, not embeddings)
//	  -> read what it demands (identifiers, values, numbers, time, names, shape)
//	  -> model writes one SELECT, shown only the schema
//	  -> validator (parse tree, allowlist)            reject: one correction
//	  -> verifier (the demands are in the query)      unmet:  one correction, else ABSTAIN
//	  -> executor (server-built view, read-only, priced, capped)
//	  -> shape check and, for an empty result, a search for the identifier everywhere
//	  -> deterministic answer, the query shown, the conditions checked listed
//
// THREE OUTCOMES, never a fourth:
//
//	answered   a result that passed every check, with the query that produced it
//	abstained  the lane says which condition it could not apply, and does NOT show a
//	           number anyway. This is a success (CONTINUATION §5), and the analyst is told
//	           what to change.
//	declined   the lane has no opinion (no recognised family, the model was unavailable,
//	           the query could not be made valid): the request continues down the old path
//	           exactly as if the lane did not exist.
//
// The lane never narrates. The headline is built from the result's own column names and
// values; no model sees a row. A number on the screen is a number the database computed.
const govSQLMaxAttempts = 2

// GovSQLAuditV1 is what the lane records about a request it looked at, on every
// response it touched and in the X-Governed-SQL header.
type GovSQLAuditV1 struct {
	State       string   `json:"state"` // answered | abstained | declined
	Reason      string   `json:"reason,omitempty"`
	Views       []string `json:"views,omitempty"`
	Attempts    int      `json:"attempts"`
	SQL         string   `json:"sql,omitempty"`
	Checked     []string `json:"conditions_checked,omitempty"`
	ModelMS     int64    `json:"model_ms"`
	ExecMS      int64    `json:"exec_ms"`
	TotalMS     int64    `json:"total_ms"`
	Rows        int      `json:"rows"`
	Truncated   bool     `json:"truncated,omitempty"`
	PromptBytes int      `json:"prompt_bytes,omitempty"`
	PromptID    string   `json:"prompt_id,omitempty"`
}

// govSQLReclassifiable is true for a question the keyword classifier labelled a
// concept or a clarification although it asks for a computed value (earliest, total,
// how many...). Such a request may reach the lane, but only an ANSWERED result may
// replace the reply it would otherwise have got: an abstention or a decline leaves
// the existing reply untouched, so concepts, greetings and help are never changed.
func govSQLReclassifiable(req hybridQueryRequest) bool {
	switch req.RequestClass {
	case forensicrequest.GeneralDomainKnowledge, forensicrequest.Clarify:
		return forensicrequest.HasAnalyticalIntent(req.Query) || govSQLEarliestLatest.MatchString(req.Query)
	}
	return false
}

var govSQLEarliestLatest = regexp.MustCompile(`(?i)\b(?:earliest|latest|first|last|oldest|newest|most\s+recent)\b`)

func govSQLEligible(req hybridQueryRequest) (bool, string) {
	switch {
	case strings.TrimSpace(req.Query) == "":
		return false, "no question"
	case !govSQLReclassifiable(req) && req.RequestClass != forensicrequest.GovernedAnalysis:
		return false, "not a data question"
	case req.Template != "":
		return false, "an explicit operation was requested"
	case req.Group != nil || req.Compare != nil || len(req.Projection) > 0:
		return false, "a structured request"
	case req.TenantID == "" || req.CollectionID == "":
		return false, "no authorised scope"
	}
	if _, kind := questionStatesRelationalCondition(req.Query); kind != relationalNone {
		return false, "a relationship between records needs a join, which this lane does not attempt"
	}
	return true, ""
}

// govSQLMediaCue marks a question about text or media evidence (a transcript, a photo, a document).
// Those are answered by the retrieval path, which searches the text itself; the structured
// views cannot say whether a number is mentioned in a recording.
// govSQLWholeCase marks a question about the case as a whole ("what time period does this case
// cover", "where does X appear across all evidence"). One evidence family cannot answer it.
var govSQLWholeCase = regexp.MustCompile(`(?i)\b(?:this case|the case|entire case|whole case|all (?:the )?evidence|every evidence|across (?:all|every|the)\b|all (?:the )?(?:families|record types))\b`)

var govSQLMediaCue = regexp.MustCompile(`(?i)\b(?:images?|photos?|pictures?|jpe?g|video|videos|footage|audio|recordings?|transcripts?|speech|spoken|documents?|pdfs?|ocr|scans?|scanned|faces?)\b`)

// runGovernedSQLLane returns a response when the lane answered or abstained, and nil
// when it declined. The audit is always filled.
func runGovernedSQLLane(ctx context.Context, cfg config, db *pgxpool.Pool, req hybridQueryRequest, startedAt time.Time) (*hybridQueryResponse, GovSQLAuditV1) {
	audit := GovSQLAuditV1{State: "declined"}
	finish := func(resp *hybridQueryResponse) (*hybridQueryResponse, GovSQLAuditV1) {
		// A response that is not an answer is an abstention: it says why and shows no number.
		if resp != nil && audit.State == "declined" {
			audit.State = "abstained"
		}
		audit.TotalMS = time.Since(startedAt).Milliseconds()
		if resp != nil {
			resp.Telemetry.TotalLatencyMS = audit.TotalMS
			if resp.Planner != nil {
				resp.Planner["governed_sql"] = audit
			}
		}
		return resp, audit
	}
	decline := func(reason string) (*hybridQueryResponse, GovSQLAuditV1) {
		audit.State, audit.Reason = "declined", reason
		return finish(nil)
	}
	if ok, reason := govSQLEligible(req); !ok {
		return decline(reason)
	}
	if db == nil {
		return decline("no database")
	}
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return decline("no semantic layer")
	}
	all := govSQLViews(layer, req)
	offered := govSQLShortlist(all, req.Query)
	if govSQLMediaCue.MatchString(req.Query) && (len(offered) == 0 || !offered[0].binding.Derived) {
		return decline("text and media evidence is answered by the retrieval path")
	}
	if len(offered) == 0 {
		// A name the evidence cannot hold is answered "not answerable" even when no family is recognised:
		// declining would hand the question to a path that answers it with a total (measured 2026-10-06).
		if facts := govSQLExtractFacts(req.Query, all, all); len(facts.Names) > 0 {
			return finish(govSQLAbstainResponse(req, nil, facts, []govSQLObligation{govSQLNameObligation(facts.Names[0])}, audit))
		}
		return decline("no evidence family recognised in the question")
	}
	for _, view := range offered {
		audit.Views = append(audit.Views, view.Name)
		// A derived view's scope function does not honour a selected evidence item, so a
		// question scoped to one is left to the path that does.
		if view.binding.Derived && (req.EvidenceID != "" || req.QueryScope.Kind == string(EvidenceScopeSelected)) {
			return decline("derived evidence scoped to a selected item")
		}
	}
	facts := govSQLExtractFacts(req.Query, offered, all)
	if govSQLWholeCase.MatchString(req.Query) && len(facts.NamedViews) == 0 {
		return decline("the question is about the whole case, not one evidence family")
	}
	// A question that names no evidence family, no column and no declared value gives the shortlist
	// nothing but loose words to go on; the model would be guessing the family. A name the evidence
	// cannot hold is still an abstention (a decline would hand it to a path that answers with a total).
	if len(facts.NamedViews) == 0 && len(facts.Fields) == 0 && len(facts.ValueFacts) == 0 {
		if len(facts.Names) > 0 {
			return finish(govSQLAbstainResponse(req, offered, facts, []govSQLObligation{govSQLNameObligation(facts.Names[0])}, audit))
		}
		return decline("the question names no evidence family or field the curated layer knows")
	}
	if facts.WithheldAttr != "" {
		return decline("the question asks for an attribute the evidence withholds (" + facts.WithheldAttr + ")")
	}
	offeredByName := map[string]*govSQLView{}
	for _, view := range offered {
		offeredByName[view.Name] = view
	}
	model := strings.TrimSpace(req.SynthesisModel)
	if model == "" {
		model = cfg.SynthesisModel
	}
	turns := []govSQLTurn{
		{Role: "system", Content: govSQLSystemPrompt(offered)},
		{Role: "user", Content: govSQLUserPrompt(req.Query, facts, offered)},
	}
	audit.PromptBytes = len(turns[0].Content) + len(turns[1].Content)
	audit.PromptID = govSQLDigest(turns[:1])

	retry := func(raw string, problems ...string) {
		turns = append(turns, govSQLTurn{Role: "assistant", Content: raw}, govSQLTurn{Role: "user", Content: govSQLRetryPrompt(problems)})
	}
	for attempt := 1; attempt <= govSQLMaxAttempts; attempt++ {
		audit.Attempts = attempt
		last := attempt == govSQLMaxAttempts

		modelStart := time.Now()
		raw, failure := govSQLModel(ctx, cfg, model, turns)
		audit.ModelMS += time.Since(modelStart).Milliseconds()
		if failure != "" {
			return decline("model: " + failure)
		}
		answerable, sql, bad := govSQLParseOutput(raw)
		if bad != "" {
			if last {
				return decline("model output was not the requested JSON")
			}
			retry(raw, "Return exactly one JSON object with answerable and sql.")
			continue
		}
		if !answerable || sql == "" {
			if len(facts.Names) > 0 {
				return finish(govSQLAbstainResponse(req, offered, facts, []govSQLObligation{govSQLNameObligation(facts.Names[0])}, audit))
			}
			return decline("the model judged the question not answerable from the listed columns")
		}

		validated, rejection := govSQLValidate(sql, offeredByName)
		if rejection != nil {
			audit.SQL = govSQLClip(sql, 400)
			if last {
				return decline("query rejected: " + rejection.Code)
			}
			retry(raw, rejection.Message)
			continue
		}
		audit.SQL = validated.SQL

		if unmet := govSQLCheck(facts, validated); len(unmet) > 0 {
			// A missing name cannot be repaired by rewriting: no column holds one.
			names := false
			problems := make([]string, 0, len(unmet))
			for _, item := range unmet {
				names = names || item.Kind == "NAME"
				problems = append(problems, item.Message)
			}
			if last || names {
				return finish(govSQLAbstainResponse(req, offered, facts, unmet, audit))
			}
			retry(raw, problems...)
			continue
		}

		execStart := time.Now()
		result, err := govSQLExecute(ctx, db, req, validated, 0, 0)
		audit.ExecMS += time.Since(execStart).Milliseconds()
		if err != nil {
			run, _ := err.(*govSQLRunError)
			if run != nil && run.Code == "QUERY_ERROR" && !last {
				retry(raw, "The database rejected it: "+run.Message)
				continue
			}
			code := "DATABASE"
			if run != nil {
				code = run.Code
			}
			return decline("execution: " + code)
		}
		audit.Rows, audit.Truncated = len(result.Rows), result.Truncated

		if shape := govSQLCheckShape(facts, result); shape != nil {
			if last {
				return decline("the result was a list where one value was asked for")
			}
			retry(raw, shape.Message)
			continue
		}

		absence := ""
		if govSQLResultEmpty(result) && len(facts.Identifiers) > 0 {
			hits, err := govSQLProbeIdentifiers(ctx, db, req, all, facts.Identifiers)
			if err != nil {
				return decline("could not search for the identifier")
			}
			switch verdict := govSQLReadProbe(hits, facts.Identifiers, validated); {
			case verdict.elsewhereView != "":
				return finish(govSQLElsewhereResponse(req, offered, all, verdict, audit))
			case verdict.sameViewColumns != "" && !last:
				retry(raw, fmt.Sprintf("Nothing matched, but %s occurs in %s of this view. Look in those columns, joined with OR.", verdict.id, verdict.sameViewColumns))
				continue
			case verdict.absentEverywhere:
				absence = verdict.searched
			}
		}

		audit.State = "answered"
		audit.Checked = govSQLConditionsChecked(facts)
		return finish(govSQLAnswerResponse(req, validated, result, facts, absence, audit))
	}
	return decline("no attempt produced a verified query")
}

func govSQLNameObligation(name string) govSQLObligation {
	return govSQLObligation{
		Kind: "NAME", Subject: name,
		Reason: fmt.Sprintf("I could not tie %q to any field: these records hold numbers, plates, addresses and codes, not personal names", name),
	}
}

// govSQLResultEmpty is true for no rows, and for the single zero or NULL an aggregate
// returns over nothing.
func govSQLResultEmpty(result *govSQLResult) bool {
	if len(result.Rows) == 0 {
		return true
	}
	if len(result.Rows) == 1 {
		for _, value := range result.Rows[0] {
			switch v := value.(type) {
			case nil:
			case int64:
				if v != 0 {
					return false
				}
			case int32:
				if v != 0 {
					return false
				}
			case float64:
				if v != 0 {
					return false
				}
			case string:
				if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err != nil || f != 0 {
					return false
				}
			default:
				return false
			}
		}
		return true
	}
	return false
}

func govSQLConditionsChecked(facts govSQLQuestionFacts) []string {
	out := []string{}
	for _, id := range facts.Identifiers {
		out = append(out, "identifier "+id)
	}
	for _, v := range facts.ValueFacts {
		out = append(out, fmt.Sprintf("%s = %s", v.Column, v.Value))
	}
	if facts.Magnitude != "" {
		out = append(out, facts.Magnitude)
	}
	if len(facts.TimeCues) > 0 {
		out = append(out, "time: "+strings.Join(facts.TimeCues, ", "))
	}
	return out
}

// ---------------------------------------------------------------- the search for an identifier

type govSQLHit struct {
	View   *govSQLView
	Column string
	Rows   int64
}

// govSQLProbeIdentifiers searches EVERY identifier column of EVERY view for each
// identifier, so that "nothing matched" is only ever said after a real search. It is
// server-built SQL with the identifier bound as a parameter; no model text is in it.
func govSQLProbeIdentifiers(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, all []*govSQLView, ids []string) (map[string][]govSQLHit, error) {
	hits := map[string][]govSQLHit{}
	for _, id := range ids {
		var args []any
		ctes := []string{}
		unions := []string{}
		type slot struct {
			view   *govSQLView
			column string
		}
		slots := []slot{}
		for _, view := range all {
			cols := []string{}
			seen := map[string]int{}
			for _, c := range view.Columns {
				seen[c.Name]++
			}
			for _, c := range view.Columns {
				// A name the view holds twice cannot be referenced, and provenance columns
				// hold file names, never an identifier.
				if seen[c.Name] > 1 || c.Name == "source_file" || c.Name == "record_id" || c.Name == "artifact_id" || c.Name == "evidence_id" {
					continue
				}
				// Every free-text column is searched, not only those flagged as identifiers: an
				// address held in a column the curator did not flag (an IPDR destination) must
				// not make "nothing matched" untrue. Enumerated columns hold category words,
				// never an identifier, so they are skipped.
				if c.Type == "text" && (c.Identifier || (!view.binding.Derived && len(c.Values) == 0)) {
					cols = append(cols, c.Name)
				}
			}
			if len(cols) == 0 {
				continue
			}
			cte, next, err := govSQLCTE(view, req, args)
			if err != nil {
				return nil, err
			}
			args = next
			ctes = append(ctes, cte)
			digits := govSQLDigits(id)
			for _, col := range cols {
				var predicate string
				if len(digits) >= 7 {
					tail := digits
					if len(tail) > 10 {
						tail = tail[len(tail)-10:]
					}
					args = append(args, tail)
					predicate = fmt.Sprintf("RIGHT(REGEXP_REPLACE(%s, '[^0-9]', '', 'g'), 10) = $%d", col, len(args))
				} else {
					args = append(args, strings.ToLower(id))
					predicate = fmt.Sprintf("LOWER(%s) = $%d", col, len(args))
				}
				unions = append(unions, fmt.Sprintf("SELECT %d AS slot, COUNT(*) AS n FROM %s WHERE %s", len(slots), view.Name, predicate))
				slots = append(slots, slot{view, col})
			}
		}
		if len(unions) == 0 {
			continue
		}
		final := "WITH " + strings.Join(ctes, ", ") + "\n" + strings.Join(unions, "\nUNION ALL\n")
		result, err := govSQLRun(ctx, db, req.TenantID, final, args, 5000, 0)
		if err != nil {
			return nil, err
		}
		for _, row := range result.Rows {
			index, n := int(int64FromAny(row[0])), int64FromAny(row[1])
			if n > 0 && index >= 0 && index < len(slots) {
				hits[id] = append(hits[id], govSQLHit{View: slots[index].view, Column: slots[index].column, Rows: n})
			}
		}
	}
	return hits, nil
}

type govSQLProbeVerdict struct {
	id               string
	elsewhereView    string
	elsewhereColumn  string
	elsewhereRows    int64
	sameViewColumns  string
	absentEverywhere bool
	searched         string
}

// govSQLReadProbe says where an identifier that produced nothing actually lives.
func govSQLReadProbe(hits map[string][]govSQLHit, ids []string, validated *govSQLValidated) govSQLProbeVerdict {
	used := map[string]bool{}
	for _, c := range validated.Columns {
		used[c] = true
	}
	verdict := govSQLProbeVerdict{}
	foundAny := false
	for _, id := range ids {
		list := hits[id]
		if len(list) == 0 {
			continue
		}
		foundAny = true
		sameView, other := []string{}, []govSQLHit{}
		usedHere := false
		for _, hit := range list {
			if hit.View.Name == validated.View.Name {
				if used[hit.Column] {
					usedHere = true
				} else {
					sameView = append(sameView, hit.Column)
				}
			} else {
				other = append(other, hit)
			}
		}
		switch {
		case usedHere:
			// It is where the query looked; the OTHER conditions excluded every row.
		case len(sameView) > 0:
			verdict.id, verdict.sameViewColumns = id, strings.Join(sameView, ", ")
			return verdict
		case len(other) > 0:
			sort.Slice(other, func(i, j int) bool { return other[i].Rows > other[j].Rows })
			verdict.id = id
			verdict.elsewhereView, verdict.elsewhereColumn, verdict.elsewhereRows = other[0].View.Display, other[0].Column, other[0].Rows
			return verdict
		}
	}
	if !foundAny {
		verdict.absentEverywhere = true
		verdict.searched = "every identifier field of the structured evidence (records and media metadata). Text inside recordings, images and documents is searched by a different path and was not searched here"
	}
	return verdict
}

// ---------------------------------------------------------------- responses

func govSQLHumanize(alias string) string {
	words := strings.Fields(strings.ReplaceAll(strings.TrimSpace(alias), "_", " "))
	if len(words) == 0 {
		return "Result"
	}
	words[0] = strings.ToUpper(words[0][:1]) + words[0][1:]
	return strings.Join(words, " ")
}

var govSQLRxNumeric = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

func govSQLFormatValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "no value (nothing to compute over)"
	case bool:
		return strconv.FormatBool(v)
	case int:
		return govSQLGroup(strconv.FormatInt(int64(v), 10))
	case int32:
		return govSQLGroup(strconv.FormatInt(int64(v), 10))
	case int64:
		return govSQLGroup(strconv.FormatInt(v, 10))
	case float64:
		return govSQLFormatNumber(strconv.FormatFloat(v, 'f', -1, 64))
	case string:
		text := strings.TrimSpace(v)
		if govSQLRxNumeric.MatchString(text) {
			return govSQLFormatNumber(text)
		}
		if t, err := time.Parse(time.RFC3339, text); err == nil {
			return t.UTC().Format("2006-01-02 15:04:05") + " UTC"
		}
		return v
	default:
		return fmt.Sprint(v)
	}
}

// govSQLFormatNumber groups thousands and rounds (never truncates) a long fraction to four places.
func govSQLFormatNumber(text string) string {
	if !strings.ContainsAny(text, ".") {
		return govSQLGroup(text)
	}
	if rat, ok := new(big.Rat).SetString(text); ok {
		text = rat.FloatString(4)
	}
	whole, frac := text, ""
	if i := strings.Index(text, "."); i >= 0 {
		whole, frac = text[:i], text[i+1:]
	}
	frac = strings.TrimRight(frac, "0")
	if frac == "" {
		return govSQLGroup(whole)
	}
	return govSQLGroup(whole) + "." + frac
}

// govSQLFormatCell formats one result value. A value in a text column of the view (a phone
// number, an account, a plate) is an identifier: it is shown exactly as stored, never
// grouped as if it were a quantity.
func govSQLFormatCell(view *govSQLView, column string, value any) string {
	if view != nil {
		for _, c := range view.Columns {
			if c.Name == column && c.Type == "text" {
				if text, ok := value.(string); ok {
					if label := c.Labels[text]; label != "" {
						return label
					}
					return text
				}
			}
		}
	}
	return govSQLFormatValue(value)
}

func govSQLGroup(digits string) string {
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	for i := len(digits) - 3; i > 0; i -= 3 {
		digits = digits[:i] + "," + digits[i:]
	}
	return sign + digits
}

func govSQLHeadline(view *govSQLView, result *govSQLResult, facts govSQLQuestionFacts, absence string, ids []string) string {
	if absence != "" && len(ids) > 0 {
		return fmt.Sprintf("No record in this case contains %s: I searched %s.", strings.Join(ids, " or "), absence)
	}
	switch {
	case len(result.Rows) == 0:
		return "The query matched no rows."
	case len(result.Rows) == 1 && len(result.Columns) == 1:
		return fmt.Sprintf("%s: %s.", govSQLHumanize(result.Columns[0]), govSQLFormatCell(view, result.Columns[0], result.Rows[0][0]))
	case len(result.Rows) == 1:
		parts := make([]string, 0, len(result.Columns))
		for i, column := range result.Columns {
			parts = append(parts, fmt.Sprintf("%s: %s", govSQLHumanize(column), govSQLFormatCell(view, column, result.Rows[0][i])))
		}
		return strings.Join(parts, "; ") + "."
	case len(result.Columns) == 2 && len(result.Rows) <= 12 && !facts.WantsSingle:
		// A short two-column result is a breakdown: say all of it, in the order the query gave,
		// instead of naming the first row "top".
		parts := make([]string, 0, len(result.Rows))
		for _, row := range result.Rows {
			parts = append(parts, fmt.Sprintf("%s: %s", govSQLFormatCell(view, result.Columns[0], row[0]), govSQLFormatCell(view, result.Columns[1], row[1])))
		}
		return fmt.Sprintf("%d rows (%s by %s). In full: %s.", len(result.Rows), strings.ToLower(govSQLHumanize(result.Columns[1])), strings.ToLower(govSQLHumanize(result.Columns[0])), strings.Join(parts, "; "))
	default:
		first := result.Rows[0]
		lead := ""
		if len(result.Columns) >= 2 && !facts.WantsSingle {
			lead = fmt.Sprintf("Top result: %s = %s (%s: %s). ", result.Columns[0], govSQLFormatCell(view, result.Columns[0], first[0]), strings.ToLower(govSQLHumanize(result.Columns[len(result.Columns)-1])), govSQLFormatCell(view, result.Columns[len(result.Columns)-1], first[len(first)-1]))
		}
		more := ""
		if result.Truncated {
			more = fmt.Sprintf(" (showing the first %d)", len(result.Rows))
		}
		return fmt.Sprintf("%s%d rows%s.", lead, len(result.Rows), more)
	}
}

func govSQLRowMaps(result *govSQLResult) []map[string]any {
	rows := make([]map[string]any, 0, len(result.Rows))
	for _, row := range result.Rows {
		item := make(map[string]any, len(result.Columns))
		for i, column := range result.Columns {
			if i < len(row) {
				item[column] = row[i]
			}
		}
		rows = append(rows, item)
	}
	return rows
}

func govSQLAnswerResponse(req hybridQueryRequest, validated *govSQLValidated, result *govSQLResult, facts govSQLQuestionFacts, absence string, audit GovSQLAuditV1) *hybridQueryResponse {
	view := validated.View
	headline := govSQLHeadline(view, result, facts, absence, facts.Identifiers)
	limitations := []string{
		fmt.Sprintf("Computed by a model-written query over %s (%s), checked against the question and run read-only. The query is shown under derivation.", view.Display, view.Name),
	}
	if len(audit.Checked) > 0 {
		limitations = append(limitations, "The query was checked to apply: "+strings.Join(audit.Checked, "; ")+".")
	}
	if len(facts.TimeCues) > 0 {
		// Several columns of a view can hold a time (an event time, a call start). Which one the
		// condition used is part of the answer.
		used := map[string]bool{}
		for _, name := range validated.Columns {
			used[name] = true
		}
		named := []string{}
		for _, column := range view.Columns {
			if used[column.Name] && (column.Type == "timestamptz" || column.Type == "date") {
				named = append(named, fmt.Sprintf("%s (%s)", column.Display, column.Name))
			}
		}
		if len(named) > 0 {
			limitations = append(limitations, "The time condition was applied to: "+strings.Join(named, ", ")+".")
		}
	}
	if facts.TimeOfDay {
		limitations = append(limitations, "A time of day uses the hours as recorded at the source; 'night' was taken as 00:00 to 05:59.")
	}
	if result.Truncated {
		limitations = append(limitations, fmt.Sprintf("Only the first %d rows are shown; the query matched more.", len(result.Rows)))
	}
	rows := govSQLRowMaps(result)
	source := "records_sql"
	if view.binding.Derived {
		source = "derived_artifacts_sql"
	}
	answer := map[string]any{
		"result_state": "COMPLETED", "answer": headline, "grounding": "GOVERNED_SQL",
		"governed_sql": map[string]any{"view": view.Name, "family": view.Family, "sql": validated.SQL, "columns": result.Columns, "row_count": len(result.Rows)},
	}
	enterprise := map[string]any{
		"status": "completed", "result_state": "COMPLETED", "executive_answer": headline,
		"summary": limitations[0], "limitations": limitations,
		"data_grid": enterpriseDataGrid(rows), "row_count": len(rows),
		"provenance": []map[string]any{{
			"source": source, "kind": "governed_sql", "view": view.Name, "record_type": view.RecordType,
			"table": view.binding.Table, "sql": validated.SQL, "row_count": len(rows),
		}},
		"metrics": govSQLMetrics(validated.View, result),
		"operation": map[string]any{
			"template": "governed_sql", "operation_id": "forensics.governed_sql",
			"family_id": view.Family, "source_access": source,
		},
		"coverage": map[string]any{
			"tenant_id": req.TenantID, "collection_id": req.CollectionID,
			"route": []string{"governed_sql"}, "queried_record_sql": !view.binding.Derived, "queried_kb": false,
		},
		"derivation": map[string]any{"sql": validated.SQL, "view": view.Name, "conditions_checked": audit.Checked},
	}
	return &hybridQueryResponse{
		CollectionID: req.CollectionID, RequestClass: forensicrequest.GovernedAnalysis, Intent: intentSemantic,
		Route: []string{"governed_sql"}, Policy: "governed_sql_lane",
		Planner:   map[string]any{"contract_version": forensicrequest.ContractV1, "request_class": forensicrequest.GovernedAnalysis},
		QueryPlan: QueryPlan{}, Capability: queryCapabilityAssessment{Status: "supported"},
		Answer: answer, GeneratedAt: time.Now().UTC(), Enterprise: enterprise,
		Telemetry: QueryTelemetry{TotalLatencyMS: audit.TotalMS},
	}
}

func govSQLMetrics(view *govSQLView, result *govSQLResult) []map[string]any {
	if len(result.Rows) != 1 {
		return []map[string]any{}
	}
	metrics := make([]map[string]any, 0, len(result.Columns))
	for i, column := range result.Columns {
		metrics = append(metrics, map[string]any{"label": govSQLHumanize(column), "value": govSQLFormatCell(view, column, result.Rows[0][i]), "source": "governed_sql"})
	}
	return metrics
}

// govSQLClarification builds a response that asks for what is missing and states
// plainly what was and was not done.
func govSQLClarification(req hybridQueryRequest, text, limitation string, audit GovSQLAuditV1) *hybridQueryResponse {
	answer := map[string]any{"result_state": "CLARIFICATION_REQUIRED", "clarification": text, "grounding": "GOVERNED_SQL"}
	enterprise := terminalEnterprisePayload(req, forensicrequest.Clarify, answer)
	enterprise["limitations"] = []string{limitation}
	enterprise["summary"] = limitation
	enterprise["derivation"] = map[string]any{"views": audit.Views}
	return &hybridQueryResponse{
		CollectionID: req.CollectionID, RequestClass: forensicrequest.GovernedAnalysis, Intent: intentClarify,
		Route: []string{"governed_sql", "clarification"}, Policy: "governed_sql_lane",
		Planner:   map[string]any{"contract_version": forensicrequest.ContractV1, "request_class": forensicrequest.GovernedAnalysis},
		QueryPlan: QueryPlan{}, Capability: queryCapabilityAssessment{Status: "clarification"},
		Answer: answer, GeneratedAt: time.Now().UTC(), Enterprise: enterprise,
		Telemetry: QueryTelemetry{TotalLatencyMS: audit.TotalMS},
	}
}

func govSQLAbstainResponse(req hybridQueryRequest, offered []*govSQLView, facts govSQLQuestionFacts, unmet []govSQLObligation, audit GovSQLAuditV1) *hybridQueryResponse {
	reasons := make([]string, 0, len(unmet))
	for _, item := range unmet {
		reasons = append(reasons, item.Reason)
	}
	display := "the evidence"
	if len(offered) > 0 {
		display = offered[0].Display
	}
	text := "I did not run this question, because I could not apply everything it asks: " + strings.Join(reasons, "; ") + "."
	hasName := false
	for _, item := range unmet {
		hasName = hasName || item.Kind == "NAME"
	}
	if hasName {
		text += " Name the number, plate, address or code you mean, for example \"calls made by 923001110001\"."
	} else if len(offered) > 0 {
		text += fmt.Sprintf(" Rephrase it with the exact field and value; the fields of %s are: %s.", display, offered[0].columnNames(14))
	}
	return govSQLClarification(req, text, "No result is shown, because a number that ignores part of the question would be a wrong answer. Nothing was inferred.", audit)
}

func govSQLElsewhereResponse(req hybridQueryRequest, offered, all []*govSQLView, verdict govSQLProbeVerdict, audit GovSQLAuditV1) *hybridQueryResponse {
	here := "this evidence family"
	if len(offered) > 0 {
		here = offered[0].Display
	}
	text := fmt.Sprintf("%s does not appear in %s, but it does appear in %s (%s, %s rows). Ask about %s instead.",
		verdict.id, here, verdict.elsewhereView, verdict.elsewhereColumn, govSQLGroup(strconv.FormatInt(verdict.elsewhereRows, 10)), verdict.elsewhereView)
	return govSQLClarification(req, text, "I searched every identifier field of the structured evidence before saying this. No count of zero is reported, because the value exists elsewhere in the case.", audit)
}

// ---------------------------------------------------------------- the headers and the fallback

func setGovernedSQLHeader(w http.ResponseWriter, audit GovSQLAuditV1) {
	if w == nil || audit.State == "" {
		return
	}
	value := fmt.Sprintf("state=%s; attempts=%d; views=%s; model_ms=%d; exec_ms=%d; total_ms=%d", audit.State, audit.Attempts, strings.Join(audit.Views, ","), audit.ModelMS, audit.ExecMS, audit.TotalMS)
	if audit.Reason != "" {
		value += "; reason=" + strings.ReplaceAll(govSQLClip(audit.Reason, 120), ";", ",")
	}
	w.Header().Set("X-Governed-SQL", value)
}
