from __future__ import annotations

from typing import Literal
from urllib.parse import urlparse

from pydantic import BaseModel, ConfigDict, Field, field_validator


class Principal(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    name: str = Field(min_length=1)
    roles: tuple[str, ...] = Field(min_length=1)
    metadata: dict[str, str] = Field(default_factory=dict)

    @field_validator("roles")
    @classmethod
    def validate_roles(cls, value: tuple[str, ...]) -> tuple[str, ...]:
        normalized = tuple(sorted(dict.fromkeys(value)))
        if not normalized:
            raise ValueError("roles must not be empty")
        return normalized


class NamespacePolicy(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    roles: tuple[str, ...] = Field(min_length=1)
    token_env: str = Field(min_length=1)
    read_prefixes: tuple[str, ...] = Field(min_length=1)
    write_prefixes: tuple[str, ...] = Field(default_factory=tuple)

    @field_validator("roles", "read_prefixes", "write_prefixes")
    @classmethod
    def normalize_unique_items(cls, value: tuple[str, ...]) -> tuple[str, ...]:
        return tuple(sorted(dict.fromkeys(value)))

    @field_validator("token_env")
    @classmethod
    def validate_token_env(cls, value: str) -> str:
        normalized = value.strip()
        if not normalized:
            raise ValueError("token_env must not be empty")
        return normalized

    @field_validator("read_prefixes", "write_prefixes")
    @classmethod
    def validate_prefixes(cls, value: tuple[str, ...]) -> tuple[str, ...]:
        for item in value:
            if not item.startswith("/") or not item.endswith("/"):
                raise ValueError("namespace prefixes must start and end with '/'")
        return value


class AuthorizationConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    principals: dict[str, NamespacePolicy]


class RepositoryConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    root_path: str = Field(min_length=1)
    bundle_root: str = "/"

    @field_validator("bundle_root")
    @classmethod
    def validate_bundle_root(cls, value: str) -> str:
        if value != "/":
            raise ValueError("bundle_root must be '/'")
        return value


class LimitsConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    max_concept_bytes: int = Field(default=262_144, ge=1)
    max_search_results: int = Field(default=100, ge=1)


class MCPExecuteLimitsConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    max_operations: int = Field(default=12, ge=1, le=32)
    max_intermediates: int = Field(default=12, ge=1, le=64)
    max_records: int = Field(default=50, ge=1, le=500)
    max_output_bytes: int = Field(default=65_536, ge=512)
    max_time_seconds: float = Field(default=3.0, gt=0, le=30)


class MCPConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    tool_surface: Literal["compact", "standard", "read_only", "curator", "admin"] = "compact"
    compact_answer_enabled: bool = True
    max_request_bytes: int = Field(default=72 * 1024 * 1024, ge=4 * 1024 * 1024)
    allowed_origins: tuple[str, ...] = ()
    execute: MCPExecuteLimitsConfig = Field(default_factory=MCPExecuteLimitsConfig)

    @field_validator("allowed_origins")
    @classmethod
    def validate_allowed_origins(cls, value: tuple[str, ...]) -> tuple[str, ...]:
        normalized = tuple(sorted(dict.fromkeys(item.strip() for item in value)))
        for item in normalized:
            parsed = urlparse(item)
            if (
                parsed.scheme not in {"http", "https"}
                or not parsed.hostname
                or parsed.path not in {"", "/"}
                or parsed.query
                or parsed.fragment
            ):
                raise ValueError("allowed origins must be HTTP(S) origins without paths")
        return normalized


class DeepAnswerLimitsConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    max_steps: int = Field(default=8, ge=1, le=12)
    max_time_seconds: float = Field(default=3.0, gt=0)
    max_concepts: int = Field(default=6, ge=1)
    max_chars: int = Field(default=12_000, ge=256)
    max_answer_chars: int = Field(default=2_000, ge=32)


class DeepAnswersConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    enabled: bool = False
    model_policy_revision: str = Field(default="disabled", min_length=1)
    prompt_version: str = Field(default="v1", min_length=1)
    tool_version: str = Field(default="v1", min_length=1)
    trace_max_entries: int = Field(default=50, ge=1)
    trace_max_age_days: int = Field(default=30, ge=1)
    limits: DeepAnswerLimitsConfig = Field(default_factory=DeepAnswerLimitsConfig)


class ExactAnswerCacheConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    enabled: bool = False
    ttl_seconds: int = Field(default=86_400, ge=1)
    max_entries: int = Field(default=200, ge=1)


class HotWorkingMemoryConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    enabled: bool = False
    ttl_seconds: int = Field(default=3_600, ge=1)
    max_changed_concepts: int = Field(default=10, ge=1, le=10)
    max_answers: int = Field(default=10, ge=1, le=10)
    max_excerpt_chars: int = Field(default=4_000, ge=128)


class ModelProposalLimitsConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    max_search_results: int = Field(default=5, ge=1, le=10)
    max_consulted_concepts: int = Field(default=6, ge=1, le=10)
    max_context_chars: int = Field(default=12_000, ge=512)
    max_output_chars: int = Field(default=8_000, ge=256)
    max_diff_chars: int = Field(default=2_000_000, ge=1)
    max_changes: int = Field(default=20, ge=1, le=100)
    max_body_chars: int = Field(default=32_000, ge=1)
    max_rationale_chars: int = Field(default=4_000, ge=1)
    max_secret_entropy_chars: int = Field(default=32, ge=8)


class ModelProposalsConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    enabled: bool = False
    model_policy_revision: str = Field(default="disabled", min_length=1)
    prompt_version: str = Field(default="v1", min_length=1)
    tool_version: str = Field(default="v1", min_length=1)
    limits: ModelProposalLimitsConfig = Field(default_factory=ModelProposalLimitsConfig)


class DreamBudgetsConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    max_signals_per_run: int = Field(default=25, ge=1, le=100)
    max_model_proposals_per_run: int = Field(default=5, ge=0, le=20)
    max_runtime_seconds: float = Field(default=30.0, gt=0)
    daily_proposal_limit: int = Field(default=20, ge=0, le=200)


class DreamScannerConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    oversized_body_chars: int = Field(default=6000, ge=256)
    oversized_top_level_sections: int = Field(default=6, ge=1)
    max_oversized_candidates: int = Field(default=3, ge=1, le=3)
    duplicate_similarity_threshold: float = Field(default=0.86, ge=0.5, le=1.0)


class DreamConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    mode: Literal["disabled", "report_only", "propose"] = "disabled"
    model_policy_revision: str = Field(default="disabled", min_length=1)
    prompt_version: str = Field(default="v1", min_length=1)
    tool_version: str = Field(default="v1", min_length=1)
    interval_seconds: int = Field(default=21600, ge=300)
    quiet_period_seconds: int = Field(default=300, ge=0)
    scanner: DreamScannerConfig = Field(default_factory=DreamScannerConfig)
    budgets: DreamBudgetsConfig = Field(default_factory=DreamBudgetsConfig)


class ModelEndpointConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    base_url: str = Field(min_length=1)
    api_format: Literal["openai", "anthropic"]
    api_key_env: str | None = None
    model: str = Field(min_length=1)
    headers: dict[str, str] = Field(default_factory=dict)

    @field_validator("base_url")
    @classmethod
    def validate_base_url(cls, value: str) -> str:
        parsed = urlparse(value)
        if parsed.scheme not in {"http", "https"} or not parsed.netloc:
            raise ValueError("base_url must be an absolute http or https URL")
        return value.rstrip("/")

    @field_validator("api_key_env")
    @classmethod
    def validate_api_key_env(cls, value: str | None) -> str | None:
        if value is None:
            return None
        normalized = value.strip()
        if not normalized:
            raise ValueError("api_key_env must not be empty")
        return normalized

    @field_validator("headers")
    @classmethod
    def validate_headers(cls, value: dict[str, str]) -> dict[str, str]:
        normalized: dict[str, str] = {}
        for key, item in value.items():
            header = key.strip()
            if not header:
                raise ValueError("header names must not be empty")
            normalized[header] = item
        return normalized


class ModelSlotConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    primary: ModelEndpointConfig | None = None
    fallbacks: tuple[ModelEndpointConfig, ...] = ()
    timeout_seconds: float = Field(default=3.0, gt=0, le=120)
    max_output_chars: int = Field(default=2000, ge=32)
    retry_budget: int = Field(default=0, ge=0, le=5)
    concurrency_limit: int = Field(default=1, ge=1, le=32)
    allowed_data_classifications: tuple[str, ...] = Field(default=("internal",), min_length=1)
    allow_cross_trust_boundary: bool = False
    fallback_enabled: bool = True
    fallback_on_rate_limit: bool = False
    overload_status_codes: tuple[int, ...] = Field(default=(529,), max_length=16)

    @field_validator("allowed_data_classifications", mode="before")
    @classmethod
    def normalize_allowed_data_classifications(cls, value: object) -> tuple[str, ...]:
        if value is None:
            return ("internal",)
        if not isinstance(value, (list, tuple, set, frozenset)):
            raise ValueError("allowed_data_classifications must be a sequence")
        items = tuple(str(item).strip() for item in value if str(item).strip())
        if not items:
            raise ValueError("allowed_data_classifications must not be empty")
        return tuple(dict.fromkeys(items))

    @field_validator("overload_status_codes")
    @classmethod
    def validate_overload_status_codes(cls, value: tuple[int, ...]) -> tuple[int, ...]:
        for item in value:
            if item < 100 or item > 599:
                raise ValueError("overload status codes must be valid HTTP status codes")
        return tuple(dict.fromkeys(value))


class ModelProviderSlotsConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    hot_query: ModelSlotConfig = Field(
        default_factory=lambda: ModelSlotConfig(fallback_enabled=True)
    )
    deep_query: ModelSlotConfig = Field(
        default_factory=lambda: ModelSlotConfig(fallback_enabled=True)
    )
    proposal: ModelSlotConfig = Field(
        default_factory=lambda: ModelSlotConfig(fallback_enabled=False)
    )
    dream: ModelSlotConfig = Field(default_factory=lambda: ModelSlotConfig(fallback_enabled=False))


class SemanticSearchConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    enabled: bool = False
    worker_mode: Literal["subprocess", "in_process"] = "subprocess"
    worker_path: str = "/usr/local/bin/memento-embed"
    worker_timeout_seconds: float = Field(default=300.0, gt=0)
    ffi_library_path: str | None = None
    sqlite_extension_path: str | None = None
    model_path: str | None = None
    model_id: str = Field(default="rust-gte", min_length=1)
    dimensions: int = Field(default=384, ge=1)
    max_input_chars: int = Field(default=4096, ge=1)
    max_batch_size: int = Field(default=16, ge=1)
    max_candidates: int = Field(default=200, ge=1)
    default_search_mode: Literal["lexical", "semantic", "hybrid"] = "lexical"
    refresh_on_startup: bool = True

    @field_validator("ffi_library_path", "sqlite_extension_path", "model_path")
    @classmethod
    def validate_optional_paths(cls, value: str | None) -> str | None:
        if value is None:
            return None
        normalized = value.strip()
        if not normalized:
            raise ValueError("path values must not be empty")
        return normalized

    @field_validator("worker_path")
    @classmethod
    def validate_worker_path(cls, value: str) -> str:
        normalized = value.strip()
        if not normalized:
            raise ValueError("worker_path must not be empty")
        return normalized

    @field_validator("model_id")
    @classmethod
    def validate_model_id(cls, value: str) -> str:
        normalized = value.strip()
        if not normalized:
            raise ValueError("model_id must not be empty")
        return normalized


class NeedleRouterConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    enabled: bool = False
    ffi_library_path: str = "/usr/local/lib/memento/libmemento_needle_ffi.so"
    model_path: str = "/usr/local/share/memento/models/memento-router.ndl"
    tokenizer_path: str = "/usr/local/share/memento/models/needle.model"

    @field_validator("ffi_library_path", "model_path", "tokenizer_path")
    @classmethod
    def validate_paths(cls, value: str) -> str:
        normalized = value.strip()
        if not normalized:
            raise ValueError("path values must not be empty")
        return normalized


class IntelligentTiersConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    deep_answers: DeepAnswersConfig = Field(default_factory=DeepAnswersConfig)
    exact_answer_cache: ExactAnswerCacheConfig = Field(default_factory=ExactAnswerCacheConfig)
    hot_working_memory: HotWorkingMemoryConfig = Field(default_factory=HotWorkingMemoryConfig)
    model_proposals: ModelProposalsConfig = Field(default_factory=ModelProposalsConfig)
    dream: DreamConfig = Field(default_factory=DreamConfig)
    model_provider_slots: ModelProviderSlotsConfig = Field(default_factory=ModelProviderSlotsConfig)
    semantic_search: SemanticSearchConfig = Field(default_factory=SemanticSearchConfig)
    needle_router: NeedleRouterConfig = Field(default_factory=NeedleRouterConfig)


class GraphExplorerConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    enabled: bool = False
    route_prefix: str = "/graph"
    direct_node_limit: int = Field(default=2_000, ge=1, le=2_000)
    overview_cluster_limit: int = Field(default=500, ge=1, le=1_000)
    expansion_node_limit: int = Field(default=2_000, ge=1, le=2_000)
    edge_limit: int = Field(default=12_000, ge=1, le=20_000)
    preview_chars: int = Field(default=4_000, ge=0, le=16_000)
    semantic_neighbours: int = Field(default=12, ge=1, le=100)
    export_node_limit: int = Field(default=2_000, ge=1, le=2_000)
    refresh_max_paths: int = Field(default=2_000, ge=1, le=10_000)

    @field_validator("route_prefix")
    @classmethod
    def validate_route_prefix(cls, value: str) -> str:
        normalized = value.strip()
        if (
            not normalized.startswith("/")
            or normalized == "/"
            or normalized.endswith("/")
            or "?" in normalized
            or "#" in normalized
            or ".." in normalized.split("/")
        ):
            raise ValueError(
                "graph route_prefix must be an absolute non-root path without a trailing slash"
            )
        return normalized


class ObservabilityConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    graph_explorer: GraphExplorerConfig = Field(default_factory=GraphExplorerConfig)


class ServiceConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    schema_version: Literal[2]
    repository: RepositoryConfig
    authorization: AuthorizationConfig
    limits: LimitsConfig = Field(default_factory=LimitsConfig)
    mcp: MCPConfig = Field(default_factory=MCPConfig)
    intelligent_tiers: IntelligentTiersConfig = Field(default_factory=IntelligentTiersConfig)
    observability: ObservabilityConfig = Field(default_factory=ObservabilityConfig)
