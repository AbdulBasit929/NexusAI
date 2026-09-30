# U2a EVIDENCE SEARCH FIRST -- RESULT

Measured 2026-09-28. Thresholds in `THRESHOLD_PREREGISTRATION.md`, written before the image was built.
Scored by `scripts/nexusai_evsearch_compare.py`.

    FORENSIC_EVIDENCE_SEARCH_FIRST   SHIPPED, default ON

## Results

    corpus       off vs deployed build 0 moved; on vs off exactly 1 moved:
                 DOC-01 CLARIFIED -> CORRECT
                   "nexusai-multimodal-acceptance-brief.pdf states: 'NexusAI Multimodal Acceptance Brief ...'"
                 privacy probes H1-MEDIA-WHICH-PLATES and VID-01 unchanged (still refused)
    pre-flight   33 -> 36 of 38
                 "Find OCR text mentioning BCX-567" -> "DSC_1105.JPG contains the text 'BCX-567'."
                 "Search image OCR for BCX-567"     -> "DSC_1105.JPG contains the text 'BCX-567'."
                 "Find OCR text mentioning MNA-08"  -> "DSC_0990.JPG contains the text 'MNA-08'."
                 the other 35 identical in pass/fail and text
    everyday 14  identical      relational 16  identical

**No threshold fired.**

## Still open: U2b (a plate the image-text reader did not read whole)

"Which image shows plate MN1367?" and "Find plate LEB15491 in the images" still fail, as predicted.
Those plates exist only in the plate reader's output (`anpr_model_observation.plate_text`), which the
curated layer marks `sensitivity: PII`. The query engine is never offered PII fields, by design.
Answering needs a product-owner decision: may an analyst-supplied plate be used as an exact filter
on that field, with the plate never projected, grouped or sorted?

Rollback image: `nexusai-forensic-records-api:rollback-before-evsearch-20260928`.
