# NexusAI Phase 7.2 Agent Chat acceptance

Date: 2026-08-05  
Status: accepted after final guarded image refresh and health verification

## Outcome

NexusAI now has one retained, fully populated demonstration collection:
`nexusai-forensic-demo`. Agent Chat accepts ordinary natural-language questions;
users do not need to remember operation templates, limits, model names, or API
syntax. Recognized deterministic questions bypass the LLM, while explicit
`Explain` and `Interpret` requests run bounded Qwen synthesis over the returned
facts.

## Retained collection

| Measure | Accepted value |
|---|---:|
| Evidence items | 8 |
| Completed ingestion jobs | 8 |
| Total input rows | 9,282 |
| Accepted rows | 9,272 |
| Duplicate rows | 3 |
| Rejected rows | 7 |
| Canonical records | 9,272 |
| Record/entity links | 44,318 |
| Failed or in-flight jobs | 0 |

The eight evidence sources cover CDR, IPDR/network, ANPR, subscriber identity,
tower/site reference, and the structured/cross-family support required by the
six agents. The cleanup retained only this Knowledge Base collection and only
this forensic database collection. Thirty old Knowledge Base collections and
all non-demo forensic rows were removed under the user's explicit cleanup
authorization. Named Docker volumes and rollback images were preserved, but the
deleted logical collections are not recoverable through the UI.

Cleanup evidence:
`reports/runtime-activation-20260804/phase7.2-demo-collection-consolidation.json`.

## Natural-language query acceptance

The complete 65-operation matrix was sent without `template=...` in any user
question. Agent Chat/records routing inferred the intended operation and bounded
limit internally.

| Result | Value |
|---|---:|
| Queries | 65 |
| Answered with rows | 64 |
| Accepted no-results | 1 |
| Route mismatches | 0 |
| Incomplete/stuck requests | 0 |
| Request failures | 0 |
| Records-API route-check p95 | 257 ms |
| Full Agent Chat/SSE p95 | 9,815 ms |
| Full Agent Chat/SSE maximum | 22,916 ms |

The accepted no-results outcome is evidence absence, not a routing or execution
failure. The complete copy-ready list is in
`reports/nexusai-agent-query-catalog-phase7.1-20260804.md`.

## Model-assisted Agent Chat acceptance

The real `/api/agents/{agent}/chat` plus request-correlated SSE completion path
was exercised for all six configured agents using
`qwen_qwen3-4b-instruct-2507`. Exact SQL findings are rendered independently of
model prose. Unsafe synthesis is rejected and replaced by an explicit,
complete deterministic fallback.

| Agent | Result | Latency |
|---|---|---:|
| Forensic Records Analyst | compliant model interpretation | 42,716 ms |
| Communications CDR Analyst | compliant model interpretation | 38,219 ms |
| Network IPDR Capture Analyst | policy-rejected synthesis; deterministic fallback | 47,489 ms |
| Vehicle ANPR Geospatial Analyst | policy-rejected synthesis; deterministic fallback | 47,308 ms |
| Subscriber Identity Analyst | compliant model interpretation | 43,449 ms |
| Tower Location Reference Analyst | policy-rejected synthesis; deterministic fallback | 49,474 ms |

All six responses contained authoritative `Deterministic Findings` and the
correct inferred template. Three also contained policy-compliant `Model
Interpretation`; three deliberately failed closed because the draft prose
crossed a family-specific inference guard. This is accepted behavior: no unsafe
model statement is displayed and the deterministic answer remains complete.

The final CPU-only model run completed in 38.2-49.5 seconds per agent. The smoke
harness now correlates every status, error, and completion event to the exact
POST acknowledgement `message_id`; it never accepts an unrelated replayed SSE
event. The server applies a 210-second upper bound and the UI clears `Working...`
only for the matching request.

## Routing and stall prevention

- All 65 accepted operations have natural-language mappings.
- A recognized forensic question takes the direct deterministic route.
- Summary wording no longer silently invokes a model for deterministic anomaly
  or cross-dataset summaries.
- Only explicit interpretation wording requests model synthesis.
- Analytical execution has a 30-second bounded timeout to accommodate a cold
  cross-family query without creating an unbounded working state.
- Native agent execution has a 210-second upper bound and emits request IDs on
  status, stream, error, and final events.
- Unknown natural forensic wording stays inside the governed hybrid forensic
  planner; it does not fall through to the legacy autonomous records-tool loop.
- Legacy generic records tools are disabled for forensic specialist agents, so
  a stale/default collection cannot leak into a case-scoped answer.
- The word `briefly` is no longer misread as a request for a forensic `brief`.
- Explicit templates remain optional power-user/API diagnostics, not a UI
  requirement.

## Verification artifacts

- Natural matrix: `scripts/smoke_forensic_full_demo_query_matrix.ps1`
- Natural matrix result: `reports/runtime-activation-20260805/phase7.2-natural-query-matrix.json`
- Full Agent Chat matrix result: `reports/runtime-activation-20260805/phase7.2-agent-chat-full-matrix.json`
- Six-agent LLM matrix: `scripts/smoke_forensic_agent_llm_matrix.ps1`
- Six-agent LLM result: `reports/runtime-activation-20260805/phase7.2-agent-chat-llm-matrix.json`
- Demo seeding: `scripts/seed_forensic_full_demo.ps1`
- Consolidation: `scripts/consolidate_forensic_demo_collection.ps1`
- Full guarded rebuild: `scripts/build_deploy_forensic_phase6_gate.ps1`

The final source tests include natural coverage for all 65 operations and the
`briefly` regression. The guarded rebuild retains the 6 GiB free-memory gate,
rollback images, and named volumes; no safety threshold was lowered.

The final activation passed with API/worker/LocalAI build times of 3.0/3.9/33.5
seconds, 6.59 GiB free at the gate, and one LocalAI attempt. The deployed
LocalAI/UI image is
`sha256:fcb3355210d78235c677fc3b6f86fa13122c4271c54518cef3beace2b47ecf29`;
the final forensic API image is
`sha256:3a6c39db71ca6a47e30538a01e9a865fbdbd5ef0130edd4e5420f4ab190976ec`.
BuildKit Go module and compile caches now persist across bounded retries so a
late external-proxy EOF cannot force every module to be downloaded again.

Final browser acceptance showed the demo case selected and the exact screenshot
regression question `How are the CDR events divided by call type?` completing
with `call_type_breakdown`, collection `nexusai-forensic-demo`, nine returned
rows, and no remaining `Working...` indicator. A final six-query Agent Chat/SSE
regression covering overview, CDR breakdown, correlation, timeline, evidence,
and executive brief passed after the final API image refresh.

## Guardrails

Answers are evidence observations and deterministic calculations, not findings
of ownership, identity, guilt, intent, communication content, RF coverage,
device presence, continuous route, driver identity, or association. Raw CNIC
and subscriber-name lookup remain prohibited. Model prose is subordinate to the
returned rows, counts, provenance, and limitations.
