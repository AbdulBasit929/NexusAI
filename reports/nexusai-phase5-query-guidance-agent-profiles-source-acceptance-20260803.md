# NexusAI Phase 5 query guidance and specialist profiles source acceptance

Date: 2026-08-03  
State: source accepted; operator rebuild and live UI/API acceptance still required

## Outcome

This bounded closure slice makes Records Intelligence executable and
explainable for analysts. It does not replace deterministic stores with model
reasoning and it does not claim the newly built source is live.

- The query-template API now publishes operation/family IDs, required and
  optional inputs, accepted identifier kinds, measures, grouping, calculation,
  example query, expected output, and operation-specific limitations.
- Records UI retrieves exact authorized identifiers through `entity_activity`,
  presents them as selectable target chips, and never treats masked coverage
  examples as executable values.
- Target-required templates fail in the browser before dispatch when no exact
  identifier is selected. Accepted target kinds adapt to the selected workflow.
- Family analysis accepts explicit inclusive start and exclusive end bounds.
  CDR contacts, type breakdown, temporal statistics, duration extremes, tower
  activity, and geospatial observation sequence now actually apply those bounds;
  IPDR and ANPR family scopes already apply them.
- Template cards use progressive disclosure: a concise workflow summary plus
  expandable inputs, measures, grouping, calculation, expected result, example,
  and inference limits.
- Four versioned source profiles define distinct system prompts, user-request
  contracts, family scopes, preferred synthesis-model role, permanent goals,
  examples, and prohibited inferences. One PowerShell command can reproducibly
  create/update all four profiles after deployment.

## Rebuild transcript finding

The supplied operator transcript did not complete activation. API and worker
images built successfully. LocalAI failed before compilation because the root
`.dockerignore` excluded `.git` while the upstream Dockerfile executes
`COPY ./.git ./.git`. After `.git` was manually made visible, the next attempt
failed correctly at the strict memory gate: 5.26 GiB free versus 6 GiB required.
There was no `Phase6Activation=PASS` line.

The source contract is now explicit:

- `.git` remains in the Docker context for LocalAI version/commit stamping;
- `.tmp`, `.cache`, `.phase2-backups`, and nested `node_modules` remain excluded;
- the observed valid context was approximately 63.5 MB;
- the guarded deploy checks the Dockerfile/ignore contract before stopping any
  running service and returns an actionable preflight error on mismatch.

## Specialist profiles

Source: `configuration/forensic_agent_profiles.json`

| Agent | Family | Synthesis role | Deterministic boundary |
|---|---|---|---|
| `Forensic_Records_Analyst` | cross-family orchestrator | `synthesis` / Qwen 4B preferred | routes and explains; exact operations remain authoritative |
| `Communications_CDR_Analyst` | CDR | `synthesis` / Qwen 4B preferred | no identity, ownership, content, relationship, home, or continuous-location inference |
| `Network_IPDR_Capture_Analyst` | IPDR | `synthesis` / Qwen 4B preferred | no ownership, DNS resolution, payload, geolocation, or maliciousness inference |
| `Vehicle_ANPR_Geospatial_Analyst` | ANPR/geospatial | `synthesis` / Qwen 4B preferred | no owner/driver/occupants, association, road route, or OCR correction inference |

Explicit embedding, reranker, ASR, audio, vision, or other non-chat model IDs are
rejected by the bootstrap script. If the preferred Qwen model is unavailable,
the script selects an installed chat/instruct model; if none exists, the agents
remain useful for deterministic forensic mode and report the missing synthesis
capability.

## Verification

- `go test ./api/forensic_records -count=1`: PASS (`16.434s` final test runtime after
  workspace-cache compilation)
- `go vet ./api/forensic_records`: PASS
- agent profile JSON parse: PASS
- PowerShell AST parse for guarded deploy and both bootstrap scripts: PASS
- targeted ESLint for Records page and E2E spec: zero errors; five existing
  unused-symbol warnings in the already modified Records page
- React production build: PASS, 658 modules, 1.16 seconds
- `git diff --check` for the bounded file set: PASS
- focused Playwright launch: not executed because the configured web-server
  process cannot start in this Windows sandbox (`The system cannot find the path
  specified`). The deterministic E2E scenario is committed to source and must be
  run in the operator environment after rebuild.

No Docker rebuild, model download, database migration, upload, reprocess,
collection deletion, agent mutation, staging, commit, push, or external action
was performed in this slice.

## Operator activation

After restart and with Docker Desktop plus PowerShell only:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$os = Get-CimInstance Win32_OperatingSystem
$freeGiB = [math]::Round($os.FreePhysicalMemory / 1MB, 2)
"Free RAM: $freeGiB GiB"
if ($freeGiB -lt 6) { throw 'At least 6 GiB free RAM is required.' }

powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File '.\scripts\build_deploy_forensic_phase6_gate.ps1'
```

Proceed only after the terminal prints `Phase6Activation=PASS`. Then apply the
versioned profiles to the governed case:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File '.\scripts\bootstrap_forensic_specialists.ps1' `
  -CollectionId 'records-demo-verified'
```

Run the UI/API sequence in
`reports/nexusai-records-ui-acceptance-runbook-20260731.md`. Physical deletion
of the ten verified empty collection candidates remains a separate destructive
approval; the routine analyst UI already exposes only the governed collection.

## Next phase

After live acceptance, Phase 6 continues with the next approved evidence-family
vertical slice. Do not expand to additional model-only functions until the
current CDR/IPDR/ANPR discovery, target selection, all deterministic operations,
specialist prompts, exports, responsive UI, and audit trace pass against the
retained governed case.
