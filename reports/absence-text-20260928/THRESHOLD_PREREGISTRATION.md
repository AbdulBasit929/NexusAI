# S2 + S3 -- STATEMENTS OF ABSENCE THAT WERE NOT FINDINGS -- THRESHOLDS

Written 2026-09-28, BEFORE either switch was built into an image or any arm was scored.

## S2: a word search that found nothing, reported as absence

P2 *"Were any of the transcribed audio segments taken from video?"* (WRONG in every recorded arm)

    route      audio_transcript_search, relevance mode ("source"), no exact term
    result     NO_MATCH over 18 candidate observations
    headline   "No transcript segment intersects the requested source-time range."
    truth      one of the 11 segments DID come from a video's audio track

Two defects. The sentence describes a time-range search that was never requested; it is the headline
`enterpriseExecutiveAnswer` maps to `NO_MATCH` for this template, whatever the search mode. And a
relevance search for the question's own words that returns nothing cannot establish that no segment
came from video.

**Mechanism:** `FORENSIC_TEXT_SEARCH_NOT_ABSENCE`. In `withholdPostExecution`, a transcript, image-text
or document search in relevance mode (no exact term, not time-range) that returns `NO_MATCH` is
withheld with the new code `text_search_not_absence`. The sentence says what was searched and that a
word search cannot show absence. Exact-phrase and time-range searches are untouched: their absence
statements are scoped to what was searched, and they are correct.

## S3: a clarification whose text states a result

R5 *"What do the two busiest numbers have in common?"* (relational probe)

    route      clarification, reason missing_required_parameter (nothing was run)
    shown      "No qualifying phone-counterparty events were found for the selected target in the
                exact source set; no relationship was inferred."
    asked      "Which exact MSISDN and two to eight exact CDR source identities should I compare?"

`buildEnterprisePayload` sends every `multi_cdr_comparison` response, clarification included, to the
comparison's own payload builder. That builder sees zero rows and writes a no-results sentence.

**Mechanism:** `FORENSIC_CLARIFICATION_NOT_RESULT`. A clarifying response never reaches a
result-specific payload builder (`multi_cdr_comparison` or composition); it takes the generic path,
which states the clarification question.

## Census (recorded responses, deployed build + probes)

    S2  empty retrieval results        2: P2 (relevance, NO_MATCH) -> in scope
                                          DOC-06 (exact phrase, NO_EXACT_MATCH, CORRECT) -> out of scope
    S3  clarification text != question  1: R5

## Arms (switches asserted from `docker inspect`)

    off  deployed posture, both switches OFF (must reproduce reports/derived-labels-20260928/on)
    on   deployed posture, both switches ON (the configuration that would ship)
    Each: 103-question corpus, 14 everyday questions, 38-check pre-flight, 16 relational probes.

The effects are disjoint by census (one answer each, different code paths). If anything other than
P2 and R5 moves, the switches are measured separately before any decision.

## Pass criteria

    corpus      P2 WRONG -> not WRONG (withheld, reason text_search_not_absence). No other verdict moves.
                DOC-06 unchanged in verdict and text.
    relational  R5: shown text is the clarification question; no "were found" sentence.
                All other 15 unchanged.
    pre-flight  38 of 38 identical in pass/fail and text
    everyday 14 identical

Fail any line -> the switches stay OFF and the result is recorded.
