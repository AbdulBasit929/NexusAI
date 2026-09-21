#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Provision only the private, authorized Video V2 dependency overlay.

Run in the existing immutable Linux worker image, never in a live container.
The base image is part of the lock: its site-packages are read-only; only the
private venv receives the resolved override wheels. No model code runs here.
"""
from __future__ import annotations

import argparse
import hashlib
import importlib.metadata as metadata
import json
import platform
import subprocess
import sys
import urllib.parse
import urllib.request
import zipfile
from pathlib import Path

BASE_IMAGE = "sha256:be13dbec4bd9bb62c824140df3777b898d7ad2801a0e68cd512c00dafaf4f9ef"
CORE = {
    "torch": "2.5.1+cpu", "torchvision": "0.20.1+cpu",
    "ultralytics": "8.0.114", "numpy": "2.3.5", "fast-alpr": "0.4.0",
    "fast-plate-ocr": "1.1.0", "open-image-models": "0.6.0",
}
SOURCES = {"download.pytorch.org", "pypi.org", "files.pythonhosted.org"}


def normalized(name):
    return name.lower().replace("_", "-").replace(".", "-")


def write_new(path, value):
    with path.open("x", encoding="utf-8") as stream:
        json.dump(value, stream, indent=2, ensure_ascii=False)
        stream.write("\n")


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def inventory():
    from pip._vendor.packaging.tags import sys_tags
    return {
        "base_image": BASE_IMAGE, "platform": platform.system(),
        "machine": platform.machine(), "python": sys.version,
        "implementation": sys.implementation.name,
        "wheel_tags": [str(tag) for tag in sys_tags()],
        "packages": {normalized(dist.metadata["Name"]): dist.version for dist in metadata.distributions()},
    }


def acquire(root):
    inv = inventory()
    assert inv["platform"] == "Linux" and inv["machine"] == "x86_64"
    assert sys.version_info[:2] == (3, 11) and inv["implementation"] == "cpython"
    write_new(root / "base-platform-fingerprint.json", inv)
    venv = root / "venv"
    if venv.exists():
        raise RuntimeError("refusing to reprovision an existing evaluator")
    subprocess.run([sys.executable, "-m", "venv", "--copies", "--system-site-packages", str(venv)], check=True)
    python = str(venv / "bin/python")
    constraints = {**inv["packages"], **CORE}
    constraint_path = root / "base-and-core-constraints.txt"
    constraint_path.write_text("".join(f"{name}=={version}\n" for name, version in sorted(constraints.items())))
    requirements = []
    for name, version in CORE.items():
        if name in {"torch", "torchvision"}:
            filename = f"{name}-{version}-cp311-cp311-linux_x86_64.whl"
            requirements.append(f"{name} @ https://download.pytorch.org/whl/cpu/{urllib.parse.quote(filename)}")
        else:
            requirements.append(f"{name}=={version}")
    reqpath = root / "core-request.txt"
    reqpath.write_text("\n".join(requirements) + "\n")
    reportpath = root / "resolution-report.json"
    command = [python, "-m", "pip", "install", "--dry-run", "--only-binary=:all:",
               "--disable-pip-version-check", "--index-url", "https://pypi.org/simple",
               "--cache-dir", str(root / "pip-cache"), "--report", str(reportpath),
               "-c", str(constraint_path), "-r", str(reqpath)]
    result = subprocess.run(command, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    (root / "resolution.log").write_text(result.stdout)
    print(result.stdout, flush=True)
    if result.returncode:
        write_new(root / "provisioning-blocked.json", {"phase": "resolve", "exit_code": result.returncode})
        raise RuntimeError("resolver failed; no installation performed")
    report = json.loads(reportpath.read_text())
    planned = {normalized(item["metadata"]["name"]): item["metadata"]["version"] for item in report["install"]}
    for name, version in CORE.items():
        assert planned.get(name, inv["packages"].get(name)) == version, (name, planned)
    for item in report["install"]:
        url = item["download_info"]["url"]
        assert urllib.parse.urlparse(url).hostname in SOURCES, url
        assert url.split("?")[0].endswith(".whl"), url
    wheelhouse = root / "wheelhouse"
    wheelhouse.mkdir()
    manifest, lock_lines = [], []
    for item in report["install"]:
        info = item["download_info"]
        url = info["url"]
        filename = urllib.parse.unquote(Path(urllib.parse.urlparse(url).path).name)
        target = wheelhouse / filename
        expected = info["archive_info"]["hashes"]["sha256"]
        print(f"Acquiring {filename}", flush=True)
        with urllib.request.urlopen(url, timeout=60) as response:
            assert urllib.parse.urlparse(response.url).hostname in SOURCES, response.url
            with target.open("xb") as output:
                while block := response.read(1024 * 1024):
                    output.write(block)
        assert digest(target) == expected, filename
        with zipfile.ZipFile(target) as wheel:
            wheel_meta = next(name for name in wheel.namelist() if name.endswith(".dist-info/WHEEL"))
            tags = [line[5:] for line in wheel.read(wheel_meta).decode().splitlines() if line.startswith("Tag: ")]
        assert set(tags) & set(inv["wheel_tags"]), (filename, tags)
        name, version = normalized(item["metadata"]["name"]), item["metadata"]["version"]
        manifest.append({"package": name, "version": version, "filename": filename,
                         "wheel_tags": tags, "source_url": url,
                         "source_index": "https://download.pytorch.org/whl/cpu" if name in {"torch", "torchvision"} else "https://pypi.org/simple",
                         "size_bytes": target.stat().st_size, "sha256": expected})
        lock_lines.append(f"{name}=={version} --hash=sha256:{expected}\n")
    write_new(root / "downloaded-artifacts.json", {"base_image": BASE_IMAGE, "artifacts": manifest})
    (root / "overlay-requirements.lock").write_text("".join(sorted(lock_lines)))
    write_new(root / "resolved-runtime-lock.json", {"base_image": BASE_IMAGE, "core": CORE,
              "resolved_versions": {**inv["packages"], **planned}, "base_constraints_sha256": digest(constraint_path)})
    print(json.dumps({"acquisition": "PASS", "artifact_count": len(manifest), "bytes": sum(x["size_bytes"] for x in manifest)}), flush=True)


def install(root):
    manifest = json.loads((root / "downloaded-artifacts.json").read_text())
    for item in manifest["artifacts"]:
        assert digest(root / "wheelhouse" / item["filename"]) == item["sha256"]
    python = str(root / "venv/bin/python")
    assert Path(sys.prefix).resolve() != (root / "venv").resolve()
    result = subprocess.run([python, "-m", "pip", "install", "--no-index", "--no-deps",
               "--require-hashes", "--find-links", str(root / "wheelhouse"),
               "-r", str(root / "overlay-requirements.lock"), "--disable-pip-version-check"],
               text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    (root / "installation.log").write_text(result.stdout)
    print(result.stdout, flush=True)
    result.check_returncode()
    check = subprocess.run([python, "-m", "pip", "check"], text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    write_new(root / "pip-check.json", {"exit_code": check.returncode, "output": check.stdout})
    print(check.stdout, flush=True)
    check.check_returncode()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=["acquire", "install"])
    parser.add_argument("--root", type=Path, default=Path("/evaluator"))
    args = parser.parse_args()
    {"acquire": acquire, "install": install}[args.phase](args.root)
