# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $PSScriptRoot 'byte_safe_transport.ps1')
$results = New-Object 'System.Collections.Generic.List[object]'

function Record([string]$Name, [scriptblock]$Test) {
    try {
        & $Test
        $results.Add([pscustomobject]@{name = $Name; status = 'PASS'})
    } catch {
        $results.Add([pscustomobject]@{name = $Name; status = 'FAIL'; error = $_.Exception.Message})
    }
}

function New-TestEnvelopeBytes([string]$Literal, [string]$Question, [string]$Capability = 'document.phrase') {
    $entities = @()
    if ($Capability -eq 'image.plate') { $entities = @($Literal) }
    $proposal = [ordered]@{
        query_capability_id = $Capability
        semantic = if ($Capability -eq 'image.plate') { 'EXACT_VALUE' } else { 'PHRASE_CONTAINS' }
        literal_text = $Literal
        entities = $entities
        scope = 'selected'
        clarification_required = $false
    }
    $content = ($proposal | ConvertTo-Json -Compress -Depth 10)
    return (ConvertTo-NxDJsonUtf8NoBOMBytes ([ordered]@{choices = @([ordered]@{message = [ordered]@{content = $content}}); question = $Question})).Bytes
}

Record 'urdu_response_round_trip' {
    $parsed = ConvertFrom-NxDPlannerResponseBytes (New-TestEnvelopeBytes 'اکاؤنٹ منجمد' 'اس دستاویز میں اکاؤنٹ منجمد تلاش کریں')
    if (-not $parsed.strict_utf8_decode -or -not $parsed.envelope_json_parse -or -not $parsed.planner_json_parse) { throw 'Urdu response did not parse.' }
    if ([string]$parsed.proposal.literal_text -cne 'اکاؤنٹ منجمد') { throw 'Urdu literal changed.' }
}
Record 'mixed_script_response_round_trip' {
    $parsed = ConvertFrom-NxDPlannerResponseBytes (New-TestEnvelopeBytes 'رقم موصول' 'Selected document میں رقم موصول check کریں')
    if ([string]$parsed.proposal.literal_text -cne 'رقم موصول') { throw 'Mixed-script literal changed.' }
}
Record 'outgoing_request_has_no_bom' {
    $request = ConvertTo-NxDJsonUtf8NoBOMBytes ([ordered]@{question = 'کل صبح'; literal = 'رقم موصول'})
    if ($request.Utf8BOM -or (Test-NxDUtf8BOM $request.Bytes)) { throw 'Outgoing request contains a BOM.' }
    if ((ConvertFrom-NxDStrictUtf8Bytes $request.Bytes) -notlike '*کل صبح*') { throw 'Outgoing Urdu was not preserved.' }
}
Record 'frozen_schema_request_serialization_is_bounded' {
    $config = Read-NxDStrictUtf8Text (Join-Path $PSScriptRoot 'nxb21d_q4_32case_config.json') | ConvertFrom-Json
    $prompt = Read-NxDStrictUtf8Text (Join-Path $root $config.candidate.prompt_path)
    $schema = Read-NxDStrictUtf8Text (Join-Path $root $config.candidate.schema_path) | ConvertFrom-Json
    $body = [ordered]@{
        model = $config.candidate.model_name
        messages = @([ordered]@{role = 'system'; content = $prompt}, [ordered]@{role = 'user'; content = 'اس منتخب عکس میں نمبر پلیٹ PEW-6143 چیک کریں۔'})
        temperature = 0
        max_tokens = 160
        response_format = [ordered]@{type = 'json_schema'; json_schema = [ordered]@{name = 'nxb21d_planner_proposal'; strict = $true; schema = $schema}}
    }
    $timer = [Diagnostics.Stopwatch]::StartNew()
    $request = ConvertTo-NxDJsonUtf8NoBOMBytes $body
    $timer.Stop()
    if ($timer.Elapsed.TotalSeconds -gt 5) { throw "Request serialization exceeded five seconds: $($timer.Elapsed.TotalSeconds)" }
    if ($request.Bytes.Length -gt 100000) { throw "Request serialization exceeded 100 KB: $($request.Bytes.Length)" }
    if (Test-NxDUtf8BOM $request.Bytes) { throw 'Frozen-schema request contains a BOM.' }
    if ((ConvertFrom-NxDStrictUtf8Bytes $request.Bytes) -notlike '*PEW-6143*') { throw 'Frozen-schema request lost the exact identifier.' }
}
Record 'windows_powershell_51_strict_utf8_serialization' {
    $testPath = Join-Path $PSScriptRoot 'test_nxb21d_q4_32case_windows_powershell.ps1'
    $output = & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $testPath
    if ($LASTEXITCODE -ne 0 -or ($output -join "`n") -notmatch 'NXB21D_PS51_SERIALIZATION=PASS') {
        throw "Windows PowerShell 5.1 serialization regression: $($output -join ' ')"
    }
}
Record 'strict_utf8_rejects_invalid_bytes' {
    $parsed = ConvertFrom-NxDPlannerResponseBytes ([byte[]](0xC3, 0x28))
    if ($parsed.strict_utf8_decode -or $parsed.error_classification -cne 'INVALID_UTF8_RESPONSE') { throw 'Invalid UTF-8 was accepted.' }
}
Record 'urdu_literal_codepoints_are_exact' {
    $diagnostic = Get-NxDUnicodeDiagnostics 'رقم موصول' 'رقم موصول'
    if (-not $diagnostic.exact -or $diagnostic.expected_codepoints.Count -eq 0) { throw 'Exact Unicode diagnostic failed.' }
    if (($diagnostic.expected_codepoints -join ',') -cne ($diagnostic.actual_codepoints -join ',')) { throw 'Codepoints differ.' }
}
Record 'raw_body_parser_is_iwr_independent' {
    $source = Get-Content (Join-Path $PSScriptRoot 'byte_safe_transport.ps1') -Raw
    if ($source -match 'Invoke-WebRequest') { throw 'Transport depends on Invoke-WebRequest.' }
    $parsed = ConvertFrom-NxDPlannerResponseBytes (New-TestEnvelopeBytes 'کل صبح' 'یہاں کل صبح تلاش کریں')
    if (-not $parsed.planner_json_parse) { throw 'Raw-byte parsing failed.' }
}
Record 'required_unicode_regression_fixtures' {
    foreach ($literal in @('اکاؤنٹ منجمد', 'کل صبح', 'رقم موصول')) {
        $parsed = ConvertFrom-NxDPlannerResponseBytes (New-TestEnvelopeBytes $literal ("اس source میں " + $literal + " تلاش کریں"))
        if ([string]$parsed.proposal.literal_text -cne $literal) { throw "Fixture changed: $literal" }
    }
}
Record 'planner_shape_is_strict' {
    $valid = [pscustomobject]@{query_capability_id = 'document.phrase'; semantic = 'PHRASE_CONTAINS'; literal_text = 'کل صبح'; entities = @(); scope = 'selected'; clarification_required = $false}
    if (@(Test-NxDPlannerObject $valid).Count -ne 0) { throw 'Valid planner object was rejected.' }
    $imageParsed = ConvertFrom-NxDPlannerResponseBytes (New-TestEnvelopeBytes 'LEA-4402' 'اس تصویر میں LEA-4402 تلاش کریں' 'image.plate')
    if (@(Test-NxDPlannerObject $imageParsed.proposal).Count -ne 0) { throw 'Single-entity planner object was rejected.' }
    $invalid = [pscustomobject]@{query_capability_id = 'document.phrase'; semantic = 'PHRASE_CONTAINS'; literal_text = 'کل صبح'; entities = @(); scope = 'selected'; clarification_required = $false; sql = 'SELECT 1'}
    if (@(Test-NxDPlannerObject $invalid) -cnotcontains 'additional_property:sql') { throw 'Additional planner property was accepted.' }
}
Record 'powershell_syntax' {
    foreach ($path in Get-ChildItem $PSScriptRoot -Filter '*.ps1') {
        $tokens = $null
        $errors = $null
        [Management.Automation.Language.Parser]::ParseFile($path.FullName, [ref]$tokens, [ref]$errors) | Out-Null
        if ($errors.Count) { throw "$($path.Name): $($errors[0].Message)" }
    }
}
Record 'corpus_validator' {
    $corpus = Join-Path $PSScriptRoot 'nxb21d-q4-32case-development-corpus-v1.json'
    $output = & python (Join-Path $PSScriptRoot 'validate_nxb21d_q4_32case_corpus.py') --repo $root --corpus $corpus
    if ($LASTEXITCODE -ne 0) { throw ($output -join [Environment]::NewLine) }
    $summary = ($output -join [Environment]::NewLine) | ConvertFrom-Json
    if ($summary.status -cne 'PASS' -or [int]$summary.case_count -ne 32 -or [int]$summary.fresh_question_count -ne 32) { throw 'Corpus validation result is incomplete.' }
}
Record 'framework_integrity' {
    $config = Get-Content (Join-Path $PSScriptRoot 'nxb21d_q4_32case_config.json') -Raw | ConvertFrom-Json
    foreach ($property in $config.framework_files.PSObject.Properties) {
        $path = Join-Path $root $property.Name
        $actual = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actual -cne [string]$property.Value) { throw "Framework hash mismatch: $($property.Name)" }
    }
    $sidecar = ((Get-Content (Join-Path $PSScriptRoot 'nxb21d-q4-32case-development-corpus-v1.json.sha256') -Raw).Trim() -split '\s+')[0]
    if ($sidecar -cne [string]$config.corpus.sha256) { throw 'Corpus sidecar mismatch.' }
}
Record 'runner_safety_and_receipt_contract' {
    $runner = Get-Content (Join-Path $PSScriptRoot 'run_nxb21d_q4_32case_development.ps1') -Raw
    foreach ($forbidden in @('Invoke-WebRequest', 'docker compose down', 'docker down -v', '--remove-orphans', 'docker system prune', 'docker volume prune', 'd-unseen-query-corpus-v1.json')) {
        if ($runner.Contains($forbidden)) { throw "Forbidden runner token: $forbidden" }
    }
    foreach ($required in @('CORPUS_FROZEN=PASS', 'CANDIDATE_FREEZE=PASS', 'BYTE_SAFE_TRANSPORT=PASS', 'RUNTIME_INTEGRITY=PASS', 'DEVELOPMENT_RECEIPT=', "qualification_holdout = `$false", "D_status = 'OPEN'", "activation = 'BLOCKED'")) {
        if (-not $runner.Contains($required)) { throw "Missing runner contract: $required" }
    }
}

$results | Format-Table -AutoSize
$passed = @($results | Where-Object status -eq 'PASS').Count
$failed = @($results | Where-Object status -eq 'FAIL').Count
if ($failed) {
    Write-Host "NXB21D_Q4_32CASE_SOURCE_TEST=FAIL passed=$passed failed=$failed"
    exit 1
}
Write-Host "NXB21D_Q4_32CASE_SOURCE_TEST=PASS count=$passed"
exit 0
