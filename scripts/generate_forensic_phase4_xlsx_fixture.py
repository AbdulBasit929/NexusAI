"""Generate the versioned synthetic mixed-family XLSX acceptance fixture."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from ingestion.forensic_records.tests.test_phase4_xlsx import (
    mixed_family_xlsx_parts,
    write_xlsx,
)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    write_xlsx(args.output, mixed_family_xlsx_parts())
    print(f"Generated synthetic XLSX fixture: {args.output.name}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
