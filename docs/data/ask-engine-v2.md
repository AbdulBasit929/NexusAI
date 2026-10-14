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

## Next

The model call with the laptop's output cap and repair loop (at most two), the EXPLAIN cost ceiling and row cap in the executor, the obligation checks and back-translation, the audit record, and the `FORENSIC_ASK_V2` switch with its arms against the lane: roadmap 6.8.11, M1 and M2.
