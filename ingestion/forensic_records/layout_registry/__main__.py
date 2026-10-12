"""Command line for the layout registry.

    python -m ingestion.forensic_records.layout_registry list
    python -m ingestion.forensic_records.layout_registry validate
    python -m ingestion.forensic_records.layout_registry sql [--out FILE]

Run it from the repository root. `sql` writes the idempotent statements that record every entry in forensic.layout_versions
(the ops harness uses it on a throwaway database); nothing here connects to a database or reads a real file.
"""

from __future__ import annotations

import argparse
import sys

from .registry import LayoutRegistryError, load_registry


def _confirmation_items(entry: dict) -> int:
    return len((entry.get("confirmation") or {}).get("unconfirmed") or [])


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="python -m ingestion.forensic_records.layout_registry", description=__doc__.split("\n")[0])
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("list", help="one line per layout entry")
    commands.add_parser("validate", help="load and validate every entry; exit 1 on a problem")
    sql = commands.add_parser("sql", help="SQL that registers every entry in forensic.layout_versions")
    sql.add_argument("--out", help="write to this file (LF line endings) instead of standard output")
    args = parser.parse_args(argv)

    try:
        registry = load_registry()
    except LayoutRegistryError as exc:
        print("LAYOUTS=FAIL")
        for problem in str(exc).split("; "):
            print("  - " + problem)
        return 1

    if args.command == "validate":
        print("LAYOUTS=PASS entries=%d" % len(registry))
        return 0
    if args.command == "list":
        print("%-34s %3s %-9s %-14s %-17s %4s %s" % ("layout_id", "ver", "status", "family", "dataset_kind", "cols", "unconfirmed"))
        for entry in sorted(registry.entries, key=lambda e: (e["layout_id"], e["layout_version"])):
            print("%-34s %3d %-9s %-14s %-17s %4d %d" % (
                entry["layout_id"], entry["layout_version"], entry["status"], entry["family"], entry["dataset_kind"],
                len(entry["header"]["columns"]), _confirmation_items(entry)))
        return 0
    text = registry.register_sql()
    if args.out:
        with open(args.out, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(text)
    else:
        sys.stdout.write(text)
    return 0


if __name__ == "__main__":
    sys.exit(main())
