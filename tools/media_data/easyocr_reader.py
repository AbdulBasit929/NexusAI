"""EasyOCR as a reader for the OCR bake-off v2 (docs/data/ocr-bakeoff-v2-preregistration-20261117.md): one Reader, one line-assembly rule, the same for every set.

Reader: Reader(languages, gpu=False, model_storage_directory=<data>/models/ocr/easyocr/model, download_enabled=False), readtext(image, paragraph=False), detector and recogniser parameters at their defaults.
Line assembly (amendment A1: grouped by vertical overlap): boxes are grouped into lines by vertical position (a box joins the line when its vertical extent overlaps the line's by at least 40% of the smaller height);
lines are ordered top to bottom; inside a line the boxes are ordered right to left when more than half of the line's letters are Arabic-script, left to right otherwise; boxes are joined by one space and
lines by a line break. No other post-processing.
"""
import os
import re
import statistics

ARABIC = re.compile("[؀-ۿݐ-ݿﭐ-﷿ﹰ-﻿]")
LETTER = re.compile("[^\\W\\d_]", re.UNICODE)
OVERLAP = 0.4      # amendment A1 of the bake-off plan: a box joins a line when their vertical extents overlap by at least this share of the smaller height


def make_reader(languages, data_root=None):
    import torch
    import easyocr
    torch.set_num_threads(int(os.environ.get("EASYOCR_THREADS", "1")))
    root = os.path.join(data_root or os.environ.get("NEXUSAI_MEDIA_DATA", "/data"), "models/ocr/easyocr/model")
    return easyocr.Reader(list(languages), gpu=False, model_storage_directory=root, user_network_directory=root, download_enabled=False, verbose=False)


def assemble(results):
    """results: EasyOCR's [(box, text, confidence), ...] -> (text, mean confidence)."""
    items = []
    for box, text, confidence in results:
        ys = [p[1] for p in box]
        xs = [p[0] for p in box]
        items.append({"cy": (min(ys) + max(ys)) / 2.0, "cx": (min(xs) + max(xs)) / 2.0, "h": max(ys) - min(ys), "text": text, "conf": confidence})
    if not items:
        return "", None
    lines = []
    for item in sorted(items, key=lambda i: i["cy"]):
        item["top"], item["bottom"] = item["cy"] - item["h"] / 2.0, item["cy"] + item["h"] / 2.0
        if lines:
            members = lines[-1]
            top, bottom = statistics.mean(m["top"] for m in members), statistics.mean(m["bottom"] for m in members)
            overlap = min(item["bottom"], bottom) - max(item["top"], top)
            if overlap >= OVERLAP * min(item["h"], bottom - top):
                members.append(item)
                continue
        lines.append([item])
    out = []
    for line in lines:
        letters = "".join(i["text"] for i in line)
        arabic = len(ARABIC.findall(letters))
        total = len(LETTER.findall(letters))
        rtl = total > 0 and arabic * 2 > total
        out.append(" ".join(i["text"] for i in sorted(line, key=lambda i: i["cx"], reverse=rtl)))
    return "\n".join(out), sum(i["conf"] for i in items) / len(items)


def read_path(reader, path):
    return assemble(reader.readtext(str(path), detail=1, paragraph=False))


def read_bytes(reader, data):
    import numpy as np
    import cv2
    image = cv2.imdecode(np.frombuffer(data, np.uint8), cv2.IMREAD_COLOR)
    return assemble(reader.readtext(image, detail=1, paragraph=False))
