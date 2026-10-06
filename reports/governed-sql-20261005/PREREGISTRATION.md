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

## Run 1 (2026-10-06): arms A and C, recorded as measured

Image built from commit `da818a7`. Factory set `questions-demo.json`, 82 questions, front door off.

| Arm | Correct | Confident-wrong | Abstained |
|---|---|---|---|
| baseline v1 (previous image) | 35 | 22 | 25 |
| A (new image, switches off) | 35 | 22 | 25 (identical to baseline question by question except one answer's wording) |
| C (lane after the existing path) | 53 | 24 | 5 |

**G3 FAILED.** Arm C added 5 wrong answers on questions arm A had abstained on, and fixed 3 wrong answers of arm A, so confident-wrong rose from 22 to 24. The lane answered 23 requests (18 correct, 5 wrong), declined 5, and abstained 0.
Median model time for an answered question was 16.6 s (p90 34 s, max 56 s); execution 0.04 s; no question needed a second attempt.

### Failure taxonomy of the five (from `Explain-Question`, the key, the lane's query and the oracle)

| Question | What the lane did | Class |
|---|---|---|
| TOWE-min "smallest position" | `min(longitude)`; the layer declares "position" a name of **latitude** | lane defect: a named column not used |
| CDR-latest "most recent call start time among the calls" | `WHERE call_type = 'CALL'`, a filter the question never asked for (the word "call" belongs to the column name "call start time") | lane defect: unrequested filter |
| SUBS-earliest "first activation date among the registered numbers" | answered from the plate view (`v_anpr`), the question's own words copied into the label | lane defect: wrong evidence family |
| CDR-month_count "June 2026" (2,455 vs key 2,463) | filtered `event_time`, the layer's documented field for case-level time filtering; the key uses `call_start` | instrument/definition: two valid time fields; not yet resolved |
| CDR-night "at night" (1,986 vs key 1,768) | hours of `event_time`; the key uses hours of `call_start` | instrument/definition: same cause |

### Changes made after this reading, before any further run

Code, not prompt wording (the rule against re-wording prompts stands; none of the prompt text changed): the verifier now (1) requires the query to use a column the question names by a word only that column answers to, with the longest name winning (`FIELD`);
(2) refuses an equality filter on an enumerated column that the question does not say, or whose value the layer does not declare (`FILTER`); (3) the view shortlist weights a column name that only one view has.
Also the earlier reach fix (a computed-value question labelled a concept or clarification may reach the lane; only an answered result replaces the old reply) and the probe over every free-text records column.
Arm C is run once more on the corrected image, against the same gates. The two instrument items are reported, not tuned for: the answer will name the time field it used.

## Run 2 (2026-10-06): arm C on the corrected image, and arm B

Image from commit `ae2db2a`. Same 82 questions.

| Arm | Correct | Confident-wrong | Abstained | Lane answered / declined |
|---|---|---|---|---|
| A | 35 | 22 | 25 | - |
| C, run 1 (`arm-lane-C-run1`) | 53 | 24 | 5 | 23 (18 right, 5 wrong) / 5 |
| C, run 2 | 57 | 21 | 4 | 24 (22 right, 2 wrong) / 4 |
| B | 63 | 13 | 6 | 75 (63 right, 12 wrong) / 7 |

**G3 (factory set): met by C run 2** (21 against A's 22; the corpus and pre-flight halves are still to be measured with `Run-Regression`).
**G4: not met by B.** Confident-wrong 13 (15.9%) against a limit of 11 and 10%. Correct 77% against a line of A + 15 points = 58%: met.
**G5: not met by B.** `ACCE-unbound_name-01` was declined by the lane (no evidence family recognised) and the existing path answered it with a collection overview.
**G7: met.** No HTTP errors. The abstention count in the lane's own header was wrong (abstentions were recorded as `declined`): fixed below, and it also means the header summary of this run under-reports abstentions.

### Failure taxonomy of arm B's 13 (from `Explain-Question`, key, query and answer side by side)

| Item(s) | What happened | Class |
|---|---|---|
| CDR-avg, CDR-max, ANPR-avg, TOWE-avg, IPDR-avg | the lane's number is right (`31.6118` for 31.611806..., `3,905,649.9336` for 3905649.9336) but written to four places; the judge accepted only two-decimal or full-precision forms | **instrument:** the judge's number matcher |
| CDR-top_group, IPDR-top_group | the right caller / user, written `923,461,678,183`: identifiers were digit-grouped like quantities | **lane defect, fixed:** text columns are shown as stored |
| CDR-sum | `WHERE call_type = 'GPRS' OR direction = 'DATA'`: the word "data" of the column name "data volume" was read as a request for data-type calls | **lane defect, fixed:** words spent on naming a column do not also ground a value |
| IPDR-max | read the call records (`v_cdr`) for a question about "data sessions", with the same GPRS filter | **lane defect, fixed:** the query must read the evidence family the question names |
| ACCE-unbound_name | declined (no family recognised); the existing path then gave a total | **lane defect, fixed:** a name is an abstention even when no family is recognised |
| CDR-month_count, CDR-night | `event_time` against the key's `call_start` | **instrument/definition**, unchanged from run 1 |

### Changes made after this reading, before any further run

Lane: the five fixes marked above; rounding instead of truncation in the headline number; abstentions recorded as `abstained` in the header.
Instrument: the factory's number matcher now also accepts a number the text states that is the key correctly rounded to the places it shows (two or more), with or without thousands separators. It cannot accept a wrong value (the specs include 31.6119 and 31.7 against 31.611806, and 1,986 against 1,768).
It is applied to every arm, including the baseline and A, by `factory.py rejudge`, so no arm is advantaged. The original `results.json` files are untouched; the re-read goes to `results-rejudged.json`.

## Run 3 (2026-10-06): arm B on the corrected image (`12361ac`), factory set, 82 questions

| Arm | Correct | Confident-wrong | Abstained |
|---|---|---|---|
| A | 35 (43%) | 22 (26.8%) | 25 |
| C, run 2 | 57 (70%) | 21 | 4 |
| B, run 1 (old image, as judged then) | 63 | 13 | 6 |
| **B, run 2** | **73 (89%)** | **2 (2.4%)** | **7** |

Lane for B: 82 of 82 responses carry the header; answered 75 (73 right, 2 wrong); abstained 6 (all ABSTAINED); declined 1; two attempts 13; model time median 8.7 s, p90 21.4 s, max 53.1 s; execution median 0.02 s.

| Gate | Result on the factory set |
|---|---|
| G4 | **met**: confident-wrong 2 (limit 11 and 10%); correct 89% against 58%. The "every abstention states a reason" half is read from the answers below. |
| G5 | **met**: every `unbound_name` question abstained (6 of 6); every `absent_value` question was answered "nothing found" after the search (8 of 8); none got a total or a zero. |
| G6 | met on the factory set; the response says it searched every identifier field of every evidence family. |
| G7 | met: 0 HTTP errors; the one declined request was counted. |
| G1 (PII probe and identity questions), G2, G3 on the 103 corpus and the pre-flight | **not yet measured**: `Run-Regression` for A, C and B. |

The two remaining wrong answers are `CDR-month_count` and `CDR-night`, the time-field definition difference (the lane filters `event_time`, the layer's documented field for case-level time filtering; the key uses `call_start`). They are unchanged since run 1 and are not tuned for.
The answer now states which time column the condition used. Whether "June" and "night" for call records mean `event_time` or `call_start` is a curation decision for the product owner.

### Reading

Arm B's result is the lane's own accuracy with the existing path unchanged underneath it (declined requests fall to it). On this question set it is a 46-point gain in correct answers and a 24-point drop in confident-wrong ones against arm A.
The set is the one the failures were found on, so it is evidence of the fixes, not of the next unseen question: a new generation (new seed) and the 103-question corpus are the independent checks, and the lane stays off by default until they are in.
