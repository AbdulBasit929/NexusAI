# A3 TEMPLATE ANSWERS STATE THEIR RESULT -- THRESHOLDS

Written 2026-09-29, BEFORE the switch was built into an image or any arm was scored.

## The defect

Two corpus questions score NOT_STATED on the deployed build (reports/a1-1-a2-20260929/on). The
result is computed and shown in the table; the direct answer does not say it.

    CDR-10   "Who did 923001110001 contact most frequently?"
             -> "8 phone contacts ranked for 923001110001."
    ANPR-03  "When was LHR-2026 first and last seen?"
             -> "Computed exact normalized target matches, record-family coverage, ..."

## The change, `FORENSIC_TEMPLATE_STATES_RESULT` (default off)

- `frequent_contacts`: name the contact with the highest ranked count, or every contact tied at it,
  and the count ("CDR records"). The old ranking sentence follows. If every returned row ties and the
  page is full, the tie may continue past the page, so nothing is added.
- `cross_family_correlation` and a first/last-seen question: state both times, in UTC to the second,
  from `family_coverage` (computed over every exact match), never from the capped displayed page.
- Filters the question applied are named ("in the requested date range"), not restated.
- Direction is not restated as who called whom: it is relative to each record's owner.

Expected on CDR-10, from the deployed rows: 923009998887 and 923009998883 are TIED at 121 CDR
records. The corpus expects 923009998887 (its note: first external contact by outgoing count, 68).
Naming both is the truthful answer to "most frequently" over total records; naming one would pick a
side of a tie the data does not break.

## Arms

    off   A3 OFF, A1.1 + A2 ON (the shipped configuration; also the post-ship check)
    on    A3 ON
    Each: corpus, 14 everyday, 38 pre-flight, 16 relational, plate probe.

## Pass criteria

    targets     CDR-10 and ANPR-03 move NOT_STATED -> CORRECT; the CDR-10 text names BOTH tied
                contacts with 121; the ANPR-03 text states 2026-07-14 06:00:00 UTC and
                2026-07-19 07:22:00 UTC and 87 ANPR sightings
    corpus      no other verdict moves; answer_text changes ONLY on frequent_contacts and
                cross_family_correlation answers, and every changed text is read
    inert off   off arm reproduces reports/a1-1-a2-20260929/on (verdicts and answer_text)
    probes      pre-flight 38/38 with no pass->fail; everyday 14, relational 16 and plate 8/8 (no
                leaks) unchanged except texts of frequent_contacts / cross_family_correlation
                answers, each read
    data grid   keys, rows and headers byte-identical between arms (A3 changes text only)

Fail any line -> A3 stays OFF.
