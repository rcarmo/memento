from __future__ import annotations

import io
import os
import stat
import zipfile
from pathlib import Path

import pytest

from memento.skill_import import SkillImportConflictError, import_skill_pack


def pack(skill_md: str) -> bytes:
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, "w") as archive:
        archive.writestr("SKILL.md", skill_md)
        script = zipfile.ZipInfo("scripts/run.sh")
        script.external_attr = 0o100755 << 16
        archive.writestr(script, "#!/bin/sh\necho ok\n")
        archive.writestr("assets/icon.png", b"\x89PNG\r\n\x1a\nimage")
    return stream.getvalue()


def test_import_skill_pack_writes_complete_tree_non_executable(tmp_path: Path) -> None:
    skill_md = "# Demo\n"
    destination = import_skill_pack(
        workspace=tmp_path,
        skill_name="demo",
        version="1.0.0",
        skill_md=skill_md,
        zip_bytes=pack(skill_md),
    )
    assert destination == tmp_path / ".pi/skills/demo"
    assert (destination / "SKILL.md").read_text() == skill_md
    assert (destination / "assets/icon.png").read_bytes().startswith(b"\x89PNG")
    mode = os.stat(destination / "scripts/run.sh").st_mode
    assert stat.S_IMODE(mode) == 0o644


def test_import_skill_pack_fails_if_destination_exists(tmp_path: Path) -> None:
    destination = tmp_path / ".pi/skills/demo"
    destination.mkdir(parents=True)
    marker = destination / "local.txt"
    marker.write_text("keep")
    with pytest.raises(SkillImportConflictError):
        import_skill_pack(
            workspace=tmp_path,
            skill_name="demo",
            version="1.0.0",
            skill_md="# Demo\n",
            zip_bytes=pack("# Demo\n"),
        )
    assert marker.read_text() == "keep"


def test_import_skill_pack_leaves_no_partial_directory_on_validation_failure(
    tmp_path: Path,
) -> None:
    with pytest.raises(ValueError):
        import_skill_pack(
            workspace=tmp_path,
            skill_name="demo",
            version="1.0.0",
            skill_md="# Different\n",
            zip_bytes=pack("# Demo\n"),
        )
    assert not (tmp_path / ".pi/skills/demo").exists()
