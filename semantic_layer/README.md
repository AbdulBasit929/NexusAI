# NexusAI semantic layer — governed entity contracts

This directory is **data, not code**: one YAML file per evidence family, hand-curated,
reviewable in a diff, and validated by `api/forensic_records/semantic_layer.go`.
Structured source-record and derived-media entities both use
`forensics.semantic-layer/v1`. Derived-media entities additionally bind explicitly to one
exact `forensic.derived_artifacts.artifact_type` through their `source` block.

It exists because the runtime catalogue is inferred per request from sampled rows
(`buildSourceNativeFieldCatalog`) and therefore has **no descriptions, no synonyms, no
value domains, no metrics and no joins**. `grep -c Description source_native_algebra.go`
returns 0. Nothing tells a model that `call_org_num` means "originating number", or that
`status` and `account_status` are the same concept stored under two headers.

## What every entry must carry, and why

Each requirement below traces to a **measured** failure, not a preference.

| Element | Closes | Evidence |
|---|---|---|
| `description`, `display_name`, `synonyms` | a model cannot reason about `inv_tot` or `Lac_Id` | WI-0: curated IDs + descriptions were the working configuration at 74.3% |
| `source_names` (aliases) | **SUB-02** — "how many subscribers are active" returns 11, not 6 | `status` holds 5 values, `account_status` holds only `ACTIVE`; the correct answer needs both |
| `values` (enumerated domain) | **the SUB-02 class** — "active" is not an identifier pattern, so S2 never made it an obligation and the CONSTRAINT_APPLIED guard had nothing to check | WI-0 §5, confirmed live 2026-09-21 |
| `metrics` with `expression` | **CDR-15** "the longest call" | the CDR payload has **no duration column** — only `CALL_START_DT_TM` and `CALL_END_DT_TM`. SQL confirms the oracle 1798 is exactly `MAX(end − start)` |
| `default_measure` | **TXN-02** "the largest transaction" | with no declared default the compiler can only abstain; WI-3 made it abstain rather than answer COUNT |
| `distinct_capable` | **CDR-07, CDR-08, ANPR-05** | the IR now issues `COUNT_DISTINCT` directly over declared identifier fields; CDR-08 and ANPR-05 have executable gold plans rather than stale inexpressible markers |
| `joins` | cross-family questions | no join graph is declared anywhere today |

## Field IDs are readable and stable

`sourceNativeFieldID` mints runtime IDs as `fld_` + a SHA-256 prefix, scoped to
tenant/collection/evidence. Those are unstable across cases and meaningless to a model
asked to pick one from an enum. The curated layer uses `<entity>.<field>` — `cdr.call_type`,
`subscriber.status` — which is stable, diffable and reviewable. The enum mechanism that
makes a hallucinated field structurally impossible works identically either way.

## Expressions are allowlisted, never SQL

A metric may carry an `expression`, but only from a fixed vocabulary:

- `timestamp_diff_seconds(<field_id>, <field_id>)` — elapsed seconds between two timestamps

The validator rejects anything else. A metric never carries raw SQL, and the layer never
names a table.

## Sensitivity

`sensitivity: PII` fields are masked server-side at projection. The masking is a control,
not a display preference: the golden oracle for SUB-03 expects an **unmasked** CNIC and is
wrong to do so — an oracle that demands unmasked PII rewards a violation.

## Validation

`LoadSemanticLayer` fails closed. It rejects duplicate field IDs, unknown types, an
aggregate a field does not allow, a metric referencing a missing field, a join whose two
sides are in the same entity, a `default_measure` that resolves to nothing, and — importantly
— **a synonym that maps to two fields in the same entity**, because an ambiguous synonym is
how a filter silently binds to the wrong field.

Curation order (one family at a time, as each is reviewed):
**cdr → subscriber → ipdr → anpr → access_log → tower → transaction**.

## Deliberate exclusions

- `generic` / `Case notes for records-demo` is free-text document content, not a
  structured column. It remains preserved in source data but is not promoted into
  a structured semantic field.
- `tower_location` / `TAC` occurs in five source rows, but this dataset does not
  establish what the acronym represents in that export. It remains uncurated until
  a source contract or domain owner supplies the meaning; the layer does not guess.

## Join graph

Join declarations are evidence-backed contracts, not suggestions to the planner. The
read-only audit in [`audits/wi-layer-2-join-audit.sql`](audits/wi-layer-2-join-audit.sql)
was run against `nexusai-forensic-demo` on 2026-09-25 before this graph was changed.

The one relationship supported by both the current contract and retained demo rows is:

| Join | Cardinality | Non-empty left/right rows | Exact joined pairs | Meaning |
|---|---|---:|---:|---|
| `cdr.lac_id` → `tower.lac` | `many_to_one` | 8,591 / 4 | 2 | The retained tower source has four non-empty, unique LAC values. The edge supplies same-LAC reference context; it does not identify the serving site. |

This cardinality is source-audited, not a universal claim about LACs. A later tower
source with duplicate non-empty LAC values must not activate this edge without a new
cardinality contract; otherwise an equality join would multiply CDR rows.

The v1 runtime currently loads and validates `joins`, but no planner or executor consumes
them. This declaration therefore records the governed graph; it does not by itself make a
cross-family question executable. Duplicate-safe join planning and execution remain a
backend contract change outside this curation item.

The following plausible-looking edges are deliberately not declared:

- `cdr.msisdn` → `subscriber.msisdn`: 8,642 CDR rows and 11 subscriber rows,
  **0 exact pairs** and 0 pairs after conservative Pakistan-number formatting
  normalization. The previous declaration was false for the retained data.
- `ipdr.subscriber_id` → `subscriber.msisdn`: 2,500 IPDR rows and 11 subscriber
  rows, **0 exact or conservatively normalized pairs**. The previous declaration
  was false for the retained data.
- `cdr.msisdn` ↔ `ipdr.subscriber_id`: 3,579,534 exact row pairs covering 8,634
  CDR rows and all 2,500 IPDR rows. Both sides repeat the identifier, so this is
  `many_to_many`. The v1 loader does not accept that cardinality and `api/**` is
  outside this work item; declaring either directional one-to-many shape would
  silently understate row multiplication.
- `cdr.cell_site_id` → `tower.site_code`: two apparent pairs reach one tower row,
  but both disagree with the LAC on the same rows. They are treated as namespace
  collisions, not site matches. `cdr.site_id` → `tower.site_code` has 0 pairs.
- `transaction.account` → subscriber number or subscriber ID: **0 pairs** for
  both candidate meanings. No account-to-person relationship is inferred.
- ANPR location or coordinate pair → tower location or coordinate pair: **0 exact
  pairs**. Approximate spatial correlation needs an explicit distance/uncertainty
  contract; it is not represented as equality.

### Media families

The retrieval surface remains outside this algebraic layer: a question asking what a
document passage says is governed cited retrieval, not a structured aggregate. Typed
derived observations are different. OCR regions, model plate reads, grouped video plate
reads, timed transcript segments, Roman-Urdu representations, face candidates and
vector/fingerprint metadata are rows with governed scalar fields, so they use the same
`forensics.semantic-layer/v1` entity contract with an explicit derived-artifact source.

Every media entity binds to an exact `/v1` artifact type. `source.payload: metadata`
selects the JSONB payload column, and dotted `source_names` such as
`observation.start_seconds` resolve nested payload fields through the shipped source loader.
`source_truth_state`, `review_state` and `manual_review_required` remain first-class where
the retained rows contain them. Model-derived ANPR reads are intentionally separate from
the `anpr` source-record entity and are never described as sightings.

The read-only field oracle in
[`audits/wi-layer-3-media-field-audit.sql`](audits/wi-layer-3-media-field-audit.sql)
was last run against `nexusai-multimodal-product-acceptance` on 2026-09-26. It matched exact
artifact counts of 307 ANPR model reads, 24 video plate groups, 309 image OCR regions,
11 timestamped audio segments, 7 Roman-Urdu representations, 20 face candidates,
22 image fingerprints and 18 image-vector observations. For every curated field it reports
non-null count, retained-corpus coverage percentage and distinct count and, for numeric
fields, minimum and maximum.

#### Sparse fields and binding

The retained image-OCR corpus contains two disjoint producer shapes: 263 Tesseract general
scene-text observations carry `observation.confidence`, while 46 PaddleOCR multilingual
observations carry `recognition_confidence` and `detection_confidence`. The general phrases
“OCR confidence”, “confidence” and “how confident” therefore belong only to
`image_ocr_observation.general_ocr_confidence`. The two 46-row scores have producer-specific
synonyms. This is deliberate: a general average over 46 of 309 rows would be numerically
correct but analytically false about corpus coverage. The rebinding is the least-wrong
curation choice, not permission to describe the result as all-image confidence: no one score
exists on all 309 observations, so any general average must disclose its 263/309 denominator.

Every curated field below half coverage states its retained-corpus denominator in its own
description: the PaddleOCR text/confidence/review fields (46/309), OCR frame offset (40/309),
audio detected language (4/11), audio timing status (5/11), and image-vector frame offset
(2/18). The related 263/309 Tesseract confidence, language-claim and review fields also state
their producer and denominator because the producer schemas are complementary.

#### Discriminator-backed split for `forensics.image-observation/v1`

The read-only oracle in
[`audits/wi-layer-4-image-observation-split-audit.sql`](audits/wi-layer-4-image-observation-split-audit.sql)
proves a reliable discriminator on all 26 retained rows:

- 20 rows have `metadata.observation_type = image_technical_observation`, carry
  `observation.source_metadata`, and carry none of the frame/sampling fields.
- 6 rows have `metadata.observation_type = video_sampled_frame_observation`, carry
  `frame_number`, `frame_number_semantics`, `sampling_interval_seconds` and
  `sampling_strategy`, and do not carry `source_metadata`.
- Discriminator violations: 0.

The source contract now matches `source.observation_type` exactly, as a bound predicate
against `metadata ->> 'observation_type'`. Two curated entities use it:

- `image_technical_metadata` exposes governed scalar dimensions plus format, decode,
  validation and privacy fields from the 20 technical rows. Orientation-adjusted
  dimensions and orientation metadata are the deliberate sparse fields at 8/20; every
  other curated technical field is 20/20.
- `video_sampled_frame` exposes frame number, number semantics, sampling interval,
  sampling strategy and storage state from the 6 sampled-frame rows. Six rows are a small
  population: aggregates are arithmetically valid but forensically thin.

Hashes, parent identifiers, extractor/backend identifiers, intake limits and the
`requested_analysis_roles` array remain excluded. The independent oracle in
[`audits/wi-layer-5-discriminator-curation-audit.sql`](audits/wi-layer-5-discriminator-curation-audit.sql)
reports presence inside each partition and the cross-partition 20/0 versus 0/6 field shape.

#### Audio source-modality ruling

The three audio contracts do not support one undifferentiated statement that their field
shapes are identical:

- `forensics.audio-timestamp-segment/v1` has 10 standalone-audio rows and 1
  video-embedded row. Its analyst-meaningful transcript fields are shared, so the entity
  stays whole and adds a groupable/filterable `source_modality` field. `storage_state`
  exists only on the embedded derivative and `timing_status` is sparse, but neither
  changes what a transcript segment means.
- `forensics.audio-roman-urdu-segment/v1` has 6 standalone-audio rows and 1
  video-embedded row. Its representation fields are shared, so it also stays whole and
  exposes `source_modality`; the embedded-only `storage_state` is lineage, not a second
  transcript vocabulary.
- `forensics.audio-observation/v1` is materially different. Four standalone rows expose
  audio-stream metadata such as sample rate, channels and bits per sample; the one
  video-embedded row exposes container/track metadata instead. Only a small common core
  overlaps. It remains outside the semantic layer in this slice: one whole entity would
  recreate the mixed-schema defect, while two one-purpose technical entities need a
  separately bounded curation decision. No internal extractor or container inventory is
  exposed merely to make the contract appear complete.

Thus option (a), source modality as a field, is adopted for the two transcript concepts;
the technical-observation contract is not falsely treated as the same shape and is not
split speculatively.

#### PII projection ruling

WI-LAYER-7 closes the earlier handoff with a field-by-field curation decision. The
read-only oracle in
[`audits/wi-layer-7-pii-synchronization-audit.sql`](audits/wi-layer-7-pii-synchronization-audit.sql)
emits counts and booleans only; it never selects a plate, transcript, identifier or filename.

| Fields | Projection decision | Safe boundary |
|---|---|---|
| `anpr_model_observation.plate_text`, `video_anpr_plate_group.plate_text` | `PII` / `MASKED`: stable alias | Replace the whole value with one opaque, case-scoped alias shared across both families. Revealing a prefix, suffix or length is not approved. |
| `image_ocr_observation.raw_text`, `image_ocr_observation.normalized_text` | `PII` / `WITHHELD` | Token masking is not sufficient: OCR text can contain names and addresses that these rows do not type. Withhold from typed projection and untargeted browse. |
| `audio_timestamp_segment.text` | `PII` / `WITHHELD` | Withhold from typed projection and untargeted browse. Keep count, source, language, time range and locator. |
| `audio_roman_urdu_segment.raw_urdu_text`, `audio_roman_urdu_segment.roman_urdu_text` | `PII` / `WITHHELD` | The two forms inherit the same sensitivity. Withhold both from typed projection and untargeted browse. |

Governed targeted retrieval is deliberately separate from typed projection. An analyst who
supplies a term, identifier or time range may still receive the bounded matching snippet as
cited evidence; this ruling does not weaken that working path or turn a targeted search into
a corpus browse.

The present retained data proves three relevant token classes, without disclosing values:

- all 307 individual and 24 grouped plate values are vehicle-registration candidates and
  are PII as whole values;
- one of 11 timestamp transcripts contains a phone-like token; the same source token is
  carried by one of seven Urdu rows and its paired Roman representation, so those three
  matches are not three independent people or identifiers;
- 22 OCR raw rows and two OCR normalized rows contain Arabic-Indic or Eastern-Arabic digit
  characters. Their meaning cannot be established from shape alone, so they are not
  relabelled as phones, CNICs or accounts.

The bounded patterns found zero CNIC-like, email-like, IPv4-like or Pakistan-IBAN-like rows.
That negative result does not make free text safe: person names, addresses and contextual
identifiers are not exhaustively recognizable by token syntax. Any future token redactor
must at least cover phone/MSISDN, CNIC, full plate, email, attributable IP, and typed
financial account/card/IBAN/wallet identifiers across ASCII, Arabic-Indic and
Eastern-Arabic digits. Until a redactor proves that wider boundary, free text stays
`WITHHELD`; typed placeholders are not treated as an implemented capability.

The Urdu and Roman-Urdu values are not independent model outputs. The producer derives both
inside the same artifact using the versioned deterministic transliterator
`roman-urdu-m1-v1`. All seven retained rows carry both forms, name that processor revision,
and report identifier preservation. One row contains ASCII digits in both forms; none of
the seven contains Arabic-Indic or Eastern-Arabic digits. The transliterator protects
Unicode decimal-digit identifiers but does not normalize numeral scripts.

Stored parent linkage is not fully reliable: all six standalone-audio derivatives link by
`parent_observation_id` to the exact timestamp segment and match its text, time and locator.
The one video-embedded derivative has a non-matching parent ID, although exactly one
same-evidence/version timestamp segment matches its text, time and locator. Therefore a
future synchronized redactor must operate on the raw/Roman pair held in the derivative
artifact (ideally redact raw Urdu and regenerate the Roman form with the recorded revision),
not assume every stored parent ID can be joined.

The adjacent template fields do not require PII reclassification: language is a model
language tag; `observation_id` is a technical lineage identifier; and `citation_locator` is
a page/region/time locator. `source_file` is an evidence locator retained by the accepted
text-browse guard, not evidence content; filenames remain user-controlled display strings
and must never be parsed into a factual claim. The five text fields above were already PII
and are now explicitly `WITHHELD`; the two plate fields remain PII with whole-value aliasing.

Deliberate boundaries and exclusions:

- `document-native-text-passage` remains retrieval: its only meaningful content is the
  passage text and digest, while `native_text_only` alone does not form a useful algebra.
- `audio-observation` and `video-observation` remain technical container metadata. The
  audio contract's two source-modality payload shapes differ materially; the video payload
  remains object-heavy. Neither is flattened into a misleading common entity here.
- Embedding vectors, perceptual-hash values, source digests, face crops, bounding boxes
  and identifier arrays are not queryable fields. Only metadata about vectors and hashes
  is curated.
- Plate candidates use `sensitivity: PII` with `redaction: MASKED`; the five free-text
  fields use `sensitivity: PII` with `redaction: WITHHELD`. The current dynamic
  `CatalogFields` path omits every PII and restricted field, so any future plate projection
  still requires server-side whole-value aliasing. The YAML alone does not authorize a new
  derived-media projection or weaken the targeted-retrieval boundary.
- `detected_language_probability` and image-embedding `embedding_status` are absent from
  the retained rows, so they are not curated. A declared field that always resolves to
  null would recreate the confident-empty-answer defect.
- Face `quality` is not a quality verdict: the object contains only crop width, height and
  minimum dimension. Those numeric dimensions are curated, but no “low quality” threshold
  is invented.
- The present expression vocabulary only subtracts `TIMESTAMP` fields. Video offsets are
  correctly typed `NUMBER`, so a `last_seen_seconds - first_seen_seconds` duration metric
  remains undeclared until the backend adds a governed numeric-difference expression.

## Provenance

Field names and value domains in these files were read from the live demo case
(`nexusai-forensic-demo`, 12,912 rows) on 2026-09-21 by direct SQL, not copied from code.
Counts anchor to CDR 8,642 · IPDR 2,500 · ANPR 750 · access log 1,000 · subscribers 11 ·
towers 5 · transactions 4.
