#!/usr/bin/env python3
"""Generate deterministic forensic seed data for KB + records testing.

The output intentionally uses canonical LocalAI records field names so the same
files exercise UI upload, exact records queries, agent tools, and KB evidence.
"""

from __future__ import annotations

import argparse
import csv
import json
import random
from datetime import datetime, timedelta, timezone
from pathlib import Path


AREAS = [
    ("Gulberg", "CELL-GLB-01", 31.5204, 74.3587),
    ("DHA Phase 5", "CELL-DHA-05", 31.4697, 74.4097),
    ("Model Town", "CELL-MT-03", 31.4832, 74.3229),
    ("Liberty Market", "CELL-LB-02", 31.5102, 74.3441),
    ("Airport Road", "CELL-ARP-08", 31.5216, 74.4036),
    ("North Road", "CAM-NR-09", 31.5591, 74.3273),
    ("Gate 4", "CAM-G4-07", 31.5011, 74.3512),
]

PRIMARY_NUMBERS = [
    "923001110001",
    "923001110002",
    "923001110003",
    "923461678183",
    "923331234567",
    "923451112233",
]

TARGET_NUMBERS = [
    "923009998881",
    "923009998882",
    "923009998883",
    "923009998884",
    "923009998885",
    "923009998886",
    "923009998887",
    "923009998888",
]

PLATES = ["ABC-123", "XYZ-789", "LEA-447", "LHR-2026", "ICT-404", "KHI-777"]
CAMERAS = ["CAM-7", "CAM-9", "CAM-12", "CAM-21"]
HOSTS = ["10.20.1.7", "10.20.1.8", "10.20.2.12", "172.16.4.22", "192.168.10.44"]
DOMAINS = ["mail.example.test", "maps.example.test", "chat.example.test", "storage.example.test"]


def jitter(value: float, rng: random.Random, scale: float = 0.002) -> float:
    return round(value + rng.uniform(-scale, scale), 6)


def write_cdr(path: Path, rows: int, rng: random.Random) -> None:
    start = datetime(2026, 4, 2, 0, 0, tzinfo=timezone.utc)
    headers = [
        "MSISDN",
        "call_org_num",
        "CALL_DIALED_NUM",
        "IMSI",
        "IMEI",
        "CALL_START_DT_TM",
        "CALL_END_DT_TM",
        "INBOUND_OUTBOUND_IND",
        "Call_Network_Volume",
        "Lac_Id",
        "Site_Id",
        "Cell_SITE_ID",
        "lat",
        "longitude",
        "CALL_TYPE",
        "location",
    ]
    with path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=headers)
        writer.writeheader()
        for i in range(rows):
            area, cell, lat, lon = rng.choice(AREAS[:5])
            source = rng.choice(PRIMARY_NUMBERS)
            target = rng.choice(TARGET_NUMBERS)
            call_type = rng.choices(["CALL", "SMS", "GPRS", "VOLTE"], weights=[18, 16, 54, 12])[0]
            direction = rng.choice(["incoming", "outgoing"])
            status = rng.choices(["completed", "failed", "dropped"], weights=[86, 9, 5])[0]
            duration = 0 if call_type in {"SMS", "GPRS"} or status != "completed" else rng.randint(4, 1800)
            ts = start + timedelta(minutes=i * rng.randint(1, 7), seconds=rng.randint(0, 59))
            end = ts + timedelta(seconds=duration)
            writer.writerow(
                {
                    "MSISDN": source,
                    "call_org_num": source if direction == "outgoing" else target,
                    "CALL_DIALED_NUM": target if direction == "outgoing" else source,
                    "IMSI": f"41001{rng.randint(1000000000, 9999999999)}",
                    "IMEI": f"35678901{rng.randint(100000, 999999)}",
                    "CALL_START_DT_TM": ts.strftime("%Y-%m-%d %H:%M:%S"),
                    "CALL_END_DT_TM": end.strftime("%Y-%m-%d %H:%M:%S"),
                    "INBOUND_OUTBOUND_IND": direction.upper(),
                    "Call_Network_Volume": rng.randint(0, 8_000_000) if call_type == "GPRS" else 0,
                    "Lac_Id": rng.randint(1000, 9999),
                    "Site_Id": rng.randint(100, 999),
                    "Cell_SITE_ID": cell,
                    "lat": jitter(lat, rng),
                    "longitude": jitter(lon, rng),
                    "CALL_TYPE": call_type,
                    "location": area,
                }
            )


def write_anpr(path: Path, rows: int, rng: random.Random) -> None:
    start = datetime(2026, 7, 14, 6, 0, tzinfo=timezone.utc)
    headers = ["timestamp", "plate_number", "camera_id", "location", "latitude", "longitude", "confidence", "lane"]
    with path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=headers)
        writer.writeheader()
        for i in range(rows):
            area, _, lat, lon = rng.choice(AREAS[5:] + AREAS[:3])
            plate = rng.choices(PLATES, weights=[28, 18, 14, 14, 13, 13])[0]
            writer.writerow(
                {
                    "timestamp": (start + timedelta(minutes=i * rng.randint(2, 11))).isoformat().replace("+00:00", "Z"),
                    "plate_number": plate,
                    "camera_id": rng.choice(CAMERAS),
                    "location": area,
                    "latitude": jitter(lat, rng),
                    "longitude": jitter(lon, rng),
                    "confidence": round(rng.uniform(0.74, 0.99), 3),
                    "lane": rng.randint(1, 4),
                }
            )


def write_ipdr(path: Path, rows: int, rng: random.Random) -> None:
    start = datetime(2026, 7, 14, 0, 0, tzinfo=timezone.utc)
    csv_path = path.with_suffix(".csv")
    fieldnames = ["timestamp", "source_ip", "destination_ip", "domain", "protocol", "bytes", "subscriber_id", "session_id"]
    generated_rows = []
    for i in range(rows):
        generated_rows.append(
            {
                "timestamp": (start + timedelta(seconds=i * rng.randint(5, 45))).isoformat().replace("+00:00", "Z"),
                "source_ip": rng.choice(HOSTS),
                "destination_ip": rng.choice(["8.8.8.8", "1.1.1.1", "20.42.65.90", "104.18.12.20", "151.101.1.69"]),
                "domain": rng.choice(DOMAINS),
                "protocol": rng.choice(["TCP", "UDP", "HTTPS", "DNS"]),
                "bytes": rng.randint(220, 8_000_000),
                "subscriber_id": rng.choice(PRIMARY_NUMBERS),
                "session_id": f"ipdr-{i + 1:08d}",
            }
        )
    with path.open("w", encoding="utf-8") as f:
        for row in generated_rows:
            f.write(json.dumps(row, separators=(",", ":")) + "\n")
    with csv_path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(generated_rows)


def write_access_log(path: Path, rows: int, rng: random.Random) -> None:
    start = datetime(2026, 7, 14, 8, 0, tzinfo=timezone.utc)
    paths = ["/login", "/case/records-demo", "/api/records/query", "/api/agents/chat", "/download/report"]
    with path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=["timestamp", "source_ip", "http_method", "path", "status", "user_agent"])
        writer.writeheader()
        for i in range(rows):
            writer.writerow(
                {
                    "timestamp": (start + timedelta(seconds=i * rng.randint(3, 18))).isoformat().replace("+00:00", "Z"),
                    "source_ip": rng.choice(HOSTS),
                    "http_method": rng.choice(["GET", "POST", "POST", "GET"]),
                    "path": rng.choice(paths),
                    "status": rng.choices([200, 201, 400, 401, 403, 500], weights=[70, 8, 7, 5, 6, 4])[0],
                    "user_agent": rng.choice(["AnalystWorkbench/1.0", "CasePortal/2.4", "Mozilla/5.0"]),
                }
            )


def write_docs(out: Path) -> None:
    (out / "policy_records_handling.md").write_text(
        """# Records Handling Policy

Collection records-demo is a controlled evidence workspace. Structured CDR, ANPR, IPDR,
subscriber, tower, transaction, and access-log files must be ingested through the Records
pipeline for exact analytics. Knowledge Base retrieval is used for policy context, source
previews, case notes, and citations. Exact counts, durations, relationships, and timelines
must come from deterministic records queries.

Retention rule: raw uploads are retained as evidence; derived analytics may be regenerated
from source files. Duplicate uploads should be flagged by SHA-256 and should not create a new
batch unless an analyst explicitly forces a re-ingest for audit testing.
""",
        encoding="utf-8",
    )
    (out / "case_notes_records_demo.txt").write_text(
        """Case notes for records-demo

Targets of interest:
- Phone number 923461678183 appears in CDR records.
- Plate ABC-123 appears near Gate 4 and North Road.
- Cross-source timeline should combine structured observations with this narrative note.

Analyst question examples:
- Summarize evidence for ABC-123 with source citations.
- Show call type breakdown for 923461678183.
- Show data quality and duplicate-row counts.
""",
        encoding="utf-8",
    )


def write_manifest(out: Path, args: argparse.Namespace) -> None:
    manifest = {
        "collection_id": args.collection,
        "seed": args.seed,
        "files": [
            {"path": "seed_cdr_large.csv", "record_type": "cdr", "rows": args.cdr_rows},
            {"path": "seed_anpr.csv", "record_type": "anpr", "rows": args.anpr_rows},
            {"path": "seed_ipdr.csv", "record_type": "ipdr", "rows": args.ipdr_rows},
            {"path": "seed_ipdr.jsonl", "record_type": "ipdr", "rows": args.ipdr_rows},
            {"path": "seed_access_log.csv", "record_type": "access_log", "rows": args.access_rows},
            {"path": "policy_records_handling.md", "record_type": "document"},
            {"path": "case_notes_records_demo.txt", "record_type": "document"},
        ],
    }
    (out / "manifest.json").write_text(json.dumps(manifest, indent=2), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--out", default="fixtures/forensic_seed/records-demo")
    parser.add_argument("--collection", default="records-demo")
    parser.add_argument("--seed", type=int, default=20260714)
    parser.add_argument("--cdr-rows", type=int, default=5000)
    parser.add_argument("--anpr-rows", type=int, default=750)
    parser.add_argument("--ipdr-rows", type=int, default=2500)
    parser.add_argument("--access-rows", type=int, default=1000)
    args = parser.parse_args()

    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    rng = random.Random(args.seed)
    write_cdr(out / "seed_cdr_large.csv", args.cdr_rows, rng)
    write_anpr(out / "seed_anpr.csv", args.anpr_rows, rng)
    write_ipdr(out / "seed_ipdr.jsonl", args.ipdr_rows, rng)
    write_access_log(out / "seed_access_log.csv", args.access_rows, rng)
    write_docs(out)
    write_manifest(out, args)
    print(f"Wrote forensic seed pack to {out.resolve()}")


if __name__ == "__main__":
    main()
