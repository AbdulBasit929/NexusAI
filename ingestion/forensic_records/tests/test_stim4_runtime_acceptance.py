import json
import unittest
from collections import Counter
from pathlib import Path

from ingestion.forensic_records.worker import (
    ADAPTERS,
    IngestJob,
    audit_structured_source,
    iter_records,
    normalize_cdr_row,
    normalize_generic_row,
)


FIXTURES = Path(__file__).parent / "fixtures" / "stim4_runtime_acceptance"
ORACLE = json.loads(
    (
        Path(__file__).resolve().parents[3]
        / "api"
        / "forensic_records"
        / "contracts"
        / "stim4-runtime-acceptance-oracle-v1.json"
    ).read_text(encoding="utf-8")
)


class STIM4RuntimeAcceptancePackTests(unittest.TestCase):
    def job(self, fixture: Path, record_type: str) -> IngestJob:
        return IngestJob(
            job_id="00000000-0000-0000-0000-000000000094",
            tenant_id="stim-runtime-test",
            user_id="stim-runtime-test",
            collection_id="stim-runtime-acceptance",
            file_id=fixture.stem,
            source_file=fixture.name,
            spool_path=str(fixture),
            sha256="4" * 64,
            record_type=record_type,
            headers=[],
            metadata={"source_timezone": "Asia/Karachi", "jurisdiction": "PK"},
        )

    def normalized_generic(self, fixture_name: str, record_type: str) -> list[dict]:
        fixture = FIXTURES / fixture_name
        job = self.job(fixture, record_type)
        return [
            normalize_generic_row(job, raw, job.job_id, row_number, ADAPTERS[record_type])
            for row_number, raw in enumerate(iter_records(fixture, fixture.name), start=2)
        ]

    def test_every_fixture_is_auto_routable_with_complete_row_accounting(self):
        expected = {
            "cdr_alpha.csv": ("cdr", 4),
            "cdr_bravo.csv": ("cdr", 2),
            "cdr_charlie.tsv": ("cdr", 2),
            "ipdr.csv": ("ipdr", 3),
            "subscriber.jsonl": ("subscriber", 4),
            "anpr_east.csv": ("anpr", 3),
            "anpr_west.csv": ("anpr", 3),
        }
        for fixture_name, (record_type, accepted_rows) in expected.items():
            audit = audit_structured_source(
                FIXTURES / fixture_name,
                record_type="auto",
                source_timezone="Asia/Karachi",
                jurisdiction="PK",
            )
            with self.subTest(fixture=fixture_name):
                self.assertEqual(audit["routing"]["detected_record_type"], record_type)
                self.assertEqual(audit["accounting"]["accepted_rows"], accepted_rows)
                self.assertEqual(audit["accounting"]["rejected_rows"], 0)
                self.assertTrue(audit["accounting"]["row_accounting_complete"])

    def test_three_cdr_sources_match_the_hand_authored_intersection_oracle(self):
        expectation = ORACLE["expectations"]["multi_cdr"]
        target = expectation["target_canonical"]
        contacts_by_source = {}
        all_imei = set()
        all_imsi = set()
        event_count = 0
        duplicate_count = 0
        for fixture_name in ORACLE["fixtures"]["multi_cdr"]:
            fixture = FIXTURES / fixture_name
            job = self.job(fixture, "cdr")
            contacts = set()
            seen_hashes = set()
            for row_number, raw in enumerate(iter_records(fixture, fixture.name), start=2):
                row = normalize_cdr_row(job, raw, job.job_id, row_number)
                if row["row_hash"] in seen_hashes:
                    duplicate_count += 1
                    continue
                seen_hashes.add(row["row_hash"])
                event_count += 1
                caller = row["originating_number_canonical"]
                callee = row["called_number_canonical"]
                self.assertIn(target, (caller, callee))
                contacts.add((callee if caller == target else caller).lstrip("+"))
                all_imei.add(row["imei"])
                all_imsi.add(row["imsi"])
            contacts_by_source[fixture_name] = contacts

        occurrence = Counter(contact for contacts in contacts_by_source.values() for contact in contacts)
        common = sorted(contact for contact, count in occurrence.items() if count >= 2)
        unique = {
            source: sorted(contact for contact in contacts if occurrence[contact] == 1)
            for source, contacts in contacts_by_source.items()
        }
        self.assertEqual(event_count, expectation["accepted_event_count"])
        self.assertEqual(duplicate_count, expectation["duplicate_source_row_count"])
        self.assertEqual(common, expectation["common_contacts"])
        self.assertEqual(unique, expectation["source_unique_contacts"])
        self.assertEqual(sorted(all_imei), expectation["shared_imei"])
        self.assertEqual(sorted(all_imsi), expectation["shared_imsi"])

    def test_ipdr_subscriber_and_cross_source_anpr_keys_match_the_oracle(self):
        expectation = ORACLE["expectations"]["ipdr_subscriber"]
        ipdr_rows = [json.loads(row["normalized_record"]) for row in self.normalized_generic("ipdr.csv", "ipdr")]
        subscriber_rows = [json.loads(row["normalized_record"]) for row in self.normalized_generic("subscriber.jsonl", "subscriber")]
        target_digits = expectation["target_canonical"].lstrip("+")
        ipdr = ipdr_rows[0]
        subscriber = subscriber_rows[0]
        self.assertEqual(ipdr["subscriber_identifier"], target_digits)
        self.assertEqual(subscriber["msisdn_canonical"].lstrip("+"), target_digits)
        self.assertEqual(len(ipdr_rows), expectation["ipdr_source_rows"])
        self.assertEqual(len(subscriber_rows), expectation["subscriber_source_rows"])
        self.assertGreater(subscriber_rows[1]["valid_from_at"], "2026-08-18T06:10:00+00:00")
        self.assertEqual(subscriber_rows[2]["subscriber_reference"], expectation["wrong_entity_type_value"])
        self.assertEqual(ipdr_rows[0]["source_ip_canonical"], expectation["wrong_entity_type_value"])
        self.assertFalse(expectation["ownership_inference"])

        sightings = []
        unique_by_source = {}
        for fixture_name in ORACLE["fixtures"]["cross_source_anpr"]:
            rows = self.normalized_generic(fixture_name, "anpr")
            source_sightings = []
            for row in rows:
                normalized = json.loads(row["normalized_record"])
                source_sightings.append((normalized["plate_search_key"], row["secondary_entity"], row["observed_at"]))
            sightings.extend(source_sightings)
            unique_by_source[fixture_name] = sorted(item[0] for item in source_sightings if item[0] != "STIM404")
        anpr_expectation = ORACLE["expectations"]["cross_source_anpr"]
        repeated = [row for row in sightings if row[0] == anpr_expectation["plate_search_key"]]
        self.assertEqual(len(sightings), anpr_expectation["accepted_sighting_count"])
        self.assertEqual([row[1] for row in repeated], anpr_expectation["ordered_camera_ids"])
        self.assertEqual(int((repeated[1][2] - repeated[0][2]).total_seconds()), anpr_expectation["elapsed_seconds"])
        self.assertEqual(unique_by_source, anpr_expectation["source_unique_plate_keys"])
        self.assertEqual(sorted(anpr_expectation["plate_like_non_match"]), sorted(["LOOK777", "LOOK778"]))
        self.assertFalse(anpr_expectation["ownership_inference"])


if __name__ == "__main__":
    unittest.main()
