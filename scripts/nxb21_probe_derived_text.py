"""Bounded read-only derived-text diagnostics through the authorized public proxy.

Uses current retained artifacts as integration oracles, not recognition accuracy
oracles. Does not invoke Ask/Activity, synthesis, inference, uploads or reprocessing.
Run nxb21_reconcile.py first. All targets and full responses remain Git-ignored.
"""
import hashlib
import json
import re
import time
import urllib.error
import urllib.request
import uuid

from nxb21_reconcile import PRIVATE, PUBLIC, save


def text(artifact):
    metadata = artifact["metadata"]
    observation = metadata.get("observation", {})
    return (observation.get("raw_text") or observation.get("roman_urdu_text") or
            observation.get("text") or metadata.get("text") or "")


def main():
    snapshot = json.loads((PRIVATE / "snapshot.json").read_text(encoding="utf-8"))
    candidates = []
    for detail in snapshot["details"]:
        meta = detail["item"]["metadata"]
        version = meta.get("media_processing", {}).get("version_id") or meta.get("version_id")
        artifacts = [a for a in detail.get("derived_artifacts", [])
                     if a["processing_status"] == "completed" and a["version_id"] == version]
        candidates.append((detail, artifacts))
    raw_contract = "forensics.audio-timestamp-segment/v1"
    probes, private = [], []

    def run(label, detail, artifacts, term, template="audio_transcript_search", mode="exact", workspace=False,
            oracle="literal substring in retained observation", expected=True):
        body = {"collection_id": snapshot["case"], "query": "Find phrase: " + term,
                "template": template, "exact_term": term, "transcript_mode": mode,
                "max_kb_results": 20, "limit": 20}
        if not workspace:
            body["evidence_id"] = detail["evidence_id"]
            body["evidence_version_id"] = artifacts[0]["version_id"]
        request = urllib.request.Request("http://localhost:8080/api/records/forensic/query",
            data=json.dumps(body, ensure_ascii=False).encode(), headers={"Content-Type": "application/json"})
        start = time.monotonic()
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                result, status = json.load(response), response.status
        except urllib.error.HTTPError as error:
            result, status = json.loads(error.read()), error.code
        elapsed = round((time.monotonic() - start) * 1000, 2)
        evidence = result.get("evidence", {})
        rows = evidence.get("results", [])
        requested_ids = {a["artifact_id"] for a in artifacts}
        returned_ids = {r.get("metadata", {}).get("artifact_id") for r in rows}
        source_found = requested_ids.issubset(returned_ids)
        oracle_agrees = (source_found if expected else not rows) and status == 200
        result_state = evidence.get("result_state")
        state_consistent = ((result_state == "COMPLETE_RESULTS") if rows else
                            result_state in {"NO_EXACT_MATCH", "NO_MATCH", "COMPLETE_ZERO_RESULTS"})
        record = {"id": label, "http_status": status, "template": result.get("template"),
            "query_mode": evidence.get("query_mode"), "retrieval_state": evidence.get("result_state"),
            "answer_state": result.get("answer", {}).get("transcript_state") or result.get("answer", {}).get("ocr_state"),
            "returned_rows": len(rows), "requested_source_found": source_found,
            "expected_source_found": expected, "oracle": oracle,
            "oracle_agrees_with_retrieval": oracle_agrees,
            "result_state_consistent": state_consistent,
            "contract_pass": oracle_agrees and state_consistent,
            "latency_ms": elapsed, "query_sha256": hashlib.sha256(term.encode()).hexdigest(),
            "citation_count": sum(bool(r.get("citation")) for r in rows),
            "returned_source_owned": all(r.get("metadata", {}).get("evidence_id") == detail["evidence_id"] for r in rows) if not workspace else None,
            "synthesis_requested": False, "browser_ask_activity_tested": False}
        probes.append(record)
        private.append({"id": label, "request": body, "response": result,
                        "oracle_artifact_ids": sorted(requested_ids)})
        print(json.dumps(record))

    ur_detail, ur_artifacts = next((d, [a for a in aa if a["artifact_type"] == raw_contract])
        for d, aa in candidates if any(a["artifact_type"] == raw_contract and
            a["metadata"].get("observation", {}).get("language") == "ur" and len(text(a).split()) >= 10 for a in aa))
    ur = next(a for a in ur_artifacts if len(text(a).split()) >= 10)
    words = text(ur).split()
    phrase = " ".join(words[3:7])
    assert phrase in text(ur)
    run("urdu_full", ur_detail, [ur], text(ur))
    run("urdu_short_whole_words", ur_detail, [ur], phrase)
    run("urdu_workspace_short", ur_detail, [ur], phrase, workspace=True)
    # Deliberately cut inside a word: PHRASE_CONTAINS differs from the legacy
    # token-boundary behavior. This is not proof of the owner's original input.
    token = next(w for w in words if len(w) >= 5 and re.fullmatch(r"[\u0600-\u06ff]+", w))
    run("urdu_literal_inside_word", ur_detail, [ur], token[1:-1], oracle="literal inside-word substring; future PHRASE_CONTAINS contract")
    absent = "NXB21 absent " + uuid.uuid4().hex
    assert all(absent not in text(a) for _, aa in candidates for a in aa)
    run("urdu_absent_explicit_exact", ur_detail, [ur], absent, expected=False, oracle="absent from all captured text")
    run("urdu_absent_unclassified_mode", ur_detail, [ur], absent, mode="", expected=False,
        oracle="absent literal; missing semantic classification must not return full transcript")
    for lang in ["ur", "en"]:
        for detail, artifacts in candidates:
            segments = sorted([a for a in artifacts if a["artifact_type"] == raw_contract and
                a["metadata"].get("observation", {}).get("language") == lang],
                key=lambda a: a["citation_locator"].get("start_seconds", -1))
            pair = next(((a, b) for a, b in zip(segments, segments[1:])
                if a["run_id"] == b["run_id"] and a["citation_locator"].get("end_seconds") == b["citation_locator"].get("start_seconds")), None)
            if pair:
                a, b = pair
                term = " ".join(text(a).split()[-2:] + text(b).split()[:2])
                assert term not in text(a) and term not in text(b)
                run(lang + "_cross_segment", detail, [a, b], term,
                    oracle="last two plus first two words across touching same-run, same-version segments")
                if lang == "en":
                    run("english_short", detail, [a], " ".join(text(a).split()[2:5]))
                break
    for contract, template, label in [
        ("forensics.audio-roman-urdu-segment/v1", "audio_transcript_search", "roman_urdu"),
        ("forensics.image-ocr-observation/v1", "image_ocr_search", "ocr"),
        ("forensics.document-native-text-passage/v1", "document_search", "document")]:
        match = next(((d, a) for d, aa in candidates for a in aa if a["artifact_type"] == contract and len(text(a).split()) >= 3), None)
        if not match:
            probes.append({"id": label, "status": "NO_ELIGIBLE_CURRENT_ARTIFACT"})
            continue
        detail, artifact = match
        phrase = " ".join(text(artifact).split()[1:3])
        run(label + "_short", detail, [artifact], phrase, template=template)
        if label in ("document", "ocr"):
            run(label + "_absent", detail, [artifact], absent, template=template,
                mode="" if label == "document" else "exact", expected=False, oracle="absent from all captured text")
    save(PRIVATE / "derived-text-probes.json", private)
    save(PUBLIC / "derived-text-probe-results-v1.json", {"schema_version": "nexusai.nxb21.derived-text-probes/v1",
         "captured_at": snapshot["captured_at"], "scope": "read-only public query; no UI replay or model inference", "probes": probes})


if __name__ == "__main__":
    main()
