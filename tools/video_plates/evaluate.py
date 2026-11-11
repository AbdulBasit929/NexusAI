#!/usr/bin/env python3
"""Runs the methods of docs/data/video-plates-research-and-preregistration-20261111.md on the clips of one set and scores them against the gates.

    docker run --rm --network none --entrypoint python -v <repo>:/repo:ro -v <repo>/models/media:/models/media:ro -v C:/tmp/video-plates:/clips \
        nexusai-media-eval:latest /repo/tools/video_plates/evaluate.py --set dev --methods b0,b1,b2,v4 [--limit N]

Methods: b0 today's live path (1 frame per second, at most 60, M5 engine per frame, one group per distinct string); b1 V3 as built (licence-clean fast-alpr default engine, 4 frames per second);
b2 V3's scheduler and policy with the M5 engine; v4 the method of the decision; v4-norefine, v4-nosharp ablations; v4-charvote reports what a per-character vote would have read (not shippable).
Writes /clips/results/<set>-<method>.json (per clip) and prints the gate table. Strings in the key are synthetic.
"""
import argparse
import json
import os
import resource
import statistics
import sys
import time
import uuid

os.environ.setdefault("FORENSIC_ANPR_ONNX_PROVIDERS", "CPUExecutionProvider")
os.environ.setdefault("FORENSIC_ANPR_NEXUS_DETECTOR_MODEL_PATH", "/models/media/nexus-plates/rfdetr_alpr.onnx")
os.environ.setdefault("FORENSIC_ANPR_NEXUS_OCR_MODEL_PATH", "/models/media/nexus-plates/cct_s_v2_global.onnx")
os.environ.setdefault("FORENSIC_ANPR_NEXUS_OCR_CONFIG_PATH", "/models/media/nexus-plates/cct_s_v2_global_plate_config.yaml")
os.environ.setdefault("FORENSIC_ANPR_DETECTOR_MODEL_PATH", "/models/media/open-image-models/yolo-v9-t-384-license-plate-end2end/yolo-v9-t-384-license-plates-end2end.onnx")
os.environ.setdefault("FORENSIC_ANPR_OCR_MODEL_PATH", "/models/media/fast-plate-ocr/cct-xs-v2-global-model/cct_xs_v2_global.onnx")
os.environ.setdefault("FORENSIC_ANPR_OCR_CONFIG_PATH", "/models/media/fast-plate-ocr/cct-xs-v2-global-model/cct_xs_v2_global_plate_config.yaml")
sys.path.insert(0, "/repo")
sys.path.insert(0, "/repo/ingestion/forensic_records")

import cv2  # noqa: E402

CTX = {"source_sha256": "0" * 64, "evidence_id": str(uuid.uuid4()), "version_id": str(uuid.uuid4())}


def edit_distance(a, b):
    prev = list(range(len(b) + 1))
    for i, ca in enumerate(a, 1):
        cur = [i]
        for j, cb in enumerate(b, 1):
            cur.append(min(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + (ca != cb)))
        prev = cur
    return prev[-1]


def norm(text):
    return "".join(ch for ch in (text or "").upper() if ch.isalnum())


def groups_from_v(run):
    return [{"plate": g.selected_normalized_plate, "support": g.support_count, "first": g.first_observed_seconds, "last": g.last_observed_seconds,
             "alternatives": [(a.normalized_plate_text, a.support_count) for a in g.alternatives]} for g in run.groups]


def run_b0(path, state):
    """Today's live path: one frame per second (at most 60), the M5 engine per frame, one group per distinct normalised string and no support threshold."""
    engine = state["nexus"].engine
    capture = cv2.VideoCapture(path)
    fps, count = capture.get(cv2.CAP_PROP_FPS), int(capture.get(cv2.CAP_PROP_FRAME_COUNT))
    duration = count / fps
    seen, frames = {}, 0
    step = max(1.0, duration / 60)
    t = 0.0
    while t < duration and frames < 60:
        capture.set(cv2.CAP_PROP_POS_FRAMES, round(t * fps))
        ok, frame = capture.read()
        if ok:
            frames += 1
            for r in engine.predict(frame):
                text = norm(r.ocr.text if r.ocr else "")
                if text:
                    entry = seen.setdefault(text, {"plate": text, "support": 0, "first": t, "last": t})
                    entry["support"] += 1
                    entry["last"] = t
        t += step
    capture.release()
    return {"groups": sorted(seen.values(), key=lambda g: g["first"]), "frames": frames, "observations": sum(g["support"] for g in seen.values())}


def run_v3_like(processor, path):
    run = processor.run_video(__import__("pathlib").Path(path), context={**CTX, "source_file": os.path.basename(path)})
    cov = run.result.metadata["coverage"]
    return {"groups": groups_from_v(run), "frames": cov["frames_decoded"], "observations": len(run.observations)}


def run_v4(processor, path):
    run = processor.run_video(__import__("pathlib").Path(path), context={**CTX, "source_file": os.path.basename(path)})
    out = {"groups": groups_from_v(run), "frames": run.coverage.frames_decoded, "frames_refined": run.frames_refined, "observations": len(run.observations)}
    # what a per-character vote over each group's reads would have said (an ablation; a string nobody read is not shippable)
    votes = []
    for g in run.groups:
        reads = [o for o in run.observations if g.first_observed_seconds <= o.timestamp_seconds <= g.last_observed_seconds and edit_distance(o.normalized_ocr, g.selected_normalized_plate) <= 1]
        length = max((len(o.normalized_ocr) for o in reads), default=0)
        text = ""
        for i in range(length):
            tally = {}
            for o in reads:
                if i < len(o.normalized_ocr):
                    tally[o.normalized_ocr[i]] = tally.get(o.normalized_ocr[i], 0.0) + o.support_weight()
            text += max(tally, key=tally.get) if tally else ""
        votes.append(text)
    out["charvote"] = votes
    return out


def build_methods(names):
    from plate_processor_nexus import NexusPlateImageProcessor
    from video_anpr_nexus_v4 import VideoANPRNexusV4Processor
    from video_anpr_onnx_v3 import VideoANPROnnxV3Processor
    state = {"nexus": NexusPlateImageProcessor.from_environment(minimum_confidence=0.5)}
    methods = {}
    if "b0" in names:
        methods["b0"] = lambda p: run_b0(p, state)
    if "b1" in names:
        from media_pipeline import FastALPRImageProcessor
        v3_default = VideoANPROnnxV3Processor(FastALPRImageProcessor.from_environment(minimum_confidence=0.25), cv2)
        methods["b1"] = lambda p: run_v3_like(v3_default, p)
    if "b2" in names:
        v3_nexus = VideoANPROnnxV3Processor(state["nexus"], cv2)
        methods["b2"] = lambda p: run_v3_like(v3_nexus, p)
    for name, refine, sharp in (("v4", True, True), ("v4-norefine", False, True), ("v4-nosharp", True, False)):
        if name in names:
            processor = VideoANPRNexusV4Processor(state["nexus"], cv2, refinement=refine, sharpness_weight=sharp)
            methods[name] = (lambda proc: (lambda p: run_v4(proc, p)))(processor)
    return methods


def score(clips, results):
    pos = [c for c in clips if c["kind"] == "positive"]
    neg = [c for c in clips if c["kind"] != "positive"]
    read = top = spurious = timed = exact = 0
    for c in pos:
        groups = results[c["clip"]]["groups"]
        truth = c["plate"]
        hit = [g for g in groups if g["plate"] == truth]
        read += bool(hit)
        if groups and max(groups, key=lambda g: (g["support"], -g["first"]))["plate"] == truth:
            top += 1
        spurious += sum(1 for g in groups if g["support"] >= 2 and edit_distance(g["plate"], truth) > 1)
        if hit:
            exact += 1
            g0 = min(hit, key=lambda g: g["first"])
            if c["legible_start_seconds"] is not None and c["legible_start_seconds"] - 1 <= g0["first"] <= c["legible_end_seconds"] + 1:
                timed += 1
    neg_with = sum(1 for c in neg if any(g["support"] >= 2 for g in results[c["clip"]]["groups"]))
    ratio = [results[c["clip"]]["wall"] / c["duration_seconds"] for c in clips]
    return {"positives": len(pos), "clip_read": read, "clip_read_rate": round(read / len(pos), 3), "top_group_exact": top, "spurious_groups_per_positive": round(spurious / len(pos), 3),
            "negatives": len(neg), "negative_clips_with_group_support_2plus": neg_with, "time_sane_of_exact": f"{timed}/{exact}",
            "median_wall_over_duration": round(statistics.median(ratio), 2), "max_wall_over_duration": round(max(ratio), 2),
            "median_frames": statistics.median(results[c["clip"]]["frames"] for c in clips)}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--set", choices=["dev", "sealed"], required=True)
    parser.add_argument("--methods", default="b0,b1,b2,v4")
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--tag", default="")
    args = parser.parse_args()
    key = json.load(open(f"/clips/key-{args.set}.json", encoding="utf-8"))
    clips = key["clips"][: args.limit] if args.limit else key["clips"]
    names = [m.strip() for m in args.methods.split(",") if m.strip()]
    methods = build_methods(names)
    os.makedirs("/clips/results", exist_ok=True)
    for name in names:
        results = {}
        for c in clips:
            path = f"/clips/{args.set}/{c['file']}"
            started = time.perf_counter()
            out = methods[name](path)
            out["wall"] = round(time.perf_counter() - started, 2)
            results[c["clip"]] = out
            print(f"{name} {c['clip']} {c['kind'][:12]:12} truth={c['plate']} -> {[(g['plate'], g['support']) for g in out['groups']][:4]} wall={out['wall']}s frames={out['frames']}", flush=True)
        summary = score(clips, results)
        summary["peak_rss_mib"] = round(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024)
        json.dump({"set": args.set, "method": name, "summary": summary, "clips": results}, open(f"/clips/results/{args.set}-{name}{args.tag}.json", "w", encoding="utf-8"), indent=1)
        print("SUMMARY", args.set, name, json.dumps(summary), flush=True)


if __name__ == "__main__":
    main()
