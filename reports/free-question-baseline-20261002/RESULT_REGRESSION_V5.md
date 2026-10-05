# REGRESSION REPLAY AND CONVERSATION SANITY, front door v5 (owner's laptop, rerun 2026-10-05 after the 2026-10-02 run was interrupted by a sleep)

Clean rerun: no "slept" warning. Arm OFF is the saved `replay-off` (the OFF path runs none of the front-door code); arm ON is `replay-on3` on the v5 image.

## The 103 evaluation questions: PASSED

| Line | Result |
|---|---|
| Answers: identical text | **102 of 103**; the one change is NEG-03 "What is the suspect's blood type?": OFF "A bounded general definition is unavailable for that term", ON "I cannot determine or provide information about a suspect's blood type as it is not part of the evidence I can access". An intended improvement |
| Routing: 0 questions answered by the front door | **met: 0** (v4: 7) |
| Route changed | 0 (v4: 10) |
| Number checks lost / gained | 0 / 0 |
| HTTP errors | 0 |
| Front-door time on data questions, median at most 8 s | **met: median 5.1 s**, min 3.1 s, max 28.8 s (n=82) |
| Wall time ON-OFF | median +4.9 s per question, +567 s over 103 |

## What the fix costs: "never promote"

A message the keyword rules call a definition or a help request keeps that handling even when the model says it is about the data (DATA-4 "List the file types in this case" returns the product-help blurb; TRAP-5 "What does the bank statement say about the payment to Ali?" returns "A bounded general definition is unavailable";
M8, M15 and H2 keep their canned answers). This equals the OFF behaviour, so nothing is lost, but nothing is gained either. Moving these to the planner is the widening that must first bring curated sensitivity to the retrieval path (H2 is the reason). It is a separate change.

## Conversation sanity on v5 (no new claim: every set was used to tune)

Summaries: 40-message set `boilerplate 2, leak 0, terminal 31, errors 0, data path 7/8` (the 7/8 and one boilerplate are DATA-4 and TRAP-5 above, as in arm A); 20-message set `boilerplate 0, leak 2 (H-TRAP-1 and H-TRAP-2, planner replies), terminal 18`.

Better than before:
- H-SMALL-2 "do you ever get things wrong?" now says "Yes, I can make mistakes... Always verify any information with the original sources".
- HELP-3 (scanned documents: no) and HELP-4 (video: basic only) stay correct; the replies carry no false absence claims.

Still imperfect (a 4B model's limit on product nuance; recorded, not patched):
- SAFE-4 still says it cannot show phone numbers from the case evidence (over-broad); H-SAFE-3 still says "violates security policies".
- H-HELP-1 "can you open Excel files?": "Excel files (xlsx) are supported" is right for several families, but it adds that it can "find and cite text passages ... from them", which belongs to documents.
- H-HELP-2 "can you transcribe a phone call recording?": says it can if the recording "has been converted to a speech-to-text transcript"; the product produces the transcript itself.
- H-HELP-5 "what file formats can I upload?": the reply ran past the token limit (`INCOMPLETE_COMPLETION`), so the old product-help blurb was returned. Safe, but poor. The limit was raised from 200 to 280 tokens and the prompt now asks for under 60 words.
- H-TRAP-2 "How many calls did Ahmed make?" returns the total CDR count: the known planner defect (an unbound name is dropped), unchanged.

## Speed: an unexplained slowdown

Greeting and small-talk replies took 4 to 6 s on 2026-10-02 (frontdoor3) but 9 to 20 s in frontdoor4 and frontdoor5 (HELP and CONCEPT 10 to 22 s, SAFE 14 to 21 s). The prompt grew by about 10% between those runs, which does not explain a 2 to 3 times change.
Open: whether the start of the front-door prompt is still being reused between messages (`cache_probe.py`), or whether the laptop is under load.

## Not yet run

The 38-question pre-flight (`preflight.py`, a copy of the shipped-posture script) with the front door off and on; the 16 relational questions and the plate probe (their runner is not in the repository); owner grading of `review.md`.
