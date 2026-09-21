# NexusAI evidence test and demo program

Status: active cross-cutting contract; R8-only implementation in progress  
Registry: `configuration/nexusai_test_dataset_registry.json`

## Purpose and tiers

NexusAI develops independently of ad-hoc operational samples without treating
generated evidence as production proof:

| Tier | Authority | Current R8 use |
| --- | --- | --- |
| T0 | Exact deterministic fixtures | image admission, hostile-input and unit goldens |
| T1 | Synthetic realistic evidence | detector/OCR integration and false-positive challenges |
| T2-V | Controlled NexusAI virtual/rendered evidence | reproducible difficult scenes and exact generated truth; never real/physical |
| T2-P | Optional controlled physical evidence | future real capture variability when a suitable environment exists |
| T3 | Public/licensed benchmarks | distribution-shift and comparable model evaluation |
| T4 | Authorized operational evidence | production-oriented agreement only |

Every acceptance report must separately state `synthetic_accepted`,
`controlled_demo_accepted`, `public_benchmark_accepted`,
`authorized_real_data_accepted`, `runtime_accepted`, and
`production_promoted`. None implies another.

## Storage, integrity and reproducibility

- Small safe fixtures may live in repository testdata or be generated from a
  committed deterministic generator.
- Large/licensed T3 media belongs in a controlled local cache. Git stores only
  provenance, license decisions, version, expected hashes and benchmark output.
- Model assets remain in isolated model caches. T4 remains in separately
  protected authorized storage.
- Every pack records generator/version/seed, count, schema and SHA-256 manifest.
- A hash mismatch blocks use. Demo evidence follows normal ingestion and query
  paths; production logic never recognizes a demo case specially.
- Evidence readiness is created just in time for the active phase. R8 does not
  initiate audio, video, document or archive dataset projects.

## R8 T0/T1 pack

`tools/r8_ocr_eval/generate_fixtures.py` version
`nexusai-r8-synthetic-v2`, seed `nexusai-r8-t0-t1-20260812`, produces 29
fixtures: 6 T0 and 23 T1. It covers all 14 contract classes and adds perspective,
small/large/border plates, motion blur, exposure extremes, JPEG artifacts,
multiple plates, six negative detector scene families and an OCR-abstention
crop. Four hostile files are benign
admission tests and have `safe_for_model_input=false`; detector/OCR runners must
skip them. Repeated generation produced manifest SHA-256
`e847542ad12f4eec0731e66f9f5361affbda310e24b2f479b18a6d3226ef24f3`.

Materialization under `.tmp` is reproducibility evidence, not retained case
ingest or runtime acceptance.

## R8 T2-P controlled-physical protocol (deferred)

T2-P tooling is `source_accepted`; evidence is
`deferred_environment_unavailable`. The workflow is preserved for future use
but is no longer the active Boundary-A dependency.
T1 scenes may be shown as explicitly synthetic demonstrations but never relabeled
T2 or real. A future controlled capture requires:

1. project-test purpose and creator/owner/consent record;
2. an owned/authorized vehicle/plate or a deliberately synthetic plate;
3. avoidance of unrelated people, documents, homes and sensitive location;
4. original bytes, SHA-256, capture time if known, device and transformation
   provenance, plus a privacy review of EXIF/GPS/free text;
5. separate annotations by a named actor, with original-pixel plate boxes and
   exact visible text; and
6. internal-demo, model-input and redistribution decisions recorded separately.

Assets remain outside Git until privacy/ownership review. A controlled pack is
not operational acceptance.

The canonical draft is
`configuration/nexusai_r8_t2_manifest_template.json`; validate a completed pack
with `scripts/validate_nexusai_r8_t2_pack.py`. The validator checks independent
ground truth, ownership, attributed privacy review, safe relative paths,
original-byte hashes, image signatures, plate-count consistency and derivative
lineage. An empty draft may validate structurally but reports
`acceptance_ready=false`.

### Physical capture checklist

1. Use only a project-owned/explicitly consented vehicle and plate, or an owned
   non-operational display plate. Never photograph incidental traffic for T2.
2. Capture a compact diversity matrix: frontal daylight, angle, near/far,
   small/large plate, partial occlusion, shadow, glare, low light, controlled
   motion, edge framing, portrait/landscape and multiple owned plates/vehicles
   where safely available. Use T1 for safely unavailable conditions.
3. Keep people, unrelated plates, homes, addresses, documents and screens out
   of frame. Use a non-sensitive controlled location.
4. Copy original bytes into a local untracked pack directory. Do not rename,
   edit, crop or strip metadata before hashing the original.
5. A human annotator records original-pixel polygons and exact visible text
   without consulting model output. A second named reviewer performs privacy
   and ownership review.
6. Reject unsafe originals. A sanitized demonstration derivative must retain
   parent/original hash, its own hash and the exact transformation; never present
   it as an undisclosed original benchmark image.

## R8 T3 research disposition

- **CCPD 2019:** over 300,000 single-plate images with blur, rotate, tilt and
  challenge subsets and filename annotations. The official repository says MIT.
  It is useful for adverse-condition benchmark comparison but real plate imagery
  creates a privacy review and it has no Pakistan agreement value. Download is
  not approved.
- **UFPR-ALPR v1:** 4,500 annotated 1920x1080 images from moving cameras with
  cars and motorcycles. The publisher limits use to academic, non-commercial
  research, so it is evaluation-only and unsuitable as a commercial product
  corpus. Download is not approved.
- **P-LPCD:** the official Zenodo 1.0.0 record contains one 1,235,177,299-byte
  `P-LPCD.zip` with publisher MD5 `293f7c2497d13f53ab282b630a362603`,
  40,000 synthetic crops and 650 real crops. Zenodo says CC BY 4.0, but the
  associated author repository says the repository and associated files are
  CC BY-NC 4.0. That conflict requires publisher clarification. The metadata
  also says "36 classes" while enumerating A-Z, 0-9 and `PUNJAB` (37 labels).
  It is OCR/character-localization evidence, not full-scene detector evidence.
  Download is not approved.

No single T3 candidate covers multi-vehicle detection, Pakistan OCR and negative
false-positive behavior. Separate per-tier metrics are mandatory.

## Applied feasibility-corrected two-boundary gate

The product-owner directive applies two independent boundaries without changing
any quantitative threshold:

- **Boundary A—engineering pipeline eligibility:** requires accepted T0/T1,
  controlled virtual T2-V, suitable licensed/privacy-reviewed real-image T3-D
  and T3-O where available, hostile-input safety,
  candidate accuracy/calibration/resource passes and reproducible artifacts.
  T4 may remain pending. A pass authorizes only a governed, production-disabled
  R8.5 engineering state.
- **Boundary B—operational production promotion:** additionally requires
  authorized T4, T1/T2/T3-to-T4 agreement, live authorized-case acceptance,
  overlay/source/citation and operational resource validation, plus explicit
  promotion approval.

Both boundaries are currently blocked. R8.5 has not started. T4 no longer blocks
engineering indefinitely after Boundary A, but T0-T3 can never be described as
operational agreement or production approval.

Physical T2-P is optional/deferred for Boundary A only when accepted T2-V and
suitable real-image T3 both pass. This amendment does not reduce thresholds or
allow T2-V to establish real-world or operational generalization.

## R8 T2-V materialization — 2026-08-13

`tools/r8_t2v/generate_t2v.py` and
`configuration/nexusai_r8_t2v_contract.json` define a source-owned procedural
scene compositor. It models vehicle/background geometry, perspective-warped
plate surfaces, exact original-pixel polygons, Pakistan-oriented synthetic
Latin/Urdu styles, scale, lighting, glare/shadow, blur, compression, occlusion,
noise and 14 hard-negative classes. It uses no external photographs, network or
personal data.

The isolated pack at `C:\NexusAI-Evaluation\R8\T2V\1.0.0` contains 96 images:
56 development, 20 validation and 20 sealed holdout. Validation passes and the
manifest SHA-256 is
`180889ad2dc78736cba6617ab49b3b910ddfd29e8dfb77a143fc9ce26f50e510`.
Generation acceptance is not candidate accuracy acceptance. Tuning mode is
forbidden from accessing `all`, `holdout` or `sealed_holdout` partitions.

## R8 Boundary-A no-stall update — 2026-08-13 (superseded for T2 feasibility)

T2 is now operationally specified as one 32-image controlled session: 18
positive/target scenes, 10 text-like/rectangular negatives and four sealed
holdout images. The generator, strict manifest validator and exact operator
runbook require a distinct annotator/reviewer, forbid model-influenced truth and
record development/holdout partitions. Physical assets remain absent, so T2 is
not accepted.

T3 no longer depends on P-LPCD. Artificial Mercosur v1 is the preferred
detector authorization candidate after privacy/source-rights review; CCPD is a
secondary detector/OCR stress candidate after privacy and artifact-integrity
review. SYNLIP v2 has clear CC BY 4.0 terms but is synthetic and supplemental,
so it cannot independently close the real-world T3 gate. INDO-ALPR remains
deferred because publisher metadata does not establish annotations, splits,
integrity or privacy suitability. UFPR and RodoSol remain non-commercial
reference-only; P-LPCD remains blocked. No dataset was downloaded.

## Future phase contract

At R9-R17 entry, add an Evidence/Data Readiness section with T0-T4 status and
build only the missing pack for that phase. T4 absence is a non-blocking
operational dependency unless that phase's explicit exit gate requires it. R17
consolidates accepted T0-T3 manifests into immutable release regression goldens;
protected T4 evidence remains outside the distributed corpus.
