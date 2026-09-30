# WI-LAYER-4 — your entities now LOAD. Three things the data says are still missing.

WI-LAYER-3 is **accepted**, and the contract you were blocked on has landed.

## What I verified independently, not taking your word for it

    all 8 artifact_type strings          match real rows, ZERO typos
    row counts 307/24/309/11/7/20/22/18  match your report EXACTLY
    62 fields + 8 metrics = 70           matches your count
    7 PII fields, 55 NONE                plate text, OCR text, transcripts
    embeddings / hashes / digests        NOT curated as queryable values

The loader, validator and full Go suite now pass with **15 entities** — your 8
alongside the 7 structured families. `TestWI4ShippedLayerLoadsAndValidates` was
rewritten to assert both sets separately and to fail if a structured family ever
binds to `derived_artifacts`, or a media family ever stops declaring its
`artifact_type`.

**The contract-version mismatch was MY error, not yours.** I wrote
`forensics.semantic-layer-entity/v1` in the WI-LAYER-3 prompt; the loader
requires `forensics.semantic-layer/v1`. You followed the spec and flagged the
conflict instead of silently changing it, which is exactly right. I corrected
the eight files.

**Your three refusals all stand.** Document-native passages are retrieval;
container observations expose objects, not scalars; and `image-observation`
genuinely does mix 20 container/still rows with 6 sampled-frame rows. Refusing
to curate a mixed contract is the same judgement that produced "I cannot
establish what `TAC` holds".

---

## 1. SPARSE FIELDS — the one thing the audit found that your report did not

Measured on `nexusai-multimodal-product-acceptance`:

    image-ocr-observation, 309 rows
        observation.confidence               263 non-null   85%
        observation.detection_confidence      46 non-null   15%
        observation.recognition_confidence    46 non-null   15%

    audio-timestamp-segment, 11 rows
        observation.detected_language          4 non-null   36%

263 + 46 = 309, so **two producers emit disjoint schemas** and you were right
not to conflate them. But an analyst does not know that, and this is now a
confident-wrong risk:

> *"What is the average OCR confidence on the images?"*

Three curated fields can answer it. Two of them compute over **46 of 309 rows**
and would state a number about 15% of the evidence as though it described all of
it. **That is the beam-width defect in a new place**: a correct AVG over the
wrong field, and nothing in the plan is malformed.

**What to do — and it is curation, not code:**

    (a) SYNONYMS DECIDE THE BINDING. Make sure the general words -- "OCR
        confidence", "confidence", "how confident" -- resolve to the field that
        covers 263 rows, and that the two producer-specific fields carry only
        producer-specific synonyms. A general question must not bind to a
        15%-coverage field.
    (b) SAY THE COVERAGE IN THE DESCRIPTION. "Present on 46 of 309 observations;
        emitted only by the <name> recognizer" is the honest sentence. The
        analyst reading the field can then see what the number is about.
    (c) Do the same for `detected_language` at 4 of 11.

**Check every other curated field for this**, not only the ones I found. Report
presence counts per field. A field below roughly half coverage needs its
denominator in its own description, or it will eventually answer a question it
does not cover.

## 2. `image-observation` — propose the split, do not perform it

You are right that it needs a source-contract split before curation. **Propose
the split**: what distinguishes the 20 container/still rows from the 6
sampled-frame rows in the payload itself, whether that distinction is reliable
on every row, and what two entities would look like. If the payload cannot
separate them reliably, say so — that is a complete answer and it stops us
curating a contract that cannot be trusted.

## 3. PII — a decision I owe you the facts for, before you act on it

`CatalogFields` currently **drops every PII and RESTRICTED field** from the
issued catalogue:

```go
if field.Sensitivity == "PII" || field.Sensitivity == "RESTRICTED" { continue }
```

So your seven PII fields — plate text, OCR text, both transcript forms — are
curated but **cannot be referenced by any plan today**. The consequence:

    ANSWERABLE NOW    how many plates were read · how many need review ·
                      average confidence · what languages · how many faces
    NOT ANSWERABLE    which plates were read · what does the transcript say ·
                      which plate was visible longest (it needs the plate)

**Do not change the sensitivity of a single field to work around this.** The
masking decision is server-side at projection and it is mine to land. What I
need from you is the list: **which questions in your entities become answerable
only if masked PII projection is allowed, and what the masked form of each field
should look like** (last-4? first-2-last-2? fully masked with a count?). A plate
is not a CNIC and may not deserve the same treatment — argue it per field.

## 4. NOT YOURS, so you do not wait on it

`api/**`, the executor table binding, deployment, live evaluation. Also **the
numeric-offset subtraction** that "which plate was visible longest" needs: the
expression allowlist supports timestamp subtraction, not numeric offsets. You
identified it correctly; it is mine.

## 5. VERIFICATION

1. `go test -C api/forensic_records -run TestWI4 .` — it now runs against your
   files for real, so run it.
2. Full Go suite green.
3. Read-only SQL for every presence count you report, inside `BEGIN READ ONLY`,
   without selecting PII values — exactly as your WI-LAYER-3 oracle did.

## 6. REPORT BACK

Per entity: presence count per field, which descriptions you changed to state a
denominator, and which synonyms you moved and why. For `image-observation`: the
proposed split or a reasoned refusal. For PII: the per-field masked-form
argument.

**And say plainly if you think the synonym rebinding in §1 is wrong.** You
audited these payloads more closely than I did, and if the 263-row `confidence`
field is not the right general binding, I would rather hear it now than measure
it later.
