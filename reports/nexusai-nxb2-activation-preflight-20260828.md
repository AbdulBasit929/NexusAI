# NX-B2.0 live/source reconciliation and activation preflight

Status: preflight complete; activation not authorized and RAM gate closed  
Date: 2026-08-28  
Repository: `C:\Users\sheik\Workspace\Office\Projects\NexusAI`

# Verified Starting State

- Branch: `codex/forensic-hybrid-checkpoint-20260723`.
- HEAD: `40717b83510c08db25dc26b9d6674bf46db363ac`.
- Dirty-worktree entries: 601, all treated as intentional existing work.
- `git diff --check`: no whitespace error; Git emitted only existing
  LF/CRLF working-copy notices.
- NX-B1 source: 79 executable operations, A=5/B=63/C=11, 214 governed query
  variants, catalogue `2026-08-28.nxb1.1`.
- NX-B1 source changes were never deployed.

# NX-B1 Reconciliation

The 12 source-only operations remain Tier B, `bounded_uncertified`, limited and
suggestion-ineligible. No certification exposure was changed by this preflight.

# Live / Source Gap

Authenticated live inspection proves the runtime is pre-NX-B1:

| Surface | Source | Live | Verdict |
| --- | ---: | ---: | --- |
| Executable templates | 79 | 67 | stale |
| NX-B1 operations present | 12 | 0 | stale |
| Query variants/corpus entries | 214 | 68 | stale |
| Reconciled platform operation projection | 104 | 98 | stale |
| Platform catalogue version | `2026-08-28.nxb1.1` | `2026-08-23.post-bfa-runtime.1` | stale |

The live `/query/templates`, `/query/capabilities` and platform discovery
responses returned HTTP 200 under the existing authenticated scope. Every NX-B1
operation membership check returned false. Unit/source evidence is therefore
not represented as live evidence.

# Deployment Preflight

## Current runtime

| Service | Container ID | Image ID | State | Restart |
| --- | --- | --- | --- | --- |
| LocalAI/UI `api` | `321ba681774b371dfbbf8a85c34ee45596e62f659800e6d647a0f5b7974c4021` | `sha256:628f56af541ac739887ddfb6f08d661e1070c4d0a2d343fb902882ab0f5dba9a` | running/healthy | `no` |
| forensic API | `b5c13f96e67f21f129f477c1b0d1a921a7d761ca528757240ca2ee90a85157d8` | `sha256:d2992062b8f55334d602530eb96cec8289dd714f0ae9523d3b27edebb724b5c5` | running/no Docker healthcheck; HTTP healthy | `unless-stopped` |
| worker | `296e0881d891b421b36da873f902b485669d1df82a258d37aa9db8e5de19e2fd` | `sha256:be13dbec4bd9bb62c824140df3777b898d7ad2801a0e68cd512c00dafaf4f9ef` | running/healthy | `unless-stopped` |
| PostgreSQL | `f66e05a3b17978c9912aa911177f22e57d69c17de87881a33f23bcba228ad361` | `sha256:61f891691050da6032023c01ea885730eeeba06b7c17b403e7d0b9c49c37dfe9` | running/healthy | `no` |
| NATS | `5c49e70d132ac01b4adf86f27b1735d4d5cf6cc205d848c0881164f1f200eb04` | `sha256:e4bf19f15fd3218814a4e3c9e0064e1334bd8aa20d5984b9f1a0afd084f8cc00` | running/healthy | `no` |
| spool initializer | `21597db0ccc77358920edf28158ed39c832cbe0d84e926dd772e8bdd827afdc7` | `sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce` | exited successfully | `no` |

All services use `nexusai_default`. Named volumes remain
`nexusai_forensic_postgres_data`, `nexusai_forensic_nats_data`,
`nexusai_forensic_spool`, `nexusai_models`, `nexusai_configuration`,
`nexusai_data`, `nexusai_backends` and `nexusai_images`. The worker retains the
approved `/models/media` bind. No mount was changed.

## Health and resources

- forensic API `/healthz`: HTTP 200.
- worker `/metrics` on port 9109: HTTP 200; container healthy.
- PostgreSQL: `pg_isready` accepting connections; container healthy.
- NATS monitoring `/healthz`: HTTP 200; container healthy.
- LocalAI `/readyz`, `/healthz` and `/`: HTTP 200; container healthy.
- Fresh free physical RAM: 3.201 GiB of 15.713 GiB.
- Mandatory activation floor: 6 GiB.
- C: free disk: 1648.013 GiB.
- `DeploymentGate=CLOSED` because the RAM floor is not met.

The accepted forensic Compose topology resolves to PostgreSQL, spool-init,
NATS, API and worker, with the expected three named forensic volumes. The
runtime-hardening verifier passes dependency readiness, restart policies,
service/network/volume identity and secret nondisclosure. LocalAI resolves as
the single `api` service through the merged base/runtime overlay.

The runtime environment exists with accepted SHA-256
`82bb4660ef9615f821f3ef3157652054d6b338fa5324cb894f53ea6e9e5de726`.
Required API/History variables are present. The History database URL is absent
from the env file by design but was safely resolved from the live accepted
container via `Set-ForensicAgentHistoryDatabaseEnvironment`; no secret value
was printed or persisted.

# Deployment Decision

Activation is required but was not performed. Two independent gates are not
open:

1. this directive does not constitute separate deployment approval; and
2. 3.201 GiB free RAM is below the mandatory 6 GiB floor.

# Exact Activation Scope

Three services require source parity:

1. `forensic-records-api`: owns the 79-operation query catalogue, the 12
   executors, capability projection, ledgers, citations and result contracts.
2. `api` (LocalAI/UI monolith): owns the changed forensic direct-agent routing
   and presentation adapter needed for live Ask behavior. No React UX change is
   implied, but this service contains the changed Go code.
3. `forensic-records-worker`: embeds `forensic-platform-v1.json`. Its live
   catalogue is `2026-08-23.post-bfa-runtime.1` and lacks six NX-B1 static
   descriptors; source is `2026-08-28.nxb1.1`. No processing or reprocessing is
   authorized, but image parity requires this bounded rebuild.

PostgreSQL, NATS, models and volumes must not be rebuilt or recreated.

# Required Rollback References

Before any build, create and verify these exact tags:

```powershell
docker image tag sha256:d2992062b8f55334d602530eb96cec8289dd714f0ae9523d3b27edebb724b5c5 nexusai/forensic-records-api:rollback-before-nxb2-20260828
docker image tag sha256:be13dbec4bd9bb62c824140df3777b898d7ad2801a0e68cd512c00dafaf4f9ef nexusai/forensic-records-worker:rollback-before-nxb2-20260828
docker image tag sha256:628f56af541ac739887ddfb6f08d661e1070c4d0a2d343fb902882ab0f5dba9a nexusai/localai-forensic:rollback-before-nxb2-20260828
```

The current LocalAI image already has
`nexusai-ui:rollback-before-integration-repair-20260828-093252`; the older
pre-NX-UX1 rollback image remains
`nexusai-ui:rollback-before-nxux1-20260828-091424`. The dedicated NX-B2 tags
still must be verified before build so rollback intent is unambiguous.

# Exact Activation Plan

Run only after explicit approval and a fresh `>= 6 GiB` free-RAM check. Use one
PowerShell process so the accepted History URL remains process-local:

```powershell
. .\scripts\forensic_runtime_common.ps1
$runtime = Get-ForensicRuntimeEnvironment -Path (Resolve-Path '.env.forensic-runtime.local').Path
Set-ForensicAgentHistoryDatabaseEnvironment -RuntimeEnvironment $runtime -ComposeProject 'nexusai' -ComposeService 'api'

docker compose -p nexusai --env-file .env.forensic-runtime.local `
  -f .\docker-compose.forensic-records.yaml `
  -f .\docker-compose.forensic-records.runtime.yaml `
  -f .\docker-compose.forensic-records.asr-small.yaml `
  build forensic-records-api

docker compose -p nexusai --env-file .env.forensic-runtime.local `
  -f .\docker-compose.forensic-records.yaml `
  -f .\docker-compose.forensic-records.runtime.yaml `
  -f .\docker-compose.forensic-records.asr-small.yaml `
  build forensic-records-worker

docker compose -p nexusai --env-file .env.forensic-runtime.local `
  -f .\docker-compose.yaml `
  -f .\docker-compose.forensic-runtime.localai.yaml `
  build api

docker compose -p nexusai --env-file .env.forensic-runtime.local `
  -f .\docker-compose.forensic-records.yaml `
  -f .\docker-compose.forensic-records.runtime.yaml `
  -f .\docker-compose.forensic-records.asr-small.yaml `
  up -d --no-deps --force-recreate forensic-records-api

docker compose -p nexusai --env-file .env.forensic-runtime.local `
  -f .\docker-compose.forensic-records.yaml `
  -f .\docker-compose.forensic-records.runtime.yaml `
  -f .\docker-compose.forensic-records.asr-small.yaml `
  up -d --no-deps --force-recreate forensic-records-worker

docker compose -p nexusai --env-file .env.forensic-runtime.local `
  -f .\docker-compose.yaml `
  -f .\docker-compose.forensic-runtime.localai.yaml `
  up -d --no-deps --force-recreate api
```

This plan does not use `down`, `--remove-orphans`, pruning, migration,
reprocessing, upload or volume operations.

# Expected Mutation

- Three new service images are built under the existing runtime references.
- Three rollback tags are added.
- Exactly three containers are recreated and receive new container IDs.
- PostgreSQL/NATS containers and every named volume remain unchanged.
- The retained tuple and Activity count must remain unchanged during activation.
- No model, profile, backend, schema, evidence, job or Activity entry changes.

# Rollback Plan

If health, catalogue, scope, citation or retained invariants fail, set the three
image overrides to the verified rollback tags and recreate only those services:

```powershell
$env:NEXUSAI_FORENSIC_API_IMAGE='nexusai/forensic-records-api:rollback-before-nxb2-20260828'
$env:NEXUSAI_FORENSIC_WORKER_IMAGE='nexusai/forensic-records-worker:rollback-before-nxb2-20260828'
$env:NEXUSAI_LOCALAI_RUNTIME_IMAGE='nexusai/localai-forensic:rollback-before-nxb2-20260828'

docker compose -p nexusai --env-file .env.forensic-runtime.local `
  -f .\docker-compose.forensic-records.yaml `
  -f .\docker-compose.forensic-records.runtime.yaml `
  -f .\docker-compose.forensic-records.asr-small.yaml `
  up -d --no-deps --force-recreate forensic-records-api forensic-records-worker

docker compose -p nexusai --env-file .env.forensic-runtime.local `
  -f .\docker-compose.yaml `
  -f .\docker-compose.forensic-runtime.localai.yaml `
  up -d --no-deps --force-recreate api
```

After rollback, recheck the exact retained tuple and all protected container,
network, mount and health invariants. Never roll back PostgreSQL volumes.

# Retained Tuple Before

Global retained tuple: `51|51|64|0|22207|441|47`.

Primary workspace tuple (evidence, versions, canonical rows, artifacts, KB):
`24|24|9275|426|22`.

Activity count: 20.

# Retained Tuple After

Not measured because activation was not performed. No retained write occurred
during preflight.

# Post-Activation Validation Plan

Before NX-B2.3 retained certification:

1. verify API HTTP 200, worker metrics/health, PostgreSQL, NATS and LocalAI/UI;
2. verify API/worker restart policies remain `unless-stopped`;
3. verify PostgreSQL/NATS IDs, all networks/mounts and the complete retained
   tuple are identical to pre-activation;
4. verify Activity remains 20 before any deliberately approved Ask submission;
5. verify live templates=79, all 12 NX-B1 IDs present, query ledger/corpus is
   source-current and the platform projection is source-current;
6. run focused NX-A1/NX-B1/no-unrestricted-SQL tests;
7. only then begin representative retained family, Ask/action/Activity,
   cross-family and responsive product certification.

# Open P0

0.

# Open NX-B2 P1

1: `NXB2-P1-001` — all 12 NX-B1 operations are unavailable in the live 67-
operation runtime. Resolution requires the approved activation above.

# Runtime / Retained / Model / Database Impact

```text
DeploymentPerformed=false
RuntimeMutated=false
RetainedStateMutated=false
ActivityMutated=false
ModelsChanged=false
ProfilesChanged=false
BackendsChanged=false
DatabaseMigration=false
VolumesChanged=false
```

# NX-B2 Status

`NX-B2.0=COMPLETE`; NX-B2 overall remains active and incomplete.

# Exact Next Phase

NX-B2.1 — bounded B1 runtime activation, not started.

# Exact Next Action

Free/recover physical RAM until a fresh preflight reports at least 6 GiB, then
explicitly approve the exact rollback-tagged three-service activation. Do not
run retained certification against the stale runtime.

```text
NXB2Preflight=PASS
LiveSourceGap=PROVEN
B1RuntimeActivated=false
DeploymentRequired=true
DeploymentAuthorized=false
DeploymentGate=CLOSED
FreePhysicalRAMGiB=3.201
RequiredFreePhysicalRAMGiB=6
RetainedTupleBefore=51|51|64|0|22207|441|47
RetainedStateMutated=false
OpenP0=0
OpenNXB2P1=1
NXB2=INCOMPLETE
```
