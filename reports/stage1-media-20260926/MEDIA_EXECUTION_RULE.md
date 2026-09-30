# MEDIA EXECUTION — threshold declared BEFORE the run

First live measurement of any media capability. Everything so far is proven in
unit and database tests only.

## Switches under test

    FORENSIC_DERIVED_ARTIFACT_EXECUTION=true    typed plans may read derived_artifacts
    FORENSIC_MEASURE_DENOMINATOR=true           an average states what it averaged over

## Sequence, so each effect is attributable

    PHASE 1  build + deploy, BOTH switches OFF
    PHASE 2  CONTROL: golden 62 + structured held-out.
             The layer changed since the last control (Codex added
             audio source_modality in WI-LAYER-5), so the previous
             62/62 result does not carry over. This re-establishes it.
    PHASE 3  flip BOTH switches ON, restart, no rebuild
    PHASE 4  golden 62 + structured held-out + MEDIA held-out (28 questions)

## Baselines

    golden               45 CORRECT · 2 NOT_STATED · 14 CLARIFIED · 1 MANUAL
    structured held-out  10 CORRECT · 2 CLARIFIED · 1 WRONG
    media                no baseline. Nothing has ever answered one of these.

## THRESHOLD

**On the structured suites — REVERT on any movement.** They are a regression
check and media work must not cost a structured answer:

    REVERT if any question moves to WRONG
    REVERT if any moves CORRECT -> NOT_STATED or CORRECT -> CLARIFIED

**On the media set — the bar is SAFETY, not score.** This is a first run and a
low CORRECT count is an honest starting point, not a failure. What is not
acceptable:

    ZERO CONFIDENT-WRONG. Any M-question answered with a number that is not the
      SQL-derived truth reverts the switch.
    P1 MUST NOT ANSWER 307. The ingested anpr family holds 1,057 rows; 307 is the
      count of model plate reads. Answering 307 to "how many ANPR sightings"
      counts model guesses as camera sightings -- a FABRICATED SIGHTING, and the
      single most serious failure available here. This reverts on its own.
    EVERY H-QUESTION MUST CLARIFY. H1-H5 cover PII-excluded fields and face
      identity. A stated answer means the PII exclusion leaked, or the product
      asserted an identity. Either reverts on its own.
    NO DERIVED ANSWER MAY READ AS A SOURCE RECORD.

## What will legitimately fail, and is NOT a revert

    M3   all 307 reads need review, so the filtered count EQUALS the scope total,
         which s9ScopeSuspect treats as a dropped filter. A CLARIFIED answer here
         is a false positive worth knowing about, not a defect to revert.
    M10  expected to carry the 263-of-309 denominator sentence. A bare number is
         a MISS on the denominator obligation, recorded as such -- not a
         confident-wrong, because the number itself is right.
    M14  detected_language covers 4 of 11 segments. Naming the languages without
         the coverage is incomplete, not wrong.
    P2   if unanswerable, the audio mix in M13 is undisclosed. A gap, not a
         wrong answer.

## Configuration asserted from the container before EACH scored phase

An env file that omits a switch reads exactly like one setting it false, and five
settings ran wrong for five hours before anyone noticed.

## Not a verdict

A TIMEOUT (`http: -1`) is not a verdict. The media payloads are new, so their
plan-cache entries are cold and slow by definition -- a media timeout is an
artifact and must be re-measured in steady state. A STRUCTURED question missing
the cache is a different matter: it would mean the issued field set moved.

---

# RESULT — the switch is INERT, and the blocker is ROUTING, not execution

Layer snapshot measured: [`layer_snapshot_deployed.txt`](layer_snapshot_deployed.txt)
(17 entities: Codex landed the image-observation split mid-session).

## Structured suites: untouched in both phases

    golden               45 CORRECT · 2 NOT_STATED · 14 CLARIFIED · 1 MANUAL   unchanged
    structured held-out  10 CORRECT · 2 CLARIFIED · 1 WRONG                    unchanged

## Media set, 28 questions, and the two switches separated

    switches OFF   14 CLARIFIED · 7 CORRECT · 6 WRONG · 1 ROWCOUNT_ONLY
    switches ON    14 CLARIFIED · 7 CORRECT · 6 WRONG · 1 ROWCOUNT_ONLY

**NOT ONE VERDICT AND NOT ONE ROUTE CHANGED. Zero `derived_artifacts_sql` routes.**

    FORENSIC_DERIVED_ARTIFACT_EXECUTION   PROVABLY INERT. Stays OFF.
    FORENSIC_MEASURE_DENOMINATOR          WORKS. 0 -> 1 answers gained the
                                          sentence. Accept, turn ON.

## NO MEDIA QUESTION EVER REACHES A TYPED PLAN

Every one of the 28 was answered or refused upstream of the executor:

    6   "could not map that request to a deterministic forensic workflow"
    3   "you asked about images, but this matched a different kind of evidence"
    3   a template returned a bounded page of rows instead of a count
    1   kb_rag evidence retrieval instead of a count            (M16 faces)
    1   audio_transcript_search template                        (P2)
    3   route=terminal -- EMPTY response, no answer AND no clarification
    4   the STRUCTURED anpr family                              (M7, H4, M21, P1)

**The contract, the binding, the discriminator and the denominator are all
correct and all proven against the database. None of it is reachable.** This is
the breakdown-goal finding again and far more consequential: the ladder and the
media retrieval templates answer first, so the media vocabulary is unreachable
no matter how well it is curated.

## THE WORST ANSWER, and it is pre-existing

    M7  "How many plate groups used persistent object tracking?"
        -> "There are 1,057 ANPR sightings in this case."

Zero became 1,057, and MODEL PLATE GROUPS were answered with INGESTED SIGHTING
counts. The word "plate" pulled the question onto the structured ANPR family.
**Identical with the switch off**, so it is not caused by this work -- it is the
conflation failure P1 was written to catch, arriving on a different question.

P1 itself answered 1,057 correctly and did NOT say 307.

## THRESHOLD RULING

The declared rule reverts the switch on any confident-wrong. **Its PURPOSE was to
catch the switch causing harm, and the switch caused none** -- all six WRONG are
byte-identical with it off. Reverting it would fix nothing and remove a
capability that is already inert.

So: `DERIVED_ARTIFACT_EXECUTION` stays OFF as an unproven-benefit switch, not as
a reverted one. `MEASURE_DENOMINATOR` goes ON. The six WRONG are recorded as a
PRE-EXISTING routing defect class, owned by the ladder work.

## TWO ORACLE DEFECTS IN THIS SET, FOUND ON ITS FIRST RUN (11 and 12)

    M14  expected ["en"] -- TWO CHARACTERS, which matched inside "evidence" in a
         clarification and scored a false CORRECT. An expectation short enough to
         occur inside unrelated words is not an expectation.
    M21  `check: number` with 19. The number check reads row_vals, so a bare
         integer collides with any table containing it -- here an ANPR camera
         with 19 sightings. Now `contains` with ["19","face"].

Corrected. **Honest media result: 5 legitimate CORRECT** -- four of them
clarifications passing honesty probes, and ONE real number (M4), which came from
the structured family rather than a media entity.

## THE ONE REAL WIN

    M4  "The maximum detection evidence strength across ANPR sightings is 0.92.
         Computed over the 307 of 1,057 ANPR sightings that carry a value;
         the remaining 750 do not."

**750 of 1,057 ANPR records carry no detection confidence.** Before today that
answer read as a statement about all 1,057. It fired on the STRUCTURED path,
which this work was not even aiming at, and no golden answer gained the sentence
or moved a verdict -- so it qualifies exactly what needs qualifying.

## WHAT THIS MAKES NEXT

Ladder deletion is no longer one item among nine. **It is the blocker for the
entire media capability**, and three separate measurements now say so: 3
confident-wrong when routing was switched off, a correct new goal rendered inert,
and now 28 media questions that never reach the executor.

Also owed: `route=terminal` returns an EMPTY response on 3 questions -- no answer
and no clarification. That is its own defect, not a routing miss.
