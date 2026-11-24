#!/usr/bin/env python3
"""M1 gates S0 (integrity of the product descriptors against the bake-off tools) and T1 (ingest speed and memory), docs/data/image-search-m1-preregistration-20261120.md.

    docker run --rm --network none --memory 3g -v C:/tmp/nexusai-media-data:/data:ro -v C:/tmp/nexusai-media-data/work-faces:/work -v <repo>:/repo:ro \
        -e FORENSIC_IMAGE_SEARCH_DINO_MODEL_PATH=/data/models/image/dinov2-base -e FORENSIC_IMAGE_SEARCH_DINO_MODEL_SHA256=<sha> \
        -e FORENSIC_IMAGE_SEARCH_SIGLIP_MODEL_PATH=/data/models/image/siglip2-base-patch16-224 -e FORENSIC_IMAGE_SEARCH_SIGLIP_MODEL_SHA256=<sha> \
        --entrypoint python nexusai-media-eval-face:20261119 /repo/tools/media_data/image_search_s0_t1.py s0|t1

S0: 150 Flickr 1K photos (the ones with the assistant-translated Urdu captions). Product = ImageSearchProcessor built from the environment. Bake-off = image_eval.py (PDQ, DINOv2 base through its Embedder),
the SigLIP 2 image vectors and English caption vectors already saved by image_text_retrieval.py. T1: the first 300 photos of the same set through the whole processor, one at a time, 4 threads.
"""
import gc
import json
import os
import resource
import sys
import time
from pathlib import Path

import numpy as np
from PIL import Image

sys.path.insert(0, "/repo")
sys.path.insert(0, "/repo/tools/media_data")
WORK = Path("/work") / "flickr1k"
IMAGES = WORK / "images" / "images_flickr_1k_test"


def rows():
    return json.load(open(WORK / "captions-en.json"))


def processor():
    from ingestion.forensic_records.image_search import ImageSearchProcessor
    os.environ.setdefault("FORENSIC_IMAGE_SEARCH_CPU_THREADS", "4")
    return ImageSearchProcessor.from_environment()


def s0():
    from ingestion.forensic_records import image_search as s
    import image_eval
    names = [r["filename"] for r in json.load(open(WORK / "captions-ur-claude.json", encoding="utf-8"))]
    captions = rows()
    order = {r["filename"]: i for i, r in enumerate(captions)}
    saved_images = np.load(WORK / "emb-siglip2-images.npy")
    saved_text = np.load(WORK / "emb-siglip2-en.npy")
    proc = processor()
    os.environ.setdefault("IMG_THREADS", "4")

    def reduced(name):
        image = Image.open(IMAGES / name).convert("RGB")
        if max(image.size) > 1024:
            image.thumbnail((1024, 1024))
        return image

    # pass A: the hash and DINOv2 base, product against the bake-off tool (one model in memory at a time: a 3 GB container cannot hold the four models together)
    bake = image_eval.Embedder("dinov2b")
    cos_dino, hash_equal = [], 0
    for n, name in enumerate(names, start=1):
        image = s.load_reduced_image(IMAGES / name)
        hash_equal += int(s.bits_to_hex(s.pdq_bits(image)) == s.bits_to_hex(image_eval.pdq_bits(reduced(name))))
        cos_dino.append(float(np.dot(proc.dino.encode(image), bake(reduced(name)))))
        if n % 50 == 0:
            print(f"  pass A {n}/{len(names)}", flush=True)
    proc.dino.release()
    del bake
    gc.collect()
    # pass B: SigLIP 2 vision tower against the vectors the bake-off saved from the full model
    cos_sig = []
    for n, name in enumerate(names, start=1):
        cos_sig.append(float(np.dot(proc.siglip.encode(s.load_reduced_image(IMAGES / name)), saved_images[order[name]])))
        if n % 50 == 0:
            print(f"  pass B {n}/{len(names)}", flush=True)
    proc.siglip.release()
    gc.collect()
    # pass C: the SigLIP 2 text tower against the caption vectors the bake-off saved
    text = s.SigLIPTextEncoder(Path(os.environ["FORENSIC_IMAGE_SEARCH_SIGLIP_MODEL_PATH"]), model_sha256=os.environ["FORENSIC_IMAGE_SEARCH_SIGLIP_MODEL_SHA256"], cpu_threads=4)
    cos_text = [float(np.dot(text.encode([captions[order[name]]["captions"][0]])[0], saved_text[5 * order[name]])) for name in names]
    out = {"photos": len(names),
           "dino_cosine_min": min(cos_dino), "dino_share_ge_0.999": float(np.mean(np.array(cos_dino) >= 0.999)),
           "siglip2_image_cosine_min": min(cos_sig), "siglip2_image_share_ge_0.999": float(np.mean(np.array(cos_sig) >= 0.999)),
           "siglip2_text_cosine_min": min(cos_text), "siglip2_text_share_ge_0.999": float(np.mean(np.array(cos_text) >= 0.999)),
           "pdq_equal_share": hash_equal / len(names)}
    json.dump(out, open(Path("/work") / "m1" / "results" / "s0.json", "w"), indent=1)
    print(json.dumps(out, indent=1))


def t1():
    baseline_mb = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024
    proc = processor()
    names = [r["filename"] for r in rows()[:300]]
    proc.process_image(IMAGES / names[0], evidence_id="e", version_id="v", source_sha256="0" * 64, source_file=names[0])  # warm-up: loads the models, not timed
    times = []
    for name in names:
        t0 = time.perf_counter()
        proc.process_image(IMAGES / name, evidence_id="e", version_id="v", source_sha256="0" * 64, source_file=name)
        times.append(time.perf_counter() - t0)
    peak_mb = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024
    out = {"images": len(names), "median_s": float(np.median(times)), "p95_s": float(np.percentile(times, 95)), "mean_s": float(np.mean(times)),
           "baseline_rss_mb": round(baseline_mb), "peak_rss_mb": round(peak_mb), "above_baseline_mb": round(peak_mb - baseline_mb), "threads": 4}
    json.dump(out, open(Path("/work") / "m1" / "results" / "t1.json", "w"), indent=1)
    print(json.dumps(out, indent=1))


if __name__ == "__main__":
    (Path("/work") / "m1" / "results").mkdir(parents=True, exist_ok=True)
    {"s0": s0, "t1": t1}[sys.argv[1]]()
