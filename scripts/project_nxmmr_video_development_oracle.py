#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Export only allowlisted development records from the shared locked CSV.

Non-allowlisted record payloads are discarded lexically, never CSV-decoded,
returned, logged or supplied to an evaluator. No model output is an oracle.
"""
from __future__ import annotations
import csv
import argparse
import hashlib
import io
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def selected_records(stream, allowed):
    header = stream.readline()
    if not header.lstrip("\ufeff").startswith("event_id,"):
        raise ValueError("oracle identity must be first CSV field")
    yield header.lstrip("\ufeff")
    quoted = False
    identity = ""
    selected = None
    retained = []
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


def main(candidate="v2"):
    directory = "v21-development" if candidate == "v21" else "exact-runtime"
    results = ROOT / "local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1" / directory
    assert json.loads((results / "smoke.json").read_text())["state"] == "PASS"
    source_root = ROOT / "local-acceptance-models/nxmmr/private-ground-truth/human-verification"
    validation = json.loads((source_root / "ground-truth-validation.json").read_text())
    assert validation["GroundTruthValidation"] == "PASS" and validation["model_output_used_to_create_truth"] is False
    split = json.loads((ROOT / "local-acceptance-models/nxmmr/private-benchmarks/reference-parity-adapter-v1/video-development-reserved-split.json").read_text())
    assert validation["gold_digest"] == split["gold_digest"]
    allowed = set(split["development_event_ids"])
    assert not allowed & set(split["reserved_event_ids"])
    with (source_root / "video-events.csv").open(encoding="utf-8-sig", newline="") as stream:
        projected = "".join(selected_records(stream, allowed))
    rows = list(csv.DictReader(io.StringIO(projected)))
    assert {row["event_id"] for row in rows} == allowed and len(rows) == 11
    output = results / "development-oracle.csv"
    with output.open("x", encoding="utf-8", newline="") as stream:
        stream.write(projected)
    with (results / "development-oracle-projection.json").open("x") as stream:
        json.dump({"event_count": len(rows), "gold_digest": validation["gold_digest"],
                   "split_digest": split["split_digest"],
                   "projection_sha256": hashlib.sha256(output.read_bytes()).hexdigest(),
                   "non_development_payloads_csv_decoded": False,
                   "reserved_payloads_returned_or_scored": False}, stream, indent=2)
        stream.write("\n")
    print(json.dumps({"development_events_projected": len(rows), "reserved_rows_returned": 0,
                      "label_source_values": sorted({row["label_source"] for row in rows})}))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate", choices=["v2", "v21"], default="v2")
    main(parser.parse_args().candidate)
