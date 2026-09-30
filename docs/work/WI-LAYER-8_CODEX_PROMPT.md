# WI-LAYER-8 — the goal taxonomy landed and now three questions are blocked on one curated flag

WI-LAYER-7 is **accepted and shipped in the layer**. I verified it myself rather than reading the
report, and the thing I most needed to check was safe: `sensitivity: PII` is preserved on every
text field, so `CatalogFields` still drops them. `redaction: WITHHELD` turned out to be already in
the validator allowlist, not a new value you invented.

**Your Urdu/Roman finding was the most valuable part and it changed the design.** "Parent-ID
synchronisation is not universally reliable — the video derivative has a non-matching parent ID"
is exactly the honest answer I asked for, and it is why free text stays WITHHELD rather than
token-redacted. Same for the 22 raw + 2 normalised OCR rows with non-ASCII digits: meaning
unclassifiable from shape, so not relabelled. **Both refusals were worth more than the work they
cancelled**, which is now the third time that has been true of your reports.

---

## 1. What changed on my side, and what it exposed

I built the **goal taxonomy slices** (`FORENSIC_GOAL_TAXONOMY_SLICES`, default off). `lookup` is
the classifier default and means UNRECOGNISED: it skips shape verification, and the algebra goal
gate refuses it, so a media question in that bucket never reaches the compiler.

    slice A   earliest/latest -> rank         2 questions, 0 disturbed
    slice B   which/what <field> detected|produced|used|read -> distinct
                                              5 questions, 0 disturbed

Offline effect: **media routing coverage 15 -> 21 of 28 (75%)**.

Measured live, three arms, control first: **routing changed and not one verdict did.** Four
questions now reach the compiler and are refused there with the same message:

    M11-OCR-SCRIPTS    What script families were detected in the image text?
    M18-FACE-MODELS    Which model produced the face vectors?
    M20-HASH-ALGOS     Which perceptual hash algorithm was used for the images?

    "I could not map that request to a deterministic forensic workflow."

**The compiler is right to refuse, and the reason is a curated flag.**

## 2. THE ASK — and please check my reading before you act on it

A `distinct` goal needs a DISTINCT-CAPABLE field. `TestWI4DistinctCountsRestOnDistinctCapableFields`
enforces it, and that guard is correct: a distinct count over a field the layer does not declare
countable-distinct is a number nobody can justify.

What I see today:

| Question | Field it needs | `allowed_aggregates` today |
|---|---|---|
| M11 | `image_ocr_observation.script` | `[COUNT]` — no COUNT_DISTINCT |
| M18 | `face_model_observation.embedding_model` | the whole entity declares **no** COUNT_DISTINCT |
| M20 | `image_fingerprint_observation.perceptual_hash_algorithm` | the whole entity declares **no** COUNT_DISTINCT |

By contrast `anpr_model_observation`, `video_anpr_plate_group`, `image_ocr_observation`,
`audio_timestamp_segment` and `audio_roman_urdu_segment` all declare COUNT_DISTINCT on at least
one field, so the capability exists in the layer and these three fields simply do not carry it.

**For each of the three fields, rule on:**

1. **Is COUNT_DISTINCT meaningful over it, on the retained data?** Not "would it execute" —
   whether the distinct set is a thing an analyst should be shown. `script` and
   `perceptual_hash_algorithm` look like small closed vocabularies to me, which is the ideal
   shape; `embedding_model` looks like a producer identifier. **If any of them is not a
   meaningful distinct set, say so and leave it.** An unanswerable question is better than a
   number nobody can defend.
2. **Is `distinct_capable` the right flag, or `allowed_aggregates`, or both?** You own that
   contract and I do not want to guess at its semantics.
3. **Cardinality.** The source-native executor caps a grouping at 100. Please report the distinct
   count of each field from the data — if one exceeds 100 the question is refused anyway and the
   flag would be misleading.

**M14-AUDIO-LANGUAGES is a separate, simpler item.** The census says it needs only a synonym:
`goal=distinct is algebraic, so a synonym is enough`. "What languages were detected in the audio?"
names no curated multi-word phrase today. If an honest phrase exists for
`audio_timestamp_segment`, add it; I dropped `audio language` from the WI-LAYER-6 brief myself
because I judged it a weak description, and I would rather you rule than inherit my judgement.

## 3. WHAT IS NOT YOURS, and one thing I got wrong

- **The classifier, the slices, masking, the browse guard** — `api/**`, mine.
- **M8 and M15 are blocked upstream of you.** They reach the REQUEST classifier as
  `GENERAL_DOMAIN_KNOWLEDGE` and route `[terminal]`, so they never reach the goal classifier at
  all. That is my `analyticalIntent` revert, made after I caused a PII disclosure, and re-enabling
  it is my call and my measurement. **Do not add vocabulary to work around it.**
- **H2 must stay blocked.** It is the PII probe. The goal gate is what keeps it off the retrieval
  path until free-text masking exists, and that is deliberate.

**The mistake worth naming:** I predicted the goal slices would move verdicts and they moved none.
Routing was necessary and not sufficient, and the next gate down was a curated flag I had not
checked. That is the third time this week I have fixed one gate while another still held — and
twice now the instrument had already told me, if I had read it.

## 4. VERIFY

```bash
go test -C api/forensic_records -count=1 -run TestWI4 -v .
FORENSIC_GOAL_TAXONOMY_SLICES=true \
  go test -C api/forensic_records -count=1 -run TestMediaFamilyRoutingCensus -v .
FAMILY_AUDIT_DB="postgresql://localrecall:localrecall@localhost:5433/localrecall" \
  go test -C api/forensic_records -count=1 -run TestSemanticLayerCoverage -v .
```

**Expected after your change:** the census stays at 21 of 28 — routing is already solved and this
does not touch it. What should change is that M11, M18 and M20 stop being refused by the compiler.
I will measure that live; do not try to verify it yourself, and **report the distinct counts from
the data** so I can predict the outcome before the run rather than after.

### Stop and report, do not proceed, if

- `api/**` does not compile. Say so **with the wall-clock time** and stop. Never fix it.
- A field's distinct set is not meaningful, or exceeds the 100-group cap. **That is a finding, not
  a failure** — it decides whether the question is answerable at all.
- You would have to change a `sensitivity` value to make something work. **Say so and stop.**
  `TestMediaPIIFieldsAreNeverIssuedInTheCatalogue` derives its list FROM the layer, so unmarking a
  field silently removes its protection.

## 5. Also owed to you, so you are not blocked on me

Extending the held-out set to >=40 is still the primary gate and is still yours; I will send that
brief separately rather than bundle it here. Two things it will carry, both paid for this week:
every expectation SQL-verified and derived from the same expression the executor uses (we are at
**thirteen** oracle defects in our own suites, the last one in my own comparator), and honesty
probes written deliberately with what each must refuse and why.
