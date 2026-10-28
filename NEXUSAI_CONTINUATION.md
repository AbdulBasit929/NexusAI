# NexusAI — Current State

> **CURRENT STATE ONLY. Rewrite it; never append. Hard cap 300 lines.** History in
> `reports/archive/` and `docs/work/HANDOFF_LOCAL_20261007.md` (live log). **TWO AGENTS WRITE THIS FILE:**
> everything above `## CODEX_UI_TRACK` is the backend track, that section is the interface session's;
> neither edits the other's. **The plan of record is [`docs/roadmap/master-plan-20261021.md`](docs/roadmap/master-plan-20261021.md)**
> (ledger of every open item, programmes P0 to P5, the interface programme, the media plan, decisions).

**Last updated:** 2026-10-21 · **Branch:** `lane-arms` (worktree `NexusAI-localwork`), pushed to `origin claude/gifted-pasteur-4kpx0r`

---

## 1. Where the product is (measured)

    STRUCTURED + LINK ANALYSIS (strong)   the Ask engine v2 answers call, internet, device, cell and connection
        questions on typed cases: fresh seeds 45/45, 81/81 (judgement case 108), 72/72 (co-location at the hour),
        0 wrong; p50 ~30 s, p90 ~50 s on the CPU laptop; a connection question on the real collection 39 s.
    MEDIA (weak, and OFF in the worker)   every media role is switched off; the engine reads no derived media;
        plates 59% exact on images, 7% on video, Urdu OCR 0/1, Urdu speech 45.5% WER on 2 clips, no Punjabi/Pashto.
    INTERFACE (one directive since 2026-10-26)   17 routes, 75 vitest files (549 tests), 26 Playwright specs; the link-
        analysis workbench (layout B) is live on Connections and inside Ask answers; ONE directive of record now:
        docs/ux/MASTER_UI_UX_DIRECTIVE.md (stages U1 to U10, merge ledger, sources register, Codex start prompt);
        UI owner for stage U1: Codex in its own worktree NexusAI-ui (:4182), decisions O-1/O-2 answered 2026-10-28;
        4 of 30 backend requests delivered.
    ENTERPRISE (open)   no case record, one shared actor, no authority record; read-only role and RLS are live.

**Typed live (14 collections):** the real-data collection `nexusai-real-private-demo` (24 files in the vault,
**counts and booleans only**), `ingest-v2-shadow-synthetic-20261013`, the acceptance case, forensic-demo,
runtime-acceptance, records-demo, and the eight generated development cases `nexusai-wave1-devN-20261021`.
**Time basis:** every typed row is `declared`, `explicit_offset` or `confirmed_by_analyst` (Asia/Karachi confirmed
by the owner for forensic-demo 2026-10-20 and records-demo 2026-10-21); none is assumed.

## 2. Lessons — the lists that bind. Reasoning: [`reports/LESSONS.md`](reports/LESSONS.md)

**Every capability arrives with the check that bounds it, behind its own switch, measurable and withdrawable.**
Verification proves a plan against ITSELF, never against the question · an invented filter is a symptom and **a
missing one has none** · fix the instrument first · a probe must reproduce the PATH · **write a threshold against
what a change CAUSES** · **a switch must be DECLARED in compose** and asserted from the container before scoring ·
**a timeout is never a verdict** · re-derive every expectation when a definition changes (the typed layer drops
exact repeats; judge a typed case on the EFFECTIVE views) · **gates are written before a run; a fix is measured on
another fresh seed; judgement and sealed cases are never tuned on** · the oracle's and the judge's own defects
are reported · **thick views, thin SQL**: a 4B model on a CPU writes ~5.5 tokens a second, so the output length is
the latency (a materialised view took a connection answer from 24 s to 82 ms) · run the typing report BEFORE any
question after a data change · no spread after each fix: use the tiers (`tools/dev/README.md`).

**Rejected after measuring, do not retry:** removing generator-invented filters by any means · switching the
ladder off without replacing its scope · agreement of two drafts (half the questions abstain, twice the time) ·
a fixed all-in prompt prefix · withholding uncurated fields · embeddings on structural decisions.

## 3. Open work — the ledger is in the plan (master plan section 4)

**First:** P0 consolidate (one repository history; the main checkout is 176 commits behind with 574 uncommitted
paths: archive first), then P1 the structured core (A5 at 15 minutes, A7, A9 to A11, typed writers for ANPR,
subscribers, towers, access logs, transactions, front door D6, composition under the number lock M5, backend
batches BR-1 to BR-3, retire the ladder per family), P2 the interface (one direction, designed first), P3 media
(documents, image forensics, plates, speech, Urdu OCR, video, faces at detection only, captures), P4 cross-family
(mention index, entity resolution), P5 enterprise, PL languages (needs reviewer names, D10).

**Known live defects and limits:** the coverage guard counts only files version 1 detected as call or internet
records · the interface templates and helpers that read `forensic.records` directly still read version 1 (G44) ·
Postgres shared memory is the 64 MB default (a big parallel query failed once) · the 13 provider layouts are
drafts (no real-file confirmation yet) · `VOICE` label not mapped in `call-c4` (3 rows) · reverse IMEI form
(14 asked, 15 stored) not handled · the connection class is 39 s, the 15 s and 30 s targets are unmet on the laptop.

## 4. Architecture — decided, do not relitigate

1. **Questions are answered by the Ask engine v2 first** (`FORENSIC_ASK_V2`, `_FIRST`): the model writes ONE
   SELECT over ten reviewed analyst views (`analyst.v_*`), code validates it (parse-tree allowlist, 96-statement
   red team), checks obligations (HOPS, RECURSION, SERVICES, HUBS, DIRECTION, NAME, VOLUME, population, coverage),
   runs it read-only as `forensic_analyst_ro` (cost 5,000,000, row cap 1,000, 15 s) and writes one audit row. A
   decline goes to the governed lane and then the older ladder. Three outcomes only: answered, abstained, declined
   (`docs/architecture/QUERY_ANSWERING_DECISION_20261005.md`). `docs/data/ask-engine-v2.md` is the record.
2. **Typed tables are the source for call, internet and application-usage records** (`comm_event`, `data_session`,
   `app_usage`, effective views drop exact repeats; `FORENSIC_TYPED_FAMILIES=communications_cdr,network_ipdr`).
   Layouts are data (`ingestion/forensic_records/layout_registry/layouts`, 20 entries, matching on the latest
   version). Ingest v2 writes typed rows in shadow beside version 1.
3. **Deterministic SQL, no model narrates a number.** D17 (2026-10-14) lets the model explain the analyst's own
   masked, capped result under a number lock; until M5 is measured the deterministic sentence is the answer.
4. **Derived artifacts are never merged with ingested records.** A model's plate read is not a camera's sighting;
   `source_truth_state` travels with every derived citation. Media views for the engine are planned (migration 022).
5. **One case clock:** `FORENSIC_ANALYSIS_TIMEZONE=Asia/Karachi`; evidence times are stored as UTC instants.
6. **The ladder (`query.go`, 12 templates) is frozen,** still on (`FORENSIC_LADDER_ROUTING=true`) for what the
   engine declines; it is retired per family after parity (M8). **No new templates.**
7. **LocalAI becomes a pinned, unmodified upstream image** (not yet done); NexusAI owns identity and cases (open).
8. **UI contract:** [`docs/ux/NEXUSAI_PRODUCT_UX.md`](docs/ux/NEXUSAI_PRODUCT_UX.md) is binding (IA is CASE →
   EVIDENCE → QUESTION; provenance is the primary object; the analyst never sees SQL or a model).

## 5. Hard rules

- **No new operation templates**; do not extend `query.go`'s keyword ladder. **No embeddings on structural
  decisions.** **No model roulette:** no evaluating, benchmarking, downloading or swapping a model except by a
  decision rule written BEFORE the run and the owner's yes per item (licence ledger row first).
- **Abstention is a success.** A confident wrong answer never is. **Never fabricate** a result, citation, OCR
  text, transcript, sighting, face identity, location, timeline or relationship. No reports without a measurement.
- **Real data is a test input, never a repository file.** Counts and booleans only for the real collection;
  the vault, the policy (`docs/data/real-data-handling-policy.md`) and the scanner apply; Stitch and any cloud
  tool receive synthetic content only.
- **Every behavioural change is measured against a pre-declared threshold** and reverted if it fires.
- **Commit trailers:** `Assisted-by: Claude Code:<model-id> [tools]` and `Claude-Session: <id>`; NEVER
  `Co-Authored-By` of an AI (AGENTS.md; an eleven-commit exception is recorded as G53).
- **Never merge upstream LocalAI into this repo.**

## 6. Operational hazards

- **Auth fails closed** (`FORENSIC_RECORDS_API_KEY` from `.env.forensic-runtime.local`, in the MAIN checkout).
  **Never "fix" a crash-loop by disabling auth.**
- **Recreate the API only with the Run-Arms helpers or `recreate_api_askv2.ps1`** (about 60 settings and the
  secrets would be reset by a bare compose); the image is a **host-built Linux binary layered on the previous
  image** (`go build -mod=mod`, restore `go.mod` and `go.sum` afterwards) because the in-container module download
  fails here. Tag a rollback image first. **After an API recreate restart the dev servers** (`tools/dev/Restart-Workspace.ps1`)
  or requests hang. The API image is distroless: use `docker inspect` for its environment, not `exec printenv`.
- **Never** `docker compose down`, `down -v`, `--remove-orphans` (would delete `nexusai-api-1-prev-gbnf`, the LocalAI that serves the language model and speech), `system prune`,
  `volume prune`. Never reprocess, backfill, migrate or delete retained evidence without the owner's go.
  **Git:** no `reset --hard`, `clean`, `checkout --`, `restore`, `stash`, `merge`, `rebase`, `pull` unless the
  owner asked this session; **the main checkout holds 574 uncommitted paths: do not touch it before the archive.**
- Env from PowerShell, not Git Bash (`MSYS_NO_PATHCONV=1` for `docker exec`); Postgres host port 5433, db
  `localrecall`; `make` and `golangci-lint` exist; a docs-only commit skips the hook, a Go commit runs lint and
  the package's tests (2 to 5 min); `worker.py` has mixed line endings (an Edit made a 2,600-line diff).
- An empty JSON-Schema `enum` is an unparseable grammar (HTTP 500). Ginkgo has a 1 h suite limit: pass
  `-ginkgo.timeout 6h` for the live specs.
- One canonical tree since 2026-10-21 (P0.4 done): `NexusAI-localwork` (`lane-arms`) serves :4181 and :4190; the old `NexusAI` checkout is archived untouched (not reset).

## 7. The live system (read 2026-10-21)

| Part | State |
|---|---|
| LocalAI (language model, embeddings, speech) | the container **`nexusai-api-1-prev-gbnf`** (alias `api` on `nexusai_default`, host port 8080; restart policy `no`, so after Docker restarts it may need `docker start`). **`nexusai-api-1` is a detached duplicate from another compose project: never start it.** Models and backends are in the shared volumes `nexusai_models` and `nexusai_backends`. Speech: `faster-whisper-turbo` (new, MOD-043) and `faster-whisper-small-ur`; the faster-whisper backend reads `beam_size` and `cpu_threads` from the model YAML (original saved as `backend.py.before-asr-options-20261022`). Start order after a Docker restart: Postgres and NATS, then this container; processes started from the app shell die with the app (see the memory note `start-docker-detached`) |
| API | image `a48f2ff888fd` (**2026-10-23: link analysis 3b and 3c LIVE** - successor numbers, movement and anchors, speed alerts, change of rhythm; migrations 025 and 026; rollbacks `before-successor-movement-20261022` = `b86f2b0b7ffe` (the 3a image), `3bc-first-20261022` = `3e14a2acfbbf`, `3bc-second-20261023` = `c46f3890a463`); before it `b86f2b0b7ffe` (**2026-10-22: link analysis 3a** - co-location within N minutes and possible meetings; rollback `before-colocation-20261022` = `293f55838d74`); before it `293f55838d74` (case record `GET/PATCH /cases`, repair dry-run fix); rollbacks `before-case-record-20261021` (`6935d14271c7`), `before-s6-20261021` (`77ec230274a2`), `before-020-20261021` (`ee276c1f9bcd`), `before-s2b-20261020` (`0dcf72adddf8`), `before-s2-20261020`, `before-name-20261020`, `before-guards-20261020`, `before-imei-20261020`, `before-d22-20261019`, `before-ask-v2-20261015` |
| Worker | the Ingest v2 overlay image `db43bb68c042` (kept as `before-media-ocr-20261021`) **plus the media overlay (2026-10-21):** a patched copy of the image's own `media_pipeline.py` and the new modules `onnx_text_ocr.py`, `onnx_ocr_processor.py`, `ocr_router.py`, `multilingual_ocr.py` (rollback of the last step `before-media-ocr-v4-20261021`); recreated with the main compose + Ingest v2 overlay + `docker-compose.forensic-records.media.yaml`, model folder `models/media` of the lane checkout. **OCR role ON** (`FORENSIC_OCR_BACKEND=auto`). **2026-10-22: worker image `037b7b720365`** (rollback `before-media-asr-20261022` = `e7f47e81d59c`) adds the speech patch: **ASR role ON** (`faster-whisper-turbo` through LocalAI, beam 1, 8 threads; recordings over 600 s go to the small model; Hindi heard with no language requested is read again as Urdu; Punjabi and Pashto are stored `LOW_RESOURCE_LANGUAGE`); every other media role off (`_FACE_`, `_IMAGE_EMBEDDING_`, `_ANPR_`, `_VIDEO_ANPR_V2_`, `_V3_ENABLED=false`). Measurements and gates: `docs/data/media-activation-results-20261021.md` |
| Models | one container: Qwen3-4B-Instruct Q4_K_M on CPU (context 4096, one request at a time), Qwen3-Embedding-0.6B; ~22 tok/s reading, ~5.5 writing; 15.7 GB laptop, no GPU (D21; the RTX 2080 machine is not available) |
| Switches on | `GOVERNED_SQL` (+`_FIRST`), `ASK_V2` (+`_FIRST`), `TYPED_FAMILIES`, `VERIFIED_ONLY`, `PLAN_CACHE`, `IR_ARBITRATION`, `IR_FALLBACK`, `LADDER_ROUTING`, `MEDIA_FAMILY_ROUTING`, `DERIVED_ARTIFACT_EXECUTION`, `TEXT_BROWSE_GUARD`, `CONSTRAINT_OBLIGATIONS`, `BOOLEAN_RESTRICTION` and the A1 to A4, S1 to S4 presentation switches |
| Switches off | `ASK_V2_AGREE`, `CONVERSATION_FRONT_DOOR`, `CONVERSATION_ROUTER`, `PII_MASKED_PROJECTION`, `IR_CROSSCHECK`, `BREAKDOWN_GOAL`, `GOAL_TAXONOMY_SLICES`, `IDENTIFIER_BINDING`, `FRAME_FAMILY_PRECEDENCE` |
| Database | PostgreSQL 16 + TimescaleDB; migrations 001 to 021 and **024** applied (**024 co-location and meetings: applied 2026-10-22 after backup `C:/tmp/nexusai-backups/localrecall-before-migration-024-20261022.dump` (456 MB, 4,837 archive entries), preflight, a rolled-back dry run that gave the oracle's counts on a loaded case, verify; rollback `ops/024_colocation_and_meetings/rollback.sql` drops the two views; 022 and 023 are drafts not applied**; 021 case record: applied 2026-10-21 after backup `localrecall-before-migration-021-20261021.dump`; 012 to 015 typed tables, entity and relationship tables (empty), pgvector, `cube`, `earthdistance`; 016/018 effective views; 017 analyst schema and role; 019 observed cell positions; 020 thick views); PostGIS not in the image; backups in `C:/tmp/nexusai-backups/` |
| Analyst views (10) | `v_comm_events`, `v_data_sessions`, `v_app_usage`, `v_cells`, `v_cell_visits`, `v_contact_pairs`, `v_contact_links`, `v_connection_edges`, `v_device_pairings`, `v_evidence_inventory` |
| Reads | `GET /analysis/connections`, `/places`, `/engine-health`, `/typing-report` (fixed reads of the views, scoped, behind the coverage guard); `GET /cases`, `GET /cases/{id}`, `PATCH /cases/{id}` (the case record: name and status, audited) |
| Derived media | 865 artifacts on the acceptance case (309 OCR, 307 ANPR, 115 document passages, 26 image, 24 video plate groups, 22 fingerprints, 20 face, 18 embeddings, 23 audio); plus ~1,900 document passages in the generated cases |

**Regression anchors:** forensic-demo call records 8,642 in version 1, **6,239 effective** (2,702 exact repeats)
and 2,500 internet sessions; the acceptance case 10,168 structured rows (43 sources); judgement cases 101 to 104,
106 to 108 are spent (105 unspent), the four sealed cases are never used.

## 8. Tools and evidence

`tools/dev/check_change.py` (tiers as one command), `typing_report.py COLLECTION`, `live_smoke.py [--quick] [--real]`,
`Restart-Workspace.ps1`; `tools/case_generator/` (20 cases with planted truth, loader, `check_ask_engine.py`,
`check_ask_catalogue.py`); `tools/ask_eval/` (bank builders, `judge.py`, red team); `evaluation/question_factory/`
(`--effective-views`, `reeffective`); `tools/real_data/` (vault tools, layout confirmation, scanner);
`ingestion/forensic_records/layout_registry/` (20 layouts, 61 tests) and `typed_ingest.py` (47 tests).

## 9. Next — the exact next action

**Done 2026-10-26 (later; all local, nothing deployed):** (1) the **wording** of the Ask engine's first sentence (Amendments 5 and 6 of the stage-3 pre-registration: a no-row sentence says what was searched, a score is a candidate not proof, a stated conclusion is corrected, one alert, move and cell visit are sentences), `ask_engine_wording.go`, measured once on judgement cases 113 and 114 (20 of 20 correct each, wording gates met; my own reading found and fixed one defect), **deploy of both amendments waits for the owner's go**; register extended to 32 cases (judgement 113 to 120); (2) the **ANPR typed family** on the lane, tested on throwaway databases only: draft layout `anpr-a1`, interpreter and typed writer for `forensic.plate_sighting`, migration 027 (`v_plate_sightings`, `v_plate_visits`, ops scripts rehearsed), generated sightings with an oracle (`anpr_case.py`, outside the default build so the register hashes are unchanged), plates as identifiers of the question in the engine (search key, look-alikes, absence, caveat, examples, coverage), the bank and the `plates` judge kind; design, truth and gates G1 to G6 in `docs/data/anpr-typed-family-20261026.md`; G4 MET on judgement case 116 (15 of 15, zero wrong; case 115 failed one gate, 13 correct 0 wrong 1 abstained, cause fixed: a year-like plate number), any live step waits for the owner; (3) the **master interface directive** (see CODEX_UI_TRACK).

**Done 2026-10-21:** P0 (archive, classification, one tree, restore rehearsal, retained-copy repair: 68 asset rows), the direction proof, decisions D23 to D32 recorded in the master plan (D23: the existing teal light and dark themes stay), the case record (D30: migration 021 applied, API deployed, the interface shows case names and has a rename control), licence ledger rows, the media readiness tool and the capture kit. The style files `shell.css` and `workspace.css` were restored to the owner's versions after the tree switch (see the handoff).
**Done 2026-10-22:** the **P1-A pre-registration** of the stage-3 link-analysis bank (`docs/data/link-analysis-stage3-preregistration-20261022.md`: scope 3a, 3b, 3c; truth = the generator's oracles; cases 102, 104, 106 for the gate and 105 and a to-be-registered 109 to 112 for fixes (108 is spent on the stage-1 fix); gates written first; nothing built yet). **Done 2026-10-22: stage 3a built, measured and DEPLOYED** (`docs/data/link-analysis-stage3-preregistration-20261022.md`, Run log: judgement 29 of 30 with gates 1 to 3 not met, the one cause fixed, 10 of 10 on the unopened case 105; live smoke 5 of 5 and 6 of 6 earlier-stage questions after the deploy). **Done 2026-10-22/23: stages 3b (A9 successor numbers) and 3c (A10 movement, A11 change of rhythm) built, measured and DEPLOYED (owner's go; backup `localrecall-before-migration-025-026-20261022.dump`; migrations 025 and 026 applied live; API image `a48f2ff888fd`; gate 7 met):** migrations 025 and 026 (rehearsed on a throwaway database, not on the live one), every rule's numbers declared in `askParams` and read from the question, parity with the oracle on all 8 development cases with zero differences, judgement 102/104/106 with 0 wrong of 60 but gate 2 failing in `successor_of` and `anchor_week` (two causes found and fixed: a bank wording the engine rightly declines, and a night check that refused the anchor views' `slot = 'night'`), then 10 of 10 in each sub-stage on the unopened case 109 with every gate 1 to 6 met (Run log in the pre-registration; code 59d754f). **Next:** the backend reads for the workbench (BR-2); the interface foundations (U1) and the workbench prototype once the owner says go; also interface stages U2 onward on the current identity; migration 022 (typed tables for access logs and transactions) and 023 (media views) as drafts with rehearsal. **Needs the owner:** names for D26 (plate captures, Urdu labels, speech with consent, the 50 questions) and D10 reviewers; a go for each media role switched on (all are off), and for the later live steps. **Ask the owner first** for: rebuild or redeploy, compose actions beyond
**Done 2026-10-26 (interface, U3 v1):** the owner chose layout B and decided that questions belong on the Ask page and the Connections page is the whole case. The workbench is built over the live reads: `pages/workbench/WorkbenchCore.jsx` (links, places, table, timeline, evidence panel), `/cases/:id/connections` (case-wide, no question field; "Ask about this case" sends numbers to Ask with `?question=...&run=1`), and inside each drawable Ask answer (`ResultViews` -> `WorkbenchResult`); the previous page is at `/connections/classic` until accepted. Tests: 549 in the UI suite plus 40 workbench tests against real recorded answers (`src/lib/__fixtures__/wb/`). **Review of the 15 stage-3 answers: all agree with the records by my own code; none wrong, none overclaiming; 9 thin in wording** (`docs/data/link-analysis-stage3-review-check-20261026.md`); the headline fix ("times together", "possible meetings" instead of "positions") and the T4 fixes (4acbf23) are **DEPLOYED 2026-10-26** on API image `8145101db0fc` (rollback tag `before-engine-fixes-20261026` = `a48f2ff888fd`; smoke 5 of 5). **Open finding:** "How many events are there between A and B?" for two SUBSCRIBERS answers 8 where the records hold 4 (each call is in both exports and the model adds both directions); FIXED in code (`0ee8491`, obligation `PAIR_DIRECTION`: MAX(events) / COUNT(DISTINCT start_utc) over `src IN (A,B) AND dst IN (A,B)`), measured once on case 112: **19 of 19 correct**, median 60 s (one extra model call per pair question); **DEPLOYED 2026-10-26 (API image `59d033208817`, rollback tag `before-pair-direction-20261026` = `8145101db0fc`; smoke 5 of 5; the live question answers 4).**
`ps` and `logs`, any database write, migration or backfill, model downloads or backend installs, any git write
beyond a plain push to the working branch, long builds, live evaluation suites, worker role activation,
anything touching retained evidence.

## CODEX_UI_TRACK

- **2026-10-28 U1 foundations ACCEPTED; O-3 = B · Ink and signal.** B is now the product default in light and dark; A remains only in `/design/foundations` as the decision record. DTCG tokens, the generator, fixed cascade layers, all 150 legacy aliases, base/layout CSS, the token-driven ECharts theme, `DESIGN.md`, research R45–R60 and the evidence packet are complete. The obsolete `PaletteOptions` component/style are retired after the choice. Proof before close: 160/160 contrast pairs; focused U1 8/8; production build passes; foundation CSS 24,881 B raw / 3,846 B gzip; local Event Timing p75 48/32/32 ms. Full regression remains 536/551 only because the linked worktree lacks the three ignored legacy fixture captures; no test was weakened or fixture fabricated. Stage note: `docs/ux/U1_FOUNDATIONS_STAGE_NOTE_20261028.md`.
- **2026-10-28 U2 shell and navigation OPEN on `ui/u2-shell-navigation` (stacked on accepted U1).** Stage research R61–R71 is recorded in `docs/ux/research/U2_RESEARCH_NOTE.md`; three synthetic, contract-compliant directions are ready at `/design/components`: A Ledger rail, B Case atlas and C Signal dock, with light/dark/high-contrast, 1440/390 and case/global comparisons. **Owner decision pending; the live shell has not been changed.** Next interface action: owner selects A, B or C, then U2 implementation begins from that direction. Do not merge U1 or U2 into `lane-arms` without an explicit repository instruction. Still open for later: O-6, O-8, O-11 and O-12. Nothing live, retained, backend or database-related changed.
- (older entries below are history; the directive wins where they differ)
- **Shell, Dashboard & Workspace Context complete:** Mineral Signal tokens, Type Proof, command search, collapsible rail, zoned Dashboard (D1-D8), Case Context Header with quick actions, and keyboard-roving tab strip live on port 4181.
- **Stage 3 Case Overview Dossier (`/cases/:id/overview`) COMPLETED 2026-10-19:**
  - Dossier Hero & Bento Panels: Header with OpenType tabular monospace ID, tactile Copy button, status badge, hero bento metrics (Sources, Rows, Yield, Ingest), and quick-action buttons. Panels for Record Families (share bars), Event Horizon calendar (intensity cells & deep link), Data Quality (3-card accounting), Next Steps, Attention Triage, Question composer, and Analyst Notes & Leads tracker.
  - Test gate: `CaseOverviewPage.vitest.jsx` (6/6), `caseDossier.vitest.js` (8/8), `caseNotes.vitest.js` (2/2), `CaseShell.vitest.jsx` (18/18) 100% green. Clean 2,710-module build. Dual checkouts synced (`..\NexusAI`); verified HTTP 200 on port 4181. Backend gap: Gap 4 logged (Authoritative Case Annotations & Hypotheses Store). Next: **Stage 4: Governed Investigate Experience (`/cases/:id/investigate` & `/investigate`)**.
