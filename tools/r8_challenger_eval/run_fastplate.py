#!/usr/bin/env python3
"""Offline FastPlateOCR Latin evaluation on governed immutable truth crops."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import platform
import statistics
import time
import unicodedata
from pathlib import Path

try:
    import resource
except ImportError:  # pragma: no cover - evaluator runs in Linux; supports host unit tests.
    resource = None


CANDIDATE = "fast-plate-ocr-cct-s-v2-global-v1.1.0"
REVISION = "9ce7a5b64a939aa421c243b331d42e6bc25ffd44"
MODEL_SHA256 = "384bbbd2cea3ef54761d3df70822ef3a349ee1a112aeafddbe0e3ba06bc6e47b"
CONFIG_SHA256 = "0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6"


def hash_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        while chunk := stream.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def normalize(value: str) -> str:
    return "".join(c for c in unicodedata.normalize("NFKC", value).upper() if c.isalnum())


def geometric_mean(values: list[float]) -> float | None:
    valid = [float(value) for value in values if 0.0 <= float(value) <= 1.0]
    if not valid:
        return None
    if any(value == 0.0 for value in valid):
        return 0.0
    return math.exp(sum(math.log(value) for value in valid) / len(valid))


def percentile(values: list[float], quantile: float) -> float | None:
    if not values:
        return None
    ordered = sorted(values)
    index = min(len(ordered) - 1, max(0, math.ceil(quantile * len(ordered)) - 1))
    return round(ordered[index], 3)


def peak_rss_mib() -> float | None:
    if resource is None:
        return None
    return round(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024, 3)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--fixtures", type=Path, required=True)
    parser.add_argument("--model", type=Path, required=True)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--threads", type=int, default=2)
    args = parser.parse_args()
    if os.environ.get("NEXUSAI_R8_OFFLINE") != "1":
        parser.error("NEXUSAI_R8_OFFLINE=1 is required")
    if args.threads < 1 or args.threads > 4:
        parser.error("threads must be between 1 and 4 for the D0 evaluator")
    if hash_file(args.model) != MODEL_SHA256 or hash_file(args.config) != CONFIG_SHA256:
        parser.error("pinned FastPlateOCR model/config integrity failed")

    import fast_plate_ocr
    import onnxruntime as ort
    from fast_plate_ocr import LicensePlateRecognizer

    manifest = json.loads(args.fixtures.read_text(encoding="utf-8-sig"))
    session_options = ort.SessionOptions()
    session_options.intra_op_num_threads = args.threads
    session_options.inter_op_num_threads = 1
    session_options.execution_mode = ort.ExecutionMode.ORT_SEQUENTIAL
    recognizer = LicensePlateRecognizer(
        device="cpu",
        providers=["CPUExecutionProvider"],
        sess_options=session_options,
        onnx_model_path=args.model,
        plate_config_path=args.config,
    )

    predictions: list[dict] = []
    latencies: list[float] = []
    for fixture in manifest["fixtures"]:
        if fixture.get("ocr_evaluation_eligible", True) is not True:
            continue
        crop_name = fixture.get("truth_crop_file")
        if not crop_name:
            predictions.append({
                "fixture_id": fixture["fixture_id"], "plate_text_raw": None,
                "plate_text_normalized": None, "confidence": None,
                "character_probabilities": [], "abstention_state": "no_plate_found",
                "latency_ms": 0.0,
            })
            continue
        crop = args.fixtures.parent / crop_name
        if not crop.is_file():
            raise RuntimeError(f"truth crop is missing: {crop_name}")
        started = time.perf_counter()
        prediction = recognizer.run_one(str(crop), return_confidence=True)
        latency_ms = (time.perf_counter() - started) * 1000
        latencies.append(latency_ms)
        raw = str(prediction.plate or "").strip()
        normalized = normalize(raw)
        raw_probabilities = prediction.char_probs if prediction.char_probs is not None else []
        probabilities = [round(float(value), 8) for value in raw_probabilities]
        confidence = geometric_mean(probabilities[: len(raw)])
        predictions.append({
            "fixture_id": fixture["fixture_id"],
            "plate_text_raw": raw or None,
            "plate_text_normalized": normalized or None,
            "confidence": round(confidence, 8) if confidence is not None else None,
            "confidence_aggregation": "geometric_mean_of_returned_non_padding_character_probabilities",
            "character_probabilities": probabilities,
            "minimum_character_probability": min(probabilities[: len(raw)], default=None),
            "region_raw": getattr(prediction, "region", None),
            "region_probability": (
                float(prediction.region_prob) if getattr(prediction, "region_prob", None) is not None else None
            ),
            "abstention_state": "candidate" if normalized else "ocr_abstained",
            "latency_ms": round(latency_ms, 3),
            "model_name": "cct-s-v2-global-model",
            "preprocessing": "publisher config: RGB uint8 channels-last, linear resize 64x128, no aspect-ratio preservation",
            "normalization_steps": ["Unicode NFKC", "uppercase", "retain alphanumeric code points"],
        })

    package_version = getattr(fast_plate_ocr, "__version__", "unknown")
    payload = {
        "contract_version": "forensics.ocr-evaluation-output/v1",
        "generated_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "candidate": CANDIDATE,
        "candidate_id": CANDIDATE,
        "candidate_revision": REVISION,
        "evaluation_role": "ocr_only_annotated_crop",
        "fixture_manifest_sha256": hash_file(args.fixtures),
        "source_tiers": sorted({str(item.get("tier", "unknown")) for item in manifest["fixtures"]}),
        "offline": True,
        "hardware_profile_id": "D0-20260813-i7-1260P-docker-cpu",
        "execution_profile": {
            "provider": "CPUExecutionProvider", "precision": "model_native",
            "device": "Docker Desktop Linux CPU", "intra_op_threads": args.threads,
            "inter_op_threads": 1, "batch_size": 1, "warmup_iterations": 0,
            "timed_samples": len(latencies), "network": "none",
        },
        "host": {
            "platform": platform.platform(), "cpu_count": os.cpu_count(),
            "python": platform.python_version(), "onnxruntime": ort.__version__,
            "fast_plate_ocr": package_version,
        },
        "latency_ms": {
            "mean": round(statistics.fmean(latencies), 3) if latencies else None,
            "p50": percentile(latencies, 0.50), "p95": percentile(latencies, 0.95),
            "p99": percentile(latencies, 0.99),
        },
        "preprocessing_recipe": "immutable annotated truth crop; publisher plate config; no enhancement, script conversion or character substitution",
        "package_size_mib": round((args.model.stat().st_size + args.config.stat().st_size) / 1048576, 6),
        "peak_process_rss_mib": peak_rss_mib(),
        "model_sha256": MODEL_SHA256,
        "config_sha256": CONFIG_SHA256,
        "predictions": predictions,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(json.dumps({"status": "pass", "candidate": CANDIDATE, "predictions": len(predictions), "output": str(args.output)}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
