#!/usr/bin/env python3
"""Per-face CPU cost with 4 threads (gate R4 and the speed figures of D2): one image at a time, median of 100 calls after 5 warm-ups, and the peak resident memory of the process.

    docker run --rm --network none --memory 2000m -v C:/tmp/nexusai-media-data:/data:ro -v C:/tmp/nexusai-media-data/work-faces:/work -v <repo>:/repo:ro \
        --entrypoint python -e FACE_THREADS=4 nexusai-media-eval-face:20261119 /repo/tools/media_data/face_timing.py

Images: aligned LFW crops for the recognisers; the first 100 WIDER validation images (dev part) for the detectors, at their own size.
"""
import json
import sys
import time
from pathlib import Path

import cv2
import numpy as np

sys.path.insert(0, "/repo/tools/media_data")
import face_common as fc  # noqa: E402
from face_detect_wider import BlazeFace, YuNet  # noqa: E402


def median_ms(function, items, warm=5):
    for item in items[:warm]:
        function(item)
    times = []
    for item in items[:100]:
        t0 = time.perf_counter()
        function(item)
        times.append(time.perf_counter() - t0)
    return round(1000 * float(np.median(times)), 2), round(1000 * float(np.percentile(times, 95)), 2)


def main():
    threads = 4
    cv2.setNumThreads(threads)
    uniq = np.load(Path(fc.WORK) / "bins" / "lfw" / "uniq.npy", mmap_mode="r")
    crops = [np.ascontiguousarray(uniq[i]) for i in range(120)]
    out = {"threads": threads}
    for model in ("sface", "auraface"):
        emb = fc.embedder(model, threads)
        out[model] = {"median_ms_p95_ms_per_face_batch1": median_ms(lambda c: emb.embed([c]), crops), "peak_rss_mb_after": round(fc.rss_mb())}
        print(model, out[model], flush=True)
    records = json.load(open(Path(fc.WORK) / "widerface" / "wider-val.json"))
    images = [cv2.imread(r["file"]) for r in records if fc.partition(r["name"], "01") == "dev"][:105]
    for name, det in (("yunet", YuNet()), ("blazeface", BlazeFace(threads))):
        out[name] = {"median_ms_p95_ms_per_image_own_size": median_ms(lambda im: det.detect(im), images), "mean_pixels": int(np.mean([im.shape[0] * im.shape[1] for im in images])), "peak_rss_mb_after": round(fc.rss_mb())}
        print(name, out[name], flush=True)
    Path(fc.WORK, "results").mkdir(parents=True, exist_ok=True)
    json.dump(out, open(Path(fc.WORK) / "results" / "timing-4threads.json", "w"), indent=1)


if __name__ == "__main__":
    main()
