# NexusAI R8.3/R8.4 measured evaluation decision

Date: 2026-08-12  
Decision: **NO PROMOTION**  
Production mutation: none

## Execution evidence

The operator-approved isolated evaluation completed successfully. The pinned
`nexusai/r8-ocr-eval:3.7.0-cpu` image downloaded official
`PP-OCRv5_mobile_det`, `en_PP-OCRv5_mobile_rec`, and
`arabic_PP-OCRv5_mobile_rec` assets into the dedicated
`nexusai_r8_paddlex_model_cache` volume. Tesseract `eng+urd` and all Paddle runs
then executed with networking disabled. The retained bundle is
`reports/runtime-evaluation-r8/r8-model-evaluation-20260812T163225Z.json`.
The corrected review bundle is
`reports/runtime-evaluation-r8/r8-model-evaluation-review-20260812T163225Z.json`
(SHA-256 `94e365a918c88e67e8e6439d91a29c1cefd1e76d1182dacb945a108798e9274e`).
The forensic API, worker, and LocalAI production container identities remained
unchanged, and no model role was assigned.

## Corrected scoring decision

The v1 score was useful as raw evidence but was not promotion-safe. It compared
detector confidence against absent OCR text, treated OCR raw equality as the
only exact metric, included a zero-latency OCR no-op, and did not enforce the
required fixture taxonomy. The v2 scorer corrects those semantics:

- detector calibration is candidate-to-ground-truth IoU outcome;
- OCR reports raw and deterministic normalized exactness/CER separately;
- normalization is NFKC + uppercase + retained alphanumerics and never fills a
  missing character;
- role-inapplicable metrics remain explicitly not applicable;
- zero-time no-op entries are excluded from latency percentiles;
- peak process RSS and read-only installed candidate asset size are retained;
- all 14 required fixture classes and explicit numerical thresholds are gated.

The saved predictions were rescored locally without inference or downloads:

| Candidate | Governed result | Accuracy / localization | CER / calibration | P95 / peak RSS |
| --- | --- | --- | --- | --- |
| Paddle detector | blocked | precision 1.000, recall 1.000 on 7 positive synthetic scenes | Brier 0.00753 | 640.436 ms / 652.285 MiB |
| Paddle Arabic-script OCR | blocked | normalized exact 0.7143 (5/7) | normalized CER 0.14583; Brier 0.16614 | 162.530 ms / 447.223 MiB |
| Paddle English OCR | blocked | normalized exact 0.7143 (5/7) | normalized CER 0.31250; Brier 0.18765 | 165.276 ms / 451.395 MiB |
| Tesseract `eng+urd` | blocked | normalized exact 0.4286 (3/7) | normalized CER 0.16667; Brier 0.45797 | 211.874 ms / 19.055 MiB |

The detector's numbers are a promising synthetic baseline, not acceptance. The
pack covers only 8/14 required classes. Missing classes are
`multiple_vehicles`, `pre_cropped_plate`, `malformed_image`,
`oversized_dimensions`, `extension_signature_mismatch`, and
`polyglot_or_trailing_payload`. The evaluator image is 537.076 MiB; candidate
assets measure 4.707 MiB (detector), 7.633 MiB (English), 7.781 MiB (Arabic),
and 15.477 MiB (Tesseract runtime/language data). No authorized real-image
agreement pack exists. All three OCR candidates fail
the normalized exactness, normalized CER, and calibration thresholds.

## Gate disposition

- R8.3 contract, crop provenance, overlay, and preliminary synthetic detector
  measurement are complete; detector accuracy acceptance remains blocked.
- R8.4 installation and first measured comparison are complete; no detector or
  OCR candidate is approved for production.
- R8.5 must not start until the 14-class pack is complete, package/license
  evidence is recorded, an explicitly authorized real-image pack agrees with
  synthetic results, and one detector/OCR configuration passes every gate.
- Existing production services, evidence, profiles, models, databases, and
  named production volumes require no rebuild or change for this decision.

## Reproducibility

Corrected score hashes:

- Paddle detector: `7ca725c914825f999a74674f9c675576ef5f28532f9839089b5f98c33392383a`
- Paddle Arabic: `8af2b37918afc7baa48aaffd324cff7a83b4603614779f035be8db6e2683317a`
- Paddle English: `72d9af9654077068bd711e03707288cdd421c02e63134ce04c74eb0adfe19c97`
- Tesseract: `6ae1f4303aaa6e3ffc2de92f0e714b13edc529c42dd311f48e31a690e8e0f7e9`

Focused scorer tests: 4/4 pass. JSON approval-gate parsing and all four v2
rescoring invocations pass.
