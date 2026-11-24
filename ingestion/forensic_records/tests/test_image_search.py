# SPDX-License-Identifier: Apache-2.0
"""M1 image search descriptors: the PDQ-style hash, the processor with fake encoders (no model needed), and that vectors never enter an observation payload."""

import io
import json
import tempfile
import unittest
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw, ImageOps

from ingestion.forensic_records.image_search import (
    IMAGE_HASH_CONTRACT,
    IMAGE_SEARCH_CONTRACTS,
    IMAGE_SEARCH_VECTOR_CONTRACT,
    IMAGE_TILES_CONTRACT,
    ImageSearchAugmentedProcessor,
    ImageSearchProcessor,
    TileObservation,
    VectorObservation,
    tile_layout,
    bits_to_hex,
    bits_to_sql,
    hamming,
    hex_to_bits,
    load_reduced_image,
    pdq_bits,
    persist_image_search_rows,
    vector_literal,
)
from ingestion.forensic_records.media_pipeline import MediaProcessingError, MediaProcessResult, ModelUnavailableError, READY, READY_WITH_WARNINGS


def photo(seed=1, size=(320, 240)):
    rng = np.random.default_rng(seed)
    base = rng.integers(0, 255, (6, 8, 3), dtype=np.uint8)
    image = Image.fromarray(base).resize(size, Image.BICUBIC)
    draw = ImageDraw.Draw(image)
    draw.ellipse((size[0] // 4, size[1] // 4, size[0] * 3 // 4, size[1] * 3 // 4), outline=(250, 250, 250), width=6)
    return image


def jpeg(image, quality):
    buffer = io.BytesIO()
    image.save(buffer, format="JPEG", quality=quality)
    return Image.open(io.BytesIO(buffer.getvalue())).convert("RGB")


class FakeEncoder:
    def __init__(self, kind, value):
        self.kind, self.value = kind, value
        self.model_id, self.model_revision, self.model_sha256 = f"test/{kind}", "r1", "a" * 64
        self.released = 0

    def encode(self, image):
        return [self.value] * 768

    def encode_batch(self, images, batch=12):
        return [[self.value] * 768 for _ in images]

    def release(self):
        self.released += 1


class HashTests(unittest.TestCase):
    def test_hash_is_256_bits_and_deterministic(self):
        image = photo()
        first, second = pdq_bits(image), pdq_bits(image)
        self.assertEqual(first.shape, (256,))
        self.assertTrue(set(np.unique(first)) <= {0, 1})
        self.assertTrue((first == second).all())

    def test_resave_and_resize_stay_close_and_a_different_photo_is_far(self):
        image = photo(1)
        base = bits_to_hex(pdq_bits(image))
        self.assertLessEqual(hamming(base, bits_to_hex(pdq_bits(jpeg(image, 70)))), 40)
        self.assertLessEqual(hamming(base, bits_to_hex(pdq_bits(image.resize((160, 120), Image.LANCZOS)))), 60)
        other = bits_to_hex(pdq_bits(photo(2)))
        self.assertGreater(hamming(base, other), 80)

    def test_lossless_roundtrip_is_distance_zero(self):
        image = photo(3)
        buffer = io.BytesIO()
        image.save(buffer, format="PNG")
        again = Image.open(io.BytesIO(buffer.getvalue())).convert("RGB")
        self.assertEqual(hamming(bits_to_hex(pdq_bits(image)), bits_to_hex(pdq_bits(again))), 0)

    def test_mirror_is_far_so_the_hash_tier_does_not_claim_it(self):
        image = photo(4)
        self.assertGreater(hamming(bits_to_hex(pdq_bits(image)), bits_to_hex(pdq_bits(ImageOps.mirror(image)))), 40)

    def test_hex_and_sql_forms_round_trip(self):
        bits = pdq_bits(photo(5))
        text = bits_to_hex(bits)
        self.assertEqual(len(text), 64)
        self.assertTrue((hex_to_bits(text) == bits).all())
        sql = bits_to_sql(bits)
        self.assertEqual(len(sql), 256)
        self.assertEqual(set(sql) <= {"0", "1"}, True)
        with self.assertRaises(ValueError):
            hex_to_bits("xyz")


class LoadTests(unittest.TestCase):
    def test_large_image_is_reduced_and_orientation_is_applied(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "wide.jpg"
            wide = photo(6, (2400, 1200))
            exif = Image.Exif()
            exif[274] = 6  # rotate 90 degrees clockwise when displayed
            wide.save(path, exif=exif)
            loaded = load_reduced_image(path)
            self.assertEqual(max(loaded.size), 1024)
            self.assertGreater(loaded.size[1], loaded.size[0], "the orientation tag turns the wide picture upright")

    def test_pixel_limit_and_garbage_are_refused(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "p.png"
            photo(7, (200, 100)).save(path)
            with self.assertRaises(MediaProcessingError):
                load_reduced_image(path, max_pixels=1000)
            bad = Path(folder) / "bad.jpg"
            bad.write_bytes(b"not an image at all")
            with self.assertRaises(MediaProcessingError):
                load_reduced_image(bad)


class ProcessorTests(unittest.TestCase):
    def run_processor(self, **kwargs):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "photo.jpg"
            photo(8).save(path)
            processor = ImageSearchProcessor(dino=FakeEncoder("dinov2", 0.036), siglip=FakeEncoder("siglip2", 0.036), **kwargs)
            result = processor.process_image(
                path, evidence_id="e1", version_id="v1", source_sha256="b" * 64, source_file="photo.jpg", locator_prefix=None)
            return processor, result

    def test_three_observations_and_the_vector_is_not_in_any_payload(self):
        processor, result = self.run_processor()
        kinds = [o.contract_version for o in result.observations]
        self.assertEqual(kinds, [IMAGE_HASH_CONTRACT, IMAGE_SEARCH_VECTOR_CONTRACT, IMAGE_SEARCH_VECTOR_CONTRACT])
        for observation in result.observations:
            self.assertIsInstance(observation, VectorObservation)
            self.assertNotIn("embedding", observation.payload)
            self.assertNotIn("vector", observation.payload)
        hash_observation, dino, siglip = result.observations
        self.assertEqual(len(hash_observation.payload["pdq256_hex"]), 64)
        self.assertEqual(hash_observation.vector, ())
        self.assertEqual(len(dino.vector), 768)
        self.assertEqual(dino.payload["embedding_role"], "near_duplicate")
        self.assertEqual(siglip.payload["embedding_role"], "semantic")
        self.assertEqual(dino.payload["embedding_dimension"], 768)
        self.assertEqual(dino.payload["vector_storage"], "forensic.content_embedding")
        self.assertEqual(dino.payload["review_state"], "model_candidate")

    def test_observation_ids_are_deterministic_and_distinct(self):
        _, first = self.run_processor()
        _, second = self.run_processor()
        ids = [o.observation_id for o in first.observations]
        self.assertEqual(ids, [o.observation_id for o in second.observations])
        self.assertEqual(len(set(ids)), 3)

    def test_release_reaches_both_encoders(self):
        processor, _ = self.run_processor()
        processor.release()
        self.assertEqual(processor.dino.released, 1)
        self.assertEqual(processor.siglip.released, 1)

    def test_one_encoder_alone_is_allowed_and_none_is_not(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "photo.jpg"
            photo(9).save(path)
            only_dino = ImageSearchProcessor(dino=FakeEncoder("dinov2", 0.036), siglip=None)
            result = only_dino.process_image(path, evidence_id="e", version_id="v", source_sha256="c" * 64, source_file="photo.jpg")
            self.assertEqual(len(result.observations), 2)
        with self.assertRaises(ModelUnavailableError):
            ImageSearchProcessor(dino=None, siglip=None)

    def test_oversized_input_is_refused_before_decoding(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "photo.jpg"
            photo(10).save(path)
            processor = ImageSearchProcessor(dino=FakeEncoder("dinov2", 0.036), siglip=None, max_input_bytes=10)
            with self.assertRaises(MediaProcessingError):
                processor.process_image(path, evidence_id="e", version_id="v", source_sha256="d" * 64, source_file="photo.jpg")

    def test_locator_prefix_changes_the_identity_so_two_frames_do_not_collide(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "photo.jpg"
            photo(11).save(path)
            processor = ImageSearchProcessor(dino=FakeEncoder("dinov2", 0.036), siglip=None)
            a = processor.process_image(path, evidence_id="e", version_id="v", source_sha256="e" * 64, source_file="clip.mp4", locator_prefix={"timestamp_seconds": 1.0})
            b = processor.process_image(path, evidence_id="e", version_id="v", source_sha256="e" * 64, source_file="clip.mp4", locator_prefix={"timestamp_seconds": 2.0})
            self.assertNotEqual(a.observations[0].observation_id, b.observations[0].observation_id)
            self.assertEqual(a.observations[0].payload["frame_timestamp_seconds"], 1.0)


class RecordingCursor:
    def __init__(self):
        self.calls = []

    def execute(self, sql, params=()):
        self.calls.append((" ".join(sql.split()), params))


class Job:
    tenant_id, collection_id, evidence_id = "t1", "c1", "00000000-0000-4000-8000-000000000001"


class PersistTests(unittest.TestCase):
    def processed(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "photo.jpg"
            photo(12).save(path)
            processor = ImageSearchProcessor(dino=FakeEncoder("dinov2", 0.036), siglip=FakeEncoder("siglip2", 0.036))
            return processor.process_image(path, evidence_id=Job.evidence_id, version_id="v1", source_sha256="f" * 64, source_file="photo.jpg")

    def test_hash_row_is_a_256_character_bit_string_keyed_by_version(self):
        cursor = RecordingCursor()
        result = self.processed()
        persist_image_search_rows(cursor, Job, "11111111-1111-4111-8111-111111111111", result.observations[0], "22222222-2222-4222-8222-222222222222")
        sql, params = cursor.calls[0]
        self.assertIn("INSERT INTO forensic.image_hash", sql)
        self.assertIn("ON CONFLICT", sql)
        self.assertEqual(params[5], "v:11111111-1111-4111-8111-111111111111:full")
        self.assertEqual(len(params[7]), 256)
        self.assertEqual(set(params[7]) <= {"0", "1"}, True)

    def test_vector_row_goes_to_content_embedding_with_model_and_dimension(self):
        cursor = RecordingCursor()
        result = self.processed()
        persist_image_search_rows(cursor, Job, "11111111-1111-4111-8111-111111111111", result.observations[1], "22222222-2222-4222-8222-222222222222")
        sql, params = cursor.calls[0]
        self.assertIn("INSERT INTO forensic.content_embedding", sql)
        self.assertIn("'image'", sql)
        self.assertEqual(params[7], "test/dinov2")
        self.assertEqual(params[9], 768)
        self.assertTrue(params[10].startswith("[") and params[10].endswith("]"))
        self.assertEqual(len(params[10].strip("[]").split(",")), 768)

    def test_a_vector_missing_from_the_observation_is_refused(self):
        result = self.processed()
        from dataclasses import replace
        broken = replace(result.observations[1], vector=())
        with self.assertRaises(MediaProcessingError):
            persist_image_search_rows(RecordingCursor(), Job, "v", broken, "a")

    def test_contract_set_names_exactly_the_search_contracts(self):
        self.assertEqual(IMAGE_SEARCH_CONTRACTS, {IMAGE_HASH_CONTRACT, IMAGE_SEARCH_VECTOR_CONTRACT, IMAGE_TILES_CONTRACT})
        self.assertEqual(vector_literal([0.5, -1.0]), "[0.5,-1]")


class TileTests(unittest.TestCase):
    def tiled(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "photo.jpg"
            photo(21, (640, 480)).save(path)
            processor = ImageSearchProcessor(dino=FakeEncoder("dinov2", 0.036), siglip=FakeEncoder("siglip2", 0.036))
            image = load_reduced_image(path)
            windows = processor.tile_vectors(image)
            result = processor.process_image(path, evidence_id=Job.evidence_id, version_id="v1", source_sha256="f" * 64, source_file="photo.jpg")
        from ingestion.forensic_records.image_search import build_search_result

        encodings = [{"kind": e.kind, "role": "semantic", "semantics": "s", "model_id": e.model_id, "revision": e.model_revision, "sha256": e.model_sha256, "vector": [e.value] * 768} for e in (processor.dino, processor.siglip)]
        built = build_search_result(evidence_id=Job.evidence_id, version_id="v1", source_sha256="f" * 64, source_file="photo.jpg", locator_prefix=None, pdq_hex="0" * 64, width=640, height=480,
                                    encodings=encodings, processor_id="p", processor_revision="r", tiles=windows)
        return windows, result, built

    def test_the_layout_has_thirty_four_windows_inside_the_picture(self):
        layout = tile_layout()
        self.assertEqual(len(layout), 9 + 9 + 16)
        self.assertEqual(len({tile_id for tile_id, _ in layout}), 34)
        for _, (x0, y0, x1, y1) in layout:
            self.assertTrue(0 <= x0 < x1 <= 1.0001 and 0 <= y0 < y1 <= 1.0001)
        self.assertEqual(len(tile_layout("a")), 9)
        self.assertEqual(tile_layout("z"), [])

    def test_every_window_has_a_vector_per_model(self):
        windows, _, _ = self.tiled()
        self.assertEqual(len(windows), 34)
        for window in windows:
            self.assertEqual(sorted(window["vectors"]), ["dinov2", "siglip2"])
            self.assertEqual(len(window["vectors"]["dinov2"]), 768)

    def test_the_plain_search_result_has_no_tile_observation_and_the_tiled_one_has_exactly_one(self):
        _, plain, built = self.tiled()
        self.assertNotIn(IMAGE_TILES_CONTRACT, [o.contract_version for o in plain.observations])
        tile_observations = [o for o in built.observations if o.contract_version == IMAGE_TILES_CONTRACT]
        self.assertEqual(len(tile_observations), 1)
        observation = tile_observations[0]
        self.assertIsInstance(observation, TileObservation)
        self.assertEqual(len(observation.tiles), 34)
        self.assertNotIn("vector", observation.payload)
        self.assertNotIn("tiles", observation.payload)
        self.assertEqual(observation.payload["models"]["dinov2"]["model"], "test/dinov2#tile")
        self.assertEqual(len(json.dumps(observation.payload)) < 4000, True, "the payload carries names and counts, never vectors")

    def test_the_tile_rows_go_to_content_embedding_under_the_tile_model_name(self):
        _, _, built = self.tiled()
        observation = [o for o in built.observations if o.contract_version == IMAGE_TILES_CONTRACT][0]
        cursor = RecordingCursor()
        persist_image_search_rows(cursor, Job, "11111111-1111-4111-8111-111111111111", observation, "22222222-2222-4222-8222-222222222222")
        self.assertEqual(len(cursor.calls), 34 * 2)
        sql, params = cursor.calls[0]
        self.assertIn("INSERT INTO forensic.content_embedding", sql)
        self.assertEqual(params[4], "v:11111111-1111-4111-8111-111111111111:tile:a00")
        self.assertTrue(params[7].endswith("#tile"))
        self.assertEqual(params[9], 768)
        self.assertEqual(json.loads(params[5])["box"], [0.0, 0.0, 0.5, 0.5])
        self.assertEqual({call[1][7] for call in cursor.calls}, {"test/dinov2#tile", "test/siglip2#tile"})


class FakeInner:
    processor_id = "inner"

    def __init__(self):
        self.calls = 0

    def process(self, path, *, modality, **context):
        self.calls += 1
        return MediaProcessResult(modality=modality, readiness=READY, processor_id="inner", processor_revision="1", observations=(), limitations=("inner limitation",), metadata={"result_states": {}})


class FailingSearch:
    def process_image(self, *args, **kwargs):
        raise MediaProcessingError("model files missing")

    def release(self):
        pass


class WrapperTests(unittest.TestCase):
    context = dict(evidence_id="e", version_id="v", source_sha256="a" * 64, source_file="photo.jpg", admitted_metadata={})

    def test_descriptors_are_appended_and_the_inner_result_is_kept(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "photo.jpg"
            photo(13).save(path)
            wrapper = ImageSearchAugmentedProcessor(FakeInner(), ImageSearchProcessor(dino=FakeEncoder("dinov2", 0.036), siglip=FakeEncoder("siglip2", 0.036)), idle_release_seconds=0)
            result = wrapper.process(path, modality="image", **self.context)
            self.assertEqual(len(result.observations), 3)
            self.assertIn("inner limitation", result.limitations)
            self.assertIn("image_search", result.metadata)
            self.assertEqual(wrapper.processor_id, "inner")

    def test_a_search_failure_adds_a_warning_and_does_not_raise(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "photo.jpg"
            photo(14).save(path)
            wrapper = ImageSearchAugmentedProcessor(FakeInner(), FailingSearch(), idle_release_seconds=0)
            result = wrapper.process(path, modality="image", **self.context)
            self.assertEqual(result.readiness, READY_WITH_WARNINGS)
            self.assertTrue(any("model files missing" in item for item in result.limitations))
            self.assertEqual(result.observations, ())

    def test_non_images_pass_straight_through(self):
        inner = FakeInner()
        wrapper = ImageSearchAugmentedProcessor(inner, FailingSearch(), idle_release_seconds=0)
        result = wrapper.process(Path("clip.wav"), modality="audio", **self.context)
        self.assertEqual(result.readiness, READY)
        self.assertEqual(inner.calls, 1)


if __name__ == "__main__":
    unittest.main()
