from __future__ import annotations

import json
import re
from time import monotonic
from typing import Any, Literal, cast

from pydantic import BaseModel, ConfigDict, Field, TypeAdapter, ValidationError

from memento.envelopes import ErrorEnvelope, SuccessEnvelope, error_envelope
from memento.mcp_registry import OPERATION_SPEC_BY_OP

_SAVE_AS_RE = re.compile(r"^[A-Za-z][A-Za-z0-9_]{0,31}$")
_SEGMENT_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$|^[0-9]+$")


class EmptyArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)


class SearchArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    query: str
    concept_type: str | None = None
    limit: int = 20
    cursor: str | None = None
    search_mode: str | None = None


class ReadArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    id_or_path: str


class ListArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    path_prefix: str = "/"


class GraphArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    id_or_path: str
    depth: int = 1


class AuditArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    path: str | None = None


class AnswerArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    question: str
    answer_mode: str = "summary"


class ProposeArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    intent: str
    base_revision: str
    changes: list[dict[str, Any]]
    rationale: str | None = None


class ProposeFreeformArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    content: str
    suggested_path: str | None = None
    intent: str | None = None


class ProposeUpdateArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    instruction: str
    target_hint: str | None = None


class ProposalGetArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    proposal_id: str


class ProposalListArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    status: str | None = None


class ProposalReviewArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    proposal_id: str
    decision: str
    comment: str | None = None


class ProposalApplyArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    proposal_id: str
    expected_revision: str
    idempotency_key: str


class SkillSearchArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    query: str
    limit: int = 20


class SkillGetArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    skill_name: str
    version: str | None = None


class SkillProposeArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    skill_name: str
    version: str
    skill_md: str
    zip_base64: str
    rationale: str | None = None


class SkillProposalGetArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    proposal_id: str


class SkillProposalListArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    status: str | None = None


class SkillProposalReviewArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    proposal_id: str
    decision: str
    comment: str | None = None


class SkillProposalApplyArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    proposal_id: str
    expected_revision: str
    idempotency_key: str


class SkillPruneArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    skill_name: str
    keep: int = 5
    expected_revision: str
    idempotency_key: str


class CreateArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    path: str
    concept_type: str
    title: str
    body: str
    expected_revision: str
    idempotency_key: str
    description: str | None = None
    tags: tuple[str, ...] = ()
    aliases: tuple[str, ...] = ()


class PatchArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    path: str
    expected_revision: str
    idempotency_key: str
    title: str | None = None
    description: str | None = None
    body: str | None = None
    status: str | None = None
    tags: tuple[str, ...] | None = None
    aliases: tuple[str, ...] | None = None


class RenameArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    path: str
    new_path: str
    expected_revision: str
    idempotency_key: str


class ExecuteOperationBase(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    save_as: str | None = None


class HelpOperation(ExecuteOperationBase):
    op: Literal["help"]
    args: EmptyArgs = Field(default_factory=EmptyArgs)


class StatusOperation(ExecuteOperationBase):
    op: Literal["status"]
    args: EmptyArgs = Field(default_factory=EmptyArgs)


class SearchOperation(ExecuteOperationBase):
    op: Literal["search"]
    args: SearchArgs


class ReadOperation(ExecuteOperationBase):
    op: Literal["read"]
    args: ReadArgs


class ListOperation(ExecuteOperationBase):
    op: Literal["list"]
    args: ListArgs = Field(default_factory=ListArgs)


class GraphOperation(ExecuteOperationBase):
    op: Literal["graph"]
    args: GraphArgs


class AuditOperation(ExecuteOperationBase):
    op: Literal["audit"]
    args: AuditArgs = Field(default_factory=AuditArgs)


class AnswerOperation(ExecuteOperationBase):
    op: Literal["answer"]
    args: AnswerArgs


class ProposeOperation(ExecuteOperationBase):
    op: Literal["propose"]
    args: ProposeArgs


class ProposeFreeformOperation(ExecuteOperationBase):
    op: Literal["propose_freeform"]
    args: ProposeFreeformArgs


class ProposeUpdateOperation(ExecuteOperationBase):
    op: Literal["propose_update"]
    args: ProposeUpdateArgs


class ProposalGetOperation(ExecuteOperationBase):
    op: Literal["proposal_get"]
    args: ProposalGetArgs


class ProposalListOperation(ExecuteOperationBase):
    op: Literal["proposal_list"]
    args: ProposalListArgs = Field(default_factory=ProposalListArgs)


class ProposalReviewOperation(ExecuteOperationBase):
    op: Literal["proposal_review"]
    args: ProposalReviewArgs


class ProposalApplyOperation(ExecuteOperationBase):
    op: Literal["proposal_apply"]
    args: ProposalApplyArgs


class CreateOperation(ExecuteOperationBase):
    op: Literal["create"]
    args: CreateArgs


class PatchOperation(ExecuteOperationBase):
    op: Literal["patch"]
    args: PatchArgs


class RenameOperation(ExecuteOperationBase):
    op: Literal["rename"]
    args: RenameArgs


ExecuteOperation = (
    HelpOperation
    | StatusOperation
    | SearchOperation
    | ReadOperation
    | ListOperation
    | GraphOperation
    | AuditOperation
    | AnswerOperation
    | ProposeOperation
    | ProposeFreeformOperation
    | ProposeUpdateOperation
    | ProposalGetOperation
    | ProposalListOperation
    | ProposalReviewOperation
    | ProposalApplyOperation
    | CreateOperation
    | PatchOperation
    | RenameOperation
)

EXECUTE_OPERATION_ADAPTER: TypeAdapter[ExecuteOperation] = TypeAdapter(ExecuteOperation)


class ExecuteReturnProjection(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    name: str | None = None
    ref: str
    fields: tuple[str, ...] = ()
    limit: int | None = Field(default=None, ge=1)


class ExecutePlan(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    operations: tuple[ExecuteOperation, ...]
    stop_on_error: bool = True
    returns: tuple[ExecuteReturnProjection, ...] = ()


class ExecuteLimits(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    max_operations: int = Field(default=12, ge=1, le=32)
    max_intermediates: int = Field(default=12, ge=1, le=64)
    max_records: int = Field(default=50, ge=1, le=500)
    max_output_bytes: int = Field(default=65_536, ge=512)
    max_time_seconds: float = Field(default=3.0, gt=0, le=30)


def execute_plan_schema() -> dict[str, Any]:
    return ExecutePlan.model_json_schema()


class MemoryExecutor:
    def __init__(self, service: Any, limits: ExecuteLimits) -> None:
        self._service = service
        self._limits = limits

    def run(
        self, context: Any, *, plan: dict[str, Any]
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        try:
            parsed = ExecutePlan.model_validate(plan)
            if len(parsed.operations) > self._limits.max_operations:
                raise ValueError("plan exceeds configured max_operations")
            commit_ops = sum(
                1 for item in parsed.operations if OPERATION_SPEC_BY_OP[item.op].commit_capable
            )
            if commit_ops > 1:
                raise ValueError("plan may contain at most one commit-capable operation")
            started = monotonic()
            saved: dict[str, Any] = {}
            trace: list[dict[str, Any]] = []
            revisions: list[dict[str, Any]] = []
            last_success: dict[str, Any] | None = None
            stopped = False
            stop_reason: str | None = None
            warnings: list[str] = []
            commit_succeeded = False
            for index, item in enumerate(parsed.operations, start=1):
                self._check_time(started)
                if item.save_as is not None:
                    if not _SAVE_AS_RE.match(item.save_as):
                        raise ValueError(f"invalid save_as identifier: {item.save_as}")
                    if len(saved) >= self._limits.max_intermediates and item.save_as not in saved:
                        raise ValueError("plan exceeds configured max_intermediates")
                args = _resolve_references(item.args.model_dump(mode="python"), saved)
                envelope = self._dispatch(context, item.op, args)
                entry = {
                    "index": index,
                    "op": item.op,
                    "save_as": item.save_as,
                    "status": envelope.status,
                    "repo_revision": envelope.repo_revision,
                    "index_revision": envelope.index_revision,
                    "operation_id": envelope.operation_id,
                }
                if envelope.status == "success":
                    payload = _bound_value(envelope.data, self._limits.max_records)
                    entry["data"] = payload
                    last_success = payload
                    if item.save_as is not None:
                        saved[item.save_as] = payload
                    revisions.append(
                        {
                            "index": index,
                            "op": item.op,
                            "repo_revision": envelope.repo_revision,
                            "index_revision": envelope.index_revision,
                            "operation_id": envelope.operation_id,
                        }
                    )
                    if OPERATION_SPEC_BY_OP[item.op].commit_capable:
                        commit_succeeded = True
                else:
                    entry["error_class"] = envelope.error_class
                    entry["message"] = envelope.message
                    if parsed.stop_on_error:
                        stopped = True
                        stop_reason = f"operation {index} failed"
                trace.append(entry)
                self._ensure_output_size(
                    {"trace": trace, "revisions": revisions, "saved": saved},
                    commit_succeeded=commit_succeeded,
                )
                self._check_time(started)
                if entry["status"] == "error" and parsed.stop_on_error:
                    break
            returns = self._project_returns(parsed, saved, last_success)
            payload = {
                "trace": trace,
                "revisions": revisions,
                "returns": returns,
                "stopped": stopped,
                "stop_reason": stop_reason,
            }
            payload = self._fit_output_payload(payload, commit_succeeded=commit_succeeded)
            if payload.get("truncated"):
                warnings.append("memory_execute_output_truncated_after_commit")
            return cast(
                SuccessEnvelope[dict[str, Any]] | ErrorEnvelope,
                self._service._success(payload, warnings=tuple(warnings)),
            )
        except ValidationError as exc:
            return error_envelope("validation_error", str(exc))
        except ValueError as exc:
            return error_envelope("validation_error", str(exc))

    def _dispatch(
        self, context: Any, op_name: str, args: dict[str, Any]
    ) -> SuccessEnvelope[dict[str, Any]] | ErrorEnvelope:
        spec = OPERATION_SPEC_BY_OP[op_name]
        method = getattr(self._service, spec.method_name)
        return cast(SuccessEnvelope[dict[str, Any]] | ErrorEnvelope, method(context, **args))

    def _project_returns(
        self, plan: ExecutePlan, saved: dict[str, Any], last_success: dict[str, Any] | None
    ) -> dict[str, Any]:
        if not plan.returns:
            return {"result": last_success}
        projected: dict[str, Any] = {}
        for item in plan.returns:
            value = _resolve_reference(item.ref, saved)
            if item.fields:
                if not isinstance(value, list):
                    value = [value]
                extracted = []
                for row in value[: item.limit or self._limits.max_records]:
                    extracted.append({field: _extract_field(row, field) for field in item.fields})
                value = extracted
            elif item.limit is not None and isinstance(value, list):
                value = value[: item.limit]
            name = item.name or item.ref.removeprefix("$").replace(".", "_")
            projected[name] = _bound_value(value, self._limits.max_records)
        return projected

    def _check_time(self, started: float) -> None:
        if monotonic() - started > self._limits.max_time_seconds:
            raise ValueError("plan exceeded configured max_time_seconds")

    def _ensure_output_size(self, payload: dict[str, Any], *, commit_succeeded: bool) -> None:
        if self._payload_size(payload) <= self._limits.max_output_bytes:
            return
        if commit_succeeded:
            return
        raise ValueError("plan exceeded configured max_output_bytes")

    def _fit_output_payload(
        self, payload: dict[str, Any], *, commit_succeeded: bool
    ) -> dict[str, Any]:
        if self._payload_size(payload) <= self._limits.max_output_bytes:
            return {str(key): value for key, value in payload.items()}
        if not commit_succeeded:
            raise ValueError("plan exceeded configured max_output_bytes")
        loaded = json.loads(json.dumps(payload))
        if not isinstance(loaded, dict):
            raise ValueError("executor payload must remain an object")
        fitted: dict[str, Any] = {str(key): value for key, value in loaded.items()}
        fitted["truncated"] = True
        for entry in fitted.get("trace", []):
            if isinstance(entry, dict) and "data" in entry:
                entry["data"] = {"truncated": True}
        if self._payload_size(fitted) <= self._limits.max_output_bytes:
            return fitted
        fitted["returns"] = {"truncated": True}
        if self._payload_size(fitted) <= self._limits.max_output_bytes:
            return fitted
        trace = fitted.get("trace")
        if isinstance(trace, list) and len(trace) > 4:
            fitted["trace"] = [trace[0], {"truncated": True}, *trace[-2:]]
        if self._payload_size(fitted) <= self._limits.max_output_bytes:
            return fitted
        revisions = fitted.get("revisions")
        if isinstance(revisions, list) and len(revisions) > 2:
            fitted["revisions"] = [revisions[-1]]
        if self._payload_size(fitted) <= self._limits.max_output_bytes:
            return fitted
        fitted["trace"] = [{"truncated": True}]
        fitted["returns"] = {"truncated": True}
        fitted["revisions"] = (
            fitted.get("revisions", [])[-1:] if isinstance(fitted.get("revisions"), list) else []
        )
        if self._payload_size(fitted) <= self._limits.max_output_bytes:
            return fitted
        raise ValueError("plan exceeded configured max_output_bytes")

    def _payload_size(self, payload: dict[str, Any]) -> int:
        return len(json.dumps(payload, sort_keys=True, separators=(",", ":")).encode("utf-8"))


def _resolve_references(value: Any, saved: dict[str, Any]) -> Any:
    if isinstance(value, str) and value.startswith("$"):
        return _resolve_reference(value, saved)
    if isinstance(value, list):
        return [_resolve_references(item, saved) for item in value]
    if isinstance(value, tuple):
        return tuple(_resolve_references(item, saved) for item in value)
    if isinstance(value, dict):
        return {key: _resolve_references(item, saved) for key, item in value.items()}
    return value


def _resolve_reference(reference: str, saved: dict[str, Any]) -> Any:
    if not reference.startswith("$"):
        return reference
    parts = reference[1:].split(".")
    if not parts or not _SAVE_AS_RE.match(parts[0]):
        raise ValueError(f"invalid reference: {reference}")
    current = saved.get(parts[0])
    if current is None and parts[0] not in saved:
        raise ValueError(f"unknown reference: {reference}")
    for segment in parts[1:]:
        if not _SEGMENT_RE.match(segment):
            raise ValueError(f"invalid reference segment: {reference}")
        if isinstance(current, list):
            index = int(segment)
            if index >= len(current):
                raise ValueError(f"reference index out of range: {reference}")
            current = current[index]
            continue
        if not isinstance(current, dict):
            raise ValueError(f"reference does not resolve to a container: {reference}")
        if segment not in current:
            raise ValueError(f"reference field not found: {reference}")
        current = current[segment]
    return current


def _extract_field(value: Any, field_path: str) -> Any:
    parts = field_path.split(".")
    current = value
    for segment in parts:
        if not _SEGMENT_RE.match(segment):
            raise ValueError(f"invalid field path: {field_path}")
        if isinstance(current, list):
            current = current[int(segment)]
        elif isinstance(current, dict) and segment in current:
            current = current[segment]
        else:
            raise ValueError(f"field not found: {field_path}")
    return current


def _bound_value(value: Any, max_records: int) -> Any:
    if isinstance(value, list):
        return [_bound_value(item, max_records) for item in value[:max_records]]
    if isinstance(value, tuple):
        return [_bound_value(item, max_records) for item in value[:max_records]]
    if isinstance(value, dict):
        return {key: _bound_value(item, max_records) for key, item in value.items()}
    return value
