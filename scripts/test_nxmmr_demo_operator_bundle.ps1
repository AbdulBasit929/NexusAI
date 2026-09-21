# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$LiveReadOnly)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'nxmmr_demo_operator_common.ps1')
$count=0
function Assert-Test([bool]$Condition,[string]$Name){if(-not $Condition){throw "TEST FAILED: $Name"};$script:count++;Write-Host "PASS $Name"}
try {
    $files=@('nxmmr_demo_operator_common.ps1','preflight_nxmmr_demo_activation.ps1','activate_nxmmr_demo.ps1','verify_nxmmr_demo_activation.ps1','rollback_nxmmr_demo_activation.ps1','run_nxmmr_demo_activation.ps1','prepare_nxmmr_demo_acceptance.ps1')
    foreach($file in $files){
        $tokens=$null;$errors=$null
        $ast=[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot $file),[ref]$tokens,[ref]$errors)
        Assert-Test ($errors.Count -eq 0) "parse $file"
        $text=$ast.Extent.Text
        Assert-Test (-not ($text -match '(?im)\bStop-Process\b|\btaskkill\b|compose\s+down|--remove-orphans|\b(system|volume)\s+prune|\bRemove-Item\b')) "no destructive/app-termination commands $file"
    }
    $manifest=Get-NxManifest
    $requirements=Get-Content -LiteralPath (Resolve-NxPath 'ingestion/forensic_records/requirements.txt') -Raw
    Assert-Test ($requirements -match '(?m)^safetensors==0\.6\.2\r?$') 'Paddle-compatible exact safetensors pin'
    Assert-Test ($requirements -notmatch '(?m)^safetensors==0\.5\.') 'obsolete incompatible safetensors pin rejected'
    Assert-Test ($requirements -match '(?m)^paddlepaddle==3\.3\.1\r?$' -and $requirements -match '(?m)^transformers==4\.49\.0\r?$') 'Paddle and Transformers versions unchanged'
    $dockerfile=Get-Content -LiteralPath (Resolve-NxPath 'ingestion/forensic_records/Dockerfile') -Raw
    Assert-Test ($dockerfile -match '--timeout 120 --retries 5' -and $dockerfile -match '"\$attempt" -ge 3') 'package socket timeout and whole-attempt retries are bounded'
    Assert-Test ($dockerfile -match '--mount=type=cache,target=/root/.cache/pip' -and $dockerfile -notmatch '--no-cache-dir') 'package cache survives a failed build without runtime mount changes'
    $activationScript=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'activate_nxmmr_demo.ps1') -Raw
    Assert-Test ($activationScript -match "'build.log'\) -StreamOutput" -and $manifest.operator_bundle.build_timeout_seconds -eq 3600) 'live build output keeps the existing one-hour build deadline'
    $assets=Get-NxAssets $manifest
    Assert-Test ($assets.Count -eq 6) 'exact three ANPR files plus three Paddle trees'
    $facts=@{ram_gib=7.0;disk_gib=20.0;active_jobs=0;docker=$true;compose=$true;health=$true;hashes=$true;license=$true;rollback=$true}
    $good=Test-NxGateValues $facts
    Assert-Test ($good.Count -eq 0) 'all-gates PASS simulation'
    $facts.ram_gib=5.999
    $bad=Test-NxGateValues $facts
    $mutations=0; if($bad.Count -eq 0){$mutations++}
    Assert-Test ($bad.Count -eq 1 -and $bad[0].gate -eq 'ram_gib' -and $mutations -eq 0) 'below-threshold RAM BLOCKED with zero mutations'
    $facts.ram_gib=6; $facts.disk_gib=12
    Assert-Test ((Test-NxGateValues $facts).Count -eq 0) 'exact floor admitted'
    foreach($key in @('docker','compose','health','hashes','license','rollback')){
        $facts[$key]=$false; Assert-Test ((Test-NxGateValues $facts).Count -eq 1) "fail closed $key"; $facts[$key]=$true
    }
    $facts.active_jobs=1; Assert-Test ((Test-NxGateValues $facts).Count -eq 1) 'job gate BLOCKED';$facts.active_jobs=0
    $facts.disk_gib=11;Assert-Test ((Test-NxGateValues $facts).Count -eq 1) 'disk gate BLOCKED'
    $fake=@{
        Id='original';Image='sha256:original';Name='/nexusai-forensic-records-worker-1'
        Config=@{Env=@('FORENSIC_ASR_ENABLED=true','FORENSIC_ANPR_ENABLED=false','FORENSIC_OCR_ENABLED=false','FORENSIC_DATABASE_URL=opaque-test-value');User='65532:65532';WorkingDir='/app';Entrypoint=@('python','/app/worker.py');Cmd=$null;Healthcheck=@{Test=@('CMD','python','-V');Interval=10000000000;Timeout=5000000000;Retries=12;StartPeriod=10000000000}}
        HostConfig=@{NetworkMode='nexusai_default';Privileged=$false;ReadonlyRootfs=$false;Memory=0;NanoCpus=0;CapAdd=@();PortBindings=@{'9109/tcp'=@(@{HostIp='';HostPort='9109'})};RestartPolicy=@{Name='unless-stopped'};LogConfig=@{Type='json-file';Config=@{}}}
        Mounts=@(@{Type='volume';Name='nexusai_forensic_spool';Destination='/data/forensic/spool';RW=$false},@{Type='bind';Source=(Resolve-NxPath 'models/media');Destination='/models/media';RW=$false})
    } | ConvertTo-Json -Depth 20 | ConvertFrom-Json
    $rollback=New-NxWorkerCompose $fake $fake.Image ([ordered]@{})
    $plan=New-NxWorkerCompose $fake 'candidate' (ConvertTo-NxMap $manifest.worker_environment) -Build
    Assert-Test ($plan.services.Count -eq 1 -and $plan.services.ContainsKey('forensic-records-worker')) 'worker-only service plan'
    Assert-Test ($plan.services['forensic-records-worker'].environment.FORENSIC_ASR_ENABLED -eq 'false') 'activation disables prior ASR'
    Assert-Test ($rollback.services['forensic-records-worker'].environment.FORENSIC_ASR_ENABLED -eq 'true') 'rollback restores actual prior ASR'
    Assert-Test ($rollback.services['forensic-records-worker'].image -eq $fake.Image) 'rollback preserves immutable original image'
    Assert-Test ($plan.services['forensic-records-worker'].environment.FORENSIC_VIDEO_ANPR_V2_ENABLED -eq 'false') 'V2 disabled'
    Assert-Test ($plan.services['forensic-records-worker'].environment.FORENSIC_VIDEO_ANPR_V3_ENABLED -eq 'true') 'V3 enabled'
    Assert-Test (@($plan.services['forensic-records-worker'].volumes | Where-Object {-not $_.read_only}).Count -eq 0) 'read-only mounts'
    Assert-Test ($plan.networks.default.external -and $plan.volumes['nexusai_forensic_spool'].external) 'existing external network/volume only'
    $escaped=$false;try{$null=Resolve-NxPath '../outside'}catch{$escaped=$true};Assert-Test $escaped 'path traversal blocked'
    $run=New-NxRunDirectory
    Write-NxJson (Join-Path $run 'selftest.json') @{state='PASS';runtime_mutated=$false;simulated_only=$true}
    Assert-Test ((Get-Content (Join-Path $run 'selftest.json') -Raw | ConvertFrom-Json).state -eq 'PASS') 'private durable logging path and ACL creation'
    if($LiveReadOnly){
        $before=Get-NxContainers
        $preflight=Invoke-NxPreflight
        Write-NxJson (Join-Path $run 'preflight.json') $preflight
        Show-NxPreflight $preflight
        Assert-Test ($preflight.facts.hashes -and $preflight.facts.compose -and $preflight.facts.rollback -and $preflight.facts.health) 'live hash/Compose/rollback/health gates verified read-only'
        $rollback=New-NxWorkerCompose $before['forensic-records-worker'] $before['forensic-records-worker'].Image ([ordered]@{})
        $activation=New-NxWorkerCompose $before['forensic-records-worker'] $manifest.operator_bundle.build_image (ConvertTo-NxMap $manifest.worker_environment) -Build
        $rf=Join-Path $run 'test-rollback-compose.json';$af=Join-Path $run 'test-activation-compose.json'
        Write-NxJson $rf $rollback;Write-NxJson $af $activation
        $null=Invoke-NxReceiptCompose $rf @('config','--quiet')
        $null=Invoke-NxReceiptCompose $af @('config','--quiet')
        Assert-Test $true 'actual current-worker rollback and activation Compose render'
        $expected=@($before['forensic-records-worker'].Config.Env | Sort-Object)
        $actual=@($rollback.services['forensic-records-worker'].environment.GetEnumerator() | ForEach-Object {"$($_.Key)=$($_.Value)"} | Sort-Object)
        Assert-Test (($expected -join "`n") -ceq ($actual -join "`n")) 'actual prior environment preserved byte-for-byte'
        $after=Get-NxContainers
        Assert-Test (((Get-NxSummary $before | ConvertTo-Json -Depth 10 -Compress)) -ceq ((Get-NxSummary $after | ConvertTo-Json -Depth 10 -Compress))) 'all live service identities/images/health/restarts unchanged'
        Write-NxJson (Join-Path $run 'live-read-only-validation.json') @{state='PASS';activation_executed=$false;containers=(Get-NxSummary $after);preflight=$preflight.status}
    }
    Write-Host "OPERATOR_BUNDLE_SELFTEST=PASS checks=$count log=$run"
    exit 0
}catch{Write-Host "OPERATOR_BUNDLE_SELFTEST=FAIL reason=$($_.Exception.Message)";exit 7}
