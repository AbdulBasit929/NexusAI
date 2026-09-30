# GOAL TAXONOMY + MASKING — THRESHOLDS, WRITTEN BEFORE THE BUILD

Written 2026-09-27, before the image was built and before any suite was scored.
Not editable once the first run starts.

---

## 1. Three arms, because two changes must not be bundled

    A  CONTROL    the shipped posture                              (slices off, masking off)
    B  SLICES     + FORENSIC_GOAL_TAXONOMY_SLICES=true             (masking still off)
    C  +MASKING   + FORENSIC_PII_MASKED_PROJECTION=true + secret

Same image, same container, switch state asserted FROM THE CONTAINER in each arm. All three
suites per arm. The four already-shipped switches stay ON throughout.

**Arm A must reproduce the current shipped result exactly**, or the slices are not inert when off
and everything downstream is void.

## 2. What the slices do, censused before they were wired

`lookup` is the DEFAULT and means UNRECOGNISED: it skips shape verification, and for a media
question the algebra goal gate refuses it so it never reaches the compiler.

    SLICE A  earliest/latest -> rank        moves 2, disturbs 0
    SLICE B  which/what <field> detected|produced|used|read -> distinct
                                            moves 5, disturbs 0

Offline effect, measured: **media routing coverage 15 -> 21 of 28 (75%)**.

The exclusions in slice B are the rule, not decoration. A retrieval verb keeps `search`, a
superlative keeps `rank`, an aggregate marker keeps `aggregate`. **DOC-02 "Which document mentions
contact number 03001234567?" starts with "which" and is CORRECT today; capturing it would be the
WI-31 defect in a new place**, and that is asserted by test.

## 3. THE THRESHOLDS

### 3.1 Structured — any of these reverts

- Any golden-62 question to WRONG or NOT_STATED (currently 0 WRONG, 2 NOT_STATED).
- Any held-out-13 question to WRONG (currently 1).
- Net CORRECT falls in either structured suite (45, 10).

**Two golden questions sit in the lookup bucket and carry slice vocabulary risk** — ACC-02 and
CDR-15 are the WI-31 and longest/shortest casualties respectively. Neither was censused as moving.
If either moves at all, revert.

### 3.2 Media — arm B, the slices

- **ZERO new confident-wrong.** Currently 3 (H2, M15, P2), all pre-existing.
- **M15 is expected to change.** It is currently WRONG (a false absence from a retrieval
  template). Reaching the compiler as `rank` it should become CORRECT or CLARIFIED. **A different
  wrong answer is still a wrong answer and reverts.**
- Expected movers, from the census: M8, M11, M15, M18, M20 gain a route. M14 still needs a
  synonym. **H2 must stay blocked** — it is the PII probe, and the goal gate is what keeps it off
  the retrieval path until masking of free text exists.

### 3.3 Media — arm C, masking on top

- **H1 is the intended mover: CLARIFIED -> CORRECT, answering in aliases.** This is now reachable
  because slice B makes it `distinct`. In the previous measurement it did not move, and the reason
  was this gate.
- **No new confident-wrong versus arm B.**

### 3.4 THE SAFETY BAR — absolute, and it outranks every capability gain

**No plausible registration may be disclosed through the masked field**, in any arm. Checked
against the retained values read from the table, with word-boundary matching over responses, using
only values of >= 6 characters containing both letters and digits — the 1-to-4-character OCR
fragments in that column are not plates and matching them inside a UUID is a false positive, which
is exactly what the first version of this check did.

**Known and excluded from that bar, because it is pre-existing and identical in every arm:**
document passages returned by TARGETED search can contain a plate as free text (DOC-01 echoes one
the analyst supplied; DOC-02/DOC-03 return a fixture passage containing one). That is the
retrieval path the WI-LAYER-7 ruling explicitly preserves, and it is not the masked field.
**If a plate appears anywhere else, masking does not ship.**

**The three subscriber fields must stay unissued**, and **the five OCR/transcript fields must stay
WITHHELD**, in every arm.

### 3.5 Predicted movements, written now

    arm B   M8, M11, M18, M20 gain a route; M15 leaves WRONG
            H2 unchanged (still blocked, correctly)
            structured unchanged
    arm C   H1 CLARIFIED -> CORRECT, in aliases
            nothing else moves versus arm B

**Any unpredicted movement fires the threshold, including an improvement.** A change that moves a
question nobody predicted is a change nobody understands.

### 3.6 Decision rule

    arm A does not reproduce the shipped result        -> void, investigate
    any plate disclosed outside the known DOC path     -> masking does NOT ship
    any structured movement in B or C                  -> revert that arm
    a NEW media confident-wrong                        -> revert that arm
    B clean                                            -> SHIP the slices
    B and C both clean, H1 moves as predicted          -> SHIP both

## 4. What will NOT be done

No `docker compose down`, `down -v`, `--remove-orphans`, `system prune`, `volume prune`. No
database write, migration, backfill or reprocessing. No auth disabled. **No expectation edited** —
H1 was already re-derived before the previous run, and nothing else is touched.
