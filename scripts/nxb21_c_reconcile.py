"""Reconcile C proof/coverage metadata; never execute queries or mutate runtime.

The embedded capability references own semantics. These reports are derived
review artifacts, not operation, certification or suggestion registries.
"""
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "reports/nxb21"


def read(path):
    return json.loads((ROOT / path).read_text(encoding="utf-8"))


def write(name, value):
    (OUT / name).write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def main():
    inventory = read("reports/nxb21/reconciliation-inventory-v1.json")
    reference_path = "api/forensic_records/contracts/query-capability-references-v1.json"
    refs = read(reference_path)
    variants = read("api/forensic_records/contracts/query-variant-ledger-v1.json")["entries"]
    operations = {o["operation_id"]: o for o in inventory["operations"]}
    noncatalog = {v["canonical_operation"] for v in variants if v["source_corpus"] != "operation_catalog"}
    catalog_only = set(operations) - noncatalog
    assert catalog_only == set(inventory["query_summary"]["only_catalog_coverage"])
    depth = {
        "cdr.geospatial_movement": "Movement interpretation needs separate tower coverage and location limitations.",
        "anpr.co_travel": "Co-travel needs bounded independent route/timing proof; co-occurrence is not association.",
        "tower.cdr_join": "Join cardinality, ambiguity and interval coverage need a dedicated correlation slice.",
        "forensics.relationship_network": "Graph depth requires independent edge authority and ownership proof.",
        "forensics.repeated_location_visits": "Repeated visits need explicit location/time uncertainty fixtures.",
        "forensics.executive_case_brief": "Broad summary composition follows validated family packets.",
    }
    classifications = []
    for operation_id in sorted(catalog_only):
        o = operations[operation_id]
        internal = o["exposure_status"] == "engineering_only"
        disposition = "INTERNAL_ONLY" if internal else "DEFER_DEPTH" if operation_id in depth else "CERTIFY_IN_F"
        reason = "Canonical engineering-only exposure is preserved." if internal else depth.get(operation_id, "Useful bounded analyst operation; expand independent positive/zero and presentation proof in F.")
        classifications.append({
            "operation_id": operation_id, "family": o["family_id"], "current_tier": o["criticality_tier"],
            "canonical_certification": o["certification"]["certification_level"],
            "analyst_usefulness": "INTERNAL_ONLY" if internal else "APPLICABLE",
            "existing_evidence": "A family inventory only; target-specific eligibility still requires scoped runtime projection.",
            "query_capability_mapping": [c["query_capability_id"] for c in refs["capabilities"] if c["operation_ref"] == operation_id],
            "language_relevance": "NOT_APPLICABLE_TO_ORDINARY_ASK" if internal else "ENGLISH_URDU_ROMAN_URDU_MIXED_APPLICABLE",
            "positive_proof_status": "CANONICAL_OPERATION_PROOF_ONLY" if o["certification"]["certification_level"] == "PRODUCT_CERTIFIED" else "UNTESTED_IN_C",
            "negative_proof_status": "REQUIRES_OPERATION_SPECIFIC_ORACLE",
            "citation_proof_status": "EXPECTED; canonical operation proof retained separately",
            "activity_proof_status": "NOT_INFERRED_FROM_OPERATION_PASS",
            "disposition": disposition, "reason": reason,
            "suggestion_eligible": o["certification"]["suggestion_eligible"],
            "new_suggestion_eligibility": False,
        })
    write("c-catalog-only-disposition-v1.json", {"schema_version": "nexusai.nxb21.c-catalog-disposition/v1", "classification_count": len(classifications), "operation_count": len(operations), "records": classifications})
    language = []
    proof = []
    category_families = {
        "plain_text_notes": ["document_intelligence"], "pdf_office_email_documents": ["document_intelligence"],
        "cdr": ["communications_cdr"], "ipdr_network_sessions": ["network_ipdr"],
        "anpr_vehicle_sightings": ["anpr_vehicles"], "subscriber_identity": ["subscriber_identity"],
        "tower_location": ["tower_location"], "financial_transactions": ["financial_transactions"],
        "logs_access_security": ["access_security_logs"], "generic_tabular": ["generic_tabular"],
        "images_and_ocr": ["image_intelligence", "face_intelligence", "anpr_vehicles"],
        "audio_and_stt": ["audio_intelligence"], "video": ["video_intelligence", "audio_intelligence"],
        "spreadsheets_and_columnar": ["communications_cdr", "network_ipdr", "anpr_vehicles", "subscriber_identity", "tower_location", "financial_transactions", "access_security_logs", "generic_tabular"],
    }
    for c in refs["capabilities"]:
        operation_id = c["operation_ref"]
        language.append({"query_capability_id": c["query_capability_id"], "typed_target": "LANGUAGE_INDEPENDENT", "languages": {
            l: {"applicability": "APPLICABLE", "natural_query_validation": c["language_coverage"][l],
                "existing_ledger_entries": sum(v["canonical_operation"] == operation_id and v["language"] == l for v in variants),
                "literal_payload_proof_refs": c["proof_refs"], "next_phase": "D/E/F"}
            for l in ["english", "urdu", "roman_urdu", "mixed"]}})
        proof.append({"query_capability_id": c["query_capability_id"], "required_cases": c["proof_requirements"],
            "proof_refs": c["proof_refs"], "readiness_source_tests": "C query capability references; C database capability isolation",
            "positive_zero_execution": "B1_BOUNDED_SOURCE_PROOF" if c["proof_refs"] else "NOT_RUN_IN_C",
            "locators": c["locator_types"], "citation_contract": "EXPECTED", "navigation": "NOT_CERTIFIED_BY_C",
            "activity": "ADDITIVE_SOURCE_SERIALIZATION_ONLY", "live_reopen": "NOT_RUN_IN_C",
            "certification_source": "api/forensic_records/contracts/operation-certification-v1.json"})
    write("c-language-coverage-v1.json", {"schema_version": "nexusai.nxb21.c-language-coverage/v1", "records": language,
        "legacy_variant_count": len(variants), "legacy_duplicate_count": sum(bool(v.get("duplicate_of")) for v in variants),
        "natural_language_coverage_is_not_literal_matching": True, "proof_dimensions": proof})
    write("c-input-boundaries-v1.json", {"schema_version": "nexusai.nxb21.c-input-boundaries/v1", "source": reference_path,
        "qualification_scope": "Existing bounded source pipelines, not universal codec or format certification.",
        "boundaries": refs["input_boundaries"], "family_matrix": [
            {"advertised_category": f["id"], "actual_adapter_or_pipeline": f["adapter"],
             "advertised_formats": f["formats"],
             "live_evidence_inventory": {k: f.get(k, 0) for k in ["registered_evidence", "indexed_records", "derived_artifacts"]},
             "counts_overlap_do_not_sum": True,
             "executable_families": category_families.get(f["id"], []),
             "query_capability_refs": [c["query_capability_id"] for c in refs["capabilities"] if c["family"] in category_families.get(f["id"], []) and set(c["input_types"]) & set(f["formats"])],
             "readiness_authority": "Authorized current-version artifacts/records and separate current processor receipt; format overlap does not imply capability.",
             "limitations": f["next_gate"]}
            for f in inventory["families"]]})
    write("c-query-capability-ledger-v1.json", {"schema_version": "nexusai.nxb21.c-capability-review/v1", "status": "SOURCE_CANDIDATE",
        "canonical_reference_source": reference_path, "reference_sha256": hashlib.sha256((ROOT / reference_path).read_bytes()).hexdigest(),
        "not_an_executable_registry": True, "capability_count": len(refs["capabilities"]),
        "reference_map": [{"query_capability_id": c["query_capability_id"], "operation_ref": c["operation_ref"], "reference_kind": c["reference_kind"],
                           "canonical_certification": operations.get(c["operation_ref"], {}).get("certification", {}).get("certification_level", "DIRECT_DESCRIPTOR_NOT_IN_QUERY_CERTIFICATION_REGISTRY")}
                          for c in refs["capabilities"]],
        "design_reconciliation": refs["design_reconciliation"], "added_capabilities": ["case.evidence_package"],
        "unavailable_intents": refs["unavailable_intents"]})
    old = read("reports/nxb21/query-capability-design-v1.json")
    old.update(status="SUPERSEDED_BY_C_SOURCE_REFERENCES", canonical_reference_source=reference_path,
               c_review_artifact="reports/nxb21/c-query-capability-ledger-v1.json")
    old.pop("capabilities", None)
    old["design_reconciliation"] = refs["design_reconciliation"]
    old["family_gap_matrix_source"] = "reports/nxb21/c-input-boundaries-v1.json"
    old.pop("family_gap_matrix", None)
    write("query-capability-design-v1.json", old)
    print(f"Reconciled {len(refs['capabilities'])} capability references, {len(classifications)} catalog-only operations, {len(refs['input_boundaries'])} category-format boundaries.")


if __name__ == "__main__":
    main()
