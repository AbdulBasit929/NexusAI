# NexusAI query, agent, and tool architecture vNext

Status: approved target architecture; implementation phase-gated.

## Design laws

1. Deterministic truth precedes model prose.
2. Authorization precedes routing and execution.
3. Identifiers are typed before family-specific regex extraction.
4. Every operation is allowlisted, versioned, bounded, and independently tested.
5. Specialists do not call specialists. The orchestrator owns the plan.
6. Models may explain grounded facts, not invent facts, identity, ownership,
   co-location, or causal relationships.
7. Result states survive every boundary without collapse.

## Logical flow

```text
Analyst question
  -> QueryIntent (language, typed IDs, time/place/family constraints)
  -> Evidence authorization and policy
  -> QueryPlan (ordered allowlisted operations + budgets)
  -> Nexus orchestrator
       -> family adapter / deterministic operation
       -> optional specialist tool
       -> optional LocalAI primitive
  -> ToolResult(s)
  -> canonical merge and contradiction accounting
  -> FactPacket
  -> deterministic answer
  -> optional grounded LocalAI synthesis
  -> AnswerEnvelope -> Ask / History / API / export / audit
```

## Contracts

### QueryIntent

Contains original and normalized language, typed identifiers (`evidence_id`,
`case_id`, phone, IMSI/IMEI, IP, plate, tower/cell, entity), temporal and spatial
constraints, requested analytical class, follow-up references, and ambiguity.
UUID and exact evidence/case references are recognized before plate/phone regexes.

### QueryPlan

Contains tenant/case/evidence scope, policy decision, ordered operation IDs and
versions, typed parameters, maximum rows/time/tool calls, expected output state,
and explanation permission. Plans are serializable and auditable. No free-form
SQL, shell, connector, or MCP method enters an analyst plan.

### ToolResult

Carries authoritative rows and count, processing state, quality accounting,
source and canonical citations, warnings, missing fields, operation version,
latency, and typed failure. A rendered summary cannot override these values.

### FactPacket

Contains proven facts, candidate observations, contradictions, missing evidence,
source-field mappings, canonical transformations, operation/model provenance,
and synthesis constraints. It is the only factual input to answer synthesis.

### AnswerEnvelope

The result state is one of `results_present`, `complete_zero_results`,
`no_match_for_filter`, `not_processed`, `processing`, `failed`, `unavailable`,
`unauthorized`, or `invalid_request`. It includes deterministic findings,
optional explanation, citations, quality/limitations, follow-up context, audit
ID, and progressive-disclosure technical metadata.

## Agent and tool topology

The orchestrator is Nexus-owned. Family specialists are policy-limited tool
providers: CDR, IPDR, subscriber, tower/location, ANPR/geospatial, and forensic
records. New financial/log/document/media specialists are admitted only when
their operation families have independent oracles. Direct peer calls are
forbidden; cross-family work is an explicit orchestrator plan.

LocalAI Agents may host the loop after contract certification, but receive only
short-lived scoped tool capabilities. MCP is a transport boundary: analyst,
service, and admin servers/credentials are separate. Administrative install,
edit, toggle, or upgrade tools never appear in an analyst session.

## Multilingual and follow-up semantics

Normalization preserves original text and script. English, messy English,
Roman Urdu, and Urdu map to the same typed intent without translating evidence
values. Follow-ups resolve against an explicit conversation state containing
prior plan, evidence scope, result state, and cited entities. Low-confidence
pronouns or identifiers trigger clarification rather than scope expansion.

## Failure and performance behavior

Each stage has timeout, cancellation, row, tool-call, token, and concurrency
budgets. Operation timeout yields a typed failure; model timeout falls back to
the deterministic answer. Partial results are labeled and never presented as
complete. Retries are idempotent and bounded. Audit telemetry correlates every
plan/tool/packet/answer while redacting secrets and unnecessary PII.

## Admission path

NX-1 closes current truth defects. NX-2 introduces the contracts behind
compatibility adapters. NX-3 certifies intent/planning. Later family slices
promote operations from C/B to A only with source, runtime, API, UI, and
independent-oracle evidence.
