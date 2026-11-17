#!/usr/bin/env python3
"""Product-level acceptance runner: reads fresh images through the worker's own OcrRouter (built by OcrRouter.from_environment(), the code and the environment the worker uses) and writes one JSON line
per image: the text of the observations in their stored reading order, the route taken, the probe share, seconds, and the truth (synthetic or public, so the file stays in the data folder).

Arm `today` runs with FORENSIC_OCR_EASYOCR_ENABLED=false (the router as deployed); arm `easyocr` with it true. Each arm is its own process; scoring is done separately by accept_router_score.py.

    docker run --rm --network none --memory 3500m -v <repo>\models\media:/models/media:ro -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro --env-file <router env> \
        -e FORENSIC_OCR_EASYOCR_ENABLED=true --entrypoint python nexusai-forensic-records-worker:<overlay tag> /repo/tools/media_data/accept_router_runner.py --arm easyocr --out /data/results/ocr/accept-easyocr.jsonl
"""
import argparse
import hashlib
import json
import os
import random
import sys
import time
from pathlib import Path

sys.path.insert(0, "/app")
DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
SETS = {  # set -> (folder, row filter)
    "en": ("ocr_en_accept", lambda r: True),
    "ur_pages": ("ocr_ur_accept", lambda r: r["kind"] == "page"),
    "ur_lines": ("ocr_ur_accept", lambda r: r["kind"] == "line"),
    "mixed": ("ocr_mixed_accept", lambda r: True),
    "unb": ("ocr_unb_accept", lambda r: True),
}
DEFAULT_COUNTS = {"en": 48, "ur_pages": 40, "ur_lines": 60, "mixed": 120, "unb": 30}


def chosen_rows(name, count):
    folder, keep = SETS[name]
    rows = [json.loads(line) for line in open(Path(DATA) / "generated" / folder / "truth.jsonl", encoding="utf-8")]
    rows = sorted((r for r in rows if keep(r)), key=lambda r: hashlib.sha256(r["id"].encode()).hexdigest())  # a fixed pseudo-random order by id hash, the same for both arms
    return folder, rows[:count]


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--arm", required=True, choices=("today", "easyocr"))
    ap.add_argument("--out", required=True)
    ap.add_argument("--sets", nargs="+", default=list(SETS))
    ap.add_argument("--count", nargs="*", default=[], help="name=N overrides of the default image counts")
    args = ap.parse_args(argv)
    counts = {**DEFAULT_COUNTS, **{k: int(v) for k, v in (x.split("=") for x in args.count)}}
    os.environ["FORENSIC_OCR_EASYOCR_ENABLED"] = "true" if args.arm == "easyocr" else "false"
    import ocr_router
    started = time.time()
    router = ocr_router.OcrRouter.from_environment()
    print(f"router built in {time.time() - started:.1f} s; easyocr configured: {router.easyocr is not None}; urdu share threshold {router.urdu_share_threshold}", flush=True)
    done = {json.loads(line)["id"] for line in open(args.out, encoding="utf-8")} if os.path.exists(args.out) else set()
    for name in args.sets:
        folder, rows = chosen_rows(name, counts[name])
        for row in rows:
            key = f"{name}:{row['id']}"
            if key in done:
                continue
            path = Path(DATA) / "generated" / folder / row["file"]
            t0 = time.time()
            error = ""
            try:
                result = router.process_image(path, evidence_id="e", version_id="v", source_sha256="0" * 64, source_file=path.name)
                observations = sorted(result.observations, key=lambda o: o.payload.get("reading_order", 0))
                text = "\n".join(o.payload["raw_text"] for o in observations)
                route = result.metadata.get("ocr_route", {})
            except Exception as exc:  # recorded as a failed row, never hidden
                text, route, error = "", {}, f"{type(exc).__name__}: {str(exc)[:200]}"
            record = {"id": key, "set": name, "row": row["id"], "group": f"{row.get('kind')}-{row.get('style')}-{row.get('variant')}", "arm": args.arm, "seconds": round(time.time() - t0, 2),
                      "engine": route.get("engine_used"), "tried": route.get("engines_tried"), "share": route.get("arabic_script_share"), "script": route.get("script_detected"), "error": error,
                      "hyp": text, "truth": row["text"], "identifiers": row.get("identifiers", [])}
            with open(args.out, "a", encoding="utf-8", newline="\n") as handle:
                handle.write(json.dumps(record, ensure_ascii=False) + "\n")
            print(f"{key:40} {record['seconds']:6.1f} s  {record['engine']}  {error}", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
