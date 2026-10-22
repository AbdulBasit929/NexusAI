"""Behaviour test of the patched speech client (patch_image_media_pipeline.py), run against the media_pipeline.py that the overlay would install, inside the worker image, offline:

    python -X utf8 tools/media_data/patch_image_media_pipeline.py <image media_pipeline.py> <dir>/media_pipeline.py
    docker run --rm --network none -v <dir>:/patched:ro -v <repo>/tools/media_data:/t:ro nexusai-forensic-records-worker:before-media-ocr-20261021 python /t/test_patched_asr.py

Nothing is sent over a network: the HTTP call is replaced by a fake that answers like the service did on the Urdu clips (Hindi detected when no language is given, Urdu when it is told).
"""
import importlib.util
import json
import os
import sys
import tempfile
import urllib.request
from pathlib import Path

sys.path.insert(0, "/app")
spec = importlib.util.spec_from_file_location("media_pipeline_patched", "/patched/media_pipeline.py")
module = importlib.util.module_from_spec(spec)
sys.modules["media_pipeline_patched"] = module
spec.loader.exec_module(module)

calls = []


class FakeResponse:
    def __init__(self, body):
        self._body = body

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False

    def read(self):
        return self._body


def fake_urlopen(request, timeout=None):
    data = request.data.decode("utf-8", "ignore")
    told = 'name="language"' in data
    model = data.split('name="model"')[1].split("\r\n")[2]
    calls.append({"told": told, "model": model, "timeout": timeout})
    if told:
        return FakeResponse(json.dumps({"language": "ur", "segments": [{"start": 0, "end": 2.5, "text": "یہ ایک تجربہ ہے"}]}).encode())
    return FakeResponse(json.dumps({"language": "hi", "segments": [{"start": 0, "end": 2.5, "text": "यह एक प्रयोग है"}]}).encode())


urllib.request.urlopen = fake_urlopen
module._ffprobe = lambda path: {"format": {"duration": "30.0"}}
audio = Path(tempfile.mkdtemp()) / "clip.wav"
audio.write_bytes(b"RIFF....WAVEfmt ")


def read(processor, **kwargs):
    calls.clear()
    return processor.transcribe(audio, evidence_id="e1", version_id="v1", source_file="clip.wav", **kwargs)


def segments(observations):
    return [o for o in observations if o.observation_type == "audio_transcript_segment"]


os.environ["FORENSIC_ASR_HINDI_AS_URDU"] = "false"
processor = module.LocalAIASRProcessor("http://api:8088/v1/audio/transcriptions", "faster-whisper-turbo", 60, long_model="faster-whisper-small-ur", long_seconds=600)

# 1. flag off: what the service says is kept, one request, no language sent
out = read(processor)
assert len(calls) == 1 and not calls[0]["told"] and segments(out)[0].payload["language"] == "hi", (calls, out)

# 2. flag on, no language requested, Hindi heard: a second request that names Urdu; the kept reading is the Urdu one and says why
os.environ["FORENSIC_ASR_HINDI_AS_URDU"] = "true"
out = read(processor)
assert len(calls) == 2 and not calls[0]["told"] and calls[1]["told"], calls
seg = segments(out)[0]
assert seg.payload["language"] == "ur" and seg.payload["requested_language"] == "ur", seg.payload
assert seg.payload["first_detected_language"] == "hi" and "read again as ur" in seg.payload["language_resolution"], seg.payload
assert seg.payload["model"] == "faster-whisper-turbo" and seg.payload["manual_review_required"] is True
assert any(o.observation_type == "audio_roman_urdu_segment" for o in out), "the Urdu reading must also get its Roman Urdu derivative"

# 3. flag on but the caller named a language: no second request, no rewriting
out = read(processor, language="ur")
assert len(calls) == 1 and calls[0]["told"] and "first_detected_language" not in segments(out)[0].payload

# 4. flag on, English audio (service says en): untouched
module.urllib.request.urlopen = lambda request, timeout=None: FakeResponse(json.dumps({"language": "en", "segments": [{"start": 0, "end": 1, "text": "hello there"}]}).encode())
out = read(processor)
assert segments(out)[0].payload["language"] == "en" and "language_resolution" not in segments(out)[0].payload

# 5. long recordings use the fast model (duration above long_seconds), short ones the accurate one
module.urllib.request.urlopen = fake_urlopen
module._ffprobe = lambda path: {"format": {"duration": "1200.0"}}
read(processor, language="ur")
assert calls[0]["model"] == "faster-whisper-small-ur", calls
module._ffprobe = lambda path: {"format": {"duration": "30.0"}}
read(processor, language="ur")
assert calls[0]["model"] == "faster-whisper-turbo", calls
segs = segments(read(processor, language="ur"))
assert segs[0].payload["model"] == "faster-whisper-turbo"
# 6. Punjabi / Pashto readings are marked LOW_RESOURCE_LANGUAGE with the measured figure and a warning; Urdu and English are not; the list can be changed
module.urllib.request.urlopen = lambda request, timeout=None: FakeResponse(json.dumps({"language": "pa", "segments": [{"start": 0, "end": 3, "text": "sat sri"}]}).encode())
seg = segments(read(processor, language="pa"))[0]
assert seg.payload["reliability_state"] == "LOW_RESOURCE_LANGUAGE" and "90%" in seg.payload["reliability_note"] and any("Low-resource" in w for w in seg.warnings), (seg.payload, seg.warnings)
module.urllib.request.urlopen = fake_urlopen
seg = segments(read(processor, language="ur"))[0]
assert "reliability_state" not in seg.payload and not any("Low-resource" in w for w in seg.warnings)
os.environ["FORENSIC_ASR_LOW_RESOURCE_LANGUAGES"] = "ur"
seg = segments(read(processor, language="ur"))[0]
assert seg.payload["reliability_state"] == "LOW_RESOURCE_LANGUAGE"
print("patched speech client: 6 behaviours OK")
