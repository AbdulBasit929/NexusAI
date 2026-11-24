# Reconciled plan and next phases (2026-11-24)

**What this is.** One read of the plans of record against the live system, to say what is true today, what the plans got out of date, and what to do next, in order. Owner's request of 2026-11-24: reconcile the roadmaps, find the next best phases, add **cross-case findings and Ask across cases**, and take the media data families to their best working state. **Nothing here is built by this note; every deploy still needs the owner's word.**

**Read for it:** `master-roadmap-20261103/` (pack, files 00 to 07), `media-modules-reconciled-plan-20261120.md`, `master-plan-20261021.md` (ledger), `reference-cases-programme-20261028.md`, `ask-exhaustive-search-plan-20261119.md`, `image-search-real-world-plan-20261123.md`, `docs/architecture/CONVERSATIONS_AND_CASE_LAYER_20261103.md`, and the live system (containers, images, the catalog of the live database, the code of both trees, the memory of the sessions). Claims about the live system were checked on 2026-11-24.

## 1. Where each data family stands (the analyst's loop: ingest, derive, open, ask, cite, link, report)

| Family | Live today | Measured | What is missing for the loop | Next step |
|---|---|---|---|---|
| **Typed records** (calls, internet, apps, cells, handsets, registrations, plate sightings, bank and wallet, subscribers) | yes; Ask engine over 34 analyst views; case layer (threads, notes, findings, members, audit) live since 2026-11-10 | verified bank 473/484 (2026-11-03), sample of 64 re-run 2026-11-23: 62 correct, the two differences are newer data in the staging copy (proved by a switch-off control) | message-export family, phone-extraction, email, location history; subscribers gate was not met | message family (section 4) |
| **Documents** (docx, pdf, txt) | native text; **any-source tolerant text search live 2026-11-24** | fresh bank 69%; fixed build 70 to 86% fully answered, against 20% before | files for records answers that have rows (Phase 2 of the Ask results note); Urdu scanned documents | Phase 2; Urdu OCR bake-off |
| **Images: text (OCR)** | EasyOCR role live (Latin and digits fixed 2026-11-19) | Urdu Nastaliq print 70% character error: not usable | Urdu route; chat-screenshot, ID and cheque parsers | label Urdu OCR honestly; screenshot parser |
| **Images: search** | live on Lahore: copies, region search, 34 window vectors, duplicate groups, word search, draw-a-box | gates N1 to N3 not met as registered (recall 90.3%); owner is testing | scale (indexed window query); watch lists; duplicate badge; categories | owner's order, section 4 |
| **Plates** (images, video) | engine, image search, video V4 live | sealed image gate 83% against 90%: not met; the 13 provider (call and internet record) layouts are DRAFT | real casework calibration; registry promotion needs the vault | keep, measure on real files when supplied |
| **Audio and speech** | speech routes, automatic language detection, Pashto lead search live | Urdu, English, Pashto read; fresh-wording audio 27/34 (two gates not met); Punjabi and Saraiki **not measured** | the audio product (VAD, diarisation, spoken numbers, translation, call linking) | audio product (section 4) |
| **Video** | plates in video (V4) | real clips: 9/9, 7/7, 13/15 exact | overview (shots, motion index, clock text, clip export), tracking, moments linked to cells | video overview |
| **Faces** | detection **off** in the live worker | bake-off done: SFace and YuNet stay, default threshold far too permissive, no consented set | **no governance code at all** (per-case switch, permission, search log, kill switch); embeddings still stored inside observation payloads | governance first; L1 inside a case only |
| **Cross-case** | case-layer threads that put one question to each of 2 to 10 cases in turn; interface asks every case a few at a time | nothing joins one case to another, by design | identifier registry, overlaps ("also appears in"), notices and access requests, cross-case findings, entity resolution, reference cases to test them | section 3 |

## 2. What the reconciliation found

1. **Live is ahead of committed.** 115 paths of the lane tree and 24 of the interface tree are uncommitted, and 95 local commits are not pushed. Live API and worker images were built from uncommitted files and from an older commit than HEAD; on 2026-11-24 a deploy had to be rebuilt by comparing binaries because the live image did not contain committed code. One laptop holds all of it. **This is the first thing to fix.**
2. **`NEXUSAI_CONTINUATION.md` is stale** (last refreshed 2026-11-03; it does not say that the case layer, image search and the text search are live).
3. **Migration numbers drifted from the plan.** The pack reserved 033 for media tables, 034 for conversations, 035 for the identifier registry and cross-case, 036 for message families. In fact 033 was never used, 034 (conversations and the case layer) is live, 035 to 037 became media view descriptions and the audio language views, 038 is image search. **The next free number is 039:** identifier registry and cross-case take 039, message families 040, face governance 041. This note supersedes the numbering of file 06.
4. **Cross-case was planned late (Phase 3, weeks 11 to 17), and the case layer built only the fan-out.** `internal/caselayer/crosscase.go` says it plainly: the engine does not read across cases and nothing joins one case's rows to another's. The owner's observation is right: findings across cases and a true Ask across cases are not implemented. The decision D41 (a: existence, owner's name and an access request) stands and is the design rule below.
5. **No reference cases to test cross-case.** Only Orion exists in `C:\NexusAI-Showcase`; Meridian and Control (overlaps, decoys, shared identifiers) were specified in 2026-10-28 and never generated.
6. **The Lahore reference case is now also the image-search test bed.** 60 test photographs were uploaded into it, which changes its photo counts and so the oracles of the verified bank. The test pack should live in its own collection.
7. **Gates 0 and 1 of the pack were never formally closed** (the full verified-bank re-run, the speech gate per language, media bank v1).
8. **Two sessions share one laptop** and slowed each other many times (the Docker VM is 7.6 GB, the model is 4 GB resident); a build base trap cost a deploy rebuild. Rules in section 6.
9. **Hardware has not changed: no GPU.** A larger intent model, captions by a vision model, fine-tuning and faces at scale stay parked; nothing in the plan below needs them.

## 3. Cross-case: findings and Ask across cases (new track X)

**Design rules (from `03_architecture.md` section 4 and D41, unchanged):** the case is the unit of trust; nothing crosses a case boundary except through this mechanism; a person who may not open the other case learns only that an overlap exists, the owner's name and how to ask for access; every cross-case query and notice is audited in the case's hash chain; a link is a **lead** ("candidate"), confirmed or rejected by a person, reversible. Strong keys only for automatic links: phone (0, 92 and 0092 forms are one), account and IBAN, IMEI, ICCID, IMSI, plate key; national ID numbers stay dark; names are never merged automatically.

| Stage | What it adds | Reuses | Gate | Size |
|---|---|---|---|---|
| **X0 reference cases** | generate **Meridian** and **Control** with planted overlaps (same phone in other formats, same account, same plate in an image, same IMEI), look-alike decoys, and a value written only in a document or recording; oracle of truth per overlap | the case generator, Orion | oracle checked by an independent script | 3 days |
| **X1 identifier registry** (migration 039) | one table of normalised strong keys per case with counts and first/last seen, filled from the typed families and from the text mentions (the same normaliser as the tolerant text search); "**also appears in**" chip on a number, account or plate; notice rows and access requests; backfill job and ingest hook | tolerant matcher, mention index, `case_role_in_all()` of migration 034 | overlap recall at least 95% of the planted ones; **zero false overlaps** on Control; role test: a user who cannot open B never sees B's name or value | 5 days |
| **X2 Ask across cases** | the engine gets a governed multi-case scope: (a) "where else is this value written" (the any-source search over every case the person may open, grouped by case); (b) "what do cases A and B share" answered from the registry, without raw rows crossing; (c) "everything about X in all my cases" as per-case answers plus one summary; each answer cites rows of its own case | text search module, case-layer threads (already ask each case), registry | the oracle questions of Meridian and Control answered correctly or declined; every cross-case query audited | 5 days |
| **X3 findings across cases** | a finding may cite turns or rows of two or more cases: stored as one finding per case (each custody chain stays whole) tied by a `cross_case_group_id`, plus a case-relation record (candidate or confirmed, reason, who); visible only to people who may open all the cases; report export labels every citation with its case | `finding`, `case_audit_event`, report export of the case layer | export shows no value of a case the reader cannot open; confirmed relation shown on both cases | 3 days |
| **X4 later** | link-chart overlay (cross-case edges dashed until confirmed), reversible `same_as` entity resolution, transliteration-aware names | registry | zero false merges (Control) | after X3 |

**Why X1 comes before the rest of the media work.** The same registry is what the media plan calls the "mention index" and the **cross-modal links** (recording to call, a number read from a picture to its subscriber, plate to sighting, image copy to case, video moment to a cell): build it once and both tracks use it.

## 4. Media: the best working order, one family at a time (each: pre-registered gate, switch, owner's word to deploy)

1. **Audio product** (largest everyday value). Finish the five-language path on what is measured (Urdu, English, Pashto) with the transcript viewer, spoken numbers read as numbers, a recording linked to its call record and to the registry, review and correction events. Punjabi and Saraiki run the pre-registered CPU bake-off in the background and keep `LOW_RESOURCE_LANGUAGE` until they pass; Saraiki needs the owner's Common Voice click.
2. **Messages** (the family that is half in the plan and not built): chat exports (text) and **chat screenshots** (the OCR route reads them into the same rows), one family, one set of views, one bank. It makes the chat evidence askable and feeds the registry.
3. **Images**: keep the owner's test cycle; the next two items are the owner's order (watch lists of hashes, or the duplicate badge on gallery thumbnails first; watch lists first is recommended: a rehearsed migration, gate 100% flagged and 0 false flags); then EXIF filters, categories, scale with an indexed window query. Move the 60 test photographs to their own collection.
4. **Documents and Urdu OCR**: bake-off for scanned Urdu documents; state the limit on screen where it fails; ID, cheque and MRZ parsers only after real samples exist.
5. **Video**: overview first (motion index, shots, clock text, clip export), then detection and tracking; faces in video only after face governance.
6. **Faces**: build the **governance first** (per-case switch with the authority, a `face_search` permission, a search log with reason, a kill switch, embeddings moved out of observation payloads into their own table, migration 041), then clustering inside one case; search and compare (L2) stay off until the owner and counsel say so.
7. **Plates**: keep what is live; improve by real casework calibration and promote the draft layouts only with a vault file.

A family is **finished** only when it has the loop of section 1: a view, a viewer, an Ask route with citations, a link into the registry, a bank with exact truth and an owner's look.

## 5. Recommended order

| Step | Work | Who | Done when |
|---|---|---|---|
| **A0 (now)** | **Make live equal committed.** Commit the media session's files and mine in separate commits with explicit paths, rebuild the API and worker images from HEAD, tag them, push the branch (outward step: the owner's word) | both sessions, owner's word for the hook bypass and the push | a build from HEAD equals the live binary except for the build id |
| A1 | Refresh `NEXUSAI_CONTINUATION.md` and the supersession map; move the image test pack into its own collection; regenerate the verified-bank oracles and re-run the whole bank once in the background | this session | file under 300 lines; bank counts reported |
| A2 | Ask Phase 2: run the text search beside every value-bearing message and attach the files in a secondary field (headline unchanged); one interface line | this session (Go), interface session | records answers with rows name their files; 0 changed headlines on the verified sample |
| **B (weeks 1 to 3)** | X0, X1, then the audio product; the image test cycle continues | this session (X), media session (audio, images) | X1 gate met; audio gate per language reported |
| **C (weeks 3 to 6)** | X2, X3, the message family, video overview | this session, media session | cross-case oracle met with zero false overlaps; message bank at 0 confident-wrong |
| **D (weeks 6 to 8)** | face governance and L1, Urdu OCR bake-off, cut-over preparation (usability test, interface order) | media session, interface session | governance tests pass; the unaided usability test |

**Ownership.** This session: Ask, text search, the registry, cross-case (lanes E and C). Media session: speech, images, plates, video, faces (lane M). Interface session: `NexusAI-ui`. Files that two lanes touch (`main.go`, `query.go`) are edited one lane at a time; routes are registered in a final merge commit. **One heavy job at a time on the laptop**: model runs, window indexing, a database restore and an image build never overlap (they starved each other repeatedly: watchdog kills, model timeouts, a near-full Docker VM).

## 6. Decisions needed from the owner (short list)

1. **A0:** may both sessions commit with the hook bypass (the Windows lint failure is unrelated to the code) and may the branch be pushed to origin?
2. **Cross-case:** approve X0 to X3 in this order, and confirm D41 (a).
3. **Images next:** watch lists first (recommended) or the duplicate badge first.
4. **Test pack:** move the 60 image-search test photographs out of the Lahore reference case.
5. **Still owed from earlier:** reviewer names (D10), capture names (D26), the plate masking choice (D42), the Common Voice click for Saraiki.

## 7. Not recommended now, and why

A bigger intent model, vision-model captions, fine-tuning and faces at scale (no GPU); face search on real cases (no governance code, counsel gate); another change to the intent prompt (measured to regress the intent step); a global vector index across cases (measured recall 0.86 with filters; per-case exact scan is fast enough); a rewrite of the Ask engine (the verified bank shows it is not the limit).

## 8. What would change this plan

A GPU (the larger intent model, captions and the speech fine-tunes come back), real casework files or consented data (calibration and the face gates), or the owner choosing a different order for cross-case against media.
