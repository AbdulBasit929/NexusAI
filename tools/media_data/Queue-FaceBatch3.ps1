# Batch 3 of the face bake-off: the same steps as batch 2 that were not finished, in the order that gives the key result first. The host has little memory to spare (the Docker VM is large and the language model is
# resident), so every run starts only when 1800 MB are available and retries up to 40 times; the long runs checkpoint, so a watchdog kill costs at most a few minutes.
$run = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork\tools\media_data\Run-FaceMeasure.ps1'
& powershell -NoProfile -File $run -Only fairface-score -StartAvail 1500 -Retries 40
& powershell -NoProfile -File $run -Only lfw-e2e -StartAvail 1800 -Retries 40
& powershell -NoProfile -File $run -Only recog-rest -StartAvail 1800 -Retries 60
"[{0}] batch 3 complete" -f (Get-Date -Format 'HH:mm:ss')
