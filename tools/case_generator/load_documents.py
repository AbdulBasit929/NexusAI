"""Load the documents of a generated case into a THROWAWAY PostgreSQL the way the worker keeps them, and index their mentions with the PRODUCTION code (test-only fixture).

    stats = load_documents(db, root, key)

Per document: an evidence item, its storage object, version and extract run (the rows the foreign keys of forensic.derived_artifacts need), the passages as derived artifacts exactly as the
worker's persist_document_result writes them (the passages come from the production `extract_document`), and the mention rows from `ingestion/forensic_records/document_mentions.py` written through its
own `CursorSink` with the product's own INSERT statement (a cursor adapter replaces %s by SQL literals and runs the statements with psql). Nothing here is shown to the engine.
"""
import hashlib
import json
import os
import uuid
from pathlib import Path

import load_case
from ingestion.forensic_records import document_mentions
from ingestion.forensic_records.document_pipeline import DOCUMENT_PASSAGE_CONTRACT, extract_document

TENANT = load_case.TENANT


def literal(value):
    if value is None:
        return "NULL"
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, (int, float)):
        return str(value)
    return load_case.sql_text(str(value))


class LiteralCursor:
    """A DB-API cursor look-alike: execute(sql, params) fills the %s placeholders with SQL literals; flush() sends everything in one psql session."""

    def __init__(self, db):
        self.db, self.statements = db, []

    def execute(self, sql, params=()):
        self.statements.append((sql % tuple(literal(p) for p in params)) + ";")

    def flush(self):
        if self.statements:
            self.db.script("\n".join(["SELECT set_config('app.tenant_id', %s, false);" % load_case.sql_text(TENANT)] + self.statements))
        self.statements = []


def u(*parts):
    return str(uuid.uuid5(uuid.NAMESPACE_URL, "nexusai-doc/" + "/".join(parts)))


def load_documents(db, root, key, places=None):
    case_id = key["case_id"]
    stats = {}
    cursor = LiteralCursor(db)
    for entry in key["documents"]:
        rel = entry["file"]
        path = Path(root) / Path(*rel.split("/"))
        payload = path.read_bytes()
        sha = hashlib.sha256(payload).hexdigest()
        evidence_id = u(case_id, rel)
        version_id, run_id = u(case_id, rel, "version"), u(case_id, rel, "run")
        name = os.path.basename(rel)
        ext = os.path.splitext(rel)[1].lstrip(".")
        cursor.execute(
            "INSERT INTO forensic.evidence_items (evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension, size_bytes, sha256, modality, "
            "detected_type, classifier_confidence, processing_route, processing_status, raw_storage_ref, metadata) VALUES (%s::uuid, %s, %s, %s, 'case-loader', %s, %s, 'application/octet-stream', %s, %s, %s, "
            "'document', 'document', 1, 'document_native_text', 'completed', %s, jsonb_build_object('request_id', 'case-load', 'version_id', %s))",
            (evidence_id, TENANT, case_id, "SYNTHETIC-" + case_id, name, name, ext, len(payload), sha, "immutable://case/sha256/" + sha, version_id))
        # the evidence registration trigger (migration 008) creates the storage object, the version (the id given in the metadata) and the current-version link
        cursor.execute("INSERT INTO forensic.processing_runs (run_id, tenant_id, collection_id, evidence_id, version_id, run_kind, pipeline_id, idempotency_key, status, input_sha256) "
                       "VALUES (%s::uuid, %s, %s, %s::uuid, %s::uuid, 'extract', 'bounded-native-document-extractor', %s, 'succeeded', %s)", (run_id, TENANT, case_id, evidence_id, version_id, "load/" + evidence_id, sha))
        cursor.flush()
        result = extract_document(path, source_file=rel, evidence_id=evidence_id, version_id=version_id, source_sha256=sha)
        for passage in result.passages:
            metadata = {"passage_contract": DOCUMENT_PASSAGE_CONTRACT, "text": passage.text, "text_sha256": passage.text_sha256, "processor": result.processor_id,
                        "processor_revision": result.processor_revision, "source_truth_state": "derived_native_text", "native_text_only": True}
            cursor.execute(
                "INSERT INTO forensic.derived_artifacts (artifact_id, tenant_id, collection_id, evidence_id, version_id, run_id, artifact_type, media_type, processing_status, confidence, "
                "citation_locator, metadata, warnings) VALUES (%s::uuid, %s, %s, %s::uuid, %s::uuid, %s::uuid, %s, 'text/plain; charset=utf-8', 'completed', 1.0, %s::jsonb, %s::jsonb, '[]'::jsonb) "
                "ON CONFLICT (artifact_id) DO NOTHING",
                (passage.artifact_id, TENANT, case_id, evidence_id, version_id, run_id, DOCUMENT_PASSAGE_CONTRACT, json.dumps(passage.locator), json.dumps(metadata)))
        cursor.flush()
        rows = document_mentions.mention_rows(result.passages, places)
        written = document_mentions.CursorSink(cursor).write(TENANT, case_id, evidence_id, rows)
        cursor.flush()
        stats[name] = {"evidence_id": evidence_id, "passages": len(result.passages), "mentions": written, "by_kind": document_mentions.summarise(rows), "rows": rows}
    return stats
