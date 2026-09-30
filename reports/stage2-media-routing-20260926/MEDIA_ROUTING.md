# MEDIA FAMILY ROUTING — the one change that makes Stage 1 reachable

Landed 2026-09-26 behind `FORENSIC_MEDIA_FAMILY_ROUTING`, default OFF.
Source-only; nothing deployed.

## Why this and not ladder deletion

The live run proved media questions never reach the executor: **0 of 28**,
identical verdicts AND routes with derived execution on and off. The queued fix
was "delete the ladder" -- 9,041 lines that measured **3 confident-wrong and 3
correct lost** when switched off. That is a multi-session project with a real
chance of going sideways, and it is not the smallest change that unblocks media.

The actual cause is one line:

    func semanticQuestionFamily(question string) string {
        return capabilityFamilyForRecordType(extractCanonicalRecordType(question))
    }

Family resolution goes through the STRUCTURED record types, so a media family is
unreachable. "How many plate groups used persistent object tracking?" resolved
`anpr_vehicles` -- because "plate" names the ANPR RECORD TYPE -- and answered
**"There are 1,057 ANPR sightings in this case"** to a question about 24 model
plate groups.

## The change

A question may resolve to a DERIVED family by asking the CURATED LAYER whether it
names one, and the ladder ABSTAINS when it does. The ladder abstaining is already
a designed outcome (`return ""`, and 39 of 62 questions reach the compiler that
way), so no new state is introduced.

**IT ADDS NO VOCABULARY OF ITS OWN.** It reuses the phrase matcher that already
binds curated values, so there is one vocabulary rather than two that can
disagree. Widening coverage is then a synonym in a YAML file, not a code change.

Three safety properties, each load-bearing:

    MULTI-WORD ONLY     the structured families own the single words -- "plate",
                        "sighting", "camera" -- and single-word phrases are
                        dropped in CODE, not merely assumed absent from the
                        layer, because the layer is curated by another track
    EVERY WORD PRESENT  "video plate group" needs video AND plate AND group, so
                        "how many ANPR sightings" cannot reach it
    AMBIGUITY CLARIFIES two matching families means the question does not
                        establish its evidence; picking one is a confident
                        answer about possibly the wrong thing

## STEMMING WAS THE DIFFERENCE BETWEEN 4% AND USEFUL

First implementation used the existing exact-token matcher and reached **1 of
28**. "faces were detected" cannot match "face detection"; "plate reads" cannot
match "plate read".

**That is the H7 defect in a new place** -- multi-word declared synonyms never
bound because one matcher compared raw words while everything else stemmed -- and
its recorded fix is the precedent: stem, but only for MULTI-WORD phrases, because
stemming a single word once made "calls" bind `CALL` and would have supplied a
filter to five correct questions.

With stemming: **7 of 28**, and every safety test still passes.

    M1  -> anpr_model_observation      M10 -> image_ocr_observation
    M5  -> video_anpr_plate_group      M12 -> image_ocr_observation
    M9  -> image_ocr_observation       M16 -> face_model_observation
                                       M17 -> face_model_observation

Coverage is now a CURATION number, visible in `TestMediaFamilyRoutingCensus`, and
closed by adding synonyms rather than changing code.

## A REGRESSION THIS CAUSED, AND THE TEST THAT NOW CATCHES IT

Enabling the switch broke **four ACCEPTED routings** -- caught not by the 62 or
the held-out sets, but by the query-variant ledger, which holds every accepted
routing in the product:

    "Find cited OCR observations in images for {target}"
        image_ocr_search          <- claimed by image_ocr_observation ("image ocr")
    "Show face candidate observations in this image." (+2 variants)
        face_candidate_observations <- claimed by face_model_observation ("face candidate")

**A working capability must not be broken to gain an unproven one.** The ledger
was regenerated with the switch OFF so no regression is baked into an
accepted-sources contract, and
`TestMediaFamilyRoutingNeverClaimsAnAcceptedVariantRouting` now fails on any NEW
collision while recording these four as owed curation.

**This is a DESIGN question, not a typo.** `image_ocr_search` and
`face_candidate_observations` RETRIEVE cited artifacts; the new entities
AGGREGATE over them. Both are legitimate and answer different questions, so the
vocabulary must separate retrieval from algebra -- "image ocr" and "face
candidate" name neither exclusively.

## State

    suite, switch OFF   GREEN
    suite, switch ON    one failure: the query-variant ledger goes stale,
                        because the switch genuinely changes accepted routings

**That failing test IS the gate.** The switch is not enabled until curation
separates the four colliding phrases. Nothing is deployed.

## What is owed, smallest first

    1  CURATION (Codex): narrow "image ocr" and "face candidate" so they do not
       claim a retrieval template's queries. Four collisions, two entities.
    2  CURATION (Codex): synonyms for the 15 media questions that resolve no
       family -- "plate group" without "video", "audio segment", "fingerprint",
       "perceptual hash", "script family", "text region".
    3  BACKEND (me): deploy with a control, measure the media set, and read it
       against MEDIA_EXECUTION_RULE.md. Expect partial coverage; that is the
       honest starting point, not a failure.

---

# THE COLLISIONS ARE RESOLVED IN CODE, NOT IN CURATION

`mediaFamilyAlgebraicGoal`: a media ENTITY may claim a question only when its
goal is a COMPUTATION -- `aggregate`, `breakdown`, `rank`, `distinct`, `range`.

`lookup`, `search` and `extract` are excluded. `lookup` is also the UNRECOGNISED
default, so admitting it would hand every unparsed question naming a media phrase
to the compiler, and "show face candidate observations" is exactly that shape.

    all 4 accepted-routing collisions   RESOLVED
    media routing coverage              7 of 28, UNCHANGED
    suite, switch OFF                   green
    suite, switch ON                    green  <- the gate is cleared

This is better than narrowing the synonyms, which was the first plan. "image ocr"
and "face candidate" are the RIGHT phrases for both surfaces; the real distinction
is whether the question computes or retrieves, and that was already available as
the frame goal. No curation dependency, and Codex's vocabulary stays intact.

**The allowlist in the collision test is now EMPTY and must stay empty.** A
non-empty list means a collision is being tolerated.

---

# LIVE RESULT — the media path WORKS, and it produced two defects of mine

Control and measurement on the same container, config asserted for both phases.
Both switches flipped TOGETHER, because routing without execution only routes a
question to be refused, and execution alone was already proven inert.

    golden               45 · 2 NOT_STATED · 14 CLAR · 1 MANUAL   UNCHANGED
    structured held-out  10 · 2 CLARIFIED · 1 WRONG               UNCHANGED
    media  control       5 CORRECT · 7 WRONG · 15 CLAR · 1 ROWCOUNT
    media  ON            6 CORRECT · 6 WRONG · 14 CLAR · 1 ROWCOUNT · 1 ERROR

## THE FIRST MEDIA QUESTION EVER ANSWERED FROM A TYPED PLAN

    M17  "What is the average face detection confidence?"
         CLARIFIED -> CORRECT
         "The average face-region detection confidence across records is 0.79."

**Proven to have read derived_artifacts, not records:**

    RECORDS  avg detection_confidence (307 anpr rows)   0.552413
    DERIVED  face avg detection_confidence               0.789580
    M17 answered                                         0.79

The structured table could not have produced 0.79. `route: records_sql` is a
MISLABEL, which is defect 2 below.

## DEFECT 1 — I INTRODUCED AN HTTP 500. Fixed.

    M16  "How many faces were detected in the evidence?"   WRONG -> ERROR, http 500
         "a plan that names no field cannot be scoped: plan mixes evidence
          sources: face_model_observation reads forensic.derived_artifacts and
          records reads forensic.records"

That is MY mixed-source guard firing on a good question. A COUNT(*) names no
field, and the ISSUED CATALOGUE legitimately mixes curated media fields with
INFERRED forensic.records fields. **The Phase 1 gate is ZERO HTTP 500s.**

Fixed by ordering the binding precedence explicitly:

    1  the PLAN's fields        authoritative -- they name what is read
    2  the ISSUED CATALOGUE     it BOUNDED the plan
    3  the QUESTION's family    TIEBREAKER only, when the catalogue is mixed
    4  records                  never an error; ambiguity is declined downstream

**The first attempt at this fix put the question's family FIRST and broke the
9,580 regression test.** `semanticQuestionFamily` comes from the STRUCTURED
record-type extractor, so "across the video frames" resolves a structured family
and overrode a media catalogue. The catalogue is the stronger signal; the family
is the tiebreaker. Both orderings pass a naive reading, and only the test
distinguished them.

Also fixed: a mixed, unscopeable COUNT now falls back to records rather than
erroring. Erroring was an HTTP 500 for a question the product should decline.

## DEFECT 2 — A DERIVED OBSERVATION IS CITED AS `records_sql`. NOT fixed.

M17's provenance reads `"source": "records_sql"` with `record_type`, `row_hash`,
`row_number` and `source_file` all NULL -- the records columns a derived row does
not have. The label is hardcoded in ~8 places in `query.go`, independent of the
lineage expression this work added.

**This matters more than it looks.** The entire "a model's plate read is not a
camera's sighting" property depends on the citation saying which it is, and
`source_truth_state` is not surfaced at all. It is recorded here rather than
bundled into this slice so the measurement stays attributable.

## HONEST READING

One question converted. The other six routed media questions still do not reach
the executor -- they are withheld by verified-only or clarified upstream, which is
SAFE but not yet useful. Coverage is 7 of 28 routed and 1 of 28 answered.

Structured suites did not move, so nothing was traded away for it. That is the
bar this slice had to clear, and it cleared it.
