# NX-A1 shared typed query, agent, and tool execution foundation

Status: source complete and certified 2026-08-28  
Contract generation: v1 additive contracts  
Runtime boundary: not deployed by NX-A1

## Decision

NexusAI has one governed analytical execution foundation for visual utility
actions, natural-language Ask, specialist-agent transport, typed case APIs, and
Activity. NX-A1 extends the accepted APF-3 implementation rather than replacing
it.

```text
UI action or analyst question
  -> authenticated tenant/case/collection/subject scope
  -> query understanding or typed operation adapter
  -> bounded Investigation Context
  -> derived Capability Snapshot
  -> allowlisted Query Plan
  -> shared plan/scope/readiness/budget/authority validator
  -> existing deterministic, retrieval, or approved model-backed executor
  -> typed Tool Result
  -> Fact Packet and/or Observation Packet
  -> enterprise Answer Envelope
  -> Ask/UI presentation and scoped Activity
```

The existing `forensics.query-understanding/v1`,
`forensics.execution-plan/v1`, `forensics.fact-packet/v1`, and
`forensics.enterprise-response/v1` contracts remain authoritative. NX-A1 uses
Go aliases for their Query Plan, Plan Step, Operation Contract, Execution
Budget, Entity Reference, Citation, and Answer Envelope meanings. This is an
intentional anti-duplication decision.

## Query intelligence and execution intelligence

Query intelligence normalizes language, preserves identifiers, resolves a
small outcome-oriented intent, identifies candidate families and operations,
applies bounded same-scope follow-up context, and either proposes a typed plan
or requests clarification.

Execution intelligence rebinds authenticated scope, derives current capability
and readiness, validates operation IDs and parameters, checks dependencies,
enforces budgets, selects only an allowlisted implementation, executes the
existing operation, and validates result authority/citations. A language model
cannot perform these policy decisions.

## Query Intent

`QueryIntentV1` reuses the accepted `QueryIntentClass` vocabulary:
`lookup`, `rank`, `aggregate`, `timeline`, `compare`, `correlate`, `summarize`,
`retrieve`, `extract`, `search`, `inspect`, `validate`, `readiness`,
`relationship`, and `media_analysis`. Operation IDs remain specific and come
only from the executable operation catalogue. An intent alone never authorizes
an executor.

## Entity Reference

`EntityReferenceV1` is an alias of the accepted `QueryEntityV2`. It preserves:

- entity type;
- exact original value;
- separately normalized value;
- value origin;
- normalizer identity.

The current turn, deterministic extractors, explicit scope, validated model
proposal, and conversation context are distinguishable origins. Model proposals
cannot silently replace an exact identifier. UUID classification remains ahead
of plate-like patterns.

## Evidence Scope

`EvidenceScopeV1` supports exactly these bounded kinds:

- current workspace;
- selected evidence;
- explicit evidence IDs;
- family-limited scope;
- prior authoritative result.

The collection ID must exactly match the authenticated case/collection. An
explicit or selected scope requires exact server-authoritative evidence IDs. A
prior-result scope requires an authoritative result reference; prior model
prose is not accepted as evidence or facts.

## Time Scope

`TimeScopeV1` distinguishes:

- record/calendar timestamps;
- media source-time seconds;
- event-relative ranges.

Calendar bounds require a timezone and preserve source timezone, date order,
precision, origin, and normalizer where known. Media seconds require one exact
evidence ID and cannot coexist with calendar timestamps. Event-relative scope
requires an authoritative event reference. Validation rejects mixed time
domains and reversed media ranges.

## Location Scope

`LocationScopeV1` can hold a governed location reference or bounded
latitude/longitude/radius with datum and uncertainty. Latitude and longitude
must appear together and remain within valid bounds. Tower/site references
provide cited context only; they do not prove RF coverage, handset position, or
subscriber presence.

## Investigation Context

`forensics.investigation-context/v1` contains one complete workspace scope,
evidence scope, active entities, current question, bounded prior references,
time/location scope, available families, capability-snapshot reference,
permissions, conversation identity, and context provenance.

Tenant, case, collection, subject, and permissions are resolved fresh per
request. Case and collection must match. Conversation state can replace a
target/date/direction only under the accepted same-conversation follow-up
policy; it cannot widen scope, grant permission, or override evidence.

## Operation Contract and registry

`OperationContractV1` aliases `ResolvedCapabilityV1`. It is derived from:

1. `supportedQueryTemplates()` and `runAnalyticalTemplate` execution truth;
2. the risk-based operation-certification ledger;
3. platform adapter/agent/model-role metadata;
4. current authorized workspace data state;
5. runtime/model-role availability.

There is no second hand-maintained NX-A1 operation catalogue. The projection
retains operation/family/intent, required and optional parameters, scope modes,
implementation key, specialist/model roles, result/presentation contract,
citation policy, certification/exposure, limitations, and availability.

The only execution implementation keys admitted by the NX-A1 validator are the
existing governed handlers:

- `runAnalyticalTemplate`;
- `queryKnowledgeBaseEvidence`;
- `hybridQueryHandler`.

## Capability Requirement, discovery, and snapshot

`CapabilityRequirementV1` declares family, parameters, required fields,
model-role IDs, citation policy, and whether Tier-A certification is required.
`forensics.capability-snapshot/v1` materializes requirements and current
readiness from the existing resolved projection. It does not infer capability
from a model file alone.

The request response carries only the selected one-to-three capability entries,
not the entire catalogue, keeping the fast path bounded. Full discovery remains
available through the existing registry/capability APIs.

## Evidence readiness

`DataReadinessState` preserves:

- `NOT_RUN`;
- `PROCESSING`;
- `COMPLETE_ZERO_RESULTS`;
- `COMPLETE_RESULTS`;
- `FAILED`;
- `UNAVAILABLE`.

The pre-execution snapshot derives `PROCESSING`, `NOT_RUN`,
`COMPLETE_RESULTS`, or `UNAVAILABLE` from current capability truth.
`COMPLETE_ZERO_RESULTS` and `FAILED` are also retained by Tool Results after an
attempt. A registered source without a completed accepted processor is never
presented as a negative analytical finding.

## Query Plan and Plan Step

`QueryPlanV1` and `PlanStepV1` alias the accepted governed execution types.
Plans contain a complete workspace, one or more registered capability IDs,
typed parameters, explicit evidence scope, declared dependencies/input
bindings, expected output contract, citation policy, and a read-only budget.

The fast path has exactly one step. Bounded composition currently permits two
or three steps. Dependencies form a validated acyclic graph. Downstream input
bindings name one typed source field and one target parameter; arbitrary JSON
expressions, URLs, code, tool names, and SQL are not representable.

## Plan validation result

`forensics.plan-validation/v1` records:

- overall validity;
- exact scope authorization;
- capability/readiness acceptance;
- dependency validity;
- budget acceptance;
- authority preservation;
- estimated cost class;
- typed error/warning issues.

The shared validator runs after a plan is built and before any operation
executes. Invalid plans are held and returned as a clarification/invalid request
instead of reaching an executor.

## Execution Budget and large-data safety

`ExecutionBudgetV1` aliases the accepted resource policy. Current ceilings are:

- three steps;
- 100 returned rows per step;
- 300 intermediate rows;
- 100,000 maximum offset;
- 20 KB results;
- eight targets;
- 12 retrieval chunks;
- one model call;
- 60 seconds for bounded composition.

Single deterministic operations remain model-free and close to their existing
latency. Large datasets stay inside parameterized SQL/adapter execution; only
bounded rows, aggregates, retrieval passages, facts, and citations can cross
into result packets or model synthesis.

## Tool Invocation and Tool Result

`forensics.tool-invocation/v1` binds request, plan, step, operation, workspace,
evidence scope, typed parameters, read-only policy, timeout, and idempotency
key.

`forensics.tool-result/v1` carries operation/result contract, execution status,
public result state, authority class, typed intermediate fields, deterministic
facts, observation packets, bounded rows, citations, limitations, failure code,
row accounting, truncation, and duration.

The adapter is shared by direct and composition execution. Composition returns
bounded fields and row accounting rather than copying intermediate datasets.

## Fact Packet

`forensics.fact-packet/v1` remains the only model-synthesis fact authority. It
contains bounded deterministic facts, relationships, citations, scope,
warnings, limitations, and presentation hints. NX-A1 does not create a second
Fact Packet. The existing validator rejects changed numbers, identifiers,
citations, certainty, relationships, or forbidden inference and falls back to
the deterministic narrative.

## Observation Packet

`forensics.observation-packet/v1` is the new shared envelope for model
observations, semantic retrieval statements, and candidate correlations. Every
observation must declare one of those non-factual authority classes and cite a
same-scope source. Validation rejects deterministic-fact authority inside an
Observation Packet.

Bounded composition exposes a cited `candidate_correlation` observation only
when the underlying steps provide citations. It explicitly does not prove
identity, ownership, association, causation, guilt, motive, or intent.

## Answer Envelope

`AnswerEnvelopeV1` aliases `forensics.enterprise-response/v1`. Additive NX-A1
fields carry Tool Results, Observation Packets, Plan Validation, and an optional
Clarification Request. Existing deterministic findings, tables,
visualizations, citations, model interpretation, limitations, next actions, and
execution trace remain compatible.

## Result states and execution statuses

Public states remain:

- `results_present`;
- `complete_zero_results`;
- `no_match_for_filter`;
- `not_processed`;
- `processing`;
- `failed`;
- `unavailable`;
- `unauthorized`;
- `invalid_request`.

Execution status separately preserves planned, running, completed, partial,
skipped, failed, cancelled, and timed-out lifecycle. Validation prevents
complete-zero from containing rows, no-match from masquerading as an
unexecuted request, and not-processed from masquerading as completed.

## Citation propagation

Tool Result citations contain tenant/collection, evidence/version, source name,
and bounded locator. Finding, fact, observation, visualization, and composed
claim references resolve to those citation IDs. Validation rejects missing IDs
and cross-collection citations. Representative citations remain distinct from
complete contribution lineage.

## Clarification, unsupported capability, and partial results

`forensics.clarification-request/v1` holds a reason code, one focused question,
missing fields, optional choices, and `execution_held=true`. Ambiguous operation
or missing required target paths produce this contract without execution.

Unknown operation IDs and unavailable/readiness-gated capabilities return the
existing typed unsupported/unavailable response. Composition preserves each
successful or zero-result step; dependency skips or failures produce partial
analysis and never a complete-correlation claim.

## Authority model

The shared authority classes are:

- deterministic fact;
- deterministic derivation;
- model observation;
- semantic retrieval;
- candidate correlation;
- LLM interpretation.

Tool Result validation permits only deterministic authority in its fact list.
Observation Packet validation permits only observation, retrieval, or candidate
authority. LLM interpretation remains separately labeled in the Answer
Envelope and cannot become either list.

## No unrestricted SQL

Plans have typed parameter fields and registered operation IDs; they have no
SQL/code/command/URL/tool-name slot. The public typed case-query adapter rejects
prohibited execution fields recursively, including `sql`, `raw_sql`,
`query_sql`, `statement`, `command`, arbitrary executables/tool URLs, and
implementation-key overrides. Executors use existing parameterized SQL
templates. Model output cannot mint an operation or implementation key.

## Direct operation fast path

High-confidence known questions and typed UI actions resolve one registered
operation, derive current readiness, build one step, pass the shared validator,
and call the existing executor. No mandatory model call or multi-stage planner
was added. The response still carries the same understanding, plan-validation,
tool-result, Answer Envelope, and Activity semantics as composition.

## Dynamic composition path

The current production admission remains intentionally narrow: two or three
read-only registered capabilities with explicit typed dependencies. The plan is
validated as a complete DAG before execution. Results are topologically merged;
citations and limitations remain per step. Partial dependency failure stops the
dependent step. NX-A1 establishes the contract for broader previously unseen
questions; NX-B1 will add useful operations and later phases may widen
composition only through tests and approval.

## Cross-family planning

Cross-family plans declare every operation and dependency explicitly. Exact
identifier or time-window bindings remain typed. Combined claims are candidate
correlations unless a separately certified deterministic relationship operation
establishes a stronger governed state. Specialists never call each other; the
case orchestrator owns plan, budget, scope, and merge semantics.

## Visual utility and Ask shared engine

The typed case-query API (`forensics.query-plan/v1`) is the visual-action
adapter. It converts a registered operation ID, entities, time, filters, and
limit into the same legacy request adapter used by Ask, which then enters the
same capability projection, plan builder, validator, executor, Tool Result, and
Answer Envelope. React does not calculate analytical results.

An operation-equivalence test proves a typed “frequent contacts” action and the
natural-language Ask path select the same operation and executor contract.

## Data, Ask, Activity, and advanced application integration

- Data consumes evidence/capability state and may submit a typed action; it does
  not own operation logic.
- Ask sends the natural question and bounded context through the same forensic
  sidecar route.
- Activity stores the answer and bounded presentation/technical metadata under
  tenant/user/agent/case/collection scope. Stored model prose is presentation,
  never a future fact source.
- Advanced `/app` continues to use the same agent transport, technical
  provenance, retry, cancellation, lifecycle, and History contracts.

No broad UI change is required for NX-A1.

## Agent, LLM, and model roles

Specialist agents advertise families/operations and execute assigned typed
steps. They do not own truth, scope, permission, IDs, or peer delegation.

An LLM may assist unresolved language with a schema-bound proposal and may
explain a validated Fact Packet. It cannot execute arbitrary tools, author SQL,
widen scope, alter exact values, or upgrade observations to facts.

Model roles remain hardware-neutral and are resolved only from accepted runtime
and product capability. NX-A1 changes no model, profile, backend, or LocalAI
runtime.

## Telemetry and reproducibility

Existing request and query telemetry remains authoritative. NX-A1 adds bounded
plan ID, selected capability snapshot, validation decisions, step/invocation
IDs, operation, result state, row count, truncation, duration, and error
category. It records no chain-of-thought and need not log raw protected values.

The plan, operation/catalog revision, parameters, evidence scope, citations,
and result states are sufficient for a future reproducibility record without a
database migration in this phase.

## How to add a new certified analytical operation

1. Implement the bounded read-only executor in the owning family module.
2. Add one entry to the authoritative executable operation catalogue with a
   stable operation ID, family, inputs, calculation, result/presentation,
   scope, and limitations.
3. Add independent fixture/oracle evidence to the operation-certification
   ledger and declare exposure/suggestion eligibility truthfully.
4. Declare required fields, data/model readiness, citation policy, and accepted
   authority class.
5. Add operation, parameter, calculation, order/tie, zero/no-match/readiness,
   citation, scope, budget, and presentation tests.
6. Add multilingual/follow-up variants in the existing query corpus when the
   operation is available through Ask.
7. Use the typed case-query contract for a visual action; do not add React
   calculation logic or a second route.

The derived capability projection then makes the operation available to Ask,
typed UI actions, specialists, the plan validator, Tool Results, presentation,
and Activity. A new operation does not require separate operation logic in each
consumer.

## Compatibility and impact

- Existing APIs remain valid; all response changes are additive.
- Existing query templates and executors remain execution truth.
- No database schema or migration is required.
- No retained evidence or Activity is mutated for certification.
- No runtime service, Compose file, model, profile, backend, or deployment is
  changed.
- NX-B1 may add all-major-family baseline operations through this extension
  workflow; it must not reinvent planner contracts.

## NX-B1 breadth extension

NX-B1 exercised this extension workflow without changing its architecture.
Twelve family operations were registered through the existing executable
catalogue, bringing it from 67 to 79 entries. Records operations continue to
use `runAnalyticalTemplate`; bounded document/image/audio derived-artifact
search uses the allowlisted `derivedTextEvidence` implementation; existing KB
and hybrid implementations are unchanged.

The typed case-query adapter now preserves exact video evidence identity and
source-second bounds for `video.timeline`. Its returned derived artifacts are
not facts: the shared Tool Result adapter labels them `model_observation` and
wraps them in `forensics.observation-packet/v1` with evidence/version/source
citations and limitations. Document, image-OCR and audio-transcript searches
remain `semantic_retrieval`. No new contract, planner, registry, React
calculation or Activity path was introduced.
