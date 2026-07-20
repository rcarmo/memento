from __future__ import annotations

import os
import stat
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import pytest
from pydantic import ValidationError

from memento.authz import (
    AuthorizationError,
    authorize_path,
    filter_authorized_paths,
    resolve_policy,
)
from memento.config import Principal, ServiceConfig
from memento.envelopes import error_envelope, success_envelope
from memento.repository.bundle import (
    audit_repository,
    generate_directory_indexes,
    generate_root_log,
    scan_bundle,
)
from memento.repository.frontmatter import (
    FrontmatterError,
    parse_concept_text,
    serialize_concept,
)
from memento.repository.links import extract_structural_links, rewrite_links_for_rename
from memento.repository.paths import PathSafetyError, validate_repository_write_path
from memento.repository.schema import ConceptDocument, ConceptFrontmatter

VALID_CONCEPT = """---
schema_version: 1
id: 5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d
type: instance
title: Smith
status: active
description: Primary Piclaw assistant instance.
aliases:
  - smith-piclaw
  - smith-piclaw
tags:
  - piclaw
  - assistant
created_at: 2026-07-16T19:00:00Z
updated_at: 2026-07-16T19:20:00Z
updated_by: rui/tablet
---
# Smith

Smith links to [Piclaw](/projects/piclaw.md#overview).
"""


def test_strict_config_and_authorization() -> None:
    config = ServiceConfig.model_validate(
        {
            "schema_version": 2,
            "repository": {"root_path": "/tmp/repo", "bundle_root": "/"},
            "authorization": {
                "principals": {
                    "smith": {
                        "roles": ["reader", "proposer"],
                        "token_env": "MEMENTO_TOKEN_SMITH",
                        "read_prefixes": ["/instances/", "/shared/"],
                        "write_prefixes": ["/instances/"],
                    }
                }
            },
        }
    )
    principal = Principal(name="smith", roles=("proposer", "reader"), metadata={"instance": "lxc"})
    policy = resolve_policy(config.authorization, principal)
    authorize_path(policy, "/instances/smith.md", action="read")
    authorize_path(policy, "/instances/smith.md", action="write")
    assert filter_authorized_paths(policy, ["/instances/a.md", "/secret/a.md"], action="read") == [
        "/instances/a.md"
    ]
    with pytest.raises(AuthorizationError):
        authorize_path(policy, "/secret/a.md", action="read")
    with pytest.raises(ValidationError):
        ServiceConfig.model_validate(
            {
                "schema_version": 2,
                "repository": {"root_path": "/tmp/repo", "bundle_root": "/"},
                "authorization": {"principals": {}},
                "extra": True,
            }
        )


def test_envelopes_are_strict() -> None:
    success = success_envelope({"ok": True}, repo_revision="abc", index_revision="abc")
    assert success.model_dump()["status"] == "success"
    error = error_envelope("validation_error", "bad request")
    assert error.model_dump()["status"] == "error"
    with pytest.raises(ValidationError):
        success.__class__.model_validate({"status": "success", "data": {}, "repo_revision": "a"})


def test_frontmatter_parse_and_deterministic_serialize() -> None:
    document = parse_concept_text(VALID_CONCEPT)
    serialized_once = serialize_concept(document)
    serialized_twice = serialize_concept(parse_concept_text(serialized_once))
    assert serialized_once == serialized_twice
    assert "  - assistant" in serialized_once
    assert serialized_once.endswith("\n")
    assert "smith-piclaw\n" in serialized_once


def test_concept_serialization_is_thread_safe() -> None:
    document = parse_concept_text(VALID_CONCEPT)
    expected = serialize_concept(document)
    with ThreadPoolExecutor(max_workers=8) as executor:
        outputs = list(executor.map(lambda _: serialize_concept(document), range(100)))
    assert outputs == [expected] * 100


def test_frontmatter_rejects_unknown_keys() -> None:
    invalid = VALID_CONCEPT.replace("updated_by: rui/tablet", "updated_by: rui/tablet\nunknown: no")
    with pytest.raises(FrontmatterError):
        parse_concept_text(invalid)


def test_extract_links_and_rewrite_rename() -> None:
    body = (
        "# Title\n\nSee [Piclaw](/projects/piclaw.md#overview) and ![Logo](/projects/piclaw.md).\n"
    )
    links = extract_structural_links(body)
    assert len(links) == 1
    assert links[0].href == "/projects/piclaw.md#overview"
    rewritten = rewrite_links_for_rename(
        body,
        old_path="/projects/piclaw.md",
        new_path="/projects/piclaw-core.md",
    )
    assert rewritten.changed is True
    assert "/projects/piclaw-core.md#overview" in rewritten.content
    assert "/projects/piclaw-core.md" in rewritten.content


def test_safe_path_containment_rejects_traversal_symlink_special_and_reserved(
    tmp_path: Path,
) -> None:
    (tmp_path / "instances").mkdir()
    safe = validate_repository_write_path(tmp_path, "/instances/smith.md")
    assert safe.absolute_path == tmp_path / "instances" / "smith.md"
    with pytest.raises(PathSafetyError):
        validate_repository_write_path(tmp_path, "/../etc/passwd")
    with pytest.raises(PathSafetyError):
        validate_repository_write_path(tmp_path, "/index.md")

    target = tmp_path / "target"
    target.mkdir()
    symlink_path = tmp_path / "linked"
    symlink_path.symlink_to(target, target_is_directory=True)
    with pytest.raises(PathSafetyError):
        validate_repository_write_path(tmp_path, "/linked/escape.md")

    fifo_path = tmp_path / "instances" / "events.md"
    os.mkfifo(fifo_path)
    try:
        assert stat.S_ISFIFO(os.lstat(fifo_path).st_mode)
        with pytest.raises(PathSafetyError):
            validate_repository_write_path(tmp_path, "/instances/events.md")
    finally:
        fifo_path.unlink()


def test_deterministic_index_and_log_generation(tmp_path: Path) -> None:
    bundle_root = tmp_path / "bundle"
    (bundle_root / "instances").mkdir(parents=True)
    (bundle_root / "projects").mkdir(parents=True)
    (bundle_root / "instances" / "smith.md").write_text(VALID_CONCEPT, encoding="utf-8")
    (bundle_root / "projects" / "piclaw.md").write_text(
        VALID_CONCEPT.replace("type: instance", "type: project")
        .replace("5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", "9d11af4d-381d-443d-9ca5-86bc61570b0a")
        .replace("title: Smith", "title: Piclaw")
        .replace("# Smith", "# Piclaw")
        .replace("Smith links to [Piclaw](/projects/piclaw.md#overview).", "Piclaw itself."),
        encoding="utf-8",
    )
    bundle = scan_bundle(bundle_root)
    indexes = generate_directory_indexes(bundle)
    log_output = generate_root_log(bundle)
    assert indexes["/"] == generate_directory_indexes(bundle)["/"]
    assert "[instances](/instances/index.md)" in indexes["/"]
    assert "[projects](/projects/index.md)" in indexes["/"]
    assert log_output == generate_root_log(bundle)
    assert "[Smith](/instances/smith.md)" in log_output


def test_generated_indexes_and_log_escape_markdown_titles_and_authors(tmp_path: Path) -> None:
    bundle_root = tmp_path / "bundle-escaped"
    (bundle_root / "projects").mkdir(parents=True)
    escaped = VALID_CONCEPT.replace("title: Smith", "title: Link [trap](x)").replace(
        "updated_by: rui/tablet", "updated_by: agent_*`demo`"
    )
    (bundle_root / "projects" / "escaped.md").write_text(escaped, encoding="utf-8")
    bundle = scan_bundle(bundle_root)
    indexes = generate_directory_indexes(bundle)
    log_output = generate_root_log(bundle)
    assert r"[Link \[trap\]\(x\)](/projects/escaped.md)" in indexes["/projects/"]
    assert "agent\\_\\*\\`demo\\`" in log_output


def test_repository_audit_reports_duplicates_and_broken_links(tmp_path: Path) -> None:
    bundle_root = tmp_path / "bundle"
    (bundle_root / "instances").mkdir(parents=True)
    (bundle_root / "projects").mkdir(parents=True)
    (bundle_root / "instances" / "smith.md").write_text(VALID_CONCEPT, encoding="utf-8")
    duplicate = VALID_CONCEPT.replace("/projects/piclaw.md#overview", "/projects/missing.md")
    duplicate = duplicate.replace("type: instance", "type: project")
    duplicate = duplicate.replace("title: Smith", "title: Duplicate")
    (bundle_root / "projects" / "dupe.md").write_text(duplicate, encoding="utf-8")
    audit = audit_repository(bundle_root)
    codes = {issue.code for issue in audit.issues}
    assert "duplicate_id" in codes
    assert "broken_link" in codes
    assert audit.generated_indexes["/"].startswith("# Index\n")
    assert audit.generated_log.startswith("# Mutation Log\n")


def test_concept_schema_validation_model() -> None:
    frontmatter = ConceptFrontmatter.model_validate(
        {
            "schema_version": 1,
            "id": "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d",
            "type": "instance",
            "title": "Smith",
            "status": "active",
            "aliases": ["smith", "smith"],
            "tags": ["z", "a"],
            "created_at": "2026-07-16T19:00:00Z",
            "updated_at": "2026-07-16T19:20:00Z",
            "updated_by": "rui/tablet",
        }
    )
    document = ConceptDocument(frontmatter=frontmatter, body="# Smith")
    assert document.frontmatter.aliases == ("smith",)
    assert document.frontmatter.tags == ("a", "z")
    with pytest.raises(ValidationError):
        ConceptFrontmatter.model_validate(
            {
                "schema_version": 2,
                "id": "x",
                "type": "invalid",
                "title": "Smith",
                "created_at": "2026-07-16T19:00:00Z",
                "updated_at": "2026-07-16T19:20:00Z",
                "updated_by": "rui/tablet",
            }
        )
    with pytest.raises(ValidationError):
        ConceptFrontmatter.model_validate(
            {
                "schema_version": 1,
                "id": "ok",
                "type": "instance",
                "title": "Bad\nTitle",
                "created_at": "2026-07-16T19:00:00Z",
                "updated_at": "2026-07-16T19:20:00Z",
                "updated_by": "rui/tablet",
            }
        )
    with pytest.raises(ValidationError):
        ConceptFrontmatter.model_validate(
            {
                "schema_version": 1,
                "id": "ok",
                "type": "instance",
                "title": "Smith",
                "created_at": "2026-07-16T19:00:00Z",
                "updated_at": "2026-07-16T19:20:00Z",
                "updated_by": "rui\noperator",
            }
        )
