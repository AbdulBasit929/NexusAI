#!/usr/bin/env python3
"""Offline PARSeq-tiny Latin OCR evaluation on governed truth crops."""

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


CANDIDATE = "parseq-tiny-v1.0.0"
REVISION = "315d19be8ef473a864950ab497a649a69e37c6a4"
SOURCE_SHA256 = "4e905c31909ab86467b7b654d68d89e91aa680e0449a5f94b02231e4914f0569"
WEIGHT_SHA256 = "e7a21b543c98e67414a584c93b1dbb71c26e9463ae4aaa391d54e833660a4711"


def hash_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        while chunk := stream.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def normalize(value: str) -> str:
    return "".join(c for c in unicodedata.normalize("NFKC", value).upper() if c.isalnum())


def safe_extract_source(archive: Path, destination: Path) -> Path:
    with ZipFile(archive) as zipped:
        members = zipped.infolist()
        for member in members:
            parts = Path(member.filename).parts
            if member.filename.startswith(("/", "\\")) or ".." in parts:
                raise RuntimeError(f"unsafe PARSeq source archive member: {member.filename}")
        roots = {Path(member.filename).parts[0] for member in members if Path(member.filename).parts}
        if len(roots) != 1:
            raise RuntimeError("PARSeq source archive must have one root")
        zipped.extractall(destination)
    root = destination / next(iter(roots))
    if not (root / "hubconf.py").is_file() or not (root / "strhub" / "models" / "parseq" / "system.py").is_file():
        raise RuntimeError("PARSeq source contract is incomplete")
    return root


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--fixtures", type=Path, required=True)
    parser.add_argument("--weights", type=Path, required=True)
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if os.environ.get("NEXUSAI_R8_OFFLINE") != "1":
        parser.error("NEXUSAI_R8_OFFLINE=1 is required")
    if hash_file(args.weights) != WEIGHT_SHA256 or hash_file(args.source) != SOURCE_SHA256:
        parser.error("pinned PARSeq source/weight integrity failed")

    import sys
    import torch
    from PIL import Image
    from torchvision import transforms as T

    manifest = json.loads(args.fixtures.read_text(encoding="utf-8-sig"))
    with tempfile.TemporaryDirectory() as directory:
        source_root = safe_extract_source(args.source, Path(directory))
        sys.path.insert(0, str(source_root))
        model = torch.hub.load(str(source_root), "parseq_tiny", source="local", pretrained=False)
        checkpoint = torch.load(args.weights, map_location="cpu")
        model.load_state_dict(checkpoint)
        model.eval()
        transform = T.Compose([
            T.Resize((32, 128), T.InterpolationMode.BICUBIC),
            T.ToTensor(),
            T.Normalize(0.5, 0.5),
        ])
        predictions = []
        for fixture in manifest["fixtures"]:
            if fixture.get("ocr_evaluation_eligible", True) is not True:
                continue
            crop_name = fixture.get("truth_crop_file")
            if not crop_name:
                predictions.append({
                    "fixture_id": fixture["fixture_id"], "plate_text_raw": None,
                    "plate_text_normalized": None, "confidence": None,
                    "abstention_state": "no_plate_found", "latency_ms": 0.0,
                })
                continue
            with Image.open(args.fixtures.parent / crop_name) as image:
                tensor = transform(image.convert("RGB")).unsqueeze(0)
            started = time.perf_counter()
            with torch.inference_mode():
                logits = model(tensor)
                probabilities = logits.softmax(-1)
                raw_labels, raw_confidences = model.tokenizer.decode(probabilities)
            latency_ms = (time.perf_counter() - started) * 1000
            raw = str(raw_labels[0]).strip()
            confidence_tensor = raw_confidences[0]
            confidence = float(confidence_tensor.prod().item()) if confidence_tensor.numel() else None
            normalized = normalize(raw)
            predictions.append({
                "fixture_id": fixture["fixture_id"], "plate_text_raw": raw or None,
                "plate_text_normalized": normalized or None, "confidence": confidence,
                "abstention_state": "candidate" if normalized else "ocr_abstained",
                "latency_ms": round(latency_ms, 3), "model_name": "parseq_tiny",
                "preprocessing": "official bicubic resize 32x128, tensor, normalize mean/std 0.5",
                "normalization_steps": ["Unicode NFKC", "uppercase", "retain alphanumeric code points"],
            })

    payload = {
        "contract_version": "forensics.ocr-evaluation-output/v1",
        "generated_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "candidate": CANDIDATE, "candidate_id": CANDIDATE,
        "candidate_revision": REVISION, "evaluation_role": "ocr_only_annotated_crop",
        "fixture_manifest_sha256": hash_file(args.fixtures),
        "source_tiers": sorted({str(item.get("tier", "unknown")) for item in manifest["fixtures"]}),
        "offline": True,
        "host": {"platform": platform.platform(), "cpu_count": os.cpu_count(), "python": platform.python_version(), "torch": torch.__version__},
        "preprocessing_recipe": "immutable annotated truth crop; official PARSeq v1.0.0 parseq-tiny; bicubic 32x128; Normalize(0.5,0.5); no character substitution",
        "package_size_mib": round(args.weights.stat().st_size / 1048576, 6),
        "peak_process_rss_mib": round(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024, 3),
        "source_sha256": SOURCE_SHA256, "weight_sha256": WEIGHT_SHA256,
        "predictions": predictions,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(json.dumps({"status": "pass", "candidate": CANDIDATE, "predictions": len(predictions), "output": str(args.output)}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
