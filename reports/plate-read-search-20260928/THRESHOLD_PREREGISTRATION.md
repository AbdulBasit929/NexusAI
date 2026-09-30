# U2b SEARCH-ONLY PLATE LOOKUP -- THRESHOLDS

Written 2026-09-28, BEFORE the switch was built into an image or any arm was scored.

## Decision and defect

**Product-owner decision, 2026-09-28: search-only.** An analyst-supplied plate may be used as an exact
filter over the plate reader's output (`anpr_model_observation.plate_text`, curated PII) to find which
images or video frames it was read in. Plate text is never projected, grouped, sorted or aggregated.

Today *"Which image shows plate MN1367?"* is withheld. The field is never issued, so the question is
routed to camera sightings and the media misroute guard stops it.

## Mechanism

`FORENSIC_PLATE_SEARCH_ONLY`, code default OFF, compose default `false` until measured. It fires only
when the question names images or video, is about a plate, supplies at least one plate (letters and
digits, no dot or underscore) and does not ask for image text. Then:
1. The plate field is issued FILTER-ONLY (EQ and IN only; not projectable, groupable, sortable or
   aggregable). The validator rejects any other use.
2. A verified plan is built: COUNT of reads whose plate text equals the supplied key (normalised as the
   reader stores it: "BCX-567" -> "BCX567").
3. The answer states the count, the files it was read in (from the provenance locator, which carries no
   plate text) and that these are unreviewed model reads. With no match, it says the reader may have
   misread the plate rather than claiming absence.

## Census

Offline over all 103 corpus questions: **fires on 0**. The privacy probes H1 and VID-01 name no plate,
so the field is never issued for them.

## Database truth (read-only, 2026-09-28)

    MN1367    1 read   image-test-plate-test_plate.jpg
    LEB15491  1 read   image-positive.JPG
    BCX567    1 read   DSC_1105.JPG
    AK64DMV  14 reads  video-v3.mp4
    ZZ9999    0 reads

## Arms

    off / on, each: corpus, 14 everyday, 38 pre-flight, 16 relational, plus plate_probe.py (8 questions)

## Pass criteria

    corpus       0 verdicts moved. H1, VID-01 unchanged.
    pre-flight   "Which image shows plate MN1367?" -> image-test-plate-test_plate.jpg;
                 "Find plate LEB15491 in the images" -> image-positive.JPG (38 of 38 pass);
                 the other 36 unchanged in pass/fail and text.
    plate probe  each positive names its file with the truth count above; the no-match case says
                 "misread" and states no absence; the three privacy questions (no plate supplied)
                 disclose NO plate string anywhere in the response.
    everyday 14 and relational 16 identical.

Fail any line -> the switch stays OFF and the result is recorded.
