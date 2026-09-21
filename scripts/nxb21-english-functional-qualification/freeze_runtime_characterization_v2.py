#!/usr/bin/env python3
"""Freeze/verify the one nonsemantic v2 experiment without changing v1 evidence."""
import argparse
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
CONFIG = HERE / "current4b-runtime-characterization-freeze-v2.json"


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--prepare", action="store_true")
    args = parser.parse_args()
    if args.prepare:
        if CONFIG.exists():
            raise SystemExit("Freeze already exists; do not overwrite a frozen experiment")
        old = HERE / "current4b-resource-lifecycle-soak-freeze-v1.json"
        config = json.loads(old.read_text())
        config["contract_version"] = "nexusai.current4b-runtime-characterization-freeze/v2"
        config["frozen_at"] = "2026-09-10"
        config["soak_id"] = "nxb21-current4b-runtime-characterization-v2-20260910"
        config["runner"] = "scripts/nxb21-english-functional-qualification/run_current4b_runtime_characterization_v2.ps1"
        config["runner_sha256"] = digest(ROOT / config["runner"])
        config["resource"]["loaded_floor_gib"] = None
        config["resource"]["admission_basis"] = "Unchanged conservative preload admission; 4 GiB is diagnostic during this owner-authorized experiment, not a replacement operating floor"
        config["resource"]["experimental_abort_guards"] = {
            "physical_available_gib": 1.5,
            "commit_reserve_gib": 2.0,
            "wsl_swap_growth_kib": 262144,
            "paging_pages_per_second": 1024,
            "paging_duration_seconds": 15,
            "maximum_experiment_seconds": 1200,
            "basis": "Physical headroom reuses the existing isolated evaluator policy. Other limits are conservative experiment abort criteria, not measured model capability or an adopted product floor. No automatic floor adoption.",
        }
        # Fewer resets than v1, while retaining repeated calls and two reloads.
        config["batch_plan"] = [4, 4]
        config["tiny_calls"] = 8
        config["files"][str(old.relative_to(ROOT)).replace("\\", "/")] = digest(old)
        for name in ["run_disposable_current4b_resource_lifecycle_soak.ps1", "test_runtime_characterization_v2.ps1", Path(__file__).name]:
            path = HERE / name
            config["files"][path.relative_to(ROOT).as_posix()] = digest(path)
        prior = ROOT / "local-acceptance-models/nxb21-current4b-resource-lifecycle-soak/run-20260909T114835361Z/resource-lifecycle-receipt-v1.json"
        config["files"][prior.relative_to(ROOT).as_posix()] = digest(prior)
        CONFIG.write_text(json.dumps(config, indent=2) + "\n", encoding="utf-8")
        Path(str(CONFIG) + ".sha256").write_text(digest(CONFIG) + "\n")
    config = json.loads(CONFIG.read_text())
    assert digest(CONFIG) == Path(str(CONFIG) + ".sha256").read_text().strip()
    assert digest(ROOT / config["runner"]) == config["runner_sha256"]
    for path, expected in config["files"].items():
        assert digest(ROOT / path) == expected, path
    script = (ROOT / config["runner"]).read_text()
    assert "Measure-Object -Property rss_mib" not in script
    assert "Get-Telemetry ($Stage+'_inflight')" in script
    assert "NOT_ADOPTED_REQUIRES_MEASUREMENT_ADJUDICATION" in script
    assert "ENGLISH_FUNCTIONAL_LIVE" not in script
    assert "Stop-Process" not in script and "taskkill" not in script.lower()
    assert "english-functional-corpus-v2.json" not in script
    assert config["tiny_calls"] == sum(config["batch_plan"]) == 8
    assert config["resource"]["admission_floor_gib"] == 8.627
    assert config["resource"]["preload_floor_gib"] == 6.0
    print(json.dumps({"status": "PASS", "frozen_sha256": digest(CONFIG), "prior_evidence_unchanged": True, "live_inference": False}))


if __name__ == "__main__":
    main()
