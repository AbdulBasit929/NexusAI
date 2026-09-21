# SPDX-License-Identifier: MIT
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'nxmmr_demo_operator_common.ps1')

function Assert-NxcSources {
    $seal=Get-Content (Resolve-NxPath 'configuration/nxmmr_correction_operator_integrity.json') -Raw | ConvertFrom-Json
    foreach($p in $seal.files.PSObject.Properties) {
        if((Get-NxHash (Resolve-NxPath $p.Name)) -cne $p.Value){throw "Correction source changed: $($p.Name). Review/reseal; do not bypass."}
    }
    foreach($p in $seal.trees.PSObject.Properties) {
        if((Get-NxTreeHash (Resolve-NxPath $p.Name) -ExcludeBytecode) -cne $p.Value){throw "Sealed source tree changed: $($p.Name)"}
    }
}
function Wait-NxcRam([int]$Minutes) {
    $deadline=[DateTime]::UtcNow.AddMinutes($Minutes)
    do {
        $ram=Get-NxRam
        Write-Host ('AvailableRAMGiB={0:N3} RequiredRAMGiB=6' -f $ram)
        if($ram -ge 6){return}
        if([DateTime]::UtcNow -ge $deadline){throw 'RAM gate not met. No gate override; close apps or preserve this receipt for review.'}
        Start-Sleep -Seconds 10
    } while($true)
}
function Wait-NxcOperatorAppsClosed([int]$Minutes) {
    $deadline=[DateTime]::UtcNow.AddMinutes($Minutes)
    do {
        $apps=@(Get-Process -Name ChatGPT,codex -ErrorAction SilentlyContinue)
        if(-not $apps.Count){Write-Host 'OPERATOR_APPS_CLOSED=PASS';return}
        $working=[math]::Round((($apps|Measure-Object WorkingSet64 -Sum).Sum/1MB),0)
        Write-Host "OPERATOR_APPS_CLOSED=WAITING Processes=$($apps.Count) WorkingSetMiB=$working Close ChatGPT/Codex completely."
        if([DateTime]::UtcNow -ge $deadline){throw 'ChatGPT/Codex remained open. Target services were not stopped.'}
        Start-Sleep -Seconds 5
    } while($true)
}
function Get-NxcCompletedImages([string]$Directory) {
    $pin=Get-Content (Resolve-NxPath 'configuration/nxmmr_correction_completed_build.json') -Raw | ConvertFrom-Json
    $path=Get-NxRunDirectory $Directory
    $expected=Resolve-NxPath $pin.run_directory
    if($path -ine $expected){throw 'This resume path is not the reviewed completed build receipt.'}
    foreach($p in $pin.receipt_hashes.PSObject.Properties){
        if((Get-NxHash (Join-Path $path $p.Name)) -cne $p.Value){throw "Completed build receipt changed: $($p.Name)"}
    }
    $prior=Get-Content (Join-Path $path 'correction-state.json') -Raw | ConvertFrom-Json
    if($prior.contract -ne 'nxmmr.correction/v1' -or @($prior.touched).Count -ne 0){throw 'Prior run already touched live services; use its guarded recovery, not resume.'}
    $images=@{}
    foreach($p in $pin.images.PSObject.Properties){
        $candidate=Get-Content (Join-Path $path "candidate-$($p.Name).json") -Raw | ConvertFrom-Json
        if($candidate.services.($p.Name).image -cne $p.Value){throw 'Completed candidate image differs from reviewed immutable ID.'}
        $actual=Invoke-NxNative docker @('image','inspect',$p.Value,'--format','{{.Id}}')
        if($actual -cne $p.Value){throw 'Completed image is not present locally; no download/rebuild fallback is allowed.'}
        $images[$p.Name]=$p.Value
    }
    if($images.Count -ne 2 -or -not $images.ContainsKey('api') -or -not $images.ContainsKey('forensic-records-worker')){throw 'Completed image scope is invalid.'}
    return $images
}
function Assert-NxcInitial($Containers) {
    $expected=@{
        api=@('d015424c9d74fa6caba16c08b501b19157d2d34b1c22f9f75741c5c818ba81ce','sha256:628f56af541ac739887ddfb6f08d661e1070c4d0a2d343fb902882ab0f5dba9a')
        'forensic-records-worker'=@('baec952b54bca1c4ed83ac13a2b172e709f4e096522e60851e0c7da95fac9e4f','sha256:e495788b85f07d9a4552c07796f8b810fa4f600ecd96a08acfa6c2afbb02ce73')
    }
    foreach($name in $expected.Keys) {
        if($Containers[$name].Id -cne $expected[$name][0] -or $Containers[$name].Image -cne $expected[$name][1]){throw "Initial identity changed: $name. Do not run old or guessed commands."}
    }
    $hostHashes=@{api='e4c4f4f7f94cf67314dff779d6a44960f0ffd681b5749a2b1885122eaf45f9fb';'forensic-records-worker'='d93a73c14ecd21044db437c5bbc0ee8bb485f52dee98cd0ea4276e61256431c7'}
    foreach($name in $hostHashes.Keys) {
        $json=$Containers[$name].HostConfig | ConvertTo-Json -Depth 50 -Compress
        $sha=[Security.Cryptography.SHA256]::Create()
        try {$hash=([BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($json)))).Replace('-','').ToLowerInvariant()} finally {$sha.Dispose()}
        if($hash -cne $hostHashes[$name]){throw "Host configuration changed: $name. Snapshot serializer needs renewed review."}
    }
}
function Assert-NxcProtected($Before,$Now) {
    foreach($name in @('forensic-records-api','forensic-postgres','forensic-nats')) {
        if($Now[$name].Id -cne $Before.$name.id -or $Now[$name].Image -cne $Before.$name.image -or $Now[$name].RestartCount -ne $Before.$name.restart_count){throw "Protected service changed: $name"}
    }
}
function Get-NxcCounts($Containers) {
    $sql='SELECT (SELECT count(*) FROM forensic.evidence_items),(SELECT count(*) FROM forensic.evidence_versions),(SELECT count(*) FROM forensic.records_ingest_jobs),(SELECT count(*) FROM forensic.records),(SELECT count(*) FROM forensic.derived_artifacts),(SELECT count(*) FROM forensic.kb_collection_assets);'
    return Invoke-NxNative docker @('exec',$Containers['forensic-postgres'].Id,'psql','-U','localrecall','-d','localrecall','-Atc',$sql)
}
function ConvertTo-NxcLiteral([string]$Text) { return $Text.Replace('$','$$') }
function New-NxcApiCompose($Container,[string]$Image,[switch]$Correction) {
    $h=$Container.HostConfig
    # This serializer admits only the inspected topology, not arbitrary Docker options.
    foreach($key in @('Memory','NanoCpus','CpuShares','CpuPeriod','CpuQuota','MemoryReservation','MemorySwap','BlkioWeight','OomScoreAdj')) {
        if($h.$key -ne 0){throw "Unsupported API host option: $key"}
    }
    foreach($key in @('CapAdd','CapDrop','Devices','DeviceRequests','SecurityOpt','Ulimits','ExtraHosts','Dns','DnsOptions','DnsSearch','GroupAdd','VolumesFrom','Links')) {
        if(@($h.$key | Where-Object {$null -ne $_}).Count){throw "Unsupported API host option: $key"}
    }
    if($h.Privileged -or $h.ReadonlyRootfs -or $h.AutoRemove -or $h.PublishAllPorts -or $h.NetworkMode -ne 'nexusai_default' -or $h.RestartPolicy.Name -ne 'no' -or $h.Runtime -ne 'runc' -or $h.IpcMode -ne 'private' -or $h.ShmSize -ne 67108864){throw 'Unsupported API host topology.'}
    if(@($Container.NetworkSettings.Networks.PSObject.Properties).Count -ne 1){throw 'Unexpected API networks.'}
    $expected=@{'/models'='nexusai_models';'/backends'='nexusai_backends';'/configuration'='nexusai_configuration';'/data'='nexusai_data';'/tmp/generated/images'='nexusai_images'}
    $mounts=@();$volumes=[ordered]@{}
    if(@($Container.Mounts).Count -ne 5){throw 'API mount count changed.'}
    foreach($mount in $Container.Mounts) {
        if($mount.Type -ne 'volume' -or -not $mount.RW -or $expected[$mount.Destination] -cne $mount.Name){throw 'API mount contract changed.'}
        $volumes[$mount.Name]=@{external=$true;name=$mount.Name}
        $mounts+=@{type='volume';source=$mount.Name;target=$mount.Destination;read_only=$false}
    }
    $envMap=[ordered]@{}
    foreach($entry in $Container.Config.Env){$pair=$entry -split '=',2;$envMap[$pair[0]]=ConvertTo-NxcLiteral $pair[1]}
    if($Correction){$envMap['LOCALAI_UPLOAD_LIMIT']='320';$envMap['UPLOAD_LIMIT']='320'}
    $ports=@()
    foreach($p in $h.PortBindings.PSObject.Properties){foreach($binding in $p.Value){
        $spec=@{target=[int]($p.Name -split '/')[0];published=$binding.HostPort;protocol=($p.Name -split '/')[1]}
        if($binding.HostIp){$spec.host_ip=$binding.HostIp};$ports+=$spec
    }}
    $service=[ordered]@{image=$Image;container_name=$Container.Name.TrimStart('/');user=$Container.Config.User;working_dir=$Container.Config.WorkingDir;environment=$envMap;entrypoint=@($Container.Config.Entrypoint | ForEach-Object {ConvertTo-NxcLiteral $_});command=@($Container.Config.Cmd | ForEach-Object {ConvertTo-NxcLiteral $_});restart='no';ports=$ports;volumes=$mounts;networks=@('default');logging=@{driver=$h.LogConfig.Type;options=$h.LogConfig.Config};stop_grace_period='30s'}
    $health=$Container.Config.Healthcheck
    $service.healthcheck=@{test=@($health.Test | ForEach-Object {ConvertTo-NxcLiteral $_});interval="$($health.Interval)ns";timeout="$($health.Timeout)ns";retries=$health.Retries}
    if($health.PSObject.Properties['StartPeriod']){$service.healthcheck.start_period="$($health.StartPeriod)ns"}
    return [ordered]@{services=@{api=$service};networks=@{default=@{external=$true;name='nexusai_default'}};volumes=$volumes}
}
function Assert-NxcEnvironment($Original,$Current,[switch]$ApiCorrection) {
    $want=@{};$actual=@{}
    foreach($entry in $Original.Config.Env){$p=$entry -split '=',2;$want[$p[0]]=$p[1]}
    foreach($entry in $Current.Config.Env){$p=$entry -split '=',2;$actual[$p[0]]=$p[1]}
    if($ApiCorrection){$want['LOCALAI_UPLOAD_LIMIT']='320';$want['UPLOAD_LIMIT']='320'}
    foreach($key in $want.Keys){if(-not $actual.ContainsKey($key) -or $actual[$key] -cne $want[$key]){throw "Runtime environment mismatch: $key"}}
    if(@($Original.Mounts).Count -ne @($Current.Mounts).Count){throw 'Runtime mount count changed.'}
    foreach($mount in $Original.Mounts){
        $match=@($Current.Mounts | Where-Object {$_.Destination -ceq $mount.Destination -and $_.Type -ceq $mount.Type -and $_.Source -ceq $mount.Source -and $_.RW -eq $mount.RW})
        if($match.Count -ne 1){throw "Runtime mount changed: $($mount.Destination)"}
    }
}
function Get-NxcSnapshotContainer($Snapshot,[string]$Name) {
    $entry=$Snapshot.PSObject.Properties[$Name].Value
    if($entry.PSObject.Properties['Config']){return $entry}
    if($entry.PSObject.Properties['value'] -and $entry.value.PSObject.Properties['Config']){return $entry.value}
    throw "Private snapshot container shape is invalid: $Name"
}
function Invoke-NxcRollback([string]$Run) {
    $state=Get-Content (Join-Path $Run 'correction-state.json') -Raw | ConvertFrom-Json
    if($state.contract -ne 'nxmmr.correction/v1' -or @($state.touched).Count -eq 0){throw 'No correction replacement recorded to roll back.'}
    if((Get-NxHash (Join-Path $Run 'before.json')) -cne $state.before_hash){throw 'Original summary hash mismatch.'}
    $before=Get-Content (Join-Path $Run 'before.json') -Raw | ConvertFrom-Json
    # Do not require six GiB to restore known-good images, but never interrupt jobs.
    $savedServices=$script:NxServices
    try {
        $script:NxServices=@('forensic-records-api','forensic-postgres','forensic-nats')
        $now=Get-NxContainers
    } finally {$script:NxServices=$savedServices}
    Assert-NxcProtected $before $now
    if((Get-NxJobs $now) -ne 0){throw 'Active jobs: manual intervention required; rollback will not interrupt evidence processing.'}
    foreach($name in @('api','forensic-records-worker')) {
        if($name -notin $state.touched){continue}
        $file=Join-Path $Run ("rollback-$name.json")
        if((Get-NxHash $file) -cne $state.rollback_hashes.$name){throw 'Rollback snapshot hash mismatch.'}
        $compose=Get-Content $file -Raw | ConvertFrom-Json
        if(@($compose.services.PSObject.Properties).Count -ne 1 -or $compose.services.$name.image -cne $before.$name.image){throw 'Invalid rollback scope/image.'}
        $id=Invoke-NxNative docker @('ps','-aq','--filter','label=com.docker.compose.project=nexusai','--filter',"label=com.docker.compose.service=$name",'--filter',"name=^/nexusai-$name-1`$")
        if($id){
            $image=Invoke-NxNative docker @('inspect',$id,'--format','{{.Image}}')
            if($image -cne $before.$name.image -and $image -cne $state.images.$name){throw 'Unexpected image: refusing to replace a different operator deployment.'}
        }
        $null=Invoke-NxReceiptCompose $file @('up','-d','--no-deps','--no-build','--pull','never','--force-recreate',$name) 240 (Join-Path $Run "rollback-$name.log") -StreamOutput
    }
    $after=Wait-NxHealth 300; Assert-NxcProtected $before $after
    foreach($name in @($state.touched)) {if($after[$name].Image -cne $before.$name.image){throw 'Rollback image verification failed.'}}
    $original=Get-Content (Join-Path $Run 'private-container-snapshot.json') -Raw | ConvertFrom-Json
    foreach($name in @($state.touched)){Assert-NxcEnvironment (Get-NxcSnapshotContainer $original $name) $after[$name]}
    if((Get-NxJobs $after) -ne 0 -or (Get-NxcCounts $after) -cne $state.counts_before){throw 'Rollback service restore completed, but retained counts/jobs differ; investigate.'}
    Write-NxJson (Join-Path $Run 'rollback-result.json') @{state='PASS';containers=(Get-NxSummary $after)}
    Write-Host 'CORRECTION_ROLLBACK=PASS'
}
