# NX-MMR ANPR/OCR vertical manual acceptance

Status: post-activation plan only. Do not execute until the exact activation is
separately approved and completed. Use fresh, operator-authorized test files;
do not reprocess retained evidence or use a sealed benchmark item.

## Preconditions

- Record worker/API image and container IDs, health, active jobs, named volumes,
  retained tuple, Activity count, and model-tree hashes.
- Confirm the V2 source freeze digest is
  `212c00970d7bed1c40612102c11f1a0dd24c43435c406df83d80bd8f40117b20`.
- Confirm `FORENSIC_ASR_ENABLED=false`, face/image-semantic roles remain off,
  and visible suggestions remain restricted to `PRODUCT_CERTIFIED` operations.
- Use one fresh positive and one fresh negative per applicable path. Record the
  evidence/version IDs and remove nothing after the test.

## Image ANPR

1. In Add Data, upload a known non-holdout local car image.
2. Verify automatic Image classification and a visible processing state.
3. Open Data detail and verify the original image, bounded plate finding and
   reconstructable crop.
4. Verify raw and normalized candidate text are distinguishable.
5. Verify detector and OCR values are labelled confidence, never accuracy.
6. Verify the result is a model observation with review recommended.
7. Ask an unseen, evidence-scoped question such as “What plate candidates were
   found in this image?” and its Roman Urdu equivalent.
8. Open the citation and confirm it returns to the same image and plate region.
9. Upload a generated/authorized negative image and verify
   `COMPLETE_ZERO_RESULTS`, not `MODEL_REQUIRED` or a ghost result panel.

## Video ANPR V2

1. In Add Data, upload the authorized `sample.mp4` fresh acceptance copy.
2. Verify automatic Video classification and bounded processing.
3. Open Data and verify actual-frame plate observations and source-time groups.
4. Confirm no persistent vehicle ID, journey, tracking, or interpolated
   observation is shown.
5. Click a group citation and verify the player seeks to the cited source time.
6. Ask which candidate plates were observed, then ask for one plate occurrence
   and for observations in an unseen time range.
7. Verify every answer uses the same scoped groups shown in Data and carries a
   clickable video/time citation and model-observation limitation.
8. Record elapsed time, peak worker memory, CPU, failed frames, analyzed frames,
   OCR crop count, partial-sampling flag, and result state.

## Printed Urdu OCR

1. Upload a fresh printed-Urdu image with a known identifier.
2. Verify OCR processing, region boxes, Unicode logical order, and RTL display.
3. Open a region citation and confirm it resolves to the same source region.
4. Ask an unseen Urdu-script question, then a semantically equivalent Roman
   Urdu question.
5. Confirm numbers, dates, phone/reference identifiers, and punctuation remain
   readable and are not reversed or silently substituted.

## Printed mixed Urdu/English OCR

1. Upload a fresh mixed-script image containing English, Urdu, and an identifier.
2. Verify both scripts, region order, per-region direction, and raw text.
3. Search one unseen English term and one unseen Urdu term.
4. Ask for the source of the identifier and open its region citation.
5. Confirm the UI does not reverse identifiers or claim handwriting support.

## Failure-state and governance checks

- With the model preflight pointed at a deliberately absent test path, verify
  `MODEL_REQUIRED` is returned before job submission; do not alter live mounts.
- An enabled processor with a backend/hash failure must be `UNAVAILABLE`.
- A successful run with no observations must be `COMPLETE_ZERO_RESULTS`.
- Data and Ask must remain evidence/version scoped and must not expose raw
  embeddings, unrestricted SQL, model-output-as-oracle, or benchmark answers.
- No suggestion becomes visible and no operation/model becomes
  `PRODUCT_CERTIFIED` from this run alone.

## Pass record and rollback trigger

The pass record must include fresh evidence/version IDs, operation IDs, result
states, citation targets, screenshots at 390/820/1024/1440 widths, browser
console status, resource receipt, health checks, retained tuple, Activity delta,
and protected container/volume identities. Any health, model hash, contract,
scope, citation, resource, retained-invariant, or UI failure triggers the exact
rollback in `configuration/nxmmr_anpr_ocr_vertical_activation_v1.json` and
blocks promotion.
