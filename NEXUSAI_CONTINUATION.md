# NexusAI — Current State

> **CURRENT STATE ONLY. Rewrite it; never append. Hard cap 300 lines.** History in
> `reports/archive/`: `continuation-20260925.md` (WI-9..WI-31) · `continuation-wi0-wi8-20260923.md`
> · `continuation-20260921.md`. **TWO AGENTS WRITE THIS FILE:** everything above
> `## CODEX_UI_TRACK` is the backend track, that section is Codex's, neither edits the other's.

**Last updated:** 2026-09-26 · **Branch:** `codex/forensic-hybrid-checkpoint-20260723`

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
   one self-correction → parameterized SQL → verify → **abstain, never guess**. Build on `SourceNativePlanV1`.
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
- **Every behavioural change is measured against a pre-declared threshold**, and reverted if it fires. **Fired
  three times, honoured three times** — most recently Step 4, 2026-09-26 (§7).

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

**Switches — ASSERTED FROM THE CONTAINER 2026-09-26 after the arm C ship**, and all of them now PERSISTED in
`.env.forensic-runtime.local` AND declared in compose: `VERIFIED_ONLY=true` · `IR_ARBITRATION=true` ·
`IR_FALLBACK=true` · `PLAN_CACHE=true` with `PLAN_CACHE_DIR=/data/forensic/spool/plan-cache` ·
`ANSWER_STATES_VALUES=true` · `MEASURE_DENOMINATOR=true` · `LADDER_ROUTING=true` (**defaults ON** — it gates
EXISTING behaviour; off cost 3 confident-wrong) · **`MEDIA_FAMILY_ROUTING=true` ·
`DERIVED_ARTIFACT_EXECUTION=true` · `BOOLEAN_RESTRICTION=true` (SHIPPED, §7)** · `BREAKDOWN_GOAL=false` ·
`IR_SHADOW=false` · `IR_CROSSCHECK=false` · `DROP_INVENTED_FILTERS=false` (measured, rejected). **A SWITCH MUST
BE DECLARED IN COMPOSE, NOT JUST SET IN THE SHELL — compose forwards only what it names**, and an undeclared one
measures the feature silently OFF (§7).

Rollback images: `rollback-before-step4b-20260926` · `rollback-before-step4-20260926`.

**ASK THE PRODUCT OWNER FIRST:** container rebuild or redeploy · any `docker compose` action beyond `ps`/`logs`
· any database write, migration or backfill · model downloads or backend installs · any git operation that
writes · long builds · running a live evaluation suite · anything touching retained evidence.

## 7. Next — RE-BASED 2026-09-26

    (0) ANSWER PATH 7 WRONG->1 · (1) VOCABULARY 86%->99%, routing 7/28->15/28   CLOSED
    (2) MEDIA ANSWERING   SHIPPED  5 CORRECT/7 WRONG -> 13/3 (arm C)
    (3) SCOPE bound by the ladder   OPEN · (4) HELD-OUT 13+28, needs >=40   PARTIAL

**BREAKDOWN GOAL — BUILT, MEASURED, NEUTRAL, OFF 2026-09-25**, behind `FORENSIC_BREAKDOWN_GOAL`, registered at
all five sites a new goal needs: 62/62 identical verdicts, all four targets BYTE-IDENTICAL. **The ladder answers
them first** (`query.go:2418` keys on "breakdown"), so the goal is correct and UNREACHABLE. **THIS REVERSES THE
ORDERING** — it cannot pay until the ladder stops answering them, but it stays a PREREQUISITE (it empties 4 of
the 17 `lookup` questions, and an S9 `rows` case cannot be added while that bucket holds six kinds of question).
Measure it TOGETHER with the `rows` case, and **RE-MEASURE: its neutral verdict was scored against a control
that over-credited four of its targets.** **STAGE 0 DONE 2026-09-25** behind `FORENSIC_ANSWER_STATES_VALUES`,
+4, ON. **Left behind:** CDR-10 and ANPR-03 stay NOT_STATED because REGISTERED TEMPLATES answer them, so
`sourceNativeResultAnswer` never runs — `tabularResultAnswer` needs the same treatment.

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

**THE LADDER'S PII PATH — FOUND AND GUARDED. The boundary covers the COMPILER, not the LADDER:**
`CatalogFields` drops PII/RESTRICTED so no typed plan can name those fields (proven at every goal), while
`derivedTextEvidenceSQL` reads `raw_text`, `normalized_text`, `text`, `roman_urdu_text` and `raw_urdu_text`
straight out of the JSON. **I caused the discovery:** fixing H2's misclassification sent it to
`audio_transcript_search`, which returned the actual Urdu transcript. Reverted the same day. **The lesson was
already written down and I still walked into it — a census of the CLASSIFIER said nothing about where the
reclassified question would LAND.**

**The defect is UNTARGETED BROWSE, not search.** `derived_text_relevance.go:83` judges every passage relevant
when a question names nothing to filter on, so all come back IN FULL. **AUD-03 showing an Urdu snippet is
CORRECT** — the analyst supplied the identifier and the snippet IS the citation; AUD-01/02, DOC-02/03 and IMG-01
are the same, all CORRECT in the 62. `FORENSIC_TEXT_BROWSE_GUARD` (**default ON**, declared in compose AND
persisted) withholds text for untargeted requests, keeps count/files/languages/timestamps/locators and **says it
withheld them**. Census first: **1 question, H2, zero structured.** **Live probe, same template and route:
untargeted -> ZERO Urdu anywhere in the payload; targeted -> snippet intact. Golden 62: ZERO transitions.**

**EMPTY 200s FIXED.** All FOUR terminal classes returned HTTP 200 with NO analyst-facing text:
`terminalRequestResponse` filled `resp.Answer` and never built `resp.Enterprise`, where the analyst surface and
the grader both read — **the server produced an answer and dropped it**; H2 and M15 went 0 -> 243 chars. It does
NOT use `finalizeEnterprisePayload`, which asserts an EVIDENCE shape these classes lack. **Those four classes
ARE the unwired conversational surface (§7 item 5), so this is its groundwork.**

**Remaining, in order:**

1. **PII MASKING — the largest capability gap, and one part is CODEX'S to rule on.** The browse guard is the
   *fallback* (withhold the whole text); the masking itself is owed, and 7 curated media fields stay withheld
   until it lands. Ruling ACCEPTED: case-scoped stable alias (`Plate candidate ••••-A7C2`), **NOT last-four** —
   plate strings are LOW ENTROPY, so last-four both reveals too much and collides; typed placeholders in
   OCR/transcript text; **if token-level redaction cannot be PROVEN, withhold the text** and keep counts,
   timestamps, language, review state and locators. **`audio_roman_urdu_segment` holds the SAME utterance in two
   scripts** — redacting a number in one and not the other discloses it anyway, and synchronising depends on
   whether the Roman form is a deterministic transliteration and whether digits are Urdu-Indic in one and ASCII
   in the other. Asked of Codex FROM THE DATA in
   [`WI-LAYER-7_CODEX_PROMPT.md`](docs/work/WI-LAYER-7_CODEX_PROMPT.md), with "cannot be synchronised safely"
   named as a valid answer. **H2/M8/M15 stay on the terminal path until it lands** —
   `TestClassifierSuperlativeBoundary` keeps `earliest|latest` ABSENT so they are not re-added by accident.
2. **M4 "highest plate detection confidence" went CORRECT -> CLARIFIED** when media routing landed. A lost
   answer, not a wrong one, so it did not fire a threshold — but it is unexplained and worth one trace.
3. **Derive SCOPE from the curated layer, then delete the ladder in slices** — it blocks the taxonomy work as
   well as Phase 3's remaining deliverable.
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

- **WI-LAYER-7 COMPLETE locally 2026-09-26:** both plate fields remain `PII/MASKED` with one opaque case-scoped alias across families; OCR raw/normalized text, timestamp transcript text, and both Urdu representations are now `PII/WITHHELD` for typed projection and untargeted browse. Targeted cited retrieval remains a separate allowed path. No `api/**`, database, deployment, container configuration or live evaluation was changed.
- **Data ruling:** the aggregate-only oracle `semantic_layer/audits/wi-layer-7-pii-synchronization-audit.sql` found 307+24 populated plate candidates; one phone-like source token repeated across timestamp/raw-Urdu/Roman text; 22 raw + 2 normalized OCR rows with non-ASCII digits; and zero pattern matches for CNIC, email, IPv4 or Pakistan IBAN. Free text is still withheld because names, addresses and contextual identifiers are not exhaustively classifiable by syntax.
- **Urdu linkage:** all 7 derivatives carry raw+Roman text, the expected deterministic processor/revision and identifier-preservation pass; one row has ASCII digits in both, none has Arabic-Indic/Persian digits. Standalone parent linkage is 6/6 exact; the video derivative is 0/1 by parent ID but has exactly one same-scope text/time/locator match, so parent-ID synchronization is not reliable for all stored rows.
- **Verification:** requested `TestWI4`, `TestMediaPII|TestTextBrowse|TestTargetedSearches`, and DB-backed `TestSemanticLayerCoverage` all PASS; coverage remains 93/94 (99%). Direct Roman-Urdu function checks pass. An extra ingestion unittest run has one unrelated existing red case: its `language='ur'` fixture supplies Latin `bounded`, which the current trusted-Urdu guard rejects (1.27 s run); no out-of-scope ingestion behavior was changed.
