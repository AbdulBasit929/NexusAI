# NexusAI Enterprise Forensic Agent Chat Runtime Status

Date: 2026-07-30  
Production URL: `http://localhost:8080/app/agents/Forensic_Records_Analyst/chat`

## Current deployed state

- The temporary Vite preview on port 3000 is stopped and is not part of the
  production demonstration path.
- LocalAI, the forensic records API, PostgreSQL, NATS and the ingestion worker
  are healthy after rollback recovery.
- The running rollback snapshots were captured immediately before the final
  timeout-only rebuild attempt. They include the enterprise Agent Chat UI,
  explicit local proxy identity, low-latency exact-query routing and the latest
  forensic API synthesis behavior.
- Named model, data, configuration, Knowledge Base and forensic database volumes
  were preserved. No real evidence was uploaded, modified or deleted.

## Proven production behavior

- Gate 13 completed successfully with `APIBuildSeconds=5.5`,
  `LocalAIBuildSeconds=454.7`, `APIHealth=PASS`, `LocalAIReady=PASS`,
  `ProxyIdentity=PASS`, preserved volumes and preserved rollback images.
- Gate 13b completed successfully after the records-only explicit-synthesis
  correction: `PreviewOptOut=PASS`, `APIHealth=PASS`, preserved volumes and a
  preserved API rollback image.
- Production Agent Chat loaded the active collection
  `nexusai-structured-demo-v2-20260730`, tenant `default`, and explanation model
  `qwen_qwen3-4b-instruct-2507`.
- The previously failing `who are the frequent contacts?` workflow returned an
  exact `frequent_contacts` / `records_sql` answer with planner confidence 1,
  six rows and the raw-table-to-LLM guardrail. No browser console error occurred.
- Direct warm bounded Qwen synthesis returned a 535-character `llm_summary` in
  43.4 seconds with no fallback. The cold attempt correctly hit the sidecar's
  60-second ceiling and returned deterministic fallback rather than an
  unsupported model claim.

## Source-complete change awaiting one clean build

- Agent-to-sidecar POST timeout is increased from 45 to 75 seconds so the
  verified 43.4-second warm synthesis plus HTTP/UI overhead can complete.
- Focused Agent tests pass, including local proxy identity, deterministic routing
  and bounded model synthesis.
- The timeout-only image rebuild was stopped after Windows fell to 1.19 GiB free
  RAM and the compiler wrapper showed no meaningful CPU advance for 30 seconds.
  The pre-build snapshots were restored and health-checked.
- The rebuild gate now has a single-instance mutex, bounded local Go compilation
  (`GOMAXPROCS=4`, `GOFLAGS=-p=2`), automatic dependency startup, health checks,
  rollback restoration and named-volume preservation.

## One clean post-reboot publication command

After rebooting, start Docker Desktop, keep Codex and browsers closed, open
PowerShell in the repository and run:

```powershell
Set-Location "C:\Users\sheik\Workspace\Office\Projects\NexusAI"
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\build_deploy_forensic_phase4_gate13.ps1
```

Do not start a second copy of the command. The gate now rejects overlapping
executions. Completion requires a final `Phase4Gate13=PASS` line.

## Git and publication state

All work remains unstaged, uncommitted and unpushed. No GitHub operation was
performed.
