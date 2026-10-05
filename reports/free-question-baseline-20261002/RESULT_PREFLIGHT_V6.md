# PRE-FLIGHT (38 questions) AND CACHE PROBE, front door v6 (owner's laptop, 2026-10-05)

Image built from commit 2d40934 (reply limit 280 tokens, under 60 words). Pre-flight is `preflight.py`, a copy of the shipped-posture script whose every question carries its own expected substrings.

## Pre-flight: both arms pass

| Arm | Result |
|---|---|
| Front door OFF | **PASSED 38 of 38** |
| Front door ON | **PASSED 38 of 38** |

The per-answer difference listing (step 4 of the script) was lost in the pasted console output, so "no answer text differs between the arms" is **not verified**; both arms met every expected substring.

## Prefix reuse works; a cold start is expensive

Six greetings back to back with the front door on:

| Call | Front-door time |
|---|---|
| 1 "Hello" (after the container was recreated) | **43.4 s** |
| 2 to 6 | 5.8, 6.4, 7.1, 5.9, 6.8 s (median 6.4 s) |

So the fixed start of the prompt (about 1,000 tokens) is read once and reused. The earlier slow runs (9 to 20 s for greetings) were not a cache failure: reply length matters (about 7 tokens per second, so a 70-token reply costs about 10 s) and the machine state varied (those runs followed 15-minute replays).
The cost to plan for: the first message after a restart, or after a question that had to generate a plan, pays the read again (measured 25 to 43 s). A shared prompt start for all call types (`L1B_SHARED_PREAMBLE_SPEC_20261002.md`) is the fix for that.

## 20-message set after the token-limit change

H-HELP-5 "what file formats can I upload?" now completes (ANSWERED, 21.5 s, lists formats) instead of falling back to the old blurb. H-SMALL-2 "do you ever get things wrong?": "Yes, I can make mistakes. Always check the sources for case details to verify accuracy."
Unchanged imperfections: H-HELP-1 (Excel) and H-HELP-2 (call recordings) are muddled; see `RESULT_REGRESSION_V5.md`.

## Pre-registered lines, status

| Line | Status |
|---|---|
| 103-question regression | passed (v5) |
| Pre-flight 38 of 38, off and on | passed |
| Front-door time on data questions, median at most 8 s | passed (5.1 s); cold start 25 to 43 s |
| Conversation quality (owner grading of `review.md`) | pending |
| 16 relational questions, plate probe (runner not in the repository) | not run; the pre-flight covers six plate and OCR searches |
| Default | stays off; enabling it is the owner's decision |
