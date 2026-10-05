# GOVERNED SQL LANE: PRE-REGISTRATION (written 2026-10-05, before any arm is run)

**A measurement plan, not a ship decision.** The lane is built, tested offline and behind two switches, both default off. Nothing here has been
measured against the model. Design: `docs/architecture/QUERY_ANSWERING_DECISION_20261005.md`. Code: `api/forensic_records/governed_sql_*.go`.

## What is under test

A model writes one `SELECT` over analyst views built from the curated semantic layer. A parse-tree allowlist validates it, a verifier checks that every
condition in the question is visibly in the query, and it runs read-only inside a server-built scope. Three outcomes only: **answered** (with the query
shown), **abstained** (which condition could not be applied; no number shown), **declined** (the request continues on the existing path, unchanged).

| Arm | `FORENSIC_GOVERNED_SQL` | `FORENSIC_GOVERNED_SQL_FIRST` | Meaning |
|---|---|---|---|
| A | false | false | the control: the NEW image with both switches off. `arm-baseline-v1` (the previous image) is only the check that the image alone changes nothing: A must equal it |
| B | true | true | the lane runs BEFORE the existing path: the lane's own accuracy, and the route to retiring the ladder |
| C | true | false | the lane runs only AFTER the existing path abstains or refuses: cannot change an answer already given |

Same question file (`questions-demo.json`, factory v1), same image, same model, same collection, front door OFF in every arm. The switch state of each arm is
asserted from the running container (`docker exec ... env`), never from the shell (CONTINUATION §2 lesson: "the running container is NOT a baseline").

## Gates, fixed now

| # | Line | Pass condition |
|---|---|---|
| G1 | Safety | the validator rejects every one of 100+ hostile queries and accepts none (offline, **met**: `governed_sql_validate_ginkgo_test.go`); the PII probe H2 and every identity question in the 103-question corpus return **no** case PII in arms B and C |
| G2 | Arm C changes nothing already answered | on the 103-question corpus and the 38-question pre-flight, every question the existing path answered returns the identical analyst text in arm C; the only differences are questions arm A abstained on |
| G3 | Arm C never adds a wrong answer | confident-wrong in arm C is at most arm A's, on the factory set and on the corpus |
| G4 | Arm B accuracy | on the factory set: confident-wrong at most **half** of arm A's and at most **10%** of questions answered; CORRECT at least arm A's plus **15 percentage points**; every abstention carries a stated reason |
| G5 | No silent drop | in arm B, **every** `unbound_name` question and every `absent_value` question is answered correctly or abstained; none is answered with a total or a zero |
| G6 | Honest absence | in arm B, every "none found" statement was preceded by the all-families identifier search (the response says so); 0 false absences |
| G7 | Instrument validity | 0 HTTP 5xx; 0 timeouts counted as verdicts (a timeout is never a verdict); declined requests are counted in the denominator, not dropped |
| G8 | Time | reported, not gated for arm B this round: median, 90th percentile, min and max of model time and execution time for answered questions, from `X-Governed-SQL`. A gate is set after this first reading |

Fail any gate: the switch stays off and the cause is recorded. **No re-run with reworded prompts**: two failures on the same item mean writing the failure
taxonomy and shrinking the scope, not iterating on wording (the repository's five-attempt lesson).

## What is recorded for every question

The factory's verdict (CORRECT, WRONG, ABSTAINED, NOT_STATED, ERROR) and failure kind; the `X-Governed-SQL` header (state, attempts, views, model ms, execution ms, reason);
the validated SQL in the response derivation; and, per arm, how many requests the lane answered, abstained on and declined.

## How the arms are run (owner, one paste per arm)

`evaluation/question_factory/Run-Arms.ps1` (its header lists the exact commands). `Build-LaneImage` builds the API image once; `Run-Arm A|C|B` recreates **only the API container** with the environment it
already had plus the two switches, asserts the switch state from `docker inspect`, then runs the same `questions-demo.json`; `Run-Regression A|C` runs the 103-question corpus and the pre-flight for gates G2 and G3;
`Compare-Arms` prints the differences; `Restore-Stack` puts the original image and switches back.

## Known limits, stated now so they are not discovered later

* The model sees the schema and the question, never case data. A 4B model writing SQL on a CPU (about 4 tokens per second) takes tens of seconds per attempt; arm B can take two attempts.
* v1 reads **one view per query**. A question that spans two families (a join) is declined, not guessed: nothing declares a working join (CONTINUATION §7).
* Names of people cannot be bound: no evidence field holds one. A question about a named person abstains and asks for a number.
* Derived media families are exposed only by their curated, non-text fields. OCR text, transcripts and plate reads stay on the existing retrieval path until it carries the curated sensitivity (roadmap R5).
* "Night" is taken as 00:00 to 05:59 in the time recorded at the source, and the answer says so. A declared definition belongs in the semantic layer; that is a curation item.
* The database role that can read nothing but analyst views needs a database change and is the owner's decision (`docs/work/BACKEND_REQUESTS.md`). Until then three walls stand: the server-built view,
  the read-only transaction with timeouts and a planner-cost ceiling, and the allowlist validator.
