# The media spine: one backbone for every media family (2026-11-10)

Owner, 2026-11-10: implement the best architecture, pipeline and backend for every phase and every family (images and OCR, faces, video, plates, audio), accurate, productive, useful; the team lead's direction is that search and analysis go through Ask, understood by the model at run time, with no search page and no question templates.

This record adds what the roadmap pack of 2026-11-03 (`master-roadmap-20261103/02_media_programme.md`, `03_architecture.md`) leaves open and fixes the order of building. The pack stays the authority on rules, models, gates and work packages M0 to M11; where this file is more specific it says so.

## 1. The finding that changes the order

The text retrieval that serves transcripts, image text and document passages is a scan: up to 10,000 passages or 16 MB per question in 20 pages, a 5-second limit, comparison in code, one family per question, exact characters only, and no text index on `derived_artifacts` (checked 2026-11-10: key indexes only; 3,450 text passages in the whole database). It is exhaustive and honest while data is small and says "search incomplete" when it is not. Every media family writes text or identifiers that must be found across files (words said, words in pictures, plates, numbers), so the index layer is the backbone, not a later optimisation. Also checked: `pg_trgm` 1.6 and `pgvector` 0.8.1 are installed; trigrams work on Urdu script in this database (en_US.utf8).

## 2. The layers (each can be built, measured and switched on alone)

| Layer | Job | Scales with data? | State |
|---|---|---|---|
| 1. Understanding | A model reads only the analyst's sentence and says what it is about, which operation, which literal. The server validates (verbatim literal, fixed sets, no national ID, no whole-message literal) and builds the structured query. No templates, no stored questions. | Yes: cost is per question, not per row | Built for text search behind `FORENSIC_TEXT_INTENT`; being measured (docs/roadmap/text-intent-preregistration-20261110.md). Widens to plates, similar images, people, moments by adding operations to the allowlisted capability registry, not code paths per question |
| 2. Observations | Every model output is a typed row with the artifact contract: model, revision, params, input hash, locator, confidence, review and truth state. Free text withheld from the SQL engine's views. | Yes (typed, keyed tables) | Migration 033 draft: 12 tables, 9 views, unapplied, no writer yet |
| 3. Index | One shared index over what media families emit. (a) `media_text`: normalised, case-folded passage text of transcripts, OCR regions and document passages, trigram GIN, one row per passage with its locator and family. (b) `media_mention`: normalised identifiers (phone, plate, ID-shaped, account, amount) from every family, btree on the normalised value. (c) vectors: one HNSW index per model id (image, text, face), never mixed. | Yes, by design; proven by a pre-registered scale test | Not built. Today's scan stays as the oracle |
| 4. Retrieval | Index finds candidates; the existing matcher (`forensictext.Match`) verifies, so exact semantics, citations and completeness states are unchanged. "Any source" is one query. A separate, labelled "similar" tier (trigram similarity, vectors) for recognition errors, never mixed with exact matches. | Yes | Scan today |
| 5. Links | Rules produce candidates in `media_link` (recording to call record, plate read to sighting, number in a recording to subscriber, same image elsewhere). Confirmed only by an analyst. | Yes (equality joins on `media_mention`) | Draft table only |
| 6. Review and governance | Review desk; faces at L0 to L2 with recorded authority, audit of every search, no embedding exposed; no identification claim from a face, voice or plate | n/a | Rules written; build in M7 and M8 |

## 3. Each family, end to end

| Family | Intake and processing | What lands in layer 2 and 3 | Understood through Ask as |
|---|---|---|---|
| Documents | Native text, layout, tables, scanned pages through OCR | passage rows, mentions | find words, find a number or name, compare two documents |
| Image and OCR | EXIF, hashes (SHA-256, PDQ, pHash), content class, OCR router (Tesseract best for Urdu, PP-OCRv5 otherwise), parsers for chat screenshots and IDs | OCR region rows, text index, mentions, similarity pairs | find words in pictures, find similar pictures, what does this say |
| Audio | normalise, VAD, language ID, ASR router by language (turbo for en/ur, a measured route for Pashto/Punjabi/Saraiki), spoken-number conversion | segment rows, text index, mentions | find a phrase said, which recordings mention a number, link to the call |
| Video | probe, proxy, shots, motion intervals, overlay-time OCR, detect and track, audio track through the audio path | video-event rows, plate reads, mentions | when does a plate/person/vehicle appear, what was said then |
| Plates (image and video) | detector, grammar-constrained reader with the plate-format registry, track voting on video | plate-read rows, normalised plate in `media_mention` | where was this plate seen, which files show it |
| Faces | detect, align, quality gate; at L1 embed and cluster within a case; at L2 search/compare with audit | detection rows, cluster rows, vectors (per model); no embedding exposed | how many faces, who appears in more than one file (candidates only) |

## 4. Build order (cut line of the pack: M0 to M4 first, then M5 and M6, then M7 to M9)

1. **Finish measuring the understanding layer** (running): development bank 2, then the holdout once.
2. **Spine, step 1: schema.** Extend the unapplied 033 with `media_text` (+ trigram GIN), `media_mention` (+ btree on the normalised value, + tenant/collection/family keys) and the vector tables; rehearse on a throwaway copy of the live database (heavy job, one at a time). Nothing is applied to the live database without the owner's word.
3. **Spine, step 2: writer and backfill.** The worker writes observations to the typed tables and the index as it processes; a backfill reads the existing `derived_artifacts` (3,450 text rows). Reconciliation: counts and hashes equal the source.
4. **Spine, step 3: indexed retrieval behind a switch**, with the scan kept as the oracle. Pre-registered scale test on synthetic passages at 100k, 1M and 10M: same results as the scan on the existing bank (recall 100%), p95 latency target, written before running.
5. **Families in the pack's order** on that backbone: speech (M1/M2, blocked until the two model downloads finish), images (M3), OCR bake-off (M4), plates (M5), video (M6), faces L0 to L1 with the governance first (M7), cross-file links and review desk (M9). Each is a pre-registered work package with its own gate; each switches on alone.

## 5. What this does not change

Rules 1 to 9 of the pack, section 1 (observation never fact; no identification claim; evidence stays on the premises; licence-clean; one role on at a time; abstain when unsure; hostile input; reprocess with lineage; analyst wellbeing). No cloud service sees evidence. Licence-blocked models stay out (AGPL detectors; research-only face weights). Real data stays local and is never used for tuning; sealed sets are run once.

## 6. Open items that need the owner

* Applying any migration to the live database (each time, by word).
* Consented real-kind samples (D26): telephone audio, plates in CCTV conditions, faces with consent. Without them, ANPR and face gates are certified on public and synthetic data only and stated as such.
* Licence review for the plate detector and reader currently running (ship-blocking in the ledger); the replacement path is a fine-tuned RF-DETR plus our own reader.
* Faces beyond detection: recorded authority per case and the legal review of the pack's section 7.4 before L1 is switched on for any real case.
