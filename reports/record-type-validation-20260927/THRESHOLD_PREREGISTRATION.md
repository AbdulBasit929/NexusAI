# RECORD-TYPE VALIDATION — THRESHOLDS, WRITTEN BEFORE THE RUN

Written 2026-09-27, before the image was built and before any arm was scored.

## The defect

Measured against the running service, collection `nexusai-forensic-demo`, question
*"How many cell towers are in this case?"*:

    record_type=tower            "There are no tower records in this case."     <- FALSE; there are 5
    record_type=tower_location   "There are 5 tower records in this case."

The analyst workspace's Tower scope sent `tower`. The service normalised the field and used it as a
scope filter with no validation. **A confident false negative.** Found while verifying Codex's UI in
a browser, not by any suite: the eval harness never sends `record_type`.

## The change

`FORENSIC_RECORD_TYPE_VALIDATION`, **default ON** (the off-state is the unsafe one). A known family
passes; a documented alias resolves through the same mapping questions already use; anything else
is refused with 400 and the statement that nothing was counted. `email` stays known — that is how an
absent family correctly scopes to zero.

The client was also corrected (`ScopeChips.jsx`: `tower` → `tower_location`). This change makes the
service safe against the whole class, from any client.

## Arms

    A  control   FORENSIC_RECORD_TYPE_VALIDATION=false
    B  guard     FORENSIC_RECORD_TYPE_VALIDATION=true

## Thresholds

- **Arm A must reproduce the shipped posture** (67 / 28 / 4 / 2 / 1 / 1).
- **Zero corpus movement in arm B.** The eval harness sends no `record_type`, and an empty value is
  returned unchanged. This is argued by construction — and an argument by construction failed on
  A3.3 today, which is why it is measured anyway.
- **Direct proof, arm B, against the running service:**

        record_type=tower            -> 5 tower records   (alias canonicalised)
        record_type=tower_location   -> 5 tower records   (unchanged)
        record_type=xyz              -> HTTP 400, "nothing was counted"
        record_type=email            -> zero email records, NOT a 400

**Any corpus movement, or any of the four direct checks failing, reverts.**
