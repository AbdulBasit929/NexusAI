#!/usr/bin/env python3
"""Offline EasyOCR arabic_g1 recognizer evaluation on governed truth crops."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import platform
import resource
import tempfile
import time
import unicodedata
from pathlib import Path
from zipfile import ZipFile


CANDIDATE = "easyocr-arabic-g1-v1.7.2"
REVISION = "c4f3cd7225efd4f85451bd8b4a7646ae9a092420"
MEMBER_MD5 = "993074555550e4e06a6077d55ff0449a"
MEMBER_SIZE = 215400714


def hash_file(path: Path, algorithm: str = "sha256") -> str:
    digest = hashlib.new(algorithm)
    with path.open("rb") as stream:
        while chunk := stream.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def normalize(value: str) -> str:
    return "".join(c for c in unicodedata.normalize("NFKC", value).upper() if c.isalnum())


def extract_verified_weight(archive: Path, destination: Path) -> Path:
    with ZipFile(archive) as zipped:
        members = zipped.infolist()
        if len(members) != 1 or members[0].filename != "arabic.pth" or members[0].file_size != MEMBER_SIZE:
            raise RuntimeError("EasyOCR archive member contract failed")
        output = destination / "arabic.pth"
        digest = hashlib.md5()
        with zipped.open(members[0]) as source, output.open("wb") as target:
            while chunk := source.read(1024 * 1024):
                digest.update(chunk)
                target.write(chunk)
        if digest.hexdigest() != MEMBER_MD5:
            raise RuntimeError("EasyOCR arabic.pth publisher MD5 failed")
        return output


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--fixtures", type=Path, required=True)
    parser.add_argument("--model-archive", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if os.environ.get("NEXUSAI_R8_OFFLINE") != "1":
        parser.error("NEXUSAI_R8_OFFLINE=1 is required")

    import cv2
    import easyocr

    manifest = json.loads(args.fixtures.read_text(encoding="utf-8-sig"))
    with tempfile.TemporaryDirectory() as directory:
        model_root = Path(directory)
        weight = extract_verified_weight(args.model_archive, model_root)
        reader = easyocr.Reader(
            ["ur", "en"], gpu=False, model_storage_directory=str(model_root),
            download_enabled=False, detector=False, recognizer=True, verbose=False,
        )
        predictions = []
        for fixture in manifest["fixtures"]:
            if fixture.get("ocr_evaluation_eligible", True) is not True:
                continue
            crop_name = fixture.get("truth_crop_file")
            if not crop_name:
                predictions.append(
                    {
                        "fixture_id": fixture["fixture_id"], "plate_text_raw": None,
                        "plate_text_normalized": None, "confidence": None,
                        "abstention_state": "no_plate_found", "latency_ms": 0.0,
                    }
                )
                continue
            image = cv2.imread(str(args.fixtures.parent / crop_name), cv2.IMREAD_COLOR)
            if image is None:
                raise RuntimeError(f"could not decode governed crop: {crop_name}")
            height, width = image.shape[:2]
            started = time.perf_counter()
            results = reader.recognize(
                image, horizontal_list=[[0, width, 0, height]], free_list=[],
                decoder="greedy", beamWidth=1, batch_size=1, workers=0,
                detail=1, rotation_info=None, paragraph=False,
            )
            latency_ms = (time.perf_counter() - started) * 1000
            raw = str(results[0][1]).strip() if results else ""
            confidence = float(results[0][2]) if results else None
            normalized = normalize(raw)
            predictions.append(
                {
                    "fixture_id": fixture["fixture_id"],
                    "plate_text_raw": raw or None,
                    "plate_text_normalized": normalized or None,
                    "confidence": confidence,
                    "abstention_state": "candidate" if normalized else "ocr_abstained",
                    "latency_ms": round(latency_ms, 3),
                    "model_name": "arabic_g1",
                    "languages": ["ur", "en"],
                    "preprocessing": "EasyOCR 1.7.2 standard recognizer preprocessing on immutable truth crop",
                    "normalization_steps": ["Unicode NFKC", "uppercase", "retain alphanumeric code points"],
                }
            )
        weight_sha256 = hash_file(weight)

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
        "host": {
            "platform": platform.platform(), "cpu_count": os.cpu_count(),
            "python": platform.python_version(), "easyocr": easyocr.__version__,
        },
        "preprocessing_recipe": "immutable annotated truth crop; EasyOCR 1.7.2 standard arabic_g1 recognizer; ur+en; greedy decode; no character substitution",
        "package_size_mib": round(MEMBER_SIZE / 1048576, 6),
        "peak_process_rss_mib": round(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024, 3),
        "archive_sha256": hash_file(args.model_archive),
        "weight_sha256": weight_sha256,
        "predictions": predictions,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(json.dumps({"status": "pass", "candidate": CANDIDATE, "predictions": len(predictions), "output": str(args.output)}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
