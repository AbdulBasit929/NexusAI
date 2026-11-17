#!/usr/bin/env python3
"""Reads real Nastaliq newspaper text with the readers of docs/data/urdu-nastaliq-reader-preregistration-20261116.md and scores them (character and word error, loops, seconds, memory).

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data:ro -v C:/tmp/nexusai-media-data/results/nastaliq:/out -v <repo>:/repo:ro nexusai-media-eval:latest \\
        python /repo/tools/media_data/nastaliq_lab.py --reader tesseract --part dev --out /out/t-dev.jsonl

Readers: tesseract (the live Urdu route: tessdata best, urd+eng, page mode 3, one thread), qaari (Qwen2-VL-2B-Instruct with the Qaari-0.1 Urdu LoRA merged by hand, the card's prompt, greedy, bf16 on CPU),
base (the same model without the adapter, the control). Rows: Urdu Newspaper Benchmark (ledger DAT-024: no licence stated, private evaluation only; the images and the transcriptions never leave
the data folder, and the per-row output written to --out holds the reader's text, so --out must be outside the repository). Rows are chosen by hash, fixed before the first run: sha256(str(row index)),
first hex digit 0 = development, anything else = sealed, each taken in hash order.
"""
import argparse
import glob
import hashlib
import io
import json
import os
import random
import re
import resource
import statistics
import subprocess
import sys
import tempfile
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import editdistance  # noqa: E402
import ocr_eval as oe  # noqa: E402  (normalise: NFKC, zero-width removed, whitespace collapsed; the same measure as the earlier OCR results)

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
PROMPT = "Below is the image of one page of a document, as well as some raw textual content that was previously extracted for it. Just return the plain text representation of this document as if you were reading it naturally. Do not hallucinate."
LORA_ALPHA, LORA_RANK = 16, 16


def rows(part):
    import pyarrow.parquet as pq
    path = glob.glob(os.path.join(DATA, "datasets/urdu-newspaper-benchmark/*/data/*.parquet"))[0]
    parquet = pq.ParquetFile(path)
    keyed = sorted(((hashlib.sha256(str(i).encode()).hexdigest(), i) for i in range(parquet.metadata.num_rows)))
    if part == "dev":
        chosen = [k for k in keyed if k[0][0] == "0"][:12]
    elif part == "sealed":
        chosen = [k for k in keyed if k[0][0] != "0"][:60]
    elif part == "fresh":   # the rows of the sealed side AFTER the 60 that the bake-off read: no EasyOCR reading of them exists (product acceptance, 2026-11-17)
        chosen = [k for k in keyed if k[0][0] != "0"][60:]
    else:
        chosen = keyed
    # Memory only (2026-11-17, amendment A2 of the bake-off plan): the whole table used to be turned into Python objects, which with the EasyOCR reader beside it exceeded the 3 GB cap.
    # The rows chosen above are the same; only these are held, read in small batches.
    wanted, held, index = {i for _, i in chosen}, {}, 0
    for batch in parquet.iter_batches(batch_size=4, columns=["high_resolution", "transcription"]):
        for record in batch.to_pylist():
            if index in wanted:
                held[index] = (record["high_resolution"]["bytes"], record["transcription"])
            index += 1
    for key, i in chosen:
        yield i, key, held[i][0], held[i][1]


def read_tesseract(image_bytes):
    with tempfile.TemporaryDirectory() as tmp:
        path = os.path.join(tmp, "page.png")
        open(path, "wb").write(image_bytes)
        out = subprocess.run(["tesseract", path, "stdout", "-l", "urd+eng", "--psm", "3", "--tessdata-dir", os.path.join(DATA, "tessdata/best")],
                             capture_output=True, text=True, encoding="utf-8", env=dict(os.environ, OMP_THREAD_LIMIT="1"), timeout=300)
    return out.stdout, None


class Vlm:
    def __init__(self, merge, device="cpu", max_pixels=None):
        import torch
        from safetensors.torch import load_file
        from transformers import AutoProcessor, Qwen2VLForConditionalGeneration
        torch.set_num_threads(int(os.environ.get("LAB_THREADS", "16")))
        base = glob.glob(os.path.join(DATA, "models/ocr/qwen2-vl-2b-instruct/*"))[0]
        started = time.time()
        self.model = Qwen2VLForConditionalGeneration.from_pretrained(base, torch_dtype=torch.float16 if device == "cuda" else torch.bfloat16).eval()
        self.processor = AutoProcessor.from_pretrained(base, **({"max_pixels": max_pixels} if max_pixels else {}))
        self.max_pixels = max_pixels
        self.torch = torch
        self.device = device
        merged = 0
        if merge:
            adapter = glob.glob(os.path.join(DATA, "models/ocr/qaari-0.1-urdu-adapter/*/adapter_model.safetensors"))[0]
            tensors = load_file(adapter)
            params = dict(self.model.named_parameters())
            scale = LORA_ALPHA / LORA_RANK
            for key in tensors:
                if not key.endswith(".lora_A.weight"):
                    continue
                name = key[len("base_model.model."):-len(".lora_A.weight")] + ".weight"
                weight = params[name]          # a KeyError here means the layout differs: stop, do not guess
                delta = (tensors[key.replace("lora_A", "lora_B")].float() @ tensors[key].float()) * scale
                weight.data.copy_((weight.data.float() + delta).to(weight.dtype))
                merged += 1
            assert merged * 2 == len(tensors), (merged, len(tensors))
        if device == "cuda":
            self.model.to("cuda")
        print("loaded in %.0f s; %d adapter modules merged; peak %.2f GB" % (time.time() - started, merged, resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1e6), flush=True)

    def read(self, image_bytes):
        from PIL import Image
        image = Image.open(io.BytesIO(image_bytes)).convert("RGB")
        messages = [{"role": "user", "content": [{"type": "image"}, {"type": "text", "text": PROMPT}]}]
        text = self.processor.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)
        inputs = self.processor(text=[text], images=[image], padding=True, return_tensors="pt")
        inputs = inputs.to(self.device)
        with self.torch.no_grad():
            ids = self.model.generate(**inputs, max_new_tokens=2000, do_sample=False)
        new = ids[:, inputs["input_ids"].shape[1]:]
        return self.processor.batch_decode(new, skip_special_tokens=True, clean_up_tokenization_spaces=False)[0], int(new.shape[1])


def image_pixels(image_bytes):
    from PIL import Image
    w, h = Image.open(io.BytesIO(image_bytes)).size
    return w * h


def looped(reference, hypothesis):
    ref = oe.normalise(reference).replace(" ", "")
    hyp = oe.normalise(hypothesis).replace(" ", "")
    if ref and len(hyp) > 2 * len(ref):
        return True
    words = oe.normalise(hypothesis).split()
    grams = {}
    for i in range(len(words) - 7):
        g = tuple(words[i:i + 8])
        grams[g] = grams.get(g, 0) + 1
    return any(n > 5 for n in grams.values())


def score(reference, hypothesis):
    ref_c = oe.normalise(reference).replace(" ", "")
    hyp_c = oe.normalise(hypothesis).replace(" ", "")
    ref_w, hyp_w = oe.normalise(reference).split(), oe.normalise(hypothesis).split()
    return {"ref_chars": len(ref_c), "hyp_chars": len(hyp_c), "edits": editdistance.eval(ref_c, hyp_c), "ref_words": len(ref_w), "word_edits": editdistance.eval(ref_w, hyp_w)}


def summary(records):
    if not records:
        return
    edits, chars = sum(r["edits"] for r in records), sum(r["ref_chars"] for r in records)
    wedits, words = sum(r["word_edits"] for r in records), sum(r["ref_words"] for r in records)
    rng = random.Random(1)
    draws = sorted(sum(r["edits"] for r in s) / sum(r["ref_chars"] for r in s) for s in (rng.choices(records, k=len(records)) for _ in range(2000)))
    print("rows %d | CER pooled %.1f%% [95%% %.1f, %.1f] | mean per row %.1f%% | WER pooled %.1f%% | loops %d | seconds median %.1f max %.1f | peak %.2f GB" % (
        len(records), 100 * edits / chars, 100 * draws[50], 100 * draws[1949], 100 * statistics.mean(r["edits"] / max(r["ref_chars"], 1) for r in records), 100 * wedits / words,
        sum(1 for r in records if r["loop"]), statistics.median(r["seconds"] for r in records), max(r["seconds"] for r in records), resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1e6), flush=True)


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--reader", required=True, choices=("tesseract", "qaari", "base", "easyocr"))
    ap.add_argument("--part", required=True, choices=("dev", "sealed", "fresh", "all"))
    ap.add_argument("--limit", type=int, default=0, help="the first N rows in hash order (0 = all of the part)")
    ap.add_argument("--out", required=True)
    ap.add_argument("--max-pixels", type=int, default=0, help="cap the image size in pixels (amendment A5: the CPU laptop cannot hold the vision attention of a full-size page); 0 = the processor default, no resizing")
    ap.add_argument("--device", default="cpu", choices=("cpu", "cuda"), help="cuda runs the same weights in float16 (amendment A4); nothing else changes")
    args = ap.parse_args(argv)
    if os.path.abspath(args.out).startswith("/repo") or os.path.commonpath([os.path.abspath(args.out), os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))]) == os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))):
        raise SystemExit("--out holds the reader's text of unlicensed newspaper pages and must be outside the repository")
    vlm = Vlm(merge=args.reader == "qaari", device=args.device, max_pixels=args.max_pixels) if args.reader in ("qaari", "base") else None
    easy = None
    if args.reader == "easyocr":
        import easyocr_reader
        easy = easyocr_reader.make_reader(["ur", "en"], DATA)
    done = {json.loads(l)["row"] for l in open(args.out, encoding="utf-8")} if os.path.exists(args.out) else set()
    records = [json.loads(l) for l in open(args.out, encoding="utf-8")] if done else []
    for n, (i, key, image, reference) in enumerate(rows(args.part), 1):
        if args.limit and n > args.limit:
            break
        if i in done:
            continue
        started = time.time()
        if args.reader == "easyocr":
            text, tokens = easyocr_reader.read_bytes(easy, image)[0], None
        else:
            text, tokens = read_tesseract(image) if args.reader == "tesseract" else vlm.read(image)
        record = dict(row=i, key=key[:12], part=args.part, reader=args.reader, max_pixels=args.max_pixels, resized=bool(args.max_pixels and image_pixels(image) > args.max_pixels), seconds=round(time.time() - started, 1), tokens=tokens, loop=looped(reference, text), hyp=text, **score(reference, text))
        records.append(record)
        with open(args.out, "a", encoding="utf-8", newline="\n") as handle:
            handle.write(json.dumps(record, ensure_ascii=False) + "\n")
        print("row %4d %5.1f s  CER %5.1f%%  chars %4d -> %4d%s" % (i, record["seconds"], 100 * record["edits"] / max(record["ref_chars"], 1), record["ref_chars"], record["hyp_chars"], "  LOOP" if record["loop"] else ""), flush=True)
    summary(records)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
