# SPDX-License-Identifier: MIT
[CmdletBinding()]
param(
    [ValidateRange(1,120)][int]$WaitForRamMinutes=120,
    [switch]$AllowGuardedDrain,
    [bool]$RecoverBuildCache=$true
)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$run='';$mutationStarted=$false;$transcript=$false;$mutex=$null;$state=$null
try {
    . (Join-Path $PSScriptRoot 'nxmmr_four_p1_operator_common.ps1')
    $mutex=Enter-NxP1OperatorLock;$run=New-NxP1RunDirectory
    Start-Transcript -LiteralPath (Join-Path $run 'activation.log')|Out-Null;$transcript=$true
    $preflight=Invoke-NxP1Preflight;Write-NxJson (Join-Path $run 'preflight.json') $preflight;Show-NxP1Preflight $preflight
    if($preflight.status -ne 'PASS'){throw 'Preflight blocked before every live mutation.'}
    $manifest=Get-NxP1Manifest;$before=Get-NxSttContainers;Assert-NxHealth $before;Assert-NxP1StartingRuntime $before $manifest;Assert-NxP1WorkerRoles $before['forensic-records-worker'] $manifest
    $beforeSummary=Get-NxSummary $before;$counts=Get-NxP1RetainedCounts $before;Assert-NxP1RetainedState $before $counts
    $originalImages=[ordered]@{};$candidateTags=[ordered]@{};$originalContainers=[ordered]@{}
    foreach($name in $script:NxSttChanged){$originalImages[$name]=$before[$name].Image;$candidateTags[$name]=[string]$manifest.services.candidate_images.$name;$originalContainers[$name]=$before[$name].Id}
    $rollback=New-NxSttCompose $before $originalImages ([ordered]@{}) $manifest
    $activation=New-NxSttCompose $before $candidateTags ([ordered]@{}) $manifest -Build
    Set-NxP1BuildPolicy $activation $manifest
    $rollbackFile=Join-Path $run 'rollback-compose.json';$activationFile=Join-Path $run 'activation-compose.json'
    Write-NxJson $rollbackFile $rollback;Write-NxJson $activationFile $activation
    Write-NxJson (Join-Path $run 'container-before.json') $beforeSummary
    Write-NxJson (Join-Path $run 'private-container-snapshot.json') $before
    $null=Invoke-NxReceiptCompose $rollbackFile @('config','--quiet');$null=Invoke-NxReceiptCompose $activationFile @('config','--quiet')
    $state=[ordered]@{contract_version='nexusai.nxmmr.four-p1-state/v1';state='PREPARED';mutation_started=$false;drained=@();touched=@();expected_images=[ordered]@{};manifest_sha256=(Get-NxHash $script:NxSttManifestPath);source_seal_sha256=(Get-NxHash $script:NxSttIntegrityPath);rollback_compose_sha256=(Get-NxHash $rollbackFile);retained_counts_before=$counts}
    Write-NxJson (Join-Path $run 'state.json') $state;Write-NxJson (Join-Path $script:NxSttRunRoot 'latest.json') @{run_directory=$run}
    $null=Wait-NxSttRam $manifest $WaitForRamMinutes -RecoverBuildCache:$RecoverBuildCache
    $built=Find-NxP1ResumableBuildSet $before $run
    $buildSet=[ordered]@{contract_version='nexusai.nxmmr.four-p1-build-set/v1';manifest_sha256=(Get-NxHash $script:NxSttManifestPath);source_seal_sha256=(Get-NxHash $script:NxSttIntegrityPath);original_images=$originalImages;images=$built;recreation_started=$false}
    $buildSetFile=Join-Path $run 'build-set.json';Write-NxJson $buildSetFile $buildSet
    foreach($name in $script:NxSttChanged){
        if($built.Contains($name)){Write-Host "Reusing source-bound candidate image for ${name}: $($built[$name].id)";continue}
        $null=Wait-NxSttRam $manifest $WaitForRamMinutes -RecoverBuildCache:$RecoverBuildCache
        Write-Host "Building $name only with network=default and pull=false. Approved Dockerfile build dependencies may download; model downloads and runtime package installation remain prohibited."
        $null=Invoke-NxReceiptCompose $activationFile @('--progress','plain','build','--pull=false',$name) $manifest.resource_gates.build_timeout_seconds (Join-Path $run "build-$name.log") -StreamOutput
        $imageID=Invoke-NxNative docker @('image','inspect',$candidateTags[$name],'--format','{{.Id}}')
        $built[$name]=[ordered]@{tag=$candidateTags[$name];id=$imageID};$buildSet.images=$built;Write-NxJson $buildSetFile $buildSet
    }
    $immutable=[ordered]@{}
    foreach($name in $script:NxSttChanged){
        if(-not $built.Contains($name)){throw "Candidate image missing from sealed build set: $name"}
        $immutable[$name]=[string]$built[$name].id;$activation.services[$name].image=$immutable[$name];$activation.services[$name].Remove('build')
    }
    Write-NxJson $activationFile $activation;$null=Invoke-NxReceiptCompose $activationFile @('config','--quiet')
    $state.expected_images=$immutable;$state.state='BUILT_AND_SEALED';Write-NxJson (Join-Path $run 'state.json') $state
    $null=Invoke-NxNative docker @('run','--rm','--pull','never','--network','none','--read-only','--entrypoint','/local-ai',$immutable.api,'--version') 180 (Join-Path $run 'api-version-smoke.log')
    $null=Invoke-NxNative docker @('image','inspect',$immutable['forensic-records-api'],'--format','{{.Id}}')
    $null=Get-NxP1Manifest
    $now=Get-NxSttContainers;Assert-NxHealth $now;Assert-NxP1StartingRuntime $now $manifest;Assert-NxP1RetainedState $now $counts
    $ready=$false
    try {$null=Wait-NxSttRam $manifest ([int]$manifest.resource_gates.post_build_ram_wait_minutes) -RecoverBuildCache:$RecoverBuildCache;$ready=$true} catch {Write-Host "POST_BUILD_RAM_GATE=PENDING reason=$($_.Exception.Message)"}
    if(-not $ready){
        if(-not $AllowGuardedDrain){throw 'Final RAM gate requires the admitted guarded candidate-service drain. Rerun through the single operator command.'}
        $now=Get-NxSttContainers;Assert-NxHealth $now;Assert-NxP1StartingRuntime $now $manifest;Assert-NxP1RetainedState $now $counts
        $state.mutation_started=$true;$state.drained=@($script:NxSttChanged);$state.touched=@($script:NxSttChanged);$state.state='DRAINING_CANDIDATE_SERVICES';Write-NxJson (Join-Path $run 'state.json') $state
        $buildSet.recreation_started=$true;Write-NxJson $buildSetFile $buildSet;$mutationStarted=$true
        Write-Host 'GUARDED_DRAIN=START services=api,forensic-records-api protected=forensic-records-worker,forensic-postgres,forensic-nats'
        $null=Invoke-NxNative docker @('stop','--timeout','30',$before.api.Id,$before['forensic-records-api'].Id) 90 (Join-Path $run 'drain.log') -StreamOutput
        $drained=Get-NxSttContainers;Assert-NxSttProtected ($beforeSummary|ConvertTo-Json -Depth 30|ConvertFrom-Json) $drained;Assert-NxP1RetainedState $drained $counts
        if($drained.api.State.Running -or $drained['forensic-records-api'].State.Running){throw 'Candidate services did not stop cleanly.'}
        $null=Wait-NxSttRam $manifest $WaitForRamMinutes -RecoverBuildCache:$RecoverBuildCache
    } else {
        $state.mutation_started=$true;$state.touched=@($script:NxSttChanged);$state.state='RECREATION_STARTED';Write-NxJson (Join-Path $run 'state.json') $state
        $buildSet.recreation_started=$true;Write-NxJson $buildSetFile $buildSet;$mutationStarted=$true
    }
    $null=Get-NxP1Manifest
    $current=Get-NxSttContainers;Assert-NxSttProtected ($beforeSummary|ConvertTo-Json -Depth 30|ConvertFrom-Json) $current;Assert-NxP1RetainedState $current $counts
    $state.state='STARTING_CANDIDATE_SERVICES';Write-NxJson (Join-Path $run 'state.json') $state
    $null=Invoke-NxReceiptCompose $activationFile @('up','-d','--no-deps','--no-build','--pull','never','--force-recreate') 300 (Join-Path $run 'recreate.log') -StreamOutput
    & (Join-Path $PSScriptRoot 'verify_nxmmr_four_p1_activation.ps1') -RunDirectory $run -Internal
    Write-Host "P1_CORRECTION_ACTIVATION=PASS receipt=$run"
} catch {
    Write-Host "P1_CORRECTION_ACTIVATION=FAILED reason=$($_.Exception.Message)"
    if($mutationStarted){try{& (Join-Path $PSScriptRoot 'rollback_nxmmr_four_p1_activation.ps1') -RunDirectory $run -Internal}catch{Write-Host "P1_CORRECTION_ROLLBACK=FAILED reason=$($_.Exception.Message)"}}
    else{Write-Host 'No live service was stopped or recreated. Completed source-bound images, if any, remain resumable.'}
    exit 3
} finally {if($transcript){Stop-Transcript|Out-Null};if($mutex){$mutex.ReleaseMutex();$mutex.Dispose()}}
