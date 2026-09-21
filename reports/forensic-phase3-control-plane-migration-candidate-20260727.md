# Phase 3 Evidence Control-Plane Migration Candidate

Date: 2026-07-27  
Status: isolated acceptance passed; active local registry not migrated

1. **Objective:** deliver the first Phase 3 vertical slice: an additive,
   tenant-scoped evidence/version/source/run/event/artifact/custody schema with
   preflight, compatibility, repeat-apply and rollback evidence.
2. **Assumptions verified:** migrations 001-007 remain the installed baseline;
   disposable inputs are synthetic; the dirty worktree is preserved; application
   to the retained registry requires separate approval.
3. **Files changed:** migration 008, preflight, verification, synthetic legacy
   fixture, native smoke, idempotency verification, gated rollback, Go contract
   tests, database/design/product docs, team brief and living checkpoint.
4. **Schema/API/config:** seven additive tables normalize storage objects,
   evidence versions, source links, processing runs, processing events, derived
   artifacts and custody events. `evidence_items.current_version_id`, composite
   tenant FKs, RLS policies, synchronization and immutability triggers are added.
   No endpoint, subject, model or running configuration changed.
5. **Tests:** focused Phase 3 Go contracts passed; final forensic suite passed
   102/102 Ginkgo specs plus legacy tests in 3.152 s; `go vet` passed; Python
   worker/adapters passed 24/24 in 0.297 s; Compose, JSON, Python compilation and
   diff-whitespace checks passed.
6. **Live runtime:** no build, recreate, model load or migration. A read-only
   preflight against the running registry passed for 110 evidence items, 104
   linked KB assets and 41 linked jobs. Read-only inspection confirms Phase 3
   tables and `current_version_id` remain absent.
7. **Dataset/evidence:** the isolated legacy fixture normalized 1 evidence into
   1 storage object, 1 version, 3 distinct source links, 1 run, 1 processing
   event and 1 custody event. Native smoke added 1/1/2/1/3/3 respectively.
   Repeat application retained totals 2/2/5/2/4/4 with zero count drift.
8. **Models:** unchanged Qwen chat/embedding baseline; no model download,
   activation, revision, parameter or prompt change.
9. **Accuracy/retrieval:** not applicable to this schema slice. The fixed 20/20
   modality, 18/18 chat and 27/27 deterministic-route Phase 2 gates remain the
   unchanged regression baseline.
10. **Latency/memory:** focused contract package 19.321 s after compilation;
    final package 3.152 s; Python 0.297 s. No workload or memory benchmark was
    required because the active services and models were unchanged.
11. **Failures/fallbacks:** testing found and fixed native reapply source-link
    drift, a non-monotonic synthetic legacy timestamp, non-atomic DDL, and a
    deferred-FK/RLS transaction ordering issue. An injected incompatible row was
    rejected by preflight; forced migration failure left no partial schema.
12. **Security/provenance:** byte identity and append-only versions/source/events
    are protected; custody events form a serialized SHA-256 chain; every FK is
    tenant/collection scoped; seven RLS policies include `USING` and `WITH CHECK`.
    Authenticated tenant binding and non-owner RLS tests remain required before
    production authorization claims.
13. **Git:** changes remain unstaged, uncommitted and unpushed; no PR or external
    publication was created.
14. **Rollback:** rollback is blocked unless
    `app.phase3_allow_destructive_rollback=true`. In disposable PostgreSQL it
    removed every Phase 3 object and pointer atomically while retaining both
    original Phase 2 evidence rows.
15. **Next action/approval:** prepare and verify a fresh scoped backup, capture
    pre-migration counts/hashes, then request explicit approval to apply migration
    008 to the retained local registry. Source-only auth/RLS and reliable queue
    lifecycle work may continue without applying it. Real sensitive ingestion,
    model changes, heavy builds, destructive recovery and Git publication remain
    separately gated.
