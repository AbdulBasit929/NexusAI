# SPDX-License-Identifier: MIT
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'nxmmr_demo_operator_common.ps1')

$script:NxSttManifestPath = Resolve-NxPath 'configuration/nxmmr_stt_query_activation_v1.json'
$script:NxSttIntegrityPath = Resolve-NxPath 'configuration/nxmmr_stt_query_operator_integrity_v1.json'
$script:NxSttChanged = @('api', 'forensic-records-api', 'forensic-records-worker')
$script:NxSttProtected = @('forensic-postgres', 'forensic-nats')
$script:NxSttRunRoot = Resolve-NxPath 'local-acceptance-models/nxmmr/private-activation-stt-query'

function Get-NxSttManifest {
    $seal = Get-Content -LiteralPath $script:NxSttIntegrityPath -Raw | ConvertFrom-Json
    if ($seal.contract_version -ne 'nexusai.nxmmr.stt-query-operator-integrity/v1') { throw 'STT/query integrity contract is invalid.' }
    foreach ($property in $seal.files.PSObject.Properties) {
        if ((Get-NxHash (Resolve-NxPath $property.Name)) -cne $property.Value) { throw "Source integrity mismatch: $($property.Name)" }
    }
    $manifest = Get-Content -LiteralPath $script:NxSttManifestPath -Raw | ConvertFrom-Json
    if ($manifest.contract_version -ne 'nexusai.nxmmr.stt-query-activation/v1') { throw 'STT/query activation manifest is invalid.' }
    if (($manifest.services.build_and_recreate_no_deps -join ',') -cne ($script:NxSttChanged -join ',')) { throw 'Changed service scope differs from the admitted three-service boundary.' }
    if (($manifest.services.must_remain_identical -join ',') -cne ($script:NxSttProtected -join ',')) { throw 'Protected service scope changed.' }
    $roles = ConvertTo-NxMap $manifest.worker_environment
    foreach ($name in @('FORENSIC_ASR_ENABLED','FORENSIC_IMAGE_EMBEDDING_ENABLED','FORENSIC_FACE_ENABLED','FORENSIC_ANPR_ENABLED','FORENSIC_OCR_ENABLED','FORENSIC_VIDEO_ANPR_V3_ENABLED')) {
        if ($roles[$name] -cne 'true') { throw "Required worker role is not enabled: $name" }
    }
    foreach ($name in @('FORENSIC_VIDEO_ANPR_V2_ENABLED','FORENSIC_TTS_ENABLED')) {
        if ($roles[$name] -cne 'false') { throw "Forbidden worker role is enabled: $name" }
    }
    if ($roles['FORENSIC_ASR_MODEL'] -cne 'faster-whisper-small-ur' -or $roles['HF_HUB_OFFLINE'] -cne '1' -or $roles['TRANSFORMERS_OFFLINE'] -cne '1') { throw 'ASR or offline model policy changed.' }
    return $manifest
}

function Get-NxSttAssets($Manifest, [switch]$DestinationRequired) {
    $sourceRoot = [IO.Path]::GetFullPath([string]$Manifest.asset_plan.source_root).TrimEnd('\','/')
    $allowedRoot = [IO.Path]::GetFullPath('C:\Users\sheik\.cache\nexusai\models\google-siglip-base-patch16-224\7fd15f0689c79d79e38b1c2e2e2370a7bf2761ed').TrimEnd('\','/')
    if ($sourceRoot -cne $allowedRoot) { throw 'SigLIP source root is not the exact admitted cache revision.' }
    if (-not (Test-Path -LiteralPath $sourceRoot -PathType Container)) { throw 'Admitted SigLIP source cache is missing.' }
    $destinationRoot = Resolve-NxPath ([string]$Manifest.asset_plan.destination)
    $rows = New-Object 'System.Collections.Generic.List[object]'
    foreach ($file in $Manifest.asset_plan.files) {
        if ([IO.Path]::GetFileName([string]$file.name) -cne [string]$file.name) { throw 'Invalid asset filename.' }
        $source = Join-Path $sourceRoot $file.name
        $destination = Join-Path $destinationRoot $file.name
        if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { throw "SigLIP source asset missing: $($file.name)" }
        if ((Get-Item -LiteralPath $source).Length -ne [int64]$file.size_bytes -or (Get-NxHash $source) -cne [string]$file.sha256) { throw "SigLIP source asset mismatch: $($file.name)" }
        if ($DestinationRequired -or (Test-Path -LiteralPath $destination)) {
            if (-not (Test-Path -LiteralPath $destination -PathType Leaf) -or (Get-Item -LiteralPath $destination).Length -ne [int64]$file.size_bytes -or (Get-NxHash $destination) -cne [string]$file.sha256) { throw "SigLIP destination conflict: $destination" }
        }
        $rows.Add([pscustomobject]@{name=$file.name;source=$source;destination=$destination;size_bytes=[int64]$file.size_bytes;sha256=$file.sha256})
    }
    if ($rows.Count -ne 7) { throw 'Exactly seven SigLIP files are required.' }
    return ,$rows.ToArray()
}

function Copy-NxSttAssets($Manifest) {
    $assets = Get-NxSttAssets $Manifest
    foreach ($asset in $assets) {
        if (Test-Path -LiteralPath $asset.destination) { continue }
        $null = New-Item -ItemType Directory -Force -Path (Split-Path -Parent $asset.destination)
        [IO.File]::Copy($asset.source, $asset.destination, $false)
        (Get-Item -LiteralPath $asset.destination).IsReadOnly = $true
    }
    return Get-NxSttAssets $Manifest -DestinationRequired
}

function Get-NxSttContainers {
    $all = Get-NxContainers
    foreach ($name in @($script:NxSttChanged + $script:NxSttProtected)) {
        if (-not $all.Contains($name)) { throw "Required service is missing: $name" }
    }
    return $all
}

function Assert-NxSttProtected($Before, $After) {
    foreach ($name in $script:NxSttProtected) {
        if ($Before.$name.id -cne $After[$name].Id -or $Before.$name.image -cne $After[$name].Image -or $Before.$name.restart_count -ne $After[$name].RestartCount) { throw "Protected service changed: $name" }
    }
}

function Get-NxSttModelInventory($Manifest) {
    $response = Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:8080/v1/models' -TimeoutSec 15
    if ($response.StatusCode -ne 200) { throw 'LocalAI model inventory is unavailable.' }
    $payload = $response.Content | ConvertFrom-Json
    $ids = @($payload.data | ForEach-Object { [string]$_.id })
    foreach ($model in $Manifest.localai_required_models) {
        if ($ids -cnotcontains [string]$model) { throw "Required existing LocalAI model is missing: $model" }
    }
    return @($Manifest.localai_required_models)
}

function ConvertTo-NxSttDuration([int64]$Nanoseconds) {
    if ($Nanoseconds -le 0) { return '0s' }
    return "$($Nanoseconds)ns"
}

function New-NxSttService($Container, [string]$ServiceName, [string]$Image, $Overrides, [string]$Dockerfile, [switch]$Build) {
    if ($Container.HostConfig.NetworkMode -cne 'nexusai_default' -or $Container.HostConfig.Privileged -or $Container.HostConfig.ReadonlyRootfs -or $Container.HostConfig.Memory -ne 0 -or $Container.HostConfig.NanoCpus -ne 0 -or ($null -ne $Container.HostConfig.CapAdd -and @($Container.HostConfig.CapAdd).Count -gt 0)) { throw "Unsupported host configuration for exact rollback: $ServiceName" }
    $environment = [ordered]@{}
    foreach ($entry in $Container.Config.Env) { $pair = $entry -split '=',2; $environment[$pair[0]] = $pair[1] }
    foreach ($name in $Overrides.Keys) { $environment[$name] = [string]$Overrides[$name] }
    foreach ($value in $environment.Values) { if ([string]$value -match '\$') { throw "Literal dollar requires explicit Compose review: $ServiceName" } }
    $allowedVolumes = @{
        'api'=@('nexusai_models','nexusai_images','nexusai_backends','nexusai_configuration','nexusai_data')
        'forensic-records-api'=@('nexusai_forensic_spool')
        'forensic-records-worker'=@('nexusai_forensic_spool')
    }
    $volumes = [ordered]@{}; $mounts = @()
    foreach ($mount in $Container.Mounts) {
        if ($mount.Type -eq 'volume' -and $allowedVolumes[$ServiceName] -ccontains [string]$mount.Name) {
            $volumes[$mount.Name] = @{external=$true;name=$mount.Name}
            $mounts += @{type='volume';source=$mount.Name;target=$mount.Destination;read_only=(-not $mount.RW)}
        } elseif ($ServiceName -eq 'forensic-records-worker' -and $mount.Type -eq 'bind' -and $mount.Destination -eq '/models/media' -and -not $mount.RW) {
            $observed = $mount.Source.Replace('\','/') -replace '^/run/desktop/mnt/host/([A-Za-z])/', '$1:/' -replace '^/host_mnt/([A-Za-z])/', '$1:/'
            if ($observed.TrimEnd('/') -ine (Resolve-NxPath 'models/media').Replace('\','/').TrimEnd('/')) { throw 'Worker media bind does not target the authoritative models/media directory.' }
            $mounts += @{type='bind';source=(Resolve-NxPath 'models/media');target='/models/media';read_only=$true;bind=@{create_host_path=$false}}
        } else { throw "Unexpected mount for ${ServiceName}: $($mount.Destination)" }
    }
    $expectedMounts = if ($ServiceName -eq 'api') { 5 } elseif ($ServiceName -eq 'forensic-records-worker') { 2 } else { 1 }
    if ($mounts.Count -ne $expectedMounts) { throw "Mount count differs from the admitted topology: $ServiceName" }
    $ports = @()
    foreach ($port in $Container.HostConfig.PortBindings.PSObject.Properties) {
        foreach ($binding in $port.Value) {
            $portParts = $port.Name -split '/'
            $spec = @{target=[int]$portParts[0];published=$binding.HostPort;protocol=$portParts[1]}
            if ($binding.HostIp) { $spec.host_ip=$binding.HostIp }
            $ports += $spec
        }
    }
    $service = [ordered]@{image=$Image;container_name=$Container.Name.TrimStart('/');environment=$environment;restart=$Container.HostConfig.RestartPolicy.Name;ports=$ports;volumes=$mounts;networks=@('default');logging=@{driver=$Container.HostConfig.LogConfig.Type;options=$Container.HostConfig.LogConfig.Config};stop_grace_period='30s'}
    if ($Container.Config.User) { $service.user=$Container.Config.User }
    if ($Container.Config.WorkingDir) { $service.working_dir=$Container.Config.WorkingDir }
    if ($Container.Config.Entrypoint) { $service.entrypoint=@($Container.Config.Entrypoint) }
    if ($Container.Config.Cmd) { $service.command=@($Container.Config.Cmd) }
    if ($Container.Config.PSObject.Properties['Healthcheck'] -and $Container.Config.Healthcheck -and $Container.Config.Healthcheck.Test) {
        $health = $Container.Config.Healthcheck
        $startPeriod=0;if($health.PSObject.Properties['StartPeriod']){$startPeriod=[int64]$health.StartPeriod}
        $healthTest=@($health.Test)
        if($healthTest.Count -ge 2 -and $healthTest[0] -ceq 'CMD-SHELL'){$healthTest[1]=$healthTest[1].Replace('$','$$')}
        $service.healthcheck=@{test=$healthTest;interval=(ConvertTo-NxSttDuration $health.Interval);timeout=(ConvertTo-NxSttDuration $health.Timeout);retries=$health.Retries;start_period=(ConvertTo-NxSttDuration $startPeriod)}
    }
    if ($Build) { $service.build=@{context=$script:NxRoot;dockerfile=$Dockerfile} }
    return [pscustomobject]@{service=$service;volumes=$volumes}
}

function New-NxSttCompose($Containers, $Images, $WorkerOverrides, $Manifest, [switch]$Build) {
    $services=[ordered]@{}; $volumes=[ordered]@{}
    foreach ($name in $script:NxSttChanged) {
        $overrides = if ($name -eq 'forensic-records-worker') { $WorkerOverrides } else { [ordered]@{} }
        $entry = New-NxSttService $Containers[$name] $name ([string]$Images[$name]) $overrides ([string]$Manifest.services.dockerfiles.$name) -Build:$Build
        $services[$name]=$entry.service
        foreach ($volume in $entry.volumes.Keys) { $volumes[$volume]=$entry.volumes[$volume] }
    }
    return [ordered]@{services=$services;networks=@{default=@{external=$true;name='nexusai_default'}};volumes=$volumes}
}

function Test-NxSttGateValues($Facts, $Manifest) {
    $failures=New-Object 'System.Collections.Generic.List[object]'
    foreach ($item in @(@('ram_gib',[double]$Manifest.resource_gates.minimum_available_ram_gib,'minimum'),@('disk_gib',[double]$Manifest.resource_gates.minimum_workspace_free_disk_gib,'minimum'),@('active_jobs',[int]$Manifest.resource_gates.maximum_active_jobs,'equal'))) {
        $value=$Facts[$item[0]]; $pass=$null -ne $value
        if ($pass) { $pass=if($item[2] -eq 'minimum'){$value -ge $item[1]}else{$value -eq $item[1]} }
        if (-not $pass) { $failures.Add([pscustomobject]@{gate=$item[0];measured=$value;required=$item[1]}) }
    }
    foreach ($name in @('docker','health','integrity','assets','models','rollback','compose')) { if ($Facts[$name] -ne $true) { $failures.Add([pscustomobject]@{gate=$name;measured=$Facts[$name];required=$true}) } }
    return ,$failures.ToArray()
}

function Assert-NxSttRam($Manifest) {
    $ram=Get-NxRam; $required=[double]$Manifest.resource_gates.minimum_available_ram_gib
    Write-Host "AvailableRAMGiB=$ram RequiredRAMGiB=$required"
    if ($ram -lt $required) { throw 'RAM gate failed; no further build or recreation is allowed.' }
}

function Invoke-NxSttWslMemoryCommand([ValidateSet('dirty','writeback','drop-clean-cache')][string]$Command) {
    # wsl.exe on this host interprets quoted option switches as a Linux command.
    # These are fixed, non-user-controlled commands with no shell interpreter.
    $arguments = switch($Command) {
        'dirty' {'--distribution docker-desktop --user root --exec grep ^Dirty: /proc/meminfo'}
        'writeback' {'--distribution docker-desktop --user root --exec grep ^Writeback: /proc/meminfo'}
        'drop-clean-cache' {'--distribution docker-desktop --user root --exec /sbin/sysctl -w vm.drop_caches=1'}
    }
    $info=New-Object Diagnostics.ProcessStartInfo
    $info.FileName=(Join-Path $env:WINDIR 'System32\wsl.exe');$info.Arguments=$arguments
    $info.WorkingDirectory=$script:NxRoot;$info.UseShellExecute=$false;$info.CreateNoWindow=$true
    $info.RedirectStandardOutput=$true;$info.RedirectStandardError=$true
    $process=New-Object Diagnostics.Process;$process.StartInfo=$info;$started=$false
    try {
        if(-not $process.Start()){throw 'Could not start fixed WSL memory command.'};$started=$true
        $stdoutTask=$process.StandardOutput.ReadToEndAsync();$stderrTask=$process.StandardError.ReadToEndAsync()
        if(-not $process.WaitForExit(30000)){$process.Kill();$process.WaitForExit();throw 'Fixed WSL memory command timed out.'}
        if($process.ExitCode -ne 0){throw "Fixed WSL memory command failed: $Command (exit $($process.ExitCode))."}
        return $stdoutTask.Result.Trim()
    } finally {
        if($started -and -not $process.HasExited){$process.Kill()}
        $process.Dispose()
    }
}

function Invoke-NxSttCleanLinuxPageCache {
    for($attempt=1;$attempt -le 6;$attempt++){
        $dirtyLine=Invoke-NxSttWslMemoryCommand dirty
        $writebackLine=Invoke-NxSttWslMemoryCommand writeback
        $lines="$dirtyLine`n$writebackLine"
        $values=@{}
        foreach($line in ($lines -split '\r?\n')){
            if($line -match '^(Dirty|Writeback):\s+([0-9]+)\s+kB$'){$values[$Matches[1]]=[int64]$Matches[2]}
        }
        if($values.Count -eq 2 -and $values.Dirty -eq 0 -and $values.Writeback -eq 0){
            $null=Invoke-NxSttWslMemoryCommand drop-clean-cache
            Write-Host 'BUILD_CACHE_RAM_RECOVERY=PASS Mode=1 DirtyKiB=0 WritebackKiB=0 Sync=false'
            return $true
        }
        $dirty=if($values.ContainsKey('Dirty')){$values.Dirty}else{'unknown'}
        $writeback=if($values.ContainsKey('Writeback')){$values.Writeback}else{'unknown'}
        Write-Host "BUILD_CACHE_RAM_RECOVERY=WAITING DirtyKiB=$dirty WritebackKiB=$writeback"
        Start-Sleep -Seconds 5
    }
    Write-Host 'BUILD_CACHE_RAM_RECOVERY=SKIPPED reason=dirty_or_writeback_nonzero'
    return $false
}

function Wait-NxSttRam($Manifest, [int]$WaitMinutes=30, [switch]$RecoverBuildCache) {
    $required=[double]$Manifest.resource_gates.minimum_available_ram_gib
    $deadline=[DateTime]::UtcNow.AddMinutes($WaitMinutes)
    $nextRecovery=[DateTime]::MinValue
    do {
        $ram=Get-NxRam
        Write-Host "AvailableRAMGiB=$ram RequiredRAMGiB=$required"
        if($ram -ge $required){return $ram}
        if($RecoverBuildCache -and [DateTime]::UtcNow -ge $nextRecovery){
            $nextRecovery=[DateTime]::UtcNow.AddSeconds(30)
            $null=Invoke-NxSttCleanLinuxPageCache
            continue
        }
        if([DateTime]::UtcNow -ge $deadline){break}
        Write-Host 'RAM_GATE=WAITING reason=post_build_cache_pressure'
        Start-Sleep -Seconds 10
    } while([DateTime]::UtcNow -lt $deadline)
    throw "RAM gate remained below $required GiB for $WaitMinutes minutes; completed candidate images remain resumable and no live service was recreated."
}

function Invoke-NxSttPreflight {
    $facts=[ordered]@{ram_gib=$null;disk_gib=$null;active_jobs=$null;docker=$false;health=$false;integrity=$false;assets=$false;models=$false;rollback=$false;compose=$false}
    $errors=New-Object 'System.Collections.Generic.List[string]';$manifest=$null;$containers=$null;$assets=@();$models=@();$head='';$dirty=@()
    try {$head=Invoke-NxNative git @('rev-parse','HEAD');$dirty=@((Invoke-NxNative git @('status','--short')) -split '\r?\n' | Where-Object {$_})} catch {$errors.Add($_.Exception.Message)}
    try {$facts.ram_gib=Get-NxRam;$drive=([IO.Path]::GetPathRoot($script:NxRoot)).Substring(0,1);$facts.disk_gib=[double](Get-PSDrive -Name $drive).Free/1GB} catch {$errors.Add($_.Exception.Message)}
    try {$manifest=Get-NxSttManifest;$facts.integrity=$true;$assets=Get-NxSttAssets $manifest;$facts.assets=$true} catch {$errors.Add($_.Exception.Message)}
    try {
        $null=Invoke-NxNative docker @('info','--format','{{.ServerVersion}}');$facts.docker=$true
        $containers=Get-NxSttContainers;Assert-NxHealth $containers;$facts.health=$true;$facts.active_jobs=Get-NxJobs $containers
        if ($manifest) {
            $models=Get-NxSttModelInventory $manifest;$facts.models=$true
            $images=[ordered]@{};foreach($name in $script:NxSttChanged){$images[$name]=$containers[$name].Image}
            $rollback=New-NxSttCompose $containers $images ([ordered]@{}) $manifest
            $temp=Join-Path ([IO.Path]::GetTempPath()) ('nxmmr-stt-query-'+[guid]::NewGuid().ToString('n')+'.json')
            try {Write-NxJson $temp $rollback;$null=Invoke-NxReceiptCompose $temp @('config','--quiet');$facts.rollback=$true;$facts.compose=$true} finally {if(Test-Path -LiteralPath $temp){[IO.File]::Delete($temp)}}
        }
    } catch {$errors.Add($_.Exception.Message)}
    $failures=if($manifest){Test-NxSttGateValues $facts $manifest}else{@([pscustomobject]@{gate='manifest';measured=$false;required=$true})}
    $pass=$failures.Count -eq 0 -and $errors.Count -eq 0
    return [pscustomobject]@{status=$(if($pass){'PASS'}else{'BLOCKED'});utc=[DateTime]::UtcNow.ToString('o');git_head=$head;dirty_count=$dirty.Count;dirty_summary=$dirty;facts=$facts;failures=$failures;errors=$errors.ToArray();containers=$(if($containers){Get-NxSummary $containers}else{$null});assets=$assets;models=$models;runtime_mutated=$false;retained_state_mutated=$false}
}

function Show-NxSttPreflight($Receipt) {
    Write-Host "STT_QUERY_ACTIVATION_PREFLIGHT=$($Receipt.status)"
    foreach($failure in $Receipt.failures){Write-Host "BLOCKED reason=$($failure.gate) measured=$($failure.measured) required=$($failure.required)"}
    foreach($message in $Receipt.errors){Write-Host "BLOCKED reason=$message measured=unverified required=verified"}
    Write-Host "RAMGiB=$($Receipt.facts.ram_gib) DiskGiB=$($Receipt.facts.disk_gib) ActiveJobs=$($Receipt.facts.active_jobs) DirtyEntries=$($Receipt.dirty_count)"
    Write-Host 'RuntimeMutated=false DatabaseMigration=false VolumesChanged=false RetainedStateMutated=false'
}

function New-NxSttRunDirectory {
    if (-not (Test-Path -LiteralPath $script:NxSttRunRoot)) { $null=New-Item -ItemType Directory -Force -Path $script:NxSttRunRoot }
    $path=Join-Path $script:NxSttRunRoot ([DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'))
    $null=New-Item -ItemType Directory -Path $path
    $acl=Get-Acl -LiteralPath $path;$acl.SetAccessRuleProtection($true,$false)
    foreach($sid in @([Security.Principal.WindowsIdentity]::GetCurrent().User.Value,'S-1-5-18','S-1-5-32-544')){$identity=New-Object Security.Principal.SecurityIdentifier($sid);$rule=New-Object Security.AccessControl.FileSystemAccessRule($identity,'FullControl','ContainerInherit,ObjectInherit','None','Allow');$acl.AddAccessRule($rule)}
    Set-Acl -LiteralPath $path -AclObject $acl
    return $path
}

function Get-NxSttRunDirectory([string]$RunDirectory) {
    if (-not $RunDirectory) {$pointer=Join-Path $script:NxSttRunRoot 'latest.json';$RunDirectory=(Get-Content -LiteralPath $pointer -Raw|ConvertFrom-Json).run_directory}
    $full=[IO.Path]::GetFullPath($RunDirectory)
    if (-not $full.StartsWith($script:NxSttRunRoot+'\',[StringComparison]::OrdinalIgnoreCase)) { throw 'Receipt path is outside the STT/query private activation root.' }
    return $full
}

function Enter-NxSttOperatorLock {
    $mutex=New-Object Threading.Mutex($false,'Local\NexusAI_NXMMR_STT_Query_Activation_V1')
    try {if(-not $mutex.WaitOne(0)){throw 'Another STT/query activation or rollback is running.'}} catch [Threading.AbandonedMutexException] {$mutex.ReleaseMutex();$mutex.Dispose();throw 'Prior operator process ended unexpectedly; inspect receipts and rollback.'} catch {$mutex.Dispose();throw}
    return $mutex
}
