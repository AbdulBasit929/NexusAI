package caselayer

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"
)

// Engine is how a question is answered. The API supplies the production one, which posts the question to the Ask engine's own handler in this process, exactly as POST /query/hybrid does;
// the specs supply a scripted one. The layer never looks inside the engine and never edits what it returns.
type Engine interface {
	Ask(ctx context.Context, req EngineRequest) (EngineResult, error)
}

// EngineRequest is one question. Question is what the engine must answer (the follow-up resolved to a question of its own when the resolver is on); Original is what the analyst typed.
type EngineRequest struct {
	Scope     Scope
	CaseID    string
	ThreadID  string
	TurnID    string
	Question  string
	Original  string
	History   []HistoryTurn
	RequestID string
}

// HistoryTurn is what an earlier turn of the branch contributes to the context of the next one: the question, the question that was sent, the frame the engine read and the headline it gave.
type HistoryTurn struct {
	TurnID   string          `json:"turn_id"`
	Question string          `json:"question"`
	Resolved string          `json:"resolved_question,omitempty"`
	Headline string          `json:"headline,omitempty"`
	Outcome  string          `json:"outcome,omitempty"`
	Frame    json.RawMessage `json:"frame,omitempty"`
}

// EngineResult is the engine's answer exactly as it was returned (Response is the JSON of POST /query/hybrid), with the HTTP status of that call, the engine's frame of the question
// (what it understood: numbers, plates, dates, limits; empty when it does not say) and the build that produced it.
type EngineResult struct {
	Response      json.RawMessage
	Status        int
	RequestID     string
	Frame         json.RawMessage
	EngineVersion string
}

// ---- the stream of a turn --------------------------------------------------------------------------------------------------------------------------------------------------

// Event is one object of the stream that POST /conversations/{id}/turns writes (NDJSON or server-sent events). Only true phases are sent: the layer never invents progress.
//
// In a conversation across cases the question is put to each case in turn: the events of one case's turn carry case_id and group_id, the clock (at_ms) runs over the whole question, a "group"
// event carries the summary of what was asked, and the single "done" comes after it.
type Event struct {
	Event     string `json:"event"` // phase | heartbeat | turn | group | done | error
	Phase     string `json:"phase,omitempty"`
	Label     string `json:"label,omitempty"`
	TurnID    string `json:"turn_id,omitempty"`
	ThreadID  string `json:"thread_id,omitempty"`
	CaseID    string `json:"case_id,omitempty"`
	GroupID   string `json:"group_id,omitempty"`
	Position  int    `json:"position,omitempty"` // in the queue for the model, on a queued phase
	ElapsedMS int64  `json:"elapsed_ms,omitempty"`
	AtMS      int64  `json:"at_ms"`
	Turn      *Turn  `json:"turn,omitempty"`
	Diff      *Diff  `json:"diff,omitempty"`
	Group     *Group `json:"group,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
}

// EventSink receives the events of a turn. An implementation must not block for long and must ignore its own write errors (a client that went away does not stop the turn).
type EventSink interface{ Send(Event) }

type nopSink struct{}

func (nopSink) Send(Event) {}

// SinkFunc adapts a function to an EventSink.
type SinkFunc func(Event)

func (f SinkFunc) Send(e Event) { f(e) }

// phases is the clock and the sink of one turn.
type phases struct {
	sink  EventSink
	start time.Time
	turn  string
	th    string
}

func newPhases(sink EventSink, threadID, turnID string) *phases {
	if sink == nil {
		sink = nopSink{}
	}
	return &phases{sink: sink, start: time.Now(), turn: turnID, th: threadID}
}

func (p *phases) at() int64 { return time.Since(p.start).Milliseconds() }

func (p *phases) phase(name, label string) {
	p.sink.Send(Event{Event: "phase", Phase: name, Label: label, TurnID: p.turn, ThreadID: p.th, AtMS: p.at()})
}

func (p *phases) queued(position int) {
	p.sink.Send(Event{Event: "phase", Phase: "queued", Label: "Waiting for the language model", Position: position, TurnID: p.turn, ThreadID: p.th, AtMS: p.at()})
}

// ---- phases reported by the engine -----------------------------------------------------------------------------------------------------------------------------------------

type phaseEmitterKey struct{}

// WithPhaseEmitter puts into ctx the function that forwards a phase of the engine's own work to the stream of the turn.
func WithPhaseEmitter(ctx context.Context, emit func(phase, label string)) context.Context {
	return context.WithValue(ctx, phaseEmitterKey{}, emit)
}

// EmitPhase is what an engine calls at a boundary it already has (the frame is read, the query is written, it is checked, it runs, the evidence is traced). It does nothing when the
// context carries no emitter, so it is safe to call from any path. The phase reaches the stream as "engine.<phase>".
func EmitPhase(ctx context.Context, phase, label string) {
	if emit, ok := ctx.Value(phaseEmitterKey{}).(func(phase, label string)); ok && emit != nil {
		emit(phase, label)
	}
}

// ---- the queue for the model -----------------------------------------------------------------------------------------------------------------------------------------------

// slots limits how many turns are in the engine at once. The model answers one request at a time, so a longer queue only moves the waiting from the model to the API; it is kept to a
// couple so that a second analyst's turn is not refused while the first is answered.
type slots struct {
	ch      chan struct{}
	waiting atomic.Int32
}

func newSlots(n int) *slots {
	if n < 1 {
		n = 1
	}
	return &slots{ch: make(chan struct{}, n)}
}

// acquire waits for a free slot. It returns false if ctx ends first. onQueue is called once with the place in the queue when the turn has to wait.
func (s *slots) acquire(ctx context.Context, onQueue func(position int)) bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
	}
	pos := int(s.waiting.Add(1))
	defer s.waiting.Add(-1)
	if onQueue != nil {
		onQueue(pos)
	}
	select {
	case s.ch <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *slots) release() { <-s.ch }
