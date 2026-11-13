"""Picks the public clips for the Orion Lahore speech pack and writes them as WAV files with their reference texts.

Run in the evaluation container (see run_speech_baseline.sh): the parquet files with the audio are on the mounted data folder.

    python tools/media_data/export_orion_speech_pack.py OUT_DIR

Rules (docs/data/speech-lab-results-20261112.md, section 10):
* only clips of the DEVELOPMENT partition of the speech lab are used (the sealed clips were read once for the gates and stay unused);
* read speech (FLEURS, CC-BY-4.0, Pashto / Urdu / English) of 5 to 11 seconds, and spontaneous Pashto from the Omnilingual corpus of up to 20 seconds;
* the order is the lab's own hash order, so the choice is reproducible and not hand-picked for how well a model reads it.
Writes OUT_DIR/<name>.wav (16 kHz mono) and OUT_DIR/pack.json (name, language, source, key, seconds, speaker gender when the corpus has it, reference text).
"""
import io
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import pyarrow.parquet as pq  # noqa: E402
import soundfile  # noqa: E402
import speech_lab as sl  # noqa: E402

PLAN = (
    # (set, language, how many, shortest, longest seconds)
    ("ps_af", "ps", 3, 5.0, 11.0),
    ("pst_Arab", "ps", 2, 4.0, 20.0),
    ("ur_pk", "ur", 3, 5.0, 11.0),
    ("en_us", "en", 3, 5.0, 11.0),
)


def fleurs_candidates(name, low, high):
    path = os.path.join(sl.DATA, "fleurs", name + "-test.parquet")
    table = pq.read_table(path, columns=["id", "transcription", "num_samples", "gender"]).to_pylist()
    rows = []
    for index, r in enumerate(table):
        key = "%s-%s" % (name, r["id"])
        seconds = r["num_samples"] / 16000.0
        if sl.partition(key) == "dev" and low <= seconds <= high and len(r["transcription"]) >= 40:
            rows.append({"key": key, "path": path, "row": index, "seconds": seconds, "gender": r["gender"], "text": r["transcription"]})
    return sorted(rows, key=lambda r: sl.hashlib.sha256(r["key"].encode("utf-8")).hexdigest())


def omni_candidates(name, low, high, look=40):
    """The Omnilingual files hold no duration column: the first `look` development clips in hash order are decoded and the short enough ones kept."""
    clips, _ = sl.load_clips(name, "dev", look)
    return [{"key": c["key"], "samples": c["samples"], "seconds": c["seconds"], "gender": None, "text": c["text"]} for c in clips if low <= c["seconds"] <= high and len(c["text"]) >= 15]


def read_fleurs_audio(chosen):
    wanted = {}
    for item in chosen:
        wanted.setdefault(item["path"], {})[item["row"]] = item
    for path, rows in wanted.items():
        offset = 0
        for batch in pq.ParquetFile(path).iter_batches(batch_size=8, columns=["audio"]):
            for j, row in enumerate(batch.to_pylist()):
                if offset + j in rows:
                    samples, rate = soundfile.read(io.BytesIO(row["audio"]["bytes"]), dtype="float32")
                    if samples.ndim > 1:
                        samples = samples.mean(axis=1)
                    if rate != 16000:
                        from math import gcd
                        from scipy.signal import resample_poly
                        g = gcd(int(rate), 16000)
                        samples = resample_poly(samples, 16000 // g, int(rate) // g).astype("float32")
                    rows[offset + j]["samples"] = samples
            offset += batch.num_rows


def main(out_dir):
    os.makedirs(out_dir, exist_ok=True)
    pack = []
    for name, language, count, low, high in PLAN:
        if name in sl.FLEURS:
            chosen = fleurs_candidates(name, low, high)[:count]
            read_fleurs_audio(chosen)
            source = "FLEURS (CC-BY-4.0), read speech"
        else:
            chosen = omni_candidates(name, low, high)[:count]
            source = "Omnilingual ASR corpus slice %s, spontaneous speech" % name
        for item in chosen:
            pack.append({"language": language, "set": name, "source": source, "key": item["key"], "seconds": round(len(item["samples"]) / 16000.0, 2), "gender": item["gender"], "reference": item["text"], "samples": item["samples"]})
        print(name, language, "chosen", len(chosen), "of", count)
    # neutral file names that do not give the language away; the order mixes the languages so that no pattern in the names is a clue
    order = [p for group in zip(*[[x for x in pack if x["language"] == lg] for lg in ("ur", "ps", "en")]) for p in group]
    order += [x for x in pack if x not in order]
    manifest = []
    for number, item in enumerate(order, start=1):
        name = "voice-note-%02d.wav" % number
        soundfile.write(os.path.join(out_dir, name), item.pop("samples"), 16000)
        item["file"] = name
        manifest.append(item)
    json.dump(manifest, open(os.path.join(out_dir, "pack.json"), "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    for m in manifest:
        print(m["file"], m["language"], m["set"], m["seconds"], "s", m["gender"] or "-", "|", m["reference"][:70])


if __name__ == "__main__":
    main(sys.argv[1])
