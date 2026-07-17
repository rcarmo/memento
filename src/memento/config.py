from __future__ import annotations

from typing import Literal

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
    read_prefixes: tuple[str, ...] = Field(min_length=1)
    write_prefixes: tuple[str, ...] = Field(default_factory=tuple)

    @field_validator("roles", "read_prefixes", "write_prefixes")
    @classmethod
    def normalize_unique_items(cls, value: tuple[str, ...]) -> tuple[str, ...]:
        return tuple(sorted(dict.fromkeys(value)))

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


class IntelligentTiersConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    deep_answers: DeepAnswersConfig = Field(default_factory=DeepAnswersConfig)
    exact_answer_cache: ExactAnswerCacheConfig = Field(default_factory=ExactAnswerCacheConfig)
    hot_working_memory: HotWorkingMemoryConfig = Field(default_factory=HotWorkingMemoryConfig)


class ServiceConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    schema_version: Literal[1]
    repository: RepositoryConfig
    authorization: AuthorizationConfig
    limits: LimitsConfig = Field(default_factory=LimitsConfig)
    intelligent_tiers: IntelligentTiersConfig = Field(default_factory=IntelligentTiersConfig)
