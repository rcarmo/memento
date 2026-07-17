from __future__ import annotations

from typing import Generic, Literal, TypeVar

from pydantic import BaseModel, ConfigDict, Field

T = TypeVar("T")


class ErrorEnvelope(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    status: Literal["error"]
    error_class: str = Field(min_length=1)
    message: str = Field(min_length=1)
    warnings: tuple[str, ...] = ()
    repo_revision: str | None = None
    index_revision: str | None = None
    index_stale: bool = False
    operation_id: str | None = None


class SuccessEnvelope(BaseModel, Generic[T]):
    model_config = ConfigDict(extra="forbid", frozen=True)

    status: Literal["success"]
    data: T
    warnings: tuple[str, ...] = ()
    next_tools: tuple[str, ...] = ()
    repo_revision: str
    index_revision: str
    index_stale: bool = False
    operation_id: str | None = None


def success_envelope(
    data: T,
    *,
    repo_revision: str,
    index_revision: str,
    warnings: tuple[str, ...] = (),
    next_tools: tuple[str, ...] = (),
    index_stale: bool = False,
    operation_id: str | None = None,
) -> SuccessEnvelope[T]:
    return SuccessEnvelope[T](
        status="success",
        data=data,
        warnings=warnings,
        next_tools=next_tools,
        repo_revision=repo_revision,
        index_revision=index_revision,
        index_stale=index_stale,
        operation_id=operation_id,
    )


def error_envelope(
    error_class: str,
    message: str,
    *,
    warnings: tuple[str, ...] = (),
    repo_revision: str | None = None,
    index_revision: str | None = None,
    index_stale: bool = False,
    operation_id: str | None = None,
) -> ErrorEnvelope:
    return ErrorEnvelope(
        status="error",
        error_class=error_class,
        message=message,
        warnings=warnings,
        repo_revision=repo_revision,
        index_revision=index_revision,
        index_stale=index_stale,
        operation_id=operation_id,
    )
