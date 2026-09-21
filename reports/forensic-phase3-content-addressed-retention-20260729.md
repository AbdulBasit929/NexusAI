# Phase 3 Content-Addressed Retention Acceptance — 2026-07-29

1. **Objective:** replace mutable job-named spool assumptions with scoped,
   create-only content addresses, independent receipts and full hash
   re-verification before processing.
2. **Assumptions verified:** the API is the only spool writer; the worker can
   consume a read-only mount; source filenames must remain metadata because
   content addresses are extensionless; retained services/database remain gated.
3. **Files changed:** storage implementation/tests, API upload registration,
   worker validation/tests, Compose source config, migration 008 contracts and
   project/operator/design documentation.
4. **Schema/API/config:** no new endpoint and no applied schema. Upload metadata
   now carries stable object/receipt URIs, layout, scope, verified time/method and
   reuse state. Migration 008 can normalize these facts when later approved.
5. **Tests:** full `api/forensic_records` package passed (`ok`, 24.397 s test
   time; 99.6 s cold wall); `go vet` passed; Python worker/adapter suite passed
   27/27 in 2.116 s; Python compilation and Compose validation passed. Storage
   focus also passed five independent concurrency/retry runs before final suite.
6. **Live runtime:** no service build/recreate, retained database write,
   evidence ingest, volume rewrite, model change or runtime secret change.
7. **Dataset/evidence:** synthetic bytes only. The real retained registry and
   previously supplied Pakistan CDR sample were not altered or recopied.
8. **Models:** Qwen chat/embedding baseline unchanged; no model downloaded,
   installed, promoted or benchmarked.
9. **Accuracy/retrieval:** actual streamed size and SHA-256 are authoritative;
   content objects keep original-name parser semantics; Phase 2 support claims
   and truthful pending/manual routes are unchanged.
10. **Latency/memory:** no live service/model measurement was changed. Full Go
    validation used a cold local cache; source storage tests cover bounded
    publish-window retries rather than unbounded waits.
11. **Failures/fallbacks:** a cold Go compile previously hit its time bound and
    later passed with a local cache. Windows temporary-directory symlink
    resolution was made platform-aware while Linux keeps root symlink checks.
    Disposable acceptance wrappers exposed startup/readiness, filename and final
    query mistakes; the final exact run passed. A real nullable legacy
    `write_once` backfill defect was found and fixed with `coalesce(..., false)`.
12. **Security/provenance:** caller path components are hashed; publish is
    atomic/create-only; objects and receipts are read-only; traversal, symlink,
    corruption, receipt tamper, size/URI mismatch and 12-writer concurrency are
    tested; conflicting good bytes are preserved in a read-only quarantine.
13. **Git:** all work remains unstaged, uncommitted and unpushed; no PR or other
    publication was created.
14. **Rollback:** source changes are undeployed. Disposable TimescaleDB ran the
    full 001–008 chain, preflight, verify, storage smoke, second forward apply,
    idempotency and non-owner RLS smoke; it returned
    `forensic_spool_content_addressed|verified|true|sha256-full-read-after-retain`
    and was removed. The retained database remains unmigrated.
15. **Next action/approval:** implement the reliable JetStream queue lifecycle—
    durable acknowledgement, bounded retry/backoff, dead-lettering, lease/
    concurrency rules and explicit reprocessing without erasing prior outputs.
    Build/deploy, migration, backup, runtime grants and infrastructure WORM remain
    separate approval gates.
