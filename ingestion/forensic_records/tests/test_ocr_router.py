# SPDX-License-Identifier: Apache-2.0
"""The OCR router with fake engines: script routing, the confidence gate, the choice between readings, and that nothing is merged."""

import unittest
from pathlib import Path

from ingestion.forensic_records.media_pipeline import MediaObservation, MediaProcessResult
from ingestion.forensic_records.ocr_router import OcrRouter, reading_quality


def observation(text, confidence):
    return MediaObservation(observation_id=text, contract_version="c", observation_type="general_image_ocr_observation", confidence=confidence,
                            citation_locator={}, payload={"raw_text": text}, warnings=())


def result(*pairs, name="engine"):
    return MediaProcessResult(modality="image", readiness="READY", processor_id=name, processor_revision="1", observations=tuple(observation(t, c) for t, c in pairs),
                              limitations=("base limitation",), metadata={"result_state": "COMPLETE_RESULTS"})


class FakeEngine:
    def __init__(self, outcome, name):
        self.outcome, self.name, self.calls = outcome, name, 0

    def process_image(self, path, **context):
        self.calls += 1
        return self.outcome


def router(latin, urdu, fallback, script):
    return OcrRouter(latin, urdu, fallback, lambda image: script, confident=0.88, minimum_characters=20)


class RouterTests(unittest.TestCase):
    path = Path("page.png")

    def test_a_confident_latin_reading_is_returned_without_the_fallback(self):
        latin = FakeEngine(result(("Subscriber record 0300-1234567", 0.95)), "latin")
        fallback = FakeEngine(result(("other", 0.99)), "onnx")
        out = router(latin, None, fallback, "Latin").process_image(self.path)
        self.assertEqual(fallback.calls, 0)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], "tesseract-eng-psm3")
        self.assertEqual(out.observations[0].payload["raw_text"], "Subscriber record 0300-1234567")

    def test_a_low_confidence_reading_is_read_again_and_the_better_reading_wins_whole(self):
        latin = FakeEngine(result(("garbled text of a receipt", 0.55)), "latin")
        fallback = FakeEngine(result(("TOTAL 1,200", 0.93), ("CASH 1,500", 0.91)), "onnx")
        out = router(latin, None, fallback, "Latin").process_image(self.path)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], "onnx-ppocrv5-latin")
        self.assertEqual([o.payload["raw_text"] for o in out.observations], ["TOTAL 1,200", "CASH 1,500"])  # one engine's lines, not a mix

    def test_the_first_reading_stays_when_the_fallback_is_no_better(self):
        latin = FakeEngine(result(("some low text here for twenty chars", 0.80)), "latin")
        fallback = FakeEngine(result(("x", 0.60)), "onnx")
        out = router(latin, None, fallback, "Latin").process_image(self.path)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], "tesseract-eng-psm3")
        self.assertEqual(out.metadata["ocr_route"]["engines_tried"], ["tesseract-eng-psm3", "onnx-ppocrv5-latin"])

    def test_an_arabic_script_page_goes_to_urdu_data_and_never_to_the_latin_fallback(self):
        urdu = FakeEngine(result(("\u0627\u0633 \u0628\u0627\u062a \u0633\u06d2", 0.70)), "urdu")
        fallback = FakeEngine(result(("garbage", 0.99)), "onnx")
        out = router(FakeEngine(result(), "latin"), urdu, fallback, "Arabic").process_image(self.path)
        self.assertEqual(urdu.calls, 1)
        self.assertEqual(fallback.calls, 0)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], "tesseract-urd+eng-psm3")
        self.assertTrue(any("Arabic-script page" in item for item in out.limitations))

    def test_no_script_answer_is_read_as_latin(self):
        latin = FakeEngine(result(("enough characters here to pass the gate", 0.93)), "latin")
        out = router(latin, FakeEngine(result(), "urdu"), None, None).process_image(self.path)
        self.assertEqual(latin.calls, 1)
        self.assertIsNone(out.metadata["ocr_route"]["script_detected"])

    def test_empty_first_reading_uses_the_fallback_even_with_lower_confidence_numbers(self):
        latin = FakeEngine(result(), "latin")
        fallback = FakeEngine(result(("TOTAL 5", 0.5)), "onnx")
        out = router(latin, None, fallback, "Latin").process_image(self.path)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], "onnx-ppocrv5-latin")

    def test_reading_quality(self):
        self.assertEqual(reading_quality(result()), (0.0, 0))
        self.assertEqual(reading_quality(result(("ab", 0.5), ("cde", 1.0))), (0.75, 5))


if __name__ == "__main__":
    unittest.main()
