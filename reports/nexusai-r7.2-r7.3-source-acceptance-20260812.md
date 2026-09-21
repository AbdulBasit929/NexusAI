# NexusAI R7.2 and R7.3 source acceptance

Date: 2026-08-12  
Phase: R7 — Pakistan CDR and Tower Intelligence Deepening  
Status: R7.2 source accepted; R7.3 source accepted; R7.4 active

## Accepted outcomes

R7.2 upgrades the CDR adapter to 1.2.0 / `cdr-canonical/v2` while preserving
the accepted legacy CDR table projection and row hash. The canonical record now
retains raw and deterministic Pakistan-canonical subscriber, originating-party
and called-party identifiers as separate roles. It records raw/canonical
direction, service class, source timezone, timestamp assumption, UTC canonical
timezone, provider/reference aliases, sentinel conditions, review state and
quality flags. The versioned normalized projection is mirrored into
`forensic.records.metadata.normalized_fields`; no database migration is needed.

R7.3 upgrades the subscriber adapter to 1.2.0 /
`subscriber-identity/v2`. It keeps subscriber MSISDN/reference, SIM IMSI/ICCID,
device IMEI and service reference/type/plan/provider distinct. Every link is
explicitly labeled `explicit_source_row_co_observation`, accompanied by open,
closed, inverted-review or unknown validity status. Exact subscriber operations
return the additive role fields; conflict and reuse operations include ICCID,
service and provider counts without asserting ownership, physical use or
entitlement. CNIC remains masked and names remain omitted from public rows.

The shared Python platform-catalog loader was reconciled with the already
published optional operation fields (`required_inputs`, target kinds,
case-wide default and missing-input behavior). This removed a pre-existing
contract drift that prevented the wider worker suite from reaching adapters.

## Synthetic evidence

- `pakistan_cdr_messy_synthetic.csv` exercises Pakistan number variants,
  service/short-code values, a cell sentinel, timezone conversion, Urdu content
  and duplicate stability.
- `pakistan_subscriber_populated_acceptance_v1.jsonl` preserves the existing
  six-operation privacy and accounting benchmark.
- `pakistan_cdr_subscriber_role_goldens_v1.json` is the bounded R7.2/R7.3 role
  contract for canonical numbers, timezone provenance, SIM/device/service roles
  and observation-only association semantics.

No protected CDR was copied, ingested or modified.

## Verification

- Focused CDR/subscriber Python tests: 10 passed.
- Full forensic ingestion Python suite: 79 passed, 2 intentionally skipped.
- Forensic API Go package: passed; 235 specs total, 233 executed and 2 skipped.
- Go formatting completed on changed Go sources.
- No deployment, database migration/backfill, evidence mutation, model change,
  retained configuration change, staging, commit, push or publication occurred.

The expected worker error logs during the full suite are assertions for invalid
messages, terminal hash mismatch and retryable connection failure; the suite
completed successfully.

## Remaining boundary

These slices are source accepted only. Runtime/UI acceptance requires an
explicitly approved guarded rebuild. Protected real-CDR validation remains the
separate R7.8 approval gate. R7.4 now owns tower provider/sector reference
history, validity, uncertainty and overlap review.
