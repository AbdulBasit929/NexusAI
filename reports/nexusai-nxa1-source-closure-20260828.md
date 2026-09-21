# NexusAI NX-A1 shared typed execution foundation — source closure

# Verified Starting State

NX-UX1 was closed and live. APF-3 already supplied typed query understanding,
single-operation and bounded-composition plans, 67 executable operations, a
derived capability projection, certified Fact Packets, enterprise responses,
citations, agent transport/presentation and scoped Activity persistence. The
worktree contained substantial pre-existing user work; NX-A1 preserved it and
made no destructive Git operation.

# Runtime Baseline

Read-only inspection on 2026-08-28 found:

- LocalAI/UI `nexusai-api-1`: image `sha256:628f56af541ac739887ddfb6f08d661e1070c4d0a2d343fb902882ab0f5dba9a`, running and healthy.
- Forensic API: image `sha256:d2992062b8f55334d602530eb96cec8289dd714f0ae9523d3b27edebb724b5c5`, running, no container healthcheck, `unless-stopped`.
- Worker: image `sha256:be13dbec4bd9bb62c824140df3777b898d7ad2801a0e68cd512c00dafaf4f9ef`, running and healthy, `unless-stopped`.
- NATS: image `sha256:e4bf19f15fd3218814a4e3c9e0064e1334bd8aa20d5984b9f1a0afd084f8cc00`, healthy.
- PostgreSQL: image `sha256:61f891691050da6032023c01ea885730eeeba06b7c17b403e7d0b9c49c37dfe9`, healthy.

# NX-UX1 / Runtime-Hardening Reconciliation

NX-A1 preserved the accepted NX-UX1 visual/Ask/Activity architecture and the
runtime-hardening decision that only forensic API and worker use
`restart: unless-stopped`. It did not recreate, restart or reconfigure a
service. The read-only runtime view is evidence, not activation of this source.

# Existing Query Architecture Inventory

## Query Routing

`POST /query/hybrid` remains the authoritative language route. Typed case
actions adapt `forensics.query-plan/v1` into that same path.

## Operation Registry

The 67-entry executable template catalogue and its real handlers remain
execution truth. NX-A1 created no second catalogue.

## Capability Registry

`ResolvedCapabilityV1` remains the derived projection of operation metadata,
certification, workspace data, authorization, runtime and model-role state.

## Context

APF-3 conversation provenance and follow-up rules remain authoritative. NX-A1
adds a bounded, validated Investigation Context around them.

## Result Contracts

`forensics.fact-packet/v1` and `forensics.enterprise-response/v1` remain the
authoritative fact and answer contracts. NX-A1 adds Tool Results and Observation
Packets without replacing them.

## Citations

Existing evidence/version/source locators and aggregate-contribution lineage
remain authoritative and are propagated into the new packets.

## History / Activity

Existing tenant/user/agent/case/collection-scoped analysis history remains the
only Activity store. Stored narrative is presentation, not future fact input.

## Agent Execution

Existing deterministic forensic agent routing and bounded presentation remain
the transport. Specialists do not receive scope or truth authority.

## Request Lifecycle

Existing request IDs, cancellation, timeout, retry and telemetry continue.
NX-A1 adds plan/snapshot/step/invocation identity and validation decisions.

# Reuse Decisions

Go aliases reuse the accepted entity, operation, plan, plan-step, budget,
citation and answer-envelope types. Existing direct/composed executors, Fact
Packet validation, presentation and Activity are extended, not forked.

# Architectural Gaps

The starting architecture lacked one explicit Investigation Context, selected
capability/readiness snapshot, shared post-plan validator, Tool Invocation/Tool
Result contract, Observation Packet, authority vocabulary and typed
Clarification Request. NX-A1 fills those gaps.

# NX-A1 Shared Architecture

## QueryIntent

Typed intent class plus qualifiers and value origin; operation IDs still come
only from the registry.

## EntityReference

Reuses `QueryEntityV2` and preserves exact original, normalized value, type,
normalizer and origin.

## EvidenceScope

Supports current workspace, selected/explicit evidence, family-limited and
prior-authoritative-result scopes. Collection widening fails closed.

## TimeScope

Separates calendar/record timestamps, media source seconds and event-relative
time. Mixed domains and reversed source-time ranges are rejected.

## LocationScope

Supports cited reference IDs or bounded coordinates/radius/datum/uncertainty;
coordinates are validated and do not imply physical presence.

## InvestigationContext

`forensics.investigation-context/v1` binds authenticated workspace, evidence,
entities, question, bounded prior references, time/location, families,
permissions, conversation and provenance.

## CapabilitySnapshot

`forensics.capability-snapshot/v1` contains only the selected one-to-three
derived capabilities and their current readiness.

## OperationContract

Aliases `ResolvedCapabilityV1`, including registered operation, executor key,
inputs, result contract, citations, certification, limitations and availability.

## CapabilityRequirement

Declares families, parameters/fields, model roles, citation and certification
requirements.

## QueryPlan

Aliases the accepted `forensics.execution-plan/v1` governed plan.

## PlanStep

Aliases the accepted typed step with operation, parameters, scope,
dependencies/bindings, expected result and citation requirement.

## PlanValidationResult

`forensics.plan-validation/v1` records scope, readiness, dependency, budget and
authority decisions plus typed issues before execution.

## ExecutionBudget

Reuses the APF-3 read-only resource policy and ceiling.

## ToolInvocation

`forensics.tool-invocation/v1` binds request/plan/step/operation, exact scope,
typed parameters, timeout and idempotency.

## ToolResult

`forensics.tool-result/v1` carries lifecycle/result state, authority, typed
fields, facts, observations, rows, citations, limitations and accounting.

## FactPacket

The accepted Fact Packet remains the only bounded deterministic input to model
synthesis.

## ObservationPacket

`forensics.observation-packet/v1` carries cited model observations, semantic
retrieval or candidate correlations and rejects deterministic-fact authority.

## AnswerEnvelope

The enterprise response is additively extended with Tool Results, Observation
Packets, Plan Validation and Clarification.

## Citation

Reuses `CitationV1`; validators require same-collection, resolvable IDs.

## ClarificationRequest

`forensics.clarification-request/v1` carries a reason, focused question,
missing fields/options and `execution_held=true`.

## ResultState

Preserves all nine accepted public states, including complete zero, filter miss
and not processed as distinct values.

## ExecutionStatus

Separately preserves planned, running, completed, partial, skipped, failed,
cancelled and timed-out lifecycle.

# Query Intelligence vs Execution Intelligence

Language interpretation may propose typed meaning. Only execution intelligence
rebinds scope, derives readiness, validates the plan and invokes an allowlisted
executor. A model cannot authorize or execute.

# Direct Operation Fast Path

One resolved capability produces one validated step and calls the existing
executor without a mandatory model call.

# Dynamic Composition Path

Two/three-step compositions use the existing typed DAG, bindings and bounded
executor. Plans validate completely before any step executes.

# Cross-Family Planning

Every operation/dependency is explicit. Combined outputs remain cited candidate
correlations unless a separately certified deterministic operation proves more.

# Capability Discovery

Snapshots are derived from the actual executable catalogue and current
projection, never from a hand-written model/test map.

# Evidence Readiness

The contract preserves `NOT_RUN`, `PROCESSING`, `COMPLETE_ZERO_RESULTS`,
`COMPLETE_RESULTS`, `FAILED` and `UNAVAILABLE`.

# Authority Model

## Deterministic Facts

Only registered deterministic execution may emit facts.

## Deterministic Derivations

Calculated counts/aggregates retain a distinct derivation label.

## Model Observations

Remain cited observations with model metadata/confidence/limitations.

## Semantic Retrieval

Remains cited non-factual evidence, not a database fact.

## Candidate Correlations

Require citations and explicit non-proof limitations.

## LLM Interpretation

Remains separately labeled narrative validated against the Fact Packet.

# Result-State Preservation

Tool Result validation and the Answer Envelope preserve public state through
direct/composed presentation and Activity compatibility.

# Complete Zero vs No Match

Completed analysis over a complete scope with zero results is distinct from a
filter that matched no records.

# Not Processed vs Zero

Skipped/unavailable processing cannot be presented as completed-zero.

# Citation Propagation

Direct provenance and composition citations flow into Tool Results, facts,
observations, findings and presentation without changing scope.

# Clarification

Ambiguous operation or missing required parameter returns a typed held request
and executes nothing.

# Unsupported Capability

Unknown or unavailable operations retain typed unavailable/unsupported
semantics and do not fall through to an arbitrary executor.

# Partial Results

Successful independent composition steps remain visible; failed/dependent
steps are labeled and no complete correlation is claimed.

# Execution Budget

Ceilings: three steps, 100 returned rows/step, 300 intermediate rows, offset
100,000, 20 KB results, eight targets, 12 retrieval chunks, one model call and
60 seconds.

# Large-Data Safety

Full datasets stay in parameterized server-side operations. Only bounded rows,
aggregates, passages, facts and citations enter packets or synthesis.

# No-Unrestricted-SQL Enforcement

Plans expose no SQL slot. The public typed adapter recursively rejects SQL,
statements, commands, executables, tool/callback URLs, arbitrary tools and
implementation-key overrides. Executor keys are allowlisted.

# Tenant / Workspace Authorization

Plan, context and evidence collection must exactly match authenticated
tenant/case/collection/subject scope; missing query permission fails closed.

# Follow-Up Context

Only accepted same-conversation typed references can supply bounded context;
they cannot widen scope or promote prior prose to evidence.

# Multilingual Query Support

English, messy English, Roman Urdu, Urdu and mixed-language normalization reuse
the accepted deterministic/query-assistance paths.

# Identifier Preservation

Tests preserve exact phone identifiers across Roman Urdu, Urdu and mixed
queries; original and normalized values remain separate.

# Visual Utility / Ask Shared Engine

Typed visual actions and Ask both resolve through the same registry, validator,
executor, Tool Result and Answer Envelope. React performs no analysis.

# Data Integration

Data exposes readiness and submits typed operations; it does not own analytical
logic.

# Ask Integration

Ask uses the same forensic route and bounded context.

# Activity Integration

The existing scoped store retains the same typed presentation/result meanings.

# Advanced /app Compatibility

Existing agent transport, lifecycle, retry, technical details and History are
unchanged. Focused agent/presentation tests and the production build pass.

# Operation Extension Workflow

Add one bounded executor and authoritative descriptor, declare readiness/
authority/citations, certify with independent fixtures/oracles, add contract/
domain/language tests, and let Ask/GUI/Agent/Activity consume the derived shared
contracts. Do not duplicate behavior per consumer.

# Files Changed

NX-A1 source: `nxa1_execution_foundation.go`, its Ginkgo specs, additive fields
in `query.go`, `platform_contracts.go` and `cases.go`. Architecture, feature,
roadmap, continuation, maturity and operation-matrix documentation was updated.
The required initial reconciliation is
`reports/nexusai-nxa1-initial-architecture-reconciliation-20260828.md`.

# Contract Tests

Focused NX-A1 Ginkgo passes, including aliases, scopes, time domains, readiness,
authority, result states, multilingual IDs, UI/Ask equivalence and extension.

# Planner / Validator Tests

Direct validation, unauthorized scope, budget overflow, unapproved executor,
dependency-cycle rejection and recursive execution-field rejection pass.

# Existing Query Regression

Full `go test ./api/forensic_records -count=1`: PASS, 74.212s.

# Agent Regression

Focused forensic records-tool/presentation Ginkgo: PASS, 4.187s. Six standalone
forensic tests: PASS, 4.329s. Full package: 105/119 pass; 14 database specs are
environment-blocked before assertions by Windows rootless-Docker testcontainers.

# Analyst Regression

`node --test core/http/react-ui/src/analyst/*.test.js`: 32/32 PASS.

# Advanced App Regression

No advanced-app source changed. Shared agent/presentation tests pass and the
whole React production bundle builds, preserving generic `/app` compatibility.

# Build

`npm --prefix core/http/react-ui run build`: PASS; 684 modules transformed.

# Lint

`go vet ./api/forensic_records ./core/services/agents`: PASS. `gofmt -d` for all
NX-A1 Go integration files is empty. Repository-wide ESLint remains red on six
pre-existing React-hook errors in untouched `e2e/coverage-fixtures.js` and
`src/pages/Chat.jsx`; NX-A1 adds no React lint finding.

# Git Diff Check

`git diff --check`: PASS; only existing line-ending conversion warnings.

# Anti-Hardcode Scan

NX-A1 production paths contain no demo tenant/case/evidence/identifier answer.
The only SQL-related literals are recursive denylist keys; existing registered
parameterized SQL remains executor implementation truth.

# Runtime Impact

None. Source was not activated.

# Protected Services

LocalAI/UI, forensic API, worker, PostgreSQL and NATS were inspected read-only
and not changed.

# Retained State

No evidence, Activity, volume, collection or retained result was mutated.

# Model Impact

No model download, replacement, role or profile change.

# Database Impact

No schema change, migration, backfill or database write.

# Open P0

0.

# Open NX-A1 P1

0. The global lint and Windows testcontainer limitations are pre-existing
repository/environment issues, not NX-A1 source defects.

# Deferred P2 / P3

Broader family-operation breadth, richer cross-family workflows, live
all-family acceptance and any wider composition admission belong to NX-B1/B2.

# NX-A1 Status

Source complete. Success standard satisfied without unrestricted SQL or a new
hardcoded prompt template per question.

# NX-B1 Handoff

NX-B1 must register useful all-major-family operations into these contracts,
preserve authority/readiness/citations, and avoid family-specific consumer
implementations. NX-B1 has not started.

# Exact Next Phase

NX-B1 — All-Major-Family Breadth Baseline.

# Exact Next Action

Stop and wait for a new explicit directive. If authorized later, begin NX-B1
with a bounded family-operation/readiness gap inventory; do not deploy.

## Closure markers

```text
NXA1Source=PASS
ExistingArchitectureReconciled=PASS
NoDuplicateArchitecture=PASS
SharedQueryContracts=PASS
QueryIntentContract=PASS
EntityReferenceContract=PASS
EvidenceScopeContract=PASS
TimeScopeContract=PASS
InvestigationContextContract=PASS
OperationContract=PASS
OperationRegistryAdapter=PASS
CapabilityRegistryAdapter=PASS
CapabilityDiscovery=PASS
EvidenceReadiness=PASS
QueryPlanContract=PASS
PlanStepContract=PASS
PlanValidation=PASS
PlanDependencyValidation=PASS
PlanCycleRejection=PASS
ExecutionBudget=PASS
LargeDataBoundedExecution=PASS
DirectOperationPath=PASS
CompositionDryRun=PASS
CrossFamilyPlanDryRun=PASS
ClarificationPath=PASS
UnsupportedCapabilityPath=PASS
PartialSupportPath=PASS
ToolResultContract=PASS
FactPacketCompatibility=PASS
ObservationPacketContract=PASS
AnswerEnvelopeContract=PASS
FactObservationAuthority=PASS
CandidateCorrelationAuthority=PASS
CitationPropagation=PASS
ResultStatePreservation=PASS
CompleteZeroVsNoMatch=PASS
NotProcessedVsZero=PASS
TenantScopeBinding=PASS
EvidenceScopeBinding=PASS
UnauthorizedScopeFailClosed=PASS
NoUnrestrictedSQL=PASS
FollowUpContext=PASS
IdentifierPreservation=PASS
EnglishQuerySupport=PASS
RomanUrduQuerySupport=PASS
UrduQuerySupport=PASS
MixedLanguageQuerySupport=PASS
VisualUtilitySharedEngineFoundation=PASS
ConversationalSharedEngineFoundation=PASS
OperationExtensionWorkflow=PASS
ExistingStructuredQueryRegression=PASS
APF3Regression=PASS
AnalystRegression=PASS
GenericAppRegression=PASS
ProductionHardcodeScan=PASS
GitDiffCheck=PASS
OpenP0=0
OpenNXA1SourceP1=0
RuntimeMutated=false
RetainedStateMutated=false
ModelsChanged=false
ProfilesChanged=false
BackendsChanged=false
DatabaseMigration=false
DeploymentPerformed=false
NXA1=COMPLETE
ExactNextPhase=NX-B1
```
