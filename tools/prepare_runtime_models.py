from __future__ import annotations

import argparse
import hashlib
import json
import os
import tarfile
import urllib.request
from pathlib import Path
from typing import Any, cast

MANIFEST_PATH = Path("models/runtime-models.json")


def load_manifest(root: Path = Path(".")) -> dict[str, Any]:
    return cast(dict[str, Any], json.loads((root / MANIFEST_PATH).read_text(encoding="utf-8")))


def bundle_key(root: Path = Path(".")) -> str:
    return hashlib.sha256((root / MANIFEST_PATH).read_bytes()).hexdigest()


def verify(root: Path, manifest: dict[str, Any]) -> None:
    for name, expected in cast(dict[str, str], manifest["files"]).items():
        path = root / name
        actual = hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else "missing"
        if actual != expected:
            raise SystemExit(f"runtime model digest mismatch for {name}: {actual} != {expected}")


def download(root: Path, token: str | None, manifest: dict[str, Any]) -> None:
    tag = str(manifest["release_tag"])
    asset = str(manifest["asset_name"])
    url = f"https://github.com/rcarmo/memento/releases/download/{tag}/{asset}"
    request = urllib.request.Request(url)
    if token:
        request.add_header("Authorization", f"Bearer {token}")
    archive = root / asset
    digest = hashlib.sha256()
    with urllib.request.urlopen(request, timeout=300) as response, archive.open("wb") as output:
        while chunk := response.read(1024 * 1024):
            digest.update(chunk)
            output.write(chunk)
    if digest.hexdigest() != manifest["asset_sha256"]:
        archive.unlink(missing_ok=True)
        raise SystemExit("runtime model release asset digest mismatch")
    with tarfile.open(archive) as bundle:
        members = bundle.getmembers()
        expected = set(cast(dict[str, str], manifest["files"]))
        names = {member.name for member in members}
        if names != expected or any(not member.isfile() for member in members):
            archive.unlink(missing_ok=True)
            raise SystemExit(f"unexpected runtime model archive members: {sorted(names)}")
        bundle.extractall(root, filter="data")
    archive.unlink()


def main() -> None:
    parser = argparse.ArgumentParser(description="Prepare verified runtime model artefacts")
    parser.add_argument("--root", type=Path, default=Path("."))
    parser.add_argument("--key-only", action="store_true")
    args = parser.parse_args()
    manifest = load_manifest(args.root)
    if args.key_only:
        print(bundle_key(args.root))
        return
    try:
        verify(args.root, manifest)
    except (SystemExit, FileNotFoundError):
        download(args.root, os.environ.get("GITHUB_TOKEN"), manifest)
        verify(args.root, manifest)
    print(bundle_key(args.root))


if __name__ == "__main__":
    main()
