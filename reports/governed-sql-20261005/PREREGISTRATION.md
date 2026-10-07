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
