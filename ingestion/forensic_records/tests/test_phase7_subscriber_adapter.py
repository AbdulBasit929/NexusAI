import hashlib
import json
import unittest
from collections import Counter, defaultdict
from pathlib import Path

from ingestion.forensic_records.adapters.subscriber import (
    adapter_capabilities,
    detect_schema,
    load_manifest,
    verify_normalized_record,
)
from ingestion.forensic_records.worker import ADAPTERS, IngestJob, normalize_generic_row, normalize_header


FIXTURE = Path(__file__).parent / "fixtures" / "pakistan_subscriber_messy_synthetic.jsonl"
POPULATED_FIXTURE = Path(__file__).parent / "fixtures" / "pakistan_subscriber_populated_acceptance_v1.jsonl"
POPULATED_GOLDEN = Path(__file__).resolve().parents[3] / "tests" / "fixtures" / "forensic_modalities" / "pakistan_subscriber_populated_acceptance_v1.json"
ROLE_GOLDEN = Path(__file__).resolve().parents[3] / "tests" / "fixtures" / "forensic_modalities" / "pakistan_cdr_subscriber_role_goldens_v1.json"


class Phase7SubscriberAdapterTests(unittest.TestCase):
    def job(self, fixture=FIXTURE):
        return IngestJob(
            job_id="00000000-0000-0000-0000-000000000071",
            tenant_id="default",
            user_id="phase7-test",
            collection_id="phase7-subscriber-golden",
            file_id="subscriber-golden-1",
            source_file=fixture.name,
            spool_path=str(fixture),
            sha256="7" * 64,
            record_type="subscriber",
            headers=[],
            metadata={"source_timezone": "Asia/Karachi", "jurisdiction": "PK"},
        )

    def normalize(self, raw, row_number=2, fixture=FIXTURE):
        return normalize_generic_row(
            self.job(fixture), raw, "00000000-0000-0000-0000-000000000071", row_number, ADAPTERS["subscriber"]
        )

    def test_manifest_promotes_privacy_safe_bounded_subscriber_slice(self):
        manifest = load_manifest()
        self.assertEqual(manifest["contract_version"], "forensics.adapter-manifest/v1")
        self.assertEqual(manifest["id"], "nexusai.adapter.subscriber_identity")
        self.assertEqual(manifest["version"], "1.3.0")
        self.assertEqual(manifest["status"], "operational")
        self.assertTrue(manifest["preserves_legacy_output"])
        self.assertFalse(manifest["resource_profile"]["model_required"])
        self.assertEqual(manifest["privacy"]["default_redaction"], ["cnic", "subscriber_name"])
        self.assertFalse(manifest["privacy"]["ownership_inference"])
        self.assertIn("subscriber.reuse_candidates", manifest["operation_ids"])
        self.assertEqual(adapter_capabilities()["normalization_version"], "subscriber-identity/v2")

    def test_detection_requires_msisdn_and_explicit_identity_shape(self):
        accepted = detect_schema(["mobile_no", "subscriber_id", "activation_date"], normalize_header)
        self.assertTrue(accepted["matched"])
        self.assertFalse(accepted["review_required"])
        partial = detect_schema(["mobile_no", "activation_date"], normalize_header)
        self.assertFalse(partial["matched"])
        self.assertEqual(partial["required_groups_matched"], 1)

    def test_normalization_preserves_legacy_fields_and_adds_validity_and_default_redaction(self):
        role_golden = json.loads(ROLE_GOLDEN.read_text(encoding="utf-8"))["subscriber"]
        normalized = self.normalize({
            **role_golden["source_row"],
            "mobile_no": "0300 1234567",
            "full_name": "عائشہ تجرباتی",
            "cnic_no": "35202-1234567-1",
            "subscriber_id": "PK-SUB-SYN-001",
            "imsi": "410010123456789",
            "device_imei": "490154203237518",
            "sim_serial": "8992410000000000001",
            "service_id": "PK-SVC-SYN-001",
            "subscription_type": "prepaid",
            "operator": "SyntheticTel",
            "rate_plan": "Synthetic Gold",
            "activation_date": "2026-07-17 08:00:00",
            "deactivation_date": "2026-08-01 10:30:00",
            "status": "active",
        })
        details = normalized["normalized_record"]
        details = json.loads(details)
        self.assertEqual({key: details[key] for key in role_golden["expected"]}, role_golden["expected"])
        self.assertEqual(details["msisdn_canonical"], "+923001234567")
        self.assertEqual(details["subscriber_reference"], "PK-SUB-SYN-001")
        self.assertEqual(details["subscriber_status"], "ACTIVE")
        self.assertEqual(details["cnic_masked"], "*********5671")
        self.assertEqual(details["subscriber_name_script"], "arabic")
        self.assertEqual(details["valid_from_at"], "2026-07-17T03:00:00+00:00")
        self.assertEqual(details["valid_to_at"], "2026-08-01T05:30:00+00:00")
        self.assertEqual(details["iccid_raw"], "8992410000000000001")
        self.assertTrue(details["iccid_format_valid"])
        self.assertEqual(details["service_identifier"], "PK-SVC-SYN-001")
        self.assertEqual(details["service_type"], "PREPAID")
        self.assertEqual(details["provider"], "SyntheticTel")
        self.assertEqual(details["service_plan"], "Synthetic Gold")
        self.assertEqual(details["validity_status"], "closed_window")
        self.assertEqual(details["association_basis"], "explicit_source_row_co_observation")
        self.assertIn("sim_iccid", details["association_roles"])
        self.assertIn("device_imei", details["association_roles"])
        self.assertIn("service_reference", details["association_roles"])
        self.assertNotIn("validity_window_inverted", details["quality_flags"])
        self.assertEqual(normalized["primary_entity"], "+923001234567")
        self.assertEqual(normalized["secondary_entity"], "PK-SUB-SYN-001")
        self.assertNotIn("35202-1234567-1", normalized["secondary_entity"])
        self.assertNotIn("Ø¹Ø§Ø¦Ø´Û", normalized["secondary_entity"])
        verification = verify_normalized_record(details)
        self.assertTrue(verification["valid"])
        self.assertTrue(verification["cnic_default_redaction_ready"])
        self.assertTrue(verification["has_sim_identity"])
        self.assertTrue(verification["has_service_identity"])
        self.assertTrue(verification["association_is_observation_only"])

    def test_inverted_validity_is_preserved_and_flagged_for_human_review(self):
        normalized = self.normalize({
            "mobile_no": "03001234567",
            "subscriber_id": "PK-SUB-SYN-REVIEW",
            "activation_date": "2026-08-01 00:00:00",
            "deactivation_date": "2026-07-01 00:00:00",
        })
        details = json.loads(normalized["normalized_record"])
        self.assertTrue(details["manual_review_required"])
        self.assertIn("deactivation_before_activation", details["quality_flags"])
        self.assertIn("validity_window_inverted", details["quality_flags"])

    def test_generic_entity_projection_masks_cnic_and_omits_names(self):
        cnic_only = self.normalize({
            "mobile_no": "03001234567",
            "cnic_no": "00000-1234567-1",
            "activation_date": "2026-07-17 08:00:00",
        })
        self.assertEqual(cnic_only["secondary_entity"], "*********5671")

        name_only = self.normalize({
            "mobile_no": "03001234567",
            "full_name": "Ø¹Ø§Ø¦Ø´Û ØªØ¬Ø±Ø¨Ø§ØªÛŒ",
            "activation_date": "2026-07-17 08:00:00",
        })
        self.assertIsNone(name_only["secondary_entity"])

    def test_populated_fixture_reconciles_all_six_operations_and_default_privacy(self):
        golden = json.loads(POPULATED_GOLDEN.read_text(encoding="utf-8"))
        source_bytes = POPULATED_FIXTURE.read_bytes()
        self.assertEqual(hashlib.sha256(source_bytes).hexdigest(), golden["source_sha256"])
        raw_rows = [json.loads(line) for line in source_bytes.decode("utf-8-sig").splitlines() if line.strip()]

        candidates = []
        rejected = []
        for row_number, raw in enumerate(raw_rows, start=2):
            try:
                candidates.append(self.normalize(raw, row_number, POPULATED_FIXTURE))
            except ValueError as exc:
                rejected.append(str(exc))

        unique = []
        seen_hashes = set()
        for row in candidates:
            if row["row_hash"] in seen_hashes:
                continue
            seen_hashes.add(row["row_hash"])
            row["details"] = json.loads(row["normalized_record"])
            unique.append(row)

        accounting = golden["accounting"]
        self.assertEqual(len(raw_rows), accounting["total_rows"])
        self.assertEqual(len(candidates), accounting["normalized_candidates"])
        self.assertEqual(len(candidates) - len(unique), accounting["duplicate_rows"])
        self.assertEqual(len(unique), accounting["unique_accepted_rows"])
        self.assertEqual(len(rejected), accounting["rejected_rows"])
        for fragment in golden["expected_rejection_fragments"]:
            self.assertTrue(any(fragment in message for message in rejected), (fragment, rejected))

        def public_row(row):
            details = row["details"]
            return {
                "msisdn": details.get("msisdn_canonical"),
                "subscriber_reference": details.get("subscriber_reference"),
                "imsi": details.get("imsi_raw"),
                "imei": details.get("imei_raw"),
                "subscriber_status": details.get("subscriber_status"),
                "activation_at": details.get("activation_at"),
                "deactivation_at": details.get("deactivation_at"),
                "valid_from_at": details.get("valid_from_at"),
                "valid_to_at": details.get("valid_to_at"),
                "cnic_masked": details.get("cnic_masked"),
                "subscriber_name_present": bool(details.get("subscriber_name_search_key")),
                "subscriber_name_script": details.get("subscriber_name_script"),
                "manual_review_required": details.get("manual_review_required"),
                "quality_flags": details.get("quality_flags"),
                "source_file": row["source_file"],
                "row_number": row["row_number"],
                "row_hash": row["row_hash"],
                "batch_id": row["batch_id"],
                "file_id": row["file_id"],
                "tenant_id": row["tenant_id"],
                "collection_id": row["collection_id"],
            }

        public_rows = [public_row(row) for row in unique]
        public_blob = json.dumps(public_rows, ensure_ascii=False)
        for raw in raw_rows:
            for key in ("cnic", "cnic_no", "national_id", "nic", "name", "full_name", "subscriber_name", "customer_name"):
                value = str(raw.get(key) or "").strip()
                if value:
                    self.assertNotIn(value, public_blob)
        self.assertNotIn("cnic_digits", public_blob)
        self.assertNotIn("subscriber_name_search_key", public_blob)
        self.assertTrue(all(row["cnic_masked"].startswith("*********") for row in public_rows))
        self.assertTrue(all(row["source_file"] == POPULATED_FIXTURE.name and row["row_hash"] for row in public_rows))

        alpha = [row for row in public_rows if row["subscriber_reference"] == "PK-SUB-SYN-ALPHA"]
        identity = golden["operations"]["subscriber.identity_lookup"]
        self.assertEqual(len(alpha), identity["row_count"])
        self.assertEqual([row["row_number"] for row in alpha], identity["source_rows"])

        timeline = golden["operations"]["subscriber.validity_timeline"]
        self.assertEqual(len(alpha), timeline["row_count"])
        self.assertEqual(sum(row["valid_to_at"] is not None for row in alpha), timeline["closed_windows"])
        self.assertEqual(sum(row["valid_to_at"] is None for row in alpha), timeline["open_windows"])

        devices = golden["operations"]["subscriber.device_links"]
        self.assertEqual(len(alpha), devices["row_count"])
        self.assertEqual(len({row["imsi"] for row in alpha}), devices["distinct_imsi_count"])
        self.assertEqual(len({row["imei"] for row in alpha}), devices["distinct_imei_count"])

        status_counts = Counter((row["subscriber_status"], row["manual_review_required"]) for row in public_rows)
        actual_groups = sorted(
            ({"subscriber_status": status, "manual_review_required": review, "row_count": count}
             for (status, review), count in status_counts.items()),
            key=lambda item: (item["subscriber_status"], item["manual_review_required"]),
        )
        expected_groups = sorted(
            golden["operations"]["subscriber.status_summary"]["groups"],
            key=lambda item: (item["subscriber_status"], item["manual_review_required"]),
        )
        self.assertEqual(actual_groups, expected_groups)

        by_stable = defaultdict(list)
        for row in public_rows:
            by_stable[row["subscriber_reference"] or row["msisdn"]].append(row)
        conflicts = []
        for stable, rows in by_stable.items():
            distinct_counts = [
                len({row[key] for row in rows if row[key] is not None})
                for key in ("msisdn", "imsi", "imei", "subscriber_status", "cnic_masked", "subscriber_name_script")
            ]
            if any(count > 1 for count in distinct_counts):
                conflicts.append(stable)
        conflict_golden = golden["operations"]["subscriber.conflict_audit"]
        self.assertEqual(sorted(conflicts), sorted(conflict_golden["stable_identifiers"]))
        self.assertEqual(len(conflicts), conflict_golden["row_count"])

        candidates_by_key = defaultdict(list)
        for row in public_rows:
            for kind, key in (("subscriber_reference", "subscriber_reference"), ("imsi", "imsi"), ("imei", "imei")):
                if row[key]:
                    candidates_by_key[(kind, row[key])].append(row)
        reuse = sorted(
            ({"identifier_kind": kind, "identifier_value": value, "distinct_msisdn_count": len({row["msisdn"] for row in rows})}
             for (kind, value), rows in candidates_by_key.items()
             if len({row["msisdn"] for row in rows}) > 1),
            key=lambda item: (item["identifier_kind"], item["identifier_value"]),
        )
        reuse_expected = sorted(
            golden["operations"]["subscriber.reuse_candidates"]["candidates"],
            key=lambda item: (item["identifier_kind"], item["identifier_value"]),
        )
        self.assertEqual(reuse, reuse_expected)
        self.assertEqual(len(reuse), golden["operations"]["subscriber.reuse_candidates"]["row_count"])


if __name__ == "__main__":
    unittest.main()
