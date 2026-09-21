#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Windows host safety monitor for one owned, network-disabled evaluator."""
from __future__ import annotations

import argparse
import ctypes
import hashlib
import json
import subprocess
import threading
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
RUNTIME = ROOT / "local-acceptance-models/nxmmr/private-runtime/video-v2-cp311-20260831"
RESULTS = ROOT / "local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/exact-runtime"
BASE = "sha256:be13dbec4bd9bb62c824140df3777b898d7ad2801a0e68cd512c00dafaf4f9ef"
PERSONAL = Path("C:/Users/sheik/Workspace/Personal/Vehicle-License-Plate-Detection/src")
CANDIDATE = "v2"


def select_candidate(candidate):
    global CANDIDATE, RESULTS
    CANDIDATE = candidate
    if candidate == "v21":
        RESULTS = ROOT / "local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/v21-development"


def available_ram_gib():
    class MemoryStatus(ctypes.Structure):
        _fields_ = [("length", ctypes.c_ulong), ("load", ctypes.c_ulong)] + [
            (name, ctypes.c_ulonglong) for name in ("total_phys", "avail_phys", "total_page", "avail_page", "total_virtual", "avail_virtual", "extended")]
    status = MemoryStatus()
    status.length = ctypes.sizeof(status)
    if not ctypes.windll.kernel32.GlobalMemoryStatusEx(ctypes.byref(status)):
        raise OSError("host physical memory query failed")
    return status.avail_phys / 2**30


def run(phase):
    if CANDIDATE == "v21" and phase == "reserved":
        raise RuntimeError("V2.1 development authorization forbids reserved evaluation")
    RESULTS.mkdir(parents=True, exist_ok=True)
    output = RESULTS / f"{phase}.json"
    host_receipt = RESULTS / f"{phase}-host-resources.json"
    if output.exists() or host_receipt.exists():
        raise RuntimeError("refusing to repeat or overwrite this phase")
    if phase in {"smoke", "development", "reserved"}:
        assert json.loads((RESULTS / "imports.json").read_text())["state"] == "PASS"
    if CANDIDATE == "v21" and phase in {"smoke", "development"}:
        assert json.loads((RESULTS / "contracts-host-resources.json").read_text())["container_exit_code"] == 0
    if phase in {"development", "reserved"}:
        assert json.loads((RESULTS / "smoke.json").read_text())["state"] == "PASS"
    pre = available_ram_gib()
    if pre < 3.0:
        raise RuntimeError(f"3 GiB evaluator admission failed: {pre:.6f}")
    if phase == "reserved":
        dev = json.loads((RESULTS / "development.json").read_text())
        admission = json.loads((RESULTS / "development-admission.json").read_text())
        assert admission["development_gate"] == "PASS" and dev["numeric_utility_gate"] == "PASS"
        assert admission["candidate_freeze"] == dev["candidate_freeze"]
        dev_host = json.loads((RESULTS / "development-host-resources.json").read_text())
        assert not dev_host["safety_stop"]
        measured_requirement = dev["resources"]["incremental_tree_peak_mib"] / 1024 + 1.5
        assert pre >= measured_requirement
        assert not (RESULTS / "reserved.consumed").exists()
    assert json.loads((RUNTIME / "pip-check.json").read_text())["exit_code"] == 0
    name = "nxmmr-" + CANDIDATE + "-exact-" + phase + "-20260831"
    command = ["docker", "run", "--rm", "--pull", "never", "--name", name,
       "--network", "none", "--read-only", "--user", "0", "--cpus", "4",
       "--memory", "3g", "--pids-limit", "256", "--tmpfs", "/tmp:rw,nosuid,size=256m",
       "--env", "PYTHONDONTWRITEBYTECODE=1", "--env", "PYTHONUNBUFFERED=1",
       "--env", "MPLCONFIGDIR=/tmp/matplotlib", "--env", "YOLO_CONFIG_DIR=/tmp/ultralytics",
       "--env", "HF_HUB_OFFLINE=1", "--env", "TRANSFORMERS_OFFLINE=1",
       "--env", "FORENSIC_VIDEO_ANPR_PLATE_DETECTOR_PATH=/assets/license_plate_detector.pt",
       "--env", "FORENSIC_VIDEO_ANPR_VEHICLE_DETECTOR_PATH=/assets/yolov8n.pt",
       "--env", "FORENSIC_ANPR_OCR_MODEL_PATH=/repo/local-acceptance-models/nxmmr/cct_xs_v2_global.onnx",
       "--env", "FORENSIC_ANPR_OCR_CONFIG_PATH=/repo/local-acceptance-models/nxmmr/cct_xs_v2_global_plate_config.yaml",
       "--mount", f"type=bind,src={ROOT},dst=/repo,readonly",
       "--mount", f"type=bind,src={RUNTIME},dst=/evaluator,readonly",
       "--mount", f"type=bind,src={RESULTS},dst=/results",
       "--mount", f"type=bind,src={PERSONAL / 'license_plate_detector.pt'},dst=/assets/license_plate_detector.pt,readonly",
       "--mount", f"type=bind,src={PERSONAL / 'yolov8n.pt'},dst=/assets/yolov8n.pt,readonly",
       "--mount", f"type=bind,src={PERSONAL / 'sample.mp4'},dst=/assets/sample.mp4,readonly",
       "--entrypoint", "/evaluator/venv/bin/python", BASE,
       "-B", "/repo/scripts/evaluate_nxmmr_video_v2_exact_runtime.py", phase,
       "--freeze", "/repo/local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/video-v2-product-source-freeze.json",
       "--manifest", "/repo/configuration/nxmmr_anpr_ocr_vertical_activation_v1.json",
       "--output", f"/results/{phase}.json"]
    if CANDIDATE == "v21":
        command += ["--candidate", "v21", "--evaluation-manifest", "/results/development-input-manifest.json"]
    if phase == "contracts":
        command = command[:command.index(BASE) + 1] + [
            "-B", "/repo/scripts/test_nxmmr_video_v21_contracts.py", "-v"]
    if phase in {"development", "reserved"}:
        assert (RESULTS / f"{phase}-oracle.csv").is_file()
        command += ["--oracle", f"/results/{phase}-oracle.csv", "--video", "/assets/sample.mp4",
                    "--split", "/repo/local-acceptance-models/nxmmr/private-benchmarks/reference-parity-adapter-v1/video-development-reserved-split.json"]
    if phase == "reserved":
        command += ["--development-receipt", "/results/development-admission.json"]
    started = time.perf_counter()
    process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                               text=True, encoding="utf-8", errors="replace")
    def drain():
        with (RESULTS / f"{phase}.log").open("x", encoding="utf-8") as log:
            for line in process.stdout:
                log.write(line)
                log.flush()
                print(line.rstrip(), flush=True)
    reader = threading.Thread(target=drain, daemon=True)
    reader.start()
    readings, stopped = [pre], False
    try:
        while process.poll() is None:
            available = available_ram_gib()
            readings.append(available)
            if available <= 1.5 or time.perf_counter() - started > 900:
                (RESULTS / "HOST_RAM_STOP").write_text("Host resource safety stop\n")
                subprocess.run(["docker", "stop", "--time", "2", name], check=False, capture_output=True)
                stopped = True
                break
            time.sleep(.2)
        code = process.wait(timeout=20)
        reader.join(timeout=5)
    finally:
        value = {"phase": phase, "pre_run_available_ram_gib": pre,
          "minimum_available_ram_gib": min(readings), "final_available_ram_gib": available_ram_gib(),
          "sample_count": len(readings), "sample_interval_seconds": .2,
          "wall_seconds_including_container_start": time.perf_counter() - started,
          "safety_stop": stopped, "container_exit_code": process.poll(),
          "network": "none", "owned_container": name, "base_image": BASE,
          "runtime_lock_sha256": hashlib.sha256((RUNTIME / "resolved-runtime-lock.json").read_bytes()).hexdigest()}
        with host_receipt.open("x") as stream:
            json.dump(value, stream, indent=2)
            stream.write("\n")
    print(json.dumps(value), flush=True)
    if code or stopped:
        raise RuntimeError(f"{phase} admission/evaluation failed before further phases")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=["imports", "contracts", "smoke", "development", "reserved"])
    parser.add_argument("--candidate", choices=["v2", "v21"], default="v2")
    args = parser.parse_args()
    select_candidate(args.candidate)
    run(args.phase)
