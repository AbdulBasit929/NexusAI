# NexusAI — Current State

> **CURRENT STATE ONLY. Rewrite it; never append. Hard cap 300 lines.**
> History: [`continuation-20260925.md`](reports/archive/continuation-20260925.md) (WI-9..WI-31
> in full) · [`continuation-wi0-wi8-20260923.md`](reports/archive/continuation-wi0-wi8-20260923.md)
> · [`continuation-20260921.md`](reports/archive/continuation-20260921.md).

**Last updated:** 2026-09-25 · **Branch:** `codex/forensic-hybrid-checkpoint-20260723`

> **TWO AGENTS WRITE THIS FILE.** Everything above `## CODEX_UI_TRACK` is the backend track;
> that section is Codex's. Neither edits the other's.

---

## 1. Where the product is

    62-question golden suite v2, --analyst-text, live, CONFIRMED 2026-09-25:
    47 CORRECT · 14 CLARIFIED · 1 MANUAL · 0 WRONG
    latency (warm cache) median 2.2s · p95 5.2s · max 5.9s

Evidence: [`phase3-close-20260925`](reports/phase3-close-20260925/) — a full confirming run on
the deployed build, ZERO verdict differences from the recorded state. Baseline for comparison:
26% correct, 26 confident-wrong (2026-09-18).

**Every question completes inside the P3 latency gate** — max 5.9 s against 15 s, warm.

### PHASE 1 — held-out 7 WRONG -> 3, golden unchanged throughout, 2026-09-25

Reconciliation: [`RECONCILIATION_20260925.md`](docs/architecture/RECONCILIATION_20260925.md).
Every fix measured on BOTH suites. Golden stayed 47 CORRECT · 14 CLARIFIED · 0 WRONG with
**no verdict or execution-strategy change** across all of them.

    held-out   5 CORRECT · 7 WRONG   ->   9 CORRECT · 3 WRONG · 1 CLARIFIED

    H2  KEPT  the executor applied ANY extracted target as a SUBJECT predicate
              (`primary_target='VOLTE'`) although classifyTargetType already returned
              "call_type". A verified plan over 739 rows returned 0 -- a FALSE NEGATIVE.
    H1  KEPT  "different handsets" counted rows, not distinct IMEIs. "different" implies
              distinct ONLY without a superlative -- unguarded it captures CDR-07 ("the
              MOST different people"), a ranking. Rank markers are now ONE shared list.
    H7  KEPT  -> CORRECT (46). TWO fixes: multi-word declared synonyms never bound
              ("server errorS" vs "server error") because this matcher compared raw words
              while everything else stems; then an S9 RESTRICTION OBLIGATION refused the
              unfiltered plan, and arbitration EARNED one that filters on 500.
    H12 REVERTED withholding uncurated columns closed the field-substitution vector and
              cost TWR-02 a FALSE NEGATIVE via an invented `district IS_NULL` filter.

**Two S9 obligations added, each with a control that MEASUREMENT supplied and intuition
would have missed:**

- **Measure obligation** — an `avg`/`sum` question cannot be answered by COUNT. Restricted
  to avg/sum deliberately: `max` also arrives from "most", where the correct plan is
  GROUP BY + ORDER BY + LIMIT. Including max/min would have refused FOUR correct answers.
- **Restriction obligation** — a value named through a MULTI-WORD curated synonym that the
  plan never filters on is a dropped restriction. Single-word matches do not count: `CALL`
  is declared by any question containing "call", and SEVEN correct answers would be refused.

**THE ENUM IS NOT A MENU** (twice proven: ladder scope, uncurated fields). Changing any part
of the issued field set changes plans for questions that already worked.

**The 3 remaining, and TWO are NOT code:**

    H12  "average beam width" -> "average AZIMUTH is 209.97"   CURATION (WI-LAYER-1)
    H11  "where was X seen"    cdr location uncurated, 8,642 rows   CURATION (WI-LAYER-1)
    H4   ROOT-CAUSED AND FIXED, AWAITING DEPLOY. "What is the largest network volume on
         any call record?" classified GENERAL_DOMAIN_KNOWLEDGE -- a DEFINITION request --
         so it never reached the compiler (30ms, terminal route) and answered "a bounded
         general definition is unavailable for that term". `IsConceptExplanation` keys on
         the shape "what is X", which also fits "what is the LARGEST ...".
         A definition request never asks for a maximum: an aggregate/superlative marker
         now vetoes the concept reading. Measured across the 75-question corpus, exactly
         TWO questions reach that branch -- NEG-03 ("blood type", correctly refused, no
         marker) and H4. Only the analytical one moves.
         "mean" is deliberately EXCLUDED from the markers: the existing classification
         test caught that "What does IMSI MEAN?" is the canonical concept phrasing.

**H8 WAS AN ORACLE DEFECT OF MY OWN — the sixth on this project and the first one we wrote.**
"How many subscribers are currently active?" answers **6** and 6 is CORRECT.
`subscriber.status` is stored under TWO headers depending on the source file -- `status`
(5 ACTIVE) and `account_status` (1 ACTIVE) -- and the layer declares both as `source_names`,
so the executor's COALESCE counts 6. My SQL read one header. **Derive an expectation from the
SAME expression the executor uses, not from the first column that looks right.** Held-out
corrected to 9 CORRECT · 3 WRONG · 1 CLARIFIED.

H12 now emits a correct `AVG` over an UNDESCRIBED field, so no operation-level guard can
catch it — only a description makes the field choosable. **WI-LAYER-1 is unblocked and owns
2 of the 3.**

### THE HELD-OUT RESULT — READ THIS BEFORE TRUSTING ANY NUMBER ABOVE

[`holdout-20260925`](reports/holdout-20260925/) ·
[`holdout_questions_v1.json`](evaluation/holdout_questions_v1.json): 13 questions written
against the DATA, SQL-verified, phrasings nobody wrote for this system.

    golden 62   0 WRONG        held-out 13   7 WRONG · 5 CORRECT · 1 CLARIFIED

    H1  "how many different handsets"      -> 8,642 (all rows)   truth 4,997 distinct IMEIs
    H2  "how many VoLTE calls"             -> "there are NO CDR records involving VOLTE"
                                              truth 739 — A FALSE NEGATIVE
    H7  "how many server errors"           -> 1,000 (all rows)   truth 46 — filter dropped, D1's signature
    H8  "subscribers currently active"     -> 6                  truth 5
    H11 "where was 923001110001 seen"      -> "the cell site with the LOWEST count is CELL-ARP-08"
    H12 "average BEAM WIDTH of the towers" -> "the average AZIMUTH is 209.97" — FIELD SUBSTITUTION
    H4  "largest network volume"           -> empty answer, 10 ms

**The 62 measure the system against questions it was built around.** On unseen phrasings it
answers a different field (H12), asserts absence that is false (H2), silently drops the
filter (H7), and counts rows where distinct values were asked for (H1). The honesty probes
did not clarify — **they fabricated.**

**This supersedes "0 confident-wrong" as the headline.** That number is true of the 62 and
NOT true of the product. Every capability claim from here must cite held-out evidence; the 62
prove only that we have not regressed on what we already knew to ask.

### PHASE 1 GATE — MET 2026-09-23, holds

0 confident-wrong · 0 HTTP 500 · correct-or-clarified 98.4% · answer stated in analyst text.

### PHASE 3 GATE (on the 62) — 3 of 4; abstention 22.6% vs ≤20% ACCEPTED 2026-09-25

Accepted after five measured attempts produced SEVEN confident-wrong and ZERO conversions,
each reverted on a pre-written threshold: drop invented filters post-hoc (2 wrong) · prompt
them away (4 wrong, incl. a negative control) · filter variant split (0/0, KEPT for
structural safety) · TWR-02 shape (measured dead, not built) · goal vocabulary (1 wrong).

**The 14 are blocked BY DESIGN.** CDR-11 and TWR-02 fail on the invented-filter signal, which
marks a plan the model did not understand and **MUST NOT be removed** — proven twice from
opposite directions. SUB-03 is a PII boundary. The rest are `no_family` coverage and correct
clarifications. **But see the held-out result above before treating this gate as capability.**

### PHASE 3 — NOT COMPLETE. One deliverable remains.

The PHASE also requires deleting the template catalogue and the `query.go` ladder
(MASTER_EXECUTION_PROMPT §10 P3). Neither is done: **9,041 lines** (was 8,644 at project
start — it has GROWN) · 12 templates in use.

### SLICE 1 MEASURED 2026-09-25 — THE LADDER CANNOT SIMPLY BE SWITCHED OFF

Rule: [`SLICE_RULE.md`](reports/ladder-deletion-20260925/SLICE_RULE.md) · evidence:
[`slice1-ladder-off`](reports/ladder-deletion-20260925/slice1-ladder-off/). Slice 1 deleted NO
code — it gated the ladder behind `FORENSIC_LADDER_ROUTING` (**default ON**, unlike every
other switch here, because it gates EXISTING behaviour) and measured with it off.

    47 CORRECT · 14 CLARIFIED · 0 WRONG   ->   44 · 14 · 3 WRONG
    CASE-01 · TWR-02 · X-01  CLARIFIED -> WRONG
    CDR-12 · CDR-16 · DOC-02  CORRECT -> CLARIFIED

**The ladder stays ON.** Threshold fired; restored and confirmed on all seven.

**THE MECHANISM the next attempt must solve.** `chooseTemplate` binds `req.Template`, which
then SCOPES family and record type for the compiler. TWR-02 went CLARIFIED → WRONG **while
still reporting `bounded_source_native_algebra`** — the compiler ran either way, but unscoped
it ran wider and asserted something false. X-01, the cross-family false-negative guard, began
asserting absence. **The ladder is not only a router; it is the product's main source of
SCOPE.** Deleting it means first deriving that scope from the curated layer.

**INSTRUMENT CORRECTION.** The census predicted a risk surface of 5 and **under-predicted
badly** — six regressions, three confident-wrong, mostly OFF that surface.
`execution_strategy` names the executor that produced an answer, not what the ladder
contributed upstream; the script now says so. **Its before/after diff is sound** and is what
caught the six. Current (ladder ON) census: compiler 39 · media/derived-text 18 · other 5,
superseding the old "27 of 62 bypass the compiler" figure.

---

## 2. Lessons, each paid for with a measurement

**Every capability arrives with the check that bounds it, behind its own switch, default off,
measurable and withdrawable.**

1. **Verification proves a plan against ITSELF, never against the question it answers.**
   DOC-04 (missing filter) and DOC-01 (every filter present, wrong evidence) taught it twice.
2. **An invented filter is a SYMPTOM, not a blemish.** Suppress it and the disease reaches
   the analyst — proven twice, from opposite directions.
3. **Fix the instrument first.** Three runs mis-measured (WI-6); 119 s attributed to nothing
   (WI-14); five withholding causes sharing one code (WI-20); a coverage audit that indicted
   every working field because it treated field IDs as column names (2026-09-25).
4. **Re-measure a rejected candidate when the instrument that rejected it has been
   corrected.** DOC-01's fix measured 1:1 and was rejected, then 1:0 on a corrected oracle.
   *The fix did not change; the evidence about it did.*
5. **A probe must reproduce the PATH, not just the function.** Calling the right GBNF
   converter with a `[]string` enum instead of the JSON the server receives produced a
   confident, plausible, entirely wrong conclusion that the keystone was broken. It was not.
6. **A thing that routes may also SCOPE.** The ladder looked like a keyword router; switching
   it off cost 3 confident-wrong because it was also binding family and record type.

**Kept:** withhold-reason classification · allowlist fix (alphabetic identifiers, derived date
bounds) · S9 scalar guard reading the curated layer · S4 distinct narrowing (+2) · measure and
filter schema variants · persisted plan cache · `FORENSIC_LADDER_ROUTING` A/B switch.

**Rejected after measuring — DO NOT RETRY:** removing generator-invented filters by ANY means
(post-hoc, or by prompt) · switching the ladder off without replacing its scope · narrowing
the issued FIELD set (a top-12 cut made CDR-11 unanswerable at any model quality) ·
withholding measures for a `lookup` goal · "breakdown" → aggregate goal · cross-verification
of template answers · arbitrating CONFIDENT answers on a shape mismatch.

---

## 3. Known gaps, characterised but not closed

- **The goal taxonomy has no `breakdown`.** `aggregate` + no group hints resolves to shape
  `scalar`, so calling an explicit breakdown `aggregate` asserts it is a single number (ACC-02
  went CORRECT → WRONG proving it). The `lookup` default means *"unrecognised"* and skips
  shape verification — **16 of 62 (26%) unclassified; 8 answer CORRECT with no S9 shape
  check.** A fix adds a breakdown goal with a grouping obligation, touching every consumer of
  `frame.Goal`. **Do NOT add an S9 `rows` case first** — it would refuse those 8.
  Separable, never measured alone: "longest"/"shortest" → `rank`.
- **Grammar failure is silent.** If generation fails server-side, `chat.go` logs
  `Failed generating grammar` and **proceeds with NO grammar at all** — every constraint lost,
  200 response. Worth a startup assertion.
- **Cold-start timeout.** Any schema change invalidates every plan-cache key; on the cold pass
  a generating question can cross the planner timeout and lose an answer it would give warm.
- **Distinct answers do not name what was counted** ("the count across CDR records is 10" when
  there are 8,642) · **`ir_shadow_*` fields are shadow-mode only**, so
  `withhold_detail`'s `issued_fields=0` means "not recorded", not "empty enum".

---

## 4. Defect status

**FIXED AND CONFIRMED:** D3 HTTP 500s (0/62) · D2 spurious GROUP BY · Fact Packets carried
plumbing. **FIXED AT THE COMPILER, STILL LIVE ON THE LADDER** (27/62 bypass them): **D1**
unbound literals dropped · **D4** cross-family misrouting.

**OPEN:** D8 payload filters discarded unless the template is `canonical_records`
(`query.go:7728`) · D7 "Explain …" treated as a dictionary definition · 3 `no_family`
structured gaps (CDR-07, CDR-10, ANPR-03).

**Oracle defects found (five):** filename checks scoring a coverage listing (DOC-01,
AUD-01/03) · CDR-13 expecting raw values the UX contract forbids · CDR-08/ANPR-05 carrying
`gold: null` with an `inexpressible` note that expired when COUNT_DISTINCT shipped. **Read any
expectation against the analyst-facing text.** SUB-03's *scoring* gold is correct (masked);
only `scripts/ir_spike/gold_plans.json` holds the raw CNIC.

---

## 5. Architecture — decided, do not relitigate

Rationale: [`docs/architecture/RECONCILIATION_20260921.md`](docs/architecture/RECONCILIATION_20260921.md).

1. **Stop defining operations.** 79 templates, 104 operations and the ladder are a
   *per-question* abstraction. **Frozen; deleted at the end of Phase 3.**
2. **Governed Semantic Compiler**: classify → literals → scope → narrow → enum-constrained IR
   → validation → one self-correction → parameterized SQL → verify → **abstain, never guess**.
   The IR (`SourceNativePlanV1`) and its SQL compiler already exist; build on them.
3. **The keystone HOLDS, verified on the SERVING path.** The schema travels as JSON and the
   server converts it (`core/http/endpoints/openai/chat.go`) to a GBNF alternation per enum.
   `grammar_keystone_test.go` asserts it through the JSON round-trip — calling the converter
   directly gives a misleading answer.
4. **LocalAI becomes a pinned, unmodified upstream image** (`v4.10.x`), internal only. Fork
   baseline v4.5.6; rebase impossible; core divergence ~330 lines across 8 files.
5. **NexusAI owns analyst identity and cases.** No case entity exists (`case_id` 3× vs
   `collection_id` 224×); every analyst is one shared principal. Blocks real deployment.
6. **UI contract is [`docs/ux/NEXUSAI_PRODUCT_UX.md`](docs/ux/NEXUSAI_PRODUCT_UX.md)** — IA is
   CASE → EVIDENCE → QUESTION, families are filters, standalone app.

---

## 6. Hard rules

- **No new operation templates**; do not extend `query.go`'s keyword ladder.
- **No model roulette** — no evaluating, benchmarking, downloading or swapping LLMs except via
  a decision rule written down BEFORE the run and surfaced to the user.
- **No embeddings on structural decisions** (family, group, measure, field role).
- **No reports without a measurement.**
- **Abstention is a success.** A confident wrong answer never is.
- **Never fabricate** a result, citation, OCR text, transcript, sighting, face identity,
  location, timeline or cross-family relationship.
- **Never merge upstream LocalAI into this repo.** Clone to a sibling directory to compare.
- **Every behavioural change is measured on the 62 against a pre-declared threshold**, and
  reverted if it fires. This has fired twice and was honoured both times.

---

## 7. Operational hazards

- **Auth fails closed.** Needs `FORENSIC_RECORDS_API_KEY` from `.env.forensic-runtime.local`.
  **Never "fix" a crash-loop by disabling auth** — on 2026-09-17 a whole session served the
  forensic API unauthenticated on real evidence.
- **Set env from PowerShell, never Git Bash** (MSYS rewrote `/data/...` to
  `C:/Program Files/Git/data/...`). **Postgres is host port 5433, db `localrecall`.**
- **`docker compose build` is unusable for LocalAI** (uploads the repo, wedges the daemon);
  the forensic API builds fine (~7 s to distroless). **`make` is not on PATH.**
- **An empty JSON-Schema `enum` is an unparseable grammar** — HTTP 500 before inference.
  Seed runtime enums with a constant; `semantic_dynamic_schema_test.go` walks every schema.
- **Never** `docker compose down`, `down -v`, `--remove-orphans` (would delete
  `nexusai-api-1`), `system prune`, `volume prune`; never reprocess, backfill, migrate or
  delete retained evidence. **Git:** never `reset --hard`, `clean`, `checkout --`, `restore`,
  `stash`, `merge`, `rebase`, `cherry-pick`, `pull`, `push` unless asked this session.

**Regression anchors** (SQL-verified, case `nexusai-forensic-demo`): CDR 8,642 · IPDR 2,500 ·
ANPR 750 · access log 1,000 · subscribers 11 · towers 5 · transactions 4 — **12,912 rows**.

**Live models:** synthesis `qwen3-4b-instruct-2507-q4km-nxb21d-dev` (**Q4_K_M**), embeddings
`qwen3-embedding-0.6b`. CPU only, ~4 tok/s, ~15.7 GiB RAM.

**Switches in production:** `FORENSIC_VERIFIED_ONLY=true` · `FORENSIC_IR_ARBITRATION=true` ·
`FORENSIC_IR_FALLBACK=true` · `FORENSIC_PLAN_CACHE=true` with
`FORENSIC_PLAN_CACHE_DIR=/data/forensic/spool/plan-cache` · `FORENSIC_IR_SHADOW=false` ·
`FORENSIC_IR_CROSSCHECK=false` · `FORENSIC_DROP_INVENTED_FILTERS=false` (measured, rejected).

Latest rollback image: `nexusai-forensic-records-api:rollback-before-goalclass-20260925`.

---

## 8. Ask before

Container rebuild or redeploy · any `docker compose` action beyond `ps`/`logs` · any database
write, migration or backfill · model downloads or backend installs · any git operation that
writes · long builds · running the live golden suite · anything touching retained evidence.

---

## 9. Next — RE-BASED 2026-09-25 on what actually bounds OPEN-ENDED questions

**The 62 measure regression, not capability — now PROVEN, not argued** (held-out: 7 WRONG of
13). Four things bound open-ended answering, in order of how much they bind:

**(0) THE ANSWER PATH ITSELF, on unseen phrasings.** The held-out failures are not coverage
gaps — H7 dropped a filter (D1's signature, still live on the ladder path), H2 asserted a
false absence, H12 answered a different field than the one asked about, H1 counted rows
instead of distinct values. **These are correctness defects that the 62 never exposed.** Fix
these before anything else; a wider vocabulary on a path that substitutes fields makes the
product more confidently wrong, not more useful.

**(1) VOCABULARY — the real ceiling, now measured.** `semantic_coverage_audit_test.go`
compares curated `source_names` against the columns actually present (read-only, no model).
On `nexusai-forensic-demo`: **81 of 94 columns curated, 86%.** A column the layer does not
describe is never issued in the enum, so no verified plan can reference it — correct and
safe, and exactly the ceiling. **Coverage of the layer IS coverage of the product**, and the
golden suite cannot see it. The concentrated gaps:

    cdr            11/16  location(8642) Site_Id(8638) Lac_Id(8638) lat(8638) longitude(8638)
    tower_location  8/15  Azimuth Beam-Width TAC Sector-ID Uncertainty-Radius-M
                          Coordinate-Datum Updated-At
    subscriber     31/32  location(1)

**EVERY CDR row carries a location and no question about it can be answered.** "Where was this
number seen", on the largest family in the case, is structurally unanswerable today. That is a
CURATION gap (Codex's `semantic_layer/**`), not a compiler, model or gate problem.

**(2) SCOPE.** Slice 1 proved the ladder is the product's main source of family/record-type
binding, not merely a router. Deleting it requires deriving scope from the layer first.

**(3) MEASUREMENT.** We have never asked an out-of-corpus question. Build a held-out set —
questions written against the DATA, not against the templates — and score it. Until then
"can it answer any question" is untested, in either direction.

**Order of work:** fix the answer path (0) → close vocabulary gaps (1) → derive scope from
the layer (2) → delete the ladder → hold-out measurement (3) throughout, not at the end.
**Extend the held-out set as the primary gate**; the 62 become a regression check only. The
breakdown taxonomy and P4 identity remain open but neither touches the ceiling above.

**Start here:** `docs/work/DEEP_RECONCILIATION_PROMPT.md` — a full reconciliation and roadmap
covering the answer path, all question classes (including greetings, product help and
out-of-scope, which are unwired), the media pipeline (FOUR ANPR versions; which is live is
unknown), evidence lifecycle, identity, UI and deployment staging.

**Codex track:** WI-UI-7 complete locally, awaiting product-owner review of
`design-review/wi-ui-7/`, then a separate deployment/auth decision.

**Read more:** `docs/work/MASTER_EXECUTION_PROMPT.md` (what to implement) ·
`docs/ux/NEXUSAI_PRODUCT_UX.md` (binding UI contract) ·
`reports/nexusai-tl-audit-20260918/P0-baseline-report.md` (baseline defects) ·
`reports/tier-decision-20260924/` (the five decision rules and their outcomes).

**Phase 0 debt:** reconstruct the LocalAI v4.5.6 baseline in a sibling clone →
`docs/integration/FORK_DELTA_v4.5.6.md`.

---

## CODEX_UI_TRACK

- **ACTIVE_WORK_ITEM:** WI-UI-9 / WI-LAYER-1 COMPLETE locally 2026-09-25. Changes are confined to `semantic_layer/**`, `apps/investigation-workspace/**`, and this section. `api/**`, retained database state, evidence, containers, and deployment were not touched.
- **Curated answerable surface:** demo coverage moved from 81/94 (86%) to 93/94 (99%). Per family: access log 6/6→6/6; ANPR 8/8→8/8; CDR 11/16→16/16; IPDR 8/8→8/8; subscriber 31/32→32/32; tower/location 8/15→14/15; transaction 9/9→9/9. The multimodal acceptance collection measures 69/70 (99%). Against the all-collection inventory, aliases now cover 146/148 columns (99%), up from 88/148 (59%).
- **Curation scope:** added observed CDR location/site/LAC/coordinate aliases and alternate schema spellings; ANPR derived-observation provenance, confidence, plate/camera/source aliases; tower aliases plus site/sector/operator/coverage/azimuth/beam-width/datum/uncertainty/update fields; IPDR session/port/traffic/URL/cell/location/timezone fields; subscriber account type, masked CNIC suffix, and location alias. Numeric measures are typed NUMBER; identifier-like fields advertise distinct capability; confidence fields are projectable but not analyst filters.
- **Deliberate exclusions:** `generic` / `Case notes for records-demo` remains document prose rather than a structured field. `tower_location.TAC` remains uncurated because this export does not establish the acronym's meaning. Both are documented in `semantic_layer/README.md`; no meaning was guessed.
- **UI follow-ups:** question history is collapsed by default; the three-source rail is bounded to the answer row and the result table spans the full analytical canvas below it; the Ask surface is lighter; derived-artifact lineage now shows the recorded producer and producer version matched through `run_id`, or explicit `Producer not recorded` / `Version not recorded`. Model/backend identity remains absent everywhere else, as UX §9 requires.
- **Verification:** `TestSemanticLayerCoverage` passes for demo and multimodal collections; `TestWI4`, `TestFamilyCatalogAudit` (zero starved enums; only the intentionally wrong PII gold SUB-03 remains unanswerable), and the full `api/forensic_records` Go package suite pass. UI ported 46/46; unit 75/75 across 14 files including contrast and lineage; Playwright 76 passed, 36 intentional per-project skips, 0 failures across 112 checks; production build passes with 168 transformed modules. Updated 1440 two-theme captures were visually inspected; 375px overflow and touch-target checks remain green.
- **Honest endpoint gaps:** no login/session endpoint; no cursor-paginated full structured-evidence endpoint beyond `records_preview`; no guaranteed normalized page-text, image-region, timed transcript/diarization, or video-event endpoint; no case entity/server-side pinned-question/history endpoint. Lineage displays producer/version only when the existing evidence-detail payload records them in `processing_runs` or artifact metadata.
- **Not deployed/activated:** WI-UI-9 is source-complete and locally verified only. The backend track owns the coordinated live answer evaluation and any deployment. Exact next action: backend measures the curated layer against the held-out/live gate, then the product owner separately decides deployment.
