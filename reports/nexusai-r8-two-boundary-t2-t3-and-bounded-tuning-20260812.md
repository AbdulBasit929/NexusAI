# NexusAI R8 two-boundary gate, T2/T3 review and bounded tuning

Date: 2026-08-12  
Disposition: Boundary A blocked; Boundary B blocked; R8.5 not started; **NO PROMOTION**

## 2026-08-13 Boundary-A no-stall update

R8.4-A through R8.4-E now have bounded engineering preparation while R8.4-F
remains blocked. T2 has an exact 32-image plan, workspace generator, 28/4
development/holdout partition, independent annotation-review requirements and
a one-session operator runbook. No physical assets were created.

T3 research expanded without downloading data. Artificial Mercosur v1 is the
preferred detector authorization candidate after privacy/source-rights review;
CCPD remains a privacy/integrity-gated secondary path. SYNLIP v2 is clear-license
but public synthetic evidence only. INDO-ALPR is deferred; UFPR and RodoSol are
non-commercial reference-only; P-LPCD remains blocked.

Detector research is capped at OMZ 0123 and OMZ 0106 plus the Paddle baseline.
OCR research retains Paddle English/Arabic controls and identifies PARSeq for
Latin and EasyOCR `arabic_g1` for Urdu/Arabic evaluation. Evaluator output now
attests candidate revision, exact fixture hash, tiers, partition, preprocessing
and environment. Scoring adds detector F1 and incorrect high-confidence OCR
acceptance. These changes do not authorize download, R8.5 or promotion.

## Gate decision

The product-owner-approved two-boundary model gate is applied. Boundary A can
eventually authorize a production-disabled R8.5 engineering pipeline after T0,
T1, T2, suitable T3, license/privacy, hostile-input, unchanged quantitative,
calibration, resource and reproducibility gates pass. T4 may remain pending.
Boundary B still requires authorized T4, distribution-agreement analysis, live
authorized-case acceptance and explicit promotion approval.

Current state:

- T0/T1 fixture packs: complete;
- T2: manifest/tooling/capture instructions complete, physical assets absent;
- T3: publisher review complete, no approved/downloaded benchmark;
- detector/OCR thresholds: failing;
- Boundary A and R8.5: blocked;
- Boundary B and production authority: blocked.

## Controlled T2 implementation

`configuration/nexusai_r8_t2_manifest_template.json` and
`scripts/validate_nexusai_r8_t2_pack.py` establish independent human ground
truth, original hashes, safe paths, ownership, attributed privacy review,
plate-count/polygon consistency, signature checks and sanitized-derivative
lineage. Raw assets remain outside Git. The empty template is structurally valid
but explicitly not acceptance-ready.

## T3 publisher findings

- **UFPR-ALPR v1:** 4,500 1920x1080 annotated moving-camera images; academic
  research/non-commercial only. Evaluation-only and not approved for download.
- **CCPD 2019:** over 300,000 single-plate images with filename annotations and
  adverse subsets; official repository says MIT. Privacy-sensitive and not a
  Pakistan agreement set; archive size/checksum are not authoritatively stated,
  so download is not approved.
- **P-LPCD 1.0.0:** official Zenodo metadata reports CC BY 4.0, one
  1,235,177,299-byte archive and MD5
  `293f7c2497d13f53ab282b630a362603`. The associated author repository says CC
  BY-NC 4.0 for the repository and associated files. This conflict blocks
  download. Official metadata also says 36 classes while enumerating A-Z, 0-9
  and `PUNJAB` (37 labels). P-LPCD is an OCR/character-localization benchmark,
  not a full-scene plate-detector benchmark.

## Hardware profile

- Intel Core i7-1260P, 12 physical/16 logical cores;
- 15.71 GiB RAM; Docker Linux allocation 7.61 GiB;
- integrated Intel UHD graphics, reported 2 GiB adapter memory;
- no NVIDIA/CUDA runtime;
- approximately 1.72 TiB free on the workspace drive;
- Docker Engine 29.6.2, x86_64.

The current target is CPU-first. A future GPU workstation must rerun identical
artifacts and record its accelerator/backend rather than inheriting CPU claims.

## Detector analysis and bounded experiments

Baseline Paddle text detector: TP 17, FP 11, FN 1, precision 0.6071, recall
0.9444, Brier 0.22351. False positives were classified as vehicle-body regions
(3), signage (1), logos (1), background structures (1), and text-like positive-
scene extras/misaligned proposals (5). The sole false-negative is the occluded
plate; its returned region does not meet IoU 0.5.

- confidence sweep: 0.8 yields 0.8947 precision/0.9444 recall; 0.9 yields 1.0
  precision/0.5 recall—no threshold passes both targets;
- NMS at 0.3/0.5/0.7: no change;
- 640-long-side input: precision 0.85, recall 0.9444, Brier 0.12537, P95
  546.496 ms, RSS 550.223 MiB—meaningful improvement but still FAIL;
- 1280-long-side input: precision 0.6071, recall 0.9444, Brier 0.22293, P95
  933.03 ms—regression to baseline behavior.

No geometry or OCR-format filter was used to hide detector errors. The generic
text detector is classified for replacement by a specialized plate detector;
the 640 result remains diagnostic evidence only.

## OCR analysis and bounded ablation

Paddle English and Arabic were each run at baseline plus exactly one scale2x and
one contrast change. Neither ablation changed exact accuracy or CER.

| Candidate/variant | Exact | CER | Brier | P95 ms | Decision |
| --- | ---: | ---: | ---: | ---: | --- |
| Paddle English baseline | 0.875 | 0.14706 | 0.07993 | 130.184 | blocked |
| Paddle English scale2x | 0.875 | 0.14706 | 0.08460 | 106.968 | blocked |
| Paddle English contrast | 0.875 | 0.14706 | 0.07537 | 151.154 | blocked |
| Paddle Arabic baseline | 0.875 | 0.06863 | 0.07340 | 154.568 | blocked |
| Paddle Arabic scale2x | 0.875 | 0.06863 | 0.07335 | 130.632 | blocked |
| Paddle Arabic contrast | 0.875 | 0.06863 | 0.07073 | 198.145 | blocked |

Both Paddle models miss the occluded Latin crop (`ICT-006` becomes raw
`ICT-O`), deleting two zeros and confusing 6/O. English misreads the Urdu crop
as Latin `CATU3`. Arabic preserves the script family better but confuses
alef-with-madda/alef-with-hamza, drops Eastern Persian 7 and maps Eastern
Persian 8/6 to Arabic-Indic 8/6. Raw results, normalization steps, confidence and
abstention remain separately recorded. Tesseract remains a rejected primary
candidate and a diagnostic baseline.

## Narrow alternative shortlist

1. Open Model Zoo `vehicle-license-plate-detection-barrier-0123`: Apache-2.0,
   0.547M parameters, 256x256, CPU/OpenVINO viable; Chinese/front-barrier domain
   and 96-pixel minimum plate require T2/T3 validation. No download.
2. Open Model Zoo `vehicle-license-plate-detection-barrier-0106`: 0.634M
   parameters, 300x300, CPU/OpenVINO viable; same domain limitations and artifact
   notice capture required. No download.
3. P-LPCD-author YOLOv11 character-recognition approach: Pakistan relevant but
   no verified reusable weights and the dataset/license conflict blocks work.
4. Open Model Zoo recognition `barrier-0001` was rejected before download:
   Chinese blue-plate domain and publisher 88.58% correct-read ratio are below
   the NexusAI 0.90 gate.

## Preservation and next gate

All inference ran in the existing evaluator with networking disabled and cached
models. Production container identities remained unchanged. No evidence ingest,
model/backend/dataset download, production build, role assignment or threshold
change occurred.

The genuine next dependency is physical T2 capture plus clarification from the
P-LPCD publisher on CC BY versus CC BY-NC. R8.5 remains blocked.
