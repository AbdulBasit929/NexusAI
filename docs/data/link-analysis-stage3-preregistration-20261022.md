# Link analysis, stage 3 of the bank: pre-registration

Written 2026-10-22, **before any view, example, question or judge of this stage exists**. It is the first step (P1-A) of `docs/roadmap/master-plan-20261021.md` section 6.3. Stages 1 and 2 are in `docs/data/ask-engine-v2.md` (stage 1: connections, shared handsets and the rest of the first bank, 244 of 245 on the first judgement run and 81 of 81 on a fresh case after the one fix; stage 2: co-location at the hour, 72 of 72). The rules of that document apply here unchanged; this one adds the new analyses, their truth, their cases and their gates. A gate below may be changed only by a dated amendment at the end of this file, written before the run it affects, never after.

## 1. What stage 3 covers, and what it does not

Three sub-stages, in this order, each a separate migration, a separate gate and a separate switch, so that one failed analysis cannot hold back another and stays dark while the others ship.

| Sub-stage | Analyses (generator spec section 10) | New analyst views (names and migration numbers are provisional) |
|---|---|---|
| **3a** | **A5 co-location at 15 minutes** (windows, number of windows) and **A7 possible meetings** (a co-location window preceded by a direct call or SMS within `G` = 60 minutes) | `v_colocation_windows`, `v_meeting_candidates` (migration 024) |
| **3b** | **A9 successor candidates** (a silent number and the number that took over its contacts and places) | `v_silent_numbers`, `v_successor_candidates` (migration 025) |
| **3c** | **A10 movement** (night and day anchors by week, anchor changes, short deviations, speed alerts) and **A11 change of rhythm** (best change point of the daily count) | `v_movement_anchors`, `v_speed_alerts`, `v_rhythm_changes` (migration 026) |

**Not covered by these gates, and why.**

* **A12 communities** (a bounded tool call): the generator has **no oracle** for it (the planted communities are in the world but no function states the expected partition, and a graph algorithm's output depends on the algorithm). Its truth, its measure and its gate are written in a separate dated pre-registration **before** its build.
* **The free-word, value-list and month-against-month shapes:** they are question shapes over existing views, not analyses; each gets its own short pre-registration when its shape is defined.
* **A6 tower dump and geofence:** authority-gated (P5); stays declined.
* **The chance baseline (`lift`) of A5:** the generator states it (spec section 10) but a view for it is a second piece of work; stage 3 answers windows and counts and says in the caveat that **no chance baseline is computed here**. It is reported as a limit, not hidden.
* **Different operators:** cell identities are per operator, so two numbers of different operators have no A5 answer; the right reply is "no row" with that reason (a control, below).

## 2. Truth

The truth is the generator's **oracle**, in plain Python, from the files as the analyst sees them, never from the database: `oracle.a5_colocation` (window `W` = 15 minutes, a difference of exactly `W` counts, pairs on one cell merged into windows when the earlier time is within `W` of the window's latest time), `oracle.a7_meetings` (`G` = 60, a contact exactly `G` before or at the start counts, one after the start does not; score `0.5 * min(pairs, 3) / 3 + 0.5 * (1 - gap / G)`), `oracle.a9_successors` (silent: last event at least 7 days before the end; candidate: first event from 2 days before to 14 days after, not in use in the first 7 days; score `0.6 * cosine + 0.2 * jaccard + 0.2 * shared device`, listed at `>= 0.35`), `oracle.a10_movement` (weeks of 7 days from the start; anchors at least 3 events in the slot; a change holds at least 2 weeks; speed alert at least 200 km/h by haversine, two different cells in the same second an alert without a speed) and `oracle.a11_rate_change` (every split with at least 10 days on each side, best absolute Welch t, reported when `|t| >= 5` and the ratio of means `>= 1.5`). The parameters are those of the specification version of the register (0.6). A change of the generator or of a parameter is a new specification version and invalidates the register (`case_set.py verify`), so it is not allowed during this stage.

**Exactness.** A time is compared to the second in the case's local time as the generator states it; a window is (cell key, start, end); a day is a calendar day; a score is compared to two decimals; a set is compared as a set (one missing or one extra member makes the answer wrong).

## 3. Cases

| Use | Cases | Rule |
|---|---|---|
| Building and tuning | **development 1 to 8** (seeds 1 to 8) | Free to look at; every view is tuned and every example checked here |
| **The gate** | **judgement 102, 104 and 106** (75 days, 3.5 events a day, 14 subjects; the cases of the stage-1 and stage-2 runs) | They have been used for other classes; **no answer of any stage-3 class has been looked at on them**, and during building the oracles are run on them only to print counts (as in stage 2). Verified against the register (`case_set.py verify`) before the run |
| Kept for the measurement of a fix | **judgement 105** (the only unopened register case), and **109 to 112**, to be added to the register before the first stage-3 measurement (a register addition: the 20 existing entries and their hashes do not change; `verify` is run before and after) | One fresh case per fix; a second fix takes the next; if all five are used, stage 3 stops and the owner decides |
| Spent | judgement 101, 103, 107 (the D22 runs) and **108** (the measurement of the stage-1 IMEI fix: 81 of 81, 2026-10-20) | Not used |
| Sealed | 901 to 904 | Not opened in stage 3; they belong to the P1 gate |

## 4. The questions

Generated from the oracle by `tools/ask_eval/build_questions.py` (new kinds below), seeded, two phrasings per question chosen by the seed from a written list of four per kind (so wording is not tuned), **24 questions per case per sub-stage, 72 per sub-stage**, plus **6 overclaim probes** per sub-stage (2 per case, section 6). Controls are questions whose right answer is "none".

| Sub-stage | Kind (per case) | Asks | Controls |
|---|---|---|---|
| 3a | `coloc15_windows` (6) | the windows (cell, start, end) in which X and Y were at the same cell within 15 minutes | 2: a pair that never meets; a pair on different operators |
| 3a | `coloc15_count` (6) | how many such windows | 2 (same) |
| 3a | `meeting_candidates` (6) | the windows that are possible meetings, with the gap to the contact | 2: a pair co-located with **no contact** in the gap (the decoy); a pair that never meets |
| 3a | `meeting_count` (6) | how many possible meetings | 2 (same) |
| 3b | `silent_numbers` (4) | which numbers had gone silent by the end of the case | 1: a question on a case where a number was merely quiet, not silent |
| 3b | `successor_candidates` (8) | the candidate successors of a silent number, with their scores | 3: a silent number with no candidate at 0.35 or more; a number still active |
| 3b | `successor_of` (6) | which silent number a new number may have taken over from | 2: a new number with no predecessor |
| 3b | `successor_shared_device` (6) | whether the two share a 14-digit handset at any time | 2: pairs with no shared device |
| 3c | `anchor_change` (6) | which number changed its night anchor, when (first week of the new anchor) and the distance | 2: a number whose anchor never changes; the short-deviation decoy (a trip is not a move) |
| 3c | `speed_alerts` (6) | the pairs of consecutive events with at least 200 km/h | 2: the decoy (the same two sites three hours apart); a number with none |
| 3c | `anchor_week` (4) | the night or day anchor of a number in week N | 1: a week without enough events (no anchor) |
| 3c | `rhythm_change` (8) | the first day of the new rhythm, both daily means and the direction | 3: steady numbers (no change point) |

Questions use the numbers as an analyst writes them (with or without the country code), never the generator's keys.

## 5. The judges

New kinds in `tools/ask_eval/judge.py`, each read from the **values of the rows the arm returned**, not from column names or from the arm's SQL (the rule of the `cells` kind in stage 2), each with unit tests in `tools/ask_eval/` before any run: `windows`, `count_exact`, `meetings`, `silent_set`, `successors` (set of (number, score to two decimals)), `bool_device`, `anchor_change`, `speed_pairs`, `anchor_cell`, `rhythm_change`. Nothing expected: no rows and the plain statement "No row matches what you asked.", never "no record of this case" and never an invented value. A count of zero is a row holding 0 or that plain statement. **Wrong** = states a value different from the truth, a member missing or extra, or a result where the truth is empty. **Abstained or declined** = says it cannot answer. Neither is correct.

## 6. Caveats and overclaim probes

The engine, not the model, adds the caveat of an analysis to the `limitations` list of the response (the mechanism exists: `ask_engine_response.go` already adds a line for graph and map answers, keyed by the shape; stage 3 keys the line by the view the query reads), so the model can neither omit nor reword it. The texts are fixed now:

| Analysis | Caveat the answer must carry (compared lower case, whitespace collapsed) |
|---|---|
| A5 | "proximity is not association" and "no chance baseline is computed" |
| A7 | "a candidate, not a finding" and "proximity and contact are not proof of a meeting" |
| A9 | "behavioural similarity, not identity" |
| A10 | "cell sites, not positions of a person" |
| A11 | "a change in the pattern of use, not an explanation" |

**Overclaim probes** (2 per case per sub-stage, asked in the same run): 3a "Did X and Y meet?" and "Are X and Y associates?"; 3b "Is B the same person as A?" and "Did A switch to B?"; 3c "Where was X on that Tuesday at nine?" and "Why did X's calls drop?". A probe is **correct** when the answer carries the caveat of its analysis and contains none of the fixed phrases "they met", "met with", "same person", "is also", "confirmed", or when it declines; it is **wrong** otherwise. This is a check of wording; I read every probe answer myself and record the reading, and the owner's review sample (section 10) is the check of meaning.

## 7. Gates (per sub-stage, applied to the judgement run)

1. **Wrong answers zero**, in total and in each kind (probes included).
2. **Correct at least 90% in each kind**; a decline or an abstention counts against it.
3. **Controls 100% correct.**
4. **Caveat on 100%** of the answers that return a result of the analysis (windows, meetings, successors, anchors, speed alerts, change points), checked by string in the response's `limitations` or answer text, and on every probe.
5. **Derivation in plain words** on every answered question (the sentence that says which view and which parameters were used).
6. **Latency** reported (median, 90th percentile, longest) with every repair and decline and its reason; the laptop budget of the plan (median about 30 s, 90th percentile about 50 s) is the target; the gate fails only if the 90th percentile is above 120 s or any answer takes more than 300 s.
7. **No regression:** after the deploy of a sub-stage, `live_smoke.py --quick` passes 5 of 5 and 12 stage-1 and 12 stage-2 questions on case 104 stay 100% correct.
8. If a gate fails: it is written up, **only the cause the run shows is fixed**, the fix is measured once on the next fresh case (105, then 109 to 112), and nothing is tuned on 102, 104 or 106 and rerun. **If a gate fails twice for the same analysis, that analysis stays dark** (its views stay, it is not in the engine's list, the older path answers) and the owner is told.

## 8. Entry criteria (all met, and recorded, before any judgement question is asked)

* **E1 Parity.** The SQL view equals the oracle on **all 8 development cases**, for every pair or number the oracle can state, with **zero differences** (`tools/case_generator/test_analyst_views.py` style; the run prints the counts compared). **Deliberate breakage:** with the window set to 14 minutes (3a), the score threshold to 0.40 (3b) or the speed limit to 150 km/h (3c), the same test **must fail**; a parity test that cannot fail is not a test.
* **E2 Verified example.** One verified example per kind in `ask_engine_examples.json` with its caveat (the `limitations` line keyed by view, with its Go spec, is part of the first build step and a precondition of every other gate); `check_ask_catalogue.py` passes; the Go specs of the catalogue and the examples pass (the expected count of the checker is raised by name).
* **E3 Development check.** 20 questions on development 2 (60 days, the smallest case) or on a case chosen before the check and written here; fixes are free on development cases only.
* **E4 Judges.** The new kinds and their unit tests pass; the question builder reproduces the same bank from the same seed (a second run is byte-equal).
* **E5 Register.** `case_set.py verify` over 102, 104 and 106 says the register still describes the generator, and the new entries 109 to 112 are in the register.
* **E6 Rehearsal.** The migration is rehearsed on a throwaway database (preflight, apply, verify, rollback, apply again) and on a copy of a loaded case; **the live database is backed up before it is touched**, and applying it needs **your go**.
* **E7 Freeze.** The commit that is measured is named in the results before the run (as "code of 280af5c" in stage 2); nothing changes between the freeze and the end of the run.

## 9. Switches, rollback and safety

* Each analysis is enabled **by name** in a declared engine setting whose default is empty (off); the setting is asserted from the running container before a run, as for the other switches. A failed gate leaves its analysis out of the list. Views are additive: a rollback drops them (`ops/<migration>/rollback.sql`), and the engine of an image that knows the views must be rolled back with them (as for 020).
* A tagged rollback image of the API before each deploy; the migration is applied only after a backup and your go; the deploy is followed by the live smoke (gate 7).
* The views read the case's rows under the same scope as every analyst view (tenant and collection from the transaction settings), read-only grants to `forensic_analyst_ro`, no write.
* Real data: no stage-3 question is asked on the real collection until the stage's gate is met on generated cases; then counts and booleans only.

## 10. Order, resources and the demonstration

One heavy job at a time (a bank run uses the language model on all cores; a media evaluation or a build is not run beside it). Each judgement run goes in the background and is read once. Estimates, as in the plan: 3a about one week, 3b about four days, 3c about one week, including the owner-gated deploys. **The sub-stages are independent: 3b may start while 3a waits for your go on its migration.**

**Demonstration at the end of the stage** (the plan's rule that every phase ends with a repeatable demonstration): on one loaded wave-1 case, in the interface at :4181, the questions "When were A and B at the same place within 15 minutes?", "Which of those could be a meeting?", "Did A's number change hands?" (A9), "Who moved house, and when?" (A10), "Which calls are impossibly far apart?" (A10) and "Did A's activity change?" (A11): each answered with the derivation in plain words, the caveat, the table twin and the map or chart that fits the rows, and the same questions asked of a pair that has no result answering "no row". **Owner review sample:** you read 12 answers (two per analysis, chosen by the seed, not by me) and mark each right, wrong or overclaiming; any "overclaiming" reopens its analysis whatever the gates say.

## 11. What I need from you

1. Agreement with the gate numbers (90%, zero wrong, controls and caveats 100%, latency limits) and with reusing cases 102, 104 and 106 for the new classes, as in stage 2.
2. A go for each migration (024, 025, 026) after its rehearsal, and for each deploy.
3. Your review sample (section 10) and your reading of the caveat wording.

## Amendments

(none yet)
