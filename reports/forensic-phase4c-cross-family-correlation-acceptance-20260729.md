# Phase 4C Exact Cross-Family Correlation Acceptance

Date: 2026-07-29  
State: source accepted; not rebuilt, deployed, migrated, staged, committed, or pushed

1. **Objective:** add a deterministic analyst route that can find one or more
   identifiers across CDR, IPDR, ANPR, subscriber, tower/location, financial
   transaction, access/security-log, and generic canonical records, then return
   source-cited related entities without using model inference for exact facts.
2. **Assumptions verified:** `forensic.records` is the canonical structured
   store; exact correlations require tenant/collection scope and source lineage;
   punctuation-tolerant phone/plate lookup must be bounded and must not become
   arbitrary substring matching; legacy exploratory relationship behavior must
   remain backward compatible.
3. **Files changed:** `api/forensic_records/query.go`,
   `api/forensic_records/phase4_cross_family_ginkgo_test.go`,
   `api/forensic_records/capabilities.go`,
   `core/services/agents/records_tools.go`, the versioned Pakistan-oriented
   golden, API/user documentation, this report, the team-lead brief, and the
   living continuation checkpoint.
4. **Schema/API/config:** no schema, migration, endpoint, or runtime-config
   change. `GET /query/templates` now advertises `cross_family_correlation`, and
   `POST /query/hybrid` can plan or explicitly select it. The query plan exposes
   the canonical table, exact matching policy, target cap, and execution strategy.
5. **Tests:** six focused Phase 4C Ginkgo specs passed in 20.838 s package time;
   the final complete `go test ./api/forensic_records -count=1` regression passed
   in 11.309 s; the focused forensic agent-tools suite passed in 31.065 s package
   time (117.4 s wall time on this constrained laptop); `go vet` passed for the
   forensic API and agent packages in 15.9 s. JSON and whitespace validation
   also passed.
6. **Live runtime:** no API/worker/LocalAI rebuild or restart, no retained query,
   no database migration/write, no queue publication, and no live evidence
   upload. SQL behavior is source-contract tested and still needs activation-time
   read-only execution against an approved disposable or retained environment.
7. **Dataset/evidence:** one synthetic, de-identified Pakistan-oriented golden
   contract: four planner questions, four cited related occurrences, and three
   expected aggregates spanning transaction XLSX/JSONL, CDR CSV, and subscriber
   JSONL. No real or retained evidence was read.
8. **Models:** the configured Qwen chat/embedding baseline is unchanged. No model
   was called, downloaded, installed, benchmarked, or promoted for this slice.
9. **Accuracy/retrieval:** planner agreement is 4/4; expected relation aggregate
   agreement is 3/3; citations are preserved 4/4; source-contract assertions
   enforce exact equality and prohibit `ILIKE`. These are fixture-contract
   results, not a live case accuracy claim.
10. **Latency/memory:** no live-query p50/p95 or memory claim. SQL fan-out is
    bounded to eight requested targets, capped displayed matches, a 1,000-row
    related-occurrence scan, and five citations per related entity.
11. **Failures/fallbacks:** comparison questions without two targets request
    clarification; more than eight caller targets return HTTP 400 before database
    work; phone compaction is disabled below eight digits; relation/display caps
    produce visible non-exhaustive limitations. The legacy
    `relationship_network` route remains available and unchanged.
12. **Security/provenance:** queries retain explicit tenant, collection, optional
    date, and optional record-type filters. Each displayed match carries available
    `evidence_id`, `version_id`, `record_id`, source file, row number, row hash,
    timestamp, and XLSX sheet/row locator. No source text is treated as an
    instruction.
13. **Git:** the existing dirty worktree remains unstaged, uncommitted, and
    unpushed. No GitHub action was performed.
14. **Rollback:** source-only changes can be reviewed or reverted before
    activation; no retained state or deployed image needs rollback.
15. **Next action/approval:** Phase 4D should add per-sheet typed XLSX mapping and
    messy multi-provider schema/timezone/duplicate goldens, followed by deeper
    family-specific financial/log queries. Separate approval remains mandatory
    for rebuild/deployment, migrations, model changes, sensitive-data ingest,
    staging, commit, push, or pull request creation.

## Delivered analyst behavior

- `correlate 923461678183 across record families`
- `connect ABC-123 across datasets`
- explicit template aliases: `cross_family`, `cross_dataset_correlation`, and
  `correlate_records`
- exact case-normalized keys, phone digit keys only at eight or more digits, and
  alphanumeric compact keys only for mixed letter/digit identifiers
- matched record count, per-target/per-family coverage, capped source records,
  cited related entities, and explicit truncation state

## Boundary retained

This slice does not claim probabilistic entity resolution, ownership inference,
fuzzy name matching, OCR-derived plate matching, STT-derived phone matching, or
exhaustive graph construction. Those require separate goldens, uncertainty
semantics, authorization, and later-phase acceptance.
