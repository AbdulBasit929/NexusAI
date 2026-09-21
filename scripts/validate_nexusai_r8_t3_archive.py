#!/usr/bin/env python3
"""Fail-closed structural validation for isolated R8 T3 archives."""

from __future__ import annotations

import argparse
import json
import os
import stat
import tarfile
import zipfile
from pathlib import Path, PurePosixPath


EXECUTABLE_SUFFIXES = {
    ".bat", ".cmd", ".com", ".dll", ".exe", ".hta", ".jar", ".js", ".lnk",
    ".msi", ".ps1", ".scr", ".sh", ".vbs",
}


def safe_member_name(raw_name: str) -> str:
    name = raw_name.replace("\\", "/")
    pure = PurePosixPath(name)
    if not name or name.startswith("/") or pure.is_absolute():
        raise ValueError(f"absolute_or_empty_member:{raw_name!r}")
    if any(part in {"", ".", ".."} for part in pure.parts):
        raise ValueError(f"unsafe_path_component:{raw_name!r}")
    if len(pure.parts[0]) >= 2 and pure.parts[0][1] == ":":
        raise ValueError(f"drive_qualified_member:{raw_name!r}")
    if pure.suffix.lower() in EXECUTABLE_SUFFIXES:
        raise ValueError(f"unexpected_executable_member:{raw_name!r}")
    return name


def validate_tar(path: Path) -> tuple[int, int]:
    members = files = 0
    with tarfile.open(path, mode="r:*") as archive:
        for member in archive:
            members += 1
            safe_member_name(member.name)
            if member.issym() or member.islnk() or member.isdev() or member.isfifo():
                raise ValueError(f"unsafe_special_member:{member.name!r}")
            if member.isfile():
                files += 1
    return members, files


def validate_zip(path: Path) -> tuple[int, int]:
    members = files = 0
    with zipfile.ZipFile(path) as archive:
        for member in archive.infolist():
            members += 1
            safe_member_name(member.filename)
            unix_mode = member.external_attr >> 16
            if unix_mode and stat.S_ISLNK(unix_mode):
                raise ValueError(f"unsafe_symlink_member:{member.filename!r}")
            if not member.is_dir():
                files += 1
    return members, files


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--archive", type=Path, required=True)
    parser.add_argument("--expected-bytes", type=int, required=True)
    args = parser.parse_args()

    archive = args.archive.resolve()
    if not archive.is_file():
        raise SystemExit(f"R8T3ArchiveValidation=FAIL Reason=missing_archive Path={archive}")
    actual_bytes = archive.stat().st_size
    if actual_bytes != args.expected_bytes:
        raise SystemExit(
            f"R8T3ArchiveValidation=FAIL Reason=size_mismatch Expected={args.expected_bytes} Actual={actual_bytes}"
        )
    try:
        if zipfile.is_zipfile(archive):
            archive_format = "zip"
            members, files = validate_zip(archive)
        elif tarfile.is_tarfile(archive):
            archive_format = "tar"
            members, files = validate_tar(archive)
        else:
            raise ValueError("unsupported_or_corrupt_archive")
        if members == 0 or files == 0:
            raise ValueError("archive_has_no_regular_files")
    except (OSError, tarfile.TarError, zipfile.BadZipFile, ValueError) as exc:
        raise SystemExit(f"R8T3ArchiveValidation=FAIL Reason={exc}") from exc

    print(json.dumps({
        "status": "pass",
        "archive": os.fspath(archive),
        "format": archive_format,
        "members": members,
        "regular_files": files,
        "path_traversal": False,
        "links_or_special_members": False,
        "unexpected_executables": False,
        "extraction_performed": False,
    }, sort_keys=True))
    print("R8T3ArchiveValidation=PASS ExtractionPerformed=false")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
