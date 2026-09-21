# NexusAI Phase 2A Universal Ingress Acceptance

Date: 2026-07-24  
Host profile: Windows laptop, Intel Core i7-1260P, 15.71 GiB RAM, Docker Desktop CPU runtime  
Acceptance collection: `forensic-phase2a-acceptance-20260724`  
Production comparison collection: `records-demo-verified` (unchanged)

## Executive result

Phase 2A universal evidence registration and routing passed across all 20 declared
evidence families. Twenty-one source objects were used because the document family
was exercised with both PDF and DOCX.

| Measure | Result |
| --- | ---: |
| Knowledge Base entries | 21 |
| Evidence objects | 21 |
| KB asset/source links | 21 |
| Structured jobs completed | 7 / 7 |
| Structured rows accepted | 20 / 20 |
| Rejected rows | 0 |
| Duplicate rows | 0 |
| Failed jobs/evidence | 0 |
| Evidence warnings | 0 |
| Pending/manual evidence items | 13 |
| Pending items incorrectly sent to worker | 0 |

This acceptance proves registration, classification, source hashing, KB/evidence
linkage, deterministic routing for currently supported records, safe abstention for
unsupported adapters, and STT/TTS parent lineage. It does not claim that pending
OCR, Office extraction, spreadsheet, STT, video, capture, database, or archive
adapters are implemented.

## Deployed runtime

| Component | Image digest | State |
| --- | --- | --- |
| LocalAI | `sha256:0731ab05fa080fae9bc90ddbcc26f74fcb554389ee82aa0282e5373a27144c9b` | Healthy |
| Forensic API | `sha256:03a55612567e25fb04d9a8c1cec67bd2d35a7e4de30f6c67fd081e1582471bbe` | Healthy |
| Forensic worker | `sha256:d2ec7cf626d21b6be399017e87b660f5a679ef22962583afd91e4ff43c8f8d18` | Running |

Baseline models were unchanged: `qwen_qwen3-4b-instruct-2507` and
`qwen3-embedding-0.6b`. No model was downloaded or promoted.

## Evidence-family results

| Family/case | Bytes | SHA-256 | Evidence ID | Actual route | Final behavior |
| --- | ---: | --- | --- | --- | --- |
| Plain text notes | 575 | `74402bc45e75d15fce897e36c30b22a6913a2fa74433ce43abcb4b387b9f0bad` | `1dbad7bb-bc44-46ca-a996-659ccef0f0cd` | `kb_text_index` | Completed KB text |
| Access/security log | 274 | `92c7f9b2f8d9de752b4d681c8be8ff4d72f6210adbfb4963301388c0bb72bab5` | `06f62a5c-6893-4a86-98df-2b949ce8a516` | `forensic_records_worker` | 2/2 rows accepted as `access_log` |
| CDR | 581 | `c77abd4ca57bffe4839c9de4550f0533cb74831df03d754295931aac3eee7360` | `39e93d08-6fdd-41f5-a428-54cd09a5fdf1` | `forensic_records_worker` | 4/4 rows accepted |
| IPDR/network sessions | 403 | `8e6df5e956c766a07a343cfe24510c1e351173ff21feac54dc2cc59ce77ca376` | `351d3dac-f6ac-43fa-88b3-cd12df514a87` | `forensic_records_worker` | 2/2 rows accepted |
| ANPR sightings | 307 | `44b10ccabb8376123c53d7b35d38e3b5baecf71736760824a0d1b36fdcb6f8f7` | `07f9cf08-24ee-4464-a599-77a8fe99c2a7` | `forensic_records_worker` | 4/4 rows accepted |
| Subscriber identity | 203 | `465fe69d043f97668b9f93ab7718b3cafdd7e3c6991827ceb75eee9cd9a3ce3e` | `9c57b9d3-7ab1-4023-971d-af47421433b6` | `forensic_records_worker` | 2/2 rows accepted |
| Tower/location | 213 | `42247de0def10e98e39e9f1a564c90addd193d6c819b97022a162b9c942afebd` | `a181a955-588b-4526-b6af-99f549c8a519` | `forensic_records_worker` | 3/3 rows accepted |
| Financial transactions | 281 | `8cc5db5bc376fe4646172cb58ebcca3be2d496cb2a04ce3d8d288a603191b252` | `cc52254a-28f9-4416-89ff-11b3309187f7` | `forensic_records_worker` | 3/3 rows accepted |
| Generic tabular TSV | 126 | `2424a25cd80db4b968e389e9641f28f2d4a6fcdab09e940de9544f3dab3830a1` | `d33cbfec-a5ae-436c-867b-f17d83745da6` | `tabular_adapter_pending` | Registered; not queued |
| XLSX spreadsheet | 1,629 | `3835d2b550809646f4e3b10a9c5ed51ea0c87d3586eaf1d070c109de9d407aba` | `def57160-9c91-421a-84d5-e949c4eed4a8` | `tabular_adapter_pending` | Registered; not queued |
| PDF document | 613 | `e5ff358d05d1eccdec5d211477d7ba3bb5ff2ce0e60926afd68932e3604fe8da` | `a0d5e327-9b89-4291-a51f-cb5f502ffbec` | `document_extraction_pending` | Registered; not queued |
| DOCX document | 34,352 | `85b1ab53150a556f84391a222b7fb4dbf69aa80d1c4d7d7a7b54d5f283ea4865` | `2dac581f-e745-4462-a130-d714cca0a0c0` | `document_extraction_pending` | Registered; not queued |
| Image | 711 | `05bf15a7353c05eb0ae642b0e2551d623c5a0b1e6815929f5ef0b3c769386777` | `8e052b43-3b33-475f-9acb-568cbbab9201` | `image_ocr_vision_pending` | Registered; not queued |
| Audio | 4,044 | `9dc3f6c0a6c0b8b7ecbad30db415c10cff8e504db988145ad74afd0e927c9b4c` | `de205a1a-562b-43bf-af3d-393613b509c6` | `audio_stt_pending` | Registered; not queued |
| TTS artifact | 4,844 | `7f270cdbfd22484f287180c74f8e3a915cd9b8550703a6c2eb5bdebacacab5fd` | `0a586961-8398-43c4-94ee-20f8d799a8e0` | `tts_artifact_registry` | Registered with PDF parent |
| STT transcript | 92 | `a713b6c7d9b50b33b5e2dcea614946da3f228add932dceb5d77b3916635a20d2` | `f22a0b89-d890-49e2-aeeb-817a2f34bdb6` | `transcript_index_pending` | Registered with audio parent |
| Video | 272 | `9e8c893383b686dd7fe0b950c75b19f92a419e59457b195388c3c63d2c100365` | `8bf019a8-e6fc-461f-8ffa-12fa2c209f65` | `video_analysis_pending` | Registered; not queued |
| Network capture | 111 | `fc8415b80ba1024f45651e7c2bdbdc0ee0f079712fc1656405589b14f278813f` | `747f4552-36fe-459f-897d-1fec78c5aaae` | `capture_adapter_pending` | Registered; not queued |
| SQLite database | 8,192 | `14b5a8dc2330209aa29067a9fcd90d6b0db19133040b45c8cbdb0b40288f83e1` | `0b4cfad8-c1c1-440f-9fcd-775857bc8ffd` | `database_adapter_pending` | Registered; not queued |
| ZIP archive | 314 | `a0e2a6f02f366e227e26029f188429c9f3f579382f7417556defccc6337bf631` | `df7447ce-e010-4b6f-8158-a823bf77ee61` | `archive_inventory_pending` | Registered; not queued |
| Unknown binary | 20 | `54e5b231d8dbec164eb03a72940467148717305e0696531709a85e4bee772c16` | `62d3c364-db3f-4a6a-b198-5ff03f17d6c2` | `manual_review` | Registered; not queued |

## Derived-artifact lineage

- TTS evidence `0a586961-8398-43c4-94ee-20f8d799a8e0` references PDF parent
  `a0d5e327-9b89-4291-a51f-cb5f502ffbec` in both evidence metadata and KB routing
  decision.
- STT evidence `f22a0b89-d890-49e2-aeeb-817a2f34bdb6` references audio parent
  `de205a1a-562b-43bf-af3d-393613b509c6` in both locations.
- Both derivatives have their own evidence ID, version ID, source hash, raw spool
  reference and KB source entry.

## Defects found and closed during live validation

1. PostgreSQL advisory locks originally used NUL-delimited text; PostgreSQL text
   rejected the NUL. The lock key is now an unambiguous JSON tuple.
2. The worker required legacy CDR columns while classification accepted modern
   aliases. Modern `call_time`, source/target, duration and area aliases now pass.
3. Pretty JSON was initially treated like a single CSV header. Top-level JSON
   fields are now decoded from the whole document.
4. Arbitrary binary payloads could be interpreted as CSV headers; embedded NUL
   bytes then caused PostgreSQL JSONB SQLSTATE `22P05`. Header inspection is now
   allowlisted to worker-readable formats and rejects invalid UTF-8, NUL,
   excessive columns and overlong names.
5. The ready access-log fixture uses `client_ip`; API and worker aliases now agree
   and the fixture is processed as `access_log` rather than generic data.

Focused verification after the final code changes:

- Python: 12 tests passed in 0.194 seconds.
- Go forensic package: passed in 5.512 seconds.
- Live clean matrix: all counts above reconciled through LocalAI, the forensic
  API, NATS, the worker, PostgreSQL and KB storage.

## Build incident and recovery

The first approved LocalAI build completed in 671.7 seconds. A second build for
multipart MIME preservation appeared terminated at the client but remained in
BuildKit as running ref `khk58bwg259mz3ck70wsdjidc`, with zero discovered and
completed steps since 05:54. That exact orphan record was removed and the running
build-record count returned to zero. No image or cache was pruned. BuildKit retains
20.21 GB of cache, including 11.23 GB marked reclaimable; disk pressure is not a
current constraint.

Multipart MIME preservation remains tested in source but is not in the running
LocalAI image. The deployed forensic API safely uses bounded server-side content
sniffing when the forwarded declaration is `application/octet-stream`, as proven
by the clean matrix.

## Safety and governance statement

- `records-demo-verified` was not changed and remains at 9,250 structured rows.
- The clean matrix is isolated under tenant `default` and its dedicated collection.
- The earlier `forensic-ingress-audit-20260724` collection retains failed-before-fix
  attempts as an audit trail and is not used as the acceptance baseline.
- No database backfill, schema migration, model download, Git staging, commit,
  push, pull request or GitHub publication occurred.
- Production reconciliation of four KB-only entries remains a separate explicit
  database-write approval gate.

## Recommended next phase

Proceed to Phase 2B by replacing the small ingress-only media samples with fixed
golden fixture packs containing expected extraction outputs and provenance
locators. Implement the first bounded deterministic media-metadata slice before
downloading OCR/STT/vision models: image dimensions/hash, WAV duration/sample
rate/channels, video container/duration/streams, PDF page count, archive inventory,
SQLite read-only inventory and PCAP session metadata. Promote no pending route to
operational until its golden accuracy, safety, resource and rollback gates pass.
