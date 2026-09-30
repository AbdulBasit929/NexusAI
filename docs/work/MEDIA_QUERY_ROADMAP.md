# ROADMAP — answering ANY question, over EVERY data type, with no predefined operations

Written 2026-09-25 against measured facts, not intentions. The goal in one line:
**an analyst asks a question in their own words about any evidence in the case,
and the system either answers it from a plan it can verify, or says why it
cannot.** No template written in advance for that question.

## Where the product actually stands

**Structured families: solved in principle.** The Governed Semantic Compiler
emits a typed plan constrained to a curated vocabulary, verifies it, and abstains
otherwise. Curation reached 93/94 columns (99%). Held-out questions nobody wrote
for the system went from 7 WRONG to 1.

**Media: not started.** Two measured facts define the gap:

    source_native_sql_executor.go   hard-wired to forensic.records in 5 places
    semantic_layer/*.yaml           zero references to derived_artifacts

So no typed plan can reference a single OCR line, plate read, face detection or
transcript segment. Every media question that works today works because somebody
wrote a template for it. **That is the dependency being removed.**

The evidence is already there and already typed — 865 derived artifacts across
11 contracts in `nexusai-multimodal-product-acceptance` alone, with normalized
text, numeric confidences, review states and time offsets.

---

# STAGE 0 — THE ANSWER MUST REACH THE ANALYST. Blocks everything.

**Found 2026-09-25 and it invalidates the headline metric.** The plan is right,
the SQL is right, the rows are right, and the narration reports the PROCESS
instead of the RESULT.

    CDR-09  "What date range do the CDR records cover?"        graded CORRECT
            analyst sees "1 CDR record matched this question."  -> no dates
    CDR-10  "Who did 923001110001 contact most frequently?"    graded CORRECT
            analyst sees "8 phone contacts ranked."             -> no contact

Eight of 62 are genuinely affected; three are total misses. The harness missed it
because `contains` grades the whole response blob, which includes raw rows the
analyst never sees. Evidence:
[`ANSWER_NOT_STATED.md`](../../reports/answer-not-stated-20260925/ANSWER_NOT_STATED.md).

    0.1  grade `contains` against ANALYST TEXT, normalised for formatting, the
         way `number_stated` already does. Re-baseline the 62 honestly. The
         number will DROP. A metric counting answers the analyst never got is
         worse than a lower one.
    0.2  answer_presentation.go states the retrieved values: the range, the top
         contact, every group of a breakdown.
    0.3  GATE: no question scores CORRECT unless its value appears in the text
         an analyst reads. Enforced in the harness, permanently.

**Why this is first.** Every stage below adds new answers. Shipping them onto a
narration layer that withholds results multiplies the defect across four more
modalities. It also makes every measurement from here honest, and the standing
lesson — fix the instrument first — has now been paid for five times.

# STAGE 1 — THE EXECUTOR REACHES DERIVED ARTIFACTS

Backend only. The one change that converts media from "templates" to "algebra".

    1.1  extend SemanticLayerEntityV1 with an optional source block:
             source: {table, artifact_type, payload}
         default table = records, so every existing entity is unchanged.
    1.2  the executor resolves table + payload + artifact_type filter PER ENTITY
         instead of assuming forensic.records / raw_payload.
    1.3  PROVENANCE GUARD, new and non-negotiable: a plan over rows carrying
         source_truth_state = derived_model_observation must carry that fact
         into the answer. A model's plate read is not a camera's sighting.
    1.4  S9 PROVENANCE OBLIGATION: an answer drawn from derived observations
         that does not state they are unreviewed model output is REFUSED.
    1.5  GATE: one media question answered end-to-end by a typed plan, with the
         provenance stated, and the 62 unchanged.

Behind its own switch, default off, measured against a control on the same
container — the discipline that has now caught a neutral result, five confident-
wrong candidates and a five-setting config drift.

# STAGE 2 — CURATE THE MEDIA VOCABULARY (Codex WI-LAYER-3, in flight)

11 artifact contracts become entities:
[`WI-LAYER-3_CODEX_PROMPT.md`](WI-LAYER-3_CODEX_PROMPT.md).

    ANPR from images        normalized_plate_text, ocr_confidence,
                            detection_confidence, manual_review_required
    ANPR from video         plate groups: first_seen_seconds, last_seen_seconds,
                            sightings_count, persistent_tracking
    OCR from images         normalized_text, recognition_confidence,
                            script_family, language_claim, reading_order
    audio transcription     text, start_seconds, end_seconds, detected_language,
                            detected_language_probability
    Roman-Urdu audio        roman_urdu_text, raw_urdu_text, identifiers
    faces                   detection_confidence, quality, review_state
    document / pdf / txt    native text passages (may stay with retrieval)
    image fingerprints      perceptual_hash_algorithm, near_duplicate_semantics

**GATE: a held-out media set, written against the DATA, scored like the
structured one.** Minimum 20 questions across all modalities. The 62 do not
measure this and cannot.

# STAGE 3 — SIMILARITY AND COMPARISON, which are NOT algebra

Face comparison and image near-duplicates are the one media capability that must
NOT become a curated field, because a similarity score is not an answer.

    3.1  a governed similarity executor: explicit threshold, both scores shown,
         the crop shown, and the decision left to the analyst.
    3.2  ABSOLUTE: the system NEVER asserts identity. "These two faces score
         0.91 under model X at threshold Y" is admissible. "This is the same
         person" is a fabricated identity and is refused.
    3.3  near-duplicate images answered from perceptual_hash, stated as
         near-duplicate, never as "the same image".
    3.4  GATE: a negative control — two different people the model scores high —
         must NOT produce an identity claim.

# STAGE 4 — CROSS-MODAL, the capability the product was bought for

Today cross-family is 0 of 5, and WI-LAYER-2 proved the joins we assumed existed
do not: `cdr.msisdn -> subscriber.msisdn` matches ZERO rows.

    4.1  make the runtime CONSUME the join graph (nothing does today; Joins is
         validated and ignored).
    4.2  duplicate-safe join execution, and a decision on many_to_many, which
         the v1 validator currently rejects and CDR<->IPDR requires.
    4.3  the same plate across: structured ANPR, image ANPR reads, video plate
         groups, and OCR text — each labelled with WHICH it came from.
    4.4  GATE: "where does <identifier> appear across all evidence" answered
         with every modality it truly appears in, and no modality it does not.

# STAGE 5 — DELETE THE LADDER

Phase 3's remaining deliverable, and Stage 2 is its precondition: the ladder
cannot be removed while it is the only thing answering media questions.

Measured, so not guesswork: with routing off, 3 confident-wrong and 3 correct
lost, because `chooseTemplate` also SCOPES family and record type. And on
2026-09-25 the ladder made a correct new goal completely inert — it intercepted
the four questions before the compiler could act.

    5.1  derive scope from the curated layer.
    5.2  delete in slices, each measured against a control.
    5.3  then the goal taxonomy pays: breakdown, rank and range slices measured
         together with the S9 rows check.

# STAGE 6 — THE QUESTIONS THAT ARE NOT ABOUT DATA

Unwired today, and a real user hits them in the first minute: greetings,
chit-chat, "what can you do", product help, and out-of-scope. D7 ("Explain ...")
is the same area. Cheap, visible, and currently absent.

---

# SEQUENCING AND EFFORT

Honest ranges in working sessions, not dates. Every stage ends in a measurement
against a threshold written BEFORE the run.

    STAGE 0   1-2 sessions   me        BLOCKS EVERYTHING
    STAGE 1   2-3 sessions   me        contract + executor + provenance guard
    STAGE 2   2-3 sessions   Codex     in flight, plus my held-out media set
    STAGE 3   2-3 sessions   me        similarity, and the identity refusal
    STAGE 4   3-4 sessions   me        cross-modal, the hardest
    STAGE 5   3-5 sessions   me        ladder deletion in slices
    STAGE 6   1 session      me+Codex  the unwired classes

Stages 0 and 1 are mine and sequential. Stage 2 runs in PARALLEL on Codex once
the Stage 1 contract exists. Stages 3-5 need 1 and 2 complete.

**What "done" means, and it is measurable:** a held-out set of >=40 structured
and >=20 media questions, written against the data by someone who did not write
the system, scoring zero confident-wrong, with every stated answer containing its
value in the analyst-facing text and every derived observation labelled as model
output.
