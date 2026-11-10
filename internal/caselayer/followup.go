package caselayer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// The follow-up resolver is a model role (default OFF, behind FORENSIC_CONVERSATION_FOLLOWUP in the API): given the earlier turns of a branch and a new message such as "and for last week?" it
// returns a question that stands alone. The model writes the question; CODE decides whether to use it, with a number lock: the question may contain no number, date, plate or identifier that is in
// neither the earlier questions nor the new message. A question that fails the lock is dropped and the message goes to the engine as typed. The role is measured before it is switched on.

// FollowUpInput is what the resolver sees: the new message and up to six earlier turns, oldest first.
type FollowUpInput struct {
	Question string
	History  []HistoryTurn
}

// FollowUpResult is a proposed standalone question and what it took over from the history ("time window", "number", "view").
type FollowUpResult struct {
	Question  string   `json:"question"`
	Inherited []string `json:"inherited"`
	Model     string   `json:"-"`
}

// FollowUpResolver is implemented by the API with a call to the language model.
type FollowUpResolver interface {
	Resolve(ctx context.Context, in FollowUpInput) (FollowUpResult, error)
}

// Resolution is how the question that was sent came about; it is stored with the turn and returned for display ("Interpreted as ...").
type Resolution struct {
	Method    string   `json:"method"` // none | model | rejected | failed
	Inherited []string `json:"inherited,omitempty"`
	Model     string   `json:"model,omitempty"`
	Reason    string   `json:"reason,omitempty"`
}

// valueToken is a token that carries a value: it has a digit in it (923001234567, 2026-02-03, LEA-1234, 14:30, 5).
var valueToken = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}\-_.:/+]*`)

func hasDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// numberLock reports the value tokens of the resolved question that are in neither the new message nor the history (questions, resolved questions, frames).
func numberLock(original string, history []HistoryTurn, resolved string) []string {
	var allowed strings.Builder
	allowed.WriteString(strings.ToLower(original))
	for _, h := range history {
		allowed.WriteString(" " + strings.ToLower(h.Question) + " " + strings.ToLower(h.Resolved) + " " + strings.ToLower(string(h.Frame)))
	}
	pool := allowed.String()
	var offending []string
	seen := map[string]bool{}
	for _, tok := range valueToken.FindAllString(strings.ToLower(resolved), -1) {
		tok = strings.TrimRight(tok, ".-:/_+")
		if tok == "" || !hasDigit(tok) || seen[tok] {
			continue
		}
		seen[tok] = true
		if !strings.Contains(pool, tok) {
			offending = append(offending, tok)
		}
	}
	return offending
}

// applyFollowUp runs the resolver (if any) and applies the lock. It returns the question to send and the resolution to store. A failure of the resolver is not a failure of the turn.
func applyFollowUp(ctx context.Context, r FollowUpResolver, original string, history []HistoryTurn) (string, Resolution) {
	if r == nil || len(history) == 0 {
		return original, Resolution{Method: "none"}
	}
	res, err := r.Resolve(ctx, FollowUpInput{Question: original, History: history})
	if err != nil {
		return original, Resolution{Method: "failed", Reason: "The earlier questions could not be read into this one; it was sent as typed."}
	}
	q := strings.TrimSpace(res.Question)
	switch {
	case q == "" || q == original:
		return original, Resolution{Method: "none", Model: res.Model}
	case utf8.RuneCountInString(q) > 4000 || strings.IndexFunc(q, func(r rune) bool { return r < 0x20 && r != '\n' && r != '\t' }) >= 0:
		return original, Resolution{Method: "rejected", Model: res.Model, Reason: "The proposed question was not usable; it was sent as typed."}
	}
	if offending := numberLock(original, history, q); len(offending) > 0 {
		return original, Resolution{Method: "rejected", Model: res.Model, Reason: fmt.Sprintf("The proposed question added %s, which is in neither the earlier questions nor this one; it was sent as typed.", strings.Join(offending, ", "))}
	}
	return q, Resolution{Method: "model", Inherited: res.Inherited, Model: res.Model}
}

// FollowUpSystemPrompt and FollowUpUserPrompt are the messages the API sends to the model. They are here, with the lock, so that one file says what the role may do.
const FollowUpSystemPrompt = `You rewrite a follow-up message in an investigation chat into one question that stands alone.
Use only what the earlier questions say. Never add a number, a date, a plate, a name or an identifier that is not in the earlier questions or in the new message.
If the new message already stands alone, return it unchanged. If it depends on an earlier question, copy what it needs from that question (the subject, the number, the period, the kind of record) and apply the change the new message asks for.
Answer with JSON: {"question": "...", "inherited": ["..."]} where inherited names what you took from the earlier questions (for example "number", "period", "kind of record").`

// FollowUpUserPrompt renders the history and the new message for the model.
func FollowUpUserPrompt(in FollowUpInput) string {
	var b strings.Builder
	b.WriteString("Earlier questions, oldest first:\n")
	for i, h := range in.History {
		q := h.Resolved
		if q == "" {
			q = h.Question
		}
		fmt.Fprintf(&b, "%d. %s\n", i+1, strings.Join(strings.Fields(q), " "))
	}
	fmt.Fprintf(&b, "\nNew message: %s\n", strings.Join(strings.Fields(in.Question), " "))
	return b.String()
}

// FollowUpSchema is the JSON schema the model's answer must follow.
func FollowUpSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question":  map[string]any{"type": "string"},
			"inherited": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
		"required":             []string{"question", "inherited"},
		"additionalProperties": false,
	}
}

// ParseFollowUp reads the model's answer.
func ParseFollowUp(content string) (FollowUpResult, error) {
	var res FollowUpResult
	if err := json.Unmarshal([]byte(content), &res); err != nil {
		return FollowUpResult{}, err
	}
	return res, nil
}
