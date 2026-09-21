# NexusAI Phase 6.4 Analyst Workspace Source Acceptance

Date: 2026-08-03  
Status: source accepted; one guarded rebuild and focused live UI gate required

## Outcome

This slice corrects the product-contract defects exposed by the Phase 6.3 live
screenshots. It does not add decorative UI around incorrect data. It makes the
collection catalog, evidence registry, deterministic agent routing, and analyst
workspace agree on one authoritative case/collection scope.

Implemented outcomes:

- every collection accessible to the current user is available from the Case
  Workspace and forensic Agent Chat when `include_system=true` is requested;
- active, legacy/demo, and retained validation/system collections remain visibly
  classified and retain their non-deletion governance warnings;
- operations are capability-aware: a selected collection is described as hybrid
  ready, exact-records ready, knowledge-only, processing, or inventory-only;
- knowledge-only or empty collections do not advertise unsupported CDR, IPDR, or
  ANPR specialist shortcuts;
- pre-case-contract evidence rows with an empty `case_id` are visible in their
  exact collection-bound v1 case, while rows explicitly bound to another case
  remain excluded;
- the production sidebar renders a native NexusAI wordmark for the default
  product identity and uses the configured served branding asset for custom
  identities, instead of a source-only `/brand` path that returned HTTP 404;
- the Analyze surface removes the redundant embedded page title and three-step
  development walkthrough, limits the primary workflow shortcuts, and keeps
  templates/data management secondary;
- Relationships and Jobs use compact, evidence-safe action/status layouts instead
  of full-height placeholder screens;
- Settings presents human-readable governance and resource labels rather than raw
  underscore identifiers;
- Agent Chat suggestions are derived from live collection capabilities;
- case readiness, CDR, IPDR, and ANPR analyst questions are regression-tested to
  stay on the fast deterministic route rather than silently falling through to a
  slow synthesis model;
- an empty evidence catalog now returns a clear governed no-data answer instead of
  an internal formatter fallback.

No collection, KB entry, evidence object, database row, model, volume, or Docker
image was deleted or reset in this slice.

## Source verification completed

- `go test ./api/forensic_records -run "Test(Evidence|ClampEvidence)" -count=1`
  passed.
- `go test ./core/services/agents -run
  "Test(DeterministicForensicRouteCoverage|EmptyEvidenceCatalogHasGovernedAnalystSummary)"
  -count=1` passed.
- ESLint passed with zero errors for `Sidebar.jsx`, `CaseWorkspace.jsx`,
  `RecordsIntelligence.jsx`, and `AgentChat.jsx`.
- `npm.cmd run build` passed: 658 modules transformed.
- `git diff --check` passed for the bounded Phase 6.4 files.
- Source-preview browser acceptance passed at 390, 820, 1024, and 1440 pixels:
  zero page-level horizontal overflow, zero broken images, and zero browser
  console warnings/errors. Overview, Evidence, Analyze, Relationships, and Jobs
  were inspected against the running backend; the embedded Analyze view exposes
  six primary presets and no redundant ANPR hero panel.
- The broad LocalAI endpoint package exceeded the bounded Windows link/test time;
  this is recorded as not concluded, not represented as a pass.
- Live pre-rebuild Agent Chat proved source-audit routing completed through
  `forensic_hybrid_query` with a 1,845-character response. The deployed image
  allowed a case-readiness prompt to fall through to the local model and remain
  processing beyond 55 seconds. The new focused regression initially reproduced
  that defect and passed after the routing fix.

The current running image does not include this Phase 6.4 source. Do not judge the
new UI from the old browser tabs until the guarded rebuild succeeds.

## One guarded rebuild

After a laptop restart, start Docker Desktop and wait until the engine reports
`Engine running`. Open one ordinary Windows PowerShell window:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

$deadline = (Get-Date).AddMinutes(5)
do {
    $dockerVersion = docker info --format '{{.ServerVersion}}' 2>$null
    $dockerReady = $LASTEXITCODE -eq 0
    if (-not $dockerReady) { Start-Sleep -Seconds 3 }
} until ($dockerReady -or (Get-Date) -gt $deadline)
if (-not $dockerReady) { throw 'Docker Desktop was not ready within five minutes.' }
"Docker ready: $dockerVersion"

$os = Get-CimInstance Win32_OperatingSystem
$freeGiB = [math]::Round($os.FreePhysicalMemory / 1MB, 2)
"Free RAM before gate: $freeGiB GiB"

$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File '.\scripts\build_deploy_forensic_phase6_gate.ps1' `
  2>&1 | Tee-Object -FilePath ".\reports\phase6.4-rebuild-$stamp.log"
```

The gate owns sequential service shutdown, the 6 GiB memory wait, build logs,
health checks, rollback, and activation. Do not manually remove containers,
images, volumes, or BuildKit cache. Required terminal marker:

```text
Phase6Activation=PASS
```

If the marker is absent, stop and preserve the transcript. Do not continue to UI
acceptance and do not delete anything.

## Focused post-rebuild API gate

```powershell
$base = 'http://localhost:8080'

$cases = Invoke-RestMethod "$base/api/v1/forensics/cases?include_system=true"
if (@($cases.cases).Count -lt 2) { throw 'Accessible collection catalog is still restricted.' }
if (@($cases.cases | Where-Object { -not $_.selectable }).Count -ne 0) {
    throw 'An accessible collection is not selectable.'
}
$cases.cases |
    Select-Object display_name, collection_id, environment, visibility, case_status |
    Format-Table -AutoSize

$status = Invoke-RestMethod "$base/api/records/forensic/status?collection_id=records-demo-verified"
$evidence = Invoke-RestMethod "$base/api/v1/forensics/cases/records-demo-verified/evidence?limit=100"
if ([int]$evidence.summary.evidence_total -ne [int]$status.summary.evidence_total) {
    throw "Evidence registry/status mismatch: $($evidence.summary.evidence_total) vs $($status.summary.evidence_total)"
}
if (@($evidence.items).Count -eq 0) { throw 'Verified case evidence registry is unexpectedly empty.' }

$branding = Invoke-RestMethod "$base/api/branding"
$logo = Invoke-WebRequest -UseBasicParsing ($base + $branding.logo_horizontal_url)
if ($logo.StatusCode -ne 200 -or $logo.Headers['Content-Type'] -notmatch '^image/') {
    throw 'Configured horizontal logo is not a valid served image.'
}

@{
    AccessibleCollections = @($cases.cases).Count
    VerifiedEvidence = @($evidence.items).Count
    EvidenceTotal = $evidence.summary.evidence_total
    LogoStatus = $logo.StatusCode
} | Format-List
```

## Sequential analyst UI gate

Use Chrome at `http://localhost:8080/app/cases/records-demo-verified/overview`.
Keep DevTools Console open and preserve screenshots of each numbered checkpoint.

1. **Brand and command bar**
   - NexusAI logo renders; no broken-image icon appears.
   - Active Collection is a real grouped selector and reports the accessible
     count.
   - Selectors contain active, legacy/demo, and retained validation/system groups.

2. **Verified hybrid collection**
   - Select `records-demo-verified`.
   - Readiness is `Hybrid ready`.
   - Evidence, 9,250 queryable rows, KB assets, and live family coverage agree
     with the API.

3. **Knowledge-only or retained collection**
   - Select one retained collection containing KB entries but no normalized rows.
   - The workspace reports `Knowledge-only` and explicitly says exact record
     operations require normalization.
   - Analyze does not show irrelevant CDR/IPDR/ANPR specialist shortcuts.
   - A cited KB question is allowed; an unsupported exact-record question must
     return a typed unavailable/limitation result, never fabricated rows.

4. **Evidence registry**
   - Return to `records-demo-verified`, open Evidence, and confirm the registered
     sources are visible immediately.
   - Search and Status filters work independently and together.
   - `Add evidence` is collapsed by default and expands without changing case
     scope. Do not upload during this read-only gate.

5. **Analyze**
   - The first viewport contains one analysis desk, one natural-query composer,
     exact target input, and no redundant page walkthrough.
   - Only family shortcuts supported by the selected case are shown.
   - Run `show CDR call type breakdown`, `show network protocol breakdown`, and
     `show ANPR camera activity`.
   - Each result must show a readable summary, operating metrics, a bounded data
     grid, provenance/source locators, limitations, route `records_sql`, and the
     correct template. No raw JSON wall is acceptable.

6. **Relationships, Jobs, Settings**
   - Relationship workflows appear as four compact cards with evidence-safe
     explanations; no full-height empty canvas appears before a query.
   - Jobs shows completed/in-progress/failed/rejected KPIs plus compact history.
   - Settings labels are human-readable and the technical system-of-record value
     remains available for audit.

7. **Agent Chat**
   - Open `Forensic_Records_Analyst`, confirm the Active Case selector lists the
     same accessible collections, then choose `records-demo-verified`.
   - Ask `which files were ingested?` and `is this case ready for production?`.
     Both must complete through the deterministic forensic tool; they must not
     remain on `Working...` or load the explanation model for exact facts.
   - Repeat one family query in each specialist: CDR call type breakdown, IPDR
     protocol breakdown, and ANPR camera activity.
   - Confirm source citations, bounded limitations, and the selected case remain
     visible. Tool/reasoning trace stays collapsed unless opened.

8. **Responsive and accessibility**
   - Repeat Overview, Evidence, Analyze result, and Agent Chat at viewport widths
     390, 820, 1024, and 1440 pixels.
   - Required: zero page-level horizontal overflow, selector usable by keyboard,
     visible focus indicators, tables scroll only inside their own containers,
     and zero Console errors/warnings caused by these surfaces.

## Phase order after live acceptance

Do not start another family until this focused gate is accepted. The approved
bounded order is:

1. Phase 6.4 live acceptance: collection/evidence/agent/UI contract in this file.
2. Subscriber identity and tower/site intelligence vertical slice.
3. Financial transaction evidence family.
4. Access/security log family.
5. Generic tabular adapter and schema-governed analyst workflows.
6. Only then continue multimodal/document/media families and advanced graph/map
   renderers under the approved architecture.

Each slice must ship adapter detection, normalization, deterministic operations,
specialist prompt/model role, public contract, capability-aware UI, citations,
non-inference limits, unit/integration tests, and one focused live acceptance
before proceeding.
