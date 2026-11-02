# Answers that show their evidence and a reading of the result (plan, 2026-11-02)

Owner request (2026-11-02): an answer must not be a bare result. It must show the sources, the rows and the best evidence behind the computed figure, how it was done, and a language model must help to interpret the answer. Status 2026-11-02: part A and part B BUILT, checked on a restored copy of the live database with the real model, and DEPLOYED at 12:29 PKT on the owner's word (image `round9b-20261102`, commits 175e5f7, f545a6c, a459d11); the plain presentation for analysts (headings, the file's own column names, no ids or hashes) is the next image, round 9c. Decision D33 DECIDED by the owner the same day (yes, the model may read the rows of a result, local model only, labelled, number-checked, after the figure). Part A2 (the records behind summary views) is next. Results: `live-window-20261030.md`, section Round 9b.

## What exists today (checked 2026-11-02 on the live engine, round 8)

* The first sentence is written by code from the result (`askHeadline`, `askWording` in `ask_engine_response.go`); the model writes only the query. "Longest call: 1,800." is a figure with no row behind it.
* "How was this answered" already exists: the plain-words steps come from the query's parse tree (`ask_engine_describe.go`), so they cannot claim a filter the query lacks.
* The Sources box of the workspace says "None openable" because the engine puts only one provenance item in `enterprise.provenance` (`source: analyst_views_sql`), with no file and no row.
* The workspace already knows what to do with evidence: `lib/citations.js` turns every item of `enterprise.provenance` that has `evidence_id`, `version_id`, a row number and a `row_hash` into an openable source, and `enterprise.contribution_lineage` into "N files". So the work is in the engine; no new interface is needed to show it (to be confirmed on :4181 with one built example before the rest).
* The data to point at exists: every stored event carries `evidence_id`, `source_row` and `row_hash` (`forensic.comm_event`); `evidence_items` has `original_filename` and `current_version_id`. `analyst.v_comm_events` exposes `event_id`, `evidence_id` and `source_row` but not `row_hash` or the version.

## Part A: the rows behind the figure (code only, no model)

1. After the answer query has run, derive an evidence query from the answer query's own parse tree: the same single view, the same WHERE conditions, ordered as the answer is ordered (a maximum, a minimum, a first, a last or a top N is the row or rows themselves; a count, a sum or an average is the matching rows, capped at 50, with the total and the count per source file).
2. Look the rows up (event id to file, version, row number, row hash) and write them as provenance items with `source_truth_state = ingested_source_record`, plus a contribution lineage with the files and their counts.
3. A check before anything is shown: recompute the figure over exactly those rows (the count of the evidence rows equals the count; the maximum of their durations equals the maximum). When it does not match, say "the rows behind this figure could not be matched to it" instead of showing rows that do not prove it.
4. A question whose query cannot be traced this way (a join of two views, a recursive link search, a set operation) says plainly that its rows are not itemised, and why; it never shows an empty box.
5. Needs: a migration (033) that adds a read-only analyst view of event sources (event id, file name, version, source row, row hash), an audit of which views carry a row key (calls do; the other families to be checked one by one), Ginkgo specs on a throwaway database, then the live-window procedure (backup, rehearsal, apply, rollback tag) and the owner's word for the deploy.

## Part B: a reading of the result by the language model (decision D33)

* After the answer, one more model call receives the question, the derivation sentences, the result rows (at most 20) and the evidence summary (files and counts), and writes two to four plain sentences: what was found, what it rests on, and what it does not show. It is shown under the figure as "Reading by the model: check it against the rows", never in place of the first sentence.
* Guards in code: every number in the reading must appear in the rows or the evidence summary, otherwise the reading is dropped; the wording rules of the engine (no statement about who a person is or what they did; a registration is not an owner) are applied to it; it has its own time limit and never delays the figure (it arrives after it).
* **What changes, and why the owner decides.** Every answer today says "the model saw no case data and wrote none of the numbers". With Part B the model sees result rows (and, with Part A, names of source files and row numbers). The model is local (LocalAI on this machine, `qwen3-4b-instruct-2507-q4km`), nothing leaves the machine, but the sentence becomes "the model saw the rows of this result and wrote only the reading, never the figure", and the audit record keeps the prompt and the model. A 4-billion-parameter model reads short results well and long ones badly: the reading is limited to results of 20 rows and says nothing when the result is larger.
* A reading cannot repair a wrong query: when the query is wrong (see ORL-EXT-007, -009, -012 of the pack) the reading explains a wrong figure. Part A helps here, because rows that do not match the question show the fault; Part B must never make a wrong figure look more convincing.

## The pack after Part A and B

Each question of the pack gets an evidence column: how many source files and rows are expected (from SQL on the stored rows), and the first row to open. The judge scores the citation: the file and row the answer cites must exist and must contain the figure.

## Order and cost

A first (it needs no decision), measured on the pack; then B on the owner's word. Both change only the engine and one migration; the interface on :4181 keeps its old design and shows the new items through the slots it already has.

D33 (owner): may the model read the rows of a result to write the reading (Part B), with the new wording of the limitation? Recommended: yes, local model only, labelled, number-checked, shown after the figure.
