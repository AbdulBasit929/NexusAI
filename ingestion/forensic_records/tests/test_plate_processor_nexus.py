import sys
import tempfile
from pathlib import Path

import cv2
import numpy as np

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import plate_processor_nexus as pn  # noqa: E402


class FakeDetector:
    model = None

    def __init__(self, found):
        self.found = found

    def detect(self, _frame):
        return self.found


class FakeOcr:
    def __init__(self, text):
        self.text = text

    def predict(self, _crop):
        return type("Read", (), {"text": self.text, "confidence": [0.9] * len(self.text)})()


def _process(found, text, minimum=0.5):
    engine = pn.NexusPlateEngine(FakeDetector(found), FakeOcr(text))
    processor = pn.NexusPlateImageProcessor(engine, cv2, minimum, model_hashes={"detector": "d", "ocr": "o", "ocr_config": "c"})
    with tempfile.TemporaryDirectory() as directory:
        path = Path(directory) / "scene.png"
        cv2.imwrite(str(path), np.full((120, 200, 3), 200, np.uint8))
        return processor.process_image(path, evidence_id="e", version_id="v", source_sha256="s" * 64, source_file="scene.png")


def test_identities_and_registry_verdict_for_a_read_that_fits_a_format():
    result = _process([(20, 30, 120, 70, 0.9)], "LEA-1234")
    assert result.processor_id == "nexus-plates-onnx-cpu" and result.processor_revision == "nexusai-m5-v1"
    assert len(result.observations) == 1
    payload = result.observations[0].payload
    assert payload["normalized_plate_text"] == "LEA1234" and payload["plate_format"] == "L3_S4" and payload["plate_format_flags"] == []
    assert payload["processor"] == "nexus-plates-onnx-cpu" and payload["detector_model"] == "rfdetr-alpr-851dc6b2db27" and payload["ocr_model"] == "cct-s-v2-global-model"
    assert payload["candidate"]["detector_version"].startswith("autolane/rfdetr-alpr@851dc6b2db27")
    assert payload["manual_review_required"] is True and payload["model_provenance"].startswith("provisional")
    assert any("provisional" in item for item in result.limitations)


def test_a_read_that_fits_no_format_is_flagged_and_never_edited():
    payload = _process([(20, 30, 120, 70, 0.9)], "XQ").observations[0].payload
    assert payload["normalized_plate_text"] == "XQ" and payload["plate_format"] is None and payload["plate_format_flags"] == ["format_unknown"]


def test_detections_below_the_configured_threshold_are_omitted_and_counted():
    result = _process([(20, 30, 120, 70, 0.4)], "LEA1234", minimum=0.5)
    assert result.observations == () and any("below the configured threshold" in item for item in result.limitations)


def test_the_incumbent_processor_keeps_its_identity_and_detector_version():
    import media_pipeline as mp

    assert mp.FastALPRImageProcessor.processor_id == "fastalpr-onnx-cpu"
    assert mp.FastALPRImageProcessor.detector_version == "fast-alpr-0.4.0/open-image-models-0.6.0"
    assert pn.NexusPlateImageProcessor.detector_version != mp.FastALPRImageProcessor.detector_version


def test_non_plate_boxes_are_dropped_and_plates_and_close_ups_are_kept():
    import plate_detector_onnx as pdo

    W, H = 1100, 906
    car, plate, neighbour, rickshaw = (65, 62, 1035, 822, 0.94), (515, 570, 678, 655, 0.94), (875, 182, 1100, 628, 0.89), (45, 140, 380, 560, 0.69)
    assert pdo.drop_non_plates([car, plate, neighbour, rickshaw], W, H) == [plate]  # the car contains the plate; the other two are portrait-shaped
    assert pdo.drop_non_plates([(0, 0, 297, 173, 0.92)], 297, 173) == [(0, 0, 297, 173, 0.92)]  # a close-up: fills the frame, contains nothing smaller, aspect 1.7: kept
    assert pdo.drop_non_plates([(10, 10, 60, 160, 0.7)], W, H) == []  # a tall, narrow box is not a plate
    big_wide = (0, 0, 800, 300, 0.9)  # wide enough (aspect 2.7) to be a plate even though it is large, and it holds a smaller detection: kept
    assert big_wide in pdo.drop_non_plates([big_wide, (100, 100, 200, 140, 0.8)], 1100, 906)
    assert pdo.drop_non_plates([], W, H) == []


def test_a_model_file_that_differs_from_its_pin_is_refused(monkeypatch=None):
    import os

    import media_pipeline as mp

    with tempfile.TemporaryDirectory() as directory:
        files = {}
        for name in ("det.onnx", "ocr.onnx", "ocr.yaml"):
            files[name] = Path(directory) / name
            files[name].write_bytes(b"not a model")
        env = {"FORENSIC_ANPR_NEXUS_DETECTOR_MODEL_PATH": str(files["det.onnx"]), "FORENSIC_ANPR_NEXUS_OCR_MODEL_PATH": str(files["ocr.onnx"]),
               "FORENSIC_ANPR_NEXUS_OCR_CONFIG_PATH": str(files["ocr.yaml"]), "FORENSIC_ANPR_NEXUS_DETECTOR_SHA256": "0" * 64}
        previous = {key: os.environ.get(key) for key in env}
        os.environ.update(env)
        try:
            try:
                pn.NexusPlateImageProcessor.from_environment()
            except mp.ModelUnavailableError as error:
                assert "pinned SHA-256" in str(error)
            else:
                raise AssertionError("a file that differs from its pin must be refused")
        finally:
            for key, value in previous.items():
                if value is None:
                    os.environ.pop(key, None)
                else:
                    os.environ[key] = value
