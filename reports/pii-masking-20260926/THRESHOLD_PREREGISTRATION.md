# PII MASKING — THRESHOLDS, WRITTEN BEFORE THE BUILD

Written 2026-09-26, before the image carrying `FORENSIC_PII_MASKED_PROJECTION` was built and
before any suite was scored with it on. Nothing here may be edited after the first run starts.

---

## 1. Arms

    A  CONTROL     PII_MASKED_PROJECTION=false                  (the shipped posture)
    B  MASKING     PII_MASKED_PROJECTION=true + ALIAS_SECRET    (the change)

Both on the SAME image and container, with the four already-shipped switches ON in each
(`MEDIA_FAMILY_ROUTING`, `DERIVED_ARTIFACT_EXECUTION`, `BOOLEAN_RESTRICTION`,
`TEXT_BROWSE_GUARD`), and the switch state asserted FROM THE CONTAINER before either is scored.

All three suites in both arms: golden 62, held-out 13, media 28, `--analyst-text`.

**Arm A must reproduce the current shipped result exactly.** If it does not, masking is not inert
when off and everything downstream is void.

## 2. ONE expectation was re-derived, before the run, and here is the whole reasoning

`H1-MEDIA-WHICH-PLATES` moves from `check: clarify` to `check: contains ["Plate candidate"]`.

This is sanctioned by the question's own prior note, which read: *"This becomes answerable only
when masked projection ships, and then this expectation must be re-derived."* The suite anticipated
exactly this change.

**The product owner ruled, before the measurement:** with masking on, H1 answering in aliases is the
PASS. That is the capability the WI-LAYER-7 ruling was written to enable — the analyst learns there
are 24 distinct plate candidates and can correlate one vehicle across stills and video, without
ever seeing a registration.

DB truth, re-derived 2026-09-26 from the same expression the executor uses: **24 rows, 24 distinct
`normalized_plate_text`** over `forensics.video-anpr-plate-group/v1`.

**This is the only expectation touched, it was touched BEFORE the run, and it was touched because
the product rule changed — not because a result was disappointing.**

## 3. THE THRESHOLDS

### 3.1 Structured — ANY of these reverts

- Any golden-62 question moves to WRONG or NOT_STATED. Currently 0 WRONG, 2 NOT_STATED.
- Any held-out-13 question moves to WRONG. Currently 1.
- Net CORRECT falls in either structured suite (45, 10).

Masked fields belong to two derived media families. **Zero structured questions can reach them, so
zero structured movement is expected and any movement at all is a defect.**

### 3.2 THE SAFETY BAR — absolute, and it outranks every capability gain

**No raw plate value may appear anywhere in any response, in either arm.** Not in the analyst text,
not in a group label, not in a citation, not in a metadata echo, not in the field catalogue.

Checked against the actual retained values read from the table, over the whole serialised payload —
not only the fields a test knew to look at. If this fires, masking does not ship at any coverage.

**Masked fields must not be sortable.** SQL orders by the raw value while the analyst sees aliases,
so an ordered list of aliases discloses raw lexical order. A sortable masked field is a leak even
when every value shown is an alias.

**The three subscriber fields must stay unissued.** `subscriber.cnic`, `subscriber.full_name` and
`subscriber.cnic_last4` are `PII`/`MASKED` in curation but have NO implemented scheme. Admitting
every MASKED field was the bug this change already made once; if one reappears, revert.

**The five OCR/transcript fields must stay WITHHELD**, masking on or off.

### 3.3 Media — the arm under test

- **ZERO confident-wrong.** Currently 3 (H2, M15, P2), all pre-existing and all wrong with the
  switches off. **No NEW confident-wrong is permitted.**
- **H1 is the intended mover.** Expected: CLARIFIED in arm A, CORRECT in arm B, with the answer
  naming aliases.
- **P1 must still answer 1,057 and never 307.** Masking has no path to it; if it moves, something
  unrelated broke.
- **H2, H3, H5 and M6 must still clarify.**

### 3.4 Predicted movements, written now

    H1   CLARIFIED -> CORRECT     the ruled capability
    every other question          UNCHANGED, all three suites

**Any unpredicted movement fires the threshold**, including an improvement. A change that moves a
question nobody predicted is a change nobody understands.

### 3.5 The decision rule

    Arm A does not reproduce the shipped result        -> void, investigate
    any raw plate anywhere                             -> masking does NOT ship, at any coverage
    a subscriber field issued                          -> revert
    a NEW media confident-wrong                        -> revert
    any structured movement                            -> revert
    H1 moves as predicted and nothing else moves       -> SHIP arm B

## 4. What will NOT be done

No `docker compose down`, `down -v`, `--remove-orphans`, `system prune`, `volume prune`. No database
write, migration, backfill or reprocessing. No auth disabled. **No further expectation edited** —
H1 is the one, ruled and recorded above, before the run. The alias secret is 256 bits from a CSPRNG,
lives only in the gitignored env file, and has never been printed to a transcript.
