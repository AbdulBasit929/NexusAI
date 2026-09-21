# NexusAI Phase 7.2 Tower/Site Source and Runtime Acceptance

Date: 2026-08-04  
Verdict: **PASS — bounded adapter/API/profile/runtime slice; full LocalAI UI/direct-route refresh pending memory gate**

## Delivered vertical slice

Phase 7.2 adds a dedicated `tower_location` family adapter, a separately
governed `Tower_Location_Reference_Analyst`, and six deterministic operations:

1. `tower.site_lookup` / `tower_site_lookup`
2. `tower.reference_timeline` / `tower_reference_timeline`
3. `tower.coordinate_audit` / `tower_coordinate_audit`
4. `tower.status_summary` / `tower_status_summary`
5. `tower.alias_conflicts` / `tower_alias_conflicts`
6. `tower.cdr_join` / `tower_cdr_join`

The registry now exposes 6 adapters, 50 operations, and 65 question templates.
The specialist profile is version 1.1.0, active, bound to
`records-demo-verified`, and permits Qwen 4B only for optional bounded
synthesis. Bootstrap completed with six active specialist profiles and did not
create or delete a collection.

## Adapter and forensic constraints

The adapter preserves exact site/sector/LAC/TAC/provider aliases, coordinates,
datum, uncertainty, status, technology, and validity bounds. WGS84/EPSG:4326 is
accepted; another datum is explicitly marked transform-required rather than
silently converted. Latitude, longitude, azimuth, beamwidth, and uncertainty
are bounded. Pakistan coordinate bounds are a screening flag, not a geolocation
conclusion. Missing sector, datum, azimuth, or uncertainty remains flagged and
is not invented.

The 10-row synthetic adapter fixture reconciles to 6 candidates, 5 unique
accepted rows, 1 exact duplicate, and 4 rejected rows (invalid latitude,
longitude, azimuth, and missing identifier).

No operation infers RF propagation, sector coverage, handset position, device
presence, subscriber location, or identity. The time-aware CDR join uses the
latest eligible supplied reference valid at the CDR timestamp and keeps
unmatched CDR rows explicit.

## Live read-only acceptance

Acceptance used the preserved synthetic collection
`forensic-phase2-complete-acceptance-20260727`; no Tower upload, reprocess,
cleanup, or evidence mutation was needed.

- canonical coverage: 34 records — transaction 6, access_log 5, ANPR 5,
  subscriber 5, tower_location 5, CDR 4, generic 2, IPDR 2
- `tower_coordinate_audit`: 5 cited rows
- `tower_site_lookup`, target `PK-LHR-SYN-001`: 1 cited row
- `tower_reference_timeline`, same target: 1 cited row
- `tower_status_summary`: 4 groups totaling 5 rows — ACTIVE/LTE 2,
  ACTIVE/5G 1, PLANNED/LTE 1 review, RETIRED/3G 1 review; request
  `7c8614b7-1d37-4ffd-9338-eec736d76c05`
- `tower_alias_conflicts`: 0 rows, an accepted no-results outcome
- `tower_cdr_join`, target `CELL-01`: 2 cited CDR rows, 0 matched references,
  2 explicitly unmatched; request `d60fde6c-14e6-4766-a357-f89b0e1581aa`

The unmatched join is correct: the CDR fixture uses `CELL-01/02/03`, while the
reference rows use separate synthetic Pakistan site identifiers. The system did
not manufacture a link.

## Deployment and verification

Two guarded API/worker sidecar activations completed while preserving the
LocalAI image, rollback images, named volumes, and existing data:

- `reports/runtime-activation-20260804/phase7.2-sidecar-activation.json`
- `reports/runtime-activation-20260804/phase7.2-coverage-sidecar-activation.json`

The second activation included the canonical-coverage correction. Final gates:

- Tower plus Phase 5A Python: 8 passed
- combined family adapters: 26 passed
- forensic API: 222 specs registered, 220 passed, 2 skipped
- Go vet: passed
- direct-agent routing test: passed
- React production build: passed (658 modules; existing warnings only)

## Agent Chat and honest activation boundary

The live Tower specialist route opened correctly and the question
`Look up tower/site reference observations; target=PK-LHR-SYN-001; limit=2`
returned the exact one-row cited result from the correct collection with no
model call. Because available free RAM was only approximately 2.6 GiB, the
guarded 6 GiB LocalAI rebuild was not attempted. The unchanged LocalAI image
routed that natural-language phrase through compatible legacy
`tower_activity`, while the new `tower_site_lookup` direct route and new Tower
UI controls are source-tested but not yet present in the running LocalAI image.

This does not block the bounded Phase 7.2 adapter/API/runtime acceptance. It is
a precise pending activation item for the next safe full LocalAI refresh.

## Next bounded work

When at least 6 GiB free RAM is available, run one guarded full LocalAI refresh,
then repeat only the exact Tower direct-route/UI smoke. After that, continue the
approved sequence with Pakistan CDR deepening against time-aware tower
references; do not infer CDR-to-reference matches when exact identifiers and
validity do not support them.

