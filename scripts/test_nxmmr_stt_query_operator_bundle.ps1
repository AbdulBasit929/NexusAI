# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$LiveReadOnly)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'nxmmr_stt_query_operator_common.ps1')
$count=0
function Assert-SttTest([bool]$Condition,[string]$Name){if(-not $Condition){throw "TEST FAILED: $Name"};$script:count++;Write-Host "PASS $Name"}
try {
    $files=@('nxmmr_stt_query_operator_common.ps1','preflight_nxmmr_stt_query_activation.ps1','run_nxmmr_stt_query_activation.ps1','activate_nxmmr_stt_query.ps1','verify_nxmmr_stt_query_activation.ps1','rollback_nxmmr_stt_query_activation.ps1')
    foreach($file in $files){
        $tokens=$null;$errors=$null;$ast=[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot $file),[ref]$tokens,[ref]$errors)
        Assert-SttTest ($errors.Count -eq 0) "parse $file"
        $text=$ast.Extent.Text
        Assert-SttTest (-not ($text -match '(?im)\bStop-Process\b|\btaskkill\b|compose\s+down|--remove-orphans|\b(?:system|volume)\s+prune|docker\s+volume\s+rm')) "no destructive or app-termination command $file"
    }
    $manifest=Get-NxSttManifest
    Assert-SttTest (($manifest.services.build_and_recreate_no_deps -join ',') -ceq 'api,forensic-records-api,forensic-records-worker') 'exact three-service activation scope'
    Assert-SttTest (($manifest.services.must_remain_identical -join ',') -ceq 'forensic-postgres,forensic-nats') 'database and broker protected'
    Assert-SttTest ($manifest.worker_environment.FORENSIC_ASR_ENABLED -ceq 'true' -and $manifest.worker_environment.FORENSIC_ASR_MODEL -ceq 'faster-whisper-small-ur') 'existing ASR alias activated'
    Assert-SttTest ($manifest.worker_environment.FORENSIC_IMAGE_EMBEDDING_ENABLED -ceq 'true' -and $manifest.worker_environment.FORENSIC_FACE_ENABLED -ceq 'true') 'SigLIP and candidate-face roles activated'
    Assert-SttTest ($manifest.worker_environment.FORENSIC_ANPR_ENABLED -ceq 'true' -and $manifest.worker_environment.FORENSIC_OCR_ENABLED -ceq 'true' -and $manifest.worker_environment.FORENSIC_VIDEO_ANPR_V3_ENABLED -ceq 'true') 'existing ANPR OCR VideoV3 roles preserved'
    Assert-SttTest ($manifest.worker_environment.FORENSIC_VIDEO_ANPR_V2_ENABLED -ceq 'false' -and $manifest.worker_environment.FORENSIC_TTS_ENABLED -ceq 'false') 'unadmitted roles remain disabled'
    Assert-SttTest ($manifest.worker_environment.HF_HUB_OFFLINE -ceq '1' -and $manifest.worker_environment.TRANSFORMERS_OFFLINE -ceq '1') 'implicit model downloads disabled'
    $assets=Get-NxSttAssets $manifest
    Assert-SttTest ($assets.Count -eq 7) 'exact seven-file SigLIP cache admitted'
    Assert-SttTest (($assets|Measure-Object -Property size_bytes -Sum).Sum -eq 815871927) 'SigLIP byte total fixed'
    $facts=@{ram_gib=6;disk_gib=20;active_jobs=0;docker=$true;health=$true;integrity=$true;assets=$true;models=$true;rollback=$true;compose=$true}
    Assert-SttTest ((Test-NxSttGateValues $facts $manifest).Count -eq 0) 'all gates pass at exact floors'
    $facts.ram_gib=5.999;Assert-SttTest ((Test-NxSttGateValues $facts $manifest)[0].gate -eq 'ram_gib') 'RAM gate fails closed'
    $facts.ram_gib=6;$facts.active_jobs=1;Assert-SttTest ((Test-NxSttGateValues $facts $manifest)[0].gate -eq 'active_jobs') 'active-job gate fails closed'
    $source=Get-Content -LiteralPath (Resolve-NxPath 'scripts/activate_nxmmr_stt_query.ps1') -Raw
    Assert-SttTest ($source -match "'up','-d','--no-deps','--no-build','--pull','never','--force-recreate'" -and $source -notmatch "'down'") 'bounded no-dependency recreation only'
    Assert-SttTest (([regex]::Matches($source,'Wait-NxSttRam \$manifest')).Count -ge 3) 'RAM recovery wait surrounds sequential builds and recreation'
    Assert-SttTest ($source -match '(?s)foreach\(\$name in \$script:NxSttChanged\).*?Wait-NxSttRam \$manifest.*?build.*?\$name') 'each new image build is individually RAM gated'
    Assert-SttTest ($source -match 'nexusai\.nxmmr\.stt-query-build-set/v1' -and $source -match 'source_seal_sha256' -and $source -match 'RESUMABLE_BUILD_SET=PASS') 'completed builds are sealed and resumable after RAM failure'
    $common=Get-Content -LiteralPath (Resolve-NxPath 'scripts/nxmmr_stt_query_operator_common.ps1') -Raw
    Assert-SttTest ($common -match '--exec grep \^Dirty:' -and $common -match '--exec grep \^Writeback:' -and $common -match 'Dirty -eq 0' -and $common -match 'Writeback -eq 0' -and $common -match '--exec /sbin/sysctl -w vm\.drop_caches=1') 'safe page-cache recovery requires clean Docker Linux writes'
    Assert-SttTest ($common -notmatch 'vm\.drop_caches=3' -and $common -notmatch "'sync'") 'RAM recovery never invokes sync or mode 3'
    Assert-SttTest ($common -match '\$nextRecovery' -and $common -notmatch '\$recoveryAttempted') 'RAM wait retries safe recovery after dirty writes drain'
    Assert-SttTest ($common -match "healthTest\[1\].*Replace\('\$','\$\$'\)") 'compose healthcheck preserves container-side environment expansion'
    if($LiveReadOnly){
        $before=Get-NxSttContainers;$receipt=Invoke-NxSttPreflight;Show-NxSttPreflight $receipt
        Assert-SttTest ($receipt.facts.integrity -and $receipt.facts.assets -and $receipt.facts.models -and $receipt.facts.rollback -and $receipt.facts.compose) 'live read-only integrity model rollback and compose gates'
        $after=Get-NxSttContainers
        Assert-SttTest (((Get-NxSummary $before|ConvertTo-Json -Depth 20 -Compress)) -ceq ((Get-NxSummary $after|ConvertTo-Json -Depth 20 -Compress))) 'live service identities remain unchanged'
    }
    Write-Host "STT_QUERY_OPERATOR_BUNDLE_SELFTEST=PASS checks=$count runtime_mutated=false"
    exit 0
} catch {Write-Host "STT_QUERY_OPERATOR_BUNDLE_SELFTEST=FAIL reason=$($_.Exception.Message)";exit 7}
