[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$library=Join-Path $PSScriptRoot 'run_current4b_final_characterization.ps1'
$tokens=$null;$errors=$null
$ast=[Management.Automation.Language.Parser]::ParseFile($library,[ref]$tokens,[ref]$errors)
if($errors.Count){throw 'LIBRARY_PARSE'}
foreach($f in $ast.FindAll({param($n)$n -is [Management.Automation.Language.FunctionDefinitionAst]},$false)){. ([scriptblock]::Create($f.Extent.Text))}
$runner=Join-Path $PSScriptRoot 'run_english_product_proof_v2.ps1'
$runnerAst=[Management.Automation.Language.Parser]::ParseFile($runner,[ref]$tokens,[ref]$errors)
if($errors.Count){throw 'RUNNER_PARSE'}
$text=[IO.File]::ReadAllText($runner)
if($text.IndexOf('New-ExclusiveLock $dispatchLock') -gt $text.IndexOf('Start-OwnedModel $false $true')){throw 'CONSUMPTION_ORDER'}
if($text -notmatch 'WindowStyle Hidden' -or $text -notmatch 'Invoke-UnloadOwnedModel' -or $text -notmatch 'Assert-ExperimentalSafety'){throw 'GUARDS_MISSING'}
if($text -match 'Measure-Object\s+-Property\s+rss_mib'){throw 'ORDERED_DICTIONARY_MEASURE_BUG'}
$scratch=Join-Path $root ('tmp\v2 powershell selftest '+[guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $scratch | Out-Null
$owned=@()
try{
 $lock=Join-Path $scratch 'exclusive lock.txt';$owned+=@($lock)
 New-ExclusiveLock $lock 'synthetic'
 $rejected=$false;try{New-ExclusiveLock $lock 'overwrite'}catch{$rejected=$true}
 if(-not $rejected -or [IO.File]::ReadAllText($lock) -cne 'synthetic'){throw 'EXCLUSIVE_LOCK_FAILURE'}
 $path=Join-Path $scratch 'checkpoint with spaces.json';$owned+=@($path)
 Write-JSON $path ([ordered]@{items=@();nested=[ordered]@{rss_mib=4.5};path=$path})
 $data=Get-Content -Raw -LiteralPath $path | ConvertFrom-Json
 if(@($data.items).Count -ne 0 -or $data.path -cne $path){throw 'JSON_EMPTY_ARRAY_OR_PATH'}
 $digest=Get-SHA256 $path
 if($digest -cnotmatch '^[a-f0-9]{64}$'){throw 'HASH_FORMAT'}
 [IO.File]::AppendAllText($path,' ')
 if((Get-SHA256 $path) -ceq $digest){throw 'HASH_DRIFT_NOT_DETECTED'}
 $rows=@([ordered]@{rss_mib=4.5},[ordered]@{rss_mib=3.25});$sum=0.0
 foreach($row in $rows){$sum+=[double]$row['rss_mib']};if($sum -ne 7.75){throw 'ORDERED_DICTIONARY_SUM'}
 $empty=@();$sum=0.0;foreach($row in $empty){$sum+=[double]$row['rss_mib']};if($sum -ne 0){throw 'EMPTY_BACKEND_SUM'}
 # Mock only observation of unloaded state; no shutdown endpoint is called.
 function Get-LoadedState { return $null }
 $script:modelOwned=$false;$script:unloadState='NOT_REQUIRED';Invoke-UnloadOwnedModel
 if($unloadState -cne 'NOT_REQUIRED'){throw 'UNOWNED_UNLOAD'}
 $script:modelOwned=$true;Invoke-UnloadOwnedModel
 if($modelOwned -or $unloadState -cne 'ALREADY_UNLOADED'){throw 'ALREADY_UNLOADED'}
 $cleanup=$false;try{try{throw 'synthetic failure'}finally{$cleanup=$true}}catch{}
 if(-not $cleanup){throw 'EXCEPTION_CLEANUP'}
 $argScript=Join-Path $scratch 'echo args.ps1';$owned+=@($argScript)
 [IO.File]::WriteAllText($argScript,'param([string]$Value) if($Value -cne ''-ginkgo.focus=English product proof V2''){exit 9}',[Text.UTF8Encoding]::new($false))
 $exe=(Get-Process -Id $PID).Path
 $child=Start-Process -FilePath $exe -ArgumentList @('-NoProfile','-File',('"'+$argScript+'"'),'-Value','"-ginkgo.focus=English product proof V2"') -PassThru -Wait -WindowStyle Hidden
 if($child.ExitCode -ne 0){throw 'DOTTED_FLAG_QUOTING'}
 Write-Host "POWERSHELL_V2_SELFTEST=PASS version=$($PSVersionTable.PSVersion) checks=13 live_inference=false"
}finally{
 foreach($path in $owned){if(Test-Path -LiteralPath $path){[IO.File]::Delete($path)}}
 [IO.Directory]::Delete($scratch,$false)
}
