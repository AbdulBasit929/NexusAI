# The analyst views and the read-only analyst role (migration 017, a draft)

Status 2026-10-14: **drafted and tested on a throwaway database; not applied to the live database; no Go code reads the views yet.** It is the first build step of the Ask engine (roadmap 6.8, milestone M1) and the first half of decision D19 (the read-only analyst role). Applying it to the live database waits for the owner's explicit go, after a backup.

## Why

The Ask engine lets the language model write SQL for every data family and for link analysis. Two things follow:

1. The model must reach only reviewed, documented, already-scoped relations, never a base table. So the model queries the schema `analyst`, and a database role that can read only that schema runs its statements.
2. On the CPU laptop the model's output length is the latency (about 5 tokens a second; roadmap 6.8.13). So the views are **thick**: they already carry the joins, the exclusions (services and hubs), the number classes and the pre-aggregates, and a question becomes a short SELECT.

## The views

| View | One row is | What it saves the model from writing |
|---|---|---|
| `analyst.v_comm_events` | a call or SMS of a target call record (repeats and tower dumps left out) | choosing the table, the dataset kind, the repeat flag, the number normalisation, the sentinel counterparties |
| `analyst.v_contact_pairs` | a subscriber (`src`) and an other party (`dst`) | the aggregation by direction and time, and the `is_service` and `is_hub` rules |
| `analyst.v_cells` | a cell site (the case's own list wins over the tenant-wide one) | the choice between sources and validity |
| `analyst.v_cell_visits` | a subscriber, cell and hour, with the cell's position and radius | the join to the cell reference and the hourly grouping |
| `analyst.v_device_pairings` | a number with a SIM and a handset, first and last seen | the grouping for device and SIM changes |
| `analyst.v_data_sessions` | an internet session; `holds_from` and `holds_to` are the interval in which the public address and port block belonged to the subscriber | the address-translation interval rule |
| `analyst.v_app_usage` | an application usage record | the table choice |

Every view and every column has a description (`COMMENT`); the schema catalogue of roadmap 6.8.3 is generated from them, and `verify.sql` fails if one is missing. Withheld on purpose: raw payloads, URLs, user agents, record hashes. Hub numbers (in contact with more than `app.hub_threshold` distinct subscribers, 20 when unset) are flagged `is_hub`; short codes, sender names and any non-person-like party are `is_service`; a sentinel such as `-1` is not a counterparty (`dst` is null).

## Scope is structural

The views are **not** `security_invoker`: they read the tables as their owner, and the owner is not bound by row-level security. So each view carries its own predicate, `tenant_id = current_setting('app.tenant_id', true) AND collection_id = current_setting('app.collection_id', true)`. The application sets both settings with `SET LOCAL` before a statement; an unset setting is NULL, the predicate is never true, and the views return no rows (fail closed). The model's SQL cannot name another case, because the views do not let it.

## The role

`forensic_analyst_ro`: NOLOGIN, no superuser, no BYPASSRLS, USAGE on schema `analyst` and SELECT on its views, nothing else (the migration first revokes anything it held, so applying it again repairs a role that was given too much). `forensic_runtime` is made a member, so the application can run, inside one transaction:

```sql
BEGIN READ ONLY;
SET LOCAL ROLE forensic_analyst_ro;
SET LOCAL statement_timeout = '15s';
SET LOCAL search_path = analyst, pg_catalog;   -- the model names views without a schema
SELECT set_config('app.tenant_id', $1, true);
SELECT set_config('app.collection_id', $2, true);
SELECT set_config('app.hub_threshold', $3, true);
-- the one validated SELECT of the model
COMMIT;
```

Role-level settings (`ALTER ROLE ... SET statement_timeout`) are **not** used: they apply at login, not at `SET ROLE`. The timeout and the read-only transaction are set per transaction by the application.

## What the test proves (`tools/case_generator/test_analyst_views.py`, 10 tests, about 70 s)

- The ops scripts pass on a database built like a fresh one; the migration applies twice; rollback and forward again.
- Queries written over the views give the planted-truth keys of a generated case written by the production typed ingest: A1 (contact profile of every subject), A2 (common contacts of every planted pair, hubs and services left out by the views' flags) and A13 (who held an address and port at an instant).
- Through the role the views return the case's rows for the right settings and nothing for another tenant, another collection, no settings, or only one of the two (fail closed), for every view.
- The role cannot read `forensic.comm_event`, `forensic.records`, `evidence_items`, `cell_site` or `layout_versions` ("permission denied"), cannot insert, update or delete in a view, cannot create a table (read-only transaction), and the application's login role cannot become the owner from inside the analyst role.
- `verify.sql` fails for each of eight kinds of damage (a base-table grant, USAGE on `forensic`, a write grant, a missing column or view description, an unreviewed relation in the schema, a view without its tenant filter, a role that can log in), and re-applying the migration repairs each.
- Nine deliberate faults in the migration were each caught by the tests (see the run recorded in the handoff).

## Facts and traps found

- `SET ROLE` is checked against the **session** user. In a superuser session `SET LOCAL ROLE x` can go anywhere, so the escalation test logs in as `forensic_runtime` (a unix-socket login works in the throwaway image).
- Every role can read TimescaleDB's own metadata views (hypertable names, jobs) and the system catalogue by default. They hold no case data; `verify.sql` excuses them and still flags the **chunks**, which do hold rows. The engine's validator allows only `analyst` relations.
- A view with `DISTINCT` is not auto-updatable, so a write attempt on `v_cells` fails with a different message from "permission denied"; the write tests use an updatable view.
- `json_agg` puts line breaks between elements; tests use `jsonb_agg` so that one result is one line.

## What it does not do yet

No Go reads the views; the schema catalogue generator, the validator extensions and the model prompt are the next steps of M1. Views for the entity and relationship layer, plates, subscribers and transactions come with their typed writers. `v_cells` has rows only where a cell reference exists (the generated case has none by itself; the test adds some).

## Apply and roll back (by hand, on a retained database, after a backup and the owner's go)

```bash
psql -f db/forensic_records/ops/017_analyst_views/preflight.sql
psql -f db/forensic_records/017_analyst_views.sql
psql -f db/forensic_records/ops/017_analyst_views/verify.sql
psql -f db/forensic_records/ops/017_analyst_views/rollback.sql   # removes schema analyst and the role
```
