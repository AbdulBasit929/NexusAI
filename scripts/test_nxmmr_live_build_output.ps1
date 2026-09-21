# SPDX-License-Identifier: MIT
[CmdletBinding()]
param([switch]$LinuxReadOnly)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'nxmmr_demo_operator_common.ps1')
$run=New-NxRunDirectory
$script:seen=New-Object 'System.Collections.Generic.List[string]'
$script:ackPath=Join-Path $run 'stream-ack.json'
$script:liveLog=Join-Path $run 'stream.log'
$count=0
function Assert-Test([bool]$Condition,[string]$Name) {
    if(-not $Condition){throw "TEST FAILED: $Name"}
    $script:count++; Microsoft.PowerShell.Utility\Write-Host "PASS $Name"
}
function Write-Host($Object) {
    $script:seen.Add([string]$Object)
    if ($Object -eq 'early-stdout') {
        # The child cannot finish until the terminal callback sees flushed output.
        if ((Get-Content -LiteralPath $script:liveLog -Raw) -notmatch 'early-stdout') { throw 'Log was not flushed before terminal output.' }
        Write-NxJson $script:ackPath @{seen_before_child_exit=$true}
    }
}
function Invoke-TestChild([string]$Code,[string]$Log,[int]$Timeout=15,[switch]$Quiet) {
    $encoded=[Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($Code))
    return Invoke-NxNative powershell.exe @('-NoProfile','-NonInteractive','-EncodedCommand',$encoded) $Timeout $Log -StreamOutput:(-not $Quiet)
}
try {
    $escaped=$script:ackPath.Replace("'","''")
    $code="[Console]::Out.WriteLine('early-stdout'); [Console]::Error.WriteLine('early-stderr'); " +
        "`$deadline=[DateTime]::UtcNow.AddSeconds(8); while(-not (Test-Path -LiteralPath '$escaped')) { if([DateTime]::UtcNow -gt `$deadline){exit 8}; Start-Sleep -Milliseconds 25 }; [Console]::Out.Write('final-without-newline')"
    $result=Invoke-TestChild $code $script:liveLog
    Assert-Test (Test-Path -LiteralPath $script:ackPath) 'terminal and durable log receive output before child exits'
    Assert-Test ($result -match 'early-stdout' -and $result -match 'final-without-newline' -and $result -notmatch 'early-stderr') 'stdout return stays separate from stderr and host output'
    $logged=Get-Content -LiteralPath $script:liveLog -Raw
    Assert-Test ($logged -match 'early-stderr' -and $script:seen.Contains('early-stderr') -and $script:seen.Contains('final-without-newline')) 'stderr and unterminated final line reach terminal and log'

    $failed=$false; $failureLog=Join-Path $run 'failure.log'
    try {$null=Invoke-TestChild "[Console]::Error.WriteLine('failure-marker'); exit 23" $failureLog} catch {$failed=$_.Exception.Message -match 'exit 23'}
    Assert-Test ($failed -and (Get-Content $failureLog -Raw) -match 'failure-marker') 'nonzero exit fails closed and preserves output'

    $timeoutLog=Join-Path $run 'timeout.log';$timedOut=$false;$clock=[Diagnostics.Stopwatch]::StartNew()
    try {$null=Invoke-TestChild "[Console]::Out.WriteLine('before-timeout'); Start-Sleep -Seconds 20" $timeoutLog 2} catch {$timedOut=$_.Exception.Message -match 'timed out'}
    Assert-Test ($timedOut -and $clock.Elapsed.TotalSeconds -lt 10 -and (Get-Content $timeoutLog -Raw) -match 'before-timeout') 'bounded timeout retains partial log'

    $script:seen.Clear();$quietLog=Join-Path $run 'quiet.log'
    $result=Invoke-TestChild "[Console]::Out.WriteLine('private-inspect-fixture'); [Console]::Error.WriteLine('private-error-fixture')" $quietLog -Quiet
    Assert-Test ($script:seen.Count -eq 0 -and $result -eq 'private-inspect-fixture') 'default inspect/config output is not echoed'

    $bulkLog=Join-Path $run 'bulk.log'
    $result=Invoke-TestChild "for(`$i=0;`$i -lt 2000;`$i++){[Console]::Out.WriteLine('out-'+`$i);[Console]::Error.WriteLine('err-'+`$i)}" $bulkLog
    Assert-Test ((Get-Content $bulkLog).Count -eq 4000 -and $result -match 'out-1999') 'concurrent stdout/stderr drains without deadlock or dropped lines'
    $refused=$false;try{$null=Invoke-NxNative powershell.exe @('-NoProfile','-Command','exit 0') -StreamOutput}catch{$refused=$_.Exception.Message -match 'durable log'}
    Assert-Test $refused 'streaming without durable receipt is refused'
    if ($LinuxReadOnly) {
        # Exercise the exact Dockerfile shell with fake pip: no network, installs or mounts.
        $dockerfile=Get-Content -LiteralPath (Resolve-NxPath 'ingestion/forensic_records/Dockerfile') -Raw
        $match=[regex]::Match($dockerfile,'(?s)RUN --mount=type=cache,target=/root/\.cache/pip \\\r?\n(.*?)\r?\n\r?\nCOPY')
        if (-not $match.Success) { throw 'Could not extract bounded package command.' }
        $body=($match.Groups[1].Value -replace '\\\r?\n',' ').Trim()
        foreach ($case in @(@{failures=0;expected=1;exit=0},@{failures=1;expected=2;exit=0},@{failures=9;expected=3;exit=19})) {
            $stub='calls=0; python() { calls=$((calls + 1)); echo "FAKE_PIP_CALL=$calls"; if [ "$calls" -le '+$case.failures+' ]; then return 19; fi; return 0; }; sleep() { :; }; '
            $log=Join-Path $run ("retry-$($case.failures).log");$failed=$false
            try {
                $null=Invoke-NxNative docker @('run','--rm','--pull','never','--network','none','--read-only','--memory','128m','--cpus','1','--entrypoint','sh','sha256:90744cff8f32887f075c47d747a173ff333e9e98801667af93c357fa9f5e28ff','-c',($stub+$body)) 30 $log
            } catch { if ($case.exit -eq 0 -or $_.Exception.Message -notmatch "exit $($case.exit)") { throw };$failed=$true }
            $calls=@(Select-String -LiteralPath $log -Pattern 'FAKE_PIP_CALL=').Count
            Assert-Test ($calls -eq $case.expected -and $failed -eq ($case.exit -ne 0)) "Linux package loop failures=$($case.failures) attempts=$($case.expected) exit=$($case.exit)"
        }
    }
    Write-NxJson (Join-Path $run 'streaming-selftest.json') @{state='PASS';checks=$count;image_built=$false;services_mutated=$false}
    Microsoft.PowerShell.Utility\Write-Host "LIVE_BUILD_OUTPUT_SELFTEST=PASS checks=$count log=$run"
    exit 0
} catch { Microsoft.PowerShell.Utility\Write-Host "LIVE_BUILD_OUTPUT_SELFTEST=FAIL reason=$($_.Exception.Message) log=$run";exit 7 }
