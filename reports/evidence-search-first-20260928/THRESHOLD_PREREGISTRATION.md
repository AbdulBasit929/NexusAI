# U2a EVIDENCE SEARCH FIRST -- THRESHOLDS

Written 2026-09-28, BEFORE the switch was built into an image or any arm was scored.

## The defect (deployed build, 2026-09-28)

    "Find OCR text mentioning SINDH"     -> "DSC_1105.JPG contains the text SINDH."      works
    "Find OCR text mentioning BCX-567"   -> withheld, "matched a different kind of evidence"
    "Find OCR text mentioning MNA-08"    -> withheld, same
    "Search image OCR for BCX-567"       -> withheld, same
    DOC-01 "Search the case documents for mentions of plate MN1367" -> withheld, same (CLARIFIED)

Traced: the keyword router chose `image_ocr_search` (and `document_search` for DOC-01) in every case.
- Gate 1: for a plate-shaped term, the one-identifier hybrid shortcut then overrode the choice with
  `cross_family_correlation`.
- Gate 2: for DOC-01, the word "plate" gave the question the ANPR family, so verified-only demanded a
  typed plan and arbitration swapped in an ANPR records plan.

The media misroute guard then withheld the result, correctly. The withhold's own clarification option
for images, "Find OCR text mentioning %s", therefore also failed for any plate-shaped term.

## Census (offline, the router's own functions over all 103 corpus questions)

    gate 1  derived-text ladder choice AND one hybrid tuple AND the question names that evidence:
            0 corpus questions answered through the shortcut (DOC-03/DOC-06 have one tuple but are
            answered by document_search today). Pre-flight: the 3 plate-in-OCR questions.
    gate 2  derived-text ladder choice AND verifiedOnlyWithholds: exactly DOC-01.

## Mechanism

`FORENSIC_EVIDENCE_SEARCH_FIRST`, code default OFF, compose default `false` until measured. When the
chosen template is `image_ocr_search`, `document_search` or `audio_transcript_search`, and the question
names that same evidence (the `mediaEvidenceScopes` keywords, word-bounded), then (1) the hybrid
shortcut does not override it, and (2) verified-only does not withhold it. The search runs where the
question asked, as cited retrieval.

## Arms (switch asserted from `docker inspect`)

    off   deployed posture, switch OFF (must reproduce reports/absence-text-20260928/on)
    on    deployed posture, switch ON
    Each: corpus, 14 everyday, 38-check pre-flight, 16 relational.

## Pass criteria

    corpus      DOC-01 CLARIFIED -> CORRECT (names nexusai-multimodal-acceptance-brief.pdf).
                No other verdict moves. H1 and VID-01 (plate-text privacy probes) unchanged.
    pre-flight  "Find OCR text mentioning BCX-567" and "Search image OCR for BCX-567" -> DSC_1105.JPG,
                "Find OCR text mentioning MNA-08" -> DSC_0990.JPG. The other 35 unchanged in pass/fail
                and text. The two plate-reader searches (MN1367, LEB15491) are expected to STILL fail:
                they are U2b.
    everyday    14 identical     relational  16 identical

Fail any line -> the switch stays OFF and the result is recorded.
