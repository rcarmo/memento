from __future__ import annotations

from datetime import UTC, datetime
from enum import StrEnum
from uuid import uuid4

from pydantic import BaseModel, ConfigDict, Field, field_validator


def _validate_plain_text(value: str, *, field_name: str) -> str:
    if any(ord(char) < 32 or ord(char) == 127 for char in value):
        raise ValueError(f"{field_name} must not contain control characters")
    if "\n" in value or "\r" in value:
        raise ValueError(f"{field_name} must not contain newlines")
    return value


CONTROLLED_TYPES = (
    "concept",
    "instance",
    "person",
    "project",
    "service",
    "system",
)


class ConceptStatus(StrEnum):
    ACTIVE = "active"
    DEPRECATED = "deprecated"
    TOMBSTONE = "tombstone"


class ConceptFrontmatter(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True, use_enum_values=True)

    schema_version: int = Field(default=1)
    id: str = Field(min_length=1)
    type: str = Field(min_length=1)
    title: str = Field(min_length=1)
    status: ConceptStatus = ConceptStatus.ACTIVE
    description: str | None = None
    aliases: tuple[str, ...] = ()
    tags: tuple[str, ...] = ()
    source_refs: tuple[str, ...] = ()
    supersedes: tuple[str, ...] = ()
    created_at: datetime
    updated_at: datetime
    updated_by: str = Field(min_length=1)

    @field_validator("schema_version")
    @classmethod
    def validate_schema_version(cls, value: int) -> int:
        if value != 1:
            raise ValueError("schema_version must be 1")
        return value

    @field_validator("type")
    @classmethod
    def validate_type(cls, value: str) -> str:
        if value not in CONTROLLED_TYPES:
            raise ValueError(f"type must be one of: {', '.join(CONTROLLED_TYPES)}")
        return value

    @field_validator("title", "updated_by")
    @classmethod
    def validate_plain_text_fields(cls, value: str, info: object) -> str:
        field_name = getattr(info, "field_name", "value")
        return _validate_plain_text(value, field_name=field_name)

    @field_validator("aliases", "tags", "source_refs", "supersedes")
    @classmethod
    def normalize_string_tuples(cls, value: tuple[str, ...]) -> tuple[str, ...]:
        normalized = tuple(sorted(item for item in dict.fromkeys(value) if item))
        return normalized

    @field_validator("created_at", "updated_at")
    @classmethod
    def require_utc(cls, value: datetime) -> datetime:
        if value.tzinfo is None:
            raise ValueError("timestamps must be timezone-aware")
        return value.astimezone(UTC)


class ConceptDocument(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    frontmatter: ConceptFrontmatter
    body: str


def new_concept_frontmatter(
    *, title: str, concept_type: str, updated_by: str
) -> ConceptFrontmatter:
    now = datetime.now(tz=UTC).replace(microsecond=0)
    return ConceptFrontmatter(
        id=str(uuid4()),
        type=concept_type,
        title=title,
        created_at=now,
        updated_at=now,
        updated_by=updated_by,
    )
