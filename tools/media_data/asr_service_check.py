#!/usr/bin/env python3
"""Measure a speech model THROUGH the running LocalAI service (the path the worker uses), on the dev clips exported by C:/tmp/nexusai-media-data/export_clips.py.

    python -X utf8 tools/media_data/asr_service_check.py --model faster-whisper-turbo --lang ur_pk --limit 24

Per clip: word error against the FLEURS transcription after the same written normalisation as asr_eval.py, the share of letters in the expected script, the wall time of the request and the
real-time factor (wall seconds per second of audio, which includes the service's own overhead). The first request is reported apart because it includes loading the model. While the clips run, the
memory of the LocalAI container (docker stats) and the free memory of the Docker VM are sampled, so a model that does not fit next to the language model shows up here and not in production.
Standard library only; nothing leaves the machine (http://localhost:8080 is the LocalAI port).
"""
import argparse
import json
import re
import statistics
import subprocess
import threading
import time
import unicodedata
import urllib.request
import uuid
from pathlib import Path

CLIPS = Path("C:/tmp/nexusai-media-data/debug/asr_clips")
LANG = {"ur_pk": "ur", "en_us": "en", "pa_in": "pa", "ps_af": "ps"}
SCRIPT = {"ur": (0x0600, 0x06FF), "ps": (0x0600, 0x06FF), "pa": (0x0A00, 0x0A7F), "en": (0x0041, 0x024F)}
DIGITS = {**{ord(c): str(i) for i, c in enumerate("٠١٢٣٤٥٦٧٨٩")}, **{ord(c): str(i) for i, c in enumerate("۰۱۲۳۴۵۶۷۸۹")}}


def normalise(text):
    text = unicodedata.normalize("NFKC", text).lower().translate(DIGITS)
    text = "".join(ch for ch in text if unicodedata.category(ch)[0] not in ("P", "S"))
    return re.sub(r"\s+", " ", text).strip()


def edit_distance(a, b):
    previous = list(range(len(b) + 1))
    for i, x in enumerate(a, 1):
        current = [i]
        for j, y in enumerate(b, 1):
            current.append(min(previous[j] + 1, current[j - 1] + 1, previous[j - 1] + (x != y)))
        previous = current
    return previous[-1]


def looping(reference, hypothesis):
    """A 3-word sequence repeated five times or more, or an output more than three times the reference length (same rule as asr_eval.py)."""
    hyp = hypothesis.split()
    if len(hyp) > 3 * max(1, len(reference.split())) + 10:
        return True
    grams = {}
    for i in range(len(hyp) - 2):
        grams[tuple(hyp[i:i + 3])] = grams.get(tuple(hyp[i:i + 3]), 0) + 1
    return any(v >= 5 for v in grams.values())


def script_share(text, lang):
    low, high = SCRIPT[lang]
    letters = [ch for ch in text if unicodedata.category(ch).startswith("L")]
    return sum(1 for ch in letters if low <= ord(ch) <= high) / len(letters) if letters else 0.0


def transcribe(url, model, language, path, timeout):
    boundary = f"nexusai-{uuid.uuid4().hex}"
    parts = []
    for name, value in (("model", model), ("language", language), ("response_format", "verbose_json")):
        if value is None:
            continue
        parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"\r\n\r\n{value}\r\n'.encode())
    parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{path.name}"\r\nContent-Type: audio/wav\r\n\r\n'.encode() + path.read_bytes() + b"\r\n")
    parts.append(f"--{boundary}--\r\n".encode())
    request = urllib.request.Request(url, data=b"".join(parts), headers={"Content-Type": f"multipart/form-data; boundary={boundary}"})
    started = time.time()
    with urllib.request.urlopen(request, timeout=timeout) as response:
        body = json.loads(response.read().decode("utf-8"))
    return body, time.time() - started


def sample_memory(stop, samples, container):
    while not stop.is_set():
        try:
            used = subprocess.run(["docker", "stats", "--no-stream", "--format", "{{.MemUsage}}", container], capture_output=True, text=True, timeout=20).stdout.split("/")[0].strip()
            free = subprocess.run(["docker", "exec", container, "sh", "-c", "grep MemAvailable /proc/meminfo"], capture_output=True, text=True, timeout=20).stdout.split()
            samples.append((used, int(free[1]) / 1048576 if len(free) > 1 else None))
        except Exception:
            pass
        stop.wait(10)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", required=True)
    parser.add_argument("--lang", required=True, choices=sorted(LANG))
    parser.add_argument("--limit", type=int, default=24)
    parser.add_argument("--url", default="http://localhost:8080/v1/audio/transcriptions")
    parser.add_argument("--container", default="nexusai-api-1-prev-gbnf")
    parser.add_argument("--timeout", type=int, default=900)
    parser.add_argument("--auto-language", action="store_true", help="do not tell the service the language (what the worker does when the upload has no asr_language): reports how often it detects the right one")
    args = parser.parse_args()
    manifest = [m for m in json.load(open(CLIPS / "manifest.json", encoding="utf-8")) if m["lang"] == args.lang][: args.limit]
    lang = LANG[args.lang]
    samples, stop = [], threading.Event()
    sampler = threading.Thread(target=sample_memory, args=(stop, samples, args.container), daemon=True)
    sampler.start()
    rows = []
    for item in manifest:
        body, wall = transcribe(args.url, args.model, None if args.auto_language else lang, CLIPS / item["file"], args.timeout)
        ref, hyp = normalise(item["text"]), normalise(body.get("text", ""))
        words = ref.split()
        rows.append({"file": item["file"], "wer": edit_distance(words, hyp.split()) / max(1, len(words)), "script": script_share(hyp, lang), "seconds": item["seconds"], "wall": wall,
                     "segments": len(body.get("segments", [])), "empty": not hyp, "loop": looping(ref, hyp), "detected": str(body.get("language") or "")})
    stop.set()
    audio = sum(r["seconds"] for r in rows)
    steady = rows[1:] or rows
    print(f"{args.model} {args.lang}: {len(rows)} clips, {audio / 60:.1f} min of audio")
    print(f"  WER mean {100 * statistics.mean(r['wer'] for r in rows):.1f}%  median {100 * statistics.median(r['wer'] for r in rows):.1f}%   script share {100 * statistics.mean(r['script'] for r in rows):.1f}%   empty {sum(r['empty'] for r in rows)}   loops {sum(r['loop'] for r in rows)}/{len(rows)}")
    print(f"  first request {rows[0]['wall']:.1f} s (includes loading the model);  RTF all {sum(r['wall'] for r in rows) / audio:.2f}   RTF without the first {sum(r['wall'] for r in steady) / sum(r['seconds'] for r in steady):.2f}")
    if args.auto_language:
        print(f"  language detected correctly ({lang}) on {sum(r['detected'] == lang for r in rows)} of {len(rows)} clips; detected: " + str({d: sum(1 for r in rows if r['detected'] == d) for d in sorted({r['detected'] for r in rows})}))
    print(f"  every clip has at least one timed segment: {all(r['segments'] >= 1 for r in rows)}")
    if samples:
        frees = [f for _, f in samples if f is not None]
        print(f"  container memory during the run: last {samples[-1][0]};  VM memory available: lowest {min(frees):.2f} GiB over {len(frees)} samples" if frees else f"  container memory: {samples[-1][0]}")
    out = Path("C:/tmp/nexusai-media-data/results/asr") / f"service{'-auto' if args.auto_language else ''}-{args.lang}-{args.model}-{time.strftime('%Y%m%d-%H%M%S')}.json"
    out.write_text(json.dumps({"model": args.model, "lang": args.lang, "rows": rows, "memory_samples": samples}, ensure_ascii=False), encoding="utf-8")
    print("  result file:", out)


if __name__ == "__main__":
    main()
