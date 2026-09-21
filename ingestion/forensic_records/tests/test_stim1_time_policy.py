import json
import unittest
from pathlib import Path

from ingestion.forensic_records.worker import (
    AmbiguousDateOrderError,
    IngestJob,
    UnknownSourceTimezoneError,
    parse_ts_any,
    source_timezone_for_job,
    time_policy_for_job,
    timestamp_provenance,
)


FIXTURES = Path(__file__).parent / "fixtures"
GOLDEN = json.loads((FIXTURES / "stim_time_policy_goldens_v1.json").read_text(encoding="utf-8"))


def job(metadata=None):
    return IngestJob(
        job_id="00000000-0000-0000-0000-000000000201",
        tenant_id="stim-test", user_id="stim-test", collection_id="stim-time",
        file_id="time-source", source_file="time.csv", spool_path="time.csv",
        sha256="2" * 64, record_type="cdr", headers=[], metadata=metadata or {},
    )


class STIM1TimePolicyTests(unittest.TestCase):
    def test_independent_time_goldens(self):
        self.assertEqual(GOLDEN["contract_version"], "forensics.time-policy-golden/v1")
        for case in GOLDEN["cases"]:
            with self.subTest(case=case["id"]):
                parsed = parse_ts_any(case["input"], case["timezone"], case["date_order"])
                self.assertEqual(parsed.isoformat(), case["expected_utc"])

    def test_ambiguous_slash_date_is_not_guessed(self):
        with self.assertRaises(AmbiguousDateOrderError):
            parse_ts_any("03/04/2026 00:30:00", "Asia/Karachi")

    def test_unknown_timezone_is_not_utc(self):
        with self.assertRaises(UnknownSourceTimezoneError):
            parse_ts_any("2026-02-13 00:30:00")

    def test_explicit_offset_needs_no_assumed_timezone(self):
        self.assertEqual(
            parse_ts_any("2026-02-13T00:30:00+05:00").isoformat(),
            "2026-02-12T19:30:00+00:00",
        )

    def test_pakistan_jurisdiction_is_not_timezone_proof(self):
        policy_job = job({"jurisdiction": "PK"})
        self.assertEqual(source_timezone_for_job(policy_job), "")
        self.assertEqual(time_policy_for_job(policy_job)["timezone_state"], "unknown")
        self.assertEqual(time_policy_for_job(policy_job)["date_order_state"], "unresolved")

    def test_profile_default_is_used_only_when_explicitly_allowed(self):
        denied = job({"profile_default_timezone": "Asia/Karachi"})
        allowed = job({
            "profile_default_timezone": "Asia/Karachi",
            "allow_profile_timezone_default": "true",
            "source_timezone_state": "profile_defaulted",
        })
        self.assertEqual(source_timezone_for_job(denied), "")
        self.assertEqual(source_timezone_for_job(allowed), "Asia/Karachi")
        self.assertEqual(time_policy_for_job(allowed)["timezone_state"], "profile_defaulted")

    def test_provenance_records_declared_policy_and_canonical_instant(self):
        policy_job = job({
            "source_timezone": "Asia/Karachi", "source_timezone_state": "source_declared",
            "source_date_order": "DMY", "source_date_order_state": "source_declared",
        })
        canonical = parse_ts_any("03/04/2026 00:30:00", "Asia/Karachi", "DMY")
        provenance = timestamp_provenance(policy_job, "03/04/2026 00:30:00", canonical, "cdr-source-mapping/v2")
        self.assertEqual(provenance["date_order_source"], "source_declared")
        self.assertEqual(provenance["timezone_state"], "source_declared")
        self.assertEqual(provenance["canonical_instant"], "2026-04-02T19:30:00+00:00")


if __name__ == "__main__":
    unittest.main()
