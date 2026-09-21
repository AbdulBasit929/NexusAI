# NexusAI — Reconciliation v2 + Target Architecture
## Incorporating the team-lead constraint: *"we cannot define so many operations — any runtime query must be converted to the correct SQL / retrieval against our available data, accurately"*

**Date:** 2026-09-21 · **Mode:** read-only. No source, config, database, container or model state was changed.
**Supersedes:** Reconciliation v1 (same date). Sections A, G, H, I, S and T are substantially rewritten; the rest is carried forward with updates.

**Evidence labels:** `[SOURCE]` read from this repository · `[UPSTREAM]` current LocalAI release/API data fetched this session · `[RESEARCH]` published result fetched this session · `[TEST]` measurement already in `reports/` · `[INFERENCE]` my reasoning · `[RECOMMENDATION]` my judgement · `[UNKNOWN]` explicitly not established.

---

## A. EXECUTIVE VERDICT

**The team lead is right, and the research agrees with him.** A catalog of 79 templates and 104 operations is the wrong abstraction: it is *per-question*, so it can never cover a question nobody wrote in advance, and every phrase that misses produces a confident wrong answer. The correct abstraction is **per-field, defined once** — a curated semantic layer over the data — plus **one general compiler** that turns any question into a validated typed plan and then into parameterized SQL.

**The decisive published result `[RESEARCH]`:** a semantic-layer-mediated agent that has an LLM emit a compact intermediate representation (**SMQ**) which a **deterministic engine compiles into SQL** scored **94.15% execution accuracy on Spider2-snow** (547 real enterprise NL2SQL instances) — against **2.2%** for schema-only DAIL-SQL + GPT-4o and **23–26%** for Spider-Agent approaches on the same benchmark. Industry reporting agrees: text-to-SQL grounded in a semantic layer moves from ~40% to **86–95%**, where raw frontier text-to-SQL sits at 70–85% on *clean* data and far lower on enterprise schemas.

**NexusAI already has the hard half of this and did not realise it `[SOURCE]`.** `SourceNativePlanV1` — `{Project, Filters, GroupFields, Measures, TimeBucket, Having, Sort, Limit}` — is a **richer IR than the published SMQ** (`{metrics, filters, group_by}`), it already has a strict parser (`DisallowUnknownFields` + trailing-content rejection), and `source_native_sql_executor.go` already compiles it to parameterized SQL with scope predicates and lineage. What is missing is not the engine. It is:
1. the **semantic layer is inferred per request from sampled rows**, not curated — `buildSourceNativeFieldCatalog(req, rows)` profiles whatever rows it sees `[SOURCE]`;
2. `FieldDescriptorV1` has **zero description fields** — `grep -c Description` on the algebra returns **0** `[SOURCE]`, so nothing tells a model that `inv_tot` means "invoice total";
3. there is **no join graph**, which is why cross-family scored **0/5** `[TEST]`;
4. nothing ever asks a model to **fill the IR** — the LLM was asked to pick an operation ID from ~104 opaque strings, which is why it scored **0/5**.

**ARE_WE_BUILDING_ON_THE_RIGHT_FOUNDATION =** Yes — more so than v1 concluded. The IR and compiler are the state-of-the-art pattern. The template catalog sitting on top of them is the mistake.

**BIGGEST_CURRENT_ARCHITECTURAL_PROBLEM =** The system routes questions to *pre-written answers* instead of *compiling* them. Symptom in source: the planner extracts the user's filter and then never applies it — `compileDeterministicSourceNativePlan` initialises `plan.Filters` and never reads `frame.Filters` again `[SOURCE]`. A phone number that does not exist returns a count of all 12,912 case rows `[TEST]`.

**BEST_QUERY_ARCHITECTURE =** **Governed Semantic Compiler** (§G): curated semantic layer → deterministic scope + family narrowing → **enum-constrained IR generation** → hard validation → parameterized SQL → **verification** → **abstention instead of guessing**. The LLM fills a typed schema; it never picks an operation, never writes SQL, and never becomes database authority.

**DO_WE_NEED_AN_LLM_AT_ALL =** Yes — but for a different job than it was given. Asking a 4B model to classify among 104 opaque operation IDs is a task small models are demonstrably bad at (**0/5** `[TEST]`). Asking it to fill six typed slots from an **enumerated** vocabulary of ~25 field IDs is a task that **constrained decoding makes structurally safe**, and LocalAI already supports it.

**THE KEYSTONE FINDING `[SOURCE]`:** LocalAI at the v4.5.6 baseline — the version already running — supports `response_format: {type:"json_schema", json_schema:{name, strict, schema}}` and a `grammar` parameter, and ships a **JSON-Schema→GBNF converter** (`pkg/functions/grammars/`) in which **`enum` compiles to a GBNF alternation rule** (`json_schema.go:129-138`). Put the authorized field IDs in the schema as an `enum` and the decoder **physically cannot emit a field that does not exist**. Hallucinated columns become impossible rather than unlikely. The `MALFORMED_OUTPUTS=2` failure recorded for Phi-4-mini was avoidable with a capability already sitting in this repository.

**HONEST UNCERTAINTY `[UNKNOWN]`:** no published number exists for "Qwen3-4B filling an enum-constrained analytical IR over a 25-field curated catalog." On-prem open models doing *raw* text-to-SQL on BIRD score 39.1% (Qwen2.5-Coder-7B), 47.4% (14B), 50.4% (32B), 32.9% (Llama-3.1-8B) `[RESEARCH]` — which is exactly why the model must never write SQL. The IR task is far easier, but **it must be measured before the architecture is committed to a model tier**, not assumed. §T WI-0 is a two-day spike with a pre-declared decision rule that settles it.

**THE PRODUCT INSIGHT THAT MAKES THIS ACHIEVABLE:** you do not need perfect plan accuracy. You need **≈0% confident-wrong**. Verification plus abstention converts the residual error budget from *wrong answers* into *clarifying questions*. An analyst can work with "Which of these three fields do you mean by 'total'?" An analyst cannot work with a confident 12,912.

**BEST_LONG_TERM_LOCALAI_INTEGRATION_STRATEGY =** Unchanged from v1: extract NexusAI, consume upstream `v4.10.x` unmodified. Reinforced — the compiler needs LocalAI only for constrained generation and embeddings, which is a narrow, stable, versionable contract.

**CPU_DEVELOPMENT_VIABILITY =** Improved by this design. IR generation is ~100–300 output tokens under a grammar, not a 250-token narration: seconds, not 37–80 s. The deterministic answer always ships in under five seconds; narration stays optional.

---

## B. CURRENT NEXUSAI TRUTH

### B.1 Git and worktree `[SOURCE]`
| Fact | Value |
|---|---|
| Branch / HEAD | `codex/forensic-hybrid-checkpoint-20260723` / `40717b8` (2026-07-22) |
| Commits in history | **1** (3,053 files: LocalAI v4.5.6 **+** already-merged NexusAI work) |
| Remote | `github.com/AbdulBasit929/NexusAI` — no upstream remote, no merge base |
| Uncommitted | **94 modified, 852 untracked** |
| Real diff (CR/whitespace-normalised) | **+30,979 / −6,407** (raw diff +62,389/−37,817 ≈ half CRLF churn) |

> Months of work exist only as uncommitted working-tree state in a one-commit repository. Largest **operational** risk in the project, independent of every architectural question.

### B.2 Runtime topology `[SOURCE: docker-compose.forensic-records.yaml]`
```
React SPA (served by LocalAI :8080)
  ├─ LocalAI :8080 ── /api/v1/forensics/*  (NexusAI-authored facade)
  │                      └──► forensic-records-api :8091 (Go, 38,956 LOC non-test)
  │                                ├──► forensic-postgres :5433 (TimescaleDB pg16, RLS)
  │                                ├──► forensic-nats :4222 (JetStream)
  │                                └──► LocalAI :8080 (embeddings, synthesis, face, ASR)
  └─ forensic-records-worker (Python, ~4,300 LOC, 8 adapters)
```
Live models: synthesis `qwen3-4b-instruct-2507-q4km-nxb21d-dev` (**Q4_K_M**, not Q8), embeddings `qwen3-embedding-0.6b`. CPU only, ~4 tok/s, ~15.7 GiB RAM, 6 GiB floor.

### B.3 Measured accuracy — the number that matters `[TEST: reports/nexusai-tl-audit-20260918/P0-baseline-report.md]`
62 English questions, 13 families, **every expected value computed by direct SQL independent of the API**, every response hand-adjudicated.

| Verdict | Count | Share |
|---|---:|---:|
| CORRECT | 16 | 26% |
| PARTIAL | 9 | 15% |
| SAFE_FAIL (honest clarification) | 5 | 8% |
| **WRONG — confidently wrong** | **26** | **42%** |
| WRONG — false "no records found" | 3 | 5% |
| ERROR (HTTP 500 shown to analyst) | 3 | 5% |

**Correct-or-properly-clarified 21/62 = 34%.** Answer stated in analyst text: **5 of 20**. Latency p50 0.56 s / p95 4.5 s / max 82 s.

### B.4 Defect status — re-verified in source today
| ID | Defect | Status | Evidence |
|---|---|---|---|
| D3 | HTTP 500 on every dynamic row listing | ✅ **FIXED** | `source_native_sql_executor.go:64-70`, with explanatory comment |
| D2 | Spurious GROUP BY from whole-question embedding | ✅ **FIXED** | `deterministic_semantic_compiler.go:436-443` empty-hint guard |
| **D1** | **Target/date filters silently dropped** | ❌ **OPEN** | file never references `.Filters` after init at :418; `semantic_frame.go:382` populates `frame.Filters`, nothing consumes it |
| D4 | Cross-family misrouting | ❌ OPEN | no family guard on the winning operation |
| D5 | Irrelevant retrieval returned as a match (audio) | ❌ OPEN | identifier tokens not required to match |
| D7 | "Explain …" → dictionary definition | ❌ OPEN | request-class classifier |
| **NEW** | **Measure field resolved from the whole question** | ❌ OPEN — *found this session* | `:461-463` sets `hint = question` when `MeasureFieldHint` is empty, running the same embedding resolver D2 was fixed for. Explains "largest transaction → COUNT" |

### B.5 Structural findings `[SOURCE]`
1. **The semantic layer is inferred, not curated.** `buildSourceNativeFieldCatalog(req, rows)` profiles sampled rows at request time. The catalog is therefore unstable across requests and unreviewable.
2. **`FieldDescriptorV1` has no description field.** `grep -c Description` on `source_native_algebra.go` = **0**.
3. **No join graph.** No declared inter-family relationships → cross-family 0/5.
4. **No Case entity.** `case_id` occurs **3×** in the schema (one nullable column on `evidence_items`); `collection_id` 224×, `tenant_id` 317×. No users/roles/membership tables. No case-creation path anywhere in the React app. Every analyst authenticates as the same shared service key.
5. **Dead parallel records store still wired.** `core/services/records/` — 2,188 LOC, file-backed, unmodified since the initial commit, serving `/api/records/*`, bypassing the Postgres RLS path. **Confirmed absent from upstream v4.10.0** `[UPSTREAM]`.
6. **The `/api/v1/forensics/*` facade is NexusAI-authored** — `core/http/endpoints/localai/records.go` confirmed absent upstream `[UPSTREAM]`.
7. **Catalog self-contradiction:** 5/79 templates certified; 52/104 operations certified; **33 templates `bounded_uncertified` in one catalog and `CERTIFIED` in the other** `[TEST]`.
8. **Ingestion families:** exactly 8 adapters — `cdr, ipdr, anpr, subscriber, tower_location, transaction, access_log, generic` `[SOURCE: worker.py:338+]`.
9. **`query.go` is 8,644 lines** of literal keyword routing ladder.

### B.6 Stale-report register
| Claim | Status |
|---|---|
| "0 wrong confident executions" | **Disproved** — 26 confident-wrong |
| Synthesis model is Q8 | **Wrong** — Q4_K_M |
| D2 / D3 open | **Stale** — both fixed in source |
| "~96% top-5 retrieval recall" solves selection | **Misread** — that is retrieval recall; end-to-end is 34% |
| 33 templates CERTIFIED | **Contradicted** by the template catalog |
| `nexusai_stim_maturity_matrix.json` (2026-09-14) | **Not authoritative** — predates the auth incident and the accuracy baseline |

---

## C. CURRENT LOCALAI UPSTREAM TRUTH `[UPSTREAM]`

| Field | Value |
|---|---|
| **CURRENT_LATEST_RELEASE** | **v4.10.0**, published **2026-09-17T19:46:38Z** |
| **NEXUSAI BASELINE** | **v4.5.6** (`docs/data/version.json`) `[SOURCE]` |
| Gap | 5 minor releases (4.6–4.10), ~3 months |
| v4.10.0 volume | 280 PRs / 382 commits / 633 files / +46,550−3,316 |
| Gallery | 1,221 → 1,847 models · Go 1.26.0 |

### C.1 Structured generation — available **today** at the v4.5.6 baseline `[SOURCE]`
This is the capability the new architecture is built on, and it is already in the repository:
- `core/schema/openai.go:167-176` — `JsonSchemaRequest{Type, JsonSchema}`, `JsonSchema{Name, Strict bool, Schema functions.Item}` → OpenAI-style **strict structured outputs**
- `core/schema/openai.go:237` — `Grammar string` on the prediction request (GBNF)
- `pkg/functions/function_structure.go:20-37` — `JSONFunctionStructure.Grammar()`, `NewSchemaConverter`, `GrammarFromBytes`
- `pkg/functions/grammars/json_schema.go:129-138` — **`enum` compiles to GBNF alternation** ← the keystone
- `pkg/functions/` also ships `json_mode.go`, `json_stack_parser.go`, `iterative_parser.go`, a PEG parser, and per-family grammar dialects (`llama31_schema.go`)
- Backend support: **llama.cpp** — the backend already in use

### C.2 Delta v4.5.6 → v4.10.0, filtered to relevance

**Security the fork is missing:**
- **v4.9.0 BREAKING — authentication is deny-by-default.** Every route requires credentials unless registered public; closes bypasses on `/moderations`, `/models`, `/backends`. `ApplicationConfig.PathWithoutAuth` for exceptions.
- **v4.6.0** — gallery config URL fetches validated against private/loopback/metadata addresses: **SSRF fix on `POST /models/apply`**.
- **v4.10.0** — four CVEs: `ip-address`, `containerd`, **`react-router`** (the fork's heavily-modified UI depends on it).
- **v4.8.0** — tar hardlink traversal validation; cyclic JSON-schema `$ref` rejection; inline fine-tune reward code now opt-in.
- **v4.7.0** — OIDC accepts EC/PS/EdDSA, not RS256-only.

**Capability relevant to this design:**
- **KNN routing (4.9)** — *"similarity-weighted voting over a persisted, curated prompt corpus — no classifier model needed."* Candidate mechanism for deterministic family/intent routing.
- **audio.cpp (4.8)** — one process for TTS, transcription, VAD, **diarization**, separation. **moss-transcribe-cpp diarization (4.7)**. **Streaming TTS (4.7, 17× latency)**. **Voice profiles + `/api/voice-profiles` (4.7)**.
- **`POST /backend/load` + realtime pipeline warmup (4.6)** — model pre-warm; removes cold start from the latency budget.
- **Context compression (4.9)**, **PII pseudonyms (4.9)** (CNIC relevance), **HunyuanOCR (4.9)**, **`local-ai benchmark` (4.10)**, **`credentials.yaml` (4.10)**, **per-model `env:` (4.10)**, **`template.system_messages_after_first` (4.10)**, **`enable_thinking=false` honoured across text backends (4.10)**.
- **Backends:** `vllm-cpp` (C++20, no Python, CPU/CUDA/Metal/Vulkan, ~1.045× vLLM), `audio-cpp`, **`bonsai`** (1-bit/ternary — CPU relevant), `trellis2cpp`, `magpie-tts-cpp`, `moss-tts-cpp`, `valkey-store`.
- **Platform/UI:** React bundle **2.8 MB → 808 KB (3.48×)**; traces 21 MB → 7 KB; `/app/activity`; unified models/backends pages; **fleet dashboard with VRAM/RAM/CPU/disk gauges (4.10)**; per-node VRAM budget; `LOCALAI_VRAM_BUDGET`; paginated `/api/traces`.
- **Distributed:** advisory-lock wedges fixed, orphaned workers self-terminate, phantom stubs fixed, `in_flight` leaks fixed, cold loads as durable jobs.

---

## D. LOCALAI DELTA MAP `[SOURCE: git diff --ignore-cr-at-eol -w]`

| Zone | Files | Real lines | Verdict |
|---|---:|---:|---|
| NexusAI-only trees (`api/`, `ingestion/`, `db/`, `scripts/`, `configuration/`, `reports/`, `docs/design/`) | ~950 | — | Pure NexusAI. **Moves out cleanly.** |
| LocalAI core Go — genuinely modified | 8 | **~330** | `auth/features.go` 21 · `agents.go` 240 · `agent_collections.go` 117 · `api_instructions.go` 9 · `distributed.go` 5 · `transcription.go` 10 · `schema/localai.go` 17 · `routes/*` 20 |
| LocalAI core Go — NexusAI-authored files | 3 | ~1,320 | `endpoints/localai/records.go` 243 · `routes/records.go` 28 · `services/records/**` 2,188 — **all confirmed absent upstream → DELETE** |
| `core/services/agents*` forensic integration | 8 | **~2,040** | `forensic_direct.go` **1,031** · `agentpool` 509 · `records_tools.go` 178 · others — **product logic in the platform → MOVE OUT** |
| React UI | ~24 | **~3,600** | `AgentChat.jsx` 1,408 · `RecordsIntelligence.jsx` 1,333 · `App.css` 428 · + untracked analyst tree |
| Backend/infra | 5 | ~100 | `faster-whisper/backend.py` 51 · Dockerfile 6 |
| Generated swagger | 3 | ~14,400 | Regenerate, never merge |

**`[INFERENCE]`** Excluding generated files the fork modifies ~6,000 real lines of LocalAI, of which **~3,400 should be deleted or moved out entirely**. The genuinely-must-keep core delta is **~330 lines across 8 files** — small enough to eliminate via adapters or config.

---

## E. LOCALAI ADOPTION MATRIX

| Feature | Upstream | NexusAI today | Action | Why | Risk |
|---|---|---|---|---|---|
| **Strict JSON-Schema structured output + GBNF + enum→grammar** | **4.5.6 (have it)** | **unused** | **REUSE_AS_IS — immediately** | Makes invalid IR structurally impossible; the keystone of §G | **Low — highest value/effort ratio in the project** |
| Deny-by-default auth | 4.9 | pre-4.9 + own fail-closed guard | REPLACE_CUSTOM_WITH_UPSTREAM (admin plane) | Matches required posture; prevents the 2026-09-17 incident class | Low |
| Gallery SSRF validation | 4.6 | absent | REUSE_AS_IS | Security | Low |
| OIDC EC/PS/EdDSA | 4.7 | none | REUSE_AS_IS (operators) | Free | Low |
| Users / API keys / quotas | ≤4.10 | LocalAI owns all identity | KEEP_NEXUSAI_CUSTOM (analysts) / REUSE (operators) | Analyst identity must be case-scoped — that is domain | Medium — never two analyst identity systems |
| `credentials.yaml`, per-model `env:` | 4.10 | ~40 `FORENSIC_*` env vars in compose | REUSE_AS_IS | Large config simplification | Low |
| OpenAI-compatible chat | ≤4.10 | used for narration | WRAP_VIA_LOCALAI_API | Correct already | Low |
| Tool calling | ≤4.10 | via agent pool | KEEP but **relocate** | Protocol is platform; which tools and their authority is domain | Medium |
| **KNN routing** | 4.9 | months of failed LLM selectors | **PORT_SELECTIVELY / evaluate in P7** | Deterministic corpus-vote routing; candidate for family/intent | Medium — must beat the incumbent on the golden set |
| Context compression | 4.9 | none | DEFER | Only matters for long follow-up chains | Low |
| `backend/load` pre-warm | 4.6 | none | **REUSE_AS_IS in P3** | Removes cold start from IR-generation latency | Low |
| `enable_thinking=false`, `system_messages_after_first` | 4.10 | n/a | REUSE_AS_IS | Qwen3.x correctness; thinking tokens are pure cost under a grammar | Low |
| Embeddings | ≤4.10 | `qwen3-embedding-0.6b` | REUSE_AS_IS | Correct | Low |
| Vector store / collections | ≤4.10 | used as the *case* container — wrong | ADAPT | A collection is storage, not a case | Medium |
| Reranking | ≤4.10 | absent | REUSE_AS_IS in P7 | Fixes D5-class irrelevant matches | Low |
| **audio.cpp** (STT+TTS+VAD+**diarization**) | 4.8 | patched faster-whisper + ASR HTTP | REPLACE_CUSTOM_WITH_UPSTREAM | One process replaces a patched fork backend; unlocks diarization | Medium — re-verify transcripts |
| `core/services/facerecognition` | ≤4.10 `[UPSTREAM]` | custom `/v1/face` call | WRAP_VIA_LOCALAI_API | Keep NexusAI semantics (candidate similarity, never identity) | Low |
| HunyuanOCR | 4.9 | Tesseract/Paddle; photo OCR is noise at 0.90 confidence | PORT_SELECTIVELY / evaluate P7 | Current behaviour is a truthfulness defect | Medium |
| ANPR (yolo-v9-t + fast-plate-ocr) | not upstream | custom, works | KEEP_NEXUSAI_CUSTOM | No upstream equivalent | — |
| 3D / video / image generation | 4.8–4.10 | n/a | **NOT_RELEVANT** | Generative media has no place in evidence handling | — |
| Metrics / traces / `/app/activity` | 4.8 | worker :9109 only | REUSE_AS_IS (system plane) / KEEP_CUSTOM (investigation audit) | Two different audiences | Low |
| `local-ai benchmark` | 4.10 | `scripts/benchmark_forensic_models.py` | REPLACE_CUSTOM_WITH_UPSTREAM | Delete bespoke scripting | Low |
| VRAM budget / scheduler / fleet | 4.8–4.10 | n/a | REUSE_AS_IS at Phase B/C | Future GPU | Low |
| Admin SPA | 4.10 | forked and modified | **REUSE_AS_IS, unmodified, operators only** | Stop maintaining a fork of someone else's console | Medium |
| Bundle optimisation (3.48×) | 4.8 | absent in fork | REUSE_AS_IS | Free performance | Low |
| Analyst portal | — | `src/analyst/**` fused into admin SPA | **KEEP_CUSTOM, extract to its own app** | — | Medium |

---

## F. TARGET ARCHITECTURE

```
┌──────────────────────────────────────────────────────────────────────┐
│ ANALYST WEB APP   (own origin, own build, own design system)         │
│ /login /  /cases /cases/new                                          │
│ /cases/:id/{overview,evidence,investigate,timeline,activity}         │
└────────────────────────────┬─────────────────────────────────────────┘
                             │ session cookie / short-lived JWT
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│ NEXUSAI PRODUCT API  (the only API an analyst ever reaches)          │
│  identity · cases · membership · evidence · jobs · query ·          │
│  citations · source locators · activity · capability discovery      │
├──────────────────────────────────────────────────────────────────────┤
│ ╔══════════════════════════════════════════════════════════════════╗ │
│ ║ GOVERNED SEMANTIC COMPILER          ◄── the product's core       ║ │
│ ║  SEMANTIC LAYER (curated, versioned, reviewed — DATA, not code)  ║ │
│ ║    entities · dimensions · measures · metrics · joins · synonyms ║ │
│ ║  ↓ deterministic scope + family narrowing (500 fields → ~25)     ║ │
│ ║  ↓ ENUM-CONSTRAINED IR GENERATION (JSON Schema → GBNF)           ║ │
│ ║  ↓ hard validation → self-correction (one bounded retry)         ║ │
│ ║  ↓ compile → PARAMETERIZED SQL / typed retrieval                 ║ │
│ ║  ↓ VERIFY (constraint-applied · shape · scope · plausibility)    ║ │
│ ║  ↓ ABSTAIN → specific clarifying question, never a guess         ║ │
│ ╚══════════════════════════════════════════════════════════════════╝ │
│  EVIDENCE SERVICE  items · versions · custody · storage · lineage    │
│  RETRIEVAL SERVICE exact · phrase · FTS · semantic · hybrid · rerank │
│  JOB SERVICE       NATS JetStream                                    │
│  ACTIVITY SERVICE  user activity + investigation audit               │
└──────┬───────────────────────────────┬───────────────────────────────┘
       │                               │  internal only — never analyst-reachable
       ▼                               ▼
┌──────────────────┐        ┌──────────────────────────────────────────┐
│ PostgreSQL /     │        │ LOCALAI  (PINNED UPSTREAM v4.10.x,       │
│ TimescaleDB      │        │           ZERO NEXUSAI PATCHES)          │
│ · nexus.*  identity,      │  constrained generation (JSON Schema/    │
│   cases, members,│        │  GBNF) · embeddings · rerank · STT +     │
│   activity  (RLS)│        │  diarization · TTS · face & vision       │
│ · semantic_layer.*        │  primitives · model/backend lifecycle    │
│   entities, fields,       │  ┌────────────────────────────────────┐  │
│   joins, synonyms │       │  │ llama.cpp · vllm-cpp · audio-cpp · │  │
│ · forensic.* evidence,    │  │ bonsai ··· CPU now, GPU/fleet later│  │
│   records, derived│       │  └────────────────────────────────────┘  │
│   custody   (RLS)│        │  LocalAI Admin SPA — operators only      │
└──────────────────┘        └──────────────────────────────────────────┘
       ▲
┌──────┴─────────────────────────────────────┐
│ PROCESSOR WORKERS (Python)                 │
│ structured adapters · documents · OCR ·    │
│ ANPR · ASR · face · video frames           │
└────────────────────────────────────────────┘
```

**Platform boundary.** LocalAI owns model serving, backend lifecycle, hardware abstraction, **constrained generation**, tool protocol, embeddings, rerank, speech incl. diarization, vision and face primitives, MCP, distributed inference, platform metrics, operator admin UI. NexusAI owns analyst identity, cases and membership, the **semantic layer**, the evidence model and lineage, forensic normalization, capability governance, query semantics, typed execution, case authorization, cross-family rules, Fact Packets, citations, investigation activity, analyst UX.

**Hard rule:** no analyst-originated request ever reaches LocalAI directly. LocalAI sits on an internal network behind a service credential the analyst never holds.

---

## G. TARGET QUERY ARCHITECTURE — THE GOVERNED SEMANTIC COMPILER

> **The answer to the team lead.** We stop defining operations. We define the **data** once — a semantic layer — and compile every question against it. 20 families × ~25 fields ≈ **500 descriptors** generate an unbounded question space. 79 templates generate 79 question shapes. The semantic layer is an order of magnitude smaller to maintain and combinatorially more expressive.

### G.1 The pipeline

```
English question + case scope + authenticated identity
  │
S1 REQUEST CLASSIFICATION            deterministic
   {governed_analysis | evidence_retrieval | product_help |
    contextual_followup | out_of_scope}                     → fixes D7
  │
S2 LITERAL & CONSTRAINT EXTRACTION   deterministic, precision over recall
   MSISDN · IMEI · IMSI · plate · CNIC · IP · account · money ·
   absolute & relative dates · quoted phrases · file names · durations
   Every literal is recorded as a REQUIRED OBLIGATION for S9.
  │
S3 SCOPE RESOLUTION                  case → authorized evidence, families
   present in the case, RLS predicate. Families present in the CASE and
   families named in the QUESTION are both recorded.
  │
S4 CATALOG NARROWING                 deterministic, THIS IS WHAT MAKES A
   SMALL MODEL WORK. Family + question terms + literal types reduce the
   authorized catalog from ~500 fields to a working set of ~15–30.
   Cheap, explainable, auditable. NOT an embedding ranker.
  │
S5 IR GENERATION                     ◄── the LLM's ONLY structural job
   JSON Schema built AT REQUEST TIME from the working set, where
   every field_id slot is an `enum` of exactly the authorized IDs.
   LocalAI compiles it to GBNF; the decoder CANNOT emit anything else.
   The model fills: project · filters · group_by · measures ·
   time_bucket · having · sort · limit.
   It NEVER emits SQL. It NEVER names a table. It NEVER picks an
   operation ID. There are no operation IDs.
  │
S6 HARD VALIDATION                   deterministic, total
   field exists in the authorized working set · aggregate ∈
   AllowedAggregates · filter op ∈ AllowedFilters · type compatibility ·
   Groupable/Projectable/Sortable respected · Sensitivity/RedactionState
   honoured · join declared in the join graph · limits bounded
  │
S7 SELF-CORRECTION                   ONE bounded retry, structured
   Validation errors are returned to the model as a typed diff
   ("field_id f_x is not groupable; groupable fields are …").
   [RESEARCH] self-correction is the single most effective technique for
   on-prem open models (+3.65 pp on Llama-3.1-8B, +3.06 pp on
   CodeLlama-7B, both p<10⁻¹⁰) and is MOST valuable at small sizes.
   Exactly one retry. Never a loop.
  │
S8 COMPILATION                       deterministic engine, parameterized
   IR → SQL with scope predicate always injected, joins from the join
   graph, args bound — never string interpolation.
   (This engine already exists: source_native_sql_executor.go)
  │
S9 VERIFICATION                      ◄── where "accurate" is actually won
   CONSTRAINT-APPLIED : every obligation from S2 appears in the plan.
                        One unbound → ABSTAIN. Never silently drop.  → D1
   FAMILY            : no field from a family absent from the question
                        and the case.                                → D4
   SHAPE             : count ⇒ scalar · rank ⇒ ordered+limited ·
                        list ⇒ rows · comparison ⇒ ≥2 groups
   SCOPE             : result row count ≤ scope total; a "filtered"
                        answer equal to the unfiltered total is a
                        red flag, not a result
   PLAUSIBILITY      : non-negative counts, dates within evidence range,
                        no empty-filter aggregate presented as filtered
  │
S10 ABSTAIN OR ANSWER
    Fail any S9 check, or S7 exhausted → ONE SPECIFIC CLARIFYING
    QUESTION naming the ambiguity. THIS IS A SUCCESS.
    "By 'total' do you mean call duration, data volume, or amount?"
  │
S11 FACT PACKET v2                   facts are RESULT VALUES and CITED
    PASSAGES. "CDR record count = 8,642" is a fact. "Template: …",
    "Route: …", "Planner Confidence" are NOT — they move to `trace`.
  │
S12 DETERMINISTIC ANSWER             built from facts, ALWAYS, < 5 s
    "There are 8,642 CDR records in this case."
  │
S13 NARRATION (OPTIONAL)             streamed after S12, grounded-validated.
    Timeout or rejection → the analyst keeps S12. Never blocks.
  │
S14 PRESENTATION                     answer · result · citations ·
    derivation (collapsed) · limitations · follow-ups
```

### G.2 Why this is the right architecture — the evidence

| Claim | Evidence |
|---|---|
| A semantic layer + IR + deterministic compiler beats raw text-to-SQL on enterprise data by a very large margin | **94.15%** vs **2.2%** (DAIL-SQL+GPT-4o) and **23–26%** (Spider-Agent) on Spider2-snow, 547 enterprise instances `[RESEARCH]` |
| Grounding in a semantic layer is the dominant factor | text-to-SQL 40% → **86–95%** with a semantic layer; frontier raw 70–85% on *clean* data only `[RESEARCH]` |
| A small local model must never write SQL | on-prem BIRD: Qwen2.5-Coder 7B **39.1%**, 14B 47.4%, 32B 50.4%; Llama-3.1-8B **32.9%**, 70B 49.2% `[RESEARCH]` |
| Hallucinated identifiers can be eliminated, not merely reduced | `enum` → GBNF alternation in `pkg/functions/grammars/json_schema.go:129-138`; strict `json_schema` response format in `core/schema/openai.go:167-176` — **present at the v4.5.6 baseline** `[SOURCE]` |
| Self-correction is the best available technique at small model sizes | +3.65 pp / +3.06 pp, p<10⁻¹⁰, "robust, near-free win" `[RESEARCH]` |
| Do **not** over-invest in embedding schema linking | embedding linking at **96.5% gold-table recall** was *statistically indistinguishable from no linking* (p≥0.55); lexical linking **hurt** `[RESEARCH]`. Corroborated locally: D2 and the new measure defect were both caused by embedding signals on structural roles `[SOURCE]` |
| Self-consistency is not worth it here | +0.13 pp for ~5× token cost (p=0.86) — unaffordable at 4 tok/s `[RESEARCH]` |
| The IR is sound | `SourceNativePlanV1` is strictly richer than the published SMQ and already has a strict parser `[SOURCE]` |

### G.3 What changes for the LLM
| | Before (failed 0/5) | After |
|---|---|---|
| Task | classify among ~104 opaque operation IDs | fill 6 typed slots |
| Vocabulary | free text | **enumerated, decoder-enforced** |
| Invalid output | possible (`MALFORMED_OUTPUTS=2`) | **impossible** |
| Hallucinated field | possible | **impossible** |
| Wrong semantics | undetected → confident wrong answer | caught by S6/S9 → **abstain** |
| Retries | none | one structured self-correction |
| Output size | ~250 narration tokens | ~100–300 constrained tokens |
| On failure | confident wrong answer | **clarifying question** |

### G.4 Honest risk register
| Risk | Mitigation |
|---|---|
| **`[UNKNOWN]`** 4B IR accuracy on our data is unmeasured | **WI-0 spike** with a pre-declared decision rule (§T). Measure before committing. |
| Semantic-layer curation is real work (~500 descriptors) | It is **data, not code** — YAML, reviewable, diffable, testable. Bootstrap from the existing profiler, then curate per family. One family at a time; CDR first. |
| Catalog narrowing (S4) could drop the right field | S4 failures are **recall failures → abstention**, not wrong answers. Instrument recall on the golden set; widen the working set when it misses. |
| Small model picks a plausible-but-wrong measure | S9 SHAPE + PLAUSIBILITY catch most; ambiguity between two valid measures → **S10 clarify**, which is the correct product behaviour. |
| Latency on CPU | ~100–300 constrained tokens ≈ seconds. S12 always ships in <5 s regardless. `backend/load` pre-warm removes cold start. |
| Joins are the hardest case | Join graph is **declared, not inferred**. An undeclared join is not attempted — it abstains. Cross-family is P7, not P3. |

### G.5 Non-negotiables
1. The LLM never emits SQL, never names a table, never picks an operation.
2. Every field the model can reference is enumerated in the schema at request time.
3. Every extracted literal is an obligation; failing to bind one abstains.
4. Every query is parameterized and carries the scope predicate.
5. Abstention is a success. A confident wrong answer never is.
6. The deterministic answer is always produced; narration is always optional.

---

## H. TARGET DATA ARCHITECTURE — including the semantic layer

### H.1 The semantic layer (new — the foundation of §G)
Persisted, versioned, reviewed. **Declarative data, not code.**

```yaml
entity: cdr
display_name: Call Detail Records
description: One row per call, SMS or data session recorded by the operator.
grain: one call event
fields:
  - field_id: cdr.msisdn
    display_name: Subscriber number
    description: The number belonging to the subscriber this record is about.
    synonyms: [msisdn, caller, subscriber number, their number, phone number]
    source_names: [MSISDN, A_PARTY, CALLING_NUM]
    type: identifier/msisdn
    sensitivity: pii
    filters: [EQ, IN, CONTAINS]
    groupable: true
    projectable: true
  - field_id: cdr.duration_seconds
    display_name: Call duration
    description: Length of the call in seconds. Zero for SMS and data.
    synonyms: [duration, call length, how long, talk time]
    type: number/duration
    aggregates: [SUM, AVG, MIN, MAX]
metrics:
  - metric_id: cdr.total_talk_time_minutes
    description: Total talk time in minutes across the selected calls.
    expr: SUM(duration_seconds)/60.0
joins:
  - to: subscriber
    on: [cdr.msisdn = subscriber.msisdn]
    cardinality: many_to_one
    description: Links a call to the registered subscriber.
```

**Gap vs. today `[SOURCE]`:** `FieldDescriptorV1` has `FieldID, SourceName, SourceNames, NormalizedName, EffectiveType, AllowedFilters, AllowedAggregates, Projectable, Groupable, Sortable, Sensitivity, RedactionState, FamilyProvenance, SourceProvenance` — a genuinely good start — but **no `Description`, no `Synonyms`, no `DisplayName`, no metrics, no joins**, and it is **rebuilt from sampled rows at every request** rather than persisted. Add the missing fields, persist per `(case, family)`, and curate.

### H.2 The rest
| Layer | Content | Status |
|---|---|---|
| **Identity & workspace** *(new)* | `nexus.orgs/users/sessions/cases/case_members/case_activity`, RLS on membership | **Does not exist** |
| **Semantic layer** *(new)* | `semantic_layer.entities/fields/metrics/joins/synonyms`, versioned | **Does not exist** |
| **Evidence control plane** *(keep)* | `evidence_items`, versions, custody events, storage objects, source links | Good; add real `case_id` FK |
| **Structured records** *(keep)* | `forensic.records` + `cdr_records` + `generic_records`, canonical normalized fields, `record_entities` | Working foundation |
| **Derived artifacts** *(keep)* | OCR text, transcripts, ANPR sightings, face observations, frames — with lineage to source version + model/backend id + confidence | Good |
| **Retrieval indexes** | Postgres FTS (exact/phrase), pgvector or LocalAI store (semantic), typed per-family indexes | Choose by evidence type — never force everything through SQL *or* through RAG |
| **Citations** | `(evidence_id, version_id, source_file, row_number, row_hash, page, char_span, t_start, t_end, frame_ts)` | Partly present in the projection SQL; make it the single contract |
| **Jobs & audit** | `records_ingest_jobs`, `processing_runs/events`, `records_audit_log` | Keep; separate user activity from system log |

---

## I. ALL-FAMILY CAPABILITY MATRIX

Maturity: ABSENT · SCAFFOLDED · FUNCTIONAL · DEMO_READY · BASELINE · CERT_PENDING · CERTIFIED

| Family | Current | Semantic layer | Query path | Upstream reuse | CPU | GPU | Priority |
|---|---|---|---|---|---|---|---|
| Generic structured | FUNCTIONAL | **to curate** | compiler | — | OK | OK | **P2/P3** |
| CDR | FUNCTIONAL (2/16) | **first to curate** | compiler | — | OK | OK | **P2 first** |
| IPDR | FUNCTIONAL (2/5) | to curate | compiler | — | OK | OK | P2 |
| ANPR | FUNCTIONAL (2/6) | to curate | compiler | — | OK | OK | P2 |
| Access log | FUNCTIONAL (1/4) | to curate | compiler | — | OK | OK | P2 |
| Subscriber | FUNCTIONAL (1/3+2P) | to curate | compiler + join to CDR | — | OK | OK | P2 |
| Tower / site | PARTIAL (0/2) | to curate | compiler | — | OK | OK | P2 |
| Financial | PARTIAL (0/3) | to curate | compiler | — | OK | OK | P2 |
| Documents | **FUNCTIONAL (4/6 — best)** | n/a | retrieval, not SQL | rerank | OK | OK | P7 |
| Plain text | FUNCTIONAL | n/a | exact/phrase/FTS | — | OK | OK | P7 |
| RAG / knowledge | SCAFFOLDED | n/a | hybrid + rerank | embeddings, rerank | slow | OK | P7 |
| OCR (images) | **UNSAFE** — noise stored at 0.90 confidence | n/a | derived text | HunyuanOCR | slow | REC | **P7 (safety)** |
| OCR (documents) | ABSENT for scanned PDFs | n/a | derived text | HunyuanOCR | slow | REC | P7 |
| Images | FUNCTIONAL (1/4) | metadata fields | metadata + similarity | image embeddings | slow | REC | P7 |
| Video | SCAFFOLDED (0/1) | metadata fields | derived events | — | slow | REQ | P7 |
| Audio / STT | FUNCTIONAL (1/3) | transcript fields | derived text + timestamps | **audio.cpp + diarization** | slow | REC | P7 |
| TTS | FUNCTIONAL (platform) | n/a | n/a | 4.7 streaming | OK | OK | DEFER |
| Face | SCAFFOLDED, off by default | n/a | candidate similarity only | `facerecognition` | slow | REQ | P7 |
| **Cross-family** | **WEAK (0/5)** | **join graph** | compiler + declared joins | — | OK | OK | **P7** |

---

## J. AUTH / CASE / EVIDENCE MODEL

**Decision `[RECOMMENDATION]`: NexusAI owns analyst identity and cases. LocalAI identity becomes operator-only.**

Rationale `[SOURCE]`: no users/roles/cases/membership tables exist; `case_id` occurs 3× vs `collection_id` 224×; the UI has no case-creation path; and the forensic API authenticates with a **single shared service key plus a trusted tenant id** — so today **every analyst is the same principal**. Case-level authorization cannot be built on that.

```
Analyst ──login──► NexusAI API ──session──► case-scoped RLS on every query
                      └── service credential ──► LocalAI (internal network only)
Operator ──login──► LocalAI admin SPA (deny-by-default auth, v4.9+) — separate origin
```

Roles `owner · investigator · contributor · viewer` + org `admin`. Every case-scoped request carries `(user_id, case_id)`; RLS enforces membership **in the database**, not in Go. Case lifecycle `draft → active → under_review → closed → archived`. Evidence attaches to a case; a LocalAI collection becomes an internal storage detail. Keep the existing fail-closed startup guard `validateForensicAuthConfig` — it is correct and tested `[SOURCE]`.

---

## K. TARGET UI / UX

### Routes
| Route | Purpose | States |
|---|---|---|
| `/login` | sign in / accept invite | error, locked, SSO |
| `/` | dashboard: my cases, recent activity, processing, what needs attention | empty → "Create your first case" |
| `/cases`, `/cases/new` | list/filter/search; create with name, reference, members | validation |
| `/cases/:id/overview` | summary, evidence counts by family, **data-quality notes**, members | — |
| `/cases/:id/evidence` | inventory, filters, add data, per-item detail | processing / failed / empty |
| `/cases/:id/evidence/:eid` | per-type viewers: document (page), table (rows), image (OCR/ANPR overlay), audio (transcript + seek), video (timeline + frames) | lineage always visible |
| `/cases/:id/investigate` | **Ask** — answer-first, findings, table/timeline, citations, follow-ups | **clarify** / zero-result / unsupported / partial |
| `/cases/:id/timeline` | chronology across families | — |
| `/cases/:id/activity` | who did what, questions asked, exports | — |
| `/settings`, `/admin` | profile; org/users/health | authorized only |

### Answer presentation contract — fixed order, never inverted
**(1) the answer sentence · (2) the number / table / timeline · (3) citations with openable sources · (4) how this was derived (collapsed) · (5) limitations · (6) follow-ups.**
Process jargon ("Executed bounded source-native typed algebra over 8642 authorized source rows…", column `M1`, "Records Row Count: 1" for a count of 8,642) is a **defect** `[TEST]`.

### The clarification surface — new and important
Abstention is now a first-class outcome, so it needs a first-class UI: the ambiguity named in plain language, 2–4 tappable options drawn from the semantic layer's display names, and one click to re-run. Designed well, this reads as competence. Designed badly, it reads as failure. It is the difference between *"By 'total' do you mean call duration, data volume, or transaction amount?"* and *"Query could not be planned."*

### Visual system
Light: cool neutral app background (**not white**), elevated white working surfaces, soft blue conversational surfaces, restrained professional blue accent. Dark: deep cool slate backdrop with **3–4 intentional surface levels**. Excluded: flat white canvas, flat black canvas, purple AI gradients, neon/cyberpunk, glassmorphism, marketing hero blocks, card soup, decorative motion. Analysts never choose model, agent, operation, processor, backend, SQL or vector index.

### Full UX specification
**The complete UX contract now lives in [`docs/ux/NEXUSAI_PRODUCT_UX.md`](../ux/NEXUSAI_PRODUCT_UX.md)** — information architecture, screen contracts with every state, the provenance/citation system, the clarification surface, the visual token system, accessibility, responsive behaviour, the component inventory, and the definition of done. This section is the summary; that document is authoritative.

### UX references `[VERIFIED 2026-09-21]`
> **Corrected:** the earlier path `https://github.com/satiricalguru/Local-Mindui/ux` returns HTTP 404. The real reference is **`https://github.com/satiricalguru/Local-Mind`**, which does exist and **was inspected this session**.
>
> It is an **Electron desktop app for running local AI offline** — Electron + React 18 + Vite + TanStack Router + Tailwind v4, SQLite, pnpm/Turborepo — whose surfaces are *playgrounds*: chat with live tok/s metrics, image generation, audio/video synthesis, Whisper STT, TTS, and a Model Hub. Themes are "GPU-Noir Dark" and "Vanilla Light", with **glassmorphic** styling and a CPU/RAM/GPU/temperature sidebar.
>
> **Assessment:** a good reference for *craft and density*, a bad reference for *information architecture*. **Take:** the feel of a tool rather than a website; honest system state surfaced in the interaction (adapted to *evidence* state, not GPU state); two named opinionated themes driven by CSS custom properties; a single model-management surface clearly separated from use (→ our `/admin`). **Reject:** glassmorphism and the GPU-Noir aesthetic (both excluded by the product directive); the playground-per-modality IA (adopting it would make NexusAI exactly the "generic local-chat application" the directive forbids); the end-user telemetry sidebar; the single-user Electron shape, which cannot carry case membership and row-level security. Full assessment in `docs/ux/NEXUSAI_PRODUCT_UX.md` §2.
>
> The current LocalAI UI was also inspected `[SOURCE: router.jsx + pages/]`: its console-rail pattern (`ConsoleLayout`, `buildConsole`/`operateConsole`), route-level code splitting with hover preloading (`preloadRoute`), and capability-gated routes (`RequireFeature`) are worth keeping as *patterns* in the new analyst app.

### The provenance differentiator `[RESEARCH 2026-09-21]`
Industry analysis in 2026, citing a 2025 study, reports that **more than half of RAG-generated citations exhibit post-hoc rationalization** — the model decides its answer first, then scans retrieved documents for surface-level token matches to manufacture a reference. **NexusAI is structurally immune to this**, because the answer is computed from Fact Packet values produced by SQL over authorized rows and the citation *is the row that produced the value*. This is the most valuable property of the architecture and the UI must communicate it: claim-level attribution, deep-links to exact locators, and visible degradation when evidence is weak.

---

## L. MODEL / HARDWARE STRATEGY

Hardware: ThinkPad T16, i7-1260P (12P/16L), ~15.7 GiB RAM, Intel iGPU, no CUDA, Win11 + WSL2 + Docker Desktop. Measured: ~4 tok/s, 6 GiB deployment floor `[TEST]`.

| AI role | CPU now | GPU later | Note |
|---|---|---|---|
| Request classification (S1) | **not a model** | — | deterministic |
| Literal extraction (S2) | **not a model** | — | deterministic |
| Catalog narrowing (S4) | **not a model** | — | deterministic; **not** an embedding ranker `[RESEARCH]` |
| **IR generation (S5)** | **CPU_SLOW_BUT_USABLE** | GPU_RECOMMENDED | ~100–300 constrained tokens; **the number to measure in WI-0** |
| Self-correction (S7) | CPU_SLOW_BUT_USABLE | GPU_RECOMMENDED | one retry only |
| Narration (S13) | **CPU_SLOW — off critical path** | GPU_RECOMMENDED | 37–80 s measured; must stay optional |
| Embeddings | CPU_DEV_OK | GPU_RECOMMENDED | semantic retrieval only, never structural roles |
| Reranking | CPU_SLOW_BUT_USABLE | GPU_RECOMMENDED | P7 |
| OCR (documents) | CPU_DEV_OK | — | Tesseract/Paddle |
| OCR (photos, quality) | CPU_SLOW | GPU_RECOMMENDED | HunyuanOCR |
| STT / diarization | CPU_SLOW_BUT_USABLE / CPU_SLOW | GPU_RECOMMENDED | audio.cpp |
| Vision / face / video | CPU_SLOW | GPU_REQUIRED | defer to P7 |

**Model tier ladder for S5 — escalate only on a measured miss, never on a hunch:**
`Qwen3-4B-Instruct Q4_K_M (current)` → `Qwen2.5-Coder-7B / Qwen3-8B Q4` → `14B Q4 (GPU)` → `32B (GPU)`.
Escalation requires a WI-0-style measurement on the golden IR set showing the current tier below threshold. This is **not** model roulette: it is one measurement against a pre-declared rule.

**Policy:** when a capability is too heavy, return a **truthful capability state** and record it in the capability matrix. Never fake it, never silently degrade.

---

## M. SECURITY MODEL

1. **Analyst identity is NexusAI's**, case-scoped RLS in the database. Today one shared service key = one principal `[SOURCE]`.
2. **LocalAI is internal-only** — no analyst-reachable route, no published port on the analyst network.
3. **Adopt upstream deny-by-default auth (v4.9)** for operators; keep the fail-closed startup guard.
4. **Close the v4.6–v4.10 security gap** — SSRF on gallery fetch, tar hardlink traversal, cyclic `$ref`, and four v4.10 CVEs including **react-router**.
5. **Delete the dead `/api/records/*` surface** — 2,188 LOC of file-backed store bypassing the Postgres RLS path. An authorization bypass by architecture.
6. **The compiler is a security control.** The `enum`-constrained schema plus S6 validation is what prevents a model from naming a field it is not authorized to see. `Sensitivity` and `RedactionState` must be enforced at S4 (never enumerate an unauthorized field) **and** S6 (reject if one appears), not only at projection.
7. Never string-interpolate SQL. The compiler emits parameterized statements only — already true `[SOURCE]`.
8. Uploads: content-addressed storage with SHA-256 already exists — keep; enforce type allowlist, size caps, traversal checks.
9. PII: CNIC masking enforced server-side at projection. Evaluate upstream request-scoped pseudonyms `[UPSTREAM 4.9]`.
10. **Never "fix" a crash-loop by disabling auth** — this already happened once, live, for a whole session `[TEST]`.

---

## N. OBSERVABILITY / ACTIVITY MODEL

| Plane | Audience | Content |
|---|---|---|
| **User activity** | analyst | "You added 3 files", "You asked X", "Report exported" |
| **Investigation audit** | analyst + reviewer | evidence added/reprocessed, custody events, **every question with its compiled plan and result**, exports |
| **AI execution trace** | admin / debug | working set, generated IR, validation errors, self-correction diff, compiled SQL shape, model, tokens, latency, verification verdicts, abstention reason |
| **System log** | ops | service logs, job failures, DLQ |
| **Admin health** | ops | LocalAI metrics/traces/benchmark, backend status, queue depth, VRAM |

**New requirement:** the compiled plan and the abstention reason are **audit records**, not debug output. In a forensic product, "why did the system answer this way" must be reconstructable months later. Store the IR with the answer.

Rule: the analyst sees *what happened to the investigation*. The admin sees *what happened to the machine*. Plumbing never appears in an answer.

---

## O. CLAUDE + CODEX SHARED WORK PROTOCOL

**Diagnosis `[SOURCE]`:** the current protocol is the largest productivity sink. `NEXUSAI_CONTINUATION.md` is **874 KB / 12,836 lines** of append-only prose; four competing root-level directives (`NEXUSAI_MASTER_DIRECTIVE.md` 128 KB, `NEXUSAI_NEXT_CHAT_PROMPT.md` 61 KB, `NEXUSAI_TL_DIRECTIVE_PROMPT_20260918.md` 24 KB); 40+ report directories; and `nexusai_stim_maturity_matrix.json` still points at a phase predating the two most important events in the project. No agent can read this in a session, so each one re-derives state and sometimes acts on stale claims.

| File | Role | Cap |
|---|---|---|
| `AGENTS.md` | permanent guardrails, precedence, safety, ownership map | 200 lines |
| `CLAUDE.md` / `.agents/CODEX.md` | bootstrap pointers only | 20 lines |
| `NEXUSAI_CONTINUATION.md` | **current state only** — active work item, owner, files owned, last verified numbers, exact next action. **Rewritten, never appended.** | **300 lines** |
| `docs/architecture/TARGET_ARCHITECTURE.md` | stable target design incl. §G | — |
| `docs/architecture/SEMANTIC_LAYER.md` | the layer's schema, curation process, review rules | — |
| `semantic_layer/*.yaml` | **the curated layer itself — reviewed like code** | — |
| `docs/integration/LOCALAI_UPSTREAM_ADOPTION.md` | pinned upstream version + adoption matrix | — |
| `docs/ux/PRODUCT_UX.md` | routes, screens, components, visual system | — |
| `configuration/product_capability_matrix.json` | **machine-readable capability truth** — the only place a capability state is asserted | — |
| `docs/work/WORK_ITEMS.md` | active + queued work items | — |
| `reports/<id>/` | **immutable** evidence; never where state is asserted | — |

Archive the existing continuation to `reports/archive/continuation-20260921.md` (it contains real findings) and replace it.

**Every session starts:** `AGENTS.md` → agent bootstrap → `NEXUSAI_CONTINUATION.md` → `git status --short` + `git diff --name-only` → identify `ACTIVE_WORK_ITEM` → re-read the files it owns before editing.

**Every work item declares:** `WORK_ITEM_ID / OWNER / FILES_OWNED / DEPENDENCIES / EXPECTED_OUTPUT / TESTS / STOP_CONDITION / APPROVAL_GATES`.
**Every completion updates:** what changed · files changed · tests run · **results as numbers** · known limitations · exact next action.

---

## P. COLLISION PREVENTION

1. No two agents own the same file at the same time. `FILES_OWNED` is explicit paths; overlap is a protocol violation.
2. Before any edit: `git status --short` and `git diff --name-only`. If a target file changed since your session began, **re-read it before patching**.
3. **Natural parallel split for this plan:** **Claude → the compiler** (`api/forensic_records/**` Go query/answer layer). **Codex → the spine and the surface** (`semantic_layer/**` YAML curation, `db/**` migrations, `ingestion/**`, the analyst UI app). These are disjoint and both are on the critical path, so the two tracks genuinely parallelise.
4. **Commits are not the sync mechanism** (commit/push is not authorized). `NEXUSAI_CONTINUATION.md` + `WORK_ITEMS.md` are.
5. Never overwrite another agent's uncommitted work. Never `git reset --hard`, `clean`, `checkout --`, `restore`, `stash`, `merge`, `rebase`, `cherry-pick`.
6. At most two active work items, one per agent, in provably disjoint zones.

---

## Q. WHAT TO STOP DOING

1. **Stop defining operations and templates.** 79 templates (5 certified), 104 operations (52 certified), 33 contradictory. This is the abstraction the team lead has ruled out and the research contradicts. **Freeze the catalog now; delete it at the end of P3.**
2. **Stop treating `query.go` as the router.** 8,644 lines of keyword ladder is the defect generator — every new phrasing exposes a new wrong route. Do not extend it. Route through the compiler.
3. **Stop benchmarking LLMs for operation selection.** There will be no operation IDs to select. Two selectors already failed at 0/5 — at a task the architecture deletes.
4. **Stop using embeddings for structural decisions** (family, group, measure, field role). D2 and the new measure defect were both caused by exactly this, and the published result says an embedding linker at 96.5% recall is statistically indistinguishable from no linker `[RESEARCH]`. Embeddings belong in *semantic text retrieval*, nowhere else.
5. **Stop producing reports as the deliverable.** 40+ report directories, an 874 KB continuation, four directives. The 2026-09-18 golden audit is worth more than all prior reports combined **because it produced a number**.
6. **Stop maintaining two records stores and the proxy facade.** Delete `core/services/records/`, `endpoints/localai/records.go`, `routes/records.go`.
7. **Stop investing in `core/services/agents/forensic_direct.go`** (1,031 lines of product logic inside the platform).
8. **Stop extending multilingual/Urdu** (TL-directed). Keep it working; do not grow it.
9. **Stop "certifying" against catalogs.** Only the golden suite certifies.
10. **Stop tuning narration tokens/threads.** Diagnosed: packet content is the problem, hardware is a ceiling `[TEST]`.

---

## R. WHAT TO PRESERVE

1. **`SourceNativePlanV1` + `source_native_algebra.go` + `source_native_sql_executor.go`** — the IR and the deterministic compiler. **This is the state-of-the-art pattern and it is already built.** Richer than the published SMQ that scored 94.15%. Strict parser with `DisallowUnknownFields` already in place.
2. **`FieldDescriptorV1`** — a genuinely good semantic-element skeleton. Extend it (description, synonyms, display name, metrics, joins) and persist it; do not replace it.
3. **The evidence control plane** — `evidence_items`, versions, custody events, storage objects, source links, RLS policies (23 migrations), content-addressed storage with SHA-256. Expensive to rebuild, hard to get right, already right.
4. **The golden-set harness** — `golden_questions_v0.json`, `scripts/nexusai_live_eval.py`, `adjudicate_baseline.py`, SQL-derived oracles, hand adjudication. **The single most valuable artifact in the repository.** It becomes the certification gate.
5. **The fail-closed auth guard** `validateForensicAuthConfig`.
6. **The 8 ingestion adapters**, derived-artifact lineage, job lifecycle, DLQ, lease handling.
7. **101 Go test files / 19,654 test LOC**, including `p1_correctness_test.go` regressions written by hand from question text.
8. **The analyst presentation modules** — `analystPrimaryAnswer.js`, `analystAskPresentation.js`, `analystDataPresentation.js` and their tests. Port, don't rewrite.
9. **The D2/D3 fixes and their explanatory comments** — good engineering; keep the comment style.

---

## S. PHASED IMPLEMENTATION PLAN

> Tracks: **[C]** Claude (compiler) · **[X]** Codex (spine & surface). P2/P3 run in parallel with P4/P6.

### PHASE 0 — Truth & Boundary Lock  *(2–3 days)*
**PURPOSE** Make the current state safe to change and upstream comparison possible.
**IMPLEMENTATION** (a) **Protect the work** — branch + commit 852 untracked / 94 modified files in reviewed batches, *with user approval, no push*. (b) Clone `mudler/LocalAI` to a **sibling** directory, check out **v4.5.6**, diff trees → `docs/integration/FORK_DELTA_v4.5.6.md`. (c) Collapse the doc set per §O; archive the 874 KB continuation. (d) Rewrite `configuration/product_capability_matrix.json` from §I.
**TESTS** tree diff reproducible; `git status` before/after proves no worktree file lost.
**DONE** fork delta is a reviewed list; one continuation ≤300 lines; capability matrix matches §I.
**STOP** when the delta inventory exists. Do not start refactoring. **RISK** low; (a) needs approval.

### PHASE 1 — Stop the Bleeding  *(the safety gate)* **[C]**
**PURPOSE** Eliminate confident-wrong answers on the path that exists today. This work is **not throwaway** — the constraint-obligation mechanism built here *becomes* S2/S9 of the compiler.
**SOURCE AREAS** `deterministic_semantic_compiler.go`, `semantic_frame.go`, `stim_fact_packet.go`, `answer_presentation.go`, `operation_applicability.go`
**IMPLEMENTATION** WI-1 constraint binding + **refuse-on-unapplied** (D1) · WI-2 **Fact Packet v2** + deterministic answer sentence · WI-3 family guard + measure-hint fix (D4 + new) · then D5, D7, shape fixes.
**TESTS** golden v1 (≥3 phrasings + 1 edge case per exercised operation, SQL oracles, hand adjudication) + per-defect regressions.
**DONE** **0 confident-wrong · 0 HTTP 500 · ≥70% correct-or-clarified · answer stated in text for 100% of correct/partial factual answers.**
**STOP** at the gate. Do not add capabilities. **RISK** low, high value.
> Note the gate is ≥70%, not ≥90%. P1 fixes *safety*; the compiler delivers *coverage*. Do not try to reach 90% by adding templates.

### PHASE 2 — The Semantic Layer  *(the long pole — start it in parallel with P1)* **[X]**
**PURPOSE** Define the data once, so no operation ever has to be defined again.
**IMPLEMENTATION** `semantic_layer.*` tables + `semantic_layer/*.yaml` + loader/validator. Extend `FieldDescriptorV1` with `DisplayName`, `Description`, `Synonyms`, metrics, joins. Bootstrap from `buildSourceNativeFieldCatalog`, then **curate by hand, one family at a time: CDR → subscriber → IPDR → ANPR → access log → tower → financial → generic.** Persist per `(case, family)`; version and review it. Declare the join graph.
**TESTS** loader rejects malformed layers; every golden question's required fields exist with descriptions and synonyms; layer diff is reviewable; a **coverage report** lists fields with no description.
**DONE** all 8 structured families curated; join graph declared for the pairs the golden set exercises.
**STOP** at 8 families. Media families get descriptors in P7. **RISK** low technically; it is **effort**, not risk — budget for it honestly.

### PHASE 3 — The Governed Semantic Compiler  *(the centrepiece)* **[C]**
**DEPENDENCIES** P1 gate, P2 CDR family minimum
**IMPLEMENTATION** S4 deterministic catalog narrowing · S5 request-time JSON Schema with **`enum` field IDs** → LocalAI `response_format: json_schema, strict:true` (or `grammar`) · S6 hard validation · S7 one structured self-correction · S8 reuse the existing compiler · S9 verification (constraint-applied, family, shape, scope, plausibility) · S10 abstention with a specific question · store the IR as an audit record. Adopt `POST /backend/load` pre-warm. **Then delete the template/operation catalog and the `query.go` routing ladder.**
**TESTS** **Golden-IR set**: hand-written gold IR for every golden question; measure IR accuracy separately from answer accuracy so failures are attributable. Adversarial set: ambiguous, unanswerable, out-of-scope, and questions whose fields do not exist — all must **abstain**, never answer. Property test: no generated IR ever references a field outside the working set (should be impossible by construction — assert it anyway).
**DONE** **≥90% correct-or-properly-clarified · 0 confident-wrong · abstention rate ≤20% · p95 < 15 s.**
**STOP** when the gate passes on CPU. Do not chase the last 10% with a bigger model until P7. **RISK** medium — WI-0 de-risks the model-tier question first.

### PHASE 4 — Identity, Cases, Activity **[X]**
**IMPLEMENTATION** `nexus.orgs/users/sessions/cases/case_members/case_activity`; RLS on membership; case lifecycle; evidence→case FK; `/auth`, `/cases`, `/cases/:id/*`, `/activity`; retire the shared analyst service principal.
**TESTS** authorization matrix per role × case; RLS smoke; negative tests for cross-case leakage.
**DONE** two analysts on two cases cannot see each other's evidence, **proven by test**. **RISK** medium (collection→case migration).

### PHASE 5 — Extraction & Upstream Realignment **[C+X]**
**IMPLEMENTATION** Delete `core/services/records/`, `endpoints/localai/records.go`, `routes/records.go`. Move `forensic_direct.go` logic into the NexusAI API. Re-express the ~330 lines of genuine core delta as config or adapters. Move `api/`, `ingestion/`, `db/`, `semantic_layer/` to a top-level layout. Run **`localai/localai:v4.10.x` unmodified**, internal network only.
**TESTS** contract tests against the pinned tag; full golden suite still passes; **zero diff** for LocalAI-owned paths.
**DONE** upgrading LocalAI is a tag bump. **RISK** medium — after P3 so correctness is not moving while packaging moves.

### PHASE 6 — Analyst Product UI **[X]**
**IMPLEMENTATION** New app; routes and answer contract per §K; **the clarification surface as a designed, first-class experience**; port `analyst*Presentation.js`; layered light/dark; every viewer type; every failure state; no model/backend/agent controls anywhere.
**TESTS** Playwright E2E for every flow incl. clarify, zero-result, unsupported, partial; accessibility; responsive.
**DONE** a non-technical analyst completes login → create case → add data → ask → answer → citation → open source → follow-up **unaided, observed**. **RISK** medium.

### PHASE 7 — Depth, Adoption & Scale
**IMPLEMENTATION** Media-family descriptors + cross-family joins in the layer. Document chunking (a PDF is currently **one passage per document**) + page/span citations + scanned-PDF OCR. **OCR confidence calibration — suppress noise rather than store it at 0.90.** ASR → `audio-cpp` + diarization. Rerank. ANPR/video derived-event querying. Face candidate-similarity semantics. Then upstream wave 2: evaluate **KNN routing** for S1/S4 (adopt only if it beats the deterministic incumbent on the golden set), context compression, PII pseudonyms, `local-ai benchmark`. Then GPU: Phase A (CPU laptop) → B (single GPU) → C (distributed workers), with product APIs and domain contracts unchanged across all three.
**DONE** every family at BASELINE or an explicit, displayed capability gap; the same golden suite passes on every provisioned tier. **RISK** medium.

---

## T. FIRST WORK ITEMS

### WI-0 (DO THIS FIRST — 2 days) — IR-generation feasibility spike **[C]**
**PURPOSE** Settle the one genuine unknown in §G before the architecture commits to a model tier.
**FILES_OWNED** `reports/ir-spike-<date>/**` and a throwaway harness under `scripts/` — **no production source changes**
**METHOD** Take 40 governed-analysis questions from `golden_questions_v0.json`. Hand-write the gold `SourceNativePlanV1` for each. Curate a minimal CDR + subscriber semantic layer (~30 fields with descriptions and synonyms) as YAML. Build the request-time JSON Schema with `field_id` slots as **`enum`**. Call live LocalAI with `response_format: {type:"json_schema", json_schema:{strict:true, …}}` on `qwen3-4b-instruct-2507-q4km-nxb21d-dev`. Measure, with and without one self-correction round: **exact-IR match · execution-equivalent match · abstention rate · malformed rate (expect 0) · p50/p95 latency.**
**PRE-DECLARED DECISION RULE — write it down before running:**
- **≥80% execution-equivalent** → proceed with the 4B model. Build P3 as specified.
- **60–79%** → proceed, but widen S4 narrowing and add the second tier (Qwen3-8B Q4) as a fallback for low-confidence plans.
- **<60%** → stop and escalate to the user with the numbers. Do **not** silently start trying models.
**DONE** one report with those five numbers and the rule's verdict. **STOP** immediately after. This is one measurement against a pre-declared threshold — it is explicitly **not** model roulette.

### WI-1 — Bind extracted constraints; refuse rather than drop *(closes D1)* **[C]**
**FILES_OWNED** `deterministic_semantic_compiler.go`, `p1_correctness_test.go` · **READ_ONLY** `semantic_frame.go`, `source_native_sql_executor.go`, `source_native_algebra.go`
Compile `frame.Filters` and the extracted target/date/family into `plan.Filters` / request constraints, resolving each `FieldHint` against the authorized catalog by **exact name and alias match only — never whole-question embedding.** Add a **CONSTRAINT_APPLIED guard**: if any extracted literal cannot bind, return `NEEDS_INPUT` naming the specific unbound literal. Make silent dropping structurally impossible.
> This is S2/S9 of the compiler, built early against the current path. It carries forward unchanged.

**TESTS** (write first, by hand, from question text)
- `"How many calls did 923001110001 make?"` → EQ filter on the MSISDN field, **not** a case-wide count
- `+ "in August 2026"` → also carries the date range
- `"How many calls did 03999999999 make?"` (nonexistent) → **0**, not 12,912
- literal with no matching authorized field → `NEEDS_INPUT`, never a count
- all 9 existing `p1_correctness_test.go` tests still pass

**DONE** unit tests green; NEG-01, CDR-11, CDR-14 correct live. **STOP** there — do not refactor `query.go`. **APPROVAL** rebuild + redeploy.

### WI-2 — Fact Packet v2: facts are answers, not plumbing **[C]**
**FILES_OWNED** `stim_fact_packet.go`, `answer_presentation.go` · **READ_ONLY** `query.go` (metric construction sites), `narrative_grounding.go`
Result values, totals and cited passages become first-class facts. `Template:`, `Route:`, `Planner Confidence`, `Display Rows` move to a `trace` block that never reaches narration. Build the deterministic headline **from the facts** — *"There are 8,642 CDR records in this case."* — and emit it **before** any narration attempt. Re-point `validateNarrative` at the value facts.
**TESTS** packets for one count, one breakdown, one document question assert: ≥1 fact whose value **is** the answer; zero plumbing facts; deterministic headline contains the answer; narration rejection still leaves a complete answer.
**DONE** answer stated in analyst text for 100% of correct/partial factual answers (baseline 5/20). **STOP** — Role A latency is explicitly out of scope.

### WI-3 — Family guard + measure-hint fix **[C]**
**FILES_OWNED** `operation_applicability.go`, `deterministic_semantic_compiler.go` (measure block only)
An operation whose family is absent from the question's families is filtered **before** ranking; if nothing survives → `NEEDS_INPUT`, never a cross-family guess. Separately, at `:461-463`, stop falling back to `hint = question` for measure resolution: with no explicit measure hint, use the declared default measure or return `MEASURE_FIELD_UNRESOLVED`.
**TESTS** IPDR-02, IPDR-05, TWR-01, ACC-02/03/04, DOC-05 no longer route to CDR/ingest templates; `"largest transaction"` → MAX(amount) not COUNT; `"account with most transactions"` → COUNT not amount.
**DONE** 0 cross-family misroutes; measure questions correct.

### WI-4 — Semantic layer: CDR family **[X]** *(parallel with WI-1..3)*
**FILES_OWNED** `semantic_layer/**`, `db/forensic_records/0xx_semantic_layer.sql`, the loader/validator
Schema + loader + validator + the curated CDR entity: every field with `display_name`, `description`, `synonyms`, type, sensitivity, allowed filters/aggregates; metrics; the `cdr → subscriber` join.
**TESTS** loader rejects malformed layers; a coverage report lists fields with no description; every field referenced by a CDR golden question resolves.
**DONE** CDR curated and loading. **STOP** — one family. **APPROVAL** migration must be reviewed before it runs.

**Then:** re-run the full golden suite, write a new scorecard under `reports/<date>/`, update `NEXUSAI_CONTINUATION.md` and the capability matrix, and report before starting P3.

---

## PRODUCT NAME `[RECOMMENDATION]`

"NexusAI" is heavily contested (multiple companies, funds and tokens), the `*AI` suffix dates quickly, and it says nothing about evidence or investigation.

**Recommended: `Evidentia`.** Evidence-centric, enterprise-credible, pronounceable in English- and Urdu-speaking markets, not tied to a technology that will change, and it namespaces cleanly: `evidentia-api`, `EVIDENTIA_*`, `evidentia.app`, *Evidentia Investigation Workspace*. Alternates if clearance fails: **Kestrel** or **Casefile**.

**Caveats:** a trademark and domain search is required — I have not performed one. The rename is mechanical but wide (Go module path, image names, ~40 env vars, DB role `localrecall`, docs, UI strings). Do it **once, as a single dedicated commit**, in Phase 0 or Phase 5 — never incrementally. It is **not** urgent relative to 26 confident-wrong answers.

---

## FINAL ANSWER

> *"If this were your engineering team and you had to finish this accurately, maintainably and productively from its current state, what exact architecture and implementation sequence would you commit the team to now?"*

**Architecture.** Stop writing operations. Write the **data model once** — a curated, versioned semantic layer of entities, dimensions, measures, metrics, synonyms and declared joins — and build **one Governed Semantic Compiler** that turns any question into a validated typed plan and then into parameterized SQL. The LLM's only structural job is to fill that plan under a JSON Schema whose field slots are an **`enum` of the authorized field IDs**, so the decoder physically cannot name a field that does not exist. Everything before it (classification, literal extraction, scope, catalog narrowing) is deterministic. Everything after it (validation, compilation, verification, abstention) is deterministic. The model never writes SQL, never names a table, never picks an operation — because after P3 there are no operations to pick.

This is not a bet. It is the published result: **94.15% versus 2.2%** on real enterprise NL2SQL for exactly this pattern, and NexusAI already owns the expensive half of it — `SourceNativePlanV1` is a richer IR than the one in that paper, the deterministic SQL compiler is written and working, and LocalAI has shipped strict JSON-Schema-to-GBNF constrained decoding **in the version already running**. What is missing is the cheap half: descriptions, synonyms and a join graph. There are zero description fields in the catalog today.

LocalAI becomes a pinned, unmodified v4.10.x container on an internal network — constrained generation, embeddings, rerank, speech, vision primitives, lifecycle, hardware. NexusAI owns identity, cases, evidence, lineage, the semantic layer, the compiler, Fact Packets, citations, activity and a separate analyst application.

**Sequence.** Protect the work — 852 untracked files in a one-commit repository is the biggest operational risk here and it has nothing to do with architecture. Reconstruct the v4.5.6 baseline in a sibling clone. Collapse four directives and an 874 KB log into one 300-line state file. Then **two tracks in parallel**: Claude stops the bleeding (bind the filters, fix the Fact Packet, add the family guard) to a gate of **zero confident-wrong**; Codex curates the CDR semantic layer, because that curation is the long pole and it starts now, not later. Before committing P3 to a model tier, run the two-day WI-0 spike with the decision rule written down first. Then build the compiler, delete the template catalog, and go: cases and identity, extraction to upstream, the analyst UI, family depth, GPU.

**The judgement underneath it.** The team lead's instinct is exactly right and the evidence backs him: you cannot enumerate the questions an investigator will ask, and every attempt to do so has produced another confidently wrong answer. But the fix is not to give an LLM more freedom — a 7B model writing raw SQL scores 39%. The fix is to give it **less** freedom over a **much better described** world: a small, typed, enumerated plan over data that has been properly named. And then to accept the one thing that makes forensic accuracy actually achievable — **that the system is allowed to ask instead of guess.** You do not need a planner that is right every time. You need one that is never confidently wrong. An analyst can work with *"By 'total' do you mean call duration, data volume, or transaction amount?"* No analyst can work with a confident 12,912.

---

### Sources for published results cited as `[RESEARCH]`
- [A Semantic-Layer-Mediated Agent for Natural Language to SQL over Heterogeneous Enterprise Databases (arXiv 2606.31041)](https://arxiv.org/abs/2606.31041) — SMQ IR, deterministic compiler, 94.15% on Spider2-snow
- [How Far Do On-Prem Open LLMs Get on Text-to-SQL? A Cross-Family Size×Technique Frontier on BIRD (arXiv 2606.29733)](https://arxiv.org/html/2606.29733) — on-prem model accuracy, self-correction, schema-linking ineffectiveness
- [Semantic Layer vs. Text-to-SQL: 2026 Benchmark Update — dbt Developer Blog](https://docs.getdbt.com/blog/semantic-layer-vs-text-to-sql-2026)
- [Why Semantic Layers Make Enterprise Text-to-SQL Safer](https://datalakehousehub.com/blog/2026-05-semantic-layers-text-to-sql/)
- [BIRD-SQL Benchmark](https://bird-bench.github.io/)
- [Text-to-SQL Benchmarks are Broken: An In-Depth Analysis of Annotation Errors (CIDR 2026)](https://www.vldb.org/cidrdb/papers/2026/p5-jin.pdf) — caution on benchmark numbers
- [Reliable End-to-End Text-to-SQL Generation (EDBT 2026)](https://openproceedings.org/2026/conf/edbt/paper-177.pdf) — *fetch returned unreadable binary; cited as a pointer only, not relied on*
- [DRL: A Deterministic Relational Middleware Layer for Transaction-Safe Enterprise NL2SQL (arXiv 2608.26172)](https://arxiv.org/html/2608.26172)
