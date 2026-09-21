"""Build the Operation & Query Inventory from the live catalogs plus baseline evidence.

Inputs: catalogs/templates.json and catalogs/operations.json (captured from the
live API: GET /query/templates, GET /api/v1/forensics/operations) and
baseline/results_adjudicated.json. Output: inventory.json.
"""
import collections
import json
import os

BASE = os.path.dirname(os.path.abspath(__file__))


def main():
    templates = json.load(open(os.path.join(BASE, "catalogs", "templates.json"), encoding="utf-8"))["templates"]
    operations = json.load(open(os.path.join(BASE, "catalogs", "operations.json"), encoding="utf-8"))["operations"]
    results = json.load(open(os.path.join(BASE, "baseline", "results_adjudicated.json"), encoding="utf-8"))

    hits = collections.defaultdict(list)
    for r in results:
        hits[r.get("template") or "(none)"].append({"id": r["id"], "verdict": r["manual_verdict"], "question": r["question"]})

    ops_by_id = {o["id"]: o for o in operations}
    items = []
    for t in sorted(templates, key=lambda x: (x["family_id"], x["name"])):
        op = ops_by_id.get((t.get("operation_id") or "").replace("forensics.", ""), None) or ops_by_id.get(t.get("operation_id"))
        observed = hits.get(t["name"], [])
        verdicts = collections.Counter(h["verdict"] for h in observed)
        if not observed:
            status = "NOT_EXERCISED"
        elif verdicts.get("ERROR"):
            status = "ERROR"
        elif verdicts.get("WRONG"):
            status = "MISROUTED_OR_WRONG"
        elif verdicts.get("PARTIAL"):
            status = "INCOMPLETE"
        elif verdicts.get("SAFE_FAIL") and not verdicts.get("CORRECT"):
            status = "CLARIFY_ONLY"
        else:
            status = "PASSED_BASELINE_UNCERTIFIED"
        items.append({
            "kind": "template", "name": t["name"], "family": t["family_id"], "route": t["route"],
            "operation_id": t.get("operation_id"), "required_inputs": [i["name"] for i in t.get("inputs", []) if i.get("required")],
            "declared_certification": t["certification_status"], "declared_exposure": t["exposure_status"],
            "platform_operation_maturity": (op or {}).get("maturity"),
            "baseline_observations": observed, "p0_status": status,
            "note": "Template was selected for these golden questions; a WRONG verdict can mean the question was misrouted TO this template, not that its SQL is wrong.",
        })
    template_ops = {t.get("operation_id") for t in templates}
    for o in sorted(operations, key=lambda x: x["id"]):
        if o["id"] in template_ops or ("forensics." + o["id"]) in template_ops:
            continue
        items.append({"kind": "platform_operation", "name": o["id"], "family": o["family_id"], "operation_kind": o["kind"],
                      "implementation": o.get("implementation"), "platform_operation_maturity": o["maturity"],
                      "baseline_observations": [], "p0_status": "NOT_EXERCISED"})

    cert_conflicts = [i["name"] for i in items if i["kind"] == "template"
                      and i["declared_certification"] != "certified" and i["platform_operation_maturity"] == "CERTIFIED"]
    summary = {
        "templates": len(templates), "platform_operations": len(operations),
        "p0_status": dict(collections.Counter(i["p0_status"] for i in items)),
        "declared_template_certified": sum(1 for t in templates if t["certification_status"] == "certified"),
        "declared_operation_CERTIFIED": sum(1 for o in operations if o["maturity"] == "CERTIFIED"),
        "certification_label_conflicts": cert_conflicts,
        "not_yet_inventoried": [
            "ingestion adapters / format decoders (ingestion/forensic_records/worker.py, media_metadata_*.go)",
            "UI controls that trigger backend operations (core/http/react-ui)",
        ],
    }
    json.dump({"contract": "nexusai.operation-inventory/v0", "generated": "2026-09-18", "summary": summary, "items": items},
              open(os.path.join(BASE, "inventory.json"), "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    print(json.dumps(summary, indent=1))


if __name__ == "__main__":
    main()
