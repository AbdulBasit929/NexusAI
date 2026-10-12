<#
.SYNOPSIS
  Brings one real file into the vault and records the authority under which it may be used, in the same step.

.DESCRIPTION
  Part of docs/data/real-data-handling-policy.md (decision D1). A file is registered only with its authority: the legal basis,
  a reference, who authorised the use, what it may be used for and how long it may be kept. The script:
    - computes the SHA-256 and refuses a file that is already registered;
    - copies the file into <vault>\incoming\<sample id>\ (or moves it with -Move) and verifies the copy by hash before touching
      the original;
    - appends one row to <vault>\register\real-data-authority-register.csv (values that start with = + - @ are neutralised so
      the register is safe to open in a spreadsheet).
  Nothing in the repository is read or written. The register never leaves the vault.

  The authority fields are yours to fill: the project does not guess a legal basis. See the template header in
  docs/data/templates/real-data-authority-register.csv.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\tools\real_data\Register-Sample.ps1 -Source "$env:USERPROFILE\Downloads\export.csv" `
    -ProviderLabel 'operator-x export' -Family call_records -LegalBasis '<your basis>' -AuthorityReference '<reference>' `
    -AuthorisedBy '<name and role>' -AllowedUses 'structure profiling; local reconciliation' -RetentionUntil 2027-06-30 -WhatIf
#>
[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'Medium')]
param(
  [Parameter(Mandatory = $true)][string]$Source,
  [string]$VaultPath = $(if ($env:NEXUSAI_REAL_DATA_VAULT) { $env:NEXUSAI_REAL_DATA_VAULT } else { 'C:\NexusAI-RealDataVault' }),
  [Parameter(Mandatory = $true)][string]$ProviderLabel,
  [Parameter(Mandatory = $true)][ValidateSet('call_records', 'geofence_dump', 'ipdr', 'app_usage', 'subscriber', 'transactions', 'audio', 'image', 'video', 'document', 'other')][string]$Family,
  [Parameter(Mandatory = $true)][string]$LegalBasis,
  [Parameter(Mandatory = $true)][string]$AuthorityReference,
  [Parameter(Mandatory = $true)][string]$AuthorisedBy,
  [Parameter(Mandatory = $true)][string]$AllowedUses,
  [Parameter(Mandatory = $true)][datetime]$RetentionUntil,
  [string]$ProhibitedUses = 'commit to any repository; paste, upload or e-mail outside the vault; send to a cloud service or a model',
  [string]$DisposalMethod = 'secure deletion of the vault copy and every extracted working copy on the retention date',
  [ValidateSet('yes', 'no', 'unknown')][string]$ContainsPersonalData = 'yes',
  [string]$Notes = '',
  [switch]$Move
)

$ErrorActionPreference = 'Stop'
$MarkerName = '.nexusai-real-data-vault'
$Columns = @('sample_id', 'registered_utc', 'registered_by', 'sha256', 'size_bytes', 'file_extension', 'vault_relative_path',
  'provider_label', 'family', 'legal_basis', 'authority_reference', 'authorised_by', 'allowed_uses', 'prohibited_uses',
  'retention_until', 'disposal_method', 'contains_personal_data', 'notes')

function Protect-CsvCell {
  param([string]$Value)
  if ($Value -match '^[=+\-@\t\r]') { return "'" + $Value }
  return $Value
}

foreach ($name in 'ProviderLabel', 'LegalBasis', 'AuthorityReference', 'AuthorisedBy', 'AllowedUses') {
  $value = (Get-Variable -Name $name -ValueOnly)
  if ([string]::IsNullOrWhiteSpace($value) -or $value -match '^<.*>$') {
    throw "$name must be filled in with the real value (a template placeholder such as '<your basis>' is not accepted)."
  }
}
if ($RetentionUntil -le (Get-Date)) { throw 'RetentionUntil must be a future date.' }

$vault = [System.IO.Path]::GetFullPath($VaultPath)
if (-not (Test-Path -LiteralPath (Join-Path $vault $MarkerName))) {
  throw "'$vault' is not a vault. Create it first with Initialize-Vault.ps1."
}
$src = (Resolve-Path -LiteralPath $Source).Path
if (-not (Test-Path -LiteralPath $src -PathType Leaf)) { throw 'Source must be a file.' }
if ($src.StartsWith($vault, [System.StringComparison]::OrdinalIgnoreCase)) { throw 'Source is already inside the vault.' }

$registerDir = Join-Path $vault 'register'
$register = Join-Path $registerDir 'real-data-authority-register.csv'
$hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $src).Hash.ToLowerInvariant()
$size = (Get-Item -LiteralPath $src).Length

if (Test-Path -LiteralPath $register) {
  $existing = @(Import-Csv -LiteralPath $register)
  if ($existing | Where-Object { $_.sha256 -eq $hash }) { throw 'A file with this SHA-256 is already registered.' }
}

$sampleId = 'RD-{0}-{1}' -f (Get-Date).ToUniversalTime().ToString('yyyyMMdd'), $hash.Substring(0, 8)
$safeName = [regex]::Replace([System.IO.Path]::GetFileName($src), '[^A-Za-z0-9._-]', '_')
$targetDir = Join-Path (Join-Path $vault 'incoming') $sampleId
$target = Join-Path $targetDir $safeName
$relative = $target.Substring($vault.Length).TrimStart('\')

$row = [ordered]@{
  sample_id = $sampleId
  registered_utc = (Get-Date).ToUniversalTime().ToString('o')
  registered_by = "$env:USERDOMAIN\$env:USERNAME"
  sha256 = $hash
  size_bytes = $size
  file_extension = [System.IO.Path]::GetExtension($src).ToLowerInvariant()
  vault_relative_path = $relative
  provider_label = $ProviderLabel
  family = $Family
  legal_basis = $LegalBasis
  authority_reference = $AuthorityReference
  authorised_by = $AuthorisedBy
  allowed_uses = $AllowedUses
  prohibited_uses = $ProhibitedUses
  retention_until = $RetentionUntil.ToString('yyyy-MM-dd')
  disposal_method = $DisposalMethod
  contains_personal_data = $ContainsPersonalData
  notes = $Notes
}
$safeRow = [ordered]@{}
foreach ($column in $Columns) { $safeRow[$column] = Protect-CsvCell ([string]$row[$column]) }

$action = if ($Move) { 'Move into the vault, verify by hash, then remove the original' } else { 'Copy into the vault and verify by hash' }
if ($PSCmdlet.ShouldProcess("$safeName -> $relative", $action)) {
  New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
  Copy-Item -LiteralPath $src -Destination $target
  $copyHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $target).Hash.ToLowerInvariant()
  if ($copyHash -ne $hash) {
    Microsoft.PowerShell.Management\Remove-Item -LiteralPath $targetDir -Recurse -Force
    throw 'The copy in the vault does not match the original by SHA-256; nothing was registered.'
  }
  New-Item -ItemType Directory -Force -Path $registerDir | Out-Null
  $line = [pscustomobject]$safeRow
  if (Test-Path -LiteralPath $register) {
    $line | Export-Csv -LiteralPath $register -Append -NoTypeInformation -Encoding UTF8
  } else {
    $line | Export-Csv -LiteralPath $register -NoTypeInformation -Encoding UTF8
  }
  if ($Move) {
    Microsoft.PowerShell.Management\Remove-Item -LiteralPath $src -Force
  }
  Write-Host ("Registered {0}: {1} bytes, SHA-256 {2}..." -f $sampleId, $size, $hash.Substring(0, 12))
  Write-Host ("Vault copy   : {0}" -f $relative)
  Write-Host 'Next         : profile it - python -I -X utf8 tools\real_data\profile_structure.py <vault copy> --out <vault>\reports\<id>.json'
  if (-not $Move) { Write-Warning 'The original is still where it was. Delete it (or rerun with -Move) once you have confirmed the vault copy.' }
}
