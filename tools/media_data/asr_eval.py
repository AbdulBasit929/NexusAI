#!/usr/bin/env python3
"""Speech measurement for track A (docs/data/media-activation-programme-20261021.md): faster-whisper models against FLEURS test clips, on the dev partition.

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval-asr:latest \
        python /repo/tools/media_data/asr_eval.py --lang ur_pk --model faster-whisper-small --model faster-whisper-medium --limit 60

Per clip: word error and character error against the FLEURS transcription after the written normalisation (NFKC, lower case, punctuation and symbols removed, Arabic-Indic digits mapped to
ASCII, whitespace collapsed), the share of letters in the expected script, a repetition-loop flag (a 3-word sequence repeated five times or more, or an output more than three times the
reference length), the real-time factor (seconds of compute per second of audio, int8, CPU threads as set), and the language the model detected when it was not told. A result file per run is
written to /data/results/asr/. Clips belong to dev or sealed by the SHA-256 of their id; only dev is used to choose.
"""
import argparse
import hashlib
import io
import json
import os
import re
import sys
import time
import unicodedata
from pathlib import Path

import editdistance

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
LANG = {"ur_pk": "ur", "en_us": "en", "pa_in": "pa", "ps_af": "ps"}
SCRIPT = {"ur": (0x0600, 0x06FF), "ps": (0x0600, 0x06FF), "pa": (0x0A00, 0x0A7F), "en": (0x0041, 0x024F)}
DIGITS = {**{ord(c): str(i) for i, c in enumerate("٠١٢٣٤٥٦٧٨٩")}, **{ord(c): str(i) for i, c in enumerate("۰۱۲۳۴۵۶۷۸۹")}}


def normalise(text):
    text = unicodedata.normalize("NFKC", text).lower().translate(DIGITS)
    text = "".join(ch for ch in text if unicodedata.category(ch)[0] not in ("P", "S"))
    return re.sub(r"\s+", " ", text).strip()


def error_rate(reference, hypothesis, unit):
    ref = reference.split() if unit == "word" else list(reference.replace(" ", ""))
    hyp = hypothesis.split() if unit == "word" else list(hypothesis.replace(" ", ""))
    if not ref:
        return 0.0 if not hyp else 1.0
    return editdistance.eval(ref, hyp) / len(ref)


def script_share(text, lang):
    low, high = SCRIPT[lang]
    letters = [ch for ch in text if unicodedata.category(ch).startswith("L")]
    return sum(1 for ch in letters if low <= ord(ch) <= high) / len(letters) if letters else 0.0


def looping(reference, hypothesis):
    hyp = hypothesis.split()
    if len(hyp) > 3 * max(1, len(reference.split())) + 10:
        return True
    grams = {}
    for i in range(len(hyp) - 2):
        grams[tuple(hyp[i:i + 3])] = grams.get(tuple(hyp[i:i + 3]), 0) + 1
    return any(v >= 5 for v in grams.values())


def partition(key):
    return "dev" if hashlib.sha256(key.encode()).hexdigest()[-1] in "0123456789ab" else "sealed"


def clips(lang_dir, limit, part):
    import pyarrow.parquet as pq
    table = pq.read_table(Path(DATA) / "fleurs" / f"{lang_dir}-test.parquet")
    count = 0
    for index in range(table.num_rows):
        record = table.slice(index, 1).to_pylist()[0]
        key = f"{lang_dir}-{record.get('id', index)}"
        if partition(key) != part:
            continue
        audio = record["audio"]
        yield {"id": key, "bytes": audio["bytes"], "text": record["transcription"], "seconds": record["num_samples"] / 16000.0}
        count += 1
        if limit and count >= limit:
            return


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--lang", required=True, choices=sorted(LANG))
    parser.add_argument("--model", action="append", required=True)
    parser.add_argument("--limit", type=int, default=60)
    parser.add_argument("--partition", default="dev", choices=("dev", "sealed"))
    parser.add_argument("--threads", type=int, default=int(os.environ.get("ASR_THREADS", "6")))
    parser.add_argument("--beam", type=int, default=5)
    parser.add_argument("--no-language", action="store_true", help="let the model detect the language instead of being told")
    args = parser.parse_args()
    from faster_whisper import WhisperModel
    import soundfile
    lang = LANG[args.lang]
    items = list(clips(args.lang, args.limit, args.partition))
    audio_seconds = sum(i["seconds"] for i in items)
    print(f"{args.lang} {args.partition}: {len(items)} clips, {audio_seconds / 60:.1f} min of audio, threads {args.threads}")
    os.makedirs(os.path.join(DATA, "results", "asr"), exist_ok=True)
    for model_name in args.model:
        folder = Path(DATA) / "models" / "asr" / model_name
        model = WhisperModel(str(folder), device="cpu", compute_type="int8", cpu_threads=args.threads)
        rows, started = [], time.time()
        for item in items:
            samples, rate = soundfile.read(io.BytesIO(item["bytes"]), dtype="float32")
            if samples.ndim > 1:
                samples = samples.mean(axis=1)
            t0 = time.time()
            segments, info = model.transcribe(samples, language=None if args.no_language else lang, beam_size=args.beam, condition_on_previous_text=False)
            text = " ".join(s.text.strip() for s in segments)
            elapsed = time.time() - t0
            ref, hyp = normalise(item["text"]), normalise(text)
            rows.append({"id": item["id"], "wer": error_rate(ref, hyp, "word"), "cer": error_rate(ref, hyp, "char"), "script": script_share(hyp, lang), "loop": looping(ref, hyp),
                         "rtf": elapsed / item["seconds"], "seconds": item["seconds"], "detected": info.language, "empty": not hyp})
        wers = sorted(r["wer"] for r in rows)
        stamp = time.strftime("%Y%m%d-%H%M%S")
        out = os.path.join(DATA, "results", "asr", f"{args.lang}-{args.partition}-{model_name}{'-autolang' if args.no_language else ''}-{stamp}.json")
        json.dump({"lang": args.lang, "partition": args.partition, "model": model_name, "threads": args.threads, "beam": args.beam, "rows": rows}, open(out, "w"))
        total_audio = sum(r["seconds"] for r in rows)
        weighted_rtf = sum(r["rtf"] * r["seconds"] for r in rows) / total_audio
        corpus_wer = None
        print(f"\nmodel {model_name}  ({time.time() - started:.0f} s wall)  n={len(rows)}")
        print(f"  WER median {100 * wers[len(wers) // 2]:5.1f}%  mean {100 * sum(wers) / len(wers):5.1f}%   CER mean {100 * sum(r['cer'] for r in rows) / len(rows):5.1f}%")
        print(f"  script share {100 * sum(r['script'] for r in rows) / len(rows):5.1f}%   loops {sum(r['loop'] for r in rows)}/{len(rows)}   empty {sum(r['empty'] for r in rows)}   RTF {weighted_rtf:.2f}   detected {dict((l, sum(1 for r in rows if r['detected'] == l)) for l in set(r['detected'] for r in rows))}")
        del model


if __name__ == "__main__":
    main()
