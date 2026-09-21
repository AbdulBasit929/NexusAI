import hashlib

from ingestion.forensic_records.processor_readiness import (
    COMPLETE_RESULTS,
    COMPLETE_ZERO_RESULTS,
    MODEL_REQUIRED,
    NOT_RUN,
    READY,
    UNAVAILABLE,
    ModelAssetRequirement,
    assess_processor_readiness,
    processor_result_state,
)


def requirement(path, identity="model"):
    return ModelAssetRequirement(identity, path, hashlib.sha256(path.read_bytes()).hexdigest())


def test_missing_model_is_not_a_zero_result(tmp_path):
    receipt = assess_processor_readiness(
        role="image_anpr", processor_id="fastalpr-onnx-cpu", code_present=True,
        role_enabled=True, role_admitted=True, worker_healthy=True,
        resource_policy_satisfied=True,
        assets=[ModelAssetRequirement("detector", tmp_path / "missing.onnx", "0" * 64)],
    )
    assert receipt.state == MODEL_REQUIRED
    assert processor_result_state(receipt.state, started=False, completed=False, failed=False, observation_count=0) == MODEL_REQUIRED


def test_disabled_or_unadmitted_role_is_not_run(tmp_path):
    asset = tmp_path / "model.onnx"
    asset.write_bytes(b"model")
    receipt = assess_processor_readiness(
        role="ocr", processor_id="paddleocr", code_present=True,
        role_enabled=False, role_admitted=True, worker_healthy=True,
        resource_policy_satisfied=True, assets=[requirement(asset)],
    )
    assert receipt.state == NOT_RUN


def test_integrity_or_backend_failure_is_unavailable(tmp_path):
    asset = tmp_path / "model.onnx"
    asset.write_bytes(b"changed")
    receipt = assess_processor_readiness(
        role="video_anpr", processor_id="nxmmr-video-anpr-v2", code_present=True,
        role_enabled=True, role_admitted=True, worker_healthy=True,
        resource_policy_satisfied=True,
        assets=[ModelAssetRequirement("detector", asset, "f" * 64)],
        backend_modules=["module_that_does_not_exist_for_nexusai_test"],
    )
    assert receipt.state == UNAVAILABLE
    assert receipt.integrity_failures == ("detector",)


def test_ready_execution_distinguishes_zero_and_results(tmp_path):
    asset = tmp_path / "model.onnx"
    asset.write_bytes(b"model")
    receipt = assess_processor_readiness(
        role="image_anpr", processor_id="fastalpr-onnx-cpu", code_present=True,
        role_enabled=True, role_admitted=True, worker_healthy=True,
        resource_policy_satisfied=True, assets=[requirement(asset)],
    )
    assert receipt.state == READY
    assert processor_result_state(READY, started=True, completed=True, failed=False, observation_count=0) == COMPLETE_ZERO_RESULTS
    assert processor_result_state(READY, started=True, completed=True, failed=False, observation_count=2) == COMPLETE_RESULTS
