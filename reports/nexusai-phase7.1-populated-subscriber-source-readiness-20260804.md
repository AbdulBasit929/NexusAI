# NexusAI Phase 7.1 Populated Subscriber Source Readiness

Date: 2026-08-04  
Environment: Windows 11, CPU-only, approximately 16 GiB RAM  
Scope: safe source/offline preparation for populated subscriber acceptance; no
retained upload, database write, rebuild, model call, or cleanup.

## Executive verdict

The populated-data gate is now source-ready but remains authorization-blocked.
A clearly synthetic Pakistan fixture and deterministic oracle cover all six
subscriber operations, exact row accounting, open/closed validity, conflicts,
reuse candidates, Urdu names, Pakistan phone variants, MCC 410 IMSIs, valid and
invalid IMEIs, CNIC-shaped fictional tokens, and explicit statuses.

The audit also found and fixed two source privacy gaps before any upload:

1. a raw or typed CNIC/name target could be rejected by SQL yet still be echoed
   into planner, response, audit-export, or optional model fields;
2. generic compatibility fields and canonical row export could expose a raw
   subscriber name or CNIC even though subscriber operations were redacted.

The fixes are source-tested and not deployed. The current deployed no-data path
remains accepted because it contains zero subscriber rows. This report does not
claim populated runtime acceptance.

## Phase report

1. **Objective:** complete every safe Phase 7.1 source/offline gate while
   retained synthetic upload authorization is unavailable.
2. **Assumptions verified:** branch is
   `codex/forensic-hybrid-checkpoint-20260723` at `40717b83510c`; the worktree is
   intentionally dirty; four live health surfaces return HTTP 200; no retained
   subscriber rows were assumed or created.
3. **Files changed:** subscriber worker projection, raw/typed query guards,
   canonical response redaction, focused Python/Ginkgo tests, populated JSONL
   fixture, machine-readable golden, benchmark, phase reports, and checkpoint.
4. **Schema/API/config:** no schema, route, auth registry, model, agent, or active
   configuration change. Existing subscriber and canonical-query behavior now
   fails closed on prohibited target types and redacts protected export fields.
5. **Tests:** focused Phase 7 Python 6/6 PASS in 0.128 s; combined Phase 2 plus
   Phase 7 Python 26/26 PASS in 0.130 s; final focused Phase 7 Ginkgo 23/23 PASS
   with package time 13.528 s; final full forensic API package PASS in 5.768 s; Go vet, Python
   compilation, JSON parsing, gofmt, and bounded diff check PASS.
6. **Live runtime:** read-only probes returned HTTP 200 for LocalAI, forensic
   API, worker metrics, and NATS. New privacy/source-fixture changes were not
   rebuilt or deployed, and no live evidence query or model call was made.
7. **Dataset/evidence:** fixture
   `pakistan_subscriber_populated_acceptance_v1.jsonl`, SHA-256
   `a5edc8f2658b12e8564b49560fe40959be397de6e086eaa1bfe2aa11bad912b0`;
   15 input rows, 12 normalizable candidates, 1 exact duplicate, 11 unique
   accepted rows, and 3 explicit rejects. Runtime evidence/version/batch/job IDs
   remain `data_pending_authorization` in the golden.
8. **Models:** installed/deployed model policy is unchanged: Qwen 4B is optional
   bounded explanation and Qwen 0.6B is retrieval-only. This slice made zero
   model calls and no model download/configuration change.
9. **Accuracy:** offline deterministic oracle agrees exactly: identity lookup 2
   cited rows; validity timeline 2 rows with 1 closed and 1 open window; device
   links 2 rows with 1 IMSI and 2 IMEIs; status summary 11 rows across 6 exact
   status/review groups; conflict audit 2 stable identifiers; reuse review 3
   candidates. Public projections contain masked CNIC only and omit names.
10. **Latency/memory:** bounded 200-iteration adapter benchmark measured 157.8104
    ms cold per 15-row pass, 3,094.0496 microseconds per warm input row, 323.2
    input rows/s, 439,373 peak Python allocation bytes, and 12,636 adapter-package
    bytes. These are local microbenchmarks, not runtime SLAs.
11. **Failures/fallbacks:** the new response-level test first found that the
    normalized CNIC survived in the planner's nested query-plan copy after the
    top-level target was cleared; source now clears both and the final 23/23
    focused run passes. The first 2,000-iteration benchmark exceeded the
    30-second command bound and is not a pass; the bounded 200-iteration run
    completed. Populated API/SQL, Agent Chat, export download, responsive UI,
    audit trace, and optional model explanation remain unexecuted without
    retained-data authorization and deployment.
12. **Security/provenance:** CNIC-shaped raw targets and typed `cnic`/name
    entities clarify with no SQL/model and are removed from response/export
    fields. New subscriber rows index canonical MSISDN plus subscriber reference
    or masked CNIC, never raw CNIC/name. Protected raw evidence remains preserved
    in storage; canonical API/export rows redact name/search-key/raw CNIC/digits.
13. **Git:** all work remains unstaged, uncommitted, and unpushed; no PR or
    external publication action occurred.
14. **Rollback:** source-only review/revert. No database, evidence, KB, model,
    container, image, volume, collection, or agent rollback is required. The two
    legacy subscriber-name collections remain untouched.
15. **Next action/approval:** obtain explicit authorization to deploy these
    source protections and upload the exact isolated synthetic fixture. Record
    generated evidence/version/batch/job IDs, run all six live operations,
    privacy/citation/pagination/export/audit checks, Agent Chat, one bounded Qwen
    explanation, and 390/820/1024/1440 UI/a11y/resource gates. Then clean up only
    through the approved manifest-backed procedure and begin Phase 7.2.

## Exact authorization boundary

No retained upload or cleanup was authorized in this request. Consequently:

- **live:** Phase 7.1 hardening and professional no-data behavior from the prior
  accepted deployment;
- **source-verified:** populated fixture, all-six-operation oracle, target-type
  guard, ingest projection privacy, canonical API/export redaction, and bounded
  adapter performance;
- **data-pending:** evidence/version/job identifiers, live SQL results,
  pagination/limits, CSV/JSON/audit downloads, Agent Chat populated behavior,
  model explanation comparison, responsive success state, and cleanup proof;
- **planned:** Phase 7.2 Tower/Site Intelligence after this gate closes.
