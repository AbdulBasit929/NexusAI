import json
import unittest
from pathlib import Path

from ingestion.forensic_records.adapters.cdr import schema_profile
from ingestion.forensic_records.worker import (
    IngestJob,
    audit_structured_source,
    clean_text,
    iter_records,
    normalize_cdr_row,
    normalize_header,
    profile_records,
)


FIXTURES = Path(__file__).parent / "fixtures"
GOLDEN = json.loads((FIXTURES / "stim_cdr_multi_schema_goldens_v1.json").read_text(encoding="utf-8"))


class STIM1CDRMultiSchemaTests(unittest.TestCase):
    def job(self, fixture: Path) -> IngestJob:
        return IngestJob(
            job_id="00000000-0000-0000-0000-000000000101",
            tenant_id="stim-test",
            user_id="stim-test",
            collection_id="stim-cdr-multi-schema",
            file_id=fixture.stem,
            source_file=fixture.name,
            spool_path=str(fixture),
            sha256="1" * 64,
            record_type="cdr",
            headers=[],
            metadata={"source_timezone": "Asia/Karachi", "jurisdiction": "PK"},
        )

    def analytical_events(self, fixture: Path) -> list[dict[str, object]]:
        events = []
        job = self.job(fixture)
        for row_number, raw in enumerate(iter_records(fixture, fixture.name), start=2):
            record = normalize_cdr_row(job, raw, job.job_id, row_number)
            events.append({
                "originating_number_canonical": record["originating_number_canonical"],
                "called_number_canonical": record["called_number_canonical"],
                "call_start_utc": record["call_start_ts"].isoformat(),
                "call_end_utc": record["call_end_ts"].isoformat(),
                "duration_seconds": record["duration_seconds"],
                "direction_canonical": record["direction_canonical"],
                "service_class": record["service_class"],
            })
        return events

    def test_three_source_layouts_match_one_independent_analytical_golden(self):
        expected = GOLDEN["expected_analytical_events"]
        for source in GOLDEN["schemas"]:
            fixture = FIXTURES / source["file"]
            with self.subTest(fixture=fixture.name):
                self.assertEqual(self.analytical_events(fixture), expected)

    def test_schema_profiles_explain_mapping_and_preserve_extensions(self):
        for source in GOLDEN["schemas"]:
            fixture = FIXTURES / source["file"]
            profile = profile_records(fixture, fixture.name)
            mapping = schema_profile(
                profile["headers"], profile["sample_records"], normalize_header, clean_text
            )
            with self.subTest(fixture=fixture.name):
                self.assertEqual(mapping["contract_version"], "forensics.schema-profile/v1")
                self.assertFalse(mapping["review_required"])
                self.assertEqual(mapping["ambiguous_fields"], [])
                self.assertEqual(mapping["unmapped_fields"], [])
                self.assertEqual(mapping["recognized_extension_fields"], source["unmapped_fields"])
                raw = next(iter(iter_records(fixture, fixture.name)))
                normalized = normalize_cdr_row(self.job(fixture), raw, self.job(fixture).job_id, 2)
                self.assertEqual(
                    json.loads(normalized["raw_record"])[source["unmapped_fields"][0]],
                    raw[source["unmapped_fields"][0]],
                )

    def test_offline_audit_carries_family_mapping_without_source_values(self):
        for source in GOLDEN["schemas"]:
            fixture = FIXTURES / source["file"]
            audit = audit_structured_source(
                fixture,
                record_type="auto",
                source_timezone="Asia/Karachi",
                jurisdiction="PK",
            )
            with self.subTest(fixture=fixture.name):
                self.assertEqual(audit["routing"]["detected_record_type"], "cdr")
                self.assertEqual(audit["accounting"]["accepted_rows"], 2)
                self.assertTrue(audit["accounting"]["row_accounting_complete"])
                mapping = audit["schema"]["family_mapping"]
                self.assertEqual(mapping["mapping_profile_id"], "cdr-source-mapping/v2")
                self.assertEqual(mapping["unmapped_fields"], [])
                self.assertEqual(mapping["recognized_extension_fields"], source["unmapped_fields"])
                first_raw = next(iter(iter_records(fixture, fixture.name)))
                first_normalized = normalize_cdr_row(self.job(fixture), first_raw, self.job(fixture).job_id, 2)
                serialized_mapping = json.dumps(mapping)
                self.assertNotIn(first_normalized["originating_number_raw"], serialized_mapping)
                self.assertNotIn(first_normalized["called_number_raw"], serialized_mapping)


if __name__ == "__main__":
    unittest.main()
