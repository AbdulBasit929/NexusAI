# Text-search intent in Ask: design and pre-registration (2026-11-10)

Written before any live number exists. The thresholds below are fixed; a miss is reported as a miss.

## The direction this implements

Team lead, 2026-11-10: no search page. A search is an ordinary question in Ask. The model understands the question at run time and the server routes it; no predefined templates, no stored questions, no code that knows a particular question.

## What was measured before this (capability audit, 2026-11-10)

The governed text retrieval over audio transcripts, image text and document passages is exhaustive, cited and fast when it is called with a literal and a family. A natural question reached it only by accident:

| Question wording | What the live engine did |
|---|---|
| "Which documents mention wire transfer?"-style sentences | the SQL engine answered first with a number or an unrelated table (the "N: 7" and endpoint-table answers) |
| An unquoted phrase with no source named | "Choose a source or name its evidence family for this literal text search" |
| The same literal typed in another capitalisation | "nothing matched" (the retrieval compares case-exactly), which reads as a false absence |

## What is built (all behind `FORENSIC_TEXT_INTENT`, default off, nothing deployed)

1. **One model call per eligible question** (`ask_text_intent.go`). The model sees only the analyst's sentence and a fixed instruction, never case data. Strict JSON schema, temperature 0, 160 tokens at most: `kind` (text_search or other), `family` (audio, image, document, any, none), `match` (phrase, exact_phrase, all_words, none), `literal`.
2. **The server disposes.** The proposal is used only if: the literal is a verbatim part of the sentence (case and spacing ignored; for all_words each word is), the family and match are in the fixed sets, `forensictext.Query.Validate` accepts it, and the literal is not a national identity number. Anything else, and any failure of the call, changes nothing.
3. **A valid proposal answers nothing by itself.** It fills the structured text query and operation an analyst could have sent; the existing governed retrieval runs, with its own completeness states and citations. The SQL engine and the governed-SQL lane are not given a text search to turn into a count.
4. **Case-insensitive matching** is a new, additive field on the structured text query (`case_insensitive`, false unless set). Only this route sets it.
5. A family of `any` keeps the literal and leaves the operation to the existing path, which asks one precise question. Retrieval across all three families is the next step (section "Step 2" below).

Not built, deliberately: any keyword list, any question pattern, any per-question rule. The prompt describes the task; it contains no example question.

## Measurement design

Bank (built from the case data, not typed by hand): for each of the three families, literals of one to three words are drawn from passages that exist in the synthetic and public-data cases (`nexusai-multimodal-product-acceptance`, `nexusai-media-wave1-20261021`, `nexusai-wave1-dev3-20261021`, `orion-lahore-20261030`), each appearing in at least one and at most thirty passages. Each literal is put into sentences of different shapes, with and without the source named, with the literal capitalised differently from the passage, plus Urdu-script literals from audio transcripts in Urdu sentences. The oracle for each is a SQL query over the stored passage text, independent of the product's matching code (case-folded substring, evidence ids and passage counts).

Negatives: 50 plain questions that are not text searches (counts and lists of calls, messages, transactions, vehicles, plates, locations; definitions; greetings; "what does the audio transcript say?"; "summarise the documents"), taken from the existing verified bank and the front-door corpus, plus 8 hard negatives where a word is looked for in a structured field (for example message bodies), reported and not gated.

Two tiers, so no long run follows each change:

* **Tier 1, the decision.** The real model is called with the production request and the server's validator is applied (a live test, `TestTextIntentLive`, skipped unless a bank path is given). Measures routing, family, literal fidelity, false routes and the added time. Minutes.
* **Tier 2, the answer.** Only the questions that routed are sent through the API (a staging instance of this build on 127.0.0.1:8191, switch on), and the evidence ids returned are compared with the oracle. Retrieval is fast; no SQL generation runs for these.
* **Tier 3, no change when it should not.** Ten negatives are sent through the staging instance with the switch on and off; the answers must be identical.

## Gates

| Gate | Threshold |
|---|---|
| G1 routing | at least 90% of text-search positives that name their source are routed to that family; at least 90% of all positives are routed or reach the one precise question |
| G2 wrong answers | 0. A routed question whose result says the search was exhaustive but whose evidence ids differ from the oracle, or any answer that is a count or table in place of a search |
| G3 literal fidelity | 100% of routed questions search the literal the oracle expects (case-insensitively) |
| G4 false routes | 0 of the 50 plain negatives are routed |
| G5 added time | median at most 8 s, 95th percentile at most 25 s, on this laptop with the model resident |
| G6 failure | a failed or malformed model call changes nothing (unit test) |
| G7 identity | the Tier 3 answers with the switch on equal the answers with it off |

If G1 to G4 are met the switch may be proposed for deployment (the owner decides, per deploy). A miss is analysed from the audit, not patched by a rule that names the failing question.

## Step 2: all sources

A question that names no source ("is 'invoice' mentioned anywhere?", "does the audio or the documents say…") needs the retrieval over all three families, merged, each with its own completeness. That is built after Tier 1 and Tier 2 are read, and measured against the same oracle with a source-free sentence set; its gates are written into an amendment to this file before it runs.

## Amendment 1 (2026-11-10, after the first model run; thresholds unchanged)

**Run 1 is a development run.** Production model (qwen3-4b-instruct), 142 decisions in 762 s, bank `evaluation/text_intent/bank-20261110.json`:

| Gate | Run 1 |
|---|---|
| G1a routed to the named family | 51/63 = 81.0% (not met) |
| G1b routed at all | 77/84 = 91.7% (met) |
| G3 literal fidelity | 68/77 (not met) |
| G4 false routes on plain negatives | 11/50 (not met) |
| G5 added time | median 5.1 s, p95 7.3 s, slowest 14.4 s (met) |

**Reading the misses (from the audit, none by hand-picking):**

* Model errors: a phone number or plate that only identifies who a count or ranking question is about was taken as words to find (8 of the 11 false routes); a greeting and "what does the audio transcript say?" were routed with the whole message as the literal (2); one more count question was routed as a document search; a 12-digit number came back with its last digit cut off, which the verbatim test accepted because the shorter number is still a part of the sentence; six searches (two plain ones, four with junk phrases) were judged "other"; one answer used family `none` with kind `text_search`.
* Defects of the bank, mine: the Urdu sentences always said "recording" even when the words came from an image or a document (9 of the 12 family misses are the model correctly following the sentence); "search the files for" does not name a source; some sampled phrases began or ended with a filler word ("and other", "enough and"), so the model's trimmed literal differs from the expected one although it is the better search (4 of the 9 literal differences).

**Changes, all general (none names a question):**

1. Server: the literal must stand alone in the sentence (not be a part of a longer word or number), and a literal equal to the whole message is refused.
2. Instruction v2: says what a text search is and, in the same general words, what is not one (counts, rankings, times and relationships about calls, messages, transactions, devices, vehicles, people; identifiers that only say whom or what a question is about; greetings; definitions; summaries); asks for the literal with every letter and digit and never the whole message. No example question.
3. Bank v2: Urdu sentences name the right source for each family; sentences that do not name a source are only used in the source-free set; sampled phrases may not begin or end with one of the family's most frequent words (language-independent). Run 1's bank is kept as the development set.
4. **Holdout.** A second bank built with a different seed and frames is built before the changed step is run on it and is run once for the gates. Development reruns on the run 1 bank are reported but not counted.

If the holdout misses a gate, that is the result; the step stays off and the analysis says why. The model is the production model on purpose: a larger model that the deployment does not run would measure something else.

## Amendment 2 (2026-11-10, after the second development run; thresholds unchanged)

**Run 2 (development bank 2, instruction v2 and the two server checks, 133 decisions, 852 s):**

| Gate | Run 2 |
|---|---|
| G1a routed to the named family | 53/57 = 93.0% (met) |
| G1b routed at all | 70/75 = 93.3% (met) |
| G3 literal fidelity | 62/70 (not met) |
| G4 false routes on plain negatives | 11/50 (not met) |
| G5 added time | median 5.6 s, p95 10.0 s, slowest 30.9 s (met) |

**Reading it.** The false routes did not fall (11 of 50 again, a different eleven): nine are questions about voice calls, bank debits and totals that carry a phone number or an account number, which the 4B model reads as a search of the recordings or documents. Rewording the instruction did not change that, so the next change is to the question the model answers, not to its wording. Literal fidelity: four of the eight differences are filler words the model correctly left out of a sampled phrase (a defect of the bank); three are weak Urdu (the model returned the word that names the source, or the whole message); one is a check of mine that ignored the question mark on the model's literal (fixed, with a test).

**Change.** The model no longer answers "is this a search, and of what". It states what the message is about from a fixed list: spoken words, text in images, text in documents, words in an unnamed source, and then call records, message records, bank transactions, device records, vehicle records, location records, internet records, people and numbers, other. Only the first four are searches; every other value is not, whatever literal comes with it. Calls, messages and transactions now have a place to go that is not "recordings". The server's checks are unchanged. The kind and family are derived from the answer; they are no longer asked for.

**Procedure.** The same rules as Amendment 1: the change is tested on development bank 2 (reported, not counted); the holdout bank, built before any of this and not yet run, is run once for the gates. The strict literal gate stays as written. The holdout report will also show how many literal differences are a filler word at the edge of the sampled phrase, so a reader can see how much of a G3 miss is the bank.
