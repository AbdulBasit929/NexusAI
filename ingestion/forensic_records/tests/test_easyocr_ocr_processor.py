# SPDX-License-Identifier: Apache-2.0
"""The EasyOCR role with a stub reader and a stand-in for OpenCV: line assembly (the rule the bake-off measured), the observation contract, scaling back to original pixels, failures, and the startup
refusals. No model files and no GPU are needed."""

import hashlib
import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from ingestion.forensic_records.easyocr_ocr_processor import (
    DETECTOR_FILE,
    RECOGNIZER_FILE,
    EasyOcrProcessor,
    assemble_lines,
)
from ingestion.forensic_records.media_pipeline import (
    IMAGE_OCR_OBSERVATION_CONTRACT,
    MediaProcessingError,
    ModelUnavailableError,
)

URDU_ONE, URDU_TWO, URDU_THREE = "اسلام", "آباد", "کراچی"


def box(x1, y1, x2, y2):
    return [[x1, y1], [x2, y1], [x2, y2], [x1, y2]]


class Frame:
    """The little of a NumPy image the processor touches: a shape and a crop."""

    def __init__(self, height, width):
        self.shape = (height, width, 3)

    def __getitem__(self, key):
        rows, columns = key
        return Frame(rows.stop - rows.start, columns.stop - columns.start)


class FakeCv2:
    INTER_AREA = 3

    def __init__(self, height=400, width=1000):
        self.frame, self.resized_to = Frame(height, width), None

    def imread(self, path):
        return self.frame

    def resize(self, frame, size, interpolation=None):
        self.resized_to = size
        return Frame(size[1], size[0])

    def imencode(self, extension, frame):
        return True, bytes(f"{extension}:{frame.shape[0]}x{frame.shape[1]}", "ascii")


class StubReader:
    def __init__(self, results=None, error=None):
        self.results, self.error, self.seen = results or [], error, []

    def readtext(self, image, detail=1, paragraph=False):
        self.seen.append((image.shape, detail, paragraph))
        if self.error:
            raise self.error
        return self.results


CONTEXT = {"source_file": "scan.png", "source_sha256": "ab" * 32, "evidence_id": "evidence-1", "version_id": "version-1"}
HASHES = {"detector": "d" * 64, "recognizer": "r" * 64}


class AssemblyTests(unittest.TestCase):
    def test_an_urdu_line_reads_right_to_left_and_a_latin_line_left_to_right(self):
        results = [
            (box(500, 10, 700, 50), URDU_ONE, 0.9), (box(100, 12, 300, 52), URDU_TWO, 0.8), (box(320, 11, 480, 51), URDU_THREE, 0.7),
            (box(300, 110, 380, 150), "Total", 0.95), (box(100, 112, 200, 152), "Order", 0.9), (box(400, 111, 520, 151), "1,500", 0.99),
        ]
        lines = assemble_lines(results)
        self.assertEqual([line["text"] for line in lines], [f"{URDU_ONE} {URDU_THREE} {URDU_TWO}", "Order Total 1,500"])
        self.assertAlmostEqual(lines[0]["confidence"], 0.8)

    def test_boxes_of_one_row_that_sit_a_little_lower_still_join_the_row(self):
        lines = assemble_lines([(box(100, 10, 300, 50), "0300-1234567", 0.9), (box(320, 28, 600, 66), URDU_ONE, 0.8)])
        self.assertEqual(len(lines), 1)
        self.assertEqual(lines[0]["boxes"], 2)

    def test_lines_run_top_to_bottom_and_empty_text_is_dropped(self):
        lines = assemble_lines([(box(0, 200, 90, 240), "second", 0.9), (box(0, 0, 90, 40), "first", 0.9), (box(0, 100, 90, 140), "  ", 0.9)])
        self.assertEqual([line["text"] for line in lines], ["first", "second"])

    def test_a_line_box_is_the_union_of_its_boxes(self):
        lines = assemble_lines([(box(10, 10, 100, 50), "a", 0.9), (box(120, 12, 260, 52), "b", 0.9)])
        self.assertEqual(lines[0]["box"], (10.0, 10.0, 260.0, 52.0))

    def test_the_same_rule_as_the_bake_off_lab_reader(self):
        lab_path = Path(__file__).resolve().parents[3] / "tools" / "media_data" / "easyocr_reader.py"
        spec = importlib.util.spec_from_file_location("easyocr_reader_for_parity", lab_path)
        lab = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(lab)
        results = [
            (box(500, 10, 700, 50), URDU_ONE, 0.9), (box(100, 12, 300, 52), URDU_TWO, 0.8), (box(320, 28, 600, 66), "AB-1234", 0.7),
            (box(300, 110, 380, 150), "Total", 0.95), (box(100, 112, 200, 152), "Order", 0.9), (box(0, 300, 99, 340), URDU_THREE, 0.5),
        ]
        lab_text, lab_confidence = lab.assemble(results)
        lines = assemble_lines(results)
        self.assertEqual("\n".join(line["text"] for line in lines), lab_text)
        self.assertAlmostEqual(sum(line["confidence"] * line["boxes"] for line in lines) / sum(line["boxes"] for line in lines), lab_confidence)


class ProcessImageTests(unittest.TestCase):
    path = Path("scan.png")

    def processor(self, reader, cv2_module=None, **kwargs):
        return EasyOcrProcessor(reader, model_hashes=HASHES, cv2_module=cv2_module or FakeCv2(), **kwargs)

    def test_one_observation_per_line_with_the_contract_and_review_required(self):
        reader = StubReader([(box(500, 10, 700, 50), URDU_ONE, 0.9), (box(100, 12, 300, 52), "AB-1234 0300-1234567", 0.8), (box(0, 100, 99, 140), "second line", 0.95)])
        out = self.processor(reader).process_image(self.path, **CONTEXT)
        self.assertEqual(len(out.observations), 2)
        first = out.observations[0]
        self.assertEqual((first.contract_version, first.observation_type), (IMAGE_OCR_OBSERVATION_CONTRACT, "general_image_ocr_observation"))
        self.assertTrue(first.payload["manual_review_required"])
        self.assertEqual(first.payload["identifiers_preserved"], ["AB-1234", "0300-1234567"])
        self.assertEqual(first.payload["bbox"], {"x": 100, "y": 10, "width": 600, "height": 42})
        self.assertEqual(first.payload["model_sha256"], HASHES)
        self.assertEqual(first.citation_locator["source_file"], "scan.png")
        self.assertEqual(first.citation_locator["coordinate_space"], "original_image_pixels")
        self.assertEqual([item.payload["reading_order"] for item in out.observations], [1, 2])
        self.assertEqual(out.metadata["result_state"], "COMPLETE_RESULTS")
        self.assertEqual(out.metadata["weights_licence_status"], "unverified_evaluation_only")
        self.assertEqual(reader.seen[0][1:], (1, False))
        self.assertTrue(any("not certified" in item for item in out.limitations))

    def test_observation_ids_are_stable_for_the_same_image_and_differ_between_lines(self):
        reader = StubReader([(box(0, 0, 90, 40), "one", 0.9), (box(0, 100, 90, 140), "two", 0.9)])
        a = self.processor(reader).process_image(self.path, **CONTEXT)
        b = self.processor(reader).process_image(self.path, **CONTEXT)
        self.assertEqual([o.observation_id for o in a.observations], [o.observation_id for o in b.observations])
        self.assertEqual(len({o.observation_id for o in a.observations}), 2)

    def test_a_large_image_is_reduced_for_the_reader_and_the_boxes_come_back_in_original_pixels(self):
        fake = FakeCv2(height=2000, width=4000)
        reader = StubReader([(box(100, 50, 500, 150), "scaled", 0.9)])
        out = self.processor(reader, fake, maximum_side=1000).process_image(self.path, **CONTEXT)
        self.assertEqual(fake.resized_to, (1000, 500))
        self.assertEqual(reader.seen[0][0][:2], (500, 1000))
        self.assertEqual(out.observations[0].payload["bbox"], {"x": 400, "y": 200, "width": 1600, "height": 400})
        self.assertEqual(out.metadata["image_scale_applied"], 0.25)

    def test_a_small_image_is_not_resized_when_a_cap_is_set(self):
        fake = FakeCv2(height=400, width=1000)
        out = self.processor(StubReader([(box(0, 0, 90, 40), "x", 0.9)]), fake, maximum_side=1500).process_image(self.path, **CONTEXT)
        self.assertIsNone(fake.resized_to)
        self.assertEqual(out.metadata["image_scale_applied"], 1.0)

    def test_boxes_outside_the_image_are_clamped(self):
        out = self.processor(StubReader([(box(-20, -10, 1500, 600), "wide", 0.9)])).process_image(self.path, **CONTEXT)
        self.assertEqual(out.observations[0].payload["bbox"], {"x": 0, "y": 0, "width": 1000, "height": 400})

    def test_a_reader_error_becomes_a_processing_error_without_the_text(self):
        reader = StubReader(error=RuntimeError("out of memory while reading secret text"))
        with self.assertRaises(MediaProcessingError) as raised:
            self.processor(reader).process_image(self.path, **CONTEXT)
        self.assertIn("RuntimeError", str(raised.exception))
        self.assertNotIn("secret", str(raised.exception))

    def test_an_image_that_cannot_be_decoded_is_refused(self):
        class NoImage(FakeCv2):
            def imread(self, path):
                return None

        with self.assertRaises(MediaProcessingError):
            self.processor(StubReader(), NoImage()).process_image(self.path, **CONTEXT)

    def test_no_text_is_a_ready_with_warnings_result_with_no_observations(self):
        out = self.processor(StubReader([])).process_image(self.path, **CONTEXT)
        self.assertEqual((len(out.observations), out.readiness, out.metadata["result_state"]), (0, "READY_WITH_WARNINGS", "COMPLETE_ZERO_RESULTS"))
        self.assertTrue(any("no non-empty OCR region" in item for item in out.limitations))

    def test_the_line_limit_keeps_the_top_lines(self):
        reader = StubReader([(box(0, 100 * i, 90, 100 * i + 40), f"line {i}", 0.9) for i in range(5)])
        out = self.processor(reader, maximum_lines=3).process_image(self.path, **CONTEXT)
        self.assertEqual([o.payload["raw_text"] for o in out.observations], ["line 0", "line 1", "line 2"])

    def test_invalid_limits_are_refused(self):
        with self.assertRaises(ValueError):
            self.processor(StubReader(), maximum_lines=0)
        with self.assertRaises(ValueError):
            self.processor(StubReader(), maximum_side=-1)


class StartupTests(unittest.TestCase):
    def write_models(self, directory):
        for name, content in ((DETECTOR_FILE, b"detector bytes"), (RECOGNIZER_FILE, b"recogniser bytes")):
            (Path(directory) / name).write_bytes(content)
        return hashlib.sha256(b"detector bytes").hexdigest(), hashlib.sha256(b"recogniser bytes").hexdigest()

    def environment(self, directory, detector, recognizer):
        return {"FORENSIC_EASYOCR_MODEL_DIR": str(directory), "FORENSIC_EASYOCR_DETECTOR_SHA256": detector, "FORENSIC_EASYOCR_RECOGNIZER_SHA256": recognizer}

    def test_no_model_directory_refuses_to_start(self):
        with patch.dict("os.environ", {"FORENSIC_EASYOCR_MODEL_DIR": ""}):
            with self.assertRaises(ModelUnavailableError):
                EasyOcrProcessor.from_environment()

    def test_a_missing_model_file_refuses_to_start(self):
        with tempfile.TemporaryDirectory() as directory:
            with patch.dict("os.environ", self.environment(directory, "0" * 64, "0" * 64)):
                with self.assertRaises(ModelUnavailableError):
                    EasyOcrProcessor.from_environment()

    def test_a_hash_that_does_not_match_the_file_refuses_to_start(self):
        with tempfile.TemporaryDirectory() as directory:
            detector, recognizer = self.write_models(directory)
            with patch.dict("os.environ", self.environment(directory, detector, "f" * 64)):
                with self.assertRaises(ModelUnavailableError):
                    EasyOcrProcessor.from_environment()

    def test_an_absent_hash_refuses_to_start(self):
        with tempfile.TemporaryDirectory() as directory:
            detector, _ = self.write_models(directory)
            with patch.dict("os.environ", self.environment(directory, detector, "")):
                with self.assertRaises(ModelUnavailableError):
                    EasyOcrProcessor.from_environment()

    def test_matching_files_without_the_packages_refuse_to_start_and_nothing_is_downloaded(self):
        with tempfile.TemporaryDirectory() as directory:
            detector, recognizer = self.write_models(directory)
            with patch.dict("os.environ", self.environment(directory, detector, recognizer)), patch.dict(sys.modules, {"easyocr": None}):
                with self.assertRaises(ModelUnavailableError):
                    EasyOcrProcessor.from_environment()


if __name__ == "__main__":
    unittest.main()
