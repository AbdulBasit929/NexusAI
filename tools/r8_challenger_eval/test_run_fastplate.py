import unittest

from tools.r8_challenger_eval.run_fastplate import geometric_mean, normalize, percentile


class FastPlateEvaluatorUnitTests(unittest.TestCase):
    def test_normalization_does_not_substitute_ambiguous_characters(self):
        self.assertEqual(normalize(" ab-O01_ "), "ABO01")

    def test_geometric_mean_is_length_normalized(self):
        self.assertAlmostEqual(geometric_mean([0.81, 1.0]), 0.9)
        self.assertEqual(geometric_mean([0.9, 0.0]), 0.0)
        self.assertIsNone(geometric_mean([]))

    def test_nearest_rank_percentile(self):
        self.assertEqual(percentile([4.0, 1.0, 3.0, 2.0], 0.50), 2.0)
        self.assertEqual(percentile([4.0, 1.0, 3.0, 2.0], 0.95), 4.0)


if __name__ == "__main__":
    unittest.main()
