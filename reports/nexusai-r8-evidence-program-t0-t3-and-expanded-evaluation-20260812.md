# NexusAI R8 evidence program and expanded model evaluation

Date: 2026-08-12  
Disposition: T0/T1 source and offline evaluation complete; T2 protocol ready;
T3 research complete without download; T4 pending; **NO PROMOTION**

## Verified current state

R0-R7 remain accepted and frozen. R8 remains active at R8.3/R8.4 acceptance
closure. The existing UI/API deployment was not rebuilt. Production API, worker
and LocalAI container identities remained unchanged during the expanded offline
evaluation. No evidence was ingested and no production model role was assigned.

## Evidence program implementation

- One authoritative registry now records T0-T4 evidence:
  `configuration/nexusai_test_dataset_registry.json`.
- The durable contract, storage rules, acceptance terminology, T2 capture
  protocol, T3 dispositions and proposed-but-unapproved gate refinement are in
  `docs/data/nexusai-evidence-test-and-demo-program.md`.
- The registry validator passes and enforces required fields, unique IDs, tier
  values, official URLs for public candidates and non-Git T4 storage.
- Future R9-R17 evidence remains just-in-time; no future-family data project was
  started.

## R8 T0/T1 fixture result

Generator `nexusai-r8-synthetic-v2`, seed
`nexusai-r8-t0-t1-20260812`, produces 29 non-personal fixtures: 6 T0 and 23 T1.
Manifest SHA-256 is
`e847542ad12f4eec0731e66f9f5361affbda310e24b2f479b18a6d3226ef24f3`.
Repeated generation is byte-for-byte deterministic.

All 14 required classes are present. Safe model lanes include difficult
perspective, scale, border, blur, glare, night, exposure, compression,
occlusion, multiple-plate and Urdu/English cases; six negative detector scenes;
and a negative OCR-abstention crop. Four benign hostile fixtures cover malformed,
oversized, extension/signature mismatch and trailing/polyglot input. Those four
are admission-only and are never opened by detector/OCR code.

## Expanded offline evaluation

| Candidate | Gate | Key accuracy | Calibration/abstention | Resource result |
| --- | --- | --- | --- | --- |
| Paddle text-detector adapter | FAIL | precision 0.6071, recall 0.9444; TP 17, FP 11, FN 1 | Brier 0.22351 | P95 940.336 ms; 649.848 MiB |
| Paddle Arabic-script OCR | FAIL | normalized exact 0.8750; CER 0.06863 | Brier 0.07340; abstention precision 1.0 | P95 184.602 ms; 448.117 MiB |
| Paddle English OCR | FAIL | normalized exact 0.8750; CER 0.14706 | Brier approximately 0.0799; abstention precision 1.0 | bounded CPU result; below latency/RSS limits |
| Tesseract `eng+urd` | FAIL | normalized exact 0.5000; CER 0.31373 | Brier 0.21061; false recognition on blank crop | P95 241.767 ms; 19.105 MiB |

The text detector fires on vehicle body details, signboards, rectangular
objects, logos and overexposed text regions. It is rejected as a plate detector
candidate. Paddle Arabic is the strongest current recognizer but still misses
the 0.90 exact and 0.05 CER thresholds. Thresholds were not weakened.

Corrected review bundle:
`reports/runtime-evaluation-r8/r8-model-evaluation-review-t0-t1-v2-20260812.json`
with SHA-256
`5698f312b0a44381dd3e66ca79a816a4547b0a8a494b80a51b54fbfc54d1e62c`.

## License and public benchmark research

Installed PaddleOCR/PaddlePaddle and Tesseract/Tessdata notices were inspected
offline and recorded in `tools/r8_ocr_eval/THIRD_PARTY_NOTICES.md`. Evaluation
licenses are verified; a full transitive redistribution inventory remains a
future production-packaging gate.

No public dataset was downloaded:

- CCPD 2019 is a large MIT-claimed adverse-condition benchmark, but requires
  privacy/download review and has no Pakistan agreement value.
- UFPR-ALPR has 4,500 moving-camera images and strong annotations, but its
  academic/non-commercial restriction makes it evaluation-only.
- Pakistan-relevant P-LPCD reports 40,000 synthetic and 650 real Punjab crops,
  but its Zenodo dataset license, privacy provenance, size and hashes require
  verification; the article's CC BY license is not treated as dataset proof.

## Acceptance dimensions

- `synthetic_accepted`: false for model promotion; fixture source/reproducibility
  accepted, candidates fail.
- `controlled_demo_accepted`: false; protocol ready, assets pending.
- `public_benchmark_accepted`: false; research only, no download.
- `authorized_real_data_accepted`: false; T4 pending.
- `runtime_accepted`: false for image inference; existing metadata UI/API remains
  live accepted.
- `production_promoted`: false.

## Gate disposition and next action

The current real-image gate remains T4. T2/T3 are complementary and do not
silently replace it. R8.5 remains blocked. The next bounded decision is whether
to approve the documented gate refinement allowing R8.5 **source-only** after
T0-T3 acceptance while retaining T4 for production promotion/runtime activation
and R8 closure. Separately, a public benchmark download choice requires explicit
approval and an operator handoff. Until then, tune preprocessing/crop handling
against the owned T1 pack and research a genuinely specialized plate detector;
do not add models blindly.

## Tests

- dataset registry validation: PASS;
- fixture generator determinism/lane safety: 1/1 PASS;
- scorer role/normalization/calibration/lane tests: 5/5 PASS;
- Python compilation: PASS;
- forensic API package, including hostile image intake: PASS;
- expanded offline candidates: 4/4 executed; all return governed FAIL/no
  promotion rather than an inferred pass.
