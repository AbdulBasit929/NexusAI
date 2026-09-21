import csv
import json
import unittest
from pathlib import Path

from ingestion.forensic_records.worker import (
    ADAPTERS,
    IngestJob,
    audit_structured_source,
    normalize_generic_row,
)


FIXTURES = Path(__file__).parent / "fixtures"
IPDR_FIXTURE = FIXTURES / "pakistan_ipdr_messy_synthetic.csv"
PHASE4_GOLDEN = FIXTURES / "pakistan_phase4_structured_goldens_v1.json"


class Phase4StructuredDepthTests(unittest.TestCase):
    def job(self) -> IngestJob:
        return IngestJob(
            job_id="00000000-0000-0000-0000-000000000001",
            tenant_id="default",
            user_id="",
            collection_id="phase4-structured",
            file_id="ipdr-fixture",
            source_file=IPDR_FIXTURE.name,
            spool_path=str(IPDR_FIXTURE),
            sha256="0" * 64,
            record_type="ipdr",
            headers=[],
            metadata={"source_timezone": "Asia/Karachi", "jurisdiction": "PK"},
        )

    def test_ipdr_fixture_has_exact_normalization_and_visible_rejects(self):
        golden = json.loads(PHASE4_GOLDEN.read_text(encoding="utf-8"))["datasets"]["ipdr"]
        with IPDR_FIXTURE.open(newline="", encoding="utf-8-sig") as source:
            rows = list(csv.DictReader(source))
        accepted = []
        rejected = []
        for row_number, raw in enumerate(rows, start=2):
            try:
                accepted.append(
                    normalize_generic_row(
                        self.job(),
                        raw,
                        self.job().job_id,
                        row_number,
                        ADAPTERS["ipdr"],
                    )
                )
            except ValueError as exc:
                rejected.append(str(exc))

        self.assertEqual(len(rows), golden["total_rows"])
        self.assertEqual(len(accepted), golden["accepted_rows"])
        self.assertEqual(len(rejected), golden["rejected_rows"])
        first = json.loads(accepted[0]["normalized_record"])
        ipv6 = json.loads(accepted[1]["normalized_record"])
        explicit_offset = json.loads(accepted[2]["normalized_record"])
        self.assertEqual(accepted[0]["observed_at"].isoformat(), golden["exact_checks"]["first_timestamp_utc"])
        self.assertEqual(first["duration_seconds"], golden["exact_checks"]["first_duration_seconds"])
        self.assertEqual(first["nat_source_ip_canonical"], golden["exact_checks"]["nat_source_canonical"])
        self.assertEqual(ipv6["source_ip_canonical"], golden["exact_checks"]["ipv6_source_canonical"])
        self.assertEqual(explicit_offset["source_ip_version"], 4)
        self.assertEqual(
            accepted[2]["observed_at"].isoformat(),
            golden["exact_checks"]["explicit_offset_timestamp_utc"],
        )
        self.assertIn("invalid source IP", rejected[0])
        self.assertIn("non-negative integer", rejected[1])

    def test_all_typed_structured_families_have_complete_privacy_safe_accounting(self):
        matrix = json.loads(PHASE4_GOLDEN.read_text(encoding="utf-8"))["acceptance_matrix"]
        self.assertEqual(
            {item["record_type"] for item in matrix},
            {"cdr", "ipdr", "anpr", "transaction", "subscriber", "tower_location", "access_log"},
        )
        for expected in matrix:
            with self.subTest(record_type=expected["record_type"]):
                audit = audit_structured_source(
                    FIXTURES / expected["path"],
                    record_type=expected["record_type"],
                    source_timezone="Asia/Karachi",
                    jurisdiction="PK",
                )
                self.assertEqual(audit["routing"]["detected_record_type"], expected["record_type"])
                self.assertEqual(audit["accounting"]["total_rows"], expected["total_rows"])
                self.assertEqual(audit["accounting"]["accepted_rows"], expected["accepted_rows"])
                self.assertEqual(audit["accounting"]["rejected_rows"], expected["rejected_rows"])
                self.assertEqual(audit["accounting"]["exact_duplicate_rows"], expected["duplicate_rows"])
                self.assertEqual(audit["accounting"]["unique_normalized_rows"], expected["unique_normalized_rows"])
                self.assertTrue(audit["accounting"]["row_accounting_complete"])
                self.assertFalse(audit["privacy"]["raw_rows_emitted"])

    def test_ipdr_privacy_safe_audit_matches_versioned_golden(self):
        golden = json.loads(PHASE4_GOLDEN.read_text(encoding="utf-8"))["datasets"]["ipdr"]
        audit = audit_structured_source(
            IPDR_FIXTURE,
            record_type="auto",
            source_timezone="Asia/Karachi",
            jurisdiction="PK",
        )

        self.assertEqual(audit["routing"]["detected_record_type"], "ipdr")
        self.assertEqual(audit["accounting"]["total_rows"], golden["total_rows"])
        self.assertEqual(audit["accounting"]["accepted_rows"], golden["accepted_rows"])
        self.assertEqual(audit["accounting"]["rejected_rows"], golden["rejected_rows"])
        self.assertEqual(audit["accounting"]["exact_duplicate_rows"], golden["duplicate_rows"])
        self.assertEqual(audit["accounting"]["unique_normalized_rows"], golden["unique_normalized_rows"])
        self.assertEqual(audit["accounting"]["rejection_codes"], golden["exact_checks"]["rejection_codes"])
        self.assertTrue(audit["accounting"]["row_accounting_complete"])
        self.assertFalse(audit["privacy"]["identifiers_emitted"])

    def test_ipdr_rejects_unsafe_domain_and_unbounded_byte_count(self):
        base = {
            "session_start": "2026-07-20 08:00:00",
            "source_ip": "10.0.0.5",
            "destination_ip": "8.8.8.8",
            "bytes": "1",
        }
        with self.assertRaisesRegex(ValueError, "invalid IPDR domain"):
            normalize_generic_row(
                self.job(),
                {**base, "domain": "invalid domain.example"},
                self.job().job_id,
                2,
                ADAPTERS["ipdr"],
            )
        with self.assertRaisesRegex(ValueError, "signed 64-bit"):
            normalize_generic_row(
                self.job(),
                {**base, "bytes": "9223372036854775808"},
                self.job().job_id,
                3,
                ADAPTERS["ipdr"],
            )


if __name__ == "__main__":
    unittest.main()
