# Batch 4: after the pgvector index test has removed its containers, the registered flip-test arm for AuraFace on CPLFW (Amendment A2b: only where the reproduction target was missed by more than the tolerance),
# then the image-retrieval stages (translation of captions into Urdu, embeddings of SigLIP v1 and SigLIP 2, scoring, near-duplicate retrieval). One run at a time, each starting only with headroom.
$run = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork\tools\media_data\Run-FaceMeasure.ps1'
while (-not (Select-String -Path C:\tmp\nexusai-g4\run_face_index.log -Pattern 'containers and network removed' -Quiet -ErrorAction SilentlyContinue)) { Start-Sleep -Seconds 20 }
"[{0}] index test over: batch 4 starts" -f (Get-Date -Format 'HH:mm:ss')
& powershell -NoProfile -File $run -Only recog-cplfw-flip -StartAvail 1800 -Retries 40
& powershell -NoProfile -File $run -Only retr-translate -StartAvail 1500 -Retries 20 -Threads 6
& powershell -NoProfile -File $run -Only retr-embed -StartAvail 2200 -Retries 20
& powershell -NoProfile -File $run -Only retr-score -StartAvail 1200 -Retries 10
& powershell -NoProfile -File $run -Only neardup -StartAvail 2200 -Retries 20
"[{0}] batch 4 complete" -f (Get-Date -Format 'HH:mm:ss')
