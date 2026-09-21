# NexusAI P0 Baseline Audit — 2026-09-18

Directive: `NEXUSAI_TL_DIRECTIVE_PROMPT_20260918.md` (P0 = inventory, golden set,
live baseline, Role A timeout diagnosis). **No production behavior was changed.**
One temporary diagnostic test was created, run, and deleted.

## 1. Method

- Live stack: `nexusai-forensic-records-api-1` (auth enforced; unauthenticated → 401),
  Postgres, NATS, worker, LocalAI. Live synthesis model is
  **`qwen3-4b-instruct-2507-q4km-nxb21d-dev` (Q4_K_M)**, not the Q8 model named in the
  2026-09-17 checkpoint.
- Golden set `golden_questions_v0.json`: 62 English questions over 13 families
  (structured, documents, images, audio, video, cross-family, negatives). **Every expected
  value was computed by direct SQL**, independent of the API code.
- Harness `scripts/nexusai_live_eval.py` (stdlib only, repeatable) grades result values
  against the oracle and separately checks whether the analyst-facing text states the answer.
- **Every response was then read by hand** (`adjudicate_baseline.py`). The automated grader
  passed 4 items that were actually wrong-shaped (CDR-15, TWR-02, TXN-03, ANPR-05) and failed
  several that were partially right; the manual verdicts below are authoritative.
- Anchors re-derived by SQL: CDR 8,642 · IPDR 2,500 · ANPR 750 · access log 1,000 ·
  subscribers 11 · towers 5 · transactions 4 (`nexusai-forensic-demo`).

## 2. Scorecard (62 questions, manually adjudicated)

| Verdict | Count | Share |
|---|---:|---:|
| CORRECT | 16 | 26% |
| PARTIAL (right data, wrong shape / no total) | 9 | 15% |
| SAFE_FAIL (honest clarification; capability gap) | 5 | 8% |
| **WRONG — confidently wrong** | **26** | **42%** |
| WRONG — false "no records found" | 3 | 5% |
| ERROR (HTTP 500 shown to analyst) | 3 | 5% |

- **Correct-or-properly-clarified: 21 / 62 (34%).** Directive gate for LLM-planner
  switch-over is ≥ 90% with 0 confident-wrong.
- **Answer actually stated in the analyst-facing text: 5 of 20** correct/partial
  factual answers. The rest were buried in a table with process text on top.
- Latency: p50 0.56 s · p95 4.5 s · max 82 s (Role A narration on a document question).

| Family | Correct | Partial | Safe fail | Wrong | Error |
|---|---:|---:|---:|---:|---:|
| CDR (16) | 2 | 2 | 4 | 7 | 1 |
| IPDR (5) | 2 | 1 | – | 2 | – |
| ANPR (6) | 2 | 1 | – | 3 | – |
| Access log (4) | 1 | – | – | 3 | – |
| Subscriber (3) | 1 | 2 | – | – | – |
| Tower (2) | – | – | – | 2 | – |
| Transactions (3) | – | 2 | – | 1 | – |
| Cross-family (5) | – | 1 | – | 3 | 1 |
| Negatives (4) | 2 | – | – | 2 | – |
| Documents (6) | 4 | – | – | 2 | – |
| Images (4) | 1 | – | – | 2 | 1 |
| Audio (3) | 1 | – | – | 2 | – |
| Video (1) | – | – | 1 | – | – |

## 3. Defects found (ranked)

### P0 — confident wrong answers / crashes

| # | Defect | Evidence | Root cause (verified in source) |
|---|---|---|---|
| D1 | **Target and date filters silently dropped on dynamic counts.** "How many calls did 923001110001 make?", "…from August 2026?", and even a **nonexistent** number (`03999999999`) all return counts of every record in the case | CDR-11, CDR-14, NEG-01 | Dynamic plan compiles measure + (spurious) group but no filter for the extracted identifier / month; scope = all families (12,912 rows) |
| D2 | **Spurious GROUP BY from embedding similarity → nondeterministic answers.** Same question returned 8,642 at 06:24 and `3637/1/5004 by manual_review_required` at 06:40 | CDR-01, NEG-04, IMG-04 | `sourceNativeFieldByHint` (`deterministic_semantic_compiler.go:318-395`) adds `EmbeddingCosine × weight` computed against the **whole question** even when the group hint is empty; result depends on whether the embedding runtime answered |
| D3 | **HTTP 500 on every dynamic row-listing plan** — raw `execute analytical template: expected 3 arguments, got 23` shown to the analyst | CDR-09, CASE-03, IMG-03 (3/3 reproducible) | `executeSourceNativeProjectionSQL` (`source_native_sql_executor.go:65-81`) appends projection field-name args before the COUNT query, then passes them to a WHERE clause with only scope placeholders |
| D4 | **Cross-family misrouting** — IPDR domain / tower count → CDR `top_locations`; access-log status/IP → collection overview / CDR entities / ingest errors; HP guide question → `access_failed_events` | IPDR-02, IPDR-05, TWR-01, ACC-02..04, DOC-05 | Keyword ladder + curated synonyms; family of the question not enforced on the selected operation |
| D5 | **Irrelevant retrieval returned as a match** (the "J" class, still open for audio) — "Is 03001234567 mentioned in any audio?" returns the Japanese-cuisine segment; true hit exists | AUD-03, AUD-02, DOC-04 | Derived-text relevance filtering; identifier tokens not required to match |
| ~~D6~~ | **Withdrawn (oracle error).** The 6th ACTIVE row stores status in source column `account_status`; the API is correct | SUB-02 | Corrected 2026-09-18 during P1 |
| D7 | **"Explain …" treated as a dictionary definition** — returns "makes no statement about the current case" | CASE-04 | Request-class classifier routes to `GENERAL_DOMAIN_KNOWLEDGE` |

### P1 — wrong shape / capability gaps

- Count-vs-list still broken for ANPR ("how many times was LHR-2026 seen" → 20 rows) and
  rank-vs-list (camera / plate ranking → raw listing): ANPR-02/04/06.
- Totals never computed on top of breakdowns: CDR-02/04/05, IPDR-04, SUB-01, TXN-01.
- Wrong measure: "largest transaction" → COUNT; "account with most transactions" → ranked by amount.
- `longest_call` template returns daily totals, not the longest call (CDR-15).
- Tower lookup returns schema field-role profile, not coordinates (TWR-02).
- Grouped counts per record type → schema output (CASE-01, known).
- No case-wide ranking operations: top talkers, distinct counterparties, busiest cell,
  video plate list (CDR-06/07/08/12, VID-01).
- False negatives: per-source-file counts, image text lookup via correlation, cross-evidence
  identifier search (CDR-16, IMG-02, X-01).

### Answer / presentation (affects every question)

- **Fact Packets carry plumbing, not answers.** Facts for a transaction-total question are
  "Template: financial_transaction_summary", "Route: records_sql", "Planner Confidence: 1",
  "Display Rows: 2" — the PKR totals are not facts. Neither deterministic text nor the LLM can
  state an answer from that. This is the single biggest cause of poor answers.
- Headline text is process jargon ("Executed bounded source-native typed algebra over 8642
  authorized source rows and returned 1 deterministic results with contribution lineage");
  table column "M1"; metric "Records Row Count: 1" for a count of 8,642. Text is duplicated
  2–3× in `summary`.

## 4. Role A (LLM narration) diagnosis

The "exactly 120 s" mystery is **not** a hidden queue. Measured with the real
`synthesizeFactPacketNarrativeWithTrace` against live LocalAI (DOC-04 packet):

| Run | Prompt tok | Output tok | Time | Outcome |
|---|---:|---:|---:|---|
| In-product (cold-ish) | ~1,030 | ~145–250 | 79.2 s | GROUNDING_REJECTED |
| Diagnostic (warm) | 1,030 | 145 | 37.1 s | GROUNDING_REJECTED — `direct answer does not preserve a referenced fact` |

- Throughput ≈ 4 tok/s on CPU with the Q4 model; prefill of larger packets (50 citations,
  CDR results) pushes generation past the 120 s budget — that is the observed ceiling.
- Even when it finishes, **validation correctly rejects** it: given only plumbing facts, the
  model paraphrased and misattributed the source ("case notes document" for a PDF passage).
- Conclusion: tuning tokens/threads will not fix this. Fixing the Fact Packet content (real
  values and passages as facts) is the prerequisite; the hardware ceiling (~37–80 s per
  narration) must then be addressed by streaming the deterministic answer first and either
  a much smaller narration contract or faster hardware (costed recommendation required).

## 5. Data / ingestion observations

- PDF acceptance brief stored as **one passage for the whole document** (chunking gap);
  scanned-PDF OCR path absent; DOCX split into 112 paragraph passages.
- Image OCR on photographs is mostly noise (random Urdu/Latin fragments) stored as
  observations with confidence up to 0.90.
- 425 CDR rows where 923001110001 is its own counterparty — not surfaced as a data-quality note.
- Stray fixture `gate7.txt` (a PowerShell command) indexed as case evidence.
- Two evidence items in failed state (1 structured in demo, 1 image in multimodal) are not
  disclosed in count answers.

## 6. Catalog inconsistencies

- Template catalog: 5 / 79 `certified`. Platform operations: 52 / 104 `CERTIFIED`.
  **33 templates are "bounded_uncertified" in one catalog and CERTIFIED in the other**
  (list in `inventory.json`). Per the directive, nothing is treated as certified until it
  passes the golden suite.
- 80 inventory items are not yet exercised by any golden question.

## 7. Not done in this pass (explicit)

- Ingestion adapter / format decoder inventory and UI-control inventory (listed in
  `inventory.json.summary.not_yet_inventoried`).
- ≥3 phrasings + edge case for each of the 79 templates (golden v0 covers 62 questions,
  ~22 templates). Golden v1 expands this during P1.
- No fix was applied, rebuilt, or deployed.

## 8. Recommended P1 order (smallest change → largest win)

1. D3 projection-SQL argument bug (one-function fix, unblocks all row-listing plans).
2. D2 remove question-level embedding from group selection when there is no grouping hint.
3. D1 compile extracted identifiers/dates/families into dynamic-plan filters; refuse to answer
   when an extracted constraint cannot be applied (never drop it silently).
4. Family guard: an operation whose family is absent from the question's families cannot win.
5. Fact Packet v2: result values, totals, and cited passages as first-class facts; rewrite the
   deterministic headline from them ("There are 8,642 CDR records in this case").
6. D5/D7, then shape fixes (totals, count-vs-list, rank-vs-list, measures) and the missing
   case-wide ranking operations.

Each fix requires a container rebuild + redeploy to verify live (approval needed).

## 9. Files

- `golden_questions_v0.json` — golden set with SQL-derived oracles
- `baseline/results.json`, `baseline/results_adjudicated.json`, `baseline/scorecard.json`,
  `baseline/raw/*.json` — full live evidence
- `inventory.json`, `catalogs/*.json` — operation & query inventory
- `adjudicate_baseline.py`, `build_inventory.py`, `../../scripts/nexusai_live_eval.py`
