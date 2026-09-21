from types import SimpleNamespace
import hashlib

from ingestion.forensic_records.multilingual_ocr import (
    PaddleMultilingualOCRProcessor,
    _identifiers,
    _select_candidate,
    _script_and_direction,
)
from ingestion.forensic_records.media_pipeline import (
    ContextualImageProcessor,
    MediaProcessResult,
    READY_WITH_WARNINGS,
    MediaProcessingError,
)


class FakeFrame:
    shape = (400, 800, 3)

    def __getitem__(self, _key):
        return self


class FakeCV2:
    def imread(self, _path):
        return FakeFrame()

    def imencode(self, _extension, _frame):
        return True, bytearray(b"crop")


class Engine:
    def __init__(self, payload):
        self.payload = payload
        self.inputs = []

    def predict(self, **_kwargs):
        self.inputs.append(_kwargs["input"])
        return [self.payload]


def test_extensionless_source_passes_decoded_pixels_and_preserves_bytes(tmp_path):
    source = tmp_path / ("a" * 64)
    source.write_bytes(b"unchanged synthetic source")
    before = hashlib.sha256(source.read_bytes()).hexdigest()
    detector = Engine({"dt_polys": [], "dt_scores": []})
    recognizers = {role: Engine({}) for role in ("english", "arabic_multilingual")}
    processor = PaddleMultilingualOCRProcessor(detector, recognizers, FakeCV2(), model_hashes={})
    result = processor.process_image(source)
    assert isinstance(detector.inputs[0], FakeFrame)
    assert result.metadata["result_state"] == "COMPLETE_ZERO_RESULTS"
    assert not result.observations
    assert all(not engine.inputs for engine in recognizers.values())
    assert hashlib.sha256(source.read_bytes()).hexdigest() == before


def test_bad_decode_fails_before_paddle(tmp_path):
    detector = Engine({})
    processor = PaddleMultilingualOCRProcessor(
        detector, {role: Engine({}) for role in ("english", "arabic_multilingual")},
        SimpleNamespace(imread=lambda _path: None), model_hashes={},
    )
    try:
        processor.process_image(tmp_path / "undecodable")
    except MediaProcessingError:
        pass
    else:
        raise AssertionError("Invalid images must not become completed-zero results")
    assert not detector.inputs


def test_extensionless_and_named_inputs_have_same_ocr_contract(tmp_path):
    processor = PaddleMultilingualOCRProcessor(
        Engine({"dt_polys": [[[0, 0], [100, 0], [100, 40], [0, 40]]], "dt_scores": [0.9]}),
        {role: Engine({"rec_text": "ABC-123", "rec_score": 0.9}) for role in ("english", "arabic_multilingual")},
        FakeCV2(), model_hashes={},
    )
    context = dict(evidence_id="e1", version_id="v1", source_sha256="a" * 64, source_file="original.png")
    named = processor.process_image(tmp_path / "named.png", **context)
    extensionless = processor.process_image(tmp_path / ("a" * 64), **context)
    assert named == extensionless
    assert "inputfix1" in named.observations[0].payload["processor_revision"]


def test_automatic_zero_anpr_falls_back_to_extensionless_ocr(tmp_path):
    zero = MediaProcessResult(modality="image", readiness=READY_WITH_WARNINGS,
                              processor_id="synthetic-anpr", processor_revision="test")
    detector = Engine({"dt_polys": [], "dt_scores": []})
    ocr = PaddleMultilingualOCRProcessor(
        detector, {role: Engine({}) for role in ("english", "arabic_multilingual")},
        FakeCV2(), model_hashes={},
    )
    result = ContextualImageProcessor(SimpleNamespace(process_image=lambda *_a, **_k: zero), ocr).process_image(tmp_path / ("a" * 64))
    assert result.metadata["result_states"] == {"image_anpr": "COMPLETE_ZERO_RESULTS", "ocr": "COMPLETE_ZERO_RESULTS"}
    assert not result.observations
    assert isinstance(detector.inputs[0], FakeFrame)


def test_paddle_multilingual_contract_preserves_unicode_order_and_identifiers(tmp_path):
    source = tmp_path / "mixed.png"
    source.write_bytes(b"image")
    detector = Engine({"dt_polys": [[[10, 20], [300, 20], [300, 70], [10, 70]]], "dt_scores": [0.9]})
    processor = PaddleMultilingualOCRProcessor(
        detector,
        {
            "english": Engine({"rec_text": "Case ABC-123", "rec_score": 0.7}),
            "arabic_multilingual": Engine({"rec_text": "کیس ABC-123", "rec_score": 0.9}),
        },
        FakeCV2(),
        model_hashes={"detector": "a" * 64, "english": "b" * 64, "arabic_multilingual": "c" * 64},
    )
    result = processor.process_image(
        source, evidence_id="e1", version_id="v1", source_sha256="d" * 64,
        source_file="mixed.png",
    )
    payload = result.observations[0].payload
    assert payload["raw_text"] == "کیس ABC-123"
    assert payload["display_direction"] == "mixed"
    assert payload["identifiers_preserved"] == ["ABC-123"]
    assert result.metadata["result_state"] == "COMPLETE_RESULTS"


def test_mixed_script_superset_is_preserved_when_partial_english_has_higher_confidence(tmp_path):
    detector = Engine({"dt_polys": [[[10, 20], [300, 20], [300, 70], [10, 70]]], "dt_scores": [0.9]})
    processor = PaddleMultilingualOCRProcessor(
        detector,
        {
            "english": Engine({"rec_text": "Islamabad", "rec_score": 0.9992083311080933}),
            "arabic_multilingual": Engine({"rec_text": "Islamabad كيس", "rec_score": 0.9571371674537659}),
        },
        FakeCV2(),
        model_hashes={"detector": "a" * 64, "english": "b" * 64, "arabic_multilingual": "c" * 64},
    )
    result = processor.process_image(
        tmp_path / "mixed.png", evidence_id="e1", version_id="v1",
        source_sha256="d" * 64, source_file="mixed.png",
    )

    observation = result.observations[0]
    payload = observation.payload
    assert payload["raw_text"] == "Islamabad كيس"
    assert payload["normalized_text"] == "Islamabad كيس"
    assert payload["script_family"] == "mixed_arabic_latin"
    assert payload["display_direction"] == "mixed"
    assert payload["selected_recognizer"] == "arabic_multilingual"
    assert payload["candidate_selection_reason"] == "mixed_script_superset_within_confidence_tolerance"
    assert payload["mixed_script_candidate_preserved"] is True
    assert observation.confidence == 0.9571371674537659
    assert len(payload["recognizer_candidates"]) == 2
    assert payload["processor_revision"].endswith("inputfix1-mixedfix1")


def test_mixed_script_superset_outside_confidence_tolerance_does_not_override_winner():
    selected, reason = _select_candidate([
        {"recognizer_role": "english", "raw_text": "Islamabad", "confidence": 0.99},
        {"recognizer_role": "arabic_multilingual", "raw_text": "Islamabad كيس", "confidence": 0.939},
    ])
    assert selected["recognizer_role"] == "english"
    assert reason == "highest_recognition_confidence"


def test_non_superset_mixed_candidate_does_not_replace_higher_confidence_text():
    selected, reason = _select_candidate([
        {"recognizer_role": "english", "raw_text": "Reference REF-2026-02", "confidence": 0.99},
        {"recognizer_role": "arabic_multilingual", "raw_text": "رپورٹ REF-2026-02", "confidence": 0.98},
    ])
    assert selected["recognizer_role"] == "english"
    assert reason == "highest_recognition_confidence"


def test_script_direction_does_not_reverse_identifier_text():
    assert _script_and_direction("اردو 03001234567") == ("arabic_derived_urdu_candidate", "rtl")
    assert _identifiers("اردو 03001234567") == ["03001234567"]


def test_contextual_router_honors_explicit_anpr_without_ocr_fallback(tmp_path):
    source = tmp_path / "vehicle.png"
    source.write_bytes(b"image")
    calls = []

    class Processor:
        def __init__(self, role):
            self.role = role

        def process_image(self, _path, **_context):
            calls.append(self.role)
            return MediaProcessResult(
                modality="image",
                readiness=READY_WITH_WARNINGS,
                processor_id=self.role,
                processor_revision="test",
                metadata={"result_state": "COMPLETE_ZERO_RESULTS"},
            )

    result = ContextualImageProcessor(Processor("anpr"), Processor("ocr")).process_image(
        source,
        evidence_id="e1",
        version_id="v1",
        source_sha256="d" * 64,
        source_file="vehicle.png",
        requested_roles=("anpr",),
    )

    assert calls == ["anpr"]
    assert result.metadata["routing_policy"] == "explicit_anpr"
    assert result.metadata["result_states"] == {
        "image_anpr": "COMPLETE_ZERO_RESULTS",
        "ocr": "NOT_RUN",
    }
