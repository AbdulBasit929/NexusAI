# SPDX-License-Identifier: Apache-2.0
"""Model-free checks of the ONNX text OCR module: decoding order, reading order, quadrilateral helpers and the refusal to start without its pinned files.

The recogniser and detector themselves are exercised against real model files by tools/media_data/ocr_eval.py in the evaluation image; these tests keep the pure logic honest in CI.
"""

import tempfile
import unittest
from pathlib import Path

import numpy as np

from ingestion.forensic_records import onnx_text_ocr as ocr


class DecodingOrderTests(unittest.TestCase):
    def test_arabic_runs_are_reversed_and_latin_digit_runs_are_kept(self):
        # drawn left to right: the digits "123" then two Arabic letters; logical order is the letters reversed after the digits' position
        self.assertEqual(ocr._reverse_visual_to_logical("ab"), "ab")
        self.assertEqual(ocr._reverse_visual_to_logical("با"), "اب")
        self.assertEqual(ocr._reverse_visual_to_logical("123 با"), "اب123 ")

    def test_script_of(self):
        self.assertEqual(ocr._script_of("ابت 12"), "arabic")
        self.assertEqual(ocr._script_of("abc ا"), "latin")
        self.assertEqual(ocr._script_of("12 34"), "other")


class GeometryTests(unittest.TestCase):
    def test_order_quad_is_clockwise_from_top_left(self):
        shuffled = np.array([[90, 40], [10, 10], [10, 40], [90, 10]], np.float32)
        ordered = ocr._order_quad(shuffled)
        self.assertEqual(ordered.tolist(), [[10, 10], [90, 10], [90, 40], [10, 40]])

    def test_vertical_box_is_rotated_to_a_horizontal_line(self):
        image = np.zeros((200, 100, 3), np.uint8)
        quad = np.array([[10, 10], [40, 10], [40, 190], [10, 190]], np.float32)  # 30 wide, 180 tall
        crop = ocr._crop_quad(image, quad)
        self.assertGreater(crop.shape[1], crop.shape[0])

    def test_reading_order_groups_lines_and_goes_left_to_right_within_a_line(self):
        def box(x0, y0, x1, y1):
            return np.array([[x0, y0], [x1, y0], [x1, y1], [x0, y1]], np.float32)
        boxes = [box(200, 100, 300, 130), box(10, 12, 100, 42), box(120, 10, 190, 40), box(10, 100, 100, 130)]
        order = ocr.PaddleOnnxOCR._reading_order(boxes)
        self.assertEqual(order, [1, 2, 3, 0])


class JoinLinesTests(unittest.TestCase):
    def line(self, text, x, y, script):
        quad = ((x, y), (x + 80, y), (x + 80, y + 30), (x, y + 30))
        return ocr.TextLine(text, 0.9, quad, script)

    def test_same_row_joined_right_to_left_for_arabic_and_left_to_right_for_latin(self):
        arabic_row = [self.line("A", 10, 10, "arabic"), self.line("B", 200, 12, "arabic")]
        latin_row = [self.line("C", 10, 100, "latin"), self.line("D", 200, 102, "latin")]
        self.assertEqual(ocr.join_lines(arabic_row + latin_row), "B A\nC D")


class StartupTests(unittest.TestCase):
    def test_a_missing_model_file_refuses_to_start(self):
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(ocr.OnnxOcrUnavailableError):
                ocr.PaddleOnnxDetector(Path(directory) / "absent.onnx")

    def test_a_missing_configuration_refuses_to_start(self):
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(ocr.OnnxOcrUnavailableError):
                ocr.PaddleOnnxRecognizer(Path(directory) / "m.onnx", Path(directory) / "inference.yml", arabic_script=True)


if __name__ == "__main__":
    unittest.main()
