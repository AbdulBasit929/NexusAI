# A2 COMPILER-FIRST — THRESHOLDS, WRITTEN BEFORE THE RUN

Written 2026-09-27, before the image was built and before any arm was scored.
**Not editable once the first arm starts.**

---

## 1. WHAT THIS CHANGE IS

A question the keyword ladder does not recognise reaches the `template == ""` clarification in
`query.go` **having never been planned**. A2 runs the DETERMINISTIC compiler at that point and
adopts its result only when the compiler itself resolved.

**Measured before building, not assumed.** *"List the audio files in this case"* returns
`template=""`, `route=[clarification]` and carries **no semantic planner audit at all**. The
existing entry, `shouldUseLanguageAssistance`, requires `req.SynthesisModel != ""`, so a request
without a synthesis model cannot reach the compiler however well formed the question is.

**This is the first change in this series where movement is EXPECTED.** A1a and A1b predicted zero
corpus movement. A2 is a capability change: questions that clarify today may answer. The thresholds
below therefore distinguish a WIN from a REGRESSION rather than treating all movement as failure.

## 2. THE PRIOR ATTEMPT, AND WHY THIS ONE IS SHAPED DIFFERENTLY

Returning `canonical_records` from `chooseTemplate` for media questions looked equally
well-evidenced and **turned five honest abstentions into five confident-wrong answers** — including
*"which plates were read from the videos"* answered as *"20 ANPR sightings"*. The post-mortem is in
the comment at `query.go:2087`.

That change handed out a template and assumed the compiler ran inside it. **A2 runs the compiler
and adopts a result only if the compiler produced one**, already carrying its own S6, S9 SHAPE and
CONSTRAINT_APPLIED checks. If the compiler abstains, the request falls through to today's
clarification unchanged.

## 3. THE ARMS

    A  control   FORENSIC_COMPILER_FIRST=false
    B  compiler  FORENSIC_COMPILER_FIRST=true

Same image, same container, switch asserted from `docker inspect` before each arm is scored.

## 4. THE THRESHOLDS

### 4.1 Arm A must reproduce the shipped posture exactly

    CORRECT 67 | CLARIFIED 28 | WRONG 4 | NOT_STATED 2 | MANUAL 1 | ROWCOUNT_ONLY 1

Or the run is void.

### 4.2 REVERT THE WHOLE ARM if any of these hold in arm B

- **Any question that is CORRECT in arm A is not CORRECT in arm B.** A capability change may not
  cost a working answer.
- **Any question becomes WRONG.** A different wrong answer is still a wrong answer.
- **ANY HONESTY OR PROVENANCE PROBE STOPS ABSTAINING.** These must remain CLARIFIED:

        golden    NEG-01  NEG-02  NEG-03  NEG-04
        holdout   every H-prefixed question
        media     every H-prefixed and P-prefixed question

  For these, answering IS the failure. The media set says so in its own scoring note: *"Never 'fix'
  an H- or P-question by making it answer."* This clause is the one the prior attempt violated.
- **Latency p95 regresses beyond the P3 gate (15 s on the warm path).** A2 adds a catalogue build
  to a path that previously did no work at all.

### 4.3 THE WIN CONDITION

    a non-probe question moves CLARIFIED -> CORRECT

Any number of these is a pass. **Zero is not a failure** — it would mean the compiler abstains on
everything the ladder abstains on, which is itself a finding worth having, and A2 would then ship
OFF pending the A3 gaps that make those questions compilable.

### 4.4 Every movement must be explainable

An unpredicted movement, **including an improvement**, must be named and explained in the result
before anything ships. A change that moves a question nobody predicted is a change nobody
understands.

## 5. DECISION RULE

    arm A does not reproduce the shipped posture    -> void, investigate
    any CORRECT lost                                -> REVERT the whole arm
    any new WRONG                                   -> REVERT the whole arm
    any probe stops abstaining                      -> REVERT IMMEDIATELY; this is the
                                                       prior attempt's exact failure
    latency p95 > 15 s warm                         -> REVERT
    clean, and >=1 non-probe CLARIFIED -> CORRECT   -> SHIP, default ON
    clean, and 0 moved                              -> ship OFF; record why, proceed to A3

### 5.1 Default state if it passes

**Default ON if it moves questions and nothing regresses.** Unlike A1b this is not a narration
concern — a question either compiles or it clarifies, and both outcomes are already honest.

## 6. WHAT WILL NOT BE DONE

No `docker compose down`, `down -v`, `--remove-orphans`, `system prune`, `volume prune`. No database
write, migration, backfill or reprocessing. No auth disabled. **No expectation edited.** No evidence
uploaded. Both arms use the same image and the same container.
