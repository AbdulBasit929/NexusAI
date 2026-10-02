# REGRESSION REPLAY: the 103 corpus questions with the front door OFF, then ON, on the same image. See PREREGISTRATION.md.
#
# The questions are the repository's own evaluation files (golden 62, held-out 13, media 28). Nothing is scored against an oracle
# here except a simple number check; the line that matters is whether switching the front door on changes any answer.
#
#     python replay_corpus.py run <arm>            # asks every question, writes <arm>/replay.json
#     python replay_corpus.py compare <off> <on>   # lists every difference and prints the verdict against the thresholds
#
# Read-only against the running API. Run it once with FORENSIC_CONVERSATION_FRONT_DOOR=false and once with true.
import io
import json
import os
import re
import statistics
import sys
import time
import urllib.error
import urllib.request

URL = os.environ.get("NEXUSAI_PROBE_URL", "http://127.0.0.1:8091/query/hybrid")
API_KEY = os.environ.get("FORENSIC_RECORDS_API_KEY", "")
HERE = os.path.dirname(os.path.abspath(__file__))
EVAL = os.path.normpath(os.path.join(HERE, "..", "..", "evaluation"))
FILES = [("golden", "golden_questions_v2.json"), ("holdout", "holdout_questions_v1.json"), ("media", "holdout_media_v1.json")]


def load_questions():
    rows = []
    for label, name in FILES:
        with io.open(os.path.join(EVAL, name), encoding="utf-8") as handle:
            data = json.load(handle)
        aliases = data["collections"]
        for q in data["questions"]:
            rows.append({"set": label, "id": q["id"], "collection": aliases[q["c"]], "question": q["q"],
                         "check": q.get("check"), "expect": q.get("expect")})
    return rows


def ask(collection, question, timeout=900):
    payload = json.dumps({"tenant_id": "default", "collection_id": collection, "query": question}).encode("utf-8")
    request = urllib.request.Request(URL, data=payload, method="POST")
    for header, value in (("Content-Type", "application/json"), ("X-Forensic-Tenant-ID", "default"),
                          ("X-Forensic-Collection-ID", collection), ("X-Forensic-Actor-ID", "investigation-workspace"),
                          ("X-Forensic-Subject-ID", "investigation-workspace"), ("X-Forensic-Actor-Role", "user")):
        request.add_header(header, value)
    if API_KEY:
        request.add_header("Authorization", "Bearer " + API_KEY)
    started = time.time()
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return response.status, json.loads(response.read().decode("utf-8")), time.time() - started, response.headers.get("X-Front-Door", "")
    except urllib.error.HTTPError as exc:
        return exc.code, {"_body": exc.read().decode("utf-8")[:300]}, time.time() - started, ""
    except Exception as exc:  # noqa: BLE001 - a probe reports its own failure
        return 0, {"_error": str(exc)[:300]}, time.time() - started, ""


def text_of(blob):
    enterprise = blob.get("enterprise") if isinstance(blob.get("enterprise"), dict) else {}
    narrative = enterprise.get("narrative") if isinstance(enterprise.get("narrative"), dict) else {}
    parts = [enterprise.get("executive_answer"), narrative.get("direct_answer"), enterprise.get("summary")]
    return "\n".join(p.strip() for p in parts if isinstance(p, str) and p.strip())


def number_stated(expect, text):
    if not isinstance(expect, (int, float)):
        return None
    forms = {str(expect), "{:,}".format(int(expect)) if float(expect).is_integer() else str(expect)}
    return any(re.search(r"(?<![\d,.])" + re.escape(f) + r"(?![\d,]|\.\d)", text) for f in forms)


def run(arm):
    out = os.path.join(HERE, arm)
    os.makedirs(out, exist_ok=True)
    rows = []
    for item in load_questions():
        status, blob, seconds, header = ask(item["collection"], item["question"])
        text = text_of(blob)
        row = dict(item, http=status, route=blob.get("route"), request_class=blob.get("request_class"), seconds=round(seconds, 1),
                   front_door=header, answer_text=text, stated=number_stated(item["expect"], text) if item["check"] == "number" else None)
        rows.append(row)
        print("%-9s %-18s http=%s %6.1fs %-26s %s" % (item["set"], item["id"], status, seconds, ",".join(row["route"] or []) or "-", header or "-"))
        with io.open(os.path.join(out, "replay.json"), "w", encoding="utf-8") as handle:  # written as it goes, so an interrupted run keeps its rows
            json.dump(rows, handle, indent=1, ensure_ascii=False)
    numeric = [r for r in rows if r["stated"] is not None]
    print(json.dumps({"questions": len(rows), "errors": sum(1 for r in rows if r["http"] != 200),
                      "number_checks_stated": "%d/%d" % (sum(1 for r in numeric if r["stated"]), len(numeric))}))


def norm(text):
    return re.sub(r"\s+", " ", text or "").strip()


def compare(off_arm, on_arm):
    def load(arm):
        with io.open(os.path.join(HERE, arm, "replay.json"), encoding="utf-8") as handle:
            return {r["id"]: r for r in json.load(handle)}
    off, on = load(off_arm), load(on_arm)
    ids = [i for i in off if i in on]
    changed, rerouted, moved_to_chat, errors = [], [], [], []
    for i in ids:
        a, b = off[i], on[i]
        if b["http"] != 200 or a["http"] != 200:
            errors.append(i)
        if a["route"] != b["route"]:
            rerouted.append(i)
        if (b["route"] or []) == ["terminal"] and (a["route"] or []) != ["terminal"]:
            moved_to_chat.append(i)
        if norm(a["answer_text"]) != norm(b["answer_text"]):
            changed.append(i)
    lost = [i for i in ids if off[i]["stated"] and on[i]["stated"] is False]
    gained = [i for i in ids if off[i]["stated"] is False and on[i]["stated"]]
    print("compared %d questions (%s vs %s)" % (len(ids), off_arm, on_arm))
    print("answer text identical : %d" % (len(ids) - len(changed)))
    print("answer text changed   : %d  %s" % (len(changed), changed))
    print("route changed         : %d  %s" % (len(rerouted), rerouted))
    print("sent to the chat path : %d  %s   (data questions the front door answered itself)" % (len(moved_to_chat), moved_to_chat))
    print("number checks lost    : %s    gained: %s" % (lost, gained))
    print("HTTP errors           : %s" % errors)
    ms = []
    for i in ids:
        m = re.search(r"ms=(\d+)", on[i].get("front_door") or "")
        if m and (on[i]["route"] or []) != ["terminal"]:
            ms.append(int(m.group(1)) / 1000.0)
    if ms:
        print("front-door time on data questions: median %.1fs  min %.1fs  max %.1fs  (n=%d)" % (statistics.median(ms), min(ms), max(ms), len(ms)))
    deltas = [on[i]["seconds"] - off[i]["seconds"] for i in ids]
    print("wall-time change ON-OFF: median %+.1fs  total %+.0fs  (plan cache state matters; read with the front-door figure above)" % (statistics.median(deltas), sum(deltas)))
    for i in changed:
        print("\n--- %s  %s\n  OFF [%s]: %s\n  ON  [%s]: %s" % (i, off[i]["question"], ",".join(off[i]["route"] or []), norm(off[i]["answer_text"])[:260],
                                                              ",".join(on[i]["route"] or []), norm(on[i]["answer_text"])[:260]))


if __name__ == "__main__":
    if len(sys.argv) >= 3 and sys.argv[1] == "run":
        run(sys.argv[2])
    elif len(sys.argv) >= 4 and sys.argv[1] == "compare":
        compare(sys.argv[2], sys.argv[3])
    else:
        print(__doc__)
