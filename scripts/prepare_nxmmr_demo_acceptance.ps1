# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$ListOnly)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
try {
    . (Join-Path $PSScriptRoot 'nxmmr_demo_operator_common.ps1')
    $null=Get-NxManifest
    $samples=@(
        @{role='image-positive';source='C:\Users\sheik\Downloads\archive\Pakistani License Number Plates Data\Cars\DSC_1068.JPG';sha256='87946843c81894c395bb8681197af2ac679bb8a577d83990ac944781a6227a6c'},
        @{role='image-negative';source=(Resolve-NxPath 'local-acceptance-models/nxmmr/private-benchmarks/ocr/fixtures/empty_scene.png');sha256='675453522fb2237fe3e59482810caa2d3afa67fdf8d3b3773e3ce2a3fcb01e12'},
        @{role='video-v3';source='C:\Users\sheik\Downloads\sample.mp4';sha256='d470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9fafc83762d137ee'},
        @{role='printed-english';source=(Resolve-NxPath 'local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/multilingual-ocr-fixtures/english-01.png');sha256='4a4ba6ee923cb500fe85a88916896ac2c0c59a198c6a21c48440f6fbf0bd9949'},
        @{role='printed-urdu';source=(Resolve-NxPath 'local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/multilingual-ocr-fixtures/urdu-01.png');sha256='fa65bd4d0b67794f9bbdbf9b4430d4f63e41de67b83a482d764b453f2e7fcf6d'},
        @{role='printed-mixed';source=(Resolve-NxPath 'local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/multilingual-ocr-fixtures/mixed-01.png');sha256='1e084ad8fc2e8a9437cdced19d3337447da1307f84481401d3c924ab5c3aea19'}
    )
    foreach($sample in $samples){if((Get-NxHash $sample.source) -cne $sample.sha256){throw "Missing/changed acceptance sample: $($sample.source)"}}
    if($ListOnly){$samples | ConvertTo-Json -Depth 5; Write-Host 'ACCEPTANCE_SAMPLE_INVENTORY=PASS. No files copied or evidence uploaded.';return}
    $run=Get-NxRunDirectory ''
    $state=Get-Content -LiteralPath (Join-Path $run 'state.json') -Raw | ConvertFrom-Json
    if($state.state -ne 'VERIFIED'){throw 'ACTIVATION_VERIFICATION=PASS is required before preparing fresh copies.'}
    $containers=Get-NxContainers; Assert-NxHealth $containers
    if($containers['forensic-records-worker'].Image -ne $state.expected_image){throw 'The verified worker image is no longer running.'}
    $pack=Join-Path $run ('fresh-acceptance-'+[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffZ'))
    $null=New-Item -ItemType Directory -Path $pack
    $inventory=@()
    foreach($sample in $samples){
        $target=Join-Path $pack ($sample.role+[IO.Path]::GetExtension($sample.source))
        [IO.File]::Copy($sample.source,$target,$false)
        if((Get-NxHash $target) -cne $sample.sha256){throw 'Fresh copy hash mismatch.'}
        $inventory+=@{role=$sample.role;file=$target;sha256=$sample.sha256;source=$sample.source;oracle='human source inspection; no model oracle';acceptance='PENDING'}
    }
    Write-NxJson (Join-Path $pack 'inventory.json') $inventory
    Write-Host "ACCEPTANCE_PACK=READY path=$pack"
    Write-Host 'Open http://localhost:8080. Use a new controlled case/collection and Add Data, one file at a time.'
    Write-Host 'Verify fresh processing identity; if content deduplication reuses an old result, STOP instead of bulk reprocessing.'
    Write-Host 'Follow docs/demo/nexusai-team-lead-multimodal-demo-v3.md: Data, Ask, citations, Activity, timeline/seek, zero state, RTL and four viewports.'
    Write-Host 'No benchmark or upload has been started. Browser acceptance uses normal workload-aware resources, not the build RAM floor.'
} catch {Write-Host "ACCEPTANCE_PACK=BLOCKED reason=$($_.Exception.Message)";exit 6}
