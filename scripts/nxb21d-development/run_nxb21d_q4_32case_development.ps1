# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $root 'scripts\nxb21d-offline\common.ps1')
. (Join-Path $PSScriptRoot 'byte_safe_transport.ps1')

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

function Get-ContainerBaseline([string]$Container) {
    $row = (Invoke-NxDNative docker @('inspect', $Container) | ConvertFrom-Json)[0]
    if (-not (Test-NxDContainerHealthy $row)) { throw 'LOCALAI_CONTAINER_UNHEALTHY' }
    return [ordered]@{id = [string]$row.Id; image = [string]$row.Image; restart_count = [int]$row.RestartCount; status = [string]$row.State.Status; health = if ($row.State.PSObject.Properties['Health']) { [string]$row.State.Health.Status } else { 'none' }}
}

function Get-ContainerFileHash([string]$Container, [string]$Path) {
    return ((Invoke-NxDNative docker @('exec', $Container, 'sha256sum', $Path)) -split '\s+')[0].ToLowerInvariant()
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

try {
    $cfgPath = Join-Path $PSScriptRoot 'nxb21d_q4_32case_config.json'
    if (-not (Test-Path -LiteralPath $cfgPath)) { throw 'PREFLIGHT_MISSING:development config' }
    $cfg = Read-NxDStrictUtf8Text $cfgPath | ConvertFrom-Json
    Assert-NxDBundleIntegrity
    foreach ($property in $cfg.framework_files.PSObject.Properties) {
        $path = Resolve-NxDPath $property.Name
        if (-not (Test-Path -LiteralPath $path)) { throw "PREFLIGHT_MISSING:$($property.Name)" }
        Assert-Equal $property.Name (Get-NxDHash $path) ([string]$property.Value)
    }

    $runRoot = Resolve-NxDPath $cfg.output_root
    $run = Join-Path $runRoot ('run-' + [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'))
    New-Item -ItemType Directory -Force -Path $run | Out-Null
    $summaryPath = Join-Path $run '32case-development-summary-v1.json'
    [IO.File]::WriteAllText((Join-Path $runRoot 'latest-run.txt'), $run + "`n", [Text.UTF8Encoding]::new($false))
    Start-Transcript -LiteralPath (Join-Path $run 'operator.log') -Force | Out-Null
    $transcriptStarted = $true
    Write-Host "NXB21D_Q4_32CASE=START run=$run"

    $corpusPath = Resolve-NxDPath $cfg.corpus.path
    $corpusBytes = [IO.File]::ReadAllBytes($corpusPath)
    if (Test-NxDUtf8BOM $corpusBytes) { throw 'CORPUS_UTF8_BOM' }
    Assert-Equal 'corpus' (Get-NxDHash $corpusPath) ([string]$cfg.corpus.sha256)
    $validationOutput = & python (Resolve-NxDPath $cfg.corpus.validator) --repo $root --corpus $corpusPath
    if ($LASTEXITCODE -ne 0) { throw "CORPUS_VALIDATION_FAILED:$($validationOutput -join ' ')" }
    $corpusValidation = ($validationOutput -join [Environment]::NewLine) | ConvertFrom-Json
    $corpus = ConvertFrom-NxDStrictUtf8Bytes $corpusBytes | ConvertFrom-Json
    if ([int]$corpusValidation.case_count -ne 32) { throw 'CORPUS_VALIDATION_FAILED:count' }
    Write-Host "CORPUS_FROZEN=PASS sha256=$($cfg.corpus.sha256) cases=32"

    foreach ($namedPath in @(
        @{name = 'q4_artifact'; path = $cfg.candidate.artifact_path; hash = $cfg.candidate.artifact_sha256},
        @{name = 'q4_profile_host'; path = $cfg.candidate.profile_path; hash = $cfg.candidate.profile_sha256},
        @{name = 'planner_prompt'; path = $cfg.candidate.prompt_path; hash = $cfg.candidate.prompt_sha256},
        @{name = 'planner_schema'; path = $cfg.candidate.schema_path; hash = $cfg.candidate.schema_sha256},
        @{name = 'adjudicated_12case_receipt'; path = $cfg.prior_adjudicated_receipt.path; hash = $cfg.prior_adjudicated_receipt.sha256}
    )) {
        $path = Resolve-NxDPath ([string]$namedPath.path)
        if (-not (Test-Path -LiteralPath $path)) { throw "PREFLIGHT_MISSING:$($namedPath.name)" }
        Assert-Equal ([string]$namedPath.name) (Get-NxDHash $path) ([string]$namedPath.hash)
    }
    $artifactBytes = (Get-Item -LiteralPath (Resolve-NxDPath $cfg.candidate.artifact_path)).Length
    if ($artifactBytes -ne [int64]$cfg.candidate.artifact_bytes) { throw "INTEGRITY_MISMATCH:q4_artifact_bytes actual=$artifactBytes" }

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
    Assert-DevelopmentRAM (Get-NxDRamGiB) $requiredInitial 'before_first_inference'
    Write-Host "Q4_REGISTRATION=PASS loaded=$q4LoadedBefore"
    Write-Host "CANDIDATE_FREEZE=PASS artifact_sha256=$($cfg.candidate.artifact_sha256) profile_sha256=$($cfg.candidate.profile_sha256) prompt_sha256=$($cfg.candidate.prompt_sha256) schema_sha256=$($cfg.candidate.schema_sha256)"

    $plannerSystem = Read-NxDStrictUtf8Text (Resolve-NxDPath $cfg.candidate.prompt_path)
    $plannerSchema = Read-NxDStrictUtf8Text (Resolve-NxDPath $cfg.candidate.schema_path) | ConvertFrom-Json
    $httpClient = [Net.Http.HttpClient]::new()
    $httpClient.Timeout = [TimeSpan]::FromSeconds([double]$cfg.runtime.request_timeout_seconds)
    try {
        $ordinal = 0
        foreach ($case in @($corpus.cases)) {
            $ordinal++
            $caseDirectory = Join-Path $run ([string]$case.id)
            $loadedNow = @(Get-LoadedModelIDs $cfg.runtime.localai_url) -contains [string]$cfg.candidate.model_name
            $beforeResource = Get-ResourceSnapshot $cfg.runtime.container
            $required = if ($loadedNow) { [double]$cfg.resource.minimum_loaded_gib } else { [double]$cfg.resource.minimum_before_load_gib }
            Assert-DevelopmentRAM ([double]$beforeResource.available_host_ram_gib) $required ([string]$case.id)
            Write-Host "NXB21D_CASE $ordinal/32 id=$($case.id) language=$($case.language) loaded=$loadedNow"

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
            $errors = New-Object 'System.Collections.Generic.List[string]'
            $expected = $case.expected
            $proposal = $transport.parsed.proposal
            $shapeErrors = @()
            $fatalError = $null
            $fatalClassification = $null
            if ($transport.http_status -lt 200 -or $transport.http_status -ge 300) { $fatalClassification = 'LOCALAI_HTTP_FAILURE'; $fatalError = "LOCALAI_HTTP_FAILURE:$($transport.http_status) case=$($case.id)" }
            elseif (-not $transport.parsed.strict_utf8_decode) { $fatalClassification = 'INVALID_UTF8_RESPONSE'; $fatalError = "INVALID_UTF8_RESPONSE case=$($case.id)" }
            elseif (-not $transport.parsed.envelope_json_parse) { $fatalClassification = 'MALFORMED_OPENAI_ENVELOPE'; $fatalError = "MALFORMED_OPENAI_ENVELOPE case=$($case.id)" }
            if ($null -ne $fatalError) {
                $errors.Add($fatalClassification)
            } elseif (-not $transport.parsed.planner_json_parse) {
                $errors.Add('planner_json_parse')
            } else {
                $shapeErrors = @(Test-NxDPlannerObject $proposal)
                foreach ($shapeError in $shapeErrors) { $errors.Add("planner_schema:$shapeError") }
                $actualCapability = [string](Get-OptionalProperty $proposal 'query_capability_id')
                $actualSemantic = [string](Get-OptionalProperty $proposal 'semantic')
                $actualLiteral = [string](Get-OptionalProperty $proposal 'literal_text')
                $actualScope = [string](Get-OptionalProperty $proposal 'scope')
                $actualClarification = Get-OptionalProperty $proposal 'clarification_required'
                $actualEntities = @(Get-OptionalProperty $proposal 'entities')
                if ($actualCapability -cne [string]$expected.query_capability_id) { $errors.Add('capability') }
                if ($actualSemantic -cne [string]$expected.semantic) { $errors.Add('semantic') }
                if ($actualLiteral -cne [string]$expected.literal_text) { $errors.Add('literal_exactness') }
                if ($actualScope -cne [string]$expected.scope) { $errors.Add('scope') }
                if ($null -eq $actualClarification -or [bool]$actualClarification -ne [bool]$expected.clarification_required) { $errors.Add('clarification') }
                $expectedEntities = @($expected.entities)
                if ($actualEntities.Count -ne $expectedEntities.Count) { $errors.Add('entity_count') }
                foreach ($entity in $expectedEntities) { if ($actualEntities -cnotcontains [string]$entity) { $errors.Add("entity_missing:$entity") } }
            }
            $actualCapability = [string](Get-OptionalProperty $proposal 'query_capability_id')
            $actualSemantic = [string](Get-OptionalProperty $proposal 'semantic')
            $actualLiteral = [string](Get-OptionalProperty $proposal 'literal_text')
            $actualScope = [string](Get-OptionalProperty $proposal 'scope')
            $actualClarification = Get-OptionalProperty $proposal 'clarification_required'
            $unicode = Get-NxDUnicodeDiagnostics ([string]$expected.literal_text) $actualLiteral
            $status = if ($errors.Count) { 'FAIL' } else { 'PASS' }
            $classification = if ($null -ne $fatalError) { $fatalClassification } elseif (-not $transport.parsed.planner_json_parse) { 'MALFORMED_PLANNER_JSON' } elseif ($shapeErrors.Count) { 'PLANNER_SCHEMA_MISMATCH' } elseif ($errors.Count) { 'SEMANTIC_ORACLE_MISMATCH' } else { 'NONE' }
            $receipt = [ordered]@{
                schema_version = 'nexusai.nxb21d-q4-development-case/v1'
                created_at = [DateTime]::UtcNow.ToString('o')
                development_only = $true
                qualification_holdout = $false
                case_id = [string]$case.id
                language = [string]$case.language
                question = [string]$case.question
                expected_oracle = $expected
                actual_planner_proposal = $proposal
                result = $status
                errors = @($errors)
                raw_request_sha256 = [string]$transport.request_sha256
                raw_response_sha256 = [string]$transport.response_sha256
                request_utf8_no_bom = -not [bool]$transport.transmitted_request_utf8_bom
                strict_utf8_decode = [bool]$transport.parsed.strict_utf8_decode
                envelope_json_parse = [bool]$transport.parsed.envelope_json_parse
                planner_json_parse = [bool]$transport.parsed.planner_json_parse
                planner_schema_compatible = [bool]($transport.parsed.planner_json_parse -and $shapeErrors.Count -eq 0)
                capability_result = $null -ne $proposal -and $actualCapability -ceq [string]$expected.query_capability_id
                semantic_result = $null -ne $proposal -and $actualSemantic -ceq [string]$expected.semantic
                literal_exactness = [bool]$unicode.exact
                entity_exactness = $null -ne $proposal -and -not (@($errors | Where-Object { $_ -like 'entity*' }).Count)
                scope_result = $null -ne $proposal -and $actualScope -ceq [string]$expected.scope
                clarification_result = $null -ne $proposal -and $null -ne $actualClarification -and [bool]$actualClarification -eq [bool]$expected.clarification_required
                unicode_diagnostics = $unicode
                latency_ms = [long]$transport.latency_ms
                resource_before = $beforeResource
                resource_after = $afterResource
                error_classification = $classification
                transport_error = $transport.parsed.error
                evidence_paths = [ordered]@{request = $transport.request_path; raw_response = $transport.raw_response_path; strict_response = $transport.strict_response_path}
            }
            Write-NxDJson (Join-Path $caseDirectory 'case-receipt-v1.json') $receipt
            $results.Add([pscustomobject]$receipt)
            if ($null -eq $fatalError) { Write-Host "BYTE_SAFE_TRANSPORT=PASS id=$($case.id) request_sha256=$($transport.request_sha256) response_sha256=$($transport.response_sha256)" }
            else { Write-Host "BYTE_SAFE_TRANSPORT=FAIL id=$($case.id) classification=$classification" }
            Write-Host "NXB21D_CASE_RESULT id=$($case.id) result=$status latency_ms=$($transport.latency_ms) ram_before_gib=$($beforeResource.available_host_ram_gib) ram_after_gib=$($afterResource.available_host_ram_gib)"
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
        Write-Host "NXB21D_Q4_32CASE=DEVELOPMENT_FAILURE failed=$failed"
    } else {
        $exitCode = 0
        Write-Host 'NXB21D_Q4_32CASE=PASS'
    }
} catch {
    $terminalError = $_.Exception.Message
    if ($terminalError -like 'RAM_TOO_LOW*' -or $terminalError -like 'UNSAFE_RESOURCE_CONDITION*') { $exitCode = 10 }
    elseif ($terminalError -like 'INTEGRITY_*' -or $terminalError -like 'PREFLIGHT_*' -or $terminalError -like 'CORPUS_*' -or $terminalError -like 'Q4_REGISTRATION*' -or $terminalError -like 'LOCALAI_CONTAINER_*') { $exitCode = 11 }
    elseif ($terminalError -like 'LOCALAI_HTTP_*' -or $terminalError -like 'INVALID_UTF8_*' -or $terminalError -like 'MALFORMED_OPENAI_*') { $exitCode = 20 }
    Write-Host "NXB21D_Q4_32CASE=FAIL error=$terminalError exit_code=$exitCode"
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
            schema_version = 'nexusai.nxb21d-q4-32case-development-summary/v1'
            created_at = [DateTime]::UtcNow.ToString('o')
            development_only = $true
            qualification_holdout = $false
            candidate_tuple = if ($null -ne $cfg) { [ordered]@{model_name = $cfg.candidate.model_name; model_artifact_sha256 = $cfg.candidate.artifact_sha256; q4_profile_sha256 = $cfg.candidate.profile_sha256; q8_profile_sha256 = $cfg.candidate.q8_profile_sha256; prompt_sha256 = $cfg.candidate.prompt_sha256; schema_sha256 = $cfg.candidate.schema_sha256; corpus_sha256 = $cfg.corpus.sha256; request_temperature = $cfg.candidate.request_temperature; effective_context = $cfg.candidate.effective_context} } else { $null }
            counts = [ordered]@{total = 32; executed = $rows.Count; pass = @($rows | Where-Object result -eq 'PASS').Count; fail = @($rows | Where-Object result -eq 'FAIL').Count; language = Get-CountMap $rows 'language'; capability = Get-CountMap @($rows | ForEach-Object { [pscustomobject]@{value = $_.expected_oracle.query_capability_id} }) 'value'; semantic = Get-CountMap @($rows | ForEach-Object { [pscustomobject]@{value = $_.expected_oracle.semantic} }) 'value'; scope = Get-CountMap @($rows | ForEach-Object { [pscustomobject]@{value = $_.expected_oracle.scope} }) 'value'}
            failed_case_ids = @($rows | Where-Object result -eq 'FAIL' | ForEach-Object case_id)
            transport = [ordered]@{request_mode = 'UTF-8 without BOM raw bytes'; response_mode = 'raw byte capture then strict UTF-8'; malformed_response_count = @($rows | Where-Object { -not $_.planner_json_parse }).Count; schema_incompatible_count = @($rows | Where-Object { -not $_.planner_schema_compatible }).Count; invalid_utf8_count = @($rows | Where-Object { -not $_.strict_utf8_decode }).Count}
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
        [IO.File]::WriteAllText($summaryPath + '.sha256', (Get-NxDHash $summaryPath) + "  32case-development-summary-v1.json`n", [Text.UTF8Encoding]::new($false))
        Write-Host "DEVELOPMENT_RECEIPT=$summaryPath"
        Write-Host "DEVELOPMENT_RECEIPT_SHA256=$(Get-NxDHash $summaryPath)"
        if ($exitCode -eq 0) { Write-Host 'EXACT_NEXT_ACTION=Reopen Codex and provide the receipt. Do not freeze a qualification holdout or activate D.' }
        elseif ($exitCode -eq 22) { Write-Host 'EXACT_NEXT_ACTION=Reopen Codex and provide the receipt for development-only diagnosis. Do not use the consumed holdout.' }
        else { Write-Host 'EXACT_NEXT_ACTION=Preserve the run directory, reopen Codex, and report the terminal error. Do not retry automatically if integrity or transport corruption was reported.' }
    }
    if ($transcriptStarted) { Stop-Transcript -ErrorAction SilentlyContinue | Out-Null }
}
exit $exitCode
