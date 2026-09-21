# NexusAI Phase 2B.1 PCAP and PCAPNG Inventory Acceptance

Date: 2026-07-27  
Deployment profile: Pakistan (`PK`, `Asia/Karachi`)  
Scope: deterministic packet-capture technical inventory without payload decoding

## 1. Objective

Add bounded structural inventory for classic PCAP and PCAPNG during evidence
registration. Validate capture structure, packet boundaries, timestamps,
interfaces, lengths, and risk indicators without reading packet payload bytes,
extracting endpoints, interpreting protocols, reconstructing sessions, or
retaining names, addresses, filters, name-resolution values, or secrets.

## 2. Assumptions and capacity verified

- The previous API was SQLite inventory v1.3 at
  `sha256:9dc1ec79b34aa69913230f2d76e7e4b89c1c1795a7031cad146f381eb8032772`.
- LocalAI, worker, PostgreSQL, and NATS were running and healthy.
- Pre-build capacity was 4.34 GiB free physical memory and 1,780.75 GiB free
  disk.
- The API remains a `CGO_ENABLED=0` static binary with no packet-processing
  library or new dependency.
- `records-demo-verified` remained a protected baseline.
- All acceptance captures were synthetic; no real network traffic or real CDR
  data was inspected.

## 3. Files changed

- `api/forensic_records/media_metadata.go`: extractor v1.4.0 and capture dispatch.
- `api/forensic_records/media_metadata_capture.go`: bounded PCAP/PCAPNG parser.
- `api/forensic_records/capture_inventory_ginkgo_test.go`: eight positive,
  endian, multi-section, sensitive-block, corrupt, and limit cases.
- `api/forensic_records/file_inventory_ginkgo_test.go` and
  `sqlite_inventory_ginkgo_test.go`: current extractor version assertions.
- `api/forensic_records/evidence_classification.go`: capture modality accepted
  as an explicit declared modality.
- `configuration/forensic_modality_evaluation_matrix.json`: capture support moved
  from planned to foundation.
- `docs/content/features/forensic-intelligence-phase1.md`: behavior and limits.
- This report, the team-lead brief, and `NEXUSAI_CONTINUATION.md`.

## 4. Schema, API, and configuration impact

No schema migration, backfill, endpoint, auth/RLS, Compose, dependency, or
environment change occurred. Existing `media_metadata` now contains capture
inventory from extractor 1.4.0. The processing route remains
`capture_adapter_pending`, `queue_records=false`; captures do not reach NATS or
the records worker.

## 5. Automated tests

- Final `go test ./api/forensic_records -count=1`: passed in 29.418 seconds.
- Package suite total: 77 Ginkgo specs after adding eight capture cases.
- `go vet ./api/forensic_records`: passed.
- Modality matrix JSON parsed successfully.

Coverage includes little/big-endian PCAP; micro/nanosecond precision; extension
disagreement; bad timestamp fractions, snapshot lengths and truncation; PCAPNG
multi-interface resolution, simple/enhanced/obsolete packets, concatenated mixed-
endian sections, decryption-secrets abstention, mismatched trailers, missing
interfaces, oversized blocks, and bounded interface options.

## 6. Live runtime and deployment

- API-only Docker build/recreate: 46.5 seconds.
- Go compile step: 34.7 seconds.
- Deployed image:
  `sha256:bff1fd08162918504ba3d503b03ce1ef964c7165bdedeaf4a7de8673e43d68f5`.
- API creation: approximately 2026-07-27 05:32:55 UTC.
- API restart count: 0.
- LocalAI remained unchanged and healthy at
  `sha256:0731ab05fa080fae9bc90ddbcc26f74fcb554389ee82aa0282e5373a27144c9b`.
- Worker was not rebuilt. PostgreSQL and NATS remained healthy.
- API logs show exactly three expected capture registrations and no errors.

## 7. Dataset and evidence counts

Collection: `forensic-packet-capture-acceptance-20260727`  
Case: `PK-SYNTHETIC-CAPTURE-20260727`

| Measure | Result |
| --- | ---: |
| Synthetic captures | 3 |
| Total source bytes | 522 |
| Evidence items | 3 |
| KB assets | 3 |
| LocalAI KB entries | 3 |
| Records-worker jobs | 0, expected |
| Failures/errors | 0 |
| Warning-bearing evidence | 1 secret-marker PCAPNG |

Sources:

- Classic PCAP, 158 bytes, SHA-256
  `19dda44a43624507083c10baad5c8d6602bbf90f2ab0fdc0d6e93c91cfb1e560`.
- Multi-interface PCAPNG, 272 bytes, SHA-256
  `9ae210826d012ddf9544e19da21b4abc06bc4488766cfd421594e24c4ba5dd78`.
- Decryption-secrets marker PCAPNG, 92 bytes, SHA-256
  `f86edcf67ee6c7d48f20106d3a91e4eb339ae1faa637112dfe59f4ec919d8283`.

## 8. Models

No model was called, downloaded, changed, or benchmarked. Visible IDs remain:

- `qwen3-embedding-0.6b`
- `qwen_qwen3-4b-instruct-2507`

## 9. Accuracy and agreement

- Classic PCAP: 2 packets, Ethernet link type, 102 captured bytes, 124 original
  bytes, one truncated packet, and both UTC timestamp bounds matched exactly.
- Multi-interface PCAPNG: one section, two interfaces, six blocks, three packets,
  84 captured bytes, 104 original bytes, one truncation, one timestamp-less
  Simple Packet, and mixed micro/nanosecond bounds matched exactly.
- Secret-marker PCAPNG: one section, one interface, three blocks, zero packets,
  one decryption-secrets block, and no retained secret value matched exactly.
- 3/3 evidence items have one KB asset and one KB entry.
- Clean PCAP and PCAPNG returned zero warnings.
- The secret-marker warning matched the immediate response, evidence item, and KB
  asset `quality_report`.
- All three report `packet_payload_bytes_read=0` and `protocols_decoded=false`.

No endpoint, DNS/HTTP, protocol, flow, session, anomaly, or semantic accuracy is
claimed.

## 10. Latency and memory snapshots

- API health: 275.26 ms.
- Three-item catalog: 228.08 ms.
- API: 7.18 MiB.
- Worker: 47.02 MiB.
- PostgreSQL: 52.78 MiB.
- NATS: 9.797 MiB.
- LocalAI: 675.8 MiB.
- Post-run free physical memory: 3.79 GiB.

These are single local snapshots, not percentile benchmarks.

## 11. Failures, fallbacks, and unresolved limits

The first compile found two unused imports left after fixture cleanup. They were
removed; the package then passed. There was no live parser or deployment failure.

Pending capabilities include Ethernet/IP/TCP/UDP/DNS parsing, endpoint and flow
normalization, session reconstruction, PCAPNG name-resolution use, long-capture
streaming benchmarks, common link-type expansion, packet-level provenance,
protocol queries, and EVTX. Capture inventory intentionally does not make these
claims.

## 12. Security, tenant isolation, and provenance

Limits are 1,000,000 packets, 1,250,000 blocks, 1,024 sections, 10,000
interfaces, 4,096 interfaces per section, 64 MiB of header/technical-option
reads, 16 MiB per captured packet, 32 MiB per block, and 64 KiB per interface
option area. Leading/trailing lengths, byte order, references, alignment, file
bounds, and timestamp fields fail closed.

Packet payloads are skipped by offset. Interface names/addresses, filters,
name-resolution values, and decryption-secret data are not retained. Existing
tenant, collection, case, evidence/version, SHA-256, KB, Pakistan jurisdiction/
timezone, warning, audit, and extractor lineage remains intact.

## 13. Git state

Dirty worktree remains unstaged, uncommitted, and unpushed. No branch, commit,
pull request, GitHub action, or external publication occurred.

## 14. Rollback and recovery

Pre-capture SQLite v1.3 is preserved as
`nexusai-forensic-records-api:rollback-20260727-sqlite-v1.3`, resolving to
`sha256:9dc1ec79b34aa69913230f2d76e7e4b89c1c1795a7031cad146f381eb8032772`.
Current v1.4 is `sha256:bff1fd08162918504ba3d503b03ce1ef964c7165bdedeaf4a7de8673e43d68f5`.
No data rollback is needed because there was no schema change.

## 15. Next action and approval gates

Next bounded slice: deterministic MP3 and ISO Base Media/MP4/MOV container
metadata, followed by video-stream inventory. Then expand Pakistan structured
goldens and deeper typed adapters. Full network-session extraction belongs to a
separate capture adapter after real versioned fixtures and packet-level lineage.

Model changes/downloads, sensitive real-data ingest, migrations/backfills, heavy
LocalAI rebuild, destructive evidence cleanup, and Git publication remain
separate approval gates.
