# SPDX-License-Identifier: MIT
"""One owned offline evaluator at a time; raw-color development only."""
import argparse
import hashlib
import json
import subprocess
import threading
import time
from pathlib import Path

from run_nxmmr_video_v2_exact_runtime import ROOT, RUNTIME, BASE, PERSONAL, available_ram_gib

RESULTS = ROOT / "local-acceptance-models/nxmmr/private-benchmarks/raw-color-parity-20260831"
MODES = ("RAW_BGR_PARITY", "RAW_RGB")


def run(phase, mode):
    if phase not in ("contracts", "imports", "smoke", "development") or mode not in MODES:
        raise RuntimeError("Only the two authorized development arms are permitted; no reserved mode")
    folder = RESULTS if phase in ("contracts", "imports") else RESULTS / mode
    folder.mkdir(parents=True, exist_ok=True)
    host_receipt = folder / (phase + "-host-resources.json")
    if host_receipt.exists() or (folder / (phase + ".json")).exists():
        raise RuntimeError("refusing repeated phase")
    if phase in ("smoke", "development"):
        assert json.loads((RESULTS / "imports.json").read_text())["state"] == "PASS"
        assert json.loads((RESULTS / "contracts-host-resources.json").read_text())["container_exit_code"] == 0
    if phase == "development":
        for arm in MODES:
            assert json.loads((RESULTS / arm / "smoke.json").read_text())["state"] == "PASS"
            assert json.loads((RESULTS / arm / "smoke-process-resources.json").read_text())["state"] == "PASS"
    pre = available_ram_gib()
    if pre < 3:
        raise RuntimeError(f"3-GiB admission failed: {pre:.6f}")
    assert json.loads((RUNTIME / "pip-check.json").read_text())["exit_code"] == 0
    name = "nxmmr-raw-color-" + mode.lower().replace("_", "-") + "-" + phase
    relative = "" if folder == RESULTS else mode + "/"
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
        "--entrypoint", "/evaluator/venv/bin/python", BASE, "-B"]
    if phase == "contracts":
        command += ["/repo/scripts/test_nxmmr_raw_color.py"]
    else:
        command += ["/repo/scripts/evaluate_nxmmr_raw_color.py", phase, "--mode", mode,
            "--registration", "/results/registration.json", "--output", f"/results/{relative}{phase}.json"]
        if phase == "development":
            command += ["--video", "/assets/sample.mp4", "--oracle",
                "/repo/local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/v21-development/development-oracle.csv",
                "--split", "/repo/local-acceptance-models/nxmmr/private-benchmarks/reference-parity-adapter-v1/video-development-reserved-split.json"]
    started = time.perf_counter()
    process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                               text=True, encoding="utf-8", errors="replace")
    def drain():
        with (folder / (phase + ".log")).open("x", encoding="utf-8") as log:
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
        value = {"phase": phase, "mode": mode, "pre_run_available_ram_gib": pre,
            "minimum_available_ram_gib": min(readings), "final_available_ram_gib": available_ram_gib(),
            "sample_interval_seconds": .2, "sample_count": len(readings),
            "wall_seconds_including_container_start": time.perf_counter() - started,
            "safety_stop": stopped, "container_exit_code": process.poll(), "network": "none",
            "owned_container": name, "base_image": BASE,
            "runtime_lock_sha256": hashlib.sha256((RUNTIME / "resolved-runtime-lock.json").read_bytes()).hexdigest()}
        with host_receipt.open("x", encoding="utf-8") as stream:
            json.dump(value, stream, indent=2)
            stream.write("\n")
    print(json.dumps(value), flush=True)
    if code or stopped:
        raise RuntimeError("Arm failed; do not repair or repeat the experiment")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=("contracts", "imports", "smoke", "development"))
    parser.add_argument("--mode", choices=MODES, default="RAW_RGB")
    args = parser.parse_args()
    run(args.phase, args.mode)
