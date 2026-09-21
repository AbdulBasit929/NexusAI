# NexusAI Phase 3 non-owner runtime source preparation

Date: 2026-07-30 (Asia/Karachi)

## 1. Objective

Prepare a fail-closed, locally executable activation path for the retained Phase
3 control plane without running long builds, starting processors, changing
secrets, applying migration 010, or admitting real evidence from Codex.

## 2. Assumptions verified

Migrations 008/009 are retained and verified; PostgreSQL/NATS were last healthy;
API/worker remain an intentional stopped boundary; the operator prefers long and
mutating commands in local PowerShell.

## 3. Files changed

Added migration 010 preflight/forward/verify, runtime grants/RLS proof, runtime
Compose overrides and env template, Go/Python contracts, authenticated smoke
support, the local PowerShell runbook, and this report. Updated DB documentation,
the ignored-secret rule, the worker and the continuation checkpoint.

## 4. Schema, API and configuration changes

Source only. Migration 010 will add explicit policies to three existing
RLS-enabled tables and set two aggregate views to `security_invoker`. Runtime
Compose will require shared API auth and use `forensic_runtime`. Nothing was
applied to the retained database or active containers in this step.

## 5. Tests and exact results

Final four-module Python activation suite: 49/49 passed in 1.164 seconds. The
operator's full forensic Go package passed in 18.178 seconds using repository-local
`GOCACHE`/`GOTMPDIR`. Both Compose merges passed `config --quiet`; the smoke script
and all runbook blocks parsed; selected diff whitespace passed. The first operator
Python attempt was interrupted only because Windows PowerShell 5 promoted expected
test diagnostic stderr to `NativeCommandError`; a reusable exit-code-authoritative
runner corrected the shell behavior without changing or weakening a test.

## 6. Live-runtime checks

No API, worker or LocalAI build/recreate/start/restart occurred. Activation Gate
2 created the ignored local runtime environment. The operator then manually
regenerated its two cryptographic values once after the protected runner had
already passed; final 64-hex shapes were validated locally, the ACL denies Codex
read access, and Git confirms the file is ignored. No value was printed.

## 7. Dataset and evidence counts

No dataset was read, uploaded or changed. The runbook permits only the five-row
synthetic Pakistan CDR fixture for the first runtime write and prohibits the real
supplied CDR and verified retained collection from that smoke.

## 8. Models

No model was searched, called, downloaded, installed, changed or promoted. The
existing Qwen configuration remains a recorded baseline only.

## 9. Accuracy and retrieval metrics

Not applicable to source preparation. Existing deterministic structured-data
acceptance remains unchanged.

## 10. Latency and memory

Final Python ran in 1.164 seconds and the operator Go package reported 18.178 seconds. The
runbook requires at least 6 GiB free RAM before the focused LocalAI compilation/
build and records elapsed build times locally.

## 11. Failures, fallbacks and unresolved blockers

The source audit exposed and corrected worker DDL under the application identity,
three policy-less RLS tables, owner-context aggregate views and a Windows
PowerShell 5 native-stderr logging trap. Compose Gate 5 passed without expansion
or secret output. Focused LocalAI validation passed in 43.827 seconds. The first
rebuild attempt stopped safely at 4.89 GiB free RAM before tagging/building. Four
post-reboot retries tagged only the unchanged API/worker images, then stopped
before LocalAI tagging or any build because implicit Compose discovery omitted
the healthy retained `nexusai-api`. Read-only inventory proved its image and
health; Gate 7 now pins project `nexusai` and has a running-label fallback.
Rebuild, migration 010, runtime role, deployment and live smokes remain
unexecuted. Live retry/DLQ/redelivery fault injection remains a disposable-stack
gate; it must not be simulated by corrupting or interrupting the retained stack.

## 12. Security, tenant isolation and provenance

The worker now verifies schema readiness and performs no owner DDL. The fixed
runtime role is non-superuser, non-owner and `NOBYPASSRLS`, without schema create,
delete or truncate grants. Secrets are generated into a Git-ignored ACL-restricted
file and never expanded by Compose output. Synthetic RLS writes roll back.

## 13. Git state

The existing dirty worktree remains unstaged, uncommitted and unpushed. No Git or
GitHub action occurred.

## 14. Rollback or recovery point

No service/database state changed, so no new recovery artifact exists yet. The
ignored runtime secret file must now be preserved and not regenerated. The
runbook creates and verifies a pre-010 custom dump before migration and
rollback-tags all existing images before rebuild. The already verified post-009
dump remains authoritative.

## 15. Next action and approval required

Reboot or close nonessential applications, keep Docker available, and rerun only
the full rollback-tagged Gate 7 rebuild; Gate 6 is already persisted. Then
continue the remaining ordered gates in
`reports/nexusai-phase3-runtime-local-powershell-runbook-20260730.md`, return only
the listed non-sensitive summaries, and stop on the first failure. Do not claim
Phase 3 runtime completion until migration/role/auth/JetStream, synthetic success
and disposable failure-path acceptance all pass. Real evidence, models and Git
publication remain separate decisions.
