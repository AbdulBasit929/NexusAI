# Media first: the programme of record for the media data types (2026-11-25)

Owner, 2026-11-25: "I approve Wave 1, but cover the media data types first, each implemented best of the best, accurate and productive, with the best utility and use cases, from proper research and study, from the work already done and from online sources."

This file puts the media families at the front of Wave 1 and says, for each one, **what an analyst does with it, what is built, what the research says, what to build, how it is measured, and what it needs from you**. It changes the *order* of `01_roadmap_flow_architecture.md` section 6; everything else in that file stands. Nothing here is built, enabled, deployed or migrated by writing it.

Evidence marks (as in `02_research_sources.md`): **[P]** read at its own page in this pass (2026-11-24 or 25), **[R]** our repository note or measurement, **[S]** secondary: a search result or summary that was not read at the source, **[K]** a standard or well-known fact. Nothing below rests on an [S] claim alone. Sizes are working days at this laptop's pace, not commitments.

## 1. What "best of the best" means here (the rule used for every choice)

A method is chosen in this order, and a method that fails an earlier test is out whatever its score:

1. **Evidence stays on the premises** (no hosted OCR, speech, face or vision service, ever).
2. **Licence is clean for our use** (permissive, or a non-commercial licence only for a lab comparison that never ships). Every licence is a ledger row; licences are recorded, they do not stop development, but an unread or conflicting licence stops *shipping*.
3. **Measured accuracy on our own sealed data**, with the gate written first. A published number is only a reason to measure.
4. **Fits this CPU** (4 threads, about 2 GB per job, one heavy job at a time).
5. **Every reading is a candidate**: it carries its tier (`MACHINE_TRANSCRIPT`, `SEARCH_LEAD`, `LOW_RESOURCE_LANGUAGE`, and the same idea for every family), the model, its version, the input hash and the measured error. A face, a voice or a plate is never an identification.
6. **Search returns every occurrence**, tolerant of reading errors, with a citation and a completeness line.

**Rejected on these rules, for the record:** Ultralytics and BoxMOT trackers (AGPL-3.0); InsightFace pretrained models (non-commercial research only [P]); TruFor (nonprofit-only licence [P]); UTRNet (non-commercial); HunyuanOCR (licence blocks us); MMS-1b-all (CC-BY-NC); the hosted pyannote service; RF-DETR XL and 2XL (Platform Model License, Roboflow account [S]); any cloud OCR, speech or vision API; anything whose weights or training-data terms are unverified, for shipping (see the licence debts below).

**Licence debts, as read at the source by the media session on 2026-11-25 (rows in `docs/compliance/licence-ledger.csv`):** *resolved*: Silero VAD is MIT (the "CC BY-NC" text I saw on 2026-11-25 is only the alt text of one badge; conditional: pin the commit, keep the LICENSE and the model hash, never a fork). *Ship-blocking until resolved (development and evaluation only)*: TransNetV2 weights (code MIT, no licence for the weights anywhere, training data includes IACC.3); WeSpeaker pretrained models ("follow the licence of their dataset"); SpeechBrain ECAPA (tagged Apache-2.0 but trained on VoxCeleb, whose page gives no licence for the audio); OFIQ model files (source MIT, but their own licence files arrive only inside `OFIQ-MODELS.zip`, not yet downloaded); SFace training data (WIDER Face and CASIA-WebFace terms unresolved). Voice embeddings are biometric data: counsel's question before any voice clustering ships. RF-DETR: only Nano to Large (and their Seg sizes) are Apache-2.0; Atto, Femto, Pico, XL, 2XL and NAS fall under the Platform Model License (a Roboflow account, agreement and usage tracking), unusable here.

## 2. The loop every media family must complete (the "major implementation")

For each family the seven parts are: **intake** (read the file) → **derive** (typed rows with model, version, hash, tier) → **view** (an openable viewer beside the source) → **ask** (the model-written query reaches it, with citations) → **link** (to other evidence and cases) → **bank** (a question bank with exact truth, gate and a fresh sealed part) → **operate** (switch, budget, rollback, audit).

| Family | intake | derive | view | ask | link | bank | operate |
|---|---|---|---|---|---|---|---|
| Plates (image, video) | yes | yes | yes | yes | partly (sightings) | yes (83% sealed: not met) | yes |
| Images (copies, parts, words, watch lists) | yes | yes | yes | yes | no (EXIF, cross-case) | partly (N1b met; N1 to N3 not met as registered) | yes |
| OCR (Latin, digits) | yes | yes | yes | yes | no | yes | yes |
| Audio and speech | yes | Urdu, English; Pashto as a lead | yes | yes | **no** (recording to call record) | partly (27 of 34) | yes |
| Video | yes (plates, speech) | plates only | partly | plates only | **no** | plates only | yes |
| Faces | detection off | bake-off only | no | no | no | no | **no governance code** |
| Scanned Urdu | yes | **about 70% character error** | yes | tolerant search only | no | yes | yes |

The gaps in this table are the programme.

**Shared parts that all of it uses (MF-0):**
* **One reading contract** for every machine output (tier, model, version, input hash, measured error, "candidate" label) shown the same way in every viewer and every Ask answer.
* **Moments**: a time-coded event from any media (a clock time in a video, a position in a recording, an EXIF time, a plate sighting) in one view, so the timeline and the cell records can be joined to them. Built as a view over the typed views already there, not a new table.
* **A reproducible worker image.** Today's worker image is not reproducible from HEAD (`docs/roadmap/worker-image-reproduction-20261124.md` [R]) and exists only as a local Docker tag. Every new media processor below ships in that worker, so rebuilding it from HEAD in a rehearsal, and exporting the live images with `docker save`, comes first.
* **Note on the shared index.** The media spine of 2026-11-10 [R] designed a shared mention and text index (`media_mention`, `media_text`). No migration in the repository creates them today (a search of the `.sql` files finds neither), and the any-source text search runs as a tolerant scan. That is acceptable at the present size (3,450 passages when last counted) and is a scale item in Wave 2, not a blocker here.

## 3. Order (media first), and what changes in Wave 1

| Step | Work | Was | Size | An analyst can then |
|---|---|---|---|---|
| **MF-0** | Foundations: worker rebuild rehearsal, `docker save` (your disk), reading contract, moments view, move the 60 test photographs (your word) | W0-2, W0-3 | 2 d | trust that what runs can be rebuilt and restored |
| **MF-1** | **Video overview**: probe, proxy, thumbnails, motion index, shots, clock text, same-footage, clip export | W1-10 | 5 d | triage 12 hours of CCTV in minutes; "how many minutes of video"; cut a court clip with a hash manifest |
| **MF-2** | **Images**: EXIF and file metadata view and filters, Content Credentials reader, screenshot detection, indexed scale, categories | W1-11 | 5 d | "photos taken with this camera or at this place"; "is it signed"; a case of 10,000 pictures searched as fast as one of 100 |
| **MF-3** | **Audio product**: speech-activity detection, speaker turns, spoken numbers, recording-to-call link, review events; Punjabi and Saraiki bake-off in the background | W1-4 | 9 d | "what did the second speaker say about X"; a number said aloud found as digits; the recording opened from its call record |
| **MF-4** | **Faces**: governance first (migration 042), then grouping inside one case | W1-12 | 8 d | how many different faces appear in the case, best crop of each, candidates only, every search logged |
| **MF-5** | **Scanned documents**: Urdu OCR bake-off, script routing, honest error shown | W1-13 | 4 d | scanned Urdu pages searched with the measured error on screen |
| **MF-6** | **Video depth**: detect and track people and vehicles, search by appearance, find the moment by words | Wave 2 | 9 d | "how many people entered between 22:00 and 23:00"; "red motorcycle"; counts and best crops |
| **MF-7** | **Plates**: grow the development bank with public Pakistani plate sets (licence checked), re-measure the sealed gate; calibrate on casework only when you supply files | new | 2 d | fewer missed plates; a number stated for what remains |
| **MF-8** | **Ask over media** (the lane session, light): files beside every records answer (W1-1), so every media output above is reachable from a question | W1-1 | 3 d | the same question returns the rows and the files that write the value |

About 47 working days when run one after another with one heavy job at a time (roughly 9 to 10 weeks). The lane session can do MF-8 and X0 (W1-2, Python generators, no heavy job) while media runs; **media has first call on every heavy-job slot** (model run, indexing, restore, image build). The non-media families (registry, messages, email, extractions, towers, cross-case, remaining pages) continue in parallel only where they need no heavy slot, and otherwise follow MF-6. I expect the whole of Wave 1 to take roughly 11 to 13 weeks rather than 8 to 10, because media now takes the first heavy-job slots: that is the price of your order (an estimate from the sizes above, not a measurement).

**Migration numbers (assigned 2026-11-25; the highest committed is 039).** 040 identifier registry, 041 messages, 042 face governance (as in the reconciled plan); **043 MF-1 video overview** (if it needs tables at all: moments are a view), **044 MF-2 image metadata and credentials**, **045 MF-3 audio product**; MF-6 and MF-5 take the next free numbers when they need one. Email, phone extractions and tower positions (lane) take 046 onwards. Each migration still needs a rehearsal on a throwaway copy and the owner's word before it touches the live database. MF-1's pre-registration is `docs/data/video-overview-preregistration-20261125.md` (gates MF1-P1 to MF1-R; V-A and V-F of the 2026-11-19 registration stand).

Why this order inside media: MF-1 and MF-2 are deterministic and cheap, have exact truth, close the largest visible gaps (the "minutes of video" question that abstains today, metadata questions) and need no new model risk; MF-3 has the most analyst value (calls) but is the longest and shares the CPU with the language model; MF-4 starts with governance because no face feature may ship without it, and the governance code is independent of any model; MF-5 is a measurement before any build; MF-6 is the deepest and comes last.

## 4. Images (MF-2)

**Use cases.** Find the same picture or a copy; find a picture by words in any language; find a region of a picture inside others; mark known harmless or known illegal pictures (watch lists); answer "which camera, when, where"; show whether a picture carries a signed provenance record; separate screenshots (chat evidence) from photographs; keep a large case fast.

**Built today [R]:** copies (hash tier, 100% precision and recall at its operating point on a fresh set, gate N1b met), parts of pictures (region search 215 of 216), window vectors, words in any language, duplicate groups, draw-a-box, more-like-this, watch lists (live 2026-11-24, gates K1 to K5 met), plates and OCR text. Gates N1 to N3 of the first near-duplicate registration are **not met as registered** (precision 98.84%, recall 90.27%, two transformations below 80%); the results note says why.

**What the research says (this pass).**
* **Content Credentials (C2PA)**: `c2pa-rs` is MIT and Apache-2.0 [P 2026-11-24]. Reading a manifest shows who signed and what edits were declared. Absence proves nothing, and a valid manifest proves only that the file is what the signer claims.
* **Metadata**: EXIF, XMP and IPTC give camera, time, software and GPS, and are trivially editable, so every answer says "the metadata states", never "was taken". ExifTool is the reference reader [K]; its page could not be fetched today, so its licence is unconfirmed here; an in-process reader (Pillow or exifread) is the fallback and is what the first version uses.
* **Manipulation and AI-generated image detection is not a verdict**: a 2026 benchmark of 16 detection methods on 12 datasets found the newest commercial generators defeat most detectors, with 18 to 30% average accuracy [S: summary of arXiv 2602.07814]; the best-known academic manipulation detector, TruFor, is licensed for informational and nonprofit use only [P]. SWGDE's *Best Practices for Image Authentication* (18-I-001-2.0, 2025-03-03) says that a still image can be manipulated in a way current tools may not detect [S: search summary; the PDF was not read in this pass]. So the product shows **indicators and provenance** (credentials, metadata consistency, a picture that is a derived copy of a listed original, error-prone regions by hash overlap) and never "authentic" or "fake".
* **Scale**: pgvector 0.8.1 is in the image [R]; exact per-case scans were 22 ms per 1,000 and 169 ms per 10,000 vectors [R]. The scale gate decides between an HNSW index with iterative scans and per-case partitions; measure, do not assume.

**Best implementation.**
1. Metadata at ingest into typed rows (make, model, lens, capture time with offset and source, software, orientation, GPS with uncertainty, file times, dimensions, colour profile) and a view; filters in Ask and in the gallery; GPS shown on the map as a point with the word "stated".
2. C2PA reader run out of process (the worker calls the `c2pa` tool); rows: signer, claim generator, actions, validation status, or "no credentials found".
3. Screenshot detector: aspect ratio, exact device sizes, interface-bar patterns and OCR text density, scored on a public mix of screenshots and photographs; feeds the messages family later (W1-5).
4. Indexed window search with a pre-registered scale gate.
5. Categories: a fixed, short list of words searched with the existing text-to-image model, shown as search hints ("probably a document"), never as labels.

**Gates (proposed, to be pre-registered before building).** Metadata exact on planted fields written by a different tool (100%); C2PA: every public sample file with credentials reads, every file with a broken manifest is reported as invalid, files without credentials say so (0 wrong); screenshot detector precision and recall at least 95% on a public mix; scale: a case of 10,000 pictures answers a copy search in at most 2 s at p95 with results identical to the exact scan; no cross-case leakage in the role test.

**Needs from you.** Your word to move the 60 `imgsearch-*` photographs out of the reference case (MF-0).

## 5. Scanned documents and OCR (MF-5)

**Use cases.** Search scanned pages, forms, ID-card photographs, statements and chat screenshots by words and numbers, in English, Urdu and mixed lines; read identifiers exactly; know how much to trust each reading.

**Built today [R]:** English and digits are good (character error about 0% on printed pages, identifiers exact 99.7 to 99.8%); EasyOCR is routed for Latin text; table-row order fixed; tolerant search over readings is live. **Real Nastaliq newspaper print: about 70% character error, not usable.** Naskh lines: about 9% (Tesseract).

**What the research says (this pass).**
* There is **no open consensus winner for Urdu**: the strongest open numbers come from different papers on different data. The LREC 2026 paper *From Press to Pixels* (829 real newspaper paragraphs) is the only benchmark of this size on real Urdu print; it reports Tesseract far worse than EasyOCR (character error 24.6%), Kraken (22.1%) and a fine-tuned TrOCR (15.9%), and strong proprietary models far ahead (Gemini 2.5 Pro word error 0.133) [R: `OCR_MULTILINGUAL_RESEARCH_20261117.md`; the search of 2026-11-25 returned the same paper and found no newer open leaderboard [S]]. It also reports that a super-resolution step before reading improves LLM-based OCR by up to 70% [S: search summary], which is cheap to test.
* **PaddleOCR-VL** (about 1.0B parameters, Apache-2.0 [P]): the model card says it "supports 109 languages" and names Chinese, English, Japanese, Latin, Korean, Russian, Arabic, Hindi and Thai; **it does not name Urdu** [P]. Third-party documentation lists Urdu under Arabic-script languages [S], so Urdu must be measured on a sample, not assumed. A newer card, PaddleOCR-VL-1.6, is also Apache-2.0 and needs `transformers>=5.0.0` and `paddleocr[doc-parser]>=3.6.0` [P]: a dependency jump that must not touch the live worker; test it in the throw-away evaluation container.
* A community fine-tune of TrOCR for printed Urdu is listed under Apache-2.0 [S]; it is single-line only and not for degraded pages. TrOCR remains the best open reader in the LREC table but needs line detection first.
* UTRNet is non-commercial; Surya's weights carry revenue limits; dots.ocr has a second agreement counsel must read; HunyuanOCR is licence-blocked [R].

**Best implementation.**
1. Run the **pre-registered OCR bake-off v2** already written (`docs/data/ocr-bakeoff-v2-preregistration-20261117.md`) with two additions: PaddleOCR-VL 0.9B/1.0B in the memory-capped container on Urdu and mixed lines, and a super-resolution pre-step on the real-newspaper sample.
2. A **script router**: a page or line goes to the reader that suits its script; mixed Urdu and English lines are the weak spot nobody has benchmarked, so build the mixed set ourselves (forms, statement rows, ID layouts, chat screenshots).
3. If no reader passes, the product says so on screen with the measured error beside every Urdu reading and relies on tolerant search; nothing is presented as a transcript of the page.
4. Fine-tune a line recogniser on rendered Urdu and mixed lines only if the bake-off shows a gap worth closing (public-data cloud training is approved in principle; nothing real is uploaded).
5. ID card, cheque and MRZ parsers only when real samples exist in the vault.

**Gates.** As pre-registered in the bake-off; identifiers and numbers must be exact (at least 99%) for any reader that is allowed to feed a number search; Urdu is labelled by its measured character error until a reader reaches 25% on the real sealed rows.

## 6. Audio and speech (MF-3)

**Use cases.** Search what was said across all recordings in any supported language and get every occurrence with the audio position; find a number or a name said aloud as text; see who spoke when; ask "what did the second speaker say about X"; open the recording from its call record and the call record from the recording; correct a transcript with the original kept; cluster voices inside one case as candidates only.

**Built today [R]:** Urdu and English by Whisper large-v3-turbo (23.4% word error on 40 FLEURS clips); Pashto as a search lead (Omnilingual CTC 300M, sealed word error 30.3%, character error 10.7%); Pakistani Punjabi `LOW_RESOURCE_LANGUAGE` (character error 39.8%, gate 20%); Saraiki not supported; automatic language detection; viewer with transcript and waveform; a fresh-wording run answered 27 of 34. **Not built:** speaker turns, spoken numbers, recording-to-call link, review events. Studio and broadcast figures must never be quoted for telephone audio: the only Pakistani telephone figure found is 62.0% word error for a fine-tuned Whisper small on Urdu telephone speech (CHiPSAL 2025 [P 2026-11-19]).

**What the research says (this pass).**
* **Speech reading**: Whisper (MIT) and faster-whisper (MIT, CPU int8 supported, small model 13 minutes of audio in 1 min 42 s on 8 threads of an i7-12700K) [P]. The COLING 2025 benchmark *WER We Stand* compared Whisper, MMS and Seamless on Urdu: Seamless-large best on read speech, Whisper-large best on conversational speech [P: abstract]; **no newer open Urdu model was found in 2026** [S: search]. Omnilingual ASR is Apache-2.0 and lists Saraiki and Western Punjabi among its languages (Meta's own 7B figures: Saraiki 17.0% character error with 1.5 training hours; Western Punjabi 31.2%) [R: read 2026-11-19]; these are not our model's numbers.
* **Speaker turns**: pyannote `speaker-diarization-community-1` is CC-BY-4.0, access is gated behind agreeing to share contact information, 16 kHz mono input [P 2026-11-24]. A third party measured a real-time factor of about 0.82 on an Apple M2 Pro CPU and a weighted error of about 11.2% on VoxConverse (self-reported by pyannote) [S]; **no telephone-audio figure was found for it**. NVIDIA Sortformer is limited to four speakers [S]; DiariZen reached 13.3% on CALLHOME in a 2025 benchmark and its licence was not read [S]. So: measure community-1 and an embedding-clustering baseline on our own telephone-band mixes.
* **Speaker embeddings (for clustering inside a case)**: SpeechBrain ECAPA-TDNN is tagged Apache-2.0, trained on VoxCeleb 1 and 2, equal error 0.80% on VoxCeleb1-test [P]; VoxCeleb's page gives no licence for the audio, so the trained model is **ship-blocking** until counsel or the authors settle it. WeSpeaker code is Apache-2.0 and its models "follow the licence of their dataset": also ship-blocking. So the speaker-turn route that can ship is pyannote community-1 (CC-BY-4.0); embedding clustering is a development and evaluation baseline only, and voice clustering inside a case waits for counsel [P, media session 2026-11-25].
* **Spoken-language ID**: SpeechBrain's VoxLingua107 ECAPA model is tagged Apache-2.0, covers Urdu, Sindhi, Punjabi and Pashto, 6.7% error on its dev set, noted weak on small languages and foreign accents [P]. Our live detection is a candidate to compare on short telephone clips.
* **Speech-activity detection**: Silero VAD (about 2 MB, under 1 ms per 30 ms chunk on one CPU thread [P]) is MIT (conflict resolved 2026-11-25, section 1). The pyannote pipeline contains its own segmentation, so a separate VAD may not be needed.
* **Synthetic-voice and tamper detection is an indicator at most**: AASIST is MIT [P] but was trained on ASVspoof 2019, and the ASVspoof 5 evaluation of 53 teams (IEEE TASLP, April 2026) concludes that solutions "consistently struggle with generalization", degrade under compression and out-of-domain data [S]. SWGDE's audio documents (Best Practices for Forensic Audio 2.5, 2022; Enhancement of Digital Audio 20-A-001-2.0, listed 2025-08-21; Digital Audio Authentication 1.3, 2018; Core Competencies 3.3, 2025-03-03 [S: search listing; PDFs not read]) require enhancement on a working copy, logged and reversible. The product enhances only a copy, records every operation, and never calls a recording genuine or fake.
* **Data for Punjabi and Saraiki**: Common Voice Saraiki 24.0 is CC0 (5,837 clips, 4.33 validated hours, 60 speakers) and needs your click on the Mozilla data site; MirasAI's 10-hour Saraiki set (2026-04-01) is CC-BY-NC-SA, development comparison only; the Pakistani Punjabi set used in the literature was not released [S: listing pages read by search]. Research and tiers: `PUNJABI_SARAIKI_STT_RESEARCH_20261119.md` [R].

**Best implementation.**
1. Decode (ffmpeg) to 16 kHz mono; keep the original untouched; telephone-band simulation (8 kHz, G.711 and AMR) for tests.
2. Segment by speech activity; language per segment (compare our detector with VoxLingua107); reader by language route; **words with timestamps**.
3. **Speaker turns**: community-1 if it fits the CPU budget and the telephone gate; if it does not, the embedding-clustering baseline (ECAPA or WeSpeaker, development only until their model licences are settled) decides whether the feature is shown at all, with its measured error; turns merged with word times; speaker labels are "Speaker 1, 2" and analyst-renameable, never a name from a voice.
4. **Spoken numbers**: Urdu, Pashto and English digit words and mixed forms converted to digits by a tested normaliser (the same cases as the text normaliser), stored as mentions so the text search finds a number said aloud; the original wording kept.
5. **Recording to call record**: a candidate link by time window and number (rule-based, shown as "may be the call at ..."), confirmed only by an analyst; the registry (X1) later widens it across cases.
6. **Review**: an analyst corrects a reading as an event; the original reading and the correction are both kept and shown.
7. Background, as memory allows and one job at a time: the pre-registered Omnilingual v2 300M and 1B comparison on the unread Punjabi half; Saraiki once the data exists.

**Gates.** Existing speech-lab tiers per language (never one average). Proposed, to pre-register: speaker-turn error (DER) at most 20% on synthetic two-speaker telephone-band mixes built from public single-speaker clips with exact truth, and reported separately on real-speech public conversations; digit strings read aloud: at least 95% exact in Urdu, English and Pashto on public clips, per language; call link: 100% of planted recording-and-call pairs proposed as candidates, 0 confirmed without an analyst; fresh-wording audio bank at least 85% (27 of 34 today).

**Needs from you.** The Common Voice click for Saraiki (D38); the Hugging Face terms click for the diarisation model (it shares your contact information with its authors: your decision, not mine); telephone-line recordings from your team under consent (D26); native-speaker reviewers (D10).

## 7. Video (MF-1, then MF-6)

**Use cases.** Triage hours of CCTV: when does anything move; how many minutes of video are in the case; read the burned-in clock and align it to cell records and messages; find the moment a person or vehicle appears; cut a clip with a burned-in hash and a manifest for a report or court; detect the same footage in two files; read plates (live); read speech (live).

**Built today [R]:** plates in video (V4, live since 2026-11-11; real clips 9 of 9, 7 of 7, 13 of 15 exact; cost 4 to 6 times the video length at 1080p); speech from the soundtrack; the older 12-frame path. **Not built:** the overview, detection, tracking, appearance search, integrity indicators, clip export.

**What the research says.** The full ranked list is `docs/research/VIDEO_EVIDENCE_RESEARCH_20261119.md` [R]; this pass added or re-checked:
* **Licences** [P 2026-11-25]: TransNetV2 code is MIT but **no licence for its weights exists anywhere** and its training data includes IACC.3, so it is ship-blocking (a measured upgrade only if the weights' terms are confirmed) [P, media session 2026-11-25]; ByteTrack MIT (its README notes code borrowed from YOLOX, FairMOT and others, whose terms may apply); Roboflow `supervision` MIT. PySceneDetect BSD-3-Clause [P 2026-11-19]. **RF-DETR**: Nano to Large (and the Seg sizes) are Apache-2.0; Atto, Femto, Pico, XL, 2XL and NAS are under the Platform Model License, which needs a Roboflow agreement, account and usage tracking [P, media session 2026-11-25]. We use only Nano, Small, Medium and Large.
* **CPU speed is unpublished**: no CPU latency for RF-DETR was found; the figures are GPU (T4 TensorRT) [S]. Our pre-registered cost gate (V-B2: one minute of 1080p at 3 frames per second in at most 2 minutes, 4 threads, 2 GB) decides.
* **Vision-language captions on CPU**: no laptop-CPU figure for SmolVLM2 or Qwen3-VL-2B was found; one llama.cpp report has image encoding taking 82 seconds [S]. Captions stay a measured option behind the V-D2 gate (accuracy at least 90%, hallucination at most 5%), search hints only if they fail.
* **Forensic practice**: SWGDE *Best Practices for Digital Forensic Video Analysis* (18-V-001) and *Digital Video Authentication* (23-V-001) require timeline reconstruction, reversible and logged enhancement and three authentication tasks (contextualisation, source identification, content authentication) [S: search summaries]. Integrity output is "indicator", never "tampered".

**Best implementation.** MF-1: ffprobe and proxy; **motion index** by frame difference on a downscaled grey stream; **shots** with the FFmpeg scene filter first (already in the worker, licence-clean; the media session's pick, changed from PySceneDetect and TransNetV2 on 2026-11-25), PySceneDetect (BSD-3-Clause) as the measured alternative, TransNetV2 only if its weights' terms are confirmed; **clock text** at one frame per minute with drift and jump detection and a clock-offset table; thumbnails at shot starts; **same footage** by file hash then perceptual frame signature; **clip export** with burned-in hash, time and a manifest, verified by decoding; a view over probe duration so "total minutes of video" is a plain aggregate. MF-6: person and vehicle detection and tracking (RF-DETR small or RT-DETR R18 with ByteTrack or BoT-SORT) at 3 frames per second, counts per window and best crops; appearance search ("red motorcycle") with SigLIP 2 crop embeddings in pgvector; moment search by frame embeddings; plates with track voting extended; integrity indicators with planted-edit truth; faces in video only after MF-4.

**Gates (already pre-registered 2026-11-19, `docs/data/video-evidence-preregistration-20261119.md` [R]).** V-A motion index: recall at least 95%, at most 1 false interval per hour; V-F clip export: 30 of 30 exports decode with matching frame counts and hashes; V-B1 detection and tracking: recall and precision at least 80%, counts within one object in 90% of 10-second windows; V-B2 cost as above; V-C appearance recall@10 at least 80%; V-D1 moment in the top 5 for 70%; V-D2 captions; V-E indicators at least 95% recall with at most 2% false alarms.

**Needs from you.** Authorised or consented real clips (D26) to turn "measured on synthetic and five public clips" into "measured on footage like yours"; a decision on the licence stance for research-only tracking benchmarks (used for development scoring only).

## 8. Faces (MF-4)

**Use cases.** Count how many different faces appear in a case and show the best crop of each; show other photos that may contain the same face (candidates for a trained reviewer); mark low-quality faces so weak matches are not over-read; later, watch lists and faces in video, always governed.

**Built today [R]:** detection exists but is off in the worker; the bake-off is done (SFace and YuNet stay; the default threshold 0.363 is far too permissive; false-match rates by group were measured on public sets). **No governance code exists** (no per-case switch, permission, search log, kill switch), and embeddings are still stored inside observation payloads.

**What the research says.**
* **Practice**: NIST's face-recognition evaluation (FRTE) treats investigative searches as having effectively no score threshold, with a person reviewing every returned candidate; where scores are weak, true and false matches cannot be told apart and human adjudication is needed; reports were updated 2026-03-30 [S: search results; the NIST page was blocked for the fetch tool today]. A NIST-associated study found trained examiners work best paired with an algorithm [S]. FISWG's documents could not be fetched today (404); the earlier plan keeps its marks as [S].
* **Quality**: OFIQ is the open reference implementation of ISO/IEC 29794-5:2025, now maintained with eu-LISA; a third-party package states MIT with the models under their own terms [S: the BSI repository refused the connection today]. Measured here [R]: with SFace the false non-match rate for probes of quality score 0 to 9 is 9.6% against 0 to 2.5% for 10 and above, so "low quality" is a warning band, not a refusal.
* **Licences** [P 2026-11-25]: InsightFace code is MIT but its pretrained models are for non-commercial research only (a licensing contact is given for `buffalo_l`): **excluded**. AuraFace is Apache-2.0, self-described as trained on a commercial dataset, LFW 0.9965 (our bake-off found SFace at least as good on 4 of 5 public sets and 13 times faster [R]). OpenCV Zoo's repository is Apache-2.0 but says each model has its own licence, so YuNet's and SFace's licences are verified in their folders before shipping.

**Best implementation.**
1. **Governance first** (migration 042, rehearsed on a throw-away copy first): a per-case switch held by a named authority, a `face_search` permission, a search log (who, when, probe hash, case, scope, reason, result count), a kill switch, embeddings in their own table with retention and the same row-level security, nothing exported with an embedding.
2. **L1: grouping inside one case** (SFace's training-data terms are unresolved, so L1 ships only after counsel has read them; building and measuring is allowed now) with SFace at the strict threshold (cosine 0.669 for a false-match rate of 1e-4), OFIQ quality shown, analyst merge and split with undo, every group labelled "may show the same face".
3. **L2 (search a probe against the case, compare two faces) stays off** until you and counsel say so and a consented set exists; it is built and measured on a public-figure development set (openly licensed Wikimedia Commons photographs with the licence and attribution recorded, degraded to look like CCTV, plus FairFace for rates by group), clearly labelled as such.

**Gates.** Governance tests: a user without the permission gets nothing, every search is logged, the kill switch stops all use, no export carries an embedding. L1: pairwise group precision at least 95% at the strict threshold on the development set; false-match rate by group reported; exact-scan completeness 100%; latency at most 1 s at 30,000 faces. These are the gates of `docs/roadmap/media-modules-reconciled-plan-20261120.md` [R].

**Needs from you.** Confirmation of the three face decisions of that plan (1e-4 default and 1e-3 "wider"; a dedicated per-case embedding table; building the public-figure development set and telling counsel it exists). Real-case use of L2 stays your decision with counsel.

## 9. Plates (MF-7)

**Use cases.** Find a plate across every picture and video with citations; list the sightings of one vehicle in time order; join sightings to cell records (later); count vehicles by colour and type (needs MF-6 crops).

**Built today [R]:** plate engine, image search (sealed gate 83% against 90%: **not met**), video V4, plates in the Orion showcase, plate format grammar.

**What the research says.** Two Pakistani licence-plate datasets are listed on Roboflow Universe under CC BY 4.0, and a 2021 paper (Sensors 21(22):7696) introduced a Pakistani plate dataset for non-standard plates; image counts, labels and download terms were **not checked** [S: search]. Whether more local public data closes the 83% to 90% gap is a measurement, not a promise.

**Best implementation.** Check licence and labels of the public Pakistani sets; add them to the *development* bank only; re-measure on a fresh sealed part; keep the plate masking choice (D42) open; calibrate on casework files only when you supply them (counts and booleans only in the repository).

## 10. The research ledger of this pass (what was read, and what was not)

| Item | Result | Mark |
|---|---|---|
| Silero VAD | **MIT** (the CC BY-NC text was one badge's alt text; the LICENSE and README say MIT); conditional: pin commit, keep LICENSE and model hash; about 2 MB; under 1 ms per chunk on one thread | [P] resolved 2026-11-25 |
| WeSpeaker | code Apache-2.0; models follow their dataset's licence | [P] ship-blocking for models |
| SpeechBrain ECAPA (speaker) | Apache-2.0 tag, trained on VoxCeleb whose page gives no licence for the audio | [P] ship-blocking |
| SpeechBrain VoxLingua107 (language ID) | model Apache-2.0; data CC BY 4.0 per the authors (their page refused the connection); Urdu, Sindhi, Punjabi, Pashto in the language list | [P] model, [S] data; conditional |
| AASIST | MIT; trained on ASVspoof 2019; ASVspoof 5 says generalisation is poor | [P] and [S] |
| faster-whisper | MIT; CPU int8; small model 13 min in 1 min 42 s on 8 threads of an i7-12700K | [P] |
| *WER We Stand* (COLING 2025) | Seamless-large best on read Urdu, Whisper-large best on conversational Urdu; no code or data licence stated on the page | [P] abstract only |
| TransNetV2 | code MIT; **no licence for the weights**; training data includes IACC.3: ship-blocking | [P] |
| ByteTrack, supervision | MIT on repository pages | [P] |
| TruFor | informational and nonprofit use only | [P] |
| OpenCV Zoo | Apache-2.0 repository; each model has its own licence | [P] |
| PaddleOCR-VL and VL-1.6 | Apache-2.0; 109 languages, Urdu not named; 1.0B parameters; 1.6 needs transformers 5 | [P] |
| AuraFace; InsightFace | AuraFace Apache-2.0; InsightFace code MIT, models non-commercial | [P] |
| pyannote community-1 speed and accuracy | RTF about 0.82 on an M2 Pro CPU (third party); 11.2% on VoxConverse (self-reported); no telephone figure | [S] |
| RF-DETR licence | Nano to Large (+Seg) Apache-2.0; Atto, Femto, Pico, XL, 2XL, NAS under the Platform Model License | [P] media session 2026-11-25 |
| RF-DETR CPU speed | no CPU figure found | [S] none |
| AI-generated image detection | newest generators defeat most detectors (18 to 30% average) | [S] |
| NIST FRTE, OFIQ, FISWG | investigative lists reviewed by people; OFIQ is the ISO/IEC 29794-5:2025 reference; FISWG page 404 | [S] (fetch blocked or failed) |
| SWGDE audio, image and video documents | document titles, versions and dates from the SWGDE listings | [S] (PDFs not read) |
| Urdu OCR (2026) | no newer open leaderboard; LREC 2026 remains the reference | [S] |
| Saraiki and Punjabi speech data | Common Voice Saraiki 24.0 CC0; MirasAI 10 h CC-BY-NC-SA; CHiPSAL set unreleased | [S] |
| Pakistani plate datasets | two Roboflow sets CC BY 4.0, one 2021 paper; contents unchecked | [S] |
| VLM captions on CPU | no laptop figure found | [S] none |

**Not researched in this pass (and so not relied on):** handwriting recognition, visual document retrieval models for scans without OCR, GeoCLIP-style picture geolocation, vehicle make and model recognition, speech enhancement models, and the legal position on biometric processing in Pakistan (counsel's question). They are listed as Wave 2 leads, not plans.

## 11. Decisions and clicks this programme needs from you

| # | What | For |
|---|---|---|
| 1 | A disk for `docker save` of the live API and worker images (the only copy today) | MF-0 |
| 2 | Your word to move the 60 `imgsearch-*` photographs out of the reference case | MF-0 |
| 3 | The Hugging Face access terms for the diarisation model (shares your contact information with its authors) | MF-3 |
| 4 | The Common Voice click for Saraiki; recordings from your team (telephone line, consented) | MF-3, MF-1 |
| 5 | The three face decisions (thresholds, dedicated embedding table, public-figure development set) | MF-4 |
| 6 | Authorised or consented real clips and photographs (D26), when you have them | MF-1, MF-6, MF-7 |
| 7 | The licence stance for research-only tracking benchmarks used for development scoring | MF-6 |
| 8 | Each deploy, migration (042 for faces), push and `--no-verify` commit, by your chat word as always | all |

## 12. Risks specific to this order

* **One CPU, many models**: diarisation, OCR bake-offs, Omnilingual runs and the language model compete for 7.6 GB; heavy jobs run one at a time, in the background, with the language model unloaded when a measurement needs the headroom (never stop the model container).
* **Public data is easier than yours**: every gate is certified on public or synthetic data and says so; telephone audio, CCTV and real Urdu scans are not measured until you supply them.
* **Licence debts that remain after the 2026-11-25 reading**: TransNetV2 weights, VoxCeleb-trained and WeSpeaker models, OFIQ model files (need `OFIQ-MODELS.zip` downloaded with the owner's word), SFace training data. They stay ship-blocking, so the first versions use only what is clean: the FFmpeg scene filter, pyannote community-1, RF-DETR Nano to Large, and SFace only for development and the governed L1 once counsel has read its training-data terms.
* **The worker is not reproducible from HEAD** (`docs/roadmap/worker-rebuild-rehearsal-20261125.md`): HEAD pins 19 packages, the live worker has 57, and HEAD pins paddleocr and paddlepaddle, which are not installed live. A drafted lock file and `Dockerfile.rebuild` exist; running the rebuild downloads about 1 GB and is one heavy job.
* **Estimates**: 47 days is one person-session at this pace; interruptions, memory squeezes and failed gates move it.
