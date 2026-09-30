# GOAL TAXONOMY + MASKING — MEASURED 2026-09-27. **BOTH SAFE. BOTH INERT. NEITHER SHIPS TODAY.**

    A  CONTROL    shipped posture
    B  SLICES     + FORENSIC_GOAL_TAXONOMY_SLICES
    C  +MASKING   + FORENSIC_PII_MASKED_PROJECTION + secret

    ALL THREE ARMS IDENTICAL, all three suites. Not one verdict moved.

Thresholds: [`THRESHOLD_PREREGISTRATION.md`](THRESHOLD_PREREGISTRATION.md), written before the build.

---

## 1. The numbers

| Suite | control | slices | +masking |
|---|---|---|---|
| golden 62 | 45 · 14 · 2 NS · 1 MAN | identical | identical |
| held-out 13 | 10 · 2 · 1 WRONG | identical | identical |
| media 28 | 12 · 12 · 3 WRONG · 1 ROWCOUNT | identical | identical |

**Arm A reproduced the shipped result exactly**, so the gate held and both arms are attributable.
**Safety bar PASS:** 105 plausible registrations checked against every response in all three arms;
9 files contain one, all 9 on the pre-existing DOC-passage path, **0 elsewhere**.

## 2. The threshold fired on both named predictions

    M15 is still WRONG in arm B
    H1 does not answer in aliases with masking on

I predicted both would move. Neither did. **Routing was necessary and not sufficient** — and there
turned out to be a different downstream blocker behind each one.

## 3. THE THREE BLOCKERS, now precisely known

### M8, M15 — blocked UPSTREAM of the goal classifier, by me

    route = [terminal]      request_class = GENERAL_DOMAIN_KNOWLEDGE

They never reach the goal classifier at all. `analyticalIntent` in `pkg/forensicrequest` lacks
`earliest|latest`, because **I reverted exactly that after it caused a PII disclosure** — and
`TestClassifierSuperlativeBoundary` now asserts they stay absent so nobody re-adds them casually.

That revert was correct at the time: reclassifying them sent M15 to `audio_transcript_search`, a
retrieval template, where it answered a FALSE ABSENCE. **What has changed since is that slice A
makes the goal `rank`, which is algebraic, so media routing would now CLAIM the question and send
it to the compiler instead of that template.** Re-enabling the two words is therefore a real
candidate again — but it is a separate change, gated by the same switch, with its own control.
**Mine, not curation.**

### M11, M18, M20 — blocked on a curated flag

They now reach the compiler and are correctly refused:

    "I could not map that request to a deterministic forensic workflow."

A `distinct` goal needs a distinct-capable field, and `TestWI4DistinctCountsRestOnDistinctCapable-
Fields` enforces it. That guard is right: a distinct count over a field the layer does not declare
countable-distinct is a number nobody can defend.

    image_ocr_observation.script                          [COUNT] only
    face_model_observation.embedding_model                entity declares NO COUNT_DISTINCT
    image_fingerprint_observation.perceptual_hash_algorithm   entity declares NO COUNT_DISTINCT

**Curation, and it is Codex's.** Asked in
[`WI-LAYER-8_CODEX_PROMPT.md`](../../docs/work/WI-LAYER-8_CODEX_PROMPT.md), which also asks for the
distinct CARDINALITY from the data, because of the next item.

### H1 — structurally unanswerable as posed, and this is the interesting one

    request_class = GOVERNED_EVIDENCE_ANALYSIS     (reaches the system)
    template      = ''                             (the ladder correctly abstained)
    route         = [clarification]
    planner.reason = "no approved deterministic workflow matched the raw question"

`plate_text` IS distinct-capable (`[COUNT, COUNT_DISTINCT]`), masking was on, the secret was
present, and the goal was `distinct`. It still produced no plan, and the arithmetic says why:

- **Derived PROJECTION is refused** — `derived_artifacts` has no `record_id` or `row_hash`, so a
  row listing cannot be cited. That is a deliberate refusal, not a gap.
- **The GROUP BY fallback needs 307 groups** — H1 routes to `anpr_model_observation`, the
  individual observations — **against a 100-group cap.**

So "which plates were read from the videos" is not answerable over the 307-row family by either
available shape. Over `video_anpr_plate_group` (24 groups) it would fit comfortably. **The
question routes to the wrong one of the two plate families**, and that is a routing question, not
a masking one.

## 4. RECOMMENDATION: ship neither today

Both are **safe** — the suites are green in every configuration, the safety bar passes, and arm A
proves both are inert when off. Both are also **inert when on**, and shipping an inert change adds
live surface for no capability. That was the recommendation for masking yesterday and the same
reasoning now covers the slices.

**Keep both built, both off.** The three blockers above are each separately actionable, and each
one is now named rather than suspected.

## 5. What this cost, and what it bought

It cost three arms and an afternoon to move nothing.

It bought the end of guessing. Before today, "the goal taxonomy is worth ~7 media questions" was an
inference from an offline census. It is now measured, and the census was **right about routing and
wrong about answers** — coverage did go 15 -> 21, and zero of those became answers. The three real
blockers were invisible behind the first one.

**The recurring error is mine and it is the same one three times this week:** fix a gate, predict
the questions will flow, and find another gate behind it. The instrument had already printed the
evidence each time — the routing census said M8/M15 were `VOCABULARY+GOAL`, the curated YAML said
`[COUNT]`, and the group cap is a constant in the executor. **Reading further down the path before
predicting is cheaper than an afternoon of arms.**
