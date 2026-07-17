from __future__ import annotations

import shutil
import subprocess
from dataclasses import dataclass
from pathlib import Path


class GitError(RuntimeError):
    """Raised when a Git command fails."""


@dataclass(frozen=True, slots=True)
class GitRepositoryPaths:
    bare_dir: Path
    current_dir: Path
    worktrees_dir: Path


@dataclass(frozen=True, slots=True)
class Worktree:
    op_id: str
    path: Path
    base_revision: str


@dataclass(frozen=True, slots=True)
class StagedCommit:
    revision: str
    changed_paths: tuple[str, ...]


@dataclass(frozen=True, slots=True)
class BootstrapResult:
    revision: str
    changed_paths: tuple[str, ...]


@dataclass(frozen=True, slots=True)
class MaterializedCheckout:
    revision: str
    path: Path


def bootstrap_repository(
    paths: GitRepositoryPaths, seed_dir: Path | None = None
) -> BootstrapResult:
    paths.bare_dir.parent.mkdir(parents=True, exist_ok=True)
    paths.worktrees_dir.mkdir(parents=True, exist_ok=True)
    _git("init", "--bare", paths.bare_dir)
    _git("--git-dir", paths.bare_dir, "symbolic-ref", "HEAD", "refs/heads/main")
    bootstrap_dir = paths.worktrees_dir / ".bootstrap"
    if bootstrap_dir.exists():
        shutil.rmtree(bootstrap_dir)
    _git("init", "-b", "main", bootstrap_dir)
    if seed_dir is not None:
        _copy_tree(seed_dir, bootstrap_dir)
    _git("-C", bootstrap_dir, "add", "--all")
    _git(
        "-C",
        bootstrap_dir,
        "-c",
        "user.name=Memento",
        "-c",
        "user.email=memento@example.invalid",
        "commit",
        "--allow-empty",
        "-m",
        "memento: bootstrap main",
    )
    revision = _git_stdout("-C", bootstrap_dir, "rev-parse", "HEAD").strip()
    _git("-C", bootstrap_dir, "remote", "add", "origin", paths.bare_dir)
    _git("-C", bootstrap_dir, "push", "origin", "main:refs/heads/main")
    changed_paths = tuple(sorted(_tracked_paths(bootstrap_dir)))
    materialized = materialize_current_checkout(paths, revision=revision)
    shutil.rmtree(bootstrap_dir)
    return BootstrapResult(revision=materialized.revision, changed_paths=changed_paths)


def get_main_revision(paths: GitRepositoryPaths) -> str:
    return _git_stdout("--git-dir", paths.bare_dir, "rev-parse", "refs/heads/main").strip()


def create_operation_worktree(
    paths: GitRepositoryPaths, *, op_id: str, base_revision: str
) -> Worktree:
    worktree_path = paths.worktrees_dir / op_id
    if worktree_path.exists():
        shutil.rmtree(worktree_path)
    _git("--git-dir", paths.bare_dir, "worktree", "add", "--detach", worktree_path, base_revision)
    return Worktree(op_id=op_id, path=worktree_path, base_revision=base_revision)


def remove_operation_worktree(paths: GitRepositoryPaths, op_id: str) -> None:
    worktree_path = paths.worktrees_dir / op_id
    if not worktree_path.exists():
        return
    _git("--git-dir", paths.bare_dir, "worktree", "remove", "--force", worktree_path)


def commit_exact_paths(
    worktree: Worktree,
    *,
    changed_paths: tuple[str, ...],
    message: str,
    author_name: str,
    author_email: str,
) -> StagedCommit:
    if not changed_paths:
        raise GitError("changed_paths must not be empty")
    _git(
        "-C",
        worktree.path,
        "add",
        "--",
        *[path.removeprefix("/") for path in changed_paths],
    )
    _git(
        "-C",
        worktree.path,
        "-c",
        f"user.name={author_name}",
        "-c",
        f"user.email={author_email}",
        "commit",
        "-m",
        message,
    )
    revision = _git_stdout("-C", worktree.path, "rev-parse", "HEAD").strip()
    return StagedCommit(revision=revision, changed_paths=tuple(sorted(changed_paths)))


def publish_main_compare_and_swap(
    paths: GitRepositoryPaths, *, base_revision: str, new_revision: str
) -> bool:
    result = subprocess.run(
        [
            "git",
            "--git-dir",
            str(paths.bare_dir),
            "update-ref",
            "refs/heads/main",
            new_revision,
            base_revision,
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode == 0:
        return True
    if "cannot lock ref" in result.stderr or "is at" in result.stderr:
        return False
    raise GitError(result.stderr.strip() or "git update-ref failed")


def materialize_current_checkout(
    paths: GitRepositoryPaths, *, revision: str | None = None
) -> MaterializedCheckout:
    target_revision = revision or get_main_revision(paths)
    if paths.current_dir.exists():
        shutil.rmtree(paths.current_dir)
    paths.current_dir.mkdir(parents=True, exist_ok=True)
    tracked = _git_stdout(
        "--git-dir", paths.bare_dir, "ls-tree", "-r", "--name-only", target_revision
    )
    if tracked.strip():
        _git(
            "--git-dir",
            paths.bare_dir,
            "--work-tree",
            paths.current_dir,
            "checkout",
            "--force",
            target_revision,
            "--",
            ".",
        )
    return MaterializedCheckout(revision=target_revision, path=paths.current_dir)


def resolve_worktree_revision(worktree_path: Path) -> str | None:
    if not worktree_path.exists():
        return None
    return _git_stdout("-C", worktree_path, "rev-parse", "HEAD").strip()


def exact_staged_paths(worktree_path: Path) -> tuple[str, ...]:
    output = _git_stdout("-C", worktree_path, "diff", "--cached", "--name-only")
    return tuple(sorted(f"/{line}" for line in output.splitlines() if line))


def diff_main_paths(
    paths: GitRepositoryPaths, *, base_revision: str, end_revision: str
) -> tuple[str, ...]:
    output = _git_stdout(
        "--git-dir",
        paths.bare_dir,
        "diff",
        "--name-only",
        base_revision,
        end_revision,
        "--",
        "*.md",
    )
    return tuple(sorted(f"/{line}" for line in output.splitlines() if line))


def _tracked_paths(root: Path) -> list[str]:
    output = _git_stdout("-C", root, "ls-files")
    return [f"/{line}" for line in output.splitlines() if line]


def _copy_tree(source: Path, target: Path) -> None:
    for path in sorted(source.rglob("*")):
        relative = path.relative_to(source)
        destination = target / relative
        if path.is_dir():
            destination.mkdir(parents=True, exist_ok=True)
            continue
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(path, destination)


def _git_stdout(*args: object) -> str:
    result = subprocess.run(
        ["git", *[str(arg) for arg in args]],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        raise GitError(result.stderr.strip() or "git command failed")
    return result.stdout


def _git(*args: object) -> None:
    _git_stdout(*args)
