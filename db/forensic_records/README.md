# Forensic Records DB

This folder contains Phase 1 database migrations for the enterprise forensic records pipeline.

Apply with the TimescaleDB container in `docker-compose.forensic-records.yaml`; the SQL files are mounted into `/docker-entrypoint-initdb.d` on first database initialization.

Main objects:

- `forensic.evidence_items`
- `forensic.records_ingest_jobs`
- `forensic.cdr_records`
- `forensic.generic_records`
- `forensic.record_entities`
- `forensic.kb_collection_assets`
- `forensic.kb_active_metadata`
- `forensic.records_ingest_errors`
- `forensic.records_audit_log`
- `forensic.cdr_frequent_contacts`
- `forensic.entity_activity_summary`

The schema enables row-level security and expects callers to set:

```sql
SET app.tenant_id = 'default';
```

For databases initialized before a later migration was added, apply the new migration manually without deleting data:

```powershell
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/002_job_row_counters.sql
```

Phase 2 KB asset and entity indexing migration:

```powershell
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/003_phase2_kb_assets_and_entities.sql
```

Unified evidence registry migration:

```powershell
docker compose -f docker-compose.forensic-records.yaml exec forensic-postgres `
  psql -U localrecall -d localrecall `
  -f /docker-entrypoint-initdb.d/007_evidence_items.sql
```

`forensic.evidence_items` is the evidence-first catalog. It links raw uploads to Knowledge Base entries, structured ingest jobs, canonical records, and future document/media processing results through `evidence_id`.
