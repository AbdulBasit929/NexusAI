# NX-B2.1D hybrid source freeze handoff — 2026-09-06

Completed source validation and standalone DEVELOPMENT freeze. Live gate remains unexecuted. This is not a qualification result or a production deployment.

## 1. Repository reconciliation

Branch `codex/forensic-hybrid-checkpoint-20260723`; HEAD `40717b83510c08db25dc26b9d6674bf46db363ac`. Existing modified and untracked work preserved. No stage, commit, reset, clean, push, PR, service recreation, migration, model download, or retained-evidence mutation occurred. Full status snapshot: `tmp/hybrid-git-status-final.txt`; these pre-existing changes are not all attributable to this phase.

## 2–6. Implemented ownership and contracts

Architecture: deterministic facts + server-issued compatible tuples + residual model selection when needed + server assembly + authoritative validation. The network language-assistance entry point uses this hybrid path. Existing deterministic operation routing remains available.

Deterministic facts own tenant/user/collection authorization, selected/workspace scope, validated follow-up context and target replacement, exact raw source spans and canonical identifiers, quoted literals, text representation, explicit dates/direction/time bounds/top-k, explicit family descriptors, and refusal states. Context from another authorization tuple is rejected. IP addresses and dates are protected before permissive plate patterns, avoiding partial captures with neighbouring prose.

Tuple IDs bind capability/semantic/scope. Every tuple includes family, operation, representation rule, and required/optional fact types derived from the capability registry. Independent capability/semantic Cartesian products are not offered. A unique compatible tuple is deterministic; unresolved choices alone use Q4.

Q4 emits exactly five fields: `contract_version`, `selected_tuple_id`, `ambiguity_state`, `clarification_code`, `confidence`. Version is `forensics.hybrid-residual/v1`; tuple IDs are enumerated by the server. Unknown fields, unknown tuple IDs, invalid ambiguity, malformed/invalid UTF-8 responses and incomplete completion envelopes fail closed. Q4 cannot supply target, literal, scope, date, top-k, representation, authorization, or execution parameters. Budget stays 512 tokens, temperature 0.

Server assembly verifies tuple membership and authorization, builds the complete typed proposal from source facts, invokes production validation, and resolves the executable request. Audit separates `deterministic_facts`, `raw_model_residual`/`model_decision`, and `final_plan`/`resolved_target`; candidate tuples, decision source and terminal state remain visible. Refusal/ambiguity cases have a null executable plan, not invented execution fields. Raw tuple correctness is scored separately from final-plan correctness.

## 7. Prior defect regression outcomes

| Regression | Source result |
|---|---|
| Retained contact-ranking capability with replacement target | PASS; same capability is valid and the actual resolved target is scored. Removed impossible stale-capability oracle assumption. |
| Urdu document phrase representation | PASS; document/raw, never roman_derivative. |
| Urdu endpoint summary | PASS; compatible ipdr.endpoint/AGGREGATE; sessions/AGGREGATE excluded. |
| analyst-k29 grammar | PASS; complete source span preserved, no K29 suffix substitution. |

These are source regressions. They are not new live scores, and their retired prompts are not included in the fresh corpus.

## 8. Files in this implementation

- `api/forensic_records/query.go`
- `api/forensic_records/query_hybrid_planner.go`
- `api/forensic_records/query_language_assistance.go`
- `api/forensic_records/query_language_assistance_test.go`
- `api/forensic_records/query_hybrid_planner_ginkgo_test.go`
- `api/forensic_records/nxb21d_hybrid_development_ginkgo_test.go`
- `pkg/forensictext/query.go`

Bundle files: `scripts/nxb21d-hybrid-development/acceptance-v1.json`, `scripts/nxb21d-hybrid-development/check_hybrid_ram.ps1`, `scripts/nxb21d-hybrid-development/hybrid-32case-corpus-v1.json`, `scripts/nxb21d-hybrid-development/hybrid_config.json`, `scripts/nxb21d-hybrid-development/hybrid_config.json.sha256`, `scripts/nxb21d-hybrid-development/run_nxb21d_q4_hybrid_32case_development.ps1`, `scripts/nxb21d-hybrid-development/runtime-baseline.json`, `scripts/nxb21d-hybrid-development/test_hybrid_framework.ps1`, `scripts/nxb21d-hybrid-development/validate_hybrid_corpus.py`.

Reports: this handoff, `d-hybrid-development-freeze-v1.json` and sidecar, `d-hybrid-development-freshness-v1.json`, `d-hybrid-source-validation-v1.json`. Program ledger changes: `NEXUSAI_CONTINUATION.md`, `NEXUSAI_MASTER_DIRECTIVE.md`, all three existing roadmap files, and `configuration/nexusai_stim_maturity_matrix.json` (including its existing issue register). Temporary generation scripts and validation logs are in `tmp/`.

## 9–10. Source verification

| Check | Result / duration |
|---|---|
| Full `go test ./api/forensic_records -count=1` | PASS, 40.333 s; Ginkgo 526 PASS / 0 FAIL / 29 SKIP; live-only tests skipped. Standard Go test counts in validation JSON. |
| Hybrid + read-only corpus oracle admission | PASS, 42 specs / 0 failures / 513 filtered skips, 3.724 s. No model HTTP calls. |
| Dynamic planner + query-language focused tests | PASS, 40 Ginkgo specs plus query-language Go tests; 0 failures, 515 filtered skips; 1.127 s. Exact names in `tmp/hybrid-dynamic-language-final.log`. |
| Agent routing source tests | PASS, 0.377 s. |
| Agents Ginkgo excluding DB-only groups | PASS, 0.478 s. |
| `pkg/forensictext` | Builds; no package-local test files. Shared parser exercised by API tests. |
| Windows PowerShell byte-safe transport | PASS, 31 ms, 3282 bytes. |
| Qualification/consumed-holdout/activation guards | PASS, 28 assertions; no qualification dispatched. |
| PowerShell syntax, frozen source, identity tamper rejection, ValidateOnly | PASS; framework invocation about 5 s including freshness scan; no inference. |

The broader agents suite previously encountered 15 PostgreSQL/Testcontainers setup failures: rootless Docker is not supported on Windows. These remain ENVIRONMENT_BLOCKED, not model/planner failures or passing database tests. The final source-only suite excludes AgentScheduler, AgentStore, Four P1 terminal persistence, and Analysis history contract. No DB-container setup was retried as part of final source verification. Historical setup failures remain documented, not erased.

The initial current-source oracle check exposed neighbouring-word/IP/date identifier captures. These were corrected in source; no corpus question or expected result was edited. All 32 expected source tuples/states then admitted successfully. Source admission is not execution of the live development gate.

## 11–13. Corpus and freshness

32 cases, 8 each English / Urdu / Roman Urdu / mixed; 26 deterministic and 6 expected residual calls. IDs, questions and relevant values unique; UTF-8 without BOM. Freshness scanner compared 179 historical evidence files, including consumed/retired corpora: zero question overlap and zero relevant-value overlap. The proof includes the SHA256 of every historical file inspected. This is a bounded repository scan, not a claim about external material.

Corpus SHA256: `caec70a2bbd5023358afffe24c00a51805b7539f5eb1c77c0e827325194407c8`. Unchanged from the user-approved corpus. Freshness proof: `reports/nxb21/d-hybrid-development-freshness-v1.json`.

## 14–18. Freeze identity and full tuple

Freeze ID: `nxb21d-q4-deterministic-first-hybrid-dev-r1`.

Identity SHA256: `85566bcfcf0681fed5df649d442d25b975106e03e9d44476f88e85776547bad2`.

Identity algorithm: SHA256 of UTF-8 LF-terminated ordered key=value lines: freeze ID, artifact SHA, Q4 profile SHA, Q8 profile SHA, runtime SHA, completion budget, then every framework path/hash sorted using ordinal string order. This avoids locale-dependent PowerShell ordering. Complete 181-file manifest, including all API/query-contract source, parser, agent source, module dependencies, evaluator and bundle controls: `reports/nxb21/d-hybrid-development-freeze-v1.json`.

| Role / file | SHA256 |
|---|---|
| model artifact | `2fde00ce69dd4899c70d020845e2638353015bba0fdf161b3eb965f2bca4464e` |
| Q4 profile | `30d3bcd8c92684e191277dad992354323b7e46c7d30d3c69149010e90f117129` |
| Q8 profile | `ef036e0a02cc5e8331dc4dff41c0e0cb68e6e0bda71d92b92f47f673472e666f` |
| query handler / extraction: `api/forensic_records/query.go` | `9108703f7c6979fc6b50e05c6fb9a5c2b43fc854816f9640c04783fd4502580a` |
| fact packet, tuple builder, residual schema, assembly: `api/forensic_records/query_hybrid_planner.go` | `64a4dad50146d94302abd83ecec710556e0668781d26036ab3559b953d3539b2` |
| language assistance / production validator: `api/forensic_records/query_language_assistance.go` | `f2d960f68867576d17b667242a59c27e1d0bd70a1fc090e365fc9c60c7825760` |
| exact authority (historical helper): `scripts/nxb21d-remediation/exact_identifier_authority.ps1` | `216b0ec4e2b69c5482c65a3eef306f41b170a11a38a4b2b7a715b81134c7d304` |
| phrase authority (historical helper): `scripts/nxb21d-phrase-remediation/phrase_literal_authority.ps1` | `b2ee25a909558656d53ccd84d14e33d2d8f9eb7181b8820613cf3e5ef41bab81` |
| scope authority (historical helper): `scripts/nxb21d-scope-remediation/explicit_scope_authority.ps1` | `55cf31315216d5ec3c5524868f488af9a479ece488b1fde75f3256921960ef85` |
| shared text authority: `pkg/forensictext/query.go` | `2b75f9378c2700eb8c104013446af86a298264df188327d21de87d475b660ad8` |
| agent routing: `core/services/agents/forensic_direct.go` | `383b36623ee7f1530ddd782715bf5d84f6f0840748dc13ef26fe16dea1aa994f` |
| capability registry: `api/forensic_records/contracts/query-capability-references-v1.json` | `085982f58e9f3560f9f7291db45e520c4008ed9c492005e8e9f542cd9aa35ca0` |
| query contracts: `api/forensic_records/query_intelligence_contracts.go` | `56ea4c09d1b4d55e8f42b86b27a703aaaf646be3eeb0358d035f13c3d96cb726` |
| byte-safe PowerShell transport: `scripts/nxb21d-development/byte_safe_transport.ps1` | `9deed398e86e0347afc33964c47b8f4e489cc88372a77d16b4ba9ef8ed4a8c64` |
| corpus: `scripts/nxb21d-hybrid-development/hybrid-32case-corpus-v1.json` | `caec70a2bbd5023358afffe24c00a51805b7539f5eb1c77c0e827325194407c8` |
| evaluator: `api/forensic_records/nxb21d_hybrid_development_ginkgo_test.go` | `c80e04afba3ec0d6258f4e452278ae37daa0c5a1816352e0ae7db5381f0a2765` |
| runner: `scripts/nxb21d-hybrid-development/run_nxb21d_q4_hybrid_32case_development.ps1` | `b0410108a4c6d61db90328dde60a0cb4f77f3022950601b56c00830b3bd5194c` |
| RAM helper: `scripts/nxb21d-hybrid-development/check_hybrid_ram.ps1` | `cb11349e966ce43155707d13d2b448a998e64a71201a4624fc85a704ce63afc6` |
| acceptance: `scripts/nxb21d-hybrid-development/acceptance-v1.json` | `4fbd97aa7a645919af4442736388635df0da75853dd04c207ffb94401cc5561c` |
| runtime baseline | `756f0813c35e8c8d70e0d8c4e061d783ccd772ba859400b12b72693040540961` |
| operator configuration | `b312c676f2e2031995078b1d2ad098d8f4e24e7fc2a0c3a2aead223cdff91e38` |

Fact extraction, tuple builder, residual contract and final assembly share one module; its full-file hash binds each responsibility. Historical PowerShell authority helpers are frozen for provenance; production deterministic ownership runs in Go, not those old live evaluators. Runtime container IDs/images/restart counts are frozen in runtime-baseline.json. Read-only observation: all five healthy, retained tuple `64|64|77|22507|828|61`, Activity 314, active jobs 0, Q4 unloaded. Actual artifact and Q4/Q8 profile hashes were verified.

## 19. Frozen acceptance

32/32 final outcomes correct; critical/refusal 100%; explicit identifier/scope/date/number/literal/representation/follow-up facts 100%; six expected residual calls structurally valid and all expected tuple/ambiguity decisions correct; zero unknown tuple, authoritative override, UTF-8/schema/transport defects; three-layer audit for every case; runtime integrity PASS. Immutable acceptance file and corpus acceptance are in the identity manifest. A source PASS does not establish these live acceptance outcomes.

## 20–22. Standalone operator handoff (not executed here)

Validate without inference:

```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI
Set-ExecutionPolicy -Scope Process Bypass -Force
.\scripts\nxb21d-hybrid-development\run_nxb21d_q4_hybrid_32case_development.ps1 -ValidateOnly
```

For a subsequent operator-authorized development execution only, use the same command without `-ValidateOnly`:

```powershell
.\scripts\nxb21d-hybrid-development\run_nxb21d_q4_hybrid_32case_development.ps1
```

Expected validation markers: `CORPUS_FROZEN=PASS`, `HYBRID_FRAMEWORK=PASS live_inference=false`. Subsequent live markers: `Q4_REGISTRATION=PASS`, `RAM_GATE`, `NXB21D_HYBRID_32CASE=START`, `DEVELOPMENT_FREEZE=PASS`, `NXB21D_HYBRID_CASE`, `NXB21D_HYBRID_RESULT`, then only on success `RUNTIME_INTEGRITY=PASS`, `NXB21D_HYBRID_32CASE=PASS`, `DEVELOPMENT_RECEIPT`, `DEVELOPMENT_RECEIPT_SHA256`. A FAIL is not qualification eligibility.

Runner verifies source/config/corpus/runtime/model profiles and registration, refuses already-loaded Q4, applies RAM floors 6 GiB before load / 4 GiB loaded, preserves raw UTF-8 request/response bytes, audits and metrics, and creates a one-shot FileMode.CreateNew dispatch lock. Cleanup only targets this candidate after this run claimed dispatch; it never unloads unrelated models. Preloaded Q4 is rejected to prevent assuming ownership. Postchecks verify unloading, retained counts, Activity, no active jobs, and container identities/restarts. Failures seal incomplete aggregates and remain FAIL. The lock is not automatically removed or reused.

Freeze receipt: `reports/nxb21/d-hybrid-development-freeze-v1.json` and `.sha256`. Future live receipt (does not exist yet): `local-acceptance-models/nxb21-d/hybrid-development-runs/run-<UTC>/hybrid-development-summary-v1.json` and `.sha256`. Preserve any failed run and return its exact receipt; do not auto-retry consumed dispatch.

## 23–26. Boundaries and state

No live model inference occurred. The 32-case development gate was not dispatched. No replacement qualification holdout was created or exists for this hybrid candidate; historical consumed holdouts remain preserved. No activation, service rebuild/recreation, DB migration or retained-evidence mutation occurred. Source code has not been deployed. Live residual performance remains unknown. D remains OPEN; activation BLOCKED; replacement qualification candidate NOT_YET.

NX-B2.1D
Q4_FULL_PLAN_ROLE=RETIRED
Q4_HYBRID_ROLE=HYBRID_PROMISING
DETERMINISTIC_FIRST_HYBRID=SOURCE_VALIDATED
HYBRID_DEVELOPMENT_GATE=FROZEN_NOT_EXECUTED
FINAL_REPLACEMENT_QUALIFICATION_CANDIDATE=NOT_YET
REPLACEMENT_HOLDOUT=NOT_CREATED
D_STATUS=OPEN
ACTIVATION=BLOCKED
