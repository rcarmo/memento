"""Build a deterministic synthetic bare Git store with source diff/CAS results.

Objects and packed refs are exported as base64 files for Go-only test runners.
"""

from __future__ import annotations

import base64
import importlib
import os
import subprocess
import tempfile
from pathlib import Path
from typing import Any
from unittest.mock import patch


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.repository.git")
    with tempfile.TemporaryDirectory(prefix="memento-git-oracle-") as directory:
        root = Path(directory)
        seed = root / "seed"
        seed.mkdir()
        (seed / "a.md").write_text("first\n")
        (seed / "remove.md").write_text("removed\n")
        (seed / "bin").mkdir()
        (seed / "bin" / "run").write_text("#!/bin/sh\nexit 0\n")
        (seed / "bin" / "run").chmod(0o755)
        paths = module.GitRepositoryPaths(
            bare_dir=root / "repo.git",
            current_dir=root / "current",
            worktrees_dir=root / "worktrees",
        )
        env = {
            "GIT_AUTHOR_DATE": "2026-09-18T00:00:00+00:00",
            "GIT_COMMITTER_DATE": "2026-09-18T00:00:00+00:00",
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_CONFIG_GLOBAL": "/dev/null",
        }
        with patch.dict(os.environ, env):
            bootstrap = module.bootstrap_repository(paths, seed)
            worktree = module.create_operation_worktree(
                paths, op_id="synthetic-op", base_revision=bootstrap.revision
            )
            (worktree.path / "a.md").write_text("second\n")
            (worktree.path / "remove.md").unlink()
            (worktree.path / "added.md").write_text("added\n")
            (worktree.path / "link").symlink_to("a.md")
            staged = module.commit_exact_paths(
                worktree,
                changed_paths=("/a.md", "/added.md", "/remove.md", "/link"),
                message="synthetic commit",
                author_name="Synthetic Agent",
                author_email="test@example.invalid",
            )
            updated = module.publish_main_compare_and_swap(
                paths, base_revision=bootstrap.revision, new_revision=staged.revision
            )
            stale = module.publish_main_compare_and_swap(
                paths, base_revision=bootstrap.revision, new_revision=bootstrap.revision
            )
            materialized = module.materialize_current_checkout(paths, revision=staged.revision)
            checkout = {}
            for item in sorted(materialized.path.rglob("*")):
                relative = item.relative_to(materialized.path).as_posix()
                if item.is_symlink():
                    checkout[relative] = {"link": os.readlink(item)}
                elif item.is_file():
                    checkout[relative] = {
                        "content": base64.b64encode(item.read_bytes()).decode(),
                        "executable": bool(item.stat().st_mode & 0o111),
                    }
            diff = module.diff_main_paths(
                paths, base_revision=bootstrap.revision, end_revision=staged.revision
            )
            timestamps = {
                path: module.get_path_commit_timestamp(
                    paths, revision=staged.revision, repository_path=path
                )
                for path in ["/a.md", "/added.md", "/link", "/bin/run"]
            }
            subprocess.run(
                [
                    "git",
                    "--git-dir",
                    str(paths.bare_dir),
                    "update-ref",
                    "refs/heads/main",
                    bootstrap.revision,
                ],
                check=True,
                capture_output=True,
            )
            # Keep both commits reachable while exercising packed objects/refs.
            subprocess.run(
                [
                    "git",
                    "--git-dir",
                    str(paths.bare_dir),
                    "update-ref",
                    "refs/heads/candidate",
                    staged.revision,
                ],
                check=True,
                capture_output=True,
            )
            module.remove_operation_worktree(paths, "synthetic-op")
            subprocess.run(
                ["git", "--git-dir", str(paths.bare_dir), "repack", "-ad"],
                check=True,
                capture_output=True,
            )
            subprocess.run(
                ["git", "--git-dir", str(paths.bare_dir), "pack-refs", "--all", "--prune"],
                check=True,
                capture_output=True,
            )
        files = {}
        for path in sorted(paths.bare_dir.rglob("*")):
            relative = path.relative_to(paths.bare_dir).as_posix()
            if path.is_file() and (
                relative in {"HEAD", "config", "packed-refs"}
                or relative.startswith("objects/")
                and relative.endswith((".pack", ".idx"))
            ):
                files[relative] = base64.b64encode(path.read_bytes()).decode()
        return {
            "files": files,
            "base": bootstrap.revision,
            "next": staged.revision,
            "bootstrap_paths": bootstrap.changed_paths,
            "changed_paths": diff,
            "published": updated,
            "stale": stale,
            "timestamps": timestamps,
            "checkout": checkout,
        }
