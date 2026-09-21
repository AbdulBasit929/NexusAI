# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$runner=Join-Path $PSScriptRoot 'run_current4b_runtime_characterization_v2.ps1'
$tokens=$null; $parseErrors=$null
$ast=[Management.Automation.Language.Parser]::ParseFile($runner,[ref]$tokens,[ref]$parseErrors)
if ($parseErrors.Count -ne 0) { throw ($parseErrors | Out-String) }
# Import definitions only: the test cannot dispatch the runner's top-level work.
foreach ($definition in $ast.FindAll({param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst]},$false)) {
    . ([scriptblock]::Create($definition.Extent.Text))
}
function Assert-Equal($Actual,$Expected,[string]$Name) {
    if ($Actual -ne $Expected) { throw "${Name}: expected=$Expected actual=$Actual" }
}
function Assert-Fails([scriptblock]$Action,[string]$Expected) {
    $caught=$false
    try { & $Action } catch {
        $caught=$true
        if ($_.Exception.Message -notlike "*$Expected*") { throw }
    }
    if (-not $caught) { throw "Expected failure: $Expected" }
}
$config=[pscustomobject]@{model=[pscustomobject]@{id='dummy'};runtime=[pscustomobject]@{localai_url='http://127.0.0.1:1'}}
$before=[ordered]@{containers=[ordered]@{'nexusai-api-1'=[ordered]@{id='api';restart_count=0}}}
$observationsPath='unused'
$backendRows=@()
function Get-BackendMemory { return $script:backendRows }
function Get-FreeRAMGiB { return 3.5 }
function Get-APIContainerMemoryMiB { return 10 }
function Get-VmmemWorkingSetMiB { return 20 }
function Get-CimInstance([string]$ClassName) {
    if ($ClassName -eq 'Win32_PageFileUsage') { return [pscustomobject]@{AllocatedBaseSize=1000;CurrentUsage=10;PeakUsage=20} }
    return [pscustomobject]@{CommittedBytes=4GB;CommitLimit=10GB;PagesInputPersec=0;PagesOutputPersec=0}
}
function Get-Process { return @() }
function Invoke-Native([string]$File,[string[]]$Arguments) {
    if ($File -eq 'wsl.exe') { return "SwapTotal: 1024 kB`nSwapFree: 1000 kB" }
    return '{"Name":"nexusai-api-1","MemUsage":"10MiB / 10GiB","CPUPerc":"0%"}'
}
function Append-JSONLine([string]$Path,$Value) {}
$zero=Get-Telemetry 'unloaded' 0 0
Assert-Equal $zero.backend_rss_total_mib 0 'unloaded RSS'
Assert-Equal $zero.commit_reserve_gib 6 'commit reserve'
Assert-Equal $zero.wsl_swap_used_kib 24 'swap accounting'
$backendRows=@([ordered]@{rss_mib=1.5},[ordered]@{rss_mib=2.5})
Assert-Equal (Get-Telemetry 'loaded' 1 1).backend_rss_total_mib 4 'dictionary RSS'
Assert-Equal (Get-Telemetry 'loaded' 1 1).historical_4gib_guard_met $false 'diagnostic floor'

$modelOwned=$false; $requestCount=0
function Get-Telemetry { throw 'INJECTED_TELEMETRY_FAILURE' }
Assert-Fails { Invoke-ResourceCall 'model_load' 1 1 $false } 'INJECTED_TELEMETRY_FAILURE'
Assert-Equal $modelOwned $false 'no ownership before dispatch'
Assert-Equal $requestCount 0 'no request before telemetry'
$modelOwned=$true; $unloadState='NOT_REQUIRED'
function Get-LoadedState { return $null }
function Invoke-RestMethod { throw 'UNEXPECTED_HTTP' }
Invoke-UnloadOwnedModel
Assert-Equal $modelOwned $false 'absent backend ownership cleared'
Assert-Equal $unloadState 'ALREADY_UNLOADED' 'absent backend cleanup'

$experimentTimer=[Diagnostics.Stopwatch]::StartNew()
$baselineSwapKiB=$null; $pagingSince=$null
function Get-Containers { return [ordered]@{'nexusai-api-1'=[ordered]@{id='api';restart_count=0;oom_killed=$false;status='running';health='healthy'}} }
Assert-ExperimentalSafety $zero
$zero.available_ram_gib=1.5
Assert-Fails { Assert-ExperimentalSafety $zero } 'PHYSICAL_RESERVE'
$zero.available_ram_gib=3.5; $zero.commit_reserve_gib=1.9
Assert-Fails { Assert-ExperimentalSafety $zero } 'COMMIT_RESERVE'
$zero.commit_reserve_gib=6; $zero.wsl_swap_used_kib=300000
Assert-Fails { Assert-ExperimentalSafety $zero } 'SWAP_GROWTH'
$zero.wsl_swap_used_kib=24; $zero.pages_input_per_second=2000; $pagingSince=[DateTime]::UtcNow.AddSeconds(-20)
Assert-Fails { Assert-ExperimentalSafety $zero } 'SUSTAINED_PAGING'
$zero.pages_input_per_second=0; $pagingSince=$null
function Get-Containers { return [ordered]@{'nexusai-api-1'=[ordered]@{id='api';restart_count=1;oom_killed=$false;status='running';health='healthy'}} }
Assert-Fails { Assert-ExperimentalSafety $zero } 'SERVICE_STATE'
$tiny=(New-GenericBody 'READY' $false).body | ConvertFrom-Json
$planner=(New-GenericBody 'READY' $true).body | ConvertFrom-Json
$synthesis=(New-GenericBody 'READY' $true $true).body | ConvertFrom-Json
Assert-Equal $tiny.max_tokens 24 'tiny budget'
Assert-Equal @($planner.response_format.json_schema.schema.properties.status.enum).Count 69 'planner schema size'
Assert-Equal $synthesis.max_tokens 256 'synthesis budget'
Assert-Equal ($synthesis.response_format.json_schema.schema.required -contains 'summary') $true 'synthesis shape'
Write-Host "CHARACTERIZATION_V2_UNIT=PASS POWERSHELL=$($PSVersionTable.PSVersion) LIVE_INFERENCE=false"
