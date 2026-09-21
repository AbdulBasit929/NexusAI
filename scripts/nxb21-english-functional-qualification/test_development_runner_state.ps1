$ErrorActionPreference='Stop'
$tokens=$null;$errors=$null
$ast=[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'run_product_convergence_development.ps1'),[ref]$tokens,[ref]$errors)
if($errors.Count){throw 'RUNNER_PARSE_FAILED'}
$source=[IO.File]::ReadAllText((Join-Path $PSScriptRoot 'run_product_convergence_development.ps1'))
if($source.Contains('$`{script:scratchName}') -or $source.Contains('$`{script:scratchPassword}')){throw 'SCRATCH_DSN_LITERAL_INTERPOLATION'}
if(-not $source.Contains('$($script:scratchName)') -or -not $source.Contains('$($script:scratchPassword)')){throw 'SCRATCH_DSN_INTERPOLATION_MISSING'}
foreach($name in @('Get-LatencySummary','Get-DevelopmentOutcome')){
 $fn=$ast.Find({param($n) $n -is [Management.Automation.Language.FunctionDefinitionAst] -and $n.Name -eq $name},$true)
 . ([scriptblock]::Create($fn.Extent.Text))
}
$row={param($correct,$validated)[pscustomobject]@{correct=$correct;validated=$validated;latency_ms=100}}
$good=[pscustomobject]@{
 complete=$true
 registered=@(1..5|ForEach-Object{&$row $true $false})
 dynamic=@(1..2|ForEach-Object{&$row $true $false})
 terminal=@(1..3|ForEach-Object{&$row $true $false})
 synthesis=@(1..3|ForEach-Object{&$row $false $true})
}
foreach($code in @(0,7)) {
 $child=Start-Process powershell.exe -ArgumentList @('-NoProfile','-Command',"Start-Sleep -Milliseconds 100; exit $code") -WindowStyle Hidden -PassThru
 $null=$child.Handle
 while(-not $child.HasExited){Start-Sleep -Milliseconds 30;$child.Refresh()}
 $child.WaitForExit()
 if($null -eq $child.ExitCode -or $child.ExitCode -ne $code){throw "CHILD_EXIT_MISMATCH:$code"}
 $state=Get-DevelopmentOutcome $true $true $child.ExitCode $good '' 'PASS' 'PASS'
 $expected=if($code -eq 0){'DEVELOPMENT_COMPLETED_PASS'}else{'DEVELOPMENT_DRIVER_ERROR_AFTER_INFERENCE'}
 if($state -ne $expected){throw "STATE_MISMATCH:$state"}
 $child.Dispose()
}
foreach($code in @($null,0,7)){foreach($result in @($null,$good)){foreach($errorText in @('','injected failure')){
 $state=Get-DevelopmentOutcome $true $true $code $result $errorText 'PASS' 'PASS'
 if($state -like '*NO_INFERENCE*'){throw 'FALSE_NO_INFERENCE'}
}}}
$good.registered[0].correct=$false
if((Get-DevelopmentOutcome $true $true 0 $good '' 'PASS' 'PASS') -ne 'DEVELOPMENT_COMPLETED_FUNCTIONAL_FAILURE'){throw 'FUNCTIONAL_FAILURE_LOST'}
if((Get-DevelopmentOutcome $true $true 0 $good '' 'FAIL' 'PASS') -ne 'RUNTIME_INTEGRITY_FAILURE'){throw 'INTEGRITY_FAILURE_LOST'}
if((Get-DevelopmentOutcome $false $true $null $null '' 'PASS' 'NOT_REQUIRED') -ne 'RESOURCE_ADMISSION_FAILURE_NO_INFERENCE'){throw 'ADMISSION_FAILURE_LOST'}
Write-Output 'RUNNER_STATE_TESTS=PASS native_exit_0_and_7=true no_inference_after_dispatch=false'
