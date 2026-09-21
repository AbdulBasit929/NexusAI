# SPDX-License-Identifier: MIT
"""Local model-load readiness, no evidence, inference, database or network access."""
import builtins
import importlib.util
import json
import os
import socket
import sys


def denied(*args, **kwargs):
    raise RuntimeError("Network access is forbidden in the activation readiness probe")


socket.create_connection = denied
socket.socket.connect = denied
socket.socket.connect_ex = denied
assert importlib.util.find_spec("ultralytics") is None, "Ultralytics must be unavailable"
original_import = builtins.__import__


def guarded_import(name, *args, **kwargs):
    if name.split(".")[0] == "ultralytics":
        raise RuntimeError("Ultralytics import forbidden")
    return original_import(name, *args, **kwargs)


builtins.__import__ = guarded_import
sys.path.insert(0, "/app")
from media_pipeline import FastALPRImageProcessor
from video_anpr_onnx_v3 import VideoANPROnnxV3Processor
from multilingual_ocr import PaddleMultilingualOCRProcessor

for key in ("FORENSIC_ASR_ENABLED", "FORENSIC_FACE_ENABLED", "FORENSIC_IMAGE_EMBEDDING_ENABLED", "FORENSIC_VIDEO_ANPR_V2_ENABLED", "FORENSIC_TTS_ENABLED"):
    assert os.environ.get(key) == "false", key
for key in ("FORENSIC_ANPR_ENABLED", "FORENSIC_OCR_ENABLED", "FORENSIC_VIDEO_ANPR_V3_ENABLED"):
    assert os.environ.get(key) == "true", key
assert os.environ.get("FORENSIC_OCR_BACKEND") == "paddle"
image = FastALPRImageProcessor.from_environment()
video = VideoANPROnnxV3Processor.from_environment()
ocr = PaddleMultilingualOCRProcessor.from_environment()
print("NXMMR_READINESS_JSON=" + json.dumps({
    "state": "PASS", "image_anpr": "MODEL_LOAD_READY", "video_v3": "MODEL_LOAD_READY",
    "paddle_ocr": "MODEL_LOAD_READY", "ultralytics_available": False,
    "model_downloads": False, "inference_performed": False,
    "product_certified": False, "live_product_acceptance": "PENDING",
}))
