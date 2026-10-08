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
| G5 | **met**: every `unbound_name` question abstained (5 of 5); every `absent_value` question was answered "nothing found" after the search (6 of 6); none got a total or a zero. |
| G6 | met on the factory set; the response says it searched every identifier field of every evidence family. |
| G7 | met: 0 HTTP errors; the one declined request was counted. |
| G1 (PII probe and identity questions), G2, G3 on the 103 corpus and the pre-flight | **not yet measured**: `Run-Regression` for A, C and B. |

The two remaining wrong answers are `CDR-month_count` and `CDR-night`, the time-field definition difference (the lane filters `event_time`, the layer's documented field for case-level time filtering; the key uses `call_start`). They are unchanged since run 1 and are not tuned for.
The answer now states which time column the condition used. Whether "June" and "night" for call records mean `event_time` or `call_start` is a curation decision for the product owner.

### Reading

Arm B's result is the lane's own accuracy with the existing path unchanged underneath it (declined requests fall to it). On this question set it is a 46-point gain in correct answers and a 24-point drop in confident-wrong ones against arm A.
The set is the one the failures were found on, so it is evidence of the fixes, not of the next unseen question: a new generation (new seed) and the 103-question corpus are the independent checks, and the lane stays off by default until they are in.

## Run 4 (2026-10-06): the 38-question pre-flight and the 103-question corpus (`Run-Regression`), image `50d9afc`

| Arm | Pre-flight | Corpus questions | HTTP errors |
|---|---|---|---|
| A | **38 of 38** | 103 | 0 |
| C | **35 of 38** | 103 | 0 |
| B | **27 of 38** | 103 | 0 |

**G2 and G3 are not met on the pre-flight.** The three arm C failures are questions the existing path *refused* (arm A: withheld) and the lane then answered: "calls that lasted longer than ten minutes" (a list), "do any subscribers share the same handset" (answered "Number of unique handsets: 1": a different question), "where was 923001110001 seen" (5 rows).
Arm B has the same three, plus these, each read from the output:

| Pre-flight item | Arm B answer | Class |
|---|---|---|
| "Is the number 03001234567 mentioned in any audio?" | "No record in this case contains 03001234567: I searched every identifier field of every evidence family" while the old path finds it in a recording | **false absence: G6 violated.** The search covers structured evidence, not text inside recordings, images and documents; the sentence claimed more than was searched |
| "Find OCR text mentioning Investigation Workspace" | abstained: "could not tie 'Investigation Workspace' to any field" | a text-search question read as an unknown name |
| "Find plate LEB15491 in the images" | a row dump of `Camera id: no value ...` | wrong evidence family for a media question |
| "How many plate reads were produced from the images?" | 1,057 (camera sightings); the old path says 307 (plate reads from images) | wrong evidence family for a media question |
| "Show the call type breakdown", "...HTTP status codes" | "Top result: call_type = GPRS (5,863). 5 rows." and "Top result: status = 500 (46)" | presentation: the first row of a breakdown called "top", and a raw code where the layer declares a name |
| "How many call records are from August 2026?", "How many transactions have an amount above 50000?", "Where was plate ZZZ-0000 seen?" | a correct number or an honest absence, in different words than the pre-flight's expected phrase | wording check of the pre-flight, not a wrong answer |

### Changes made after this reading, before any further run

* Text and media evidence (a recording, a photo, a document, OCR, a face) is left to the retrieval path unless the best-matching view is a derived media view.
* A question that states a relationship between records ("share the same ...", "in common", "between ... and ...") is declined: a relationship needs a join and the lane does not attempt one (the existing detector, `questionStatesRelationalCondition`, is reused).
* The absence sentence says what was searched ("every identifier field of the structured evidence"; text inside recordings, images and documents "was not searched here"). It no longer says "every evidence family".
* A short two-column result is a breakdown and is listed in full with the layer's display names ("Data session: 5,863"), not summarised as "top".

Arm B on the 103-question corpus (`number_checks_stated` 33 of 47, the same as A) and arm C (36 of 47) are read next with `replay_corpus.py compare`; the honesty items (H11, H12, H13, NEG-01) are the first to look at.

## Run 5 (2026-10-06): the 103-question corpus read side by side (A against C, A against B), image `50d9afc`

Arm C changed 21 of 103 answers (all questions the existing path had withheld, asked back or answered "unavailable"); 82 are identical. The number checks lost: none; gained: 3. Arm B changed 76.
No change touched a question the existing path had answered correctly in arm C, which is what G2 asks. G1 (no case PII) holds: no changed answer shows a name, an alias or a masked value; the face-identity question stayed a clarification in every arm.

Read one by one, the 21 changes in arm C were:

| Count | What | Verdict |
|---|---|---|
| 8 | plausible, useful answers where the existing path had withheld (CDR-07 most different contacts: 45; CDR-11 calls by 923001110001: 447; TWR-02 tower location; H5 four cameras; H12 average beam width 65; M4, M10, M20, M21 over the derived media views) | answered |
| 3 | an honest "no record contains ... I searched every identifier field" for an identifier that is absent (NEG-01, NEG-02, H13) | answered, but the sentence over-claimed (see below) |
| 5 | **wrong evidence family or dropped condition** | defects |
| 3 | text/media questions the lane should not take (VID-01 "plates in the videos" answered from camera sightings; H1 same; IMG-03/AUD-01/IMG-01 abstained on a phrase read as a person's name) | defects |
| 2 | **false absence**: X-01 "where does 03001234567 appear across all evidence" and AUD-03, answered "no record contains it" while a recording does | defects (G6) |

The five wrong-family or dropped-condition answers: CASE-01 "records for each record type" answered by call type; CASE-03 "what time period does this case cover" answered from the access log alone; M3 "plate reads still needing manual review" counted all sightings; M1/M2 "plate reads ... from the images" answered from camera sightings (1,057 against 307).

### Changes made after this reading, before any further run

* **Plurals fold.** "plate reads" finds the curated phrase "plate read" (the derived plate-read observations); the longest name wins, so the camera sightings' "plate" does not also name that family.
* **A grouping must be a column.** "for each record type" is refused when no column of the evidence is a record type; "for each source file" and "by protocol" are accepted.
* **A question about the whole case** ("this case", "across all evidence") that names no evidence family is declined.
* **A question that names no family, no column and no declared value** is declined (7 of 249 generated questions: "log entries", "registered numbers" are not words the layer knows); a name the evidence cannot hold is still an abstention.
* The earlier fixes (text and media cue, relationships, absence wording, breakdowns) stand.

Not fixed, stated plainly: **a condition the question states in words no field of the layer maps to is still dropped silently** (M3 is the example: "still need manual review"). The verifier checks identifiers, declared values, magnitudes, time, names, named columns and groupings; it cannot check a free predicate. A word-list check flagged 94 of 249 generated questions (most harmless words), so it was not adopted. The two routes that remain are the layer naming the predicate as a field, and the model reporting what it did not apply; neither has been built.

### Instrument

The factory's anpr nouns included "plate reads", which the layer (and the existing path) reads as the image-derived plate-read observations, not the camera sightings the answer key counted. Those three arm A "wrong" answers (ANPR count, sum, night) were the existing path following the layer. The noun is replaced by "camera sightings", and a new question set (different seed) is generated for the next run so the lane is measured on questions it was not corrected on (`New-QuestionSet`, then `Run-Arm B -Questions questions-demo-v2.json -Tag -v2`).

## Run 6 (2026-10-06): unseen question set (seed 2, 82 questions) and the pre-flight again, image `c92d9ad`

| Arm B | Correct | Confident-wrong | Abstained |
|---|---|---|---|
| factory set, tuned on | 73 (89%) | 2 | 7 |
| **unseen set** (`questions-demo-v2.json`) | **70 (85%)** | **6 (7.3%)** | 6 |

Unseen set, lane: 82 of 82 carry the header; answered 74 (68 right, 6 wrong); abstained 6; declined 2 (both correct); two attempts 6; model time median 7.2 s, p90 18.6 s, max 29.0 s; execution 0.02 s. G5: all five unbound-name questions abstained; all absent-value questions said "nothing found". G7: 0 HTTP errors.
The six wrong: `CDR-month_count` and `CDR-night` (the time-field definition, unchanged); `ACCE-top_group` and `ACCE-count_distinct` (a value word, "failed", used by the factory as a field name: instrument wording, to be confirmed); `CDR-avg` and `CDR-max` on "event latitude" (to be explained).
Pre-flight, arm B: **33 of 38** (27 before the run-5 fixes). The five remaining misses are a correct number or an honest absence worded differently from the check's expected phrase (three), and two list answers whose headlines were uninformative ("Top result: ... 861", "5 rows."): fixed after this run: a list is described as a list, never as a ranking, and a short single column is shown in full.

## Run 7 (2026-10-07): arm A on the unseen set, the corpus comparisons, and the last wrong answers, image `c1d3735`

| Unseen set (82) | Correct | Confident-wrong | Abstained | Time |
|---|---|---|---|---|
| A (old path) | 42 (51%) | 18 (22%) | 22 | up to 130-290 s |
| B (lane first) | 70 (85%) | 6 (7.3%) | 6 | median 7 s, p90 19 s |

**G4 on the unseen set: met** (6 against a limit of 9 and 10%; correct 85% against A + 15 points = 66%). B improved 35 questions and was worse on 5. Four of A's five unbound-name questions were answered with a total; B abstained on all five.
**G2 on the corpus: met.** Arm C changed 18 of 103 answers (85 identical); every one is a question the old path had withheld, asked back or answered "unavailable" (CDR-11, TWR-02, NEG-01, H4, H5, H11, H12, M3, M4, M6, M7, M8, M10, M15, M18, M20, M21, P2). Number checks lost: none; gained: 5. 0 HTTP errors.
Arm B changed 70 and lost two number checks (H7, H13), both read in the explain below.

### The five questions where B was worse than A on the unseen set

| Question | What the lane did | Class |
|---|---|---|
| CDR-avg, CDR-max ("event latitude of the call records") | added `WHERE call_type = 'CALL'`: the "call" of "call records" (the evidence's own name) was read as a request for the call type CALL | **lane defect, fixed:** words of the evidence's own name are spent and do not ground a value |
| ACCE-count_distinct ("unique failed") | counted distinct source IPs where `status LIKE '4%'` | **lane defect, fixed:** LIKE, NOT IN and <> on an enumerated column are filters too |
| ACCE-top_group ("most common failed") | `status NOT IN ('200','201')`, reading "failed" as "not successful"; the key counts all statuses because the factory used "failed" as a field name | **fixed (the NOT IN is now refused) and instrument:** "failed" is added to the factory's bad synonyms |
| CDR-night | hours of `event_time` against the key's `call_start` | the open time-field definition |

Also found in reading the corpus: arm B abstained on "How many server errors are in the access log" (H7) although the old path answers it, because "server errors" (plural) did not match the layer's value phrase "server error". The value check now matches a value by any phrase the layer gives it (code, display name, synonyms) with plurals folded, and a phrase the layer gives to a value is never spent on naming a column.

Pre-flight on arm C: 36 of 38 (the two misses are list answers whose headlines were fixed after that image).

## Run 8 (2026-10-07): the 18 answers arm C added to the corpus, read one by one (image `c1d3735`)

Right or plausible: CDR-11 (447 calls, matches the old path's own ranked list), TWR-02, NEG-01 and H4 (honest absence, worded as structured evidence), H5 (4 cameras), H12 (beam width 65), M4, M6, M10 (OCR confidence 0.451 from the image-text reads), M15, M20, M21; M18 and H11 are lists (their headlines were fixed after this image); P2 abstained with its reason; M7 answered 0.
**Two wrong, so G3 is not met on the corpus:**

| Question | What the lane did | Class |
|---|---|---|
| M3 "How many plate reads still need manual review?" | counted all 1,057 camera sightings | wrong family **and** a dropped condition. The layer names the family ("plate read") and the field (`manual_review_required`, "needs manual review"), but the family was not among the two the shortlist offered, so neither the view check nor the field check could apply |
| M8 "earliest offset at which a plate group was first seen" | `1,784,008,800` from the camera sightings (an epoch value) | wrong family: "plate group" names the video plate-group observations, not offered |

Fix: a family the question names is always offered, first (`govSQLOffer`); the named-field check then forces `manual_review_required`. Specified (M3's query must use the field; the family of M3 and M8 is offered first); on 249 generated questions the offered families cover the question's own family as before (245 of 249; the four misses are the "registered numbers" vocabulary gap).
Confirmation to come: `Verify-Round` (the unseen set on arm B as `-v3`, the corpus and pre-flight on C and B, the comparisons against A).

## Run 9 (2026-10-07): confirmation round on image `4223547`, and the decision table

| Measurement | Result |
|---|---|
| Unseen set, arm B | 72 correct (88%), **3 confident-wrong (3.7%)**, 7 abstained; median 7.5 s, p90 21.9 s |
| Corpus, arm C | 0 errors; number checks 40/47 (A 33, B 38); pre-flight 36/38 |
| Corpus, arm B | 0 errors; pre-flight 33/38 |
| Arm C's 18 added corpus answers | 14 right or plausible, M4 and P2 abstained with their reason, M3 (307 plate reads that need manual review: the question's family and field are now used) and M8 (earliest offset 0, from the plate groups) answered from the right family |
| Arm B worse than A on the unseen set | 2 of 82 (CDR-max: `call_type = 'CALL'` from "the calls"; CDR-night: the time-field definition) |

The two pre-flight "refuse" items that still print FAIL in arm C are answers the lane now gives correctly (a list of 200 calls with their columns; "5 values (location): Airport Road; DHA Phase 5; Gulberg; Liberty Market; Model Town"). The check was written for the old path's refusal; the expectation is stale, not the answer.

After this reading: a single-word value ("call") is only asked for by that exact word, so the plural "calls" is the noun for the records, not the value CALL (`CDR-max`). Specified; this only tightens the verifier (it can turn an answer into an abstention, not add one).

### Gates

| Gate | Result |
|---|---|
| G1 PII | met: no changed answer, in any arm, shows a name, alias or masked value |
| G2 arm C changes nothing already answered | met: 18 of 103 changed, every one a question the old path had withheld, asked back or called "unavailable"; 0 lost number checks |
| G3 arm C adds no wrong answer | met on the factory set (C 21 against A 22) and on the corpus read (no wrong answer among the 18) |
| G4 arm B accuracy | met on the tuned set and on the unseen set (3.7% confident-wrong; correct 88% against A's 51%) |
| G5 no silent drop (names, absences) | met: every unbound-name question abstained; every absent-value question was answered "nothing found" after the search |
| G6 honest absence | met: the sentence names what was searched and says transcripts, images and documents were not |
| G7 instrument | met: 0 HTTP errors; declined requests counted |
| G8 time | reported: arm B median 7.5 s, p90 21.9 s, max 30.8 s on the unseen set; the old path alone took 1-5 minutes on many of the same questions |

### Decision (to be taken by the product owner)

Stage 1: turn `FORENSIC_GOVERNED_SQL` on with `FORENSIC_GOVERNED_SQL_FIRST` off. Recommended: it can only change a question the existing path declined, and everything it adds is checked and shows its query.
Stage 2 (lane first) is NOT recommended yet: it replaces correct old-path answers with lane answers, wins on accuracy and time in the measurement, and should first run for a while as stage 1 with the confident-wrong count watched.
Stage 3 (front door) waits for R5.
Rollback: `Disable-Lane`, or unset the switch and recreate the API container. Open items: the time field for "June" and "night" on call records; vocabulary gaps ("registered numbers", "log entries"); a condition stated in words no layer field maps to is still dropped silently.

## Run 10 plan (2026-10-07): the lane on the main case, written before the arms were read

The lane has only been measured on `nexusai-forensic-demo` (structured records, no media). The case the analysts work in is `nexusai-multimodal-product-acceptance` (structured records plus derived media: audio, video, ANPR images, faces, documents; 42 of 43 sources ready). Nothing below has been read yet: when this was written arm A had answered one question (ACCE-count_all-01, CORRECT) and arm C had not started.

| Item | Fixed now |
|---|---|
| Question set | `New-QuestionSet -Seed 3 -Name questions-multimodal.json -Collection nexusai-multimodal-product-acceptance`: 249 generated, 79 asked (`--per-intent 1`, one per family and intent). Structured families only: the factory has no independent oracle for derived media |
| Arms | A (both switches off, the control) and C (lane after the existing path), same question file, same image, same model; arm B is not run this round |
| Image | `nexusai-forensic-records-api:latest`, id `5b253d84d32d`, created 2026-10-07 07:30:36 UTC, built straight after commit `07093fcf` (07:28:59 UTC). It does **not** contain `cb0f3bcf` (07:57:32 UTC) although the handoff note says it does; that commit changes only the `intent` field of an answered lane response, not the lane's logic |
| Switch state | asserted from `docker inspect` by `Run-Arm` before the first question, never from the shell |

Gates for this run, from the table above, unchanged:

| Gate | Pass condition on this set |
|---|---|
| G2 | every question arm A answered (CORRECT, WRONG or NOT_STATED) has the identical verdict and analyst text in arm C; every difference is on a question arm A abstained on |
| G3 | confident-wrong in arm C is at most arm A's |
| G5 | in arm C every `unbound_name` and `absent_value` question is answered correctly or abstained; none is answered with a total or a zero |
| G6 | every "none found" in arm C says what was searched |
| G7 | 0 HTTP 5xx; no timeout counted as a verdict; declined requests stay in the denominator |
| G1 | no answer, in either arm, shows a name, an alias or a masked value |
| G8 | reported, not gated: median, p90 and max of model, execution and total time for the questions the lane answered in arm C |

G4 (arm B accuracy) is not evaluated: arm B is not run. A wrong answer in arm C gets a failure taxonomy (what the lane did, against the key and its query), not a reworded prompt.

Media questions (audio, video, plates from images, faces, documents) are tried by hand through `Ask-Case` and the workspace. They are exploratory and not gated: there is no independent key for derived media yet. The corpus questions M1 to M21 and P1, P2 are the reference list for what the lane answered on this case before. This file records only counts, ids and failure classes: the question file and the `arm-*` directories hold case data, stay local and gitignored, and must not be committed.
### Known before arm C was started (2026-10-07, 15:40)

Arm A (lane off) is complete: 79 asked, 34 CORRECT, 21 confident-wrong, 23 ABSTAINED, 1 ERROR; median 61.7 s per question, p90 123 s, max 293 s; no row carries a lane header.

The one ERROR is `ANPR-top_group-01` ("Which plate number appears most often in the camera sightings?"): HTTP 500 after 85.7 s. Asked again alone on the lane-off stack it fails again, after 1 s (the plan is now cached), with `{"error":"source-native group cardinality exceeds 100"}`: the existing path's guard on the number of groups, returned as a 500 instead of an abstention. It is deterministic, not load, and the API logs nothing for it.

Stage 1 (`governedSQLFallback`, `api/forensic_records/governed_sql_fallback.go`) passes the existing path's reply through whenever its status is not 200, and tries the lane only when a 200 reply declined. The lane therefore never sees this question, and arm C is expected to return the same 500. G7 ("0 HTTP 5xx") is then reported as not met in both arms by this one inherited error, and is not attributed to the lane. Whether an existing-path 5xx should also reach the lane is an owner decision: a 500 is a fourth outcome, and the decision record allows three.
## Run 10 (2026-10-07): the lane on the main case, arms A and C, image `5b253d84d32d` (built after `07093fcf`)

Question set `questions-multimodal.json` (seed 3, 79 asked, structured families), the same questions, image and model in both arms; the switch state of each arm was asserted from the container (A: both false; C: `FORENSIC_GOVERNED_SQL=true`, `_FIRST=false`).

| Arm | Correct | Confident-wrong | Abstained | HTTP error |
|---|---|---|---|---|
| A (lane off) | 34 (43%) | 21 (27%) | 23 | 1 |
| C (lane after the existing path) | **54 (68%)** | 21 (27%) | 3 | 1 |

Transitions A to C: ABSTAINED to CORRECT 18, ABSTAINED to WRONG 2, WRONG to CORRECT 2, ABSTAINED to ABSTAINED 3, CORRECT to CORRECT 34, WRONG to WRONG 19, ERROR to ERROR 1.

**What the lane did in arm C.** It touched 25 of the 79 questions: answered 22 (20 CORRECT, 2 WRONG) and abstained on 3 (two name questions and one date-range question). The existing path answered 53 and returned the one error. By family the lane answered: access log 3, ANPR 4, CDR 4, IPDR 3, subscriber 3, tower 3, transaction 2.

**Where the 21 confident-wrong answers of arm C come from.** 2 from the lane, 19 from the existing path, the same 19 as in arm A (stage 1 cannot change an answer the existing path gave). The 19 by intent: unbound_name 3, count_distinct 2, count_eq 2, night 2, max 2, min 2, latest 2, earliest 1, date_range 1, sum 1, avg 1; by family: IPDR 6, access log 4, CDR 4, ANPR 3, subscriber 2. This is the part only the lane going first (arm B) can address; on the demo case arm B cut confident-wrong from 22% to 3.7% (run 9). Arm B has not been measured on this case.

### Gates (fixed before the run; read as measured)

| Gate | Reading |
|---|---|
| G2 | **Not met by the letter**: 3 questions arm A answered differ in arm C. None is the lane replacing a data answer. ACCE-latest-01 and TRAN-latest-01: the existing path replied "A bounded general definition is unavailable for that term" (route `terminal`), which the factory's judge scored WRONG in arm A, and the lane replaced it with a correct answer (the designed "called unavailable" case). IPDR-absent_value-01: the lane did not run (no lane header); the existing path's wording differs between runs (arm A names the number searched; arm C, with a warm plan cache, says only that there are no IPDR sessions in the case) |
| G3 | **By the judge's count, met (21 against 21). On a fair reading, not met (19 against 21).** The judge scores the two "definition unavailable" declines above as confident-wrong, which flatters arm A. The lane added two wrong answers on questions arm A abstained on: ANPR-night-01 and CDR-night-01 |
| G5 | Written for arm B, which was not run, so it cannot be evaluated. Information for arm C: unbound_name 3 WRONG (the existing path answered with a total) and 2 ABSTAINED (the lane); absent_value 5 CORRECT. No change is possible in stage 1 for an answer the existing path already gave |
| G6 | Met: 2 lane "none found" answers, both say what was searched |
| G7 | **Not met, by one inherited error**: ANPR-top_group-01, HTTP 500 (`source-native group cardinality exceeds 100`), identical in both arms, from the existing path; stage 1 never hands a non-200 reply to the lane |
| G1 | Not exercised: structured questions only. The identity probes (H2, H5) are in the 103 corpus |
| G8 | Reported: lane model time for the 22 answered, median 17.8 s, p90 35.8 s, max 52.5 s; execution median 0.0 s, max 0.1 s; two attempts: 2. Including the existing path's own time before the lane: median 20.3 s, p90 202 s, max 315 s. (Demo case, run 9: model median 7.5 s, p90 21.9 s.) Times between arms are confounded by the persisted plan cache: arm A filled it, so arm C's median per question was 4.5 s against 61.7 s |

Consequence written before the run: "fail any gate: the switch stays off and the cause is recorded". Causes: (1) G7, an existing-path error; (2) G2, an instrument scoring and an existing-path wording difference; (3) G3, two answers that depend on an open definition. None is a fault in the lane's own checks. Stage 1 was already on in the running container when this run began; leaving it on is therefore a decision of the product owner and an exception to the rule above, not a result of it.

### Failure taxonomy of the two wrong answers the lane added

| Question | What the lane did | Class |
|---|---|---|
| ANPR-night-01 | counted camera sightings whose `event_time` hour is 0 to 5 (165); the key uses the hour of the source's own recorded timestamp (162) | the open time-field definition |
| CDR-night-01 | the same condition over call records (1,323); the key uses the call start recorded at the source (1,283) | the same |

The answer names the time column it used. Which column "night", "June" and similar mean for each family is a curation decision for the product owner; it is not tuned for here.

### Instrument and tooling findings

* The factory judge scores the existing path's "definition unavailable" decline as confident-wrong (its abstention test matches neither that wording nor the route `terminal`). Corrected, arm A reads 34 correct, 19 wrong, 25 abstained, 1 error; arm C is unchanged. The judge was not changed after seeing the results; a one-line correction applied to every arm by `rejudge` is proposed.
* The plan cache persists across arms, so the old path is faster, and sometimes worded differently, in whichever arm runs second.
* `Compare-Arms` only compares the demo arms; tagged arms are compared with `python factory.py compare arm-lane-A-mm arm-lane-C-mm`.

### Media questions tried by hand (not gated; no independent key for derived media)

Ten questions through `Ask-Case`, judged against the SQL-verified values of the corpus: 8 correct, 1 correct but not distilled, 1 honest abstention, 0 wrong. The existing path answered 5 (plate groups, audio segments, faces, text regions, and the document search, 0.4 to 4.3 s); the lane answered 4 (largest supporting reads, latest audio end offset, average OCR confidence, and which model produced the face vectors, 29 to 50 s in total) and abstained once (highest plate detection confidence: after two attempts the model's query did not use the field the question names, so no number was shown). The "which model" answer was a 20-row table of one value, because the query has no `DISTINCT`: the content is right, the headline is a row count. Two earlier tries in the workspace UI: a correct lane answer (307 plate reads needing manual review) was shown under "Analysis unavailable" (the image predates the intent fix); a lane abstention on "Were any of the transcribed audio segments taken from video?" blanked the page (the defect fixed in `eb70a2b7`). Text and document content questions stay on the existing path, as designed (R5).

### Reading and decision (product owner)

On the main case, stage 1 lifts correct answers from 43% to 68% and cuts "cannot answer" from 29% to 4%, without replacing any data answer the existing path gave. It does not touch the existing path's 19 confident-wrong answers (24% of the set); only the lane going first can. Decisions: keep stage 1 on or roll it back (`Disable-Lane`); the time field for "night"; whether an existing-path 5xx should reach the lane; correcting the judge; whether to measure arm B on this set (about two hours) before any step towards stage 2.
### Decision taken (product owner, 2026-10-07, in the working session)

Stage 1 stays ON (`FORENSIC_GOVERNED_SQL=true`, `FORENSIC_GOVERNED_SQL_FIRST=false`) although G2, G3 (on the fair reading) and G7 were not met. This is an exception to the rule "fail any gate: the switch stays off", taken knowing the causes recorded above: an existing-path error, a judge scoring, and an open definition. Stage 1 is not persistent across a full restart of the stack. Still open: the time field for "night", whether an existing-path 5xx should reach the lane, the judge correction, and arm B on this case.
### Correction (2026-10-07, later the same day): the cause of the two "night" answers is a defect, not an open definition

The taxonomy above calls the two wrong "night" answers "the open time-field definition", and the reading above says none of the misses is a fault in the lane's own checks. Both statements are superseded. Reading the data (read-only SELECTs) shows two concrete defects in how the lane reads time, and the same cause behind the "June" and "night" gaps recorded in runs 1 to 7:

| Finding | Evidence (main case unless stated) |
|---|---|
| **Time zone.** The layer's `event_time` is the normalised `forensic.records.timestamp`, a UTC instant. For CDR the source clock is naive local time (`CALL_START_DT_TM`), and the ingest converted it with `FORENSIC_DEFAULT_SOURCE_TIMEZONE=Asia/Karachi`. A condition on the hour, day or month evaluated on the UTC value counts a different window than "as recorded at the source", which is what the lane's own answer note says | the column is exactly 5 hours behind the call-start text on all 5,000 CDR rows; hours 0 to 5 of the UTC column: 1,323 (the lane's answer); converted to Asia/Karachi: 1,283 (the key). Demo case: every CDR row is exactly 5 hours behind; "June 2026" by the UTC column 2,455 (the lane), converted to Asia/Karachi 2,463 (the key) |
| **No shift elsewhere.** Access log, ANPR, IPDR and transaction times are explicit UTC (`Z`) in the source | shift 0 on all of their rows with a source time |
| **Fallback time.** 307 ANPR rows are plate reads derived from media (299 from videos, 8 from photos; `metadata.derived_from_media`). They have no camera time, and the normalised column holds their ingestion time. Counting them in a time condition counts a time that is not when anything was seen | all 307 have `timestamp = ingested_at`; 3 of them fall in hours 0 to 5, which is the 165 against 162 of ANPR-night-01 |

So the lane has a real gap: it exposes time in UTC while claiming the source clock, and it lets ingestion times answer questions about when something happened. The existing path's six confident-wrong answers on time questions are different causes (a condition dropped, a list given where a date was asked, "between" read as end-exclusive) and are not explained by this.

What remains a decision for the product owner is one precise question: which clock defines "night", a day or a month. Either the clock recorded at the source (UTC for the `Z` families, local for CDR, which is what the answer key and the lane's note say) or one jurisdiction zone (Asia/Karachi) for every family. CDR needs the conversion to Asia/Karachi under both readings.

Proposed, not implemented (it changes the API and needs a rebuild): (A) expose each time column to the model in the chosen clock (an `AT TIME ZONE` in the view definition, `govSQLCTE`), so hour, day and month conditions mean what the answer says; (B) give media-derived rows no time in time conditions. Each needs a spec with a 5-hour fixture and a live-database spec, then a re-measure of CDR-night, CDR-month_count and ANPR-night on both cases. The current behaviour is the unsafe default, so whether it ships behind a switch is the owner's call.
## Run 11 plan (2026-10-07): the case clock, written before any measurement

**What was prepared** (tested offline and kept as a patch, `reports/governed-sql-20261005/case-clock-fix.patch`, because the Go commit is blocked by the pre-commit hook for reasons unrelated to it; NOT deployed; the running image is unchanged): the governed SQL lane reads and shows time on one case clock, `FORENSIC_ANALYSIS_TIMEZONE` (default Asia/Karachi), for every evidence family, as decided by the product owner (the decision document, "Time: one case clock"). It sets the zone for its read-only transaction, formats times by the column's type (a timestamptz on the case clock with its abbreviation; a date as the date, never shifted), says which clock an answer used, refuses a query that converts to another zone, and keeps a record with no time of its own out of time conditions. The factory's answer keys and the stand-in loader follow the same clock.

**Offline evidence.** Lane specs with a database: 259 before, 283 after, all passing; without a database (what CI sees): 214 before, 231 after. Forcing the case zone to UTC, which re-creates the old behaviour, makes 11 of the new or changed specs fail, so they do hold the behaviour in place. Lint: 0 issues. Factory tests: 15 pass. The stand-in loader now reads a naive time in the source zone and gives a no-time row its ingest time, as the real ingest does.

**Predictions, fixed now from read-only SELECTs on the main case over the raw payload (not from the lane).** After deployment the lane must return exactly these for "at night" (hours 0 to 5 on the case clock, records with no time of their own excluded):

| Family | Case clock (prediction) | Old reading, hours of the UTC instant |
|---|---|---|
| CDR | 1,283 | 1,323 |
| ANPR (camera sightings with a recorded time) | 176 | 165 |
| IPDR | 371 | 1,199 |
| Access log | 0 | 0 |
| Transactions | 2 | 2 |

On the demo case "calls in June 2026" must be 2,463 (the lane said 2,455). The answer keys move with the decision: ANPR night 162 to 176, IPDR night 1,199 to 371; the CDR keys do not move.

**Gates for the re-measure, fixed now.**

| Gate | Pass condition |
|---|---|
| T1 | every `night` question the lane answers equals its new key exactly, on both cases |
| T2 | every `month_count`, `date_range`, `earliest` and `latest` question the lane answers equals its new key exactly |
| T3 | no other question changes verdict or text against run 10 arm C (the change must not move what does not depend on the clock) |
| T4 | 0 HTTP 5xx and no timeout counted, other than the existing-path 500 already recorded |
| T5 | every lane answer that reads or shows a time says which clock it used |
| T6 | no ANPR time answer counts a plate read that has no recorded time |

**Procedure, each step needing the product owner's go-ahead because it rebuilds and redeploys:** build the API image from this branch; recreate the API with stage 1 as before; ask arm C on the multimodal set (79 questions, keys re-evaluated at run time) and on the unseen demo set (82 questions); re-judge arm A's stored time answers against the new keys so the control is read on the same clock. No prompt wording is tuned if a gate fails: a failure gets a taxonomy and a smaller scope.

### Run 11 plan, correction found before any measurement (2026-10-07)

The product owner gave the go-ahead for this procedure on 2026-10-07 (build, recreate the API only, arm C on the two sets). The image `nexusai-forensic-records-api:latest` was then built from commit `b111b585`; the previous image is kept as `nexusai-lane-arms-prev:before-case-clock`. The running API had not been recreated and no question had been asked when the flaw below was found.

**What was wrong in the plan.** It said the keys are "re-evaluated at run time". The factory does re-evaluate a key at run time, but it runs the SQL stored in the question file, and both files (`questions-multimodal.json`, `questions-demo-v2.json`) were generated before the case clock and store the old reading of every time field (the text with its `Z` dropped). Run as planned, a correct case-clock answer (ANPR night 176, IPDR night 371) would have been judged against the old keys (162, 1,199) and marked wrong. It was found by reading the stored SQL, not by a result.

**The correction touches the instrument, not a gate.** `factory.py rekey` rewrites exactly that stored expression to the current one in a copy of each question file (suffix `-clock`): same questions, same ids, same order (so `--per-intent 1` picks the same spreads), keys refreshed with read-only SELECTs, and a file in which an old-form expression is left behind is refused. `factory.py reclock` judges an arm's saved answers to the time questions again against those keys and writes `results-clock.json`; `results.json` is not touched. Tests: 24 factory tests pass (6 judge, 18 time); breaking the rewrite on purpose makes the 4 tests that exercise it fail.

**Keys after re-keying (read-only, before any measurement).** 20 of the 249 multimodal questions and 22 of the 296 demo-v2 questions contain a time expression and were rewritten. Keys that moved: ANPR night 162 to 176 and IPDR night 1,199 to 371, both as predicted, and one the prediction table did not list: the earliest transaction date, 2026-07-10 to 2026-07-11, on both cases. Every other time key is unchanged, among them CDR night (1,283 on the multimodal case, 1,768 on the demo case) and access log night (0). Where the files carry the question, the predictions above hold against the new keys: CDR 1,283, ANPR 176, IPDR 371, access log 0. Transactions at night (2) and demo calls in June (2,463) are not among the sampled questions; they are asked by hand after deployment.

**As run.** Arm C on `questions-multimodal-clock.json` (tag `-mm11`) and `questions-demo-v2-clock.json` (tag `-v2-11`), `--per-intent 1`. Run 10's arm C (`arm-lane-C-mm`) is the T3 baseline on the multimodal set. No arm C exists for the demo-v2 set (only A and B), so T3 cannot be read against run 10 there and is reported as not assessable on that set; what can be said instead (the questions the lane did not touch must equal arm A's answer, since stage 1 only acts after the existing path declines) is stated as a description, not as a gate. The gates are applied by `evaluation/question_factory/check_clock_gates.py`, written and tested (10 tests) before any Run 11 answer existed and committed before the results; it reports a gate it cannot decide as NOT ASSESSABLE, never as a pass. T5 needs each lane-answered time question asked again, because the notes that state the clock are not kept in the saved results.

## Run 11 results (2026-10-07, 22:57 to 23:52): the case clock, measured

Image `nexusai-forensic-records-api:latest` (`17a2b5179e27`, built from `b111b585`), stage 1 (`FORENSIC_GOVERNED_SQL=true`, `_FIRST=false`), `FORENSIC_ANALYSIS_TIMEZONE=Asia/Karachi`. Arm C on `questions-multimodal-clock.json` (79 asked) and `questions-demo-v2-clock.json` (82 asked), `--per-intent 1`, keys on the case clock. 55 minutes for both sets; no container restart and no timeout. Nothing in the plan, the gates, the questions or any prompt wording was changed after the plan was committed, except the instrument correction recorded above. The gates were applied by `check_clock_gates.py`.

| Set | Arm | Correct | Wrong | Abstained | Error |
|---|---|---|---|---|---|
| multimodal (79) | A, lane off (control; unchanged when read on the case clock) | 34 | 21 | 23 | 1 |
| | run 10 arm C (old image), read on the case clock | 53 | 22 | 3 | 1 |
| | **run 11 arm C** | **55 (70%)** | **19 (24.1%)** | 4 | 1 |
| demo, unseen v2 (82) | A, lane off (control; unchanged when read on the case clock) | 42 | 18 | 22 | 0 |
| | **run 11 arm C** | **60 (73%)** | **19 (23.2%)** | 3 | 0 |

The lane in run 11: multimodal, 25 of 79 responses carry the header: answered 21 (all correct), abstained 3, declined 1. Demo, 22 of 82: answered 19 (18 correct, 1 wrong), abstained 3, declined 0. All 19 wrong answers on the multimodal set, and 18 of the 19 on the demo set, are the existing path's, which the lane never sees in stage 1. Lane answers took a model time median of 19.8 s (multimodal) and 9.3 s (demo); the total-time p90 was 274 s and 207 s.

| Gate | Multimodal | Demo (unseen v2) |
|---|---|---|
| T1 night | PASS: the lane answered 2 of 4 (ANPR 176, CDR 1,283), both equal the key | **FAIL**: the lane answered 2 of 4: ANPR 176 equals the key; `CDR-night-01` answered 1,771, key 1,768 |
| T2 month, date range, earliest, latest | PASS: 16 questions, the lane answered 9, all equal their keys | PASS: 18 questions, the lane answered 11, all equal |
| T3 nothing else moves | **FAIL**: `TOWE-min-01` CORRECT to ABSTAINED; no verdict or text change outside the time questions otherwise | NOT ASSESSABLE (no run 10 arm C exists on this set). Description, not a gate: of the 60 questions the lane did not answer, 58 equal arm A in verdict and text and 2 differ in wording only (existing path, plan cache) |
| T4 errors | PASS: only the recorded existing-path 500 | PASS: none |
| T5 clock stated | PASS: 11 asked again, all name the case clock | PASS: 13 asked again, all name the case clock |
| T6 no no-time ANPR reads | PASS: 2 ANPR time answers by the lane (earliest, night), correct | PASS: 3 (earliest, latest, night), correct |

**Predictions.** CDR night 1,283 held (multimodal, answered by the lane). ANPR night 176 held on both sets (the lane). Transactions at night 2 and demo calls in June 2026 2,463 held (asked by hand after deployment). IPDR night 371 and access-log night 0 were not reached by the lane: in stage 1 the existing path answered first with a total (2,500 and 1,000), so the lane's answer for them was not measured; the keys are from read-only SELECTs. The earliest transaction date, which the prediction table did not list, was answered 2026-07-11 by the lane (the old image said 07-10).

**Taxonomy of the two failures** (no prompt wording was changed).

1. **T1, demo `CDR-night-01`: the model's choice of time column.** The lane's query applied "night" to `call_start OR call_end`; the key uses the call start. The 3 extra records are calls that began between 23:36 and 23:57 and ended between 00:05 and 00:17 (start 1,768, end 1,763, either 1,771). Replicating the lane's own reading in SQL on the stored data, on the case clock, gives 1,768, and the no-time rule changes nothing, so this is not a clock or a data error. The answer discloses it ("The time condition was applied to: Call end time (call_end), Call start time (call_start)", and the clock), and the same query came back on a second ask. The curated layer says `cdr.event_time` is the column for case-level time filtering; the verifier accepts any time column for a time condition. On the multimodal case the model used `event_time` and got 1,283.
2. **T3, multimodal `TOWE-min-01` ("the smallest position among the towers"): a lost lane answer, suspected prompt sensitivity.** Run 10: the lane answered at its second attempt, correctly. Run 11: declined at the first attempt ("the model judged the question not answerable from the listed columns"), and 5 of 5 further asks on the same image declined the same way, so it is consistent and not random variance. The only prompt-visible change in this release is the time rule (rule 6). That the rewording is the cause is NOT proven: it needs the same asks on the old image, which is kept for rollback. Cost: CORRECT to ABSTAINED; no wrong answer.

**Other observations (not gates).**
* In stage 1 the existing path answers first, and it is confidently wrong on time questions before the lane is consulted: multimodal 6 of 20 (`ACCE-night-01`, `ANPR-latest-01`, `CDR-earliest-01`, `CDR-latest-01`, `CDR-date_range-01`, `IPDR-night-01`); demo 7 of 22 (`ACCE-night-01`, `CDR-earliest-01`, `CDR-latest-01`, `CDR-date_range-01`, `CDR-month_count-01`, `IPDR-night-01`, `SUBS-earliest-01`). All are identical in arm A. Only a lane-first arm can change them.
* The existing path's CDR date range on the multimodal case (2026-04-11 up to, not including, 04-21) says 1,322. Counting on the stored UTC instant gives 1,333; the case-clock key is 1,373. So the existing path is wrong for a reason not established here. On the demo case its April CDR count is 890 against a key of 5,177.
* A date-only field (activation date) is shown as "00:00:00 PKT": correct and it names the clock, but it reads like a time.

**Consequence under the plan's own rule** ("no prompt wording is tuned if a gate fails: a failure gets a taxonomy and a smaller scope"): nothing was changed. Smaller-scope changes are proposed, not implemented, each needing the product owner's go-ahead and a new pre-registered run: (a) a verifier obligation that a time condition applies to the family's `event_time` unless the question names the start or the end; (b) put the time rule in the prompt only for a question that has a time condition, so every other question's prompt is byte-identical to run 10's (this removes the suspected cause of T3 by construction); (c) a lane-first (arm B) spot check of the time questions only, to measure IPDR night 371 and access-log night 0 end to end. State at the time of writing: the new image is deployed with stage 1 on; the old image is tagged `nexusai-lane-arms-prev:before-case-clock` for rollback.

## Run 12 plan (2026-10-08): the two run 11 misses, written before any measurement

**State when written.** Prepared and tested offline; the Go change is kept as `reports/governed-sql-20261005/run12-time-column.patch` (7 files, applies to the branch tip, reproduces the working tree) and as uncommitted files, because the Go pre-commit hook cannot pass on this machine and the one-time `--no-verify` of run 11 is spent. NOT deployed: the running image is still run 11's (`17a2b5179e27`), stage 1 on, stage 2 off. The product owner approved preparing this offline on 2026-10-08, with no Go commit or deploy without asking again.

**What changes, and nothing else.**
1. **The shared prompt rules are the run 10 text again.** Rule 6 reads "Times are as recorded at the source…" (sha256 `e6004f75…`, pinned by a spec). What a time question needs to know, the case clock and which time column to use, is said in that question's own hint, which only a question with a time cue receives. This removes by construction the suspected cause of the T3 miss (`TOWE-min-01`).
2. **A time condition belongs on the record's own time** (`event_time`, present in the five dated families: access log, ANPR, CDR, IPDR, transactions) **unless the question names another time of the record** ("started", "ended"; a word counts when it begins the way a name of the column does, so a doubtful case is decided in favour of letting the query use that column). A new obligation `TIME_COLUMN` sends the model back once, then the lane abstains with the reason. Subscribers, towers and the derived media views have no such field and are unchanged. This is the fix for the T1 miss (`CDR-night-01`).

**Offline evidence.** Lane specs: 303 with a database (283 before) and 247 without (231 before), all pass; 20 are new (16 without a database, 4 with one). Two assertions that encoded the old behaviour were changed on purpose (a `call_start` condition for a question that never says "start", and the case-clock sentence in the shared rules). Breaking the new behaviours on purpose fails the specs that hold them (the check disabled: 5 specs; the hint ignoring the record's own time: 2). `golangci-lint` 0 issues; vet clean on Windows and linux/amd64; 40 factory tests pass. **Prompt equivalence, measured:** the prompts the lane builds for all 249 multimodal and 296 demo-unseen questions were compared with those of the run 10 code (commit `faa9ee74`; no lane commit between the run 10 image and it touched the prompt, verifier or catalog code): the 241 and 286 questions without a time cue get a system AND a user prompt byte-identical to run 10's; the 8 and 9 with a time cue get an identical system prompt and a changed user prompt (the hint). Run 11 had changed the system prompt of every question.

**Predictions, fixed now.**
* P1, demo `CDR-night-01`: the lane answers **1,768**, the key (run 11: 1,771).
* P2, multimodal `TOWE-min-01`: the lane answers **24.8138**, the key, as in run 10. If it still declines, the reword of rule 6 was not the (sole) cause of the run 11 decline; that is a finding to report, not a gate.
* P3, multimodal, outside the time questions: every verdict and text equals run 10's arm C (that is gate T3 exactly), because the prompts are byte-identical to run 10's and the model's decoding repeated itself in 5 of 5 asks.
* P4, both sets: the lane's time answers stay correct and name the case clock: ANPR night 176, CDR night 1,283 (multimodal) and 1,768 (demo), and the earliest and latest answers as in run 11.
* P5, both sets: the existing path's answers are exactly run 11's (stage 1 is unchanged), so the 19 and 18 wrong answers it gave remain.

**Gates, part A (arm C, stage 1, both sets, the same `-clock` question files and spreads, tags `-mm12` and `-v2-12`).** T1, T2, T4, T5 and T6 as defined for run 11, applied to both sets. T3 is read three ways: on the multimodal set against run 10's arm C (`arm-lane-C-mm`), which must now pass; and on both sets against run 11's arm C (`arm-lane-C-mm11`, `arm-lane-C-v2-11`) as "nothing gets worse": no question outside the time questions changes from CORRECT to another verdict (improvements are listed, not counted against). The demo set has no run 10 arm C, so there T3 is only the "nothing gets worse" reading. P1 and P2 are read as hypothesis tests next to the gates.

**Part B, a lane-first spot check of the time questions only (stage 2, arm B).** It runs only after part A, and only with the product owner's separate go-ahead for the stage 2 switch: `Run-Arm -Arm B -Intents night,month_count,date_range,earliest,latest` on both question files (tags `-mm12b`, `-v2-12b`; 20 and 22 questions), then `Enable-LaneStage1` to return to stage 1. It exists because in stage 1 the existing path answers first and is confidently wrong on 13 of these questions (multimodal `ACCE-night-01`, `ANPR-latest-01`, `CDR-earliest-01`, `CDR-latest-01`, `CDR-date_range-01`, `IPDR-night-01`; demo `ACCE-night-01`, `CDR-earliest-01`, `CDR-latest-01`, `CDR-date_range-01`, `CDR-month_count-01`, `IPDR-night-01`, `SUBS-earliest-01`), so the lane never ran on them.
* B-P1: the lane answers correctly at least **9 of those 13**. Keys: access-log night 0, IPDR night 371, ANPR latest 2026-07-19, CDR date range 1,373 (multimodal) and 1,013 (demo), CDR month count (April) 5,177, the CDR, subscriber and demo earliest and latest dates as in the question files. The doubtful ones are the two CDR date ranges: the lane abstained on the subscriber date range in run 10 and 11 ("the query did not apply the condition 'between 2023'").
* B-gates: (B1) no lane answer to a time question differs from its key; (B2) no time question that was CORRECT in run 11's arm C is worse in arm B; (B3) no HTTP 5xx and no timeout on these questions. B2 is the real test of stage 2: the lane is now ahead of an existing path that was right on some of these.

**Procedure, each step needing the product owner's go-ahead.** (1) How the Go change is committed (the hook; the one-time `--no-verify` is spent, so another authorisation, or a Linux environment after the stale `tests/e2e/distributed` calls are fixed). (2) Tag the running image `nexusai-lane-arms-prev:case-clock-run11`, build from the run 12 commit, recreate only the API with stage 1. (3) Part A and its gates (`check_clock_gates.py` with the baselines above, and `--ask` for T5). (4) Part B, separately. Rollback: the run 11 image (`case-clock-run11`) or the run 10 image (`before-case-clock`). **If a gate fails:** a taxonomy and a smaller scope, no prompt wording tuned, as in run 11.

## Run 12 part A results (2026-10-08): both run 11 misses fixed, no gate failed

Image `nexusai-forensic-records-api:latest` (`3db72cf63191`, built from `93f1489d`, committed with the product owner's second one-time `--no-verify`, now spent), stage 1, case clock `Asia/Karachi`; arm C on the same `-clock` question files and spreads as run 11 (tags `-mm12`, `-v2-12`). The previous images are tagged `nexusai-lane-arms-prev:case-clock-run11` and `:before-case-clock`. Nothing in the plan, gates or questions changed after the plan was committed; the checker's two extra readings (T3b, part B) were committed before any result.

| Set | Arm | Correct | Wrong | Abstained | Error |
|---|---|---|---|---|---|
| multimodal (79) | A, lane off | 34 | 21 | 23 | 1 |
| | run 11 arm C | 55 | 19 | 4 | 1 |
| | **run 12 arm C** | **56 (70.9%)** | **19 (24.1%)** | 3 | 1 |
| demo, unseen v2 (82) | A, lane off | 42 | 18 | 22 | 0 |
| | run 11 arm C | 60 | 19 | 3 | 0 |
| | **run 12 arm C** | **61 (74.4%)** | **18 (22.0%)** | 3 | 0 |

The lane answered 22 of 22 correctly on the multimodal set (run 11: 21 of 21) and 19 of 19 on the demo set (run 11: 18 of 19); it abstained on 3 in each. The existing path's wrong answers are exactly run 11's (19 and 18), so the lane has no wrong answer left in either set.

| Gate | Multimodal | Demo |
|---|---|---|
| T1 night | PASS: lane answered 2 of 4, both equal their keys | PASS: lane answered 2 of 4, both equal their keys |
| T2 month, date range, earliest, latest | PASS: 9 answered by the lane, all equal | PASS: 11 answered by the lane, all equal |
| T3 against run 10 arm C | **PASS**: no verdict or text change outside the time questions | not assessable (no run 10 arm C) |
| T3b nothing gets worse than run 11 | PASS (one improvement, `TOWE-min-01`) | PASS (none worse) |
| T4 errors | PASS: only the recorded 500 | PASS: none |
| T5 clock stated | PASS: 11 asked again, all name the clock | PASS: 13 asked again |
| T6 no no-time ANPR reads | PASS | PASS |

**Predictions.** P1 held: demo `CDR-night-01` is answered **1,768** on the first attempt (run 11: 1,771). P2 held: multimodal `TOWE-min-01` is answered **24.8138** (attempt 2, as in run 10); with the prompt rules the run 10 text again and nothing else changed for that question, the run 11 decline is attributed to the reword of rule 6. P3 held: the multimodal set equals run 10 outside the time questions exactly. P4 held: ANPR night 176 on both sets, CDR night 1,283 and 1,768. P5 held. Differences from run 11 across both sets: `TOWE-min-01` (ABSTAINED to CORRECT), `CDR-night-01` (WRONG to CORRECT), and one harmless wording change in a time answer ("Latest" to "Most recent").

Timing: the first lane question after the API restart took 323 s (cold prompt cache for the restored rules); the other lane answers took a median of about 13 s.

## Run 12 part B results (2026-10-08): the lane first, on the time questions only

`Run-Arm -Arm B -Intents night,month_count,date_range,earliest,latest` on the same two question files (tags `-mm12b`, `-v2-12b`; 20 and 22 questions), with `FORENSIC_GOVERNED_SQL_FIRST=true` for 11 minutes, then stage 1 put back and checked (switch false, same image `3db72cf63191`). Read against run 11's arm C on the same questions.

| Set | Stage 1 (existing path first) | Lane first |
|---|---|---|
| multimodal, 20 time questions | 13 correct, 6 wrong, 1 abstained | **19 correct, 0 wrong**, 1 abstained |
| demo unseen, 22 time questions | 14 correct, 8 wrong | **20 correct, 0 wrong**, 2 abstained |
| both, 42 | 27 correct, 14 wrong, 1 abstained | **39 correct, 0 wrong**, 3 abstained |

| Gate | Multimodal | Demo |
|---|---|---|
| B1 every lane answer equals its key | PASS (19 answered) | PASS (20 answered) |
| B2 nothing right gets worse | PASS | **FAIL**: `SUBS-date_range-01` CORRECT (existing path) to ABSTAINED |
| B3 no 5xx or timeout | PASS | PASS |
| B-P1 prediction (at least 9 of the 13 the existing path got wrong are now right) | 6 of 6 | 6 of 7 |

B-P1 held: **12 of 13**; the miss is the demo `CDR-date_range-01`, which the lane abstained on (no wrong number). Examples: access-log night 0 (existing path: 1,000), IPDR night 371 (2,500), ANPR latest 2026-07-19, CDR date range 1,373, CDR April 5,177 (existing path: 890), demo CDR night 1,768. Warm lane timing: total median 13.4 s and 11.1 s, p90 31.5 s and 19.6 s.

**Taxonomy of the B2 miss and of the three date-range abstentions.** The reason the lane gives, "the query did not apply the condition `between 2025`" (or 2026, 2023), is the `MAGNITUDE` obligation firing on a date range: `questionStatesMagnitudeCondition` (`constraint_obligations.go`) reads "between" followed by a digit, so the year that begins "between 2025-03-09 and 2025-12-13" is taken for a number to compare, and a query that compares dates cannot satisfy it. It is a defect of the checker, not of the model, and a deterministic one. Of the four date-range questions the lane saw lane-first, three abstained (multimodal subscriber, demo CDR, demo subscriber; the multimodal subscriber one also abstained in stage 1 in runs 10 to 12); the multimodal CDR one was answered. The abstentions are honest (no wrong number), but lane-first replaces whatever the existing path said, and for the demo subscriber question that was a right answer.

**Reading for stage 2.** On these 42 time questions lane-first removed all 14 wrong answers and lost one right one to an abstention. That is promising and not yet a reason to change the default: only a full arm B on every question can decide it. **Smaller-scope fix, proposed and not implemented (it needs the product owner's go-ahead and its own pre-registered run 13):** read the magnitude on the question with its dates removed, so a date range is a time condition and not a number. Predicted: the three abstentions become answers equal to their keys (multimodal subscriber 2, demo CDR 1,013, demo subscriber 1) and nothing else changes.

## Run 13 plan (2026-10-08): the date-range abstentions, written before any measurement

**State when written.** Prepared offline with the product owner's go-ahead (2026-10-08); the Go change is kept as `reports/governed-sql-20261005/run13-date-range.patch` (3 files, applies to the branch tip, reproduces the working tree) and as uncommitted files, because the Go pre-commit hook cannot pass here and both one-time `--no-verify` authorisations are spent. NOT deployed: the running image is run 12's (`3db72cf63191`), stage 1 on.

**What changes, and nothing else.** In the lane only, the stated quantity is read on the question with its dates removed (`govSQLMagnitudeText`: ISO dates, slash dates and "between <year>" are blanked before the shared detector runs). A date range then stays a time condition (the time cues are read from the original text) and is no longer a number to compare. "Between 10 and 20 calls", "more than 2000" and the rest are read exactly as before; the existing path's own detector is untouched.

**Offline evidence.** Lane specs: 311 with a database (303 before) and 254 without (247 before), all pass; 8 are new (7 without a database, 1 with one: a date-range question answered on the first attempt with the count the independent key gives). Undoing the change on purpose fails 3 specs. `golangci-lint` 0 issues; vet clean on Windows and linux/amd64. **Scope, measured:** the prompts the lane builds for all 249 multimodal and 296 demo-unseen questions, compared with those of the committed code: the system prompt differs for none, and the user prompt differs for exactly 4 questions, the four date-range questions (multimodal `CDR-date_range-01`, `SUBS-date_range-01`; demo `CDR-date_range-01`, `SUBS-date_range-01`). Every other question is byte-identical.

**Predictions, fixed now.**
* P1, part A (arm C, stage 1, both sets, tags `-mm13`, `-v2-13`): against run 12's arm C exactly one verdict changes: multimodal `SUBS-date_range-01`, the lane's abstention in stage 1, becomes **2**, its key (CORRECT). The other three date-range questions are answered by the existing path first and stay as they are (multimodal CDR wrong at 1,322; demo CDR wrong at 912; demo subscriber right at 1).
* P2, part B (lane first, the 42 time questions, tags `-mm13b`, `-v2-13b`): the three abstentions become answers equal to their keys, multimodal subscriber **2**, demo CDR **1,013**, demo subscriber **1**; at least 2 of the 3 must; the multimodal CDR date range stays **1,373**.
* P3: nothing else changes in either part.

**Gates.** Part A: T1, T2, T4, T5, T6 as in run 12; T3 against run 10's arm C on the multimodal set and T3b against run 12's arm C on both sets. Part B: B1 (every lane answer equals its key), B2 (nothing right gets worse than stage 1), B3 (no 5xx or timeout); with the date ranges answered B2 is expected to pass for the first time. Procedure and rules as in run 12: the owner's go-ahead for the Go commit route, for tagging the running image (`nexusai-lane-arms-prev:case-clock-run12`), building, recreating only the API, and the two parts; stage 1 restored after part B. If a gate fails: a taxonomy and a smaller scope, no prompt wording tuned.

## Phase 1 plan (2026-10-08): the lane first on every question, the stage 2 decision; written before any measurement

**What is being decided.** Whether `FORENSIC_GOVERNED_SQL_FIRST` (stage 2: the lane before the existing path) is good enough to become the product's default for the structured families. In stage 1 the existing path answers first and is confidently wrong on 18 to 19 of every 80 questions (runs 11 and 12), which the lane never sees. Part B of run 12 showed the effect on the time questions alone (14 wrong answers to 0). This measures it on everything. It is a measurement; the switch goes back to stage 1 afterwards, and only the product owner flips the default.

**Runs, after run 13 is finished and stage 1 is restored.** (1) Arm B, every question of the two spreads already used (multimodal 79, demo unseen 82), the same `-clock` question files, tags `-mmp1` and `-v2p1`. (2) The 103-question corpus and the 38-question pre-flight with the lane first, on the demo case (`Run-Regression`, with a tag so that the earlier `replay-lane-*` and `preflight-lane-*` results, which this document cites, are not overwritten); the control is the existing `replay-lane-A`, because the existing path is unchanged. Same image as run 13, same model, stage 1 restored at the end.

**Predictions, fixed now.**
* Multimodal, arm B: **at least 64 of 79 correct, at most 5 wrong** (stage 1, run 12: 56 correct, 19 wrong). Demo unseen, arm B: **at least 72 of 82 correct, at most 5 wrong** (stage 1: 61 correct, 18 wrong; arm B on the older image and the older clock: 72 of 82 with 3 wrong).
* The wrong answers that remain are few and each has a named cause; the name questions (`unbound_name`) abstain instead of returning a total.
* The existing path's HTTP 500 on the plate question disappears or stays the only error.

**Gates, fixed now** (the pre-registered G-gates above, applied to arm B on these sets; G1 and G6 are read on the corpus and pre-flight).

| Gate | Pass condition |
|---|---|
| S1 (G4) accuracy | on each set: CORRECT at least arm A's plus 15 points (multimodal at least 46 of 79, demo at least 55 of 82); confident-wrong at most half of arm A's and at most 10% of the questions (multimodal at most 7, demo at most 8); every abstention carries its reason |
| S2 nothing right gets worse | against run 13's arm C on the same set: no question goes from CORRECT to WRONG; at most 3 per set go from CORRECT to ABSTAINED, each listed with its cause |
| S3 (G5) no silent drop | every `unbound_name` and `absent_value` question is answered correctly or abstained on; none is answered with a total or a zero |
| S4 (G7) instrument | 0 HTTP 5xx and 0 timeouts counted as verdicts (the recorded plate-question 500 is allowed only if it still occurs) |
| S5 (G1, G6, G2 read for arm B) regression | on the 103 corpus and 38 pre-flight: no number check lost against arm A, 0 HTTP errors, no personal data in any answer to the PII probes, no "none found" without the all-families search; every changed answer text is listed and read |
| S6 (G8) time | reported: median, 90th percentile and maximum of model time and total time for lane answers |

**The goal reading, reported and not gated:** the roadmap's targets of at least 90% correct and at most 1% confident-wrong on in-scope questions. **If a gate fails:** a taxonomy and a smaller scope; the switch stays off and nothing is reworded. **What the result can and cannot decide:** passing S1 to S5 on these sets is the evidence for proposing stage 2 as the default for the structured families; it does not cover text and media evidence (roadmap R5) or any question the factory does not generate.