# Unified Investigation Workspace — Operator Guide

Date: 2026-09-07  
Required service mutation: `api` only  
Required free host RAM immediately before build/recreate: 6 GiB

## Before activation

1. Docker Desktop must be using the Linux engine.
2. The accepted API image must still be
   `sha256:feb935b42a93c1810394cb270a809189ca240a225253159a47b8daa72a799171`.
3. `forensic-records-api`, `forensic-records-worker`, `forensic-nats` and
   `forensic-postgres` must be running.
4. Active forensic jobs must be zero.
5. Do not edit any file in the sealed build context.

Optional read-only validation:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\sheik\Workspace\Office\Projects\NexusAI\scripts\activate_nexusai_unified_workspace_ui_20260907.ps1" -ValidateOnly
```

Expected validation markers:

```text
DEPLOYMENT_MANIFEST=PASS
CONTEXT_INTEGRITY=PASS
D_UNQUALIFIED_SOURCE_INCLUDED=NO
COMPOSE_OVERLAY_8088=PASS
COMPOSE_OVERLAY_8080=PASS
ACTIVE_JOBS=0
UNIFIED_UI_PREFLIGHT=PASS
MUTATION_PERFORMED=false
UI_ACTIVATION=READY
```

## Activation

Run exactly:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\sheik\Workspace\Office\Projects\NexusAI\scripts\activate_nexusai_unified_workspace_ui_20260907.ps1"
```

The script repeats every read-only preflight, then checks the 6 GiB RAM floor
immediately before its first mutation. If the floor is not met it reports the
largest processes read-only and exits without building, tagging or recreating
anything. It never stops a process.

Success markers:

```text
RAM_GATE=PASS
SEALED_UI_IMAGE_BUILD=PASS
LIVE_ANALYST_INDEX=PASS
PROTECTED_SERVICES_PRESERVED=true
UNIFIED_ANALYST_UX=LIVE_VERIFIED
INVESTIGATION_WORKSPACE_UI=ACTIVE
UI_ACTIVATION=PASS
RETAINED_DATA_MUTATED=false
NX-B2.1D=OPEN
D_ACTIVATION=BLOCKED
```

## Receipt

- Receipt:
  `reports/nxb21/investigation-workspace-unified-ui-activation-20260907/activation-receipt.json`
- Checksum:
  `reports/nxb21/investigation-workspace-unified-ui-activation-20260907/activation-receipt.json.sha256`

The receipt is written only after the live `/analyst` bytes match the sealed UI
index, the backend is ready, protected container IDs/images are unchanged, and
the retained-state tuple is unchanged.

## Quick live smoke

Open `http://localhost:8080/analyst` and verify:

1. the unified top bar has evidence, History and Add data;
2. no permanent Home/Data/Ask/Activity primary navigation appears;
3. Add data opens the real governed intake dialog (do not upload during a
   read-only smoke);
4. Evidence opens the right-side source drawer;
5. the Ask composer remains visible and usable;
6. the browser console is clean.

## Rollback

The activation creates
`nexusai/localai-forensic:rollback-before-unified-workspace-ui-20260907` from
the exact pre-activation image. To restore it:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\sheik\Workspace\Office\Projects\NexusAI\scripts\activate_nexusai_unified_workspace_ui_20260907.ps1" -Rollback
```

Rollback also gates on 6 GiB immediately before the API-only recreate and
verifies protected services plus retained state afterward.

## Live URL

`http://localhost:8080/analyst`

