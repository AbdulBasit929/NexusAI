#!/usr/bin/env python3
"""Generate a tiny private visual parity pack without modifying reference projects."""

from __future__ import annotations

import argparse
import csv
import json
from pathlib import Path
from typing import Any

import cv2
import numpy as np


def normalize(value: str) -> str:
    return "".join(character for character in value.upper() if character.isalnum())


def fit(image: np.ndarray, width: int = 720, height: int = 520) -> np.ndarray:
    scale = min(width / image.shape[1], height / image.shape[0])
    resized = cv2.resize(image, (round(image.shape[1] * scale), round(image.shape[0] * scale)))
    canvas = np.full((height, width, 3), 245, dtype=np.uint8)
    top = (height - resized.shape[0]) // 2
    left = (width - resized.shape[1]) // 2
    canvas[top:top + resized.shape[0], left:left + resized.shape[1]] = resized
    return canvas


def title(image: np.ndarray, value: str) -> np.ndarray:
    bar = np.full((48, image.shape[1], 3), 255, dtype=np.uint8)
    cv2.putText(bar, value, (14, 32), cv2.FONT_HERSHEY_SIMPLEX, 0.75, (20, 20, 20), 2, cv2.LINE_AA)
    return np.vstack([bar, image])


def annotate(image: np.ndarray, predictions: list[dict[str, Any]], color: tuple[int, int, int]) -> np.ndarray:
    result = image.copy()
    for prediction in predictions:
        bbox = prediction.get("bbox")
        if not bbox:
            continue
        if isinstance(bbox, dict):
            if "x1" in bbox:
                x1, y1, x2, y2 = (int(bbox[key]) for key in ("x1", "y1", "x2", "y2"))
            else:
                x1, y1 = int(bbox["x"]), int(bbox["y"])
                x2, y2 = x1 + int(bbox["width"]), y1 + int(bbox["height"])
        else:
            x1, y1, x2, y2 = (int(value) for value in bbox)
        cv2.rectangle(result, (x1, y1), (x2, y2), color, max(2, round(result.shape[1] / 700)))
        text = str(prediction.get("raw_plate_text") or "")
        cv2.putText(result, text, (x1, max(24, y1 - 8)), cv2.FONT_HERSHEY_SIMPLEX, 0.75, color, 2, cv2.LINE_AA)
    return result


def read_frame(capture: cv2.VideoCapture, frame_number: int) -> np.ndarray:
    capture.set(cv2.CAP_PROP_POS_FRAMES, frame_number)
    ok, frame = capture.read()
    if not ok:
        raise RuntimeError(f"could not decode frame {frame_number}")
    return frame


def contains(event: dict[str, Any], timestamp: float) -> bool:
    return float(event["start_seconds"]) <= timestamp <= float(event["end_seconds"])


def image_pack(args: argparse.Namespace, output: Path) -> list[dict[str, Any]]:
    current = json.loads(args.current_image.read_text(encoding="utf-8"))
    reference = json.loads(args.reference_image.read_text(encoding="utf-8"))
    with args.image_labels.open("r", encoding="utf-8-sig", newline="") as source:
        labels = {row["sample_id"]: row for row in csv.DictReader(source)}
    current_rows = {row["sample_id"]: row for row in current["results"]}
    exact = next(row for row in reference["results"] if row["error_class"] == "exact")
    error = next(row for row in reference["results"] if row["error_class"] != "exact")
    index = []
    for row in (exact, error):
        sample_id = row["sample_id"]
        path = args.image_root / labels[sample_id]["relative_file"]
        original = cv2.imread(str(path))
        if original is None:
            raise RuntimeError(f"could not decode {sample_id}")
        current_view = annotate(original, current_rows[sample_id]["predictions"], (255, 80, 0))
        reference_view = annotate(original, row["predictions"], (0, 150, 0))
        contact = np.hstack([
            title(fit(original), "Original"),
            title(fit(current_view), "CURRENT_NXMMR_BASELINE_V1"),
            title(fit(reference_view), "Frozen FastALPR reference"),
        ])
        target = output / f"image-{sample_id}-comparison.jpg"
        cv2.imwrite(str(target), contact)
        index.append({"kind": "image", "sample_id": sample_id, "error_class": row["error_class"], "path": str(target)})
    return index


def video_pack(args: argparse.Namespace, output: Path) -> list[dict[str, Any]]:
    current = json.loads(args.current_video.read_text(encoding="utf-8"))
    historical = json.loads(args.historical_video.read_text(encoding="utf-8"))
    state = json.loads(args.review_state.read_text(encoding="utf-8"))
    events = [event for event in state["video_events"] if event["event_type"] == "plate"]
    current_frames = current["private_results"]["frames"]
    raw_frames = historical["private_detected_frames"]["raw"]

    current_positive = next(
        frame for frame in current_frames
        if frame["predicted_raw"] and any(contains(event, float(frame["timestamp_seconds"])) for event in events)
    )
    recovered = None
    for event in events:
        event_current_frames = [
            frame for frame in current_frames if contains(event, float(frame["timestamp_seconds"]))
        ]
        if not event_current_frames or any(frame["predicted_raw"] for frame in event_current_frames):
            continue
        truth = {normalize(value) for value in str(event["plate_text"]).splitlines() if value.strip()}
        recovered = next((
            frame for frame in raw_frames
            if contains(event, float(frame["timestamp_seconds"]))
            and truth.intersection(normalize(value) for value in frame["predicted_raw"])
        ), None)
        if recovered:
            break
    temporal_fp = next(
        frame for frame in raw_frames
        if not any(contains(event, float(frame["timestamp_seconds"])) for event in events)
    )
    selected = [("current-positive", current_positive), ("reference-recovery", recovered), ("reference-temporal-gap", temporal_fp)]
    if recovered is None:
        raise RuntimeError("no deterministic reference-recovery frame found")

    original_capture = cv2.VideoCapture(str(args.original_video))
    historical_capture = cv2.VideoCapture(str(args.historical_annotated_video))
    index = []
    try:
        for label, selected_frame in selected:
            timestamp = float(selected_frame["timestamp_seconds"])
            frame_number = round(timestamp * 60)
            original = read_frame(original_capture, frame_number)
            historical_view = read_frame(historical_capture, frame_number)
            nearest = min(current_frames, key=lambda frame: abs(float(frame["timestamp_seconds"]) - timestamp))
            if abs(float(nearest["timestamp_seconds"]) - timestamp) < 0.01:
                current_view = annotate(original, nearest["predictions"], (255, 80, 0))
                current_label = "CURRENT_NXMMR sampled frame"
            else:
                current_view = original.copy()
                current_label = "Nearest 1s policy sample had no detection"
            contact = np.hstack([
                title(fit(original), f"Original at {timestamp:.3f}s"),
                title(fit(current_view), current_label),
                title(fit(historical_view), "Historical dense reference output"),
            ])
            target = output / f"video-{label}-{timestamp:.3f}s-comparison.jpg"
            cv2.imwrite(str(target), contact)
            index.append({"kind": "video", "selection": label, "timestamp_seconds": timestamp, "path": str(target)})
    finally:
        original_capture.release()
        historical_capture.release()
    return index


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image-root", type=Path, required=True)
    parser.add_argument("--image-labels", type=Path, required=True)
    parser.add_argument("--current-image", type=Path, required=True)
    parser.add_argument("--reference-image", type=Path, required=True)
    parser.add_argument("--original-video", type=Path, required=True)
    parser.add_argument("--historical-annotated-video", type=Path, required=True)
    parser.add_argument("--current-video", type=Path, required=True)
    parser.add_argument("--historical-video", type=Path, required=True)
    parser.add_argument("--review-state", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.output.exists() and any(args.output.iterdir()):
        raise RuntimeError("refusing to overwrite a prior visual parity pack")
    args.output.mkdir(parents=True, exist_ok=True)
    index = image_pack(args, args.output) + video_pack(args, args.output)
    (args.output / "index.json").write_text(json.dumps(index, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"output": str(args.output), "comparisons": len(index)}))


if __name__ == "__main__":
    main()
