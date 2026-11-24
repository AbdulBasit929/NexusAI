#!/usr/bin/env python3
"""Calibrates the relevance cut-off of photos-by-words against ground truth the pack already has: every in-case picture of the test pack has a COCO caption written by a person, so "a picture is
relevant to the query 'a laptop'" can be read from whether its caption names a laptop. Pictures without a caption (the real plate photographs, other case files) are not counted either way.

    python tools/media_data/image_words_calibration.py --collection orion-lahore-reference-20261030 --pack C:/NexusAI-Showcase/orion/image-search-test-pack

For each query the live words search returns the ranked list with similarities; several cut-off rules are scored by precision, recall and F1 over all queries together, and the best rule is printed.
Caption mentions miss pictures that contain the object without naming it, so precision is a lower bound; recall is not affected.
"""
import argparse
import json
import re
import sys
import urllib.parse
import urllib.request
from pathlib import Path

SECRETS = r"C:\Users\sheik\Workspace\Office\Projects\NexusAI\.env.forensic-runtime.local"
API = "http://127.0.0.1:8091"

QUERIES = {
    "an airplane": r"\b(plane|planes|airplane|airplanes|jet|jets|airliner)\b",
    "a laptop": r"\b(laptop|laptops|computer|computers)\b",
    "a dog": r"\b(dog|dogs|puppy)\b",
    "a bus": r"\b(bus|buses)\b",
    "a truck": r"\b(truck|trucks)\b",
    "a motorcycle": r"\b(motorcycle|motorcycles|motorbike|scooter)\b",
    "a backpack": r"\b(backpack|backpacks)\b",
    "a clock": r"\b(clock|clocks)\b",
    "a boat": r"\b(boat|boats|ship|ships)\b",
    "a kitchen": r"\b(kitchen)\b",
    "a cat": r"\b(cat|cats|kitten)\b",
    "a train": r"\b(train|trains)\b",
    "a bicycle": r"\b(bicycle|bicycles|bike|bikes)\b",
    "people eating food": r"\b(eating|pizza|sandwich|food|meal)\b",
    "a cell phone": r"\b(phone|phones|cellphone)\b",
    "a person holding an umbrella": r"\b(umbrella|umbrellas)\b",
}


def secret(name):
    for line in open(SECRETS, encoding="utf-8"):
        if line.startswith(name + "="):
            return line.split("=", 1)[1].strip()
    sys.exit(f"{name} is not in the secrets file")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--collection", required=True)
    parser.add_argument("--pack", required=True)
    args = parser.parse_args()
    sys.stdout.reconfigure(encoding="utf-8")
    tenant = secret("FORENSIC_RECORDS_TENANT_ID") or "default"
    headers = {"Authorization": "Bearer " + secret("FORENSIC_RECORDS_API_KEY"), "X-Forensic-Tenant-ID": tenant, "X-Forensic-Collection-ID": args.collection,
               "X-Forensic-Actor-ID": "words-calibration", "X-Forensic-Subject-ID": "words-calibration", "X-Forensic-Actor-Role": "user"}
    manifest = json.load(open(Path(args.pack) / "manifest.json"))
    captions = {Path(f["file"]).name: f["caption"].lower() for f in manifest["files"] if f["file"].startswith("in-case/") and f.get("caption")}
    # the copies carry the caption of their source
    for name in list(captions):
        for suffix in ("-resaved", "-cropped"):
            captions[name.replace(".jpg", suffix + ".jpg")] = captions[name]
    runs = []
    for query, pattern in QUERIES.items():
        qs = urllib.parse.urlencode({"tenant_id": tenant, "collection_id": args.collection, "kind": "by_words", "q": query, "top_k": 100})
        with urllib.request.urlopen(urllib.request.Request(f"{API}/images/search?{qs}", headers=headers), timeout=300) as response:
            body = json.loads(response.read().decode("utf-8"))
        rows = [(h["original_filename"], h["similarity"]) for h in body["results"] if h.get("similarity") is not None]
        labelled = [(name, sim, bool(re.search(pattern, captions[name]))) for name, sim in rows if name in captions]
        relevant = sum(1 for _, _, ok in labelled if ok)
        runs.append((query, labelled, relevant))
        top = rows[0][1] if rows else 0
        found = [f"{name.replace('imgsearch-', '')}:{sim:.3f}" for name, sim, ok in labelled if ok][:4]
        print(f"{query:<30} relevant={relevant:<3} top={top:.3f}  first relevant: {', '.join(found)}")

    def score(keep):
        tp = fp = fn = 0
        for _, labelled, relevant in runs:
            if not labelled:
                continue
            top = max(sim for _, sim, _ in labelled)
            kept = [(n, s, ok) for n, s, ok in labelled if keep(s, top)]
            tp += sum(1 for _, _, ok in kept if ok)
            fp += sum(1 for _, _, ok in kept if not ok)
            fn += relevant - sum(1 for _, _, ok in kept if ok)
        precision = tp / (tp + fp) if tp + fp else 0
        recall = tp / (tp + fn) if tp + fn else 0
        f1 = 2 * precision * recall / (precision + recall) if precision + recall else 0
        return precision, recall, f1, tp, fp, fn

    print("\nrule                                    precision  recall   F1    kept(tp+fp)")
    best = None
    for ratio in (0.70, 0.75, 0.80, 0.85, 0.88, 0.90, 0.92, 0.94, 0.96):
        for floor in (0.0, 0.06, 0.08, 0.10, 0.12):
            p, r, f1, tp, fp, fn = score(lambda s, top, ratio=ratio, floor=floor: s >= ratio * top and s >= floor)
            if best is None or f1 > best[0]:
                best = (f1, ratio, floor, p, r)
            if floor in (0.0, 0.08) or (ratio, floor) == (best[1], best[2]):
                print(f"ratio >= {ratio:.2f} x top, floor {floor:.2f}           {p:7.2f}  {r:7.2f}  {f1:5.2f}   {tp + fp}")
    for floor in (0.06, 0.08, 0.09, 0.10, 0.11, 0.12, 0.14):
        p, r, f1, tp, fp, fn = score(lambda s, top, floor=floor: s >= floor)
        print(f"absolute similarity >= {floor:.2f}                {p:7.2f}  {r:7.2f}  {f1:5.2f}   {tp + fp}")
    print(f"\nbest ratio rule: similarity >= {best[1]} x the top one and >= {best[2]} (F1 {best[0]:.2f}, precision {best[3]:.2f}, recall {best[4]:.2f})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
