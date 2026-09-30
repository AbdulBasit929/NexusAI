# NexusAI — WI-0 through WI-8 (archived 2026-09-23)

Moved out of NEXUSAI_CONTINUATION.md to keep it under its 300-line cap. Nothing here is
superseded; the linked reports hold the measurements. The **do-not-retry** findings from
these items were deliberately LEFT in the continuation file, because their whole purpose is
to stop the next agent repeating a measured dead end.

### Completed work — detail in the linked reports

**WI-0** IR spike, 86 live calls ([`RESULTS.md`](reports/ir-spike-20260921/RESULTS.md)):
Arm A **74.3%** execution-equivalent, **5.7%** confident-wrong, **0 malformed**. **KEYSTONE
HOLDS** — 0 plans named a field outside the issued enum, so a hallucinated field is
structurally impossible.

**WI-1** (D1) extracted filters reach `plan.Filters` by exact name/alias; an unbound literal
forces `CLARIFY` naming it. **WI-2** the direct answer is the computed result. **WI-3** (D4)
family guard applied **after** ranking (filtering first collapsed the BM25 margin
0.760 → 0.028). **WI-4** curated layer: 7 entities, 83 source_names SQL-verified.
**WI-5** (items 0-5; item 6 / D8 open) — `reports/nexusai-wi5-items12-20260921/`. Fixed
ACC-04 · SUB-02 · CDR-13 · CDR-06 · ACC-03. Built: curated layer IS the catalogue · value
literals bind as filters · several values of one field mean **group by** · exact-synonym
resolution · null-keyed rankings refused · ladder family guard on `FamilyID`.

### WI-6 / WI-7 / WI-8 — 2026-09-22 (detail in the linked reports)

**WI-6** `reports/nexusai-s9-20260922/` — the INSTRUMENT was fixed first; three runs were
mis-measured. The harness needs opt-in `--analyst-text`: **anything graded without that flag
is NOT comparable.** S9 SHAPE/SCOPE added: a ranking must be grouped *and* ordered, a scalar
must not acquire a grouping, a breakdown must have one, a filtered COUNT equal to the scope
total is refused.

**WI-7** [`RESULTS.md`](reports/golden-ir-20260922-fixed/RESULTS.md) — plan-exact **16% ->
68%**. Six defects, **all ours**, found OFFLINE by `family_catalog_audit_test.go`, which
replays every golden question against real data in ~25 s with no model calls. **Run it
before any live run.** The one worth remembering: **curated NUMBER never mapped to DECIMAL**,
so `MAX` was lexicographic and 9,200 beat 75,000 — misdiagnosed for weeks as model
instability. *Rejected after measuring:* a layer-vocabulary family resolver fixed CDR-07/10
and ANPR-03 but misrouted 6 negative/media questions.

**WI-8** [`RESULTS.md`](reports/golden-ir-20260922-arbitration/RESULTS.md) — **IMPROVED 3 ·
REGRESSED 0.** Arbitration replaces a CLARIFICATION with a re-verified plan. Safe by scope:
a clarification asserts nothing, so it cannot turn a correct answer wrong. **Arbitrating
CONFIDENT answers was measured first and REJECTED** — 3 wrong / 2 CORRECT.



---

### WI-9 — verified-only + rescue: structured side at ZERO, 2026-09-23

Evidence: [`RESULTS.md`](reports/golden-ir-20260923-final/RESULTS.md).
`FORENSIC_IR_FALLBACK` · `FORENSIC_IR_ARBITRATION` · `FORENSIC_VERIFIED_ONLY` all **true**.

    confident-wrong      23 -> 5      correct-or-clarified  61% -> 90%
    CORRECT 42 · WRONG 5 · CLARIFIED 14 · ERROR 0 · PRESENT_NOT_TOP 0

**All 5 remaining are media/document/audio/cross-family** — CASE-01, DOC-04, IMG-04,
AUD-02, X-01. **STRUCTURED analytics is at 0** (was 13). CDR alone went 6 -> 0.

A structured question is answered from a plan that passed S6/S9/CONSTRAINT_APPLIED or not at
all; when an unverified route is about to answer, arbitration first tries to EARN a verified
plan. **14 questions now answer from a GENERATED PLAN, not a template.**

**Rejected after measuring, do not retry:** cross-verification (unable to judge template
answers — they expose no single comparable value; `FORENSIC_IR_CROSSCHECK` is inert);
overriding confident answers on a shape mismatch (3 wrong / 2 CORRECT); withholding every
unverified structured answer (4 wrong / 3 CORRECT lost).

**Four defects fixed:** `"?"` bound as a source-file filter · media questions answered by
structured templates · COUNT_DISTINCT made a WRONG plan possible where the question used to
clarify safely (**a new capability must arrive with the check that bounds it** — now an S9
obligation) · orphaned SQL parameters in derived metrics, an HTTP-500 that survived because
the test asserted the SQL STRING and never ran it.



---

## 1a. The inverted gate — built, proven, and now ON

**The IR generator was never gated; it was disconnected.** The LLM operation selector is
referenced **only by tests**, and `resolveSemanticDynamicPlan` was called **only from inside
that dead function**. What WI-0 measured at 74.3% with 0 hallucinated fields had never run
in production. **Now wired:** deterministic compile (~2.5 s) → enum-constrained IR where the
compiler could not resolve → clarification. The selector stays disabled on purpose: it picks
one of ~104 opaque operation IDs that nothing can re-verify, whereas a generated plan is
re-checked by S6, S9 SHAPE and CONSTRAINT_APPLIED and **discarded** on any failure.

**Five blockers each silently defeated the whole path:** `semantic_ir_fallback` missing from
`hybridResolvedStatus`; `SourceNativeCatalog` never set; `Limit` zeroed though execution
requires `>= 1`; the legacy wrapper schema at 185 s; and `RecordType` never set, so every IR
plan ran across ALL families (12,912 rows instead of 8,642).

It shipped OFF because one question produced **75,000 then 9,200** across builds, recorded
as "field selection is not stable at this model size". **That diagnosis was wrong**: curated
NUMBER never mapped to DECIMAL, so `MAX` compared text. A type mapping. See WI-7.

**Audit:** `ir_fallback_outcome` records `accepted` or the exact check that discarded the
plan; `ir_arbitrated` marks an answer that replaced a clarification. Both required.

**Cost:** with shadow AND fallback on, an unresolved question pays for TWO generations
(167/174/219 s observed). Turn shadow off in production. P3's gate is p95 < 15 s.

---



---

### WI-10 — the last five, 2026-09-23 · DEPLOYED AND VERIFIED LIVE

Three guards in `ir_crosscheck.go` — full rationale and rejected alternatives are in the
comments there. Each was scored OFFLINE against the 62 saved responses BEFORE it was
written. Wired in `query.go` at the pre-execution gate plus a new post-execution one.

- **`answerNamesUnfilteredTarget`** (DOC-04) — the plan filtered on NOTHING while the
  sentence said "218 ANPR sightings **involving ABC-123**". `applied_filters.target` is
  recorded from EXTRACTION and is no evidence a filter ran; when a typed plan executes its
  filters are the only ground truth. 1 of 33 plan-backed, 0 correct lost.
- **`uncomputedQuantityAnswer`** (CASE-01, IMG-04) — both answered "20 records matched this
  question" where 20 is the PAGE LIMIT (CASE-01's own payload reports `total_count` 12,912;
  IMG-04's true count is 22). 2 of 62, 0 correct lost.
- **`singleFamilyNegativeClaimsCaseScope`** (X-01) — `top_locations` searched CDR locations
  only and asserted absence "in the selected case scope"; the gold shows the number DOES
  appear, in a PDF and a WAV. Falls out of declared capability metadata, no new heuristic.
  1 of 62, 0 correct lost.

**Rejected on the numbers, do not retry:** dropping the `SourceNative` exemption from the
media guard (fires on DOC-04 WRONG *and* DOC-01 CORRECT — a 1:1 trade); withholding when a
quantity question resolves to `bounded_records_table` (15 fire — 13 CORRECT, 2 WRONG).

**These ABSTAIN rather than answer, by necessity:** the offline audit puts CASE-01, IMG-04,
AUD-02 and X-01 in `no_family` — the generator never gets an enum, so a verified plan is
impossible today. Making them CORRECT is COVERAGE work, not a Phase 1 safety fix. DOC-04 is
the opposite: it *gets* a family, the wrong one (`anpr`).

**AUD-02 asserts nothing false** — "The text search is incomplete … absence of further
matches has not been established." A retrieval miss scored WRONG, not a confident-wrong
answer. Check the scoring before treating it as a defect.
**DOC-01 is a WEAK CORRECT** — check `contains ["MN1367"]`, and the answer "There are no
ANPR sightings involving MN1367" contains that string while answering about ANPR, not
documents. Do not read a future change there as a regression.

**Live result** ([`RESULTS.md`](reports/golden-ir-20260923-wi10/RESULTS.md)):
**confident-wrong 5 -> 1**, and the remaining one (AUD-02) asserts nothing false. DOC-04,
X-01, CASE-01 and IMG-04 all now CLARIFY. **Regression check: the 13 previously-CORRECT
quantity questions plus both guard controls (DOC-01, DOC-06) were probed live — 15 of 15
preserved, 0 regressions.** 20 live probes, 0 HTTP errors.

Tests: `wi10_guards_test.go` (9, each with its control). Full package suite green (115 s).
Offline audit clean against real data: 0 catalog errors, 0 starved.

**NOT YET DONE: the full 62 has not been re-run since these guards landed.** The other 42
are untouched by all three guards by construction, but only a full run makes that a
measurement. Rollback image `nexusai-forensic-records-api:rollback-before-wi10-20260923`.

### WI-11 — clarification options, 2026-09-23 · DEPLOYED AND VERIFIED LIVE

Evidence: [`RESULTS.md`](reports/clarification-options-20260923/RESULTS.md).
**`clarification.options` was always `[]`.** 18 of 62 questions now clarify (14 + WI-10's 4),
so nearly a third of all traffic ended in a dead end: told the question cannot be answered
and handed nothing to do about it. UX §7 requires 2-4 re-runnable choices.

**CONTRACT CHANGE — `forensics.clarification-request/v2`.** `options` is now
`[{label, query}]`, not `[]string`. The UI needs no phrasing logic and cannot drift from
what the compiler accepts. Wired at all five clarification sites.

Nothing is invented. Labels are curated `display_name` values; only `sensitivity: NONE`
fields are offered, so an option can never surface what the sensitive-field guard withholds;
only `groupable: true` fields become breakdowns. Media scope labels come from UX §5's own
chip vocabulary, and each media query shape is one the corpus PROVES answers today
(DOC-02, IMG-01, AUD-03).

**Three defects found by probing live, each fixed:** breakdowns offered for a media question
(DOC-04 was offered "Break down by Camera" for a question about a document) · scope options
drawn only from the 7 structured entities, so X-01 — whose answer is a PDF and a WAV — was
offered CDR counts · an option that re-asked the withheld question verbatim (IMG-04 offered
itself back, a loop).

**Click-through verified.** X-01 now clarifies with "Documents mentioning 03001234567" first,
and clicking it returns the **NexusAI Multimodal Acceptance Brief** — exactly the `.pdf` its
gold expects. A false negative became a clarification that reaches the right evidence in one
click. Tests: `clarification_options_test.go` (11). Full suite green.

