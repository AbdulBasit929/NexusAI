# Ask: does a search return ALL occurrences? Baseline on the live engine (2026-11-19)

**Owner rule (2026-11-19):** questions are answered by LLM-built queries at runtime, with no predefined templates or phrase rules; a search for a word, name, number, plate or any other thing returns **all** of its occurrences, best and accurately, across images, documents, plates, recordings and any other source, not a single hit.

## 1. Method

The synthetic Orion Lahore reference case (`orion-lahore-reference-20261030`) holds documents (18 files, 295 passages), images (39 files, 231 OCR lines) and recordings (15 files, 39 segments). For 8 probes (a phone number, a reference number, the word "synthetic", the word "IMSI", a name, a plate, an account number, the word "video") the **true list of evidence files** was computed from the database itself (`forensic.derived_artifacts`, documents, image OCR lines and audio segments; the plate also from the plate readings). Each probe was then asked in 3 to 5 natural ways (plain, indirect, formatted differently, Roman Urdu): **30 questions** (`tools/showcase/ask_exhaustive_bank.py`, bank-A) against the live engine (`/query/hybrid`, engine v2, text-intent step on). Scoring is on the files an answer cites: recall = true files cited over true files; precision = true files cited over files cited. The wordings are test questions, not something the engine was built around.

## 2. Result

| Measure | Value |
|---|---|
| mean recall | **0.29** |
| every true file cited | **6 of 30 (20%)** |
| partial (some, not all) | 5 |
| nothing cited | **15** |
| engine states | answered 7, declined 5, no engine state (older path) 18 |
| median seconds | 7.8 (30 to 90 s when the model-written query ran) |

By probe (questions, mean recall, full recall): phone 5, 0.50, 2/5; reference 4, 0.50, 2/4; "synthetic" 4, 0.48, 1/4; "IMSI" 4, 0.00, 0/4; name 3, 0.00, 0/3; plate 4, 0.25, 0/4; account 3, 0.33, 1/3; "video" 3, 0.07, 0/3. When the same fact was asked two ways the outcome often differed: "how many and which documents contain the word SYNTHETIC" cited all 11 files, "find every document marked as a synthetic test document" 10 of 11, "which files say synthetic?" and "show all evidence mentioning synthetic" none.

## 3. Why (read from what the engine answered and from the code)

1. **"Search in any source" is a dead end by design.** The text-intent step (`ask_text_intent.go`) lets the model say `words_in_any_source`; the step then sets no template (the unit test asserts `Template == ""`), and the request ends in the clarification "Choose a source or name its evidence family for this literal text search" (`query.go`). Five questions ended there (no source named: "search REF00000394", "show all evidence mentioning synthetic", ...). This contradicts the rule: an unnamed place must mean everywhere.
2. **Identifier and word questions are claimed by the structured-data route.** "Where does 042-35761234 appear", "Which files contain an IMSI", "Where is the IMSI recorded" and "where is bank account number ... used" were answered by the model-written SQL over analyst views ("No row matches: no cell visit of any number...", "Modality structured_records ...", "36 rows"): a question about where a thing is written got a table answer. The step's own instruction tells the model that "a number, account, plate or name that only says whom or what a question is about is not a text to look for", which the model applies to "which files contain X".
3. **The search is family-scoped and exact.** Plates in an image were found only through the plate route; the same plate written with Urdu digits in an Urdu witness statement (`SYNT-۹۱۵۰`) was never found; "04235761234" and "042 35761234" match "042-35761234" only by luck. There is no tolerance for digit scripts, separators or reading errors (an OCR'd "لاہول" for "لاہور").
4. **Incomplete searches stop at a budget.** For a name that appears once, two questions ended "The text search is incomplete ... absence of further matches has not been established" (budget reached, `derived_text_semantics.go`); on a small case this should never happen.
5. **A list is shortened when it is shown.** "D01-fir-english.docx and 9 other documents contains 10 cited passages" names one file; the other nine are not in the text the analyst reads.
6. **The older keyword routes still decide too much** (`forensictext.Extract` with `audioHint` / `imageHint` / `documentHint` regular expressions, `evidence_search_first.go` keyword lists, the "media misroute guard"): the owner's rule is that no predefined code decides what a question means.

## 4. What the fix looks like (no templates, no phrase rules)

See `docs/roadmap/ask-exhaustive-search-plan-20261119.md`. In short: one runtime path for a lookup of text or an identifier: the model proposes the literals and the kind; the server validates the proposal (the literal is verbatim in the sentence, the family is a member of a fixed set, national ID numbers are never routed) and runs the governed retrieval over **every** text-bearing source by default, with tolerant matching tiers labelled on each hit, every file listed, and an explicit completeness state.

## 5. How the fix will be judged

Bank-A is the baseline and may be used while tuning. The decision rests on a **second bank of new wordings and new probes (bank-B)** written after the change is built and not seen while building; a change that fixes bank-A and not bank-B is not a fix. The 484-question verified bank (`tools/showcase/build_verified_bank.py`) is the regression check: no answer that was correct may become wrong or declined.
