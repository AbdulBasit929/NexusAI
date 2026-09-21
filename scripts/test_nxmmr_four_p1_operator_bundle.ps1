# SPDX-License-Identifier: MIT
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'nxmmr_four_p1_operator_common.ps1')
$count=0
function Assert-P1BundleTest([bool]$Condition,[string]$Name){if(-not $Condition){throw "TEST FAILED: $Name"};$script:count++;Write-Host "PASS $Name"}
try {
    $files=@('nxmmr_four_p1_operator_common.ps1','preflight_nxmmr_four_p1_activation.ps1','run_nxmmr_four_p1_activation.ps1','activate_nxmmr_four_p1.ps1','verify_nxmmr_four_p1_activation.ps1','rollback_nxmmr_four_p1_activation.ps1')
    foreach($file in $files){
        $tokens=$null;$errors=$null;$ast=[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot $file),[ref]$tokens,[ref]$errors)
        Assert-P1BundleTest ($errors.Count -eq 0) "parse $file"
        Assert-P1BundleTest (-not ($ast.Extent.Text -match '(?im)\bStop-Process\b|\btaskkill\b|compose\s+down|--remove-orphans|\b(?:system|image|volume)\s+prune|docker\s+volume\s+rm|drop_caches=3')) "no destructive cleanup or arbitrary app termination $file"
    }
    $manifest=Get-NxP1Manifest
    Assert-P1BundleTest ($manifest.previous_activation_receipt -ceq '20260902T154901947Z') 'new candidate is bound to accepted R2 receipt'
    Assert-P1BundleTest ($manifest.activation_namespace -ceq 'NX-MMR-FOUR-P1-SOFTWARE-DEFECT-CLOSURE-V1') 'new activation identity is distinct from used R2'
    Assert-P1BundleTest ($script:NxSttRunRoot -match 'private-activation-four-p1$') 'old private receipts cannot be resumed'
    $baseline=@{}
    foreach($property in $manifest.starting_runtime.PSObject.Properties){$baseline[$property.Name]=[pscustomobject]@{Id=$property.Value.container_id;Image=$property.Value.image_id;RestartCount=$property.Value.restart_count}}
    Assert-NxP1StartingRuntime $baseline $manifest
    Assert-P1BundleTest $true 'exact R2 runtime and restart baseline admitted'
    $baseline['forensic-records-worker'].RestartCount++;$rejected=$false
    try{Assert-NxP1StartingRuntime $baseline $manifest}catch{$rejected=$true}
    Assert-P1BundleTest $rejected 'changed protected worker restart count rejected'
    Assert-P1BundleTest (($manifest.services.build_and_recreate_no_deps -join ',') -ceq 'api,forensic-records-api') 'minimum source-owned two-service candidate scope'
    Assert-P1BundleTest (($manifest.services.preserve_exact_identity -join ',') -ceq 'forensic-records-worker,forensic-postgres,forensic-nats') 'worker database and broker protected'
    Assert-P1BundleTest ([double]$manifest.resource_gates.minimum_available_ram_gib -eq 6) '6 GiB RAM floor preserved'
    Assert-P1BundleTest ([string]$manifest.services.build_network -ceq 'default' -and [string]$manifest.services.pull_policy -ceq 'never') 'approved build networking and runtime no-pull policy'
    Assert-P1BundleTest ($manifest.prohibitions -contains 'database migration' -and $manifest.prohibitions -contains 'bulk evidence reprocessing' -and $manifest.prohibitions -contains 'model download') 'migration reprocess and model download prohibited'
    $facts=@{ram_gib=6;disk_gib=20;active_jobs=0;docker=$true;health=$true;integrity=$true;assets=$true;models=$true;rollback=$true;compose=$true}
    Assert-P1BundleTest ((Test-NxSttGateValues $facts $manifest).Count -eq 0) 'all gates pass at exact floors'
    $facts.ram_gib=5.999;Assert-P1BundleTest ((Test-NxSttGateValues $facts $manifest)[0].gate -eq 'ram_gib') 'RAM gate fails closed'
    $facts.ram_gib=6;$facts.active_jobs=1;Assert-P1BundleTest ((Test-NxSttGateValues $facts $manifest)[0].gate -eq 'active_jobs') 'active-job gate fails closed'
    $activate=Get-Content -LiteralPath (Resolve-NxPath 'scripts/activate_nxmmr_four_p1.ps1') -Raw
    $buildPattern=[regex]::Escape("'build','--pull=false',`$name")
    Assert-P1BundleTest ($activate -match 'Set-NxP1BuildPolicy \$activation \$manifest' -and $activate -match $buildPattern -and $activate -notmatch "'pull'") 'every candidate build uses approved policy without forced base refresh'
    $compose=[ordered]@{services=[ordered]@{api=@{build=@{context='fixture';dockerfile='Dockerfile'};environment=@{UNCHANGED='yes'}};'forensic-records-api'=@{build=@{context='fixture';dockerfile='api/forensic_records/Dockerfile'}}}}
    Set-NxP1BuildPolicy $compose $manifest
    Assert-P1BundleTest ($compose.services.api.build.network -ceq 'default' -and $compose.services['forensic-records-api'].build.network -ceq 'default') 'actual compose maps allow dependency networking in both builds'
    Assert-P1BundleTest ($compose.services.api.build.args.LOCALAI_BUILD_GOMAXPROCS -ceq '4' -and $compose.services.api.build.args.LOCALAI_BUILD_GOFLAGS -ceq '-p=2') 'actual API build receives bounded Go parallelism'
    Assert-P1BundleTest ($compose.services.api.environment.UNCHANGED -ceq 'yes' -and $compose.services.api.environment.Count -eq 1 -and -not $compose.services['forensic-records-api'].build.Contains('args')) 'build policy does not alter runtime environment or unrelated build arguments'
    foreach($field in @('build_network','pull_policy','build_dependency_acquisition')) {
        $invalid=$manifest|ConvertTo-Json -Depth 30|ConvertFrom-Json;$invalid.services.$field='unapproved';$rejected=$false
        try{Assert-NxP1BuildPolicy $invalid}catch{$rejected=$true}
        Assert-P1BundleTest $rejected "reject unapproved $field"
    }
    $invalid=$manifest|ConvertTo-Json -Depth 30|ConvertFrom-Json;$invalid.services.api_build_args.LOCALAI_BUILD_GOFLAGS='-p=20';$rejected=$false
    try{Assert-NxP1BuildPolicy $invalid}catch{$rejected=$true}
    Assert-P1BundleTest $rejected 'reject increased Go parallelism'
    $compose.services['forensic-records-worker']=@{build=@{}};$rejected=$false
    try{Set-NxP1BuildPolicy $compose $manifest}catch{$rejected=$true}
    Assert-P1BundleTest $rejected 'reject build scope expansion to protected worker'
    $receipt='{"state":"STARTING_CANDIDATE_SERVICES","retained_counts_before":"fixture"}'|ConvertFrom-Json
    Set-NxP1ReceiptOutcome $receipt 'VERIFIED'
    $receipt=$receipt|ConvertTo-Json|ConvertFrom-Json
    Assert-P1BundleTest ($receipt.state -ceq 'VERIFIED' -and $receipt.activation_verification -ceq 'PASS' -and $receipt.live_failed_cells_acceptance -ceq 'PENDING' -and $receipt.retained_counts_before -ceq 'fixture') 'verification outcome survives JSON round trip from legacy receipt'
    Set-NxP1ReceiptOutcome $receipt 'VERIFIED'
    Assert-P1BundleTest ($receipt.activation_verification -ceq 'PASS') 'verification outcome write is repeatable'
    $receipt='{"state":"STARTING_CANDIDATE_SERVICES"}'|ConvertFrom-Json
    Set-NxP1ReceiptOutcome $receipt 'ROLLED_BACK'
    $receipt=$receipt|ConvertTo-Json|ConvertFrom-Json
    Assert-P1BundleTest ($receipt.state -ceq 'ROLLED_BACK' -and $receipt.rollback_verification -ceq 'PASS') 'rollback outcome survives JSON round trip from legacy receipt'
    $snapshotContainer=[pscustomobject]@{Config=[pscustomobject]@{Env=@('SAFE=value')}}
    $direct=[pscustomobject]@{api=$snapshotContainer}
    Assert-P1BundleTest ((Get-NxP1SnapshotContainer $direct 'api').Config.Env[0] -ceq 'SAFE=value') 'snapshot reader accepts direct container object'
    $wrapped=[pscustomobject]@{api=[pscustomobject]@{value=@($snapshotContainer);Count=1}}
    Assert-P1BundleTest ((Get-NxP1SnapshotContainer $wrapped 'api').Config.Env[0] -ceq 'SAFE=value') 'snapshot reader accepts observed single-element value wrapper'
    $invalid=[pscustomobject]@{api=[pscustomobject]@{value=@($snapshotContainer,$snapshotContainer);Count=2}};$rejected=$false
    try{$null=Get-NxP1SnapshotContainer $invalid 'api'}catch{$rejected=$true}
    Assert-P1BundleTest $rejected 'snapshot reader rejects ambiguous multi-element wrapper'
    Assert-P1BundleTest ($activate -match 'Find-NxP1ResumableBuildSet' -and $activate -match 'source_seal_sha256') 'completed images are source-bound and resumable'
    Assert-P1BundleTest (([regex]::Matches($activate,'Wait-NxSttRam \$manifest')).Count -ge 4) 'RAM checks surround sequential build and recreation boundaries'
    Assert-P1BundleTest ($activate -match "state='BUILT_AND_SEALED'" -and $activate.IndexOf("state='BUILT_AND_SEALED'") -lt $activate.IndexOf("'stop','--timeout'")) 'guarded drain occurs only after candidates are built and sealed'
    $drainPattern=[regex]::Escape("'stop','--timeout','30',`$before.api.Id,`$before['forensic-records-api'].Id")
    Assert-P1BundleTest ($activate -match $drainPattern -and $activate -notmatch "forensic-records-worker'\]\.Id") 'drain is limited to admitted mutable services'
    Assert-P1BundleTest ($activate -match "'up','-d','--no-deps','--no-build','--pull','never','--force-recreate'" -and $activate -notmatch "'down'") 'bounded no-dependency recreation only'
    $rollback=Get-Content -LiteralPath (Resolve-NxPath 'scripts/rollback_nxmmr_four_p1_activation.ps1') -Raw
    Assert-P1BundleTest ($rollback -match 'Assert-NxSttProtected' -and $rollback -match 'Assert-NxP1RetainedState') 'rollback protects identities and retained accounting'
    $verify=Get-Content -LiteralPath (Resolve-NxPath 'scripts/verify_nxmmr_four_p1_activation.ps1') -Raw
    Assert-P1BundleTest ($verify -match 'Assert-NxP1WorkerRoles' -and $verify -match 'Get-NxSttModelInventory' -and $verify -match 'RestartCount') 'verification preserves worker roles models and restart health'
    Assert-P1BundleTest ($verify.Contains('Set-NxP1ReceiptOutcome $state ''VERIFIED''') -and $rollback.Contains('Set-NxP1ReceiptOutcome $state ''ROLLED_BACK''')) 'verification and rollback use tested receipt writer'
    $base=Get-Content -LiteralPath (Resolve-NxPath 'scripts/nxmmr_stt_query_operator_common.ps1') -Raw
    Assert-P1BundleTest ($base -match '--exec /sbin/sysctl -w vm\.drop_caches=1' -and $base -notmatch 'vm\.drop_caches=3') 'safe clean page-cache mode 1 only'
    $runner=Get-Content -LiteralPath (Resolve-NxPath 'scripts/run_nxmmr_four_p1_activation.ps1') -Raw
    Assert-P1BundleTest ($runner -match 'WaitForRamMinutes=120' -and $runner -match 'AllowGuardedDrain' -and $runner -match "gate -ne 'ram_gib'") 'single command waits and admits guarded drain only for RAM'
    Write-Host "P1_CORRECTION_OPERATOR_BUNDLE_SELFTEST=PASS checks=$count runtime_mutated=false"
    exit 0
} catch {Write-Host "P1_CORRECTION_OPERATOR_BUNDLE_SELFTEST=FAIL reason=$($_.Exception.Message)";exit 7}
