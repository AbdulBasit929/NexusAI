# NexusAI R8 ANPR image and OCR intelligence contract

Status: active; R8.1 complete; R8.2 source/live UI-API accepted; R8.3 preliminary synthetic localization measured but accuracy acceptance blocked; R8.4 measured with no promotion  
Date: 2026-08-12  
Scope: authorized plate imagery, image-derived plate observations, OCR review,
and deterministic links to existing structured ANPR records

## Product outcome

R8 turns the accepted structured ANPR foundation into a governed visual-evidence
workflow. An analyst can register an image, inspect immutable technical metadata,
view a provenance-bound plate crop, review model OCR candidates and confidence,
accept or correct text with an append-only audit event, and query exact links to
structured sightings. Models propose; evidence and human-reviewed state remain
authoritative.

Structured ANPR CSV rows do not count as image, detector or OCR acceptance. A
plate string does not establish vehicle ownership, driver, occupants, intent,
association, continuous travel or a road route.

## Non-negotiable evidence contract

- Preserve original bytes, content hash, MIME determination, dimensions,
  orientation and acquisition/source metadata without silent rewriting.
- Every derived crop stores parent evidence/version ID, detector/model version,
  bounding coordinates in original-image space, transform/orientation history,
  crop hash and creation time.
- OCR candidates retain raw text, normalized candidate, per-candidate confidence,
  model/version, preprocessing recipe and crop locator.
- Human decisions are append-only: actor, role, time, prior value, new value,
  reason and source crop. A correction never rewrites original model output.
- Low confidence, conflicting candidates, multiple plates, truncation, glare,
  blur, occlusion and unsupported scripts stay explicit review states.
- Pakistan plate normalization is deterministic formatting only. It never fills
  unseen characters or invents a registration jurisdiction.
- No image, crop, OCR text or identifier enters a third-party service or model
  outside the approved local processing boundary.

## R8 execution slices

### R8.1 — entry reconciliation and threat model

Status: complete. The accepted reuse/capability matrix and threat model are in
`reports/nexusai-r8.1-r8.2-source-acceptance-20260812.md`.

- Inventory reusable image header extraction, content-addressed evidence,
  custody/version APIs, ANPR structured adapter/query operations, Ask renderer,
  installed backends and current authorization boundaries.
- Define supported image types, maximum dimensions/bytes/pixels, decompression-
  bomb controls, malformed-header behavior, EXIF/orientation handling and
  archive prohibition.
- Publish fixture taxonomy: clear plates, regional styles, night, glare, blur,
  skew, occlusion, multiple vehicles, cropped plates, empty scenes and hostile
  files.
- Exit: signed capability/reuse matrix, privacy/threat model and no premature
  model claim.

### R8.2 — immutable image intake and technical metadata

Status: source accepted and UI/API live accepted. `forensics.image-intake/v1`
publishes the bounded admission decision and the Evidence desk presents it
without claiming OCR or detection. The rollback-preserving deployment changed
only the forensic API and NexusAI UI/API and preserved the worker, models,
profiles and named volumes.

- Reuse governed Evidence Operations registration and content-addressed storage.
- Validate type by bytes, bound decoding, preserve original hash and expose
  dimensions, color model, orientation and metadata warnings.
- Register derived work only after source/case/tenant authorization; never
  execute embedded content or macros.
- Exit: valid/malformed/oversized/polyglot fixtures, custody and accounting tests.

### R8.3 — plate-region detection and crop provenance

Status: platform contract source accepted; expanded synthetic detector accuracy
measured and failed.
`forensics.plate-region-candidate/v1`, bounded deterministic NMS, exact lossless
crop reconstruction/hash lineage and the read-only candidate overlay are
implemented and tested. The 29-fixture T0/T1 v2 pack covers all 14 classes and
the offline Paddle text-detector adapter measures 0.6071 precision and 0.9444
recall. False positives on text-like/rectangular negatives disqualify it as a
plate detector; no detector is selected or assigned.
The overlay is deployed and responsive acceptance passes, but the active live
case contains no image evidence; live-image overlay acceptance therefore
remains pending.

- Introduce a typed local detector role with bounded candidates and non-max
  suppression policy.
- Preserve original-coordinate bounding boxes, confidence, detector version,
  transform chain and parent/derived hashes.
- UI overlays candidates without altering original pixels; analyst can reject,
  choose or manually bound a region with an audit event.
- Exit: localization precision/recall benchmark and exact crop reconstruction.

### R8.4 — OCR model benchmark and approval gate

Status: isolated installation and expanded T0/T1 evaluation complete; no
production promotion. The production runtime inventory still has no eligible
detector/OCR role assignment or runtime package. The isolated evaluator contains
the approved PaddleOCR Urdu/English and Tesseract `urd+eng` candidates only;
their isolated presence does not imply production installation, acceptance or
role assignment. See `configuration/forensic_r8_model_approval_gate.json` and
`reports/nexusai-r8-evidence-program-t0-t3-and-expanded-evaluation-20260812.md`.

The approved isolated evaluator pins PaddleOCR 3.7.0, PaddlePaddle 3.3.1 and
Tesseract `urd+eng`. Its completed offline run leaves production unchanged. All
14 required T0/T1 fixture classes are now present and evaluator notices are
verified. The expanded evaluation blocks every candidate: the detector fails precision,
recall and calibration; Paddle Arabic/English OCR reach 0.875 normalized exact
accuracy but miss exactness/CER; Tesseract is materially weaker. T2/T3 agreement
and T4 authorized operational agreement remain incomplete. See
`reports/nexusai-r8-evidence-program-t0-t3-and-expanded-evaluation-20260812.md`.

- Benchmark reusable installed OCR-capable backends first; research/download a
  new model only if the existing inventory cannot meet the fixture gate.
- Score exact plate accuracy, character accuracy, calibration, abstention,
  latency, memory, package size, license, offline operation and CPU/GPU fit.
- Evaluate English alphanumerics plus only those Urdu/regional scripts actually
  represented by approved fixtures.
- Exit: evidence-backed shortlist and explicit operator approval before any new
  model/backend download or production role assignment.

#### R8.4-A through R8.4-F Boundary-A closure program

R8.4 proceeds through bounded internal work items without creating a new
top-level phase: controlled T2-V materialization, T3 authorization, detector
replacement, bounded OCR improvement/replacement, unified evaluator readiness
and the final Boundary A decision. Physical T2-P tooling is preserved and its
evidence is `deferred_environment_unavailable`. The separate 96-image T2-V pack
reserves 56 development, 20 validation and 20 sealed-holdout images. Every new candidate result records
candidate revision, exact fixture-manifest hash, source tier, partition,
preprocessing and environment. Thresholds and production authority remain
unchanged.

The bounded detector shortlist is OMZ 0123 then OMZ 0106, with Paddle retained
only as the measured generic-text baseline. The bounded OCR shortlist retains
Paddle English/Arabic as incumbent controls, with a future confidence-bounded
dual-output script policy; PARSeq is the Latin research challenger and EasyOCR
`arabic_g1` the Urdu/Arabic research challenger. These are research decisions,
not download or promotion approval.

#### Query and API readiness before R8.5

No new public operation is enabled before Boundary A. Existing evidence detail
and typed plate-region/crop contracts already carry parent evidence,
version/hash, original-pixel geometry, detector provenance, crop lineage and
review state. R8.5 may extend those objects with ranked OCR candidates containing
raw text, normalized text, script-policy decision, confidence, model revision,
preprocessing and crop locator; it does not require a duplicate image API.

Future natural-language operations remain design-only: list image plate
candidates, list uncertain readings, locate reviewed occurrences of an exact
plate, compare reviewed image observations with structured ANPR, and list review
work. Until Boundary A passes, these queries must remain unavailable rather
than silently routing to structured ANPR or model guesses.

### R8.5 — bounded OCR pipeline

Status: not started; Boundary A blocked. T4 may remain pending at Boundary A,
but T2-V/T3 acceptance and unchanged candidate accuracy, calibration, hostile-
input, resource and reproducibility gates have not passed.

- Run approved preprocessing variants deterministically and retain their recipe.
- Return ranked candidates with calibrated confidence and explicit abstention.
- Keep detection failure, OCR failure and no-plate-found as different states.
- Exit: golden candidate outputs, reproducibility, timeout/resource bounds and
  zero uncited plate-text promotion.

### R8.6 — Pakistan plate normalization and structured linkage

- Preserve raw OCR beside deterministic uppercase/separator normalization.
- Validate configurable regional format patterns without guessing missing text.
- Link a reviewed candidate to structured ANPR only by exact normalized plate
  plus authorized case/time/camera criteria; expose zero/one/many match states.
- Exit: regional-format, ambiguity, false-link and no-result goldens.

### R8.7 — human review, correction and custody

- Build an accessible image/crop/candidate review desk with zoom, keyboard
  navigation, confidence explanations and side-by-side source facts.
- Require an actor and reason for acceptance/correction/rejection; append custody
  and decision events and retain model output unchanged.
- Separate `model_candidate`, `human_reviewed` and `externally_verified` status.
- Exit: authorization, concurrency, audit-chain and undo-by-new-event tests.

### R8.8 — ANPR specialist and Ask NexusAI integration

- Extend the existing Vehicle ANPR specialist rather than create overlapping
  authority. Add exact image/crop/OCR operations and typed presentation.
- Ask responses show the source image, crop, candidate state, confidence,
  structured matches, citations and limitations without owner/driver inference.
- Provide model failure/timeout fallback to deterministic evidence inventory.
- Exit: natural-query corpus, model-policy rejection, citation and responsive UI
  acceptance.

### R8.9 — performance, security and provenance hardening

- Bound concurrent decodes/inference, image pixels, candidates, crops and result
  windows; collect CPU/GPU/RAM/latency evidence.
- Verify tenant/case denial, source-versus-derived authorization, malicious image
  handling, temporary-file cleanup and no model prompt/data exfiltration.
- Sample every locator chain from answer to crop to parent bytes and custody.
- Exit: large-image/adversarial suite, stable pagination and resource SLO report.

### R8.10 — guarded runtime and manual acceptance

- Use sequential rollback-preserving builds only for services actually changed.
- Apply approved model role/profile configuration separately and verify health,
  rollback, retained volumes and restart recovery.
- Run product-owner desktop/tablet/mobile pack: intake, inspect, detect, OCR,
  abstain, review, correct, cite, query and report.
- Exit: live acceptance report, team-lead demonstration, limitations register and
  R8 closure without starting R9 early.

## R8 two-boundary model gate

**Boundary A—engineering/pipeline eligibility** requires complete T0/T1,
accepted controlled virtual T2-V, accepted suitable licensed real-image T3-D
and T3-O where legally/technically available, verified license/privacy,
hostile-input safety, unchanged candidate thresholds, calibration and resource
passes, and reproducible artifacts. T4 may remain pending. Boundary A permits
only production-disabled R8.5 engineering integration with explicit
`operational_agreement_pending` status.

Physical T2-P is optional/deferred for Boundary A only when both T2-V and
suitable public-real T3 acceptance pass. T2-V is never physical, public-real,
Pakistan-operational or production evidence. This feasibility amendment leaves
every quantitative threshold unchanged.

**Boundary B—operational/production promotion** additionally requires authorized
T4, measured T1/T2/T3-to-T4 agreement, live authorized-case acceptance,
overlay/source/citation validation, operational resource acceptance and explicit
promotion approval. Only Boundary B can authorize a production role.

## R8 phase exit gate

R8 closes only with either an accepted engineering ANPR pipeline plus explicit
operational-promotion status, or a documented capability limitation with model
promotion deferred. Production approval still requires Boundary B. Hostile
images must fail safely, human/model provenance must remain exact, responsive
workflows must pass, and deployed behavior must match accepted source.
