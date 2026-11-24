#!/usr/bin/env python3
"""OFIQ face-image quality (gates Q1 to Q3 of docs/data/face-image-search-preregistration-20261119.md) through the third-party package `ofiqpy` 0.2.2 and the BSI model files.

Stages (the first and last run in the normal evaluation environment, the middle one in the isolated OFIQ environment /opt/ofiq):

    ... --entrypoint python         nexusai-media-eval-face:20261119 /repo/tools/media_data/face_ofiq.py prepare
    ... --entrypoint /opt/ofiq/bin/python nexusai-media-eval-face:20261119 /repo/tools/media_data/face_ofiq.py run
    ... --entrypoint python         nexusai-media-eval-face:20261119 /repo/tools/media_data/face_ofiq.py score

prepare: unpack the model archive and the conformance images (names checked), list every licence file that came with the models, cut the FairFace sample (300 Indian + 300 White by SHA-256 order, original and degraded copy).
run:     assess the 28 conformance images (Q1) and the FairFace sample (Q2), one image at a time with 4 threads; writes CSV and timing.
score:   compare with the C++ expected results (Q1), the assessment rate and time (Q2), and the Spearman correlation between the quality of the degraded copy and its genuine cosine score with AuraFace (Q3).
"""
import csv
import hashlib
import json
import os
import shutil
import sys
import time
import zipfile
from pathlib import Path

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
WORK = Path(os.environ.get("NEXUSAI_FACE_WORK", "/work"))
OFIQ = WORK / "ofiq-data"
SAMPLE = 300


def safe_extract(archive, dest):
    dest.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(archive) as z:
        for member in z.namelist():
            if not str((dest / member).resolve()).startswith(str(dest.resolve())):
                raise RuntimeError(f"unsafe path in {archive}: {member}")
        z.extractall(dest)


def prepare():
    import cv2
    import numpy as np
    import pyarrow.parquet as pq
    sys.path.insert(0, "/repo/tools/media_data")
    import face_common as fc
    from face_fairface import degrade
    OFIQ.mkdir(parents=True, exist_ok=True)
    shutil.copy(Path(DATA) / "datasets" / "face" / "ofiq-data" / "ofiq_config.jaxn", OFIQ / "ofiq_config.jaxn")
    safe_extract(Path(DATA) / "models" / "face" / "ofiq" / "OFIQ-MODELS.zip", OFIQ)
    safe_extract(Path(DATA) / "datasets" / "face" / "ofiq" / "OFIQ-IMAGES.zip", OFIQ / "tests")
    licences = [p for p in OFIQ.rglob("*") if p.is_file() and any(w in p.name.lower() for w in ("licen", "copying", "notice", "readme"))]
    keep = WORK / "results" / "ofiq-model-licences"
    keep.mkdir(parents=True, exist_ok=True)
    for p in licences:
        shutil.copy(p, keep / ("__".join(p.relative_to(OFIQ).parts)))
    print("licence/readme files that came with the OFIQ models and images:", [str(p.relative_to(OFIQ)) for p in licences])
    labels = json.load(open(WORK / "fairface" / "fairface-labels.json"))
    race = labels["race"]
    chosen = {}
    for k in (1, 3):
        ids = sorted((i for i, r in enumerate(race) if r == k), key=lambda i: hashlib.sha256(f"ofiq-{i}".encode()).hexdigest())[:SAMPLE]
        chosen[k] = set(ids)
    wanted = {i: k for k, s in chosen.items() for i in s}
    out = WORK / "ofiq-in"
    for sub in ("orig", "degr"):
        (out / sub).mkdir(parents=True, exist_ok=True)
    pf = pq.ParquetFile(Path(DATA) / "datasets" / "face" / "fairface" / "validation-1.25.parquet")
    i0, n = 0, 0
    for batch in pf.iter_batches(batch_size=256, columns=["image"]):
        for k, cell in enumerate(batch.column("image").to_pylist()):
            i = i0 + k
            if i in wanted:
                bgr = cv2.imdecode(np.frombuffer(cell["bytes"], np.uint8), cv2.IMREAD_COLOR)
                cv2.imwrite(str(out / "orig" / f"{wanted[i]}_{i}.png"), bgr)
                cv2.imwrite(str(out / "degr" / f"{wanted[i]}_{i}.png"), degrade(bgr))
                n += 1
        i0 += len(batch)
    json.dump({"indian": sorted(chosen[1]), "white": sorted(chosen[3])}, open(out / "sample.json", "w"))
    print(f"FairFace sample written: {n} originals and {n} degraded copies in {out}")


def assess_all(assessor, paths, label):
    rows, started = [], time.perf_counter()
    times = []
    for k, p in enumerate(paths):
        t0 = time.perf_counter()
        try:
            result = assessor.assess(p)
            status = str(result.status)
            comps = {}
            for name, value in dict(result.components).items():
                comps[name] = value
        except Exception as error:  # a failure is recorded, never hidden
            status, comps = f"ERROR {type(error).__name__}: {error}", {}
        times.append(time.perf_counter() - t0)
        rows.append((p, status, comps))
        if k % 100 == 0:
            print(f"  {label} {k}/{len(paths)}", flush=True)
    return rows, times


def scalar_of(value):
    """Best-effort reading of an ofiqpy component value: object with .scalar / .raw, or a (raw, scalar) tuple, or a number."""
    for attr in ("scalar", "scalar_score", "quality"):
        if hasattr(value, attr):
            return getattr(value, attr)
    if isinstance(value, (tuple, list)) and len(value) == 2:
        return value[1]
    if isinstance(value, (int, float)):
        return value
    return None


def run():
    os.environ["OFIQPY_OFIQ_DATA"] = str(OFIQ)
    os.environ.setdefault("OMP_NUM_THREADS", "4")
    from ofiqpy import Assessor
    from ofiqpy.config import OFIQConfig
    assessor = Assessor(OFIQConfig(data_root=OFIQ))
    results = WORK / "results"
    results.mkdir(parents=True, exist_ok=True)
    expected = list(csv.DictReader(open(Path(DATA) / "datasets" / "face" / "ofiq-data" / "expected_results_v1.1.0.csv", encoding="utf-8"), delimiter=";"))
    conf_paths = [OFIQ / "tests" / "images" / Path(r["Filename"]).name for r in expected]   # the CSV holds paths relative to the C++ project layout
    missing = [str(p) for p in conf_paths if not p.exists()]
    print("conformance images missing:", missing)
    conf_rows, conf_times = assess_all(assessor, [p for p in conf_paths if p.exists()], "conformance")
    print("example result repr:", repr(conf_rows[0][2].get("UnifiedQualityScore")), conf_rows[0][1], flush=True)
    dump = {"conformance": [{"file": p.name, "status": s, "components": {k: repr(v) for k, v in c.items()}} for p, s, c in conf_rows], "conformance_seconds": conf_times}
    sample = sorted((WORK / "ofiq-in" / "orig").glob("*.png")) + sorted((WORK / "ofiq-in" / "degr").glob("*.png"))
    rows, times = assess_all(assessor, sample, "fairface")
    dump["fairface"] = [{"file": f"{p.parent.name}/{p.name}", "status": s, "uqs": repr(c.get("UnifiedQualityScore")), "uqs_scalar": scalar_of(c.get("UnifiedQualityScore")) if c else None} for p, s, c in rows]
    dump["fairface_seconds"] = times
    json.dump(dump, open(results / "ofiq-run.json", "w"), indent=1, default=str)
    print("written ofiq-run.json; median seconds per image", sorted(times)[len(times) // 2])


def parse_component(text):
    """ComponentResult(raw=24.33, scalar=63.0, status=<ComponentStatus.SUCCESS: 'success'>, failure=None) -> (raw, scalar, status word)."""
    import re
    m = re.search(r"raw=([-\w.+]+), scalar=([-\w.+]+), status=<ComponentStatus\.(\w+)", str(text))
    if not m:
        return None, None, "UNPARSED"

    def num(v):
        try:
            return float(v)
        except ValueError:
            return None
    return num(m.group(1)), num(m.group(2)), m.group(3)


def score():
    import numpy as np
    from scipy.stats import spearmanr
    run_ = json.load(open(WORK / "results" / "ofiq-run.json"))
    expected = {Path(r["Filename"]).name: r for r in csv.DictReader(open(Path(DATA) / "datasets" / "face" / "ofiq-data" / "expected_results_v1.1.0.csv", encoding="utf-8"), delimiter=";")}
    out = {}
    # ---- Q1: conformance against the C++ expected results (all expected components are successful with non-negative scalars)
    img_ok = uqs_ok = 0
    comp_total = comp_equal = comp_within1 = 0
    max_diff, worst = 0.0, None
    statuses = {}
    for c in run_["conformance"]:
        exp = expected[c["file"]]
        status_ok = "SUCCESS" in str(c["status"])
        img_ok += status_ok
        statuses[str(c["status"])] = statuses.get(str(c["status"]), 0) + 1
        for name, text in c["components"].items():
            raw, scalar, cstat = parse_component(text)
            e = exp.get(name + ".scalar")
            if e in (None, "") or scalar is None:
                continue
            e = float(e)
            comp_total += 1
            d = abs(scalar - e)
            comp_equal += d == 0
            comp_within1 += d <= 1
            if d > max_diff:
                max_diff, worst = d, (c["file"], name, scalar, e)
            if name == "UnifiedQualityScore":
                uqs_ok += d <= 1
    out["Q1"] = {"images": len(run_["conformance"]), "image_status_success": img_ok, "statuses": statuses, "uqs_scalar_within_1_point": uqs_ok, "component_scalars_compared": comp_total, "component_scalars_exactly_equal": comp_equal,
                 "component_scalars_within_1_point": comp_within1, "largest_scalar_difference": max_diff, "largest_difference_at": worst}
    # ---- Q2: assessment rate and time on the FairFace sample
    fair = run_["fairface"]
    scored = [f for f in fair if f["uqs_scalar"] is not None]
    by_status = {}
    for f in fair:
        by_status[str(f["status"])] = by_status.get(str(f["status"]), 0) + 1
    sec = run_["fairface_seconds"]
    out["Q2"] = {"images": len(fair), "with_a_unified_score": len(scored), "share_percent": round(100 * len(scored) / len(fair), 2), "by_status": by_status,
                 "median_seconds_per_image_4_threads_while_another_job_ran": round(float(np.median(sec)), 2), "p95_seconds": round(float(np.percentile(sec, 95)), 2)}
    # ---- degradation seen by OFIQ, and Q3: Spearman between the quality of the degraded copy and its genuine cosine score (AuraFace)
    emb = np.load(WORK / "fairface" / "emb-auraface.npy")
    degr = np.load(WORK / "fairface" / "degr-emb-auraface.npy")
    index = np.load(WORK / "fairface" / "degr-index.npy")
    pos = {int(i): k for k, i in enumerate(index)}
    q = {"orig": {}, "degr": {}}
    for f in scored:
        kind, name = f["file"].split("/")
        q[kind][int(name.split("_")[1].split(".")[0])] = float(f["uqs_scalar"])
    qual, cos = [], []
    for i, qd in q["degr"].items():
        if i in pos and not np.isnan(emb[i, 0]) and not np.isnan(degr[pos[i], 0]):
            a, b = emb[i], degr[pos[i]]
            cos.append(float(a @ b / (np.linalg.norm(a) * np.linalg.norm(b))))
            qual.append(qd)
    rho, pval = spearmanr(qual, cos)
    both = [i for i in q["orig"] if i in q["degr"]]
    out["Q3"] = {"pairs": len(qual), "spearman_quality_of_degraded_copy_vs_genuine_cosine_auraface": round(float(rho), 3), "p_value": float(pval),
                 "mean_unified_score_original": round(float(np.mean([q["orig"][i] for i in both])), 1), "mean_unified_score_degraded": round(float(np.mean([q["degr"][i] for i in both])), 1),
                 "share_of_pairs_where_degraded_scores_lower": round(100 * float(np.mean([q["degr"][i] < q["orig"][i] for i in both])), 1)}
    json.dump(out, open(WORK / "results" / "ofiq-score.json", "w"), indent=1)
    print(json.dumps(out, indent=1))


if __name__ == "__main__":
    {"prepare": prepare, "run": run, "score": score}[sys.argv[1]]()
