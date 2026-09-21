# NexusAI query architecture gap analysis — 2026-08-25

## Current strengths

Deterministic-first querying, canonical records, source preservation,
case/evidence scope, Fact Packets, citations, six specialists, and 67 governed
operations already exist. Mature structured paths have multilingual evidence.

## Priority gaps

| Priority | Gap | Consequence | Closure |
|---|---|---|---|
| P1 | UUID parsed as plate | wrong agent/operation | identifier pass before family regex; regression |
| P1-live | public row/count contradiction | positive facts appear empty | activate tested row fix |
| P1-live | completed-zero state loss | valid media appears generic | activate typed UI/history fix |
| P2 | 62/67 operations not A-certified | breadth exceeds trust | family certification slices |
| P2 | no shared typed plan envelope | direct/agent/API/UI drift | versioned QueryPlan/ToolResult |
| P2 | limited entity/time fusion | manual correlation | bounded timeline/entity graph ops |
| P2 | technical leakage in default UI | cognitive load | progressive disclosure |
| P2 | LocalAI discovery/version drift | unsafe reuse assumptions | provenance/capability admission |
| P3 | weak performance/failure budgets | unpredictable behavior | stage telemetry and fallback |

## Required contracts

1. `QueryIntent`: language, normalized constraints, typed identifiers, requested
   operation class, and ambiguity evidence.
2. `QueryPlan`: authorized evidence scope, ordered allowlisted operations,
   bounded parameters, expected state, and cost budget.
3. `ToolResult`: operation/version, authoritative rows/counts, quality,
   canonical/source citations, warnings, errors, and processing state.
4. `FactPacket`: grounded facts, candidates, contradictions, missing evidence,
   provenance, and synthesis permissions.
5. `AnswerEnvelope`: typed state, deterministic answer, optional explanation,
   citations, limitations, context, and audit ID.

Routing is identifier-first and authorization-before-execution. Models may
explain but cannot add facts. Every state remains distinct through API, Ask,
History, export, and audit. Ambiguity produces clarification or labeled
hypotheses, never invented identity/ownership.

NX-1 closes only the three P1s. A new orchestration framework requires a later
phase approval.
