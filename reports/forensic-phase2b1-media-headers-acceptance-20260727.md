# NexusAI Phase 2B.1 Deterministic Media Headers Acceptance

Date: 2026-07-27  
Scope: bounded BMP, TIFF/EXIF, WebP, FLAC, and hardened RIFF/WAVE technical
metadata for Pakistan-hosted forensic evidence registration

## Executive result

Deterministic media extractor `1.1.0` is deployed in the forensic API. It adds
model-free BMP, TIFF, JPEG EXIF-orientation, WebP, and FLAC header metadata while
hardening WAV chunk boundaries. A neutral isolated collection passed four live
synthetic fixtures through forensic upload, LocalAI KB mirroring, evidence
registration, PostgreSQL persistence, and evidence-catalog retrieval.

The phase did not decode pixels, execute codecs, transcribe audio, run OCR, copy
embedded EXIF/XMP payloads, ingest real evidence, change a schema, download a
model, or rebuild LocalAI. Nothing was staged, committed, pushed, or published.

## 1. Objective

Close the next Phase 2B.1 vertical slice with bounded, auditable technical
metadata for additional real-world image and lossless-audio containers. Preserve
the existing OCR/STT pending routes so header success cannot be mistaken for
semantic extraction.

## 2. Assumptions verified

- The living checkpoint and repository build/testing guidance were read first.
- Pre-build resources were 4.45 GiB free RAM and 1,781.04 GiB free disk.
- The previous API image was
  `sha256:d32e5118d18849fd4aae024944e547573c9643189a7b5bb2a9f72cbecde64980`.
- The laptop restart had left PostgreSQL, NATS, API, and worker containers exited;
  their volumes and images were intact. LocalAI remained healthy.
- The implementation uses only the Go standard library and existing project
  code; no dependency download or installation was added.
- Binary layout and security checks were aligned with the WebP RIFF container
  specification, TIFF 6 tag definitions, and RFC 9639 FLAC STREAMINFO contract.

## 3. Files changed

- `api/forensic_records/media_metadata.go`
- `api/forensic_records/media_metadata_image.go` (new)
- `api/forensic_records/media_metadata_ginkgo_test.go`
- `docs/content/features/forensic-intelligence-phase1.md`
- `reports/forensic-phase2b1-media-headers-acceptance-20260727.md` (new)
- `NEXUSAI_CONTINUATION.md`

Unrelated pre-existing worktree changes were preserved.

## 4. Schema, API, and configuration changes

- No database schema, migration, endpoint, authentication, RLS, environment, or
  Compose contract changed.
- Existing `evidence_items.media_metadata` JSONB stores extractor `1.1.0` output.
- Supported deterministic image extensions are now PNG, JPEG, GIF, BMP, TIFF,
  and WebP. FLAC joins WAV for deterministic audio headers.
- Filename extension is compared with detected image signature. Mismatch keeps
  evidence registered with detected metadata and adds a warning.

## 5. Implemented contracts and limits

| Format | Deterministic fields | Bounded validation |
| --- | --- | --- |
| PNG/JPEG/GIF | detected format, width, height, pixel count | 4 MiB header reader |
| JPEG EXIF | orientation code/label and display dimensions | APP1 only; 4 MiB and 512-segment ceilings |
| BMP | dimensions, bit depth, compression code, top-down rows | file/DIB/pixel offsets checked against file size |
| TIFF | dimensions, orientation, display dimensions | primary IFD only; 512 entries; offsets/counts checked |
| WebP | canvas, animation and alpha/EXIF/XMP/ICC presence | RIFF boundary, chunk order/size/reserved bits, 512 chunks, 4 MiB header position |
| WAV | format, channels, rate, byte rate, bit depth, data bytes, duration | every padded chunk checked against RIFF boundary; 512 chunks |
| FLAC | block/frame bounds, channels, rate, bit depth, total samples, duration | native signature; first 34-byte STREAMINFO; RFC block/rate/depth validation |

Images above 100,000,000 pixels receive `decode_review_required=true`; the
metadata pass still does not allocate a pixel canvas. EXIF GPS, camera identity,
timestamps, comments, XMP bodies, and ICC bodies are deliberately not retained.

## 6. Tests and exact results

- `go test ./api/forensic_records -count=1`: passed in 9.741 seconds.
- `go vet ./api/forensic_records`: passed with no findings.
- New positive cases cover BMP, TIFF orientation/display geometry, WebP canvas,
  JPEG EXIF orientation, FLAC STREAMINFO, and mislabeled PNG-as-JPEG behavior.
- New negative cases cover BMP size overflow, a 513-entry TIFF IFD bomb, WebP
  chunk overflow, zero-rate FLAC, and WAV chunk overflow. Existing corrupt-media
  registration coverage remains active.
- An earlier focused command used an incompatible Windows Ginkgo flag form and
  exited before running specs; the complete package run above supersedes it.

## 7. Live-runtime checks

| Component | Deployed image | Final state |
| --- | --- | --- |
| LocalAI | `sha256:0731ab05fa080fae9bc90ddbcc26f74fcb554389ee82aa0282e5373a27144c9b` | healthy; unchanged |
| Forensic API | `sha256:43489c192a7d24982ad8361f3aa84d048b2aaf9c379869c8a866ce7b11974aba` | running; `/healthz` OK |
| Worker | `sha256:faeb65590b1f6a61e7458a9c73c763b5c82dc772df29f20fd8843691c9615268` | running; unchanged |
| PostgreSQL | existing persistent service | healthy |
| NATS | existing persistent service | healthy |

The API-only image build completed in 43.5 seconds. Both existing LocalAI models
remained visible after deployment.

## 8. Dataset and evidence counts

Live collection: `forensic-media-headers-acceptance-20260727`  
Case: `PK-SYNTH-MEDIA-20260727`

| Measure | Result |
| --- | ---: |
| Synthetic source fixtures | 4 |
| LocalAI KB entries | 4 |
| Evidence items | 4 |
| KB assets | 4 |
| Evidence warnings | 0 |
| Evidence errors/failures | 0 |
| Records-worker jobs | 0, expected for media pending routes |

Persisted expectations:

- BMP: 7 x 5, 24-bit, uncompressed, 35 pixels.
- TIFF: 9 x 6, orientation 6 (`right_top`), display geometry 6 x 9.
- WebP: 11 x 8, non-animated, no declared embedded metadata, 88 pixels.
- FLAC: stereo, 48 kHz, 24-bit, 96,000 samples, 2,000 ms.
- All four: `PK`, `Asia/Karachi`, mirrored KB asset, registered pending route,
  extractor `1.1.0`, zero warnings/errors.

Production baseline `records-demo-verified` remains 9,250 total and accepted
rows, with zero rejected or duplicate rows.

## 9. Models, accuracy, and retrieval metrics

- No model was called, changed, downloaded, or benchmarked in this slice.
- Existing visible model IDs remained `qwen3-embedding-0.6b` and
  `qwen_qwen3-4b-instruct-2507`; their revisions and parameters were unchanged.
- Deterministic live-field agreement was 4/4 fixtures. Unit-test rejection was
  5/5 targeted malformed/resource-amplifying headers.
- OCR, object detection, ASR, diarization, language ID, and TTS quality metrics
  are not applicable and are not claimed.
- KB mirroring completeness was 4/4; evidence-to-asset completeness was 4/4.

## 10. Latency and memory metrics

- API health request: 226.05 ms from Windows PowerShell.
- Evidence catalog request for four items: 30.94 ms.
- Post-acceptance free RAM: 3.22 GiB.
- Container memory snapshots: API 7.555 MiB; worker 51.94 MiB; PostgreSQL
  51.56 MiB; NATS 8.23 MiB; LocalAI 644.5 MiB.

These are single local snapshots, not percentile or load-test claims.

## 11. Failures, fallbacks, and unresolved blockers

- The first acceptance client attempt failed before any HTTP request because
  Windows PowerShell had not loaded `System.Net.Http`. Loading the standard
  assembly fixed the client; no partial collection/evidence state was created.
- The existing server-side content sniffing fallback remains active.
- Explicit LocalAI forwarding of caller-provided `jurisdiction` and
  `source_timezone` is source-complete but still absent from the running LocalAI
  image. API defaults make the deployed Pakistan path correct; deploying those
  explicit overrides still requires the separately approved/diagnosed LocalAI
  rebuild gate.
- Full media validity and semantic content are intentionally outside a bounded
  header inventory. Header success is not a codec-decoding guarantee.

## 12. Security, tenant isolation, and provenance impact

- No codec, executable payload, archive member, pixel canvas, model, or external
  service is invoked by these parsers.
- Lengths, offsets, reserved bits, entry counts, and chunk counts are checked
  before allocation or seeking.
- Embedded sensitive metadata is reduced to orientation or presence flags; GPS
  and free text are not persisted.
- Tenant, collection, immutable SHA-256, evidence/version ID, case, KB entry,
  jurisdiction, timezone, warnings, and extractor version remain queryable.
- Tenant/RLS behavior is unchanged. All live fixtures used tenant `default` and
  a new isolated neutral collection.

## 13. Git state

The worktree remains dirty, unstaged, uncommitted, and unpushed. No GitHub action,
branch, pull request, or external publication was performed.

## 14. Rollback and recovery

- API rollback tag `nexusai-forensic-records-api:rollback-20260727-media-v1.0`
  points to
  `sha256:d32e5118d18849fd4aae024944e547573c9643189a7b5bb2a9f72cbecde64980`.
- Current API image is
  `sha256:43489c192a7d24982ad8361f3aa84d048b2aaf9c379869c8a866ce7b11974aba`.
- Existing persistent volumes and all earlier rollback images remain intact.

## 15. Next action and approval required

Continue model-free Phase 2B.1 in this order:

1. PDF page/encryption/attachment inventory.
2. ZIP/TAR inventory with member, traversal, link, expanded-size, ratio, and
   nesting limits.
3. SQLite read-only schema/table/count inventory.
4. PCAP/PCAPNG deterministic capture/session inventory.
5. MP3/MP4-family and video container/stream/duration inventory.
6. Pakistan ANPR, financial-transaction, and access-log messy golden expansion.

These bounded source/test changes and the small forensic API-only deployment are
the next reversible work. Model downloads, sensitive real-data ingest, database
backfill/migrations, a heavy LocalAI rebuild, and Git publication remain separate
explicit approval gates.
