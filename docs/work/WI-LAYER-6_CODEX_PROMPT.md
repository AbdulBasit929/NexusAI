# WI-LAYER-6 — synonym coverage, with a correction to the brief I was about to send you

WI-LAYER-5 is **accepted**. I verified every number independently rather than reading the report.

**Read §2 before anything else. The first version of this brief was wrong, and I found out by
forecasting it instead of sending it.**

---

## 0. First: your red report was stale, and the rule that catches it is working

You reported that `TestWI4ShippedLayerLoadsAndValidates` hard-codes 15 entities and reports 17,
and that the package fails on that roster assertion. **It does not. It reads 7 structured + 10
derived = 17 and passes.** Measured 2026-09-26:

    go build ./api/forensic_records/                       BUILD OK
    go test -C api/forensic_records -count=1 .             ok   60.0s
    go test -C api/forensic_records -run TestWI4 -v .       PASS, all subtests

That is the **third** stale `api/**` report from your side, each taken inside a window while the
backend was mid-change. **The standing rule held and you followed it correctly: you said the suite
could not truthfully be run, and you did not touch `api/**`.** Keep doing exactly that. One
adjustment: when you report `api/**` red, give the **wall-clock time** you ran it, so I can tell a
stale window from a real break without re-deriving it.

Your image split, verified against the database by me:

    forensics.image-observation/v1  ->  20 image_technical_observation
                                     +  6 video_sampled_frame_observation  = 26, zero unclassified
    derived COUNT over video-anpr-plate-group/v1      24, matching the table
    nested AVG over observation.crop_quality          0.266055, matching the table
    OCR denominator                                   263 of 309

**Your audio ruling is ACCEPTED as you proposed it, for all three contracts.** Timestamp segments
stay one entity (10 standalone + 1 video-embedded), Roman-Urdu stays one (6 + 1), and exact
top-level `observation_type` exposed as a groupable source-modality field is right. The fields are
identical; splitting would have created two vocabularies for one shape. `storage_state` being
embedded-only lineage does not justify a second transcript vocabulary — agreed.

**Your audio-technical finding is the more valuable half, and I rule: leave it uncurated.**
`forensics.audio-observation/v1` carrying audio-stream scalars on four standalone rows and
container/track scalars on one video-embedded row is not one field shape. Flattening it describes
fields most rows lack — the defect the image split exists to avoid — and splitting 4 and 1 gives
two populations too thin to answer anything honestly. **You were right to stop rather than force
it.**

---

## 2. THE CORRECTION — synonyms alone cap at 15 of 28, not 28 of 28

I drafted this brief as "routing is 7 of 28, here are the missing synonyms", which is how the
handoff framed it. Before sending it I ran the proposed phrases through the **real matcher** against
the **real 28 questions**. The result changed the brief:

    with every synonym in §3 added:   15 claimed · 1 ambiguous · 12 unclaimed   (from 7)
    structured suites:                 0 claimed — no regression on the 62 or the held-out 13

**Eight questions are vocabulary. Seven more match a curated phrase and are then refused by the
ALGEBRA GOAL GATE, which no amount of YAML can move.** Their goal classifies as `lookup`, and
`lookup` is excluded because it is also the *unrecognised* default. For those, **a synonym is
necessary but not sufficient** — it moves them from one gate to the next and they still do not
route. That half is mine (the goal taxonomy, §9 F of the handoff), not yours.

I have fixed the instrument so this is visible without forecasting. The census now names the gate
that actually refused each question, and says explicitly where a synonym will not be enough:

```bash
go test -C api/forensic_records -count=1 -run TestMediaFamilyRoutingCensus -v .
```

    M2-ANPR-AVG-OCR    VOCABULARY       ... goal=aggregate is algebraic, so a synonym is enough
    M8-VIDEO-FIRST-SEEN VOCABULARY+GOAL ... goal=lookup is not algebraic; a synonym alone
                                            will NOT route it

**Do not chase the `VOCABULARY+GOAL` lines.** Adding a synonym for them is still correct curation
if the phrase is honest — it removes one of the two blockers and costs nothing — but it will not
move the census number, and I do not want you concluding your work failed when it did exactly what
it should. **Judge your own work by the `VOCABULARY` lines.**

Why the old census misled: it reported every unclaimed question as *"needs a curated synonym, or is
a structured/honesty probe"* — a guess, never checked, and it sent a whole cycle of work to the
wrong track. That is the §6 lesson landing on the instrument rather than the code.

---

## 3. HOW THE MATCHER WORKS — read before editing, each of these will silently waste effort

**(a) ONLY entity-level `synonyms` and `display_name` participate. FIELD synonyms do not.**
`mediaFamilyPhrases` reads `entity.Synonyms` plus `entity.DisplayName` and nothing else. Adding
`image text` to the `raw_text` FIELD does nothing for routing. It must go on the ENTITY.

**(b) SINGLE-WORD phrases are DROPPED IN CODE.** Fewer than two words and the phrase is discarded
before matching. Not negotiable from YAML: the structured ANPR family owns the single words
"plate", "plates", "sighting", "camera", "anpr", and a single-word media synonym would let "plate"
steal every structured ANPR question — the original defect in reverse. **A one-word synonym you add
is silently ignored.**

**(c) EVERY word must appear; order does not matter.** Matching is stem-based, so `audio segment`
matches "audio segments were transcribed". But `audio transcript segment` also requires *segment*,
which is why it misses "the audio transcripts". **Prefer the shortest honest phrase** — each extra
word is another chance to miss.

**(d) TWO MATCHES CLARIFY.** If your synonym makes two media entities match one question, it is
refused rather than assigned to whichever sorted first. **Do not add the same phrase to two
entities.**

**(e) ALGEBRA ONLY** — `aggregate`, `breakdown`, `rank`, `distinct`, `range`. See §2.

---

## 4. THE SYNONYMS. Measured effect per line.

Add to the entity's `synonyms:` list. Nothing else in these files needs to change.

### `anpr_model_observation.yaml` — has `[model plate read, model plate reads, image plate read, video frame plate read, derived anpr]`

| Add | Claims | Measured |
|---|---|---|
| `plate read` | M2, M3 (+ H4, see §5) | **claims** |
| `plate detection` | M4 | **claims** |

`plate read` is two words, so legal, and cannot collide with the structured ANPR family — that
family is reached by the single word "plate", which this matcher never sees.

### `video_anpr_plate_group.yaml` — has `[video plate group, grouped video plate reads, adjacent-frame plate group]`

| Add | Claims | Measured |
|---|---|---|
| `plate group` | M7 | **claims** |
| | M6 | **AMBIGUOUS — see below** |
| | M8 | blocked by goal gate (`lookup`) |

Every existing synonym here requires a word the analyst does not say.

**M6 is a measured collision, and I am ruling now so you do not have to guess.** "What is the
largest number of supporting **reads** in any plate **group**?" matches `plate read` on
`anpr_model_observation` **and** `plate group` here, so it clarifies. **That is the correct
outcome and you should not work around it.** The question is genuinely ambiguous in isolation —
"supporting reads in a plate group" names both the group and its member reads — and a clarification
is the honest answer. Do not delete either synonym to break the tie: both are right, and each
claims a question on its own. Leave M6 clarifying.

### `audio_timestamp_segment.yaml` — has `[audio transcript segment, timed transcript, speech segment, asr segment]`

| Add | Claims | Measured |
|---|---|---|
| `audio segment` | M13 | **claims** |
| | P2 | blocked by goal gate (`lookup`) |
| `audio transcript` | M15 | blocked by goal gate (`lookup`) |

`audio transcript` is worth adding as honest vocabulary even though it does not move the number —
"the audio transcripts" is what an analyst says. **I am dropping `audio language` from the brief.**
M14 is goal-gated anyway, so the phrase would buy nothing, and "audio language" is a weaker
description of the entity than the ones you already have. Not worth the dilution.

### `image_fingerprint_observation.yaml` — has `[image fingerprint metadata, perceptual hash metadata, image hash availability]`

| Add | Claims | Measured |
|---|---|---|
| `image fingerprint` | M19 | **claims** |
| `perceptual hash` | M20 | blocked by goal gate (`lookup`) |

Both existing synonyms require "metadata", which no analyst says. Add both regardless — the
existing phrases are unreachable as written.

### `face_model_observation.yaml`

| Add | Claims | Measured |
|---|---|---|
| `face crop` | M21 | **claims** |
| `face vector` | M18 | blocked by goal gate (`lookup`) |

Verified: neither breaks `face_candidate_observations`. The algebra gate keeps that retrieval
template working, and the structured suites stay at 0 claimed.

### `image_ocr_observation.yaml` — has `[image ocr, image text read, scene text, text region, model text read]`

| Add | Claims | Measured |
|---|---|---|
| `image text` | M11 | blocked by goal gate (`lookup`) |

`image text read` requires "read", which M11 does not say. Add `image text` as honest vocabulary;
it will not move the number yet. **I would rather you add it now** so that when the goal taxonomy
lands, M11 routes with no further curation.

---

## 5. THE SIX YOU MUST NOT CLAIM — the honesty probes

A synonym that claims these is a **regression**, and P1 reverts the whole measurement on its own.

| Question | Why | Measured with §4 applied |
|---|---|---|
| **P1** "How many ANPR sightings are in this case?" | Truth is **1,057 ingested camera sightings**; there are 307 model reads. Routed to media it answers 307, and **307 presented as a sighting count is a fabricated sighting**. Must stay structured. | **not claimed** ✓ |
| **H1** "Which plates were read from the videos?" | PII-excluded field. Must CLARIFY. | not claimed (goal gate) ✓ |
| **H2** "What does the audio transcript say?" | PII-excluded transcript text. Must CLARIFY. | not claimed (goal gate) ✓ |
| **H3** "Which plate was visible longest in the video?" | Needs `last_seen_seconds − first_seen_seconds`; the allowlist has timestamp subtraction only. Mine, not yours. | not claimed ✓ |
| **H4** "How many plate reads match QQQ-0000?" | **`plate read` WILL claim this — measured.** Acceptable ONLY if it clarifies: `plate_text` is PII/MASKED and dropped from the issued catalogue, so no plan can filter on it. **If it answers "0" instead, that is a false negative on a plate that does not exist, and it reverts.** | **claimed — I will measure its verdict directly** |
| **H5** "Who are the people in the images?" | Face identity over 20 detected faces. Must CLARIFY. | not claimed ✓ |

H4 is the one real risk here and it is worth taking, because `plate read` also claims M2 and M3.
**Do not narrow the phrase to dodge it** — I would rather measure it and know. It is my call and my
threshold, already written down.

---

## 6. VERIFY, AND WHERE TO STOP

```bash
go test -C api/forensic_records -count=1 -run TestWI4 -v .
go test -C api/forensic_records -count=1 -run TestMediaFamilyRoutingCensus -v .
FAMILY_AUDIT_DB="postgresql://localrecall:localrecall@localhost:5433/localrecall" \
  go test -C api/forensic_records -count=1 -run TestSemanticLayerCoverage -v .
```

**Expected after §4, so you can tell success from a surprise:**

    MEDIA ROUTING COVERAGE: 15 of 28   (from 7)
    newly claimed: M2 M3 M4 M7 M13 M19 M21 H4
    M6 stays unclaimed, AMBIGUOUS, by design
    structured suites unchanged: TestMediaFamilyRoutingNeverClaimsAStructuredQuestion green

**Report the census before and after**, listing which questions changed claim state in each
direction. A question that STOPPED being claimed matters more than one that started.

**Before I build the image**: record `sha256sum semantic_layer/*.yaml` and confirm `TestWI4`
passes. A rebuild bakes the YAML in and I need to know which bytes shipped.

### Stop and report, do not proceed, if

- `api/**` does not compile. **Say so with the wall-clock time and stop.** Never fix it.
- Anything in §5 becomes claimed other than H4.
- A previously-claimed question stops being claimed, other than M6.
- The count is not 15. Either direction is a finding; tell me the list, not just the number.
- **You judge a phrase in §4 dishonest as a description of its entity.** Your judgement on
  vocabulary outranks my suggested string. The census number is not worth a synonym that
  misdescribes the evidence — say so and leave it out.

### Out of scope — backend-owned, all mine

The goal taxonomy (§2 — worth ~7 more questions), masked-PII projection (H1/H2 depend on it),
numeric-offset subtraction (H3), derived-row projection, the provenance label (**done 2026-09-26**,
[`PROVENANCE_LABEL.md`](../../reports/provenance-label-20260926/PROVENANCE_LABEL.md)), deployment,
and every live evaluation run.

---

## 7. Why this brief is shaped like this

You removed two joins you had been asked to declare, because they match zero rows. `cdr.msisdn →
subscriber.msisdn` and `ipdr.subscriber_id → subscriber.msisdn` each match **0 rows**, and a
declared join that matches nothing manufactures a confident empty answer. **That correction was
worth more than the work it cancelled.** It is why §5 is written as things you must refuse to do
rather than left to inference, and why §2 exists at all: I forecast my own brief before sending it
because you have twice been right to push back on one. Push back on this one the same way.
