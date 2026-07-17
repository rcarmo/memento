from __future__ import annotations

import json
import os
import socket
import sqlite3
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from memento.config import Principal, ServiceConfig
from memento.control.db import connect_control_db, migrate_control_db
from memento.control.proposals import ProposalStatus, list_proposals
from memento.derived.index import DerivedIndex
from memento.model_clients import RoutedFallbackModelClient, build_endpoint_clients
from memento.repository.bundle import scan_bundle
from memento.repository.git import (
    GitRepositoryPaths,
    bootstrap_repository,
    get_main_revision,
    materialize_current_checkout,
)
from memento.repository.lease import WriterLease, acquire_writer_lease
from memento.repository.transactions import TransactionManager
from memento.server import MementoMCPServer
from memento.service import MemoryService, ServiceDependencies


class RuntimeClosedError(RuntimeError):
    """Raised when using a closed runtime."""


@dataclass(frozen=True, slots=True)
class RuntimePaths:
    root: Path
    repo_paths: GitRepositoryPaths
    control_db: Path
    derived_db: Path
    writer_lock: Path


@dataclass(slots=True)
class MementoRuntime:
    config_path: Path
    config: ServiceConfig
    paths: RuntimePaths
    control_connection: sqlite3.Connection
    derived_index: DerivedIndex
    transaction_manager: TransactionManager
    service: MemoryService
    lease: WriterLease
    closed: bool = False

    def build_server(self) -> MementoMCPServer:
        self._require_open()
        return MementoMCPServer(self.service, bearer_tokens=self._bearer_tokens())

    def status_snapshot(self) -> dict[str, Any]:
        self._require_open()
        state = self.derived_index.get_state()
        visible_concepts = len(scan_bundle(self.paths.repo_paths.current_dir).entries)
        proposals = list_proposals(self.control_connection)
        return {
            "service_version": "0.1.0",
            "schema_version": self.config.schema_version,
            "repo_revision": get_main_revision(self.paths.repo_paths),
            "index_revision": state.index_revision,
            "index_stale": state.index_revision != state.repo_revision,
            "visible_concepts": visible_concepts,
            "proposal_backlog": len(
                [
                    item
                    for item in proposals
                    if item.status in {ProposalStatus.SUBMITTED, ProposalStatus.APPROVED}
                ]
            ),
            "control_db": str(self.paths.control_db),
            "derived_db": str(self.paths.derived_db),
            "repo_root": str(self.paths.root),
            "closed": self.closed,
        }

    def rebuild_index(self) -> dict[str, Any]:
        self._require_open()
        revision = get_main_revision(self.paths.repo_paths)
        self.derived_index.rebuild(self.paths.repo_paths.current_dir, repo_revision=revision)
        parity = self.derived_index.parity_check(
            self.paths.repo_paths.current_dir,
            repo_revision=revision,
        )
        return {
            "repo_revision": revision,
            "index_revision": self.derived_index.get_state().index_revision,
            "parity_matches": parity.matches,
            "parity_details": parity.details,
        }

    def close(self) -> None:
        if self.closed:
            return
        self.control_connection.close()
        self.lease.release()
        self.closed = True

    def _bearer_tokens(self) -> dict[str, Principal]:
        tokens: dict[str, Principal] = {}
        for principal_name, principal_policy in self.config.authorization.principals.items():
            tokens[principal_name] = Principal(
                name=principal_name,
                roles=principal_policy.roles,
                metadata={},
            )
        return tokens

    def _require_open(self) -> None:
        if self.closed:
            raise RuntimeClosedError("runtime is closed")


def load_service_config(config_path: Path) -> ServiceConfig:
    return ServiceConfig.model_validate(json.loads(config_path.read_text(encoding="utf-8")))


def runtime_paths_for(config: ServiceConfig) -> RuntimePaths:
    root = Path(config.repository.root_path)
    return RuntimePaths(
        root=root,
        repo_paths=GitRepositoryPaths(
            bare_dir=root / "repo.git",
            current_dir=root / "current",
            worktrees_dir=root / "worktrees",
        ),
        control_db=root / "control.sqlite",
        derived_db=root / "derived.sqlite",
        writer_lock=root / "locks" / "writer.lock",
    )


def _resolve_model_api_keys(config: ServiceConfig) -> dict[str, str]:
    slots = config.intelligent_tiers.model_provider_slots
    keys: dict[str, str] = {}
    for slot in (slots.hot_query, slots.deep_query, slots.proposal, slots.dream):
        for endpoint in ([slot.primary] if slot.primary is not None else []) + list(slot.fallbacks):
            if endpoint.api_key_env is None:
                continue
            keys[endpoint.api_key_env] = os.environ.get(endpoint.api_key_env, "")
    return keys


def build_runtime(config_path: Path, *, bootstrap_seed: Path | None = None) -> MementoRuntime:
    config = load_service_config(config_path)
    paths = runtime_paths_for(config)
    paths.root.mkdir(parents=True, exist_ok=True)
    lease = acquire_writer_lease(
        paths.writer_lock,
        owner=f"memento[{os.getpid()}]@{socket.gethostname()}",
    )
    try:
        if not paths.repo_paths.bare_dir.exists():
            bootstrap_repository(paths.repo_paths, bootstrap_seed)
        elif not paths.repo_paths.current_dir.exists():
            materialize_current_checkout(paths.repo_paths)
        control_connection = connect_control_db(paths.control_db)
        migrate_control_db(control_connection)
        derived_index = DerivedIndex(paths.derived_db)
        if not derived_index.db_path.exists() or derived_index.get_state().index_revision == "":
            derived_index.rebuild(
                paths.repo_paths.current_dir,
                repo_revision=get_main_revision(paths.repo_paths),
            )

        def apply_update(
            materialized_root: Path,
            repo_revision: str,
            changed_paths: tuple[str, ...],
        ) -> None:
            if changed_paths:
                derived_index.update_paths(
                    materialized_root,
                    repo_revision=repo_revision,
                    changed_paths=changed_paths,
                )
            else:
                derived_index.rebuild(materialized_root, repo_revision=repo_revision)

        manager = TransactionManager(
            control_connection,
            paths.repo_paths,
            derived_update=apply_update,
        )
        endpoint_clients = build_endpoint_clients(
            config.intelligent_tiers.model_provider_slots,
            api_keys=_resolve_model_api_keys(config),
        )
        routed_client = (
            RoutedFallbackModelClient(
                config.intelligent_tiers.model_provider_slots,
                endpoint_clients=endpoint_clients,
            )
            if endpoint_clients
            else None
        )
        service = MemoryService(
            ServiceDependencies(
                config=config,
                repo_paths=paths.repo_paths,
                control_connection=control_connection,
                derived_index=derived_index,
                transaction_manager=manager,
                model_client=routed_client,
            )
        )
        manager.recover_startup()
        return MementoRuntime(
            config_path=config_path,
            config=config,
            paths=paths,
            control_connection=control_connection,
            derived_index=derived_index,
            transaction_manager=manager,
            service=service,
            lease=lease,
        )
    except Exception:
        lease.release()
        raise
