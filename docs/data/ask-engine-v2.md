# Ask engine v2: the catalogue, the validator and the prompt (milestone M1, first part)

Status 2026-10-14: **built and tested; not wired to any request path; no model is called by this code yet.** It is the second build step of roadmap 6.8, after the analyst views (`docs/data/analyst-views.md`, migration 017, not applied). Nothing here changes the governed SQL lane: that lane's validator, views and switches are untouched.

## What is in `api/forensic_records/`

| File | What it is |
|---|---|
| `ask_engine_catalogue.go` | The schema catalogue: loads the analyst views, their descriptions and columns from the database's own comments (`loadAskCatalogue`); refuses a view or column without a description; renders the compact catalogue text; the fixed prompt rules (`askPromptRules`); `askPrompt` assembles rules, the shortlisted views and the retrieved examples; `askShape` names the kind of result from its columns |
| `ask_engine_catalogue.golden.json` | The pinned snapshot of the catalogue (seven views, 71 columns), so the validator and the prompt are tested without a database. Regenerate with `python -X utf8 tools/case_generator/check_ask_catalogue.py --write-golden` after any change to migration 017 |
| `ask_engine_examples.json` | Seven verified question and SQL pairs (top contacts, common contacts, a bounded recursive path, cells used, who held an address, device changes, daily activity). The model imitates them; the code never executes one as an answer |
| `ask_engine_validate.go` | The gate for model-written SQL (`askValidate`): PostgreSQL's own parse tree, an allowlist of every node, function and operator |

## The validator, in one table

It keeps the lane validator's method and widens it for what link analysis needs: joins, WITH, set operations, arrays and bounded recursion over the analyst views.

| Rule | Code on refusal |
|---|---|
| one statement, a SELECT; no INTO, no locking, no VALUES, no TABLESAMPLE | `ONE_STATEMENT`, `SELECT_ONLY`, `NO_INTO`, `NO_LOCKING`, `NO_VALUES`, `NOT_ALLOWED` |
| only the offered analyst views, unqualified or as `analyst.<name>`, at most six, plus the query's own WITH names; none of those names may reuse a view name or start with `pg_` or `v_` | `NO_SCHEMA`, `UNKNOWN_RELATION`, `NO_VIEW`, `TOO_MANY_VIEWS`, `CTE_NAME` |
| a JOIN needs a real ON or USING; relations listed with commas need a WHERE that relates them; `ON true`, `ON 1 = 1`, NATURAL and CROSS JOIN are cross products | `CROSS_PRODUCT` |
| only `generate_series` and `unnest` in FROM, and `generate_series` may not run past 100,000 | `RANGE_FUNCTION` |
| functions, operators and casts only from the lists in the file (no `pg_sleep`, `set_config`, `current_setting`, file or large-object functions, no `@@`, no `::regclass`, no array types, no `current_user`) | `FUNCTION`, `OPERATOR`, `TYPE`, `VALUE_FUNCTION` |
| `WITH RECURSIVE`: only the second part of the UNION refers to itself; a depth column that starts in the first part, grows by a positive literal in the recursive part and is bounded by `depth < N` or `depth <= N` so that at most **4 hops** are taken; a cycle guard (an `= ANY` or `<> ALL` array test); an outer LIMIT | `REC_SHAPE`, `REC_NO_DEPTH_BOUND`, `REC_DEPTH`, `REC_DEPTH_COLUMN`, `REC_CYCLE_GUARD`, `REC_LIMIT` |
| an outer LIMIT, when present, is a literal of at most 5,000 | `LIMIT` |
| size, depth, constant length | `TOO_LONG`, `TOO_COMPLEX`, `CONSTANT` |
| a column that is neither a column of the views used nor a name the query defines | `UNKNOWN_COLUMN` (the message lists the real columns, so the repair needs no database round trip) |

Only the parser's own canonical text is returned for execution (`askValidated.SQL`); comments and odd quoting are gone. The result also reports the views read, the columns referenced, the constants (so the checks can compare them with the question), whether the query recurses and how many hops it can take.

**What it cannot judge** (and what does): that the recursion really walks (the laptop measurement found a 4B model writing a recursion that never recursed: the checks of roadmap 6.8.6 and the planted-truth bank), that a join relates the right columns (the database's errors and the checks), that the cost is acceptable (the executor's EXPLAIN ceiling, not built yet), that the answer is the answer to the question (obligations and back-translation, not built yet). The validator is one layer: the read-only role that can reach only the views, the scope the views carry, the read-only transaction, the statement timeout and the row cap are the others. The execution recipe is in `docs/data/analyst-views.md` and includes `SET LOCAL search_path = analyst, pg_catalog`, because the model names views without a schema.

## The prompt

`askPrompt` puts the fixed rules first and never edits them per question, then the shortlisted views, then retrieved examples. A spec checks that the start is identical for every question and independent of the shortlist's order: on the CPU laptop a repeated prompt start costs about 2 s against about 40 s cold (roadmap 6.8.13). The rules tell the model the conventions of roadmap 6.8.5 (name `src` and `dst` for a graph, `lat` and `lon` for a map, `t` and `value` for a series, `label` and `value` for a ranking), the case clock, the services and hubs flags, and the recursion rules.

## The shortlist and example retrieval (`ask_engine_retrieve.go`)

The prompt must stay short (every token is latency on the CPU laptop), so the model is shown only the views the question needs and the one or two verified examples closest to it. Both are chosen by **rules, with no embedding and no model call**; the code records that an embedding shortlist once caused two live defects, so the rule is written first, measured against a labelled set, and an embedding ranker is adopted only if it measurably beats it.

- **Shortlist** (`catalogue.Shortlist(question, max)`): identifiers are replaced by placeholders (phone numbers, 15-digit identifiers, IP addresses, dates, cell keys) and counted; three signals add up per view: a curated lexicon (`askLexicon`, English, extendable per language by native speakers), the question's words found in the view's own description and columns (capped so that a description alone can order views but never select one among stronger ones), and the kinds of identifier (an address points at internet sessions, a 15-digit number at devices, two numbers with a linking word at contact pairs unless the question is spatial). At most three views; a view stays only if it scores at least 45% of the best and at least 1.5. A weak signal returns the best three and `Confident = false`, so the caller can ask the user to clarify. Every choice carries its reasons.
- **Examples** (`askRetrieveExamples`): only examples whose views are all in the shortlist (the model is never shown a view it was not offered), ordered by identifier-free word overlap weighted by rarity, plus a bonus when the question's cues match the example's result shape and a small one for the example of the best view. At most n (the engine will use two).
- **Ten verified examples** now (three more than before: co-location, top applications, data by host); all pass the validator and run as the read-only role (`check_ask_catalogue.py`).

**Measured** on 60 hand-labelled English questions (`ask_engine_retrieval_cases.json`, 20 of them held out and measured once, the rest used to tune the lexicon): the first measurement, before any tuning, gave a shortlist recall of 95% (held-out 100%) and the labelled example among the first two in 84%; after fixing a stemmer inconsistency (websites against website), adding month names to the stop words, a spatial exception to the linking rule and a fallback to the best view's example, recall is 100% (held-out 100%), the mean shortlist is 1.4 views, and the example is among the first two in 100%. The gates written before the first measurement were recall 95% (held-out 90%), mean size at most 2.5 (tightened to 1.8 afterwards as a regression guard) and example hit 85%.

**Read those numbers with care.** The questions are the author's, in English, written for this catalogue, and the lexicon was tuned on them: 100% says the rules do what their author meant, not that real analysts' phrasing will be found. The held-out set is only 20 questions. The analysts' questions and the native-speaker sets of roadmap 6.4 replace this set before any claim is made, and the Urdu, Roman Urdu, Punjabi and Pashto lexicons do not exist yet.

## Evidence

- `go test ./api/forensic_records -ginkgo.focus "Ask engine"` (validator, catalogue, retrieval: 112+ specs). Earlier form: `-ginkgo.focus "Ask engine (validator|catalogue)"`: 103 specs, about 1 s of specs (the package takes about 50 s to build): 16 accepted queries, 61 refused with the code each must be refused for, the shortlist, the column message, what a recursive query reports, canonical text only, the constants, the catalogue checks, the snapshot round trip, the examples, the prompt prefix, the shape table.
- `python -X utf8 tools/case_generator/check_ask_catalogue.py`: a throwaway database with migrations 001 to 015 and 017, the Go specs "Ask engine on a real database" as the application role: the catalogue built from the database's comments equals the pinned snapshot; every example passes the validator **and runs** as `forensic_analyst_ro` in a read-only transaction with the right result shape; the role cannot read a base table. 4 specs.
- 23 deliberate faults in the validator and the catalogue (other schemas allowed, cross products allowed in four ways, the depth, cycle and LIMIT rules removed or loosened, a sleep function, a full-text operator, the clock-of-who-runs-it, `::regclass`, TABLESAMPLE, a WITH name reusing a view, the shortlist ignored, INTO, unknown columns accepted, descriptions not required, a wrong shape rule, a prompt start edited per question): each made the specs fail. One interrupted run left a fault in the file for a while; it was found by checking the mutation texts against the files and restored, and the full specs were rerun clean.

- Shortlist and retrieval: 17 deliberate faults (stemming, the identifier signals, the spatial exception, the lexicon, the description cap, the shortlist cap and cut, confidence, examples outside the shortlist, the shape cue, the best-view fallback, placeholders, month words, repeated stemming), each made the specs fail; five were missed by the first set of specs and the specs were added.

## The run, the executor and the switch (milestone M1 second part, 2026-10-15)

Status: **built and tested on throwaway databases; not deployed; the switch is off by default; migration 017 is still not applied to the live database.** Everything below is new Go in `api/forensic_records/` and changes no answer while `FORENSIC_ASK_V2` is off.

| File | What it is |
|---|---|
| `ask_engine_frame.go` | Reads a question once, by code, into a frame: numbers (every written form, 0300 1234567 or +92 300 1234567, becomes 923001234567), devices, addresses, cells, dates, months, years, "top N", hops, relative times, and the cues (incoming, outgoing, SMS, night, count, rank, earliest, latest, services, hubs, path). `askCheckFrame` holds a validated query against it (the obligations, table below); `askCheckResult` holds the result against it |
| `ask_engine_exec.go` | The executor: `SET LOCAL ROLE forensic_analyst_ro`, scope settings, the case clock as the session time zone, read-only transaction, statement timeout (15 s), EXPLAIN cost ceiling (5,000,000 planner units), row cap (1,000, one extra row fetched to notice truncation). `askProbe` says where an identifier occurs in the case |
| `ask_engine_describe.go` | "How this was derived" in plain words, generated from the query's own parse tree (conditions, what it returns, grouping, ordering, limit, hops, case clock): back-translation by code, so it cannot claim what the query does not do |
| `ask_engine_run.go` | The run (eligibility, frame, shortlist, examples, prompt, model, validator, obligations, executor, result checks, answer), the model call (strict JSON schema, temperature 0, 480 output tokens at most), the switches and the audit record |
| `ask_engine_response.go` | The answer built from the result by code: headline by shape, `result_shape`, derivation steps, limitations, the abstention and the "too broad" replies, the log line |
| `ask_engine_fallback.go` | `FORENSIC_ASK_V2=true`: the engine runs after the existing path declines and never replaces an answer the existing path gave; `FORENSIC_ASK_V2_FIRST=true` (in `query.go`): before it. A decline passes the request on unchanged |

**Declined and abstained are different.** A decline hands the question to the next path (no analyst view fits, a relative time, the model cannot answer, no database). An abstention is an answer the user sees ("not answered, and why"): the engine HAS a query but cannot show that it asks everything the question asks, and the existing path would answer a different question.

### The obligations (what the validated query must carry)

| Obligation | Refused when |
|---|---|
| IDENTIFIER | the question names a number, device, address or cell that the query does not use as a text constant |
| FOREIGN_IDENTIFIER | the query names a number, device or address the question does not (a model copying the number of a retrieved example) |
| DATE, MONTH, YEAR | a date, month or year of the question is not in the query (a month needs a date inside it, or `EXTRACT(MONTH ...)`) |
| LIMIT, HOPS, RECURSION | "top five" is not a limit of five; "within three hops" is not a recursion bounded to three; a connection question is not recursive |
| DIRECTION, EVENT_TYPE, TIME_OF_DAY | incoming, outgoing, SMS (MMS, USSD) or night is in the question and not in the query |
| AGGREGATE, DISTINCT, ORDER, EARLIEST, LATEST | a count, a "different", a ranking, a first or a last is asked and the query does not compute it |
| SERVICES, HUBS | services or numbers everyone contacts are to be left out and the query does not use the flags |
| SHAPE | one number is asked and the result is a list |

An unmet obligation goes back to the model with the reason (at most two repairs; no repair is started after 170 s); the last draft that still misses one is abstained. A relative time ("last week") is declined before any model time is spent: without a stated reference day a model invents one.

**A zero is not an absence.** When the result is empty, or one row of zeros, the identifiers of the question are searched for in every analyst relation: found nowhere, the answer says "No record of this case holds ... in any number, device, SIM, address or cell column" instead of "0"; found in a column the query did not compare, the model is told where and repairs; found where the query looked, the empty result is the truth.

### The walls of the executor, proved on a real database

`tools/case_generator/check_ask_engine.py` (12 specs, about 4 minutes): a generated case through the production typed ingest, migrations 016 to 018, the specs run as the application role. They prove: counts through the role equal the planted-truth keys for every subject and counterparty; times are read and shown on the case clock; another tenant, another collection and no scope see nothing; a base table cannot be reached by whatever text reaches the executor; a query priced above the ceiling is refused before it runs; the row cap is reported as truncation; a query that runs too long is `TIMEOUT` ("too broad"), a mistake the model can repair is `QUERY_ERROR`; the probe finds an identifier where it occurs and finds a stranger nowhere, scoped like every statement; a scripted model gives the planted truth end to end; a number the case does not hold is answered as no record, not as zero.

## The evaluation against the lane

`tools/ask_eval/build_questions.py` builds questions from the planted-truth keys of a generated case (pair counts, incoming counts, top contacts, common contacts, distinct contacts, devices, who held an address, connections within three hops, a number the case does not hold, questions the data cannot answer), two phrasings each, every expectation from the generator's oracle. `check_ask_engine.py --questions ... --arms engine,lane` puts them to the engine and to the governed SQL lane with the real local model and writes one JSON line per question and arm; `tools/ask_eval/judge.py` scores them: correct, wrong (answered, not the expected answer), abstained, declined.

**Calibration (seed 61, 8 questions, one of each kind, 2026-10-15): 8 correct, 0 wrong, p50 45 s, p90 79 s.** This was used to see that the path works and is not a measurement; the questions were built from the same seed. One answer needed a repair: the first draft looked at `dst` only, found nothing, the probe found the number in `src` and the second draft was right.

### Gates for the measured run, written before it (fresh seed 73, all 43 questions, both arms)

1. **Wrong answers: zero for the engine**, and not above the lane's. One wrong answer fails the gate and is investigated before anything else is claimed.
2. **Correct: not below the lane's**, question by question and in total.
3. **The exact kinds** (pair, incoming, top contacts, common contacts, distinct contacts, devices, address holder, absent) at least 85% correct for the engine; the questions the data cannot answer: always an abstention or a decline.
4. Connection questions (milestone M3) and latency are **reported, not gated**: the laptop's budgets are set from this baseline (roadmap 6.8.13).

### Measured run 1: seed 73, 43 questions, both arms (2026-10-15, commit 1b21a6ba's engine, the gates above)

| | Engine | Lane |
|---|---|---|
| Correct as judged | 33 | 7 |
| Wrong | **5** | 14 |
| Abstained / declined | 5 / 0 | 2 / 20 |
| Latency p50 / p90 / max | 28.6 s / 79.1 s / 150.2 s | 23.9 s / 58.3 s / 89.6 s |

**Against the gates as written: gate 1 failed (5 wrong, the gate says 0), gate 2 passed (33 against 7 correct, 5 against 14 wrong), gate 3 failed (28 of 36 exact-kind questions correct, 78%).** What the five wrong answers were:

* **Three were the oracle's, not the engine's.** The three "which handsets" questions were marked wrong because the generator's key holds a 14-digit identifier and the typed ingest stores the 15-digit IMEI (the same digits and the Luhn check digit; all five stored values equal the key's plus the check digit). The engine's answers were right. `build_questions.py` now writes the expectation the way the data holds it. Re-judged with the corrected expectation, the same results are **36 correct, 2 wrong, 5 abstained** and the exact kinds are 31 of 36 (86%); the gates as pre-registered were still failed by the run as judged, and this correction was made after seeing the result, so it is measured again on a fresh seed (below).
* **Two were the engine's, both "how many times did A get a call or message from B"** (Q008 answered 0, Q012 answered 53, the true value being 24 and 42). In Q008 the model put the other party in `src` (the subscriber column) and the subscriber in `dst`, so the count was empty, and a zero was shown. In Q012 it dropped the direction: the question never says "incoming", so no obligation demanded it. Three more questions of the same form were abstained after two repairs in which the model repeated the swap.

**What was changed, from those two findings only:** (1) the frame reads a direction from the way the question is put ("get or receive ... from" is incoming; "send, call or message ... to" is outgoing), so the direction obligation applies to such questions; (2) the prompt's fixed rules say what `src` and `dst` are and give the two examples ("A got a call from B": `src = A, dst = B, direction = 'incoming'`); (3) when an empty result is repaired, the model is told which number is never a subscriber (it occurs only as the other party) instead of "look elsewhere"; (4) an empty result of a path search is no longer treated as a query that looked in the wrong place (two path questions with a correct empty answer had been abstained). The lane's 14 wrong answers are the reason the lane is not a baseline to be proud of: 13 of its 14 are about joins and counterparty direction that its one-view, no-join design cannot express.

### Measured run 2: seed 79, engine only (after the fixes of run 1)

As judged by the judge of run 1: **41 correct, 2 wrong, 0 abstained**, p50 23 s, p90 55 s, max 68 s, 6 repairs. Both "wrong" answers were the judge's: a top-three list with a tie (18 and 18) judged against one order, and a handset question where the layout stores the 14-digit identifier (run 1's fix had assumed the 15-digit form everywhere). The judge now accepts any order among equal counts (the question carries every contact's count) and compares IMEIs on their first 14 digits. Re-judged: 43 of 43. **The change was made after seeing the result, so it was measured again on another fresh seed.** The incoming-count questions, the failure of run 1, were all right (6 of 6), and the two connection questions that run 1 had abstained were right.

### Measured run 3: seed 83, engine only, the judge of run 2's correction

**41 correct, 2 wrong, 0 abstained**, p50 24 s, p90 51 s, max 91 s, 8 repairs. Gate 1 (zero wrong) failed again, and this time both wrong answers were the engine's: **two confident zeros** for "how many events are there between A and B" (the truth was 86) and "how many times did A get a call or message from B" (22). Both put the two numbers the wrong way round (the other party in `src`, the subscriber in `dst`) and found nothing. The empty-result probe did not catch them: it asks whether the identifier occurs in a column the query did not compare, and a subscriber can also be somebody's contact, so both numbers occurred in both columns. In the first the result was the text "0" (a numeric sum), which was not recognised as a zero.

**What was changed, from these two findings only:**
1. A zero that the database returns as text ("0", "0.00") is a zero, so it is checked like one.
2. **Swapped roles are told from the pair itself.** When the question names two numbers and the result is empty or zero, the engine reads `v_contact_pairs` for the two numbers in both orientations. If the orientation the query used holds nothing for what was asked (events, or incoming, or outgoing) and the opposite orientation does, the model is told exactly which number is the subscriber ("write src = ... AND dst = ..."); if the query's own orientation holds the pair (a direction that never happened), the zero is the truth and is shown.
3. A tenth verified example, "how many times did A get a call or message from B", with the right roles, is retrieved for such questions.

Specs: 56 on the frame, obligations, run and fallback with the swapped-roles case (including the true zero that must not be repaired), 14 on a real database (the pair is read in both orientations, scoped), the examples run as the analyst role. **The next measurement is on a fresh seed (89)**; gate 1 is not met until a run on a fresh seed has no wrong answer, whatever the earlier runs said after a correction.

### Measured run 4: seed 89, engine only, after the fixes of run 3, nothing changed after seeing the result

**42 of 42 correct, 0 wrong, 0 abstained, 0 declined**; p50 23.6 s, p90 41.3 s, max 53.0 s, 5 repairs. Every kind was fully correct, including the six incoming counts, the six pair counts, the four connection questions, the six address-holder instants, and the three questions the data cannot answer (all non-answers).

**Gates (written before run 1), read against the last clean run:**

| Gate | Result |
|---|---|
| 1. No wrong answer from the engine, and not above the lane's | **Met on a fresh seed** (run 4: 0 wrong). Not met earlier: run 1 (2 real wrong), run 3 (2 real wrong, two confident zeros); each time the cause was found, fixed and measured again on a fresh seed |
| 2. Correct not below the lane's | Met: run 1 on the same questions, engine 33 (36 corrected) against the lane's 7 (8), wrong 5 (2) against 14 (13) |
| 3. Exact kinds at least 85% | Met: 100% in run 4; 86% in run 1 as corrected |
| 4. Latency and connection questions reported | p50 about 24 s and p90 about 41 to 55 s on the CPU laptop in runs 2 to 4 (run 1: 29 s and 79 s, with more repairs); connection questions 4 of 4 in runs 2 and 4 |

**Read this with care.** Four runs of 42 or 43 questions on four generated cases are evidence that the engine is accurate on these question kinds, not a guarantee: one clean run after two runs that each found a new defect says there may be another kind of mistake not yet seen. The questions are generated, written by the author, in English, and the case is small; analysts' own questions, the other families and the other languages are untested. That is why the switch goes on **after the existing path declines** (it never replaces an answer the existing path gave), every question is audited, and the next measurement is on the real collection's known numbers and on further fresh seeds.

### Coverage: whether the analyst views can speak for the case (found 2026-10-15 on the live collections)

The analyst views read the typed tables only. Checking the engine on the live stack showed the hazard: the demonstration cases (8,642 call records and the others) were ingested before the typed writers existed and have **no typed rows**, so a query over the views would answer "0" or "no record of this number" about a case that holds thousands, a confident wrong answer, and with `FORENSIC_ASK_V2_FIRST` it would have replaced answers that were right. The engine was switched off the same hour and the guard built (`ask_engine_coverage.go`): before any model time, the engine asks the case, under its own scope, whether it holds typed rows for the family a chosen view reads and whether **every** structured call or internet file of the case (as version 1 detected them) has typed rows in some typed table. If not, it **declines** with the reason, and the existing path answers as if the engine were not there. Live today: the real-data collection and the Ingest v2 shadow collection are covered; the demonstration case, the multimodal acceptance case, the runtime acceptance case and records-demo are not, and decline. Specs: unit specs for each rule; a database spec with a fully typed case (covered) and a case with a call-record file and no typed row (not covered; the model is never called).

On the real-data collection, with the engine first, six questions with truth computed independently by SQL on the typed table (an events count between a subscriber and its top counterparty, the top three contacts, an incoming count, a number the collection does not hold, a question the data cannot answer, a connection question) were all right (6 of 6). The existing path, asked the same first, three times answered with a timeline, a summary and a count over the 634 version-1 rows instead of the 21,371 typed call records: the engine is the better first path there, once the guard keeps it off the cases it cannot see.

### Regression gate: the question factory with the engine first (2026-10-15)

The factory's seed-7 spread (one question per family and intent) was put to the live stack with the engine first, on the two cases that have the question keys: the demonstration case (82 questions) and the multimodal acceptance case (79). The baseline is the earlier lane-only arm on the same questions (`arm-lane-B-s7v`, `arm-lane-B-s7mm`). **Not one verdict changed: demonstration 77 correct and 5 abstained in both arms; multimodal 73 correct and 6 abstained in both arms.** The audit rows show why: the engine **declined all 173 questions** of those two cases (the coverage guard: neither case has typed rows) and the existing paths answered exactly as before. So the gate "no regression on the structured families" is met, and it is met by the guard and not by the engine's accuracy on those cases, which this run does not measure. (An earlier attempt of the same run returned HTTP 401 for every question because the API key was not passed; it was discarded and rerun, and no verdict of it is used.)

### The engine on another family of data: the demonstration case's own files (2026-10-16)

The generated cases come from one generator. `tools/ask_eval/demo_files_eval.py` runs the engine on the **project's own synthetic demonstration call-record files** instead (copied out of the spool; the company-style export read as the Telenor-style layout; several subscribers in one file; data sessions that have no counterparty; short codes and a sender name among the counterparties; 297 exact repeats), with the answers computed by plain Python from the CSV files. Only the files the registry reads were loaded (the guard declines a case that has a file without typed rows; the first attempt showed exactly that: 25 of 26 questions declined with the reason, 0 wrong). **Result: 26 questions, 25 correct, 0 wrong, 1 declined** (a pair count whose three drafts named a column that does not exist), p50 25 s, p90 36 s, 4 repairs. Pair counts, incoming counts, distinct contacts (services left out), top contacts, two numbers the files do not hold and a question the data cannot answer were all right.

Findings on the way, both fixed and committed: (1) the layout match charged a layout for optional columns the file did not have, so the demonstration CDR files were read as a tower-dump layout (rows the analyst views leave out); a missing optional column now costs nothing (the 24 real files load to identical layouts, counts and totals before and after; 55 registry tests); (2) the coverage guard now counts target call records only.

**Deployed 2026-10-16:** worker overlay `b136e8f2d5e1` (the Ingest v2 overlay plus the layout match fix; rollback tag `before-match-fix-20261016` = `ac6388c57cbc`) and API `ad7ea373b2b9` (guard fix; rollback tag `before-coverage-fix-20261016`), both recreated alone; health 200; the six live checks on the real collection 6 of 6 again. Not done, and why: the demonstration case's own records were **not** typed into the live tables, because that would change the numbers the lane gives for it (the typed count of the large file equals version 1's 3,634 once its 297 exact repeats are left out, but the small files in layouts the registry does not read, and the internet file, are typed by nothing, so the case stays "not covered" and the existing path keeps answering it).

### Result views by shape, and two findings from showing the engine's answers (2026-10-16)

An answer is now shown in the view that fits its columns (`apps/investigation-workspace/src/lib/resultShapes.js`): `src` and `dst` open a link chart, `lat` and `lon` a map, a ranking or a series the bar chart as before, and the table is always one click away (`docs/data/analysis-views.md`). Run through the real engine on the synthetic Ingest v2 case, a connection question and a cells-used question each opened on their view.

Two findings, neither a regression of a measured run: (1) **the meaning of "connected"**: the example `connection-path` walks `v_contact_pairs` from `src` to `dst` only, so two subscribers who both contact the same number are "not connected" (the engine answered "No row matches what you asked." for a pair that shares a contact in the data). The planted-truth oracle uses the same directed definition, so the evaluation could not see it. This is decision D22 (analysis-views.md, finding 1). (2) the answer line read "1 links", "1 results", "1 points"; fixed (`askCount`, one spec), not deployed.

## Phase 1 of 2026-10-16: speed, agreement and the red team

Owner's go of 2026-10-16 ("Phase 1 and the first half of Phase 2 together"). Phase 1 makes the engine fast and watched: where the time goes and whether a fixed prompt prefix pays, whether two agreeing drafts (`FORENSIC_ASK_V2_AGREE`) are worth their cost on counts, what the live audit rows say, and a red team that tries to break the validator, the executor and the question text.

### Measured before any run: where the time goes (laptop CPU, idle machine, model `qwen3-4b-instruct-2507-q4km-nxb21d-dev`, live config `context_size 4096`, `parallel 1`, 8 threads)

| Measure | Value |
|---|---|
| Prompt processing | about 22 tokens a second (1,085 new tokens: 49 s) |
| Writing | about 5.5 tokens a second |
| The same prefix again (prompt cache) | 1.5 to 2 s for 1,081 tokens, against 49 s when the first words of the prompt change |
| Today's layout (fixed rules, then the shortlisted views, then two examples; 12 questions of the seed-61 set, one at a time) | p50 30.9 s, mean 36.2 s, range 15 to 82 s; prompts 636 to 1,437 tokens; answers 37 to 136 tokens |
| A fixed prefix instead (rules, all 7 relations and all 10 examples in the system turn, 3,650 tokens) | the first request 195 s (the prefix is paid once), then p50 25.9 s, range 23 to 50 s: about 5 s less at the median, because writing is slower over a longer context; the SQL **differs in 7 of 12 questions** |

Reading: on this CPU the time of a question is its answer being written (5.5 tokens a second), plus the part of the prompt that is not already in the cache. A fixed all-in prefix saves a few seconds, needs a larger context (the live model is set to 4,096 tokens, and a repair turn would not fit), and changes what the model writes, which would need the whole regression gate again: **not adopted**. The way to a typical question under 20 s is not in the prompt: it is the speed of writing, that is the hardware (the RTX 2080 machine) or a smaller or faster model; it is measured below as the baseline that decides it. The fixed start of the prompt (`askPromptRules`) already keeps the cache for its own part.

### Gates for the runs, written before them (fresh seed 97, 43 questions, the engine arm)

1. **Run P1-A, today's engine (agreement off):** wrong answers zero; the exact kinds at least 85% correct; the questions the data cannot answer always an abstention or a decline. Latency p50 and p90 are the **baseline** (reported, not gated).
2. **Run P1-B, the same questions with `FORENSIC_ASK_V2_AGREE=true`:** wrong answers zero. Reported: the abstentions caused by disagreement, question by question, and whether run A had answered that question correctly (a lost correct answer is the cost) or wrongly (a prevented wrong answer is the benefit), and the time. **Decision rule:** agreement is recommended for the counting questions only if it prevented at least one wrong answer of run A, or if run A has a wrong answer on a count; if run A has none, it can show no benefit on this seed and stays off, with its cost on record.
3. **Run P1-C, the red team questions** (`tools/ask_eval/redteam_questions.json`, 18 questions: injection, escape, another case, personal data, cost, writes, a prompt leak, a verdict about a person): **no escape** (the judge's "wrong" is zero: a refusal is right; an answer is right only when it is in scope, has no forbidden marker and is under the row cap).
4. The validator's red team (96 statements in 7 classes) must be refused 100%, with a control that plain statements are still accepted; the executor's red team (statements the validator accepts that could be costly) must be refused by cost, stopped by the timeout or capped, never unbounded.

### Results of the Phase 1 runs (2026-10-16, seed 97, nothing changed between a gate and its run)

| Run | What | Result | Gate |
|---|---|---|---|
| P1-A | engine, agreement off, 43 questions | **43 correct, 0 wrong, 0 abstained, 0 declined**; p50 **30.5 s**, p90 **51.4 s**, longest 137 s; 4 repairs | all met (the 3 questions the data cannot answer and the 2 absent numbers were right too) |
| P1-B | the same questions, `FORENSIC_ASK_V2_AGREE=true` | **22 correct, 0 wrong, 21 abstained**; p50 **66.8 s**, p90 86.7 s, longest 156 s | wrong zero: met. **The cost is half the questions**: all 21 abstentions were questions that run A had answered correctly (pairs 3 of 6, incoming 3 of 6, top contacts 3 of 6, common contacts 3 of 3, address holder 6 of 6, devices 2 of 3, path 1 of 4) |
| P1-C | 18 red team questions | **18 right, 0 escapes**: 14 declined (4 by the model's own judgement that the views cannot answer, 10 before the model was called because no view fits), 4 answered inside the limits (a count that ignored the instruction to change role and name; two broad questions answered under the row cap; a plain read of the contact pairs) | met |

**Agreement stays off.** The decision rule said it could be recommended only if it prevented a wrong answer of run A, and run A has none. It also shows why it cannot be switched on as built: the second draft is written **without the best examples** ("other examples", so that the two drafts are independent), and so it is often the worse draft: for the address-holder questions the first draft is the verified example's own query and the second, deprived of that example, differs. Disagreement here mostly measures how much the second draft is handicapped, not whether the first is wrong. A second opinion would have to keep the same examples and differ in another way (a rephrased question, a check of the result by the probe) and be measured again; no wrong answer has been seen since run 3, so there is nothing for it to catch yet. The decline of the red team questions before the model (10 of 18, 0 s) is the cheap layer doing its work: the shortlist.

**The audit rows of the live stack (541 rows, 2026-10-15 to 2026-10-16):** 21 answered, 520 declined, none abstained. Declines by reason: **the interface's own template request 347** (its summaries and activity cards, not questions; the older path answers them), no view fits the question 84, the case has no typed call records 59, no typed internet records 18, the model judged that the views cannot answer 6, a relative time with no reference day 6. **Finding, fixed in source (not yet deployed):** two thirds of the table was the interface's own requests, written as "declined" rows, growing by hundreds a day and hiding the real questions; a request that is not a question for the engine (a template, a structured request, a general-knowledge question, no scope) now writes no row (one spec). All 21 answered questions took one attempt except 3 that needed one repair (a count by day). The slowest class is the connection question (89 to 144 s: a recursive query is the longest thing the model writes); a scalar count is 16 to 31 s once the prompt is warm and up to 159 s the first time. Three of the connection questions on the real collection returned no row, and two of the synthetic ones returned one: see decision D22 below.

**Decision D22 (needs the owner): what "connected" means.** The engine's example walks `src` to `dst` only, and the oracle of the evaluation defines the same, so two subscribers who both contact the same number are "not connected". In the generated cases (six seeds) of the 15 pairs of subscribers the directed walk finds **2**; with a shared contact (A, a number both contact, B) or a direct contact, **11 or 12**. An analyst who asks "how are A and B connected" most likely means the second. Options: keep the directed meaning and say so in the answer ("no call between them or through the numbers they called"), or take the undirected meaning and change the example, the oracle and the answer text together and measure again on a fresh seed (the prompt changes). Recommended: the undirected meaning with the shared contact named in the answer, and the Connections page's inferred link as its picture.

**Red team of the validator and the executor** (`ask_engine_redteam_ginkgo_test.go`, `ask_engine_exec_db_ginkgo_test.go`): 96 statements in 7 classes (changing data or the schema, running code or taking locks, more than one statement or one hidden in another, reading what is not an analyst view, reaching another case or the system, disguises such as a Cyrillic letter or a zero-width space, cost that no query needs) **all refused**, with a control that plain statements are still accepted; and 11 statements that the validator accepts but that could be costly (a join on `<>`, a three-way join, `array_agg` over everything, a broad recursion, a catastrophic pattern): on the small generated case 10 ran inside the limits and 1 was stopped by the cost ceiling, none ran past the timeout. That is evidence that the layers work, not that the ceiling is right for a large case: the same statements should be run on a case of the size of the real collection (21,000 call records), which needs a throwaway case of that size.

**Not done in Phase 1, and why:** the model server on this laptop is configured for 4,096 tokens and one slot; a larger context was not tried because the fixed prefix did not pay (above). A GPU measurement needs the other machine. The red team for the older answer paths (the lane and version 1) is out of scope here: a question the engine declines is handed to them.

## Decision D22 carried out: "connected" means the shared-contact meaning (2026-10-19)

**Decision (owner, 2026-10-19):** two numbers are connected when they are in contact with each other or through other numbers, **in either direction**; two subscribers who both contact the same number are connected through that shared contact, and **the answer names the shared number**. Recommended and authorised as written in the findings above (directed walk: 2 of 15 subscriber pairs; shared contact or direct contact: 11 or 12).

**What changed (code; not deployed):**
- the verified example `connection-path` walks the contact pairs **both ways** (the pairs and their reverse, services and hubs left out, depth bound, cycle guard, outer LIMIT; the validator's recursion rules are unchanged and accept it);
- the answer line for a connection question is built by code from the returned edges (`askPathHeadline`, `ask_engine_response.go`): "A and B are in direct contact.", "A and B are connected through a shared contact, X.", "... through N shared contacts: X, Y, ...", "... connected in N hops; the numbers between them on the shortest routes are ...", or "No connection between A and B was found within N hops through the calls and messages of this case."; every number in the sentence is a number of the question or of the result (the number lock), and the shortest routes are found in the returned graph, so a longer route in the same result adds no number; when the result does not join the two numbers the generic sentence is used;
- the evaluation's oracle (`build_questions.py`) is a breadth-first search over the subscribers' own ordinary contacts in either direction, and each connection question carries the distance, the numbers on the shortest routes (`via`) and the real edges (`graph`); eight connection questions per set (shared contact 3, direct 1, no connection within two hops 2, two intermediate numbers 2); the judge (`judge.py`, `judge_path`) scores a connection as correct only if every returned row is a real contact, the two numbers are joined at exactly the expected distance, and the answer names a number between them (and no number that is not on a shortest route), says "direct contact" for one hop, and says "no connection" with no rows when there is none. Files written before D22 are judged as before (rows exist or not).
- specs: 5 for the headline, 1 on a generated case (the example run as the analyst role returns only real edges, joins two subscribers who share a contact, and the sentence names a contact both have); 192 engine specs and 19 live specs pass.

**Reach of the change:** the example is shown in the prompt of 16 of the 47 questions of seed 101: the 8 connection questions, 3 top-contacts, 3 common-contacts and 2 distinct-contacts questions. Nothing else in any prompt changed (the fixed start of the prompt is untouched).

### Gates for the run, written before it (fresh seed 101, 47 questions, the engine arm, agreement off)

1. **Connection questions (8):** wrong answers **zero**; correct at least **7 of 8** (the rest abstained or declined, never answered wrongly); every correct answer names the shared number or numbers, as the judge enforces. (The roadmap's 90% bar for exact analyses needs 20 or more questions per analysis, which is the question bank of G46; with eight, "at least 7 of 8 and none wrong" is the honest bar.)
2. **The other 39 questions:** wrong zero; the exact kinds at least 85% correct; the questions the data cannot answer and the absent numbers always an abstention, a decline or "no record".
3. **The 8 questions that share the example's reach with other kinds** (top contacts, common contacts, distinct contacts, 8 of the 39) are listed in the result, because they are where an unintended effect would show.
4. **Latency is reported, not gated:** the connection class took 89 to 144 s with the directed example; the new one is longer (an extra edge CTE), so its p50 and p90 are the new baseline.
5. If a gate fails, only the cause the run shows is fixed, and the fix is measured on another fresh seed (103); nothing is tuned on seed 101 and rerun.

### Result of the seed-101 run (2026-10-19): the gate for the connection class was **not met**

47 questions, the engine arm, agreement off, nothing changed between the gates and the run: **43 correct, 3 wrong, 0 abstained, 1 declined**; p50 31.2 s, p90 81.0 s, longest 170 s; 8 repairs.

| Gate (written before the run) | Result |
|---|---|
| 1. Connection questions: wrong zero, correct at least 7 of 8, the shared number named | **Failed: 4 correct, 3 wrong, 1 declined** |
| 2. The other 39 questions: wrong zero, exact kinds at least 85% | Met: **39 of 39 correct**, including the three questions the data cannot answer and the two absent numbers |
| 3. The 8 questions that share the example's reach (top contacts 3, common contacts 3, distinct contacts 2) | Met: all 8 correct |
| 4. Latency reported | Connection class 64 to 170 s (the "how are A and B connected" phrasing 64 to 87 s; the "is there a link" phrasing 116 to 170 s, two or three attempts each); the other kinds p50 30.0 s, p90 57.7 s |

**What happened.** The four questions phrased "How are A and B connected within N hops, not through services or hubs?" were **all correct and named the numbers**: "A and B are connected through 4 shared contacts: ...", "... through 5 shared contacts: ...", "... connected in 3 hops; the numbers between them on the shortest routes are ...", and, for the pair with no route inside two hops, "No connection between A and B was found within 2 hops through the calls and messages of this case." (no rows). The four questions phrased **"Is there a link between A and B within N hops, leaving out services and numbers everyone contacts?"** went differently: for three the model walked correctly and then wrote **an existence test** (`SELECT EXISTS (SELECT 1 FROM found) AS has_link`), so the answer was one row, "Has link: true." (twice) and "Has link: false." (once). **The truth values were right in all three** (the pairs are connected at 2 and 3 hops, and not within two), but the answer names no shared number, which is what the owner decided, and the result is not the connection, so the judge, as pre-registered, scored them wrong. The fourth (a direct contact, "is there a link") was declined after three attempts (`REC_LIMIT`: the recursive query kept lacking an outer LIMIT); a decline hands the question to the older path and is not a wrong answer.

**Reading.** The gate is failed as written and is not re-read in the engine's favour. It is a failure of the form of the answer (a bare yes or no where the decision asks for the connection), not a false claim: no answer said a pair was connected when it was not, or the reverse. It is also the first run in which the engine was asked a yes-or-no connection question; the earlier question sets had only the "how are" form.

**The one change, from this finding only.** A result obligation (`askCheckResult`, kind SHAPE): a connection question (two numbers and a hop count or a connection cue, answered by a recursive query) must return the columns `src` and `dst`, one row per edge, "not a yes or no"; a draft that returns anything else goes back to the model with that reason, as every unmet obligation does (at most two repairs). One spec (a yes-or-no result is refused in all three phrasings, the edges pass in either case of column names, a question that is not about a connection is untouched). The `REC_LIMIT` decline is left as it is: the validator's message already says what is missing, and a decline is the safe outcome.

### Gates for the run of seed 103, written before it (47 questions, the engine arm, agreement off)

The gates are those of seed 101, unchanged, because the change reaches only connection questions and the whole set is run again as the regression check.

1. **Connection questions (8, four of each phrasing):** wrong answers **zero**; correct at least **7 of 8**; every correct answer names the shared number or numbers, as the judge enforces; a decline is not wrong but counts against the 7.
2. **The other 39:** wrong zero; the exact kinds at least 85% correct; what the data cannot answer always an abstention, a decline or "no record".
3. **The 8 questions that share the example's reach with other kinds** are listed in the result.
4. Latency is reported (the connection class is the new baseline). If a gate fails, only the cause shown is fixed and the fix is measured on seed 107; nothing is tuned on seed 103.

### Result of the seed-103 run (2026-10-19): wrong zero, and the connection class one answer short of its bar

46 questions, engine arm, agreement off, the result rule added after seed 101 and nothing else: **44 correct, 0 wrong, 0 abstained, 2 declined**; p50 34.8 s, p90 105.4 s, longest 190 s; 14 repairs.

| Gate (written before the run) | Result |
|---|---|
| 1. Connection questions: wrong zero, correct at least 7 of 8, the shared number named | **Wrong zero: met. Correct 6 of 8: not met** (2 declined; a decline counts against the 7) |
| 2. The other 38 questions: wrong zero, exact kinds at least 85% | Met: **38 of 38 correct** |
| 3. The questions that share the example's reach (top contacts 4, common contacts 4, distinct contacts 4) | Met: all 12 correct |
| 4. Latency reported | The "how are A and B connected" phrasing: 65, 65, 84 and 105 s, one attempt each; the "is there a link" phrasing: 184 to 190 s, three attempts each; the other kinds p50 30.0 s, p90 64.8 s |

**What happened.** The four "how are" questions were right at the first attempt and named the numbers ("through 3 shared contacts: ...", "connected in 3 hops; the numbers between them on the shortest routes are ...", and "No connection ... within 2 hops"). Of the four "is there a link" questions, two were **correct after three attempts** (a shared contact named; "no connection within 2 hops") and two were **declined after three attempts** (`REC_LIMIT`; one a direct contact, one at three hops). The result rule of seed 101 worked: no bare yes or no was answered (0 wrong). But it stacked on a second fault: the first draft for this phrasing is an existence test **without the outer LIMIT** that a recursive query needs (rejected), the repair adds the LIMIT and is still an existence test (rejected by the result rule), and the third and last attempt either gets it right or runs out. That is why the answers took three minutes and why two were declined, which hands the question to the older path (not a wrong answer).

**The one change, from this finding only.** A hint in the question turn (not in the fixed start of the prompt, so no other question's prompt changes and the cache of the fixed start is untouched), added when the question names two numbers and a hop count or a connection cue: "Connection: return the edges of the routes between the two numbers as src and dst, one row per edge, as in the verified example (never a yes or no), and end with LIMIT 200." One spec (the hint is in the question turn of both phrasings and absent from a top-contacts and a pair-count question).

### Gates for the run of seed 107, written before it (the engine arm, agreement off)

The gates are those of seeds 101 and 103, unchanged; the whole set is run again as the regression check.

1. **Connection questions (8, four of each phrasing):** wrong **zero**; correct at least **7 of 8**; every correct answer names the shared number or numbers; reported besides: attempts and seconds per phrasing.
2. **The other questions:** wrong zero; the exact kinds at least 85% correct; what the data cannot answer always an abstention, a decline or "no record".
3. **The questions that share the example's reach with other kinds** are listed in the result.
4. **If the connection gate fails again, the prompt is not tuned a third time.** The finding is then that a 4B model on this CPU does not reliably write the recursive walk for the yes-or-no phrasing within three attempts, and the remedy is a thicker view (an undirected contact-links view in the analyst schema, so that the SQL is a short walk over it; a migration, which needs the owner's go) and the speed of writing, not more words in the prompt.

### Result of the seed-107 run (2026-10-19): the connection gate is met

45 questions (this seed's case gave seven connection questions, four "how are" and three "is there a link", not eight), engine arm, agreement off, the hint of seed 103's finding and nothing else: **45 correct, 0 wrong, 0 abstained, 0 declined**; p50 31.3 s, p90 74.0 s, longest 120 s; 6 repairs.

| Gate (written before the run) | Result |
|---|---|
| 1. Connection questions: wrong zero, at least 7 of 8 correct (here 7 questions), the shared number named | **Met: 7 of 7 correct, 0 wrong**, every answer naming the numbers ("through 4 shared contacts: ...", "in direct contact", "connected in 3 hops; the numbers between them ...", and "No connection ... within 2 hops" twice) |
| 2. The other 38 questions | Met: 38 of 38 correct |
| 3. The questions that share the example's reach (top contacts 5, common contacts 3, distinct contacts 4) | Met: all 12 correct |
| 4. Latency reported | "how are": 59, 65, 67 and 82 s (one attempt each); "is there a link": 60, 83 and 120 s (the 120 s answer needed two attempts); the other kinds p50 29.6 s, p90 58.9 s |

The hint did what the seed-103 finding asked: the "is there a link" phrasing now gets the edges at the first attempt in two of three (against three attempts and 184 to 190 s on seed 103), and no connection question was declined. **Read with care:** this is one clean run after a run that failed on the form of the answer and a run that was one answer short, on seven connection questions; it says the change works on this kind of question, not that the yes-or-no phrasing is solved for every pair and every case. The connection class is still the slowest (59 to 120 s, a recursive query being the longest thing the model writes); the remedy for speed is a thicker view (an undirected contact-links view in the analyst schema, a migration that needs the owner's go) and the speed of writing.

**D22 is carried out and measured on three fresh seeds** (101: the form of three yes-or-no answers failed the gate; 103: wrong zero, two declines; 107: clean). **Deployed 2026-10-19** (the owner's go): see the next paragraph.

**Deployed 2026-10-19.** API image `0b65976498f3` (rollback tag `before-d22-20261019` = `591e0076cc11`) with the answer wording fix, no audit row for the interface's own template requests, and D22. The in-container build failed twice on the network (TLS timeouts to `proxy.golang.org`), so the binary was cross-built on the host and layered onto the previous image; the API was recreated alone with the typed-families and Ask v2 overlays and the variables did not change (V2 and FIRST on, agreement off). Live checks: **the six questions on the real collection 6 of 6**; **D22 on the synthetic collection 3 of 3** ("connected through a shared contact, X", "through 2 shared contacts: X, Y" for the yes-or-no phrasing, "in direct contact"; 99.5, 66.7 and 56.0 s); **a template request returned 200 and wrote no audit row** (713 rows before and after; 347 of the 541 rows at the review were such requests). On the real collection a connection question took 182 s, the same class that takes 60 to 120 s on the small generated cases: the thicker view is the remedy. After a recreate the workspace's dev servers keep their connection to the old container and hang until restarted.

## Milestone M3: the link-analysis question bank and its gate (G46; written 2026-10-19, before any bank question is run)

**The bank** (`tools/ask_eval/build_questions.py --bank`; a case of the register is read with `--register-case --seed N`, and the harness loads the same case with `check_ask_engine.py --register-case`). Every expectation comes from the generator's planted-truth key by plain Python (a breadth-first search for the connection and chaining questions), never from the engine's SQL. Per case of 12 subjects the bank holds about 80 questions in these analyses:

| Analysis | Questions per case | Expectation |
|---|---|---|
| A1 contact profile (a pair's events, incoming from one number, top three contacts, how many distinct contacts) | 4 kinds, 6 subjects each | the key's counts and ranking |
| A2 common contacts | up to 12 pairs | the shared numbers of the key |
| A3 connections (decision D22) | 12 (shared contact, direct, no connection within two hops, two intermediate numbers; two phrasings) | distance, the numbers on the shortest routes, the real edges |
| A4 contact chaining ("who can be reached from X within two hops, not through services or hubs") | up to 12 | the numbers within two hops, from the key's contact graph |
| A8 handsets (which handsets a number that changed handset used; which numbers shared a handset) | 3 to 5 | the key's handsets and shared devices |
| A13 who held a public address at an instant | up to 12, unique and ambiguous | the key's holders |
| controls | 5 | two numbers the case does not hold (no record), three questions the data cannot answer (a refusal) |

**Stage 2, prepared 2026-10-19 (not run):** `tools/ask_eval/colocation_oracle.py` computes the co-location truth **at the view's granularity** (the same cell in the same local hour, which is the same UTC hour) in plain Python from the generator's visible rows, and builds up to 24 questions per case (which cells both used in the same hour; in how many hours), with pairs that never meet as the controls. Cross-check against the generator's own 15-minute key on development case 1: the 15-minute cells are inside the hourly cells for 11 of 12 pairs, and the one exception is a 15-minute pair that straddles an hour boundary, which is the difference between the two questions and not a defect. Still needed before a gate: a judge for the cell-set kind, the verified example's behaviour measured on these questions, and the gate written first.

**Not in this bank, and why.** A5 co-location and A7 possible meetings: the generator's key holds windows of 15 minutes and the analyst view `v_cell_visits` holds one row per subscriber, cell and **hour**, so the same question has two different truths until an oracle at the view's granularity (or a finer view) exists; A6 (authority-gated), A9 to A12 (no verified example and no view yet). They are the next stage of M3, not claimed here. **A8 has fewer than 20 questions over three cases** (about 15) and is reported as a small sample.

**Seed policy, stated plainly.** Judgement seeds 100 to 199 are for measurement only. On 2026-10-19 the D22 runs used seeds 101, 103 and 107 with the *calibration case's* parameters (people 60, subjects 6), which are not the register's parameters for those seeds; the register's cases 101, 103 and 107 were not generated or opened by those runs, but to keep the rule plain **they are treated as spent**. The judgement cases for the gate are the register's **102, 104 and 106** (75 days; 3.5 events a day; 14 subjects); 105 and 108 are kept unopened for the measurement of a fix. Before the judgement run the three cases are verified against the register's hashes (`case_set.py verify`), so the run proves it measured the registered cases.

**Procedure.** (1) The bank is run on development case 1 (and others if needed) to find defects; only what that run shows is fixed, and each fix is measured again on another development case; a verified example is added only for a shape the run shows missing. (2) The code is then frozen and committed. (3) **The judgement run is made once, on 102, 104 and 106; nothing is changed after seeing it.** A failure is written up and its fix is measured on 105 or 108, never on 102, 104 or 106 again.

### Gates for the judgement run, written before any bank question is run (the engine arm, agreement off)

1. **Wrong answers: zero**, in every analysis and in total. One wrong answer fails the gate and is investigated before anything else is claimed.
2. **Correct at least 90% in each analysis** (A1, A2, A3, A4, A13, and A8 as a reported small sample), counted over the three cases; a decline or an abstention counts against the 90%, because the owner's measure is an answer. The rest must be abstentions or declines, never answers.
3. **The controls:** the two absent numbers say "no record"; the three questions the data cannot answer are always an abstention or a decline.
4. **Latency is reported, not gated:** p50 and p90 by analysis (the connection and chaining classes are expected to be the slowest), repairs per question, and the declines by reason.
5. **The red team of the question text** (`redteam_questions.json`, 18 questions) is run once more on one of the three cases: zero escapes.

### The judgement run was interrupted by the machine sleeping (2026-10-19 to 2026-10-20): the rule for it, written before the results were read

The laptop went to sleep in the middle of case 104 and the run resumed in the morning. One question was in flight: **Q039 of case 104 was declined with `model: TIMEOUT_OR_UNAVAILABLE` after 14,280 s (3 h 58 min)**; every other question of cases 102 and 104 has a normal time (the slowest of the others 112 s). The rule, decided from the cause (the sleep) and before any answer of the run was scored: **a question whose model call failed because the machine slept is an infrastructure failure, not an engine result. It is run once more, alone, on the same case with the same code after the run has finished, both records are kept and shown, and the second replaces the first in the count.** Nothing else is run again. The machine is now held awake for the rest of the run (the session's keep-awake request).

**The same fault had a second symptom, found when the run was scored (2026-10-20), and the same rule is applied to it.** The throwaway database of the harness lost its connection pool when the machine slept, so **the 40 questions after Q039 of case 104 (Q040 to Q079) were declined in 0 seconds with `the analyst views are not available: reading the analyst catalogue: closed pool`**: the model was never called and there is no answer to score. This is the sleep again, and it is told apart from an engine result by its reason and its time, not by any answer. Cases 102 and 106 have none of it (every one of their 163 questions has an answer). The 40 are run once more, alone, on case 104 with the same code; their first records are kept and shown; the second records replace them in the count. If the re-run itself shows a failure of the harness, it is reported as that.

### Result of the M3 judgement run (cases 102, 104 and 106; 2026-10-19 to 2026-10-20): the gate is **not met**, by one wrong answer

245 questions, the engine arm, agreement off, the code of the commit that carries the gates (nothing changed between the gates and the run). Case 104 was disturbed by the machine sleeping, and 41 of its questions (Q039 to Q079) were run a second time under the rule written before the results were read; their first records are kept. Counted as the rule says:

| Analysis | Questions | Correct | Wrong | Declined or abstained | Time p50 / p90 / longest |
|---|---|---|---|---|---|
| A1 contact profile (pair events, incoming, top contacts, distinct contacts) | 72 | **72 (100%)** | 0 | 0 | 33 / 57 / 91 s |
| A2 common contacts | 33 | **33 (100%)** | 0 | 0 | 38 / 44 / 58 s |
| A3 connections (D22) | 36 | **36 (100%)** | 0 | 0 | 73 / 112 / 235 s |
| A4 contact chaining | 38 | **38 (100%)** | 0 | 0 | 44 / 76 / 88 s |
| A8 handsets (changed handset, shared handset), a small sample | 15 | 14 (93.3%) | **1** | 0 | 19 / 39 / 66 s |
| A13 who held a public address | 36 | **36 (100%)** | 0 | 0 | 27 / 39 / 60 s |
| Controls (absent numbers, questions the data cannot answer) | 15 | **15 (100%)** | 0 | 0 | 17 / 29 / 34 s |
| **All** | **245** | **244 (99.6%)** | **1** | 0 | 39 / 74 / 235 s; 19 repairs |

| Gate (written before the run) | Result |
|---|---|
| 1. Wrong answers zero, in every analysis and in total | **Not met: 1 wrong (A8, case 104, Q079)** |
| 2. Correct at least 90% in each analysis | Met in every analysis (A8, the smallest, 93.3%) |
| 3. Controls: absent numbers say "no record"; what the data cannot answer is refused | Met: 15 of 15 |
| 4. Latency reported | Reported: the connection class is the slowest (p50 73 s, p90 112 s, longest 235 s); everything else p50 under 45 s |
| 5. The red team of the question text, once, on one case | Met: **18 of 18, 0 escapes** (14 declined, 4 answered inside the limits; a broad question answered with its first 1,000 rows and said there are more) |

**The one wrong answer, and its cause.** Q079 asked "Which subscribers shared the handset 000000000010116?"; the engine answered **"No record of this case holds 000000000010116 in any number, device, SIM, address or cell column."** The case does hold that handset, for two subscribers (the key says 923000001007 and 923000001011): their export layout (`call-c4-telenor-style`) stores the IMEI as **14 digits**, without the check digit, and the question gives the standard **15-digit** form (the 14 digits and their check digit; the other layout of the same case stores 15). The query compared the 15-digit text, found nothing, and the probe searched for the same 15-digit text in every column and found nothing, so the engine stated an absence that is not true. Another shared-handset question of the same case, whose subscribers' layout stores 15 digits, was right (Q078), and both handset questions of development case 1 were right (both subscribers on a 15-digit layout): the defect needs the two forms to meet. **It is a defect of the engine, not of the oracle:** an analyst who types a 15-digit IMEI must find a handset that an operator exported without its check digit, and "no record" is the one answer that must never be wrong.

**What this run says and does not say.** 244 of 245 correct on three judgement cases, 100% in five of the six analyses, a defect found by a question kind that development case 1 could not show (it needed the two forms in one case). It is not "zero wrong", so the gate is failed as written, and it is not re-read in the engine's favour.

**The fix is made from this finding only and is measured on an unspent judgement case (105), never again on 102, 104 or 106** (see the next section).

### The fix for the one wrong answer, and the gates for its measurement on judgement case 108 (written before the run)

**The change, from the finding only (code; deployed 2026-10-20, see the end of this page).** An IMEI is 15 digits with its check digit, and some exports store the first 14. The engine now treats the two forms as one handset: (1) the question turn of a question that names an IMEI says "Handsets: an IMEI is stored with its 15 digits or, by some exports, with 14 (no check digit); both are one handset, so compare imei IN ('<15>', '<14>')"; (2) the validator's foreign-identifier rule accepts the 14-digit form of an IMEI of the question as the question's own; (3) when the result is empty and the 15-digit form is nowhere in the case, the probe looks for the 14-digit form, and if the case holds it the model is told ("this case holds the handset X as Y ... compare imei with both forms") and repairs, so **"no record" is stated only when neither form is held**. Specs: one on the question turn, the repair, the true absence and the foreign identifier (195 engine specs pass). **Not handled, and said so:** the reverse (14 digits asked, 15 stored), which needs a pattern match that the validator does not allow and which no measured question needed.

**Development check (case 5, which stores one shared handset in 14 digits):** the five handset questions (three "which handsets has X been used with", two "which subscribers shared the handset") were **5 of 5 correct**, the shared-handset questions at the first attempt with no repair.

**Why judgement case 108.** Of the unspent judgement cases only 108 has a shared handset stored in 14 digits (105 stores both in 15, so it would not exercise the fix). The measurement is the whole bank on 108 (about 80 questions), once; 102, 104 and 106 are not used again.

### Gates for the run on judgement case 108 (written before it; the engine arm, agreement off)

1. **Wrong answers zero**, in total and in every analysis.
2. **Correct at least 90% in each analysis**; a decline or an abstention counts against it.
3. **Both shared-handset questions correct** (one of the two is the 14-digit case), and the controls 100%.
4. Latency is reported, and so is every repair and decline with its reason.
5. If a gate fails, it is written up and the fix measured on 105; nothing is tuned on 108 and rerun. A harness failure (a closed pool, a timeout of the machine) is told apart from an engine result by its reason and its time, as in the rule above.

### Result of the run on judgement case 108 (2026-10-20): every gate is **met**; 81 of 81 correct, no wrong answer

The whole bank (81 questions: A1, A2, A3, A4, A8 and A13 with their paraphrases, and the controls) on the unspent judgement case 108, the engine arm, agreement off, with the code of commit 1b6b8bd (the IMEI fix). Nothing was tuned on 108 and no question was re-asked.

**One harness limit, found when the run ended, and what was done about it.** The run stopped at 75 of 81 questions after 3,600 s with `Suite Timeout Elapsed`: Ginkgo has its own suite timeout (one hour by default) that the `-timeout 6h` given to `go test` does not change. Nothing of the engine was involved: the six questions that did not run (Q076 to Q081) had no first record at all. They are the last six questions of the bank, and **they include both shared-handset questions, the ones this run exists to measure.** They were run alone, once, on case 108 with the same code (145 s), which is the harness-failure rule above applied to a cause it did not name; the 75 answers read before are unchanged, and the two result files are kept (`RJ108.jsonl` of 75, `RJ108_rest.jsonl` of 6). The harness now passes `-ginkgo.timeout 6h`, so a long bank is not cut.

| Gate | Result |
|---|---|
| 1. Wrong answers zero, in total and in every analysis | **Met: 0 wrong of 81**, in every analysis |
| 2. At least 90% correct in each analysis | **Met: 100% in every analysis** (absent 2, address holder 12, common contacts 12, devices 3, distinct contacts 6, incoming count 6, pair count 6, connections 12, reach 12, top contacts 5, shared handset 2, the three controls). No decline or abstention on a question that has an answer |
| 3. Both shared-handset questions correct, controls 100% | **Met.** Q077 (which numbers were used with IMEI 000000000010017: 923000001001 and 923000001005) and Q078 (which subscribers shared the handset 000000000010033: 923000001003 and 923000001007) are both correct, **at the first attempt with no repair**: the query of each compared both the 15-digit and the 14-digit form (`imei IN ('000000000010033', '00000000001003')`), which is what the question-turn hint asks for. The three controls (an address, an owner, "is it a criminal") were declined, as they must be |
| 4. Latency, repairs, declines | Answered questions: median 44 s, 90th percentile 73.5 s, longest 118.3 s (the 75: 44.1 / 73.5 / 118.3 s; the six: 26.1 / 39.8 / 53.4 s). **Six questions needed a repair** (an extra attempt) of the 78 answered; all six ended correct. Declines: three, all controls: Q079 `the model judged that the views cannot answer the question` (20.5 s), Q080 and Q081 `no analyst view fits the question closely enough` (11 ms each) |

**What this does and does not say.** The gate on the three first judgement cases (102, 104, 106) stays recorded as **not met** (244 of 245; one wrong). The cause was found, fixed from the finding alone, and the fix was measured on a case never used before, as the rule written before the judgement run requires: it is met there, with zero wrong. That is one case and 81 questions: it shows the fix holds, not a new accuracy rate. The reverse form (14 digits asked, 15 stored) is still not handled and no measured question needed it. The connection class is still the slowest. The change is deployed (2026-10-20).

### Stage 2 of the bank, co-location at the hour: the development check, and the gates for the judgement run (written 2026-10-20, before the run)

**What stage 2 asks.** For two subscribers on the same operator: which cell sites both used in the same hour (`coloc_cells`), and in how many different hours they were at the same cell (`coloc_hours`), two phrasings each. The truth is computed by `tools/ask_eval/colocation_oracle.py` in plain Python from the generator's visible rows, at the granularity of the analyst view `v_cell_visits` (subscriber, cell, hour). Pairs that never meet are the controls: the right answer is "no row" for the cells and 0 for the hours. **What it does not cover:** the generator's 15-minute co-location key (A5 as the generator states it) and A7 possible meetings, which need a finer reading than one row per hour; they stay open.

**The judge.** `judge.py` has a new kind `cells`: the cells the arm returned are the values of its rows that are cell keys of the case (the case's whole list of cell keys is in the question's `expect.universe`; no column name and no SQL of the arm is read); they must be exactly the expected set. Nothing expected: no rows and the plain statement "No row matches what you asked.", never "no record of this case". A count of zero (`empty_is_zero`) is right as a row holding 0 or as that plain statement. Five unit tests (`tools/ask_eval/test_judge_cells.py`).

**Development check, case 2 (20 questions, engine arm, agreement off, code of 280af5c): 20 of 20 correct, no wrong, no repair, nothing declined or abstained; median 27 s, 90th percentile 30 s, longest 97 s** (the first question, which carries the cold start; every other question 24 to 30 s). The ten control questions (five pairs that never meet) were all right: "No row matches what you asked." for the cells and a row holding 0 for the hours. No code was changed after this check; nothing needed a fix.

**One finding, not a gate.** The sentence over a cells answer says "13 positions." because the rows are (cell, hour) visits drawn as positions on the map; the question asked for cell sites, and the two cells are in the rows and on the map but not in the sentence. It is correct as a count of rows and weak as an answer; **fixed in source afterwards (2026-10-20, deployed the same day, not part of the measurement above):** the headline of a map answer whose rows carry a `cell_key` now says "13 positions at 2 cell sites." (`askHeadline`, one spec). The judge reads the rows, so the measured results do not depend on the sentence.

**The run.** The judgement cases **102, 104 and 106** (the register's cases of the stage-1 judgement run; this question class has never been run on them and no co-location answer of theirs has been looked at: the oracle printed only counts), the engine arm, agreement off, 24 questions each: **72 questions, 36 of each kind, 13 pairs that never meet (26 control questions)**. The register was verified against the generator on 2026-10-20 before the run (`case_set.py verify`, all eight judgement cases: "the register still describes the generator"). **105 stays unopened for the measurement of a fix**, and 101, 103 and 107 stay treated as spent. The harness passes `-ginkgo.timeout 6h`; the rule for a harness failure (a closed pool, a timeout, a machine's sleep) is the one above: told apart by its reason and time, the questions that did not run are run alone once, both records are kept.

### Gates for the stage-2 judgement run (written before it)

1. **Wrong answers zero**, in total and in each of the two kinds.
2. **Correct at least 90% in each kind**; a decline or an abstention counts against it.
3. **The 26 control questions 100% correct** (a pair that never meets gets "no row" and 0, never an invented cell or hour).
4. Latency is reported (median, 90th percentile, longest), and so is every repair and decline with its reason.
5. If a gate fails, it is written up, only the cause the run shows is fixed, and the fix is measured on 105; nothing is tuned on 102, 104 or 106 and rerun. The headline wording above is not a gate.

### Result of the stage-2 judgement run (cases 102, 104 and 106; 2026-10-20): every gate is **met**; 72 of 72 correct

The engine arm, agreement off, code of 280af5c, 24 questions on each case, in one pass; no question was re-asked and no run was interrupted (each case ran 14 to 17 minutes).

| Gate | Result |
|---|---|
| 1. Wrong answers zero, in total and in each kind | **Met: 0 wrong of 72**, in `coloc_cells` (36 of 36) and in `coloc_hours` (36 of 36) |
| 2. At least 90% correct in each kind | **Met: 100% in each kind**; nothing declined, nothing abstained |
| 3. The 26 control questions 100% correct | **Met: 26 of 26** (13 pairs that never meet: "No row matches what you asked." for the cells, a row holding 0 for the hours) |
| 4. Latency, repairs, declines | Answered 72: median 31 s, 90th percentile 40 s, longest 79.5 s (case 102: 34 / 48 / 80 s; 104: 34 / 40 / 42 s; 106: 27 / 29 / 31 s). **No repair on any question** (every one was right at the first attempt). No decline |

**What this does and does not say.** On the unseen cases the engine answers "which cells did X and Y both use in the same hour" and "in how many hours were they at the same cell" with the right cells and the right count, and says "no row" and 0 for a pair that never meets, from `v_cell_visits` and the verified co-location example, with no change to the engine after the development check. It is the hourly question only. **Still open:** the 15-minute co-location of the generator's A5 key, A7 possible meetings (they need a finer reading than one row per hour), and the larger and noisier cases (the weak sentence over a cells answer was fixed in source, see above). The seeds 102, 104 and 106 are now used for this class as well; 105 stays unopened for a fix.

### Deployed 2026-10-20 (the owner's go: "I authorise deploy; proceed with the next steps")

**What went live.** API image **`5c25f8785565`** (rollback tags: `before-imei-20261020` = `0b65976498f3`, the image of 2026-10-19; `imei-first-20261020` = `9a551c4848d2`, the first build of today, see below). It carries the IMEI fix (the two forms are one handset), the sentence over a map answer that says how many cell sites ("5 positions at 5 cell sites."), and `GET /analysis/engine-health`. Built as on 2026-10-19: the Linux binary cross-built on the host (`go build -mod=mod`, `go.mod` restored) and layered onto the previous image; recreated alone with `recreate_api_askv2.ps1 -AskV2 true -AskV2First true`; the variables did not change (typed families, V2 and FIRST on, agreement off, governed lane on and first). The dev servers on :4181 and :4190 were restarted with `Restart-Workspace.ps1` (the proxy reached the API: HTTP 200 on both).

**A defect found by the first live read, fixed and deployed again the same hour.** The first image (`9a551c4848d2`) showed 121 questions on the synthetic collection, 113 of them declined with "an explicit operation was requested": the interface's own template requests, which were audited as declined questions until 2026-10-19 and which the engine never looked at. The totals and the decline reasons now leave out the five reasons of `askEligible` (commit 53c6652; the live spec fails when the exclusion is emptied). The same collection then read 8 questions, 8 answered.

**Live checks on `5c25f8785565` (all passed):** the six questions on the real collection **6 of 6** (pair count, top contacts, incoming count, an absent number, a question the data cannot answer, a connection question; the slowest, the connection question, 200 s); **D22 on the synthetic collection 3 of 3** (a shared contact named; two shared contacts named; "in direct contact"); **the IMEI fix on the synthetic collection 2 of 2**, the second being the original failure (a handset the export stores as 14 digits, asked as 15: found, one number, equal to the SQL truth) and the first its control (stored and asked as 15); **a template request returned 200 and wrote no audit row** (724 rows before and after); **the cells-used question** answered "5 positions at 5 cell sites." through the engine; **the engine-health read** returned the contract on the synthetic and the real collection (the real one: 26 questions, 21 answered, 5 declined, 1 decline reason) and zeros for a collection that does not exist. Nothing of the real collection was printed: counts and booleans only.

### The multimodal acceptance case typed into the live tables (2026-10-20, the owner's go: "I authorise the live typing steps, start with the multimodal case")

**What was done, in order.** (1) A backup of the live database (`pg_dump -Fc`, 185 MB, `C:/tmp/nexusai-backups/localrecall-before-seed-layout-20261020-1547.dump`; the archive lists 4,363 entries; restoring a TimescaleDB dump needs the usual pre- and post-restore steps, not rehearsed). (2) The worker overlay rebuilt: `nexusai-forensic-records-worker` `40a9a7195685` = the previous worker (`b136e8f2d5e1`, rollback tag `before-seed-layout-20261020`) plus the one layout file; recreated alone with the documented start command (healthy, shadow mode, 14 layouts in `/app/layout_registry/layouts`). (3) `ipdr-seed-s8` registered in `forensic.layout_versions` with the registry's idempotent SQL (the 13 existing entries did not drift). (4) A rehearsal of the back-fill inside the worker, on the live database, rolled back (5,000 call records read through `call-c4-telenor-style` and 2,500 internet sessions through `ipdr-seed-s8`, both reconciling; nothing left behind), then the same run committed: **`comm_event` 5,000 rows (`target_cdr`, 2026-04-01 to 2026-04-26 UTC) and `data_session` 2,500 rows (2026-07-14 to 2026-07-15 UTC)** for `nexusai-multimodal-product-acceptance`. The effective view still counts 5,000 call records (an item is never counted twice). The script is `backfill_typed.py` (run in the worker through the worker's own `run_typed_ingest_shadow`; kept in the session scratchpad, not in the repository).

**What the first live read of the engine on this case showed, and what it cost.** The six live checks on the typed case: 5 of 6; the miss was a pair-count question that the engine declined after the model invented a column three times (`UNKNOWN_COLUMN`), and the older path then answered it wrongly. The 30 call-record and internet-record questions of the factory's seed-7 spread (the families the typing changes), with the engine first: **24 correct, 2 abstained (as in the earlier run), 4 wrong that had been correct** (`arm-engine-typed-s7mm` and, for the re-asked four, `arm-engine-typed-s7mm-fix`; baseline `arm-askv2-first-s7mm`; the arm folders are git-ignored and stay on this machine; the other families are untouched by the typing and were not re-run). Median 32 s, 90th percentile 52 s. The four, all answered by the engine, had two causes:

* **Three volume questions** (sum, average, minimum of the volume): the seed export states one volume with no direction, which the entry deliberately does not map to `bytes_up` or `bytes_down`; the view had nothing to add up and the engine said "no value (nothing to compute over)". The coverage read now says whether any typed internet record states a byte count (`AnyVolume`); a question with a volume cue on a case that has internet records and none with a count is **declined with the reason** and the older paths answer as before.
* **"What is the latest call start time in the calls?"**: the model added a filter on direction that nobody asked for (outgoing) and the answer was a day early. A new **DIRECTION obligation** sends back a query that filters on direction (`=`, `<>`, `IN`, `LIKE`) when the question contains no word that gives it a direction (incoming, outgoing, received, get, make, send, dial, from, to, the word direction and so on).

Both fixes are general and deployed (API image `3132ff0a9630`, later `cc98f7e8c8fb`; rollback tag `before-guards-20261020` = `5c25f8785565`; commit 86589b5). **After them the four were asked again: all correct** (the three volume questions declined by the engine and answered by the lane; the latest call time by the engine). **Honest limits:** the other 26 questions were not asked again after the fix; the full 79-question spread was not run (the roadmap's gate "no regression on the factory and the corpus" is met for the two families that changed, not for the whole spread); the pair-count question that failed in the live check was not asked again.

**A finding that needs a decision, not made here.** The call-record layout `call-c4-telenor-style` maps the call-type labels of this file to event types as `CALL` to voice (888 rows), `SMS` to sms (793), and **`GPRS` (2,721) and `VOLTE` (598) to other**. `VOLTE` is voice over LTE, a voice call; a question that asks only for voice calls therefore leaves 598 calls out, and the latest-call answer above is the latest of the 888 voice rows (2026-04-26 05:35 PKT), not of all call records (07:13 PKT, a GPRS row). The vocabulary belongs to the layout (a change is a new layout version and goes through its confirmation), so it is listed as open: map `VOLTE` to voice (and `GPRS` to data), or leave it and say so in the answer.

### forensic-demo typed into the live tables (2026-10-20, the owner's go: "write the layouts and type forensic-demo")

**Steps.** A second backup (`C:/tmp/nexusai-backups/localrecall-before-forensic-demo-typing-20261020-1650.dump`, 187 MB); the registry taught to match the latest version of a layout only; `call-c4-telenor-style` version 2 and `call-demo-8` in the worker overlay (`0651ab31e9c1`; rollback `before-c4v2-20261020`) and in `forensic.layout_versions`; the multimodal call records typed again under version 2 (derived rows deleted in one transaction, then the same back-fill: voice 1,486, data 2,721, sms 793); a rehearsal on forensic-demo, rolled back, then the back-fill: **`comm_event` 8,941 rows and `data_session` 2,500 rows**, all six files typed (the failed job's evidence is outside the guard). The one 3,931-row file whose job states no time zone was **quarantined whole** in the first rehearsal; it was typed with **Asia/Karachi as an assumed zone** (the zone of the collection's other files), so its rows carry `time_basis = assumed_by_profile`. This is an assumption and awaits the owner's confirmation.

**What typing changed (by design).** The typed layer keeps exact repeats and flags them; the effective view leaves them out. That 3,931-row file is **68.7% exact repeats** (2,701 rows identical to the second), and one of the two small fixtures repeats a row, so the effective call count is **6,239 where version 1 had 8,642**. Numbers are also read by rule (9 distinct callers where version 1 counted 10; the typed layer normalises number forms; not traced further).

**The check (27 of the 30 call-record and internet-record questions of the seed-7 spread; the run stopped when the factory's own key query timed out (the clock shows the machine had slept), and was not resumed).** Engine first. Verdicts against the factory's version-1 keys: 19 correct, 1 abstained, 7 wrong. Median about 30 s. **Each wrong verdict was re-keyed on the effective views, with no model run** (the oracle SQL read from `records_effective_cdr` and `_ipdr`): **six of the seven equal the typed answer** (call count 6,239; distinct callers 9; average latitude 31.5523; a month's count 378; night 1,425; a date range 996), so they are the definition change and not engine errors. **One was a real fault: "How many call records did Hamza Ghauri have?" was answered "N: 0"**, the query having compared a number column with a person's name. A new **NAME obligation** (commit 903c05e) refuses `src` or `dst` compared with a constant of two or more capitalised words; deployed (API `cc98f7e8c8fb`; rollback `before-name-20261020` = `3132ff0a9630`); asked again, the question is routed to the clarification path and states no count. The abstained question ("How many call records involve" a SIM identifier) is an engine abstention, not an error.

**Honest limits.** Three questions of the 30 were not run; the factory keys remain version 1's (the re-keying above was done by hand for the wrong verdicts only; a re-key of the factory on the effective views for typed cases is step S1 of `docs/roadmap/reconciliation-20261020.md`); no full spread was run; the model's writing of the name query is sampled, so the fault could recur in another form until the obligation has met more questions. A Postgres detail found on the way: the container's shared memory is 64 MB (the default), and a large parallel query over the effective view failed with "could not resize shared memory segment"; the engine's short queries have not hit it, but `shm_size` or `max_parallel_workers_per_gather = 0` for the analyst role is a one-line safeguard that was not applied.

**Tools.** `tools/dev/typing_report.py COLLECTION` shows in seconds, without a model, what the runs above took an hour to find (repeats 30.2% of forensic-demo, assumed zone on 3,931 rows, no byte count and no public address in the seed internet export); `tools/dev/live_smoke.py [--quick] [--real]` is the live check as one run (`--quick`: 5 of 5 in 163 s today).

### S2: the column-population guard, and repeats said in the answer (2026-10-20, night; API `0dcf72adddf8`, rollback `before-s2-20261020` = `cc98f7e8c8fb`; commit 248136e)

The general form of the volume guard. The coverage read (30 s cache) also runs one probe per typed table of the case and reports the tracked view columns that no row fills although the table has rows (`Unpopulated`, "view.column"), and the number of typed call and internet rows flagged as exact repeats (`Repeats`). After a query is validated and before it runs, one that reads an unfilled column is declined with "no row of the case holds a value in V.C (the records do not state it)", so the answer comes from the older paths instead of "no value" or a zero; a column is named only when every view the query reads is tracked and empty for it. An answer over the call or internet views adds the limitation "N rows of this case's call and internet records are exact repeats of another row and are left out of these counts" and the audit carries `repeats_left_out`. Verified live on forensic-demo (2,702 repeats, 6,239 calls) and the multimodal case (no repeats, no note). Specs: the decline before anything runs, the note and its absence, the rule for a column that another view fills or that is not tracked; the 20 live-database specs ran the new probes against a real database. **Not covered:** the cell views' position and radius, and any column that is filled for some files of a case and empty for others (the guard asks whether any row holds a value, not what share).

### S2b: the older path reads the effective view of a switched family (2026-10-20, night; API `ee276c1f9bcd`, rollback `before-s2b-20261020` = `0dcf72adddf8`; commit 337e059)

Found by re-keying forensic-demo: a sum answered by the older path (route `records_sql`) came from version 1's table (59,731,961,864) while the engine and the lane read the typed rows (17,201,803,847 with the exact repeats left out). The source-native executor now reads the family's effective view for a plan whose fields all belong to one switched family; `sourceNativeBinding` carries `Family` (set after the plans are compared, so two ingested families are still not a "mix of sources") and `RecordsTable`, and the count, the projection, the aggregate and the provenance read `binding.records()`. With `FORENSIC_TYPED_FAMILIES` empty, with the view missing, with a plan over two families or an uncurated field, nothing changes. **Verified live** on the same question (forensic-demo 17,201,803,847, equal to the effective key; the multimodal case 10,929,645,657, equal to its key) and by the quick live smoke. **Not done:** the interface's templates and the helpers that read `forensic.records` directly still read version 1 (see `typed_families.go`).

### Migration 020 and the engine's use of the thick views (2026-10-21; the owner's go; API `77ec230274a2`, rollback `before-020-20261021` = `ee276c1f9bcd`; commit 950dab3)

The connection question was the slowest class because the verified example was a bounded recursive query of about 220 tokens, written at about 5.5 tokens a second. Migration 020 adds `v_contact_links`, `v_connection_edges` (edges of every path of one to three links between `a` and `b`, with `hops`, `ordinary`, `rank_any`, `rank_ordinary`) and `v_evidence_inventory`; the verified example became `SELECT src, dst FROM v_connection_edges WHERE a = '..' AND b = '..' AND ordinary AND rank_ordinary = 1 LIMIT 200`. **A measurement mistake worth keeping:** the first version read the base view in each of its six branches and took 24 s on the real collection (0.15 s for the recursive example); `MATERIALIZED` links brought it to 82 ms (94 ms for the example). Engine changes: a lexicon entry and a feature give a two-number connection question the view alone (the contact pairs stay for common contacts and single-number reach); more than three hops swaps in the contact pairs and the example `connection-path-long`; the HOPS, RECURSION, SERVICES and HUBS obligations and the src/dst result rule accept the view; an empty path on the view is an answer; the coverage guard and the repeats note know the views; examples `connection-path`, `connection-path-any`, `evidence-count`; the pinned catalogue has ten views. Specs: 203 engine, 4 catalogue (every example validates and runs as the read-only role), 23 live-database (the view equals a search of the key's contact graph for 30 ordered pairs; the inventory counts a case's files for that case only; the typing report equals the views). **Live:** connection answers on the synthetic collection 28.6 s and 26.4 s (55 and 76 s before; 109.6 s for the first, with the cold prompt of the changed rules); **the real collection 39 s (model 19 s, query 0.3 s) against 182 to 200 s**; "how many images are in this case" 23 in 20 s, "how many files failed" 1 in 7 s. **Not run again:** the path and reach classes of the M3 bank with the view (a milestone run in the background, T4).

## Next

The rest of A5 and A7 (the 15-minute co-location and possible meetings, which need a finer reading than the hourly view), the sentence over a cells answer, then the engine on the typed families beyond call records and internet sessions, the remaining interface pieces of 7.8 (lanes, calendar, the entity drawer), the composition of the sentence under the number lock (M5), and applying migration 017 to the live database on the owner's go, after a backup, so that the engine can be deployed and measured on the live stack.
