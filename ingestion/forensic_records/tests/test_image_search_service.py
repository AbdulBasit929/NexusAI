# SPDX-License-Identifier: Apache-2.0
"""The query-side service of image search with fake encoders: authorisation, the two calls, bounds, and that nothing is kept."""

import io
import json
import threading
import unittest
import urllib.error
import urllib.request
from http.server import ThreadingHTTPServer

import numpy as np
from PIL import Image

import os
import tempfile
from pathlib import Path
from unittest import mock

from ingestion.forensic_records.image_search import ImageSearchProcessor, RemoteImageSearchProcessor, search_processor_from_environment
from ingestion.forensic_records.media_pipeline import MediaProcessingError, ModelUnavailableError
from ingestion.forensic_records.image_search_service import ImageSearchService, make_handler


class FakeVision:
    def __init__(self, kind):
        self.kind, self.model_id, self.model_revision, self.model_sha256 = kind, f"test/{kind}", "r", "a" * 64
        self.released = 0

    def encode(self, image):
        return [0.5] * 768

    def encode_batch(self, images, batch=12):
        return [[0.5] * 768 for _ in images]

    def release(self):
        self.released += 1


class FakeText:
    def __init__(self):
        self.released = 0
        self.seen = []

    def encode(self, texts):
        self.seen.extend(texts)
        return [[0.25] * 768 for _ in texts]

    def release(self):
        self.released += 1


def png_bytes(size=(64, 48)):
    buffer = io.BytesIO()
    Image.fromarray(np.random.default_rng(1).integers(0, 255, (size[1], size[0], 3), dtype=np.uint8)).save(buffer, format="PNG")
    return buffer.getvalue()


class ServiceTests(unittest.TestCase):
    def setUp(self):
        self.text = FakeText()
        self.vision = (FakeVision("dinov2"), FakeVision("siglip2"))
        self.service = ImageSearchService(ImageSearchProcessor(dino=self.vision[0], siglip=self.vision[1]), self.text, idle_release_seconds=0)
        self.server = ThreadingHTTPServer(("127.0.0.1", 0), make_handler(self.service, "secret-key"))
        self.port = self.server.server_address[1]
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()

    def call(self, path, body=b"", key="secret-key", method="POST"):
        request = urllib.request.Request(f"http://127.0.0.1:{self.port}{path}", data=body if method == "POST" else None, method=method)
        if key is not None:
            request.add_header("X-Image-Search-Key", key)
        try:
            with urllib.request.urlopen(request, timeout=10) as response:
                return response.status, json.loads(response.read().decode("utf-8"))
        except urllib.error.HTTPError as error:
            return error.code, json.loads(error.read().decode("utf-8"))

    def test_health_needs_no_key_but_the_calls_do(self):
        self.assertEqual(self.call("/healthz", method="GET", key=None)[0], 200)
        self.assertEqual(self.call("/embed-text", json.dumps({"text": "a dog"}).encode(), key=None)[0], 401)
        self.assertEqual(self.call("/embed-text", json.dumps({"text": "a dog"}).encode(), key="wrong")[0], 401)

    def test_embed_text_returns_a_vector_and_the_model(self):
        status, body = self.call("/embed-text", json.dumps({"text": "A Dog On A Beach"}).encode())
        self.assertEqual(status, 200)
        self.assertEqual(body["dimension"], 768)
        self.assertEqual(len(body["vector"]), 768)
        self.assertEqual(body["model"], "test/siglip2")
        self.assertEqual(self.text.seen, ["A Dog On A Beach"])

    def test_embed_text_refuses_an_empty_body_and_bad_json(self):
        self.assertEqual(self.call("/embed-text", b"")[0], 413)
        self.assertEqual(self.call("/embed-text", b"not json")[0], 400)

    def test_embed_image_returns_hash_and_both_vectors(self):
        status, body = self.call("/embed-image", png_bytes())
        self.assertEqual(status, 200)
        self.assertEqual(len(body["pdq256_hex"]), 64)
        self.assertEqual(sorted(body["vectors"]), ["dinov2", "siglip2"])
        self.assertEqual(len(body["vectors"]["dinov2"]), 768)
        self.assertEqual(body["models"]["siglip2"]["model"], "test/siglip2")

    def test_windows_are_returned_only_when_asked_for(self):
        _, plain = self.call("/embed-image", png_bytes((320, 240)))
        self.assertNotIn("tiles", plain)
        status, body = self.call("/embed-image?tiles=1", png_bytes((320, 240)))
        self.assertEqual(status, 200)
        self.assertEqual(len(body["tiles"]), 34)
        first = body["tiles"][0]
        self.assertEqual((first["id"], first["box"]), ("a00", [0.0, 0.0, 0.5, 0.5]))
        self.assertEqual(sorted(first["vectors"]), ["dinov2", "siglip2"])
        self.assertEqual(len(first["vectors"]["siglip2"]), 768)

    def test_crop_cuts_a_drawn_box_out_of_a_picture(self):
        import base64

        body = json.dumps({"b64": base64.b64encode(png_bytes((200, 100))).decode(), "box": [0.0, 0.0, 0.5, 0.5]}).encode()
        status, out = self.call("/crop", body)
        self.assertEqual((status, out["width"], out["height"]), (200, 100, 50))
        self.assertTrue(base64.b64decode(out["png_b64"]).startswith(b"\x89PNG"))
        bad = json.dumps({"b64": base64.b64encode(png_bytes((200, 100))).decode(), "box": [0, 0, 0.01, 0.01]}).encode()
        self.assertEqual(self.call("/crop", bad)[0], 400)
        self.assertEqual(self.call("/crop", json.dumps({"b64": "!!", "box": [0, 0, 1, 1]}).encode())[0], 400)

    def test_garbage_is_not_an_image(self):
        status, body = self.call("/embed-image", b"this is not an image")
        self.assertEqual(status, 400)
        self.assertIn("decoded", body["error"])

    def test_unknown_path_is_404_and_release_reaches_every_model(self):
        self.assertEqual(self.call("/nothing", b"{}")[0], 404)
        self.service._release()
        self.assertEqual((self.vision[0].released, self.vision[1].released, self.text.released), (1, 1, 1))

    def test_release_hands_the_freed_heap_back_to_the_operating_system(self):
        with mock.patch("ingestion.forensic_records.image_search_service.trim_memory") as trim:
            self.service._release()
        trim.assert_called_once_with()

    def test_the_trim_never_raises_where_it_cannot_run(self):
        from ingestion.forensic_records import image_search_service

        with mock.patch.object(image_search_service.ctypes, "CDLL", side_effect=OSError("no libc.so.6")):
            image_search_service.trim_memory()


class RemoteProcessorTests(unittest.TestCase):
    def setUp(self):
        self.local = ImageSearchProcessor(dino=FakeVision("dinov2"), siglip=FakeVision("siglip2"))
        self.service = ImageSearchService(ImageSearchProcessor(dino=FakeVision("dinov2"), siglip=FakeVision("siglip2")), FakeText(), idle_release_seconds=0)
        self.server = ThreadingHTTPServer(("127.0.0.1", 0), make_handler(self.service, "secret-key"))
        self.url = f"http://127.0.0.1:{self.server.server_address[1]}"
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.folder = tempfile.TemporaryDirectory()
        self.path = Path(self.folder.name) / "photo.png"
        self.path.write_bytes(png_bytes((120, 90)))

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.folder.cleanup()

    def context(self):
        return dict(evidence_id="e1", version_id="v1", source_sha256="a" * 64, source_file="photo.png")

    def test_remote_rows_are_identical_to_the_local_rows(self):
        remote = RemoteImageSearchProcessor(self.url, "secret-key")
        a = remote.process_image(self.path, **self.context())
        b = self.local.process_image(self.path, **self.context())
        self.assertEqual([o.observation_id for o in a.observations], [o.observation_id for o in b.observations])
        self.assertEqual([o.payload for o in a.observations], [o.payload for o in b.observations])
        self.assertEqual([o.vector for o in a.observations], [o.vector for o in b.observations])
        self.assertEqual(a.metadata, b.metadata)
        self.assertEqual(a.observations[0].payload["image_width"], 120)

    def test_a_wrong_key_or_a_dead_service_is_a_processing_error_not_a_crash(self):
        with self.assertRaises(MediaProcessingError) as wrong:
            RemoteImageSearchProcessor(self.url, "not-the-key").process_image(self.path, **self.context())
        self.assertIn("401", str(wrong.exception))
        with self.assertRaises(MediaProcessingError):
            RemoteImageSearchProcessor("http://127.0.0.1:9", "k", timeout_seconds=2).process_image(self.path, **self.context())

    def test_a_garbage_image_is_refused_by_the_service(self):
        bad = Path(self.folder.name) / "bad.jpg"
        bad.write_bytes(b"not an image")
        with self.assertRaises(MediaProcessingError) as error:
            RemoteImageSearchProcessor(self.url, "secret-key").process_image(bad, **self.context())
        self.assertIn("400", str(error.exception))

    def test_environment_selects_remote_when_the_url_is_set(self):
        with mock.patch.dict(os.environ, {"FORENSIC_IMAGE_SEARCH_URL": self.url, "FORENSIC_IMAGE_SEARCH_SERVICE_KEY": "secret-key"}):
            self.assertIsInstance(search_processor_from_environment(), RemoteImageSearchProcessor)
        with mock.patch.dict(os.environ, {"FORENSIC_IMAGE_SEARCH_URL": self.url, "FORENSIC_IMAGE_SEARCH_SERVICE_KEY": ""}):
            with self.assertRaises(ModelUnavailableError):
                search_processor_from_environment()


if __name__ == "__main__":
    unittest.main()
