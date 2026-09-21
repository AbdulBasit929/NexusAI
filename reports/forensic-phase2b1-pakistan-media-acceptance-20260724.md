# NexusAI Phase 2B.1 Pakistan and Deterministic Media Acceptance

Date: 2026-07-24  
Scope: deploy Pakistan-aware timestamp provenance, validate a synthetic messy CDR end to end, and deliver the first model-free media metadata slice

## Executive result

The forensic API and worker were rebuilt and redeployed successfully with explicit
rollback images. Pakistan jurisdiction and timezone assumptions now survive from
upload through job and evidence provenance. A synthetic messy CDR passed the full
LocalAI → forensic API → NATS → worker → PostgreSQL path. Deterministic PNG and
WAV metadata extraction also passed live while OCR and STT remained correctly
pending.

No real CDR was ingested. No model, schema migration, backfill, Git stage, commit,
push, pull request, or external publication was performed.

## Deployed runtime

| Component | Deployed image digest | State |
| --- | --- | --- |
| LocalAI | `sha256:0731ab05fa080fae9bc90ddbcc26f74fcb554389ee82aa0282e5373a27144c9b` | Healthy; unchanged |
| Forensic API | `sha256:d32e5118d18849fd4aae024944e547573c9643189a7b5bb2a9f72cbecde64980` | Running; healthz OK |
| Forensic worker | `sha256:faeb65590b1f6a61e7458a9c73c763b5c82dc772df29f20fd8843691c9615268` | Running; NATS subscribed |

The worker reports `tzdata=2026.3` and resolves `Asia/Karachi` through IANA
timezone data.

Rollback tags:

- `nexusai-forensic-records-api:pk-profile-pre-20260724` →
  `sha256:03a55612567e25fb04d9a8c1cec67bd2d35a7e4de30f6c67fd081e1582471bbe`
- `nexusai-forensic-records-worker:pk-profile-pre-20260724` →
  `sha256:d2ec7cf626d21b6be399017e87b660f5a679ef22962583afd91e4ff43c8f8d18`
- Intermediate API rollback tags preserve the timezone-only and
  evidence-provenance checkpoints before deterministic media extraction.

## Pakistan CDR live acceptance

Synthetic source:
`ingestion/forensic_records/tests/fixtures/pakistan_cdr_messy_synthetic.csv`

Final collection: `forensic-pakistan-cdr-acceptance-20260724`  
Diagnostic predecessor: `forensic-pakistan-cdr-validation-20260724`

| Measure | Final result |
| --- | ---: |
| Knowledge Base entries | 1 |
| Evidence items | 1 |
| Ingest jobs completed | 1 / 1 |
| Input rows | 5 |
| Accepted rows | 4 |
| Exact duplicates | 1 |
| Rejected rows | 0 |
| 14-digit IMEI rows retained | 4 |
| Negative cell sentinel rows retained | 1 |
| Evidence warnings/errors | 0 / 0 |

Provenance verification:

- Job jurisdiction: `PK`.
- Job source timezone: `Asia/Karachi`.
- Evidence jurisdiction: `PK`.
- Evidence source timezone: `Asia/Karachi`.
- Naïve source times beginning at 08:00 local were stored beginning at 03:00 UTC.
- Explicit offsets continue to override the deployment default.

The first diagnostic ingest exposed that jurisdiction and source timezone were
present on the job but absent from evidence metadata. A pure metadata builder and
Ginkgo regression closed that gap before the final acceptance upload.

## Deterministic media metadata v1

The extractor runs synchronously against the immutable spool file during evidence
registration. It is bounded, deterministic, versioned, and does not call a model.

Implemented:

- PNG/JPEG/GIF: bounded header decode, detected format, width, height.
- RIFF/WAVE: audio format code, channels, sample rate, byte rate, bit depth,
  audio-data byte count, and integer duration in milliseconds.
- Extractor identity: `nexusai_deterministic_media_metadata` version `1.0.0`.
- Corrupt supported media: preserve source evidence, store no invented metadata,
  and emit a bounded warning.
- Unsupported formats: preserve normal registration/pending behavior.

Live collection: `forensic-media-metadata-acceptance-20260724`

| Evidence | Verified metadata | Route preserved |
| --- | --- | --- |
| `favicon-16x16.png` | PNG, 16 × 16 pixels | `image_ocr_vision_pending` |
| `audio_call.wav` | PCM format 1, mono, 8 kHz, 16-bit, 16,000 byte/s, 4,000 data bytes, 250 ms | `audio_stt_pending` |

Both evidence items carry `PK` and `Asia/Karachi`; both have zero warnings and
errors. Technical metadata does not mark OCR, object detection, transcription,
language identification, diarization, or TTS quality as complete.

## Verification

- Python worker and adapter suites before deployment: 17 tests passed.
- Go forensic API after Pakistan evidence-provenance fix: passed; package reported
  29.733 seconds after compilation.
- Go forensic API after deterministic media implementation: passed; package
  reported 16.829 seconds.
- Go forensic API after PNG/JPEG/GIF matrix expansion: passed; package reported
  30.937 seconds.
- Worker image build: 172.1 seconds; pinned Python dependencies installed.
- Initial API build: 27.2 seconds.
- Evidence-provenance API rebuild/deploy: 48.4 seconds.
- Deterministic-media API rebuild/deploy: 49.6 seconds.
- Compose configuration remained valid.
- Live LocalAI, API, worker, NATS and PostgreSQL path passed.

## Security, provenance, and failure behavior

- Image parsing reads at most 4 MiB for header configuration and does not decode
  full pixel payloads.
- WAV parsing reads fixed headers and seeks over chunks; it does not execute media
  or invoke codecs.
- Original hashes, spool references, KB source entries, evidence IDs, collection,
  jurisdiction, timezone, extractor ID/version, warnings and errors remain
  queryable.
- Media failures are partial extraction failures, not evidence loss.
- Tenant and collection behavior is unchanged; no RLS policy or schema change was
  required because `media_metadata` already existed.

## Baseline and Git state

- `records-demo-verified`: unchanged at 9,250 deterministic records.
- Three new isolated neutral collections contain only synthetic acceptance data.
- Working tree remains unstaged and uncommitted.
- Nothing was pushed to GitHub.

## Next gate

Continue Phase 2B.1 with fixed binary goldens and bounded parsers in this order:

1. EXIF orientation and safe WebP/BMP/TIFF image inventory.
2. FLAC, MP3 and MP4-family audio/container headers without codec execution.
3. PDF page count and attachment/encryption inventory.
4. ZIP/TAR inventory with traversal, link, member-count, size, ratio and nesting
   limits.
5. SQLite read-only schema/table/count inventory.
6. PCAP/PCAPNG capture and deterministic session metadata.
7. Video container/stream/duration inventory and bounded frame extraction.

No OCR, ASR, diarization, vision, or TTS model should be downloaded until its
fixed Pakistan-aware golden dataset, accuracy metrics, resource budget and
rollback plan are approved.
