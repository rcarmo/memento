from __future__ import annotations

import difflib
import json
import sqlite3
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Literal
from uuid import uuid4

from pydantic import BaseModel, ConfigDict

from memento.authz import (
    AuthorizationError,
    EffectivePolicy,
    authorize_path,
    require_role,
    resolve_policy,
)
from memento.config import Principal, ServiceConfig
from memento.control.operations import IdempotencyConflictError, OperationRequest
from memento.control.proposals import (
    ProposalRecord,
    ProposalStatus,
    create_proposal,
    get_proposal,
    list_proposals,
    update_proposal_status,
)
from memento.derived.index import DerivedIndex, SearchFreshness
from memento.envelopes import ErrorEnvelope, SuccessEnvelope, error_envelope, success_envelope
from memento.repository.bundle import (
    BundleError,
    audit_repository,
    read_bundle_entry,
    scan_bundle,
)
from memento.repository.frontmatter import FrontmatterError, parse_concept_text, serialize_concept
from memento.repository.git import GitError, GitRepositoryPaths, get_main_revision
from memento.repository.links import rewrite_links_for_rename
from memento.repository.paths import PathSafetyError, validate_repository_write_path
from memento.repository.schema import ConceptDocument, ConceptFrontmatter, ConceptStatus
from memento.repository.transactions import (
    TransactionConflictError,
    TransactionManager,
    TransactionRequest,
)


class ServiceError(RuntimeError):
    error_class = "validation_error"


class NotFoundError(ServiceError):
    error_class = "not_found"


class ForbiddenError(ServiceError):
    error_class = "forbidden"


class ConflictError(ServiceError):
    error_class = "conflict"


class RepoUnavailableError(ServiceError):
    error_class = "repo_unavailable"


class TemporarilyReadOnlyError(ServiceError):
    error_class = "temporarily_read_only"


class CreateChange(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    kind: Literal["create"]
    path: str
    concept_type: str
    title: str
    body: str
    description: str | None = None
    tags: tuple[str, ...] = ()
    aliases: tuple[str, ...] = ()


class PatchChange(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    kind: Literal["patch"]
    path: str
    title: str | None = None
    description: str | None = None
    body: str | None = None
    status: ConceptStatus | None = None
    tags: tuple[str, ...] | None = None
    aliases: tuple[str, ...] | None = None


class RenameChange(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    kind: Literal["rename"]
    path: str
    new_path: str


ProposalChange = CreateChange | PatchChange | RenameChange


@dataclass(frozen=True, slots=True)
class ServiceContext:
    principal: Principal
    client_instance_id: str | None = None
    mcp_session_id: str | None = None
    source_chat: str | None = None


@dataclass(frozen=True, slots=True)
class ServiceDependencies:
    config: ServiceConfig
    repo_paths: GitRepositoryPaths
    control_connection: sqlite3.Connection
    derived_index: DerivedIndex
    transaction_manager: TransactionManager


class MemoryService:
    def __init__(self, deps: ServiceDependencies) -> None:
        self._deps = deps

    def memory_help(
        self, context: ServiceContext
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        self._policy(context)
        envelope = self._success(
            {
                "goals": {
                    "read": ["memory_search", "memory_read", "memory_graph"],
                    "browse": ["memory_list", "memory_read"],
                    "propose": ["memory_propose", "memory_proposal_get"],
                    "curate": [
                        "memory_proposal_list",
                        "memory_proposal_review",
                        "memory_proposal_apply",
                        "memory_create",
                        "memory_patch",
                        "memory_rename",
                    ],
                },
                "formats": ("summary", "detailed"),
            }
        )
        return envelope

    def memory_status(
        self, context: ServiceContext
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        try:
            policy = self._policy(context)
            state = self._deps.derived_index.get_state()
            bundle = scan_bundle(self._deps.repo_paths.current_dir)
            visible_paths = [
                entry.bundle_path
                for entry in bundle.entries
                if self._is_authorized(policy, entry.bundle_path, action="read")
            ]
            proposals = list_proposals(self._deps.control_connection)
            return self._success(
                {
                    "service_version": "0.1.0",
                    "schema_version": self._deps.config.schema_version,
                    "repo_revision": get_main_revision(self._deps.repo_paths),
                    "index_revision": state.index_revision,
                    "index_stale": state.index_revision != state.repo_revision,
                    "principal": policy.principal,
                    "visible_concepts": len(visible_paths),
                    "proposal_backlog": len(
                        [
                            item
                            for item in proposals
                            if item.status in {ProposalStatus.SUBMITTED, ProposalStatus.APPROVED}
                        ]
                    ),
                    "limits": self._deps.config.limits.model_dump(mode="python"),
                    "roles": policy.roles,
                    "features": {
                        "resources": True,
                        "streamable_http": True,
                        "proposal_rebase": False,
                    },
                },
                index_revision=state.index_revision,
                index_stale=state.index_revision != state.repo_revision,
            )
        except Exception as exc:
            return self._failure(exc)

    def memory_search(
        self,
        context: ServiceContext,
        *,
        query: str,
        concept_type: str | None = None,
        limit: int = 20,
        cursor: str | None = None,
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        try:
            policy = self._policy(context)
            page = self._deps.derived_index.search(
                policy=policy,
                query=query,
                concept_type=concept_type,
                limit=limit,
                cursor=cursor,
                freshness=SearchFreshness.EVENTUAL,
            )
            return self._success(
                {
                    "results": [
                        {
                            "id": item.concept_id,
                            "path": item.path,
                            "title": item.title,
                            "type": item.concept_type,
                            "status": item.status,
                            "tags": item.tags,
                            "score": item.score,
                            "snippet": item.snippet,
                        }
                        for item in page.results
                    ],
                    "next_cursor": page.next_cursor,
                },
                repo_revision=page.repo_revision,
                index_revision=page.index_revision,
            )
        except Exception as exc:
            return self._failure(exc)

    def memory_read(
        self,
        context: ServiceContext,
        *,
        id_or_path: str,
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        try:
            policy = self._policy(context)
            bundle_path = self._resolve_path(id_or_path)
            authorize_path(policy, bundle_path, action="read")
            entry = read_bundle_entry(self._deps.repo_paths.current_dir, bundle_path)
            return self._success(
                {
                    "path": entry.bundle_path,
                    "frontmatter": entry.document.frontmatter.model_dump(mode="json"),
                    "body": entry.document.body,
                }
            )
        except Exception as exc:
            return self._failure(exc)

    def memory_list(
        self,
        context: ServiceContext,
        *,
        path_prefix: str = "/",
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        try:
            policy = self._policy(context)
            bundle = scan_bundle(self._deps.repo_paths.current_dir)
            visible = []
            for entry in bundle.entries:
                if not entry.bundle_path.startswith(path_prefix):
                    continue
                if not self._is_authorized(policy, entry.bundle_path, action="read"):
                    continue
                visible.append(
                    {
                        "path": entry.bundle_path,
                        "id": entry.document.frontmatter.id,
                        "title": entry.document.frontmatter.title,
                        "type": entry.document.frontmatter.type,
                    }
                )
            return self._success({"entries": visible})
        except Exception as exc:
            return self._failure(exc)

    def memory_graph(
        self,
        context: ServiceContext,
        *,
        id_or_path: str,
        depth: int = 1,
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        try:
            policy = self._policy(context)
            concept_id = self._resolve_concept_id(id_or_path)
            graph = self._deps.derived_index.graph(
                policy=policy, concept_id=concept_id, depth=depth
            )
            return self._success(
                {
                    "center_id": graph.center_id,
                    "outbound": [edge.__dict__ for edge in graph.outbound],
                    "inbound": [edge.__dict__ for edge in graph.inbound],
                    "broken_targets": graph.broken_targets,
                },
                repo_revision=graph.repo_revision,
                index_revision=graph.index_revision,
            )
        except Exception as exc:
            return self._failure(exc)

    def memory_audit(
        self,
        context: ServiceContext,
        *,
        path: str | None = None,
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        try:
            policy = self._policy(context)
            if path is not None:
                authorize_path(policy, path, action="read")
            audit = audit_repository(self._deps.repo_paths.current_dir)
            issues = [
                issue.__dict__
                for issue in audit.issues
                if (path is None or issue.bundle_path == path)
                and self._is_authorized(policy, issue.bundle_path, action="read")
            ]
            if path is not None and not self._is_authorized(policy, path, action="read"):
                raise ForbiddenError(f"principal {policy.principal} cannot read {path}")
            return self._success({"ok": not issues, "issues": issues})
        except Exception as exc:
            return self._failure(exc)

    def memory_propose(
        self,
        context: ServiceContext,
        *,
        intent: str,
        base_revision: str,
        changes: list[dict[str, Any]],
        rationale: str | None = None,
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        try:
            policy = self._policy(context)
            require_role(policy, "proposer")
            normalized = self._normalize_changes(changes)
            self._validate_change_auth(policy, normalized, action="write")
            proposal_id = str(uuid4())
            preview = self._preview_changes(normalized)
            record = create_proposal(
                self._deps.control_connection,
                proposal_id=proposal_id,
                author_principal=policy.principal,
                client_instance_id=context.client_instance_id,
                base_revision=base_revision,
                intent=intent,
                rationale=rationale,
                patch={"changes": [item.model_dump(mode="json") for item in normalized]},
            )
            return self._success({"proposal": self._proposal_payload(record, preview)})
        except Exception as exc:
            return self._failure(exc)

    def memory_proposal_get(
        self,
        context: ServiceContext,
        *,
        proposal_id: str,
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        try:
            policy = self._policy(context)
            record = self._visible_proposal(policy, proposal_id)
            preview = self._preview_changes(self._normalize_changes(record.patch["changes"]))
            record = self._refresh_proposal_status(record)
            return self._success({"proposal": self._proposal_payload(record, preview)})
        except Exception as exc:
            return self._failure(exc)

    def memory_proposal_list(
        self,
        context: ServiceContext,
        *,
        status: str | None = None,
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        try:
            policy = self._policy(context)
            require_role(policy, "proposer")
            requested_status = ProposalStatus(status) if status is not None else None
            proposals = list_proposals(self._deps.control_connection, status=requested_status)
            visible: list[dict[str, Any]] = []
            for proposal in proposals:
                if proposal.author_principal != policy.principal and "curator" not in policy.roles:
                    continue
                refreshed = self._refresh_proposal_status(proposal)
                preview = self._preview_changes(self._normalize_changes(refreshed.patch["changes"]))
                visible.append(self._proposal_payload(refreshed, preview))
            return self._success({"proposals": visible})
        except Exception as exc:
            return self._failure(exc)

    def memory_proposal_review(
        self,
        context: ServiceContext,
        *,
        proposal_id: str,
        decision: str,
        comment: str | None = None,
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        try:
            policy = self._policy(context)
            require_role(policy, "curator")
            proposal = get_proposal(self._deps.control_connection, proposal_id)
            proposal = self._refresh_proposal_status(proposal)
            if proposal.author_principal == policy.principal and decision == "approve":
                raise ForbiddenError("proposal authors cannot self-approve")
            if proposal.status in {ProposalStatus.APPLIED, ProposalStatus.EXPIRED}:
                raise ConflictError(
                    f"proposal {proposal.proposal_id} is already {proposal.status.value}"
                )
            new_status = {
                "approve": ProposalStatus.APPROVED,
                "reject": ProposalStatus.REJECTED,
                "request_changes": ProposalStatus.DRAFT,
            }.get(decision)
            if new_status is None:
                raise ServiceError(f"unsupported proposal decision: {decision}")
            updated = update_proposal_status(
                self._deps.control_connection,
                proposal_id,
                status=new_status,
                reviewed_by=policy.principal,
                review_comment=comment,
            )
            preview = self._preview_changes(self._normalize_changes(updated.patch["changes"]))
            return self._success({"proposal": self._proposal_payload(updated, preview)})
        except Exception as exc:
            return self._failure(exc)

    def memory_proposal_apply(
        self,
        context: ServiceContext,
        *,
        proposal_id: str,
        expected_revision: str,
        idempotency_key: str,
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        try:
            policy = self._policy(context)
            require_role(policy, "curator")
            proposal = get_proposal(self._deps.control_connection, proposal_id)
            proposal = self._refresh_proposal_status(proposal)
            if proposal.status is not ProposalStatus.APPROVED:
                raise ConflictError(f"proposal {proposal.proposal_id} is {proposal.status.value}")
            changes = self._normalize_changes(proposal.patch["changes"])
            request = self._transaction_request(
                context,
                idempotency_key=idempotency_key,
                tool_name="memory_proposal_apply",
                expected_revision=expected_revision,
                request_json=json.dumps({"proposal_id": proposal_id}, sort_keys=True),
                commit_message=f"proposal: apply {proposal_id}",
            )
            result = self._deps.transaction_manager.apply(
                request,
                lambda worktree: self._apply_changes(worktree, changes, actor=policy.principal),
            )
            updated = update_proposal_status(
                self._deps.control_connection,
                proposal_id,
                status=ProposalStatus.APPLIED,
                reviewed_by=proposal.reviewed_by,
                review_comment=proposal.review_comment,
                applied_operation_id=result.operation.op_id,
                applied_revision=result.result_revision,
            )
            return self._success(
                {
                    "proposal": self._proposal_payload(updated, self._preview_changes(changes)),
                    "changed_paths": result.changed_paths,
                    "replayed": result.replayed,
                },
                repo_revision=result.result_revision,
                index_revision=result.result_revision,
                operation_id=result.operation.op_id,
            )
        except Exception as exc:
            return self._failure(exc)

    def memory_create(
        self,
        context: ServiceContext,
        *,
        path: str,
        concept_type: str,
        title: str,
        body: str,
        expected_revision: str,
        idempotency_key: str,
        description: str | None = None,
        tags: tuple[str, ...] = (),
        aliases: tuple[str, ...] = (),
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        change = CreateChange(
            kind="create",
            path=path,
            concept_type=concept_type,
            title=title,
            body=body,
            description=description,
            tags=tags,
            aliases=aliases,
        )
        return self._commit_changes(
            context,
            [change],
            expected_revision=expected_revision,
            idempotency_key=idempotency_key,
            tool_name="memory_create",
            commit_message=f"memory: create {path}",
        )

    def memory_patch(
        self,
        context: ServiceContext,
        *,
        path: str,
        expected_revision: str,
        idempotency_key: str,
        title: str | None = None,
        description: str | None = None,
        body: str | None = None,
        status: ConceptStatus | str | None = None,
        tags: tuple[str, ...] | None = None,
        aliases: tuple[str, ...] | None = None,
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        normalized_status = ConceptStatus(status) if isinstance(status, str) else status
        change = PatchChange(
            kind="patch",
            path=path,
            title=title,
            description=description,
            body=body,
            status=normalized_status,
            tags=tags,
            aliases=aliases,
        )
        return self._commit_changes(
            context,
            [change],
            expected_revision=expected_revision,
            idempotency_key=idempotency_key,
            tool_name="memory_patch",
            commit_message=f"memory: patch {path}",
        )

    def memory_rename(
        self,
        context: ServiceContext,
        *,
        path: str,
        new_path: str,
        expected_revision: str,
        idempotency_key: str,
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        change = RenameChange(kind="rename", path=path, new_path=new_path)
        return self._commit_changes(
            context,
            [change],
            expected_revision=expected_revision,
            idempotency_key=idempotency_key,
            tool_name="memory_rename",
            commit_message=f"memory: rename {path} -> {new_path}",
        )

    def _commit_changes(
        self,
        context: ServiceContext,
        changes: list[ProposalChange],
        *,
        expected_revision: str,
        idempotency_key: str,
        tool_name: str,
        commit_message: str,
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        try:
            policy = self._policy(context)
            require_role(policy, "curator")
            self._validate_change_auth(policy, changes, action="write")
            request = self._transaction_request(
                context,
                idempotency_key=idempotency_key,
                tool_name=tool_name,
                expected_revision=expected_revision,
                request_json=json.dumps(
                    {"changes": [item.model_dump(mode="json") for item in changes]}, sort_keys=True
                ),
                commit_message=commit_message,
            )
            result = self._deps.transaction_manager.apply(
                request,
                lambda worktree: self._apply_changes(worktree, changes, actor=policy.principal),
            )
            return self._success(
                {
                    "changed_paths": result.changed_paths,
                    "diff": self._preview_changes(changes),
                    "replayed": result.replayed,
                },
                repo_revision=result.result_revision,
                index_revision=result.result_revision,
                operation_id=result.operation.op_id,
            )
        except Exception as exc:
            return self._failure(exc)

    def _apply_changes(
        self, worktree: Path, changes: list[ProposalChange], *, actor: str
    ) -> tuple[str, ...]:
        changed_paths: set[str] = set()
        for change in changes:
            if isinstance(change, CreateChange):
                self._apply_create(worktree, change, actor=actor)
                changed_paths.add(change.path)
            elif isinstance(change, PatchChange):
                self._apply_patch(worktree, change, actor=actor)
                changed_paths.add(change.path)
            elif isinstance(change, RenameChange):
                rewritten = self._apply_rename(worktree, change, actor=actor)
                changed_paths.update(rewritten)
            else:  # pragma: no cover
                raise TypeError(f"unsupported change: {type(change)!r}")
        return tuple(sorted(changed_paths))

    def _apply_create(self, worktree: Path, change: CreateChange, *, actor: str) -> None:
        target = validate_repository_write_path(worktree, change.path)
        if target.absolute_path.exists():
            raise ConflictError(f"path already exists: {change.path}")
        document = ConceptDocument(
            frontmatter=ConceptFrontmatter(
                schema_version=1,
                id=str(uuid4()),
                type=change.concept_type,
                title=change.title,
                status=ConceptStatus.ACTIVE,
                description=change.description,
                aliases=change.aliases,
                tags=change.tags,
                source_refs=(),
                supersedes=(),
                created_at=self._now(),
                updated_at=self._now(),
                updated_by=actor,
            ),
            body=change.body,
        )
        target.absolute_path.parent.mkdir(parents=True, exist_ok=True)
        target.absolute_path.write_text(serialize_concept(document), encoding="utf-8")

    def _apply_patch(self, worktree: Path, change: PatchChange, *, actor: str) -> None:
        entry = read_bundle_entry(worktree, change.path)
        frontmatter = entry.document.frontmatter.model_copy(
            update={
                "title": change.title
                if change.title is not None
                else entry.document.frontmatter.title,
                "description": change.description
                if change.description is not None
                else entry.document.frontmatter.description,
                "status": change.status
                if change.status is not None
                else entry.document.frontmatter.status,
                "tags": change.tags if change.tags is not None else entry.document.frontmatter.tags,
                "aliases": change.aliases
                if change.aliases is not None
                else entry.document.frontmatter.aliases,
                "updated_at": self._now(),
                "updated_by": actor,
            }
        )
        document = ConceptDocument(
            frontmatter=frontmatter,
            body=change.body if change.body is not None else entry.document.body,
        )
        target = validate_repository_write_path(worktree, change.path)
        target.absolute_path.write_text(serialize_concept(document), encoding="utf-8")

    def _apply_rename(self, worktree: Path, change: RenameChange, *, actor: str) -> set[str]:
        old_target = validate_repository_write_path(worktree, change.path)
        new_target = validate_repository_write_path(worktree, change.new_path)
        if not old_target.absolute_path.exists():
            raise NotFoundError(change.path)
        if new_target.absolute_path.exists():
            raise ConflictError(f"path already exists: {change.new_path}")
        entry = read_bundle_entry(worktree, change.path)
        document = ConceptDocument(
            frontmatter=entry.document.frontmatter.model_copy(
                update={"updated_at": self._now(), "updated_by": actor}
            ),
            body=entry.document.body,
        )
        new_target.absolute_path.parent.mkdir(parents=True, exist_ok=True)
        new_target.absolute_path.write_text(serialize_concept(document), encoding="utf-8")
        old_target.absolute_path.unlink()
        changed_paths = {change.path, change.new_path}
        for candidate in sorted(worktree.rglob("*.md")):
            bundle_path = "/" + candidate.relative_to(worktree).as_posix()
            if bundle_path == change.new_path:
                continue
            original = parse_concept_text(candidate.read_text(encoding="utf-8"))
            rewritten = rewrite_links_for_rename(
                original.body, old_path=change.path, new_path=change.new_path
            )
            if rewritten.changed:
                updated = ConceptDocument(
                    frontmatter=original.frontmatter.model_copy(
                        update={"updated_at": self._now(), "updated_by": actor}
                    ),
                    body=rewritten.content,
                )
                candidate.write_text(serialize_concept(updated), encoding="utf-8")
                changed_paths.add(bundle_path)
        return changed_paths

    def _preview_changes(self, changes: list[ProposalChange]) -> str:
        diffs: list[str] = []
        for change in changes:
            if isinstance(change, CreateChange):
                new_text = serialize_concept(self._preview_create_document(change))
                diffs.extend(
                    difflib.unified_diff(
                        [],
                        new_text.splitlines(keepends=True),
                        fromfile=f"a{change.path}",
                        tofile=f"b{change.path}",
                    )
                )
            elif isinstance(change, PatchChange):
                entry = read_bundle_entry(self._deps.repo_paths.current_dir, change.path)
                before = serialize_concept(entry.document)
                doc = ConceptDocument(
                    frontmatter=entry.document.frontmatter.model_copy(
                        update={
                            "title": change.title
                            if change.title is not None
                            else entry.document.frontmatter.title,
                            "description": change.description
                            if change.description is not None
                            else entry.document.frontmatter.description,
                            "status": change.status
                            if change.status is not None
                            else entry.document.frontmatter.status,
                            "tags": change.tags
                            if change.tags is not None
                            else entry.document.frontmatter.tags,
                            "aliases": change.aliases
                            if change.aliases is not None
                            else entry.document.frontmatter.aliases,
                            "updated_by": entry.document.frontmatter.updated_by,
                            "updated_at": entry.document.frontmatter.updated_at,
                        }
                    ),
                    body=change.body if change.body is not None else entry.document.body,
                )
                after = serialize_concept(doc)
                diffs.extend(
                    difflib.unified_diff(
                        before.splitlines(keepends=True),
                        after.splitlines(keepends=True),
                        fromfile=f"a{change.path}",
                        tofile=f"b{change.path}",
                    )
                )
            elif isinstance(change, RenameChange):
                diffs.append(f"rename {change.path} -> {change.new_path}\n")
            else:  # pragma: no cover
                raise TypeError(type(change))
        return "".join(diffs)

    def _preview_create_document(self, change: CreateChange) -> ConceptDocument:
        now = datetime(2026, 1, 1, tzinfo=timezone.utc)
        return ConceptDocument(
            frontmatter=ConceptFrontmatter(
                schema_version=1,
                id="<generated>",
                type=change.concept_type,
                title=change.title,
                status=ConceptStatus.ACTIVE,
                description=change.description,
                aliases=change.aliases,
                tags=change.tags,
                source_refs=(),
                supersedes=(),
                created_at=now,
                updated_at=now,
                updated_by="<proposal>",
            ),
            body=change.body,
        )

    def _proposal_payload(self, record: ProposalRecord, preview: str) -> dict[str, Any]:
        return {
            "proposal_id": record.proposal_id,
            "author_principal": record.author_principal,
            "base_revision": record.base_revision,
            "intent": record.intent,
            "rationale": record.rationale,
            "status": record.status.value,
            "reviewed_by": record.reviewed_by,
            "review_comment": record.review_comment,
            "applied_operation_id": record.applied_operation_id,
            "applied_revision": record.applied_revision,
            "expires_at": record.expires_at,
            "changes": record.patch["changes"],
            "diff": preview,
        }

    def _refresh_proposal_status(self, proposal: ProposalRecord) -> ProposalRecord:
        now = self._now().isoformat().replace("+00:00", "Z")
        if (
            proposal.expires_at is not None
            and proposal.expires_at < now
            and proposal.status is not ProposalStatus.APPLIED
        ):
            return update_proposal_status(
                self._deps.control_connection, proposal.proposal_id, status=ProposalStatus.EXPIRED
            )
        if proposal.status in {
            ProposalStatus.SUBMITTED,
            ProposalStatus.APPROVED,
        } and proposal.base_revision != get_main_revision(self._deps.repo_paths):
            return update_proposal_status(
                self._deps.control_connection, proposal.proposal_id, status=ProposalStatus.STALE
            )
        return proposal

    def _visible_proposal(self, policy: EffectivePolicy, proposal_id: str) -> ProposalRecord:
        proposal = get_proposal(self._deps.control_connection, proposal_id)
        if proposal.author_principal != policy.principal and "curator" not in policy.roles:
            raise ForbiddenError(f"principal {policy.principal} cannot read proposal {proposal_id}")
        return proposal

    def _normalize_changes(self, changes: list[dict[str, Any]]) -> list[ProposalChange]:
        normalized: list[ProposalChange] = []
        for item in changes:
            kind = item.get("kind")
            if kind == "create":
                normalized.append(CreateChange.model_validate(item))
            elif kind == "patch":
                normalized.append(PatchChange.model_validate(item))
            elif kind == "rename":
                normalized.append(RenameChange.model_validate(item))
            else:
                raise ServiceError(f"unsupported change kind: {kind}")
        return normalized

    def _validate_change_auth(
        self, policy: EffectivePolicy, changes: list[ProposalChange], *, action: str
    ) -> None:
        for change in changes:
            authorize_path(policy, change.path, action=action)
            if isinstance(change, RenameChange):
                authorize_path(policy, change.new_path, action=action)

    def _resolve_path(self, id_or_path: str) -> str:
        if id_or_path.startswith("/"):
            return id_or_path
        bundle = scan_bundle(self._deps.repo_paths.current_dir)
        for entry in bundle.entries:
            if entry.document.frontmatter.id == id_or_path:
                return entry.bundle_path
        raise NotFoundError(id_or_path)

    def _resolve_concept_id(self, id_or_path: str) -> str:
        if not id_or_path.startswith("/"):
            return id_or_path
        entry = read_bundle_entry(self._deps.repo_paths.current_dir, id_or_path)
        return entry.document.frontmatter.id

    def _policy(self, context: ServiceContext) -> EffectivePolicy:
        return resolve_policy(self._deps.config.authorization, context.principal)

    def _success(
        self,
        data: dict[str, Any],
        *,
        repo_revision: str | None = None,
        index_revision: str | None = None,
        index_stale: bool = False,
        operation_id: str | None = None,
    ) -> SuccessEnvelope[dict[str, Any]]:
        revision = repo_revision or get_main_revision(self._deps.repo_paths)
        return success_envelope(
            data,
            repo_revision=revision,
            index_revision=index_revision or revision,
            index_stale=index_stale,
            operation_id=operation_id,
        )

    def _failure(self, exc: Exception) -> ErrorEnvelope:
        if isinstance(exc, AuthorizationError):
            return error_envelope("forbidden", str(exc))
        if isinstance(exc, ForbiddenError):
            return error_envelope(exc.error_class, str(exc))
        if isinstance(exc, NotFoundError | KeyError):
            return error_envelope("not_found", str(exc))
        if isinstance(exc, (ServiceError, FrontmatterError, BundleError, PathSafetyError)):
            return error_envelope(getattr(exc, "error_class", "validation_error"), str(exc))
        if isinstance(exc, IdempotencyConflictError):
            return error_envelope("idempotency_conflict", str(exc))
        if isinstance(exc, (ConflictError, TransactionConflictError)):
            return error_envelope("conflict", str(exc))
        if isinstance(exc, GitError):
            return error_envelope("repo_unavailable", str(exc))
        raise exc

    def _transaction_request(
        self,
        context: ServiceContext,
        *,
        idempotency_key: str,
        tool_name: str,
        expected_revision: str,
        request_json: str,
        commit_message: str,
    ) -> TransactionRequest:
        return TransactionRequest(
            operation=OperationRequest(
                op_id=str(uuid4()),
                principal=context.principal.name,
                idempotency_key=idempotency_key,
                tool_name=tool_name,
                request_json=request_json,
                client_instance_id=context.client_instance_id,
                mcp_session_id=context.mcp_session_id,
                source_chat=context.source_chat,
            ),
            expected_revision=expected_revision,
            commit_message=commit_message,
            author_name="Rui Carmo",
            author_email="rui.carmo@gmail.com",
        )

    def _is_authorized(self, policy: EffectivePolicy, path: str, *, action: str) -> bool:
        try:
            authorize_path(policy, path, action=action)
            return True
        except AuthorizationError:
            return False

    @staticmethod
    def _now() -> datetime:
        return datetime.now(tz=timezone.utc).replace(microsecond=0)
