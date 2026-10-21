# SPDX-License-Identifier: Apache-2.0
"""The OCR router with fake engines: script routing, the confidence gate, the choice between readings, and that nothing is merged."""

import unittest
from pathlib import Path

from ingestion.forensic_records.media_pipeline import MediaObservation, MediaProcessResult
from ingestion.forensic_records.ocr_router import ONNX_LATIN, TESSERACT_LATIN, TESSERACT_URDU, OcrRouter, reading_quality


def observation(text, confidence):
    return MediaObservation(observation_id=text, contract_version="c", observation_type="general_image_ocr_observation", confidence=confidence,
                            citation_locator={}, payload={"raw_text": text}, warnings=())


def result(*pairs):
    return MediaProcessResult(modality="image", readiness="READY", processor_id="engine", processor_revision="1", observations=tuple(observation(t, c) for t, c in pairs),
                              limitations=("base limitation",), metadata={"result_state": "COMPLETE_RESULTS"})


class FakeEngine:
    def __init__(self, outcome):
        self.outcome, self.calls = outcome, 0

    def process_image(self, path, **context):
        self.calls += 1
        return self.outcome


def router(tesseract_latin, tesseract_urdu, onnx, script):
    return OcrRouter(tesseract_latin, tesseract_urdu, onnx, lambda image: script, confident=0.75, minimum_characters=12)


class RouterTests(unittest.TestCase):
    path = Path("page.png")

    def test_latin_is_read_by_the_onnx_engine_and_a_confident_reading_is_returned_alone(self):
        onnx = FakeEngine(result(("Subscriber record 0300-1234567", 0.95)))
        tess = FakeEngine(result(("other", 0.99)))
        out = router(tess, None, onnx, "Latin").process_image(self.path)
        self.assertEqual((onnx.calls, tess.calls), (1, 0))
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], ONNX_LATIN)

    def test_a_low_confidence_onnx_reading_is_read_again_by_tesseract_and_the_better_reading_wins_whole(self):
        onnx = FakeEngine(result(("garbled", 0.40)))
        tess = FakeEngine(result(("TOTAL 1,200", 0.93), ("CASH 1,500", 0.91)))
        out = router(tess, None, onnx, "Latin").process_image(self.path)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], TESSERACT_LATIN)
        self.assertEqual([o.payload["raw_text"] for o in out.observations], ["TOTAL 1,200", "CASH 1,500"])  # one engine's lines, not a mix

    def test_the_first_reading_stays_when_the_second_is_no_better(self):
        onnx = FakeEngine(result(("some low text here for twenty chars", 0.70)))
        tess = FakeEngine(result(("x", 0.60)))
        out = router(tess, None, onnx, "Latin").process_image(self.path)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], ONNX_LATIN)
        self.assertEqual(out.metadata["ocr_route"]["engines_tried"], [ONNX_LATIN, TESSERACT_LATIN])

    def test_an_arabic_script_page_goes_to_urdu_data_and_never_to_the_latin_engines(self):
        urdu = FakeEngine(result(("\u0627\u0633 \u0628\u0627\u062a \u0633\u06d2", 0.70)))
        onnx, tess = FakeEngine(result(("garbage", 0.99))), FakeEngine(result())
        out = router(tess, urdu, onnx, "Arabic").process_image(self.path)
        self.assertEqual((urdu.calls, onnx.calls, tess.calls), (1, 0, 0))
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], TESSERACT_URDU)
        self.assertTrue(any("Arabic-script page" in item for item in out.limitations))

    def test_a_misdetected_script_is_rescued_by_the_urdu_second_opinion_when_the_onnx_reading_is_not_confident(self):
        onnx = FakeEngine(result(("xqz kvw", 0.35)))
        urdu = FakeEngine(result(("اس بات سے کہا", 0.89)))
        tess = FakeEngine(result(("never used", 0.99)))
        out = router(tess, urdu, onnx, "Japanese").process_image(self.path)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], TESSERACT_URDU)
        self.assertEqual(out.metadata["ocr_route"]["engines_tried"], [ONNX_LATIN, TESSERACT_URDU])
        self.assertEqual(tess.calls, 0)

    def test_a_clearly_latin_page_gets_the_english_second_opinion_not_the_urdu_one(self):
        onnx = FakeEngine(result(("faded", 0.4)))
        tess, urdu = FakeEngine(result(("Faded receipt text restored here", 0.8))), FakeEngine(result(("junk", 0.99)))
        out = router(tess, urdu, onnx, "Latin").process_image(self.path)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], TESSERACT_LATIN)
        self.assertEqual(urdu.calls, 0)

    def test_without_urdu_data_an_arabic_page_is_read_as_latin_not_failed(self):
        onnx = FakeEngine(result(("some text on a page of enough size", 0.9)))
        out = router(FakeEngine(result()), None, onnx, "Arabic").process_image(self.path)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], ONNX_LATIN)

    def test_no_script_answer_is_read_as_latin_and_recorded_as_unknown(self):
        onnx = FakeEngine(result(("enough characters here to pass the gate", 0.93)))
        out = router(FakeEngine(result()), FakeEngine(result()), onnx, None).process_image(self.path)
        self.assertIsNone(out.metadata["ocr_route"]["script_detected"])

    def test_an_empty_first_reading_uses_the_second_even_with_lower_confidence_numbers(self):
        onnx = FakeEngine(result())
        tess = FakeEngine(result(("TOTAL 5", 0.5)))
        out = router(tess, None, onnx, "Latin").process_image(self.path)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], TESSERACT_LATIN)

    def test_with_no_onnx_models_the_latin_path_is_tesseract_alone(self):
        tess = FakeEngine(result(("Plain page text of sufficient length", 0.9)))
        out = router(tess, None, None, "Latin").process_image(self.path)
        self.assertEqual(out.metadata["ocr_route"]["engines_tried"], [TESSERACT_LATIN])

    def test_reading_quality(self):
        self.assertEqual(reading_quality(result()), (0.0, 0))
        self.assertEqual(reading_quality(result(("ab", 0.5), ("cde", 1.0))), (0.75, 5))

    def test_a_router_with_no_latin_reader_is_refused(self):
        with self.assertRaises(ValueError):
            OcrRouter(None, None, None, lambda image: None)


class ProbedOnnx(FakeEngine):
    """An ONNX engine with the Arabic-script probe: `share` is what the probe reports."""

    def __init__(self, outcome, share):
        super().__init__(outcome)
        self.share = share
        self.script_probe = object()

    def arabic_script_share(self, path):
        return self.share


class ProbeRoutingTests(unittest.TestCase):
    path = Path("page.png")

    def test_a_high_arabic_share_routes_to_urdu_even_when_the_script_pass_says_latin(self):
        urdu = FakeEngine(result(("urdu text line one", 0.9)))
        onnx = ProbedOnnx(result(("garbage", 0.9)), 0.92)
        out = OcrRouter(FakeEngine(result()), urdu, onnx, lambda image: "Latin").process_image(self.path)
        self.assertEqual((urdu.calls, onnx.calls), (1, 0))
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], TESSERACT_URDU)
        self.assertEqual(out.metadata["ocr_route"]["arabic_script_share"], 0.92)

    def test_a_low_arabic_share_stays_latin_even_when_the_script_pass_says_arabic(self):
        urdu = FakeEngine(result(("never used", 0.9)))
        onnx = ProbedOnnx(result(("Receipt total 1,200 cash 1,500", 0.95)), 0.05)
        out = OcrRouter(FakeEngine(result()), urdu, onnx, lambda image: "Arabic").process_image(self.path)
        self.assertEqual((urdu.calls, onnx.calls), (0, 1))
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], ONNX_LATIN)

    def test_a_probe_with_no_text_falls_back_to_the_script_pass(self):
        urdu = FakeEngine(result(("urdu text line one", 0.9)))
        onnx = ProbedOnnx(result(("x", 0.9)), None)
        out = OcrRouter(FakeEngine(result()), urdu, onnx, lambda image: "Arabic").process_image(self.path)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], TESSERACT_URDU)
        self.assertIsNone(out.metadata["ocr_route"]["arabic_script_share"])


if __name__ == "__main__":
    unittest.main()
