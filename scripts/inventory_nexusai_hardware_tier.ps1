param(
  [string]$OutputPath = '',
  [string]$HardwareTier = 'D0'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ($HardwareTier -ne 'D0') {
  throw 'This script inventories installed hardware only. Future tiers are target profiles, not installed facts.'
}

function Convert-BytesToGiB([double]$Bytes) {
  return [math]::Round($Bytes / 1GB, 3)
}

$cpu = Get-CimInstance Win32_Processor | Select-Object -First 1
$computer = Get-CimInstance Win32_ComputerSystem
$os = Get-CimInstance Win32_OperatingSystem
$gpus = @(Get-CimInstance Win32_VideoController | ForEach-Object {
  [ordered]@{
    model = [string]$_.Name
    reported_adapter_memory_gib = if ($null -ne $_.AdapterRAM) { Convert-BytesToGiB $_.AdapterRAM } else { $null }
    driver_version = [string]$_.DriverVersion
    video_processor = [string]$_.VideoProcessor
    status = [string]$_.Status
  }
})
$disk = Get-CimInstance Win32_LogicalDisk -Filter "DeviceID='C:'"
$nvidiaSmi = Get-Command nvidia-smi -ErrorAction SilentlyContinue
$docker = $null
try {
  $dockerRaw = docker info --format '{{json .}}' 2>$null
  if ($LASTEXITCODE -eq 0 -and $dockerRaw) {
    $dockerInfo = $dockerRaw | ConvertFrom-Json
    $docker = [ordered]@{
      available = $true
      server_version = [string]$dockerInfo.ServerVersion
      operating_system = [string]$dockerInfo.OperatingSystem
      architecture = [string]$dockerInfo.Architecture
      logical_cpus = [int]$dockerInfo.NCPU
      memory_gib = Convert-BytesToGiB $dockerInfo.MemTotal
      root_dir = [string]$dockerInfo.DockerRootDir
    }
  }
} catch {
  $docker = [ordered]@{ available = $false; reason = $_.Exception.Message }
}
if ($null -eq $docker) { $docker = [ordered]@{ available = $false; reason = 'docker info unavailable' } }

$profile = [ordered]@{
  contract_version = 'nexusai.hardware-inventory/v1'
  hardware_tier = $HardwareTier
  captured_at_utc = (Get-Date).ToUniversalTime().ToString('o')
  host = [ordered]@{
    os = [string]$os.Caption
    os_version = [string]$os.Version
    architecture = [string]$os.OSArchitecture
    cpu_model = ([string]$cpu.Name).Trim()
    physical_cores = [int]$cpu.NumberOfCores
    logical_cores = [int]$cpu.NumberOfLogicalProcessors
    max_clock_mhz_reported = [int]$cpu.MaxClockSpeed
    system_ram_gib = Convert-BytesToGiB $computer.TotalPhysicalMemory
    available_ram_gib = [math]::Round(([double]$os.FreePhysicalMemory * 1KB) / 1GB, 3)
    system_drive_size_gib = Convert-BytesToGiB $disk.Size
    system_drive_free_gib = Convert-BytesToGiB $disk.FreeSpace
  }
  accelerators = [ordered]@{
    gpus = $gpus
    nvidia_smi_available = [bool]$nvidiaSmi
    cuda_state = if ($nvidiaSmi) { 'installed_cli_detected_requires_runtime_qualification' } else { 'not_detected' }
    directml_state = 'not_qualified; Windows capability is not evidence that an ONNX DirectML runtime is installed or accurate'
    openvino_state = 'isolated_Docker_CPU_evaluator_measured; host_CLI_not_required'
  }
  container_runtime = $docker
  interpretation = [ordered]@{
    installed_tier = 'D0'
    discrete_gpu_available = [bool]$nvidiaSmi
    production_gpu_claim_allowed = $false
    note = 'This is a point-in-time resource record. Available RAM and disk are volatile.'
  }
}

$json = $profile | ConvertTo-Json -Depth 8
if ($OutputPath) {
  $parent = Split-Path -Parent $OutputPath
  if ($parent) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
  [IO.File]::WriteAllText($OutputPath, $json + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
}
$json
Write-Host "NexusAIHardwareInventory=PASS Tier=$HardwareTier Output=$OutputPath ProductionMutation=false"
