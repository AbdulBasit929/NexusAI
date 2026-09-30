# ARBITRATION SUPERSEDES THE REGISTERED SELECTION -- RESULT

Measured 2026-09-28. Thresholds in `THRESHOLD_PREREGISTRATION.md`, written before the image was
built. Scored by `scripts/nexusai_supersede_compare.py`.

    FORENSIC_ARBITRATION_SUPERSEDES_SELECTION   SHIPPED, default ON

## The defect, and why it replaced "A3.2 v2"

With the ladder OFF, CDR-12 *"Which cell site handled the most calls?"* had the correct plan in hand
(GROUP BY cell site, COUNT, DESC, limit 1, adopted by IR arbitration) and still asked *"Which target
identifier should I analyze?"*. The audit kept the registered operation the compiler had chosen
first (`cdr.tower_activity`, which requires a target), and the handler recomputed binding readiness
from it just before deciding whether to clarify. So the operation that needed a target was never
going to run.

The planned A3.2 v2 (skip target-requiring operations in `deterministicRegisteredMatch`) would have
changed the compiler's choice for 5 questions, including two plate-text PII probes. This change
alters one readiness check, on one path.

## Results

    shipped (on-a31only)  CORRECT 67 | CLARIFIED 28 | WRONG 4 | NOT_STATED 2 | MANUAL 1 | ROWCOUNT_ONLY 1
    on-control            identical -- switch-off build is inert
    on-fix                identical -- 0 of 103 moved
    off-control           CORRECT 65 | CLARIFIED 28 | WRONG 6 | ...
    off-fix               CORRECT 66 | CLARIFIED 27 | WRONG 6 | ...
        CDR-12  CLARIFIED -> CORRECT   "The cell site with the highest count is 149631808 (1,114)."
                audit: binding_state=SUPERSEDED_BY_ARBITRATED_PLAN, selected=cdr.tower_activity

    probes  VID-01, H1-MEDIA-WHICH-PLATES, H13-HONESTY-NOPLATE: unchanged in all five arms

Each arm's switch state was asserted from `docker inspect` and saved as `switches.txt`. The runner
aborts if the container does not show the intended value.

**No threshold fired.** The census predicted every movement: 0 in the shipped posture and exactly
CDR-12 with the ladder off. In the ladder-on arms, 12 arbitrated requests changed only their audit
field (`binding_state` READY -> SUPERSEDED_BY_ARBITRATED_PLAN), never a verdict. The two PII probes
carrying a stale selection are withheld before the binding check, so the switch cannot reach them.

## Where the template-free path stands

    ladder ON  (shipped)   CORRECT 67 | WRONG 4
    ladder OFF (today)     CORRECT 66 | WRONG 6      (2026-09-27 baseline: 64 | 6)

Remaining ladder-off gaps: DOC-02 (passage retrieval), CASE-01 and X-01 (WRONG on the compiler
path).

Rollback image: `nexusai-forensic-records-api:rollback-before-arbsupersede-20260928`.
