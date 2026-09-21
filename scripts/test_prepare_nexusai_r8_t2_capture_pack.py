import unittest

from scripts.prepare_nexusai_r8_t2_capture_pack import build_plan


class T2CapturePlanTest(unittest.TestCase):
    def test_plan_has_exact_coverage_and_sealed_holdout(self):
        plan = build_plan()
        self.assertEqual(32, plan["planned_image_count"])
        self.assertEqual(28, sum(item["partition"] == "development" for item in plan["items"]))
        self.assertEqual(4, sum(item["partition"] == "holdout" for item in plan["items"]))
        self.assertEqual(10, sum(
            item["partition"] == "development" and item["negative_class"] is not None
            for item in plan["items"]
        ))
        self.assertEqual(11, sum(item["negative_class"] is not None for item in plan["items"]))
        self.assertEqual(32, len({item["filename"] for item in plan["items"]}))


if __name__ == "__main__":
    unittest.main()
