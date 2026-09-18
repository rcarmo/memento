"""Synthetic repository path fixtures from the unchanged Memento baseline."""

from __future__ import annotations

import importlib
import os
import tempfile
from pathlib import Path
from typing import Any

paths_module: Any = importlib.import_module("memento.repository.paths")
PathSafetyError = paths_module.PathSafetyError
is_reserved_bundle_path = paths_module.is_reserved_bundle_path
validate_bundle_path = paths_module.validate_bundle_path
validate_repository_read_path = paths_module.validate_repository_read_path
validate_repository_write_path = paths_module.validate_repository_write_path


def fixtures() -> list[dict[str, Any]]:
    paths = [
        "",
        "/",
        "x",
        "//x",
        "/x/",
        "/./x",
        "/../x",
        "/a/../x",
        "/x\\y",
        "/x\x00",
        "/x\x1f",
        "/x\x7f",
        "/x\x85",
        "/a/%2e%2e/x",
        "/café/日本.md",
        "/index.md",
        "/.memory-schema.json",
        "/log.md",
        "/nested/log.md",
        "/nested/index.md",
        "/.assets",
        "/.assets/file",
        "/.assets-other",
        "/new/deep/file.md",
        "/regular",
        "/directory",
        "/linked/file",
        "/link-target",
        "/fifo",
        "/regular/child",
        "/dangling",
    ]
    results = []
    with tempfile.TemporaryDirectory(prefix="memento-path-oracle-") as directory:
        base = Path(directory)
        root = base / "root"
        root.mkdir()
        (root / "regular").write_text("synthetic")
        (root / "directory").mkdir()
        (root / "linked").symlink_to(root / "directory", target_is_directory=True)
        (root / "link-target").symlink_to(root / "regular")
        (root / "dangling").symlink_to(root / "absent")
        os.mkfifo(root / "fifo")
        (base / "root-link").symlink_to(root, target_is_directory=True)
        for path in paths:
            item: dict[str, Any] = {"path": path, "reserved": is_reserved_bundle_path(path)}
            try:
                validate_bundle_path(path)
                item["canonical_error"] = ""
            except PathSafetyError as exc:
                item["canonical_error"] = str(exc)
            for name, check in [
                ("read", validate_repository_read_path),
                ("write", validate_repository_write_path),
            ]:
                try:
                    check(root, path)
                    item[name + "_error"] = ""
                except PathSafetyError as exc:
                    item[name + "_error"] = str(exc)
            results.append(item)
        for path in [
            "log.md",
            "./log.md",
            "/./log.md",
            "//log.md",
            "/nested/../log.md",
            "/nested//index.md",
        ]:
            results.append(
                {
                    "path": path,
                    "classification_only": True,
                    "reserved": is_reserved_bundle_path(path),
                }
            )
    return results
