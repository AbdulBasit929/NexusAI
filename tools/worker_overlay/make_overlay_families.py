#!/usr/bin/env python3
"""Builds the Docker build context of the FAMILIES overlay: the worker image that is running now, plus the typed readers of the new families and the mention index of documents.

    docker tag nexusai-forensic-records-worker:latest nexusai-forensic-records-worker:before-families-YYYYMMDD     # the rollback
    docker cp nexusai-forensic-records-worker-1:/app/. LIVE_APP_DIR                                                  # the code that is running, read only
    python -X utf8 tools/worker_overlay/make_overlay_families.py LIVE_APP_DIR OUT_DIR [BASE_TAG]
    docker build --pull=false --network none -t nexusai-forensic-records-worker:families-YYYYMMDD OUT_DIR

What it changes in the running image and nothing else (the media overlays of 2026-10-21 and 22 stay as they are):
  * worker.py of the running image: the typed block (`run_typed_ingest_shadow` up to `validate_sample_profile`) is replaced by the repository's block (it offers .pdf and .docx exports to the
    typed writer as well, reports whether it loaded records, and adds `run_document_mentions_shadow` with its switch and its gazetteer), and the one call of the typed helper in the
    document branch of `process_job` becomes two lines: the typed helper, then the mention index, which skips a file the typed writer loaded as records; and the document route is decided
    by the route alone (`should_process_native_document`, taken from the repository), because the API sends a plain .txt document with the modality structured_records. Every other line of the
    running worker.py is untouched.
  * typed_ingest.py and the layout_registry package (the readers and layouts of plates, registrations, devices, statements and wallets, PDF and Word statements).
  * mention_extractor.py, document_mentions.py and gazetteers/ (new).
The base is the image that is running now (the rollback tag), never an older one. Standard library only; needs git-free access to the repository's ingestion folder.
"""
import os
import shutil
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
SOURCE = os.path.join(REPO, "ingestion", "forensic_records")

BLOCK_START = b"def run_typed_ingest_shadow("
BLOCK_END = b"def validate_sample_profile("


def make(live_app, out, base="nexusai-forensic-records-worker:latest"):
    live = open(os.path.join(live_app, "worker.py"), "rb").read()
    repo = open(os.path.join(SOURCE, "worker.py"), "rb").read()
    if b"run_document_mentions_shadow" in live:
        raise SystemExit("the running worker.py already has the mention hook; nothing to add (build from the image that is running, and check it is not the overlay itself)")
    eol = b"\r\n" if live.count(b"\r\n") > live.count(b"\n") // 2 else b"\n"
    for name, anchor in (("start", BLOCK_START), ("end", BLOCK_END)):
        if live.count(anchor) != 1 or repo.count(anchor) != 1:
            raise SystemExit("the %s of the typed block was not found exactly once in both worker.py files; review by hand" % name)
    new_block = repo[repo.index(BLOCK_START):repo.index(BLOCK_END)].replace(b"\r\n", b"\n").replace(b"\n", eol)
    patched = live[:live.index(BLOCK_START)] + new_block + live[live.index(BLOCK_END):]

    call_old = b"                run_typed_ingest_shadow_for_text(conn, job, path, document_profile)" + eol
    call_new = (b"                records_loaded = run_typed_ingest_shadow_for_text(conn, job, path, document_profile)" + eol +
                b"                run_document_mentions_shadow(conn, job, skip=records_loaded)" + eol)
    if patched.count(call_old) != 1:
        raise SystemExit("the typed call of the document branch was not found exactly once; the image has moved, so review by hand")
    patched = patched.replace(call_old, call_new, 1)

    # The route is authoritative (found by the first live load, 2026-10-30): the API sends a plain .txt DOCUMENT with the modality structured_records and the route native_document_worker, and the old rule
    # of the running worker (route AND modality in {document, text}) sent it to the generic row adapter: no passages, no mention index. The repository's rule is the route alone.
    route_old = b'            if route == "native_document_worker" and modality in {"document", "text"}:' + eol
    route_new = b"            if should_process_native_document(route):" + eol
    if patched.count(route_old) != 1 or b"def should_process_native_document(" in live:
        raise SystemExit("the document-route condition was not found exactly once in the running worker.py; the image has moved, so review by hand")
    patched = patched.replace(route_old, route_new, 1)
    start = repo.index(b"def should_process_native_document(")
    rule = repo[start:repo.index(b"def process_job(", start)].replace(b"\r\n", b"\n").rstrip(b"\n") + b"\n\n\n"
    if patched.count(b"def process_job(") != 1:
        raise SystemExit("process_job was not found exactly once; review by hand")
    patched = patched.replace(b"def process_job(", rule.replace(b"\n", eol) + b"def process_job(", 1)

    os.makedirs(out, exist_ok=True)
    open(os.path.join(out, "worker.py"), "wb").write(patched)
    for name in ("typed_ingest.py", "mention_extractor.py", "document_mentions.py"):
        shutil.copy(os.path.join(SOURCE, name), os.path.join(out, name))
    for name in ("layout_registry", "gazetteers"):
        target = os.path.join(out, name)
        if os.path.exists(target):
            shutil.rmtree(target)
        shutil.copytree(os.path.join(SOURCE, name), target, ignore=shutil.ignore_patterns("__pycache__"))
    with open(os.path.join(out, "Dockerfile"), "w", newline="\n") as handle:
        handle.write("# Families overlay: the worker image that is running, plus the typed readers of plates, registrations, devices and statements, and the mention index of documents.\n"
                     "ARG BASE=%s\n"
                     "FROM ${BASE}\n"
                     "COPY worker.py /app/worker.py\n"
                     "COPY typed_ingest.py /app/typed_ingest.py\n"
                     "COPY mention_extractor.py /app/mention_extractor.py\n"
                     "COPY document_mentions.py /app/document_mentions.py\n"
                     "COPY layout_registry /app/layout_registry\n"
                     "COPY gazetteers /app/gazetteers\n" % base)
    return len(patched.splitlines()) - len(live.splitlines())


if __name__ == "__main__":
    if len(sys.argv) not in (3, 4):
        print(__doc__)
        sys.exit(2)
    print("overlay context written; lines added to worker.py: %d" % make(sys.argv[1], sys.argv[2], *(sys.argv[3:4])))
