# SPDX-License-Identifier: Apache-2.0
"""The OCR router with fake engines: script routing, the confidence gate, the choice between readings, and that nothing is merged."""

import unittest
from pathlib import Path
from unittest.mock import patch

from ingestion.forensic_records.media_pipeline import MediaObservation, MediaProcessingError, MediaProcessResult, ModelUnavailableError
from ingestion.forensic_records.ocr_router import EASYOCR_URDU, ONNX_LATIN, TESSERACT_LATIN, TESSERACT_URDU, OcrRouter, reading_quality


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


class FailingEngine(FakeEngine):
    def process_image(self, path, **context):
        self.calls += 1
        raise MediaProcessingError("engine failed")


URDU_LINE = "اس بات سے کہا گیا"


class EasyOcrRoutingTests(unittest.TestCase):
    path = Path("page.png")

    def build(self, easy, tess_urdu=None, share=0.9, onnx=None, **kwargs):
        onnx = onnx or ProbedOnnx(result(("Receipt total 1,200 cash 1,500", 0.95)), share)
        onnx.share = share
        return OcrRouter(FakeEngine(result()), tess_urdu, onnx, lambda image: "Arabic", confident=0.75, minimum_characters=12, easyocr=easy, **kwargs), onnx

    def test_an_urdu_page_is_read_by_easyocr_and_nothing_else_is_run(self):
        easy, tess = FakeEngine(result((URDU_LINE, 0.80))), FakeEngine(result(("other", 0.99)))
        router_, onnx = self.build(easy, tess, share=0.92)
        out = router_.process_image(self.path)
        self.assertEqual((easy.calls, tess.calls, onnx.calls), (1, 0, 0))
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], EASYOCR_URDU)
        self.assertEqual(out.metadata["ocr_route"]["engines_tried"], [EASYOCR_URDU])
        self.assertEqual(out.metadata["ocr_route"]["arabic_script_share"], 0.92)
        self.assertTrue(any("Arabic-script page" in item for item in out.limitations))

    def test_a_latin_page_is_still_read_by_the_onnx_engine_and_easyocr_is_not_run(self):
        easy = FakeEngine(result(("never used", 0.9)))
        router_, onnx = self.build(easy, FakeEngine(result()), share=0.05)
        out = router_.process_image(self.path)
        self.assertEqual((easy.calls, onnx.calls), (0, 1))
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], ONNX_LATIN)
        self.assertTrue(out.metadata["ocr_route"]["easyocr_configured"])

    def test_the_urdu_share_threshold_decides_where_a_mixed_page_goes(self):
        easy = FakeEngine(result((URDU_LINE + " 0300-1234567", 0.8)))
        router_, onnx = self.build(easy, FakeEngine(result()), share=0.40, urdu_share_threshold=0.30)
        self.assertEqual(router_.process_image(self.path).metadata["ocr_route"]["engine_used"], EASYOCR_URDU)
        easy2 = FakeEngine(result(("never used", 0.8)))
        router2, onnx2 = self.build(easy2, FakeEngine(result()), share=0.40, urdu_share_threshold=0.5)
        self.assertEqual(router2.process_image(self.path).metadata["ocr_route"]["engine_used"], ONNX_LATIN)
        self.assertEqual(easy2.calls, 0)

    def test_the_default_threshold_is_the_one_chosen_on_development_rows(self):
        # every English page and receipt measured at most 0.047 and every Urdu page at least 0.50 (acceptance plan, amendment A1)
        self.assertEqual(OcrRouter(FakeEngine(result()), None, None, lambda image: None).urdu_share_threshold, 0.10)
        easy_low, easy_high = FakeEngine(result(("never used", 0.8))), FakeEngine(result((URDU_LINE, 0.8)))
        self.assertEqual(self.build(easy_low, FakeEngine(result()), share=0.05)[0].process_image(self.path).metadata["ocr_route"]["engine_used"], ONNX_LATIN)
        self.assertEqual(self.build(easy_high, FakeEngine(result()), share=0.12)[0].process_image(self.path).metadata["ocr_route"]["engine_used"], EASYOCR_URDU)

    def test_without_a_probe_the_script_pass_decides(self):
        easy = FakeEngine(result((URDU_LINE, 0.8)))
        out = OcrRouter(FakeEngine(result()), None, FakeEngine(result(("x", 0.9))), lambda image: "Arabic", easyocr=easy).process_image(self.path)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], EASYOCR_URDU)
        self.assertIsNone(out.metadata["ocr_route"]["arabic_script_share"])

    def test_a_failing_easyocr_falls_back_to_tesseract_urdu_and_says_so(self):
        easy, tess = FailingEngine(result()), FakeEngine(result((URDU_LINE, 0.7)))
        router_, _ = self.build(easy, tess)
        out = router_.process_image(self.path)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], TESSERACT_URDU)
        self.assertEqual(out.metadata["ocr_route"]["engines_tried"], [EASYOCR_URDU, TESSERACT_URDU])
        self.assertTrue(any("easyocr-ur+en failed" in item for item in out.limitations))

    def test_a_failing_easyocr_with_no_fallback_is_an_error_not_a_silent_latin_reading(self):
        router_, onnx = self.build(FailingEngine(result()), None)
        with self.assertRaises(MediaProcessingError):
            router_.process_image(self.path)
        self.assertEqual(onnx.calls, 0)

    def test_both_engines_failing_raises_the_fallbacks_error(self):
        router_, _ = self.build(FailingEngine(result()), FailingEngine(result()))
        with self.assertRaises(MediaProcessingError):
            router_.process_image(self.path)

    def test_an_almost_empty_easyocr_reading_is_replaced_only_by_a_longer_fallback_reading(self):
        easy, tess = FakeEngine(result(("ab", 0.9))), FakeEngine(result((URDU_LINE, 0.4)))
        out = self.build(easy, tess)[0].process_image(self.path)
        self.assertEqual(out.metadata["ocr_route"]["engine_used"], TESSERACT_URDU)
        easy2, tess2 = FakeEngine(result(("abcdef", 0.5))), FakeEngine(result(("ab", 0.99)))
        out2 = self.build(easy2, tess2)[0].process_image(self.path)
        self.assertEqual(out2.metadata["ocr_route"]["engine_used"], EASYOCR_URDU)  # the fallback read less: confidences of different engines are never compared

    def test_a_long_easyocr_reading_is_kept_even_when_its_confidence_is_low(self):
        easy, tess = FakeEngine(result((URDU_LINE + " " + URDU_LINE, 0.30))), FakeEngine(result(("other text here", 0.99)))
        out = self.build(easy, tess)[0].process_image(self.path)
        self.assertEqual((out.metadata["ocr_route"]["engine_used"], tess.calls), (EASYOCR_URDU, 0))

    def test_an_empty_easyocr_reading_with_an_empty_fallback_returns_an_empty_result(self):
        easy, tess = FakeEngine(result()), FakeEngine(result())
        out = self.build(easy, tess)[0].process_image(self.path)
        self.assertEqual(len(out.observations), 0)

    def test_the_urdu_share_threshold_must_be_a_share(self):
        for bad in (0, -0.1, 1.5):
            with self.assertRaises(ValueError):
                OcrRouter(FakeEngine(result()), None, None, lambda image: None, urdu_share_threshold=bad)

    def test_the_router_names_version_5(self):
        self.assertEqual(OcrRouter.processor_revision, "nexusai-ocr-router-v5")


class EnvironmentTests(unittest.TestCase):
    base = {"FORENSIC_OCR_LANGUAGES": "eng", "FORENSIC_ONNX_OCR_DETECTOR_PATH": ""}

    def test_easyocr_is_off_unless_it_is_enabled(self):
        with patch.dict("os.environ", {**self.base, "FORENSIC_OCR_EASYOCR_ENABLED": ""}), \
                patch("ingestion.forensic_records.ocr_router.TesseractImageOCRProcessor.from_environment", return_value=FakeEngine(result())):
            self.assertIsNone(OcrRouter.from_environment().easyocr)

    def test_an_enabled_easyocr_with_a_model_file_that_does_not_match_refuses_to_start(self):
        import hashlib
        import tempfile

        with tempfile.TemporaryDirectory() as directory:
            for name in ("craft_mlt_25k.pth", "arabic.pth"):
                (Path(directory) / name).write_bytes(b"x")
            environment = {**self.base, "FORENSIC_OCR_EASYOCR_ENABLED": "true", "FORENSIC_EASYOCR_MODEL_DIR": directory,
                           "FORENSIC_EASYOCR_DETECTOR_SHA256": hashlib.sha256(b"x").hexdigest(), "FORENSIC_EASYOCR_RECOGNIZER_SHA256": "0" * 64}
            with patch.dict("os.environ", environment), \
                    patch("ingestion.forensic_records.ocr_router.TesseractImageOCRProcessor.from_environment", return_value=FakeEngine(result())):
                with self.assertRaises(ModelUnavailableError):
                    OcrRouter.from_environment()


try:
    import cv2  # noqa: F401  (the worker image has OpenCV; the ONNX engine module imports it)
    HAS_CV2 = True
except ImportError:
    HAS_CV2 = False


@unittest.skipUnless(HAS_CV2, "needs OpenCV (runs inside the worker image)")
class LatinRecogniserWiringTests(unittest.TestCase):
    def test_the_router_hands_its_latin_recogniser_and_its_hash_to_the_easyocr_engine(self):
        marker = object()

        class FakeOnnx(FakeEngine):
            recognizers = {"latin": marker}
            model_hashes = {"latin": "a" * 64}

        captured = {}

        def fake_easyocr(**kwargs):
            captured.update(kwargs)
            return FakeEngine(result())

        environment = {"FORENSIC_OCR_LANGUAGES": "eng", "FORENSIC_ONNX_OCR_DETECTOR_PATH": "/x/det.onnx", "FORENSIC_OCR_EASYOCR_ENABLED": "true"}
        with patch.dict("os.environ", environment), \
                patch("ingestion.forensic_records.ocr_router.TesseractImageOCRProcessor.from_environment", return_value=FakeEngine(result())), \
                patch("ingestion.forensic_records.onnx_ocr_processor.OnnxPaddleOCRProcessor.from_environment", return_value=FakeOnnx(result())), \
                patch("ingestion.forensic_records.easyocr_ocr_processor.EasyOcrProcessor.from_environment", side_effect=fake_easyocr):
            router_ = OcrRouter.from_environment()
        self.assertIs(captured["latin_recognizer"], marker)
        self.assertEqual(captured["latin_model_sha256"], "a" * 64)
        self.assertIsNotNone(router_.easyocr)


if __name__ == "__main__":
    unittest.main()
