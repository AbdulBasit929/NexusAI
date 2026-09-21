from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Iterable, Mapping, Protocol


@dataclass(frozen=True)
class OperationDescriptor:
    id: str
    version: str
    family_id: str
    kind: str
    implementation: str
    required_tools: tuple[str, ...]
    input_contract: str
    output_contract: str
    citation_required: bool
    maturity: str = "LIMITED"
    required_inputs: tuple[str, ...] = ()
    accepted_target_kinds: tuple[str, ...] = ()
    case_wide_default: bool | None = None
    missing_input_behavior: str = ""


@dataclass(frozen=True)
class AdapterDescriptor:
    id: str
    version: str
    family_ids: tuple[str, ...]
    formats: tuple[str, ...]
    operation_ids: tuple[str, ...]
    implementation_key: str
    implementation_status: str
    configuration_schema_version: str
    output_contract_version: str
    resource_profile: str
    preserves_legacy_output: bool


@dataclass(frozen=True)
class ModelRoleReference:
    id: str
    kind: str
    configured_model: str
    status: str
    selection_policy: str


@dataclass(frozen=True)
class SpecialistAgentManifest:
    id: str
    version: str
    family_ids: tuple[str, ...]
    operation_ids: tuple[str, ...]
    tool_allowlist: tuple[str, ...]
    model_role_ids: tuple[str, ...]
    scope_source: str
    status: str
    response_contract: str
    required_citations: bool
    fallback_agent_id: str


@dataclass(frozen=True)
class PlatformCatalog:
    schema_version: str
    catalog_version: str
    query_plan_contract: str
    response_contract: str
    adapter_lifecycle: tuple[str, ...]
    adapters: tuple[AdapterDescriptor, ...]
    operations: tuple[OperationDescriptor, ...]
    model_roles: tuple[ModelRoleReference, ...]
    agents: tuple[SpecialistAgentManifest, ...]

    @classmethod
    def from_dict(cls, payload: Mapping[str, Any]) -> PlatformCatalog:
        return cls(
            schema_version=str(payload["schema_version"]),
            catalog_version=str(payload["catalog_version"]),
            query_plan_contract=str(payload["query_plan_contract"]),
            response_contract=str(payload["response_contract"]),
            adapter_lifecycle=tuple(payload["adapter_lifecycle"]),
            adapters=tuple(
                AdapterDescriptor(
                    **{
                        **item,
                        "family_ids": tuple(item["family_ids"]),
                        "formats": tuple(item["formats"]),
                        "operation_ids": tuple(item["operation_ids"]),
                    }
                )
                for item in payload["adapters"]
            ),
            operations=tuple(
                OperationDescriptor(
                    **{
                        **item,
                        "required_tools": tuple(item["required_tools"]),
                        "required_inputs": tuple(item.get("required_inputs", ())),
                        "accepted_target_kinds": tuple(item.get("accepted_target_kinds", ())),
                    }
                )
                for item in payload["operations"]
            ),
            model_roles=tuple(ModelRoleReference(**item) for item in payload["model_roles"]),
            agents=tuple(
                SpecialistAgentManifest(
                    **{
                        **item,
                        "family_ids": tuple(item["family_ids"]),
                        "operation_ids": tuple(item["operation_ids"]),
                        "tool_allowlist": tuple(item["tool_allowlist"]),
                        "model_role_ids": tuple(item["model_role_ids"]),
                    }
                )
                for item in payload["agents"]
            ),
        )


@dataclass(frozen=True)
class AdapterDetectionV1:
    score: float
    reasons: tuple[str, ...]
    schema_profile: Mapping[str, Any]
    warnings: tuple[str, ...]


@dataclass(frozen=True)
class AdapterValidationV1:
    status: str
    reasons: tuple[str, ...]
    warnings: tuple[str, ...]


@dataclass(frozen=True)
class AdapterProcessingStepV1:
    id: str
    lifecycle_stage: str
    operation_id: str
    resource_profile: str


@dataclass(frozen=True)
class AdapterOutputV1:
    observations: tuple[Mapping[str, Any], ...]
    artifacts: tuple[Mapping[str, Any], ...]
    warnings: tuple[str, ...]


@dataclass(frozen=True)
class AdapterVerificationV1:
    accounting: Mapping[str, int]
    provenance_complete: bool
    warnings: tuple[str, ...]


@dataclass(frozen=True)
class AdapterCapabilitiesV1:
    operation_ids: tuple[str, ...]
    unavailable_reasons: Mapping[str, str]


class FamilyAdapterLifecycle(Protocol):
    def descriptor(self) -> AdapterDescriptor: ...

    def detect(self, sample: Any, declared_hints: Mapping[str, str]) -> AdapterDetectionV1: ...

    def validate(self, source: Any, policy: Mapping[str, Any]) -> AdapterValidationV1: ...

    def plan(
        self,
        source: Any,
        case_capabilities: Mapping[str, Any],
    ) -> tuple[AdapterProcessingStepV1, ...]: ...

    def extract(self, source: Any) -> AdapterOutputV1: ...

    def normalize(self, observations: AdapterOutputV1) -> AdapterOutputV1: ...

    def derive(self, facts: AdapterOutputV1) -> AdapterOutputV1: ...

    def persist(self, outputs: AdapterOutputV1) -> Mapping[str, str]: ...

    def index(self, outputs: AdapterOutputV1) -> Mapping[str, str]: ...

    def verify(self, run: Any) -> AdapterVerificationV1: ...

    def capabilities(self, case: Any) -> AdapterCapabilitiesV1: ...


@dataclass(frozen=True)
class CompatibilityAdapterBinding:
    descriptor: AdapterDescriptor
    implementation: Any


class CompatibilityAdapterRegistry:
    """Binds the accepted worker implementations to versioned descriptors.

    The registry owns selection only. The legacy implementation still performs
    parsing, normalization, persistence, accounting, and provenance, which keeps
    Phase 5A free of output or storage changes.
    """

    def __init__(
        self,
        catalog: PlatformCatalog,
        implementations: Mapping[str, Any],
        normalizers: Mapping[str, Callable[..., Any]],
        normalize_name: Callable[[str], str],
    ) -> None:
        self.catalog = catalog
        self._implementations = implementations
        self._normalizers = normalizers
        self._normalize_name = normalize_name
        self._bindings = {
            descriptor.implementation_key: CompatibilityAdapterBinding(
                descriptor=descriptor,
                implementation=implementations[descriptor.implementation_key],
            )
            for descriptor in catalog.adapters
            if descriptor.implementation_key in implementations
        }

        missing_normalizers = set(self._bindings).difference(normalizers)
        if missing_normalizers:
            raise ValueError(f"missing compatibility normalizers: {sorted(missing_normalizers)}")

    def descriptor(self, implementation_key: str) -> AdapterDescriptor:
        return self._bindings[implementation_key].descriptor

    def normalize(self, implementation_key: str, *args: Any, **kwargs: Any) -> Any:
        return self._normalizers[implementation_key](*args, **kwargs)

    def resolve(
        self,
        requested_record_type: str,
        headers: Iterable[str],
        force_generic: bool = False,
    ) -> Any:
        requested = self._normalize_name(requested_record_type)
        if force_generic and requested in {"", "auto", "generic"}:
            return self._implementations["generic"]
        if requested in self._implementations and requested != "generic":
            implementation = self._implementations[requested]
            if implementation.matches(headers):
                return implementation
            raise ValueError(f"{requested} adapter selected but required columns are missing")
        for name, implementation in self._implementations.items():
            if name == "generic":
                continue
            if implementation.matches(headers):
                return implementation
        return self._implementations["generic"]


def platform_catalog_path() -> Path:
    container_path = Path(__file__).with_name("forensic-platform-v1.json")
    if container_path.is_file():
        return container_path
    return Path(__file__).resolve().parents[2] / "api" / "forensic_records" / "contracts" / "forensic-platform-v1.json"


def load_platform_catalog(path: Path | None = None) -> PlatformCatalog:
    catalog_path = path or platform_catalog_path()
    return PlatformCatalog.from_dict(json.loads(catalog_path.read_text(encoding="utf-8")))
