"""Truthful readiness and result-state contracts for specialist media roles.

The helpers are deliberately independent of model implementations.  They do
not discover or download assets and can therefore be used by route planning,
workers, tests, and activation preflight without changing evidence state.
"""

from __future__ import annotations

import hashlib
import importlib.util
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Iterable


READINESS_CONTRACT = "forensics.processor-readiness/v1"
RESULT_STATE_CONTRACT = "forensics.processor-result-state/v1"

NOT_RUN = "NOT_RUN"
PROCESSING = "PROCESSING"
COMPLETE_ZERO_RESULTS = "COMPLETE_ZERO_RESULTS"
COMPLETE_RESULTS = "COMPLETE_RESULTS"
FAILED = "FAILED"
UNAVAILABLE = "UNAVAILABLE"
MODEL_REQUIRED = "MODEL_REQUIRED"
READY = "READY"


@dataclass(frozen=True)
class ModelAssetRequirement:
    asset_id: str
    path: Path
    sha256: str


@dataclass(frozen=True)
class ProcessorReadinessReceipt:
    role: str
    processor_id: str
    state: str
    code_present: bool
    role_enabled: bool
    role_admitted: bool
    worker_healthy: bool
    resource_policy_satisfied: bool
    backend_available: bool
    model_assets_verified: bool
    missing_assets: tuple[str, ...]
    integrity_failures: tuple[str, ...]
    unavailable_backends: tuple[str, ...]
    contract_version: str = READINESS_CONTRACT

    def as_dict(self) -> dict[str, object]:
        value = asdict(self)
        value["missing_assets"] = list(self.missing_assets)
        value["integrity_failures"] = list(self.integrity_failures)
        value["unavailable_backends"] = list(self.unavailable_backends)
        return value


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def assess_processor_readiness(
    *,
    role: str,
    processor_id: str,
    code_present: bool,
    role_enabled: bool,
    role_admitted: bool,
    worker_healthy: bool,
    resource_policy_satisfied: bool,
    assets: Iterable[ModelAssetRequirement] = (),
    backend_modules: Iterable[str] = (),
) -> ProcessorReadinessReceipt:
    """Return one fail-closed readiness receipt without mutating runtime state."""

    missing: list[str] = []
    integrity: list[str] = []
    for asset in assets:
        if not asset.path.is_file():
            missing.append(asset.asset_id)
            continue
        if sha256_file(asset.path) != asset.sha256.lower():
            integrity.append(asset.asset_id)
    unavailable_backends = [
        name for name in backend_modules if importlib.util.find_spec(name) is None
    ]
    models_verified = not missing and not integrity
    backend_available = not unavailable_backends

    if not role_enabled or not role_admitted:
        state = NOT_RUN
    elif missing:
        state = MODEL_REQUIRED
    elif not code_present or integrity or not backend_available or not worker_healthy:
        state = UNAVAILABLE
    elif not resource_policy_satisfied:
        state = UNAVAILABLE
    else:
        state = READY
    return ProcessorReadinessReceipt(
        role=role,
        processor_id=processor_id,
        state=state,
        code_present=code_present,
        role_enabled=role_enabled,
        role_admitted=role_admitted,
        worker_healthy=worker_healthy,
        resource_policy_satisfied=resource_policy_satisfied,
        backend_available=backend_available,
        model_assets_verified=models_verified,
        missing_assets=tuple(missing),
        integrity_failures=tuple(integrity),
        unavailable_backends=tuple(unavailable_backends),
    )


def processor_result_state(
    readiness: str,
    *,
    started: bool,
    completed: bool,
    failed: bool,
    observation_count: int,
) -> str:
    """Map execution facts to the canonical state without inventing absence."""

    if observation_count < 0:
        raise ValueError("observation_count cannot be negative")
    if readiness == MODEL_REQUIRED:
        return MODEL_REQUIRED
    if readiness == UNAVAILABLE:
        return UNAVAILABLE
    if failed:
        return FAILED
    if not started:
        return NOT_RUN
    if not completed:
        return PROCESSING
    return COMPLETE_RESULTS if observation_count else COMPLETE_ZERO_RESULTS
