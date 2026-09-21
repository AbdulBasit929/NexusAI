# NexusAI R7.4 and R7.5 source acceptance

Date: 2026-08-12  
Phase: R7 — Pakistan CDR and Tower Intelligence Deepening  
Status: R7.4 source accepted; R7.5 source accepted; R7.6 active

## R7.4 accepted outcome

Tower adapter 1.2 / `tower-location/v2` preserves supplied reference facts and
adds a deterministic provider/site/sector history contract. It normalizes only
search keys, while preserving raw site, sector, provider, datum, coordinate,
uncertainty, version and validity values. Provider code, MCC/MNC, CGI/ECGI,
eNodeB/gNodeB, reference identifier/version and coordinate method/source are
explicit. Datum basis distinguishes supplied WGS84 equivalents, assumed WGS84
and values requiring transformation. Uncertainty is classified without
inventing precision.

Validity uses start-inclusive/end-exclusive windows. Missing end bounds remain
open or unknown; observed-at fallback is labeled rather than represented as a
supplied valid-from value. The deterministic conflict operation groups exact
provider/site/sector history keys and exposes overlapping validity-window pairs,
coordinate, technology, status, datum and uncertainty conflicts. It never
silently reconciles an overlap.

## R7.5 accepted outcome

The existing 65-operation public query catalog remains intact. Telecom catalog
metadata now describes the deeper subscriber/service and tower-history fields.
Natural routing covers provider/sector history, overlapping reference review,
ICCID and service links, packet-data/USSD classification and SIM-identity
history. Target-required workflows still clarify before SQL, unsupported
requests remain typed, and zero-row telecom responses remain bounded negative
results rather than evidence that an event never occurred.

No new API route, LLM, model or free-form query executor was introduced. Exact
facts remain deterministic and case/tenant scoped.

## Synthetic evidence and verification

- `pakistan_tower_history_synthetic.csv` contains closed, open, overlapping,
  provider-alias, multi-sector and missing-bound reference cases.
- `pakistan_tower_history_goldens_v1.json` fixes the provider/site/sector key,
  overlap count, datum/uncertainty and RF-inference expectations.
- Focused tower Python suite: 6 passed.
- Full ingestion Python suite: 80 passed, 2 intentionally skipped.
- Forensic API Go package: passed; 235+ specs including new R7.4/R7.5 table
  cases, target clarification and no-result contracts.
- Python compile, JSON parsing, Go formatting and patch checks passed.

Expected invalid-message, terminal-hash and retryable-connection logs are test
assertions; the full suite passed.

## Security and runtime boundary

No protected CDR, deployment, migration, backfill, retained-data mutation,
model/backend change, configuration mutation, staging, commit, push or
publication occurred. Runtime remains the accepted R6.6 deployment. R7.4 and
R7.5 are source accepted only; R7.6 owns the telecom map/timeline presentation.
