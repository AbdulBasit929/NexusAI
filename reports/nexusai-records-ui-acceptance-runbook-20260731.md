# NexusAI Records Analyst UX, Rebuild, Logs, and Acceptance Runbook

Date: 2026-07-31  
Platform: Windows PowerShell and Docker Desktop  
Repository: `C:\Users\sheik\Workspace\Office\Projects\NexusAI`

## Acceptance target

This runbook activates and verifies the post-Phase-6.3 Records redesign. The
required result flow is:

1. a plain-language analyst answer;
2. four decision KPIs;
3. source-derived visual analysis where supported;
4. cited deterministic findings;
5. an eight-row exact preview;
6. recommended next checks;
7. collapsed evidence limits;
8. secondary views for all rows, timeline, evidence, and audit details.

The page must never show an empty `Value: {}` block, duplicate limitation
sidebars, fabricated map points, inferred ownership, or an operation ID parsed
as an ANPR target.

## 1. Restart and minimum-memory preparation

Restart Windows. Start only Docker Desktop and one PowerShell window. Wait for
Docker Desktop to report that the engine is running, then run:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force

$deadline = (Get-Date).AddMinutes(3)
do {
  Start-Sleep -Seconds 3
  docker info --format '{{.ServerVersion}}' 2>$null
  $dockerReady = $LASTEXITCODE -eq 0
} until ($dockerReady -or (Get-Date) -gt $deadline)
if (-not $dockerReady) { throw 'Docker Desktop was not ready within three minutes.' }

$os = Get-CimInstance Win32_OperatingSystem
$freeRAMGiB = [math]::Round($os.FreePhysicalMemory / 1MB, 2)
"Free physical RAM: $freeRAMGiB GiB"
if ($freeRAMGiB -lt 6) { throw 'Close other applications and retry; 6 GiB is mandatory.' }
```

Do not run `docker system prune --volumes`. Do not delete the `.env` file,
models, images, containers, or named volumes. The repository `.dockerignore`
excludes `.tmp`, `.cache`, `.phase2-backups`, and nested `node_modules`.
It intentionally includes `.git` because the upstream LocalAI Dockerfile copies
Git metadata to stamp the build version and commit. The gate now fails during
preflight—before stopping services—if Dockerfile and `.dockerignore` disagree.
The observed correct root context was about 63.5 MB; multi-gigabyte scratch
directories must remain excluded.

## 2. Validate compose and capture one complete transcript

```powershell
docker compose -p nexusai --env-file .env.forensic-runtime.local `
  -f .\docker-compose.forensic-records.yaml `
  -f .\docker-compose.forensic-records.runtime.yaml config --quiet
if ($LASTEXITCODE -ne 0) { throw 'Forensic compose validation failed.' }

docker compose -p nexusai --env-file .env.forensic-runtime.local `
  -f .\docker-compose.yaml `
  -f .\docker-compose.forensic-runtime.localai.yaml config --quiet
if ($LASTEXITCODE -ne 0) { throw 'LocalAI compose validation failed.' }

$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$transcriptPath = Join-Path $PWD "reports\runtime-activation-20260731\records-rebuild-$stamp.transcript.log"
Start-Transcript -LiteralPath $transcriptPath -Force
try {
  & powershell.exe -NoProfile -ExecutionPolicy Bypass `
    -File '.\scripts\build_deploy_forensic_phase6_gate.ps1'
  if ($LASTEXITCODE -ne 0) { throw "Phase 6 gate failed with exit code $LASTEXITCODE" }
} finally {
  Stop-Transcript
}
"Transcript: $transcriptPath"
```

The final terminal line must begin with `Phase6Activation=PASS`. The gate builds
API, worker, and LocalAI/UI sequentially, streams logs, preserves rollback
images and volumes, checks four health surfaces, and writes
`reports\runtime-activation-20260731\phase6.3-live-activation.json`.

## 3. Watch the build and runtime logs

Open a second PowerShell window and use one command at a time. `Ctrl+C` stops
only the log follower.

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

Get-Content '.\reports\runtime-activation-20260731\phase6-api-build.log' -Tail 100 -Wait
Get-Content '.\reports\runtime-activation-20260731\phase6-worker-build.log' -Tail 100 -Wait
Get-Content '.\reports\runtime-activation-20260731\phase6-localai-build-attempt-1.log' -Tail 100 -Wait
```

After deployment, follow application logs:

```powershell
docker compose -p nexusai --env-file .env.forensic-runtime.local `
  -f .\docker-compose.forensic-records.yaml `
  -f .\docker-compose.forensic-records.runtime.yaml `
  logs -f --tail 200 forensic-records-api forensic-records-worker forensic-nats
```

In another window, LocalAI/UI logs are:

```powershell
docker compose -p nexusai --env-file .env.forensic-runtime.local `
  -f .\docker-compose.yaml `
  -f .\docker-compose.forensic-runtime.localai.yaml `
  logs -f --tail 200 local-ai
```

Read-only resource checks:

```powershell
docker stats --no-stream
Get-CimInstance Win32_OperatingSystem |
  Select-Object @{n='FreeRAMGiB';e={[math]::Round($_.FreePhysicalMemory / 1MB, 2)}}
```

## 4. Health, image, and discovery gates

```powershell
$healthURLs = @(
  'http://localhost:8080/readyz',
  'http://localhost:8091/healthz',
  'http://localhost:9109/metrics',
  'http://localhost:8222/healthz'
)
foreach ($url in $healthURLs) {
  $response = Invoke-WebRequest -UseBasicParsing -TimeoutSec 15 -Uri $url
  if ($response.StatusCode -ne 200) { throw "$url returned $($response.StatusCode)" }
  "PASS $url"
}

Get-Content -Raw '.\reports\runtime-activation-20260731\phase6.3-live-activation.json'
docker ps --format '{{.Names}}|{{.Status}}|{{.Image}}|{{.ID}}'

$adapters = Invoke-RestMethod 'http://localhost:8080/api/v1/forensics/adapters'
$operations = Invoke-RestMethod 'http://localhost:8080/api/v1/forensics/operations'
$agents = Invoke-RestMethod 'http://localhost:8080/api/v1/forensics/agents'
$contracts = Invoke-RestMethod 'http://localhost:8080/api/v1/forensics/contracts'
$cases = Invoke-RestMethod 'http://localhost:8080/api/v1/forensics/cases'

@{
  AdapterCount = @($adapters.adapters).Count
  OperationCount = @($operations.operations).Count
  SpecialistCount = @($agents.agents).Count
  CaseCount = @($cases.cases).Count
} | ConvertTo-Json
```

Required: catalog `2026-07-31.phase6.3`, four adapters, 34 catalog operations,
all four discovery endpoints returning HTTP 200 through LocalAI, and
`records-demo-verified` selectable. The CDR, IPDR, and ANPR specialists must be
operational and bound to that governed case.

## 5. Deterministic API operation matrix

Use this helper. It fails immediately on contract, operation, adapter, or target
leakage errors and saves each complete response for audit.

```powershell
$case = 'records-demo-verified'
$resultDir = Join-Path $PWD "reports\runtime-activation-20260731\records-api-$stamp"
New-Item -ItemType Directory -Path $resultDir -Force | Out-Null

function Invoke-ForensicOperation {
  param(
    [Parameter(Mandatory)][string]$Operation,
    [Parameter(Mandatory)][string]$Family,
    [string]$EntityType = '',
    [string]$Target = ''
  )
  $entities = @()
  if ($Target) { $entities = @(@{ type = $EntityType; value = $Target }) }
  $body = @{
    contract_version = 'forensics.query-plan/v1'
    tenant_id = 'default'
    case_id = $case
    collection_id = $case
    intent = $Operation
    families = @($Family)
    entities = $entities
    time_range = @{ timezone = 'Asia/Karachi' }
    limit = 25
  } | ConvertTo-Json -Depth 10

  $result = Invoke-RestMethod -Method Post -ContentType 'application/json' `
    -Uri "http://localhost:8080/api/v1/forensics/cases/$case/query" -Body $body
  if ($result.contract_version -ne 'forensics.enterprise-response/v1') {
    throw "$Operation returned the wrong contract"
  }
  if (@($result.execution_trace.tool_ids) -notcontains $Operation) {
    throw "$Operation is missing from execution trace"
  }
  $adapter = "nexusai.adapter.$($Family -replace '^communications_|^network_|_vehicles$','')"
  if (@($result.execution_trace.adapter_ids) -notcontains $adapter) {
    throw "$Operation did not report $adapter"
  }
  $safeName = $Operation -replace '[^a-zA-Z0-9.-]', '_'
  $result | ConvertTo-Json -Depth 40 |
    Set-Content -LiteralPath (Join-Path $resultDir "$safeName.json") -Encoding utf8
  [pscustomobject]@{
    Operation = $Operation
    Status = $result.status
    Rows = @($result.tables[0].rows).Count
    Tools = @($result.execution_trace.tool_ids) -join ','
  }
}

$results = @()
$results += Invoke-ForensicOperation 'cdr.frequent_contacts' 'communications_cdr'
$results += Invoke-ForensicOperation 'cdr.call_type_breakdown' 'communications_cdr'
$results += Invoke-ForensicOperation 'cdr.temporal_activity' 'communications_cdr'
$results += Invoke-ForensicOperation 'cdr.duration_extremes' 'communications_cdr'
$results += Invoke-ForensicOperation 'cdr.timeline' 'communications_cdr' 'msisdn' '923001234567'
$results += Invoke-ForensicOperation 'cdr.geospatial_movement' 'communications_cdr' 'msisdn' '923001234567'
$results += Invoke-ForensicOperation 'cdr.device_identity_changes' 'communications_cdr' 'msisdn' '923001234567'
$results += Invoke-ForensicOperation 'cdr.service_usage' 'communications_cdr' 'msisdn' '923001234567'
$results += Invoke-ForensicOperation 'cdr.tower_activity' 'communications_cdr' 'msisdn' '923001234567'

$results += Invoke-ForensicOperation 'ipdr.endpoint_summary' 'network_ipdr'
$results += Invoke-ForensicOperation 'ipdr.domain_summary' 'network_ipdr'
$results += Invoke-ForensicOperation 'ipdr.protocol_breakdown' 'network_ipdr'
$results += Invoke-ForensicOperation 'ipdr.session_volume' 'network_ipdr'
$results += Invoke-ForensicOperation 'ipdr.subscriber_sessions' 'network_ipdr' 'subscriber_identifier' '923001234567'
$results += Invoke-ForensicOperation 'ipdr.concurrent_sessions' 'network_ipdr' 'subscriber_identifier' '923001234567'
$results += Invoke-ForensicOperation 'ipdr.timeline' 'network_ipdr' 'subscriber_identifier' '923001234567'

$results += Invoke-ForensicOperation 'anpr.sightings' 'anpr_vehicles' 'plate' 'ABC-123'
$results += Invoke-ForensicOperation 'anpr.camera_sequence' 'anpr_vehicles' 'plate' 'ABC-123'
$results += Invoke-ForensicOperation 'anpr.camera_activity' 'anpr_vehicles'
$results += Invoke-ForensicOperation 'anpr.co_travel' 'anpr_vehicles' 'plate' 'ABC-123'
$results += Invoke-ForensicOperation 'anpr.route_timing' 'anpr_vehicles' 'plate' 'ABC-123'
$results += Invoke-ForensicOperation 'anpr.plate_variants' 'anpr_vehicles' 'plate' 'ABC-123'
$results += Invoke-ForensicOperation 'anpr.timeline' 'anpr_vehicles' 'plate' 'ABC-123'

$results | Format-Table -AutoSize
```

Empty results are acceptable for an exact target absent from retained evidence;
contract, trace, provenance, explicit limitation, and non-inference behavior are
still mandatory. `anpr.camera_activity` must run without a target and must not
contain `CAMERA_ACTIVITY` as an entity filter.

## 6. Sequential desktop UI acceptance

Open `http://localhost:8080/app/records` only after all API gates pass.

### A. First-load productivity

1. Confirm the Active Collection control lists every collection accessible to
   the current user, grouped as active investigations, legacy/demo, and retained
   validation/system collections. Selecting a retained collection must not
   authorize merge, archive, reset, or deletion.
2. Confirm status/readiness information is concise and the query composer is
   visible without traversing tabs.
3. Click **Discover exact case identifiers**. Confirm exact authorized values
   appear as selectable chips, while masked coverage hints are explicitly marked
   non-executable and never appear in the target datalist.
4. Confirm **Exact target** accepts a discovered phone, subscriber, IP,
   IMEI/IMSI, or plate and shows accepted identifier kinds for the selected
   template.
5. Open advanced options, set a start and end time, run an operation, and verify
   the Audit request scope contains the same inclusive/exclusive time bounds.
6. Confirm template discovery is collapsed by default and opens by family:
   CDR, IPDR, ANPR, Operations, Timeline & Movement, Entities & Links, Evidence
   & Briefing, and Records Analytics.
7. Expand **Inputs & calculation** on one CDR, IPDR, and ANPR workflow. Confirm
   required inputs, measures, grouping, calculation, output meaning, example,
   and non-inference limits are readable before execution.

### B. Result presentation

1. Run **Camera activity** with Exact target empty.
2. Confirm the result automatically scrolls into view.
3. Confirm the heading is human-readable, not an operation ID.
4. Confirm the first view is **Analyst summary** and shows answer, exact-row,
   evidence, findings, and execution-time KPIs.
5. Confirm **Visual analysis** is visible and every plotted value is present in
   the exact result rows.
6. Confirm **Exact result preview** shows at most eight rows and provides a
   one-click path to all rows.
7. Confirm there is no `Value: {}`, no duplicate case-health sidebar, and no
   repeated limitation block.
8. Expand **Evidence limits and cautions** and verify ANPR non-inference text.
9. Verify CSV row export and JSON audit export download successfully.

### C. Target safety and family workflows

1. Clear Exact target and click a target-required Camera sequence chip. Confirm
   the UI asks for a target and does not send a query.
2. Enter `ABC-123`; run exact sightings, camera sequence, co-observations, route
   timing, plate variants, and timeline.
3. Confirm route timing labels distance as straight-line and never draws or
   claims a road route.
4. Enter `923001234567`; run CDR timeline, geospatial movement, device changes,
   service usage, tower activity, and IPDR subscriber/concurrent/timeline
   operations.
5. Run target-free CDR call-type/temporal/duration and IPDR endpoint/domain/
   protocol/session-volume templates.
6. For every result, check All records search/sort, Timeline ordering, Evidence
   locators, and Audit details tool/adapter/coverage fields.

### D. Responsive and accessibility pass

1. Test at 1440x900, 1024x768, and 390x844 browser viewports.
2. Confirm the command composer, result tabs, KPIs, charts, and tables do not
   overlap or clip; tables may scroll horizontally on mobile.
3. Navigate the query, target, action chips, result tabs, disclosure, and export
   controls with Tab/Shift+Tab and activate them with Enter/Space.
4. Confirm focus remains visible and status/result updates are announced by the
   live region.
5. Open browser developer tools: the Console must have no red application
   errors and Network requests must not return 4xx/5xx.

### E. Collections page

1. Open `http://localhost:8080/app/collections`.
2. Confirm `records-demo-verified` is the only routine analyst collection.
3. Confirm retained collections are represented by a hidden-count notice.
4. Use **Review retained system collections** and verify all 26 collections can
   still be audited.
5. Do not press Reset/Delete during acceptance.

## 7. Source regression commands

These do not rebuild Docker and can be run before or after UI acceptance:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
$env:GOCACHE = (Resolve-Path '.tmp\phase6.3-go-cache').Path
$env:GOTMPDIR = (Resolve-Path '.tmp\phase6.3-go-tmp').Path

go run github.com/swaggo/swag/cmd/swag@v1.16.6 init `
  -g core/http/app.go --output swagger
go test .\api\forensic_records -count=1
go vet .\api\forensic_records

Push-Location '.\core\http\react-ui'
npm.cmd exec eslint -- src/pages/RecordsIntelligence.jsx src/pages/Collections.jsx e2e/records-intelligence.spec.js
npm.cmd run build
Pop-Location

git diff --check
```

The focused Playwright specification is authored at
`core/http/react-ui/e2e/records-intelligence.spec.js`. Run it only when the
repository-pinned Chromium is installed:

```powershell
Push-Location '.\core\http\react-ui'
npx.cmd playwright test e2e/records-intelligence.spec.js --reporter=line
Pop-Location
```

Do not claim browser automation passed if Playwright reports that its pinned
headless-shell executable is missing.

## 8. Failure handling and handoff

If the rebuild gate fails, preserve the transcript and stage logs. The gate
automatically attempts rollback; verify the four health URLs and do not delete
volumes. If only Go-module DNS fails, verify Docker DNS before retrying:

```powershell
docker run --rm alpine:3.22 nslookup proxy.golang.org
docker run --rm alpine:3.22 wget -q -S -O NUL https://proxy.golang.org
```

Return the `Phase6Activation=PASS` line, activation JSON, API operation table,
and any failed UI step or response JSON to Codex. These artifacts are sufficient
to diagnose the exact gate without repeating the whole rebuild.
