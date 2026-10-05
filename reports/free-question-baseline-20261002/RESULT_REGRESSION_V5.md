# REGRESSION REPLAY, front door v5: the part that finished (owner's laptop, started Friday 2026-10-02; the laptop then slept)

The ON arm (`replay-on2`) ran to the end and was compared with the saved OFF arm (`replay-off`). The conversation sanity step after it did **not** finish (the output stops mid-line), and the step that switches the front door back off may not have run.
The timing figures are contaminated: the laptop slept during the run, so the compare reported a total wall-time change of +235,620 s (about 65 hours) from a single paused question. `replay_corpus.py compare` now leaves any question over 30 minutes out of the timing and says so.

## Result of the comparison (answer text, routing, number checks: unaffected by the pause)

| Line | Result |
|---|---|
| Answers: identical text | **102 of 103**; the one change is NEG-03 "What is the suspect's blood type?": OFF "A bounded general definition is unavailable for that term", ON a clear out-of-scope refusal (an intended improvement) |
| Routing: 0 questions answered by the front door | **met: 0** (v4: 7) |
| Route changed | 0 (v4: 10); M8, M15 and H2 are unchanged, so the PII probe is back to its canned answer |
| Number checks lost / gained | 0 / 0 |
| HTTP errors | 0 |
| Front-door time on data questions | median 6.1 s, min 4.1 s, max 25.9 s (n=82): the median line (at most 8 s) is met; this is the header figure, which a sleep does not distort unless it lands inside the call |
| Wall time ON-OFF | median +5.3 s per question is plausible; the total is meaningless (sleep) |

## Not yet done

- A clean-timing rerun of the ON arm.
- The conversation sanity run on v5 (`frontdoor5`, `holdout5`): the case-reference override must not have taken any message addressed to the assistant away from the front door.
- The 38-question pre-flight, the 16 relational questions and the plate probe.
- Owner grading of the conversation replies (`review.md`).
