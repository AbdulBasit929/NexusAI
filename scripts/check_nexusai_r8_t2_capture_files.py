#!/usr/bin/env python3
"""Check T2 capture completeness before independent annotation begins."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
from typing import Any


ALLOWED_MAGIC = {
    ".jpg": (b"\xff\xd8\xff",),
    ".jpeg": (b"\xff\xd8\xff",),
    ".png": (b"\x89PNG\r\n\x1a\n",),
}


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def inspect_pack(pack_root: Path) -> dict[str, Any]:
    plan_path = pack_root / "capture-plan.json"
    if not plan_path.is_file():
        return {"status": "fail", "capture_complete": False, "errors": ["capture-plan.json is missing"]}
    plan = json.loads(plan_path.read_text(encoding="utf-8-sig"))
    items = plan.get("items", [])
    expected = {str(item["filename"]).replace("\\", "/"): item for item in items}
    discovered = {
        path.relative_to(pack_root).as_posix(): path
        for path in (pack_root / "images").glob("*") if path.is_file()
    }
    missing = sorted(set(expected) - set(discovered))
    unexpected = sorted(set(discovered) - set(expected))
    invalid: list[dict[str, str]] = []
    hashes: dict[str, list[str]] = {}
    accepted: list[dict[str, Any]] = []
    for relative_path in sorted(set(expected) & set(discovered)):
        path = discovered[relative_path]
        if path.stat().st_size == 0:
            invalid.append({"file": relative_path, "reason": "empty file"})
            continue
        signatures = ALLOWED_MAGIC.get(path.suffix.lower())
        with path.open("rb") as source:
            header = source.read(8)
        if signatures is None or not any(header.startswith(value) for value in signatures):
            invalid.append({"file": relative_path, "reason": "extension/signature mismatch"})
            continue
        digest = sha256(path)
        hashes.setdefault(digest, []).append(relative_path)
        accepted.append({
            "image_id": expected[relative_path]["image_id"],
            "file": relative_path,
            "partition": expected[relative_path]["partition"],
            "bytes": path.stat().st_size,
            "sha256": digest,
        })
    duplicates = [files for files in hashes.values() if len(files) > 1]
    errors = []
    if missing:
        errors.append(f"{len(missing)} planned files are missing")
    if unexpected:
        errors.append(f"{len(unexpected)} unexpected files are present")
    if invalid:
        errors.append(f"{len(invalid)} files are empty or have invalid signatures")
    if duplicates:
        errors.append(f"{len(duplicates)} duplicate-content groups were found")
    return {
        "status": "pass" if not errors else "incomplete",
        "capture_complete": not errors and len(accepted) == len(items),
        "planned_count": len(items),
        "accepted_file_count": len(accepted),
        "development_file_count": sum(item["partition"] == "development" for item in accepted),
        "holdout_file_count": sum(item["partition"] == "holdout" for item in accepted),
        "missing": missing,
        "unexpected": unexpected,
        "invalid": invalid,
        "duplicate_content_groups": duplicates,
        "accepted_files": accepted,
        "errors": errors,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--pack-root", type=Path, required=True)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    result = inspect_pack(args.pack_root)
    rendered = json.dumps(result, indent=2) + "\n"
    if args.output:
        args.output.write_text(rendered, encoding="utf-8")
    print(rendered, end="")
    return 0 if result["capture_complete"] else 2


if __name__ == "__main__":
    raise SystemExit(main())
