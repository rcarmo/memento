from __future__ import annotations

import io
from typing import Any

import pytest

from memento.answers import ModelRequest
from memento.config import ModelEndpointConfig
from memento.model_clients import EndpointModelClient, ModelClientError


class Response(io.BytesIO):
    def getcode(self) -> int:
        return 200


def test_model_response_is_bounded(monkeypatch: pytest.MonkeyPatch) -> None:
    response = Response(b"x" * 2_000_000)
    sizes: list[int] = []
    original = response.read

    def read(size: int = -1) -> bytes:
        sizes.append(size)
        return original(size)

    monkeypatch.setattr(response, "read", read)

    def urlopen(*args: Any, **kwargs: Any) -> Response:
        return response

    monkeypatch.setattr("urllib.request.urlopen", urlopen)
    client = EndpointModelClient(
        ModelEndpointConfig(base_url="http://localhost", api_format="openai", model="test")
    )
    with pytest.raises(ModelClientError, match="1 MiB"):
        client.complete(
            ModelRequest(task="answer", prompt="test", max_output_chars=100, timeout_seconds=1)
        )
    assert sizes == [1_048_577]
