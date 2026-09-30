# WI-LAYER-3 — curate the MEDIA families. Your §2 ruling was half right, and the half that was wrong is the biggest capability gap in the product.

WI-LAYER-2 is **accepted**, and the join work was right — especially the part
nobody asked for. Removing `cdr.msisdn -> subscriber.msisdn` and
`ipdr.subscriber_id -> subscriber.msisdn` because they match **0 rows** is worth
more than the edge you kept: a declared join that matches nothing manufactures a
confident empty answer, and you found two of them before they could.
`cdr.lac_id -> tower.lac` as same-LAC reference context, explicitly not proof of
a serving site, is the right way to state a weak relationship.

Verified independently: `Joins` has exactly one runtime consumer, the layer
validator (`semantic_layer.go:259`). Nothing plans or executes on it. Your
"loaded and validated only" is accurate.

---

## 1. THE MEDIA RULING — revised, with the evidence you did not have

You wrote:

> document, image, audio and video remain outside the algebraic semantic layer.
> Their working questions are governed retrieval over page/region/timed
> artifacts; duplicating them as YAML entities would create a second vocabulary
> without enabling a typed join or aggregate.

**For RETRIEVAL that is correct and it stands.** "What does the case-notes
document say about plate ABC-123" is retrieval, the dedicated executor owns it,
and a YAML entity would only create a second vocabulary to disagree with.

**For DERIVED OBSERVATIONS it is wrong, and the data says so.** OCR lines, ANPR
reads, face detections, transcript segments and plate groups are not
unstructured. They are TYPED ROWS in `forensic.derived_artifacts`, with numeric
confidences, normalized text, review states and time offsets. Measured on
`nexusai-multimodal-product-acceptance`:

    forensics.image-ocr-observation/v1        309   normalized_text, recognition_confidence,
                                                    script_family, language_claim, review_state
    forensics.anpr-observation/v1             307   normalized_plate_text, ocr_confidence,
                                                    detection_confidence, manual_review_required
    forensics.document-native-text-passage/v1 115   text, text_sha256, native_text_only
    forensics.image-observation/v1             26   frame_number, sampling_interval_seconds
    forensics.video-anpr-plate-group/v1        24   normalized_plate_text, first_seen_seconds,
                                                    last_seen_seconds, sightings_count
    forensics.image-fingerprint/v1             22   perceptual_hash, perceptual_hash_algorithm
    forensics.face-observation/v1              20   detection_confidence, quality, review_state
    forensics.image-embedding-observation/v1   18   embedding_model, review_state
    forensics.audio-timestamp-segment/v1       11   text, start_seconds, end_seconds,
                                                    detected_language, detected_language_probability
    forensics.audio-roman-urdu-segment/v1       7   roman_urdu_text, raw_urdu_text, identifiers
    forensics.audio-observation/v1              5   container_probe, source_metadata
    forensics.video-observation/v1              1   container_probe, source_metadata

**865 artifacts, and every analytical field is typed.** These questions are
ALGEBRA, and nothing but a hand-written template can answer them today:

    how many plates were read from the videos?
    which plates still need manual review?
    what is the average OCR confidence on the images?
    which plate was visible longest in the video?          (last_seen - first_seen)
    how many faces were detected, and how many are low quality?
    what languages were detected in the audio?
    which audio segments mention 923001110001?

**This is the whole reason the product still depends on predefined operations
for media.** The structured families got a curated layer and stopped needing
templates. Media never did.

## 2. YOUR ITEM — curate the derived-observation entities

### The contract extension you are writing against

`SemanticLayerEntityV1` binds an entity to `forensic.records` through
`record_type`. The backend track is adding an optional source block so an entity
can bind to `forensic.derived_artifacts` instead:

```yaml
contract_version: forensics.semantic-layer-entity/v1
family: image_ocr
record_type: image_ocr_observation
source:
  table: derived_artifacts              # omit for the default, forensic.records
  artifact_type: forensics.image-ocr-observation/v1
  payload: metadata                     # the jsonb column holding the fields
```

**This block does not exist yet.** Do not wait for it and do not build it — it is
`api/**` and it is mine. Write the YAML against this shape; I land the loader and
validator, and `TestWI4` then accepts or rejects your files. If the shape does
not fit what the data needs, say so in your report rather than inventing an
alternative.

Note the payload nesting: most types carry their analytical fields under
`metadata->observation`, a few directly under `metadata`. Establish which per
type from the data — do not assume.

### Deliver in this order, highest analytical value first

    1  forensics.anpr-observation/v1           plates read from IMAGES
    2  forensics.video-anpr-plate-group/v1     plates tracked through VIDEO
    3  forensics.image-ocr-observation/v1      text read from images
    4  forensics.audio-timestamp-segment/v1    transcript segments with time
    5  forensics.audio-roman-urdu-segment/v1   the Roman-Urdu representation
    6  forensics.face-observation/v1           face detections
    7  forensics.document-native-text-passage/v1
    8  the container/frame/fingerprint types, or a reasoned decline

## 3. FOUR RULES THAT ARE NOT NEGOTIABLE HERE

**(a) A DERIVED MODEL OBSERVATION IS NOT A SOURCE RECORD.** Every one of these
rows carries `source_truth_state: derived_model_observation`. The structured
`anpr` family in `forensic.records` is INGESTED SOURCE DATA; an
`anpr-observation` is what a model *thinks* it read off a pixel crop. **They must
be separate entities with different display names**, and no description may let
one be read as the other. Answering "there were 307 ANPR sightings" by conflating
them is a fabricated sighting, which is the one thing this product must never
produce. Make the distinction impossible to miss in `display_name` and
`description` — something like "ANPR plate read (model observation, unreviewed)",
never "ANPR sighting".

**(b) `review_state` AND `manual_review_required` ARE LOAD-BEARING.** Curate
them, describe them honestly, and say in the entity description that these are
unreviewed model output. An analyst asking "which plates were seen" must be able
to learn that the answer is what a model read, not what a camera recorded.

**(c) DO NOT CURATE EMBEDDINGS AS QUERYABLE FIELDS.** The vectors in
`face-observation` and `image-embedding-observation` are for similarity retrieval
only, and embeddings must never drive a structural decision — family, grouping,
measure or field role. Curate `embedding_model`, `embedding_status` and
`review_state`, which are metadata ABOUT the vector; never the vector. Same for
`perceptual_hash`: curate the algorithm and the fact a hash exists, not the hash
as an askable value.

**(d) PII.** A face crop, a plate, and a transcript carrying an identifier are
PII or RESTRICTED. Use `sensitivity:` exactly as you did for `cnic`. Masking is
enforced server-side at projection, never in the UI.

## 4. THE TIME FIELDS ARE THE POINT OF THE VIDEO AND AUDIO ENTITIES

`first_seen_seconds`, `last_seen_seconds`, `start_seconds`, `end_seconds` and
`frame_timestamp_seconds` are what make "which plate was visible longest" and
"what was said between 00:30 and 01:00" answerable. Type them NUMBER so they
compare numerically — **a numeric field typed STRING compares as text and MAX
returns the lexicographically largest value**, the defect that once returned
9,200 as the largest of 75,000.

`sightings_count` on a plate group is likewise NUMBER, and is the natural
`default_measure` for that entity.

## 5. WHAT IS STILL NOT YOURS

`api/**`, the executor, deployment, the live suites. And **do not curate the
retrieval surface**: document passages sit at position 7 precisely because they
are the boundary case. A passage has `text` and `text_sha256` and little to
compute over, so if you conclude it belongs with retrieval rather than algebra,
**that is a complete and welcome answer**. The same honesty that produced "I
cannot establish what `TAC` holds" applies here.

## 6. VERIFICATION

1. A read-only SQL oracle per entity, inside `BEGIN READ ONLY`, showing for each
   curated field: non-null count, distinct count, and for NUMBER fields min and
   max. **A field you cannot show real values for must not be curated.**
2. Every `artifact_type` string must match the database EXACTLY — they carry
   `/v1` version suffixes, and a typo yields an entity that silently matches zero
   rows, which is the confident-empty-answer failure again.
3. `go test -C api/forensic_records -run TestWI4 .` once the contract lands.
4. Full Go suite green.

Do not run the live evaluation — that is the backend track's gate.

## 7. REPORT BACK

Per entity: fields curated, fields deliberately excluded and why, the SQL
evidence, and the `sensitivity` decisions. Name anything whose meaning you cannot
recover from the data — an honest "I do not know what `crop_quality` measures"
beats a guessed description, because a guessed description is exactly how
"average beam width" was answered with azimuth.

**And state plainly whether you still think any of these belong outside the
layer.** You were right to push back in WI-LAYER-2, and I want the same judgement
here, applied to each type separately.
