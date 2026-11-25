#!/usr/bin/env python3
"""Scene generator for the video overview (programme step MF-1): synthetic footage with exact truth.

Pre-registration: docs/data/video-overview-preregistration-20261125.md (sections 2 and 3). This file is part of the data of that
registration (VD1-M, VD1-S, VD1-C, VD1-F, VD1-P). Everything is deterministic from the scene id: the same id gives the same pictures,
the same truth and (with the same ffmpeg version) the same bytes, because every random number comes from SHA-256 of the id and the
purpose, and the encoder runs with one thread and bit-exact flags.

Runs inside the worker image (numpy, OpenCV and ffmpeg are there; the host has none of them), without network:

    docker run --rm --network none --memory 1500m -v <out>:/out -v <repo>/tools/showcase:/tools:ro \\
        --entrypoint python <worker image> -B /tools/build_video_overview_scenes.py --partition dev --parts motion,cuts --limit 3 --out /out

Parts (one set of files per scene id, under <out>/<id>/):
    motion     640x360, 10 fps, 3 min: static camera, planted motion intervals with exact truth, nuisances that are not motion (light drift,
               exposure steps, sensor noise); one scene in four has no motion at all                          -> motion.mp4, motion.truth.json (VD1-M)
    cuts       the same format, 14 shots joined by 8 hard cuts, 3 dissolves (12 frames) and 2 dips to black (10 + 10 frames), with lamp-on steps
               and a large passing vehicle as events that are NOT cuts                                           -> cuts.mp4, cuts.truth.json (VD1-S)
    clock      1280x720, 1 frame per second, --clock-minutes long (default 120), a burned-in hh:mm:ss clock in one of four conditions, planted
               drift (+2 or -2 s per hour) and planted jumps (10 s to 6 h), the camera clock starting off the true time -> clock.mp4, clock.truth.json (VD1-C)
    signature  the motion clip re-encoded at three bitrates, cropped by 5%, mirrored, trimmed (extra, reported only), plus two hard negatives
               (same camera view, different recording: H1 same light, H2 other time of day)                       -> signature/*.mp4, signature.truth.json (VD1-F)
Not per scene:
    probe      100 good files (mp4/h264, mkv/h264, avi/mpeg4, mov) and 20 planted bad files with exact truth      -> probe/, probe.truth.json (VD1-P)
    budget     one hour of 1280x720 at 10 fps with motion, 5 hard cuts and a burned-in clock (MF1-B)            -> budget/budget.mp4, budget.truth.json

Partition: a scene is dev if the last hex digit of the SHA-256 of its id is 0 to 9, a or b; sealed if c to f. Of mf1-0001 to mf1-0120 that is 95
dev and 25 sealed. Sealed scenes are generated only for the sealed run: --allow-sealed is required for any of them.

Truth rules (written here so a scorer cannot guess them):
    * An object pass is CORE in a frame when at least 25% of the object's box is inside the picture, FRINGE when any of it is. A motion interval
      is the run of CORE frames; the fringe frames next to it are listed and are not penalised by a scorer that allows its tolerance (the
      results will state it). Intervals of different objects are at least 4 s apart.
    * A hard cut is the first frame of the new shot. A dissolve is the 12 frames [start, end) in which the new shot is blended in. A dip to black
      is 10 frames fading out then 10 frames fading in around the cut frame. Lamp-on steps and occluders are listed under non_cut_events.
    * The clock shows floor(start_time_of_day + t + offset(t)) as hh:mm:ss; offset(t) = initial offset + drift * t / 3600 + the planted jumps.
      The truth lists the offset at every full minute. hh:mm:ss wraps at 24 h.
    * Hard negatives for the signature part share the camera view (the same background) and differ in everything that happens in front of it.
      A plain one-frame-per-second perceptual hash is expected to find H1 hard; that is the point of the gate.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import subprocess
import sys
import time

import numpy as np

try:  # the unit tests of the planning code run without OpenCV
    import cv2
except ImportError:  # pragma: no cover
    cv2 = None

GENERATOR_VERSION = "mf1-scenes-1"
SCENE_IDS = ["mf1-%04d" % n for n in range(1, 121)]
DEV_DIGITS = "0123456789ab"

MOTION_W, MOTION_H, MOTION_FPS, MOTION_SECONDS = 640, 360, 10, 180
CLOCK_W, CLOCK_H = 1280, 720
N_SHOTS = 14
TRANSITION_PLAN = ["hard"] * 8 + ["dissolve"] * 3 + ["fade"] * 2
DISSOLVE_FRAMES, FADE_FRAMES = 12, 10
MIN_SHOT_SECONDS = 6.5
MIN_GAP_SECONDS = 4.0

# object sizes at 640 pixels of picture width: (width, height)
KINDS = {"person": (22, 58), "cyclist": (42, 46), "car": (88, 38), "van": (108, 50), "bus": (330, 170)}
CONTRAST = {"day": (62, 44, 32), "dusk": (50, 36, 26), "night": (40, 30, 22)}
CLOCK_STYLES = {
    "clean_day": dict(height_frac=0.040, color=(255, 255, 255), outline=True, thickness=2, crf=23, condition="day", sigma=2.0),
    "night": dict(height_frac=0.040, color=(175, 175, 175), outline=False, thickness=2, crf=26, condition="night", sigma=7.0),
    "compressed": dict(height_frac=0.040, color=(255, 255, 255), outline=True, thickness=2, crf=35, condition="day", sigma=2.5),
    "small_font": dict(height_frac=0.0195, color=(255, 255, 255), outline=True, thickness=1, crf=28, condition="day", sigma=2.0),
}
CLOCK_CONDITIONS = ["clean_day", "night", "compressed", "small_font"]
JUMP_SECONDS = [10, 25, 90, 300, 2400, 3600, 10800, 21600]


# ---------------------------------------------------------------------------------------------------------------------------------
# determinism and partition
# ---------------------------------------------------------------------------------------------------------------------------------
def sha_hex(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def partition_of(scene_id):
    return "dev" if sha_hex(scene_id)[-1] in DEV_DIGITS else "sealed"


def seed_of(*parts):
    return int(sha_hex("|".join(str(p) for p in parts))[:15], 16)


def rng_of(*parts):
    return np.random.default_rng(seed_of(*parts))


def condition_of(scene_id, purpose="motion"):
    r = rng_of(scene_id, purpose, "condition").random()
    return "day" if r < 0.5 else ("dusk" if r < 0.7 else "night")


def sigma_of(rng, condition):
    lo, hi = {"day": (1.2, 2.5), "dusk": (2.0, 3.5), "night": (3.0, 6.0)}[condition]
    return float(rng.uniform(lo, hi))


# ---------------------------------------------------------------------------------------------------------------------------------
# pictures
# ---------------------------------------------------------------------------------------------------------------------------------
def make_background(rng, W, H, condition):
    base = {"day": rng.uniform(120, 150), "dusk": rng.uniform(70, 95), "night": rng.uniform(24, 40)}[condition]
    yy = np.linspace(-1.0, 1.0, H, dtype=np.float32)[:, None]
    xx = np.linspace(-1.0, 1.0, W, dtype=np.float32)[None, :]
    lum = base * (1.0 + 0.16 * yy * rng.choice([-1.0, 1.0]) + 0.10 * xx * rng.uniform(-1.0, 1.0))
    img = np.repeat(lum[:, :, None], 3, axis=2)
    img *= np.array([rng.uniform(0.92, 1.08) for _ in range(3)], dtype=np.float32)[None, None, :]
    img = np.clip(img, 0, 255).astype(np.uint8)
    for _ in range(int(rng.integers(6, 11))):
        w = int(rng.integers(W // 10, W // 3))
        h = int(rng.integers(H // 8, H // 2))
        x0 = int(rng.integers(0, W - w))
        y0 = int(rng.integers(0, H - h))
        tone = np.clip(base * rng.uniform(0.55, 1.4) * rng.uniform(0.9, 1.1, 3), 0, 255)
        cv2.rectangle(img, (x0, y0), (x0 + w, y0 + h), tuple(float(v) for v in tone), -1)
    low = rng.normal(0, 1, (H // 24 + 2, W // 24 + 2)).astype(np.float32)
    low = cv2.resize(low, (W, H), interpolation=cv2.INTER_CUBIC) * base * 0.05
    fine = rng.normal(0, 1, (H, W)).astype(np.float32) * 2.0
    out = img.astype(np.float32) + (low + fine)[:, :, None]
    return np.clip(out, 0, 255).astype(np.uint8)


def relight(bg, strength, direction):
    """The same camera view at another time of day: a different shading across the picture and a different overall level."""
    H, W = bg.shape[:2]
    xx = np.linspace(-1.0, 1.0, W, dtype=np.float32)[None, :, None]
    yy = np.linspace(-1.0, 1.0, H, dtype=np.float32)[:, None, None]
    gain = 1.0 + strength * (direction * xx + 0.4 * yy) - 0.05 * direction
    return np.clip(bg.astype(np.float32) * gain, 0, 255).astype(np.uint8)


class Canvas:
    def __init__(self, w, h):
        self.img = np.zeros((h, w, 3), np.uint8)
        self.mask = np.zeros((h, w), np.uint8)

    def rect(self, x0, y0, x1, y1, color):
        cv2.rectangle(self.img, (int(x0), int(y0)), (int(x1), int(y1)), color, -1)
        cv2.rectangle(self.mask, (int(x0), int(y0)), (int(x1), int(y1)), 255, -1)

    def circle(self, cx, cy, r, color):
        cv2.circle(self.img, (int(cx), int(cy)), int(max(1, r)), color, -1)
        cv2.circle(self.mask, (int(cx), int(cy)), int(max(1, r)), 255, -1)


def make_sprite(kind, scale, tone):
    w0, h0 = KINDS[kind]
    w, h = max(6, int(round(w0 * scale))), max(8, int(round(h0 * scale)))
    t = np.array(tone, dtype=np.float32)
    body = tuple(float(v) for v in t)
    dark = tuple(float(v) for v in np.clip(t * 0.55, 0, 255))
    light = tuple(float(v) for v in np.clip(t * 1.25 + 10, 0, 255))
    c = Canvas(w, h)
    if kind == "person":
        r = max(2, w // 4)
        c.rect(w // 6, 2 * r, w - 1 - w // 6, int(0.6 * h), body)
        c.rect(w // 5, int(0.6 * h), w // 2 - 1, h - 1, dark)
        c.rect(w // 2 + 1, int(0.6 * h), w - 1 - w // 5, h - 1, dark)
        c.circle(w // 2, r, r, light)
    elif kind == "cyclist":
        r = max(2, h // 4)
        c.circle(w // 5 + r // 2, h - 1 - r, r, dark)
        c.circle(w - 1 - w // 5 - r // 2, h - 1 - r, r, dark)
        c.rect(w // 3, int(0.35 * h), w * 2 // 3, int(0.62 * h), body)
        c.circle(w // 2, int(0.2 * h), max(2, h // 8), light)
    elif kind in ("car", "van"):
        top = 0 if kind == "van" else int(0.15 * h)
        c.rect(0, int(0.42 * h), w - 1, h - 1 - h // 6, body)
        c.rect(int(0.18 * w), top, int(0.78 * w), int(0.46 * h), light)
        c.rect(int(0.24 * w), top + max(1, h // 14), int(0.72 * w), int(0.40 * h), dark)
        r = max(2, h // 5)
        c.circle(int(0.22 * w), h - 1 - r, r, dark)
        c.circle(int(0.78 * w), h - 1 - r, r, dark)
    else:  # bus
        c.rect(0, 0, w - 1, h - 1 - h // 8, body)
        n = 6
        for i in range(n):
            x0 = int((0.05 + 0.15 * i) * w)
            c.rect(x0, int(0.12 * h), x0 + int(0.11 * w), int(0.42 * h), dark)
        r = max(3, h // 6)
        c.circle(int(0.2 * w), h - 1 - r, r, dark)
        c.circle(int(0.8 * w), h - 1 - r, r, dark)
    return c.img, c.mask


def paste(frame, sprite, mask, x, y):
    h, w = mask.shape
    H, W = frame.shape[:2]
    x0, y0, x1, y1 = max(0, x), max(0, y), min(W, x + w), min(H, y + h)
    if x1 <= x0 or y1 <= y0:
        return
    m = mask[y0 - y:y1 - y, x0 - x:x1 - x] > 0
    frame[y0:y1, x0:x1][m] = sprite[y0 - y:y1 - y, x0 - x:x1 - x][m]


class Pass:
    """One object crossing the picture. k = 0 is about the first core frame of the planned interval."""

    def __init__(self, kind, scale, direction, y, t0, dur_frames, W, tone):
        self.kind, self.direction, self.y, self.t0, self.W = kind, direction, int(y), int(t0), int(W)
        self.sprite, self.mask = make_sprite(kind, scale, tone)
        self.h, self.w = self.mask.shape
        self.v = (self.W + 0.5 * self.w) / max(1, int(dur_frames))
        span = int(2 * self.w / self.v) + 8
        core, fringe = [], []
        for k in range(self.t0 - span, self.t0 + int(dur_frames) + span):
            f = self.fraction(k)
            if f > 0:
                fringe.append(k)
            if f >= 0.25:
                core.append(k)
        assert core and core == list(range(core[0], core[-1] + 1)), "core frames must be one run"
        assert fringe == list(range(fringe[0], fringe[-1] + 1)), "fringe frames must be one run"
        self.core = (core[0], core[-1] + 1)
        self.fringe = (fringe[0], fringe[-1] + 1)

    def x_at(self, k):
        kk = k - self.t0
        if self.direction > 0:
            return int(round(-0.75 * self.w + self.v * kk))
        return int(round((self.W - 0.25 * self.w) - self.v * kk))

    def fraction(self, k):
        x = self.x_at(k)
        vis = min(x + self.w, self.W) - max(x, 0)
        return max(0, vis) / float(self.w)

    def visible(self, k):
        return self.fringe[0] <= k < self.fringe[1]

    def draw(self, frame, k):
        paste(frame, self.sprite, self.mask, self.x_at(k), self.y)

    def truth(self):
        return {"kind": self.kind, "direction": self.direction, "box": [int(self.w), int(self.h)], "y": self.y,
                "core_start_frame": self.core[0], "core_end_frame": self.core[1],
                "fringe_start_frame": self.fringe[0], "fringe_end_frame": self.fringe[1]}


def plan_intervals(rng, fps, n_frames, n, lo_s=3.0, hi_s=40.0, gap_s=MIN_GAP_SECONDS, margin_s=2.0):
    """n non-overlapping intervals (start_frame, duration_frames), each lo_s to hi_s long, at least gap_s apart."""
    if n <= 0:
        return []
    total = n_frames / float(fps)
    durs = np.clip(lo_s + rng.exponential(7.0, n), lo_s, hi_s)
    while n > 0 and durs.sum() + gap_s * (n - 1) + 2 * margin_s > 0.9 * total:
        if np.all(durs <= lo_s + 1e-9):
            n -= 1
            durs = durs[:n]
        else:
            durs = np.clip(durs * 0.9, lo_s, hi_s)
    if n <= 0:
        return []
    free = total - 2 * margin_s - durs.sum() - gap_s * (n - 1)
    extra = rng.dirichlet(np.ones(n + 1)) * free
    out, pos = [], margin_s + extra[0]
    for i in range(n):
        out.append((int(round(pos * fps)), max(1, int(round(durs[i] * fps)))))
        pos += durs[i] + gap_s + extra[i + 1]
    return out


def build_passes(rng, W, H, fps, bg, condition, intervals, kinds=("person", "person", "cyclist", "car", "van")):
    scale = W / 640.0
    out = []
    for start, dur in intervals:
        kind = str(rng.choice(kinds))
        h = max(8, int(round(KINDS[kind][1] * scale)))
        y = int(rng.integers(int(H * 0.30), max(int(H * 0.30) + 1, H - h - 4)))
        mean = float(bg[y:y + h, :, :].mean())
        sign = -1.0 if mean > 115 else 1.0
        contrast = float(rng.choice(CONTRAST[condition]))
        tone = np.clip((mean + sign * contrast) * rng.uniform(0.95, 1.05, 3), 5, 250)
        out.append(Pass(kind, scale, int(rng.choice([-1, 1])), y, start, dur, W, tone))
    return out


class Shot:
    """A static-camera view: background, sensor noise, slow light, steps in exposure and the object passes. frame(k) works for any k."""

    def __init__(self, W, H, fps, condition, bg_key, noise_key, sigma, passes=(), light_amp=0.0, light_period_s=60.0, light_phase=0.0,
                 nuisances=(), relight_args=None, shift=(0, 0), bank=48, bg=None):
        self.W, self.H, self.fps, self.K = W, H, fps, bank
        self.bg = make_background(rng_of(*bg_key, "bg"), W, H, condition) if bg is None else bg.copy()
        if relight_args:
            self.bg = relight(self.bg, *relight_args)
        if shift != (0, 0):
            self.bg = np.roll(self.bg, shift=(shift[1], shift[0]), axis=(0, 1))
        self.passes = list(passes)
        self.light_amp, self.light_period, self.light_phase = light_amp, light_period_s, light_phase
        self.nuisances = list(nuisances)
        nr = rng_of(*noise_key, "noise")
        self.noise = np.clip(np.rint(nr.normal(0, sigma, (bank, H, W))), -127, 127).astype(np.int8)
        self.composed = np.clip(self.bg[None].astype(np.int16) + self.noise[:, :, :, None], 0, 255).astype(np.uint8)

    def light(self, k):
        v = self.light_amp * math.sin(2 * math.pi * (k / float(self.fps)) / self.light_period + self.light_phase)
        for n in self.nuisances:
            v += n["delta"] * min(1.0, max(0.0, (k - n["at_frame"]) / float(max(1, n["ramp"]))))
        return int(round(v))

    def frame(self, k):
        idx = (k * 7 + 3) % self.K
        vis = [p for p in self.passes if p.visible(k)]
        lamp = self.light(k)
        if not vis and lamp == 0:
            return self.composed[idx].copy()
        base = self.bg.copy()
        for p in vis:
            p.draw(base, k)
        out = base.astype(np.int16) + self.noise[idx][:, :, None]
        if lamp:
            out += lamp
        np.clip(out, 0, 255, out=out)
        return out.astype(np.uint8)


# ---------------------------------------------------------------------------------------------------------------------------------
# the motion scene (VD1-M)
# ---------------------------------------------------------------------------------------------------------------------------------
def motion_shot(scene_id, variant="a", W=MOTION_W, H=MOTION_H, fps=MOTION_FPS, seconds=MOTION_SECONDS):
    """variant a is the scene itself; h1 is the same camera view recorded again (same light, other events); h2 the same view at another time of day."""
    n_frames = int(seconds * fps)
    condition = condition_of(scene_id)
    prng = rng_of(scene_id, variant, "plan")
    sigma = sigma_of(prng, condition)
    none = seed_of(scene_id, "none") % 4 == 0
    n = 0 if (none and variant == "a") else int(prng.integers(6, 13))
    bg_key = (scene_id, "motion")
    probe_bg = make_background(rng_of(*bg_key, "bg"), W, H, condition)
    if variant == "h2":
        probe_bg = relight(probe_bg, 0.30, 1.0)
    intervals = plan_intervals(prng, fps, n_frames, n)
    passes = build_passes(prng, W, H, fps, probe_bg, condition, intervals)
    nuis = []
    for _ in range(int(prng.integers(0, 3))):
        if n_frames <= int(12 * fps):   # a scene too short for the 5 s margins (only the unit tests use one) gets no exposure step
            break
        nuis.append({"type": "exposure_step", "at_frame": int(prng.integers(int(5 * fps), n_frames - int(5 * fps))),
                     "delta": float(prng.choice([-1, 1]) * prng.uniform(3, 6)), "ramp": 5})
    shot = Shot(W, H, fps, condition, bg_key, (scene_id, variant), sigma, passes,
                light_amp=float(prng.uniform(0, 4)), light_period_s=float(prng.uniform(40, 120)), light_phase=float(prng.uniform(0, 6.28)),
                nuisances=nuis, relight_args=((0.30, 1.0) if variant == "h2" else None))
    shot.meta = {"condition": condition, "noise_sigma": round(sigma, 3), "nuisances": nuis, "light_amplitude": round(shot.light_amp, 3),
                 "n_frames": n_frames}
    return shot


def motion_truth(scene_id, shot, fps=MOTION_FPS):
    objs = [p.truth() for p in shot.passes]
    ivs = [{"start_frame": o["core_start_frame"], "end_frame": o["core_end_frame"],
            "start_s": round(o["core_start_frame"] / fps, 3), "end_s": round(o["core_end_frame"] / fps, 3)} for o in objs]
    ivs.sort(key=lambda d: d["start_frame"])
    covered = sum(d["end_frame"] - d["start_frame"] for d in ivs)
    return {"id": scene_id, "kind": "motion", "partition": partition_of(scene_id), "generator": GENERATOR_VERSION,
            "width": shot.W, "height": shot.H, "fps": fps, "frames": shot.meta["n_frames"], "duration_s": shot.meta["n_frames"] / float(fps),
            "condition": shot.meta["condition"], "noise_sigma": shot.meta["noise_sigma"], "light_amplitude": shot.meta["light_amplitude"],
            "nuisances": shot.meta["nuisances"], "objects": objs, "intervals": ivs, "n_intervals": len(ivs),
            "motion_fraction": round(covered / float(shot.meta["n_frames"]), 5),
            "truth_rule": "interval = run of frames with at least 25% of the object box inside the picture; fringe frames listed per object"}


# ---------------------------------------------------------------------------------------------------------------------------------
# the cut sequence (VD1-S)
# ---------------------------------------------------------------------------------------------------------------------------------
def plan_sequence(scene_id, fps=MOTION_FPS, seconds=MOTION_SECONDS, n_shots=N_SHOTS):
    rng = rng_of(scene_id, "cuts", "plan")
    total = int(seconds * fps)
    min_len = int(round(MIN_SHOT_SECONDS * fps))
    spare = total - n_shots * min_len
    assert spare > 0
    lens = (min_len + np.floor(rng.dirichlet(np.ones(n_shots)) * spare)).astype(int)
    lens[-1] += total - int(lens.sum())
    starts = [int(x) for x in np.concatenate([[0], np.cumsum(lens)[:-1]])]
    kinds = [str(x) for x in rng.permutation(TRANSITION_PLAN)]
    assert len(kinds) == n_shots - 1
    return {"total": total, "lens": [int(x) for x in lens], "starts": starts, "transitions": kinds}


def sequence_shot(scene_id, j, plan, prev_family, W=MOTION_W, H=MOTION_H, fps=MOTION_FPS):
    rng = rng_of(scene_id, "cuts", "shot", j)
    n_frames = plan["lens"][j] + 2 * DISSOLVE_FRAMES
    similar = j > 0 and rng.random() < 0.3
    family = prev_family if similar else (scene_id, "cuts", "family", j)
    condition = condition_of(scene_id, "cuts%d" % (0 if similar else j))
    sigma = sigma_of(rng, condition)
    probe_bg = make_background(rng_of(*family, "bg"), W, H, condition)
    n_pass = int(rng.integers(1, 4))
    intervals = plan_intervals(rng, fps, n_frames, n_pass, lo_s=2.0, hi_s=10.0, gap_s=2.0, margin_s=0.5)
    passes = build_passes(rng, W, H, fps, probe_bg, condition, intervals)
    events = []
    nuis = []
    if rng.random() < 0.4 and n_frames > 4 * fps:
        at = int(rng.integers(int(1.5 * fps), n_frames - int(1.5 * fps)))
        nuis.append({"type": "lamp_on", "at_frame": at, "delta": float(rng.choice([-1, 1]) * 25), "ramp": 2})
        events.append({"type": "lamp_on", "local_frame": at, "delta": nuis[-1]["delta"]})
    if rng.random() < 0.2 and n_frames > 6 * fps:
        at = int(rng.integers(int(1.5 * fps), n_frames - int(4 * fps)))
        bus = build_passes(rng, W, H, fps, probe_bg, condition, [(at, int(2.5 * fps))], kinds=("bus",))
        passes.extend(bus)
        events.append({"type": "occluder", "local_core_start": bus[0].core[0], "local_core_end": bus[0].core[1]})
    shift = (int(rng.integers(-40, 41)), int(rng.integers(-20, 21))) if similar else (0, 0)
    shot = Shot(W, H, fps, condition, family, (scene_id, "cuts", j), sigma, passes, light_amp=float(rng.uniform(0, 3)),
                light_period_s=float(rng.uniform(40, 120)), light_phase=float(rng.uniform(0, 6.28)), nuisances=nuis, shift=shift, bank=24)
    shot.meta = {"family": family, "similar_to_previous": bool(similar), "events": events, "condition": condition}
    return shot


def sequence_effects(plan):
    """Per global frame: how the output is made. ('blend', j, alpha) mixes shot j (continuing) with shot j+1; ('black', alpha) darkens."""
    fx = {}
    truth, ends = [], [plan["starts"][j] + plan["lens"][j] for j in range(len(plan["lens"]))]
    for j, kind in enumerate(plan["transitions"]):
        e = ends[j]
        if kind == "hard":
            truth.append({"type": "hard", "frame": e})
        elif kind == "dissolve":
            for i in range(DISSOLVE_FRAMES):
                fx[e + i] = ("blend", j, (i + 1) / float(DISSOLVE_FRAMES + 1))
            truth.append({"type": "dissolve", "start_frame": e, "end_frame": e + DISSOLVE_FRAMES, "center_frame": e + DISSOLVE_FRAMES // 2})
        else:
            for i in range(FADE_FRAMES):
                fx[e - FADE_FRAMES + i] = ("black", (i + 1) / float(FADE_FRAMES))
                fx[e + i] = ("black", 1.0 - (i + 1) / float(FADE_FRAMES))
            truth.append({"type": "fade", "start_frame": e - FADE_FRAMES, "end_frame": e + FADE_FRAMES, "center_frame": e})
    return fx, truth


def sequence_frames(scene_id, plan, fx, overlay=None):
    shots = {}
    families = {}

    def get(j):
        if j not in shots:
            shots[j] = sequence_shot(scene_id, j, plan, families.get(j - 1))
            families[j] = shots[j].meta["family"]
            for old in [x for x in shots if x < j - 2]:
                del shots[old]
        return shots[j]

    for j in range(len(plan["lens"])):
        get(j)
    shots.clear()
    j = 0
    for i in range(plan["total"]):
        while j + 1 < len(plan["lens"]) and i >= plan["starts"][j + 1]:
            j += 1
        a = get(j)
        f = a.frame(i - plan["starts"][j])
        e = fx.get(i)
        if e and e[0] == "blend":
            b = get(e[1] + 1)
            fb = b.frame(i - plan["starts"][e[1] + 1])
            f = cv2.addWeighted(f, 1.0 - e[2], fb, e[2], 0)
        elif e and e[0] == "black":
            f = (f.astype(np.float32) * (1.0 - e[1])).astype(np.uint8)
        yield f


def sequence_truth(scene_id, plan, cut_truth, shots_meta, fps=MOTION_FPS):
    non_cut = []
    for j, meta in enumerate(shots_meta):
        s = plan["starts"][j]
        for ev in meta["events"]:
            if ev["type"] == "lamp_on":
                non_cut.append({"type": "lamp_on", "frame": s + ev["local_frame"], "delta": ev["delta"]})
            else:
                non_cut.append({"type": "occluder", "start_frame": s + ev["local_core_start"], "end_frame": s + ev["local_core_end"]})
    return {"id": scene_id, "kind": "cuts", "partition": partition_of(scene_id), "generator": GENERATOR_VERSION,
            "width": MOTION_W, "height": MOTION_H, "fps": fps, "frames": plan["total"], "duration_s": plan["total"] / float(fps),
            "shots": [{"index": j, "start_frame": plan["starts"][j], "end_frame": plan["starts"][j] + plan["lens"][j],
                       "similar_to_previous": shots_meta[j]["similar_to_previous"]} for j in range(len(plan["lens"]))],
            "cuts": cut_truth, "hard_cut_frames": [c["frame"] for c in cut_truth if c["type"] == "hard"],
            "non_cut_events": non_cut,
            "truth_rule": "hard = first frame of the new shot; dissolve = the 12 blended frames; fade = 10 frames out then 10 in around center_frame"}


# ---------------------------------------------------------------------------------------------------------------------------------
# the clock (VD1-C, and inside the budget video)
# ---------------------------------------------------------------------------------------------------------------------------------
class ClockModel:
    def __init__(self, key, duration_s, jumps=None):
        rng = rng_of(key, "clock")
        self.start_sod = int(rng.integers(0, 86400))
        self.init_offset = 0 if rng.random() < 0.3 else int(rng.integers(-300, 301))
        self.drift = float(rng.choice([0.0, 2.0, -2.0], p=[0.4, 0.3, 0.3]))
        self.jumps = []
        n = int(rng.choice([0, 1, 2, 3], p=[0.25, 0.35, 0.25, 0.15])) if jumps is None else jumps
        if duration_s >= 1800:
            for _ in range(200):
                if len(self.jumps) >= n:
                    break
                at = int(rng.integers(600, int(duration_s) - 600))
                if all(abs(at - j["at_s"]) >= 600 for j in self.jumps):
                    self.jumps.append({"at_s": at, "delta_s": int(rng.choice(JUMP_SECONDS)) * int(rng.choice([-1, 1]))})
        self.jumps.sort(key=lambda j: j["at_s"])

    def offset(self, t):
        return self.init_offset + self.drift * t / 3600.0 + sum(j["delta_s"] for j in self.jumps if t >= j["at_s"])

    def shown_sod(self, t):
        return int(math.floor(self.start_sod + t + self.offset(t))) % 86400

    def text(self, t):
        s = self.shown_sod(t)
        return "%02d:%02d:%02d" % (s // 3600, (s // 60) % 60, s % 60)

    def truth(self, duration_s):
        return {"start_time_of_day_s": self.start_sod, "initial_offset_s": self.init_offset, "drift_s_per_hour": self.drift, "jumps": self.jumps,
                "minute_samples": [{"t_s": t, "offset_s": round(self.offset(t), 3), "shown": self.text(t)} for t in range(0, int(duration_s), 60)]}


class ClockOverlay:
    def __init__(self, model, W, H, style, key):
        rng = rng_of(key, "overlay")
        self.model, self.style = model, style
        self.font = cv2.FONT_HERSHEY_DUPLEX
        thick = style["thickness"]
        target = max(8, int(round(style["height_frac"] * H)))
        (_, h1), _ = cv2.getTextSize("88:88:88", self.font, 1.0, thick)
        self.scale = target / float(h1)
        (tw, th), base = cv2.getTextSize("88:88:88", self.font, self.scale, thick)
        margin = int(round(24 * W / 1280.0))
        self.corner = ["tl", "tr", "bl", "br"][int(rng.integers(0, 4))]
        x = margin if self.corner in ("tl", "bl") else W - margin - tw
        y = margin + th if self.corner in ("tl", "tr") else H - margin - base
        self.org = (int(x), int(y))
        self.box = [int(x - 4), int(y - th - 4), int(tw + 8), int(th + base + 8)]
        label_y = y + th + base + 6 if self.corner in ("tl", "tr") else y - th - 10
        self.label = "CAM %02d  2026-03-%02d" % (int(rng.integers(1, 40)), int(rng.integers(1, 29)))
        (lw, _), _ = cv2.getTextSize(self.label, self.font, max(0.3, self.scale * 0.6), 1)
        label_x = x if self.corner in ("tl", "bl") else W - margin - lw
        self.label_org = (int(label_x), int(label_y))
        self.weekday = "THU" if (rng.random() < 0.5 and self.corner in ("tl", "bl")) else ""   # beside the time, so only where there is room to the right

    def draw(self, frame, t):
        text = self.model.text(t)
        st = self.style
        small = max(0.3, self.scale * 0.6)
        cv2.putText(frame, self.label, self.label_org, self.font, small, st["color"], 1, cv2.LINE_AA)
        if st["outline"]:
            cv2.putText(frame, text, self.org, self.font, self.scale, (0, 0, 0), st["thickness"] + 2, cv2.LINE_AA)
        cv2.putText(frame, text, self.org, self.font, self.scale, st["color"], st["thickness"], cv2.LINE_AA)
        if self.weekday:
            cv2.putText(frame, self.weekday, (self.org[0] + self.box[2] + 10, self.org[1]), self.font, small, st["color"], 1, cv2.LINE_AA)


def clock_condition_of(scene_id):
    return CLOCK_CONDITIONS[seed_of(scene_id, "clockcond") % 4]


# ---------------------------------------------------------------------------------------------------------------------------------
# encoding and files
# ---------------------------------------------------------------------------------------------------------------------------------
def ffmpeg_version():
    try:
        return subprocess.run(["ffmpeg", "-version"], capture_output=True, text=True).stdout.splitlines()[0]
    except Exception:  # pragma: no cover
        return "ffmpeg not found"


def encode(path, frames, W, H, fps, crf=23, bitrate=None, codec="libx264", audio_seconds=None, audio_codec="aac", extra_out=(),
           faststart=True, scale_to=None):
    cmd = ["ffmpeg", "-y", "-loglevel", "error", "-f", "rawvideo", "-pix_fmt", "bgr24", "-s", "%dx%d" % (W, H), "-framerate", str(fps), "-i", "-"]
    if audio_seconds:
        cmd += ["-f", "lavfi", "-i", "sine=frequency=440:sample_rate=16000:duration=%s" % audio_seconds]
    vf = []
    if scale_to:
        vf.append("scale=%d:%d" % scale_to)
    if vf:
        cmd += ["-vf", ",".join(vf)]
    cmd += ["-c:v", codec, "-pix_fmt", "yuv420p", "-threads", "1", "-g", "100"]
    if codec == "libx264":
        cmd += ["-preset", "veryfast"]
        cmd += ["-b:v", bitrate] if bitrate else ["-crf", str(crf)]
    else:
        cmd += ["-q:v", "5"]
    if audio_seconds:
        cmd += ["-c:a", audio_codec]   # the audio is made a little shorter than the video, so that the container duration is the video duration (no -shortest: it would cut the video)
    else:
        cmd += ["-an"]
    cmd += ["-fflags", "+bitexact", "-flags:v", "+bitexact"]
    if faststart and str(path).lower().endswith((".mp4", ".mov")):
        cmd += ["-movflags", "+faststart"]
    cmd += list(extra_out) + [str(path)]
    os.makedirs(os.path.dirname(path), exist_ok=True)
    p = subprocess.Popen(cmd, stdin=subprocess.PIPE, stderr=subprocess.PIPE)
    try:
        for f in frames:
            p.stdin.write(memoryview(np.ascontiguousarray(f)))
        p.stdin.close()
    except BrokenPipeError:
        pass
    err = p.stderr.read().decode("utf-8", "replace")
    if p.wait() != 0:
        raise RuntimeError("ffmpeg failed for %s: %s" % (path, err[-800:]))


def transcode(src, dst, vf=None, crf=None, bitrate=None, extra_in=(), extra_out=()):
    cmd = ["ffmpeg", "-y", "-loglevel", "error"] + list(extra_in) + ["-i", str(src)]
    if vf:
        cmd += ["-vf", vf]
    cmd += ["-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-threads", "1", "-an", "-fflags", "+bitexact", "-flags:v", "+bitexact"]
    cmd += ["-b:v", bitrate] if bitrate else ["-crf", str(crf or 23)]
    cmd += list(extra_out) + ["-movflags", "+faststart", str(dst)]
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode != 0:
        raise RuntimeError("ffmpeg failed for %s: %s" % (dst, r.stderr[-800:]))


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def write_json(path, obj):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        json.dump(obj, f, indent=1, sort_keys=True)
        f.write("\n")


def ffprobe(path):
    r = subprocess.run(["ffprobe", "-v", "error", "-show_format", "-show_streams", "-of", "json", str(path)], capture_output=True, text=True)
    if r.returncode != 0:
        return None, r.stderr.strip()
    return json.loads(r.stdout), ""


class Manifest:
    def __init__(self, out):
        self.out, self.files = out, []

    def add(self, path):
        self.files.append({"path": os.path.relpath(path, self.out).replace("\\", "/"), "bytes": os.path.getsize(path), "sha256": sha256_file(path)})

    def save(self, tag):
        write_json(os.path.join(self.out, "manifest-%s.json" % tag),
                   {"generator": GENERATOR_VERSION, "ffmpeg": ffmpeg_version(), "created_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                    "files": sorted(self.files, key=lambda f: f["path"])})


# ---------------------------------------------------------------------------------------------------------------------------------
# the parts
# ---------------------------------------------------------------------------------------------------------------------------------
def build_motion(scene_id, out, man, variant="a", name="motion"):
    shot = motion_shot(scene_id, variant)
    base = os.path.join(out, scene_id)
    path = os.path.join(base, "%s.mp4" % name)
    encode(path, (shot.frame(k) for k in range(shot.meta["n_frames"])), shot.W, shot.H, MOTION_FPS, crf=23)
    truth = motion_truth(scene_id, shot)
    truth["variant"] = variant
    write_json(os.path.join(base, "%s.truth.json" % name), truth)
    man.add(path)
    return truth


def build_cuts(scene_id, out, man):
    plan = plan_sequence(scene_id)
    fx, cut_truth = sequence_effects(plan)
    metas = []

    def frames():
        # the shot objects are built on demand inside sequence_frames; their meta is read afterwards by rebuilding the cheap parts
        for f in sequence_frames(scene_id, plan, fx):
            yield f

    base = os.path.join(out, scene_id)
    path = os.path.join(base, "cuts.mp4")
    encode(path, frames(), MOTION_W, MOTION_H, MOTION_FPS, crf=23)
    fam = None
    for j in range(len(plan["lens"])):
        s = sequence_shot(scene_id, j, plan, fam)
        fam = s.meta["family"]
        metas.append(s.meta)
    write_json(os.path.join(base, "cuts.truth.json"), sequence_truth(scene_id, plan, cut_truth, metas))
    man.add(path)


def build_clock(scene_id, out, man, minutes):
    style_name = clock_condition_of(scene_id)
    style = CLOCK_STYLES[style_name]
    seconds = int(minutes * 60)
    model = ClockModel(scene_id, seconds)
    rng = rng_of(scene_id, "clock", "bg")
    shot = Shot(CLOCK_W, CLOCK_H, 1, style["condition"], (scene_id, "clockbg"), (scene_id, "clock"), style["sigma"], bank=48)
    overlay = ClockOverlay(model, CLOCK_W, CLOCK_H, style, scene_id)
    base = os.path.join(out, scene_id)
    path = os.path.join(base, "clock.mp4")

    def frames():
        for t in range(seconds):
            f = shot.frame(t)
            overlay.draw(f, t)
            yield f

    encode(path, frames(), CLOCK_W, CLOCK_H, 1, crf=style["crf"], extra_out=["-metadata", "creation_time=2026-03-14T00:00:00Z"])
    truth = {"id": scene_id, "kind": "clock", "partition": partition_of(scene_id), "generator": GENERATOR_VERSION, "width": CLOCK_W, "height": CLOCK_H,
             "fps": 1, "frames": seconds, "duration_s": seconds, "condition": style_name, "crf": style["crf"], "clock_box_xywh": overlay.box,
             "clock_corner": overlay.corner, "container_start_utc_note": "container creation_time is the true start; the shown clock starts at start_time_of_day_s plus the initial offset"}
    truth.update(model.truth(seconds))
    write_json(os.path.join(base, "clock.truth.json"), truth)
    man.add(path)


def build_signature(scene_id, out, man):
    base = os.path.join(out, scene_id)
    src = os.path.join(base, "motion.mp4")
    if not os.path.exists(src):
        build_motion(scene_id, out, man)
    sig = os.path.join(base, "signature")
    os.makedirs(sig, exist_ok=True)
    # the three bitrates are fractions of the source's own bitrate, so that every re-encode really is a lossy copy (a fixed number can be above the rate of a quiet source)
    src_kbps = max(80.0, os.path.getsize(src) * 8 / 1000.0 / MOTION_SECONDS)
    rates = {name: int(max(40, src_kbps * f)) for name, f in (("low", 0.15), ("mid", 0.35), ("high", 0.70))}
    variants = [("reenc_low", dict(bitrate="%dk" % rates["low"], extra_out=["-maxrate", "%dk" % int(rates["low"] * 1.3), "-bufsize", "%dk" % (rates["low"] * 2)]),
                 "re-encode at 15%% of the source bitrate (%d kbit/s)" % rates["low"]),
                ("reenc_mid", dict(bitrate="%dk" % rates["mid"], extra_out=["-maxrate", "%dk" % int(rates["mid"] * 1.3), "-bufsize", "%dk" % (rates["mid"] * 2)]),
                 "re-encode at 35%% of the source bitrate (%d kbit/s)" % rates["mid"]),
                ("reenc_high", dict(bitrate="%dk" % rates["high"], extra_out=["-maxrate", "%dk" % int(rates["high"] * 1.3), "-bufsize", "%dk" % (rates["high"] * 2)]),
                 "re-encode at 70%% of the source bitrate (%d kbit/s)" % rates["high"]),
                ("crop5", dict(vf="crop=trunc(iw*0.95/2)*2:trunc(ih*0.95/2)*2,scale=%d:%d" % (MOTION_W, MOTION_H), crf=23), "5% of width and height cropped (2.5% per edge), scaled back"),
                ("mirror", dict(vf="hflip", crf=23), "mirrored left to right"),
                ("trim", dict(extra_in=["-ss", "20"], extra_out=["-t", "120"], crf=23), "20 s to 140 s of the clip (extra, reported, not gated)")]
    entries = []
    for name, kw, note in variants:
        dst = os.path.join(sig, "%s.mp4" % name)
        transcode(src, dst, **kw)
        man.add(dst)
        entries.append({"file": "signature/%s.mp4" % name, "relation": "same_footage", "source": "motion.mp4", "transform": name, "note": note,
                        "gated": name != "trim"})
    for variant, note in (("h1", "same camera view recorded again, same light, other events and noise"),
                          ("h2", "same camera view at another time of day (different shading and level)")):
        t = build_motion(scene_id, os.path.join(out, scene_id, "signature_tmp"), man, variant=variant, name=variant)
        tmp = os.path.join(out, scene_id, "signature_tmp", scene_id)
        for ext in ("mp4", "truth.json"):
            dst = os.path.join(sig, "%s.%s" % (variant, ext))
            os.replace(os.path.join(tmp, "%s.%s" % (variant, ext)), dst)
        man.files = [f for f in man.files if "signature_tmp" not in f["path"]]
        man.add(os.path.join(sig, "%s.mp4" % variant))
        entries.append({"file": "signature/%s.mp4" % variant, "relation": "different_footage", "source": "motion.mp4", "transform": variant,
                        "note": note, "hard_negative": True, "gated": True})
    write_json(os.path.join(base, "signature.truth.json"), {"id": scene_id, "partition": partition_of(scene_id), "generator": GENERATOR_VERSION,
                                                           "source": "motion.mp4", "entries": entries})
    for d in ("signature_tmp",):
        try:
            import shutil
            shutil.rmtree(os.path.join(base, d), ignore_errors=True)
        except Exception:  # pragma: no cover
            pass


PROBE_FORMATS = [("mp4", "libx264", "aac"), ("mkv", "libx264", "aac"), ("avi", "mpeg4", "pcm_s16le"), ("mov", "libx264", "aac")]
PROBE_FPS = [(5, 1), (10, 1), (25, 2), (15, 1), (24, 1), (25, 1), (30000, 1001), (30, 1)]
PROBE_SIZES = [(320, 240), (640, 360), (1280, 720), (1920, 1080)]


def probe_frames(i, W, H, n, fps_f):
    rw, rh = (W, H) if W <= 640 else (640, 360)
    shot = Shot(rw, rh, fps_f, "day", ("probe", i), ("probe", i), 1.5, bank=8)
    bw, bh = max(8, rw // 8), max(8, rh // 6)
    for k in range(n):
        f = shot.frame(k)
        x = (k * 3) % max(1, rw - bw)
        cv2.rectangle(f, (x, rh // 3), (x + bw, rh // 3 + bh), (40, 200, 255), -1)
        cv2.putText(f, str(k), (8, rh - 12), cv2.FONT_HERSHEY_DUPLEX, 0.7, (255, 255, 255), 1, cv2.LINE_AA)
        yield f


def build_probe(out, man):
    base = os.path.join(out, "probe")
    os.makedirs(base, exist_ok=True)
    rows = []
    for i in range(1, 101):
        rng = rng_of("probe", i)
        ext, vcodec, acodec = PROBE_FORMATS[(i - 1) % 4]
        fn, fd = PROBE_FPS[int(rng.integers(0, len(PROBE_FPS)))]
        W, H = PROBE_SIZES[int(rng.integers(0, len(PROBE_SIZES)))]
        dur = float(rng.uniform(2, 10 if W >= 1920 else 30))
        n = max(10, int(round(dur * fn / fd)))
        has_audio = bool(rng.random() < 0.4)
        fps_arg = "%d/%d" % (fn, fd)
        path = os.path.join(base, "probe-%04d.%s" % (i, ext))
        encode(path, probe_frames(i, W, H, n, fn / float(fd)), W if W <= 640 else 640, H if W <= 640 else 360, fps_arg, codec=vcodec,
               audio_seconds=(max(0.3, n * fd / float(fn) - 0.15) if has_audio else None), audio_codec=acodec, scale_to=((W, H) if W > 640 else None))
        rows.append({"file": "probe/probe-%04d.%s" % (i, ext), "readable": True, "container": ext, "video_codec": "h264" if vcodec == "libx264" else "mpeg4",
                     "width": W, "height": H, "fps_num": fn, "fps_den": fd, "frames": n, "duration_s": n * fd / float(fn), "has_audio": has_audio})
        man.add(path)
    # the 20 planted bad files: 5 of each kind
    good = [r for r in rows if r["container"] == "mp4" and not r["has_audio"]][:5]
    for n_, kind in enumerate(["truncated", "empty", "not_video", "corrupt_header"]):
        for m in range(5):
            idx = n_ * 5 + m + 1
            ext = ["mp4", "mkv", "avi", "mov", "mp4"][m]
            path = os.path.join(base, "bad-%02d-%s.%s" % (idx, kind, ext))
            if kind == "truncated":
                tmp = os.path.join(base, "_tmp_%d.mp4" % idx)
                encode(tmp, probe_frames(900 + idx, 320, 240, 150, 10.0), 320, 240, "10/1", faststart=False)
                data = open(tmp, "rb").read()
                os.remove(tmp)
                path = os.path.join(base, "bad-%02d-%s.mp4" % (idx, kind))
                open(path, "wb").write(data[: int(len(data) * 0.55)])
            elif kind == "empty":
                open(path, "wb").close()
            elif kind == "not_video":
                r = rng_of("probe", "bad", idx)
                open(path, "wb").write(bytes(r.integers(0, 256, 4096, dtype=np.uint8)) if m % 2 == 0 else b"This is not a video file, it only has a video file name.\n" * 20)
            else:
                src = os.path.join(base, good[m]["file"].split("/")[-1])
                data = bytearray(open(src, "rb").read())
                data[:256] = b"\x00" * 256
                open(path, "wb").write(bytes(data))
            rows.append({"file": "probe/" + os.path.basename(path), "readable": False, "bad_kind": kind})
            man.add(path)
    write_json(os.path.join(out, "probe.truth.json"), {"generator": GENERATOR_VERSION, "files": rows, "good": 100, "bad": 20})


def build_budget(out, man, minutes=60):
    """One hour of 1280x720 at 10 fps: motion, 5 hard cuts, a burned-in clock (MF1-B, dev only)."""
    W, H, fps = CLOCK_W, CLOCK_H, 10
    seconds = int(minutes * 60)
    total = seconds * fps
    key = "mf1-budget"
    rng = rng_of(key, "plan")
    n_sh = 6
    lens = [total // n_sh] * n_sh
    lens[-1] += total - sum(lens)
    starts = [sum(lens[:i]) for i in range(n_sh)]
    model = ClockModel(key, seconds, jumps=2)
    style = CLOCK_STYLES["clean_day"]
    overlay = ClockOverlay(model, W, H, style, key)
    passes_truth, cut_frames = [], starts[1:]

    def frames():
        for j in range(n_sh):
            n = lens[j] + 5
            condition = ["day", "day", "dusk", "day", "night", "day"][j]
            r = rng_of(key, "shot", j)
            sigma = sigma_of(r, condition)
            probe_bg = make_background(rng_of(key, "family", j, "bg"), W, H, condition)
            ivs = plan_intervals(r, fps, n, max(1, int(n / fps / 60 * 1.2)), lo_s=3.0, hi_s=40.0, gap_s=4.0, margin_s=2.0)
            ps = build_passes(r, W, H, fps, probe_bg, condition, ivs)
            shot = Shot(W, H, fps, condition, (key, "family", j), (key, "noise", j), sigma, ps, light_amp=float(r.uniform(0, 3)),
                        light_period_s=90.0, bank=24)
            for p in ps:
                passes_truth.append({"start_frame": starts[j] + p.core[0], "end_frame": starts[j] + p.core[1]})
            for k in range(lens[j]):
                f = shot.frame(k)
                overlay.draw(f, (starts[j] + k) / float(fps))
                yield f
            del shot

    base = os.path.join(out, "budget")
    path = os.path.join(base, "budget.mp4")
    encode(path, frames(), W, H, fps, crf=23)
    passes_truth = [p for p in passes_truth if p["start_frame"] < total]
    truth = {"id": key, "kind": "budget", "generator": GENERATOR_VERSION, "width": W, "height": H, "fps": fps, "frames": total, "duration_s": seconds,
             "hard_cut_frames": cut_frames, "intervals": sorted(passes_truth, key=lambda d: d["start_frame"]), "clock_box_xywh": overlay.box,
             "clock": {k: v for k, v in model.truth(seconds).items() if k != "minute_samples"}}
    write_json(os.path.join(base, "budget.truth.json"), truth)
    man.add(path)


# ---------------------------------------------------------------------------------------------------------------------------------
# self check and command line
# ---------------------------------------------------------------------------------------------------------------------------------
def verify_outputs(out, ids, parts):
    """Compare what ffprobe says about the files with the truth files (a check of the DATA, not of the system under test)."""
    problems = 0
    for sid in ids:
        for part, stem, want_w, want_h in (("motion", "motion", MOTION_W, MOTION_H), ("cuts", "cuts", MOTION_W, MOTION_H), ("clock", "clock", CLOCK_W, CLOCK_H)):
            if part not in parts:
                continue
            tp = os.path.join(out, sid, stem + ".truth.json")
            vp = os.path.join(out, sid, stem + ".mp4")
            if not (os.path.exists(tp) and os.path.exists(vp)):
                print("MISSING", sid, stem)
                problems += 1
                continue
            truth = json.load(open(tp))
            info, err = ffprobe(vp)
            v = next((s for s in (info or {}).get("streams", []) if s["codec_type"] == "video"), None)
            if not v or int(v["width"]) != want_w or int(v["height"]) != want_h or abs(int(v.get("nb_frames", -1)) - truth["frames"]) > 0:
                print("MISMATCH", sid, stem, v and (v["width"], v["height"], v.get("nb_frames")), truth["frames"])
                problems += 1
    if "probe" in parts:
        truth = json.load(open(os.path.join(out, "probe.truth.json")))
        for row in truth["files"]:
            info, err = ffprobe(os.path.join(out, row["file"]))
            if not row["readable"]:
                if info is not None and any(s["codec_type"] == "video" for s in info.get("streams", [])):
                    print("BAD FILE READABLE", row["file"], row["bad_kind"])
                    problems += 1
                continue
            v = next((s for s in (info or {}).get("streams", []) if s["codec_type"] == "video"), None)
            a = any(s["codec_type"] == "audio" for s in (info or {}).get("streams", []))
            if not v:
                print("UNREADABLE GOOD", row["file"], err[:80])
                problems += 1
                continue
            dur = float(info["format"].get("duration", 0))
            one_frame = row["fps_den"] / float(row["fps_num"])
            if (int(v["width"]), int(v["height"])) != (row["width"], row["height"]) or a != row["has_audio"] or abs(dur - row["duration_s"]) > one_frame:
                print("PROBE MISMATCH", row["file"], (v["width"], v["height"]), a, dur, row["duration_s"])
                problems += 1
    print("verify:", "OK" if problems == 0 else "%d PROBLEMS" % problems)
    return problems


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--out", default="/out")
    ap.add_argument("--parts", default="motion", help="comma list of motion,cuts,clock,signature,probe,budget or all")
    ap.add_argument("--ids", default="", help="comma list of scene ids")
    ap.add_argument("--partition", choices=["dev", "sealed"], default=None)
    ap.add_argument("--limit", type=int, default=0)
    ap.add_argument("--clock-minutes", type=float, default=120.0)
    ap.add_argument("--budget-minutes", type=float, default=60.0)
    ap.add_argument("--allow-sealed", action="store_true", help="required for any sealed scene: sealed scenes are generated only for the sealed run")
    ap.add_argument("--verify", action="store_true")
    ap.add_argument("--list", action="store_true")
    ap.add_argument("--tag", default=None)
    a = ap.parse_args(argv)
    if a.list:
        dev = [i for i in SCENE_IDS if partition_of(i) == "dev"]
        print("scenes %d: dev %d, sealed %d" % (len(SCENE_IDS), len(dev), len(SCENE_IDS) - len(dev)))
        if a.partition:
            print(" ".join(i for i in SCENE_IDS if partition_of(i) == a.partition))
        return 0
    parts = {"motion", "cuts", "clock", "signature", "probe", "budget"} if a.parts == "all" else set(p.strip() for p in a.parts.split(",") if p.strip())
    ids = [i for i in a.ids.split(",") if i] or [i for i in SCENE_IDS if a.partition is None or partition_of(i) == a.partition]
    if a.limit:
        ids = ids[:a.limit]
    sealed = [i for i in ids if partition_of(i) == "sealed"]
    per_scene = parts & {"motion", "cuts", "clock", "signature"}
    if sealed and per_scene and not a.allow_sealed:
        print("Refusing: %d sealed scenes selected (%s ...). Sealed scenes are generated only for the sealed run: pass --allow-sealed." % (len(sealed), sealed[0]))
        return 2
    os.makedirs(a.out, exist_ok=True)
    man = Manifest(a.out)
    t0 = time.time()
    for sid in (ids if per_scene else []):
        print("%s [%s] %s" % (sid, partition_of(sid), ",".join(sorted(per_scene))), flush=True)
        if "motion" in parts:
            build_motion(sid, a.out, man)
        if "cuts" in parts:
            build_cuts(sid, a.out, man)
        if "clock" in parts:
            build_clock(sid, a.out, man, a.clock_minutes)
        if "signature" in parts:
            build_signature(sid, a.out, man)
    if "probe" in parts:
        build_probe(a.out, man)
    if "budget" in parts:
        build_budget(a.out, man, a.budget_minutes)
    man.save(a.tag or a.partition or "custom")
    print("done in %.0f s, %d files" % (time.time() - t0, len(man.files)))
    if a.verify:
        return 1 if verify_outputs(a.out, ids, parts) else 0
    return 0


if __name__ == "__main__":
    sys.exit(main())
