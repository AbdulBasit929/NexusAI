# Face and image-search bake-off runs (docs/data/face-image-search-preregistration-20261119.md). One container at a time, memory-capped, no network, with a watchdog that kills ONLY the test container
# when host available memory falls below 600 MB (the same pattern as Run-OcrMeasure.ps1). Results go under <data>\work-faces\results, logs under C:\tmp\nexusai-g4.
#
#   powershell -NoProfile -File tools\media_data\Run-FaceMeasure.ps1 -Only recog-lfw
#   powershell -NoProfile -File tools\media_data\Run-FaceMeasure.ps1 -Only detect-yunet-dev
#   powershell -NoProfile -File tools\media_data\Run-FaceMeasure.ps1 -Only detect-yunet-sealed -Extra '--threshold 0.55'      (a sealed run needs the value chosen on dev)
# A run is always named: nothing starts by accident. Do not run docker, compose or git commands while a timed run is going.
param([string]$Data = 'C:/tmp/nexusai-media-data', [string]$Repo = 'C:/Users/sheik/Workspace/Office/Projects/NexusAI-localwork', [string]$Logs = 'C:/tmp/nexusai-g4',
      [string[]]$Only = @(), [string]$Extra = '', [string]$Threads = '8', [string]$Image = 'nexusai-media-eval-face:20261119', [int]$Retries = 5, [int]$StartAvail = 1200, [string]$YunetThr = '0.8', [string]$BlazeThr = '0.6')   # the thresholds chosen on the WIDER development half (wider-*-dev-report.json)
$docker = "C:\Users\sheik\AppData\Local\Programs\DockerDesktop\resources\bin\docker.exe"
$Only = @($Only | ForEach-Object { $_ -split ',' } | Where-Object { $_ })
function Avail { [int]((Get-Counter '\Memory\Available MBytes').CounterSamples[0].CookedValue) }
$py = '/repo/tools/media_data'
# name, memory (MB), python arguments
$runs = @(
  @('prepare-lfw-orig', 2000, "$py/face_prepare.py lfw-orig"),
  @('recog-lfw',        2000, "$py/face_recog_bins.py --models sface auraface --sets lfw"),
  @('recog-lfw-flip',   2000, "$py/face_recog_bins.py --models sface auraface --sets lfw --flip"),
  @('recog-cplfw-flip', 2000, "$py/face_recog_bins.py --models auraface --sets cplfw --flip"),
  @('recog-rest',       2000, "$py/face_recog_bins.py --models sface auraface --sets cfp_fp agedb_30 calfw cplfw"),
  @('detect-yunet-dev',    2500, "$py/face_detect_wider.py --detector yunet --part dev"),
  @('detect-blaze-dev',    2500, "$py/face_detect_wider.py --detector blazeface --part dev"),
  @('detect-yunet-sealed', 2500, "$py/face_detect_wider.py --detector yunet --part sealed --threshold $YunetThr"),
  @('detect-blaze-sealed', 2500, "$py/face_detect_wider.py --detector blazeface --part sealed --threshold $BlazeThr"),
  @('fairface',         3000, "$py/face_fairface.py"),
  @('fairface-score',   3000, "$py/face_fairface_score.py --yunet-threshold $YunetThr --blaze-threshold $BlazeThr"),
  @('lfw-e2e',          3000, "$py/face_lfw_e2e.py --models sface auraface"),
  @('lfw-e2e-centre',   3000, "$py/face_lfw_e2e.py --select centre"),
  @('ofiq-run',         2500, "$py/face_ofiq.py run", '/opt/ofiq/bin/python'),
  @('retr-translate',   2500, "$py/image_text_retrieval.py translate"),
  @('retr-embed',       3800, "$py/image_text_retrieval.py embed --models siglip siglip2"),
  @('retr-score',       1500, "$py/image_text_retrieval.py score --models siglip siglip2"),
  @('neardup',          3800, "$py/image_eval.py --methods siglip,siglip2,dinov2b --photos 200", 'python', @('-v', "${Data}/work-faces/imgres:/data/results/image")),
  @('neardup-hash',     1500, "$py/image_eval.py --methods dhash64,pdq --photos 200", 'python', @('-v', "${Data}/work-faces/imgres:/data/results/image")),
  @('timing',           2000, "$py/face_timing.py")
)
foreach ($r in $runs) {
  $name, $mem, $pyargs, $entry, $dockerExtra = $r
  if (-not $dockerExtra) { $dockerExtra = @() }
  if (-not $entry) { $entry = 'python' }
  if ($Only -notcontains $name) { continue }
  # the long runs (recog-*, fairface) write checkpoints, so after a watchdog kill the same run is started again once the host has memory again (at most $Retries times)
  for ($attempt = 1; $attempt -le ($Retries + 1); $attempt++) {
    $wait0 = 0
    while ((Avail) -lt $StartAvail -and $wait0 -lt 7200) { Start-Sleep -Seconds 20; $wait0 += 20 }   # start only when the host has headroom: other work on this machine moves host memory by 1 GB within seconds
    $cname = "nxface-$name-" + (Get-Date -Format 'HHmmss')
    "[{0}] {1}: start, attempt {2} (host available {3} MB)" -f (Get-Date -Format 'HH:mm:ss'), $name, $attempt, (Avail)
    $volumes = @('-v', "${Data}:/data:ro", '-v', "${Data}/work-faces:/work", '-v', "${Repo}:/repo:ro", '-v', "${Repo}/models/media:/models/media:ro") + $dockerExtra
    $argv = @($pyargs -split ' ') + @($Extra -split ' ' | Where-Object { $_ })
    & $docker run -d --name $cname --network none --memory "${mem}m" --memory-swap "${mem}m" --entrypoint $entry -e "FACE_THREADS=$Threads" @volumes $Image @argv 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { "[{0}] {1}: the container did not start (docker exit {2}); nothing was read" -f (Get-Date -Format 'HH:mm:ss'), $name, $LASTEXITCODE; break }
    $reason = ''
    while ($true) {
      Start-Sleep -Seconds 15
      $running = (& $docker inspect -f '{{.State.Running}}' $cname 2>&1)
      if ("$running" -ne 'true') { break }
      if ((Avail) -lt 600) { $reason = 'host available memory below 600 MB'; break }
    }
    if ($reason) { "[{0}] {1}: WATCHDOG {2}: killing only the test container" -f (Get-Date -Format 'HH:mm:ss'), $name, $reason; & $docker kill $cname 2>&1 | Out-Null }
    $state = (& $docker inspect -f 'exit={{.State.ExitCode}} oom_killed={{.State.OOMKilled}}' $cname 2>&1)
    & $docker logs $cname 2>&1 | Where-Object { $_ -notmatch 'device_discovery|VerifyOutputSizes|Expected shape|WARN:0' } | Out-File -Encoding utf8 "$Logs/face_${name}_$(Get-Date -Format 'yyyyMMdd_HHmmss').log"
    & $docker rm -f $cname 2>&1 | Out-Null
    "[{0}] {1}: finished, {2}" -f (Get-Date -Format 'HH:mm:ss'), $name, $state
    if (-not $reason) { break }
    $wait = 0
    while ((Avail) -lt $StartAvail -and $wait -lt 7200) { Start-Sleep -Seconds 20; $wait += 20 }
    "[{0}] {1}: waited {2} s for memory (available {3} MB)" -f (Get-Date -Format 'HH:mm:ss'), $name, $wait, (Avail)
  }
}
"[{0}] done" -f (Get-Date -Format 'HH:mm:ss')
