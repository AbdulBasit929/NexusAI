from pathlib import Path

import pytest

from ingestion.forensic_records import image_intelligence
from ingestion.forensic_records.image_intelligence import (
    IMAGE_COMPARISON_CONTRACT,
    PERCEPTUAL_HASH_ALGORITHM,
    compare_image_signals,
    hamming_distance,
    image_dhash,
)


def test_dhash_is_independent_of_file_identity(monkeypatch, tmp_path: Path):
    source = tmp_path / "image.jpg"
    source.write_bytes(b"not-decoded-in-unit-test")
    pixels = bytes([value for row in range(8) for value in range(row * 9, row * 9 + 9)])
    monkeypatch.setattr(image_intelligence, "_grayscale_thumbnail", lambda _path: pixels)

    assert image_dhash(source) == "0000000000000000"


def test_hamming_distance_validates_the_dhash_contract():
    assert hamming_distance("0000000000000000", "ffffffffffffffff") == 64
    with pytest.raises(ValueError, match="16 hexadecimal"):
        hamming_distance("not-a-hash", "0" * 16)


def test_comparison_separates_exact_near_and_semantic_similarity():
    result = compare_image_signals(
        {
            "source": {"evidence_id": "a"},
            "sha256": "a" * 64,
            "perceptual_hash": "0000000000000000",
            "dimensions": {"width": 640, "height": 480},
            "ocr_text": "Vehicle MN 1367",
            "plates": ["MN-1367"],
            "face_count": 1,
        },
        {
            "source": {"evidence_id": "b"},
            "sha256": "b" * 64,
            "perceptual_hash": "0000000000000003",
            "dimensions": {"width": 320, "height": 240},
            "ocr_text": "MN 1367",
            "plates": ["MN1367"],
            "face_count": 2,
        },
    )

    assert result["contract_version"] == IMAGE_COMPARISON_CONTRACT
    assert result["exact_duplicate"] is False
    assert result["perceptual"]["algorithm"] == PERCEPTUAL_HASH_ALGORITHM
    assert result["perceptual"]["hamming_distance"] == 2
    assert result["perceptual"]["near_duplicate_candidate"] is True
    assert result["anpr"]["shared"] == ["MN1367"]
    assert result["visual_similarity"]["available"] is False


def test_exact_duplicate_does_not_depend_on_perceptual_hash():
    result = compare_image_signals(
        {"sha256": "c" * 64, "perceptual_hash": None},
        {"sha256": "c" * 64, "perceptual_hash": None},
    )

    assert result["exact_duplicate"] is True
    assert result["perceptual"]["near_duplicate_candidate"] is False
