#!/usr/bin/env python3
"""Forensic records ingestion worker.

Subscribes to NATS jobs emitted by api/forensic_records, routes files through
typed adapters, and materializes Knowledge Base metadata plus deterministic
record stores for exact analytics.
"""

from __future__ import annotations

import asyncio
import csv
import hashlib
import json
import logging
import os
import re
import signal
import sys
import tempfile
from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Any, Callable, Iterable

try:
    import polars as pl
except ModuleNotFoundError:  # pragma: no cover - allows pure adapter tests without worker deps
    pl = None  # type: ignore[assignment]

try:
    import psycopg
    from psycopg import sql
except ModuleNotFoundError:  # pragma: no cover - allows pure adapter tests without worker deps
    psycopg = None  # type: ignore[assignment]
    sql = None  # type: ignore[assignment]

try:
    from prometheus_client import Counter, Histogram, start_http_server
except ModuleNotFoundError:  # pragma: no cover - allows pure adapter tests without worker deps
    class _NoopMetric:
        def labels(self, *_: Any) -> "_NoopMetric":
            return self

        def inc(self) -> None:
            return None

        def time(self) -> "_NoopMetric":
            return self

        def __enter__(self) -> "_NoopMetric":
            return self

        def __exit__(self, *_: Any) -> None:
            return None

    def Counter(*_: Any, **__: Any) -> _NoopMetric:  # type: ignore[no-redef]
        return _NoopMetric()

    def Histogram(*_: Any, **__: Any) -> _NoopMetric:  # type: ignore[no-redef]
        return _NoopMetric()

    def start_http_server(*_: Any, **__: Any) -> None:  # type: ignore[no-redef]
        return None

LOG = logging.getLogger("forensic-records-worker")

INGEST_SUBJECT = os.getenv("NATS_RECORDS_SUBJECT", "forensic.records.ingest.requested")
PROGRESS_SUBJECT = os.getenv("NATS_RECORDS_PROGRESS_SUBJECT", "forensic.records.ingest.progress")
NATS_URL = os.getenv("NATS_URL", "nats://nats:4222")
DATABASE_URL = os.getenv("FORENSIC_DATABASE_URL", "postgresql://localrecall:localrecall@postgres:5432/localrecall")
PROMETHEUS_ADDR = int(os.getenv("FORENSIC_WORKER_METRICS_PORT", "9109"))
TENANT_SETTING = "app.tenant_id"

JOBS_TOTAL = Counter("forensic_records_jobs_total", "Ingest jobs processed", ["record_type", "status"])
ROWS_TOTAL = Counter("forensic_records_rows_total", "Rows observed by worker", ["record_type", "status"])
INGEST_JOBS_COMPLETED_TOTAL = Counter("forensic_ingest_jobs_completed_total", "Completed ingest jobs", ["record_type"])
INGEST_ROWS_TOTAL = Counter("forensic_ingest_rows_total", "Rows processed by ingest worker", ["record_type", "status"])
INGEST_SECONDS = Histogram("forensic_records_ingest_seconds", "Wall time per ingest job", ["record_type"])

CDR_REQUIRED_COLUMNS = {
    "MSISDN",
    "call_org_num",
    "CALL_DIALED_NUM",
    "IMSI",
    "IMEI",
    "CALL_START_DT_TM",
    "CALL_END_DT_TM",
    "INBOUND_OUTBOUND_IND",
    "Call_Network_Volume",
    "Lac_Id",
    "Site_Id",
    "Cell_SITE_ID",
    "lat",
    "longitude",
    "CALL_TYPE",
    "location",
}

GENERIC_COPY_COLUMNS = [
    "tenant_id",
    "collection_id",
    "file_id",
    "batch_id",
    "record_type",
    "row_number",
    "row_hash",
    "observed_at",
    "primary_entity",
    "secondary_entity",
    "location",
    "latitude",
    "longitude",
    "raw_record",
    "source_file",
]

COPY_COLUMNS = [
    "tenant_id",
    "collection_id",
    "file_id",
    "batch_id",
    "row_number",
    "row_hash",
    "msisdn",
    "call_org_num",
    "call_dialed_num",
    "imsi",
    "imei",
    "call_start_ts",
    "call_end_ts",
    "duration_seconds",
    "direction",
    "network_volume",
    "lac_id",
    "site_id",
    "cell_site_id",
    "latitude",
    "longitude",
    "call_type",
    "location",
    "raw_record",
    "source_file",
]

IDENTIFIER_PATTERNS: dict[str, re.Pattern[str]] = {
    "email": re.compile(r"(?i)^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$"),
    "ipv4": re.compile(r"^(?:\d{1,3}\.){3}\d{1,3}$"),
    "ipv6": re.compile(r"(?i)^(?:[a-f0-9]{0,4}:){2,7}[a-f0-9]{0,4}$"),
    "phone": re.compile(r"^\+?\d(?:[\s().\-]?\d){9,18}$"),
    "imei_imsi": re.compile(r"^\d{14,15}$"),
    "vehicle": re.compile(r"(?i)^[A-Z0-9][A-Z0-9\- ]{3,12}[A-Z0-9]$"),
}

TIMESTAMP_HINTS = {
    "timestamp",
    "time",
    "date",
    "datetime",
    "event_time",
    "observed_at",
    "created_at",
    "updated_at",
    "call_start_dt_tm",
    "call_start_time",
    "session_start",
    "capture_time",
}


@dataclass(frozen=True)
class RecordAdapter:
    name: str
    required_any: tuple[tuple[str, ...], ...]
    timestamp_fields: tuple[str, ...]
    primary_fields: tuple[str, ...]
    secondary_fields: tuple[str, ...]
    location_fields: tuple[str, ...]
    latitude_fields: tuple[str, ...] = ("lat", "latitude", "y")
    longitude_fields: tuple[str, ...] = ("longitude", "lon", "lng", "x")

    def matches(self, headers: Iterable[str]) -> bool:
        normalized = {normalize_header(header) for header in headers}
        return all(any(normalize_header(alias) in normalized for alias in aliases) for aliases in self.required_any)


ADAPTERS: dict[str, RecordAdapter] = {
    "cdr": RecordAdapter(
        name="cdr",
        required_any=(("MSISDN",), ("CALL_START_DT_TM", "call_start_time", "timestamp"), ("CALL_DIALED_NUM", "dialed_number")),
        timestamp_fields=("CALL_START_DT_TM", "call_start_time", "timestamp", "start_time"),
        primary_fields=("MSISDN", "call_org_num", "source_number", "a_number"),
        secondary_fields=("CALL_DIALED_NUM", "dialed_number", "target_number", "b_number"),
        location_fields=("location", "Cell_SITE_ID", "cell_id", "site_id"),
    ),
    "ipdr": RecordAdapter(
        name="ipdr",
        required_any=(("source_ip", "src_ip", "ip_address"), ("destination_ip", "dst_ip", "dest_ip")),
        timestamp_fields=("timestamp", "session_start", "start_time", "event_time", "time"),
        primary_fields=("source_ip", "src_ip", "ip_address", "msisdn"),
        secondary_fields=("destination_ip", "dst_ip", "dest_ip", "domain", "url"),
        location_fields=("location", "cell_id", "site_id"),
    ),
    "anpr": RecordAdapter(
        name="anpr",
        required_any=(("plate", "plate_number", "license_plate", "registration_number"),),
        timestamp_fields=("timestamp", "event_time", "capture_time", "observed_at", "time"),
        primary_fields=("plate", "plate_number", "license_plate", "registration_number"),
        secondary_fields=("camera_id", "camera", "checkpoint", "lane"),
        location_fields=("location", "camera_location", "checkpoint", "site"),
    ),
    "subscriber": RecordAdapter(
        name="subscriber",
        required_any=(("msisdn", "phone_number", "subscriber_number"), ("cnic", "cnic_last4", "customer_id", "subscriber_id", "name", "subscriber_name", "customer_name")),
        timestamp_fields=("activation_date", "created_at", "updated_at", "timestamp"),
        primary_fields=("msisdn", "phone_number", "subscriber_number"),
        secondary_fields=("cnic", "cnic_last4", "customer_id", "subscriber_id", "name", "subscriber_name", "customer_name"),
        location_fields=("address", "city", "home_city", "billing_region", "location"),
    ),
    "tower_location": RecordAdapter(
        name="tower_location",
        required_any=(("cell_id", "cell_site_id", "site_id"), ("lat", "latitude"), ("longitude", "lon", "lng")),
        timestamp_fields=("timestamp", "updated_at", "created_at"),
        primary_fields=("cell_id", "cell_site_id", "site_id"),
        secondary_fields=("lac", "lac_id", "tower_id"),
        location_fields=("location", "address", "site_name"),
    ),
    "transaction": RecordAdapter(
        name="transaction",
        required_any=(("amount", "transaction_amount"), ("account", "account_number", "from_account", "sender")),
        timestamp_fields=("timestamp", "transaction_time", "transaction_date", "created_at"),
        primary_fields=("account", "account_number", "from_account", "sender"),
        secondary_fields=("counterparty", "to_account", "receiver", "merchant"),
        location_fields=("location", "merchant_location", "city"),
    ),
    "access_log": RecordAdapter(
        name="access_log",
        required_any=(("ip", "source_ip", "src_ip"), ("user", "username", "user_id", "path", "url")),
        timestamp_fields=("timestamp", "time", "event_time", "created_at"),
        primary_fields=("user", "username", "user_id", "source_ip", "src_ip", "ip"),
        secondary_fields=("path", "url", "endpoint", "status"),
        location_fields=("geo", "country", "city", "location"),
    ),
    "generic": RecordAdapter(
        name="generic",
        required_any=(),
        timestamp_fields=("timestamp", "time", "date", "created_at", "updated_at", "event_time"),
        primary_fields=("id", "msisdn", "phone", "plate", "source_ip", "account", "user"),
        secondary_fields=("target", "destination", "counterparty", "destination_ip", "name"),
        location_fields=("location", "address", "city", "site", "area"),
    ),
}


@dataclass(frozen=True)
class IngestJob:
    job_id: str
    tenant_id: str
    user_id: str
    collection_id: str
    file_id: str
    source_file: str
    spool_path: str
    sha256: str
    record_type: str
    headers: list[str]
    metadata: dict[str, str]
    request_id: str = ""
    evidence_id: str = ""

    @classmethod
    def from_message(cls, payload: bytes) -> "IngestJob":
        data = json.loads(payload)
        return cls(
            job_id=data["job_id"],
            evidence_id=data.get("evidence_id") or "",
            tenant_id=data.get("tenant_id") or "default",
            user_id=data.get("user_id") or "",
            collection_id=data["collection_id"],
            file_id=data["file_id"],
            source_file=data["source_file"],
            spool_path=data["spool_path"],
            sha256=data["sha256"],
            record_type=data.get("record_type") or data.get("detected_record_type") or "generic",
            headers=list(data.get("headers") or []),
            metadata=dict(data.get("metadata") or {}),
            request_id=data.get("request_id") or "",
        )


def main() -> None:
    require_runtime_dependencies()
    logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"), format="%(asctime)s %(levelname)s %(name)s %(message)s")
    start_http_server(PROMETHEUS_ADDR)
    asyncio.run(run_worker())


def require_runtime_dependencies() -> None:
    missing = []
    if pl is None:
        missing.append("polars")
    if psycopg is None or sql is None:
        missing.append("psycopg")
    if missing:
        raise RuntimeError(f"missing worker runtime dependencies: {', '.join(missing)}")


def log_progress_publish_error(done: Any) -> None:
    try:
        error = done.exception()
    except Exception as exc:  # noqa: BLE001 - callback guard
        error = exc
    if error is not None:
        LOG.debug("failed to publish ingest progress: %s", error)


async def run_worker() -> None:
    import nats

    stop = asyncio.Event()
    loop = asyncio.get_running_loop()

    def _stop(*_: object) -> None:
        stop.set()

    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            asyncio.get_running_loop().add_signal_handler(sig, _stop)
        except NotImplementedError:
            pass

    nc = await nats.connect(NATS_URL, name="forensic-records-ingestion-worker")

    async def handle_message(msg: Any) -> None:
        job = IngestJob.from_message(msg.data)
        if not job.request_id:
            job = IngestJob(
                job_id=job.job_id,
                evidence_id=job.evidence_id,
                tenant_id=job.tenant_id,
                user_id=job.user_id,
                collection_id=job.collection_id,
                file_id=job.file_id,
                source_file=job.source_file,
                spool_path=job.spool_path,
                sha256=job.sha256,
                record_type=job.record_type,
                headers=job.headers,
                metadata=job.metadata,
                request_id=getattr(msg, "headers", {}).get("X-Request-ID", "") if getattr(msg, "headers", None) else "",
            )

        def publish_progress(event: dict[str, Any]) -> None:
            payload = json.dumps(event, default=str, sort_keys=True).encode()
            future = asyncio.run_coroutine_threadsafe(nc.publish(PROGRESS_SUBJECT, payload), loop)
            future.add_done_callback(log_progress_publish_error)

        try:
            await asyncio.to_thread(process_job, job, publish_progress)
            JOBS_TOTAL.labels(job.record_type, "completed").inc()
            INGEST_JOBS_COMPLETED_TOTAL.labels(job.record_type).inc()
        except Exception as exc:  # noqa: BLE001 - top-level worker guard
            LOG.exception("ingest job failed", extra={"job_id": job.job_id})
            JOBS_TOTAL.labels(job.record_type, "failed").inc()
            await asyncio.to_thread(mark_job_failed, job, str(exc))

    await nc.subscribe(INGEST_SUBJECT, cb=handle_message)
    LOG.info("worker subscribed", extra={"subject": INGEST_SUBJECT, "nats_url": NATS_URL})
    await stop.wait()
    await nc.drain()


def process_job(job: IngestJob, progress_cb: Callable[[dict[str, Any]], None] | None = None) -> None:
    path = Path(job.spool_path)
    if not path.exists():
        raise FileNotFoundError(job.spool_path)
    assert_sha256(path, job.sha256)
    started_at = datetime.now(timezone.utc)

    with INGEST_SECONDS.labels(job.record_type).time():
        with psycopg.connect(DATABASE_URL, autocommit=False) as conn:
            conn.execute("SELECT set_config(%s, %s, true)", (TENANT_SETTING, job.tenant_id))
            ensure_job_row(conn, job)
            mark_job_running(conn, job)
            mark_evidence_processing(conn, job)
            profile = profile_records(path)
            adapter = resolve_adapter(job, profile["headers"])
            batch_id = job.job_id
            emit_progress(progress_cb, job, "started", empty_stats(), profile, started_at)
            with conn.cursor() as cur:
                ensure_dynamic_records_table(cur)
                materialize_kb_asset(cur, job, batch_id, profile, adapter, "pending", rag_status_from_job(job), {})
                if adapter.name == "cdr":
                    validate_cdr_headers(profile["headers"])
                    create_temp_cdr_stage(cur)
                    stats = copy_cdr_to_stage(cur, job, path, batch_id, profile, started_at, progress_cb)
                    inserted = insert_stage_into_cdr(cur)
                    inserted_dynamic = insert_stage_into_dynamic_from_cdr(cur, profile, job)
                    indexed_entities = index_cdr_entities(cur)
                else:
                    create_temp_generic_stage(cur)
                    stats = copy_generic_to_stage(cur, job, path, batch_id, adapter, profile, started_at, progress_cb)
                    inserted = insert_stage_into_generic(cur)
                    inserted_dynamic = insert_stage_into_dynamic_from_generic(cur, profile, job)
                    indexed_entities = index_generic_entities(cur)
                materialize_metadata(cur, job, batch_id, profile, stats, inserted, adapter.name)
                mark_job_completed(cur, job, batch_id, stats, inserted)
                mark_evidence_completed(cur, job, batch_id, adapter, profile, stats, inserted, inserted_dynamic, indexed_entities)
                materialize_kb_asset(
                    cur,
                    job,
                    batch_id,
                    profile,
                    adapter,
                    "completed",
                    rag_status_from_job(job),
                    {
                        "total_rows": stats["total_rows"],
                        "inserted_rows": inserted,
                        "canonical_rows": inserted_dynamic,
                        "duplicate_rows": max(0, int(stats["total_rows"]) - int(stats["rejected_rows"]) - inserted),
                        "rejected_rows": stats["rejected_rows"],
                        "indexed_entities": indexed_entities,
                        "detected_schema": profile.get("classification", {}),
                    },
                )
                audit(
                    cur,
                    job,
                    "records.ingest.completed",
                    {
                        "batch_id": batch_id,
                        "record_type": adapter.name,
                        "inserted_rows": inserted,
                        "canonical_rows": inserted_dynamic,
                        "indexed_entities": indexed_entities,
                        "request_id": job.request_id,
                        "evidence_id": job.evidence_id,
                    },
                )
            conn.commit()
            emit_progress(progress_cb, job, "completed", stats, profile, started_at)
            LOG.info(
                "ingest completed",
                extra={"job_id": job.job_id, "record_type": adapter.name, "inserted_rows": inserted, "total_rows": stats["total_rows"]},
            )


def assert_sha256(path: Path, expected: str) -> None:
    digest = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1024 * 1024), b""):
            digest.update(chunk)
    actual = digest.hexdigest()
    if actual != expected:
        raise ValueError(f"sha256 mismatch for {path}: expected {expected}, got {actual}")


def ensure_job_row(conn: psycopg.Connection[Any], job: IngestJob) -> None:
    conn.execute(
        """
        INSERT INTO forensic.records_ingest_jobs (
          id, tenant_id, user_id, collection_id, file_id, source_file,
          spool_path, sha256, record_type, status, evidence_id, metadata
        )
        VALUES (%s::uuid, %s, %s, %s, %s, %s, %s, %s, %s::forensic_record_type, 'queued', NULLIF(%s, '')::uuid, %s::jsonb)
        ON CONFLICT (id) DO UPDATE SET
          evidence_id=coalesce(forensic.records_ingest_jobs.evidence_id, EXCLUDED.evidence_id),
          metadata=forensic.records_ingest_jobs.metadata || EXCLUDED.metadata
        """,
        (
            job.job_id,
            job.tenant_id,
            job.user_id,
            job.collection_id,
            job.file_id,
            job.source_file,
            job.spool_path,
            job.sha256,
            job.record_type,
            job.evidence_id,
            json.dumps(job.metadata),
        ),
    )


def mark_job_running(conn: psycopg.Connection[Any], job: IngestJob) -> None:
    conn.execute(
        """
        UPDATE forensic.records_ingest_jobs
        SET status='running', started_at=now(), attempt_count=attempt_count + 1, error_message=NULL
        WHERE id=%s::uuid
        """,
        (job.job_id,),
    )


def mark_evidence_processing(conn: psycopg.Connection[Any], job: IngestJob) -> None:
    if not job.evidence_id:
        return
    conn.execute(
        """
        UPDATE forensic.evidence_items
        SET processing_status='processing',
            records_batch_id=%s::uuid,
            metadata = metadata || %s::jsonb
        WHERE evidence_id=%s::uuid
        """,
        (
            job.job_id,
            json.dumps(
                {
                    "worker_started_at": datetime.now(timezone.utc).isoformat(),
                    "job_id": job.job_id,
                    "request_id": job.request_id,
                }
            ),
            job.evidence_id,
        ),
    )


def mark_job_failed(job: IngestJob, message: str) -> None:
    with psycopg.connect(DATABASE_URL, autocommit=True) as conn:
        conn.execute("SELECT set_config(%s, %s, true)", (TENANT_SETTING, job.tenant_id))
        ensure_job_row(conn, job)
        conn.execute(
            """
            UPDATE forensic.records_ingest_jobs
            SET status = (CASE WHEN attempt_count + 1 >= max_attempts THEN 'dead_letter' ELSE 'failed' END)::forensic_ingest_status,
                attempt_count = attempt_count + 1,
                error_message = %s,
                completed_at = now()
            WHERE id = %s::uuid
            """,
            (message[:4000], job.job_id),
        )
        if job.evidence_id:
            conn.execute(
                """
                UPDATE forensic.evidence_items
                SET processing_status='failed',
                    errors = errors || jsonb_build_array(jsonb_build_object('message', %s, 'recorded_at', now())),
                    metadata = metadata || %s::jsonb
                WHERE evidence_id=%s::uuid
                """,
                (
                    message[:4000],
                    json.dumps({"failed_job_id": job.job_id, "request_id": job.request_id}),
                    job.evidence_id,
                ),
            )



def profile_records(path: Path) -> dict[str, Any]:
    ext = path.suffix.lower()
    if ext in {".json", ".jsonl", ".ndjson"}:
        return profile_json(path)
    if ext == ".parquet":
        return profile_parquet(path)
    return profile_csv(path)


def profile_csv(path: Path) -> dict[str, Any]:
    if pl is None:
        with path.open(newline="", encoding="utf-8-sig") as fh:
            reader = csv.DictReader(fh)
            headers = list(reader.fieldnames or [])
            records: list[dict[str, Any]] = []
            for row in reader:
                records.append(dict(row))
                if len(records) >= 5000:
                    break
        classification = classify_columns(headers, records[:500])
        return {
            "headers": headers,
            "sample_rows": len(records),
            "schema": {name: "text" for name in headers},
            "sample_records": records[:500],
            "classification": classification,
        }
    sample = pl.read_csv(path, n_rows=5000, infer_schema_length=1000, ignore_errors=True)
    records = sample.head(500).to_dicts()
    headers = list(sample.columns)
    classification = classify_columns(headers, records)
    return {
        "headers": headers,
        "sample_rows": sample.height,
        "schema": {name: str(dtype) for name, dtype in sample.schema.items()},
        "sample_records": records,
        "classification": classification,
    }


def profile_cdr_csv(path: Path) -> dict[str, Any]:
    return profile_records(path)


def profile_json(path: Path) -> dict[str, Any]:
    records = []
    ext = path.suffix.lower()
    if ext == ".json":
        value = json.loads(path.read_text(encoding="utf-8-sig"))
        if isinstance(value, list):
            records = [item for item in value if isinstance(item, dict)]
        elif isinstance(value, dict):
            records = [value]
    else:
        with path.open(encoding="utf-8-sig") as fh:
            for line in fh:
                line = line.strip()
                if not line:
                    continue
                value = json.loads(line)
                if isinstance(value, dict):
                    records.append(value)
                if len(records) >= 5000:
                    break
    headers = ordered_headers(records)
    classification = classify_columns(headers, records[:500])
    return {
        "headers": headers,
        "sample_rows": len(records),
        "schema": {name: "json" for name in headers},
        "sample_records": records[:500],
        "classification": classification,
    }


def profile_parquet(path: Path) -> dict[str, Any]:
    if pl is None:
        raise RuntimeError("polars is required to profile Parquet files")
    sample = pl.read_parquet(path).head(5000)
    records = sample.head(500).to_dicts()
    headers = list(sample.columns)
    classification = classify_columns(headers, records)
    return {
        "headers": headers,
        "sample_rows": sample.height,
        "schema": {name: str(dtype) for name, dtype in sample.schema.items()},
        "sample_records": records,
        "classification": classification,
    }


def iter_records(path: Path) -> Iterable[dict[str, Any]]:
    ext = path.suffix.lower()
    if ext in {".json", ".jsonl", ".ndjson"}:
        yield from iter_json_records(path)
        return
    if ext == ".parquet":
        yield from iter_parquet_records(path)
        return
    with path.open(newline="", encoding="utf-8-sig") as fh:
        yield from csv.DictReader(fh)


def iter_json_records(path: Path) -> Iterable[dict[str, Any]]:
    ext = path.suffix.lower()
    if ext == ".json":
        value = json.loads(path.read_text(encoding="utf-8-sig"))
        if isinstance(value, list):
            for item in value:
                if isinstance(item, dict):
                    yield item
        elif isinstance(value, dict):
            yield value
        return
    with path.open(encoding="utf-8-sig") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            value = json.loads(line)
            if isinstance(value, dict):
                yield value


def iter_parquet_records(path: Path) -> Iterable[dict[str, Any]]:
    if pl is None:
        raise RuntimeError("polars is required to read Parquet files")
    frame = pl.read_parquet(path)
    yield from frame.iter_rows(named=True)


def ordered_headers(records: Iterable[dict[str, Any]]) -> list[str]:
    headers: list[str] = []
    seen: set[str] = set()
    for record in records:
        for key in record:
            if key not in seen:
                seen.add(key)
                headers.append(key)
    return headers


def classify_columns(headers: Iterable[str], records: Iterable[dict[str, Any]]) -> dict[str, Any]:
    rows = list(records)
    columns: dict[str, dict[str, Any]] = {}
    by_role: dict[str, list[str]] = {}
    for header in headers:
        values = [clean_text(row.get(header)) for row in rows if clean_text(row.get(header))]
        role_scores = score_column(header, values[:200])
        role = max(role_scores, key=role_scores.get) if role_scores else "text"
        confidence = role_scores.get(role, 0.0)
        if confidence < 0.3:
            role = "text"
        columns[header] = {
            "normalized": normalize_header(header),
            "role": role,
            "confidence": round(confidence, 3),
            "sampled_non_empty": len(values),
        }
        by_role.setdefault(role, []).append(header)
    return {
        "columns": columns,
        "roles": by_role,
        "primary_target": first_role_column(by_role, "phone", "email", "ip", "imei_imsi", "vehicle", "identifier"),
        "secondary_target": second_role_column(by_role, "phone", "email", "ip", "imei_imsi", "vehicle", "identifier"),
        "timestamp": first_role_column(by_role, "timestamp"),
    }


def score_column(header: str, values: list[str]) -> dict[str, float]:
    if not values:
        return {}
    normalized = normalize_header(header)
    scores = {
        "timestamp": 0.15 if any(hint in normalized for hint in TIMESTAMP_HINTS) else 0.0,
        "phone": 0.2 if any(term in normalized for term in ("msisdn", "phone", "mobile", "caller", "callee", "dialed")) else 0.0,
        "ip": 0.2 if "ip" in normalized else 0.0,
        "email": 0.2 if "email" in normalized or "mail" in normalized else 0.0,
        "imei_imsi": 0.2 if "imei" in normalized or "imsi" in normalized else 0.0,
        "vehicle": 0.2 if any(term in normalized for term in ("plate", "vehicle", "registration")) else 0.0,
        "identifier": 0.1 if any(term in normalized for term in ("id", "target", "entity", "account", "user")) else 0.0,
    }
    total = len(values)
    for value in values:
        if parse_ts_any(value) is not None or looks_like_epoch(value):
            scores["timestamp"] += 1 / total
        if IDENTIFIER_PATTERNS["email"].match(value):
            scores["email"] += 1 / total
        if IDENTIFIER_PATTERNS["ipv4"].match(value) or IDENTIFIER_PATTERNS["ipv6"].match(value):
            scores["ip"] += 1 / total
        digits = re.sub(r"\D", "", value)
        if 10 <= len(digits) <= 15 and IDENTIFIER_PATTERNS["phone"].match(value):
            scores["phone"] += 1 / total
        if IDENTIFIER_PATTERNS["imei_imsi"].match(digits):
            scores["imei_imsi"] += 0.85 / total
        if IDENTIFIER_PATTERNS["vehicle"].match(value) and not value.isdigit():
            scores["vehicle"] += 0.7 / total
    return scores


def looks_like_epoch(value: str) -> bool:
    text = clean_text(value)
    if not re.fullmatch(r"\d{10}|\d{13}", text):
        return False
    try:
        parse_epoch(text)
        return True
    except ValueError:
        return False


def parse_epoch(value: str) -> datetime:
    number = int(value)
    if len(value) == 13:
        number = number / 1000
    return datetime.fromtimestamp(number, tz=timezone.utc)


def first_role_column(by_role: dict[str, list[str]], *roles: str) -> str:
    for role in roles:
        values = by_role.get(role) or []
        if values:
            return values[0]
    return ""


def second_role_column(by_role: dict[str, list[str]], *roles: str) -> str:
    seen_first = False
    for role in roles:
        for value in by_role.get(role) or []:
            if not seen_first:
                seen_first = True
                continue
            return value
    return ""


def resolve_adapter(job: IngestJob, headers: Iterable[str]) -> RecordAdapter:
    requested = normalize_header(job.record_type)
    if requested in ADAPTERS and requested != "generic":
        adapter = ADAPTERS[requested]
        if adapter.matches(headers):
            return adapter
        raise ValueError(f"{requested} adapter selected but required columns are missing")
    for name, adapter in ADAPTERS.items():
        if name == "generic":
            continue
        if adapter.matches(headers):
            return adapter
    return ADAPTERS["generic"]


def validate_cdr_headers(headers: Iterable[str]) -> None:
    missing = CDR_REQUIRED_COLUMNS.difference(set(headers))
    if missing:
        raise ValueError(f"missing required CDR columns: {', '.join(sorted(missing))}")


def create_temp_cdr_stage(cur: psycopg.Cursor[Any]) -> None:
    cur.execute("DROP TABLE IF EXISTS tmp_cdr_records")
    cur.execute(
        """
        CREATE TEMP TABLE tmp_cdr_records (
          tenant_id text,
          collection_id text,
          file_id text,
          batch_id uuid,
          row_number bigint,
          row_hash text,
          msisdn text,
          call_org_num text,
          call_dialed_num text,
          imsi text,
          imei text,
          call_start_ts timestamptz,
          call_end_ts timestamptz,
          duration_seconds integer,
          direction text,
          network_volume numeric,
          lac_id text,
          site_id text,
          cell_site_id text,
          latitude double precision,
          longitude double precision,
          call_type text,
          location text,
          raw_record jsonb,
          source_file text
        ) ON COMMIT DROP
        """
    )


def copy_cdr_to_stage(
    cur: psycopg.Cursor[Any],
    job: IngestJob,
    path: Path,
    batch_id: str,
    profile: dict[str, Any] | None = None,
    started_at: datetime | None = None,
    progress_cb: Callable[[dict[str, Any]], None] | None = None,
) -> dict[str, Any]:
    stats: dict[str, Any] = {
        "total_rows": 0,
        "rejected_rows": 0,
        "min_timestamp": None,
        "max_timestamp": None,
        "targets": set(),
        "originators": set(),
        "locations": set(),
        "call_types": {},
    }
    rejects: list[tuple[int, str, str, dict[str, str]]] = []
    copy_sql = sql.SQL("COPY tmp_cdr_records ({}) FROM STDIN").format(
        sql.SQL(", ").join(sql.Identifier(col) for col in COPY_COLUMNS)
    )
    with cur.copy(copy_sql) as copy:
        for line_number, raw in enumerate(iter_records(path), start=2):
            stats["total_rows"] += 1
            try:
                normalized = normalize_cdr_row(job, raw, batch_id, line_number)
                copy.write_row([normalized[col] for col in COPY_COLUMNS])
                update_stats(stats, normalized)
                ROWS_TOTAL.labels("cdr", "accepted").inc()
                INGEST_ROWS_TOTAL.labels("cdr", "accepted").inc()
            except Exception as exc:  # noqa: BLE001 - row-level rejection
                stats["rejected_rows"] += 1
                ROWS_TOTAL.labels("cdr", "rejected").inc()
                INGEST_ROWS_TOTAL.labels("cdr", "rejected").inc()
                rejects.append((line_number, "cdr_row_parse_error", str(exc), raw))
            if stats["total_rows"] % 1000 == 0:
                emit_progress(progress_cb, job, "running", stats, profile or {}, started_at)
    for line_number, code, message, raw in rejects:
        write_reject(cur, job, line_number, code, message, raw)
    return stats


def normalize_cdr_row(job: IngestJob, raw: dict[str, str], batch_id: str, row_number: int) -> dict[str, Any]:
    start = parse_required_ts(first_value(raw, ("CALL_START_DT_TM", "call_start_time", "timestamp", "start_time")))
    end = parse_ts_any(first_value(raw, ("CALL_END_DT_TM", "call_end_time", "end_time")))
    duration = int((end - start).total_seconds()) if start and end else None
    canonical = {
        "tenant_id": job.tenant_id,
        "collection_id": job.collection_id,
        "file_id": job.file_id,
        "batch_id": batch_id,
        "row_number": row_number,
        "msisdn": first_value(raw, ("MSISDN", "msisdn")),
        "call_org_num": first_value(raw, ("call_org_num", "source_number", "a_number")),
        "call_dialed_num": first_value(raw, ("CALL_DIALED_NUM", "dialed_number", "target_number", "b_number")),
        "imsi": first_value(raw, ("IMSI", "imsi")),
        "imei": first_value(raw, ("IMEI", "imei")),
        "call_start_ts": start,
        "call_end_ts": end,
        "duration_seconds": duration,
        "direction": clean_text(first_value(raw, ("INBOUND_OUTBOUND_IND", "direction"))).upper() or None,
        "network_volume": parse_decimal(first_value(raw, ("Call_Network_Volume", "network_volume", "bytes"))),
        "lac_id": clean_numeric_identifier(first_value(raw, ("Lac_Id", "lac_id", "lac"))),
        "site_id": clean_numeric_identifier(first_value(raw, ("Site_Id", "site_id"))),
        "cell_site_id": clean_numeric_identifier(first_value(raw, ("Cell_SITE_ID", "cell_site_id", "cell_id"))),
        "latitude": parse_float(first_value(raw, ("lat", "latitude"))),
        "longitude": parse_float(first_value(raw, ("longitude", "lon", "lng"))),
        "call_type": clean_text(first_value(raw, ("CALL_TYPE", "call_type"))).upper() or None,
        "location": clean_text(first_value(raw, ("location", "site_name"))),
        "raw_record": json.dumps(raw, ensure_ascii=False),
        "source_file": job.source_file,
    }
    canonical["row_hash"] = row_hash(canonical)
    return canonical


def parse_ts(value: str | None) -> datetime:
    text = clean_text(value)
    if not text:
        raise ValueError("timestamp is required")
    return datetime.strptime(text, "%Y-%m-%d %H:%M:%S").replace(tzinfo=timezone.utc)


def parse_required_ts(value: str | None) -> datetime:
    parsed = parse_ts_any(value)
    if parsed is None:
        raise ValueError("timestamp is required")
    return parsed


def parse_ts_any(value: str | None) -> datetime | None:
    text = clean_text(value)
    if not text:
        return None
    if looks_like_epoch(text):
        try:
            return parse_epoch(text)
        except (OverflowError, OSError, ValueError):
            return None
    formats = (
        "%Y-%m-%d %H:%M:%S",
        "%Y-%m-%dT%H:%M:%S",
        "%Y-%m-%dT%H:%M:%SZ",
        "%d/%m/%Y %H:%M:%S",
        "%m/%d/%Y %H:%M:%S",
        "%Y-%m-%d",
        "%d/%m/%Y",
        "%m/%d/%Y",
    )
    for fmt in formats:
        try:
            return datetime.strptime(text, fmt).replace(tzinfo=timezone.utc)
        except ValueError:
            continue
    try:
        parsed = datetime.fromisoformat(text.replace("Z", "+00:00"))
    except ValueError:
        return None
    if parsed.tzinfo is None:
        return parsed.replace(tzinfo=timezone.utc)
    return parsed.astimezone(timezone.utc)


def parse_decimal(value: str | None) -> Decimal | None:
    text = clean_text(value)
    if not text:
        return None
    try:
        return Decimal(text)
    except InvalidOperation as exc:
        raise ValueError(f"invalid decimal {text!r}") from exc


def parse_float(value: str | None) -> float | None:
    text = clean_text(value)
    if not text:
        return None
    return float(text)


def parse_float_optional(value: str | None) -> float | None:
    try:
        return parse_float(value)
    except ValueError:
        return None


def parse_int_optional(value: str | None) -> int | None:
    text = clean_text(value)
    if not text:
        return None
    try:
        return int(text)
    except ValueError:
        return None


def clean_text(value: str | None) -> str:
    return "" if value is None else str(value).strip()


def normalize_header(value: str) -> str:
    text = clean_text(value).lower()
    chars: list[str] = []
    last_underscore = False
    for char in text:
        if char.isalnum():
            chars.append(char)
            last_underscore = False
            continue
        if not last_underscore:
            chars.append("_")
            last_underscore = True
    return "".join(chars).strip("_")


def first_value(raw: dict[str, str], candidates: Iterable[str]) -> str | None:
    by_normalized = {normalize_header(key): value for key, value in raw.items()}
    for candidate in candidates:
        value = by_normalized.get(normalize_header(candidate))
        if clean_text(value):
            return clean_text(value)
    return None


def value_from_field(raw: dict[str, Any], field: str) -> str | None:
    if not field:
        return None
    wanted = normalize_header(field)
    for key, value in raw.items():
        if normalize_header(key) == wanted and clean_text(value):
            return clean_text(value)
    return None


def normalize_record_type(value: str) -> str:
    normalized = normalize_header(value)
    return normalized if normalized in ADAPTERS else "generic"


def rag_status_from_job(job: IngestJob) -> str:
    status = clean_text(job.metadata.get("kb_mirror_status")).lower()
    if status == "mirrored":
        return "mirrored"
    if status == "failed":
        return "failed"
    if status == "disabled":
        return "skipped"
    return "pending"


def clean_numeric_identifier(value: str | None) -> str | None:
    text = clean_text(value)
    if not text:
        return None
    if text.endswith(".0"):
        return text[:-2]
    return text


def row_hash(row: dict[str, Any]) -> str:
    keys = [
        "msisdn",
        "call_org_num",
        "call_dialed_num",
        "imsi",
        "imei",
        "call_start_ts",
        "call_end_ts",
        "direction",
        "network_volume",
        "lac_id",
        "site_id",
        "cell_site_id",
        "latitude",
        "longitude",
        "call_type",
        "location",
    ]
    payload = {key: str(row.get(key) or "") for key in keys}
    return hashlib.sha256(json.dumps(payload, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


def update_stats(stats: dict[str, Any], row: dict[str, Any]) -> None:
    ts = row["call_start_ts"]
    stats["min_timestamp"] = ts if stats["min_timestamp"] is None else min(stats["min_timestamp"], ts)
    stats["max_timestamp"] = ts if stats["max_timestamp"] is None else max(stats["max_timestamp"], ts)
    if row["call_dialed_num"]:
        stats["targets"].add(row["call_dialed_num"])
    if row["call_org_num"]:
        stats["originators"].add(row["call_org_num"])
    if row["location"]:
        stats["locations"].add(row["location"])
    call_type = row["call_type"] or "UNKNOWN"
    stats["call_types"][call_type] = stats["call_types"].get(call_type, 0) + 1


def write_reject(cur: psycopg.Cursor[Any], job: IngestJob, row_number: int, code: str, message: str, raw: dict[str, str]) -> None:
    cur.execute(
        """
        INSERT INTO forensic.records_ingest_errors (
          job_id, tenant_id, collection_id, file_id, row_number, error_code, error_message, raw_record
        )
        VALUES (%s::uuid, %s, %s, %s, %s, %s, %s, %s::jsonb)
        """,
        (job.job_id, job.tenant_id, job.collection_id, job.file_id, row_number, code, message[:2000], json.dumps(raw)),
    )


def insert_stage_into_cdr(cur: psycopg.Cursor[Any]) -> int:
    cur.execute(
        """
        INSERT INTO forensic.cdr_records (
          tenant_id, collection_id, file_id, batch_id, row_number, row_hash,
          msisdn, call_org_num, call_dialed_num, imsi, imei,
          call_start_ts, call_end_ts, duration_seconds, direction,
          network_volume, lac_id, site_id, cell_site_id, latitude, longitude,
          call_type, location, raw_record, source_file
        )
        SELECT
          tenant_id, collection_id, file_id, batch_id, row_number, row_hash,
          msisdn, call_org_num, call_dialed_num, imsi, imei,
          call_start_ts, call_end_ts, duration_seconds, direction,
          network_volume, lac_id, site_id, cell_site_id, latitude, longitude,
          call_type, location, raw_record, source_file
        FROM tmp_cdr_records
        ON CONFLICT DO NOTHING
        """
    )
    return cur.rowcount


def create_temp_generic_stage(cur: psycopg.Cursor[Any]) -> None:
    cur.execute("DROP TABLE IF EXISTS tmp_generic_records")
    cur.execute(
        """
        CREATE TEMP TABLE tmp_generic_records (
          tenant_id text,
          collection_id text,
          file_id text,
          batch_id uuid,
          record_type text,
          row_number bigint,
          row_hash text,
          observed_at timestamptz,
          primary_entity text,
          secondary_entity text,
          location text,
          latitude double precision,
          longitude double precision,
          raw_record jsonb,
          source_file text
        ) ON COMMIT DROP
        """
    )


def copy_generic_to_stage(
    cur: psycopg.Cursor[Any],
    job: IngestJob,
    path: Path,
    batch_id: str,
    adapter: RecordAdapter,
    profile: dict[str, Any] | None = None,
    started_at: datetime | None = None,
    progress_cb: Callable[[dict[str, Any]], None] | None = None,
) -> dict[str, Any]:
    stats: dict[str, Any] = {
        "total_rows": 0,
        "rejected_rows": 0,
        "min_timestamp": None,
        "max_timestamp": None,
        "targets": set(),
        "originators": set(),
        "locations": set(),
        "call_types": {},
    }
    rejects: list[tuple[int, str, str, dict[str, str]]] = []
    copy_sql = sql.SQL("COPY tmp_generic_records ({}) FROM STDIN").format(
        sql.SQL(", ").join(sql.Identifier(col) for col in GENERIC_COPY_COLUMNS)
    )
    with cur.copy(copy_sql) as copy:
        for line_number, raw in enumerate(iter_records(path), start=2):
            stats["total_rows"] += 1
            try:
                normalized = normalize_generic_row(job, raw, batch_id, line_number, adapter, profile)
                copy.write_row([normalized[col] for col in GENERIC_COPY_COLUMNS])
                update_generic_stats(stats, normalized)
                ROWS_TOTAL.labels(adapter.name, "accepted").inc()
                INGEST_ROWS_TOTAL.labels(adapter.name, "accepted").inc()
            except Exception as exc:  # noqa: BLE001 - row-level rejection
                stats["rejected_rows"] += 1
                ROWS_TOTAL.labels(adapter.name, "rejected").inc()
                INGEST_ROWS_TOTAL.labels(adapter.name, "rejected").inc()
                rejects.append((line_number, "generic_row_parse_error", str(exc), raw))
            if stats["total_rows"] % 1000 == 0:
                emit_progress(progress_cb, job, "running", stats, profile or {}, started_at)
    for line_number, code, message, raw in rejects:
        write_reject(cur, job, line_number, code, message, raw)
    return stats


def normalize_generic_row(
    job: IngestJob,
    raw: dict[str, Any],
    batch_id: str,
    row_number: int,
    adapter: RecordAdapter,
    profile: dict[str, Any] | None = None,
) -> dict[str, Any]:
    classification = (profile or {}).get("classification") or {}
    timestamp_field = classification.get("timestamp") or ""
    primary_field = classification.get("primary_target") or ""
    secondary_field = classification.get("secondary_target") or ""
    canonical = {
        "tenant_id": job.tenant_id,
        "collection_id": job.collection_id,
        "file_id": job.file_id,
        "batch_id": batch_id,
        "record_type": adapter.name,
        "row_number": row_number,
        "observed_at": parse_ts_any(value_from_field(raw, timestamp_field) or first_value(raw, adapter.timestamp_fields)),
        "primary_entity": value_from_field(raw, primary_field) or first_value(raw, adapter.primary_fields),
        "secondary_entity": value_from_field(raw, secondary_field) or first_value(raw, adapter.secondary_fields),
        "location": first_value(raw, adapter.location_fields),
        "latitude": parse_float_optional(first_value(raw, adapter.latitude_fields)),
        "longitude": parse_float_optional(first_value(raw, adapter.longitude_fields)),
        "raw_record": json.dumps(raw, ensure_ascii=False),
        "source_file": job.source_file,
    }
    canonical["row_hash"] = hashlib.sha256(
        json.dumps(raw, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()
    ).hexdigest()
    return canonical


def insert_stage_into_generic(cur: psycopg.Cursor[Any]) -> int:
    cur.execute(
        """
        INSERT INTO forensic.generic_records (
          tenant_id, collection_id, file_id, batch_id, record_type, row_number,
          row_hash, observed_at, primary_entity, secondary_entity, location,
          latitude, longitude, raw_record, source_file
        )
        SELECT
          tenant_id, collection_id, file_id, batch_id, record_type::forensic_record_type,
          row_number, row_hash, observed_at, primary_entity, secondary_entity,
          location, latitude, longitude, raw_record, source_file
        FROM tmp_generic_records
        ON CONFLICT DO NOTHING
        """
    )
    return cur.rowcount


def ensure_dynamic_records_table(cur: psycopg.Cursor[Any]) -> None:
    cur.execute(
        """
        CREATE TABLE IF NOT EXISTS forensic.records (
          record_id uuid NOT NULL DEFAULT gen_random_uuid(),
          tenant_id varchar NOT NULL DEFAULT 'default',
          collection_id varchar NOT NULL,
          file_id varchar NOT NULL,
          batch_id uuid NOT NULL,
          record_type varchar NOT NULL DEFAULT 'generic',
          "timestamp" timestamptz NOT NULL DEFAULT now(),
          primary_target varchar,
          secondary_target varchar,
          source_file varchar NOT NULL,
          row_number bigint NOT NULL,
          row_hash varchar NOT NULL,
          raw_payload jsonb NOT NULL,
          metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
          ingested_at timestamptz NOT NULL DEFAULT now(),
          PRIMARY KEY (record_id, "timestamp"),
          UNIQUE (tenant_id, collection_id, file_id, batch_id, row_hash, "timestamp")
        )
        """
    )
    cur.execute(
        """
        SELECT create_hypertable(
          'forensic.records',
          'timestamp',
          if_not_exists => TRUE,
          chunk_time_interval => INTERVAL '7 days'
        )
        """
    )
    cur.execute(
        """
        CREATE INDEX IF NOT EXISTS forensic_records_lookup_idx
          ON forensic.records (tenant_id, collection_id, record_type, "timestamp" DESC, primary_target)
        """
    )
    cur.execute(
        """
        CREATE INDEX IF NOT EXISTS forensic_records_secondary_target_idx
          ON forensic.records (tenant_id, collection_id, secondary_target, "timestamp" DESC)
        """
    )
    cur.execute(
        """
        CREATE INDEX IF NOT EXISTS forensic_records_payload_gin_idx
          ON forensic.records USING gin (raw_payload)
        """
    )
    cur.execute("ALTER TABLE forensic.records ENABLE ROW LEVEL SECURITY")
    cur.execute(
        """
        DO $$
        BEGIN
          IF NOT EXISTS (
            SELECT 1
            FROM pg_policies
            WHERE schemaname = 'forensic'
              AND tablename = 'records'
              AND policyname = 'tenant_isolation_records'
          ) THEN
            CREATE POLICY tenant_isolation_records ON forensic.records
              USING (tenant_id = current_setting('app.tenant_id', true));
          END IF;
        END $$
        """
    )


def insert_stage_into_dynamic_from_cdr(cur: psycopg.Cursor[Any], profile: dict[str, Any], job: IngestJob) -> int:
    cur.execute(
        """
        INSERT INTO forensic.records (
          tenant_id, collection_id, file_id, batch_id, record_type, "timestamp",
          primary_target, secondary_target, source_file, row_number, row_hash,
          raw_payload, evidence_id, metadata
        )
        SELECT
          tenant_id, collection_id, file_id, batch_id, 'cdr', call_start_ts,
          coalesce(nullif(msisdn, ''), nullif(call_org_num, '')),
          nullif(call_dialed_num, ''),
          source_file, row_number, row_hash, raw_record, NULLIF(%s, '')::uuid,
          jsonb_build_object(
            'schema_classification', %s::jsonb,
            'dynamic_store', 'forensic.records',
            'legacy_store', 'forensic.cdr_records',
            'normalized_fields', jsonb_strip_nulls(jsonb_build_object(
              'msisdn', msisdn,
              'call_org_num', call_org_num,
              'call_dialed_num', call_dialed_num,
              'imsi', imsi,
              'imei', imei,
              'call_start_ts', call_start_ts,
              'call_end_ts', call_end_ts,
              'duration_seconds', duration_seconds,
              'direction', direction,
              'network_volume', network_volume,
              'lac_id', lac_id,
              'site_id', site_id,
              'cell_site_id', cell_site_id,
              'latitude', latitude,
              'longitude', longitude,
              'call_type', call_type,
              'location', location
            )),
            'source_fields', jsonb_build_object(
              'timestamp', 'call_start_ts',
              'primary_target', 'msisdn',
              'secondary_target', 'call_dialed_num'
            )
          )
        FROM tmp_cdr_records
        ON CONFLICT DO NOTHING
        """,
        (job.evidence_id, json.dumps(profile.get("classification", {}))),
    )
    return cur.rowcount


def insert_stage_into_dynamic_from_generic(cur: psycopg.Cursor[Any], profile: dict[str, Any], job: IngestJob) -> int:
    cur.execute(
        """
        INSERT INTO forensic.records (
          tenant_id, collection_id, file_id, batch_id, record_type, "timestamp",
          primary_target, secondary_target, source_file, row_number, row_hash,
          raw_payload, evidence_id, metadata
        )
        SELECT
          tenant_id, collection_id, file_id, batch_id, record_type, coalesce(observed_at, now()),
          nullif(primary_entity, ''), nullif(secondary_entity, ''),
          source_file, row_number, row_hash, raw_record, NULLIF(%s, '')::uuid,
          jsonb_build_object(
            'schema_classification', %s::jsonb,
            'dynamic_store', 'forensic.records',
            'legacy_store', 'forensic.generic_records',
            'normalized_fields', jsonb_strip_nulls(jsonb_build_object(
              'record_type', record_type::text,
              'observed_at', observed_at,
              'primary_entity', primary_entity,
              'secondary_entity', secondary_entity,
              'location', location,
              'latitude', latitude,
              'longitude', longitude
            )),
            'source_fields', jsonb_build_object(
              'timestamp', %s,
              'primary_target', %s,
              'secondary_target', %s
            )
          )
        FROM tmp_generic_records
        ON CONFLICT DO NOTHING
        """,
        (
            job.evidence_id,
            json.dumps(profile.get("classification", {})),
            (profile.get("classification") or {}).get("timestamp") or "",
            (profile.get("classification") or {}).get("primary_target") or "",
            (profile.get("classification") or {}).get("secondary_target") or "",
        ),
    )
    return cur.rowcount


def update_generic_stats(stats: dict[str, Any], row: dict[str, Any]) -> None:
    ts = row["observed_at"]
    if ts is not None:
        stats["min_timestamp"] = ts if stats["min_timestamp"] is None else min(stats["min_timestamp"], ts)
        stats["max_timestamp"] = ts if stats["max_timestamp"] is None else max(stats["max_timestamp"], ts)
    if row["secondary_entity"]:
        stats["targets"].add(row["secondary_entity"])
    if row["primary_entity"]:
        stats["originators"].add(row["primary_entity"])
    if row["location"]:
        stats["locations"].add(row["location"])


def empty_stats() -> dict[str, Any]:
    return {
        "total_rows": 0,
        "rejected_rows": 0,
        "min_timestamp": None,
        "max_timestamp": None,
        "targets": set(),
        "originators": set(),
        "locations": set(),
        "call_types": {},
    }


def emit_progress(
    progress_cb: Callable[[dict[str, Any]], None] | None,
    job: IngestJob,
    stage: str,
    stats: dict[str, Any],
    profile: dict[str, Any],
    started_at: datetime | None,
) -> None:
    if progress_cb is None:
        return
    started = started_at or datetime.now(timezone.utc)
    elapsed_ms = int((datetime.now(timezone.utc) - started).total_seconds() * 1000)
    try:
        progress_cb(
            {
                "request_id": job.request_id,
                "evidence_id": job.evidence_id,
                "job_id": job.job_id,
                "tenant_id": job.tenant_id,
                "collection_id": job.collection_id,
                "file_id": job.file_id,
                "source_file": job.source_file,
                "record_type": job.record_type,
                "stage": stage,
                "rows_processed": int(stats.get("total_rows") or 0),
                "rows_failed": int(stats.get("rejected_rows") or 0),
                "detected_schema": profile.get("classification") or {},
                "elapsed_ms": elapsed_ms,
                "emitted_at": datetime.now(timezone.utc).isoformat(),
            }
        )
    except Exception:  # noqa: BLE001 - telemetry should never fail ingest
        LOG.debug("failed to emit ingest progress", exc_info=True, extra={"job_id": job.job_id})


def index_cdr_entities(cur: psycopg.Cursor[Any]) -> int:
    cur.execute(
        """
        WITH entities AS (
          SELECT tenant_id, collection_id, file_id, batch_id, 'cdr'::forensic_record_type AS record_type,
                 row_number, row_hash, call_start_ts AS observed_at, 'phone' AS entity_type,
                 msisdn AS entity_value, 'MSISDN' AS source_field, source_file
          FROM tmp_cdr_records WHERE msisdn IS NOT NULL AND msisdn <> ''
          UNION ALL
          SELECT tenant_id, collection_id, file_id, batch_id, 'cdr'::forensic_record_type,
                 row_number, row_hash, call_start_ts, 'phone', call_org_num, 'call_org_num', source_file
          FROM tmp_cdr_records WHERE call_org_num IS NOT NULL AND call_org_num <> ''
          UNION ALL
          SELECT tenant_id, collection_id, file_id, batch_id, 'cdr'::forensic_record_type,
                 row_number, row_hash, call_start_ts, 'phone', call_dialed_num, 'CALL_DIALED_NUM', source_file
          FROM tmp_cdr_records WHERE call_dialed_num IS NOT NULL AND call_dialed_num <> ''
          UNION ALL
          SELECT tenant_id, collection_id, file_id, batch_id, 'cdr'::forensic_record_type,
                 row_number, row_hash, call_start_ts, 'imsi', imsi, 'IMSI', source_file
          FROM tmp_cdr_records WHERE imsi IS NOT NULL AND imsi <> ''
          UNION ALL
          SELECT tenant_id, collection_id, file_id, batch_id, 'cdr'::forensic_record_type,
                 row_number, row_hash, call_start_ts, 'imei', imei, 'IMEI', source_file
          FROM tmp_cdr_records WHERE imei IS NOT NULL AND imei <> ''
          UNION ALL
          SELECT tenant_id, collection_id, file_id, batch_id, 'cdr'::forensic_record_type,
                 row_number, row_hash, call_start_ts, 'cell', cell_site_id, 'Cell_SITE_ID', source_file
          FROM tmp_cdr_records WHERE cell_site_id IS NOT NULL AND cell_site_id <> ''
          UNION ALL
          SELECT tenant_id, collection_id, file_id, batch_id, 'cdr'::forensic_record_type,
                 row_number, row_hash, call_start_ts, 'location', location, 'location', source_file
          FROM tmp_cdr_records WHERE location IS NOT NULL AND location <> ''
        )
        INSERT INTO forensic.record_entities (
          tenant_id, collection_id, file_id, batch_id, record_type, row_number,
          row_hash, observed_at, entity_type, entity_value, source_field, source_file
        )
        SELECT tenant_id, collection_id, file_id, batch_id, record_type, row_number,
               row_hash, observed_at, entity_type, entity_value, source_field, source_file
        FROM entities
        ON CONFLICT DO NOTHING
        """
    )
    return cur.rowcount


def index_generic_entities(cur: psycopg.Cursor[Any]) -> int:
    cur.execute(
        """
        WITH entities AS (
          SELECT tenant_id, collection_id, file_id, batch_id, record_type::forensic_record_type AS record_type,
                 row_number, row_hash, observed_at, 'primary' AS entity_type,
                 primary_entity AS entity_value, 'primary_entity' AS source_field, source_file
          FROM tmp_generic_records WHERE primary_entity IS NOT NULL AND primary_entity <> ''
          UNION ALL
          SELECT tenant_id, collection_id, file_id, batch_id, record_type::forensic_record_type,
                 row_number, row_hash, observed_at, 'secondary',
                 secondary_entity, 'secondary_entity', source_file
          FROM tmp_generic_records WHERE secondary_entity IS NOT NULL AND secondary_entity <> ''
          UNION ALL
          SELECT tenant_id, collection_id, file_id, batch_id, record_type::forensic_record_type,
                 row_number, row_hash, observed_at, 'location',
                 location, 'location', source_file
          FROM tmp_generic_records WHERE location IS NOT NULL AND location <> ''
        )
        INSERT INTO forensic.record_entities (
          tenant_id, collection_id, file_id, batch_id, record_type, row_number,
          row_hash, observed_at, entity_type, entity_value, source_field, source_file
        )
        SELECT tenant_id, collection_id, file_id, batch_id, record_type, row_number,
               row_hash, observed_at, entity_type, entity_value, source_field, source_file
        FROM entities
        ON CONFLICT DO NOTHING
        """
    )
    return cur.rowcount


def materialize_metadata(
    cur: psycopg.Cursor[Any],
    job: IngestJob,
    batch_id: str,
    profile: dict[str, Any],
    stats: dict[str, Any],
    inserted: int,
    record_type: str,
) -> None:
    total = int(stats["total_rows"])
    duplicate_rows = max(0, total - int(stats["rejected_rows"]) - inserted)
    quality = {
        "call_types": stats["call_types"],
        "sample_rows_profiled": profile["sample_rows"],
        "detected_schema": profile.get("classification", {}),
    }
    cur.execute(
        """
        INSERT INTO forensic.kb_active_metadata (
          tenant_id, collection_id, file_id, batch_id, record_type, source_file, sha256,
          min_timestamp, max_timestamp, total_rows, inserted_rows, duplicate_rows,
          rejected_rows, unique_targets_count, unique_originators_count,
          unique_locations_count, normalized_schema, quality_report
        )
        VALUES (
          %s, %s, %s, %s::uuid, %s::forensic_record_type, %s, %s, %s, %s, %s, %s, %s, %s,
          %s, %s, %s, %s::jsonb, %s::jsonb
        )
        ON CONFLICT (tenant_id, collection_id, file_id, batch_id)
        DO UPDATE SET
          min_timestamp=EXCLUDED.min_timestamp,
          max_timestamp=EXCLUDED.max_timestamp,
          total_rows=EXCLUDED.total_rows,
          inserted_rows=EXCLUDED.inserted_rows,
          duplicate_rows=EXCLUDED.duplicate_rows,
          rejected_rows=EXCLUDED.rejected_rows,
          unique_targets_count=EXCLUDED.unique_targets_count,
          unique_originators_count=EXCLUDED.unique_originators_count,
          unique_locations_count=EXCLUDED.unique_locations_count,
          normalized_schema=EXCLUDED.normalized_schema,
          quality_report=EXCLUDED.quality_report,
          updated_at=now()
        """,
        (
            job.tenant_id,
            job.collection_id,
            job.file_id,
            batch_id,
            record_type,
            job.source_file,
            job.sha256,
            stats["min_timestamp"],
            stats["max_timestamp"],
            total,
            inserted,
            duplicate_rows,
            int(stats["rejected_rows"]),
            len(stats["targets"]),
            len(stats["originators"]),
            len(stats["locations"]),
            json.dumps(
                {
                    "columns": profile["schema"],
                    "classification": profile.get("classification", {}),
                }
            ),
            json.dumps(quality),
        ),
    )


def materialize_kb_asset(
    cur: psycopg.Cursor[Any],
    job: IngestJob,
    batch_id: str,
    profile: dict[str, Any],
    adapter: RecordAdapter,
    structured_status: str,
    rag_status: str,
    quality: dict[str, Any],
) -> None:
    routing = {
        "storage_mode": "hybrid",
        "requested_record_type": job.record_type,
        "detected_record_type": adapter.name,
        "structured_store": "cdr_records" if adapter.name == "cdr" else "generic_records",
        "canonical_store": "records",
        "kb_collection": job.collection_id,
        "routing_reason": "typed_adapter" if adapter.name != "generic" else "generic_csv_adapter",
        "schema_classification": profile.get("classification", {}),
    }
    cur.execute(
        """
        INSERT INTO forensic.kb_collection_assets (
          tenant_id, user_id, collection_id, file_id, batch_id, source_file,
          source_entry, sha256, detected_record_type, requested_record_type, storage_mode,
          rag_status, structured_status, content_type, size_bytes, evidence_id, headers,
          routing_decision, quality_report
        )
        VALUES (
          %s, %s, %s, %s, %s::uuid, %s,
          %s, %s, %s::forensic_record_type, %s::forensic_record_type, 'hybrid',
          %s, %s, %s, %s, NULLIF(%s, '')::uuid, %s::jsonb,
          %s::jsonb, %s::jsonb
        )
        ON CONFLICT (tenant_id, collection_id, file_id, sha256)
        DO UPDATE SET
          batch_id=EXCLUDED.batch_id,
          source_entry=EXCLUDED.source_entry,
          detected_record_type=EXCLUDED.detected_record_type,
          requested_record_type=EXCLUDED.requested_record_type,
          rag_status=EXCLUDED.rag_status,
          structured_status=EXCLUDED.structured_status,
          evidence_id=coalesce(forensic.kb_collection_assets.evidence_id, EXCLUDED.evidence_id),
          headers=EXCLUDED.headers,
          routing_decision=EXCLUDED.routing_decision,
          quality_report=forensic.kb_collection_assets.quality_report || EXCLUDED.quality_report,
          updated_at=now()
        """,
        (
            job.tenant_id,
            job.user_id,
            job.collection_id,
            job.file_id,
            batch_id,
            job.source_file,
            job.metadata.get("kb_source_entry"),
            job.sha256,
            adapter.name,
            normalize_record_type(job.record_type),
            rag_status,
            structured_status,
            job.metadata.get("content_type"),
            parse_int_optional(job.metadata.get("size_bytes")),
            job.evidence_id,
            json.dumps(profile["headers"]),
            json.dumps(routing),
            json.dumps(quality),
        ),
    )


def mark_evidence_completed(
    cur: psycopg.Cursor[Any],
    job: IngestJob,
    batch_id: str,
    adapter: RecordAdapter,
    profile: dict[str, Any],
    stats: dict[str, Any],
    inserted: int,
    inserted_dynamic: int,
    indexed_entities: int,
) -> None:
    if not job.evidence_id:
        return
    total = int(stats["total_rows"])
    rejected = int(stats["rejected_rows"])
    duplicate_rows = max(0, total - rejected - inserted)
    metadata = {
        "completed_job_id": job.job_id,
        "request_id": job.request_id,
        "structured_summary": {
            "adapter": adapter.name,
            "total_rows": total,
            "inserted_rows": inserted,
            "canonical_rows": inserted_dynamic,
            "duplicate_rows": duplicate_rows,
            "rejected_rows": rejected,
            "indexed_entities": indexed_entities,
        },
        "schema_classification": profile.get("classification", {}),
    }
    cur.execute(
        """
        UPDATE forensic.evidence_items
        SET processing_status='completed',
            records_batch_id=%s::uuid,
            modality='structured_records',
            detected_type=%s,
            processing_route='forensic_records_worker',
            kb_entry_ref=coalesce(kb_entry_ref, NULLIF(%s, '')),
            entities=jsonb_build_object('indexed_count', %s),
            metadata = metadata || %s::jsonb
        WHERE evidence_id=%s::uuid
        """,
        (
            batch_id,
            adapter.name,
            job.metadata.get("kb_source_entry") or "",
            indexed_entities,
            json.dumps(metadata),
            job.evidence_id,
        ),
    )


def mark_job_completed(cur: psycopg.Cursor[Any], job: IngestJob, batch_id: str, stats: dict[str, Any], inserted: int) -> None:
    total = int(stats["total_rows"])
    rejected = int(stats["rejected_rows"])
    duplicate_rows = max(0, total - rejected - inserted)
    cur.execute(
        """
        UPDATE forensic.records_ingest_jobs
        SET status='completed',
            completed_at=now(),
            total_rows=%s,
            accepted_rows=%s,
            duplicate_rows=%s,
            rejected_rows=%s,
            metadata = metadata || %s::jsonb
        WHERE id=%s::uuid
        """,
        (
            total,
            inserted,
            duplicate_rows,
            rejected,
            json.dumps(
                {
                    "batch_id": batch_id,
                    "total_rows": total,
                    "inserted_rows": inserted,
                    "duplicate_rows": duplicate_rows,
                    "rejected_rows": rejected,
                    "kb_collection_id": job.collection_id,
                }
            ),
            job.job_id,
        ),
    )


def audit(cur: psycopg.Cursor[Any], job: IngestJob, action: str, details: dict[str, Any]) -> None:
    cur.execute(
        """
        INSERT INTO forensic.records_audit_log (tenant_id, user_id, action, collection_id, file_id, batch_id, details)
        VALUES (%s, %s, %s, %s, %s, %s::uuid, %s::jsonb)
        """,
        (job.tenant_id, job.user_id, action, job.collection_id, job.file_id, details.get("batch_id"), json.dumps(details)),
    )


def validate_sample_profile(path: Path) -> dict[str, Any]:
    """Small deterministic validator for the attached 923461678183.csv sample."""
    with path.open(newline="", encoding="utf-8-sig") as fh:
        reader = csv.DictReader(fh)
        total = 0
        targets: set[str] = set()
        locations: set[str] = set()
        call_types: dict[str, int] = {}
        min_ts: datetime | None = None
        max_ts: datetime | None = None
        for row in reader:
            total += 1
            targets.add(clean_text(row.get("CALL_DIALED_NUM")))
            locations.add(clean_text(row.get("location")))
            call_type = clean_text(row.get("CALL_TYPE")).upper()
            call_types[call_type] = call_types.get(call_type, 0) + 1
            ts = parse_ts(row.get("CALL_START_DT_TM"))
            min_ts = ts if min_ts is None else min(min_ts, ts)
            max_ts = ts if max_ts is None else max(max_ts, ts)
    return {
        "total_rows": total,
        "unique_targets_count": len(targets),
        "unique_locations_count": len(locations),
        "min_timestamp": min_ts.isoformat() if min_ts else None,
        "max_timestamp": max_ts.isoformat() if max_ts else None,
        "call_types": call_types,
    }


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "validate-sample":
        print(json.dumps(validate_sample_profile(Path(sys.argv[2])), indent=2, sort_keys=True))
    else:
        main()
