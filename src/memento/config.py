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


class ServiceConfig(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    schema_version: Literal[1]
    repository: RepositoryConfig
    authorization: AuthorizationConfig
    limits: LimitsConfig = Field(default_factory=LimitsConfig)
