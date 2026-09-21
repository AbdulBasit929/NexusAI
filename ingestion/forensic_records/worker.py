#!/usr/bin/env python3
"""Forensic records ingestion worker.

Subscribes to NATS jobs emitted by api/forensic_records, routes files through
typed adapters, and materializes Knowledge Base metadata plus deterministic
record stores for exact analytics.
"""

from __future__ import annotations

import argparse
import asyncio
import contextlib
import csv
import hashlib
import ipaddress
import json
import logging
import math
import os
import re
import signal
import socket
import stat
import sys
import tempfile
import unicodedata
import uuid
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone, tzinfo
from decimal import Decimal, InvalidOperation
from functools import lru_cache
from pathlib import Path
from typing import Any, Callable, Iterable
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError

MAX_STRUCTURED_COLUMNS = 512

try:
    import polars as pl
except ModuleNotFoundError:  # pragma: no cover - allows pure adapter tests without worker deps
    pl = None  # type: ignore[assignment]

try:
    from ingestion.forensic_records.xlsx_reader import iter_xlsx_records, profile_xlsx
except ModuleNotFoundError:  # pragma: no cover - supports direct worker.py execution
    from xlsx_reader import iter_xlsx_records, profile_xlsx

try:
    from ingestion.forensic_records.forensic_contracts import (
        CompatibilityAdapterRegistry,
        load_platform_catalog,
    )
except ModuleNotFoundError:  # pragma: no cover - supports direct worker.py execution
    from forensic_contracts import CompatibilityAdapterRegistry, load_platform_catalog

try:
    from ingestion.forensic_records.media_pipeline import (
        ANPR_OBSERVATION_CONTRACT,
        AUDIO_TIMESTAMP_SEGMENT_CONTRACT,
        FACE_OBSERVATION_CONTRACT,
        IMAGE_EMBEDDING_CONTRACT,
        IMAGE_OCR_OBSERVATION_CONTRACT,
        ROMAN_URDU_SEGMENT_CONTRACT,
        MediaProcessResult,
        configured_media_processor,
    )
except ModuleNotFoundError:  # pragma: no cover - supports direct worker.py execution
    from media_pipeline import (
        ANPR_OBSERVATION_CONTRACT,
        AUDIO_TIMESTAMP_SEGMENT_CONTRACT,
        FACE_OBSERVATION_CONTRACT,
        IMAGE_EMBEDDING_CONTRACT,
        IMAGE_OCR_OBSERVATION_CONTRACT,
        ROMAN_URDU_SEGMENT_CONTRACT,
        MediaProcessResult,
        configured_media_processor,
    )

try:
    from ingestion.forensic_records.document_pipeline import (
        DOCUMENT_PASSAGE_CONTRACT,
        DocumentProcessResult,
        extract_document,
    )
except ModuleNotFoundError:  # pragma: no cover - supports direct worker.py execution
    from document_pipeline import DOCUMENT_PASSAGE_CONTRACT, DocumentProcessResult, extract_document

try:
    from ingestion.forensic_records.structured_maturity import (
        TIME_POLICY_VERSION,
        dynamic_attributes as build_dynamic_attributes,
        quality_summary as build_quality_summary,
        registry_snapshot as structured_registry_snapshot,
        schema_profile as build_schema_profile,
    )
except ModuleNotFoundError:  # pragma: no cover - supports direct worker.py execution
    from structured_maturity import (
        TIME_POLICY_VERSION,
        dynamic_attributes as build_dynamic_attributes,
        quality_summary as build_quality_summary,
        registry_snapshot as structured_registry_snapshot,
        schema_profile as build_schema_profile,
    )

try:
    from ingestion.forensic_records.adapters.cdr import (
        CDRAdapterRuntime,
        CDR_REQUIRED_COLUMN_GROUPS,
        normalize_record as normalize_cdr_record,
        schema_profile as profile_cdr_schema,
        validate_headers as validate_cdr_adapter_headers,
    )
except ModuleNotFoundError:  # pragma: no cover - supports direct worker.py execution
    from adapters.cdr import (
        CDRAdapterRuntime,
        CDR_REQUIRED_COLUMN_GROUPS,
        normalize_record as normalize_cdr_record,
        schema_profile as profile_cdr_schema,
        validate_headers as validate_cdr_adapter_headers,
    )

try:
    from ingestion.forensic_records.adapters.ipdr import (
        IPDRAdapterRuntime,
        IPDR_REQUIRED_COLUMN_GROUPS,
        normalize_record as normalize_ipdr_record,
    )
except ModuleNotFoundError:  # pragma: no cover - supports direct worker.py execution
    from adapters.ipdr import (
        IPDRAdapterRuntime,
        IPDR_REQUIRED_COLUMN_GROUPS,
        normalize_record as normalize_ipdr_record,
    )

try:
    from ingestion.forensic_records.adapters.anpr import (
        ANPRAdapterRuntime,
        ANPR_REQUIRED_COLUMN_GROUPS,
        normalize_record as normalize_anpr_record,
    )
except ModuleNotFoundError:  # pragma: no cover - supports direct worker.py execution
    from adapters.anpr import (
        ANPRAdapterRuntime,
        ANPR_REQUIRED_COLUMN_GROUPS,
        normalize_record as normalize_anpr_record,
    )

try:
    from ingestion.forensic_records.adapters.subscriber import (
        SubscriberAdapterRuntime,
        normalize_record as normalize_subscriber_record,
    )
except ModuleNotFoundError:  # pragma: no cover - supports direct worker.py execution
    from adapters.subscriber import (
        SubscriberAdapterRuntime,
        normalize_record as normalize_subscriber_record,
    )

try:
    from ingestion.forensic_records.adapters.tower_location import (
        TowerAdapterRuntime,
        normalize_record as normalize_tower_record,
    )
except ModuleNotFoundError:  # pragma: no cover - supports direct worker.py execution
    from adapters.tower_location import (
        TowerAdapterRuntime,
        normalize_record as normalize_tower_record,
    )

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
DEAD_LETTER_SUBJECT = os.getenv("NATS_RECORDS_DLQ_SUBJECT", "forensic.records.ingest.dead_letter")
INGEST_STREAM = os.getenv("NATS_RECORDS_STREAM", "FORENSIC_RECORDS_INGEST")
DEAD_LETTER_STREAM = os.getenv("NATS_RECORDS_DLQ_STREAM", "FORENSIC_RECORDS_DLQ")
INGEST_CONSUMER = os.getenv("NATS_RECORDS_CONSUMER", "forensic-records-worker-v1")
# nats-py uses the queue name as the durable consumer identity. Keeping these
# identities equal makes restarts bind the same durable consumer instead of
# failing before the worker can receive retained messages.
INGEST_QUEUE_GROUP = os.getenv("NATS_RECORDS_QUEUE_GROUP", INGEST_CONSUMER)
NATS_URL = os.getenv("NATS_URL", "nats://nats:4222")
DATABASE_URL = os.getenv("FORENSIC_DATABASE_URL", "postgresql://localrecall:localrecall@postgres:5432/localrecall")
PROMETHEUS_ADDR = int(os.getenv("FORENSIC_WORKER_METRICS_PORT", "9109"))
TENANT_SETTING = "app.tenant_id"
SPOOL_ROOT = Path(os.getenv("FORENSIC_SPOOL_DIR", "/data/forensic/spool"))
WORKER_ID = os.getenv("FORENSIC_WORKER_ID", f"{socket.gethostname()}:{os.getpid()}")
WORKER_LEASE_SECONDS = max(30, int(os.getenv("FORENSIC_WORKER_LEASE_SECONDS", "600")))
ACK_HEARTBEAT_SECONDS = max(5, min(WORKER_LEASE_SECONDS // 3, 30))
MAX_ACK_PENDING = max(1, int(os.getenv("FORENSIC_WORKER_MAX_INFLIGHT", "1")))
CONTENT_ADDRESSED_LAYOUT_VERSION = "sha256-scope-v1"
CONTENT_ADDRESSED_URI_RE = re.compile(
    rf"^forensic-spool://{CONTENT_ADDRESSED_LAYOUT_VERSION}/([0-9a-f]{{64}})/([0-9a-f]{{64}})$"
)

JOBS_TOTAL = Counter("forensic_records_jobs_total", "Ingest jobs processed", ["record_type", "status"])
ROWS_TOTAL = Counter("forensic_records_rows_total", "Rows observed by worker", ["record_type", "status"])
INGEST_JOBS_COMPLETED_TOTAL = Counter("forensic_ingest_jobs_completed_total", "Completed ingest jobs", ["record_type"])
INGEST_ROWS_TOTAL = Counter("forensic_ingest_rows_total", "Rows processed by ingest worker", ["record_type", "status"])
INGEST_SECONDS = Histogram("forensic_records_ingest_seconds", "Wall time per ingest job", ["record_type"])
QUEUE_DELIVERIES_TOTAL = Counter(
    "forensic_records_queue_deliveries_total", "JetStream ingest deliveries", ["disposition"]
)
QUEUE_RETRIES_TOTAL = Counter(
    "forensic_records_queue_retries_total", "Scheduled ingest retries", ["error_class"]
)
QUEUE_DLQ_TOTAL = Counter(
    "forensic_records_queue_dead_letter_total", "Terminal ingest messages", ["reason"]
)

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
    "normalized_record",
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
    "normalized_record",
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
    "call_time",
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
        required_any=CDR_REQUIRED_COLUMN_GROUPS,
        timestamp_fields=("CALL_START_DT_TM", "call_start_time", "call_time", "timestamp", "start_time"),
        primary_fields=("MSISDN", "call_org_num", "source_number", "a_number"),
        secondary_fields=("CALL_DIALED_NUM", "dialed_number", "target_number", "b_number"),
        location_fields=("location", "Cell_SITE_ID", "cell_id", "site_id"),
    ),
    "ipdr": RecordAdapter(
        name="ipdr",
        required_any=IPDR_REQUIRED_COLUMN_GROUPS,
        timestamp_fields=("timestamp", "session_start", "start_time", "event_time", "time"),
        primary_fields=("source_ip", "src_ip", "ip_address", "msisdn"),
        secondary_fields=("destination_ip", "dst_ip", "dest_ip", "domain", "url"),
        location_fields=("location", "cell_id", "site_id"),
    ),
    "anpr": RecordAdapter(
        name="anpr",
        required_any=ANPR_REQUIRED_COLUMN_GROUPS,
        timestamp_fields=("timestamp", "event_time", "capture_time", "captured_at", "capture_datetime", "observed_at", "camera_time", "reading_time", "date_time", "time"),
        primary_fields=("plate", "plate_number", "license_plate", "registration_number", "registration_no", "reg_no", "vehicle_no", "number_plate", "plate_no", "vehicle_registration_no", "registration_mark", "vrn"),
        secondary_fields=("camera_id", "camera", "camera_code", "camera_name", "device_id", "checkpoint", "checkpoint_id", "gantry_id", "lane", "lane_id"),
        location_fields=("location", "camera_location", "checkpoint", "site", "camera_site", "toll_plaza", "district", "city"),
    ),
    "subscriber": RecordAdapter(
        name="subscriber",
        required_any=(("msisdn", "phone_number", "subscriber_number", "mobile_no", "cellular_number", "mdn", "account_msisdn"), ("cnic", "cnic_no", "national_id", "nic", "cnic_last4", "customer_id", "customer_no", "subscriber_id", "name", "full_name", "subscriber_name", "customer_name")),
        timestamp_fields=("activation_date", "created_at", "updated_at", "timestamp"),
        primary_fields=("msisdn", "phone_number", "subscriber_number", "mobile_no", "cellular_number", "mdn", "account_msisdn"),
        secondary_fields=("cnic", "cnic_no", "national_id", "nic", "cnic_last4", "customer_id", "customer_no", "subscriber_id", "name", "full_name", "subscriber_name", "customer_name"),
        location_fields=("address", "city", "home_city", "billing_region", "location"),
    ),
    "tower_location": RecordAdapter(
        name="tower_location",
        required_any=(("cell_id", "cell_site_id", "site_id", "site_code", "cgi", "ecgi", "enodeb_id", "gnodeb_id"), ("lat", "latitude", "latitude_wgs84"), ("longitude", "lon", "lng", "longitude_wgs84")),
        timestamp_fields=("timestamp", "updated_at", "created_at"),
        primary_fields=("cell_id", "cell_site_id", "site_id", "site_code", "cgi", "ecgi", "enodeb_id", "gnodeb_id"),
        secondary_fields=("sector_id", "sector", "lac", "lac_id", "tac", "tower_id"),
        location_fields=("location", "address", "site_name", "site_location", "area", "district", "city"),
        latitude_fields=("lat", "latitude", "latitude_wgs84", "y"),
        longitude_fields=("longitude", "lon", "lng", "longitude_wgs84", "x"),
    ),
    "transaction": RecordAdapter(
        name="transaction",
        required_any=(("amount", "transaction_amount", "txn_amount", "amount_pkr", "debit_amount", "credit_amount", "transaction_value", "local_amount"), ("account", "account_number", "iban", "from_account", "from_iban", "source_account", "debit_account", "payer_account", "remitter_account", "sender", "sender_iban")),
        timestamp_fields=("timestamp", "transaction_time", "transaction_timestamp", "txn_time", "transaction_date", "booking_date", "value_date", "created_at"),
        primary_fields=("account", "account_number", "iban", "from_account", "from_iban", "source_account", "debit_account", "payer_account", "remitter_account", "sender", "sender_iban"),
        secondary_fields=("counterparty", "to_account", "to_iban", "beneficiary_account", "beneficiary_iban", "destination_account", "credit_account", "payee_account", "receiver_account", "receiver", "beneficiary", "merchant", "merchant_name"),
        location_fields=("location", "merchant_location", "city"),
    ),
    "access_log": RecordAdapter(
        name="access_log",
        required_any=(("ip", "source_ip", "src_ip", "client_ip", "remote_addr", "remote_ip", "source_address"), ("user", "username", "user_id", "principal", "actor", "path", "url", "uri", "event_action")),
        timestamp_fields=("timestamp", "time", "event_time", "event_timestamp", "created_at", "@timestamp"),
        primary_fields=("user", "username", "user_id", "principal", "actor", "source_ip", "src_ip", "client_ip", "remote_addr", "remote_ip", "source_address", "ip"),
        secondary_fields=("path", "url", "uri", "endpoint", "event_action", "action", "status", "http_status"),
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


@dataclass(frozen=True)
class JobClaim:
    disposition: str
    lease_token: str = ""
    attempt_count: int = 0
    max_attempts: int = 0
    reprocess_generation: int = 0
    delay_seconds: float = 0.0


@dataclass(frozen=True)
class FailureDisposition:
    disposition: str
    attempt_count: int
    max_attempts: int
    error_class: str
    delay_seconds: float = 0.0


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
    from nats.js.api import AckPolicy, ConsumerConfig
    from nats.js.errors import NotFoundError

    stop = asyncio.Event()
    loop = asyncio.get_running_loop()

    def _stop(*_: object) -> None:
        stop.set()

    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            asyncio.get_running_loop().add_signal_handler(sig, _stop)
        except NotImplementedError:
            pass

    await asyncio.to_thread(validate_queue_schema)
    nc = await nats.connect(NATS_URL, name="forensic-records-ingestion-worker")
    js = nc.jetstream()
    try:
        await js.stream_info(INGEST_STREAM)
    except NotFoundError:
        await js.add_stream(
            name=INGEST_STREAM,
            subjects=[INGEST_SUBJECT],
            retention="workqueue",
            storage="file",
            max_age=30 * 24 * 60 * 60,
            duplicate_window=10 * 60,
        )
    try:
        await js.stream_info(DEAD_LETTER_STREAM)
    except NotFoundError:
        await js.add_stream(
            name=DEAD_LETTER_STREAM,
            subjects=[DEAD_LETTER_SUBJECT],
            retention="limits",
            storage="file",
            max_age=90 * 24 * 60 * 60,
            max_msgs=100_000,
            max_bytes=1 << 30,
            duplicate_window=10 * 60,
        )

    async def handle_message(msg: Any) -> None:
        await handle_ingest_message(msg, js, nc, loop)

    if INGEST_QUEUE_GROUP != INGEST_CONSUMER:
        raise RuntimeError(
            "NATS_RECORDS_QUEUE_GROUP must equal NATS_RECORDS_CONSUMER for a durable JetStream queue subscription"
        )
    await js.subscribe(
        INGEST_SUBJECT,
        durable=INGEST_CONSUMER,
        queue=INGEST_QUEUE_GROUP,
        cb=handle_message,
        manual_ack=True,
        config=ConsumerConfig(
            ack_policy=AckPolicy.EXPLICIT,
            ack_wait=float(WORKER_LEASE_SECONDS),
            max_deliver=-1,
            max_ack_pending=MAX_ACK_PENDING,
        ),
    )
    LOG.info(
        "worker subscribed to durable JetStream consumer",
        extra={
            "subject": INGEST_SUBJECT,
            "stream": INGEST_STREAM,
            "consumer": INGEST_CONSUMER,
            "queue": INGEST_QUEUE_GROUP,
            "nats_url": NATS_URL,
        },
    )
    await stop.wait()
    await nc.drain()


def validate_queue_schema() -> None:
    required_columns = (
        "queue_message_id", "queue_published_at", "queue_publish_attempts",
        "queue_publish_next_at", "queue_publish_lease_token", "worker_lease_token",
        "worker_lease_expires_at", "next_attempt_at", "dead_lettered_at",
        "acknowledged_at", "reprocess_of_job_id", "reprocess_generation",
        "reprocess_request_key",
    )
    with psycopg.connect(DATABASE_URL, autocommit=True) as conn:
        row = conn.execute(
            """
            SELECT to_regclass('forensic.records_ingest_jobs') IS NOT NULL,
                   to_regprocedure('forensic.protect_ingest_job_history()') IS NOT NULL,
                   ARRAY(
                     SELECT column_name::text
                     FROM information_schema.columns
                     WHERE table_schema='forensic'
                       AND table_name='records_ingest_jobs'
                   ),
                   to_regclass('forensic.records') IS NOT NULL,
                   ARRAY(
                     SELECT column_name::text
                     FROM information_schema.columns
                     WHERE table_schema='forensic'
                       AND table_name='records'
                   ),
                   EXISTS (
                     SELECT 1
                     FROM pg_policies
                     WHERE schemaname='forensic'
                       AND tablename='records'
                       AND policyname='tenant_isolation_records'
                   ),
                   3 = (
                     SELECT count(*)
                     FROM pg_policies
                     WHERE schemaname='forensic'
                       AND (tablename, policyname) IN (
                         ('records_ingest_jobs', 'tenant_isolation_records_ingest_jobs'),
                         ('records_ingest_errors', 'tenant_isolation_records_ingest_errors'),
                         ('records_audit_log', 'tenant_isolation_records_audit_log')
                       )
                   )
            """
        ).fetchone()
    present = set(row[2] or ()) if row else set()
    missing = sorted(set(required_columns) - present)
    records_required_columns = {
        "record_id", "tenant_id", "collection_id", "file_id", "batch_id",
        "record_type", "timestamp", "source_file", "row_number", "row_hash",
        "raw_payload", "metadata", "evidence_id",
    }
    records_present = set(row[4] or ()) if row else set()
    records_missing = sorted(records_required_columns - records_present)
    if not row or not row[0] or not row[1] or missing:
        detail = f"; missing columns: {', '.join(missing)}" if missing else ""
        raise RuntimeError(
            "Phase 3 queue schema is not ready; apply and verify migration 009 "
            f"before starting the worker{detail}"
        )
    if not row[3] or not row[5] or records_missing:
        detail = f"; missing columns: {', '.join(records_missing)}" if records_missing else ""
        raise RuntimeError(
            "Canonical records schema is not ready; apply migrations 004 and 007 "
            f"before starting the worker{detail}"
        )
    if not row[6]:
        raise RuntimeError(
            "Phase 3 runtime tenant policies are not ready; apply and verify "
            "migration 010 before starting the worker"
        )


async def handle_ingest_message(msg: Any, js: Any, nc: Any, loop: asyncio.AbstractEventLoop) -> None:
    try:
        job = IngestJob.from_message(msg.data)
    except Exception as exc:  # noqa: BLE001 - poison messages must be terminal
        LOG.error("invalid ingest message", exc_info=True)
        await publish_dead_letter(js, None, msg.data, "invalid_message", str(exc), 0, 0)
        QUEUE_DLQ_TOTAL.labels("invalid_message").inc()
        QUEUE_DELIVERIES_TOTAL.labels("terminal_invalid").inc()
        await msg.term()
        return

    if not job.request_id:
        headers = getattr(msg, "headers", None) or {}
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
            request_id=headers.get("X-Request-ID", ""),
        )

    try:
        claim = await asyncio.to_thread(claim_job, job)
    except Exception:  # noqa: BLE001 - database unavailability is retryable
        LOG.exception("could not claim ingest job", extra={"job_id": job.job_id})
        QUEUE_DELIVERIES_TOTAL.labels("claim_error").inc()
        await msg.nak(delay=2.0)
        return

    if claim.disposition == "completed":
        await acknowledge_completed_message(msg, job)
        QUEUE_DELIVERIES_TOTAL.labels("completed_redelivery").inc()
        return
    if claim.disposition == "dead_letter":
        await publish_dead_letter(
            js, job, msg.data, "job_already_dead_lettered", "terminal job redelivery",
            claim.attempt_count, claim.max_attempts,
        )
        await msg.term()
        QUEUE_DELIVERIES_TOTAL.labels("dead_letter_redelivery").inc()
        return
    if claim.disposition in {"busy", "deferred"}:
        QUEUE_DELIVERIES_TOTAL.labels(claim.disposition).inc()
        await msg.nak(delay=max(0.1, claim.delay_seconds))
        return

    def publish_progress(event: dict[str, Any]) -> None:
        payload = json.dumps(event, default=str, sort_keys=True).encode()
        future = asyncio.run_coroutine_threadsafe(nc.publish(PROGRESS_SUBJECT, payload), loop)
        future.add_done_callback(log_progress_publish_error)

    heartbeat = asyncio.create_task(ack_heartbeat(msg))
    try:
        await asyncio.to_thread(process_job, job, publish_progress, claim.lease_token)
    except Exception as exc:  # noqa: BLE001 - classified below
        LOG.exception("ingest job failed", extra={"job_id": job.job_id})
        JOBS_TOTAL.labels(job.record_type, "failed").inc()
        error_class = classify_job_error(exc)
        failure = await asyncio.to_thread(
            mark_job_failed, job, claim.lease_token, str(exc), error_class
        )
        if failure.disposition == "retry":
            QUEUE_RETRIES_TOTAL.labels(error_class).inc()
            QUEUE_DELIVERIES_TOTAL.labels("retry").inc()
            await msg.nak(delay=failure.delay_seconds)
            return
        await publish_dead_letter(
            js, job, msg.data, error_class, str(exc),
            failure.attempt_count, failure.max_attempts,
        )
        QUEUE_DLQ_TOTAL.labels(error_class).inc()
        QUEUE_DELIVERIES_TOTAL.labels("dead_letter").inc()
        await msg.term()
        return
    finally:
        heartbeat.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await heartbeat

    await acknowledge_completed_message(msg, job)
    JOBS_TOTAL.labels(job.record_type, "completed").inc()
    INGEST_JOBS_COMPLETED_TOTAL.labels(job.record_type).inc()
    QUEUE_DELIVERIES_TOTAL.labels("completed").inc()


async def ack_heartbeat(msg: Any) -> None:
    try:
        while True:
            await asyncio.sleep(ACK_HEARTBEAT_SECONDS)
            await msg.in_progress()
    except asyncio.CancelledError:
        raise
    except Exception:  # noqa: BLE001 - the database lease remains authoritative
        LOG.warning("JetStream acknowledgement heartbeat failed", exc_info=True)


async def acknowledge_completed_message(msg: Any, job: IngestJob) -> None:
    await msg.ack_sync(timeout=5.0)
    try:
        await asyncio.to_thread(mark_job_acknowledged, job)
    except Exception:  # noqa: BLE001 - acknowledgement already succeeded
        LOG.exception("could not record acknowledged_at", extra={"job_id": job.job_id})


async def publish_dead_letter(
    js: Any,
    job: IngestJob | None,
    original_payload: bytes,
    reason: str,
    message: str,
    attempt_count: int,
    max_attempts: int,
) -> None:
    job_id = job.job_id if job is not None else ""
    digest = hashlib.sha256(original_payload).hexdigest()
    event_id = f"dlq:{job_id or digest}:{attempt_count}:{reason}"
    envelope = {
        "event_id": event_id,
        "job_id": job_id,
        "evidence_id": job.evidence_id if job is not None else "",
        "tenant_id": job.tenant_id if job is not None else "",
        "collection_id": job.collection_id if job is not None else "",
        "reason": reason,
        "message": message[:4000],
        "attempt_count": attempt_count,
        "max_attempts": max_attempts,
        "original_sha256": digest,
        "recorded_at": datetime.now(timezone.utc).isoformat(),
    }
    await js.publish(
        DEAD_LETTER_SUBJECT,
        json.dumps(envelope, sort_keys=True).encode(),
        headers={"Nats-Msg-Id": event_id},
    )


def should_process_native_document(route: str) -> bool:
    """Treat the authenticated processing route as authoritative.

    Older generic-text admissions can carry ``structured_records`` modality
    metadata even though the API selected the bounded native document route.
    Reprocessing must not silently fall back to the generic row adapter.
    """

    return route == "native_document_worker"


def process_job(
    job: IngestJob,
    progress_cb: Callable[[dict[str, Any]], None] | None = None,
    lease_token: str = "",
) -> None:
    path = validate_job_storage(job)
    started_at = datetime.now(timezone.utc)

    with INGEST_SECONDS.labels(job.record_type).time():
        with psycopg.connect(DATABASE_URL, autocommit=False) as conn:
            conn.execute("SELECT set_config(%s, %s, true)", (TENANT_SETTING, job.tenant_id))
            ensure_job_row(conn, job)
            if lease_token:
                verify_job_lease(conn, job, lease_token)
            else:
                mark_job_running(conn, job)
            mark_evidence_processing(conn, job)
            modality = job.metadata.get("evidence_modality", "").strip().lower()
            route = job.metadata.get("evidence_processing_route", "").strip().lower()
            if should_process_native_document(route):
                document_profile, document_stats = process_document_job(conn, job, path)
                conn.commit()
                emit_progress(progress_cb, job, "completed", document_stats, document_profile, started_at)
                LOG.info(
                    "native document processing completed",
                    extra={
                        "job_id": job.job_id,
                        "format": document_profile.get("format"),
                        "passages": document_stats["accepted_rows"],
                    },
                )
                return
            if modality in {"image", "audio", "video"}:
                media_profile, media_stats = process_media_job(conn, job, path, modality)
                conn.commit()
                emit_progress(progress_cb, job, "completed", media_stats, media_profile, started_at)
                LOG.info(
                    "media processing completed",
                    extra={
                        "job_id": job.job_id,
                        "modality": modality,
                        "readiness": media_profile.get("readiness"),
                        "observations": media_stats["accepted_rows"],
                    },
                )
                return
            profile = profile_records(path, job.source_file)
            validate_structured_profile_bounds(profile)
            xlsx_mappings: dict[int, dict[str, Any]] = {}
            if profile.get("format") == "xlsx":
                xlsx_mappings, adapter = prepare_xlsx_sheet_mappings(job, profile)
            else:
                adapter = resolve_adapter(job, profile["headers"])
                attach_family_schema_profile(profile, adapter, job)
            batch_id = job.job_id
            emit_progress(progress_cb, job, "started", empty_stats(), profile, started_at)
            with conn.cursor() as cur:
                materialize_kb_asset(cur, job, batch_id, profile, adapter, "pending", rag_status_from_job(job), {})
                if adapter.name == "cdr" and profile.get("format") != "xlsx":
                    validate_cdr_headers(profile["headers"])
                    create_temp_cdr_stage(cur)
                    stats = copy_cdr_to_stage(cur, job, path, batch_id, profile, started_at, progress_cb)
                    inserted = insert_stage_into_cdr(cur)
                    inserted_dynamic = insert_stage_into_dynamic_from_cdr(cur, profile, job)
                    indexed_entities = index_cdr_entities(cur)
                else:
                    create_temp_generic_stage(cur)
                    stats = copy_generic_to_stage(
                        cur,
                        job,
                        path,
                        batch_id,
                        adapter,
                        profile,
                        started_at,
                        progress_cb,
                        xlsx_mappings,
                    )
                    inserted = insert_stage_into_generic(cur)
                    inserted_dynamic = insert_stage_into_dynamic_from_generic(cur, profile, job)
                    indexed_entities = index_generic_entities(cur)
                profile["structured_quality"] = build_quality_summary(
                    (profile.get("classification") or {}).get("family_mapping") or {}, stats, inserted
                )
                materialize_metadata(cur, job, batch_id, profile, stats, inserted, adapter.name)
                mark_job_completed(cur, job, batch_id, stats, inserted, lease_token)
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
                        "record_types": stats.get("record_types", {}),
                        "structured_quality": profile["structured_quality"],
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
                        "record_types": stats.get("record_types", {}),
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


@lru_cache(maxsize=1)
def media_processor() -> Any:
    # Model construction is expensive; a worker process owns one immutable
    # configured processor and never discovers or downloads assets per job.
    return configured_media_processor()


def process_media_job(
    conn: psycopg.Connection[Any],
    job: IngestJob,
    path: Path,
    modality: str,
) -> tuple[dict[str, Any], dict[str, Any]]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT current_version_id::text, media_metadata
            FROM forensic.evidence_items
            WHERE tenant_id=%s AND collection_id=%s AND evidence_id=%s::uuid
            """,
            (job.tenant_id, job.collection_id, job.evidence_id),
        )
        row = cur.fetchone()
        if row is None or not row[0]:
            raise RuntimeError("media evidence version is unavailable")
        version_id = str(row[0])
        admitted_metadata = admitted_media_metadata(row[1], job.metadata)

    result = media_processor().process(
        path,
        modality=modality,
        evidence_id=job.evidence_id,
        version_id=version_id,
        source_sha256=job.sha256,
        source_file=job.source_file,
        admitted_metadata=admitted_metadata,
    )
    with conn.cursor() as cur:
        persist_media_result(cur, job, version_id, result)
        materialize_media_kb_asset(cur, job, result)
        mark_media_job_completed(cur, job, result)
        audit(
            cur,
            job,
            "media.process.completed",
            {
                "modality": modality,
                "readiness": result.readiness,
                "processor": result.processor_id,
                "processor_revision": result.processor_revision,
                "observation_count": len(result.observations),
                "limitations": list(result.limitations),
            },
        )
    stats = empty_stats()
    stats["total_rows"] = len(result.observations)
    stats["accepted_rows"] = len(result.observations)
    profile = {
        "classification": {
            "modality": modality,
            "processor": result.processor_id,
            "contract_version": result.contract_version,
        },
        "readiness": result.readiness,
    }
    return profile, stats


def admitted_media_metadata(
    media_metadata: dict[str, Any] | None,
    job_metadata: dict[str, str] | None,
) -> dict[str, Any]:
    admitted = dict(media_metadata or {})
    language = str((job_metadata or {}).get("asr_language") or "").strip().lower()
    if language:
        admitted["asr_language"] = language
    route_plan = str((job_metadata or {}).get("evidence_route_plan") or "").strip()
    if route_plan:
        try:
            roles = json.loads(route_plan).get("composed_roles") or []
        except (json.JSONDecodeError, AttributeError):
            roles = []
        admitted["requested_analysis_roles"] = [
            str(role).strip().lower() for role in roles if str(role).strip()
        ]
    return admitted


def process_document_job(
    conn: psycopg.Connection[Any],
    job: IngestJob,
    path: Path,
) -> tuple[dict[str, Any], dict[str, Any]]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT current_version_id::text
            FROM forensic.evidence_items
            WHERE tenant_id=%s AND collection_id=%s AND evidence_id=%s::uuid
            """,
            (job.tenant_id, job.collection_id, job.evidence_id),
        )
        row = cur.fetchone()
        if row is None or not row[0]:
            raise RuntimeError("document evidence version is unavailable")
        version_id = str(row[0])
    result = extract_document(
        path,
        source_file=job.source_file,
        evidence_id=job.evidence_id,
        version_id=version_id,
        source_sha256=job.sha256,
    )
    with conn.cursor() as cur:
        persist_document_result(cur, job, version_id, result)
        materialize_document_kb_asset(cur, job, result)
        mark_document_job_completed(cur, job, result)
        audit(
            cur,
            job,
            "document.native_text.completed",
            {
                "format": result.format,
                "processor": result.processor_id,
                "processor_revision": result.processor_revision,
                "passage_count": len(result.passages),
                "limitations": list(result.limitations),
            },
        )
    stats = empty_stats()
    stats["total_rows"] = len(result.passages)
    stats["accepted_rows"] = len(result.passages)
    profile = {
        "format": result.format,
        "classification": {
            "modality": "document",
            "processor": result.processor_id,
            "contract_version": result.contract_version,
        },
        "readiness": result.readiness,
    }
    return profile, stats


def persist_document_result(
    cur: psycopg.Cursor[Any],
    job: IngestJob,
    version_id: str,
    result: DocumentProcessResult,
) -> None:
    for passage in result.passages:
        metadata = {
            "passage_contract": DOCUMENT_PASSAGE_CONTRACT,
            "text": passage.text,
            "text_sha256": passage.text_sha256,
            "processor": result.processor_id,
            "processor_revision": result.processor_revision,
            "source_truth_state": "derived_native_text",
            "native_text_only": True,
        }
        cur.execute(
            """
            INSERT INTO forensic.derived_artifacts (
              artifact_id, tenant_id, collection_id, evidence_id, version_id, run_id,
              artifact_type, media_type, processing_status, confidence,
              citation_locator, metadata, warnings
            ) VALUES (
              %s::uuid, %s, %s, %s::uuid, %s::uuid, %s::uuid,
              %s, 'text/plain; charset=utf-8', 'completed', 1.0,
              %s::jsonb, %s::jsonb, '[]'::jsonb
            ) ON CONFLICT (artifact_id) DO NOTHING
            """,
            (
                passage.artifact_id,
                job.tenant_id,
                job.collection_id,
                job.evidence_id,
                version_id,
                job.job_id,
                DOCUMENT_PASSAGE_CONTRACT,
                json.dumps(passage.locator),
                json.dumps(metadata),
            ),
        )
    summary = {
        "contract_version": result.contract_version,
        "version_id": version_id,
        "readiness": result.readiness,
        "processor": result.processor_id,
        "processor_revision": result.processor_revision,
        "passage_count": len(result.passages),
        "limitations": list(result.limitations),
        "metadata": result.metadata,
    }
    cur.execute(
        """
        UPDATE forensic.evidence_items
        SET processing_status='completed',
            processing_route='native_document_worker',
            metadata=metadata || jsonb_build_object('document_processing', %s::jsonb),
            warnings=warnings || %s::jsonb
        WHERE tenant_id=%s AND collection_id=%s AND evidence_id=%s::uuid
        """,
        (
            json.dumps(summary),
            json.dumps(list(result.limitations)),
            job.tenant_id,
            job.collection_id,
            job.evidence_id,
        ),
    )


def materialize_document_kb_asset(
    cur: psycopg.Cursor[Any],
    job: IngestJob,
    result: DocumentProcessResult,
) -> None:
    source_entry = job.metadata.get("kb_source_entry", "")
    if not source_entry:
        return
    cur.execute(
        """
        UPDATE forensic.kb_collection_assets
        SET structured_status='completed',
            routing_decision=routing_decision || %s::jsonb,
            quality_report=quality_report || %s::jsonb,
            updated_at=now()
        WHERE tenant_id=%s AND collection_id=%s AND evidence_id=%s::uuid
        """,
        (
            json.dumps(
                {
                    "storage_mode": "hybrid",
                    "modality": "document",
                    "processing_route": "native_document_worker",
                    "processor": result.processor_id,
                    "passage_count": len(result.passages),
                }
            ),
            json.dumps({"limitations": list(result.limitations), "metadata": result.metadata}),
            job.tenant_id,
            job.collection_id,
            job.evidence_id,
        ),
    )


def mark_document_job_completed(
    cur: psycopg.Cursor[Any],
    job: IngestJob,
    result: DocumentProcessResult,
) -> None:
    passage_count = len(result.passages)
    cur.execute(
        """
        UPDATE forensic.records_ingest_jobs
        SET status='completed', completed_at=now(), total_rows=%s, accepted_rows=%s,
            duplicate_rows=0, rejected_rows=0,
            worker_lease_token=NULL, worker_lease_owner=NULL, worker_lease_expires_at=NULL,
            next_attempt_at=NULL, last_error_class=NULL, error_message=NULL,
            metadata=metadata || jsonb_build_object(
              'document_readiness', %s::text,
              'document_processor', %s::text,
              'document_processor_revision', %s::text,
              'document_limitations', %s::jsonb
            )
        WHERE id=%s::uuid
        """,
        (
            passage_count,
            passage_count,
            result.readiness,
            result.processor_id,
            result.processor_revision,
            json.dumps(list(result.limitations)),
            job.job_id,
        ),
    )


def persist_media_result(
    cur: psycopg.Cursor[Any],
    job: IngestJob,
    version_id: str,
    result: MediaProcessResult,
) -> None:
    for index, observation in enumerate(result.observations, start=1):
        metadata = {
            "observation_id": observation.observation_id,
            "observation_contract": observation.contract_version,
            "observation_type": observation.observation_type,
            "observation": observation.payload,
            "candidate": observation.payload.get("candidate"),
            "crop": observation.payload.get("crop"),
            "face_crop": observation.payload.get("face_crop_artifact"),
            "processor": result.processor_id,
            "processor_revision": result.processor_revision,
            "source_truth_state": (
                "derived_text_representation"
                if observation.contract_version == ROMAN_URDU_SEGMENT_CONTRACT
                else "derived_model_observation"
                if observation.contract_version in {
                    ANPR_OBSERVATION_CONTRACT,
                    AUDIO_TIMESTAMP_SEGMENT_CONTRACT,
                    FACE_OBSERVATION_CONTRACT,
                    IMAGE_EMBEDDING_CONTRACT,
                    IMAGE_OCR_OBSERVATION_CONTRACT,
                }
                else "derived_technical_observation"
            ),
        }
        cur.execute(
            """
            INSERT INTO forensic.derived_artifacts (
              artifact_id, tenant_id, collection_id, evidence_id, version_id, run_id,
              artifact_type, media_type, processing_status, confidence,
              citation_locator, metadata, warnings
            ) VALUES (
              %s::uuid, %s, %s, %s::uuid, %s::uuid, %s::uuid,
              %s, 'application/json', 'completed', %s,
              %s::jsonb, %s::jsonb, %s::jsonb
            )
            ON CONFLICT (artifact_id) DO NOTHING
            """,
            (
                observation.observation_id,
                job.tenant_id,
                job.collection_id,
                job.evidence_id,
                version_id,
                job.job_id,
                observation.contract_version,
                observation.confidence,
                json.dumps(observation.citation_locator),
                json.dumps(metadata),
                json.dumps(list(observation.warnings)),
            ),
        )
        if observation.contract_version == ANPR_OBSERVATION_CONTRACT:
            persist_anpr_media_observation(cur, job, observation, index)

    summary = {
        "contract_version": result.contract_version,
        "readiness": result.readiness,
        "processor": result.processor_id,
        "processor_revision": result.processor_revision,
        "observation_count": len(result.observations),
        "limitations": list(result.limitations),
        "metadata": result.metadata,
    }
    cur.execute(
        """
        UPDATE forensic.evidence_items
        SET processing_status='completed',
            processing_route='unified_media_worker',
            metadata=metadata || jsonb_build_object('media_processing', %s::jsonb),
            warnings=warnings || %s::jsonb
        WHERE tenant_id=%s AND collection_id=%s AND evidence_id=%s::uuid
        """,
        (
            json.dumps(summary),
            json.dumps(list(result.limitations)),
            job.tenant_id,
            job.collection_id,
            job.evidence_id,
        ),
    )


def persist_anpr_media_observation(
    cur: psycopg.Cursor[Any],
    job: IngestJob,
    observation: Any,
    row_number: int,
) -> None:
    raw_plate = str(observation.payload.get("raw_plate_text") or "")
    normalized_plate = str(observation.payload.get("normalized_plate_text") or "")
    if not normalized_plate:
        return
    raw_payload = {
        "plate": raw_plate,
        "ocr_confidence": observation.payload.get("ocr_confidence"),
        "detection_confidence": observation.payload.get("detection_confidence"),
        "source_locator": observation.citation_locator,
        "observation_id": observation.observation_id,
        "observation_state": "model_candidate",
    }
    row_hash = hashlib.sha256(
        json.dumps(raw_payload, sort_keys=True, separators=(",", ":")).encode("utf-8")
    ).hexdigest()
    normalized_fields = {
        "plate_raw": raw_plate,
        "plate_search_key": normalized_plate,
        "ocr_confidence": observation.payload.get("ocr_confidence"),
        "manual_review_required": True,
        "quality_flags": ["model_observation", "pakistan_ocr_accuracy_not_certified"],
        "canonical_time_state": "missing",
    }
    cur.execute(
        """
        INSERT INTO forensic.records (
          tenant_id, collection_id, file_id, batch_id, record_type, "timestamp",
          primary_target, secondary_target, source_file, row_number, row_hash,
          raw_payload, evidence_id, metadata
        ) VALUES (
          %s, %s, %s, %s::uuid, 'anpr', now(),
          %s, NULL, %s, %s, %s, %s::jsonb, %s::uuid,
          jsonb_build_object(
            'normalized_fields', %s::jsonb,
            'source_fields', jsonb_build_object('primary_target', 'model_observation'),
            'derived_observation_id', %s::text,
            'derived_from_media', true
          )
        ) ON CONFLICT DO NOTHING
        """,
        (
            job.tenant_id,
            job.collection_id,
            job.file_id,
            job.job_id,
            normalized_plate,
            job.source_file,
            row_number,
            row_hash,
            json.dumps(raw_payload),
            job.evidence_id,
            json.dumps(normalized_fields),
            observation.observation_id,
        ),
    )


def materialize_media_kb_asset(
    cur: psycopg.Cursor[Any],
    job: IngestJob,
    result: MediaProcessResult,
) -> None:
    source_entry = job.metadata.get("kb_source_entry", "")
    if not source_entry:
        return
    routing = {
        "storage_mode": "hybrid",
        "modality": result.modality,
        "processing_route": "unified_media_worker",
        "processor": result.processor_id,
        "readiness": result.readiness,
        "observation_count": len(result.observations),
    }
    cur.execute(
        """
        INSERT INTO forensic.kb_collection_assets (
          tenant_id, user_id, collection_id, file_id, batch_id, source_file,
          source_entry, sha256, detected_record_type, requested_record_type, storage_mode,
          rag_status, structured_status, content_type, size_bytes, evidence_id, headers,
          routing_decision, quality_report
        ) VALUES (
          %s, %s, %s, %s, %s::uuid, %s, %s, %s,
          'generic'::forensic_record_type, 'generic'::forensic_record_type, 'hybrid',
          'mirrored', 'completed', %s, %s, %s::uuid, '[]'::jsonb, %s::jsonb, %s::jsonb
        )
        ON CONFLICT (tenant_id, collection_id, file_id, sha256)
        DO UPDATE SET
          batch_id=EXCLUDED.batch_id,
          rag_status=EXCLUDED.rag_status,
          structured_status=EXCLUDED.structured_status,
          routing_decision=forensic.kb_collection_assets.routing_decision || EXCLUDED.routing_decision,
          quality_report=forensic.kb_collection_assets.quality_report || EXCLUDED.quality_report,
          updated_at=now()
        """,
        (
            job.tenant_id,
            job.user_id,
            job.collection_id,
            job.file_id,
            job.job_id,
            job.source_file,
            source_entry,
            job.sha256,
            job.metadata.get("content_type"),
            parse_int_optional(job.metadata.get("size_bytes")),
            job.evidence_id,
            json.dumps(routing),
            json.dumps({"limitations": list(result.limitations), "metadata": result.metadata}),
        ),
    )


def mark_media_job_completed(
    cur: psycopg.Cursor[Any],
    job: IngestJob,
    result: MediaProcessResult,
) -> None:
    observation_count = len(result.observations)
    cur.execute(
        """
        UPDATE forensic.records_ingest_jobs
        SET status='completed', completed_at=now(), total_rows=%s, accepted_rows=%s,
            duplicate_rows=0, rejected_rows=0,
            worker_lease_token=NULL, worker_lease_owner=NULL, worker_lease_expires_at=NULL,
            next_attempt_at=NULL, last_error_class=NULL, error_message=NULL,
            metadata=metadata || jsonb_build_object(
              'media_readiness', %s::text,
              'media_processor', %s::text,
              'media_processor_revision', %s::text,
              'media_limitations', %s::jsonb,
              'media_result_states', %s::jsonb
            )
        WHERE id=%s::uuid
        """,
        (
            observation_count,
            observation_count,
            result.readiness,
            result.processor_id,
            result.processor_revision,
            json.dumps(list(result.limitations)),
            json.dumps(result.metadata.get("result_states") or {}),
            job.job_id,
        ),
    )


def storage_scope_key(tenant_id: str, collection_id: str) -> str:
    tenant = tenant_id.encode("utf-8")
    collection = collection_id.encode("utf-8")
    payload = str(len(tenant)).encode() + b":" + tenant + str(len(collection)).encode() + b":" + collection
    return hashlib.sha256(payload).hexdigest()


def validate_job_storage(job: IngestJob, spool_root: Path | None = None) -> Path:
    root = (spool_root or SPOOL_ROOT).resolve(strict=True)
    supplied = Path(job.spool_path)
    if supplied.is_symlink():
        raise ValueError("spool path must not be a symbolic link")
    resolved = supplied.resolve(strict=True)
    if not resolved.is_relative_to(root):
        raise ValueError("spool path escapes the configured evidence storage root")

    expected_size: int | None = None
    if job.metadata.get("size_bytes", "").strip():
        try:
            expected_size = int(job.metadata["size_bytes"])
        except ValueError as exc:
            raise ValueError("invalid retained evidence size metadata") from exc
        if expected_size < 0:
            raise ValueError("invalid retained evidence size metadata")

    layout = job.metadata.get("storage_layout_version", "").strip()
    if layout:
        if expected_size is None:
            raise ValueError("retained evidence size metadata is required")
        if layout != CONTENT_ADDRESSED_LAYOUT_VERSION:
            raise ValueError("unsupported retained evidence storage layout")
        uri = job.metadata.get("storage_uri", "").strip()
        match = CONTENT_ADDRESSED_URI_RE.fullmatch(uri)
        if match is None:
            raise ValueError("invalid retained evidence storage URI")
        scope_key, uri_hash = match.groups()
        expected_scope = storage_scope_key(job.tenant_id, job.collection_id)
        if scope_key != expected_scope or job.metadata.get("storage_scope_key", "") != expected_scope:
            raise ValueError("retained evidence scope identity mismatch")
        if uri_hash != job.sha256:
            raise ValueError("retained evidence URI hash mismatch")
        expected_path = root / "objects" / scope_key / uri_hash[:2] / uri_hash
        if expected_path.is_symlink() or resolved != expected_path:
            raise ValueError("retained evidence path does not match its content address")
        if job.metadata.get("storage_integrity_state") != "verified":
            raise ValueError("retained evidence is not marked verified")
        if job.metadata.get("storage_write_once") != "true":
            raise ValueError("retained evidence is not marked write-once")
        validate_storage_receipt(job, root, scope_key, uri_hash, expected_size)

    mode = resolved.stat().st_mode
    if layout and mode & (stat.S_IWUSR | stat.S_IWGRP | stat.S_IWOTH):
        raise ValueError("retained evidence object is writable")
    assert_sha256(resolved, job.sha256, expected_size)
    return resolved


def validate_storage_receipt(
    job: IngestJob,
    root: Path,
    scope_key: str,
    content_hash: str,
    expected_size: int | None,
) -> None:
    receipt_uri = job.metadata.get("storage_receipt_uri", "").strip()
    expected_receipt_uri = (
        f"forensic-spool-receipt://{CONTENT_ADDRESSED_LAYOUT_VERSION}/{scope_key}/{content_hash}"
    )
    if receipt_uri != expected_receipt_uri:
        raise ValueError("retained evidence receipt URI mismatch")
    receipt_path = root / "receipts" / scope_key / content_hash[:2] / f"{content_hash}.json"
    if receipt_path.is_symlink():
        raise ValueError("retained evidence receipt must not be a symbolic link")
    receipt_mode = receipt_path.stat().st_mode
    if receipt_mode & (stat.S_IWUSR | stat.S_IWGRP | stat.S_IWOTH):
        raise ValueError("retained evidence receipt is writable")
    if receipt_path.stat().st_size > 64 * 1024:
        raise ValueError("retained evidence receipt exceeds 64 KiB")
    with receipt_path.open(encoding="utf-8") as fh:
        receipt = json.load(fh)
    required_keys = {
        "layout_version",
        "scope_key",
        "tenant_id",
        "collection_id",
        "sha256",
        "size_bytes",
        "storage_uri",
        "retained_at",
    }
    if set(receipt) != required_keys:
        raise ValueError("retained evidence receipt fields are invalid")
    if (
        receipt["layout_version"] != CONTENT_ADDRESSED_LAYOUT_VERSION
        or receipt["scope_key"] != scope_key
        or receipt["tenant_id"] != job.tenant_id
        or receipt["collection_id"] != job.collection_id
        or receipt["sha256"] != content_hash
        or receipt["size_bytes"] != expected_size
        or receipt["storage_uri"] != job.metadata.get("storage_uri")
        or not isinstance(receipt["retained_at"], str)
        or not receipt["retained_at"].strip()
    ):
        raise ValueError("retained evidence receipt identity mismatch")


def assert_sha256(path: Path, expected: str, expected_size: int | None = None) -> None:
    if not re.fullmatch(r"[0-9a-f]{64}", expected):
        raise ValueError("expected sha256 must be 64 lowercase hexadecimal characters")
    digest = hashlib.sha256()
    size = 0
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1024 * 1024), b""):
            digest.update(chunk)
            size += len(chunk)
    if expected_size is not None and size != expected_size:
        raise ValueError(f"size mismatch for retained evidence: expected {expected_size}, got {size}")
    actual = digest.hexdigest()
    if actual != expected:
        raise ValueError(f"sha256 mismatch for retained evidence: expected {expected}, got {actual}")


def retry_backoff_seconds(attempt_count: int) -> float:
    return float(min(300, 2 ** max(1, attempt_count)))


def classify_job_error(exc: Exception) -> str:
    if isinstance(exc, (ValueError, UnicodeError, csv.Error, json.JSONDecodeError, PermissionError)):
        return "permanent_input"
    if psycopg is not None and isinstance(exc, psycopg.OperationalError):
        return "transient_database"
    if isinstance(exc, (ConnectionError, TimeoutError, OSError)):
        return "transient_io"
    return "transient_processing"


def claim_job(job: IngestJob) -> JobClaim:
    with psycopg.connect(DATABASE_URL, autocommit=False) as conn:
        conn.execute("SELECT set_config(%s, %s, true)", (TENANT_SETTING, job.tenant_id))
        ensure_job_row(conn, job)
        row = conn.execute(
            """
            SELECT status::text, attempt_count, max_attempts,
                   worker_lease_expires_at, next_attempt_at, reprocess_generation
            FROM forensic.records_ingest_jobs
            WHERE id=%s::uuid
            FOR UPDATE
            """,
            (job.job_id,),
        ).fetchone()
        if row is None:
            raise RuntimeError("ingest job disappeared during claim")
        status, attempt_count, max_attempts, lease_expires_at, next_attempt_at, generation = row
        now = datetime.now(timezone.utc)
        if status == "completed":
            conn.commit()
            return JobClaim("completed", attempt_count=attempt_count, max_attempts=max_attempts, reprocess_generation=generation)
        if status == "dead_letter":
            conn.commit()
            return JobClaim("dead_letter", attempt_count=attempt_count, max_attempts=max_attempts, reprocess_generation=generation)
        if status == "running" and lease_expires_at is not None and lease_expires_at > now:
            delay = max(0.1, min(30.0, (lease_expires_at - now).total_seconds()))
            conn.commit()
            return JobClaim("busy", attempt_count=attempt_count, max_attempts=max_attempts, reprocess_generation=generation, delay_seconds=delay)
        if next_attempt_at is not None and next_attempt_at > now:
            delay = max(0.1, min(300.0, (next_attempt_at - now).total_seconds()))
            conn.commit()
            return JobClaim("deferred", attempt_count=attempt_count, max_attempts=max_attempts, reprocess_generation=generation, delay_seconds=delay)
        if attempt_count >= max_attempts:
            conn.execute(
                """
                UPDATE forensic.records_ingest_jobs
                SET status='dead_letter', dead_lettered_at=coalesce(dead_lettered_at, now()),
                    completed_at=coalesce(completed_at, now()),
                    worker_lease_token=NULL, worker_lease_owner=NULL,
                    worker_lease_expires_at=NULL, next_attempt_at=NULL
                WHERE id=%s::uuid
                """,
                (job.job_id,),
            )
            conn.commit()
            return JobClaim("dead_letter", attempt_count=attempt_count, max_attempts=max_attempts, reprocess_generation=generation)

        lease_token = str(uuid.uuid4())
        new_attempt = attempt_count + 1
        conn.execute(
            """
            UPDATE forensic.records_ingest_jobs
            SET status='running', attempt_count=%s, started_at=now(),
                completed_at=NULL, error_message=NULL, last_error_class=NULL,
                next_attempt_at=NULL, worker_lease_token=%s::uuid,
                worker_lease_owner=%s,
                worker_lease_expires_at=now() + (%s * interval '1 second'),
                metadata=metadata || jsonb_build_object(
                  'queue_last_claimed_at', now(),
                  'queue_last_worker_id', %s::text,
                  'queue_last_attempt', %s::integer
                )
            WHERE id=%s::uuid
            """,
            (new_attempt, lease_token, WORKER_ID, WORKER_LEASE_SECONDS, WORKER_ID, new_attempt, job.job_id),
        )
        conn.commit()
        return JobClaim(
            "claimed", lease_token=lease_token, attempt_count=new_attempt,
            max_attempts=max_attempts, reprocess_generation=generation,
        )


def ensure_job_row(conn: psycopg.Connection[Any], job: IngestJob) -> None:
    conn.execute(
        """
        INSERT INTO forensic.records_ingest_jobs (
          id, tenant_id, user_id, collection_id, file_id, source_file,
          spool_path, sha256, record_type, status, evidence_id, metadata,
          queue_message_id, queue_published_at
        )
        VALUES (%s::uuid, %s, %s, %s, %s, %s, %s, %s, %s::forensic_record_type,
                'queued', NULLIF(%s, '')::uuid, %s::jsonb, %s, now())
        ON CONFLICT (id) DO UPDATE SET
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
            f"ingest-job:{job.job_id}",
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


def verify_job_lease(conn: psycopg.Connection[Any], job: IngestJob, lease_token: str) -> None:
    row = conn.execute(
        """
        SELECT 1
        FROM forensic.records_ingest_jobs
        WHERE id=%s::uuid AND status='running' AND worker_lease_token=%s::uuid
        FOR UPDATE
        """,
        (job.job_id, lease_token),
    ).fetchone()
    if row is None:
        raise RuntimeError("worker lease was lost before processing began")


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


def mark_job_failed(
    job: IngestJob,
    lease_token: str,
    message: str,
    error_class: str,
) -> FailureDisposition:
    with psycopg.connect(DATABASE_URL, autocommit=False) as conn:
        conn.execute("SELECT set_config(%s, %s, true)", (TENANT_SETTING, job.tenant_id))
        row = conn.execute(
            """
            SELECT attempt_count, max_attempts
            FROM forensic.records_ingest_jobs
            WHERE id=%s::uuid AND worker_lease_token=%s::uuid
            FOR UPDATE
            """,
            (job.job_id, lease_token),
        ).fetchone()
        if row is None:
            raise RuntimeError("worker lease was lost before failure could be recorded")
        attempt_count, max_attempts = row
        terminal = error_class == "permanent_input" or attempt_count >= max_attempts
        job_status = "dead_letter" if terminal else "failed"
        disposition = "dead_letter" if terminal else "retry"
        delay_seconds = 0.0 if terminal else retry_backoff_seconds(attempt_count)
        conn.execute(
            """
            UPDATE forensic.records_ingest_jobs
            SET status=%s::forensic_ingest_status,
                error_message=%s, last_error_class=%s,
                completed_at=CASE WHEN %s THEN now() ELSE NULL END,
                dead_lettered_at=CASE WHEN %s THEN now() ELSE NULL END,
                next_attempt_at=CASE WHEN %s THEN NULL ELSE now() + (%s * interval '1 second') END,
                worker_lease_token=NULL, worker_lease_owner=NULL,
                worker_lease_expires_at=NULL,
                metadata=metadata || jsonb_build_object(
                  'queue_last_failure_at', now(),
                  'queue_last_error_class', %s::text,
                  'queue_last_attempt', %s::integer
                )
            WHERE id=%s::uuid AND worker_lease_token=%s::uuid
            """,
            (
                job_status, message[:4000], error_class, terminal, terminal,
                terminal, delay_seconds, error_class, attempt_count,
                job.job_id, lease_token,
            ),
        )
        if job.evidence_id:
            conn.execute(
                """
                UPDATE forensic.evidence_items
                SET processing_status=%s,
                    errors = errors || jsonb_build_array(
                      jsonb_build_object(
                        'message', %s::text, 'recorded_at', now(),
                        'job_id', %s::text, 'attempt_count', %s::integer,
                        'error_class', %s::text
                      )
                    ),
                    metadata = metadata || %s::jsonb
                WHERE evidence_id=%s::uuid
                """,
                (
                    "failed" if terminal else "queued",
                    message[:4000], job.job_id, attempt_count, error_class,
                    json.dumps(
                        {
                            "failed_job_id": job.job_id,
                            "request_id": job.request_id,
                            "queue_disposition": disposition,
                            "queue_error_class": error_class,
                            "queue_attempt_count": attempt_count,
                            "queue_next_attempt_seconds": delay_seconds,
                        }
                    ),
                    job.evidence_id,
                ),
            )
        conn.execute(
            """
            INSERT INTO forensic.records_audit_log
              (tenant_id, user_id, action, collection_id, file_id, batch_id, details)
            VALUES (%s, %s, %s, %s, %s, %s::uuid, %s::jsonb)
            """,
            (
                job.tenant_id, job.user_id,
                "records.ingest.dead_lettered" if terminal else "records.ingest.retry_scheduled",
                job.collection_id, job.file_id, job.job_id,
                json.dumps(
                    {
                        "attempt_count": attempt_count,
                        "max_attempts": max_attempts,
                        "error_class": error_class,
                        "delay_seconds": delay_seconds,
                        "request_id": job.request_id,
                        "evidence_id": job.evidence_id,
                    }
                ),
            ),
        )
        conn.commit()
        return FailureDisposition(
            disposition=disposition,
            attempt_count=attempt_count,
            max_attempts=max_attempts,
            error_class=error_class,
            delay_seconds=delay_seconds,
        )


def mark_job_acknowledged(job: IngestJob) -> None:
    with psycopg.connect(DATABASE_URL, autocommit=False) as conn:
        conn.execute("SELECT set_config(%s, %s, true)", (TENANT_SETTING, job.tenant_id))
        conn.execute(
            """
            UPDATE forensic.records_ingest_jobs
            SET acknowledged_at=coalesce(acknowledged_at, now())
            WHERE id=%s::uuid AND status='completed'
            """,
            (job.job_id,),
        )
        conn.commit()



def source_extension(path: Path, source_name: str | None = None) -> str:
    return Path(source_name).suffix.lower() if source_name else path.suffix.lower()


def profile_records(path: Path, source_name: str | None = None) -> dict[str, Any]:
    ext = source_extension(path, source_name)
    if ext == ".xlsx":
        profile = profile_xlsx(path)
        profile["classification"] = classify_columns(profile["headers"], profile["sample_records"])
        samples_by_sheet: dict[int, list[dict[str, Any]]] = {}
        for sample in profile["sample_records"]:
            sheet_index = parse_int_optional(sample.get("_xlsx_sheet_index"))
            if sheet_index is not None:
                samples_by_sheet.setdefault(sheet_index, []).append(sample)
        for sheet_index, sheet in enumerate(profile["workbook"]["sheets"], start=1):
            sheet["classification"] = classify_columns(
                sheet.get("headers") or [],
                samples_by_sheet.get(sheet_index, []),
            )
        return profile
    if ext in {".json", ".jsonl", ".ndjson"}:
        return profile_json(path, ext)
    if ext == ".parquet":
        return profile_parquet(path)
    return profile_csv(path, source_name)


def profile_csv(path: Path, source_name: str | None = None) -> dict[str, Any]:
    encoding = detect_tabular_encoding(path)
    delimiter = detect_tabular_delimiter(path, source_name, encoding)
    headers, source_headers, records = read_delimited_records(path, encoding, delimiter, 5000)
    classification = classify_columns(headers, records[:500])
    return {
        "headers": headers,
        "source_headers": source_headers,
        "sample_rows": len(records),
        "schema": {name: "source_text" for name in headers},
        "sample_records": records[:500],
        "classification": classification,
        "tabular_dialect": {
            "delimiter": "\\t" if delimiter == "\t" else delimiter,
            "encoding": encoding,
            "duplicate_or_blank_headers": headers != source_headers,
        },
    }


def detect_tabular_encoding(path: Path) -> str:
    sample = path.read_bytes()[:65536]
    if sample.startswith((b"\xff\xfe", b"\xfe\xff")):
        return "utf-16"
    if sample.startswith(b"\xef\xbb\xbf"):
        return "utf-8-sig"
    if sample:
        even_nuls = sample[0::2].count(0)
        odd_nuls = sample[1::2].count(0)
        if max(even_nuls, odd_nuls) > max(2, len(sample) // 10):
            return "utf-16-le" if odd_nuls > even_nuls else "utf-16-be"
    try:
        sample.decode("utf-8")
        return "utf-8-sig"
    except UnicodeDecodeError:
        return "cp1252"


def detect_tabular_delimiter(path: Path, source_name: str | None, encoding: str) -> str:
    if source_extension(path, source_name) == ".tsv":
        return "\t"
    with path.open("r", newline="", encoding=encoding) as fh:
        sample = fh.read(65536)
    try:
        return csv.Sniffer().sniff(sample, delimiters=",\t;|").delimiter
    except csv.Error:
        return ","


def disambiguate_headers(source_headers: list[str]) -> list[str]:
    headers: list[str] = []
    counts: dict[str, int] = {}
    for index, source_header in enumerate(source_headers, start=1):
        base = source_header.strip() or f"_column_{index}"
        counts[base] = counts.get(base, 0) + 1
        headers.append(base if counts[base] == 1 else f"{base}__duplicate_{counts[base]}")
    return headers


def read_delimited_records(
    path: Path,
    encoding: str,
    delimiter: str,
    limit: int | None = None,
) -> tuple[list[str], list[str], list[dict[str, Any]]]:
    records: list[dict[str, Any]] = []
    with path.open("r", newline="", encoding=encoding) as fh:
        reader = csv.reader(fh, delimiter=delimiter)
        source_headers = next(reader, [])
        headers = disambiguate_headers(source_headers)
        for values in reader:
            row = {header: values[index] if index < len(values) else "" for index, header in enumerate(headers)}
            for index, value in enumerate(values[len(headers):], start=1):
                row[f"_extra_column_{index}"] = value
            records.append(row)
            if limit is not None and len(records) >= limit:
                break
    return headers, source_headers, records


def iter_delimited_records(path: Path, encoding: str, delimiter: str) -> Iterable[dict[str, Any]]:
    with path.open("r", newline="", encoding=encoding) as fh:
        reader = csv.reader(fh, delimiter=delimiter)
        headers = disambiguate_headers(next(reader, []))
        for values in reader:
            row = {header: values[index] if index < len(values) else "" for index, header in enumerate(headers)}
            for index, value in enumerate(values[len(headers):], start=1):
                row[f"_extra_column_{index}"] = value
            yield row


def profile_cdr_csv(path: Path) -> dict[str, Any]:
    return profile_records(path)


def profile_json(path: Path, source_ext: str | None = None) -> dict[str, Any]:
    records = []
    ext = source_ext or path.suffix.lower()
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


def iter_records(path: Path, source_name: str | None = None) -> Iterable[dict[str, Any]]:
    ext = source_extension(path, source_name)
    if ext == ".xlsx":
        yield from iter_xlsx_records(path)
        return
    if ext in {".json", ".jsonl", ".ndjson"}:
        yield from iter_json_records(path, ext)
        return
    if ext == ".parquet":
        yield from iter_parquet_records(path)
        return
    encoding = detect_tabular_encoding(path)
    delimiter = detect_tabular_delimiter(path, source_name, encoding)
    yield from iter_delimited_records(path, encoding, delimiter)


def iter_json_records(path: Path, source_ext: str | None = None) -> Iterable[dict[str, Any]]:
    ext = source_ext or path.suffix.lower()
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
    timestamp_header = any(hint in normalized for hint in TIMESTAMP_HINTS)
    scores = {
        "timestamp": 0.15 if timestamp_header else 0.0,
        "phone": 0.2 if any(term in normalized for term in ("msisdn", "phone", "mobile", "caller", "callee", "dialed")) else 0.0,
        "ip": 0.2 if "ip" in normalized else 0.0,
        "email": 0.2 if "email" in normalized or "mail" in normalized else 0.0,
        "imei_imsi": 0.2 if "imei" in normalized or "imsi" in normalized else 0.0,
        "vehicle": 0.2 if any(term in normalized for term in ("plate", "vehicle", "registration")) else 0.0,
        "identifier": 0.1 if any(term in normalized for term in ("id", "target", "entity", "account", "user")) else 0.0,
    }
    total = len(values)
    for value in values:
        if timestamp_header or looks_like_timestamp_value(value):
            try:
                parsed_timestamp = parse_ts_any(value)
            except StructuredTimeError:
                # Profiling identifies syntactically timestamp-shaped values;
                # policy resolution happens later with the source declaration.
                parsed_timestamp = True
            except ValueError:
                parsed_timestamp = None
            if parsed_timestamp is not None:
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


def looks_like_timestamp_value(value: str) -> bool:
    text = clean_text(value)
    if looks_like_epoch(text):
        return True
    return bool(
        re.match(r"^\d{4}-\d{1,2}-\d{1,2}(?:[ T]|$)", text)
        or re.match(r"^\d{1,2}/\d{1,2}/\d{4}(?:[ T]|$)", text)
    )


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


def normalize_ipdr_row(
    job: IngestJob,
    raw: dict[str, Any],
    batch_id: str,
    row_number: int,
) -> dict[str, Any]:
    return normalize_generic_row(job, raw, batch_id, row_number, ADAPTERS["ipdr"])


def normalize_anpr_row(
    job: IngestJob,
    raw: dict[str, Any],
    batch_id: str,
    row_number: int,
) -> dict[str, Any]:
    return normalize_generic_row(job, raw, batch_id, row_number, ADAPTERS["anpr"])


def normalize_subscriber_row(
    job: IngestJob,
    raw: dict[str, Any],
    batch_id: str,
    row_number: int,
) -> dict[str, Any]:
    return normalize_generic_row(job, raw, batch_id, row_number, ADAPTERS["subscriber"])


def normalize_tower_row(
    job: IngestJob,
    raw: dict[str, Any],
    batch_id: str,
    row_number: int,
) -> dict[str, Any]:
    return normalize_generic_row(job, raw, batch_id, row_number, ADAPTERS["tower_location"])


@lru_cache(maxsize=1)
def compatibility_adapter_registry() -> CompatibilityAdapterRegistry:
    return CompatibilityAdapterRegistry(
        load_platform_catalog(),
        ADAPTERS,
        {
            "cdr": normalize_cdr_row,
            "ipdr": normalize_ipdr_row,
            "anpr": normalize_anpr_row,
            "subscriber": normalize_subscriber_row,
            "tower_location": normalize_tower_row,
            "generic": normalize_generic_row,
        },
        normalize_header,
    )


def resolve_adapter(job: IngestJob, headers: Iterable[str], force_generic: bool = False) -> RecordAdapter:
    return compatibility_adapter_registry().resolve(job.record_type, headers, force_generic)


def attach_family_schema_profile(profile: dict[str, Any], adapter: RecordAdapter, job: IngestJob | None = None) -> None:
    classification = profile.setdefault("classification", {})
    classification["family_mapping"] = build_schema_profile(
        adapter.name, profile.get("headers") or [], profile.get("sample_records") or [], normalize_header, clean_text
    )
    classification["schema_registry"] = {
        "contract_version": structured_registry_snapshot()["contract_version"],
        "mapping_profile_id": classification["family_mapping"]["mapping_profile_id"],
    }
    if job is not None:
        classification["time_policy"] = time_policy_for_job(job)


def validate_structured_profile_bounds(profile: dict[str, Any]) -> None:
    column_sets = [("source", profile.get("headers") or [])]
    for sheet in (profile.get("workbook") or {}).get("sheets") or []:
        column_sets.append((f"sheet {sheet.get('name') or '?'}", sheet.get("headers") or []))
    for label, headers in column_sets:
        if len(headers) > MAX_STRUCTURED_COLUMNS:
            raise ValueError(
                f"{label} has {len(headers)} columns; maximum supported structured width is {MAX_STRUCTURED_COLUMNS}"
            )


def prepare_xlsx_sheet_mappings(
    job: IngestJob,
    profile: dict[str, Any],
) -> tuple[dict[int, dict[str, Any]], RecordAdapter]:
    """Resolve each sheet independently while retaining unmatched sheets.

    Auto mode promotes only an unambiguous schema match. Explicit typed mode
    validates the requested adapter per sheet; nonmatching sheets remain generic
    and carry a review decision instead of being discarded or falsely coerced.
    """

    requested = normalize_header(job.record_type)
    mode = normalize_header(job.metadata.get("record_type_mode", ""))
    if mode not in {"auto", "explicit"}:
        mode = "explicit" if requested not in {"", "auto", "generic"} else "legacy_generic"

    mappings: dict[int, dict[str, Any]] = {}
    mapped_types: set[str] = set()
    workbook = profile.get("workbook") or {}
    for sheet_index, sheet in enumerate(workbook.get("sheets") or [], start=1):
        headers = list(sheet.get("headers") or [])
        candidates = [
            name
            for name, candidate in ADAPTERS.items()
            if name != "generic" and candidate.matches(headers)
        ]
        adapter = ADAPTERS["generic"]
        decision = "generic_requested"
        review_required = False

        if mode == "auto":
            if len(candidates) == 1:
                adapter = ADAPTERS[candidates[0]]
                decision = "auto_typed_unique_schema"
            elif len(candidates) > 1:
                decision = "auto_generic_ambiguous_schema"
                review_required = True
            else:
                decision = "auto_generic_no_schema_match"
        elif mode == "explicit" and requested in ADAPTERS and requested != "generic":
            if ADAPTERS[requested].matches(headers):
                adapter = ADAPTERS[requested]
                decision = "explicit_typed_schema_validated"
            else:
                decision = "explicit_type_mismatch_preserved_generic"
                review_required = True
        elif mode == "legacy_generic":
            decision = "legacy_generic_preserved"

        mapping = {
            "sheet_index": sheet_index,
            "sheet_name": sheet.get("name") or "",
            "sheet_state": sheet.get("state") or "visible",
            "header_row": sheet.get("header_row"),
            "headers": headers,
            "header_fingerprint": hashlib.sha256(
                json.dumps(headers, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
            ).hexdigest(),
            "requested_record_type": requested or "generic",
            "record_type_mode": mode,
            "detected_record_type": adapter.name,
            "matching_adapters": candidates,
            "decision": decision,
            "review_required": review_required,
            "classification": sheet.get("classification") or {},
        }
        mapping["classification"]["family_mapping"] = build_schema_profile(
            adapter.name,
            headers,
            sheet.get("sample_records") or [],
            normalize_header,
            clean_text,
        )
        mapping["classification"]["schema_registry"] = {
            "contract_version": structured_registry_snapshot()["contract_version"],
            "mapping_profile_id": mapping["classification"]["family_mapping"]["mapping_profile_id"],
        }
        mapping["classification"]["time_policy"] = time_policy_for_job(job)
        sheet["mapping"] = mapping
        mappings[sheet_index] = mapping
        if int(sheet.get("data_row_count") or 0) > 0:
            mapped_types.add(adapter.name)

    workbook["sheet_mapping_policy"] = "per_sheet_schema_v1"
    workbook["mapped_record_types"] = sorted(mapped_types)
    workbook["mapping_review_required"] = any(mapping["review_required"] for mapping in mappings.values())
    summary_adapter = ADAPTERS[next(iter(mapped_types))] if len(mapped_types) == 1 else ADAPTERS["generic"]
    return mappings, summary_adapter


def xlsx_row_adapter(
    raw: dict[str, Any],
    mappings: dict[int, dict[str, Any]],
    fallback: RecordAdapter,
) -> tuple[RecordAdapter, dict[str, Any] | None]:
    sheet_index = parse_int_optional(raw.get("_xlsx_sheet_index"))
    mapping = mappings.get(sheet_index or -1)
    if mapping is None:
        return fallback, None
    return ADAPTERS.get(str(mapping.get("detected_record_type")), ADAPTERS["generic"]), mapping


def validate_cdr_headers(headers: Iterable[str]) -> None:
    validate_cdr_adapter_headers(headers, normalize_header)


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
          normalized_record jsonb,
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
        "rejection_codes": {},
    }
    rejects: list[tuple[int, str, str, dict[str, str]]] = []
    copy_sql = sql.SQL("COPY tmp_cdr_records ({}) FROM STDIN").format(
        sql.SQL(", ").join(sql.Identifier(col) for col in COPY_COLUMNS)
    )
    with cur.copy(copy_sql) as copy:
        for line_number, raw in enumerate(iter_records(path, job.source_file), start=2):
            stats["total_rows"] += 1
            try:
                normalized = compatibility_adapter_registry().normalize("cdr", job, raw, batch_id, line_number)
                attach_record_maturity(job, raw, normalized, profile, ADAPTERS["cdr"], line_number)
                copy.write_row([normalized[col] for col in COPY_COLUMNS])
                update_stats(stats, normalized)
                ROWS_TOTAL.labels("cdr", "accepted").inc()
                INGEST_ROWS_TOTAL.labels("cdr", "accepted").inc()
            except Exception as exc:  # noqa: BLE001 - row-level rejection
                stats["rejected_rows"] += 1
                ROWS_TOTAL.labels("cdr", "rejected").inc()
                INGEST_ROWS_TOTAL.labels("cdr", "rejected").inc()
                code = getattr(exc, "code", "cdr_row_parse_error")
                stats["rejection_codes"][code] = stats["rejection_codes"].get(code, 0) + 1
                rejects.append((line_number, code, str(exc), raw))
            if stats["total_rows"] % 1000 == 0:
                emit_progress(progress_cb, job, "running", stats, profile or {}, started_at)
    for line_number, code, message, raw in rejects:
        write_reject(cur, job, line_number, code, message, raw)
    return stats


def normalize_cdr_row(job: IngestJob, raw: dict[str, str], batch_id: str, row_number: int) -> dict[str, Any]:
    return normalize_cdr_record(
        job,
        raw,
        batch_id,
        row_number,
        CDRAdapterRuntime(
            normalize_header=normalize_header,
            clean_text=clean_text,
            source_timezone=source_timezone_for_job,
            parse_required_timestamp=lambda value, source_timezone: parse_required_ts(
                value, source_timezone, source_date_order_for_job(job)
            ),
            parse_timestamp=lambda value, source_timezone: parse_ts_any(
                value, source_timezone, source_date_order_for_job(job)
            ),
            parse_integer=parse_int_optional,
            parse_decimal=parse_decimal,
            parse_float=parse_float,
            clean_numeric_identifier=clean_numeric_identifier,
            normalize_phone=normalize_pk_phone,
            row_hash=row_hash,
        ),
    )


def parse_ts(value: str | None) -> datetime:
    text = clean_text(value)
    if not text:
        raise ValueError("timestamp is required")
    return datetime.strptime(text, "%Y-%m-%d %H:%M:%S").replace(tzinfo=timezone.utc)


def source_timezone_for_job(job: IngestJob) -> str:
    explicit = clean_text(job.metadata.get("source_timezone"))
    if explicit:
        return explicit
    allow_default = clean_text(job.metadata.get("allow_profile_timezone_default")).casefold() in {"1", "true", "yes"}
    if allow_default:
        return clean_text(job.metadata.get("profile_default_timezone")) or clean_text(
            os.getenv("FORENSIC_DEFAULT_SOURCE_TIMEZONE")
        )
    return ""


def source_date_order_for_job(job: IngestJob) -> str:
    order = clean_text(job.metadata.get("source_date_order")).upper()
    return order if order in {"DMY", "MDY", "YMD"} else ""


def _policy_state(
    value: str | None,
    *,
    allowed: set[str],
    has_value: bool,
    value_default: str,
    missing_default: str,
) -> str:
    state = clean_text(value).lower()
    if state in allowed:
        return state
    return value_default if has_value else missing_default


def time_policy_for_job(job: IngestJob) -> dict[str, Any]:
    timezone_name = source_timezone_for_job(job)
    date_order = source_date_order_for_job(job)
    return {
        "contract_version": TIME_POLICY_VERSION,
        "source_timezone": timezone_name or None,
        "timezone_state": _policy_state(
            job.metadata.get("source_timezone_state"),
            allowed={"source_declared", "analyst_confirmed", "profile_defaulted", "unknown"},
            has_value=bool(timezone_name), value_default="source_declared", missing_default="unknown",
        ),
        "date_order": date_order or None,
        "date_order_state": _policy_state(
            job.metadata.get("source_date_order_state"),
            allowed={"source_declared", "analyst_confirmed", "profile_defaulted", "unresolved"},
            has_value=bool(date_order), value_default="source_declared", missing_default="unresolved",
        ),
        "profile_default_allowed": clean_text(job.metadata.get("allow_profile_timezone_default")).casefold() in {"1", "true", "yes"},
        "jurisdiction_is_not_time_proof": True,
    }


class StructuredTimeError(ValueError):
    code = "invalid_timestamp_or_timezone"


class AmbiguousDateOrderError(StructuredTimeError):
    code = "ambiguous_date_order"


class UnknownSourceTimezoneError(StructuredTimeError):
    code = "unknown_source_timezone"


@lru_cache(maxsize=64)
def source_zone(assumed_timezone: str | None) -> tzinfo:
    timezone_name = clean_text(assumed_timezone)
    if not timezone_name:
        raise UnknownSourceTimezoneError("naive timestamp requires a declared or confirmed source timezone")
    if timezone_name in {"UTC", "Etc/UTC", "Z"}:
        return timezone.utc
    try:
        return ZoneInfo(timezone_name)
    except ZoneInfoNotFoundError as exc:
        # Pakistan has observed UTC+05:00 continuously since 2009. Keeping the
        # operational default available without an OS tzdata package makes
        # laptop validation deterministic; ZoneInfo remains authoritative when
        # it is available, including for historical timestamps.
        if timezone_name == "Asia/Karachi":
            return timezone(timedelta(hours=5), name="Asia/Karachi")
        raise ValueError(f"unknown source timezone {timezone_name!r}") from exc


def parse_required_ts(value: str | None, assumed_timezone: str | None = None, date_order: str | None = None) -> datetime:
    parsed = parse_ts_any(value, assumed_timezone, date_order)
    if parsed is None:
        raise ValueError("timestamp is required")
    return parsed


def parse_ts_any(value: str | None, assumed_timezone: str | None = None, date_order: str | None = None) -> datetime | None:
    text = clean_text(value)
    if not text:
        return None
    if looks_like_epoch(text):
        try:
            return parse_epoch(text)
        except (OverflowError, OSError, ValueError):
            return None
    slash = re.fullmatch(r"(\d{1,2})/(\d{1,2})/(\d{4})(?:[ T](\d{1,2}):(\d{2})(?::(\d{2}))?)?", text)
    if slash:
        first, second = int(slash.group(1)), int(slash.group(2))
        governed_order = clean_text(date_order).upper()
        if first > 12 and second <= 12:
            governed_order = "DMY"
        elif second > 12 and first <= 12:
            governed_order = "MDY"
        elif first <= 12 and second <= 12 and governed_order not in {"DMY", "MDY"}:
            raise AmbiguousDateOrderError(f"ambiguous slash date {text!r} requires an accepted DMY or MDY policy")
        if governed_order not in {"DMY", "MDY"}:
            return None
        fmt = "%d/%m/%Y" if governed_order == "DMY" else "%m/%d/%Y"
        if slash.group(4):
            fmt += " %H:%M" + (":%S" if slash.group(6) else "")
        try:
            parsed = datetime.strptime(text.replace("T", " "), fmt)
        except ValueError:
            return None
        return parsed.replace(tzinfo=source_zone(assumed_timezone)).astimezone(timezone.utc)
    try:
        parsed = datetime.fromisoformat(text.replace("Z", "+00:00"))
    except ValueError:
        return None
    if parsed.tzinfo is None:
        return parsed.replace(tzinfo=source_zone(assumed_timezone)).astimezone(timezone.utc)
    return parsed.astimezone(timezone.utc)


def timestamp_has_explicit_offset(value: str) -> bool:
    return bool(re.search(r"(?:Z|[+-]\d{2}:?\d{2})$", clean_text(value), re.IGNORECASE))


def timestamp_provenance(job: IngestJob, raw_value: str, canonical_value: datetime | None, mapping_profile_id: str) -> dict[str, Any]:
    text = clean_text(raw_value)
    policy = time_policy_for_job(job)
    slash = re.match(r"^(\d{1,2})/(\d{1,2})/(\d{4})", text)
    if slash:
        first, second = int(slash.group(1)), int(slash.group(2))
        if first > 12 and second <= 12:
            order, order_source = "DMY", "structure_proven"
        elif second > 12 and first <= 12:
            order, order_source = "MDY", "structure_proven"
        else:
            order, order_source = policy["date_order"], policy["date_order_state"]
    elif re.match(r"^\d{4}-\d{1,2}-\d{1,2}", text):
        order, order_source = "YMD", "format_explicit"
    else:
        order, order_source = None, "not_applicable"
    explicit_offset = timestamp_has_explicit_offset(text)
    return {
        "contract_version": TIME_POLICY_VERSION,
        "raw_value": text or None,
        "date_order": order,
        "date_order_source": order_source,
        "source_timezone": None if explicit_offset else policy["source_timezone"],
        "timezone_state": "source_declared_offset" if explicit_offset else policy["timezone_state"],
        "canonical_instant": canonical_value.isoformat() if canonical_value else None,
        "canonical_timezone": "UTC" if canonical_value else None,
        "mapping_profile_id": mapping_profile_id,
        "review_status": "resolved" if canonical_value else "needs_review",
    }


def attach_record_maturity(
    job: IngestJob,
    raw: dict[str, Any],
    normalized: dict[str, Any],
    profile: dict[str, Any] | None,
    row_adapter: RecordAdapter,
    row_number: int,
    sheet_mapping: dict[str, Any] | None = None,
) -> None:
    classification = (sheet_mapping or {}).get("classification") or (profile or {}).get("classification") or {}
    mapping = classification.get("family_mapping")
    if not isinstance(mapping, dict):
        mapping = build_schema_profile(row_adapter.name, raw.keys(), [raw], normalize_header, clean_text)
    details = json.loads(normalized.get("normalized_record") or "{}")
    combined = {**normalized, **details}
    details["schema_registry_version"] = structured_registry_snapshot()["contract_version"]
    details["mapping_profile_id"] = mapping.get("mapping_profile_id")
    details["dynamic_attributes"] = build_dynamic_attributes(
        raw,
        mapping,
        combined,
        evidence_id=job.evidence_id,
        version_id=clean_text(job.metadata.get("version_id")),
        source_file=job.source_file,
        row_number=row_number,
    )
    time_fields = list(mapping.get("time_fields") or [])
    if time_fields:
        canonical_field = time_fields[0]
        source_field = next((
            field.get("source_field") for field in mapping.get("source_columns", [])
            if field.get("canonical_field") == canonical_field
        ), None)
        raw_time = value_from_field(raw, source_field or "")
        canonical_time = combined.get(canonical_field)
        if canonical_field in {"session_start", "observed_at"}:
            canonical_time = normalized.get("observed_at")
        details["time_provenance"] = timestamp_provenance(
            job, raw_time, canonical_time if isinstance(canonical_time, datetime) else normalized.get("observed_at"),
            str(mapping.get("mapping_profile_id") or ""),
        )
    normalized["normalized_record"] = json.dumps(details, ensure_ascii=False)


def parse_decimal(value: str | None) -> Decimal | None:
    text = clean_text(value)
    if not text:
        return None
    try:
        return Decimal(text)
    except InvalidOperation as exc:
        raise ValueError(f"invalid decimal {text!r}") from exc


MONEY_FIELDS = (
    "amount",
    "transaction_amount",
    "txn_amount",
    "amount_pkr",
    "transaction_value",
    "local_amount",
    "debit_amount",
    "credit_amount",
)
ANPR_CONFIDENCE_FIELDS = ("ocr_confidence", "plate_confidence", "confidence", "recognition_confidence", "score")
ROW_TIMEZONE_FIELDS = ("source_timezone", "camera_timezone", "transaction_timezone", "timezone")
PK_IBAN_RE = re.compile(r"^PK\d{2}[A-Z]{4}[A-Z0-9]{16}$")
HEX_SHA256_RE = re.compile(r"(?i)^[0-9a-f]{64}$")


def parse_money_amount(value: str | None, currency_value: str | None = None) -> tuple[Decimal, str]:
    text = clean_text(value).replace("\u00a0", " ")
    if not text:
        raise ValueError("transaction amount is required")
    negative_parentheses = text.startswith("(") and text.endswith(")")
    if negative_parentheses:
        text = text[1:-1].strip()
    elif text.startswith("(") or text.endswith(")"):
        raise ValueError("transaction amount has unbalanced parentheses")

    embedded_currency = ""
    prefix = re.match(r"(?i)^(PKR|RS\.?)\s*", text)
    if prefix:
        embedded_currency = "PKR"
        text = text[prefix.end():].strip()
    elif text.startswith("₨"):
        embedded_currency = "PKR"
        text = text[1:].strip()

    declared_currency = clean_text(currency_value).upper().replace(".", "")
    if declared_currency in {"RS", "RUPEE", "RUPEES"}:
        declared_currency = "PKR"
    if declared_currency and not re.fullmatch(r"[A-Z]{3}", declared_currency):
        raise ValueError(f"invalid ISO currency code {currency_value!r}")
    if embedded_currency and declared_currency and embedded_currency != declared_currency:
        raise ValueError("embedded and declared transaction currencies disagree")
    currency = declared_currency or embedded_currency or "PKR"

    sign = ""
    if text[:1] in {"+", "-"}:
        sign, text = text[0], text[1:].strip()
    if negative_parentheses and sign:
        raise ValueError("transaction amount has two sign conventions")
    if not re.fullmatch(r"(?:\d+|\d{1,3}(?:,\d{3})+)(?:\.\d{1,2})?", text):
        raise ValueError(f"invalid money amount {value!r}; grouping and at most two minor-unit digits are required")
    numeric = text.replace(",", "")
    if sign == "-" or negative_parentheses:
        numeric = "-" + numeric
    try:
        amount = Decimal(numeric)
    except InvalidOperation as exc:
        raise ValueError(f"invalid money amount {value!r}") from exc
    if abs(amount) > Decimal("1000000000000000000"):
        raise ValueError("transaction amount exceeds the supported 18-digit major-unit bound")
    return amount, currency


def decimal_money_text(amount: Decimal) -> str:
    return format(amount.quantize(Decimal("0.01")), "f")


def validate_pk_iban(value: str | None) -> tuple[str | None, bool | None]:
    raw = clean_text(value)
    if not raw:
        return None, None
    compact = re.sub(r"\s", "", raw).upper()
    if not compact.startswith("PK"):
        return compact, None
    if not PK_IBAN_RE.fullmatch(compact):
        return compact, False
    rearranged = compact[4:] + compact[:4]
    numeric = "".join(str(ord(char) - 55) if char.isalpha() else char for char in rearranged)
    return compact, int(numeric) % 97 == 1


def parse_confidence(value: str | None) -> Decimal | None:
    text = clean_text(value)
    if not text:
        return None
    percent = text.endswith("%")
    if percent:
        text = text[:-1].strip()
    try:
        confidence = Decimal(text)
    except InvalidOperation as exc:
        raise ValueError(f"invalid ANPR confidence {value!r}") from exc
    if percent:
        confidence /= 100
    if confidence < 0 or confidence > 1:
        raise ValueError(f"ANPR confidence {value!r} is outside 0..1")
    return confidence


def plate_script(value: str) -> str:
    has_arabic = False
    has_latin = False
    for char in value:
        name = unicodedata.name(char, "")
        has_arabic = has_arabic or "ARABIC" in name
        has_latin = has_latin or "LATIN" in name
    if has_arabic and has_latin:
        return "mixed_arabic_latin"
    if has_arabic:
        return "arabic_derived"
    if has_latin:
        return "latin"
    if any(char.isdigit() for char in value):
        return "digits_only"
    return "other"


def plate_search_key(value: str) -> str:
    normalized = unicodedata.normalize("NFKC", value).upper()
    return "".join(char for char in normalized if char.isalnum())


def strict_optional_coordinate(value: str | None, label: str, lower: float, upper: float) -> float | None:
    text = clean_text(value)
    if not text:
        return None
    try:
        parsed = float(text)
    except ValueError as exc:
        raise ValueError(f"invalid {label} {text!r}") from exc
    if not math.isfinite(parsed) or parsed < lower or parsed > upper:
        raise ValueError(f"{label} {text!r} is outside {lower}..{upper}")
    return parsed


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
          normalized_record jsonb,
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
    xlsx_mappings: dict[int, dict[str, Any]] | None = None,
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
        "rejection_codes": {},
        "record_types": {},
        "missing_time_rows": 0,
    }
    rejects: list[tuple[int, str, str, dict[str, str]]] = []
    copy_sql = sql.SQL("COPY tmp_generic_records ({}) FROM STDIN").format(
        sql.SQL(", ").join(sql.Identifier(col) for col in GENERIC_COPY_COLUMNS)
    )
    with cur.copy(copy_sql) as copy:
        for line_number, raw in enumerate(iter_records(path, job.source_file), start=2):
            stats["total_rows"] += 1
            row_adapter, sheet_mapping = xlsx_row_adapter(raw, xlsx_mappings or {}, adapter)
            source_row_number = parse_int_optional(raw.get("_xlsx_row_number")) or line_number
            try:
                normalized = compatibility_adapter_registry().normalize(
                    "generic",
                    job,
                    raw,
                    batch_id,
                    source_row_number,
                    row_adapter,
                    profile,
                    sheet_mapping,
                )
                attach_record_maturity(
                    job, raw, normalized, profile, row_adapter, source_row_number, sheet_mapping
                )
                copy.write_row([normalized[col] for col in GENERIC_COPY_COLUMNS])
                update_generic_stats(stats, normalized)
                stats["record_types"][row_adapter.name] = stats["record_types"].get(row_adapter.name, 0) + 1
                ROWS_TOTAL.labels(row_adapter.name, "accepted").inc()
                INGEST_ROWS_TOTAL.labels(row_adapter.name, "accepted").inc()
            except Exception as exc:  # noqa: BLE001 - row-level rejection
                stats["rejected_rows"] += 1
                ROWS_TOTAL.labels(row_adapter.name, "rejected").inc()
                INGEST_ROWS_TOTAL.labels(row_adapter.name, "rejected").inc()
                code = getattr(exc, "code", f"{row_adapter.name}_row_parse_error")
                stats["rejection_codes"][code] = stats["rejection_codes"].get(code, 0) + 1
                rejects.append((source_row_number, code, str(exc), raw))
            if stats["total_rows"] % 1000 == 0:
                emit_progress(progress_cb, job, "running", stats, profile or {}, started_at)
    for line_number, code, message, raw in rejects:
        write_reject(cur, job, line_number, code, message, raw)
    return stats


def normalize_anpr_details(raw: dict[str, Any], adapter: RecordAdapter, observed_at: datetime | None) -> dict[str, Any]:
    return normalize_anpr_record(
        raw,
        observed_at,
        ANPRAdapterRuntime(
            normalize_header=normalize_header,
            first_value=first_value,
            parse_confidence=parse_confidence,
            plate_script=plate_script,
            plate_search_key=plate_search_key,
            valid_sha256=lambda value: HEX_SHA256_RE.fullmatch(value) is not None,
        ),
    )


def transaction_amount_field(raw: dict[str, Any]) -> tuple[str, str]:
    present = [(field, value_from_field(raw, field)) for field in MONEY_FIELDS]
    present = [(field, value) for field, value in present if clean_text(value)]
    if not present:
        raise ValueError("transaction amount is required")
    if len(present) > 1:
        raise ValueError("transaction row supplies multiple non-empty amount fields")
    return present[0][0], clean_text(present[0][1])


def normalize_transaction_details(raw: dict[str, Any], adapter: RecordAdapter, observed_at: datetime | None) -> dict[str, Any]:
    source_field, amount_text = transaction_amount_field(raw)
    currency_value = first_value(raw, ("currency", "currency_code", "transaction_currency", "ccy"))
    amount, currency = parse_money_amount(amount_text, currency_value)
    primary_account = first_value(raw, adapter.primary_fields)
    if not primary_account:
        raise ValueError("transaction source account is required")
    secondary_account = first_value(raw, adapter.secondary_fields)
    primary_iban, primary_iban_valid = validate_pk_iban(primary_account)
    secondary_iban, secondary_iban_valid = validate_pk_iban(secondary_account)
    transaction_id = first_value(raw, ("transaction_id", "txn_id", "transaction_reference", "tran_ref", "reference", "rrn", "stan"))
    status = first_value(raw, ("status", "transaction_status", "txn_status", "state"))
    reversal_reference = first_value(raw, ("reversal_of_transaction_id", "reversal_of", "original_transaction_id", "chargeback_of"))
    normalized_status = normalize_header(status or "")
    flags: list[str] = []
    if observed_at is None:
        flags.append("timestamp_missing")
    if not transaction_id:
        flags.append("transaction_reference_missing")
    if primary_iban_valid is False:
        flags.append("primary_pk_iban_invalid")
    if secondary_iban_valid is False:
        flags.append("secondary_pk_iban_invalid")
    amount_role = "unspecified"
    if source_field == "debit_amount":
        amount_role = "debit"
    elif source_field == "credit_amount":
        amount_role = "credit"
    return {
        "amount_decimal": decimal_money_text(amount),
        "amount_minor_units": int(amount * 100),
        "amount_source_field": source_field,
        "amount_role": amount_role,
        "currency": currency,
        "currency_assumed": not bool(clean_text(currency_value)) and not bool(re.match(r"(?i)^\(?\s*(PKR|RS\.?|₨)", amount_text)),
        "transaction_reference": transaction_id,
        "transaction_status": status,
        "is_reversal_or_chargeback": bool(reversal_reference) or normalized_status in {"reversal", "reversed", "chargeback", "charged_back"},
        "reversal_of_transaction_id": reversal_reference,
        "primary_account_kind": "pk_iban" if primary_iban and primary_iban.startswith("PK") else "provider_account",
        "primary_account_compact": primary_iban,
        "primary_pk_iban_valid": primary_iban_valid,
        "secondary_account_kind": "pk_iban" if secondary_iban and secondary_iban.startswith("PK") else "provider_account",
        "secondary_account_compact": secondary_iban,
        "secondary_pk_iban_valid": secondary_iban_valid,
        "manual_review_required": bool(flags),
        "quality_flags": flags,
    }


def normalize_pk_phone(value: str | None) -> tuple[str | None, str | None]:
    raw = clean_text(value)
    if not raw:
        return None, None
    compact = re.sub(r"[\s().-]", "", raw)
    if re.fullmatch(r"\+923\d{9}", compact):
        return compact, "pk_e164"
    if re.fullmatch(r"00923\d{9}", compact):
        return "+" + compact[2:], "pk_e164"
    if re.fullmatch(r"923\d{9}", compact):
        return "+" + compact, "pk_e164"
    if re.fullmatch(r"03\d{9}", compact):
        return "+92" + compact[1:], "pk_e164"
    if re.fullmatch(r"(?:\*\d+(?:#)?|\d{3,6})", compact):
        return compact, "service_or_short_code"
    return compact, "unrecognized"


def luhn_valid(value: str) -> bool:
    if not value.isdigit():
        return False
    total = 0
    parity = len(value) % 2
    for index, char in enumerate(value):
        digit = int(char)
        if index % 2 == parity:
            digit *= 2
            if digit > 9:
                digit -= 9
        total += digit
    return total % 10 == 0


def normalize_subscriber_details(
    raw: dict[str, Any],
    adapter: RecordAdapter,
    observed_at: datetime | None,
    source_timezone: str = "",
    date_order: str = "",
) -> dict[str, Any]:
    """Compatibility hook delegating to the versioned subscriber adapter."""
    _ = adapter
    return normalize_subscriber_record(
        raw,
        observed_at,
        source_timezone,
        SubscriberAdapterRuntime(
            normalize_header=normalize_header,
            first_value=first_value,
            clean_text=clean_text,
            normalize_phone=normalize_pk_phone,
            luhn_valid=luhn_valid,
            parse_timestamp=lambda value, timezone_name: parse_ts_any(value, timezone_name, date_order),
        ),
    )


def parse_bounded_decimal(value: str | None, label: str, lower: Decimal, upper: Decimal, upper_inclusive: bool = True) -> Decimal | None:
    text = clean_text(value)
    if not text:
        return None
    try:
        parsed = Decimal(text)
    except InvalidOperation as exc:
        raise ValueError(f"invalid {label} {text!r}") from exc
    outside = parsed < lower or parsed > upper if upper_inclusive else parsed < lower or parsed >= upper
    if outside:
        comparator = f"{lower}..{upper}" if upper_inclusive else f"{lower}..< {upper}"
        raise ValueError(f"{label} {text!r} is outside {comparator}")
    return parsed


def normalize_tower_details(
    raw: dict[str, Any],
    adapter: RecordAdapter,
    latitude: float | None,
    longitude: float | None,
    observed_at: datetime | None,
    source_timezone: str,
    date_order: str = "",
) -> dict[str, Any]:
    _ = adapter
    return normalize_tower_record(
        raw,
        latitude,
        longitude,
        observed_at,
        source_timezone,
        TowerAdapterRuntime(
            normalize_header=normalize_header,
            first_value=first_value,
            clean_text=clean_text,
            parse_timestamp=lambda value, timezone_name: parse_ts_any(value, timezone_name, date_order),
        ),
    )


def normalize_access_log_details(raw: dict[str, Any], observed_at: datetime | None) -> dict[str, Any]:
    if observed_at is None:
        raise ValueError("access/security log timestamp is required")
    source_ip = first_value(raw, ("ip", "source_ip", "src_ip", "client_ip", "remote_addr", "remote_ip", "source_address"))
    if not source_ip:
        raise ValueError("access/security log source IP is required")
    try:
        parsed_ip = ipaddress.ip_address(source_ip)
    except ValueError as exc:
        raise ValueError(f"invalid source IP {source_ip!r}") from exc
    status_text = first_value(raw, ("status", "http_status", "status_code", "response_code"))
    status_code = None
    if status_text:
        if not status_text.isdigit():
            raise ValueError(f"invalid HTTP status {status_text!r}")
        status_code = int(status_text)
        if status_code < 100 or status_code > 599:
            raise ValueError(f"HTTP status {status_text!r} is outside 100..599")
    port_text = first_value(raw, ("source_port", "src_port", "client_port"))
    source_port = None
    if port_text:
        if not port_text.isdigit() or not 1 <= int(port_text) <= 65535:
            raise ValueError(f"invalid source port {port_text!r}")
        source_port = int(port_text)
    user = first_value(raw, ("user", "username", "user_id", "principal", "actor"))
    flags: list[str] = []
    if not user:
        flags.append("user_or_principal_missing")
    return {
        "source_ip_raw": source_ip,
        "source_ip_canonical": parsed_ip.compressed,
        "source_ip_version": parsed_ip.version,
        "source_ip_private": parsed_ip.is_private,
        "source_ip_reserved": parsed_ip.is_reserved,
        "source_port": source_port,
        "user_or_principal": user,
        "http_method": (first_value(raw, ("method", "http_method", "request_method")) or "").upper() or None,
        "request_path": first_value(raw, ("path", "url", "uri", "endpoint", "request_uri")),
        "http_status": status_code,
        "event_action": first_value(raw, ("event_action", "action", "operation")),
        "event_outcome": first_value(raw, ("outcome", "result", "event_result")),
        "host": first_value(raw, ("host", "hostname", "device_name", "server")),
        "event_id": first_value(raw, ("event_id", "request_id", "correlation_id", "trace_id")),
        "manual_review_required": bool(flags),
        "quality_flags": flags,
    }


def _normalize_ipdr_details_legacy(
    raw: dict[str, Any],
    observed_at: datetime | None,
    source_timezone: str,
) -> dict[str, Any]:
    if observed_at is None:
        raise ValueError("IPDR/session timestamp is required")

    def parsed_ip(aliases: Iterable[str], label: str, required: bool = False) -> ipaddress.IPv4Address | ipaddress.IPv6Address | None:
        value = first_value(raw, aliases)
        if not value:
            if required:
                raise ValueError(f"{label} IP is required")
            return None
        try:
            return ipaddress.ip_address(value)
        except ValueError as exc:
            raise ValueError(f"invalid {label} IP") from exc

    def parsed_port(aliases: Iterable[str], label: str) -> int | None:
        value = first_value(raw, aliases)
        if not value:
            return None
        if not value.isdigit() or not 1 <= int(value) <= 65535:
            raise ValueError(f"invalid {label} port")
        return int(value)

    source_ip_raw = first_value(raw, ("source_ip", "src_ip", "ip_address")) or ""
    destination_ip_raw = first_value(raw, ("destination_ip", "dst_ip", "dest_ip")) or ""
    source_ip = parsed_ip(("source_ip", "src_ip", "ip_address"), "source", True)
    destination_ip = parsed_ip(("destination_ip", "dst_ip", "dest_ip"), "destination", True)
    nat_source_ip = parsed_ip(("nat_source_ip", "translated_source_ip", "public_ip", "post_nat_source_ip"), "NAT source")
    nat_destination_ip = parsed_ip(("nat_destination_ip", "translated_destination_ip", "post_nat_destination_ip"), "NAT destination")

    bytes_text = first_value(raw, ("bytes", "total_bytes", "byte_count", "octets", "network_volume"))
    byte_count = None
    if bytes_text:
        if not bytes_text.isdigit():
            raise ValueError("IPDR byte count must be a non-negative integer")
        byte_count = int(bytes_text)
        if byte_count > 9223372036854775807:
            raise ValueError("IPDR byte count exceeds the signed 64-bit audit bound")

    end_text = first_value(raw, ("session_end", "end_time", "stop_time", "ended_at"))
    ended_at = parse_ts_any(end_text, source_timezone)
    if end_text and ended_at is None:
        raise ValueError("invalid IPDR/session end timestamp")
    if ended_at is not None and ended_at < observed_at:
        raise ValueError("IPDR/session end timestamp precedes start timestamp")

    domain_raw = first_value(raw, ("domain", "hostname", "host", "fqdn", "dns_name")) or ""
    domain_ascii = ""
    if domain_raw:
        candidate = domain_raw.rstrip(".").lower()
        try:
            domain_ascii = candidate.encode("idna").decode("ascii")
        except UnicodeError as exc:
            raise ValueError("invalid IPDR domain") from exc
        labels = domain_ascii.split(".")
        if len(domain_ascii) > 253 or any(
            not re.fullmatch(r"[a-z0-9_](?:[a-z0-9_-]{0,61}[a-z0-9_])?", label)
            for label in labels
        ):
            raise ValueError("invalid IPDR domain")

    return {
        "source_ip_raw": source_ip_raw,
        "source_ip_canonical": source_ip.compressed,
        "source_ip_version": source_ip.version,
        "source_ip_private": source_ip.is_private,
        "destination_ip_raw": destination_ip_raw,
        "destination_ip_canonical": destination_ip.compressed,
        "destination_ip_version": destination_ip.version,
        "destination_ip_private": destination_ip.is_private,
        "source_port": parsed_port(("source_port", "src_port"), "source"),
        "destination_port": parsed_port(("destination_port", "dst_port", "dest_port"), "destination"),
        "protocol": (first_value(raw, ("protocol", "transport_protocol", "application_protocol")) or "").upper() or None,
        "byte_count": byte_count,
        "session_end_utc": ended_at.isoformat() if ended_at else None,
        "duration_seconds": int((ended_at - observed_at).total_seconds()) if ended_at else None,
        "subscriber_identifier": first_value(raw, ("subscriber_id", "msisdn", "imsi", "user_id")),
        "session_identifier": first_value(raw, ("session_id", "flow_id", "record_id", "correlation_id")),
        "domain_raw": domain_raw or None,
        "domain_ascii": domain_ascii or None,
        "nat_source_ip_canonical": nat_source_ip.compressed if nat_source_ip else None,
        "nat_destination_ip_canonical": nat_destination_ip.compressed if nat_destination_ip else None,
    }


def normalize_ipdr_details(
    raw: dict[str, Any],
    observed_at: datetime | None,
    source_timezone: str,
    date_order: str = "",
) -> dict[str, Any]:
    return normalize_ipdr_record(
        raw,
        observed_at,
        source_timezone,
        IPDRAdapterRuntime(
            normalize_header=normalize_header,
            clean_text=clean_text,
            first_value=first_value,
            parse_timestamp=lambda value, timezone_name: parse_ts_any(value, timezone_name, date_order),
        ),
    )


def normalize_generic_row(
    job: IngestJob,
    raw: dict[str, Any],
    batch_id: str,
    row_number: int,
    adapter: RecordAdapter,
    profile: dict[str, Any] | None = None,
    sheet_mapping: dict[str, Any] | None = None,
) -> dict[str, Any]:
    classification = (sheet_mapping or {}).get("classification") or (profile or {}).get("classification") or {}
    timestamp_field = classification.get("timestamp") or ""
    primary_field = classification.get("primary_target") or ""
    secondary_field = classification.get("secondary_target") or ""
    timestamp_value = value_from_field(raw, timestamp_field) or first_value(raw, adapter.timestamp_fields)
    row_timezone = first_value(raw, ROW_TIMEZONE_FIELDS) or source_timezone_for_job(job)
    date_order = source_date_order_for_job(job)
    observed_at = parse_ts_any(timestamp_value, row_timezone, date_order)
    if clean_text(timestamp_value) and observed_at is None:
        raise ValueError(f"invalid timestamp {timestamp_value!r}")
    latitude = strict_optional_coordinate(first_value(raw, adapter.latitude_fields), "latitude", -90, 90)
    longitude = strict_optional_coordinate(first_value(raw, adapter.longitude_fields), "longitude", -180, 180)
    canonical = {
        "tenant_id": job.tenant_id,
        "collection_id": job.collection_id,
        "file_id": job.file_id,
        "batch_id": batch_id,
        "record_type": adapter.name,
        "row_number": row_number,
        "observed_at": observed_at,
        "primary_entity": value_from_field(raw, primary_field) or first_value(raw, adapter.primary_fields),
        "secondary_entity": value_from_field(raw, secondary_field) or first_value(raw, adapter.secondary_fields),
        "location": first_value(raw, adapter.location_fields),
        "latitude": latitude,
        "longitude": longitude,
        "raw_record": json.dumps(raw, ensure_ascii=False),
        "source_file": job.source_file,
    }
    normalized_record: dict[str, Any] = {
        "source_timezone": row_timezone,
        "timestamp_assumed_timezone": bool(clean_text(timestamp_value)) and not timestamp_has_explicit_offset(clean_text(timestamp_value)),
        "canonical_time_state": "source_derived" if observed_at is not None else "unresolved",
        "persistence_timestamp_role": "source_event_time" if observed_at is not None else "ingest_surrogate_not_source_time",
        "schema_classification": classification,
        "source_fields": {
            "timestamp": timestamp_field,
            "primary_target": primary_field,
            "secondary_target": secondary_field,
        },
    }
    if raw.get("_xlsx_sheet_name") is not None:
        normalized_record["xlsx_locator"] = {
            "sheet_name": raw.get("_xlsx_sheet_name"),
            "sheet_index": raw.get("_xlsx_sheet_index"),
            "sheet_state": raw.get("_xlsx_sheet_state"),
            "header_row": raw.get("_xlsx_header_row"),
            "row_number": raw.get("_xlsx_row_number"),
            "hidden_row": bool(raw.get("_xlsx_hidden_row")),
        }
        if sheet_mapping is not None:
            normalized_record["xlsx_mapping"] = {
                "policy": "per_sheet_schema_v1",
                "requested_record_type": sheet_mapping.get("requested_record_type"),
                "record_type_mode": sheet_mapping.get("record_type_mode"),
                "detected_record_type": sheet_mapping.get("detected_record_type"),
                "decision": sheet_mapping.get("decision"),
                "matching_adapters": sheet_mapping.get("matching_adapters") or [],
                "review_required": bool(sheet_mapping.get("review_required")),
                "header_fingerprint": sheet_mapping.get("header_fingerprint"),
            }
    if adapter.name == "ipdr":
        normalized_record.update(normalize_ipdr_details(raw, observed_at, row_timezone, date_order))
    elif adapter.name == "anpr":
        normalized_record.update(normalize_anpr_details(raw, adapter, observed_at))
    elif adapter.name == "transaction":
        normalized_record.update(normalize_transaction_details(raw, adapter, observed_at))
    elif adapter.name == "subscriber":
        subscriber_details = normalize_subscriber_details(raw, adapter, observed_at, row_timezone, date_order)
        normalized_record.update(subscriber_details)
        canonical["primary_entity"] = subscriber_details.get("msisdn_canonical") or canonical["primary_entity"]
        # Raw evidence remains in protected storage, but generic entity/audit
        # fields must never index a full CNIC or subscriber name by default.
        canonical["secondary_entity"] = (
            subscriber_details.get("subscriber_reference")
            or subscriber_details.get("cnic_masked")
        )
    elif adapter.name == "tower_location":
        normalized_record.update(normalize_tower_details(raw, adapter, latitude, longitude, observed_at, row_timezone, date_order))
    elif adapter.name == "access_log":
        normalized_record.update(normalize_access_log_details(raw, observed_at))
    canonical["normalized_record"] = json.dumps(normalized_record, ensure_ascii=False)
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
            ) || coalesce(normalized_record, '{}'::jsonb)),
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
            )) || coalesce(normalized_record, '{}'::jsonb),
            'source_fields', coalesce(
              normalized_record->'source_fields',
              jsonb_build_object(
                'timestamp', %s::text,
                'primary_target', %s::text,
                'secondary_target', %s::text
              )
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
    else:
        stats["missing_time_rows"] = int(stats.get("missing_time_rows") or 0) + 1
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
        "rejection_codes": {},
        "missing_time_rows": 0,
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
        "record_types": stats.get("record_types", {}),
        "sample_rows_profiled": profile["sample_rows"],
        "detected_schema": profile.get("classification", {}),
        "source_format": profile.get("format") or "delimited_or_structured",
        "parser_warnings": profile.get("warnings", []),
        "structured_quality": profile.get("structured_quality", {}),
    }
    if profile.get("workbook"):
        quality["workbook"] = profile["workbook"]
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
                    "source_format": profile.get("format") or "delimited_or_structured",
                    "workbook": profile.get("workbook"),
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
        "structured_store": "cdr_records" if adapter.name == "cdr" and profile.get("format") != "xlsx" else "generic_records",
        "canonical_store": "records",
        "kb_collection": job.collection_id,
        "routing_reason": "per_sheet_typed_xlsx" if profile.get("format") == "xlsx" else ("typed_adapter" if adapter.name != "generic" else "generic_structured_adapter"),
        "schema_classification": profile.get("classification", {}),
        "structured_maturity": {
            "schema_profile": (profile.get("classification") or {}).get("family_mapping") or {},
            "time_policy": (profile.get("classification") or {}).get("time_policy") or {},
            "quality": profile.get("structured_quality") or {},
        },
        "source_format": profile.get("format") or "delimited_or_structured",
        "workbook": profile.get("workbook"),
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
            "record_types": stats.get("record_types", {}),
        },
        "schema_classification": profile.get("classification", {}),
        "structured_maturity": {
            "schema_profile": (profile.get("classification") or {}).get("family_mapping") or {},
            "time_policy": (profile.get("classification") or {}).get("time_policy") or {},
            "quality": profile.get("structured_quality") or {},
        },
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


def mark_job_completed(
    cur: psycopg.Cursor[Any],
    job: IngestJob,
    batch_id: str,
    stats: dict[str, Any],
    inserted: int,
    lease_token: str = "",
) -> None:
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
            worker_lease_token=NULL,
            worker_lease_owner=NULL,
            worker_lease_expires_at=NULL,
            next_attempt_at=NULL,
            last_error_class=NULL,
            error_message=NULL,
            metadata = metadata || %s::jsonb
        WHERE id=%s::uuid
          AND (NULLIF(%s, '') IS NULL OR worker_lease_token=NULLIF(%s, '')::uuid)
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
            lease_token,
            lease_token,
        ),
    )
    if cur.rowcount != 1:
        raise RuntimeError("worker lease was lost before completion")


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


def source_content_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def safe_audit_rejection_code(exc: Exception) -> str:
    if isinstance(exc, StructuredTimeError):
        return exc.code
    message = str(exc).lower()
    if "timestamp" in message or "timezone" in message:
        return "invalid_timestamp_or_timezone"
    if "latitude" in message or "longitude" in message or "coordinate" in message:
        return "invalid_coordinate"
    if "amount" in message or "currency" in message or "decimal" in message:
        return "invalid_money_value"
    if "ip" in message or "port" in message or "http status" in message:
        return "invalid_network_value"
    if "required" in message or "missing" in message:
        return "missing_required_value"
    return f"{type(exc).__name__.lower()}_row_error"


def audit_structured_source(
    path: Path,
    *,
    source_name: str | None = None,
    record_type: str = "auto",
    source_timezone: str = "",
    source_timezone_state: str = "unknown",
    source_date_order: str = "",
    source_date_order_state: str = "unresolved",
    jurisdiction: str = "",
) -> dict[str, Any]:
    """Profile and normalize a supported source without exposing row values.

    This is the offline provider-onboarding and demonstration boundary. It uses
    the production profiler/adapters but returns only schema and aggregate quality
    facts; source identifiers and raw/rejected rows never enter the report.
    """

    if not path.is_file():
        raise ValueError(f"structured source does not exist: {path}")
    display_name = source_name or path.name
    profile = profile_records(path, display_name)
    validate_structured_profile_bounds(profile)
    job = IngestJob(
        job_id="00000000-0000-0000-0000-000000000001",
        tenant_id="offline-audit",
        user_id="",
        collection_id="offline-audit",
        file_id="offline-audit",
        source_file=display_name,
        spool_path=str(path),
        sha256=source_content_sha256(path),
        record_type=record_type,
        headers=list(profile.get("headers") or []),
        metadata={
            "source_timezone": source_timezone,
            "source_timezone_state": source_timezone_state,
            "source_date_order": source_date_order,
            "source_date_order_state": source_date_order_state,
            "jurisdiction": jurisdiction,
            "record_type_mode": "auto" if normalize_header(record_type) in {"", "auto"} else "explicit",
        },
    )
    mappings: dict[int, dict[str, Any]] = {}
    if source_extension(path, display_name) == ".xlsx":
        mappings, adapter = prepare_xlsx_sheet_mappings(job, profile)
    else:
        adapter = resolve_adapter(job, profile.get("headers") or [])
        attach_family_schema_profile(profile, adapter, job)

    total_rows = 0
    accepted_rows = 0
    rejected_rows = 0
    overflow_rows = 0
    exact_duplicate_rows = 0
    row_hashes: set[str] = set()
    record_types: dict[str, int] = {}
    rejection_codes: dict[str, int] = {}
    min_timestamp: datetime | None = None
    max_timestamp: datetime | None = None

    for source_row, raw in enumerate(iter_records(path, display_name), start=2):
        total_rows += 1
        if any(str(key).startswith("_extra_column_") for key in raw):
            overflow_rows += 1
        row_adapter = adapter
        sheet_mapping: dict[str, Any] | None = None
        if mappings:
            row_adapter, sheet_mapping = xlsx_row_adapter(raw, mappings, adapter)
            row_number = parse_int_optional(raw.get("_xlsx_row_number")) or source_row
        else:
            row_number = source_row
        try:
            if row_adapter.name == "cdr":
                normalized = compatibility_adapter_registry().normalize("cdr", job, raw, job.job_id, row_number)
                observed_at = normalized.get("call_start_ts")
            else:
                normalized = compatibility_adapter_registry().normalize(
                    "generic",
                    job,
                    raw,
                    job.job_id,
                    row_number,
                    row_adapter,
                    profile,
                    sheet_mapping,
                )
                observed_at = normalized.get("observed_at")
            row_digest = str(normalized["row_hash"])
            if row_digest in row_hashes:
                exact_duplicate_rows += 1
            else:
                row_hashes.add(row_digest)
            accepted_rows += 1
            record_types[row_adapter.name] = record_types.get(row_adapter.name, 0) + 1
            if isinstance(observed_at, datetime):
                min_timestamp = observed_at if min_timestamp is None else min(min_timestamp, observed_at)
                max_timestamp = observed_at if max_timestamp is None else max(max_timestamp, observed_at)
        except Exception as exc:  # noqa: BLE001 - audit must count all row failures
            rejected_rows += 1
            code = safe_audit_rejection_code(exc)
            rejection_codes[code] = rejection_codes.get(code, 0) + 1

    dialect = profile.get("tabular_dialect") or {}
    workbook = profile.get("workbook") or {}
    mapping = (profile.get("classification") or {}).get("family_mapping") or {}
    audit_quality = build_quality_summary(
        mapping,
        {"total_rows": total_rows, "rejected_rows": rejected_rows, "rejection_codes": rejection_codes},
        accepted_rows - exact_duplicate_rows,
    )
    return {
        "audit_version": "structured_source_audit_v1",
        "source": {
            "name": display_name,
            "extension": source_extension(path, display_name),
            "size_bytes": path.stat().st_size,
            "sha256": job.sha256,
        },
        "routing": {
            "requested_record_type": record_type,
            "detected_record_type": adapter.name,
            "source_timezone": source_timezone,
            "source_timezone_state": source_timezone_state,
            "source_date_order": source_date_order or None,
            "source_date_order_state": source_date_order_state,
            "jurisdiction": jurisdiction or None,
        },
        "schema": {
            "column_count": len(profile.get("headers") or []),
            "headers": list(profile.get("headers") or []),
            "encoding": dialect.get("encoding"),
            "delimiter": dialect.get("delimiter"),
            "duplicate_or_blank_headers": bool(dialect.get("duplicate_or_blank_headers")),
            "sample_rows_profiled": int(profile.get("sample_rows") or 0),
            "family_mapping": (profile.get("classification") or {}).get("family_mapping"),
        },
        "accounting": {
            "total_rows": total_rows,
            "accepted_rows": accepted_rows,
            "rejected_rows": rejected_rows,
            "exact_duplicate_rows": exact_duplicate_rows,
            "unique_normalized_rows": len(row_hashes),
            "overflow_rows": overflow_rows,
            "row_accounting_complete": total_rows == accepted_rows + rejected_rows,
            "record_types": dict(sorted(record_types.items())),
            "rejection_codes": dict(sorted(rejection_codes.items())),
        },
        "quality": audit_quality,
        "time_range": {
            "min_utc": min_timestamp.isoformat() if min_timestamp else None,
            "max_utc": max_timestamp.isoformat() if max_timestamp else None,
        },
        "workbook": {
            "policy": workbook.get("mapping_policy"),
            "sheet_count": len(workbook.get("sheets") or []),
            "mapped_record_types": workbook.get("mapped_record_types") or [],
            "review_required": bool(workbook.get("review_required")),
        } if workbook else None,
        "privacy": {
            "raw_rows_emitted": False,
            "identifiers_emitted": False,
            "rejected_values_emitted": False,
        },
    }


def run_source_audit_cli(arguments: list[str]) -> None:
    parser = argparse.ArgumentParser(description="Privacy-safe offline audit for a supported structured source")
    parser.add_argument("path", type=Path)
    parser.add_argument("--record-type", default="auto", choices=tuple(["auto", *ADAPTERS.keys()]))
    parser.add_argument("--source-timezone", default="")
    parser.add_argument("--source-timezone-state", default="unknown", choices=("source_declared", "analyst_confirmed", "profile_defaulted", "unknown"))
    parser.add_argument("--source-date-order", default="", choices=("", "DMY", "MDY", "YMD"))
    parser.add_argument("--source-date-order-state", default="unresolved", choices=("source_declared", "analyst_confirmed", "profile_defaulted", "unresolved"))
    parser.add_argument("--jurisdiction", default="")
    args = parser.parse_args(arguments)
    result = audit_structured_source(
        args.path,
        record_type=args.record_type,
        source_timezone=args.source_timezone,
        source_timezone_state=args.source_timezone_state,
        source_date_order=args.source_date_order,
        source_date_order_state=args.source_date_order_state,
        jurisdiction=args.jurisdiction,
    )
    print(json.dumps(result, indent=2, sort_keys=True, ensure_ascii=False))


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "validate-sample":
        print(json.dumps(validate_sample_profile(Path(sys.argv[2])), indent=2, sort_keys=True))
    elif len(sys.argv) >= 3 and sys.argv[1] == "audit-source":
        run_source_audit_cli(sys.argv[2:])
    else:
        main()
