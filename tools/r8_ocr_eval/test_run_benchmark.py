import tempfile
import unittest
from pathlib import Path

from PIL import Image

from run_benchmark import prepared_detector_input, prepared_ocr_input


class PreprocessingTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.image_path = Path(self.temp_dir.name) / "fixture.png"
        Image.new("RGB", (100, 50), "white").save(self.image_path)

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_scale2x_is_exactly_twice_the_source_dimensions(self):
        with prepared_ocr_input(self.image_path, "scale2x") as prepared:
            with Image.open(prepared) as image:
                self.assertEqual(image.size, (200, 100))

    def test_detector_resize_reports_input_scale(self):
        with prepared_detector_input(self.image_path, 640) as (prepared, scale_x, scale_y):
            with Image.open(prepared) as image:
                self.assertEqual(image.size, (640, 320))
            self.assertAlmostEqual(scale_x, 640 / 100)
            self.assertAlmostEqual(scale_y, 320 / 50)


if __name__ == "__main__":
    unittest.main()
