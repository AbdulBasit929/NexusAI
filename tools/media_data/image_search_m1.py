#!/usr/bin/env python3
"""M1 image-search acceptance (docs/data/image-search-m1-preregistration-20261120.md, with amendment A4): fresh COCO set (2014 val, Karpathy 5K test), the pre-registered transformations, the product's own descriptors, the gates.

Runs inside the evaluation image (nexusai-media-eval-face:20261119) with:
    -v C:/tmp/nexusai-media-data:/data:ro -v C:/tmp/nexusai-media-data/work-faces:/work -v <repo>:/repo:ro -v <repo>/models/media:/models/media:ro
    python /repo/tools/media_data/image_search_m1.py select|transform|embed-nd|embed-sem|score-nd|score-sem

Every descriptor comes from ingestion/forensic_records/image_search.py (load_reduced_image, pdq_bits, VisionEncoder, SigLIPTextEncoder): the code that ships is the code that is measured.
Sealed numbers are written once (work/m1/results/sealed-<gate>.json); a second read needs --reread and a stated reason, which is recorded.
"""
import argparse
import gc
import hashlib
import io
import json
import os
import sys
import time
import zipfile
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw, ImageEnhance, ImageFont

sys.path.insert(0, "/repo")
DATA = Path(os.environ.get("NEXUSAI_MEDIA_DATA", "/data"))
WORK = Path(os.environ.get("NEXUSAI_FACE_WORK", "/work")) / "m1"
COCO = DATA / "datasets" / "image" / "coco2014-5k"
MODELS = DATA / "models" / "image"
SIGLIP1 = os.environ.get("SIGLIP_PATH", "/models/media/nexusai/models/google-siglip-base-patch16-224/7fd15f0689c79d79e38b1c2e2e2370a7bf2761ed")
DINO_SHA = "d73036b56966966d07975d696bde331762f37297e2f095de8cea0040c3aa0841"
SIGLIP2_SHA = "612923381c76ec5a9bed335d1c48827e3f2e506ac31b044b63b2031fadee6a0b"

RESAVE = ["jpeg75", "wa_like", "wa_twice", "resize50", "png_roundtrip"]
EDIT = ["crop85", "mirror", "rot5", "tone", "caption_bar", "social_frame", "screenshot_tight", "crop60_wa"]
REPORT_ONLY = ["screenshot_ui"]
ALL_TRANSFORMS = RESAVE + EDIT + REPORT_ONLY


def sha(text):
    return hashlib.sha256(text.encode()).hexdigest()


def partition(image_id):
    return "dev" if sha(str(image_id))[-1] in "0123456789ab" else "sealed"


def in_gallery(image_id):
    return int(sha("g" + str(image_id)), 16) % 3 != 0


# ----------------------------------------------------------------------------------------------------------------------------------------- select


def select():
    import csv
    WORK.mkdir(parents=True, exist_ok=True)
    images, caps = {}, {}
    with open(COCO / "test_5k_mscoco_2014.csv", encoding="utf-8", newline="") as handle:
        for row in csv.DictReader(handle):
            i = int(row["cocoid"])
            images[i] = row["filename"]
            caps[i] = [c.strip() for c in json.loads(row["raw"])]
    order = sorted(images, key=lambda i: sha("m1nd" + str(i)))
    nd600, d1000 = order[:600], order[600:1600]
    used = set(nd600) | set(d1000)
    sem_pool = sorted((i for i in images if i not in used and len(caps.get(i, [])) >= 5), key=lambda i: sha("m1sem" + str(i)))
    sem1000 = sem_pool[:1000]
    (WORK / "src").mkdir(exist_ok=True)
    with zipfile.ZipFile(COCO / "images_mscoco_2014_5k_test.zip") as z:
        members = {Path(n).name: n for n in z.namelist() if n.lower().endswith(".jpg")}
        for i in nd600 + d1000 + sem1000:
            target = WORK / "src" / f"{i:012d}.jpg"
            if not target.exists():
                target.write_bytes(z.read(members[images[i]]))
    manifest = {
        "source": "COCO 2014 validation, Karpathy 5K test split, as published by nlphuji on Hugging Face (nlphuji/mscoco_2014_5k_test_image_text_retrieval); photographs from Flickr under Creative Commons licences "
                  "(no per-photo licence id in this copy), captions CC BY 4.0; amendment A4 of the pre-registration",
        "nd600": [{"id": i, "file": f"src/{i:012d}.jpg", "original_name": images[i], "in_gallery": in_gallery(i), "partition": partition(i)} for i in nd600],
        "d1000": [{"id": i, "file": f"src/{i:012d}.jpg", "original_name": images[i], "partition": partition(i)} for i in d1000],
        "sem1000": [{"id": i, "file": f"src/{i:012d}.jpg", "original_name": images[i], "partition": partition(i), "captions": caps[i][:5]} for i in sem1000],
    }
    json.dump(manifest, open(WORK / "manifest.json", "w"), indent=1)
    print(f"nd600 {len(nd600)} (in gallery {sum(in_gallery(i) for i in nd600)}), d1000 {len(d1000)}, sem1000 {len(sem1000)}; partitions nd600 dev/sealed "
          f"{sum(partition(i) == 'dev' for i in nd600)}/{sum(partition(i) == 'sealed' for i in nd600)}; sem1000 sealed {sum(partition(i) == 'sealed' for i in sem1000)}")


# ----------------------------------------------------------------------------------------------------------------------------------------- transformations


def _jpeg_bytes(image, quality):
    buffer = io.BytesIO()
    image.save(buffer, format="JPEG", quality=quality)
    return buffer.getvalue()


def _wa_like(image, scale=0.8, quality=65):
    w, h = image.size
    small = image.resize((max(8, round(w * scale)), max(8, round(h * scale))), Image.LANCZOS)
    return _jpeg_bytes(small, quality)


def _decode(data):
    return Image.open(io.BytesIO(data)).convert("RGB")


def _centre(image, fraction):
    w, h = image.size
    cw, ch = round(w * fraction), round(h * fraction)
    left, top = (w - cw) // 2, (h - ch) // 2
    return image.crop((left, top, left + cw, top + ch))


def _font(size):
    try:
        return ImageFont.load_default(size=size)
    except TypeError:
        return ImageFont.load_default()


def _png(img):
    buffer = io.BytesIO()
    img.save(buffer, format="PNG")
    return buffer.getvalue()


def make_resave(image):
    """The five family R copies (re-save, messenger-style recompression twice, resize, lossless)."""
    w, h = image.size
    out = {"jpeg75": (_jpeg_bytes(image, 75), "jpg")}
    wa = _wa_like(image)
    out["wa_like"] = (wa, "jpg")
    out["wa_twice"] = (_wa_like(_decode(wa), 0.9, 60), "jpg")
    out["resize50"] = (_jpeg_bytes(image.resize((max(8, w // 2), max(8, h // 2)), Image.LANCZOS), 90), "jpg")
    out["png_roundtrip"] = (_png(image), "png")
    return out


def make_transforms(image):
    """name -> (bytes, extension). Deterministic; the pre-registered list of section 3."""
    w, h = image.size
    png = _png
    out = make_resave(image)
    out["crop85"] = (png(_centre(image, 0.85)), "png")
    out["mirror"] = (png(image.transpose(Image.FLIP_LEFT_RIGHT)), "png")
    out["rot5"] = (png(image.rotate(5, resample=Image.BICUBIC, expand=False, fillcolor=(0, 0, 0))), "png")
    out["tone"] = (png(ImageEnhance.Contrast(ImageEnhance.Brightness(image).enhance(1.25)).enhance(0.8)), "png")
    bar = image.copy()
    draw = ImageDraw.Draw(bar)
    draw.rectangle((0, h - h // 8, w, h), fill=(0, 0, 0))
    draw.text((w // 20, h - h // 9), "EVIDENCE COPY - NOT FOR RELEASE", fill=(255, 255, 255), font=_font(max(10, h // 20)))
    out["caption_bar"] = (png(bar), "png")
    border = round(0.08 * min(w, h))
    strip = round(0.10 * h)
    framed = Image.new("RGB", (w + 2 * border, h + 2 * border + strip), (255, 255, 255))
    framed.paste(image, (border, border))
    ImageDraw.Draw(framed).text((border, h + border + strip // 4), "shared via chat", fill=(110, 110, 110), font=_font(max(10, strip // 2)))
    out["social_frame"] = (_wa_like(framed), "jpg")
    shot = image.resize((1080, max(8, round(h * 1080 / w))), Image.LANCZOS)
    cut_x, cut_y = round(0.02 * shot.size[0]), round(0.02 * shot.size[1])
    out["screenshot_tight"] = (_jpeg_bytes(shot.crop((cut_x, cut_y, shot.size[0] - cut_x, shot.size[1] - cut_y)), 80), "jpg")
    out["crop60_wa"] = (_wa_like(_centre(image, 0.6)), "jpg")
    canvas = Image.new("RGB", (1080, 2340), (236, 236, 240))
    d = ImageDraw.Draw(canvas)
    d.rectangle((0, 0, 1080, 90), fill=(20, 20, 24))
    d.text((40, 24), "9:41", fill=(255, 255, 255), font=_font(40))
    for k in range(3):
        d.rectangle((820 + k * 70, 30, 870 + k * 70, 60), fill=(255, 255, 255))
    d.rectangle((0, 90, 1080, 240), fill=(255, 255, 255))
    d.text((60, 140), "Photo", fill=(30, 30, 30), font=_font(48))
    pic = image.resize((1000, max(8, round(h * 1000 / w))), Image.LANCZOS)
    d.rounded_rectangle((20, 300 - 20, 1060, 300 + pic.size[1] + 20), radius=36, fill=(255, 255, 255))
    canvas.paste(pic, (40, 300))
    d.rectangle((0, 2180, 1080, 2340), fill=(255, 255, 255))
    d.rounded_rectangle((40, 2210, 1040, 2310), radius=50, fill=(235, 235, 240))
    out["screenshot_ui"] = (png(canvas), "png")
    return out


def transform():
    manifest = json.load(open(WORK / "manifest.json"))
    (WORK / "q").mkdir(exist_ok=True)
    rows = []
    started = time.time()
    for n, item in enumerate(manifest["nd600"], start=1):
        image = Image.open(WORK / item["file"]).convert("RGB")
        for name, (data, ext) in make_transforms(image).items():
            path = WORK / "q" / f"{item['id']:012d}__{name}.{ext}"
            if not path.exists():
                path.write_bytes(data)
            rows.append({"id": item["id"], "transform": name, "file": f"q/{path.name}"})
        if n % 100 == 0:
            print(f"  {n}/600 photos, {time.time() - started:.0f} s", flush=True)
    json.dump(rows, open(WORK / "queries.json", "w"))
    print(f"{len(rows)} query files")


# ----------------------------------------------------------------------------------------------------------------------------------------- descriptors with the product code


def _imports():
    from ingestion.forensic_records import image_search as s
    return s


def _retry(call, attempts=8):
    """The Docker VM on this laptop runs close to its memory limit (the language model and other containers share it): a read or an encode can fail with ENOMEM. Wait and try again; any other error is raised."""
    for attempt in range(1, attempts + 1):
        try:
            return call()
        except Exception as exc:  # noqa: BLE001
            if "Cannot allocate memory" not in str(exc) and not isinstance(exc, MemoryError):
                raise
            if attempt == attempts:
                raise
            print(f"  memory pressure ({exc}); waiting {10 * attempt} s", flush=True)
            time.sleep(10 * attempt)


def embed_nd(batch=100):
    s = _imports()
    threads = int(os.environ.get("IMG_THREADS", "4"))
    manifest = json.load(open(WORK / "manifest.json"))
    queries = json.load(open(WORK / "queries.json"))
    items = [(f"orig:{m['id']}", m["file"]) for m in manifest["nd600"]] + [(f"dist:{m['id']}", m["file"]) for m in manifest["d1000"]] +             [(f"q:{q['id']}:{q['transform']}", q["file"]) for q in queries]
    out = WORK / "emb"
    out.mkdir(exist_ok=True)
    done = set()
    for shard in sorted(out.glob("nd-*.npz")):
        done.update(str(k) for k in np.load(shard)["keys"])
    todo = [item for item in items if item[0] not in done]
    print(f"{len(done)} items already embedded, {len(todo)} to do", flush=True)
    dino = s.VisionEncoder("dinov2", MODELS / "dinov2-base", model_id="facebook/dinov2-base", model_revision="", model_sha256=DINO_SHA, cpu_threads=threads)
    started = time.time()
    for start in range(0, len(todo), batch):
        keys, bits, vecs, t_load, t_dino = [], [], [], [], []
        for key, rel in todo[start:start + batch]:
            t0 = time.perf_counter()
            image = _retry(lambda: s.load_reduced_image(WORK / rel))
            b = s.pdq_bits(image)
            t1 = time.perf_counter()
            v = _retry(lambda: dino.encode(image))
            t2 = time.perf_counter()
            keys.append(key)
            bits.append(b)
            vecs.append(np.asarray(v, dtype=np.float32))
            t_load.append(t1 - t0)
            t_dino.append(t2 - t1)
        np.savez(out / f"nd-{len(done) + start:05d}.npz", keys=np.array(keys), bits=np.stack(bits), vecs=np.stack(vecs), t_load=np.array(t_load), t_dino=np.array(t_dino))
        print(f"  shard done, {len(done) + start + len(keys)}/{len(items)} items, {time.time() - started:.0f} s", flush=True)
    print("embed-nd complete")


def embed_sem():
    s = _imports()
    import torch
    from transformers import AutoImageProcessor, AutoModel, AutoTokenizer
    threads = int(os.environ.get("IMG_THREADS", "4"))
    torch.set_num_threads(threads)
    manifest = json.load(open(WORK / "manifest.json"))
    sem = manifest["sem1000"]
    out = WORK / "emb"
    out.mkdir(exist_ok=True)
    target = out / "sem-siglip2.npz"
    if not target.exists():
        enc = s.VisionEncoder("siglip2", MODELS / "siglip2-base-patch16-224", model_id="google/siglip2-base-patch16-224", model_revision="", model_sha256=SIGLIP2_SHA, cpu_threads=threads)
        text = s.SigLIPTextEncoder(MODELS / "siglip2-base-patch16-224", model_sha256=SIGLIP2_SHA, cpu_threads=threads)
        images, t_img = [], []
        for n, m in enumerate(sem, start=1):
            t0 = time.perf_counter()
            images.append(np.asarray(enc.encode(s.load_reduced_image(WORK / m["file"])), dtype=np.float32))
            t_img.append(time.perf_counter() - t0)
            if n % 200 == 0:
                print(f"  siglip2 images {n}/{len(sem)}", flush=True)
        enc.release()  # one model in memory at a time (the container shares a 7.6 GB VM with the language model)
        gc.collect()
        caps = [c for m in sem for c in m["captions"]]
        vectors = []
        for i in range(0, len(caps), 32):
            vectors.extend(text.encode(caps[i:i + 32]))
        text.release()
        np.savez(target, images=np.stack(images), texts=np.asarray(vectors, dtype=np.float32), t_img=np.array(t_img))
        print("siglip2 done")
        del enc, text
        gc.collect()
    target1 = out / "sem-siglip1.npz"
    if not target1.exists():
        # the model the product carries today, loaded the way the bake-off loaded it
        proc, tok, model = AutoImageProcessor.from_pretrained(SIGLIP1), AutoTokenizer.from_pretrained(SIGLIP1), AutoModel.from_pretrained(SIGLIP1).eval()
        images = []
        for n, m in enumerate(sem, start=1):
            image = s.load_reduced_image(WORK / m["file"])
            with torch.no_grad():
                f = model.get_image_features(**proc(images=image, return_tensors="pt"))
            images.append((f / f.norm(dim=-1, keepdim=True))[0].numpy())
            if n % 200 == 0:
                print(f"  siglip1 images {n}/{len(sem)}", flush=True)
        caps = [c for m in sem for c in m["captions"]]
        vectors = []
        for i in range(0, len(caps), 32):
            enc_t = tok([c.lower() for c in caps[i:i + 32]], padding="max_length", max_length=64, truncation=True, return_tensors="pt")
            with torch.no_grad():
                f = model.get_text_features(input_ids=enc_t["input_ids"])
            vectors.append((f / f.norm(dim=-1, keepdim=True)).numpy())
        np.savez(target1, images=np.stack(images), texts=np.concatenate(vectors))
        print("siglip1 done")


# ----------------------------------------------------------------------------------------------------------------------------------------- scoring


def load_nd():
    keys, bits, vecs, t_load, t_dino = [], [], [], [], []
    for shard in sorted((WORK / "emb").glob("nd-*.npz")):
        z = np.load(shard)
        keys += list(z["keys"])
        bits.append(z["bits"])
        vecs.append(z["vecs"])
        t_load.append(z["t_load"])
        t_dino.append(z["t_dino"])
    return {k: i for i, k in enumerate(keys)}, np.concatenate(bits), np.concatenate(vecs), np.concatenate(t_load), np.concatenate(t_dino)


def sealed_guard(gate, reread, reason):
    path = WORK / "results" / f"sealed-{gate}.json"
    path.parent.mkdir(exist_ok=True)
    if path.exists() and not reread:
        print(f"the sealed part of {gate} was already read ({path}); pass --reread --reason '...' to read again (recorded)")
        sys.exit(3)
    return path


def score_nd(reread=False, reason="", dev_only=False):
    index, bits, vecs, t_load, t_dino = load_nd()
    manifest = json.load(open(WORK / "manifest.json"))
    queries = json.load(open(WORK / "queries.json"))
    nd = manifest["nd600"]
    gallery_keys = [f"dist:{m['id']}" for m in manifest["d1000"]] + [f"orig:{m['id']}" for m in nd if m["in_gallery"]]
    gallery_ids = [None] * len(manifest["d1000"]) + [m["id"] for m in nd if m["in_gallery"]]
    gi = np.array([index[k] for k in gallery_keys])
    g_bits, g_vecs = bits[gi].astype(np.int16), vecs[gi]
    by_id = {m["id"]: m for m in nd}
    rows = []  # photo, transform, positive, d_hash, best_hash, d_dino, best_dino
    for q in queries:
        i = index[f"q:{q['id']}:{q['transform']}"]
        dh = (g_bits != bits[i].astype(np.int16)).sum(axis=1)
        dd = 1.0 - g_vecs @ vecs[i]
        bh, bd = int(np.argmin(dh)), int(np.argmin(dd))
        rows.append((q["id"], q["transform"], by_id[q["id"]]["in_gallery"], float(dh[bh]), gallery_ids[bh], float(dd[bd]), gallery_ids[bd]))

    def score(part, distance_col, best_col, threshold, families):
        tp = fp = positives = 0
        for r in rows:
            if partition(r[0]) != part or r[1] not in families:
                continue
            positives += 1 if r[2] else 0
            if r[distance_col] <= threshold:
                if r[2] and r[best_col] == r[0]:
                    tp += 1
                else:
                    fp += 1
        precision = tp / (tp + fp) if tp + fp else 1.0
        return precision, (tp / positives if positives else 0.0), tp, fp, positives

    def choose(distance_col, best_col, grid, families):
        chosen = None
        for t in grid:
            if score("dev", distance_col, best_col, t, families)[0] >= 0.995:
                chosen = t
        return chosen

    fam_a, fam_b = set(RESAVE), set(RESAVE) | set(EDIT)
    t_a = choose(3, 4, range(0, 257), fam_a)
    t_b = choose(5, 6, [round(x, 3) for x in np.arange(0.0, 0.9, 0.002)], fam_b)
    report = {"threshold_hash_bits": t_a, "threshold_dino_cosine_distance": t_b, "gallery_size": len(gallery_keys), "queries": len(rows)}
    for part in ("dev",):
        report[part] = {"tier_a_R": score(part, 3, 4, t_a, fam_a), "tier_b_R_E": score(part, 5, 6, t_b, fam_b)}
    # speed (T1 for the two descriptors measured here; the three-descriptor figure is measured by the speed command)
    report["seconds_per_image_load_and_hash_median"] = round(float(np.median(t_load)), 4)
    report["seconds_per_image_dino_median"] = round(float(np.median(t_dino)), 4)
    # N4 control
    zero_hash = zero_cos = total = 0
    for m in nd:
        a, b = index[f"orig:{m['id']}"], index[f"q:{m['id']}:png_roundtrip"]
        total += 1
        zero_hash += int((bits[a] != bits[b]).sum() == 0)
        zero_cos += int(1.0 - float(vecs[a] @ vecs[b]) <= 1e-6)
    report["png_roundtrip_control"] = {"photos": total, "hash_distance_zero": zero_hash, "cosine_distance_within_1e-6": zero_cos}
    if dev_only:  # a check of the script on the development part; the sealed part is not touched and no guard file is written
        for part in ("dev",):
            for name, fam, dcol, bcol, thr in (("tier_a_per_transform_dev", fam_a | set(EDIT), 3, 4, t_a), ("tier_b_per_transform_dev", fam_b, 5, 6, t_b)):
                report[name] = {t: dict(zip(("precision", "recall", "tp", "fp", "positives"), score("dev", dcol, bcol, thr, {t}))) for t in RESAVE + EDIT + REPORT_ONLY}
        print(json.dumps(report, indent=1, default=float))
        return
    path = sealed_guard("nd", reread, reason)
    sealed = {"reread_reason": reason or None, "read_at": time.strftime("%Y-%m-%d %H:%M:%S")}
    sealed["tier_a_R"] = dict(zip(("precision", "recall", "tp", "fp", "positives"), score("sealed", 3, 4, t_a, fam_a)))
    sealed["tier_b_R_E"] = dict(zip(("precision", "recall", "tp", "fp", "positives"), score("sealed", 5, 6, t_b, fam_b)))
    sealed["tier_b_per_transform"] = {t: dict(zip(("precision", "recall", "tp", "fp", "positives"), score("sealed", 5, 6, t_b, {t}))) for t in RESAVE + EDIT + REPORT_ONLY}
    sealed["tier_a_per_transform"] = {t: dict(zip(("precision", "recall", "tp", "fp", "positives"), score("sealed", 3, 4, t_a, {t}))) for t in RESAVE + EDIT + REPORT_ONLY}
    # SUPPLEMENTARY and post hoc (not a gate, not used to choose anything): the sealed precision/recall at other distances, so that a later question about another operating point needs no second read of the sealed part
    cols = ("precision", "recall", "tp", "fp", "positives")
    sealed["supplementary_curve_tier_b_R_E"] = {str(t): dict(zip(cols, score("sealed", 5, 6, t, fam_b))) for t in [round(x, 2) for x in np.arange(0.04, 0.321, 0.02)]}
    sealed["supplementary_curve_tier_a_R"] = {str(t): dict(zip(cols, score("sealed", 3, 4, t, fam_a))) for t in (0, 10, 20, 30, 40, 50, 60, 70, 80, 91, 100, 110, 120)}
    # combined: label A when the hash is within its distance, else B when the vector is within its distance
    for part in ("dev", "sealed"):
        tp = fp = positives = 0
        for r in rows:
            if partition(r[0]) != part or r[1] not in fam_b:
                continue
            positives += 1 if r[2] else 0
            if r[3] <= t_a:
                answer = r[4]
            elif r[5] <= t_b:
                answer = r[6]
            else:
                continue
            if r[2] and answer == r[0]:
                tp += 1
            else:
                fp += 1
        (sealed if part == "sealed" else report[part]).__setitem__("combined", {"precision": tp / (tp + fp) if tp + fp else 1.0, "recall": tp / positives if positives else 0.0, "tp": tp, "fp": fp, "positives": positives})
    json.dump(sealed, open(path, "w"), indent=1)
    report["sealed"] = sealed
    json.dump(report, open(WORK / "results" / "m1-nd-score.json", "w"), indent=1, default=float)
    print(json.dumps(report, indent=1, default=float))


def recall_at(sims, owner, ks=(1, 5, 10)):
    order = np.argsort(-sims, axis=1)
    rank = np.array([int(np.where(order[i] == owner[i])[0][0]) for i in range(len(owner))])
    return rank, {f"R@{k}": round(100 * float(np.mean(rank < k)), 2) for k in ks}


def score_sem(reread=False, reason=""):
    manifest = json.load(open(WORK / "manifest.json"))
    sem = manifest["sem1000"]
    path = sealed_guard("sem", reread, reason)
    report = {"read_at": time.strftime("%Y-%m-%d %H:%M:%S"), "reread_reason": reason or None}
    rng = np.random.default_rng(20261120)
    for name in ("siglip2", "siglip1"):
        if not (WORK / "emb" / f"sem-{name}.npz").exists():
            report[name] = "not run (the like-for-like comparison with the older model was dropped on 2026-11-23 on the owner's direction to stop further testing: gate S2 not measured)"
            continue
        z = np.load(WORK / "emb" / f"sem-{name}.npz")
        images, texts = z["images"], z["texts"]
        owner_all = np.repeat(np.arange(len(sem)), 5)
        caption_part = np.array([partition(sem[o]["id"]) for o in owner_all])
        entry = {}
        for part in ("dev", "sealed"):
            sel = caption_part == part
            rank, rec = recall_at(texts[sel] @ images.T, owner_all[sel])
            photos = np.unique(owner_all[sel])
            # bootstrap over photos for R@1 and R@10
            by_photo = {p: rank[owner_all[sel] == p] for p in photos}
            boot = {1: [], 10: []}
            for _ in range(1000):
                pick = rng.choice(photos, size=len(photos), replace=True)
                allrank = np.concatenate([by_photo[p] for p in pick])
                boot[1].append(100 * float(np.mean(allrank < 1)))
                boot[10].append(100 * float(np.mean(allrank < 10)))
            entry[part] = {"captions": int(sel.sum()), "photos": int(len(photos)), **rec,
                           "R@1_95ci": [round(float(np.percentile(boot[1], 2.5)), 2), round(float(np.percentile(boot[1], 97.5)), 2)],
                           "R@10_95ci": [round(float(np.percentile(boot[10], 2.5)), 2), round(float(np.percentile(boot[10], 97.5)), 2)]}
        report[name] = entry
    json.dump(report, open(path, "w"), indent=1)
    json.dump(report, open(WORK / "results" / "m1-sem-score.json", "w"), indent=1)
    print(json.dumps(report, indent=1))


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("stage", choices=("select", "transform", "embed-nd", "embed-sem", "score-nd", "score-sem"))
    parser.add_argument("--reread", action="store_true")
    parser.add_argument("--dev-only", action="store_true")
    parser.add_argument("--reason", default="")
    a = parser.parse_args()
    if a.stage == "score-nd":
        score_nd(a.reread, a.reason, dev_only=a.dev_only)
    elif a.stage == "score-sem":
        score_sem(a.reread, a.reason)
    else:
        {"select": select, "transform": transform, "embed-nd": embed_nd, "embed-sem": embed_sem}[a.stage]()
