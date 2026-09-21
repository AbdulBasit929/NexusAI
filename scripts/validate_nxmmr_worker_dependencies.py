# SPDX-License-Identifier: MIT
"""Resolve dependency metadata only; never install packages, build images or load models."""
import argparse
import hashlib
import importlib.metadata
import json
import platform
import subprocess
import sys
import time
from pathlib import Path


def installed():
    return sorted((d.metadata["Name"], d.version) for d in importlib.metadata.distributions())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--requirements", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    if sys.version_info[:2] != (3, 11) or platform.system() != "Linux" or platform.machine() != "x86_64":
        raise RuntimeError("Use the existing Linux x86-64 Python 3.11 image, not host Python")
    digest = hashlib.sha256(args.requirements.read_bytes()).hexdigest()
    args.output.mkdir(parents=True, exist_ok=True)
    before = installed()
    command = [sys.executable, "-m", "pip", "install", "--dry-run", "--ignore-installed",
               "--only-binary=:all:", "--no-binary=nats-py", "--no-build-isolation",
               "--disable-pip-version-check", "--timeout", "30", "--retries", "2",
               "--cache-dir", str(args.output / "pip-cache"), "--report", str(args.output / "resolution.json"),
               "-r", str(args.requirements)]
    started = time.monotonic()
    timed_out = False
    with (args.output / "resolver.log").open("x", encoding="utf-8") as log:
        try:
            result = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT, timeout=900, check=False)
            exit_code = result.returncode
        except subprocess.TimeoutExpired:
            timed_out = True
            exit_code = 124
    unchanged = before == installed()
    report = {
        "state": "PASS" if exit_code == 0 and unchanged else "FAIL",
        "requirements_sha256": digest,
        "python": sys.version,
        "platform": platform.platform(),
        "pip": importlib.metadata.version("pip"),
        "exit_code": exit_code, "timed_out": timed_out,
        "installed_distributions_unchanged": unchanged,
        "packages_installed": False, "image_built": False,
        "models_loaded_or_downloaded": False, "live_services_mutated": False,
        "wall_seconds": round(time.monotonic() - started, 3),
    }
    assert hashlib.sha256(args.requirements.read_bytes()).hexdigest() == digest
    if exit_code == 0:
        resolved = json.loads((args.output / "resolution.json").read_text())
        versions = {p["metadata"]["name"].lower().replace("_", "-"): p["metadata"]["version"] for p in resolved["install"]}
        assert versions["safetensors"] == "0.6.2"
        assert "ultralytics" not in versions
        report["resolved_versions"] = versions
        report["resolution_sha256"] = hashlib.sha256((args.output / "resolution.json").read_bytes()).hexdigest()
    (args.output / "validation.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))
    return 0 if report["state"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
