package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mudler/LocalAI/pkg/httpclient"
)

// ASK ENGINE V2: A READING OF THE RESULT (docs/data/answer-evidence-and-interpretation-20261102.md, part B, decision D33 of 2026-11-02).
//
// The figure is written by code and is on the screen at once. The reading is a second, separate thing: a few plain sentences by the local language model that say what the result is, what it
// rests on and what it does not show. It comes AFTER the figure (the workspace asks for it when the answer is on the screen), is labelled as the model's reading and never replaces the figure.
//
// What the model sees, and what it never does: the question, the figure and at most 20 rows of the result, the plain-words steps, the notes of the answer and the summary of the evidence (how many
// rows, in which files). It never sees the rest of the case and never writes a figure: every number, date and identifier of the reading must occur in what it was shown, or the reading is dropped
// and the analyst is told that none could be checked. Words that accuse are refused too. A reading that fails a check is asked for once more with the fault named, then dropped.
//
// The inputs wait in memory for 30 minutes under a random token that only the scope that asked can use; nothing of them is written to disk.
const (
	askReadingEnv       = "FORENSIC_ASK_V2_READING"
	askReadingRows      = 20
	askReadingTTL       = 30 * time.Minute
	askReadingStoreMax  = 300
	askReadingMaxTokens = 320
	askReadingTimeout   = 60 * time.Second
	askReadingMaxChars  = 700
)

// askReadingEnabled: on unless the environment turns it off.
func askReadingEnabled() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv(askReadingEnv)), "false") //nolint:forbidigo // forensic records product API reads its settings from the environment by design (docker compose)
}

type askReadingInput struct {
	Tenant, Collection string
	Question           string
	Headline           string
	Shown              string // the text the model is given: what it may use
	Created            time.Time

	mu      sync.Mutex
	reading *askReading
}

// askReading is what the workspace shows.
type askReading struct {
	State      string `json:"state"` // "ready" or "unavailable"
	Text       string `json:"text,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Model      string `json:"model,omitempty"`
	ElapsedMS  int64  `json:"elapsed_ms,omitempty"`
	Disclosure string `json:"disclosure"`
}

const askReadingDisclosure = "Written by the local language model from the rows of this result only; it saw nothing else of the case and wrote none of the figures. Check it against the rows."

var askReadings = struct {
	sync.Mutex
	items map[string]*askReadingInput
	order []string
}{items: map[string]*askReadingInput{}}

func askReadingToken() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return ""
	}
	return hex.EncodeToString(raw)
}

func askReadingStore(in *askReadingInput) string {
	token := askReadingToken()
	if token == "" {
		return ""
	}
	askReadings.Lock()
	defer askReadings.Unlock()
	now := time.Now()
	kept := askReadings.order[:0]
	for _, t := range askReadings.order {
		item := askReadings.items[t]
		if item == nil || now.Sub(item.Created) > askReadingTTL {
			delete(askReadings.items, t)
			continue
		}
		kept = append(kept, t)
	}
	askReadings.order = kept
	for len(askReadings.order) >= askReadingStoreMax {
		delete(askReadings.items, askReadings.order[0])
		askReadings.order = askReadings.order[1:]
	}
	in.Created = now
	askReadings.items[token] = in
	askReadings.order = append(askReadings.order, token)
	return token
}

func askReadingFind(token string) *askReadingInput {
	askReadings.Lock()
	defer askReadings.Unlock()
	item := askReadings.items[token]
	if item == nil || time.Since(item.Created) > askReadingTTL {
		return nil
	}
	return item
}

// askReadingText builds what the model is shown, and nothing else.
func askReadingText(question, headline string, r *govSQLResult, steps, notes []string, ev *askEvidence) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Question: %s\nFigure: %s\n", question, headline)
	shown := len(r.Rows)
	if shown > askReadingRows {
		shown = askReadingRows
	}
	suffix := ""
	if r.Truncated || len(r.Rows) > shown {
		suffix = fmt.Sprintf(" (only the first %d are shown to you; say nothing about the others)", shown)
	}
	fmt.Fprintf(&b, "Result: %d rows%s\ncolumns: %s\n", len(r.Rows), suffix, strings.Join(r.Columns, " | "))
	for _, row := range r.Rows[:shown] {
		cells := make([]string, len(row))
		for i, c := range row {
			cells[i] = govSQLClip(fmt.Sprint(c), 80)
		}
		b.WriteString(strings.Join(cells, " | ") + "\n")
	}
	if len(steps) > 0 {
		b.WriteString("How it was found: " + strings.Join(steps, " ") + "\n")
	}
	if ev != nil && ev.State == "itemised" {
		files := ""
		for i, f := range ev.Files {
			if i == 3 {
				break
			}
			files += fmt.Sprintf("%s (%d rows) ", f.SourceFile, f.Rows)
		}
		fmt.Fprintf(&b, "Records behind it: %d rows in %s", ev.Total, askEvidencePlural(len(ev.Files), "source file"))
		if files != "" {
			b.WriteString("; largest: " + strings.TrimSpace(files))
		}
		b.WriteString(". " + ev.Check + "\n")
	} else if ev != nil {
		b.WriteString("Records behind it: not itemised (" + ev.Reason + ").\n")
	}
	for _, n := range notes {
		if strings.HasPrefix(n, "Computed by a model-written query") {
			continue
		}
		b.WriteString("Note: " + govSQLClip(n, 240) + "\n")
	}
	return b.String()
}

// askAttachReading registers the inputs and puts the token in the answer. Only an answer with at least one row has a reading.
func askAttachReading(resp *hybridQueryResponse, req hybridQueryRequest, r *govSQLResult, steps []string, ev *askEvidence, absence string) {
	if resp == nil || resp.Enterprise == nil || !askReadingEnabled() || absence != "" || r == nil || len(r.Rows) == 0 {
		return
	}
	headline, _ := resp.Answer["answer"].(string)
	notes, _ := resp.Enterprise["limitations"].([]string)
	in := &askReadingInput{Tenant: req.TenantID, Collection: req.CollectionID, Question: req.Query, Headline: headline,
		Shown: askReadingText(req.Query, headline, r, steps, notes, ev)}
	token := askReadingStore(in)
	if token == "" {
		return
	}
	resp.Enterprise["reading"] = map[string]any{"state": "pending", "token": token, "path": "/query/reading?token=" + token, "disclosure": askReadingDisclosure}
}

// ---- the checks ---------------------------------------------------------------------------------------------------------------------------------------

var (
	askReadingNumber = regexp.MustCompile(`\d{4}-\d{2}-\d{2}|\d[\d,]*(?:\.\d+)?`)
	askReadingAccuse = regexp.MustCompile(`(?i)\b(guilty|suspect\w*|suspicious\w*|criminal\w*|crime\w*|illegal\w*|conspir\w*|accomplice\w*|culprit\w*|perpetrator\w*|murder\w*|terror\w*|innocent|proves?|proven|definitely|certainly|obviously)\b`)
)

// askReadingAllowed is every number, date and identifier of the text the model was shown; a decimal also in the roundings an analyst would write.
func askReadingAllowed(shown string) map[string]bool {
	allowed := map[string]bool{}
	for _, tok := range askReadingNumber.FindAllString(shown, -1) {
		plain := strings.TrimRight(strings.ReplaceAll(tok, ",", ""), ".")
		allowed[plain] = true
		if f, err := strconv.ParseFloat(plain, 64); err == nil && strings.Contains(plain, ".") {
			for _, places := range []int{0, 1, 2, 3} {
				allowed[strconv.FormatFloat(f, 'f', places, 64)] = true
			}
		}
	}
	return allowed
}

// askReadingCheck returns what is wrong with a reading, or "" when it passes.
func askReadingCheck(text, shown string) string {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return "the reading is empty"
	case len(text) > askReadingMaxChars:
		return fmt.Sprintf("the reading is longer than %d characters; write two to four short sentences", askReadingMaxChars)
	case !utf8.ValidString(text):
		return "the reading is not valid text"
	}
	if m := askReadingAccuse.FindString(text); m != "" {
		return "the word \"" + m + "\" is not allowed: a record shows that something was recorded, never what a person did or why"
	}
	allowed := askReadingAllowed(shown)
	for _, tok := range askReadingNumber.FindAllString(text, -1) {
		plain := strings.TrimRight(strings.ReplaceAll(tok, ",", ""), ".")
		if !allowed[plain] {
			return "the number " + tok + " does not occur in the text you were given; use only the figures and values you were shown"
		}
	}
	return ""
}

// ---- the model -----------------------------------------------------------------------------------------------------------------------------------------------

const askReadingSystem = `You explain the result of a query over forensic records to an analyst, in plain English, in two to four short sentences.
Say, in this order: what the result is (use the figure and values exactly as given); what it rests on (the records and files named); one limit the analyst must keep in mind.
Use ONLY the text you are given. Write every number, date and identifier exactly as it appears there; never add, round up, estimate or compute one.
A record shows that something was recorded, never who a person is, what they did or why. Do not say anything is suspicious, and do not draw conclusions about people.
If the result shows no rows, say that nothing was found. Answer with JSON: {"reading": "..."}.`

type askReadingModelFunc func(ctx context.Context, cfg config, model, system, user string) (content string, failure string)

// askReadingModel is a variable only so that a spec can stand in a scripted model.
var askReadingModel askReadingModelFunc = askReadingModelHTTP

func askReadingModelHTTP(ctx context.Context, cfg config, model, system, user string) (string, string) {
	payload, _ := json.Marshal(map[string]any{
		"model": model, "temperature": 0, "max_tokens": askReadingMaxTokens,
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "forensic_ask_engine_v2_reading", "strict": true, "schema": map[string]any{
			"type": "object", "properties": map[string]any{"reading": map[string]any{"type": "string"}}, "required": []string{"reading"}, "additionalProperties": false}}},
		"messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}},
	})
	timeout := askReadingTimeout
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
		return "", "NOT_FINISHED"
	}
	return envelope.Choices[0].Message.Content, ""
}

// askReadingMake asks the model for the reading, checks it, asks once more with the fault named, and drops it when it still fails.
func askReadingMake(ctx context.Context, cfg config, in *askReadingInput) *askReading {
	started := time.Now()
	out := &askReading{Disclosure: askReadingDisclosure, Model: cfg.SynthesisModel}
	user := in.Shown
	for attempt := 0; attempt < 2; attempt++ {
		content, failure := askReadingModel(ctx, cfg, cfg.SynthesisModel, askReadingSystem, user)
		if failure != "" {
			out.State, out.Reason = "unavailable", "the model did not answer in time ("+failure+")"
			break
		}
		var parsed struct {
			Reading string `json:"reading"`
		}
		if err := json.Unmarshal([]byte(content), &parsed); err != nil {
			out.State, out.Reason = "unavailable", "the model's reading could not be read"
			break
		}
		problem := askReadingCheck(parsed.Reading, in.Shown)
		if problem == "" {
			out.State, out.Text = "ready", strings.TrimSpace(parsed.Reading)
			break
		}
		out.State, out.Reason = "unavailable", "a reading was written but did not pass the check against the rows ("+problem+"), so none is shown"
		user = in.Shown + "\nYour last reading was refused: " + problem + ". Write it again."
	}
	if out.State == "ready" {
		out.Reason = ""
	}
	out.ElapsedMS = time.Since(started).Milliseconds()
	return out
}

// askReadingHandler: GET /query/reading?token=... returns the reading of an answer that is on the screen. The token is random and held in memory; only the scope that asked can use it.
func askReadingHandler(cfg config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		scope, err := bindForensicScope(r, r.URL.Query().Get("tenant_id"), strings.TrimSpace(r.URL.Query().Get("collection_id")), "", "")
		if err != nil {
			writeScopeError(w, err)
			return
		}
		in := askReadingFind(strings.TrimSpace(r.URL.Query().Get("token")))
		if in == nil || in.Tenant != scope.TenantID || (scope.CollectionID != "" && in.Collection != scope.CollectionID) {
			writeError(w, http.StatusNotFound, errors.New("this reading is no longer available; ask the question again"))
			return
		}
		in.mu.Lock()
		defer in.mu.Unlock()
		if in.reading != nil {
			writeJSON(w, http.StatusOK, in.reading)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), askReadingTimeout+5*time.Second)
		defer cancel()
		made := askReadingMake(ctx, cfg, in)
		log.Printf("ask engine v2 reading state=%s ms=%d", made.State, made.ElapsedMS)
		// a reading that was written, or refused by the check, is final; a model that did not answer is tried again at the next request
		if made.State == "ready" || strings.Contains(made.Reason, "did not pass the check") {
			in.reading = made
		}
		writeJSON(w, http.StatusOK, made)
	}
}
