#!/usr/bin/env python3
"""Fetch the published plate models the plates programme evaluates before it trains anything of its own (docs/data/plates-m5-preregistration-20261110.md, Amendment 2).

    python -X utf8 tools/media_data/plate_fetch.py --list
    python -X utf8 tools/media_data/plate_fetch.py plate_ocr_pretrained plate_rfdetr

Reuses fetch.py (resumable, refuses a short read, manifest.jsonl with SHA-256, URL, licence and ledger id). Files are pinned: Hugging Face files to a commit, GitHub release files to their tag.
These are candidates for EVALUATION on synthetic and public data and on the owner-labelled real set; adopting one for the product needs its ledger row closed first.
"""
import argparse
import os
import sys

import fetch  # tools/media_data/fetch.py

HF = "https://huggingface.co"
GH = "https://github.com/ankandrew/cnn-ocr-lp/releases/download/arg-plates"
RFDETR_REV = "851dc6b2db27"  # autolane/rfdetr-alpr, read 2026-11-10 (card: apache-2.0; training data not stated)

OCR_FILES = ("cct_s_v2_global.onnx", "cct_s_v2_global_plate_config.yaml", "global_mobile_vit_v2_ocr.onnx", "global_mobile_vit_v2_ocr_config.yaml")
GROUPS = {
    "plate_ocr_pretrained": [
        dict(url=f"{GH}/{name}", rel=f"models/plates/pretrained-ocr/{name}", licence="MIT (ankandrew projects: code; the project says all its repos are MIT; a separate statement for the weights was not found)", ledger="MOD-007",
             note="fast-plate-ocr published global plate readers (trained on plates from 65+ countries; training data not published); GitHub release arg-plates of ankandrew/cnn-ocr-lp")
        for name in OCR_FILES
    ],
    "plate_rfdetr": [
        dict(url=f"{HF}/autolane/rfdetr-alpr/resolve/{RFDETR_REV}/rfdetr_alpr.onnx", rel="models/plates/rfdetr-alpr/rfdetr_alpr.onnx", licence="Apache-2.0 (card front matter; same as RF-DETR)", ledger="MOD-041",
             note="RF-DETR Medium fine-tuned for one class, 576x576, 117 MB; training data and metrics not stated on the card"),
        dict(url=f"{HF}/autolane/rfdetr-alpr/resolve/{RFDETR_REV}/README.md", rel="models/plates/rfdetr-alpr/README.md", licence="Apache-2.0 (card)", ledger="MOD-041", note="model card"),
    ],
}
fetch.GROUPS.update(GROUPS)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("groups", nargs="*")
    parser.add_argument("--list", action="store_true")
    args = parser.parse_args()
    if args.list or not args.groups:
        for name, items in GROUPS.items():
            print(f"{name}: {len(items)} files")
            for item in items:
                print(f"   {item['rel']:62} {item['licence'][:48]:48} {item['ledger']}")
        return
    os.makedirs(fetch.ROOT, exist_ok=True)
    for name in args.groups:
        if name not in GROUPS:
            sys.exit(f"unknown group {name}; known: {', '.join(GROUPS)}")
        fetch.fetch_group(name)


if __name__ == "__main__":
    main()
