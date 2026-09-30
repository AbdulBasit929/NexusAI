# Runtime query plan, version 2: one investigator agent, governed tools, my cases first

2026-09-29. Shared copy for the team lead: https://claude.ai/artifact/Kqs7uTM7xs2nEiLJZw64Th (version 2).
Research behind it: `docs/research/`. From Phase B on, this supersedes the ordering in `ROADMAP_20260929.md`.

## Direction (team lead and product owner)

- The LLM classifies the question, writes the query against the live schema, runs it and presents the
  results. There is no per-question code and there are no answer templates.
- The LLM also handles greetings, concepts and product help.
- **First:** an analyst asks across all of their own cases.
- **Hardware:** this laptop is the only machine for now (i7-1260P, 15.7 GB, CPU only), so it has to be made capable. A GPU is optional later and must be affordable.

## Architecture (evidence: `docs/research/ARCHITECTURE_20260929.md`)

**One investigator agent with governed tools, not one agent per data type.** Type-specific knowledge
lives in short family guides that load when a type is chosen.

**The agent's tools:**

| Tool | What it does |
|---|---|
| `describe_schema` | The schema catalogue generated from the database plus semantic-layer meanings and sensitivity, with masked samples |
| `find_value` | Which fields, types and cases hold an identifier |
| `plan_query` | The existing structured compiler; exact, or it refuses openly; used first |
| `run_sql` | One checked `SELECT` over analyst views; the fallback |
| `search_text` | Hybrid FTS, trigram and vector search over passages, transcripts and OCR; plates by exact search only |

**Safety:**
- the read-only role and read-only transaction with a timeout
- masked analyst views
- the case scope set by the server from the analyst's own cases
- a parser check (`wasilibs/go-pgquery`, pure Go, CGO-free; check its licence first) with an allowlist
  and cost limit
- evidence text treated as data
- every question logged

**By machine:** on the CPU, one step with one retry. On a GPU, up to four tool steps.

## Phases

| Phase | Steps | Gate |
|---|---|---|
| 0 Protect and speed up the laptop (days) | 0.1 Commit (owner approval). 0.2 Laptop speed-up, each lever measured (`docs/research/HARDWARE_MODELS_20260929.md` L1–L8): prompt-start reuse with one cache slot per call type, fewer calls, shorter outputs, thread tuning, then (with approval) a newer backend with Q4_0 weights and a 1.7B helper; non-blocking answers in the UI. | Same 103 answers, 0 new wrong; new-question time measured before and after |
| 1 Investigate across my cases (2–3 weeks) | 1.1 Case registry with owner and members, plus a server-trusted workspace analyst identity (**database change, approval**). 1.2 API scope "all my cases" or selected, grouped responses, audit. 1.3 Plan once, run per case, merge rules, complete-search rule. 1.4 "Where does this identifier appear across my cases" (fixes X-01). 1.5 Cross-case text search. 1.6 Codex global Investigate page (BACKEND_REQUESTS row 11). 1.7 A ~25-question cross-case set, database-verified. | Verified, 0 wrong, complete-search rule, 38/38, 103 unchanged |
| 2 Governed data access (about 2 weeks) | 2.1 Schema catalogue from the database, plus a drift test. 2.2 Analyst views, read-only role, server scope (**database change**). 2.3 SQL checker and query log. 2.4 Value index. | At least 50 red-team queries rejected, 0 regressions |
| 3 Investigator agent (3–4 weeks, behind a switch) | 3.1 LLM request understanding with grounded help and concepts. 3.2 Family guides. 3.3 Tools, verification, up to 2 corrections, refusal, answers from rows. 3.4 Approved-example library (added only after review). 3.5 Fix CASE-01 and X-01 with templates off. | More correct, 0 new wrong on the 103, the cross-case set and the investigator set |
| 4 Model choice on free GPUs (1–2 weeks) | 4.1 Kaggle free-GPU bake-off (30 h/week) on demo and synthetic data only (owner approval): Arctic-Text2SQL-R1-7B, OmniSQL-7B, Qwen3-4B/8B and Qwen3-1.7B against the current 4B, for accuracy. Latency is judged on the laptop. 4.2 A GPU purchase stays optional for later (16 GB new at $430–550, or 24 GB used at $700–1,050). | The chosen model beats the current one with 0 confidently wrong, and runs on the laptop |
| 5 Fine-tuning a laptop-sized model on Kaggle, only for a measured gap (2–3 weeks) | 800–1,000 execution-verified examples, never from the evaluation sets; a QLoRA adapter; compare under a written rule (`docs/research/FINE_TUNING_20260929.md`) | More correct, 0 new wrong |
| 6 Retire templates | One evidence type at a time | Templates-off at least equals templates-on, 0 new wrong |
| 7 Production | Keycloak login, sharing and roles, audit per analyst, security review, backups, runbook, production GPU sizing | — |

## Decisions needed

- Is the HP EliteDesk with the RTX 2080 available now, and what is its exact model?
- Approve the database changes: the case registry and ownership (Phase 1), and the analyst views and
  read-only role (Phase 2).
- Which analyst owns the existing cases.
- A small cloud budget for GPU rental, with demo data only.
- The development GPU budget.
- Commit approval.
- Who writes the ~50 investigator questions.
