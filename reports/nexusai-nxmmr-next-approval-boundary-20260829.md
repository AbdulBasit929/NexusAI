# NX-MMR next bounded approval boundary

Status: `PARITY_ADAPTER_V1_FROZEN_CANDIDATE_EVALUATION_APPROVAL_REQUIRED`
(2026-08-30).

The reference-parity investigation and
`NX-MMR-REFERENCE-PARITY-ADAPTER-V1` source-development slice are complete.
The model-output-independent split contains 11 development and 8 reserved
event components. Only development was used for A/B selection. Candidate
evaluation has not started.

The frozen candidate is fixed 4 FPS, plate-only, neutral normalized majority,
minimum support 2, and one vote per actual crop, using the pinned dedicated
640 detector and EasyOCR assets. Freeze digest:
`1dcfabf17a8b66165b6c1390e65a8784eb875d3650b1b7f27d328c9ad2f0bb13`.
Development event recall improved to 0.818182, but exact TP/FP/FN remains
2/12/22 and group TP/FP/FN 1/17/16, so the candidate is `PARTIAL` and is not
eligible for operation or product promotion.

## Exact next action

Explicitly authorize one and only one local, network-disabled, non-retained
evaluation of the exact frozen candidate on the 8 locked reserved event/
interval components. The run will compute event recall; exact plate precision,
recall and F1; CER; group precision, recall and F1; negative-frame FPR;
boundary mean/median/p95 error; wall/CPU/peak RAM; frames analyzed; and OCR
calls. It will compare the freeze with accepted incumbent and historical-final
predictions on the same reserved scope.

Resource scope is HEAVY with a 4.5 GiB free-RAM gate, one run at a time,
approximately 181 seconds expected duration and 1.2 GiB expected process RSS.
The run must not alter Activity, database, volumes, retained evidence, live
models, services, routing, or certification. It must not include tuning,
another candidate, image-holdout rescoring, external evidence, downloads,
fine-tuning, deployment, or NX-B2.1.

The broader independent multi-video/negative/boxed Pakistan ANPR and positive
human-transcribed ASR evidence pack remains a later generalization and product-
certification gap. Live API/Ask/Data/citation/Activity/UI/security/performance/
manual gates remain separate and mandatory.
