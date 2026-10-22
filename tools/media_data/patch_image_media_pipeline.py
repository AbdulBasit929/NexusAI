#!/usr/bin/env python3
"""Patch the media_pipeline.py that is INSIDE the running worker image (not the repository's newer file) so it can start the new OCR backends and choose a speech model by duration, and write the
result for the overlay build.

    python -X utf8 tools/media_data/patch_image_media_pipeline.py <extracted image media_pipeline.py> <output path>

Why: the worker image in production is the older build plus the Ingest v2 overlay; its media_pipeline.py is 500 lines older than the repository's and a straight copy of the repository file would
bring in unrelated, unreviewed changes (video plate readers, contextual routing) next to an older worker.py. This script makes a few edits to the image's own file, each of which must match once:
(1) the Tesseract processor can take its languages and page mode as arguments (the OCR router builds an English and an Urdu reader from the same pinned data); (2) the OCR branch of the factory
selects the `auto` router or the `onnx` backend from FORENSIC_OCR_BACKEND and otherwise keeps the existing Tesseract behaviour byte for byte; (3) the speech client can name a fast model for long
recordings (FORENSIC_ASR_LONG_AUDIO_MODEL above FORENSIC_ASR_LONG_AUDIO_SECONDS) and records which model read each recording; (4) when no language was requested and the service hears Hindi, the recording is read again as Urdu (FORENSIC_ASR_HINDI_AS_URDU, off unless set).
Line endings of the input are preserved. Behaviour test: tools/media_data/test_patched_asr.py (runs offline inside the worker image).
"""
import sys

HINDI_WRAPPER = '''    def transcribe(
        self,
        path: Path,
        *,
        evidence_id: str,
        version_id: str,
        source_file: str,
        language: str | None = None,
    ) -> tuple[MediaObservation, ...]:
        """One reading; when no language was requested and the service hears Hindi, the recording is read again as Urdu (FORENSIC_ASR_HINDI_AS_URDU).

        Spoken Hindi and Urdu are one language and Whisper picks between them by script habit: on 24 Urdu test clips with no language given, 7 were detected as Hindi and written in Devanagari
        (script correct 70.8%); told the language, all 24 came out in Urdu script. The first reading is discarded; the second one records what happened.
        """
        kwargs = {"evidence_id": evidence_id, "version_id": version_id, "source_file": source_file}
        observations = self._transcribe_once(path, language=language, **kwargs)
        enabled = os.getenv("FORENSIC_ASR_HINDI_AS_URDU", "false").strip().lower() in {"1", "true", "yes", "on"}
        heard = {str(o.payload.get("language") or "").strip().lower() for o in observations if o.observation_type == "audio_transcript_segment"}
        if language is None and enabled and "hi" in heard:
            observations = self._transcribe_once(path, language="ur", **kwargs)
            for observation in observations:
                if observation.observation_type == "audio_transcript_segment":
                    observation.payload["first_detected_language"] = "hi"
                    observation.payload["language_resolution"] = "detected hi with no language requested; read again as ur (FORENSIC_ASR_HINDI_AS_URDU)"
        return self._mark_low_resource(observations)

    @staticmethod
    def _mark_low_resource(observations):
        """Punjabi and Pashto were measured at word error 91% and 96% on read speech (FLEURS dev, 16 clips each, 2026-10-22): such a reading is a pointer to listen, never a transcript.

        Segments in a language of FORENSIC_ASR_LOW_RESOURCE_LANGUAGES (default pa,ps) get reliability_state LOW_RESOURCE_LANGUAGE, the measured figure and a warning that the interface shows.
        """
        import dataclasses
        low = {item.strip().lower() for item in os.getenv("FORENSIC_ASR_LOW_RESOURCE_LANGUAGES", "pa,ps").split(",") if item.strip()}
        marked = []
        for observation in observations:
            language = str(observation.payload.get("language") or "").strip().lower()
            if observation.observation_type == "audio_transcript_segment" and language in low:
                observation.payload["reliability_state"] = "LOW_RESOURCE_LANGUAGE"
                observation.payload["reliability_note"] = "measured word error about 90% on read speech; listen to the recording, do not rely on this text"
                observation = dataclasses.replace(observation, warnings=observation.warnings + ("Low-resource language: measured word error about 90% on read speech; the text is a pointer to listen, not a transcript",))
            marked.append(observation)
        return tuple(marked)

'''

EDITS = [
    (
        '    @classmethod\n    def from_environment(cls) -> "TesseractImageOCRProcessor":\n        executable_value',
        '    @classmethod\n    def from_environment(cls, *, languages: str | None = None, page_segmentation_mode: int | None = None) -> "TesseractImageOCRProcessor":\n'
        '        """`languages` and `page_segmentation_mode` override the environment for one instance (the OCR router builds an English and an Urdu reader from the same pinned data)."""\n'
        '        executable_value',
    ),
    (
        '            languages=os.getenv("FORENSIC_OCR_LANGUAGES", "eng+urd"),\n',
        '            languages=languages or os.getenv("FORENSIC_OCR_LANGUAGES", "eng+urd"),\n',
    ),
    (
        '            page_segmentation_mode=int(os.getenv("FORENSIC_OCR_PSM", "11")),\n        )\n',
        '            page_segmentation_mode=int(os.getenv("FORENSIC_OCR_PSM", "11")) if page_segmentation_mode is None else page_segmentation_mode,\n        )\n',
    ),
    (
        '    if os.getenv("FORENSIC_OCR_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}:\n        image_processors.append(TesseractImageOCRProcessor.from_environment())\n',
        '    if os.getenv("FORENSIC_OCR_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}:\n'
        '        ocr_backend = os.getenv("FORENSIC_OCR_BACKEND", "tesseract").strip().lower()\n'
        '        if ocr_backend == "auto":\n'
        '            from ocr_router import OcrRouter\n'
        '            image_processors.append(OcrRouter.from_environment())\n'
        '        elif ocr_backend == "onnx":\n'
        '            from onnx_ocr_processor import OnnxPaddleOCRProcessor\n'
        '            image_processors.append(OnnxPaddleOCRProcessor.from_environment())\n'
        '        else:\n'
        '            image_processors.append(TesseractImageOCRProcessor.from_environment())\n',
    ),
    (
        '    def __init__(self, endpoint: str, model: str, timeout_seconds: int = 300) -> None:\n        self.endpoint = endpoint\n        self.model = model\n        self.timeout_seconds = timeout_seconds\n',
        '    def __init__(self, endpoint: str, model: str, timeout_seconds: int = 300, long_model: str = "", long_seconds: int = 0) -> None:\n'
        '        self.endpoint = endpoint\n        self.model = model\n        self.timeout_seconds = timeout_seconds\n'
        '        self.long_model = long_model.strip()\n        self.long_seconds = max(0, int(long_seconds))\n',
    ),
    (
        '        return cls(endpoint, model, max(15, int(os.getenv("FORENSIC_ASR_TIMEOUT_SECONDS", "300"))))\n',
        '        return cls(endpoint, model, max(15, int(os.getenv("FORENSIC_ASR_TIMEOUT_SECONDS", "300"))),\n'
        '                   long_model=os.getenv("FORENSIC_ASR_LONG_AUDIO_MODEL", ""), long_seconds=int(os.getenv("FORENSIC_ASR_LONG_AUDIO_SECONDS", "0") or 0))\n'
        '\n'
        '    def _model_for(self, path: Path) -> str:\n'
        '        """The accurate model for a recording up to long_seconds, the fast one above it (real-time factor 1.7 against 1.0 on the laptop CPU); the duration is read with ffprobe."""\n'
        '        if not self.long_model or self.long_seconds <= 0:\n'
        '            return self.model\n'
        '        try:\n'
        '            duration = float((_ffprobe(path).get("format") or {}).get("duration") or 0)\n'
        '        except (MediaProcessingError, ValueError):\n'
        '            return self.model\n'
        '        return self.long_model if duration > self.long_seconds else self.model\n',
    ),
    (
        '        boundary = f"nexusai-{uuid.uuid4().hex}"\n        audio = path.read_bytes()\n        fields = {"model": self.model, "response_format": "verbose_json"}\n',
        '        boundary = f"nexusai-{uuid.uuid4().hex}"\n        audio = path.read_bytes()\n        model_used = self._model_for(path)\n        fields = {"model": model_used, "response_format": "verbose_json"}\n',
    ),
    (
        '                        "processor_revision": self.processor_revision,\n                        "model": self.model,\n                        "parent_evidence_id": evidence_id,\n'
        '                        "parent_version_id": version_id,\n                        "manual_review_required": True,\n',
        '                        "processor_revision": self.processor_revision,\n                        "model": model_used,\n                        "parent_evidence_id": evidence_id,\n'
        '                        "parent_version_id": version_id,\n                        "manual_review_required": True,\n',
    ),
    (
        # (4) the wrapper: the image's transcribe() becomes _transcribe_once(), and a new transcribe() reads a recording again as Urdu when no language was requested and the service heard Hindi
        '        return self.long_model if duration > self.long_seconds else self.model\n\n    def transcribe(\n        self,\n        path: Path,\n        *,\n        evidence_id: str,\n        version_id: str,\n        source_file: str,\n        language: str | None = None,\n    ) -> tuple[MediaObservation, ...]:\n        maximum =',
        '        return self.long_model if duration > self.long_seconds else self.model\n\n' + HINDI_WRAPPER
        + '    def _transcribe_once(\n        self,\n        path: Path,\n        *,\n        evidence_id: str,\n        version_id: str,\n        source_file: str,\n        language: str | None = None,\n    ) -> tuple[MediaObservation, ...]:\n        maximum =',
    ),
]


def main(source, target):
    raw = open(source, "rb").read().decode("utf-8")
    crlf = "\r\n" in raw
    text = raw.replace("\r\n", "\n")
    for old, new in EDITS:
        found = text.count(old)
        if found != 1:
            sys.exit(f"expected exactly one match, found {found}: {old[:80]!r}")
        text = text.replace(old, new)
    open(target, "wb").write((text.replace("\n", "\r\n") if crlf else text).encode("utf-8"))
    print(f"patched {source} -> {target} ({'CRLF' if crlf else 'LF'}, {len(EDITS)} edits)")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    main(sys.argv[1], sys.argv[2])
