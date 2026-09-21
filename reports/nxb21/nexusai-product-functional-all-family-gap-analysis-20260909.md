# NexusAI Product Functional / All-Family Gap Analysis — 2026-09-09

## 2026-09-16 NX-B2.1D functional baseline closure

This section supersedes the earlier open-state entries for the NX-B2.1D
milestone. It closes a functional baseline, not product certification.

| Work package / capability | Final state | Closing evidence / remaining boundary |
|---|---|---|
| WP0 / baseline integrity | CLOSED | Frozen branch/HEAD and protected dirty-tree rules preserved. |
| WP1 / deterministic English compiler | CLOSED | Accepted deterministic compiler remains unchanged. |
| WP2 / embedding assistance | CLOSED | 131/137 top-1, 132/137 top-3, 134/137 top-5; 122/137 confident (89.051%); 0 wrong confident; four explicit fallbacks. |
| WP3 / current-4B synthesis-role adjudication | R3_AUTHORIZED_PREPARED_NOT_EXECUTED | r1 admitted RAM but aborted before inference on a PowerShell flag defect. The authorized r2 attempt reached at most four consecutive qualifying samples (`8.661/8.663/8.665/8.666` GiB) before falling to 8.553 GiB, so the unchanged 8.627 GiB/five-sample gate failed. No Go test, model call, raw evidence, or adjudication receipt exists. r2 RAM SHA-256 is `73213cbe45979eaf0489f41b0d888fadc2a6f35e7fc71bd42fb15a059862c0fc`; sealed r2 incident SHA-256 is `e497c0177832cf2660acd5a20132cb0c133ef1a0e1f4257f9c67609a258db433`. One collision-free Q8 r3 execution is authorized; parser and full validate-only preflight pass, and all r3 outputs remain absent. |
| WP4 / document retrieval depth | CLOSED | Explicit exact/full-text/hybrid/source-scoped routing, bounded fair comparison, real extractor locators, and focused/native tests green. |
| WP5 / document answer composition | CLOSED | Passage-grounded executive answer, citations, source count, locators, limitations, and method preserved through the public response. |
| WP6 / premium UI within accepted IA | CLOSED | Fresh Vite build PASS; changed-file ESLint 0 errors; 34 existing warnings, none new; full current Playwright suite 31/31; broad 50-combination matrix PASS. |
| WP7 / five vertical flows | CLOSED | `reports/nxb21/nxb21d-five-vertical-flow-acceptance-20260916.md`: CDR, document/RAG, ANPR, generic source-native, and audio/text all PASS with request/response or screenshot-equivalent references. |
| NX-B2.1D | FUNCTIONAL_BASELINE_COMPLETE | Do not relabel as `PRODUCT_CERTIFIED`; WP3 and broader qualification boundaries remain explicit. |

The generic source-native closing proof ran through the public API against an
owned disposable forced-RLS database and then deleted it. Receipt
`reports/nxb21/source-native-sql-api-20260916T163944208Z.json` records
`passed=true`, `cleanup=true`, `retained_unchanged=true`; retained tuple
`69|69|82|22512|876|69`, Activity `354`, and active jobs `0` were unchanged.

The final repository-wide `go build ./...` is still blocked by existing
Windows-excluded `go-piper`/`go-gl` code, a local acceptance fixture missing an
embedded contract, an unreadable private-activation directory, and a sandboxed
module-cache stat write. The changed forensic package passes its complete
post-WP5 regression in 74.349 seconds. No production regression is hidden by
the repository-wide build status.

Forward register: execute the single authorized Q8 r3 attempt after additional
host-memory preparation, then adjudicate the Role A/B/C receipt if produced.
Do not weaken the RAM gate or treat admission as model-quality evidence. Retain
deterministic fallback and do not reopen WP0-WP2.
Multilingual qualification, GPU
qualification, fine-tuning, broader provider/format fixtures, deployment, and
retained-runtime product certification remain deferred. The broad matrix and
five-flow acceptance discovered no new blocking functional or IA defect.

## 2026-09-14 convergence source acceptance — development inference not admitted

Current report: `reports/nxb21/product-convergence-development-20260914.json`.
Hierarchical family/operation selection and conservative grounded paraphrases are
source-tested. Canonical calendar buckets, source/period count comparisons and
filter/group/count/sort/top-k passed 11 disposable SQL/API specifications.
The outer typed comparison guard, comparison presentation, and canonical typed
record-type/source-file filter conversion were fixed during these checks.
Full forensic and agent regressions PASS; scratch cleanup and retained tuple
`68|68|81|22511|870|68` unchanged. Running services were not deployed or modified.

The latest governed development attempt stopped before inference: peak recovered
RAM 6.602 GiB versus the unchanged 8.627 GiB admission gate. Runtime integrity PASS;
no model loaded, no semantic/synthesis scores measured, no formal proof created.
The six-question/three-synthesis development driver is built and digest-checked.
Operator next: save work, close Codex/unused applications, keep Docker running,
and run `scripts/nxb21-english-functional-qualification/run_product_convergence_development.ps1`.
Its service/model checks, admission gate, telemetry and unload guards remain active.
Model replacement is UNDETERMINED, not justified by a failed RAM preflight.

Full natural-English CDR/document/ANPR/generic/text-audio flows, remaining general
help/unsupported response integration, broader grounding and comparison coverage
remain OPEN. These SQL/API fixtures do not certify those analyst flows. V2 is
consumed and immutable; no rerun or V3. NX-B2.1D OPEN, strict PENDING.

## 2026-09-11 product convergence development — implementation in progress

Latest owner directive: hierarchical semantic selection and safe paraphrase
grounding, then disposable execution and a six-question development smoke.
V2 remains consumed; its 202 frozen source files were hash-verified and copied to
`local-acceptance-models/nxb21-english-product-proof-v2-20260911/consumed-source-snapshot`.
No new formal proof, holdout, model/profile change or deployment is authorized by
this checkpoint. Runtime admission and abort guards remain unchanged.

Source now partitions the scoped registry into 14 semantic families (66 total
operations; mean 4.71 per family, maximum 18), selects a family then its operation
under one planner deadline, and binds facts on the server. Negative English
family decisions cannot fall through to the old question-pruned chooser.
Synthesis permits conservative grammatical paraphrases with exact critical
tokens, single-proposition mapping, citation-to-fact linkage, polarity checks,
entity/value ordering and server-issued follow-up choices. This is a conservative
deterministic guard, not a general natural-language entailment oracle.
Initial full forensic regression PASS after adapting stage-specific mocks;
calendar bucket extension subsequently passed focused UTC/Pakistan/DST tests.
SQL/API, development model measurements and broader end-to-end work remain open.
NX-B2.1D OPEN; resource track CLOSED; strict certification PENDING.

## 2026-09-11 V2 consumed — runtime duration abort and functional gate failures

Current authority: `reports/nxb21/english-product-proof-v2-consumed-adjudication.json`.
V2 is CONSUMED_DO_NOT_RERUN: 40/48 complete (A20/B6/C14/D0); D01 started,
no durable D answer. The unchanged 1200-second runner wall-time guard aborted.
Cold RAM admission PASS, owned-model unload PASS, runtime/retained integrity PASS.
A supported direct operation selection 0/14; B routing 5/6 (B05 deterministic
fast-path error); C validated narratives 1/14, thirteen safe grounding fallbacks.
All fourteen C HTTP/model outputs completed; no C timeout. Rejection includes
verbatim-sentence constraints and unissued follow-ups, not thirteen proven
hallucinations. C05 retains values but fails punctuation-sensitive exact-sentence
coverage. Frozen scores remain unchanged; C09 stored-output review passes.
Current integrated configuration is insufficient for English functional
qualification; model artifact remains runtime baseline only, strict PENDING.
No resume, lock deletion, duration increase or immediate V3. Next work is generic
planner, routing and synthesis-contract diagnosis with independent development
fixtures. NX-B2.1D OPEN; resource track remains CLOSED. Earlier handoff below
was valid before consumption and must not be used as a new run authorization.


## 2026-09-11 fresh English product proof V2 — frozen operator handoff

Authoritative handoff: `reports/nxb21/english-product-proof-v2-handoff.json`.
Proof ID: `nxb21-english-product-proof-v2-20260911`; 48 fresh cases: A20/B6/C14/D8.
Disposable PostgreSQL + actual typed API grouping acceptance PASS, including
source-row/group counts, ties, exact lineage, scope, filtering, zero results,
100-group/1000-row boundaries and fail-closed controls. The NULL bucket uses an
explicitly nullable adversarial scratch fixture; production columns stay NOT NULL.
Owned disposable databases/roles removed; retained tuple unchanged.
Fixed the help adapter discovery URL and rejection of ignored group sort aliases.
Full forensic and agents regressions, proxy, Ask presentation (8/8), evaluator
self-tests and PowerShell 5.1/7 checks PASS. ValidateOnly PASS in both shells:
identities/runtime integrity pass, no inference, no dispatch or consumption.
Memory state: OPERATOR_MEMORY_CLEANUP_REQUIRED; cold gate remains 8.627 GiB.
Current 4B model/profile/envelope unchanged; resource track CLOSED.
Freshness: 15,845 files scanned; zero normalized exact/prior-corpus overlap.
The consumed V1 artifacts and all 177 snapshot files remain preserved.
Next action is the single handoff command after manual app closure; no additional
build, corpus edit, source tweak, tuning, or automatic proof dispatch.
Postrun content review must use immutable captured outputs and separate hash-bound
adjudication. Safe fallback does not count as model narrative success (12/14
validated model narratives required); supported direct selection requires 14/14.
NX-B2.1D remains OPEN; strict certification, general TIME_BUCKET/COMPARE/MULTI_STEP,
broader family acceptance, browser/deployment and model-effectiveness closure are
not established. Historical entries below describe their earlier checkpoint.


## 2026-09-11 NX-B2.1D post-proof source remediation

Final source checks: full forensic package PASS (53.556 s), full agents PASS
(50.747 s), proxy timeout PASS, Ask presentation 8/8 PASS. Grouping passes
governed plan validation, executor dispatch and presentation helpers with the
100-group output ceiling. SQL/API live acceptance remains pending.

45_CASE_PROOF=CONSUMED_FAILED; RESOURCE=PASS; current 4B KEEP as the English
functional runtime baseline. English model effectiveness and strict certification
remain unproven/pending; NX-B2.1D remains OPEN. No live inference, deployment,
service restart, model/profile change, RAM-gate change or new proof in this pass.

PLANNER=GENERIC_REMEDIATION: the selection prompt now separates operation meaning
from execution requirements. The model still emits only an issued enum; the
server binds scope/facts and records READY, SERVER_BINDABLE or USER_FACT_REQUIRED, rechecking after
final request binding. All 66 workspace descriptors are audited in
`reports/nxb21/semantic-binding-descriptor-audit-v1.json`; no question-based candidate
pruning or benchmark phrase patches. This repairs a contract defect; it does not
prove the cause of every historical model rejection or improved model behavior.

SYNTHESIS_TIMEOUT=SOURCE_DEFECT: removed the internal eight-second cap. Both
synthesis paths default to configured 120 seconds, clamp to 180 seconds and honor
earlier parent deadlines/cancellation. The agent and proxy share a 405-second
transport ceiling (180 planning + 30 execution + 180 synthesis + 15 delivery).
Individual model calls and frozen runtime guards are unchanged. Tests cover an
actual five-second cancellation, valid output after eight seconds, parent cancel,
malformed responses, invented values/foreign references and exact fallback retention.

GENERAL_HELP=GROUNDING_REMEDIATION: non-deterministic forensic assistant turns
receive question-independent adapter/template discovery with authenticated scope,
a five-second deadline, bounded response bytes and bounded model context. Catalog
status is preserved; registry presence cannot establish runtime availability,
licensing or successful processing. Domain policy distinguishes subscription,
SIM/profile, equipment and phone identifiers; IPDR fields vary by provider;
similarity is not calibrated probability and universal thresholds are forbidden.
Model compliance requires fresh evidence and is not certified by source tests.

PROJECT retains its existing seven-field source-validated implementation.
AD_HOC_GROUP now has a bounded canonical count implementation for record_type or
source_file: at most 1000 complete input rows and 100 groups, explicit null bucket,
count-descending deterministic ties, exact row lineage, scope/type checks and no
SQL or expressions from the caller. Overflow fails closed. Projection, custom
sort and pagination cannot be silently combined. Typed lowering, agent argument,
catalog input and executor are wired; live SQL/API acceptance remains pending.
Source-native grouping, general TIME_BUCKET/COMPARE/MULTI_STEP remain OPEN.
Document literal/source/version correctness remains source tested; broad OCR/RAG,
all-family acceptance and deployed Ask interaction qualification remain OPEN.

Historical consumed runner, corpus, freeze, evaluator binary and receipts are
preserved. Original frozen API/agent source is retained under
`local-acceptance-models/nxb21-small-english-product-proof-v1/consumed-source-snapshot`.
Current source intentionally differs from that historical freeze; do not rebuild
or rerun the consumed package. Final source checks and exact next actions are in
`reports/nxb21/english-functional-remediation-source-20260910.json`.


## 2026-09-10 English proof completed — functional gate failed, resource fit retained

Run `run-20260910T100919072Z` completed all 45 cases. Receipt hash verified;
runtime_before equals runtime_after, unload PASS, retained tuple/Activity/jobs and
service identities/restarts unchanged. Corpus is CONSUMED_DO_NOT_RERUN. The runner's
combined INCOMPLETE_OR_FAILED label means completed-but-failed here: all rows exist,
evaluator completed normally with 11 automated failures. No RAM-admission issue
remains for this run and no further resource tuning is called for.

Stage A: schema-valid 16/16, zero operation selections. Twelve insufficient-facts
clarifications and four unsupported responses; 11 frozen-oracle failures (nine
supported-operation clarifications and two ambiguity/follow-up rejections). Five
accepted enum outcomes are not five successful analytical operations. Planner
root cause is not proven: task framing/binding context needs source diagnosis.
Stage B: 8/8 router outcomes pass (three deterministic, five model proposals), not
live end-to-end tool execution. Stage C: 0/13 model narratives, 13/13 deterministic
fallbacks. Every call hit the internal 8-second maxNarrativeWait cap despite config
SynthesisTimeout=120s. Earlier statements that this path used 120s were incorrect.
Exact fallback facts, identifiers, limitations and references pass independent
comparison; this does not establish model synthesis quality.
Stage D: eight responses captured; content review finds 3 acceptable, 3 qualified,
2 failures. OCR answer invents universal confidence thresholds (90%/70%) without
calibration; file-type answer invents licensing dependence and an unsupported claim
that no forensic capability lookup exists. Other caveats include IMSI/device wording,
IPDR provider variation, and treating face similarity as a probability.

Runtime: physical minimum 3.683 GiB; minimum commit reserve 10.526 GiB; WSL swap
growth 48652 KiB (~47.5 MiB); four isolated paging spikes, maximum 4604 pages/s, no
recorded sustained guard breach. Evaluator duration 571.618 s. KEEP runtime baseline;
English functional qualification FAILED, strict PENDING, historical multilingual
insufficiency preserved, resource track CLOSED, D OPEN. No deployment or new inference.

Machine adjudication with artifact hashes and per-answer review:
`reports/nxb21/small-english-product-proof-adjudication-v1.json`.
Next: repair narrative-timeout integration with cancellation tests, diagnose generic
planner task boundaries without phrase patches, ground help in capability discovery,
and continue bounded algebra/family work. Do not rerun this consumed proof or silently
change its oracles, historical receipts, runner, binary or freeze.


## 2026-09-10 authorized RAM cleanup and deferred proof launch

Owner requested safe cleanup and completion without changing the gate. Closed the
Docker dashboard and PowerToys Quick Access windows gracefully; stopped the verified
idle Command Palette background process. No files deleted, no protected service
stopped, no model unloaded. All five NexusAI containers remained running. Available
physical RAM sampled 6.483 GiB while Codex remained open; no gate-pass claim.

One-time helper PID 21068 is armed in
`local-acceptance-models/nxb21-small-english-product-proof-v1/launch-20260910T100236548Z`.
It waits up to 15 minutes for exact Codex app PID 27640/start-time identity to exit,
then invokes the unchanged frozen proof once. It does not close Codex itself,
change admission, or retry. Read launcher-state.json and proof-console.log on
return, then latest product receipt and all answers. English proof remains pending
until actual execution and independent review; do not launch a concurrent runner.


## 2026-09-10 standalone English proof — cold admission not met

Receipt `run-20260910T095549213Z/product-proof-receipt.json` and its SHA-256
verified. All 24 cold-admission samples were below the frozen 8.627 GiB floor;
maximum and last sample 8.473 GiB, shortfall 0.154 GiB (157.696 MiB). The recovered
plateau was approximately 8.403–8.473 GiB. No model request was dispatched;
`dispatch_created=false`, both dispatch locks absent, corpus remains UNCONSUMED.
Before/after runtime state is identical: retained tuple 68|68|81|22511|870|68,
Activity 353, jobs 0, service identities/restarts and loaded-model set unchanged.
Unload NOT_REQUIRED; runtime integrity PASS.

This is PRELOAD_ADMISSION_NOT_MET, not model execution or semantic failure.
KEEP / ENGLISH_FUNCTIONAL_BASELINE and the completed tested-shape runtime PASS
remain unchanged. English product proof and D remain pending. No lowering of cold
admission, no reuse of the 8.5 GiB reload gate for cold loading, no replacement
corpus, and no repeated resource-tuning loop. The existing proof can run only when
its unchanged admission prerequisites are met. Source-only work may continue;
editing frozen evaluator dependencies requires explicit package reconciliation
before any eventual dispatch. Earlier immediate-run instructions are superseded
by this admission status.


## 2026-09-10 completed characterization — runtime frozen; English proof prepared

Read-only proof preflight on 2026-09-10 verified frozen/source/model identities,
accepted service IDs and retained baseline, then stopped at the unchanged emergency
physical guard: available 1.181 GiB, commit reserve 7.453 GiB, paging 0, current 4B
unloaded. State PREFLIGHT_FAILURE_NO_INFERENCE. Both proof dispatch locks remain
absent. This is current-host admission state with applications open, not a reversal
of the completed model characterization. No threshold change or resource rerun.
Operator action remains manual app closure and one standalone product-proof run.

Final run `run-20260910T052648583Z` completed all 14 requests with clean unload
and runtime/retained integrity PASS. Current 4B decision KEEP, runtime stage
ENGLISH_FUNCTIONAL_BASELINE, sustained fit PASS_FOR_TESTED_PRODUCT_SHAPES.
Strict certification PENDING; historical multilingual insufficiency and consumed
r1/r2/Decision8/8B receipts are preserved. The current-host 8B resource path and
4B resource-tuning track are CLOSED.

`configuration/current4b-runtime-envelope-v1.json` freezes the measured policy:
cold 8.627 GiB; reload 8.5 GiB with five stable multi-signal samples; emergency
physical >1.5 GiB, commit >2 GiB, swap growth <=256 MiB; sustained paging guard
1024 pages/s for 15 s; request cap 180 s and session cap 1200 s. Build/recreate
remains 6 GiB. The 4.191 GiB observed minimum is not proof of sustained operation
at the emergency floor. One transient paging spike and incomplete immediate WSL
recovery are disclosed. Large identical-body timings 131.100/6.093 s are consistent
with prompt/context reuse; cache-hit causality is unproven and tiny priming already
preceded the slow request. No additional latency experiment is planned.

A single fresh 45-case English proof covers 16 planner, 8 router, 13 synthetic
Fact Packet synthesis and 8 general-assistant cases. Its offline freshness check
scanned 5425 text files without normalized exact-question overlap. Independent
content review is mandatory; fallback is reported separately and never counts as
model synthesis success. This is source-component evidence, not deployed end-to-end
or family certification. Runner:
`scripts/nxb21-english-functional-qualification/run_small_english_product_proof.ps1`.
Run once after manually closing nonessential apps; keep Docker/WSL and a terminal.
No inference has been dispatched by this work package.

Source progress: typed canonical PROJECT now retains provenance, exact identifiers
and nulls after privacy redaction; seven allowlisted fields, no payload expressions,
no ignored controls. Full forensic regression passed in 41.120 s; later offline
fixture/projection/deadline checks passed. The planner has a cancellable 180 s
maximum, and Ask displays actual elapsed time and cancellation acknowledgement.
AD_HOC_GROUP, TIME_BUCKET, general COMPARE and multistep composition remain OPEN.
P1 family and live acceptance remain OPEN. NX-B2.1D remains OPEN pending English
proof and remaining functional requirements, not future multilingual optimization.
Next sequence remains D → E proof framework → F family certification → G Ask UX
→ H live demo. Deployment/recreation and retained mutation require explicit approval.
Earlier next-run instructions below are historical where superseded.


Frozen package verification: `small-english-product-freeze-v1.json` and evaluator
hashes PASS; final freshness scan 5426 files. The existing guarded runtime functions
and the new runner pass PowerShell 5.1 offline checks. The standalone binary runs
its fixture validation without dispatching the live proof. Focused agent routing
regression PASS (0.367 s). No dispatch lock exists.

After manually closing nonessential applications, run once from a terminal:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\sheik\Workspace\Office\Projects\NexusAI\scripts\nxb21-english-functional-qualification\run_small_english_product_proof.ps1"
```

This invokes one bounded product proof, not another resource characterization.
Do not rerun after a dispatch lock is created, including on partial completion.
Keep all results and return the receipt for adjudication. Successful automated
completion still requires inspection of all answers; it cannot certify the model
or close D by itself.

Service CPU maxima from final telemetry (Docker percent can exceed 100 across
cores): API 800.48%, records API 1.07%, worker 35.36%, NATS 10.30%, PostgreSQL 9.94%.
First-to-final service memory: records API 19.95→19.88 MiB, worker 426.1→427.4 MiB,
NATS 14.32→14.92 MiB, PostgreSQL 143.4→144.1 MiB. These are sampled observations,
not a guarantee of unobserved peak usage.

### Required final runtime adjudication

```text
FINAL_4B_CHARACTERIZATION=PASS_14_OF_14
CURRENT_4B_MODEL_DECISION=KEEP
CURRENT_4B_RUNTIME_STAGE=ENGLISH_FUNCTIONAL_BASELINE
MODEL_RESOURCE_ENVELOPE=CURRENT_4B_RUNTIME_ENVELOPE_V1_FROZEN_FOR_TESTED_SHAPES
BUILD_GATE=6_GIB_UNCHANGED
COLD_ADMISSION=8.627_GIB_FIVE_STABLE_SAMPLES
RELOAD_ADMISSION=8.5_GIB_FIVE_MULTI_SIGNAL_SAMPLES
PHYSICAL_RESERVE_POLICY=ABORT_AT_OR_BELOW_1.5_GIB_OBSERVED_MINIMUM_4.191_GIB
COMMIT_POLICY=ABORT_AT_OR_BELOW_2_GIB_RELOAD_AT_LEAST_6.9_GIB
PAGING_POLICY=ABORT_AT_1024_PAGES_PER_SECOND_FOR_15_SECONDS
SWAP_POLICY=ABORT_GROWTH_ABOVE_256_MIB
BACKEND_RSS_POLICY=RELOAD_UNLOADED_BASELINE_533.898_MIB_PLUS_256_MIB_TOLERANCE
TIMEOUT_POLICY=180_SECONDS_MAX_REQUEST_1200_SECONDS_SESSION_SYNTHESIS_120_SECONDS
MODEL_LOAD_LATENCY=14.756_13.794_13.956_SECONDS
TINY_CALL_LATENCY=6.047_TO_6.813_SECONDS
PLANNER_SHAPED_LATENCY=131.100_AND_6.093_SECONDS
SYNTHESIS_SHAPED_LATENCY=43.724_SECONDS
LATENCY_CLASSIFICATION=PROMPT_CONTEXT_REUSE_CONSISTENT_CAUSALITY_UNPROVEN
RUNTIME_INTEGRITY=PASS
RETAINED_INTEGRITY=PASS_TUPLE_68|68|81|22511|870|68_ACTIVITY_353_ACTIVE_JOBS_0
NO_MORE_4B_RESOURCE_TUNING=YES
FRESH_ENGLISH_PROOF_REQUIRED=YES_45_CASE_PACKAGE_PREPARED_UNCONSUMED
QUERY_ALGEBRA_STATE=CANONICAL_PROJECT_SOURCE_VALIDATED_BROADER_COMPOSITION_OPEN
P1_FAMILY_STATE=REPRESENTATIVE_ACCEPTANCE_OPEN
D_STATE=OPEN_FUNCTIONAL_REQUIREMENTS_PENDING
NEXT_IMPLEMENTATION_TARGET=ENGLISH_PROOF_THEN_TYPED_GROUP_TIME_COMPARE_COMPOSITION_AND_P1_FAMILIES
APPROVAL_REQUIRED=LIVE_ACTIVATION_ONLY_STANDALONE_PROOF_ALREADY_AUTHORIZED
```

The receipt contains protected-service memory and CPU samples, unchanged container
identities/restart counters, zero OOM, constant WSL swap 793048 KiB, pagefile usage
1101 → 1050 MiB, commit reserve minimum 10.554 GiB, and final owned-backend removal.
Backend RSS peaked at 5394.777 MiB and returned to 533.898 MiB; API memory peaked
at 1478.656 MiB. Final immediate physical availability was 4.653 GiB and vmmemWSL
5407.934 MiB: unload completion is not instantaneous full Windows RAM recovery.

UI validation: eight Ask presentation helper tests PASS. JSX build verification
was unavailable because frontend dependencies are absent; no dependency installation
or production build was performed. Existing cancellation remains server-acknowledged.


## Final current-4B admission adjudication — 2026-09-10

The completed v2 run `run-20260910T050248809Z` is immutable and classified
`INCOMPLETE_PRE_RELOAD_ADMISSION`. Machine extraction of every NDJSON sample is
`reports/nxb21/current4b-v2-runtime-adjudication.json`. This section supersedes
prior next-run instructions; do not rerun v2 unchanged.

| Required field | Adjudication |
|---|---|
| CURRENT_BRANCH | codex/forensic-hybrid-checkpoint-20260723 |
| CURRENT_HEAD | 40717b83510c08db25dc26b9d6674bf46db363ac |
| FIRST_LOAD_RUNTIME_RESULT | SAFE_AND_STABLE_FOR_TESTED_TINY_WORKLOAD; larger prompts unproven |
| FIRST_LOAD_CALLS_COMPLETED | 5: load plus 4 tiny calls; no timeout |
| INITIAL_STABLE_PRELOAD | 8.771, 8.777, 8.777, 8.758, 8.750 GiB |
| FIRST_LOAD_RAM_RANGE | Settled/in-flight loaded range 4.095–4.163 GiB; initial loading transition also sampled 4.990 GiB |
| POST_LOAD / CALL_1 / CALL_2 / CALL_3 / CALL_4 | 4.163 / 4.159 / 4.159 / 4.145 / 4.135 GiB |
| FIRST_LOAD_RAM_DRIFT | Post-load to final -0.028 GiB; four-call batch boundaries -0.035 GiB |
| FIRST_LOAD_LATENCY | Load 13,364 ms; tiny median 6,793.5 ms, max 6,816 ms; runner latency includes telemetry polling overhead |
| REQUEST_TOKEN_COUNTS | Each input 34 tokens; load output 12 tokens; each tiny output 9 tokens |
| COMMIT_RESERVE | Preload 15.532 GiB; loaded minimum 10.975 GiB; post-load to end -0.011 GiB |
| COMMIT_USED / LIMIT | 12,404,101,120 bytes before load; peak 17,296,928,768; limit 29,081,632,768 |
| PAGEFILE_STATE | 775 MiB before load to 770 MiB; unchanged through four calls; allocation 11,644 MiB |
| MEMORY_COMPRESSION | 515.512 to 485.055 MiB working set |
| PAGING_STATE | Input peak 127 pages/s, output zero; no recorded sustained severe paging; distinct hard-fault counter not recorded |
| WSL_SWAP_STATE | 793,060 KiB before and throughout inference; zero growth; post-unload swap not recorded in v2 |
| VMMEMWSL | 1,068.406 MiB before load to 5,844.129 MiB last; post-load growth 46.871 MiB |
| BACKEND_RSS_TREND | Existing PID 41 stays 533.898 MiB; new PID 4396 post-load 4,765.688 to 4,770.578 MiB, +4.890 MiB |
| API_CONTAINER_MEMORY | 639.9 MiB before load; 1,376.256 MiB post-load; 1,394.688 MiB final (Docker accounting differs from summed process RSS) |
| PROTECTED_SERVICE_MEMORY | Records API 19.95→19.95 MiB; worker 428.3→427.2; Postgres 141.6→143.8; NATS 15.17→14.71 |
| SERVICE_CPU_PEAK | API 85%; records API 0.44%; worker 17.11%; Postgres 2.21%; NATS 0.75% (sampled Docker percentages) |
| PROTECTED_SERVICES | All running, no OOMKilled; restart counts unchanged at 0/6/4/0/0; records API has no Docker healthcheck |
| MODEL_UNLOAD / RUNTIME_INTEGRITY | PASS / PASS; before and after state objects identical |
| RETAINED / ACTIVITY / JOBS | `68\|68\|81\|22511\|870\|68` / 353 / 0 before and after |
| MEASURED_UNLOAD_RECOVERY | Last physical sample 8.527 GiB, +4.392 GiB over final loaded sample; peak recovery 8.679 GiB. Commit/pagefile/swap after unload were not sampled; no claim is inferred |
| RELOAD_8_627_ORIGIN | Desired 4 GiB loaded reserve + historical 4.627 GiB load delta |
| RELOAD_8_627_CLASSIFICATION | Conservative predictive harness admission guard, formally frozen for that run; not measured hard limit |
| RELOAD_8_627_EMPIRICALLY_REQUIRED | NOT_ESTABLISHED |
| MODEL_SPECIFIC_ENVELOPE_JUSTIFIED | Candidate final-experiment reload envelope justified; sustained product envelope not yet qualified |
| CURRENT_INFERENCE_GATE_POLICY | Initial admission 8.627 GiB unchanged; reload candidate 8.5 GiB plus multi-signal checks; existing inference abort guards unchanged |
| BUILD_GATE_POLICY | 6_GIB_UNCHANGED |
| HARNESS_CORRECTION_REQUIRED | YES, reload admission only plus final recovery telemetry; no model/profile/prompt changes |
| LIVE_INFERENCE | false during this preparation |

The measured before-to-postload draw is 4.609 GiB. Conservative peak draw across
all recorded loaded samples is 8.772−4.095 = 4.677 GiB. The reload calculation is
`ceil_to_0.1(4.677 + 3.5 + 0.25) = 8.5 GiB`. The 3.5 GiB selected reserve is an
explicit engineering allowance of 2 GiB above the unchanged 1.5 GiB emergency
abort; it is not a measured hard boundary. The 0.25 GiB margin is three times the
observed 0.068 GiB steady physical range, rounded upward to a quarter GiB. The
threshold is derived independently of the final recovered RAM reading.

Five consecutive reload samples must satisfy physical admission, >=6.9 GiB
commit reserve (measured 4.557 GiB peak commit draw + existing 2 GiB emergency
reserve + 0.25 GiB margin, rounded upward), <=127 combined paging pages/s (the
observed successful interval peak), WSL swap growth <=256 MiB, and unloaded
backend RSS <=533.898+256 MiB. The latter headroom tolerates telemetry noise but
rejects a remaining multi-GiB model process. First-to-last physical and commit
reserve decline must each be <=0.25 GiB. All protected service checks, clean
model unload, unrelated loaded-model state, retained tuple, Activity and jobs
must pass. Wait is bounded at 120 seconds. The final operating envelope is still
CANDIDATE, never automatically claimed proven by threshold arithmetic.

The inherited runtime aborts remain physical <=1.5 GiB, commit <=2 GiB,
WSL swap growth >256 MiB, >=1024 paging pages/s sustained 15 seconds, service
identity/restart/OOM/health failure, 180-second request timeout and 20-minute
experiment guard. Backend trends and prompt-shape latency require final receipt
adjudication; no tiny-call result certifies synthesis or planning. Final cleanup
now records resource telemetry to close the previous post-unload measurement gap.

Final live read-only ValidateOnly PASS; no inference. Receipt: `local-acceptance-models/nxb21-current4b-final-characterization/run-20260910T052153270Z/validation-only.json`.

Validation: old/new model identity checks, request construction, inference call,
inference aborts and unload functions are byte-identical. Hash verification
preserves all v2 run files and prior freezes. Unit tests pass in Windows
PowerShell 5.1 and PowerShell 7, including threshold arithmetic, fluctuating
valid windows, count/floor/trend rejection, commit/swap/paging/backend admission,
and inherited dictionary/empty telemetry, abort and ownership tests.
Six new isolated document acceptance cases cover exact identifier boundaries,
phrase versus unordered tokens, raw page text/citations and tenant/collection/
source/version exclusion. Full forensic-records regression PASS, 66.603 seconds.
No production behavior, query architecture, retained evidence or UI was changed
in this adjudication package. This is additional source acceptance, not a claim
of all-family live acceptance.

FINAL_CHARACTERIZATION_RUNNER=
`scripts/nxb21-english-functional-qualification/run_current4b_final_characterization.ps1`

FINAL_RUNNER_SHA256=
`ec0387052e0de7041799f869341000469211ccaaeda53dbf6bbcbbba53b2470b`

Final freeze SHA-256:
`833e39d6acfdb466a3709deebe42d6b909cddc313a7b7e2ac84f19a807e50b02`.

Per the owner's final directive sections 13 and 22, do not execute with Codex
open. Close Codex/ChatGPT, browsers, IDEs, Teams/Slack/Office and other
nonessential applications manually. Keep Docker Desktop, required WSL and one
PowerShell terminal. Wait 20–30 seconds. Run ONCE:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\sheik\Workspace\Office\Projects\NexusAI\scripts\nxb21-english-functional-qualification\run_current4b_final_characterization.ps1"
```

The unchanged request sequence is cold load, four tiny calls, unload/reload,
four tiny calls, unload/reload, two planner-shaped generic calls and one
synthesis-shaped generic call, final unload and recovery checks. Generic payloads
are unchanged from v2, no corpus is consumed, and token/byte counts are recorded.
The exclusive dispatch lock is under
`local-acceptance-models/nxb21-current4b-final-characterization/`.
No automatic app termination, downloads, build/recreate, migration or retained
mutation occurs. On completed acceptable runtime evidence: keep 4B as the runtime
baseline, freeze the measured envelope, strict certification still pending, then
small English planner/router/synthesis proofs and product work. On failure:
reject sustained local role and select one smaller artifact from at most three
primary-source candidates. No further 4B profile/threshold/soak loop is authorized.
D remains OPEN, and no new English corpus is created here.


## 2026-09-10 v2 convergence checkpoint — source correction and operator handoff

This is the current checkpoint of the existing product-completion report and
`product-query-capability-matrix-v1.json`, not a second program. The owner's
2026-09-10 completion directive supersedes earlier instruction to stop all
product work pending strict multilingual qualification. Historical evaluations,
freezes, failures, and consumed locks remain unchanged. D is still OPEN; no
functional model baseline, strict certification, family closure, or deployment
is claimed by this work package.

### Actual state

| Field | Verified state |
|---|---|
| CURRENT_BRANCH | `codex/forensic-hybrid-checkpoint-20260723` |
| CURRENT_HEAD | `40717b83510c08db25dc26b9d6674bf46db363ac` |
| GIT_STATUS_SUMMARY | Initial 93 modified tracked files and 786 untracked entries; preserved, no staging/commit |
| CURRENT_ACTIVE_PHASE / CURRENT_D_STATE | NX-B2.1D / OPEN |
| CURRENT_UI_STATE | Accepted API image `sha256:91620e4c932764c46931dd4eecb47f9f72408e2387da3189f8fcd4ab4d7506be`; `/analyst` HTML SHA-256 `a0f1e4288c1b9ee46b4fa3c3c94870fef46b12d7c1718c36eee2e905d14ae7de` matches |
| CURRENT_RETAINED_TUPLE / CURRENT_ACTIVITY / ACTIVE_JOBS | `68|68|81|22511|870|68` / 353 / 0 |
| Containers | All five running; no OOMKilled; restarts API/records API/worker/Postgres/NATS = 0/6/4/0/0, unchanged from old soak |
| QWEN3_8B | CLOSED_RESOURCE_INSUFFICIENT_ON_CURRENT_HOST; no rerun |
| CURRENT_4B_MODEL | `qwen_qwen3-4b-instruct-2507`, Q8_0, 4,280,405,216 bytes; artifact/profile hashes match frozen identity |
| CURRENT_MODEL_LOADED | 4B unloaded; face-detect-yunet-sface and qwen3-embedding-0.6b loaded |
| R1_STATE / R2_STATE | Consumed incomplete resource/runtime evidence / consumed first-request resource abort with zero completed semantic cases |
| LATEST_RESOURCE_SOAK_STATE | Historical v1 receipt labels resource failure, but its property-aggregation failure is HARNESS_FAILURE; no semantic-quality conclusion |
| CURRENT_OPERATION_COUNT / CURRENT_CERTIFIED_OPERATION_COUNT | 79 / 5 (4 PRODUCT_CERTIFIED, 1 FIXTURE_CERTIFIED); 74 others |
| CURRENT_SEMANTIC_CANDIDATE_COUNT | 66 workspace candidates, existing scope-projection tests pass; exposure 5 queryable / 63 limited / 11 engineering-only |
| CURRENT_QUERY_ALGEBRA | Bounded registered-operation algebra; new control-shape and repeated-scalar checks source validated |
| PROJECT / AD_HOC_GROUP / TIME_BUCKET / COMPARE / MULTI_STEP | Arbitrary forms remain MISSING; registered operation-specific calculations are separate |
| DOCUMENT / EXACT_SEARCH / PHRASE_SEARCH | PARTIAL end-to-end acceptance; existing scoped derived-text exact/phrase paths retained and package regressions pass |
| FULL_TEXT_SEARCH / SEMANTIC_SEARCH / HYBRID_RAG | Lexical ranking breadth unresolved; semantic/KB routes exist; current full functional flow not requalified |
| OCR / ANPR / IMAGE / VIDEO / AUDIO_STT / FACE_SIMILARITY | PARTIAL current product acceptance; bounded prior capabilities retained; no processor inference/reprocessing in this package |
| CDR / IPDR / SUBSCRIBER / TOWER / FINANCIAL / LOGS_ACCESS / GENERIC_TABULAR | Bounded source operations exist; PARTIAL against the owner's complete family-done rule |
| SPREADSHEET | PARTIAL; read-only normalization exists, representative retained acceptance absent |
| CROSS_FAMILY | PARTIAL; existing governed exact correlation/composition retained, broad new workflow acceptance pending |
| GROUNDED_SYNTHESIS / GENERAL_ASSISTANT / FOLLOW_UP | Source paths and deterministic fallback retained; new real English functional evidence pending |
| PREMIUM_UI | Accepted baseline preserved; requested further visual refinement NOT_STARTED |

### Model resource adjudication

`MODEL_RAM_FLOOR_ORIGIN=FORMALIZED_QUALIFICATION_GUARD_WITH_UNPROVEN_EMPIRICAL_BASIS`.
The September qualification runners and freezes explicitly enforce 6 GiB
preload/4 GiB loaded. The reviewed receipts demonstrate threshold crossings,
not a measured OOM or severe paging boundary at 4 GiB. Do not relabel the
historical guard as nonexistent policy or retroactively pass its failed runs.
`MODEL_RAM_FLOOR_RISK_BEING_CONTROLLED=HOST_AND_PROTECTED_RUNTIME_RESOURCE_EXHAUSTION`.
`MODEL_RAM_GATE_POLICY_DECISION=ONE_OWNER_AUTHORIZED_CHARACTERIZATION_PENDING`.
`BUILD_RAM_GATE=6_GIB_UNCHANGED`.

The old soak receipt is
`local-acceptance-models/nxb21-current4b-resource-lifecycle-soak/run-20260909T114835361Z/resource-lifecycle-receipt-v1.json`.
Its observations stop at preload; zero batch or shaped results exist. Windows
PowerShell 5.1 reproduces failure of `Measure-Object -Property rss_mib` on
OrderedDictionary rows. Empty input also fails under strict mode. The failing
telemetry call precedes HTTP dispatch in the source, while `modelOwned` was set
earlier; this reconstructs a pre-request harness failure and explains the
unnecessary shutdown attempt. The receipt itself is preserved, including its
original classification. No model-quality conclusion follows from this failure.

The new v2 runner fixes dictionary/empty aggregation, claims ownership only at
dispatch, handles an already-absent backend, and prevents unload/integrity failure
from yielding success. It records durable per-call/in-flight RAM, commit reserve,
pagefile, paging, WSL swap, vmmem, backend RSS, service CPU/memory, token usage,
latency, and before/after retained and unrelated loaded-model state.
Memory Compression is nullable when the OS does not expose that process.
Backend RSS includes all matching llama.cpp backends and per-process IDs; it is
not falsely labeled as exclusively the 4B process.

The bounded experiment uses 8 tiny calls in two batches, 3 load calls (two
reloads), 2 planner-shaped calls and 1 synthesis-shaped call. Maximum inference
request time is 180 seconds; the overall experiment guard is 20 minutes, plus
bounded cleanup/precondition overhead. The preload admission remains 8.627 GiB
for five stable samples. The old 4 GiB guard is recorded diagnostically during
this experiment. Experimental aborts are physical reserve <=1.5 GiB, commit
reserve <=2 GiB, WSL swap growth >256 MiB, >=1024 paging pages/sec sustained
15 seconds, duration limit, or protected-service failure/change. Physical
headroom reuses the existing isolated-evaluator policy; the other thresholds
are conservative experiment stop criteria, not measured operating limits.
**No lower model operating floor is adopted.** Passing the workload still
requires latency, memory trend/recovery, paging, unload and integrity adjudication;
it does not produce semantic certification or automatically freeze a model role.

### Implemented and tested

- Typed plans now reject malformed filter/sort arrays and entries, non-string or
  blank measures/grouping, and unconsumed filter/sort extensions before permissive
  compatibility conversion can discard them. Unsupported projection/bucket/
  comparison/step controls are rejected.
- Repeated scalar direction and video-bound predicates, including aliases, are
  rejected instead of losing constraints. Canonical lower/upper range conjunctions
  remain intact. This is a P1 constraint-preservation fix, not new algebra breadth.
- Full `go test ./api/forensic_records -count=1` PASS in 39.778 seconds, including
  18 new Ginkgo cases. An earlier focused invocation failed because PowerShell
  split the unquoted dotted flag; it is not a test or product failure.
- Characterization tests PASS on Windows PowerShell 5.1 and PowerShell 7.6.5:
  empty/dictionary RSS, telemetry-failure ownership, absent-model unload, memory/
  commit/swap/paging/service aborts, and small/planner/synthesis request shapes.
- Freeze verification PASS; consumed r2 and original v1 soak files unchanged.
- Live `-ValidateOnly` PASS with **no inference**. Receipt:
  `local-acceptance-models/nxb21-current4b-runtime-characterization-v2/run-20260910T045347582Z/validation-only.json`.
  This sample measured 2.109 GiB available, 8.690 GiB commit reserve, zero input/
  output paging rates and 793,060 KiB existing WSL swap. It followed artifact
  hashing and is not a 4B-loaded measurement or proof of a sustainable envelope.

### Operator handoff and exact next work

The owner's directive section 92 says: “PREPARE EVERYTHING FIRST. Then STOP.”
This handoff is for the required manual application close, not a new model or
deployment approval. Save work and close nonessential apps manually, including
Codex/ChatGPT, browsers and IDEs. Keep Docker Desktop, required WSL and one
PowerShell terminal. Wait 20–30 seconds, then run once:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\sheik\Workspace\Office\Projects\NexusAI\scripts\nxb21-english-functional-qualification\run_current4b_runtime_characterization_v2.ps1"
```

Freeze SHA-256: `ca96f1b0fe3d3cff4d5bc223c7273924f883de45dc1ccae707a970dd2d0cb326`.
No compilation, downloads, semantic corpus dispatch, service recreation,
retained processing or application termination occurs. The runner preserves the
existing governed clean-page-cache recovery before preload/reload. Its exclusive
lock prevents a second dispatch. Receipt and NDJSON are written under
`local-acceptance-models/nxb21-current4b-runtime-characterization-v2/`.

`CURRENT_MODEL_DECISION=PENDING_THIS_ONE_CHARACTERIZATION`.
If usable, retain the 4B candidate for a fresh bounded English functional gate
covering planner, synthesis and help. If unusable, research at most three current
primary-source small-model candidates and request one exact artifact approval.
Never rerun r1/r2/Decision8/8B corpora. Continue source algebra and document
retrieval work; new controls require actual executors and independent oracles.

`TOP_P1_GAPS=MODEL_RUNTIME_AND_ENGLISH_FLOW;COMPOSABLE_ALGEBRA;ALL_FAMILY_ACCEPTANCE;LIVE_ACTIVATION`.
The shortest legitimate D path is usable model or truthful resource fallback,
fresh English routing/planning/synthesis/follow-up proof, remaining bounded
algebra work, then separately approved sealed activation and real flow acceptance.
The actual next roadmap phases are **NX-B2.1E proof framework → F family
certification → G Ask UX → H live demo**. None is marked completed here.

`APPROVAL_REQUIRED=NONE_FOR_COMPLETED_SOURCE_WORK;MANUAL_OPERATOR_RUN_PENDING`.
New artifacts, retained mutation and production activation retain their explicit
approval boundaries. UI refinement and broad family completion remain open.

## Current-4B consumed-r2 resource adjudication checkpoint

Both fresh English work packages are consumed and must not be rerun. R1 closed as `INCOMPLETE_RESOURCE_RUNTIME_FAILURE` after one attempted request. The final stable-state R2 run passed model load, loaded-RAM, three generic transport calls, and its post-soak floor, then crossed below 4 GiB while the first full-registry semantic request was still in flight. R2 completed zero semantic, router, or synthesis cases, so current-4B semantic quality remains `INSUFFICIENT_EVIDENCE`, not failed. Retained tuple, Activity, jobs, containers, and the accepted analyst UI were unchanged.

R2 admitted with only 0.177 GiB of minimum post-soak headroom. Available RAM declined from 4.160 GiB at the first qualification sample to 4.027 GiB and then 3.966 GiB over 46.584 seconds. The in-process evaluator peaked at only 22.137 MiB and stayed flat. Each semantic request contains 66 candidates and approximately 2,778–2,800 tokens. This supports observed host-RAM drift with the large first-request shape as the likely trigger, but does not establish KV/context accumulation, backend/API/vmmem growth, or a runtime leak because no request completed and R2 lacked per-call component telemetry.

The only authorized next live action is the disposable non-semantic resource-lifecycle soak. It runs 20 tiny generic calls in fixed batches `1,2,4,8,5` with owned-model unload/reload boundaries, followed by three non-forensic payload-shaped calls, while recording host, API-container, backend-RSS, vmmemWSL, and latency data around every request. Runner SHA-256 is `18f27c767982e4e078ad3280b69aa44f7af830a7d7642378e2a70e71df3c95a2`; freeze SHA-256 is `5e206207dc49737056481eb3f4c510d417e2540271340e075ba1dcc62a66c0c2`. It is frozen, statically validated, and not executed. R3 must not be created unless this soak passes and is adjudicated; model/profile changes remain unauthorized.

## Decision summary

The shortest safe path to an English-first conversational investigation product is to close the semantic-planning bottleneck before expanding adapters or changing the accepted UI. The source already contains 79 registered executable operations, parameterized/typed execution, deterministic Fact Packets, strict grounded narratives, citations, follow-up context, document/derived-text retrieval, and family specialists. The prior runtime language-assistance path, however, reduced unresolved language to 22 capability tuples and phrase-shaped candidate filtering.

This slice adds `forensics.semantic-operation-proposal/v1`: an English semantic chooser over every scope-compatible non-engineering operation. Its model schema contains only one server-enumerated decision. Tenant, user, case, evidence scope, identifiers, dates, filters, limits, tools, and execution remain server-owned. The 22-capability residual chooser remains a compatibility/fallback path. Source tests pass; no deployment, retained-data mutation, model load, qualification, or product certification was performed.

## Initial gap report

| Required field | Reconciled state |
|---|---|
| Current branch / commit | `codex/forensic-hybrid-checkpoint-20260723` / `40717b83510c08db25dc26b9d6674bf46db363ac` |
| Accepted live baseline | Running API image `nexusai/localai-forensic:conversation-ui-final-20260908`; sidecar image `828e4b03e029`; accepted premium UI preserved and untouched |
| Live service state | API healthy; forensic records API running; worker healthy; Postgres and NATS healthy |
| Current retained tuple | Read-only observation `68|68|81|22511|870|68`; Activity `353`; active jobs `0`. This is newer than the historical accepted `67|67|80|22510|863|66` / Activity `344`; its external delta attribution was not inferred. |
| Existing language/query architecture | Deterministic fast paths; 79 registered operations; 104 live discovery descriptors; 214 variant ledger; canonical parameterized SQL/JSONB filters; governed capability/execution plans; 22-capability residual model chooser; audited follow-up context |
| Existing model architecture | Deterministic truth policy; configured planner/synthesis model `qwen_qwen3-4b-instruct-2507`; local embeddings; separate ASR/face roles. Roles are logically separated even when one local chat model may serve planning and synthesis. |
| Existing grounded synthesis | `forensics.fact-packet/v1` → strict `forensics.narrative/v1`; exact-token/fact/citation validation; forbidden certainty/identity claims; deterministic fallback when the model is absent, malformed, slow, or ungrounded |
| Existing document/RAG | KB evidence search plus native document, image OCR, and timestamped audio/Roman-Urdu derived-text routes with bounded results and source locators; semantic KB embeddings are available. Scanned/table/document-format breadth remains incomplete. |
| Existing media | Image/face/OCR, audio metadata/ASR/Roman Urdu, and bounded video observations exist with explicit candidate/review limitations. Diarization, general video understanding, identity recognition, and several codecs/formats remain unqualified or unavailable. |
| Current agents | 16 manifests in source; 6 running specialist agents in the last reconciled runtime inventory. Agent selection is governed by capability/family contracts, not a substitute for missing operations. |
| Final Qwen3-8B disposition | Consumed final-local selection-12 attempt is `CLOSED_RESOURCE_INSUFFICIENT`. Preload was stable (~8.48–8.50 GiB); loaded available RAM fell below the frozen 4 GiB floor before semantic/HTTP completion. One model call was attempted; quality is `INSUFFICIENT_EVIDENCE`, not failed. Receipt SHA-256: `430f40db43950981d064f2e52b5cc96290f038f28c33e14771a6e34f4583f4db`. Do not rerun this corpus. |
| Principal foundational gap | The runtime could execute far more operations than open-ended English could select. It had no full-registry, meaning-based semantic operation proposal layer. |
| Implemented in this slice | Full-registry bounded semantic-operation selection; strict decision schema; scope projection; server fact binding; safe residual fallback; request-class separation; typed-plan measure/group assertions; executor-aware filter/sort validation; and focused plus complete package regression tests |
| Remaining product-critical gaps | Representative English model qualification for the new operation and assistant/tool-routing contracts; deployment/activation acceptance; arbitrary multi-step aggregate composition beyond registered operations; broader all-family real fixtures; UI live acceptance of planner/fallback/clarification states |

## Architecture after this slice

`question → deterministic safety/scope/fact extraction → full registered operation meaning snapshot → bounded model operation decision → server registry validation → server fact/parameter binding → existing governed plan/tool execution → Fact Packet → strict grounded narrative or deterministic fallback → enterprise response/UI`

The model cannot emit SQL, a tool name, a URL, an identifier, a date, a filter, a limit, or a scope in the new proposal contract. This removes the earlier dependence on memorized sentence shapes without transferring factual authority to the model.

## All-family functional matrix

The data counts below come from the last complete reconciliation inventory (2026-09-03), not from a new retained-data acceptance run. “Queryable” means a bounded route exists and the inventory had relevant data; it does not mean every listed format or semantic behavior is product-certified.

The complete 79-row human-readable operation matrix is in `reports/nxb21/nexusai-product-query-capability-matrix-20260909.md`; its machine authority and join rules are recorded in `reports/nxb21/product-query-capability-matrix-v1.json`.

| Family / formats | Ingestion and deterministic extraction | Model dependency | Queryability and provenance | Real-fixture / presentation state | Remaining gap | Priority |
|---|---|---|---|---|---|---|
| Plain text / TXT, MD, YAML, YML | Native text, hash, size, bounded passages | Embedding/retrieval; optional grounded narrative | Queryable; passage/source/version citations | 1 registered, 1 KB-ready, 1 passage artifact; cited evidence presentation | Retained multilingual/noisy-note retrieval, abstention, and citation acceptance | P1 |
| Access/security logs / LOG, TXT, CSV, JSONL, NDJSON; EVTX listed but unsupported | Access-log normalization, event/user/IP/path/time filters, failed-event operation | Optional explanation only | Queryable; canonical source rows | 1,000 indexed; table/timeline outputs | EVTX decoder, drift/late-arrival/redaction and incident-timeline goldens | P1 |
| CDR / CSV, JSON, JSONL, NDJSON, Parquet | Mature number roles, service/sentinel classification, counts, durations, contacts, time/cell/direction | Semantic planning and optional narrative | Queryable; certified critical operations and row/aggregate lineage | 5,000 indexed; rich table/timeline/ranking presentation | Provider packs, deeper time-valid tower semantics, broaden product certification | P1 |
| IPDR / CSV, JSON, JSONL, NDJSON, Parquet; PCAP/PCAPNG inventory only | IP/NAT/session/domain/protocol/volume/concurrency/timeline | Semantic planning and optional explanation | Queryable structured sessions with row provenance | 2,500 indexed; table/timeline/aggregate | Provider DNS/NAT packs, clock drift, capture-to-session adapter and goldens | P1 |
| ANPR / structured plus common raster formats | Exact sightings, camera activity/sequence, co-observation, timing, variants; optional image observation | Optional FastALPR/OCR for image-derived observations | Queryable; structured facts authoritative, image output review-required | 750 indexed; 281 artifacts; map/table/timeline descriptors | Deploy/qualify opt-in image worker, adverse-condition and camera-clock goldens | P1 |
| Subscriber/identity / structured and XLSX/TSV | Exact privacy-safe lookup, validity, device/SIM/service observations, conflicts/reuse | Optional grounded explanation | Queryable with masked sensitive fields and row provenance | 5 indexed; identity tables/timelines | Provider-format benchmarks and field-level authorization | P1 |
| Tower/location / structured, GeoJSON, XLSX/TSV | Exact site/sector lookup, history, coordinate audit, conflicts, time-aware CDR join | Optional bounded location explanation | Queryable; supplied coordinates/uncertainty and source rows retained | 5 indexed; geo/table/timeline descriptors | Provider drift, datum transforms, retained joins, map UI, specialist acceptance | P1 |
| Financial transactions / structured and XLSX; OFX/MT940 unsupported | Currency-separated exact counts/sums and lineage | Optional explanation | Queryable with complete bounded contribution lineage | 4 indexed; aggregate/table | Bank/wallet profiles, reconciliation, multi-currency, OFX/MT940 goldens | P1 |
| Generic tabular / CSV, JSON*, TSV, XLSX; several columnar formats only declared | Canonical rows, schema discovery, typed payload predicates, exists/range/sort/page | Planner can choose operation; values stay deterministic | Queryable; row provenance and preserved raw payload | 9 indexed; schema/table presentation | Schema drift, encoding, nested JSON, unknown columns, representative Parquet | P1 |
| Spreadsheets/columnar / TSV, XLSX bounded; XLS/ODS/Arrow/Feather/Avro/ORC incomplete | Read-only TSV/XLSX per-sheet mapping, formulas preserved but never executed | Optional explanation over normalized evidence | No retained data in last inventory; generic route only after normalization | No representative retained fixture | Multi-provider workbooks, hidden/merged/formula cases, ODS/columnar qualification | P1 |
| PDF/office/email/ebook | Native TXT/PDF/DOCX bounded text and passage lineage; breadth partial | Retrieval/embedding and optional grounded narrative | Queryable for completed native passages with citations | 2 registered, 2 KB-ready, 113 artifacts | Retained acceptance; scanned OCR, tables/geometry, attachments, more formats | P1 |
| Images/OCR | Metadata, fingerprints, bounded OCR/ANPR/face candidates, crop lineage | OCR/face/SigLIP roles where configured | Queryable over stored observations; candidates are not identity/source truth | 17 registered, 281 artifacts; review-focused grids | Retained image/OCR acceptance, Urdu text-image quality, HEIC/raw/SVG | P1 |
| Audio/STT | Metadata, source-time ASR segments, raw Urdu plus Roman derivative | ASR for new processing; retrieval/narrative after persistence | Queryable over persisted segments with time citations | 7 registered, 20 artifacts; transcript/timeline | Retained Roman Urdu acceptance; identifier speech M2; diarization unavailable | P1 |
| TTS artifacts | Registry and deterministic WAV metadata; generation policy not closed | TTS backend for generation only | Existing artifacts queryable; generation not advertised as ready | Shares 7 audio items / 20 artifacts in prior inventory | Consent/voice policy, pronunciation, intelligibility, latency, accessibility | P2 |
| Transcripts/subtitles / TXT, JSON; SRT/VTT/ASS pending | Parent/source inventory; cue parser not complete | Retrieval after cue indexing | Only existing text/ASR-derived material queryable | 1 registered / 1 artifact in prior inventory | Cue parsing, parent/time citations, real subtitle fixtures | P2 |
| Video | Metadata, ffprobe, bounded frames and persisted source-time observations | Optional image/ANPR roles; no general event understanding | Queryable metadata/timeline only for retained authorized evidence | 2 registered, 397 artifacts; timeline descriptors | Real-codec/corrupt/long fixtures; ASR, tracking/dedup, governed events | P1 |
| Network/system captures / PCAP, PCAPNG, CAP, EVTX | Safe inventory foundation; no accepted session/event decoder | None should infer missing sessions | No data; analytical query unavailable | No retained fixture/presentation acceptance | Real capture fixtures, bounded protocol/session derivatives, EVTX separately | P2 |
| Databases/SQL dumps / SQLite, DB, SQL | Read-only inventory foundation; arbitrary SQL prohibited | Planner must route to inventory/clarification only | No data; row query adapter unavailable | No retained fixture | Encoding/WAL/schema/corruption goldens and bounded typed query adapter | P2 |
| Archives/bundles / ZIP, TAR; other formats pending | Safe inventory foundation; recursive extraction not accepted | No model authority over extraction | No data; inventory only when present | No retained fixture | ZIP64/PAX/GNU, bombs/recursion safety, hashes, child registration | P2 |
| Unknown/mixed | Hash, size, signature sampling, duplicate/manual disposition | Classification explanation only after deterministic inspection | Manual review; never silently treated as supported | No representative accepted fixture | Polyglot/corrupt/renamed-extension fixtures and operator workflow | P1 safety |

## Query capability reconciliation

- Registered executable operations: 79.
- Certification coverage: 79/79 have correctness contracts; 5 are certified, 74 bounded-uncertified.
- Exposure: 5 queryable, 63 limited, 11 engineering-only.
- New workspace semantic snapshot: 66 scope-compatible non-engineering operations; evidence-required operations appear only when an authorized selected-evidence scope exists.
- Existing thin residual references: 22. They remain a fallback and are no longer the ceiling for English semantic operation selection.
- Typed plans now fail closed when `measures` or `group_by` differ from the selected registered operation, when an analytical filter/sort would be ignored, when more than one sort is supplied, or when a canonical sort field/direction or payload operator is outside its allowlist. `cdr.frequent_contacts` direction, retained-video source-time filters, and canonical/generic payload filters are lowered only to executors that consume them.
- The bounded algebra currently covers server-scoped exact lookup and time range, allowlisted `FILTER`, one deterministic `SORT`, bounded `TOP_K`, and cited `SOURCE_ROWS`; registered operations supply their declared `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`/`GROUP`/timeline semantics. An arbitrary multi-step aggregate AST (`PROJECT`, ad-hoc `GROUP`, chained `COMPARE`, etc.) remains unexposed and uncertified.

## Grounded synthesis and general assistant contracts

Grounded investigation synthesis is operational in source: deterministic execution is normalized to `forensics.fact-packet/v1`; `forensics.narrative/v1` accepts only existing fact/citation IDs, verifies exact tokens against referenced facts, rejects unknown limitations/follow-ups and prohibited certainty, and falls back deterministically on any model failure.

A distinct general product/help/relevant-knowledge route is now wired at the agent boundary. Only explicit or provably recognized forensic operations take the deterministic fast path. Greetings, product help, relevant general knowledge, and unfamiliar language reach the configured assistant/tool router. Its policy requires governed tools for investigation, follow-up, evidence/RAG, case-state, processing, model-availability, and computed-finding claims; direct general answers may not assert case facts. Focused routing tests and the complete agents suite pass. Model behavior across these request classes still requires representative qualification.

## Ordered closure path

1. Qualify the new English semantic-operation contract against a fresh paraphrase corpus covering all exposed operations, ambiguity, unsupported requests, and adversarial scope/fact injection. Do not reuse consumed D corpora.
2. Qualify the source-validated request-class/general-assistant contract across investigation, follow-up, RAG, product help, greeting, relevant knowledge, clarification, unsupported, and adversarial fact-claim cases.
3. Extend the source-validated typed algebra only for legitimate questions no registered operation can express. Add independently-oracled `PROJECT`, ad-hoc `GROUP`, `TIME_BUCKET`, and `COMPARE` slices before exposing any of them to the semantic planner.
4. Close P1 real-fixture gaps family by family, then run synchronized API/UI presentation acceptance.
5. With explicit approval, build/deploy the source candidate and run live regression plus retained read-only verification. Do not claim product certification from source tests.

## Active issue register

Current source details and acceptance boundaries are in the post-proof checkpoint above. New issue NX-PFC-P1-006: bounded canonical grouping needs scoped SQL/API and presentation acceptance before product qualification. Evidence: independent count/null/tie/lineage source oracle; decision: implement bounded slice, retain D OPEN; acceptance: complete positive/zero/overflow and source-boundary results through the governed API.


| ID | Priority | Open gap |
|---|---|---|
| NX-PFC-P1-001 | P1 | Resource track CLOSED/PASS; 45-case English proof consumed/failed. Generic planner and synthesis source fixes require fresh model qualification; no rerun of historical evidence. |
| NX-PFC-P1-002 | P1 | Latest router proposals pass 8/8; this is not tool execution acceptance. Help now receives scoped registry grounding and explicit confidence policy; model compliance and live tool paths remain unqualified. |
| NX-PFC-P1-003 | P1 | Partial source closure: existing-operation algebra now fails closed and safely lowers supported filters/sorts/top-K/source rows; arbitrary independently-oracled aggregate composition remains open |
| NX-PFC-P1-004 | P1 | All-family representative real-fixture and negative/zero/provenance acceptance |
| NX-PFC-P1-005 | P1 | Build/deploy and live API/UI planner, clarification, fallback, citation, and follow-up acceptance; explicit approval required |
| NX-PFC-P2-001 | P2 | Capture, database, archive, subtitle, and TTS foundation maturation |

## Verification performed

- Focused Go tests cover full-registry projection, semantic selection, deterministic fact binding, scope preservation, strict unknown/trailing/fact-bearing rejection, selected-evidence family scoping, residual compatibility, typed-plan assertion mismatches, ignored analytical controls, and canonical filter/sort allowlists. The complete forensic-records package passes (527 specs passed, 33 intentionally skipped).
- Agent routing tests prove that product help/general/unfamiliar language is not coerced into case analytics while known operations retain the deterministic fast path; the complete agents package passes. The agent-pool suite passed 58/59, with one unrelated repeatable Windows file-lock race in `agent_jobs_test.go`.
- The consumed R1 run made one model request. The consumed R2 run passed basic runtime and generic transport, then was canceled by its RAM guard during the first semantic request with zero cases completed. The new disposable soak has been prepared and validated without live inference. No image rebuild, deployment, data migration, upload, reprocessing, retained mutation, or UI change was performed by this adjudication slice.
