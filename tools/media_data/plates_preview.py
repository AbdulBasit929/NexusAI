#!/usr/bin/env python3
"""Write a contact sheet of synthetic plates (clean on the left, as a detector would crop it on the right) so the renderer can be looked at.

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval:latest python /repo/tools/media_data/plates_preview.py /data/generated/plates-preview.jpg
"""
import random
import sys
from pathlib import Path

import cv2
import numpy as np

sys.path.insert(0, str(Path(__file__).resolve().parent))
import plates_synth as ps  # noqa: E402

pf = ps.pf


def main(out):
    rng = random.Random(3)
    rows = []
    for _ in range(12):
        plate = pf.random_plate(rng)
        clean = ps.render_plate(rng, plate)
        field = ps.degrade(rng, clean)
        cell = np.full((190, 760, 3), 235, np.uint8)
        scale = min(410 / clean.shape[1], 170 / clean.shape[0])
        c = cv2.resize(clean, (int(clean.shape[1] * scale), int(clean.shape[0] * scale)))
        cell[10:10 + c.shape[0], 10:10 + c.shape[1]] = c
        fscale = min(310 / field.shape[1], 170 / field.shape[0], 6.0)
        f = cv2.resize(field, (max(1, int(field.shape[1] * fscale)), max(1, int(field.shape[0] * fscale))), interpolation=cv2.INTER_NEAREST)
        cell[10:10 + f.shape[0], 430:430 + f.shape[1]] = f
        cv2.putText(cell, f"{plate['norm']} {plate['format']} {plate['layout']}", (10, 187), cv2.FONT_HERSHEY_SIMPLEX, 0.45, (0, 0, 0), 1)
        rows.append(cell)
    half = len(rows) // 2
    sheet = np.hstack([np.vstack(rows[:half]), np.vstack(rows[half:])])
    cv2.imwrite(out, sheet, [cv2.IMWRITE_JPEG_QUALITY, 90])
    print("wrote", out, sheet.shape)


if __name__ == "__main__":
    main(sys.argv[1])
