import json
import unittest
from pathlib import Path

from ingestion.forensic_records.structured_maturity import (
    DYNAMIC_ATTRIBUTE_VERSION,
    QUALITY_CONTRACT_VERSION,
    SCHEMA_REGISTRY_VERSION,
    dynamic_attributes,
    quality_summary,
    registry_snapshot,
    schema_profile,
)
from ingestion.forensic_records.worker import ADAPTERS, IngestJob, audit_structured_source, clean_text, normalize_generic_row, normalize_header, profile_records, validate_structured_profile_bounds


FIXTURES = Path(__file__).parent / "fixtures"
ADAPTER_ROOT = Path(__file__).parent.parent / "adapters"


class STIM2StructuredMaturityTests(unittest.TestCase):
    def test_registry_has_one_versioned_contract_for_all_bounded_families(self):
        snapshot = registry_snapshot()
        self.assertEqual(snapshot["contract_version"], SCHEMA_REGISTRY_VERSION)
        self.assertEqual(set(snapshot["dynamic_attribute_states"]), {"canonical", "recognized_extension", "unmapped", "ambiguous", "invalid"})
        families = {entry["adapter_name"]: entry for entry in snapshot["families"]}
        self.assertTrue({"cdr", "ipdr", "subscriber", "tower_location", "anpr", "generic", "transaction", "access_log"}.issubset(families))
        self.assertEqual(families["cdr"]["mapping_profile_id"], "cdr-source-mapping/v2")
        self.assertEqual(families["generic"]["status"], "baseline")
        self.assertNotIn("inpr", families)
        cdr_fields = {field["name"]: field for field in families["cdr"]["canonical_fields"]}
        self.assertEqual(cdr_fields["msisdn"]["identifier_type"], "msisdn")
        self.assertTrue(cdr_fields["call_start_ts"]["provenance_required"])

    def test_dedicated_adapter_manifests_pin_shared_contract_versions(self):
        for adapter_name in ("cdr", "ipdr", "subscriber", "tower_location", "anpr"):
            manifest = json.loads((ADAPTER_ROOT / adapter_name / "manifest.json").read_text(encoding="utf-8"))
            with self.subTest(adapter=adapter_name):
                self.assertEqual(manifest["schema_registry_version"], SCHEMA_REGISTRY_VERSION)
                self.assertEqual(manifest["dynamic_attribute_contract"], DYNAMIC_ATTRIBUTE_VERSION)
                self.assertEqual(manifest["quality_contract"], QUALITY_CONTRACT_VERSION)
                self.assertEqual(manifest["time_policy_version"], "forensics.time-policy/v1")

    def test_representative_family_profiles_use_the_shared_registry(self):
        examples = {
            "cdr": "stim_cdr_schema_a.csv",
            "ipdr": "pakistan_ipdr_messy_synthetic.csv",
            "subscriber": "pakistan_subscriber_messy_synthetic.jsonl",
            "tower_location": "pakistan_tower_sector_messy_synthetic.csv",
            "anpr": "pakistan_anpr_messy_synthetic.csv",
            "transaction": "pakistan_financial_messy_synthetic.jsonl",
            "access_log": "pakistan_access_security_messy_synthetic.jsonl",
            "generic": "stim_generic_tabular_v1.csv",
        }
        for family, filename in examples.items():
            source = profile_records(FIXTURES / filename, filename)
            mapping = schema_profile(family, source["headers"], source["sample_records"], normalize_header, clean_text)
            with self.subTest(family=family):
                self.assertEqual(mapping["registry_version"], SCHEMA_REGISTRY_VERSION)
                self.assertEqual(len(mapping["source_columns"]), len(source["headers"]))
                self.assertTrue(mapping["mapping_profile_id"].endswith("/v1") or mapping["mapping_profile_id"].endswith("/v2"))
                if family == "transaction":
                    self.assertTrue({"observed_at", "amount", "source_account"}.issubset(mapping["recognized_canonical_fields"]))
                if family == "access_log":
                    self.assertTrue({"observed_at", "source_ip"}.issubset(mapping["recognized_canonical_fields"]))

    def test_dynamic_attributes_preserve_every_source_field_without_second_raw_store(self):
        raw = {
            "mobile_no": "03001234567", "full_name": "Protected Person",
            "cnic_no": "35202-1234567-1", "provider_note": "verified export", "novel_column": "x",
        }
        mapping = schema_profile("subscriber", raw.keys(), [raw], normalize_header, clean_text)
        canonical = {"msisdn": "+923001234567", "subscriber_name": "Protected Person", "cnic": "3520212345671"}
        attributes = dynamic_attributes(raw, mapping, canonical, evidence_id="e-1", version_id="v-1", source_file="subscriber.csv", row_number=2)
        self.assertEqual(len(attributes), len(raw))
        self.assertTrue(all(attribute["contract_version"] == DYNAMIC_ATTRIBUTE_VERSION for attribute in attributes))
        by_source = {attribute["source_field"]: attribute for attribute in attributes}
        self.assertEqual(by_source["provider_note"]["mapping_state"], "recognized_extension")
        self.assertEqual(by_source["novel_column"]["mapping_state"], "unmapped")
        for source_field in ("mobile_no", "full_name", "cnic_no"):
            self.assertEqual(by_source[source_field]["value_visibility"], "protected_source_only")
            self.assertIsNone(by_source[source_field]["raw_value"])
            self.assertIsNone(by_source[source_field]["typed_value"])
        self.assertEqual(by_source["mobile_no"]["source_locator"]["row_number"], 2)

    def test_quality_accounting_is_deterministic_and_capability_specific(self):
        raw = {"MSISDN": "923001111111", "CALL_START_DT_TM": "03/04/2026 01:00:00", "CALL_DIALED_NUM": "923002222222"}
        mapping = schema_profile("cdr", raw.keys(), [raw], normalize_header, clean_text)
        quality = quality_summary(mapping, {
            "total_rows": 2, "rejected_rows": 1,
            "rejection_codes": {"ambiguous_date_order": 1},
        }, inserted=1)
        self.assertEqual(quality["contract_version"], QUALITY_CONTRACT_VERSION)
        self.assertEqual(quality["state"], "needs_review")
        self.assertEqual(quality["counts"]["input_records"], 2)
        self.assertEqual(quality["counts"]["duplicate_records"], 0)
        readiness = {entry["id"]: entry["status"] for entry in quality["capability_readiness"]}
        self.assertEqual(readiness["record_inspection"], "available")
        self.assertEqual(readiness["temporal_activity"], "unavailable")

    def test_schema_mapping_contains_no_source_values(self):
        source = profile_records(FIXTURES / "stim_cdr_schema_a.csv", "stim_cdr_schema_a.csv")
        mapping = schema_profile("cdr", source["headers"], source["sample_records"], normalize_header, clean_text)
        serialized = json.dumps(mapping)
        for record in source["sample_records"]:
            self.assertNotIn(record["MSISDN"], serialized)
            self.assertNotIn(record["CALL_DIALED_NUM"], serialized)

    def test_missing_generic_event_time_is_explicitly_unresolved(self):
        source_job = IngestJob(
            job_id="00000000-0000-0000-0000-000000000202", tenant_id="stim-test", user_id="",
            collection_id="generic", file_id="generic", source_file="generic.csv", spool_path="generic.csv",
            sha256="3" * 64, record_type="generic", headers=[], metadata={},
        )
        record = normalize_generic_row(source_job, {"subject": "row-1", "note": "no source time"}, source_job.job_id, 2, ADAPTERS["generic"])
        details = json.loads(record["normalized_record"])
        self.assertIsNone(record["observed_at"])
        self.assertEqual(details["canonical_time_state"], "unresolved")
        self.assertEqual(details["persistence_timestamp_role"], "ingest_surrogate_not_source_time")

    def test_pathological_structured_width_is_rejected_instead_of_truncated(self):
        with self.assertRaisesRegex(ValueError, "maximum supported structured width is 512"):
            validate_structured_profile_bounds({"headers": [f"field_{index}" for index in range(513)]})

    def test_generic_fallback_preserves_unknown_schema_without_family_invention(self):
        audit = audit_structured_source(FIXTURES / "stim_generic_tabular_v1.csv", record_type="auto")
        self.assertEqual(audit["routing"]["detected_record_type"], "generic")
        self.assertEqual(audit["accounting"]["accepted_rows"], 2)
        self.assertEqual(audit["schema"]["family_mapping"]["recognized_canonical_fields"], [])
        self.assertEqual(audit["quality"]["state"], "ready_with_warnings")


if __name__ == "__main__":
    unittest.main()
