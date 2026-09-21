# SPDX-License-Identifier: MIT
[CmdletBinding()]
param(
    [ValidateRange(1,120)][int]$WaitForRamMinutes=30,
    [bool]$RecoverBuildCache=$true
)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$run='';$recreation=$false;$transcript=$false;$mutex=$null
try {
    . (Join-Path $PSScriptRoot 'nxmmr_shared_query_operator_common.ps1')
    $mutex=Enter-NxSttOperatorLock;$run=New-NxSttRunDirectory
    Start-Transcript -LiteralPath (Join-Path $run 'activation.log')|Out-Null;$transcript=$true
    $preflight=Invoke-NxSttPreflight;Write-NxJson (Join-Path $run 'preflight.json') $preflight;Show-NxSttPreflight $preflight
    if($preflight.status -ne 'PASS'){throw 'Preflight blocked before all mutation.'}
    $manifest=Get-NxSttManifest;$before=Get-NxSttContainers;Assert-NxHealth $before
    $originalImages=[ordered]@{};$candidateImages=[ordered]@{}
    foreach($name in $script:NxSttChanged){$originalImages[$name]=$before[$name].Image;$candidateImages[$name]=[string]$manifest.services.candidate_images.$name}
    $rollback=New-NxSttCompose $before $originalImages ([ordered]@{}) $manifest
    $activation=New-NxSttCompose $before $candidateImages (ConvertTo-NxMap $manifest.worker_environment) $manifest -Build
    $rollbackFile=Join-Path $run 'rollback-compose.json';$activationFile=Join-Path $run 'activation-compose.json'
    Write-NxJson $rollbackFile $rollback;Write-NxJson $activationFile $activation
    Write-NxJson (Join-Path $run 'container-before.json') (Get-NxSummary $before)
    $null=Invoke-NxReceiptCompose $rollbackFile @('config','--quiet');$null=Invoke-NxReceiptCompose $activationFile @('config','--quiet')
    $originalContainers=[ordered]@{};foreach($name in $script:NxSttChanged){$originalContainers[$name]=$before[$name].Id}
    Write-NxJson (Join-Path $run 'rollback.json') @{state='PREPARED';scope=$script:NxSttChanged;original_images=$originalImages;original_containers=$originalContainers;compose_sha256=(Get-NxHash $rollbackFile);exact_runtime_snapshot=$true}
    Write-NxJson (Join-Path $script:NxSttRunRoot 'latest.json') @{run_directory=$run}
    Assert-NxSttRam $manifest
    $assets=Get-NxSttAssets $manifest -DestinationRequired;Write-NxJson (Join-Path $run 'asset-hashes.json') $assets
    Write-NxJson (Join-Path $run 'state.json') @{state='BUILD_STARTED';recreation_started=$false;manifest_sha256=(Get-NxHash $script:NxSttManifestPath)}
    $built=Find-NxSharedQueryResumableBuildSet $before $run
    $buildSet=[ordered]@{contract_version='nexusai.nxmmr.shared-query-build-set/v1';manifest_sha256=(Get-NxHash $script:NxSttManifestPath);source_seal_sha256=(Get-NxHash $script:NxSttIntegrityPath);original_containers=$originalContainers;images=$built;recreation_started=$false}
    $buildSetFile=Join-Path $run 'build-set.json';Write-NxJson $buildSetFile $buildSet
    foreach($name in $script:NxSttChanged){
        if($built.Contains($name)){Write-Host "Reusing sealed candidate image for ${name}: $($built[$name].id)";continue}
        $null=Wait-NxSttRam $manifest $WaitForRamMinutes -RecoverBuildCache:$RecoverBuildCache
        Write-Host "Building $name only; no model acquisition is allowed."
        $null=Invoke-NxReceiptCompose $activationFile @('--progress','plain','build','--pull=false',$name) $manifest.resource_gates.build_timeout_seconds (Join-Path $run "build-$name.log") -StreamOutput
        $imageID=Invoke-NxNative docker @('image','inspect',$candidateImages[$name],'--format','{{.Id}}')
        $built[$name]=[ordered]@{tag=$candidateImages[$name];id=$imageID};$buildSet.images=$built;Write-NxJson $buildSetFile $buildSet
        if($name -ne $script:NxSttChanged[-1]){$null=Wait-NxSttRam $manifest $WaitForRamMinutes -RecoverBuildCache:$RecoverBuildCache}
    }
    $immutable=[ordered]@{}
    foreach($name in $script:NxSttChanged){
        if(-not $built.Contains($name)){throw "Candidate image is missing from the sealed build set: $name"}
        $immutable[$name]=[string]$built[$name].id;$activation.services[$name].image=$immutable[$name];$activation.services[$name].Remove('build')
    }
    Write-NxJson $activationFile $activation;$null=Invoke-NxReceiptCompose $activationFile @('config','--quiet')
    $now=Get-NxSttContainers;Assert-NxHealth $now;Assert-NxSttProtected ($preflight.containers|ConvertTo-Json -Depth 20|ConvertFrom-Json) $now
    foreach($name in $script:NxSttChanged){if($now[$name].Id -cne $before[$name].Id){throw "Service changed during build: $name"}}
    if((Get-NxJobs $now) -ne 0){throw 'Active jobs appeared during build.'}
    $null=Get-NxSttManifest;$null=Get-NxSttAssets $manifest -DestinationRequired
    $null=Wait-NxSttRam $manifest $WaitForRamMinutes -RecoverBuildCache:$RecoverBuildCache
    Write-NxJson (Join-Path $run 'state.json') @{state='RECREATION_STARTED';recreation_started=$true;expected_images=$immutable;manifest_sha256=(Get-NxHash $script:NxSttManifestPath)}
    $buildSet.recreation_started=$true;Write-NxJson $buildSetFile $buildSet;$recreation=$true
    $null=Invoke-NxReceiptCompose $activationFile @('up','-d','--no-deps','--no-build','--pull','never','--force-recreate') 300 (Join-Path $run 'recreate.log')
    & (Join-Path $PSScriptRoot 'verify_nxmmr_shared_query_activation.ps1') -RunDirectory $run -Internal
    Write-Host "SHARED_QUERY_ACTIVATION=PASS receipt=$run"
} catch {
    Write-Host "SHARED_QUERY_ACTIVATION=FAILED reason=$($_.Exception.Message)"
    if($recreation){try{& (Join-Path $PSScriptRoot 'rollback_nxmmr_shared_query_activation.ps1') -RunDirectory $run -Internal}catch{Write-Host "ROLLBACK=FAILED reason=$($_.Exception.Message)"}}
    else{Write-Host 'No live service was recreated and no model asset was copied.'}
    exit 3
} finally {if($transcript){Stop-Transcript|Out-Null};if($mutex){$mutex.ReleaseMutex();$mutex.Dispose()}}
