# SPDX-License-Identifier: MIT
Set-StrictMode -Version Latest
Add-Type -AssemblyName System.Net.Http

function Read-NxDStrictUtf8Text([string]$Path) {
    $bytes = [IO.File]::ReadAllBytes($Path)
    if (Test-NxDUtf8BOM $bytes) {
        if ($bytes.Length -eq 3) { return '' }
        $withoutBOM = New-Object byte[] ($bytes.Length - 3)
        [Array]::Copy($bytes, 3, $withoutBOM, 0, $withoutBOM.Length)
        return ConvertFrom-NxDStrictUtf8Bytes $withoutBOM
    }
    return ConvertFrom-NxDStrictUtf8Bytes $bytes
}

function Get-NxDBytesSHA256([byte[]]$Bytes) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        return ([BitConverter]::ToString($sha.ComputeHash($Bytes))).Replace('-', '').ToLowerInvariant()
    } finally {
        $sha.Dispose()
    }
}

function Test-NxDUtf8BOM([byte[]]$Bytes) {
    return $Bytes.Length -ge 3 -and $Bytes[0] -eq 0xEF -and $Bytes[1] -eq 0xBB -and $Bytes[2] -eq 0xBF
}

function ConvertTo-NxDJsonUtf8NoBOMBytes($Value, [ValidateRange(7, 20)][int]$Depth = 12) {
    $json = $Value | ConvertTo-Json -Depth $Depth -Compress
    $bytes = [Text.UTF8Encoding]::new($false).GetBytes($json)
    if (Test-NxDUtf8BOM $bytes) {
        throw 'OUTGOING_UTF8_BOM'
    }
    return [pscustomobject]@{
        Json = $json
        Bytes = $bytes
        SHA256 = Get-NxDBytesSHA256 $bytes
        Utf8BOM = $false
    }
}

function ConvertFrom-NxDStrictUtf8Bytes([byte[]]$Bytes) {
    return [Text.UTF8Encoding]::new($false, $true).GetString($Bytes)
}

function Get-NxDUnicodeCodePoints([AllowEmptyString()][string]$Text) {
    $points = New-Object 'System.Collections.Generic.List[string]'
    for ($index = 0; $index -lt $Text.Length; $index++) {
        $value = [char]::ConvertToUtf32($Text, $index)
        if ([char]::IsHighSurrogate($Text[$index])) {
            $index++
        }
        $points.Add(('U+{0:X4}' -f $value))
    }
    return @($points)
}

function Get-NxDUnicodeDiagnostics(
    [AllowEmptyString()][string]$Expected,
    [AllowEmptyString()][string]$Actual
) {
    return [ordered]@{
        exact = $Expected -ceq $Actual
        expected_length_utf16 = $Expected.Length
        actual_length_utf16 = $Actual.Length
        expected_codepoints = @(Get-NxDUnicodeCodePoints $Expected)
        actual_codepoints = @(Get-NxDUnicodeCodePoints $Actual)
        expected_normalization_form = if ($Expected.IsNormalized([Text.NormalizationForm]::FormC)) { 'NFC' } else { 'NON_NFC' }
        actual_normalization_form = if ($Actual.IsNormalized([Text.NormalizationForm]::FormC)) { 'NFC' } else { 'NON_NFC' }
        nfc_equal_diagnostic_only = $Expected.Normalize([Text.NormalizationForm]::FormC) -ceq $Actual.Normalize([Text.NormalizationForm]::FormC)
    }
}

function ConvertFrom-NxDPlannerResponseBytes([byte[]]$Bytes) {
    $result = [ordered]@{
        strict_utf8_decode = $false
        envelope_json_parse = $false
        planner_json_parse = $false
        response_text = $null
        planner_content = $null
        envelope = $null
        proposal = $null
        error_classification = $null
        error = $null
    }
    try {
        $result.response_text = ConvertFrom-NxDStrictUtf8Bytes $Bytes
        $result.strict_utf8_decode = $true
    } catch {
        $result.error_classification = 'INVALID_UTF8_RESPONSE'
        $result.error = $_.Exception.Message
        return [pscustomobject]$result
    }
    try {
        $result.envelope = $result.response_text | ConvertFrom-Json
        if (@($result.envelope.choices).Count -lt 1 -or $null -eq $result.envelope.choices[0].message.content) {
            throw 'OpenAI response has no choices[0].message.content.'
        }
        $result.planner_content = [string]$result.envelope.choices[0].message.content
        $result.envelope_json_parse = $true
    } catch {
        $result.error_classification = 'MALFORMED_OPENAI_ENVELOPE'
        $result.error = $_.Exception.Message
        return [pscustomobject]$result
    }
    try {
        $result.proposal = $result.planner_content | ConvertFrom-Json
        $result.planner_json_parse = $true
    } catch {
        $result.error_classification = 'MALFORMED_PLANNER_JSON'
        $result.error = $_.Exception.Message
    }
    return [pscustomobject]$result
}

function Test-NxDPlannerObject($Proposal) {
    $errors = New-Object 'System.Collections.Generic.List[string]'
    if ($null -eq $Proposal) {
        $errors.Add('proposal_missing')
        return @($errors)
    }
    $required = @('query_capability_id', 'semantic', 'literal_text', 'entities', 'scope', 'clarification_required')
    $names = @($Proposal.PSObject.Properties.Name)
    foreach ($name in $required) { if ($names -cnotcontains $name) { $errors.Add("required_missing:$name") } }
    foreach ($name in $names) { if ($required -cnotcontains $name) { $errors.Add("additional_property:$name") } }
    $capabilityProperty = $Proposal.PSObject.Properties['query_capability_id']
    $semanticProperty = $Proposal.PSObject.Properties['semantic']
    $literalProperty = $Proposal.PSObject.Properties['literal_text']
    $entitiesProperty = $Proposal.PSObject.Properties['entities']
    $scopeProperty = $Proposal.PSObject.Properties['scope']
    $clarificationProperty = $Proposal.PSObject.Properties['clarification_required']
    $capability = $null; if ($null -ne $capabilityProperty) { $capability = $capabilityProperty.Value }
    $semantic = $null; if ($null -ne $semanticProperty) { $semantic = $semanticProperty.Value }
    $literal = $null; if ($null -ne $literalProperty) { $literal = $literalProperty.Value }
    $entities = $null; if ($null -ne $entitiesProperty) { $entities = $entitiesProperty.Value }
    $scope = $null; if ($null -ne $scopeProperty) { $scope = $scopeProperty.Value }
    $clarification = $null; if ($null -ne $clarificationProperty) { $clarification = $clarificationProperty.Value }
    if (@('image.plate', 'document.phrase', 'transcript.phrase') -cnotcontains [string]$capability) { $errors.Add('invalid_capability') }
    if (@('EXACT_VALUE', 'PHRASE_CONTAINS') -cnotcontains [string]$semantic) { $errors.Add('invalid_semantic') }
    if ($null -eq $literal -or $literal -isnot [string]) { $errors.Add('invalid_literal_type') }
    if ($null -eq $entities -or $entities -isnot [Array]) { $errors.Add('invalid_entities_type') }
    else { foreach ($entity in $entities) { if ($entity -isnot [string]) { $errors.Add('invalid_entity_type') } } }
    if (@('selected', 'workspace') -cnotcontains [string]$scope) { $errors.Add('invalid_scope') }
    if ($clarification -isnot [bool]) { $errors.Add('invalid_clarification_type') }
    return @($errors)
}

function Invoke-NxDByteSafePlannerPost(
    [Net.Http.HttpClient]$HttpClient,
    [string]$Uri,
    $Body,
    [string]$EvidenceDirectory
) {
    if (-not (Test-Path -LiteralPath $EvidenceDirectory)) {
        New-Item -ItemType Directory -Force -Path $EvidenceDirectory | Out-Null
    }
    Write-Host 'BYTE_SAFE_TRANSPORT stage=serialize_begin'
    $request = ConvertTo-NxDJsonUtf8NoBOMBytes $Body
    Write-Host "BYTE_SAFE_TRANSPORT stage=serialize_pass bytes=$($request.Bytes.Length) sha256=$($request.SHA256)"
    $requestPath = Join-Path $EvidenceDirectory 'request-utf8-no-bom.json'
    $responsePath = Join-Path $EvidenceDirectory 'response-body-raw.bin'
    $decodedPath = Join-Path $EvidenceDirectory 'response-body-strict-utf8.json'
    [IO.File]::WriteAllBytes($requestPath, $request.Bytes)
    Write-Host "BYTE_SAFE_TRANSPORT stage=request_saved path=$requestPath"

    $content = New-Object Net.Http.ByteArrayContent -ArgumentList (,$request.Bytes)
    $content.Headers.ContentType = [Net.Http.Headers.MediaTypeHeaderValue]::Parse('application/json; charset=utf-8')
    $timer = [Diagnostics.Stopwatch]::StartNew()
    try {
        Write-Host "BYTE_SAFE_TRANSPORT stage=http_post_begin uri=$Uri"
        $response = $HttpClient.PostAsync($Uri, $content).GetAwaiter().GetResult()
        try {
            $responseBytes = $response.Content.ReadAsByteArrayAsync().GetAwaiter().GetResult()
            $statusCode = [int]$response.StatusCode
        } finally {
            $response.Dispose()
        }
    } finally {
        $timer.Stop()
        $content.Dispose()
    }
    [IO.File]::WriteAllBytes($responsePath, $responseBytes)
    Write-Host "BYTE_SAFE_TRANSPORT stage=http_post_pass status=$statusCode latency_ms=$($timer.ElapsedMilliseconds)"
    $parsed = ConvertFrom-NxDPlannerResponseBytes $responseBytes
    if ($parsed.strict_utf8_decode) {
        [IO.File]::WriteAllText($decodedPath, $parsed.response_text, [Text.UTF8Encoding]::new($false))
    }
    return [pscustomobject]@{
        http_status = $statusCode
        latency_ms = $timer.ElapsedMilliseconds
        request_sha256 = $request.SHA256
        response_sha256 = Get-NxDBytesSHA256 $responseBytes
        transmitted_request_utf8_bom = $request.Utf8BOM
        request_path = $requestPath
        raw_response_path = $responsePath
        strict_response_path = if ($parsed.strict_utf8_decode) { $decodedPath } else { $null }
        parsed = $parsed
    }
}
