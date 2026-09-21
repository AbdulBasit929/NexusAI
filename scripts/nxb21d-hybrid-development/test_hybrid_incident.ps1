# SPDX-License-Identifier: MIT
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'immutable_receipt.ps1')
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$dir=Join-Path $root ('tmp\hybrid-receipt-regression-'+[Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $dir|Out-Null
$lock=Join-Path $dir 'development-dispatched.lock'
[IO.File]::WriteAllText($lock,'synthetic claimed dispatch')
$lockHash=(Get-FileHash $lock).Hash
foreach($gate in @('PASS','FAIL')){
    $path=Join-Path $dir ($gate+'.json')
    $seal=Seal-NxHybridReceipt $path ([pscustomobject]@{development_gate=$gate;runtime_postcheck='PASS'})
    $before=Get-Item -LiteralPath $path
    $timestamp=$before.LastWriteTimeUtc.Ticks
    if((Get-FileHash $path).Hash.ToLowerInvariant()-cne$seal.sha256){throw 'HASH_MISMATCH'}
    if(([IO.File]::ReadAllText($path+'.sha256').Trim()-split' ')[0]-cne$seal.sha256){throw 'SIDECAR_MISMATCH'}
    Write-NxHybridReceiptOutput $seal
    Write-NxHybridReceiptOutput $seal
    $rejected=$false
    try{$null=Seal-NxHybridReceipt $path ([pscustomobject]@{development_gate='changed'})}catch{$rejected=$true}
    if(-not$rejected){throw 'RESEAL_NOT_REJECTED'}
    if((Get-FileHash $path).Hash.ToLowerInvariant()-cne$seal.sha256-or(Get-Item $path).LastWriteTimeUtc.Ticks-ne$timestamp){throw 'FINAL_MUTATED'}
}
if((Get-FileHash $lock).Hash-cne$lockHash){throw 'LOCK_MUTATED'}
$runner=[IO.File]::ReadAllText((Join-Path $PSScriptRoot 'run_nxb21d_q4_hybrid_32case_development.ps1'))
if([regex]::Matches($runner,'\$seal=Seal-NxHybridReceipt').Count-ne1){throw 'FINALIZATION_NOT_SINGLE'}
if($runner-match 'Write-NxDJson \$summaryPath|WriteAllText\(\$summaryPath'){throw 'DIRECT_FINAL_WRITE'}
if($runner-notmatch '\[IO.FileMode\]::CreateNew'){throw 'DISPATCH_GUARD_MISSING'}
$tokens=$null;$errors=$null
$null=[Management.Automation.Language.Parser]::ParseInput($runner,[ref]$tokens,[ref]$errors)
if($errors.Count){throw "RUNNER_SYNTAX:$errors"}
Write-Host 'HYBRID_INCIDENT_RECEIPT_TEST=PASS success_and_failure_sealed_once=true sidecars_match=true repeated_output_stable=true lock_preserved=true'
