<#
.SYNOPSIS
  Creates the local real-data vault: one folder, outside every repository, cloud-sync folder and scratch directory, that only
  you (and the system and local administrators) can open.

.DESCRIPTION
  Part of docs/data/real-data-handling-policy.md (decision D1). The script:
    1. refuses a location inside a git work tree, a cloud-synchronised folder, or a temporary / scratch / Downloads folder;
    2. creates incoming, working, reports and register sub-folders and a marker file the other tools look for;
    3. removes inherited permissions and grants access to you, SYSTEM and local Administrators only;
    4. reports whether the volume is protected by BitLocker (needs an elevated shell to read; otherwise it says UNKNOWN), and with
       -EnableEfs also encrypts the folder with the Encrypting File System;
    5. optionally records the path in the user environment variable NEXUSAI_REAL_DATA_VAULT.
  It never reads, copies or moves a data file. Use -WhatIf to see every action first.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\tools\real_data\Initialize-Vault.ps1 -WhatIf
  powershell -ExecutionPolicy Bypass -File .\tools\real_data\Initialize-Vault.ps1 -SetEnvironmentVariable
#>
[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'Medium')]
param(
  [string]$Path = 'C:\NexusAI-RealDataVault',
  [switch]$EnableEfs,
  [switch]$SetEnvironmentVariable,
  # Tests only: skip the location refusal so the script can be exercised on a temporary folder with synthetic files.
  [switch]$AllowUnsafeLocation
)

$ErrorActionPreference = 'Stop'
$MarkerName = '.nexusai-real-data-vault'

function Get-LocationProblems {
  param([Parameter(Mandatory = $true)][string]$Candidate)
  $full = [System.IO.Path]::GetFullPath($Candidate)
  $problems = @()
  $probe = $full
  while ($probe) {
    if (Test-Path -LiteralPath (Join-Path $probe '.git')) {
      $problems += 'inside a git work tree: real data must live outside every repository'
      break
    }
    $parent = Split-Path -Parent $probe
    if (-not $parent -or $parent -eq $probe) { break }
    $probe = $parent
  }
  if ($full -match '(?i)(^|[\\/])(onedrive|dropbox|google ?drive|icloud|box|mega|sync\.com)[^\\/]*([\\/]|$)') {
    $problems += 'inside a cloud-synchronised folder: a sync client would upload the data'
  }
  if ($full -match '(?i)(^|[\\/])(appdata[\\/]local[\\/]temp|temp|tmp|scratch|scratchpad|downloads)([\\/]|$)') {
    $problems += 'inside a temporary, scratch or Downloads folder'
  }
  return $problems
}

function Get-VolumeProtection {
  param([Parameter(Mandatory = $true)][string]$Candidate)
  $drive = [System.IO.Path]::GetPathRoot([System.IO.Path]::GetFullPath($Candidate)).TrimEnd('\')
  try {
    $v = Get-BitLockerVolume -MountPoint $drive -ErrorAction Stop
    return [pscustomobject]@{ Method = 'BitLocker'; State = $(if ($v.ProtectionStatus -eq 'On') { 'ON' } else { 'OFF' }); Detail = "$drive $($v.VolumeStatus), protection $($v.ProtectionStatus)" }
  } catch {
    $text = ''
    try { $text = (& manage-bde -status $drive 2>&1 | Out-String) } catch { $text = '' }
    if ($text -match 'Protection On') { return [pscustomobject]@{ Method = 'BitLocker'; State = 'ON'; Detail = "$drive protection on" } }
    if ($text -match 'Protection Off') { return [pscustomobject]@{ Method = 'BitLocker'; State = 'OFF'; Detail = "$drive protection off" } }
    return [pscustomobject]@{ Method = 'BitLocker'; State = 'UNKNOWN'; Detail = "could not read $drive (run an elevated shell, or check Settings > Privacy and security > Device encryption)" }
  }
}

$problems = Get-LocationProblems -Candidate $Path
if ($problems.Count -gt 0 -and -not $AllowUnsafeLocation) {
  throw ("Refusing to create the vault at '{0}': {1}. Choose a folder such as C:\NexusAI-RealDataVault." -f $Path, ($problems -join '; '))
}

$full = [System.IO.Path]::GetFullPath($Path)
$identity = "$env:USERDOMAIN\$env:USERNAME"
$subfolders = @('incoming', 'working', 'reports', 'register')

if ($PSCmdlet.ShouldProcess($full, 'Create the vault folders and marker file')) {
  New-Item -ItemType Directory -Force -Path $full | Out-Null
  foreach ($name in $subfolders) { New-Item -ItemType Directory -Force -Path (Join-Path $full $name) | Out-Null }
  $marker = Join-Path $full $MarkerName
  if (-not (Test-Path -LiteralPath $marker)) {
    Set-Content -LiteralPath $marker -Encoding ASCII -Value ("NexusAI real-data vault. Created {0:o} by {1}. Do not place this folder under a repository or a sync client." -f (Get-Date).ToUniversalTime(), $identity)
  }
  $readme = Join-Path $full 'README.txt'
  if (-not (Test-Path -LiteralPath $readme)) {
    Set-Content -LiteralPath $readme -Encoding ASCII -Value @(
      'NexusAI real-data vault',
      '',
      'incoming  files as received, one folder per registered sample',
      'working   extracted copies for profiling; empty it when you finish',
      'reports   local-mode structure reports (exact counts) - stay here',
      'register  real-data-authority-register.csv: the authority for each file',
      '',
      'Rules: never copy, commit, paste, upload or e-mail anything from here.',
      'Only shareable structure reports (profile_structure.py, default mode) may leave, after you have read them.',
      'Policy: docs/data/real-data-handling-policy.md in the repository.')
  }
}

if ($PSCmdlet.ShouldProcess($full, "Restrict access to $identity, SYSTEM and local Administrators")) {
  & icacls $full /inheritance:r /grant:r "${identity}:(OI)(CI)F" 'SYSTEM:(OI)(CI)F' 'BUILTIN\Administrators:(OI)(CI)F' | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "icacls failed with exit code $LASTEXITCODE" }
}

$protection = Get-VolumeProtection -Candidate $full
if ($EnableEfs) {
  if ($PSCmdlet.ShouldProcess($full, 'Encrypt the folder with the Encrypting File System (cipher /e)')) {
    & cipher /e /s:"$full" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "cipher /e failed with exit code $LASTEXITCODE (EFS needs Windows Pro or Enterprise and an NTFS volume)" }
    Write-Warning 'EFS keys live in your Windows profile. Back up the certificate now (cipher /x) and keep the backup outside this machine and outside any repository.'
  }
}

if ($SetEnvironmentVariable) {
  if ($PSCmdlet.ShouldProcess('NEXUSAI_REAL_DATA_VAULT', "Set the user environment variable to $full")) {
    [Environment]::SetEnvironmentVariable('NEXUSAI_REAL_DATA_VAULT', $full, 'User')
  }
}

Write-Host ("Vault path      : {0}" -f $full)
Write-Host ("Location checks : {0}" -f $(if ($problems.Count -eq 0) { 'passed' } else { 'OVERRIDDEN (tests only): ' + ($problems -join '; ') }))
Write-Host ("Volume encryption: {0} ({1})" -f $protection.State, $protection.Detail)
if ($protection.State -ne 'ON' -and -not $EnableEfs) {
  Write-Warning 'The volume is not confirmed encrypted. Turn on BitLocker for this drive, or rerun with -EnableEfs, before registering any real file.'
}
Write-Host 'Next: tools\real_data\Register-Sample.ps1 -Source <file> ... (it refuses to register a file without its authority).'
