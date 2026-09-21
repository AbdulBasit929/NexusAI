# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([Parameter(Mandatory=$true)][int]$CodexProcessId,
      [Parameter(Mandatory=$true)][string]$CodexStartedAt,
      [Parameter(Mandatory=$true)][string]$LogDirectory)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$statePath=Join-Path $LogDirectory 'launcher-state.json'
function Write-State([string]$State,[string]$Detail){
    [IO.File]::WriteAllText($statePath,(@{state=$State;detail=$Detail;utc=[DateTime]::UtcNow.ToString('o')}|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
}
try{
    Set-Location $root
    $lock=Join-Path $root 'local-acceptance-models\nxb21-small-english-product-proof-v1\product-proof.dispatched.lock'
    if(Test-Path -LiteralPath $lock){throw 'Proof already consumed; no dispatch permitted'}
    Write-State 'WAITING_FOR_CODEX_TO_CLOSE' 'No inference, cleanup, or service changes while waiting; timeout 15 minutes.'
    $timer=[Diagnostics.Stopwatch]::StartNew()
    while($true){
        $process=Get-Process -Id $CodexProcessId -ErrorAction SilentlyContinue
        if($null -eq $process -or $process.StartTime.ToUniversalTime().ToString('o') -cne $CodexStartedAt){break}
        if($timer.Elapsed.TotalMinutes -ge 15){Write-State 'EXPIRED_NO_INFERENCE' 'Codex remained open; no attempt made.';exit 10}
        Start-Sleep -Seconds 3
    }
    Start-Sleep -Seconds 10
    Write-State 'RUNNING_FROZEN_PROOF' 'One attempt; all frozen admission and runtime guards remain active.'
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'run_small_english_product_proof.ps1') *> (Join-Path $LogDirectory 'proof-console.log')
    $proofExit=$LASTEXITCODE
    if($proofExit -ne 0){Write-State 'PROOF_STOPPED' "Exit code $proofExit; inspect proof-console.log and receipt. No automatic retry.";exit $proofExit}
    Write-State 'PROOF_FINISHED_REVIEW_REQUIRED' 'Read the generated receipt and all answers before declaring English proof passed.'
}catch{Write-State 'LAUNCHER_FAILED' $_.Exception.Message;exit 20}
