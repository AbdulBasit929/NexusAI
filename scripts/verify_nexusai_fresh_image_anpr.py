#!/usr/bin/env python3
"""Verify fresh image ANPR on explicit non-holdout local files without writes."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import sys
import time
from pathlib import Path


REPOSITORY = Path(__file__).resolve().parents[1]
if str(REPOSITORY) not in sys.path:
    sys.path.insert(0, str(REPOSITORY))

from ingestion.forensic_records.media_pipeline import FastALPRImageProcessor  # noqa: E402
from ingestion.forensic_records.processor_readiness import (  # noqa: E402
    ModelAssetRequirement,
    assess_processor_readiness,
)


EXPECTED_HASHES = {
    "detector": "888397b96d761c89db40bc9c305838e8652660f5e282c2cadebbe8d2951a77a8",
    "ocr": "8031afb5fdc6b4d80462c9d542f1284ebd2cfddf5dbacd62609848d7e2855f44",
    "ocr_config": "0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6",
}


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", type=Path, action="append", required=True)
    parser.add_argument("--negative", type=Path, required=True)
    parser.add_argument("--detector", type=Path, required=True)
    parser.add_argument("--ocr", type=Path, required=True)
    parser.add_argument("--ocr-config", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    assets = [
        ModelAssetRequirement("detector", args.detector, EXPECTED_HASHES["detector"]),
        ModelAssetRequirement("ocr", args.ocr, EXPECTED_HASHES["ocr"]),
        ModelAssetRequirement("ocr_config", args.ocr_config, EXPECTED_HASHES["ocr_config"]),
    ]
    readiness = assess_processor_readiness(
        role="image_anpr", processor_id=FastALPRImageProcessor.processor_id,
        code_present=True, role_enabled=True, role_admitted=True, worker_healthy=True,
        resource_policy_satisfied=True, assets=assets,
        backend_modules=("cv2", "fast_alpr", "open_image_models", "onnxruntime"),
    )
    if readiness.state != "READY":
        raise RuntimeError(f"image ANPR readiness failed: {readiness.as_dict()}")

    os.environ.update({
        "FORENSIC_ANPR_DETECTOR_MODEL_PATH": str(args.detector.resolve()),
        "FORENSIC_ANPR_OCR_MODEL_PATH": str(args.ocr.resolve()),
        "FORENSIC_ANPR_OCR_CONFIG_PATH": str(args.ocr_config.resolve()),
        "FORENSIC_ANPR_ONNX_PROVIDERS": "CPUExecutionProvider",
    })
    started = time.perf_counter()
    processor = FastALPRImageProcessor.from_environment()
    rows = []
    for path in [*args.image, args.negative]:
        source_hash = sha256_file(path)
        result = processor.process_image(
            path,
            evidence_id="non-retained-local-verification",
            version_id=source_hash,
            source_sha256=source_hash,
            source_file=path.name,
        )
        rows.append({
            "source_file": path.name,
            "source_sha256": source_hash,
            "negative_control": path == args.negative,
            "readiness": result.readiness,
            "result_state": result.metadata["result_state"],
            "observation_count": len(result.observations),
            "observations": [
                {
                    "contract_version": item.contract_version,
                    "observation_type": item.observation_type,
                    "confidence": item.confidence,
                    "citation_locator": item.citation_locator,
                    "payload": item.payload,
                }
                for item in result.observations
            ],
            "limitations": list(result.limitations),
        })
    missing = assess_processor_readiness(
        role="image_anpr", processor_id=processor.processor_id,
        code_present=True, role_enabled=True, role_admitted=True, worker_healthy=True,
        resource_policy_satisfied=True,
        assets=[ModelAssetRequirement("detector", args.output.parent / "absent-model", EXPECTED_HASHES["detector"])],
    )
    report = {
        "contract_version": "nexusai.nxmmr.fresh-image-anpr-local-verification/v1",
        "scope": "explicit_local_nonretained_development_and_generated_negative",
        "readiness": readiness.as_dict(),
        "model_missing_probe": missing.as_dict(),
        "processor": processor.processor_id,
        "processor_revision": processor.processor_revision,
        "elapsed_seconds": round(time.perf_counter() - started, 6),
        "results": rows,
        "sealed_holdout_scored": False,
        "runtime_mutated": False,
        "retained_state_mutated": False,
        "activity_mutated": False,
        "database_or_volumes_mutated": False,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({
        "readiness": readiness.state,
        "model_missing_state": missing.state,
        "results": [{key: row[key] for key in ("source_file", "result_state", "observation_count")} for row in rows],
        "elapsed_seconds": report["elapsed_seconds"],
        "sealed_holdout_scored": False,
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
