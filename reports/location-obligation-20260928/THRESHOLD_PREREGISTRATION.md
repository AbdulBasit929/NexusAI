# A1d LOCATION OBLIGATION -- THRESHOLDS

Written 2026-09-28, BEFORE the switch was built into an image or any arm was scored.

## The defect

H11 *"Where was 923001110001 seen according to the call records?"* is WRONG in every recorded arm:
"There are 447 CDR records involving 923001110001 in this case." The IR plan that answered it:

    filters   cdr.originating_number EQ 923001110001 ; cdr.location IS_NOT_NULL
    measures  COUNT
    group     (none)      project (none)

The plan knew the question was about location (it requires one to be present) and counted anyway.
S9 SHAPE did not refuse it because every S9 shape rule keys on the frame goal, and "where" has none:
the frame reads H11 as a lookup.

## Census (recorded responses, both paths)

Corpus questions that OPEN with "where", and whether the executed plan was a bare aggregate:

    question   ladder ON (on-control)            ladder OFF (ladderoff-baseline)
    TWR-02     CLARIFIED, no aggregate           CLARIFIED, no aggregate
    NEG-02     CORRECT, GROUP BY anpr.location   CORRECT, GROUP BY anpr.location
    X-01       CLARIFIED, top_locations          WRONG, entity_timeline (not a typed plan)
    H11        WRONG, bare COUNT                 WRONG, bare COUNT          <- the only match

## Mechanism

`FORENSIC_LOCATION_OBLIGATION`, code default OFF, compose default `false` until measured. It is a
rule in `verifySourceNativePlanShape`, so it applies to the deterministic compiler and to the IR
fallback/arbitration alike. A question whose first word is "where", compiled to measures with no
grouping and no projection, is an `S9_SHAPE_VIOLATION`. It refuses and computes nothing.

## Pass criteria (ladder ON; measured with the relational-conditions switch in its own arms)

    on-fix vs on-control   H11 WRONG -> not WRONG (withheld or clarified). No other question moves.
                           0 CORRECT lost. NEG-02 stays CORRECT.

Fail any line -> the switch stays OFF and the result is recorded. Pass all -> compose default flips
to `true` in the same change that records the measurement.
