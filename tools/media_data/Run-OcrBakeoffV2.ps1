# OCR bake-off v2, sealed runs (docs/data/ocr-bakeoff-v2-preregistration-20261117.md). One container at a time, memory-capped, no network, with a watchdog that kills ONLY the test container
# when host available memory falls below 600 MB. Each sealed run is made once: a failed run is reported, never repeated automatically.
#
#   powershell -NoProfile -File tools\media_data\Run-OcrBakeoffV2.ps1                 (all runs)
#   powershell -NoProfile -File tools\media_data\Run-OcrBakeoffV2.ps1 -Only unb-E       (named runs; a value without -Only used to be taken as the data folder)
#
# Results: JSON files under <data>\results\ocr (one per set and reader), logs under C:\tmp\nexusai-g4\ocr_v2_*.log.
param([string]$Data = 'C:/tmp/nexusai-media-data', [string]$Repo = 'C:/Users/sheik/Workspace/Office/Projects/NexusAI-localwork', [string]$Logs = 'C:/tmp/nexusai-g4', [string[]]$Only = @())
$docker = "C:\Users\sheik\AppData\Local\Programs\DockerDesktop\resources\bin\docker.exe"
$base = 'nexusai-media-eval:latest'
$easy = 'nexusai-media-eval-easyocr:20261117'
function Avail { [int]((Get-Counter '\Memory\Available MBytes').CounterSamples[0].CookedValue) }

# name, image, memory (MB), python arguments
$runs = @(
  @('mixed-T',        $base, 2000, '/repo/tools/media_data/ocr_eval.py --set mixed --engine tess:best:urd+eng:3 --partition sealed'),
  @('mixed-P',        $base, 2000, '/repo/tools/media_data/ocr_eval.py --set mixed --engine onnx:latin:page --partition sealed'),
  @('mixed-E',        $easy, 3000, '/repo/tools/media_data/ocr_eval.py --set mixed --engine easyocr:ur+en --partition sealed'),
  @('urlines-naskh-T',    $base, 2000, '/repo/tools/media_data/ocr_eval.py --set ur_lines_naskh --engine tess:best:urd+eng:7 --partition sealed'),
  @('urlines-naskh-E',    $easy, 3000, '/repo/tools/media_data/ocr_eval.py --set ur_lines_naskh --engine easyocr:ur+en --partition sealed'),
  @('urlines-nastaliq-T', $base, 2000, '/repo/tools/media_data/ocr_eval.py --set ur_lines_nastaliq --engine tess:best:urd+eng:7 --partition sealed'),
  @('urlines-nastaliq-E', $easy, 3000, '/repo/tools/media_data/ocr_eval.py --set ur_lines_nastaliq --engine easyocr:ur+en --partition sealed'),
  @('urpages-T',      $base, 2000, '/repo/tools/media_data/ocr_eval.py --set ur_pages --engine tess:best:urd+eng:3 --partition sealed'),
  @('urpages-E',      $easy, 3000, '/repo/tools/media_data/ocr_eval.py --set ur_pages --engine easyocr:ur+en --partition sealed'),
  @('enpages-P',      $base, 2000, '/repo/tools/media_data/ocr_eval.py --set en_pages --engine onnx:latin:page --partition sealed'),
  @('enpages-E',      $easy, 3000, '/repo/tools/media_data/ocr_eval.py --set en_pages --engine easyocr:ur+en --partition sealed'),
  @('enpages-Een',    $easy, 3000, '/repo/tools/media_data/ocr_eval.py --set en_pages --engine easyocr:en --partition sealed'),
  @('cord-P',         $base, 2000, '/repo/tools/media_data/ocr_eval.py --set cord --engine onnx:latin:page --partition sealed'),
  @('cord-Een',       $easy, 3000, '/repo/tools/media_data/ocr_eval.py --set cord --engine easyocr:en --partition sealed'),
  @('unb-E',          $easy, 3000, '/repo/tools/media_data/nastaliq_lab.py --reader easyocr --part sealed --out /out/e-sealed.jsonl')
)
foreach ($r in $runs) {
  $name, $image, $mem, $pyargs = $r
  if ($Only.Count -gt 0 -and ($Only -notcontains $name)) { continue }
  $cname = "nxocr-$name-" + (Get-Date -Format 'HHmmss')
  "[{0}] {1}: start (host available {2} MB)" -f (Get-Date -Format 'HH:mm:ss'), $name, (Avail)
  $volumes = @('-v', "${Data}:/data", '-v', "${Repo}:/repo:ro", '-v', "${Data}/results/nastaliq:/out")
  & $docker run -d --name $cname --network none --memory "${mem}m" --memory-swap "${mem}m" --entrypoint python @volumes $image @($pyargs -split ' ') 2>&1 | Out-Null
  if ($LASTEXITCODE -ne 0) { "[{0}] {1}: the container did not start (docker exit {2}); nothing was read" -f (Get-Date -Format 'HH:mm:ss'), $name, $LASTEXITCODE; continue }
  $reason = ''
  while ($true) {
    Start-Sleep -Seconds 20
    $running = (& $docker inspect -f '{{.State.Running}}' $cname 2>&1)
    if ("$running" -ne 'true') { break }
    if ((Avail) -lt 600) { $reason = 'host available memory below 600 MB'; break }
  }
  if ($reason) { "[{0}] {1}: WATCHDOG {2}: killing only the test container" -f (Get-Date -Format 'HH:mm:ss'), $name, $reason; & $docker kill $cname 2>&1 | Out-Null }
  $state = (& $docker inspect -f 'exit={{.State.ExitCode}} oom_killed={{.State.OOMKilled}}' $cname 2>&1)
  & $docker logs $cname 2>&1 | Where-Object { $_ -notmatch 'device_discovery|Warning|warn' } | Out-File -Encoding utf8 "$Logs/ocr_v2_${name}_$(Get-Date -Format 'yyyyMMdd_HHmmss').log"
  & $docker rm -f $cname 2>&1 | Out-Null
  "[{0}] {1}: finished, {2}" -f (Get-Date -Format 'HH:mm:ss'), $name, $state
}
"[{0}] all runs finished" -f (Get-Date -Format 'HH:mm:ss')
