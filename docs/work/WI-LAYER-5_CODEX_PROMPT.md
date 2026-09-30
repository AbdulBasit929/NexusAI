# WI-LAYER-5 — the discriminator you asked for has landed. Two items, one of them a finding you did not make.

WI-LAYER-4 is **accepted**. I verified every number independently rather than
reading your report:

    ocr normalized_text    46 / 309      matches
    ocr frame_timestamp    40 / 309      matches
    audio timing_status     5 / 11       matches
    anpr crop_quality     299 / 307      matches
    image-observation split  20 technical + 6 sampled frames = 26, zero unclassified

**The synonym rebinding is right and it is the important part.** General
phrasings — "ocr confidence", "confidence", "how confident" — now bind only to
`observation.confidence` at 263/309, and the two 46-row PaddleOCR scores carry
producer-specific synonyms only. That is what stops a general question averaging
15% of the evidence and presenting it as all of it.

The denominator sentences are exactly the right shape: *"Present on 263 of 309
retained observations and absent on the 46 PaddleOCR multilingual-region
observations."*

**Your qualification is correct and it is now my work.** 263/309 is not
whole-corpus coverage, and a description the model reads does not make the
ANSWER disclose it. An S9 obligation and a presentation change are owed so that
"the average general OCR confidence is 0.87" cannot be stated without its
denominator. Do not work around it in YAML.

## THE COMPILE FAILURE YOU HIT WAS MINE, AND IT IS FIXED

`go build`, `go vet` and the full Go suite are all green now. You attempted
TestWI4 inside a ~15 minute window while I was mid-refactor on the executor
helper signatures — the helpers were changed and two test files had not caught up
yet. **You were right not to touch `api/**` and right to report that the suite
could not truthfully be run.** Please re-run it now; it works.

**Standing coordination rule, since this will recur:** we share one worktree. If
`api/**` does not compile, say so and stop — never "fix" it. Report and I will.

---

## 1. `observation_type` HAS LANDED. Curate the split.

The contract now accepts an exact, bound discriminator:

```yaml
source:
  table: derived_artifacts
  artifact_type: forensics.image-observation/v1
  payload: metadata
  observation_type: video_sampled_frame_observation
```

Matched EXACTLY against `metadata->>'observation_type'`, never as a prefix.
Proven against the table: the predicate partitions the contract into **20 + 6 =
26** with nothing left over (`TestDerivedObservationTypePredicateNarrowsExactly`).

Build the two entities exactly as you proposed:

    image_technical_metadata   observation_type: image_technical_observation   20 rows
    video_sampled_frame        observation_type: video_sampled_frame_observation 6 rows

Keep hashes, parent identifiers and the roles array excluded, as you argued.

**Six rows is a small population. Say so in the entity description.** An
aggregate over 6 rows is arithmetically fine and forensically thin, and the
analyst should learn that from the entity rather than from the row count.

## 2. A FINDING FROM MY VERIFICATION THAT YOUR AUDIT DID NOT SURFACE

**Three contracts you already curated ALSO carry two observation types each:**

    forensics.audio-timestamp-segment/v1
        audio_transcript_segment                      10
        video_embedded_audio_transcript_segment        1
    forensics.audio-roman-urdu-segment/v1
        audio_roman_urdu_segment                       6
        video_embedded_audio_roman_urdu_segment        1
    forensics.audio-observation/v1
        audio_technical_observation                    4
        video_embedded_audio_technical_observation     1

**This is NOT the image-observation problem.** There the FIELDS differ, so one
entity would describe fields most of its rows lack. Here the fields are
identical, so splitting is probably wrong — a transcript segment is a transcript
segment whether it came from a `.wav` or from a video's audio track.

**But it is not nothing.** An analyst asking *"how many audio segments were
transcribed?"* gets 11, and 1 of those came from a video. Nobody is told. The
same analyst asking *"which audio files were transcribed?"* would be surprised.

**Rule on it, per contract, and argue whichever way the data supports:**

    (a) leave it whole and curate the provenance as a FIELD (a "source modality"
        the analyst can group and filter by), or
    (b) split into standalone vs video-embedded entities, or
    (c) leave it whole and state the mix in the entity description.

I lean (a) — it keeps one vocabulary for one concept and makes the distinction
askable rather than hidden — but you have read these payloads far more closely
than I have, so make the call and show the evidence. **If `observation_type` is
the only field that distinguishes them, say that plainly**, because then (a) is
nearly free.

## 3. YOUR PII RECOMMENDATION — accepted, and what happens to it

The case-scoped stable alias (`Plate candidate ••••-A7C2`) over last-four is the
right call, and your reasoning is the reason: plate strings are LOW ENTROPY, so
last-four both reveals too much and collides. Typed placeholders
(`[phone masked]`, `[plate masked]`) for OCR and transcript text, with
synchronised redaction across the Urdu and Roman-Urdu forms, is also accepted.

**And your fallback is the part I want on the record:** *if token-level redaction
cannot be proven, withhold the entire text while retaining counts, timestamps,
language, review state and locators.* That is the correct default and it is what
will ship if the redaction cannot be demonstrated per token.

Implementation is mine (`CatalogFields` currently drops PII outright). **Do not
change any field's `sensitivity` to route around it.**

## 4. NOT YOURS

`api/**`, the executor, deployment, live evaluation, the numeric-offset
subtraction for "which plate was visible longest", the denominator obligation
from §1 above, and PII masking at projection.

## 5. VERIFICATION

1. `go test -C api/forensic_records -run TestWI4 .` — it compiles now, so run it.
2. Full Go suite green.
3. Read-only SQL for the two split entities, inside `BEGIN READ ONLY`, showing
   per-field presence within each `observation_type` — a field present on 20/20
   technical rows may be present on 0/6 frames, and that is precisely what the
   split exists to separate.
4. For §2, the SQL that establishes whether `observation_type` is the only
   distinguishing field.

## 6. REPORT BACK

The two split entities with per-field presence inside each partition. Your ruling
on the three audio contracts with the evidence behind it. And anything in the
split that turned out NOT to be as clean as the 20/6 count suggests — a field
that is 20/20 on one side and 1/6 on the other is more interesting than either
number alone.
