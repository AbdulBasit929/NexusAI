# SPDX-License-Identifier: MIT
[CmdletBinding()]
param(
  [Parameter(Mandatory=$true)][ValidateSet('MODEL_LOADED_RAM','INFERENCE_RAM')][string]$Stage,
  [Parameter(Mandatory=$true)][int]$CompletedCalls
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$os = Get-CimInstance Win32_OperatingSystem
$available = [math]::Round(([int64]$os.FreePhysicalMemory * 1024) / 1GB, 3)
Write-Host "$Stage available_gib=$available required_gib=4 completed_calls=$CompletedCalls"
if ($available -lt 4) { exit 10 }
