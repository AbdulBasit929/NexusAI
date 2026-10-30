package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensicrequest"
	"github.com/mudler/LocalAI/pkg/httpclient"
)

// ASK ENGINE V2: THE RUN (roadmap 6.8.2).
//
// One question goes through: eligibility, the frame (ask_engine_frame.go), the shortlist and the retrieved examples (ask_engine_retrieve.go), the prompt
// (ask_engine_catalogue.go), the model, the validator (ask_engine_validate.go), the obligations, the executor (ask_engine_exec.go), the checks on the result,
// and the answer built from the result by code. The model writes one query and nothing else: it sees no case data, composes no sentence and computes no
// number (the composition under the number lock is milestone M5).
//
// The switches (all default off; the engine changes nothing until one is on):
//
//	FORENSIC_ASK_V2=true         run the engine AFTER the existing path declines (and before the governed SQL lane when that is on): an answer the existing path
//	                             gave is never replaced
//	FORENSIC_ASK_V2_FIRST=true   run it BEFORE the existing path (what the measurement of the engine's own accuracy uses)
//	FORENSIC_ASK_V2_AGREE=true   write two independent drafts and answer only when their results are equal (doubles the model time)
//
// A decline hands the question to the next path unchanged; an abstention is an answer ("not answered, and why") that the user sees. The engine abstains,
// rather than declines, when it HAS a query but cannot show that the query answers THIS question, because the existing path would answer a different one.
const (
	askV2Env         = "FORENSIC_ASK_V2"
	askV2FirstEnv    = "FORENSIC_ASK_V2_FIRST"
	askV2AgreeEnv    = "FORENSIC_ASK_V2_AGREE"
	askMaxTokens     = 480
	askMaxAttempts   = 3 // the first draft and at most two repairs
	askModelTimeout  = 240 * time.Second
	askRepairBudget  = 170 * time.Second // no further repair is started after this much time: the laptop writes about five tokens a second
	askCatalogueTTL  = 5 * time.Minute
	askAnswerPolicy  = "ask_engine_v2"
	askAnswerGround  = "ASK_ENGINE_V2"
	askHeaderName    = "X-Ask-Engine-V2"
	askHeadlineItems = 5
)

// askQuestionBudget is the most time the engine spends waiting for the model on ONE question, over all its attempts: a call that would end after it is cut at it, and no call is started with
// less than askMinModelCall left. Without it the worst case was the repair budget plus one more full call (about 410 s), and a stalled model service held the analyst for as long as the
// call timeout (the evaluation harness allows ten minutes). Variables so that a specification can shorten them; the question then goes to the existing path, as for any decline.
var (
	askQuestionBudget = 300 * time.Second
	askMinModelCall   = 20 * time.Second
)

func askEnvTrue(name string) bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(name)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose)
}

func askV2Enabled() bool      { return askEnvTrue(askV2Env) || askV2FirstEnabled() }
func askV2FirstEnabled() bool { return askEnvTrue(askV2FirstEnv) }
func askV2AgreeEnabled() bool { return askEnvTrue(askV2AgreeEnv) }

// AskAuditV2 is what the engine records about a question it looked at: on the response, in the header and in the log. It holds no row of the case.
type AskAuditV2 struct {
	State       string   `json:"state"` // answered | abstained | declined
	Reason      string   `json:"reason,omitempty"`
	Views       []string `json:"views,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	Attempts    int      `json:"attempts"`
	SQL         string   `json:"sql,omitempty"`
	Checks      []string `json:"checks,omitempty"`
	Repairs     []string `json:"repairs,omitempty"` // the kind of each thing that had to be repaired, in order
	Shape       string   `json:"shape,omitempty"`
	Rows        int      `json:"rows"`
	Truncated   bool     `json:"truncated,omitempty"`
	Hops        int      `json:"hops,omitempty"`
	PlanCost    float64  `json:"plan_cost,omitempty"`
	Agreement   string   `json:"agreement,omitempty"` // "", agreed or disagreed
	Model       string   `json:"model,omitempty"`
	PromptBytes int      `json:"prompt_bytes,omitempty"`
	PromptID    string   `json:"prompt_id,omitempty"`
	ModelMS     int64    `json:"model_ms"`
	ExecMS      int64    `json:"exec_ms"`
	TotalMS     int64    `json:"total_ms"`
	// RepeatsLeftOut is how many exact repeated rows of the case's call and internet records the views leave out; an answer over them says so.
	RepeatsLeftOut int64 `json:"repeats_left_out,omitempty"`
}

func (a *AskAuditV2) repaired(kinds ...string) {
	a.Repairs = append(a.Repairs, kinds...)
}

// ---- the model -------------------------------------------------------------------------------------------------------------------------------------------------

type askModelFunc func(ctx context.Context, cfg config, model string, turns []govSQLTurn) (content string, failure string)

// askModel is a variable only so that the specs can stand in a scripted model without a network.
var askModel askModelFunc = askModelHTTP

func askResponseSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"answerable": map[string]any{"type": "boolean"},
			"sql":        map[string]any{"type": "string"},
		},
		"required":             []string{"answerable", "sql"},
		"additionalProperties": false,
	}
}

// askModelHTTP asks the LocalAI endpoint as the lane does: a strict JSON schema, temperature 0, a hard cap on output tokens (the output length IS the latency
// on the laptop), and a completion that must have stopped by itself.
func askModelHTTP(ctx context.Context, cfg config, model string, turns []govSQLTurn) (string, string) {
	messages := make([]map[string]string, 0, len(turns))
	for _, turn := range turns {
		messages = append(messages, map[string]string{"role": turn.Role, "content": turn.Content})
	}
	payload, _ := json.Marshal(map[string]any{
		"model": model, "temperature": 0, "max_tokens": askMaxTokens,
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "forensic_ask_engine_v2", "strict": true, "schema": askResponseSchema()}},
		"messages":        messages,
	})
	timeout := askModelTimeout
	if cfg.SynthesisTimeout > timeout {
		timeout = cfg.SynthesisTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(callCtx, http.MethodPost, strings.TrimRight(cfg.LocalAIURL, "/")+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", "REQUEST_FAILED"
	}
	request.Header.Set("Content-Type", "application/json")
	if cfg.LocalAIAPIKey != "" {
		request.Header.Set("Authorization", "Bearer "+cfg.LocalAIAPIKey)
	}
	response, err := httpclient.NewWithTimeout(timeout).Do(request)
	if err != nil {
		return "", "TIMEOUT_OR_UNAVAILABLE"
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Sprintf("HTTP_%d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 || !utf8.Valid(raw) {
		return "", "MALFORMED_TRANSPORT"
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if json.Unmarshal(raw, &envelope) != nil || len(envelope.Choices) != 1 {
		return "", "MALFORMED_ENVELOPE"
	}
	if envelope.Choices[0].FinishReason != "stop" {
		return "", "INCOMPLETE_COMPLETION"
	}
	return envelope.Choices[0].Message.Content, ""
}

// ---- the catalogue and the seams the specs replace -----------------------------------------------------------------------------------------------

var askCatalogueCache struct {
	sync.Mutex
	cat *askCatalogue
	at  time.Time
}

// askCatalogueFor is the catalogue of the analyst views of the connected database, read from its own comments at most every five minutes. The engine
// declines, and says why, when the views are not installed (migration 017) or are not fully described.
func askCatalogueFor(ctx context.Context, db *pgxpool.Pool) (*askCatalogue, error) {
	askCatalogueCache.Lock()
	defer askCatalogueCache.Unlock()
	if askCatalogueCache.cat != nil && time.Since(askCatalogueCache.at) < askCatalogueTTL {
		return askCatalogueCache.cat, nil
	}
	cat, err := loadAskCatalogue(ctx, db)
	if err != nil {
		return nil, err
	}
	askCatalogueCache.cat, askCatalogueCache.at = cat, time.Now()
	return cat, nil
}

var (
	// askV2Run is how the handler enters the engine; a variable only so that a spec can stand in for it.
	askV2Run           = runAskEngineV2
	askCatalogueLoader = askCatalogueFor
	askExec            = askExecute
	askProbeFn         = askProbe
)

// ---- eligibility -------------------------------------------------------------------------------------------------------------------------------------------

func askEligible(req hybridQueryRequest) (bool, string) {
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
	return true, ""
}

// ---- the prompt's question turn -------------------------------------------------------------------------------------------------------------------

// askUserPrompt is the question (with every number in its canonical form) and the hints that say what the checks will demand, so that the first draft is the
// one that passes. The hints come from the frame by code.
func askUserPrompt(f askFrame) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Q: %s\n", f.Question)
	var hints []string
	if ids := f.Identifiers(); len(ids) > 0 {
		hints = append(hints, "Use these exactly, as text constants, and no other numbers: "+strings.Join(ids, ", ")+".")
	}
	if len(f.Plates) > 0 {
		hints = append(hints, "Plates: compare plate_norm with "+strings.Join(f.Plates, ", ")+" exactly (the plate in capitals, no separators), never plate_raw; a plate that looks alike (O for 0) is another plate.")
	}
	if len(f.Dates) > 0 {
		hints = append(hints, "Dates: "+strings.Join(f.Dates, ", ")+".")
	}
	for _, m := range f.Months {
		hints = append(hints, fmt.Sprintf("Month %d%s: compare the time with its first day and the first day of the next month.", m.Month, askYearNote(m.Year)))
	}
	if f.Limit > 0 {
		hints = append(hints, fmt.Sprintf("Keep exactly %d rows (LIMIT %d).", f.Limit, f.Limit))
	}
	if f.Hops > 0 {
		hints = append(hints, fmt.Sprintf("Recursion: depth < %d.", f.Hops))
	}
	if len(f.IMEIs) > 0 {
		// Measured on the judgement run of 2026-10-19 (case 104): an export that stores the 14 digits of an IMEI, asked with its 15 digits, was answered "no record". Both forms are one handset.
		var forms []string
		for _, imei := range f.IMEIs {
			forms = append(forms, fmt.Sprintf("imei IN ('%s', '%s')", imei, askIMEI14(imei)))
		}
		hints = append(hints, "Handsets: an IMEI is stored with its 15 digits or, by some exports, with 14 (no check digit); both are one handset, so compare "+strings.Join(forms, " and ")+".")
	}
	if (f.Hops > 0 || f.Cues["path"]) && len(f.Numbers) >= 2 {
		// Measured on seeds 101 and 103: asked "is there a link between A and B within N hops", the first draft was an existence test with no outer LIMIT, which used up the
		// repairs (two answers took 190 s, two were declined) or answered with a bare yes or no. The answer to a connection question is the connection, as the example's.
		hints = append(hints, "Connection: return the edges of the routes between the two numbers as src and dst, one row per edge, as in the verified example (never a yes or no), and end with LIMIT 200. With v_connection_edges: a = first number AND b = second number, then AND ordinary AND rank_ordinary = 1 when services and hubs are left out, else AND rank_any = 1, and hops <= N when fewer than three hops are asked.")
	}
	if f.Cues["incoming"] {
		hints = append(hints, "Incoming events only.")
	}
	if f.Cues["outgoing"] {
		hints = append(hints, "Outgoing events only.")
	}
	for _, kind := range []string{"sms", "mms", "ussd"} {
		if f.Cues[kind] {
			hints = append(hints, "Event type "+kind+".")
		}
	}
	if f.Cues["night"] {
		hints = append(hints, "Night is the hours 0 to 5 of the case clock.")
	}
	if len(hints) > 0 {
		b.WriteString("Hints: " + strings.Join(hints, " ") + "\n")
	}
	return b.String()
}

// askReplaceView swaps one view of a shortlist for another (once, keeping no duplicate).
func askReplaceView(views []string, from, to string) []string {
	out := make([]string, 0, len(views))
	seen := map[string]bool{}
	for _, v := range views {
		if v == from {
			v = to
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// ---- the run -----------------------------------------------------------------------------------------------------------------------------------------------

func runAskEngineV2(ctx context.Context, cfg config, db *pgxpool.Pool, req hybridQueryRequest, startedAt time.Time) (*hybridQueryResponse, AskAuditV2) {
	audit := AskAuditV2{State: "declined"}
	engineStart := time.Now() // the repair budget counts the engine's own time, not what the existing path spent before it declined
	var frame askFrame
	var digest string
	finish := func(resp *hybridQueryResponse) (*hybridQueryResponse, AskAuditV2) {
		if resp != nil && audit.State == "declined" {
			audit.State = "abstained"
		}
		audit.TotalMS = time.Since(startedAt).Milliseconds()
		askAuditSink(ctx, db, req, audit, frame, digest)
		if resp != nil {
			resp.Telemetry.TotalLatencyMS = audit.TotalMS
			if resp.Planner != nil {
				resp.Planner["ask_engine_v2"] = audit
			}
		}
		return resp, audit
	}
	decline := func(reason string) (*hybridQueryResponse, AskAuditV2) {
		audit.State, audit.Reason = "declined", reason
		return finish(nil)
	}
	if ok, reason := askEligible(req); !ok {
		// Not a question for the engine: the interface's own template and structured requests (its summaries, its activity cards), a general-knowledge question, a request with no
		// scope. Nothing was evaluated, so nothing is audited: the live table had grown by 347 such rows in two days, two thirds of all of its rows, and hid the real questions.
		audit.State, audit.Reason = "declined", reason
		return nil, audit
	}
	if db == nil {
		return decline("no database")
	}
	cat, err := askCatalogueLoader(ctx, db)
	if err != nil {
		return decline("the analyst views are not available: " + govSQLClip(err.Error(), 120))
	}
	examples, err := askExamples()
	if err != nil {
		return decline("the verified examples are not valid")
	}
	frame = askExtractFrameWith(req.Query, cat.View("v_plate_sightings") != nil)
	if len(frame.NationalIDs) > 0 {
		// identity data is withheld until the masking design is approved (R9): the question is not run and no other path is asked, so the number is neither searched nor shown
		audit.Reason = "identity: national ID number"
		return finish(askClarification(req, askNationalIDAnswer, "No result is shown, because national ID numbers are withheld. Nothing was inferred.", audit, nil))
	}
	if frame.Relative != "" {
		// "last week" has no meaning without a reference day, and a model that is not given one invents it (measured 2026-10-14). The question is not run.
		return decline("the question uses a relative time (" + frame.Relative + ") and no reference day is stated")
	}
	short := cat.Shortlist(frame.Question, askShortlistMax)
	if len(short.Views) == 0 || !short.Confident {
		return decline("no analyst view fits the question closely enough")
	}
	chosen := append([]string{}, short.Views...)
	if frame.Hops > 3 {
		// v_connection_edges walks up to three links: a question for more is answered by the recursive query over the contact pairs
		chosen = askReplaceView(chosen, "v_connection_edges", "v_contact_pairs")
	}
	sort.Strings(chosen)
	audit.Views = chosen
	// Runtime parameters (a window of 20 minutes, a gap of 30): read from the question by code, never by the model, within their bounds. A question that names one twice with
	// different values, names minutes without a number, or asks for a value outside the bounds gets a question back, not a guess (ask_engine_params.go).
	paramUses, paramIssue := askResolveParams(frame, chosen)
	if paramIssue != nil {
		audit.Reason = "parameter: " + paramIssue.Kind
		return finish(askClarification(req, paramIssue.Message, "No result is shown, because the question names a number the analysis cannot use as it stands. Nothing was inferred.", audit, nil))
	}
	frame.ParamUses = paramUses
	// The views read the typed tables only: a case that is not (fully) in them gets no answer from the engine (ask_engine_coverage.go).
	coverage, err := askCoverageFor(ctx, db, req.TenantID, req.CollectionID)
	if err != nil {
		return decline("could not read the case's coverage of the typed tables")
	}
	if gap := askCoverageGap(coverage, chosen); gap != "" {
		return decline("the analyst views cannot speak for this case: " + gap)
	}
	if gap := askVolumeGap(coverage, frame, chosen); gap != "" {
		return decline("the analyst views cannot speak for this case: " + gap)
	}
	if gap := askTimeOfDayGap(coverage, frame, chosen); gap != "" {
		return decline("the analyst views cannot speak for this case: " + gap)
	}
	shown := askRetrieveExamples(frame.Question, chosen, examples, 2)
	for _, e := range shown {
		audit.Examples = append(audit.Examples, e.ID)
	}
	model := strings.TrimSpace(req.SynthesisModel)
	if model == "" {
		model = cfg.SynthesisModel
	}
	audit.Model = model
	system := askPrompt(cat, append([]string{}, chosen...), shown)
	turns := []govSQLTurn{{Role: "system", Content: system}, {Role: "user", Content: askUserPrompt(frame)}}
	audit.PromptBytes = len(turns[0].Content) + len(turns[1].Content)
	audit.PromptID = govSQLDigest(turns[:1])
	limits := askDefaultLimits()
	limits.Settings = askParamSettings(paramUses)

	retry := func(raw string, problems ...string) {
		turns = append(turns, govSQLTurn{Role: "assistant", Content: raw}, govSQLTurn{Role: "user", Content: govSQLRetryPrompt(problems)})
	}
	for attempt := 1; attempt <= askMaxAttempts; attempt++ {
		audit.Attempts = attempt
		last := attempt == askMaxAttempts || time.Since(engineStart) > askRepairBudget
		remaining := askQuestionBudget - time.Since(engineStart)
		if remaining < askMinModelCall {
			return decline("the question budget is spent")
		}
		modelStart := time.Now()
		callCtx, cancelCall := context.WithTimeout(ctx, remaining)
		raw, failure := askModel(callCtx, cfg, model, turns)
		cancelCall()
		audit.ModelMS += time.Since(modelStart).Milliseconds()
		if failure != "" {
			return decline("model: " + failure)
		}
		answerable, sql, bad := govSQLParseOutput(raw)
		if bad != "" {
			if last {
				return decline("the model's output was not the requested JSON")
			}
			audit.repaired("JSON")
			retry(raw, "Return exactly one JSON object with answerable and sql.")
			continue
		}
		if !answerable || sql == "" {
			return decline("the model judged that the views cannot answer the question")
		}
		validated, rejection := askValidate(sql, cat, chosen)
		if rejection != nil {
			audit.SQL = govSQLClip(sql, 400)
			if last {
				return decline("query rejected: " + rejection.Code)
			}
			audit.repaired("VALIDATOR:" + rejection.Code)
			retry(raw, rejection.Message)
			continue
		}
		audit.SQL, audit.Hops = validated.SQL, validated.MaxHops
		audit.RepeatsLeftOut = coverage.Repeats
		if gap := askPopulationGap(coverage, validated); gap != "" {
			return decline("the analyst views cannot speak for this case: " + gap)
		}

		if unmet := askCheckFrame(frame, validated); len(unmet) > 0 {
			problems := make([]string, 0, len(unmet))
			for _, item := range unmet {
				problems = append(problems, item.Message)
				audit.repaired("OBLIGATION:" + item.Kind)
			}
			if last {
				return finish(askAbstainResponse(req, cat, frame, validated, unmet, audit))
			}
			retry(raw, problems...)
			continue
		}

		execStart := time.Now()
		result, err := askExec(ctx, db, req.TenantID, req.CollectionID, validated, limits)
		audit.ExecMS += time.Since(execStart).Milliseconds()
		if err != nil {
			run, _ := err.(*govSQLRunError)
			code := "DATABASE"
			if run != nil {
				code = run.Code
			}
			switch code {
			case "QUERY_ERROR", "TOO_EXPENSIVE", "TIMEOUT":
				if last {
					if code == "QUERY_ERROR" {
						return decline("execution: " + code)
					}
					return finish(askTooBroadResponse(req, run, audit))
				}
				audit.repaired("EXECUTION:" + code)
				retry(raw, "The database could not run it: "+run.Message)
				continue
			}
			return decline("execution: " + code)
		}
		audit.Rows, audit.Truncated, audit.PlanCost = len(result.Rows), result.Truncated, result.PlanCost

		if shape := askCheckResult(frame, validated, result.Columns, len(result.Rows)); shape != nil {
			audit.repaired("RESULT:" + shape.Kind)
			if last {
				return finish(askAbstainResponse(req, cat, frame, validated, []askObligation{*shape}, audit))
			}
			retry(raw, shape.Message)
			continue
		}

		absence := ""
		if askResultEmpty(result) {
			verdict, err := askReadEmpty(ctx, db, req, cat, frame, validated, limits)
			if err != nil {
				return decline("could not check an empty result")
			}
			switch {
			case verdict.elsewhere != "" && !last:
				audit.repaired("RESULT:EMPTY_WRONG_COLUMN")
				message := verdict.hint
				if message == "" {
					message = fmt.Sprintf("Nothing matched, but %s occurs in %s. Look there.", verdict.identifier, verdict.elsewhere)
				}
				retry(raw, message)
				continue
			case verdict.elsewhere != "":
				reason := verdict.reason
				if reason == "" {
					reason = "the query found nothing although the identifier occurs elsewhere in the case"
				}
				return finish(askAbstainResponse(req, cat, frame, validated, []askObligation{{Kind: "EMPTY", Reason: reason}}, audit))
			}
			absence = verdict.absence
		}

		if askV2AgreeEnabled() && len(result.Rows) > 0 {
			agreed, err := askAgree(ctx, cfg, model, db, req, cat, chosen, frame, examples, shown, result, limits)
			audit.Agreement = "agreed"
			if err != nil || !agreed {
				audit.Agreement = "disagreed"
				return finish(askAbstainResponse(req, cat, frame, validated, []askObligation{{Kind: "AGREEMENT", Reason: "two independent drafts of the query gave different results"}}, audit))
			}
		}

		audit.State = "answered"
		digest = askResultDigest(result)
		audit.Shape = askShape(result.Columns, len(result.Rows))
		audit.Checks = askChecksPassed(frame, validated)
		return finish(askAnswerResponse(req, cat, validated, result, frame, absence, audit))
	}
	return decline("no draft passed the checks")
}

// askResultEmpty is true when the result holds nothing: no row, or one row whose every value is a zero or a null (a COUNT or a SUM over nothing). A zero is the
// answer an analyst would take for "there is no such record", so it is read the same way: the identifiers of the question are searched for before a zero is shown
// (the project's own history: "0 internet sessions" was a confident wrong answer about a case that held 14,895).
func askResultEmpty(r *govSQLResult) bool {
	if len(r.Rows) == 0 {
		return true
	}
	if len(r.Rows) != 1 {
		return false
	}
	for _, v := range r.Rows[0] {
		switch x := v.(type) {
		case nil:
		case int:
			if x != 0 {
				return false
			}
		case int32:
			if x != 0 {
				return false
			}
		case int64:
			if x != 0 {
				return false
			}
		case float64:
			if x != 0 {
				return false
			}
		case string:
			// a number the database returned as text (a numeric sum): "0" and "0.00" are zeros
			if strings.Trim(x, "0.") != "" || x == "" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// askEmptyVerdict is what an empty result means: the identifier is nowhere in the case (so "no record" is true), or it is somewhere the query did not look.
type askEmptyVerdict struct {
	absence    string // the sentence that says what was searched, when nothing was found anywhere
	elsewhere  string // "view.column" where the identifier does occur, when the query looked elsewhere
	identifier string
	hint       string // what to tell the model, in words
	reason     string // why the answer is withheld on the last attempt, when it is not "the identifier occurs elsewhere in the case"
}

// askPairCount is what v_contact_pairs holds for one subscriber (src) and one other party (dst).
type askPairCount struct{ Events, Incoming, Outgoing int64 }

// askPairs reads v_contact_pairs for two numbers, in both orientations, scoped like every statement. A variable only so that a spec can stand in for the database.
var askPairs = func(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID, a, b string, limits askLimits) (map[[2]string]askPairCount, error) {
	result, err := askRun(ctx, db, tenantID, collectionID,
		"SELECT src, dst, events, incoming, outgoing FROM v_contact_pairs WHERE (src = $1 AND dst = $2) OR (src = $2 AND dst = $1)", []any{a, b}, limits, 4, false)
	if err != nil {
		return nil, err
	}
	out := map[[2]string]askPairCount{}
	num := func(v any) int64 {
		n, _ := strconv.ParseInt(fmt.Sprint(v), 10, 64)
		return n
	}
	for _, row := range result.Rows {
		out[[2]string{fmt.Sprint(row[0]), fmt.Sprint(row[1])}] = askPairCount{Events: num(row[2]), Incoming: num(row[3]), Outgoing: num(row[4])}
	}
	return out, nil
}

// askSwappedRoles catches the confident zero that survives the column probe: the question names two numbers, the query puts them the wrong way round (the other party in
// src, the subscriber in dst) and finds nothing, while the records do hold the pair in the opposite roles (measured 2026-10-15, seed 83: two confident zeros for "how many
// times did A get a call from B" where 22 and 86 were the truth). A subscriber can also be somebody's contact, so the numbers' own columns do not settle it: the pair does.
// It speaks only when the orientation the query used has nothing for what was asked and the opposite one has something.
func askSwappedRoles(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, f askFrame, v *askValidated, limits askLimits) string {
	a, b := f.Numbers[0], f.Numbers[1]
	ca, cb := askComparedColumns(v.SQL, a), askComparedColumns(v.SQL, b)
	var src, dst string
	switch {
	case ca["src"] && cb["dst"] && !ca["dst"] && !cb["src"]:
		src, dst = a, b
	case cb["src"] && ca["dst"] && !cb["dst"] && !ca["src"]:
		src, dst = b, a
	default:
		return ""
	}
	pairs, err := askPairs(ctx, db, req.TenantID, req.CollectionID, a, b, limits)
	if err != nil {
		return ""
	}
	relevant := func(c askPairCount) int64 {
		switch {
		case f.Cues["incoming"] && !f.Cues["outgoing"]:
			return c.Incoming
		case f.Cues["outgoing"] && !f.Cues["incoming"]:
			return c.Outgoing
		}
		return c.Events
	}
	used, opposite := pairs[[2]string{src, dst}], pairs[[2]string{dst, src}]
	if relevant(used) > 0 || relevant(opposite) == 0 {
		return ""
	}
	return fmt.Sprintf("Nothing matched, because the roles are the wrong way round. In these records %s is the subscriber (src) and %s is the other party (dst), not the opposite: write src = '%s' AND dst = '%s'.", dst, src, dst, src)
}

// askRoleHint says, for an identifier that the query compared in one column and that occurs in others, what the columns mean. In the call views src is the subscriber
// whose records these are and dst is the other party; a number that only ever occurs as the other party cannot be a subscriber, and the model that puts it in src
// gets an empty result (measured 2026-10-15: five questions of the form "how many times did A get a call from B").
func askRoleHint(id string, compared map[string]bool, hits []string) string {
	hitColumns := map[string]bool{}
	for _, hit := range hits {
		hitColumns[hit[strings.Index(hit, ".")+1:]] = true
	}
	switch {
	case compared["account"] && !hitColumns["account"] && (hitColumns["counterparty_account"] || hitColumns["counterparty"] || hitColumns["received_from"] || hitColumns["sent_to"]):
		return fmt.Sprintf("Nothing matched. %s is never the account of a statement in this case: it occurs only as the other side of a transaction. account is the account a statement belongs to and counterparty_account (counterparty in v_account_counterparties) is the other side, so look for %s there. When %s paid the account of a statement, that is a credit of that account (direction = 'credit'); when the account of a statement paid %s, that is a debit.", id, id, id, id)
	case compared["counterparty_account"] && !hitColumns["counterparty_account"] && hitColumns["account"]:
		return fmt.Sprintf("Nothing matched. %s is the account of a statement in this case and never the other side of a transaction. Put %s in account.", id, id)
	case compared["src"] && !hitColumns["src"] && hitColumns["dst"]:
		return fmt.Sprintf("Nothing matched. %s is never a subscriber (src) in this case: it occurs only as the other party (dst). src is the subscriber whose records these are and dst is the other party, so put %s in dst and the other number of the question in src.", id, id)
	case compared["dst"] && !hitColumns["dst"] && hitColumns["src"]:
		return fmt.Sprintf("Nothing matched. %s is a subscriber (src) in this case and never the other party (dst). src is the subscriber whose records these are and dst is the other party, so put %s in src.", id, id)
	}
	return ""
}

func askReadEmpty(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, cat *askCatalogue, f askFrame, v *askValidated, limits askLimits) (askEmptyVerdict, error) {
	ids := f.Identifiers()
	if len(ids) == 0 {
		return askEmptyVerdict{}, nil
	}
	// A path search over the subscribers' contacts (a recursive query, or a query on v_connection_edges, whose columns a and b hold the two numbers) is empty when no path exists: that is
	// an answer, not a query that looked in the wrong place.
	// The same holds for the co-location windows and the possible meetings (columns a and b hold the two numbers): a pair that never met has no row, and that is the answer.
	wrongPlaceMatters := !v.Recursive && !askUsesAnalysisView(v)
	var absent []string
	for _, id := range ids {
		hits, err := askProbeFn(ctx, db, req.TenantID, req.CollectionID, cat, id, limits)
		if err != nil {
			return askEmptyVerdict{}, err
		}
		if len(hits) == 0 {
			// The other form of a handset identifier: 15 digits asked and 14 stored, or 14 asked and 15 stored. A record under the other form is a record of this handset.
			if other, form, err := askOtherIMEIForm(ctx, db, req, cat, id, limits); err != nil {
				return askEmptyVerdict{}, err
			} else if len(other) > 0 {
				return askEmptyVerdict{elsewhere: strings.Join(other, ", "), identifier: id, hint: fmt.Sprintf("Nothing matched, because this case holds the handset %s as %s (the other form of the same IMEI: 15 digits with the check digit, or 14 without). Compare imei with both forms: imei IN ('%s', '%s').", id, form, id, form)}, nil
			}
			absent = append(absent, id)
			continue
		}
		// A registration or a handset is a statement of its own family: a number that occurs in the call records and has no registration is an answer, not a wrong place (see askFamilyViews).
		if family := askFamilyViews(v); family != nil {
			hits = askHitsIn(hits, family)
			if len(hits) == 0 {
				continue
			}
			// The question is about the family's own records. A query that also reads a view of another family (a join the question does not ask for) and finds nothing has not shown that the
			// records are absent: the identifier is there (measured 2026-10-30, SUB-002: a registration view joined to the device pairings on a nonsense key).
			if outside := askViewsOutside(v, family); len(outside) > 0 && wrongPlaceMatters {
				return askEmptyVerdict{elsewhere: strings.Join(hits, ", "), identifier: id, reason: "the query joined views the question does not ask about and found nothing although the identifier is in the records the question is about",
					hint: fmt.Sprintf("Nothing matched, but %s does occur in %s, and your query also reads %s, which the question does not ask about. Answer from %s alone, without a join to %s.", id, strings.Join(hits, ", "), strings.Join(outside, ", "), strings.Join(askHitViews(hits), ", "), strings.Join(outside, ", "))}, nil
			}
		}
		// The query looked where it compared the identifier. A hit in any other column means it looked in the wrong place.
		compared := askComparedColumns(v.SQL, id)
		var elsewhere []string
		for _, hit := range hits {
			if column := hit[strings.Index(hit, ".")+1:]; !compared[column] {
				elsewhere = append(elsewhere, hit)
			}
		}
		if len(elsewhere) == len(hits) && wrongPlaceMatters {
			return askEmptyVerdict{elsewhere: strings.Join(elsewhere, ", "), identifier: id, hint: askRoleHint(id, compared, hits)}, nil
		}
	}
	if len(absent) == 0 && wrongPlaceMatters && len(f.Numbers) >= 2 {
		if hint := askSwappedRoles(ctx, db, req, f, v, limits); hint != "" {
			return askEmptyVerdict{elsewhere: "v_contact_pairs (the roles of src and dst)", identifier: f.Numbers[0], hint: hint}, nil
		}
	}
	if len(absent) == len(ids) {
		return askEmptyVerdict{absence: askAbsenceOf(absent, f)}, nil
	}
	return askEmptyVerdict{}, nil
}

// askOtherIMEIForm looks for the 14-digit form of a 15-digit IMEI of the question (the form some exports store, without the check digit) and returns the columns that hold it and
// that form. The reverse (14 digits asked, 15 stored) is not handled: it needs a pattern match, which the validator does not allow, and no measured question needed it.
func askOtherIMEIForm(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, cat *askCatalogue, id string, limits askLimits) ([]string, string, error) {
	if len(id) != 15 {
		return nil, "", nil
	}
	short := askIMEI14(id)
	hits, err := askProbeFn(ctx, db, req.TenantID, req.CollectionID, cat, short, limits)
	return hits, short, err
}

// askComparedColumns is the set of columns the query compares with the identifier (column = 'id', column IN (..., 'id', ...), 'id' = column), read from the
// parser's canonical text. An identifier the query never compares (a LIKE pattern, a computed expression) leaves the set empty, which reads as "looked in the
// wrong place" and costs one repair at most.
func askComparedColumns(sql, id string) map[string]bool {
	quoted := regexp.QuoteMeta("'" + id + "'")
	out := map[string]bool{}
	for _, pattern := range []string{
		`([a-z_][a-z0-9_.]*)\s*(?:=|<>|!=)\s*` + quoted,
		quoted + `\s*(?:=|<>|!=)\s*([a-z_][a-z0-9_.]*)`,
		`([a-z_][a-z0-9_.]*)\s+(?:not\s+)?in\s*\([^)]*` + quoted,
		`host\(([a-z_][a-z0-9_.]*)\)\s*=\s*` + quoted,
	} {
		for _, m := range regexp.MustCompile("(?i)"+pattern).FindAllStringSubmatch(sql, -1) {
			name := strings.ToLower(m[1])
			out[name[strings.LastIndex(name, ".")+1:]] = true
		}
	}
	return out
}

// askAgree writes a second draft with other examples and compares the two results. Equal results pass; unequal ones abstain, with both queries in the audit.
func askAgree(ctx context.Context, cfg config, model string, db *pgxpool.Pool, req hybridQueryRequest, cat *askCatalogue, chosen []string, f askFrame, all, used []askExample, first *govSQLResult, limits askLimits) (bool, error) {
	var other []askExample
	for _, e := range all {
		skip := false
		for _, u := range used {
			skip = skip || u.ID == e.ID
		}
		if !skip {
			other = append(other, e)
		}
	}
	second := askRetrieveExamples(f.Question, chosen, other, 2)
	turns := []govSQLTurn{{Role: "system", Content: askPrompt(cat, append([]string{}, chosen...), second)}, {Role: "user", Content: askUserPrompt(f)}}
	raw, failure := askModel(ctx, cfg, model, turns)
	if failure != "" {
		return false, fmt.Errorf("second draft: %s", failure)
	}
	answerable, sql, bad := govSQLParseOutput(raw)
	if bad != "" || !answerable {
		return false, nil
	}
	validated, rejection := askValidate(sql, cat, chosen)
	if rejection != nil || len(askCheckFrame(f, validated)) > 0 {
		return false, nil
	}
	result, err := askExec(ctx, db, req.TenantID, req.CollectionID, validated, limits)
	if err != nil {
		return false, nil //nolint:nilerr // a second draft that cannot run is a disagreement, not a failure of the question
	}
	return askSameResult(first, result), nil
}

// askSameResult compares two results by their rows, ignoring the order of rows and the names of columns.
func askSameResult(a, b *govSQLResult) bool {
	if len(a.Columns) != len(b.Columns) || len(a.Rows) != len(b.Rows) {
		return false
	}
	key := func(r *govSQLResult) []string {
		out := make([]string, len(r.Rows))
		for i, row := range r.Rows {
			parts := make([]string, len(row))
			for j, v := range row {
				parts[j] = fmt.Sprint(v)
			}
			out[i] = strings.Join(parts, "\x1f")
		}
		sort.Strings(out)
		return out
	}
	ka, kb := key(a), key(b)
	for i := range ka {
		if ka[i] != kb[i] {
			return false
		}
	}
	return true
}

// askChecksPassed says, in plain words, what was checked: it is shown to the analyst under the answer.
func askChecksPassed(f askFrame, v *askValidated) []string {
	checks := []string{"The query is one read-only SELECT over the case's analyst views, checked against an allowlist before it ran."}
	if ids := f.Identifiers(); len(ids) > 0 {
		checks = append(checks, fmt.Sprintf("Every identifier of the question (%d) is in the query, and the query names none the question does not.", len(ids)))
	}
	if len(f.Dates)+len(f.Months)+len(f.Years) > 0 {
		checks = append(checks, "Every date, month and year of the question is applied by the query.")
	}
	if f.Limit > 0 {
		checks = append(checks, fmt.Sprintf("The query keeps exactly the %d results the question asks for.", f.Limit))
	}
	if v.Recursive {
		checks = append(checks, fmt.Sprintf("The path search is bounded to %d hops and never revisits a number.", v.MaxHops))
	}
	for _, cue := range []struct{ key, text string }{
		{"incoming", "The question's direction (incoming) is applied."}, {"outgoing", "The question's direction (outgoing) is applied."},
		{"sms", "The kind of event (SMS) is applied."}, {"night", "The hours of the night are applied on the case clock."},
	} {
		if f.Cues[cue.key] {
			checks = append(checks, cue.text)
		}
	}
	return checks
}

// ---- the header and the log ---------------------------------------------------------------------------------------------------------------------------

func setAskV2Header(w http.ResponseWriter, a AskAuditV2) {
	parts := []string{"state=" + a.State, fmt.Sprintf("attempts=%d", a.Attempts), fmt.Sprintf("ms=%d", a.TotalMS)}
	if a.Reason != "" {
		parts = append(parts, "reason="+strings.NewReplacer(";", ",", "\n", " ").Replace(govSQLClip(a.Reason, 120)))
	}
	if len(a.Views) > 0 {
		parts = append(parts, "views="+strings.Join(a.Views, ","))
	}
	w.Header().Set(askHeaderName, strings.Join(parts, "; "))
	askLogAudit(a)
}
