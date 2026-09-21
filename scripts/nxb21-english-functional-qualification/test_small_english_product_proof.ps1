[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$runner=Join-Path $PSScriptRoot 'run_small_english_product_proof.ps1'
$tokens=$null;$parseErrors=$null
$ast=[Management.Automation.Language.Parser]::ParseFile($runner,[ref]$tokens,[ref]$parseErrors)
if($parseErrors.Count -ne 0){throw ($parseErrors | Out-String)}
$commands=@($ast.FindAll({param($node) $node -is [Management.Automation.Language.CommandAst]},$true) | ForEach-Object{$_.GetCommandName()})
if(@($commands | Where-Object{$_ -eq 'Start-OwnedModel'}).Count -ne 1){throw 'Expected one model session'}
if($commands -contains 'Get-StableReloadWindow'){throw 'No reload experiment permitted'}
if($commands -contains 'Stop-Process'){throw 'No process-name termination permitted'}
if($commands -contains 'Invoke-Expression'){throw 'No dynamic top-level runner invocation permitted'}
if($commands -contains 'go'){throw 'Standalone inference may not compile'}
$text=[IO.File]::ReadAllText($runner)
if($text.IndexOf('New-ExclusiveLock $dispatchLock') -gt $text.IndexOf('Start-OwnedModel $false $true')){throw 'Consumption must precede model load'}
if(-not $text.Contains('-WindowStyle Hidden')){throw 'Background helper must be hidden'}
if(-not $text.Contains('Assert-RuntimeUnchanged $before $after')){throw 'Cleanup integrity check missing'}
Write-Host "SMALL_ENGLISH_RUNNER_OFFLINE=PASS POWERSHELL=$($PSVersionTable.PSVersion) LIVE_INFERENCE=false"
