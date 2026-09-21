# NexusAI STIM-5 / STIM-6 source and runtime acceptance — 2026-08-20

## Boundary and status

- STIM-3/STIM-4 source/runtime closure remains preserved.
- STIM-5 source/runtime: accepted; final status closed.
- STIM-6 source/runtime: accepted; final status closed.
- Open P0: 0. Open P1: 0.
- Database migration: not needed.
- Deployment: completed through the product-owner guarded gate; no further deployment needed.
- Retained data, worker, model inventory, profiles and named volumes: unchanged.

## STIM-5 evidence

- Multilingual semantic normalization covers English, messy English, Roman Urdu,
  Urdu and mixed script without modifying exact entity strings.
- Follow-up context is contract/version/scope/time bounded and may inherit only
  exact authorized target, operation, dates, direction and source set.
- Unsupported capability assessment precedes language assistance, SQL, KB and
  model work.
- `forensics.fact-packet/v1` bounds facts/rows/citations to 24/20/50 and labels
  representative versus aggregate proof.
- `forensics.narrative/v1` uses strict schema decoding and rejects unknown refs,
  altered exact tokens, omitted referenced facts, widened relationships,
  forbidden certainty, invented limitations/follow-ups and injected evidence
  instructions. Invalid/unavailable/timeout output falls back deterministically.
- Query variant ledger: 172 accepted occurrences, 160 unique, 12 duplicates.
- Narrative benchmark: 8/8 deterministic reconciliation cases.
- Installed-model direct probes: JSON schema parsed in three language cases, but
  all three were rejected for exact-number fidelity; observed latency was
  20,953 ms, 21,827 ms and 44,891 ms. This output is not accepted by NexusAI.
- System factual acceptance after validator/fallback: unsupported claims = 0 in
  the independent golden cases.

## STIM-6 evidence

- Data: source summary, overview, five-column mapping, quality and capability
  readiness; unreported older metadata remains explicitly unreported.
- Ask: explicit result and fallback states, direct answer, findings, metrics,
  facts/inferences, relationships, citations, limitations, suggestions and
  how-determined trace.
- Presentation: hidden six/seven-column server truncation removed; governed
  priority/progressive controls retain bounded columns; proof roles are explicit.
- History: restored findings/table/citations/limitations; Continue in Ask does
  not execute.
- Localization: English, Roman Urdu, Urdu and mixed direction metadata; Urdu RTL
  and exact identifiers isolated LTR with BiDi markup.
- Browser: Analyst Portal 11/11, rich answer case PASS, Urdu 390px mobile PASS;
  interactive 390px page/dialog widths equal scroll widths with zero overflow.

## Source validation

- `go test ./api/forensic_records`: PASS, 285 specs, 2 intentional skips.
- `go vet ./api/forensic_records`: PASS.
- `go test ./core/services/agents -args --ginkgo.focus=forensic.presentation.contract`: PASS, 7 specs.
- `go test ./core/services/agents -args "--ginkgo.skip=AgentStore|AgentScheduler"`: PASS for all non-container specs.
- Full agent attempt: 13 Windows rootless PostgreSQL testcontainer environment
  failures; the one source regression exposed by that run was fixed and the
  non-container suite then passed.
- `npm run build --offline`: PASS, 677 modules; non-blocking chunk warning.
- Focused ESLint: 0 errors, 22 warnings.
- `scripts/verify_nexusai_stim56_live.ps1`: PASS, 14/14 live checks; report
  `runtime-activation-stim56/stim56-runtime-acceptance-20260820T070738Z.json`.
- Full demo query matrix: PASS, 65 queries, 64 answered, one accepted no-result,
  P95 454.9 ms.
- `scripts/verify_nexusai_stim56_browser.ps1`: PASS, Analyst Portal 11/11,
  rich governed answer PASS, Urdu RTL at 390px PASS.
- Deployed in-app inspection: `/analyst/data`, `/analyst/ask` and
  `/analyst/history` render retained runtime content; Ask and History have no
  horizontal overflow at 390px and emit zero browser warnings/errors.

## Deferred, non-blocking issues

- P2: installed Qwen raw narrative exactness and 21--45 second direct latency;
  safely rejected/fallback-protected, challenger benchmark deferred.
- P2: model configuration reports a 4096/8192 context ambiguity; no profile
  change is authorized in this slice.
- P2: 22 focused ESLint warnings and Windows rootless testcontainer limitation.
- P2: narrative streaming is deferred; the source uses deterministic-first
  delivery and an eight-second optional narrative timeout, without fake progress.
- P3: none open in the STIM matrix.

## Challenger approval gate

`KeepCurrentModel=false` for promotion as a trusted factual narrative model.
The current model may remain installed only behind strict validation and
deterministic fallback while the product owner decides whether to authorize a
challenger benchmark.

The approval-only candidate is official `Qwen/Qwen3-8B-GGUF`, quantization
`Q4_K_M`, Apache-2.0, approximately 5.03 GB. It is an 8.2B multilingual,
post-trained model with official llama.cpp instructions and is compatible with
the existing LocalAI llama.cpp/GGUF backend. Engineering capacity estimate for
an 8K CPU profile is 7--10 GB process RAM; same-host narrative latency is
unknown and conservatively expected to be 30--90 seconds on the current
eight-thread CPU, so it must not be promoted unless the strict eight-second
system timeout and exact-fidelity benchmark pass. Official source:
https://huggingface.co/Qwen/Qwen3-8B-GGUF

No model file was downloaded. If explicitly approved later, benchmark the same
eight Fact Packets plus English, messy English, Roman Urdu, Urdu, mixed,
prompt-injection, no-result and conflict cases. Require schema/reference/value/
identifier/date/unit/relationship preservation, zero unsupported claims after
validation, zero timeouts at the system boundary, and record TTFT, total
latency, CPU and peak RAM before any alias/profile change.

## Guarded runtime closure

The product-owner ran `scripts/build_deploy_nexusai_r8_ui_api_gate.ps1` first
with `-PreflightOnly`, then without it. Activation marker:

`R8UIAPIActivation=PASS ... WorkerRebuilt=false ModelsChanged=false ProfilesChanged=false VolumesPreserved=true RollbackImages=preserved`

The live verifier and focused browser deck then passed. Runtime markers:

`STIM56RuntimeAcceptance=PASS ... DatabaseMutated=false ModelsChanged=false`

`STIM56BrowserAcceptance=PASS AnalystPortal=11/11 RichAnswer=PASS UrduRTL390=PASS ExternalServer=true`

`DeploymentNeeded=NO`. STIM-7 remains outside this slice and not started.
