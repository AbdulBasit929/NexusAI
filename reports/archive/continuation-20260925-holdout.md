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

- **Auth fails closed.** `forensic-records-api` needs `FORENSIC_RECORDS_API_KEY` from
  `.env.forensic-runtime.local`. **Never "fix" a crash-loop by disabling auth** — on
  2026-09-17 a whole session served the forensic API unauthenticated on real evidence.
- **Set env from PowerShell, never Git Bash.** MSYS rewrote `/data/forensic/spool/plan-cache`
  to `C:/Program Files/Git/data/...`; the startup log caught it.
- **`docker compose build` is unusable** for LocalAI (uploads the repo, stalls, wedges the
  daemon). The forensic API builds fine (~7 s to distroless).
- **Postgres is on host port 5433, database `localrecall`.**
- **An empty JSON-Schema `enum` is an unparseable grammar** — HTTP 500 before inference.
  `semantic_dynamic_schema_test.go` walks every schema. Seed runtime enums with a constant.
- **Never** `docker compose down`, `down -v`, `--remove-orphans` (would delete
  `nexusai-api-1`), `system prune`, `volume prune`. Never reprocess, backfill, migrate or
  delete retained evidence.
- **Git:** never `reset --hard`, `clean`, `checkout --`, `restore`, `stash`, `merge`,
  `rebase`, `cherry-pick`, `pull`, `push` without being asked in the current session.
- **Build gates unrunnable**: `make` is not on PATH.

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

- **ACTIVE_WORK_ITEM:** WI-UI-7 — UX-contract conformance COMPLETE locally 2026-09-25. Owned changes are in `apps/investigation-workspace/**`, `semantic_layer/{cdr,anpr}.yaml`, `semantic_layer/README.md`, `scripts/ir_spike/gold_plans.json`, and this section. `api/**`, database state, retained evidence, containers, and deployment were not changed.
- **Provenance:** numeric/date/identifier claims receive claim-local exact-locator markers; markers have no redundant brackets. Markers without an openable locator are suppressed. Hover/focus previews expose source file, locator, supplied timestamp, a supplied excerpt capped at 200 characters, proof role, and a text/icon `EvidenceStrengthBadge`. Three or more distinct sources produce a rail; one or two remain inline. Missing timestamps/excerpts stay explicitly unavailable.
- **Shell and identity:** the global, case, and task bands are separate. NexusAI ships with a code-native mark, horizontal lockup, `/favicon.svg`, runtime customer identity overrides, Dashboard/Cases/Activity/Settings navigation, capability-gated Admin, a persistent collapsible rail at 1024/1440, and a trapped modal drawer below 1024 with Escape and focus return. Case switching changes the URL and clears incompatible `nexusai.case.*` session state first. Unsupported tenant/user/role/classification fields are absent.
- **Transport:** all analyst requests use the same-origin `/api` proxy; browser-direct credential mode and every fixture token were removed. Absolute and protocol-relative API overrides fail closed with `CFG-PROXY`. The browser never receives an API credential.
- **Scopes and states:** exact evidence-family chips persist per session on Evidence and Investigate; an empty or unsupported scope fails loudly and is never widened. Curated semantic-layer labels are used when present; otherwise raw identifiers remain unchanged. Every route exposes loading, ready, empty, partial, error, forbidden, and unavailable fixtures. Timeline's inert disabled filters were removed. Partial Activity retains completed artifacts and names the failed scope.
- **Answer and overview:** finding hierarchy emphasizes the measured quantity rather than enlarging the whole sentence. “How this was derived” is a real collapsed disclosure. Overview uses a real case title, one primary evidence metric, and concrete data-quality sentences. Theme selection lives under account/preferences, survives reload, falls back to `prefers-color-scheme`, and remains usable when storage throws.
- **Palette and typography:** governed blue `#2563EB`, teal `#14B8A6`, navy/charcoal canvases, `--analyst-surface-0..3`, `--analyst-surface-ask`, focus, status, and evidence-strength roles replace the undocumented petrol/cyan palette. Noto Sans / Arabic / Mono remain deliberate because Geist lacks Arabic coverage and the corpus contains mixed-script evidence such as `کال 03001234567 at 1035`.
- **Measured contrast:** Daylight minimum text 4.51:1 and component/focus 4.16:1; Operations minimum text 6.35:1 and component/focus 5.20:1. `tokens.vitest.js` enforces all governed roles and thresholds.
- **Semantic correction:** CDR-08 now has `COUNT_DISTINCT(cdr.msisdn)` gold; ANPR-05 has `COUNT_DISTINCT(anpr.plate_number)` gold. The inert `cdr.distinct_subscribers` and `anpr.distinct_plates` declarations were deleted. SUB-03 was intentionally not changed because its literal-CNIC gold conflicts with server-side masking. `go test ./api/forensic_records -run TestBucketAGoldPlanConformance -count=1` passes.
- **Visual evidence:** 68 PNGs in `apps/investigation-workspace/design-review/wi-ui-7/`: the eight requested surfaces at 375/820/1024/1440 in both themes, plus drawer-open, rail-collapsed, claim-level close-up, and all-route-state fixtures. The 375px captures have no horizontal page scroll, retain NexusAI identity, and use 44px mobile targets; the 768–1279 table range pins the first column.
- **Verification:** ported 46/46; unit 67/67 across 10 files; Playwright 46 passed + 30 intentional project/owner skips, covering clarification, zero, unsupported, drawer focus trap/return, and the core loop; production build passes with 162 transformed modules.
- **Missing endpoints kept honest:** no case entity/create-case endpoint (the first accepted file creates the collection); no tenant/user/role/classification source; no cross-case attention/assignment feed; no global/team audit history; no cross-family timeline event feed; no family-scoped Documents/Images/Audio/Video query without an evidence ID; no guaranteed citation excerpt/timestamp; no authorized admin summary. The corresponding fields, counts, events, and controls are absent or explicitly unavailable rather than fabricated.
- **Not deployed/activated:** WI-UI-7 is implemented and reviewable locally only. Production activation still requires an explicit deployment decision and an owned identity/session layer in front of the same-origin proxy. Exact next action: product-owner review of `design-review/wi-ui-7/`, then authorize the deployment/auth work separately.
