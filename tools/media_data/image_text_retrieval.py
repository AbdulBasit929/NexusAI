#!/usr/bin/env python3
"""Text-to-image retrieval on Flickr30k 1K test, English and machine-translated Urdu (gates V-a, V-b, V-d of docs/data/face-image-search-preregistration-20261119.md).

    docker run --rm --network none --memory 3500m -v C:/tmp/nexusai-media-data:/data:ro -v C:/tmp/nexusai-media-data/work-faces:/work -v <repo>:/repo:ro \
        --entrypoint python nexusai-media-eval-face:20261119 /repo/tools/media_data/image_text_retrieval.py prep|translate|embed|score [--models siglip siglip2]

prep:      unpack the 1,000 test images (names checked) and read the captions (five English captions per image).
translate: the FIRST English caption of each image into Urdu with opus-mt-en-ur (Apache-2.0), unreviewed machine translation: the limit is stated in every result.
embed:     image and caption embeddings with the product's SigLIP v1 base and SigLIP 2 base, 4 threads; per-item times and peak memory are recorded.
score:     recall@1/5/10 of text-to-image inside the gallery of the partition (dev: last hex digit of SHA-256 of the file name in 0 to 3; sealed: the rest), and image-to-text recall@1. Sealed is scored once.
"""
import argparse
import hashlib
import json
import os
import resource
import sys
import time
import zipfile
from pathlib import Path

import numpy as np

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
WORK = Path(os.environ.get("NEXUSAI_FACE_WORK", "/work")) / "flickr1k"
MODELS = {"siglip": "/models/media/nexusai/models/google-siglip-base-patch16-224/7fd15f0689c79d79e38b1c2e2e2370a7bf2761ed",
          "siglip2": f"{DATA}/models/image/siglip2-base-patch16-224"}
DEV_DIGITS = "0123"


def partition(name):
    return "dev" if hashlib.sha256(name.encode()).hexdigest()[-1] in DEV_DIGITS else "sealed"


def prep():
    import csv
    WORK.mkdir(parents=True, exist_ok=True)
    out = WORK / "images"
    out.mkdir(exist_ok=True)
    with zipfile.ZipFile(Path(DATA) / "datasets" / "retrieval" / "flickr1k" / "images_flickr_1k_test.zip") as z:
        for member in z.namelist():
            if (out / member).resolve().parent != out.resolve() and not str((out / member).resolve()).startswith(str(out.resolve())):
                raise RuntimeError(f"unsafe path {member}")
        z.extractall(out)
    rows = []
    with open(Path(DATA) / "datasets" / "retrieval" / "flickr1k" / "test_1k_flickr.csv", encoding="utf-8", newline="") as handle:
        for row in csv.DictReader(handle):
            rows.append({"filename": row["filename"], "captions": json.loads(row["raw"])})
    json.dump(rows, open(WORK / "captions-en.json", "w"))
    files = list(out.rglob("*.jpg"))
    print(f"{len(rows)} images listed, {len(files)} jpg files unpacked, captions per image {sorted({len(r['captions']) for r in rows})}")


def translate():
    import torch
    from transformers import MarianMTModel, MarianTokenizer
    torch.set_num_threads(6)
    path = f"{DATA}/models/text/opus-mt-en-ur"
    tok, model = MarianTokenizer.from_pretrained(path), MarianMTModel.from_pretrained(path).eval()
    rows = json.load(open(WORK / "captions-en.json"))
    source = [r["captions"][0].strip() for r in rows]
    out, started = [], time.time()
    for i in range(0, len(source), 16):
        batch = tok(source[i:i + 16], return_tensors="pt", padding=True, truncation=True, max_length=128)
        with torch.no_grad():
            generated = model.generate(**batch, num_beams=2, max_new_tokens=96)
        out.extend(tok.batch_decode(generated, skip_special_tokens=True))
        if (i // 16) % 10 == 0:
            print(f"  translated {len(out)}/{len(source)} in {time.time() - started:.0f} s", flush=True)
    json.dump([{"filename": r["filename"], "en": s, "ur": u} for r, s, u in zip(rows, source, out)], open(WORK / "captions-ur.json", "w"), ensure_ascii=False)
    for s, u in list(zip(source, out))[:5]:
        print(" ", s, "->", u)


def load(model_key):
    import torch
    from transformers import AutoImageProcessor, AutoModel, AutoTokenizer
    torch.set_num_threads(4)
    path = MODELS[model_key]
    return torch, AutoImageProcessor.from_pretrained(path), AutoTokenizer.from_pretrained(path), AutoModel.from_pretrained(path).eval()


def embed(model_key):
    from PIL import Image
    torch, proc, tok, model = load(model_key)
    rows = json.load(open(WORK / "captions-en.json"))
    ur_rows = json.load(open(WORK / "captions-ur-claude.json", encoding="utf-8"))   # 150 sealed captions translated by the assistant (the Marian machine translation was unusable)
    out = {"model": model_key}
    t_img, images = [], []
    for k, r in enumerate(rows):
        image = Image.open(WORK / "images" / "images_flickr_1k_test" / r["filename"]).convert("RGB")
        t0 = time.perf_counter()
        with torch.no_grad():
            f = model.get_image_features(**proc(images=image, return_tensors="pt"))
        t_img.append(time.perf_counter() - t0)
        images.append((f / f.norm(dim=-1, keepdim=True))[0].numpy())
        if k % 200 == 0:
            print(f"  {model_key} images {k}/{len(rows)}", flush=True)

    def text_vectors(texts):
        vectors, times = [], []
        for i in range(0, len(texts), 32):
            batch = [t.lower() for t in texts[i:i + 32]]
            enc = tok(batch, padding="max_length", max_length=64, truncation=True, return_tensors="pt")
            t0 = time.perf_counter()
            with torch.no_grad():
                f = model.get_text_features(input_ids=enc["input_ids"])
            times.append((time.perf_counter() - t0) / len(batch))
            vectors.append((f / f.norm(dim=-1, keepdim=True)).numpy())
        return np.concatenate(vectors), float(np.median(times))
    en_texts = [c for r in rows for c in r["captions"][:5]]
    en_vec, t_en = text_vectors(en_texts)
    ur_vec, t_ur = text_vectors([r["ur"] for r in ur_rows])
    np.save(WORK / f"emb-{model_key}-images.npy", np.stack(images))
    np.save(WORK / f"emb-{model_key}-en.npy", en_vec)
    np.save(WORK / f"emb-{model_key}-ur.npy", ur_vec)
    out.update({"median_s_per_image": round(float(np.median(t_img)), 3), "median_s_per_caption_en": round(t_en, 4), "median_s_per_caption_ur": round(t_ur, 4), "peak_rss_mb": round(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024)})
    json.dump(out, open(WORK / f"embed-timing-{model_key}.json", "w"), indent=1)
    print(out)


def recall(sims, owner, ks=(1, 5, 10)):
    """sims: captions x images; owner: index of the true image of each caption."""
    order = np.argsort(-sims, axis=1)
    rank = np.array([int(np.where(order[i] == owner[i])[0][0]) for i in range(len(owner))])
    return {f"R@{k}": round(100 * float(np.mean(rank < k)), 2) for k in ks}


def score(models):
    rows = json.load(open(WORK / "captions-en.json"))
    names = [r["filename"] for r in rows]
    ur_files = [r["filename"] for r in json.load(open(WORK / "captions-ur-claude.json", encoding="utf-8"))]
    report = {}
    for key in models:
        images = np.load(WORK / f"emb-{key}-images.npy")
        en = np.load(WORK / f"emb-{key}-en.npy")
        ur = np.load(WORK / f"emb-{key}-ur.npy")
        for part in ("dev", "sealed"):
            idx = [i for i, n in enumerate(names) if partition(n) == part]
            pos = {i: k for k, i in enumerate(idx)}
            gallery = images[idx]
            cap_idx = [5 * i + j for i in idx for j in range(5)]
            owner_en = [pos[i] for i in idx for _ in range(5)]
            r_en = recall(en[cap_idx] @ gallery.T, owner_en)
            entry = {"images": len(idx), "text_to_image_en": r_en}
            top = np.argmax((en[cap_idx] @ gallery.T).T, axis=1)
            entry["image_to_text_en_R@1"] = round(100 * float(np.mean([owner_en[top[k]] == k for k in range(len(idx))])), 2)
            if part == "sealed":
                # Urdu: 150 sealed captions as queries against ALL sealed images; the English control uses the first English caption of the same 150 images
                name_to_i = {n: i for i, n in enumerate(names)}
                owner_ur = [pos[name_to_i[f]] for f in ur_files]
                entry["text_to_image_ur_150_hand_translated"] = recall(ur @ gallery.T, owner_ur)
                en_first = en[[5 * name_to_i[f] for f in ur_files]]
                entry["text_to_image_en_same_150_first_captions"] = recall(en_first @ gallery.T, owner_ur)
            report.setdefault(key, {})[part] = entry
            print(key, part, json.dumps(entry), flush=True)
    json.dump(report, open(WORK / "retrieval-score.json", "w"), indent=1)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("stage", choices=("prep", "translate", "embed", "score"))
    parser.add_argument("--models", nargs="+", default=["siglip", "siglip2"])
    a = parser.parse_args()
    if a.stage == "prep":
        prep()
    elif a.stage == "translate":
        translate()
    elif a.stage == "embed":
        for m in a.models:
            embed(m)
    else:
        score(a.models)
