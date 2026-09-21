# NexusAI R8 feasibility-corrected evidence program

Date: 2026-08-13  
Disposition: T2-P environment unavailable; T2-V source pack sealed; Boundary A blocked; Boundary B blocked; R8.5 not started; no promotion

## Verified current state

- R8.1 is accepted; R8.2 and R8.3 source/live surfaces remain accepted.
- R8.4 evaluation infrastructure is accepted. Historical measured results are
  unchanged: the generic Paddle detector is unsuitable; Paddle English and
  Arabic OCR are promising but below the unchanged OCR gate; Tesseract is a
  diagnostic baseline only.
- T0 and T1 evidence exist. The T2 physical-capture tooling is source accepted,
  but the operator lacks the physical environment and independent reviewer.
- No T3 dataset or replacement model was downloaded. R8.5 has not started.
- Production containers, models, roles, databases, evidence and volumes were not
  changed.

## Physical T2 feasibility decision

The previous Boundary-A rule required an accepted controlled physical T2 pack.
That method is valid, but is not feasible in the current operator environment.
This is classified as:

```text
physical_t2_capture_status = environment_unavailable
physical_T2_tooling = source_accepted
physical_T2_evidence = deferred_environment_unavailable
```

It is not a model/product failure. The capture assistant, plan, targets,
validator, manifest, checker, runbook and tests are preserved. The empty
`C:\NexusAI-Evaluation\R8\T2\1.0.0` workspace is not evidence and is not populated
with generated data.

## Explicit Boundary-A amendment

Old rule:

```text
T0 + T1 + mandatory accepted controlled physical T2 + suitable T3 + unchanged
accuracy/security/privacy/resource/reproducibility gates
```

Amended engineering-eligibility rule:

```text
T0 PASS
T1 PASS
T2-V controlled virtual PASS
T3-D licensed public real-image detector benchmark PASS
T3-O licensed public real-image OCR benchmark PASS where legally/technically available
license/privacy PASS
hostile-input PASS
unchanged detector/OCR thresholds PASS
calibration/abstention/resource/reproducibility PASS
```

T2-P is optional/deferred for Boundary A only when both T2-V and suitable
real-image T3 evidence pass. This does not make T2-V real, physical, public or
operational. Boundary B is unchanged: it still requires Boundary A, authorized
T4, distribution-agreement analysis, live-case/overlay/citation/lineage and
resource acceptance, plus explicit promotion approval.

Maximum claim after Boundary A is `engineering_accepted`,
`controlled_virtual_accepted`, `public_real_benchmark_accepted`,
`operational_agreement_pending`, and `production_disabled`. Pakistan
operational, field-proven, law-enforcement or production-ready claims are
prohibited until Boundary B.

## Accuracy and generalization risk

The amendment does not reduce the numeric thresholds. Detector precision and
recall remain at least 0.95; OCR normalized exact accuracy remains at least 0.90
with CER at most 0.05; calibration, abstention, latency and memory limits remain
unchanged.

T2-V improves controlled coverage and exact ground truth but cannot establish
camera/sensor or real-scene generalization. T3 is therefore mandatory and must
be reported separately. Pakistan-oriented T2-V plus an international T3 can
support an engineering claim, but only the statement “Pakistan controlled
evidence and international public-real benchmark accepted.” It cannot establish
Pakistan operational accuracy. Synthetic-to-virtual and virtual-to-public-real
metric gaps must remain visible rather than being hidden in one average.

## T2-V architecture and materialized pack

The source-owned contract is
`forensics.controlled-virtual-anpr-pack/v1`. The generator uses the already
approved isolated Pillow runtime and no network or external photographs. It
procedurally composes backgrounds and vehicle geometry, renders synthetic plate
surfaces and text, calculates a four-point perspective transform, and preserves
the source and transformed polygons. It models scale, horizontal/vertical
perspective, multiple plates, edge placement, occlusion, glare, shadow,
night-like exposure, over/underexposure, motion/defocus blur, compression,
low contrast, busy scenes and sensor noise.

Hard negatives include signage, billboards, logos, grilles, reflectors,
documents, screens, shop boards, random/house-number-like text, road signs,
plain vehicles, repeated rectangles and architecture. Plate styles are labelled
only `Pakistan-oriented synthetic style`; they are not official provincial
replicas. Identifiers have no ownership or person association.

Materialized pack:

```text
Location: C:\NexusAI-Evaluation\R8\T2V\1.0.0
Images: 96
Development: 56
Validation: 20
Sealed holdout: 20
Manifest SHA-256: 180889ad2dc78736cba6617ab49b3b910ddfd29e8dfb77a143fc9ce26f50e510
Validation: PASS
Holdout sealed: true
Production authority: false
```

Random seeds and plate identities do not overlap across partitions. The scorer
rejects `tuning` mode against `all`, `holdout` or `sealed_holdout`. The pack is
generated and validated, but not candidate-accuracy accepted.

Offline incumbent results (development / validation / sealed holdout):

| Candidate | Development | Validation | Sealed holdout | Decision |
| --- | --- | --- | --- | --- |
| Paddle generic detector | precision 0.1852, recall 0.8889, Brier 0.5169 | precision 0.2113, recall 0.9375, Brier 0.4818 | precision 0.2027, recall 0.9375, Brier 0.4914 | replace; fails unchanged localization/calibration gates |
| Paddle English OCR | exact 0.5897, CER 0.1779, Brier 0.2558 | exact 0.5000, CER 0.1881, Brier 0.3117 | exact 0.6429, CER 0.1485, Brier 0.2319 | below gate on every partition |
| Paddle Arabic OCR | exact 0.2564, CER 0.2384, Brier 0.3633 | exact 0.2857, CER 0.2079, Brier 0.3746 | exact 0.3571, CER 0.1683, Brier 0.3769 | below gate on every partition |
| Tesseract | exact 0.2564, CER 0.5445, Brier 0.2889 | exact 0.2143, CER 0.5446, Brier 0.1640 | exact 0.2857, CER 0.4752, Brier 0.3031 | diagnostic only |

All results are under
`C:\NexusAI-Evaluation\R8\T2V\1.0.0\results`. The run used cached artifacts,
network disabled, no tuning and no automatic promotion. The holdout result is a
final benchmark observation and must not be used to select preprocessing or
thresholds.

## T3 current primary-source decision

No reviewed dataset is immediately T3 accepted. Legal usability, artifact
integrity, privacy and task relevance are separate decisions.

| Candidate | Role | Rights/status | Privacy/integrity | Decision |
| --- | --- | --- | --- | --- |
| CCPD 2019 | primary T3-D and T3-O authorization target | official repository presents MIT | real plates; official download artifacts/sizes/checksums and bounded privacy handling still unresolved | do not download; complete artifact and privacy gate |
| Artificial Mercosur v1 | supplemental T3-D mixed-real target | CC BY 4.0, DOI 10.17632/nx9xbs4rgx.1, 3,840 images | synthetic identities over real monitoring/parking backgrounds; bystander/source-background review required; publisher does not expose one complete archive checksum | do not download; not sufficient alone as real-plate OCR evidence |
| UFPR-ALPR | real detector/OCR reference | academic/non-commercial terms | real moving-camera imagery and identifiers | reject for normal product evaluation unless separate permission is obtained |
| RodoSol-ALPR | real detector/OCR reference | academic research only, signed agreement, non-commercial | faces reportedly blurred; real toll identifiers | reject for normal product evaluation unless separate permission is obtained |
| P-LPCD | Pakistan OCR research | Zenodo CC BY versus author-repository CC BY-NC conflict | real-crop privacy and class-count discrepancy | blocked; not the critical path |
| SYNLIP v2 | supplemental public synthetic OCR | CC BY 4.0 | non-personal synthetic | clear-license supplement only, never independent real T3 |

Primary sources:

- <https://github.com/detectRecog/CCPD>
- <https://data.mendeley.com/datasets/nx9xbs4rgx/1>
- <https://web.inf.ufpr.br/vri/databases/ufpr-alpr/>
- <https://github.com/raysonlaroca/rodosol-alpr-dataset>
- <https://doi.org/10.5281/zenodo.17182320>
- <https://data.mendeley.com/datasets/dfzkr3p73v/2>

Before any T3 download: verify the exact official artifact, version, size,
publisher checksum or an independently pinned acquisition hash, license copy,
internal-evaluation/commercial-training/redistribution/demo-display rights,
faces/locations/identifiers, safe extraction, annotation audit and fixed splits.
Real imagery remains isolated and cannot be displayed in the product demo.

## Replacement model decision

No download is authorized by this source work.

Detector shortlist remains intentionally small:

1. Open Model Zoo `vehicle-license-plate-detection-barrier-0123`, public/2022.1,
   0.547M parameters, 256x256, CPU/OpenVINO capable. Its Chinese front-facing
   barrier domain and 96-pixel minimum plate width create high Pakistan/small
   plate risk.
2. Open Model Zoo `vehicle-license-plate-detection-barrier-0106`, 0.634M
   parameters, 300x300, CPU/OpenVINO capable, with the same domain/minimum-size
   risk.
3. Paddle detector remains the measured replacement-required baseline.

OCR shortlist:

1. Paddle English and Arabic PP-OCRv5 mobile recognizers remain incumbent
   controls; a dual-recognizer script policy may route but never invent truth.
2. PARSeq v1.0.0 revision `315d19b` is the primary Latin challenger; its
   PyTorch/resource cost and exact checkpoint hash/size require pre-download
   capture.
3. EasyOCR 1.7.2 `arabic_g1` is the Urdu/Arabic challenger; it supports Urdu but
   is an older general-scene CRNN and requires exact weight/runtime inventory.

Primary sources:

- <https://docs.openvino.ai/2023.3/omz_models_group_public.html>
- <https://docs.openvino.ai/2023.3/omz_models_model_vehicle_license_plate_detection_barrier_0106.html>
- <https://github.com/baudm/parseq/releases/tag/v1.0.0>
- <https://github.com/baudm/parseq>
- <https://jaided.ai/easyocr/modelhub/>
- <https://jaided.ai/easyocr/>

## Gate status and next approval

Boundary A remains blocked because every incumbent fails T2-V, no T3-D/T3-O
has completed artifact/privacy acquisition and evaluation, and every measured
candidate fails at least one unchanged threshold. Boundary B remains blocked by
Boundary A, absent T4 and absent explicit promotion approval. R8.5 remains not
started.

The next consequential gate is not manual photography. It is a narrowly scoped
approval for (a) one integrity/privacy-cleared T3 acquisition and/or (b) the
bounded replacement model artifacts. Exact acquisition commands must be issued
only after the official artifact metadata is complete. No production rebuild is
required for the T2-V work.
