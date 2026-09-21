# SPDX-License-Identifier: MIT
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'nxmmr_correction_common.ps1')
$script:Checks=0
function Check($Condition,[string]$Message){if(-not $Condition){throw $Message};$script:Checks++}
function MustFail([scriptblock]$Action){$failed=$false;try{& $Action}catch{$failed=$true};Check $failed 'Expected fail-closed rejection.'}
foreach($file in @('nxmmr_correction_common.ps1','deploy_nxmmr_acceptance_correction.ps1','deploy_nxmmr_mixed_ocr_worker.ps1')) {
    $errors=$null;$tokens=$null
    $null=[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot $file),[ref]$tokens,[ref]$errors)
    Check ($errors.Count -eq 0) "Parse error: $file"
}
$container=[pscustomobject]@{
    Name='/nexusai-api-1'
    HostConfig=[pscustomobject]@{
        Memory=0;NanoCpus=0;CpuShares=0;CpuPeriod=0;CpuQuota=0;MemoryReservation=0;MemorySwap=0;BlkioWeight=0;OomScoreAdj=0
        CapAdd=$null;CapDrop=$null;Devices=$null;DeviceRequests=$null;SecurityOpt=$null;Ulimits=$null;ExtraHosts=@();Dns=$null;DnsOptions=$null;DnsSearch=$null;GroupAdd=$null;VolumesFrom=$null;Links=$null
        Privileged=$false;ReadonlyRootfs=$false;AutoRemove=$false;PublishAllPorts=$false;NetworkMode='nexusai_default';RestartPolicy=@{Name='no'};Runtime='runc';IpcMode='private';ShmSize=67108864
        PortBindings=[pscustomobject]@{'8080/tcp'=@([pscustomobject]@{HostIp='127.0.0.1';HostPort='8080'})}
        LogConfig=@{Type='json-file';Config=@{}}
    }
    NetworkSettings=@{Networks=[pscustomobject]@{nexusai_default=@{}}}
    Mounts=@('/models','/backends','/configuration','/data','/tmp/generated/images' | ForEach-Object {
        $label=if($_ -eq '/tmp/generated/images'){'images'}else{$_.TrimStart('/')}
        [pscustomobject]@{Type='volume';RW=$true;Destination=$_;Name="nexusai_$label";Source="/synthetic/$label"}
    })
    Config=[pscustomobject]@{User='';WorkingDir='/';Env=@('SECRET=literal$dollar','LOCALAI_UPLOAD_LIMIT=15');Entrypoint=@('/entrypoint.sh');Cmd=@('phi-2');Healthcheck=[pscustomobject]@{Test=@('CMD-SHELL','curl ${HEALTHCHECK_ENDPOINT}');Interval=60000000000;Timeout=600000000000;Retries=10}}
}
$original=New-NxcApiCompose $container 'sha256:original'
$candidate=New-NxcApiCompose $container 'sha256:candidate' -Correction
Check ($original.services.api.environment.LOCALAI_UPLOAD_LIMIT -ceq '15') 'Original cap not preserved.'
Check ($candidate.services.api.environment.LOCALAI_UPLOAD_LIMIT -ceq '320') 'Candidate cap missing.'
Check ($candidate.services.api.environment.UPLOAD_LIMIT -ceq '320') 'Legacy cap conflicts.'
Check ($candidate.services.api.environment.SECRET -ceq 'literal$$dollar') 'Literal credential interpolation not escaped.'
Check ($candidate.services.api.healthcheck.test[1] -ceq 'curl $${HEALTHCHECK_ENDPOINT}') 'Container health environment not preserved.'
Check ($candidate.services.api.ports[0].host_ip -ceq '127.0.0.1') 'Port host binding lost.'
Check ($candidate.volumes.Count -eq 5) 'External volume mapping lost.'
Check ($candidate.services.Count -eq 1) 'Candidate expands service scope.'
Check ($candidate.services.api.command[0] -ceq 'phi-2') 'Original command lost.'
$container.HostConfig.Memory=1;MustFail {New-NxcApiCompose $container 'bad'};$container.HostConfig.Memory=0
$container.Mounts[0].RW=$false;MustFail {New-NxcApiCompose $container 'bad'};$container.Mounts[0].RW=$true
function Get-NxRam {return 5.999}
MustFail {Wait-NxcRam 0}
function Get-NxRam {return 6.0}
Wait-NxcRam 0;$script:Checks++
Assert-NxcEnvironment $container $container;$script:Checks++
MustFail {Assert-NxcEnvironment $container $container -ApiCorrection}
$deploySource=[IO.File]::ReadAllText((Join-Path $PSScriptRoot 'deploy_nxmmr_acceptance_correction.ps1'))
$smokeAt=$deploySource.IndexOf('embedded-ui-smoke.log')
$appsAt=$deploySource.IndexOf('Wait-NxcOperatorAppsClosed $WaitForRamMinutes')
$touchAt=$deploySource.IndexOf("`$state.touched=@('forensic-records-worker','api')")
$stopAt=$deploySource.IndexOf("@('stop','--timeout','30'")
Check ($deploySource.Contains('if($DrainTargetsForRam -and (-not $UseBuiltRunDirectory -or $PreflightOnly))')) 'Drain must require actual completed-build resume.'
Check ($smokeAt -ge 0 -and $smokeAt -lt $touchAt) 'Candidate smoke must precede target drain.'
Check ($appsAt -gt $smokeAt -and $appsAt -lt $touchAt) 'Operator apps must close before rollback state and target drain.'
Check ($touchAt -ge 0 -and $touchAt -lt $stopAt) 'Rollback state must be durable before target stop.'
Check ($deploySource.Contains("Assert-NxcProtected (`$summary | ConvertTo-Json -Depth 20 | ConvertFrom-Json) `$now")) 'Protected-service check missing after drain.'
Check ($deploySource.Contains("if((Get-NxJobs `$now) -ne 0 -or (Get-NxcCounts `$now) -cne `$counts)")) 'Retained/job guard missing after drain.'
Check ($deploySource.Contains("try {Invoke-NxcRollback `$run}")) 'Post-drain failures must invoke rollback.'
$mixedSource=[IO.File]::ReadAllText((Join-Path $PSScriptRoot 'deploy_nxmmr_mixed_ocr_worker.ps1'))
$mixedAppsAt=$mixedSource.IndexOf('Wait-NxcOperatorAppsClosed $WaitForRamMinutes')
$mixedBuildAt=$mixedSource.IndexOf("'build','--pull=false','--network=none'")
$mixedSmokeAt=$mixedSource.IndexOf("'model-smoke.log'")
$mixedTouchAt=$mixedSource.IndexOf("`$state.touched=@('forensic-records-worker')")
$mixedStopAt=$mixedSource.IndexOf("@('stop','--timeout','30',`$worker.Id,`$before.api.Id)")
Check ($mixedSource.Contains("Scope=replace forensic-records-worker only")) 'Mixed OCR activation must declare worker-only replacement scope.'
Check (-not $mixedSource.Contains('New-NxcApiCompose')) 'Mixed OCR activation must not construct an API replacement.'
Check ($mixedAppsAt -ge 0 -and $mixedAppsAt -lt $mixedTouchAt) 'Operator apps must close before rollback/drain state is armed.'
Check ($mixedTouchAt -ge 0 -and $mixedTouchAt -lt $mixedStopAt) 'Worker rollback state must be durable before the RAM drain.'
Check ($mixedStopAt -ge 0 -and $mixedStopAt -lt $mixedBuildAt) 'Worker and unchanged LocalAI must drain before the six-GiB build gate.'
Check ($mixedBuildAt -ge 0 -and $mixedBuildAt -lt $mixedSmokeAt) 'Network-disabled worker build must precede model smoke.'
Check ($mixedSource.Contains("try {Invoke-NxcRollback `$run}")) 'Mixed OCR post-stop failures must invoke rollback.'
Check ($mixedSource.Contains("Assert-NxProtected (`$summary | ConvertTo-Json -Depth 20 | ConvertFrom-Json) `$after")) 'Mixed OCR activation must preserve every non-worker service.'
Check ($mixedSource.Contains("docker @('start',`$before.api.Id)")) 'Original LocalAI container must restart without recreation.'
Check ($mixedSource.Contains("'PADDLE_PDX_CACHE_HOME=/tmp/.paddlex'")) 'Read-only Paddle smoke must use an ephemeral writable cache.'
Check ($mixedSource.Contains('Get-Process -Name PhoneExperienceHost,LockApp')) 'Optional Windows recovery must remain narrowly allowlisted.'
Check ($mixedSource.Contains("'echo 1 > /proc/sys/vm/drop_caches'")) 'Optional Linux recovery must release clean page cache only.'
Check (-not $mixedSource.Contains('echo 3 > /proc/sys/vm/drop_caches')) 'Optional Linux recovery must not evict inode/dentry caches.'
Check ($mixedSource.Contains("Sync=false")) 'Optional Linux recovery must disclose that sync is not invoked.'
$wrapped=[pscustomobject]@{api=[pscustomobject]@{value=$container}}
Check ((Get-NxcSnapshotContainer $wrapped 'api').Config.Cmd[0] -ceq 'phi-2') 'Serialized snapshot wrapper must unwrap deterministically.'
Write-Host "CORRECTION_OPERATOR_SELFTESTS=PASS Checks=$script:Checks RuntimeMutated=false"
