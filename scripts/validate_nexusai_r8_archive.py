#!/usr/bin/env python3
"""Inspect and hash an R8 ZIP member without extracting untrusted content."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
from zipfile import ZipFile


def validate_archive(
    archive: Path, member_name: str, expected_size: int, algorithm: str, checksum: str
) -> dict:
    with ZipFile(archive) as zipped:
        members = zipped.infolist()
        if len(members) != 1:
            raise ValueError(f"archive must contain exactly one member, found {len(members)}")
        info = members[0]
        member_path = PurePosixPath(info.filename)
        if member_path.is_absolute() or ".." in member_path.parts or info.filename != member_name:
            raise ValueError(f"unsafe or unexpected archive member: {info.filename}")
        if info.is_dir() or info.file_size != expected_size:
            raise ValueError(f"member size mismatch: {info.file_size}!={expected_size}")
        digest = hashlib.new(algorithm.lower())
        with zipped.open(info) as stream:
            while chunk := stream.read(1024 * 1024):
                digest.update(chunk)
        actual = digest.hexdigest()
        if actual.lower() != checksum.lower():
            raise ValueError(f"member {algorithm} mismatch")
    return {
        "status": "pass",
        "member": member_name,
        "size_bytes": expected_size,
        algorithm.lower(): actual,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--archive", type=Path, required=True)
    parser.add_argument("--member", required=True)
    parser.add_argument("--size", type=int, required=True)
    parser.add_argument("--algorithm", required=True)
    parser.add_argument("--checksum", required=True)
    args = parser.parse_args()
    result = validate_archive(args.archive, args.member, args.size, args.algorithm, args.checksum)
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
