#!/usr/bin/env python3
"""Puts the questions of a reference case's catalogue to a RUNNING forensic API (the live Ask engine, through /query/hybrid) and writes the answers in the format of tools/ask_eval/judge.py.

    python -X utf8 tools/showcase/ask_case.py CATALOGUE.json --collection orion-lahore-reference-20261030 --out R.jsonl [--banks documents,plates] [--types doc_mentions] [--skip-types makers]
                                                              [--per-type 2] [--ids DOC-001,DOC-002]
    python -X utf8 tools/ask_eval/judge.py CATALOGUE.json R.jsonl --show-wrong

One question at a time (the model is one process), each answer written and flushed as soon as it arrives, so a partial run can be judged. The engine's own state comes from the response header
X-Ask-Engine-V2 (answered, abstained or declined); when the engine declined and the existing path answered, the line says declined and carries that answer under `older_path_answer`, and the judge scores
it as a decline (it is not the engine's answer). The API key is read from the main checkout's .env.forensic-runtime.local and never printed. Nothing here writes to the case.
"""
import argparse
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
MAIN = os.path.join(os.path.dirname(os.path.abspath(os.path.join(HERE, "..", ".."))), "NexusAI")


def api_key():
    with open(os.path.join(MAIN, ".env.forensic-runtime.local"), encoding="utf-8") as handle:
        for line in handle:
            if line.startswith("FORENSIC_RECORDS_API_KEY="):
                return line.split("=", 1)[1].strip()
    raise SystemExit("FORENSIC_RECORDS_API_KEY is not in the env file of the main checkout")


def ask(url, key, collection, question, timeout):
    body = json.dumps({"tenant_id": "default", "collection_id": collection, "query": question}).encode("utf-8")
    request = urllib.request.Request(url.rstrip("/") + "/query/hybrid", data=body, method="POST")
    for header, value in (("Content-Type", "application/json"), ("Authorization", "Bearer " + key), ("X-Forensic-Tenant-ID", "default"), ("X-Forensic-Collection-ID", collection),
                          ("X-Forensic-Actor-ID", "investigation-workspace"), ("X-Forensic-Subject-ID", "investigation-workspace"), ("X-Forensic-Actor-Role", "user")):
        request.add_header(header, value)
    started = time.time()
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            code, headers, text = response.status, dict(response.headers.items()), response.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as error:
        code, headers, text = error.code, dict(error.headers.items()), error.read().decode("utf-8", "replace")
    except Exception as error:  # noqa: BLE001 - a timeout or a refused connection is a result, not a crash
        return {"http": 0, "error": type(error).__name__, "seconds": round(time.time() - started, 1)}
    try:
        data = json.loads(text)
    except ValueError:
        data = {}
    return {"http": code, "headers": headers, "data": data, "seconds": round(time.time() - started, 1)}


def to_result(question, reply):
    header = reply.get("headers", {}).get("X-Ask-Engine-V2", "") if reply.get("http") else ""
    state = re.search(r"state=(\w+)", header)
    state = state.group(1) if state else "declined"
    data = reply.get("data") or {}
    enterprise = data.get("enterprise") or {}
    derivation = enterprise.get("derivation") or {}
    grid = enterprise.get("data_grid") or {}
    answer = enterprise.get("executive_answer") or data.get("answer") or ""
    result = {"id": question["id"], "arm": "engine", "state": state, "answer": answer if state != "declined" else "", "rows": grid.get("rows") if state == "answered" else None,
              "shape": enterprise.get("result_shape") or "", "sql": derivation.get("sql") or "", "views": derivation.get("views") or [], "attempts": derivation.get("attempts"),
              "repairs": derivation.get("repairs"), "limitations": enterprise.get("limitations") or [], "reason": "" if state != "declined" else "the engine declined; the existing path answered or not",
              "total_ms": int(reply.get("seconds", 0) * 1000), "http": reply.get("http"), "header": header[:200]}
    if state == "declined":
        result["older_path_answer"] = answer[:300]
    if reply.get("error"):
        result["error"] = reply["error"]
    return result


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("catalogue")
    parser.add_argument("--collection", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--url", default="http://127.0.0.1:8091")
    parser.add_argument("--banks")
    parser.add_argument("--types")
    parser.add_argument("--skip-types")
    parser.add_argument("--per-type", type=int, default=0)
    parser.add_argument("--ids")
    parser.add_argument("--timeout", type=int, default=420)
    args = parser.parse_args(argv)
    with open(args.catalogue, encoding="utf-8") as handle:
        loaded = json.load(handle)
    questions = loaded["questions"] if isinstance(loaded, dict) else loaded
    split = lambda value: set(value.split(",")) if value else set()   # noqa: E731
    banks, types, skip, ids = split(args.banks), split(args.types), split(args.skip_types), split(args.ids)
    seen, chosen = {}, []
    for q in questions:
        if (banks and q.get("bank") not in banks) or (types and q["type"] not in types) or q["type"] in skip or (ids and q["id"] not in ids):
            continue
        seen[q["type"]] = seen.get(q["type"], 0) + 1
        if args.per_type and seen[q["type"]] > args.per_type:
            continue
        chosen.append(q)
    key = api_key()
    print("%d questions to collection %s" % (len(chosen), args.collection), flush=True)
    with open(args.out, "a", encoding="utf-8") as out:
        for q in chosen:
            reply = ask(args.url, key, args.collection, q["question"], args.timeout)
            result = to_result(q, reply)
            out.write(json.dumps(result, ensure_ascii=False) + "\n")
            out.flush()
            print("%-9s %-20s %-9s %6.1fs  %s" % (q["id"], q["type"], result["state"], reply.get("seconds", 0), (result["answer"] or result.get("older_path_answer", ""))[:70].replace("\n", " ")), flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
