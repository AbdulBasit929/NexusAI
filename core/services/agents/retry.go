package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
)

const AgentChatRetryObservableType = "chat_retry"

var (
	ErrRetryPayloadConflict = errors.New("idempotency key was already used for a different retry request")
	ErrRetryNotEligible     = errors.New("only failed, timed out, or cancelled requests can be retried")
)

// ChatRetryReservation is the minimal retained lineage needed to make an
// analyst-initiated retry idempotent. Conversation content is deliberately
// excluded; MessageHash binds a key to the exact submitted text.
type ChatRetryReservation struct {
	AgentName          string `json:"agent_name"`
	UserID             string `json:"user_id,omitempty"`
	CaseID             string `json:"case_id"`
	OriginalMessageID  string `json:"retry_of"`
	RetryMessageID     string `json:"message_id"`
	IdempotencyKeyHash string `json:"idempotency_key_hash"`
	MessageHash        string `json:"message_hash"`
}

func (r ChatRetryReservation) sameRequest(other ChatRetryReservation) bool {
	return r.AgentName == other.AgentName &&
		r.UserID == other.UserID &&
		r.CaseID == other.CaseID &&
		r.OriginalMessageID == other.OriginalMessageID &&
		r.IdempotencyKeyHash == other.IdempotencyKeyHash &&
		r.MessageHash == other.MessageHash
}

func ChatRetryMessageHash(message string) string {
	sum := sha256.Sum256([]byte(message))
	return hex.EncodeToString(sum[:])
}

// ChatRetryReservationID is stable for a user/agent/case/idempotency tuple and
// fits the existing 36-character observable primary key.
func ChatRetryReservationID(agentName, userID, caseID, idempotencyKey string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{userID, agentName, caseID, idempotencyKey}, "\x00")))
	return "retry-" + hex.EncodeToString(sum[:15])
}

func IsRetryEligibleStatus(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "timed_out" || status == "cancelled" || status == "error" || strings.HasPrefix(status, "error:")
}

// ChatRetryRegistry provides zero-value, process-local reservation semantics
// for standalone mode. Distributed mode uses AgentStore.ReserveChatRetry.
type ChatRetryRegistry struct {
	mu      sync.Mutex
	entries map[string]ChatRetryReservation
}

func (r *ChatRetryRegistry) Reserve(id string, requested ChatRetryReservation) (ChatRetryReservation, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = make(map[string]ChatRetryReservation)
	}
	if existing, ok := r.entries[id]; ok {
		if !existing.sameRequest(requested) {
			return ChatRetryReservation{}, false, ErrRetryPayloadConflict
		}
		return existing, false, nil
	}
	r.entries[id] = requested
	return requested, true, nil
}
