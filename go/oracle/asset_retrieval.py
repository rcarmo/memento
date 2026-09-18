"""Bounded file retrieval and manifest integrity on synthetic pack bytes."""

from __future__ import annotations

import base64
import copy
import importlib
import io
import struct
from typing import Any

from asset_pack import archive


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.asset_retrieval")
    packs: Any = importlib.import_module("memento.skill_packs")
    raw = archive(
        [
            ("dir/", b"", 0o040755),
            ("dir/text.md", "é日本\n".encode(), 0o100644),
            ("binary", b"\x00\xff\x01", 0o100644),
        ]
    )
    manifest = packs.validate_asset_pack(
        asset_kind="asset", version="1.0.0", zip_bytes=raw
    ).manifest
    cases = []
    for path in ["dir/text.md", "binary", "missing", "../bad", "/bad", "a\\b", "a\x00", "", "a//b"]:
        for offset, limit in [(0, 10), (1, 2), (0, 1), (10, 1), (-1, 1), (0, 0), (0, 262145)]:
            item: dict[str, Any] = {"path": path, "offset": offset, "limit": limit}
            try:
                item["expected"] = module.read_pack_file(
                    io.BytesIO(raw), manifest, file_path=path, offset=offset, limit=limit
                )
            except Exception as exc:
                item["error"] = str(exc)
                item["error_type"] = type(exc).__name__
            cases.append(item)
    invalid = []
    for kind in [
        "zip-digest",
        "manifest-digest",
        "count",
        "zero-count",
        "duplicate",
        "path",
        "file-digest",
        "size",
        "total",
    ]:
        value = copy.deepcopy(manifest.model_dump(mode="json"))
        digest = manifest.sha256
        if kind == "zip-digest":
            digest = "BAD"
        elif kind == "manifest-digest":
            value["sha256"] = "f" * 64
        elif kind == "count":
            value["file_count"] = 3
        elif kind == "zero-count":
            value["entries"] = []
            value["file_count"] = 0
        elif kind == "duplicate":
            value["entries"][1] = value["entries"][0]
        elif kind == "path":
            value["entries"][0]["path"] = "/bad"
        elif kind == "file-digest":
            value["entries"][0]["sha256"] = "Z" * 64
        elif kind == "size":
            value["entries"][0]["size"] = 16 * 1024 * 1024 + 1
        else:
            value["total_uncompressed_bytes"] = 0
        item = {"manifest": value, "digest": digest}
        try:
            module.checked_manifest(value, digest)
        except Exception as exc:
            item["error"] = str(exc)
        invalid.append(item)
    bad_zips = []
    malformed = {
        "duplicate": archive([("dir/text.md", "é日本\n".encode(), 0o100644)] * 2),
        "directory-mode": archive([("bad/", b"", 0o100644)]),
        "symlink": archive([("dir/text.md", "é日本\n".encode(), 0o120777)]),
        "extra": archive([("extra", b"x", 0o100644)]),
        "missing": archive([("dir/text.md", "é日本\n".encode(), 0o100644)]),
        "size": archive([("dir/text.md", b"x", 0o100644)]),
        "entries": archive([(f"d{i}/", b"", 0o040755) for i in range(1025)]),
        "bad-path": archive([("./dir/text.md", "é日本\n".encode(), 0o100644)]),
        "invalid": b"bad zip",
    }
    for kind in ["crc", "method", "digest"]:
        data = bytearray(raw)
        position = data.index(b"PK\x01\x02", data.index(b"PK\x01\x02") + 4)
        if kind == "method":
            struct.pack_into("<H", data, position + 10, 99)
        elif kind == "crc":
            struct.pack_into("<I", data, position + 16, 0)
        else:
            # Keep valid ZIP structure/CRC, but differ from the declared file digest.
            data = bytearray(
                archive(
                    [("dir/text.md", b"different", 0o100644), ("binary", b"\x00\xff\x01", 0o100644)]
                )
            )
        malformed[kind] = bytes(data)
    for name, payload in malformed.items():
        item = {"name": name, "zip": base64.b64encode(payload).decode()}
        try:
            module.read_pack_file(
                io.BytesIO(payload), manifest, file_path="dir/text.md", offset=0, limit=10
            )
        except Exception as exc:
            item["error"] = str(exc)
        bad_zips.append(item)
    return {
        "zip": base64.b64encode(raw).decode(),
        "manifest": manifest.model_dump(mode="json"),
        "manifest_digest": module.manifest_digest(manifest),
        "limits": module.retrieval_limits(),
        "cases": cases,
        "invalid_manifests": invalid,
        "bad_zips": bad_zips,
    }
