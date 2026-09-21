# Phase 4E CSV, IPDR, and Demonstration Readiness Acceptance

Date: 2026-07-29  
State: source accepted; not rebuilt, deployed, migrated, staged, committed, or pushed

1. **Objective:** prove the supplied real-format CDR CSV through the production
   adapter without ingesting or exposing it, close IPDR normalized-field depth,
   and provide a repeatable team-lead demonstration.
2. **Assumptions verified:** CSV remains a primary structured input; source
   timezone must be declared rather than guessed; row accounting must include
   exact duplicates and rejects; provider onboarding must not print identifiers.
3. **Files changed:** worker audit/optimization/IPDR normalizer; worker, attached
   CDR and Phase 4 structured tests; synthetic IPDR fixture/golden; capability
   matrix/docs; safe demo script and guide; phase report/checkpoint/team brief.
4. **Schema/API/config:** no database schema, endpoint, migration or active
   runtime configuration changed. New offline command:
   `worker.py audit-source <path> --record-type ... --source-timezone ...`.
5. **Tests:** final combined worker/adapter/XLSX/IPDR/attached-CDR regression
   passed 50/50 in 3.116 s; full forensic Go package passed in 30.520 s; React
   production build passed in 2.70 s. Python compile, two JSON contracts and
   `git diff --check` passed. The safe demo completed in 15 s.
6. **Live runtime:** no service, container or image rebuild/restart; no upload,
   queue event, retained database write, migration, model call or live query.
7. **Dataset/evidence:** supplied 828,259-byte CSV read-only: 3,931 rows, 3,931
   accepted, zero rejected, 297 exact duplicates, 3,634 unique normalized rows,
   zero overflow rows, 16 headers. Synthetic IPDR: six rows, four accepted,
   two visible rejects, one exact duplicate, three unique normalized sessions.
8. **Models:** configured Qwen baseline unchanged; no model search, download,
   call, benchmark, install or promotion.
9. **Accuracy/retrieval:** supplied CSV routes to CDR automatically and achieves
   100% row accounting. IPDR exact golden covers IPv4, IPv6, NAT, ports, bytes,
   start/end/duration, domain, subscriber, Pakistan timezone and explicit offset.
10. **Latency/memory:** real CSV full audit improved from measured 35–55 seconds
    to 2.94 seconds by caching immutable timezone resolution and normalizing CDR
    header aliases once per row. Semantics and exact accounting are unchanged.
11. **Failures/fallbacks:** direct PowerShell script execution was blocked by
    machine policy; the documented process-scoped `-ExecutionPolicy Bypass`
    invocation passed without changing system policy. Initial IPDR golden grouped
    two network-field rejects differently; aligned to the stable privacy-safe
    `invalid_network_value` category.
12. **Security/provenance:** audit output includes source hash/schema/accounting
    but emits no raw rows, identifiers or rejected values. IPDR performs no DNS
    lookup. Original real CSV was read only and never copied into fixtures.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
    operation.
14. **Rollback:** source-only. The temporary profiler output was deleted; no
    retained application state needs rollback.
15. **Next action/approval:** Phase 4 structured-data v1 is source-complete at
    this acceptance boundary. Run the read-only activation audit next. Backup,
    migrations 008/009, runtime settings and bounded rebuild remain separately
    approved steps before a live UI/API demonstration. Provider-specific long-tail
    schema packs remain continuous compatibility work rather than an exhaustive
    claim that every private vendor export has already been observed.

## Read-only activation audit

- Host memory at audit: 15.71 GiB total, 4.07 GiB free.
- `nexusai-api`: running and healthy on port 8080 using the earlier universal
  image; it does not contain the latest React/source changes.
- forensic PostgreSQL, NATS, API and worker: stopped, all exit 255 at the same
  Docker-engine timestamp; none was OOM-killed and no container error string was
  recorded.
- Compose configuration: valid; declared services are PostgreSQL, NATS,
  spool-init, forensic API and worker.
- API/worker current and rollback images are retained locally.
- Migration 008 SHA-256:
  `b130044ea7c5f39338ffd35f271934c0a36e99a6d86f9336aca91bef0b10476a`.
- Migration 009 SHA-256:
  `0d285379d6b719086508bb0f8ec41126647bc5e1d1f6b12143588c51c00fae9c`.
- No service was started, stopped, recreated or modified during the audit.
