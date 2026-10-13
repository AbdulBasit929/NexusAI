#!/usr/bin/env python3
"""Builds the Docker build context of the Ingest v2 OVERLAY image: the image that is running now, plus only the typed shadow writer.

    docker create --name nx-imgdump nexusai-forensic-records-worker:before-ingest-v2
    docker cp nx-imgdump:/app/. OLD_APP_DIR ; docker rm nx-imgdump
    python -X utf8 tools/worker_overlay/make_overlay.py OLD_APP_DIR OUT_DIR
    docker build --pull=false --network none -t nexusai-forensic-records-worker:ingest-v2-shadow-YYYYMMDD OUT_DIR

Why an overlay and not `docker build` of ingestion/forensic_records/Dockerfile (docs/data/ingest-v2.md section 8): the running worker image was built on
2026-08-23 and is seven weeks behind the repository (worker.py, media_pipeline.py, roman_urdu.py differ; multilingual_ocr.py, processor_readiness.py and the
video_anpr_*.py modules are not in it; requirements.txt has since grown). A full rebuild would ship all of that, and pip-install the new packages, together with
Ingest v2. The overlay changes nothing but Ingest v2: worker.py of the running image gets exactly the two hunks of the repository's worker.py (the call after
version 1's audit, and `run_typed_ingest_shadow`), and `typed_ingest.py` and the `layout_registry/` package are added. Standard library only; needs git and the repository.
"""
import os
import re
import shutil
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
SOURCE = os.path.join(REPO, "ingestion", "forensic_records")

CALL = rb"run_typed_ingest_shadow(conn, cur, job, path, profile)"
HUNK_ONE = re.compile(rb'(                        "evidence_id": job\.evidence_id,\r?\n                    \},\r?\n                \)\r?\n)'
                      rb'(            conn\.commit\(\)\r?\n            emit_progress\(progress_cb, job, "completed", stats, profile, started_at\))')


def make(old_app, out):
    old = open(os.path.join(old_app, "worker.py"), "rb").read()
    new = open(os.path.join(SOURCE, "worker.py"), "rb").read()
    eol = b"\r\n" if old.count(b"\r\n") > old.count(b"\n") // 2 else b"\n"
    start, end = new.index(b"def run_typed_ingest_shadow("), new.index(b"def validate_sample_profile(")
    function = new[start:end].replace(b"\r\n", b"\n").rstrip(b"\n") + b"\n\n\n"
    function = function.replace(b"\n", eol)
    if len(HUNK_ONE.findall(old)) != 1:
        raise SystemExit("the call site was not found exactly once in the old worker.py; the image has moved, so review by hand")
    patched = HUNK_ONE.sub(lambda m: m.group(1) + b"                " + CALL + eol + m.group(2), old, count=1)
    anchor = b"def validate_sample_profile("
    if patched.count(anchor) != 1:
        raise SystemExit("the insertion point for the function was not found exactly once")
    patched = patched.replace(anchor, function + anchor, 1)
    os.makedirs(out, exist_ok=True)
    open(os.path.join(out, "worker.py"), "wb").write(patched)
    shutil.copy(os.path.join(SOURCE, "typed_ingest.py"), os.path.join(out, "typed_ingest.py"))
    target = os.path.join(out, "layout_registry")
    if os.path.exists(target):
        shutil.rmtree(target)
    shutil.copytree(os.path.join(SOURCE, "layout_registry"), target, ignore=shutil.ignore_patterns("__pycache__"))
    with open(os.path.join(out, "Dockerfile"), "w", newline="\n") as handle:
        handle.write("# Ingest v2 overlay: the running worker image plus the typed shadow writer, nothing else changes.\n"
                     "ARG BASE=nexusai-forensic-records-worker:before-ingest-v2\n"
                     "FROM ${BASE}\n"
                     "COPY worker.py /app/worker.py\n"
                     "COPY typed_ingest.py /app/typed_ingest.py\n"
                     "COPY layout_registry /app/layout_registry\n")
    return patched.count(b"\n") - old.count(b"\n")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print(__doc__)
        sys.exit(2)
    print("overlay context written; lines added to worker.py: %d" % make(sys.argv[1], sys.argv[2]))
