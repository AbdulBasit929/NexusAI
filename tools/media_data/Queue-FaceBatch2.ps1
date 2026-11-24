# Batch 2 of the face bake-off (docs/data/face-image-search-preregistration-20261119.md): the sealed half of WIDER FACE once per detector at the thresholds chosen on the development half
# (YuNet 0.80 and BlazeFace 0.60, from wider-*-dev-report.json), the 4-thread cost timing, then the long recognition runs. One run at a time; each long run checkpoints and retries after a watchdog kill.
$run = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork\tools\media_data\Run-FaceMeasure.ps1'
& powershell -NoProfile -File $run -Only detect-yunet-sealed -YunetThr 0.8
& powershell -NoProfile -File $run -Only detect-blaze-sealed -BlazeThr 0.6
& powershell -NoProfile -File $run -Only timing -Threads 4
& powershell -NoProfile -File $run -Only fairface
& powershell -NoProfile -File $run -Only lfw-e2e
& powershell -NoProfile -File $run -Only recog-rest
& powershell -NoProfile -File $run -Only fairface-score
"[{0}] batch 2 complete" -f (Get-Date -Format 'HH:mm:ss')
