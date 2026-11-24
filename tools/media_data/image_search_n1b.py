#!/usr/bin/env python3
"""M1 gate N1b (amendment A5 of docs/data/image-search-m1-preregistration-20261120.md): the hash tier at the fixed operating point of 56 bits, on photographs that took no part in any decision.

    docker run --rm --network none --memory 900m -v C:/tmp/nexusai-media-data:/data:ro -v C:/tmp/nexusai-media-data/work-faces:/work -v <repo>:/repo:ro \
        --entrypoint python nexusai-media-eval-face:20261119 /repo/tools/media_data/image_search_n1b.py

Set: the 1,000 SEM1000 photos. Gallery = the originals of the photos with sha256("g" + id) mod 3 != 0; queries = the five family R copies of every photo (re-save, messenger-style recompression, twice, resize, lossless); a query whose original is absent is a negative.
The hash is the product's (load_reduced_image + pdq_bits). One read; the result file refuses a second run unless --reread --reason is given.
"""
import json
import sys
import tempfile
import time
from pathlib import Path

import numpy as np
from PIL import Image

sys.path.insert(0, "/repo")
sys.path.insert(0, "/repo/tools/media_data")
import image_search_m1 as m  # noqa: E402

THRESHOLD_BITS = 56


def main():
    reread = "--reread" in sys.argv
    out_path = m.WORK / "results" / "n1b.json"
    if out_path.exists() and not reread:
        print(f"already read: {out_path}")
        return 3
    s = m._imports()
    sem = json.load(open(m.WORK / "manifest.json"))["sem1000"]
    gallery_ids, gallery_bits = [], []
    queries = []  # (id, transform, bits)
    started = time.time()
    with tempfile.TemporaryDirectory() as folder:
        for n, item in enumerate(sem, start=1):
            source = m.WORK / item["file"]
            bits = s.pdq_bits(s.load_reduced_image(source))
            if m.in_gallery(item["id"]):
                gallery_ids.append(item["id"])
                gallery_bits.append(bits.astype(np.int16))
            image = Image.open(source).convert("RGB")
            for name, (data, ext) in m.make_resave(image).items():
                path = Path(folder) / f"q.{ext}"
                path.write_bytes(data)
                queries.append((item["id"], name, s.pdq_bits(s.load_reduced_image(path)).astype(np.int16)))
            if n % 100 == 0:
                print(f"  {n}/{len(sem)} photos, {time.time() - started:.0f} s", flush=True)
    gallery = np.stack(gallery_bits)
    rows = []
    for photo, name, bits in queries:
        d = (gallery != bits).sum(axis=1)
        best = int(np.argmin(d))
        rows.append((photo, name, m.in_gallery(photo), int(d[best]), gallery_ids[best]))

    def score(select):
        tp = fp = positives = 0
        for photo, name, positive, dist, best in rows:
            if not select(name):
                continue
            positives += 1 if positive else 0
            if dist <= THRESHOLD_BITS:
                if positive and best == photo:
                    tp += 1
                else:
                    fp += 1
        return {"precision": tp / (tp + fp) if tp + fp else 1.0, "recall": tp / positives if positives else 0.0, "tp": tp, "fp": fp, "positives": positives}

    result = {"threshold_bits": THRESHOLD_BITS, "photos": len(sem), "gallery": len(gallery_ids), "queries": len(rows), "read_at": time.strftime("%Y-%m-%d %H:%M:%S"),
              "overall": score(lambda n: True), "per_transform": {t: score(lambda n, t=t: n == t) for t in m.RESAVE}}
    true_d = [d for photo, name, positive, d, best in rows if positive and best == photo]
    unrelated = [d for photo, name, positive, d, best in rows if not positive or best != photo]
    result["true_copy_distance_max"] = max(true_d) if true_d else None
    result["nearest_unrelated_min_among_wrong_or_negative"] = min(unrelated) if unrelated else None
    out_path.parent.mkdir(exist_ok=True)
    json.dump(result, open(out_path, "w"), indent=1)
    print(json.dumps(result, indent=1))


if __name__ == "__main__":
    sys.exit(main() or 0)
