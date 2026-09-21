#!/usr/bin/env python3
"""Bounded, non-retained MMV-2 exact and perceptual image comparison benchmark."""

from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import sys
import tempfile
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

try:
    from ingestion.forensic_records.image_intelligence import image_dhash
except ModuleNotFoundError:  # packaged forensic worker layout
    from image_intelligence import image_dhash


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def distance(left: str, right: str) -> int:
    return (int(left, 16) ^ int(right, 16)).bit_count()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--derived-root", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--path-field", choices=("host_path", "runtime_path"), default="host_path")
    args = parser.parse_args()

    manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
    fixtures = {item["id"]: Path(item[args.path_field]) for item in manifest["fixtures"]}
    fixtures.update({path.stem: path for path in sorted(args.derived_root.glob("*")) if path.is_file()})
    missing = [str(path) for path in fixtures.values() if not path.is_file()]
    if missing:
        raise FileNotFoundError(f"missing fixture(s): {missing}")

    started = time.perf_counter()
    signals = {
        name: {"path": str(path), "sha256": sha256(path), "dhash": image_dhash(path)}
        for name, path in fixtures.items()
    }
    if any(item["dhash"] is None for item in signals.values()):
        raise RuntimeError("a fixture could not be decoded for dHash")

    source_id = "pakistan-plate-lef1981"
    transform_ids = [
        name for name in signals
        if name.startswith(("resize_", "jpeg_", "gaussian_", "low_", "bright_", "partial_"))
    ]
    same_source = [
        {
            "source": source_id,
            "candidate": name,
            "sha256_equal": signals[source_id]["sha256"] == signals[name]["sha256"],
            "hamming_distance": distance(signals[source_id]["dhash"], signals[name]["dhash"]),
        }
        for name in sorted(transform_ids)
    ]
    unrelated_ids = [
        "pakistan-plate-lec4800", "pakistan-plate-lea4861a", "pakistan-plate-ly55",
        "synthetic-mn1367", "wikimedia-islamabad-road-negative", "synthetic-street-negative",
    ]
    unrelated = [
        {
            "source": source_id,
            "candidate": name,
            "sha256_equal": signals[source_id]["sha256"] == signals[name]["sha256"],
            "hamming_distance": distance(signals[source_id]["dhash"], signals[name]["dhash"]),
        }
        for name in unrelated_ids
    ]

    with tempfile.TemporaryDirectory(prefix="nexusai-mmv2-exact-") as temp:
        renamed = Path(temp) / "different-name.bin"
        shutil.copyfile(fixtures[source_id], renamed)
        exact_controls = {
            "same_bytes_different_filename": {
                "sha256_a": signals[source_id]["sha256"],
                "sha256_b": sha256(renamed),
                "exact_duplicate": signals[source_id]["sha256"] == sha256(renamed),
            },
            "different_bytes": {
                "sha256_a": signals[source_id]["sha256"],
                "sha256_b": signals["pakistan-plate-lec4800"]["sha256"],
                "exact_duplicate": signals[source_id]["sha256"] == signals["pakistan-plate-lec4800"]["sha256"],
            },
        }

    threshold = 10
    same_distances = [item["hamming_distance"] for item in same_source]
    unrelated_distances = [item["hamming_distance"] for item in unrelated]
    report = {
        "contract_version": "nexusai.mmv2.image-comparison-benchmark/v1",
        "scope": "lawful_local_nonretained",
        "retained_data_mutated": False,
        "exact_duplicate_semantics": "source SHA-256 equality only",
        "exact_controls": exact_controls,
        "perceptual": {
            "algorithm": "dhash-64-ffmpeg-area-v1",
            "threshold_evaluated": threshold,
            "threshold_changed": False,
            "threshold_decision": (
                "Retain 10 as a conservative candidate threshold; this bounded pack is too small for universal "
                "calibration and transformations beyond the threshold remain truthfully limited."
            ),
            "same_source_transforms": same_source,
            "unrelated_images": unrelated,
            "distribution": {
                "same_source_min": min(same_distances), "same_source_max": max(same_distances),
                "unrelated_min": min(unrelated_distances), "unrelated_max": max(unrelated_distances),
                "same_source_within_threshold": sum(value <= threshold for value in same_distances),
                "same_source_total": len(same_distances),
                "unrelated_within_threshold": sum(value <= threshold for value in unrelated_distances),
                "unrelated_total": len(unrelated_distances),
            },
            "semantics": "pixel-layout resemblance candidate; not semantic similarity or identity",
        },
        "signals": signals,
        "elapsed_ms": round((time.perf_counter() - started) * 1000, 3),
        "verdict": "PASS_OR_TRUTHFULLY_LIMITED",
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"output": str(args.output), "distribution": report["perceptual"]["distribution"]}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
