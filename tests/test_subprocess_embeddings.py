from __future__ import annotations

import stat
from pathlib import Path

import pytest

from memento.semantic import SemanticSearchError
from memento.subprocess_embeddings import SubprocessEmbeddingClient


def fake_worker(path: Path, *, dimensions: int = 4, fail: bool = False) -> None:
    body = f"""#!/usr/bin/env python3
import json,struct,sys
if {fail!r}:
    print("worker failed", file=sys.stderr)
    raise SystemExit(7)
raw=sys.stdin.buffer.read()
size=struct.unpack_from("<I",raw)[0]
request=json.loads(raw[4:4+size])
texts=request["texts"]
values=[]
for i,_text in enumerate(texts):
    values.extend([float(i+1)]*{dimensions})
payload=struct.pack("<{{}}f".format(len(values)),*values)
header=json.dumps({{"id":"batch","ok":True,"method":"embed_batch","dimensions":{dimensions},"count":len(texts),"payload_len":len(payload),"error":None}},separators=(",",":")).encode()
total=4+len(header)+len(payload)
sys.stdout.buffer.write(struct.pack("<II",total,len(header))+header+payload)
"""
    path.write_text(body)
    path.chmod(path.stat().st_mode | stat.S_IXUSR)


def model_file(path: Path) -> None:
    path.write_bytes(b"model")


def test_subprocess_embedding_client_batches_and_exits(tmp_path: Path) -> None:
    worker = tmp_path / "worker.py"
    model = tmp_path / "model.gte"
    fake_worker(worker)
    model_file(model)
    client = SubprocessEmbeddingClient(
        worker,
        model,
        dimensions=4,
        max_batch=3,
        max_input_chars=20,
    )
    assert client.embed_batch(("one", "two")) == (
        (1.0, 1.0, 1.0, 1.0),
        (2.0, 2.0, 2.0, 2.0),
    )
    assert client.embed("one") == (1.0, 1.0, 1.0, 1.0)


def test_subprocess_embedding_client_enforces_limits_and_errors(tmp_path: Path) -> None:
    worker = tmp_path / "worker.py"
    model = tmp_path / "model.gte"
    fake_worker(worker)
    model_file(model)
    client = SubprocessEmbeddingClient(
        worker,
        model,
        dimensions=4,
        max_batch=1,
        max_input_chars=3,
    )
    with pytest.raises(SemanticSearchError, match="maximum"):
        client.embed_batch(("one", "two"))
    with pytest.raises(SemanticSearchError, match="character limit"):
        client.embed("four")

    failed_worker = tmp_path / "failed.py"
    fake_worker(failed_worker, fail=True)
    failed = SubprocessEmbeddingClient(
        failed_worker,
        model,
        dimensions=4,
        max_batch=1,
        max_input_chars=10,
    )
    with pytest.raises(SemanticSearchError, match="exited 7"):
        failed.embed("one")


def test_subprocess_embedding_client_cancellation(tmp_path: Path) -> None:
    worker = tmp_path / "worker.py"
    model = tmp_path / "model.gte"
    fake_worker(worker)
    model_file(model)
    client = SubprocessEmbeddingClient(
        worker,
        model,
        dimensions=4,
        max_batch=1,
        max_input_chars=10,
    )
    with pytest.raises(SemanticSearchError, match="cancelled"):
        client.embed("one", cancelled=lambda: True)
