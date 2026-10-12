<#
.SYNOPSIS
  Tests Initialize-Vault.ps1 and Register-Sample.ps1 on a temporary folder with a dummy synthetic file. Touches nothing else.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\tools\real_data\Test-VaultTools.ps1
#>
$ErrorActionPreference = 'Stop'
$here = $PSScriptRoot
$init = Join-Path $here 'Initialize-Vault.ps1'
$register = Join-Path $here 'Register-Sample.ps1'
$work = Join-Path ([System.IO.Path]::GetTempPath()) ('vaulttest-' + [guid]::NewGuid().ToString('N').Substring(0, 8))
$vault = Join-Path $work 'vault'
$dropbox = Join-Path $work 'inbox'
$passed = 0
$failures = @()

function Check {
  param([string]$Name, [scriptblock]$Body)
  try {
    & $Body
    $script:passed++
    Write-Host "ok    $Name"
  } catch {
    $script:failures += $Name
    Write-Host "FAIL  $Name : $($_.Exception.Message)"
  }
}

function Expect-Failure {
  param([scriptblock]$Body, [string]$Pattern)
  try { & $Body } catch {
    if ($_.Exception.Message -notmatch $Pattern) { throw "failed with an unexpected message: $($_.Exception.Message)" }
    return
  }
  throw "expected a failure matching '$Pattern' but it succeeded"
}

$authority = @{
  ProviderLabel = 'synthetic test provider'; Family = 'call_records'; LegalBasis = 'synthetic test basis'
  AuthorityReference = 'TEST-0001'; AuthorisedBy = 'test runner'; AllowedUses = 'structure profiling in tests'
  RetentionUntil = (Get-Date).AddDays(30)
}

try {
  New-Item -ItemType Directory -Force -Path $dropbox | Out-Null
  $sample = Join-Path $dropbox 'synthetic export (1).csv'
  Set-Content -LiteralPath $sample -Encoding ASCII -Value @('A,B', '1,2', '3,4')

  Check 'vault creation is refused in a scratch folder without the override' {
    Expect-Failure { & $init -Path $vault } 'Refusing to create the vault'
    if (Test-Path -LiteralPath $vault) { throw 'the folder was created anyway' }
  }
  Check 'vault creation is refused inside a git work tree' {
    $fake = Join-Path $work 'repo'
    New-Item -ItemType Directory -Force -Path (Join-Path $fake '.git') | Out-Null
    Expect-Failure { & $init -Path (Join-Path $fake 'vault') } 'git work tree'
  }
  Check 'vault creation is refused inside a cloud-synchronised folder' {
    Expect-Failure { & $init -Path (Join-Path $work 'OneDrive\vault') } 'cloud-synchronised'
  }
  Check '-WhatIf creates nothing' {
    & $init -Path $vault -AllowUnsafeLocation -WhatIf | Out-Null
    if (Test-Path -LiteralPath $vault) { throw 'WhatIf created the folder' }
  }
  Check 'vault is created with sub-folders, a marker and restricted access' {
    & $init -Path $vault -AllowUnsafeLocation | Out-Null
    foreach ($name in 'incoming', 'working', 'reports', 'register', '.nexusai-real-data-vault', 'README.txt') {
      if (-not (Test-Path -LiteralPath (Join-Path $vault $name))) { throw "missing $name" }
    }
    $acl = (& icacls $vault | Out-String)
    if ($acl -match '(?i)Everyone|Authenticated Users|BUILTIN\\Users') { throw "broad access remains: $acl" }
    if ($acl -notmatch [regex]::Escape($env:USERNAME)) { throw 'the current user has no access entry' }
  }
  Check 'a placeholder authority is refused' {
    $bad = $authority.Clone(); $bad.LegalBasis = '<your basis>'
    Expect-Failure { & $register -Source $sample -VaultPath $vault @bad } 'real value'
  }
  Check 'a past retention date is refused' {
    $bad = $authority.Clone(); $bad.RetentionUntil = (Get-Date).AddDays(-1)
    Expect-Failure { & $register -Source $sample -VaultPath $vault @bad } 'future date'
  }
  Check 'a folder that is not a vault is refused' {
    Expect-Failure { & $register -Source $sample -VaultPath $dropbox @authority } 'not a vault'
  }
  Check '-WhatIf registers nothing' {
    & $register -Source $sample -VaultPath $vault @authority -WhatIf | Out-Null
    if (Test-Path -LiteralPath (Join-Path $vault 'register\real-data-authority-register.csv')) { throw 'register was written' }
    if (@(Get-ChildItem (Join-Path $vault 'incoming')).Count -ne 0) { throw 'a file was copied' }
  }
  Check 'registration copies the file, verifies it and records the authority' {
    & $register -Source $sample -VaultPath $vault @authority -Notes '=HYPERLINK("x")' | Out-Null
    $rows = @(Import-Csv -LiteralPath (Join-Path $vault 'register\real-data-authority-register.csv'))
    if ($rows.Count -ne 1) { throw "expected 1 register row, found $($rows.Count)" }
    $row = $rows[0]
    $copy = Join-Path $vault $row.vault_relative_path
    if (-not (Test-Path -LiteralPath $copy)) { throw 'vault copy missing' }
    if ((Get-FileHash -Algorithm SHA256 -LiteralPath $copy).Hash.ToLowerInvariant() -ne $row.sha256) { throw 'hash mismatch' }
    if ($row.authority_reference -ne 'TEST-0001' -or $row.authorised_by -ne 'test runner') { throw 'authority not recorded' }
    if (-not $row.notes.StartsWith("'")) { throw 'a formula in a note was not neutralised' }
    if (-not (Test-Path -LiteralPath $sample)) { throw 'the original was removed without -Move' }
  }
  Check 'the same file cannot be registered twice' {
    Expect-Failure { & $register -Source $sample -VaultPath $vault @authority } 'already registered'
  }
  Check 'a file inside the vault cannot be registered' {
    $inside = Get-ChildItem -Recurse -File (Join-Path $vault 'incoming') | Select-Object -First 1
    Expect-Failure { & $register -Source $inside.FullName -VaultPath $vault @authority } 'already inside the vault'
  }
  Check '-Move verifies, records, then removes the original' {
    $second = Join-Path $dropbox 'second.csv'
    Set-Content -LiteralPath $second -Encoding ASCII -Value @('C,D', '5,6')
    & $register -Source $second -VaultPath $vault @authority -Move | Out-Null
    if (Test-Path -LiteralPath $second) { throw 'the original is still there' }
    $rows = @(Import-Csv -LiteralPath (Join-Path $vault 'register\real-data-authority-register.csv'))
    if ($rows.Count -ne 2) { throw "expected 2 register rows, found $($rows.Count)" }
  }
} finally {
  if (Test-Path -LiteralPath $work) { Microsoft.PowerShell.Management\Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue }
}

Write-Host ("{0} checks passed, {1} failed" -f $passed, $failures.Count)
if ($failures.Count -gt 0) { exit 1 }
