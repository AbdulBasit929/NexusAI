#!/usr/bin/env python3
"""Controlled completed-job redelivery proof for the isolated runtime fixture."""

import asyncio
import json
import os
import sys
import time
import urllib.request

import nats
import psycopg


COLLECTION = "nexusai-runtime-acceptance-20260730"
SUBJECT = "forensic.records.ingest.requested"
STREAM = "FORENSIC_RECORDS_INGEST"
METRIC = 'forensic_records_queue_deliveries_total{disposition="completed_redelivery"}'


def completed_metric() -> float:
    text = urllib.request.urlopen("http://127.0.0.1:9109/metrics", timeout=5).read().decode()
    for line in text.splitlines():
        if line.startswith(METRIC + " "):
            return float(line.rsplit(" ", 1)[1])
    return 0.0


def string_metadata(value: object) -> dict[str, str]:
    if not isinstance(value, dict):
        return {}
    result: dict[str, str] = {}
    for key, item in value.items():
        if item is None:
            continue
        result[str(key)] = item if isinstance(item, str) else json.dumps(item, separators=(",", ":"))
    return result


def load_completed_job(tenant: str) -> tuple[dict[str, object], tuple[int, int]]:
    with psycopg.connect(os.environ["FORENSIC_DATABASE_URL"]) as conn:
        with conn.cursor() as cur:
            cur.execute("SELECT set_config('app.tenant_id', %s, true)", (tenant,))
            cur.execute(
                """
                SELECT id::text, evidence_id::text, tenant_id, user_id, collection_id,
                       file_id, source_file, spool_path, sha256, record_type::text,
                       queued_at, metadata
                FROM forensic.records_ingest_jobs
                WHERE tenant_id=%s AND collection_id=%s AND status='completed'
                ORDER BY reprocess_generation DESC, queued_at DESC
                LIMIT 1
                """,
                (tenant, COLLECTION),
            )
            row = cur.fetchone()
            if row is None:
                raise RuntimeError("completed synthetic job is missing")
            job = {
                "job_id": row[0], "evidence_id": row[1], "tenant_id": row[2],
                "user_id": row[3], "collection_id": row[4], "file_id": row[5],
                "source_file": row[6], "spool_path": row[7], "sha256": row[8],
                "record_type": row[9], "headers": [], "queued_at": row[10].isoformat(),
                "metadata": string_metadata(row[11]),
            }
            cur.execute(
                """
                SELECT
                  (SELECT count(*) FROM forensic.records WHERE tenant_id=%s AND evidence_id=%s::uuid),
                  (SELECT count(*) FROM forensic.records_ingest_jobs WHERE tenant_id=%s AND evidence_id=%s::uuid)
                """,
                (tenant, row[1], tenant, row[1]),
            )
            counts = cur.fetchone()
            if counts is None:
                raise RuntimeError("synthetic count snapshot failed")
            return job, (int(counts[0]), int(counts[1]))


def load_counts(tenant: str, evidence_id: str) -> tuple[int, int]:
    with psycopg.connect(os.environ["FORENSIC_DATABASE_URL"]) as conn:
        with conn.cursor() as cur:
            cur.execute("SELECT set_config('app.tenant_id', %s, true)", (tenant,))
            cur.execute(
                """
                SELECT
                  (SELECT count(*) FROM forensic.records WHERE tenant_id=%s AND evidence_id=%s::uuid),
                  (SELECT count(*) FROM forensic.records_ingest_jobs WHERE tenant_id=%s AND evidence_id=%s::uuid)
                """,
                (tenant, evidence_id, tenant, evidence_id),
            )
            row = cur.fetchone()
            if row is None:
                raise RuntimeError("synthetic count verification failed")
            return int(row[0]), int(row[1])


async def main() -> None:
    tenant = sys.argv[1] if len(sys.argv) > 1 else "default"
    job, before_counts = load_completed_job(tenant)
    before_metric = completed_metric()
    nc = await nats.connect(os.environ["NATS_URL"], name="nexusai-redelivery-acceptance")
    try:
        js = nc.jetstream()
        message_id = f"{job['job_id']}:completed-redelivery:{time.time_ns()}"
        await js.publish(
            SUBJECT,
            json.dumps(job, separators=(",", ":")).encode(),
            headers={"Nats-Msg-Id": message_id},
        )
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            info = await js.stream_info(STREAM)
            after_metric = completed_metric()
            if info.state.messages == 0 and after_metric >= before_metric + 1:
                break
            await asyncio.sleep(0.5)
        else:
            raise RuntimeError("completed redelivery was not synchronously acknowledged")
    finally:
        await nc.drain()
    after_counts = load_counts(tenant, str(job["evidence_id"]))
    if after_counts != before_counts:
        raise RuntimeError("completed redelivery created duplicate database effects")
    print("ActivationGate11=PASS CompletedRedeliveryAck=PASS DuplicateEffects=0 StreamDrained=true RealEvidenceUsed=false")


if __name__ == "__main__":
    asyncio.run(main())
