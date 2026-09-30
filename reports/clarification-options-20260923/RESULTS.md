# WI-11 — clarification options

**Date:** 2026-09-23 · **Contract:** `forensics.clarification-request/v2`
**Rollback:** `nexusai-forensic-records-api:rollback-before-clarify-options-20260923`
**Fixtures:** `fixtures/*.json` — 8 real v2 clarification responses for offline UI work.

## The problem

`clarification.options` was always `[]`. 18 of 62 golden questions now clarify (14 from
verified-only and arbitration, 4 more from WI-10), so nearly a third of all traffic ended
in a dead end: the analyst is told the question cannot be answered and handed nothing to do
about it. Abstaining was correct; offering nothing afterwards was not.

## Contract change

`options` is now `[{label, query}]` rather than `[]string`, and the version is bumped to
`/v2`. `query` is the exact question to re-send, so the UI needs no phrasing logic of its
own and cannot drift from what the compiler actually accepts. Wired at all five
clarification sites.

## Nothing is invented

| Source | Rule |
|---|---|
| Field and entity labels | curated `display_name`, SQL-verified (83 source_names, WI-4) |
| Offerable fields | `sensitivity: NONE` only — never what the sensitive-field guard withholds |
| Breakdowns | `groupable: true` only — the layer's own statement that grouping is meaningful |
| Media scope labels | UX contract §5's chip vocabulary, verbatim |
| Media query shapes | only phrasings the corpus proves answer today (DOC-02, IMG-01, AUD-03) |
| Inventory option | `source_file_audit`, a real template already suggested elsewhere in `query.go` |

An option is a SUGGESTION, not a claim about evidence. If one fails to resolve the system
clarifies again — that is the product working. What it must never do is name a field that
does not exist, or one the analyst is not entitled to see.

## Three defects found by probing live, each fixed

1. **Breakdowns offered for a media question.** DOC-04 ("What does the case notes document
   say about plate ABC-123?") resolved to the ANPR family because it says "plate", and was
   offered *"Break down by Camera / Lane / Location"* — pushing the analyst further toward
   the evidence WI-10 had just established it should not be reading. A question naming an
   artifact now always gets scope choices, never breakdowns.
2. **Scope drawn only from the 7 structured entities.** The curated layer has no document,
   image or audio entity, so X-01 — whose answer is a PDF and a WAV — was offered CDR and
   tower counts. Every option pointed away from the answer.
3. **An option that re-asked the withheld question.** IMG-04 was offered
   *"Images → How many images are in this case?"*, the question it had just declined. A
   loop, not a choice. Self-referential options are now removed, and the inventory option
   gives that case somewhere real to go.

## Click-through verification — the options actually work

    "Which document mentions 03001234567?"   -> 1 cited passage, NexusAI Multimodal
                                                Acceptance Brief  (X-01's expected .pdf)
    "Which files were ingested?"             -> 12 source files registered
    "Is 03001234567 mentioned in any audio?" -> 0 rows  (see D5 below)

**X-01 went from a confident false negative to a clarification whose first option returns
the correct evidence in one click.**

## D5 confirmed and located — the next work item

Audio search returns zero rows for every query, while the data is present and complete:

    forensics.audio-timestamp-segment/v1   16 rows, all processing_status='completed'
    metadata->'observation'->>'text' = "Call 03001234567 at 1035."   <- X-01 / AUD-03 gold
    one segment contains "coconut"                                   <- AUD-02 gold

This is RETRIEVAL, not ingestion. The transcript lives in
`metadata->'observation'->>'text'` on `derived_artifacts`; chase where the audio path reads
its text from.

**AUD-03 is very likely a FALSE CORRECT**, like DOC-01. Its check is
`contains ["urdu-english-identifier.wav"]` and the scorer searches the entire response
blob, so a filename appearing in any coverage listing scores CORRECT while retrieval found
nothing. Verify before counting audio as working.

Note: `processing_status` values are lowercase `completed`. A query testing for `'COMPLETED'`
matches zero rows and makes working data look like a processing failure — that error was
made and corrected during this session.

## Verification performed

- `go build` / `go vet` clean; full package suite green
- `clarification_options_test.go` — 11 tests: contract range, curated-label provenance,
  sensitive-field exclusion, groupable-only breakdowns, scope fallback, target carry-through,
  determinism, no-breakdown-for-media, relevance ranking, media scopes, no self-echo
- 8 live clarification probes, all `/v2` with populated options, retained as fixtures
- 3 click-through queries run against the live API
