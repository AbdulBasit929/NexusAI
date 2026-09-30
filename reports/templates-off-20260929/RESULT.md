# TEMPLATES OFF -- WHAT THE RUNTIME QUERY PATH ANSWERS ALONE -- RESULT

Census, 2026-09-29. Image de5c372e1b70, every switch as shipped, with `FORENSIC_LADDER_ROUTING=false`.
Compared with the same image with the ladder on (`reports/count-names-field-20260929/on`).
Comparator: `scripts/nexusai_templatesoff_compare.py`, output in `compare.txt`. No ship decision.

## Headline

| | Ladder on | Ladder off |
|---|---|---|
| Correct (of 103) | **70** | **68** |
| Confident wrong | 2 | **4** |
| Clarified or refused | 29 | 29 |

Where the correct answers came from:

| Path | Ladder on | Ladder off |
|---|---|---|
| LLM-written plan (checked, then run on the database) | 30 | 27 |
| Compiler plan built at run time | 25 | 28 |
| Registered operation ("template") | 11 | 9 |
| Terminal (definitions) | 4 | 4 |

**Turning the keyword ladder off does not remove templates.** The semantic planner still selects
registered operations (`semantic_deterministic_registered+explicit_template`) for 25 questions, and 9
of those are correct. Retiring templates means the runtime planner must be able to express what those
operations do. Switching the ladder off does not achieve that.

## Must be closed before templates can be retired: 2 new confident wrong answers

| Question | Ladder on | Ladder off |
|---|---|---|
| CASE-01 "How many records do we have for each record type?" | refused, correctly | `call_type_breakdown`: "8,642 CDR records across 5 call type values", which answers a different question |
| X-01 "Where does 03001234567 appear across all evidence?" | refused, correctly | `entity_timeline`: "No matching records were found for 03001234567", a **false absence**, because the number is in a document (DOC-02) |

Both come from the semantic planner choosing the wrong registered operation once the ladder no longer
claims the question first.

## Work order: answers only a template gives

With the ladder off, these are still correct only through a registered operation:

| Kind | Questions | What the runtime planner needs |
|---|---|---|
| Transcript search | AUD-01, AUD-02, AUD-03 | a governed "text contains" filter over transcript segments, with time locators |
| Document search | DOC-03, DOC-06, and DOC-01/DOC-02 (lost with the ladder off) | the same filter over document passages, with page locators |
| Image-text search | IMG-01 | the same filter over OCR regions, with region locators |
| Frequent contacts | CDR-10 | "counterparty", meaning the caller or receiver that isn't the target, as a groupable field (roadmap D2) |
| First and last seen | ANPR-03 | MIN and MAX of time with a target filter. The plan language has this already (CDR-09), so the registered operation just claims it first |

Text search is 6 of the 9. One governed text filter in the plan language would move the largest share.

## Instrument defect found and fixed during this run

The arm runner re-read `.env` to restore the shipped posture, but `.env` doesn't define
`FORENSIC_LADDER_ROUTING`. The arm's `false` stayed in the process, and the service was left running
**ladder off** after the census. It was caught by asserting the container after the run. The service
was redeployed with the ladder on and asserted (30 switches on, 9 off, image de5c372e1b70), and the
pre-flight passed 38/38 (`reports/count-names-field-20260929/shipped-preflight/`). Rule for future
runners: **assert the container after the last arm, not only before each one.**
