#!/usr/bin/env python3
"""Prepare the bake-off data inside a container without network (docs/data/face-image-search-preregistration-20261119.md).

    docker run --rm --network none --memory 2000m -v C:/tmp/nexusai-media-data:/data:ro -v C:/tmp/nexusai-media-data/work-faces:/work -v <repo>:/repo:ro \
        --entrypoint python nexusai-media-eval-face:20261119 /repo/tools/media_data/face_prepare.py bins|wider|fairface

bins:     the five verification `.bin` files are pickles from an unofficial mirror. They are read with a RESTRICTED unpickler that only allows numpy array reconstruction; any other global aborts.
          Output per set: uniq.npy (unique images, 112x112x3 uint8 BGR), pairs.npy (index pairs), issame.npy.
wider:    unpack WIDER_val.zip (names checked against path traversal) and parse the annotation file into wider-val.json.
fairface: read the parquet labels into fairface-labels.json (the images stay in the parquet).
"""
import io
import json
import pickle
import sys
import zipfile
from pathlib import Path

import cv2
import numpy as np

sys.path.insert(0, "/repo/tools/media_data")
import face_common as fc  # noqa: E402

ALLOWED = {
    ("numpy.core.multiarray", "_reconstruct"), ("numpy._core.multiarray", "_reconstruct"), ("numpy", "ndarray"), ("numpy", "dtype"),
    ("numpy.core.multiarray", "scalar"), ("numpy._core.multiarray", "scalar"), ("_codecs", "encode"), ("builtins", "bytearray"),
}


class Restricted(pickle.Unpickler):
    seen = set()

    def find_class(self, module, name):
        Restricted.seen.add((module, name))
        if (module, name) in ALLOWED:
            return super().find_class(module, name)
        raise pickle.UnpicklingError(f"refusing global {module}.{name}")


def prepare_bins():
    import hashlib
    out_root = Path(fc.WORK) / "bins"
    wanted = sys.argv[2:] or ["lfw", "cfp_fp", "agedb_30", "calfw", "cplfw"]
    for name in wanted:
        out = out_root / name
        if (out / "issame.npy").exists():  # written last: a set that has it is complete
            print(f"{name}: already prepared")
            continue
        path = Path(fc.DATA) / "datasets" / "face" / "verification" / f"{name}.bin"
        with open(path, "rb") as handle:
            bins, issame = Restricted(handle, encoding="bytes").load()
        n = len(bins)
        issame = np.asarray(issame, dtype=bool)
        assert n == 2 * len(issame), (name, n, len(issame))
        index, blobs, keys = [], [], {}
        for blob in bins:
            blob = bytes(blob)
            key = hashlib.sha1(blob).digest()
            if key not in keys:
                keys[key] = len(blobs)
                blobs.append(blob)
            index.append(keys[key])
        del bins
        out.mkdir(parents=True, exist_ok=True)
        mm = np.lib.format.open_memmap(out / "uniq.npy", mode="w+", dtype=np.uint8, shape=(len(blobs), 112, 112, 3))
        for k, blob in enumerate(blobs):
            image = cv2.imdecode(np.frombuffer(blob, np.uint8), cv2.IMREAD_COLOR)
            assert image is not None and image.shape == (112, 112, 3), (name, None if image is None else image.shape)
            mm[k] = image
        mm.flush()
        del mm
        np.save(out / "pairs.npy", np.asarray(index, dtype=np.int32).reshape(-1, 2))
        np.save(out / "issame.npy", issame)
        print(f"{name}: {len(issame)} pairs ({int(issame.sum())} same, {int((~issame).sum())} different), {len(blobs)} unique images of {n}; globals seen {sorted(Restricted.seen)}", flush=True)


def prepare_wider():
    src = Path(fc.DATA) / "datasets" / "face" / "widerface"
    out = Path(fc.WORK) / "widerface"
    out.mkdir(parents=True, exist_ok=True)
    for archive in ("WIDER_val.zip", "wider_face_split.zip"):
        with zipfile.ZipFile(src / archive) as z:
            for member in z.namelist():
                target = (out / member).resolve()
                if not str(target).startswith(str(out.resolve())):
                    raise RuntimeError(f"unsafe path in {archive}: {member}")
            z.extractall(out)
    ann = next(out.rglob("wider_face_val_bbx_gt.txt"))
    images_root = next(out.rglob("WIDER_val")) / "images"
    lines = ann.read_text().splitlines()
    i, records = 0, []
    while i < len(lines):
        name = lines[i].strip()
        count = int(lines[i + 1])
        rows = lines[i + 2:i + 2 + max(count, 1)]  # a file with 0 faces still has one placeholder row
        faces = []
        for row in rows[:count]:
            x, y, w, h, blur, expr, illum, invalid, occl, pose = [int(v) for v in row.split()[:10]]
            faces.append({"x": x, "y": y, "w": w, "h": h, "blur": blur, "expression": expr, "illumination": illum, "invalid": invalid, "occlusion": occl, "pose": pose})
        records.append({"file": str(images_root / name), "name": name, "faces": faces})
        i += 2 + max(count, 1)
    json.dump(records, open(out / "wider-val.json", "w"))
    print(f"wider val: {len(records)} images, {sum(len(r['faces']) for r in records)} labelled faces, {sum(1 for r in records for f in r['faces'] if f['invalid'])} flagged invalid")


def prepare_fairface():
    import pyarrow.parquet as pq
    table = pq.read_table(Path(fc.DATA) / "datasets" / "face" / "fairface" / "validation-1.25.parquet", columns=["age", "gender", "race"])
    labels = {"age": table["age"].to_pylist(), "gender": table["gender"].to_pylist(), "race": table["race"].to_pylist()}
    names = ["East Asian", "Indian", "Black", "White", "Middle Eastern", "Latino_Hispanic", "Southeast Asian"]
    labels["race_names"] = names
    out = Path(fc.WORK) / "fairface"
    out.mkdir(parents=True, exist_ok=True)
    json.dump(labels, open(out / "fairface-labels.json", "w"))
    from collections import Counter
    print("fairface validation:", len(labels["race"]), "images;", {names[k]: v for k, v in sorted(Counter(labels["race"]).items())})


def prepare_lfw_orig():
    import shutil
    import tarfile
    src = Path(fc.DATA) / "datasets" / "face" / "lfw-orig"
    out = Path(fc.WORK) / "lfw-orig"
    out.mkdir(parents=True, exist_ok=True)
    with tarfile.open(src / "lfw.tgz") as archive:
        for member in archive.getmembers():
            target = (out / member.name).resolve()
            if not str(target).startswith(str(out.resolve())) or not (member.isfile() or member.isdir()):
                raise RuntimeError(f"unsafe member in lfw.tgz: {member.name}")
        archive.extractall(out, filter="data")
    shutil.copy(src / "pairs.txt", out / "pairs.txt")
    names = list((out / "lfw").rglob("*.jpg"))
    print(f"lfw originals: {len(names)} jpg files under {out / 'lfw'}")


if __name__ == "__main__":
    {"bins": prepare_bins, "wider": prepare_wider, "fairface": prepare_fairface, "lfw-orig": prepare_lfw_orig}[sys.argv[1]]()
