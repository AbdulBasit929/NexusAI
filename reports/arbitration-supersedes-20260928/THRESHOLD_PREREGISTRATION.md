# ARBITRATION SUPERSEDES THE REGISTERED SELECTION -- THRESHOLDS

Written 2026-09-28, BEFORE the switch was built into an image or any arm was scored.

## The defect

On the ladder-off arm, CDR-12 *"Which cell site handled the most calls?"* ends as
"Which target identifier should I analyze?". The recorded response shows why:

    language_assistance     semantic_ir_fallback
    ir_arbitrated           true
    ir_fallback_outcome     accepted
    dynamic_plan            GROUP BY cdr.cell_site_id, COUNT, sort m1 DESC, limit 1   <- the correct plan
    selected_operation      cdr.tower_activity (required: target)                     <- stale
    binding_state           USER_FACT_REQUIRED

IR arbitration adopted a verified typed plan and cleared `BindingState`, but left
`Selected` pointing at the registered operation the compiler chose first. The
handler then RECOMPUTES binding readiness from that stale selection
(query.go, just before `bindingMissing`), finds no target, and clarifies. The
plan that will actually execute needs no target. With the ladder ON the same
plan is adopted with no stale selection and CDR-12 is CORRECT
("149631808 (1,114)").

This replaces the planned "A3.2 v2" (skip target-requiring operations in
`deterministicRegisteredMatch`). That change would have altered the compiler's
choice for 5 questions including two plate-text PII probes; this one alters
one boolean on one path.

## Census (offline, from recorded responses -- the production pipeline's own output)

Questions where `ir_arbitrated` AND a registered `selected_operation` is present
AND the final binding state is `USER_FACT_REQUIRED`:

    on-a31only (shipped posture)   0 of 103
    ladderoff-baseline             1 of 103   CDR-12
    off-fix                        1 of 103   CDR-12

14 (ladder on) / 22 (ladder off) arbitrated questions carry a stale selection
whose readiness is READY; for them the switch changes only the audit field
`binding_state`, never a verdict. VID-01 and H1 (plate-text PII probes) carry a
stale selection but are withheld BEFORE the binding check (binding_state absent),
so the switch cannot reach them.

## Mechanism

`FORENSIC_ARBITRATION_SUPERSEDES_SELECTION`, code default OFF (requires "true"),
declared in compose default `false` until measured. When on, and only when the
executing request is a typed plan adopted by IR arbitration
(`IRArbitrated && SourceNative != nil`), readiness is not recomputed from the
superseded selection; the audit records `binding_state=SUPERSEDED_BY_ARBITRATED_PLAN`.
The adopted plan has already passed S6, S9 SHAPE and CONSTRAINT_APPLIED inside
`resolveSemanticIRFallback`.

## Arms (switch state asserted from `docker inspect` before each is scored)

    1 on-control    ladder ON,  switch OFF   must equal on-a31only (build inert)
    2 on-fix        ladder ON,  switch ON
    3 off-control   ladder OFF, switch OFF   (A3.1 on, as shipped)
    4 off-fix       ladder OFF, switch ON

## Pass criteria

    on-fix  vs on-control    0 CORRECT lost, 0 new WRONG. Census predicts 0 moved;
                             any movement is explained question by question before shipping.
    off-fix vs off-control   CDR-12 CLARIFIED -> CORRECT. 0 CORRECT lost, 0 new WRONG.
                             Any other movement explained before shipping.
    all arms                 VID-01, H1-MEDIA-WHICH-PLATES, H13-HONESTY-NOPLATE verdicts unchanged.

Fail any line -> switch stays OFF and the result is recorded. Pass all -> compose
default flips to `true` in the same change that records the measurement.
