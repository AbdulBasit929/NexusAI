# MASTER_EXECUTION_PROMPT — v2
*(Paste everything between the `====` markers into a fresh Claude Code session opened at `C:\Users\sheik\Workspace\Office\Projects\NexusAI`. Self-contained. The `CODEX_SYNC_BOOTSTRAP` at the bottom is a separate, shorter paste for Codex.)*

======================================================================
BEGIN MASTER_EXECUTION_PROMPT
======================================================================

You are the lead engineer on a forensic intelligence product built on a fork of
LocalAI, at `C:\Users\sheik\Workspace\Office\Projects\NexusAI`.

This is an IMPLEMENTATION session. A full repository + upstream + research
reconciliation was completed on 2026-09-21; its conclusions are restated below
and are binding. Do not redo it. Do not produce another architecture report.
Your output is working, tested code.

----------------------------------------------------------------------
1. THE PRODUCT-OWNER CONSTRAINT THAT DEFINES THIS WORK
----------------------------------------------------------------------

The team lead has ruled: **we cannot keep defining operations.** Any runtime
question an analyst asks must be converted, accurately, into the correct SQL or
the correct retrieval against whatever data the case actually holds, executed,
and answered.

The repository currently contains 79 query templates (5 certified) and 104
operations (52 certified, 33 in contradictory states), plus an 8,644-line
keyword routing ladder in `query.go`. That is a per-question abstraction. It
can never cover a question nobody wrote in advance, and every miss produces a
confident wrong answer.

**The replacement is the GOVERNED SEMANTIC COMPILER (§6).** We define the DATA
once — a curated semantic layer — and compile every question against it.
20 families x ~25 fields = ~500 descriptors generate an unbounded question
space. 79 templates generate 79 question shapes.

----------------------------------------------------------------------
2. AUTHORITY AND STARTUP
----------------------------------------------------------------------

Read, in this order, before editing anything:
  1. AGENTS.md
  2. NEXUSAI_CONTINUATION.md — **only the newest entry at the top.** The file
     is 874 KB / 12,836 lines of append-only history. Do not read it all.
  3. reports/nexusai-tl-audit-20260918/P0-baseline-report.md — the only real
     measurement of product accuracy that exists.
  4. .agents/api-endpoints-and-auth.md and .agents/coding-style.md (LocalAI
     conventions apply to any LocalAI-tree file you touch).

PRECEDENCE, highest first:
  this prompt > source code and live behavior > P0-baseline-report.md >
  AGENTS.md > NEXUSAI_CONTINUATION.md > reports/ >
  NEXUSAI_MASTER_DIRECTIVE.md > NEXUSAI_NEXT_CHAT_PROMPT.md > the STIM matrix.

TREAT AS STALE UNLESS RE-VERIFIED IN SOURCE:
  - any "0 wrong confident executions" claim — disproved, there are 26
  - any claim the synthesis model is Q8 — it is Q4_K_M
  - any "CERTIFIED" status in configuration/ or the operations catalog
  - configuration/nexusai_stim_maturity_matrix.json (2026-09-14; predates both
    the auth incident and the accuracy baseline)
  - defects D2 and D3 from the P0 report — BOTH ARE ALREADY FIXED IN SOURCE

Source outranks every report. Verify before you assume.

----------------------------------------------------------------------
3. PRESERVE THE WORKING TREE — READ THIS TWICE
----------------------------------------------------------------------

The repository has exactly ONE commit (`40717b8`, 2026-07-22) and the working
tree holds **852 untracked files and 94 modified files** — months of work that
exists nowhere else.

NEVER run, without the user explicitly asking in this session: `git reset
--hard`, `git clean`, `git checkout --`, `git restore`, `git stash`,
`git merge`, `git rebase`, `git cherry-pick`, `git pull`, `git push`, or any
force operation. Never delete an untracked file you did not create. Never
resolve a conflict by discarding someone's changes. Do not `git add`/`commit`
or open a PR unless asked in this session.

Docker: never `docker compose down`, `down -v`, `--remove-orphans`,
`system prune`, `volume prune`. Never reprocess, backfill, migrate or delete
retained evidence. SQL-verified anchors in the demo case: CDR 8,642 · IPDR
2,500 · ANPR 750 · access log 1,000 · subscribers 11 · towers 5 · transactions 4.

Operational hazard: `forensic-records-api` requires `FORENSIC_RECORDS_API_KEY`
and `FORENSIC_API_AUTH_REQUIRED=true` exported (or sourced from
`.env.forensic-runtime.local`) before every `docker compose up`. Auth fails
closed — the container refuses to start rather than serving unauthenticated.
**Never "fix" a crash-loop by disabling auth.** That already happened once,
live, for an entire session, on real forensic evidence.

----------------------------------------------------------------------
4. WHERE THE PROJECT ACTUALLY IS
----------------------------------------------------------------------

Measured 2026-09-18, 62 English questions, SQL-derived oracles, every response
hand-adjudicated:

    CORRECT 16 (26%) · PARTIAL 9 · SAFE_FAIL 5 · WRONG-CONFIDENT 26 (42%)
    · FALSE-NEGATIVE 3 · HTTP 500 shown to analyst 3
    Correct-or-properly-clarified 21/62 = 34%
    Answer actually stated in the analyst's text: 5 of 20
    Latency p50 0.56 s · p95 4.5 s · max 82 s

26 confident-wrong answers in a forensic product is a SAFETY defect: the
analyst cannot tell a wrong answer from a right one.

Runtime (already correct — do not redesign):
    React SPA (served by LocalAI :8080)
      └─ LocalAI :8080 — /api/v1/forensics/* facade (NexusAI-authored)
           └─ forensic-records-api :8091 (Go, 38,956 LOC non-test)
                ├─ forensic-postgres :5433 (TimescaleDB pg16, 23 migrations, RLS)
                ├─ forensic-nats :4222 (JetStream)
                └─ LocalAI :8080 (embeddings, synthesis, face, ASR)
      └─ forensic-records-worker (Python, ~4,300 LOC, 8 adapters)

Live models: synthesis `qwen3-4b-instruct-2507-q4km-nxb21d-dev` (Q4_K_M),
embeddings `qwen3-embedding-0.6b`. CPU only, ~4 tok/s, ~15.7 GiB RAM, 6 GiB floor.

----------------------------------------------------------------------
5. WHAT ALREADY EXISTS AND MUST BE BUILT ON, NOT REPLACED
----------------------------------------------------------------------

**`SourceNativePlanV1`** in `api/forensic_records/source_native_algebra.go:107`
is already a typed analytical IR:
    { Project, Filters, GroupFields, Measures, TimeBucket, Having, Sort, Limit }
It has a strict parser (`UnmarshalJSON` with `DisallowUnknownFields` plus a
trailing-content check). `source_native_sql_executor.go` already compiles it to
**parameterized** SQL with the scope predicate injected and full row lineage.

This is the state-of-the-art pattern and it is already written. The published
semantic-layer-mediated approach that scored **94.15% execution accuracy on
Spider2-snow** (547 real enterprise NL2SQL instances, versus 2.2% for schema-only
DAIL-SQL+GPT-4o and 23–26% for Spider-Agent) uses an IR called SMQ with only
`{metrics, filters, group_by}` — **strictly less** than what this repo has.

**`FieldDescriptorV1`** (`source_native_algebra.go:48`) is a good semantic-element
skeleton: FieldID, SourceName, SourceNames, NormalizedName, EffectiveType,
AllowedFilters, AllowedAggregates, Projectable, Groupable, Sortable,
Sensitivity, RedactionState, FamilyProvenance, SourceProvenance.

**What is actually missing — this is the real gap:**
  a. NO description field. `grep -c Description source_native_algebra.go` = 0.
     Nothing tells a model that `inv_tot` means "invoice total".
  b. NO synonyms, NO display names, NO metrics, NO join graph.
  c. The catalog is INFERRED PER REQUEST from sampled rows
     (`buildSourceNativeFieldCatalog(req, rows)`), so it is unstable and
     unreviewable. It must be curated and persisted.
  d. Nothing ever asks a model to FILL the IR. The LLM was asked to classify
     among ~104 opaque operation IDs, which is why it scored 0/5.

**THE KEYSTONE — verify this yourself, it is already in the repo at v4.5.6:**
  - `core/schema/openai.go:167-176` — `JsonSchemaRequest` / `JsonSchema{Name,
    Strict bool, Schema}` → OpenAI-style STRICT structured outputs
  - `core/schema/openai.go:237` — `Grammar string` (GBNF)
  - `pkg/functions/function_structure.go:20-37` — JSON-Schema → GBNF converter
  - `pkg/functions/grammars/json_schema.go:129-138` — **`enum` compiles to a
    GBNF alternation rule**
  - Backend: llama.cpp — the one already in use

Put the authorized field IDs in the schema as an `enum` and the decoder
**physically cannot emit a field that does not exist.** Hallucinated columns
become impossible, not merely unlikely. The `MALFORMED_OUTPUTS=2` failure
recorded for Phi-4-mini was avoidable with a capability already in this repo.

----------------------------------------------------------------------
6. THE TARGET: GOVERNED SEMANTIC COMPILER — BINDING
----------------------------------------------------------------------

```
English question + case scope + authenticated identity
 S1  REQUEST CLASSIFICATION       deterministic
     {governed_analysis | evidence_retrieval | product_help |
      contextual_followup | out_of_scope}
 S2  LITERAL & CONSTRAINT EXTRACTION   deterministic, precision over recall
     MSISDN · IMEI · IMSI · plate · CNIC · IP · account · money · absolute and
     relative dates · quoted phrases · file names · durations.
     EVERY literal becomes a REQUIRED OBLIGATION checked at S9.
 S3  SCOPE RESOLUTION             case → authorized evidence, families present
     in the CASE and families named in the QUESTION, RLS predicate
 S4  CATALOG NARROWING            deterministic. THIS IS WHAT MAKES A SMALL
     MODEL WORK. Family + question terms + literal types cut ~500 authorized
     fields to a working set of ~15–30. Cheap, explainable, auditable.
     NOT an embedding ranker.
 S5  IR GENERATION                ◄── the LLM's ONLY structural job
     Build a JSON Schema AT REQUEST TIME from the working set, where every
     field_id slot is an `enum` of exactly the authorized IDs. Send it as
     response_format {type:"json_schema", json_schema:{strict:true,…}}.
     The model fills project · filters · group_by · measures · time_bucket ·
     having · sort · limit. It NEVER emits SQL, NEVER names a table, NEVER
     picks an operation ID. There are no operation IDs.
 S6  HARD VALIDATION              deterministic, total: field ∈ working set ·
     aggregate ∈ AllowedAggregates · filter op ∈ AllowedFilters · type
     compatibility · Groupable/Projectable/Sortable respected ·
     Sensitivity/RedactionState honoured · join declared · limits bounded
 S7  SELF-CORRECTION              ONE bounded retry. Return validation errors
     as a typed diff ("field_id f_x is not groupable; groupable fields are …").
     Research: self-correction is the single most effective technique for
     on-prem open models (+3.65 pp Llama-3.1-8B, +3.06 pp CodeLlama-7B, both
     p<10^-10) and is MOST valuable at small sizes. Exactly one retry, never a loop.
 S8  COMPILATION                  deterministic, parameterized, scope predicate
     always injected, joins from the declared join graph. Never string
     interpolation. THIS ENGINE ALREADY EXISTS.
 S9  VERIFICATION                 ◄── where "accurate" is actually won
     CONSTRAINT-APPLIED : every S2 obligation appears in the plan. One unbound
                          → ABSTAIN. Never silently drop.
     FAMILY             : no field from a family absent from the question+case
     SHAPE              : count ⇒ scalar · rank ⇒ ordered+limited ·
                          list ⇒ rows · comparison ⇒ ≥2 groups
     SCOPE              : rows ≤ scope total; a "filtered" answer equal to the
                          unfiltered total is a RED FLAG, not a result
     PLAUSIBILITY       : non-negative counts, dates within evidence range, no
                          empty-filter aggregate presented as filtered
 S10 ABSTAIN OR ANSWER            Fail any S9 check, or S7 exhausted → ONE
     SPECIFIC CLARIFYING QUESTION naming the ambiguity. THIS IS A SUCCESS.
     "By 'total' do you mean call duration, data volume, or amount?"
 S11 FACT PACKET v2               facts are RESULT VALUES and CITED PASSAGES.
     "CDR record count = 8,642" is a fact. "Template: …", "Route: …",
     "Planner Confidence" are NOT — they move to a `trace` block.
 S12 DETERMINISTIC ANSWER         built from facts, ALWAYS produced, < 5 s.
     "There are 8,642 CDR records in this case."
 S13 NARRATION (OPTIONAL)         streamed after S12, grounded-validated.
     Timeout or rejection → the analyst keeps S12. Never blocks.
 S14 PRESENTATION                 answer · result · citations · derivation
     (collapsed) · limitations · follow-ups
```

NON-NEGOTIABLES
 1. The LLM never emits SQL, never names a table, never picks an operation.
 2. Every field the model can reference is enumerated in the schema at request time.
 3. Every extracted literal is an obligation; failing to bind one abstains.
 4. Every query is parameterized and carries the scope predicate.
 5. Abstention is a SUCCESS. A confident wrong answer never is.
 6. The deterministic answer is always produced; narration is always optional.

WHY THIS AND NOT SOMETHING ELSE — the evidence, so you do not relitigate it:
 · Semantic layer + IR + deterministic compiler: 94.15% vs 2.2% / 23–26% on
   Spider2-snow enterprise NL2SQL. Industry: 40% → 86–95% with a semantic layer.
 · A small local model must NEVER write SQL: on-prem BIRD execution accuracy is
   Qwen2.5-Coder 7B 39.1% · 14B 47.4% · 32B 50.4% · Llama-3.1-8B 32.9% · 70B 49.2%.
 · DO NOT over-invest in embedding schema linking: an embedding linker at 96.5%
   gold-table recall was statistically indistinguishable from NO linking
   (p≥0.55); lexical linking HURT. Corroborated locally — defect D2 and the
   newly found measure defect were BOTH caused by embedding signals on
   structural roles.
 · Self-consistency is not worth it: +0.13 pp for ~5x token cost (p=0.86).
   Unaffordable at 4 tok/s.

THE PRODUCT INSIGHT: you do not need perfect plan accuracy. You need ≈0%
confident-wrong. Verification plus abstention converts the residual error budget
from WRONG ANSWERS into CLARIFYING QUESTIONS. An analyst can work with "which
field do you mean?" An analyst cannot work with a confident 12,912.

----------------------------------------------------------------------
7. HARD RULES THAT PREVENT THE PREVIOUS FAILURE MODES
----------------------------------------------------------------------

ANTI-TEMPLATE RULE (absolute): do not add operation templates. The catalog is
FROZEN and will be DELETED at the end of Phase 3. Do not extend `query.go`'s
keyword ladder. Route through the compiler.

ANTI-MODEL-ROULETTE RULE (absolute): do not evaluate, benchmark, download or
swap LLMs except by running WI-0 below, which has a written-down decision rule.
Two operation selectors already failed at 0/5 — at a task this architecture
deletes. Model escalation requires a measurement against a pre-declared
threshold, surfaced to the user. A model benchmark is never a product milestone.

ANTI-EMBEDDING RULE: embeddings are for semantic TEXT retrieval only. Never for
family, group, measure, or any other structural role. Both D2 and the new
measure defect came from exactly this.

ANTI-REPORT RULE: write a report only when it carries a measurement. Do not add
to `reports/` otherwise.

----------------------------------------------------------------------
8. DEFECT STATUS — VERIFIED IN SOURCE 2026-09-21
----------------------------------------------------------------------

FIXED, do not redo:
  D3 projection-SQL argument bug — `source_native_sql_executor.go:64-70`
  D2 spurious GROUP BY from whole-question embedding —
     `deterministic_semantic_compiler.go:436-443` (empty-hint guard landed)

OPEN:
  D1 **target/date filters silently dropped** —
     `deterministic_semantic_compiler.go` initialises `plan.Filters` at line 418
     and then NEVER references `.Filters` again. `semantic_frame.go:382`
     populates `frame.Filters` and NOTHING consumes it. This is why a
     nonexistent phone number returns a count of all 12,912 case rows.
     HIGHEST PRIORITY.
  D4 cross-family misrouting (IPDR/tower/access-log → CDR) — no family guard
  D5 irrelevant retrieval returned as a match (audio) — identifier tokens not
     required to match
  D7 "Explain …" classified as a dictionary definition
  NEW (found 2026-09-21) **measure field resolved from the whole question** —
     `deterministic_semantic_compiler.go:461-463` sets `hint = question` when
     `frame.MeasureFieldHint` is empty and runs the same embedding-weighted
     resolver D2 was fixed for. Explains "largest transaction → COUNT" and
     "account with most transactions → ranked by amount".

ALSO OPEN, affects EVERY question:
  Fact Packets carry plumbing, not answers — `stim_fact_packet.go:205` builds
  facts from metrics, and the metrics are "Template:", "Route:", "Planner
  Confidence", "Display Rows". Neither deterministic text nor the LLM can state
  an answer from that. `validateNarrative` correctly rejects the narration; the
  PACKET is the problem, not the model and not the hardware.

----------------------------------------------------------------------
9. YOUR WORK ITEMS — EXACT SEQUENCE. START AT WI-0.
----------------------------------------------------------------------

### WI-0 (FIRST, ~2 days) — IR-generation feasibility spike

The one genuine unknown: no published number exists for "a 4B model filling an
enum-constrained analytical IR over a ~25-field curated catalog." Settle it
before the architecture commits to a model tier.

FILES_OWNED   reports/ir-spike-<date>/**, a throwaway harness under scripts/
              **NO production source changes**

METHOD  Take 40 governed-analysis questions from
        `reports/nexusai-tl-audit-20260918/golden_questions_v0.json`.
        Hand-write the gold `SourceNativePlanV1` for each.
        Curate a minimal CDR + subscriber semantic layer (~30 fields WITH
        descriptions and synonyms) as YAML.
        Build the request-time JSON Schema with field_id slots as `enum`.
        Call live LocalAI with response_format json_schema strict:true on
        `qwen3-4b-instruct-2507-q4km-nxb21d-dev`.
        Measure, with and without one self-correction round:
          exact-IR match · execution-equivalent match · abstention rate ·
          malformed rate (expect 0) · p50/p95 latency.

DECISION RULE — write it into the report BEFORE you run anything:
  ≥80% execution-equivalent → proceed with the 4B model; build Phase 3 as spec'd
  60–79%                    → proceed, widen S4 narrowing, add Qwen3-8B Q4 as a
                              fallback tier for low-confidence plans
  <60%                      → STOP and escalate to the user with the numbers.
                              Do NOT silently start trying models.

DONE   one report with those five numbers and the rule's verdict.
STOP   immediately after. This is ONE measurement against a pre-declared
       threshold — it is explicitly NOT model roulette.
APPROVAL  ask before running live inference across 40 questions (it costs real
          time on CPU).

### WI-1 — Bind extracted constraints; refuse rather than drop (closes D1)

FILES_OWNED  api/forensic_records/deterministic_semantic_compiler.go
             api/forensic_records/p1_correctness_test.go
READ_ONLY    semantic_frame.go, source_native_sql_executor.go,
             source_native_algebra.go

Compile `frame.Filters` and the extracted target/date/family into
`plan.Filters` / request constraints, resolving each `FieldHint` against the
authorized catalog by **exact name and alias match only — never whole-question
embedding.** Add a CONSTRAINT_APPLIED guard: if any extracted literal cannot
bind, return `NEEDS_INPUT` naming the specific unbound literal. Make silent
dropping structurally impossible.

This is S2/S9 of the compiler, built early against the path that exists today.
It carries forward into Phase 3 unchanged — it is not throwaway work.

TESTS — write them FIRST, by hand, from the question text, never from helper output:
  "How many calls did 923001110001 make?"   → EQ filter on the MSISDN field,
                                               NOT a case-wide count
  + "in August 2026"                        → also carries the date range
  "How many calls did 03999999999 make?"    → 0, not 12,912
  literal with no matching authorized field → NEEDS_INPUT, never a count
  all 9 existing p1_correctness_test.go tests still pass

DONE      unit tests green; NEG-01, CDR-11, CDR-14 correct on a live run
STOP      there. Do NOT refactor query.go.
APPROVAL  ask before container rebuild + redeploy

### WI-2 — Fact Packet v2: facts are answers, not plumbing

FILES_OWNED  api/forensic_records/stim_fact_packet.go
             api/forensic_records/answer_presentation.go
READ_ONLY    query.go (metric construction sites), narrative_grounding.go

Result values, totals and cited passages become first-class facts. Plumbing
moves to a separate `trace` block that never reaches narration. Build the
deterministic headline FROM the facts — "There are 8,642 CDR records in this
case." — and emit it BEFORE any narration attempt. Re-point `validateNarrative`
at the value facts. Kill the process jargon: "Executed bounded source-native
typed algebra over 8642 authorized source rows…", column header "M1", and
"Records Row Count: 1" for a count of 8,642 are all defects.

TESTS  packets for one count, one breakdown, one document question assert:
       ≥1 fact whose value IS the answer; zero plumbing facts; deterministic
       headline contains the answer; narration rejection still leaves a
       complete usable answer.
DONE   answer stated in analyst text for 100% of correct/partial factual
       answers (baseline 5 of 20)
STOP   when the headline is correct without the LLM. Role A narration latency
       is explicitly OUT OF SCOPE — it is diagnosed, and it is a hardware
       ceiling, not a bug.

### WI-3 — Family guard + measure-hint fix (closes D4 and the new defect)

FILES_OWNED  api/forensic_records/operation_applicability.go
             api/forensic_records/deterministic_semantic_compiler.go
                                                        (measure block only)
READ_ONLY    semantic_frame.go, query.go

An operation whose family is absent from the question's families is filtered
BEFORE ranking; if nothing survives → NEEDS_INPUT, never a cross-family guess.
Separately, at lines 461-463, stop falling back to `hint = question` for measure
resolution: with no explicit measure hint, use the declared default measure or
return MEASURE_FIELD_UNRESOLVED.

TESTS  IPDR-02, IPDR-05, TWR-01, ACC-02/03/04, DOC-05 no longer route to
       CDR/ingest templates; "largest transaction" → MAX(amount) not COUNT;
       "account with most transactions" → COUNT not amount
DONE   0 cross-family misroutes; measure questions correct

### AFTER WI-3
Re-run the golden suite (`scripts/nexusai_live_eval.py` +
`reports/nexusai-tl-audit-20260918/adjudicate_baseline.py`), write the new
scorecard to a fresh `reports/<date>/`, update NEXUSAI_CONTINUATION.md and the
capability matrix — then STOP and report before starting Phase 3.

PHASE 1 GATE: **0 confident-wrong · 0 HTTP 500 · ≥70% correct-or-clarified ·
answer stated in text for 100% of correct/partial factual answers.**
The gate is ≥70%, NOT ≥90%. Phase 1 fixes SAFETY; the compiler delivers
COVERAGE. Do not try to reach 90% by adding templates.

----------------------------------------------------------------------
10. PHASES AFTER THE GATE (do not start early)
----------------------------------------------------------------------

P2 SEMANTIC LAYER [Codex track, runs in parallel with Phase 1]
   `semantic_layer.*` tables + `semantic_layer/*.yaml` + loader/validator.
   Extend FieldDescriptorV1 with DisplayName, Description, Synonyms, metrics,
   joins. Bootstrap from `buildSourceNativeFieldCatalog`, then CURATE BY HAND,
   one family at a time: CDR → subscriber → IPDR → ANPR → access log → tower →
   financial → generic. Persist per (case, family). Declare the join graph.
   This is DATA, not code — YAML, reviewable, diffable, testable. It is the
   long pole; it starts now, not later.

P3 GOVERNED SEMANTIC COMPILER [Claude track] — S4..S10 per §6, plus
   `POST /backend/load` pre-warm. THEN delete the template/operation catalog
   and the query.go routing ladder.
   Gate: ≥90% correct-or-properly-clarified · 0 confident-wrong ·
         abstention ≤20% · p95 < 15 s.
   Tests: a GOLDEN-IR SET (hand-written gold IR per question) so IR accuracy is
   measured separately from answer accuracy and failures are attributable; an
   adversarial set (ambiguous, unanswerable, out-of-scope, nonexistent fields)
   where every item MUST abstain; a property test asserting no generated IR
   ever references a field outside the working set.

P4 IDENTITY, CASES, ACTIVITY [Codex]. There is NO case entity today: `case_id`
   occurs 3x in the schema (one nullable column) vs `collection_id` 224x; no
   users/roles/membership tables; no case-creation path anywhere in the UI;
   every analyst authenticates as the same shared service principal. Build
   `nexus.orgs/users/sessions/cases/case_members/case_activity` with RLS on
   membership. NexusAI owns analyst identity; LocalAI identity becomes
   operator-only. Never two analyst identity systems.

P5 EXTRACTION & UPSTREAM REALIGNMENT. Fork baseline is v4.5.6; current upstream
   is v4.10.0 (2026-09-17). A rebase is impossible — one squashed commit
   containing v4.5.6 plus already-merged NexusAI work, no upstream remote, no
   merge base. Strategy is EXTRACT NEXUSAI, CONSUME UPSTREAM UNMODIFIED.
   Delete `core/services/records/` (2,188 LOC dead file-backed store),
   `core/http/endpoints/localai/records.go`, `core/http/routes/records.go` —
   all three confirmed ABSENT from upstream v4.10.0. Move
   `core/services/agents/forensic_direct.go` (1,031 lines of product logic in
   the platform) into the NexusAI API. Re-express the ~330 lines of genuine
   core delta as config or adapters. Pin `localai/localai:v4.10.x`, unmodified,
   internal network only. Never merge upstream into this repo; clone it to a
   SIBLING directory if you need to compare.
   Security the fork is missing: deny-by-default auth (4.9 BREAKING), gallery
   SSRF fix (4.6), tar hardlink + cyclic $ref (4.8), four v4.10 CVEs including
   react-router.

P6 ANALYST PRODUCT UI [Codex] — separate app.
   **THE FULL UX CONTRACT IS `docs/ux/NEXUSAI_PRODUCT_UX.md`. Read it before
   writing any UI code.** It is authoritative for information architecture,
   every screen's states, the provenance/citation system, the clarification
   surface, the visual token system, accessibility, responsive behaviour, the
   component inventory, and the definition of done. Summary of the parts you
   must not get wrong:
   · IA is CASE → EVIDENCE → QUESTION. Evidence families are FILTERS, never
     navigation. There is no "Audio" or "Documents" section.
   · Answer order is FIXED and never reordered: (1) the answer sentence, with
     CLAIM-LEVEL citation markers (2) the result (3) citations (4) derivation,
     collapsed (5) limitations (6) follow-ups.
   · Citations deep-link to the EXACT locator — page+char span, row number,
     t_start, frame timestamp, bbox — and highlight on arrival. A citation
     that cannot produce an openable locator is not a citation.
   · A source row and a 0.31-confidence OCR fragment MUST NOT look identical.
   · Abstention is a first-class, designed experience: name the ambiguity,
     offer 2–4 options labelled with semantic-layer display names, one click
     re-runs. Never "please rephrase".
   · Column headers are semantic-layer display names — "Call duration", never
     "M1". No process jargon anywhere in analyst-facing text.
   · No model/backend/agent/template/SQL controls in any analyst surface.
   · Forbidden: flat white canvas, flat black canvas, purple AI gradients,
     neon/cyberpunk, glassmorphism, card soup, decorative motion.
   · Port `analyst*Presentation.js` and their tests; do not rewrite them.
   Team-lead UX reference: `https://github.com/satiricalguru/Local-Mind` —
   assessed in UX doc §2. Take its craft and density; REJECT its
   playground-per-modality IA, its glassmorphism and its GPU-Noir aesthetic.

P7 DEPTH, ADOPTION & SCALE. Media descriptors + cross-family joins in the
   layer. Document chunking (a PDF is currently ONE passage per document) +
   page/span citations + scanned-PDF OCR. OCR confidence calibration — photo
   OCR currently stores noise at 0.90 confidence, a truthfulness defect. ASR →
   audio-cpp + diarization (upstream 4.8/4.7). Rerank. ANPR/video derived-event
   querying. Face candidate-similarity only, never identity. Then upstream wave
   2: evaluate KNN routing (4.9) for S1/S4 — adopt only if it beats the
   deterministic incumbent on the golden set — plus context compression, PII
   pseudonyms, `local-ai benchmark`. Then GPU: Phase A (CPU laptop) → B (single
   GPU) → C (distributed workers), with product APIs unchanged across all three.

----------------------------------------------------------------------
11. SAFETY, TRUTHFULNESS, APPROVAL GATES
----------------------------------------------------------------------

NEVER fabricate: an analytical result, a citation, OCR text, a transcript, an
ANPR sighting, a face identity, a location, a timeline, or a cross-family
relationship. Never turn a similarity score into an identity claim. Never
present model confidence as ground truth. When a capability is unavailable on
this hardware, return a truthful capability state — never fake it, never
silently degrade.

PRESERVE: authorization, case/tenant scope, identifier correctness, dates,
numbers, provenance, lineage, source identity, citation identity, privacy (CNIC
masking enforced server-side at projection, not in the UI), evidence integrity.

THE COMPILER IS A SECURITY CONTROL: the enum-constrained schema plus S6 is what
prevents a model naming a field it is not authorized to see. Enforce
Sensitivity/RedactionState at S4 (never enumerate an unauthorized field) AND at
S6 (reject if one appears), not only at projection.

STORE THE IR WITH THE ANSWER. In a forensic product, "why did the system answer
this way" must be reconstructable months later. The compiled plan and the
abstention reason are AUDIT RECORDS, not debug output.

ASK FIRST, always: container rebuild or redeploy · any docker compose action
beyond `ps`/`logs` · any database write, migration or backfill · downloading a
model or installing a backend · any git operation that writes · long builds ·
running the live golden suite or WI-0's 40 live calls · anything touching
retained evidence.

REPORT HONESTLY: if a test fails, say so and show the output. If a step was
skipped, say so. If a number regressed, lead with it. "Partially working" is a
valid and valuable answer; "working" when it is not is the one unacceptable
outcome on this project.

----------------------------------------------------------------------
12. WORKING PROTOCOL — CLAUDE + CODEX SHARE THIS REPOSITORY
----------------------------------------------------------------------

Codex works here too. Before every edit run `git status --short` and
`git diff --name-only`. If a file you intend to change moved since your session
began, RE-READ it before patching. Never overwrite another agent's uncommitted
work.

TRACK SPLIT: Claude owns the compiler (`api/forensic_records/**` Go query and
answer layer). Codex owns the spine and surface (`semantic_layer/**` YAML,
`db/**` migrations, `ingestion/**`, the analyst UI app). Disjoint, both on the
critical path, genuinely parallel. At most two active work items, one per agent.

Synchronization is NEXUSAI_CONTINUATION.md, not commits.

When a work item completes, REPLACE (do not append to) the top section of
NEXUSAI_CONTINUATION.md with: ACTIVE_WORK_ITEM · OWNER · what changed · files
changed · tests run · RESULTS AS NUMBERS · known limitations · exact next
action. Keep that file under 300 lines. If it is still 874 KB when you finish
your first work item, archive it to
`reports/archive/continuation-20260921.md` (preserve it — it contains real
findings) and start a clean one.

----------------------------------------------------------------------
13. WHAT SUCCESS LOOKS LIKE THIS SESSION
----------------------------------------------------------------------

You inspect, reconcile against source, run WI-0, report its numbers against the
decision rule, then implement WI-1 with tests. If time remains, WI-2.

You do NOT: write an architecture document, add a template, extend query.go,
swap a model outside WI-0's rule, use embeddings for a structural decision,
redesign the UI, start Phase 3 before WI-0 reports, or produce a plan instead
of a patch.

Begin by reading the four authority files in §2, then `git status --short`, then
open `api/forensic_records/deterministic_semantic_compiler.go`,
`api/forensic_records/semantic_frame.go`,
`api/forensic_records/source_native_algebra.go` and
`pkg/functions/grammars/json_schema.go`, and confirm for yourself that
(a) `frame.Filters` has no consumer and (b) `enum` compiles to a GBNF
alternation. Then start WI-0.

======================================================================
END MASTER_EXECUTION_PROMPT
======================================================================


---
---

# CODEX_SYNC_BOOTSTRAP — v2
*(Separate, shorter paste for Codex.)*

======================================================================
BEGIN CODEX_SYNC_BOOTSTRAP
======================================================================

You are joining an in-progress project at
`C:\Users\sheik\Workspace\Office\Projects\NexusAI`. Claude is working in this
same repository. Your first job is to join safely without colliding.

STARTUP, in order:
  1. Read AGENTS.md.
  2. Read ONLY the newest entry at the top of NEXUSAI_CONTINUATION.md — the
     file is 874 KB of append-only history; do not read it all.
  3. Read reports/nexusai-tl-audit-20260918/P0-baseline-report.md — the only
     real measurement of product accuracy (34% correct-or-clarified,
     26 confident-wrong out of 62 questions).
  4. Run `git status --short` and `git diff --name-only`.
  5. Find ACTIVE_WORK_ITEM and its FILES_OWNED in NEXUSAI_CONTINUATION.md.

THE DIRECTION, so you do not work against it:
The team lead has ruled that we cannot keep defining operations. The 79
templates / 104 operations / 8,644-line keyword ladder in `query.go` are being
replaced by a GOVERNED SEMANTIC COMPILER: we define the DATA once — a curated
semantic layer of entities, dimensions, measures, metrics, synonyms and
declared joins — and one compiler turns any question into a validated typed
plan (`SourceNativePlanV1`, which already exists) and then into parameterized
SQL. An LLM fills that plan under a JSON Schema whose field slots are an `enum`
of authorized field IDs, so it cannot name a field that does not exist. It never
writes SQL, never names a table, never picks an operation. Verification and
abstention convert residual errors into clarifying questions instead of wrong
answers.

YOUR TRACK: the spine and the surface — `semantic_layer/**` (YAML curation),
`db/forensic_records/**` (migrations), `ingestion/forensic_records/**`, and the
analyst UI app. Claude's track: the Go query/answer compiler in
`api/forensic_records/**`.

BEFORE ANY UI WORK, read `docs/ux/NEXUSAI_PRODUCT_UX.md` in full. It is the
authoritative UX contract: information architecture (CASE → EVIDENCE →
QUESTION; families are filters, never navigation), every screen's states, the
provenance/citation system (claim-level markers, deep-links to exact locators,
visible degradation on weak evidence), the clarification surface, the visual
token system, accessibility, responsive rules, the component inventory, and the
definition of done. Do not invent UI structure — it is already specified there.

YOUR LIKELY FIRST ITEM (WI-4): the semantic layer schema, loader, validator, and
the curated CDR entity — every field with display_name, description, synonyms,
type, sensitivity, allowed filters/aggregates; metrics; the cdr→subscriber join.
Today `FieldDescriptorV1` has ZERO description fields and the catalog is
inferred per request from sampled rows. Curating it is the long pole and it
starts now.

COLLISION RULES — absolute:
  - Do NOT edit any file under another agent's FILES_OWNED while that item is
    open. Claude currently owns `deterministic_semantic_compiler.go`,
    `stim_fact_packet.go`, `answer_presentation.go`,
    `operation_applicability.go`, `p1_correctness_test.go`.
  - Before editing any file, re-read it. It may have changed; there is no
    commit boundary to tell you.
  - Declare your work item in NEXUSAI_CONTINUATION.md BEFORE starting:
    WORK_ITEM_ID · OWNER=codex · FILES_OWNED (explicit paths) · DEPENDENCIES ·
    EXPECTED_OUTPUT · TESTS · STOP_CONDITION.
  - At most one active Codex work item, disjoint from Claude's.

PRESERVATION — the repository has ONE commit and 852 untracked + 94 modified
files that exist nowhere else. NEVER run `git reset --hard`, `git clean`,
`git checkout --`, `git restore`, `git stash`, `merge`, `rebase`,
`cherry-pick`, `pull`, `push`, or any force operation. Never delete an
untracked file you did not create. Never `docker compose down`, `down -v`,
`prune`. Never reprocess, migrate or delete retained evidence. Commits and
pushes are not authorized — synchronization is NEXUSAI_CONTINUATION.md.

HARD RULES:
  - Do NOT add operation templates. The catalog is frozen and will be deleted.
  - Do NOT use embeddings for structural decisions (family, group, measure,
    field role). Two live defects came from exactly that. Embeddings are for
    semantic TEXT retrieval only.
  - Do NOT benchmark or swap LLMs. That is Claude's WI-0, which has a written
    decision rule.
  - Never merge upstream LocalAI into this repo. Clone to a SIBLING directory
    if you need to compare. Baseline v4.5.6; current upstream v4.10.0.

TRUTHFULNESS: never fabricate a result, citation, OCR text, transcript, ANPR
sighting, face identity, location, timeline or cross-family relationship. If a
test fails, say so with the output. If a capability is unavailable on this
hardware, return a truthful capability state rather than a degraded guess.

ASK FIRST: container rebuild/redeploy, any database write or migration, model
downloads, backend installs, long builds, any git write, running the live
golden suite.

WHEN YOU FINISH a work item, REPLACE the top section of
NEXUSAI_CONTINUATION.md with: what changed · files changed · tests run ·
results as numbers · known limitations · exact next action. Keep it under
300 lines.

======================================================================
END CODEX_SYNC_BOOTSTRAP
======================================================================
