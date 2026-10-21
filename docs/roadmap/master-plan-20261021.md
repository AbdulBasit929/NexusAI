# NexusAI master plan of record (2026-10-21)

**What this is.** The one document to follow. It reconciles every roadmap, continuation file, backlog, prompt, contract and report in the repository (about 80 planning documents, 25,000 lines), lists **every instruction, step and phase that is still open** with where it lands, and sequences the work: the structured and link-analysis core, the interface (redesign and the link-analysis workbench), the media families in depth, cross-family intelligence, the languages and the enterprise layer. It is written in the manner of `reconciliation-20261020.md`: findings first, rulings where documents conflict, gates written before work, decisions stated with a recommendation.

**Status:** plan; nothing in it is built by writing it. **Author:** Claude session `0554a485` for the product owner. **Written:** 2026-10-21, after the owner's request: "study, analyse, research and reconcile all the roadmaps and continuation files … nothing left … the best adjustment of UI/UX … focus on media families in depth".

**How the evidence was gathered.** Documents were read (not searched) in this order: the state file and rules (`NEXUSAI_CONTINUATION.md`, `AGENTS.md`), the newest roadmaps (`nexusai-maturity-roadmap-20261012.md` in full, `reconciliation-20261020.md`, the handoff), the September roadmaps and backlogs, the master directive's phase list (R0 to R17), the binding contracts (UX, query answering, R8 evidence), the UI redesign generations, the 26 backend requests, the licence ledger and the evidence test programme. Facts about the system were read from the running stack on 2026-10-21 (database, container environment, image tags, worker flags, routes) and from the code. Facts about outside models were searched on 2026-10-21 and are leads to test, not predictions (section 8 says which). **No real value, number or name from the owner's real files appears here.**

**Precedence (unchanged, with one addition).** Source code and live behaviour > `NEXUSAI_CONTINUATION.md` (rewritten today to match this plan) > `docs/architecture/QUERY_ANSWERING_DECISION_20261005.md` and the binding contracts (`docs/ux/NEXUSAI_PRODUCT_UX.md`, `docs/data/real-data-handling-policy.md`, the country profile) > **this file for sequencing** > `nexusai-maturity-roadmap-20261012.md` (design detail, gaps G1 to G53, decisions D1 to D22) > `reconciliation-20261020.md` (kept as the record of 2026-10-20 and 21) > everything else. This file does not change a rule of the binding documents; where it proposes to amend one (the UX contract's visual section, the ownership split) it says so and waits for the owner.

---

## 1. The answer in one page

**Where the product stands (measured, 2026-10-21).**

* **Structured data and link analysis are the strong part.** Every collection with call or internet records is typed (the real one, the synthetic Ingest v2 one, multimodal, forensic-demo, runtime-acceptance, records-demo, and the eight generated development cases); 20 provider layouts are registered; the Ask engine (model writes one SELECT over ten reviewed views, code validates, runs and checks) answers call, internet, device, cell and connection questions at **0 wrong** on fresh seeds (45 of 45, 81 of 81, 72 of 72), a connection question on the real collection in 39 s, p50 about 30 s on the CPU laptop.
* **Three things are not strong:**
  1. **The interface.** The owner dislikes the Connections, Places and link-analysis views as built, and the redesign did not converge: **three different redesign directions** exist in the repository (September teal "Identity A", the October 20 light-first Stitch plan, the October 20 dark Linear/Palantir directive), each with its own stages, and a fourth layer (my link-analysis views) sits on top. Twenty-six backend requests from the interface have 4 delivered. Section 7 resolves this.
  2. **Media.** Seven families of derived evidence exist (865 artifacts on the acceptance case) but **the worker's media roles are all switched off** today (OCR, ASR, face, image embedding, video plate reading: `FORENSIC_*_ENABLED=false`), the engine does not read derived media at all (no media view in the analyst schema), measured accuracy is weak where Pakistani evidence needs it (plates 59% exact on images and 7% on video, Urdu OCR 0 of 1, Urdu speech 45.5% word error on two clips, nothing for Punjabi or Pashto), three running components have unresolved licences and the older video-plate stack is AGPL. Section 8 is the in-depth plan.
  3. **Cross-family intelligence and the enterprise layer.** The relationship tables are empty, the identifier mention index does not exist, cross-family is 0 of 5, there is no case record, no per-user identity and no authority record. Two whole phases of the master directive (R9 general image forensics, R14 captures, databases and archives) are **not in any current roadmap**: this plan adds them (section 4.2).
* **What went wrong in process, and the fix.** Planning documents multiplied (about 80), each correct when written and several contradicting the next; the state file that every agent is told to read first is five weeks stale; two working trees hold the interface and are copied by hand; the interface has had three owners with three directions. The fix is one plan of record (this file), one state file (rewritten), one interface programme with one direction and one owner, and one repository history (section 6, P0).

**The plan in seven lines.**

| # | Programme | Weeks (estimate) | Outcome you can see | Gate to leave it |
|---|---|---|---|---|
| P0 | Consolidate | 0 to 1 | One plan, one state file, one tree, the UI direction decided | Repository history single; state file true; D23 and D24 decided |
| P1 | Finish the structured and link-analysis core | 1 to 6 | Every typed family answered by the engine; A5 at 15 minutes, A7, A9 to A11; the conversation front door; answers composed under the number lock; the case record | Bank gates on judgement cases; no regression on a fresh seed |
| P2 | Interface programme (design first) | 1 to 16, parallel | A world-class link-analysis workbench and the redesigned shell and pages, shown on generated cases | Owner accepts each stage on :4181; accessibility and keyboard checks |
| P3 | Media depth | 3 to 24, tracks in parallel | Documents and printed OCR first, then ANPR on images, audio, general image forensics, video, Urdu OCR, faces at detection only; each with a sealed set and a viewer | Per-track pre-registered thresholds |
| P4 | Cross-family intelligence | 10 to 28 | One timeline per entity across families, mention index, entity resolution, link strength, message exports and location history | Planted-truth precision and recall |
| P5 | Enterprise and release | 20 to 36 | Identity, case registry, authority, audit, backup, load and release regression | Release approval |
| PL | Languages (parallel) | 6 to 26 | Urdu, Roman Urdu, Punjabi, Pashto question sets and answers | Native-reviewed sets; needs the owner to name reviewers (D10) |

**The decisions that only you can make** are in section 11. The ones that unblock the most: **D23** (one interface direction: light-first or dark-first, decided by looking, not by reading), **D24** (one interface owner and one tree), **D25** (the media activation order, which switches the worker's media roles on one family at a time), **D30** (a minimal case record before the full identity layer, because most interface backend requests need it) and **D10** (reviewer names).

**What to do first (next 5 working days):** section 13.

---

## 2. Source register: every document, its status, and what it contributes

Status words: **ACTIVE** (binding or current), **PLAN** (sequencing, superseded here where noted), **STATE** (current-state file), **HISTORY** (kept for provenance, not a plan), **EVIDENCE** (immutable record), **SUPERSEDED** (a later document wins).

### 2.1 Rules and state

| Document | Lines | Status | Contribution and ruling |
|---|---|---|---|
| `AGENTS.md` | 88 | ACTIVE | Entry point; precedence; three rules (no templates, no model swaps outside a written rule, no embeddings for structural decisions); commit trailers (`Assisted-by`, never `Co-Authored-By` of an AI). **Updated today** to point here |
| `NEXUSAI_CONTINUATION.md` | 300 | STATE, was stale | Written 2026-09-29: described the ladder, the 70/2 posture and Phase 3 as open. **Rewritten today** (backend part) to the current state, cap 300 lines; the `CODEX_UI_TRACK` section is the interface session's and was left as it is |
| `BACKEND_ROADMAP_GAPS.md` | 78 | PLAN (interface asks) | Four more interface-to-backend requests (co-occurrence across cases, intraday density, case metadata and horizon, case notes store); merged into the request list (section 4.4) |
| `docs/work/BACKEND_REQUESTS.md` | 176 | PLAN (interface asks) | 26 numbered requests; 4 delivered (6 labels, 7 truth state, 14 reprocess, 2 connections in part); the rest open |
| `docs/architecture/QUERY_ANSWERING_DECISION_20261005.md` | 115 | ACTIVE | Three outcomes, the verifier, the case clock; amended by D17 (the model may see the analyst's own result under the number lock). **Stands** |
| `docs/architecture/RECONCILIATION_20260921.md` / `_20260925.md` | 799 / 227 | HISTORY, partly ACTIVE | §G (Governed Semantic Compiler) stands with the 10-05 amendment; the 09-25 reconciliation's phases 1 to 6 are done or replaced (section 4.1) |
| `reports/OPEN_ITEMS.md`, `reports/LESSONS.md` | 47 / 37 | EVIDENCE | The lessons bind (written down in the state file); the open items were read item by item (section 4.1) |

### 2.2 Roadmaps and plans

| Document | Lines | Status | Contribution and ruling |
|---|---|---|---|
| `docs/roadmap/nexusai-maturity-roadmap-20261012.md` | 1,524 | PLAN, design source | The most complete design: families (5), query intelligence (6), link analysis (7), data programme (8), governance (9), gaps G1 to G53 (10), phases (11), decisions D1 to D22 (12). **Stays the design reference**; its phase table is re-sequenced by this plan |
| `docs/roadmap/reconciliation-20261020.md` | 109 | PLAN (record) | S1 to S8; S1 to S5 done, S6 partly, S7 and S8 open; carried into P0 to P2 here |
| `docs/roadmap/visual-spike-findings-20261013.md` | 83 | EVIDENCE | Cytoscape.js, MapLibre GL JS, ECharts decided (D15); the spike's views are references |
| `docs/work/HANDOFF_LOCAL_20261007.md` | 172 | STATE (log) | Live images, rollback tags, tools, traps; updated today |
| `docs/work/ROADMAP_FREE_QUESTION_20261005.md` | 73 | PLAN, mostly done | R0 to R9 of the question engine; R0 to R4 done (the engine is R4 generalised), R5 retrieval and PII, R6 retire the ladder, R7 speed, R8 bake-off, R9 identity remain (section 4.1) |
| `docs/work/ROADMAP_20260929.md`, `RUNTIME_QUERY_PLAN_20260929.md`, `NEXUSAI_PROGRAMME_20260927.md`, `CODEX_BACKLOG_20260927.md`, `REMAINING_STAGES.md`, `MEDIA_QUERY_ROADMAP.md` | 178, 62, 351, 283, 130, 175 | SUPERSEDED / HISTORY | Their query-answering content is superseded by the 10-05 decision and the engine; their **open items were extracted** (section 4.1: PII masking, derived-row projection, cross-modal joins, ladder deletion, unwired question classes, E1 investigator questions, Phase C endpoints) |
| `docs/roadmap/nexusai-current-roadmap.md`, `…-phase-ledger.md`, `…-next-generation-roadmap.md`, `…-next-generation-phase-ledger.md`, `…-product-engineering-backlog.md`, `…-phase-mapping.md`, `…-r3-implementation-plan.md` | 891, 626, 1,020, 1,315, 606, 54, 50 | HISTORY | August and September logs (STIM-0 to STIM-7 closed, R0 to R7 complete, R8 paused). The **R-phase list R8 to R17 is the family sequence the master directive defines**; R9, R12 to R15 and R17 are carried into section 4.2 |
| `NEXUSAI_MASTER_DIRECTIVE.md` (3,427), `NEXUSAI_NEXT_CHAT_PROMPT.md` (1,038), `NEXUSAI_TL_DIRECTIVE_PROMPT_20260918.md` (381) | | SUPERSEDED where they conflict (AGENTS.md) | Still the source of: the phase list R0 to R17, the family expectations (19.1 to 19.12), the vertical-slice definition of done (34 items), the team-lead demonstration requirement (visible repeatable demonstrations), the specialist-agent and model-governance sections. Extracted in 4.2 and 10 |

### 2.3 Interface documents (three generations plus the contract)

| Document | Status | Contribution and ruling |
|---|---|---|
| `docs/ux/NEXUSAI_PRODUCT_UX.md` (327) | **ACTIVE, binding** | IA by case, evidence, question; provenance as the primary object; claim-level citations; state contracts (answered, clarify, zero, unsupported, partial, processing, failed); non-technical contract; accessibility; component inventory; definition of done (an unaided analyst completes the core loop). **Stands**; §8 (visual system) is the part the redesign may amend |
| `docs/work/UI_REDESIGN_MASTER_GUIDE.md`, `UI_REDESIGN_BRIEF_20260929.md`, `CODEX_UI_*`, `WI-UI-*`, `WI-LAYER-*`, `UI_PAGE_SPECS/` (September, Codex) | HISTORY, built | "Identity A" (navy-teal), stages S0 to S12, 70 vitest files and 26 e2e specs; most pages built; **this is the look the owner now wants to leave** |
| `docs/ux/redesign-2026-10/README.md`, `STITCH_PRODUCT_BRIEF.md` (October 20, main checkout, uncommitted) | PLAN, Stage 0 done | Light-first "editorial calm" redesign with Google Stitch as an exploration tool, stages 1 to 11, decisions owed (theme, personality, type, brand, branch, density) |
| `docs/work/UI_UX_MASTER_REDESIGN_PROMPT.md` (October 20, uncommitted) and the `CODEX_UI_TRACK` section | PLAN, Stages 0, 1 and 3 reported done | Dark Linear/Vercel/Palantir directive: layered charcoal canvas, monochrome plus one accent, tri-pane synchronisation, pivot-and-probe drawers, strict one-section-at-a-time law; Stage 4 (Ask) next |
| My link-analysis views (Connections, Places, result views, data panel) | BUILT, disliked | See section 7.1 |

**Ruling:** the three generations are **one programme from now on** (section 7.2), with the UX contract binding it and the dark directive's useful laws (one section at a time, real data only, backend gaps logged, live verification) kept as working rules, not as a visual direction.

### 2.4 Data, media and compliance

| Document | Status | Contribution |
|---|---|---|
| `docs/data/*` (ask-engine-v2, analysis-views, analyst-views, layout-registry, ingest-v2, typed-family-switch, case-generator-spec, real-data policy, d2 runbook, offline-basemap) | ACTIVE records | Designs, gates and measurements of the structured line (all current) |
| `docs/design/nexusai-r8-anpr-image-ocr-intelligence-contract.md` | ACTIVE for R8 | ANPR slices R8.1 to R8.10, two-boundary gate (A engineering, B production); R8.1 to R8.4 done, **Boundary A blocked** (detector precision 0.61, OCR below thresholds, no real T3 data) |
| `docs/data/nexusai-evidence-test-and-demo-program.md` | ACTIVE | Tiers T0 (exact) to T4 (authorised real); the 96-image T2-V pack exists (56/20/20 development, validation, sealed) |
| `docs/design/nexusai-r7-…`, `…-structured-intelligence-maturity.md`, `…-family-adapter-agent-api-architecture.md`, `nexusai-apf3-…` | HISTORY / partly ACTIVE | Telecom contract (R7 complete), STIM (closed), adapter architecture (superseded where it conflicts) |
| `docs/compliance/licence-ledger.md` (+csv, 83 rows) | ACTIVE | 18 approved, 34 conditional, 8 blocked, 23 unverified; three running components unresolved (YOLOv9-t plate weights, CCT-XS v2 plate reader weights, SFace); the AGPL video-plate stack off |
| `docs/research/*`, `docs/work/MODEL_AND_ARCHITECTURE_RESEARCH_20261002.md` | HISTORY | Hardware, model and fine-tuning research (laptop: 5.5 tokens a second writing) |
| `docs/demo/*`, `docs/work/DEMO_*` | HISTORY / reusable | Demonstration scripts; the team lead's rule is **visible, repeatable demonstrations** |

---

## 3. Current truth (read 2026-10-21)

### 3.1 The live system

| Part | State |
|---|---|
| API | `nexusai-forensic-records-api` image `6935d14271c7`; rollbacks `before-s6-20261021` = `77ec230274a2`, `before-020-20261021` = `ee276c1f9bcd`, then back to `before-ask-v2-20261015`; built as a host-built binary layered on the previous image (the in-container module download fails on this network); recreate **only** with the Run-Arms helpers or `recreate_api_askv2.ps1` |
| Worker | overlay image `db43bb68c042` (rollback `before-s5-layouts-20261021`); Ingest v2 in shadow; **media roles off** (`FORENSIC_OCR_ENABLED`, `_ASR_ENABLED`, `_FACE_ENABLED`, `_IMAGE_EMBEDDING_ENABLED`, `_VIDEO_ANPR_V2_ENABLED`, `_V3_ENABLED` all `false`) |
| Model | one container, Qwen3-4B-Instruct Q4_K_M, CPU, 4,096 context, one request at a time; about 22 tokens a second reading, 5.5 writing |
| Database | PostgreSQL 16 + TimescaleDB; migrations 012 to 020 applied (typed tables, analyst schema with **10 views**, read-only role `forensic_analyst_ro`, thick connection views); typed rows in 14 collections; backups in `C:/tmp/nexusai-backups` |
| Engine switches (asserted from the container) | `FORENSIC_ASK_V2=true`, `_FIRST=true`, `_AGREE=false`; `FORENSIC_GOVERNED_SQL=true` and first; `FORENSIC_TYPED_FAMILIES=communications_cdr,network_ipdr`; cost ceiling 5,000,000, row cap 1,000, timeout 15 s. **Still off:** `FORENSIC_CONVERSATION_FRONT_DOOR`, `FORENSIC_PII_MASKED_PROJECTION`, `FORENSIC_IR_CROSSCHECK`, `FORENSIC_BREAKDOWN_GOAL`, `FORENSIC_GOAL_TAXONOMY_SLICES`, `FORENSIC_IDENTIFIER_BINDING`, `FORENSIC_FRAME_FAMILY_PRECEDENCE`. **Still on (the ladder):** `FORENSIC_LADDER_ROUTING=true` |
| Interface | workspace at `:4181` (main checkout, Vite) and `:4190` (lane tree); 16 routes; 70 vitest files, 26 Playwright specs, about 9,170 lines of CSS; ECharts, Cytoscape.js, MapLibre GL JS |
| Repository | branch `lane-arms` (local worktree `NexusAI-localwork`) pushed to `origin claude/gifted-pasteur-4kpx0r` at `97bd0f9`; **the main checkout's local branch of the same name is 176 commits behind and holds 574 uncommitted paths** (408 reports, 73 api, 61 apps, 18 db …); the two interface trees differ in 11 source files once line endings are ignored |

### 3.2 What is built, by area

| Area | Built and measured | Not built or not proven |
|---|---|---|
| Structured ingestion | Layout registry (20 entries, drafts), Ingest v2 typed writer for call records, internet records, application usage; all 14 collections typed; time basis on every row; effective views drop exact repeats | Writers for ANPR, subscriber, tower reference, transactions, access logs (G43); import wizard; real-file confirmation of the 13 provider drafts; declared-total reconciliation check to be verified |
| Ask engine | Ten views, validator (96-statement red team), obligations (HOPS, RECURSION, SERVICES, HUBS, DIRECTION, NAME, VOLUME), probe, coverage guard, population guard, repeats note, audit row per question, engine-health and typing-report reads | Other families (M4), composition under the number lock (M5), bake-off (M6), languages (M7), retiring the ladder (M8), A5 at 15 minutes, A7, A9 to A11 through the engine |
| Link analysis | Common contacts, connections (shared-contact meaning), devices, cells, address holder; A1, A2, A3, A4, A5 (hourly), A8, A13 measured against planted truth | Meetings, successors, movement, temporal change, communities, mention index, entity resolution, link strength, relationship tables populated |
| Text evidence | Retrieval with citations, text-browse guard, PII withheld | Complete-search rule and curated sensitivity on every retrieval path (R5), reranker, BGE-M3, Docling |
| Media | Pipelines exist (plates on images, plate groups on video, OCR, ASR via LocalAI, faces YuNet+SFace, image embeddings SigLIP, document passages); 865 derived artifacts on the acceptance case; T2-V pack of 96 images; R8.1 to R8.4 done | Everything above is **off in the worker**; no analyst media view; accuracy weak (section 8.2); Boundary A blocked; no sealed Pakistani sets |
| Interface | 16 routes, tests, tokens, charts, link chart, map | Direction not settled; 22 of 26 backend requests open; link-analysis views disliked; media viewers read old derived data only |
| Governance | RLS, custody chain, read-only role, audit rows, real-data vault and scanner, licence ledger | Per-user identity, case record, authority record, rate limit, retention, backup rehearsal of restore |
| Process | Tier policy (T0 to T4), typing report, live smoke, `check_change.py`, gates written before runs | One repository history; repo-wide commit gate (the lane-scoped hook works); upstream pin |

---

## 4. The ledger: every open instruction, step and phase, with where it lands

Each row has an ID (`L-` and a letter for its source group), the source, the state read on 2026-10-21, and **where it lands** (a programme P0 to P5, PL, or "retired" with the reason). Nothing from the sources is dropped without a stated reason. State words: **done**, **partly**, **open**, **parked** (built, off, not measured), **retired** (superseded; the reason is given).

### 4.1 The question-answering line (state file, September and October roadmaps, backlogs, decision record)

| ID | Item | Source | State 2026-10-21 | Lands in |
|---|---|---|---|---|
| L-Q1 | Delete the keyword ladder and the templates (`query.go` 9,041 lines, 12 templates); first derive scope from the curated layer; delete in slices, each measured against a control | CONTINUATION §1; REMAINING_STAGES E; ROADMAP_FREE_QUESTION R6; M8 | **open.** `FORENSIC_LADDER_ROUTING=true`; the engine answers first for typed cases and the ladder still answers what the engine declines | P1 (M8): per family, after engine parity; gate "templates-off at least equal on the bank, the 103 and the pre-flight, 0 new wrong" |
| L-Q2 | Goal taxonomy: `breakdown` goal, `rows` shape, "longest/shortest" to `rank`, "first and last seen" to `range` | OPEN_ITEMS; REMAINING_STAGES F | **parked** (`BREAKDOWN_GOAL`, `GOAL_TAXONOMY_SLICES` off) | **retired with the ladder**: the engine writes these shapes in SQL; measure on the media bank (P3) before deleting the code |
| L-Q3 | PII masking at projection: case-scoped HMAC plate aliases, typed placeholders in OCR and transcript text, withhold the whole text if token redaction cannot be proven | REMAINING_STAGES A; CONTINUATION §7 | **parked** (built 2026-09-26, `FORENSIC_PII_MASKED_PROJECTION=false`, not measured; decision owed: with masking on, H1 stops being a refusal) | **D28**, then P3 (media views) |
| L-Q4 | Numeric-offset subtraction ("which plate was visible longest") | REMAINING_STAGES B | **open for media**; typed families are covered by SQL arithmetic | P3 with the derived-media views |
| L-Q5 | Template-path answers state their result (CDR-10, ANPR-03) | CONTINUATION §7 | **done** 2026-09-29 (A3) | |
| L-Q6 | Derived-row projection: list individual observations with `artifact_id` and a citation locator | REMAINING_STAGES D | **open** | P3: the derived-media views carry `artifact_id` and locator |
| L-Q7 | Cross-modal joins (cross-family 0 of 5; `Joins` validated and unused; `many_to_many`) | CONTINUATION §7; MEDIA_QUERY_ROADMAP stage 4 | **open**; the joins assumed did not exist (`cdr.msisdn` to `subscriber.msisdn` matched 0 rows) | P4: replaced by the identifier mention index and entity resolution, not by declared joins |
| L-Q8 | Unwired question classes: greetings, small talk, "what can you do", product help, concept explanation, out-of-scope | CONTINUATION §7; PROGRAMME phase 3; decision record | **open.** `FORENSIC_CONVERSATION_FRONT_DOOR=false`, `_ROUTER=false` | P1, **D6** (front door on behind its measured gates; the glossary is curated, the model never free-generates a definition) |
| L-Q9 | A held-out set of at least 40 structured and 20 media questions written against the data, scored zero confident-wrong | CONTINUATION §7; REMAINING_STAGES | structured: **replaced** by the factory and the generator's banks (0 wrong on fresh seeds); media: **open** | P3 (the media bank) |
| L-Q10 | E1: about 50 investigator-written questions with verified answers and unanswerable ones, scored so a wrong answer costs more than a refusal: the honest headline number | ROADMAP_20260929 E1 | **open**; needs an analyst to write them | **D26**; P1 gate |
| L-Q11 | E3 several candidate queries and answer only when they agree | ROADMAP_20260929 E3 | **parked**: measured unusable on the laptop (G49) | P-H (revisit only with a faster model) |
| L-Q12 | E4 approved-question library: verified examples guide the model | ROADMAP_20260929 E4 | **partly**: 14 verified examples with retrieval inside the engine | P1: grow it from the triage queue ("this answer is wrong", L-U6) |
| L-Q13 | E2 model bake-off (SQL-specialist models, larger models) under a written rule | ROADMAP_20260929 E2; M6 | **open**; needs the GPU (D3, not available) | P-H |
| L-Q14 | R5: curated sensitivity on every retrieval path; the complete-search rule ("not found" only after a full search); then the front door may promote a message to data | ROADMAP_FREE_QUESTION R5 | **partly** (text-browse guard, evidence-search-first); the full rule and PII probes at 100% open | P3 (documents track) |
| L-Q15 | R7 speed: shared prompt start (measured, not adopted), second memory stick (slot unconfirmed), thick views (done, migration 020) | R7; 6.8.13 | **partly** | P1: more thick views where a class is slow; the RAM stick when a slot is free; GPU later |
| L-Q16 | Free-word and value-list conditions; month against month | G11, G16 | **open** (not in the bank) | P1: add to the bank (stage 3) |
| L-Q17 | Phase C endpoints for the interface (processing history, image overlays, timeline events, server question history) | ROADMAP_20260929 C | **open** (rows 3, 4, 1, 5 of the request list) | P1/P3 (section 4.4) |
| L-Q18 | Phase F question types: relationships computed, links across evidence, date-first across families, entity profile ("everything about X"), summaries | ROADMAP_20260929 F | F1 **done** (shared handset, A8); F2, F4 (A16) **open**; F3 **open**; F5 = M5 | P1 (F3, F5), P4 (F2, F4) |
| L-Q19 | Phase D unanswered corpus questions by root cause (media details, query shapes, transcripts, one number across all evidence) | ROADMAP_20260929 D | **retired as a list**: replaced by banks per family; the media ones return in P3 | P3 |
| L-Q20 | Pin an unmodified upstream LocalAI image; rebuild the fork baseline in a sibling clone (`FORK_DELTA_v4.5.6.md`) | CONTINUATION §4 and phase 0 debt; G4 of ROADMAP_20260929 | **open** | P5 (R17) |
| L-Q21 | Startup switch log, compose-declared switches, container-asserted state before any run | OPEN_ITEMS; LESSONS | **done** (switch registry, startup line) and kept as a rule | rule |
| L-Q22 | The standing "ask the owner first" list (rebuild or redeploy, compose actions, database writes and migrations, model downloads, git writes, long builds, live evaluation suites, retained evidence) | CONTINUATION §6 | **active** | rule (section 10) |

### 4.2 The master directive's phases R9 to R17 and cross-cutting sections (the part missing from the current roadmaps)

R0 to R8 are done or covered (R8 is in 4.5). The directive's R9 to R17 and sections 12 to 23 were **not carried into the maturity roadmap** except in part; they are carried here.

| ID | Item | Source | State | Lands in |
|---|---|---|---|---|
| L-R1 | **R9 General image forensics**: OCR, object detection, metadata, perceptual hashing, visual embeddings, similarity candidates, screenshot analysis, document-image handling, policy-gated face features | Directive R9; 19.9 | **partly built**: image observation (26 artifacts), fingerprint (22), SigLIP embedding (18), `GET /images/similar`, `GET /faces/similar`; **not built**: general object detection, screenshot and document-image handling, a similarity candidate review view, EXIF and GPS policy | P3 track I (section 8.6) |
| L-R2 | **R10 Audio and speech**: benchmark ASR, VAD, time-coded transcript, search, speaker turns, optional diarization, citations, professional audio UI; TTS only as disclosed derived output | Directive R10 | **partly**: faster-whisper small through LocalAI, Roman Urdu, transcript artifacts; benchmark, VAD, diarization, gold sets **open** | P3 track A |
| L-R3 | **R11 Video**: metadata, scene detection, frame sampling, audio extraction and speech linkage, plates, objects, event timeline, frame and clip citations, selected-frame interpretation, video workspace | Directive R11 | **partly**: plate groups on video (24), ffprobe, frame sampling, audio extraction; tracking, scenes, objects, clips, workspace **open** | P3 track V |
| L-R4 | **R12 Financial and access/security**: dedicated adapters, exact decimals, no silent currency mixing, security-log normalisation, specialist views, correlation | Directive R12; 19.6, 19.7 | **partly**: adapters and templates exist (4 transactions, 1,000 access-log rows in the demo case); **no typed writer** (G43), no engine views | P1 (typed writers and views) |
| L-R5 | **R13 Documents and knowledge**: native extraction then OCR, tables, attachments, layout, page and region citations, document search and RAG, viewer | Directive R13; 15 | **partly**: native passages (115 on the acceptance case, 1,900 in the generated cases), retrieval, viewer; scanned-page OCR, tables (Docling), BGE-M3, reranker **open** | P3 track D |
| L-R6 | **R14 Captures, databases and archives**: PCAP/PCAPNG, EVTX, SQLite, SQL exports, safe archive extraction, parent-child lineage, inventory, bounded querying; never execute, no traversal, no decompression escape | Directive R14; 19.12 | **admission and inventory only** (2026-07 acceptance: PCAP, PCAPNG, SQLite, PDF and archive inventories, MPEG headers; EVTX classified); no bounded querying, no parent-child evidence rows for archive members | P3 track X (section 8.12), **D27** |
| L-R7 | **R15 Entity and relationship intelligence**: canonical entity resolution, aliases, confidence, conflicts, time validity, evidence, cross-family graph, relationship workspace; no model-only identity; zero cross-case leakage | Directive R15 | **open**; tables exist and are empty | P4 |
| L-R8 | **R16 Enterprise administration and scale**: multi-user, roles, quotas, API keys, OIDC, retention, legal hold, distributed workers, NATS security, object storage, backup, restore, monitoring, SLOs, security and load tests | Directive R16 | **open** (read-only role and RLS done) | P5 (a minimal case record earlier: D30) |
| L-R9 | **R17 Release and regression**: fixed gold sets, model, adapter, UI, security and performance regression, upgrade strategy, upstream merge strategy, deployment artifacts, release notes, support documents; the release lists what is operational and what is pending | Directive R17 | **open** | P5 |
| L-R10 | Directive 12 unified case knowledge fabric; 15 modality-aware chunking, retrieval security, prompt-injection defence; 17 specialist agents; 18 model governance (pinned hashes, ledger, no egress, model id on every derived artifact) | Directive 12, 15, 17, 18 | **partly**: custody chain, derived-artifact provenance, ledger, read-only tools; the red-team set for injection through evidence text **open** | P3, P5 |
| L-R11 | Directive 20 professional answer format: direct answer, validated interpretation, evidence, limitations, next steps | Directive 20 | **partly** (UX contract 6.6 fixes the order; interpretation under the number lock is M5) | P1 (M5) |
| L-R12 | Directive 26 phase execution contract, 27 vertical-slice definition of done (34 items) and 28 team-lead demonstration requirement | Directive 26 to 28 | **adopted in short form** (section 9.4): every programme step reports planned, implemented, tested, deployed, runtime-accepted, data-accepted, remaining; every phase ends with a repeatable demonstration | rule |
| L-R13 | Directive 30 living documents (state file, programme, capability log, handoff) | Directive 30 | **active**: state file `NEXUSAI_CONTINUATION.md`, plan `master-plan`, log `docs/work/CAPABILITY_LOG.md` and `HANDOFF_LOCAL` | P0 |

### 4.3 The maturity roadmap: gaps and decisions still open

Section 10 of `nexusai-maturity-roadmap-20261012.md` stays the register of G1 to G53. Open or partly open on 2026-10-21 (closed ones are not repeated):

| ID | Gap | State | Lands in |
|---|---|---|---|
| G1/G2 | The 13 provider layouts are drafts, none confirmed on a real file; no assisted mapping or import wizard | partly | P1: `confirm_layout.py` on the vault, then promote; the wizard in P2 |
| G3/G43 | No typed writer or layout for ANPR, subscribers, tower lists, transactions, access logs | open | P1 (M4) |
| G5/G6 | Relationship tables empty; relationship and correlation operations registered, not proven | open | P4 |
| G8/G9 | Foreign rows in call files; completeness against a declared total | to verify in Ingest v2 | P1 |
| G11/G16 | Free-word, value-list, month-against-month | open | P1 bank stage 3 |
| G15 | Front door off | open | P1, D6 |
| G17/G45 | Speed on the CPU: p50 30 s, p90 50 s, connection class about 40 s; the 15 s and 30 s targets unmet | partly | P1 levers; P-H |
| G18 | Questions in Urdu, Punjabi, Pashto unmeasured | open | PL, D10 |
| G19 to G21 | Speech, Urdu OCR, plates: weak, small sets | open | P3 |
| G22/D5 | Face model licence and policy | open; stays at detection only | P3 track F |
| G23 | No accuracy evidence on real data beyond counts and booleans | partly (real collection 6 of 6 live checks) | P1 (aggregate reconciliation per layout), P3 |
| G25 | No case registry; one fixed actor | open | D30, P5 |
| G26/G27 | Uncommitted UI work; repository commit gate cannot pass as imported | open (lane-scoped hook works) | P0, D13 |
| G28 | Lane-only synonyms not in the shared semantic layer | open | P1 |
| G29 | Text to speech blocked | not planned | |
| G30 | Dashboard counts 750 plate sightings, the lane 1,057 | open (copy fix) | P2 |
| G32 | Three sources show "retained copy missing" | open | P0 (the repair-assets route exists) |
| G33 | Multilingual embedding and reranker unmeasured | open | P3 track D |
| G34/D11 | No legal-basis (authority) fields for tower-dump, geofence and face analyses | open | P5 (before A6 or face L2 ship) |
| G42 | Collection listing from the API | open | P1 (request row 9) |
| G44 | Interface templates and direct `forensic.records` helpers still read version 1 | partly | P1 |
| G46 | Bank: A5 at 15 minutes, A7, A9 to A11, other languages | partly | P1, PL |
| G49 | Agreement as built is unusable | parked | P-H |
| G51/D13/D20 | Two working trees | open | P0 |
| G52 | Executor red team on a case of 21,000 call records | open (wave-1 cases of 4,000 to 9,000 call rows are now available) | P1 |
| G53 | Eleven or more pushed commits carry an AI `Co-Authored-By` trailer; rewriting needs a force-push | open; needs your word | P0 (D32) |

Decisions still open (D-list): **D3** GPU (unavailable), **D5** faces (stay at L0), **D6** front door, **D8** inventory page (engine view done), **D9** more real samples, **D10** reviewer names, **D11** tower-dump scope, **D12** priority, **D13/D20** repository history, **D2b** PostGIS (deferred). New in this plan: **D23 to D32** (section 11).

### 4.4 The interface and its backend requests

**The 30 requests** (26 from `BACKEND_REQUESTS.md` and 4 from `BACKEND_ROADMAP_GAPS.md`). Status read against the routes the API registers today (`GET /analysis/connections|places|engine-health|typing-report`, `GET /collections/status`, `GET /evidence…`, `POST /query/hybrid`, `POST /reports/generate`, `POST /evidence/{id}/reprocess`, `GET /faces|images/similar`, `GET /query/capabilities|templates`).

| # | Request | State | Served by (proposed) | Lands in |
|---|---|---|---|---|
| 1 | Timeline events across families (paginated, filter) | open | a thick view over typed events plus a fixed read | P1 batch BR-2 |
| 2 | Connections for one identifier | **partly** (`/analysis/connections`) | exists; extend to the entity drawer | P2 |
| 3 | Processing history per case | open | fixed read over `records_ingest_jobs` | BR-1 |
| 4 | Image overlays (OCR regions, plate boxes) | open | derived artifacts and `citation_locator` | P3 |
| 5 | Server-side question history | open | audit rows already hold it | BR-3 |
| 6 | Display labels, chart hint | labels **done**, hint open | | P2 |
| 7 | `source_truth_state` on citations | **done** | | |
| 8 | Evidence census by modality, family, state | open | **`v_evidence_inventory` exists** | BR-1 |
| 9 | Authorised collection directory | open | fixed read | BR-1 |
| 10 | Activity and custody history | open | custody chain and audit rows | P5 (early slice with D30) |
| 11 | Multi-case question scope | open (UI stopgap) | the engine over several collections | P1 |
| 12 | Workspace summary in one request | open | fixed read | BR-1 |
| 13 | Needs-attention feed (failed, copy missing, stalled) | open | fixed read | BR-1 |
| 14 | Reprocess and retry | **mostly done**; `retryable` field open | | BR-1 |
| 15 | Live workspace events (SSE) | open | NATS subjects exist | BR-4 |
| 16 | Data quality and coverage per family | open | **the typing report is the same idea** | BR-1 |
| 17 | Report history | open | store generated reports | BR-4 |
| 18 | Case display name and status | open | needs the case record | **D30**, BR-1 |
| 19 | Case activity buckets, top entities, rhythm matrix | open | **views already hold it** (`v_comm_events`, `v_cell_visits`) | BR-2 |
| 20 | Intraday timeline (hour buckets) | open | same views | BR-2 |
| 21 | Timeline annotations shared across analysts | open | new table | BR-3 |
| 22 | Event clusters and co-occurrence | open | thick view (the A5 logic) | BR-2 |
| 23 | Case record (name, status, created_by, owner) | open | new table | **D30**, BR-1 |
| 24 | Case membership and `mine=true` | open | | P5 |
| 25 | Real identity end to end | open | | P5 |
| 26 | Provenance read (`registered_by`, custody events) | open | data exists | BR-4 |
| G-1 | Co-occurrence of an entity across cases | open | cross-case view; **authority and tenancy rules first** | P4 |
| G-2 | Micro-timeline density (hour buckets) | = row 20 | | BR-2 |
| G-3 | Case metadata and horizon | = rows 18, 23 | | BR-1 |
| G-4 | Case notes and hypotheses store | open (localStorage today) | new table | BR-3 |

**Ruling:** build the requests as **five batches, each a small set of fixed reads over views already in the database**, in the order BR-1 (case record, census, health, summary, attention, directory, retry flag), BR-2 (timeline events, density, co-occurrence clusters, entity activity), BR-3 (annotations, notes, question history), BR-4 (events stream, report history, provenance read), then identity (P5). The engine's thick-view approach (migration 020) is the pattern: a reviewed view, a fixed read, a spec that equals the oracle. **None of this changes the safety wall**; every read is scoped, read-only and masked as the existing routes are.

**Interface items carried from the plan, the contract and the three redesigns:**

| ID | Item | State | Lands in |
|---|---|---|---|
| L-U1 | Lanes, calendar and device timelines (Timeline: Chronology, Lanes, Places, Calendar) | Chronology and Places built; Lanes and Calendar open | P2 workbench |
| L-U2 | Quality tab, layout chip, import wizard, assumption chips | typing-report card built; wizard open | P2 and BR-1 |
| L-U3 | Evidence inventory and coverage on the Case Overview; data-basis chip (synthetic, real-shaped, real) | open (view exists) | P2 |
| L-U4 | Entity drawer everywhere an identifier appears, with "Ask about this" | open | P2 |
| L-U5 | Dashboard engine and data-quality tiles | engine card exists (mounted on the Ask start only) | P2 |
| L-U6 | Activity triage queue ("this answer is wrong" files the question into the bank) | open | P1 and P2 |
| L-U7 | Settings for the engine and basemap (administrator) | open | P5 |
| L-U8 | Demonstration panel on the acceptance collection | open | P2 (D18 loading done) |
| L-U9 | Mentions across evidence in the evidence viewers (page boxes, waveform second, frame) | open | P3 and P4 |
| L-U10 | Design pages for every new component with loading, empty, abstention, error and large-data states, light, dark, right-to-left | partly | P2 |
| L-U11 | Contract amendments: "too broad", "engine unavailable", "basemap unavailable" states; result views and workbench; Connections screen contract; right-to-left rules for charts and maps; the visual section if the direction changes | open | P2 (the owner accepts each) |
| L-U12 | Entity-resolution review screen (merge, split, undo with reason); saved views; PDF export with evidence appendix; analyst usability test with a written task list | open | P4 |
| L-U13 | Core-loop test: an unaided analyst completes login, create case, add data, wait, ask, read, open a citation, follow up, find it in Activity (UX contract 13.1) | open | P2 gate, P5 |
| L-U14 | Accessibility and right-to-left audit; four widths; keyboard-only core loop; the Urdu typeface decision (Nastaliq or Naskh) | partly | P2 |
| L-U15 | Three redesign directions and their stage lists (Identity A S0 to S12; Stitch stages 0 to 11; the dark directive's stages 0 to 8) | conflicting | **one programme, section 7** |
| L-U16 | The 7.9 offline map rules (basemap status badge, fallback grid, the owner's tile server through our cache) | done in the spike and the workspace | keep |
| L-U17 | Chart rules: every chart answers a named question; a table path and a text equivalent; no pie above four slices | active | keep |

### 4.5 Media (details in section 8)

| ID | Item | State | Lands in |
|---|---|---|---|
| L-M1 | R8.5 to R8.10: bounded OCR pipeline, Pakistan plate normalisation and structured linkage, human review and correction with custody, ANPR specialist in Ask, performance and security hardening, guarded runtime and manual acceptance | **blocked at Boundary A** | P3 track P |
| L-M2 | Boundary A closure: a detector that passes precision and recall, OCR above thresholds, T2-V accepted, licensed real-image T3; Boundary B needs authorised T4 | open | P3 track P |
| L-M3 | Plate detector and reader licences (YOLOv9-t weights and CCT-XS v2 weights unresolved; the AGPL video stack blocked) and the replacement path (Apache-2.0 detectors; a fine-tuned reader) | open | P3 track P, **D31** |
| L-M4 | Sealed Pakistani plate set (at least 300 tracks across day, night, rain, blur, angle) and the consented capture protocol | open | P3, **D26** |
| L-M5 | Confusion-aware plate matching (M and W) as labelled candidates | open | P3 track P |
| L-M6 | Video: tracking (ByteTrack-class, licence to confirm), crop selection, character voting, adaptive sampling, shot boundaries, captions as "model description, unreviewed" | open | P3 track V |
| L-M7 | Speech gold sets per language, bake-off under a written rule, VAD, language ID, diarization, spoken-number conversion, digit-sequence exact match, script correctness | open | P3 track A, PL |
| L-M8 | Urdu OCR set (500 to 1,000 lines), bake-off, synthetic fine-tune; Pashto OCR evaluation | open | P3 track O |
| L-M9 | Documents: Docling for scans and tables, page and box citations, BGE-M3, reranker, forms and ID extraction with validators | open | P3 track D |
| L-M10 | Faces: L0 detection only; L1 and L2 only after D5, a consented South Asian evaluation set and counsel | open | P3 track F |
| L-M11 | General image forensics (R9) | partly | P3 track I |
| L-M12 | Captures, databases, archives (R14) | inventory only | P3 track X, D27 |
| L-M13 | New families: message exports, location history | open | P4, D29 |
| L-M14 | Worker media roles switched off; activation family by family with the resource policy | open | **D25**, P3 |
| L-M15 | The engine reads no derived media: add views over `derived_artifacts` to the analyst schema (curated, non-text fields; text only through retrieval) | open | P3 (M4 media) |
| L-M16 | Media viewers with overlays, review desk with custody, candidate cards with the authority banner | open | P2 and P3 |
| L-M17 | Media bank (at least 20 questions across modalities against the data) and the sealed sets | open | P3 |
| L-M18 | Open licence questions (SeamlessM4T, MMS, Silero artifact, pyannote gate, DiariZen, datasets) | open | section 10 |

### 4.6 Process, governance and repository

| ID | Item | State | Lands in |
|---|---|---|---|
| L-P1 | One repository history; the main checkout is 176 commits behind with 574 uncommitted paths; the interface trees differ in 11 source files | open | **P0** |
| L-P2 | The state file is stale; AGENTS.md does not point to the plan of record | **done today** | P0 |
| L-P3 | Repository-wide commit gate cannot pass (missing mock backend; distributed test does not compile); the lane-scoped hook works | open | P0, D13 |
| L-P4 | Lint debt: 409 line-level `nolint` | open | P5 |
| L-P5 | Trailers (G53) | open; the owner's word | D32 |
| L-P6 | Backup restore rehearsal for the TimescaleDB dump (the dumps exist; restore needs the pre and post steps, not rehearsed) | open | P0 (one rehearsal) |
| L-P7 | Postgres shared memory 64 MB (a large parallel query failed once); `shm_size` or `max_parallel_workers_per_gather = 0` for the analyst role | open | P1 (one line) |
| L-P8 | Licence scanner for Go and npm dependencies; fonts and base images | open | P5 |
| L-P9 | Disk: many rollback images and backups accumulate | open | P0 (list, keep the named tags, report sizes; delete nothing without a go) |
| L-P10 | Testing policy: tiers T0 to T4; no spread after each fix; fresh seed after a fix; judgement and sealed cases never tuned on | **active** | rule (section 9) |

---

## 5. Rulings: where documents conflict

| # | Conflict | Ruling and reason |
|---|---|---|
| C1 | Three interface directions (Identity A teal, light-first Stitch plan, dark Linear/Palantir directive) with three stage lists | **One programme** (section 7): the UX contract binds it; direction decided by D23 after a side-by-side proof, not by reading; the dark directive's working laws (one section at a time, real data only, backend gaps logged, live check) are kept as rules |
| C2 | Ownership: 2026-09-27 "Claude owns `apps/**`, Codex owns `api/**`"; 2026-09-29 reversed ("Claude: `api/**`; Codex: `apps/**`"); today one session edits both and another edits the shared styles | **D24:** the API, ingestion, database and evaluation belong to this session; the interface programme has **one owner at a time per stage**, who alone edits the shell, tokens and workspace styles; anyone else writes a request. Changes to `api/**` for the interface are small fixed reads built in P1 batches |
| C3 | "The model never sees case data" (decision record, §G.5) against D17 (the model may explain the analyst's own result, masked and capped, under the number lock) | **D17 stands** (decided 2026-10-14). Until M5 is measured, the deterministic sentence is the answer and the model only understands the question and writes the query |
| C4 | The master directive asks for templates, operations and specialist agents per family; the 10-05 decision freezes templates | **The decision record wins.** Operations remain oracles and building blocks; new capability is a reviewed view, an example and a bank, never a template |
| C5 | R8's Boundary A and B gating (plates) against the maturity roadmap's looser Phase 3 thresholds | **Both apply.** R8's evidence tiers (T0 to T4) and two boundaries remain the gate for plates and any image model; the maturity roadmap's numeric targets are added thresholds, set before the first measurement |
| C6 | R-numbers (R0 to R17), NX tracks, STIM phases, Phase 0 to 5, S1 to S8, M1 to M8, P0 to P5 here | **Mapping table below.** New work is tracked in P-programmes; old labels are scope names only |
| C7 | The contract says an analyst never sees SQL or a model; a power-user console was an option | **Option A stands** (D17b): derivation and checks in plain words; the query one level deeper for administrators |
| C8 | The dark directive's near-black canvas with inner bevels and tactile physics, against UX contract 8.3 (no flat black canvas, no neon, no glass, no card soup) | **The contract wins** until the owner accepts an amendment (L-U11); the direction proof must show both directions inside the contract's rules or state the amendment it needs |
| C9 | "Real data only, zero fake data" (dark directive law 3) against the real-data policy (no real values in screenshots, repositories, cloud design tools) | **Both:** the app shows only verified service data; demonstrations and screenshots use generated or synthetic collections; Stitch and any cloud tool receive synthetic content only |
| C10 | The older roadmaps say "no new media model downloads or evaluations except by a rule written first" and "ask the owner first" | **Stands, per item (D4):** each model, dataset or backend is a ledger row, a written decision rule and an owner's yes before download |
| C11 | R14 in the directive means full analysis of captures and databases | **Inventory first** (D27): safe listing, hashing, parent-child lineage and bounded read-only queries; no execution, no extraction of members without limits |
| C12 | The 09-26 plan says media derived rows are never merged with ingested records | **Stands.** A derived family is its own view; a cross-family answer is a labelled composition (P4), never a join of provenance |

**Label mapping (so old documents stay readable):**

| Old | Means | Here |
|---|---|---|
| R0 to R7, STIM-0 to 7, NX-B2.1 | done | |
| R8 | ANPR and OCR contract | P3 track P |
| R9 | general image forensics | P3 track I |
| R10, R11, R13 | audio, video, documents | P3 tracks A, V, D |
| R12 | financial, access | P1 (typed writers), P4 |
| R14 | captures, databases, archives | P3 track X |
| R15 | entity and relationship | P4 |
| R16 | enterprise | P5 |
| R17 | release | P5 |
| Maturity Phase 0, 1, 1.5 | safeguards, structured data, dependable and fast | done (P0 here is a new consolidation) |
| Maturity Phase 2 (M1 to M8) | the Ask engine | P1 (M3, M4, M5, M8), P-H (M6), PL (M7) |
| Maturity Phase 3, 4, 5 | media, cross-family, enterprise | P3, P4, P5 |
| S1 to S5 | reconciliation steps | done |
| S6 | interface | P2 |
| S7 | widen the engine | P1 |
| S8 | process | P0 |

---

## 6. The plan

### 6.1 Principles (all programmes)

1. **Accuracy before breadth.** A family or view ships when its bank passes its gate on judgement cases; otherwise it stays declined, and the older path answers.
2. **Gates before runs.** Thresholds are written and committed before the first measurement; a fix is measured on another fresh seed; judgement and sealed cases are never tuned on; the oracle's and the judge's own defects are reported.
3. **Less testing, better placed.** The tiers T0 to T4 of `tools/dev/README.md` apply: a change is checked by the cheapest tier that can catch it; full banks and spreads run once per milestone, in the background (section 9).
4. **One change, one switch, one measurement.** Every behaviour change is behind a declared switch (default off unless off is the unsafe state), asserted from the container before a run, with a tagged rollback image.
5. **The interface ships with the capability, designed first** (V1 to V9 of the maturity roadmap, with the design-first rule of section 7).
6. **Nothing live without a go.** Deploys, migrations, backfills, downloads, worker role activations and any git write that is not a plain push to the working branch wait for the owner.
7. **Real data stays real-data-handled.** Counts and booleans only for the real collection; the vault, the policy and the scanner stay.
8. **Every phase ends with a repeatable demonstration** (the team lead's rule): a script, exact questions and clicks, expected results, run on a generated case.

### 6.2 P0: consolidate (days 0 to 7)

Purpose: stop the drift before adding anything. Nothing here changes behaviour.

| Step | Work | Output | Needs you |
|---|---|---|---|
| P0.1 | **Archive the main checkout's uncommitted state** (574 paths) outside the repository (a zip and a path list); change nothing | An archive that makes every later step reversible | |
| P0.2 | **State file and AGENTS.md pointer** rewritten (done with this plan); the capability log and handoff updated | A state file an agent can act on | |
| P0.3 | **Classify the 574 paths** by content, not by `git status`: for each path compare the main tree with `lane-arms` and the merge base (`3fca698`); the expected result is that api, db, evaluation, docs, scripts and pkg paths are **older versions of files `lane-arms` already holds**, and the interface and reports hold the unique work; list the unique files | A list "unique to main" (expected: the interface redesign files, some design-review images, research notes) | |
| P0.4 | **One canonical tree.** Recommended: `NexusAI-localwork` (`lane-arms`) becomes the only tree that is edited; the interface session's unique files are committed there on their own branch (their commits, their trailer), the eleven source files that differ are reconciled by taking the newer of each (a three-way view), `:4181` is served from the canonical tree, and the main checkout is left as an archive until you say it can be reset | One history; the copying stops | **Yes: D13 with D20 and D24** (a step-by-step merge, as decided 2026-10-14) |
| P0.5 | **Restore rehearsal** of the latest dump into a throwaway container (the TimescaleDB pre and post steps), then delete the throwaway | A proven restore | |
| P0.6 | **Disk and image inventory**: sizes of the rollback images and backups, which tags are named in the state file; nothing deleted | A report | |
| P0.7 | **Missing retained copies** (G32): the three sources; plan the repair with the existing repair-assets route | A plan | a go |
| P0.8 | Commit gate: keep the lane-scoped hook; write down what the repository-wide gate needs (restore or remove the mock-backend dependency; fix the distributed test) | A one-page decision | D13 |

**Gate:** one repository history and one served tree; the state file and plan of record true; the restore rehearsed; the uncommitted interface work committed on a branch. **Demonstration:** `:4181` served from the canonical tree shows the same pages as before.

### 6.3 P1: finish the structured and link-analysis core (weeks 1 to 6)

This is the owner's priority (D12). It is also the part where the measurement discipline is strongest, so it moves fastest.

| Stream | Work | Gate (written first) | Time |
|---|---|---|---|
| **P1-A Link analysis to the end** | Bank stage 3: **A5 at 15 minutes** (a finer co-location view than the hourly one), **A7 possible meetings**, **A9 successors**, **A10 movement and anchors**, **A11 temporal change**, A12 communities (a bounded tool call, NetworkX), the free-word, value-list and month-against-month shapes. Each: a reviewed (thick) view, a verified example, bank questions with paraphrases scored against the generator's keys, a caveat text attached by the engine ("proximity is not association", "candidate, not a finding") | Per analysis at least 90% correct and no wrong on judgement cases, the rest abstained; a T4 run once at the end of the stage, in the background | 3 weeks |
| **P1-B Other families typed** (G43, M4) | Typed writers, layouts and views, in value order: **ANPR sightings** (A15; table `plate_sighting` exists), **subscriber, SIM and device reference** (tables exist), **cell and tower reference** (`cell_site`), then **access logs** and **transactions** (**no typed table exists: migration 021**, with preflight, verify and rollback as before). Shape-preserving synthetic goldens; the engine's coverage guard counts them | Each family: layout acceptance, reconciliation on aggregates, bank at the M2 standard on a fresh seed | 3 to 4 weeks |
| **P1-C Conversation and composition** (D6, M5) | Front door on behind its gates: greetings, help, what can you ask here (suggestions generated from what the case holds), a **curated glossary** for concepts (the model never free-generates a definition), follow-up memory with the assumption shown; the **answer sentence composed under the number lock** (every number in the text must exist in the result, else the deterministic sentence is shown); the clarification policy (at most one question, at most four options from the data) | Zero case facts in conversational replies; the lock never violated on the red-team and evaluation sets; the owner's review of the wording | 2 to 3 weeks |
| **P1-D Backend batches** | BR-1 (case record and display name, census, health, summary, attention feed, directory, retry flag), BR-2 (timeline events, density, co-occurrence, entity activity), BR-3 (annotations, case notes, question history): each a fixed read over a reviewed view, with a spec equal to the oracle | Spec equals the SQL truth; masked as the existing routes; honest empty states | 3 weeks, in parallel |
| **P1-E Retire the ladder** (M8) | Per family where parity is shown: derive scope from the curated layer, switch the family's templates off, measure templates-off against templates-on on the bank, the 103 and the pre-flight, delete in slices | Templates-off at least equal, 0 new wrong, per family | from week 4 |
| **P1-F Honest accuracy and hygiene** | **E1: 50 investigator-written questions** (D26) scored with a wrong answer costing more than a refusal; the executor red team on a wave-1 case of 9,000 call rows (G52); `shm_size` or no parallel workers for the analyst role (L-P7); real-file aggregate reconciliation per provider layout and promotion of confirmed layouts (G1, G23; real data stays counts-only); declared-total reconciliation (G9); lane-only synonyms into the shared layer (G28) | The headline number recorded; red team passes | 2 weeks |

**P1 gate (proposed):** every family the engine claims answers its bank at the M2 standard on a fresh seed (correct at least the older path's, wrong at most its, declines not above); link-analysis banks at their thresholds; the front door's gates met; one T4 run read; E1 recorded; the live smoke green after each deploy. **Demonstration:** an analyst asks, on a wave-1 case, in plain words: who talks to whom, how two numbers are connected, who was at the same cell, did a number change phone, which plates travelled with this one, what is in the case, and gets answers with the derivation in plain words and a number lock.

**Speed in P1 (D21 laptop first).** The laptop budget is p50 about 30 s, p90 about 50 s, a recursive question about 40 s with the thick view. The levers in order: more thick views for slow classes (a migration each), the second memory stick when a slot is free, a shorter output, fewer repairs; a GPU only if measured to be a must (P-H).

### 6.4 P2: the interface programme (weeks 1 to 16, parallel; section 7)

Design first, one direction, one owner, one stage at a time; the link-analysis workbench is its centrepiece. Runs in parallel with P1 because it uses the backend batches as they land. Details, stages, gates and the direction proof are in section 7.

### 6.5 P3: media depth (weeks 3 to 24, tracks in parallel; section 8)

Tracks: **D** documents and printed OCR with retrieval; **P** plates on images (R8 to Boundary A); **A** audio and speech; **I** general image forensics; **V** video; **O** Urdu and Pashto OCR; **F** faces at detection only; **X** captures, databases, archives (inventory first). Each track has an entry criterion, a sealed set, a decision rule written first, a viewer and an engine view. The first step of every track is the same: **switch the worker role on for that family only (D25), on the generated or controlled data, measure, and switch it off again if the gate fails.** Section 8 is the in-depth plan.

### 6.6 P4: cross-family intelligence (weeks 10 to 28)

| Work | Detail | Gate |
|---|---|---|
| Identifier mention index | An identifier (number, IMEI, IMSI, IP, plate, account) found in a document passage, an OCR line, a transcript segment or a plate read becomes an `entity_mention` row with its locator (page and box, second, frame) and the extractor's confidence; Urdu digits and letter forms normalised; **a mention is `observed` with the OCR or speech confidence carried, never a fact about the person** | Planted mentions found, noise tolerated, no mention without a locator |
| Entity resolution | Rules, not a model: exact strong keys; aliases kept with their source; conflicts shown with both sources (two subscribers for one number and time); time validity; nothing merged without a recorded reason and an undo; names never keys | Planted aliases and conflicts; no model-only identity; cross-case leakage zero |
| Relationships populated | `observed` from typed events, `inferred` (co-located, shared address, co-travelled) with their stated chance baselines and caveats, `candidate` (successor, face) never used by link analysis until a person confirms | Explanation completeness: every edge maps to at least one evidence row |
| Link strength | A **documented formula, not a learned model** (frequency, recency, duration, reciprocity, night and weekend share, independent evidence types, time consistency), shown with its features, calibrated on planted truth | Calibration on planted cases; never replaces the evidence list |
| Analyses | A6 tower dump and geofence (authority-gated), A12 communities and brokers, A14 mentions, A15 convoys, A16 one timeline per entity, A17 flows, A18 anomalies | Precision and recall on planted cases at thresholds set after the baseline |
| New families | **Message exports** (chat and email: sender, recipient, time, text, attachments, linking phone entities) and **location history** (method and uncertainty on every point), through the same layout registry (D29) | Layout acceptance; goldens |
| Cross-case co-occurrence (G-1) | "Has this number appeared in another case" | Only with the authority and tenancy rules of P5 designed first; off by default |
| Interface | The full link chart, entity-resolution review screen, per-entity timeline, saved views, export with evidence appendix, a usability test with the task completion and error rates written before it | Section 7 |

### 6.7 P5: enterprise and release (weeks 20 to 36)

Identity (per-user sign-in carried through the proxy; analyst, reviewer, administrator), case registry and membership (rows 23 to 25), the **authority record** per case (legal basis and reference, who authorised, which sensitive capabilities are enabled: off by default for face L1 and L2, voice identification, tower-dump and geofence), audit and custody views, retention and legal hold (set with counsel), quotas and rate limits, backup and restore proven, monitoring and SLOs, injection and isolation tests (the evidence-text red-team set), load tests at the interface budgets, the upstream LocalAI pin and fork baseline (L-Q20), the Go and npm licence scanner, release regression (fixed gold sets for models, adapters, interface, security, performance), an upgrade and rollback strategy, deployment artifacts and the release document that **lists what is operational and what is pending**. **Gate:** release approval (NX-C2), including the interface checks. **D30 pulls the minimal case record forward** into P1 because most interface requests need it.

### 6.8 PL: languages (weeks 6 to 26, parallel)

Needs **D10** (named native reviewers for Urdu, Punjabi in Shahmukhi, Pashto, and Roman Urdu) and consent for recordings. Work: normalise digits and letter forms; synonym lists curated by native speakers (not generated); 200 questions per language across the families with their answers; frame accuracy and end-to-end accuracy against thresholds set after the baseline; answers and chart labels localised and right-to-left; speech and OCR gold sets share the same reviewers. **Not claimed:** any model's language coverage until measured on these sets (Qwen3's list of languages names neither Pashto nor Punjabi in the sources found).

### 6.9 P-H: hardware and models (conditional)

The RTX 2080 machine is not available (owner, 2026-10-20); D3 stays open and **nothing depends on a GPU**. When it is free: measure the shared-prompt cache and a larger query-writing model, the speech bake-off and a vision-language model for captions, under written decision rules. Cheap laptop levers first (thick views, the second memory stick). A purchase only if a measurement shows a budget cannot be met or a larger model is needed for accuracy.

### 6.10 Order, dependencies and the critical path

```
P0 (days 0-7) ──► P1-A, P1-B, P1-C, P1-D (weeks 1-6) ──► P1 gate ──► P4 (weeks 10-28)
      │                  │
      │                  └─► P1-D feeds P2 (backend batches as they land)
      ├──► P2 U0 direction proof (week 1) ─► U1.. stages (weeks 2-16)
      └──► P3 media: D25/D26 decisions (week 1) ─► track D and I first (week 3), P, A, V, O, F, X staggered
PL (reviewers named, week 1) ─► sets (weeks 6-26)        P5 (weeks 20-36) after P1, P2 stages 1-6, P3 documents
```

**Critical path:** P0.4 (one tree) → U0 (direction) → the workbench stage; and P1-B (typed writers) → P4 (mentions and relationships). **What can run in parallel without touching the same files:** API and ingestion (P1, P3 pipelines) in one session; the interface in a second; the evaluation and documents in either. Never two sessions on the same files.

### 6.11 Effort, honestly

Weeks are planning estimates for one owner plus Claude sessions; they are re-planned at each gate and are not commitments. The estimate that is most uncertain is P3 (each media track depends on data that does not exist yet: sealed Pakistani plate captures, Urdu lines, consented speech); the one that is most certain is P1-A (the oracles, the bank tooling and the thick-view method exist). A rough size: P0 one week; P1 six weeks; P2 sixteen weeks of stage-by-stage work; P3 twenty weeks with tracks staggered; P4 eighteen weeks after the typed writers; P5 sixteen weeks; PL twenty weeks of reviewers' time. **The first visible result for you is in week 1 (the direction proof of the link-analysis workbench) and week 3 (the engine answering A7 and the other families on a wave-1 case).**

---

## 7. The interface programme: one direction, designed first, with the link-analysis workbench at its centre

### 7.1 Diagnosis: why the redesign has not converged

Read from the documents, the code and the owner's two statements (2026-10-20: "it didn't work well"; 2026-10-21: "I don't like the present added UI/UX for connections and the other link-analysis views").

| Layer | When | What it is | State |
|---|---|---|---|
| 1. "Identity A" and stages S0 to S12 (Codex) | 2026-09-29 to 10-02 | Navy-teal chrome, near-white cards, teal accent; every page rebuilt section by section; 16 routes, 70 vitest files, 26 Playwright specs, about 9,170 lines of CSS | **Built and working; the look the owner wants to leave** |
| 2. The Stitch plan (light-first, "editorial calm") | 2026-10-20 | Stage 0 brief and a plan of 11 stages; decisions D1 to D6 owed (theme, personality, type, brand, branch, density); Stitch as an exploration tool | **Stage 0 only; nothing designed or changed** |
| 3. The dark directive ("Linear, Vercel, Palantir") and `CODEX_UI_TRACK` | 2026-10-19 to 10-20 | Layered charcoal canvas, monochrome plus one accent, tactile motion, tri-pane synchronisation, pivot-and-probe drawers; Stages 0, 1 and 3 reported done (tokens, shell, dashboard, case overview); Stage 4 (Ask) next | **Uncommitted, in two trees; the owner's own attempt; "did not work well"** |
| 4. The link-analysis views (mine) | 2026-10-15 to 10-21 | Connections page (filter bar, link chart, panel, table), Places view, result views by shape, data panel; 424 tests | **Built for correctness, not designed as an investigator's workflow; disliked** |

**Why it did not converge (my reading, stated so you can correct it).**

1. **No design decision came before code, at any layer.** Each generation began from a prompt and a stylesheet, not from a design you had looked at and accepted. A taste problem cannot be solved by describing taste; it is solved by looking at two or three concrete things and choosing.
2. **Three directions in one repository.** The three disagree on the basic question (teal-navy, light paper, charcoal) and on the contract (the dark directive's near-black bevelled canvas sits against the contract's 8.3, which forbids a flat black canvas, glass and "card soup"). Each new session built on or against the last.
3. **My link-analysis views inherited the September look and a developer's logic.** They start from a filter bar and a chart, show one view per page (Connections here, Places there), and make the analyst operate controls rather than pursue a question. They do not have the three things investigation tools agree on: **coordinated views, a small working set, and evidence one click away from every mark.** That is my defect, not a taste difference, and it is why they are redone from a design rather than polished.
4. **Shared files with more than one editor** (`shell.css`, `tokens.css`, `workspace.css`, `identity-a.css`) and two working trees copied by hand.
5. **The backend asks accumulated** (30 requests, 4 delivered), so many designed screens show honest empty states instead of the data a design needs.

### 7.2 The resolution: one programme, six rules

1. **One direction, chosen by looking (D23).** Week 1 delivers a **direction proof** (7.6): the same three screens in two directions (A: light-first "paper" with a designed dark theme; B: dark "instrument"), clickable, on synthetic data, each inside the UX contract or naming the amendment it needs. You choose in one sitting. Type, density and brand are decided at the same time (the Stitch plan's D1 to D6, merged).
2. **One owner per stage, one tree, one branch.** The canonical tree is the lane worktree (P0.4). The owner of a stage alone edits the shell, tokens and workspace styles; everyone else files a request. Stage gates are accepted by you on `:4181`.
3. **Design first, at every stage** (the loop of the Stitch plan, kept): a one-page **research note** with sources; **two options as prototypes or mockups**; **you decide**; **build** over the existing, tested data hooks; **prove** (keyboard, four widths, light and dark, contrast, a generated case, a demonstration script); **close** (contract and state file updated). No stage starts by editing CSS.
4. **Keep what works.** The data hooks, the presentation logic and their tests (`analystPrimaryAnswer`, `analystAskPresentation`, `analystDataPresentation`, `connectionsModel`, `resultShapes`, the evidence viewers' logic), the ECharts wrapper, Cytoscape.js and MapLibre GL JS stay; **markup and CSS are replaced stage by stage**; the old and new tokens coexist only behind an alias layer that is deleted at the end.
5. **Honest data only.** The app shows verified service data; where a design needs a backend that does not exist, it shows a dignified designed state and the request is filed (section 4.4). Demonstrations and screenshots use generated or synthetic collections; cloud design tools (Stitch) receive synthetic content only.
6. **The contract binds.** `NEXUSAI_PRODUCT_UX.md` is amended only by a decision you make at a stage gate (L-U11).

### 7.3 The link-analysis workbench (the centrepiece)

**What "world-class" means here** is not decoration. Investigation tools (Maltego, i2 Analyst's Notebook, Magnet AXIOM Connections, Cellebrite Pathfinder, Siren, Linkurious) differ in polish but agree on a small set of properties. The workbench is designed to have all of them, with the product's own differentiator, **provenance as the primary object** (UX contract §4), made visible.

| Principle | What it means in the workbench | Source |
|---|---|---|
| **Coordinated views, not separate pages** | Graph, map and timeline are three panes of one workspace. A selection in one is a selection in all: brushing the timeline filters the graph and the map; selecting a cell lights its subscribers and its events; selecting an edge isolates its rows in the table. Add a view only if analysts use it | [dciWebMapper2, coordinated views](https://arxiv.org/pdf/2509.07897); the 2014 VAST crime-analysis design (multi-lane timeline linked to a social network); Siren's pivot between graph, map and timeline ([Siren link analysis](https://docs.siren.io/siren-platform-user-guide/14.8/graph-browser/t_link-analysis.html)) |
| **A small working set, grown deliberately** | The default view starts from a question (two numbers, or one number and its strongest neighbours), never "everything". Expand by one hop with a stated limit; **save the state before expanding**; crop to a selection; hubs and service numbers are shown as *excluded*, not hidden | Siren: "link analysis is best performed on a small selection"; save before expanding; crop |
| **Question first, then the picture** | The Ask box opens the workbench with the path highlighted and the sentence above it; any identifier chip in the app opens the entity drawer with "Ask about this" | Product direction (U4); Maltego-style pivots become questions to the engine |
| **Every mark opens its evidence** | Node, edge, cell, window: a drawer with the rows (virtualised), the locators, the time basis, the repeats note; the table twin is one key away; claim-level citations as in the contract | UX contract §4 |
| **Honest encoding** | Edge style by kind (observed solid, inferred dashed, candidate dotted) with the chance baseline and the caveat text ("proximity is not association", "candidate, not a finding"); node shape by entity type; colour by group, colour-blind-safe, **never the only encoding**; size by a stated measure with a legend; uncertainty drawn (cell radius) | Maturity roadmap V4; Cambridge Intelligence styling guidance |
| **Scale without a hairball** | Level of detail beyond about 2,000 nodes (community collapse, path shortening, grouping); map clustering beyond 10,000 points; time-series decimation with exact values one click away; budgets: first view under 2 s, 60 frames a second to 5,000 nodes and 50,000 points on the laptop | Maturity roadmap V5; Ogma's own descriptions of level-of-detail and grouping are vendor claims, used as a checklist and measured on our library |
| **Reproducible and exportable** | The state (question, filters, selection, window) is in the URL; the same state gives the same picture; PNG, SVG and PDF export with the query, conditions, case and layout versions and an **evidence appendix** | Maturity roadmap V8 |
| **Accessible and bilingual** | Keyboard traversal (focus, arrow keys by neighbour, escape), a text summary and table alternative, reduced motion, right-to-left labels, Urdu names with a deliberate typeface | UX contract §10; roadmap V7 |

**Layout (to be tested in the direction proof, not fixed here).**

```
┌ case header ──────────────────────────────────────────────────────────────────────┐
│ question box / chips ("How are A and B connected?")      [save view] [export]     │
├ working set ┬ main canvas (two of three visible; toggle the third) ┬ drawer ────────┤
│ entities    │ GRAPH  ▸ path highlighted, hubs shown as excluded     │ entity or edge │
│ saved sets  │ MAP    ▸ cells with uncertainty circles, clusters     │ evidence rows  │
│ filters     │ TIMELINE ▸ brush, lanes per number, windows marked    │ caveats        │
│ (kind,      │                                                      │ Ask about this │
│ carrier,    │ table twin (virtualised), one key away                │ notes          │
│ period)     │                                                      │                │
└─────────────┴───────────────────────────────────────────────────────┴────────────────┘
```

**States (all designed, as the contract requires):** empty ("no relationships within the limits", with the limits), too broad (narrowing chips for time, entity, family), abstained (the reason and a repair), partial coverage (the case is not fully typed), engine unavailable, basemap unavailable. **Honest limits stated on screen:** which analyses are exact (A1 to A5, A13) and which are inferred.

**What is dropped from today's pages:** the filter bar as the first thing; Connections and Places as separate pages (they become panes); the "starters" and the data panel on the Ask start (the quality information moves to the Case Overview and the Evidence quality tab). **What is kept:** `connectionsModel.js`, `resultShapes.js`, the carrier badges, the `/analysis/connections` and `/places` reads, the tile cache and offline basemap rules, the link chart's data model.

**Not in the first version:** free-form drawing and annotation on the chart (the i2 style), cross-case views (G-1; needs the authority rules), entity merging (P4's review screen).

### 7.4 The single stage list

Replaces the three lists. Each stage follows the loop in 7.2. Estimates are for the stage's own work; the backend batches land in P1.

| Stage | Scope | Depends on | Gate (acceptance by you on `:4181`) | Weeks |
|---|---|---|---|---|
| **U0** | **Direction proof**: research note; two directions A and B of three screens (the workbench, an Ask answer, the Case Overview); type, density, brand | D23 inputs | You choose a direction; the contract amendment list (if any) is written | 1 |
| **U1** | **Foundations**: tokens (neutrals, one accent, status, evidence strength, chart series), light and dark, type (Latin UI, tabular figures, Urdu), spacing, radius, elevation, motion, icon grammar, ECharts theme; `DESIGN.md` with the linter's contrast check mapped onto the custom properties | D23 | Contrast asserted in tests; the type proof page | 2 |
| **U2** | **Shell and navigation**: header, rail, case header and tabs, command palette, global states (offline, error, 404), density control | U1 | Keyboard path, 4 widths, both themes | 2 |
| **U3** | **The link-analysis workbench** (7.3): graph, map, timeline lanes and calendar as coordinated panes, the entity and edge drawer, the table twin, saved views, export, the states | U1, U2, BR-2 (timeline events, density, clusters) | A walkthrough on a wave-1 judgement-style case: connection, common contact, co-location, device change; each mark opens its rows; the keyboard path; 60 frames a second at the budget | 4 |
| **U4** | **Ask (the core)**: answer anatomy, claim-level citations with hover preview, the derivation in plain words, clarification, conversation (front door), result views by shape with the table one click away, "this answer is wrong" into the triage queue | U1, U2, P1-C | The contract's state list, all first-class; the core loop by keyboard | 3 |
| **U5** | **Case Overview, Cases and Dashboard**: inventory and coverage, quality and data-basis chip, engine tiles, attention feed, directory, new case | U1, U2, BR-1, D30 | No fabricated numbers; every tile opens its rows | 3 |
| **U6** | **Evidence**: inventory, add data with the layout card and import wizard, quality tab, viewers (document, table, image overlays, audio and transcript, video events) | U1, U2, BR-1; media views as P3 delivers | Unaided add-data and open-a-citation | 3 |
| **U7** | **Entity drawer, saved views, notes** everywhere an identifier appears | U3, BR-3 | "Ask about this" from any chip | 2 |
| **U8** | **Activity, Settings, Admin, Reports** | BR-4, P5 slices | Honest states; roles | 2 |
| **U9** | **Media workbench and review desk** (plates, faces candidate cards with the authority banner, audio, video events), with custody on every decision | P3 tracks | Review with reason and undo by new event | with P3 |
| **U10** | **Polish and the usability test**: motion and empty states sweep, accessibility and right-to-left audit, remove the alias layer, an **unaided core-loop test** with the task completion and error rates written first | all | The contract's 13.1 | 2 |

### 7.5 Screen by screen

| Route | Disposition | Notes and dependencies |
|---|---|---|
| `/` Dashboard | redo (U5) | One summary request (row 12); a needs-attention queue (row 13); an engine tile and a data-quality tile; no invented trends |
| `/investigate` (global) | redo (U4) | Multi-case scope (row 11) replaces the per-case stopgap |
| `/cases`, `/cases/new` | redo (U5) | Needs the case record (D30): display name, status, created by |
| `/cases/:id/overview` | redo (U5) | Inventory (`v_evidence_inventory`), coverage, quality, data-basis chip; the dossier work of the interface session is input, not a constraint |
| `/cases/:id/evidence`, `/evidence/:eid` | redo (U6) | Layout chip, quality tab, wizard, viewers with overlays as P3 delivers |
| `/cases/:id/investigate` | redo (U4) | The core screen; fixed order of the contract |
| `/cases/:id/connections` and `/timeline` | **merge into the workbench** (U3) | Chronology, Lanes, Places, Calendar become panes and views of one workspace |
| `/cases/:id/activity`, `/activity` | redo (U8) | Real audit (row 10) and the triage queue |
| `/cases/:id/data` | keep as the private real-data inventory | Real data policy; never in screenshots or cloud tools |
| `/settings` | redo (U8) | Browser data inventory stays; engine and basemap settings for administrators |
| `/design/*` | keep as the component library | Every new component with its states, light, dark, right-to-left |
| **new** `/admin` | U8, P5 | Absent from analyst navigation |
| **new** Reports | U8 (row 17) | Deterministic case brief with evidence appendix |
| **new** Entity-resolution review | P4 | Merge, split, undo with reason |
| **new** Media review desk | U9 | R8.7 |

### 7.6 The direction proof (U0), concretely

* **What you receive:** a private page with two clickable prototypes (A and B) of three screens: the workbench on a generated case (a path highlighted between two numbers, a cell selected, the drawer open on an edge with its evidence rows), an Ask answer (sentence, citations with hover preview, derivation, limitations, follow-ups, an abstention variant), and a Case Overview (inventory, coverage, quality, data-basis chip). Desktop and phone widths; light and dark for A, dark and a light variant for B. **Synthetic data only; no real value.**
* **Direction A, "paper":** warm-neutral working surface, deep ink text, one restrained accent, hairline structure, editorial type hierarchy, a designed dark theme that is deep slate, never black. Reference points: Linear's restraint, Stripe's dense tables, Cambridge Intelligence's chart styling.
* **Direction B, "instrument":** layered charcoal canvas inside the contract's 8.3 (no flat black, no glass, no neon), monochrome with one accent, dense and tactile. Reference points: Linear, Vercel, Palantir Blueprint's density and keyboard consistency.
* **Choices asked in the same sitting:** typeface (Inter, Geist or IBM Plex for Latin; Noto Nastaliq Urdu or Noto Sans Arabic for Urdu, with the line-height consequence), default density (comfortable or compact, with a toggle), keep or evolve the NexusAI mark.
* **What is not decided by the proof:** the stages after U1; they follow the loop.
* **How it is made:** research note first (sources above and the Stitch plan's), then the prototypes in plain HTML with the contract's token names; Stitch may be used for exploration with synthetic content only; the output is a reference, never shipped code.

### 7.7 The work already in flight

* **The interface session's uncommitted redesign** (stages 0, 1 and 3 of the dark directive: tokens, shell, dashboard, case overview) is **kept, committed on its own branch in P0.4 and treated as provisional** until D23. If the chosen direction is B, much of it carries over; if A, its logic and tests carry over and its CSS is replaced.
* **My Connections and Places code** is a reference for the data hooks and models, which stay; its markup is replaced in U3.
* **Nothing in the current app is deleted before its replacement is accepted** (no half-migrated screens).

### 7.8 Definition of done and testing (the tier policy applied to the interface)

Per stage: unit tests for every count, mapping and state (T0, run on the changed files only); **one** Playwright run of the stage's flows on a generated case at the stage gate (not after each edit); the contrast and accessibility checks; four widths and both themes by script; the keyboard path walked; zero console errors and no horizontal overflow at 375 px; a demonstration script; the owner's acceptance; the contract and state file updated. The usability test (U10) is the only long interface test, written before it is run.

### 7.9 Risks

| Risk | Handling |
|---|---|
| Another round of churn | One direction, chosen by looking; no stage starts by editing CSS; one owner per stage |
| Taste mismatch after the proof | The proof offers two directions and the choices; a third can be requested once before U1 |
| The backend lags the design | Honest designed states; the five batches are in P1 and are small |
| Style collisions between sessions | One tree, one stage owner, one editor of the shared styles (D24) |
| Stitch or any cloud tool seeing real data | Synthetic content only (policy Part I of the brief); screenshots only from generated cases |
| The workbench grows into a hairball tool | The working-set rules; budgets measured; "path first" default |
| Interface work starves the engine or the reverse | Separate sessions and files; P1 and P2 gates are independent |

---

## 8. Media families in depth (P3)

### 8.1 Position, constraints and what changes

Structured data is where the product is strong; media is where Pakistani evidence is hardest and where the product's measured numbers are weakest. Five constraints shape every track:

1. **The hardware is the 15.7 GB CPU laptop** (D21: laptop first; the RTX 2080 machine is not available). Heavy models run one at a time, off the live question path, in a batch window; anything that needs a GPU to be useful (large speech models, vision-language captions, fine-tuning) is queued behind P-H and does not block the CPU-feasible work.
2. **The worker's media roles are all off today.** New media uploads are admitted, classified and inventoried but not processed. Each track therefore begins by switching **its own role on, on controlled data, behind the readiness receipt (`processor_readiness.py`) and the resource policy**, and switching it off if the gate fails (D25).
3. **The test sets do not exist yet** for the evidence that matters: Pakistani plates, Urdu lines, Punjabi and Pashto speech, consented faces. They cannot be made by a model. They need the owner's captures, labelling and consent (D26).
4. **Licences block three running components** (YOLOv9-t plate weights, CCT-XS v2 plate-reader weights, SFace) and the video plate stack that is configured but off is AGPL. The replacement route is part of the plate track (D31).
5. **The policy is conservative by design:** model output is `model_observation`, never a fact; a face or voice is never identified; a plate string is not an owner; low confidence abstains; human review is append-only; no evidence enters a third-party service.

### 8.2 Baselines (measured by us; small samples, wide intervals, which is itself the finding)

| Family | Baseline | Source |
|---|---|---|
| Plates on images | detection 20 of 22; exact read 13 of 22 (59%) sealed holdout; 5 of 10 on development | NX-MMR sealed tests, 2026-08-30 |
| Plates on video | 3 of 41 occurrences exact at one frame a second (7%); the V3 pipeline at four frames a second is not yet measured on the same sealed video | same |
| Plate detector candidate (R8.4) | the Paddle text detector on 29 fixtures: precision 0.607, recall 0.944 (fails); Paddle OCR 0.875 normalised exact but misses exactness and CER; Tesseract weaker | R8 expanded evaluation, 2026-08-12 |
| Printed OCR (Latin) | 83% exact on a small English fixture (PaddleOCR PP-OCRv5) | NX-MMR |
| Urdu OCR | 0 of 1 exact, character error rate 0.36, one synthetic line | NX-MMR |
| Speech | Urdu word error 45.5%, English 10.8% on small read-speech sets; no Punjabi or Pashto | NX-MMR |
| Faces | detection only; no recognition claim | policy |
| Derived data on the acceptance case | 865 artifacts: 309 OCR observations, 307 ANPR observations, 115 document passages, 26 image observations, 24 video plate groups, 22 fingerprints, 20 face observations, 18 embeddings, 23 audio observations | live database, 2026-10-21 |

### 8.3 The common foundation (every track uses it)

| Piece | What it is | Why |
|---|---|---|
| **M0.1 Role activation protocol (D25)** | One worker role at a time: readiness receipt (code, role enabled and admitted, worker healthy, resource policy satisfied, backend available, model assets verified by hash) → switch on for the controlled collection → run → measure → keep on only if the gate passes. Heavy models one at a time (`parallel_heavy_models=1`), a RAM floor, timeouts, output caps (existing variables) | Today's roles are off; a family never goes live by accident |
| **M0.2 Evidence tiers and sealed sets** | T0 exact fixtures, T1 synthetic, **T2-V controlled virtual scenes with exact truth (a 96-image plate pack exists)**, T2-P physical (deferred), T3 public licensed, T4 authorised real. Every pack has a generator, a seed, a count and a SHA-256 manifest; a hash mismatch blocks use; development, validation and sealed partitions; **sealed sets are never tuned on**; every acceptance report states `synthetic`, `controlled_demo`, `public_benchmark`, `authorised_real`, `runtime`, `production` separately | `nexusai-evidence-test-and-demo-program.md` |
| **M0.3 Derived-artifact contract** | Every derived row carries model id, hash and parameters, confidence, review state, a **citation locator** (page and box, second, frame and box), `source_truth_state`, and is never merged with ingested records | Already in the 11 contracts; the locator is the missing part for projection (L-Q6) |
| **M0.4 Human review with custody (R8.7)** | `model_candidate`, `human_reviewed`, `externally_verified`; every decision append-only with actor, role, time, prior value, new value, reason, source crop; correction never rewrites the model's output; undo is a new event | Contract; the review desk is stage U9 |
| **M0.5 Media views for the engine (L-M15)** | Migration 022: reviewed views over `derived_artifacts` in the analyst schema, **curated non-text fields only**: `v_media_observations` (one row per observation: artifact id, evidence id, family, kind, start and end seconds or box, confidence, review state, model id, truth state, locator) and per-family views (`v_plate_reads`, `v_transcript_segments`, `v_ocr_regions`, `v_face_detections`, `v_video_events`, `v_image_similarity`); text and plate strings come only through retrieval or the masked alias (D28). Thick views, a verified example each, a bank, the same gate as M2 | The engine reads no derived media today; "which plate was read most often", "how many audio files in Urdu", "which video has the most plate groups" become answerable with the labels "model observation" |
| **M0.6 Retrieval and PII (R5)** | Curated sensitivity applied on every retrieval path; "not found" only after a complete search; the text-browse guard stays; the PII probes at 100% before any text widening | Closes L-Q14 |
| **M0.7 Media bank** | At least 20 questions per family across the modalities, written against the data, with the controlled truth; zero confident-wrong; every derived answer states that it is a model observation | L-Q9 |
| **M0.8 Security** | Hostile inputs (polyglots, decompression bombs, malformed headers, oversized pixels, path traversal), temporary-file cleanup, no network egress from model containers, the red-team set for instructions hidden in OCR text or transcripts (the model sees no rows; tools are read-only) | R8.9; section 9 of the maturity roadmap |

### 8.4 Order, and why

| Order | Track | Why now | Hardware |
|---|---|---|---|
| 1 | **D** documents, printed OCR, retrieval | Cheapest and deterministic; native text exists; feeds the identifier mention index (P4) and the PII boundary the engine needs; no new model class | CPU |
| 2 | **I** general image forensics (deterministic parts) | Hashes, metadata, near-duplicates and similarity candidates need no new accuracy claim about people; quick visible value | CPU |
| 3 | **P** plates on images (R8) | Highest operational value in Pakistan; the lead time is data and licences, so the capture and the detector replacement start in week 3 and run in the background | CPU for inference; fine-tuning short runs on CPU or later GPU |
| 4 | **A** audio and speech | High value, but the model choice is GPU-bound and the gold sets need reviewers; start the gold sets, the pipeline (VAD, normalisation, spoken numbers) and CPU baselines now; the bake-off waits for P-H | CPU baseline; GPU for large models and fine-tuning |
| 5 | **O** Urdu and Pashto OCR | A bake-off on a set that must be built; CPU-feasible recognisers first | CPU; VLM on GPU |
| 6 | **V** video | Depends on P (the reader), A (the audio track) and tracking; heavy | CPU slow; GPU for captions |
| 7 | **F** faces | Policy first (D5); only L0 is built; the evaluation set and counsel review start early because they have long lead times | CPU for L0 |
| 8 | **X** captures, databases, archives | Inventory exists; bounded querying and message-database extraction come with P4's message-export family | CPU |

**Recommendation (D25):** this order, with the data and licence work for P, A, O and F started in week 1 so their lead times overlap tracks D and I.

### 8.5 Track D: documents, printed OCR and retrieval (R13)

| | |
|---|---|
| **Now** | Native text passages (115 on the acceptance case, about 1,900 across the eight generated cases), retrieval with citations (full text, trigram, Qwen3-Embedding-0.6B), a document viewer; PaddleOCR PP-OCRv5 and Tesseract exist; scanned PDFs not validated; the OCR role is off |
| **Target** | Native extraction first, scanned pages and images through **Docling** (MIT per the maturity roadmap) with OCR fallback; tables, reading order, attachments, encryption and corruption states, page and box citations; **identifier extraction** with validators (the 10 to 16 digit identifiers at least 95% exact on clean print); **BGE-M3** (MIT) embeddings and a cross-encoder reranker (licence to verify) with Urdu-aware normalisation (zero-width characters, Arabic and Persian letter forms, Arabic-Indic digits); forms and ID documents (CNIC pattern, IBAN checksum, dates) **only with case authority** |
| **Data** | Generated documents in the wave-1 cases (T1), controlled virtual scenes (T2-V), small public slices after the ledger and D4 |
| **Gate (proposed, fixed after the baseline)** | Character error per script and domain; identifier exact match at least 95% on clean print; retrieval recall at k on a native-reviewed set; PII probes 100%; every OCR region cited with page and box; low-confidence regions flagged, never dropped |
| **Engine and views** | Retrieval is the path for "what does it say"; `v_document_passages` (counts, pages, languages, extraction state; **no text**) answers "how many pages", "which files are scanned" |
| **Viewer** | Page rail, OCR overlay, tables, reading order, search matches with deep links from any answer, extraction warnings |
| **Risks** | OCR text is untrusted input to the model (instruction injection): the red-team set; PII in passages: the guard |

### 8.6 Track I: general image forensics (R9, deterministic parts first)

| | |
|---|---|
| **Now** | Image observation (technical metadata), fingerprint (perceptual hash, 22), SigLIP-base embedding (18, role off), `GET /images/similar` |
| **Target** | **Metadata:** EXIF and GPS extracted under a policy (location is sensitive: kept with its method, shown with uncertainty, stripped from exports unless the case enables it); **hashes:** exact SHA-256 plus a perceptual family (**PDQ**, 256-bit, with the Hamming rule stated; the ThreatExchange repository states BSD, the PDQ directory's own licence is to be read before use; pHash as the simple baseline) and near-duplicate clusters labelled "near-duplicate, not the same image"; **similarity candidates** by SigLIP embeddings, always "candidate, review required"; **screenshot and document-image detection** that routes to track D; **object detection** limited to a fixed vocabulary with an Apache-2.0 detector (RF-DETR Nano to Large are Apache-2.0; checkpoints to be checked individually; avoid AGPL YOLO code) |
| **Data** | A near-duplicate set built by transformations (crop, JPEG, resize, rotate, mirror, overlay) with known pairs (T1), a metadata fixture set, public slices later |
| **Gate** | Near-duplicate precision and recall at stated distances on the built set; metadata extraction exact on fixtures; **no identity claim anywhere**; CPU latency recorded |
| **Engine and views** | `v_image_observations`, `v_image_similarity` (pair, distance, rule) |
| **Viewer** | Image with a metadata panel, side-by-side comparison of candidates, explicit labels |

### 8.7 Track P: plates on images (R8, to Boundary A)

| | |
|---|---|
| **Now** | Section 8.2; R8.1 to R8.4 done; R8.5 to R8.10 blocked at Boundary A; the detector (YOLOv9-t weights) and reader (CCT-XS v2 weights) have unresolved licences; a P-LPCD dataset record conflicts on CC BY versus CC BY-NC; UFPR and RodoSol are non-commercial |
| **Target (proposed)** | Exact read at least 90% on the sealed image set with the lower 95% bound at least 85%; detection recall at least 98%; calibration (confidence at least 0.9 gives accuracy at least 95%); abstention explicit; R8's thresholds unchanged |
| **Detector** | Replace with a licence-clean detector: **RF-DETR Nano to Large (Apache-2.0)** fine-tuned on Pakistani crops, YOLOX (Apache-2.0) as the challenger; the Ultralytics and YOLO family are AGPL and stay out of the product. Decision rule written first: recall at IoU 0.5, then false positives per hour, then latency |
| **Reader** | Fine-tune a compact recogniser on Pakistani crops with province and series grammar-constrained decoding, keep the top candidates with confidence; challenger: a fine-tuned PaddleOCR recogniser; Pakistani plates are non-standard in size, style and font (a 2021 YOLO plus Tesseract pipeline reached 73% average accuracy), which is why the Pakistani data matters |
| **Data** | The 96-image T2-V pack (exact truth, partitions); the owner-authorised capture protocol (T2-P, deferred) or a consented capture campaign (D26); a Pakistani set (the 2021 PLPD paper describes one; its licence and location must be asked of the authors); public sets only for what their licences allow (CCPD MIT for robustness; Artificial Mercosur and SYNLIP CC BY as supplements) |
| **Linkage (R8.6)** | Raw OCR beside deterministic uppercase and separator normalisation; regional format checks that never fill missing characters; a link to structured ANPR **only** by exact normalised plate plus authorised case, time and camera criteria, with zero, one or many match states; **confusion-aware matching** (M and W, 0 and O) ranked by visual edit cost and labelled candidate |
| **Review (R8.7) and Ask (R8.8)** | The review desk with custody (U9); the ANPR specialist extended, never duplicated; typed presentation with the source image, crop, candidate state, confidence and structured matches; a plate string never implies an owner, a driver or a route |
| **Plate masking (D28)** | Search-only today. Options: keep search-only, or ship the case-scoped HMAC alias with its key required (plate strings are low entropy, so an unkeyed digest is reversible) and masked fields not sortable; the decision also re-rules H1 |
| **Gate** | Boundary A then B as in the R8 contract; the media bank for plates; PII probes |
| **Views** | `v_plate_reads` (alias or search-only text, confidences, review state, box, frame) |

### 8.8 Track A: audio and speech (R10)

| | |
|---|---|
| **Now** | faster-whisper small through the model container; Roman Urdu rendering; transcript artifacts with timestamps; the role is off; Urdu 45.5% and English 10.8% word error on two small sets; Punjabi and Pashto unsupported |
| **What the research says (2026-10-21; verify before relying on any number)** | No trusted public figure for a fine-tuned Whisper large-v3 on Urdu was found; a 2026 Interspeech paper reports SeamlessM4T best on Urdu (16.3%) and Punjabi (22.1%) FLEURS word error but its weights are CC BY-NC (evaluation only) and finds script confusion drives most Whisper failures ([Interspeech 2026](https://aghaaliraza.com/publications/2026--Interspeech--Dissecting_ASR_Failures_in_Low_Resource_South_Asian_Languages.pdf)); a benchmark of Urdu systems found Whisper-large best on conversational speech ([WER We Stand](https://arxiv.org/pdf/2409.11252)); small Whisper zero-shot on Urdu 33.7% word error ([lightweight Whisper](https://arxiv.org/pdf/2508.09865)); a Whisper benchmark covers Pashto, Punjabi and Urdu with few-shot gains ([CHiPSAL](https://preview.aclanthology.org/credits/2025.chipsal-1.20)); Omnilingual ASR (Meta, 2025, Apache-2.0) covers 1,600 languages but no per-language Urdu, Punjabi or Pashto figure was found. **Conclusion: our own gold sets decide** |
| **Gold sets first** | Per language (English with Pakistani accent, Urdu, Punjabi in Shahmukhi, Pashto; then Saraiki, Sindhi, Balochi, Hindko; Urdu and English code-switching, Roman Urdu): 2 to 5 hours of read speech, conversation and **telephone-band audio (8 kHz with phone codecs applied to clean audio, plus real call audio only under authority)**, with noise; labelled by named native speakers (D10) with written normalisation rules |
| **Pipeline** | Normalisation; **VAD** (Silero VAD: MIT per the official licence text, supports 8 kHz, packaged copies differ so the exact artifact is checked); **language identification** per segment (MMS-LID or VoxLingua107, licences to verify, because Whisper's language label does not work for Pashto); ASR with timestamps; **diarization** (pyannote Community-1 is CC BY 4.0 behind an access gate and expects 16 kHz, so 8 kHz calls are upsampled and tested; DiariZen as the open challenger, 13.3% against 11.2% for the commercial one on a 2026 mixed-audio benchmark, [ICML 2026](https://icml.cc/virtual/2026/77326)); Unicode normalisation; **spoken-number conversion** (digit words to digits in Urdu, Punjabi, Pashto); identifier extraction for P4; a script-conversion rule layer for known confusions |
| **Bake-off under a rule written first** | Current small, Whisper large-v3 and turbo (zero-shot and LoRA), Omnilingual ASR (300M to 7B), IndicConformer (MIT per its page), a fine-tuned XLS-R; SeamlessM4T and MMS large for evaluation only. Metrics: word and character error with the written normalisation, **digit-sequence exact match**, share of output in the expected script, repetition and hallucination rate, timestamp error, real-time factor on the target hardware. Fine-tuning only after a measured gap, LoRA adapters served beside the base model, switchable off, never trained on test sets |
| **Out of scope** | **Voice identification** (the earlier decision stands); text to speech (blocked) |
| **Gate (proposed, after the baseline)** | Per language word error at or below a target set from the baseline; digit-sequence exact match at least 90% on clean speech with telephone-band reported separately; correct script at least 99%; hallucination loops at most 1% of files; every segment cited with start and end |
| **Hardware** | CPU baselines now (small and medium, int8); large models and fine-tuning wait for P-H; real-time factor decides what is usable on the laptop |
| **Views and viewer** | `v_transcript_segments` (timing, language, confidence, speaker turn; **text withheld**, retrieved through the retrieval path with the PII rule); waveform, synchronised transcript, speaker turns, quality warnings (the existing viewer logic stays) |

### 8.9 Track O: Urdu and Pashto OCR

| | |
|---|---|
| **Now** | Urdu 0 of 1 exact, character error 0.36; PaddleOCR Arabic and Tesseract `urd` exist; Pashto absent |
| **Research (verify)** | On the Urdu Newspaper Benchmark (829 paragraph images, LREC 2026) the best classic system was TrOCR at word error 0.42 and character error 0.159 against Tesseract the weakest; a hosted frontier model reached 0.133 word error but cannot be used (data would leave the premises); fine-tuning a frontier model on 500 in-domain samples gained 6 points; a super-resolution step raised LLM recognition by 25 to 70% ([Press to Pixels](https://arxiv.org/pdf/2505.13943), [LREC 2026](https://aclanthology.org/2026.lrec-1.235/)); a PARSeq model on about 160,000 Urdu images reached character error 0.178; **no source evaluates PaddleOCR-VL or Qwen3-VL on Urdu**, so we measure |
| **Plan** | Build a labelled Urdu set (500 to 1,000 lines: newspapers, screenshots, social posts, documents, with Latin digits mixed in) from licence-clean sources plus synthetic Nastaliq from openly licensed fonts; bake-off: Tesseract (floor), PP-OCRv5 Arabic, fine-tuned TrOCR and PARSeq, a Nastaliq-specific recogniser, a small vision-language model (GPU); decision rule first; fine-tune a recogniser on synthetic plus real lines; Pashto: evaluation first (public sets' licences unknown), fine-tune only after a measured gap |
| **Gate** | Character error per script and domain after the baseline; digit-sequence exact match at least 95% on clean print; low-confidence regions flagged |

### 8.10 Track V: video (R11)

| | |
|---|---|
| **Now** | ONNX V3 plate reading with plate groups (24 on the acceptance case), ffprobe, frame sampling, audio extraction; v2 (AGPL stack) configured and off; 3 of 41 occurrences at one frame a second on the old policy |
| **Target** | Adaptive sampling (four frames a second or more on motion), shot boundaries, duplicate-frame removal; **tracking** so one vehicle is one track (ByteTrack first, BoT-SORT for moving cameras; the Apache-2.0 [Roboflow Trackers](https://trackers.roboflow.com/develop/details/) library is the candidate, its licence file to be read; the Ultralytics tracker code is AGPL and excluded); the sharpest crop per track and **character voting** across the track; OCR on frames for signs and overlays; the audio track through track A, linked to frame times; **captions** from a vision-language model (Qwen3-VL has Apache-2.0 model cards and a technical report with video timestamps, [Qwen3-VL](https://arxiv.org/pdf/2511.21631); useful speed needs the GPU) stored as "model description, unreviewed", searchable, never counted; one event timeline per video; clip creation; duplicate-video hashing (TMK with PDQ frames or vPDQ, licences to read) |
| **Data** | T2-V virtual scenes with exact truth, owner-authorised capture, public slices after the ledger; a sealed labelled video set |
| **Gate (proposed)** | Event recall on the sealed set; track-level exact read at least 85% with false plates per hour reported; every claim with a frame and box; bounded resources; **no whole-video unbounded model call** |
| **Views and viewer** | `v_video_events`; player with an event scrubber, frames, clips, search results mapped to time |

### 8.11 Track F: faces (policy first)

| | |
|---|---|
| **Levels** | **L0** detection and quality (always allowed, no embedding stored); **L1** within-evidence clustering ("may show the same face", reviewer confirms); **L2** gallery search (1 to N against an analyst-supplied set, ranked candidates with similarity and the error rate at that threshold, "candidate, review required", no name unless an analyst assigned one); **L3** large watchlists out of scope. Each level off by default, enabled per case with recorded authority (P5's authority record) |
| **Now** | L0 detection (YuNet); SFace comparison exists behind `GET /faces/similar`; the face role is off |
| **Models and licences** | YuNet MIT; SFace labelled Apache-2.0 but its training data provenance is unresolved (reported CASIA-WebFace); InsightFace model packs need a commercial licence ([InsightFace licensing](https://www.insightface.ai/solutions/face-recognition-licensing)); AdaFace and EdgeFace non-commercial; **AuraFace-v1 (fal) lists Apache-2.0 on its model card, a ResNet100 ArcFace-style model trained on commercially and publicly available data, with a stated caveat that ethnic coverage may be incomplete** ([model card](https://huggingface.co/fal/AuraFace-v1/blob/main/README.md)): the candidate to evaluate if D5 ever moves to L1 or L2, after reading the file and the training-data statement |
| **Evaluation before any L2 use** | A consented South Asian set with CCTV-like degradation, masks, pose and age spread; ROC; false non-match rate at false match rates 1e-3 and 1e-4; precision at k; per-group error rates; thresholds calibrated on a held-out set; **a negative control** (two different people that the model scores high must not produce an identity claim) |
| **Integration rule** | A face link between two images is a `candidate` edge with its score; **link analysis uses only analyst-confirmed face links** |
| **What is built now** | Nothing beyond L0 and the review-card design; **D5 stays at detection only** (recommended); the evaluation set and the counsel review start early because they take the longest |

### 8.12 Track X: captures, databases and archives (R14; D27)

| | |
|---|---|
| **Now** | Admission and inventory: PCAP, PCAPNG, SQLite, PDF and archive inventories and MPEG headers were accepted in 2026-07; EVTX is classified; nothing is executed |
| **Target (inventory first)** | Safe listing with limits on member count, depth, ratio and size; hashing; **parent-child lineage** for archive members registered as child evidence only within the limits; SQLite opened read-only on a copy with table list, schema and counts, then **bounded SELECTs through the same validator pattern** over reviewed views of an extracted copy (message-app databases become the **message-export family** of P4); PCAP flow summaries (addresses, ports, protocol, time, DNS names, TLS server names; **no payload**); EVTX event counts by id, time and account; parser libraries chosen by licence ledger row |
| **Gate** | Exact inventory; no execution, no path traversal, no decompression escape; unsupported members preserved or explicitly rejected; a hostile-input suite (zip bombs, traversal names, malicious SQLite files, polyglots) in the tests |
| **Interface** | An inventory tree with parent-child, a bounded table browser for extracted databases, the capture summary |

### 8.13 Test sets to build (the owner's part is the labelling and consent)

| Set | Family | Size to start | Tier | Who |
|---|---|---|---|---|
| Pakistani plate set with sealed holdout | P, V | at least 300 sealed plate tracks across day, night, rain, blur, angle, frame-level boxes | T2-P or T4, T2-V for the rest | owner captures under authority; a named annotator and a second reviewer |
| Urdu OCR lines | O, D | 500 to 1,000 | T1 plus real lines | native reviewers |
| Speech gold sets with telephone-band variants | A | 2 to 5 hours per language | T1 and T3 | native reviewers; recordings with consent |
| Consented face set | F | with degradations | consent | volunteers (only if D5 moves) |
| Near-duplicate and metadata sets | I | built by transformations | T0, T1 | generated by us |
| Document scans and forms | D | 100 pages with identifiers | T1, T2-V | generated plus reviewed |
| Sealed labelled video set | V | a few hours | T2-V, T2-P | owner capture later |
| Hostile-input set for captures and archives | X | 50 files | T0 | generated by us |
| Media bank | all | at least 20 questions per family | | written against the data |
| Red team: instructions hidden in OCR and transcripts, SQL escape, PII extraction | all | 100 | T0 | generated by us |

### 8.14 Gates, in one table (proposed; each is fixed in a pre-registration before the first measurement and after the baseline where the baseline is unknown)

| Track | Gate |
|---|---|
| D | Character error per script and domain; identifier exact match at least 95%; PII probes 100%; every region cited |
| I | Near-duplicate precision and recall; metadata exact; no identity claim |
| P | R8 Boundary A, then B; exact read at least 90% (lower bound at least 85%); recall at least 98%; calibration |
| A | Per-language word error target; digit-sequence exact at least 90%; script at least 99%; loops at most 1% |
| O | Character error per script and domain; digits exact at least 95%; low confidence flagged |
| V | Event recall; track exact read at least 85%; every claim with frame and box; bounded resources |
| F | L0: no recognition claim; L1 and L2 only after D5, the set, counsel; negative control passes |
| X | Exact inventory; no execution, traversal or escape |
| All | The media bank at zero confident-wrong; every derived answer labelled "model observation"; sealed sets untouched by tuning |

### 8.15 What is not done, on purpose

Identification of a person from a face or a voice; any model output shown as a fact; downloads without a ledger row, a written decision rule and your yes; real files in cloud tools; tuning on sealed sets; AGPL stacks in the product; merging derived and ingested rows; text to speech as evidence; a plate string read as an owner, a driver or a route.

---

## 9. Evaluation and testing: the policy that keeps the cycle short

### 9.1 The tiers (already in `tools/dev/README.md`; applied to every programme)

| Tier | What | Time | When |
|---|---|---|---|
| T0 | Unit and live-database specs of what changed (Go engine, registry, typed ingest, judge, factory, interface logic) | 1 to 3 min | every change; `tools/dev/check_change.py`; automatic in the pre-commit hook for Go |
| T1 | Typing report of a collection (counts and shares; what the engine could be misled by) | seconds | after any layout, typing or back-fill change, **before** any question |
| T2 | Live smoke (engine health, a template request writes no audit row, the IMEI check, the cell-site sentence, connection answers; `--real` for the private collection) | about 5 min | after a deploy |
| T3 | Re-ask only what failed or what the change can touch; re-judge saved answers on the effective views with no model | 1 to 3 min | after a fix |
| T4 | Milestone runs: full banks and spreads on fresh seeds, gates written first | 20 to 70 min | once per milestone, **in the background**, read afterwards |

### 9.2 Seeds, cases and gates

* **Generated cases:** 8 development (loaded as wave 1: `nexusai-wave1-devN-20261021`, free to develop and demonstrate on), 8 judgement (101 to 108; **101, 102, 103, 104, 106, 107, 108 are spent; 105 is unspent**; judgement cases are never loaded where a developer or a demonstration can see them), 4 sealed (never used for an engine gate).
* **Rules that stay:** gates written and committed before a run; a fix measured on another fresh seed; a prompt-hash comparison to show what a change can reach; mutation tests that prove a check catches its fault; the oracle's and the judge's own defects reported; a timeout is never a verdict; the switch state asserted from the container before scoring; a probe must reproduce the path, not only the function.
* **Banks:** the link-analysis bank (stages 1 and 2 done; stage 3 in P1-A), the factory spreads, the 103 corpus and the 38 pre-flight (regression for the older paths), the **media bank** (M0.7), the **investigator set E1** (about 50 questions, a wrong answer costs more than a refusal), the multilingual sets (PL), the red team (96 statements and 18 question-level attacks today; the evidence-text injection set is new).

### 9.3 How the waiting is cut (the owner's request, 2026-10-20 and 21)

1. No per-change spread. A change is checked by the cheapest tier that can catch it.
2. After a data change, the typing report first; most surprises of 2026-10-20 were visible there in seconds.
3. A typed case is judged against keys read from the effective views, not version 1's table.
4. A run that hits a limit re-runs only what did not run (the Ginkgo suite limit is now 6 h).
5. Heavy runs only at milestones, in the background, on an idle machine; the model is one CPU process, so no builds or tests during a timed run.
6. Interface: one Playwright run per stage at the gate, not per edit.
7. Media: every track is measured on its controlled pack first; a real-shaped or authorised run only at the track's gate.

### 9.4 What every programme step reports (the directive's execution contract, shortened)

`planned`, `implemented`, `tested` (tier), `deployed` (image and rollback tag), `runtime accepted`, `data accepted`, `remaining`; and, at each gate, **a repeatable demonstration**: route or script, exact questions and clicks, expected results, the data used, the limitation, the deployment state and the rollback.

---

## 10. Governance, security and licences (consolidated)

| Area | State and plan |
|---|---|
| **Authority and legal** | A `case_authority` record per case (legal basis and reference, who authorised, which sensitive capabilities are enabled) is built in P5; **off by default:** face L1 and L2, voice identification, tower-dump and geofence analyses (A6). Tower-dump and geofence law is contested; Pakistan-qualified counsel decides; the product builds the gate, not the legal conclusion |
| **Identity and roles** | One shared actor today. P5: per-user sign-in carried through the proxy; analyst, reviewer, administrator; optional second-person approval for a sensitive capability. **D30** adds a minimal case record earlier |
| **Audit** | One audit row per engine question (live), custody chain for evidence (live); extend to exports, sensitive views and reviews; hash-chained append-only as the custody chain |
| **Data protection** | Field-level masking for names, CNIC, URLs by case policy; encryption at rest and in transit; retention and disposal set with counsel (no universal schedule is encoded) |
| **Isolation** | RLS by tenant (live); the read-only role reaches only analyst views (live); rate limit and per-user quotas (P5); cross-case views only with the authority rules (G-1) |
| **Model supply chain** | Pin file hashes; the licence ledger row before any download; no network egress from model containers; model id, hash and parameters on every derived artifact |
| **Prompt injection** | Evidence text can contain instructions; tools are read-only, scope is server-set, the model sees no rows (except D17's masked, capped result); the injection red-team set is built in P3 |
| **Real data** | Vault outside every repository, container context and cloud-synced folder; authority recorded per file; counts and booleans only; the scanner runs before every commit; synthetic goldens are shape-preserving from reserved ranges |
| **Change control** | "Ask the owner first" for: rebuild or redeploy, compose actions beyond `ps` and `logs`, any database write, migration or backfill, model downloads or backend installs, git writes beyond a plain push to the working branch, long builds, live evaluation suites, retained evidence, worker role activation. A rollback tag before every deploy; the API is recreated only with the Run-Arms helpers |

**Licence blockers and what research on 2026-10-21 changes** (the ledger is `docs/compliance/licence-ledger.csv`; each row needs the file opened):

| Component | Today | Route |
|---|---|---|
| YOLOv9-t plate weights, CCT-XS v2 reader weights | running, terms unresolved | Replace the detector with **RF-DETR Nano to Large (Apache-2.0; XL and 2XL are under a separate licence)** trained on our data; fine-tune our own reader; counsel reads the checkpoints in the meantime (**D31**) ([RF-DETR licence](https://blog.roboflow.com/rf-detr-is-free-to-use-commercially/)) |
| Ultralytics YOLO and its trackers (video v2) | configured, off, AGPL | Stay out of the product; ByteTrack-class tracker from Apache-2.0 Roboflow Trackers (read the licence file) |
| SFace | running behind `/faces/similar`, provenance unresolved | Keep at L0 (detection by YuNet); no comparison exposed until D5; evaluate **AuraFace-v1 (Apache-2.0 card)** if D5 moves |
| InsightFace models, AdaFace, EdgeFace | blocked | Commercial licence or research only |
| SeamlessM4T, MMS large | CC BY-NC | Evaluation only |
| Silero VAD | MIT per the official text; some packaged copies differ | Read the exact artifact |
| pyannote Community-1 | CC BY 4.0, gated | Accept terms; test at 8 kHz; DiariZen as challenger (licence unknown) |
| PDQ / TMK (Meta) | repository states BSD; the PDQ directory's own text to read | Read before use |
| Qwen3-VL | Apache-2.0 on model cards | Confirm on the official repository |
| Datasets: UFPR-ALPR, RodoSol (non-commercial), P-LPCD (conflict), the Pakistani PLPD set (licence unknown) | not downloaded | Evaluation only, or ask the authors |

---

## 11. Decisions only you can make

Existing decisions that are still open keep their numbers; **D23 to D32 are new**. "Recommended" is my recommendation, not a decision.

| # | Decision | Why now | Recommended |
|---|---|---|---|
| **D23** | **One interface direction**: A (light-first "paper", a designed dark theme), B (dark "instrument"), or a third; plus typeface, density and brand | Everything in P2 waits for it; three directions are in the repository | **Decide after the direction proof (U0, week 1), by looking.** My lean is A: it is closest to the UX contract's rules and to "clean, elegant, not techy"; B is acceptable inside the contract's 8.3. Either way one direction only |
| **D24** | **One interface owner per stage; one canonical tree** (the lane worktree); one editor of the shared styles; the other session's work committed on its own branch | Two trees, copied by hand; a collision already cost one overwrite | Yes |
| **D25** | **Media activation order and role switching**: D, I, P, A, O, V, F, X; one worker role on at a time, on controlled data, switched off if the gate fails | The roles are all off; each needs your go to switch on | Yes, in this order, with the data and licence work for P, A, O, F starting in week 1 |
| **D26** | **Who captures and labels**: named people for plate captures (authorised, with the protocol), plate and Urdu labelling, speech recording with consent, the 50 investigator questions (E1); which datasets to download first (D4, per item) | The test sets cannot be made by a model; their lead time is the critical path of P3 | Name people in week 1; download nothing until its ledger row and a written decision rule exist |
| **D27** | **R14 scope**: inventory first (safe listing, hashing, lineage, bounded read-only queries), no extraction of members without limits, message databases through P4 | The directive asks for full analysis; the safe part is cheap | Inventory first |
| **D28** | **Plate masking**: keep search-only, or the keyed case-scoped alias (which also re-rules H1) | Blocks the plate views of M0.5 | Keep search-only until the plate track reaches Boundary A, then decide |
| **D29** | **New families**: message exports and location history; their priority against the typed writers | They link directly to phone entities | After the typed writers (P1-B); message exports first |
| **D30** | **A minimal case record now** (display name, status, created by, created at; no membership yet) | Rows 18, 23 and most of the interface need it | Yes, as migration 021 with preflight, verify and rollback, applied on your go after a backup |
| **D31** | **Licence route for the three running components** (sections 8.1 and 10) | Ship-blocking rows | Replace the detector (RF-DETR); fine-tune our own reader; keep faces at L0; counsel reads the checkpoints |
| **D32** | **The trailers (G53)**: rewrite the eleven or more pushed commits (a force-push) or leave them with a note | AGENTS.md forbids the trailer; later commits are right | **Do not force-push.** Leave them, record it, keep the correct trailers from now on; rewrite only on your explicit word after a backup |
| D3 | Measure the RTX 2080 | Optional under D21; not available | Left; revisit when free |
| D5 | Face policy | Highest legal and reputational risk | Stay at L0; start the counsel review and the evaluation set |
| D6 | Conversation front door on | Greetings and help return errors | Yes, behind its gates (P1-C) |
| D8 | Evidence inventory page | The engine view exists; the page is U5 | Yes |
| D9 | More real samples under authority (subscribers, cell lists, other operators, plate images, call audio) and operator documentation for direction codes | Layouts and models need real shapes | Provide what is lawful to share |
| D10 | **Reviewer names** for Urdu, Punjabi (Shahmukhi), Pashto, Roman Urdu | Gates PL, the speech and OCR gold sets | **Names**; the lead time is the longest of any item |
| D11 | Scope of tower-dump and geofence analyses | Contested law | Build with authority gating, off by default (P4, P5) |
| D12 | Priority if capacity is limited | Parallel work competes | Structured and link analysis first (P1), the interface (P2) in parallel, then media (P3) |
| D13/D20 | One repository history, step by step | G51 | Yes (P0.4) |
| D2b | PostGIS | Polygons, line of travel, ellipsoid distance | Defer; `earthdistance` covers radius lookups |

---

## 12. Risks, stated plainly

| # | Risk | Likelihood and effect | Handling |
|---|---|---|---|
| 1 | **Lost work in the main checkout** (574 uncommitted paths, 176 commits behind the pushed branch) | A reset, a checkout or a disk event loses interface work that exists nowhere else | P0.1 archives it first; P0.3 classifies by content; nothing is reset until the archive and the commit exist |
| 2 | **Media accuracy cannot be proven without data that does not exist** | The most likely cause of P3 slipping | D26 in week 1; controlled T2-V first; never claim beyond a tier |
| 3 | **CPU speed** (p50 30 s, recursive 40 s; speech and video heavier) | Slow demonstrations and slow tests | Thick views, batch windows, the tier policy; P-H when the machine is free |
| 4 | **Interface churn repeats** | A fourth direction, a fifth rewrite | One direction chosen by looking, one owner per stage, the proof before code |
| 5 | **Licence exposure** (three running components, AGPL video stack, non-commercial models) | A shipping blocker found late | The ledger gate before download; D31; replacements started early |
| 6 | **Legal exposure** (tower-dump, geofence, faces, voice) | Reputational and legal | Authority record, off by default, counsel, L0 for faces |
| 7 | **A model's explanation states a number it did not compute** (M5) | A confident wrong sentence | The number lock; the deterministic sentence as the fallback; red-team and evaluation gates |
| 8 | **Real data leaks** into a cloud tool, a screenshot or the repository | Policy breach | Synthetic only for Stitch and screenshots; the scanner before every commit; counts and booleans |
| 9 | **Scope explosion** (nine media tracks, three programmes, 30 requests) | Nothing finishes | Gates, an order, one live change at a time, the tier policy; each track can stop at its gate |
| 10 | **Owner bandwidth for labelling and review** | Gold sets late | Name people early; start with generated sets; ask for small, high-value labelling first |
| 11 | **Two code owners editing the same files** | Overwrites | D24; the canonical tree; requests instead of edits |
| 12 | **Testing time creeps back** | The cycle slows | The tier policy is a rule; heavy runs only at milestones |
| 13 | **Disk and backups** (many images and dumps; one restore never rehearsed) | A failed restore when needed | P0.5 and P0.6 |

---

## 13. What to do first

### 13.1 The next 14 days (2026-10-21 to 2026-11-04; an estimate, re-planned at the gate)

| Days | Work | Output | Needs you |
|---|---|---|---|
| 0 to 1 | **P0.1 to P0.3**: archive the main checkout's uncommitted state; classify the 574 paths by content; commit this plan, the rewritten state file and the AGENTS.md pointer | An archive; a "unique to main" list; the plan of record in the repository | |
| 1 to 3 | **U0 research note and the direction proof** (two directions, three screens, synthetic data) | A private page to look at | **D23 inputs** |
| 1 to 3 | **Decisions** D24, D25, D26, D30, D31 | Written decisions | **Yes** |
| 1 to 5 | **P1-A pre-registration**: gates for A5 at 15 minutes, A7, A9 to A11, free-word and month-against-month; reference SQL and oracle specs written before the views | Gates committed | |
| 2 to 5 | **Licence ledger rows** for RF-DETR, Docling, BGE-M3, PDQ, Silero, pyannote, Roboflow Trackers, AuraFace, Qwen3-VL (no downloads); readiness receipts for tracks D and I | Ledger updated | |
| 2 to 6 | **P0.4 consolidation** (the step-by-step merge): commit the interface session's unique files on a branch, reconcile the eleven files, serve `:4181` from the canonical tree | One history, one tree | **A go for the merge steps** |
| 3 to 7 | **P1-D BR-1 design**: migration 021 drafted with preflight, verify and rollback (case record; **and the typed tables for access logs and transactions**); the fixed reads (census, health, summary, attention, directory) with specs equal to the SQL truth; a throwaway-database rehearsal | Files for your approval; nothing live | |
| 3 to 7 | **P1-B first family**: ANPR sightings layout and golden, typed writer, `v_plate_sightings` view draft | Tests green; not applied | |
| 5 to 10 | **You choose the direction** (D23) → **U1 foundations** start; the interface session is told the one direction and the one tree | Tokens and type proof page | D23 |
| 5 to 10 | **P0.5 restore rehearsal**; **P0.6 disk report**; **P0.7** retained-copy repair plan | Reports | a go for the repair |
| 7 to 14 | **P1-A stage 3 build and development-case measurement** (A5 at 15 minutes, A7), then one fresh-seed read; T4 in the background | The first A7 answers from model-written SQL, failures listed | |
| 7 to 14 | **Migration 022 draft**: the media views (M0.5) over the acceptance case's derived artifacts, with examples and the labels "model observation"; activation of **no** media role yet | Draft and specs | |
| 10 to 14 | **P3 track D kickoff** (needs D25 and a yes for each download): Docling and BGE-M3 evaluated on generated documents | A baseline | downloads per item |
| 14 | Review: what worked, the gates, the repair and abstention counts, latency; re-plan | One page | |

**Gate for these 14 days (proposed):** one repository history and the plan of record in it; the direction chosen and U1 started; the P1-A gates committed before any view is built; migration 021 and 022 drafted and rehearsed on a throwaway database with nothing live; no more than one live change.

### 13.2 What I will not do without your word

Apply a migration; recreate a container; switch a worker role on; download a model or dataset; merge or reset a checkout; force-push; delete an image or backup; touch real data beyond counts and booleans.

---

## 14. What changes in the other documents (supersession map)

| Document | Change |
|---|---|
| `NEXUSAI_CONTINUATION.md` | Backend part rewritten to the current state (cap 300 lines); the `CODEX_UI_TRACK` section is the interface session's and was not edited; both point to this plan |
| `AGENTS.md` | Pointer table gains this plan and a one-line precedence note; the three rules and the trailer policy unchanged |
| `docs/roadmap/nexusai-maturity-roadmap-20261012.md` | Top note: this plan sequences; the roadmap remains the design reference and the register of G1 to G53 and D1 to D22 |
| `docs/roadmap/reconciliation-20261020.md` | Top note: S1 to S5 done, S6 to S8 carried as P2, P1 and P0 here |
| `docs/work/HANDOFF_LOCAL_20261007.md` | A line pointing here; its live-state facts stay current |
| `docs/ux/redesign-2026-10/README.md` (main checkout, uncommitted) and `docs/work/UI_UX_MASTER_REDESIGN_PROMPT.md` (uncommitted) | Superseded as stage lists by section 7 once D23 is decided; kept as inputs; their working laws are adopted |
| `docs/work/BACKEND_REQUESTS.md`, `BACKEND_ROADMAP_GAPS.md` | Add a pointer to the five batches (4.4); rows keep their numbers |
| `docs/work/ROADMAP_20260929.md`, `RUNTIME_QUERY_PLAN`, `NEXUSAI_PROGRAMME`, `CODEX_BACKLOG`, `REMAINING_STAGES`, `MEDIA_QUERY_ROADMAP` | Already superseded; the ledger (4.1) holds their open items |

---

## 15. What was read, and what was not

**Read in full:** `NEXUSAI_CONTINUATION.md`, `AGENTS.md`, `BACKEND_ROADMAP_GAPS.md`, `BACKEND_REQUESTS.md`, the maturity roadmap (sections 1, 2, 5, 6.1 to 6.8.13, 7, 8, 9, 10, 11, 12), `reconciliation-20261020.md`, the handoff (from 2026-10-07), `ask-engine-v2.md` (the live and next parts), `ROADMAP_20260929`, `ROADMAP_FREE_QUESTION_20261005`, `REMAINING_STAGES`, `MEDIA_QUERY_ROADMAP`, `NEXUSAI_PROGRAMME_20260927`, the query-answering decision, `OPEN_ITEMS`, the UX contract, the Stitch redesign plan, the dark redesign directive, the redesign master guide, the R8 contract, the evidence test program, the licence ledger summary, the master directive's phase list R8 to R17, family expectations (19.1 to 19.12) and the media workspace section.

**Read by headings or in part:** the August and September roadmaps and ledgers (`nexusai-current-roadmap` 502 to 800, the backlog, the phase ledgers and mapping), `RECONCILIATION_20260925`, `CODEX_BACKLOG`, `CAPABILITY_LOG`, `CURATION_GAP_INVENTORY`, the Stitch product brief (headings), the master directive outside the sections named.

**Not read (immutable evidence or not about the plan):** the `reports/` folders (about 400 entries), the Codex work-item prompts (`WI-LAYER-*`, `WI-UI-*`), `docs/demo/*`, `docs/content/*`, the vendored LocalAI documentation, the design documents for the shell, brand and information architecture (the UX contract supersedes them).

**Facts read from the running system on 2026-10-21:** the container environment of the API (switch names and values, no secrets) and the worker (media role flags), the image tags, the database (analyst views, derived artifact counts by contract, evidence counts per collection, job counts), the registered routes, the repository state of both checkouts, the workspace's routes and test counts.

**Outside sources searched on 2026-10-21 (leads to test, not predictions):** Urdu and South Asian speech recognition, Urdu OCR, licence-clean plate detectors and trackers, commercially usable face models, perceptual hashing, diarization and voice-activity licences, link-analysis interface practice, Palantir Blueprint, investigation-tool features. Where a source was a vendor page or a summary the text says "verify".

---

## Appendix A: sources used for the research sections

* Speech: [Interspeech 2026, ASR failures in low-resource South Asian languages](https://aghaaliraza.com/publications/2026--Interspeech--Dissecting_ASR_Failures_in_Low_Resource_South_Asian_Languages.pdf) · [WER We Stand: benchmarking Urdu ASR](https://arxiv.org/pdf/2409.11252) · [lightweight Whisper for Urdu](https://arxiv.org/pdf/2508.09865) · [Whisper on Pashto, Punjabi and Urdu (CHiPSAL)](https://preview.aclanthology.org/credits/2025.chipsal-1.20) · Omnilingual ASR, arXiv 2511.09690 · [ICML 2026 diarization benchmark](https://icml.cc/virtual/2026/77326) · [pyannote Community-1](https://huggingface.co/pyannote/speaker-diarization-community-1) · [Silero VAD licence text](https://www.fon.hum.uva.nl/praat/manual/Silero_VAD_MIT_License.html)
* OCR: [From Press to Pixels (Urdu text recognition)](https://arxiv.org/pdf/2505.13943) · [LREC 2026 Urdu benchmark](https://aclanthology.org/2026.lrec-1.235/)
* Plates, detection and tracking: [RF-DETR commercial licence](https://blog.roboflow.com/rf-detr-is-free-to-use-commercially/) · [Pakistani plate localisation and recognition, PLPD](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC8622297/) · [Roboflow Trackers](https://trackers.roboflow.com/develop/details/) · [Qwen3-VL technical report](https://arxiv.org/pdf/2511.21631)
* Faces: [AuraFace-v1 model card](https://huggingface.co/fal/AuraFace-v1/blob/main/README.md) · [InsightFace licensing](https://www.insightface.ai/solutions/face-recognition-licensing)
* Image forensics: [pdqhash bindings](https://pypi.org/project/pdqhash/) · [Meta ThreatExchange (PDQ, TMK)](https://github.com/facebook/ThreatExchange)
* Link-analysis interface: [dciWebMapper2, linked maps and charts](https://arxiv.org/pdf/2509.07897) · [Siren, performing a link analysis](https://docs.siren.io/siren-platform-user-guide/14.8/graph-browser/t_link-analysis.html) · [Ogma comparison pages (vendor)](https://doc.linkurious.com/ogma/latest/compare/cytoscape.html) · [Palantir Blueprint](https://blog.palantir.com/scaling-product-design-with-blueprint-25492827bb4a) · [Cambridge Intelligence link visualisation styles](https://cambridge-intelligence.com/link-visualization-styles/) · [Magnet AXIOM Connections](https://www.officer.com/command-hq/technology/computers-software/investigation-software/product/21012225/magnet-forensics-digital-forensics-software-know-how-a-file-was-downloaded-uploaded-and-more-magnet-axiom-12-with-connections-digital-investigation-software)

## Appendix B: ID index

`L-Q`: question-answering line (4.1) · `L-R`: master directive phases (4.2) · `G`: maturity roadmap gaps (4.3) · request rows 1 to 26 and G-1 to G-4: interface requests (4.4) · `L-U`: interface items (4.4) · `L-M`: media (4.5) · `L-P`: process (4.6) · `P0` to `P5`, `PL`, `P-H`: programmes (6) · `U0` to `U10`: interface stages (7.4) · `M0.1` to `M0.8`: media foundation (8.3) · `BR-1` to `BR-4`: backend batches (4.4) · `D23` to `D32`: new decisions (11).
