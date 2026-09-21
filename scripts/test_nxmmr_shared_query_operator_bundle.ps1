# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'nxmmr_shared_query_operator_common.ps1')
$count=0
function Assert-SharedQueryTest([bool]$Condition,[string]$Name){if(-not $Condition){throw "TEST FAILED: $Name"};$script:count++;Write-Host "PASS $Name"}
try {
    $files=@('nxmmr_shared_query_operator_common.ps1','preflight_nxmmr_shared_query_activation.ps1','run_nxmmr_shared_query_activation.ps1','activate_nxmmr_shared_query.ps1','verify_nxmmr_shared_query_activation.ps1','rollback_nxmmr_shared_query_activation.ps1')
    foreach($file in $files){
        $tokens=$null;$errors=$null;$ast=[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot $file),[ref]$tokens,[ref]$errors)
        Assert-SharedQueryTest ($errors.Count -eq 0) "parse $file"
        Assert-SharedQueryTest (-not ($ast.Extent.Text -match '(?im)\bStop-Process\b|\btaskkill\b|compose\s+down|--remove-orphans|\b(?:system|volume)\s+prune|docker\s+volume\s+rm')) "no destructive or app-termination command $file"
    }
    $manifest=Get-NxSttManifest
    Assert-SharedQueryTest (($manifest.services.build_and_recreate_no_deps -join ',') -ceq 'api,forensic-records-api,forensic-records-worker') 'exact three-service activation scope'
    Assert-SharedQueryTest (($manifest.services.must_remain_identical -join ',') -ceq 'forensic-postgres,forensic-nats') 'database and broker protected'
    Assert-SharedQueryTest ([double]$manifest.resource_gates.minimum_available_ram_gib -eq 6) '6 GiB RAM floor preserved'
    Assert-SharedQueryTest ($manifest.prohibitions -contains 'prune' -and $manifest.prohibitions -contains 'model download' -and $manifest.prohibitions -contains 'bulk evidence reprocessing') 'destructive and expansive actions prohibited'
    $facts=@{ram_gib=6;disk_gib=20;active_jobs=0;docker=$true;health=$true;integrity=$true;assets=$true;models=$true;rollback=$true;compose=$true}
    Assert-SharedQueryTest ((Test-NxSttGateValues $facts $manifest).Count -eq 0) 'all gates pass at exact floors'
    $facts.ram_gib=5.999;Assert-SharedQueryTest ((Test-NxSttGateValues $facts $manifest)[0].gate -eq 'ram_gib') 'RAM gate fails closed'
    $source=Get-Content -LiteralPath (Resolve-NxPath 'scripts/activate_nxmmr_shared_query.ps1') -Raw
    Assert-SharedQueryTest ($source -match "'up','-d','--no-deps','--no-build','--pull','never','--force-recreate'" -and $source -notmatch "'down'") 'bounded no-dependency recreation only'
    Assert-SharedQueryTest (([regex]::Matches($source,'Wait-NxSttRam \$manifest')).Count -ge 3) 'RAM recovery surrounds sequential builds and recreation'
    Assert-SharedQueryTest ($source -match 'nexusai\.nxmmr\.shared-query-build-set/v1' -and $source -match 'source_seal_sha256') 'completed builds are source-bound and resumable'
    Assert-SharedQueryTest ($source -notmatch 'Copy-NxSttAssets') 'activation copies no model assets'
    Assert-SharedQueryTest ($manifest.worker_environment.FORENSIC_ASR_MODEL -ceq 'faster-whisper-small-ur' -and $manifest.worker_environment.FORENSIC_ASR_ENABLED -ceq 'true') 'worker preserves admitted Urdu-capable ASR role'
    $runner=Get-Content -LiteralPath (Resolve-NxPath 'scripts/run_nxmmr_shared_query_activation.ps1') -Raw
    Assert-SharedQueryTest ($runner -match 'WaitForRamMinutes=120' -and $runner -match 'Wait-NxSttRam' -and $runner -match "gate -ne 'ram_gib'") 'single command waits safely when RAM is the only pending gate'
    $base=Get-Content -LiteralPath (Resolve-NxPath 'scripts/nxmmr_stt_query_operator_common.ps1') -Raw
    Assert-SharedQueryTest ($base -match '--exec /sbin/sysctl -w vm\.drop_caches=1' -and $base -notmatch 'vm\.drop_caches=3') 'safe page-cache mode 1 only'
    Write-Host "SHARED_QUERY_OPERATOR_BUNDLE_SELFTEST=PASS checks=$count runtime_mutated=false"
    exit 0
} catch {Write-Host "SHARED_QUERY_OPERATOR_BUNDLE_SELFTEST=FAIL reason=$($_.Exception.Message)";exit 7}
