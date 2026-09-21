# NexusAI team-lead brief and Phase 4 demonstration

Date: 2026-07-30  
Recommended duration: 12-15 minutes

## Say this first

> Over the last four days I turned the LocalAI Knowledge Base into a governed
> forensic-intelligence pipeline. Evidence is hash-preserved and tenant-scoped,
> structured data is normalized by deterministic adapters, exact questions are
> answered by SQL with source-row citations, and models are limited to planning
> and explaining bounded evidence. Phase 3 runtime security and durable queueing
> are live. Phase 4 now processes the current structured families and has a
> simpler analyst UI. OCR, STT, TTS, image and video understanding are registered
> but deliberately remain separate model-promotion phases.

## Architecture in easy words

```text
File / Knowledge Base upload
  -> preserve original bytes, hash, version and custody history
  -> identify type and safe processing route
  -> deterministic adapter validates and normalizes supported rows
  -> PostgreSQL stores exact facts; rejected/duplicate rows remain auditable
  -> NATS JetStream provides durable delivery, retry, DLQ and reprocessing
  -> analyst asks a normal question in the UI
  -> planner selects exact SQL, cited KB retrieval, or both
  -> answer shows facts, evidence, limitations and next actions
```

The one sentence to emphasize is: **the language model explains evidence; it
does not invent or calculate forensic facts.**

## What is working

- Phase 3: authentication, non-owner RLS, tenant isolation, immutable custody,
  content-addressed retention, retry/DLQ, redelivery and linked reprocessing.
- Phase 4 structured adapters: CDR, IPDR, ANPR, subscriber, tower/location,
  financial transactions, access/security logs and generic tabular data.
- Formats: CSV, JSON, JSONL/NDJSON, TSV and bounded read-only multi-sheet XLSX.
- Pakistan defaults: `Asia/Karachi`, PK jurisdiction, phone/identity/IBAN/plate
  shapes, Urdu/English text preservation and provider-style messy schemas.
- UI: Ask and analyze, Review evidence, Manage data.
- Models currently loaded: Qwen3 4B Instruct for planning/explanation and Qwen3
  Embedding 0.6B for multilingual retrieval. Exact SQL remains authoritative.

## Before the demo

First close the tested capability fix:

```powershell
Set-Location "C:\Users\sheik\Workspace\Office\Projects\NexusAI"

powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\repair_forensic_phase4_api_gate13b.ps1

powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\smoke_forensic_phase4_structured_gate12.ps1
```

Do not continue unless both lines say PASS. Do not upload
`C:\Users\sheik\Downloads\923461678183.csv`; the live demo collection is
synthetic and named `nexusai-structured-demo-v2-20260730`.

## Live UI demonstration

1. Open `http://localhost:8080/app/records`.
2. In **Case Collection**, enter
   `nexusai-structured-demo-v2-20260730` and click **Refresh**.
3. Point out the accepted, duplicate, evidence and family counters.
4. Ask `which files were ingested?` and click **Analyze**.
5. Ask `correlate 35678901234567 across record families`.
6. Open **Review evidence** and show completed processing, hashes/provenance,
   family capability labels and the XLSX/TSV evidence.
7. Open **Manage data** only to explain that uploads now go to the governed
   evidence pipeline and the legacy batch view; do not upload real data.

Expected synthetic acceptance:

| Files | Rows | Accepted unique | Duplicates | Rejected |
| ---: | ---: | ---: | ---: | ---: |
| 9 | 68 | 39 | 8 | 21 |

Explain that rejected rows are intentional malformed test cases, not lost data.

## Technical proof points

- Python worker/adapter suite: 51/51 passed.
- Focused LocalAI forwarding suite: passed.
- Full forensic API package: passed after each repair.
- React production build: passed, 656 modules.
- Complete Phase 4 deployment: API 34.3 s; LocalAI/UI 474.9 s; health ready;
  rollback images and named volumes preserved.
- Gate 12 reached all nine golden accounting assertions; the two commands above
  deploy and verify the last capability fix plus cross-family/source-audit SQL.

## Current model position

| Role | Active model | What it may do |
| --- | --- | --- |
| Chat/planning | `qwen_qwen3-4b-instruct-2507` Q8_0 | Interpret a question, select tools, explain cited results |
| Retrieval | `qwen3-embedding-0.6b` Q8_0, 1024d | Retrieve multilingual Knowledge Base evidence |
| Exact analytics | No LLM | Adapters and parameterized PostgreSQL queries |

Stored three-round chat benchmark: 6/6 checks passed, average 8.57 s, p50
4.78 s, p95 12.84 s on the CPU laptop. This is a development baseline, not a
production SLA.

## What comes next

1. Close Phase 4 with the two PASS commands and preserve their marker/report.
2. Phase 5A documents/OCR: native PDF/Office extraction, Urdu/English OCR,
   tables/layout, page citations and Tesseract vs PaddleOCR evaluation.
3. Phase 5B audio/STT: metadata, Whisper tiny/small Urdu-English benchmark,
   timestamps, diarization and transcript indexing.
4. Phase 5C images/ANPR vision: EXIF, OCR/detectors, adverse-condition plate
   tests, grounded boxes and legally controlled face functions.
5. Phase 5D video/captures/databases/archives: bounded extraction, timelines,
   session derivation, read-only inventories and child-evidence lineage.
6. Phase 6 retrieval/model promotion: fixed Pakistan gold sets, accuracy,
   abstention, latency/RAM/license checks, one model change at a time.
7. Phase 7 production hardening: RBAC, retention/object lock, monitoring,
   performance/concurrency and disaster recovery.

## Likely questions

**Why are some formats shown as pending?**  Registration and deep analysis are
different. We preserve every format safely, but do not claim OCR/STT/video
results until its adapter and model pass fixed accuracy and resource gates.

**Does the real CSV work?**  Yes. The supplied real-format CDR was audited
offline: 3,931 rows, 3,634 unique normalized, 297 exact duplicates, zero
rejected. It was not uploaded to retained storage.

**Why keep duplicates and rejects?**  Evidence must not be rewritten to look
clean. We preserve source truth, prevent duplicate inflation, and give rejected
rows a reason and locator for review.

**Has anything been pushed to GitHub?**  No. The worktree is still unstaged,
uncommitted and unpushed so the team can review scope and decide the publication
workflow.
