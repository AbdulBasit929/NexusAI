# SPDX-License-Identifier: MIT
"""Bounded official FLEURS acquisition and private, non-retained ASR evaluation."""
from __future__ import annotations

import argparse
import hashlib
import json
import re
import struct
import subprocess
import sys
import time
import urllib.parse
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

REPO = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(REPO))
from scripts.evaluate_nexusai_asr_transcript import normalize, edit_distance

ROOT = REPO / "local-acceptance-models/nxmmr/private-benchmarks/speech-breadth-v1/fleurs"
REVISION = "70bb2e84b976b7e960aa89f1c648e09c59f894dd"
ROWS_URL = "https://datasets-server.huggingface.co/rows?dataset=google%2Ffleurs&config=en_us&split=validation&offset=0&length=10"
CARD_URL = f"https://huggingface.co/datasets/google/fleurs/raw/{REVISION}/README.md"
MAX_FILE = 4 * 1024 * 1024
MAX_AUDIO_TOTAL = 40 * 1024 * 1024


def sha(data):
    return hashlib.sha256(data).hexdigest()


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, indent=2).encode("utf-8") + b"\n"


def admitted_license(value):
    return value == "cc-by-4.0" or value == ["cc-by-4.0"]


def private_root():
    result = subprocess.run(["git", "check-ignore", str(ROOT / "manifest.json")], cwd=REPO, capture_output=True)
    if result.returncode != 0:
        raise ValueError("private benchmark destination must be Git ignored")
    ROOT.mkdir(parents=True, exist_ok=True)
    return ROOT


def save_new(path, value):
    data = canonical(value)
    if path.exists():
        if path.read_bytes() != data:
            raise ValueError(f"refusing to overwrite immutable receipt {path.name}")
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("xb") as output:
        output.write(data)


def fetch(url, maximum, audio=False):
    def allowed(target):
        parsed = urllib.parse.urlparse(target)
        if parsed.scheme != "https" or parsed.username or parsed.password:
            raise ValueError("HTTPS official-source URL required")
        hosts = {"datasets-server.huggingface.co"} if audio else {"huggingface.co", "datasets-server.huggingface.co"}
        if parsed.hostname not in hosts:
            raise ValueError("unapproved download host")
        if audio and not re.fullmatch(rf"/cached-assets/google/fleurs/--/{REVISION}/--/en_us/validation/[0-9]/audio/audio\.wav", parsed.path):
            raise ValueError("audio source revision/config/split mismatch")
    allowed(url)

    class Redirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            allowed(newurl)
            return super().redirect_request(req, fp, code, msg, headers, newurl)

    request = urllib.request.Request(url, headers={"User-Agent": "NexusAI-bounded-speech-validation/1"})
    with urllib.request.build_opener(Redirect()).open(request, timeout=45) as response:
        length = response.headers.get("Content-Length")
        if length and int(length) > maximum:
            raise ValueError("download exceeds bounded byte allowance")
        data = response.read(maximum + 1)
        if len(data) > maximum:
            raise ValueError("download exceeds bounded byte allowance")
        return data


def wav_info(data):
    if len(data) < 44 or data[:4] != b"RIFF" or data[8:12] != b"WAVE":
        raise ValueError("expected RIFF WAV source")
    offset, fmt, size = 12, None, None
    while offset + 8 <= len(data):
        tag, length = data[offset:offset + 4], struct.unpack_from("<I", data, offset + 4)[0]
        start = offset + 8
        if start + length > len(data):
            raise ValueError("truncated WAV chunk")
        if tag == b"fmt ":
            if length < 16:
                raise ValueError("invalid WAV format chunk")
            fmt = struct.unpack_from("<HHIIHH", data, start)
        if tag == b"data":
            size = length
        offset = start + length + length % 2
    if fmt is None or size is None:
        raise ValueError("missing WAV format/data")
    encoding, channels, rate, byte_rate, align, bits = fmt
    if encoding not in (1, 3) or channels != 1 or rate != 16000 or align != channels * bits // 8 or byte_rate != rate * align:
        raise ValueError("unexpected FLEURS WAV format")
    duration = size / byte_rate
    if not 0 < duration <= 30:
        raise ValueError("speech clip outside pre-registered duration bound")
    return {"duration_seconds": duration, "sample_rate": rate, "samples": size // align, "encoding": encoding, "bits": bits}


def preregister():
    root = private_root()
    if (root / "selection.json").exists():
        raise ValueError("sample is already frozen; validate/acquire it, do not select again")
    metadata = fetch("https://huggingface.co/api/datasets/google/fleurs", 2 * 1024 * 1024)
    meta = json.loads(metadata)
    if meta["sha"] != REVISION or not admitted_license(meta["cardData"]["license"]):
        raise ValueError("dataset revision/license changed; stop for review")
    card = fetch(CARD_URL, 1024 * 1024)
    if b"cc-by-4.0" not in card:
        raise ValueError("official card does not confirm expected license")
    raw_rows = fetch(ROWS_URL, 2 * 1024 * 1024)
    rows = json.loads(raw_rows)["rows"]
    if [row["row_idx"] for row in rows] != list(range(10)):
        raise ValueError("fixed row selection incomplete")
    selected = []
    for item in rows:
        row, index = item["row"], item["row_idx"]
        truth = row["raw_transcription"]
        if not isinstance(truth, str) or not truth.strip() or not 0 < row["num_samples"] <= 480000:
            raise ValueError("fixed row fails pre-inference admission; do not substitute")
        src = row["audio"][0]["src"]
        if urllib.parse.urlparse(src).path != f"/cached-assets/google/fleurs/--/{REVISION}/--/en_us/validation/{index}/audio/audio.wav":
            raise ValueError("preview asset is not pinned to the selected source row")
        selected.append({"config": "en_us", "split": "validation", "row_index": index,
                         "source_id": row["id"], "source_filename": Path(row["path"]).name,
                         "num_samples": row["num_samples"], "duration_seconds": row["num_samples"] / 16000,
                         "publisher_transcript": truth, "transcript_sha256": sha(truth.encode()),
                         "audio_url": src, "local_filename": f"english/fleurs-en_us-validation-row-{index:02d}.wav"})
    selection = {"schema": "nexusai.speech-sample-preregistration/v1", "frozen_at": datetime.now(timezone.utc).isoformat(),
                 "dataset": "google/fleurs", "revision": REVISION, "license": "CC-BY-4.0",
                 "selection_method": "fixed validation rows 0-9, selected before inference; no replacement",
                 "split_choice_reason": "official test preview and rows failed server scan-size limit before inference",
                 "seed": None, "audio_count": 10, "max_clip_seconds": 30, "max_audio_bytes": MAX_AUDIO_TOTAL,
                 "oracle": "publisher raw_transcription only; no ASR/LLM/transliteration oracle",
                 "existing_urdu_comparison": "Separate two-clip historical FLEURS set; no new Urdu acquisition",
                 "classification_policy": "No post-hoc accuracy threshold or model tuning. Report inference functionality separately from live product acceptance; manual intelligibility unverified until reviewed.",
                 "metadata_download_bytes": len(metadata) + len(card) + len(raw_rows),
                 "rows_response_sha256": sha(raw_rows), "card_sha256": sha(card), "clips": selected}
    for name, data in [("official-card.md", card), ("official-metadata.json", metadata), ("official-rows.json", raw_rows)]:
        with (root / name).open("xb") as out:
            out.write(data)
    save_new(root / "selection.json", selection)
    digest = sha((root / "selection.json").read_bytes())
    save_new(root / "selection-seal.json", {"sha256": digest, "audio_inference_performed": False})
    save_new(root / "attribution.json", {"dataset": "Google FLEURS", "source": "https://huggingface.co/datasets/google/fleurs",
        "revision": REVISION, "license": "CC-BY-4.0", "license_url": "https://creativecommons.org/licenses/by/4.0/",
        "paper": "FLEURS: Few-shot Learning Evaluation of Universal Representations of Speech (Conneau et al., 2022)",
        "use": "Private local internal ASR validation only; no broader redistribution/legal admission claim",
        "changes": "No audio conversion or transcript editing", "official_card_sha256": sha(card)})
    print(json.dumps({"selection_frozen": True, "clips": 10, "duration_seconds": sum(x["duration_seconds"] for x in selected), "selection_sha256": digest}))


def load_selection():
    root = private_root()
    data = (root / "selection.json").read_bytes()
    if sha(data) != json.loads((root / "selection-seal.json").read_text())["sha256"]:
        raise ValueError("selection seal mismatch")
    return root, json.loads(data)


def acquire():
    root, selection = load_selection()
    if (root / "acquisition.json").exists():
        return verify()
    acquired = []
    total = 0
    for clip in selection["clips"]:
        path = root / clip["local_filename"]
        if path.exists():
            data = path.read_bytes()
        else:
            data = fetch(clip["audio_url"], min(MAX_FILE, MAX_AUDIO_TOTAL - total), audio=True)
            info = wav_info(data)
            if info["samples"] != clip["num_samples"]:
                raise ValueError("source metadata and downloaded WAV sample count disagree")
            path.parent.mkdir(parents=True, exist_ok=True)
            with path.open("xb") as output:
                output.write(data)
        info = wav_info(data)
        if info["samples"] != clip["num_samples"]:
            raise ValueError("sample count mismatch")
        total += len(data)
        if total > MAX_AUDIO_TOTAL:
            raise ValueError("total download budget exceeded")
        acquired.append({**clip, "audio_sha256": sha(data), "audio_bytes": len(data), "wav": info})
        print(json.dumps({"acquired_row": clip["row_index"], "bytes": len(data)}), flush=True)
    save_new(root / "acquisition.json", {"schema": "nexusai.speech-acquisition/v1", "selection_sha256": sha((root / "selection.json").read_bytes()),
        "audio_download_bytes": total, "metadata_download_bytes": selection["metadata_download_bytes"],
        "total_payload_bytes": total + selection["metadata_download_bytes"], "full_dataset_downloaded": False,
        "clips": acquired})
    verify()


def verify():
    root, selection = load_selection()
    receipt = json.loads((root / "acquisition.json").read_text(encoding="utf-8"))
    if receipt["selection_sha256"] != sha((root / "selection.json").read_bytes()):
        raise ValueError("acquisition selection digest mismatch")
    if len(receipt["clips"]) != 10 or len(selection["clips"]) != 10:
        raise ValueError("acquisition must contain exactly the ten selected clips")
    for clip, selected in zip(receipt["clips"], selection["clips"]):
        if any(clip[key] != value for key, value in selected.items()):
            raise ValueError("acquisition differs from frozen selection")
        data = (root / clip["local_filename"]).read_bytes()
        if sha(data) != clip["audio_sha256"] or len(data) != clip["audio_bytes"]:
            raise ValueError("acquired audio hash/size mismatch")
        if sha(clip["publisher_transcript"].encode()) != clip["transcript_sha256"]:
            raise ValueError("publisher transcript digest mismatch")
        if wav_info(data)["samples"] != clip["num_samples"]:
            raise ValueError("acquired audio sample count mismatch")
    if sha((root / "official-card.md").read_bytes()) != selection["card_sha256"]:
        raise ValueError("official license card digest mismatch")
    print(json.dumps({"FleursBoundedAcquisition": "PASS", "clips": 10, "audio_bytes": receipt["audio_download_bytes"], "total_payload_bytes": receipt["total_payload_bytes"]}))


def score(reference, hypothesis):
    ref, hyp = normalize(reference), normalize(hypothesis)
    words, chars = ref.split(), list(ref.replace(" ", ""))
    return {"word_edits": edit_distance(words, hyp.split()), "reference_words": len(words),
            "character_edits": edit_distance(chars, list(hyp.replace(" ", ""))), "reference_characters": len(chars),
            "normalized_exact": ref == hyp,
            "reference_number_tokens": re.findall(r"\d+(?:[.:/-]\d+)*", reference),
            "hypothesis_number_tokens": re.findall(r"\d+(?:[.:/-]\d+)*", hypothesis)}


def aggregate(results):
    result = {"clips": len(results), "duration_seconds": sum(x["duration_seconds"] for x in results),
              "failures": sum(bool(x.get("error")) for x in results), "wall_seconds": sum(x["wall_seconds"] for x in results)}
    for key in ["word_edits", "reference_words", "character_edits", "reference_characters"]:
        result[key] = sum(x["score"][key] for x in results)
    result["wer"] = result["word_edits"] / result["reference_words"] if result["reference_words"] else None
    result["cer"] = result["character_edits"] / result["reference_characters"] if result["reference_characters"] else None
    result["normalized_exact_sentence_rate"] = sum(x["score"]["normalized_exact"] for x in results) / len(results)
    result["clips_with_complete_segment_timing"] = sum(x["timing_complete"] for x in results)
    result["nonempty_transcripts"] = sum(bool(x["hypothesis"].strip()) for x in results)
    return result


def evaluate():
    from ingestion.forensic_records.media_pipeline import LocalAIASRProcessor
    from scripts.nxmmr_speech_resources import ASRResources
    root, selection = load_selection()
    verify()
    if (root / "evaluation.json").exists():
        raise ValueError("evaluation already exists; no unversioned replay/tuning")
    clips = [{**x, "path": root / x["local_filename"], "language": "en", "cohort": "english-validation-new-v1"}
             for x in json.loads((root / "acquisition.json").read_text(encoding="utf-8"))["clips"]]
    for name, split, row, expected_hash in [
        ("clear-urdu", "test", 1, "c499e06e26b8cbf37386b88bba26f37045439a775e964bdad8634376435a056e"),
        ("conversational-urdu", "validation", 4, "f016076b153c2ed6be21fa5f7a4bbc22b3e391d40080b9fa3de3b8a2dd517550")]:
        path = REPO / "local-acceptance-inputs/pakistan-audio" / f"{name}.wav"
        truth = path.with_suffix(".ground-truth.txt").read_text(encoding="utf-8-sig").strip()
        if sha(path.read_bytes()) != expected_hash:
            raise ValueError("existing Urdu source hash mismatch")
        clips.append({"language": "ur", "cohort": "urdu-existing-two-clips-v1", "path": path,
            "row_index": row, "split": split, "config": "ur_pk", "duration_seconds": wav_info(path.read_bytes())["duration_seconds"],
            "publisher_transcript": truth, "audio_sha256": expected_hash})
    save_new(root / "evaluation-preregistration.json", {"selection_sha256": sha((root / "selection.json").read_bytes()),
        "model": "faster-whisper-small-ur", "endpoint": "http://127.0.0.1:8080/v1/audio/transcriptions",
        "normalization": "NFKC + casefold + punctuation/symbols to spaces + collapsed whitespace; CER excludes spaces",
        "aggregation": "micro WER/CER by summed edit/reference counts; failures scored as empty hypotheses; cohorts separate",
        "clips": [{k: v for k, v in x.items() if k != "path"} for x in clips]})
    processor = LocalAIASRProcessor("http://127.0.0.1:8080/v1/audio/transcriptions", "faster-whisper-small-ur", 300)
    results = []
    for index, clip in enumerate(clips):
        with ASRResources() as resources:
            started, error, observations = time.perf_counter(), None, ()
            try:
                observations = processor.transcribe(clip["path"], evidence_id=f"private-speech-{index}", version_id="v1",
                    source_file=clip["path"].name, language=clip["language"])
            except Exception as exc:
                error = type(exc).__name__
            latency = time.perf_counter() - started
        segments = [x for x in observations if x.observation_type == "audio_transcript_segment"]
        hypothesis = " ".join(x.payload["text"] for x in segments)
        record = {"cohort": clip["cohort"], "language": clip["language"], "row_index": clip["row_index"],
            "split": clip["split"], "audio_sha256": clip["audio_sha256"], "duration_seconds": clip["duration_seconds"],
            "reference": clip["publisher_transcript"], "hypothesis": hypothesis, "error": error,
            "wall_seconds": latency, "resources": resources.receipt, "score": score(clip["publisher_transcript"], hypothesis),
            "timing_complete": bool(segments) and all(x.payload.get("timing_status") == "reported" for x in segments),
            "observations": [{"id": x.observation_id, "type": x.observation_type, "locator": x.citation_locator, "payload": x.payload} for x in observations]}
        results.append(record)
        save_new(root / f"results/clip-{index:02d}.json", record)
        print(json.dumps({"evaluated_clip": index, "language": clip["language"], "failure": error, "seconds": round(latency, 3)}), flush=True)
    summary = {language: aggregate([x for x in results if x["language"] == language]) for language in ["en", "ur"]}
    save_new(root / "evaluation.json", {"selection_sha256": sha((root / "selection.json").read_bytes()),
        "model": processor.model, "adapter_revision": processor.processor_revision, "summary": summary,
        "resources": "Per-clip ASR-process CPU deltas and 50ms sampled RSS; lifetime high-water is separately labeled",
        "live_retained_mutation": False, "results": results})
    print(json.dumps(summary))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["preregister", "acquire", "verify", "evaluate"])
    args = parser.parse_args()
    globals()[args.command]()
