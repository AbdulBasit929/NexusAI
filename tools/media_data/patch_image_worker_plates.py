#!/usr/bin/env python3
"""Patch the RUNNING worker image's own worker.py: the plate observation insert (docs/data/plates-m5-preregistration-20261110.md, Amendment 8).

    python -X utf8 patch_image_worker_plates.py <image's worker.py> <output path>

persist_anpr_media_observation inserts a row with `jsonb_build_object(... 'derived_observation_id', %s ...)`; PostgreSQL cannot type an untyped parameter inside that polymorphic function, so the
insert fails with `could not determine data type of parameter $12` for every plate observation. The path had never run in production (the plate role was off). The repository's worker.py already has the
fix (`%s::text`); the image's older file does not. This applies that one edit to the image's text (its CRLF line endings are kept) and refuses unless the anchor occurs exactly once.
"""
import sys

OLD = "'derived_observation_id', %s,"
NEW = "'derived_observation_id', %s::text,"


def main():
    source, target = sys.argv[1], sys.argv[2]
    text = open(source, encoding="utf-8", newline="").read()
    if text.count(OLD) != 1:
        sys.exit(f"anchor occurs {text.count(OLD)} times (expected 1); nothing written")
    open(target, "w", encoding="utf-8", newline="").write(text.replace(OLD, NEW))
    print("patched: 1 edit,", target)


if __name__ == "__main__":
    main()
