import json
import unittest
from pathlib import Path

from ingestion.forensic_records.forensic_contracts import load_platform_catalog
from ingestion.forensic_records.worker import (
    ADAPTERS,
    IngestJob,
    compatibility_adapter_registry,
    normalize_header,
    resolve_adapter,
)


GOLDEN_PATH = Path(__file__).parent / "fixtures" / "phase5a_adapter_contract_goldens_v1.json"


class Phase5AContractTests(unittest.TestCase):
    def job(self, record_type: str = "auto") -> IngestJob:
        return IngestJob(
            job_id="00000000-0000-0000-0000-000000000001",
            tenant_id="default",
            user_id="",
            collection_id="phase5a-contract-test",
            file_id="file-1",
            source_file="phase5a.csv",
            spool_path="phase5a.csv",
            sha256="0" * 64,
            record_type=record_type,
            headers=[],
            metadata={"source_timezone": "Asia/Karachi"},
        )

    def legacy_resolve(self, job: IngestJob, headers: list[str], force_generic: bool = False):
        requested = normalize_header(job.record_type)
        if force_generic and requested in {"", "auto", "generic"}:
            return ADAPTERS["generic"]
        if requested in ADAPTERS and requested != "generic":
            adapter = ADAPTERS[requested]
            if adapter.matches(headers):
                return adapter
            raise ValueError(f"{requested} adapter selected but required columns are missing")
        for name, adapter in ADAPTERS.items():
            if name != "generic" and adapter.matches(headers):
                return adapter
        return ADAPTERS["generic"]

    def test_catalog_defines_versioned_lifecycle_operations_agents_and_model_roles(self):
        catalog = load_platform_catalog()
        self.assertEqual(catalog.schema_version, "forensics.platform/v1")
        self.assertEqual(catalog.query_plan_contract, "forensics.query-plan/v1")
        self.assertEqual(catalog.response_contract, "forensics.enterprise-response/v1")
        self.assertEqual(
            catalog.adapter_lifecycle,
            ("detect", "validate", "plan", "extract", "normalize", "derive", "persist", "index", "verify", "capabilities"),
        )
        self.assertEqual(
            {adapter.implementation_key for adapter in catalog.adapters},
            {
                "cdr",
                "ipdr",
                "anpr",
                "subscriber",
                "tower_location",
                "generic",
                "unified_media_worker",
                "native_document_worker",
            },
        )
        self.assertTrue(all(adapter.preserves_legacy_output for adapter in catalog.adapters))
        self.assertEqual(len({operation.id for operation in catalog.operations}), len(catalog.operations))
        self.assertTrue(
            {"PLANNED", "ENGINEERING_ONLY", "LIMITED", "CERTIFIED"}.issuperset(
                operation.maturity for operation in catalog.operations
            )
        )
        self.assertIn("Case_Intelligence_Orchestrator", {agent.id for agent in catalog.agents})
        self.assertIn("Communications_CDR_Analyst", {agent.id for agent in catalog.agents})
        self.assertIn("Generic_Data_Quality_Analyst", {agent.id for agent in catalog.agents})
        roles = {role.id: role for role in catalog.model_roles}
        self.assertEqual(roles["structured_exact"].selection_policy, "no_model")
        self.assertEqual(roles["query_planner"].configured_model, "qwen_qwen3-4b-instruct-2507")
        self.assertEqual(roles["embedding"].configured_model, "qwen3-embedding-0.6b")
        self.assertEqual(roles["extraction"].status, "benchmark_required")

    def test_registry_selection_is_equivalent_to_the_pre_contract_algorithm(self):
        cases = [
            ("auto", ["MSISDN", "CALL_DIALED_NUM", "CALL_START_DT_TM"], False),
            ("cdr", ["source_number", "target_number", "call_time"], False),
            ("auto", ["event_time", "custom_value"], False),
            ("generic", ["plate_number", "camera_id", "capture_time"], True),
        ]
        for record_type, headers, force_generic in cases:
            with self.subTest(record_type=record_type, headers=headers, force_generic=force_generic):
                expected = self.legacy_resolve(self.job(record_type), headers, force_generic)
                actual = resolve_adapter(self.job(record_type), headers, force_generic)
                self.assertIs(actual, expected)

        registry = compatibility_adapter_registry()
        self.assertEqual(registry.descriptor("cdr").id, "nexusai.adapter.cdr")
        self.assertEqual(registry.descriptor("ipdr").id, "nexusai.adapter.ipdr")
        self.assertEqual(registry.descriptor("anpr").id, "nexusai.adapter.anpr")
        self.assertEqual(registry.descriptor("subscriber").id, "nexusai.adapter.subscriber_identity")
        self.assertEqual(registry.descriptor("tower_location").id, "nexusai.adapter.tower_location")
        self.assertEqual(registry.descriptor("generic").id, "nexusai.adapter.generic_tabular")

    def test_cdr_and_generic_compatibility_wrappers_match_golden_outputs(self):
        golden = json.loads(GOLDEN_PATH.read_text(encoding="utf-8"))

        cdr = compatibility_adapter_registry().normalize(
            "cdr",
            self.job("cdr"),
            golden["cdr"]["input"],
            "00000000-0000-0000-0000-000000000001",
            2,
        )
        cdr_selected = {key: cdr[key] for key in golden["cdr"]["expected"]}
        cdr_selected["call_start_ts"] = cdr_selected["call_start_ts"].isoformat()
        cdr_selected["call_end_ts"] = cdr_selected["call_end_ts"].isoformat()
        self.assertEqual(cdr_selected, golden["cdr"]["expected"])

        generic = compatibility_adapter_registry().normalize(
            "generic",
            self.job("generic"),
            golden["generic"]["input"],
            "00000000-0000-0000-0000-000000000001",
            2,
            ADAPTERS["generic"],
        )
        generic_selected = {key: generic[key] for key in golden["generic"]["expected"]}
        generic_selected["observed_at"] = generic_selected["observed_at"].isoformat()
        self.assertEqual(generic_selected, golden["generic"]["expected"])


if __name__ == "__main__":
    unittest.main()
