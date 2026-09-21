# SPDX-License-Identifier: MIT
Set-StrictMode -Version Latest

function Get-NxQ4SHA256([string]$Path) {
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
}

function Assert-NxQ4HoldoutUnconsumed([string]$HoldoutPath, [string]$RegistryPath) {
    if (-not (Test-Path -LiteralPath $HoldoutPath -PathType Leaf)) { throw "HOLDOUT_MISSING:$HoldoutPath" }
    if (-not (Test-Path -LiteralPath $RegistryPath -PathType Leaf) -or -not (Test-Path -LiteralPath ($RegistryPath + '.sha256') -PathType Leaf)) {
        throw 'CONSUMED_HOLDOUT_REGISTRY_MISSING'
    }
    $expected = ((Get-Content -LiteralPath ($RegistryPath + '.sha256') -Raw).Trim() -split '\s+')[0]
    if ((Get-NxQ4SHA256 $RegistryPath) -cne $expected) { throw 'CONSUMED_HOLDOUT_REGISTRY_INTEGRITY_FAILED' }
    $actual = Get-NxQ4SHA256 $HoldoutPath
    $registry = Get-Content -LiteralPath $RegistryPath -Raw | ConvertFrom-Json
    if (@($registry.entries | Where-Object { [string]$_.holdout_sha256 -ceq $actual }).Count -gt 0) {
        throw "HOLDOUT_ALREADY_CONSUMED_INVALID_INCOMPLETE:$actual"
    }
}

function Assert-NxQ4QualificationReceipt($Receipt) {
    if ([string]$Receipt.schema_version -cne 'nexusai.nxb21d-q4-final-qualification-receipt/v1' -or
        [string]$Receipt.qualification_state -cne 'complete_pass' -or
        -not [bool]$Receipt.holdout_consumed -or [int]$Receipt.cases_completed -ne 168 -or
        [string]$Receipt.DExitDecision -cne 'PASS' -or [double]$Receipt.critical.percent -ne 100 -or
        [string]$Receipt.suitability -cne 'SUITABLE' -or [string]$Receipt.runtime_postcheck -cne 'PASS' -or
        -not [bool]$Receipt.model_unloaded) {
        throw 'ACTIVATION_PREREQUISITE_FAILED:Q4 qualification gate'
    }
}
