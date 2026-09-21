# NX-MMR P1 snapshot-reader repair R2 — 2026-09-02

## Outcome

R1 built both candidates, briefly activated them, and automatically restored
the original immutable images after verification failed on receipt parsing.
Independent rollback verification passes. R2 fixes and tests the parser and is
resealed; R2 has not been built or activated.

## R1 receipt

Receipt: `20260902T152519865Z`.

Built immutable candidates:

- API: `sha256:82c3eeecccc2b57e874d230a35e6c0f89fcfed72ee69542d536cf43f66a975fa`
- Forensic API: `sha256:debb5504abacf33b532ef2a9c912a4254a5f854998ff8d0c155884039c08271b`

Both candidate services started. Verification failed with
`Private runtime snapshot shape is invalid: api`. Rollback Compose then
recreated both original images before the same verification parser failed.
The orphan warning was informational; `--remove-orphans` was not used and the
worker, PostgreSQL, NATS and spool-init were not removed.

## Independent rollback oracle

The saved snapshot was inspected without printing private environment values.
Every service entry has `value, Count`; `value` is a one-element array whose
first element is the complete Docker inspect object. Correctly unwrapping that
object produced PASS for:

- API original image `sha256:b1f15e80...`, healthy, restart count zero.
- Forensic API original image `sha256:f0f49921...`, running, restart count zero.
- Exact environment, mount, network and restart-policy parity for both.
- Worker, PostgreSQL and NATS exact protected container/image identities.
- Worker role flags and required LocalAI model inventory.
- Active jobs zero and retained tuple `63|63|76|22507|827|61`.

Post-rollback mutable-service container IDs are:

- API: `580268751d33a04b5661d5488191c84bf89a7f2e8780e942f616cf8c59738dc8`
- Forensic API: `084d3757e025b5566bf51fcfd999542296d581aef5c432ec23437cfa14a2e780`

These new IDs are expected because rollback is a bounded recreation. They are
now bound into R2; protected container identities did not change.

## R2 correction and validation

`Get-NxP1SnapshotContainer` now accepts the original direct inspect-object
shape and the observed single-element `value` wrapper. It fails closed for
zero/multiple elements or entries without `Config`. Exact direct, wrapped and
ambiguous-wrapper tests pass under Windows PowerShell 5.1.

- Operator self-test: 45/45 PASS.
- R2 read-only preflight: every non-RAM gate PASS; active jobs zero.
- Current RAM with Codex open: 5.003723/6 GiB; disk 1642.605 GiB.
- No R2 build, activation, migration, upload, reprocessing or model download.

R2 namespace:
`NX-MMR-SHARED-P1-CORRECTION-ACTIVATION-V1-BUILD-DEPS-R2`.

Manifest SHA-256:
`6db9eb32d29b9b2b53fed8b4f679b48ed5d934771d1079a7ba82b66bbc1bf7cf`.

Integrity seal SHA-256:
`0823f0db215806af8fdba689c891efefead4ea531a8727e6b93f11bc000ffec9`.

Candidate tags:

- `nexusai/localai-forensic:nxmmr-shared-p1-correction-v1-build-deps-r2`
- `nexusai/forensic-records-api:nxmmr-shared-p1-correction-v1-build-deps-r2`

R1 candidates remain local but cannot satisfy R2's distinct manifest, seal and
tag contract. Docker may reuse valid build layers; that is not receipt reuse.

## Exact next command

Close Codex/ChatGPT, browsers/editors and optional apps. Keep Docker Desktop,
internet and one standalone PowerShell terminal running. Do not start jobs.

```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_nxmmr_p1_correction_activation.ps1 -WaitForRamMinutes 120
```

Type `YES` only after preflight PASS. Do not rerun R1 or manually manipulate
its receipt. Product certification and failed-cells acceptance remain pending.
