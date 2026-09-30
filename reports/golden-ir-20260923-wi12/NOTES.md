# WI-12 — audio was never broken

**Date:** 2026-09-23 · **Rollback:** `nexusai-forensic-records-api:rollback-before-transcript-20260923`

## The misdiagnosis

D5 was recorded as an audio RETRIEVAL defect: every audio question returned zero rows and
"The text search is incomplete." The data was checked and found present and complete —
16 transcript segments, `processing_status='completed'`, containing exactly the gold strings.
The conclusion drawn was "retrieval does not reach it."

**That was wrong.** Retrieval reached it every time. Dumping the full response for
`Find the exact phrase "coconut sugar" in the audio transcripts` showed:

    evidence.row_count        1
    evidence.results          "especially Japanese coconut sugar, and various aromatic spices."
    citation_locator          fleurs-en_us-validation-row-01.wav, start 11.28s, end 15.08s
    answer.transcript_text    "especially Japanese coconut sugar, and various aromatic spices."

The match was found, scored, cited and located. Two PRESENTATION defects threw it away.

## Defect 1 — the caveat replaced the answer

`enterpriseExecutiveAnswer` returned "The text search is incomplete. Any listed observations
are supported matches; absence of further matches has not been established." whenever
`result_state` was SEARCH_INCOMPLETE — as the ENTIRE answer. The sentence claims listed
observations exist while listing none, because returning it discarded the finding.

Fixed: the caveat is the whole answer only when there is nothing to report. Otherwise the
finding is stated and the caveat appended as its limit — sections 1 and 5 of the answer
contract, not section 1 replaced by section 5.

**The first attempt was wrong and a test caught it.** Removing the early return dropped the
disclosure entirely; `b1_derived_text_ginkgo_test.go` "discloses positive result truncation
and incomplete zero" failed. The requirement is BOTH: state the finding AND disclose that
absence is not established.

## Defect 2 — chronology outranked relevance

The result sort ordered segments of one recording by start time BEFORE score. Segments of a
recording share an evidence and run, so the chronological comparison always fired and score
never broke the tie. Truncation to `MaxKBResults` then kept the EARLIEST segment.

AUD-02 returned "seasoned dishes, the predominant flavorings, the Japanese favor being peanut
chilies, sugar," at 5.56s, while "especially Japanese coconut sugar, and various aromatic
spices." sat at 11.28s and was discarded before the answer was built.

Fixed: score outranks chronology; chronology remains the tie-break. A genuine browse is
unaffected — with no meaningful terms every score is equal and ordering is unchanged.

## New: transcriptExecutiveAnswer

States WHICH recording and WHEN, from the citation locator of the segment that actually
carried the match. Every value comes from the locator; a segment with no usable start time
contributes no time claim rather than a guessed one, and no citable source produces no
answer at all rather than a half-claim.

## Result — measured live

| | Before | After |
|---|---|---|
| AUD-01 | "search is incomplete", 0 rows shown — **FALSE CORRECT** | names `fleurs-en_us-validation-row-01.wav` — genuinely CORRECT |
| AUD-02 | WRONG | `...at 11.28 seconds: "especially Japanese coconut sugar..."` — CORRECT |
| AUD-03 | "search is incomplete" — **FALSE CORRECT** | names `urdu-english-identifier.wav` — genuinely CORRECT |
| DOC-02 / DOC-03 | correct | unchanged |
| DOC-06 | correctly-scoped negative | unchanged |

**AUD-01 and AUD-03 were false CORRECTs.** The scorer's `contains` check searches the whole
response blob, so a filename appearing in a coverage listing scored CORRECT while the analyst
was shown nothing. This is the same oracle weakness as DOC-01. Treat any `contains` check
whose expectation is a FILENAME as suspect until the analyst-facing text is inspected.

## Still weak

IMG-01 answers "Retrieved 1 cited evidence result from the selected case scope" without
naming the image. Pre-existing generic fallback — `image_ocr_search` deserves the same
citation-naming answer audio now has.

## Verification

- `go build` / `go vet` clean; full package suite green (711 specs)
- `transcript_answer_test.go` — 6 tests including the claim-carrying-citation case
- 7 live probes (3 audio, 3 document, 1 image)
- **Known flake:** `TestForensicRecordsSynthesis` failed twice while a container redeploy had
  the synthesis model under load, then passed 4 consecutive runs in isolation. Load-correlated,
  not reproducible standalone, not caused by these changes.
