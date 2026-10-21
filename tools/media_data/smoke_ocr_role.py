"""Smoke test of the OCR role as the worker builds it: configured_media_processor() with the environment of docker-compose.forensic-records.media.yaml, then three generated pages (English, receipt, Urdu).
Prints routes, counts and the first line of each; synthetic and public images only.

    docker run --rm --network none -v <models/media>:/models/media:ro -v C:/tmp/nexusai-media-data:/data:ro -v <repo>:/repo:ro -e FORENSIC_OCR_ENABLED=true ... nexusai-forensic-records-worker:latest python /repo/tools/media_data/smoke_ocr_role.py
"""
import io
import json
import sys
import time
from pathlib import Path

sys.path.insert(0, "/app")
import media_pipeline

processor = media_pipeline.configured_media_processor()
role = processor.image_processor
print("image processor:", type(role).__name__, getattr(role, "processor_id", ""))
rows = [json.loads(l) for l in open("/data/generated/ocr_en/truth.jsonl", encoding="utf-8")]
english = next(r for r in rows if r["partition"] == "dev" and r["variant"] == "clean")
urdu_rows = [json.loads(l) for l in open("/data/generated/ocr_ur/truth.jsonl", encoding="utf-8")]
urdu = next(r for r in urdu_rows if r["kind"] == "page" and r["partition"] == "dev" and r["style"] == "naskh" and r["variant"] == "clean")
receipt_path = Path("/data/debug/receipt-1.png")  # exported from the public CORD test split with the evaluation image (the worker image has no pyarrow)
for name, path in (("english page", Path("/data/generated/ocr_en") / english["file"]), ("receipt photo", receipt_path), ("urdu page", Path("/data/generated/ocr_ur") / urdu["file"])):
    t0 = time.time()
    result = role.process_image(path, evidence_id="e", version_id="v", source_sha256="0" * 64, source_file=path.name)
    route = result.metadata.get("ocr_route", {})
    first = result.observations[0].payload["raw_text"] if result.observations else ""
    print(f"{name:14} {time.time() - t0:5.1f} s  observations {len(result.observations):3}  route {route.get('engine_used')}  script {route.get('script_detected')}  conf {route.get('first_confidence')}  first line: {first[:60]!r}")
