from __future__ import annotations

import argparse
import asyncio
import json
from pathlib import Path

from memento.config import ServiceConfig
from memento.control.db import connect_control_db, migrate_control_db
from memento.derived.index import DerivedIndex
from memento.repository.git import GitRepositoryPaths, get_main_revision
from memento.repository.transactions import TransactionManager
from memento.server import MementoMCPServer
from memento.service import MemoryService, ServiceDependencies


def build_service(config_path: Path) -> tuple[MemoryService, ServiceConfig]:
    config = ServiceConfig.model_validate(json.loads(config_path.read_text(encoding="utf-8")))
    root = Path(config.repository.root_path)
    repo_paths = GitRepositoryPaths(
        bare_dir=root / "repo.git",
        current_dir=root / "current",
        worktrees_dir=root / "worktrees",
    )
    control_connection = connect_control_db(root / "control.sqlite")
    migrate_control_db(control_connection)
    derived_index = DerivedIndex(root / "derived.sqlite")
    if not derived_index.db_path.exists():
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
    service = MemoryService(
        ServiceDependencies(
            config=config,
            repo_paths=repo_paths,
            control_connection=control_connection,
            derived_index=derived_index,
            transaction_manager=manager,
        )
    )
    return service, config


def main() -> int:
    parser = argparse.ArgumentParser(description="Serve the Memento MCP service")
    parser.add_argument("--config", required=True)
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=8000)
    parser.add_argument("--endpoint", default="/mcp")
    args = parser.parse_args()

    service, config = build_service(Path(args.config))
    bearer_tokens = {}
    for principal_name, principal_policy in config.authorization.principals.items():
        token = principal_name
        bearer_tokens[token] = {
            "name": principal_name,
            "roles": principal_policy.roles,
            "metadata": {},
        }
    from memento.config import Principal

    server = MementoMCPServer(
        service,
        bearer_tokens={
            token: Principal.model_validate(payload) for token, payload in bearer_tokens.items()
        },
    )
    asyncio.run(
        server.run_streamable_http_async(host=args.host, port=args.port, endpoint=args.endpoint)
    )
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
