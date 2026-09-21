"""Model-free image identity and comparison primitives.

Exact identity and perceptual similarity are deliberately separate.  The
perceptual hash is a compact pixel resemblance signal; it is not semantic
understanding and must never be presented as identity or scene equivalence.
"""

from __future__ import annotations

import os
import re
import shutil
import subprocess
from pathlib import Path
from typing import Any, Iterable


IMAGE_FINGERPRINT_CONTRACT = "forensics.image-fingerprint/v1"
IMAGE_COMPARISON_CONTRACT = "forensics.image-comparison/v1"
PERCEPTUAL_HASH_ALGORITHM = "dhash-64-ffmpeg-area-v1"
DEFAULT_NEAR_DUPLICATE_MAX_DISTANCE = 10


def image_dhash(path: Path) -> str | None:
    """Return a deterministic 64-bit difference hash, or ``None`` on decode failure."""

    thumbnail = _grayscale_thumbnail(path)
    if thumbnail is None or len(thumbnail) != 72:
        return None
    value = 0
    for row in range(8):
        offset = row * 9
        for column in range(8):
            value = (value << 1) | int(thumbnail[offset + column] > thumbnail[offset + column + 1])
    return f"{value:016x}"


def hamming_distance(left: str, right: str) -> int:
    """Return bit distance for two validated 64-bit hexadecimal fingerprints."""

    for value in (left, right):
        if not re.fullmatch(r"[0-9a-fA-F]{16}", value or ""):
            raise ValueError("perceptual hashes must be 16 hexadecimal characters")
    return (int(left, 16) ^ int(right, 16)).bit_count()


def compare_image_signals(
    left: dict[str, Any],
    right: dict[str, Any],
    *,
    near_duplicate_max_distance: int = DEFAULT_NEAR_DUPLICATE_MAX_DISTANCE,
) -> dict[str, Any]:
    """Compare governed image signals without collapsing metric semantics."""

    if not 0 <= near_duplicate_max_distance <= 64:
        raise ValueError("near-duplicate distance threshold must be between zero and 64")
    left_sha = str(left.get("sha256") or "").lower()
    right_sha = str(right.get("sha256") or "").lower()
    sha_match = bool(left_sha and right_sha and left_sha == right_sha)
    left_hash = str(left.get("perceptual_hash") or "").lower()
    right_hash = str(right.get("perceptual_hash") or "").lower()
    distance: int | None = None
    similarity: float | None = None
    if left_hash and right_hash:
        distance = hamming_distance(left_hash, right_hash)
        similarity = 1.0 - (distance / 64.0)
    near_duplicate = bool(not sha_match and distance is not None and distance <= near_duplicate_max_distance)
    ocr_left, ocr_right = _normalized_tokens(left.get("ocr_text")), _normalized_tokens(right.get("ocr_text"))
    plate_left, plate_right = _normalized_values(left.get("plates")), _normalized_values(right.get("plates"))
    return {
        "contract_version": IMAGE_COMPARISON_CONTRACT,
        "source_a": left.get("source"),
        "source_b": right.get("source"),
        "sha256_match": sha_match,
        "exact_duplicate": sha_match,
        "dimensions": {
            "source_a": left.get("dimensions"),
            "source_b": right.get("dimensions"),
            "match": bool(left.get("dimensions") and left.get("dimensions") == right.get("dimensions")),
        },
        "perceptual": {
            "algorithm": PERCEPTUAL_HASH_ALGORITHM,
            "hash_a": left_hash or None,
            "hash_b": right_hash or None,
            "hamming_distance": distance,
            "similarity": similarity,
            "near_duplicate_max_distance": near_duplicate_max_distance,
            "near_duplicate_candidate": near_duplicate,
            "semantics": "pixel-layout resemblance; not semantic visual similarity",
        },
        "ocr": _overlap(ocr_left, ocr_right),
        "anpr": _overlap(plate_left, plate_right),
        "faces": {
            "source_a_count": _optional_non_negative_int(left.get("face_count")),
            "source_b_count": _optional_non_negative_int(right.get("face_count")),
            "similarity": None,
        },
        "visual_similarity": {
            "available": False,
            "score": None,
            "limitation": "no approved semantic image-embedding role is configured",
        },
        "limitations": [
            "Perceptual similarity is a near-duplicate signal, not scene understanding or identity.",
            "Missing OCR, ANPR, or face observations are reported as unavailable rather than inferred.",
        ],
    }


def _grayscale_thumbnail(path: Path) -> bytes | None:
    executable = shutil.which(os.getenv("FORENSIC_FFMPEG_PATH", "ffmpeg"))
    if not executable:
        return None
    try:
        completed = subprocess.run(
            [
                executable,
                "-nostdin",
                "-v",
                "error",
                "-i",
                str(path),
                "-vf",
                "scale=9:8:flags=area,format=gray",
                "-frames:v",
                "1",
                "-f",
                "rawvideo",
                "-pix_fmt",
                "gray",
                "pipe:1",
            ],
            check=True,
            capture_output=True,
            timeout=max(5, int(os.getenv("FORENSIC_IMAGE_HASH_TIMEOUT_SECONDS", "20"))),
        )
    except (OSError, subprocess.SubprocessError, ValueError):
        return None
    return completed.stdout if len(completed.stdout) == 72 else None


def _normalized_tokens(value: Any) -> set[str]:
    if value is None:
        return set()
    if isinstance(value, (list, tuple, set)):
        text = " ".join(str(item) for item in value)
    else:
        text = str(value)
    return {token.casefold() for token in re.findall(r"[^\W_]+", text, flags=re.UNICODE)}


def _normalized_values(value: Any) -> set[str]:
    values: Iterable[Any]
    if value is None:
        return set()
    if isinstance(value, (list, tuple, set)):
        values = value
    else:
        values = (value,)
    return {re.sub(r"[^A-Z0-9]", "", str(item).upper()) for item in values if str(item).strip()}


def _overlap(left: set[str], right: set[str]) -> dict[str, Any]:
    union = left | right
    intersection = left & right
    return {
        "available": bool(left or right),
        "shared": sorted(intersection),
        "jaccard": (len(intersection) / len(union)) if union else None,
    }


def _optional_non_negative_int(value: Any) -> int | None:
    if value is None:
        return None
    try:
        result = int(value)
    except (TypeError, ValueError):
        return None
    return result if result >= 0 else None
