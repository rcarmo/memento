from __future__ import annotations

from datetime import datetime, timezone
from io import StringIO
from pathlib import Path
from typing import Any

import frontmatter
from ruamel.yaml import YAML

from memento.repository.schema import ConceptDocument, ConceptFrontmatter

_YAML = YAML()
_YAML.default_flow_style = False
_YAML.allow_unicode = True
_YAML.indent(mapping=2, sequence=4, offset=2)
_YAML.width = 4096

_FRONTMATTER_ORDER = (
    "schema_version",
    "id",
    "type",
    "title",
    "status",
    "description",
    "aliases",
    "tags",
    "source_refs",
    "supersedes",
    "created_at",
    "updated_at",
    "updated_by",
)


class FrontmatterError(Exception):
    """Raised when a concept file cannot be parsed or serialized safely."""


def parse_concept_text(text: str) -> ConceptDocument:
    try:
        post = frontmatter.loads(text)
    except Exception as exc:  # pragma: no cover - dependency boundary
        raise FrontmatterError("invalid frontmatter") from exc
    if not isinstance(post.metadata, dict):
        raise FrontmatterError("frontmatter metadata must be a mapping")
    try:
        model = ConceptFrontmatter.model_validate(post.metadata)
    except Exception as exc:
        raise FrontmatterError("frontmatter validation failed") from exc
    return ConceptDocument(frontmatter=model, body=_normalize_body(post.content))


def parse_concept_file(path: Path) -> ConceptDocument:
    return parse_concept_text(path.read_text(encoding="utf-8"))


def serialize_concept(document: ConceptDocument) -> str:
    metadata = _ordered_metadata(document.frontmatter)
    yaml_output = StringIO()
    _YAML.dump(metadata, yaml_output)
    body = _normalize_body(document.body)
    return f"---\n{yaml_output.getvalue()}---\n{body}\n"


def _ordered_metadata(model: ConceptFrontmatter) -> dict[str, Any]:
    metadata = model.model_dump(mode="python", exclude_none=True)
    ordered: dict[str, Any] = {}
    for key in _FRONTMATTER_ORDER:
        if key not in metadata:
            continue
        value = metadata[key]
        if isinstance(value, datetime):
            ordered[key] = _format_timestamp(value)
        else:
            ordered[key] = value
    return ordered


def _format_timestamp(value: datetime) -> str:
    return value.astimezone(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def _normalize_body(body: str) -> str:
    lines = [line.rstrip() for line in body.replace("\r\n", "\n").split("\n")]
    normalized = "\n".join(lines).strip("\n")
    return normalized
