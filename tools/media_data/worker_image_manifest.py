#!/usr/bin/env python3
"""Fingerprints the files of a running worker (or search-service) container and compares them with the repository.

Why: the live worker image is a chain of overlays (see docs/roadmap/worker-image-reproduction-20261124.md), so its /app files are NOT the repository's. Before any rebuild
of the worker from the repository, run this on the live container to see exactly which files would change, and run it again on the rebuilt image's container to prove it
equals the live one (or to list what differs on purpose).

    python tools/media_data/worker_image_manifest.py compare nexusai-forensic-records-worker-1
    python tools/media_data/worker_image_manifest.py manifest nexusai-forensic-records-worker-1 docs/roadmap/worker-image-live-manifest-20261124.json
    python tools/media_data/worker_image_manifest.py check nexusai-forensic-records-worker-1 docs/roadmap/worker-image-live-manifest-20261124.json

It only reads: `docker cp` of /app out of the container into a temporary folder, `git show HEAD:...`. Nothing is started, stopped or changed.
"""
import argparse
import difflib
import hashlib
import json
import pathlib
import subprocess
import sys
import tempfile

REPO = pathlib.Path(__file__).resolve().parents[2]
SOURCE = "ingestion/forensic_records"


def copy_app(container: str, into: pathlib.Path) -> pathlib.Path:
    done = subprocess.run(["docker", "cp", f"{container}:/app", str(into)], capture_output=True, text=True)
    if done.returncode != 0:
        sys.exit(f"docker cp failed: {done.stderr.strip()[:300]}")
    return into / "app"


def files_of(app: pathlib.Path) -> dict[str, bytes]:
    return {p.name: p.read_bytes() for p in sorted(app.iterdir()) if p.is_file()}


def head_file(name: str):
    done = subprocess.run(["git", "-C", str(REPO), "show", f"HEAD:{SOURCE}/{name}"], capture_output=True)
    return done.stdout if done.returncode == 0 else None


def lf(data: bytes) -> bytes:
    return data.replace(b"\r\n", b"\n")


def image_of(container: str) -> str:
    done = subprocess.run(["docker", "inspect", "--format", "{{.Image}}", container], capture_output=True, text=True)
    return done.stdout.strip()


def cmd_compare(container: str) -> None:
    with tempfile.TemporaryDirectory() as tmp:
        live = files_of(copy_app(container, pathlib.Path(tmp)))
    listing = subprocess.run(["git", "-C", str(REPO), "ls-tree", "--name-only", f"HEAD:{SOURCE}/"], capture_output=True, text=True).stdout.split()
    print(f"container {container}, image {image_of(container)[:19]}, {len(live)} files in /app")
    same = 0
    for name, data in live.items():
        head = head_file(name)
        if head is None:
            print(f"  only in the container: {name}")
        elif lf(head) == lf(data):
            same += 1
        else:
            a, b = lf(data).decode("utf-8", "replace").splitlines(), lf(head).decode("utf-8", "replace").splitlines()
            changes = [line for line in difflib.unified_diff(a, b, lineterm="", n=0) if line[:1] in "+-" and line[:3] not in ("+++", "---")]
            print(f"  DIFFERS: {name}: HEAD has {sum(1 for c in changes if c[0] == '+')} lines the container lacks, the container has {sum(1 for c in changes if c[0] == '-')} lines HEAD lacks")
    print(f"  identical to HEAD (line endings ignored): {same}")
    print("  in HEAD but not in the container:", ", ".join(n for n in listing if n.endswith((".py", ".json")) and n not in live) or "none")


def cmd_manifest(container: str, out: str) -> None:
    with tempfile.TemporaryDirectory() as tmp:
        live = files_of(copy_app(container, pathlib.Path(tmp)))
    manifest = {"container": container, "image": image_of(container), "files": {n: {"sha256": hashlib.sha256(d).hexdigest(), "bytes": len(d)} for n, d in live.items()}}
    pathlib.Path(out).write_text(json.dumps(manifest, indent=1, sort_keys=True) + "\n", encoding="utf-8")
    print(f"wrote {out}: {len(live)} files")


def cmd_check(container: str, manifest_path: str) -> None:
    expected = json.loads(pathlib.Path(manifest_path).read_text(encoding="utf-8"))["files"]
    with tempfile.TemporaryDirectory() as tmp:
        live = files_of(copy_app(container, pathlib.Path(tmp)))
    bad = 0
    for name in sorted(set(expected) | set(live)):
        have = hashlib.sha256(live[name]).hexdigest() if name in live else None
        want = expected.get(name, {}).get("sha256")
        if have != want:
            bad += 1
            print(f"  {'missing' if have is None else 'extra' if want is None else 'DIFFERS'}: {name}")
    print("EQUAL to the manifest" if not bad else f"{bad} file(s) differ from the manifest")
    sys.exit(1 if bad else 0)


def main() -> None:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="mode", required=True)
    sub.add_parser("compare").add_argument("container")
    p = sub.add_parser("manifest"); p.add_argument("container"); p.add_argument("out")
    p = sub.add_parser("check"); p.add_argument("container"); p.add_argument("manifest")
    args = parser.parse_args()
    if args.mode == "compare":
        cmd_compare(args.container)
    elif args.mode == "manifest":
        cmd_manifest(args.container, args.out)
    else:
        cmd_check(args.container, args.manifest)


if __name__ == "__main__":
    main()
