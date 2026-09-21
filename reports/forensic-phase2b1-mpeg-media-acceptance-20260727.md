# NexusAI Phase 2B.1 MPEG Media Inventory Acceptance

Date: 2026-07-27  
Collection: `forensic-mpeg-container-acceptance-20260727`  
Case: `PK-SYNTHETIC-MPEG-20260727`

## 1. Objective

Add deterministic, bounded MP3 plus ISO Base Media technical inventory during
evidence registration without decoding compressed audio/video, reading media
payloads or ID3 values, extracting frames, or claiming STT/video understanding.

## 2. Assumptions verified

- The previous deployed API image was capture inventory v1.4 at
  `sha256:bff1fd08162918504ba3d503b03ce1ef964c7165bdedeaf4a7de8673e43d68f5`.
- The API restart count was zero and PostgreSQL/NATS/worker were healthy.
- LocalAI remained the healthy `nexusai-localai:universal-20260724` image.
- The existing `media_metadata` JSON field can carry the new extractor output;
  no schema migration is required.

## 3. Files changed

- `api/forensic_records/media_metadata_mpeg.go`: MP3 and ISO Base Media parser.
- `api/forensic_records/media_metadata_mpeg_ginkgo_test.go`: eleven focused specs.
- `api/forensic_records/media_metadata.go`: v1.5 dispatch and version.
- Existing extractor-version assertions updated to v1.5.
- Evaluation matrix, product documentation, team-lead brief, acceptance report,
  and continuation checkpoint updated.

## 4. Schema, API, and configuration changes

No endpoint, database schema, migration, authentication, RLS, Compose,
dependency, environment, model, or queue contract changed. MP3 remains on
`audio_stt_pending`; MP4/MOV family evidence remains on `video_analysis_pending`.

## 5. Tests and exact results

- Focused MPEG Ginkgo run: 11/11 passed.
- Final `go test -v ./api/forensic_records -count=1`: 88/88 Ginkgo specs plus
  legacy package tests passed; package time 6.427 seconds.
- `go vet ./api/forensic_records`: passed.
- `configuration/forensic_modality_evaluation_matrix.json`: parsed successfully;
  audio and video adapters resolve to v1.5 inventory contracts.
- Coverage includes ID3v2/ID3v1 boundaries, CBR/VBR MP3, MP4 video/audio tracks,
  M4A, rotation, QuickTime-major-brand disagreement, free-format MP3,
  truncation, oversized ID3, bad box length, excessive sample descriptions,
  and MP3 content disguised as MP4.

## 6. Live runtime checks

- API-only build and recreation completed in 46.7 seconds.
- Deployed API image:
  `sha256:f24d90c5c7941165d137492420d486fd82fbb0c95af62c173f15bb0ec1a6e772`.
- API health returned `{"status":"ok"}`; API restart count remained zero.
- LocalAI readiness returned HTTP 200; PostgreSQL and NATS remained healthy;
  the worker and LocalAI containers were not rebuilt or recreated.
- API logs contain startup plus exactly three expected registration messages and
  no error for this acceptance run.

## 7. Dataset and evidence counts

All fixtures are synthetic and contain no real person, call, camera, or case data.

| Measure | Result |
| --- | ---: |
| Source files | 3 |
| Source bytes | 2,595 |
| Evidence items | 3 |
| KB assets | 3 |
| KB entries | 3 |
| Records jobs | 0 |
| Accepted/rejected/duplicate rows | 0/0/0 |
| Failed jobs/evidence | 0/0 |
| Warning-bearing evidence | 1 intentional extension/brand mismatch |

Sources:

- MP3, 1,515 bytes, SHA-256
  `9788941789c6e909e648496edbaabc0597c77fd3df3ac92ba7081e54eb0aea91`.
- MP4, 660 bytes, SHA-256
  `a0dff7f325391a3f5214359d76784065973a6e4b2c95058507d7e5cc9ea86372`.
- QuickTime-major-brand file labelled `.mp4`, 420 bytes, SHA-256
  `bdf411bcd2c71616754082b746ca717c9a1369ee0bb2d6ea814c592868a33518`.

## 8. Models, revisions, and parameters

No model was invoked, downloaded, reconfigured, or promoted. Installed IDs
remain `qwen_qwen3-4b-instruct-2507` and `qwen3-embedding-0.6b`.

## 9. Accuracy and retrieval results

- MP3: three MPEG-1 Layer III frames, 44.1 kHz stereo, 3,456 samples/channel,
  78 ms, 128-160 kbps VBR, ID3v2.4 plus ID3v1 presence, 25 technical bytes
  inspected, zero compressed payload bytes read, and no tag value retained.
- MP4: two tracks; `avc1` video at 1280x720 rotated 90 degrees and `mp4a`
  stereo audio at 48 kHz/16 bits; both tracks and movie report 5,000 ms; 32
  `mdat` bytes accounted but zero media payload bytes read.
- QuickTime marker: one `avc1` track and the expected `.mp4`/QuickTime warning.
- The warning matched the immediate upload response, evidence catalog/detail,
  collection status, and KB asset `quality_report.warnings` exactly.
- The protected `records-demo-verified` baseline remains 9,250/9,250 accepted,
  four completed jobs, zero rejected/duplicate rows, and zero failures.

## 10. Latency and memory snapshots

- API health: 287.04 ms.
- Three-item evidence catalog: 66.90 ms.
- API: 7.203 MiB; worker: 47.02 MiB; PostgreSQL: 53.7 MiB; NATS: 9.18 MiB.
- Values are single post-acceptance snapshots, not load-test percentiles.

## 11. Failures, fallbacks, and unresolved blockers

No product or acceptance failure remains. Development-only issues were resolved:
the managed sandbox required a workspace-local Go cache; one focused Ginkgo flag
needed PowerShell-safe quoting; and one read-only evidence-detail URL needed
`${id}` interpolation. None changed service or evidence state.

Pending capabilities include real-codec goldens, free-format MP3, APE/trailing
tag variants, AAC elementary streams, OGG/Opus/WMA/AMR, Matroska/WebM, AVI,
transport streams, fragmented/edited ISO media, frame extraction, embedded
audio decoding, STT, diarization, objects, plates, faces, and video timelines.

## 12. Security, tenant isolation, and provenance

- MP3 caps: 1,000,000 frames, 4,096 bytes/frame, 8 MiB aggregate header reads.
- ISO media caps: 100,000 boxes, depth 12, 4,096 tracks, 256 sample entries per
  track, 64 brands, and 8 MiB aggregate header reads.
- ID3 values, compressed audio, `mdat`, user metadata, frames, and samples are
  not read or retained. Invalid boundaries and amplification counts fail closed.
- Existing tenant, collection, case, evidence/version, SHA-256, Pakistan
  jurisdiction/timezone, KB, and audit lineage remained intact.

## 13. Git state

The working tree remains unstaged, uncommitted, and unpushed. No GitHub or
external publication action occurred; unrelated user edits were preserved.

## 14. Rollback or recovery point

Capture v1.4 is preserved as
`nexusai-forensic-records-api:rollback-20260727-capture-v1.4` at
`sha256:bff1fd08162918504ba3d503b03ce1ef964c7165bdedeaf4a7de8673e43d68f5`.
No database/data rollback is needed because this slice added no migration.

## 15. Next action and approval gates

Next bounded slice: expand Pakistan structured goldens, beginning with messy
ANPR and PKR financial-transaction CSV/JSONL variants, followed by subscriber,
tower and access/security logs. Require exact row accounting, raw-value
preservation, timezone/currency precision, visible rejection, and source-row
lineage before widening formats.

Separate approval remains required for real sensitive evidence, downloads or
new OCR/STT/vision models, migrations/backfills, heavy LocalAI builds,
policy-sensitive identity features, destructive cleanup, and Git publication.
