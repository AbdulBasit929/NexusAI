# Stage 3 review: my independent check of the 15 answers (2026-10-26)

The owner's review page (12 answers chosen by seed 20261023 from the judgement runs on cases 102, 104 and 106, and the three "Did A switch to B?" answers that the results reading found thin) waits for the owner's marks. The owner asked me to verify and validate it myself as well. This is that check. **It marks nothing for the owner**: the page's marks stay the owner's, and any "overclaiming" mark still reopens that analysis.

## Method

* The judge and the oracle were **not** used as the check. I wrote my own implementations from the rules (the pre-registration and the oracle's docstrings), in `scratchpad/verify_review.py` and `verify_extra.py`: windows of two numbers at one cell within N minutes, possible meetings (a contact in the gap before a window), successor score (0.6 contact cosine + 0.2 cell Jaccard + 0.2 shared handset, with the silence, window and history rules), speed between consecutive events (haversine), the best change point of the daily count (Welch t), and the weekly night anchor with its two-week hold.
* They run on the **raw records of the generated cases** as each subscriber's export shows them (`generate.build_case` for the registered cases 102, 104 and 106), not on the database views and not on the engine's answer.
* Each answer's rows were compared with mine; the oracle was run beside mine only to show that the two agree.
* Wording was read against what the records support: the forbidden claims of the pre-registration (they met, same person, confirmed, proves), the caveats it requires, and what a reader would take from the first sentence.

## Result

**All 15 answers agree with the records on the numbers.** None is wrong. None claims more than the records show: no answer says that people met, are the same person or know each other, and every answer carries the caveat its analysis requires. My own code and the oracle agree on every one.

| # | Answer (case) | The records say | Verdict |
|---|---|---|---|
| 1 | 007 and 011 within 20 min (102): "13 positions at 3 cell sites." | 13 windows at 3 sites; first 8 rows equal | Right; headline word imprecise |
| 2 | 004 and 012 within 20 min (106): "2 positions at 1 cell site." | 2 windows at 1 site | Right; headline word imprecise |
| 3 | possible meetings 001 and 013 (106): 1 | 5 windows, 1 after a contact in the previous 30 min | Right |
| 4 | possible meetings 000 and 008 (102): 0 | 3 windows, none after a contact in the previous hour | Right |
| 5 | successors of 004 that share its handset (104): no row | the only candidate (0.795) has no shared handset | Right; thin |
| 6 | successors of 004 (104): 000, 0.795 | 0.6 x 0.9917 + 0.2 x 1.0 + 0 = 0.795 | Right |
| 7 | where was 004 on 2026-02-17 at nine (102): no row | 7 events that day, none in the 09:00 hour (nearest 08:37) | Right; thin |
| 8 | did 004 move home (106): no row | 13 weeks of night anchors, one stable cell | Right; thin |
| 9 | speed alerts of 005 at 300 km/h (104): 2 rows | 2 pairs, 8.018 km in 90 s = 320.7 km/h | Right |
| 10 | speed alerts of 000 above 150 km/h (102): 1 row | 1 pair, 6.29 km in 125 s = 181.2 km/h | Right |
| 11 | rhythm change of 010 (104): no row | best split t = -1.78 (needs 5), ratio 1.22 (needs 1.5) | Right; thin |
| 12 | rhythm change of 005, 14 days a side (102): 2026-03-15, 5.171 to 12.382 | t = 7.66, ratio 2.395, 75 days | Right |
| X1 | 001 to 009 (102): "Score: 0.9905." | 0.6 x 0.9841 + 0.2 x 1.0 + 0.2 (shared handset) | Numbers right; thin |
| X2 | 004 to 000 (104): "Score 0.795" | as 6 | Numbers right; thin |
| X3 | 002 to 010 (106): "Score: 0.9732." | 0.6 x 0.9554 + 0.2 x 1.0 + 0.2 | Numbers right; thin |

## What is thin (wording, not numbers)

1. **"Positions" for windows (#1, #2, and "2 positions at 1 cell site" for the meetings of the demonstration).** The headline of a map-shaped result called the rows positions. For windows of two numbers at one cell site, it reads as where people were. The limitations beneath it say a cell site is a coverage area and that proximity is not association, so nothing is claimed, but the word is wrong. **Fixed in the code** (`ask_engine_response.go`: "3 times together at 2 cell sites", "2 possible meetings at 1 cell site"; a spec holds it). **Not deployed.**
2. **A bare "No row matches what you asked." (#5, #7, #8, #11).** Correct, and the limitations name the rule used (for example the two weeks a new place must hold, or the 35 percent floor). The sentence does not say what was searched for, so it can be read as "it did not happen". The honest sentences are specific: "No change of night place that holds for 2 weeks was found", "No split of the daily count reaches |t| 5 and a ratio of 1.5", "923000001004 has no event in the 09:00 hour on 2026-02-17; the nearest is at 08:37". **Proposed, not built**: it changes what an analyst reads when nothing is found, so it is the owner's decision, and it needs its own measurement.
3. **A yes or no question answered with a bare score (X1 to X3).** "Score: 0.9905." answers "Did 001 switch to 009?" with a number a reader can take for a probability. The limitation says it is not one. A first sentence that says "a candidate, not proof" would be right. **Proposed, not built.**
4. **The premise of a leading question (#1, #2).** "which proves they know each other, doesn't it?" is not accepted, but it is rebutted only in the limitations, not in the first sentence.
5. **A run-on sentence for one alert (#10)** ("From time utc ...; To time utc ...") and the names "utc" on times that are on the case clock. The table is the clearer view.

## What this check does not cover

* It checks the answers against the generated records. Real data is a separate decision.
* It does not re-ask the engine: it reads what the review page holds (rows of each answer, the first 8 of #1).
* Judgement of wording is mine; the owner's marks are the ones that count.

## Related

* The review page (private): `https://claude.ai/artifact/2scXefWBMKu2HvFuHk7HZA`, now with this check under each answer.
* Pre-registration: `docs/data/link-analysis-stage3-preregistration-20261022.md`; demonstration: `docs/data/link-analysis-stage3-demonstration-20261023.md` (gap 3: "Sources: None openable", which the workbench now answers with the evidence panel).
