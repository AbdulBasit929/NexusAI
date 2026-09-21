# NexusAI R6.4 source closure and R6.5-A1 audit

Date: 2026-08-11  
Program phase: R6 — Ask NexusAI and Specialist Orchestration Experience  
Disposition: R6.4 source accepted; R6.5-A in progress

Final update: the corrected guarded activation and R6.5 closure subsequently
passed. R6 is complete; authoritative final evidence is in
`nexusai-r6-final-runtime-acceptance-20260811.md`.

## Reconciled live baseline

The operator-provided activation evidence is accepted for
`R6.3-B+R6.4-A1`: API/worker/LocalAI build times were 14.6/4.5/139.5 seconds,
one LocalAI attempt completed with 6.26 GiB free RAM, rollback images and named
volumes were preserved, and both readiness endpoints returned HTTP 200. The
marker records the expected history, presentation and A1 corpus contracts with
zero mutating requests. This evidence does not claim deployment of the R6.4
source additions described below.

## R6.4 closure

| Acceptance surface | Result | Evidence |
| --- | --- | --- |
| Operation completeness | pass | Authoritative template catalog contains exactly 65 entries. |
| Corpus completeness | pass | Embedded `2026-08-11.r6.4` materialization covers every accepted template. |
| Clean suggestion UX | pass | Only explicit `suggested: true` corpus entries enter the starter grid. |
| Family/specialist presentation | pass | Enterprise operation metadata is transported into `forensics.agent-presentation/v1`. |
| Source transparency | pass | Every catalog item declares records, KB or hybrid access; the UI displays the actual source. |
| Model authority | pass | Incompatible forensic synthesis models are rejected and deterministic fallback remains explicit. |
| Governed edge cases | pass | Versioned scenarios cover ambiguity, no results, unsupported audio/OCR, model rejection and records/KB access. |

Three A1 curated questions referenced nonexistent template names. They were
corrected to `longest_call`, `ipdr_session_volume` and an explicit
`canonical_records` transaction query before R6.4 closure. Generated catalog
entries fill the remaining operation coverage without expanding the visual
starter grid.

## R6.5-A1 source-closure audit

| Domain | A1 result | Remaining closure work |
| --- | --- | --- |
| API | hardened | Full forensic sidecar package passes; validate the final deployed catalog marker. |
| Documentation | updated | Reconcile final runtime evidence after the operator gate. |
| Security/authority | preserved | GET-only activation; no schema drop, history mutation or model-authority widening. Complete final route/auth regression. |
| Accessibility | hardened | Typed result region, table caption/column scopes, keyboard scrolling and polite progress status; desktop/mobile browser acceptance passes. |
| Performance | bounded | Catalog/corpus are embedded and deterministically materialized; UI renders only curated suggestions; the 669-module production build completes in 4.84 seconds. |
| Rollback | ready | R6.4 gate delegates to the proven sequential combined gate and preserves rollback images and named volumes. Operator execution remains approval-gated. |

## Verification ledger

- Focused forensic sidecar catalog/corpus/capability/model-policy tests: pass.
- Full forensic sidecar package: pass.
- All 65 Agent Chat explicit-routing and unique-presentation-title tests: pass.
- Typed forensic presentation contract suite: pass.
- Focused ESLint: zero errors (16 pre-existing warnings in `AgentChat.jsx`).
- UI production build: pass, 669 modules in 4.84 seconds.
- Complete Agent Chat browser suite: 17/17 pass.
- Independent in-app 1440×900 and 390×844 acceptance: active governed case,
  zero horizontal overflow and zero captured console warnings/errors.

## Boundary and next action

No service rebuild, deployment, database/schema/evidence/history mutation,
model download, staging, commit or publication occurred. Complete the remaining
R6.5-A source checks. Then, only with explicit operator approval, run
`scripts/build_deploy_nexusai_r6_4_gate.ps1` and require
`r6.4-live-acceptance.json` to report exact 65-template coverage, the accepted
corpus version, at least seven governance scenarios, zero mutating requests and
preserved rollback images/volumes. That operator gate is the bounded beginning
of R6.5-B, not an automatic consequence of source acceptance.

## First activation attempt and recovery

The first operator run stopped before the combined rebuild because the new
PowerShell process did not contain `NEXUSAI_AGENT_HISTORY_DATABASE_URL`. At the
same time, Docker Desktop had stopped all current Compose containers with exit
code 255, so readiness endpoints were unavailable and no acceptance marker was
written. This is a failed-closed precondition outcome, not a partial deployment.

The fallback had two incorrect assumptions: that the API container must be
running and that its environment retained the host-facing variable name.
Compose actually maps the value to `LOCALAI_AGENT_POOL_DATABASE_URL`. The base
gate now checks the newest running-or-stopped Compose API container and accepts
either name. It does not print or persist the URL. A credential-safe inspection
confirmed a PostgreSQL URL exists in the stopped current API container, and
PowerShell syntax validation passes for both activation scripts. The operator
may rerun the same R6.4 command; no manual secret copying is required.

A second attempt revealed that Windows PowerShell preserved a stale
`$LASTEXITCODE = -1` after the Docker-output pipeline despite returning the
correct container ID. The resolver incorrectly treated the valid output as a
failure. The two native exit-code dependencies were removed; returned values
are now validated directly. An exact replay confirms the container and mapped
entry are found, the process-local URL is assigned, it is PostgreSQL-shaped and
is not a placeholder. This attempt also stopped before the combined rebuild.
