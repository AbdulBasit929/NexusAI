# NexusAI Knowledge Base Collection Reconciliation

Date: 2026-07-31  
Mode: read-only inventory and UI decluttering; no collection reset or evidence deletion

## Decision

`records-demo-verified` is the only analyst-selectable governed case collection.
The Records and Collections interfaces now hide retained acceptance, legacy demo,
and accidentally name-derived collections by default. A deliberate **Review
retained system collections** control exposes them for audit. This removes daily
workspace clutter without silently destroying retained evidence.

## Live inventory

The live API returned 26 collections. Ten have zero entries and are deletion
candidates:

1. `Communications_CDR_Analyst`
2. `communications_cdr_analyst`
3. `Forensic_Records_Analyst`
4. `forensic_records_analyst`
5. `Network_IPDR_Capture_Analyst`
6. `network_ipdr_capture_analyst`
7. `Vehicle_ANPR_Geospatial_Analyst`
8. `vehicle_anpr_geospatial_analyst`
9. `nexusai-structured-demo-20260730`
10. `nexusai-structured-demo-v2-20260730`

The specialist-name collections were automatically materialized while agent
configurations were created. They are not the specialist agents themselves.
The two `nexusai-structured-demo*` collections are superseded by the governed
pilot.

Collections containing retained entries are not deletion candidates in this
pass:

| Collection group | Entry range | Disposition |
|---|---:|---|
| `records-demo-verified` | 8 | Keep; only analyst-selectable governed case |
| `records-demo` | 6 | Retain hidden until separately archived or reconciled |
| Phase 2 acceptance/golden/audit collections | 1-39 | Retain hidden as acceptance evidence |

All source-feed endpoints returned zero configured external sources. Hiding
these collections changes presentation only; it does not change vector data,
evidence registry rows, canonical records, agents, or named Docker volumes.

## Safe operator verification

Run this before approving any deletion:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

$base = 'http://localhost:8080/api/agents/collections'
$names = (Invoke-RestMethod -Uri $base -TimeoutSec 15).collections
$inventory = foreach ($name in $names) {
  $encoded = [uri]::EscapeDataString($name)
  $entryResult = Invoke-RestMethod -Uri "$base/$encoded/entries" -TimeoutSec 15
  $sourceResult = Invoke-RestMethod -Uri "$base/$encoded/sources" -TimeoutSec 15
  [pscustomobject]@{
    Name = $name
    Entries = [int]$entryResult.count
    Sources = @($sourceResult.sources).Count
  }
}
$inventory | Sort-Object Entries, Name | Format-Table -AutoSize
```

The destructive reset endpoint used by the current LocalAI collection UI can
remove stored collection contents. Therefore no batch reset command is included
and none was executed. Delete only the exact ten zero-entry candidates after a
fresh zero-entry/zero-source readback and explicit operator approval. Never
include `records-demo-verified`, an acceptance collection, a collection with a
non-zero count, or a name not present in the manifest above.

## UI acceptance

After the next operator rebuild:

1. Open `/app/collections`.
2. Confirm the default list shows `records-demo-verified` and omits the retained
   system/legacy names.
3. Confirm the page reports the number of hidden retained collections.
4. Select **Review retained system collections** and confirm all 26 names are
   available for audit.
5. Return to Records and confirm only `records-demo-verified` is selectable as a
   governed case.

This is the reversible production-safe state. Physical deletion remains a
separate, explicitly approved operation.
