package agents

import (
	"context"
	"errors"
	"sync"
	"time"
)

// DefaultAgentChatTimeout is the governed whole-request execution limit shared
// by local and distributed agent paths.
const DefaultAgentChatTimeout = 210 * time.Second

const (
	requestStatusTTL        = 30 * time.Minute
	requestStatusMaxEntries = 4096
)

// AgentRequestStatus is the bounded, read-only lifecycle snapshot used to
// reconcile one request after an event-stream interruption.
type AgentRequestStatus struct {
	AgentName string    `json:"agent_name"`
	UserID    string    `json:"-"`
	CaseID    string    `json:"case_id"`
	MessageID string    `json:"message_id"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RequestStatusRegistry retains recent lifecycle snapshots without persisting
// conversation content or creating a retained-state dependency.
type RequestStatusRegistry struct {
	mu      sync.Mutex
	entries map[string]AgentRequestStatus
}

func requestStatusKey(agentName, userID, caseID, messageID string) string {
	return AgentCancellationKey(agentName, userID, caseID, messageID)
}

func (r *RequestStatusRegistry) Record(agentName, userID, caseID, messageID, status string) {
	if agentName == "" || caseID == "" || messageID == "" || status == "" {
		return
	}
	now := time.Now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = make(map[string]AgentRequestStatus)
	}
	status = canonicalAnalysisStatus(status)
	key := requestStatusKey(agentName, userID, caseID, messageID)
	if previous, ok := r.entries[key]; ok && IsAnalysisTerminalStatus(previous.Status) {
		return
	}
	r.pruneLocked(now)
	if len(r.entries) >= requestStatusMaxEntries {
		var oldestKey string
		var oldest time.Time
		for key, entry := range r.entries {
			if oldestKey == "" || entry.UpdatedAt.Before(oldest) {
				oldestKey, oldest = key, entry.UpdatedAt
			}
		}
		delete(r.entries, oldestKey)
	}
	r.entries[requestStatusKey(agentName, userID, caseID, messageID)] = AgentRequestStatus{
		AgentName: agentName, UserID: userID, CaseID: caseID, MessageID: messageID,
		Status: status, UpdatedAt: now,
	}
}

func (r *RequestStatusRegistry) Get(agentName, userID, caseID, messageID string) (AgentRequestStatus, bool) {
	now := time.Now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		return AgentRequestStatus{}, false
	}
	r.pruneLocked(now)
	status, ok := r.entries[requestStatusKey(agentName, userID, caseID, messageID)]
	return status, ok
}

func (r *RequestStatusRegistry) pruneLocked(now time.Time) {
	cutoff := now.Add(-requestStatusTTL)
	for key, entry := range r.entries {
		if entry.UpdatedAt.Before(cutoff) {
			delete(r.entries, key)
		}
	}
}

// AgentChatDeadlineUnixMilli returns the absolute deadline attached when work
// is accepted, ensuring distributed queue time is part of the request limit.
func AgentChatDeadlineUnixMilli() int64 {
	return time.Now().Add(DefaultAgentChatTimeout).UnixMilli()
}

// WithAgentChatDeadline creates a request context from an absolute dispatch
// deadline. A zero value supports legacy events by starting the governed limit
// when the worker receives them.
func WithAgentChatDeadline(parent context.Context, deadlineUnixMilli int64) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(DefaultAgentChatTimeout)
	if deadlineUnixMilli > 0 {
		deadline = time.UnixMilli(deadlineUnixMilli)
	}
	return context.WithDeadline(parent, deadline)
}

// ExecutionTerminalStatus maps context termination to a stable lifecycle state.
// Callers retain ownership of publishing the terminal event so each request
// emits one correlated outcome.
func ExecutionTerminalStatus(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timed_out"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	default:
		return "error"
	}
}
