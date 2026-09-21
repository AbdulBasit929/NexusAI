# NexusAI — Current State

> **This file holds CURRENT STATE ONLY. Rewrite it; never append to it.**
> Hard cap: **300 lines.** If you are adding rather than replacing, you are using it wrong.
> The previous 874 KB / 12,836-line append-only log is preserved at
> [`reports/archive/continuation-20260921.md`](reports/archive/continuation-20260921.md).
> It contains real findings; read it only when tracing a specific historical claim.

**Last updated:** 2026-09-21 · **Branch:** `codex/forensic-hybrid-checkpoint-20260723` · **HEAD:** `584c135`

---

## 1. Active work items

| ID | Owner | State | Files owned (exclusive while open) |
|---|---|---|---|
| **WI-0** | claude | **NEXT — not started** | `reports/ir-spike-<date>/**`, throwaway harness under `scripts/`. **No production source changes.** |
| WI-1 | claude | queued | `api/forensic_records/deterministic_semantic_compiler.go`, `p1_correctness_test.go` |
| WI-2 | claude | queued | `api/forensic_records/stim_fact_packet.go`, `answer_presentation.go` |
| WI-3 | claude | queued | `api/forensic_records/operation_applicability.go`, `deterministic_semantic_compiler.go` (measure block) |
| **WI-4** | codex | **available now — parallel** | `semantic_layer/**`, `db/forensic_records/0xx_semantic_layer.sql`, loader/validator |

At most **two** active items, one per agent, in disjoint file sets. Full definitions in
[`docs/work/MASTER_EXECUTION_PROMPT.md`](docs/work/MASTER_EXECUTION_PROMPT.md) §9.

**Exact next action:** run **WI-0** — the IR-generation feasibility spike. Write its
decision rule into the report *before* running anything. Ask before the 40 live
inference calls.

---

## 2. Measured truth

**2026-09-18 live audit** — 62 English questions, 13 families, every expected value
computed by direct SQL independent of the API, every response hand-adjudicated.
Full evidence: [`reports/nexusai-tl-audit-20260918/P0-baseline-report.md`](reports/nexusai-tl-audit-20260918/P0-baseline-report.md).

| Verdict | Count | Share |
|---|---:|---:|
| CORRECT | 16 | 26% |
| PARTIAL | 9 | 15% |
| SAFE_FAIL (honest clarification) | 5 | 8% |
| **WRONG — confidently wrong** | **26** | **42%** |
| WRONG — false "no records found" | 3 | 5% |
| ERROR (HTTP 500 shown to analyst) | 3 | 5% |

Correct-or-properly-clarified **21/62 = 34%**. Answer actually stated in the analyst's
text: **5 of 20**. Latency p50 0.56 s · p95 4.5 s · max 82 s.

**26 confident-wrong answers is a safety defect, not a quality issue** — the analyst
cannot tell a wrong answer from a right one. Closing it outranks everything else.

**Regression anchors** (SQL-verified, case `nexusai-forensic-demo`): CDR 8,642 ·
IPDR 2,500 · ANPR 750 · access log 1,000 · subscribers 11 · towers 5 · transactions 4.

**Live models:** synthesis `qwen3-4b-instruct-2507-q4km-nxb21d-dev` (**Q4_K_M**, not Q8),
embeddings `qwen3-embedding-0.6b`. CPU only, ~4 tok/s, ~15.7 GiB RAM, 6 GiB floor.

---

## 3. Defect status — verified in source 2026-09-21

**FIXED — do not redo:**
- **D3** HTTP 500 on every dynamic row listing — `source_native_sql_executor.go:64-70`
- **D2** spurious GROUP BY from whole-question embedding — `deterministic_semantic_compiler.go:436-443`

**OPEN:**
- **D1 — target/date filters silently dropped.** `deterministic_semantic_compiler.go`
  initialises `plan.Filters` at line 418 and **never references `.Filters` again**.
  `semantic_frame.go:382` populates `frame.Filters` and nothing consumes it. A
  nonexistent phone number returns a count of all 12,912 case rows. **Highest priority → WI-1.**
- **D4** cross-family misrouting (IPDR / tower / access-log → CDR) — no family guard → WI-3
- **D5** irrelevant retrieval returned as a match (audio) — identifier tokens not required to match
- **D7** "Explain …" classified as a dictionary definition
- **NEW (2026-09-21)** measure field resolved from the whole question —
  `deterministic_semantic_compiler.go:461-463` sets `hint = question` when
  `MeasureFieldHint` is empty, running the same embedding resolver D2 was fixed for.
  Explains "largest transaction → COUNT". → WI-3
- **Fact Packets carry plumbing, not answers** — `stim_fact_packet.go:205` builds facts
  from metrics, and the metrics are "Template:", "Route:", "Planner Confidence".
  Affects **every** question. `validateNarrative` correctly rejects the narration; the
  packet is the problem, not the model and not the hardware. → WI-2

---

## 4. Architecture — decided, do not relitigate

Rationale and evidence: [`docs/architecture/RECONCILIATION_20260921.md`](docs/architecture/RECONCILIATION_20260921.md).

1. **Stop defining operations.** 79 templates (5 certified), 104 operations (52
   certified, 33 contradictory) and an 8,644-line keyword ladder in `query.go` are a
   *per-question* abstraction. **Frozen now, deleted at the end of Phase 3.**
2. **Governed Semantic Compiler.** Define the **data** once as a curated semantic layer,
   then compile every question: deterministic classify → literal extraction → scope →
   catalog narrowing → **enum-constrained IR generation** → hard validation → one
   self-correction → parameterized SQL → verification → **abstain rather than guess**.
3. **The IR already exists.** `SourceNativePlanV1` is richer than the published SMQ that
   scored 94.15% on Spider2-snow, and `source_native_sql_executor.go` already compiles it
   to parameterized SQL with scope predicates and lineage. **Build on it, do not replace it.**
4. **The keystone is already in the repo.** LocalAI at the v4.5.6 baseline supports strict
   `json_schema` response format and compiles JSON-Schema `enum` to a GBNF alternation
   rule (`pkg/functions/grammars/json_schema.go:129-138`). Enumerating authorized field
   IDs makes a hallucinated field **structurally impossible**.
5. **LocalAI becomes a pinned, unmodified upstream image** (`v4.10.x`), internal network
   only. Fork baseline is v4.5.6; a rebase is impossible (one squashed commit, no upstream
   remote, no merge base). Real core divergence is ~330 lines across 8 files.
6. **NexusAI owns analyst identity and cases.** There is no case entity today: `case_id`
   occurs 3× in the schema vs `collection_id` 224×; no users/roles/membership tables; no
   case-creation path in the UI; every analyst authenticates as the same shared service key.
7. **UI contract is [`docs/ux/NEXUSAI_PRODUCT_UX.md`](docs/ux/NEXUSAI_PRODUCT_UX.md).**
   IA is CASE → EVIDENCE → QUESTION; families are filters, never navigation.

---

## 5. Hard rules

- **No new operation templates.** Do not extend `query.go`'s keyword ladder.
- **No model roulette.** Do not evaluate, benchmark, download or swap LLMs except via
  WI-0's written decision rule. Two operation selectors already failed at 0/5 — at a task
  this architecture deletes.
- **No embeddings on structural decisions** (family, group, measure, field role). D2 and
  the new measure defect both came from exactly this. Embeddings are for semantic *text*
  retrieval only.
- **No reports without a measurement.**
- **Abstention is a success.** A confident wrong answer never is.
- **Never fabricate** an analytical result, citation, OCR text, transcript, ANPR sighting,
  face identity, location, timeline or cross-family relationship. Never turn a similarity
  score into an identity claim.
- **Never merge upstream LocalAI into this repo.** Clone to a *sibling* directory to compare.

---

## 6. Operational hazards

- **Auth fails closed.** `forensic-records-api` needs `FORENSIC_RECORDS_API_KEY` and
  `FORENSIC_API_AUTH_REQUIRED=true` exported (or sourced from
  `.env.forensic-runtime.local`) before **every** `docker compose up`. The container
  refuses to start otherwise. **Never "fix" a crash-loop by disabling auth** — on
  2026-09-17 every redeploy of an entire session silently served the forensic API
  unauthenticated, on real evidence.
- **Never** `docker compose down`, `down -v`, `--remove-orphans`, `system prune`,
  `volume prune`. Never reprocess, backfill, migrate or delete retained evidence.
- **Git:** never `reset --hard`, `clean`, `checkout --`, `restore`, `stash`, `merge`,
  `rebase`, `cherry-pick`, `pull`, `push`, or any force operation without being asked in
  the current session. Never delete an untracked file you did not create.
- **Build gates are currently unrunnable** — `make` is not on PATH in the development
  shell, so `make lint` and `make test-coverage-check` cannot execute and the pre-commit
  hook fails closed. The six Phase 0 checkpoint commits were made with `--no-verify` and
  say so. **Debt:** install the toolchain and establish a real coverage baseline for the
  NexusAI Go tree (`coverage-baseline.txt` was set for upstream LocalAI and is meaningless
  against 39k added lines) before any Phase 1 change is reviewed.

---

## 7. Ask before

Container rebuild or redeploy · any `docker compose` action beyond `ps`/`logs` · any
database write, migration or backfill · model downloads or backend installs · any git
operation that writes · long builds · running the live golden suite or WI-0's 40 live
inference calls · anything touching retained evidence.

---

## 8. Where to read more

| Question | File |
|---|---|
| Why is the architecture this way? | `docs/architecture/RECONCILIATION_20260921.md` |
| What should the UI be? | `docs/ux/NEXUSAI_PRODUCT_UX.md` |
| What do I do, exactly? | `docs/work/MASTER_EXECUTION_PROMPT.md` |
| How do I join as Codex? | `.agents/CODEX.md` |
| What is actually broken? | `reports/nexusai-tl-audit-20260918/P0-baseline-report.md` |
| Historical detail | `reports/archive/continuation-20260921.md` (874 KB — trace specific claims only) |

**Superseded where they conflict with the above:** `NEXUSAI_MASTER_DIRECTIVE.md`,
`NEXUSAI_NEXT_CHAT_PROMPT.md`, `NEXUSAI_TL_DIRECTIVE_PROMPT_20260918.md`,
`configuration/nexusai_stim_maturity_matrix.json` (dated 2026-09-14 — predates both the
auth incident and the accuracy baseline). Kept for provenance, not authority.

---

## 9. Phase 0 — complete

Six checkpoint commits on 2026-09-21 (`d374f3d`..`584c135`) brought 3,481 files under
version control; the repository previously had **one** commit and ~950 untracked entries.
Excluded: 27,538 Go build-cache files and 69 MB of duplicated build binaries inside
`reports/*/build-context/`. Working tree clean. Not pushed.

**Remaining Phase 0 debt:** reconstruct the LocalAI v4.5.6 baseline in a sibling clone and
produce `docs/integration/FORK_DELTA_v4.5.6.md`. Not blocking WI-0 or WI-4.
