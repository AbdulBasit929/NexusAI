@echo off
rem Starts the workspace dev server (Vite, lane tree) on port 4181 so that it SURVIVES the Claude app: processes started from the app's shell die when the app quits or restarts
rem (seen 2026-10-22), a process started by Explorer does not. Open this file through Explorer instead of running it from a shell:
rem     explorer.exe C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork\tools\dev\Start-Workspace-Detached.cmd
rem The API key is read from .env.forensic-runtime.local of the main checkout into this process only; it is never printed or written. Close the window to stop the server.
setlocal
set "LANE=%~dp0..\.."
set "MAINENV=%LANE%\..\NexusAI\.env.forensic-runtime.local"
set "PORT=4181"
for /f "usebackq tokens=1,* delims==" %%a in (`findstr /b /c:"FORENSIC_RECORDS_API_KEY=" "%MAINENV%"`) do set "FORENSIC_RECORDS_API_KEY=%%b"
if not defined FORENSIC_RECORDS_API_KEY (
  echo FORENSIC_RECORDS_API_KEY is not in %MAINENV%
  pause
  exit /b 1
)
set "FORENSIC_RECORDS_API_KEY=%FORENSIC_RECORDS_API_KEY:"=%"
cd /d "%LANE%\apps\investigation-workspace"
title NexusAI workspace dev server %PORT%
node node_modules\vite\bin\vite.js --port %PORT% --strictPort
