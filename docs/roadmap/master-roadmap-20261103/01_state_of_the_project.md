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
