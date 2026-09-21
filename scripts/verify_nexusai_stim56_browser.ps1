param([string]$BaseUrl = 'http://127.0.0.1:8089')

$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$uiRoot = Join-Path $repoRoot 'core\http\react-ui'
$vitePath = Join-Path $uiRoot 'node_modules\vite\bin\vite.js'
$nodePath = (Get-Command node -ErrorAction Stop).Source
$npmPath = (Get-Command npm.cmd -ErrorAction Stop).Source
$temporaryID = [guid]::NewGuid().ToString('N')
$stdoutPath = Join-Path ([IO.Path]::GetTempPath()) "nexusai-stim56-vite-$temporaryID.out.log"
$stderrPath = Join-Path ([IO.Path]::GetTempPath()) "nexusai-stim56-vite-$temporaryID.err.log"
$previousExternalServer = [Environment]::GetEnvironmentVariable('PLAYWRIGHT_EXTERNAL_SERVER', 'Process')
$previousBaseUrl = [Environment]::GetEnvironmentVariable('PLAYWRIGHT_BASE_URL', 'Process')
$server = $null
$locationPushed = $false

if (-not (Test-Path -LiteralPath $vitePath -PathType Leaf)) {
  throw "Vite is not installed at $vitePath; restore the existing UI workspace dependencies before running this deck"
}

try {
  $server = Start-Process -FilePath $nodePath `
    -ArgumentList @($vitePath, '--host', '127.0.0.1', '--port', '8089', '--strictPort') `
    -WorkingDirectory $uiRoot `
    -WindowStyle Hidden `
    -RedirectStandardOutput $stdoutPath `
    -RedirectStandardError $stderrPath `
    -PassThru

  $ready = $false
  $deadline = (Get-Date).AddSeconds(60)
  while ((Get-Date) -lt $deadline) {
    if ($server.HasExited) {
      $stderr = if (Test-Path -LiteralPath $stderrPath) { Get-Content -Raw -LiteralPath $stderrPath } else { '' }
      throw "The STIM-5/6 browser server exited before readiness: $stderr"
    }
    try {
      $response = Invoke-WebRequest -UseBasicParsing -TimeoutSec 2 -Uri $BaseUrl
      if ($response.StatusCode -eq 200) {
        $ready = $true
        break
      }
    } catch {
      Start-Sleep -Milliseconds 250
    }
  }
  if (-not $ready) { throw "Timed out waiting for the STIM-5/6 browser server at $BaseUrl" }

  $env:PLAYWRIGHT_EXTERNAL_SERVER = '1'
  $env:PLAYWRIGHT_BASE_URL = $BaseUrl
  Push-Location $uiRoot
  $locationPushed = $true

  & $npmPath run test:e2e -- e2e/analyst-portal.spec.js
  if ($LASTEXITCODE -ne 0) { throw 'The 11-case Analyst Portal browser deck failed' }

  $focusedAgentCases = 'renders only curated corpus prompts and exposes typed specialist provenance accessibly|renders Urdu results RTL while exact identifiers remain LTR at mobile width'
  & $npmPath run test:e2e -- e2e/agents.spec.js --grep $focusedAgentCases
  if ($LASTEXITCODE -ne 0) { throw 'The rich-answer or Urdu RTL browser case failed' }

  Write-Host 'STIM56BrowserAcceptance=PASS AnalystPortal=11/11 RichAnswer=PASS UrduRTL390=PASS ExternalServer=true'
} finally {
  if ($locationPushed) { Pop-Location }
  if ($null -eq $previousExternalServer) { Remove-Item Env:\PLAYWRIGHT_EXTERNAL_SERVER -ErrorAction SilentlyContinue } else { $env:PLAYWRIGHT_EXTERNAL_SERVER = $previousExternalServer }
  if ($null -eq $previousBaseUrl) { Remove-Item Env:\PLAYWRIGHT_BASE_URL -ErrorAction SilentlyContinue } else { $env:PLAYWRIGHT_BASE_URL = $previousBaseUrl }
  if ($server -and -not $server.HasExited) {
    Stop-Process -Id $server.Id -ErrorAction SilentlyContinue
    Wait-Process -Id $server.Id -Timeout 10 -ErrorAction SilentlyContinue
  }
  Remove-Item -LiteralPath $stdoutPath, $stderrPath -Force -ErrorAction SilentlyContinue
}
