[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
foreach($file in @('run_current4b_final_characterization.ps1','run_english_product_proof_v2.ps1')){
 $t=$null;$e=$null;$ast=[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot $file),[ref]$t,[ref]$e)
 if($e.Count){throw 'PARSE_FAILURE'}
 foreach($f in $ast.FindAll({param($n)$n -is [Management.Automation.Language.FunctionDefinitionAst]},$false)){. ([scriptblock]::Create($f.Extent.Text))}
}
$private=Join-Path $root 'local-acceptance-models\nxb21-english-product-proof-v2-20260911'
$binary=Join-Path $private 'product-proof-evaluator.exe'
if(-not(Test-Path -LiteralPath $binary)){throw 'BUILD_EVALUATOR_FIRST'}
if(Test-Path -LiteralPath (Join-Path $private 'product-proof.dispatched.lock')){throw 'CONSUMED'}
try{
 Set-V2DiscoveryEnvironment
 $env:NXB21_V2_LIVE=$null;$env:NXB21_V2_PACKAGE=$PSScriptRoot
 $env:NXB21_V2_CONTEXT_OUT=Join-Path $PSScriptRoot 'english-product-proof-v2-product-context.txt'
 & $binary '-test.run=^TestForensicRecordsSynthesis$' '-ginkgo.focus=English product proof V2 captures current product discovery' '-ginkgo.no-color' '-test.timeout=90s' *> (Join-Path $private 'context-preparation.log')
 if($LASTEXITCODE -ne 0){throw 'CURRENT_DISCOVERY_CHECK_FAILED'}
 Write-Host "CURRENT_PRODUCT_DISCOVERY=PASS sha256=$(Get-SHA256 $env:NXB21_V2_CONTEXT_OUT) live_inference=false"
}finally{
 foreach($name in @('NXB21_V2_DISCOVERY_KEY','NXB21_V2_DISCOVERY_TENANT','NXB21_V2_LIVE','NXB21_V2_PACKAGE','NXB21_V2_CONTEXT_OUT')){[Environment]::SetEnvironmentVariable($name,$null,'Process')}
}
