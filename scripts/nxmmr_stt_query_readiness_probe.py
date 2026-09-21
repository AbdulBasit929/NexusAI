# SPDX-License-Identifier: MIT
"""No-inference readiness probe for the activated STT/image/face worker roles."""

import json
import os
import socket
import sys


def denied(*_args, **_kwargs):
    raise RuntimeError("network access is forbidden in the worker readiness probe")


socket.create_connection = denied
socket.socket.connect = denied
socket.socket.connect_ex = denied
sys.path.insert(0, "/app")

from media_pipeline import (  # noqa: E402
    LocalAIASRProcessor,
    LocalAIFaceProcessor,
    SigLIPImageEmbeddingProcessor,
)

expected = {
    "FORENSIC_ASR_ENABLED": "true",
    "FORENSIC_ASR_MODEL": "faster-whisper-small-ur",
    "FORENSIC_IMAGE_EMBEDDING_ENABLED": "true",
    "FORENSIC_FACE_ENABLED": "true",
    "FORENSIC_ANPR_ENABLED": "true",
    "FORENSIC_OCR_ENABLED": "true",
    "FORENSIC_VIDEO_ANPR_V2_ENABLED": "false",
    "FORENSIC_VIDEO_ANPR_V3_ENABLED": "true",
    "FORENSIC_TTS_ENABLED": "false",
    "HF_HUB_OFFLINE": "1",
    "TRANSFORMERS_OFFLINE": "1",
}
for name, value in expected.items():
    assert os.environ.get(name) == value, name

asr = LocalAIASRProcessor.from_environment()
face = LocalAIFaceProcessor.from_environment()
siglip = SigLIPImageEmbeddingProcessor.from_environment()
assert asr.model == "faster-whisper-small-ur"
assert face.model == "face-detect-yunet-sface"
assert siglip.model_sha256 == "2c63cb7d1f2e95ba501893cbb8faeb4ea9a3af295498d35097126228659c2af8"

print("NXMMR_STT_QUERY_READINESS=" + json.dumps({
    "state": "PASS",
    "asr": "CONFIG_READY",
    "siglip": "ASSET_READY",
    "face": "CONFIG_READY",
    "network_used": False,
    "inference_performed": False,
    "model_downloads": False,
    "product_certified": False,
}))
