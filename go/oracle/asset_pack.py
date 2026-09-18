"""Synthetic ZIP validation cases and pinned Python MIME lookup table."""

from __future__ import annotations

import base64
import hashlib
import importlib
import io
import mimetypes
import platform
import struct
import warnings
import zipfile
from pathlib import Path
from typing import Any


def archive(entries: list[tuple[str, bytes, int]], method: int = 0) -> bytes:
    output = io.BytesIO()
    with warnings.catch_warnings(), zipfile.ZipFile(output, "w", compression=method) as z:
        warnings.simplefilter("ignore")
        for name, data, mode in entries:
            item = zipfile.ZipInfo(name, (2026, 9, 18, 0, 0, 0))
            item.compress_type = method
            item.create_system = 3
            item.external_attr = mode << 16
            z.writestr(item, data)
    return output.getvalue()


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.skill_packs")
    paths: Any = importlib.import_module("memento.repository.asset_packs")
    cases: list[dict[str, Any]] = []

    def add(
        name: str,
        entries: list[tuple[str, bytes, int]],
        *,
        method: int = 0,
        version: str = "1.0.0",
        kind: str = "asset",
        skill: bool = False,
        raw: bytes | None = None,
    ) -> None:
        payload = archive(entries, method) if raw is None else raw
        item: dict[str, Any] = {
            "name": name,
            "zip": base64.b64encode(payload).decode(),
            "version": version,
            "kind": kind,
            "skill": skill,
        }
        try:
            if skill:
                pack = module.validate_skill_pack(
                    skill_name=kind, version=version, skill_md="# Synthetic\n", zip_bytes=payload
                )
            else:
                pack = module.validate_asset_pack(
                    asset_kind=kind, version=version, zip_bytes=payload
                )
            item["expected"] = pack.manifest.model_dump(mode="json")
        except Exception as exc:
            item["error"] = str(exc)
            item["error_type"] = type(exc).__name__
        cases.append(item)

    add("empty", [])
    add(
        "stored",
        [
            ("doc.md", b"# Synthetic\n", 0o100644),
            ("data.json", b"{}", 0o100644),
            ("bin.dat", b"\x00\xff", 0o100644),
        ],
    )
    for method in [zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED, zipfile.ZIP_BZIP2]:
        add(
            f"method-{method}",
            [("SKILL.md", b"# Synthetic\n", 0o100644)],
            method=method,
            skill=True,
            kind="demo-skill",
        )
    for path in [
        "../evil",
        "/absolute",
        "a\\b",
        "a/../b",
        "./a.txt",
        "a//b.txt",
        "a/./b.txt",
        ".",
        "a\x01",
        "é/日本.md",
        "dir/",
        "dir",
        "nested.ZIP",
        "archive.tar.gz",
        "image.gz",
        "code.exe",
    ]:
        add("path-" + repr(path), [(path, b"harmless", 0o100644)])
    add("directory", [("dir/", b"", 0o040755), ("dir/a.txt", b"hello", 0o100644)])
    add("duplicates", [("a", b"a", 0o100644), ("./a", b"b", 0o100644)])
    add("raw-duplicates", [("a", b"a", 0o100644), ("a", b"b", 0o100644)])
    for mode in [0, 0o644, 0o100755, 0o120777, 0o010600, 0o020600]:
        add(f"mode-{mode}", [("file", b"x", mode)])
    for raw in [
        b"\x7fELF",
        b"MZ",
        b"\xfe\xed\xfa\xce",
        b"\xfe\xed\xfa\xcf",
        b"\xce\xfa\xed\xfe",
        b"\xcf\xfa\xed\xfe",
        b"\xca\xfe\xba\xbe",
        b"\xca\xfe\xba\xbe\x00\x00\x00\x01",
        b"\xca\xfe\xba\xbe\x00\x00\x00\x00",
        b"\xca\xfe\xba\xbe\x00\x00\x01\x00",
    ]:
        add("magic-" + raw.hex(), [("file", raw, 0o100644)])
    add("missing-root", [("nested/SKILL.md", b"# Synthetic\n", 0o100644)], skill=True, kind="demo")
    add("wrong-root", [("SKILL.md", b"different", 0o100644)], skill=True, kind="demo")
    add("bad-skill", [], skill=True, kind="INVALID")
    add("generic-kind-not-validated", [], kind="INVALID")
    add("bad-version", [], version="v1.0.0")
    add("invalid", [], raw=b"not a zip")
    add("ratio", [("large.txt", b"a" * 20000, 0o100644)], method=zipfile.ZIP_DEFLATED)
    # Modify only central directory metadata; limits must reject before reading.
    basic = archive([("file", b"x", 0o100644)])
    for name, field, value in [
        ("encrypted", 8, 1),
        ("file-limit", 24, module.MAX_FILE_BYTES + 1),
        ("compressed-zero", 20, 0),
    ]:
        data = bytearray(basic)
        offset = data.index(b"PK\x01\x02")
        struct.pack_into("<H" if field == 8 else "<I", data, offset + field, value)
        add(name, [], raw=bytes(data))
    add("file-count", [(f"f{i}", b"", 0o100644) for i in range(513)])
    add("entry-count", [(f"d{i}/", b"", 0o040755) for i in range(1025)])
    versions = []
    for version in [
        "0.0.0",
        "1.2.3",
        "01.2.3",
        "1.2",
        "1.2.3-rc1",
        "1.2.3+build",
        "1.2.3\n",
        "1.٢.3",
        "1٢.3.4",
        "１.2.3",
        "1234567890123456789012345.0.1",
    ]:
        item = {"input": version}
        try:
            item["expected"] = module.parse_stable_semver(version)
        except Exception as exc:
            item["error"] = str(exc)
        versions.append(item)
    kinds = []
    for kind in ["skill", "asset-pack", "a1", "a--b", "A", "", "-a", "a_", "é"]:
        item = {"input": kind}
        try:
            item["expected"] = paths.validate_asset_kind(kind)
        except Exception as exc:
            item["error"] = str(exc)
        kinds.append(item)
    mimetypes.init()
    return {
        "environment": {
            "python": platform.python_version(),
            "mime_sources": {
                name: hashlib.sha256(Path(name).read_bytes()).hexdigest()
                for name in mimetypes.knownfiles
                if Path(name).is_file()
            },
        },
        "cases": cases,
        "versions": versions,
        "kinds": kinds,
        "mime": {
            "types": mimetypes.types_map,
            "common": mimetypes.common_types,
            "suffixes": mimetypes.suffix_map,
            "encodings": mimetypes.encodings_map,
        },
    }
