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

## Amendment 3 (2026-11-10, written before any plate-routing number exists): the model also routes "which photos show this plate"

**Why.** The owner's rule: every runtime question is understood by the model and answered from the database, not routed by a keyword list. The plate-in-images search (`plate_image_search.go`, exact and similar readings, search-only) was first reached by a keyword cue plus a format pattern, which rejects plates that fit no known format and misses unusual wordings (measured: a plate typed as `SYNT4O24` was missed by the pattern). The intent step already reads each sentence once; it now has one more subject in its fixed list.

**Change.** One new value of `about`: `plate_in_images_or_video` (the investigator typed a vehicle number plate, in any format, and asks which photos, images or videos show it). The plate is returned as the literal. The server keeps the same discipline as for words: the literal is a verbatim part of the sentence, stands alone in it, is not the whole message, and is 3 to 12 letters and digits; no list of plate formats is consulted. The `vehicle_records` line of the instruction now says that camera (ANPR) sightings are vehicle records and that a question about which images or videos show a typed plate is the new value. When the step does not decide (switch off, model unavailable, unsure) the old keyword route still runs afterwards, so nothing that worked is lost. Nothing else in the instruction changes.

**Bank (new, `evaluation/text_intent/bank-plate-20261110.json`, built by `build_plate_bank.py`, seed fixed in the file).** 36 positives: plates generated in nine formats (Pakistani letters-and-digits shapes, province-style, dashed, spaced, foreign-looking, and a free shape that matches no registry entry), put into twelve sentence frames in English, Roman Urdu and Urdu. 24 negatives: the same generated plates in questions about camera sightings, counts, owners, timelines and "plates containing", which must NOT be routed to the image search. No owner-labelled plate is used; every plate string is generated.

**Gates (fixed now).**

| Gate | Threshold |
|---|---|
| P1 routing and fidelity | at least 90% of the 36 positives routed to `plate` with the plate exactly as typed (case, dashes and spaces ignored) |
| P2 false routes | 0 of the 24 plate negatives routed to `plate` |
| P3 no regression on words | on development bank 2 (reported against the last run, 2026-11-10 `ti-tier1-dev2c`: G1a 84.2%, G4 0 of 50): G4 stays 0 of 50 and no word-search positive is routed to `plate`; G1a may not fall by more than 3 points |
| P4 time | median at most 8 s, p95 at most 25 s over the plate bank with the model resident |

A miss is reported as a miss and analysed from the audit; the instruction is not patched with the failing sentence. The holdout bank remains unrun and sealed for the text-search gates.

## Amendment 4 (2026-11-10, after the first plate run; thresholds unchanged)

**Result of run 1 on plate bank A (`ti-plate-run1`).** P1 MET: 36 of 36 positives routed to `plate` with the plate exactly as typed, in all nine plate shapes including the shape that matches no registry entry. P4 MET: median 4.8 s, p95 6.1 s. **P2 NOT MET: 15 of 24 plate negatives were routed to `plate`** (camera-sighting, count, owner and "plates containing" questions). Reading: the model took "a plate is in the question" as the subject, although the instruction says the question must ask about photos, images or videos. A wrong route here is costly: the image search would answer "no photo shows it" for a plate that the camera table does record.

**Change (general, no sentence of the bank is quoted).** The plate value now states what makes it that value and what does not: it is chosen only when the message itself asks to look in photos, images, pictures, video or footage; a plate that appears in a message about sightings, cameras, times, counts, owners, registrations or other records is `vehicle_records`. The cross-reference inside the `vehicle_records` line is replaced by the same statement from its side, and the plate value moves to sit directly after `vehicle_records`, so the two are read together. The sentence "Only the first four are searches" is reworded to cover the five.

**Procedure.** Plate bank A (now development, rerun and reported). **Plate bank B (`bank-plate-B-20261111.json`: new seed, new wording, 36 positives and 24 negatives) was built before this change and is run once for the P1, P2 and P4 gates.** P3 is measured on development bank 2 as stated. If bank B misses, the miss is reported as a miss and the switch for the plate value stays off; the keyword route keeps running as before.

## Amendment 5 (2026-11-10, after the second plate run on development bank A; thresholds unchanged)

**Result of run 2 (wording change of Amendment 4) on bank A (development).** P1 34 of 36 (94.4%, MET; two sentences went to the written-in-images value instead of the plate value), P2 **6 of 24 false routes (NOT MET; was 15)**, P4 median 4.7 s MET. The remaining false routes are two sentence shapes: "when was the plate ... last seen" and "which vehicles have a plate containing ...". Bank B (sealed) has not been run.

**Change.** The reply gets one more field, `a_media_words`, first in the reply (the grammar emits fields in alphabetical order, and this name sorts before `about`; the order is deliberate and tested): the words of the message that name photos, images, pictures, video, footage or stills as the place to look, copied exactly, empty if the message names none. The model must quote before it chooses a subject. The server then refuses the plate value unless the quote is non-empty, is a verbatim part of the message (same rule as the literal: stands alone, case and spacing ignored) and is not the plate itself. No list of media words exists in code; the model supplies the quote and the server checks that it is really in the sentence. The value is ignored for every other `about`, so word searches change only through the extra field in the reply.

**Procedure.** Bank A rerun (development, reported); development bank 2 for P3 (reported against the 2026-11-10 `ti-tier1-dev2c` numbers); bank B run once for the gates. If bank B misses P1, P2 or P4 the plate value stays off in the live switch and the keyword route continues as before.

## Amendment 6 (2026-11-10, after run 3 on development bank A; thresholds unchanged)

**Result of run 3 (media quote of Amendment 5) on bank A.** P2 **MET: 0 of 24 false routes** (was 6). P4 MET (median 6.1 s, p95 7.7 s; the extra field costs about 1.4 s). P1 **86.1% (31 of 36), NOT MET**: three misses are the sentence "Show me every photo of ..." where the model quoted "photos" for the word "photo" and the server's verbatim check refused a correct route; two misses are "list the images containing <plate>", which the model read as a search for the written text in images (`written_in_images`), a legitimate other reading that is left as a miss.

**Change (server check only; no model change).** The quote is grounded if it stands alone in the message as written, or if the quote with up to two trailing letters removed (at least four letters left) starts a word of the message. This accepts the model's singular/plural and tense variants of a word that really is in the sentence, and still refuses a word that is not (a quote of "photographs" for a sentence about when a plate was last seen is refused). The quote still must not be the plate.

**Procedure.** Bank A rerun (development, reported); development bank 2 for P3; bank B once for the gates.

## Results of Amendments 3 to 6 (2026-11-10)

| Run | Bank | P1 positives routed with the plate as typed | P2 plate negatives routed | P4 median / p95 |
|---|---|---|---|---|
| 1 (first wording) | A (development) | 36 / 36 | **15 / 24** | 4.8 s / 6.1 s |
| 2 (Amendment 4 wording) | A | 34 / 36 | 6 / 24 | 4.7 s / 5.5 s |
| 3 (Amendment 5 media quote) | A | 31 / 36 (three refused by the verbatim quote check, repaired by Amendment 6) | **0 / 24** | 6.1 s / 7.7 s |
| **Sealed, run once (Amendments 5 and 6)** | **B** | **30 / 36 = 83.3%, gate 90%: NOT MET** | **0 / 24: MET** | 6.2 s / 7.9 s: MET |

* P3 (development bank 2, Amendments 5 and 6): false routes on the 50 plain negatives 0 of 50 (MET); routed to the named family 49 of 57 = 86.0% (the earlier run: 84.2%, so it did not fall); routed at all 66 of 75 = 88.0% (earlier 82.7%); five word-bank questions were called a plate question by the model and all five were refused by the server (none reached the plate search). Model time median 5.7 s, p95 8.9 s.
* The sealed bank misses by 4 sentences. Five of the six misses are the model reading "photographs showing <plate>" or "image evidence show <plate>" as a search for text written in images (`written_in_images`); one is a literal with an extra word ("gari <plate>", a wrong key, which would have searched the wrong string; the literal-in-question rule cannot see that).
* For comparison, the keyword route (English only: needs a subject word such as plate or vehicle and a media word) applies to 12 of 36 positives on each bank with the right key, and 0 of 24 negatives. The model route is 30 of 36 with the same 0 false routes. It is the better router, and it still misses its own gate.

**Decision under the pre-registration.** A miss is reported as a miss. The plate value of the step ships in the code (tested), the switch `FORENSIC_TEXT_INTENT` stays off in compose, and the keyword route keeps running, now with plate extraction that accepts any format. Turning the switch on is the owner's decision with these numbers, because it adds one model call (median about 6 s) to every eligible question and its word-search gate G1 (86.0%, gate 90%) is also unmet.

## Decision (2026-11-11): the switch is ON live, on the owner's word

The gates were not all met (sealed plate bank P1 83.3% against 90%; word-search G1a 86.0% against 90%), and the pre-registration had said the switch stays off in that case. The owner decided, with these numbers in front of them, to turn it on: "Turn it on". It is applied to the live API (image `plate-search-20261110c`, `FORENSIC_TEXT_INTENT=true`; compose default flipped to true so a recreate keeps it). The numbers above stay as the record; they are not revised.

Checked live straight after: "Check the footage for SYNT9150" (no word "plate") is routed to the plate image search (10.2 s); "Which photos show the plate SYNT4O24?" is routed (6.4 s, one similar reading); "How many evidence files are in this case?" is not routed and answers `N: 90` (18.6 s, of which the intent step is about 4 s); "When was the plate SYNT4O24 last seen?" is refused by the media-quote check (`NO_MEDIA_NAMED`) and answered by the camera engine (1 sighting at 1 camera, 65.8 s).

Cost to expect: one model call (about 4 to 10 s on this laptop) before each eligible question. Undo: `.\tools\dev\live-window\3-recreate-api.ps1 -Apply -To plate-search-20261110c -RollbackTo plate-search-20261110c -Set FORENSIC_TEXT_INTENT=false`, restart the workspaces, and set the compose default back to `false` so a later recreate does not turn it on again.

## Amendment 7 (2026-11-12): questions about videos and their readings are no longer claimed as searches

**Evidence (live, 2026-11-12, after the media views went in).** Of 36 free-form questions about five real clips, 6 (17%) never reached the SQL engine because this step claimed them: three asked on how many frames of a named video a plate was read (the model proposed the word "plate", or the whole file name, as the plate or as the words to find), and three asked which video is the longest, which have speech, which has the most text visible (the model proposed "longest", "speech", "text visible" as the words to find). With the original instruction, on ten such questions the proposals were: `plate`, `owner-sample-4k.mp4`, `longest`, `speech`, `text visible`, `clip`, `longest video`: words the instruction already says to leave out.

**First attempt, reverted.** A new value `media_readings` ("a question about the videos, photos or recordings, or what was read from them, that types no plate and no words to look for") on the video development bank (`bank-video-20261112.json`, 56 questions about videos that must not be routed, 24 typed plates, some naming a file): 0 of 56 routed, but typed plates 17 of 24 (run 1); after a second sentence 23 of 24 (run 2). On plate bank A: P1 30 of 36 (run 3: 31 of 36), P2 0 of 24. On development bank 2 (P3): routed to the named family **44 of 57 = 77.2% (run 3: 86.0%)**, routed at all 53 of 75 (run 3: 66), false routes 0 of 50: the new value took 12 ordinary word searches ("where does X appear", "check whether X shows up in any file", "find the audio where ..."). P3 says no regression on words, so the change was **reverted**; the instruction is again the one of Amendment 6.

**Final change (server side only; the model, the instruction and the schema are unchanged).** The model proposes, the server disposes:
1. A plate, or the words to find, that lies inside a file name of the question (`pexels-5293898-hd.mp4` gives `5293898-hd`), is refused (`LITERAL_IN_FILE_NAME`); the name of a video, a recording or a picture is never words to find, and never a plate.
2. A literal made only of words that name the kind of source or what is asked of it (a closed set of 50 words: video, clip, footage, recording, speech, photo, image, frame, plate, text, longest, most, first, last, total, duration, distinct, visible, heard, said and the like; for a plate also vehicle, car, number, registration) is refused (`LITERAL_DESCRIBES_THE_REQUEST`). A literal with any other word in it is not touched.
3. A question that names a video, a recording or a picture by its file name is not put to the model (the case-wide text search cannot be scoped to a file by name; the engine can).

**What this costs the measured banks.** Replayed on every recorded proposal, rules 1 and 2 would refuse 0 of the 53 routed positives of development bank 2 and 0 of the 30 of plate bank A; none of the 522 questions of the five banks (development, sealed B, the unrun holdout) names a media file, so rule 3 changes nothing for them. The guards only remove routes. With the original instruction and rules 1 and 2, the video bank gave 3 of 56 negatives routed (each names a media file, which rule 3 now covers) and 23 of 24 typed plates (the miss: a plate typed after "look through the footage for", refused for a media quote that is not in the sentence: an existing check). Plate bank B (sealed) and the holdout were not run and are unchanged.

**Result.** Recorded with the sealed free-form video bank in `docs/data/video-questions-20261112.md`.
