package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mudler/LocalAI/pkg/httpclient"
)

// GOVERNED SQL: WHAT THE MODEL IS TOLD, AND HOW IT IS ASKED.
//
// The model is shown the schema and the question. It is never shown case data: not a
// row, not a count, not a sample value that was read from the evidence. The only
// values in the prompt are the ones the curated layer DECLARES (enumerations such as
// call types), which are part of the schema's meaning and not of any case.
//
// The reply is a JSON object, `{"answerable": bool, "sql": string}`, under a strict
// JSON schema, so the endpoint turns it into a grammar and the model cannot ramble,
// return prose, or run past the structure. Nothing in it is trusted: governed_sql_validate.go
// takes the `sql` string apart and governed_sql_verify.go checks it against the question.
//
// PROMPT LAYOUT. The system message is everything that does not change between two
// questions about the same family: the rules, the examples and the schema. The user
// message is the question and its hints. The endpoint reuses the cached prompt start,
// so a second CDR question pays only for its own few tokens (the L1b layout,
// docs/work/L1B_SHARED_PREAMBLE_SPEC_20261002.md).
const (
	govSQLMaxTokens    = 320
	govSQLModelTimeout = 240 * time.Second
)

type govSQLTurn struct{ Role, Content string }

// govSQLModelFunc returns the model's text, or a short failure code. It is a
// variable so the specs can stand in a scripted model without a network.
type govSQLModelFunc func(ctx context.Context, cfg config, model string, turns []govSQLTurn) (content string, failure string)

var govSQLModel govSQLModelFunc = govSQLModelHTTP

const govSQLRules = `You write one PostgreSQL SELECT query that answers an investigator's question about the evidence in a case. A server runs your query, so it must be exact.

RULES
1. Read from exactly one view named under SCHEMA. Use only the columns listed for it. Never invent a view or a column. No joins, no semicolon, no comments.
2. Put EVERY condition in the question into the query: each value, number, identifier, date, month, year, time of day and threshold. Copy identifiers exactly as the question writes them, as text literals in quotes.
3. Compare text columns with the exact values listed for them (for example call_type = 'SMS'). When a value is listed, use it exactly as listed.
4. "How many" is COUNT(*), or COUNT(DISTINCT column) when it asks for different or unique ones. "Which", "most" and "top" group, count, order by the count descending and use LIMIT. "Earliest" or "first" is MIN of the time column; "latest" or "last" is MAX. "Average", "total", "largest" and "smallest" use AVG, SUM, MAX and MIN of the matching numeric column.
5. A number that can be either party of an event needs every identifier column that can hold it, joined with OR.
6. Times are on the case clock: the hour, day and month of a time column already mean the case's local time, so never convert time zones. Night means the hours 0 to 5: use EXTRACT(HOUR FROM column) BETWEEN 0 AND 5.
7. Give every output column a short readable alias such as number_of_calls or earliest_call.
8. If the question cannot be answered from the listed columns, set answerable to false and leave sql empty. Do not guess. A person's name is never a column value here, so a question about a named person is not answerable unless the name is a value of a listed column.

EXAMPLES, written for an invented view v_example; use the real views under SCHEMA.
Q: How many events have kind A?
{"answerable": true, "sql": "SELECT COUNT(*) AS number_of_events FROM v_example WHERE kind = 'A'"}
Q: Which kind occurs most often?
{"answerable": true, "sql": "SELECT kind, COUNT(*) AS number_of_events FROM v_example GROUP BY kind ORDER BY number_of_events DESC LIMIT 5"}
Q: When was the first event?
{"answerable": true, "sql": "SELECT MIN(event_time) AS first_event FROM v_example"}
Q: How many different owners had events in March 2026?
{"answerable": true, "sql": "SELECT COUNT(DISTINCT owner) AS number_of_owners FROM v_example WHERE event_time >= '2026-03-01' AND event_time < '2026-04-01'"}
Q: How many events involve 5551234?
{"answerable": true, "sql": "SELECT COUNT(*) AS number_of_events FROM v_example WHERE sender = '5551234' OR receiver = '5551234'"}
Q: What is the owner's home address?
{"answerable": false, "sql": ""}
`

// govSQLSystemPrompt is the rules, then the schema of the views offered for this
// question. It contains no case data.
func govSQLSystemPrompt(views []*govSQLView) string {
	var b strings.Builder
	b.WriteString(govSQLRules)
	b.WriteString("\nSCHEMA (the only views and columns that exist)\n")
	for _, view := range views {
		fmt.Fprintf(&b, "\nVIEW %s: %s\n", view.Name, view.Description)
		for _, c := range view.Columns {
			fmt.Fprintf(&b, "  %s %s", c.Name, c.Type)
			if c.Identifier {
				b.WriteString(" [identifier]")
			}
			label := strings.TrimSpace(c.Display)
			note := govSQLFirstSentence(c.Description, 120)
			switch {
			case label != "" && note != "":
				fmt.Fprintf(&b, " - %s: %s", label, note)
			case label != "":
				fmt.Fprintf(&b, " - %s", label)
			case note != "":
				fmt.Fprintf(&b, " - %s", note)
			}
			if len(c.Values) > 0 {
				values := c.Values
				if len(values) > 12 {
					values = values[:12]
				}
				fmt.Fprintf(&b, " (values: %s)", strings.Join(values, ", "))
			}
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "  Also available when listing rows: %s\n", strings.Join(view.Provenance, ", "))
	}
	return b.String()
}

// govSQLUserPrompt is the question and what the server read in it. The hints are
// deterministic and tell the model what the checker will demand, so the first
// attempt is usually the only one.
func govSQLUserPrompt(question string, facts govSQLQuestionFacts, views []*govSQLView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Q: %s\n", strings.TrimSpace(question))
	hints := []string{}
	for _, id := range facts.Identifiers {
		cols := govSQLIdentifierColumns(views)
		if len(cols) > 0 {
			hints = append(hints, fmt.Sprintf("%q is an identifier. Use it as a text literal on the column or columns that fit what it is (a phone number belongs in a number column, not in imei or imsi): %s.", id, strings.Join(cols, ", ")))
		} else {
			hints = append(hints, fmt.Sprintf("%q is an identifier. Use it as a text literal in a condition.", id))
		}
	}
	for _, fact := range facts.ValueFacts {
		hints = append(hints, fmt.Sprintf("The question says %s = '%s'.", fact.Column, fact.Value))
	}
	for _, fact := range facts.WeakValues {
		hints = append(hints, fmt.Sprintf("If the question means it, %s = '%s' is a listed value of %s.", fact.Column, fact.Value, fact.Column))
	}
	if facts.Magnitude != "" {
		hints = append(hints, fmt.Sprintf("The question states a condition: %q. Use > or < with the number.", facts.Magnitude))
	}
	if len(facts.TimeCues) > 0 && len(views) > 0 {
		hints = append(hints, fmt.Sprintf("The question restricts time (%s). Put it in the WHERE clause on a time column (%s).", strings.Join(facts.TimeCues, ", "), govSQLTimeColumns(views[0])))
	}
	for _, name := range facts.Names {
		hints = append(hints, fmt.Sprintf("The question names %q. No column holds personal names; use it only as a value of a listed column, otherwise set answerable to false.", name))
	}
	if facts.WantsSingle {
		hints = append(hints, "The question asks for one value: return one row, with no GROUP BY.")
	}
	if len(hints) > 0 {
		b.WriteString("Notes from the question:\n")
		for _, hint := range hints {
			b.WriteString("- " + hint + "\n")
		}
	}
	return b.String()
}

// govSQLIdentifierColumns lists the text identifier columns of the offered views,
// the ones named like a number first, since "a number that can be either party" is
// the common case.
func govSQLIdentifierColumns(views []*govSQLView) []string {
	first, rest := []string{}, []string{}
	for _, view := range views {
		for _, c := range view.Columns {
			if !c.Identifier || c.Type != "text" {
				continue
			}
			name := strings.ToLower(c.Name + " " + c.Display)
			if strings.Contains(name, "number") || strings.Contains(name, "msisdn") || strings.Contains(name, "phone") {
				first = append(first, c.Name)
			} else {
				rest = append(rest, c.Name)
			}
		}
	}
	return append(first, rest...)
}

// govSQLRetryPrompt tells the model exactly what was wrong, once.
func govSQLRetryPrompt(problems []string) string {
	var b strings.Builder
	b.WriteString("That query cannot be used:\n")
	for _, problem := range problems {
		b.WriteString("- " + problem + "\n")
	}
	b.WriteString("Write a corrected query as the same JSON object.")
	return b.String()
}

func govSQLResponseSchema() map[string]any {
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

// govSQLParseOutput reads the model's JSON. A failure code is returned for anything
// that is not exactly the object asked for.
func govSQLParseOutput(content string) (answerable bool, sql string, failure string) {
	var out struct {
		Answerable bool   `json:"answerable"`
		SQL        string `json:"sql"`
	}
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil || decoder.More() {
		return false, "", "INVALID_JSON"
	}
	return out.Answerable, strings.TrimSpace(out.SQL), ""
}

// govSQLModelHTTP asks the LocalAI endpoint, exactly as the front door does: strict
// JSON schema, temperature 0, bounded tokens, a completion that must have stopped.
func govSQLModelHTTP(ctx context.Context, cfg config, model string, turns []govSQLTurn) (string, string) {
	messages := make([]map[string]string, 0, len(turns))
	for _, turn := range turns {
		messages = append(messages, map[string]string{"role": turn.Role, "content": turn.Content})
	}
	payload, _ := json.Marshal(map[string]any{
		"model": model, "temperature": 0, "max_tokens": govSQLMaxTokens,
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "forensic_governed_sql", "strict": true, "schema": govSQLResponseSchema()}},
		"messages":        messages,
	})
	timeout := govSQLModelTimeout
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

func govSQLDigest(turns []govSQLTurn) string {
	h := sha256.New()
	for _, turn := range turns {
		h.Write([]byte(turn.Role + "\x00" + turn.Content + "\x00"))
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
