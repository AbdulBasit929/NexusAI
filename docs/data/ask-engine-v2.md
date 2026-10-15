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

## Next

Measured arms on a fresh seed (above), then the engine on the typed families beyond call records and internet sessions, the interface pieces of 7.8 (result views by shape), the composition of the sentence under the number lock (M5), and applying migration 017 to the live database on the owner's go, after a backup, so that the engine can be deployed and measured on the live stack.
