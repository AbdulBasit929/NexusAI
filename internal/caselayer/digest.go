package caselayer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// Column is one column of a result, as the engine names it (Key) and as the analyst reads it (Header).
type Column struct {
	Key    string `json:"key"`
	Header string `json:"header"`
}

// Metric is one labelled figure of an answer ("Number of records", "5,005").
type Metric struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// EvidenceRef names an evidence file (and its version) that an answer cites.
type EvidenceRef struct {
	EvidenceID string `json:"evidence_id"`
	VersionID  string `json:"version_id,omitempty"`
	SourceFile string `json:"source_file,omitempty"`
}

// Digest is what the layer keeps of an engine response besides the response itself: the outcome in one word, the sentence, the shape and size of the result, the query, the cited evidence,
// the figures. It is read from the response by code; the engine is never asked for it, so it works for every path that answers (the Ask engine, the governed lane, the older ladder).
type Digest struct {
	Outcome    string
	Headline   string
	AnswerText string
	Shape      string
	Query      string
	Model      string
	PromptID   string
	Views      []string
	RowCount   int
	Columns    []Column
	Rows       [][]any
	Metrics    []Metric
	Evidence   []EvidenceRef
	TotalMS    int64
	ModelMS    int64
	ExecMS     int64
	Attempts   int
}

// The outcomes of a turn. "failed" is the layer's own (the engine could not be reached or returned something that is not an answer).
const (
	OutcomeAnswered      = "answered"
	OutcomeAbstained     = "abstained"
	OutcomeClarification = "clarification"
	OutcomeWithheld      = "withheld"
	OutcomeUnsupported   = "unsupported"
	OutcomeRefused       = "refused"
	OutcomeFailed        = "failed"
	OutcomeUnknown       = "unknown"
)

func dig(v any, path ...string) any {
	for _, p := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[p]
	}
	return v
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

func asInt64(v any) int64 {
	switch x := v.(type) {
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return n
		}
		if f, err := x.Float64(); err == nil {
			return int64(f)
		}
	case float64:
		return int64(x)
	}
	return 0
}

func firstString(vals ...any) string {
	for _, v := range vals {
		if s := strings.TrimSpace(asString(v)); s != "" {
			return s
		}
	}
	return ""
}

func decodeJSON(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("the response is not a JSON object")
	}
	return m, nil
}

// ExtractDigest reads the digest from an engine response.
func ExtractDigest(raw []byte) (*Digest, error) {
	m, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}
	d := &Digest{}
	resultState := strings.ToUpper(asString(dig(m, "answer", "result_state")))
	if resultState == "" {
		resultState = strings.ToUpper(asString(dig(m, "enterprise", "result_state")))
	}
	stateTag := strings.ToLower(firstString(dig(m, "planner", "ask_engine_v2", "state"), dig(m, "planner", "governed_sql", "state")))
	clarification := firstString(dig(m, "answer", "clarification"), dig(m, "enterprise", "clarification"))
	answer := firstString(dig(m, "answer", "answer"), dig(m, "enterprise", "executive_answer"))
	d.Headline = clip(firstString(answer, clarification), 600)
	d.AnswerText = firstString(answer, clarification)
	if utf8.RuneCountInString(d.AnswerText) > 4000 {
		d.AnswerText = string([]rune(d.AnswerText)[:4000])
	}
	switch {
	case stateTag == "abstained":
		d.Outcome = OutcomeAbstained
	case resultState == "COMPLETED":
		d.Outcome = OutcomeAnswered
	case resultState == "CLARIFICATION_REQUIRED" || asString(m["intent"]) == "clarification":
		d.Outcome = OutcomeClarification
	case resultState == "WITHHELD":
		d.Outcome = OutcomeWithheld
	case resultState == "UNSUPPORTED" || resultState == "NOT_ENOUGH_INFORMATION":
		d.Outcome = OutcomeUnsupported
	case resultState == "REFUSED":
		d.Outcome = OutcomeRefused
	case answer != "":
		d.Outcome = OutcomeAnswered
	default:
		d.Outcome = OutcomeUnknown
	}
	d.Shape = firstString(dig(m, "enterprise", "result_shape"), dig(m, "answer", "ask_engine_v2", "shape"))
	d.Query = firstString(dig(m, "enterprise", "derivation", "sql"), dig(m, "answer", "ask_engine_v2", "sql"), dig(m, "answer", "governed_sql", "sql"))
	d.Model = firstString(dig(m, "enterprise", "derivation", "model"), dig(m, "planner", "ask_engine_v2", "model"), dig(m, "planner", "governed_sql", "model"))
	d.PromptID = firstString(dig(m, "enterprise", "derivation", "prompt_id"), dig(m, "planner", "ask_engine_v2", "prompt_id"), dig(m, "planner", "governed_sql", "prompt_id"))
	if views, ok := dig(m, "enterprise", "derivation", "views").([]any); ok {
		for _, v := range views {
			d.Views = append(d.Views, asString(v))
		}
	}
	for _, tag := range []string{"ask_engine_v2", "governed_sql"} {
		if a := dig(m, "planner", tag); a != nil {
			d.TotalMS = asInt64(dig(a, "total_ms"))
			d.ModelMS = asInt64(dig(a, "model_ms"))
			d.ExecMS = asInt64(dig(a, "exec_ms"))
			d.Attempts = int(asInt64(dig(a, "attempts")))
			break
		}
	}
	if d.TotalMS == 0 {
		d.TotalMS = asInt64(dig(m, "telemetry", "total_latency_ms"))
	}
	d.readGrid(m)
	if mets, ok := dig(m, "enterprise", "metrics").([]any); ok {
		for _, x := range mets {
			// the "Request Class" and "Route" lines of a refusal describe the planner, not the case
			if asString(dig(x, "source")) == "planner" {
				continue
			}
			label := asString(dig(x, "label"))
			if label != "" {
				d.Metrics = append(d.Metrics, Metric{Label: label, Value: asString(dig(x, "value"))})
			}
		}
	}
	d.Evidence = ExtractEvidenceRefs(m)
	return d, nil
}

// readGrid reads the result table: the columns in their order and every row as a list of cells in that order.
func (d *Digest) readGrid(m map[string]any) {
	grid, _ := dig(m, "enterprise", "data_grid").(map[string]any)
	if grid == nil {
		return
	}
	if cols, ok := grid["columns"].([]any); ok {
		for _, c := range cols {
			key := asString(dig(c, "key"))
			if key != "" {
				d.Columns = append(d.Columns, Column{Key: key, Header: firstString(dig(c, "header"), key)})
			}
		}
	}
	rows, _ := grid["rows"].([]any)
	if len(d.Columns) == 0 && len(rows) > 0 {
		// no declared columns: the keys of the first row, in alphabetical order, so that two runs agree
		if first, ok := rows[0].(map[string]any); ok {
			keys := make([]string, 0, len(first))
			for k := range first {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				d.Columns = append(d.Columns, Column{Key: k, Header: k})
			}
		}
	}
	for _, r := range rows {
		row, ok := r.(map[string]any)
		if !ok {
			continue
		}
		cells := make([]any, len(d.Columns))
		for i, c := range d.Columns {
			cells[i] = row[c.Key]
		}
		d.Rows = append(d.Rows, cells)
	}
	d.RowCount = len(d.Rows)
	if n := asInt64(dig(m, "enterprise", "row_count")); n > int64(d.RowCount) {
		d.RowCount = int(n)
	}
}

// ExtractEvidenceRefs finds every evidence file an answer cites, wherever the path that answered put it: any object with an evidence_id is a citation. The walk is bounded in depth and
// in nodes, so a large result cannot make it slow; the list is de-duplicated and sorted, at most 500 entries.
func ExtractEvidenceRefs(root any) []EvidenceRef {
	seen := map[string]EvidenceRef{}
	nodes := 0
	var walk func(v any, depth int)
	walk = func(v any, depth int) {
		nodes++
		if nodes > 40000 || depth > 8 || len(seen) >= 500 {
			return
		}
		switch x := v.(type) {
		case map[string]any:
			if id := asString(x["evidence_id"]); looksLikeUUID(id) {
				ref := EvidenceRef{EvidenceID: strings.ToLower(id), VersionID: strings.ToLower(asString(x["version_id"])), SourceFile: firstString(x["source_file"], x["file"], x["filename"])}
				if !looksLikeUUID(ref.VersionID) {
					ref.VersionID = ""
				}
				key := ref.EvidenceID + "|" + ref.VersionID
				if old, ok := seen[key]; !ok || (old.SourceFile == "" && ref.SourceFile != "") {
					seen[key] = ref
				}
			}
			for _, child := range x {
				walk(child, depth+1)
			}
		case []any:
			for _, child := range x {
				walk(child, depth+1)
			}
		}
	}
	walk(root, 0)
	out := make([]EvidenceRef, 0, len(seen))
	for _, r := range seen {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].EvidenceID != out[j].EvidenceID {
			return out[i].EvidenceID < out[j].EvidenceID
		}
		return out[i].VersionID < out[j].VersionID
	})
	return out
}

func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if r != '-' {
				return false
			}
		case !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')):
			return false
		}
	}
	return true
}

func sha256Hex(parts ...[]byte) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write(p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// QueryHash is the SHA-256 of the query text with its whitespace collapsed; empty when there is no query.
func QueryHash(query string) string {
	q := strings.Join(strings.Fields(query), " ")
	if q == "" {
		return ""
	}
	return sha256Hex([]byte(q))
}

// ResultDigest is the SHA-256 over the column keys and every row of the result in the order returned; empty when the answer has no result table.
func (d *Digest) ResultDigest() string {
	if len(d.Columns) == 0 {
		return ""
	}
	h := sha256.New()
	for _, c := range d.Columns {
		h.Write([]byte(c.Key))
		h.Write([]byte{0x1f})
	}
	h.Write([]byte{'\n'})
	for _, row := range d.Rows {
		raw, _ := json.Marshal(row)
		h.Write(raw)
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}
