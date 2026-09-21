# NexusAI APF-3.1–3.3 source acceptance — 2026-08-17

## Decision

APF-3.1, APF-3.2 and APF-3.3 are source accepted. Live/runtime acceptance is
pending an operator-authorized guarded deployment and browser test.

## Accepted boundary

- Versioned `forensics.query-understanding/v1` and
  `forensics.execution-plan/v1` contracts adapt the existing router.
- One projection derives all 65 executable capabilities from the authoritative
  query templates and resolves workspace data, authorization, runtime/model,
  maturity, specialist, result, presentation and citation state.
- One validated read-only step dispatches only the registered existing
  deterministic, retrieval or hybrid implementation.
- The platform catalog retains 50 historical descriptors and derives 30
  missing query descriptors: 80 total, with 65/65 executable parity.

## Verification

- APF Go test events: 26 pass, 0 fail, 0 skip.
- Forensic package: pass.
- Ginkgo: 258 pass, 0 fail, 2 skip (258 of 260 specs run).
- `go vet ./api/forensic_records`: pass.
- Deterministic planning: 1,000 understanding passes in 971.0366 ms.
- Race mode: not run; the Windows Go environment reports that `-race` requires
  CGO and CGO is disabled.

Security cases cover unknown/invalid capability IDs, unauthorized capability,
workspace widening, missing/processing evidence, runtime/model/maturity gates,
missing required parameters, arbitrary SQL/URL/tool IDs, multiple steps and
disabled citations. English, Urdu and Roman Urdu legacy routing is retained.

## Mutation and deployment state

No migration, retained upload/query/reprocess, model, worker, UI redesign,
R8/CCPD action or deployment occurred. The backend source changed, therefore
deployment is required before manual/live acceptance. The verified authoritative
gate is `scripts/build_deploy_nexusai_r8_ui_api_gate.ps1`; it has read-only
preflight, rollback images, automatic rollback, named-volume preservation and
does not rebuild the worker or change models/profiles.

APF-3.4 is next and remains not started.

## Guarded activation attempt

The later operator-run activation passed preflight and built the APF-3 API
image, but did not emit the deployment PASS marker. The API health connection
closed unexpectedly and automatic rollback restored the prior API and LocalAI
images. Read-only inspection then showed PostgreSQL, NATS and the worker exited
with code 255; the rolled-back API exited because `forensic-nats` could not be
resolved. Live acceptance therefore remains pending. Volumes were not removed;
recovery is limited to restarting the existing dependency services before
rerunning the guarded gate.
