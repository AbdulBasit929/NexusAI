# Golden-suite scorecard after WI-1 / WI-2 / WI-3 — 2026-09-21

62 questions, live stack, `qwen3-4b-instruct-2507-q4km-nxb21d-dev`. Every response
hand-adjudicated; the automated grader's verdicts are kept alongside for transparency.

Evidence: `baseline/raw/*.json` (62 raw responses) · `baseline/results.json` (automated) ·
`regrade.json` (answer-location analysis) · `hand_verdicts.json` (authoritative) ·
`run_log.txt`.

**Build note.** The image under test (`94a13b6c0fca`) was **not** produced by
`docker compose build`. That stalled for 65 minutes on the whole-repo build context plus
`go mod download`. The binary was compiled on the host with the Dockerfile's identical
flags (`CGO_ENABLED=0 -trimpath -ldflags="-s -w"`, linux/amd64) and layered onto the same
`gcr.io/distroless/static-debian12:nonroot` base. Same runtime, different build path.
Adding `reports/` to `.dockerignore` and splitting the Go build cache into its own layer
would make the canonical build usable again; that change has NOT been made.

Pre-run verification: API on image `94a13b6c0fca` · unauthenticated request → **HTTP 401**
(auth fails closed) · SQL anchors all exact — CDR 8,642 · IPDR 2,500 · ANPR 750 ·
access log 1,000 · subscribers 11 · towers 5 · transactions 4.

---

## 1. Scorecard

| Verdict | 2026-09-18 baseline | After WI-1/2/3 | Δ |
|---|---:|---:|---|
| CORRECT | 16 (26%) | **27 (44%)** | +11 |
| PARTIAL | 9 (15%) | 10 (16%) | +1 |
| SAFE_FAIL (honest clarification) | 5 (8%) | **9 (15%)** | +4 |
| **WRONG — confidently wrong** | **26 (42%)** | **16 (26%)** | **−10** |
| WRONG — false "no records found" | 3 (5%) | folded into WRONG | — |
| **ERROR (HTTP 500)** | **3 (5%)** | **0** | **−3** |

- **Correct-or-properly-clarified: 21/62 (34%) → 36/62 (58%)**
- **Confident-wrong: 26 (42%) → 16 (26%)**
- **Answer stated in the analyst's text: 5/20 → 14/37 (38%)**
- Latency p50 **2.53 s** · p95 **4.18 s** · max 120.3 s

### Phase 1 gate — NOT MET (1 of 4)

| Criterion | Target | Actual | |
|---|---|---|---|
| HTTP 500 shown to analyst | 0 | **0** | **PASS** |
| Confident-wrong | 0 | 16 | FAIL |
| Correct-or-properly-clarified | ≥70% | 58% | FAIL |
| Answer stated in text (correct/partial) | 100% | 38% | FAIL |

**D3 is confirmed fixed in production**: the three baseline HTTP 500s (CDR-09, CASE-03,
IMG-03) are gone, and no request in the suite returned 500.

---

## 2. The dominant cause: 44% of questions never reach the compiler

`planner.source` across the 62 responses:

| Path | Count | Reaches WI-1/2/3 guards? |
|---|---:|---|
| `runtime_query` — the `query.go` keyword ladder | **27** | **No** |
| `semantic_deterministic_registered` | 16 | Yes |
| `semantic_deterministic_dynamic` | 13 | Yes |
| `explicit_template` | 5 | No |
| none | 1 | — |

WI-1, WI-2 and WI-3 all modify the **deterministic semantic compiler**. Questions routed
through the keyword ladder bypass every guard added, which is why D1 and D4 still appear
live despite both being closed at the compiler and unit-tested:

- **ACC-04** "How many requests failed with a server error (status 500)?" →
  *"There are 1,000 access log entries in this case."* The `500` literal was dropped.
  **This is D1, on the ladder path.**
- **ACC-03** "Which IP address made the most requests?" →
  *"37,601 records across 5 entity type values. Largest: phone: 20,408; location: …"*
  A cross-family entity operation answered an access-log question.
  **This is D4, on the ladder path.**

The compiler path behaves as designed. CDR-01 routes
`semantic_deterministic_dynamic`, answers *"There are 8,642 CDR records in this case."*,
and states the answer in the analyst's text — the exact WI-2 objective.

---

## 3. Regressions introduced by this work

**CDR-13 — "How many incoming versus outgoing calls are there?"**
Returns *"There are 8,642 CDR records in this case."* — a total, not a breakdown.
**Was CORRECT at baseline.** The WI-3 family guard removed the registered operation that
served it, and the dynamic fallback compiled a plain COUNT because the question names no
explicit grouping dimension ("versus" is not parsed as one). A confident-wrong answer, and
the clearest debt this work created.

**CDR-11 / CDR-14 — now clarify instead of answering.**
Baseline answered 12,912 for both (D1). They now return an honest clarification, which is
strictly safer, but WI-1 was meant to make them *answer* 872 and 2. They clarify at the
LLM operation selector (`AMBIGUOUS_INTENT`, `dynamic_selected=false`) **upstream of the
compiler**, so none of WI-1's reason codes (`CONSTRAINT_UNBOUND`) reach the response.
Counted as SAFE_FAIL, not CORRECT.

---

## 4. WI-2 shortfall: five headlines still carry process jargon

CDR-09, CASE-01, CASE-03, IMG-03, TWR-02 still show, for example:

> *"Executed bounded source-native typed algebra over 8642 authorized source rows…"*
> *"Queried forensic.records with parameterized canonical filters, paging, totals…"*

WI-2 guarded the **Fact Packet**, but `enterpriseExecutiveAnswer` (`query.go:6779`, which
WI-2 does not own) still falls back to `records_summary` — the jargon sentence generated at
`query.go:6098` — whenever `buildResultAnswer` declines to produce a headline.

---

## 5. Two measurement caveats

**The automated grader under-reports.** It reported 24 CORRECT / 26 WRONG; hand
adjudication gives 27 / 16. It searches **result rows** for the expected value, while WI-2
deliberately moved the answer into the **headline**. CDR-02, CDR-04 and CASE-04 were graded
WRONG even though their analyst text states *"GPRS: 5,863; SMS: 1,108"*. Any future run
must grade the analyst-facing text, not only the grid.

**SUB-03 is an oracle bug, not a defect.** The response returns `*********0011` because
server-side CNIC masking is working exactly as the privacy rule requires; the golden file
expects the unmasked `00000-1000001-1`. The golden oracle should be corrected to expect the
masked form — an oracle that demands unmasked PII would reward a privacy violation.

---

## 6. What improved, question by question

**Baseline WRONG/PARTIAL → now CORRECT (11):** CDR-01, CDR-02, CDR-03, CDR-04, CDR-15,
IPDR-02, ANPR-05, SUB-01, TXN-03, CASE-04, NEG-04, AUD-03.

**Baseline WRONG → now honest clarification (3):** CDR-11, CDR-14, TXN-02 — TXN-02 was the
"largest transaction answered with COUNT" defect; it now declines rather than guessing.

**Baseline ERROR → no longer crashes (3):** CDR-09, CASE-03, IMG-03 — all three now return
an answer (still wrong, but no HTTP 500 reaches the analyst).

**Unchanged and still wrong (ladder path):** ACC-02, ACC-03, ACC-04, IPDR-05, DOC-04,
DOC-05, IMG-02, X-01, CDR-16.
