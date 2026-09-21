# NexusAI Structured and Textual Intelligence Maturity

Current execution overlay (2026-09-03): historical STIM acceptance below remains
closed. NX-B2.1A has reconciled current source/runtime breadth; B2.1B shared
derived-text semantics is next under the existing architecture. The complete
current plan, family inventory, semantic contract design and independent
artifact/DB defect evidence are in
`reports/nxb21/nexusai-nxb21-all-family-query-productization-reconciliation-v1-20260903.md`.
Canonical issues remain in `configuration/nexusai_stim_maturity_matrix.json`;
this does not create a parallel STIM phase or certification ledger.

Status: STIM program closed; final source/runtime acceptance passed  
Updated: 2026-08-20  
Authority: `NEXUSAI_MASTER_DIRECTIVE.md` and the approved family-adapter-agent API architecture

## Verified starting state

APF-3 is closed and remains frozen at its accepted source/runtime boundary:
18/18 browser cases passed, with zero open P0/P1 acceptance defects. The
2026-08-19 STIM read-only check returned HTTP 200 from LocalAI `/readyz`, the
forensic service `/healthz`, and `/app`. The installed model set remains
`qwen_qwen3-4b-instruct-2507` plus `qwen3-embedding-0.6b`. STIM source work does
not authorize a rebuild, deployment, retained-data mutation, model download, or
profile change.

The worktree was already materially dirty before this slice. Existing modified
and untracked files are user-owned and must be preserved. STIM changes are
bounded to the files named in the continuation checkpoint and version-control
diff.

## Governance and phase boundaries

Repository-scoped STIM work must load
`.agents/skills/nexusai-structured-intelligence-maturity/SKILL.md`. The skill
defines evidence precedence, family semantics, Pakistan-specific cautions,
normalization and dynamic-attribute contracts, independent-oracle rules,
correlation strength, query quality, grounded synthesis, UI presentation, and
phase gates. It activates only for material STIM work.

| Phase | Boundary | Current status |
| --- | --- | --- |
| STIM-0 | source audit, matrix, issues, benchmark corpus, governance | source baseline complete |
| STIM-1 | multi-schema intake, profiling, mapping, time policy | source accepted |
| STIM-2 | canonical and dynamic attribute survival | source accepted |
| STIM-3 | analytical truth and operation certification | closed; source and runtime accepted |
| STIM-4 | multi-source, multi-CDR, and cross-family correlation | closed; source and runtime accepted |
| STIM-5 | natural-language semantics and grounded Fact Packets | closed; source and runtime accepted |
| STIM-6 | analyst presentation and proof UX | closed; source and runtime accepted |
| STIM-7 | performance, security, deployment, full acceptance | closed; source and runtime accepted |

P0 security or integrity defects stop the active phase. A bounded P1 that can
invalidate analytical truth is fixed or explicitly gated in the active phase.
P2/P3 findings are recorded in the machine-readable matrix and remain owned by
their target phase. Accepted APF and R-phase boundaries are not reopened for
STIM polish.

## Domain glossary and family authority

- CDR: communications event records. Observed SIM/device/contact relationships
  are not ownership, identity, guilt, intent, or continuous presence.
- IPDR: network session and traffic metadata. An IP address is a time-bounded
  observed network attribute, not a person.
- Subscriber/identity: supplied registration and account assertions with
  source and validity context; a record does not prove the current user.
- Tower/location: site/sector reference data and time-valid observations.
  Coverage is not exact device location.
- ANPR: vehicle/plate observations with OCR/detection uncertainty; an
  observation is not ownership or driver identity.
- Financial transactions: exact source amounts and parties, with no motive or
  criminality inference.
- Access/security logs: system observations whose account, device, IP, and
  event relationships remain source-scoped.
- Generic tabular: schema-preserving intake where family semantics are not yet
  authoritative.
- INPR/INPRS: `NeedsDomainDefinition`. Repository search found no authoritative
  expansion, semantic contract, schema, fixture, or configuration. NexusAI must
  not invent or advertise these meanings.

Pakistan behavior is governed by
`configuration/forensic_country_profiles/pakistan.json`. It provides locale,
phone/MSISDN/operator and identifier cautions, but Pakistan context alone does
not prove a timestamp's timezone, a slash-date order, an operator, ownership,
identity, or location.

## Source-to-answer architecture baseline

The accepted path is:

1. The workspace-scoped upload API registers content-addressed evidence and a
   spool object, then publishes a transactional ingest job.
2. The records worker profiles the container, resolves a family adapter,
   normalizes records, preserves source rows, and stages output.
3. Legacy family tables and the canonical `forensic.records` projection receive
   bounded normalized records, metadata, quality, evidence, and provenance.
4. Capability projection exposes only governed operations supported by the
   authorized retained sources.
5. Query understanding resolves language, intent, operation, parameters, scope,
   and risk tier before deterministic execution.
6. The enterprise response carries facts, aggregates, source/citation lineage,
   limitations, and presentation hints to the Analyst UI.

The container surface currently covers CSV, TSV, JSON, JSONL/NDJSON, Parquet,
and XLSX; the tower manifest also declares GeoJSON. Validated fixtures are
uneven: CDR now has several schemas; tower has two; IPDR, subscriber, ANPR,
financial, and access/security each have one principal synthetic layout.
Generic structured tests cover the container formats but do not establish
family-level semantic equivalence.

Complete source rows survive in `raw_record`. STIM-2 adds a single
`forensics.schema-registry/v1`, family mapping profiles, per-record
`forensics.dynamic-attribute/v1`, `forensics.structured-quality/v1`, and
source/canonical time provenance through the existing JSON record and evidence
metadata. It creates no parallel raw store or database migration. Protected
subscriber name/CNIC attribute values remain available only in the governed raw
source path and are suppressed from presentation metadata.

## Correctness and analytical baseline

The accepted platform catalog contains six adapter descriptors—five dedicated
family adapters plus the generic compatibility adapter—50 historical
family-operation descriptors, ten roles, and sixteen agents. The derived
executable operation ledger now has 66 entries: five are independently
certified/queryable, 50 are explicitly limited, and 11 are engineering-only.
One certified operation is KB-only; structured scope is therefore 65 operations
with four certified/queryable, 50 limited, and 11 engineering-only. No
ordinary-user operation is silently uncertified. Uncertified does not mean
incorrect; it means independent result, citation, and presentation truth has
not yet been proven and the operation cannot be promoted beyond its explicit
gate.

The two STIM-1 time P1s are closed. `forensics.time-policy/v1` distinguishes
source-declared, analyst-confirmed, profile-defaulted, unknown, and unresolved
states. Ambiguous slash dates fail safely without a governed DMY/MDY decision;
naive timestamps fail when timezone is unknown; explicit offsets need no
assumption; and Pakistan jurisdiction alone never establishes time meaning.
Independent goldens cover unambiguous DMY/MDY, both governed ambiguous orders,
explicit offsets, unknown timezone, opt-in profile default, and cross-midnight
UTC conversion.

No P0 or foundational P1 remains. The full issue evidence, analyst impact,
decision, owner, and acceptance condition are in
`configuration/nexusai_stim_maturity_matrix.json`.

## Query and language baseline

The regenerated query ledger has 172 accepted occurrences, including 16 new
STIM-5 language variants. Deterministic normalization covers realistic and
messy English, Roman Urdu, Urdu and mixed-script input while preserving exact
identifiers. Scoped follow-up context may inherit only authorized targets,
operations, time/direction fields and exact source sets; expired or cross-scope
context is rejected. Ambiguous and unsupported requests remain deterministic
clarification/capability responses and cannot silently invoke SQL, KB or model
execution.

## Grounded narrative baseline and target

STIM-5 now emits deterministic `forensics.fact-packet/v1` before any optional
narrative. Facts, relationships and citations use request-local F/R/C IDs;
rows, facts and citations are bounded to 20/24/50, and proof roles distinguish
representative evidence from aggregate lineage. `forensics.narrative/v1` is a
strict JSON-schema contract. Its validator requires existing references, exact
fact text/identifiers/numbers, source-approved limitations and follow-ups, and
rejects unknown fields, altered values, widened relationships and forbidden
certainty. Invalid, unavailable or timed-out model output is discarded and a
deterministic narrative is delivered.

The reusable corpus is
`api/forensic_records/contracts/stim-answer-narrative-benchmark-v1.json`.
It covers exact results, rankings, Urdu, Roman Urdu, mixed-language
relationships, conflicts, no-results, and prompt injection. The corpus is
executed under the new contract. Direct installed-model probes produced valid
JSON but failed exact-number preservation in English, Roman Urdu and Urdu and
took about 21--45 seconds. Therefore the installed Qwen model is retained only
as an optional challenger behind the strict eight-second validator/fallback
boundary; its raw prose is not accepted as factual output and no download is
authorized.

## Analyst presentation baseline

The Analyst Data experience provides source status, usable-row/readiness
summary, filters, source details, five-column field mapping, quality and
source-specific capability readiness. Missing metadata is shown as not reported
instead of inferred. Ask presents direct answers, findings, metrics, explicit
result/fallback states, fact versus inference, relationships, bounded tables,
citations, limitations and how-the-result-was-determined. The old six/seven
column information-loss path is removed; responsive progressive controls keep
governed columns accessible. History restores the same presentation and lets an
analyst continue in Ask without executing. English, Roman Urdu, Urdu and mixed
direction metadata flow through presentation; Urdu is RTL and exact identifiers
remain isolated LTR with BiDi markup.

## STIM-1 and STIM-2 accepted structured foundation

CDR adapter contract 1.4 adds `forensics.schema-profile/v1` without changing the
canonical record or database schema. For each source column the profile records
the original/display/normalized name, candidate and selected canonical fields,
mapping state, inferred type, sampled non-empty/null counts, sample patterns,
and review need. Values are not copied into the profile. Required-group
coverage, unmapped/ambiguous fields, mapping version, and raw-preservation
policy flow through existing classification metadata and evidence lineage.

Three independent synthetic CDR layouts encode the same two analytical
events using distinct headers, delimiters, timestamp representations, and
provider extensions. A hand-authored golden proves canonical equivalence,
mapping explainability, and raw extension preservation. The shared registry now
extends the same mapping/dynamic/quality/provenance infrastructure to IPDR,
subscriber/identity, tower/location, structured ANPR, generic tabular, and the
truthfully partial financial/access families. INPR/INPRS remain
`NeedsDomainDefinition`.

## STIM-3 analytical truth source gate

STIM-3 reconciles the generated registry, not a remembered catalog count. The
66-operation ledger has five independently certified/queryable operations, 50
explicitly limited operations, and 11 engineering-only operations. Excluding
the KB-only evidence operation gives a structured scope of 65: four certified,
50 limited, and 11 engineering-only. Every ordinary-user structured operation
is therefore certified or visibly limited; silently uncertified exposure is
zero. Engineering-only diagnostics remain non-queryable.

The independently certified structured set comprises CDR frequent contacts,
CDR temporal activity, the bounded multi-CDR comparison, and the hybrid
evidence-package summary. IPDR, subscriber, tower, structured ANPR, financial,
access/security, generic, and broad cross-family operations remain explicitly
limited unless an individual operation has its own independent result,
citation, presentation, scope, and security oracle. This is a truthful source
closure, not a blanket certification claim.

## STIM-4 bounded correlation architecture

`forensics.structured-source-set/v1` accepts two through eight exact source
files, rejects duplicate source/evidence/version identities and incomplete
version bindings, and preserves collection, tenant, source file, evidence ID,
and version ID boundaries. `forensics.multi-cdr-comparison/v1` executes as a
read-only query over selected retained CDR rows, capped at 5,000 observations.
It introduces no materialized relationship store and no database migration.

The multi-CDR result certifies common and source-unique contacts; incoming,
outgoing, reciprocal, and one-way directionality; exact frequency and duration;
shared IMEI, IMSI, and tower observations; exact normalized-instant overlaps;
cross-source duplicate candidates; explicit conflicts; typed citations; and
bounded comparison, graph, timeline, conflict, limitation, and provenance
presentation. Missing duration is excluded and counted as missing; an observed
zero remains an exact value. Pakistan phone normalization is deterministic,
while service/sentinel rows are not promoted to contacts.

The typed relationship primitive independently proves only the supported,
time-valid contracts: CDR↔subscriber, CDR↔tower, IPDR↔subscriber, CDR↔IPDR,
and exact-plate ANPR↔ANPR across sources. Identifier type and validity windows
must agree. These edges mean exact match or source-declared relationship—not
identity, ownership, guilt, intent, causation, or continuous presence. The
existing broad public cross-family operations remain limited; no new public
claim is made for unsupported ANPR↔telecom, financial, access/security, generic,
or undefined INPR/INPRS relationships.

## Acceptance and operations

The STIM-3/STIM-4 runtime correction adds a fail-closed membership preflight:
every selected CDR source must resolve inside the authorized tenant and
collection to the exact retained file and eligible family, and any supplied
evidence/version pair must bind to that source before analytical SQL starts.
The public failure is a stable, non-enumerating `invalid_source_membership`
400 response; internal tests distinguish the failure classes and prove zero
analytical executions after rejection.

Cross-family retrieval now performs one read-only, tenant/collection/source/
batch-scoped candidate query against the existing typed `record_entities`
index. A single non-CDR normalized-field expansion preserves identifiers that
the generic entity producer intentionally does not project. The query caps
target-record matches at 5,000 and fails closed on 5,001; coverage, related
entities, citations and limitations compile once in Go. Unresolved source time
remains null, the supported typed relationship validator remains authoritative
for entity type and validity intervals, and no relationship store or migration
is introduced.

The complete structured-ingestion suite passes 102 tests with two intentional
dependency skips. The complete forensic Go package and `go vet` pass. Nine
focused STIM-4 source specs plus five runtime-correction specs pass, including
membership non-execution, stable error semantics, both independent goldens,
anti-correlation, exact scoped SQL, presentation/provenance, and 1,000 pure
comparison compilations in 94.1471 ms. The 5,000-row cross-family compiler
measured 63.3869 ms on the source host. JSON contracts parse
successfully. The previously accepted 677-module UI build and focused intake
browser checks remain unchanged because this slice changes no UI source.

The guarded forensic API activation and approved normal-path acceptance ingest
have run. The retained synthetic pack now contains seven completed fixtures
with exact accounting: 21 total rows, 20 accepted rows, one duplicate row and
zero rejected rows. The first read-only closure probe exposed a P1 UUID/text
comparison defect in the corrected source-membership SQL boundary. The next
activation proved that repair: exact membership executed and a tampered version
failed with stable HTTP 400 `invalid_source_membership`. It also exposed two
narrower P1s. The multi-CDR SQL prefilter excluded `03...` and `0092...` rows
before the correct Pakistan phone canonicalizer, and cross-family optional
family filters compared the `forensic_record_type` enum to text. Both are now
activated: the seven-event oracle and fail-closed membership check pass. Closure
then exposed the last two bounded gaps: raw provider `VOICE`/`CALL` tokens
prevented the expected conflict grouping, and the cross-family candidate UNION
combined enum with varchar. Source now uses the accepted canonical CDR service
class and text-normalizes both UNION branches plus their final join.

STIM-3 through STIM-6 are closed at accepted source/runtime boundaries. The
guarded R8 UI/API activation preserved the worker, model inventory, profiles,
volumes and rollback images. STIM-5/STIM-6 then passed the 14-check live deck,
65-operation matrix, 11-case Analyst Portal deck, rich-answer case, Urdu 390px
case, and deployed `/analyst/data`, `/analyst/ask` and `/analyst/history`
inspection with no mobile horizontal overflow or browser warnings/errors.
STIM-7 then passed the complete preservation/source/runtime/product acceptance
matrix: 46/46 structured goldens, forensic Go/vet and presentation suites,
14/14 multilingual live checks, 65/65 operations (one accepted no-result),
11/11 independent retained-data oracles, the 677-module build, 13/13 controlled
browser cases, and deployed 390/820/1024/1440 inspection with zero overflow or
console errors. `DeploymentNeeded=NO`, `OpenP0=0`, `OpenP1=0`;
`StructuredIntelligenceMaturityProgram=CLOSED` and
`StructuredIntelligenceFinalAcceptance=PASS`.

## Post-STIM governed runtime query intelligence boundary

The immediate post-STIM priority is architecture reconciliation for Governed
Runtime Query Intelligence. It may propose typed plans that compose certified
operations and explicitly allowlisted analytical primitives under tenant,
collection, case, source, evidence, version, row-budget, time, citation, and
authorization constraints. It must preserve deterministic execution, stable
failure semantics, independent certification, bounded result materialization,
and complete plan/parameter/proof telemetry. It must not accept unconstrained
model-authored SQL, silently widen scope, bypass operation certification, or
treat a generated narrative as evidence. Design approval and an independent
threat/acceptance pack are required before implementation.

Documents/RAG remain evidence-retrieval context with their own extraction and
citation gates; they are not a substitute for structured computation. ANPR
image, general image, audio, and video intelligence remain outside this
post-STIM design handoff and do not resume implicitly.

## NX-A1 governed execution foundation — 2026-08-28

The later product-owner NX-A1 directive authorizes the bounded source/contract
implementation that the post-STIM handoff reserved. The implementation reuses
the accepted APF-3 operation catalogue, capability projection, direct and
bounded-composition executors, Fact Packet, enterprise response, agent
transport, citations and Activity. It adds a shared Investigation Context,
explicit evidence/calendar/media-time/location scope, readiness snapshot,
typed validation/budget result, Tool Invocation/Tool Result, Observation
Packet, and Clarification Request.

The source validator checks exact tenant/case/collection/subject scope,
permission, current capability readiness, dependencies/cycles, budgets and
governed implementation keys before execution. Typed public plans reject raw
SQL, commands, arbitrary executable/URL fields and implementation overrides.
No generalized runtime SQL, new model/tool autonomy, family-depth expansion,
database change, retained-state mutation or deployment is included. Full
regression and NX-A1 source closure remain the active gate before NX-B1.
