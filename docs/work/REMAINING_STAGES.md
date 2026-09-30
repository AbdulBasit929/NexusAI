# WHAT IS LEFT — as of 2026-09-26, against measured state

Supersedes the stage list in `MEDIA_QUERY_ROADMAP.md` where they differ. Every
item ends in a measurement against a threshold written BEFORE the run.

## DONE AND MEASURED

    STAGE 0   the stated answer contains the answer      39 -> 45 CORRECT, 0 new WRONG
              grader reads analyst text; NOT_STATED is its own verdict
    STAGE 1a  entities declare WHERE their rows live     15 entities load, suite green
    STAGE 1b  the executor reads derived_artifacts       COUNT=24 and nested AVG both
              match the table; the 9,580 defect caught by that test
    STAGE 1c  observation_type discriminator             partitions 20 + 6 = 26 exactly
    STAGE 1d  measure denominator                        263 of 309 stated in the answer
    LAYER VISIBILITY CONTROL                             62/62 and 13/13 unchanged,
              every structured question a plan-cache HIT, so the issued field set
              provably did not move

## THE IMMEDIATE NEXT STEP, and it is one deploy

**Enable derived execution and measure the media capability for the first time.**
Everything above is proven in unit and database tests; **no media question has
been answered live yet.**

    1  re-run the layer-visibility control against the CURRENT layer snapshot
       (Codex changed synonyms in WI-LAYER-4 after the last control, and synonyms
       drive family selection -- the last control measured a different snapshot,
       recorded by sha256 in reports/stage1-20260926/)
    2  write a MEDIA HELD-OUT SET against the data, minimum 20 questions across
       ANPR/OCR/audio/faces/video. I write it, not Codex: they curated the layer
       and would be scoring their own vocabulary.
    3  deploy with FORENSIC_DERIVED_ARTIFACT_EXECUTION=true and
       FORENSIC_MEASURE_DENOMINATOR=true, control pass first
    4  score golden + structured held-out (must not move) and the media set

**Gate:** zero confident-wrong on the media set; every derived answer states that
it is a model observation; the 62 and the structured held-out unchanged.

## THEN, IN ORDER

### A. PII masking at projection — the largest single capability gap

`CatalogFields` drops PII/RESTRICTED outright, so 7 curated fields cannot be
referenced by any plan. What that costs, measured:

    ANSWERABLE NOW    how many plates were read · how many need review ·
                      average confidence · what languages · how many faces
    NOT ANSWERABLE    WHICH plates were read · what the transcript says ·
                      which plate was visible longest (it needs the plate)

Codex's recommendation is accepted and is what ships: a case-scoped stable alias
(`Plate candidate ••••-A7C2`) rather than last-four, because plate strings are
low entropy so last-four both reveals too much and collides; typed placeholders
(`[phone masked]`) inside OCR and transcript text with synchronised redaction
across the Urdu and Roman-Urdu forms; **and if token-level redaction cannot be
proven, withhold the whole text while keeping counts, timestamps, language,
review state and locators.**

### B. Numeric-offset subtraction

"Which plate was visible longest?" needs `last_seen_seconds - first_seen_seconds`.
The expression allowlist supports timestamp subtraction only. Small, bounded, and
blocked behind (A) because the answer also needs the plate.

### C. The two NOT_STATED questions on the template path

CDR-10 and ANPR-03 are answered by registered templates, so
`sourceNativeResultAnswer` never runs and `tabularResultAnswer` needs the same
treatment Stage 0 gave the typed path.

### D. Derived-row PROJECTION

Listing individual observations is refused today: `derived_artifacts` has no
`record_id`, `row_number`, `batch_id` or `row_hash`, and citing rows without
provenance hands the analyst evidence they cannot attribute. Needs a derived
provenance mapping (`artifact_id` + `citation_locator`).

### E. Ladder deletion — Phase 3's remaining deliverable

`query.go` is 9,041 lines and 12 templates. Measured with routing off: **3
confident-wrong and 3 correct lost**, because `chooseTemplate` also SCOPES family
and record type. And on 2026-09-25 it made a correct new goal completely inert by
intercepting the four questions before the compiler saw them.

Media curation was its precondition and that is now done, so this is unblocked:
derive scope from the curated layer, then delete in slices.

### F. Goal taxonomy, re-measured

`breakdown` is built and measured NEUTRAL with the switch off — but that verdict
was scored against a control that over-credited four of its target questions, and
the instrument is fixed now. Re-measure together with the S9 `rows` case, the
"longest/shortest" -> `rank` slice and ANPR-03 "first and last seen" -> `range`.

### G. Cross-modal — the capability the product was bought for

Cross-family is 0 of 5. `Joins` is validated and **nothing consumes it at
runtime**. WI-LAYER-2 also proved the joins we assumed existed do not:
`cdr.msisdn -> subscriber.msisdn` matches ZERO rows.

Needs duplicate-safe join execution and a decision on `many_to_many`, which the
v1 validator rejects and CDR<->IPDR requires. Then: the same plate across
structured ANPR, image reads, video groups and OCR text — **each labelled with
which it came from**.

### H. The question classes that are not about data

Greetings, chit-chat, "what can you do", product help, out-of-scope. Unwired
today; a real user hits these in the first minute. D7 ("Explain …") is the same
area. Cheap and highly visible.

### I. Identity, cases, and the backend contracts Codex is blocked on

No case entity exists (`case_id` 3x vs `collection_id` 224x) and every analyst is
one shared principal. **This blocks real deployment.** With it: login/session,
tenant/role/classification, cursor-paginated structured records, normalized
page/region/timed locators, server-persisted pins and history, Dashboard data.

## STANDING RULES THAT KEEP COSTING MONEY WHEN IGNORED

    assert the SWITCH STATE from the CONTAINER before scoring any run -- an env
      file that omits a switch reads exactly like one setting it false, and five
      settings ran wrong for five hours before anyone noticed
    a TIMEOUT is never a verdict
    re-derive every expectation when curation lands -- TEN oracle defects so far,
      the last two on 2026-09-26
    a probe must reproduce the PATH, not just the function
    the enum is not a menu; changing the issued field set changes plans
      everywhere, and the plan cache is how you PROVE it did not
    if api/** does not compile, report it -- never fix across the track boundary
