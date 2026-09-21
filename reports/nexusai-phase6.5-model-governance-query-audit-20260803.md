# NexusAI Phase 6.5 Model Governance and Query Audit

Date: 2026-08-03  
Scope: live runtime inspection, downloaded-model role audit, model-assisted sample
queries, raw-query planning, specialist-agent prompts, Records/Agent Chat UX,
module completion audit, and approved next-family roadmap.

## 1. Executive verdict

NexusAI has a strong deterministic forensic foundation and a genuinely useful
CDR/IPDR/ANPR analyst surface, but the application is not yet the complete
multi-family forensic platform described by the approved architecture.

Three definitions are used throughout this report:

- **Live-verified:** observed against the currently running containers.
- **Source-verified:** implemented and tested in the workspace, but not present
  in the running image until the operator performs the guarded rebuild.
- **Planned:** required by the approved architecture but not yet implemented or
  accepted. A catalog/manifest entry does not mean a runtime capability exists.

The current high-confidence production boundary is:

- deterministic exact analytics and cited provenance are authoritative;
- CDR, IPDR, and ANPR have specialist depth and passed their prior 23-operation
  live matrix;
- access logs, subscriber, tower, financial, and generic records can be present
  and queried through shared canonical/correlation primitives, but do not all
  yet have equivalent specialist-agent and operation depth;
- document/OCR, image, audio/STT, video, capture, database, and archive model
  execution is not accepted and must continue to abstain;
- a chat model may explain already-computed facts, but it must never create an
  exact fact, join, identity, location, or citation.

## 2. Downloaded model inventory and correct roles

Live `/v1/models` exposes exactly two downloaded models:

| Model | Approved role | Current use | Phase 6.5 decision |
| --- | --- | --- | --- |
| `qwen_qwen3-4b-instruct-2507` | bounded planner/explanation model | assigned to the orchestrator plus CDR, IPDR, and ANPR specialists | retain for optional explanation; never use as the fact engine |
| `qwen3-embedding-0.6b` | Knowledge Base embeddings/retrieval | loaded by LocalAI and available to the KB | retain for embeddings; block from answer-model selection |

No new model download is justified in this slice. OCR, ASR, reranking, vision,
video, and extraction candidates in the evidence model catalog remain benchmark
candidates, not installed/accepted runtime dependencies.

Live inspection found that `qwen3-embedding-0.6b` advertises `FLAG_CHAT` as well
as `FLAG_EMBEDDINGS`. Therefore capability flags alone are insufficient for a
forensic model picker. The Phase 6.5 source now combines capability checks with
a fail-closed role-name filter.

The live agent configurations also assigned the text-only Qwen chat model to
`multimodal_model`. Phase 6.5 bootstrap source clears that field. Image/video
evidence will remain unavailable until a separately benchmarked and approved
model is assigned to an explicit visual role.

## 3. Model-assisted sample-query evidence

The live Qwen chat model was exercised directly with deterministic fact packets
and evidence-family constraints. These are model-involvement tests, not claims
that the current deployed Records proxy contains the Phase 6.5 corrections.

### CDR explanation test

- Supplied facts: `CALL=3,200`, `SMS=1,200`, `GPRS=600`, total `5,000`, plus a
  supplied source locator.
- Model: `qwen_qwen3-4b-instruct-2507`.
- Observed: prompt 115 tokens, completion 77 tokens, wall time about 68.95 s.
- Correct behavior: repeated the supplied counts and abstained from content,
  identity, ownership, and location inference.
- Material defect: it normalized/reworded the supplied citation locator instead
  of preserving the exact locator string.

### IPDR explanation test

- Supplied facts: `TCP=1,400`, `UDP=900`, `ICMP=200`, total `2,500`, plus a
  supplied source locator.
- Model: `qwen_qwen3-4b-instruct-2507`.
- Observed: completion 80 tokens, wall time about 87.40 s.
- Correct behavior: repeated the supplied facts and did not infer payload,
  purpose, intent, ownership, or maliciousness.
- Material defect: it altered the citation representation.

### Integrated Records-proxy test

An explicit CDR explanation request through
`POST /api/records/forensic/query` failed after about 30.4 s because the LocalAI
proxy used a 30-second HTTP deadline while CPU synthesis could take 60 seconds
or more. This was an orchestration defect, not a model-quality conclusion.

### Conclusions from the tests

1. The installed Qwen chat model is suitable for bounded, optional explanation
   when it receives already-computed facts and family-specific prohibitions.
2. It is not suitable as the authority for citation objects. Source locators
   must remain deterministic response fields rendered separately by the UI.
3. CPU inference needs a larger explicit bound and visible progress/fallback.
4. Exact queries should remain deterministic-only by default; model latency is
   justified only when the analyst explicitly requests explanation.

## 4. Phase 6.5 source corrections

### Model-role enforcement

- Records Intelligence now lists only enabled, chat-compatible models and
  excludes embedding, reranking, OCR, speech, audio, vision, image, and video
  model names.
- The forensic sidecar rejects the same incompatible model roles even if an API
  caller bypasses the UI.
- The agent bootstrap clears `multimodal_model` for all four text-only records
  agents.
- UI copy now says “Model explanation on request”; it no longer claims model
  synthesis occurred on every answer.

### Timeout correctness

- sidecar default bounded synthesis: 120 seconds;
- LocalAI Records proxy query deadline: 135 seconds;
- agent forensic POST deadline: 135 seconds;
- ordinary discovery/health endpoints retain 30 seconds.

The outer deadline intentionally outlives the sidecar deadline so the analyst
receives either the explanation or the sidecar's deterministic fallback rather
than a generic proxy failure.

### Family-specific explanation prompts

The sidecar now selects a prompt policy from the deterministic template:

- **CDR:** no inferred identity/ownership, content, personal relationship, home,
  continuous movement, or physical device user;
- **IPDR:** no inferred IP/subscriber ownership, reverse DNS, geolocation,
  payload, purpose, intent, or maliciousness;
- **ANPR:** no inferred owner/driver/occupants, association, road route,
  continuous movement, or OCR correction;
- **cross-family/generic:** no inferred identity, ownership, causation, intent,
  or continuous location.

Every prompt instructs the model to label deterministic findings, model
interpretation, and limitations, and to preserve supplied locators verbatim.
The deterministic citation objects remain authoritative even if prose deviates.

### Raw-query fail-closed behavior

Previously, an unrecognized raw question silently selected
`frequent_contacts`. That could turn an ambiguous request into an unrelated CDR
calculation. Source now returns a typed clarification with example operations and
runs no SQL, KB retrieval, or model synthesis. Capability-unavailable requests
still take priority so a request such as “transcribe this recording” correctly
returns `capability_guard`, not generic clarification.

Direct Agent Chat routing was expanded for collection overview, schema profile,
CDR service usage, CDR device-identity changes, geospatial movement, and the
existing IPDR/ANPR workflows. Exact queries continue to omit a synthesis model;
an explanation model is attached only when the user explicitly asks to explain,
interpret, summarize, or brief.

## 5. Module-by-module audit

| Module | State | What is dependable now | What remains |
| --- | --- | --- | --- |
| Case/collection governance | live foundation; Phase 6.4 refinements source-only | case-bound tenant/collection scope, governed selector, retained/system classification | rebuild Phase 6.4/6.5 and re-accept all authorized collection switching |
| Evidence registry and lineage | live foundation; legacy-case inclusion source-only | evidence/version/hash/source accounting and immutable history | complete live Evidence-tab reconciliation and modality child-artifact lineage UX |
| Queue/jobs | live foundation | NATS-backed ingest jobs, retry/reprocess history, read-only job summaries | richer analyst job failure guidance and later modality workers |
| Structured ingestion | strong | CSV/JSON/JSONL/TSV/XLSX foundations, canonical records, row accounting, provenance | additive provider schema packs; ODS/deeper columnar support |
| CDR | specialist operational | 9 accepted analyst workflows plus canonical/cross-family operations | provider/tower-join drift goldens and larger real-world language regression set |
| IPDR | specialist operational | 7 accepted analyst workflows, NAT/IP/port/domain/protocol/session depth | provider DNS/NAT packs and capture-to-session derivation |
| ANPR | specialist operational | 7 accepted analyst workflows, citations, timing/distance limitations | authorized image OCR and camera-clock calibration as separate gates |
| Subscriber identity | adapter/queryable foundation | canonical identifier lookup and shared correlation primitives | dedicated operation pack, historical validity/conflict logic, specialist runtime agent |
| Tower/location | adapter/queryable foundation | canonical site/location fields and shared tower/location calculations | dedicated time-aware site lookup/join operation pack and specialist agent |
| Financial | adapter/queryable foundation | canonical transaction records and exact filtering/correlation primitives | dedicated counterparty/flow/amount operations, specialist agent, currency/rounding policy |
| Access/security logs | operational shared adapter | 1,000 indexed rows in governed case; canonical/time/cross-family primitives | dedicated failed-access/session/incident operations and specialist agent |
| Generic tabular | compatibility operational | canonical schema/data-quality/source-row queries | promoted mapping workflow and explicit generic analyst policy |
| Knowledge Base | live foundation | two downloaded-model roles, semantic retrieval and source previews | citation UX acceptance across all accessible collections; later reranker benchmark only if justified |
| Query planner | Phase 6.5 source-verified | 53 templates; typed plans; target/date extraction; ambiguity now fail-closed | measured natural-language regression corpus per family and explicit confidence calibration |
| Model explanation | Phase 6.5 source-verified | role guard, family prompt, bounded packet, deterministic fallback | rebuild/live latency and cancellation acceptance; do not let prose own citations |
| Reports | useful synchronous foundation | deterministic report generation and analyst brief rendering | immutable report records, versions, approval/export workflow |
| Relationships/timeline/map | typed foundation | deterministic relationship/timeline operations and guarded empty state | richer renderers only when typed results exist; no inferred edges/locations |
| Records UI | Phase 6.4/6.5 source-verified | analyst-first question flow, compact findings/grid/provenance/limitations | guarded rebuild, live task-based usability, accessibility, and collection-switch acceptance |
| Agent Chat | live deterministic foundation; fixes source-only | case-bound direct forensic route and four profiles | rebuild, verify all profiles, model explanation progress/fallback, and live specialist matrix |
| Auth/audit | strong foundation | feature authorization, tenant/collection headers, request IDs, audit contracts | full role/PII/export policies for new family slices |
| Documents/OCR/media | foundation/contract-only | safe registration and deterministic metadata where implemented | accepted parsers/models/workers; remain unavailable until each vertical slice passes |

## 6. Governed-case capability truth

Live capability summary for `records-demo-verified`:

- 20 registered evidence families;
- 8 reported queryable;
- 2 semantic-only;
- 9 no-data;
- 1 manual-review.

Indexed rows are presently concentrated in access/security (`1,000`), CDR
(`5,000`), IPDR (`2,500`), and ANPR (`750`), totaling the governed `9,250`
canonical rows. Subscriber, tower, financial, and generic families report
queryable adapter state but zero dedicated indexed rows in the capability
summary; their four registered/normalized evidence references must not be
misrepresented as a deep, populated specialist workload.

Public operation discovery currently contains 34 versioned operations:

- communications CDR: 11;
- network IPDR: 9;
- ANPR/vehicles: 9;
- generic tabular: 5.

This is why the next approved slice must deepen subscriber identity and
tower/location rather than merely adding more buttons to the existing page.

## 7. Raw real-world query coverage

The following questions represent supported or intentionally clarified analyst
language. Values such as `TARGET`, `ABC-123`, IPs, dates, and source files must be
replaced with exact authorized case identifiers.

### CDR

- Who are the frequent contacts for `TARGET` between two dates?
- Show incoming versus outgoing calls for `TARGET`.
- Show the call-type breakdown.
- How many GPRS, SMS, USSD, and voice events exist?
- Show IMEI and IMSI changes for `TARGET`.
- Show the shortest and longest completed call for `TARGET`.
- Show duration extremes and non-zero duration statistics.
- Show first-seen and last-seen activity for `TARGET`.
- Show daily, hourly, and nocturnal activity.
- Show top explicit cells/locations for `TARGET`.
- Show tower activity for an exact cell/site.
- Show cited source rows for `TARGET`.
- Show CDR rows where call type is GPRS and duration is over 60 seconds.
- Compare two exact numbers and show explicit shared entities.

### IPDR

- Show endpoint/NAT/port combinations.
- Show explicit domain observations and byte totals.
- Show protocol counts and byte/duration totals.
- Show hourly session and byte volume.
- Show sessions for an exact subscriber identifier.
- Show overlapping sessions for an exact subscriber.
- Build an IPDR timeline for an exact IP or subscriber.
- Show rows for a source/destination IP and bounded time range.
- Show high-byte sessions without interpreting content.
- Compare two explicit endpoints using exact shared identifiers.
- Correlate an exact IP across available record families.

### ANPR

- Where was plate `ABC-123` explicitly sighted?
- Show the chronological camera sequence for `ABC-123`.
- Show camera activity totals.
- Show same-camera observations within five minutes for `ABC-123`.
- Show consecutive-sighting elapsed time and straight-line distance.
- Show observed raw and normalized plate variants.
- Build the plate timeline for a bounded date range.
- Show exact sightings at one camera.
- Show sightings containing a supplied location value.
- Compare two exact plates using explicit co-observation evidence.
- Correlate a plate across available record families.

### Collection, evidence, and cross-family

- Which files were ingested?
- Show collection overview and row accounting.
- Show duplicate uploads.
- Show parser errors, duplicates, and rejected rows.
- Show detected headers and schema.
- Is this case ready for production review?
- What limitations and missing data exist?
- Show available exact entities.
- Show source rows for an exact entity.
- Build an entity timeline for an exact identifier.
- Show an exact relationship network without inferred edges.
- Correlate two exact identifiers across record families.
- Find cited case-note evidence.
- Generate an executive case brief.

An unrecognized request now pauses and asks for the evidence family and desired
calculation. A missing target triggers a target-specific clarification. A request
for an unavailable modality returns a capability guard. None defaults to a
plausible but unrelated calculation.

## 8. Verification evidence

- PowerShell bootstrap parser: PASS.
- Go formatting: PASS.
- forensic API suite: PASS; 188 specs registered, 186 passed and two intentionally skipped.
- focused deterministic agent-routing/formatter tests: PASS.
- targeted changed-page ESLint: zero errors; 19 pre-existing warnings in the two large pages.
- production React build: PASS; 658 modules transformed.
- broad repository UI lint: not green at baseline; six existing hook errors and
  590 warnings outside this Phase 6.5 change.
- LocalAI endpoint package timeout test: not claimed. The package twice exceeded
  a 180-second Windows compile/link cap without returning a test result.
- Docker rebuild/live Phase 6.5 acceptance: not run by Codex.

## 9. Required activation and focused acceptance

After restart, with Docker Desktop ready and at least 6 GiB free RAM:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File '.\scripts\build_deploy_forensic_phase6_gate.ps1'

powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File '.\scripts\bootstrap_forensic_specialists.ps1' `
  -CollectionId 'records-demo-verified' `
  -TenantId 'default' `
  -Model 'qwen_qwen3-4b-instruct-2507'
```

Require `Phase6Activation=PASS`, then verify:

1. all four agent configurations use the Qwen chat model and have an empty
   `multimodal_model`;
2. Records Intelligence lists Qwen chat as the only optional explanation model;
3. an unrecognized raw forensic question returns clarification and no SQL/KB/model route;
4. “transcribe this audio” returns `capability_guard`;
5. exact CDR/IPDR/ANPR queries complete without a model;
6. one explicit CDR explanation completes within 120 seconds or returns a
   deterministic fallback through the 135-second proxy;
7. citation objects remain exact even if the model prose is imperfect;
8. all accessible collections can be selected and empty/semantic-only cases
   show accurate capability states;
9. browser console, failed network requests, keyboard flow, and 390/820/1024/1440
   overflow checks remain clean.

## 10. Approved next phases

### Phase 7A - Subscriber identity and tower/site intelligence

This is the next approved vertical slice.

1. Add versioned subscriber and tower operation packs rather than generic-query
   aliases.
2. Implement validity-at-time, identifier alias, reuse/conflict, and explicit
   subscriber-to-device evidence without treating a current row as historical ownership.
3. Implement time-aware cell/site reference lookup, coordinate validation,
   provider alias mapping, and explicit tower joins with RF/coverage limitations.
4. Create separate Subscriber Identity and Tower/Location specialist profiles
   with role-compatible Qwen explanation prompts; deterministic tools remain authoritative.
5. Add typed query templates, real-world raw-language regression corpora,
   enterprise tables/timelines/maps, and exact source-row citations.
6. Add provider-drift, timezone, duplicate, missing-coordinate, conflicting-row,
   historical-reuse, and unauthorized-field fixtures.
7. Accept API, agent, UI, export, audit, performance, responsive, accessibility,
   and no-inference gates before calling either family operational.

### Phase 7B - Financial transactions

Add exact amount/currency normalization, counterparty and flow calculations,
time windows, duplicates/reversals, source-row provenance, financial specialist
policy, and no-fraud/no-ownership inference. No finance model is needed for exact
calculations.

### Phase 7C - Access/security events

Promote the existing 1,000 indexed rows into dedicated failed-access,
user/IP/path, session, sequence, and incident-timeline operations plus a
specialist agent. Treat “suspicious” as a rule result with disclosed criteria,
not a model accusation.

### Phase 7D - Generic tabular promotion

Add analyst-approved column mapping, schema drift review, saved mapping versions,
and generic exact query plans. Unknown columns stay explicit; the model must not
silently assign semantics.

### Later evidence modalities

Proceed in architecture order: documents/email and OCR, then audio/STT and
transcripts, then images/visual comparison, then video/timeline fusion, then
capture/database/archive depth. Each modality requires real-format goldens,
deterministic metadata, bounded extraction, lineage, model benchmarks, and
explicit unsupported-operation behavior before UI promotion.

## 11. Safety and state

No model was downloaded, no container was rebuilt, no collection/evidence/KB
entry/database row/model/volume was deleted or mutated, and no migration,
upload, reprocess, staging, commit, push, or publication occurred in this slice.
The currently running image remains the previously accepted Phase 6.3 runtime;
Phase 6.4 and Phase 6.5 corrections are source-only until the operator gate.

## 12. Guarded activation, live acceptance, and final defect closure

The operator subsequently completed the guarded activation successfully:

- `Phase6Activation=PASS`;
- API build `11.9 s`, worker build `3 s`, LocalAI build `440.1 s`;
- one LocalAI build attempt with `7.3 GiB` free before compilation;
- rollback images and named volumes preserved;
- all four profile version `1.2.0` agents activated on tenant `default`, case
  `records-demo-verified`, model `qwen_qwen3-4b-instruct-2507`.

Live health passed for LocalAI, the forensic API, worker metrics, and NATS. Live
configuration readback confirmed all four specialists use the Qwen chat model,
have no multimodal model, and retain KB plus deterministic forensic tools. The
unknown-intent probe returned clarification without SQL/KB/model execution and
the audio-transcription probe returned `capability_guard`.

Deterministic live queries passed for CDR call types (8 rows), IPDR protocols
(4 rows), ANPR camera activity (20 rows), and source inventory. An explicitly
requested Qwen explanation completed without fallback in `93.228 s`; its packet
retained the eight exact CDR counts and separated deterministic findings, model
interpretation, and limitations. The embedding model is absent from the UI's
optional explanation selector. The case selector exposes all 26 authorized
collections, and switching to a retained knowledge-only collection produced an
accurate capability-aware workspace rather than pretending structured rows or
specialists were available.

Browser acceptance passed for branding, case navigation, compact workflow
presets, result KPIs, deterministic visual analysis, exact row preview,
provenance/export controls, Agent Chat runtime assurance, and zero console
warnings/errors. The accepted CDR preset rendered eight exact call-type rows in
`23 ms`, one cited source, an exact-value chart, limitations, and next checks.
Agent Chat correctly routed “which files were ingested?” to
`source_file_audit`/`records_sql` without an LLM.

This live run also found three cardinality/planning defects that are now fixed
in source with regression coverage:

1. The React runtime-query detector incorrectly promoted ordinary analytical
   phrases such as “show CDR call type breakdown” to `canonical_records`.
   Canonical promotion is now limited to explicit structural predicates (for
   example `where`, comparison operators, batch/source fields, or raw payload
   filters); natural analytical language remains with the governed backend
   planner. A live request with no explicit template proves the planner selects
   `call_type_breakdown`, operation `cdr.call_type_breakdown`, and eight rows.
2. The Evidence catalog's ordinary one-to-many joins multiplied an evidence
   item when it had multiple jobs or KB assets. Both joins now use latest-row
   lateral subqueries, preserving one canonical catalog row per evidence ID.
3. `source_file_audit` previously reused collection overview and flattened jobs,
   KB assets, and family summaries into 16 apparent rows. It now reconciles the
   three registries by source filename and returns one row per source with
   registration systems, record types, states, row accounting, size, evidence
   IDs, and last-observed time. The new SQL was executed read-only against the
   live database and returned exactly six canonical sources: two knowledge
   documents and four structured files totaling 9,250 rows.

Post-fix verification:

- `go test ./api/forensic_records`: PASS;
- targeted React ESLint: zero errors (five pre-existing unused-symbol warnings
  in the large Records page);
- production Vite build: PASS, 658 modules;
- read-only live reconciliation SQL: PASS, six canonical source rows;
- 1280 px deployed layout: zero horizontal overflow and zero broken images;
- browser warnings/errors: zero.

The three final fixes affect the API and React image, so the currently deployed
containers do not yet contain them. Run the same guarded Phase 6 gate once more,
then recheck only: typed natural CDR routing, six-row source audit, six-row
Evidence registry, Agent Chat source audit, and 390/820/1024/1440 overflow. The
full model test, specialist bootstrap, and 23-operation matrix do not need to be
repeated unless their source changes.
