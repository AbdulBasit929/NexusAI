# S4 DERIVED RESULT LABELS -- THRESHOLDS

Written 2026-09-28, BEFORE the switch was built into an image or any arm was scored.

## The defect (recorded on the deployed build, reports/rowcount-headline-20260928/on)

    M1   "How many plate reads were produced from the images?"         -> "There are 307 ANPR sightings in this case."
    M2   "What is the average OCR confidence of the plate reads?"      -> "... across ANPR sightings is 0.96."
    M5   "How many plate groups were tracked across the video frames?" -> "There are 24 ANPR sightings in this case."
    M9   "How many text regions were read from the images?"           -> "There are 309 records in this case."
    M13  "How many audio segments were transcribed?"                  -> "There are 11 records in this case."
    M16  "How many faces were detected in the evidence?"              -> "There are 20 records in this case."
    M17  "What is the average face detection confidence?"             -> "... across records is 0.79."
    M19  "How many image fingerprints were computed?"                 -> "There are 22 records in this case."

The numbers are correct and the nouns are wrong. The same case holds 1,057 CAMERA sightings, so "307
ANPR sightings" names a different kind of evidence from the one counted. The sentence noun comes from
the request's record type. A derived-media plan carries the structured family's type ("anpr") or none.

## Census

Answers whose cited provenance is exactly one derived artifact type (deployed build, 103 questions):
**8 -- exactly the eight above.** All eight are scored `check: number`, so a label change cannot
change a verdict.

## Mechanism

`FORENSIC_DERIVED_RESULT_LABELS`, code default OFF, compose default `false` until measured. The
headline noun is taken from the cited `artifact_type` when every cited row agrees on one type
(plate reads, video plate groups, image text regions, detected faces, transcribed audio segments,
image fingerprints, ...). Structured, mixed or unknown provenance keeps today's noun.

## Arms

    off   deployed posture, switch OFF (must reproduce reports/rowcount-headline-20260928/on)
    on    deployed posture, switch ON
    Each: 103-question corpus, the 14 everyday questions, the 38-check pre-flight, the 16 relational probes.

## Pass criteria

    corpus      0 verdicts moved. Answer text changes on exactly the 8 census questions, and on no other.
                Each of the 8 names its own kind of evidence and states the same number as before.
    pre-flight  38 of 38 identical in pass/fail. Text may change only on "How many faces were
                detected" and "How many plate reads were produced from the images".
    everyday 14 identical text
    relational  16 of 16 identical
    P1          "How many ANPR sightings are in this case?" still says 1,057 ANPR sightings (structured,
                must not be relabelled)

Fail any line -> the switch stays OFF and the result is recorded.
