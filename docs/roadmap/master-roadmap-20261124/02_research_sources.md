# Research sources behind the roadmap (2026-11-24)

**How to read this file.** Each source is marked **[P]** read at its own page on 2026-11-24 (the licence and terms are quoted as the page states them), **[R]** our own repository note or measurement, **[S]** a secondary finding (search result or general knowledge) that was **not** read at the source in this pass and must be verified before anything depends on it, **[K]** a published standard or protocol. The roadmap never rests on an [S] claim alone. No page showed a release date, so none is claimed; check the repository's own release list before pinning a version.

## 1. Read at the source today [P]

| Tool | What it is for | Licence and terms as the page states them | Decision |
|---|---|---|---|
| **python-phonenumbers** (github.com/daviddrysdale/python-phonenumbers) | Parse, validate, format and geocode phone numbers (port of Google's libphonenumber) | Apache-2.0; no other terms stated | **Adopt** in the ingest worker and as the E.164 registry form, beside the tested 0 / 92 / 0092 matcher |
| **Splink** (github.com/moj-analytical-services/splink) | Probabilistic record linkage and deduplication (Fellegi-Sunter); DuckDB and Spark backends; the page announces "Splink 5" | MIT (code) | **Adopt for X4** (name and person candidates for a human to confirm); never an automatic merge |
| **iLEAPP** (github.com/abrignoni/iLEAPP) | Parser for iOS and iPadOS extractions; output HTML, TSV, timeline, LAVA | MIT (code). Packaged releases bundle third-party code under other licences (for example simplekml under LGPL, heicreader under MIT and BSD-3-Clause) | **Adopt, out of process**; ingest its output; no redistribution of its binaries without a licence review |
| **ALEAPP** (github.com/abrignoni/ALEAPP) | Same, for Android extractions | MIT (code); same bundled-licence note | **Adopt, out of process** |
| **c2pa-rs** (github.com/contentauth/c2pa-rs) | Read, create and validate C2PA Content Credentials (provenance manifests) | MIT and Apache-2.0, dual; "some components and dependent crates are licensed under different terms" | **Adopt to read credentials**; absence of credentials proves nothing |
| **pyannote speaker-diarization-community-1** (huggingface.co/pyannote/speaker-diarization-community-1) | Who spoke when (16 kHz mono input) | **CC-BY-4.0**. Access is gated: you must agree to share your Hugging Face information. No commercial-use restriction stated. The page also promotes a paid hosted service, which is separate | **Evaluate in the bake-off on CPU**; attribution required; **never use the hosted service** (evidence would leave the machine). The benchmark table on the page was last updated 2025-09 |
| **OpenCelliD** (opencellid.org; terms read from docs.opencellid.org/docs/attribution and opencellid.org/downloads.php) | Community database of estimated cell tower locations | **CC BY-SA 4.0** for API results, downloads and daily changes. Attribution required (credit, link, indicate changes); **ShareAlike applies to adapted material**; exceptions only in writing from Unwired Labs; downloads need an account and an API token; CSV exports cover only cells observed in the last 18 months | **Lead layer only**, labelled "estimated"; counsel's view before building a derived table; operator-supplied tower lists are the primary source |

Caveat from the reading: the OpenCelliD home page does not state the licence itself; it is on the attribution and downloads pages. For the full licence texts check each repository's LICENSE file.

## 2. Our own research and measurements [R]

| Topic | Where | What the roadmap takes from it |
|---|---|---|
| Faces and image search | `docs/research/FACE_IMAGE_SEARCH_RESEARCH_20261119.md`, `docs/data/face-image-search-preregistration-20261119.md`, `...-results-20261119.md` | SFace and YuNet stay; default threshold far too permissive; no consented set; governance first |
| Image search | `docs/data/image-search-m1-*`, `image-search-region-results-20261123.md`, `image-known-sets-*` | Copies, parts, words, draw-a-box; gates N1 to N3 not met as registered; watch-list gates K1 to K5 met |
| Punjabi and Saraiki speech | `docs/research/PUNJABI_SARAIKI_STT_RESEARCH_20261119.md`, `docs/data/punjabi-saraiki-stt-preregistration-20261119.md` | CPU bake-off in the background; `LOW_RESOURCE_LANGUAGE` label until a language passes; Saraiki needs the Common Voice click |
| Speech lab | `docs/data/speech-lab-*` | Urdu and English read; Pashto as a search lead; per-language gates, never one average |
| Video | `docs/research/VIDEO_EVIDENCE_RESEARCH_20261119.md`, `docs/data/video-evidence-preregistration-20261119.md`, `video-plates-research-and-preregistration-20261111.md` | Plates in video live; overview before tracking; faces in video after governance |
| OCR | `docs/research/OCR_MULTILINGUAL_RESEARCH_20261117.md`, `docs/data/ocr-bakeoff-v2-*`, `urdu-nastaliq-reader-*` | Latin and digits good; Urdu Nastaliq print about 70% character error |
| Text search | `docs/data/ask-exhaustive-search-results-20261119.md` | Tolerant tiers; a 4B intent model cannot separate "where is X written" from "how many records has X" by wording |
| Architecture and cross-case | `docs/architecture/CONVERSATIONS_AND_CASE_LAYER_20261103.md`, `docs/roadmap/master-roadmap-20261103/03_architecture.md`, decisions D34 to D43 | Case is the unit of trust; D41(a) |
| Interface programme | `docs/ux/MASTER_UI_UX_DIRECTIVE.md`, `docs/ux/research/RESEARCH_REGISTER_20261026.md` | Productivity features (Wave 2) come from here, not from a new online search |

## 3. Secondary findings, not verified at the source [S]

| Claim | Status and what to do |
|---|---|
| **TruFor** (image manipulation detection) is released for non-commercial use | Seen in search; treat as a research comparison only; if ever considered for shipping, read its licence first |
| **libpst / readpst** converts PST files; its licence was not confirmed | Not read; confirm the licence before using it for PST. EML and mbox do not need it |
| **Open-source WhatsApp export parsers** exist | Seen in search; none evaluated; the roadmap writes a small parser against our own tests instead |
| **Aleph** (OCCRP) cross-referencing lists candidate overlaps; the legacy Aleph product is reported as sunset | Used only as a design reference for how overlap candidates are listed; its page was not readable for the fetch |
| Public test extractions for iLEAPP/ALEAPP | Not chosen yet; picking one is a pre-registration step, with its licence recorded |

## 4. Standards and protocols [K]

ISO/IEC 27037:2012, guidelines for identification, collection, acquisition and preservation of digital evidence (the reason every row records tool, version and source hash, and why the audit is hash-chained); E.164 numbering; EML and mbox mail formats and the Python standard-library parsers; GPX, KML and GeoJSON track formats; C2PA (via c2pa-rs above).

## 5. What was **not** researched online in this pass

Analyst productivity features (Wave 2, D2) were taken from the interface programme and the use of Wave 1, not from a new survey. A GPU-dependent option (larger intent model, captions, speech fine-tuning) was not researched because it is parked. Anything about real casework data is out of scope for online research by policy.
