#!/usr/bin/env python3
"""Focused language-forwarding contract for the faster-whisper backend."""

from __future__ import annotations

import importlib.util
import sys
import types
import unittest
from unittest.mock import patch
from pathlib import Path
from types import SimpleNamespace


class Message:
    def __init__(self, **values):
        self.__dict__.update(values)


class AbortError(RuntimeError):
    pass


class Context:
    def abort(self, code, detail):
        raise AbortError(f"{code}: {detail}")


class Model:
    def __init__(self):
        self.calls = []

    def transcribe(self, path, **options):
        self.calls.append((path, options))
        segment = SimpleNamespace(start=1.25, end=2.5, text=" bounded transcript", words=[])
        return iter((segment,)), SimpleNamespace(language=options["language"] or "en", duration=2.5)


def load_backend_module():
    grpc = types.ModuleType("grpc")
    grpc.StatusCode = SimpleNamespace(INVALID_ARGUMENT="INVALID_ARGUMENT")
    grpc.server = lambda *args, **kwargs: None
    sys.modules["grpc"] = grpc

    torch = types.ModuleType("torch")
    torch.backends = SimpleNamespace()
    sys.modules["torch"] = torch

    faster_whisper = types.ModuleType("faster_whisper")
    faster_whisper.WhisperModel = ModelFactory
    sys.modules["faster_whisper"] = faster_whisper
    tokenizer = types.ModuleType("faster_whisper.tokenizer")
    tokenizer._LANGUAGE_CODES = ("en", "ur")
    sys.modules["faster_whisper.tokenizer"] = tokenizer

    backend_pb2 = types.ModuleType("backend_pb2")
    for name in ("Reply", "Result", "TranscriptWord", "TranscriptSegment", "TranscriptResult"):
        setattr(backend_pb2, name, Message)
    sys.modules["backend_pb2"] = backend_pb2

    backend_pb2_grpc = types.ModuleType("backend_pb2_grpc")
    backend_pb2_grpc.BackendServicer = object
    backend_pb2_grpc.add_BackendServicer_to_server = lambda *args: None
    sys.modules["backend_pb2_grpc"] = backend_pb2_grpc

    grpc_auth = types.ModuleType("grpc_auth")
    grpc_auth.get_auth_interceptors = lambda: ()
    sys.modules["grpc_auth"] = grpc_auth

    path = Path(__file__).with_name("backend.py")
    spec = importlib.util.spec_from_file_location("nexusai_faster_whisper_backend", path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


class ModelFactory:
    calls = []

    def __init__(self, model, **options):
        self.calls.append((model, options))


class FasterWhisperLanguageContract(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.backend = load_backend_module()

    def transcribe(self, language):
        servicer = self.backend.BackendServicer()
        servicer.model = Model()
        request = SimpleNamespace(
            dst="bounded.wav",
            language=language,
            timestamp_granularities=[],
        )
        result = servicer.AudioTranscription(request, Context())
        return servicer.model.calls[0][1], result

    def test_forwards_urdu(self):
        options, result = self.transcribe("ur")
        self.assertEqual(options["language"], "ur")
        self.assertEqual(result.language, "ur")
        self.assertEqual((result.segments[0].start, result.segments[0].end), (1250000000, 2500000000))

    def test_forwards_english(self):
        options, result = self.transcribe("EN")
        self.assertEqual(options["language"], "en")
        self.assertEqual(result.language, "en")

    def test_preserves_automatic_detection(self):
        options, result = self.transcribe("")
        self.assertIsNone(options["language"])
        self.assertEqual(result.language, "en")

    def test_rejects_unrecognized_language(self):
        with self.assertRaisesRegex(AbortError, "unsupported Whisper language code"):
            self.transcribe("not-a-language")

    def test_cpu_model_load_uses_accepted_int8_compute(self):
        ModelFactory.calls.clear()
        servicer = self.backend.BackendServicer()
        result = servicer.LoadModel(SimpleNamespace(Model="bounded-model", CUDA=False), Context())
        self.assertTrue(result.success)
        self.assertEqual(ModelFactory.calls, [("bounded-model", {"device": "cpu", "compute_type": "int8"})])

    def test_model_load_prefers_existing_local_directory(self):
        ModelFactory.calls.clear()
        servicer = self.backend.BackendServicer()
        with patch.object(self.backend.os.path, "isdir", return_value=True):
            result = servicer.LoadModel(SimpleNamespace(Model="faster-whisper-small", CUDA=False), Context())
        self.assertTrue(result.success)
        self.assertEqual(
            ModelFactory.calls[0][0],
            self.backend.os.path.join("/models", "faster-whisper-small"),
        )


if __name__ == "__main__":
    unittest.main()
