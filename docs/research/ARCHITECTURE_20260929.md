# Query architecture research, 2026-09-29

**Question:** how should the LLM answer any evidence question from the database without predefined
templates, accurately and safely?

## Options and evidence

| Option | Evidence | Verdict |
|---|---|---|
| A. The LLM writes raw SQL for everything | dbt 2026: frontier models writing raw SQL reach 84–90% on a modeled project, and they fail with plausible but incorrect answers. BIRD: Arctic-Text2SQL-R1-7B 68.47%, 14B 70.04%, 32B 71.83%; general 7B models about 39% (on-prem study 2606.29733). | Not alone: silent wrong numbers. Keep as a fallback. |
| B. Semantic layer only (the LLM fills a structured plan, a compiler writes the SQL) | dbt 2026: 98.2–100% with semantic-layer grounding, and explicit failure outside coverage. Knowledge-graph context took GPT-4 from 16% to 54%. Bounded semantic planning (2608.16663) favours correctness over coverage. | Exact but incomplete: no joins, cross-type links or "shared" relations. |
| C. One agent per data type | Multi-agent systems (MAC-SQL, CHESS, MARS-SQL) split by task stage, not by domain. A simple ReAct agent with tools (execute, inspect schema, sample values) is competitive with elaborate pipelines (2608.22651). | Keep type-specific knowledge as family guides, not separate agents. |
| D. One agent, governed tools: structured tool first, checked SQL as fallback, text search, value lookup | dbt recommends the semantic layer when accuracy matters and text-to-SQL for the rest. Self-correction gives +3.06 and +3.65 points on 7–8B models. Schema linking cost 1.76 points on Llama-3.1-8B, and self-consistency gave +0.13 at 5× the cost (2606.29733). | **Recommended.** |

## Supporting techniques

- **Value retrieval**, where an identifier in the question is mapped to the fields that hold it: CHESS
  uses LSH plus embeddings. For NexusAI this becomes the `find_value` tool over a value index.
- **Candidate generation and ranking:** Contextual-SQL (Qwen2.5-32B, about 73% on BIRD dev) samples up
  to 1,024 candidates and ranks them with a reward model. That's too expensive locally, so it's deferred.
- **Enterprise schemas:** ReFoRCE scores 35.83 / 36.56 on Spider 2.0 Snow / Lite, using about 3.5 LLM
  calls per question with schema compression and self-refinement. Large schemas are the hard part;
  our 17 entities and 186 fields are moderate.
- **Text evidence:** hybrid search in PostgreSQL. tsvector (BM25-style), pg_trgm and pgvector are fused
  with reciprocal rank fusion. One practitioner measured precision rising from about 62% with vectors
  alone to 84% or more with the fusion.
- **Benchmarks are noisy:** CIDR 2026 found annotation errors in them. Our investigator set is the
  score that counts.

## Safety findings

- Prompt instructions are not a security boundary. Use a dedicated read-only role, `SELECT` grants on
  only the needed views, a read-only transaction and `statement_timeout`, a parser check for single
  statements, and an allowlist of functions.
- Our API image is `CGO_ENABLED=0` and distroless-static
  (`api/forensic_records/Dockerfile`). pg_query_go needs CGO, whereas `wasilibs/go-pgquery` runs
  libpg_query compiled to WebAssembly in pure Go. It's 4–5× slower than CGO, which is negligible per
  question. Check its licence before adopting it.
- Evidence text can carry injected instructions. Tools are read-only and the scope is server-set, so the
  worst case is a refusal.

## Existing tools checked

- Vanna 2.0 (MIT) has user-aware row-level security, but its repository was archived on 2026-03-29.
  Don't depend on it.
- Identity: Keycloak is Apache-2.0. Zitadel moved to AGPL-3.0 in 2025, so avoid it. Authentik is MIT
  with an enterprise tier.

## Facts about NexusAI (read-only checks, 2026-09-29)

- 5 cases hold evidence.
  - `evidence_items.user_id` and `records_ingest_jobs.user_id` exist, but they hold whatever the
    client sent.
  - One case holds evidence from 2 uploaders.
  - There is no case registry with an owner.
- The UI sends a fixed actor, `investigation-workspace`, and all analysts share one API key.
  Row-level security is by tenant only, and every forensic table has `collection_id`.
- Correct answers split into 30 LLM plans, 25 compiler plans, 11 templates and 4 terminal. With
  templates off: 68 correct and 4 wrong (`reports/templates-off-20260929/RESULT.md`).

## Sources

- dbt Labs, Semantic Layer vs. Text-to-SQL, 2026: https://docs.getdbt.com/blog/semantic-layer-vs-text-to-sql-2026
- Bounded Semantic Planning: https://arxiv.org/pdf/2608.16663
- Simple ReAct suffices: https://arxiv.org/pdf/2608.22651
- On-prem open LLMs on text-to-SQL: https://arxiv.org/html/2606.29733
- Arctic-Text2SQL-R1: https://arxiv.org/abs/2505.20315
- CHESS: https://scalingintelligence.stanford.edu/pubs/CHESSpaper.pdf
- ReFoRCE: https://arxiv.org/abs/2502.00675
- Spider 2.0: https://spider2-sql.github.io/
- Contextual-SQL: https://contextual.ai/blog/open-sourcing-the-best-local-text-to-sql-system
- Text-to-SQL Benchmarks are Broken (CIDR 2026): https://www.vldb.org/cidrdb/papers/2026/p5-jin.pdf
- ParadeDB hybrid search: https://www.paradedb.com/blog/hybrid-search-in-postgresql-the-missing-manual
- Read-only boundary: https://libredb.org/blog/postgresql-agent-read-only-boundary/
- go-pgquery: https://pkg.go.dev/github.com/wasilibs/go-pgquery
- Vanna: https://github.com/vanna-ai/vanna
- Authentication comparison: https://skycloak.io/blog/open-source-authentication-comparison-2026/
