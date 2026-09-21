#!/usr/bin/env python3
"""Generate the reproducible MMV-1 inventory pack from repository contracts."""

from __future__ import annotations

import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "reports" / "mmv1-real-world-validation-20260824"


def write(name: str, value: object) -> None:
    (OUT / name).write_text(
        json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )


catalog = json.loads(
    (ROOT / "api/forensic_records/contracts/forensic-platform-v1.json").read_text(
        encoding="utf-8"
    )
)

family_labels = {
    "communications_cdr": "CDR",
    "network_ipdr": "IPDR",
    "subscriber_identity": "subscriber",
    "tower_location": "tower",
    "anpr_vehicles": "ANPR",
    "generic_tabular": "structured",
    "image_media": "image",
    "audio_text": "audio",
    "native_document": "document",
}

operation_rows = []
for operation in catalog["operations"]:
    family_id = operation["family_id"]
    operation_rows.append(
        {
            "operation_id": operation["id"],
            "family": family_labels.get(family_id, family_id),
            "description": f"Bounded {operation['id']} operation from the platform contract.",
            "parameters": operation["input_contract"],
            "result_schema": operation["output_contract"],
            "processor_dependency": operation["implementation"],
            "execution_kind": operation["kind"],
            "authorization_scope": "authenticated tenant + collection/case + evidence membership",
            "provenance_contract": "source-bound result; citation required"
            if operation["citation_required"]
            else "source-bound result; citation optional by contract",
            "data_quick_action": "available only when the deployed capability registry exposes this exact operation",
            "ask_template": "registry-backed; certification status governs exposure",
            "english_variants": "tracked in the query inventory",
            "roman_urdu_variants": "tracked in the query inventory",
            "urdu_variants": "tracked in the query inventory",
            "positive_fixture": "existing operation certification corpus where present",
            "negative_fixture": "scope/no-result certification corpus where present",
            "test_status": "reconciled_from_66_operation_contract_not_reexecuted_in_mmv1",
            "maturity": "current_registry_state_preserved",
        }
    )

target_operations = {
    "ANPR": {
        "anpr.sightings": "implemented",
        "anpr.exact_sightings": "alias_candidate_for_anpr.sightings_not_certified",
        "anpr.evidence_plates": "missing",
        "anpr.video_timeline": "alias_candidate_for_video.timeline_not_certified",
        "anpr.sightings_by_time": "missing",
        "anpr.source_summary": "missing",
    },
    "image": {
        "image.ocr": "alias_candidate_for_image.ocr_search_not_certified",
        "image.faces": "alias_candidate_for_face.detect_not_certified",
        "image.compare": "implemented_engineering_only",
        "image.exact_duplicate": "implemented_engineering_only",
        "image.near_duplicate": "implemented_engineering_only",
        "image.visual_similarity": "implemented_limited",
        "image.metadata": "implemented_limited",
        "image.source_summary": "missing",
    },
    "face": {
        "face.detect": "implemented_limited",
        "face.candidate_similarity": "alias_candidate_for_face.similarity_not_certified",
        "face.case_candidates": "alias_candidate_for_face.search_authorized_not_certified",
        "face.compare_observations": "missing",
    },
    "audio": {
        "audio.transcript": "alias_candidate_for_audio.transcript_search_not_certified",
        "audio.transcript_at_time": "missing",
        "audio.search_transcript": "implemented_as_audio.transcript_search_limited",
        "audio.roman_urdu": "implemented_limited",
        "audio.identifiers": "missing",
        "audio.source_summary": "missing",
    },
    "video": {
        "video.timeline": "implemented_limited",
        "video.anpr": "missing_public_operation_processor_observations_exist",
        "video.ocr": "missing_public_operation_processor_observations_exist",
        "video.faces": "planned",
        "video.transcript": "missing_public_operation_processor_observations_exist",
        "video.transcript_at_time": "missing",
        "video.search_transcript": "missing",
        "video.source_summary": "missing",
    },
    "document": {
        "document.text": "alias_candidate_for_document.extract_not_certified",
        "document.search": "implemented_limited",
        "document.pages": "missing",
        "document.passages": "missing_public_operation_artifacts_exist",
        "document.kb_retrieval": "missing_as_family_operation_kb_path_exists",
        "document.source_summary": "missing",
    },
}

write(
    "operation-registry.json",
    {
        "contract_version": "nexusai.mmv1.operation-inventory/v1",
        "source_catalog": "api/forensic_records/contracts/forensic-platform-v1.json",
        "current_operation_count": len(operation_rows),
        "current_operations": operation_rows,
        "mmv_target_reconciliation": target_operations,
        "claim_boundary": "Aliases and missing targets are inventory findings, not implemented operations.",
    },
)

query_rows = []
examples = {
    "anpr.sightings": (
        "Which plates were detected in this evidence?",
        "is evidence mein konsi number plates hain",
        "اس ثبوت میں کون سی نمبر پلیٹس ملی ہیں؟",
    ),
    "video.timeline": (
        "Show the source-bound video timeline.",
        "video ki timeline dikhao",
        "ویڈیو کی ٹائم لائن دکھائیں۔",
    ),
    "audio.search_transcript": (
        "Search the transcript for 03001234567.",
        "transcript mein 03001234567 dhoondo",
        "ٹرانسکرپٹ میں 03001234567 تلاش کریں۔",
    ),
    "image.ocr": (
        "What text was extracted from this image?",
        "is tasveer mein kya text mila",
        "اس تصویر میں کیا متن ملا؟",
    ),
    "document.search": (
        "Find NEXUS-DEMO-2026 in this document.",
        "document mein NEXUS-DEMO-2026 dhoondo",
        "دستاویز میں NEXUS-DEMO-2026 تلاش کریں۔",
    ),
}
for family, operations in target_operations.items():
    for operation_id, status in operations.items():
        english, roman, urdu = examples.get(
            operation_id,
            (
                f"Run {operation_id} for the current evidence.",
                f"current evidence ke liye {operation_id} chalao",
                f"موجودہ ثبوت کے لیے {operation_id} چلائیں۔",
            ),
        )
        query_rows.append(
            {
                "operation_id": operation_id,
                "family": family,
                "implementation_status": status,
                "canonical_english": english,
                "natural_english": english.replace("current", "selected"),
                "typo_variant": f"{english} [TYPO_CASE_PENDING]",
                "roman_urdu": roman,
                "urdu": urdu,
                "follow_up": "Now narrow that to the selected time/evidence when the operation supports it.",
                "missing_parameter": "Must clarify evidence/time/identifier when required.",
                "negative_query": "Use a deliberately absent identifier and return a cited no-result response.",
                "certification": "existing_contract_preserved"
                if status.startswith("implemented")
                else "not_certified",
                "runtime_execution": "not_replayed_in_mmv1_inventory_slice",
            }
        )

write(
    "query-template-inventory.json",
    {
        "contract_version": "nexusai.mmv1.query-template-inventory/v1",
        "rows": query_rows,
        "rule": "Templates express intent only; answers and identifiers must resolve from current authorized data.",
    },
)

families = [
    ("structured", "schema registry + canonical worker", "deterministic adapters", "accepted", "accepted", "66-op catalog coverage", "accepted bounded", "partial multilingual", "source-bound", "synthetic/retained accepted", "M3 partial", "cross-family breadth", "MMV-9"),
    ("CDR", "CDR adapters", "deterministic", "accepted", "accepted", "11 current ops", "accepted", "partial multilingual", "source-bound", "multi-schema accepted", "M3", "broader real providers", "MMV-9"),
    ("IPDR", "IPDR adapter", "deterministic", "accepted", "accepted", "9 current ops", "accepted", "partial multilingual", "source-bound", "single real-world schema limited", "M2", "multi-schema", "MMV-9"),
    ("subscriber", "subscriber adapter", "deterministic", "accepted", "accepted", "8 current ops", "accepted", "partial multilingual", "source-bound", "synthetic accepted", "M2", "provider breadth", "MMV-9"),
    ("tower", "tower adapter", "deterministic", "accepted", "accepted", "8 current ops", "accepted", "partial multilingual", "source-bound", "synthetic accepted", "M2", "real site exports", "MMV-9"),
    ("ANPR", "FastALPR + structured adapter", "YOLO v9 plate + CCT XS OCR", "positive non-retained", "retained image accepted", "10 current ops; target gaps", "bounded", "identifier-safe partial", "bbox/crop/time", "sample.mp4 positive", "M1", "sampling/Pakistan robustness", "MMV-2"),
    ("general image", "image pipeline", "Tesseract + SigLIP", "accepted limited", "accepted", "6 image ops", "bounded", "partial", "bbox/hash/artifact", "synthetic + retained", "M1", "real-world OCR/semantic set", "MMV-2"),
    ("OCR", "general image OCR", "Tesseract eng+urd", "Urdu baseline inadequate", "accepted limited", "image.ocr_search", "bounded", "Urdu weak", "word bbox", "MMV-1 Urdu pack", "M1 limited", "CER 0.436842", "MMV-3"),
    ("face", "YuNet/SFace", "YuNet + SFace", "accepted candidate-only", "accepted bounded", "3 current ops", "candidate semantics", "language neutral", "bbox/crop/model", "synthetic reviewed", "M1", "real lawful diversity/quality", "MMV-4"),
    ("visual similarity", "image embedding pipeline", "SigLIP", "accepted limited", "accepted ranking", "image.visual_similarity", "bounded", "Urdu text weak", "candidate provenance", "synthetic/retained", "M1", "calibration and larger sets", "MMV-4"),
    ("audio", "media ASR + Roman Urdu", "faster-whisper-small", "3-fixture measured", "audio detail source fix", "2 current ops; target gaps", "bounded", "Urdu/Roman partial", "time segments", "2 natural + 1 synthetic", "M1", "natural code-switch/noise/identifiers", "MMV-3"),
    ("video", "sampled-frame composition", "FastALPR/Tesseract/YuNet/SigLIP/ASR", "positive non-retained", "timeline accepted; source fix pending deploy", "2 current ops; target gaps", "bounded", "derived from processors", "frame/time/bbox/crop", "sample.mp4 positive", "M1", "sampling/tracking/public ops", "MMV-2"),
    ("documents", "native extract + KB", "native parsers/Tesseract where explicit", "accepted limited", "accepted", "2 current ops; target gaps", "bounded", "Urdu scan gap", "page/passage where available", "TXT/PDF/DOCX retained", "M1", "scanned Urdu/layout/pages", "MMV-5"),
    ("KB", "hybrid retrieval", "configured embedding/reranker roles", "accepted", "accepted", "evidence-scoped retrieval", "grounded", "multilingual partial", "chunk/source citations", "retained accepted", "M2", "document family operation contract", "MMV-5"),
]
fields = ["family", "processor", "model", "data", "ui", "operations", "ask", "queries", "citations", "real_world_test", "maturity", "gaps", "next_phase"]
write(
    "family-maturity-matrix.json",
    {
        "contract_version": "nexusai.mmv1.family-maturity-matrix/v1",
        "rows": [dict(zip(fields, row)) for row in families],
        "open_p0": 0,
        "open_foundational_p1": 0,
    },
)

write(
    "model-inventory-admission-matrix.json",
    {
        "contract_version": "nexusai.mmv1.model-inventory/v1",
        "rows": [
            {"task": "Pakistan ANPR", "current": "FastALPR YOLO v9 + CCT XS v2", "classification": "CURRENT_MODEL_ACCEPTED", "evidence": "sample.mp4 detected LN15ZZC at 25s"},
            {"task": "general image OCR", "current": "Tesseract 5 eng+urd", "classification": "CURRENT_MODEL_ACCEPTED_LIMITED", "evidence": "English/general product baseline remains review-required"},
            {"task": "Urdu OCR", "current": "Tesseract 5 eng+urd", "candidate": "PaddleOCR arabic_PP-OCRv5_mobile_rec", "source": "https://paddlepaddle.github.io/PaddleOCR/main/en/version3.x/algorithm/PP-OCRv5/PP-OCRv5_multi_languages.html", "license": "Apache-2.0 project; packaged model notice must be verified", "revision": None, "sha256": None, "size": "official table reports approximately 7.6 MB recognition model", "runtime": "PaddleOCR/PaddlePaddle adapter; offline after admission", "hardware": "CPU benchmark required", "fixtures": "MMV-1 Urdu OCR pack", "accuracy": "not_run", "latency": "not_run", "memory": "not_run", "security": "new dependency and model supply-chain review", "rollback": "retain Tesseract eng+urd", "classification": "CHALLENGER_NEEDED_M2", "decision": "admission incomplete; no download authorized"},
            {"task": "Urdu/Pakistani speech ASR", "current": "faster-whisper-small", "classification": "CURRENT_MODEL_ACCEPTED_LIMITED", "evidence": "natural WER 0.6000 and 0.3333; synthetic identifiers weak"},
            {"task": "audio diarization", "current": None, "classification": "RESEARCH_M3", "evidence": "not implemented"},
            {"task": "face candidate similarity", "current": "YuNet + SFace", "classification": "CURRENT_MODEL_ACCEPTED", "evidence": "candidate similarity only; never identity"},
            {"task": "visual embeddings", "current": "SigLIP base patch16-224", "classification": "CURRENT_MODEL_ACCEPTED_LIMITED", "evidence": "bounded candidate ranking; Urdu text-image weak"},
            {"task": "document layout/scanned Urdu", "current": "native extraction + Tesseract", "classification": "RESEARCH_M3", "evidence": "layout/scanned Urdu corpus absent"},
            {"task": "general VLM replacement", "current": None, "classification": "NOT_JUSTIFIED", "evidence": "specialist processors currently provide stronger governed facts"},
        ],
        "model_download_needed": "YES_FOR_FUTURE_URDU_OCR_CHALLENGER_ONLY",
        "download_performed": False,
    },
)

write(
    "real-world-test-data-manifest.json",
    {
        "contract_version": "nexusai.mmv1.real-world-test-data-manifest/v1",
        "entries": [
            {"id": "sample-video-positive-anpr", "path": "C:/Users/sheik/Downloads/sample.mp4", "sha256": "d470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9faFC83762d137ee".lower(), "size_bytes": 184407144, "duration_seconds": 60.010, "license_ownership": "USER_PROVIDED_OWNERSHIP_NOT_ATTESTED", "purpose": "positive video ANPR control", "retained": False, "ground_truth": "LN15ZZC at approximately 25 seconds is a model observation pending human review"},
            {"id": "fleurs-ur-pk-test-row-1", "sha256": "c499e06e26b8cbf37386b88bba26f37045439a775e964bdad8634376435a056e", "license": "CC BY 4.0", "purpose": "natural Urdu ASR"},
            {"id": "fleurs-ur-pk-validation-row-4", "sha256": "f016076b153c2ed6be21fa5f7a4bbc22b3e391d40080b9fa3de3b8a2dd517550", "license": "CC BY 4.0", "purpose": "second natural Urdu speaker ASR"},
            {"id": "synthetic-urdu-english-identifiers", "sha256": "606ffc7e281940b1b175fff98169a83a6a6ab7744ffd49f05849ac94318fd314", "license": "locally generated synthetic test fixture", "purpose": "mixed-language identifier pipeline"},
            {"id": "mmv1-urdu-ocr-generated-pack", "path": "reports/mmv1-real-world-validation-20260824/urdu-ocr-fixtures", "license": "locally generated test fixtures", "purpose": "Urdu OCR baseline", "ground_truth_separate": True},
        ],
        "coverage_gaps": ["natural Pakistani English", "natural Urdu-English code switch", "spontaneous Urdu", "moderate noise", "lawful real Urdu scene text"],
    },
)

write(
    "supplemental-video-retained-manifest-entry.json",
    {
        "contract_version": "nexusai.mmv1.supplemental-retained-manifest-entry/v1",
        "path": "C:/Users/sheik/Downloads/sample.mp4",
        "sha256": "d470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9faFC83762d137ee".lower(),
        "size_bytes": 184407144,
        "duration_seconds": 60.010,
        "license_ownership_status": "USER_PROVIDED_OWNERSHIP_NOT_ATTESTED",
        "purpose": "retained positive video ANPR acceptance",
        "expected_family": "video",
        "expected_processors": ["video metadata", "six sampled frames at 0/5/10/15/20/25 seconds", "FastALPR", "general OCR", "face", "SigLIP", "embedded-audio ASR"],
        "ground_truth": "LN15ZZC at approximately 25 seconds; MODEL_OBSERVATION_NOT_HUMAN_REVIEWED",
        "target": {"tenant": "default", "collection": "nexusai-multimodal-product-acceptance", "case": "nexusai-multimodal-product-acceptance"},
        "live_preflight_counts_2026_08_24": {"global_evidence": 51, "collection_evidence": 24, "global_jobs": 64, "global_artifacts": 441},
        "expected_after_one_upload": {"global_evidence": 52, "collection_evidence": 25, "global_jobs": 65, "artifact_delta_expected_from_unchanged_nonretained_output": 549, "global_artifacts_if_all_observations_persist": 990},
        "retained_mutation_performed": False,
    },
)

write(
    "gap-backlog-matrix.json",
    {
        "contract_version": "nexusai.mmv1.gap-backlog/v1",
        "counts": {"P0": 0, "foundational_P1": 0, "P2": 9, "P3": 4},
        "rows": [
            {"id": "MMV-P1-VIDEO-ZERO-LIMITATION", "priority": "P1", "status": "source_fixed", "phase": "MMV-1", "finding": "per-frame zero limitations contradicted later positive video results"},
            {"id": "MMV-P1-AUDIO-PRESENTATION", "priority": "P1", "status": "source_fixed", "phase": "MMV-1", "finding": "audio icon/readiness ignored MIME/family and persisted results"},
            {"id": "MMV-P2-ANPR-ROBUSTNESS", "priority": "P2", "status": "open", "phase": "MMV-2", "finding": "one positive 60-second video does not establish Pakistan robustness"},
            {"id": "MMV-P2-VIDEO-SAMPLING", "priority": "P2", "status": "open", "phase": "MMV-2", "finding": "six-frame 0-25 second sampling omits the remaining 35 seconds"},
            {"id": "MMV-P2-URDU-OCR", "priority": "P2", "status": "open", "phase": "MMV-3", "finding": "aggregate non-negative CER 0.436842 is inadequate"},
            {"id": "MMV-P2-ASR-IDENTIFIERS", "priority": "P2", "status": "open", "phase": "MMV-3", "finding": "spoken plate MN1367 was not preserved"},
            {"id": "MMV-P2-AUDIO-CORPUS", "priority": "P2", "status": "approval_gated", "phase": "MMV-3", "finding": "natural code switch, Pakistani English, spontaneous Urdu and noise are absent"},
            {"id": "MMV-P2-FACE-DIVERSITY", "priority": "P2", "status": "open", "phase": "MMV-4", "finding": "lawful diverse real-world face quality corpus absent"},
            {"id": "MMV-P2-DOCUMENT-URDU", "priority": "P2", "status": "open", "phase": "MMV-5", "finding": "scanned Urdu layout/page corpus absent"},
            {"id": "MMV-P2-OPERATION-GAPS", "priority": "P2", "status": "open", "phase": "MMV-6", "finding": "requested family operations are aliases or missing, not certified"},
            {"id": "MMV-P2-QUERY-CERTIFICATION", "priority": "P2", "status": "open", "phase": "MMV-7", "finding": "MMV target multilingual templates lack fresh runtime certification"},
            {"id": "MMV-P3-VIDEO-TRACKING", "priority": "P3", "status": "deferred", "phase": "MMV-12", "finding": "cross-frame tracking and deduplication"},
            {"id": "MMV-P3-DIARIZATION", "priority": "P3", "status": "deferred", "phase": "MMV-12", "finding": "speaker diarization is not implemented"},
            {"id": "MMV-P3-CROSS-MODAL-TIMELINE", "priority": "P3", "status": "deferred", "phase": "MMV-9", "finding": "typed cross-family timeline composition"},
            {"id": "MMV-P3-VLM", "priority": "P3", "status": "deferred", "phase": "MMV-12", "finding": "advanced general video reasoning is not justified yet"},
        ],
        "note": "The two P1 findings are corrected in source and are deployment/browser acceptance gates, not open foundational design defects.",
    },
)
