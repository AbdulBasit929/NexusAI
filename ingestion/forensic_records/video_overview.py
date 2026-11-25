# SPDX-License-Identifier: MIT
"""Video overview (programme step MF-1, docs/data/video-overview-preregistration-20261125.md): what a video is and where something happens in it, without any model.

Parts here: P1 probe (what the file is, or why it cannot be read), P3 motion index (intervals where the picture changes, from frame differences of a small grey stream), P4 shots (cut positions from the
FFmpeg scene filter). Everything reads the file through ffmpeg/ffprobe and writes nothing; a source file is never modified. Every result says what produced it (method and version) so the reading contract of
MF-0 (docs/roadmap/mf0-reading-contract-and-moments-20261125.md) can show it. These are measurements of the picture, not statements about people or events: an interval is "the picture changed here".

The parameters of the motion index are chosen on the dev scenes only (the pre-registration); the defaults below are the values chosen, and the sealed scenes are read once with them.
"""

from __future__ import annotations

import json
import re
import subprocess
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Iterator

import numpy as np

OVERVIEW_VERSION = "video-overview-1.0.0-development"


# ---------------------------------------------------------------------------------------------------------------------------------
# P1 probe
# ---------------------------------------------------------------------------------------------------------------------------------
def _run(command: list[str], timeout: float = 120.0) -> subprocess.CompletedProcess:
    return subprocess.run(command, capture_output=True, text=True, timeout=timeout)


def _fraction(text: str | None) -> tuple[int, int]:
    if not text or "/" not in text:
        return 0, 0
    a, b = text.split("/", 1)
    try:
        return int(a), int(b)
    except ValueError:
        return 0, 0


def probe_video(path: str | Path) -> dict[str, Any]:
    """What the file is. An unreadable or truncated file is returned as `readable: False` with the reason from ffprobe, never as an invented value, and never raises."""
    path = Path(path)
    base: dict[str, Any] = {"method": "ffprobe", "overview_version": OVERVIEW_VERSION, "file": path.name}
    try:
        size = path.stat().st_size
    except OSError as exc:
        return {**base, "readable": False, "reason": f"cannot open the file: {exc.strerror or exc}"}
    if size == 0:
        return {**base, "readable": False, "reason": "the file is empty (zero bytes)", "bytes": 0}
    try:
        done = _run(["ffprobe", "-v", "error", "-show_format", "-show_streams", "-of", "json", str(path)])
    except (OSError, subprocess.TimeoutExpired) as exc:
        return {**base, "readable": False, "reason": f"ffprobe could not run: {exc}", "bytes": size}
    if done.returncode != 0:
        first = (done.stderr.strip().splitlines() or ["ffprobe failed"])[-1]
        return {**base, "readable": False, "reason": f"the container could not be read: {first[:200]}", "bytes": size}
    try:
        info = json.loads(done.stdout)
    except json.JSONDecodeError:
        return {**base, "readable": False, "reason": "ffprobe gave no usable answer", "bytes": size}
    streams = info.get("streams", [])
    video = next((s for s in streams if s.get("codec_type") == "video" and not (s.get("disposition") or {}).get("attached_pic")), None)
    if video is None:
        return {**base, "readable": False, "reason": "the file has no video stream", "bytes": size, "streams": [s.get("codec_type") for s in streams]}
    fmt = info.get("format", {})
    num, den = _fraction(video.get("avg_frame_rate")) if _fraction(video.get("avg_frame_rate"))[1] else _fraction(video.get("r_frame_rate"))
    fps = num / den if den else 0.0
    frames = int(video["nb_frames"]) if str(video.get("nb_frames", "")).isdigit() else None
    # duration: the video stream's own length (frames over rate) before the container's, which may include a longer audio track
    if frames is not None and fps > 0:
        duration = frames / fps
        duration_source = "frames over frame rate"
    elif video.get("duration") not in (None, "N/A"):
        duration, duration_source = float(video["duration"]), "video stream"
    elif fmt.get("duration") not in (None, "N/A"):
        duration, duration_source = float(fmt["duration"]), "container"
    else:
        duration, duration_source = None, None
    tags = {k.lower(): v for k, v in (fmt.get("tags") or {}).items()}
    return {
        **base, "readable": True, "bytes": size, "container": (fmt.get("format_name") or "").split(",")[0], "video_codec": video.get("codec_name"), "width": int(video.get("width", 0)),
        "height": int(video.get("height", 0)), "fps": round(fps, 4), "fps_num": num, "fps_den": den, "frames": frames, "duration_seconds": round(duration, 4) if duration is not None else None,
        "duration_source": duration_source, "has_audio": any(s.get("codec_type") == "audio" for s in streams), "audio_codec": next((s.get("codec_name") for s in streams if s.get("codec_type") == "audio"), None),
        "bit_rate": int(fmt["bit_rate"]) if str(fmt.get("bit_rate", "")).isdigit() else None, "creation_time": tags.get("creation_time"),
    }


# ---------------------------------------------------------------------------------------------------------------------------------
# P3 motion index
# ---------------------------------------------------------------------------------------------------------------------------------
@dataclass(frozen=True)
class MotionParameters:
    analysis_fps: float = 5.0           # frames of the small stream per second of video
    width: int = 160
    height: int = 90
    blur: int = 1                       # box blur radius in pixels of the small frame (0 = none): sensor noise is suppressed before differencing
    pixel_threshold: float = 14.0       # a pixel counts as changed when its grey level moved by more than this (on top of the video's own noise floor)
    changed_fraction: float = 0.0016    # share of changed pixels that makes a frame step "active" (0.0016 of 14400 pixels = about 23 pixels)
    close_gap_seconds: float = 2.0      # active steps closer together than this are one interval
    minimum_seconds: float = 1.0        # an interval shorter than this is dropped
    pad_seconds: float = 0.2            # intervals are widened by this on each side (the object enters before the first changed step)


def _read_small_frames(path: str | Path, parameters: MotionParameters, start: float = 0.0, duration: float | None = None) -> Iterator[np.ndarray]:
    command = ["ffmpeg", "-v", "error", "-nostdin"]
    if start:
        command += ["-ss", str(start)]
    command += ["-i", str(path)]
    if duration:
        command += ["-t", str(duration)]
    command += ["-an", "-vf", f"fps={parameters.analysis_fps},scale={parameters.width}:{parameters.height}:flags=area,format=gray", "-f", "rawvideo", "-"]
    process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    size = parameters.width * parameters.height
    try:
        while True:
            raw = process.stdout.read(size)
            if len(raw) < size:
                break
            yield np.frombuffer(raw, np.uint8).reshape(parameters.height, parameters.width)
    finally:
        process.stdout.close()
        process.kill()
        process.wait()


def _blur(frame: np.ndarray, radius: int) -> np.ndarray:
    if radius <= 0:
        return frame.astype(np.float32)
    f = frame.astype(np.float32)
    k = 2 * radius + 1
    padded = np.pad(f, radius, mode="edge")
    c = np.cumsum(np.cumsum(padded, axis=0), axis=1)
    c = np.pad(c, ((1, 0), (1, 0)))
    h, w = f.shape
    return (c[k:k + h, k:k + w] - c[:h, k:k + w] - c[k:k + h, :w] + c[:h, :w]) / float(k * k)


def motion_series(path: str | Path, parameters: MotionParameters = MotionParameters(), start: float = 0.0, duration: float | None = None) -> np.ndarray:
    """Per analysis step: the share of pixels whose grey level moved by more than the threshold since the step before, after the video's global brightness change (lamp, exposure) is removed."""
    shares: list[float] = []
    previous = None
    for frame in _read_small_frames(path, parameters, start, duration):
        current = _blur(frame, parameters.blur)
        if previous is not None:
            diff = current - previous
            diff -= np.median(diff)             # a global change of brightness moves every pixel alike: it is not motion
            shares.append(float((np.abs(diff) > parameters.pixel_threshold).mean()))
        previous = current
    return np.asarray(shares, dtype=np.float32)


def intervals_from_series(series: np.ndarray, parameters: MotionParameters = MotionParameters(), offset_seconds: float = 0.0) -> list[dict[str, float]]:
    """Active steps -> intervals: close small gaps, drop short ones, pad. Step i is the change between analysis frames i and i+1, so it starts at (i / fps)."""
    if series.size == 0:
        return []
    step = 1.0 / parameters.analysis_fps
    active = np.flatnonzero(series > parameters.changed_fraction)
    runs: list[list[float]] = []
    for i in active:
        start, end = i * step, (i + 1) * step
        if runs and start - runs[-1][1] <= parameters.close_gap_seconds:
            runs[-1][1] = end
        else:
            runs.append([start, end])
    total = (series.size + 1) * step
    out = []
    for start, end in runs:
        if end - start < parameters.minimum_seconds:
            continue
        out.append({"start_seconds": round(max(0.0, start - parameters.pad_seconds) + offset_seconds, 3), "end_seconds": round(min(total, end + parameters.pad_seconds) + offset_seconds, 3)})
    return out


def motion_index(path: str | Path, parameters: MotionParameters = MotionParameters()) -> dict[str, Any]:
    series = motion_series(path, parameters)
    intervals = intervals_from_series(series, parameters)
    covered = sum(i["end_seconds"] - i["start_seconds"] for i in intervals)
    total = (series.size + 1) / parameters.analysis_fps if series.size else 0.0
    return {"method": "frame-difference-160x90-grey", "overview_version": OVERVIEW_VERSION, "parameters": parameters.__dict__, "intervals": intervals, "interval_count": len(intervals),
            "covered_seconds": round(covered, 3), "analysed_seconds": round(total, 3), "covered_fraction": round(covered / total, 5) if total else 0.0,
            "reading": "candidate: the picture changed in these intervals; it does not say what moved"}


# ---------------------------------------------------------------------------------------------------------------------------------
# P4 shots (cuts)
# ---------------------------------------------------------------------------------------------------------------------------------
def shot_cuts(path: str | Path, threshold: float = 0.3) -> dict[str, Any]:
    """Cut positions from FFmpeg's scene filter (a scene score above `threshold`, 0 to 1). Dissolves and fades change the picture gradually and are mostly not found by a single threshold; the pre-registration
    reports them without a gate. Position is the presentation time of the first frame of the new shot."""
    command = ["ffmpeg", "-v", "error", "-nostdin", "-i", str(path), "-an", "-vf", f"select='gt(scene,{threshold})',metadata=print:file=-", "-f", "null", "-"]
    done = subprocess.run(command, capture_output=True, text=True)
    cuts = []
    for match in re.finditer(r"pts_time:([0-9.]+)\s+lavfi\.scene_score=([0-9.]+)", done.stdout):
        cuts.append({"seconds": float(match.group(1)), "score": float(match.group(2))})
    return {"method": "ffmpeg-scene-filter", "overview_version": OVERVIEW_VERSION, "threshold": threshold, "cuts": cuts, "cut_count": len(cuts),
            "reading": "candidate: the picture changes abruptly at these times"}
