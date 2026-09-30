# A1c RELATIONAL CONDITIONS -- THRESHOLDS

Written 2026-09-28, BEFORE the switch was built into an image or any arm was scored.

## The defect

Measured 2026-09-27 by the ad-hoc probe (`reports/adhoc-runtime-20260927/`), in the shipped posture:

    "Do any subscribers share the same handset?"
        -> "11 subscriber records matched this question."          ANSWERED_UNCOMPUTED
    "Is there any link between the plate sightings and the call records?"
        -> "There are 750 ANPR sightings in this case."              ANSWERED

Both questions ask for a relationship between entities, and both were answered with a count of one
kind of record. A1a's guard does not reach them: its trigger is a comparison against a number, and
its report named this as a separate item.

## Census

`TestRelationalConditionCorpusCensus`: **0 of the 103 corpus questions state a relationship.** The
corpus therefore cannot show this guard's benefit. It can only show that the guard is inert on it, and
it cannot fire there by construction, because the trigger is on the question's own text.
`relational_probe.py` supplies what the corpus lacks:

    R1-R8  relational questions (share the same / sharing one / in common / link, overlap, connection between)
    N1-N8  near-misses that share a word but state no relationship
           ("share of SMS", "Can you share a summary", "Share a list", "linked to <number>",
            plus four ordinary corpus-shaped questions)

## Mechanism

`FORENSIC_RELATIONAL_CONDITIONS`, code default OFF (requires "true"), declared in compose default
`false` until measured. Wired in `preExecutionWithhold` after A1a, before the general verified-only
withhold. It is satisfied by:

- a relational registered operation (`cross_family_correlation`, `relationship_network`,
  `tower_cdr_join`, `anpr_co_travel`, `multi_cdr_comparison`, `subscriber_device_links`), or
- for a "share" relationship only, a typed plan with GROUP BY and a HAVING lower bound on a
  COUNT / COUNT_DISTINCT.

Otherwise it withholds, with the reason code `relationship_not_computed`, and names the relationship
back to the analyst.

## Arms

    probe-control   ladder ON, switch OFF   (the shipped posture)
    probe-fix       ladder ON, switch ON
    corpus on-fix   ladder ON, switch ON  vs the same build's on-control

## Pass criteria

    R1, R2          probe-control reproduces the recorded confident-wrong answers (else VOID: the
                    defect is not what this guard was built for)
    R*              probe-fix: none ANSWERED from a plan that computes no relationship. Every R
                    question is either withheld with `relationship_not_computed`, or answered by a
                    plan that satisfies the rule above.
    N*              probe-fix identical to probe-control in state and text, all eight. ANY change
                    fails.
    corpus          on-fix vs on-control: 0 moved.

Fail any line -> the switch stays OFF and the result is recorded. Pass all -> compose default flips
to `true` in the same change that records the measurement.

This guard only withholds; it computes nothing. Computing "share the same handset" (GROUP BY
handset HAVING COUNT_DISTINCT subscriber > 1) is the follow-on item, and it gets its own
measurement, exactly as A1a (guard) preceded A1b (range filters).
