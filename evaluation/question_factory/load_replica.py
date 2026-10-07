#!/usr/bin/env python3
"""Write a SQL script that loads the demo seed files into the stand-in `forensic.records` table.

    python evaluation/question_factory/load_replica.py > /tmp/replica_data.sql
    psql -d <database> -f evaluation/question_factory/replica_schema.sql -f /tmp/replica_data.sql

Each source row becomes one record: raw_payload is the row keyed by its original headers (what the real ingestion stores), record_type comes from
fixtures/forensic_seed/records-demo/manifest.json, the timestamp is the first timestamp-like column. The real database is NOT touched by this script.
"""
import csv
import datetime
import hashlib
import io
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
SEED = os.path.normpath(os.path.join(HERE, "..", "..", "fixtures", "forensic_seed", "records-demo"))
COLLECTION = "records-demo"
TIME_COLUMNS = ("CALL_START_DT_TM", "timestamp", "call_start", "start_time", "activation_date")


# The real ingest (ingestion/forensic_records/worker.py) reads a time WITHOUT an offset in the source zone and stores the UTC instant;
# a time with an offset is converted by itself. The stand-in does the same, so its `timestamp` means what production's means.
# PostgreSQL does the conversion: it accepts "2026-04-02 08:10:00 Asia/Karachi", so this script needs no time-zone database.
SOURCE_ZONE = os.environ.get("FORENSIC_DEFAULT_SOURCE_TIMEZONE", "Asia/Karachi").replace("'", "").replace(",", "")
# A row with no time of its own gets the ingest time in production (the canonical timestamp is coalesce(observed_at, ingested_at)).
# COPY cannot say now(), so such a row is loaded with this marker and set to ingested_at afterwards.
NO_TIME_MARKER = "1970-01-01T00:00:00+00:00"


def parse_time(value):
    value = (value or "").strip()
    if not value:
        return None
    for candidate in (value.replace("Z", "+00:00"), value.replace(" ", "T")):
        try:
            parsed = datetime.datetime.fromisoformat(candidate)
            if parsed.tzinfo is None:
                return parsed.isoformat(sep=" ") + " " + SOURCE_ZONE
            return parsed.isoformat()
        except ValueError:
            continue
    return None


def rows_of(path):
    if path.endswith(".jsonl"):
        with io.open(path, encoding="utf-8") as handle:
            for line in handle:
                if line.strip():
                    yield json.loads(line)
    else:
        with io.open(path, encoding="utf-8", newline="") as handle:
            for row in csv.DictReader(handle):
                yield row


def main():
    with io.open(os.path.join(SEED, "manifest.json"), encoding="utf-8") as handle:
        manifest = json.load(handle)
    buffer = io.StringIO()
    writer = csv.writer(buffer, lineterminator="\n")
    total = 0
    for entry in manifest["files"]:
        path = os.path.join(SEED, entry["path"])
        if not os.path.exists(path) or not entry.get("record_type"):
            continue
        for number, row in enumerate(rows_of(path), start=1):
            payload = {key: ("" if value is None else str(value)) for key, value in row.items() if key is not None}  # a row with extra cells has a None key
            stamp = next((parse_time(payload.get(c)) for c in TIME_COLUMNS if payload.get(c)), None) or NO_TIME_MARKER
            digest = hashlib.sha256((entry["path"] + str(number) + json.dumps(payload, sort_keys=True)).encode()).hexdigest()
            writer.writerow(["default", COLLECTION, entry["path"], entry["record_type"], stamp, entry["path"], number, digest, json.dumps(payload, ensure_ascii=False)])
            total += 1
    sys.stdout.write("COPY forensic.records (tenant_id, collection_id, file_id, record_type, \"timestamp\", source_file, row_number, row_hash, raw_payload) FROM STDIN WITH (FORMAT csv);\n")
    sys.stdout.write(buffer.getvalue())
    sys.stdout.write("\\.\n")
    sys.stdout.write("UPDATE forensic.records SET \"timestamp\" = ingested_at WHERE collection_id = '%s' AND \"timestamp\" = '%s';\n" % (COLLECTION, NO_TIME_MARKER))
    sys.stderr.write("rows written: %d\n" % total)


if __name__ == "__main__":
    main()
