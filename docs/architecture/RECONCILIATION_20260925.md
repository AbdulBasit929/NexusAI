# NexusAI — Reconciliation & Roadmap, 2026-09-25

Scope: §3.1–3.10 of `docs/work/DEEP_RECONCILIATION_PROMPT.md`.
Every claim is labelled **[MEASURED]** (I ran it), **[SOURCE]** (read in this
repo), **[INFERENCE]** (my reasoning), or **[UNKNOWN]** (not established).

**The headline, and it reframes everything below.** The 62-question golden suite
reports 0 confident-wrong. A held-out set of 13 questions written against the
DATA — SQL-verified, phrasings nobody wrote for this system — returns **7 WRONG**
[MEASURED, `reports/holdout-20260925/`]. The suite measures regression on
questions the system was built around. **It does not measure capability, and no
number from it should be quoted as if it did.**

---

## 1. §3.1 — the query answering path

**Works** [MEASURED]: a generated typed plan answers 39 of 62 questions
(`scripts/nexusai_route_census.py`); S6/S9 verification and abstention hold; the
keystone (enum → GBNF alternation) is real on the serving path
(`grammar_keystone_test.go` asserts it through the JSON round-trip).

**Four defect classes, each root-caused from the recorded plan** [MEASURED]:

**H7 — "how many server errors in the access log" → 1,000 (truth 46).**
Plan: `COUNT` with **`filters: null`**. No literal to extract, so no
CONSTRAINT_APPLIED obligation, so no abstention — the unfiltered total was
stated as the answer. D1's signature, on the compiler path. The system does not
know "server error" means `status = 500`.

**H1 — "how many different handsets" → 8,642 (truth 4,997).**
Same shape: `COUNT`, `filters: null`. The `distinct` goal never fired because
the vocabulary is `distinct`/`unique` and "different" is absent.

**H12 — "average BEAM WIDTH of the towers" → "the average AZIMUTH is 209.97".**
Plan: `AVG` over **`fld_0355ecbda8251b2bdc2c3462`** — a *hashed, uncurated*
field. Retrieval fills spare enum slots with uncurated columns, which carry no
description, so the generator picked one that sounded plausible. **The keystone
held — the field was in the enum — but an enum containing undescribed fields is
a licence to guess.**

**H2 — "how many VoLTE calls" → "there are NO CDR records involving VOLTE"
(truth 739).** The plan is **correct**: `cdr.call_type EQ "VOLTE"`, a value the
layer declares, and SQL confirms 739 matching rows. It executed over **0
authorized source rows**. **Root cause NOT established** [UNKNOWN]; scope /
record-type binding is the prime suspect. **This is the most serious defect in
the product: a verified plan asserting false absence.**

**The generalisable finding** [INFERENCE]: the guards bound *plans*, not
*questions*. A plan can be internally valid, pass S6/S9, and still answer a
different question — H12 the wrong field, H7 the wrong scope. This is the DOC-01
lesson at a new level: *verification proves a plan against itself, never against
the question it answers.*

## 2. §3.2 — the non-analytical classes

Probed live with `scripts/nexusai_probe.py` [MEASURED]:

| Question | Class | Result |
|---|---|---|
| "what is an IMSI?" | `GENERAL_DOMAIN_KNOWLEDGE` | **Correct.** Definition plus *"This definition does not assert anything about the current case."* |
| "what is the suspect's blood type?" | `GENERAL_DOMAIN_KNOWLEDGE` | **Safe.** *"A bounded general definition is unavailable... makes no statement about the current case."* |
| "how do I add evidence?" | `PRODUCT_HELP` | **Safe and grounded** in the real registry. But the prose is meta — it describes where help comes from rather than telling the analyst what to do. |
| **"hi"** | `GOVERNED_EVIDENCE_ANALYSIS` | **Wrong.** *"I could not map that request to a deterministic forensic workflow. Please name the evidence family..."* |
| **"thanks"** | `GOVERNED_EVIDENCE_ANALYSIS` | Same. |
| **"what can this system do?"** | `GOVERNED_EVIDENCE_ANALYSIS` | **Wrong** — `PRODUCT_HELP` exists and would have answered it. |

**The gap is CLASSIFICATION, not capability** [INFERENCE]. Two of the four
classes are wired, grounded and safe. There is no greeting class at all, and
capability questions never reach `PRODUCT_HELP`. Cheap to fix, and it is the
first thing a new user types.

**Instrument note** [MEASURED]: my first probe reported "(no analyst-facing
text)" for both working classes because it read the wrong key. The probe was the
defect; fixed, with the reason recorded in the script.

## 3. §3.3 — media and derived artifacts

**Queryable today** [MEASURED, from the 62]: documents (DOC-02/03/06 CORRECT),
image OCR (IMG-01 CORRECT), audio transcripts (AUD-01/02/03 CORRECT). Video
(VID-01) clarifies.

**Dead code confirmed** [MEASURED]: `video_anpr_product_v21.py` (115 lines) and
`video_anpr_product_v22.py` (156 lines) have **zero references anywhere in
`ingestion/`**. `media_pipeline.py` imports only `video_anpr_product_v2` and
`video_anpr_onnx_v3`, selected at runtime; `video_anpr_parity_adapter` is used
by tests.

**Not established** [UNKNOWN]: whether OCR/ANPR confidence is calibrated or
noise (the 2026-09-18 baseline recorded photo OCR storing noise at 0.90);
whether every derived artifact can produce an openable locator; whether v2 vs
onnx_v3 is selected deterministically.

## 4. §3.5 — the semantic layer, and the substitution vector

**Coverage** [MEASURED, `semantic_coverage_audit_test.go`]: 88/148 columns
across all collections (59%); 81/94 on the demo collection (86%). Detail in
`docs/work/CURATION_GAP_INVENTORY.md`.

**Every CDR row carries a `location` and no question about it can be answered** —
`location`, `Site_Id`, `Lac_Id`, `lat`, `longitude`, each ~8,640 rows.

**H12 shows the cost of the other half** [INFERENCE]: uncurated columns are not
merely unanswerable — retrieval promotes *hashed* versions of them into the enum
to fill slots, and an undescribed field in the enum is how the generator
substitutes azimuth for beam width. **Curation is not only coverage; it is a
correctness control.**

Cross-family joins remain 0/5 [SOURCE] — the weakest capability.

## 5. §3.6–3.10 — carried forward, verified where cheap

- **Architecture** [SOURCE]: ~330-line core delta across 8 files; dead
  `core/services/records/` (2,188 LOC) still routed; path to a pinned unmodified
  upstream image unchanged.
- **Identity** [SOURCE]: no case entity (`case_id` 3× vs `collection_id` 224×);
  every analyst is one shared principal. **Blocks real deployment.**
- **Security** [SOURCE/MEASURED]: auth fails closed, verified; CNIC masking is
  server-side at projection; the IR is stored with the answer, so "why did the
  system answer this way" is reconstructable.
- **Operations** [MEASURED]: warm p95 5.2 s, max 5.9 s; cold p95 ~90–180 s. Any
  schema change invalidates every plan-cache key and can push a cold question
  past the planner timeout. **Silent failure mode** [SOURCE]: if grammar
  generation fails server-side, `chat.go` logs and proceeds with **no grammar at
  all** — every constraint lost, 200 response.
- **The ladder** [MEASURED]: still present (`query.go` 9,041 lines). Switching it
  off costs 3 confident-wrong and 3 correct, because it is the product's main
  source of family/record-type **scope**, not merely a router.

---

## 6. ROADMAP

Each phase: goal · work · exit gate **with numbers** · rollback · activation.
**Every gate is measured on held-out questions.** The 62 are a regression check
only, labelled as such wherever they appear.

### Phase 1 — the answer path tells the truth on unseen questions ← FIRST

1. **H2 first.** Root-cause the false negative (correct plan, 0 rows). A
   verified plan that asserts false absence is the worst outcome this product
   can produce. Nothing else ships until it is understood.
2. **Unbound-scope guard (H7).** A stated total with `filters: null`, where the
   question implies a restriction the extractor could not bind, must clarify
   rather than state the unfiltered total.
3. **Undescribed fields must not be selectable (H12).** Either exclude hashed
   uncurated columns from the issued enum, or require a description for any
   field the generator may name. Measure both; assume neither.
4. **Distinct vocabulary (H1)** — "different", and the "how many X are there"
   forms. Narrow, measurable, low risk.

**Exit gate:** held-out WRONG **7 → 0**, with ≥1 of H1/H7 converting to CORRECT
and H11/H12 to CLARIFIED; golden 62 unchanged at 0 WRONG. **Extend the held-out
set to ≥40 questions first**, so the gate is not measured on 13.
**Rollback:** each change behind its own switch, default preserving today.
**Activation:** nothing ships until the gate passes; then default-on.

### Phase 2 — classification covers every question kind

Greeting/chit-chat class; capability questions routed to `PRODUCT_HELP`;
`PRODUCT_HELP` prose made instructional rather than meta.
**Exit gate:** a 20-question intent probe set, 100% correctly classified, 0
fabricated capability. **Activation:** with Phase 1 or immediately after —
user-visible, low risk.

### Phase 3 — curation as a correctness control

`WI-LAYER-1` (Codex): every family, every column.
**Exit gate:** no family below 90% on the demo collection; H11/H12 re-score from
CLARIFIED to CORRECT; held-out WRONG stays 0.
**Dependency:** Phase 1 item 3 **must** land first — more vocabulary on a path
that substitutes fields makes the product more confidently wrong.

### Phase 4 — scope without the ladder, then delete it

Derive family/record-type scope from the curated layer; keep
`FORENSIC_LADDER_ROUTING` as the A/B instrument.
**Exit gate:** ladder OFF with 0 WRONG and 0 CORRECT lost on both suites; then
delete `choosePreciseTemplate`, `chooseGenericFallbackTemplate` and the
unreachable catalogue. **Rollback:** the switch, until the code is gone.

### Phase 5 — identity, cases, deployment

`nexus.orgs/users/sessions/cases/case_members`; RLS on membership; retire the
shared principal. **Exit gate:** two analysts on two cases cannot see each
other's evidence, proven by test. **Activation:** this is the gate for any real
deployment — the same-origin proxy is not authentication.

### Phase 6 — UI/UX to contract

`WI-UI-8` as briefed: the five §6.5 viewers, command palette and keyboard loop,
virtualised tables at 10k rows, the claim-marker readability fix.
**Exit gate:** the §13 core loop completed keyboard-only, recorded.

## 7. Measurement plan

- **Extend `evaluation/holdout_questions_v1.json` to ≥40** before Phase 1's
  gate, and to ≥100 over time. Questions written from
  `CURATION_GAP_INVENTORY.md` against columns that exist, expectations
  SQL-verified independently of the API.
- **Keep honesty probes as first-class cases** where CLARIFIED is the pass.
  H11/H12 are the model: they convert to CORRECT only when curation makes them
  answerable, so one question measures both honesty and coverage.
- **`scripts/nexusai_probe.py`** for intent classes, where the test is
  appropriateness, not a value.
- **`scripts/nexusai_route_census.py <before> <after>`** on every run; it exits
  non-zero on any lost CORRECT or new WRONG.
- **The 62 remain**, labelled *regression only*.

## 8. Missing, half-built or duplicated

- `video_anpr_product_v21.py`, `video_anpr_product_v22.py` — **dead, 271 lines**.
- `core/services/records/` — 2,188 LOC dead file-backed store, still routed,
  bypasses RLS.
- Greeting/chit-chat class — **absent**.
- `PRODUCT_HELP` — wired, but its prose is meta rather than instructional.
- Cross-family joins — declared, 0/5.
- S9 has no `rows` case; the goal taxonomy has no `breakdown` goal.
- Uncurated hashed columns in the issued enum — **the field-substitution vector**.
- `ir_shadow_*` audit fields are shadow-mode only, so `withhold_detail` always
  prints `issued_fields=0`, meaning "not recorded".

## 9. The one sentence

**The system is safe on the questions it was built for and not yet trustworthy
on the questions it was not.** Phase 1 closes that gap; every gate after it is
measured on questions nobody wrote for this system.
