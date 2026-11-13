"""Language routes for speech reading: which languages the worker reads itself, with which model, and how much to trust the result.

The live ASR step posts a recording to the local Whisper service (English and Urdu are read well enough there). Pashto, Pakistani Punjabi in the Shahmukhi script and Saraiki are not: Whisper writes the wrong
script or nothing. The speech lab (docs/data/speech-lab-preregistration-20261112.md) measured a CPU route for them. This module is that route:

* a ROUTE TABLE kept as data (speech_routes.json, versioned, `FORENSIC_ASR_ROUTES_PATH` can point to another file): per language the backend, the model folder, the measured figures and the STATUS the
  interface shows: MACHINE_TRANSCRIPT (a transcript to be reviewed), SEARCH_LEAD (text good enough to find the recording by a spoken word, not to read as a transcript) or LOW_RESOURCE_LANGUAGE (a pointer to listen);
* a CUTTER that splits a recording into pieces of at most `limit` seconds at the quietest points, so that a model with a short input limit (40 s) or one trained on short clips reads it in pieces and every
  piece has a start and an end in the recording (the Omnilingual CTC models take at most 40 s; the Whisper Pashto fine-tune loses most words on long input);
* an in-process READER for the Omnilingual CTC model converted by tools/media_data/omni_ctc.py (torch and transformers are in the worker image; nothing is installed and nothing leaves the machine);
* a TRANSCRIBER wrapper with the interface of the worker's ASR client (`transcribe(path, *, evidence_id, version_id, source_file, language=None)`): a language with a route is read here, any other language
  goes to the wrapped client unchanged; with no language requested, a recording the wrapped client detects as a routed language is read again by the route (the same rule as the Hindi-to-Urdu re-read).

Every reading is a model observation: `manual_review_required` is true, the route's status and measured figures travel with each segment, and a reading that stopped at the time budget says so.
"""
from __future__ import annotations

import json
import math
import os
import subprocess
import time
import uuid
from array import array
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable

try:
    from ingestion.forensic_records import media_pipeline as _mp
except ModuleNotFoundError:  # pragma: no cover - the worker image keeps the modules side by side in /app
    import media_pipeline as _mp  # type: ignore

ROUTES_ENV = "FORENSIC_ASR_ROUTES_PATH"
DEFAULT_ROUTES_PATH = Path(__file__).with_name("speech_routes.json")
MACHINE_TRANSCRIPT, SEARCH_LEAD, LOW_RESOURCE_LANGUAGE = "MACHINE_TRANSCRIPT", "SEARCH_LEAD", "LOW_RESOURCE_LANGUAGE"
STATUSES = (MACHINE_TRANSCRIPT, SEARCH_LEAD, LOW_RESOURCE_LANGUAGE)
BACKENDS = ("omni_ctc",)
SAMPLE_RATE = 16000
STATUS_NOTE = {
    MACHINE_TRANSCRIPT: "Machine transcript: a model observation that requires analyst review",
    SEARCH_LEAD: "Search lead: machine reading good enough to find this recording by a spoken word, not a transcript; listen before relying on any word",
    LOW_RESOURCE_LANGUAGE: "Low-resource language: a pointer to listen, not a transcript",
}


class RouteError(RuntimeError):
    pass


# ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- the table


def load_routes(path: str | os.PathLike | None = None) -> dict[str, dict[str, Any]]:
    """The routes by language code. Refuses a table with an unknown status or backend or without a model folder: a route that cannot be read must not look like one that works."""
    source = Path(path or os.getenv(ROUTES_ENV, "").strip() or DEFAULT_ROUTES_PATH)
    try:
        data = json.loads(source.read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        raise RouteError(f"speech route table {source} cannot be read: {exc}") from exc
    routes = data.get("routes")
    if not isinstance(routes, dict):
        raise RouteError("speech route table has no routes object")
    for language, route in routes.items():
        if not (isinstance(language, str) and language.isalpha() and language == language.lower() and 2 <= len(language) <= 3):
            raise RouteError(f"route language {language!r} is not a two- or three-letter lowercase code")
        if route.get("status") not in STATUSES:
            raise RouteError(f"route {language}: status {route.get('status')!r} is not one of {STATUSES}")
        if route.get("backend") not in BACKENDS:
            raise RouteError(f"route {language}: backend {route.get('backend')!r} is not one of {BACKENDS}")
        if not str(route.get("model_dir") or "").strip():
            raise RouteError(f"route {language}: model_dir is required")
    return routes


def route_for(language: str | None, routes: dict[str, dict[str, Any]]) -> dict[str, Any] | None:
    return routes.get(str(language).strip().lower()) if language else None


# ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- the cutter


def frame_energies(samples, rate: int = SAMPLE_RATE, frame: float = 0.02) -> list[float]:
    """Mean squared amplitude of every 20 ms frame (a partial last frame is dropped)."""
    size = max(1, int(frame * rate))
    count = len(samples) // size
    try:
        import numpy as np
        arr = np.asarray(samples, dtype="float32")[: count * size].reshape(count, size)
        return (arr.astype("float64") ** 2).mean(axis=1).tolist()
    except ImportError:
        return [sum(float(x) * float(x) for x in samples[i * size:(i + 1) * size]) / size for i in range(count)]


def cut_points(samples, rate: int = SAMPLE_RATE, limit: float = 38.0, low: float = 20.0, high: float = 36.0, smooth: float = 0.2, energies: list[float] | None = None) -> list[int]:
    """Sample indices where pieces end, the last being len(samples). Each piece is at most `limit` seconds; while more than `limit` seconds remain, the next cut is the quietest 20 ms frame
    (energy smoothed over `smooth` seconds) between `low` and `high` seconds into the remaining audio. Cutting is by sound only: no model, no word is split on purpose."""
    size = max(1, int(0.02 * rate))
    energy = energies if energies is not None else frame_energies(samples, rate)
    half = max(0, int(smooth / 0.02) // 2)
    points, start = [], 0
    while (len(samples) - start) / rate > limit:
        first, last = (start + int(low * rate)) // size, min(len(energy), (start + int(high * rate)) // size)
        best, best_value = first, None
        for i in range(first, max(first + 1, last)):
            window = energy[max(0, i - half):i + half + 1]
            value = sum(window) / len(window)
            if best_value is None or value < best_value:
                best, best_value = i, value
        cut = best * size + size // 2
        points.append(cut)
        start = cut
    points.append(len(samples))
    return points


def pieces(samples, rate: int = SAMPLE_RATE, **kwargs) -> list[tuple[int, int]]:
    """[(start index, end index)] of the pieces."""
    out, start = [], 0
    for end in cut_points(samples, rate, **kwargs):
        out.append((start, end))
        start = end
    return out


# ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ audio and model


ARABIC_RANGES = ((0x0600, 0x06FF), (0x0750, 0x077F), (0xFB50, 0xFDFF), (0xFE70, 0xFEFF))


def allowed_pieces(pieces, ranges=ARABIC_RANGES) -> list[bool]:
    """Per vocabulary piece: may it be emitted when the output must be in the script of `ranges`? Special tokens, spaces, digits and punctuation pass; a piece with a letter or combining mark of another script does not (a Devanagari vowel sign is a mark, not a letter)."""
    import unicodedata
    out = []
    for piece in pieces:
        if piece.startswith("<") and piece.endswith(">"):
            out.append(True)
            continue
        out.append(all(unicodedata.category(ch)[0] not in ("L", "M") or any(lo <= ord(ch) <= hi for lo, hi in ranges) for ch in piece))
    return out


def decode_audio(path: Path, max_seconds: int = 4 * 3600, timeout: int = 900) -> array:
    """16 kHz mono float samples of any recording the worker admits, through ffmpeg (already in the worker image)."""
    command = ["ffmpeg", "-v", "error", "-nostdin", "-i", str(path), "-t", str(max_seconds), "-ac", "1", "-ar", str(SAMPLE_RATE), "-f", "f32le", "-"]
    try:
        done = subprocess.run(command, capture_output=True, timeout=timeout, check=True)
    except (OSError, subprocess.SubprocessError) as exc:
        raise _mp.MediaProcessingError(f"audio could not be decoded for the speech route: {exc}") from exc
    samples = array("f")
    samples.frombytes(done.stdout[: len(done.stdout) // 4 * 4])
    return samples


@dataclass(frozen=True)
class PieceReading:
    start_seconds: float
    end_seconds: float
    text: str
    confidence: float | None


class OmniCTCReader:
    """The converted Omnilingual CTC model (tools/media_data/omni_ctc.py `convert`), run in this process on the CPU. Loaded on first use."""

    def __init__(self, folder: str | os.PathLike, threads: int | None = None, script: str | None = None) -> None:
        self.script = script
        self.folder = Path(folder)
        self.threads = threads or max(1, int(os.getenv("FORENSIC_ASR_ROUTE_THREADS", "4")))
        self._model = None
        self._pieces: list[str] = []

    def _load(self) -> None:
        if self._model is not None:
            return
        import torch
        from transformers import Wav2Vec2ForCTC
        if not (self.folder / "model.safetensors").exists():
            raise _mp.ModelUnavailableError(f"speech route model folder {self.folder} has no model.safetensors")
        torch.set_num_threads(self.threads)
        self._model = Wav2Vec2ForCTC.from_pretrained(str(self.folder), local_files_only=True).eval()
        self._pieces = json.loads((self.folder / "pieces.json").read_text(encoding="utf-8"))
        self._penalty = None
        if self.script == "arabic":      # a CTC model takes no language: the same spoken Urdu comes out in Devanagari unless the output is held to the Arabic script
            self._penalty = torch.tensor([0.0 if ok else -1e4 for ok in allowed_pieces(self._pieces)])

    def read(self, samples) -> tuple[str, float | None]:
        """Text and mean confidence (the mean of the top probability over the frames that emit a piece) of one piece of at most 40 seconds."""
        self._load()
        import torch
        x = torch.as_tensor(samples, dtype=torch.float32)
        x = (x - x.mean()) / (x.std() + 1e-7)       # the instance normalisation the model was trained with
        with torch.inference_mode():
            logits = self._model(x.unsqueeze(0)).logits[0]
            if self._penalty is not None:
                logits = logits + self._penalty
            probabilities = logits.softmax(-1)
        top, ids = probabilities.max(-1)
        out, confidences, previous = [], [], -1
        for p, i in zip(top.tolist(), ids.tolist()):
            if i != 0 and i != previous:
                out.append(self._pieces[i])
                confidences.append(p)
            previous = i
        text = "".join(piece for piece in out if not (piece.startswith("<") and piece.endswith(">"))).replace("▁", " ")
        return " ".join(text.split()), (sum(confidences) / len(confidences) if confidences else None)


# ---------------------------------------------------------------------------------------------------------------------------------------------------------------- the transcriber wrapper


class RoutedSpeechTranscriber:
    """Wraps the worker's ASR client: a routed language is read in this process, every other language by the wrapped client."""

    processor_id = "nexusai-speech-route"
    processor_revision = "speech-route-v1-20261113"

    def __init__(
        self,
        fallback,
        routes: dict[str, dict[str, Any]] | None = None,
        reader_factory: Callable[[dict[str, Any]], Any] | None = None,
        decoder: Callable[[Path], Any] | None = None,
        budget_seconds: float | None = None,
        clock: Callable[[], float] = time.monotonic,
    ) -> None:
        self.fallback = fallback
        self.routes = load_routes() if routes is None else routes
        self._reader_factory = reader_factory or (lambda route: OmniCTCReader(route["model_dir"], script=route.get("script")))
        self._readers: dict[str, Any] = {}
        self._decoder = decoder or decode_audio
        self.budget_seconds = budget_seconds if budget_seconds is not None else float(os.getenv("FORENSIC_ASR_ROUTE_BUDGET_SECONDS", "1500"))
        self._clock = clock

    def __getattr__(self, name: str):      # the wrapped client's other attributes (endpoint, model, ...) stay reachable
        return getattr(self.fallback, name)

    def _reader(self, route: dict[str, Any]):
        key = route["model_dir"]
        if key not in self._readers:
            self._readers[key] = self._reader_factory(route)
        return self._readers[key]

    def transcribe(self, path: Path, *, evidence_id: str, version_id: str, source_file: str, language: str | None = None):
        route = route_for(language, self.routes)
        if route is not None:
            return self._read(path, route, str(language).lower(), evidence_id=evidence_id, version_id=version_id, source_file=source_file, requested=language, detected=None)
        result = self.fallback.transcribe(path, evidence_id=evidence_id, version_id=version_id, source_file=source_file, language=language)
        if language is None:
            detected = next((str(o.payload.get("language") or "").lower() for o in result if o.payload.get("language")), "")
            again = route_for(detected, self.routes)
            if again is not None:
                return self._read(path, again, detected, evidence_id=evidence_id, version_id=version_id, source_file=source_file, requested=None, detected=detected)
        return result

    def _read(self, path: Path, route: dict[str, Any], language: str, *, evidence_id: str, version_id: str, source_file: str, requested: str | None, detected: str | None):
        started = self._clock()
        samples = self._decoder(path)
        seconds_total = len(samples) / SAMPLE_RATE
        reader = self._reader(route)
        status = route["status"]
        limit = float(route.get("piece_limit_seconds", 38.0))
        observations, complete = [], True
        for index, (a, b) in enumerate(pieces(samples, SAMPLE_RATE, limit=limit, low=min(20.0, limit * 0.55), high=min(36.0, limit * 0.95))):
            if index and self._clock() - started > self.budget_seconds:
                complete = False
                break
            text, confidence = reader.read(samples[a:b])
            if not text.strip():
                continue
            start, end = round(a / SAMPLE_RATE, 3), round(b / SAMPLE_RATE, 3)
            observations.append((index, start, end, text, confidence))
        out = []
        for index, start, end, text, confidence in observations:
            payload = {
                "transcript_contract": _mp.AUDIO_TRANSCRIPT_CONTRACT, "text": text, "language": language, "requested_language": requested, "detected_language": detected,
                "start_seconds": start, "end_seconds": end, "timing_status": "reported", "raw_start": start, "raw_end": end,
                "processor": self.processor_id, "processor_revision": self.processor_revision, "model": route.get("model_id") or route["model_dir"],
                "parent_evidence_id": evidence_id, "parent_version_id": version_id, "manual_review_required": True,
                "reliability_state": status, "route": {"backend": route["backend"], "table_version": route.get("table_version"), "measured": route.get("measured"), "evidence": route.get("evidence")},
                "reading_complete": complete, "audio_seconds_total": round(seconds_total, 3),
            }
            warnings = ("Transcript is a model observation and requires analyst review", STATUS_NOTE[status])
            if not complete:
                warnings += (f"Reading stopped at the time budget; the recording continues after {observations[-1][2]} seconds and is not read",)
            out.append(_mp.MediaObservation(
                observation_id=str(uuid.uuid5(uuid.NAMESPACE_URL, f"{evidence_id}:{version_id}:asr-route:{language}:{index}:{start}:{end}")),
                contract_version=_mp.AUDIO_TIMESTAMP_SEGMENT_CONTRACT, observation_type="audio_transcript_segment",
                confidence=None if confidence is None else round(float(confidence), 4), citation_locator={"source_file": source_file, "start_seconds": start, "end_seconds": end},
                payload=payload, warnings=warnings))
        return tuple(out)
