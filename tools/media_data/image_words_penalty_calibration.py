#!/usr/bin/env python3
"""Calibrates the window penalty and the best / also-possible cut-offs of photos-by-words from the database itself, so that no picture is cut out of the data.

For each of the queries of image_words_calibration.py the phrase vector is taken from the search service (docker exec into its container) and, for every picture of the case, the similarity of the whole
picture and the best similarity over its 34 windows are read with pgvector. A picture then scores max(whole, best window - penalty); the ground truth is the COCO caption of the test pictures (as in
image_words_calibration.py). The script prints precision, recall and F1 for each (penalty, ratio) pair and the best pair.

    python tools/media_data/image_words_penalty_calibration.py --collection orion-lahore-reference-20261030 --pack C:/NexusAI-Showcase/orion/image-search-test-pack

Needs docker on the PATH, the stack running, and the secrets file (tenant) readable.
"""
import argparse
import importlib.util
import json
import re
import subprocess
import sys
from pathlib import Path

spec = importlib.util.spec_from_file_location("image_words_calibration", Path(__file__).with_name("image_words_calibration.py"))
cal = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cal)

SIGLIP = "google/siglip2-base-patch16-224"
EMBED = (
    "import json, os, sys, urllib.request\n"
    "r = urllib.request.Request('http://127.0.0.1:8097/embed-text', data=json.dumps({'text': sys.argv[1]}).encode(), method='POST', headers={'X-Image-Search-Key': os.environ['FORENSIC_IMAGE_SEARCH_SERVICE_KEY']})\n"
    "print(json.dumps(json.load(urllib.request.urlopen(r, timeout=300))['vector']))\n"
)


def embed(text):
    done = subprocess.run(["docker", "exec", "nexusai-forensic-image-search-1", "python", "-c", EMBED, text], capture_output=True, text=True)
    if done.returncode != 0:
        sys.exit(done.stderr[-300:])
    return json.loads(done.stdout.strip().splitlines()[-1])


def scores(tenant, collection, vector):
    literal = "[" + ",".join(format(v, ".8g") for v in vector) + "]"
    sql = f"""
select set_config('app.tenant_id', '{tenant}', false) \\g /dev/null
with q as (select '{literal}'::vector as v),
whole as (select e.evidence_id, e.original_filename as name, 1 - (ce.embedding <=> q.v) as s
          from forensic.content_embedding ce join forensic.evidence_items e on e.tenant_id = ce.tenant_id and e.collection_id = ce.collection_id and e.evidence_id = ce.evidence_id, q
          where ce.tenant_id = '{tenant}' and ce.collection_id = '{collection}' and ce.model_id = '{SIGLIP}' and ce.chunk_ref = 'v:' || e.current_version_id::text || ':full'),
win as (select ce.evidence_id, max(1 - (ce.embedding <=> q.v)) as s
        from forensic.content_embedding ce join forensic.evidence_items e on e.tenant_id = ce.tenant_id and e.collection_id = ce.collection_id and e.evidence_id = ce.evidence_id, q
        where ce.tenant_id = '{tenant}' and ce.collection_id = '{collection}' and ce.model_id = '{SIGLIP}#tile' and ce.chunk_ref like 'v:' || e.current_version_id::text || ':tile:%'
        group by ce.evidence_id)
select whole.name, whole.s, coalesce(win.s, whole.s) from whole left join win on win.evidence_id = whole.evidence_id order by whole.name;
"""
    done = subprocess.run(["docker", "exec", "-i", "nexusai-forensic-postgres-1", "psql", "-U", "localrecall", "-d", "localrecall", "-At", "-F", "|"], input=sql, capture_output=True, text=True)
    if done.returncode != 0:
        sys.exit(done.stderr[-300:])
    out = []
    for line in done.stdout.strip().splitlines():
        parts = line.split("|")
        if len(parts) == 3:
            try:
                out.append((parts[0], float(parts[1]), float(parts[2])))
            except ValueError:
                pass
    return out


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--collection", required=True)
    parser.add_argument("--pack", required=True)
    args = parser.parse_args()
    sys.stdout.reconfigure(encoding="utf-8")
    tenant = cal.secret("FORENSIC_RECORDS_TENANT_ID") or "default"
    manifest = json.load(open(Path(args.pack) / "manifest.json"))
    captions = {Path(f["file"]).name: f["caption"].lower() for f in manifest["files"] if f["file"].startswith("in-case/") and f.get("caption")}
    for name in list(captions):
        for suffix in ("-resaved", "-cropped"):
            captions[name.replace(".jpg", suffix + ".jpg")] = captions[name]
    data = []
    for query, pattern in cal.QUERIES.items():
        rows = scores(tenant, args.collection, embed(query))
        labelled = [(name, whole, win, bool(re.search(pattern, captions[name]))) for name, whole, win in rows if name in captions]
        data.append((query, labelled, sum(1 for *_, ok in labelled if ok)))
        print(f"{query:<30} pictures={len(rows)} labelled={len(labelled)} relevant={data[-1][2]}", flush=True)

    def score(penalty, best_ratio):
        tp = fp = fn = 0
        for _, labelled, relevant in data:
            eff = [(n, max(w, ww - penalty), ok) for n, w, ww, ok in labelled]
            top = max(s for _, s, _ in eff)
            kept = [x for x in eff if x[1] >= best_ratio * top]
            tp += sum(1 for _, _, ok in kept if ok)
            fp += sum(1 for _, _, ok in kept if not ok)
            fn += relevant - sum(1 for _, _, ok in kept if ok)
        p = tp / (tp + fp) if tp + fp else 0
        r = tp / (tp + fn) if tp + fn else 0
        return p, r, (2 * p * r / (p + r) if p + r else 0), tp + fp

    print("\npenalty  ratio   precision recall  F1    kept")
    best = None
    for penalty in (0.0, 0.005, 0.01, 0.015, 0.02, 0.025, 0.03, 0.04, 0.06, 1.0):
        for ratio in (0.75, 0.80, 0.85, 0.88, 0.90, 0.92):
            p, r, f1, kept = score(penalty, ratio)
            if best is None or f1 > best[0]:
                best = (f1, penalty, ratio, p, r)
            if ratio in (0.80, 0.88, 0.92):
                print(f"{penalty:<8} {ratio:<7} {p:9.2f} {r:6.2f} {f1:5.2f} {kept:5d}")
    print(f"\npenalty 1.0 means windows never win (whole-picture score only; the first version).")
    print(f"best: penalty {best[1]}, best-match ratio {best[2]} (F1 {best[0]:.2f}, precision {best[3]:.2f}, recall {best[4]:.2f})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
