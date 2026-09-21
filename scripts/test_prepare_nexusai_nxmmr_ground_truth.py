#!/usr/bin/env python3
"""Focused tests for the NX-MMR non-inference ground-truth tool."""

from __future__ import annotations

import csv
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from PIL import Image


SCRIPT = Path(__file__).with_name("prepare_nexusai_nxmmr_ground_truth.py")


class GroundTruthPreparationTests(unittest.TestCase):
    def test_image_pack_seals_twenty_percent_and_leaves_labels_blank(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            inputs = root / "inputs"
            output = root / "output"
            inputs.mkdir()
            for index in range(10):
                Image.new("RGB", (80 + index, 60), (index, index, index)).save(inputs / f"{index}.jpg")
            subprocess.run([
                sys.executable, str(SCRIPT), "prepare-images", "--input-root", str(inputs),
                "--output-root", str(output), "--seed", "test-seed",
            ], check=True, capture_output=True, text=True)
            with (output / "image-ground-truth-template.csv").open(
                "r", encoding="utf-8-sig", newline=""
            ) as source:
                rows = list(csv.DictReader(source))
            self.assertEqual(10, len(rows))
            self.assertEqual(2, sum(row["split"] == "sealed_holdout" for row in rows))
            self.assertTrue(all(row["plate_text"] == "" for row in rows))
            self.assertTrue(all(row["model_output_seen"] == "false" for row in rows))
            self.assertTrue((output / "image-contact-sheets" / "contact-sheet-01.jpg").is_file())

    def test_validator_rejects_unlabeled_template(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            manifest = Path(temporary) / "labels.csv"
            manifest.write_text(
                "sample_id,relative_file,plate_presence,label_source,model_output_seen\n"
                "A,a.jpg,,independent_human_annotation,false\n",
                encoding="utf-8",
            )
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "validate", str(manifest)],
                capture_output=True, text=True,
            )
            self.assertEqual(2, result.returncode)
            self.assertIn("PENDING_OR_INVALID", result.stdout)


if __name__ == "__main__":
    unittest.main()
