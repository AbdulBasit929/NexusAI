#!/usr/bin/env python3
"""Privacy-minimized FastPlateOCR diagnostic over pinned CCPD demo images."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import statistics
import time
from pathlib import Path


REVISION = "02aaea15137c4d2fe662e57d257c6822356e9304"
MODEL_SHA256 = "384bbbd2cea3ef54761d3df70822ef3a349ee1a112aeafddbe0e3ba06bc6e47b"
CONFIG_SHA256 = "0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6"
# Non-authoritative diagnostic regions established by authorized visual review.
# They are applied in memory and never become benchmark ground truth.
REGIONS = {
    "0.jpg": (285, 490, 490, 590),
    "1.jpg": (195, 455, 445, 620),
    "2.jpg": (300, 430, 505, 550),
    "3.jpg": (225, 470, 440, 600),
    "4.jpg": (190, 470, 410, 590),
}


def hash_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        while chunk := stream.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def geometric_mean(values: list[float]) -> float | None:
    if not values:
        return None
    if any(value <= 0 for value in values):
        return 0.0
    return math.exp(sum(math.log(value) for value in values) / len(values))


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--pilot-root", type=Path, required=True)
    parser.add_argument("--contract", type=Path, required=True)
    parser.add_argument("--receipt", type=Path, required=True)
    parser.add_argument("--model", type=Path, required=True)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if os.environ.get("NEXUSAI_R8_OFFLINE") != "1":
        parser.error("NEXUSAI_R8_OFFLINE=1 is required")
    if hash_file(args.model) != MODEL_SHA256 or hash_file(args.config) != CONFIG_SHA256:
        parser.error("model/config integrity failed")

    import numpy as np
    import onnxruntime as ort
    from fast_plate_ocr import LicensePlateRecognizer
    from PIL import Image

    contract = json.loads(args.contract.read_text(encoding="utf-8-sig"))
    receipt = json.loads(args.receipt.read_text(encoding="utf-8-sig"))
    if contract["revision"] != REVISION or contract["acceptance_authority"]:
        parser.error("diagnostic contract authority/revision failed")
    receipt_files = {item["name"]: item for item in receipt["files"]}
    if set(receipt_files) != set(REGIONS):
        parser.error("pilot receipt does not match exact diagnostic region set")

    options = ort.SessionOptions()
    options.intra_op_num_threads = 2
    options.inter_op_num_threads = 1
    options.execution_mode = ort.ExecutionMode.ORT_SEQUENTIAL
    recognizer = LicensePlateRecognizer(
        device="cpu", providers=["CPUExecutionProvider"], sess_options=options,
        onnx_model_path=args.model, plate_config_path=args.config,
    )

    observations = []
    latencies = []
    for name, box in REGIONS.items():
        image_path = args.pilot_root / name
        if hash_file(image_path) != receipt_files[name]["sha256"]:
            raise RuntimeError(f"pilot image SHA-256 mismatch: {name}")
        with Image.open(image_path) as source:
            rgb = source.convert("RGB")
            if box[2] > rgb.width or box[3] > rgb.height:
                raise RuntimeError(f"diagnostic region exceeds image: {name}")
            crop = np.asarray(rgb.crop(box), dtype=np.uint8)
        started = time.perf_counter()
        result = recognizer.run_one(crop, return_confidence=True)
        latency = (time.perf_counter() - started) * 1000
        latencies.append(latency)
        plate = str(result.plate or "").strip()
        probabilities = [float(value) for value in (result.char_probs if result.char_probs is not None else [])]
        confidence = geometric_mean(probabilities[: len(plate)])
        observations.append({
            "image_sha256": receipt_files[name]["sha256"],
            "decoded_nonempty": bool(plate),
            "decoded_length": len(plate),
            "latin_digit_charset_only": bool(plate) and all(char in "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ" for char in plate),
            "confidence_geometric_mean": round(confidence, 8) if confidence is not None else None,
            "minimum_character_probability": round(min(probabilities[: len(plate)]), 8) if plate and probabilities else None,
            "latency_ms": round(latency, 3),
            "recognized_string_retained": False,
            "crop_retained": False,
        })

    output = {
        "contract_version": "nexusai.r8-real-image-diagnostic/v1",
        "generated_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "dataset_id": contract["dataset_id"], "publisher_revision": REVISION,
        "candidate": "fast-plate-ocr-cct-s-v2-global-v1.1.0",
        "hardware_profile_id": "D0-20260813-i7-1260P-docker-cpu",
        "provider": "CPUExecutionProvider", "offline": True,
        "images": len(observations),
        "nonempty_decodes": sum(item["decoded_nonempty"] for item in observations),
        "charset_valid_decodes": sum(item["latin_digit_charset_only"] for item in observations),
        "latency_mean_ms": round(statistics.fmean(latencies), 3),
        "observations": observations,
        "publisher_ground_truth_present": False,
        "accuracy_metrics_permitted": False,
        "acceptance_authority": False,
        "boundary_a_authority": False,
        "tuning_authority": False,
        "recognized_strings_retained": False,
        "crops_retained": False,
        "production_mutation": False,
        "automatic_promotion": False,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(output, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({
        "status": "pass", "images": len(observations),
        "nonempty_decodes": output["nonempty_decodes"],
        "charset_valid_decodes": output["charset_valid_decodes"],
        "accuracy_metrics_permitted": False, "output": str(args.output),
    }, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
