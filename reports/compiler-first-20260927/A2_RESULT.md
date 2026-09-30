# A2 COMPILER-FIRST — RESULT

Measured 2026-09-27. Thresholds pre-registered in `THRESHOLD_PREREGISTRATION.md` before the image
was built and before any arm was scored.

**Outcome: MEASURED AND REVERTED. The premise was false.**

Nothing regressed. The change was removed because it bought nothing and cost a duplicate compile.

---

## 1. THE PREMISE, AND WHERE IT CAME FROM

From the post-mortem comment at `query.go` `chooseTemplate`:

> *"The real blocker is therefore NOT the template value: it is that no compiler path exists for a
> template-less question. The handler guard at the `template == ""` check ends the request before
> planning."*

And from `shouldUseLanguageAssistance`, which requires `req.SynthesisModel != ""` — implying a
request without a synthesis model could never reach the compiler.

**Both readings were wrong for this deployment.**

## 2. WHAT THE MEASUREMENT SHOWED

### 2.1 Corpus: zero movement, no regression

    arm A reproduced the shipped posture EXACTLY   67 | 28 | 4 | 2 | 1 | 1
    GOLDEN 0 moved · HOLDOUT 0 moved · MEDIA 0 moved

Switch asserted `FORENSIC_COMPILER_FIRST=true` from `docker inspect` in the treatment arm.

No honesty or provenance probe stopped abstaining, no CORRECT was lost, no new WRONG appeared. The
change was **safe**. It was simply not useful.

### 2.2 The decisive comparison — the compiler already ran

With the switch **OFF**, for *"List the audio files in this case"*:

    template                     ''
    route                        [clarification]
    planner.semantic_operation_planner    operation_latency_ms 3076
                                          registry_candidate_count 66
                                          eligible_operation_count 66
    planner.language_assistance  UNSUPPORTED_REQUEST_CLASS

With the switch **ON**, the same question produced the same audit, the same
`UNSUPPORTED_REQUEST_CLASS`, and ~3.1 s of the same compiler latency.

**A synthesis model IS configured in this deployment**, so `shouldUseLanguageAssistance` passes and
`resolveWithLanguageAssistance` already reaches `applyDeterministicSemanticCompiler`. A2's
invocation was a second, redundant one — a duplicate catalogue build and compile on every
unrecognised question, for no change in outcome.

## 3. THE INSTRUMENT DEFECT THAT PRODUCED THE FALSE PREMISE — the sixteenth

Before building, a trace reported *"semantic_planner audit present: False"* for a template-less
question, and that was read as proof the compiler never ran.

**`SemanticPlannerAudit` is declared `json:"-"` (`query.go:111`) and is never serialised into the
response at all.** The probe could not have seen it in either arm. It measured nothing and was read
as evidence of absence.

The reading that eventually settled the question used fields that ARE serialised:
`planner.semantic_operation_planner` (real latency and candidate counts) and
`planner.language_assistance`.

**This is the same failure mode as the escaped-quote truncation earlier the same day: an instrument
that cannot observe the thing being measured, reporting a confident negative.** Sixteen such
defects are now on record in this project's own tooling.

## 4. WHAT THE REAL BLOCKER IS

The compiler runs, ranks 66 candidates, and then **refuses**, returning
`UNSUPPORTED_REQUEST_CLASS` at its own structured-competence guard
(`deterministic_semantic_compiler.go`):

    if semanticQuestionFamily(req.Query) == "" && extractCanonicalRecordType(req.Query) == "" {
        return req, "UNSUPPORTED_REQUEST_CLASS"
    }

That guard is **correct** and was written for a measured reason: a typed plan over structured rows
answered *"which image says Stay Positive Work Hard"* and *"what time period does this case cover"*
from `canonical_records`, both wrong.

So *"List the audio files in this case"* is not blocked by routing. It is blocked because the
source-native executor has no competence over an audio-family listing, and nothing in the curated
layer lets it acquire one here.

**That is the A3 work, not a routing change.** Specifically it is A3.1 (group/count over a
provenance column), A3.3 (attribute projection — a listing is a projection), and the media-family
equivalents.

## 5. WHAT WAS REMOVED

`api/forensic_records/compiler_first.go` and its test, the handler block in `query.go`, and the
`FORENSIC_COMPILER_FIRST` entry in `docker-compose.forensic-records.yaml`. A comment remains at the
insertion point recording what was tried and pointing here, so the same premise is not re-derived
from the same stale comment.

Full package tests pass after the revert. Shipped posture re-asserted from the container:

    BOOLEAN_RESTRICTION · CONSTRAINT_OBLIGATIONS · DERIVED_ARTIFACT_EXECUTION
    LADDER_ROUTING · MEDIA_FAMILY_ROUTING · RANGE_FILTERS · TEXT_BROWSE_GUARD · VERIFIED_ONLY

all true; no `COMPILER_FIRST` present.

## 6. WHAT THIS COST AND WHAT IT BOUGHT

Cost: one build, two arms, about forty minutes.

Bought: the routing hypothesis is **dead**, with evidence, so no one spends days on it again. The
stale comment that produced it is now annotated. And the next work item is precisely located — one
guard, one reason code, and a named list of the capabilities that would let it pass.

**A negative result that closes off a wrong direction is worth more than a change that measures at
zero and ships anyway.**

## 7. WHAT WAS NOT DONE

No `docker compose down`, `down -v`, `--remove-orphans`, `system prune`, `volume prune`. No database
write, migration, backfill or reprocessing. No auth disabled. No expectation edited. No evidence
uploaded. Rollback image: `nexusai-forensic-records-api:rollback-before-compilerfirst-20260927`.
