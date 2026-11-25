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
import statistics
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



def cut_shares(path: str | Path, parameters: MotionParameters | None = None) -> tuple[np.ndarray, float]:
    """Per step of the small grey stream at the video's own frame rate (at most 25 a second): the share of pixels that changed by more than the pixel threshold after the global brightness change (a lamp
    or exposure step moves every pixel alike) is removed. A hard cut changes most of the picture at once, a passing vehicle or a lamp does not. Returns the shares and the analysis rate."""
    info = probe_video(path)
    fps = min(25.0, float(info.get("fps") or 10.0))
    p = parameters or MotionParameters(analysis_fps=fps, pixel_threshold=20.0)
    p = MotionParameters(**{**p.__dict__, "analysis_fps": fps})
    shares: list[float] = []
    previous = None
    for frame in _read_small_frames(path, p):
        current = _blur(frame, p.blur)
        if previous is not None:
            diff = current - previous
            diff -= np.median(diff)
            shares.append(float((np.abs(diff) > p.pixel_threshold).mean()))
        previous = current
    return np.asarray(shares, np.float32), fps


def shot_cuts_compensated(path: str | Path, share_threshold: float = 0.3, parameters: MotionParameters | None = None) -> dict[str, Any]:
    """Hard cuts: the steps where the brightness-compensated share of changed pixels is at least `share_threshold` and is the largest within 0.3 s (one cut gives one detection). The position is the
    presentation time of the first frame of the new shot. Dissolves and fades change a little at every step and are mostly not found (they are reported without a gate)."""
    shares, fps = cut_shares(path, parameters)
    window = max(1, int(round(0.3 * fps)))
    cuts = []
    for i, s in enumerate(shares):
        if s < share_threshold:
            continue
        lo, hi = max(0, i - window), min(len(shares), i + window + 1)
        if s >= shares[lo:hi].max():
            cuts.append({"seconds": round((i + 1) / fps, 3), "score": round(float(s), 4)})
    return {"method": "brightness-compensated-difference", "overview_version": OVERVIEW_VERSION, "threshold": share_threshold, "cuts": cuts, "cut_count": len(cuts), "analysis_fps": fps,
            "reading": "candidate: most of the picture changes at these times"}



def cut_window_shares(path: str | Path, window: int = 3, pixel_threshold: float = 6.0) -> tuple[np.ndarray, np.ndarray, float]:
    """For the small grey stream at the video's own frame rate: (1) per step the share of pixels that changed after the global brightness change is removed, and (2) per frame i the same share between the mean of the
    `window` frames before i and the mean of the `window` frames from i on. A hard cut changes most of the picture at one step and the change stays; a vehicle or a lamp changes part of it, or all of it alike, for a while.
    Returns (step shares indexed by the frame the step ends on, window shares indexed by frame, analysis rate); entries without enough neighbours are 0."""
    info = probe_video(path)
    fps = min(25.0, float(info.get("fps") or 10.0))
    p = MotionParameters(analysis_fps=fps, pixel_threshold=pixel_threshold)
    frames = [_blur(f, p.blur) for f in _read_small_frames(path, p)]
    n = len(frames)
    step = np.zeros(n, np.float32)
    win = np.zeros(n, np.float32)
    for i in range(1, n):
        d = frames[i] - frames[i - 1]
        d -= np.median(d)
        step[i] = float((np.abs(d) > pixel_threshold).mean())
    for i in range(window, n - window + 1):
        d = np.mean(frames[i:i + window], axis=0) - np.mean(frames[i - window:i], axis=0)
        d -= np.median(d)
        win[i] = float((np.abs(d) > pixel_threshold).mean())
    return step, win, fps


def cuts_from_window_shares(step: np.ndarray, win: np.ndarray, fps: float, step_threshold: float, window_threshold: float, window: int = 3) -> list[dict[str, Any]]:
    cuts = []
    for i in range(len(win)):
        if win[i] < window_threshold or step[i] < step_threshold:
            continue
        lo, hi = max(0, i - window), min(len(win), i + window + 1)
        if win[i] >= win[lo:hi].max():
            cuts.append({"seconds": round(i / fps, 3), "score": round(float(win[i]), 4), "step_share": round(float(step[i]), 4)})
    return cuts


def shot_cuts_window(path: str | Path, step_threshold: float = 0.20, window_threshold: float = 0.08, window: int = 3, pixel_threshold: float = 6.0) -> dict[str, Any]:
    """Hard cuts by a persistent change: the picture changes at one step (at least `step_threshold` of the pixels, brightness change removed) and the frames after differ from the frames before over at least
    `window_threshold` of the picture. A passing vehicle, a lamp or a dissolve fails one of the two. The position is the presentation time of the first frame of the new shot."""
    step, win, fps = cut_window_shares(path, window, pixel_threshold)
    cuts = cuts_from_window_shares(step, win, fps, step_threshold, window_threshold, window)
    return {"method": "persistent-change-window", "overview_version": OVERVIEW_VERSION, "step_threshold": step_threshold, "window_threshold": window_threshold, "window_frames": window, "pixel_threshold": pixel_threshold, "cuts": cuts,
            "cut_count": len(cuts), "analysis_fps": fps, "reading": "candidate: the picture changes abruptly and stays changed from these times"}


# ---------------------------------------------------------------------------------------------------------------------------------
# P5 clock text
# ---------------------------------------------------------------------------------------------------------------------------------
_TIME = re.compile(r"(?<!\d)([01]?\d|2[0-3])\D?([0-5]\d)\D?([0-5]\d)(?!\d)")


def parse_clock_text(text: str) -> int | None:
    """A time of day in seconds from what the reader returned (`hh:mm:ss` with any separators, or six digits); None when it is not a valid time. A different valid time is a wrong reading, never 'unreadable'."""
    cleaned = (text or "").strip().replace("O", "0").replace("o", "0").replace("l", "1").replace("I", "1")
    match = _TIME.search(cleaned)
    if not match:
        return None
    h, m, sec = (int(g) for g in match.groups())
    return h * 3600 + m * 60 + sec


def _tesseract_digits(image: np.ndarray) -> str:
    import cv2

    ok, png = cv2.imencode(".png", image)
    if not ok:
        return ""
    done = subprocess.run(["tesseract", "stdin", "stdout", "--psm", "7", "-c", "tessedit_char_whitelist=0123456789:"], input=png.tobytes(), capture_output=True, timeout=30)
    return done.stdout.decode("utf-8", "replace").strip()


def _clock_candidates(gray: np.ndarray) -> list[np.ndarray]:
    """Preparations of an enlarged clock crop, each black text on white: the text isolated by a fixed bright threshold (white stamps), by a percentile of the crop (light grey stamps, any brightness), by a dark
    percentile (dark stamps), and by Otsu in both polarities. The order is the order of preference when readings tie."""
    import cv2

    big = cv2.resize(gray, None, fx=4, fy=4, interpolation=cv2.INTER_CUBIC)
    out: list[np.ndarray] = []
    for thr in (200, 225):
        out.append(np.where(big >= thr, 0, 255).astype(np.uint8))
    for p in (94, 97):
        out.append(np.where(big >= np.percentile(big, p), 0, 255).astype(np.uint8))
    for p in (6, 3):
        out.append(np.where(big <= np.percentile(big, p), 0, 255).astype(np.uint8))
    _, otsu = cv2.threshold(big, 0, 255, cv2.THRESH_BINARY + cv2.THRESH_OTSU)
    out += [otsu, 255 - otsu]
    return out


def read_clock_frame(frame_bgr: np.ndarray, box: tuple[int, int, int, int]) -> dict[str, Any]:
    """One frame, the clock's box (x, y, width, height; the camera configuration an analyst sets once). The crop is enlarged and prepared several ways (see _clock_candidates); each preparation is read as digits
    and the time most of them agree on wins (ties go to the earlier preparation). Returns the text, the time in seconds and the number of agreeing readings; None when no preparation gave a valid time."""
    import cv2
    from collections import Counter

    x, y, w, h = box
    crop = frame_bgr[max(0, y):y + h, max(0, x):x + w]
    if crop.size == 0:
        return {"text": "", "seconds": None, "votes": 0}
    gray = cv2.cvtColor(crop, cv2.COLOR_BGR2GRAY)
    readings: list[tuple[int, str]] = []
    for image in _clock_candidates(gray):
        padded = cv2.copyMakeBorder(image, 14, 14, 14, 14, cv2.BORDER_CONSTANT, value=255)
        text = _tesseract_digits(padded)
        seconds = parse_clock_text(text)
        if seconds is not None:
            readings.append((seconds, text))
    if not readings:
        return {"text": "", "seconds": None, "votes": 0}
    counts = Counter(sec for sec, _ in readings)
    best = max(counts.values())
    winner = next(sec for sec, _ in readings if counts[sec] == best)
    return {"text": next(t for sec, t in readings if sec == winner), "seconds": winner, "votes": best}


def read_clock_samples(path: str | Path, box: tuple[int, int, int, int], every_seconds: float = 60.0, start_seconds: float = 0.0, end_seconds: float | None = None) -> list[dict[str, Any]]:
    """The burned-in clock read at one frame every `every_seconds` (default one a minute). Each sample: the position in the video, the text read and the time of day it shows (None when unreadable)."""
    import cv2

    info = probe_video(path)
    if not info.get("readable") or not info.get("duration_seconds"):
        return []
    end = min(end_seconds if end_seconds is not None else info["duration_seconds"], info["duration_seconds"])
    out: list[dict[str, Any]] = []
    t = start_seconds
    while t < end:
        done = subprocess.run(["ffmpeg", "-v", "error", "-nostdin", "-ss", f"{t:.3f}", "-i", str(path), "-frames:v", "1", "-f", "image2pipe", "-vcodec", "png", "-"], capture_output=True)
        frame = cv2.imdecode(np.frombuffer(done.stdout, np.uint8), cv2.IMREAD_COLOR) if done.stdout else None
        read = read_clock_frame(frame, box) if frame is not None else {"text": "", "seconds": None}
        out.append({"position_seconds": round(t, 3), "text": read["text"], "shown_seconds_of_day": read["seconds"]})
        t += every_seconds
    return out


def _wrap(delta: float) -> float:
    return (delta + 43200.0) % 86400.0 - 43200.0


def analyse_clock(samples: list[dict[str, Any]], true_start_seconds_of_day: float, jump_seconds: float = 10.0) -> dict[str, Any]:
    """From clock readings to an offset table. offset(t) = what the clock shows minus the true time (the container's start plus the position), for each readable sample. The drift is the median slope between
    neighbouring samples; a jump is a step between two neighbours that differs from that slope by `jump_seconds` or more and stays (a reading that jumps away and back is a misread digit and is dropped).
    The table has one segment per stretch between jumps with the offset at its start; the drift applies inside it. Candidates for an analyst to check against the picture, not corrections of any record."""
    valid = [s for s in samples if s.get("shown_seconds_of_day") is not None]
    result: dict[str, Any] = {"method": "tesseract-digits-clock", "overview_version": OVERVIEW_VERSION, "samples": len(samples), "readable": len(valid), "jump_threshold_seconds": jump_seconds,
                              "reading": "candidate: what the burned-in clock shows against the container's start time; check against the picture"}
    if len(valid) < 2:
        return {**result, "drift_seconds_per_hour": None, "initial_offset_seconds": None, "jumps": [], "segments": [], "dropped_as_misreads": 0}
    t = [float(s["position_seconds"]) for s in valid]
    raw = [_wrap(s["shown_seconds_of_day"] - (true_start_seconds_of_day + s["position_seconds"])) for s in valid]
    off = [raw[0]]
    for i in range(1, len(raw)):
        off.append(off[-1] + _wrap(raw[i] - raw[i - 1]))          # unwrapped: a step of up to 12 hours is read as a step, not as a day
    drift = statistics.median((off[i + 1] - off[i]) / (t[i + 1] - t[i]) for i in range(len(off) - 1))
    # a sample that is off its neighbours' straight line while the neighbours agree with each other is a misread
    dropped = set()
    for i in range(1, len(off) - 1):
        if abs((off[i + 1] - off[i - 1]) - drift * (t[i + 1] - t[i - 1])) < jump_seconds:
            expected = off[i - 1] + drift * (t[i] - t[i - 1])
            if abs(off[i] - expected) >= jump_seconds:
                dropped.add(i)
    keep = [i for i in range(len(off)) if i not in dropped]
    t, off = [t[i] for i in keep], [off[i] for i in keep]
    jumps = []
    for i in range(len(off) - 1):
        residual = (off[i + 1] - off[i]) - drift * (t[i + 1] - t[i])
        if abs(residual) >= jump_seconds:
            jumps.append({"between_seconds": [t[i], t[i + 1]], "delta_seconds": round(residual)})
    bounds = [0.0] + [j["between_seconds"][1] for j in jumps]
    segments = []
    for n, start in enumerate(bounds):
        end = bounds[n + 1] if n + 1 < len(bounds) else None
        members = [i for i in range(len(t)) if t[i] >= start and (end is None or t[i] < end)]
        if not members:
            continue
        fit = statistics.median(off[i] - drift * t[i] for i in members)
        segments.append({"start_seconds": start, "end_seconds": end, "offset_seconds_at_start": round(fit + drift * start, 2), "samples": len(members)})
    return {**result, "drift_seconds_per_hour": round(drift * 3600.0, 2), "initial_offset_seconds": round(segments[0]["offset_seconds_at_start"], 2) if segments else None,
            "jumps": jumps, "segments": segments, "dropped_as_misreads": len(dropped)}


def clock_offset_at(analysis: dict[str, Any], position_seconds: float) -> float | None:
    """The estimated offset (clock minus true time, seconds) at a position, from the table: the offset at the start of its segment plus the drift since then."""
    drift = (analysis.get("drift_seconds_per_hour") or 0.0) / 3600.0
    for seg in analysis.get("segments") or []:
        if position_seconds >= seg["start_seconds"] and (seg["end_seconds"] is None or position_seconds < seg["end_seconds"]):
            return seg["offset_seconds_at_start"] + drift * (position_seconds - seg["start_seconds"])
    return None


# ---------------------------------------------------------------------------------------------------------------------------------
# P6 same footage
# ---------------------------------------------------------------------------------------------------------------------------------
def file_sha256(path: str | Path) -> str:
    import hashlib

    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def _dhash_bits(small: np.ndarray) -> int:
    """64-bit difference hash of a 9 x 8 grey picture (8 rows, each 8 comparisons of neighbouring pixels)."""
    bits = (small[:, 1:] > small[:, :-1]).astype(np.uint8).reshape(-1)
    value = 0
    for b in bits:
        value = (value << 1) | int(b)
    return value


def _motion_pass(path: str | Path, parameters: MotionParameters) -> tuple[np.ndarray, np.ndarray]:
    """One decoding pass over the small grey stream: per step the share of changed pixels and an 8 x 8 map of which blocks changed (a block is set when more than 3% of its pixels changed)."""
    rows = np.linspace(0, parameters.height, 9).astype(int)[:-1]
    cols = np.linspace(0, parameters.width, 9).astype(int)[:-1]
    area = np.outer(np.diff(np.append(rows, parameters.height)), np.diff(np.append(cols, parameters.width))).astype(np.float32)
    shares: list[float] = []
    layouts: list[np.ndarray] = []
    previous = None
    for frame in _read_small_frames(path, parameters):
        current = _blur(frame, parameters.blur)
        if previous is not None:
            diff = current - previous
            diff -= np.median(diff)
            changed = (np.abs(diff) > parameters.pixel_threshold).astype(np.float32)
            shares.append(float(changed.mean()))
            blocks = np.add.reduceat(np.add.reduceat(changed, rows, axis=0), cols, axis=1) / area
            layouts.append((blocks > 0.03).reshape(-1))
        previous = current
    return np.asarray(shares, np.float32), (np.asarray(layouts, bool) if layouts else np.zeros((0, 64), bool))


def _layout_value(bits: np.ndarray) -> int:
    value = 0
    for b in bits:
        value = (value << 1) | int(b)
    return value


def video_signature(path: str | Path, motion: MotionParameters = MotionParameters()) -> dict[str, Any]:
    """A signature of what the picture shows, one entry per second: a 64-bit difference hash of the frame, the same hash of the mirrored frame, and the share of the picture that moved in that second (the
    motion series). Survives re-encoding, a few per cent of cropping and mirroring; two different recordings of the same camera view share the hash of the background but not the motion series."""
    seconds_hash, seconds_mirror = [], []
    command = ["ffmpeg", "-v", "error", "-nostdin", "-i", str(path), "-an", "-vf", "fps=1,scale=9:8:flags=area,format=gray", "-f", "rawvideo", "-"]
    done = subprocess.run(command, capture_output=True)
    raw = np.frombuffer(done.stdout, np.uint8)
    n = raw.size // 72
    for i in range(n):
        small = raw[i * 72:(i + 1) * 72].reshape(8, 9)
        seconds_hash.append(_dhash_bits(small))
        seconds_mirror.append(_dhash_bits(small[:, ::-1]))
    series, layouts = _motion_pass(path, motion)
    per_second = int(round(motion.analysis_fps))
    layout_hex, layout_mirror_hex = [], []
    for i in range(0, max(n, 0) * per_second, per_second):
        steps = layouts[i:i + per_second]
        merged = steps.any(axis=0) if steps.size else np.zeros(64, bool)
        layout_hex.append(format(_layout_value(merged), "016x"))
        layout_mirror_hex.append(format(_layout_value(merged.reshape(8, 8)[:, ::-1].reshape(-1)), "016x"))
    layout_hex, layout_mirror_hex = layout_hex[:n], layout_mirror_hex[:n]
    if len(layout_hex):
        layout_hex[0] = layout_mirror_hex[0] = format(0, "016x")      # the first second of a clip is ignored, as for the motion series
    energy = [min(0.1, float(series[i:i + per_second].max())) if series[i:i + per_second].size else 0.0 for i in range(0, max(n, 0) * per_second, per_second)][:n]
    if energy:
        energy[0] = 0.0            # the first step of a clip is an encoder transient (a re-encoded or cut copy starts with a different first frame): it is ignored, and larger values are capped above
    return {"method": "dhash64-per-second+motion-series", "overview_version": OVERVIEW_VERSION, "seconds": n, "hash": [format(v, "016x") for v in seconds_hash],
            "mirror_hash": [format(v, "016x") for v in seconds_mirror], "motion": [round(v, 5) for v in energy], "layout": layout_hex, "layout_mirror": layout_mirror_hex}


def _hamming(a: int, b: int) -> int:
    return bin(a ^ b).count("1")


def _layout_grid(value: int) -> np.ndarray:
    return np.array([(value >> (63 - i)) & 1 for i in range(64)], dtype=bool).reshape(8, 8)


def _dilate_grid(grid: np.ndarray) -> np.ndarray:
    padded = np.pad(grid, 1)
    out = np.zeros_like(grid)
    for dy in (0, 1, 2):
        for dx in (0, 1, 2):
            out |= padded[dy:dy + 8, dx:dx + 8]
    return out


def layouts_agree(a: int, b: int, containment: float = 0.6) -> bool:
    """Whether two 8 x 8 motion maps show motion in the same places, with one block of tolerance (a crop or a re-scale moves the edges of what moved): at least `containment` of the set blocks of each lie
    inside the other map widened by one block. A distance in bits is not used: sparse maps (a few blocks set) are always close in bits, even when they have nothing in common."""
    ga, gb = _layout_grid(a), _layout_grid(b)
    na, nb = int(ga.sum()), int(gb.sum())
    if na == 0 or nb == 0:
        return na == nb
    return (ga & _dilate_grid(gb)).sum() / na >= containment and (gb & _dilate_grid(ga)).sum() / nb >= containment


def compare_signatures(a: dict[str, Any], b: dict[str, Any], bit_threshold: int = 12, match_fraction: float = 0.8, min_overlap_seconds: int = 20, motion_correlation: float = 0.7,
                       flat: float = 0.002, layout_containment: float = 0.6, layout_fraction: float = 0.6, layout_min_seconds: int = 3) -> dict[str, Any]:
    """Whether two videos are the same footage: for each shift of one against the other (sliding alignment), the share of overlapping seconds whose hashes differ by at most `bit_threshold` bits (also against the
    mirrored hashes), and the motion series must agree over the overlap (correlation, or both flat). The best shift that passes both is reported. A candidate for an analyst to check, never proof of identity."""
    ha, hb, hbm = [int(x, 16) for x in a["hash"]], [int(x, 16) for x in b["hash"]], [int(x, 16) for x in b["mirror_hash"]]
    ma, mb = np.asarray(a["motion"], np.float32), np.asarray(b["motion"], np.float32)
    la = [int(x, 16) for x in a.get("layout", [])]
    lb, lbm = [int(x, 16) for x in b.get("layout", [])], [int(x, 16) for x in b.get("layout_mirror", [])]
    best = {"same": False, "fraction": 0.0, "shift_seconds": None, "mirrored": False, "motion_agrees": None, "overlap_seconds": 0}
    na, nb = len(ha), len(hb)
    if na == 0 or nb == 0:
        return best
    floor = min(min_overlap_seconds, min(na, nb))
    for shift in range(-(nb - floor), na - floor + 1):          # second i of A is second i - shift of B
        lo, hi = max(0, shift), min(na, nb + shift)
        overlap = hi - lo
        if overlap < floor:
            continue
        for mirrored, hash_b in ((False, hb), (True, hbm)):
            close = sum(1 for i in range(lo, hi) if _hamming(ha[i], hash_b[i - shift]) <= bit_threshold)
            fraction = close / overlap
            if fraction < match_fraction and (best["same"] or fraction <= best["fraction"]):
                continue
            sa, sb = ma[lo:hi], mb[lo - shift:hi - shift]
            if sa.size > 2 and (sa.std() > flat or sb.std() > flat):
                agrees = bool(sa.std() > flat and sb.std() > flat and np.corrcoef(sa, sb)[0, 1] >= motion_correlation)
            else:
                agrees = bool(abs(float(sa.mean()) - float(sb.mean())) < 2 * flat + 0.0005)
            if agrees and la and lb:
                # where the picture moved must agree too, in the seconds in which both moved: two recordings of one camera view share the background but not the paths of what crossed it
                layouts_b = lbm if mirrored else lb
                both = [i for i in range(lo, hi) if la[i] and layouts_b[i - shift]]
                if len(both) >= layout_min_seconds:
                    agrees = sum(1 for i in both if layouts_agree(la[i], layouts_b[i - shift], layout_containment)) / len(both) >= layout_fraction
                elif ma[lo:hi].std() > flat and mb[lo - shift:hi - shift].std() > flat:
                    agrees = False         # both pictures move in this overlap but hardly ever at the same time: a correlation of a few seconds of motion is not evidence of the same footage
                if agrees and len(both) < layout_min_seconds and overlap < 0.5 * min(na, nb):
                    agrees = False         # a short overlap in which nothing moves in both shows only the shared background, which two recordings of one camera view also share
            if fraction >= match_fraction and agrees and (not best["same"] or fraction > best["fraction"]):    # a shift that passes always replaces one that does not
                best = {"same": True, "fraction": round(fraction, 3), "shift_seconds": shift, "mirrored": mirrored, "motion_agrees": True, "overlap_seconds": overlap}
            elif not best["same"] and fraction > best["fraction"]:
                best = {"same": False, "fraction": round(fraction, 3), "shift_seconds": shift, "mirrored": mirrored, "motion_agrees": agrees, "overlap_seconds": overlap}
    return best


# ---------------------------------------------------------------------------------------------------------------------------------
# P7 clip export
# ---------------------------------------------------------------------------------------------------------------------------------
def export_clip(source: str | Path, start_seconds: float, end_seconds: float, destination: str | Path, burn_in: bool = True) -> dict[str, Any]:
    """Cuts [start, end) out of a video into a new file (re-encoded, so the cut is frame exact), with the source file's SHA-256 prefix and the source time burned into the picture, and returns the manifest: source
    SHA-256, the interval, the output SHA-256, the ffmpeg version and the command. The source is only read. The output is decoded afterwards and its frame count compared with the interval."""
    source, destination = Path(source), Path(destination)
    info = probe_video(source)
    if not info.get("readable"):
        raise ValueError(f"cannot export from an unreadable video: {info.get('reason')}")
    source_hash = file_sha256(source)
    fps = info["fps"]
    duration = end_seconds - start_seconds
    if duration <= 0 or start_seconds < 0:
        raise ValueError("the interval must be positive and start inside the video")
    vf = ["setpts=PTS-STARTPTS"]
    if burn_in:
        label = f"{source_hash[:12]}  source time %{{eif\\:t+{start_seconds:.3f}\\:d}}s"
        vf.append(f"drawtext=text='{label}':x=10:y=h-th-10:fontsize=18:fontcolor=white:box=1:boxcolor=black@0.5")
    command = ["ffmpeg", "-v", "error", "-nostdin", "-y", "-ss", f"{start_seconds:.3f}", "-i", str(source), "-t", f"{duration:.3f}", "-vf", ",".join(vf), "-an", "-c:v", "libx264", "-preset", "veryfast",
               "-crf", "20", "-pix_fmt", "yuv420p", "-movflags", "+faststart", str(destination)]
    done = subprocess.run(command, capture_output=True, text=True)
    if done.returncode != 0:
        raise RuntimeError("the export failed: " + done.stderr.strip()[-300:])
    out = probe_video(destination)
    expected = round(duration * fps)
    version = (subprocess.run(["ffmpeg", "-version"], capture_output=True, text=True).stdout.splitlines() or [""])[0]
    return {"manifest_version": 1, "overview_version": OVERVIEW_VERSION, "source_file": source.name, "source_sha256": source_hash, "interval_seconds": [start_seconds, end_seconds],
            "output_file": destination.name, "output_sha256": file_sha256(destination), "output_frames": out.get("frames"), "expected_frames": expected,
            "decodes": bool(out.get("readable")), "frame_count_matches": out.get("frames") == expected, "ffmpeg": version, "command": command, "burned_in": burn_in,
            "reading": "an excerpt of the source with its hash and time burned in; the manifest lets anyone verify it against the original"}


# ---------------------------------------------------------------------------------------------------------------------------------
# Observation packets for the worker (behind the switch FORENSIC_VIDEO_OVERVIEW, default off)
# ---------------------------------------------------------------------------------------------------------------------------------
VIDEO_OVERVIEW_CONTRACT = "forensics.video-overview/v1"


def overview_enabled() -> bool:
    import os

    return os.getenv("FORENSIC_VIDEO_OVERVIEW", "false").strip().lower() in {"1", "true", "yes", "on"}


def overview_packets(path: str | Path, context: dict[str, Any]) -> tuple[list[dict[str, Any]], dict[str, Any]]:
    """What the worker stores for a video when the overview is on: one probe packet, one packet per motion interval and one per shot cut, each carrying the reading contract of MF-0 (tier, method, version, input
    SHA-256, measured error, candidate). The tier is UNMEASURED until the sealed measurement of the pre-registration is recorded in the reading error register. The ids are derived from the source hash and the
    position, so the same file gives the same ids. Returns the packets (as dicts of MediaObservation fields) and a summary for the result metadata."""
    import uuid

    sha, source_file = str(context["source_sha256"]), str(context["source_file"])
    packets: list[dict[str, Any]] = []

    def make(kind: str, key: str, locator: dict[str, Any], payload: dict[str, Any], method: str) -> dict[str, Any]:
        return {
            "observation_id": str(uuid.uuid5(uuid.NAMESPACE_URL, f"{sha}:video-overview:{kind}:{key}")), "contract_version": VIDEO_OVERVIEW_CONTRACT, "observation_type": kind, "confidence": None,
            "citation_locator": {"source_file": source_file, **locator},
            "payload": {**payload, "reading_contract": {"family": kind, "tier": "UNMEASURED", "method": method, "version": OVERVIEW_VERSION, "input_sha256": sha, "measured_error": None, "candidate": True},
                        "parent_evidence_id": context.get("evidence_id"), "parent_version_id": context.get("version_id"), "manual_review_required": True},
            "warnings": ["A measurement of the picture, not a statement about what happened: check against the video"],
        }

    probe = probe_video(path)
    packets.append(make("video_overview_probe_observation", "probe", {"timestamp_seconds": 0}, probe, "ffprobe"))
    summary: dict[str, Any] = {"overview_version": OVERVIEW_VERSION, "readable": bool(probe.get("readable")), "motion_intervals": 0, "shot_cuts": 0}
    if probe.get("readable"):
        motion = motion_index(path)
        for interval in motion["intervals"]:
            packets.append(make("video_motion_interval_observation", f"motion:{interval['start_seconds']}", {"timestamp_seconds": interval["start_seconds"], "end_seconds": interval["end_seconds"]},
                                {**interval, "parameters": motion["parameters"]}, motion["method"]))
        cuts = shot_cuts_window(path)
        for cut in cuts["cuts"]:
            packets.append(make("video_shot_cut_observation", f"cut:{cut['seconds']}", {"timestamp_seconds": cut["seconds"]},
                                {**cut, "step_threshold": cuts["step_threshold"], "window_threshold": cuts["window_threshold"], "pixel_threshold": cuts["pixel_threshold"]}, cuts["method"]))
        summary.update({"motion_intervals": len(motion["intervals"]), "motion_covered_fraction": motion["covered_fraction"], "shot_cuts": len(cuts["cuts"]), "analysed_seconds": motion["analysed_seconds"]})
    return packets, summary
