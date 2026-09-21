# NexusAI APF-3 unified query intelligence architecture

Status: architecture and dependency reconciliation complete, 2026-08-17  
Implementation: not started  
Authority: APF-3 directive, `NEXUSAI_MASTER_DIRECTIVE.md`, and the accepted
APF/R0-R7 contracts  
Deployment boundary: documentation only; no runtime, schema, model, retained
data, Docker, or frontend change

## Decision

NexusAI owns forensic query orchestration. The authoritative owner is a thin
query-intelligence layer in the forensic service (`api/forensic_records`),
called by the existing LocalAI agent transport. It resolves an authenticated
question into typed understanding, authorized capabilities, and a bounded plan;
then existing deterministic operations, Knowledge Base retrieval, accepted
family tools/models, and the response compiler execute that plan.

The LocalAI agent loop and models remain useful language and inference
runtimes. They do not own workspace scope, operation identity, evidence
availability, authorization, forensic facts, citation validity, or execution
policy. The Analyst Portal continues to submit a normal question through the
existing case-scoped Ask API and does not learn family, specialist, tool, model,
collection, SQL, or adapter selection.

```text
question + authenticated case + bounded prior context
  -> NexusAI query understanding
  -> authorized, data-aware capability resolution
  -> governed plan
  -> deterministic | retrieval | accepted model/tool | bounded composition
  -> factual and citation validation
  -> existing response/presentation compiler
  -> SSE response + retained case history
```

## Verified current state

The current path is operational and must be evolved in place:

1. `core/http/react-ui/src/analyst/AnalystAsk.jsx` mounts the portal mode of
   `core/http/react-ui/src/pages/AgentChat.jsx`.
2. `AgentChat.handleSend` derives only bounded prior target/template/date/
   direction context and uses the shared agent-chat client. SSE is consumed by
   the same page; presentation metadata is rendered by `ForensicPresentation`.
3. `POST /api/agents/:name/chat` is registered in
   `core/http/routes/agents.go` and handled by
   `core/http/endpoints/localai/agents.go:ChatWithAgentEndpoint`. The endpoint
   requires matching case/collection scope, checks access, and bounds supplied
   conversation context.
4. `core/services/agentpool/agent_pool.go:ChatForUserWithForensicScope` binds a
   request-specific collection to a copied agent configuration, creates the
   analysis lifecycle record, and dispatches locally or through NATS.
5. `core/services/agents/forensic_direct.go` sends every non-empty forensic
   question to the governed hybrid query tool. Explicit templates use the same
   route; they are not a separate execution system.
6. `core/services/agents/records_tools.go:ForensicHybridQueryTool.Run` calls the
   forensic sidecar `POST /query/hybrid`, forwarding subject, actor, tenant and
   collection scope.
7. `api/forensic_records/query.go:hybridQueryHandler` rebinds scope server-side,
   normalizes input, calls `planRuntimeQuery`, applies bounded follow-up
   context, validates target and capability guards, and dispatches a registered
   template through `runAnalyticalTemplate`, KB retrieval, or both.
8. Exact operations use parameterized, read-only SQL against the authorized
   forensic PostgreSQL scope. KB calls are bounded and evidence lineage is
   enriched from the evidence catalog. Optional synthesis receives bounded
   aggregates/previews, never raw full tables.
9. `buildEnterprisePayload` and the typed
   `forensics.enterprise-response/v1` bridge produce findings, tables,
   visualizations, limitations, citations and execution trace.
10. `core/services/agents/forensic_presentation.go` turns the response into the
    bounded Agent Chat presentation contract. Agent callbacks emit tool,
    lifecycle, final message, and metadata events.
11. `core/services/agents/analysis_history.go` persists the query, terminal
    status, answer and metadata under tenant/user/agent/case/collection scope.
    `agent_history.go` serves cursor-paginated authorized History.

The existing `/api/v1/forensics/cases/{case_id}/query` typed API is a governed
compatibility/product API over the same query implementation. APF-3 must not
create an Analyst Portal-only router or duplicate the Advanced Workspace path.

## Inventory and classification

| Existing concern | Current authority | APF-3 disposition |
| --- | --- | --- |
| Natural-language routing | `chooseTemplate`, `classifyIntent`, target/date/direction extractors in `query.go`; direct aliases in `forensic_direct.go` | `reuse_and_compose`; adapt behind typed understanding, then retire duplicate alias knowledge |
| Executable operation catalog | `supportedQueryTemplates()` and `runAnalyticalTemplate` | `reuse_unchanged` for execution; make its descriptors authoritative |
| Platform operation/specialist catalog | `contracts/forensic-platform-v1.json` | `extend_non_breaking`; reconcile descriptor parity before capability planning |
| Evidence-family capability registry | `capabilities.go` plus coverage/evidence counts | `reuse_and_compose`; separate implementation, runtime, data, permission and maturity states |
| Entity/target/date normalization | family-aware helpers in `query.go` and ingestion adapters | `reuse_and_compose`; expose typed provenance and never allow silent model rewrites |
| Follow-up | `ForensicConversationContext`, UI derivation and `applyConversationContext` | `extend_non_breaking`; add turn/scope provenance and expiry rules |
| Clarification/no-result/unsupported | handler guards and enterprise status compiler | `extend_non_breaking`; formalize mutually exclusive reason codes |
| Knowledge retrieval | `queryKnowledgeBaseEvidence`, `kbSearch`, collection APIs, `knowledge.go` | `reuse_and_compose`; capability-scoped retrieval with mandatory cited passages |
| Agent bindings | 16 manifests and six operational structured specialists | `reuse_and_compose`; specialists remain family executors, not policy owners |
| Model roles | 10 hardware-neutral roles in the platform catalog | `reuse_and_compose`; join with LocalAI runtime capability discovery |
| Response/presentation | enterprise v1, `buildEnterprisePayload`, `forensic_presentation.go` | `reuse_unchanged` initially; add new result statuses non-breakingly |
| Citations/provenance | enterprise citations, KB citations, evidence lineage, source locators | `reuse_and_compose`; validator enforces existence and scope |
| History/observability | analysis history, SSE lifecycle, `QueryTelemetry`, execution trace | `extend_non_breaking`; retain understanding/plan summaries without private reasoning |

## Catalog truth and reconciliation requirement

Source inspection verifies:

- 65 executable query templates: 62 records, one KB, two hybrid;
- record-family declarations include CDR, IPDR, ANPR, subscriber, tower,
  transaction, access-log, generic, and all/cross-family scopes;
- target-required, case-or-target, and case-wide input modes are published with
  measures, grouping, calculation, example, presentation and limitations;
- 19 evidence-family capability definitions are materialized against current
  workspace coverage;
- the older platform catalog contains 50 operation descriptors, 16 specialist
  manifests and 10 model roles.

The 65/50 difference is catalog drift, not permission to discard 15 accepted
operations. APF-3 must derive one versioned capability view from executable
operation descriptors plus evidence-family, specialist and model-role metadata.
A parity invariant must fail tests when an executable operation lacks a
descriptor or a descriptor points to no executor. Until parity exists, the
65-template catalog remains execution truth and the platform catalog remains
supplementary metadata; neither may silently override the other.

## Query understanding contract

Introduce `forensics.query-understanding/v1` without replacing the public Ask
request. Conceptual fields, using current names where possible:

```text
QueryUnderstandingV1
  contract_version
  original_question
  normalized_question
  language { tag, source: deterministic|model, confidence? }
  intent { class, qualifiers[] }
  families { primary?, candidates[], kind: single|multi|neutral|knowledge }
  entities[] { type, original, normalized, normalizer, source }
  targets[] { entity_ref, parameter, source: current_turn|context|explicit_scope }
  time_scope { from?, to?, timezone, source }
  filters[]
  requested_output { result_kind, sort?, limit? }
  continuation { prior_analysis_id?, inherited_fields[], reset_reason? }
  candidate_capability_ids[]
  clarification { required, reason_code?, question?, missing_fields[] }
  confidence { overall?, components? }
```

Confidence ranks or requests clarification; it never authorizes execution.
Original and normalized identifier values are both retained. Family-specific
normalizers own phone, IMSI/ICCID/IMEI, plate, IP/domain and time semantics.
Model output may propose typed values but cannot silently replace a
deterministically normalized identifier.

## Intent, entity and parameter model

The durable intent vocabulary is small and result-oriented:

`lookup`, `rank`, `aggregate`, `timeline`, `compare`, `correlate`, `summarize`,
`retrieve`, `extract`, `search`, `inspect`, `validate`, `readiness`,
`relationship`, and `media_analysis`.

Operations remain specific (`cdr.temporal_activity`, `anpr.timeline`, and so
on). An intent does not imply a family, and family is not a required single
label. Supported entity types begin with current normalized fields: phone/
MSISDN, subscriber/service reference, IMSI, ICCID/SIM, IMEI/device, plate,
camera, tower/site/cell, location, IP, domain, account, file/evidence, and a
bounded generic entity. This is routing metadata, not the R15 entity graph.

Every parameter records its source: explicit request, deterministic extraction,
validated model proposal, conversation context, or explicit evidence scope.
Required parameters are checked against the capability input contract before a
plan exists.

## Follow-up context

Context is a bounded, auditable convenience, never authority:

- case, collection, tenant and user are resolved fresh for every request;
- a workspace change invalidates all inherited forensic context;
- only the immediately preceding completed answer in the same conversation and
  scope may supply target, selected entity, operation family, date range or
  direction;
- inheritance occurs only for explicit continuation/modifier language;
- the current turn overrides inherited values and a new standalone question
  clears stale filters;
- findings, citations and permissions are never treated as executable inputs;
- every inherited field records prior analysis/turn and is visible in the
  technical trace.

The first implementation may preserve the current one-turn fields, but must add
scope/turn identity and resolution provenance before broader follow-up support.

## Capability registry decision

Do not add a second APF registry. Create an authoritative resolved projection
from the current operation catalog, family capabilities, platform catalog,
workspace coverage, authorization and LocalAI runtime discovery.

```text
ResolvedCapabilityV1
  capability_id (= stable operation/tool ID)
  version
  family_ids[]
  intent_classes[]
  executor { mode, implementation_key, specialist_id? }
  inputs { required[], optional[], entity_types[], time_support }
  scopes { case_wide, target, source }
  result_contract
  presentation_type
  data_requirements[]
  model_role_ids[]
  citation_required
  limitations[]
  maturity
  availability {
    implemented, runtime_ready, data_ready, authorized, model_ready, reason_codes[]
  }
```

`implemented`, `runtime_ready`, `data_ready`, `authorized`, `model_ready`, and
`maturity` remain independent. For example, registered ANPR image inference
with no accepted model is not executable; a deterministic ANPR operation with
no Ready ANPR rows is implemented but data-unavailable.

Resolution intersects understood intent/families/entities with this projection.
Only known, authorized, currently executable capability IDs reach the planner.
The language model cannot mint IDs. If no candidate survives, the result is a
specific clarification, unavailable, unauthorized, processing-incomplete, or
unsupported outcome—not an invented substitute.

## Governed planning and execution

Introduce `forensics.execution-plan/v1`:

```text
ExecutionPlanV1
  contract_version
  request_id
  workspace_scope { tenant_id, case_id, collection_id, subject_id }
  mode: single_operation|retrieval|model_tool|multi_operation|multi_family
        |clarification|unsupported
  steps[] {
    step_id, capability_id, depends_on[], validated_parameters,
    evidence_scope, expected_result_contract, citation_required
  }
  output_contract
  resource_budget
  policy_decisions[]
```

Steps contain typed parameters, never SQL, code, arbitrary URLs, or model-made
tool names. Validation rechecks scope, allowlist, inputs, availability, model
role and output contract immediately before execution.

Current limits are retained: result limit 100, offset 100,000, KB results 20,
cross-family targets 8, cross-family relation cap 1,000, query SQL timeout 30
seconds, forensic sidecar call 135 seconds, and whole Agent Chat request 210
seconds. Multi-step count/model-call/retrieval-token budgets are future
configuration and must be benchmarked; they are not invented in this ADR.

### Execution modes

- **Deterministic structured:** preferred for the 65 accepted operations and
  exact CDR/IPDR/ANPR/subscriber/tower/transaction/log/generic facts.
- **Knowledge retrieval:** workspace-scoped bounded passages, optional accepted
  reranking, citation validation, then synthesis; no passage means abstention.
- **Family model/tool:** hardware-neutral accepted role such as `anpr_detector`,
  `ocr`, `asr`, or `text_reasoning`; product and runtime readiness must both pass.
- **Hybrid/composed:** bounded dependency steps whose intermediate results and
  citations remain typed. Relationships preserve exact match, observed
  association, candidate correlation, conflict, and no-evidence semantics.

Process-once/query-many is the default for media and documents. Ask queries
stored derived observations first; it requests governed processing only when
no accepted durable derivative exists and policy explicitly permits it.

### Deterministic fast path

High-confidence matches to a known accepted operation bypass model understanding
and multi-step planning:

```text
normalize -> legacy router adapter -> validate capability/parameters/scope
          -> existing operation -> validate facts/citations -> compile
```

This preserves accepted latency and business logic. The adapter emits the same
typed understanding/plan records as slower paths, so observability is uniform.

### LLM-assisted path

A schema-bound language call is allowed for messy grammar, Urdu/Roman Urdu,
ambiguous entity extraction, intent/operation ranking, and later bounded plan
proposals. Its output is treated as an untrusted proposal. NexusAI validates
schema, normalizes identifiers deterministically, intersects known capabilities,
checks authorization/data/model readiness, and either builds a plan or asks a
specific clarification. LocalAI function/tool calling, JSON output, agents and
model serving are reusable mechanisms, not forensic policy authorities.

## Knowledge, media and cross-family extension points

Knowledge uses the existing collection and KB APIs with the same workspace
identity. A retrieval capability returns cited passages/locators; synthesis may
only describe those passages and must abstain when none validate. APF-4 can add
document extraction without changing the APF-3 planner contract.

Media capabilities expose stored observations or a separately governed
processing action. Model roles are hardware-neutral; runtime selection uses
accepted model-role configuration plus LocalAI `/v1/models/capabilities` and
runtime discovery. No unavailable role is silently substituted.

Cross-family plans are dependency graphs over approved capabilities, not prose
chains. Each join declares entity key, time window, relation semantics and
input citation sets. Synthesis cannot promote a candidate correlation to a
confirmed fact.

## Result semantics and validation

The compiler receives only validated execution results with a reasoned status:

- `matched`: evidence-backed result;
- `no_matching_evidence`: executable capability ran and returned zero;
- `data_unavailable`: relevant Ready evidence is absent;
- `processing_incomplete`: evidence exists but is not query-ready;
- `capability_unavailable`: implementation/runtime/model gate is absent;
- `invalid_parameters`: supplied target/filter cannot be accepted;
- `unauthorized`: scope or capability denied (without leakage);
- `clarification_required`: safe, minimal missing/ambiguous input question;
- `unsupported`: no registered capability performs the request.

The factual validator compares identifiers, numbers, dates/times, locations,
plates, rows and citation locators to typed tool results. Model prose may omit
facts but cannot change them. Citation IDs must resolve to authorized evidence
and versions in the same workspace. Existing `EnterpriseResponseV1` and
`forensic_presentation.go` remain the response boundary; additive statuses,
capability IDs and trace summaries are the only anticipated API evolution.

## Security and observability

Every request reuses the existing HTTP access check and forensic sidecar scope
binding. Conversation state cannot widen scope. Executors are read-only and
allowlisted. Model inputs are bounded, minimize sensitive fields, and never
contain a wider evidence set than the validated plan.

The private execution trace records request/analysis ID, scope identity,
understanding version, resolved intent/families, candidate and selected
capabilities, plan/step IDs, operations/tools/model roles, retrieval counts,
context provenance, policy decisions, warnings and elapsed time. It records no
chain-of-thought. The collapsed analyst Technical Details view may show method,
data sources, capabilities, accepted model role, elapsed time and warnings.

Performance targets are measured per stage. The first implementation establishes
baseline histograms for normalization/understanding, resolution, planning,
execution and first useful response. The accepted deterministic route is the
regression ceiling; no arbitrary new numeric SLO is asserted before measurement.

## Evaluation strategy

Extend, do not replace, the current 65-operation matrix, natural question pack,
R6.6 corpora, family query/answer corpus, specialist tests and APF CDR runtime
artifacts. A golden is contract-first, not prose-first:

```text
question + workspace/data prerequisites
-> expected intent, families, entities, parameters
-> expected capability/operation or clarification
-> execution yes/no
-> expected result semantics and citation requirements
```

Required partitions are canonical, natural, short, messy grammar, typo, Roman
Urdu, Urdu, mixed language, follow-up, missing parameter, ambiguity, no-result,
unsupported, multi-family, workspace-data-unavailable and authorization/scope
isolation. Deep live execution initially covers accepted CDR; future families
remain contract fixtures until their own gates pass.

Metrics are intent, family, entity and parameter accuracy; capability selection;
clarification correctness; unsupported correctness; unsafe execution rate; and
multi-family plan correctness. Wrong workspace, target, family, unauthorized or
mutating execution is a critical failure. The unsafe-execution target is zero.

## Dependency matrix

| Component | Exists | Reuse | Extension | New | Blocking |
| --- | --- | --- | --- | --- | --- |
| Query normalization | yes | family helpers | provenance/language | typed normalized view | no |
| Language detection | partial corpus/phrases | current Urdu/Roman Urdu cases | schema-bound fallback | typed language result | no |
| Intent extraction | partial (`queryIntent` + templates) | deterministic rules | durable intent vocabulary | understanding adapter | yes for assisted path |
| Entity extraction | yes, regex/family rules | exact normalizers | typed source/provenance | entity contract | yes |
| Date/time extraction | yes | parser/timezone fields | provenance/ambiguity | none initially | no |
| Follow-up context | yes, bounded fields | one-turn behavior | turn/scope identity and expiry | context resolver contract | yes for APF-3.4 |
| Operation resolver | yes, 65 templates | execution/catalog | single authority/parity | adapter | yes: 65/50 drift |
| Capability registry | yes, 19 families | coverage materialization | split availability dimensions | resolved projection | yes |
| Planner | partial `runtimePlan` | deterministic plan | approved step graph | execution-plan v1 | yes |
| Executor | yes | template/KB/tool paths | plan-step dispatcher | none initially | no |
| Specialist dispatch | yes | family manifests/tools | capability-bound dispatch | none initially | no |
| RAG dispatch | yes | KB search/lineage | cited retrieval capability | APF-4 extraction later | no |
| Model-role dispatch | catalog/runtime exists | role IDs/model serving | runtime capability join | resolver adapter | later model paths |
| Validator | partial | guards/synthesis checks | plan, fact and citation validation | composed validator | yes |
| Response compiler | yes | enterprise/presentation contracts | additive statuses/trace | none initially | no |
| Citation resolver | yes | lineage and evidence identity | strict step-output validation | none initially | no |
| History | yes | scoped lifecycle | understanding/plan summaries | none initially | no |
| Observability | partial | telemetry/execution trace | stage/policy/context fields | metrics | no |

## API and compatibility decision

The existing `POST /api/agents/:name/chat`, SSE, status/cancel/retry and history
APIs can carry APF-3 without a breaking request change. `case_id`,
`collection_id`, question and bounded context remain sufficient. New internal
contracts are persisted/streamed as additive metadata. The typed case query API
also delegates to the same orchestrator. Advanced Workspace, Responses API
agent routing and direct forensic tools remain compatible.

No new public endpoint is justified for APF-3.1. Discovery endpoints may later
publish the resolved capability schema additively. LocalAI's current official
platform capabilities—functions/tools, JSON/grammar output, in-process agents,
KB/semantic search, model capability discovery, and distributed workers—are
reused behind NexusAI governance. Runtime discovery answers what LocalAI can
serve; the NexusAI registry answers what this analyst may execute in this
workspace.

## Bounded implementation slices

1. **APF-3.1 — Contract and legacy-router adapter.** Add query-understanding v1
   and execution-plan v1 types; adapt existing deterministic routing to emit
   them; enforce 65-operation descriptor parity; preserve query response/API/UI.
2. **APF-3.2 — Resolved capability projection.** Join operation, family,
   workspace coverage, authorization, specialist and runtime/model states;
   return typed reason codes.
3. **APF-3.3 — Governed single-capability planner.** Validate one selected
   capability and dispatch existing deterministic or KB execution with common
   result/fact/citation validation.
4. **APF-3.4 — Auditable follow-up and clarification.** Add turn-bound context,
   reset/isolation, minimal clarifications and golden coverage.
5. **APF-3.5 — Schema-bound language assistance.** Use an accepted model only
   for unresolved language understanding/ranking; validate every proposal and
   retain the deterministic fast path.
6. **APF-3.6 — Knowledge/model extension contracts.** Plug APF-4 retrieval and
   later accepted media model roles into the same capability/executor boundary.
7. **APF-3.7 — Bounded composition and acceptance.** Add typed dependency
   steps, relation semantics, cross-family validation, full evaluation and one
   separately authorized live CDR acceptance.

## Selected first implementation slice and result

APF-3.1 was the selected first source slice. It introduces typed internal contracts
and an adapter around `planRuntimeQuery`; it does not change business SQL,
database schema, public request shape, Analyst Portal, model inventory, or
retained data. Its exit gate is: all 65 accepted operations retain current
routing/results, every operation has one descriptor/executor/citation contract,
clarification and unsupported paths emit typed outcomes, and existing focused
R6.6/Agent Chat tests pass with new contract goldens.

That statement describes the architecture-reconciliation phase. On 2026-08-17,
APF-3.1 through APF-3.3 were subsequently implemented and source accepted:
versioned typed understanding/plan contracts, one derived 65-operation
workspace-aware capability projection, and a governed single-capability
dispatcher now wrap the existing executors. The reconciled platform registry
retains all 50 historical descriptors and derives the 30 missing query
descriptors, for 80 total entries with 65/65 executable parity.

On 2026-08-18 APF-3.4 through APF-3.6 were source accepted. Follow-up context
is now turn/scope/expiry bound and field-provenanced; unresolved language may
use only a schema-constrained, allowlisted model proposal after the
deterministic fast path; and knowledge/model capabilities share typed scope,
citation, abstention, role, runtime, maturity and derived-artifact contracts.
The existing execution split is 62 deterministic, one KB and two hybrid
operations. APF-3.7 remains not started.

Operation correctness is tracked independently in
`api/forensic_records/contracts/operation-certification-v1.json`. Registry and
routing parity do not imply factual certification: the 2026-08-18 baseline is
0 independently certified and 65 blocked pending independent golden fixtures.
The automated registration gate makes that debt explicit and prevents a new
operation from becoming silently executable without a correctness contract.

## 2026-08-18 final implementation status

APF-3.7 is now source accepted. The orchestrator accepts either the preserved
single-operation plan or a governed `bounded_composition` of two or three
registered read-only capabilities. Composition adds typed dependency edges and
input bindings, true topological ordering, exact scope propagation, bounded
intermediate/retrieval/model/time resources, per-step citations, partial-failure
semantics and `candidate_correlation` claim lineage. Unknown/duplicate tools,
arbitrary SQL/URLs, cycles, invalid bindings, scope widening, unavailable data,
uncited facts and resource overflow fail before execution.

The certification baseline was superseded by an explicitly authorized
risk-based product gate. Tier A contains four fully certified queryable daily
operations; Tier B contains 50 bounded operations exposed only on explicit
request; Tier C contains 11 engineering-only operations. The 61 uncertified
entries retain visible independent-golden debt and are never described as
certified. There are no known correctness defects. See
`reports/nexusai-apf3-final-source-closure-20260818.md`.

## 2026-08-28 NX-A1 extension

NX-A1 preserves APF-3 as the execution authority and completes its missing
shared contract surface. `QueryPlanV1`, `PlanStepV1`, `OperationContractV1`,
`ExecutionBudgetV1`, `EntityReferenceV1`, and `AnswerEnvelopeV1` are aliases of
the accepted APF-3/platform types, not replacements. New additive contracts
cover bounded Investigation Context, evidence/time/location scope, capability
readiness snapshots, typed plan-validation results, shared Tool Invocation/Tool
Result, Observation Packets, and Clarification Requests.

The shared validator now runs after the existing direct or bounded-composition
plan builder and before execution. It rechecks authenticated scope, permission,
capability readiness, dependencies/cycles, budget and the three allowlisted
implementation keys. The typed case-query adapter recursively rejects raw SQL,
commands, arbitrary executables/URLs and implementation-key overrides. Direct
deterministic execution remains model-free. See
`docs/design/nexusai-nxa1-shared-query-agent-tool-execution-foundation.md`.
