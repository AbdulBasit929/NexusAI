# SPDX-License-Identifier: MIT
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$script:NxRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$script:NxManifestPath = Join-Path $script:NxRoot 'configuration/nxmmr_demo_activation_v1.json'
$script:NxServices = @('forensic-records-worker','forensic-records-api','api','forensic-postgres','forensic-nats')

function Resolve-NxPath([string]$Relative) {
    $path = [IO.Path]::GetFullPath((Join-Path $script:NxRoot $Relative))
    if (-not $path.StartsWith($script:NxRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Path escapes repository.' }
    $cursor = $path
    while ($cursor -and $cursor.Length -gt $script:NxRoot.Length) {
        if ((Test-Path -LiteralPath $cursor) -and ((Get-Item -LiteralPath $cursor -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw "Reparse point is not admitted: $cursor" }
        $cursor = Split-Path -Parent $cursor
    }
    return $path
}
function Get-NxHash([string]$Path) { return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant() }
function Get-NxTreeHash([string]$Path, [switch]$ExcludeBytecode) {
    $root = [IO.Path]::GetFullPath($Path).TrimEnd('\','/')
    $rows = @(Get-ChildItem -LiteralPath $root -Recurse -File -Force | Where-Object { -not $ExcludeBytecode -or $_.FullName -notmatch '[\\/]__pycache__[\\/]' })
    if (-not $rows.Count) { throw "Empty model/source tree: $Path" }
    $entries = @{}
    foreach ($row in $rows) {
        if ($row.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Model/source links are not admitted.' }
        $entries[$row.FullName.Substring($root.Length + 1).Replace('\','/')] = $row.FullName
    }
    [string[]]$names = @($entries.Keys); [Array]::Sort($names, [StringComparer]::Ordinal)
    $stream = New-Object IO.MemoryStream
    try {
        foreach ($name in $names) {
            $bytes = [Text.Encoding]::UTF8.GetBytes($name)
            $length = [BitConverter]::GetBytes([int]$bytes.Length); [Array]::Reverse($length)
            $stream.Write($length,0,4); $stream.Write($bytes,0,$bytes.Length)
            $hex = Get-NxHash $entries[$name]
            $digest = New-Object byte[] 32
            for ($i=0; $i -lt 32; $i++) { $digest[$i] = [Convert]::ToByte($hex.Substring(2*$i,2),16) }
            $stream.Write($digest,0,32)
        }
        $sha = [Security.Cryptography.SHA256]::Create()
        try { return ([BitConverter]::ToString($sha.ComputeHash($stream.ToArray()))).Replace('-','').ToLowerInvariant() } finally { $sha.Dispose() }
    } finally { $stream.Dispose() }
}
function ConvertTo-NxMap($Object) {
    $map = [ordered]@{}
    foreach ($p in $Object.PSObject.Properties) { $map[$p.Name] = $p.Value }
    return $map
}
function Write-NxJson([string]$Path, $Value) { [IO.File]::WriteAllText($Path, ($Value | ConvertTo-Json -Depth 70), (New-Object Text.UTF8Encoding($false))) }
function ConvertTo-NxArgument([string]$Value) {
    # Windows CommandLineToArgvW quoting, including embedded quotes and final slashes.
    return '"' + [regex]::Replace([regex]::Replace($Value, '(\\*)"', '$1$1\"'), '(\\+)$', '$1$1') + '"'
}
function Receive-NxLiveOutput($Process, [int]$TimeoutSeconds, [string]$LogPath) {
    # Opt-in only for safe build output. Inspect/config commands can contain secrets.
    $writer=New-Object IO.StreamWriter($LogPath,$false,(New-Object Text.UTF8Encoding($false)))
    $writer.AutoFlush=$true
    $stdout=New-Object Text.StringBuilder
    $clock=[Diagnostics.Stopwatch]::StartNew(); $timedOut=$false
    try {
        $reads=@($Process.StandardOutput.ReadLineAsync(),$Process.StandardError.ReadLineAsync())
        while ($null -ne $reads[0] -or $null -ne $reads[1] -or -not $Process.HasExited) {
            if (-not $timedOut -and $clock.Elapsed.TotalSeconds -ge $TimeoutSeconds) {
                $timedOut=$true
                if (-not $Process.HasExited) { $Process.Kill() }
            }
            # A descendant holding a pipe must not turn the command deadline into an unbounded wait.
            if ($timedOut -and $clock.Elapsed.TotalSeconds -ge ($TimeoutSeconds+5)) { break }
            foreach ($i in 0,1) {
                if ($null -ne $reads[$i] -and $reads[$i].IsCompleted) {
                    $line=$reads[$i].GetAwaiter().GetResult()
                    if ($null -eq $line) { $reads[$i]=$null; continue }
                    $writer.WriteLine($line)
                    Write-Host $line
                    if ($i -eq 0) { $null=$stdout.AppendLine($line) }
                    $reader=if($i -eq 0){$Process.StandardOutput}else{$Process.StandardError}
                    $reads[$i]=$reader.ReadLineAsync()
                }
            }
            if (@($reads | Where-Object {$null -ne $_ -and $_.IsCompleted}).Count -eq 0) { Start-Sleep -Milliseconds 20 }
        }
        if ($timedOut) {
            $message="Owned command timed out after $TimeoutSeconds seconds; do not retry activation blindly."
            $writer.WriteLine($message); Write-Host $message
            throw $message
        }
        if ($Process.ExitCode -ne 0) { throw "Native command failed (exit $($Process.ExitCode)); logs: $LogPath." }
        return $stdout.ToString().Trim()
    } finally { $writer.Dispose() }
}
function Invoke-NxNative([string]$File, [string[]]$Arguments, [int]$TimeoutSeconds=60, [string]$LogPath='', [switch]$StreamOutput) {
    if ($StreamOutput -and -not $LogPath) { throw 'Live output requires a durable log path.' }
    $info = New-Object Diagnostics.ProcessStartInfo
    $info.FileName=$File; $info.Arguments=($Arguments | ForEach-Object { ConvertTo-NxArgument $_ }) -join ' '
    $info.WorkingDirectory=$script:NxRoot; $info.UseShellExecute=$false; $info.CreateNoWindow=$true
    $info.RedirectStandardOutput=$true; $info.RedirectStandardError=$true
    $process = New-Object Diagnostics.Process; $process.StartInfo=$info; $started=$false
    try {
        if (-not $process.Start()) { throw 'Could not start bounded command.' }
        $started=$true
        if ($StreamOutput) { return Receive-NxLiveOutput $process $TimeoutSeconds $LogPath }
        $outTask=$process.StandardOutput.ReadToEndAsync(); $errTask=$process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit($TimeoutSeconds*1000)) {
            # Only the child command created above is terminated, never user apps/services.
            $process.Kill(); $process.WaitForExit()
            if ($LogPath) { [IO.File]::WriteAllText($LogPath, $outTask.Result + "`n" + $errTask.Result) }
            throw "Owned command timed out after $TimeoutSeconds seconds; do not retry activation blindly."
        }
        $stdout=$outTask.Result; $stderr=$errTask.Result
        if ($LogPath) { [IO.File]::WriteAllText($LogPath, $stdout + "`n" + $stderr) }
        if ($process.ExitCode -ne 0) { throw "Native command failed (exit $($process.ExitCode)); logs, if applicable: $LogPath. Raw environment/credentials are intentionally not printed." }
        return $stdout.Trim()
    } finally {
        if ($started -and -not $process.HasExited) { $process.Kill() }
        $process.Dispose()
    }
}
function Get-NxManifest {
    $lock = Get-Content -LiteralPath (Resolve-NxPath 'configuration/nxmmr_demo_operator_integrity.json') -Raw | ConvertFrom-Json
    foreach ($p in $lock.files.PSObject.Properties) {
        if ((Get-NxHash (Resolve-NxPath $p.Name)) -cne $p.Value) { throw "Bundle/source hash mismatch: $($p.Name)" }
    }
    foreach ($p in $lock.trees.PSObject.Properties) {
        if ((Get-NxTreeHash (Resolve-NxPath $p.Name) -ExcludeBytecode) -cne $p.Value) { throw "Source tree mismatch: $($p.Name)" }
    }
    $m=Get-Content -LiteralPath $script:NxManifestPath -Raw | ConvertFrom-Json
    if ($m.contract_version -ne 'nexusai.nxmmr.demo-activation/v1' -or $m.license_admission -ne 'ADMITTED_WITH_NOTICE') { throw 'Activation manifest schema/license invalid.' }
    if (($m.service_delta_when_gate_passes.build -join ',') -ne 'forensic-records-worker' -or ($m.service_delta_when_gate_passes.recreate_no_deps -join ',') -ne 'forensic-records-worker') { throw 'Only worker activation is admitted.' }
    if ($m.activation_preflight.minimum_available_ram_gib -ne 6 -or $m.activation_preflight.minimum_workspace_free_disk_gib -ne 12) { throw 'Resource floor changed.' }
    $admission=Get-Content -LiteralPath (Resolve-NxPath 'configuration/nxmmr_demo_model_admission_v1.json') -Raw | ConvertFrom-Json
    if ($admission.overall_demo_admission -ne 'ADMITTED_WITH_NOTICE' -or @($admission.assets | Where-Object {$_.demo_admission -ne 'ADMITTED_WITH_NOTICE'}).Count) { throw 'License admission invalid.' }
    $roles=ConvertTo-NxMap $m.worker_environment
    foreach ($name in @('FORENSIC_ASR_ENABLED','FORENSIC_FACE_ENABLED','FORENSIC_IMAGE_EMBEDDING_ENABLED','FORENSIC_VIDEO_ANPR_V2_ENABLED','FORENSIC_TTS_ENABLED')) { if ($roles[$name] -cne 'false') { throw "Forbidden role: $name" } }
    foreach ($name in @('FORENSIC_ANPR_ENABLED','FORENSIC_OCR_ENABLED','FORENSIC_VIDEO_ANPR_V3_ENABLED')) { if ($roles[$name] -cne 'true') { throw "Missing demo role: $name" } }
    return $m
}
function Get-NxRam { return [double](Get-CimInstance Win32_OperatingSystem).FreePhysicalMemory / 1MB }
function Assert-NxRam { $ram=Get-NxRam; Write-Host "AvailableRAMGiB=$ram RequiredRAMGiB=6"; if ($ram -lt 6) { throw "RAM gate: measured=$ram required=6 GiB. No build/recreate is allowed." } }
function Test-NxGateValues($Facts) {
    $failures=New-Object 'System.Collections.Generic.List[object]'
    foreach ($item in @(@('ram_gib',6,'minimum'),@('disk_gib',12,'minimum'),@('active_jobs',0,'equal'))) {
        $value=$Facts[$item[0]]; $pass=($null -ne $value)
        if ($pass) { if ($item[2] -eq 'minimum') {$pass=$value -ge $item[1]} else {$pass=$value -eq $item[1]} }
        if (-not $pass) { $failures.Add([pscustomobject]@{gate=$item[0]; measured=$value; required=$item[1]}) }
    }
    foreach ($name in @('docker','compose','health','hashes','license','rollback')) {
        if ($Facts[$name] -ne $true) { $failures.Add([pscustomobject]@{gate=$name; measured=$Facts[$name]; required=$true}) }
    }
    return ,$failures.ToArray()
}
function Get-NxContainers([switch]$AllowMissingWorker) {
    $result=[ordered]@{}
    foreach ($name in $script:NxServices) {
        $ids=Invoke-NxNative docker @('ps','-aq','--filter','label=com.docker.compose.project=nexusai','--filter',"label=com.docker.compose.service=$name",'--filter',"name=^/nexusai-$name-1`$")
        if (-not $ids -and $name -eq 'forensic-records-worker' -and $AllowMissingWorker) { continue }
        if (@($ids -split '\r?\n' | Where-Object {$_}).Count -ne 1) { throw "Expected one container: $name" }
        $raw=Invoke-NxNative docker @('inspect',$ids)
        $result[$name]=@($raw | ConvertFrom-Json)[0]
    }
    return $result
}
function Get-NxSummary($Containers) {
    $out=[ordered]@{}
    foreach($name in $Containers.Keys) {
        $c=$Containers[$name]; $health='none'
        if ($c.State.PSObject.Properties['Health']) { $health=$c.State.Health.Status }
        $out[$name]=[ordered]@{id=$c.Id; image=$c.Image; status=$c.State.Status; health=$health; restart_count=$c.RestartCount}
    }
    return $out
}
function Assert-NxHealth($Containers) {
    foreach($name in $Containers.Keys) {
        $c=$Containers[$name]
        if (-not $c.State.Running -or $c.State.Restarting -or $c.State.OOMKilled) { throw "Service is not healthy/running: $name" }
        if ($name -ne 'forensic-records-api') { if (-not $c.State.PSObject.Properties['Health'] -or $c.State.Health.Status -ne 'healthy') { throw "Health check failed: $name" } }
    }
    foreach($uri in @('http://127.0.0.1:8091/healthz','http://127.0.0.1:8080/readyz','http://127.0.0.1:8080/','http://127.0.0.1:9109/metrics','http://127.0.0.1:8222/healthz')) {
        $response=Invoke-WebRequest -UseBasicParsing -Uri $uri -TimeoutSec 10
        if ($response.StatusCode -ne 200) { throw "HTTP health failure: $uri" }
    }
}
function Get-NxJobs($Containers) {
    $sql="SELECT count(*) FROM forensic.records_ingest_jobs WHERE status NOT IN ('completed','dead_letter');"
    return [int](Invoke-NxNative docker @('exec',$Containers['forensic-postgres'].Id,'psql','-U','localrecall','-d','localrecall','-Atc',$sql))
}
function Get-NxAssets($Manifest, [switch]$DestinationRequired) {
    $rows=New-Object 'System.Collections.Generic.List[object]'
    foreach($asset in $Manifest.asset_plan) {
        $source=Resolve-NxPath $asset.source; $destination=Resolve-NxPath $asset.destination
        if ($asset.PSObject.Properties['sha256']) {
            if ((Get-NxHash $source) -cne $asset.sha256 -or (Get-Item -LiteralPath $source).Length -ne $asset.size_bytes) { throw "Source asset mismatch: $($asset.source)" }
            if ($DestinationRequired -or (Test-Path -LiteralPath $destination)) { if ((Get-NxHash $destination) -cne $asset.sha256) { throw "Destination conflict: $destination" } }
            $rows.Add([pscustomobject]@{source=$source; destination=$destination; sha256=$asset.sha256; kind='file'})
        } else {
            foreach($tree in $asset.tree_sha256.PSObject.Properties) {
                $s=Join-Path $source $tree.Name; $d=Join-Path $destination $tree.Name
                if ((Get-NxTreeHash $s) -cne $tree.Value) { throw "Source tree mismatch: $s" }
                if ($DestinationRequired -or (Test-Path -LiteralPath $d)) { if ((Get-NxTreeHash $d) -cne $tree.Value) { throw "Destination tree conflict: $d" } }
                $rows.Add([pscustomobject]@{source=$s; destination=$d; sha256=$tree.Value; kind='tree'})
            }
        }
    }
    return ,$rows.ToArray()
}
function Get-NxComposeArguments($Manifest) {
    $args=@('compose','--project-name','nexusai','--env-file',(Resolve-NxPath $Manifest.operator_bundle.runtime_env_file))
    foreach($file in $Manifest.operator_bundle.compose_files) { $args+=@('-f',(Resolve-NxPath $file)) }
    return ,$args
}
function New-NxWorkerCompose($Container, [string]$Image, $Overrides, [switch]$Build) {
    $envMap=[ordered]@{}
    foreach($item in $Container.Config.Env) { $pair=$item -split '=',2; $envMap[$pair[0]]=$pair[1] }
    foreach($name in $Overrides.Keys) { $envMap[$name]=[string]$Overrides[$name] }
    foreach($value in $envMap.Values) { if($value.Contains('$')) { throw 'Literal dollar in worker environment requires explicit Compose escaping review.' } }
    $volumes=[ordered]@{}; $mounts=@()
    foreach($mount in $Container.Mounts) {
        if ($mount.Type -eq 'volume') {
            $volumes[$mount.Name]=@{external=$true; name=$mount.Name}
            $mounts+=@{type='volume';source=$mount.Name;target=$mount.Destination;read_only=(-not $mount.RW)}
        } elseif ($mount.Type -eq 'bind' -and $mount.Destination -eq '/models/media' -and -not $mount.RW) {
            $observed=$mount.Source.Replace('\','/') -replace '^/run/desktop/mnt/host/([A-Za-z])/', '$1:/'
            $observed=$observed -replace '^/host_mnt/([A-Za-z])/', '$1:/'
            if($observed.TrimEnd('/') -ine (Resolve-NxPath 'models/media').Replace('\','/').TrimEnd('/')){throw 'Media mount source is not the authoritative models/media directory.'}
            $mounts+=@{type='bind';source=(Resolve-NxPath 'models/media');target='/models/media';read_only=$true;bind=@{create_host_path=$false}}
        } else { throw "Unexpected rollback mount: $($mount.Destination)" }
    }
    if ($mounts.Count -ne 2 -or -not $volumes.Contains('nexusai_forensic_spool')) { throw 'Worker mount contract differs from the admitted topology.' }
    if ($Container.HostConfig.NetworkMode -ne 'nexusai_default' -or $Container.HostConfig.Privileged -or $Container.HostConfig.ReadonlyRootfs -or $Container.HostConfig.Memory -ne 0 -or $Container.HostConfig.NanoCpus -ne 0 -or ($null -ne $Container.HostConfig.CapAdd -and @($Container.HostConfig.CapAdd).Count -gt 0)) { throw 'Unsupported worker host configuration; cannot guarantee exact rollback.' }
    $ports=@()
    foreach($port in $Container.HostConfig.PortBindings.PSObject.Properties) {
        foreach($binding in $port.Value) {
            $spec=@{target=[int]($port.Name -split '/')[0];published=$binding.HostPort;protocol=($port.Name -split '/')[1]}
            if ($binding.HostIp) { $spec.host_ip=$binding.HostIp }
            $ports+=$spec
        }
    }
    $worker=[ordered]@{image=$Image;container_name=$Container.Name.TrimStart('/');user=$Container.Config.User;working_dir=$Container.Config.WorkingDir;environment=$envMap;entrypoint=@($Container.Config.Entrypoint);restart=$Container.HostConfig.RestartPolicy.Name;ports=$ports;volumes=$mounts;networks=@('default');logging=@{driver=$Container.HostConfig.LogConfig.Type;options=$Container.HostConfig.LogConfig.Config};stop_grace_period='30s'}
    if ($Container.Config.Cmd) { $worker.command=@($Container.Config.Cmd) }
    $h=$Container.Config.Healthcheck
    $worker.healthcheck=@{test=@($h.Test);interval="$($h.Interval)ns";timeout="$($h.Timeout)ns";retries=$h.Retries;start_period="$($h.StartPeriod)ns"}
    if ($Build) { $worker.build=@{context=$script:NxRoot;dockerfile='ingestion/forensic_records/Dockerfile'} }
    return [ordered]@{services=@{'forensic-records-worker'=$worker};networks=@{default=@{external=$true;name='nexusai_default'}};volumes=$volumes}
}
function Invoke-NxPreflight {
    $facts=[ordered]@{ram_gib=$null;disk_gib=$null;active_jobs=$null;docker=$false;compose=$false;health=$false;hashes=$false;license=$false;rollback=$false}
    $errors=New-Object 'System.Collections.Generic.List[string]'; $containers=$null; $manifest=$null; $assets=@(); $head=''; $dirty=@()
    try { $head=Invoke-NxNative git @('rev-parse','HEAD'); $dirty=@((Invoke-NxNative git @('status','--short')) -split '\r?\n' | Where-Object {$_}) } catch {$errors.Add($_.Exception.Message)}
    try {$facts.ram_gib=Get-NxRam; $facts.disk_gib=[double](Get-PSDrive -Name ([IO.Path]::GetPathRoot($script:NxRoot).Substring(0,1))).Free/1GB} catch {$errors.Add($_.Exception.Message)}
    try {$manifest=Get-NxManifest; $assets=Get-NxAssets $manifest; $facts.hashes=$true; $facts.license=$true} catch {$errors.Add($_.Exception.Message)}
    try {
        $null=Invoke-NxNative docker @('info','--format','{{.ServerVersion}}'); $facts.docker=$true
        $containers=Get-NxContainers
        Assert-NxHealth $containers; $facts.health=$true
        $facts.active_jobs=Get-NxJobs $containers
        if ($manifest) {
            $null=Invoke-NxNative docker ((Get-NxComposeArguments $manifest)+@('config','--quiet')); $facts.compose=$true
            $c=$containers['forensic-records-worker']
            if ($c.Id -cne $manifest.rollback.current_worker_container -or $c.Image -cne $manifest.rollback.current_worker_image) { throw 'Rollback worker identity changed; refresh/review the bundle, do not guess.' }
            $null=Invoke-NxNative docker @('image','inspect',$c.Image,'--format','{{.Id}}')
            $null=New-NxWorkerCompose $c $c.Image ([ordered]@{})
            $facts.rollback=$true
        }
    } catch {$errors.Add($_.Exception.Message)}
    $failures=Test-NxGateValues $facts
    $pass=$failures.Count -eq 0 -and $errors.Count -eq 0
    $summary=$null; if($containers){$summary=Get-NxSummary $containers}
    return [pscustomobject]@{status=$(if($pass){'PASS'}else{'BLOCKED'});utc=[DateTime]::UtcNow.ToString('o');git_head=$head;dirty_count=$dirty.Count;dirty_summary=$dirty;manifest_sha256=(Get-NxHash $script:NxManifestPath);facts=$facts;failures=$failures;errors=$errors.ToArray();containers=$summary;assets=$assets;RuntimeMutated=$false;LiveModelsChanged=$false;RetainedStateMutated=$false}
}
function Show-NxPreflight($Receipt) {
    Write-Host "ACTIVATION_PREFLIGHT=$($Receipt.status)"
    foreach($f in $Receipt.failures) { Write-Host "BLOCKED reason=$($f.gate) measured=$($f.measured) required=$($f.required)" }
    foreach($message in $Receipt.errors) { Write-Host "BLOCKED reason=$message measured=unverified required=verified" }
    Write-Host "RAMGiB=$($Receipt.facts.ram_gib) DiskGiB=$($Receipt.facts.disk_gib) ActiveJobs=$($Receipt.facts.active_jobs) DirtyEntries=$($Receipt.dirty_count)"
    Write-Host 'RuntimeMutated=false LiveModelsChanged=false RetainedStateMutated=false'
}
function New-NxRunDirectory {
    $path=Resolve-NxPath ('local-acceptance-models/nxmmr/private-activation/'+[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'))
    $null=New-Item -ItemType Directory -Path $path
    # Snapshot contains credentials. Restrict inheritance to operator, SYSTEM and admins.
    $acl=Get-Acl -LiteralPath $path; $acl.SetAccessRuleProtection($true,$false)
    foreach($sid in @([Security.Principal.WindowsIdentity]::GetCurrent().User.Value,'S-1-5-18','S-1-5-32-544')) {
        $identity=New-Object Security.Principal.SecurityIdentifier($sid)
        $rule=New-Object Security.AccessControl.FileSystemAccessRule($identity,'FullControl','ContainerInherit,ObjectInherit','None','Allow'); $acl.AddAccessRule($rule)
    }
    Set-Acl -LiteralPath $path -AclObject $acl
    return $path
}
function Get-NxRunDirectory([string]$RunDirectory) {
    if (-not $RunDirectory) {
        $pointer=Resolve-NxPath 'local-acceptance-models/nxmmr/private-activation/latest.json'
        $RunDirectory=(Get-Content -LiteralPath $pointer -Raw | ConvertFrom-Json).run_directory
    }
    $full=[IO.Path]::GetFullPath($RunDirectory)
    $parent=Resolve-NxPath 'local-acceptance-models/nxmmr/private-activation'
    if (-not $full.StartsWith($parent+'\',[StringComparison]::OrdinalIgnoreCase)) { throw 'Receipt path is outside the private activation root.' }
    return $full
}
function Assert-NxProtected($Before,$After) {
    foreach($name in @('forensic-records-api','api','forensic-postgres','forensic-nats')) {
        if ($Before.$name.id -cne $After[$name].Id -or $Before.$name.image -cne $After[$name].Image) { throw "Protected service changed: $name" }
    }
}
function Copy-NxAssets($Manifest) {
    $assets=Get-NxAssets $Manifest
    foreach($asset in $assets) {
        if (Test-Path -LiteralPath $asset.destination) { continue }
        $null=New-Item -ItemType Directory -Path (Split-Path -Parent $asset.destination) -Force
        if($asset.kind -eq 'file') { [IO.File]::Copy($asset.source,$asset.destination,$false) }
        else {
            $null=New-Item -ItemType Directory -Path $asset.destination
            foreach($file in Get-ChildItem -LiteralPath $asset.source -Recurse -File) {
                $target=Join-Path $asset.destination $file.FullName.Substring($asset.source.Length+1)
                $null=New-Item -ItemType Directory -Path (Split-Path -Parent $target) -Force
                [IO.File]::Copy($file.FullName,$target,$false)
            }
        }
        $files=if($asset.kind -eq 'file'){@(Get-Item -LiteralPath $asset.destination)}else{@(Get-ChildItem -LiteralPath $asset.destination -Recurse -File)}
        foreach($file in $files) { $file.IsReadOnly=$true }
    }
    return Get-NxAssets $Manifest -DestinationRequired
}
function Invoke-NxReceiptCompose([string]$File,[string[]]$Tail,[int]$Timeout=120,[string]$Log='', [switch]$StreamOutput) {
    return Invoke-NxNative docker (@('compose','--project-name','nexusai','-f',$File)+$Tail) $Timeout $Log -StreamOutput:$StreamOutput
}
function Wait-NxHealth([int]$Seconds=240) {
    $deadline=[DateTime]::UtcNow.AddSeconds($Seconds); $last=''
    do {
        try {$c=Get-NxContainers; Assert-NxHealth $c; return $c} catch {$last=$_.Exception.Message}
        Start-Sleep -Seconds 3
    } while([DateTime]::UtcNow -lt $deadline)
    throw "Health deadline exceeded: $last"
}
function Enter-NxOperatorLock {
    $mutex=New-Object Threading.Mutex($false,'Local\NexusAI_NXMMR_Operator_Activation_V1')
    try { if(-not $mutex.WaitOne(0)){throw 'Another operator activation/rollback is already running.'} }
    catch [Threading.AbandonedMutexException] { $mutex.ReleaseMutex();$mutex.Dispose();throw 'Prior operator process ended unexpectedly. Inspect its receipt and use rollback; do not activate blindly.' }
    catch { $mutex.Dispose();throw }
    return $mutex
}
