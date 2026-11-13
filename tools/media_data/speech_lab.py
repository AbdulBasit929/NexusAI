#!/usr/bin/env python3
"""Speech lab (WP-M1) measurement, as fixed in docs/data/speech-lab-preregistration-20261112.md: one model against one evaluation set on one partition.

    docker run --rm --network none --cpus 8 -m 5g -v C:/tmp/nexusai-media-data:/data:ro -v C:/tmp/nexusai-media-data/results:/data/results -v <repo>:/repo:ro \
        --entrypoint python nexusai-media-eval-asr:latest /repo/tools/media_data/speech_lab.py --set pst_Arab --partition dev --model ps-small-cv24-ct2-int8 --told ps

Sets: FLEURS test splits (ur_pk, en_us, ps_af, pa_in, sd_in) and the Omnilingual corpus slices (pst_Arab, pbt_Arab, pbu_Arab, pnb_Arab; dev and test splits pooled). A clip belongs to `dev` or `sealed` by the
last hex digit of the SHA-256 of its key (0 to b dev, c to f sealed); within a partition clips are ordered by the SHA-256 of their key and the first N are taken, so the sample is the same on every run and
the list of keys (and its hash) is written into the result file. Sealed is read once per language, at the end. The model is told the language. Numbers: corpus word and character error (sum of edits over
sum of reference length) after normalisation N1 (the written one of 2026-10-21) and N2 (N1 plus the Arabic-script unification below), each with a 95% bootstrap interval over clips; the share of letters in
the expected script; repetition loops; empty outputs; the real-time factor. Models with a 40-second input limit read a long clip in pieces cut at the quietest point (see split_for_limit).

The pure functions (partition, selection, normalisation, error rates, interval, script share, loops, cutting) have no dependency and are tested by tools/media_data/test_speech_lab.py on the host; the
audio, the parquet files and the models are only touched in `run`.
"""
import argparse
import hashlib
import io
import json
import os
import random
import re
import sys
import time
import unicodedata

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
FLEURS = {"ur_pk": "ur", "en_us": "en", "ps_af": "ps", "pa_in": "pa", "sd_in": "sd"}
OMNI = {"pst_Arab": "ps", "pbt_Arab": "ps", "pbu_Arab": "ps", "pnb_Arab": "pnb"}
SCRIPTS = {"ur": ((0x0600, 0x06FF), (0x0750, 0x077F), (0xFB50, 0xFDFF), (0xFE70, 0xFEFF)), "ps": ((0x0600, 0x06FF), (0x0750, 0x077F), (0xFB50, 0xFDFF), (0xFE70, 0xFEFF)),
           "pnb": ((0x0600, 0x06FF), (0x0750, 0x077F), (0xFB50, 0xFDFF), (0xFE70, 0xFEFF)), "sd": ((0x0600, 0x06FF), (0x0750, 0x077F), (0xFB50, 0xFDFF), (0xFE70, 0xFEFF)),
           "pa": ((0x0A00, 0x0A7F),), "en": ((0x0041, 0x024F),)}
DIGITS = {**{ord(c): str(i) for i, c in enumerate("٠١٢٣٤٥٦٧٨٩")}, **{ord(c): str(i) for i, c in enumerate("۰۱۲۳۴۵۶۷۸۹")}}
# N2: removed (Arabic diacritics U+064B to U+065F, U+0670, tatweel, zero-width joiner and non-joiner, Arabic comma, semicolon and question mark) and unified (Arabic yeh and alef maksura to Farsi yeh,
# Arabic kaf to Keheh, Arabic heh and heh goal to heh goal). Fixed before the first measurement; not edited afterwards.
N2_REMOVE = {c: None for c in list(range(0x064B, 0x0660)) + [0x0670, 0x0640, 0x200C, 0x200D, 0x060C, 0x061B, 0x061F]}
N2_MAP = {0x064A: 0x06CC, 0x0649: 0x06CC, 0x0643: 0x06A9, 0x0647: 0x06C1}
N2_TABLE = {**N2_REMOVE, **N2_MAP}


def partition(key):
    return "dev" if hashlib.sha256(key.encode("utf-8")).hexdigest()[-1] in "0123456789ab" else "sealed"


def select(keys, part, limit):
    """The first `limit` keys of the partition, ordered by the SHA-256 of the key (limit 0 = all)."""
    mine = sorted((k for k in keys if partition(k) == part), key=lambda k: hashlib.sha256(k.encode("utf-8")).hexdigest())
    return mine[:limit] if limit else mine


def normalise(text, version="N1"):
    text = unicodedata.normalize("NFKC", text or "").lower().translate(DIGITS)
    text = "".join(ch for ch in text if unicodedata.category(ch)[0] not in ("P", "S"))
    if version == "N2":
        text = text.translate(N2_TABLE)
    return re.sub(r"\s+", " ", text).strip()


def _edits(ref, hyp):
    try:
        import editdistance
        return editdistance.eval(ref, hyp)
    except ImportError:
        previous = list(range(len(hyp) + 1))
        for i, r in enumerate(ref, 1):
            current = [i]
            for j, h in enumerate(hyp, 1):
                current.append(min(previous[j] + 1, current[j - 1] + 1, previous[j - 1] + (r != h)))
            previous = current
        return previous[-1]


def clip_errors(reference, hypothesis):
    """(word edits, reference words, character edits, reference characters) for one clip, spaces ignored for characters."""
    rw, hw = reference.split(), hypothesis.split()
    rc, hc = list(reference.replace(" ", "")), list(hypothesis.replace(" ", ""))
    return _edits(rw, hw), len(rw), _edits(rc, hc), len(rc)


def corpus_rate(edits, lengths):
    return sum(edits) / sum(lengths) if sum(lengths) else 0.0


def bootstrap_interval(edits, lengths, resamples=2000, seed=1):
    """95% interval of the corpus rate by resampling clips with replacement (percentile method)."""
    rng = random.Random(seed)
    n = len(edits)
    rates = []
    for _ in range(resamples):
        idx = [rng.randrange(n) for _ in range(n)]
        rates.append(corpus_rate([edits[i] for i in idx], [lengths[i] for i in idx]))
    rates.sort()
    return rates[int(0.025 * resamples)], rates[int(0.975 * resamples) - 1]


def script_share(text, lang):
    ranges = SCRIPTS[lang]
    letters = [ch for ch in text if unicodedata.category(ch).startswith("L")]
    return sum(1 for ch in letters if any(lo <= ord(ch) <= hi for lo, hi in ranges)) / len(letters) if letters else 0.0


def looping(reference, hypothesis):
    hyp = hypothesis.split()
    if len(hyp) > 3 * max(1, len(reference.split())) + 10:
        return True
    grams = {}
    for i in range(len(hyp) - 2):
        g = tuple(hyp[i:i + 3])
        grams[g] = grams.get(g, 0) + 1
    return any(v >= 5 for v in grams.values())


def quietest_point(samples, start, stop, rate=16000, frame=0.02, smooth=0.2):
    """The sample index of the quietest 20 ms frame (energy smoothed over 200 ms) between start and stop."""
    size = max(1, int(frame * rate))
    energies = []
    for a in range(start, max(start + size, stop - size), size):
        chunk = samples[a:a + size]
        energies.append(sum(float(x) * float(x) for x in chunk) / max(1, len(chunk)))
    width = max(1, int(smooth / frame))
    best, best_value = 0, None
    for i in range(len(energies)):
        window = energies[max(0, i - width // 2):i + width // 2 + 1]
        value = sum(window) / len(window)
        if best_value is None or value < best_value:
            best, best_value = i, value
    return start + best * size + size // 2


def split_for_limit(samples, rate=16000, limit=38.0, low=20.0, high=36.0):
    """Pieces of at most `limit` seconds for a model with a 40-second input limit: each cut at the quietest point between `low` and `high` seconds of the remaining audio."""
    pieces, start = [], 0
    while (len(samples) - start) / rate > limit:
        cut = quietest_point(samples, start + int(low * rate), start + int(high * rate), rate)
        pieces.append(samples[start:cut])
        start = cut
    pieces.append(samples[start:])
    return pieces


def summarise(rows):
    """The figures of a run from its per-clip rows."""
    out = {"clips": len(rows), "audio_seconds": round(sum(r["seconds"] for r in rows), 1)}
    for version in ("N1", "N2"):
        we, wl = [r[version]["w_edits"] for r in rows], [r[version]["w_len"] for r in rows]
        ce, cl = [r[version]["c_edits"] for r in rows], [r[version]["c_len"] for r in rows]
        per_clip = sorted(r[version]["w_edits"] / r[version]["w_len"] if r[version]["w_len"] else 0.0 for r in rows)
        out[version] = {"wer": corpus_rate(we, wl), "wer_ci": bootstrap_interval(we, wl), "cer": corpus_rate(ce, cl), "cer_ci": bootstrap_interval(ce, cl),
                        "wer_median_clip": per_clip[len(per_clip) // 2] if per_clip else 0.0}
    total = sum(r["seconds"] for r in rows) or 1.0
    out.update({"script_share": sum(r["script"] for r in rows) / len(rows), "loops": sum(1 for r in rows if r["loop"]), "empty": sum(1 for r in rows if r["empty"]),
                "rtf": sum(r["compute"] for r in rows) / total})
    return out


# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- data and models


def load_clips(set_name, part, limit):
    """[{key, samples (16 kHz mono floats), seconds, text}] of the selected clips, in the order of the selection. The parquet files hold the audio inside (up to 450 MB each) and the Docker VM is small:
    the keys and texts are read without the audio, the selection is made, and the audio is then streamed in small batches keeping only the selected rows."""
    import pyarrow.parquet as pq
    import soundfile
    if set_name in FLEURS:
        paths = [os.path.join(DATA, "fleurs", set_name + "-test.parquet")]
        columns, key_of, text_of = ["id", "transcription"], (lambda r: "%s-%s" % (set_name, r["id"])), (lambda r: r["transcription"])
    else:
        paths = [os.path.join(DATA, "omnilingual", set_name, split + ".parquet") for split in ("dev", "test")]
        columns = ["speaker_id", "prompt_id", "segment_id", "raw_text"]
        key_of, text_of = (lambda r: "%s-%s-%s-%s" % (set_name, r["speaker_id"], r["prompt_id"], r["segment_id"])), (lambda r: r["raw_text"])
    where = {}      # key -> (path, row number in that file, reference text)
    for path in paths:
        for i, row in enumerate(pq.read_table(path, columns=columns).to_pylist()):
            where[key_of(row)] = (path, i, text_of(row))
    chosen = select(list(where), part, limit)
    audio = {}
    for path in paths:
        wanted = {where[k][1]: k for k in chosen if where[k][0] == path}
        if not wanted:
            continue
        offset = 0
        for batch in pq.ParquetFile(path).iter_batches(batch_size=8, columns=["audio"]):
            for j, row in enumerate(batch.to_pylist()):
                if offset + j in wanted:
                    audio[wanted[offset + j]] = row["audio"]["bytes"]
            offset += batch.num_rows
    clips = []
    for key in chosen:
        samples, rate = soundfile.read(io.BytesIO(audio[key]), dtype="float32")
        if samples.ndim > 1:
            samples = samples.mean(axis=1)
        if rate != 16000:
            from math import gcd
            from scipy.signal import resample_poly
            g = gcd(int(rate), 16000)
            samples = resample_poly(samples, 16000 // g, int(rate) // g).astype("float32")
        clips.append({"key": key, "samples": samples, "seconds": len(samples) / 16000.0, "text": where[key][2]})
    return clips, chosen


def mem_available_mib():
    """Memory the Docker VM still has free (MemAvailable), in MiB; 0 when the file cannot be read. Recorded so that a run's memory pressure is visible next to its real-time factor."""
    try:
        with open("/proc/meminfo", encoding="ascii") as handle:
            for line in handle:
                if line.startswith("MemAvailable:"):
                    return int(line.split()[1]) // 1024
    except OSError:
        pass
    return 0


def file_sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            h.update(block)
    return h.hexdigest()


def whisper_backend(model_dir, threads, beam):
    from faster_whisper import WhisperModel
    model = WhisperModel(model_dir, device="cpu", compute_type="int8", cpu_threads=threads)

    def read(samples, lang):
        segments, info = model.transcribe(samples, language=lang, beam_size=beam, condition_on_previous_text=False)
        return " ".join(s.text.strip() for s in segments), info.language
    return read


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--set", required=True, choices=sorted(list(FLEURS) + list(OMNI)))
    ap.add_argument("--partition", default="dev", choices=("dev", "sealed"))
    ap.add_argument("--limit", type=int, default=40)
    ap.add_argument("--model", required=True, help="a CTranslate2 folder under <data>/models/asr")
    ap.add_argument("--told", required=True, help="the language code given to the model (ur, en, ps, pa)")
    ap.add_argument("--threads", type=int, default=8)
    ap.add_argument("--beam", type=int, default=1)
    ap.add_argument("--label", default="")
    args = ap.parse_args(argv)
    lang = FLEURS.get(args.set) or OMNI[args.set]
    expected_script = lang
    folder = os.path.join(DATA, "models", "asr", args.model)
    model_sha = file_sha256(os.path.join(folder, "model.bin"))
    clips, keys = load_clips(args.set, args.partition, args.limit)
    print("%s %s: %d clips, %.1f min of audio; model %s (model.bin %s...); told %s; threads %d, beam %d" % (args.set, args.partition, len(clips), sum(c["seconds"] for c in clips) / 60, args.model, model_sha[:12], args.told, args.threads, args.beam), flush=True)
    read = whisper_backend(folder, args.threads, args.beam)
    free_start, free_low = mem_available_mib(), mem_available_mib()
    rows = []
    for clip in clips:
        t0 = time.time()
        text, detected = read(clip["samples"], args.told)
        compute = time.time() - t0
        free_low = min(free_low, mem_available_mib())
        row = {"key": clip["key"], "seconds": clip["seconds"], "compute": compute, "detected": detected, "reference": clip["text"], "hypothesis": text,
               "script": script_share(text, expected_script), "empty": not normalise(text), "loop": looping(normalise(clip["text"]), normalise(text))}
        for version in ("N1", "N2"):
            ref, hyp = normalise(clip["text"], version), normalise(text, version)
            we, wl, ce, cl = clip_errors(ref, hyp)
            row[version] = {"w_edits": we, "w_len": wl, "c_edits": ce, "c_len": cl}
        rows.append(row)
    summary = summarise(rows)
    stamp = time.strftime("%Y%m%d-%H%M%S")
    out_dir = os.path.join(DATA, "results", "asr-lab")
    os.makedirs(out_dir, exist_ok=True)
    name = "%s-%s-%s-told-%s%s-%s.json" % (args.set, args.partition, args.model, args.told, ("-" + args.label) if args.label else "", stamp)
    meta = {"set": args.set, "partition": args.partition, "model": args.model, "model_bin_sha256": model_sha, "told": args.told, "threads": args.threads, "beam": args.beam, "limit": args.limit,
            "keys_sha256": hashlib.sha256("\n".join(keys).encode("utf-8")).hexdigest(), "preregistration": "docs/data/speech-lab-preregistration-20261112.md", "when": stamp, "vm_free_mib_start": free_start, "vm_free_mib_lowest": free_low}
    with open(os.path.join(out_dir, name), "w", encoding="utf-8") as handle:
        json.dump({"meta": meta, "summary": summary, "rows": rows}, handle, ensure_ascii=False)
    for version in ("N1", "N2"):
        s = summary[version]
        print("  %s  word error %5.1f%% [%4.1f, %4.1f]   character error %5.1f%% [%4.1f, %4.1f]   median clip %5.1f%%" % (version, 100 * s["wer"], 100 * s["wer_ci"][0], 100 * s["wer_ci"][1], 100 * s["cer"], 100 * s["cer_ci"][0], 100 * s["cer_ci"][1], 100 * s["wer_median_clip"]))
    print("  script share %.1f%%   loops %d/%d   empty %d   real-time factor %.2f   VM free memory lowest %d MiB   -> %s" % (100 * summary["script_share"], summary["loops"], summary["clips"], summary["empty"], summary["rtf"], free_low, name))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
