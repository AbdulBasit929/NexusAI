# STAGE 1, SLICE 1 — an entity can declare WHERE ITS ROWS LIVE

Landed 2026-09-26. Source-only; nothing deployed, no live evaluation.

## What shipped

`SemanticLayerEntityV1.Source`:

    source:
      table: derived_artifacts          # omitted means records
      artifact_type: forensics.anpr-observation/v1
      payload: metadata

**Omission is the default and means `records`.** The seven structured entities
carry no source block and are bit-for-bit unchanged — changing the issued field
set changes plans for questions that already work, proven twice, and it cost a
false negative both times.

Validation, each rule bought by a failure mode:

    table       allowlisted (records | derived_artifacts). A table named by a
                YAML file is a table named by whoever can write one.
    payload     allowlisted per table. Same reason.
    artifact_type
                REQUIRED for derived, and must be a versioned forensics
                contract. Without one the entity matches EVERY contract in the
                table -- an OCR entity counting face detections -- and the
                number looks entirely authoritative. A typo in the /v1 suffix
                matches ZERO rows, which reads as "there is no such evidence".
    nested paths
                allowed for derived only ("observation.start_seconds"), because
                an observation keeps its analytical fields under one object.

## THE REGRESSION THIS CAUSED, AND WHAT IT TAUGHT

The first implementation split EVERY source name on ".". `TestWI4` failed
immediately: `anpr.plate_number` binds to the CSV header **"Registration No."**,
which ENDS IN A PERIOD. A records source name is a LITERAL KEY and real keys
contain dots. Splitting is the declared convention for derived contracts and
only those.

`TestSemanticSourceRecordsNameWithADotIsLiteral` pins it.

## Verification

    15 entities load and validate      7 structured + 8 media
    TestWI4 rewritten                  asserts BOTH sets separately, and fails
                                       if a structured family ever binds to
                                       derived_artifacts or a media family ever
                                       drops its artifact_type
    full Go suite                      green
    8 artifact_type strings            verified against the database: zero typos
    row counts                         307/24/309/11/7/20/22/18, all match

## WHAT THIS SLICE DOES NOT DO

**The executor still reads `forensic.records` only.** The contract exists; the
binding does not. No media question is answerable yet. That is the next slice,
and it was kept separate deliberately: the executor is the hot path for all 62
questions, and bundling a new data source into it would make any regression
unattributable.

Remaining for the executor slice:

    1  resolve table + payload + artifact_type per plan, REFUSING a plan that
       mixes sources rather than silently joining them
    2  nested jsonb extraction (metadata->'observation'->>'key')
    3  provenance: derived rows carry source_truth_state =
       derived_model_observation and the answer must say so
    4  lineage expression names derived_artifacts, not forensic.records
    5  projection over derived rows needs its own provenance mapping --
       derived_artifacts has no record_id/row_number/batch_id/row_hash

## TWO FINDINGS FOR THE ANSWER PATH, both measured

**PII fields are dropped from the issued catalogue** (`CatalogFields` skips
PII/RESTRICTED), so the 7 curated PII fields — plate text, OCR text, both
transcript forms — cannot be referenced by any plan. "How many plates were read"
works; "which plates were read" does not. A decision, not a bug: widening PII
exposure belongs in its own change with masking at projection.

**Sparse fields will answer about a subset.** `image-ocr-observation` carries
THREE confidence fields from two disjoint producers — `confidence` on 263 of
309 rows, `detection_confidence` and `recognition_confidence` on 46 each. A
general question binding to a 46-row field states a number about 15% of the
evidence as though it described all of it. **That is the beam-width defect in a
new place:** a correct AVG over the wrong field, nothing malformed in the plan.
Curation fix assigned in WI-LAYER-4; an S9 obligation may also be warranted.

---

# STAGE 1, SLICE 2 — the executor reads derived artifacts

Landed 2026-09-26, behind `FORENSIC_DERIVED_ARTIFACT_EXECUTION`, default OFF.
Source-only; nothing deployed, no live evaluation.

## Proven against the real database, not by reading SQL strings

    COUNT over forensics.video-anpr-plate-group/v1   = 24, matching the table
    AVG over observation.crop_quality (nested path)  = 0.266055, matching the table

Both compare against an independent SQL oracle in the test itself.

## THE 9,580 DEFECT — found by that test, not by reading the code

The first implementation resolved the binding from the PLAN's field IDs. A plain
`COUNT(*)` carries **no field ID**, and *"how many plate groups were read from
the videos?"* is exactly that plan. With nothing to resolve it fell through to
the records default and answered **9,580** — every structured row in the
collection — where the truth is **24**.

    9580 = 5000 cdr + 2500 ipdr + 1057 anpr + 1000 access_log + 9 + 5 + 5 + 4

**A count that large over forensic evidence reads exactly like a right one.**
Nothing in the plan was malformed and no verifier could have caught it.

The fix resolves from the ISSUED CATALOGUE when the plan names no field, because
the catalogue is what BOUNDED the plan: the generator could only reference fields
it was issued, so the issued set names the evidence the question was compiled
against. If the catalogue spans more than one source and the plan names no field,
the plan is REFUSED — a COUNT has to be able to say what it counted.
`TestDerivedBindingResolvesACountStarFromTheIssuedCatalogue` pins it.

## What the slice does

    binding resolved ONCE per plan from the curated layer, never from
      FieldDescriptorV1 -- that struct is serialized into the model payload, and
      adding to it changes the issued field set, which changes plans everywhere
    MIXED SOURCES REFUSED   an ingested record and a model observation cannot be
      counted together; reading one silently answers about evidence nobody asked
      about, and joining them asserts a relationship the data does not establish
    nested jsonb paths      metadata->'observation'->>'key', every segment BOUND
    artifact_type filter    or the entity matches every contract in the table
    processing_status       only `completed` artifacts are evidence
    derived lineage         names the artifact, its evidence, its run, and
      source_truth_state -- the field that stops a model read being read as a
      camera sighting
    PROJECTION REFUSED for derived rows, explicitly. derived_artifacts has no
      record_id, row_number, batch_id or row_hash, and listing observations
      without provenance hands the analyst rows it cannot attribute.
    request target/date constraints NOT applied to derived plans: they name
      r.primary_target and the records timestamp, which a derived artifact does
      not have. Applying them is the H2 defect -- a constraint the row cannot
      satisfy, matching nothing, reporting false absence over evidence present.

## A SECOND REGRESSION THIS CAUSED, and what it taught

Splitting every source name on "." broke the SHIPPED layer: `anpr.plate_number`
binds to the CSV header **"Registration No."**, which ends in a period. A records
source name is a literal key and real keys contain dots. `TestWI4` caught it on
the first run.

## RISK THE SWITCH DOES NOT COVER — the live measurement is still owed

The switch gates EXECUTION. It does NOT hide the 8 media entities from the
COMPILER: they are in the layer now, so family selection could in principle
offer one to a structured question. **Adding to the issued vocabulary has changed
plans twice before, and cost a false negative both times.**

Offline evidence so far: `TestFamilyCatalogAudit` over the 62 reports **zero
starved enums** and surfaces no media family. That is a good signal and it is not
a substitute for the live run.

**Before enabling: run the 62 with a control on the same container, and read the
result against the pre-declared threshold.** Any question moving to WRONG or
NOT_STATED reverts the layer visibility, not just the switch.
