# Roadmap, flow and architecture (2026-11-24)

**Purpose.** Cover every data type with a working version first (breadth), then deepen each one (depth). This file reconciles `docs/roadmap/reconciled-plan-20261124.md`, `docs/roadmap/master-roadmap-20261103/`, `docs/roadmap/media-modules-reconciled-plan-20261120.md`, the media research notes in `docs/research/` and the online check in `02_research_sources.md`. Where they differ, **this file wins on order and scope; the reconciled plan wins on its measured facts**. Nothing here is built by this note; every deploy, migration, push and model download still needs the owner's word.

**Marks.** **[P]** read at the primary source on 2026-11-24 (licence page or repository page). **[R]** from our own repository notes and measurements. **[S]** secondary (search result or general knowledge, not read at the source in this pass: verify before adopting). **[K]** a published standard or protocol, not re-read.

---

## 1. Rules that bind every phase

These are the owner's standing directives; each phase below is built inside them.

1. **No templates for questions.** A language model writes the query at run time; code validates it. No keyword ladders, no per-question rules.
2. **A search returns every occurrence**, across documents, pictures, plates, recordings and video, tolerant of OCR and speech errors, digit scripts and phone forms, each result with its source. A single hit is a defect.
3. **A model's reading is a candidate, never an identification.** Plates, faces, speech and picture text carry their source, confidence and "not verified" until a person confirms. Abstaining is a success; a confident wrong answer never is.
4. **Every capability is measured before it is switched on:** a pre-registered gate written before the run, a fresh bank for certification (a bank whose misses were read cannot certify), a switch, a rollback tag, and **the owner's chat word for every deploy**.
5. **Real faces and voices only with consent;** public or synthetic data first. Real casework files stay in the vault; plates of real footage never enter the repository. Licences are recorded; for development they are not a blocker, for shipping they are.
6. **CPU only.** No GPU: nothing in this plan needs one; a GPU would bring back a larger intent model, captions and speech fine-tuning.
7. **The case is the unit of trust.** Nothing crosses a case boundary except through the identifier registry and the cross-case rules (section 3.4).

## 2. What "major implementation" means, and what "depth" means

A data type is **implemented (major)** when it has all seven parts of the analyst's loop at a basic level:

| # | Part | Test |
|---|---|---|
| 1 | **Intake**: the file is recognised (layout or reader) and its provenance is recorded | upload, see type and source on the evidence list |
| 2 | **Derive or type**: rows or readings in a table with the source and a confidence | a view answers a count with the exact number |
| 3 | **View**: a viewer in the interface that opens the evidence itself, with the reading beside it | open the item, see the reading |
| 4 | **Ask**: plain-words questions answered with citations (or declined honestly) | a bank of questions with exact truth |
| 5 | **Link**: its identifiers enter the registry and the text search | "where else is this value" finds it |
| 6 | **Bank**: a verified question bank with answers computed independently of the engine | gate written before the run |
| 7 | **Operate**: its own switch, rollback, audit row and a note of limits | switch off returns the earlier behaviour |

**Depth** comes after: analysis features (D1), analyst productivity (D2), scale, enterprise and operations (D3). Depth is listed per family in section 5 and ordered in section 6, Wave 2.

## 3. Architecture

### 3.1 Layers (what exists, and where the new parts plug in)

```
 ANALYST INTERFACES      :4181 old design (all functions)      :4182 new design (Ask, Cases, Dashboard, Evidence, image search)
                                              |  HTTP (workspace proxy, auth, case scope)
 ------------------------------------------------------------------------------------------------------------------------------
 API (Go, port 8091)     routes: /query/hybrid  /cases  /conversations  /analysis/*  /images/*  /evidence/*  /ingest  ...
   QUESTION PATH         image step -> text-intent step -> any-source text search | plate search -> Ask v2 -> governed lane -> older ladder
   CASE LAYER            threads, turns, notes, findings, members, authority, hash-chained audit, export      [cross-case: fan-out only]
   [NEW] REGISTRY        identifier registry (migration 040): normalised strong keys per case, overlaps, notices, access requests
 ------------------------------------------------------------------------------------------------------------------------------
 STORAGE (Postgres 16 + TimescaleDB + pgvector, row-level security, read-only analyst role)
   evidence_items -> derived_artifacts (readings with source, confidence, "not verified") -> typed families + analyst views (v_*)
   typed families:  calls  internet  apps  cells  handsets  registrations  plate sightings  bank/wallet  subscribers  document mentions
   [NEW families]   messages (041)  email  extraction artifacts  tower positions  face governance (042)  registry (040)
   image search: embeddings, window vectors, hashes, known sets (039)
 ------------------------------------------------------------------------------------------------------------------------------
 WORKER + SERVICES       ingest layouts (data) | processors: CDR/IPDR layouts, documents, OCR (EasyOCR), speech routes + language ID,
                         plates (images, video), image-search service | queue (NATS) | model server (Qwen3-4B on CPU, embeddings, speech)
 ------------------------------------------------------------------------------------------------------------------------------
 GOVERNANCE AND MEASUREMENT   switches (79 variables), rollback tags, owner's word, vault + real-data policy, licence ledger,
                              pre-registrations, verified banks, hash register of generated cases
```

### 3.2 Principles that make breadth cheap

1. **Layouts are data, readers are small.** A new file type adds a layout or a reader that emits rows; it does not touch the Ask engine.
2. **Thick views, thin SQL.** A 4B model on a CPU writes about 5.5 tokens a second: each family exposes analyst views that already hold the joins, so the model only selects and filters.
3. **Derived is never merged with ingested.** A model's reading lives beside the record with `source_truth_state`; the answer says which it used.
4. **One normaliser.** The same function that normalises a phone, account, plate, IMEI, IMSI or ICCID feeds typed tables, the tolerant text search and the registry, so the three always agree.
5. **Everything is a switch.** Each new route ships off, is measured on a throwaway copy of the database, and is turned on by the owner.
6. **Foreign tools run out of process** and write files that our readers ingest (iLEAPP, ALEAPP, an email converter): their licences and crashes stay at the boundary, and the source tool, version and hash are recorded with every row (chain of custody, ISO/IEC 27037 [K]).

### 3.3 Where a new data type plugs in (the recipe, used for every family in section 5)

```
 file --> detect (layout registry / reader) --> parse rows or readings (with source path, tool, version, hash)
      --> typed table (+ derived_artifacts for model readings) --> analyst views (v_*) with plain-language descriptions
      --> viewer in :4182 (item + reading side by side) --> text-search mentions + registry keys
      --> bank (oracle SQL, independent of the engine) --> pre-registered gate --> switch --> owner's word --> deploy + rollback tag
```

### 3.4 Cross-case design (decision D41(a), unchanged)

A person who may not open case B learns only **that** an overlap exists, the owner's name and how to ask for access. Automatic links use **strong keys only** (phone in its 0, 92 and 0092 forms, account and IBAN, IMEI, ICCID, IMSI, plate key); national ID numbers stay dark; names are never merged automatically. A link is a **lead**, confirmed or rejected by a person, reversible; every cross-case query and notice is written to the case's hash chain. The registry is also the mention index the media plan needs, so recording-to-call, picture-number-to-subscriber, plate-to-sighting and video-moment-to-cell links are built once.

## 4. Flows

### 4.1 The analyst's flow

```
 open case --> upload evidence --> see each item typed + its readings (quality strip: what is missing)
          --> ask in plain words (English / Roman Urdu / Urdu), attach a picture, draw a box
          --> answer with citations (rows, files, seconds, boxes) + what to keep in mind
          --> open the source --> mark a finding / note --> follow a link (also appears in ... other cases)
          --> export a report with a hash manifest and every citation labelled with its case
```

### 4.2 The question path (today, in order)

1. A sentence that names a photograph goes to the image step. 2. A model reads the sentence and decides whether it asks where a value or word is written (text-intent step); if so, the any-source text search answers with every file, and a plate asked about in pictures goes to the plate search. 3. Otherwise the Ask engine v2 builds the frame, the model writes one SELECT over the analyst views, code checks it (parse-tree allowlist, obligations, cost and row caps, read-only role), and the answer is derived and cited. 4. If it declines, the governed lane and then the frozen older ladder try. 5. An empty records answer gets the files that hold the number or name; **Phase 2 (planned)** adds the files beside every value-bearing records answer without changing its headline.

### 4.3 The cross-case path (planned, X1 to X3)

`ingest -> normalise strong keys -> registry (counts, first and last seen per case) -> "also appears in" chip + notice rows -> access request -> question across cases (per-case answers + one summary, each citing rows of its own case) -> finding tied across cases by a group id, one custody chain per case, visible only to those who may open all of them`.

## 5. Data types: best implementation, minimum version, depth later

For each family: **Now** (live or not), **Best implementation** (what to build and from what), **Minimum version** (the seven-part loop), **Depth later**. Sizes are working days at this laptop's pace; they are estimates, not commitments.

### 5.1 Typed records: calls, internet, apps, cells, handsets, registrations, bank and wallet, subscribers, plate sightings
- **Now:** live, 41 analyst views, verified bank 562 of 565 [R]; 13 provider layouts drafted; subscribers gate not met earlier.
- **Best implementation:** keep the layout registry and thick views. Add the E.164 form from **python-phonenumbers (Apache-2.0)** [P] as one more normalised form in the ingest worker and the registry key; keep the tested 0 / 92 / 0092 forms as the matcher.
- **Minimum:** nothing missing in the loop; close the gaps: promote draft layouts only against a real file from the vault; re-run the subscriber gate on a fresh case.
- **Depth:** B-party and burst analysis, SIM-swap and handset-change timelines, dwell and co-location with tower positions (5.5), anomaly flags with citations, saved views and alerts (watch lists for numbers), bulk export to the analyst's tools.

### 5.2 Messages: SMS rows (live), chat exports, chat screenshots, platform exports  *(migration 041, ~6 d)*
- **Now:** text messages inside the call-record families answer counts and top contacts (one question still abstains: VB-MESS-019). A chat export file (for example `D11-chat-export.txt`) is searchable as text but is not a typed family. No chat screenshot reader.
- **Best implementation:** one `messages` family: thread, participants, sender, time with offset and source, body, attachments (child evidence items), source layout, parse confidence. Three readers into the same rows: (a) text exports of chat apps through layouts with tests on synthetic exports; open-source WhatsApp-export parsers exist [S], none evaluated, so write the small parser against our own layout tests; (b) **chat screenshots**: the EasyOCR role plus bubble geometry (left or right edge decides the side, grouped by date headers), rows flagged `from_screenshot` with a confidence and a link to the picture; (c) extraction output from 5.4.
- **Minimum:** views `v_messages`, `v_message_threads`, `v_participants`; Ask: "who did X message most", "what did X say about Y between two dates" (model-written SQL over the view plus the text search for the body); viewer: conversation view with the source file beside it; registry keys from numbers and handles; **bank of about 60 questions** with exact truth; gate: 0 confident-wrong, at least 90% of answerable questions correct.
- **Depth:** group membership timeline, deleted-message markers, forwarded and quoted chains, attachment-to-evidence links, language mix (Roman Urdu, Urdu), topic and keyword watch lists.

### 5.3 Email: EML, mbox, PST  *(~5 d)*
- **Now:** nothing.
- **Best implementation:** EML and mbox through the **Python standard library** parsers [K] first (headers, body parts, attachments as child evidence); **PST/OST** converted out of process by a library whose licence is unconfirmed (libpst/readpst) [S]: confirm the licence and run it as a separate process before adopting; record tool, version and source hash on every row.
- **Minimum:** `email` family (from, to, cc, bcc, date with offset, subject, message-id, in-reply-to, thread id, body text, attachments); views; Ask "who emailed X in March, with attachments"; text search covers bodies; registry keys from addresses and numbers in bodies and signatures; viewer with headers and body; bank about 40 questions.
- **Depth:** header-hop and send-time analysis, thread reconstruction, duplicate and forwarded detection, attachment families.

### 5.4 Phone extractions (handset dumps)  *(~6 d)*
- **Now:** nothing.
- **Best implementation:** do **not** write handset parsers. Run **iLEAPP** (iOS) and **ALEAPP** (Android), both MIT [P], offline against the extraction and ingest their TSV or LAVA output into artifact families: contacts, call log, SMS, chats (feeds 5.2), location history (feeds 5.5), app usage (feeds the apps family), browser history, media metadata. Their packaged releases bundle third-party code under other licences (for example simplekml under LGPL) [P]: do not redistribute their binaries without a licence review. Record tool, module and version per row; keep the extraction hash.
- **Minimum:** an ingest reader for five artifact types, a viewer that lists extraction items with the module that produced them, views, bank on a **public test extraction** chosen under a pre-registration (not real casework).
- **Depth:** timeline merge across all artifacts of one handset, deleted-record recovery flags, app-by-app views, comparison of two extractions of the same handset.

### 5.5 Location: cell towers, location history, tracks  *(~4 d)*
- **Now:** observed cell positions and cell views (most used cell, distinct cells); an offline map in the old interface; no tower coordinates.
- **Best implementation:** a `tower_positions` family keyed by operator, cell id and date with latitude, longitude, azimuth and source. **First choice: tower lists supplied by the operator through legal process (a layout).** **Lead layer: OpenCelliD, CC BY-SA 4.0** [P]: attribution required, ShareAlike applies to adapted material, a token is needed, CSV covers only cells seen in the last 18 months: label results "estimated"; take counsel's view before building a derived table. Open track formats (GPX, KML, GeoJSON) [K] for location history from extractions or takeouts.
- **Minimum:** view joining CDR and IPDR cell ids to towers; Ask "where was this number around 3 pm" answered as a sector with its uncertainty, never a point; map shows sector wedges; bank on the synthetic case with planted towers.
- **Depth:** dwell and movement maps, plausibility of routes, co-location with real positions, geofence and place lists.

### 5.6 Documents: text, PDF, scans, Urdu  *(Phase 2 ~3 d; Urdu bake-off ~4 d)*
- **Now:** native text and PDF read; any-source tolerant search live; **Phase 2** (files beside records answers) not built; scanned Urdu Nastaliq unreadable (about 70% character error).
- **Best implementation:** keep native extraction; Latin and digits through EasyOCR; run a **pre-registered OCR bake-off for scanned Urdu**, starting from the research in `docs/research/OCR_MULTILINGUAL_RESEARCH_20261117.md` and the earlier Urdu Nastaliq reader results [R], keep the honest limit on screen where it fails; ID card, cheque and MRZ parsers only after real samples exist in the vault.
- **Minimum:** Phase 2 attach-never-replace (text search run beside every value-bearing message, files in a secondary field, headline unchanged; gate 0 changed headlines on the verified sample); Urdu route labelled by its measured error.
- **Depth:** clause and field extraction, document comparison and version diff, form understanding, entity pages built from mentions.

### 5.7 Images: OCR, search, plates, provenance, metadata  *(~5 d for the missing parts)*
- **Now:** copies, parts, words, duplicate groups, draw-a-box and **watch lists (known sets, live today)**; plates in images; OCR.
- **Best implementation:** keep. Add: **EXIF and file metadata** as a view and filters (camera, time, GPS when present); **C2PA Content Credentials reading with c2pa-rs (MIT and Apache-2.0)** [P]: show "signed by ... edited by ..." when present and "no credentials found" otherwise, which proves nothing about authenticity; **tamper and manipulation detection stays a research lead**: the best-known academic detector, TruFor, is reported non-commercial [S], so use it only as a development comparison, never as a shipped verdict; scale by an indexed window query; categories from the existing text-image retrieval; move the 60 `imgsearch-*` test photographs to their own collection (owner's word).
- **Minimum:** metadata view and filter, C2PA reader, a scale gate to be pre-registered (proposed: a case of 10,000 pictures answers a copy search in the same order of seconds as 97), screenshot detection (feeds 5.2).
- **Depth:** reverse search across the cases the person may open (after the registry), clustering of a scene, geolocation hints from EXIF, near-duplicate timelines, redaction and export.

### 5.8 Audio and speech  *(~9 d for the product; Punjabi and Saraiki in the background)*
- **Now:** Urdu and English read; Pashto as a search lead; automatic language detection; transcript viewer; fresh wording 27 of 34; no diarisation, no spoken-number reader, no call linking.
- **Best implementation:** **diarisation with pyannote community-1 (CC-BY-4.0, gated Hugging Face access, no commercial restriction stated; a paid hosted service is separate and must not be used because evidence would leave the machine)** [P]: measure CPU speed and error in the pre-registered bake-off; spoken numbers read as numbers (Urdu, Pashto and English digit words to digits, tested like the text normaliser); a recording linked to its call record by time and numbers through the registry; review and correction events. **Punjabi and Saraiki** follow `docs/research/PUNJABI_SARAIKI_STT_RESEARCH_20261119.md` and the pre-registration [R]: keep the label `LOW_RESOURCE_LANGUAGE` until a language passes its gate; Saraiki needs the owner's Common Voice click.
- **Minimum:** speaker turns in the viewer, "what did the second speaker say about X", a number said aloud found by the text search as digits, bank per language with its gate reported (never one average).
- **Depth:** voice clustering across recordings (clustering only, per D34 to D43), keyword spotting in noise, translation, call-to-call linking, synthetic-voice checks (research lead only).

### 5.9 Video  *(overview ~5 d)*
- **Now:** plates in video live (real clips 9 of 9, 7 of 7, 13 of 15 exact); video questions closed 2026-11-12.
- **Best implementation:** overview first: shot boundaries, a motion index, clock text read from frames, clip export with hash and a time-coded citation, video metadata (ffprobe [K]) including GPS when present; then detection and tracking; faces in video only after face governance.
- **Minimum:** "how many minutes of video does the case hold" (today abstained), "when does something move in this clip", thumbnails at shot starts in the viewer, speech and text read from video join the same search.
- **Depth:** vehicle tracking with plate re-identification, moments linked to cell records and to message times, summaries (needs a vision model: parked with the GPU).

### 5.10 Faces  *(governance ~5 d, then within-case clustering ~3 d)*
- **Now:** detection off; bake-off done (SFace and YuNet stay; the default threshold is far too permissive; no consented test set) [R: `docs/research/FACE_IMAGE_SEARCH_RESEARCH_20261119.md`, `docs/data/face-image-search-results-20261119.md`].
- **Best implementation:** **governance first** (migration 042): a per-case switch held by the authority, a `face_search` permission, a search log with a reason, a kill switch, embeddings moved out of observation payloads into their own table with retention. Then **L1: grouping inside one case**, results shown as candidates with the quality score; **L2 (search and compare) stays off** until the owner and counsel say so and a consented set exists.
- **Minimum:** governance tests pass (a user without the permission gets nothing; every search is logged; the kill switch stops all use), L1 gate on a consented or public set with the false-match rate reported by group.
- **Depth:** quality gating, cross-case search under the registry rules, watch lists of faces under the same governance.

### 5.11 Plates
- **Now:** engine, image search and video live; sealed image gate 83% against 90% not met.
- **Best implementation:** keep; calibrate on real casework files only when supplied (counts and booleans only in the repository); promote the draft layouts only with a vault file. Plate masking choice D42 is owed by the owner.

### 5.12 Cross-case and the registry  *(X0 3 d, X1 5 d, X2 5 d, X3 3 d, X4 later)*
- **Now:** fan-out only. **Best implementation:** the track X0 to X4 of the reconciled plan: generate **Meridian** and **Control** with planted overlaps and decoys (X0); the registry and the "also appears in" chip (X1, migration 040; gate: 95% of planted overlaps found, **zero false overlaps** on Control, a user who cannot open B never sees B's name or value); Ask across cases (X2); findings across cases (X3); then reversible entity resolution (X4) where **Splink (MIT, probabilistic linkage on DuckDB or Spark)** [P] generates name and person candidates for a human to confirm, with transliteration-aware keys; Aleph's cross-referencing is a design reference for how candidate overlaps are listed [S].
- **Depth:** link-chart overlay with dashed unconfirmed edges, merge and split with full undo, organisation-level views.

### 5.13 Link analysis, maps, timelines, reports, interface
- **Now:** connections, co-location, successors, movement, places as views; link chart and map in the old interface; the new interface has four pages.
- **Best implementation:** finish the new interface one page at a time with the owner's look after each (connections, places, timeline, money, vehicles, entities, findings, reports, activity, settings), with the layout rules and the measured checker `apps/investigation-workspace/scripts/ui-layout-check.mjs` (interface tree `NexusAI-ui`, plan of record `docs/ux/v2/DESIGN_REFERENCE_20261105.md` in that tree); reports export with a hash manifest, a page per finding, every citation labelled with its case, and a statement of limits.
- **Depth:** report templates **for documents only** (never for questions), saved searches, alerts, annotation, review queue, shortcuts, an Urdu right-to-left interface, offline installer.

### 5.14 Governance, security, operations
- **Now:** row-level security, read-only analyst role, hash-chained audit, switches, rollback tags, vault. **Open:** identity and multi-factor sign-in (one shared actor today), retention and legal hold, a **reproducible worker image** and an **exported copy of the live images**, push hygiene (live equals committed), backups restored on a schedule.
- **Best implementation:** make live equal committed first (done for the API); document and rebuild the worker from HEAD in a rehearsal; export the images to a disk the owner chooses; test a restore each month.
- **Depth:** roles per case, SSO, retention rules, signed exports, an installer for an air-gapped machine.

## 6. Phases, order and gates

**Capacity rules.** Two or three sessions share one laptop (Docker VM 7.6 GB, model 4 GB resident): **one heavy job at a time** (model run, window indexing, database restore, image build). The lane session owns Ask, text search, registry, cross-case and the new families' backends; the media session owns speech, images, plates, video and faces; the interface session owns `NexusAI-ui`. `main.go` and `query.go` are edited by one lane at a time and routes are registered in a final merge commit.

### Wave 0: close the loose ends (days, now)
| # | Work | Needs the owner |
|---|---|---|
| W0-1 | Push the unpushed commits (`8b74703` and later) | yes: push |
| W0-2 | Export the live API and worker images (`docker save`) to a disk of the owner's choice; rebuild the worker from HEAD in a rehearsal | yes: disk |
| W0-3 | Move the 60 `imgsearch-*` photographs to their own collection and regenerate the verified-bank oracles once | yes: a write to retained evidence |
| W0-4 | Re-ask the older UI query pack (208 questions, measured 2026-10-31) on the live engine in the background | no |

### Wave 1: breadth, every data type reaches the seven-part loop (about 8 to 10 weeks)
| Step | Work | Lane | Gate | Size |
|---|---|---|---|---|
| W1-1 | **Ask Phase 2**: files beside every value-bearing records answer | lane | 0 changed headlines on the verified sample; records answers with rows name their files | 3 d |
| W1-2 | **X0 reference cases** (Meridian, Control) with oracle of planted overlaps | lane | oracle checked by an independent script | 3 d |
| W1-3 | **X1 registry** (040) + "also appears in" chip (interface) | lane + interface | overlap recall at least 95%, zero false overlaps on Control, role test | 5 d |
| W1-4 | **Audio product**: diarisation, spoken numbers, call linking, review; Punjabi and Saraiki bake-off in the background | media | per-language gate reported; no average | 9 d |
| W1-5 | **Messages family** (041): exports, screenshots, views, bank | lane | 0 confident-wrong; 90% of answerable correct | 6 d |
| W1-6 | **Email** (EML, mbox first; PST after the licence check) | lane | bank of 40, 0 confident-wrong | 5 d |
| W1-7 | **Phone extractions** via iLEAPP and ALEAPP output | lane | public-extraction bank under a pre-registration | 6 d |
| W1-8 | **Tower positions** and "where was this number" as sectors | lane | planted-tower bank; every answer states uncertainty | 4 d |
| W1-9 | **X2 and X3**: Ask across cases, findings across cases | lane + interface | oracle questions correct or declined; export shows no value of a case the reader cannot open | 8 d |
| W1-10 | **Video overview** (shots, motion index, clock text, clip export) | media | "total minutes of video" and 5 overview questions correct | 5 d |
| W1-11 | **Images**: EXIF view and filters, C2PA reader, scale, categories | media | scale gate; metadata questions exact | 5 d |
| W1-12 | **Face governance** (042) and L1 inside one case | media | governance tests; L1 gate on a consented or public set | 8 d |
| W1-13 | **Urdu OCR bake-off**; state the limit on screen | media | pre-registered gate; honest label if it fails | 4 d |
| W1-14 | **New interface**: the remaining pages, one at a time, the owner's look after each; usability test | interface | the unaided usability test | ongoing |

**Suggested order** (to keep both sessions busy without two heavy jobs at once): lane W1-1, W1-2, W1-3 while media runs W1-4 and the bake-offs in the background; then lane W1-5, W1-6, W1-7 and media W1-10, W1-11; then lane W1-8, W1-9 and media W1-12, W1-13. Interface works on its own page order throughout and picks up the chips and views as they appear.

**Exit of Wave 1:** every row of the data-type matrix has the seven parts at the basic level, the verified bank covers every family with 0 confident-wrong answers, the question catalogue has no "cannot answer" row in section 7 except the ones the owner parked, and the owner has looked at each new page.

### Wave 2: depth and productivity (after Wave 1; order by analyst value, decided with the owner's feedback)
1. **D1 analysis features per family** as listed in section 5 (timeline merge across all artifact families of one person or handset, B-party and burst analysis, geofences, entity pages, link-chart overlay, thread reconstruction).
2. **D2 productivity**: saved searches, watch lists for numbers and plates, alerts, annotation, review queue, bulk export, report layouts, keyboard shortcuts, an Urdu right-to-left interface. These come from the interface programme in `docs/roadmap/master-roadmap-20261103/04_ui_ux_programme.md` [R] and from the analysts' use of Wave 1; they were not researched online in this pass.
3. **D3 scale and enterprise**: indexed vector queries, cases of 100,000 pictures and 10 million call rows, identity, multi-factor sign-in, per-case roles, retention, signed exports, installer, monthly restore tests.
4. **Parked until a GPU or real data**: a larger intent model (would lift text search past its gate), picture captions, fine-tuned speech models, faces at scale, face search on real casework.

## 7. How each phase is measured

Every step above is run as: pre-registration (the gate, written first) → build behind a switch → measure on a throwaway copy of the database (`rehearse-evidence.sh`, `run-staging-api.ps1 -Stamp -WithAuth`) → a fresh bank asked once for certification → owner's word → deploy with a rollback tag → re-ask the verified bank and the new bank on the live engine → note the result in `docs/data/`. The question catalogue (`docs/qa/question-catalogue-20261124.md`) is the standing test list; each new family adds its bank to it (`tools/showcase/build_question_catalogue.py`).

## 8. Risks

| Risk | Mitigation |
|---|---|
| Live runs ahead of committed; one laptop holds everything | Wave 0; build from HEAD; compare binaries; export images |
| The 4B intent model limits text-search recall | Phase 2 (attach, never replace); do not retune the intent prompt without the three intent banks; a larger model waits for a GPU |
| Foreign tool licences (OpenCelliD CC BY-SA, CC-BY on pyannote, LGPL in packaged iLEAPP/ALEAPP, libpst unconfirmed) | Out-of-process boundary; licence ledger row before adoption; counsel for shipping, not for development |
| Real data leaks into the repository or a third party | Vault, policy, scanner; counts and booleans only; no evidence to any hosted service (the hosted diarisation service is not used) |
| False cross-case links | Strong keys only, zero-false gate on Control, links are leads confirmed by a person, full audit |
| Two sessions overwrite one tree or starve the laptop | Explicit-path commits; one heavy job at a time; lock file; routes registered in a final merge |
| Models misread plates, faces, speech, Urdu print | Every reading is a candidate with confidence; honest limit on screen; abstain |

## 9. Decisions needed from the owner (short list)

1. Push the unpushed commits; choose the disk for `docker save` (Wave 0).
2. Approve Wave 1 in the order of section 6; confirm D41(a).
3. Move the 60 test photographs out of the reference case.
4. Email: allow PST conversion once the converter's licence is confirmed; tower data: operator lists first, OpenCelliD only as a labelled lead (counsel's view on the ShareAlike term).
5. Phone extractions: choose the public test extraction (pre-registration) and confirm that iLEAPP and ALEAPP may run on the laptop.
6. Still owed from earlier: reviewer names (D10), capture names (D26), plate masking (D42), the Common Voice click for Saraiki.
7. Whether the new interface replaces the old one, and when (the cut-over is not decided; :4181 keeps the old design until you say).

## 10. What would change this plan

A GPU (larger intent model, captions, speech fine-tuning return); real casework files or consented data (calibration, face gates); a different analyst priority (for example email before messages); a counsel view that blocks a licence.
