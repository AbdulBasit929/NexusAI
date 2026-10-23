# Link analysis, stage 3: the demonstration (2026-10-23)

The pre-registration (`link-analysis-stage3-preregistration-20261022.md`, section 10) ends the stage with a repeatable demonstration: on one loaded wave-1 case, in the interface at `:4181`, six questions, each answered with the derivation in plain words, the caveat, the table twin and the map or chart that fits the rows, and the same six asked of a pair or a number that has no result.

**Case:** `nexusai-wave1-dev2-20261021` (development case 2, loaded and typed on the live system; synthetic). **System:** API image `a48f2ff888fd` (code 7499baf), asked through the interface proxy at `http://localhost:4181`, one conversation, the question box. **Truth:** the generator's oracle for the same case, computed before the questions were asked (`tools/ask_eval/stage3bc_bank.py` `World`, `oracle.a5_colocation`, `a7_meetings`, `a9_successors`, `a10_movement`, `a11_rate_change`).

## 1. The twelve answers

| # | Question (numbers shortened to their last three digits) | Truth (oracle) | The interface answered | Values used, as the answer states them |
|---|---|---|---|---|
| 1 | When were 007 and 011 at the same cell within 22 minutes? | 4 windows at one cell | "4 positions at 1 cell site." Table: cell, end time, latitude, longitude (4 rows); a map tab | Co-location window 22 minutes (from the question) |
| 2 | Which of the times 003 and 007 at the same cell within 30 minutes could have been a meeting, given a call or SMS in the previous 90 minutes? | 2 possible meetings | "2 positions at 1 cell site." | Co-location window 30 minutes (from the question); the 90-minute gap is **not shown** (see 3.3) |
| 3 | Which numbers may have taken over from 005? | 001 at 0.7905 | "New number 923000001001; Score 0.7905." Table: new number, score (0.791) | Silence 7 days and start window 14 days (defaults); the lowest score (35 percent) is **not shown** (see 3.3) |
| 4 | Did 009 move house? Date and distance. | 2026-03-09, 6.29 km | "To cell key ?:10156:10444; From cell key ?:10286:10814; First week start 2026-03-09; Distance km 6.29." | Hold 2 weeks (default) |
| 5 | Which consecutive events of 002 imply travel faster than 200 km/h? | 1 pair, 2026-02-08 09:18:00 to 09:18:11 local | The pair, 2.224 km in 11 s, 727.8 km/h, times shown on the case clock | Lowest speed 200 km/h (from the question) |
| 6 | Did the daily activity of 006 change for good? From which day, averages before and after? | 2026-03-04, 8.233 to 18.267 a day, up | "First day 2026-03-04; Mean before 8.233; Mean after 18.267; Ratio 2.219; Direction up; Days 60." | Least ratio 150 percent (default) |
| 7 | (1) for 000 and 001, who never meet | none | "No row matches what you asked." | Window 22 minutes |
| 8 | (2) for 007 and 011, co-located at 30 minutes but with no call or SMS before | none (the decoy) | "No row matches what you asked." | Window 30 minutes |
| 9 | (3) successors of 011, silent with no candidate | none | "No row matches what you asked." | Silence 7, window 14 |
| 10 | (4) did 006 move house, who never moves | none | "No row matches what you asked." | Hold 2 weeks |
| 11 | (5) speed alerts of 007 at 200 km/h | none | "No row matches what you asked." | Lowest speed 200 km/h |
| 12 | (6) did the activity of 011 change, a steady number | none | "No row matches what you asked." | Least ratio 150 percent |

**All twelve are right against the oracle.** Every answer carried the caveat of its analysis ("Proximity is not association ... no chance baseline", "A candidate, not a finding", "Behavioural similarity, not identity", "Cell sites, not positions of a person", "A change in the pattern of use, not an explanation") and the line that says which rule values were used and whether they came from the question or were the default. Answers took roughly 20 to 60 seconds each on the laptop (read from the pane; not timed to the second).

## 2. What the interface showed besides the sentence

* A table twin with the rows (compact and comfortable density, a filter box, "Export current result"), and a map tab for position-shaped results (here one dot: the generated cases sit on the ocean grid, decision D16, so the offline basemap is empty by design).
* "Evidence", "Copy", "Compare" and "Ask a variation" under each answer; "Sources: None openable" on every answer of the engine (the rows are not linked to openable source rows).

## 3. What the demonstration shows is not good enough (findings for the interface programme, none of them an accuracy fault)

1. **Wrong headline for windows and meetings.** The co-location and meeting answers read "4 positions at 1 cell site" and "2 positions at 1 cell site": the headline of the *map* shape, not of the question, which asked *when* and *whether they could have met*. The periods (start, end) are only in the table, and at the pane's width the start and end columns are off the edge. The answer should lead with the periods ("4 times between 2026-02-09 and 2026-03-03, at one cell site") and offer a timeline.
2. **"Supported finding / A verified finding is available" above "No row matches what you asked."** An empty result is labelled as a finding. It needs its own designed state ("No result within these limits", with the limits and what to loosen).
3. **The last parameter note is not on screen.** The API returns the line for every rule value used (for a possible meeting: the 90-minute gap; for a successor: the lowest score of 35 percent), but the page shows five or six limitation lines and the last one is missing. The derivation gate is met at the API; the page hides part of it.
4. **A column labelled "Call direction" holds "up"** (the change-of-rhythm direction): a generic column label for `direction`.
5. **Raw cell keys** ("?:10286:10814") in the move answer, no map of the two places, no distance drawn: the result of a move is two sites and a line, and the page gives four text columns.
6. **No evidence behind a mark.** "Sources: None openable" on all engine answers: nothing opens the rows behind a window, a candidate, a move or an alert, which the workbench principle "every mark opens its evidence" requires.
7. **The case header shows a date range ending in 2065** ("2026-02-01 to 2065-01-24") for a case whose events end on 2026-04-02; a stray timestamp in some source is displayed as the case's period. A data-quality flag should say so.

These seven are the first input of the workbench design (`docs/ux/workbench/`); the accuracy of every answer above is not in question.

## 4. Repeat it

Ask the twelve questions in the case's question box at `http://localhost:4181/cases/nexusai-wave1-dev2-20261021/investigate` (Alt+A focuses the box), or run `python -X utf8 tools/ask_eval/stage3bc_bank.py --stage 3b --register-case --seed 2` and the other banks for the same questions against the oracle. The list of limitation lines the API returns for question 2 has six entries (the computed-by note, the repeats note, the cell-site note, the candidate caveat, the window used, the gap used); the page shows five, without the gap.
