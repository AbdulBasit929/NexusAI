# Gold-set corrections — v2

**Date:** 2026-09-24 · `evaluation/golden_questions_v2.json` supersedes v1.
**v1 is retained unchanged.** Every measurement before today was taken against it, and
overwriting it would have made those numbers incomparable — the same instrument-integrity
rule that WI-6 established after three runs were found mis-measured.

## Why an oracle needed correcting at all

Three separate places rewarded MACHINE VOCABULARY or accepted an answer that never reached
the analyst. Each was found by investigating a result that looked right, not by reading the
oracle.

### 1. Raw column values where the product forbids them — CDR-13, CDR-05

    CDR-13  was ["INCOMING","2906","OUTGOING","2592"]  now ["Incoming",...,"Outgoing",...]
    CDR-05  was ["GPRS","5863","VOLTE","739"]          now ["Data session",...,"VoLTE call",...]

UX §9 forbids showing an analyst a raw value, and the curated layer names these
`Incoming`/`Outgoing` and `Data session`/`VoLTE call` (`semantic_layer/cdr.yaml`). WI-13 made
the answers use the display names. The v1 expectations still passed, because the raw values
survive elsewhere in the response payload — so they rewarded the machine vocabulary and could
not have detected a regression back to it.

### 2. An expectation satisfied by echoing the question back — DOC-01

    was ["MN1367"]   now ["nexusai-multimodal-acceptance-brief.pdf"]

"Search the case documents for mentions of plate MN1367" was answered *"There are no ANPR
sightings involving MN1367 in this case"* — which contains `MN1367` and therefore passed,
while answering about ANPR rather than documents.

**Verified against the data before changing it:** MN1367 appears in exactly one document
passage, in `nexusai-multimodal-acceptance-brief.pdf`. That filename is what a correct
document search returns and **cannot be produced by echoing the question.**

### 3. Filenames matched anywhere in the response — harness change

The `contains` check searched the whole response blob, where a filename appears in coverage
and provenance listings whether or not retrieval found anything. **AUD-01 and AUD-03 scored
CORRECT this way while the analyst was shown nothing** — both were false CORRECTs until
WI-12 fixed the underlying presentation defect.

Filename expectations now must appear in the ANALYST-FACING TEXT. A filename the analyst
never sees is not an answer.

**Scoped to filenames only, and that scoping was itself a correction.** The first attempt
required every element of such a question verbatim in the text, which failed CDR-16: the
answer reads `seed_cdr_large.csv: 5,000` while the oracle writes `5000`. A number is
legitimately formatted for reading; requiring it verbatim in prose fails correct answers
over punctuation.

## What the corrections revealed

Running the affected questions on v2 exposed two defects v1 could not see:

- **DOC-02** — the document answer quoted the passage but never NAMED the document
  ("across 1 document"). Same defect images had in IMG-01. **Fixed**: document answers now
  name the source, one or two outright and the first-plus-count beyond that.
- **DOC-01** — genuinely answers about ANPR rather than documents. **Not fixed; now visible.**

## The honest consequence

**Phase 1's "confident-wrong = 0" was measured partly on a flawed oracle.** On the corrected
oracle, DOC-01 is a confident-wrong answer. The defect was always there; v1 could not see it.

Correcting the instrument and losing a number is the right trade — this project has already
paid once for the opposite, when three runs were graded by a mis-measuring harness. The
number was never the goal.
