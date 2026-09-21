#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Consume the frozen V2.2 sealed-reserved evaluation exactly once."""
from __future__ import annotations

import argparse
import hashlib
import json
import subprocess
import sys
import threading
import time
from datetime import datetime, timezone
from pathlib import Path

from run_nxmmr_video_v2_exact_runtime import ROOT, RUNTIME, BASE, PERSONAL, available_ram_gib

RESULTS = ROOT / "local-acceptance-models/nxmmr/private-benchmarks/video-v22-reserved-20260831"
ORACLE_ROOT = ROOT / "local-acceptance-models/nxmmr/private-ground-truth/human-verification"
SPLIT = ROOT / "local-acceptance-models/nxmmr/private-benchmarks/reference-parity-adapter-v1/video-development-reserved-split.json"
FREEZE_DIGEST = "b0caae6d646e9d188ec5bc387585ac43aa7f1e22504d5b2d741699bab8d9f394"
AUTHORIZATION_SHA256 = "47be79708176cbb2caa1a7d9db0154e6f0ea0c0ad004b8c5d5273aa24f831626"


def canonical_digest(value):
    encoded = json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False)
    return hashlib.sha256(encoded.encode()).hexdigest()


def write_signed(path, value):
    value["receipt_digest"] = canonical_digest(value)
    with path.open("x", encoding="utf-8") as stream:
        json.dump(value, stream, indent=2)
        stream.write("\n")


def read_signed(path):
    value = json.loads(path.read_text())
    digest = value.pop("receipt_digest")
    assert canonical_digest(value) == digest
    return value


def run():
    RESULTS.mkdir(parents=True, exist_ok=True)
    registration_path = RESULTS / "registration.json"
    registration = read_signed(registration_path)
    assert registration["authorization_sha256"] == AUTHORIZATION_SHA256
    assert registration["candidate_freeze_digest"] == FREEZE_DIGEST
    assert registration["reserved_evaluation_limit"] == 1
    for marker in ("consumption-start.json", "consumption.json", "reserved.json",
                   "reserved-oracle.csv", "reserved-oracle-projection.json"):
        if (RESULTS / marker).exists():
            raise RuntimeError("reserved authorization already consumed or attempted; repeat forbidden")
    pre = available_ram_gib()
    if pre < 3:
        raise RuntimeError(f"3-GiB evaluator admission failed before consumption: {pre:.6f}")
    assert json.loads((RUNTIME / "pip-check.json").read_text())["exit_code"] == 0
    start_receipt = {
        "contract_version": "nexusai.nxmmr.video-v22-reserved-consumption-start/v1",
        "recorded_at": datetime.now(timezone.utc).isoformat(),
        "authorization_sha256": AUTHORIZATION_SHA256,
        "candidate_freeze_digest": FREEZE_DIGEST,
        "selected_mode": "RAW_RGB",
        "authorization_consumed": True,
        "reserved_evaluation_count": 1,
        "repeat_permitted": False,
        "post_hoc_tuning": False,
    }
    write_signed(RESULTS / "consumption-start.json", start_receipt)
    projection_command = [sys.executable, "-B", str(ROOT / "scripts/project_nxmmr_video_v22_reserved_oracle.py"),
        "--oracle-root", str(ORACLE_ROOT), "--split", str(SPLIT),
        "--consumption-start", str(RESULTS / "consumption-start.json"),
        "--output", str(RESULTS / "reserved-oracle.csv"),
        "--receipt", str(RESULTS / "reserved-oracle-projection.json"),
        "--expected-freeze-digest", FREEZE_DIGEST]
    process, code, stopped = None, None, False
    readings, started = [pre], time.perf_counter()
    name = "nxmmr-video-v22-reserved-once"
    final_state, error = "CONSUMED_FAILED", None
    try:
        projected = subprocess.run(projection_command, cwd=ROOT, check=True,
                                   capture_output=True, text=True)
        print(projected.stdout.strip(), flush=True)
        command = ["docker", "run", "--rm", "--pull", "never", "--name", name,
            "--network", "none", "--read-only", "--user", "0", "--cpus", "4", "--memory", "3g",
            "--pids-limit", "256", "--tmpfs", "/tmp:rw,nosuid,size=256m",
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
            "--entrypoint", "/evaluator/venv/bin/python", BASE, "-B",
            "/repo/scripts/evaluate_nxmmr_video_v22_reserved.py",
            "--registration", "/results/registration.json",
            "--output", "/results/reserved.json",
            "--oracle", "/results/reserved-oracle.csv",
            "--projection-receipt", "/results/reserved-oracle-projection.json",
            "--consumption-start", "/results/consumption-start.json",
            "--split", "/repo/local-acceptance-models/nxmmr/private-benchmarks/reference-parity-adapter-v1/video-development-reserved-split.json",
            "--video", "/assets/sample.mp4"]
        process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                   text=True, encoding="utf-8", errors="replace")
        def drain():
            with (RESULTS / "reserved.log").open("x", encoding="utf-8") as log:
                for line in process.stdout:
                    log.write(line)
                    log.flush()
                    print(line.rstrip(), flush=True)
        reader = threading.Thread(target=drain, daemon=True)
        reader.start()
        while process.poll() is None:
            available = available_ram_gib()
            readings.append(available)
            if available <= 1.5 or time.perf_counter() - started > 900:
                (RESULTS / "HOST_RAM_STOP").write_text("Host resource safety stop\n")
                subprocess.run(["docker", "stop", "--time", "2", name], check=False,
                               capture_output=True)
                stopped = True
                break
            time.sleep(.2)
        code = process.wait(timeout=20)
        reader.join(timeout=5)
        if code or stopped:
            raise RuntimeError("one-time reserved evaluator failed; repeat forbidden")
        final_state = "CONSUMED_SCORED"
    except Exception as exc:
        error = {"type": type(exc).__name__, "message": str(exc)}
        raise
    finally:
        host = {"phase": "reserved", "candidate_freeze_digest": FREEZE_DIGEST,
            "pre_run_available_ram_gib": pre, "minimum_available_ram_gib": min(readings),
            "final_available_ram_gib": available_ram_gib(), "sample_interval_seconds": .2,
            "sample_count": len(readings), "wall_seconds_including_projection_and_container": time.perf_counter() - started,
            "safety_stop": stopped, "container_exit_code": None if process is None else process.poll(),
            "network": "none", "owned_container": name, "base_image": BASE,
            "runtime_lock_sha256": hashlib.sha256((RUNTIME / "resolved-runtime-lock.json").read_bytes()).hexdigest()}
        write_signed(RESULTS / "reserved-host-resources.json", host)
        output_sha = hashlib.sha256((RESULTS / "reserved.json").read_bytes()).hexdigest() if (RESULTS / "reserved.json").exists() else None
        write_signed(RESULTS / "consumption.json", {
            "contract_version": "nexusai.nxmmr.video-v22-reserved-consumption/v1",
            "candidate_freeze_digest": FREEZE_DIGEST,
            "authorization_sha256": AUTHORIZATION_SHA256,
            "state": final_state, "error": error,
            "reserved_evaluation_count": 1, "one_time_authorization_consumed": True,
            "scoring_completed": final_state == "CONSUMED_SCORED",
            "reserved_receipt_sha256": output_sha,
            "repeat_permitted": False, "post_hoc_tuning": False,
        })
    print(json.dumps({"state": final_state, "reserved_evaluation_count": 1,
                      "safety_stop": stopped, "container_exit_code": code}), flush=True)


if __name__ == "__main__":
    argparse.ArgumentParser(description=__doc__).parse_args()
    run()
