from pathlib import Path
from unittest.mock import patch

from ingestion.forensic_records.media_pipeline import (
    MediaObservation,
    MediaProcessResult,
    DeterministicMediaProcessor,
    READY,
)
from ingestion.forensic_records.video_anpr_parity_adapter import PlateBounds, PlateObservation
from ingestion.forensic_records.video_anpr_product_v2 import (
    ProductAggregationPolicy,
    aggregate_product_observations,
)


def observation(identity, timestamp, raw, confidence=0.8, x=100):
    return PlateObservation(
        observation_id=identity, source_sha256="a" * 64, source_file="video.mp4",
        frame_number=round(timestamp * 60), timestamp_seconds=timestamp,
        source_frame_sha256=identity.rjust(64, "0")[-64:],
        bounds=PlateBounds(x, 100, 120, 40), raw_ocr=raw,
        normalized_ocr="".join(character for character in raw.upper() if character.isalnum()),
        detector_confidence=0.9, ocr_confidence=confidence,
        crop_sha256=identity.rjust(64, "1")[-64:], detector_model_sha256="b" * 64,
        ocr_model_sha256="c" * 64, crop_quality=0.8,
    )


def test_v2_selects_an_actually_observed_candidate_without_character_synthesis():
    items = [
        observation("1", 1.0, "ABC123"),
        observation("2", 1.25, "ABC12B"),
        observation("3", 1.5, "ABC123"),
    ]
    groups = aggregate_product_observations(items, ProductAggregationPolicy())
    assert len(groups) == 1
    assert groups[0].selected_raw_plate in {item.raw_ocr for item in items}
    assert groups[0].selected_normalized_plate == "ABC123"


def test_v2_rejects_singletons_short_noise_and_weak_support():
    items = [
        observation("1", 1.0, "X"),
        observation("2", 2.0, "NOISE9", confidence=0.1),
        observation("3", 3.0, "SOLO7"),
    ]
    assert aggregate_product_observations(items, ProductAggregationPolicy()) == ()


def test_v2_association_is_short_lived_and_source_local():
    items = [
        observation("1", 1.0, "ABC123"),
        observation("2", 5.0, "ABC123"),
    ]
    assert aggregate_product_observations(items, ProductAggregationPolicy()) == ()


def test_v2_owns_video_anpr_without_legacy_resampling(tmp_path: Path):
    source = tmp_path / "clip.mp4"
    source.write_bytes(b"bounded")
    grouped = MediaObservation(
        observation_id="group-1",
        contract_version="forensics.video-anpr-plate-group/v1",
        observation_type="video_anpr_plate_group_observation",
        confidence=None,
        citation_locator={"source_file": "clip.mp4", "start_seconds": 1.0, "end_seconds": 2.0},
        payload={"normalized_plate_text": "ABC123"},
    )

    class FakeV2:
        def process_video(self, _path, **_context):
            return MediaProcessResult(
                modality="video",
                readiness=READY,
                processor_id="v2",
                processor_revision="test",
                observations=(grouped,),
                metadata={"result_states": {"video_anpr": "COMPLETE_RESULTS"}},
            )

    with (
        patch("ingestion.forensic_records.media_pipeline._ffprobe", return_value={"availability": "available"}),
        patch("ingestion.forensic_records.media_pipeline._extract_video_audio", return_value=None),
        patch("ingestion.forensic_records.media_pipeline._sample_video_frames") as legacy_sampler,
    ):
        result = DeterministicMediaProcessor(video_anpr_processor=FakeV2()).process(
            source,
            modality="video",
            evidence_id="evidence-1",
            version_id="version-1",
            source_sha256="f" * 64,
            source_file="clip.mp4",
        )

    legacy_sampler.assert_not_called()
    assert grouped in result.observations
    assert result.metadata["result_states"]["video_anpr"] == "COMPLETE_RESULTS"
