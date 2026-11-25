#!/usr/bin/env python3
"""Real-video check of the video plate methods (docs/data/video-plates-research-and-preregistration-20261111.md): counts, timings and local crops only. No ground truth exists for these videos
(the owner's `sample.mp4`, a motorway seen from an overpass, and public stock clips of fixed cameras), so nothing here is a gate. Plate text is never printed; the per-group results and the
best crop of each group are written under /clips/results and /clips/explore (outside the repository) so that a person can look at them.

    docker run --rm --network none --entrypoint python -v <repo>:/repo:ro -v <repo>/models/media:/models/media:ro -v <video dir>:/v:ro -v C:/tmp/video-plates:/clips \
        nexusai-media-eval:latest /repo/tools/video_plates/evaluate_real.py --video /v/clip.mp4 --tag clip1 --methods b0,v4,v4f,v4t --max-seconds 20

Methods: b0 today's live path (1 frame per second, at most 60); v4 the new processor, whole frame; v4f plus the plate-shape filter; v4t plus native-resolution tiles on large frames.
"""
import argparse
import json
import os
import resource
import sys
import time
import uuid
from dataclasses import asdict
from pathlib import Path

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
import numpy as np  # noqa: E402

CTX = {"source_sha256": "0" * 64, "evidence_id": str(uuid.uuid4()), "version_id": str(uuid.uuid4())}


def norm(text):
    return "".join(ch for ch in (text or "").upper() if ch.isalnum())


def plausible(text):
    return 4 <= len(text) <= 10 and any(c.isdigit() for c in text) and any(c.isalpha() for c in text)


def run_b0(path, nexus, max_seconds):
    engine = nexus.engine
    capture = cv2.VideoCapture(path)
    fps, count = capture.get(cv2.CAP_PROP_FPS), int(capture.get(cv2.CAP_PROP_FRAME_COUNT))
    duration = min(count / fps, max_seconds) if max_seconds else count / fps
    seen, frames, t = {}, 0, 0.0
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
        t += 1.0
    capture.release()
    return {"groups": sorted(seen.values(), key=lambda g: g["first"]), "frames": frames, "crops": {}}


def run_v4(processor, path, max_seconds, crops_dir, name):
    from video_anpr_parity_adapter import TimeInterval
    allowed = [TimeInterval(0.0, float(max_seconds))] if max_seconds else None
    run = processor.run_video(Path(path), context={**CTX, "source_file": os.path.basename(path)}, allowed_intervals=allowed)
    groups = [{"plate": g.selected_normalized_plate, "support": g.support_count, "first": g.first_observed_seconds, "last": g.last_observed_seconds,
               "best_frame": next((o.frame_number for o in run.observations if o.observation_id == g.best_observation_id), None),
               "bbox": [g.bounds_at_best_observation.x, g.bounds_at_best_observation.y, g.bounds_at_best_observation.width, g.bounds_at_best_observation.height]} for g in run.groups]
    # best crop of each group, for a person to look at (local only)
    capture = cv2.VideoCapture(path)
    os.makedirs(crops_dir, exist_ok=True)
    for i, g in enumerate(groups):
        capture.set(cv2.CAP_PROP_POS_FRAMES, g["best_frame"])
        ok, frame = capture.read()
        if ok:
            x, y, w, h = [int(v) for v in g["bbox"]]
            crop = frame[y:y + h, x:x + w]
            if crop.size:
                cv2.imwrite(f"{crops_dir}/{name}-g{i:02d}.png", cv2.resize(crop, None, fx=3, fy=3, interpolation=cv2.INTER_CUBIC))
    capture.release()
    return {"groups": groups, "frames": run.coverage.frames_decoded, "frames_refined": run.frames_refined, "observations": len(run.observations), "coverage": asdict(run.coverage)}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--video", required=True)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--methods", default="b0,v4,v4f,v4t")
    parser.add_argument("--max-seconds", type=float, default=0)
    parser.add_argument("--base-fps", type=float, default=4.0)
    args = parser.parse_args()
    from plate_processor_nexus import NexusPlateImageProcessor
    from video_anpr_nexus_v4 import HighResolutionPlateEngine, VideoANPRNexusV4Processor
    nexus = NexusPlateImageProcessor.from_environment(minimum_confidence=0.5)
    base_engine = nexus.engine
    capture = cv2.VideoCapture(args.video)
    width, height, fps, count = int(capture.get(3)), int(capture.get(4)), capture.get(cv2.CAP_PROP_FPS), int(capture.get(cv2.CAP_PROP_FRAME_COUNT))
    capture.release()
    duration = count / fps
    print(f"video {os.path.basename(args.video)} {width}x{height} {fps:.1f} fps {duration:.1f} s; analysing {min(duration, args.max_seconds) if args.max_seconds else duration:.1f} s")
    os.makedirs("/clips/results", exist_ok=True)
    results = {}
    for method in [m.strip() for m in args.methods.split(",") if m.strip()]:
        started = time.perf_counter()
        if method == "b0":
            nexus.engine = base_engine
            out = run_b0(args.video, nexus, args.max_seconds)
        elif method == "b1":
            from media_pipeline import FastALPRImageProcessor
            from video_anpr_onnx_v3 import VideoANPROnnxV3Processor
            from video_anpr_parity_adapter import TimeInterval
            v3 = VideoANPROnnxV3Processor(FastALPRImageProcessor.from_environment(minimum_confidence=0.25), cv2)
            run = v3.run_video(Path(args.video), context={**CTX, "source_file": os.path.basename(args.video)}, allowed_intervals=[TimeInterval(0.0, float(args.max_seconds))] if args.max_seconds else None)
            out = {"groups": [{"plate": g.selected_normalized_plate, "support": g.support_count, "first": g.first_observed_seconds, "last": g.last_observed_seconds} for g in run.groups],
                   "frames": run.result.metadata["coverage"]["frames_decoded"]}
        elif method == "v4p":
            out = run_v4(VideoANPRNexusV4Processor.from_environment(), args.video, args.max_seconds, f"/clips/explore/{args.tag}", method)
        elif method == "v4s":
            from video_anpr_nexus_v4 import default_engine_processor
            nexus.engine = base_engine
            out = run_v4(VideoANPRNexusV4Processor(nexus, cv2, extra_processors=(default_engine_processor(),), engine_every=(2, 1), base_fps=args.base_fps), args.video, args.max_seconds, f"/clips/explore/{args.tag}", method)
        elif method in ("v4", "v4f", "v4t"):
            if method == "v4":
                nexus.engine = base_engine
            elif method == "v4f":
                nexus.engine = HighResolutionPlateEngine(base_engine, tile_side=None)
            elif method == "v4t":
                nexus.engine = HighResolutionPlateEngine(base_engine, tile_side=1280, wide_px=1500)
            processor = VideoANPRNexusV4Processor(nexus, cv2, base_fps=args.base_fps)
            out = run_v4(processor, args.video, args.max_seconds, f"/clips/explore/{args.tag}", method)
        else:
            raise SystemExit(f"unknown method {method}")
        out["wall"] = round(time.perf_counter() - started, 1)
        results[method] = out
        groups = out["groups"]
        strong = [g for g in groups if g["support"] >= 3]
        print(f"{method:4} groups={len(groups):3} support>=3={len(strong):3} plausible_format(support>=3)={sum(plausible(g['plate']) for g in strong):3} frames={out['frames']} "
              f"refined={out.get('frames_refined', 0)} wall={out['wall']}s ({out['wall'] / (min(duration, args.max_seconds) if args.max_seconds else duration):.1f}x real time) rss={resource.getrusage(resource.RUSAGE_SELF).ru_maxrss // 1024}MiB", flush=True)
    names = list(results)
    for i, a in enumerate(names):
        for b in names[i + 1:]:
            sa = {g["plate"] for g in results[a]["groups"] if g["support"] >= 3}
            sb = {g["plate"] for g in results[b]["groups"] if g["support"] >= 3}
            print(f"   agreement {a} vs {b}: both {len(sa & sb)}, only {a} {len(sa - sb)}, only {b} {len(sb - sa)}")
    json.dump({"video": os.path.basename(args.video), "size": [width, height], "results": results}, open(f"/clips/results/real-{args.tag}.json", "w", encoding="utf-8"), indent=1)


if __name__ == "__main__":
    main()
