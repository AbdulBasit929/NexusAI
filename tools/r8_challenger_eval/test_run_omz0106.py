import unittest

from run_omz0106 import detections_to_regions


class OMZ0106OutputTests(unittest.TestCase):
    def test_keeps_plate_class_and_maps_original_coordinates(self):
        detections = [
            [0, 1, 0.99, 0.1, 0.1, 0.9, 0.9],
            [0, 2, 0.80, 0.25, 0.5, 0.75, 0.8],
            [0, 2, 0.49, 0.1, 0.1, 0.2, 0.2],
        ]
        self.assertEqual(
            detections_to_regions(detections, 1000, 500, 0.5),
            [{"bounds": {"x": 250, "y": 250, "width": 500, "height": 150}, "confidence": 0.8}],
        )

    def test_negative_image_id_is_ignored(self):
        self.assertEqual(detections_to_regions([[-1, 2, 1, 0, 0, 1, 1]], 100, 100, 0.5), [])


if __name__ == "__main__":
    unittest.main()
