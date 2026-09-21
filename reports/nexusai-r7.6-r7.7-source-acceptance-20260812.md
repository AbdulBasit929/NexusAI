# NexusAI R7.6 and R7.7 source acceptance

Date: 2026-08-12  
Disposition: source accepted; not deployed  
Active next gate: R7.8 protected-data approval; R7.9 synthetic/read-only hardening may be prepared without bypassing that gate

## Accepted scope

- Ask NexusAI renders typed telecom timelines and supplied-coordinate plots,
  not label-only visualization placeholders.
- Selecting an observation synchronizes the timeline/map state with a bounded
  evidence drawer containing time, coordinates, datum/uncertainty and source
  file/row/hash locator.
- Tower CDR joins retain `matched`, `ambiguous_overlapping_references`,
  `unmatched_no_reference` and `unmatched_outside_validity_window` states.
- Missing coordinates render an explicit no-points state. No route line, RF
  coverage, device presence or subscriber presence is inferred.
- Catalog `2026-08-12.r7.7-source` and profile manifests reconcile the primary
  analyst plus CDR/subscriber/tower specialists at version 1.2. Specialists are
  case-bound, citation-required and cannot delegate laterally.
- The existing Qwen explanation role is reused unchanged and remains optional
  after deterministic computation.

## Verification

| Check | Result |
| --- | --- |
| `go test ./api/forensic_records` | PASS: 248 specs, 246 run, 2 intentional skips |
| full ingestion Python suite | PASS: 80 tests, 2 intentional skips |
| focused `core/services/agents` presentation contract | PASS |
| changed Agent Chat/e2e ESLint | PASS |
| React production build | PASS: 669 modules |
| source preview shell | PASS: active app route loaded; no shell-level horizontal overflow observed |
| deterministic telecom Playwright scenario | PASS 1/1 in Chromium against the production source preview, including 390 px overflow |
| full `core/services/agents` package | 97 specs pass; 13 scheduler/store setup failures are Windows testcontainer environment failures (`rootless Docker is not supported on Windows`) |

## Non-actions and retained gates

No container rebuild/deploy, retained-case mutation, migration, protected CDR
ingest, model download/change, agent-profile application, stage, commit, push or
publication was performed. R7.8 requires explicit protected-data authorization,
named case/tenant, retention/access policy and an audit-only versus retained-
ingest decision. R7.10 remains the separate guarded runtime/manual gate.

## Remaining R7 slices

1. **R7.8 — approval-gated protected CDR validation:** privacy-safe metadata and
   read-only normalization audit first; retained ingest only under separate,
   explicit authorization.
2. **R7.9 — performance/security/provenance:** large-result bounds, stable
   pagination, case/tenant negative tests, source-locator sampling, visualization
   caps and latency/resource evidence. This can proceed without protected data.
3. **R7.10 — runtime/manual acceptance:** guarded sequential rebuild, health and
   contract verification, profile application, product-owner UI pack, rollback
   proof and R7 closure.

## Safe rebuild path when explicitly approved

The R7.6 UI and LocalAI agent presentation path requires the LocalAI/UI image;
R7.7 catalog/API changes require the forensic API image. The worker has no new
R7.6/R7.7 code but the existing all-service activation gate rebuilds it for
image consistency. Use the proven sequential gate only after reviewing the
dirty worktree and obtaining explicit deployment approval:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
docker info
Get-CimInstance Win32_OperatingSystem | Select-Object @{Name='FreeRAMGiB';Expression={[math]::Round($_.FreePhysicalMemory / 1MB, 2)}}
Get-ChildItem .env.forensic-runtime.local
powershell -ExecutionPolicy Bypass -File .\scripts\build_deploy_forensic_phase6_gate.ps1
```

That gate is the currently proven rollback-preserving full activation path: it
checks Docker and RAM, tags rollback images, builds sequentially, preserves
named volumes, starts dependencies in order and verifies health. Do not add
`--remove-orphans`, `down -v`, volume deletion or image pruning. After a PASS,
apply/verify profiles and run the phase smoke/manual pack as a separately
reviewed R7.10 step:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\bootstrap_forensic_specialists.ps1 `
  -CollectionId 'nexusai-forensic-demo' -TenantId 'default' `
  -Model 'qwen_qwen3-4b-instruct-2507' -Force
powershell -ExecutionPolicy Bypass -File .\scripts\smoke_forensic_agent_chat_full_matrix.ps1 `
  -CollectionId 'nexusai-forensic-demo'
powershell -ExecutionPolicy Bypass -File .\scripts\smoke_forensic_agent_llm_matrix.ps1 `
  -CollectionId 'nexusai-forensic-demo'
```

Profile application mutates LocalAI agent configuration, so run it only after
the rebuild is healthy and the same explicit runtime-activation approval covers
that step. Do not treat image health alone as R7 closure.

## Activation preflight correction — 2026-08-12

The first guarded rebuild attempt failed closed before tagging, stopping,
building or recreating a service. Compose required
`NEXUSAI_AGENT_HISTORY_DATABASE_URL`, while the combined activation gate had
only validated the protected runtime file and had not exported this supported
alias. The shared runtime helper and combined gate now resolve that retained
URL without console disclosure and before the first Compose command. Static
PowerShell parsing, a redacted resolver unit check and read-only Compose
interpolation all pass. Re-run the same guarded command above; no destructive
cleanup flag or manual secret copy is required.
