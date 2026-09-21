#!/usr/bin/env python3
"""Offline OMZ 0106 plate-detector evaluator for governed R8 packs."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import platform
import time
from pathlib import Path
from typing import Any


CANDIDATE = "omz-vehicle-license-plate-detection-barrier-0106-fp32"
REVISION = "af810de42bcf460a18610481f639dcdc86249e69"
ROLE = "plate_localization_detector"


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        while chunk := stream.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def detections_to_regions(
    detections: Any, width: int, height: int, threshold: float
) -> list[dict[str, Any]]:
    regions: list[dict[str, Any]] = []
    for detection in detections:
        image_id, label, confidence, x_min, y_min, x_max, y_max = map(float, detection)
        if image_id < 0 or int(label) != 2 or confidence < threshold:
            continue
        left = max(0, min(width - 1, round(x_min * width)))
        top = max(0, min(height - 1, round(y_min * height)))
        right = max(left + 1, min(width, round(x_max * width)))
        bottom = max(top + 1, min(height, round(y_max * height)))
        regions.append(
            {
                "bounds": {"x": left, "y": top, "width": right - left, "height": bottom - top},
                "confidence": confidence,
            }
        )
    return regions


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--fixtures", type=Path, required=True)
    parser.add_argument("--model", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--confidence-threshold", type=float, default=0.5)
    args = parser.parse_args()
    if os.environ.get("NEXUSAI_R8_OFFLINE") != "1":
        parser.error("NEXUSAI_R8_OFFLINE=1 is required")
    if not 0 < args.confidence_threshold < 1:
        parser.error("confidence threshold must be between zero and one")

    import numpy as np
    import openvino as ov
    import resource
    from PIL import Image

    manifest = json.loads(args.fixtures.read_text(encoding="utf-8-sig"))
    core = ov.Core()
    compiled = core.compile_model(core.read_model(args.model), "CPU")
    input_port = compiled.input(0)
    output_port = compiled.output(0)
    predictions: list[dict[str, Any]] = []
    for fixture in manifest["fixtures"]:
        if fixture.get("detector_evaluation_eligible", True) is not True:
            continue
        source = args.fixtures.parent / fixture["source_file"]
        with Image.open(source) as image:
            rgb = image.convert("RGB")
            width, height = rgb.size
            resized = rgb.resize((300, 300), Image.Resampling.BILINEAR)
            bgr = np.asarray(resized, dtype=np.uint8)[:, :, ::-1].copy()[None, ...]
        started = time.perf_counter()
        result = compiled([bgr])[output_port]
        latency_ms = (time.perf_counter() - started) * 1000
        detections = result.reshape(-1, 7)
        regions = detections_to_regions(detections, width, height, args.confidence_threshold)
        predictions.append(
            {
                "fixture_id": fixture["fixture_id"],
                "plate_regions_original_pixels": regions,
                "confidence": max((item["confidence"] for item in regions), default=None),
                "abstention_state": "candidate" if regions else "no_plate_found",
                "latency_ms": round(latency_ms, 3),
                "model_name": CANDIDATE,
                "confidence_threshold": args.confidence_threshold,
                "coordinate_transform": "normalized 300x300 model output mapped to immutable original pixels",
            }
        )

    package_bytes = args.model.stat().st_size + args.model.with_suffix(".bin").stat().st_size
    payload = {
        "contract_version": "forensics.detector-evaluation-output/v1",
        "generated_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "candidate": CANDIDATE,
        "candidate_id": CANDIDATE,
        "candidate_revision": REVISION,
        "evaluation_role": ROLE,
        "fixture_manifest_sha256": sha256(args.fixtures),
        "source_tiers": sorted({str(item.get("tier", "unknown")) for item in manifest["fixtures"]}),
        "offline": True,
        "host": {
            "platform": platform.platform(),
            "cpu_count": os.cpu_count(),
            "python": platform.python_version(),
            "openvino": ov.__version__,
        },
        "preprocessing_recipe": "Pillow bilinear resize to 300x300 RGB, deterministic RGB-to-BGR, uint8 NHWC; class 2 only; fixed confidence threshold 0.50; normalized boxes mapped to original pixels",
        "package_size_mib": round(package_bytes / 1048576, 6),
        "peak_process_rss_mib": round(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024, 3),
        "model_sha256": sha256(args.model),
        "weights_sha256": sha256(args.model.with_suffix(".bin")),
        "predictions": predictions,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(json.dumps({"status": "pass", "candidate": CANDIDATE, "predictions": len(predictions), "output": str(args.output)}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
