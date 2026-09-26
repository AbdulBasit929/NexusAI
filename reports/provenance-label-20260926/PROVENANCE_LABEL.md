# STEP 2 — THE PROVENANCE LABEL. Measured 2026-09-26.

**The last safety blocker on the media path.** A derived observation was cited to the analyst as
an ingested source record. Fixed at source, proven against the live database, with a negative
control that reproduces the defect from a real lineage row.

    STATUS   source complete, suite green both switch states, NOT deployed
    SCOPE    api/forensic_records/query.go -- presentation only, no SQL changed
    SWITCH   none, and §"Why no switch" says why

---

## 1. What the analyst was shown

Measured end to end 2026-09-26 — plan → SQL → executor lineage → display rows → enterprise
provenance — for the derived question *"How many plate groups were read from the videos?"* over
`nexusai-multimodal-product-acceptance`, whose `forensics.video-anpr-plate-group/v1` contract
holds **24 completed artifacts**.

The citation the analyst received, in full:

    [ { "source":          "records_sql",
        "record_type":     null,
        "row_hash":        null,
        "row_number":      null,
        "source_file":     null,
        "source_locator":  { "record_id": null, "batch_id": null,
                             "file_id":   null, "row_number": null },
        "evidence_id":     "a3c75ee1-...",
        "version_id":      "b33200e0-...",
        "collection_id":   "nexusai-multimodal-product-acceptance",
        "timestamp":       "2026-08-31T13:59:38.594805+00:00" } ]

**One item. For twenty-four artifacts.** Four defects in one object:

1. **It claims to be an ingested record.** `forensic.derived_artifacts` has no `record_id`,
   `record_type`, `row_hash`, `row_number` or `source_file`. Presenting them as NULL does not read
   as "this table has no such column" — it reads as *this record has no value there*, which is a
   claim about the evidence. The product's central safety property is that a model's plate read is
   never counted as a camera's sighting, and the citation is the one place an analyst goes to
   check it.
2. **It cannot be attributed.** No `artifact_id`, no `artifact_type`. Derived PROJECTION is
   refused precisely because listing observations without provenance hands the analyst evidence
   they cannot attribute; an aggregate's citation was failing the same bar silently.
3. **`source_truth_state` was dropped entirely.** It is the field that separates a derived
   observation from an ingested one, the executor selected it correctly, and nothing downstream
   carried it.
4. **24 artifacts collapsed to 1 citation.** The de-duplication key was
   `evidence_id|row_hash|source_file|source_entry|row_number|citation|source_locator` — every
   component after the first is NULL on a derived row, and one video yields one `evidence_id` for
   every artifact derived from it. So all 24 hashed to the same key. A single unattributable item
   standing in for 24 observations is indistinguishable from a citation for evidence that was
   never examined.

**The executor was already correct.** `sourceNativeLineageAggExpr` (Stage 1) selected
`source: derived_artifacts_sql`, `source_table: forensic.derived_artifacts`, `artifact_id`,
`artifact_type`, `run_id`, `citation_locator` and `source_truth_state` — all of it. The
presentation layer read that row and rebuilt it with the label hardcoded, discarding every
derived field. **This is the §6 finding again: the plumbing was right and unreachable.** It was
invisible to code review because both the SQL and the lineage expression read correctly in
isolation; only running the whole path showed the loss.

---

## 2. The fix

`api/forensic_records/query.go`, three changes, all in presentation:

| Change | What it does |
|---|---|
| `sourceRowIsDerived` | Asks the LINEAGE, never the question, family or route. The SQL that produced the row already stamped it. |
| `derivedProvenanceItem` | Builds the derived citation: `artifact_id`, `artifact_type`, `run_id`, `citation_locator`, `source_truth_state`, `proof_role`. Records columns are **omitted, not nulled**. Absent producer fields (`confidence`, `content_sha256` are genuinely absent on some contracts) are omitted rather than presented as null. |
| de-duplication key | `artifact_id` appended. It is nil on a records row, so records de-duplication is unchanged. |
| `rowIsDerived` guard | The records fallback at the end of the loop keys on a top-level `source_file`/`row_number`/`timestamp`. **No curated media field normalizes to one of those names today** — verified across all ten derived entities — so the fallback could not fire. That is a curation coincidence, not a safety property: one synonym added in `semantic_layer/**` would turn it into a derived observation cited as an ingested record, with no code change and nothing to fail. The flag makes it structural. |
| `answerSourceLabel` | The metrics panel sits beside the citations. A derived answer whose panel read `records_sql` re-asserted the same falsehood in the second place the analyst looks. |

What the analyst is shown now, for the same question:

    24 citations, one per plate group, each:
      "source":             "derived_artifacts_sql"
      "source_table":       "forensic.derived_artifacts"
      "artifact_id":        "0f231311-76e4-5b35-b041-c01c2056c8e2"
      "artifact_type":      "forensics.video-anpr-plate-group/v1"
      "source_truth_state": "derived_technical_observation"
      "proof_role":         "derived_observation"
      "citation_locator":   { bbox, first_seen_seconds, last_seen_seconds, source_file }
      no record_id / record_type / row_hash / row_number / source_file at all

---

## 3. Why no switch

**Standing rule: every behavioural change gets its own switch, default off.** This one does not,
and the reason is specific rather than convenient: **default-off would mean the lying citation
remains the default.** A switch exists so a change can be withdrawn after measurement; there is
no state of the world in which citing a model observation as an ingested source record is the
option we want to be able to fall back to.

What bounds it instead is **inertness on structured evidence, asserted rather than argued**:

- The derived branch is reachable only when a source row's own lineage says
  `derived_artifacts_sql`, which only `sourceNativeLineageAggExpr` emits, which only runs for a
  derived binding, which is refused entirely unless `FORENSIC_DERIVED_ARTIFACT_EXECUTION=true`.
- `TestRecordsProvenanceIsUnchangedByTheDerivedBranch` asserts a records source row produces
  byte-identical provenance — label, every records column, and the records-shaped
  `source_locator` — to what it produced before.
- Both full-suite runs below are green in **both** switch states.

---

## 4. Evidence — reproduce with these

    go build ./api/forensic_records/                                    BUILD OK

    go test -C api/forensic_records -count=1 .                          ok  60.0s
    FORENSIC_MEDIA_FAMILY_ROUTING=true FORENSIC_DERIVED_ARTIFACT_EXECUTION=true \
      go test -C api/forensic_records -count=1 .                        ok  70.7s

    DERIVED_EXEC_DB="postgresql://localrecall:localrecall@localhost:5433/localrecall" \
      go test -C api/forensic_records -count=1 -v \
      -run "TestDerivedProvenance|TestRecordsProvenance|TestSourceRowIsDerived|TestAnswerSourceLabel" .

    PASS TestDerivedProvenanceNeverWearsTheRecordsShape
    PASS TestDerivedProvenanceCarriesTheArtifactIdentity
    PASS TestDerivedProvenanceSurfacesSourceTruthState
    PASS TestDerivedProvenanceDoesNotCollapseDistinctArtifacts
         24 distinct artifacts cited, one per plate group            (was 1)
    PASS TestRecordsProvenanceIsUnchangedByTheDerivedBranch
    PASS TestAnswerSourceLabelFollowsTheCitations
    PASS TestDerivedProvenanceAssertionsRejectTheOldShape
         old shape: source=records_sql record_type=<nil> row_hash=<nil>
                    source_file=<nil> artifact_id=<nil>
    PASS TestSourceRowIsDerivedReadsTheLineage

Regression proofs unchanged after the fix: `TestDerived*` and `TestMeasureDenominator*` green
(24 plate groups, nested AVG 0.266055, 20+6=26 partition, denominator 263/309);
`TestMediaFamilyRoutingCensus` unchanged at 7 of 28; `TestWI4` green.

### The negative control — why these tests are not vacuous

Three of the assertions above are about SHAPE rather than a value, which is exactly the kind that
can pass whether or not anything was fixed. `TestDerivedProvenanceAssertionsRejectTheOldShape`
reads a **real derived lineage row out of the database**, rebuilds the citation exactly as the
code built it before this change, and asserts that each of the four properties rejects it: the
label is not derived, all four records columns are present-and-null, the artifact identity is
absent, and `source_truth_state` is absent. It then asserts the lineage row it started from DID
carry `artifact_id` and `source_truth_state` — which is what makes this a presentation defect
rather than a missing capability.

An attempt was made to prove the instrument the other obvious way, by adding a temporary
env-gated bypass to `sourceRowIsDerived` and watching the tests fail. **That was the wrong
method** — it puts a safety-bypass flag into production code to serve a test — and it was
correctly refused. The negative control lives entirely in the test file.

---

## 5. What this does NOT do

- **Not deployed.** Source only. No container was rebuilt, no `docker compose` action beyond
  `ps`, no database write, no evaluation suite run.
- **Does not change any number.** No SQL, no plan, no measure, no verdict. It changes what the
  citation beside the number says it came from.
- **Does not enable derived-row PROJECTION.** Still refused: `derived_artifacts` has no
  `record_id`/`row_hash`, and §9 E's derived provenance mapping is unbuilt. This fixes the
  citation for AGGREGATES, which is what the media path currently answers.
- **Does not change routing.** Census is still 7 of 28. That is Step 3, and it is curation.

---

## 6. Configuration state at the time of measurement

Asserted from the container, per the protocol, **before** anything was measured:

    FORENSIC_VERIFIED_ONLY=true            FORENSIC_IR_ARBITRATION=true
    FORENSIC_IR_FALLBACK=true              FORENSIC_PLAN_CACHE=true
    FORENSIC_ANSWER_STATES_VALUES=true     FORENSIC_MEASURE_DENOMINATOR=true
    FORENSIC_LADDER_ROUTING=true           FORENSIC_BREAKDOWN_GOAL=false
    FORENSIC_IR_SHADOW=false               FORENSIC_IR_CROSSCHECK=false
    FORENSIC_DROP_INVENTED_FILTERS=false
    FORENSIC_MEDIA_FAMILY_ROUTING=true     <-- ON
    FORENSIC_DERIVED_ARTIFACT_EXECUTION=true  <-- ON

**FINDING, and it bears on Step 4.** The running container already has both media switches ON.
The handoff records them as default-false and Step 4 as the measurement that turns them on with a
control. **The control must therefore be taken deliberately with both switches OFF — the current
container is already the treatment arm, not the control.** Scoring a run now and calling it a
baseline would repeat the config-drift error of 2026-09-25 in the opposite direction: the switch
list in a document said one thing and the container said another, and the container is the
authority.

---

## 7. SECOND FINDING, measured while writing the Step 3 brief: the routing ceiling is 15, not 28

**Step 3 was framed as pure curation. It is not, and the census said so only because the census was
guessing.**

Its output labelled every unclaimed question *"needs a curated synonym, or is a structured/honesty
probe"* — never checked, and the sentence that sent the next cycle of work entirely to the Codex
track. Forecasting the proposed synonyms through the REAL matcher against the REAL 28 questions,
before sending the brief:

    with every proposed synonym added:   15 claimed · 1 ambiguous · 12 unclaimed    (from 7)
    structured suites:                    0 claimed — no regression on the 62 or the held-out 13

| Blocker | Questions | Owner |
|---|---|---|
| **VOCABULARY** — a synonym is sufficient | M2 M3 M4 M7 M13 M19 M21 H4 | Codex, YAML |
| **AMBIGUOUS** — two families match, clarifies by design | M6 | nobody; correct outcome |
| **GOAL GATE** — a phrase matches, `goal=lookup` refuses it | M8 M11 M14 M15 M18 M20 P2 | **backend, goal taxonomy** |
| **GOAL GATE, correctly** — honesty probes that must clarify | H1 H2 H3 H5 | nobody; correct outcome |
| **Must never be claimed** | P1 | nobody; must stay structured |

**Seven questions match a curated phrase and are then refused because their goal classifies as
`lookup`** — which is also the *unrecognised* default, so it cannot be admitted to the algebraic set
without handing every unparsed question to the compiler. Six of the seven are plainly `distinct` or
`rank`/`range` in English:

    M8  "the earliest offset at which a plate group was first seen"   -> range / rank
    M11 "What script families were detected in the image text?"       -> distinct
    M14 "What languages were detected in the audio?"                  -> distinct
    M15 "the latest end offset in the audio transcripts"              -> rank / range
    M18 "Which model produced the face vectors?"                      -> distinct
    M20 "Which perceptual hash algorithm was used for the images?"    -> distinct

This is **§9 F of the handoff** — the goal taxonomy re-measure — arriving with a concrete payload it
did not have before: it is worth up to seven media questions, and four of them are the same
`distinct` misclassification. It does not change the ordering (the ladder still answers the
breakdown targets first), but it removes the uncertainty about what the taxonomy work buys.

### Instrument fixed

`mediaRoutingBlocker` now names the gate that actually refused each question, and distinguishes
**VOCABULARY** (a synonym is enough) from **VOCABULARY+GOAL** (a synonym is necessary but not
sufficient, and will not move the number). `TestMediaFamilyRoutingCensus` prints it, and the header
says which track owns which. Coverage is unchanged at 7 of 28 — this changes only what the census
can tell you about the 21.

**The Step 3 brief was rewritten on these numbers** before being sent:
[`WI-LAYER-6_CODEX_PROMPT.md`](../../docs/work/WI-LAYER-6_CODEX_PROMPT.md). It tells Codex to judge
its own work by the VOCABULARY lines only, expects exactly 15, and rules M6's ambiguity as the
correct outcome rather than something to work around.

---

## 8. STEP 3 VERIFIED INDEPENDENTLY, and one safety finding it produced

Codex landed WI-LAYER-6 (ten entity synonyms, `semantic_layer/**` only). **Verified with my own
build rather than read from the report** — the standing rule exists because Codex's `api/**`
reports have been stale three times, and it applies to a GREEN report as much as a red one:

    go build ./api/forensic_records/                                   BUILD OK
    census                                                             15 of 28 (54%), from 7
    newly claimed   M2 M3 M4 M7 M13 M19 M21 + H4          none previously claimed was lost
    M6              unclaimed, AMBIGUOUS                  as ruled — correct, not a workaround
    P1 H1 H2 H3 H5  unclaimed                             the honesty probes hold
    TestMediaFamilyRoutingNeverClaimsAStructuredQuestion               PASS
    TestMediaFamilyRoutingNeverClaimsAnAcceptedVariantRouting          PASS
    TestMediaFamilyRoutingKeepsSightingsStructured                     PASS
    TestWI4 · TestSemanticLayerCoverage 93/94 · full suite both states PASS
    all 17 YAML sha256 vs the pre-build manifest                       IDENTICAL

**The forecast predicted this exactly.** 15, the same eight questions, M6 ambiguous, H4 claimed.
That is the argument for forecasting a brief through the real mechanism before sending it: the
first draft of the brief would have sent a whole cycle to the wrong track and then read as a
failure when the number stopped at 15.

### THE SAFETY FINDING: H1 and H2 were being held by the wrong gate

With routing widened, the census shows **H1 "Which plates were read from the videos?" and H2 "What
does the audio transcript say?" now MATCH a curated media phrase** and are refused only because
`goal=lookup` is not algebraic.

**The goal gate is not a PII control.** It exists to keep `lookup` — which is also the
*unrecognised* default — away from the compiler. The very next backend item, the goal taxonomy,
exists specifically to reclassify these `lookup` questions as `distinct`. The day M11/M14/M18/M20
start routing, H1 and H2 route with them.

The actual control is `CatalogFields`, which drops PII and RESTRICTED outright so no plan can name
the field and the GBNF alternation makes it unemittable. **That was never asserted for the media
families.** It is now, in `media_pii_boundary_test.go`, written BEFORE the taxonomy work rather
than after:

    anpr_model_observation        6 issued, 1 PII withheld   plate_text
    video_anpr_plate_group        7 issued, 1 PII withheld   plate_text
    image_ocr_observation         9 issued, 2 PII withheld   raw_text, normalized_text
    audio_timestamp_segment       7 issued, 1 PII withheld   text
    audio_roman_urdu_segment      9 issued, 2 PII withheld   raw_urdu_text, roman_urdu_text

    plate_text withheld at EVERY goal — lookup, distinct, aggregate, rank, range, breakdown

Three properties are asserted: the PII list is derived from the layer (so a newly curated PII field
is covered the day it lands, not the day someone remembers); each probe field must be **declared**
PII as well as absent from the catalogue, so the test cannot pass because nobody curated it; and
withholding is proven **independent of the goal**. **If that test fails, the taxonomy work has
opened a PII path and must stop.**

This is the generalisable lesson, and it is now in the continuation's lessons list: **when a gate
widens, ask what else that gate was holding.** Routing was widened for coverage; what it exposed
was that a PII boundary had been resting on a routing heuristic.
