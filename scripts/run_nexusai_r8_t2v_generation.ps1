param(
  [string]$OutputRoot = 'C:\NexusAI-Evaluation\R8\T2V\1.0.0',
  [switch]$PreflightOnly
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$image = 'nexusai/r8-ocr-eval:3.7.0-cpu'
$contract = Join-Path $repoRoot 'configuration\nexusai_r8_t2v_contract.json'
$generator = Join-Path $repoRoot 'tools\r8_t2v\generate_t2v.py'
$validator = Join-Path $repoRoot 'scripts\validate_nexusai_r8_t2v_pack.py'
$physicalRoot = [System.IO.Path]::GetFullPath('C:\NexusAI-Evaluation\R8\T2\1.0.0')
$resolvedOutput = [System.IO.Path]::GetFullPath($OutputRoot)

if ($resolvedOutput.TrimEnd('\') -eq $physicalRoot.TrimEnd('\') -or $resolvedOutput.StartsWith($physicalRoot.TrimEnd('\') + '\', [System.StringComparison]::OrdinalIgnoreCase)) {
  throw 'T2-V output must not be written into the physical T2-P workspace'
}
if (-not (Test-Path $contract) -or -not (Test-Path $generator) -or -not (Test-Path $validator)) {
  throw 'Required T2-V source files are missing'
}
$dockerVersion = & docker info --format '{{.ServerVersion}}' 2>$null
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($dockerVersion)) {
  throw 'Docker Desktop Linux engine is not ready'
}
& docker image inspect $image *> $null
if ($LASTEXITCODE -ne 0) { throw "Required existing isolated evaluator image is missing: $image" }
$driveName = (Split-Path $resolvedOutput -Qualifier).TrimEnd(':')
$freeDiskGiB = [math]::Round((Get-PSDrive -Name $driveName).Free / 1GB, 2)
if ($freeDiskGiB -lt 2) { throw "T2-V generation requires at least 2 GiB free disk; available=$freeDiskGiB" }
if (Test-Path $resolvedOutput) {
  $existing = @(Get-ChildItem -LiteralPath $resolvedOutput -Force -ErrorAction Stop)
  if ($existing.Count -gt 0) { throw "OutputRoot already contains files; preserve it and choose a new version path: $resolvedOutput" }
}
Write-Host "R8T2VPreflight=PASS Docker=$dockerVersion Image=$image FreeDiskGiB=$freeDiskGiB Output=$resolvedOutput ProductionMutation=false"
if ($PreflightOnly) { return }

New-Item -ItemType Directory -Force -Path $resolvedOutput | Out-Null
$repoMount = $repoRoot.Replace('\', '/')
$outputMount = $resolvedOutput.Replace('\', '/')
& docker run --rm --network none `
  --mount "type=bind,source=$repoMount,target=/repo,readonly" `
  --mount "type=bind,source=$outputMount,target=/output" `
  --entrypoint python $image `
  /repo/tools/r8_t2v/generate_t2v.py `
  --contract /repo/configuration/nexusai_r8_t2v_contract.json `
  --output /output
if ($LASTEXITCODE -ne 0) { throw 'Isolated T2-V generator failed' }

& python $validator --pack-root $resolvedOutput --contract $contract --output (Join-Path $resolvedOutput 'validation.json')
if ($LASTEXITCODE -ne 0) { throw 'T2-V validation failed' }
$manifestHash = (Get-FileHash -Algorithm SHA256 (Join-Path $resolvedOutput 'manifest.json')).Hash.ToLowerInvariant()
Write-Host "R8T2VGeneration=PASS Fixtures=96 Development=56 Validation=20 SealedHoldout=20 ManifestSHA256=$manifestHash ProductionMutation=false"
