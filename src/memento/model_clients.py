from __future__ import annotations

import json
import socket
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
from collections.abc import Mapping
from dataclasses import dataclass
from typing import Any

from pydantic import BaseModel, ConfigDict

from memento.answers import ModelAttempt, ModelClient, ModelRequest, ModelResponse
from memento.config import ModelEndpointConfig, ModelProviderSlotsConfig, ModelSlotConfig


class ModelClientError(RuntimeError):
    retryable: bool = False


class ModelCancelledError(ModelClientError):
    pass


class ModelPolicyError(ModelClientError):
    pass


class ModelConnectionError(ModelClientError):
    retryable = True


class ModelTimeoutError(ModelClientError):
    retryable = True


class ModelHTTPError(ModelClientError):
    def __init__(self, status_code: int, message: str, *, retryable: bool) -> None:
        super().__init__(message)
        self.status_code = status_code
        self.retryable = retryable


class ModelValidationError(ModelClientError):
    pass


class _OpenAIResponse(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    model: str | None = None
    choices: list[dict[str, Any]]
    usage: dict[str, int] | None = None


class _AnthropicResponse(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    model: str | None = None
    content: list[dict[str, Any]]
    usage: dict[str, int] | None = None


class EndpointModelClient(ModelClient):
    def __init__(self, endpoint: ModelEndpointConfig, *, api_key: str | None = None) -> None:
        self._endpoint = endpoint
        self._api_key = api_key

    def complete(self, request: ModelRequest) -> ModelResponse:
        if request.cancelled is not None and request.cancelled():
            raise ModelCancelledError("cancelled before model call")
        payload, headers, url = self._compose_request(request)
        try:
            http_request = urllib.request.Request(
                url=url,
                data=json.dumps(payload).encode("utf-8"),
                headers=headers,
                method="POST",
            )
            with urllib.request.urlopen(http_request, timeout=request.timeout_seconds) as response:
                status_code = response.getcode()
                body = response.read().decode("utf-8")
        except urllib.error.HTTPError as exc:
            status_code = exc.code
            body = exc.read().decode("utf-8", errors="replace")
            raise ModelHTTPError(
                status_code,
                body or f"HTTP {status_code}",
                retryable=False,
            ) from exc
        except urllib.error.URLError as exc:
            if isinstance(exc.reason, socket.timeout):
                raise ModelTimeoutError(str(exc.reason) or "timeout") from exc
            raise ModelConnectionError(str(exc.reason) or "connection failure") from exc
        except TimeoutError as exc:
            raise ModelTimeoutError("timeout") from exc
        if request.cancelled is not None and request.cancelled():
            raise ModelCancelledError("cancelled after model call")
        if status_code < 200 or status_code >= 300:
            raise ModelHTTPError(status_code, body or f"HTTP {status_code}", retryable=False)
        return self._parse_response(body)

    @property
    def model_name(self) -> str:
        return self._endpoint.model

    @property
    def trust_boundary(self) -> str:
        host = urllib.parse.urlparse(self._endpoint.base_url).hostname or ""
        return "local" if host in {"localhost", "127.0.0.1", "::1"} else "remote"

    def _compose_request(self, request: ModelRequest) -> tuple[dict[str, Any], dict[str, str], str]:
        headers = {
            "Accept": "application/json",
            "Content-Type": "application/json",
            **self._endpoint.headers,
        }
        if self._api_key is not None:
            if self._endpoint.api_format == "anthropic":
                headers.setdefault("x-api-key", self._api_key)
                headers.setdefault("anthropic-version", "2023-06-01")
            else:
                headers.setdefault("Authorization", f"Bearer {self._api_key}")
        if self._endpoint.api_format == "anthropic":
            url = f"{self._endpoint.base_url}/v1/messages"
            payload = {
                "model": self._endpoint.model,
                "max_tokens": max(1, min(request.max_output_chars, 8192)),
                "messages": [{"role": "user", "content": request.prompt}],
            }
        else:
            url = f"{self._endpoint.base_url}/v1/chat/completions"
            payload = {
                "model": self._endpoint.model,
                "stream": False,
                "messages": [{"role": "user", "content": request.prompt}],
                "max_tokens": max(1, min(request.max_output_chars, 8192)),
            }
        return payload, headers, url

    def _parse_response(self, body: str) -> ModelResponse:
        try:
            payload = json.loads(body)
        except json.JSONDecodeError as exc:
            raise ModelValidationError(f"model response is not valid JSON: {exc}") from exc
        if self._endpoint.api_format == "anthropic":
            anthropic = _AnthropicResponse.model_validate(payload)
            text = "".join(
                str(item.get("text", ""))
                for item in anthropic.content
                if item.get("type") == "text"
            )
            if not text:
                raise ModelValidationError("anthropic response contained no text content")
            return ModelResponse(
                model_name=anthropic.model or self._endpoint.model,
                output_text=text,
                usage=dict(anthropic.usage or {}),
            )
        openai = _OpenAIResponse.model_validate(payload)
        if not openai.choices:
            raise ModelValidationError("openai-compatible response contained no choices")
        message = openai.choices[0].get("message")
        if not isinstance(message, dict) or not isinstance(message.get("content"), str):
            raise ModelValidationError(
                "openai-compatible response missing choices[0].message.content"
            )
        return ModelResponse(
            model_name=openai.model or self._endpoint.model,
            output_text=message["content"],
            usage=dict(openai.usage or {}),
        )


@dataclass(frozen=True, slots=True)
class _ConfiguredEndpoint:
    endpoint: ModelEndpointConfig
    client: ModelClient

    @property
    def model_name(self) -> str:
        return self.endpoint.model

    @property
    def trust_boundary(self) -> str:
        host = urllib.parse.urlparse(self.endpoint.base_url).hostname or ""
        return "local" if host in {"localhost", "127.0.0.1", "::1"} else "remote"


class RoutedFallbackModelClient(ModelClient):
    _TASK_TO_SLOT = {
        "memory_answer_hot": "hot_query",
        "memory_answer_deep": "deep_query",
        "memory_proposal_draft": "proposal",
        "dream_proposal_draft": "dream",
    }

    def __init__(
        self, slots: ModelProviderSlotsConfig, *, endpoint_clients: Mapping[str, ModelClient]
    ) -> None:
        self._slots = slots
        self._endpoint_clients = endpoint_clients
        self._semaphores = {
            "hot_query": threading.BoundedSemaphore(slots.hot_query.concurrency_limit),
            "deep_query": threading.BoundedSemaphore(slots.deep_query.concurrency_limit),
            "proposal": threading.BoundedSemaphore(slots.proposal.concurrency_limit),
            "dream": threading.BoundedSemaphore(slots.dream.concurrency_limit),
        }

    def complete(self, request: ModelRequest) -> ModelResponse:
        slot_name = request.slot_name or self._TASK_TO_SLOT.get(request.task)
        if slot_name is None:
            raise ModelPolicyError(f"no model slot for task {request.task}")
        slot = getattr(self._slots, slot_name)
        configured = self._configured_chain(slot)
        if not configured:
            raise ModelPolicyError(f"model slot {slot_name} is not configured")
        if request.data_classification not in slot.allowed_data_classifications:
            raise ModelPolicyError(
                f"slot {slot_name} disallows data classification {request.data_classification}"
            )
        semaphore = self._semaphores[slot_name]
        with self._acquire_slot(semaphore, request=request, slot_name=slot_name):
            return self._complete_with_slot(
                request, slot_name=slot_name, slot=slot, chain=configured
            )

    def _acquire_slot(
        self, semaphore: threading.BoundedSemaphore, *, request: ModelRequest, slot_name: str
    ) -> Any:
        deadline = time.monotonic() + request.timeout_seconds

        class _SemaphoreLease:
            def __enter__(inner_self) -> None:
                while True:
                    if request.cancelled is not None and request.cancelled():
                        raise ModelCancelledError(
                            f"cancelled while waiting for model slot {slot_name}"
                        )
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        raise ModelTimeoutError(f"timed out waiting for model slot {slot_name}")
                    if semaphore.acquire(timeout=min(0.05, remaining)):
                        return None

            def __exit__(inner_self, exc_type: object, exc: object, tb: object) -> None:
                semaphore.release()
                return None

        return _SemaphoreLease()

    def _configured_chain(self, slot: ModelSlotConfig) -> list[_ConfiguredEndpoint]:
        endpoints = []
        for endpoint in ([slot.primary] if slot.primary is not None else []) + list(slot.fallbacks):
            key = self._endpoint_key(endpoint)
            client = self._endpoint_clients.get(key)
            if client is not None:
                endpoints.append(_ConfiguredEndpoint(endpoint=endpoint, client=client))
        return endpoints

    def _complete_with_slot(
        self,
        request: ModelRequest,
        *,
        slot_name: str,
        slot: ModelSlotConfig,
        chain: list[_ConfiguredEndpoint],
    ) -> ModelResponse:
        attempts: list[ModelAttempt] = []
        primary_boundary = chain[0].trust_boundary
        allowed_chain = chain[:1]
        if slot.fallback_enabled:
            for candidate in chain[1:]:
                if (
                    candidate.trust_boundary != primary_boundary
                    and not slot.allow_cross_trust_boundary
                ):
                    continue
                allowed_chain.append(candidate)
        for endpoint_index, configured in enumerate(allowed_chain):
            retries_remaining = slot.retry_budget
            while True:
                try:
                    response = configured.client.complete(
                        request.model_copy(
                            update={
                                "slot_name": slot_name,
                                "timeout_seconds": min(
                                    request.timeout_seconds, slot.timeout_seconds
                                ),
                                "max_output_chars": min(
                                    request.max_output_chars, slot.max_output_chars
                                ),
                            }
                        )
                    )
                    return response.model_copy(
                        update={
                            "model_chain": tuple(
                                attempts
                                + [ModelAttempt(model=response.model_name, outcome="success")]
                            )
                        }
                    )
                except ModelCancelledError:
                    attempts.append(ModelAttempt(model=configured.model_name, outcome="cancelled"))
                    raise
                except ModelPolicyError:
                    attempts.append(
                        ModelAttempt(model=configured.model_name, outcome="policy_denied")
                    )
                    raise
                except ModelValidationError:
                    attempts.append(
                        ModelAttempt(model=configured.model_name, outcome="invalid_output")
                    )
                    raise
                except ModelHTTPError as exc:
                    retryable = self._retryable_http(slot, exc.status_code)
                    outcome = f"http_{exc.status_code}"
                    attempts.append(ModelAttempt(model=configured.model_name, outcome=outcome))
                    if retryable and retries_remaining > 0:
                        retries_remaining -= 1
                        continue
                    if retryable and endpoint_index + 1 < len(allowed_chain):
                        break
                    raise
                except (ModelConnectionError, ModelTimeoutError) as exc:
                    outcome = (
                        "timeout" if isinstance(exc, ModelTimeoutError) else "connection_failed"
                    )
                    attempts.append(ModelAttempt(model=configured.model_name, outcome=outcome))
                    if retries_remaining > 0:
                        retries_remaining -= 1
                        continue
                    if endpoint_index + 1 < len(allowed_chain):
                        break
                    raise
        raise ModelConnectionError("all model attempts failed")

    def _retryable_http(self, slot: ModelSlotConfig, status_code: int) -> bool:
        if status_code >= 500:
            return True
        if status_code in slot.overload_status_codes:
            return True
        if status_code == 429:
            return slot.fallback_on_rate_limit
        return False

    def _endpoint_key(self, endpoint: ModelEndpointConfig) -> str:
        return json.dumps(endpoint.model_dump(mode="json"), sort_keys=True)


def build_endpoint_clients(
    slots: ModelProviderSlotsConfig,
    *,
    api_keys: dict[str, str],
) -> dict[str, EndpointModelClient]:
    clients: dict[str, EndpointModelClient] = {}
    for slot in (slots.hot_query, slots.deep_query, slots.proposal, slots.dream):
        for endpoint in ([slot.primary] if slot.primary is not None else []) + list(slot.fallbacks):
            key = json.dumps(endpoint.model_dump(mode="json"), sort_keys=True)
            if key not in clients:
                api_key = None
                if endpoint.api_key_env is not None:
                    api_key = api_keys.get(endpoint.api_key_env)
                clients[key] = EndpointModelClient(endpoint, api_key=api_key)
    return clients
