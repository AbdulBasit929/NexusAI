# SPDX-License-Identifier: MIT
[CmdletBinding()]
param(
    [switch]$PreflightOnly,
    [switch]$RecoverRam,
    [ValidateRange(1,120)][int]$WaitForRamMinutes=30,
    [string]$RollbackDirectory=''
)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'nxmmr_correction_common.ps1')

$run='';$mutex=$null;$transcript=$false;$state=$null;$apiDrained=$false
$expected=[ordered]@{
    'forensic-records-worker'=@('8ba318c294fd77b2861ce8518f164b7003b2a311edf75e5c68a7910bb6272374','sha256:2755e0ffbf3774d722b12cc9a5fbfc2241aad6e876e9891ca8b1c13d24afd146')
    api=@('ede93502d54280527381233e808b189278e6b723fa21cbcb9a5e540885f0bf89','sha256:54683ddd2c2302632b25c2619d13254f7ef685d46f556ea3854fdfcd5463f8c1')
    'forensic-records-api'=@('b5c13f96e67f21f129f477c1b0d1a921a7d761ca528757240ca2ee90a85157d8','sha256:d2992062b8f55334d602530eb96cec8289dd714f0ae9523d3b27edebb724b5c5')
    'forensic-postgres'=@('f66e05a3b17978c9912aa911177f22e57d69c17de87881a33f23bcba228ad361','sha256:61f891691050da6032023c01ea885730eeeba06b7c17b403e7d0b9c49c37dfe9')
    'forensic-nats'=@('5c49e70d132ac01b4adf86f27b1735d4d5cf6cc205d848c0881164f1f200eb04','sha256:e4bf19f15fd3218814a4e3c9e0064e1334bd8aa20d5984b9f1a0afd084f8cc00')
}
function Assert-NxmCurrent($Containers) {
    foreach($name in $expected.Keys){
        if($Containers[$name].Id -cne $expected[$name][0] -or $Containers[$name].Image -cne $expected[$name][1] -or $Containers[$name].RestartCount -ne 0){
            throw "Current runtime identity changed: $name. Stop and obtain a renewed reviewed snapshot."
        }
    }
}
function Invoke-NxmOptionalWindowsRamRecovery {
    $targets=@(Get-Process -Name PhoneExperienceHost,LockApp -ErrorAction SilentlyContinue)
    if(-not $targets.Count){Write-Host 'OPTIONAL_WINDOWS_RAM_RECOVERY=NO_TARGETS';return}
    $mib=[math]::Round((($targets | Measure-Object WorkingSet64 -Sum).Sum/1MB),0)
    $targets | Stop-Process -Force
    Write-Host "OPTIONAL_WINDOWS_RAM_RECOVERY=PASS Processes=$($targets.Count) PriorWorkingSetMiB=$mib Targets=PhoneExperienceHost,LockApp"
}
function Invoke-NxmCleanLinuxPageCache {
    for($attempt=1;$attempt -le 6;$attempt++){
        $lines=Invoke-NxNative wsl.exe @('-d','docker-desktop','-u','root','sh','-lc','grep -E "^(Dirty|Writeback):" /proc/meminfo') 30
        $values=@{}
        foreach($line in ($lines -split '\r?\n')){if($line -match '^(Dirty|Writeback):\s+([0-9]+)\s+kB$'){$values[$Matches[1]]=[int64]$Matches[2]}}
        if($values.Count -eq 2 -and $values.Dirty -eq 0 -and $values.Writeback -eq 0){
            $null=Invoke-NxNative wsl.exe @('-d','docker-desktop','-u','root','sh','-lc','echo 1 > /proc/sys/vm/drop_caches') 30
            Write-Host 'CLEAN_LINUX_PAGE_CACHE=PASS Mode=1 DirtyKiB=0 WritebackKiB=0 Sync=false'
            return
        }
        Write-Host "CLEAN_LINUX_PAGE_CACHE=WAITING DirtyKiB=$($values.Dirty) WritebackKiB=$($values.Writeback)"
        Start-Sleep -Seconds 5
    }
    Write-Host 'CLEAN_LINUX_PAGE_CACHE=SKIPPED reason=dirty_or_writeback_nonzero'
}
try {
    $mutex=Enter-NxOperatorLock
    Assert-NxcSources
    if($RollbackDirectory){Invoke-NxcRollback (Get-NxRunDirectory $RollbackDirectory);exit 0}
    $run=New-NxRunDirectory
    Start-Transcript -LiteralPath (Join-Path $run 'mixed-ocr-worker.log') | Out-Null;$transcript=$true
    Write-Host "MixedOCRReceiptDirectory=$run"
    Write-Host 'Scope=replace forensic-records-worker only. LocalAI is temporarily drained and restarted unchanged; all other services are protected.'
    $before=Get-NxContainers;Assert-NxHealth $before;Assert-NxmCurrent $before
    if((Get-NxJobs $before) -ne 0){throw 'Active processing jobs: no build or replacement allowed.'}
    $disk=[double](Get-PSDrive -Name ([IO.Path]::GetPathRoot($script:NxRoot).Substring(0,1))).Free/1GB
    if($disk -lt 12){throw 'Free disk below 12 GiB.'}
    $summary=Get-NxSummary $before
    Write-NxJson (Join-Path $run 'before.json') $summary
    Write-NxJson (Join-Path $run 'private-container-snapshot.json') $before
    $counts=Get-NxcCounts $before
    $state=[ordered]@{contract='nxmmr.correction/v1';state='PREPARED';touched=@();images=[ordered]@{};rollback_hashes=[ordered]@{};counts_before=$counts;before_hash=(Get-NxHash (Join-Path $run 'before.json'))}
    $stamp=Split-Path $run -Leaf
    $worker=$before['forensic-records-worker']
    $tag="nexusai/forensic-records-worker:nxmmr-mixed-ocr-$stamp"
    $rollback=New-NxWorkerCompose $worker $worker.Image ([ordered]@{})
    $candidate=New-NxWorkerCompose $worker $tag ([ordered]@{})
    $rollbackFile=Join-Path $run 'rollback-forensic-records-worker.json'
    $candidateFile=Join-Path $run 'candidate-forensic-records-worker.json'
    Write-NxJson $rollbackFile $rollback
    Write-NxJson $candidateFile $candidate
    $state.rollback_hashes['forensic-records-worker']=Get-NxHash $rollbackFile
    Write-NxJson (Join-Path $run 'correction-state.json') $state
    $null=Invoke-NxReceiptCompose $rollbackFile @('config','--quiet')
    $null=Invoke-NxReceiptCompose $candidateFile @('config','--quiet')
    $null=Invoke-NxNative python @('-B',(Resolve-NxPath 'scripts/run_nxmmr_anpr_ocr_vertical_selftests.py')) 120 (Join-Path $run 'ocr-selftests.log') -StreamOutput
    $oldRevision='import multilingual_ocr; assert multilingual_ocr.PROCESSOR_REVISION == "nxmmr-anpr-ocr-vertical-v1-inputfix1"'
    $null=Invoke-NxNative docker @('exec',$worker.Id,'python','-c',$oldRevision) 30 (Join-Path $run 'old-revision.log')
    $ram=Get-NxRam
    Write-Host ('AvailableRAMGiB={0:N3} RequiredRAMGiB=6' -f $ram)
    if($PreflightOnly){
        if($ram -lt 6){throw 'RAM gate not met. Close Codex/ChatGPT and run the full command from standalone PowerShell.'}
        Write-Host 'MIXED_OCR_WORKER_PREFLIGHT=PASS RuntimeMutated=false'
        exit 0
    }
    Write-Host 'OPERATOR_ACTION=Close Codex, ChatGPT and browsers now. Keep Docker Desktop and this PowerShell window open.'
    Wait-NxcOperatorAppsClosed $WaitForRamMinutes
    if($RecoverRam){Invoke-NxmOptionalWindowsRamRecovery}
    $null=Invoke-NxNative docker @('image','tag',$worker.Image,"nexusai/correction-rollback:forensic-records-worker-$stamp")
    $now=Get-NxContainers;Assert-NxHealth $now;Assert-NxmCurrent $now;Assert-NxProtected ($summary | ConvertTo-Json -Depth 20 | ConvertFrom-Json) $now
    if((Get-NxJobs $now) -ne 0 -or (Get-NxcCounts $now) -cne $counts){throw 'Retained state or jobs changed before the RAM drain.'}
    $state.touched=@('forensic-records-worker');$state.drained_services=@('api');$state.state='DRAINING_FOR_BUILD'
    Write-NxJson (Join-Path $run 'correction-state.json') $state
    Write-Host 'Temporarily stopping the old worker and LocalAI to reach 6 GiB. LocalAI will restart as the exact same container/image.'
    $null=Invoke-NxNative docker @('stop','--timeout','30',$worker.Id,$before.api.Id) 90 (Join-Path $run 'drain-for-build.log') -StreamOutput
    $apiDrained=$true
    $now=Get-NxContainers -AllowMissingWorker
    Assert-NxProtected ($summary | ConvertTo-Json -Depth 20 | ConvertFrom-Json) $now
    if((Get-NxJobs $now) -ne 0 -or (Get-NxcCounts $now) -cne $counts){throw 'Retained guard changed during the RAM drain.'}
    if($RecoverRam -and (Get-NxRam) -lt 6){Invoke-NxmCleanLinuxPageCache}
    Wait-NxcRam $WaitForRamMinutes
    $state.state='BUILDING';Write-NxJson (Join-Path $run 'correction-state.json') $state
    Write-Host 'Building worker-only mixed OCR layer. Network=none; no package or model downloads.'
    $null=Invoke-NxNative docker @('build','--pull=false','--network=none','--progress=plain','--build-arg',"BASE_IMAGE=nexusai/correction-rollback:forensic-records-worker-$stamp",'-f',(Resolve-NxPath 'configuration/nxmmr_correction_worker.Dockerfile'),'-t',$tag,$script:NxRoot) 600 (Join-Path $run 'build-worker.log') -StreamOutput
    $image=Invoke-NxNative docker @('image','inspect',$tag,'--format','{{.Id}}')
    $state.images['forensic-records-worker']=$image
    $candidate=Get-Content $candidateFile -Raw | ConvertFrom-Json
    $candidate.services.'forensic-records-worker'.image=$image
    Write-NxJson $candidateFile $candidate
    $sourceSmoke='import hashlib,pathlib,multilingual_ocr; assert multilingual_ocr.PROCESSOR_REVISION == "nxmmr-anpr-ocr-vertical-v1-inputfix1-mixedfix1"; print(hashlib.sha256(pathlib.Path("/app/multilingual_ocr.py").read_bytes()).hexdigest())'
    $hash=Invoke-NxNative docker @('run','--rm','--pull','never','--network','none','--read-only','--entrypoint','python',$image,'-c',$sourceSmoke) 120 (Join-Path $run 'worker-source-smoke.log')
    if($hash -cne (Get-NxHash (Resolve-NxPath 'ingestion/forensic_records/multilingual_ocr.py'))){throw 'Worker image source mismatch.'}
    $manifest=[Convert]::ToBase64String([IO.File]::ReadAllBytes((Resolve-NxPath 'configuration/nxmmr_demo_activation_v1.json')))
    $smoke=[Convert]::ToBase64String([IO.File]::ReadAllBytes((Resolve-NxPath 'scripts/smoke_nxmmr_ocr_extensionless.py')))
    $smokeCode="import base64,pathlib,sys; pathlib.Path('/tmp/manifest.json').write_bytes(base64.b64decode('$manifest')); code=base64.b64decode('$smoke'); sys.argv=['smoke','--manifest','/tmp/manifest.json','--receipt','/receipt/model-smoke.json']; exec(compile(code,'smoke_nxmmr_ocr_extensionless.py','exec'))"
    $modelRoot=(Resolve-NxPath 'models/media')
    $null=Invoke-NxNative docker @('run','--rm','--pull','never','--network','none','--read-only','--tmpfs','/tmp:rw,noexec,nosuid,size=64m','--env','HOME=/tmp','--env','PADDLE_PDX_CACHE_HOME=/tmp/.paddlex','--mount',"type=bind,source=$modelRoot,target=/models/media,readonly",'--mount',"type=bind,source=$run,target=/receipt",'--entrypoint','python',$image,'-c',$smokeCode) 360 (Join-Path $run 'model-smoke.log') -StreamOutput
    $modelSmoke=Get-Content (Join-Path $run 'model-smoke.json') -Raw | ConvertFrom-Json
    if($modelSmoke.state -ne 'PASS' -or $modelSmoke.retained_evidence_accessed -or $modelSmoke.model_downloads -or $modelSmoke.processor_revision -ne 'nxmmr-anpr-ocr-vertical-v1-inputfix1-mixedfix1'){throw 'Offline model smoke failed its governed contract.'}
    $state.state='BUILT_AND_SMOKED';Write-NxJson (Join-Path $run 'correction-state.json') $state
    Assert-NxcSources
    $now=Get-NxContainers -AllowMissingWorker
    Assert-NxmCurrent $now
    Assert-NxProtected ($summary | ConvertTo-Json -Depth 20 | ConvertFrom-Json) $now
    if((Get-NxJobs $now) -ne 0 -or (Get-NxcCounts $now) -cne $counts){throw 'Retained state or jobs changed during the offline build.'}
    Wait-NxcRam $WaitForRamMinutes
    $state.state='STARTING_NEW_WORKER';Write-NxJson (Join-Path $run 'correction-state.json') $state
    $null=Invoke-NxReceiptCompose $candidateFile @('up','-d','--no-deps','--no-build','--pull','never','--force-recreate','forensic-records-worker') 240 (Join-Path $run 'replace-worker.log') -StreamOutput
    $null=Invoke-NxNative docker @('start',$before.api.Id) 90 (Join-Path $run 'restart-localai.log') -StreamOutput
    $apiDrained=$false
    $after=Wait-NxHealth 300
    Assert-NxProtected ($summary | ConvertTo-Json -Depth 20 | ConvertFrom-Json) $after
    if($after['forensic-records-worker'].Image -cne $image -or $after['forensic-records-worker'].RestartCount -ne 0){throw 'Candidate worker image/restart verification failed.'}
    Assert-NxcEnvironment $worker $after['forensic-records-worker']
    $probe=[Convert]::ToBase64String([IO.File]::ReadAllBytes((Resolve-NxPath 'scripts/nxmmr_demo_readiness_probe.py')))
    $text=Invoke-NxNative docker @('exec',$after['forensic-records-worker'].Id,'python','-c',"import base64; exec(base64.b64decode('$probe'))") 240 (Join-Path $run 'model-readiness.log')
    $line=@($text -split '\r?\n' | Where-Object {$_ -like 'NXMMR_READINESS_JSON=*'})
    if($line.Count -ne 1 -or ($line[0].Substring('NXMMR_READINESS_JSON='.Length) | ConvertFrom-Json).state -ne 'PASS'){throw 'Live model-load readiness failed.'}
    $after=Get-NxContainers;Assert-NxHealth $after;Assert-NxProtected ($summary | ConvertTo-Json -Depth 20 | ConvertFrom-Json) $after
    if((Get-NxJobs $after) -ne 0 -or (Get-NxcCounts $after) -cne $counts){throw 'Retained counts or jobs changed during worker activation.'}
    Write-NxJson (Join-Path $run 'after.json') (Get-NxSummary $after)
    $state.state='VERIFIED';Write-NxJson (Join-Path $run 'correction-state.json') $state
    Write-Host 'MIXED_OCR_WORKER_DEPLOYMENT=PASS'
    Write-Host 'Reopen Codex/browser and hard refresh. No retained evidence was reprocessed automatically.'
} catch {
    if($PreflightOnly){Write-Host 'MIXED_OCR_WORKER_PREFLIGHT=BLOCKED'}
    Write-Host "MIXED_OCR_WORKER=FAILED reason=$($_.Exception.Message)"
    if($apiDrained){
        try {$null=Invoke-NxNative docker @('start',$expected.api[0]) 90 (Join-Path $run 'failure-restart-localai.log') -StreamOutput;$apiDrained=$false}
        catch {Write-Host "LOCALAI_RESTART=FAILED reason=$($_.Exception.Message)"}
    }
    if($state -and @($state.touched).Count -gt 0){
        try {Invoke-NxcRollback $run} catch {Write-Host "ROLLBACK=BLOCKED_OR_FAILED reason=$($_.Exception.Message)"}
    } else {Write-Host 'No live service was replaced or stopped by this run.'}
    Write-Host "Receipt: $run"
    exit 3
} finally {
    if($transcript){Stop-Transcript | Out-Null}
    if($mutex){$mutex.ReleaseMutex();$mutex.Dispose()}
}
