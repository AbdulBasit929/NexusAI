import asyncio
import csv
import hashlib
import inspect
import json
import os
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

from ingestion.forensic_records.worker import (
    ADAPTERS,
    FailureDisposition,
    IngestJob,
    INGEST_CONSUMER,
    INGEST_QUEUE_GROUP,
    JobClaim,
    admitted_media_metadata,
    audit_structured_source,
    classify_job_error,
    classify_columns,
    handle_ingest_message,
    insert_stage_into_dynamic_from_cdr,
    normalize_generic_row,
    iter_records,
    mark_media_job_completed,
    persist_anpr_media_observation,
    profile_records,
    retry_backoff_seconds,
    should_process_native_document,
    storage_scope_key,
    validate_job_storage,
)


class DynamicWorkerTests(unittest.TestCase):
    def test_native_document_route_remains_authoritative_for_legacy_text_modality(self):
        self.assertTrue(should_process_native_document("native_document_worker"))
        self.assertFalse(should_process_native_document("forensic_records_worker"))

    def test_anpr_media_observation_types_derived_identifier_parameter(self):
        class Cursor:
            query = ""
            parameters = ()

            def execute(self, query, parameters):
                self.query = query
                self.parameters = parameters

        cursor = Cursor()
        observation = SimpleNamespace(
            payload={
                "raw_plate_text": "MN1367",
                "normalized_plate_text": "MN1367",
                "ocr_confidence": 0.99,
                "detection_confidence": 0.88,
            },
            citation_locator="image://bbox/1,2,3,4",
            observation_id="observation-anpr-1",
        )

        persist_anpr_media_observation(cursor, self.job(), observation, 1)

        self.assertIn("'derived_observation_id', %s::text", cursor.query)
        self.assertEqual(cursor.parameters[-1], "observation-anpr-1")

    def test_media_completion_types_json_metadata_parameters(self):
        class Cursor:
            query = ""
            parameters = ()

            def execute(self, query, parameters):
                self.query = query
                self.parameters = parameters

        cursor = Cursor()
        result = SimpleNamespace(
            observations=(object(),),
            readiness="READY_WITH_WARNINGS",
            processor_id="bounded-media",
            processor_revision="v1",
            limitations=("manual review",),
            metadata={"result_states": {"asr": "COMPLETE_RESULTS"}},
        )
        mark_media_job_completed(cursor, self.job(), result)

        self.assertIn("'media_readiness', %s::text", cursor.query)
        self.assertIn("'media_processor', %s::text", cursor.query)
        self.assertIn("'media_processor_revision', %s::text", cursor.query)
        self.assertIn("'media_result_states', %s::jsonb", cursor.query)
        self.assertEqual(cursor.parameters[2:5], ("READY_WITH_WARNINGS", "bounded-media", "v1"))
        self.assertEqual(json.loads(cursor.parameters[6]), {"asr": "COMPLETE_RESULTS"})

    def test_media_job_admits_only_explicit_asr_language_from_durable_job_metadata(self):
        admitted = admitted_media_metadata(
            {"duration_seconds": 6.24},
            {"asr_language": " UR ", "storage_uri": "forensic-spool://internal"},
        )
        self.assertEqual(admitted, {"duration_seconds": 6.24, "asr_language": "ur"})
        self.assertEqual(admitted_media_metadata({}, {}), {})

    def test_standalone_worker_image_packages_structured_maturity(self):
        dockerfile = (Path(__file__).resolve().parents[1] / "Dockerfile").read_text(encoding="utf-8")
        self.assertIn(
            "COPY ingestion/forensic_records/structured_maturity.py ./structured_maturity.py",
            dockerfile,
        )

    def test_jetstream_queue_and_durable_identity_are_compatible(self):
        self.assertEqual(INGEST_QUEUE_GROUP, INGEST_CONSUMER)

    def test_cdr_canonical_insert_uses_versioned_normalized_stage_projection(self):
        source = inspect.getsource(insert_stage_into_dynamic_from_cdr)
        self.assertIn("coalesce(normalized_record", source)
        self.assertIn("'legacy_store', 'forensic.cdr_records'", source)
        self.assertIn("'msisdn', msisdn", source)

    def job(self):
        return IngestJob(
            job_id="00000000-0000-0000-0000-000000000001",
            tenant_id="default",
            user_id="",
            collection_id="case-dynamic",
            file_id="file-1",
            source_file="dynamic.csv",
            spool_path="dynamic.csv",
            sha256="0" * 64,
            record_type="generic",
            headers=[],
            metadata={},
            request_id="req-1",
        )

    def test_classifies_arbitrary_identifier_columns_from_values(self):
        rows = [
            {
                "subscriberRef": "+92 346 167 8183",
                "counterpartyValue": "analyst@example.org",
                "eventWhen": "2026-07-10T12:00:00Z",
                "vehicleSeen": "ABC-123",
            },
            {
                "subscriberRef": "923001112222",
                "counterpartyValue": "case.officer@example.org",
                "eventWhen": "2026-07-10T12:05:00Z",
                "vehicleSeen": "LHR-447",
            },
        ]
        detected = classify_columns(rows[0].keys(), rows)
        self.assertEqual(detected["columns"]["subscriberRef"]["role"], "phone")
        self.assertEqual(detected["columns"]["counterpartyValue"]["role"], "email")
        self.assertEqual(detected["columns"]["eventWhen"]["role"], "timestamp")
        self.assertEqual(detected["primary_target"], "subscriberRef")
        self.assertEqual(detected["secondary_target"], "counterpartyValue")

    def test_normalizes_generic_row_using_detected_schema(self):
        profile = {
            "classification": {
                "timestamp": "eventWhen",
                "primary_target": "subscriberRef",
                "secondary_target": "counterpartyValue",
            }
        }
        raw = {
            "subscriberRef": "923461678183",
            "counterpartyValue": "10.20.30.40",
            "eventWhen": "2026-07-10T12:00:00Z",
            "note": "arbitrary payload",
        }
        row = normalize_generic_row(self.job(), raw, self.job().job_id, 2, ADAPTERS["generic"], profile)
        self.assertEqual(row["primary_entity"], "923461678183")
        self.assertEqual(row["secondary_entity"], "10.20.30.40")
        self.assertIsNotNone(row["observed_at"])
        self.assertIn("arbitrary payload", row["raw_record"])

    def test_ingest_job_parses_evidence_id_from_queue_payload(self):
        payload = {
            "job_id": "00000000-0000-0000-0000-000000000001",
            "evidence_id": "11111111-1111-1111-1111-111111111111",
            "tenant_id": "default",
            "collection_id": "case-dynamic",
            "file_id": "file-1",
            "source_file": "dynamic.csv",
            "spool_path": "dynamic.csv",
            "sha256": "0" * 64,
            "record_type": "generic",
            "headers": ["eventWhen", "subscriberRef"],
            "metadata": {"case_id": "case-42"},
            "request_id": "req-1",
        }
        job = IngestJob.from_message(json.dumps(payload).encode("utf-8"))
        self.assertEqual(job.evidence_id, "11111111-1111-1111-1111-111111111111")
        self.assertEqual(job.metadata["case_id"], "case-42")

    def test_profiles_dynamic_csv_with_non_demo_headers(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "dynamic.csv"
            with path.open("w", newline="", encoding="utf-8") as fh:
                writer = csv.DictWriter(fh, fieldnames=["eventWhen", "subscriberRef", "counterpartyValue"])
                writer.writeheader()
                for idx in range(1000):
                    writer.writerow(
                        {
                            "eventWhen": f"2026-07-10T12:{idx % 60:02d}:00Z",
                            "subscriberRef": f"92300111{idx:04d}",
                            "counterpartyValue": "analyst@example.org",
                        }
                    )
            profile = profile_records(path)

        self.assertEqual(profile["sample_rows"], 1000)
        self.assertEqual(profile["classification"]["columns"]["eventWhen"]["role"], "timestamp")
        self.assertEqual(profile["classification"]["columns"]["subscriberRef"]["role"], "phone")

    def test_privacy_safe_source_audit_uses_production_cdr_adapter(self):
        fixture = Path(__file__).parent / "fixtures" / "pakistan_cdr_messy_synthetic.csv"
        audit = audit_structured_source(
            fixture,
            record_type="auto",
            source_timezone="Asia/Karachi",
            jurisdiction="PK",
        )

        self.assertEqual(audit["routing"]["detected_record_type"], "cdr")
        self.assertEqual(audit["schema"]["column_count"], 16)
        self.assertEqual(audit["accounting"]["total_rows"], 5)
        self.assertEqual(audit["accounting"]["accepted_rows"], 5)
        self.assertEqual(audit["accounting"]["rejected_rows"], 0)
        self.assertEqual(audit["accounting"]["exact_duplicate_rows"], 1)
        self.assertEqual(audit["accounting"]["unique_normalized_rows"], 4)
        self.assertTrue(audit["accounting"]["row_accounting_complete"])
        self.assertEqual(audit["time_range"]["min_utc"], "2026-07-10T03:00:00+00:00")
        self.assertEqual(
            audit["privacy"],
            {
                "raw_rows_emitted": False,
                "identifiers_emitted": False,
                "rejected_values_emitted": False,
            },
        )

    def test_runtime_worker_verifies_schema_without_owner_ddl(self):
        worker_source = (Path(__file__).parents[1] / "worker.py").read_text(encoding="utf-8")
        self.assertIn("to_regclass('forensic.records')", worker_source)
        self.assertIn("tenant_isolation_records", worker_source)
        self.assertIn("tenant_isolation_records_ingest_jobs", worker_source)
        self.assertIn("migration 010 before starting the worker", worker_source)
        self.assertNotIn("CREATE TABLE IF NOT EXISTS forensic.records", worker_source)
        self.assertNotIn("create_hypertable(", worker_source)

    def test_profiles_and_streams_utf16_tsv_without_coercing_forensic_values(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "messy-pakistan.tsv"
            path.write_text(
                "event_time\taccount\tamount\tname\tname\n"
                "2026-07-29 09:15:00 PKT\t00012345\t001250.5000\t\u0639\u0644\u06cc\tAli\n"
                "2026-07-29 10:30:00 PKT\tPK36SCBL0000001123456702\t-000010.00\t\u0641\u0627\u0637\u0645\u06c1\tFatima\textra\n",
                encoding="utf-16",
            )

            profile = profile_records(path, "vendor-export.tsv")
            rows = list(iter_records(path, "vendor-export.tsv"))

            self.assertEqual(profile["tabular_dialect"]["delimiter"], "\\t")
            self.assertEqual(profile["tabular_dialect"]["encoding"], "utf-16")
            self.assertTrue(profile["tabular_dialect"]["duplicate_or_blank_headers"])
            self.assertEqual(profile["headers"][-1], "name__duplicate_2")
            self.assertEqual(rows[0]["account"], "00012345")
            self.assertEqual(rows[0]["amount"], "001250.5000")
            self.assertEqual(rows[0]["name"], "\u0639\u0644\u06cc")
            self.assertEqual(rows[0]["name__duplicate_2"], "Ali")
            self.assertEqual(rows[1]["_extra_column_1"], "extra")

    def test_profiles_extensionless_content_address_by_original_source_name(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / ("a" * 64)
            path.write_text('{"timestamp":"2026-07-29T08:00:00+05:00","client_ip":"192.0.2.1"}\n', encoding="utf-8")
            profile = profile_records(path, "security-events.jsonl")
        self.assertEqual(profile["headers"], ["timestamp", "client_ip"])
        self.assertEqual(profile["sample_rows"], 1)

    def test_validates_content_address_scope_receipt_size_and_hash(self):
        self.assertEqual(
            storage_scope_key("phase3-smoke", "phase3-control-plane-smoke"),
            "2dbfa9b88e4316f8cac0ee0094b8d25e65651bcf2ecc1fc385edfb255d07bb42",
        )
        payload = b"timestamp,source,target\n2026-07-29T08:00:00+05:00,923001111111,923002222222\n"
        content_hash = hashlib.sha256(payload).hexdigest()
        scope = storage_scope_key("tenant-a", "case-a")
        uri = f"forensic-spool://sha256-scope-v1/{scope}/{content_hash}"
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            object_path = root / "objects" / scope / content_hash[:2] / content_hash
            object_path.parent.mkdir(parents=True)
            object_path.write_bytes(payload)
            object_path.chmod(0o440)
            receipt_path = root / "receipts" / scope / content_hash[:2] / f"{content_hash}.json"
            receipt_path.parent.mkdir(parents=True)
            receipt_path.write_text(
                json.dumps(
                    {
                        "layout_version": "sha256-scope-v1",
                        "scope_key": scope,
                        "tenant_id": "tenant-a",
                        "collection_id": "case-a",
                        "sha256": content_hash,
                        "size_bytes": len(payload),
                        "storage_uri": uri,
                        "retained_at": "2026-07-29T03:00:00Z",
                    }
                ),
                encoding="utf-8",
            )
            receipt_path.chmod(0o440)
            job = IngestJob(
                job_id="00000000-0000-0000-0000-000000000002",
                tenant_id="tenant-a",
                user_id="user-a",
                collection_id="case-a",
                file_id="file-a",
                source_file="calls.csv",
                spool_path=str(object_path),
                sha256=content_hash,
                record_type="cdr",
                headers=[],
                metadata={
                    "size_bytes": str(len(payload)),
                    "storage_layout_version": "sha256-scope-v1",
                    "storage_uri": uri,
                    "storage_receipt_uri": f"forensic-spool-receipt://sha256-scope-v1/{scope}/{content_hash}",
                    "storage_scope_key": scope,
                    "storage_integrity_state": "verified",
                    "storage_write_once": "true",
                },
            )
            self.assertEqual(validate_job_storage(job, root), object_path)

            os.chmod(object_path, 0o600)
            with self.assertRaisesRegex(ValueError, "writable"):
                validate_job_storage(job, root)

    def test_rejects_legacy_spool_path_outside_configured_root(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "spool"
            root.mkdir()
            outside = Path(tmp) / "outside.csv"
            outside.write_text("a,b\n1,2\n", encoding="utf-8")
            job = IngestJob(
                job_id="00000000-0000-0000-0000-000000000003",
                tenant_id="default",
                user_id="",
                collection_id="case-a",
                file_id="file-a",
                source_file="outside.csv",
                spool_path=str(outside),
                sha256=hashlib.sha256(outside.read_bytes()).hexdigest(),
                record_type="generic",
                headers=[],
                metadata={},
            )
            with self.assertRaisesRegex(ValueError, "escapes"):
                validate_job_storage(job, root)

    def test_queue_retry_policy_is_bounded_and_error_classification_fails_closed(self):
        self.assertEqual([retry_backoff_seconds(i) for i in (1, 2, 3, 20)], [2.0, 4.0, 8.0, 300.0])
        self.assertEqual(classify_job_error(ValueError("bad evidence")), "permanent_input")
        self.assertEqual(classify_job_error(ConnectionError("database unavailable")), "transient_io")
        self.assertEqual(classify_job_error(RuntimeError("adapter unavailable")), "transient_processing")


class FakeJetStreamMessage:
    def __init__(self, payload: bytes):
        self.data = payload
        self.headers = {"X-Request-ID": "request-from-header"}
        self.acked = 0
        self.termed = 0
        self.naks = []
        self.progress = 0

    async def ack_sync(self, timeout=0):
        self.acked += 1

    async def term(self):
        self.termed += 1

    async def nak(self, delay=0):
        self.naks.append(delay)

    async def in_progress(self):
        self.progress += 1


class FakeJetStream:
    def __init__(self):
        self.published = []

    async def publish(self, subject, payload, headers=None):
        self.published.append((subject, json.loads(payload), headers or {}))


class FakeNATS:
    async def publish(self, *_args, **_kwargs):
        return None


class QueueLifecycleTests(unittest.IsolatedAsyncioTestCase):
    def payload(self):
        return json.dumps(
            {
                "job_id": "00000000-0000-0000-0000-000000000009",
                "evidence_id": "11111111-1111-1111-1111-111111111111",
                "tenant_id": "tenant-a",
                "user_id": "user-a",
                "collection_id": "case-a",
                "file_id": "file-a",
                "source_file": "calls.csv",
                "spool_path": "retained-object",
                "sha256": "9" * 64,
                "record_type": "cdr",
                "headers": [],
                "metadata": {},
            }
        ).encode()

    async def test_completed_redelivery_only_acknowledges_and_does_not_reprocess(self):
        message = FakeJetStreamMessage(self.payload())
        js = FakeJetStream()
        with (
            patch("ingestion.forensic_records.worker.claim_job", return_value=JobClaim("completed", attempt_count=1, max_attempts=5)),
            patch("ingestion.forensic_records.worker.process_job") as process,
            patch("ingestion.forensic_records.worker.mark_job_acknowledged"),
        ):
            await handle_ingest_message(message, js, FakeNATS(), asyncio.get_running_loop())
        self.assertEqual(message.acked, 1)
        self.assertEqual(message.termed, 0)
        process.assert_not_called()

    async def test_transient_failure_is_nacked_with_database_disposition_delay(self):
        message = FakeJetStreamMessage(self.payload())
        js = FakeJetStream()
        with (
            patch("ingestion.forensic_records.worker.claim_job", return_value=JobClaim("claimed", "lease-a", 1, 5)),
            patch("ingestion.forensic_records.worker.process_job", side_effect=ConnectionError("temporary")),
            patch(
                "ingestion.forensic_records.worker.mark_job_failed",
                return_value=FailureDisposition("retry", 1, 5, "transient_io", 2.0),
            ),
        ):
            await handle_ingest_message(message, js, FakeNATS(), asyncio.get_running_loop())
        self.assertEqual(message.naks, [2.0])
        self.assertEqual(message.acked, 0)
        self.assertEqual(message.termed, 0)
        self.assertEqual(js.published, [])

    async def test_permanent_failure_is_published_to_idempotent_dlq_before_term(self):
        message = FakeJetStreamMessage(self.payload())
        js = FakeJetStream()
        with (
            patch("ingestion.forensic_records.worker.claim_job", return_value=JobClaim("claimed", "lease-a", 1, 5)),
            patch("ingestion.forensic_records.worker.process_job", side_effect=ValueError("hash mismatch")),
            patch(
                "ingestion.forensic_records.worker.mark_job_failed",
                return_value=FailureDisposition("dead_letter", 1, 5, "permanent_input"),
            ),
        ):
            await handle_ingest_message(message, js, FakeNATS(), asyncio.get_running_loop())
        self.assertEqual(message.termed, 1)
        self.assertEqual(message.acked, 0)
        self.assertEqual(len(js.published), 1)
        subject, envelope, headers = js.published[0]
        self.assertEqual(subject, "forensic.records.ingest.dead_letter")
        self.assertEqual(envelope["reason"], "permanent_input")
        self.assertEqual(envelope["attempt_count"], 1)
        self.assertEqual(headers["Nats-Msg-Id"], envelope["event_id"])

    async def test_invalid_message_is_dead_lettered_without_database_claim(self):
        message = FakeJetStreamMessage(b'{}')
        js = FakeJetStream()
        with patch("ingestion.forensic_records.worker.claim_job") as claim:
            await handle_ingest_message(message, js, FakeNATS(), asyncio.get_running_loop())
        self.assertEqual(message.termed, 1)
        self.assertEqual(js.published[0][1]["reason"], "invalid_message")
        claim.assert_not_called()


if __name__ == "__main__":
    unittest.main()
