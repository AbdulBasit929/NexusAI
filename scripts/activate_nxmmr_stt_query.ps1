# SPDX-License-Identifier: MIT
[CmdletBinding()]
param(
    [ValidateRange(1,120)][int]$WaitForRamMinutes=30,
    [bool]$RecoverBuildCache=$true
)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$run='';$recreation=$false;$transcript=$false;$mutex=$null
function Find-NxSttResumableBuildSet($Manifest, $Before, [string]$CurrentRun) {
    $manifestHash=Get-NxHash $script:NxSttManifestPath
    $sealHash=Get-NxHash $script:NxSttIntegrityPath
    foreach($directory in @(Get-ChildItem -LiteralPath $script:NxSttRunRoot -Directory | Sort-Object Name -Descending)){
        if($directory.FullName -eq $CurrentRun){continue}
        $path=Join-Path $directory.FullName 'build-set.json'
        if(-not (Test-Path -LiteralPath $path -PathType Leaf)){continue}
        try {
            $candidate=Get-Content -LiteralPath $path -Raw|ConvertFrom-Json
            if($candidate.contract_version -ne 'nexusai.nxmmr.stt-query-build-set/v1' -or $candidate.manifest_sha256 -cne $manifestHash -or $candidate.source_seal_sha256 -cne $sealHash -or $candidate.recreation_started){continue}
            $original=ConvertTo-NxMap $candidate.original_containers
            $same=$true
            foreach($name in $script:NxSttChanged){if($original[$name] -cne $Before[$name].Id){$same=$false;break}}
            if(-not $same){continue}
            $valid=[ordered]@{}
            foreach($property in $candidate.images.PSObject.Properties){
                $entry=$property.Value
                if($property.Name -notin $script:NxSttChanged){continue}
                $actual=Invoke-NxNative docker @('image','inspect',[string]$entry.tag,'--format','{{.Id}}')
                if($actual -cne [string]$entry.id){throw "candidate image tag drifted: $($property.Name)"}
                $valid[$property.Name]=[ordered]@{tag=[string]$entry.tag;id=[string]$entry.id}
            }
            if($valid.Count){Write-Host "RESUMABLE_BUILD_SET=PASS receipt=$path images=$($valid.Keys -join ',')";return $valid}
        } catch {Write-Host "RESUMABLE_BUILD_SET=REJECTED receipt=$path reason=$($_.Exception.Message)"}
    }
    return [ordered]@{}
}
try {
    . (Join-Path $PSScriptRoot 'nxmmr_stt_query_operator_common.ps1')
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
    Write-NxJson (Join-Path $run 'rollback.json') @{state='PREPARED';scope=$script:NxSttChanged;original_images=$originalImages;original_containers=@{api=$before.api.Id;'forensic-records-api'=$before['forensic-records-api'].Id;'forensic-records-worker'=$before['forensic-records-worker'].Id};compose_sha256=(Get-NxHash $rollbackFile);exact_runtime_snapshot=$true}
    Write-NxJson (Join-Path $script:NxSttRunRoot 'latest.json') @{run_directory=$run}
    Assert-NxSttRam $manifest
    $assets=Copy-NxSttAssets $manifest;Write-NxJson (Join-Path $run 'asset-hashes.json') $assets
    Write-NxJson (Join-Path $run 'state.json') @{state='BUILD_STARTED';recreation_started=$false;manifest_sha256=(Get-NxHash $script:NxSttManifestPath)}
    $built=Find-NxSttResumableBuildSet $manifest $before $run
    $buildSet=[ordered]@{contract_version='nexusai.nxmmr.stt-query-build-set/v1';manifest_sha256=(Get-NxHash $script:NxSttManifestPath);source_seal_sha256=(Get-NxHash $script:NxSttIntegrityPath);original_containers=[ordered]@{api=$before.api.Id;'forensic-records-api'=$before['forensic-records-api'].Id;'forensic-records-worker'=$before['forensic-records-worker'].Id};images=$built;recreation_started=$false}
    $buildSetFile=Join-Path $run 'build-set.json';Write-NxJson $buildSetFile $buildSet
    foreach($name in $script:NxSttChanged){
        if($built.Contains($name)){Write-Host "Reusing sealed candidate image for ${name}: $($built[$name].id)";continue}
        $null=Wait-NxSttRam $manifest $WaitForRamMinutes -RecoverBuildCache:$RecoverBuildCache
        Write-Host "Building $name only; no model acquisition is allowed."
        $null=Invoke-NxReceiptCompose $activationFile @('--progress','plain','build','--pull=false',$name) $manifest.resource_gates.build_timeout_seconds (Join-Path $run "build-$name.log") -StreamOutput
        $imageID=Invoke-NxNative docker @('image','inspect',$candidateImages[$name],'--format','{{.Id}}')
        $built[$name]=[ordered]@{tag=$candidateImages[$name];id=$imageID}
        $buildSet.images=$built;Write-NxJson $buildSetFile $buildSet
        if($name -ne $script:NxSttChanged[-1]){$null=Wait-NxSttRam $manifest $WaitForRamMinutes -RecoverBuildCache:$RecoverBuildCache}
    }
    $immutable=[ordered]@{}
    foreach($name in $script:NxSttChanged){
        if(-not $built.Contains($name)){throw "Candidate image is missing from the sealed build set: $name"}
        $immutable[$name]=[string]$built[$name].id;$activation.services[$name].image=$immutable[$name];$activation.services[$name].Remove('build')
    }
    Write-NxJson $activationFile $activation
    $null=Invoke-NxReceiptCompose $activationFile @('config','--quiet')
    $workerImage=$immutable['forensic-records-worker']
    $probe=[Convert]::ToBase64String([IO.File]::ReadAllBytes((Join-Path $PSScriptRoot 'nxmmr_stt_query_readiness_probe.py')))
    $probeCode="import base64;exec(base64.b64decode('$probe'))"
    $probeCompose=Get-Content -LiteralPath $activationFile -Raw|ConvertFrom-Json
    $probeWorker=$probeCompose.services.'forensic-records-worker'
    if($probeWorker.PSObject.Properties['entrypoint']){$probeWorker.entrypoint=@('python')}else{$probeWorker|Add-Member -NotePropertyName entrypoint -NotePropertyValue @('python')}
    if($probeWorker.PSObject.Properties['command']){$probeWorker.command=@('-c',$probeCode)}else{$probeWorker|Add-Member -NotePropertyName command -NotePropertyValue @('-c',$probeCode)}
    $probeWorker.ports=@();$probeWorker.PSObject.Properties.Remove('healthcheck')
    $probeFile=Join-Path $run 'worker-probe-compose.json';Write-NxJson $probeFile @{services=@{'forensic-records-worker'=$probeCompose.services.'forensic-records-worker'};networks=$probeCompose.networks;volumes=$probeCompose.volumes}
    $probeOutput=Invoke-NxReceiptCompose $probeFile @('run','--rm','--no-deps','--pull','never','forensic-records-worker') 300 (Join-Path $run 'worker-readiness.log')
    if($probeOutput -notmatch 'NXMMR_STT_QUERY_READINESS='){throw 'Candidate worker readiness probe failed.'}
    $now=Get-NxSttContainers;Assert-NxHealth $now;Assert-NxSttProtected ($preflight.containers|ConvertTo-Json -Depth 20|ConvertFrom-Json) $now
    foreach($name in $script:NxSttChanged){if($now[$name].Id -cne $before[$name].Id){throw "Service changed during build: $name"}}
    if((Get-NxJobs $now) -ne 0){throw 'Active jobs appeared during build.'}
    $null=Get-NxSttManifest;$null=Get-NxSttAssets $manifest -DestinationRequired
    $null=Wait-NxSttRam $manifest $WaitForRamMinutes -RecoverBuildCache:$RecoverBuildCache
    Write-NxJson (Join-Path $run 'state.json') @{state='RECREATION_STARTED';recreation_started=$true;expected_images=$immutable;manifest_sha256=(Get-NxHash $script:NxSttManifestPath)}
    $buildSet.recreation_started=$true;Write-NxJson $buildSetFile $buildSet
    $recreation=$true
    $null=Invoke-NxReceiptCompose $activationFile @('up','-d','--no-deps','--no-build','--pull','never','--force-recreate') 300 (Join-Path $run 'recreate.log')
    & (Join-Path $PSScriptRoot 'verify_nxmmr_stt_query_activation.ps1') -RunDirectory $run -Internal
    Write-Host "STT_QUERY_ACTIVATION=PASS receipt=$run"
} catch {
    Write-Host "STT_QUERY_ACTIVATION=FAILED reason=$($_.Exception.Message)"
    if($recreation){try{& (Join-Path $PSScriptRoot 'rollback_nxmmr_stt_query_activation.ps1') -RunDirectory $run -Internal}catch{Write-Host "ROLLBACK=FAILED reason=$($_.Exception.Message)"}}
    else{Write-Host 'No live service was recreated; copied SigLIP files, if any, remain inactive and immutable.'}
    exit 3
} finally {if($transcript){Stop-Transcript|Out-Null};if($mutex){$mutex.ReleaseMutex();$mutex.Dispose()}}
