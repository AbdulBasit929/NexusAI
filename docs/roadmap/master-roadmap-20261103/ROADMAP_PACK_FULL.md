# NexusAI roadmap pack of 2026-11-03: start here

**What this is.** The reconciled state of the project, the media programme (images, video, ANPR, audio and speech in English, Urdu, Pakistani Punjabi, Pashto and Saraiki, faces), the architecture (cases, cross-case, evidence, the Ask engine, history, structured families, reports, security), the interface programme (a full redesign by Claude now that Codex is not available), the data and testing policy with the researched dataset register, and the phased roadmap with gates and the decisions the owner must make. Written 2026-11-03 after a full read of the repository, the live system, the memory of earlier sessions and today's web research.

| File | Read it for |
|---|---|
| `01_state_of_the_project.md` | what exists, what is live, what was measured today, what is missing |
| `02_media_programme.md` | **the main focus now**: every media type, language and operation, with models, licences, gates and work packages |
| `03_architecture.md` | case, cross-case, evidence viewers, the runtime-LLM Ask engine and its roles, history, families, reports, security |
| `04_ui_ux_programme.md` | the redesign: rules, pages, Ask and Evidence specs, chart catalogue, stages, cut-over |
| `05_data_testing_research.md` | datasets (with licences), synthetic programmes, tests without repetition, banks, research log |
| `06_roadmap_phases.md` | Phase 0 to 4, lanes, gates, the first 14 days, what to protect first |
| `07_decisions_and_risks.md` | decisions D34 to D43, risks, what is not promised, what is superseded |

Binding documents that this pack does **not** override: `AGENTS.md`, `NEXUSAI_CONTINUATION.md`, `docs/architecture/RECONCILIATION_20260921.md` (section G), `docs/architecture/QUERY_ANSWERING_DECISION_20261005.md`, `docs/ux/MASTER_UI_UX_DIRECTIVE.md`, `docs/ux/NEXUSAI_PRODUCT_UX.md`, `docs/data/real-data-handling-policy.md`, and the master plan `docs/roadmap/master-plan-20261021.md` for the ledger of older items.

---

## The prompt to paste into the new chat

> You are continuing the NexusAI forensic-intelligence project for its owner (a developer who must defend the work to a team lead; they want short, concrete, verified, honest answers and PowerShell-ready commands, and they do not want long repeated test cycles).
>
> **First read, in this order and in full:** `AGENTS.md`, `NEXUSAI_CONTINUATION.md`, then the roadmap pack `docs/roadmap/master-roadmap-20261103/00_START_HERE.md` and the files it lists, then `docs/ux/MASTER_UI_UX_DIRECTIVE.md` (only when working on the interface). The lane tree is `C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork` (branch `lane-arms`); the interface worktree is `C:\Users\sheik\Workspace\Office\Projects\NexusAI-ui` (branch `ui/u2-shell-navigation`, served on `:4182`); `:4181` serves the lane tree with the OLD design and must keep working; the main checkout `...\NexusAI` holds `.env.forensic-runtime.local`, which you never read, print or copy (scripts read it themselves).
>
> **What the owner wants now (2026-11-03):** (1) implement the media data types best of the best: images, video, ANPR images and video, audio, speech-to-text in English, Urdu, Pakistani Punjabi, Pashto and Saraiki, face detection, recognition and matching, and every relevant operation, with working viewers; (2) the best architecture for cases, cross-case, evidence use, the Ask chatbot, complete chat-style history with re-work, and every structured family and file type; (3) recreate and redesign the whole interface to the directive (Codex is not available, so you do it), fixing first the Ask page (sections not shown completely, wasted side space, too compact) and making every evidence file open complete and accurate in a proper viewer; best dashboards, charts and visualisations; (4) real, relevant datasets from online sources until real data is supplied; (5) **no predefined question templates or operations**: questions are understood and answered at run time by the language model with code validating (the model roles are in `03_architecture.md`), including a measured greeting/help/clarification role. The owner approved the decisions listed in section 2 of the pack's file 01 and the plan's recommendations; the new decisions D34 to D43 are in file 07: ask them **once, as one list**, in your first message.
>
> **Live state (verify first):** API image `nexusai-forensic-records-api:round9d-20261102` = `c4508a3f7714` (rollback tag `before-round9d-20261102` = `da91800b051a`); worker `47e5e829a3d4`; LocalAI container `nexusai-api-1-prev-gbnf` (never start `nexusai-api-1`); API on `127.0.0.1:8091`; verified bank 473/484 on round 9c. After **any** API recreate restart the workspace at once (`tools\dev\Restart-Workspace.ps1 -Port 4181 -Tree lane -Collection orion-lahore-reference-20261030`), then check `/healthz`.
>
> **Rules that stand:** synthetic or public data only outside the vault; no evidence, crop, transcript or embedding to any third party; model output is an observation never a fact; no identification claims from faces, voices or plates; licence-clean components only (no AGPL, non-commercial weights are evaluation-only, every component has a ledger row); one heavy model at a time and one evaluation container at a time (`--cpus 8 -m 5g`, the Docker VM is 7.6 GB); roles switch on one at a time behind a pre-registered gate; judgement and sealed cases are spent when used; never merge interface work into `lane-arms` without the owner's separate word; **ask before** a deploy, a migration, a database write, a model download, anything touching retained evidence, a long build; the owner's chat word is needed per deploy.
>
> **Commits:** explicit paths only (never `git add -A`; do not stage `tests/e2e-ui/Dockerfile`, `tests/fixtures/backend-image/Dockerfile`, `tools/visual_spike/package-lock.json`); message from a file with `git commit -F`; end with `Assisted-by: Claude Code:<model-id> [tools]` and `Claude-Session: <id>`, **never** `Co-Authored-By` for an AI; the pre-commit hook runs the whole Go package when Go files change (about 4 minutes: commit in the background); no force-push.
>
> **Work method:** follow the pack's phases; finish a capability only with a view, a Playwright test on a generated case, a bank with exact truth and an owner demonstration; test with the tiers (unit, typing report, live smoke, re-ask only what a change touches; full banks only at milestones, in the background); write a stage note and refresh `NEXUSAI_CONTINUATION.md` (rewrite, never append, under 300 lines) and the memory at every gate; report in plain words with the page to open. Tool traps: PowerShell 5.1 splits unquoted dotted flags (quote them); Git Bash has no `python3` (use PowerShell `python -I -X utf8`); the Edit tool changes line endings of mixed CRLF/LF files (check `git diff --stat`); Windows psql needs `SET app.tenant_id='default';`; start Docker via `explorer.exe`.
>
> **First actions:** (1) confirm the live state; (2) send the owner the decision list D34 to D43 and the question "have you reviewed U2 on :4182?"; (3) run Phase 0 of `06_roadmap_phases.md`: engine round 9e, data fetch under ledger rows, the Omnilingual/Whisper baselines on the development partitions, drafts of migrations 033 and 034; (4) split Phase 1 into parallel sessions with file ownership (engine, speech lab, conversations backend, U4 Ask) and start them.

---

## The same thing in twelve lines (for a short message)
1. Strong: typed structured families and the Ask engine (verified bank 473/484, 0 confident-wrong), link analysis, evidence viewers.
2. Weak: media (OCR and turbo speech on; faces, video, plates off; the engine cannot see media), no server-side case layer (notes, history, members, cross-case), interface split between old `:4181` and new `:4182`.
3. Today: bank run done; fixes cherry-picked and **round 9d deployed and checked on real files**; two small defects found (BOM in CSV headings, `.xls`/`.txt` column order); a deploy gotcha recorded.
4. Media plan: one pipeline (intake, proxies, tasks, versioned artifacts, review, views for the engine); speech with a router per language; Pashto fixable by public-data fine-tuning (about 25% WER read speech), Saraiki and Pakistani Punjabi weakest (Omnilingual CER 17 and 31); plates and video with RF-DETR, tracking and a versioned plate-format registry; faces L1 then L2 under strict governance.
5. Architecture: case profile, authority, members; identifier registry and reversible entity resolution for cross-case; viewers for every type with a safe inspector as the last resort; model-role registry; server-side conversations with re-run and "changed since".
6. Interface: new design on `:4182` (Ink and signal), order U4 Ask, U6 Evidence, U5, U3, U7, U9, U8, U10; the 2026-11-02 Ask/Evidence work is carried over.
7. Data: Common Voice (Urdu, Pashto, Saraiki), FLEURS, Omnilingual corpus, MEVA, CCPD, synthetic Pakistani plates, codec-simulated telephony, consented faces; every one with a ledger row.
8. Tests: tiers and an impact index; no unchanged reruns; banks at milestones in the background.
9. Phases: 0 close-out (week 1), 1 foundations (weeks 1 to 5), 2 media in the product (5 to 11), 3 intelligence across families and cases (11 to 17), 4 enterprise and release (17 to 24).
10. Decisions D34 to D43 wait for the owner; recommendations are given.
11. Biggest limits: no GPU, no consented data yet, low-resource languages, legal exposure of face recognition.
12. Nothing is promised that is not measured on our own gold sets.


---

# 01. Where the project stands (reconciled 2026-11-03)

**Part of the pack:** `00_START_HERE.md` · **this file** · `02_media_programme.md` · `03_architecture.md` · `04_ui_ux_programme.md` · `05_data_testing_research.md` · `06_roadmap_phases.md` · `07_decisions_and_risks.md`

**Evidence marks used everywhere in this pack.** **[M]** measured by us (repository result file named). **[R]** read today from the repository, the live database or the running containers. **[V]** read today from the primary page of a source on the web. **[S]** read today only in a search summary of a source (re-check before relying on it). **[K]** from prior knowledge, not re-verified today. Anything unmarked is an argument or a proposal, not a fact.

---

## 1. The product in one paragraph

NexusAI is a case-centric investigation workspace for analysts without technical training. They open a case, add evidence (call records, internet-session records, subscriber and handset lists, bank statements, vehicle-camera sightings, documents, and media), ask in plain words, and receive an answer in which every number opens its source row. The answering path is a **runtime** one: a language model writes one read-only SELECT over reviewed analyst views, code validates it against obligations drawn from the question, runs it under a read-only role and records the evidence rows behind each figure. There are no predefined question templates on that path (AGENTS.md: the operation catalogue is frozen and will be deleted). The data is synthetic or public wherever it leaves the laptop; real files stay in the local vault under `docs/data/real-data-handling-policy.md`.

## 2. What changed today (2026-11-03), and what it proved

| Event | Result |
|---|---|
| **Full verified-bank run** (484 questions with answers computed by independent SQL, live engine round 9c, Orion case) | **473 correct, 4 wrong, 7 declined or abstained** [M] `docs/qa/verified-bank-20261102-sheet.*`, commit c92f3d7. By family: calls 149/149, messages 28/30, pairs 31/32, internet 31/31, apps 6/6, bank 119/124, plates 50/51, handsets 24/24, cells 16/16, registrations 16/16, evidence inventory 3/5. Artifact: https://claude.ai/artifact/LagwJnwo4GNgLwkdtLsYfL |
| The 11 misses, classified | **Engine faults:** VB-PAIR-007 (pair of numbers filtered on one side only: 57,670 for 12,290), VB-INVE-003 (`tabular` read as `document`), VB-PLAT-051 (a tie at 50 sightings shown as one plate), VB-BANK-050/091/112/123 ("which accounts sent money **to** the account X" abstained because the check read it as outgoing), VB-MESS-009/019 ("which number was sent the most texts" not recognised as one kind of event). **Wording:** VB-INVE-004 ("text files" read as `.txt`: 14, not the 4 text-modality files; a fair reading). **Infrastructure:** VB-BANK-004 (largest single debit) timed out twice at the model call. No abstention gave a wrong number. |
| **Commit fdfab37** (cherry-pick of 7c767bb from the fix session) | PAIR_SCOPE (both ends of a named pair limited to the pair, other conditions in every alternative of an OR), PAIR_DIRECTION extended to SUM, VALUE_DOMAIN (the values the case's own files hold in `modality`/`detected_type` are read with the coverage and the query is held to a value the question names), verified example `pair-talk-time`; 20 new specs, 13 mutation checks, database spec 31/31. |
| **Round 9d built and DEPLOYED** (owner's word 2026-11-03) | Image `nexusai-forensic-records-api:round9d-20261102` = `c4508a3f7714`; **rollback tag `before-round9d-20261102` = `da91800b051a` (round 9c)**. No migration. Contains: source-row fast path (13.5 s to 0.3 s warm), `GET /evidence/{id}/file-rows`, plain recompute wording, PAIR_SCOPE/VALUE_DOMAIN. |
| **Live check of the new routes on the real Orion files** [M] | file-rows: call export (xlsx) 408 rows in 0.75 s with column letters A to K; bank CSV 384 rows with the file's own headings in file order; IPDR CSV 403 rows, 33 columns; SpreadsheetML `.xls` 264 records in 4.8 s (typed-table fallback, label "Record"); text IPDR export 387 records in 1.2 s (fallback); chat export (a document) correctly returns total 0 so the viewer uses the document path. source-row 93 of the call export: A93..H93 cells, 0.3 s. |
| **Two defects the check exposed** | (1) **BOM in the first CSV heading**: the IPDR CSV's first column is named `\ufeffUFDR start time` and its value shows blank; strip the byte-order mark at ingest and in `file-rows`. (2) The typed-table fallback orders columns by the database's key order (`app, sid, url, host, imei...`), not the file's; store the header list at ingest for `.xls` and `.txt` files. Both go to engine round 9e. |
| **A deploy gotcha, now recorded** | After `3-recreate-api.ps1 -Apply` the script reported "did not become healthy" although the new container was healthy (in-network `/healthz` 200). The Windows port relay (`wslrelay`) was wedged by the dev server's stale keep-alive connections to the old container; `Restart-Workspace.ps1` cleared it (direct `:8091/healthz` 14 ms). **Order for every future recreate: apply, restart the workspace at once, then check health.** |
| Real-model effect of the fdfab37 fixes [M] | Re-asked on the live engine (`C:/tmp/nexusai-g4/vb_recheck_9d.jsonl`): **VB-PAIR-007 now 12,290 (correct), VB-INVE-003 now 3 (correct), VB-INVE-004 now 4 (correct), VB-MESS-009 now answered correctly**; three other `between` questions still correct; VB-BANK-050 still abstains and VB-PLAT-051 still hides a tie (both round 9e). 7 of 9 correct; the other 475 were not re-run, so absence of regression elsewhere is not yet measured. |

## 3. The live system

| Part | State [R] |
|---|---|
| Containers | `nexusai-forensic-records-api-1` (Ask engine and read API, image `c4508a3f7714`), `nexusai-forensic-records-worker-1` (ingest and media worker, image `47e5e829a3d4` = `families-20261030b`, up since 2026-10-30), `nexusai-forensic-postgres-1` (PostgreSQL 16 + TimescaleDB; pgvector yes, PostGIS no), `nexusai-forensic-nats-1`, `nexusai-api-1-prev-gbnf` (LocalAI: the language model, embeddings and speech; never start `nexusai-api-1`). Docker VM 7.6 GB on a 15.7 GB laptop, **no GPU**. |
| Interface | `:4181` serves the lane tree through Vite = **the OLD design** (owner rule 2026-10-28), with the 2026-11-02 Ask and Evidence redo (commit ccd379c). `:4182` serves the Codex worktree `C:\Users\sheik\Workspace\Office\Projects\NexusAI-ui` (branch `ui/u2-shell-navigation`): the **new design**, U1 foundations accepted (colour direction **B, Ink and signal**), U2 shell "B · Case atlas" built and **awaiting the owner's review**. |
| Models | Language model: Qwen3-4B-Instruct-2507 Q4_K_M on CPU (about 22 tok/s reading, 5.5 writing; one request at a time; answers p50 about 30 s). Embeddings: Qwen3-Embedding-0.6B. Speech: `faster-whisper-turbo` (beam 1, 8 threads), long files to `faster-whisper-small-ur`. OCR: Tesseract (`urd+eng`, best data) behind an ONNX script probe, PP-OCRv5 ONNX for Latin. |
| Worker media roles | **OCR ON (auto router), speech ON (turbo, Hindi re-read as Urdu), face OFF (YuNet + SFace configured), image embedding OFF (SigLIP-base configured), video plate reading OFF (the v2 stack uses YOLOv8 weights: AGPL, not for the product).** |
| Data | 20 collections, 537 evidence items [R]. The reference case **Orion** (`orion-lahore-reference-20261030`, Lahore, synthetic, 53 files: 42 structured records, 4 documents, 4 text exports, 3 workbooks in the old XML format) is the demonstration and bank case. Eight generated development cases, judgement cases (most already spent) and four sealed cases exist for engine gates (the sealed ones are never used); real files are in the vault (counts only). |
| Database | 33 migrations (001 to 032 plus a PostGIS draft), about 38 base tables, **34 analyst views** in schema `analyst`: communication events, contact pairs/links/edges, data sessions, app usage, cells and visits, movement anchors and speed alerts, rhythm changes, co-location windows, meeting candidates, successor numbers, handsets and pairings, subscriber registrations, bank transactions and account flows/counterparties/limit bands, plate sightings and visits, document mentions/links/values, evidence inventory. **There is no analyst view over derived media**: migration 023's four media views are in schema `forensic` and the engine does not read them. |
| HTTP surface | 33 routes: upload webhook, collections status/repair, cases (list/get/patch), evidence (list/compare/get/content/source-row/file-rows/reprocess), faces/images similar, analysis (connections, places, engine-health, typing-report), query (hybrid, reading, evidence-line, capabilities, templates), reports/generate, legacy forensic API. **No route exists for conversations, notes, findings, members, authority or saved views.** |

## 4. What is built, by area (reconciled against the master plan of 2026-10-21)

Legend: **Done** (live, measured), **Built** (in the tree, tested, not live or off), **Partial**, **Open**.

### 4.1 Structured data and the Ask engine (the strong part)
| Item | Status |
|---|---|
| Typed families and writers | **Done:** call records, internet sessions, app usage, cell sites, devices, **subscribers and registrations**, **bank/wallet transactions**, **vehicle-camera sightings**, **document mentions**. 20+ provider layouts in the registry (13 drafts promoted only with vault confirmation). |
| Engine v2 | **Done:** frame, obligations (PAIR_DIRECTION/PAIR_SCOPE, VALUE_DOMAIN, ORDER, COUNT_NOT_SUM and others), verified examples as retrieval-selected demonstrations (data, not code paths), plan cache (509 entries), executor with walls, read-only role. Gates: 0 wrong on fresh seeds across families; verified bank 473/484. |
| Evidence under every answer | **Done:** PART A (rows behind a figure), PART A2 (records behind every summary view, 27 views traced, lazy counting after 9 s), PART B (model reading, 43 to 57 s, D33 yes), plain evidence tables, cited row under the file's own columns. |
| Conversation front door and router | **Built, OFF** (`FORENSIC_CONVERSATION_FRONT_DOOR`, `_ROUTER`): greetings, help, clarification are not yet a runtime model role in production. |
| PII masked projection | **Built, OFF** (`FORENSIC_PII_MASKED_PROJECTION`). |
| Open engine items | Round 9e (section 2 above); subscribers gate not met on 5 fresh cases (needs a new case); uncapped model call; TXN-015; recursive reach answers; evidence column in the question pack; capabilities 503; header date-range mismatch; ranking headline "5 rows."; general guard against invented date filters. |

### 4.2 Link analysis and analysis views
**Done and live on :4181:** Connections (workbench layout B), Places, Timeline, successor numbers, movement anchors and speed alerts, co-location at the hour, meeting candidates, rhythm change. The owner dislikes the 2026-10 look of Connections/Places; the redesign is stage U3 (restyle) and a design-first workbench programme (`docs/ux/MASTER_UI_UX_DIRECTIVE.md` section 16).

### 4.3 Media (the weak part, and the owner's focus from today)
| Track | Today |
|---|---|
| D documents and printed OCR | **Done (v1):** native text passages, retrieval with citations, OCR on scans/images via the auto router. Measured: Tesseract best `urd+eng` PSM 3 reads clean printed text at 99.7%; Urdu Naskh 9% CER (pass), **Nastaliq 32% CER (fail, flagged)**; receipts 33% F1 (Tesseract) vs 80% (PP-OCRv5 ONNX) [M] `docs/data/media-activation-results-20261021.md`. |
| A audio and speech | **Done (v1):** turbo, Urdu 20.9% word error and English 4.7% on FLEURS read speech, real-time factor 1.45 on CPU [M]. **Punjabi (Gurmukhi) 91% and Pashto 96% word error: stored as `LOW_RESOURCE_LANGUAGE`, "a pointer to listen, not a transcript"** [M]. No Saraiki, no telephone-band measurement, no diarisation, no translation. |
| I images | **Partial:** technical metadata, perceptual fingerprint (22 on the acceptance case), `GET /images/similar`; embeddings off. No EXIF/GPS policy, no near-duplicate clusters, no object detection. |
| P plates | **Partial/blocked:** images 59% exact on a sealed holdout [M, 2026-08]; video 7% at 1 frame/s. Detector and reader weights have unresolved licences; the older video stack is AGPL. The structured ANPR **family** (sightings from camera exports) is Done (G1 to G4 met). |
| V video | **Open:** ffprobe, frame sampling, audio extraction, V3 plate groups (24 on the acceptance case, off). No proxies, shots, tracking, captions, player. |
| F faces | **Level L0 only:** YuNet detection, recall 81% at score 0.5 for faces of 40 px or more; the false-box criterion could not be measured cleanly, role OFF [M]. SFace comparison route exists, off. **Today the owner asked for recognition and matching: see `02_media_programme.md` section 7 and decision D34.** |
| Engine and media | Derived artifacts: 865 on the acceptance case (309 OCR, 307 ANPR, 115 passages, 26 image, 24 video plate groups, 22 fingerprints, 20 face, 18 embeddings, 23 audio) [R 2026-10-21]. **The engine cannot see them** (no analyst view): "how many Urdu recordings", "which video shows plate X" are unanswerable. |
| Viewers | Document and structured-file viewers are strong after 2026-11-02. Image, audio and video viewers are basic. No review desk. |

### 4.4 Cases, evidence and cross-case
| Item | Status |
|---|---|
| Case record | **Done (minimal):** `forensic.case_record` (name, status, audited PATCH), case names in the UI. |
| Case members, roles, authority record, per-user identity | **Open** (one shared actor today; read-only role and RLS are live). |
| Notes, hypotheses, findings, tasks, saved views | **Open on the server.** The interface keeps notes and annotations in `localStorage` (`caseNotes.js`, `timelineAnnotations.js`); they do not survive a browser change and are not auditable. |
| **Ask history** | **Open on the server.** Turns are held in the page; no conversation store, no search, no re-run, no fork. |
| Entities and relationships | **Tables exist** (`entity`, `relationship`, `relationship_evidence`, `entity_mention`, `document_mention`) and are **empty** except document mentions; no identifier mention index, no entity resolution. |
| Cross-case | **Specified, not built:** reference cases Meridian and Control (overlap and decoys) are in the programme; the resolver and notices are not. |
| Evidence custody | **Done:** custody events, versions, storage objects, hash at intake, reprocess plan/route. |

### 4.5 Interface (see `04_ui_ux_programme.md`)
`:4181` old design: 17 routes, 75 vitest files / 549 tests, 26 Playwright specs [R]. New design on `:4182`: U1 accepted, U2 awaiting review, **U3 to U10 not started; the owner's order is U2 review, then U5 section by section, then U3, U4, U6.** 4 of 30 interface-to-backend requests delivered. Codex is not available; the interface work continues with Claude in the same worktree under the same directive.

### 4.6 Process and tooling
Test tiers T0 to T4 (`tools/dev/check_change.py`, `typing_report.py`, `live_smoke.py`, targeted re-asks, banks only at milestones in the background); case generator with planted truth (20 cases + hash register); reference-case builder (Orion); verified bank builder and judge; licence ledger; media data kit (`tools/media_data/`); real-data vault tools. Pre-commit hook runs the whole Go package when Go changes (about 4 minutes: commit in the background). Commit trailers: `Assisted-by` and `Claude-Session`, **never** `Co-Authored-By` for an AI (AGENTS.md).

## 5. Reconciliation: the master plan's programmes against today

| Programme (master plan 2026-10-21) | Where it stands |
|---|---|
| P0 consolidate | **Done** (one tree, archive, state file partly stale: `NEXUSAI_CONTINUATION.md` is refreshed in this pack's commit). |
| P1 structured and link core | **Mostly done:** all typed families above, link stages 3a to 3c, wording, evidence A/A2/B. **Open:** case record beyond name/status, front door, composition under the number lock, retire the keyword ladder, tower/access-log families. |
| P2 interface | **Started:** U1 done, U2 built. Old-design improvements shipped on :4181 (Ask, Evidence). |
| P3 media | **Started late:** OCR and speech on; everything else open. **This is the focus of the pack.** |
| P4 cross-family intelligence | **Barely started:** document mentions, case hub threshold; mention index, entity resolution and cross-case are open. |
| P5 enterprise | **Open.** |
| PL languages | **Open:** Roman Urdu rendering exists; question sets in Urdu/Punjabi/Pashto need named native reviewers (D10). |
| P-H hardware | **Open, conditional:** public-data fine-tuning can use rented/free GPUs now (demo and public data only). |

## 6. Module inventory (read from the repository, 2026-11-03)

| Area | What is there [R] |
|---|---|
| **Ask engine v2** (`api/forensic_records/ask_engine_*.go`, 28 files) | `frame`, `coverage`, `domains`, `cues`, `describe`, `catalogue` (the schema the model sees); `exec` (executor and walls); `validate`, `pair`, `params`, `ranges` (obligations and the runtime-parameter registry); `retrieve`, `documents`, `plates`, `registrations`, `transactions` (family support); `reading`, `wording`, `response` (answer text); `evidence`, `evidence_lines`, `trace` (PART A/A2); `audit`, `caveats`, `casesettings`; `fallback`, `run` (orchestration); `ask_engine_examples.json` (verified demonstrations) |
| **Older answering path** (retired family by family behind switches) | `governed` (17 files), `semantic` (8), `query` (12), `ir`, `operation`, `template`, `stim`, `conversation` (front door and router, OFF), `verification`, `canonical`, `answer` |
| **Evidence and reads** | `evidence` (6), `reprocess`, `source`, `spreadsheetml`, `pii`, `withhold`, `analysis` (connections, places, engine-health, typing-report), `report`, `queue`, `platform` |
| **Media on the API side** | `media` (9), `derived` (5), `video` (3), `image` (3), `similarity`, `plate` |
| **Worker** (`ingestion/forensic_records`) | `worker.py`, `typed_ingest.py`, `layout_registry/` (20 layouts, 61 tests; readers, matching, timeparse, parties, statement readers), `adapters/` (anpr, cdr, ipdr, subscriber, tower_location), `xlsx_reader.py`, `document_pipeline.py`, `document_mentions.py`, `mention_extractor.py`, `roman_urdu.py`, `media_pipeline.py`, `onnx_text_ocr.py`, `ocr_router.py`, `multilingual_ocr.py`, `image_intelligence.py`, `video_anpr_*` (v2 AGPL stack OFF, V3 ONNX), `processor_readiness.py` |
| **Interface** (`apps/investigation-workspace`) | 17 routes; 74 components, 73 page files, 99 library files, 16 ported files; ECharts 6, Cytoscape 3.34, MapLibre GL 6.11, Lucide, Noto fonts; pages Dashboard, Cases, New case, Case overview, Evidence list/detail, Investigate (Ask), Connections (workbench), Timeline/Places, Activity, Data, Settings, three design proofs; notes/annotations in `localStorage` |
| **Tools** | `tools/ask_eval`, `case_generator` (58 files), `showcase` (reference case builder, verified bank, judge, ask_case), `media_data` (29 files), `real_data` (vault), `dev` (check_change, typing_report, live_smoke, Restart-Workspace, live-window recipes), `licence_ledger`, `tilecache`, `visual_spike`, `r8_*` evaluators |
| **Tests** | Go: 232 test files (live-database specs on a throwaway Postgres); Python: layout registry 61, typed ingest 47; interface: 75 vitest files / 549 tests, 26 Playwright specs; banks: verified bank 484, generator banks, media eval sets |
| **Documentation** | about 350 files in `docs/` (binding: architecture decisions, UX directive and contract, data and runbooks); `reports/` (immutable evidence); this pack |

## 7. The three things that limit everything else

1. **No GPU.** Heavy media models (faces at scale, video captions, large speech models, a bigger SQL model) are slow or impossible on the CPU laptop. Public-data training can be rented; evidence processing cannot leave the premises. A modest local GPU is on the critical path of the media programme (decision D36).
2. **No server-side case layer.** Members, authority, notes, findings, conversations and cross-case notices do not exist yet, so history, collaboration and cross-case cannot be built on the interface alone.
3. **No test data for the languages and media that matter.** Public sets exist for Urdu, Pashto and Saraiki read speech (section 2 of `05_data_testing_research.md`); none exists for Pakistani plates at scale, telephone-band Punjabi or consented faces. Synthetic generation and a capture protocol have to fill the gap.


---

# 02. The media programme: images, video, ANPR, audio and speech (English, Urdu, Punjabi, Pashto, Saraiki), faces

**Owner's request, 2026-11-03:** implement every media type best of the best, working and productive: images, videos, ANPR images and videos, audio, speech-to-text in English, Urdu, Pakistani Punjabi, Pashto and Saraiki, face detection, recognition and matching, and every relevant operation that can be built and integrated. This file is the plan. It extends and, where stated, **changes** section 8 of the master plan of 2026-10-21 (that plan kept faces at detection only and parked speech bake-offs behind a GPU; both are revised here: section 7 and section 4).

Evidence marks as in `01_state_of_the_project.md`: [M] measured by us, [R] read from the repository, [V] primary page read today, [S] search summary only, [K] prior knowledge. Numbers from other people's papers are for orientation; **our own gold sets decide**.

---

## 1. Rules for every media feature (non-negotiable; most are already binding)

1. **A model's output is an observation, never a fact.** Every derived row carries model id and revision, parameters, input hash, confidence, review state (`model_candidate`, `human_reviewed`, `externally_verified`), a locator (page and box; second; frame and box) and a truth state. The interface shows "machine reading, unreviewed" until a person reviews it. Corrections are new versions; the model's output is never overwritten.
2. **No identification claim.** A face, a voice or a plate string never says who someone is, who owns a vehicle or who was driving. A match is a **candidate** with its score, its error rate at that score on our evaluation set, and "human review required".
3. **Evidence never leaves the premises.** No evidence file, crop, transcript or embedding goes to a cloud service, a hosted model or a design tool. Cloud GPUs are used **only** on public or synthetic data to train or evaluate, and the weights come home.
4. **Licence-clean.** Apache-2.0, MIT, BSD, CC0, CC BY components may ship. AGPL (Ultralytics YOLO and trackers) never ships. Non-commercial weights (SeamlessM4T, MMS, NLLB, InsightFace model packs, AdaFace, EdgeFace) are **evaluation-only**, never in the product, and the ledger (`tools/licence_ledger`) records each.
5. **Roles switch on one at a time** behind the readiness receipt (`processor_readiness.py`), on controlled data, with a pre-registered gate; they switch off again if the gate fails. One heavy model at a time (`parallel_heavy_models=1`); one evaluation container at a time with `--cpus 8 -m 5g` (the Docker VM is 7.6 GB).
6. **Low confidence abstains and says so.** A reading in a language or condition below its gate is stored as `LOW_RESOURCE_LANGUAGE` (or similar), shown as "a pointer to listen, not a transcript", and is never counted as a finding.
7. **Hostile input is the default assumption.** Polyglots, decompression bombs, malformed headers, oversized pixels, path traversal, and instructions hidden in OCR text or transcripts. The language model never sees rows or free text from media as instructions; tools are read-only.
8. **Everything is reviewable and reprocessable.** A newer model produces a new artifact version beside the old one; nothing is silently replaced; the interface shows which version an answer used.
9. **Analyst wellbeing.** Graphic images and video open blurred by default, with a one-click reveal, and the setting is per user.

## 2. What exists and what the capability matrix looks like

Legend for **Status:** ON live · OFF built, switched off · LAB to build · GPU needs a GPU to be useful.

| Modality | Operation | Candidate (licence) | Status today | Where it runs | Gate source |
|---|---|---|---|---|---|
| Audio | Normalise, VAD | ffmpeg; Silero VAD (MIT [K]) | partial (ffmpeg only) | CPU | WP-M2 |
| Audio | Language identification per segment | Whisper LID; SpeechBrain VoxLingua107 (Apache-2.0 [K]); case language hints | Hindi-as-Urdu rule ON | CPU | WP-M1 |
| Audio | **ASR en/ur** | faster-whisper turbo (MIT) | **ON** (ur 20.9%, en 4.7% WER) [M] | CPU | exists |
| Audio | **ASR ps (Pashto)** | Whisper-turbo/small **fine-tuned** on Common Voice Pashto (CC0): 23.4%/24.9% WER on CV24 in the cited paper [V arXiv 2604.06507]; Omnilingual-ASR `pus_Arab` CER 12.6, `pbt_Arab` 8.8 [V] | zero-shot Whisper 96% WER (wrong script) [M] | CPU/GPU | WP-M1 |
| Audio | **ASR skr (Saraiki)** | Omnilingual-ASR `skr_Arab` CER 17.0 with 1.5 h of training audio [V]; fine-tune on Common Voice Saraiki (CC0, 6.75 h, 4.33 validated [S]) | none | CPU/GPU | WP-M1 |
| Audio | **ASR pnb (Pakistani Punjabi, Shahmukhi)** | Omnilingual-ASR `pnb_Arab` CER **31.2** (11.9 h) [V]; Whisper `pa` is Gurmukhi (Indian), 91% WER on FLEURS `pa_in` [M] | none | CPU/GPU | WP-M1 (weakest language) |
| Audio | Diarisation | stereo channel split first; pyannote community-1 (CC BY 4.0, gated [K]); DiariZen challenger | none | CPU | WP-M2 |
| Audio | Spoken-number and identifier conversion | rules plus a small model (Urdu/Punjabi/Pashto digit words, "double/triple") | none | CPU | WP-M2 |
| Audio | Translation (ur/ps/pa/skr to English) | IndicTrans2 (MIT [K], Urdu/Punjabi/Sindhi, not Pashto); the local LLM; NLLB eval-only | none | CPU/GPU | WP-M2 |
| Audio | Speaker clustering / verification | SpeechBrain ECAPA (Apache-2.0 [K]); WeSpeaker; same governance as faces | none (earlier decision: no voice identification) | CPU | WP-M11, decision D35 |
| Image | EXIF/GPS, hashes, near-duplicate | exiftool; SHA-256; PDQ (BSD [K]) + pHash | partial (fingerprint 22) | CPU | WP-M3 |
| Image | Content class (photo, screenshot, document, ID, chat screenshot) | SigLIP-base zero-shot (Apache-2.0 [K]) + small head | OFF (embedding) | CPU | WP-M3 |
| Image | OCR en/ur/ar | Tesseract best + PP-OCRv5 ONNX (ON); PaddleOCR-VL 0.9B (109 languages incl. Urdu per vendor [S]); Qwen3-VL (Apache-2.0) | **ON** v1 | CPU/GPU | WP-M4 |
| Image | Chat-screenshot parser, CNIC/MRZ/cheque parsers | OCR + layout + validators (MRZ checksum, IBAN, CNIC 5-7-1 pattern) | none | CPU | WP-M3 |
| Image | Object detection (fixed vocabulary) | RF-DETR Nano to Large (Apache-2.0 [V Roboflow card via S]); YOLOX | none | CPU/GPU | WP-M3 |
| Image | Semantic search (text to image, image to image) | SigLIP-base; pgvector HNSW | route exists, OFF | CPU | WP-M3 |
| Image | Tamper indicators | ELA, copy-move, noise, metadata mismatch: **indicators, not proof** | none | CPU | WP-M3 (late) |
| Plate | Detect + read on images | RF-DETR plate detector fine-tuned on Pakistani plates; fast-plate-ocr (MIT [V], weights' terms unstated) or a fine-tuned PP-OCR recogniser; province grammar decoding | 59% exact [M]; weights unlicensed | CPU | WP-M5 |
| Video | Probe, proxy, thumbnails, sprite, waveform | ffmpeg/ffprobe | probe only | CPU | WP-M6 |
| Video | Shots, motion segments, keyframes, on-screen timestamp OCR | PySceneDetect (BSD [K]), ffmpeg select, OCR | none | CPU | WP-M6 |
| Video | Detection and tracking (person, vehicle) | RF-DETR + ByteTrack via Roboflow Trackers (Apache-2.0 [K]); never Ultralytics | none (AGPL v2 stack configured, OFF) | CPU/GPU | WP-M6 |
| Video | ANPR with track voting | as Plate, plus character voting across the track | 7% at 1 fps [M] | CPU | WP-M6 |
| Video | Captions, semantic video search | Qwen3-VL 2B/4B/8B (Apache-2.0 [S]) | none | **GPU** | WP-M10 |
| Face | Detect, align, quality | YuNet (MIT); a quality estimator | **ON in lab, role OFF** [M] | CPU | WP-M7 |
| Face | Embed, cluster, search, compare | AuraFace-v1 (Apache-2.0 on its card, 512-D ArcFace-style, training-data provenance unclear [V]); InsightFace packs only with a purchased licence | SFace route exists, OFF | CPU/GPU | WP-M7, M8, decision D34 |
| Cross-modal | Link recordings, plates, faces, OCR numbers to events and entities | rules + candidates | none | CPU | WP-M9 |

## 3. The common pipeline (build once, every track uses it)

### 3.1 Stages
```
intake -> safety scan -> fingerprint -> probe -> normalise/proxy -> route -> tasks (queue by resource class)
   -> derived artifacts (versioned) -> indexes (text, trigram, vector, entity mentions) -> links (candidates)
   -> review desk -> analyst views for the engine -> viewers
```
* **Intake and safety scan** (exists for admission; extend the hostile-input suite to media): size/pixel/duration limits, container sanity, no execution, temp-file cleanup, no network from model containers.
* **Fingerprint:** SHA-256 (custody, exists), perceptual hashes for images and key frames (PDQ + pHash), audio fingerprint (Chromaprint [K]) for "same recording" detection.
* **Probe:** ffprobe/exiftool into a typed `media_properties` record (codec, container, duration, fps, resolution, channels, sample rate, creation tool, GPS, device, timestamps, anomalies). Telephony is detected here (8 kHz, narrow-band codecs).
* **Normalise/proxy:** originals are immutable. Derived, hash-recorded copies: video H.264 MP4 proxy 720p (+ HLS if long), thumbnail sprite sheet, audio 16 kHz mono WAV (and an 8 kHz telephony variant for evaluation), waveform peaks, image web proxy and tiles for large images. The browser never plays an original.
* **Route:** by detected type and case settings (languages expected, which roles are enabled for this case, authority for faces) to a set of **tasks**.
* **Tasks and queues:** classes `cpu_light`, `cpu_heavy` (one at a time), `gpu` (when present). Priorities: interactive (one file the analyst just opened) above batch. Retries with backoff; poison-pill quarantine; per-task timeouts and output caps (existing variables); resumable chunking for long audio/video.
* **Reprocess with lineage:** `derived_artifacts` already versions by model; add `supersedes`, `reason`, and an "outdated" badge when a better model is enabled; the interface offers "re-read with the current model" per file and per case.

### 3.2 The artifact contract (extends M0.3 of the master plan)
Every row of every media task has: `artifact_id`, `evidence_id`, `family`, `kind`, `model_id`, `model_revision`, `params_hash`, `input_sha256`, `created_at`, `locator` (`{page, box}` | `{t_start, t_end}` | `{frame, box}` | `{t_start, t_end, box}`), `confidence`, `reliability_state`, `review_state`, `truth_state`, `supersedes`. Free text (transcripts, OCR) lives in text columns **withheld from the engine's views** and reached only through retrieval with the PII rule.

### 3.3 Typed media tables and the engine's views (the fix for "the engine cannot see media")
Migration **033** (next free number) adds tables and reviewed `analyst.v_*` views with curated, non-text columns; the engine writes SELECTs over them like any family, with the label "model observation":

| View | One row is | Key columns |
|---|---|---|
| `v_media_files` | a media file | evidence id, kind (image/audio/video), duration, resolution, languages detected, tasks done, review counts |
| `v_transcript_segments` | a spoken segment | evidence id, t_start, t_end, speaker label, language, script, reliability state, confidence; **text only via retrieval** |
| `v_ocr_regions` | a text region | evidence id, page, box, script, confidence |
| `v_plate_reads` | a plate read (image or video track) | evidence id, t or frame, camera, plate alias or search-only text (D28), confidence, review state |
| `v_face_detections` / `v_face_clusters` | a face / a person cluster | evidence id, t, quality, cluster id, cluster size, first/last seen (**no embedding exposed**) |
| `v_video_events` | a moment | evidence id, t_start, t_end, kind (shot, motion, track, plate, face, speech), label, confidence |
| `v_image_similarity` | a near-duplicate or similar pair | evidence ids, distance, rule, state |
| `v_media_links` | a candidate link | media artifact, linked event/entity, rule, strength, state |

Questions this makes answerable: "how many recordings are in Pashto", "which video shows plate LEA-1234", "list the images with a face seen on more than one camera", "which phone calls have a recording", "when was this person cluster first seen".

### 3.4 Clock offsets and time basis (the CCTV problem)
A DVR's clock is rarely right. A table `media_clock_offset(evidence_id, offset_seconds, basis, set_by, set_at)` stores the analyst's correction with its basis (for example "overlay shows 14:02:11 while a known event at 14:07:40"). Interface and engine show **both** stamps; all joins to CDR/cell/plate events use the corrected time and say so. The on-screen overlay is read by OCR at several points to detect drift.

## 4. Speech: English, Urdu, Punjabi, Pashto, Saraiki (the owner's explicit list)

### 4.1 What is known (measured and read)
| Language | Our measurement | Published orientation | Public training/test data |
|---|---|---|---|
| English | turbo 4.7% WER (FLEURS) [M] | solved for read speech | many |
| Urdu | turbo 20.9% WER, small 33.8% [M]; 7/24 clips auto-detected as Hindi (fixed by re-read) [M] | SeamlessM4T 16.3% on FLEURS Urdu (CC BY-NC: evaluation only) [S Interspeech 2026, LUMS]; Omnilingual 7B-LLM CER **3.5** (237.5 training hours) [V]; Whisper fails mostly by script confusion [S] | Common Voice Urdu 24.0: 302 h, 81.5 h validated, 498 speakers, CC0 [S]; FLEURS `ur_pk` CC BY 4.0 [M] |
| Pashto | Whisper zero-shot **96.3% WER** (wrong script) [M] | outputs Arabic/Dari/Urdu script, WER over 100%, at most 0.8% true Pashto script for any size [S arXiv 2604.04598]; **fine-tuning fixes it**: turbo 23.4% and small 24.9% WER on Common Voice Pashto v24 (113 h) [V arXiv 2604.06507, checkpoints said to be released; licence to read]; Omnilingual `pus_Arab` CER 12.6 (28.7 h), `pbt_Arab` 8.8 (5.6 h) [V] | Common Voice Pashto, CC0; FLEURS `ps_af` |
| Punjabi (Pakistan, Shahmukhi) | none (FLEURS `pa_in` is Gurmukhi: 91% WER [M]) | Omnilingual `pnb_Arab` CER **31.2** (11.9 training hours) [V]; SeamlessM4T 22.1% on Punjabi in the same paper (script of that test set not verified) [S] | Omnilingual's released corpus (CC BY 4.0, about 350 under-served languages [V/S]): **whether it contains `pnb_Arab` and `skr_Arab` audio must be checked**; Common Voice has Gurmukhi Punjabi [K]; **no open Shahmukhi conversational set known** |
| Saraiki | none | Omnilingual `skr_Arab` CER **17.0** with only 1.5 training hours [V] | Common Voice Saraiki 24.0: 5,837 clips, 6.75 h (4.33 validated), 60 speakers, CC0 [S]; a 10 h set under CC BY-NC-SA (research only) [S] |
| Hindko, Sindhi, Balochi/Brahui | none | Omnilingual `hno_Arab` 17.1, `snd_Arab` 6.6, `brh_Arab` 8.2 [V]; no Balochi code found in its list [V] | small |

**Readings that matter.** (1) Omnilingual's character error rates are on Meta's own test sets and are **not comparable** to our word error rates; we measure both on the same clips. (2) Pashto is the quick win: a fine-tune on public data takes it from unusable to roughly 25% WER on read speech. (3) Pakistani Punjabi is the hardest and the most valuable: weak models, little data, a script and a spoken form that differ from the Indian Gurmukhi corpora. (4) Saraiki exists only through Omnilingual and a few hours of Common Voice. (5) Every number above is **read speech**; telephone-band, noisy, conversational and code-switched audio will be worse, and **no figure is quoted to a court until measured on our own gold set**.

### 4.2 The ASR service design
* **One ASR router** (a worker-side component, no new container) chooses the backend per segment from `(language, script, duration, hardware, reliability gate)` in a **routing table stored as data** (versioned, evaluated, switchable), not in code paths.
* **Backends behind the same interface:** (a) faster-whisper (CTranslate2) for `en`, `ur`, and fine-tuned `ps`; (b) Omnilingual-ASR (Apache-2.0 code and weights [V]; inference limit 40 s per clip except the "Unlimited" LLM variants; CTC 300M needs about 2 GiB and 1B about 3 GiB of VRAM on the vendor's A100 table [V]) for `skr`, `pnb`, `pus`/`pbt` and as an independent second opinion; (c) later, an LLM-ASR for hard audio on GPU.
* **Second opinion for hard languages.** For `LOW_RESOURCE` languages run two backends and store both; disagreement is itself a flag ("the two readings differ at 40% of words").
* **Chunking:** VAD segments up to 30 s (Whisper) / 40 s (Omnilingual) with 1 s overlap and overlap-aware merge; timestamps per segment.
* **Language handling:** the case sets expected languages (Urdu, English by default; Punjabi, Pashto, Saraiki on request); per-segment language ID restricted to that set; when the top two are close, run both and keep the lower-uncertainty reading, flag "language uncertain". Keep the existing rule "Hindi heard → re-read as Urdu".
* **Script and normalisation:** NFKC; Arabic-script letter unification (Arabic/Persian yeh and kaf, heh forms, zero-width joiners), Arabic-Indic and Extended Arabic-Indic digits to Latin for search; a **script-correctness check** (share of output characters in the expected script, gate at least 99%) catches the Whisper-on-Pashto failure automatically.
* **Spoken numbers and identifiers** (high forensic value): convert digit words in Urdu, Punjabi and Pashto ("double", "triple", "sifar", "paanch") to digit strings; detect phone-number-like (10 to 13 digit), plate-like and amount-like spans; store them as `v_media_mentions` candidates that feed the identifier mention index.
* **Roman Urdu** rendering stays (exists). **Translation** is an explicit, labelled analyst action ("machine translation"), never mixed into the transcript.
* **Stereo calls.** If a recording has two channels with different speech, treat channels as speaker tracks (free diarisation) before any diarisation model.

### 4.3 Fine-tuning plan (public data, rented or free GPU, weights come home)
| Step | Detail |
|---|---|
| Data (all fetched under a ledger row, hash-checked) | Common Voice Urdu, Pashto, Saraiki (CC0); FLEURS ur/ps/pa (CC BY 4.0); Omnilingual corpus slices for `pnb_Arab`, `skr_Arab`, `pus_Arab` if the released corpus holds them (CC BY 4.0 [V]). Partition by hash as in the existing kit; **test splits are never trained on** (note: some Urdu Whisper fine-tunes in the literature include FLEURS/Common Voice in training; we use our own partitions). |
| Telephone simulation | G.711 mu-law 8 kHz, AMR-NB, Opus low-bitrate, noise at several SNR, packet-loss; applied to the training and test copies so the model sees the channel it will meet. |
| Models | Whisper large-v3-turbo (MIT) with LoRA for `ps` and `ur` (and a distilled small variant for the 7.6 GB laptop path); Omnilingual CTC 300M/1B fine-tune for `skr`, `pnb` (the repository documents fine-tuning recipes [V], not for the Unlimited variants); convert to CTranslate2 int8 for serving. |
| Decision rule (written before running) | Word error with the written normalisation, character error, digit-sequence exact match, script share, loop rate, real-time factor on the laptop; promote a route only if it beats the incumbent on the dev partition **and** passes the gate on the sealed partition. |
| Gates (proposed; fixed after the baseline) | Urdu: WER at most 15% on read speech and at most 30% on the telephone-simulated copy; Pashto: at most 30% / 45%; Saraiki and Pakistani Punjabi: **CER** at most 15% / 20% on read speech, otherwise they stay `LOW_RESOURCE_LANGUAGE` leads; script share at least 99%; loops at most 1%; every segment timed. |
| Honest limit | These are read-speech gates on public data. **A consented sample of the owner's real kind of audio** (telephone calls, voice notes, interviews; D26) is the only way to certify conversational performance. The capture and labelling kit exists (`docs/data/media-capture-and-labelling-kit.md`). |

### 4.4 Speech outputs in the product
Waveform with synchronised transcript, speaker turns, per-segment language chips, reliability banner for low-resource segments, play-from-segment, search that jumps to the second, edit-and-verify with append-only versions, translation toggle, spoken-number highlights, "link to the call record" suggestions, export with timestamps. A voice-note or call recording opens as a viewer that also shows the matching call record's cell and subscribers when linked.

## 5. Images, documents and OCR

### 5.1 Image operations (WP-M3), in the order they are built
1. **Metadata:** EXIF/XMP/IPTC, GPS, device, software, timestamps, thumbnail mismatch; location is sensitive: stored with its method, shown with uncertainty, stripped from exports unless the case enables it.
2. **Hashes and duplicates:** exact SHA-256; PDQ (256-bit) and pHash; near-duplicate clusters labelled "near-duplicate, not the same image"; within-case and (opt-in) cross-case "seen before".
3. **Content class:** photo, screenshot, scanned document, ID card, chat screenshot, map, meme. Routes screenshots and documents to OCR/layout, photos to detection/faces.
4. **OCR and layout** (section 5.2).
5. **Chat-screenshot parser:** bubbles, side (sender/receiver), timestamps, names, attachments; output is a **message family** with a low-confidence flag, entering the engine like any other typed family (message-export family of the master plan, P4).
6. **ID and financial documents parsers** (only with case authority): CNIC front/back (13 digits, 5-7-1 pattern), driving licence, passport MRZ (ICAO checksum), cheque MICR, utility bills; every parser has **validators** (checksums, patterns) and flags failures.
7. **Object detection, fixed vocabulary** (person, car, motorcycle, bus, truck, bag, phone, document, weapon-category as a later, separately gated class) with RF-DETR; counts and boxes only.
8. **Semantic search:** SigLIP embeddings in pgvector (HNSW), text-to-image and image-to-image, always "candidate, review required".
9. **Tamper indicators:** ELA, copy-move, noise inconsistencies, metadata inconsistencies, double-JPEG traces: shown as **indicators** with an explanation of what each can and cannot prove.
10. **Sensitive content:** blur-by-default; optional known-hash list matching (an integration point, not bundled).

### 5.2 OCR and document reading (WP-M4)
* **Now:** auto router ON: Arabic-script page (ONNX probe) to Tesseract best `urd+eng` PSM 3; otherwise PP-OCRv5 ONNX with a Tesseract second opinion. Latin clean print 99.7%; Urdu Naskh 9% CER; **Urdu Nastaliq 32% CER (fail)**; real receipts: PP-OCRv5 80% F1 vs Tesseract 33% [M].
* **Bake-off (decision rule first):** Tesseract (floor), PP-OCRv5 Arabic, **PaddleOCR-VL 0.9B** (vendor lists 109 languages including Urdu, Pashto, Sindhi, Balochi; no independent Urdu figure found; a multilingual parsing benchmark shows an Urdu row but its column layout was unreadable in the summary, so re-read the paper before relying on it [S arXiv 2607.02956]), **Qwen3-VL** (Apache-2.0; GPU for useful speed), fine-tuned TrOCR/PARSeq-style recognisers; reference numbers from the Urdu Newspaper Benchmark (best classic system word error 0.42 / character error 0.159 [S, LREC 2026]; super-resolution helped LLM OCR by 25 to 70% [S]).
* **Data:** a labelled Urdu line set (500 to 1,000: newspapers, screenshots, social posts, documents, with Latin digits mixed in) from licence-clean sources plus synthetic Nastaliq from openly licensed fonts; real lines only from consented captures.
* **Document layer (Track D):** Docling (MIT [K]) for layout/tables on scanned pages; BGE-M3 (MIT [K]) + cross-encoder rerank with Urdu-aware normalisation for retrieval; identifier extraction with validators (10 to 16 digit identifiers at least 95% exact on clean print).
* **Untrusted text.** OCR text and transcripts are quoted as data in prompts, never as instructions; a red-team set (instructions hidden in a document) must pass before any media text reaches the reading role.

## 6. Video and ANPR (WP-M5, WP-M6, WP-M10)

### 6.1 Plates on images and on video
* **Detector:** RF-DETR Nano/Small (Apache-2.0 framework and backbone; check each checkpoint's terms; XL variants may be under a different licence [K]) fine-tuned on Pakistani plates; YOLOX as challenger. Ultralytics code or weights never ship.
* **Reader:** a compact recogniser fine-tuned on Pakistani crops with **grammar-constrained decoding** from a versioned **plate-format registry**; fast-plate-ocr (MIT licence stated [V]; its pretrained weights' own terms are not stated, so train our own) or a fine-tuned PP-OCR recogniser; top-k candidates with confidence.
* **Plate-format registry (data, versioned by date).** Pakistani plates are heterogeneous and changing: province-specific designs, one-line and two-line motorcycle plates, commercial and government colours, Islamabad "ICT" plates, Sindh's Ajrak-themed plates mandated through 2025 after repeated deadline extensions, Punjab's moves to owner-linked registration and proposed new designs for large vehicles [S: Wikipedia, PakWheels, ProPakistani]. The registry stores, per region and date range, the pattern, the characters allowed in each position and the visual class; it constrains decoding, normalises, and **flags** (never fills in) unusual reads. Because ownership is moving to the person (owner-based plates), a plate string is **more** sensitive than before: the masking decision D28 matters.
* **Data:** the 96-image T2-V virtual-scene pack (exact truth) [R]; **synthetic Pakistani plate rendering** (province templates, embossed styles, fonts, dirt, blur, perspective, night noise) composited onto permissively licensed backgrounds (CCPD is MIT [K]) to give tens of thousands of labelled crops; public Pakistani sets are small and unverified (Roboflow "Pakistani-Number-plates", 336 images, licence not read [S]; the PLPD dataset of Yousaf et al. 2021, access to be requested from the authors [S]); real captures under authority (D26) for the sealed set (at least 300 plate tracks across day, night, rain, blur, angle).
* **Gate (R8 contract):** Boundary A: exact read at least 90% with the lower 95% bound at least 85% on the sealed image set, detection recall at least 98%, calibration (confidence at least 0.9 means accuracy at least 95%), abstention explicit. Boundary B for video: track-level exact at least 85% with false plates per hour reported.
* **Linkage (R8.6):** a link to the structured `plate_sighting` family **only** by exact normalised plate plus camera/time criteria, with zero/one/many match states; confusion-aware matching (M/W, 0/O, 8/B) ranked by visual edit cost and labelled candidate; a plate never implies an owner or a route.

### 6.2 General video analysis
| Stage | What it does | Notes |
|---|---|---|
| Probe and proxy | codec, fps, duration, rotation, streams; H.264 proxy, HLS for long files, sprite thumbnails | DVR formats (Dahua `.dav`, Hikvision containers) often need a vendor converter before ffmpeg; the intake reports "unplayable, needs conversion" with instructions rather than failing silently |
| Overlay time OCR | reads the burned-in date/time at several points; detects drift and jumps | feeds the clock-offset table |
| Shots and motion | PySceneDetect + frame-difference; **motion-only index** so a 12-hour static camera yields a short list of intervals | the single biggest time saver for CCTV |
| Detect and track | persons and vehicles with RF-DETR + ByteTrack (Roboflow Trackers, Apache-2.0 [K]; BoT-SORT for moving cameras) | tracks become `v_video_events` with a count timeline |
| ANPR | per track: sharpest 5 crops, character voting, grammar decoding | no generative super-resolution on evidence crops (it invents characters); classical upscaling only |
| Faces | per person track: best-quality face(s), tracked embedding | governed by section 7 |
| Audio track | extracted and sent through section 4 | speech moments appear on the same timeline |
| Captions and semantic search | Qwen3-VL (2B/4B/8B, Apache-2.0 [S]) describes shots; stored as "model description, unreviewed", searchable, **never counted** | needs a GPU to be usable (WP-M10) |
| Vehicle/person appearance | colour, type, make/model by zero-shot SigLIP + small heads | "red motorcycle" searches; re-identification across cameras is a later, candidate-only feature |
| Integrity indicators | duplicate frames, timestamp gaps, re-encode traces, container anomalies | indicators, not proof |
| Clips and export | cut a clip with a burned-in hash/time watermark and a manifest (source hash, in/out, operator, reason) | court-ready |

## 7. Faces: detection, recognition and matching, governed

**This changes decision D5.** The master plan held faces at detection (L0). The owner now asks for recognition and matching. The plan below builds it, with the controls that make it defensible. Nothing here identifies a person: it produces **ranked candidates for a human to confirm**.

### 7.1 Levels
| Level | Capability | Default | Enablement |
|---|---|---|---|
| **L0** | Detection, landmarks, quality; no embedding stored | on for media roles | case setting |
| **L1** | Embeddings, **clustering inside one case** ("may show the same face"), person clusters, best-face selection, appears-with co-occurrence | off | per case, recorded authority |
| **L2** | **Search** a probe face against a case (1:N), **compare** two faces (1:1), analyst-supplied galleries/watchlists | off | per case, recorded authority, role permission, reason on every search |
| **L3** | Cross-case face matching | off, separate approval | two-case authority; hit-notice workflow of `03_architecture.md` section 4 |
| not built | age/gender/emotion attributes, real-time live-CCTV alerting, any automatic decision | | |

### 7.2 Pipeline
detect (YuNet; challengers MediaPipe BlazeFace Apache-2.0 [K], RetinaFace code MIT with weights to check) → align (5 landmarks) → **quality gate** (size, blur, pose, occlusion; low-quality faces are listed but not embedded or matched) → embed (**AuraFace-v1**: ResNet-100 ArcFace-style, 512-D, Apache-2.0 on its model card, LFW 99.65% self-reported, training data described only as "a commercial dataset ... from various sources" and bundled InsightFace detectors are non-commercial [V]; read the licence file and the training statement and take counsel before shipping; the alternative is to **buy** an InsightFace commercial licence [K]) → store in pgvector HNSW (cosine) with `model_id` (an embedding space belongs to one model: re-embedding with another model creates a new index) → cluster (HDBSCAN/agglomerative over quality-gated faces) → analyst review (merge, split, name, reject) → search/compare.

### 7.3 Evaluation before any L2 use (pre-registered)
* **Set:** a **consented South Asian set** with CCTV-like degradation (downscale to 40 px, JPEG, blur, low light, masks, pose, age gap), collected from volunteers (D26) and, for baseline only, public research sets whose licences allow evaluation. NIST FRTE reports demographic differentials per algorithm with a South Asia region-of-birth group; thresholds are developer- and version-dependent [S], so **we calibrate on our own set**.
* **Metrics:** ROC; false non-match rate at false match rates 1e-3 and 1e-4; precision at k; per-group error rates (sex, age band, skin-tone proxy where consented); quality-binned results; a **negative control** (two different people the model scores high must not produce an identity claim).
* **Gate:** published as a table in the pre-registration before the first measurement; failing it keeps L2 off. A threshold shown to analysts is expressed as "pairs of different people reach this score about 1 in N times on our test set", not as a probability of identity.

### 7.4 Governance and audit (built before L1 is switched on)
Per-case enablement with a recorded authority reference; role permission `face_search`; **every search logged** (who, when, probe hash, scope, number of results, stated reason); candidate labelling on every screen and export; embeddings are never exposed or exported; retention rule per case; kill switch per case and globally; a written bias and limitation statement shown in the interface; a legal review before L2 (Pakistan's personal data protection bill is still a draft with broad state exemptions, and the Interior Ministry has directed a unified facial-recognition system [S: GenderIT, Privacy International, ProPakistani]; the law will not answer this for us, so the controls above are the defence). Face links enter link analysis **only after an analyst confirms them**.

### 7.5 Voice is the same question
Speaker clustering/verification (ECAPA-type embeddings) would be governed identically (candidate only, authority, audit). The earlier decision excluded voice identification; with the owner's wider scope it is a decision, **D35**, default: build **within-recording diarisation and within-case speaker clustering only**, no gallery search.

## 8. Cross-modal linking and media in the engine (WP-M9)

| Link | Rule | State |
|---|---|---|
| Recording to call record | recording time/duration or filename numbers match a CDR event of the same parties within a tolerance | candidate, confirmed by analyst |
| Spoken or OCR'd number to subscriber/handset/account | exact normalised identifier from `v_media_mentions` | candidate (mention) |
| Plate read to plate sighting | exact normalised plate + camera/time | zero/one/many |
| Video moment to cell activity | phone seen in a cell near the camera in the window (co-presence) | "may have been near" with the cell's uncertainty, never "was at" |
| Face cluster to person entity | analyst assignment only | confirmed |
| Image EXIF GPS/time to movement anchors | within tolerance | candidate |
| Same image/recording/face/plate in another case | hash/identifier registry (cross-case) | notice, need-to-know |

All of them are rows in `v_media_links` with a rule name and a strength word; the **link analysis workbench** and **timeline** show them with the candidate shape, never as observed facts. This is where media starts to change investigations: one timeline per person or number across calls, bank, plates, video moments, recordings and documents.

## 9. The review desk (stage U9) and quality loop
A queue of low-confidence and disagreement items (ASR second-opinion mismatches, plate reads below threshold, face clusters at the merge boundary, OCR regions flagged): side-by-side source (audio snippet, crop, frame) and reading, accept / correct / reject with a reason, undo as a new event, throughput and agreement statistics. Reviewed items become **labelled data** for the next fine-tune (kept inside the premises, never in public sets) and for the sealed-set refresh.

## 10. Hardware plan (decision D36)
| Tier | Hardware | What it runs | Status |
|---|---|---|---|
| Now | CPU laptop, Docker VM 7.6 GB | OCR, turbo ASR (RTF 1.45), detection at low fps, face L0; everything in overnight batch | live |
| Interim | HP EliteDesk + RTX 2080 (8 GB), if made available | ASR fp16, detectors, face embedding, Qwen3-VL 4B Q4, CTC models; not 7B | exists, not available (D21) |
| Target dev GPU | an affordable 16 GB card (RTX 5060 Ti 16 GB about $430 to 550) or a used 24 GB card (RTX 3090 about $700 to 1,050) [earlier research 2026-09-29; prices unverified] | the above plus an 8B to 14B SQL/reading model, VLM and ASR scheduled together | decision |
| Cloud (public data only) | Kaggle/Colab/rented | fine-tuning and bake-offs; weights return | allowed under the 2026-09-29 rule with owner approval per run |

Throughput targets to state in each gate: audio hours per hour, video hours per hour, images per second, per tier.

## 11. Evaluation and gates (summary; details in `05_data_testing_research.md`)
Pre-registration before every first measurement; development/sealed partition by hash; sealed sets never tuned on; each report states `synthetic`, `public_benchmark`, `controlled_demo`, `authorised_real`; a **media bank** of at least 20 questions per family written against the data with exact truth and the label "model observation"; red-team set (hidden instructions, PII extraction, SQL escape); zero confident-wrong at the bank gate.

## 12. Work packages (ordered; `06_roadmap_phases.md` schedules them)
| WP | Name | Depends on | Output the owner sees |
|---|---|---|---|
| M0 | Media foundation: artifact contract v2, queue classes, proxies/thumbnails/waveforms, clock offsets, review events, migration 033 views | none | Media tab with every file, state, thumbnail, tasks done |
| M1 | **Speech lab**: data under ledger, gold sets, Omnilingual CPU/GPU baseline, Pashto/Urdu/Saraiki/Punjabi fine-tunes, telephone simulation, routing table | M0 (data kit exists) | results table per language, decision, routing live |
| M2 | **Audio product**: VAD, LID, diarisation, spoken numbers, translation, transcript viewer, review, CDR linking, audio media bank | M1 | open any recording: synchronised transcript, languages, numbers, call link |
| M3 | **Image product**: EXIF, PDQ, class, chat-screenshot and ID parsers, SigLIP search, tamper indicators, viewer | M0 | image viewer with metadata, text, similar images, search by words |
| M4 | **Urdu/Pashto OCR bake-off** and Docling layer | M0, data | OCR results per script; scanned PDF viewer with overlay |
| M5 | **Plates on images**: detector, recogniser, registry, synthetic generator, review, linkage | M0, D26 for sealed set | plate reads on images linked to sightings |
| M6 | **Video v1**: proxies, shots, motion index, overlay OCR, tracking, ANPR v1, audio hook, player with event scrubber | M0, M5 | open a video: moments timeline, plates, speech |
| M7 | **Faces L0 to L1**: lab, evaluation set, clustering, governance, UI | M0, D34 | person clusters per case |
| M8 | **Faces L2**: search/compare/watchlists, audit, negative control, legal gate | M7 | search with candidate list and error-rate statement |
| M9 | **Cross-modal links and mentions** | M2, M3, M5, M6 | one timeline across families |
| M10 | **VLM captions and semantic video search** | GPU | search video by words |
| M11 | Speaker clustering (optional) | M2, D35 | speaker clusters per case |

**Cut line if time is short** (what to protect first): M0, M1, M2, M3, M4 (visible value, no policy risk); then M5, M6 (operational ANPR); then M7 to M9; M10 and M11 last.


---

# 03. Architecture: cases, cross-case, evidence, the Ask engine, history, structured families, reports and security

**Owner's request, 2026-11-03:** the best architecture, flow, pipeline and connectivity of the application's modules: case creation and the case itself, cross-case, evidence files and their best use, the Ask chatbot, history like a chat application, every structured data family and file type, and the runtime use of language models (no predefined templates or operations).

Marks as in file 01. Binding documents this file respects and does not override: `docs/architecture/RECONCILIATION_20260921.md` (section G), `docs/architecture/QUERY_ANSWERING_DECISION_20261005.md` (two lanes, one verifier), `docs/ux/NEXUSAI_PRODUCT_UX.md` (the contract), AGENTS.md ("three rules that override habit": add no operation templates; no model swaps outside WI-0's written decision rule; no embeddings for structural decisions such as family, group, measure or field role).

---

## 1. Principles

1. **The case is the unit of trust.** Access, audit, retention, time basis, languages and enabled capabilities (faces, translation) are properties of a case. Nothing crosses a case boundary except through the cross-case mechanism of section 4.
2. **Evidence is immutable; everything else is derived and versioned.** Originals are hashed at intake and never edited. Parsed records, typed events, media readings, entities and answers carry their derivation (model or rule, version, input hash) and can be recomputed.
3. **Every figure opens its source.** From an answer, a chart mark, a table cell, a transcript word or a face box, one click reaches the original row, page, second or frame (the contract, section 0 and 4).
4. **Four epistemic states, always visible, word and shape:** *observed* (in the records), *inferred* (by a stated rule), *candidate* (a lead, requires review), *confirmed* (an analyst, with name and time). A model's output is at best *candidate*.
5. **Runtime understanding, not templates.** A question is understood and answered at run time by a language model working over a reviewed schema, checked by code. What code may contain: the schema description, validators and obligations derived at run time from the question's own parse, executors, walls, evidence tracing. What it may not contain: fixed question-to-SQL or question-to-operation maps, keyword routing to canned answers (section 6.4).
6. **Offline and on-premises.** No evidence, crop, transcript or embedding reaches a third party. Public or synthetic data may use rented compute to train, never to process cases.
7. **Honest states over empty ones.** A missing capability, a running job, a withheld cell, a low-confidence reading are designed states with next actions, not blank panels.

## 2. The layers and who talks to whom

```
 Interface (React/Vite, :4181 old, :4182 new)  --HTTP/JSON-->  Read and Ask API (Go modular monolith)
                                                                 | cases | evidence | query (Ask engine) | analysis
                                                                 | media | conversations | identity | cross-case | reports | audit
        reads (read-only role)  <---------- PostgreSQL 16 + TimescaleDB + pgvector (RLS by tenant and case) ----------> writes (typed writers, worker role)
                                                                 ^
        jobs (NATS) ---> Worker (Python): ingest, layouts, typing, media tasks (cpu_light | cpu_heavy | gpu)
                                                                 |
        Model services (LocalAI): language model, embeddings, speech  |  vision services (OCR, detectors, face)  |  object storage on the evidence volume
```

* **Today** most of this exists: one Go package of 402 files (`api/forensic_records`, about 64,000 lines of non-test code), one Python worker, one database, LocalAI. The interface talks only to the Go API through the Vite proxy.
* **Recommended structure** (done gradually, never as a big-bang rewrite): new capabilities go into **internal Go packages** with narrow interfaces (`internal/conversations`, `internal/media`, `internal/identity`, `internal/crosscase`, `internal/reports`), registered from `main.go`; the flat package stops growing; each new package ships with its own migration range, view definitions, specs and a bank.
* **Roles in the database:** `localrecall` (owner, migrations), the runtime role under RLS (writers), the **read-only analyst role** (the engine). Every query path sets `app.tenant_id` and, from this pack on, `app.case_ids` (the cases the user may see).

## 3. The domain model (one vocabulary for all modules)

| Group | Object | Today | Needed |
|---|---|---|---|
| Identity and authority | user, role, permission, **case_member**, **authority_record** (reference, issuing body, scope, validity, attachments) | one shared actor; read-only role; RLS | **new** (migration 034) |
| Case | `case_record` (name, status), collection mapping | name/status, audited | + type/profile, time zone and clock basis, languages, enabled capabilities, classification, retention, owner, team |
| Evidence | `evidence_items`, `evidence_versions`, `evidence_storage_objects`, `evidence_custody_events`, `evidence_source_links` | done | + tags, legal hold, supersession, "used by" index |
| Records | `records`, typed tables (`comm_event`, `data_session`, `app_usage`, `bank_transaction`, `plate_sighting`, `cell_site`, `subscriber_*`, `device_tac`), effective views | done | + message, extraction, location-history, browser families (section 8) |
| Derived | `derived_artifacts`, `derivation`, `content_embedding`, `document_mention` | done | + media tables and `analyst` views (migration 033, `02_media_programme.md` 3.3) |
| Entities | `entity`, `identifier` (**new**: type, normalised value, keyed hash), `alias`, `entity_mention`, `relationship`, `relationship_evidence` | tables exist and are **empty** (except document mentions) | populate through the mention index and resolution (section 4.4) |
| Knowledge capture | **note, finding, hypothesis, task, tag, bookmark, saved view, annotation** | in the browser's `localStorage` only | **new**, server-side, audited |
| Conversations | **thread, turn, turn_feedback** | in the page only | **new** (section 7) |
| Reports | report, section, exhibit, export bundle | `POST /reports/generate` | + versioning, sign-off, bundle |
| Audit | `records_audit_log`, custody events | partial | one append-only, hash-chained log: access, search (including face search), export, role change, model run, cross-case query |
| Cross-case | identifier registry, `link_notice`, `access_request` | none | **new** (section 4) |

**States.** `observed | inferred | candidate | confirmed | rejected` on every relationship, link and mention; `model_candidate | human_reviewed | externally_verified` on every derived reading; both are columns, never free text.

## 4. Cross-case architecture

### 4.1 What it must do
Tell an analyst, without leaking what they may not see, that the thing they are looking at (a number, an account, a plate, a handset, a person's national ID number, a file or image, optionally a face cluster) also appears in other cases; let them ask across the cases they may access; and never merge or assert identity without a person.

### 4.2 Mechanism
1. **Identifier registry.** For each case a job extracts *strong identifiers* from typed tables, document mentions and media mentions: MSISDN (E.164), IMEI/IMSI, bank account/IBAN, plate (normalised, with date range), national ID number, email, file SHA-256, image PDQ hash. Values are stored as **keyed hashes** (HMAC with a tenant key held outside the database dump) so the registry can match without exposing values to roles that should not see them; the plain value stays in the case's own tables.
2. **Overlap detection** after each ingest: for each new hash, find other cases with the same hash; write a `link_notice(case_a, case_b, identifier_type, count_a, count_b, first_seen, last_seen, strength, state)`; `strength` is a word (exact strong key / strong key plus corroboration / weak) from rules, not from a model.
3. **Visibility rules** (configurable per tenant): a user who can open both cases sees the notice with details and a link; a user who can open only one sees "also appears in another case; owner: X; request access" (the owner name optional per policy); a case a user may not see is **never named**. Requests are recorded, answered by the other case's owner or a supervisor, and audited.
4. **Entity resolution** is a separate, reversible layer: `same_as(entity_a, entity_b, rule, evidence, state, decided_by)`. Strong keys give `candidate` automatically; names alone never do. Name matching is **transliteration-aware** (Muhammad/Mohammad/Mohd/Muhammed; Urdu script vs Roman; honorifics; father's name as corroborator) and needs at least two independent corroborators before a name-based `candidate` is even shown. Confirmation is always a person.
5. **Zero false merges** is the gate (the Control case with decoys: same name with another national ID number; a re-issued number held by another person).

### 4.3 What the analyst gets
* The **notice** on the number/account/plate/file/entity chip and in a case-level "Also in other cases" panel.
* **Ask across cases** (scope picker limited to accessible cases): the engine reads `case_id`-carrying views and answers "which numbers appear in more than one of my cases", "list repeated plates", "which SIM was swapped across cases"; the answer states the scope.
* **Link-chart overlay:** nodes coloured by case, cross-case edges dashed (candidate) or solid (confirmed); filter to cross-case only.
* **Case relations:** `related`, `same incident`, `parent/child` set by a person; shared evidence (same hash) keeps **one custody chain and many case references**.
* **Audit:** every cross-case query, notice view and access request.

### 4.4 The mention index (the missing foundation)
One table of *mentions* from every source: structured identifiers, document passages, OCR regions, transcript segments, chat messages (`mention_kind`, normalised value, locator, evidence id, confidence, state). It feeds: the identifier registry, "where else is this number mentioned" in the entity drawer, cross-family search (a number in a bank narration, a recording, a photographed document), and the entity graph. It exists for documents (`document_mention`, migration 031); generalise it to all sources.

## 5. Evidence: intake to answer, and every file must open

### 5.1 Intake and typing
`upload → hash and custody event → safety scan → type detection (extension, magic, content) → layout registry (provider layouts with confidence) → reader → typed writer → effective records (dedupe, overrides) → views → coverage (what the engine can now answer)`. An **unknown layout** does not fail: it opens a **mapping wizard** (columns, units, time basis, party roles) whose result is a draft layout that can be promoted after a real-file confirmation (the existing layout-registry process). Every stage records counts: accepted, duplicate, rejected-with-reason.

### 5.2 The evidence passport (what a person sees when they open a file)
Identity (name, type, size, hash, source, who added it, when), **processing state with per-stage progress and a reason and next action for every failure**, what was extracted (rows by kind, pages, languages, duration, faces/plates found), quality (rejected rows with reasons, repeated rows, time basis, layout confidence), **where it was used** (answers, findings, reports that cite it), custody history, and actions (open, download original with audit, tag, hold, supersede, reprocess with the current models, compare with another version, export).

### 5.3 The viewer rule: *if it is stored, it opens*
A viewer registry maps type to viewer; the last viewer, for anything unknown, is a **safe inspector** (metadata, hex, printable strings, container listing, a note saying which viewer is missing and a request button). The worst case is never a blank panel. Every viewer states completeness: "rows 1 to 50 of 408", "pages 1 to 12 of 12", "decoded as UTF-8", "N cells withheld by the privacy rule", "OCR: 3 low-confidence regions".

| Type | Viewer (state) |
|---|---|
| CSV/TSV, XLSX/XLS (incl. SpreadsheetML), JSON/XML tables | **Paged file-rows viewer with the file's own headings, order and column letters, cited-row card, spreadsheet-date reading (done 2026-11-02/03)**; to do: BOM-safe headings, header order for `.xls`/`.txt`, multi-sheet tabs, hidden sheets/rows flags, merged cells, formulas shown |
| Text, logs, chat exports | virtualised line viewer, encoding detect (UTF-8/16, Windows-1256, Arabic-script legacy), search, message-bubble mode for recognised chat formats |
| PDF | page viewer (PDF.js) with text layer, OCR overlay for scans, page rail, search hits |
| DOCX/ODT/RTF | server-side render to PDF/HTML, passages in order (done for the docx passages 2026-11-02), tracked changes flagged |
| Email (EML/MSG/MBOX/PST) | header table, body, attachment tree, thread view |
| Image | zoom/pan/tiles, EXIF panel, OCR overlay, face/plate boxes (governed), similar images |
| Audio | waveform, synchronised transcript, speakers, language chips, review |
| Video | player with event scrubber, shot list, tracks, plates, faces, speech |
| Archive (ZIP/RAR/7z/TAR) | tree with parent-child lineage, bounded extraction, hostile-input limits |
| SQLite/DB extractions | read-only table browser on a copy, bounded SELECTs |
| PCAP/EVTX | flow summaries / event counts (no payload) |
| Maps (KML/GPX/GeoJSON) | map layer with a table |
| Unknown | safe inspector |

### 5.4 File-type coverage matrix (priority order of work)
| Source type | Examples | Status | Route | Priority |
|---|---|---|---|---|
| Call detail records | operator exports, CSV/XLSX/XLS/TXT | **done** (20+ layouts) | comm_event | maintain |
| Internet session records | IPDR | **done** | data_session, app_usage | maintain |
| Subscriber/registration, handsets | operator lists, TAC | **done** (subscribers gate not met) | subscriber_*, device_tac | close gate |
| Cell sites/tower data | site lists, tower dumps | partial | cell_site | tower dumps P1 |
| Bank/wallet statements | CSV/XLSX | **done** (gate met) | bank_transaction | PDF/image statements via OCR (P1) |
| Vehicle-camera sightings | camera exports | **done** | plate_sighting | + media plates (WP-M5/M6) |
| Documents | DOCX, TXT, PDF | **done** (retrieval, mentions) | passages, mentions | scanned PDFs (WP-M4) |
| **Message exports** | WhatsApp .txt/.zip, SMS backup XML, Telegram JSON | **open** | new `message` family | **P1 (high)** |
| **Phone extraction reports** | UFDR/UFED/Oxygen/XRY CSV/XLSX/HTML | **open** (inventory only) | contacts, calls, messages, locations | **P1 (high)** |
| Email | EML/MSG/MBOX | open | message family | P2 |
| Location history | Google Takeout JSON, app exports | open | location events | P2 |
| Browser/app history | Chrome/Firefox SQLite, Takeout | open | web events | P3 |
| Access/door/VPN logs | CSV | open (migration 022 numbering plan) | access events | P3 |
| Images, audio, video | all | partial (media tracks) | media tables | **P1 (focus)** |
| Archives, SQLite, PCAP, EVTX | | inventory only | child evidence | P2 |
| Financial documents | cheques, FBR/tax forms, invoices | open | OCR + parsers | P3 |

### 5.5 Quality architecture
Per file and per case: accepted/duplicate/rejected counts with reasons; **statement integrity checks** for financial files (opening + credits - debits = closing, running-balance continuity; failures are shown as "gaps or edits in the statement" indicators); time-basis state for every typed row (`declared`, `explicit_offset`, `confirmed_by_analyst`; never assumed); unit and range checks; layout confidence; a **data-quality ledger** surfaced on the Overview and Evidence pages, never a ring without a question it answers.

## 6. The Ask engine: runtime language-model use, end to end

### 6.1 The path (as built, with the additions marked)
```
question (any language) -> [conversation role: greeting/help/clarify/scope (OFF today)]
   -> frame (what is asked: family, entities, time, measure, parameters read from the question by code into runtime settings)
   -> coverage (what this case holds: views, domains of columns, languages)           [exists]
   -> demonstrations (verified examples selected by similarity, as data)              [exists]
   -> query writer (model writes ONE read-only SELECT over reviewed views)            [exists]
   -> validator: parse tree + obligations derived from the question (pair, direction, order, count/sum, value domain, time) [exists]
   -> repair loop (bounded, messages say what is wrong)                               [exists]
   -> executor (read-only role, statement timeout, row/column walls)                  [exists]
   -> evidence tracing (rows behind each figure; records behind each summary view)    [exists: PART A / A2]
   -> answer composition (first sentence from the computed result under a number lock) [exists]
   -> reading (model interprets the computed result in plain words, grounded)         [exists: PART B]
   -> render (figure | table | chart | map | graph | timeline | passages | media moments)
```
Additions: media views and retrieval for transcript/OCR text; the conversation role; cross-case scope; follow-up context; streaming of phases.

### 6.2 The model roles (a registry, each with a contract)
| Role | Job | Model class | Guard | Status |
|---|---|---|---|---|
| R1 Frame reader | what kind of request, which family/case/scope, parameters | small instruct + code parse | schema-constrained output; code reads numbers/dates | exists (IR) |
| R2 Query writer | one SELECT over analyst views | 4B CPU now; 8B to 14B on GPU | validator, obligations, read-only, walls | **live** |
| R3 Verifier | checks the query asks what the question asked | **code first**, model second opinion optional | never lets a failing query run | live (code) |
| R4 Result reader | explains the computed result in plain words | same model | may only restate computed values; number lock | live (PART B) |
| R5 Document answerer | answers "what does it say" from retrieved passages with citations | same | quote-only, passage ids, PII guard | partial |
| R6 Media answerer | media questions: structured via views, text via retrieval, moments as links | same | label "model observation" | **to build** |
| R7 **Conversation role** | greetings, thanks, "what can I ask", clarification, refusal explanations, scope help | small and fast | answers only from case coverage and capability list; never invents data | built, OFF → **activate after measurement** |
| R8 Translator/transliterator | Urdu, Roman Urdu, English, Punjabi/Pashto glosses | local LLM / IndicTrans2 | labelled "machine translation" | partial |
| R9 Summariser/report writer | case summary, timeline narrative, report sections | same | every sentence tied to evidence ids; number lock; human sign-off | partial (reports) |
| R10 Extractor | entities/mentions from documents, OCR, transcripts | schema-constrained | validators (checksums, patterns) | partial |
| R11 Describer (VLM) | image/video captions | Qwen3-VL (GPU) | "model description, unreviewed", never counted | to build |
| R12 Judge | offline evaluation of answers | larger model or human | not in the live path | tooling exists |

**Greetings and small talk (owner request).** R7 is a real model role, not a list of canned strings: it receives the user's message, the case's coverage summary and the capability list, and answers briefly in the user's language ("Hello. This case holds call records for 12 numbers, bank statements and 14 documents. You can ask about contacts, durations, money in and out, or what a document says."). It is measured (does it ever invent a capability or a figure? does it route a real question to the engine?) before it is switched on; a failed measurement keeps it off.

### 6.3 Understanding, context and follow-ups
* **Conversation state per thread:** the last frames (family, entities, time window, parameters) are passed to R1 so "and for the last week?", "what about the second number?", "show them on the map" resolve against the previous turn. The resolved frame is shown to the analyst ("Interpreted as: calls of 923000001002 between 1 and 28 February") with a one-click edit.
* **Clarification is a successful answer:** when the request is ambiguous or a parameter is missing ("long calls": how long?), the engine asks one specific question and offers the options as buttons.
* **Languages:** English, Urdu (Nastaliq/Naskh), Roman Urdu and mixed; Punjabi/Pashto/Saraiki questions are evaluated as a language programme (PL; native reviewers needed, D10). Dictation: the analyst may speak the question; our ASR transcribes it, and the transcript is shown for correction before it runs.
* **Runtime parameters:** numbers an analyst asks for (minutes, days, thresholds) are read from the question by code into settings that the views read (`askParams` registry); never constants in SQL or code.

### 6.4 The "no templates" boundary, stated so it can be tested
Allowed: the schema description and column domains read from the case at run time; validators and obligations computed from the question's own parse; retrieval-selected demonstrations shown to the model; executors; evidence tracing; wording of the **first sentence from the computed result** (a presentation of data, not a canned answer). Not allowed: a table mapping question patterns to SQL or operations; keyword ladders that choose canned answers; per-question special cases. **Test:** an ablation run with the demonstrations removed must show the model still writes correct SQL for the families whose views are well described; the share of answers that depend on a demonstration is reported at each milestone, so examples cannot become templates by the back door. The old keyword ladder and registered operations (11 template-only correct answers on the 2026-09-29 shipped posture) are retired family by family behind switches with "0 correct lost, 0 new wrong".

### 6.5 Performance and the model path
CPU: p50 about 30 s, p90 about 50 s, reading 43 to 57 s (limit 120 s); plan cache of 509 entries; one request at a time. Levers in order: (1) a GPU and an 8B to 14B model (about 10 times faster and more accurate), (2) streaming of phases so the wait is explained, (3) caching of coverage and demonstrations, (4) a small router model for R1/R7. **A model swap needs the written decision rule of WI-0** (AGENTS.md); the rule for the GPU bake-off is drafted in `06_roadmap_phases.md` (Phase 1), measured on the verified bank plus a fresh judgement case, with "0 correct lost, 0 new confident-wrong" as the floor.

### 6.6 Safety
Prompt-injection: media text and documents are data in quoted blocks; the model never sees rows as instructions; tools are read-only. Privacy: subscriber names never returned, national ID numbers masked, plate masking per D28, "withheld" cells marked. Abstention: when the query cannot be shown to ask what was asked, the engine says so (this is why 7 bank questions abstained: honest, but the fix list in section 2 of file 01 closes them).

## 7. Conversations and history (like a chat application, but evidence-grade)

### 7.1 Model
`conversation_thread(id, case_id|null, scope_case_ids[], owner, title, pinned, archived, shared_with[], created_at, updated_at)` · `conversation_turn(id, thread_id, idx, parent_turn_id, question_text, language, frame_json, query_text, query_hash, result_ref, answer_text, evidence_refs, engine_version, model_id, data_version_hash, timings, state, error_reason)` · `turn_feedback(turn_id, user, verdict, note)`. `result_ref` points to a compressed **snapshot of the result rows** (size-bounded; larger results are re-runnable instead); `data_version_hash` is the hash of the evidence versions and effective-record version the answer was computed on.

### 7.2 What the analyst can do
* **History rail** (as in chat apps): grouped by day/case, search (full text over questions and answers), rename, pin, archive, delete-with-audit (deletion hides, the audit keeps the fact).
* **Continue** any thread; **edit and resubmit** a question (creates a branch); **fork** a thread from any turn; **re-run** a turn against current data and see **"changed since"**: a diff of the figures and of the evidence set (this is the "reworking" the owner asked for).
* **Pin to findings:** an answer (with its evidence and snapshot) becomes a finding/exhibit; findings are editable text with live evidence references.
* **Compare** two turns or two runs side by side.
* **Share** a thread to case members (read-only link inside the app); permissions follow case membership.
* **Export** a thread or a selection to the report builder.
* **Reproducibility:** each turn shows engine version, model, data version, and a "reproduce" action; a court exhibit prints these.
* **Retention** follows the case; per-user private threads are allowed but visible to auditors.

### 7.3 API (new)
`POST /conversations`, `GET /conversations?case_id=&q=`, `GET /conversations/{id}`, `POST /conversations/{id}/turns` (runs the engine; streams phases), `POST /turns/{id}/rerun`, `POST /turns/{id}/fork`, `PATCH /conversations/{id}`, `POST /turns/{id}/pin`, `POST /turns/{id}/feedback`, `GET /turns/{id}/diff?against=`.

## 8. Structured data families: how a family is added (so every chat does it the same way)
1. **Layout spec** (provider layouts in the registry: header patterns, column meaning, units, time basis, party roles) with synthetic samples and, when available, a vault-confirmed real sample.
2. **Reader and normaliser** (encoding, BOM, date/time formats incl. Excel serials and time zones, phone numbers to E.164, plate/IBAN/IMEI normalisers, amounts and signs).
3. **Typed writer** to a typed table with `source_row`, `evidence_id`, time-basis state and the idempotency key.
4. **Migration + views** in `analyst` (thick, curated, documented columns; `case_id`-carrying; effective-records variant).
5. **Engine description + one verified example** per family operation (data only).
6. **Generator + oracle** (planted truth) in the case generator or the reference-case builder; **bank** written against the data; **judge**.
7. **Gate** pre-registered (G1 typing, G3 oracle equality, G4 fresh-case engine test with 0 wrong).
8. **Viewer** (file viewer + family view) and **Playwright test on a generated case**.
9. **Documentation** (`docs/data/<family>.md`) and the coverage entry.
Families to add next, in order: message exports, phone-extraction reports, tower dumps, PDF/image bank statements via OCR, email, location history.

## 9. Reports and exports
A report is an **ordered list of references** (findings, pinned answers, charts, media exhibits, timeline excerpts) with an optional narrative written by R9 under the number lock; every sentence carries evidence ids; formats PDF and DOCX; **hash manifest** (originals, derived, report), custody log extract, reproducibility block (engine, model, data version), reviewer sign-off with name and time; versioned; exports of media use clips/crops with burned-in hash and time. `POST /reports/generate` exists; the builder UI, sections, sign-off and bundle are new (stage U8).

## 10. Identity, security, audit and operations
* **Identity:** local accounts first (offline), optional OIDC; MFA; roles `admin | lead | analyst | reviewer | viewer | auditor`; permissions `export`, `cross_case`, `face_search`, `vault_access`, `reprocess`, `delete_with_audit`.
* **Row-level security** by tenant and case membership; the engine's read-only role sees only views; secrets never in images or logs.
* **Audit log:** append-only, hash-chained, with a UI for auditors: logins, access, searches (including every face search with its stated reason), exports, cross-case queries, role changes, model runs, evidence actions.
* **Backup and restore:** nightly dump + evidence volume snapshot; a **tested restore** (a restore on this laptop takes about 9 minutes and needs free memory); restore rehearsal before every migration.
* **Deployment:** compose now; an offline installer with a model registry (hashes), later; observability from `engine-health` and queue depth in the Activity page; no uncontrolled `latest` tags (every live image has a rollback tag).
* **Scale:** benchmark at 10 times the Orion volume before the enterprise phase; partition or hypertable the event tables by case and time; index review.

## 11. Contracts the interface needs (the 30 backend requests, grouped)
BR-1 case record and census (members, authority, classification, readiness, summary, directory); BR-2 conversations (section 7.3); BR-3 knowledge capture (notes, findings, hypotheses, tasks, tags, saved views, annotations); BR-4 activity/audit/reports (global history, report builder, exports); BR-5 media (media list, derived artifacts by locator, review actions, clock offsets); BR-6 cross-case (notices, access requests, registry). `docs/work/BACKEND_REQUESTS.md` is the register; this pack's phases deliver them in the order the interface stages need them.


---

# 04. The UI/UX programme: recreate the whole application, enterprise grade

**Owner's request, 2026-11-03.** Recreate and redesign the entire interface following the instructions already given (the master UI/UX directive), because Codex is not available now. Specifically: the **Ask** page does not show its sections and answers completely, wastes side space and padding and is too compact, so tables and answers do not appear appropriately; **every evidence file must open previewed best, accurate and complete, whatever its type**; the best stats, visualisations, charts, figures and graphs from what the system computes, in dashboards and pages; a professional, up-to-date, enterprise-grade, clean, elegant, sophisticated, modern, easy-to-use, productive interface. Standing rule since 2026-10-13: **every capability is unfinished until it has its view, a Playwright test on a generated case and an owner demonstration.**

---

## 0. Who does what, where, and under which rules

| Fact | Detail |
|---|---|
| Directive of record | `docs/ux/MASTER_UI_UX_DIRECTIVE.md` (19 sections, stages U1 to U10, merge ledger, research register R1-R60) with the UX contract `docs/ux/NEXUSAI_PRODUCT_UX.md`. **Read both in full before touching the interface.** |
| `:4181` | The lane tree through Vite = **the old design with every working function and engine** (owner rule 2026-10-28). It already carries the 2026-11-02 Ask and Evidence redo (frameless 1200 px Ask column, answer cards, proof panel, paged file viewer, cited-row card). **Keep it working; never merge an interface stage into `lane-arms` on the strength of "accepted"; say what 4181 will look like afterwards and ask once more.** |
| `:4182` | The Codex worktree `C:\Users\sheik\Workspace\Office\Projects\NexusAI-ui`, branch `ui/u2-shell-navigation` (77 commits ahead of `lane-arms`, 8 behind). **U1 accepted** (colour direction **B, Ink and signal**; DTCG tokens, 160/160 contrast pairs, 150 aliases for old names). **U2 built** (shell "B · Case atlas": 56 px header, 256/64 px rail, 48 px case band, command palette, appearance menu, 12/12 viewport-theme combinations, 49/49 focused tests) and **awaiting the owner's review**. U3 to U10 not started. |
| Executor from now | **Claude, in that worktree**, under the same directive and the same per-stage loop (research note, two or three options the owner can look at on `:4182`, the owner decides, build over the existing tested data hooks, prove, close with a stage note and evidence). The worktree needs the owner's env file copied by the owner (never read or printed by Claude) and `npm ci`. |
| Forbidden to the interface executor | `api/**`, `db/**`, `ingestion/**`, `tools/**`, `evaluation/**`, `docs/data/**` (file backend requests in `docs/work/BACKEND_REQUESTS.md` instead). The backend executor edits no interface path while a stage is open and says so in `NEXUSAI_CONTINUATION.md`. |
| Real data | Screenshots, fixtures and design tools receive **synthetic cases only** (Orion, the wave-1 cases). O-8 (offline tile licence) blocks any screenshot leaving the machine. |
| Dependencies | Approved: react, react-dom, react-router-dom, lucide-react, echarts, cytoscape, maplibre-gl, yaml, Noto fonts, vite/vitest/Testing Library/Playwright. **Everything else needs the owner's yes** with the trade-off written; the media viewers need six proposals (section 7). |

## 1. What the redesign must fix first (acceptance list taken from the owner's words)

1. **Ask: use the width.** No decorative side gutters beyond 16 to 24 px; the answer column fills the space to a readable maximum for prose and goes full width for tables and media; **tables, charts, maps and graphs are never squeezed into a bordered card with their own padding**; sections do not hide: every section of an answer (headline, interpretation, table, chart/map/graph, evidence, "how this was computed", actions) is reachable and **visibly complete** (row counts "50 of 408", no silent truncation).
2. **Evidence: open anything, show everything.** The viewer rule of `03_architecture.md` 5.3: if it is stored it opens; completeness stated; the file's own headings and order; the cited row and its neighbours; documents with all passages in order; media with synchronised transcripts and boxes.
3. **Dashboards that answer questions.** Every tile and mark opens the rows behind it. No decorative numbers, no fabricated trends, no empty chart: a state the service cannot speak for is a designed state.
4. **One visual system**, light and dark and high contrast, English and Urdu (right-to-left) deliberately handled, keyboard complete, WCAG 2.2 AA.
5. **Speed that is visible:** the interface never waits silently; the Ask page narrates phases ("understanding, writing the query, checking it, running it, reading the result").

## 2. Design language (decided; do not reopen without the owner)

* **Colour direction B "Ink and signal"** in light, dark and high contrast; one accent; colour only for meaning; evidence strength by **word and shape** (observed, inferred, candidate, confirmed), never colour alone. The owner liked the old teal themes and rejected the "Paper" palette on the whole app (D23); direction B was chosen by looking (O-3).
* **Type:** Noto Sans Variable (Latin), Noto Sans Arabic for Urdu UI strings, Noto Sans Mono for identifiers and tabular figures. Urdu **body text** in Nastaliq (a Noto Nastaliq Urdu face) needs a native reader to approve (O-6); until then Naskh via Noto Sans Arabic.
* **Bidirectional text:** `dir="auto"` on user content, `<bdi>` around identifiers, **phone numbers, plates and account numbers always left-to-right**, logical CSS properties, mirrored layout only for fully Urdu UI.
* **Space and density:** the space tokens 8/12/16/20/24/32/48; density modes comfortable, default, compact (a global setting); tables default to compact with 40 px rows and sticky headers.
* **Depth and motion:** quiet; motion only where it explains (arrival of a result, selection, source-row tint); zero motion when the user prefers reduced motion.
* **Forbidden looks:** flat white or black canvas, purple "AI" gradients, neon, glassmorphism, card soup (every element in its own bordered card), emoji as status, skeleton shimmer that outlasts the request, fake progress, looping motion, tooltips holding the only copy of important text, icon-only controls without a name, more than one accent.
* **Language of the interface:** no jargon for analysts (never model, agent, operation, template, processor, backend, SQL, vector, embedding, threshold, token count); one level down for administrators.

## 3. Information architecture

**Global (rail):** Dashboard · Ask across cases · Cases · Activity · (Library: reference lists such as handset types, operators, cell sites, plate formats, for administrators) · Settings. **Header:** case switcher with the full case name, command palette, appearance, user. **Case band (sections):** Overview · Evidence · Ask · Analysis (Connections, Places, Timeline, Money, Vehicles, Media) · Entities · Findings · Reports · Activity/Audit · Access.

| Page | Purpose (one idea) | Primary actions |
|---|---|---|
| Dashboard | what needs attention across my cases | open a case, resolve an attention item, continue a thread |
| Cases / New case | find and create | create (wizard), open, filter |
| Case Overview | the dossier: what this case holds and what to do next | ask, add evidence, open a finding |
| **Ask** | question to defensible answer, with history | ask, refine, pin, export |
| Evidence | what is in the case and its state | add, open, reprocess, tag |
| Evidence viewer | the file itself, complete | read, search, cite, review |
| Connections / Places / Timeline | how things link, where, when | filter, select, ask about this |
| Money | where money came from and went | trace, compare, flag |
| Vehicles | plate sightings and routes | search a plate, replay a route |
| **Media** | gallery of images, audio, video, faces, plates, text found | filter, review, compare |
| Entities | people, numbers, accounts, vehicles, devices | merge candidates, notes |
| Findings | claims with evidence | write, reorder, export |
| Reports | build and sign | assemble, preview, sign |
| Activity | processing and history | retry, inspect |
| Audit / Access | who did what; members | review, grant |

## 4. The pages in specification form

### 4.1 Ask (stage U4; the owner's most urgent page)
**Layout (wide screens):** three zones in one frame: a collapsible **history rail** (left, 280 px), the **conversation** (centre, fills the rest to a 1280 px maximum for text blocks, full width for tables/media), and a collapsible **evidence drawer** (right, 420 px) that shows "behind this answer" for the selected turn or mark. On narrow screens the rails become sheets. The composer is pinned at the bottom with a scope line ("Case Orion · all families · 1 Feb to 3 Mar") and suggestion chips **generated from the case's coverage** (not canned).

**Answer anatomy (fixed order, every part optional but never hidden):**
1. *Interpretation chip* ("Interpreted as: ..."), editable.
2. *Headline* (the engine's first sentence, number-locked) with the evidence-strength word.
3. *Result block*, by shape: figure card; **table** (sticky header, resizable and pinnable columns, sort, filter, copy, CSV, "open source row"; virtualised beyond 500 rows; always "showing 1 to 50 of 408"); chart; map; graph; timeline; passages; **media moments** (thumbnail strip, audio snippets with transcript lines, "open at 00:12:34").
4. *Reading* (the model's plain-words interpretation, labelled, with the computed values only).
5. *Evidence tabs:* rows behind the figure (with the file and row in words), records behind each summary view, per-line checks, source row on demand.
6. *How this was computed* (plain-language steps; the technical query one level down for administrators).
7. *Actions:* pin to findings, ask a follow-up, re-run, compare with earlier, export, "this answer is wrong" (feeds the triage bank).

**States designed as results, not errors:** clarification, abstention ("I could not show that the query asks everything you asked; here is what is missing and the options"), refusal, too broad, no data in this case, waiting (phases), partial, failed with next step.

**History rail:** threads grouped by day, search, pin, rename, archive, share; opening a thread restores the page; "changed since" badges on re-runs (`03_architecture.md` section 7).

**Acceptance (proved by screenshot at 4 widths × 3 themes on Orion):** an answer with a 408-row table, a chart and a map shows all of them, with no horizontal page scroll, no clipped section, no empty side bands wider than 24 px, keyboard path from composer to evidence drawer, and the same page in Urdu.

### 4.2 Evidence and the viewers (stage U6)
* **Library:** table with type, state (per-stage progress), rows/pages/duration, languages, quality, added by/when, tags; filters by family/state/tag; bulk reprocess/tag; "add evidence" with drag and drop, folder and archive upload, per-file progress, type detection preview, the mapping wizard for unknown layouts.
* **Evidence passport** (first tab of every file; `03_architecture.md` 5.2).
* **Viewers:** the registry of `03_architecture.md` 5.3 in the new design system, carrying over **FileRowsViewer, SourceRowCard and the document passage assembly** built on 4181. Specific requirements: spreadsheets show the **cell address** and the file's order, dates read as dates with the raw serial one click away; PDFs show a page rail and OCR overlay; text exports show lines with encodings; each viewer has search, "cite this" (creates a citation chip usable in findings), and the completeness line.
* **Media viewers (with WP-M0/M2/M3/M6):**
  * *Image:* zoom/pan with tiles, EXIF panel, OCR overlay toggles, face/plate boxes (governed), "similar images", compare two images side by side, blur-by-default option.
  * *Audio:* waveform with region selection, **synchronised transcript** (click a word to play, follow-along highlighting), speaker lanes, per-segment language chips, low-resource banner, spoken-number highlights, translation toggle, edit-and-verify with history, link to the matching call record.
  * *Video:* player (HLS/MP4 proxy), **event scrubber** (shots, motion, tracks, plates, faces, speech as lanes), frame step, clip cutter with hash watermark, detections overlay with confidence, transcript pane, clock-offset control showing both stamps.
  * *Faces (when enabled):* person-cluster grid, face detail with every appearance, **compare** two faces (side by side with the "this is not an identification" notice), search with a candidate list and the error-rate statement, review queue.

### 4.3 Dashboard, Cases, New case, Case Overview (stage U5)
* **Dashboard (hero):** a band answering "what needs me today": attention queue (failed ingests, items awaiting review, new cross-case notices, long-running jobs), KPI tiles over **real** values (cases, evidence by state, questions asked, findings), per-family coverage, model/queue health, continue cards (last threads, last viewed evidence). Every tile opens its rows.
* **New case wizard:** name and number; case profile (type, jurisdiction, time zone and **clock basis**, languages expected, capabilities to enable such as faces, with a place for the authority reference); members; evidence upload with live detection; first-readiness summary with suggested questions.
* **Case Overview (dossier):** header with state and owner; **what this case holds** (family cards with counts and quality), activity calendar, key entities and numbers (top contacts, accounts, plates) as real mini-visuals, data-quality ledger, next steps, recent findings and threads, cross-case notices. No ring without a question it answers.

### 4.4 Analysis pages
* **Connections (workbench, stage U3):** links, places, table, timeline coordinated; layout B was chosen by the owner; restyle in the new system plus the research items already specified (table twin and marks listbox, time-bar brushing with a reset chip, level-of-detail labels, hidden-count banner, uncertainty circles for cell sites, muted basemap, stable layout seed).
* **Places:** cell sites with **uncertainty** (a coverage area, not a position), visits, dwell, movement anchors, speed alerts, co-location; offline basemap; heat and cluster layers.
* **Timeline:** one lane per family (calls, messages, data, bank, plates, media moments, documents), brush to filter, annotations stored server-side.
* **Money (new, with the transactions family):** running-balance line, in/out waterfall, counterparty bars and **Sankey** of flows, structuring bands (just below limits), velocity, circular flows; every mark opens the statement rows.
* **Vehicles (new):** plate search, sightings table, camera map, route replay, convoy candidates.
* **Media (new):** gallery with filters (type, language, date, camera, review state, has faces/plates/text), search by words (OCR, transcript, caption) and by example image, face clusters, plate hits, review queue shortcut.
* **Entities and drawer (stage U7):** "Ask about this" from any chip; mentions across families; cross-case notices; notes.

### 4.5 Findings, Reports, Activity, Settings (stages U7, U8)
Findings editor with live evidence references; report builder (sections, exhibits, number lock, sign-off, bundle); Activity with real job history (backend request 27); Settings (appearance, languages, density, shortcuts, model roles status for administrators); Admin (users, roles, audit viewer).

## 5. The visualisation catalogue (what to draw, from which view, with which library)

All charts use the token-driven ECharts theme; the graph uses Cytoscape; maps use MapLibre with the offline raster basemap. Every chart has a **table twin** and an **honest empty state**, and every mark opens its rows.

| Question the analyst has | Visual | Source | Notes |
|---|---|---|---|
| Who does this number talk to most? | ranked bars with call count and talk time | `v_contact_pairs` | click a bar: the pair's events |
| When is this number active? | **hour × weekday heatmap** and a daily calendar | `v_comm_events` | night/odd-hour highlight |
| How long are calls? | distribution (histogram) and the longest-call list | `v_comm_events` | parameters (minutes) read from the question |
| First and last contact, bursts | event strip with burst detection | `v_comm_events` | |
| Who connects whom? | **link chart** with communities, degree/betweenness sizing, time slider, path finding, ego network | `v_connection_edges` | cap and level-of-detail rules; candidate edges dashed |
| Where was the phone? | map of cell visits with **uncertainty circles**, dwell, movement trail | `v_cell_visits`, `v_movement_anchors` | "may have been near", never "was at" |
| Who met? | co-location windows matrix and map | `v_colocation_windows`, `v_meeting_candidates` | proximity is not association |
| Did the rhythm change? | before/after bands | `v_rhythm_changes` | |
| Which handsets/SIMs? | pairing timeline, SIM-swap markers | `v_device_pairings`, `v_successor_candidates` | |
| What did the device do online? | app/host treemap, bytes up/down area, session timeline | `v_data_sessions`, `v_app_usage` | |
| Where did money go? | running balance, waterfall, **Sankey** counterparties, velocity, limit bands | `v_bank_transactions`, `v_account_flows`, `v_account_limit_bands` | statement integrity banner |
| Where was the vehicle? | camera map, sightings timeline per plate, route replay | `v_plate_sightings`, `v_plate_visits` | plate masking per D28 |
| What do documents mention? | mention network, term timeline | `v_document_mentions` | |
| What is in the media? | counts by type/language, face-cluster grid, appearance timeline, speech language lanes | media views (033) | "model observation" label |
| Is the evidence healthy? | stacked accepted/duplicate/rejected, layout confidence | `v_evidence_inventory`, typing report | |
| What is the portfolio doing? | cases by state, evidence by state, queue depth, questions per day | case/evidence reads | real series only |

**Runtime visual selection (consistent with "no templates"):** the Ask result block chooses a chart **by the shape of the returned data** (the existing `resultShapes` logic), never by the question text; the model may *suggest* follow-up visuals as validated queries over the same views.

## 6. Quality, accessibility and performance (per stage, binding)
* Tests: vitest for every state/mapping/count a stage adds (the existing 549 stay green); **Playwright screenshot tests on generated cases at 1440, 1280, 1023 and 390 px × light, dark and high contrast**; axe checks; a keyboard-only path per page.
* Budgets: INP p75 under 200 ms; initial JS/CSS gzip budgets from the U1 baseline (foundation CSS 3.8 KB gzip); virtualised tables beyond 500 rows; no layout shift on result arrival.
* Right-to-left audit of every page at U10; native screen-reader pass of the core loop ("go to a case, open Ask, open a source row").
* **Usability test** with real analysts at U10 (names owed: D26): unaided "add data, ask, open a citation, pin a finding".

## 7. Dependencies the media interface will need (proposals; each needs the owner's yes and a licence-ledger row)
| Proposal | For | Licence [K] |
|---|---|---|
| wavesurfer.js | audio waveform and regions | BSD-3-Clause |
| hls.js | HLS video proxies | Apache-2.0 |
| PDF.js | PDF pages with text layer | Apache-2.0 |
| OpenSeadragon (or tiles in MapLibre) | very large images | BSD-3-Clause |
| a virtual-table helper (TanStack Table/Virtual) | 10k-row tables | MIT |
| docx-preview or server-side render | Word documents | MIT / server |
Rendering Word, Excel and PDF **on the server to a safe PDF/HTML** is the alternative that avoids shipping parsers to the browser; decide in the U6 research note.

## 8. The stage plan (revised order, a proposal for the owner's yes: D37)

The owner-set order was U2 review, U5 (section by section), U3, U4, U6. The owner's new message puts Ask and Evidence first, and U4/U6 are the pages with the most backend work waiting (conversations, media). Proposed order, with parallel backend work:

| Stage | Content | Needs from the backend | Proof the owner looks at | Weeks |
|---|---|---|---|---|
| U2 close | owner reviews the case-atlas shell on `:4182` | none | the review | 0.5 |
| **U4 Ask + History** | section 4.1 | BR-2 conversations (Phase 1); until then the page uses the current client-side turns and says so | the answer anatomy on Orion, every state, history rail, keyboard loop, Urdu | 3 |
| **U6 Evidence + viewers v1** | section 4.2 (structured, documents, text, images, audio v1, video v1 with WP-M0) | file-rows (live), media proxies (WP-M0) | open every one of the 53 Orion files plus the media samples; "unaided add data and open a citation" | 3 |
| U5 Dashboard, Cases, New case, Overview | section 4.3 | BR-1 (honest states until it lands) | no fabricated number; every tile opens rows | 3 |
| U3 Workbench restyle | section 4.4 | none | the same walkthrough in the new system | 1.5 |
| U7 Entity drawer, findings, saved views | 4.4 and 4.5 | BR-3 | "Ask about this" from any chip | 2 |
| U9 Media workbench and review desk | media pages, review desk | WP-M2..M8 | review with reason and undo | with media |
| U8 Activity, Settings, Admin, Reports | 4.5 | BR-4 | honest states; roles | 2 |
| U10 Polish, sweep, usability test | everything | none | the unaided test; alias layer and classic Connections deleted | 2 |

## 9. Cut-over: when `:4182` replaces `:4181`
Criteria, all required: (1) **route and function parity** with the checklist generated from `:4181`'s 17 routes and every action in them; (2) all vitest and Playwright suites green on `:4182`; (3) the owner accepts each page by looking; (4) the Ask and Evidence carry-overs are done; (5) a rollback tag `ui-old-final` on the last old-design commit. Then, and only then and with the owner's word, merge the UI branch into `lane-arms`, restart the workspace, and keep `:4181` as the old design on a different port for one week. Until the owner says so, **`:4181` keeps the old design.**

## 10. What to do first in the interface lane
1. Owner reviews U2 on `:4182` (nothing to build; remind them).
2. Port the 2026-11-02 Ask/Evidence components into the new system as the starting point of U4/U6 (they hold the learned fixes: completeness lines, file headings, cell addresses, passage assembly).
3. Write the U4 research note and two Ask layouts as **real routes on `:4182`** (`/design/ask-a`, `/design/ask-b`) over the same data hooks, so the owner can look at both on Orion before a line of production markup is final.
4. In parallel the backend lane delivers BR-2 (conversations) so the history rail is real.


---

# 05. Data, testing and research

**Owner's request, 2026-11-03:** for testing and use, fetch and research the best, most appropriate, realistic and relevant data from online sources until real data is provided; do not get stuck, repeat tasks or run long test cycles.

Marks as in file 01. Everything in the dataset register below was **found today by search and, where marked [V], read on the primary page**; licence statements marked [S] come from a search summary or a mirror and **must be read on the primary page before the download is logged**.

---

## 1. Rules for acquiring and using data

1. **Standing permission, with the ledger.** The owner's message is taken as a standing yes to *search for and fetch public, licence-clean datasets for evaluation and fine-tuning*, subject to the existing process, which is not relaxed: one **licence-ledger row per dataset** (source URL, licence text read, version, size, SHA-256, intended use, partition, who fetched, when); a **written decision rule** before the first measurement; hash manifest and short-read guard (`tools/media_data/` has the fetch/verify kit); the **size stated before any download above 2 GB**. A non-commercial or unclear licence means **evaluation-only** and is recorded as such; unknown means do not download.
2. **Synthetic first, public second, real last.** Tiers: **T0** exact fixtures; **T1** synthetic with planted truth (the case generator, the reference cases); **T2-V** controlled virtual scenes with exact truth; **T2-P** physical captures (owner); **T3** public licensed; **T4** authorised real. Every report states which tier it used.
3. **Real data stays local.** The supplied real files live in the vault (`docs/data/real-data-handling-policy.md`): counts and booleans only in reports; never in the repository, a screenshot, a cloud tool, a prompt or a design tool. The two RAR archives from the team lead are vault samples.
4. **Cloud compute is allowed for public/synthetic data only** (Kaggle/Colab/rented GPU), with the owner's approval per run, never with evidence or labels derived from evidence. Weights and metrics come home with their manifests.
5. **Synthetic identity documents carry a visible watermark** ("SYNTHETIC SPECIMEN") and numbers outside real ranges; no realistic forgeries are produced.
6. **Partitioning:** the last hex digit of `sha256(id)`: 0 to b development, c to f sealed. Sealed sets are never tuned on and are touched once per gate.

## 2. Dataset register (search results of 2026-11-03)

### 2.1 Speech
| Dataset | Content | Licence | Size (as read) | Use | Notes |
|---|---|---|---|---|---|
| Common Voice Scripted Speech 24.0, **Urdu** | read speech | CC0-1.0 [S] | 252,899 clips, 302 h, 81.5 h validated, 498 speakers [S] | train + test | `datacollective.mozillafoundation.org/datasets/cmj8u3pz600t9nxxbz9l2ck2n`; the site moved to `mozilladatacollective.com` and renders its pages with script, so read the datasheet in a browser |
| Common Voice, **Pashto** | read speech | CC0 [S] | 113 h in v24 [V arXiv 2604.06507]; 82 validated h in v23 [S] | train + test | the v25 page's headline figures (5,313 h) look wrong in the summary; verify before use |
| Common Voice 24.0, **Saraiki** | read speech | CC0-1.0 [S] | 5,837 clips, 6.75 h (4.33 validated), 60 speakers [S] | train + test | the only open Saraiki ASR data found; small |
| Saraiki 10-hour dataset (TTS-oriented) | studio speech | CC BY-NC-SA 4.0 [S] | about 10 h | **research only** | not for the product |
| FLEURS (`ur_pk`, `ps_af`, `pa_in`, `sd_in`) | read speech | CC BY 4.0 (ledger DAT-004) [M] | about 12 h per language | test + train | `pa_in` is Gurmukhi (Indian Punjabi), not Shahmukhi |
| Omnilingual ASR corpus | 350 under-served languages | CC BY 4.0 [V README] | large | train + test | check which of `pnb_Arab`, `skr_Arab`, `pus_Arab`, `hno_Arab` it holds |
| Noise and room-response sets for augmentation | noise, RIRs | MUSAN / OpenSLR sets [K] | small | augmentation | read each licence |
| Telephone-band audio | none public for these languages | | | | **simulate**: G.711 mu-law 8 kHz, AMR-NB, Opus low rate, noise, packet loss on clean copies; real calls only from consented captures (D26) |
| TTS-made test audio | | MMS-TTS is non-commercial [K] | | avoid | use public read speech with exact transcripts instead |

### 2.2 Text, translation, OCR and documents
| Dataset | Content | Licence | Use | Notes |
|---|---|---|---|---|
| Saraiki-English parallel corpus (51,447 pairs) and a Saraiki literature corpus (about 1 M tokens) | text | per page [S] | translation and language-model checks | Mozilla Data Collective |
| Urdu Newspaper Benchmark | 829 paragraph images, annotated | read the page [S LREC 2026] | OCR evaluation | confirm licence before use |
| Synthetic Nastaliq/Naskh lines | rendered from OFL fonts (for example Noto Nastaliq Urdu [K]) | ours | OCR train + test | generator in `tools/media_data/` |
| Receipts/forms sets (CORD, SROIE and similar) | scans | vary [K] | layout/OCR evaluation | read each licence |
| PuristanLabs Urdu crops | training crops | | **not valid test data** [M] | |
| Generated documents (wave-1 and Orion) | FIR, witness statements, memos, bank requests, utility bill, chat export | ours | T1 | exact truth |

### 2.3 Faces, plates, images, video
| Dataset | Content | Licence | Use | Notes |
|---|---|---|---|---|
| Open Images (face boxes) | photos | CC BY 2.0 photos [M] | detection evaluation (done) | labels incomplete: false-box rate unmeasurable |
| WIDER FACE, LFW, IJB-C and similar | faces | research terms [K] | baseline evaluation only after reading terms | no training for the product |
| **Consented South Asian face set** | volunteers, CCTV-like degradations | consent form | recognition evaluation, bias analysis | to build (D26, D34) |
| CCPD (Chinese plates) | plates in scenes | MIT [K] | backgrounds, robustness | not Pakistani |
| Roboflow "Pakistani-Number-plates" | 336 images | not read [S] | candidate small set | `universe.roboflow.com/liciece-plate` |
| PLPD (Yousaf et al., 2021) | Pakistani plates | ask the authors [S] | candidate | `ncbi.nlm.nih.gov/pmc/articles/PMC8622297` |
| Roboflow LPR v13 | plates | CC BY 4.0 [S] | detector pretraining | dataset terms apply to fine-tunes |
| **T2-V plate pack (ours)** | 96 virtual scenes, exact truth | ours | plates | [R] |
| **Synthetic Pakistani plates (to build)** | province templates, styles, dirt, blur, perspective | ours | recogniser training | grammar registry feeds the generator |
| MEVA | multi-camera surveillance, vehicles and people | CC BY 4.0 [S] | about 328 h public (516 GB): sample only | tracking and event recall; check the licence text on `mevadata.org` |
| UCF-Crime, UA-DETRAC, AI City | surveillance/traffic | research/NC [K] | evaluation only if terms allow | |
| COCO / Open Images | objects | CC BY mix [K] | detector checks | |

### 2.4 Telecom, financial and network (shape and scale tests; none is Pakistani)
| Dataset | Licence | Use |
|---|---|---|
| Telecom Italia Big Data Challenge (Milan, Trentino) | ODbL [S Nature Sci Data] | volume/shape tests for CDR-like activity |
| MIT Reality Mining (call logs, Bluetooth, towers) | academic [S] | structure checks |
| Orange D4D (Ivory Coast, Senegal) | by application [S] | antenna traffic |
| CRAWDAD traces | per trace [S] | various |
| IBM AMLworld (HI/LI, Small/Medium/Large) | CDLA-Sharing-1.0 for the data via a mirror [S]; code Apache-2.0 | transaction-graph tests with labelled laundering patterns |
| CTU-13, CIC-IDS | research [K] | flow/PCAP tests |
| OpenStreetMap | ODbL [K] | basemap, offline tiles; the owner's offline tile source is unverified (O-8) |

**Why these are not enough and what fills the gap.** No public dataset has Pakistani operator layouts, Pakistani plates at scale, Punjabi/Pashto/Saraiki telephone speech or consented South Asian faces. The case generator (20 cases, planted truth), the reference cases (Orion built; Meridian and Control to build), the T2-V scenes, synthetic plates, codec-simulated telephony and a small consented capture campaign fill the gap; the vault holds the real layouts.

## 3. Synthetic programmes (T1 and T2-V)

| Programme | State | Next |
|---|---|---|
| Case generator (P1 to P8, A1 to A13; planted truth; hash register; 98 tests) | 20 cases; any generator change invalidates the register | add media assets with truth |
| Reference cases (`docs/roadmap/reference-cases-programme-20261028.md`) | **Orion built** (53 files, 484 verified questions); Meridian (Islamabad, overlap) and Control (Karachi, decoys) planned | build with media: images (IDs with watermark, chat screenshots, scene photos), audio (read-speech clips with exact transcripts, in the five languages), video (T2-V scenes with plates and faces from consented volunteers only), cross-case plants and decoys |
| Media synthetic kit (`tools/media_data/`) | OCR/ASR/face/plate eval tools exist | speech gold-set builder with telephone simulation; plate generator; chat-screenshot generator; clock-offset scenarios |

## 4. Testing without repetition: the policy in operation

**Tiers (already binding):** T0 unit and live-database specs at commit; T1 `typing_report.py` (seconds, no model) after any layout/typing change; T2 `live_smoke.py [--quick]` after a deploy (3 to 5 minutes); T3 re-ask only the questions the change can touch; T4 full banks only at milestones, in the background (the verified bank is 484 questions at 40 to 130 s each, about 8 hours on the CPU laptop, about 1 hour with a GPU; read the summary, never wait).

**The change-impact matrix** (so that nothing is re-run needlessly):

| What changed | Run | Do not run |
|---|---|---|
| Engine obligation, validator or prompt | T0 for the module; **re-ask the questions whose results list that obligation/view** (the result files record `views`, `attempts`, `repairs` per question: build the impact index from them once); `live_smoke --quick` | the full bank |
| Wording of sentences | the wording specs; re-judge saved answers without a model (`factory.py reeffective`) | any model run |
| Layout or typing | `typing_report.py COLLECTION` | engine runs |
| Migration | rehearsal on a restored copy (about 9 minutes, needs free memory) + the migration's verify script | |
| Interface | affected vitest files; Playwright slice at 3 widths; look at both themes | the whole suite every time (the whole suite once per stage) |
| Media model or route | one evaluation container on the **development** partition; sealed partition once at the gate | parallel evaluations (the Docker VM is 7.6 GB; 2026-10-21 outage) |
| Fine-tune | development partition each epoch; sealed once | |

**Rules that stop repeated work:** (1) every run writes a result file keyed by *(bank hash, engine image id, switch set, data version)* and a run with an unchanged key is refused unless forced; (2) judgement and sealed cases are **spent** when used for a fix (register kept in the state file); (3) a failing question is re-asked alone with the full log kept; (4) a milestone run happens once per milestone, in the background, with a notification when done.

## 5. The banks and the judge

| Bank | Size | Truth | Gate |
|---|---|---|---|
| Verified bank (Orion) | 484 | independent SQL oracle (`build_verified_bank.py`) | 473/484 today; target 100% with honest abstentions counted apart |
| Fresh-case engine tests (G4) | 15 to 20 per family per fresh case | generator oracle | 0 wrong, per family |
| Media bank (new) | at least 20 per family (audio, image, OCR, plate, video, face) | exact truth of synthetic assets | 0 confident-wrong; every answer labelled "model observation" |
| Cross-case bank (new) | 30 | Meridian/Control oracle | zero false merges, planted overlaps found |
| Language bank (new) | Urdu, Roman Urdu, Punjabi, Pashto, Saraiki questions | native-reviewed (D10) | measured per language, reported apart |
| Conversation bank (new) | greetings, help, clarification, follow-ups, refusals | scripted expectations | no invented capability, no invented figure |
| Red team | 100 | scripted | instructions in OCR/transcripts, SQL escape, PII extraction: 0 successes |

**Reporting standard for every run:** n, correct / wrong / abstained / not run, Wilson 95% interval, latency p50 and p90, **cause of every miss** (engine fault, wording, data, infrastructure), the engine image and switch set, and the exact command to repeat it.

## 6. Research log

### 6.1 Established today
| Finding | Source | Mark |
|---|---|---|
| Omnilingual ASR is Apache-2.0 (code and models), CC BY 4.0 (dataset); models CTC 300M to 7B and LLM 300M to 7B plus "Unlimited" variants; inference limit 40 s per clip except Unlimited; CTC 300M about 2 GiB and 1B about 3 GiB VRAM, 3B about 8 GiB, LLM 1B about 6 GiB, 7B about 17 GiB (A100 table) | github.com/facebookresearch/omnilingual-asr | V |
| Its language list contains `urd_Arab`, `pus_Arab`, `pbt_Arab`, `pnb_Arab`, `skr_Arab`, `pan_Guru`, `snd_Arab`, `hno_Arab`, `brh_Arab`; per-language 7B-LLM CER: Urdu 3.5 (237.5 h), Pashto 12.6 (28.7 h), Southern Pashto 8.8 (5.6 h), **Western Punjabi 31.2 (11.9 h)**, **Saraiki 17.0 (1.5 h)**, Punjabi Gurmukhi 4.3, Sindhi 6.6, Hindko 17.1, Brahui 8.2, English 3.0 | `per_language_results_table_7B_llm_asr.csv` in the repo | V (read through a fetch summariser: re-read the file once before quoting) |
| Whisper is unusable zero-shot for Pashto (WER over 100%, script error); full fine-tuning on Common Voice Pashto v24 (113 h) reaches 23.4% (turbo) and 24.9% (small) WER; a base model on v20 reaches 21.2% | arXiv 2604.06507 (+ 2604.04598: at most 0.8% true Pashto script) | V / S |
| SeamlessM4T is best on Urdu (16.3% WER) and Punjabi (22.1%) in a 2026 benchmark; cross-language script contamination affects up to 35% of output characters in lower-resource languages; named entities lag by 5 to 20 points | aghaaliraza.com Interspeech 2026 paper | S |
| Common Voice has Urdu (302 h), Pashto (113 h in v24) and **Saraiki (6.75 h)** under CC0 | Mozilla Data Collective | S |
| AuraFace-v1: Apache-2.0 on its card, ResNet-100 ArcFace-style, 512-D, LFW 99.65% (self-reported), trained on "a commercial dataset ... from various sources"; bundled InsightFace detectors are non-commercial | huggingface.co/fal/AuraFace-v1 | V |
| NIST FRTE reports false-match differentials by region of birth including South Asia; thresholds are algorithm- and version-dependent | pages.nist.gov/frvt | S |
| PaddleOCR-VL (0.9B) lists 109 languages with Urdu, Pashto, Sindhi and Balochi in its Arabic-script group (vendor claim); no independent Urdu figure found | paddleocr.ai | S |
| IndicConformer 600M (22 Indian languages incl. Urdu, Punjabi, Sindhi), MIT | huggingface.co/ai4bharat/indic-conformer-600m-multilingual | S |
| Qwen3-ASR (Apache-2.0, 52 languages): Urdu/Punjabi/Pashto not confirmed in the list | github.com/QwenLM/Qwen3-ASR | S |
| fast-plate-ocr is MIT-licensed; its pretrained weights' terms are not stated | github.com/ankandrew/fast-plate-ocr | V |
| Pakistani plates are changing: Islamabad owner-based plates; Punjab linking registration to the owner's national ID number and proposing new designs; Sindh Ajrak-themed plates with deadlines extended into late 2025 | PakWheels, ProPakistani, Wikipedia | S |
| Pakistan's personal data protection law is still a draft with broad state exemptions; the Interior Ministry directed a unified facial-recognition system in 2025 | GenderIT, Privacy International, ProPakistani | S |
| MEVA is CC BY 4.0, about 328 h public | registry.opendata.aws/mevadata | S |
| IBM AMLworld data under CDLA-Sharing-1.0 (via a mirror of the README) | arXiv 2306.16424 | S |

### 6.2 Open research questions (read these before the dependent work package starts)
1. Omnilingual fine-tuning recipe, CPU latency of the 300M/1B CTC models, quality on 8 kHz audio (WP-M1).
2. Which languages the released Omnilingual **corpus** contains (WP-M1).
3. The Interspeech 2026 paper's per-model tables (is its Punjabi test set Shahmukhi?) (WP-M1).
4. Common Voice: is there a Western Punjabi (Shahmukhi) release? Pashto v25 true size (WP-M1).
5. AuraFace licence file and training-data statement; cost and terms of an InsightFace commercial licence; counsel on Pakistani law (WP-M7, D34).
6. RF-DETR checkpoint licences per size; PDQ licence file; pyannote community-1 terms; Roboflow Trackers licence (WP-M5, M6).
7. NIST FRTE values for the shortlisted algorithm families for the South Asia group (WP-M7).
8. Independent Urdu OCR results for PaddleOCR-VL and Qwen3-VL (WP-M4).
9. DVR container formats common in Pakistan and their ffmpeg support (WP-M6).
10. Official plate formats and dates per province, from the excise departments (WP-M5).
11. Licence of IBM AMLworld on the Kaggle page; Telecom Italia terms on its challenge page.
12. Sources for Urdu/Punjabi/Pashto/Saraiki question sets and native reviewers (D10).


---

# 06. The roadmap: phases, lanes, gates, proofs through the interface

**Reading rules.** Durations are **estimates** for one lead engineer (Claude sessions) with the owner answering decisions within two days; they move with hardware (a GPU shortens model runs by about ten times) and with how fast the owner supplies consented data and reviewers. A phase is left only through its **gate**. Every capability is finished only when it has: a view in the interface, a Playwright test on a generated case, a bank of questions with exact truth, and an owner demonstration (standing rule since 2026-10-13). No long test cycles: the tiers and the change-impact matrix of file 05 apply.

**Lanes (who touches which files; two writers never touch the same files).**
| Lane | Scope | Files it owns |
|---|---|---|
| **E** Engine and structured data | Ask engine, views, typed families, migrations 03x | `api/forensic_records/ask_engine*`, `db/**`, `ingestion/**` (families), `tools/ask_eval`, `tools/showcase` |
| **M** Media | speech, images, OCR, plates, video, faces, media tables and viewers' backends | `ingestion/forensic_records/media*`, `tools/media_data`, `api/.../media*`, `db/**` (033) |
| **C** Case and collaboration backend | conversations, notes/findings, members/authority, cross-case, reports builder | `api/.../conversations*`, `crosscase*`, `identity*`, `db/**` (034+) |
| **U** Interface | everything under `apps/investigation-workspace` | the Codex worktree `NexusAI-ui` |
| **D** Data and evaluation | datasets, ledger, generators, banks | `tools/case_generator`, `tools/licence_ledger`, `docs/data` |
| **P** Platform | GPU, backup/restore, installer, security, observability | compose files, `tools/dev` |

Migration numbers are allocated in this order: **033** media tables and views, **034** conversations/knowledge/members/authority, **035** identifier registry and cross-case, **036** message/extraction families, later ones as needed. Each migration has a rollback script, a verify script, a restore rehearsal and a backup before it is applied live.

---

## 0. Where the day ended (2026-11-03) and the re-measurement of today's engine change

Round 9d (with fdfab37) is live. Re-ask of the questions it was meant to fix, on the live engine, one at a time (results file `C:/tmp/nexusai-g4/vb_recheck_9d.jsonl`):

| Question | Before | After (judged against the SQL oracle) |
|---|---|---|
| VB-PAIR-007 total talk time between two numbers | 57,670 (wrong) | **12,290 (correct)**, 44 s |
| VB-INVE-003 how many tabular files | 4 (wrong) | **3 (correct)**, 29 s |
| VB-INVE-004 how many text files | 14 (wrong reading) | **4 (correct)**, 70 s |
| VB-MESS-009 who was sent the most texts | abstained | **correct** (923000001038 with 16, then 1031, 1021, 1003), 111 s |
| VB-PAIR-001 / 003 / 012 (regression sample of the `between` family) | correct | still correct (150; 14,673; last contact 2026-04-17 16:58:18) |
| VB-BANK-050 senders to an account | abstained | still abstains (round 9e) |
| VB-PLAT-051 most-seen plate | wrong (tie hidden) | still wrong: SYNT4024 (50), SYNT2905 also 50 (round 9e) |

**Reading:** 7 of the 9 re-asked questions are correct (4 of them were wrong or abstained before); the other 2 are the known round-9e items. 475 questions were not re-run (the tier policy: a full bank only at a milestone, in the background), so "no regression elsewhere" is **not yet measured**; the bank count would be at least 477 of 484 if nothing regressed. VB-MESS-019 and VB-BANK-004 were not re-asked.

## Phase 0: close-out and set-up (days 0 to 7)

| # | Task | Lane | Done when |
|---|---|---|---|
| 0.1 | **Engine round 9e**: (a) "sent money **to** the account" must not be read as outgoing (direction word vs object); (b) "which number was sent the most texts" (event-kind obligation on a ranking); (c) **ties**: a ranking whose first two values are equal lists both ("two plates tie at 50"); (d) the uncapped model call (VB-BANK-004 timeouts); (e) strip the **byte-order mark** from CSV headings at ingest and in `file-rows`; (f) keep the header list for `.xls`/`.txt` files at ingest so the fallback shows the file's column order; (g) decide VB-INVE-004 with the owner. Specs and mutation checks first; then re-ask only the 12 affected questions; deploy by the recipe (**apply, restart the workspace immediately, then health**) | E | the 12 questions answered correctly or abstained honestly; unit/DB specs green; live smoke 5/5; bank unchanged elsewhere |
| 0.2 | Refresh `NEXUSAI_CONTINUATION.md` (backend part) and the memory index so a new chat starts true; archive the October roadmaps' status in the supersession map | E | file under 300 lines, dated 2026-11-03 |
| 0.3 | Ledger rows and fetch (sizes stated first) of Common Voice Urdu/Pashto/Saraiki, FLEURS ur/ps/pa/sd, Omnilingual corpus slices; build the **speech gold sets** (development/sealed by hash, telephone simulation) | D | hash manifest verified; counts reported |
| 0.4 | Evaluation image with `omnilingual-asr` (CPU), measure CTC 300M/1B and LLM 300M on the development partitions for ur, ps, pbt, skr, pnb: CER and WER, real-time factor, memory | M | table in `docs/data/media-activation-results-*.md` |
| 0.5 | Draft migrations 033 and 034 (tables, views, rollbacks) and the BR-2/BR-3/BR-5 request rows; restore-rehearsal plan | M, C | reviewed by the owner in prose |
| 0.6 | Owner reviews U2 on `:4182`; owner answers **D34 to D40** (file 07); decide the Ask/Evidence order (D37); copy the env file into `NexusAI-ui`, `npm ci` | U | decisions recorded |
| 0.7 | GPU decision (D36): interim EliteDesk 2080, purchase, or cloud for public-data training only | P | decision recorded |

**Gate 0:** the verified bank is not worse (473/484 or better by the re-asks); decisions D34 to D40 answered; speech gold sets verified; the owner has seen `:4182`.

## Phase 1: foundations in parallel (weeks 1 to 5)

| Lane | Work | Output the owner sees |
|---|---|---|
| **M** | **WP-M0**: artifact contract v2, queue classes, media proxies/thumbnails/waveforms, clock offsets, review events, **migration 033 + analyst media views + engine descriptions + three verified examples + media bank v1 (30 questions)**; **WP-M1 speech lab**: baselines for all candidates, public-data fine-tunes on rented/free GPU (Pashto, Urdu, Saraiki, then Punjabi), telephone simulation, decision rule, routing table; **WP-M3a**: EXIF policy, PDQ/pHash, content class | A **Media tab** listing every media file with thumbnails and task states; a results table per language; "how many recordings are in Pashto" answerable |
| **C** | **BR-2 conversations** (migration 034 part 1, API, engine integration: follow-up context, snapshots, re-run/diff, streaming phases); **BR-3** notes/findings/tags/saved views server-side; minimal `case_member` and `authority_record` | A real history rail (replacing browser-only turns); notes that survive a browser change |
| **E** | round 9e deployed; **R7 conversation role** measured on the conversation bank (greetings, help, clarification, scope); GPU bake-off decision rule written (WI-0); subscribers gate: fresh case request | greeting and help answered by the model, measured |
| **U** | **U4 Ask + History**: research note, two layouts as real routes, owner chooses, build; start **U6** (evidence passport, port of FileRowsViewer/SourceRowCard/passages) | the Ask page on Orion: full-width answer with table, chart, map, evidence drawer, history |
| **D** | Meridian and Control with media assets and cross-case plants/decoys; chat-screenshot and watermarked ID generators; synthetic Pakistani plate generator spec | reference cases extended |

**Gate 1:** speech gate per language (a language that fails keeps `LOW_RESOURCE_LANGUAGE`, stated); media bank v1 at 0 confident-wrong on synthetic assets; conversations live with re-run/diff; U4 layout chosen.

## Phase 2: media in the product (weeks 5 to 11)

| Lane | Work | Output |
|---|---|---|
| **M** | **WP-M2** audio product (VAD, language ID, diarisation, spoken numbers, translation, transcript viewer, review, call-record linking); **WP-M3** images (chat-screenshot parser, ID/MRZ/cheque parsers with validators, SigLIP search, tamper indicators, viewer); **WP-M4** Urdu/Pashto OCR bake-off and Docling layer; **WP-M5** plates on images (synthetic generator, detector, recogniser, registry, review, linkage to sightings); **WP-M6** video v1 (shots, motion index, overlay OCR, tracking, ANPR v1, audio hook, player); **WP-M7** faces L0 to L1 (lab, consented evaluation set, clustering, governance, audit) | open an audio file, an image, a video: each complete with its readings; plate reads on images linked to sightings; person clusters in a case where faces are enabled |
| **C** | **mention index generalised**; **identifier registry v1 + overlap detection** (migration 035); case profile and new-case backend; report builder backend | "also appears in" chip on numbers and plates |
| **E** | **R6 media answerer**; **message-export family** (WhatsApp/SMS/Telegram) end to end; tower dumps; PDF/image bank statements through OCR | questions over messages and scanned statements |
| **U** | **U4 accepted**; **U6 viewers** (structured, text, documents, image, audio, video v1); **Media gallery**; U5 start | open every Orion file and the media samples; unaided add-data/open-citation test |
| **D** | media bank to 100+ questions; red team 100; language banks started with reviewers | numbers per language |

**Gate 2:** audio gates per language; OCR Urdu gate; plate image gate (Boundary A) or a stated shortfall; video v1 event recall; faces L1 evaluation reported (L2 stays off); media bank at 0 confident-wrong; red team 0 successes; the unaided test passed.

## Phase 3: intelligence across families and cases (weeks 11 to 17)

| Lane | Work | Output |
|---|---|---|
| **M** | **WP-M8** faces L2 (search/compare/watchlists) **only after the legal gate and the evaluation gate**; **WP-M9** cross-modal links (recording to call, OCR/spoken numbers to subscribers, plate to sighting, video moment to cell co-presence); video ANPR Boundary B; **WP-M10** captions and semantic video search if a GPU exists; WP-M11 optional | one timeline per person or number across calls, bank, plates, video, recordings, documents |
| **C** | **cross-case v1** (notices, access requests, Ask across cases, link-chart overlay, case relations); **entity resolution** (reversible `same_as`, transliteration-aware names); saved views; sharing | Control case: zero false merges; Orion/Meridian overlaps found |
| **E** | phone-extraction family; email; location history; recursive reach answers; GPU model swap under the WI-0 rule when hardware arrives | |
| **U** | **U5** Dashboard/Cases/New case/Overview; **U3** workbench restyle; **U7** entity drawer, findings, saved views; Money and Vehicles pages; face/compare screens | the dossier and dashboard on real series |
| **D** | cross-case bank; consented face set grown; more telephone-band tests | |

**Gate 3:** cross-case oracle met with zero false merges; face L2 negative control passed and counsel's review recorded (otherwise L2 stays off); cross-modal links reviewed on Orion/Meridian; the dossier shows no fabricated number.

## Phase 4: enterprise and release (weeks 17 to 24)

Identity (users, roles, MFA, per-user audit), audit viewer, report builder and sign-off, backup/restore tested, load test at 10 times the Orion volume, performance budgets, hostile-input and red-team re-runs, accessibility audit (English and Urdu, right-to-left), the **usability test with real analysts**, deletion of the alias layer and the classic Connections route, **cut-over decision** `:4182` to `:4181`, release checklist.

**Gate 4 (release):** all banks at their gates; usability results recorded; restore tested; owner acceptance by looking.

## What to protect first if time is short
1. Phase 0 and Phase 1 (they remove the structural limits).
2. Audio in five languages, OCR, image viewers, evidence viewers, Ask + History (the daily-use value).
3. Plates and video v1.
4. Faces L1.
5. Cross-case v1.
6. Faces L2, VLM captions, speaker clustering last.

## The first 14 days, in order (for the new chat)
Day 1: read `00_START_HERE.md`, `NEXUSAI_CONTINUATION.md`, AGENTS.md, this pack; confirm the live state (`docker ps`, `GET /healthz`, image ids). Day 1 to 2: owner decisions D34 to D40 (send them as one list); task 0.1 specs. Day 2 to 4: 0.1 deploy and re-ask; 0.3 data fetch; 0.5 drafts. Day 3 to 7: 0.4 baseline on the laptop (one container at a time), 0.6 owner review of U2, 0.7 GPU decision. Day 7: Gate 0 report and the Phase 1 task split into sessions (engine round, speech lab, conversations backend, U4), each with its file ownership.

## Sessions that can run in parallel (separate branches or worktrees)
1. *Engine round 9e + deploy* (lane E). 2. *Speech lab* (lane M; no repo code until the decision; data and results docs). 3. *Conversations backend* (lane C; migration 034). 4. *U4 Ask* (lane U in `NexusAI-ui`). Each brief names the files it may touch; the lane-E and lane-C sessions must not both edit `api/forensic_records/main.go` at the same time (route registration: do it in a final merge commit).

## Reporting rhythm
Every gate ends with a stage note (what was built, what was measured, what failed, what the owner can look at, the commit ids, the rollback tags), a refreshed `NEXUSAI_CONTINUATION.md`, a memory update and a short message to the owner in plain words with the link to the page to open.


---

# 07. Decisions only the owner can make, risks stated plainly, and what this pack supersedes

## 1. Decisions waiting for the owner (answer as one list; recommendations first)

| ID | Decision | Options | Recommendation | Blocks |
|---|---|---|---|---|
| **D34** | **Faces** (replaces D5). How far to build recognition | (a) detection only (L0); (b) **L1 clustering now, L2 search/compare after the evaluation and legal gates**; (c) L1 and L2 at once | **(b)**, with the controls of `02_media_programme.md` section 7: per-case authority, role permission, every search logged with a reason, candidate-only labelling, no embedding export, kill switch. Embedding route: **AuraFace-v1 (Apache-2.0 card, training data provenance unclear)** after reading its licence file and taking counsel, **or** buy an InsightFace commercial licence | WP-M7, M8 |
| **D35** | **Voice** | (a) none; (b) **diarisation and within-case speaker clustering only**; (c) gallery search too | **(b)** | WP-M11 |
| **D36** | **Hardware** | (a) CPU laptop only; (b) interim: the HP EliteDesk RTX 2080 (8 GB) if it can be made available; (c) buy a 16 GB card (about $430 to 550) or a used 24 GB card (about $700 to 1,050) [prices from 2026-09-29, unverified]; (d) cloud only for **public-data** training | **(d) now (no decision cost), (b) as soon as possible, (c) within Phase 2.** A GPU moves the language model to 8B to 14B, makes video captions and face search usable, and cuts every model run by about ten times | M6/M10/M8, engine speed, banks |
| **D37** | **Interface**: the order and the executor | proposed order U2 close, U4 Ask + History, U6 Evidence + viewers, U5, U3, U7, U9, U8, U10; Claude executes in `NexusAI-ui` on `:4182`; `:4181` keeps the old design until a cut-over decision | **yes**; the cut-over (section 9 of file 04) is a separate later decision | all of lane U |
| **D38** | **Data**: standing permission to fetch public licence-clean datasets | (a) yes, with a ledger row per dataset, sizes stated above 2 GB, NC data evaluation-only; (b) ask per dataset | **(a)** | lane D, WP-M1 |
| **D39** | **Languages and people**: priority and reviewers | speech and question languages in the order Urdu and English, Pashto, Saraiki, Pakistani Punjabi; name **native reviewers** (D10) and a **capture campaign** (D26) for telephone-band audio, plates and consented faces | **yes**; the owner or the team lead names people; without them every non-Urdu number stays a public-data number | PL, WP-M1, M5, M7 |
| **D40** | **Conversation role** (greetings, help, clarification) | switch on after it passes the conversation bank | **yes, after measurement** | R7 |
| **D41** | **Cross-case visibility** | a user who cannot open the other case sees (a) that an overlap exists plus the owner's name and an access request; (b) existence only; (c) nothing until requested | **(a)** | WP cross-case |
| **D42** | **Plate masking (D28)** | keep search-only text; or a case-scoped keyed alias with the key required | decide before WP-M5 ships; plates are more sensitive since registration follows the owner | WP-M5 |
| **D43** | **Model swap** (AGENTS.md: only under WI-0's written decision rule) | authorise a bake-off of 8B to 14B models on the verified bank and a fresh case when a GPU exists | **yes**, rule written first | engine speed |

**Still open from before (unchanged):** O-6 Urdu face for body text (needs a native reader), O-8 offline map tile source licence (hard block on screenshots leaving the machine), O-11/O-12 usability testers, D10 reviewers' names, D26 who captures and labels, D27 scope of captures/databases/archives.

**Decided and recorded:** O-1 yes, O-2 (b) own worktree, O-3 = B Ink and signal, O-4, O-5, O-7, O-9, O-10 (2026-10-28); D23 to D33; the approvals of 2026-11-03: merge fdfab37, round 9d deploy (done), round 9e to follow.

## 2. Risks

| # | Risk | Likelihood / impact | What reduces it |
|---|---|---|---|
| 1 | **Face recognition legal and ethical exposure** (draft law, broad state exemptions, regional bias, misuse) | high / high | D34 controls; consented South Asian evaluation set with per-group error rates; negative control; every search logged; candidate-only language; counsel review before L2; kill switch |
| 2 | **Licences**: AGPL YOLO stack, non-commercial weights, unclear training data (AuraFace, SFace), CC BY-NC datasets | high / high | ledger row per component; evaluation-only labelling; AGPL never ships; own-trained recognisers on licensed data; counsel |
| 3 | **Language quality**: Pakistani Punjabi and Saraiki are low-resource; read-speech numbers do not transfer to telephone audio | high / high | publish per-language numbers on our gold sets including telephone simulation; `LOW_RESOURCE_LANGUAGE` honesty; second-opinion backends; consented real samples; never quote an unmeasured figure to a court |
| 4 | **No GPU**: video, faces at scale, captions and bigger SQL models are slow or impossible | certain / high | D36; overnight batch; public-data cloud training; staged capabilities; throughput stated in every gate |
| 5 | **Data**: no public Pakistani plates/operator layouts/telephone Punjabi; consent and labelling effort | high / high | synthetic generators, codec simulation, T2-V scenes, capture kit, named annotators and reviewers (D26, D39) |
| 6 | **Scope**: the owner's list is a multi-year product; the plan must not promise it all at once | high / medium | phases with gates; the cut line (file 06); each capability finished (view + test + bank + demo) before the next starts |
| 7 | **Interface divergence** again (three directions existed) | medium / high | one directive; one executor; per-stage loop; `:4181` untouched until cut-over |
| 8 | **Model errors presented as fact** (hallucination, prompt injection through OCR text, transcripts) | medium / high | number lock, evidence rows, abstention, labelled observations, quoted-data prompts, read-only tools, red-team gate |
| 9 | **Engine regressions** from many small fixes | medium / medium | impact index; verified bank at milestones; mutation checks on every new obligation; deploy by recipe with rollback tags |
| 10 | **Long test cycles** return | medium / medium | change-impact matrix; result-key refusal of unchanged reruns; background milestone runs |
| 11 | **Infrastructure fragility**: Docker VM memory (7.6 GB), one eval container at a time, app-started Docker dying, port relay wedging after recreate | high / medium | the recorded rules (start Docker detached; `--cpus 8 -m 5g`; restart the workspace immediately after an API recreate) |
| 12 | **Real-data leakage** into screenshots, prompts, repository, cloud tools | low / critical | the vault policy; synthetic-only design tools; O-8 hard block; reviews of every export |
| 13 | **Evaluation leakage** (fine-tune data overlapping test sets, e.g. FLEURS and Common Voice) | medium / medium | our own hash partitions; sealed sets touched once; dataset overlap checks recorded in the ledger |
| 14 | **Document sprawl** (about 80 planning documents already) | medium / medium | this pack plus the master plan are the plan of record; the supersession list below; `NEXUSAI_CONTINUATION.md` rewritten, never appended |
| 15 | **Single point of knowledge** (one owner, assistants with no memory between chats) | medium / high | the pack, the continuation file, the memory index, stage notes with commit ids and rollback tags |

## 3. What is explicitly not promised
Identification of a person from a face or a voice; any model output shown as a fact; accuracy for telephone-band Punjabi/Pashto/Saraiki before it is measured on consented samples; court-grade ANPR before the sealed-set gate; real-time CCTV alerting; any step that sends evidence to a third party; any dependency without a ledger row; any merge of the new interface into `lane-arms` without the owner's separate word.

## 4. What this pack supersedes (and what it does not)
* **Supersedes:** section 8 of the master plan on faces (D5 becomes D34) and on speech sequencing (the bake-off no longer waits for a GPU: the CPU baseline and public-data fine-tunes start now); the interface order of section 7.4 (D37); the "next 14 days" of section 13.1 (2026-10-21 to 2026-11-04, now past).
* **Does not change:** the binding rules in AGENTS.md and `NEXUSAI_CONTINUATION.md` sections 2, 4, 5 and 6; the query-answering decision of 2026-10-05; the UX contract; the real-data policy; the licence ledger process; the test-tier policy.
* **Plan of record from 2026-11-03:** this pack for sequencing, the master plan for the ledger of older items, the directive for the interface, the continuation file for current state.
