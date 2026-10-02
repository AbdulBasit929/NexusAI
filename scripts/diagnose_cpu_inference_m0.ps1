<#
.SYNOPSIS
  M0 diagnostic: why is CPU token generation about 4 tokens per second? Read-only.

.DESCRIPTION
  Collects, from the Windows host and the running LocalAI container:
    - CPU model, cores, clocks and instruction-set flags (AVX2, AVX-512, VNNI, AMX)
    - installed memory modules, speed, and an estimate of channels and peak bandwidth
    - power plan, AC or battery, and current against maximum CPU clock
    - the WSL2 / Docker VM limits (.wslconfig, docker info) and the container's own CPU and memory limits
    - the llama.cpp process arguments and thread settings, when visible
  then times the real model through the LocalAI API to split prefill (reading the prompt) from decode (producing
  tokens), and compares the decode speed with what the memory could deliver.

  It changes nothing: no model download, no configuration or container change, no database access, no evidence is
  read. The only requests sent are short synthetic prompts (random nonce, so the prompt cache cannot flatter the
  result) to the local model. Output goes to reports\m0-cpu-diagnostic-<timestamp>\ (report.md and report.json).

.PARAMETER BaseUri
  LocalAI base address. Default http://localhost:8080.

.PARAMETER Model
  Model name as LocalAI knows it. Default is the shipped synthesis model.

.PARAMETER ApiKey
  Optional bearer key if LocalAI requires one.

.PARAMETER ContainerName
  LocalAI container. If omitted, the first running container whose image name contains "localai" is used.

.PARAMETER ModelSizeGB
  Size of the model file in GB, used to turn decode speed into effective memory bandwidth. Looked up in the
  container when omitted; 2.5 is used if it cannot be found.

.PARAMETER Quick
  Short prompts only, two repetitions. Finishes in a couple of minutes. Without it the script also times a long
  prompt, which is what real planner prompts look like and can take several minutes on a slow machine.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts\diagnose_cpu_inference_m0.ps1 -Quick
#>
[CmdletBinding()]
param(
  [string]$BaseUri = 'http://localhost:8080',
  [string]$Model = 'qwen3-4b-instruct-2507-q4km-nxb21d-dev',
  [string]$ApiKey = '',
  [string]$ContainerName = '',
  [double]$ModelSizeGB = 0,
  [switch]$Quick
)

$ErrorActionPreference = 'Continue'
$repoRoot = Split-Path -Parent $PSScriptRoot
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$outDir = Join-Path $repoRoot ("reports\m0-cpu-diagnostic-" + $stamp)
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

$report = [ordered]@{ generated_at = (Get-Date).ToString('o'); findings = @(); host = [ordered]@{}; docker = [ordered]@{}; timing = [ordered]@{} }
$findings = New-Object System.Collections.Generic.List[string]
function Add-Finding([string]$Text) { $script:findings.Add($Text); Write-Host ("  * " + $Text) }
function Say([string]$Text) { Write-Host $Text }

function Try-Run([scriptblock]$Block, $Default = $null) {
  try { return (& $Block) } catch { return $Default }
}

# ---------------------------------------------------------------- host: CPU
Say '1. Host CPU'
$cpu = Try-Run { Get-CimInstance Win32_Processor | Select-Object -First 1 }
if ($cpu) {
  $report.host.cpu = [ordered]@{
    name = ($cpu.Name -replace '\s+', ' ').Trim()
    physical_cores = $cpu.NumberOfCores
    logical_processors = $cpu.NumberOfLogicalProcessors
    max_clock_mhz = $cpu.MaxClockSpeed
    current_clock_mhz = $cpu.CurrentClockSpeed
    l3_cache_kb = $cpu.L3CacheSize
  }
  Say ("   " + $report.host.cpu.name + ": " + $cpu.NumberOfCores + " cores / " + $cpu.NumberOfLogicalProcessors + " threads, max " + $cpu.MaxClockSpeed + " MHz")
} else { Add-Finding 'Could not read the CPU from WMI.' }

# ---------------------------------------------------------------- host: memory
Say '2. Host memory'
$modules = @(Try-Run { Get-CimInstance Win32_PhysicalMemory } @())
$typeNames = @{ 20 = 'DDR'; 21 = 'DDR2'; 24 = 'DDR3'; 26 = 'DDR4'; 34 = 'DDR5'; 35 = 'LPDDR5'; 29 = 'LPDDR3'; 30 = 'LPDDR4' }
$memRows = @()
foreach ($m in $modules) {
  $speed = $m.ConfiguredClockSpeed
  if (-not $speed) { $speed = $m.Speed }
  $typeCode = [int]$m.SMBIOSMemoryType
  $typeName = $typeNames[$typeCode]
  if (-not $typeName) { $typeName = "type $typeCode" }
  $memRows += [ordered]@{ locator = $m.DeviceLocator; bank = $m.BankLabel; capacity_gb = [math]::Round($m.Capacity / 1GB, 1); speed_mts = $speed; type = $typeName; manufacturer = $m.Manufacturer }
}
$report.host.memory_modules = $memRows
$totalGb = 0
foreach ($memRow in $memRows) { $totalGb += $memRow.capacity_gb }
Say ("   " + $memRows.Count + " module(s), " + $totalGb + " GB total")
foreach ($row in $memRows) { Say ("   - " + $row.locator + ": " + $row.capacity_gb + " GB " + $row.type + " @ " + $row.speed_mts + " MT/s") }
$estimatedChannels = 0
$peakGBs = 0
if ($memRows.Count -ge 1) {
  $speeds = @($memRows | ForEach-Object { $_.speed_mts } | Where-Object { $_ })
  $minSpeed = 0
  if ($speeds.Count) { $minSpeed = ($speeds | Measure-Object -Minimum).Minimum }
  $isLpddr = ($memRows[0].type -like 'LPDDR*')
  if ($isLpddr) {
    # Soldered LPDDR is usually presented as several small "modules" or one; its width is set by the platform, not the count.
    $estimatedChannels = 2
    Add-Finding 'Memory is soldered LPDDR: channel count cannot be read from the module list. Treat the peak figure below as a rough upper bound.'
  } elseif ($memRows.Count -eq 1) {
    $estimatedChannels = 1
    Add-Finding 'ONE memory module is installed, so the machine is almost certainly running SINGLE-channel memory. Token generation is limited by memory bandwidth, so this alone can halve the speed. Adding a second matching module is the cheapest hardware fix.'
  } else {
    $estimatedChannels = 2
  }
  $peakGBs = [math]::Round(($minSpeed * 8 * $estimatedChannels) / 1000, 1)
  $report.host.estimated_memory_channels = $estimatedChannels
  $report.host.estimated_peak_memory_bandwidth_gbs = $peakGBs
  Say ("   Estimated peak bandwidth: about " + $peakGBs + " GB/s (" + $estimatedChannels + " channel(s) x 64 bit x " + $minSpeed + " MT/s). Real use reaches roughly 60 to 80 percent of this.")
} else { Add-Finding 'Could not read memory modules from WMI (a virtual machine or restricted account).' }

$memArray = Try-Run { Get-CimInstance Win32_PhysicalMemoryArray | Select-Object -First 1 }
$machine = Try-Run { Get-CimInstance Win32_ComputerSystem | Select-Object -First 1 }
if ($memArray) {
  $maxGb = [math]::Round([double]$memArray.MaxCapacity / 1MB, 0)
  $report.host.memory_slots = $memArray.MemoryDevices
  $report.host.memory_max_gb = $maxGb
  Say ("   Memory slots reported: " + $memArray.MemoryDevices + ", maximum " + $maxGb + " GB")
}
if ($machine) { $report.host.machine = ($machine.Manufacturer + ' ' + $machine.Model); Say ('   Machine: ' + $report.host.machine) }

# ---------------------------------------------------------------- host: power
Say '3. Power and clocks'
$scheme = Try-Run { (powercfg /getactivescheme) -join ' ' } ''
$battery = Try-Run { Get-CimInstance Win32_Battery | Select-Object -First 1 }
$onBattery = $false
if ($battery) { $onBattery = ($battery.BatteryStatus -eq 1) }
$report.host.power_scheme = $scheme
$report.host.battery_present = [bool]$battery
$report.host.on_battery = $onBattery
Say ("   " + $scheme)
if ($onBattery) { Add-Finding 'The machine is running on BATTERY. Laptops throttle the CPU on battery; plug in before measuring.' }
if ($scheme -and ($scheme -match 'Power saver|Balanced') -and -not ($scheme -match 'High performance|Ultimate')) {
  Add-Finding 'The Windows power plan is not High or Ultimate performance. Try High performance (and the "Best performance" slider) and measure again.'
}

# ---------------------------------------------------------------- WSL and Docker
Say '4. WSL2 and Docker'
$wslConfigPath = Join-Path $env:USERPROFILE '.wslconfig'
if (Test-Path $wslConfigPath) {
  $wslConfig = Get-Content $wslConfigPath -Raw
  $report.docker.wslconfig = $wslConfig
  Say '   .wslconfig present:'
  ($wslConfig -split "`n") | ForEach-Object { Say ("     " + $_.TrimEnd()) }
} else {
  $report.docker.wslconfig = $null
  Say '   No .wslconfig: WSL2 uses its defaults (memory 50 percent of RAM, all logical processors).'
}
$dockerInfo = Try-Run { docker info --format '{{.NCPU}}|{{.MemTotal}}|{{.OperatingSystem}}' } ''
if ($dockerInfo) {
  $parts = $dockerInfo.Split('|')
  $vmCpus = [int]$parts[0]
  $vmMemGb = [math]::Round([double]$parts[1] / 1GB, 1)
  $report.docker.vm_cpus = $vmCpus
  $report.docker.vm_memory_gb = $vmMemGb
  Say ("   Docker VM: " + $vmCpus + " CPUs, " + $vmMemGb + " GB (" + $parts[2] + ")")
  if ($cpu -and $vmCpus -lt $cpu.NumberOfLogicalProcessors) {
    Add-Finding ('Docker VM is given ' + $vmCpus + ' of ' + $cpu.NumberOfLogicalProcessors + ' logical processors. Raise processors= in .wslconfig if the model needs more threads.')
  }
  if ($totalGb -gt 0 -and $vmMemGb -lt ($totalGb * 0.9)) {
    Say ("   Docker's VM sees " + $vmMemGb + " GB of the host's " + $totalGb + " GB (normal; the model, database and API share it).")
  }
} else { Add-Finding 'Docker did not answer to docker info. Is Docker Desktop running? The container checks below were skipped.' }

# ---------------------------------------------------------------- the LocalAI container
$container = $ContainerName
if (-not $container -and $dockerInfo) {
  $rows = Try-Run { docker ps --format '{{.Names}}|{{.Image}}|{{.Ports}}' } @()
  # The LocalAI container is the one publishing the API port; fall back to an image name containing "localai".
  $portPattern = ':' + ([uri]$BaseUri).Port + '->'
  foreach ($row in @($rows)) { if (-not $container -and $row.Split('|').Count -ge 3 -and $row.Split('|')[2].Contains($portPattern)) { $container = $row.Split('|')[0] } }
  foreach ($row in @($rows)) { if (-not $container -and $row -match 'localai') { $container = $row.Split('|')[0] } }
}
$report.docker.container = $container
if ($container) {
  Say ("5. Container " + $container)
  $limits = Try-Run { docker inspect $container --format '{{.HostConfig.NanoCpus}}|{{.HostConfig.CpuQuota}}|{{.HostConfig.CpuPeriod}}|{{.HostConfig.Memory}}|{{.HostConfig.CpusetCpus}}' } ''
  if ($limits) {
    $l = $limits.Split('|')
    $report.docker.limits = [ordered]@{ nano_cpus = $l[0]; cpu_quota = $l[1]; cpu_period = $l[2]; memory_bytes = $l[3]; cpuset = $l[4] }
    Say ("   limits: nano_cpus=" + $l[0] + " cpu_quota=" + $l[1] + " memory_bytes=" + $l[3] + " cpuset='" + $l[4] + "'")
    if ($l[0] -ne '0' -or ($l[1] -ne '0' -and $l[1] -ne '-1')) { Add-Finding 'The container has a CPU limit set (nano_cpus or cpu_quota). That caps token speed regardless of the host. Remove or raise it.' }
    if ($l[3] -ne '0') { Add-Finding ("The container has a memory limit of " + [math]::Round([double]$l[3] / 1GB, 1) + " GB.") }
  }
  $flags = Try-Run { docker exec $container sh -c "grep -m1 '^flags' /proc/cpuinfo" } ''
  $nproc = Try-Run { docker exec $container nproc } ''
  $cpuMax = Try-Run { docker exec $container sh -c "cat /sys/fs/cgroup/cpu.max 2>/dev/null" } ''
  $memMax = Try-Run { docker exec $container sh -c "cat /sys/fs/cgroup/memory.max 2>/dev/null" } ''
  $wanted = @('avx', 'avx2', 'fma', 'f16c', 'avx512f', 'avx512_vnni', 'avx_vnni', 'amx_tile', 'amx_int8')
  $present = @()
  foreach ($flag in $wanted) { if ($flags -match ('\b' + [regex]::Escape($flag) + '\b')) { $present += $flag } }
  $report.docker.container_cpu = [ordered]@{ nproc = $nproc; cgroup_cpu_max = $cpuMax; cgroup_memory_max = $memMax; isa_flags = $present }
  Say ("   nproc=" + $nproc + "  cgroup cpu.max=" + $cpuMax + "  memory.max=" + $memMax)
  Say ("   instruction sets seen: " + ($present -join ', '))
  if ($present -notcontains 'avx2') { Add-Finding 'AVX2 is NOT visible inside the container. llama.cpp falls back to very slow code. Check the CPU and the virtualization settings.' }
  elseif ($present -notcontains 'avx512f') { Say '   (No AVX-512: prompt reading is slower than on AVX-512 CPUs; token generation is unaffected.)' }
  if ($cpuMax -and $cpuMax -notmatch '^max') { Add-Finding ("The container's cgroup cpu.max is '" + $cpuMax + "', which limits CPU time.") }

  $procs = Try-Run { docker exec $container sh -c "ps -eo args | grep -i -E 'llama|ggml|grpc' | grep -v grep | head -5" } ''
  $report.docker.llama_processes = $procs
  Say '   model backend processes:'
  ($procs -split "`n") | ForEach-Object { if ($_) { Say ("     " + $_.Trim()) } }
  $threadHints = Try-Run { docker exec $container sh -c "grep -r -i -E '^\s*threads\s*:|f16:|mmap:|mlock:|context_size:|batch:' /models/*.yaml /build/models/*.yaml 2>/dev/null | head -20" } ''
  $report.docker.model_config_hints = $threadHints
  Say '   model config hints (threads, mmap, context):'
  ($threadHints -split "`n") | ForEach-Object { if ($_) { Say ("     " + $_.Trim()) } }
  if (-not ($threadHints -match 'threads')) { Say '     (no explicit threads: setting found; llama.cpp then picks its own default)' }

  if ($ModelSizeGB -le 0) {
    $sizeBytes = Try-Run { docker exec $container sh -c "find /models /build/models -iname '*q4*.gguf' -printf '%s %p\n' 2>/dev/null | sort -rn | head -1" } ''
    if ($sizeBytes -match '^(\d+)\s') { $ModelSizeGB = [math]::Round([double]$Matches[1] / 1GB, 2) }
  }
  $stats = Try-Run { docker stats $container --no-stream --format '{{.CPUPerc}}|{{.MemUsage}}' } ''
  $report.docker.idle_stats = $stats
  Say ("   idle usage: " + $stats + "   (model size assumed " + $ModelSizeGB + " GB)")
} else { Say '5. No LocalAI container found; pass -ContainerName. Timing will still run if the API answers.' }

if ($ModelSizeGB -le 0) { $ModelSizeGB = 2.5 }
$report.docker.model_size_gb = $ModelSizeGB

# ---------------------------------------------------------------- timing through the API
Say '6. Timing the real model (prefill against decode)'
$headers = @{}
if ($ApiKey) { $headers['Authorization'] = 'Bearer ' + $ApiKey }
$chatUri = $BaseUri.TrimEnd('/') + '/v1/chat/completions'

function New-Prompt([int]$Sentences) {
  $nonce = [guid]::NewGuid().ToString('N')
  $sb = New-Object System.Text.StringBuilder
  [void]$sb.Append('Reference ' + $nonce + '. ')
  for ($i = 0; $i -lt $Sentences; $i++) { [void]$sb.Append('Record ' + $i + ' notes that subscriber group ' + ($i % 17) + ' placed calls at hour ' + ($i % 24) + ' from tower ' + ($i % 31) + '. ') }
  [void]$sb.Append('Reply with a short plain sentence summarising the first record.')
  return $sb.ToString()
}

function Invoke-Chat([string]$Prompt, [int]$MaxTokens) {
  $payload = @{ model = $Model; messages = @(@{ role = 'user'; content = $Prompt }); max_tokens = $MaxTokens; temperature = 0; stream = $false } | ConvertTo-Json -Depth 6 -Compress
  $watch = [Diagnostics.Stopwatch]::StartNew()
  try {
    $response = Invoke-RestMethod -Uri $chatUri -Method Post -ContentType 'application/json' -Headers $headers -Body $payload -TimeoutSec 900
    $watch.Stop()
    return [ordered]@{ ok = $true; seconds = $watch.Elapsed.TotalSeconds; prompt_tokens = [int]$response.usage.prompt_tokens; completion_tokens = [int]$response.usage.completion_tokens }
  } catch {
    $watch.Stop()
    return [ordered]@{ ok = $false; seconds = $watch.Elapsed.TotalSeconds; error = $_.Exception.Message }
  }
}

function Get-Median($Values) {
  $sorted = @($Values | Sort-Object)
  if (-not $sorted.Count) { return 0 }
  return $sorted[[math]::Floor(($sorted.Count - 1) / 2)]
}

# A warm-up request so a cold model load is not counted.
$warm = Invoke-Chat (New-Prompt 4) 4
if (-not $warm.ok) {
  Add-Finding ("The model did not answer at " + $chatUri + " (" + $warm.error + "). Check -BaseUri, -Model and -ApiKey. Timing was skipped.")
} else {
  $genTokens = 48
  $sizes = @(@{ label = 'short prompt'; sentences = 6 })
  $reps = 3
  if ($Quick) { $reps = 2 } else { $sizes += @{ label = 'long prompt'; sentences = 140 } }
  $timingRows = @()
  foreach ($size in $sizes) {
    $prefill = @(); $decode = @(); $promptTokens = 0
    for ($r = 0; $r -lt $reps; $r++) {
      $first = Invoke-Chat (New-Prompt $size.sentences) 1
      $more = Invoke-Chat (New-Prompt $size.sentences) $genTokens
      if ($first.ok -and $more.ok) {
        $promptTokens = $first.prompt_tokens
        $prefill += $first.seconds
        $extraTokens = $more.completion_tokens - $first.completion_tokens
        $extraSeconds = $more.seconds - $first.seconds
        if ($extraTokens -gt 4 -and $extraSeconds -gt 0.05) { $decode += ($extraTokens / $extraSeconds) }
      }
    }
    $prefillSec = Get-Median $prefill
    $decodeTps = [math]::Round((Get-Median $decode), 2)
    $prefillTps = 0
    if ($prefillSec -gt 0) { $prefillTps = [math]::Round($promptTokens / $prefillSec, 1) }
    $timingRows += [ordered]@{ label = $size.label; prompt_tokens = $promptTokens; time_to_first_token_s = [math]::Round($prefillSec, 2); prefill_tokens_per_s = $prefillTps; decode_tokens_per_s = $decodeTps }
    Say ("   " + $size.label + ": " + $promptTokens + " prompt tokens, first token after " + [math]::Round($prefillSec, 2) + " s (" + $prefillTps + " tok/s reading), generation " + $decodeTps + " tok/s")
  }
  $report.timing.rows = $timingRows
  $cpuNow = Try-Run { (Get-CimInstance Win32_Processor | Select-Object -First 1).CurrentClockSpeed } 0
  $report.timing.cpu_clock_mhz_after = $cpuNow
  $bestDecode = ($timingRows | ForEach-Object { $_.decode_tokens_per_s } | Measure-Object -Maximum).Maximum
  if ($bestDecode -gt 0) {
    $effective = [math]::Round($bestDecode * $ModelSizeGB, 1)
    $report.timing.effective_bandwidth_gbs = $effective
    Say ("   Effective memory bandwidth used by generation: about " + $effective + " GB/s (decode tokens/s x model size).")
    if ($peakGBs -gt 0) {
      $ratio = [math]::Round($effective / $peakGBs, 2)
      $report.timing.fraction_of_estimated_peak = $ratio
      Say ("   That is " + [int]($ratio * 100) + " percent of the estimated peak (" + $peakGBs + " GB/s).")
      if ($ratio -lt 0.35) { Add-Finding 'Generation uses far less memory bandwidth than the memory can deliver. Suspect too few threads, a container CPU limit, throttling, or an unfavourable thread count (try the number of PHYSICAL cores, not logical) before blaming the model.' }
      elseif ($ratio -ge 0.55) { Add-Finding 'Generation already uses most of the memory bandwidth. A smaller or more compressed model, or faster / wider memory (dual channel, DDR5), is what would speed it up; thread tuning will not.' }
    }
  }
  $long = $timingRows | Where-Object { $_.label -eq 'long prompt' } | Select-Object -First 1
  if ($long -and $long.time_to_first_token_s -gt 10) { Add-Finding ("A " + $long.prompt_tokens + "-token prompt takes " + $long.time_to_first_token_s + " s before the first word. Prompt reading dominates long questions: shorter prompts, prompt caching and a repack-friendly quantisation (Q4_0) are the levers.") }
}

# ---------------------------------------------------------------- write the report
$report.findings = @($findings)
$report | ConvertTo-Json -Depth 8 | Set-Content -Path (Join-Path $outDir 'report.json') -Encoding UTF8

$md = New-Object System.Collections.Generic.List[string]
$md.Add('# M0 CPU inference diagnostic')
$md.Add('')
$md.Add('Generated ' + $report.generated_at + '. Read-only: nothing was changed, downloaded or deleted.')
$md.Add('')
$md.Add('## Findings')
if ($findings.Count) { foreach ($f in $findings) { $md.Add('- ' + $f) } } else { $md.Add('- None of the checks flagged a problem. Read the numbers below.') }
$md.Add('')
$md.Add('## Numbers')
if ($report.host.cpu) { $md.Add('- CPU: ' + $report.host.cpu.name + ', ' + $report.host.cpu.physical_cores + ' cores / ' + $report.host.cpu.logical_processors + ' threads, max ' + $report.host.cpu.max_clock_mhz + ' MHz') }
$md.Add('- Memory modules: ' + $memRows.Count + ', ' + $totalGb + ' GB, estimated ' + $estimatedChannels + ' channel(s), peak about ' + $peakGBs + ' GB/s')
$md.Add('- Power: ' + $scheme + ', on battery: ' + $onBattery)
if ($report.docker.vm_cpus) { $md.Add('- Docker VM: ' + $report.docker.vm_cpus + ' CPUs, ' + $report.docker.vm_memory_gb + ' GB') }
if ($report.timing.rows) { foreach ($row in $report.timing.rows) { $md.Add('- ' + $row.label + ': ' + $row.prompt_tokens + ' prompt tokens, first token ' + $row.time_to_first_token_s + ' s, reading ' + $row.prefill_tokens_per_s + ' tok/s, generation ' + $row.decode_tokens_per_s + ' tok/s') } }
if ($report.timing.effective_bandwidth_gbs) { $md.Add('- Effective bandwidth in generation: about ' + $report.timing.effective_bandwidth_gbs + ' GB/s') }
$md | Set-Content -Path (Join-Path $outDir 'report.md') -Encoding UTF8

Say ''
Say ('Done. Report: ' + (Join-Path $outDir 'report.md'))
if ($findings.Count) { Say ('Findings (' + $findings.Count + '):'); foreach ($f in $findings) { Say ('  * ' + $f) } }
