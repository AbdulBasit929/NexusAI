# NexusAI Phase 3 runtime activation — completion record

Date: 2026-07-30 (Asia/Karachi)

Outcome: **complete**. The retained NexusAI runtime now uses authenticated,
tenant-scoped forensic APIs, a non-owner PostgreSQL role governed by RLS,
persistent JetStream delivery, bounded retry/dead-letter behavior, immutable
linked reprocessing and idempotent completed-message redelivery. The full
LocalAI/React UI, forensic API and worker images are deployed and healthy.

## Required 15-point ledger

1. **Objective:** activate and prove the complete Phase 3 security, provenance,
   persistence and queue lifecycle against the retained local runtime without
   risking real evidence.
2. **Assumptions verified:** migrations 008/009 were already verified; Gate 3
   produced a fresh pre-010 recovery point; only the isolated five-row Pakistan
   CDR fixture was authorized for write acceptance; named data/model volumes had
   to remain intact.
3. **Files changed:** migration 010 and runtime grants; authenticated/runtime
   Compose overlays; worker/API runtime fixes; strengthened health, success and
   redelivery gates; regression tests; runbook, report and living checkpoint.
4. **Schema/API/config:** migration 010 added the three missing tenant policies
   and caller-rights aggregate views. `forensic_runtime` is LOGIN, non-owner,
   NOSUPERUSER, NOCREATEDB, NOCREATEROLE, NOREPLICATION, NOINHERIT and
   NOBYPASSRLS. API authentication is mandatory. JetStream uses file storage,
   explicit acknowledgement and a compatible durable queue identity.
5. **Tests:** final Python worker/adapter suite 51/51 passed; final forensic Go
   package passed in 14.272 s with 136 registered Ginkgo specs plus the standard
   metadata compatibility regression; focused LocalAI forwarding had already
   passed in 43.827 s. PowerShell parsing, Compose merges and diff check passed.
6. **Live runtime:** PostgreSQL and NATS are healthy; forensic API is running and
   `/healthz` passes; worker is healthy and `/metrics` passes; rebuilt LocalAI is
   healthy and `/readyz` passes. Gate 9 proved unauthenticated 401, authenticated
   scope, wrong-tenant 403 and both JetStream streams.
7. **Dataset/evidence:** the sole activation write is one synthetic evidence item
   in `nexusai-runtime-acceptance-20260730`: five source rows, four unique
   canonical rows, one exact duplicate and zero rejected rows. It has two jobs:
   one preserved dead-letter parent and one completed generation-1 reprocess.
   Real supplied CDR data and `records-demo-verified` were not used or changed.
8. **Models:** existing Qwen chat and embedding configurations and volumes were
   preserved. No model download, replacement, promotion or new accuracy claim.
9. **Accuracy/retrieval:** exact synthetic accounting passed; repeat upload was
   idempotent; seven authenticated deterministic query routes passed; the
   20-family capability contract passed. This is pipeline correctness, not a
   new semantic-model benchmark.
10. **Latency/memory:** complete Gate 7 sidecar build 18.4 s and LocalAI/UI build
    515.5 s. Final Go suite 14.272 s. No production percentile/load claim.
11. **Failures/fallbacks:** activation correctly exposed and fixed four defects:
    missing `xlsx_reader.py` in the worker image, incompatible JetStream queue/
    durable names, missing transactional tenant context on non-owner upload
    paths, and mixed-type legacy metadata decoding. The initial canonical CDR
    SQL defect exhausted exactly 5/5 attempts and reached DLQ; history was not
    reset or deleted. Generation-1 reprocessing completed after the fix.
12. **Security/provenance:** runtime secrets were never printed; RLS proof rolled
    back; tenant context is set within every affected transaction; cross-tenant
    access fails closed; source hash/identity and failed history remain intact;
    completed redelivery was synchronously acknowledged with zero duplicate DB
    effects and the ingest stream drained.
13. **Git:** the dirty worktree remains unstaged, uncommitted and unpushed. No
    branch publication, GitHub push or pull request was performed.
14. **Rollback/recovery:** verified pre-010 dump
    `phase3-pre-010-runtime-rls-20260730T054541Z.dump` is 13,169,864 bytes with
    SHA-256 `f690323d0c2b16f6f4df12bc0b72891a26b053c23f836fa47287181880e543c5`.
    Gate 7 image tags, the stopped pre-Phase-3 LocalAI container and scoped API/
    worker repair tags remain preserved. Named volumes were never removed.
15. **Next action:** Phase 3 is closed. Proceed to Phase 4 operational depth:
    activate and benchmark the already-developed structured adapters across CDR,
    IPDR, ANPR, subscriber, tower, financial, access/security log, TSV and XLSX;
    expand provider/schema-drift goldens; then simplify and validate the React UI
    workflow before beginning model-gated document/image/audio/video pipelines.

## Final activation gates

- Gate 3: verified pre-010 backup — PASS.
- Gate 4: migration 010, non-owner role and rolled-back RLS proof — PASS.
- Gate 7: complete LocalAI/UI/API/worker rebuild — PASS.
- Gate 8/8b/8c: deployment and scoped repairs with rollback preserved — PASS.
- Gate 9: health, worker liveness, auth, tenant and JetStream contract — PASS.
- Gate 10: synthetic success, retry/DLQ history, linked reprocess, idempotency and
  authenticated query/capability smoke — PASS.
- Gate 11: completed redelivery acknowledgement, zero duplicate effects and
  drained stream — PASS.
