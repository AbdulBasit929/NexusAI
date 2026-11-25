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


def read_clock_frame(frame_bgr: np.ndarray, box: tuple[int, int, int, int]) -> dict[str, Any]:
    """One frame, the clock's box (x, y, width, height; the camera configuration an analyst sets once): the crop is enlarged and read as dark-on-light and as light-on-dark; the first reading that is a valid
    time wins. Returns the text read and the time in seconds, or None for both when nothing valid was read."""
    import cv2

    x, y, w, h = box
    crop = frame_bgr[max(0, y):y + h, max(0, x):x + w]
    if crop.size == 0:
        return {"text": "", "seconds": None}
    gray = cv2.cvtColor(crop, cv2.COLOR_BGR2GRAY)
    big = cv2.resize(gray, None, fx=4, fy=4, interpolation=cv2.INTER_CUBIC)
    _, otsu = cv2.threshold(big, 0, 255, cv2.THRESH_BINARY + cv2.THRESH_OTSU)
    for image in (otsu, 255 - otsu, big, 255 - big):
        padded = cv2.copyMakeBorder(image, 12, 12, 12, 12, cv2.BORDER_CONSTANT, value=int(255 if image.mean() > 127 else 0))
        text = _tesseract_digits(padded)
        seconds = parse_clock_text(text)
        if seconds is not None:
            return {"text": text, "seconds": seconds}
    return {"text": "", "seconds": None}


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
    series = motion_series(path, motion)
    per_second = int(round(motion.analysis_fps))
    energy = [min(0.1, float(series[i:i + per_second].max())) if series[i:i + per_second].size else 0.0 for i in range(0, max(n, 0) * per_second, per_second)][:n]
    if energy:
        energy[0] = 0.0            # the first step of a clip is an encoder transient (a re-encoded or cut copy starts with a different first frame): it is ignored, and larger values are capped above
    return {"method": "dhash64-per-second+motion-series", "overview_version": OVERVIEW_VERSION, "seconds": n, "hash": [format(v, "016x") for v in seconds_hash],
            "mirror_hash": [format(v, "016x") for v in seconds_mirror], "motion": [round(v, 5) for v in energy]}


def _hamming(a: int, b: int) -> int:
    return bin(a ^ b).count("1")


def compare_signatures(a: dict[str, Any], b: dict[str, Any], bit_threshold: int = 12, match_fraction: float = 0.8, min_overlap_seconds: int = 20, motion_correlation: float = 0.7,
                       flat: float = 0.002) -> dict[str, Any]:
    """Whether two videos are the same footage: for each shift of one against the other (sliding alignment), the share of overlapping seconds whose hashes differ by at most `bit_threshold` bits (also against the
    mirrored hashes), and the motion series must agree over the overlap (correlation, or both flat). The best shift that passes both is reported. A candidate for an analyst to check, never proof of identity."""
    ha, hb, hbm = [int(x, 16) for x in a["hash"]], [int(x, 16) for x in b["hash"]], [int(x, 16) for x in b["mirror_hash"]]
    ma, mb = np.asarray(a["motion"], np.float32), np.asarray(b["motion"], np.float32)
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
