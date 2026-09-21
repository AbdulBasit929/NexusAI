#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Project only the locked reserved rows after one-time consumption starts."""
from __future__ import annotations

import argparse
import csv
import hashlib
import io
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def canonical_digest(value):
    encoded = json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False)
    return hashlib.sha256(encoded.encode()).hexdigest()


def selected_records(stream, allowed):
    """Lexically discard non-allowlisted payloads before CSV decoding."""
    header = stream.readline()
    if not header.lstrip("\ufeff").startswith("event_id,"):
        raise ValueError("oracle identity must be first CSV field")
    yield header.lstrip("\ufeff")
    quoted, identity, selected, retained = False, "", None, []
    while char := stream.read(1):
        if char == '"':
            quoted = not quoted
        if selected is None:
            identity += char
            if char == "," and not quoted:
                selected = identity[:-1].strip('"') in allowed
                if selected:
                    retained.append(identity)
                identity = ""
        elif selected:
            retained.append(char)
        if char == "\n" and not quoted:
            if selected:
                yield "".join(retained)
            identity, retained, selected = "", [], None
    if selected:
        yield "".join(retained)


def write_signed(path, value):
    if path.exists():
        raise RuntimeError(f"refusing to overwrite {path}")
    value["receipt_digest"] = canonical_digest(value)
    with path.open("x", encoding="utf-8", newline="") as stream:
        json.dump(value, stream, indent=2)
        stream.write("\n")


def project(args):
    if args.output.exists() or args.receipt.exists():
        raise RuntimeError("reserved oracle projection is one-time only")
    start = json.loads(args.consumption_start.read_text())
    digest = start.pop("receipt_digest")
    assert canonical_digest(start) == digest
    assert start["authorization_consumed"] is True
    assert start["reserved_evaluation_count"] == 1
    assert start["candidate_freeze_digest"] == args.expected_freeze_digest
    split = json.loads(args.split.read_text())
    split_digest = split.pop("split_digest")
    assert canonical_digest(split) == split_digest
    validation = json.loads((args.oracle_root / "ground-truth-validation.json").read_text())
    assert validation["GroundTruthValidation"] == "PASS"
    assert validation["model_output_used_to_create_truth"] is False
    assert validation["gold_digest"] == split["gold_digest"]
    allowed = set(split["reserved_event_ids"])
    assert len(allowed) == split["reserved_event_count"] == 8
    assert not allowed & set(split["development_event_ids"])
    with (args.oracle_root / "video-events.csv").open(encoding="utf-8-sig", newline="") as stream:
        projected = "".join(selected_records(stream, allowed))
    rows = list(csv.DictReader(io.StringIO(projected)))
    assert {row["event_id"] for row in rows} == allowed and len(rows) == 8
    assert all(row["event_type"] == "plate" for row in rows)
    assert all(row["label_source"] == "independent_human_annotation" for row in rows)
    assert all(row["model_output_seen"].strip().lower() in ("false", "0", "no") for row in rows)
    with args.output.open("x", encoding="utf-8", newline="") as stream:
        stream.write(projected)
    write_signed(args.receipt, {
        "contract_version": "nexusai.nxmmr.video-v22-reserved-oracle-projection/v1",
        "candidate_freeze_digest": args.expected_freeze_digest,
        "reserved_event_count": len(rows),
        "reserved_event_ids": sorted(allowed),
        "gold_digest": validation["gold_digest"],
        "split_digest": split_digest,
        "projection_sha256": hashlib.sha256(args.output.read_bytes()).hexdigest(),
        "non_reserved_payloads_csv_decoded": False,
        "development_payloads_returned_or_scored": False,
        "model_output_used_as_oracle": False,
        "reserved_evaluation_count": 1,
    })
    return len(rows)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--oracle-root", type=Path, required=True)
    parser.add_argument("--split", type=Path, required=True)
    parser.add_argument("--consumption-start", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--receipt", type=Path, required=True)
    parser.add_argument("--expected-freeze-digest", required=True)
    count = project(parser.parse_args())
    print(json.dumps({"reserved_rows_projected": count, "development_rows_returned": 0}))
