# Codex Bootstrap — NexusAI

> Paste the block below into a fresh Codex session opened at this repository root.
> Canonical sources: `AGENTS.md` (guardrails) · `NEXUSAI_CONTINUATION.md` (current state,
> newest entry only) · `docs/architecture/RECONCILIATION_20260921.md` (why) ·
> `docs/ux/NEXUSAI_PRODUCT_UX.md` (UX contract) ·
> `docs/work/MASTER_EXECUTION_PROMPT.md` (the Claude track).

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
