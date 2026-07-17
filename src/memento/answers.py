from __future__ import annotations

import hashlib
import json
import sqlite3
from collections.abc import Callable, Sequence
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Protocol
from uuid import uuid4

from pydantic import BaseModel, ConfigDict, Field

UNKNOWN_ANSWER = "UNKNOWN"


class ModelRequest(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True, arbitrary_types_allowed=True)

    task: str
    prompt: str
    max_output_chars: int = Field(ge=1)
    timeout_seconds: float = Field(gt=0)
    metadata: dict[str, str] = Field(default_factory=dict)
    cancelled: Callable[[], bool] | None = None


class ModelResponse(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    model_name: str
    output_text: str
    usage: dict[str, int] = Field(default_factory=dict)


class ModelClient(Protocol):
    def complete(self, request: ModelRequest) -> ModelResponse: ...


class AnswerCitation(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    id: str
    path: str
    revision: str


class AnswerRecord(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    answer: str
    answer_source: str
    confidence: str
    unresolved: tuple[str, ...] = ()
    citations: tuple[AnswerCitation, ...] = ()
    trace_id: str | None = None
    model_chain: tuple[str, ...] = ()


@dataclass(frozen=True, slots=True)
class ReadConcept:
    concept_id: str
    path: str
    title: str
    body: str
    revision: str


@dataclass(frozen=True, slots=True)
class SearchStep:
    action: str
    detail: str


@dataclass(frozen=True, slots=True)
class DeepAnswerResult:
    record: AnswerRecord
    read_concepts: tuple[ReadConcept, ...]
    steps: tuple[SearchStep, ...]
    duration_ms: int
    usage: dict[str, int]


class AnswerStore:
    def __init__(self, connection: sqlite3.Connection) -> None:
        self._connection = connection

    def migrate(self) -> None:
        with self._connection:
            self._connection.execute(
                """
                CREATE TABLE IF NOT EXISTS answer_cache (
                    cache_key TEXT PRIMARY KEY,
                    scope_key TEXT NOT NULL,
                    repo_revision TEXT NOT NULL,
                    normalized_question TEXT NOT NULL,
                    answer_mode TEXT NOT NULL,
                    response_json TEXT NOT NULL,
                    cited_ids_json TEXT NOT NULL,
                    read_ids_json TEXT NOT NULL,
                    created_at TEXT NOT NULL,
                    expires_at TEXT NOT NULL,
                    last_accessed_at TEXT NOT NULL
                )
                """
            )
            self._connection.execute(
                """
                CREATE TABLE IF NOT EXISTS hot_changed_concepts (
                    scope_key TEXT NOT NULL,
                    concept_id TEXT NOT NULL,
                    observed_at TEXT NOT NULL,
                    PRIMARY KEY(scope_key, concept_id)
                )
                """
            )
            self._connection.execute(
                """
                CREATE TABLE IF NOT EXISTS hot_answers (
                    scope_key TEXT NOT NULL,
                    question_hash TEXT NOT NULL,
                    normalized_question TEXT NOT NULL,
                    response_json TEXT NOT NULL,
                    concept_ids_json TEXT NOT NULL,
                    created_at TEXT NOT NULL,
                    expires_at TEXT NOT NULL,
                    PRIMARY KEY(scope_key, question_hash)
                )
                """
            )
            self._connection.execute(
                """
                CREATE TABLE IF NOT EXISTS answer_traces (
                    trace_id TEXT PRIMARY KEY,
                    principal TEXT NOT NULL,
                    scope_key TEXT NOT NULL,
                    question_hash TEXT NOT NULL,
                    question_excerpt TEXT NOT NULL,
                    repo_revision TEXT NOT NULL,
                    answer_summary TEXT NOT NULL,
                    steps_json TEXT NOT NULL,
                    paths_json TEXT NOT NULL,
                    model_chain_json TEXT NOT NULL,
                    usage_json TEXT NOT NULL,
                    duration_ms INTEGER NOT NULL,
                    created_at TEXT NOT NULL
                )
                """
            )
            self._connection.execute(
                "CREATE INDEX IF NOT EXISTS idx_answer_cache_lru ON answer_cache(last_accessed_at)"
            )
            self._connection.execute(
                "CREATE INDEX IF NOT EXISTS idx_hot_changed_scope ON hot_changed_concepts(scope_key, observed_at DESC)"
            )
            self._connection.execute(
                "CREATE INDEX IF NOT EXISTS idx_hot_answers_scope ON hot_answers(scope_key, created_at DESC)"
            )
            self._connection.execute(
                "CREATE INDEX IF NOT EXISTS idx_answer_traces_created ON answer_traces(created_at DESC)"
            )

    def get_exact_cache(self, *, cache_key: str, now: datetime) -> AnswerRecord | None:
        row = self._connection.execute(
            "SELECT response_json, expires_at FROM answer_cache WHERE cache_key = ?",
            (cache_key,),
        ).fetchone()
        if row is None:
            return None
        if row["expires_at"] <= _iso(now):
            with self._connection:
                self._connection.execute(
                    "DELETE FROM answer_cache WHERE cache_key = ?", (cache_key,)
                )
            return None
        with self._connection:
            self._connection.execute(
                "UPDATE answer_cache SET last_accessed_at = ? WHERE cache_key = ?",
                (_iso(now), cache_key),
            )
        record = AnswerRecord.model_validate_json(str(row["response_json"]))
        return record.model_copy(update={"answer_source": "exact_cache"})

    def put_exact_cache(
        self,
        *,
        cache_key: str,
        scope_key: str,
        repo_revision: str,
        normalized_question: str,
        answer_mode: str,
        record: AnswerRecord,
        cited_ids: Sequence[str],
        read_ids: Sequence[str],
        now: datetime,
        ttl_seconds: int,
        max_entries: int,
    ) -> None:
        expires_at = now + timedelta(seconds=ttl_seconds)
        with self._connection:
            self._connection.execute(
                """
                INSERT OR REPLACE INTO answer_cache(
                    cache_key, scope_key, repo_revision, normalized_question, answer_mode,
                    response_json, cited_ids_json, read_ids_json, created_at, expires_at,
                    last_accessed_at
                ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    cache_key,
                    scope_key,
                    repo_revision,
                    normalized_question,
                    answer_mode,
                    record.model_dump_json(),
                    json.dumps(sorted(dict.fromkeys(cited_ids))),
                    json.dumps(sorted(dict.fromkeys(read_ids))),
                    _iso(now),
                    _iso(expires_at),
                    _iso(now),
                ),
            )
            self._connection.execute("DELETE FROM answer_cache WHERE expires_at <= ?", (_iso(now),))
            self._prune_lru("answer_cache", max_entries)

    def get_hot_context(
        self,
        *,
        scope_key: str,
        normalized_question: str,
        now: datetime,
    ) -> tuple[list[str], AnswerRecord | None]:
        with self._connection:
            self._connection.execute(
                "DELETE FROM hot_changed_concepts WHERE observed_at <= ?",
                (_iso(now - timedelta(days=3650)),),
            )
            self._connection.execute("DELETE FROM hot_answers WHERE expires_at <= ?", (_iso(now),))
        changed_rows = self._connection.execute(
            "SELECT concept_id FROM hot_changed_concepts WHERE scope_key = ? ORDER BY observed_at DESC LIMIT 10",
            (scope_key,),
        ).fetchall()
        hot_row = self._connection.execute(
            "SELECT response_json FROM hot_answers WHERE scope_key = ? AND question_hash = ? AND expires_at > ?",
            (scope_key, _hash_text(normalized_question), _iso(now)),
        ).fetchone()
        changed_ids = [str(row["concept_id"]) for row in changed_rows]
        hot_answer = (
            None
            if hot_row is None
            else AnswerRecord.model_validate_json(str(hot_row["response_json"]))
        )
        return changed_ids, hot_answer

    def list_hot_answers(self, *, scope_key: str, now: datetime, limit: int) -> list[AnswerRecord]:
        self._connection.execute("DELETE FROM hot_answers WHERE expires_at <= ?", (_iso(now),))
        rows = self._connection.execute(
            "SELECT response_json FROM hot_answers WHERE scope_key = ? ORDER BY created_at DESC LIMIT ?",
            (scope_key, limit),
        ).fetchall()
        return [AnswerRecord.model_validate_json(str(row["response_json"])) for row in rows]

    def put_hot_changed_concepts(
        self, *, scope_key: str, concept_ids: Sequence[str], now: datetime, max_entries: int
    ) -> None:
        with self._connection:
            for concept_id in concept_ids:
                self._connection.execute(
                    "INSERT OR REPLACE INTO hot_changed_concepts(scope_key, concept_id, observed_at) VALUES(?, ?, ?)",
                    (scope_key, concept_id, _iso(now)),
                )
            rows = self._connection.execute(
                "SELECT concept_id FROM hot_changed_concepts WHERE scope_key = ? ORDER BY observed_at DESC",
                (scope_key,),
            ).fetchall()
            for row in rows[max_entries:]:
                self._connection.execute(
                    "DELETE FROM hot_changed_concepts WHERE scope_key = ? AND concept_id = ?",
                    (scope_key, str(row["concept_id"])),
                )

    def put_hot_answer(
        self,
        *,
        scope_key: str,
        normalized_question: str,
        record: AnswerRecord,
        concept_ids: Sequence[str],
        now: datetime,
        ttl_seconds: int,
        max_entries: int,
    ) -> None:
        expires_at = now + timedelta(seconds=ttl_seconds)
        with self._connection:
            self._connection.execute(
                """
                INSERT OR REPLACE INTO hot_answers(
                    scope_key, question_hash, normalized_question, response_json,
                    concept_ids_json, created_at, expires_at
                ) VALUES(?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    scope_key,
                    _hash_text(normalized_question),
                    normalized_question,
                    record.model_dump_json(),
                    json.dumps(sorted(dict.fromkeys(concept_ids))),
                    _iso(now),
                    _iso(expires_at),
                ),
            )
            rows = self._connection.execute(
                "SELECT question_hash FROM hot_answers WHERE scope_key = ? ORDER BY created_at DESC",
                (scope_key,),
            ).fetchall()
            for row in rows[max_entries:]:
                self._connection.execute(
                    "DELETE FROM hot_answers WHERE scope_key = ? AND question_hash = ?",
                    (scope_key, str(row["question_hash"])),
                )

    def invalidate_hot_answers(self, *, changed_concept_ids: set[str]) -> None:
        if not changed_concept_ids:
            return
        rows = self._connection.execute(
            "SELECT scope_key, question_hash, concept_ids_json FROM hot_answers"
        ).fetchall()
        with self._connection:
            for row in rows:
                concept_ids = set(json.loads(str(row["concept_ids_json"])))
                if concept_ids & changed_concept_ids:
                    self._connection.execute(
                        "DELETE FROM hot_answers WHERE scope_key = ? AND question_hash = ?",
                        (str(row["scope_key"]), str(row["question_hash"])),
                    )

    def insert_trace(
        self,
        *,
        principal: str,
        scope_key: str,
        question: str,
        repo_revision: str,
        result: DeepAnswerResult,
        max_traces: int,
        max_age_days: int,
    ) -> str:
        trace_id = result.record.trace_id or str(uuid4())
        with self._connection:
            self._connection.execute(
                """
                INSERT OR REPLACE INTO answer_traces(
                    trace_id, principal, scope_key, question_hash, question_excerpt,
                    repo_revision, answer_summary, steps_json, paths_json,
                    model_chain_json, usage_json, duration_ms, created_at
                ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    trace_id,
                    principal,
                    scope_key,
                    _hash_text(question),
                    question[:200],
                    repo_revision,
                    result.record.answer[:200],
                    json.dumps(
                        [{"action": step.action, "detail": step.detail} for step in result.steps]
                    ),
                    json.dumps([concept.path for concept in result.read_concepts]),
                    json.dumps(list(result.record.model_chain)),
                    json.dumps(result.usage, sort_keys=True),
                    result.duration_ms,
                    _iso(_now()),
                ),
            )
            cutoff = _iso(_now() - timedelta(days=max_age_days))
            self._connection.execute("DELETE FROM answer_traces WHERE created_at < ?", (cutoff,))
            self._prune_lru(
                "answer_traces", max_traces, key_column="trace_id", order_column="created_at"
            )
        return trace_id

    def _prune_lru(
        self,
        table: str,
        max_entries: int,
        *,
        key_column: str = "cache_key",
        order_column: str = "last_accessed_at",
    ) -> None:
        rows = self._connection.execute(
            f"SELECT {key_column} FROM {table} ORDER BY {order_column} DESC"
        ).fetchall()
        for row in rows[max_entries:]:
            self._connection.execute(
                f"DELETE FROM {table} WHERE {key_column} = ?",
                (str(row[key_column]),),
            )


def normalize_question(question: str) -> str:
    return " ".join(question.split()).strip().casefold()


def scope_fingerprint(*, principal: str, roles: Sequence[str], read_prefixes: Sequence[str]) -> str:
    material = json.dumps(
        {
            "principal": principal,
            "roles": list(sorted(dict.fromkeys(roles))),
            "read_prefixes": list(sorted(dict.fromkeys(read_prefixes))),
        },
        sort_keys=True,
    )
    return _hash_text(material)


def exact_cache_key(
    *,
    repo_revision: str,
    normalized_question: str,
    scope_key: str,
    answer_mode: str,
    model_policy_revision: str,
    prompt_version: str,
    tool_version: str,
) -> str:
    material = "\n".join(
        (
            repo_revision,
            normalized_question,
            scope_key,
            answer_mode,
            model_policy_revision,
            prompt_version,
            tool_version,
        )
    )
    return _hash_text(material)


def _hash_text(value: str) -> str:
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


def _iso(value: datetime) -> str:
    return value.astimezone(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def _now() -> datetime:
    return datetime.now(tz=timezone.utc)
