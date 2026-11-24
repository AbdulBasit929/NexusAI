#!/usr/bin/env python3
"""M1 gate U0, the Urdu route as a DEVELOPMENT PROXY (docs/data/image-search-m1-preregistration-20261120.md section 5): Urdu caption -> the resident language model translates it to English -> SigLIP 2 text tower -> SigLIP 2 image vectors.

The 150 Urdu captions of the bake-off were translated from English by the assistant and are NOT reviewed by a native speaker, so this proxy is optimistic (a straightforward translation of an English caption is easy to translate back) and does not replace gate U1.

    stage 1, on the host (stdlib only; calls the local language model):   python tools/media_data/image_search_u0.py translate
    stage 2, in the evaluation image (SigLIP 2 text tower, no network):  python /repo/tools/media_data/image_search_u0.py score
"""
import json
import os
import sys
import time
from pathlib import Path

WORK_HOST = Path(os.environ.get("NEXUSAI_MEDIA_DATA_HOST", "C:/tmp/nexusai-media-data")) / "work-faces"
PROMPT = "You translate a short search phrase about photographs into plain English. Reply with the English phrase only: no quotes, no notes, no explanation."
MODEL = os.environ.get("FORENSIC_SYNTHESIS_MODEL", "qwen3-4b-instruct-2507-q4km-nxb21d-dev")


def clean(raw):
    for line in raw.split("\n"):
        line = line.strip().strip("\"'`\u201c\u201d").strip()
        if line:
            return line[:512]
    return ""


def translate():
    import urllib.request
    rows = json.load(open(WORK_HOST / "flickr1k" / "captions-ur-claude.json", encoding="utf-8"))
    out, started = [], time.time()
    for n, row in enumerate(rows, start=1):
        body = json.dumps({"model": MODEL, "temperature": 0, "max_tokens": 80,
                           "messages": [{"role": "system", "content": PROMPT}, {"role": "user", "content": row["ur"]}]}).encode("utf-8")
        request = urllib.request.Request("http://127.0.0.1:8080/v1/chat/completions", data=body, headers={"Content-Type": "application/json"})
        with urllib.request.urlopen(request, timeout=180) as response:
            text = json.loads(response.read().decode("utf-8"))["choices"][0]["message"]["content"]
        out.append({"filename": row["filename"], "ur": row["ur"], "en_from_llm": clean(text)})
        if n % 25 == 0:
            print(f"  translated {n}/{len(rows)} in {time.time() - started:.0f} s", flush=True)
    target = WORK_HOST / "m1" / "results"
    target.mkdir(parents=True, exist_ok=True)
    json.dump({"model": MODEL, "prompt": PROMPT, "rows": out}, open(target / "u0-translations.json", "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    print(f"{len(out)} translations written")


def score():
    import hashlib
    import numpy as np
    sys.path.insert(0, "/repo")
    from ingestion.forensic_records import image_search as s
    work = Path("/work") / "flickr1k"
    translations = json.load(open(Path("/work") / "m1" / "results" / "u0-translations.json", encoding="utf-8"))["rows"]
    captions = json.load(open(work / "captions-en.json"))
    names = [c["filename"] for c in captions]
    index = {n: i for i, n in enumerate(names)}
    images = np.load(work / "emb-siglip2-images.npy")
    direct_ur = np.load(work / "emb-siglip2-ur.npy")  # the Urdu text sent straight to SigLIP 2 (the bake-off's failing route)
    en_vectors = np.load(work / "emb-siglip2-en.npy")
    sealed = [i for i, n in enumerate(names) if hashlib.sha256(n.encode()).hexdigest()[-1] not in "0123"]
    pos = {i: k for k, i in enumerate(sealed)}
    gallery = images[sealed]
    text = s.SigLIPTextEncoder(Path(os.environ["FORENSIC_IMAGE_SEARCH_SIGLIP_MODEL_PATH"]), model_sha256=os.environ["FORENSIC_IMAGE_SEARCH_SIGLIP_MODEL_SHA256"], cpu_threads=4)
    owner = [pos[index[r["filename"]]] for r in translations]
    translated = np.asarray([text.encode([r["en_from_llm"]])[0] for r in translations], dtype=np.float32)

    def recall(vectors):
        sims = vectors @ gallery.T
        order = np.argsort(-sims, axis=1)
        rank = np.array([int(np.where(order[i] == owner[i])[0][0]) for i in range(len(owner))])
        return {f"R@{k}": round(100 * float(np.mean(rank < k)), 2) for k in (1, 5, 10)}

    english_control = en_vectors[[5 * index[r["filename"]] for r in translations]]
    direct = direct_ur[[k for k in range(len(translations))]]
    result = {"queries": len(translations), "gallery_images": len(sealed),
              "english_first_caption": recall(english_control), "urdu_via_llm_translation": recall(translated), "urdu_direct_to_siglip2": recall(direct)}
    result["ratio_R@10_translated_to_english_percent"] = round(100 * result["urdu_via_llm_translation"]["R@10"] / result["english_first_caption"]["R@10"], 1)
    result["note"] = "development proxy: Urdu captions assistant-translated from English and not reviewed; gate U1 (native-reviewed captions, ratio >= 80 percent) stays NOT MEASURED"
    json.dump(result, open(Path("/work") / "m1" / "results" / "u0.json", "w"), indent=1)
    print(json.dumps(result, indent=1))


if __name__ == "__main__":
    {"translate": translate, "score": score}[sys.argv[1]]()
