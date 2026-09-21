# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$PreflightOnly,[ValidateRange(0,120)][int]$WaitForRamMinutes=0,[string]$RollbackDirectory='',[string]$UseBuiltRunDirectory='',[switch]$DrainTargetsForRam)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'nxmmr_correction_common.ps1')
$run='';$mutex=$null;$transcript=$false;$state=$null
try {
    $mutex=Enter-NxOperatorLock
    Assert-NxcSources
    if($RollbackDirectory -and $UseBuiltRunDirectory){throw 'Choose rollback or completed-build resume, never both.'}
    if($DrainTargetsForRam -and (-not $UseBuiltRunDirectory -or $PreflightOnly)){throw 'Target drain is admitted only for an actual reviewed completed-build resume.'}
    if($RollbackDirectory){Invoke-NxcRollback (Get-NxRunDirectory $RollbackDirectory);exit 0}
    $run=New-NxRunDirectory
    Start-Transcript -LiteralPath (Join-Path $run 'correction.log') | Out-Null;$transcript=$true
    Write-Host "CorrectionReceiptDirectory=$run"
    Write-Host 'Only worker and LocalAI/UI are in scope. Keep browsers closed; do not upload during this run.'
    $before=Get-NxContainers;Assert-NxHealth $before;Assert-NxcInitial $before
    if((Get-NxJobs $before) -ne 0){throw 'Active processing jobs: no build/replacement allowed.'}
    $disk=[double](Get-PSDrive -Name ([IO.Path]::GetPathRoot($script:NxRoot).Substring(0,1))).Free/1GB
    if($disk -lt 12){throw 'Free disk below 12 GiB.'}
    $manifest=Get-Content $script:NxManifestPath -Raw | ConvertFrom-Json
    $null=Get-NxAssets $manifest -DestinationRequired
    foreach($p in $manifest.worker_environment.PSObject.Properties){
        if(@($before['forensic-records-worker'].Config.Env | Where-Object {$_ -ceq ($p.Name+'='+$p.Value)}).Count -ne 1){throw "Live worker role differs: $($p.Name)"}
    }
    $summary=Get-NxSummary $before
    Write-NxJson (Join-Path $run 'before.json') $summary
    Write-NxJson (Join-Path $run 'private-container-snapshot.json') $before
    $counts=Get-NxcCounts $before
    $state=[ordered]@{contract='nxmmr.correction/v1';state='PREPARED';touched=@();images=[ordered]@{};rollback_hashes=[ordered]@{};counts_before=$counts;before_hash=(Get-NxHash (Join-Path $run 'before.json'))}
    $stamp=Split-Path $run -Leaf
    $tags=@{api="nexusai/localai-forensic:nxmmr-correction-$stamp";'forensic-records-worker'="nexusai/forensic-records-worker:nxmmr-correction-$stamp"}
    if($UseBuiltRunDirectory){
        $tags=Get-NxcCompletedImages $UseBuiltRunDirectory
        Write-Host 'COMPLETED_IMAGES=VERIFIED BuildSkipped=true Downloads=false'
        Write-NxJson (Join-Path $run 'resume-origin.json') @{run_directory=$UseBuiltRunDirectory;images=$tags;build_skipped=$true}
    }
    foreach($name in @('api','forensic-records-worker')) {
        $original=$before[$name]
        $rollback=if($name -eq 'api'){New-NxcApiCompose $original $original.Image}else{New-NxWorkerCompose $original $original.Image ([ordered]@{})}
        $candidate=if($name -eq 'api'){New-NxcApiCompose $original $tags[$name] -Correction}else{New-NxWorkerCompose $original $tags[$name] ([ordered]@{})}
        $rollbackFile=Join-Path $run "rollback-$name.json"
        Write-NxJson $rollbackFile $rollback
        Write-NxJson (Join-Path $run "candidate-$name.json") $candidate
        $state.rollback_hashes[$name]=Get-NxHash $rollbackFile
        $null=Invoke-NxReceiptCompose $rollbackFile @('config','--quiet')
        $null=Invoke-NxReceiptCompose (Join-Path $run "candidate-$name.json") @('config','--quiet')
    }
    Write-NxJson (Join-Path $run 'correction-state.json') $state
    if(-not $DrainTargetsForRam){Wait-NxcRam $WaitForRamMinutes}
    if($PreflightOnly){Write-Host 'CORRECTION_PREFLIGHT=PASS RuntimeMutated=false';exit 0}
    $null=Invoke-NxNative python @('-B',(Resolve-NxPath 'scripts/run_nxmmr_anpr_ocr_vertical_selftests.py')) 120 (Join-Path $run 'ocr-selftests.log') -StreamOutput
    $null=Invoke-NxNative node @('--test',(Resolve-NxPath 'core/http/react-ui/src/utils/uploadPolicy.test.js')) 120 (Join-Path $run 'upload-selftests.log') -StreamOutput
    # Rollback tags prevent the original image IDs becoming unreferenced after replacement.
    foreach($name in @('api','forensic-records-worker')) {
        $null=Invoke-NxNative docker @('image','tag',$before[$name].Image,"nexusai/correction-rollback:$name-$stamp")
    }
    if(-not $UseBuiltRunDirectory){
    $state.state='BUILDING';Write-NxJson (Join-Path $run 'correction-state.json') $state
    Assert-NxRam
    Write-Host 'Building OCR-only worker layer; no package/model downloads.'
    $null=Invoke-NxNative docker @('build','--pull=false','--network=none','--progress=plain','--build-arg',"BASE_IMAGE=nexusai/correction-rollback:forensic-records-worker-$stamp",'-f',(Resolve-NxPath 'configuration/nxmmr_correction_worker.Dockerfile'),'-t',$tags['forensic-records-worker'],$script:NxRoot) 600 (Join-Path $run 'build-worker.log') -StreamOutput
    Assert-NxRam
    Write-Host 'Building LocalAI and embedded UI with cached layers, GOMAXPROCS=4 and GOFLAGS=-p=2. Build-only package downloads may require Internet; no models are installed.'
    # The Dockerfile-specific ignore keeps local credentials out of COPY . . .
    $dockerfile=Join-Path $run 'Dockerfile'
    [IO.File]::Copy((Resolve-NxPath 'Dockerfile'),$dockerfile,$false)
    [IO.File]::WriteAllText(($dockerfile+'.dockerignore'),([IO.File]::ReadAllText((Resolve-NxPath '.dockerignore'))+"`n.env`n.env.*`n**/.env`n**/.env.*`n"),(New-Object Text.UTF8Encoding($false)))
    $null=Invoke-NxNative docker @('build','--pull=false','--progress=plain','--build-arg','IMAGE_TYPE=core','--build-arg','BASE_IMAGE=ubuntu:24.04','--build-arg','LOCALAI_BUILD_GOMAXPROCS=4','--build-arg','LOCALAI_BUILD_GOFLAGS=-p=2','-f',$dockerfile,'-t',$tags.api,$script:NxRoot) 7200 (Join-Path $run 'build-api.log') -StreamOutput
    }
    foreach($name in @('api','forensic-records-worker')) {
        $state.images[$name]=Invoke-NxNative docker @('image','inspect',$tags[$name],'--format','{{.Id}}')
        $file=Join-Path $run "candidate-$name.json"
        $candidate=Get-Content $file -Raw | ConvertFrom-Json
        $candidate.services.$name.image=$state.images[$name]
        Write-NxJson $file $candidate
    }
    $code='import hashlib,pathlib,multilingual_ocr; assert multilingual_ocr.PROCESSOR_REVISION == "nxmmr-anpr-ocr-vertical-v1-inputfix1-mixedfix1"; print(hashlib.sha256(pathlib.Path("/app/multilingual_ocr.py").read_bytes()).hexdigest())'
    $hash=Invoke-NxNative docker @('run','--rm','--pull','never','--network','none','--read-only','--entrypoint','python',$state.images['forensic-records-worker'],'-c',$code) 120 (Join-Path $run 'worker-source-smoke.log')
    if($hash -cne (Get-NxHash (Resolve-NxPath 'ingestion/forensic_records/multilingual_ocr.py'))){throw 'Worker image source mismatch.'}
    $null=Invoke-NxNative docker @('run','--rm','--pull','never','--network','none','--read-only','--entrypoint','/local-ai',$state.images.api,'--version') 120 (Join-Path $run 'api-version-smoke.log')
    $null=Invoke-NxNative docker @('run','--rm','--pull','never','--network','none','--read-only','--entrypoint','sh',$state.images.api,'-c','grep -aqF "Waiting means selected, not uploaded" /local-ai && grep -aqF "300 MiB" /local-ai') 120 (Join-Path $run 'embedded-ui-smoke.log')
    $state.state='BUILT_AND_SMOKED'
    Write-NxJson (Join-Path $run 'correction-state.json') $state
    Assert-NxcSources
    $now=Get-NxContainers;Assert-NxHealth $now;Assert-NxcInitial $now
    Assert-NxcProtected ($summary | ConvertTo-Json -Depth 20 | ConvertFrom-Json) $now
    if((Get-NxJobs $now) -ne 0 -or (Get-NxcCounts $now) -cne $counts){throw 'Retained state/jobs changed during build; no replacement.'}
    if($DrainTargetsForRam){
        # The old targets are the dominant irreducible WSL allocation. Drain
        # only after candidates, rollback snapshots, identities and zero jobs
        # are verified. Mark both touched first so every later failure restores.
        Wait-NxcOperatorAppsClosed $WaitForRamMinutes
        $state.touched=@('forensic-records-worker','api');$state.state='DRAINING_OLD_TARGETS'
        Write-NxJson (Join-Path $run 'correction-state.json') $state
        Write-Host 'Stopping only the old worker and LocalAI to return their RAM. Forensic API, PostgreSQL and NATS remain running.'
        $null=Invoke-NxNative docker @('stop','--timeout','30',$before['forensic-records-worker'].Id,$before.api.Id) 90 (Join-Path $run 'drain-targets.log') -StreamOutput
        $now=Get-NxContainers
        Assert-NxcProtected ($summary | ConvertTo-Json -Depth 20 | ConvertFrom-Json) $now
        if($now['forensic-records-worker'].State.Running -or $now.api.State.Running){throw 'A target did not stop cleanly.'}
        if((Get-NxJobs $now) -ne 0 -or (Get-NxcCounts $now) -cne $counts){throw 'Retained guard changed during target drain.'}
        Wait-NxcRam $WaitForRamMinutes
        foreach($name in @('forensic-records-worker','api')) {
            Assert-NxRam
            $state.state="STARTING_$($name.ToUpperInvariant())"
            Write-NxJson (Join-Path $run 'correction-state.json') $state
            $null=Invoke-NxReceiptCompose (Join-Path $run "candidate-$name.json") @('up','-d','--no-deps','--no-build','--pull','never','--force-recreate',$name) 240 (Join-Path $run "replace-$name.log") -StreamOutput
        }
    } else {
        # Wait after the build if necessary; then recheck identity/jobs immediately.
        Wait-NxcRam $WaitForRamMinutes
        foreach($name in @('forensic-records-worker','api')) {
            Assert-NxRam
            $now=Get-NxContainers
            Assert-NxcProtected ($summary | ConvertTo-Json -Depth 20 | ConvertFrom-Json) $now
            if((Get-NxJobs $now) -ne 0 -or $now[$name].Id -cne $before[$name].Id){throw 'Job/service changed before replacement.'}
            $state.touched+=@($name);$state.state='REPLACING'
            Write-NxJson (Join-Path $run 'correction-state.json') $state
            $null=Invoke-NxReceiptCompose (Join-Path $run "candidate-$name.json") @('up','-d','--no-deps','--no-build','--pull','never','--force-recreate',$name) 240 (Join-Path $run "replace-$name.log") -StreamOutput
            $null=Wait-NxHealth 300
        }
    }
    $after=Wait-NxHealth 300
    Assert-NxcProtected ($summary | ConvertTo-Json -Depth 20 | ConvertFrom-Json) $after
    foreach($name in @('api','forensic-records-worker')) {
        if($after[$name].Image -cne $state.images[$name] -or $after[$name].RestartCount -ne 0){throw 'Candidate image/restart verification failed.'}
        Assert-NxcEnvironment $before[$name] $after[$name] -ApiCorrection:($name -eq 'api')
    }
    $probe=[Convert]::ToBase64String([IO.File]::ReadAllBytes((Resolve-NxPath 'scripts/nxmmr_demo_readiness_probe.py')))
    $text=Invoke-NxNative docker @('exec',$after['forensic-records-worker'].Id,'python','-c',"import base64; exec(base64.b64decode('$probe'))") 240 (Join-Path $run 'model-readiness.log')
    $line=@($text -split '\r?\n' | Where-Object {$_ -like 'NXMMR_READINESS_JSON=*'})
    if($line.Count -ne 1 -or ($line[0].Substring('NXMMR_READINESS_JSON='.Length) | ConvertFrom-Json).state -ne 'PASS'){throw 'Model-load readiness failed.'}
    # GET header probe does not upload a body or register evidence.
    $probeCode='import socket; s=socket.create_connection(("host.docker.internal",8080),10); s.sendall(b"GET /readyz HTTP/1.1\r\nHost: localhost\r\nContent-Length: 314572800\r\nConnection: close\r\n\r\n"); status=s.recv(1024).split(b"\r\n",1)[0]; print(status.decode()); assert b" 200 " in status; s.close()'
    $null=Invoke-NxNative docker @('exec',$after['forensic-records-worker'].Id,'python','-c',$probeCode) 30 (Join-Path $run 'upload-header-probe.log')
    $after=Get-NxContainers;Assert-NxHealth $after
    if((Get-NxJobs $after) -ne 0 -or (Get-NxcCounts $after) -cne $counts){throw 'Retained counts/jobs changed; investigate before claiming deployment verification.'}
    Write-NxJson (Join-Path $run 'after.json') (Get-NxSummary $after)
    $state.state='VERIFIED';Write-NxJson (Join-Path $run 'correction-state.json') $state
    Write-Host 'CORRECTION_DEPLOYMENT=PASS'
    Write-Host 'Reopen Codex/browser. Hard refresh. Product acceptance and retained negative retry remain pending; no automatic upload/reprocessing.'
} catch {
    if($PreflightOnly){Write-Host 'CORRECTION_PREFLIGHT=BLOCKED'}
    Write-Host "CORRECTION=FAILED reason=$($_.Exception.Message)"
    if($state -and @($state.touched).Count -gt 0){
        try {Invoke-NxcRollback $run} catch {Write-Host "ROLLBACK=BLOCKED_OR_FAILED reason=$($_.Exception.Message)"}
    } else {Write-Host 'No live services replaced by this run.'}
    Write-Host "Receipt: $run"
    exit 3
} finally {
    if($transcript){Stop-Transcript | Out-Null}
    if($mutex){$mutex.ReleaseMutex();$mutex.Dispose()}
}
