package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/internal/caselayer"
	"github.com/mudler/LocalAI/pkg/httpclient"
)

// THE CASE LAYER (migration 034): conversations that run the Ask engine, and the notes, findings, tags, members and authority records of a case. The logic lives in internal/caselayer, which imports
// nothing from this package; this file is the glue: who is asking, how a question reaches the engine, and the routes. Design: docs/architecture/CONVERSATIONS_AND_CASE_LAYER_20261103.md.
//
// The routes answer 503 with a sentence until migration 034 is applied, so this code can be deployed before the migration and the migration applied at the owner's word.
//
// Switches: FORENSIC_CONVERSATION_FOLLOWUP=true turns on the follow-up resolver (a model role, OFF until measured); FORENSIC_CONVERSATIONS_TURN_TIMEOUT (default 10m),
// FORENSIC_CONVERSATIONS_MAX_CONCURRENT_TURNS (default 2) and FORENSIC_ENGINE_VERSION (a build name recorded with every turn) tune it.
const caseLayerFollowUpEnv = "FORENSIC_CONVERSATION_FOLLOWUP"

// caseLayerScope turns a request into who is asking. A credential bound to one case sees only that case (BoundCase); the agent worker never uses the case layer.
func caseLayerScope(r *http.Request, caseID string) (caselayer.Scope, error) {
	scope, err := bindForensicScope(r, strings.TrimSpace(r.URL.Query().Get("tenant_id")), strings.TrimSpace(caseID), "", "")
	if err != nil {
		status, code := http.StatusForbidden, "forbidden"
		if errors.Is(err, errForensicUnauthenticated) {
			status, code = http.StatusUnauthorized, "unauthenticated"
		}
		return caselayer.Scope{}, &caselayer.Error{Status: status, Code: code, Message: err.Error()}
	}
	if scope.ActorRole == "agent-worker" {
		return caselayer.Scope{}, caselayer.Forbidden("The agent worker cannot use conversations or case notes.")
	}
	// With authentication on, the subject is the verified one; with it off (a development setup) the headers the workspace sends are taken as they come.
	subject := "anonymous"
	for _, candidate := range []string{scope.SubjectID, scope.ActorID, r.Header.Get(forensicSubjectHeader), r.Header.Get(forensicActorHeader)} {
		if candidate = strings.TrimSpace(candidate); candidate != "" {
			subject = candidate
			break
		}
	}
	return caselayer.Scope{TenantID: scope.TenantID, Subject: subject, ActorRole: scope.ActorRole, BoundCase: scope.CollectionID}, nil
}

// caseLayerPhaseLabels are the sentences for the phases of the engine's own work; emitPhase sends one to the stream of the turn that is running.
var caseLayerPhaseLabels = map[string]string{
	"understanding": "Reading the question",
	"writing_query": "Writing the query",
	"checking":      "Checking the query against the question",
	"running":       "Running it on the case's records",
	"evidence":      "Tracing the records behind the figure",
	"composing":     "Writing the answer",
}

// emitPhase reports a phase of the engine's work to the stream of the turn being run (a no-op when the call is not part of a turn). The engine calls it at the boundaries it already has.
func emitPhase(ctx context.Context, name string) {
	caselayer.EmitPhase(ctx, name, caseLayerPhaseLabels[name])
}

// caseLayerEngine answers a question by posting it to the Ask engine's own handler in this process, exactly as POST /query/hybrid would (the way forensicCaseQueryV1Handler does). The
// request context carries the caller's principal, so the engine applies the same scope and the same walls as for a direct call.
type caseLayerEngine struct {
	handler http.HandlerFunc
	version string
}

func (e *caseLayerEngine) Ask(ctx context.Context, req caselayer.EngineRequest) (caselayer.EngineResult, error) {
	emitPhase(ctx, "understanding")
	frame := caseLayerFrame(req.Question)
	body, err := json.Marshal(map[string]any{"tenant_id": req.Scope.TenantID, "user_id": req.Scope.Subject, "collection_id": req.CaseID, "query": req.Question})
	if err != nil {
		return caselayer.EngineResult{}, err
	}
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, "/query/hybrid", bytes.NewReader(body))
	if err != nil {
		return caselayer.EngineResult{}, err
	}
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("X-Request-ID", req.RequestID)
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, hr)
	return caselayer.EngineResult{
		Response:      append(json.RawMessage(nil), rec.Body.Bytes()...),
		Status:        rec.Code,
		RequestID:     rec.Header().Get("X-Request-ID"),
		Frame:         frame,
		EngineVersion: e.version,
	}, nil
}

// caseLayerFrame is what the engine's own frame reader finds in a question, as data a later turn can be read against: identifiers, plates, dates, limits, cues. National ID numbers are
// counted and never kept.
func caseLayerFrame(question string) json.RawMessage {
	f := askExtractFrame(question)
	months := make([]map[string]int, 0, len(f.Months))
	for _, m := range f.Months {
		months = append(months, map[string]int{"year": m.Year, "month": m.Month})
	}
	cues := make([]string, 0, len(f.Cues))
	for c, on := range f.Cues {
		if on {
			cues = append(cues, c)
		}
	}
	sort.Strings(cues)
	out, err := json.Marshal(map[string]any{
		"numbers": f.Numbers, "imeis": f.IMEIs, "serials": f.Serials, "accounts": f.Accounts, "ips": f.IPs, "cells": f.Cells, "plates": f.Plates,
		"dates": f.Dates, "months": months, "years": f.Years, "limit": f.Limit, "hops": f.Hops, "cues": cues, "relative": f.Relative, "national_id_count": len(f.NationalIDs),
	})
	if err != nil {
		return nil
	}
	return out
}

// caseLayerFollowUp is the follow-up resolver: one schema-constrained call to the language model. It is switched on by FORENSIC_CONVERSATION_FOLLOWUP and measured before it is relied on; the
// layer applies the number lock to whatever it returns.
type caseLayerFollowUp struct{ cfg config }

func (f caseLayerFollowUp) Resolve(ctx context.Context, in caselayer.FollowUpInput) (caselayer.FollowUpResult, error) {
	if f.cfg.LocalAIURL == "" || f.cfg.SynthesisModel == "" {
		return caselayer.FollowUpResult{}, fmt.Errorf("no language model is configured")
	}
	payload, err := json.Marshal(map[string]any{
		"model": f.cfg.SynthesisModel, "temperature": 0, "max_tokens": 160,
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "forensic_followup_resolver", "strict": true, "schema": caselayer.FollowUpSchema()}},
		"messages": []map[string]string{
			{"role": "system", "content": caselayer.FollowUpSystemPrompt},
			{"role": "user", "content": caselayer.FollowUpUserPrompt(in)},
		},
	})
	if err != nil {
		return caselayer.FollowUpResult{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(callCtx, http.MethodPost, strings.TrimRight(f.cfg.LocalAIURL, "/")+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return caselayer.FollowUpResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if f.cfg.LocalAIAPIKey != "" {
		request.Header.Set("Authorization", "Bearer "+f.cfg.LocalAIAPIKey)
	}
	response, err := httpclient.NewWithTimeout(90 * time.Second).Do(request)
	if err != nil {
		return caselayer.FollowUpResult{}, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return caselayer.FollowUpResult{}, fmt.Errorf("the language model answered HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return caselayer.FollowUpResult{}, err
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Choices) != 1 || envelope.Choices[0].FinishReason != "stop" {
		return caselayer.FollowUpResult{}, fmt.Errorf("the language model's answer could not be used")
	}
	res, err := caselayer.ParseFollowUp(envelope.Choices[0].Message.Content)
	if err != nil {
		return caselayer.FollowUpResult{}, err
	}
	res.Model = f.cfg.SynthesisModel
	return res, nil
}

// caseLayerEngineVersion names the build recorded with every turn: FORENSIC_ENGINE_VERSION when set (the compose file can carry the image tag), otherwise the revision the binary was built from.
func caseLayerEngineVersion() string {
	if v := env("FORENSIC_ENGINE_VERSION", ""); v != "" {
		return v
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		rev, modified := "", ""
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				if s.Value == "true" {
					modified = "+modified"
				}
			}
		}
		if rev != "" {
			if len(rev) > 12 {
				rev = rev[:12]
			}
			return rev + modified
		}
	}
	return "unknown"
}

// registerCaseLayer adds the case layer's routes to the mux. With no database the routes answer 503.
func registerCaseLayer(mux *http.ServeMux, cfg config, db *pgxpool.Pool) {
	engine := &caseLayerEngine{handler: hybridQueryHandler(cfg, db), version: caseLayerEngineVersion()}
	maxTurns, err := strconv.Atoi(env("FORENSIC_CONVERSATIONS_MAX_CONCURRENT_TURNS", "2"))
	if err != nil || maxTurns < 1 {
		maxTurns = 2
	}
	svc := caselayer.NewService(caselayer.NewStore(db), engine, caselayer.Config{
		TurnTimeout:   envDuration("FORENSIC_CONVERSATIONS_TURN_TIMEOUT", 10*time.Minute),
		MaxConcurrent: maxTurns,
		EngineVersion: engine.version,
	})
	if envBool(caseLayerFollowUpEnv, false) {
		svc.FollowUp = caseLayerFollowUp{cfg: cfg}
	}
	(&caselayer.Handler{Svc: svc, Scope: caseLayerScope}).Register(mux)
}
