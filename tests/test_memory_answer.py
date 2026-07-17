from __future__ import annotations

import json
import sqlite3
from collections.abc import Generator
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, cast

import pytest

from memento.answers import UNKNOWN_ANSWER, ModelAttempt, ModelClient, ModelRequest, ModelResponse
from memento.config import (
    AuthorizationConfig,
    DeepAnswerLimitsConfig,
    DeepAnswersConfig,
    DreamBudgetsConfig,
    DreamConfig,
    DreamScannerConfig,
    ExactAnswerCacheConfig,
    HotWorkingMemoryConfig,
    IntelligentTiersConfig,
    ModelEndpointConfig,
    ModelProposalsConfig,
    ModelProviderSlotsConfig,
    ModelSlotConfig,
    NamespacePolicy,
    Principal,
    RepositoryConfig,
    ServiceConfig,
)
from memento.control.db import connect_control_db, migrate_control_db
from memento.control.proposals import list_proposals
from memento.control.scheduler import claim_scheduler_run
from memento.control.signals import get_service_state, list_signals, set_service_state
from memento.derived.index import DerivedIndex
from memento.model_clients import (
    ModelCancelledError,
    ModelConnectionError,
    ModelHTTPError,
    ModelPolicyError,
    ModelValidationError,
    RoutedFallbackModelClient,
)
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
        prompt_citations = _extract_citations(request.prompt)
        citations = [
            {key: value for key, value in item.items() if key in {"id", "path", "revision"}}
            for item in prompt_citations
        ]
        if request.task == "memory_proposal_draft":
            return self._proposal_response(request.prompt, prompt_citations)
        if request.task == "dream_proposal_draft":
            return self._dream_response(prompt_citations)
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

    def _proposal_response(self, prompt: str, citations: list[dict[str, str]]) -> ModelResponse:
        consulted = [
            {
                "id": item["id"],
                "path": item["path"],
                "revision": item["revision"],
                "title": item.get("title", item["path"].split("/")[-1]),
            }
            for item in citations
        ]
        if "MALFORMED_OUTPUT" in prompt:
            return ModelResponse(model_name="fake-model-v1", output_text="{", usage={})
        if "FORBIDDEN_NAMESPACE" in prompt:
            payload = {
                "intent": "forbidden",
                "rationale": "bad namespace",
                "consulted_concepts": consulted,
                "contradictions": [],
                "reciprocal_links": [],
                "changes": [
                    {
                        "kind": "create",
                        "path": "/secret/forbidden.md",
                        "concept_type": "project",
                        "title": "Forbidden",
                        "body": "# Forbidden\n",
                    }
                ],
            }
            return ModelResponse(
                model_name="fake-model-v1", output_text=json.dumps(payload), usage={}
            )
        if "SECRET_BLOCK" in prompt:
            payload = {
                "intent": "secret",
                "rationale": "bad secret",
                "consulted_concepts": consulted,
                "contradictions": [],
                "reciprocal_links": [],
                "changes": [
                    {
                        "kind": "patch",
                        "path": "/projects/piclaw.md",
                        "body": "# Piclaw\n\napi_key=AKIA1234567890ABCDEF\n",
                    }
                ],
            }
            return ModelResponse(
                model_name="fake-model-v1", output_text=json.dumps(payload), usage={}
            )
        target_path = "/projects/piclaw.md"
        if "TARGET_HINT: Smith" in prompt or "SUGGESTED_PATH: /instances/smith.md" in prompt:
            target_path = "/instances/smith.md"
        payload = {
            "intent": "model drafted proposal",
            "rationale": "Prefer enriching the owning concept and add reciprocal links if justified.",
            "consulted_concepts": consulted,
            "contradictions": [{"path": target_path, "summary": "Existing summary may be stale."}],
            "reciprocal_links": [
                {
                    "source_path": target_path,
                    "target_path": "/projects/piclaw.md",
                    "justification": "Cross-reference related concepts.",
                }
            ],
            "changes": [
                {
                    "kind": "patch",
                    "path": target_path,
                    "body": f"# {target_path.split('/')[-1].removesuffix('.md').title()}\n\nUpdated by model proposal.\n",
                }
            ],
        }
        return ModelResponse(model_name="fake-model-v1", output_text=json.dumps(payload), usage={})

    def _dream_response(self, citations: list[dict[str, str]]) -> ModelResponse:
        consulted = [
            {
                "id": item["id"],
                "path": item["path"],
                "revision": item["revision"],
                "title": item.get("title", item["path"].split("/")[-1]),
            }
            for item in citations
        ]
        target_path = consulted[0]["path"] if consulted else "/projects/piclaw.md"
        payload = {
            "intent": "dream maintenance proposal",
            "rationale": "Repair the surfaced Dream signal with a normal proposal only.",
            "consulted_concepts": consulted,
            "contradictions": [],
            "reciprocal_links": [],
            "changes": [
                {
                    "kind": "patch",
                    "path": target_path,
                    "body": f"# {target_path.split('/')[-1].removesuffix('.md').title()}\n\nUpdated by Dream proposal.\n",
                }
            ],
        }
        return ModelResponse(model_name="fake-model-v1", output_text=json.dumps(payload), usage={})

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
        schema_version=2,
        repository=RepositoryConfig(root_path=str(tmp_path / "state")),
        authorization=AuthorizationConfig(
            principals={
                "smith": NamespacePolicy(
                    roles=("reader", "proposer", "curator"),
                    token_env="MEMENTO_TOKEN_SMITH",
                    read_prefixes=("/instances/", "/projects/"),
                    write_prefixes=("/instances/", "/projects/"),
                ),
                "flint": NamespacePolicy(
                    roles=("reader", "proposer"),
                    token_env="MEMENTO_TOKEN_FLINT",
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
            model_proposals=ModelProposalsConfig(enabled=True),
            dream=DreamConfig(
                mode="report_only",
                model_policy_revision="fake-dream-policy-v1",
                prompt_version="dream-prompt-v1",
                tool_version="dream-tool-v1",
                interval_seconds=300,
                quiet_period_seconds=0,
                scanner=DreamScannerConfig(
                    oversized_body_chars=256,
                    oversized_top_level_sections=3,
                ),
                budgets=DreamBudgetsConfig(
                    max_signals_per_run=25,
                    max_model_proposals_per_run=1,
                    max_runtime_seconds=5.0,
                    daily_proposal_limit=5,
                ),
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
        schema_version=2,
        repository=RepositoryConfig(root_path=str(tmp_path / "state-off")),
        authorization=AuthorizationConfig(
            principals={
                "smith": NamespacePolicy(
                    roles=("reader", "proposer", "curator"),
                    token_env="MEMENTO_TOKEN_SMITH",
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


def test_model_proposals_return_disabled_when_flag_is_off(
    tmp_path: Path,
    control_connection: sqlite3.Connection,
    repo_paths: GitRepositoryPaths,
    fake_model: FakeModelClient,
) -> None:
    config = ServiceConfig(
        schema_version=2,
        repository=RepositoryConfig(root_path=str(tmp_path / "state-model-proposals-off")),
        authorization=AuthorizationConfig(
            principals={
                "smith": NamespacePolicy(
                    roles=("reader", "proposer", "curator"),
                    token_env="MEMENTO_TOKEN_SMITH",
                    read_prefixes=("/instances/", "/projects/"),
                    write_prefixes=("/instances/", "/projects/"),
                )
            }
        ),
    )
    derived_index = DerivedIndex(tmp_path / "derived-model-proposals-off.sqlite")
    derived_index.rebuild(repo_paths.current_dir, repo_revision=get_main_revision(repo_paths))
    service = MemoryService(
        ServiceDependencies(
            config=config,
            repo_paths=repo_paths,
            control_connection=control_connection,
            derived_index=derived_index,
            transaction_manager=TransactionManager(
                control_connection, repo_paths, derived_update=lambda *_: None
            ),
            model_client=fake_model,
        )
    )
    context = ServiceContext(Principal(name="smith", roles=("reader", "proposer", "curator")))
    result = service.memory_propose_freeform(context, content="New fact")
    assert result.status == "error"
    assert result.error_class == "validation_error"
    assert "disabled" in result.message


def test_model_proposals_require_search_context_and_store_consulted_citations(
    service: MemoryService,
    fake_model: FakeModelClient,
    smith: ServiceContext,
) -> None:
    result = service.memory_propose_update(
        smith,
        instruction="Refresh the Piclaw summary.",
        target_hint="Piclaw",
    )
    assert result.status == "success"
    proposal = success_data(result)["proposal"]
    assert proposal["changes"][0]["path"] == "/projects/piclaw.md"
    assert proposal["consulted_concepts"]
    assert any(call.task == "memory_proposal_draft" for call in fake_model.calls)
    prompt = next(call.prompt for call in fake_model.calls if call.task == "memory_proposal_draft")
    assert "UNTRUSTED_CONCEPT_BEGIN" in prompt
    assert "PATH: /projects/piclaw.md" in prompt


def test_model_proposals_reject_malformed_output_forbidden_namespace_and_secrets(
    service: MemoryService,
    smith: ServiceContext,
) -> None:
    malformed = service.memory_propose_freeform(smith, content="MALFORMED_OUTPUT")
    assert malformed.status == "error"
    assert malformed.error_class == "validation_error"

    forbidden = service.memory_propose_freeform(smith, content="FORBIDDEN_NAMESPACE")
    assert forbidden.status == "error"
    assert forbidden.error_class == "forbidden"

    secret = service.memory_propose_freeform(smith, content="SECRET_BLOCK")
    assert secret.status == "error"
    assert secret.error_class == "validation_error"
    assert "secret scanner" in secret.message


def test_model_proposals_only_store_submitted_proposals_without_git_mutation(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    smith: ServiceContext,
) -> None:
    before_revision = get_main_revision(repo_paths)
    before_text = (repo_paths.current_dir / "projects" / "piclaw.md").read_text(encoding="utf-8")
    result = service.memory_propose_freeform(
        smith,
        content="Please update the Piclaw summary with a fresher description.",
        suggested_path="/projects/piclaw.md",
    )
    assert result.status == "success"
    proposal = success_data(result)["proposal"]
    assert proposal["status"] == "submitted"
    assert get_main_revision(repo_paths) == before_revision
    after_text = (repo_paths.current_dir / "projects" / "piclaw.md").read_text(encoding="utf-8")
    assert after_text == before_text
    assert "Updated by model proposal" in proposal["diff"]


def test_model_proposals_resolve_update_target_from_hint(
    service: MemoryService,
    smith: ServiceContext,
) -> None:
    result = service.memory_propose_update(
        smith,
        instruction="Update Smith details.",
        target_hint="Smith",
    )
    assert result.status == "success"
    proposal = success_data(result)["proposal"]
    assert proposal["changes"][0]["path"] == "/instances/smith.md"
    assert any(item["path"] == "/instances/smith.md" for item in proposal["consulted_concepts"])


def test_dream_scanner_detects_signals_dedupes_and_updates_watermark(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    smith: ServiceContext,
) -> None:
    revision = get_main_revision(repo_paths)
    created = service.memory_create(
        smith,
        path="/projects/piclaw-copy.md",
        concept_type="project",
        title="Piclaw",
        description="Visible project.",
        body="# Piclaw\n\nVisible project.\n",
        expected_revision=revision,
        idempotency_key="dream-create-duplicate",
    )
    assert created.status == "success"
    revision = get_main_revision(repo_paths)
    broken = service.memory_create(
        smith,
        path="/projects/broken.md",
        concept_type="project",
        title="Broken",
        description="Broken link holder.",
        body="# Broken\n\nSee [Missing](/projects/missing.md).\n",
        expected_revision=revision,
        idempotency_key="dream-create-broken",
    )
    assert broken.status == "success"
    revision = get_main_revision(repo_paths)
    oversized_body = "# Big\n\n" + "x" * 300
    oversized = service.memory_create(
        smith,
        path="/projects/big.md",
        concept_type="project",
        title="Big",
        description="Large concept.",
        body=oversized_body,
        expected_revision=revision,
        idempotency_key="dream-create-big",
    )
    assert oversized.status == "success"

    before_calls = len(service._deps.model_client.calls)  # type: ignore[union-attr]
    result = service.run_dream(
        mode="report_only", now=datetime(2026, 7, 17, 12, 55, tzinfo=timezone.utc)
    )
    assert result["state"] == "succeeded"
    assert result["proposal_count"] == 0
    assert len(service._deps.model_client.calls) == before_calls  # type: ignore[union-attr]

    signal_types = {signal.signal_type for signal in list_signals(service._deps.control_connection)}
    assert {"orphan", "broken_link", "likely_duplicate", "oversized_concept"} <= signal_types
    assert get_service_state(
        service._deps.control_connection, key="last_dream_revision"
    ) == get_main_revision(repo_paths)

    rerun = service.run_dream(
        mode="report_only", now=datetime(2026, 7, 17, 13, 0, tzinfo=timezone.utc)
    )
    assert rerun["state"] == "succeeded"
    assert len(list_signals(service._deps.control_connection)) >= 4


def test_dream_recent_activity_is_bounded_by_last_successful_revision(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    smith: ServiceContext,
) -> None:
    baseline = get_main_revision(repo_paths)
    set_service_state(service._deps.control_connection, key="last_dream_revision", value=baseline)
    revised = service.memory_patch(
        smith,
        path="/projects/piclaw.md",
        expected_revision=baseline,
        idempotency_key="dream-recent-activity",
        body="# Piclaw\n\nChanged for Dream.\n",
    )
    assert revised.status == "success"
    result = service.run_dream(
        mode="report_only", now=datetime(2026, 7, 17, 13, 0, tzinfo=timezone.utc)
    )
    assert result["state"] == "succeeded"
    assert any(
        signal.signal_type == "recent_activity"
        for signal in list_signals(service._deps.control_connection)
    )
    followup = service.run_dream(
        mode="report_only", now=datetime(2026, 7, 17, 13, 5, tzinfo=timezone.utc)
    )
    assert followup["state"] == "succeeded"
    recent = [
        signal
        for signal in list_signals(service._deps.control_connection)
        if signal.signal_type == "recent_activity" and signal.status != "resolved"
    ]
    assert not recent


def test_dream_no_signal_means_no_model_call(
    tmp_path: Path,
    control_connection: sqlite3.Connection,
    fake_model: FakeModelClient,
) -> None:
    seed = tmp_path / "seed-dream-clean"
    write_concept(
        seed / "projects" / "alpha.md",
        concept_id="alpha-id",
        concept_type="project",
        title="Alpha",
        description="Alpha project.",
        tags=("alpha",),
        body="# Alpha\n\nSee [Beta](/projects/beta.md).\n",
    )
    write_concept(
        seed / "projects" / "beta.md",
        concept_id="beta-id",
        concept_type="project",
        title="Beta",
        description="Beta project.",
        tags=("beta",),
        body="# Beta\n\nSee [Alpha](/projects/alpha.md).\n",
    )
    repo_paths = GitRepositoryPaths(
        bare_dir=tmp_path / "repo-dream-clean.git",
        current_dir=tmp_path / "current-dream-clean",
        worktrees_dir=tmp_path / "worktrees-dream-clean",
    )
    bootstrap_repository(repo_paths, seed)
    config = ServiceConfig(
        schema_version=2,
        repository=RepositoryConfig(root_path=str(tmp_path / "state-dream-clean")),
        authorization=AuthorizationConfig(
            principals={
                "smith": NamespacePolicy(
                    roles=("reader", "proposer", "curator"),
                    token_env="MEMENTO_TOKEN_SMITH",
                    read_prefixes=("/projects/",),
                    write_prefixes=("/projects/",),
                )
            }
        ),
        intelligent_tiers=IntelligentTiersConfig(
            model_proposals=ModelProposalsConfig(enabled=True),
            dream=DreamConfig(mode="propose", interval_seconds=300, quiet_period_seconds=0),
        ),
    )
    derived_index = DerivedIndex(tmp_path / "derived-dream-clean.sqlite")
    derived_index.rebuild(repo_paths.current_dir, repo_revision=get_main_revision(repo_paths))
    clean_service = MemoryService(
        ServiceDependencies(
            config=config,
            repo_paths=repo_paths,
            control_connection=control_connection,
            derived_index=derived_index,
            transaction_manager=TransactionManager(
                control_connection, repo_paths, derived_update=lambda *_: None
            ),
            model_client=fake_model,
        )
    )
    set_service_state(
        control_connection, key="last_dream_revision", value=get_main_revision(repo_paths)
    )
    result = clean_service.run_dream(
        mode="propose", now=datetime(2026, 7, 17, 13, 0, tzinfo=timezone.utc)
    )
    assert result["state"] == "succeeded"
    assert result["actionable_signal_count"] == 0
    assert not any(call.task == "dream_proposal_draft" for call in fake_model.calls)


def test_dream_propose_creates_proposal_without_git_mutation(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
) -> None:
    before_revision = get_main_revision(repo_paths)
    before_text = (repo_paths.current_dir / "instances" / "smith.md").read_text(encoding="utf-8")
    result = service.run_dream(
        mode="propose", now=datetime(2026, 7, 17, 13, 0, tzinfo=timezone.utc)
    )
    assert result["state"] == "succeeded"
    assert result["proposal_count"] == 1
    proposals = list_proposals(service._deps.control_connection)
    assert proposals
    assert proposals[-1].author_principal == "dream"
    assert get_main_revision(repo_paths) == before_revision
    assert (repo_paths.current_dir / "instances" / "smith.md").read_text(
        encoding="utf-8"
    ) == before_text


def test_dream_duplicate_window_and_no_overlap(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
) -> None:
    now = datetime(2026, 7, 17, 13, 0, tzinfo=timezone.utc)
    first = service.run_dream(mode="report_only", now=now)
    assert first["state"] == "succeeded"
    duplicate = service.run_dream(mode="report_only", now=now)
    assert duplicate["state"] == "skipped_duplicate_window"
    claim_scheduler_run(
        service._deps.control_connection,
        job_name="dream",
        window_key="manual-running",
        base_revision=get_main_revision(repo_paths),
    )
    overlap = service.run_dream(
        mode="report_only", now=datetime(2026, 7, 17, 13, 10, tzinfo=timezone.utc)
    )
    assert overlap["state"] == "skipped_overlap"


def test_dream_budgets_cap_oversized_candidates_and_daily_proposals(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    smith: ServiceContext,
) -> None:
    revision = get_main_revision(repo_paths)
    for index in range(4):
        created = service.memory_create(
            smith,
            path=f"/projects/huge-{index}.md",
            concept_type="project",
            title=f"Huge {index}",
            description="Huge concept.",
            body="# Huge\n\n" + ("x" * (300 + index)),
            expected_revision=revision,
            idempotency_key=f"dream-budget-{index}",
        )
        assert created.status == "success"
        revision = get_main_revision(repo_paths)
    service.run_dream(mode="report_only", now=datetime(2026, 7, 17, 13, 0, tzinfo=timezone.utc))
    oversized = [
        signal
        for signal in list_signals(service._deps.control_connection)
        if signal.signal_type == "oversized_concept" and signal.status != "resolved"
    ]
    assert len(oversized) == 3
    set_service_state(
        service._deps.control_connection,
        key="last_dream_revision",
        value=get_main_revision(repo_paths),
    )
    limited_service = _service_with_config(
        tmp_path=repo_paths.current_dir.parent,
        control_connection=service._deps.control_connection,
        repo_paths=repo_paths,
        model_client=service._deps.model_client,
        config=service._deps.config.model_copy(
            update={
                "intelligent_tiers": service._deps.config.intelligent_tiers.model_copy(
                    update={
                        "dream": DreamConfig(
                            mode="propose",
                            interval_seconds=300,
                            quiet_period_seconds=0,
                            budgets=DreamBudgetsConfig(
                                max_signals_per_run=25,
                                max_model_proposals_per_run=1,
                                max_runtime_seconds=5.0,
                                daily_proposal_limit=0,
                            ),
                        ),
                    }
                )
            }
        ),
    )
    limited = limited_service.run_dream(
        mode="propose", now=datetime(2026, 7, 17, 14, 0, tzinfo=timezone.utc)
    )
    assert limited["proposal_count"] == 0


def _service_with_config(
    *,
    tmp_path: Path,
    control_connection: sqlite3.Connection,
    repo_paths: GitRepositoryPaths,
    model_client: ModelClient | None,
    config: ServiceConfig,
) -> MemoryService:
    derived_index = DerivedIndex(tmp_path / "derived-dream-override.sqlite")
    derived_index.rebuild(repo_paths.current_dir, repo_revision=get_main_revision(repo_paths))
    return MemoryService(
        ServiceDependencies(
            config=config,
            repo_paths=repo_paths,
            control_connection=control_connection,
            derived_index=derived_index,
            transaction_manager=TransactionManager(
                control_connection, repo_paths, derived_update=lambda *_: None
            ),
            model_client=model_client,
        )
    )


class FakeEndpointClient:
    def __init__(self, model_name: str, outcomes: list[object], *, trust_boundary: str) -> None:
        self.model_name = model_name
        self._outcomes = outcomes
        self.trust_boundary = trust_boundary
        self.calls: list[ModelRequest] = []

    def complete(self, request: ModelRequest) -> ModelResponse:
        self.calls.append(request)
        outcome = self._outcomes.pop(0)
        if isinstance(outcome, Exception):
            raise outcome
        return cast(ModelResponse, outcome)


def _slot_router(
    *,
    hot: list[object] | None = None,
    deep: list[object] | None = None,
    proposal: list[object] | None = None,
    dream: list[object] | None = None,
    allow_cross_trust_boundary: bool = True,
    proposal_fallback_enabled: bool = False,
    dream_fallback_enabled: bool = False,
    fallback_on_rate_limit: bool = False,
) -> tuple[RoutedFallbackModelClient, dict[str, FakeEndpointClient]]:
    slots = ModelProviderSlotsConfig(
        hot_query=ModelSlotConfig(
            primary=ModelEndpointConfig(
                base_url="http://localhost:8001", api_format="openai", model="hot-primary"
            ),
            fallbacks=(
                ModelEndpointConfig(
                    base_url="http://remote.example", api_format="openai", model="hot-fallback"
                ),
            ),
            allowed_data_classifications=("internal",),
            allow_cross_trust_boundary=allow_cross_trust_boundary,
            fallback_enabled=True,
            fallback_on_rate_limit=fallback_on_rate_limit,
        ),
        deep_query=ModelSlotConfig(
            primary=ModelEndpointConfig(
                base_url="http://localhost:8002", api_format="openai", model="deep-primary"
            ),
            fallbacks=(
                ModelEndpointConfig(
                    base_url="http://remote.example", api_format="openai", model="deep-fallback"
                ),
            ),
            allowed_data_classifications=("internal",),
            allow_cross_trust_boundary=allow_cross_trust_boundary,
            fallback_enabled=True,
            fallback_on_rate_limit=fallback_on_rate_limit,
        ),
        proposal=ModelSlotConfig(
            primary=ModelEndpointConfig(
                base_url="http://localhost:8003", api_format="openai", model="proposal-primary"
            ),
            fallbacks=(
                ModelEndpointConfig(
                    base_url="http://remote.example", api_format="openai", model="proposal-fallback"
                ),
            ),
            allowed_data_classifications=("restricted",),
            allow_cross_trust_boundary=allow_cross_trust_boundary,
            fallback_enabled=proposal_fallback_enabled,
        ),
        dream=ModelSlotConfig(
            primary=ModelEndpointConfig(
                base_url="http://localhost:8004", api_format="openai", model="dream-primary"
            ),
            fallbacks=(
                ModelEndpointConfig(
                    base_url="http://remote.example", api_format="openai", model="dream-fallback"
                ),
            ),
            allowed_data_classifications=("restricted",),
            allow_cross_trust_boundary=allow_cross_trust_boundary,
            fallback_enabled=dream_fallback_enabled,
        ),
    )
    clients = {
        "hot-primary": FakeEndpointClient("hot-primary", hot or [], trust_boundary="local"),
        "hot-fallback": FakeEndpointClient("hot-fallback", [], trust_boundary="remote"),
        "deep-primary": FakeEndpointClient("deep-primary", deep or [], trust_boundary="local"),
        "deep-fallback": FakeEndpointClient("deep-fallback", [], trust_boundary="remote"),
        "proposal-primary": FakeEndpointClient(
            "proposal-primary", proposal or [], trust_boundary="local"
        ),
        "proposal-fallback": FakeEndpointClient("proposal-fallback", [], trust_boundary="remote"),
        "dream-primary": FakeEndpointClient("dream-primary", dream or [], trust_boundary="local"),
        "dream-fallback": FakeEndpointClient("dream-fallback", [], trust_boundary="remote"),
    }
    endpoint_clients = {
        json.dumps(slot.primary.model_dump(mode="json"), sort_keys=True): clients[
            slot.primary.model
        ]
        for slot in (slots.hot_query, slots.deep_query, slots.proposal, slots.dream)
        if slot.primary is not None
    }
    for slot in (slots.hot_query, slots.deep_query, slots.proposal, slots.dream):
        for endpoint in slot.fallbacks:
            endpoint_clients[json.dumps(endpoint.model_dump(mode="json"), sort_keys=True)] = (
                clients[endpoint.model]
            )
    return RoutedFallbackModelClient(slots, endpoint_clients=endpoint_clients), clients


def test_routed_model_fallback_tracks_attempts_and_routes_by_slot() -> None:
    deep_success = ModelResponse(
        model_name="deep-fallback",
        output_text='{"answer":"ok","confidence":"high","citations":[],"unresolved":[]}',
        usage={},
    )
    hot_success = ModelResponse(
        model_name="hot-primary",
        output_text='{"answer":"ok","confidence":"high","citations":[],"unresolved":[]}',
        usage={},
    )
    router, clients = _slot_router(
        deep=[ModelConnectionError("down")],
        hot=[hot_success],
        allow_cross_trust_boundary=True,
    )
    clients["deep-fallback"]._outcomes = [deep_success]
    deep_response = router.complete(
        ModelRequest(
            task="memory_answer_deep",
            prompt="q",
            max_output_chars=200,
            timeout_seconds=2.0,
            data_classification="internal",
        )
    )
    hot_response = router.complete(
        ModelRequest(
            task="memory_answer_hot",
            prompt="q",
            max_output_chars=200,
            timeout_seconds=2.0,
            data_classification="internal",
        )
    )
    assert [item.model for item in deep_response.model_chain] == ["deep-primary", "deep-fallback"]
    assert [item.outcome for item in deep_response.model_chain] == ["connection_failed", "success"]
    assert hot_response.model_chain[-1].model == "hot-primary"
    assert len(clients["deep-primary"].calls) == 1
    assert len(clients["deep-fallback"].calls) == 1
    assert len(clients["hot-primary"].calls) == 1


def test_routed_model_no_fallback_on_auth_validation_cancel_or_429() -> None:
    router, clients = _slot_router(
        deep=[ModelHTTPError(401, "nope", retryable=False)],
        allow_cross_trust_boundary=True,
    )
    with pytest.raises(ModelHTTPError):
        router.complete(
            ModelRequest(
                task="memory_answer_deep",
                prompt="q",
                max_output_chars=200,
                timeout_seconds=2.0,
                data_classification="internal",
            )
        )
    assert not clients["deep-fallback"].calls

    router, clients = _slot_router(
        deep=[ModelValidationError("bad")],
        allow_cross_trust_boundary=True,
    )
    with pytest.raises(ModelValidationError):
        router.complete(
            ModelRequest(
                task="memory_answer_deep",
                prompt="q",
                max_output_chars=200,
                timeout_seconds=2.0,
                data_classification="internal",
            )
        )
    assert not clients["deep-fallback"].calls

    router, clients = _slot_router(
        deep=[ModelCancelledError("cancel")],
        allow_cross_trust_boundary=True,
    )
    with pytest.raises(ModelCancelledError):
        router.complete(
            ModelRequest(
                task="memory_answer_deep",
                prompt="q",
                max_output_chars=200,
                timeout_seconds=2.0,
                data_classification="internal",
            )
        )
    assert not clients["deep-fallback"].calls

    router, clients = _slot_router(
        deep=[ModelHTTPError(429, "rate", retryable=False)],
        allow_cross_trust_boundary=True,
        fallback_on_rate_limit=False,
    )
    with pytest.raises(ModelHTTPError):
        router.complete(
            ModelRequest(
                task="memory_answer_deep",
                prompt="q",
                max_output_chars=200,
                timeout_seconds=2.0,
                data_classification="internal",
            )
        )
    assert not clients["deep-fallback"].calls


def test_routed_model_enforces_privacy_boundary_and_slot_classification() -> None:
    router, clients = _slot_router(
        deep=[ModelConnectionError("down")], allow_cross_trust_boundary=False
    )
    with pytest.raises(ModelConnectionError):
        router.complete(
            ModelRequest(
                task="memory_answer_deep",
                prompt="q",
                max_output_chars=200,
                timeout_seconds=2.0,
                data_classification="internal",
            )
        )
    assert not clients["deep-fallback"].calls

    with pytest.raises(ModelPolicyError):
        router.complete(
            ModelRequest(
                task="memory_answer_deep",
                prompt="q",
                max_output_chars=200,
                timeout_seconds=2.0,
                data_classification="restricted",
            )
        )


def test_proposal_and_dream_fallback_default_disabled() -> None:
    router, clients = _slot_router(
        proposal=[ModelConnectionError("down")],
        dream=[ModelConnectionError("down")],
        allow_cross_trust_boundary=True,
        proposal_fallback_enabled=False,
        dream_fallback_enabled=False,
    )
    clients["proposal-fallback"]._outcomes = [
        ModelResponse(model_name="proposal-fallback", output_text="{}", usage={})
    ]
    clients["dream-fallback"]._outcomes = [
        ModelResponse(model_name="dream-fallback", output_text="{}", usage={})
    ]
    with pytest.raises(ModelConnectionError):
        router.complete(
            ModelRequest(
                task="memory_proposal_draft",
                prompt="q",
                max_output_chars=200,
                timeout_seconds=2.0,
                data_classification="restricted",
            )
        )
    with pytest.raises(ModelConnectionError):
        router.complete(
            ModelRequest(
                task="dream_proposal_draft",
                prompt="q",
                max_output_chars=200,
                timeout_seconds=2.0,
                data_classification="restricted",
            )
        )
    assert not clients["proposal-fallback"].calls
    assert not clients["dream-fallback"].calls


def test_memory_answer_model_fallback_does_not_replay_whole_agent(
    service: MemoryService,
    smith: ServiceContext,
) -> None:
    answer_json = json.dumps(
        {
            "answer": "Piclaw is a visible project.",
            "confidence": "high",
            "citations": [],
            "unresolved": [],
        }
    )
    router, clients = _slot_router(
        deep=[ModelConnectionError("down")], allow_cross_trust_boundary=True
    )
    clients["deep-fallback"]._outcomes = [
        ModelResponse(
            model_name="deep-fallback",
            output_text=answer_json,
            usage={},
            model_chain=(ModelAttempt(model="deep-fallback", outcome="success"),),
        )
    ]
    instrumented = service
    instrumented._deps = ServiceDependencies(
        config=service._deps.config,
        repo_paths=service._deps.repo_paths,
        control_connection=service._deps.control_connection,
        derived_index=service._deps.derived_index,
        transaction_manager=service._deps.transaction_manager,
        model_client=router,
    )
    reads = {"count": 0}
    original = instrumented._read_concept_by_path

    def counted(*args: Any, **kwargs: Any) -> Any:
        reads["count"] += 1
        return original(*args, **kwargs)

    instrumented._read_concept_by_path = counted  # type: ignore[method-assign]
    payload = success_data(instrumented.memory_answer(smith, question="What is Piclaw?"))
    assert payload["answer"] == "UNKNOWN"
    assert reads["count"] > 0
    assert len(clients["deep-primary"].calls) == 1
    assert len(clients["deep-fallback"].calls) == 1


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
        elif line.startswith("TITLE: ") and current:
            current["title"] = line.removeprefix("TITLE: ").strip()
    if current:
        citations.append(current)
    return citations
