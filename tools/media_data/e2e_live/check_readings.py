import json, subprocess, sys
sql = ("select json_build_object('f', ei.original_filename, 'o', (da.metadata->'observation'->>'reading_order')::int, 't', da.metadata->'observation'->>'raw_text', "
       "'p', da.metadata->'observation'->>'processor', 'r', (da.metadata->'observation'->>'manual_review_required'), 'l', coalesce(jsonb_array_length(da.metadata->'observation'->'latin_reread'),0)) "
       "from forensic.derived_artifacts da join forensic.evidence_items ei on ei.evidence_id=da.evidence_id "
       "where da.collection_id='ocr-easyocr-e2e-test-20261119' and da.artifact_type='forensics.image-ocr-observation/v1'")
out = subprocess.run(["docker", "exec", "nexusai-forensic-postgres-1", "psql", "-U", "localrecall", "-d", "localrecall", "-At", "-c", sql], capture_output=True, text=True, encoding="utf-8").stdout
rows = [json.loads(line) for line in out.splitlines() if line.startswith("{")]
truth = {m["file"]: m for m in json.load(open("C:/tmp/nexusai-g4/ocr-e2e-case/truth.json", encoding="utf-8"))}
by = {}
for r in rows:
    by.setdefault(r["f"], []).append(r)
print("| file | lines | reader | review flagged | Latin boxes re-read | identifiers found |")
print("|---|---|---|---|---|---|")
total_hit = total = 0
for name, m in truth.items():
    lines = sorted(by.get(name, []), key=lambda r: r["o"])
    text = "\n".join(r["t"] for r in lines)
    hit = sum(1 for i in m["identifiers"] if i["value"] in text)
    total_hit += hit
    total += len(m["identifiers"])
    missing = [i["value"] for i in m["identifiers"] if i["value"] not in text]
    readers = sorted({r["p"] for r in lines})
    print(f"| {name} | {len(lines)} | {','.join(readers)} | {sum(1 for r in lines if r['r'] == 'true')}/{len(lines)} | {sum(r['l'] for r in lines)} | {hit}/{len(m['identifiers'])} |")
    if missing:
        print(f"|   missing in {name} | " + "; ".join(missing[:6]) + " | | | | |")
print(f"\nidentifiers found in the stored readings: {total_hit}/{total}")
