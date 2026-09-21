# NX-B2.1D — residual-only 8-case development freeze

The fresh gate is **FROZEN_NOT_EXECUTED**. This is a development preparation/source-validation result, not live acceptance or qualification. The old 32-case corpus remains retired after dispatch and its lock and all attempt-1 evidence are unchanged.

## 1. Repository

Branch `codex/forensic-hybrid-checkpoint-20260723`; HEAD `40717b83510c08db25dc26b9d6674bf46db363ac`. Worktree remains modified/untracked; existing user work preserved. Full status: `tmp/decision8-status-final.txt`. No stage/commit/push/reset/clean/PR, service recreation, model download or retained evidence mutation.

## 2. Current residual contract

`forensics.hybrid-decision/v2` has exactly one model-owned wire field: `decision`. Its enum contains server-issued tuple IDs and `CLARIFY:AMBIGUOUS_INTENT` / `CLARIFY:INSUFFICIENT_FACTS`. The server derives tuple selection, resolved/clarification state and audit projection. The model cannot supply confidence, facts, identifiers, literals, scope, authorization, representation, dates, top-k or execution parameters. No five-field model-owned state has been reintroduced.

The evaluator calls the actual `resolveHybridPlanner` implementation through a byte-capturing HTTP proxy to the configured LocalAI endpoint. It does not force candidate lists or decisions. Source admission verifies every handwritten expected outcome against current tuple generation, decoder and final assembly, without HTTP/model inference. Actual per-case schemas are frozen in `schema-bindings-v1.json` and checked again immediately before live dispatch. The admission binding file contains synthetic admission facts; live facts come directly from the actual request and are independently snapshotted before model dispatch and compared with the resulting audit.

## 3. Files changed in this preparation

New Go evaluator/source-admission test: `api/forensic_records/nxb21d_decision8_development_ginkgo_test.go`.

New bundle files:

- `scripts/nxb21d-hybrid-decision-development/acceptance-v1.json`
- `scripts/nxb21d-hybrid-decision-development/decision-8case-corpus-v1.json`
- `scripts/nxb21d-hybrid-decision-development/decision-contract-v2.json`
- `scripts/nxb21d-hybrid-decision-development/decision8_config.json`
- `scripts/nxb21d-hybrid-decision-development/decision8_config.json.sha256`
- `scripts/nxb21d-hybrid-decision-development/run_nxb21d_q4_decision_8case_development.ps1`
- `scripts/nxb21d-hybrid-decision-development/runtime-baseline.json`
- `scripts/nxb21d-hybrid-decision-development/schema-bindings-v1.json`
- `scripts/nxb21d-hybrid-decision-development/test_decision8_framework.ps1`
- `scripts/nxb21d-hybrid-decision-development/validate_decision8_corpus.py`

New reports: this handoff; `d-decision8-development-freeze-v1.json` and its sidecar; `d-decision8-freshness-v1.json`; `d-decision8-source-validation-v1.json`. Updated existing continuation/master directive, phase ledger, next-generation/current roadmaps and maturity matrix/issue register. Temporary preparation scripts and logs remain under `tmp/`.

Production planner, fact extraction, tuple construction, assembly and prior incident fixes were reused unchanged. Shared receipt/RAM/transport helpers were reused unchanged. The new runner has an isolated output root and lock; it never deletes or bypasses the old lock.

## 4. Source verification

| Check | Result |
|---|---|
| Full api/forensic_records | PASS; 36.994 s (package line 36.988 s); 527 Ginkgo PASS, 0 FAIL, 31 SKIP. Source-only run; live gates skipped. |
| Focused hybrid/interface tests | PASS; 1.574 s; 42 Ginkgo PASS, 0 FAIL, 516 filtered SKIP plus selected Go interface tests. |
| LocalAI schema conversion / enum preservation | PASS in focused/full suites using actual LocalAI Item/Grammar conversion. |
| Duplicate, unknown, empty/null, extra fields and old five-field rejection | PASS in focused/full suites. |
| Eight-case source oracle admission | PASS; 0.602 s; all eight require residual calls, correct tuples/states reachable; no HTTP. |
| Receipt immutability | PASS; success/failure once-only seals, matching sidecars, repeated output stable, unchanged modification time/bytes, preserved lock. |
| PowerShell transport | PASS; 28 ms, 3282 bytes. |
| Consumed holdout / qualification / activation guards | PASS; 28 assertions. No qualification dispatched. |
| New runner syntax / source and identity tamper / ValidateOnly | PASS; no runtime/model calls on ValidateOnly. |
| Historical evidence | All 20 attempt-1 files/lock hashes and timestamps unchanged. |

The original draft image case was rejected by source admission because image.plate requires selected-evidence scope. Before freeze it was replaced with a supported workspace document-retrieval case; no production capability or scope rule was weakened. The draft was never dispatched. Existing DB/Testcontainers limitations are unaffected; no DB-dependent tests or DB setup were requested/run in this slice.

## 5–7. Fresh corpus and proof

Eight cases: two per language; one resolved and one clarification per language. All eight expect model calls. Four resolved capabilities span network, CDR, access logs and knowledge retrieval. Two clarification cases have undecided intent and two lack a required comparison counterpart.

| ID | Language | Expected decision | Why |
|---|---|---|---|
| dec8-en-resolved | en | `ipdr.endpoint/AGGREGATE/authorized_workspace` | Aggregate network usage, not session listing. |
| dec8-en-clarify | en | `CLARIFY:AMBIGUOUS_INTENT` | Two explicit alternative outputs without a selected intent. |
| dec8-ur-resolved | ur | `cdr.time_activity/AGGREGATE/authorized_workspace` | Time distribution of calls, not identity lookup. |
| dec8-ur-clarify | ur | `CLARIFY:INSUFFICIENT_FACTS` | Comparison intent is stated but second required entity is absent. |
| dec8-roman_ur-resolved | roman_ur | `logs.failures/FILTER/authorized_workspace` | Negated subscriber alternative must not displace affirmative login failure request. |
| dec8-roman_ur-clarify | roman_ur | `CLARIFY:AMBIGUOUS_INTENT` | Contact ranking versus temporal aggregation left undecided. |
| dec8-mixed-resolved | mixed | `knowledge.semantic/SEMANTIC_RETRIEVAL/authorized_workspace` | Semantic retrieval of documentary statements about the identifier, not network aggregation. |
| dec8-mixed-clarify | mixed | `CLARIFY:INSUFFICIENT_FACTS` | Missing peer for requested comparison; unrelated single-endpoint summary is not an answer. |

Corpus SHA256: `7bf616e0c146128dcf4fd9e4d470512ae49f48d3e8feb7497f1ba1ccdda9fbe3`.

The UTF-8/no-BOM corpus has unique IDs, questions and eight relevant values. Freshness proof: `reports/nxb21/d-decision8-freshness-v1.json`. It covers 590 historical JSON/JSONL/text files, including 57 probe/smoke files, both consumed holdouts, the retired 32-case corpus and attempt-1 evidence. Zero question overlap, zero value overlap; no oversized/non-UTF8 exclusions occurred. The proof lists every inspected file hash. Normalization uses Unicode NFC, case folding and whitespace normalization, with substring checks against saved historical prompt strings. This is a bounded repository comparison, not a claim about unknown external material.

## 8–10. New development freeze

Freeze ID: `nxb21d-q4-hybrid-decision-dev-r1`.

Freeze identity SHA256: `2467e8a85b0517fcfd5f7580825ddc55611db20ba4bdec073692c6d6facea4e3`.

The immutable freeze receipt and sidecar are `reports/nxb21/d-decision8-development-freeze-v1.json` and `.json.sha256`. Full manifest: 238 paths/hashes. Identity algorithm is SHA256 of LF-terminated UTF-8 key=value lines: freeze ID, artifact SHA, Q4 profile SHA, Q8 profile SHA, runtime SHA, completion budget 512, followed by all framework paths/hashes in ordinal order. The operator config has a separate sidecar. The historical freeze/config were not rewritten.

| Frozen component | SHA256 |
|---|---|
| Q4 artifact | `2fde00ce69dd4899c70d020845e2638353015bba0fdf161b3eb965f2bca4464e` |
| Q4 profile | `30d3bcd8c92684e191277dad992354323b7e46c7d30d3c69149010e90f117129` |
| Q8 profile | `ef036e0a02cc5e8331dc4dff41c0e0cb68e6e0bda71d92b92f47f673472e666f` |
| query_hybrid_planner.go / fact extraction / tuple builder / final assembly: `api/forensic_records/query_hybrid_planner.go` | `c327ea8b2e63ef18b7bcbee923ce9253ed1ff4ae247902cfaadfa9156f0be459` |
| query.go: `api/forensic_records/query.go` | `9108703f7c6979fc6b50e05c6fb9a5c2b43fc854816f9640c04783fd4502580a` |
| query_language_assistance.go: `api/forensic_records/query_language_assistance.go` | `f2d960f68867576d17b667242a59c27e1d0bd70a1fc090e365fc9c60c7825760` |
| one-field contract descriptor: `scripts/nxb21d-hybrid-decision-development/decision-contract-v2.json` | `77cf966bb4011a638fdc547669bc535f6f6f4595053907e79b15e31c60919c80` |
| actual per-case decision schemas and tuple bindings: `scripts/nxb21d-hybrid-decision-development/schema-bindings-v1.json` | `fbb6d773d855ce581ebcb48a8e8d0c97553e61f2bde3dabb4e985257b2ca7116` |
| capability references: `api/forensic_records/contracts/query-capability-references-v1.json` | `085982f58e9f3560f9f7291db45e520c4008ed9c492005e8e9f542cd9aa35ca0` |
| query contracts: `api/forensic_records/query_intelligence_contracts.go` | `56ea4c09d1b4d55e8f42b86b27a703aaaf646be3eeb0358d035f13c3d96cb726` |
| byte-safe PowerShell transport: `scripts/nxb21d-development/byte_safe_transport.ps1` | `9deed398e86e0347afc33964c47b8f4e489cc88372a77d16b4ba9ef8ed4a8c64` |
| HTTP client: `pkg/httpclient/client.go` | `967db790647005d0e5f7015ed101bed7cf286aa4775500ef940af0483c1e1673` |
| immutable receipt helper: `scripts/nxb21d-hybrid-development/immutable_receipt.ps1` | `143d28f7b419da73a032aaa754d4279d7245a69eef3e0949d238fc9049d9b067` |
| RAM helper: `scripts/nxb21d-hybrid-development/check_hybrid_ram.ps1` | `cb11349e966ce43155707d13d2b448a998e64a71201a4624fc85a704ce63afc6` |
| corpus: `scripts/nxb21d-hybrid-decision-development/decision-8case-corpus-v1.json` | `7bf616e0c146128dcf4fd9e4d470512ae49f48d3e8feb7497f1ba1ccdda9fbe3` |
| evaluator: `api/forensic_records/nxb21d_decision8_development_ginkgo_test.go` | `6923cac1776cba2aa32a7111228bb11934d73ac7e6f9b76652838d6b6e7ce303` |
| runner: `scripts/nxb21d-hybrid-decision-development/run_nxb21d_q4_decision_8case_development.ps1` | `b33e0dd302210e677d7747f37025393b17b2461e429346b33646dfef8b001123` |
| acceptance criteria: `scripts/nxb21d-hybrid-decision-development/acceptance-v1.json` | `d6d5091a83af17bbb4a47a09ac788589c022daf5928c2d2a90257acf29ba1d66` |
| Runtime baseline | `756f0813c35e8c8d70e0d8c4e061d783ccd772ba859400b12b72693040540961` |
| Operator config | `8a5efb387daa3e858a22769c18dcd5742e96113a11e932de9f4eba64656b19c8` |

Shared-module hashes bind all listed responsibilities in those modules; they are not invented separate function hashes. The manifest also freezes LocalAI grammar conversion source, all forensic API contracts/tests, module dependencies, and the original attempt-1 evidence/lock. Runtime observed read-only: five healthy containers, identities/images/restarts unchanged, retained tuple `64|64|77|22507|828|61`, Activity 314, active jobs zero, Q4 unloaded. Artifact and profile hashes match the existing approved candidate; model/profile unchanged.

## 11. Frozen acceptance

All eight expected model calls must return HTTP success, strict UTF-8, a valid complete envelope and valid one-field decision JSON. Zero extra fields, unknown decisions, malformed/truncated replies or finish_reason=length. Four correct resolved choices and four correct clarification choices. Facts/candidate snapshots unchanged; no target/scope/authorization/representation/parameter override. Eight correct final plans or clarification states with all three audit layers. Runtime integrity PASS. Final receipt sealed once after cleanup/postchecks; matching sidecar and no post-hash rewrite. These are mandatory acceptance conditions, not outcomes already achieved.

The evaluator saves request bytes, raw response bytes, strict UTF-8 response, model decision validation, full three-layer audit, candidate tuples, final projection, envelope finish_reason/token usage, latency, per-call RAM prechecks and aggregate resource samples. Raw decision correctness and final outcome correctness are both required for gate PASS. The runner refuses preloaded Q4 so cleanup only targets its own candidate load, checks RAM at 6 GiB before first inference / 4 GiB after load, and verifies container IDs/restarts, retained counts, Activity and active jobs before sealing. Infrastructure failures stop safely and seal FAIL. The original consumed lock is itself frozen evidence; deleting it fails source validation.

## 12–14. Exact standalone handoff

Source/framework validation only (safe to rerun; does not invoke model inference):

```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI
Set-ExecutionPolicy -Scope Process Bypass -Force
.\scripts\nxb21d-hybrid-decision-development\run_nxb21d_q4_decision_8case_development.ps1 -ValidateOnly
```

Standalone operator execution command, prepared but **not executed in Codex**:

```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI
Set-ExecutionPolicy -Scope Process Bypass -Force
.\scripts\nxb21d-hybrid-decision-development\run_nxb21d_q4_decision_8case_development.ps1
```

Validation markers: `CORPUS_FROZEN=PASS cases=8`, `DECISION8_FRAMEWORK=PASS live_inference=false`.

Expected standalone markers: `Q4_REGISTRATION=PASS`, RAM_GATE, `NXB21D_DECISION8=START`, `DEVELOPMENT_FREEZE=PASS`, `NXB21D_DECISION8_CASE`, `NXB21D_DECISION8_RESULT`, `RUNTIME_INTEGRITY=PASS`, then the single final `DEVELOPMENT_RECEIPT` / `DEVELOPMENT_RECEIPT_SHA256`, and `NXB21D_DECISION8=PASS` only if all gates pass. Failures print FAIL/DEVELOPMENT_FAILURE and preserve the one-shot lock. Do not repeat a dispatched run.

Future receipt path (does not exist yet):
`local-acceptance-models/nxb21-d/hybrid-decision-development-runs/run-<UTC>/decision8-development-summary-v1.json` plus `.sha256`.

The evaluator's `decision8-development-intermediate-v1.json` is explicitly private/intermediate and may update; it is not the final receipt. One-shot lock: `local-acceptance-models/nxb21-d/hybrid-decision-development-runs/development-dispatched.lock`, created atomically only at standalone dispatch. The old gate's separate lock remains untouched.

## 15–19. Boundaries and program state

No live inference occurred during this preparation. No eight-case gate dispatch, old corpus rerun/resume, new qualification holdout, qualification, activation, model change or service recreation occurred. Historical consumed holdouts are preserved; no replacement qualification holdout exists for this candidate. Live one-field decision performance is still unmeasured. Development success, if later achieved, does not itself authorize qualification or activation.

HYBRID_ATTEMPT_1=INCOMPLETE_FAILURE_RETIRED
RESIDUAL_V1_CONTRACT=RETIRED

NX-B2.1D
Q4_FULL_PLAN_ROLE=RETIRED
Q4_HYBRID_ROLE=HYBRID_PROMISING
RESIDUAL_V2_ONE_FIELD_DECISION=SOURCE_VALIDATED
RESIDUAL_ONLY_8CASE_GATE=FROZEN_NOT_EXECUTED
FINAL_REPLACEMENT_QUALIFICATION_CANDIDATE=NOT_YET
REPLACEMENT_HOLDOUT=NOT_CREATED
D_STATUS=OPEN
ACTIVATION=BLOCKED
