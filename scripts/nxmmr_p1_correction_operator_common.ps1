# SPDX-License-Identifier: MIT
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'nxmmr_stt_query_operator_common.ps1')

$script:NxSttManifestPath = Resolve-NxPath 'configuration/nxmmr_p1_correction_activation_v1.json'
$script:NxSttIntegrityPath = Resolve-NxPath 'configuration/nxmmr_p1_correction_operator_integrity_v1.json'
$script:NxSttChanged = @('api', 'forensic-records-api')
$script:NxSttProtected = @('forensic-records-worker', 'forensic-postgres', 'forensic-nats')
$script:NxSttRunRoot = Resolve-NxPath 'local-acceptance-models/nxmmr/private-activation-p1-correction'

function Get-NxP1Manifest {
    $seal = Get-Content -LiteralPath $script:NxSttIntegrityPath -Raw | ConvertFrom-Json
    if ($seal.contract_version -ne 'nexusai.nxmmr.p1-correction-operator-integrity/v1') { throw 'P1 correction source seal contract is invalid.' }
    foreach ($property in $seal.files.PSObject.Properties) {
        if ((Get-NxHash (Resolve-NxPath $property.Name)) -cne [string]$property.Value) { throw "Source seal mismatch: $($property.Name). Reconcile and reseal; never bypass." }
    }
    $manifest = Get-Content -LiteralPath $script:NxSttManifestPath -Raw | ConvertFrom-Json
    if ($manifest.contract_version -ne 'nexusai.nxmmr.p1-correction-activation/v1') { throw 'P1 correction activation manifest is invalid.' }
    if ($manifest.previous_activation_receipt -cne '20260902T025246574Z') { throw 'Previous accepted activation receipt changed.' }
    if (($manifest.services.build_and_recreate_no_deps -join ',') -cne ($script:NxSttChanged -join ',')) { throw 'Candidate service scope differs from the source-owned two-service boundary.' }
    if (($manifest.services.preserve_exact_identity -join ',') -cne ($script:NxSttProtected -join ',')) { throw 'Protected service scope changed.' }
    if ([double]$manifest.resource_gates.minimum_available_ram_gib -ne 6) { throw 'The mandatory 6 GiB gate changed.' }
    Assert-NxP1BuildPolicy $manifest
    if (@($manifest.services.build_and_recreate_no_deps) -contains 'forensic-records-worker') { throw 'Unchanged worker must not be rebuilt for this correction.' }
    return $manifest
}

function Get-NxSttManifest { return Get-NxP1Manifest }
function Get-NxSttAssets($Manifest, [switch]$DestinationRequired) { return @() }

function Assert-NxP1BuildPolicy($Manifest) {
    if ($Manifest.services.build_network -cne 'default' -or $Manifest.services.pull_policy -cne 'never' -or $Manifest.services.build_dependency_acquisition -cne 'dockerfile_required_only_user_approved') { throw 'Approved build-dependency/runtime-no-pull policy changed.' }
    $argsMap=ConvertTo-NxMap $Manifest.services.api_build_args
    if ($argsMap.Count -ne 2 -or $argsMap['LOCALAI_BUILD_GOMAXPROCS'] -cne '4' -or $argsMap['LOCALAI_BUILD_GOFLAGS'] -cne '-p=2') { throw 'Bounded API Go build concurrency changed.' }
    if ($Manifest.prohibitions -notcontains 'model download' -or $Manifest.prohibitions -notcontains 'runtime package acquisition') { throw 'Runtime acquisition prohibitions changed.' }
}

function Set-NxP1BuildPolicy($Compose, $Manifest) {
    Assert-NxP1BuildPolicy $Manifest
    if (($Compose.services.Keys -join ',') -cne ($script:NxSttChanged -join ',')) { throw 'Build policy scope differs from the admitted two services.' }
    foreach ($name in $script:NxSttChanged) {
        $Compose.services[$name].build['network']=[string]$Manifest.services.build_network
    }
    # Build arguments do not alter the captured service environment.
    $Compose.services['api'].build['args']=ConvertTo-NxMap $Manifest.services.api_build_args
}

function Set-NxP1ReceiptOutcome($State, [ValidateSet('VERIFIED','ROLLED_BACK')][string]$Outcome) {
    # ConvertFrom-Json returns a fixed-property object in Windows PowerShell 5.1.
    # Add-Member also supports receipts created before these outcome fields existed.
    $State.state=$Outcome
    if ($Outcome -ceq 'VERIFIED') {
        $State | Add-Member -NotePropertyName activation_verification -NotePropertyValue 'PASS' -Force
        $State | Add-Member -NotePropertyName live_failed_cells_acceptance -NotePropertyValue 'PENDING' -Force
    } else {
        $State | Add-Member -NotePropertyName rollback_verification -NotePropertyValue 'PASS' -Force
    }
}

function Assert-NxP1StartingRuntime($Containers, $Manifest) {
    foreach ($name in @($script:NxSttChanged + $script:NxSttProtected)) {
        $expected = $Manifest.starting_runtime.$name
        if ($Containers[$name].Id -cne [string]$expected.container_id -or $Containers[$name].Image -cne [string]$expected.image_id) {
            throw "Starting runtime differs from accepted receipt 20260902T025246574Z: $name"
        }
    }
}

function Assert-NxP1WorkerRoles($Worker, $Manifest) {
    $environment = @{}
    foreach ($entry in $Worker.Config.Env) { $pair = $entry -split '=', 2; $environment[$pair[0]] = $pair[1] }
    foreach ($property in $Manifest.worker_environment_required.PSObject.Properties) {
        if ($environment[$property.Name] -cne [string]$property.Value) { throw "Protected worker role changed: $($property.Name)" }
    }
}

function Get-NxP1RetainedCounts($Containers) {
    $sql = "SELECT (SELECT count(*) FROM forensic.evidence_items),(SELECT count(*) FROM forensic.evidence_versions),(SELECT count(*) FROM forensic.records_ingest_jobs),(SELECT count(*) FROM forensic.records),(SELECT count(*) FROM forensic.derived_artifacts),(SELECT count(*) FROM forensic.kb_collection_assets);"
    return Invoke-NxNative docker @('exec', $Containers['forensic-postgres'].Id, 'psql', '-U', 'localrecall', '-d', 'localrecall', '-Atc', $sql)
}

function Assert-NxP1RetainedState($Containers, [string]$ExpectedCounts) {
    if ((Get-NxJobs $Containers) -ne 0) { throw 'Active analytical jobs must be zero.' }
    $actual = Get-NxP1RetainedCounts $Containers
    if ($actual -cne $ExpectedCounts) { throw "Retained accounting changed: measured=$actual expected=$ExpectedCounts" }
}

function Assert-NxP1RuntimeParity($Before, $After, [string]$Name) {
    $beforeEnvironment = @{}; $afterEnvironment = @{}
    foreach ($entry in $Before.Config.Env) { $pair = $entry -split '=', 2; $beforeEnvironment[$pair[0]] = $pair[1] }
    foreach ($entry in $After.Config.Env) { $pair = $entry -split '=', 2; $afterEnvironment[$pair[0]] = $pair[1] }
    if ($beforeEnvironment.Count -ne $afterEnvironment.Count) { throw "Environment count changed: $Name" }
    foreach ($key in $beforeEnvironment.Keys) { if ($afterEnvironment[$key] -cne $beforeEnvironment[$key]) { throw "Environment changed for ${Name}: $key" } }
    if (@($Before.Mounts).Count -ne @($After.Mounts).Count) { throw "Mount count changed: $Name" }
    foreach ($mount in $Before.Mounts) {
        $matches = @($After.Mounts | Where-Object { $_.Type -ceq $mount.Type -and $_.Source -ceq $mount.Source -and $_.Destination -ceq $mount.Destination -and $_.RW -eq $mount.RW })
        if ($matches.Count -ne 1) { throw "Mount changed for ${Name}: $($mount.Destination)" }
    }
    if ($After.HostConfig.NetworkMode -cne $Before.HostConfig.NetworkMode -or $After.HostConfig.RestartPolicy.Name -cne $Before.HostConfig.RestartPolicy.Name) { throw "Network/restart policy changed: $Name" }
}

function Get-NxP1SnapshotContainer($Snapshot, [string]$Name) {
    $entry=$Snapshot.PSObject.Properties[$Name].Value
    if($entry.PSObject.Properties['Config']){return $entry}
    if($entry.PSObject.Properties['value']){
        $values=@($entry.value)
        if($values.Count -eq 1 -and $values[0].PSObject.Properties['Config']){return $values[0]}
    }
    throw "Private runtime snapshot shape is invalid: $Name"
}

function Get-NxP1HighMemoryProcesses {
    return @(Get-Process -ErrorAction SilentlyContinue | Sort-Object WorkingSet64 -Descending | Select-Object -First 8 | ForEach-Object {
        [pscustomobject]@{ name = $_.ProcessName; id = $_.Id; working_set_mib = [math]::Round($_.WorkingSet64 / 1MB, 1) }
    })
}

function Invoke-NxP1Preflight {
    $facts = [ordered]@{ram_gib=$null;disk_gib=$null;active_jobs=$null;docker=$false;health=$false;integrity=$false;assets=$false;models=$false;rollback=$false;compose=$false}
    $errors = New-Object 'System.Collections.Generic.List[string]'; $manifest=$null; $containers=$null; $models=@(); $head=''; $dirty=@(); $counts=''
    try { $head=Invoke-NxNative git @('rev-parse','HEAD'); $dirty=@((Invoke-NxNative git @('status','--short')) -split '\r?\n' | Where-Object {$_}) } catch { $errors.Add($_.Exception.Message) }
    try { $facts.ram_gib=Get-NxRam; $drive=([IO.Path]::GetPathRoot($script:NxRoot)).Substring(0,1); $facts.disk_gib=[double](Get-PSDrive -Name $drive).Free/1GB } catch { $errors.Add($_.Exception.Message) }
    try { $manifest=Get-NxP1Manifest; $facts.integrity=$true; $facts.assets=$true } catch { $errors.Add($_.Exception.Message) }
    try {
        $null=Invoke-NxNative docker @('info','--format','{{.ServerVersion}}'); $facts.docker=$true
        $containers=Get-NxSttContainers; Assert-NxHealth $containers; $facts.health=$true; $facts.active_jobs=Get-NxJobs $containers
        if ($manifest) {
            Assert-NxP1StartingRuntime $containers $manifest
            Assert-NxP1WorkerRoles $containers['forensic-records-worker'] $manifest
            $counts=Get-NxP1RetainedCounts $containers
            if ($counts -cne [string]$manifest.retained_count_baseline) { throw "Retained count baseline drifted: $counts" }
            $models=Get-NxSttModelInventory $manifest; $facts.models=$true
            $images=[ordered]@{}; foreach ($name in $script:NxSttChanged) { $images[$name]=$containers[$name].Image }
            $rollback=New-NxSttCompose $containers $images ([ordered]@{}) $manifest
            $temporary=Join-Path ([IO.Path]::GetTempPath()) ('nxmmr-p1-'+[guid]::NewGuid().ToString('n')+'.json')
            try { Write-NxJson $temporary $rollback; $null=Invoke-NxReceiptCompose $temporary @('config','--quiet'); $facts.rollback=$true; $facts.compose=$true } finally { if (Test-Path -LiteralPath $temporary) { [IO.File]::Delete($temporary) } }
        }
    } catch { $errors.Add($_.Exception.Message) }
    $failures=if($manifest){Test-NxSttGateValues $facts $manifest}else{@([pscustomobject]@{gate='manifest';measured=$false;required=$true})}
    $pass=$failures.Count -eq 0 -and $errors.Count -eq 0
    return [pscustomobject]@{status=$(if($pass){'PASS'}else{'BLOCKED'});utc=[DateTime]::UtcNow.ToString('o');git_head=$head;dirty_count=$dirty.Count;dirty_summary=$dirty;facts=$facts;failures=$failures;errors=$errors.ToArray();containers=$(if($containers){Get-NxSummary $containers}else{$null});models=$models;retained_counts=$counts;high_memory_processes=(Get-NxP1HighMemoryProcesses);runtime_mutated=$false;database_migration=$false;volumes_changed=$false;retained_state_mutated=$false}
}

function Show-NxP1Preflight($Receipt) {
    Write-Host "P1_CORRECTION_ACTIVATION_PREFLIGHT=$($Receipt.status)"
    foreach ($failure in $Receipt.failures) { Write-Host "BLOCKED reason=$($failure.gate) measured=$($failure.measured) required=$($failure.required)" }
    foreach ($message in $Receipt.errors) { Write-Host "BLOCKED reason=$message measured=unverified required=verified" }
    Write-Host "RAMGiB=$($Receipt.facts.ram_gib) DiskGiB=$($Receipt.facts.disk_gib) ActiveJobs=$($Receipt.facts.active_jobs) DirtyEntries=$($Receipt.dirty_count)"
    foreach ($process in $Receipt.high_memory_processes) { Write-Host "HIGH_MEMORY_PROCESS name=$($process.name) pid=$($process.id) working_set_mib=$($process.working_set_mib)" }
    Write-Host 'RuntimeMutated=false DatabaseMigration=false VolumesChanged=false RetainedStateMutated=false'
}

function New-NxP1RunDirectory {
    if (-not (Test-Path -LiteralPath $script:NxSttRunRoot)) { $null=New-Item -ItemType Directory -Force -Path $script:NxSttRunRoot }
    $path=Join-Path $script:NxSttRunRoot ([DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'))
    $null=New-Item -ItemType Directory -Path $path
    $acl=Get-Acl -LiteralPath $path; $acl.SetAccessRuleProtection($true,$false)
    foreach ($sid in @([Security.Principal.WindowsIdentity]::GetCurrent().User.Value,'S-1-5-18','S-1-5-32-544')) { $identity=New-Object Security.Principal.SecurityIdentifier($sid); $rule=New-Object Security.AccessControl.FileSystemAccessRule($identity,'FullControl','ContainerInherit,ObjectInherit','None','Allow'); $acl.AddAccessRule($rule) }
    Set-Acl -LiteralPath $path -AclObject $acl
    return $path
}

function Get-NxP1RunDirectory([string]$RunDirectory) {
    if (-not $RunDirectory) { $pointer=Join-Path $script:NxSttRunRoot 'latest.json'; $RunDirectory=(Get-Content -LiteralPath $pointer -Raw|ConvertFrom-Json).run_directory }
    $full=[IO.Path]::GetFullPath($RunDirectory)
    if (-not $full.StartsWith($script:NxSttRunRoot+'\',[StringComparison]::OrdinalIgnoreCase)) { throw 'Receipt path is outside the P1 correction activation root.' }
    return $full
}

function Enter-NxP1OperatorLock {
    $mutex=New-Object Threading.Mutex($false,'Local\NexusAI_NXMMR_P1_Correction_Activation_V1')
    try { if(-not $mutex.WaitOne(0)){throw 'Another P1 correction activation or rollback is running.'} } catch [Threading.AbandonedMutexException] { $mutex.ReleaseMutex();$mutex.Dispose();throw 'Prior operator process ended unexpectedly; inspect the receipt and use rollback.' } catch { $mutex.Dispose();throw }
    return $mutex
}

function Find-NxP1ResumableBuildSet($Before, [string]$CurrentRun) {
    $manifestHash=Get-NxHash $script:NxSttManifestPath; $sealHash=Get-NxHash $script:NxSttIntegrityPath
    if(-not (Test-Path -LiteralPath $script:NxSttRunRoot)){return [ordered]@{}}
    foreach($directory in @(Get-ChildItem -LiteralPath $script:NxSttRunRoot -Directory | Sort-Object Name -Descending)){
        if($directory.FullName -eq $CurrentRun){continue}; $path=Join-Path $directory.FullName 'build-set.json'
        if(-not (Test-Path -LiteralPath $path -PathType Leaf)){continue}
        try {
            $candidate=Get-Content -LiteralPath $path -Raw|ConvertFrom-Json
            if($candidate.contract_version -ne 'nexusai.nxmmr.p1-correction-build-set/v1' -or $candidate.manifest_sha256 -cne $manifestHash -or $candidate.source_seal_sha256 -cne $sealHash -or $candidate.recreation_started){continue}
            $original=ConvertTo-NxMap $candidate.original_images; $same=$true
            foreach($name in $script:NxSttChanged){if($original[$name] -cne $Before[$name].Image){$same=$false;break}}
            if(-not $same){continue}; $valid=[ordered]@{}
            foreach($property in $candidate.images.PSObject.Properties){
                if($property.Name -notin $script:NxSttChanged){continue}
                $actual=Invoke-NxNative docker @('image','inspect',[string]$property.Value.tag,'--format','{{.Id}}')
                if($actual -cne [string]$property.Value.id){throw "Candidate image tag drifted: $($property.Name)"}
                $valid[$property.Name]=[ordered]@{tag=[string]$property.Value.tag;id=[string]$property.Value.id}
            }
            if($valid.Count){Write-Host "RESUMABLE_BUILD_SET=PASS receipt=$path images=$($valid.Keys -join ',')";return $valid}
        } catch {Write-Host "RESUMABLE_BUILD_SET=REJECTED receipt=$path reason=$($_.Exception.Message)"}
    }
    return [ordered]@{}
}
