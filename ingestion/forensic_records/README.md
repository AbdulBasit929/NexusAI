# Forensic Records Ingestion Worker

Python 3.11 worker for Phase 1/2 Knowledge Base structured-record ingestion.

Key properties:

- NATS subscriber.
- Validates SHA-256 before ingest.
- Uses bounded Polars profiling for schema validation.
- Routes files through typed adapters: CDR, IPDR, ANPR, subscriber, tower/location, transaction, access log, and generic CSV.
- Streams CSV rows into PostgreSQL through `COPY`.
- Inserts CDR into a TimescaleDB hypertable.
- Inserts non-CDR structured files into `forensic.generic_records`.
- Indexes normalized entities in `forensic.record_entities` for cross-record correlation.
- Materializes `forensic.kb_active_metadata`.
- Materializes `forensic.kb_collection_assets` so KB remains the analyst-facing feature.
- Exposes Prometheus metrics on port `9109`.

Validate the attached sample profile without starting services:

```powershell
python ingestion/forensic_records/worker.py validate-sample "C:\Users\W S Mughal\Downloads\923461678183.csv"
```

Run the standard-library test:

```powershell
python -m unittest ingestion.forensic_records.tests.test_attached_cdr_sample
```
