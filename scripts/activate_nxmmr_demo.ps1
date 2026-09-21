# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$run=''; $recreation=$false; $transcript=$false; $mutex=$null
try {
    . (Join-Path $PSScriptRoot 'nxmmr_demo_operator_common.ps1')
    $mutex=Enter-NxOperatorLock
    $run=New-NxRunDirectory
    Start-Transcript -LiteralPath (Join-Path $run 'activation.log') | Out-Null; $transcript=$true
    Write-Host "ActivationReceiptDirectory=$run"
    $preflight=Invoke-NxPreflight
    Write-NxJson (Join-Path $run 'preflight.json') $preflight
    Show-NxPreflight $preflight
    if($preflight.status -ne 'PASS'){throw 'Preflight blocked; no assets, build or recreation performed.'}
    $manifest=Get-NxManifest; $before=Get-NxContainers; Assert-NxHealth $before
    $worker=$before['forensic-records-worker']
    if($worker.Id -cne $manifest.rollback.current_worker_container -or $worker.Image -cne $manifest.rollback.current_worker_image){throw 'Worker identity changed after preflight.'}
    $rollback=New-NxWorkerCompose $worker $worker.Image ([ordered]@{})
    $activation=New-NxWorkerCompose $worker $manifest.operator_bundle.build_image (ConvertTo-NxMap $manifest.worker_environment) -Build
    $rollbackFile=Join-Path $run 'rollback-compose.json'; $activationFile=Join-Path $run 'activation-compose.json'
    Write-NxJson $rollbackFile $rollback; Write-NxJson $activationFile $activation
    Write-NxJson (Join-Path $run 'container-before.json') (Get-NxSummary $before)
    $null=Invoke-NxReceiptCompose $rollbackFile @('config','--quiet')
    $null=Invoke-NxReceiptCompose $activationFile @('config','--quiet')
    Write-NxJson (Join-Path $run 'rollback.json') @{state='PREPARED';original_image=$worker.Image;original_container=$worker.Id;compose_sha256=(Get-NxHash $rollbackFile);scope=@('forensic-records-worker');prior_roles_restored_from_snapshot=$true}
    Write-NxJson (Resolve-NxPath 'local-acceptance-models/nxmmr/private-activation/latest.json') @{run_directory=$run}
    Assert-NxRam
    $assets=Copy-NxAssets $manifest
    Write-NxJson (Join-Path $run 'model-hashes.json') $assets
    # Recheck immediately before the first build. An earlier PASS is insufficient.
    Assert-NxRam
    Write-NxJson (Join-Path $run 'state.json') @{state='BUILD_STARTED';recreation_started=$false;manifest_sha256=(Get-NxHash $script:NxManifestPath)}
    Write-Host 'Building worker only. Live output appears here and is saved continuously to build.log. Package installation can require Internet; model downloads are forbidden.'
    $null=Invoke-NxReceiptCompose $activationFile @('--progress','plain','build','--pull=false','forensic-records-worker') $manifest.operator_bundle.build_timeout_seconds (Join-Path $run 'build.log') -StreamOutput
    $image=Invoke-NxNative docker @('image','inspect',$manifest.operator_bundle.build_image,'--format','{{.Id}}')
    # Freeze the result to its immutable ID before recreation; never chase a tag.
    $activation.services['forensic-records-worker'].image=$image
    Write-NxJson $activationFile $activation
    $null=Invoke-NxNative docker @('run','--rm','--pull','never','--network','none','--read-only','--entrypoint','python',$image,'-c','import importlib.util; assert importlib.util.find_spec("ultralytics") is None; import media_pipeline, multilingual_ocr, video_anpr_onnx_v3') 120 (Join-Path $run 'image-smoke.log')
    $seal=Get-Content -LiteralPath (Resolve-NxPath 'configuration/nxmmr_demo_operator_integrity.json') -Raw | ConvertFrom-Json
    $hashCode='import pathlib,hashlib,json; print(json.dumps({p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in pathlib.Path("/app").glob("*.py")}))'
    $imageHashes=(Invoke-NxNative docker @('run','--rm','--pull','never','--network','none','--read-only','--entrypoint','python',$image,'-c',$hashCode)) | ConvertFrom-Json
    foreach($p in $seal.files.PSObject.Properties){
        if($p.Name -match '^ingestion/forensic_records/([^/]+\.py)$' -and $imageHashes.PSObject.Properties[$Matches[1]]) {
            if($imageHashes.($Matches[1]) -cne $p.Value){throw "Built image source mismatch: $($p.Name)"}
        }
    }
    foreach($required in @('worker.py','media_pipeline.py','video_anpr_onnx_v3.py','multilingual_ocr.py','processor_readiness.py')){if(-not $imageHashes.PSObject.Properties[$required]){throw "Built image source missing: $required"}}
    Write-NxJson (Join-Path $run 'built-source-hashes.json') $imageHashes
    $now=Get-NxContainers; Assert-NxHealth $now
    Assert-NxProtected ($preflight.containers | ConvertTo-Json -Depth 20 | ConvertFrom-Json) $now
    if($now['forensic-records-worker'].Id -ne $worker.Id -or (Get-NxJobs $now) -ne 0){throw 'Worker identity/jobs changed during build.'}
    $null=Get-NxManifest; $null=Get-NxAssets $manifest -DestinationRequired
    Assert-NxRam
    Write-NxJson (Join-Path $run 'state.json') @{state='RECREATION_STARTED';recreation_started=$true;expected_image=$image;manifest_sha256=(Get-NxHash $script:NxManifestPath)}
    $recreation=$true
    $null=Invoke-NxReceiptCompose $activationFile @('up','-d','--no-deps','--no-build','--pull','never','--force-recreate','forensic-records-worker') 180 (Join-Path $run 'recreate.log')
    & (Join-Path $PSScriptRoot 'verify_nxmmr_demo_activation.ps1') -RunDirectory $run -Internal
    Write-Host 'ACTIVATION=PASS. Reopen Codex/Chrome only after ACTIVATION_VERIFICATION=PASS.'
    Write-Host "Receipt: $run"
    Write-Host 'Next: .\scripts\prepare_nxmmr_demo_acceptance.ps1'
} catch {
    Write-Host "ACTIVATION=FAILED reason=$($_.Exception.Message)"
    if($recreation){
        try {& (Join-Path $PSScriptRoot 'rollback_nxmmr_demo_activation.ps1') -RunDirectory $run -Internal}
        catch {Write-Host "ROLLBACK=FAILED reason=$($_.Exception.Message)"; Write-Host ".\scripts\rollback_nxmmr_demo_activation.ps1 -RunDirectory '$run'"}
    } else {Write-Host 'Original live worker has not been recreated. Do not guess; preserve logs.'}
    Write-Host "Receipt: $run"
    exit 3
} finally {if($transcript){Stop-Transcript | Out-Null};if($mutex){$mutex.ReleaseMutex();$mutex.Dispose()}}
