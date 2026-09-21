"""Read-only NX-B2.1 inventory. No model calls, retained writes or certification promotion.

Raw evidence stays in the existing ignored local-acceptance-models tree. Public
output contains catalog contracts and aggregate coverage, never runtime targets.
"""
import argparse
import collections
import datetime
import hashlib
import json
from pathlib import Path
import subprocess
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
PRIVATE = ROOT / "local-acceptance-models/nxb21-reconciliation"
PUBLIC = ROOT / "reports/nxb21"


def read(path):
    return json.loads((ROOT / path).read_text(encoding="utf-8-sig"))


def save(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def get(path):
    with urllib.request.urlopen("http://localhost:8080" + path, timeout=30) as response:
        return json.load(response)


def count(values):
    return dict(sorted(collections.Counter(values).items()))


def capture(case):
    ignored = subprocess.run(["git", "check-ignore", str(PRIVATE / "snapshot.json")],
                             cwd=ROOT, capture_output=True, text=True)
    if ignored.returncode != 0:
        raise RuntimeError("Private acceptance output must already be Git-ignored")
    templates = get("/api/records/forensic/templates")["templates"]
    operations = get("/api/v1/forensics/operations")
    adapters = get("/api/v1/forensics/adapters")
    agents = get("/api/v1/forensics/agents")
    running_agents = get("/api/agents")
    caps = get("/api/records/forensic/capabilities?" + urllib.parse.urlencode({"collection_id": case}))
    base = "/api/v1/forensics/cases/" + urllib.parse.quote(case, safe="") + "/evidence"
    evidence = get(base + "?limit=100")
    pagination = evidence.get("pagination", {})
    if pagination.get("has_next") or pagination.get("evidence_total", 0) > len(evidence["items"]):
        raise RuntimeError("Evidence pagination requires continuation; refusing partial inventory")
    details = [get(base + "/" + item["evidence_id"]) for item in evidence["items"]]
    now = datetime.datetime.now(datetime.timezone.utc).isoformat()
    source_catalog = read("api/forensic_records/contracts/forensic-platform-v1.json")
    cert = read("api/forensic_records/contracts/operation-certification-v1.json")["entries"]
    variants = read("api/forensic_records/contracts/query-variant-ledger-v1.json")["entries"]
    corpus = read("api/forensic_records/contracts/forensic-family-query-answer-corpus-v1.json")
    cert_by_id = {x["operation_id"]: x for x in cert}
    ops_by_id = {x["id"]: x for x in operations["operations"]}
    source_ids = set(cert_by_id)
    template_ids = {x["operation_id"] for x in templates}
    if source_ids != template_ids or not template_ids.issubset(ops_by_id):
        raise RuntimeError("Source/live operation parity failed")
    by_operation = collections.defaultdict(list)
    for variant in variants:
        by_operation[variant["canonical_operation"]].append(variant)
    inventory = []
    for template in templates:
        opid = template["operation_id"]
        c = cert_by_id[opid]
        group = by_operation[opid]
        inventory.append({
            **template, "executor_descriptor": ops_by_id[opid],
            "certification": c, "source_validation": c["certification_level"],
            "live_registration": True, "live_execution_certified_this_turn": False,
            "current_live_availability": "REGISTERED; per-request scope/data/model validation required; see named probes for execution evidence",
            "query_coverage": {"count": len(group), "languages": count(x["language"] for x in group),
                "non_catalog_count": sum(x["source_corpus"] != "operation_catalog" for x in group)},
            "query_semantics_certification": "NOT_RECORDED_AS_SEPARATE_CAPABILITY",
        })
    query_inventory = []
    for entry in variants:
        query_inventory.append({**entry,
            "operation_exists": entry["canonical_operation"] in template_ids,
            "time_usage": bool(entry["expected_time"]),
            "multiple_entities_recorded": bool(entry.get("expected_entities") and len(entry["expected_entities"]) > 1),
            "composition_contract": "NOT_RECORDED_PER_VARIANT",
            "scope_proof": "NOT_RECORDED_PER_VARIANT",
            "positive_negative_zero": "NOT_RECORDED_PER_VARIANT",
            "semantic_taxonomy": "NOT_RECORDED_PER_VARIANT",
            "authority_citation_contract_ref": entry["canonical_operation"],
            "live_navigation_activity_proof": "NOT_RECORDED_PER_VARIANT",
            "status_meaning": "source-ledger routing/parameters/equivalence; not live certification"})
    artifacts = [a for detail in details for a in detail.get("derived_artifacts", [])]
    product = [x["operation_id"] for x in cert if x["certification_level"] == "PRODUCT_CERTIFIED"]
    result = {
        "schema_version": "nexusai.nxb21.reconciliation/v1", "captured_at": now,
        "authority": "NX-B2.1A read-only reconciliation; not an executable registry or certification ledger",
        "source_live_parity": {"executable_operations": len(templates),
            "discovery_descriptors": len(ops_by_id), "static_platform_descriptors": len(source_catalog["operations"]),
            "query_variants": len(variants), "source_family_corpus": len(corpus["entries"]),
            "live_family_corpus": len(caps["query_corpus"]["entries"]),
            "all_executable_ids_equal_certification_ids": True,
            "catalog_version": operations["catalog_version"]},
        "models": get("/v1/models"), "model_runtime": get("/system"),
        "family_summary": caps["summary"], "families": caps["families"],
        "adapters": adapters["adapters"], "agents": agents["agents"], "model_roles": agents["model_roles"],
        "running_agents": {"count": running_agents["agentCount"], "names": running_agents["agents"],
                           "statuses": running_agents["statuses"]},
        "operations": inventory, "non_query_descriptors": [x for x in ops_by_id.values() if x["id"] not in template_ids],
        "query_ledger": query_inventory,
        "query_summary": {"languages": count(x["language"] for x in variants),
            "statuses": count(x["status"] for x in variants),
            "duplicate_entries": [x["variant_id"] for x in variants if x.get("duplicate_of")],
            "orphan_variants": [x["variant_id"] for x in variants if x["canonical_operation"] not in template_ids],
            "only_catalog_coverage": [x["operation_id"] for x in inventory if x["query_coverage"]["non_catalog_count"] == 0],
            "missing_urdu": [x["operation_id"] for x in inventory if not x["query_coverage"]["languages"].get("urdu")],
            "missing_roman_urdu": [x["operation_id"] for x in inventory if not x["query_coverage"]["languages"].get("roman_urdu")]},
        "certification_summary": {"levels": count(x["certification_level"] for x in cert),
            "product_certified_operations": product, "promotion_performed": False},
        "retained_acceptance_case": {"evidence_count": len(evidence["items"]),
            "modalities": count(x["modality"] for x in evidence["items"]),
            "processing_states": count(x["processing_status"] for x in evidence["items"]),
            "record_counts": caps["coverage"]["record_families_present"],
            "artifact_detail_counts": count(x["artifact_type"] for x in artifacts),
            "detail_limit_note": "Per-evidence detail results are bounded; DB aggregate is authoritative for completeness"},
        "source_hashes": {path: hashlib.sha256((ROOT / path).read_bytes()).hexdigest() for path in [
            "api/forensic_records/contracts/forensic-platform-v1.json",
            "api/forensic_records/contracts/operation-certification-v1.json",
            "api/forensic_records/contracts/query-variant-ledger-v1.json",
            "api/forensic_records/derived_text_query.go", "core/services/agents/forensic_direct.go"]},
        "deployment_performed": False, "database_migration": False, "product_certification_performed": False,
    }
    save(PRIVATE / "snapshot.json", {"captured_at": now, "case": case, "details": details, "evidence": evidence})
    save(PUBLIC / "reconciliation-inventory-v1.json", result)
    print(json.dumps({"source_live_parity": result["source_live_parity"], "query_summary": result["query_summary"],
                      "certification": result["certification_summary"], "families": caps["summary"]}, indent=2))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--case", required=True, help="Existing authorized case to inspect")
    capture(parser.parse_args().case)
