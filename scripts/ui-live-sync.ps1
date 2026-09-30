# Keeps your working copy in step with the UI redesign branch while the Vite dev server runs, so design changes
# appear in the browser on their own (Vite hot reload) without typing commands after each change.
#
# Use two PowerShell windows, once:
#   1. npm --prefix apps/investigation-workspace run dev      (leave it running; open http://127.0.0.1:4181/)
#   2. .\scripts\ui-live-sync.ps1                              (leave it running)
#
# It only ever fast-forwards: if you have local edits that would be overwritten, it stops and tells you instead of
# forcing anything. Press Ctrl+C to stop.
param(
  [string]$Branch = 'claude/gifted-pasteur-4kpx0r',
  [int]$Seconds = 8
)

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
Write-Host "Watching origin/$Branch every $Seconds s. Ctrl+C to stop." -ForegroundColor Cyan

while ($true) {
  try {
    git fetch origin $Branch --quiet
    $local = (git rev-parse HEAD).Trim()
    $remote = (git rev-parse "origin/$Branch").Trim()
    if ($local -ne $remote) {
      git merge --ff-only "origin/$Branch" --quiet
      if ($LASTEXITCODE -eq 0) {
        $subject = (git log -1 --format=%s).Trim()
        Write-Host ("{0}  updated: {1}" -f (Get-Date -Format 'HH:mm:ss'), $subject) -ForegroundColor Green
      } else {
        Write-Host 'Could not fast-forward (local changes or diverged history). Nothing was changed; resolve it, then it will resume.' -ForegroundColor Yellow
      }
    }
  } catch {
    Write-Host ("{0}  {1}" -f (Get-Date -Format 'HH:mm:ss'), $_.Exception.Message) -ForegroundColor Yellow
  }
  Start-Sleep -Seconds $Seconds
}
