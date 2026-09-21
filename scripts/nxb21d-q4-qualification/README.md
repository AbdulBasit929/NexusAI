# NX-B2.1D Q4 final qualification

This bundle evaluates the refrozen Q4 candidate once against a new independent
168-case holdout. The old Q8 holdout and every development corpus are retired.
The thresholds remain 100% for all critical cases and at least 95% within each
language group.

Run only from standalone Windows PowerShell with Docker Desktop running and
optional desktop applications closed:

```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI
Set-ExecutionPolicy -Scope Process Bypass -Force
.\scripts\nxb21d-q4-qualification\run_nxb21d_q4_final_qualification.ps1
```

Return the final receipt to Codex. Do not activate D directly.

After Codex reviews a `SUITABLE` receipt and the owner explicitly approves the
deployment, run the separately gated activation entry point:

```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI
Set-ExecutionPolicy -Scope Process Bypass -Force
.\scripts\nxb21d-q4-qualification\run_nxb21_b1c_q4_activation.ps1 -OwnerApproved
```

Activation rebuilds and recreates only `api` and `forensic-records-api`. It
preserves the worker, PostgreSQL, NATS, retained evidence, Activity count, and
the unloaded Q4 model state. Use the rollback Compose receipt only if the
activation log explicitly directs recovery after a service was recreated.

If that specific recovery direction is printed, use the exact activation run
directory from its log:

```powershell
.\scripts\nxb21d-q4-qualification\rollback_nxb21_b1c_q4_activation.ps1 -RunDirectory '<exact activation run directory>'
```
