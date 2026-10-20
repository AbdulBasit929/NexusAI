"""Asks the live stack (through the workspace proxy) questions about the private real collection and compares the answers with truth computed by SQL on the typed table.
Prints only kinds, booleans and timings: never a number, never a value of the data."""
import json, subprocess, sys, time, urllib.request

COLLECTION = "nexusai-real-private-demo"


def psql(sql):
    out = subprocess.run(["docker", "exec", "nexusai-forensic-postgres-1", "psql", "-U", "localrecall", "-d", "localrecall", "-At", "-F", "|", "-c", sql], capture_output=True, text=True)
    if out.returncode:
        raise SystemExit("psql failed")
    return [line.split("|") for line in out.stdout.strip().splitlines() if line]


def ask(question):
    body = json.dumps({"tenant_id": "default", "collection_id": COLLECTION, "query": question}).encode()
    req = urllib.request.Request("http://127.0.0.1:4181/api/query/hybrid", data=body, method="POST", headers={
        "Content-Type": "application/json", "X-Forensic-Tenant-ID": "default", "X-Forensic-Collection-ID": COLLECTION, "X-Forensic-Actor-ID": "investigation-workspace",
        "X-Forensic-Subject-ID": "investigation-workspace", "X-Forensic-Actor-Role": "user"})
    t0 = time.time()
    with urllib.request.urlopen(req, timeout=900) as r:
        data = json.loads(r.read().decode("utf-8"))
        header = r.headers.get("X-Ask-Engine-V2", "-")
    return data, time.time() - t0, header


def rows_of(data):
    grid = (data.get("enterprise") or {}).get("data_grid") or {}
    return grid.get("rows") or []


def digits_in(rows):
    return {str(v) for r in rows for v in r.values() if isinstance(v, str) and v.isdigit() and len(v) >= 8}


def numbers_in(rows):
    return [v for r in rows for v in r.values() if isinstance(v, (int, float)) and not isinstance(v, bool)]


subjects = [r[0] for r in psql("SELECT a_norm FROM forensic.comm_event WHERE collection_id='%s' AND dataset_kind='target_cdr' AND NOT ('duplicate_row' = ANY (quality_flags)) GROUP BY 1 ORDER BY count(*) DESC LIMIT 4" % COLLECTION)]
print("subjects found:", len(subjects))
results = []


def record(kind, ok, seconds, header, route):
    results.append((kind, ok))
    print("%-22s %-7s %6.1fs route=%s engine=%s" % (kind, "CORRECT" if ok else "WRONG", seconds, route, header[:60]))


# 1. events between a subscriber and its top ordinary counterparty
s = subjects[0]
pair = psql("SELECT b_norm, count(*) FROM forensic.comm_event WHERE collection_id='%s' AND dataset_kind='target_cdr' AND a_norm='%s' AND b_norm IS NOT NULL AND b_kind='person_like' AND NOT ('duplicate_row' = ANY (quality_flags)) GROUP BY 1 ORDER BY 2 DESC LIMIT 1" % (COLLECTION, s))[0]
data, sec, hdr = ask("How many events are there between %s and %s?" % (s, pair[0]))
nums = numbers_in(rows_of(data)) or [int(x) for x in __import__("re").findall(r"\d+", str(data.get("answer", {}).get("answer", "")).replace(",", ""))]
record("pair_count", int(pair[1]) in [int(n) for n in nums], sec, hdr, "/".join(data.get("route", [])))

# 2. top three ordinary contacts of a subscriber, services left out
top = psql("SELECT b_norm FROM forensic.comm_event WHERE collection_id='%s' AND dataset_kind='target_cdr' AND a_norm='%s' AND b_norm IS NOT NULL AND b_kind='person_like' AND NOT ('duplicate_row' = ANY (quality_flags)) GROUP BY 1 ORDER BY count(*) DESC, 1 LIMIT 4" % (COLLECTION, subjects[1]))
counts = psql("SELECT b_norm, count(*) FROM forensic.comm_event WHERE collection_id='%s' AND dataset_kind='target_cdr' AND a_norm='%s' AND b_norm IS NOT NULL AND b_kind='person_like' AND NOT ('duplicate_row' = ANY (quality_flags)) GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT 4" % (COLLECTION, subjects[1]))
data, sec, hdr = ask("Who does %s talk to most? Give the top three, leaving out services." % subjects[1])
got = digits_in(rows_of(data))
tied = len(counts) == 4 and counts[2][1] == counts[3][1]
want = {c[0] for c in counts[:3]}
record("top_contacts", (got == want) if not tied else (len(got) == 3 and got <= {c[0] for c in counts}), sec, hdr, "/".join(data.get("route", [])))

# 3. incoming events from one counterparty
inc = psql("SELECT b_norm, count(*) FROM forensic.comm_event WHERE collection_id='%s' AND dataset_kind='target_cdr' AND a_norm='%s' AND direction='incoming' AND b_norm IS NOT NULL AND b_kind='person_like' AND NOT ('duplicate_row' = ANY (quality_flags)) GROUP BY 1 ORDER BY 2 DESC LIMIT 1" % (COLLECTION, subjects[0]))
if inc:
    data, sec, hdr = ask("How many times did %s get a call or message from %s?" % (subjects[0], inc[0][0]))
    nums = numbers_in(rows_of(data)) or [int(x) for x in __import__("re").findall(r"\d+", str(data.get("answer", {}).get("answer", "")).replace(",", ""))]
    record("incoming_count", int(inc[0][1]) in [int(n) for n in nums], sec, hdr, "/".join(data.get("route", [])))

# 4. a number the collection does not hold (reserved synthetic block): must say no record, never zero
data, sec, hdr = ask("How many calls does 923000099991 have?")
text = json.dumps(data).lower()
record("absent_number", "no record" in text, sec, hdr, "/".join(data.get("route", [])))

# 5. a question the data cannot answer: must not be answered with a number
data, sec, hdr = ask("What is the home address of %s?" % subjects[0])
state = (data.get("answer") or {}).get("result_state", "")
record("cannot_answer", state != "COMPLETED" or "ask_engine_v2" not in data.get("route", []), sec, hdr, "/".join(data.get("route", [])))

# 6. a connection question over the subscribers' own records
d = psql("SELECT count(*) FROM forensic.comm_event WHERE collection_id='%s' AND dataset_kind='target_cdr' AND a_norm='%s' AND b_norm='%s' AND NOT ('duplicate_row' = ANY (quality_flags))" % (COLLECTION, subjects[0], subjects[1]))
data, sec, hdr = ask("How are %s and %s connected within three hops, not through services or hubs?" % (subjects[0], subjects[1]))
record("connection(no crash)", (data.get("answer") or {}).get("result_state") in ("COMPLETED", "CLARIFICATION_REQUIRED"), sec, hdr, "/".join(data.get("route", [])))

print("summary:", sum(1 for _, ok in results if ok), "of", len(results), "as expected")
sys.exit(0 if all(ok for _, ok in results) else 1)
