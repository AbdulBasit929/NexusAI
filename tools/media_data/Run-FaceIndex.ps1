# pgvector HNSW test (gates I1, I2 of docs/data/face-image-search-preregistration-20261119.md): a throw-away Postgres from the product's database image on an INTERNAL docker network (no route out), and the test container next to it.
# Starts only when the host has headroom, watches host memory like Run-FaceMeasure.ps1 (kills only these two containers below 600 MB), and always removes the containers and the network at the end. It never touches the live database.
#
#   powershell -NoProfile -File tools\media_data\Run-FaceIndex.ps1 -RowsText 10000,100000
param([string]$Data = 'C:/tmp/nexusai-media-data', [string]$Repo = 'C:/Users/sheik/Workspace/Office/Projects/NexusAI-localwork', [string]$Logs = 'C:/tmp/nexusai-g4',
      [string]$RowsText = '10000,100000', [int]$StartAvail = 2600, [string]$Image = 'nexusai-media-eval-face:20261119', [string]$PgImage = 'timescale/timescaledb:latest-pg16')
$docker = "C:\Users\sheik\AppData\Local\Programs\DockerDesktop\resources\bin\docker.exe"
function Avail { [int]((Get-Counter '\Memory\Available MBytes').CounterSamples[0].CookedValue) }
$Rows = @($RowsText -split '[ ,]' | Where-Object { $_ })   # powershell -File passes a list as one string
$net, $pg, $test = 'nx-face-index-net', 'nx-face-index', 'nxface-index-test'
$waited = 0
while ((Avail) -lt $StartAvail -and $waited -lt 7200) { Start-Sleep -Seconds 20; $waited += 20 }
"[{0}] index test: start (host available {1} MB, waited {2} s)" -f (Get-Date -Format 'HH:mm:ss'), (Avail), $waited
try {
  & $docker network create --internal $net 2>&1 | Out-Null
  & $docker run -d --name $pg --network $net --memory 2200m --memory-swap 2200m -e POSTGRES_PASSWORD=x $PgImage -c shared_buffers=512MB -c maintenance_work_mem=768MB -c max_parallel_maintenance_workers=2 -c fsync=off -c synchronous_commit=off 2>&1 | Out-Null
  $ready = $false
  for ($i = 0; $i -lt 60; $i++) { Start-Sleep -Seconds 3; & $docker exec $pg pg_isready -U postgres 2>&1 | Out-Null; if ($LASTEXITCODE -eq 0) { $ready = $true; break } }
  if (-not $ready) { "[{0}] postgres did not become ready" -f (Get-Date -Format 'HH:mm:ss'); return }
  Start-Sleep -Seconds 5
  $rowsArg = ($Rows | ForEach-Object { "$_" }) -join ' '
  $argv = @('/repo/tools/media_data/face_index_test.py', '--host', $pg, '--rows') + @($Rows | ForEach-Object { "$_" })
  & $docker run -d --name $test --network $net --memory 1800m --memory-swap 1800m --entrypoint python -v "${Data}/work-faces:/work" -v "${Repo}:/repo:ro" $Image @argv 2>&1 | Out-Null
  $reason = ''
  while ($true) {
    Start-Sleep -Seconds 15
    if ("$(& $docker inspect -f '{{.State.Running}}' $test 2>&1)" -ne 'true') { break }
    if ((Avail) -lt 600) { $reason = 'host available memory below 600 MB'; break }
  }
  if ($reason) { "[{0}] WATCHDOG {1}: killing the two index-test containers" -f (Get-Date -Format 'HH:mm:ss'), $reason; & $docker kill $test $pg 2>&1 | Out-Null }
  "[{0}] index test: finished, {1}" -f (Get-Date -Format 'HH:mm:ss'), (& $docker inspect -f 'exit={{.State.ExitCode}} oom_killed={{.State.OOMKilled}}' $test 2>&1)
  & $docker logs $test 2>&1 | Out-File -Encoding utf8 "$Logs/face_index_$(Get-Date -Format 'yyyyMMdd_HHmmss').log"
}
finally {
  & $docker rm -f $test $pg 2>&1 | Out-Null
  & $docker network rm $net 2>&1 | Out-Null
  "[{0}] index test: containers and network removed" -f (Get-Date -Format 'HH:mm:ss')
}
