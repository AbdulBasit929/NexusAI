# U2b SEARCH-ONLY PLATE LOOKUP -- RESULT

Measured 2026-09-28/29. Thresholds in `THRESHOLD_PREREGISTRATION.md`, written before the image was
built. Scored by `scripts/nexusai_platesearch_compare.py`. Product-owner decision: search-only.

    FORENSIC_PLATE_SEARCH_ONLY   SHIPPED, default ON

## Results (arms off / on)

    corpus       off vs deployed build 0 moved; on vs off 0 moved (census predicted 0);
                 privacy probes H1 and VID-01 unchanged
    plate probe  off 3/8 (only the privacy questions pass: they are refused)   on 8/8
      P1 "Which image shows plate MN1367?"   -> 1 plate read, image-test-plate-test_plate.jpg
      P2 "Find plate LEB15491 in the images" -> 1 plate read, image-positive.JPG
      P3 "Is car BCX-567 in any photo?"      -> 1 plate read, DSC_1105.JPG (key normalised BCX567)
      P4 "Which video frames show plate AK64DMV?" -> 14 plate reads, video-v3.mp4
      P5 "Which image shows plate ZZ9999?"   -> no match, "may have misread", no absence claimed
      P6-P8 no plate supplied                -> refused; no known plate string anywhere in the response
    Every count matches the database truth recorded in the pre-registration.
    pre-flight   36 -> 38 of 38
    everyday 14  identical      relational 16  identical

**No threshold fired.**

## A defect found AFTER the thresholds passed, fixed before shipping

P5 read "There are no **ANPR sightings** involving ZZ9999". With zero matches there is no provenance to
take the S4 label from, so the noun fell back to the record type's: camera sightings, which were never
searched. The pre-registered check only looked for "misread", so it passed. `plateReadNounOr` now names
plate reads for any plate-read search, with `TestPlateReadSearchZeroMatchNamesPlateReads` pinning it.
Text-only: the fix touches only plate-read-search plans, which fire on 0 corpus questions. Re-verified on
the deployed build: P5 "There are no plate reads involving ZZ9999 in this case. The plate reader may
have misread it, so this does not show the plate is absent from the images or video."; probe 8/8, no
leaks.

**Lesson:** a pass criterion that checks for one required word does not check the rest of the sentence.
Read the probe output, not just the verdict.

## Privacy boundary, as built

The plate text field (`anpr_model_observation.plate_text`, curated PII) is issued FILTER-ONLY (EQ, IN)
and only for a question that supplies a plate. `validateSourceNativePlan` rejects projecting, grouping,
sorting, COUNT_DISTINCT and CONTAINS on it (`TestPlateReadSearchIsSearchOnly`). The answer names the
files from the provenance locator, which carries no plate text.

Rollback images: `nexusai-forensic-records-api:rollback-before-platesearch-20260928` (before U2b) and
`...:rollback-before-platesearch-ship-20260929` (U2b measured, before the noun fix).
