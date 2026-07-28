from __future__ import annotations

import argparse
import hashlib
import os
import re
import subprocess
import tarfile
import urllib.request
from pathlib import Path

RUNTIME_PATHS = (
    Path("models/gte/gte-small.gtemodel"),
    Path("models/needle/memento-router.ndl"),
    Path("models/needle/needle.model"),
)
POINTER_OID = re.compile(r"^oid sha256:([0-9a-f]{64})$", re.MULTILINE)
RELEASE_TAG = "model-assets-v1"


def pointer_text(path: Path) -> str:
    return subprocess.run(
        ["git", "show", f"HEAD:{path.as_posix()}"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout


def expected_oid(path: Path) -> str:
    match = POINTER_OID.search(pointer_text(path))
    if match is None:
        raise SystemExit(f"{path} is not a Git LFS pointer")
    return match.group(1)


def bundle_key() -> str:
    digest = hashlib.sha256()
    for path in RUNTIME_PATHS:
        digest.update(pointer_text(path).encode())
    return digest.hexdigest()


def verify(root: Path) -> None:
    for relative in RUNTIME_PATHS:
        path = root / relative
        actual = hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else "missing"
        expected = expected_oid(relative)
        if actual != expected:
            raise SystemExit(
                f"runtime model digest mismatch for {relative}: {actual} != {expected}"
            )


def download(root: Path, token: str | None) -> None:
    key = bundle_key()
    asset = f"runtime-models-{key}.tar"
    url = f"https://github.com/rcarmo/memento/releases/download/{RELEASE_TAG}/{asset}"
    request = urllib.request.Request(url)
    if token:
        request.add_header("Authorization", f"Bearer {token}")
    archive = root / asset
    with urllib.request.urlopen(request, timeout=300) as response, archive.open("wb") as output:
        while chunk := response.read(1024 * 1024):
            output.write(chunk)
    with tarfile.open(archive) as bundle:
        members = {member.name for member in bundle.getmembers() if member.isfile()}
        expected = {path.as_posix() for path in RUNTIME_PATHS}
        if members != expected:
            raise SystemExit(f"unexpected runtime model archive members: {sorted(members)}")
        bundle.extractall(root, filter="data")
    archive.unlink()


def main() -> None:
    parser = argparse.ArgumentParser(description="Prepare verified runtime model artefacts")
    parser.add_argument("--root", type=Path, default=Path("."))
    parser.add_argument("--key-only", action="store_true")
    args = parser.parse_args()
    if args.key_only:
        print(bundle_key())
        return
    try:
        verify(args.root)
    except (SystemExit, FileNotFoundError):
        download(args.root, os.environ.get("GITHUB_TOKEN"))
        verify(args.root)
    print(bundle_key())


if __name__ == "__main__":
    main()
