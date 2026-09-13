"""Trash locations preserve the original namespace beneath /trash/."""

from __future__ import annotations

from memento.repository.paths import PathSafetyError, validate_bundle_path

TRASH_PREFIX = "/trash/"


def original_path(path: str) -> str:
    validate_bundle_path(path)
    if not path.startswith(TRASH_PREFIX):
        raise PathSafetyError("item must be in /trash/ first")
    original = path.removeprefix("/trash")
    validate_bundle_path(original)
    if original.startswith(TRASH_PREFIX) or not original.endswith(".md"):
        raise PathSafetyError("invalid trash concept path")
    return original


def trash_path(path: str) -> str:
    validate_bundle_path(path)
    if path.startswith(TRASH_PREFIX) or not path.endswith(".md"):
        raise PathSafetyError("expected an active Markdown concept path")
    return "/trash" + path
