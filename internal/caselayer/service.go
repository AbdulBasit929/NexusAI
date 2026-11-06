package caselayer

import "time"

// Config is the layer's tuning. Zero values mean the defaults. The API fills it from its environment; the layer reads none.
type Config struct {
	TurnTimeout   time.Duration  // the most a turn may take, queue and engine together (default 10 minutes)
	MaxConcurrent int            // turns in the engine at once (default 2)
	Heartbeat     time.Duration  // the interval of heartbeat events while the engine runs (default 2 seconds)
	Snapshot      SnapshotLimits // what a turn stores of an answer
	EngineVersion string         // the build name recorded with a turn when the engine does not say
	HistoryTurns  int            // earlier turns passed to the follow-up resolver and the engine (default 6)
}

func (c Config) withDefaults() Config {
	if c.TurnTimeout <= 0 {
		c.TurnTimeout = 10 * time.Minute
	}
	if c.MaxConcurrent <= 0 {
		c.MaxConcurrent = 2
	}
	if c.Heartbeat <= 0 {
		c.Heartbeat = 2 * time.Second
	}
	if c.Snapshot.RawMax <= 0 {
		c.Snapshot = DefaultSnapshotLimits
	}
	if c.HistoryTurns <= 0 {
		c.HistoryTurns = 6
	}
	return c
}

// Service is the case layer: conversations that run the engine, and the knowledge, member and authority records of a case.
type Service struct {
	Store    *Store
	Engine   Engine
	FollowUp FollowUpResolver // nil leaves follow-ups as typed
	Config   Config

	slots   *slots
	started time.Time
}

// NewService builds the layer over a store and an engine.
func NewService(store *Store, engine Engine, cfg Config) *Service {
	cfg = cfg.withDefaults()
	return &Service{Store: store, Engine: engine, Config: cfg, slots: newSlots(cfg.MaxConcurrent), started: time.Now()}
}

// tokensLive is a hint, not a promise: the reading and evidence-line tokens inside an answer live in the engine's memory. They can be good only if this process issued them (the turn
// finished after it started) and only for a while (the engine keeps a few hundred and forgets the oldest first).
func (s *Service) tokensLive(t Turn) bool {
	return t.CompletedAt != nil && t.CompletedAt.After(s.started) && time.Since(*t.CompletedAt) < 10*time.Minute
}
