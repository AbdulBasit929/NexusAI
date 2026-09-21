# NexusAI Phase 4 structured runtime status

Date: 2026-07-30  
Scope: retained local runtime, synthetic/de-identified evidence only

## Outcome

Phase 4 structured ingestion is operational across CDR, IPDR, ANPR,
transactions, subscribers, tower/location, access/security logs, TSV and
bounded read-only multi-sheet XLSX. The rebuilt React UI and accountable
LocalAI-to-sidecar proxy are deployed. Nine synthetic files completed their
golden accounting before the capability assertion:

| Measure | Result |
| --- | ---: |
| Synthetic files | 9 |
| Operational paths | 9 |
| Source rows | 68 |
| Accepted unique rows | 39 |
| Exact duplicate rows | 8 |
| Rejected rows | 21 |
| Real evidence uploaded | No |

The rejects are intentional malformed-row goldens with reason codes and source
locators. The incomplete first demo collection is preserved as audit history;
the clean acceptance scope is `nexusai-structured-demo-v2-20260730`.

One final deployment closure remains: the tested capability query must be
rebuilt into the forensic API and Gate 12 rerun. Codex could not execute that
last privileged Docker command because the product reported its privileged-use
limit. This is an execution blocker, not a code/test failure.

## Working now

- Authenticated/non-owner runtime, tenant RLS, content-addressed evidence,
  versions, custody, explicit-ack JetStream, retry/DLQ, redelivery and linked
  reprocessing from Phase 3.
- Exact structured adapters for CDR, IPDR, ANPR, subscriber, tower/location,
  financial transaction, access/security log and generic records.
- CSV, JSON/JSONL/NDJSON, TSV and bounded read-only XLSX ingestion. XLSX maps
  each sheet independently and never executes formulas, macros or links.
- Exact duplicates and rejected rows do not inflate accepted unique facts.
- Deterministic SQL templates and cited cross-family correlation.
- React workflow: Ask and analyze, Review evidence, Manage data.
- UI upload now creates/uses one case collection, registers the source with the
  forensic processor, preserves Pakistan jurisdiction/timezone hints, and also
  populates the legacy batch view when its parser supports the format.
- Local no-auth development is explicit and accountable through the fixed
  `nexusai-local-operator` proxy identity. Authenticated deployments always
  forward the real user instead.

## Defects found and corrected by live acceptance

1. The original UI proxy forwarded blank actor/role values when LocalAI auth was
   disabled. It now fails closed unless an explicit local operator identity is
   configured; the local Compose override supplies that identity.
2. The Records UI upload previously populated only the legacy store. It now
   registers evidence through the governed forensic pipeline as well.
3. Explicit record type was ignored for classification when TSV header sniffing
   was deferred. Explicit CDR TSV now queues to the CDR worker, with a regression
   test.
4. Large messy evidence details can contain case-sensitive source keys such as
   `CNIC` and `cnic`. A privacy-safe `view=accounting` response now returns only
   processing status and whitelisted job counters.
5. Capability evidence coverage recomputed extensions from filenames and
   silently discarded query errors. It now uses the canonical `extension`
   column and returns an error instead of a false zero-data result.

## Verification

- Python worker/adapters: 51/51 passed.
- Focused LocalAI forensic forwarding: passed in 13.934 seconds.
- Forensic Go package passed repeatedly after each defect correction; latest
  pre-deployment source result: 12.175 seconds before the capability fix and
  20.927 seconds for the accounting-view addition.
- React production build: 656 modules transformed; passed in 1.81 seconds.
- Gate 13 build/deploy: API 34.3 seconds; LocalAI/UI 474.9 seconds; both ready;
  rollback images preserved; named volumes preserved.
- Gate 13b API-only repair: passed twice during evidence-detail hardening.
- Gate 12: all nine accounting checks passed and execution reached the
  capability assertion. It stopped truthfully when spreadsheet coverage was
  mislabeled; the tested source fix is awaiting the final API-only deploy.

## Models

The deployed, unchanged CPU baseline is:

- chat/planner/synthesis: `qwen_qwen3-4b-instruct-2507`, Q8_0, llama.cpp,
  8 CPU threads, approximately 4.28 GB;
- embeddings: `qwen3-embedding-0.6b`, Q8_0, 1024 dimensions, 8 CPU threads,
  approximately 639 MB.

The three-round stored benchmark reports 6/6 chat checks successful, average
8,565.53 ms, p50 4,781.66 ms and p95 12,843.57 ms. Models do not calculate
forensic counts, money, duration, identity joins or timelines; deterministic
adapters and SQL do that. OCR, STT, TTS, vision and video model promotion remain
separate accepted-model phases.

## Final local closure commands

Run from the repository in Windows PowerShell:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\repair_forensic_phase4_api_gate13b.ps1

powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\smoke_forensic_phase4_structured_gate12.ps1
```

Expected final lines:

```text
Phase4Gate13b=PASS ...
Phase4Gate12=PASS SyntheticFiles=9 Families=9 TotalRows=68 AcceptedUnique=39 DuplicateRows=8 Rejected=21 Capabilities=20 CrossFamily=PASS SourceAudit=PASS RealEvidenceUsed=false
```

If either command does not print PASS, preserve its output and stop. Do not
delete evidence, rerun migrations, or upload the supplied real CDR.

## Required phase ledger

1. **Objective:** deploy and prove Phase 4 operational structured depth and the
   analyst-first UI.
2. **Assumptions:** Phase 3 runtime healthy; synthetic-only retained writes;
   models and named volumes unchanged.
3. **Files changed:** API classification/capability/evidence detail; LocalAI
   proxy identity; collection forwarding; React upload workflow; TSV/XLSX
   fixtures/generator; Gates 12/13/13b; reports/checkpoint.
4. **Schema/API/config:** no database migration; new optional evidence detail
   accounting view; explicit local proxy identity; canonical capability extension.
5. **Tests:** 51 Python, focused LocalAI Go, forensic Go, React build passed.
6. **Runtime:** API and LocalAI rebuilt/deployed/ready; final capability source
   fix still needs API-only deployment.
7. **Data:** 9 synthetic files, 68 total, 39 accepted unique, 8 duplicate, 21
   rejected; no real evidence uploaded.
8. **Models:** unchanged Qwen CPU baseline; no model promotion.
9. **Accuracy:** exact golden accounting passed; capability and final query
   assertion awaits the final deploy/rerun.
10. **Latency/memory:** API build 34.3 s, LocalAI build 474.9 s; stored chat
    benchmark p50 4.78 s/p95 12.84 s; no production percentile claim.
11. **Failures:** TSV explicit-routing, PowerShell case-sensitive JSON and
    capability false-zero defects found and fixed; privileged Docker usage limit
    blocks the last deployment from Codex.
12. **Security/provenance:** accountable proxy identity, no secret output, no
    raw records in the gate, full evidence preservation.
13. **Git:** unstaged, uncommitted and unpushed; no GitHub action.
14. **Rollback:** Phase 4 API/LocalAI rollback tags and Phase 3 database backup
    remain available; volumes preserved.
15. **Next action:** run the two closure commands, capture only PASS summaries,
    then use the team-lead demo and begin the separately gated document/OCR phase.
