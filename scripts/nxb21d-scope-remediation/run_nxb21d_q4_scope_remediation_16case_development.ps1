# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')
. (Join-Path $PSScriptRoot '..\nxb21d-development\byte_safe_transport.ps1')
. (Join-Path $PSScriptRoot '..\nxb21d-remediation\exact_identifier_authority.ps1')
. (Join-Path $PSScriptRoot '..\nxb21d-phrase-remediation\phrase_literal_authority.ps1')
. (Join-Path $PSScriptRoot 'explicit_scope_authority.ps1')

function Get-CountMap($Rows, [string]$Property) {
    $map = [ordered]@{}
    foreach ($group in @($Rows | Group-Object -Property $Property | Sort-Object Name)) {
        $map[[string]$group.Name] = [int]$group.Count
    }
    return $map
}

function Get-LatencySummary([long[]]$Values) {
    if ($Values.Count -eq 0) { return $null }
    $sorted = @($Values | Sort-Object)
    $middle = [math]::Floor($sorted.Count / 2)
    $median = if ($sorted.Count % 2) { [double]$sorted[$middle] } else { ([double]$sorted[$middle - 1] + [double]$sorted[$middle]) / 2 }
    $p95Index = [math]::Max(0, [math]::Ceiling(0.95 * $sorted.Count) - 1)
    return [ordered]@{
        minimum_ms = [long]$sorted[0]
        median_ms = [math]::Round($median, 3)
        p95_ms = [long]$sorted[$p95Index]
        maximum_ms = [long]$sorted[-1]
        over_10s = @($Values | Where-Object { $_ -gt 10000 }).Count
        over_20s = @($Values | Where-Object { $_ -gt 20000 }).Count
        over_30s = @($Values | Where-Object { $_ -gt 30000 }).Count
    }
}

function ConvertTo-MemoryMiB([string]$Usage) {
    $value = ($Usage -split '/')[0].Trim()
    if ($value -notmatch '^([0-9.]+)([KMG]iB)$') { return $null }
    $number = [double]$Matches[1]
    switch ($Matches[2]) {
        'KiB' { return [math]::Round($number / 1024, 3) }
        'MiB' { return [math]::Round($number, 3) }
        'GiB' { return [math]::Round($number * 1024, 3) }
    }
}

function Get-ResourceSnapshot([string]$Container) {
    $stats = Invoke-NxDNative docker @('stats', '--no-stream', '--format', '{{json .}}', $Container) | ConvertFrom-Json
    return [ordered]@{
        captured_at = [DateTime]::UtcNow.ToString('o')
        available_host_ram_gib = Get-NxDRamGiB
        localai_memory_usage = [string]$stats.MemUsage
        localai_memory_mib = ConvertTo-MemoryMiB ([string]$stats.MemUsage)
        localai_cpu_percent = [string]$stats.CPUPerc
    }
}

function Get-LoadedModelIDs([string]$BaseURL) {
    $system = Invoke-RestMethod -Method Get -Uri ($BaseURL.TrimEnd('/') + '/system') -TimeoutSec 15
    return @($system.loaded_models | ForEach-Object { [string]$_.id })
}

function Assert-DevelopmentRAM([double]$Available, [double]$Required, [string]$Stage) {
    Write-Host "RAM_GATE stage=$Stage available_gib=$Available required_gib=$Required"
    if ($Available -lt $Required) {
        foreach ($process in Get-NxDHighMemoryProcesses) {
            Write-Host "HIGH_MEMORY_PROCESS name=$($process.name) pid=$($process.pid) working_set_mib=$($process.working_set_mib)"
        }
        throw "RAM_TOO_LOW:$Available required=$Required stage=$Stage"
    }
}

function Wait-DevelopmentRAM([double]$Required, [string]$Stage, [int]$WaitSeconds = 180) {
    $available = Get-NxDRamGiB
    Write-Host "RAM_GATE stage=$Stage available_gib=$available required_gib=$Required"
    if ($available -ge $Required) { return $available }

    $null = Invoke-NxDSafeLinuxCleanCache
    $deadline = [DateTime]::UtcNow.AddSeconds($WaitSeconds)
    do {
        $available = Get-NxDRamGiB
        Write-Host "RAM_RECLAIM_WAIT stage=$Stage available_gib=$available required_gib=$Required"
        if ($available -ge $Required) { return $available }
        if ([DateTime]::UtcNow -ge $deadline) { break }
        Start-Sleep -Seconds 5
    } while ([DateTime]::UtcNow -lt $deadline)

    return Assert-DevelopmentRAM $available $Required $Stage
}

function Get-ContainerBaseline([string]$Container) {
    $row = (Invoke-NxDNative docker @('inspect', $Container) | ConvertFrom-Json)[0]
    if (-not (Test-NxDContainerHealthy $row)) { throw 'LOCALAI_CONTAINER_UNHEALTHY' }
    return [ordered]@{id = [string]$row.Id; image = [string]$row.Image; restart_count = [int]$row.RestartCount; status = [string]$row.State.Status; health = if ($row.State.PSObject.Properties['Health']) { [string]$row.State.Health.Status } else { 'none' }}
}

function Wait-NxDContainerHealthy([string]$Container, [int]$TimeoutSeconds = 180) {
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    $attempt = 0
    do {
        $attempt++
        $row = (Invoke-NxDNative docker @('inspect', $Container) | ConvertFrom-Json)[0]
        $status = [string]$row.State.Status
        $health = if ($row.State.PSObject.Properties['Health']) { [string]$row.State.Health.Status } else { 'none' }
        if (Test-NxDContainerHealthy $row) {
            Write-Host "LOCALAI_HEALTH_WAIT=PASS attempt=$attempt status=$status health=$health"
            return
        }
        if ($status -in @('dead', 'exited', 'removing')) {
            throw "LOCALAI_CONTAINER_NOT_RUNNING status=$status health=$health"
        }
        Write-Host "LOCALAI_HEALTH_WAIT=WAIT attempt=$attempt status=$status health=$health timeout_seconds=$TimeoutSeconds"
        Start-Sleep -Seconds 5
    } while ([DateTime]::UtcNow -lt $deadline)
    throw "LOCALAI_CONTAINER_HEALTH_TIMEOUT timeout_seconds=$TimeoutSeconds"
}

function Get-ContainerFileHash([string]$Container, [string]$Path) {
    return ((Invoke-NxDNative docker @('exec', $Container, 'sha256sum', $Path)) -split '\s+')[0].ToLowerInvariant()
}

function Get-NxDResumeState([string]$RunRoot, $Corpus, $Config, [string]$CurrentRun) {
    $byCase = [ordered]@{}
    $sourceReceipts = New-Object 'System.Collections.Generic.List[object]'
    $summaryFiles = @(Get-ChildItem -LiteralPath $RunRoot -Directory | ForEach-Object {
        $candidate = Join-Path $_.FullName 'scope-remediation-16case-development-summary-v1.json'
        if (Test-Path -LiteralPath $candidate -PathType Leaf) { Get-Item -LiteralPath $candidate }
    })
    foreach ($summaryFile in $summaryFiles) {
        if ([IO.Path]::GetFullPath($summaryFile.DirectoryName) -ceq [IO.Path]::GetFullPath($CurrentRun)) { continue }
        $sidecarPath = $summaryFile.FullName + '.sha256'
        if (-not (Test-Path -LiteralPath $sidecarPath)) { throw "RESUME_INTEGRITY_MISSING_SIDECAR:$($summaryFile.FullName)" }
        $summaryHash = Get-NxDHash $summaryFile.FullName
        $sidecarHash = ((Read-NxDStrictUtf8Text $sidecarPath) -split '\s+')[0].ToLowerInvariant()
        if ($sidecarHash -cne $summaryHash) { throw "RESUME_INTEGRITY_SIDECAR_MISMATCH:$($summaryFile.FullName)" }
        $summary = Read-NxDStrictUtf8Text $summaryFile.FullName | ConvertFrom-Json
        if ($null -eq $summary.candidate_tuple) { continue }
        if ([string]$summary.candidate_tuple.identity_sha256 -cne [string]$Config.candidate.identity_sha256 -or [string]$summary.candidate_tuple.corpus_sha256 -cne [string]$Config.corpus.sha256) { continue }
        $acceptedFromSummary = 0
        foreach ($prior in @($summary.results)) {
            $case = @($Corpus.cases | Where-Object { [string]$_.id -ceq [string]$prior.case_id })
            if ($case.Count -ne 1) { throw "RESUME_INTEGRITY_UNKNOWN_CASE:$($prior.case_id)" }
            if ([string]$prior.question -cne [string]$case[0].question) { throw "RESUME_INTEGRITY_QUESTION_MISMATCH:$($prior.case_id)" }
            $expectedJson = $case[0].expected | ConvertTo-Json -Depth 20 -Compress
            $priorExpectedJson = $prior.expected_oracle | ConvertTo-Json -Depth 20 -Compress
            if ($priorExpectedJson -cne $expectedJson) { throw "RESUME_INTEGRITY_ORACLE_MISMATCH:$($prior.case_id)" }
            foreach ($evidence in @(
                @{path = [string]$prior.evidence_paths.request; hash = [string]$prior.raw_request_sha256; name = 'request'},
                @{path = [string]$prior.evidence_paths.raw_response; hash = [string]$prior.raw_response_sha256; name = 'response'}
            )) {
                $evidencePath = [IO.Path]::GetFullPath($evidence.path)
                $evidenceRootPrefix = [IO.Path]::GetFullPath($RunRoot).TrimEnd('\') + '\'
                if (-not $evidencePath.StartsWith($evidenceRootPrefix, [StringComparison]::OrdinalIgnoreCase)) { throw "RESUME_INTEGRITY_PATH_ESCAPE:$($prior.case_id):$($evidence.name)" }
                if (-not (Test-Path -LiteralPath $evidencePath)) { throw "RESUME_INTEGRITY_EVIDENCE_MISSING:$($prior.case_id):$($evidence.name)" }
                if ((Get-NxDHash $evidencePath) -cne $evidence.hash) { throw "RESUME_INTEGRITY_EVIDENCE_HASH:$($prior.case_id):$($evidence.name)" }
            }
            if ($byCase.Contains([string]$prior.case_id)) {
                $existing = $byCase[[string]$prior.case_id]
                if ([string]$existing.raw_request_sha256 -cne [string]$prior.raw_request_sha256 -or [string]$existing.raw_response_sha256 -cne [string]$prior.raw_response_sha256) { throw "RESUME_DUPLICATE_CASE_EVIDENCE:$($prior.case_id)" }
                continue
            }
            $byCase[[string]$prior.case_id] = $prior
            $acceptedFromSummary++
        }
        if ($acceptedFromSummary -gt 0) {
            $sourceReceipts.Add([pscustomobject][ordered]@{path = $summaryFile.FullName; sha256 = $summaryHash; accepted_cases = $acceptedFromSummary})
        }
    }
    $ordered = New-Object 'System.Collections.Generic.List[object]'
    $gapFound = $false
    foreach ($case in @($Corpus.cases)) {
        if ($byCase.Contains([string]$case.id)) {
            if ($gapFound) { throw "RESUME_NON_PREFIX_CASE_EVIDENCE:$($case.id)" }
            $ordered.Add($byCase[[string]$case.id])
        } else {
            $gapFound = $true
        }
    }
    if ($ordered.Count -ne $byCase.Count) { throw 'RESUME_CASE_ACCOUNTING_MISMATCH' }
    return [pscustomobject][ordered]@{results = @($ordered | ForEach-Object { $_ }); source_receipts = @($sourceReceipts | ForEach-Object { $_ })}
}

function Assert-Equal([string]$Name, [string]$Actual, [string]$Expected) {
    if ($Actual -cne $Expected) { throw "INTEGRITY_MISMATCH:$Name expected=$Expected actual=$Actual" }
}

function Get-OptionalProperty($Object, [string]$Name) {
    if ($null -eq $Object) { return $null }
    $property = $Object.PSObject.Properties[$Name]
    if ($null -eq $property) { return $null }
    return $property.Value
}

function Get-NxDPlannerOracleErrors($Proposal, $Expected) {
    $errors = New-Object 'System.Collections.Generic.List[string]'
    if ($null -eq $Proposal) {
        $errors.Add('planner_proposal_missing')
        return @($errors | ForEach-Object { $_ })
    }
    foreach ($shapeError in @(Test-NxDPlannerObject $Proposal)) { $errors.Add("planner_schema:$shapeError") }
    $actualCapability = [string](Get-OptionalProperty $Proposal 'query_capability_id')
    $actualSemantic = [string](Get-OptionalProperty $Proposal 'semantic')
    $actualLiteral = [string](Get-OptionalProperty $Proposal 'literal_text')
    $actualScope = [string](Get-OptionalProperty $Proposal 'scope')
    $actualClarification = Get-OptionalProperty $Proposal 'clarification_required'
    $actualEntities = @(Get-OptionalProperty $Proposal 'entities')
    if ($actualCapability -cne [string]$Expected.query_capability_id) { $errors.Add('capability') }
    if ($actualSemantic -cne [string]$Expected.semantic) { $errors.Add('semantic') }
    if ($actualLiteral -cne [string]$Expected.literal_text) { $errors.Add('literal_exactness') }
    if ($actualScope -cne [string]$Expected.scope) { $errors.Add('scope') }
    if ($null -eq $actualClarification -or [bool]$actualClarification -ne [bool]$Expected.clarification_required) { $errors.Add('clarification') }
    $expectedEntities = @($Expected.entities)
    if ($actualEntities.Count -ne $expectedEntities.Count) { $errors.Add('entity_count') }
    for ($index = 0; $index -lt [math]::Min($actualEntities.Count, $expectedEntities.Count); $index++) {
        if ([string]$actualEntities[$index] -cne [string]$expectedEntities[$index]) { $errors.Add("entity_mismatch:$index") }
    }
    return @($errors | ForEach-Object { $_ })
}

function New-NxDFinalPlannerProposal($Proposal, $ExactAuthority, $PhraseAuthority, $ScopeAuthority) {
    $final = [ordered]@{
        query_capability_id = [string](Get-OptionalProperty $Proposal 'query_capability_id')
        semantic = [string](Get-OptionalProperty $Proposal 'semantic')
        literal_text = [string](Get-OptionalProperty $Proposal 'literal_text')
        entities = @(Get-OptionalProperty $Proposal 'entities')
        scope = [string](Get-OptionalProperty $Proposal 'scope')
        clarification_required = Get-OptionalProperty $Proposal 'clarification_required'
    }
    if ($null -ne $ExactAuthority -and [bool]$ExactAuthority.applicable -and [string]$ExactAuthority.resolution_status -ceq 'RESOLVED_DETERMINISTIC_AUTHORITY') {
        $final.literal_text = [string]$ExactAuthority.final_literal_text
        $final.entities = @($ExactAuthority.final_entities)
    }
    if ($null -ne $PhraseAuthority -and [bool]$PhraseAuthority.applicable -and [string]$PhraseAuthority.resolution_status -ceq 'RESOLVED_DETERMINISTIC_AUTHORITY') {
        $final.literal_text = [string]$PhraseAuthority.final_literal
        $final.clarification_required = [bool]$PhraseAuthority.final_clarification
    }
    if ($null -ne $ScopeAuthority -and [bool]$ScopeAuthority.applicable -and [string]$ScopeAuthority.resolution_status -ceq 'RESOLVED_DETERMINISTIC_AUTHORITY') {
        $final.scope = [string]$ScopeAuthority.final_scope
    }
    return [pscustomobject]$final
}

function Get-NxDResultClassification([string[]]$Errors, [bool]$Parsed, [string]$FatalClassification) {
    if ($FatalClassification) { return $FatalClassification }
    if (-not $Parsed) { return 'MALFORMED_PLANNER_JSON' }
    if (@($Errors | Where-Object { $_ -like 'planner_schema:*' }).Count) { return 'PLANNER_SCHEMA_MISMATCH' }
    if ($Errors.Count) { return 'SEMANTIC_ORACLE_MISMATCH' }
    return 'NONE'
}

function Get-NxDCandidateIdentity($Candidate, [string]$CorpusHash) {
    $lines = @(
        "candidate_id=$($Candidate.candidate_id)",
        "artifact_sha256=$($Candidate.artifact_sha256)",
        "profile_sha256=$($Candidate.profile_sha256)",
        "q8_profile_sha256=$($Candidate.q8_profile_sha256)",
        "prompt_sha256=$($Candidate.prompt_sha256)",
        "schema_sha256=$($Candidate.schema_sha256)",
        "implementation_sha256=$($Candidate.implementation_sha256)",
        "bridge_sha256=$($Candidate.bridge_sha256)",
        "authority_sha256=$($Candidate.authority_sha256)",
        "phrase_authority_sha256=$($Candidate.phrase_authority_sha256)",
        "shared_scope_sha256=$($Candidate.shared_scope_sha256)",
        "api_scope_sha256=$($Candidate.api_scope_sha256)",
        "agent_scope_sha256=$($Candidate.agent_scope_sha256)",
        "scope_authority_sha256=$($Candidate.scope_authority_sha256)",
        "transport_sha256=$($Candidate.transport_sha256)",
        "corpus_sha256=$CorpusHash",
        "request_temperature=$($Candidate.request_temperature)",
        "effective_context=$($Candidate.effective_context)"
    )
    $bytes = [Text.UTF8Encoding]::new($false).GetBytes(($lines -join "`n") + "`n")
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
}

$exitCode = 21
$terminalError = $null
$run = $null
$summaryPath = $null
$transcriptStarted = $false
$results = New-Object 'System.Collections.Generic.List[object]'
$before = $null
$after = $null
$integrity = [ordered]@{}
$cfg = $null
$resumeSources = @()

try {
    $cfgPath = Join-Path $PSScriptRoot 'nxb21d_q4_scope_remediation_16case_config.json'
    if (-not (Test-Path -LiteralPath $cfgPath)) { throw 'PREFLIGHT_MISSING:development config' }
    $cfg = Read-NxDStrictUtf8Text $cfgPath | ConvertFrom-Json
    Assert-NxDBundleIntegrity
    Assert-Equal 'candidate_identity' (Get-NxDCandidateIdentity $cfg.candidate ([string]$cfg.corpus.sha256)) ([string]$cfg.candidate.identity_sha256)
    foreach ($property in $cfg.framework_files.PSObject.Properties) {
        $path = Resolve-NxDPath $property.Name
        if (-not (Test-Path -LiteralPath $path)) { throw "PREFLIGHT_MISSING:$($property.Name)" }
        Assert-Equal $property.Name (Get-NxDHash $path) ([string]$property.Value)
    }

    $runRoot = Resolve-NxDPath $cfg.output_root
    $run = Join-Path $runRoot ('run-' + [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'))
    New-Item -ItemType Directory -Force -Path $run | Out-Null
    $summaryPath = Join-Path $run 'scope-remediation-16case-development-summary-v1.json'
    [IO.File]::WriteAllText((Join-Path $runRoot 'latest-run.txt'), $run + "`n", [Text.UTF8Encoding]::new($false))
    Start-Transcript -LiteralPath (Join-Path $run 'operator.log') -Force | Out-Null
    $transcriptStarted = $true
    Write-Host "NXB21D_Q4_SCOPE_REMEDIATION_16CASE=START run=$run"

    $corpusPath = Resolve-NxDPath $cfg.corpus.path
    $corpusBytes = [IO.File]::ReadAllBytes($corpusPath)
    if (Test-NxDUtf8BOM $corpusBytes) { throw 'CORPUS_UTF8_BOM' }
    Assert-Equal 'corpus' (Get-NxDHash $corpusPath) ([string]$cfg.corpus.sha256)
    $validationOutput = & python (Resolve-NxDPath $cfg.corpus.validator) --repo $root --corpus $corpusPath
    if ($LASTEXITCODE -ne 0) { throw "CORPUS_VALIDATION_FAILED:$($validationOutput -join ' ')" }
    $corpusValidation = ($validationOutput -join [Environment]::NewLine) | ConvertFrom-Json
    $corpus = ConvertFrom-NxDStrictUtf8Bytes $corpusBytes | ConvertFrom-Json
    if ([int]$corpusValidation.case_count -ne 16) { throw 'CORPUS_VALIDATION_FAILED:count' }
    Write-Host "CORPUS_FROZEN=PASS sha256=$($cfg.corpus.sha256) cases=16"

    $resumeState = Get-NxDResumeState $runRoot $corpus $cfg $run
    foreach ($priorResult in @($resumeState.results)) { $results.Add($priorResult) }
    $resumeSources = @($resumeState.source_receipts)
    if ($results.Count -eq 16) { throw 'CORPUS_ALREADY_COMPLETED' }
    if ($results.Count -gt 0) {
        Write-Host "DEVELOPMENT_RESUME=PASS preserved_cases=$($results.Count) next_case=$([string]$corpus.cases[$results.Count].id) source_receipts=$($resumeSources.Count)"
    }

    foreach ($namedPath in @(
        @{name = 'q4_artifact'; path = $cfg.candidate.artifact_path; hash = $cfg.candidate.artifact_sha256},
        @{name = 'q4_profile_host'; path = $cfg.candidate.profile_path; hash = $cfg.candidate.profile_sha256},
        @{name = 'planner_prompt'; path = $cfg.candidate.prompt_path; hash = $cfg.candidate.prompt_sha256},
        @{name = 'planner_schema'; path = $cfg.candidate.schema_path; hash = $cfg.candidate.schema_sha256},
        @{name = 'adjudicated_12case_receipt'; path = $cfg.prior_adjudicated_receipt.path; hash = $cfg.prior_adjudicated_receipt.sha256},
        @{name = 'pre_remediation_32case_receipt'; path = $cfg.pre_remediation_32case_receipt.path; hash = $cfg.pre_remediation_32case_receipt.sha256},
        @{name = 'atomic_remediation_16case_receipt'; path = $cfg.atomic_remediation_16case_receipt.path; hash = $cfg.atomic_remediation_16case_receipt.sha256},
        @{name = 'phrase_remediation_20case_receipt'; path = $cfg.phrase_remediation_20case_receipt.path; hash = $cfg.phrase_remediation_20case_receipt.sha256}
    )) {
        $path = Resolve-NxDPath ([string]$namedPath.path)
        if (-not (Test-Path -LiteralPath $path)) { throw "PREFLIGHT_MISSING:$($namedPath.name)" }
        Assert-Equal ([string]$namedPath.name) (Get-NxDHash $path) ([string]$namedPath.hash)
    }
    $artifactBytes = (Get-Item -LiteralPath (Resolve-NxDPath $cfg.candidate.artifact_path)).Length
    if ($artifactBytes -ne [int64]$cfg.candidate.artifact_bytes) { throw "INTEGRITY_MISMATCH:q4_artifact_bytes actual=$artifactBytes" }

    Wait-NxDContainerHealthy $cfg.runtime.container
    $before = Get-ContainerBaseline $cfg.runtime.container
    Assert-Equal 'localai_container_id' $before.id ([string]$cfg.runtime.container_id)
    Assert-Equal 'localai_image' $before.image ([string]$cfg.runtime.image)
    if ($before.restart_count -ne [int]$cfg.runtime.restart_count) { throw "INTEGRITY_MISMATCH:localai_restart_count actual=$($before.restart_count)" }
    Assert-Equal 'q8_profile' (Get-ContainerFileHash $cfg.runtime.container $cfg.runtime.q8_profile_container_path) ([string]$cfg.candidate.q8_profile_sha256)
    Assert-Equal 'q4_profile_container' (Get-ContainerFileHash $cfg.runtime.container $cfg.runtime.q4_profile_container_path) ([string]$cfg.candidate.profile_sha256)
    Assert-Equal 'q4_artifact_container' (Get-ContainerFileHash $cfg.runtime.container $cfg.runtime.q4_artifact_container_path) ([string]$cfg.candidate.artifact_sha256)

    $models = @((Invoke-RestMethod -Method Get -Uri ($cfg.runtime.localai_url.TrimEnd('/') + '/v1/models') -TimeoutSec 15).data | ForEach-Object { [string]$_.id })
    if ($models -cnotcontains [string]$cfg.candidate.model_name) { throw 'Q4_REGISTRATION_MISSING' }
    $loadedBefore = @(Get-LoadedModelIDs $cfg.runtime.localai_url)
    $q4LoadedBefore = $loadedBefore -contains [string]$cfg.candidate.model_name
    $requiredInitial = if ($q4LoadedBefore) { [double]$cfg.resource.minimum_loaded_gib } else { [double]$cfg.resource.minimum_before_load_gib }
    $null = Wait-DevelopmentRAM $requiredInitial 'before_first_inference' 180
    Write-Host "Q4_REGISTRATION=PASS loaded=$q4LoadedBefore"
    Write-Host "SCOPE_CANDIDATE_REFREEZE=PASS candidate_id=$($cfg.candidate.candidate_id) identity_sha256=$($cfg.candidate.identity_sha256) shared_scope_sha256=$($cfg.candidate.shared_scope_sha256) api_scope_sha256=$($cfg.candidate.api_scope_sha256) agent_scope_sha256=$($cfg.candidate.agent_scope_sha256) scope_authority_sha256=$($cfg.candidate.scope_authority_sha256) exact_authority_sha256=$($cfg.candidate.authority_sha256) phrase_authority_sha256=$($cfg.candidate.phrase_authority_sha256) transport_sha256=$($cfg.candidate.transport_sha256)"

    $plannerSystem = Read-NxDStrictUtf8Text (Resolve-NxDPath $cfg.candidate.prompt_path)
    $plannerSchema = Read-NxDStrictUtf8Text (Resolve-NxDPath $cfg.candidate.schema_path) | ConvertFrom-Json
    $httpClient = [Net.Http.HttpClient]::new()
    $httpClient.Timeout = [TimeSpan]::FromSeconds([double]$cfg.runtime.request_timeout_seconds)
    try {
        $ordinal = 0
        $preservedCaseIDs = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
        foreach ($priorResult in @($results | ForEach-Object { $_ })) { $null = $preservedCaseIDs.Add([string]$priorResult.case_id) }
        foreach ($case in @($corpus.cases)) {
            $ordinal++
            if ($preservedCaseIDs.Contains([string]$case.id)) {
                Write-Host "NXB21D_CASE $ordinal/16 id=$($case.id) preserved=True"
                continue
            }
            $caseDirectory = Join-Path $run ([string]$case.id)
            $loadedNow = @(Get-LoadedModelIDs $cfg.runtime.localai_url) -contains [string]$cfg.candidate.model_name
            $required = if ($loadedNow) { [double]$cfg.resource.minimum_loaded_gib } else { [double]$cfg.resource.minimum_before_load_gib }
            $null = Wait-DevelopmentRAM $required ([string]$case.id) 180
            $beforeResource = Get-ResourceSnapshot $cfg.runtime.container
            Write-Host "NXB21D_CASE $ordinal/16 id=$($case.id) language=$($case.language) loaded=$loadedNow"

            $body = [ordered]@{
                model = [string]$cfg.candidate.model_name
                messages = @(
                    [ordered]@{role = 'system'; content = $plannerSystem},
                    [ordered]@{role = 'user'; content = [string]$case.question}
                )
                temperature = [int]$cfg.candidate.request_temperature
                max_tokens = [int]$cfg.candidate.max_tokens
                response_format = [ordered]@{type = 'json_schema'; json_schema = [ordered]@{name = 'nxb21d_planner_proposal'; strict = $true; schema = $plannerSchema}}
            }
            $transport = Invoke-NxDByteSafePlannerPost $httpClient ($cfg.runtime.localai_url.TrimEnd('/') + '/v1/chat/completions') $body $caseDirectory
            $afterResource = Get-ResourceSnapshot $cfg.runtime.container
            $expected = $case.expected
            $proposal = $transport.parsed.proposal
            $fatalError = $null
            $fatalClassification = $null
            if ($transport.http_status -lt 200 -or $transport.http_status -ge 300) { $fatalClassification = 'LOCALAI_HTTP_FAILURE'; $fatalError = "LOCALAI_HTTP_FAILURE:$($transport.http_status) case=$($case.id)" }
            elseif (-not $transport.parsed.strict_utf8_decode) { $fatalClassification = 'INVALID_UTF8_RESPONSE'; $fatalError = "INVALID_UTF8_RESPONSE case=$($case.id)" }
            elseif (-not $transport.parsed.envelope_json_parse) { $fatalClassification = 'MALFORMED_OPENAI_ENVELOPE'; $fatalError = "MALFORMED_OPENAI_ENVELOPE case=$($case.id)" }
            $modelErrors = @()
            $finalErrors = @()
            $exactAuthority = $null
            $phraseAuthority = $null
            $scopeAuthority = $null
            $finalProposal = $null
            if ($null -ne $fatalError) {
                $modelErrors = @($fatalClassification)
                $finalErrors = @($fatalClassification)
            } elseif (-not $transport.parsed.planner_json_parse) {
                $modelErrors = @('planner_json_parse')
                $finalErrors = @('planner_json_parse')
            } else {
                $modelErrors = @(Get-NxDPlannerOracleErrors $proposal $expected)
                $exactAuthority = Resolve-NxDSingleExactIdentifier `
                    ([string]$case.question) `
                    ([string](Get-OptionalProperty $proposal 'query_capability_id')) `
                    ([string](Get-OptionalProperty $proposal 'semantic')) `
                    ([string](Get-OptionalProperty $proposal 'literal_text')) `
                    (@(Get-OptionalProperty $proposal 'entities'))
                $modelClarification = Get-OptionalProperty $proposal 'clarification_required'
                $phraseAuthority = Resolve-NxDExplicitPhraseLiteral `
                    ([string]$case.question) `
                    ([string](Get-OptionalProperty $proposal 'query_capability_id')) `
                    ([string](Get-OptionalProperty $proposal 'semantic')) `
                    ([string](Get-OptionalProperty $proposal 'literal_text')) `
                    ([bool]$modelClarification)
                $scopeAuthority = Resolve-NxDExplicitScopeAuthority `
                    ([string]$case.question) `
                    ([string](Get-OptionalProperty $proposal 'scope'))
                $finalProposal = New-NxDFinalPlannerProposal $proposal $exactAuthority $phraseAuthority $scopeAuthority
                $finalErrors = @(Get-NxDPlannerOracleErrors $finalProposal $expected)
                if ([bool]$exactAuthority.applicable -and [string]$exactAuthority.resolution_status -ne 'RESOLVED_DETERMINISTIC_AUTHORITY') {
                    $finalErrors += "exact_identifier_authority:$([string]$exactAuthority.reason)"
                }
                if ([bool]$phraseAuthority.applicable -and [string]$phraseAuthority.resolution_status -ne 'RESOLVED_DETERMINISTIC_AUTHORITY') {
                    $finalErrors += "phrase_literal_authority:$([string]$phraseAuthority.reason)"
                }
                if ([bool]$scopeAuthority.applicable -and [string]$scopeAuthority.resolution_status -ne 'RESOLVED_DETERMINISTIC_AUTHORITY') {
                    $finalErrors += "explicit_scope_authority:$([string]$scopeAuthority.reason)"
                }
            }
            $modelStatus = if ($modelErrors.Count) { 'FAIL' } else { 'PASS' }
            $finalStatus = if ($finalErrors.Count) { 'FAIL' } else { 'PASS' }
            $modelClassification = Get-NxDResultClassification $modelErrors ([bool]$transport.parsed.planner_json_parse) $fatalClassification
            $finalClassification = Get-NxDResultClassification $finalErrors ([bool]$transport.parsed.planner_json_parse) $fatalClassification
            $modelLiteral = [string](Get-OptionalProperty $proposal 'literal_text')
            $finalLiteral = [string](Get-OptionalProperty $finalProposal 'literal_text')
            $modelUnicode = Get-NxDUnicodeDiagnostics ([string]$expected.literal_text) $modelLiteral
            $finalUnicode = Get-NxDUnicodeDiagnostics ([string]$expected.literal_text) $finalLiteral
            $modelShapeErrors = @($modelErrors | Where-Object { $_ -like 'planner_schema:*' })
            $finalShapeErrors = @($finalErrors | Where-Object { $_ -like 'planner_schema:*' })
            $receipt = [ordered]@{
                schema_version = 'nexusai.nxb21d-q4-scope-remediation-development-case/v1'
                created_at = [DateTime]::UtcNow.ToString('o')
                development_only = $true
                qualification_holdout = $false
                case_id = [string]$case.id
                language = [string]$case.language
                question = [string]$case.question
                expected_oracle = $expected
                model_planner_proposal = $proposal
                model_proposal_result = $modelStatus
                model_proposal_errors = $modelErrors
                exact_identifier_authority = $exactAuthority
                phrase_literal_authority = $phraseAuthority
                explicit_scope_authority = $scopeAuthority
                deterministic_exact_identifier_found = [bool]($null -ne $exactAuthority -and [bool]$exactAuthority.applicable -and @($exactAuthority.authoritative_spans).Count -eq 1)
                deterministic_phrase_found = [bool]($null -ne $phraseAuthority -and [bool]$phraseAuthority.applicable -and [string]$phraseAuthority.source_literal)
                deterministic_mismatch = [bool](($null -ne $exactAuthority -and [bool]$exactAuthority.applicable -and [bool]$exactAuthority.reconciled) -or ($null -ne $phraseAuthority -and [bool]$phraseAuthority.applicable -and [bool]$phraseAuthority.reconciled) -or ($null -ne $scopeAuthority -and [bool]$scopeAuthority.applicable -and [bool]$scopeAuthority.reconciled))
                exact_identifier_reconciled = [bool]($null -ne $exactAuthority -and [bool]$exactAuthority.applicable -and [bool]$exactAuthority.reconciled -and [string]$exactAuthority.resolution_status -ceq 'RESOLVED_DETERMINISTIC_AUTHORITY')
                phrase_literal_reconciled = [bool]($null -ne $phraseAuthority -and [bool]$phraseAuthority.applicable -and [bool]$phraseAuthority.reconciled -and [string]$phraseAuthority.resolution_status -ceq 'RESOLVED_DETERMINISTIC_AUTHORITY')
                explicit_scope_reconciled = [bool]($null -ne $scopeAuthority -and [bool]$scopeAuthority.applicable -and [bool]$scopeAuthority.reconciled -and [string]$scopeAuthority.resolution_status -ceq 'RESOLVED_DETERMINISTIC_AUTHORITY')
                deterministically_reconciled = [bool](($null -ne $exactAuthority -and [bool]$exactAuthority.applicable -and [bool]$exactAuthority.reconciled -and [string]$exactAuthority.resolution_status -ceq 'RESOLVED_DETERMINISTIC_AUTHORITY') -or ($null -ne $phraseAuthority -and [bool]$phraseAuthority.applicable -and [bool]$phraseAuthority.reconciled -and [string]$phraseAuthority.resolution_status -ceq 'RESOLVED_DETERMINISTIC_AUTHORITY') -or ($null -ne $scopeAuthority -and [bool]$scopeAuthority.applicable -and [bool]$scopeAuthority.reconciled -and [string]$scopeAuthority.resolution_status -ceq 'RESOLVED_DETERMINISTIC_AUTHORITY'))
                final_typed_plan = $finalProposal
                final_typed_plan_result = $finalStatus
                final_typed_plan_errors = $finalErrors
                result = $finalStatus
                errors = $finalErrors
                raw_request_sha256 = [string]$transport.request_sha256
                raw_response_sha256 = [string]$transport.response_sha256
                request_utf8_no_bom = -not [bool]$transport.transmitted_request_utf8_bom
                strict_utf8_decode = [bool]$transport.parsed.strict_utf8_decode
                envelope_json_parse = [bool]$transport.parsed.envelope_json_parse
                planner_json_parse = [bool]$transport.parsed.planner_json_parse
                model_planner_schema_compatible = [bool]($transport.parsed.planner_json_parse -and $modelShapeErrors.Count -eq 0)
                final_planner_schema_compatible = [bool]($transport.parsed.planner_json_parse -and $finalShapeErrors.Count -eq 0)
                model_literal_exactness = [bool]$modelUnicode.exact
                final_literal_exactness = [bool]$finalUnicode.exact
                model_unicode_diagnostics = $modelUnicode
                final_unicode_diagnostics = $finalUnicode
                latency_ms = [long]$transport.latency_ms
                resource_before = $beforeResource
                resource_after = $afterResource
                model_error_classification = $modelClassification
                error_classification = $finalClassification
                transport_error = $transport.parsed.error
                evidence_paths = [ordered]@{request = $transport.request_path; raw_response = $transport.raw_response_path; strict_response = $transport.strict_response_path}
            }
            Write-NxDJson (Join-Path $caseDirectory 'case-receipt-v1.json') $receipt
            $results.Add([pscustomobject]$receipt)
            if ($null -eq $fatalError) { Write-Host "BYTE_SAFE_TRANSPORT=PASS id=$($case.id) request_sha256=$($transport.request_sha256) response_sha256=$($transport.response_sha256)" }
            else { Write-Host "BYTE_SAFE_TRANSPORT=FAIL id=$($case.id) classification=$finalClassification" }
            $exactReconciled = [bool]($null -ne $exactAuthority -and [bool]$exactAuthority.applicable -and [bool]$exactAuthority.reconciled)
            $phraseReconciled = [bool]($null -ne $phraseAuthority -and [bool]$phraseAuthority.applicable -and [bool]$phraseAuthority.reconciled)
            $scopeReconciled = [bool]($null -ne $scopeAuthority -and [bool]$scopeAuthority.applicable -and [bool]$scopeAuthority.reconciled)
            Write-Host "NXB21D_SCOPE_REMEDIATION_CASE_RESULT id=$($case.id) model_result=$modelStatus final_result=$finalStatus exact_reconciled=$exactReconciled phrase_reconciled=$phraseReconciled scope_reconciled=$scopeReconciled latency_ms=$($transport.latency_ms) ram_before_gib=$($beforeResource.available_host_ram_gib) ram_after_gib=$($afterResource.available_host_ram_gib)"
            if ($null -ne $fatalError) { throw $fatalError }
            if ([double]$afterResource.available_host_ram_gib -lt [double]$cfg.resource.severe_pressure_gib) { throw "UNSAFE_RESOURCE_CONDITION:$($afterResource.available_host_ram_gib) case=$($case.id)" }
        }
    } finally {
        $httpClient.Dispose()
    }

    $after = Get-ContainerBaseline $cfg.runtime.container
    Assert-Equal 'post_localai_container_id' $after.id $before.id
    Assert-Equal 'post_localai_image' $after.image $before.image
    if ($after.restart_count -ne $before.restart_count) { throw 'INTEGRITY_MISMATCH:post_localai_restart_count' }
    Assert-Equal 'post_q8_profile' (Get-ContainerFileHash $cfg.runtime.container $cfg.runtime.q8_profile_container_path) ([string]$cfg.candidate.q8_profile_sha256)
    Assert-Equal 'post_q4_profile' (Get-ContainerFileHash $cfg.runtime.container $cfg.runtime.q4_profile_container_path) ([string]$cfg.candidate.profile_sha256)
    $integrity = [ordered]@{q8_profile_unchanged = $true; q4_profile_unchanged = $true; localai_container_identity_unchanged = $true; localai_image_unchanged = $true; localai_restart_count_unchanged = $true}
    Write-Host 'RUNTIME_INTEGRITY=PASS'
    $failed = @($results | Where-Object result -eq 'FAIL').Count
    if ($failed) {
        $exitCode = 22
        Write-Host "NXB21D_Q4_SCOPE_REMEDIATION_16CASE=DEVELOPMENT_FAILURE failed=$failed"
    } else {
        $exitCode = 0
        Write-Host 'NXB21D_Q4_SCOPE_REMEDIATION_16CASE=PASS'
    }
} catch {
    $terminalError = $_.Exception.Message
    if ($terminalError -like 'RAM_TOO_LOW*' -or $terminalError -like 'UNSAFE_RESOURCE_CONDITION*') { $exitCode = 10 }
    elseif ($terminalError -like 'INTEGRITY_*' -or $terminalError -like 'PREFLIGHT_*' -or $terminalError -like 'CORPUS_*' -or $terminalError -like 'RESUME_*' -or $terminalError -like 'Q4_REGISTRATION*' -or $terminalError -like 'LOCALAI_CONTAINER_*') { $exitCode = 11 }
    elseif ($terminalError -like 'LOCALAI_HTTP_*' -or $terminalError -like 'INVALID_UTF8_*' -or $terminalError -like 'MALFORMED_OPENAI_*') { $exitCode = 20 }
    Write-Host "NXB21D_Q4_SCOPE_REMEDIATION_16CASE=FAIL error=$terminalError exit_code=$exitCode"
} finally {
    if ($null -ne $summaryPath) {
        # Windows PowerShell 5.1 throws "Argument types do not match" when a
        # generic List[object] is wrapped directly in @(...). Enumerate it
        # through the pipeline so the final receipt remains PS 5.1 compatible.
        $rows = @($results | ForEach-Object { $_ })
        $latencies = @($rows | ForEach-Object { [long]$_.latency_ms })
        $ram = @($rows | ForEach-Object { [double]$_.resource_before.available_host_ram_gib; [double]$_.resource_after.available_host_ram_gib })
        $memory = @($rows | ForEach-Object { $_.resource_before.localai_memory_mib; $_.resource_after.localai_memory_mib } | Where-Object { $null -ne $_ })
        $summary = [ordered]@{
            schema_version = 'nexusai.nxb21d-q4-scope-remediation-16case-development-summary/v1'
            created_at = [DateTime]::UtcNow.ToString('o')
            development_only = $true
            qualification_holdout = $false
            candidate_tuple = if ($null -ne $cfg) { [ordered]@{candidate_id = $cfg.candidate.candidate_id; identity_sha256 = $cfg.candidate.identity_sha256; model_name = $cfg.candidate.model_name; model_artifact_sha256 = $cfg.candidate.artifact_sha256; q4_profile_sha256 = $cfg.candidate.profile_sha256; q8_profile_sha256 = $cfg.candidate.q8_profile_sha256; prompt_sha256 = $cfg.candidate.prompt_sha256; schema_sha256 = $cfg.candidate.schema_sha256; implementation_sha256 = $cfg.candidate.implementation_sha256; bridge_sha256 = $cfg.candidate.bridge_sha256; shared_scope_sha256 = $cfg.candidate.shared_scope_sha256; api_scope_sha256 = $cfg.candidate.api_scope_sha256; agent_scope_sha256 = $cfg.candidate.agent_scope_sha256; authority_sha256 = $cfg.candidate.authority_sha256; phrase_authority_sha256 = $cfg.candidate.phrase_authority_sha256; scope_authority_sha256 = $cfg.candidate.scope_authority_sha256; transport_sha256 = $cfg.candidate.transport_sha256; corpus_sha256 = $cfg.corpus.sha256; request_temperature = $cfg.candidate.request_temperature; effective_context = $cfg.candidate.effective_context} } else { $null }
            pre_remediation_32case_receipt = if ($null -ne $cfg) { $cfg.pre_remediation_32case_receipt } else { $null }
            atomic_remediation_16case_receipt = if ($null -ne $cfg) { $cfg.atomic_remediation_16case_receipt } else { $null }
            phrase_remediation_20case_receipt = if ($null -ne $cfg) { $cfg.phrase_remediation_20case_receipt } else { $null }
            resumed_from_receipts = $resumeSources
            counts = [ordered]@{total = 16; executed = $rows.Count; model_proposal_pass = @($rows | Where-Object model_proposal_result -eq 'PASS').Count; model_proposal_fail = @($rows | Where-Object model_proposal_result -eq 'FAIL').Count; final_typed_plan_pass = @($rows | Where-Object final_typed_plan_result -eq 'PASS').Count; final_typed_plan_fail = @($rows | Where-Object final_typed_plan_result -eq 'FAIL').Count; deterministic_reconciliation_count = @($rows | Where-Object deterministically_reconciled -eq $true).Count; exact_identifier_reconciliation_count = @($rows | Where-Object exact_identifier_reconciled -eq $true).Count; phrase_literal_reconciliation_count = @($rows | Where-Object phrase_literal_reconciled -eq $true).Count; explicit_scope_reconciliation_count = @($rows | Where-Object explicit_scope_reconciled -eq $true).Count; authority_rejection_count = @($rows | Where-Object { ($null -ne $_.exact_identifier_authority -and [bool]$_.exact_identifier_authority.applicable -and [string]$_.exact_identifier_authority.resolution_status -like 'REJECTED*') -or ($null -ne $_.phrase_literal_authority -and [bool]$_.phrase_literal_authority.applicable -and [string]$_.phrase_literal_authority.resolution_status -like 'REJECTED*') -or ($null -ne $_.explicit_scope_authority -and [bool]$_.explicit_scope_authority.applicable -and [string]$_.explicit_scope_authority.resolution_status -like 'REJECTED*') }).Count; language = Get-CountMap $rows 'language'; capability = Get-CountMap @($rows | ForEach-Object { [pscustomobject]@{value = $_.expected_oracle.query_capability_id} }) 'value'; semantic = Get-CountMap @($rows | ForEach-Object { [pscustomobject]@{value = $_.expected_oracle.semantic} }) 'value'; scope = Get-CountMap @($rows | ForEach-Object { [pscustomobject]@{value = $_.expected_oracle.scope} }) 'value'}
            model_failed_case_ids = @($rows | Where-Object model_proposal_result -eq 'FAIL' | ForEach-Object case_id)
            final_failed_case_ids = @($rows | Where-Object final_typed_plan_result -eq 'FAIL' | ForEach-Object case_id)
            transport = [ordered]@{request_mode = 'UTF-8 without BOM raw bytes'; response_mode = 'raw byte capture then strict UTF-8'; malformed_response_count = @($rows | Where-Object { -not $_.planner_json_parse }).Count; model_schema_incompatible_count = @($rows | Where-Object { -not $_.model_planner_schema_compatible }).Count; final_schema_incompatible_count = @($rows | Where-Object { -not $_.final_planner_schema_compatible }).Count; invalid_utf8_count = @($rows | Where-Object { -not $_.strict_utf8_decode }).Count}
            performance = [ordered]@{latency = Get-LatencySummary $latencies; resources = [ordered]@{minimum_available_host_ram_gib = if ($ram.Count) { ($ram | Measure-Object -Minimum).Minimum } else { $null }; maximum_localai_memory_mib = if ($memory.Count) { ($memory | Measure-Object -Maximum).Maximum } else { $null }}}
            integrity = $integrity
            terminal_error = $terminalError
            process_exit_code = $exitCode
            overall_development_result = if ($exitCode -eq 0) { 'PASS' } elseif ($exitCode -eq 22) { 'FAIL' } else { 'ABORTED' }
            D_status = 'OPEN'
            activation = 'BLOCKED'
            results = $rows
        }
        Write-NxDJson $summaryPath $summary
        [IO.File]::WriteAllText($summaryPath + '.sha256', (Get-NxDHash $summaryPath) + "  scope-remediation-16case-development-summary-v1.json`n", [Text.UTF8Encoding]::new($false))
        Write-Host "DEVELOPMENT_RECEIPT=$summaryPath"
        Write-Host "DEVELOPMENT_RECEIPT_SHA256=$(Get-NxDHash $summaryPath)"
        if ($exitCode -eq 0) { Write-Host 'EXACT_NEXT_ACTION=Reopen Codex and provide the receipt. Do not freeze a qualification holdout or activate D.' }
        elseif ($exitCode -eq 22) { Write-Host 'EXACT_NEXT_ACTION=Reopen Codex and provide the receipt for development-only diagnosis. Do not use the consumed holdout.' }
        else { Write-Host 'EXACT_NEXT_ACTION=Preserve the run directory, reopen Codex, and report the terminal error. Do not retry automatically if integrity or transport corruption was reported.' }
    }
    if ($transcriptStarted) { Stop-Transcript -ErrorAction SilentlyContinue | Out-Null }
}
exit $exitCode
