# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([string]$RunDirectory='', [switch]$Internal)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$run=''
try {
    . (Join-Path $PSScriptRoot 'nxmmr_demo_operator_common.ps1')
    $run=Get-NxRunDirectory $RunDirectory; $manifest=Get-NxManifest
    $state=Get-Content -LiteralPath (Join-Path $run 'state.json') -Raw | ConvertFrom-Json
    if($state.state -notin @('RECREATION_STARTED','VERIFIED')){throw 'No admitted recreation to verify.'}
    if($state.manifest_sha256 -ne (Get-NxHash $script:NxManifestPath)){throw 'Manifest differs from activation receipt.'}
    $before=Get-Content -LiteralPath (Join-Path $run 'container-before.json') -Raw | ConvertFrom-Json
    $containers=Wait-NxHealth $manifest.operator_bundle.health_timeout_seconds
    Assert-NxProtected $before $containers
    $worker=$containers['forensic-records-worker']
    if($worker.Image -cne $state.expected_image){throw 'Worker image is not the built immutable ID.'}
    if($worker.RestartCount -ne 0){throw 'New worker restarted; refusing readiness PASS.'}
    $envMap=@{}; foreach($entry in $worker.Config.Env){$p=$entry -split '=',2;$envMap[$p[0]]=$p[1]}
    foreach($p in $manifest.worker_environment.PSObject.Properties){if($envMap[$p.Name] -cne $p.Value){throw "Worker role/config mismatch: $($p.Name)"}}
    $null=Get-NxAssets $manifest -DestinationRequired
    $probe=[Convert]::ToBase64String([IO.File]::ReadAllBytes((Join-Path $PSScriptRoot 'nxmmr_demo_readiness_probe.py')))
    $code="import base64; exec(base64.b64decode('$probe'))"
    $text=Invoke-NxNative docker @('exec',$worker.Id,'python','-c',$code) 240 (Join-Path $run 'readiness-probe.log')
    $line=@($text -split '\r?\n' | Where-Object {$_ -like 'NXMMR_READINESS_JSON=*'})
    if($line.Count -ne 1){throw 'Readiness probe returned no unique receipt.'}
    $readiness=$line[0].Substring('NXMMR_READINESS_JSON='.Length) | ConvertFrom-Json
    if($readiness.state -ne 'PASS'){throw 'Model-load readiness failed.'}
    Write-NxJson (Join-Path $run 'role-readiness.json') $readiness
    Start-Sleep -Seconds 10
    $after=Get-NxContainers; Assert-NxHealth $after; Assert-NxProtected $before $after
    if($after['forensic-records-worker'].Id -ne $worker.Id -or $after['forensic-records-worker'].RestartCount -ne 0){throw 'Worker identity/restart changed during stability window.'}
    if((Get-NxJobs $after) -ne 0){throw 'Unexpected active jobs; do not claim readiness.'}
    Write-NxJson (Join-Path $run 'container-after.json') (Get-NxSummary $after)
    Write-NxJson (Join-Path $run 'health.json') @{state='PASS';utc=[DateTime]::UtcNow.ToString('o');services=(Get-NxSummary $after);active_jobs=0;browser_acceptance='PENDING';deployment_ram_gate_complete=$true}
    Write-NxJson (Join-Path $run 'state.json') @{state='VERIFIED';recreation_started=$true;expected_image=$worker.Image;manifest_sha256=(Get-NxHash $script:NxManifestPath)}
    Write-Host 'ACTIVATION_VERIFICATION=PASS'
    Write-Host "Receipt: $run"
    Write-Host 'You may reopen Chrome/Codex. Browser acceptance does not require the 6-GiB build floor.'
} catch {
    Write-Host "ACTIVATION_VERIFICATION=FAILED reason=$($_.Exception.Message)"
    if($run -and (Test-Path -LiteralPath $run)){Write-NxJson (Join-Path $run 'verification-failure.json') @{state='FAILED';reason=$_.Exception.Message;utc=[DateTime]::UtcNow.ToString('o')}}
    Write-Host 'If recreation occurred, run .\scripts\rollback_nxmmr_demo_activation.ps1; preserve private logs.'
    if($Internal){throw}
    exit 4
}
