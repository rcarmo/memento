from __future__ import annotations

import json
import sqlite3
from collections.abc import Generator
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, cast

import pytest

from memento.answers import UNKNOWN_ANSWER, ModelRequest, ModelResponse
from memento.config import (
    AuthorizationConfig,
    DeepAnswerLimitsConfig,
    DeepAnswersConfig,
    ExactAnswerCacheConfig,
    HotWorkingMemoryConfig,
    IntelligentTiersConfig,
    NamespacePolicy,
    Principal,
    RepositoryConfig,
    ServiceConfig,
)
from memento.control.db import connect_control_db, migrate_control_db
from memento.derived.index import DerivedIndex
from memento.repository.frontmatter import serialize_concept
from memento.repository.git import GitRepositoryPaths, bootstrap_repository, get_main_revision
from memento.repository.schema import ConceptDocument, ConceptFrontmatter, ConceptStatus
from memento.repository.transactions import TransactionManager
from memento.service import MemoryService, ServiceContext, ServiceDependencies


class FakeModelClient:
    def __init__(self) -> None:
        self.calls: list[ModelRequest] = []

    def complete(self, request: ModelRequest) -> ModelResponse:
        if request.cancelled is not None and request.cancelled():
            raise RuntimeError("cancelled before model call")
        self.calls.append(request)
        question = _extract_question(request.prompt)
        citations = _extract_citations(request.prompt)
        if request.task == "memory_answer_hot":
            if question == "What changed recently?":
                return self._response(
                    answer=UNKNOWN_ANSWER,
                    citations=[],
                    confidence="low",
                    unresolved=["insufficient_support"],
                )
            if citations:
                return self._response(
                    answer="Hot answer",
                    citations=[citations[0]],
                    confidence="medium",
                    unresolved=[],
                )
        if question == "Bad citation?":
            bad = dict(citations[0])
            bad["id"] = "wrong-id"
            return self._response(
                answer="Incorrectly cited answer",
                citations=[bad],
                confidence="high",
                unresolved=[],
            )
        if question == "What is Piclaw?":
            target = next(item for item in citations if item["id"] == "piclaw-id")
            return self._response(
                answer="Piclaw is a visible project.",
                citations=[target],
                confidence="high",
                unresolved=[],
            )
        if question == "What changed recently?":
            target = citations[0]
            return self._response(
                answer=f"Recent change is in {target['path']}.",
                citations=[target],
                confidence="medium",
                unresolved=[],
            )
        return self._response(
            answer=UNKNOWN_ANSWER,
            citations=[],
            confidence="low",
            unresolved=["unsupported_question"],
        )

    def _response(
        self,
        *,
        answer: str,
        citations: list[dict[str, str]],
        confidence: str,
        unresolved: list[str],
    ) -> ModelResponse:
        payload = {
            "answer": answer,
            "citations": citations,
            "confidence": confidence,
            "unresolved": unresolved,
            "model_chain": ["fake-model-v1"],
        }
        return ModelResponse(model_name="fake-model-v1", output_text=json.dumps(payload), usage={})


def success_data(result: object) -> dict[str, Any]:
    payload = cast(Any, result)
    assert payload.status == "success"
    return cast(dict[str, Any], payload.data)


@pytest.fixture()
def control_connection(tmp_path: Path) -> Generator[sqlite3.Connection, None, None]:
    connection = connect_control_db(tmp_path / "control.sqlite")
    migrate_control_db(connection)
    yield connection
    connection.close()


@pytest.fixture()
def repo_paths(tmp_path: Path) -> GitRepositoryPaths:
    seed = tmp_path / "seed"
    write_concept(
        seed / "instances" / "smith.md",
        concept_id="smith-id",
        concept_type="instance",
        title="Smith",
        description="Visible instance.",
        tags=("visible",),
        body="# Smith\n\nSee [Piclaw](/projects/piclaw.md).\n",
    )
    write_concept(
        seed / "projects" / "piclaw.md",
        concept_id="piclaw-id",
        concept_type="project",
        title="Piclaw",
        description="Visible project.",
        tags=("shared",),
        body="# Piclaw\n\nVisible project.\n",
    )
    paths = GitRepositoryPaths(
        bare_dir=tmp_path / "repo.git",
        current_dir=tmp_path / "current",
        worktrees_dir=tmp_path / "worktrees",
    )
    bootstrap_repository(paths, seed)
    return paths


@pytest.fixture()
def fake_model() -> FakeModelClient:
    return FakeModelClient()


@pytest.fixture()
def answer_config(tmp_path: Path) -> ServiceConfig:
    return ServiceConfig(
        schema_version=1,
        repository=RepositoryConfig(root_path=str(tmp_path / "state")),
        authorization=AuthorizationConfig(
            principals={
                "smith": NamespacePolicy(
                    roles=("reader", "proposer", "curator"),
                    read_prefixes=("/instances/", "/projects/"),
                    write_prefixes=("/instances/", "/projects/"),
                ),
                "flint": NamespacePolicy(
                    roles=("reader", "proposer"),
                    read_prefixes=("/instances/", "/projects/"),
                    write_prefixes=("/projects/",),
                ),
            }
        ),
        intelligent_tiers=IntelligentTiersConfig(
            deep_answers=DeepAnswersConfig(
                enabled=True,
                model_policy_revision="fake-policy-v1",
                prompt_version="prompt-v1",
                tool_version="tool-v1",
                limits=DeepAnswerLimitsConfig(
                    max_steps=8,
                    max_time_seconds=2.0,
                    max_concepts=4,
                    max_chars=2_000,
                    max_answer_chars=300,
                ),
            ),
            exact_answer_cache=ExactAnswerCacheConfig(
                enabled=True, ttl_seconds=3_600, max_entries=50
            ),
            hot_working_memory=HotWorkingMemoryConfig(
                enabled=True,
                ttl_seconds=3_600,
                max_changed_concepts=10,
                max_answers=10,
                max_excerpt_chars=1_500,
            ),
        ),
    )


@pytest.fixture()
def service(
    tmp_path: Path,
    answer_config: ServiceConfig,
    control_connection: sqlite3.Connection,
    repo_paths: GitRepositoryPaths,
    fake_model: FakeModelClient,
) -> MemoryService:
    derived_index = DerivedIndex(tmp_path / "derived.sqlite")
    derived_index.rebuild(repo_paths.current_dir, repo_revision=get_main_revision(repo_paths))

    def apply_update(
        materialized_root: Path, repo_revision: str, changed_paths: tuple[str, ...]
    ) -> None:
        if changed_paths:
            derived_index.update_paths(
                materialized_root, repo_revision=repo_revision, changed_paths=changed_paths
            )
        else:
            derived_index.rebuild(materialized_root, repo_revision=repo_revision)

    manager = TransactionManager(control_connection, repo_paths, derived_update=apply_update)
    return MemoryService(
        ServiceDependencies(
            config=answer_config,
            repo_paths=repo_paths,
            control_connection=control_connection,
            derived_index=derived_index,
            transaction_manager=manager,
            model_client=fake_model,
        )
    )


@pytest.fixture()
def smith() -> ServiceContext:
    return ServiceContext(Principal(name="smith", roles=("reader", "proposer", "curator")))


@pytest.fixture()
def flint() -> ServiceContext:
    return ServiceContext(Principal(name="flint", roles=("reader", "proposer")))


def test_memory_answer_returns_deterministic_unknown_when_flags_are_off(
    tmp_path: Path,
    control_connection: sqlite3.Connection,
    repo_paths: GitRepositoryPaths,
) -> None:
    config = ServiceConfig(
        schema_version=1,
        repository=RepositoryConfig(root_path=str(tmp_path / "state-off")),
        authorization=AuthorizationConfig(
            principals={
                "smith": NamespacePolicy(
                    roles=("reader", "proposer", "curator"),
                    read_prefixes=("/instances/", "/projects/"),
                    write_prefixes=("/instances/", "/projects/"),
                )
            }
        ),
    )
    derived_index = DerivedIndex(tmp_path / "derived-off.sqlite")
    derived_index.rebuild(repo_paths.current_dir, repo_revision=get_main_revision(repo_paths))
    manager = TransactionManager(control_connection, repo_paths, derived_update=lambda *_: None)
    service = MemoryService(
        ServiceDependencies(
            config=config,
            repo_paths=repo_paths,
            control_connection=control_connection,
            derived_index=derived_index,
            transaction_manager=manager,
            model_client=None,
        )
    )
    context = ServiceContext(Principal(name="smith", roles=("reader", "proposer", "curator")))
    first = success_data(service.memory_answer(context, question="What is Piclaw?"))
    second = success_data(service.memory_answer(context, question="What is Piclaw?"))
    assert first == second
    assert first["answer"] == UNKNOWN_ANSWER
    assert first["answer_source"] == "disabled"


def test_memory_answer_exact_cache_is_revision_scoped_and_scope_isolated(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    fake_model: FakeModelClient,
    smith: ServiceContext,
    flint: ServiceContext,
) -> None:
    first = success_data(service.memory_answer(smith, question="What is Piclaw?"))
    assert first["answer_source"] == "deep_agent"
    assert len(fake_model.calls) == 1

    second = success_data(service.memory_answer(smith, question="What is Piclaw?"))
    assert second["answer_source"] == "exact_cache"
    assert len(fake_model.calls) == 1

    third = success_data(service.memory_answer(flint, question="What is Piclaw?"))
    assert third["answer_source"] == "deep_agent"
    assert len(fake_model.calls) == 2

    revision = get_main_revision(repo_paths)
    patched = service.memory_patch(
        smith,
        path="/projects/piclaw.md",
        expected_revision=revision,
        idempotency_key="patch-piclaw-answer-cache",
        body="# Piclaw\n\nVisible project, updated.\n",
    )
    assert patched.status == "success"
    fourth = success_data(service.memory_answer(smith, question="What is Piclaw?"))
    assert fourth["answer_source"] != "exact_cache"
    assert len(fake_model.calls) == 3


def test_hot_memory_unknown_falls_back_to_deep_and_intersecting_write_invalidates_hot_answer(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    fake_model: FakeModelClient,
    smith: ServiceContext,
) -> None:
    revision = get_main_revision(repo_paths)
    changed = service.memory_patch(
        smith,
        path="/instances/smith.md",
        expected_revision=revision,
        idempotency_key="patch-smith-hot",
        body="# Smith\n\nRecently updated.\n",
    )
    assert changed.status == "success"

    answered = success_data(service.memory_answer(smith, question="What changed recently?"))
    assert answered["answer_source"] == "deep_agent"
    assert [call.task for call in fake_model.calls[-2:]] == [
        "memory_answer_hot",
        "memory_answer_deep",
    ]

    hot = success_data(service.memory_answer(smith, question="What changed recently?"))
    assert hot["answer_source"] == "exact_cache"

    updated_revision = get_main_revision(repo_paths)
    changed_again = service.memory_patch(
        smith,
        path="/instances/smith.md",
        expected_revision=updated_revision,
        idempotency_key="patch-smith-hot-2",
        body="# Smith\n\nUpdated again.\n",
    )
    assert changed_again.status == "success"
    after_invalidation = success_data(
        service.memory_answer(smith, question="What changed recently?")
    )
    assert after_invalidation["answer_source"] == "deep_agent"


def test_memory_answer_rejects_invalid_citations(
    service: MemoryService, smith: ServiceContext
) -> None:
    answer = success_data(service.memory_answer(smith, question="Bad citation?"))
    assert answer["answer"] == UNKNOWN_ANSWER
    assert answer["unresolved"] == ["citation_validation_failed"]
    assert answer["citations"] == []


def test_memory_answer_honors_cancellation_before_model_call(
    service: MemoryService, smith: ServiceContext
) -> None:
    cancelled = {"value": True}
    result = service.memory_answer(
        smith,
        question="What is Piclaw?",
        cancelled=lambda: cancelled["value"],
    )
    assert result.status == "error"
    assert result.error_class == "validation_error"
    assert "cancelled" in result.message


def write_concept(
    path: Path,
    *,
    concept_id: str,
    concept_type: str,
    title: str,
    description: str,
    tags: tuple[str, ...],
    body: str,
) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    document = ConceptDocument(
        frontmatter=ConceptFrontmatter(
            schema_version=1,
            id=concept_id,
            type=concept_type,
            title=title,
            description=description,
            tags=tags,
            aliases=(),
            source_refs=(),
            supersedes=(),
            status=ConceptStatus.ACTIVE,
            created_at=datetime(2026, 7, 17, 12, 0, tzinfo=timezone.utc),
            updated_at=datetime(2026, 7, 17, 12, 0, tzinfo=timezone.utc),
            updated_by="rui/tests",
        ),
        body=body,
    )
    path.write_text(serialize_concept(document), encoding="utf-8")


def _extract_question(prompt: str) -> str:
    for line in prompt.splitlines():
        if line.startswith("QUESTION: "):
            return line.removeprefix("QUESTION: ").strip()
    return ""


def _extract_citations(prompt: str) -> list[dict[str, str]]:
    citations: list[dict[str, str]] = []
    current: dict[str, str] = {}
    for line in prompt.splitlines():
        if line.startswith("ID: "):
            if current:
                citations.append(current)
            current = {"id": line.removeprefix("ID: ").strip()}
        elif line.startswith("PATH: ") and current:
            current["path"] = line.removeprefix("PATH: ").strip()
        elif line.startswith("REVISION: ") and current:
            current["revision"] = line.removeprefix("REVISION: ").strip()
    if current:
        citations.append(current)
    return citations
