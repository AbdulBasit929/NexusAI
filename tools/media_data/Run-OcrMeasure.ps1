# Router threshold, speed and acceptance runs of the routed EasyOCR role (docs/roadmap/ocr-easyocr-integration-plan-20261117.md and docs/data/ocr-easyocr-acceptance-preregistration-20261117.md).
# One container at a time, memory-capped, no network, with a watchdog that kills ONLY the test container when host available memory falls below 600 MB. Results go under <data>\results\ocr, logs under C:\tmp\nexusai-g4.
#
#   powershell -NoProfile -File tools\media_data\Run-OcrMeasure.ps1 -Only router-share        (development rows only)
#   powershell -NoProfile -File tools\media_data\Run-OcrMeasure.ps1 -Only easyocr-speed       (development rows only)
#   powershell -NoProfile -File tools\media_data\Run-OcrMeasure.ps1 -Only accept-prepare      (makes the fresh sets; reads no image)
#   powershell -NoProfile -File tools\media_data\Run-OcrMeasure.ps1 -Only accept-today,accept-easyocr -Image <worker overlay tag>   (fresh data through the worker's router)
param([string]$Data = 'C:/tmp/nexusai-media-data', [string]$Repo = 'C:/Users/sheik/Workspace/Office/Projects/NexusAI-localwork', [string]$Logs = 'C:/tmp/nexusai-g4', [string[]]$Only = @(),
      [string]$Image = 'nexusai-forensic-records-worker:easyocr-20261117', [string]$Threads = '4', [string]$MaxSide = '1400', [string]$UrduShare = '0.10')
$docker = "C:\Users\sheik\AppData\Local\Programs\DockerDesktop\resources\bin\docker.exe"
$base = 'nexusai-media-eval:latest'
$easy = 'nexusai-media-eval-easyocr:20261117'
$Only = @($Only | ForEach-Object { $_ -split ',' } | Where-Object { $_ })   # powershell -File passes -Only a,b as one string
function Avail { [int]((Get-Counter '\Memory\Available MBytes').CounterSamples[0].CookedValue) }

# The worker's own OCR environment (names and values, no secrets among them), read from the running container so the acceptance uses what the worker uses.
function Worker-OcrEnv {
  $raw = & $docker inspect nexusai-forensic-records-worker-1 --format '{{range .Config.Env}}{{println .}}{{end}}'
  $keep = $raw | Where-Object { $_ -match '^(FORENSIC_OCR_|FORENSIC_ONNX_OCR_)' }
  $keep = @($keep) + @('FORENSIC_EASYOCR_MODEL_DIR=/models/media/easyocr/model', "FORENSIC_EASYOCR_THREADS=$Threads", "FORENSIC_EASYOCR_MAX_SIDE=$MaxSide", "FORENSIC_OCR_ROUTER_URDU_SHARE=$UrduShare",
    'FORENSIC_EASYOCR_DETECTOR_SHA256=4a5efbfb48b4081100544e75e1e2b57f8de3d84f213004b14b85fd4b3748db17', 'FORENSIC_EASYOCR_RECOGNIZER_SHA256=2a9afd42c374deb98aed0b53c9b77d75e1d00d4e0501f3b0276c54190c89b1a8')
  $file = Join-Path $Logs 'worker-ocr.env'
  Set-Content -LiteralPath $file -Value $keep -Encoding ascii
  return $file
}

$py = '/repo/tools/media_data'
# name, image, memory (MB), extra docker arguments, entrypoint python arguments
$runs = @(
  @('router-share',    $base, 2000, @(), "$py/measure_router_share.py --out /data/results/ocr/router-share-dev-20261117.json"),
  @('easyocr-speed',   $easy, 3000, @(), "$py/measure_easyocr_speed.py --driver --threads 1 2 4 8 --caps 2400 1800 1400 --cap-threads 4 --out /data/results/ocr/easyocr-speed-dev-20261117.json"),
  @('easyocr-large',   $easy, 3500, @(), "$py/measure_easyocr_large.py --driver --caps 0 1800 --threads 4 --out /data/results/ocr/easyocr-large-dev-20261117.json"),
  @('latin-reread',    $easy, 3500, @(), "$py/measure_latin_reread.py --out /data/results/ocr/latin-reread-dev-20261117.json"),
  @('easyocr-smoke',   $Image, 3500, @('ENV'), "$py/smoke_easyocr_role.py"),   # use -MaxSide 0 so the parity check against the lab reader compares like with like
  @('accept-prepare',  $base, 2000, @(), "$py/accept_router_prepare.py --news 30"),
  @('accept2-prepare', $base, 2000, @(), "$py/accept_router_prepare.py --news 0 --suffix _accept2 --seeds 20261204 20261205 20261206"),
  @('accept2-today',   $Image, 3500, @('ENV'), "$py/accept_router_runner.py --arm today --suffix _accept2 --sets ur_pages mixed --out /data/results/ocr/accept2-today.jsonl"),
  @('accept2-easyocr', $Image, 3500, @('ENV'), "$py/accept_router_runner.py --arm easyocr --suffix _accept2 --sets ur_pages mixed --out /data/results/ocr/accept2-easyocr.jsonl"),
  @('accept2-reread',  $Image, 3500, @('ENV'), "$py/accept_router_runner.py --arm reread --suffix _accept2 --sets ur_pages mixed --out /data/results/ocr/accept2-reread.jsonl"),
  @('accept-today',    $Image, 3500, @('ENV'), "$py/accept_router_runner.py --arm today --out /data/results/ocr/accept-today.jsonl"),
  @('accept-easyocr',  $Image, 3500, @('ENV'), "$py/accept_router_runner.py --arm easyocr --out /data/results/ocr/accept-easyocr.jsonl")
)
foreach ($r in $runs) {
  $name, $img, $mem, $extra, $pyargs = $r
  if ($Only -notcontains $name) { continue }   # a run is always named: nothing starts by accident
  $cname = "nxmeasure-$name-" + (Get-Date -Format 'HHmmss')
  "[{0}] {1}: start (host available {2} MB)" -f (Get-Date -Format 'HH:mm:ss'), $name, (Avail)
  $volumes = @('-v', "${Data}:/data", '-v', "${Repo}:/repo:ro")
  $args2 = @()
  # the worker image runs as an unprivileged user that cannot write into the bind-mounted results folder; these are throw-away test containers without network, so they run as root
  if ($extra -contains 'ENV') { $args2 += @('--user', '0:0', '--env-file', (Worker-OcrEnv), '-v', "${Repo}/models/media:/models/media:ro") }
  & $docker run -d --name $cname --network none --memory "${mem}m" --memory-swap "${mem}m" --entrypoint python @volumes @args2 $img @($pyargs -split ' ') 2>&1 | Out-Null
  if ($LASTEXITCODE -ne 0) { "[{0}] {1}: the container did not start (docker exit {2}); nothing was read" -f (Get-Date -Format 'HH:mm:ss'), $name, $LASTEXITCODE; continue }
  $reason = ''
  while ($true) {
    Start-Sleep -Seconds 15
    $running = (& $docker inspect -f '{{.State.Running}}' $cname 2>&1)
    if ("$running" -ne 'true') { break }
    if ((Avail) -lt 600) { $reason = 'host available memory below 600 MB'; break }
  }
  if ($reason) { "[{0}] {1}: WATCHDOG {2}: killing only the test container" -f (Get-Date -Format 'HH:mm:ss'), $name, $reason; & $docker kill $cname 2>&1 | Out-Null }
  $state = (& $docker inspect -f 'exit={{.State.ExitCode}} oom_killed={{.State.OOMKilled}}' $cname 2>&1)
  & $docker logs $cname 2>&1 | Where-Object { $_ -notmatch 'device_discovery|Warning|warn' } | Out-File -Encoding utf8 "$Logs/measure_${name}_$(Get-Date -Format 'yyyyMMdd_HHmmss').log"
  & $docker rm -f $cname 2>&1 | Out-Null
  "[{0}] {1}: finished, {2}" -f (Get-Date -Format 'HH:mm:ss'), $name, $state
}
"[{0}] done" -f (Get-Date -Format 'HH:mm:ss')
