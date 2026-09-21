# SPDX-License-Identifier: MIT
function Seal-NxHybridReceipt([string]$Path,$Aggregate) {
    # CreateNew is the finalization boundary; an existing final can never be replaced.
    $bytes=[Text.UTF8Encoding]::new($false).GetBytes(($Aggregate|ConvertTo-Json -Depth 100)+"`n")
    $stream=[IO.File]::Open($Path,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
    try{$stream.Write($bytes,0,$bytes.Length);$stream.Flush($true)}finally{$stream.Dispose()}
    $sha=[Security.Cryptography.SHA256]::Create()
    try{$hash=([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-','').ToLowerInvariant()}finally{$sha.Dispose()}
    $sidecar=[Text.UTF8Encoding]::new($false).GetBytes($hash+'  '+[IO.Path]::GetFileName($Path)+"`n")
    $stream=[IO.File]::Open($Path+'.sha256',[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
    try{$stream.Write($sidecar,0,$sidecar.Length);$stream.Flush($true)}finally{$stream.Dispose()}
    return [pscustomobject]@{path=$Path;sha256=$hash}
}
function Write-NxHybridReceiptOutput($Seal) {
    if((Get-FileHash -LiteralPath $Seal.path -Algorithm SHA256).Hash.ToLowerInvariant() -cne $Seal.sha256){throw 'FINAL_RECEIPT_CHANGED'}
    Write-Host "DEVELOPMENT_RECEIPT=$($Seal.path)"
    Write-Host "DEVELOPMENT_RECEIPT_SHA256=$($Seal.sha256)"
}
