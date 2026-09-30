# TEMPLATES OFF -- WHAT THE RUNTIME QUERY PATH ANSWERS ALONE

Written 2026-09-29, before the arm ran. **A census, not a ship decision.** Nothing is switched in the
deployed posture because of it.

## Why now

The product owner's direction (2026-09-29) is that questions are answered by queries built at run time,
by the LLM and the compiler, from the database, not by predefined templates. The latest measured run
(reports/template-states-20260929/on) splits the 70 correct answers:

    LLM-written plan (checked, then run on the database)   30
    deterministic compiler plan (built at run time)         25
    keyword templates                                       11   (8 are document/transcript/image text search)
    terminal (definitions, refusals)                         4

## The arm

    ladder-off   FORENSIC_LADDER_ROUTING=false, every other switch as shipped (A4 on), same image as
                 the A4 arms. Compared with the A4 `on` arm (ladder on).
                 Corpus 103, 14 everyday, 38 pre-flight, 16 relational, plate probe.

## What is recorded

- Every question whose verdict differs between ladder-on and ladder-off, and why: which template
  answered it, and what the runtime path did instead (clarified, refused, wrong).
- **Every confident wrong answer the runtime path gives alone.** These are what must be closed before
  templates can be retired (roadmap G1).
- The family-by-family list of answers that only templates give. This is the work order for moving
  them onto the runtime path.

The rule for retiring templates stays: ladder-off at least as correct as ladder-on, with 0 new wrong.
