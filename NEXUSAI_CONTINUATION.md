# NexusAI — Current State

> **CURRENT STATE ONLY. Rewrite it; never append. Hard cap 300 lines.** History in
> `reports/archive/`: `continuation-20260925.md` (WI-9..WI-31) · `continuation-wi0-wi8-20260923.md`
> · `continuation-20260921.md`. **TWO AGENTS WRITE THIS FILE:** everything above
> `## CODEX_UI_TRACK` is the backend track, that section is Codex's, neither edits the other's.

**Last updated:** 2026-09-29 · **Branch:** `codex/forensic-hybrid-checkpoint-20260723`

---

## 1. Where the product is

    SHIPPED POSTURE, measured live 2026-09-26 (Step 4b arm C -- media ON, see §7):
    golden 62    45 CORRECT · 14 CLARIFIED · 2 NOT_STATED · 1 MANUAL · 0 WRONG
    held-out 13  10 CORRECT · 2 CLARIFIED · 1 WRONG        both IDENTICAL with media on or off
    media 28     13 CORRECT · 11 CLARIFIED · 3 WRONG · 1 ROWCOUNT   (was 5 CORRECT · 7 WRONG)
    The 3 remaining media WRONG are wrong with media OFF too (H2, M15, P2 -- §3).

**THE 47 WAS NEVER REAL.** `contains` graded the whole response, so a value the analyst was never shown still
scored: EIGHT questions passed that way and the honest baseline was **39**. The grader now reads the ANALYST
TEXT and `NOT_STATED` is its own verdict — computed correctly, never stated, never a pass. Stage 0 took 39 -> 45
with zero new WRONG. Baseline 2026-09-18: 26% correct, 26 confident-wrong. **Every question completes inside the
P3 latency gate.** [`ANSWER_NOT_STATED.md`](reports/answer-not-stated-20260925/ANSWER_NOT_STATED.md).

### PHASE 1 — real-world questions: 7 WRONG -> 1, 2026-09-25

Held-out set (SQL-verified, phrasings nobody wrote for this system); golden is REGRESSION only. Four backend
fixes — [`PHASE1_RESULT.md`](reports/phase1-20260925/PHASE1_RESULT.md). The one to carry forward: **H2** — the
executor applied any extracted target as a SUBJECT predicate, so a verified plan over 739 rows returned 0. **A
verified plan asserting false absence is the worst answer this product can give: the analyst stops looking.**
**Curation (WI-LAYER-1): 81/94 -> 93/94 (99%).** **The one remaining WRONG:** H11 asks WHERE and the plan
returns a COUNT — the S9 `rows`-shape gap, deliberately NOT closed, since a `rows` rule would refuse the 8
questions answering correctly under the mis-assigned `lookup` goal. **Breakdown taxonomy first.**

**STANDING RULE: when curation lands, re-derive every expectation it touches** — three oracle defects in our OWN
held-out set surfaced that way (H8 expected 5, truth 6: `subscriber.status` lives under two headers and the
executor COALESCEs). **Derive an expectation from the SAME expression the executor uses**, or the set
congratulates the system for abstaining from what it can now answer.

**SILENT CONFIGURATION DRIFT, 2026-09-25, RESOLVED.** FIVE settings ran at compose's `:-false` defaults because
the env file defined none of them, and the 62 was scored that way before anyone noticed.
[`CONFIG_DRIFT.md`](reports/config-drift-20260925/CONFIG_DRIFT.md). **PROTOCOL: take the CONTROL deliberately —
the running container is NOT a baseline (§7) — and assert the switch state from the CONTAINER.** **A TIMEOUT
(`http: -1`) is never a verdict**; re-measure the mover in steady state, both ways.

**WHY THE 62 ARE REGRESSION, NOT CAPABILITY.** First held-out run: golden said **0 WRONG**, held-out said **7 of
13 WRONG**. **Capability claims cite HELD-OUT evidence.** **PHASE 1 GATE — MET 2026-09-23, holds:** 0
confident-wrong · 0 HTTP 500 · correct-or-clarified 98.4%. **PHASE 3 GATE — 3 of 4; abstention 22.6% vs <=20%,
ACCEPTED 2026-09-25** after five attempts produced SEVEN confident-wrong and ZERO conversions, each reverted on
a pre-written threshold. **The 14 are blocked BY DESIGN:** CDR-11 and TWR-02 fail on the invented-filter signal,
which **MUST NOT be removed**; SUB-03 is a PII boundary; the rest are correct clarifications.

### PHASE 3 — NOT COMPLETE: the ladder, and it CANNOT simply be switched off

P3 also requires deleting the template catalogue and the `query.go` ladder. Neither is done: **9,041 lines**
(was 8,644 at project start — it has GROWN) · 12 templates in use. Slice 1 gated it behind
`FORENSIC_LADDER_ROUTING` (**default ON**) and measured with it off: **3 confident-wrong and 3 correct lost**,
because `chooseTemplate` binds `req.Template`, which SCOPES family and record type. **The ladder is not only a
router; it is the product's main source of SCOPE**, it made a correct new goal completely inert (§7), and **its
retrieval templates ignore curated sensitivity (§3)**. Deleting it means deriving that scope from the curated
layer first. [`SLICE_RULE.md`](reports/ladder-deletion-20260925/SLICE_RULE.md). **Census caveat:**
`nexusai_route_census.py` under-predicted the risk surface — `execution_strategy` names the executor that
answered, not what the ladder contributed upstream.

## 2. Lessons — the two lists that bind. Reasoning: [`LESSONS.md`](reports/LESSONS.md)

**Every capability arrives with the check that bounds it, behind its own switch, default off, measurable and
withdrawable** — **except a change whose default-off state would be the unsafe one** (the Step-2 citation:
default-off keeps the lie). Verification proves a plan against ITSELF, never against the question · an invented
filter is a SYMPTOM and **a MISSING one has no symptom at all** · fix the instrument first · re-measure a
rejected candidate once its instrument is corrected · a probe must reproduce the PATH · a thing that ROUTES may
also SCOPE, **and does not necessarily BIND** · **an instrument that GUESSES at a cause is a defect** ·
**forecast a brief through the real mechanism before sending it** (it predicted 15/28 exactly) · **when a gate
widens, ask what ELSE it was holding** — it cost a PII disclosure the day after it was written (§7): a clean
census of the CLASSIFIER said nothing about where the reclassified questions would LAND · **a correct citation
cannot catch a mis-bound plan, only stop one from lying** · **write a threshold against what the change CAUSES,
not an absolute count** · **a switch must be DECLARED in compose, not just set in the shell.**

**Kept:** withhold-reason classification · allowlist fix (alphabetic identifiers, derived date bounds) · S9
scalar guard reading the curated layer · S4 distinct narrowing (+2) · measure and filter schema variants ·
persisted plan cache · `FORENSIC_LADDER_ROUTING` A/B switch · `FORENSIC_BREAKDOWN_GOAL` (built, neutral, OFF —
§7).

**Rejected after measuring — DO NOT RETRY:** removing generator-invented filters by ANY means (post-hoc or by
prompt) · switching the ladder off without replacing its scope · narrowing the issued FIELD set (a top-12 cut
made CDR-11 unanswerable at any model quality) · withholding measures for a `lookup` goal · "breakdown" →
aggregate goal · cross-verification of template answers · arbitrating CONFIDENT answers on a shape mismatch ·
withholding uncurated fields (cost TWR-02 a false negative: "there are no tower records in this case").

## 3. Known gaps and defect status — detail in [`OPEN_ITEMS.md`](reports/OPEN_ITEMS.md)

**The ones that bite next:** **the ladder's retrieval templates ignore curated sensitivity — a PII path** (§7) ·
P2 asserts a false absence from a time filter nobody asked for · nothing logs the SWITCH STATE at startup, an
omitted switch reads exactly like one set false, **and one omitted from COMPOSE never reaches the container at
all** (§7) · grammar failure is silent — `chat.go` logs `Failed generating grammar` and proceeds with NO
grammar, 200 response · the `lookup` default means "unrecognised" and skips shape verification on 26% of the 62
· **CDR-10 and ANPR-03 NOT_STATED on the TEMPLATE path** · CDR-09's grid heads two timestamps `M1`/`M2`.

**Still live on the ladder** (27/62 bypass the compiler fixes): **D1** unbound literals dropped · **D4**
cross-family misrouting · **D8** payload filters discarded unless the template is `canonical_records`
(`query.go:7728`) · **D7** "Explain …" treated as a dictionary definition.

**THIRTEEN oracle defects found in our OWN measurement code**, the last in the Step 4 comparator itself (§7).
**Read every expectation against the analyst-facing text, and re-derive them whenever curation lands.**

## 4. Architecture — decided, do not relitigate. Rationale: [`RECONCILIATION_20260921.md`](docs/architecture/RECONCILIATION_20260921.md)

1. **Stop defining operations.** 79 templates, 104 operations and the ladder are a *per-question* abstraction.
   **Frozen; deleted at the end of Phase 3.**
2. **Governed Semantic Compiler**: classify → literals → scope → narrow → enum-constrained IR → validation →
   one self-correction → parameterized SQL → verify → **abstain, never guess**. Build on `SourceNativePlanV1`. **Amended 2026-10-05:** a second lane, governed SQL (`FORENSIC_GOVERNED_SQL`, default off, `governed_sql_*.go`), lets the model write one validated, verified `SELECT` over analyst views for what the typed plan cannot say; see `docs/architecture/QUERY_ANSWERING_DECISION_20261005.md`.
3. **The keystone HOLDS, verified on the SERVING path.** The schema travels as JSON and
   `core/http/endpoints/openai/chat.go` converts it to a GBNF alternation per enum;
   `grammar_keystone_test.go` asserts it through the JSON round-trip — **calling the converter directly gives
   a misleading answer**, and once did.
4. **Deterministic SQL, narration-only LLM.** The model emits an enum-constrained plan and narrates a result it
   did not compute. **Every number comes from SQL.**
5. **Derived artifacts are never merged with ingested records.** A mixed plan is REFUSED, never joined; derived
   PROJECTION is refused for want of provenance; `source_truth_state` travels with every derived citation (§7).
   **A model's plate read is not a camera's sighting.**
6. **LocalAI becomes a pinned, unmodified upstream image** (`v4.10.x`), internal only. Fork baseline v4.5.6;
   divergence ~330 lines across 8 files. **NexusAI owns analyst identity and cases** — no case entity exists
   (`case_id` 3× vs `collection_id` 224×), every analyst is one shared principal, and that blocks deployment.
7. **UI contract is [`NEXUSAI_PRODUCT_UX.md`](docs/ux/NEXUSAI_PRODUCT_UX.md)** — IA is CASE → EVIDENCE →
   QUESTION, families are filters, standalone app.

## 5. Hard rules

- **No new operation templates**; do not extend `query.go`'s keyword ladder.
- **No model roulette** — no evaluating, benchmarking, downloading or swapping LLMs except via a decision rule
  written down BEFORE the run and surfaced to the user.
- **No embeddings on structural decisions** (family, group, measure, field role).
- **No reports without a measurement.**
- **Abstention is a success.** A confident wrong answer never is.
- **Never fabricate** a result, citation, OCR text, transcript, sighting, face identity, location, timeline or
  cross-family relationship.
- **Never merge upstream LocalAI into this repo.** Clone to a sibling directory to compare.
- **Every behavioural change is measured against a pre-declared threshold**, and reverted if it fires.
  **Fired three times, honoured three times** — most recently Step 4, 2026-09-26 (§7). **A safety fix whose
  default-off state would be the unsafe one ships without a switch** (the derived citation; the browse guard).

## 6. Operational hazards

- **Auth fails closed.** Needs `FORENSIC_RECORDS_API_KEY` from `.env.forensic-runtime.local`. **Never "fix" a
  crash-loop by disabling auth** — on 2026-09-17 a whole session served the forensic API unauthenticated on real
  evidence.
- **Set env from PowerShell, never Git Bash** (MSYS rewrites `/data/...`). **Postgres is host port 5433, db
  `localrecall`.** **`docker compose build` is unusable for LocalAI** (wedges the daemon); the forensic API
  builds fine. **`make` is not on PATH.**
- **An empty JSON-Schema `enum` is an unparseable grammar** — HTTP 500 before inference. Seed runtime enums with
  a constant; `semantic_dynamic_schema_test.go` walks every schema.
- **Never** `docker compose down`, `down -v`, `--remove-orphans` (would delete `nexusai-api-1`), `system prune`,
  `volume prune`; never reprocess, backfill, migrate or delete retained evidence. **Git:** never `reset --hard`,
  `clean`, `checkout --`, `restore`, `stash`, `merge`, `rebase`, `cherry-pick`, `pull`, `push` unless asked this
  session.

**Regression anchors** (SQL-verified, `nexusai-forensic-demo`): CDR 8,642 · IPDR 2,500 · ANPR 750 · access log
1,000 · subscribers 11 · towers 5 · transactions 4 — **12,912 rows**, no media.
`nexusai-multimodal-product-acceptance`: 9,580 structured + **865 derived artifacts**. **Live models:**
synthesis `qwen3-4b-instruct-2507-q4km-nxb21d-dev` (**Q4_K_M**), embeddings `qwen3-embedding-0.6b`. CPU only, ~4
tok/s, ~15.7 GiB RAM.

**Switches — ASSERTED FROM THE CONTAINER 2026-09-27**, persisted in `.env.forensic-runtime.local` AND declared in
compose: `VERIFIED_ONLY` · `IR_ARBITRATION` · `IR_FALLBACK` · `PLAN_CACHE` · `ANSWER_STATES_VALUES` ·
`MEASURE_DENOMINATOR` · `LADDER_ROUTING` · `MEDIA_FAMILY_ROUTING` · `DERIVED_ARTIFACT_EXECUTION` ·
`BOOLEAN_RESTRICTION` all true. **Shipped 2026-09-27, each measured 0 of 103 moved:** `CONSTRAINT_OBLIGATIONS=true`
(dropped numeric condition refuses, names it) · `RANGE_FILTERS=true` (GT/LT bound to the named field; headline
names the bound) · `RECORD_TYPE_VALIDATION=true` (unknown family refused, never narrated as "no records") ·
`SOURCE_ROWS_QUANTITY_GUARD=true` (A3.1; ladder-OFF: CDR-16 CORRECT) · `ARBITRATION_SUPERSEDES_SELECTION=true` (2026-09-28;
ladder-OFF: CDR-12 CORRECT; ladder-OFF now 66 vs 67 ON) · `RELATIONAL_CONDITIONS=true` · `LOCATION_OBLIGATION=true` (2026-09-28; H11
WRONG->CLARIFIED, shipped now 67/3 WRONG) · `ROW_COUNT_HEADLINE_GUARD=true` (S1; ABC-123 "1" -> 218) · `DERIVED_RESULT_LABELS=true` (S4) · `TEXT_SEARCH_NOT_ABSENCE`, `CLARIFICATION_NOT_RESULT` (S2/S3) · `EVIDENCE_SEARCH_FIRST` (U2a) · `PLATE_SEARCH_ONLY` (U2b, owner: search-only) · `RESULT_COLUMN_LABELS` (A1; shipped 68/2 WRONG) · `TABLE_HEADER_CASING` (A1.1) · `CITATION_TRUTH_STATE` (A2) · `TEMPLATE_STATES_RESULT` (A3; CDR-10, ANPR-03 -> CORRECT) · `COUNT_NAMES_FIELD` (A4) (2026-09-29; **shipped 70/2 WRONG**, image de5c372e1b70, 30 on/9 off, pre-flight 38/38). **Parked OFF:** `CONVERSATION_ROUTER` (B1). **OWNER DIRECTION 2026-09-29: runtime queries replace templates** -- census `reports/templates-off-20260929/RESULT.md` (ladder OFF 68/4 WRONG: CASE-01, X-01 false absence; text search is 6 of 9 template-only answers). Plan: `docs/work/ROADMAP_20260929.md`. **Held OFF:** `IDENTIFIER_BINDING`, `FRAME_FAMILY_PRECEDENCE`. **REVERTED 2026-09-27:** compiler-first routing
(A2), deterministic arbitration (A3.3), compiled family scope. `docs/work/CAPABILITY_LOG.md`. **DECLARE EVERY SWITCH IN COMPOSE.**

**ASK THE PRODUCT OWNER FIRST:** container rebuild or redeploy · any `docker compose` action beyond `ps`/`logs`
· any database write, migration or backfill · model downloads or backend installs · any git operation that
writes · long builds · running a live evaluation suite · anything touching retained evidence.

## 7. Next — RE-BASED 2026-09-26

    (0) ANSWER PATH 7 WRONG->1 · (1) VOCABULARY 86%->99%, routing 7/28->15/28   CLOSED
    (2) MEDIA ANSWERING SHIPPED 5/7 -> 13/3 · (3) SCOPE bound by the ladder OPEN
    (4) HELD-OUT 13+28, needs >=40 structured   PARTIAL
    (5) ANALYST UI on the live API 2026-09-27; script docs/work/DEMO_SCRIPT_20260928.md   DEMO-READY

**BREAKDOWN GOAL — BUILT, MEASURED, NEUTRAL, OFF 2026-09-25**, behind `FORENSIC_BREAKDOWN_GOAL`, registered at
all five sites a new goal needs: 62/62 identical verdicts, all four targets BYTE-IDENTICAL. **The ladder answers
them first** (`query.go:2418` keys on "breakdown"), so the goal is correct and UNREACHABLE. **THIS REVERSES THE
ORDERING** — it cannot pay until the ladder stops answering them, but it stays a PREREQUISITE (it empties 4 of
the 17 `lookup` questions, and an S9 `rows` case cannot be added while that bucket holds six kinds of question).
Measure it TOGETHER with the `rows` case, and **RE-MEASURE: its neutral verdict was scored against a control
that over-credited four of its targets.** **STAGE 0 DONE 2026-09-25** behind `FORENSIC_ANSWER_STATES_VALUES`,
+4, ON. CDR-10 and ANPR-03 (registered templates) now state their result: A3, shipped 2026-09-29. They
still need the runtime planner to express them before those templates can retire (templates-off census).

### MEDIA — **SHIPPED 2026-09-26** as Step 4b arm C. Step 4 fired first and was reverted.

Evidence: [`STEP4B_RESULT.md`](reports/step4b-media-20260926/STEP4B_RESULT.md) · [`STEP4_RESULT.md`](reports/step4-media-20260926/STEP4_RESULT.md) · [`PROVENANCE_LABEL.md`](reports/provenance-label-20260926/PROVENANCE_LABEL.md) · [`TEXT_BROWSE_GUARD.md`](reports/browse-guard-20260926/TEXT_BROWSE_GUARD.md) · [`TERMINAL_EMPTY_200.md`](reports/terminal-fix-20260926/TERMINAL_EMPTY_200.md)

    media 28   control  5 CORRECT ·  7 WRONG     (media OFF)
               arm B   14 CORRECT ·  5 WRONG     (derived-catalogue isolation)
               arm C   13 CORRECT ·  3 WRONG     (+ boolean restriction)   <- SHIPPED
    golden 62 and held-out 13: IDENTICAL in all three arms, 0 transitions on 75 questions.

**Arm C introduces ZERO new confident-wrong and removes FOUR.** Its three remaining (H2, M15, P2) are wrong with
the switches OFF too, so turning them off only discards the four. **THE GATE PASSED:** arm A reproduced Step 4's
control identically on a rebuilt image, making B and C attributable; the ship then reproduced arm C exactly.

**DEFECT 1 — ROUTING AND CATALOGUE SELECTION DISAGREED. FIXED.** M2 "average OCR confidence of the PLATE READS"
resolved `anpr_model_observation` and then read **`forensic.records`**: `mergeSemanticLayerCatalog` appended
schema-inferred RECORDS columns into a DERIVED family's catalogue and the binding followed the plan's fields
exactly as designed. **Nothing refused it — the plan did not MIX sources. The mixture has to not be OFFERED.**
It answered 0.89 from ingested sightings where the truth is 0.955573. **The citation was HONEST** and named
records, which made it diagnosable: **a provenance label cannot catch a mis-bound plan, only stop one from
lying.** A derived family is now issued its curated fields and nothing else; **structured families KEEP the
merge** (withholding there cost TWR-02 a false negative). Live: M2 CORRECT 0.96, zero records citations.

**DEFECT 2 — A DROPPED FILTER WAS ANSWERED, NOT REFUSED. FIXED** behind `FORENSIC_BOOLEAN_RESTRICTION`. Every
derived plan carried `filters: null`: M12 said 309 where the truth is 46, M7 said 24 where the truth is 0 —
both bound to the RIGHT table and both VERIFIED, because **verification proves a plan against ITSELF, never
against the question.** An INVENTED filter is a symptom this system watches for; **a MISSING one has none.**
Census first: 3 questions across all corpora, **zero structured**. **M3 CORRECT -> CLARIFIED is an accepted loss
— `manual_review_required` is true on all 307 rows, so the count matched by accident. Right by coincidence is
not a capability.**

**WHAT HELD.** P1 answers **1,057**, never 307 — no fabricated sighting. H1 H3 H4 H5 M6 clarify. **Step 2's
provenance verified on the serving path:** every media answer cites `derived_artifacts_sql` with `artifact_id`
and `source_truth_state`, no records columns. **A NEAR-MISS, AND A THRESHOLD FLAW.**
`FORENSIC_BOOLEAN_RESTRICTION` was set in the shell but not declared in compose, which **forwards only what it
names** — arm C would have measured the rule silently OFF. And §3.4 did not separate confident-wrong CAUSED by
the change from what it does not touch, so read literally it mandated keeping 7 wrong answers to avoid 3. **Both
readings were reported and the product owner ruled; a threshold is not reinterpreted after seeing the result.**

**THE LADDER'S PII PATH — FOUND AND GUARDED. The boundary covers the COMPILER, not the LADDER:** `CatalogFields`
drops PII/RESTRICTED so no typed plan can name those fields, while `derivedTextEvidenceSQL` reads the five text
fields straight out of the JSON. **I caused the discovery:** fixing H2's misclassification sent it to
`audio_transcript_search`, which returned the actual Urdu transcript. Reverted the same day. **The lesson was
already written down and I still walked into it — a census of the CLASSIFIER said nothing about where the
reclassified question would LAND.**

**The defect is UNTARGETED BROWSE, not search.** `derived_text_relevance.go:83` judges every passage relevant
when a question names nothing to filter on, so all come back IN FULL. **AUD-03 showing an Urdu snippet is
CORRECT** — the analyst supplied the identifier and the snippet IS the citation; AUD-01/02, DOC-02/03 and IMG-01
are the same, all CORRECT in the 62. `FORENSIC_TEXT_BROWSE_GUARD` (**default ON**, in compose AND persisted)
withholds text for untargeted requests, keeps count/files/languages/timestamps/locators and **says it withheld
them**. Census first: **1 question, H2, zero structured.** **Live probe: untargeted -> ZERO Urdu anywhere in the
payload; targeted -> snippet intact. Golden 62: ZERO transitions.**
[`TEXT_BROWSE_GUARD.md`](reports/browse-guard-20260926/TEXT_BROWSE_GUARD.md).

**EMPTY 200s FIXED.** All FOUR terminal classes returned HTTP 200 with NO analyst-facing text:
`terminalRequestResponse` filled `resp.Answer` and never built `resp.Enterprise`, where the analyst surface and
the grader both read — **the server produced an answer and dropped it**; H2/M15 went 0 -> 243 chars. It does NOT
use `finalizeEnterprisePayload`, which asserts an EVIDENCE shape these classes lack. **Those four classes ARE
the unwired conversational surface (§7 item 5).**

**Remaining, in order:**

1. **PII MASKING — BUILT 2026-09-26, NOT DEPLOYED, NOT MEASURED.** `FORENSIC_PII_MASKED_PROJECTION` (default
   off) + `FORENSIC_PII_ALIAS_SECRET` (FAILS CLOSED without it). Plates project as one opaque, fixed-width,
   case-scoped HMAC alias shared across both families; **the key is not optional — plate strings are LOW
   ENTROPY, so an unkeyed digest is reversible by enumeration.** Masked fields are **not sortable**: SQL orders
   by the RAW value while the analyst sees aliases, so the ordering would hand back what the alias hides.
   DB-proven: 24 labels aliased, 24 plates none raw. **A BUG IT INTRODUCED, caught by an existing test:**
   admitting every `PII`/`MASKED` field let `subscriber.cnic` into the catalogue to be run through the PLATE
   aliaser. Fixed with a scheme whitelist — not "is this MASKED?" but **"can this actually be masked?"**
   OCR/transcript stay WITHHELD; subscriber fields have no scheme. **DECISION OWED FIRST:** with masking on, H1
   stops being a refusal and becomes an answer IN ALIASES — re-rule H1 and its threshold before shipping.
   [`PII_MASKING.md`](reports/pii-masking-20260926/PII_MASKING.md).
2. **M4 went CORRECT -> CLARIFIED** when media routing landed — a lost answer, not a wrong one, so no
   threshold fired, but it is unexplained and worth one trace.
3. **Derive SCOPE from the curated layer, then delete the ladder in slices** — it blocks the taxonomy work
   and Phase 3's remaining deliverable.
4. **Then the `rows` case + breakdown goal together**, plus "longest/shortest" -> `rank` and ANPR-03 "first and
   last seen" -> `range`, each against a control. **Worth ~7 media questions.**
5. **The unwired question classes** — greetings, chit-chat, product-help, capability, concept explanation,
   out-of-scope. They do not route; a real user hits them in the first minute, and the product owner has asked
   for them explicitly. D7 ("Explain ...") is the same area. **The LLM's legitimate role: no SQL, no evidence,
   and never allowed to answer about data.**
6. **Extend the held-out set to >=40**, derived from the DATA. It is the primary gate; the 62 are regression
   only. Re-derive expectations whenever curation lands (§1).
7. **Numeric-offset subtraction** (H3): the allowlist has timestamp subtraction only.
8. **Backend contracts Codex is blocked on:** login/session · tenant/role/classification · cursor-paginated
   structured records · normalized page/region/timed locators · case entity · server-persisted pins and history
   · Dashboard data. **Never let the UI simulate these.**

**Cross-family is 0 of 5 and the joins we assumed existed do not.** `cdr.msisdn` -> `subscriber.msisdn` and
`ipdr.subscriber_id` -> `subscriber.msisdn` each match **0 rows** and were removed as false declarations; only
`cdr.lac_id` -> `tower.lac` survives. **A declared join matching nothing manufactures a confident empty
answer.** Nothing consumes `Joins` at runtime. Decision owed: duplicate-safe join execution, and whether to
allow `many_to_many` (CDR<->IPDR is real but many-to-many, which the v1 validator rejects). **Read more:**
`docs/work/MEDIA_QUERY_ROADMAP.md` · `docs/work/MASTER_EXECUTION_PROMPT.md` · `docs/ux/NEXUSAI_PRODUCT_UX.md`.
**Phase 0 debt:** rebuild the LocalAI v4.5.6 baseline in a SIBLING clone ->
`docs/integration/FORK_DELTA_v4.5.6.md`.

## CODEX_UI_TRACK

- **Identity + shell complete 2026-09-29:** Mineral Signal tokens and Type Proof govern the live workspace; the three-band shell has a code-native NexusAI lockup, scoped command search, persistent/collapsible desktop rail, focus-trapped mobile drawer, compact case context and coordinated navy-charcoal navigation/canvas surfaces. Noto remains for mixed-script coverage. `docs/work/UI_REDESIGN_RESEARCH_20260929.md` holds the primary-source rulings.
- **Dashboard under final product review:** the live page is a real readiness workbench, not KPI card soup. It now combines direct workspace totals, a true multi-segment readiness donut, linear ingestion accounting, ranked structured-family bars with linked filtering, a readiness-ordered queue, synchronized inspector tabs and a selected-case readiness plot. Processing/failure details, curated questions and browser-local recent work derive from existing responses; unsupported trends, priority, assignment and full-census claims stay absent.
- **Measured gate:** 109 unit + 46 ported tests green; the 11-test token matrix proves text >=4.5:1 and components/focus >=3:1 in both themes; clean 2,039-module build; four Dashboard/shell Playwright contracts green. Light/dark captures cover 1440/1280/1024/768/375 plus drawer, collapsed rail, command search and case shell; 375px has zero page overflow and the keyboard drawer path traps focus, closes on Escape and returns focus. The real `127.0.0.1:4181` page was checked with 2 cases / 55 sources / 53 ready / 2 failed. No deployment, container, API, semantic-layer or DB change.
- **2026-09-29 Claude took over the UI track.** Binding: `docs/work/UI_REDESIGN_MASTER_GUIDE.md` (staged plan S0..S12: shell, header, top nav, sidebar, canvas, Dashboard section by section, then every page; research before each stage; backend needs go to `BACKEND_REQUESTS.md`) plus `UI_REDESIGN_BRIEF_20260929.md`; both supersede Codex's hold point below. **First pass done, to be redone stage by stage:** Identity A tokens (`tokens.css`, `identity-a.css`), gradient sidebar, live badges, case breadcrumb, `/investigate` page, Dashboard on ECharts (`components/charts/*`, own 173 kB gz chunk), audit `UI_PAGE_SPECS/shell-and-dashboard-audit.md`, palette gallery A/B/C. 142 unit tests green; dashboard e2e main test green. **Stale e2e from the old contract, fix in their stage:** `dashboard-slice` capture, `shell-redesign-slice` capture, `header-context` capture, `navigation-rail` rail label. **S1 shell frame + S2 header done** (`UI_PAGE_SPECS/shell.md`, `shell.css`; 149 unit tests green, header contrast tested). **S3 case bar done** (breadcrumb, readiness pill, section links with aria-current; case sections left the sidebar; 153 unit + 46 ported tests green; vitest timeout 15 s). **S4 sidebar done** (one Cases category with live status, pinned/recent two-line rows, inline actions, sticky collapse; rail 264 px; 162 unit + 46 ported tests green). **S5 canvas done** (`UI_PAGE_SPECS/canvas.md`: page tokens, PageHeader v2 without eyebrow, Card, unified state panels, Skeleton/ShellSkeleton, mobile section-bar scroll shadows; 172 unit + 46 ported tests green). **Team-lead revision done 2026-09-30:** recent/pinned questions left the sidebar for the command palette (empty-query recents, all cases) and a "Your questions" card on `/investigate`; header "Ask a question" is a measured ghost-accent button (a light-theme invisible-label defect fixed); Appearance is an icon button with a radio-tile panel (`AppearanceMenu.jsx`, shared `useThemePreference`). 186 unit + 46 ported tests green; two e2e specs updated for the new Appearance selector. **Dashboard 2026-09-30:** D1 header and D2 metric cards accepted; the page is now a zoned, widget-based layout (`UI_PAGE_SPECS/dashboard-architecture.md`, `dashboard-product-plan.md`): hero = real `activity_by_day` chart (`lib/caseActivity.js`, `useCaseActivity.js`, cap labelled partial) beside a compact attention rail, then an Evidence map (bubbles) beside readiness. Backend requests 14 (corrected: reprocess exists) and 15 to 19 filed. Next: Retry dialog on the rail, Ask bar, then readiness, ingestion, workbench. Shots: `apps/investigation-workspace/design-review/ui-redesign-20260930/`. No API, DB or Docker change.
- **Rulings / hold point (Codex, superseded):** every visualization requires a named question, authoritative source, useful encoding and filter/drill-through; chart variety follows the question, never decoration. `docs/work/BACKEND_REQUESTS.md` specifies complete processing history, evidence census, collection directory and authorized activity contracts needed for later truthful visuals. Do not advance beyond Dashboard until product review accepts this surface; preserve WI-LAYER-8's separate product-owner gate.
