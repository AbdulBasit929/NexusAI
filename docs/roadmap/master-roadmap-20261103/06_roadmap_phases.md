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
