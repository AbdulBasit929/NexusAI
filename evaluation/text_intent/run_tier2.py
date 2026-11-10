#!/usr/bin/env python3
"""Tier 2 and Tier 3 of docs/roadmap/text-intent-preregistration-20261110.md.

Tier 2: every positive that Tier 1 routed is sent, as the analyst's sentence only, to the staging API (switch on), and the evidence ids it returns are compared with the SQL-independent
oracle of the bank:  lower bound <= returned <= upper bound.  Wrong = a returned evidence outside the upper bound, or a missing lower-bound evidence while the response says the search was
exhaustive.  A missing one under an INCOMPLETE search is reported separately (honest, not wrong).  Source-free sentences routed as 'any' are expected to reach one precise question, not an answer.

Tier 3: N plain negatives are sent to the staging API (switch on) and to the reference API (switch off); the analyst-facing answers must be identical.

    python -I -X utf8 evaluation/text_intent/run_tier2.py --bank evaluation/text_intent/bank-20261110.json --decisions C:/tmp/ti-tier1-run1.json --out C:/tmp/ti-tier2-run1.json \
        [--tier3 10 --reference http://127.0.0.1:4182/api/query/hybrid]
"""
import argparse
import json
import time
import urllib.request

HEADERS = {"Content-Type": "application/json", "X-Forensic-Tenant-ID": "default", "X-Forensic-Actor-ID": "investigation-workspace",
           "X-Forensic-Subject-ID": "investigation-workspace", "X-Forensic-Actor-Role": "user"}


def ask(url, case, question, timeout=420):
    body = json.dumps({"tenant_id": "default", "collection_id": case, "query": question}).encode("utf-8")
    request = urllib.request.Request(url, data=body, headers={**HEADERS, "X-Forensic-Collection-ID": case}, method="POST")
    started = time.time()
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            payload, header = json.loads(response.read().decode("utf-8")), response.headers.get("X-Text-Intent", "")
            status = response.status
    except Exception as error:  # a failed request is a result, not a crash
        return dict(status=0, error=str(error)[:200], seconds=round(time.time() - started, 1), header="", body={})
    return dict(status=status, header=header, seconds=round(time.time() - started, 1), body=payload)


def evidence_ids(body):
    results = (body.get("evidence") or {}).get("results") or []
    return sorted({(r.get("metadata") or {}).get("evidence_id") for r in results if (r.get("metadata") or {}).get("evidence_id")})


def analyst_text(body):
    enterprise = body.get("enterprise") or {}
    return " ".join(str(enterprise.get(k) or "") for k in ("executive_answer", "summary")).strip() or json.dumps(body.get("answer"), sort_keys=True, ensure_ascii=False)[:600]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--bank", required=True)
    parser.add_argument("--decisions", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--url", default="http://127.0.0.1:8191/query/hybrid")
    parser.add_argument("--reference", default="http://127.0.0.1:4182/api/query/hybrid")
    parser.add_argument("--tier3", type=int, default=0)
    args = parser.parse_args()
    bank = json.load(open(args.bank, encoding="utf-8"))
    decisions = {d["id"]: d for d in json.load(open(args.decisions, encoding="utf-8"))}
    report = dict(tier2=[], tier3=[])
    wrong = honest_incomplete = correct = clarified = unexpected = 0
    for item in bank["positives"]:
        decision = decisions[item["id"]]
        if decision["state"] != "ROUTED":
            continue
        result = ask(args.url, item["case"], item["question"])
        body = result["body"]
        evidence = body.get("evidence") or {}
        got, lower, upper = evidence_ids(body), set(item["oracle_evidence_lower"]), set(item["oracle_evidence_upper"])
        completeness = evidence.get("search_completeness") or (body.get("answer") or {}).get("search_completeness") or ""
        row = dict(id=item["id"], case=item["case"], question=item["question"], expected_family=item["expected_family"], routed_family=decision["family"],
                   header=result["header"], seconds=result["seconds"], route=body.get("route"), template=body.get("template"), state=evidence.get("result_state"),
                   completeness=completeness, returned=got, lower=sorted(lower), upper=sorted(upper))
        if decision["family"] == "any":
            asked = bool(((body.get("enterprise") or {}).get("clarification")) or (body.get("answer") or {}).get("clarification_required"))
            row["verdict"] = "CLARIFIED" if asked else "UNEXPECTED_ANSWER"
            clarified += asked
            unexpected += not asked
        else:
            extra = set(got) - upper
            missing = lower - set(got)
            if result["status"] != 200 or extra:
                row["verdict"] = "WRONG_EXTRA" if extra else "ERROR"
                wrong += 1
            elif missing and completeness == "EXHAUSTIVE":
                row["verdict"] = "WRONG_MISSING"
                wrong += 1
            elif missing:
                row["verdict"] = "INCOMPLETE_AND_SAID_SO"
                honest_incomplete += 1
            else:
                row["verdict"] = "CORRECT"
                correct += 1
        report["tier2"].append(row)
        print(row["id"], row["verdict"], row["routed_family"], len(got), "of", len(lower), "-", len(upper), completeness, result["seconds"], "s", flush=True)
    print(f"TIER 2: correct {correct}, honest-incomplete {honest_incomplete}, WRONG {wrong}, clarified {clarified}, unexpected answers {unexpected}")
    if args.tier3:
        plain = [n for n in bank["negatives"] if n["kind"] == "plain"]
        step = max(1, len(plain) // args.tier3)
        same = 0
        for n in plain[::step][:args.tier3]:
            case = "orion-lahore-reference-20261030"  # the case the verified bank questions were written for
            on, off = ask(args.url, case, n["question"]), ask(args.reference, case, n["question"])
            identical = analyst_text(on["body"]) == analyst_text(off["body"]) and on["status"] == off["status"] == 200
            same += identical
            report["tier3"].append(dict(id=n["id"], question=n["question"], identical=identical, on_header=on["header"], on_seconds=on["seconds"], off_seconds=off["seconds"],
                                        on=analyst_text(on["body"])[:300], off=analyst_text(off["body"])[:300]))
            print("TIER 3", n["id"], "identical" if identical else "DIFFERS", on["header"], flush=True)
        print(f"TIER 3: {same}/{len(report['tier3'])} identical")
    json.dump(report, open(args.out, "w", encoding="utf-8", newline="\n"), ensure_ascii=False, indent=1)


if __name__ == "__main__":
    main()
