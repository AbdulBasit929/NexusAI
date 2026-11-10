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
