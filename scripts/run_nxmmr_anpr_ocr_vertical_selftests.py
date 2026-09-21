#!/usr/bin/env python3
"""Dependency-free runner for the NX-MMR ANPR/OCR vertical unit contracts."""

from __future__ import annotations

import importlib.util
import sys
import tempfile
from pathlib import Path


REPOSITORY = Path(__file__).resolve().parents[1]
if str(REPOSITORY) not in sys.path:
    sys.path.insert(0, str(REPOSITORY))


def load(name: str, relative: str):
    spec = importlib.util.spec_from_file_location(name, REPOSITORY / relative)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


def main() -> None:
    readiness = load("test_processor_readiness", "ingestion/forensic_records/tests/test_processor_readiness.py")
    video = load("test_video_anpr_product_v2", "ingestion/forensic_records/tests/test_video_anpr_product_v2.py")
    ocr = load("test_multilingual_ocr", "ingestion/forensic_records/tests/test_multilingual_ocr.py")
    benchmark = load("test_video_v2_benchmark", "scripts/test_benchmark_nexusai_nxmmr_video_anpr_v2.py")
    with tempfile.TemporaryDirectory() as directory:
        root = Path(directory)
        readiness.test_missing_model_is_not_a_zero_result(root)
        readiness.test_disabled_or_unadmitted_role_is_not_run(root)
        readiness.test_integrity_or_backend_failure_is_unavailable(root)
        readiness.test_ready_execution_distinguishes_zero_and_results(root)
        ocr.test_paddle_multilingual_contract_preserves_unicode_order_and_identifiers(root)
        ocr.test_contextual_router_honors_explicit_anpr_without_ocr_fallback(root)
        ocr.test_extensionless_source_passes_decoded_pixels_and_preserves_bytes(root)
        ocr.test_bad_decode_fails_before_paddle(root)
        ocr.test_extensionless_and_named_inputs_have_same_ocr_contract(root)
        ocr.test_automatic_zero_anpr_falls_back_to_extensionless_ocr(root)
        ocr.test_mixed_script_superset_is_preserved_when_partial_english_has_higher_confidence(root)
    video.test_v2_selects_an_actually_observed_candidate_without_character_synthesis()
    video.test_v2_rejects_singletons_short_noise_and_weak_support()
    video.test_v2_association_is_short_lived_and_source_local()
    with tempfile.TemporaryDirectory() as directory:
        video.test_v2_owns_video_anpr_without_legacy_resampling(Path(directory))
    ocr.test_script_direction_does_not_reverse_identifier_text()
    ocr.test_mixed_script_superset_outside_confidence_tolerance_does_not_override_winner()
    ocr.test_non_superset_mixed_candidate_does_not_replace_higher_confidence_text()
    benchmark.test_rank_prefers_exact_then_recall_then_group_precision()
    print("NX-MMR ANPR/OCR vertical self-tests: 19 passed")


if __name__ == "__main__":
    main()
