# NX-MMR P1 build-dependency repair R1 — 2026-09-02

## Outcome

The operator bundle is repaired and resealed, not rebuilt or activated.
This supersedes the offline-build handoff in the consolidated P1 candidate
report. Analytical source and trusted-Urdu worker behavior are unchanged.

## Failure and authority

Receipt `20260902T145238258Z` passed the 6-GiB RAM gate and failed at the
uncached Ubuntu requirements layer. Build RUN networking was disabled, so
apt could not resolve its configured mirrors. Exit 100 was a dependency
failure, not observed OOM. The receipt had `mutation_started=false`,
`recreation_started=false`, and an empty completed-image map. Read-only
container checks matched the accepted identities from `20260902T025246574Z`.
The user explicitly approved repairing/resealing with required build-time
dependency downloads; no model downloads or data deletion were approved.

## Revised boundary

- Identity: `NX-MMR-SHARED-P1-CORRECTION-ACTIVATION-V1-BUILD-DEPS-R1`.
- Build/recreate only `api` and `forensic-records-api`; worker, PostgreSQL and
  NATS remain exact-identity protected.
- Build RUN networking: `default`. This is not an egress-domain firewall.
  The acquisition scope is the reviewed Dockerfile/Makefile dependency path.
- Build `--pull=false` avoids forced base refresh; it does not guarantee no
  registry metadata requests or no retrieval of a missing build base.
- Runtime recreation remains `--no-deps --no-build --pull never`, with captured
  runtime environment/mounts and worker offline-model settings unchanged.
- API compilation gets `LOCALAI_BUILD_GOMAXPROCS=4` and
  `LOCALAI_BUILD_GOFLAGS=-p=2`. Builds remain sequential by service; BuildKit
  may still execute independent stages in parallel. RAM availability is not
  guaranteed by these limits. The mandatory 6-GiB gate is unchanged.
- Existing model/data volumes are not build mounts. No model acquisition,
  runtime package installation, prune, evidence rewrite or volume deletion.

The observed missing runtime-image packages were `ca-certificates`, `curl`,
`wget`, `espeak-ng`, `libgomp1`, `ffmpeg`, `libopenblas0`, `libopenblas-dev`,
`libopus0`, and `sox`. Subsequent reviewed build stages may also need Ubuntu
compiler/development packages, CMake, the declared Go toolchain, protoc and Go
generators/modules, and package-lock-declared npm dependencies. No host package
installation is introduced. Existing inference defaults are now sealed so their
absence cannot silently trigger the Makefile's generation fallback.

The Node engine warnings in the failed log were not its terminating error.
No dependency upgrades or Node base changes are included in this bounded repair.

## Receipt fix

Windows PowerShell 5.1 cannot assign a new property directly on an object read
by ConvertFrom-Json. Verification/rollback previously did this for outcome
fields absent from the initial receipt. Both now use a tested Add-Member
writer; JSON round trips, existing receipt fields and repeat writes are covered.

## Validation and limits

- Windows PowerShell 5.1 bundle tests: 42/42 PASS (static and behavioral).
- Source-seal validation: PASS; all prior unaffected sealed inputs unchanged.
- Actual generated two-service Compose build configuration: PASS, network
  `default`, API build concurrency arguments present.
- Read-only preflight: all non-RAM gates PASS, active jobs 0; RAM blocked at
  5.233444/6 GiB; disk 1643.229 GiB; retained baseline unchanged.
- No build, dependency download, activation, service restart, migration,
  evidence processing or volume mutation performed during this repair.
- Full network dependency resolution, image compilation, runtime activation
  and live rollback remain unexecuted. Static tests do not prove those outcomes.

## Seal and candidate identity

Manifest SHA-256:
`12934f66bd3b4f806163c9ed92e8f8bb6b52d51739de0c6a0996b461a01b4c0f`

Integrity seal SHA-256:
`5fb56c7bf1d5d6f84d3849bfa3aa322cf015d5c5a917e72c2aa4a83b42ddebbc`

Candidate tags:

- `nexusai/localai-forensic:nxmmr-shared-p1-correction-v1-build-deps-r1`
- `nexusai/forensic-records-api:nxmmr-shared-p1-correction-v1-build-deps-r1`

Changed manifest/seal hashes reject old receipts for candidate-image resume.
The original failed receipt and accepted runtime baseline are preserved.

## Exact next command

Close optional apps, keep Docker Desktop and internet available, then run in
standalone PowerShell. Do not start uploads/Ask/jobs during the operation.

```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_nxmmr_p1_correction_activation.ps1 -WaitForRamMinutes 120
```

Type YES after preflight PASS. Do not reopen high-memory apps while it runs.
The same command path now selects the revised sealed bundle and a fresh
receipt. If the RAM gate cannot pass, stop rather than lower it. After
`P1_CORRECTION_ACTIVATION_VERIFICATION=PASS`, rerun only failed live cells;
no product certification or NX-B2 promotion is made here.
