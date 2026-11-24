# NexusAI: what has been built and what is live (complete status, 2026-11-24)

**What this is.** One file that says what exists, what is live, what was measured and what is missing, as of 2026-11-24 about 14:45 PKT. It replaces reading the memory index, the handoff log and a dozen result notes. Numbers were checked against the running containers, the repository and the sheets on the day; where a number comes from an older run, the run date is given. The companion files are the roadmap (`docs/roadmap/master-roadmap-20261124/`) and the question catalogue (`docs/qa/question-catalogue-20261124.md` and `.csv`).

**One line.** NexusAI is a local, case-based forensic analysis workspace. An analyst uploads evidence (call and internet records, bank and wallet statements, subscriber data, documents, photographs, recordings, video), the system turns it into typed records and derived readings with their source, and the analyst asks questions in plain words (English, Roman Urdu, Urdu) that a small local language model turns into checked queries, with every answer citing its rows or files. It runs on one laptop (15.7 GB, no GPU).

---

## 1. At a glance

| Layer | State |
|---|---|
| **Typed records** (calls, internet sessions, apps, cells, handsets, registrations, plate sightings, bank and wallet, subscribers) | Live. Layouts are data; 13 provider layouts are drafts until a real file confirms them |
| **Ask engine v2** (the model writes one read-only SELECT over the analyst views; code validates it; three outcomes: answered, abstained, declined) | Live. Verified bank on the reference case: **562 of 565 correct, 0 wrong, 3 abstained** |
| **Text search across all evidence** (documents, text read from pictures, plates, transcripts; tolerant of OCR errors, digit scripts and phone forms) | Live since 2026-11-24. Fresh banks 70 to 86% fully answered; gate of 90% not met |
| **Case layer** (threads, notes, findings, members, hash-chained audit, export) | Live since 2026-11-10. Cross-case is only a fan-out of one question to 2 to 10 cases |
| **Link analysis** (connections, co-location, successors, movement, places) | Live as views and in the old interface |
| **Media** | OCR live (Latin and digits good; Urdu Nastaliq poor); speech live (Urdu, English, Pashto lead search; automatic language detection); plates in images and video live; **image search live** (copies, parts, words, draw-a-box, duplicate groups); **watch lists (known sets of picture hashes) live since today**; faces: bake-off only, detection off; video overview not built |
| **Interface** | :4181 old design with all functions; :4182 new design (Ask with conversations, Cases, Dashboard, Evidence gallery and viewers, image search). Cut-over not decided |
| **Governance** | Row-level security, read-only analyst role, hash-chained audit, a switch per capability, rollback tags, owner's word for every deploy |

---

## 2. The live system (checked 2026-11-24)

| Part | State |
|---|---|
| Language model | Container `nexusai-api-1-prev-gbnf` (LocalAI, port 8080): Qwen3-4B-Instruct Q4_K_M on CPU (about 22 tokens a second reading, 5.5 writing), Qwen3-Embedding-0.6B, speech models (`faster-whisper-turbo`, a small Urdu model, Omnilingual routes). Never stop it; `nexusai-api-1` is a detached duplicate: never start it |
| API | Container `nexusai-forensic-records-api-1`, image **`known-sets-20261124` = `9e3fca1e37a0`** (live since 14:40 PKT today, owner's word). Rollback chain: `before-known-sets-20261124` = `3b20b23d9043` (= `text-search-all-20261124`, live 11:17 PKT) ← `before-text-search-all-20261124` = `517f31ae9a0f` ← `before-image-search-20261123k` = `0e72aad96d6c` ← `audio-files-20261117f` = `22d23221aba5` ← `case-layer-20261110` = `85eeeaf98c26` ← `before-case-layer-20261110` = `b1514079de69`. The container carries **79 `FORENSIC_*` variables: 40 switches on, 13 off** |
| Worker | Image `image-search-20261123f` = `40f9bbfbd3cd`: an overlay chain on the 2026-08-23 base, with files older than the repository's in places (23 of 32 `/app` files equal HEAD). **Not reproducible from HEAD** (`docs/roadmap/worker-image-reproduction-20261124.md`) |
| Image-search service | Container `nexusai-forensic-image-search-1`, image `image-search-svc-20261123f` = `3eef9ecbb83f` |
| Database | PostgreSQL 16 with TimescaleDB, pgvector, cube, earthdistance. Migrations present: **001 to 021, 023 to 032, 034 to 039** (022 never applied; 033 never used). 46 views are defined by the migrations (the Ask engine reads the analyst views among them). **039 (image known sets) applied today** after a backup (533,548,942 bytes). Backups in `.phase2-backups/` and `C:/tmp/nexusai-backups/` |
| Collections | 25. The reference case is `orion-lahore-reference-20261030` (165 evidence items, 60 of them image-search test photographs named `imgsearch-*`; moving them needs the owner's word) |
| Interfaces | :4181 old design (lane tree); :4182 new design (branch `ui/v2-redesign`, tree `NexusAI-ui`). Restart both after every API recreate |
| Backup of the images | **The live API and worker images exist only as local Docker tags** (never exported). A `docker save` to a disk the owner chooses is the backup, and is waiting for the owner |

Switches on (examples): the Ask v2 pair, the governed SQL pair, `TEXT_INTENT`, `TEXT_SEARCH_ALL`, `IMAGE_SEARCH_ASK`, `IMAGE_REGION_SEARCH`, `IMAGE_KNOWN_SETS`, `PLATE_SEARCH_ONLY`, `MEDIA_FAMILY_ROUTING`, `LADDER_ROUTING`, `VERIFIED_ONLY`, `PLAN_CACHE`, `DERIVED_ARTIFACT_EXECUTION`. Off: `ASK_V2_AGREE`, `CONVERSATION_FRONT_DOOR`, `CONVERSATION_ROUTER`, `PII_MASKED_PROJECTION`, `IR_CROSSCHECK`, `IR_SHADOW`, `IDENTIFIER_BINDING` and others that were measured and rejected or are waiting.

---

## 3. What is built, by area

### 3.1 Ingestion and typed families
- **Layout registry** (`ingestion/forensic_records/layout_registry/layouts`): layouts are data. 13 provider layouts for call and internet records are **drafts**; promotion needs the vault and a real-file confirmation. Ingest v2 (2026-10-13) fixed `.txt` and `.xls` intake and is live on the worker and API.
- **Typed families** (migrations 012 to 032): calls (CDR), internet (IPDR), apps, cells and observed positions, handsets (IMEI), registrations (IMSI, ICCID, status), plate sightings (ANPR), subscribers and devices, bank and wallet transactions, document mentions. The typed-family switch is on; the effective-record views remove superseded versions (for example 8,642 call records in version 1, 6,239 effective on the demo case).
- **Time basis**: every typed row is `declared`, `explicit_offset` or `confirmed_by_analyst`; none is assumed. One case clock, `Asia/Karachi`; instants are stored in UTC.
- **Derived artifacts are never merged with ingested records**: a model's plate read is not a camera's sighting; the source-truth state travels with every derived citation.

### 3.2 The Ask engine
- **Route order** for a question (`/query/hybrid`, port 8091): the image step (only a sentence naming a photograph) → the text-intent step (a model decides whether the message asks where a value or word is written) → any-source text search, or plate search → Ask v2 (frame, obligations, SQL by the model, parse-tree allowlist, read-only execution with a cost cap, row cap and 15 s limit, derivation, answer with citations) → the governed lane → the older ladder (frozen).
- **No templates**: questions are understood at run time; code validates what the model wrote. The older keyword ladder is frozen and still handles what the engine declines.
- **Measured** (verified bank, Orion Lahore, answers computed from the stored rows and never through the engine): **565 questions, 562 correct, 0 wrong, 3 abstained** (VB-MESS-019, VB-VIDE-031, VB-VIDE-037); every verdict equals the sheet of 2026-11-13; a 64-question sample re-asked on the live code on 2026-11-23: 62 correct. Median about 30 s per question on the CPU laptop. Gates and rejected experiments are in `NEXUSAI_CONTINUATION.md` section 2.
- **Text search** (`ask_text_search_all.go`, `ask_text_lookup.go`, `pkg/forensictext`): exact, normalised and approximate tiers; Eastern Arabic and Persian digits, Arabic letter variants, phone forms 0, 92 and 0092; code and number runs; parts fallback; transcript windows; an empty records answer gets the files that hold its number or name; names are looked up by the model only after an empty answer. Fresh banks: committed code 69% fully answered (bank-C), the deployed build 70 to 86% (A, B, C). **Gates (90% full recall, 0.95 mean recall) not met**: the 4B intent model sends about a third of "where is X written" questions to the table engines. Next: Phase 2 (attach, never replace).
- **Analyst views and endpoints**: `GET /analysis/connections`, `/places`, `/engine-health`, `/typing-report`; `GET/PATCH /cases`; the case layer (`/conversations`, findings, members, exhibits); `/images/search` (words, copies, reference, region, crop) and the known-set routes.

### 3.3 Case layer (migration 034, `internal/caselayer`)
Threads and turns with re-run, diff, fork, pin and feedback; notes; findings; members and authority; a hash-chained audit with export. The member and authority tables are mostly empty and there is one shared actor (no identity, no multi-factor sign-in): the enterprise layer is open.

### 3.4 Link analysis (migrations 024 to 026 and the views)
Connections (who talks to whom, with a link-chart in the old interface), co-location and meetings, successor candidates, movement and rhythm, places; runtime parameters (minutes, days, thresholds) are read from the question into settings the views read. A map with offline tiles (`npm run dev:tiles`) exists in the old interface. Cell-tower coordinates are not part of the product yet.

### 3.5 Media
| Family | Built and live | Measured | Missing |
|---|---|---|---|
| **OCR** (EasyOCR, routed) | Latin and digits; table-row order; re-read fix | Latin and digits good; **Urdu Nastaliq print: about 70% character error, not usable** | Urdu route; chat-screenshot, ID and cheque parsers |
| **Speech** (faster-whisper and Omnilingual routes, automatic language detection) | Urdu, English read; Pashto as a search lead; audio length, language and word-search questions | Orion pack of 10 public clips: language 10 of 10; word error en 0%, ur 26%, ps 36.5%; fresh wording **27 of 34** (two gates not met) | Punjabi and Saraiki (not measured; kit ready), diarisation, spoken numbers, call linking |
| **Plates in images** | Plate engine, plate search, photos in the reference case | Sealed gate 83% against 90%: **not met** | Real casework calibration |
| **Plates in video** (V4) | Processor and questions over real clips | Real clips 9 of 9, 7 of 7, 13 of 15 exact | Video overview (shots, motion, clock text, clip export), tracking |
| **Image search** | Copies (exact, re-saved, resized), parts of pictures, window vectors, words in any language, duplicate groups, draw-a-box, more-like-this, interface in :4182 | Gates N1 to N3 **not met as registered** (recall 90.3%); region search 215 of 216; the owner is testing | Scale (indexed window query), EXIF filters, categories |
| **Watch lists (known sets of picture hashes)** | Migration 039, `image_known_*` tables, routes, Evidence badge and panel; **live 2026-11-24 14:40 PKT** | Gates K1 to K5 **met**: 12 of 12 originals and re-saved copies flagged, 0 of 79 false flags, hash-only storage, isolation and retirement tested | Hash sets only (a crop beyond 56 bits is not flagged by design) |
| **Faces** | Nothing live; detection off in the worker | Bake-off done: SFace and YuNet stay; default threshold far too permissive; no consented set | **No governance code** (per-case switch, permission, search log, kill switch); embeddings still stored inside observation payloads |

### 3.6 Interface
- **:4181 (old design, kept on purpose)**: the full set of functions, the workbench, the connections link chart, places map, the Ask view, the evidence list and the data panel.
- **:4182 (new design, branch `ui/v2-redesign`, tip `a10ce43`, 95 test files)**: design language and shell; **Ask** with server-side conversations; **Cases**; **Dashboard** with a data-quality strip; **Evidence** gallery with openable cards, real waveforms, quick look and media viewers (audio transcript, plates beside photo and video); **image search** (attach a picture, results as stacks, draw-a-box, known-set badge and panel). The other pages (connections, places, timeline, money, vehicles, entities, findings, reports, activity, settings) are not redone yet. Design plan of record: `docs/ux/v2/DESIGN_REFERENCE_20261105.md` in the interface tree (`NexusAI-ui`, not in the lane tree).

### 3.7 Reference cases, banks and measurement
- **Orion Lahore** (`C:/NexusAI-Showcase`, synthetic): every family, statements in many formats, five real video clips and ten public recordings. Verified bank 565 questions (`docs/qa/verified-bank-20261124*`). Meridian and Control (overlaps, decoys for cross-case) are specified and **not generated**.
- **Case generator** (`tools/case_generator`): spec v0.6, planted truth, a 20-case set with a hash register; any change to the generator invalidates the register.
- **Banks and judges**: `tools/showcase` (verified bank, search banks A, B, C, carry, judge), `evaluation/text_intent` (intent banks), `tools/ask_eval` (older banks), `tools/media_data` (faces, image search, speech data), `tools/real_data` (vault tools and scanner).
- **Discipline**: every capability arrives behind its own switch with a pre-registered gate, a rollback and the owner's word; a bank whose misses were read cannot certify the fix; abstention is a success, a confident wrong answer never is.
- **Question catalogue**: `docs/qa/question-catalogue-20261124.md` and `.csv` (920 rows, generated by `tools/showcase/build_question_catalogue.py`).

---

## 4. Timeline of what was done (from the notes)

| When | What |
|---|---|
| 2026-09 to 10-05 | Query-answering decision; Ask v2 design and first engine |
| 2026-10-12/13 | Maturity roadmap; 13 draft layouts; real-data policy and tools; migrations 012 to 015 applied; ingest v2 live |
| 2026-10-14 to 16 | Typed family switch; Ask engine phase A built and calibrated; connections, places and Ask views |
| 2026-10-20/21 | Older collections typed live (thick views, migration 020); master plan; one canonical tree; case record (021); OCR role live; public media datasets |
| 2026-10-22 to 28 | Link analysis stage 3 (024 to 026); ANPR typed family (027); wording amendments; reference-case programme (Orion built); subscriber, device and transactions (028 to 030); UI directive |
| 2026-10-29 to 11-03 | Engine gate on new families; live window: migrations 027 to 032 applied, engine rounds 8 to 9d live; roadmap pack and decisions D34 to D43 |
| 2026-11-10/11 | **Case layer live (034)**; text-intent step live; plates M5 and plate-in-images search live; video plates V4 live |
| 2026-11-13 | Speech lab routes and automatic language detection live; verified bank re-run (530 of 533); media viewers; evidence gallery |
| 2026-11-16/17 | Audio engine rounds 10 and 10b; EasyOCR role live for development |
| 2026-11-19 to 24 | Exhaustive text search built, measured and **deployed 2026-11-24**; image search live (copies, regions, words, duplicates, draw-a-box); face and retrieval bake-offs |
| 2026-11-24 | Reconciled plan; A0 (both sessions' work committed and pushed, API proven equal to HEAD); A1 (state file, verified bank rebuilt 562/565); **watch lists live** (migration 039); this status file, the roadmap and the question catalogue |

---

## 5. What is not built (the honest gap list)

Cross-case (no registry, no overlaps, no cross-case findings, no Ask across cases) · message families (chat exports, WhatsApp-style files, chat screenshots) · email · phone-extraction readers · cell-tower positions · faces (governance and everything after) · Punjabi and Saraiki speech · speech diarisation, spoken numbers and call linking · video overview and tracking · Urdu document OCR · EXIF filters and categories for images · files attached to records answers (Phase 2) · identity, multi-factor sign-in and retention policy · a published, reproducible worker image · the remaining pages of the new interface. Each is in the roadmap with a gate; the questions that depend on them are in section 7 of the catalogue, so a test of them is expected to decline.

## 6. Git state and files left on purpose

- Lane tree `lane-arms` (worktree `NexusAI-localwork`): on `origin claude/gifted-pasteur-4kpx0r` up to the watch-list commit `acb9c9a`; **the commits of this pack (`6ce3f99` and later) are local** until the owner says push. About 480 commits in the repository.
- Interface tree `ui/v2-redesign` (worktree `NexusAI-ui`): tip `a10ce43`, pushed.
- **Uncommitted by design**: `tests/e2e-ui/Dockerfile`, `tests/fixtures/backend-image/Dockerfile`, `tools/visual_spike/package-lock.json` (project rule), the interface's `runtime-config.js` (temporary local case list), old files under `reports/` and `evaluation/question_factory/*.txt`.
- **Watch lists are committed** (`e43aa3f`: migration 039, its rehearsal, pre-registration and results; `acb9c9a`: the Go files and routes `GET/POST /images/known-sets`, `POST /images/known-sets/{id}/hashes`, `POST /images/known-sets/{id}/retire`, `GET /images/known`). The live API image `known-sets-20261124` was built from the working tree before those commits; a comparison of a build from HEAD with the live binary has not been repeated for it.
- Commits: explicit paths only; message ends with `Assisted-by` and `Claude-Session` lines, never `Co-Authored-By` of an AI; a Go commit needs `--no-verify` (the Windows lint hook fails on `backend/go/*`, unrelated) with the owner's yes per commit.

## 7. Known defects and limits

The live worker image is not reproducible from HEAD · the images exist only as local tags · 13 layouts are drafts · the intent model is the limit of text search (a bigger model needs a GPU) · speech and plate sealed gates not met · image-search gates N1 to N3 not met as registered · the Lahore photo counts include the 60 test photographs · Postgres shared memory is the 64 MB default · the connection class takes 39 s on the laptop · the Docker VM is 7.6 GB, so one heavy job at a time (model run, window indexing, database restore, image build) · two sessions share the laptop and one working tree.

## 8. Where things are

| Need | Path |
|---|---|
| Current state, rules, hazards (backend) | `NEXUSAI_CONTINUATION.md` |
| What to do next | `docs/roadmap/master-roadmap-20261124/` (this pack), `docs/roadmap/reconciled-plan-20261124.md` |
| Earlier plans | `docs/roadmap/master-roadmap-20261103/`, `master-plan-20261021.md`, `media-modules-reconciled-plan-20261120.md`, `reference-cases-programme-20261028.md` |
| Measurements and pre-registrations | `docs/data/*-preregistration-*.md`, `*-results-*.md` |
| Research notes | `docs/research/` (faces and image search, Punjabi and Saraiki speech, video, OCR) |
| Questions, banks, guides | `docs/qa/` (catalogue, verified bank, image-search guide and real-world test plan) |
| Interface plan | `docs/ux/MASTER_UI_UX_DIRECTIVE.md`, `docs/ux/NEXUSAI_PRODUCT_UX.md` (lane tree); `docs/ux/v2/DESIGN_REFERENCE_20261105.md` and the stage notes S1 to S4 (interface tree `NexusAI-ui`) |
| Architecture | `docs/architecture/` (conversations and case layer, query answering decision) |
| Deploy scripts | `tools/dev/live-window/` (`3-recreate-api.ps1`, `10-` and `11-` image search, `rehearse-evidence.sh`, `run-staging-api.ps1`) |
