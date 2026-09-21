# NX-A1 initial architecture reconciliation

Status: pre-implementation inventory complete  
Phase: NX-A1.1 — current architecture inventory  
Boundary: source, contracts, tests, and documentation only

## Decision

NX-A1 will extend the accepted APF-3/STIM architecture in place. It will not
create a second query router, operation catalogue, capability catalogue,
execution engine, Fact Packet, response envelope, Activity store, agent loop,
or UI-specific analytical implementation.

## Existing architecture and disposition

| Concept | Existing authority | Status | Reuse decision | NX-A1 gap/action |
| --- | --- | --- | --- | --- |
| Query routing | `planRuntimeQuery`, `chooseTemplate`, identifier/time/direction extractors, follow-up adapter in `api/forensic_records/query.go`; Agent transport in `core/services/agents/forensic_direct.go` | accepted APF-3 direct and bounded-composition paths | reuse and adapt | expose one shared request/plan/result foundation without expanding regex routing |
| Query intent and entities | `forensics.query-understanding/v1` in `query_intelligence_contracts.go` | typed intent, language, families, original/normalized entities and provenance exist | authoritative | add explicit aliases/adapters for the NX-A1 vocabulary only where they improve contract clarity; do not replace the contract |
| Evidence/time/workspace scope | `ExecutionWorkspaceScopeV1`, `ExecutionEvidenceScopeV1`, `QueryTimeScopeV1`, authenticated `forensicScope`, active-case UI context | case binding and basic evidence/time scope exist | authoritative | formalize selected/prior-result scope, time-domain separation, optional location scope, and bounded Investigation Context validation |
| Query plan / plan step | `forensics.execution-plan/v1`, `GovernedExecutionPlanV1`, `ExecutionStepV1` | single-operation and two/three-step DAGs with typed bindings exist | authoritative | add typed validation result, execution status, and reusable budget validation; preserve existing plan wire format |
| Operation catalogue | 67 current entries from `supportedQueryTemplates()` and `runAnalyticalTemplate` (the accepted APF-3 historical baseline was 65) | execution truth | authoritative | no second catalogue; expose an Operation Contract adapter derived from the executable catalogue and certification ledger |
| Platform catalogue | `contracts/forensic-platform-v1.json`, reconciled by `reconciledForensicPlatformCatalog` | descriptive adapter/agent/model metadata | reuse | keep parity checks and use as supplementary metadata only |
| Capability catalogue | `capabilities.go` plus `buildResolvedCapabilityProjection` | derived from executable templates, certification, workspace counts, authorization, runtime and model roles | authoritative projection | add explicit readiness states/snapshot metadata without hand-authored operation duplication |
| Direct execution | `buildSingleCapabilityPlan`, `validateSingleCapabilityPlan`, `executeGovernedRecordsCapability` | accepted fast path | reuse unchanged in behavior | return the shared validation/result contracts and keep deterministic latency free of mandatory model calls |
| Composition | `buildBoundedCompositionPlan`, dependency/cycle/binding validation, `executeGovernedComposition` | accepted APF-3.7 bounded DAG | reuse and generalize contractually | preserve citations, typed intermediates, partial status and candidate-correlation authority in a shared Tool Result |
| Facts | `forensics.fact-packet/v1` | validated deterministic facts, relationships, citations and bounded synthesis input | authoritative | provide compatibility from shared Tool Results/Answer Envelope; never create a competing fact packet |
| Observations | family-specific observation contracts and enterprise inferred relationships | typed family observations exist, but there is no shared observation packet | extend | add `forensics.observation-packet/v1` with explicit observation/candidate authority and citation requirements |
| Answer envelope | `forensics.enterprise-response/v1` | public typed response with nine distinct result states | authoritative | treat as the NX-A1 Answer Envelope and add observation/plan/validation metadata only additively |
| Citations | `CitationV1`, `ExecutionCitationV1`, Fact Packet citations, contribution lineage | source/evidence/version/row locators exist | reuse and validate | add scope/reference validation across composed results and observation packets |
| History / Activity | `forensics.case-analysis-history/v1` and `forensics.agent-presentation/v1` | scoped lifecycle, answer and metadata persistence | reuse | store only bounded shared-plan/result metadata; prior prose never becomes fact authority |
| Agent execution | LocalAI Agent transport, forensic direct route, forensic sidecar tool, specialists | accepted transport/orchestration boundary | reuse | specialists consume registered capabilities; they do not own scope, facts, or peer delegation |
| Request lifecycle | authenticated endpoint, SSE callbacks, analysis lifecycle, cancellation/retry/timeouts | accepted | reuse | include plan/request/step/result telemetry without raw protected values |
| Visual utility | Investigation Workspace actions/suggestions submit through shared Ask/case APIs | no independent React SQL/analytics found | reuse | define a typed UI action adapter to the same operation request; no UI-side computation |

## Missing shared foundation

The source has most of the required mechanics, but the following are not yet
one explicit, reusable NX-A1 contract surface:

- a bounded `InvestigationContext` that combines authenticated workspace,
  selected evidence, prior-result references, active entities, time/location
  scope, permissions and capability snapshot provenance;
- explicit evidence-scope kinds and calendar-time versus media-source-time
  separation;
- readiness states that preserve `NOT_RUN`, `PROCESSING`,
  `COMPLETE_ZERO_RESULTS`, `COMPLETE_RESULTS`, `FAILED`, and `UNAVAILABLE`;
- a reusable `PlanValidationResult` with errors/warnings, estimated cost and
  budget decisions, instead of error strings as the only validation surface;
- a shared `ToolInvocation`/`ToolResult` contract usable by both the fast path
  and composition path;
- a shared `ObservationPacket` whose authority cannot be mistaken for a
  deterministic fact;
- additive Answer Envelope fields for observations, clarification, partial
  support, plan/validation summaries and citation-preserving tool results;
- a single GUI/Ask request adapter and extension workflow proving that a new
  operation is registered once and consumed everywhere;
- focused NX-A1 tests for these contracts, invalid/unauthorized scope, budget,
  dependency/cycle rejection, authority separation, result-state preservation,
  multilingual identifiers, anti-SQL and extensibility.

## Reuse boundary

NX-A1 will use type aliases or additive fields when an accepted type is already
the semantic authority. New durable contracts are justified only for concepts
that have no existing equivalent: Investigation Context, readiness snapshot,
plan-validation result, shared tool result, and observation packet. Existing
public APIs remain compatible; no database migration or retained execution is
required.

## Implementation sequence

1. Add the missing versioned shared contract types and validation helpers.
2. Adapt the existing executable operation/capability projection into those
   contracts; do not add a hand-maintained operation list.
3. Route existing direct and composition plans through one validation result
   and budget policy while preserving their executors.
4. Adapt deterministic results, observations, citations and result states into
   the existing enterprise response.
5. Add the UI-action request adapter and prove it resolves to the same operation
   contract as Ask.
6. Certify with fixtures and independent invariants, then update the living
   architecture and phase ledgers.

## Approval boundary

No Docker build, service restart/recreation, deployment, database migration,
backfill, retained evidence operation, model/profile/backend change, staging,
commit, push, or PR is authorized by this source phase.
