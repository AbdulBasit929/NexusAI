"""Tests of the pure functions of tools/media_data/speech_lab.py (no model, no audio file, no parquet needed).

    python -I -X utf8 tools/media_data/test_speech_lab.py
"""
import hashlib
import math
import os
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

import speech_lab as sl  # noqa: E402
import speech_report as sr  # noqa: E402


class Partition(unittest.TestCase):
    def test_the_partition_follows_the_last_hex_digit_of_the_hash(self):
        for key in ("ps_af-1", "pst_Arab-spk01-a001-s01", "ur_pk-77"):
            digit = hashlib.sha256(key.encode()).hexdigest()[-1]
            self.assertEqual(sl.partition(key), "dev" if digit in "0123456789ab" else "sealed")

    def test_selection_is_deterministic_disjoint_and_ordered_by_hash(self):
        keys = ["set-%d" % i for i in range(400)]
        dev, sealed = sl.select(keys, "dev", 40), sl.select(keys, "sealed", 40)
        self.assertEqual(dev, sl.select(list(reversed(keys)), "dev", 40))
        self.assertEqual(len(dev), 40)
        self.assertFalse(set(dev) & set(sealed))
        hashes = [hashlib.sha256(k.encode()).hexdigest() for k in dev]
        self.assertEqual(hashes, sorted(hashes))
        self.assertTrue(all(sl.partition(k) == "dev" for k in dev))
        # about three quarters of the keys are development keys
        self.assertTrue(250 < len(sl.select(keys, "dev", 0)) < 350)


class Normalisation(unittest.TestCase):
    def test_n1_removes_punctuation_maps_digits_and_collapses_spaces(self):
        self.assertEqual(sl.normalise("Hello,  World!  ٣۴5"), "hello world 345")

    def test_n1_keeps_arabic_letter_forms_apart_and_n2_unifies_them(self):
        arabic = "كتاب في البيت"          # Arabic kaf and Arabic yeh
        persian = "کتاب فی البیت"        # Keheh and Farsi yeh
        self.assertNotEqual(sl.normalise(arabic, "N1"), sl.normalise(persian, "N1"))
        self.assertEqual(sl.normalise(arabic, "N2"), sl.normalise(persian, "N2"))

    def test_n2_removes_diacritics_tatweel_and_joiners_and_unifies_heh(self):
        self.assertEqual(sl.normalise("كَتَابٌ", "N2"), sl.normalise("کتاب", "N2"))
        self.assertEqual(sl.normalise("ســلام", "N2"), sl.normalise("سلام", "N2"))
        self.assertEqual(sl.normalise("به‌خانه", "N2"), sl.normalise("بهخانه", "N2"))
        self.assertEqual(sl.normalise("ھہ ه", "N2").count("ہ"), 2)    # heh goal and Arabic heh become one letter; the do-chashmi heh is a different letter and stays

    def test_n2_keeps_pashto_letters(self):
        word = "ځې ښه ټول ډېر ړه"
        # every Pashto letter stays; only the plain heh is unified with heh goal (in the reference and in the reading alike)
        self.assertEqual(sl.normalise(word, "N2"), word.replace("ه", "ہ"))


class Errors(unittest.TestCase):
    def test_clip_errors_count_words_and_characters(self):
        we, wl, ce, cl = sl.clip_errors("a b c d", "a x c")
        self.assertEqual((we, wl), (2, 4))
        self.assertEqual(cl, 4)
        self.assertEqual(ce, 2)

    def test_the_corpus_rate_weights_long_clips(self):
        self.assertAlmostEqual(sl.corpus_rate([1, 10], [2, 100]), 11 / 102)
        self.assertEqual(sl.corpus_rate([], []), 0.0)

    def test_the_interval_is_reproducible_and_holds_the_point_estimate(self):
        edits, lengths = [1, 0, 3, 2, 0, 5, 1, 2], [10, 8, 12, 9, 7, 15, 11, 10]
        low, high = sl.bootstrap_interval(edits, lengths)
        self.assertEqual((low, high), sl.bootstrap_interval(edits, lengths))
        self.assertLessEqual(low, sl.corpus_rate(edits, lengths))
        self.assertGreaterEqual(high, sl.corpus_rate(edits, lengths))
        self.assertLess(low, high)

    def test_a_perfect_reading_has_a_degenerate_interval(self):
        self.assertEqual(sl.bootstrap_interval([0, 0, 0], [5, 5, 5]), (0.0, 0.0))


class ScriptAndLoops(unittest.TestCase):
    def test_script_share(self):
        self.assertEqual(sl.script_share("سلام world", "ps"), 4 / 9)
        self.assertEqual(sl.script_share("hello", "en"), 1.0)
        self.assertEqual(sl.script_share("ਸਤ ਸ੍ਰੀ", "pa"), 1.0)
        self.assertEqual(sl.script_share("", "ps"), 0.0)

    def test_loops(self):
        self.assertTrue(sl.looping("a b c", "x y z " * 6))
        self.assertTrue(sl.looping("a b", " ".join(["w"] * 40)))
        self.assertFalse(sl.looping("a b c d e f", "a b c d e f"))


def tone(seconds, rate=16000, amplitude=0.5):
    return [amplitude * math.sin(2 * math.pi * 220 * i / rate) for i in range(int(seconds * rate))]


class Cutting(unittest.TestCase):
    def test_a_short_clip_is_not_cut(self):
        samples = tone(30)
        pieces = sl.split_for_limit(samples)
        self.assertEqual(len(pieces), 1)
        self.assertEqual(len(pieces[0]), len(samples))

    def test_a_long_clip_is_cut_in_the_quiet_gap_and_no_piece_exceeds_the_limit(self):
        rate = 16000
        samples = tone(30) + [0.0] * int(0.4 * rate) + tone(40)       # a 0.4 s pause at 30 s
        pieces = sl.split_for_limit(samples, rate)
        self.assertEqual(sum(len(p) for p in pieces), len(samples))
        self.assertTrue(all(len(p) / rate <= 38.0 for p in pieces))
        first = len(pieces[0]) / rate
        self.assertTrue(30.0 <= first <= 30.5, first)

    def test_short_pieces_for_a_model_trained_on_short_clips(self):
        rate = 16000
        samples = tone(40, rate)
        pieces = sl.split_for_limit(samples, rate, limit=12.0, low=6.0, high=10.8)
        self.assertEqual(sum(len(p) for p in pieces), len(samples))
        self.assertTrue(all(len(p) / rate <= 12.0 for p in pieces))
        self.assertTrue(all(len(p) / rate >= 6.0 for p in pieces[:-1]))
        self.assertGreaterEqual(len(pieces), 4)

    def test_a_clip_of_90_seconds_is_cut_into_pieces_within_the_limit(self):
        rate = 16000
        samples = tone(90, rate)
        pieces = sl.split_for_limit(samples, rate)
        self.assertGreaterEqual(len(pieces), 3)
        self.assertEqual(sum(len(p) for p in pieces), len(samples))
        self.assertTrue(all(len(p) / rate <= 38.0 for p in pieces))


class Summary(unittest.TestCase):
    def test_summarise_reads_the_rows(self):
        def row(we, wl, ce, cl, script=1.0, loop=False, empty=False):
            part = {"w_edits": we, "w_len": wl, "c_edits": ce, "c_len": cl}
            return {"seconds": 10.0, "compute": 5.0, "script": script, "loop": loop, "empty": empty, "N1": part, "N2": part}
        s = sl.summarise([row(1, 10, 3, 50), row(0, 10, 0, 50, script=0.5, loop=True), row(2, 10, 5, 50, empty=True)])
        self.assertEqual(s["clips"], 3)
        self.assertAlmostEqual(s["N1"]["wer"], 3 / 30)
        self.assertAlmostEqual(s["N2"]["cer"], 8 / 150)
        self.assertEqual((s["loops"], s["empty"]), (1, 1))
        self.assertAlmostEqual(s["script_share"], 2.5 / 3)
        self.assertAlmostEqual(s["rtf"], 0.5)


class Recall(unittest.TestCase):
    def test_recall_counts_distinct_reference_words_of_three_letters_or_more_found_in_the_reading(self):
        rows = [{"reference": "کتاب د سړي دی کتاب", "hypothesis": "کتاب سړي"},      # distinct reference words of 3+ letters: کتاب, سړي -> both found
                {"reference": "اوبه شول زما", "hypothesis": "اوبه"}]                  # 3 words, 1 found
        self.assertAlmostEqual(sr.recall(rows), 3 / 5)

    def test_recall_ignores_letter_form_differences_through_n2_and_short_words(self):
        rows = [{"reference": "كتاب في", "hypothesis": "کتاب"}]
        self.assertEqual(sr.recall(rows), 1.0)
        self.assertEqual(sr.recall([{"reference": "د به", "hypothesis": ""}]), 0.0)


if __name__ == "__main__":
    unittest.main()
