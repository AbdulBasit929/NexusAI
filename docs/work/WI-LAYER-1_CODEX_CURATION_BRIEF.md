# WI-LAYER-1 — curate every column, every family

You own `semantic_layer/**`. This item raises the product's **answerable surface**:
how much of the evidence an analyst can ask a question about at all.

---

## 1. Why this is the highest-value work available

The compiler, the guards and the grammar are done and measured: 47 CORRECT ·
14 CLARIFIED · 0 WRONG on the 62-question suite, p95 5.2 s, and a generated
typed plan answers 63% of it. **None of that is the ceiling.**

The ceiling is vocabulary. A column the curated layer does not describe is never
issued in the field enum; the generator therefore cannot name it; therefore no
verified plan can filter, group or measure on it. That is the keystone working
exactly as designed — and it means:

> **Coverage of the semantic layer IS coverage of the product.**

Measured 2026-09-25 against the live database
(`api/forensic_records/semantic_coverage_audit_test.go`, read-only, no model):

    across ALL collections   88 of 148 columns curated   (59%)
    on nexusai-forensic-demo 81 of  94 columns curated   (86%)

Full per-column detail, with row counts:
[`CURATION_GAP_INVENTORY.md`](CURATION_GAP_INVENTORY.md).

**The golden suite cannot see any of this.** It only asks about columns someone
already wrote a question for. This is why the work is invisible in the scorecard
and decisive for real use.

## 2. The single most valuable gap

**Every CDR row carries a location and no question about it can be answered.**

    location    8,642 rows      Site_Id  8,638      Lac_Id  8,638
    lat         8,638 rows      longitude  8,638

"Where was this number seen", "which cell site handled these calls", "map this
subscriber's movement" — on the largest family in the case — are structurally
unanswerable today. Curating these five columns unlocks an entire class of
forensic question.

Note `tower_location` already curates `tower.latitude` / `tower.longitude` /
`tower.site_code`. CDR's location columns are a different, uncurated set, and
`cdr.cell_site_id` (declared, bound to `Cell_SITE_ID`) is not the same column as
`Site_Id`/`Lac_Id`.

## 3. Scope — every family, nothing skipped

Work through all eight in this order (highest row impact first). For each,
curate **every** column in the inventory, or record why one is deliberately
excluded:

| family | columns | not curated | priority |
|---|---:|---:|---|
| `cdr` | 32 | 15 | **first** — 8,600+ rows per column |
| `anpr` | 23 | 15 | second — includes `plate`, `ocr_confidence` |
| `tower_location` | 23 | 15 | third — engineering fields |
| `ipdr` | 19 | 11 | fourth |
| `subscriber` | 35 | 3 | `cnic_last4`, `account_type`, `location` |
| `generic` | 1 | 1 | `Case notes for records-demo` — see §5 |
| `access_log` | 6 | 0 | complete |
| `transaction` | 9 | 0 | complete |

**Do not stop at the demo collection.** Columns appearing only in
`nexusai-multimodal-product-acceptance` are real evidence too; the inventory's
`cols` column tells you how many collections each appears in.

## 4. What a curated field must carry, and why each part earns its place

Follow the existing entries in `semantic_layer/cdr.yaml` — they are the model.
Every key below has been paid for by a measured defect:

```yaml
  - id: cdr.cell_site_location
    display_name: Cell site location          # the analyst reads THIS, never the raw name
    description: >                            # the generator reads this; without it the
      Where the serving cell site is...       # model cannot tell inv_tot from invoice total
    source_names: [location, Site_Location]   # EVERY raw spelling; this is what binds
    synonyms: [where, location, place, area]  # how an analyst actually says it
    type: STRING                              # NUMBER maps to DECIMAL — see below
    sensitivity: NONE                         # PII/RESTRICTED are withheld from the enum
    allowed_filters: [EQ, NEQ, IN, CONTAINS, IS_NULL, IS_NOT_NULL]
    allowed_aggregates: [COUNT]
    projectable: true
    groupable: true
    sortable: false
    distinct_capable: true                    # REQUIRED for COUNT_DISTINCT to be offered
```

**Hard-won rules — violating any of these has caused a live defect:**

- **`source_names` must list every raw spelling.** `Site Code` and `site_code`
  normalise together, but `location` and `Site_Location` do not. A missing alias
  is a silently unanswerable column.
- **`type: NUMBER` maps to DECIMAL.** A numeric column typed as STRING is
  compared as text — `MAX` once returned 9,200 over 75,000 because "9" sorts
  after "7", and it was misdiagnosed as model instability for weeks.
- **`sensitivity: PII` or `RESTRICTED` excludes the field from the dynamic
  enum** (`CatalogFields`). That is deliberate. Use it for personal data, and
  expect questions about that field to clarify rather than answer.
- **`distinct_capable: true` is what puts COUNT_DISTINCT in the grammar.** Set
  it on any identifier-like column an analyst might count uniquely.
- **`groupable: true` only where grouping is meaningful.** A free-text column
  with 8,000 distinct values is not a breakdown dimension.
- **Do not add metrics without an `expression:`.** `CatalogFields` skips them,
  so they reach nothing — two such declarations were deleted as dead weight.

## 5. Judgement calls, to make deliberately and record

- **`generic` / `Case notes for records-demo`** is a free-text note, not a
  column. Curating it as a queryable field would invite structured questions
  about prose. Recommend excluding it and saying so in the YAML.
- **`ocr_confidence` / `detection_confidence` (anpr, 307 rows)** are model
  confidences, not evidence. UX §9 forbids exposing confidence thresholds to
  analysts — but §4.1 requires evidence-strength to be visible. Curate them as
  `sensitivity: NONE`, `projectable: true`, `groupable: false`, and let the UI
  render strength; do not make them a filter an analyst has to reason about.
- **`cnic_last4` (subscriber)** is partial PII. Mark `sensitivity: PII` —
  masking is enforced server-side at projection and the UI never unmasks.
- **Duplicate spellings of one concept** (`Latitude WGS84` vs `latitude`,
  `Site Code` vs `cell_site_id`) belong on ONE field as multiple `source_names`,
  not as two fields. Two fields for one concept is how the generator picks the
  wrong one.

## 6. Verification — measured, not asserted

1. `FAMILY_AUDIT_DB=postgresql://localrecall:localrecall@localhost:5433/localrecall \`
   `go test -C api/forensic_records -run TestSemanticLayerCoverage -v .`
   Report coverage before and after, per family. **Target: no family below 90%
   on the demo collection, and every deliberate exclusion named in §5.**
2. `go test -C api/forensic_records -run TestWI4 .` — the layer's own loader and
   invariant tests must stay green.
3. `go test -C api/forensic_records -run TestFamilyCatalogAudit -v .` (needs the
   same DSN) — confirms the enum each question is issued, and that **no enum is
   empty**: an empty enum is an unparseable grammar and returns HTTP 500 before
   any inference.
4. The full Go suite green.

**Do not run the live 62-question evaluation** — that is the backend track's
gate and it needs a coordinated deploy. Report your coverage numbers; the
backend track will measure the effect on answers.

## 7. Boundaries

- You own `semantic_layer/**`. **Do not touch `api/**`** — the backend track is
  mid-flight there.
- Never invent a column. If the inventory lists it, it exists; if it does not,
  it does not.
- Never widen `sensitivity` to make a question answerable. Withholding PII from
  the enum is a control, not an obstacle.
- Adding a field cannot remove one. Existing `field_id`s are referenced by gold
  plans and tests; rename nothing without saying so.

## 8. Report back

Per family: columns curated, columns deliberately excluded and why, and the
before/after coverage number from §6.1. Name anything in the inventory you could
not curate because its meaning is not recoverable from the data — an honest
"I do not know what `TAC` holds here" is worth more than a guessed description
the generator will then trust.
