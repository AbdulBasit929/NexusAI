from pathlib import Path
import hashlib
import subprocess
import sys
from types import SimpleNamespace

import pytest

from ingestion.forensic_records.media_pipeline import (
    ANPR_OBSERVATION_CONTRACT,
    AUDIO_TIMESTAMP_SEGMENT_CONTRACT,
    FACE_OBSERVATION_CONTRACT,
    IMAGE_EMBEDDING_CONTRACT,
    IMAGE_OCR_OBSERVATION_CONTRACT,
    VIDEO_ANPR_GROUP_CONTRACT,
    CompositeImageProcessor,
    ContextualImageProcessor,
    MediaObservation,
    MediaProcessResult,
    READY,
    READY_WITH_WARNINGS,
    DeterministicMediaProcessor,
    FastALPRImageProcessor,
    LocalAIFaceProcessor,
    LocalAIASRProcessor,
    SigLIPImageEmbeddingProcessor,
    TesseractImageOCRProcessor,
    MediaProcessingError,
    ModelUnavailableError,
    _ffmpeg_showinfo_timestamps,
    _frame_number_from_probe,
    normalize_plate,
)
from ingestion.forensic_records.image_intelligence import IMAGE_FINGERPRINT_CONTRACT
from ingestion.forensic_records.roman_urdu import ROMAN_URDU_SEGMENT_CONTRACT


class FakeFrame:
    shape = (720, 1280, 3)

    def __getitem__(self, _key):
        return self


class FakeCV2:
    def imread(self, _path):
        return FakeFrame()

    def imencode(self, extension, _frame):
        assert extension == ".png"
        return True, bytearray(b"bounded-crop")


def alpr_result(raw_text, detection_confidence, ocr_confidence, *, region="Australia"):
    return SimpleNamespace(
        detection=SimpleNamespace(
            confidence=detection_confidence,
            bounding_box=SimpleNamespace(x1=100, y1=200, x2=360, y2=280),
        ),
        ocr=SimpleNamespace(text=raw_text, confidence=ocr_confidence, region=region),
    )


def test_plate_normalization_is_formatting_only():
    assert normalize_plate(" mn-13 67 ") == "MN1367"
    assert normalize_plate("O0-B8") == "O0B8"


def test_showinfo_parser_preserves_source_timestamps_instead_of_output_indexes():
    stderr = "\n".join([
        "[Parsed_showinfo_1] n:0 pts:0 pts_time:0 duration:1",
        "[Parsed_showinfo_1] n:1 pts:301 pts_time:5.01667 duration:1",
        "[Parsed_showinfo_1] n:2 pts:602 pts_time:10.0333 duration:1",
    ])
    assert _ffmpeg_showinfo_timestamps(stderr) == [0.0, 5.01667, 10.0333]


def test_frame_number_is_explicitly_derived_from_declared_video_rate():
    probe = {"streams": [{"codec_type": "video", "avg_frame_rate": "60/1"}]}
    assert _frame_number_from_probe(probe, 27.0) == 1620
    assert _frame_number_from_probe({"streams": []}, 27.0) is None


def test_fastalpr_adapter_preserves_raw_prediction_and_ignores_region(tmp_path):
    source = tmp_path / "vehicle.jpg"
    source.write_bytes(b"source")
    engine = SimpleNamespace(predict=lambda _frame: [alpr_result("MN-13 67", 0.8897, [0.8, 0.9])])
    result = FastALPRImageProcessor(engine, FakeCV2()).process_image(
        source,
        evidence_id="evidence-1",
        version_id="version-1",
        source_sha256="a" * 64,
        source_file="vehicle.jpg",
    )

    assert result.readiness == READY
    assert len(result.observations) == 1
    observation = result.observations[0]
    assert observation.contract_version == ANPR_OBSERVATION_CONTRACT
    assert observation.payload["raw_plate_text"] == "MN-13 67"
    assert observation.payload["normalized_plate_text"] == "MN1367"
    assert observation.payload["ocr_confidence"] == pytest.approx(0.85)
    assert observation.payload["region_output_used"] is False
    assert "region" not in observation.payload
    assert observation.payload["candidate"]["review_state"] == "model_candidate"
    assert observation.payload["crop"]["storage_state"] == "reconstructable_from_retained_source"


def test_fastalpr_adapter_reports_no_result_truthfully(tmp_path):
    source = tmp_path / "empty.jpg"
    source.write_bytes(b"source")
    engine = SimpleNamespace(predict=lambda _frame: [alpr_result("BAD", 0.4, [0.9])])
    result = FastALPRImageProcessor(engine, FakeCV2()).process_image(
        source,
        evidence_id="evidence-1",
        version_id="version-1",
        source_sha256="b" * 64,
        source_file="empty.jpg",
    )

    assert result.readiness == READY_WITH_WARNINGS
    assert result.observations == ()
    assert any("below the configured threshold" in item for item in result.limitations)
    assert any("no plate observation" in item for item in result.limitations)


def test_fastalpr_candidate_identity_includes_video_frame_locator(tmp_path):
    source = tmp_path / "frame.png"
    source.write_bytes(b"source")
    engine = SimpleNamespace(predict=lambda _frame: [alpr_result("MN1367", 0.9, [0.9])])
    processor = FastALPRImageProcessor(engine, FakeCV2())
    common = {
        "evidence_id": "evidence-1",
        "version_id": "version-1",
        "source_sha256": "f" * 64,
        "source_file": "clip.mp4",
    }

    first = processor.process_image(source, **common, locator_prefix={"timestamp_seconds": 0.0})
    second = processor.process_image(source, **common, locator_prefix={"timestamp_seconds": 5.0})

    assert first.observations[0].observation_id != second.observations[0].observation_id
    assert first.observations[0].citation_locator["timestamp_seconds"] == 0.0
    assert second.observations[0].citation_locator["timestamp_seconds"] == 5.0


def test_image_foundation_remains_useful_when_model_role_is_unavailable(tmp_path):
    source = tmp_path / "scene.png"
    source.write_bytes(b"source")
    result = DeterministicMediaProcessor().process(
        source,
        modality="image",
        evidence_id="evidence-1",
        version_id="version-1",
        source_sha256="c" * 64,
        source_file="scene.png",
        admitted_metadata={"width_pixels": 640, "height_pixels": 480},
    )

    assert result.readiness == READY_WITH_WARNINGS
    assert result.observations[0].payload["preview_source"] == "retained_evidence"
    assert result.observations[0].payload["source_metadata"]["width_pixels"] == 640
    assert result.observations[1].contract_version == IMAGE_FINGERPRINT_CONTRACT
    assert result.observations[1].payload["source_sha256"] == "c" * 64
    assert "vision or ANPR" in result.limitations[0]


def test_fastalpr_configuration_refuses_implicit_model_download(monkeypatch):
    for name in (
        "FORENSIC_ANPR_DETECTOR_MODEL_PATH",
        "FORENSIC_ANPR_OCR_MODEL_PATH",
        "FORENSIC_ANPR_OCR_CONFIG_PATH",
    ):
        monkeypatch.delenv(name, raising=False)
    with pytest.raises(ModelUnavailableError, match="implicit model download is disabled"):
        FastALPRImageProcessor.from_environment()


def test_localai_face_processor_emits_scoped_detection_and_embedding_contract(tmp_path):
    source = tmp_path / "face.png"
    source.write_bytes(b"synthetic-face")
    processor = LocalAIFaceProcessor(
        "http://127.0.0.1:8080/v1/face",
        "face-detect-yunet-sface",
        FakeCV2(),
        model_sha256="a" * 64,
        backend_digest="sha256:" + "b" * 64,
    )

    def post(operation, _payload):
        if operation == "detect":
            return {"faces": [{"region": {"x": 100.2, "y": 200.4, "w": 259.1, "h": 79.2}, "confidence": 0.95}]}
        assert operation == "embed"
        return {"embedding": [0.125] * 128, "dim": 128, "model": "face-detect-yunet-sface"}

    processor._post_json = post
    result = processor.process_image(
        source,
        evidence_id="evidence-1",
        version_id="version-1",
        source_sha256="c" * 64,
        source_file="synthetic-face.png",
        locator_prefix={"timestamp_seconds": 5.0},
    )

    assert result.readiness == READY
    assert len(result.observations) == 1
    observation = result.observations[0]
    assert observation.contract_version == FACE_OBSERVATION_CONTRACT
    assert observation.observation_type == "face_detection_observation"
    assert observation.payload["parent_evidence_id"] == "evidence-1"
    assert observation.payload["parent_version_id"] == "version-1"
    assert observation.payload["frame_timestamp_seconds"] == 5.0
    assert observation.payload["detector_version"] == "nexusai-post-bf-a-v1"
    assert observation.payload["embedding_version"] == "nexusai-post-bf-a-v1"
    assert observation.payload["embedding_dimension"] == 128
    assert observation.payload["embedding_status"] == "available"
    assert observation.payload["face_crop_artifact"]["storage_state"] == "reconstructable_from_retained_source"
    assert observation.payload["similarity_semantics"] == "candidate visual similarity; not identity"
    assert "identity" in observation.warnings[1].lower()


def test_localai_face_processor_preserves_truthful_no_face_result(tmp_path):
    source = tmp_path / "street.png"
    source.write_bytes(b"synthetic-street")
    processor = LocalAIFaceProcessor(
        "http://127.0.0.1:8080/v1/face", "face-detect-yunet-sface", FakeCV2()
    )
    processor._post_json = lambda operation, _payload: {"faces": []}

    result = processor.process_image(
        source,
        evidence_id="evidence-1",
        version_id="version-1",
        source_sha256="d" * 64,
        source_file="synthetic-street.png",
    )

    assert result.readiness == READY
    assert result.observations == ()
    assert result.limitations == ("no faces were detected",)


def test_composite_image_processor_keeps_role_contracts_separate(tmp_path):
    source = tmp_path / "image.png"
    source.write_bytes(b"source")

    class StubProcessor:
        def __init__(self, processor_id, observation_type):
            self.processor_id = processor_id
            self.observation_type = observation_type

        def process_image(self, _path, **_context):
            return SimpleNamespace(
                readiness=READY,
                processor_id=self.processor_id,
                observations=(MediaObservation(
                    observation_id=self.processor_id,
                    contract_version="test/v1",
                    observation_type=self.observation_type,
                    confidence=None,
                    citation_locator={},
                    payload={},
                ),),
                limitations=(),
                metadata={"role": self.processor_id},
            )

    result = CompositeImageProcessor((
        StubProcessor("anpr", "anpr_ocr_observation"),
        StubProcessor("face", "face_detection_observation"),
    )).process_image(source)

    assert [item.observation_type for item in result.observations] == [
        "anpr_ocr_observation",
        "face_detection_observation",
    ]
    assert result.metadata == {
        "anpr": {"role": "anpr"},
        "face": {"role": "face"},
    }


def test_composite_image_processor_isolates_a_failed_optional_role(tmp_path):
    source = tmp_path / "image.png"
    source.write_bytes(b"source")

    class Healthy:
        processor_id = "healthy"

        def process_image(self, _path, **_context):
            return SimpleNamespace(
                readiness=READY,
                processor_id=self.processor_id,
                observations=(),
                limitations=(),
                metadata={"ok": True},
            )

    class Failed:
        processor_id = "failed"

        def process_image(self, _path, **_context):
            raise MediaProcessingError("bounded failure")

    result = CompositeImageProcessor((Healthy(), Failed())).process_image(source)

    assert result.readiness == READY_WITH_WARNINGS
    assert result.metadata == {"healthy": {"ok": True}}
    assert result.limitations == ("failed unavailable: bounded failure",)


def test_contextual_anpr_ocr_uses_plate_detection_as_automatic_gate(tmp_path):
    source = tmp_path / "scene.png"
    source.write_bytes(b"source")
    calls = []

    class Role:
        def __init__(self, name, observations):
            self.processor_id = name
            self.observations = observations

        def process_image(self, _path, **_context):
            calls.append(self.processor_id)
            return MediaProcessResult(
                modality="image", readiness=READY, processor_id=self.processor_id,
                processor_revision="test", observations=self.observations,
                metadata={"result_state": "COMPLETE_RESULTS" if self.observations else "COMPLETE_ZERO_RESULTS"},
            )

    plate = MediaObservation("plate", ANPR_OBSERVATION_CONTRACT, "anpr_ocr_observation", .8, {}, {})
    result = ContextualImageProcessor(Role("anpr", (plate,)), Role("ocr", ())).process_image(source)
    assert calls == ["anpr"]
    assert result.metadata["result_states"] == {"image_anpr": "COMPLETE_RESULTS", "ocr": "NOT_RUN"}


def test_tesseract_general_ocr_preserves_raw_text_boxes_and_role_boundary(monkeypatch, tmp_path):
    tessdata = tmp_path / "tessdata"
    tessdata.mkdir()
    hashes = {}
    for language in ("eng", "urd"):
        payload = f"{language}-traineddata".encode()
        (tessdata / f"{language}.traineddata").write_bytes(payload)
        hashes[language] = hashlib.sha256(payload).hexdigest()
    tsv = (
        "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n"
        "5\t1\t1\t1\t1\t1\t10\t20\t40\t15\t92\tNEXUS\n"
        "5\t1\t1\t1\t1\t2\t55\t20\t35\t15\t88\tAI\n"
        "5\t1\t1\t1\t2\t1\t10\t50\t70\t20\t81\tپاکستان\n"
    ).encode()

    def fake_run(args, **_kwargs):
        if args[-1] == "--version":
            return subprocess.CompletedProcess(args, 0, b"tesseract 5.3.0\n", b"")
        return subprocess.CompletedProcess(args, 0, tsv, b"")

    monkeypatch.setattr("ingestion.forensic_records.media_pipeline.subprocess.run", fake_run)
    processor = TesseractImageOCRProcessor(
        Path(sys.executable),
        tessdata,
        tessdata_revision="revision-1",
        tessdata_hashes=hashes,
    )
    source = tmp_path / "scene.png"
    source.write_bytes(b"synthetic")
    result = processor.process_image(
        source,
        evidence_id="evidence-1",
        version_id="version-1",
        source_sha256="a" * 64,
        source_file="scene.png",
    )

    assert result.readiness == READY
    assert len(result.observations) == 2
    english, urdu = result.observations
    assert english.contract_version == IMAGE_OCR_OBSERVATION_CONTRACT
    assert english.payload["raw_text"] == "NEXUS AI"
    assert english.payload["bbox"] == {"x": 10, "y": 20, "width": 80, "height": 15}
    assert english.payload["script_family"] == "latin_script_candidate"
    assert urdu.payload["raw_text"] == "پاکستان"
    assert urdu.payload["script_family"] == "arabic_script_candidate"
    assert urdu.payload["language_claim"] == "undetermined"
    assert urdu.payload["role"] == "general_scene_text_only_not_plate_ocr"
    assert "does not replace FastALPR" in result.limitations[0]


def test_siglip_processor_rejects_unpinned_or_mismatched_weights(tmp_path):
    model_path = tmp_path / "siglip"
    model_path.mkdir()
    (model_path / "model.safetensors").write_bytes(b"weights")

    with pytest.raises(ModelUnavailableError, match="checksum does not match"):
        SigLIPImageEmbeddingProcessor(
            model_path,
            model_id="google/siglip-base-patch16-224",
            model_revision="revision-1",
            model_sha256="a" * 64,
        )


def test_image_model_roles_refuse_implicit_download(monkeypatch):
    monkeypatch.delenv("FORENSIC_IMAGE_EMBEDDING_MODEL_PATH", raising=False)
    with pytest.raises(ModelUnavailableError, match="implicit model download is disabled"):
        SigLIPImageEmbeddingProcessor.from_environment()


def test_audio_foundation_composes_timestamped_asr_observations(monkeypatch, tmp_path):
    source = tmp_path / "interview.wav"
    source.write_bytes(b"bounded-audio")
    monkeypatch.setattr("ingestion.forensic_records.media_pipeline._ffprobe", lambda _path: {"availability": "available"})

    class FakeTranscriber:
        def transcribe(self, _path, *, evidence_id, version_id, source_file, language=None):
            assert (evidence_id, version_id, source_file) == ("evidence-1", "version-1", "interview.wav")
            assert language is None
            return (MediaObservation(
                observation_id="segment-1",
                contract_version=AUDIO_TIMESTAMP_SEGMENT_CONTRACT,
                observation_type="audio_transcript_segment",
                confidence=None,
                citation_locator={"source_file": source_file, "start_seconds": 1.0, "end_seconds": 2.0},
                payload={"text": "bounded transcript"},
            ),)

    result = DeterministicMediaProcessor(audio_transcriber=FakeTranscriber()).process(
        source,
        modality="audio",
        evidence_id="evidence-1",
        version_id="version-1",
        source_sha256="d" * 64,
        source_file="interview.wav",
    )

    assert len(result.observations) == 2
    assert result.observations[1].citation_locator["start_seconds"] == 1.0
    assert result.metadata["result_states"]["asr"] == "COMPLETE_RESULTS"
    assert all("ASR model is not configured" not in limitation for limitation in result.limitations)


def test_audio_foundation_distinguishes_asr_not_run_from_completed_zero(monkeypatch, tmp_path):
    source = tmp_path / "silent.wav"
    source.write_bytes(b"bounded-audio")
    monkeypatch.setattr("ingestion.forensic_records.media_pipeline._ffprobe", lambda _path: {})

    not_run = DeterministicMediaProcessor().process(
        source,
        modality="audio",
        evidence_id="evidence-1",
        version_id="version-1",
        source_sha256="a" * 64,
        source_file="silent.wav",
    )

    class EmptyTranscriber:
        def transcribe(self, *_args, **_kwargs):
            return ()

    completed_zero = DeterministicMediaProcessor(audio_transcriber=EmptyTranscriber()).process(
        source,
        modality="audio",
        evidence_id="evidence-1",
        version_id="version-1",
        source_sha256="a" * 64,
        source_file="silent.wav",
    )

    assert not_run.metadata["result_states"]["asr"] == "NOT_RUN"
    assert completed_zero.metadata["result_states"]["asr"] == "COMPLETE_ZERO_RESULTS"


def test_video_embedded_audio_preserves_unique_ids_and_segment_times(monkeypatch, tmp_path):
    source = tmp_path / "clip.mp4"
    source.write_bytes(b"bounded-video")
    embedded = tmp_path / "embedded.wav"
    embedded.write_bytes(b"bounded-audio")
    monkeypatch.setattr("ingestion.forensic_records.media_pipeline._ffprobe", lambda _path: {"availability": "available"})
    monkeypatch.setattr("ingestion.forensic_records.media_pipeline._extract_video_audio", lambda _path: embedded)
    monkeypatch.setattr("ingestion.forensic_records.media_pipeline._sample_video_frames", lambda _path, **_options: [])

    class FakeTranscriber:
        def transcribe(self, _path, *, evidence_id, version_id, source_file, language=None):
            assert language == "ur"
            return (MediaObservation(
                observation_id="segment-1",
                contract_version=AUDIO_TIMESTAMP_SEGMENT_CONTRACT,
                observation_type="audio_transcript_segment",
                confidence=None,
                citation_locator={"source_file": source_file, "start_seconds": 1.25, "end_seconds": 2.5},
                payload={"text": "bounded transcript"},
            ),)

    result = DeterministicMediaProcessor(audio_transcriber=FakeTranscriber()).process(
        source,
        modality="video",
        evidence_id="evidence-1",
        version_id="version-1",
        source_sha256="e" * 64,
        source_file="clip.mp4",
        admitted_metadata={"asr_language": "ur"},
    )

    embedded_observations = [
        item for item in result.observations if item.observation_type.startswith("video_embedded_audio_")
    ]
    assert len(embedded_observations) == 2
    assert len({item.observation_id for item in embedded_observations}) == 2
    assert embedded_observations[1].contract_version == AUDIO_TIMESTAMP_SEGMENT_CONTRACT
    assert embedded_observations[1].citation_locator == {
        "source_file": "clip.mp4",
        "start_seconds": 1.25,
        "end_seconds": 2.5,
    }
    assert all("audio transcription is unavailable" not in item for item in result.limitations)


def test_video_does_not_report_global_zero_when_a_later_sample_has_a_result(monkeypatch, tmp_path):
    source = tmp_path / "clip.mp4"
    source.write_bytes(b"bounded-video")
    first = tmp_path / "frame-0.png"
    second = tmp_path / "frame-5.png"
    first.write_bytes(b"frame-zero")
    second.write_bytes(b"frame-positive")
    monkeypatch.setattr("ingestion.forensic_records.media_pipeline._ffprobe", lambda _path: {"availability": "available"})
    monkeypatch.setattr("ingestion.forensic_records.media_pipeline._extract_video_audio", lambda _path: None)
    monkeypatch.setattr(
        "ingestion.forensic_records.media_pipeline._sample_video_frames",
        lambda _path, **_options: [(first, 0.0), (second, 5.0)],
    )

    class FrameProcessor:
        def process_image(self, _path, **context):
            timestamp = context["locator_prefix"]["timestamp_seconds"]
            if timestamp == 0.0:
                return SimpleNamespace(
                    observations=(),
                    limitations=("no plate observation passed the configured detection threshold",),
                )
            return SimpleNamespace(
                observations=(MediaObservation(
                    observation_id="plate-1",
                    contract_version=ANPR_OBSERVATION_CONTRACT,
                    observation_type="anpr_ocr_observation",
                    confidence=0.9,
                    citation_locator={"timestamp_seconds": timestamp},
                    payload={"normalized_plate_text": "LN15ZZC"},
                ),),
                limitations=(),
            )

    result = DeterministicMediaProcessor(image_processor=FrameProcessor()).process(
        source,
        modality="video",
        evidence_id="evidence-1",
        version_id="version-1",
        source_sha256="f" * 64,
        source_file="clip.mp4",
    )

    assert any(item.observation_type == "anpr_ocr_observation" for item in result.observations)
    assert all("no plate observation" not in limitation for limitation in result.limitations)
    group = next(item for item in result.observations if item.contract_version == VIDEO_ANPR_GROUP_CONTRACT)
    assert group.payload == {
        "normalized_plate_text": "LN15ZZC",
        "sightings_count": 1,
        "first_seen_seconds": 5.0,
        "last_seen_seconds": 5.0,
        "best_observation_id": "plate-1",
        "all_observation_ids": ["plate-1"],
        "grouping_semantics": "same normalized model observation; not object tracking",
        "parent_evidence_id": "evidence-1",
        "parent_version_id": "version-1",
        "processor": "nexusai-video-anpr-grouping",
        "processor_revision": "mmv2-v1",
        "manual_review_required": True,
    }


@pytest.mark.parametrize("language", ["ur", "en"])
def test_localai_asr_forwards_governed_language(monkeypatch, tmp_path, language):
    source = tmp_path / "speech.wav"
    source.write_bytes(b"bounded-audio")
    captured = {}

    class Response:
        def __enter__(self):
            return self

        def __exit__(self, *_args):
            return False

        def read(self):
            return b'{"language":"ur","segments":[{"start":1.25,"end":2.5,"text":"bounded"}]}'

    def urlopen(request, timeout):
        captured["body"] = request.data
        captured["timeout"] = timeout
        return Response()

    monkeypatch.setattr("ingestion.forensic_records.media_pipeline.urllib.request.urlopen", urlopen)
    observations = LocalAIASRProcessor("http://127.0.0.1:8080/v1/audio/transcriptions", "small").transcribe(
        source,
        evidence_id="evidence-1",
        version_id="version-1",
        source_file="speech.wav",
        language=language,
    )

    assert f'\r\n\r\n{language}\r\n'.encode() in captured["body"]
    assert observations[0].payload["requested_language"] == language
    assert observations[0].payload["language"] == "ur"
    assert observations[0].payload["detected_language"] is None
    assert observations[0].payload["detected_language_probability"] is None
    assert observations[0].citation_locator["end_seconds"] == 2.5


def test_localai_asr_adds_identifier_safe_roman_urdu_derivative(monkeypatch, tmp_path):
    source = tmp_path / "speech.wav"
    source.write_bytes(b"bounded-audio")

    class Response:
        def __enter__(self):
            return self

        def __exit__(self, *_args):
            return False

        def read(self):
            return '{"language":"ur","segments":[{"start":0,"end":2,"text":"اس آڈیو میں 03001234567 کا ذکر ہے"}]}'.encode()

    monkeypatch.setattr(
        "ingestion.forensic_records.media_pipeline.urllib.request.urlopen",
        lambda _request, timeout: Response(),
    )
    observations = LocalAIASRProcessor(
        "http://127.0.0.1:8080/v1/audio/transcriptions", "small"
    ).transcribe(
        source,
        evidence_id="evidence-1",
        version_id="version-1",
        source_file="speech.wav",
        language="ur",
    )

    assert len(observations) == 2
    assert observations[0].contract_version == AUDIO_TIMESTAMP_SEGMENT_CONTRACT
    assert observations[1].contract_version == ROMAN_URDU_SEGMENT_CONTRACT
    assert observations[1].payload["raw_urdu_text"] == observations[0].payload["text"]
    assert "03001234567" in observations[1].payload["roman_urdu_text"]
    assert observations[1].payload["identifier_preservation_pass"] is True
    assert observations[0].payload["language"] == "ur"
    assert observations[1].citation_locator["end_seconds"] == 2.0


@pytest.mark.parametrize(
    ("text", "message"),
    [
        ("यह देवनागरी है", "returned Devanagari script"),
        ("latin-only output", "did not return Urdu script"),
    ],
)
def test_localai_asr_rejects_non_urdu_script_for_trusted_urdu_hint(
    monkeypatch, tmp_path, text, message
):
    source = tmp_path / "speech.wav"
    source.write_bytes(b"bounded-audio")

    class Response:
        def __enter__(self):
            return self

        def __exit__(self, *_args):
            return False

        def read(self):
            return json.dumps({
                "language": "ur",
                "segments": [{"start": 0, "end": 1, "text": text}],
            }).encode()

    monkeypatch.setattr(
        "ingestion.forensic_records.media_pipeline.urllib.request.urlopen",
        lambda _request, timeout: Response(),
    )
    processor = LocalAIASRProcessor(
        "http://127.0.0.1:8080/v1/audio/transcriptions", "small"
    )

    with pytest.raises(MediaProcessingError, match=message):
        processor.transcribe(
            source,
            evidence_id="evidence-1",
            version_id="version-1",
            source_file="speech.wav",
            language="ur",
        )


def test_localai_asr_omits_language_for_automatic_detection(monkeypatch, tmp_path):
    source = tmp_path / "speech.wav"
    source.write_bytes(b"bounded-audio")
    captured = {}

    class Response:
        def __enter__(self):
            return self

        def __exit__(self, *_args):
            return False

        def read(self):
            return b'{"language":"hi","segments":[{"start":0,"end":1,"text":"bounded"}]}'

    def urlopen(request, timeout):
        captured["request"] = request
        return Response()
    monkeypatch.setattr("ingestion.forensic_records.media_pipeline.urllib.request.urlopen", urlopen)

    observations = LocalAIASRProcessor("http://127.0.0.1:8080/v1/audio/transcriptions", "small").transcribe(
        source,
        evidence_id="evidence-1",
        version_id="version-1",
        source_file="speech.wav",
    )

    assert b'name="language"' not in captured["request"].data
    assert observations[0].payload["requested_language"] is None
    assert observations[0].payload["detected_language"] == "hi"


def test_media_language_policy_rejects_invalid_identifier(monkeypatch, tmp_path):
    source = tmp_path / "speech.wav"
    source.write_bytes(b"bounded-audio")
    monkeypatch.setattr("ingestion.forensic_records.media_pipeline._ffprobe", lambda _path: {})

    with pytest.raises(MediaProcessingError, match="two- or three-letter"):
        DeterministicMediaProcessor(audio_transcriber=SimpleNamespace()).process(
            source,
            modality="audio",
            evidence_id="evidence-1",
            version_id="version-1",
            source_sha256="f" * 64,
            source_file="speech.wav",
            admitted_metadata={"asr_language": "not-a-language"},
        )
