# SPDX-License-Identifier: MIT
# This file is intentionally ASCII-only so Windows PowerShell 5.1 can parse it
# independently of the active ANSI code page.
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
. (Join-Path $PSScriptRoot 'byte_safe_transport.ps1')

$configPath = Join-Path $PSScriptRoot 'nxb21d_q4_32case_config.json'
$config = Read-NxDStrictUtf8Text $configPath | ConvertFrom-Json
$corpus = Read-NxDStrictUtf8Text (Join-Path $root $config.corpus.path) | ConvertFrom-Json
$prompt = Read-NxDStrictUtf8Text (Join-Path $root $config.candidate.prompt_path)
$schema = Read-NxDStrictUtf8Text (Join-Path $root $config.candidate.schema_path) | ConvertFrom-Json
$question = [string]$corpus.cases[0].question
$body = [ordered]@{
    model = [string]$config.candidate.model_name
    messages = @(
        [ordered]@{ role = 'system'; content = $prompt },
        [ordered]@{ role = 'user'; content = $question }
    )
    temperature = [int]$config.candidate.request_temperature
    max_tokens = [int]$config.candidate.max_tokens
    response_format = [ordered]@{ type = 'json_schema'; json_schema = [ordered]@{ name = 'nxb21d_planner_proposal'; strict = $true; schema = $schema } }
}
$timer = [Diagnostics.Stopwatch]::StartNew()
$request = ConvertTo-NxDJsonUtf8NoBOMBytes $body
$timer.Stop()
if ($timer.Elapsed.TotalSeconds -gt 5) { throw "PS51_SERIALIZATION_TIMEOUT:$($timer.Elapsed.TotalSeconds)" }
if ($request.Bytes.Length -lt 1000 -or $request.Bytes.Length -gt 100000) { throw "PS51_REQUEST_SIZE_INVALID:$($request.Bytes.Length)" }
if (Test-NxDUtf8BOM $request.Bytes) { throw 'PS51_REQUEST_HAS_BOM' }
$decoded = ConvertFrom-NxDStrictUtf8Bytes $request.Bytes
if (-not $decoded.Contains($question)) { throw 'PS51_QUESTION_NOT_EXACT' }
$items = New-Object 'System.Collections.Generic.List[object]'
$items.Add([pscustomobject]@{ value = 1 })
$rows = @($items | ForEach-Object { $_ })
if ($rows.Count -ne 1 -or $rows[0].value -ne 1) { throw 'PS51_GENERIC_LIST_ENUMERATION_FAILED' }
Write-Output ('NXB21D_PS51_SERIALIZATION=PASS elapsed_ms={0} bytes={1}' -f $timer.ElapsedMilliseconds, $request.Bytes.Length)
