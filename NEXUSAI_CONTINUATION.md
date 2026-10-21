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
    INTERFACE (not settled)   16 routes, 70 vitest files, 26 Playwright specs; THREE redesign directions in the
        repo; the owner dislikes the Connections/Places views; 4 of 30 backend requests delivered.
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
- **Never** `docker compose down`, `down -v`, `--remove-orphans` (would delete `nexusai-api-1`), `system prune`,
  `volume prune`. Never reprocess, backfill, migrate or delete retained evidence without the owner's go.
  **Git:** no `reset --hard`, `clean`, `checkout --`, `restore`, `stash`, `merge`, `rebase`, `pull` unless the
  owner asked this session; **the main checkout holds 574 uncommitted paths: do not touch it before the archive.**
- Env from PowerShell, not Git Bash (`MSYS_NO_PATHCONV=1` for `docker exec`); Postgres host port 5433, db
  `localrecall`; `make` and `golangci-lint` exist; a docs-only commit skips the hook, a Go commit runs lint and
  the package's tests (2 to 5 min); `worker.py` has mixed line endings (an Edit made a 2,600-line diff).
- An empty JSON-Schema `enum` is an unparseable grammar (HTTP 500). Ginkgo has a 1 h suite limit: pass
  `-ginkgo.timeout 6h` for the live specs.
- Two interface trees (`NexusAI` main on :4181, `NexusAI-localwork` lane on :4190) are copied by hand until P0.4.

## 7. The live system (read 2026-10-21)

| Part | State |
|---|---|
| API | image `6935d14271c7`; rollbacks `before-s6-20261021` (`77ec230274a2`), `before-020-20261021` (`ee276c1f9bcd`), `before-s2b-20261020` (`0dcf72adddf8`), `before-s2-20261020`, `before-name-20261020`, `before-guards-20261020`, `before-imei-20261020`, `before-d22-20261019`, `before-ask-v2-20261015` |
| Worker | overlay `db43bb68c042` (rollback `before-s5-layouts-20261021`); Ingest v2 shadow; **all media roles off** (`FORENSIC_OCR_`, `_ASR_`, `_FACE_`, `_IMAGE_EMBEDDING_`, `_VIDEO_ANPR_V2_`, `_V3_ENABLED=false`) |
| Models | one container: Qwen3-4B-Instruct Q4_K_M on CPU (context 4096, one request at a time), Qwen3-Embedding-0.6B; ~22 tok/s reading, ~5.5 writing; 15.7 GB laptop, no GPU (D21; the RTX 2080 machine is not available) |
| Switches on | `GOVERNED_SQL` (+`_FIRST`), `ASK_V2` (+`_FIRST`), `TYPED_FAMILIES`, `VERIFIED_ONLY`, `PLAN_CACHE`, `IR_ARBITRATION`, `IR_FALLBACK`, `LADDER_ROUTING`, `MEDIA_FAMILY_ROUTING`, `DERIVED_ARTIFACT_EXECUTION`, `TEXT_BROWSE_GUARD`, `CONSTRAINT_OBLIGATIONS`, `BOOLEAN_RESTRICTION` and the A1 to A4, S1 to S4 presentation switches |
| Switches off | `ASK_V2_AGREE`, `CONVERSATION_FRONT_DOOR`, `CONVERSATION_ROUTER`, `PII_MASKED_PROJECTION`, `IR_CROSSCHECK`, `BREAKDOWN_GOAL`, `GOAL_TAXONOMY_SLICES`, `IDENTIFIER_BINDING`, `FRAME_FAMILY_PRECEDENCE` |
| Database | PostgreSQL 16 + TimescaleDB; migrations 001 to 020 applied (012 to 015 typed tables, entity and relationship tables (empty), pgvector, `cube`, `earthdistance`; 016/018 effective views; 017 analyst schema and role; 019 observed cell positions; 020 thick views); PostGIS not in the image; backups in `C:/tmp/nexusai-backups/` |
| Analyst views (10) | `v_comm_events`, `v_data_sessions`, `v_app_usage`, `v_cells`, `v_cell_visits`, `v_contact_pairs`, `v_contact_links`, `v_connection_edges`, `v_device_pairings`, `v_evidence_inventory` |
| Reads | `GET /analysis/connections`, `/places`, `/engine-health`, `/typing-report` (fixed reads of the views, scoped, behind the coverage guard) |
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

**P0.1:** archive the main checkout's uncommitted state outside the repository, then classify the 574 paths by
content against `lane-arms` and the merge base `3fca698` (master plan 6.2). In parallel the **U0 direction proof**
(master plan 7.6) and the **P1-A pre-registration** of the stage-3 link-analysis bank. **Needs the owner:** D23
interface direction (after the proof), D24 one tree and one stage owner, D25 media activation order, D26 who
captures and labels, D30 a minimal case record (migration 021), D31 plate-component licence route, D10 reviewer
names, and a go for each live step. **Ask the owner first** for: rebuild or redeploy, compose actions beyond
`ps` and `logs`, any database write, migration or backfill, model downloads or backend installs, any git write
beyond a plain push to the working branch, long builds, live evaluation suites, worker role activation,
anything touching retained evidence.

## CODEX_UI_TRACK

- **Identity + shell complete 2026-09-29:** Mineral Signal tokens and Type Proof govern the live workspace; the three-band shell has a code-native NexusAI lockup, scoped command search, persistent/collapsible desktop rail, focus-trapped mobile drawer, compact case context and coordinated navy-charcoal navigation/canvas surfaces. Noto remains for mixed-script coverage. `docs/work/UI_REDESIGN_RESEARCH_20260929.md` holds the primary-source rulings.
- **Dashboard under final product review:** the live page is a real readiness workbench, not KPI card soup. It now combines direct workspace totals, a true multi-segment readiness donut, linear ingestion accounting, ranked structured-family bars with linked filtering, a readiness-ordered queue, synchronized inspector tabs and a selected-case readiness plot. Processing/failure details, curated questions and browser-local recent work derive from existing responses; unsupported trends, priority, assignment and full-census claims stay absent.
- **Measured gate:** 109 unit + 46 ported tests green; the 11-test token matrix proves text >=4.5:1 and components/focus >=3:1 in both themes; clean 2,039-module build; four Dashboard/shell Playwright contracts green. Light/dark captures cover 1440/1280/1024/768/375 plus drawer, collapsed rail, command search and case shell; 375px has zero page overflow and the keyboard drawer path traps focus, closes on Escape and returns focus. The real `127.0.0.1:4181` page was checked with 2 cases / 55 sources / 53 ready / 2 failed. No deployment, container, API, semantic-layer or DB change.
- **2026-09-29 Claude took over the UI track.** Binding: `docs/work/UI_REDESIGN_MASTER_GUIDE.md` (staged plan S0..S12: shell, header, top nav, sidebar, canvas, Dashboard section by section, then every page; research before each stage; backend needs go to `BACKEND_REQUESTS.md`) plus `UI_REDESIGN_BRIEF_20260929.md`; both supersede Codex's hold point below. **First pass done, to be redone stage by stage:** Identity A tokens (`tokens.css`, `identity-a.css`), gradient sidebar, live badges, case breadcrumb, `/investigate` page, Dashboard on ECharts (`components/charts/*`, own 173 kB gz chunk), audit `UI_PAGE_SPECS/shell-and-dashboard-audit.md`, palette gallery A/B/C. 142 unit tests green; dashboard e2e main test green. **Stale e2e from the old contract, fix in their stage:** `dashboard-slice` capture, `shell-redesign-slice` capture, `header-context` capture, `navigation-rail` rail label. **S1 shell frame + S2 header done** (`UI_PAGE_SPECS/shell.md`, `shell.css`; 149 unit tests green, header contrast tested). **S3 case bar done** (breadcrumb, readiness pill, section links with aria-current; case sections left the sidebar; 153 unit + 46 ported tests green; vitest timeout 15 s). **S4 sidebar done** (one Cases category with live status, pinned/recent two-line rows, inline actions, sticky collapse; rail 264 px; 162 unit + 46 ported tests green). **S5 canvas done** (`UI_PAGE_SPECS/canvas.md`: page tokens, PageHeader v2 without eyebrow, Card, unified state panels, Skeleton/ShellSkeleton, mobile section-bar scroll shadows; 172 unit + 46 ported tests green). **Team-lead revision done 2026-09-30:** recent/pinned questions left the sidebar for the command palette (empty-query recents, all cases) and a "Your questions" card on `/investigate`; header "Ask a question" is a measured ghost-accent button (a light-theme invisible-label defect fixed); Appearance is an icon button with a radio-tile panel (`AppearanceMenu.jsx`, shared `useThemePreference`). 186 unit + 46 ported tests green; two e2e specs updated for the new Appearance selector. **Dashboard 2026-09-30:** D1 header and D2 metric cards accepted; the page is now a zoned, widget-based layout (`UI_PAGE_SPECS/dashboard-architecture.md`, `dashboard-product-plan.md`): hero = real `activity_by_day` chart (`lib/caseActivity.js`, `useCaseActivity.js`, cap labelled partial) beside a compact attention rail, then an Evidence map (bubbles) beside readiness. Backend requests 14 (corrected: reprocess exists) and 15 to 19 filed. Next: Retry dialog on the rail, Ask bar, then readiness, ingestion, workbench. Shots: `apps/investigation-workspace/design-review/ui-redesign-20260930/`. No API, DB or Docker change.
- **Rulings / hold point (Codex, superseded):** every visualization requires a named question, authoritative source, useful encoding and filter/drill-through; chart variety follows the question, never decoration. `docs/work/BACKEND_REQUESTS.md` specifies complete processing history, evidence census, collection directory and authorized activity contracts needed for later truthful visuals. Do not advance beyond Dashboard until product review accepts this surface; preserve WI-LAYER-8's separate product-owner gate.
