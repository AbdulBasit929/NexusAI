# NexusAI team-lead current brief

## 2026-08-17 — APF-3.1 through APF-3.3 source accepted

The query-intelligence foundation now wraps, rather than replaces, the accepted
forensic runtime. Typed query understanding feeds one workspace-aware resolved
capability, one validated read-only plan and the existing deterministic/KB/
hybrid executor. The 65-operation executable catalog has complete descriptor
and reverse-lookup parity. The broader platform registry truthfully retains its
non-query surface and therefore contains 80 descriptors.

Availability and security are explicit: missing or processing data, missing
runtime/model, insufficient maturity, unauthorized or widened scope, unknown
or unsafe operation IDs, bad required parameters, multi-step plans and disabled
citations cannot execute. Existing English, Urdu and Roman Urdu routing remains
compatible. Focused APF tests pass 26/26; full forensic regression passes and
Ginkgo reports 258 passed, 0 failed, 2 skipped; static analysis passes. One
thousand deterministic understanding passes took 971.0366 ms.

This is source acceptance, not live acceptance. No deployment, schema/data/UI/
model/worker change, retained mutation, R8/CCPD action, stage, commit or push
occurred. Backend source changed, so an operator-run guarded deployment is
required before browser acceptance. APF-3.4 is next and not started.

## 2026-08-17 — APF-2 runtime accepted and closed

The accounting blocker is fixed at its source-of-truth boundary. The retained
evidence drawer now shows 4 verified records, 5 input rows, 1 duplicate and 0
rejected from authoritative catalog/processing data; missing accounting is
`Not available`, not zero. A bounded citation-opening P1 found during resumed
acceptance was also closed by unique catalog identity resolution.

The one authorized Ask query created completed analysis
`c540c875-0cd7-410d-bd6b-3f2be87b0336`. Deterministic
`cdr.temporal_activity` returned 4 exact rows and cited fixture rows 5 and 2;
History reopened the answer and citation, and the citation opened evidence
`4320772b-febe-4af2-b9e3-92cea114da0b`. History advanced 50 to 51; Continue in
Ask populated a follow-up without sending it. Playwright 10/10, scoped lint,
677-module build, responsive dark/light preview and console checks pass.

No Docker rebuild/deploy, second upload, retry, reprocess, repair, migration,
model/profile/worker change, staging, commit or push occurred. APF-3 is next but
not started; R8 remains valid, paused and non-blocking.

## 2026-08-17 — APF-2 runtime halted on source-detail row accounting

The single explicitly approved synthetic CDR upload completed and is preserved.
Evidence `4320772b-febe-4af2-b9e3-92cea114da0b`, version
`ad4b97c9-54eb-4657-afa7-5f06f52ba695` and first-attempt job/run
`605b53d8-195f-4dde-aae6-081fbef6b500` produced 4 canonical records from 5
input rows with 1 duplicate, a KB asset, verified write-once retention and a
valid 3-event custody chain. The workspace now has 11 sources, 10 Ready,
1 Failed and 9,278 accepted records.

Acceptance did not pass. Add Data and the catalog correctly reached Ready, but
opening the source directly from the modal displayed `0 verified rows` instead
of the authoritative 4. The detail response lacks `item.accepted_rows`, and the
modal callback passes only evidence ID and filename, so the drawer's fallback
is zero. The run stopped before Ask and History as required. No retry,
reprocess, cleanup, data repair or second upload occurred. APF-2 remains active
for this bounded UI truth correction; APF-3 must wait. No Docker rebuild is
needed.

## 2026-08-17 — APF-2 unified Data intake accepted in source

The permanent analyst Data journey is now operational in the source preview.
Add Data supports click, page drop, multiple files, actual upload progress, two
concurrent registrations, partial outcomes and duplicate linking. The catalog
keeps Ready, Processing, Needs attention and Failed separate, loads subsequent
pages from the server, and exposes capability-derived Ask actions only for Ready
sources. Failed sources explain preservation and the read-only approval-gated
reprocess plan; the product does not offer a fake Retry.

This is composition over the existing governed upload, content-addressed
retention, classifier, duplicate lookup, transactional queue, evidence catalog,
capability and reprocess-plan contracts. No backend or database extension was
needed. Scoped lint has zero errors, the 677-module build passes, and focused
Playwright passes 9/9. Real dark/light preview inspection shows the current 10
sources as 9 Ready and 1 Failed. No upload or other retained mutation occurred.

APF-2 remains active only for one explicit approval gate: upload
`ingestion/forensic_records/tests/fixtures/pakistan_cdr_messy_synthetic.csv`
and verify retained evidence, custody, classification, processing and Ready
behavior. The deployed backend already supports this, so no Docker rebuild is
required. Activating the new frontend in the production container is a separate
guarded deployment decision. R8 remains valid, paused and non-blocking.

## 2026-08-17 — Analyst Experience V2 accepted in source

The ordinary analyst portal is now a calm, action-oriented product across Home,
Data, Ask NexusAI and History. It removes dashboard/family-inventory emphasis,
uses friendly workspace/source language, derives suggested questions from the
backend capability registry, and makes prior answers searchable, reopenable and
continuable. Add Data remains visibly and truthfully disabled until APF-2.

The production build and scoped lint pass; 21/21 focused browser tests pass.
Real-data browser review passed at 390, 820, 1024 and 1440 pixels, in light and
dark themes, with no horizontal overflow or console errors. Docker was not
rebuilt or deployed because Vite is the correct inspection path for this
iterative source slice. No backend, data, model, worker, profile or R8 artifact
changed. APF-2 is now active; R8 remains valid, paused and non-blocking.

## 2026-08-17 — Simple analyst portal foundation source accepted

The ordinary-user product is now a distinct `/analyst` shell rather than the
advanced Case Workspace. Home, read-only Data, governed Ask NexusAI and History
reuse the authoritative case/collection registry, current evidence and records
APIs, the accepted agent SSE lifecycle, typed results, citations and governed
history. Root/login now target the portal; administrators retain an explicit
advanced-workspace link. Focused lint, production build and four Playwright
flows pass, and a source preview against the existing live backend returned a
real deterministic CDR frequent-contact result with policy fallback disclosed.

This is source acceptance only: no deployment, schema, evidence, case/
collection, container, model, or external evaluation artifact changed. The
required live question created the normal governed analysis-history record
`f7e12210-a7f2-42ed-a4ad-8144388ac72b`; it remains auditable. R0-R7 and the Case
Workspace remain preserved. R8 is valid but paused and non-blocking; T2-V,
packages, receipts, rejection
evidence and the manually stopped CCPD partial remain untouched. The one active
work item is APF-2: simple upload, evidence-based/ambiguous classification,
progress, failure and retry using existing evidence/job contracts. See
`docs/design/nexusai-analyst-portal-product-architecture.md`.

## 2026-08-13 — Hardware-aware R8 path selected; no download or promotion

NexusAI now distinguishes the current CPU-first developer machine from future
GPU production tiers without changing public API or specialist-agent roles.
The current i7-1260P/15.7-GiB/Intel-UHD system is D0 and has no qualified CUDA
device. Previous OMZ, PARSeq and EasyOCR failures were re-audited and remain
valid rejection evidence. OMZ 0123 is deferred because it repeats the failed
barrier-camera domain at added conversion risk.

The exact next small candidate is FastPlateOCR's plate-specific global ONNX
model (5.02 MiB). It is now acquired in the isolated cache with both publisher
SHA-256 values verified, but is not measured or promoted. It remains measurable
on D0 and later through qualified GPU providers without changing the model
contract. Its network-disabled CPU evaluator and read-only preflight now pass;
the container build/inference waits until the long dataset transfer ends to
avoid contention. CCPD2019 is fully specified as the
isolated real-image T3 target (12.26 GiB, immutable source/checksum, privacy
controls, resumable transfer and hostile-archive gate), but was not downloaded.
Pakistan T4 and explicit production approval remain mandatory. Boundary A/B
remain blocked, R8.5 has not started and the running product is unchanged.

The first long-transfer attempt revealed that curl could reset progress during
an internal retry; it stopped with a safe 28.35-MiB partial. NexusAI now resumes
through a range-validated downloader that re-reads disk state on every attempt,
rejects mismatched HTTP ranges and emits durable progress. Publisher HTTP 206
support and 15 focused source/safety tests pass. No downloaded byte was admitted
as a final artifact.

To avoid idle engineering time, five real images from the official CCPD
repository are now pinned, publisher-hash verified and stored in the isolated
evaluation area. They cover real vehicles, clutter, exposure, perspective and
wet-scene conditions. They have no publisher ground-truth file, so NexusAI uses
them only to prove that a candidate can decode real image inputs and produce
privacy-minimized diagnostics. No strings or crops are retained, and no
accuracy, tuning or promotion decision may use this pilot. Full official CCPD
partitions remain the T3 acceptance evidence.

The small diagnostic now passes its permitted boundary: all five real images
produce nonempty Latin/digit-valid output, mean CPU latency is 30.097 ms, and no
plate string or crop is retained. On the governed 96-image T2-V pack,
FastPlateOCR is efficient but not correct enough: development exactness/CER is
79.49%/15.66%, and validation plus sealed holdout are 78.57%/16.83%. It passes
calibration, latency and memory but fails accuracy and abstention. It is retained
as the best efficient Latin reference, not a selected production model.

## 2026-08-13 — R8 physical deadlock removed without weakening accuracy

The product owner cannot reliably produce the planned physical pack, so NexusAI
records T2-P as `deferred_environment_unavailable`, preserves every capture tool
and stops treating personal photography as the only engineering path. Boundary A
now explicitly requires a stronger feasible combination: controlled virtual
T2-V plus licensed real-image T3-D/T3-O and every unchanged accuracy, privacy,
security, calibration, abstention, resource and reproducibility gate. Boundary B
still requires authorized operational T4 and explicit promotion approval.

A 96-image source-owned T2-V pack is generated and sealed with perspective-aware
vehicles/plates, Pakistan-oriented Latin/Urdu synthetic styles, difficult optical
conditions, 14 hard-negative classes, exact polygons/crops, hashes and separate
56/20/20 development/validation/holdout partitions. Pack integrity passes, but
no model has yet passed it. Offline incumbents were tested on identical T2-V
partitions: the generic detector falls to 18.5% development precision/88.9%
recall, while Paddle English OCR is strongest at only 59.0% exactness/17.8%
CER. Validation and sealed holdout also fail unchanged gates. CCPD is the
primary real T3 authorization target and Artificial Mercosur a supplemental
mixed-real option; neither was downloaded. Boundary A/B remain blocked, R8.5
is not started and production is unchanged.

## 2026-08-13 — T2 capture workflow locally automated without weakening evidence

The analyst no longer has to manually rename and track 32 photographs. A
local-only camera assistant loads the governed plan, guides each scene, writes
the exact JPEG name into the selected pack folder and tracks progress. It sends
nothing externally and performs no inference or annotation. Human work is now
limited to arranging genuine physical conditions, granting local camera/folder
access and providing independent ground truth—the irreducible controls that
distinguish T2 evidence from synthetic T1.

## 2026-08-13 — Controlled T2 workspace created; capture validation hardened

The controlled T2 workspace now exists with the correct 32-slot plan and sealed
holdout, but contains no captured evidence yet. NexusAI now supplies clearly
non-road-use Latin/Urdu printable targets and a pre-annotation checker that
prevents incomplete, unexpected, empty, invalid-signature or duplicate captures
from entering ground-truth review. This preserves the distinction between setup
success and evidence acceptance.

## 2026-08-13 — R8 blockers prepared in parallel; evidence capture is next

R8 is no longer serialized behind one dataset or model. A reusable 32-image T2
session is ready with difficult positives, detector negatives and a sealed
holdout; strict validation prevents model-influenced labels and requires an
independent reviewer. Public-data research now provides alternative paths while
P-LPCD remains legally blocked. Two specialized OpenVINO detectors and two
bounded OCR challengers are researched without download. Evaluator provenance
is strengthened for fair same-partition comparison. No acceptance threshold,
production model, service or retained evidence changed; Boundary A remains
blocked until controlled/public evidence and passing candidates exist.

## 2026-08-12 — R8 two-boundary gate applied; controlled evidence is next

R8 now distinguishes safe non-production engineering eligibility (Boundary A)
from production promotion (Boundary B). Neither passes. The controlled T2 pack
has a strict manifest and validator, but the physical images and independent
labels still need capture. P-LPCD was not downloaded because its official
publisher and author records conflict on commercial-use licensing and class
count. Offline tuning did not move OCR accuracy; resizing detector input to 640
improved precision from 60.7% to 85.0% while retaining 94.4% recall, still
below the unchanged 95%/95% requirement. Production containers and model roles
remain unchanged, and R8.5 has not started.

## 2026-08-12 — R8 independent evidence program; expanded candidates rejected

NexusAI no longer waits exclusively for team-provided evidence. One governed
T0-T4 registry now separates deterministic, synthetic, controlled, public and
authorized-operational evidence. R8's reproducible 29-fixture pack covers all
14 required classes with difficult positives, negatives and safely isolated
hostile inputs. The expanded offline benchmark exposes the former text detector
as unsuitable (60.7% precision) and keeps every OCR candidate below the gate;
Paddle Arabic is strongest at 87.5% normalized exactness and 6.86% CER. T2 demo
capture is specified, credible T3 candidates are researched without download,
and T4 remains pending. Production is unchanged and no model is promoted.

## 2026-08-12 — R8.3/R8.4 first measured pass; superseded by expanded evidence

The isolated approved evaluation completed offline after official Paddle assets
were cached. Production containers and model roles were unchanged. A corrected
role-aware scorer shows the Paddle detector at 1.0 precision/recall on seven
simple positive synthetic scenes, but this is not acceptance: only 8/14 fixture
classes exist, package/license evidence is incomplete, and no authorized
real-image pack exists. Paddle Arabic and English OCR each achieve only 0.7143
normalized exact accuracy; Tesseract achieves 0.4286. All OCR candidates fail
the accuracy/CER/calibration gate. R8.5 remains correctly blocked.

## 2026-08-12 — R8 UI/API live; approved model evaluation pending execution (superseded)

The R8 activation defect was corrected at its read-only container-identity
preflight. The guarded run then rebuilt only the forensic API and NexusAI UI/API,
preserved both rollback images and all named volumes, and passed health,
readiness, desktop and 390 px Evidence checks with a clean browser console. The
worker, models and profiles did not change. The operator has approved the local
visual/OCR evaluation. A pinned isolated PaddleOCR/Tesseract evaluator and
non-personal synthetic fixture generator are ready in source, but the external
execution service reached its usage limit before the approved image build.
Accordingly, no model download, benchmark, selection or production assignment
is claimed, and R8.5 remains gated on measured R8.3/R8.4 acceptance.

## 2026-08-12 — R8.3 platform accepted; R8.4 approval gate complete

NexusAI now enforces versioned, original-pixel plate-region candidate geometry,
bounded deterministic NMS and exact repeatable lossless crop/hash provenance.
The Evidence desk presents recorded regions responsively as candidates rather
than proof. The live read-only inventory contains no eligible detector or OCR
runtime, and no approved visual images exist in the fixture taxonomy. R8.4
therefore correctly requires operator approval instead of downloading or
promoting a model. The complete backend and protected UI regression gates pass;
nothing was deployed, ingested or reconfigured.

## 2026-08-12 — R8.1 complete; R8.2 source accepted

Visual-evidence entry now reuses the governed case, immutable evidence,
custody, ANPR and model-approval foundations under an explicit hostile-image
threat model. Authorized raster images receive a bounded, versioned admission
decision covering bytes, signature, dimensions, pixel budget, orientation and
metadata privacy. Unsafe or unsupported inputs remain review-bound and cannot
silently enter decoding. Evidence Operations presents the decision without
claiming plate detection or OCR. Long Ask results also gain an explicit tested
Jump to latest recovery. R8.3 plate-region detection and crop provenance is
next; no model was downloaded and nothing was deployed or ingested.

## 2026-08-12 — R7 complete; R8 activated

The supplied team-lead CDR passed privacy-safe read-only validation: all 3,931
rows normalized with exact locators, explicit party roles and timezone basis;
297 exact duplicates remained visible; no raw identifiers, upload, retained
copy, database write or model call occurred. Together with the accepted 65
queries, six specialists, hardening and synchronized live telecom UI, this
closes R7. R8 is active at entry reconciliation and threat modeling for governed
ANPR image detection, OCR, human review and exact derived-evidence provenance.

## 2026-08-12 — R7.9 hardened and R7.10 live accepted

The deployed Pakistan telecom workflow now carries exact typed map and timeline
descriptors through Ask NexusAI, including synchronized selected-evidence detail
with timestamp, supplied coordinates, datum, uncertainty and source file/row/
hash. All 65 deterministic operations pass, all six specialist model paths are
covered, and desktop plus 390 px live UI acceptance has no horizontal overflow
or console errors. The API-only bridge deployment preserved rollback and named
volumes and did not alter profiles. At this historical checkpoint, R7.8
protected-CDR validation was the sole remaining R7 gate. The newer closure
entry above records its subsequent acceptance and the activation of R8.

## 2026-08-12 — R7.4 and R7.5 source accepted

Tower references now retain explicit provider/site/sector history, supplied
versions, validity basis, coordinate datum/method/source and uncertainty. The
system identifies overlapping reference windows and conflicting supplied facts
without choosing a hidden winner. A time-valid reference remains location
context only—not RF coverage, device position or movement proof.

Realistic telecom language now routes to the governed deterministic operations:
provider/sector history, overlap review, ICCID/service observations,
packet-data/USSD usage and SIM/device history. Missing targets clarify and zero
rows remain a bounded negative result. Focused tower tests, all 80 ingestion
tests and the forensic Go package pass. Nothing was deployed or ingested. R7.6
telecom map/timeline presentation is next.

## 2026-08-12 — R7.2 and R7.3 source accepted

Pakistan telecom normalization now preserves what the provider supplied and
adds deterministic canonical roles beside it. CDR records distinguish the
subscriber, originating party, called party, service label and direction;
source timezone and UTC conversion basis remain visible. Subscriber records
separately identify subscriber, SIM/ICCID/IMSI, device/IMEI and service/provider
roles, and every link is explicitly an observed source-row combination—not an
ownership or physical-use conclusion.

Both adapters are version 1.2 with legacy compatibility. Synthetic goldens,
10 focused adapter tests, the full 79-test ingestion suite and the 235-spec
forensic Go package pass. Nothing was deployed or ingested. R7.4 tower
reference history and validity deepening is next.

Demo after an approved deployment: query a normalized CDR and show raw versus
canonical party roles, timezone basis, service class and quality flags; then
open subscriber device links and show role-separated IMSI, ICCID, IMEI and
service observations with validity and provenance.

## 2026-08-12 — R7 activated

R6/R6.6 is complete and live accepted. R7 is now active. The entry audit reuses
the existing operational CDR, subscriber and tower adapters, exact forensic API,
six deployed specialists, case/tenant controls, provenance and approved Qwen
explanation role. No new telecom model is required.

The first R7 slice fixes a material analytical gap without changing evidence:
the CDR-to-tower operation now shows whether each observation has one eligible
reference, overlapping eligible references, no supplied reference, or supplied
references outside the event-time window. Exact/eligible candidate counts and
aggregate outcome metrics remain visible, so the system no longer silently
hides reference ambiguity. Focused forensic Go tests pass.

Demo: ask `Join CDR observations to the valid tower reference for
PK-LHR-SYN-001`, then inspect candidate counts, match status, validity bounds,
uncertainty and both CDR/reference provenance. Explain that a reference match is
location context—not RF coverage or handset position. R7.1 is source accepted,
not deployed; R7.2 Pakistan CDR normalization is next.

Date: 2026-08-11  
Active program phase: R6 — corrective quality extension  
Active work item: R6.6-B2-LIVE — guarded rebuild and live closure

## R6.6-A quality correction

Ask NexusAI now fails safely on underspecified frequent-contact questions,
requires one participant, and excludes service labels and self-contact from the
ranked result. Common inventory/readiness questions lead with the answer and
state their scope. The response surface hides planner plumbing, presents useful
columns and readable source names, keeps full audit data behind Technical trace,
and improves retained historical answers without rewriting them. The analyst
surface is visibly Ask NexusAI with compact case/specialist context and five-row
progressive disclosure.

Source/predeployment verification is green: 65/65 deterministic operations,
six/six operational specialists, forensic API, focused presentation, the
669-module production build, 17/17 Agent Chat contracts and the complete
responsive light/dark retained-answer browser matrix. Targeted lint has zero
errors and browser diagnostics are clean. This correction is not deployed; the accepted
`:8080` runtime remains the previous R6 image. R7 is blocked until the remaining
multi-turn/answer-matrix work and an explicitly approved guarded rebuild pass.

Demo now: run the source UI against the live API, open retained “which files
were ingested?”, and show the direct “10 source files” answer, useful seven-
column preview, readable filenames, five-row expansion and collapsed trace.
Then show that the frequent-contact starter asks the analyst to choose a number.
Follow with a valid target query and “Only outgoing”; the request reuses only
the bounded authorized filter context. A new targetless chat still clarifies.

## R5 closure and R6 activation

R5 is complete and live-runtime accepted. The final guarded deployment passed
in one attempt, preserved rollback images and named volumes, and left all five
services healthy. The governed case contains 10 evidence sources and 9,274
canonical rows with zero deletions. Evidence Operations now presents the
retained three-event same-second chain as `Hash chain verified` with zero broken
links, exact 2/2 row accounting and a read-only reprocess approval review.

R6 has begun from the existing product rather than rebuilding it. NexusAI
already has 65 deterministic query templates, nine queryable families, six
operational specialist agents, two approved models, case-scoped conversations,
stale prior-case event rejection and cited professional results. The first R6
source correction makes specialist handoff operationally truthful: Ask NexusAI
checks the agent registry before showing a link, marks an offline specialist as
unavailable, and preserves deterministic analysis if registry discovery fails.
R6.1-B now makes Ask NexusAI the explicit return point from specialist analysis,
preserving the current/latest question and governed case through send and case
switching without changing generic Agent Chat. The 669-module production build
and combined Agent Chat plus Case Workspace suite pass 25/25. R6.1 is source
accepted. R6.2-A then mapped every execution mode. Final messages are correlated,
but distributed status and live stream events omit request identity; worker
cancellation is internal-only; deadlines vary; reconnect has no replay/status
reconciliation; and retry is not idempotent. R6.2-B will first make every
lifecycle event request-correlated, because safe cancellation and retry depend
on that invariant. R6.2-B is now source accepted: local and distributed events
carry request identity, and the browser isolates interleaved live events by
request plus governed case. Focused Go checks, agentpool compilation, the
669-module production build and combined browser matrix 26/26 pass. R6.2-C now
adds explicit authorized Stop behavior without expanding into retry or timeout.
R6.2-C is now source accepted with user/agent/case/request-bound cancellation,
correlated cancelled state, regenerated public API artifacts, a 669-module build
and 27/27 production-preview acceptance.
R6.2-D is now source accepted: all governed execution modes share a 210-second
whole-request deadline, deadline expiry is a correlated `timed_out` outcome,
analyst Stop remains `cancelled`, and the concise UI state explicitly confirms
that no automatic retry was sent. Focused Go/compile and production build gates
pass; the Agent suite is 11/11, the combined matrix is 28/28, and in-app preview
inspection is clean.
R6.2-E is now source accepted with an authorized scoped status lookup, bounded
local/distributed lifecycle registry and explicit no-replay/no-retry reconnect
behavior. Focused Go/compile, pinned Swagger and the 669-module build pass; Agent
Chat is 12/12, the deterministic combined matrix is 29/29 and in-app preview is
clean. Two-worker diagnostics remain timing-unstable at 28/29 and are recorded
without a false pass claim. R6.2-F is now source accepted: explicit terminal-
state retry is user/agent/case/request bound, returns `retry_of`, atomically
deduplicates distributed submissions in the existing observability store, and
never retries automatically. R6.3-A is now source accepted end to end: an
additive tenant/user/agent/case-scoped history table retains requests before
dispatch and final answers without hidden reasoning; API-side lifecycle
persistence covers distributed workers; five case-authorized APIs provide
list, detail, idempotent save state, explicit import and bounded rollback; and
the UI uses server authority while preserving local sessions as an explicit
import source. The responsive history drawer is accessible at 390px. Focused Go
tests and compile gates, pinned Swagger 5/5, the 669-module build, Agent browser
matrix 16/16 and in-app desktop/mobile inspection pass. No live deployment,
schema activation or retained import occurred. The guarded activation command
is the active item; R6.3-B follows only after its live marker passes.

Demo: open `/app/cases/nexusai-forensic-demo/evidence`, select
`r5_live_intake_ready_20260810.csv`, and show exact 2/2 accounting plus the
verified custody chain. Then open Ask and show case-locked deterministic analysis
and only operational specialist continuations. R6 source changes are not yet
deployed; the live Ask page remains the accepted pre-R6.1 baseline.

## R3.1 acceptance update

Live deployment is now accepted. The user ran the existing guarded combined
gate; it passed with rollback images and volumes preserved. LocalAI, worker,
PostgreSQL and NATS are healthy, both readiness endpoints return 200, and the
deployed browser serves the accepted R3.1 hashes with a clean console. The
governed case remains available with 9,272 accepted rows. The corrected second
R4 gate also passed; isolated skip focus is 1/1 and the deployed protected
matrix is 61/61. R4 is complete and R5 is active.

R3.1 is complete at the source/local-production-browser boundary. The accepted
R3 functionality is now presented through a distinctive NexusAI Intelligence
Command System: a proprietary primary rail and context header, stronger
light/dark hierarchy, readable enterprise controls, consistent tables/forms and
states, a refined Home command surface, and code-native Settings previews.

Verification is 663 transformed production modules, targeted lint with zero
errors, 34/34 protected plus R3.1 production-browser contracts, no overflow at
390/820/1024/1440, persisted theme/collapse behavior, keyboard focus acceptance,
hashed production assets and a clean interactive browser console. Later R4
deployment was accepted independently; no retained data, model, staging or
repository publication change is implied by the R5 source slice.

Demo: compare Home at 1440 dark and 390 light, exercise the persisted theme and
collapsed rail, open Knowledge and the forensic Analysis desk, then open
Settings and show the square/horizontal/favicon NexusAI previews. R5 is now the
active product phase.

## Outcome

The complete master directive is now repository-governed. Historical phases and
the active R0-R17 program are separated, R0 is closed at its bounded
reconciliation criteria, and R1 is source accepted with exhaustive route and
backend inventories. R2 is source accepted, R3/R3.1 and R4 are complete, and
R5 has advanced through two bounded evidence-operations slices.

## Business value

The team now has one precedence model, one active phase, one active work item
and independent source/deployment/runtime/data status. This prevents accepted
historical capability from being mistaken for completion of the new product
program and prevents the UI roadmap from jumping directly to Agent Chat work.

## Reclassified source work

`R4-PRE-01 — Truthful Case Workspace initialization` is source, focused-test
and live-runtime accepted through the guarded combined deployment. It prevented
transient false-zero case status; later R4 slices and runtime gates completed R4.

## R0/R1/R2/R3 position

- R0: complete; current source/live/model/case/data truth documented and
  protected.
- R1: source accepted; 371 route-file plus 10 app registrations and all 64
  backend identifiers are machine-readable and dispositioned.
- R2: source accepted; asset, tagline, role, terminology, branding/report,
  shell, state/token and visible LocalAI decisions are closed.
- R3: complete at the source/browser boundary; reusable NexusAI identity, branded truthful loading,
  metadata, role/navigation separation, initial semantic tokens, desktop/tablet
  application context, compact mobile context, URL-bound case identity, skip
  navigation and shared truthful route states are implemented source-only.
  Knowledge, Talk and the non-admin Home no-model path now use truthful shared
  states, and visible administration terminology is NexusAI-contextualized.

## Verification carried forward

The protected R4-PRE-01 source baseline previously passed a 658-module Vite
build, targeted ESLint with zero errors and focused Case Workspace Playwright
5/5. R3 now passes a 662-module production Vite build and 27/27 focused
production-browser contracts. Interactive QA passed 390/820/1024/1440 widths,
light/dark themes, hashed assets, metadata/favicon and zero captured browser
warnings/errors.

## Deployment and security

The two user-run guarded R4 deployments preserved rollback images and volumes;
the corrected runtime is accepted. R5-EVID-01/02 performed no Docker deployment,
migration, upload/reprocess, cleanup, model/backend download, retained setting,
staging, commit, push or publication. Governed case data and the dirty worktree
remain preserved.

## Next priority

R5-EVID-01 and R5-EVID-02 are source accepted. The Evidence module is now a professional,
responsive operations desk with compact metrics, safe multi-file staging,
search and filters, case-scoped source selection, integrity details, exact row
accounting, immutable processing runs, lineage, linked artifacts and actionable
notes. Its new case-scoped detail adapter fails closed on unavailable case
authority. Focused Go/auth checks pass, Vite builds 669 modules, the Case
Workspace suite passes 12/12, and the protected production-preview matrix
passes 62/62 with clean 1440/390 visual acceptance.

R5-EVID-02 adds tenant/collection/evidence-bound canonical processing runs and
events, derived artifacts with stable `nexusai://` references, exact locator
metadata, and hash-chain-verified append-only custody history. The responsive
artifact inspector, pipeline ledger and custody chronology preserve explicit
unavailable states. Focused Go contracts pass, the 669-module build passes,
Case Workspace is 12/12, the protected matrix remains 62/62, and the detailed
surface has zero overflow at 1440/390.

R5-EVID-03 is source accepted and adds exact look-ahead pagination, bounded two-file
concurrency, per-file outcomes and truthful recent-job queue observability.
Focused Go, the 669-module production build and the Case Workspace browser
suite pass 13/13. R5 is not complete and this source is not deployed. Live upload-to-ready,
reprocessing and retained state remain separately approval-gated.

R5-EVID-04 is source accepted. It publishes a case-bound, read-only reprocess
plan with exact terminal-job eligibility and immutable-generation context. The
Evidence desk communicates approval requirements clearly and has no execution
control; the complete Case Workspace browser suite passes 14/14.

R5-EVID-05 closes the R5 source boundary. Swag v1.16.6 regenerated all three
published API artifacts and includes the case/evidence-bound reprocess-plan
route. Public Records Intelligence documentation now states the fail-closed
approval contract. The R5 PowerShell activation gate reuses the proven combined
rollback-safe deployment and follows it with GET-only health, app, catalog,
contract and reprocess-plan checks. Script parsing, generated JSON validation,
focused forensic Go tests, the 669-module production build and the carried
14/14 Case Workspace suite pass. R5 remains program-in-progress until the
operator-run rebuild and controlled live intake acceptance pass.

## MMV-2 final presentation update — 2026-08-25

The last two MMV-2 presentation defects are source-resolved. Positive ANPR
findings no longer display contradictory no-match text, and a successfully
completed video ANPR processor with zero groups displays as “processing
complete · 0 detections,” distinct from filter miss, not processed, failure or
unavailable. The same state survives governed History reopen.

After approved activation, demonstrate the retained positive plate in English,
Roman Urdu and Urdu, open its citation, run an absent plate, execute the
retained grouped-video operation, save/reopen each state and verify
390/820/1024/1440. No retained upload/reprocess is needed. Deployment scope is
only forensic API plus LocalAI/UI and remains gated by 6 GiB free RAM. MMV-3
has not started.
