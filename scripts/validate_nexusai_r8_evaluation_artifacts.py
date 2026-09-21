#!/usr/bin/env python3
"""Fail-closed validation for the R8 evaluation artifact acquisition contract."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from urllib.parse import urlparse


ALLOWED_HOSTS = {
    "github.com",
    "raw.githubusercontent.com",
    "storage.openvinotoolkit.org",
}
HASH_LENGTHS = {"MD5": 32, "SHA256": 64, "SHA384": 96}


def validate(path: Path) -> dict:
    payload = json.loads(path.read_text(encoding="utf-8"))
    assert payload["contract_version"] == "nexusai.r8-evaluation-artifact-acquisition/v1"
    assert payload["scope"] == "isolated_offline_evaluation_only"
    for key in (
        "production_role_assignment",
        "production_runtime_mutation",
        "retained_case_ingest",
        "automatic_promotion",
    ):
        assert payload[key] is False, f"{key} must remain false"

    candidates = payload["candidates"]
    ids = [candidate["candidate_id"] for candidate in candidates]
    assert len(ids) == len(set(ids)) and ids, "candidate IDs must be unique"
    artifacts = 0
    for candidate in candidates:
        assert len(candidate["revision"]) == 40
        assert candidate["evaluation_decision"].startswith("acquire_")
        for url_key in ("license_url", "notice_url"):
            if candidate.get(url_key):
                parsed = urlparse(candidate[url_key])
                assert parsed.scheme == "https" and parsed.hostname in ALLOWED_HOSTS
        names: set[str] = set()
        for artifact in candidate["artifacts"]:
            artifacts += 1
            name = artifact["name"]
            assert name == Path(name).name and name not in names
            names.add(name)
            parsed = urlparse(artifact["url"])
            assert parsed.scheme == "https" and parsed.hostname in ALLOWED_HOSTS
            assert isinstance(artifact["size_bytes"], int) and artifact["size_bytes"] > 0
            algorithm = artifact.get("checksum_algorithm")
            checksum = artifact.get("checksum")
            if algorithm is None:
                assert checksum is None and artifact.get("integrity_note")
            else:
                assert algorithm in HASH_LENGTHS
                assert isinstance(checksum, str) and len(checksum) == HASH_LENGTHS[algorithm]
                int(checksum, 16)
            for member in artifact.get("expected_archive_members", []):
                assert member["name"] == Path(member["name"]).name
                assert isinstance(member["size_bytes"], int) and member["size_bytes"] > 0
                member_algorithm = member["checksum_algorithm"]
                member_checksum = member["checksum"]
                assert member_algorithm in HASH_LENGTHS
                assert len(member_checksum) == HASH_LENGTHS[member_algorithm]
                int(member_checksum, 16)

    for dataset in payload["datasets"]:
        assert dataset["decision"] == "not_downloadable_yet"
        assert dataset["reason"]

    return {"status": "pass", "candidates": len(candidates), "artifacts": artifacts}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", type=Path, required=True)
    args = parser.parse_args()
    print(json.dumps(validate(args.manifest), sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
