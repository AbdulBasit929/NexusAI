# NX-B2.1D dynamic planner and operation-verification alignment — model gate outcome

Status: **D OPEN — Q4 DEVELOPMENT 31/32 FAIL; REMEDIATION REQUIRED; ACTIVATION BLOCKED**.

**2026-09-04 operator repair:** A third standalone invocation passed the 7.37
GiB RAM gate and stopped in read-only baseline SQL because `retrying` is not a
member of `forensic_ingest_status`. The query now uses the established predicate
`status NOT IN ('completed','dead_letter')`. The stop occurred before Qwen load,
inference, contract freeze, or holdout access. The full evaluation, activation,
and rollback bundle then passed 22 preparation tests, including all-service HTTP
health, exact candidate/restored image checks, protected-service parity, retained
tuple and Activity parity, zero jobs, and unloaded-model checks. At that point
the holdout was still unconsumed and D status was unchanged.

The fourth invocation passed the 7.321 GiB initial RAM gate and froze the
evaluation contract, then stopped at the 3.41 GiB immediate pre-load gate after
hashing the 4.28 GB artifact populated WSL clean file cache. It remained before
`HOLDOUT_CONSUMPTION=BEGIN`, model load, and inference. A fifth invocation passed
initially at 8.099 GiB but stopped at 4.228 GiB because harmless 4–56 KiB Dirty
pages made the recovery rule too strict while Writeback remained zero. Recovery
now drops only reclaimable clean cache when Writeback is zero, preserves dirty
pages, performs no `sync`, Docker cleanup, or process stop, and waits up to 60
seconds for Windows to observe release. The bundle now passes 26 preparation
tests.

The final end-to-end preparation dry run passed at 05:36:04Z with 6.411 GiB
initial RAM. Exact baseline and artifact verification passed; recovery ran at
Dirty 8 KiB and Writeback zero; Windows RAM recovered from a transient 2.106 GiB
minimum to 6.166 GiB before the identical immediate pre-load gate passed. No
model or holdout was accessed. No Docker or Windows restart is required.

The authorized one-shot run began at 05:39:38Z. It froze manifest SHA-256
`9e9507fb174cef2f356c452c793e2905329af5de19c2791d8ae0bb46332e42bb`
and holdout SHA-256
`eb102cdd1f1fa1d9da02c5dce4c8c9e216a523ba78d4468ce9802f880170283f`,
then consumed 81/168 cases. Eighty responses were `malformed_proposal`; case 81
timed out. Ginkgo's separate one-hour suite timeout ended the process before any
of 80 critical cases. The live profile mismatched the frozen configuration:
LocalAI reported effective context 4096 versus the frozen 8192. The live YAML
default temperature was 0.6, but the request set temperature zero.
`function.grammar.disable` applies to generated function-call grammar, while the
requested `response_format` schema uses a separate LocalAI path. No
grammar-generation errors were logged, and the original evaluator did not
preserve malformed raw completions, so their exact cause remains unresolved.
The run is therefore
`INVALID_ABORTED_CONFIGURATION_MISMATCH_AND_SUITE_TIMEOUT`. The deployed profile
is insufficient; intrinsic Qwen base-model suitability is not classifiable from
this invalid run. The holdout is consumed and cannot be retried or used for
tuning.

The admitted Q4_K_M corrective candidate is now present under ignored local
acceptance storage. Its artifact SHA-256 is
`2fde00ce69dd4899c70d020845e2638353015bba0fdf161b3eb965f2bca4464e`;
the isolated development profile remains
`30d3bcd8c92684e191277dad992354323b7e46c7d30d3c69149010e90f117129`.
The original 12-case smoke reported 9/12 because Windows PowerShell
`Invoke-WebRequest.Content` decoded three correct Urdu responses as mojibake.
Raw-byte replay with strict UTF-8 proved all three, producing an adjudicated
12/12 PASS receipt with SHA-256
`75ffdda521a33af075b4e03f95270cf32da64031968306e650774b87eeadc12a`.
The original 9/3 summary remains preserved as transport-defect evidence.

Reusable byte-safe development tooling now serializes request JSON to UTF-8
without BOM, sends byte-oriented HTTP bodies, captures response bytes before
strict UTF-8 decoding, and only then parses the OpenAI envelope and planner
object. A fresh 32-case development corpus is frozen at SHA-256
`01647f6240d9bcbabbc758c153ba7eea454458efb9348a57e22f6a0a1ed63183`.
It has eight English, eight Urdu-script, eight Roman Urdu and eight mixed cases;
source validation passes. The first standalone invocation stopped before its
first request file or HTTP call because Windows PowerShell 5.1 expanded a
depth-50 schema serialization to an observed 8,952,647,680-byte working set.
That invalid run is preserved and serialization is bounded at depth 12. A second
invocation also stopped before request-file creation or HTTP and proved that
Windows PowerShell 5.1 default decoding of BOM-less UTF-8 evaluator inputs was
the remaining CPU-loop trigger. All evaluator text reads now use strict UTF-8,
an existing input BOM is stripped, outgoing request bytes remain BOM-free, and
transport stages are printed. The exact Windows PowerShell 5.1 request now
serializes in 35 ms at 3,282 bytes; 14/14 source tests pass. The corrected
standalone run executed all 32 cases: 31 passed and `dev32-en-002` failed because
the model split atomic plate entity `ISB-5907` into `ISB-590` and `5907`. The
development receipt SHA-256 is
`bf73696f2fc89ee02d26027818ee9d0b838926d1cbaf67a7276829a9b1321132`.
The second invalid
abort receipt SHA-256 is
`1ddb04bbc255e02a0e06c8a20eb85a3e0e517b2d3ecabec2e426d74d6be8ff0d`.

## Verified Starting State

C baseline verified across 443 files. Accepted receipt 20260903T063719719Z was not rerun. Retained C exception remains untouched.

## A/B1/C Evidence Reused

A inventories, B1 semantics and frozen proof, and all 22 C references/readiness boundaries remain authoritative.

## Current Live vs Source Candidate

Live remains the earlier catalog. D is source-only and undeployed.

## C Digest Verification

PASS: 9985d71d894631e6c730c6c7ffa9caf85215427ca076e12e8bb0edc545a91c02

## Operation Inventory

79 executable operations remain registered.

## 79-Operation Verification Map

All 79 are mapped to certification requirements; none is newly certified.

## 22 Query Capabilities

The model-facing vocabulary is constrained to 22 C references; the server derives operations.

## 214 Query Variants

214/214 classified COMPATIBLE_PLAN_PARITY at source typed-plan level.

## Dynamic Planner Architecture

Deterministic structure precedes a strict capability proposal and server validation. Authorization and execution remain server-owned.

## Deterministic Fast Paths

Identifiers, quoted literals, explicit scope, directions, calendar/source times, and limits remain deterministic.

## Capability Filtering

Candidates are filtered by scope, selected family, explicit entity shape and intent; ambiguous questions retain scope-compatible candidates.

## Planner Schema

Strict JSON rejects unknown fields. Capability, semantic, exact spans, identifiers, scope, time and top-k are validated.

## Current LLM Configuration

qwen_qwen3-4b-instruct-2507; llama-cpp; Q8_0; 4,280,405,216-byte GGUF; CPU, 8 threads, gpu_layers 0, configured 8192 context.

## Current LLM Fitness Baseline

Not measured: installed Qwen is not loaded and only 3,372,187,648 host bytes were free at inspection.

## English Results

65 frozen cases; actual-model result pending.

## Urdu Results

37 frozen cases; actual-model result pending.

## Roman Urdu Results

36 frozen cases; actual-model result pending.

## Mixed-Language Results

30 frozen cases; actual-model result pending.

## Literal Preservation

PASS_SOURCE validator: exact user substring required.

## Identifier Preservation

PASS_SOURCE validator: exact query spelling and deterministic extraction required.

## Entity Extraction

Existing deterministic target extraction retained; model cannot add identifiers.

## Query Semantic Selection

Restricted to the selected C capability semantic allowlist.

## Selected Scope

PASS_SOURCE; selected-only references require authoritative evidence ID.

## Workspace Scope

PASS_SOURCE; explicit workspace intent clears evidence selection only inside the authorized case.

## Scope Switching

PASS_SOURCE for explicit selected/workspace conflict and workspace switch.

## Calendar Time

Calendar bounds must match deterministic extraction.

## Source Time

Recording bounds have a separate parser and semantic.

## Follow-Ups

Historical governed follow-ups pass typed-plan parity; 12 new transitions are frozen for model evaluation.

## Stale Context Isolation

PASS_SOURCE: explicit new plate/family does not inherit transcript context.

## Aggregate Planning

Capability semantics constrain aggregate routes.

## Top-K Planning

PASS_SOURCE: explicit integer required and bounded by maxHybridLimit.

## Document Planning

Literal and semantic references remain distinct.

## OCR Planning

OCR literal search retains B1 semantics.

## Transcript Planning

Raw/Roman and source-time semantics remain explicit.

## ANPR Planning

Exact plate is not silently similarity; selected image/video boundaries remain.

## Image Similarity Planning

Direct descriptor retained as similarity, not identity.

## Face Candidate Planning

Candidate similarity retained; identity requests are unavailable.

## Structured Planning

Typed structured operations are preferred over generic retrieval through capability candidates.

## Cross-Family Composition

Existing three-step budget retained; phone/plate remains bounded.

## Unavailable Capability Handling

PASS_SOURCE fail-closed guard.

## TTS Intent Handling

PASS_SOURCE; generation/read-aloud remains unavailable.

## Unsupported Format Handling

PASS_SOURCE for C-unqualified EVTX/OFX/MT940/HEIC/SRT/VTT/SVG boundaries.

## 214-Variant Regression

214 COMPATIBLE_PLAN_PARITY; this is question-to-plan proof, not result proof.

## Independent Unseen Corpus

168 cases frozen before evaluation; expected plans are manually authored and independent of planner/model output.

## Critical Planner Safety

Source validator tests pass. Actual-model critical safety is
`NOT_REACHED_UNMEASURED`: zero of the 80 critical holdout cases ran. D cannot
pass without 100% measured critical safety on a new independent holdout.

## Planner Failure Modes

Malformed, unknown, altered, widened, unbounded, SQL/shell/URL proposals fail closed.

## Model Proposal Validation

PASS_SOURCE with mocked transport; mocks are not language fitness.

## Model Performance

Observed partial run: 0/81 passed and 0/81 schema-valid; median 42.814 seconds,
p95 58.108 seconds, maximum 60.003 seconds, and 3,584.769 seconds summed case
latency. Host available RAM reached 1.412 GiB. The API container reached
5,951.488 MiB and 810.52% CPU. These are invalid-run diagnostics, not a
production SLA.

## Current Model Suitability Decision

Current deployed Q8 profile: **INSUFFICIENT**. Intrinsic Qwen3-4B-Instruct-2507
base model: **NOT_CLASSIFIABLE_FROM_INVALID_RUN**.

## Better Model Required?

Yes at the admission/profile level. Prefer a memory-lighter Q4_K_M build of the
same Qwen base for controlled development qualification; retain official
Ministral 3 3B Q4_K_M as the structured-output challenger. No candidate was
downloaded.

## Model Admission Report

`reports/nxb21/d-model-admission-required-v1.md` and paired JSON record exact
candidate revisions, artifact sizes/hashes, licenses, compatibility, resource
estimates, language limitations, rollback and the next approval boundary.

## Operation Verification / Certification Alignment

All operations include oracle and positive/zero/scope/version/citation/navigation/Activity/UI/performance/security requirements.

## E Framework Requirements

D artifacts are machine-readable inputs for certification schema/oracle/proof-state work. E must not promote proof from planner parity.

## F Live Certification Requirements

F requires deployed behavior, current authorized targets, independent result oracles, citations, navigation and Activity. Activation is required before F.

## Source Tests

D gate 12/12 PASS; APF35 compatibility PASS; 214 classification runner PASS.

## Integration Tests

Capability guard/integration slice 53/53 PASS. Full forensic package: 457 passed, 27 isolated/optional/environment skips, plus legacy tests. Agent package: 145 passed, one optional skip, plus legacy tests. Clean exits.

## Security

No arbitrary SQL/shell/URL, unknown operation, scope widening or invented identifier is admitted.

## Anti-Hardcode

Synthetic identifiers only; no retained target or analytical answer in production source.

## Privacy

Unredacted logs and runtime inspection remain in ignored local acceptance storage.

## Git Diff Check

PASS; HEAD/staged state unchanged; existing dirty work preserved.

## Files Changed

22 D implementation/proof-input paths plus this manifest/report and governance.

## C Baseline Digest

9985d71d894631e6c730c6c7ffa9caf85215427ca076e12e8bb0edc545a91c02

## New D Source Digest

34c69067628a58373d471aa8e75d7a17a2c4a7f789b521f62c5baa38f8a5478d

## Runtime Impact

The authorized run loaded and then safely unloaded Qwen. No build, restart,
deployment or activation occurred. The evaluator touched no normal Ask path.

## Retained Impact

No D retained query/write; C tuple exception was left untouched.

## Database Impact

No migration or D test database.

## Model Impact

Qwen remains installed and is unloaded after the one-shot inference run. Only
`qwen3-embedding-0.6b` remains loaded. No model artifact or profile changed and
no candidate was downloaded.

## Future Minimal Service Scope

Consolidated B1+C+D remains api + forensic-records-api; worker/PostgreSQL/NATS excluded.

## Deployment Dependency Decision

No deployment for source/model evaluation. Consolidated activation remains required before F.

## Can E Proceed Source-Only?

No as an accepted program phase because D failed. E schema drafting may continue
only as non-promotional preparation; D must pass a valid new independent model
evaluation first.

## Activation Required Before F?

Yes. F cannot certify old live B/C/D behavior.

## Open P0

0 observed in bounded D source work.

## Open D P1

Two linked gates: correct and qualify a resource-safe structured-output
planner/profile on development data, then pass a newly frozen independent
multilingual/critical holdout. The consumed corpus is retired.

## Remaining B2.1 P1/P2

B1/C live deployment remains pending; D admission/requalification and a new
independent evaluation are required; E/F certification and G presentation remain
future work.

## D Exit Decision

**OPEN — NOT YET QUALIFIED.** The Q8 candidate evaluation remains a terminal
failure. The Q4 candidate has a 12/12 adjudicated development smoke result, but
the completed 32-case development run is 31/32 and therefore FAIL. A corrected,
explicitly refrozen remediation candidate and later new independent qualification
holdout are required.

## Exact Next Phase

Remain in NX-B2.1D model admission and development requalification; do not start
E as an accepted phase.

## Exact Next Action

Start a separate bounded Q4 remediation work item. Diagnose the atomic plate
entity split using this development evidence; if prompt/schema/profile changes,
assign a new frozen candidate identity and use fresh development questions.
Do not rerun the consumed Q8 holdout, treat this development corpus as an
independent holdout, freeze final qualification evidence, or activate B1+C+D.

## Current proven markers

```text
CSourceDigestVerified=true
ExecutableOperationCount=79
All79OperationsAccounted=true
QueryCapabilityCount=22
QueryVariantCount=214
Historical214VariantsClassified=PASS
DeploymentPerformed=false
DatabaseMigration=false
RetainedEvidenceMutatedByD=false
RetainedActivityMutated=false
ModelsChanged=false
ProductCertificationPerformed=false
NXB2ActivationPerformed=false
ModelEvaluationAuthorized=true
AuthorizedModel=qwen_qwen3-4b-instruct-2507
RAMGateRequiredGiB=6
RAMGatePassed=true
RuntimeBaselinePreserved=true
DSourceDigestVerified=true
FrozenHoldoutCases=168
ActualModelEvaluationPerformed=true
ActualModelCasesCompleted=81
CriticalPlannerSafety=NOT_REACHED_UNMEASURED
EnglishPlannerFitness=FAIL_THRESHOLD_UNREACHABLE
UrduPlannerFitness=FAIL_THRESHOLD_UNREACHABLE
RomanUrduPlannerFitness=FAIL_THRESHOLD_UNREACHABLE
MixedPlannerFitness=FAIL_THRESHOLD_UNREACHABLE
StructuredOutputValidity=0_OF_81
PlannerLatency=MEDIAN_42814MS_P95_58108MS
PeakModelMemory=API_CONTAINER_5951.488_MIB
CurrentPlannerModelSuitability=INSUFFICIENT_PROFILE_BASE_MODEL_NOT_CLASSIFIABLE
HoldoutConsumed=true
HoldoutMayBeUsedForTuning=false
Q8EvaluationDecision=FAIL_TERMINAL_HOLDOUT_CONSUMED
Q8ModelUnloadedAfterEvaluation=true
Q4Candidate=qwen3-4b-instruct-2507-q4km-nxb21d-dev
Q4DevelopmentSmokeAdjudicated=12_OF_12_PASS
ByteSafeEvaluator=SOURCE_VALIDATED
DevelopmentCorpus32=EXECUTED_31_PASS_1_FAIL
Live32CaseEvaluation=DEVELOPMENT_FAIL
Q4Qualification=NOT_YET
All79OperationsAccounted=true
DStatus=OPEN
DExitDecision=PENDING_Q4_REMEDIATION_AND_NEW_INDEPENDENT_HOLDOUT
ECanProceedSourceOnly=false_with_reason
ActivationRequiredBeforeF=true
```

**Offline operator terminal state.** The consumed run is preserved at
`local-acceptance-models/nxb21-d/offline-runs/evaluation-20260904T053938055Z`.
The public receipt SHA-256 is
`eba308ad2a948d615bafa48fb54ef959c3fb303ec278739c3c38bc1d2c33dba5`.
Post-unload verification found all five exact containers healthy and unchanged,
zero active jobs, retained tuple `64|64|77|22507|828|61`, Activity count 303
with no new evaluation entry, no migration, and no volume/model/deployment
change. The hardened bundle now passes 29 checks, blocks consumed-holdout reuse,
uses `/system` for model-state verification, sets both Go and Ginkgo four-hour
timeouts, enables verbose streaming, and saves a partial checkpoint after every
case. Activation remains separate and rejects this failed receipt.

All 22 capability rows and all 79 operation rows are accounted for in the new runbooks. This is not model fitness or analytical certification.


## 2026-09-05 remediation refreeze addendum

The completed Q4 32-case development receipt is preserved at 31/32 FAIL
(SHA-256 `bf73696f2fc89ee02d26027818ee9d0b838926d1cbaf67a7276829a9b1321132`).
The only miss split `ISB-5907` into two entities while preserving every other
typed field.

Candidate `nxb21d-q4-atomic-identifier-authority-r1` refreezes that unchanged
model/profile/prompt/schema tuple with source-bound exact identifier authority
at the shared typed validation boundary. Candidate identity SHA-256 is
`1ccc03ab44a25cb20ec28053b898c2f975b6373829303e8265833164a89e2ccc`.
The production boundary records the model proposal and deterministically
reconciles exactly one `image.plate` / `EXACT_VALUE` source span into a
single literal/entity; absent or multiple spans fail closed to clarification.
Phrase planning is unaffected.

The fresh 16-case development corpus SHA-256 is
`937f44c241dce1abc6f0ca1781b865b8dc0d49c89ed804411c1ac58e6a329cec`.
It is frozen but not executed. Source validation passes 25 focused Go specs,
95 remediation assertions, 14 prior byte-safe checks and the corpus validator.
D remains OPEN, no qualification holdout is frozen, and activation is BLOCKED.


## 2026-09-05 remediation development result

The standalone 16-case run completed with receipt SHA-256
`83ac55c7cd9627f0d37e21f33c556245bc6e7e62f35fb267d66f95b6872c23f3`.
All transport, UTF-8, schema and runtime-integrity gates passed.

The exact-identifier slice passed 8/8 final typed plans. Two model proposals
omitted their plate entity; deterministic authority disclosed and safely
reconciled both. Phrase regression controls passed 5/8: one Urdu document
literal contained Cyrillic homoglyph substitutions, and Urdu plus Roman Urdu
workspace transcript requests omitted the literal and falsely requested
clarification. Final aggregate is 13/16 FAIL.

This corpus is now historical development evidence and must not be rerun.
Candidate `nxb21d-q4-atomic-identifier-authority-r1` does not advance to
qualification. D remains OPEN, the independent holdout is NOT_FROZEN, and
activation remains BLOCKED.

## 2026-09-05 phrase-literal authority refreeze

The atomic remediation run remains immutable at 13/16 final PASS, with exact
plates 8/8 and phrase controls 5/8. Its receipt SHA-256 is
`83ac55c7cd9627f0d37e21f33c556245bc6e7e62f35fb267d66f95b6872c23f3`.
That corpus is historical development evidence and will not be rerun.

Candidate `nxb21d-q4-phrase-literal-authority-r1` has identity SHA-256
`0129bd4ab0d660573726c744f6ba9874a9a15b9343a87cc5b37a264dcadf675e`.
It derives deterministic phrase authority only from one existing-parser quoted
source span plus matching family context. It reconciles final literal, typed
text query and clarification while retaining the raw model plan, mismatch
reasons, normalization and exact codepoint diagnostics. Ambiguous,
contradictory and unquoted inputs receive no silent phrase extraction. Exact
plate authority remains unchanged.

The standalone 20-case corpus is frozen at SHA-256
`7c273c3b45fd3103234f098cd38bce84348c8cfff27b243f6c6987a5dd1555e6`
and has not been executed. Source tests pass: 35 focused planner specs, the full
forensic-records package, 133 phrase-framework assertions plus corpus
validation, and 14 prior byte-safe checks. Candidate state is
**REFROZEN_SOURCE_VALIDATED_DEVELOPMENT_NOT_EXECUTED**. D is OPEN, no
qualification holdout is frozen, E is not accepted, and activation is BLOCKED.

## 2026-09-05 explicit-scope remediation refreeze

The phrase run completed at 19/20 with receipt SHA-256
`02a7981aa4d4918255ef056e0008250baa7fd265e72db5e10cf510da934123a0`.
Phrase literal preservation passed 16/16 and plate controls passed 4/4. The one
failure was scope only: `tamam transcript files` was planned as selected rather
than workspace while every other typed field remained exact.

The separate `nxb21d-q4-explicit-scope-authority-r1` candidate has identity
SHA-256
`ba17c5c688eaa567ce2d006d8a0a80948a4e01e1b3c62f0765c84aeaa4d9d23a`.
API and agent routing now share bounded multilingual explicit-scope recognition
outside quoted literal spans. A fresh 16-case corpus is frozen at SHA-256
`e034bb3683f23c1aeb7488232a7337c56d5e9eb7cb69f9f280698af3412b1b50`
and has not run. D remains OPEN, qualification is NOT_FROZEN, E is not accepted,
and activation is BLOCKED.

## 2026-09-05 explicit-scope runner startup hardening

The first two standalone attempts aborted at LocalAI health preflight with zero
cases executed. Preserved receipt SHA-256 values are
`b70774be1e521bc1dc33c13df155409868d180f2b6f2c365eafbfb8c40d4f7f8` and
`af4f7dffaae8b528f8f2d1951cd37d57a33c557d344cf7633d3ae182b884f44a`.
The exact container became healthy after its normal startup interval, so the
frozen corpus remains fresh.

The operator runner now performs a bounded 180-second health wait against the
existing container before enforcing the frozen ID, image and restart-count
contract. It never restarts or recreates the runtime. The updated framework
passes 122 assertions; corpus validation and candidate identity remain
unchanged. The next authorized action is one standalone execution of the same
16-case development corpus. D remains OPEN and activation remains BLOCKED.

`run-20260905T080101493Z` then confirmed the health correction and aborted at
the unchanged 6 GiB preload RAM gate before sending any case. Its receipt is
preserved at SHA-256
`a6eea5c45bed615d569dbab89a4302253d0f39ff17d8765fa996c24d5c9fc5a3`.
The runner now invokes the proven clean-page-cache recovery and waits up to 180
seconds for RAM; it does not lower the floor or stop/restart runtime services.
Framework validation passes 125 assertions and the corpus remains unconsumed.

`run-20260905T080508522Z` subsequently completed case 1 at model/final PASS,
then stopped before case 2 at 3.525 GiB versus the unchanged 4 GiB loaded-model
floor. The partial aggregate is sealed at SHA-256
`d61e6ffb44513f3448713a0cbab69db126f597fa9426c620b88a73f955a7fa33`.
The runner now reconstructs only a verified consumed prefix from sidecar-bound
aggregate receipts and hash-checked raw evidence, skips that prefix, and waits
safely for RAM before every remaining case. Framework validation passes 135
assertions; cases 2 through 16 are the only inference still authorized.

The next zero-case attempt, `run-20260905T081329300Z`, exposed an unnecessary
recursive traversal in resume discovery. Receipt SHA-256 is
`d5f4c307dfd1f45c12f03040100f16fa9ad084fa7048313147fe78fc7eb86e62`.
Discovery now reads only each run's aggregate receipt and verifies referenced
evidence separately. The sealed case-1 prefix still verifies, and the corrected
framework passes 137 assertions.

## 2026-09-05 development exit pass and final Q4 qualification freeze

The resumed explicit-scope development run `run-20260905T081643297Z` completed
at 16/16 raw model and 16/16 final typed PASS. No deterministic correction was
needed. Its immutable receipt SHA-256 is
`921229a78719a5594271b4ede2fd5aadabc6f3a52d50c3893e4dd5f5badfafe1`.

Candidate `nxb21d-q4-final-qualification-r1` is now frozen at identity SHA-256
`11c7ce2a5c0ed1d6bb83a8d2a9b26274dbcedf6c96b014a4c7e0404160c26371`.
The independent 168-case holdout SHA-256 is
`aaa8517983ea4523e235c4b3de31e5562bfaeada21ef24a7b23065a567d31d13`.
It has 168 unique questions, zero overlap with prior questions or oracle values,
80 critical cases, the frozen 65/37/36/30 language distribution, and coverage
of all 22 capability references. The deterministic source oracle passes every
case and the qualification/activation framework passes 537 assertions.

Preparation run `qualification-20260905T152339897Z` verified the exact runtime,
artifact and profile baseline without model mutation or holdout consumption;
receipt SHA-256 is
`01d28017aab8755147cd848dc4c1546c9e12f59b68a5810a1f024246406592cd`.
The holdout is FROZEN_NOT_EXECUTED. The Q4-specific activation and rollback
scripts are sealed against the same source and runtime baseline. D remains OPEN
pending its one-shot result; E is not accepted and activation remains blocked
pending a reviewed SUITABLE qualification receipt and explicit owner approval.
