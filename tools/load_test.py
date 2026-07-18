from __future__ import annotations

import argparse
import concurrent.futures
import dataclasses
import http.client
import json
import os
import platform
import random
import shutil
import sqlite3
import subprocess
import sys
import tempfile
import threading
import time
import urllib.parse
from collections import Counter
from collections.abc import Callable, Iterable, Sequence
from contextlib import AbstractContextManager
from dataclasses import dataclass, field
from datetime import UTC, datetime
from pathlib import Path
from typing import Any, TypeVar, cast
from uuid import uuid4

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))

from memento.app import MementoRuntime, build_runtime, load_service_config
from memento.backup import create_backup, restore_backup
from memento.config import Principal
from memento.control.db import migrate_control_db
from memento.derived.index import DerivedIndex
from memento.repository.frontmatter import serialize_concept
from memento.repository.git import get_main_revision
from memento.repository.schema import ConceptDocument, ConceptFrontmatter, ConceptStatus
from memento.repository.transactions import TransactionManager
from memento.service import MemoryService, ServiceContext, ServiceDependencies

T = TypeVar("T")


@dataclass(frozen=True, slots=True)
class Threshold:
    name: str
    actual: float | int
    comparator: str
    expected: float | int
    passed: bool
    note: str

    def as_dict(self) -> dict[str, Any]:
        return dataclasses.asdict(self)


@dataclass(frozen=True, slots=True)
class LatencySummary:
    p50_ms: float
    p95_ms: float
    p99_ms: float
    max_ms: float

    def as_dict(self) -> dict[str, float]:
        return {
            "p50_ms": round(self.p50_ms, 3),
            "p95_ms": round(self.p95_ms, 3),
            "p99_ms": round(self.p99_ms, 3),
            "max_ms": round(self.max_ms, 3),
        }


@dataclass(frozen=True, slots=True)
class OperationRecord:
    operation: str
    latency_ms: float
    ok: bool
    error: str | None = None
    details: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True, slots=True)
class ScenarioResult:
    name: str
    started_at: str
    ended_at: str
    duration_seconds: float
    count: int
    ok_count: int
    error_count: int
    errors: dict[str, int]
    throughput_per_second: float
    latency: LatencySummary
    invariant_failures: tuple[str, ...]
    thresholds: tuple[Threshold, ...]
    passed: bool
    details: dict[str, Any] = field(default_factory=dict)

    def as_dict(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "started_at": self.started_at,
            "ended_at": self.ended_at,
            "duration_seconds": round(self.duration_seconds, 6),
            "count": self.count,
            "ok_count": self.ok_count,
            "error_count": self.error_count,
            "errors": self.errors,
            "throughput_per_second": round(self.throughput_per_second, 6),
            "latency": self.latency.as_dict(),
            "invariant_failures": list(self.invariant_failures),
            "thresholds": [item.as_dict() for item in self.thresholds],
            "passed": self.passed,
            "details": self.details,
        }


@dataclass(frozen=True, slots=True)
class HarnessConfig:
    profile: str
    concepts: int
    workers: int
    requests: int
    duration_seconds: float
    http_concurrency: int
    semantic_enabled: bool
    include_http: bool
    include_semantic: bool
    output: Path
    seed: int
    http_url: str | None = None
    http_token: str | None = None
    http_search_ratio: int = 50
    http_read_ratio: int = 30
    http_status_ratio: int = 20


@dataclass(frozen=True, slots=True)
class SeedConcept:
    path: str
    title: str
    query: str


@dataclass(slots=True)
class HarnessEnvironment(AbstractContextManager["HarnessEnvironment"]):
    root: Path
    config_path: Path
    runtime: MementoRuntime
    concepts: tuple[SeedConcept, ...]

    @property
    def repo_revision(self) -> str:
        return get_main_revision(self.runtime.paths.repo_paths)

    def make_service(self) -> MemoryService:
        connection = sqlite3.connect(self.runtime.paths.control_db, check_same_thread=False)
        connection.row_factory = sqlite3.Row
        connection.execute("PRAGMA journal_mode=WAL")
        connection.execute("PRAGMA foreign_keys=ON")
        migrate_control_db(connection)
        derived = DerivedIndex(self.runtime.paths.derived_db)

        def apply_update(
            materialized_root: Path, repo_revision: str, changed_paths: tuple[str, ...]
        ) -> None:
            if changed_paths:
                derived.update_paths(
                    materialized_root,
                    repo_revision=repo_revision,
                    changed_paths=changed_paths,
                )
            else:
                derived.rebuild(materialized_root, repo_revision=repo_revision)

        manager = TransactionManager(
            connection,
            self.runtime.paths.repo_paths,
            derived_update=apply_update,
        )
        return MemoryService(
            ServiceDependencies(
                config=self.runtime.config,
                repo_paths=self.runtime.paths.repo_paths,
                control_connection=connection,
                derived_index=derived,
                transaction_manager=manager,
                model_client=None,
            )
        )

    def context(self, principal_name: str) -> ServiceContext:
        policy = self.runtime.config.authorization.principals[principal_name]
        return ServiceContext(Principal(name=principal_name, roles=policy.roles))

    def close_service(self, service: MemoryService) -> None:
        service._deps.control_connection.close()

    def __exit__(self, exc_type: object, exc: object, tb: object) -> None:
        self.runtime.close()
        shutil.rmtree(self.root, ignore_errors=True)


class ThreadWorkerPool:
    def __init__(self, env: HarnessEnvironment, principal_name: str) -> None:
        self._env = env
        self._principal_name = principal_name
        self._local = threading.local()
        self._services: list[MemoryService] = []
        self._lock = threading.Lock()

    def service_and_context(self) -> tuple[MemoryService, ServiceContext]:
        service = cast(MemoryService | None, getattr(self._local, "service", None))
        context = cast(ServiceContext | None, getattr(self._local, "context", None))
        if service is None or context is None:
            service = self._env.make_service()
            context = self._env.context(self._principal_name)
            self._local.service = service
            self._local.context = context
            with self._lock:
                self._services.append(service)
        return service, context

    def close(self) -> None:
        for service in self._services:
            self._env.close_service(service)


class HttpJsonRpcClient:
    def __init__(self, base_url: str, token: str) -> None:
        parsed = urllib.parse.urlparse(base_url)
        if parsed.scheme not in {"http", "https"} or not parsed.netloc:
            raise ValueError("http_url must be an absolute http(s) URL")
        self._scheme = parsed.scheme
        self._host = parsed.netloc
        self._path = parsed.path or "/mcp"
        self._token = token
        self._lock = threading.Lock()
        self._counter = 0

    def call(self, method: str, params: dict[str, Any]) -> dict[str, Any]:
        request_id = self._next_id()
        body = json.dumps({"jsonrpc": "2.0", "id": request_id, "method": method, "params": params})
        connection_cls = (
            http.client.HTTPSConnection if self._scheme == "https" else http.client.HTTPConnection
        )
        connection = connection_cls(self._host, timeout=15)
        try:
            connection.request(
                "POST",
                self._path,
                body=body,
                headers={
                    "Authorization": f"Bearer {self._token}",
                    "Content-Type": "application/json",
                    "Accept": "application/json",
                    "MCP-Protocol-Version": "2025-03-26",
                },
            )
            response = connection.getresponse()
            payload = response.read().decode("utf-8")
            if response.status >= 400:
                raise RuntimeError(f"http {response.status}: {payload}")
            decoded = json.loads(payload)
            if "error" in decoded:
                raise RuntimeError(f"jsonrpc error: {decoded['error']}")
            return cast(dict[str, Any], decoded.get("result", {}))
        finally:
            connection.close()

    def initialize(self) -> None:
        self.call(
            "initialize",
            {
                "protocolVersion": "2025-03-26",
                "capabilities": {},
                "clientInfo": {"name": "memento-load-test", "version": "0.1"},
            },
        )

    def _next_id(self) -> int:
        with self._lock:
            self._counter += 1
            return self._counter


def percentile(values: Sequence[float], rank: float) -> float:
    if not values:
        return 0.0
    if len(values) == 1:
        return float(values[0])
    bounded_rank = min(max(rank, 0.0), 1.0)
    index = bounded_rank * (len(values) - 1)
    lower = int(index)
    upper = min(lower + 1, len(values) - 1)
    fraction = index - lower
    ordered = sorted(float(value) for value in values)
    return ordered[lower] + (ordered[upper] - ordered[lower]) * fraction


def summarize_latencies(latencies_ms: Sequence[float]) -> LatencySummary:
    values = list(latencies_ms)
    if not values:
        return LatencySummary(0.0, 0.0, 0.0, 0.0)
    ordered = sorted(values)
    return LatencySummary(
        p50_ms=percentile(ordered, 0.50),
        p95_ms=percentile(ordered, 0.95),
        p99_ms=percentile(ordered, 0.99),
        max_ms=max(ordered),
    )


def build_threshold(
    name: str, actual: float | int, comparator: str, expected: float | int, note: str
) -> Threshold:
    comparisons = {
        "<=": actual <= expected,
        ">=": actual >= expected,
        "==": actual == expected,
    }
    if comparator not in comparisons:
        raise ValueError(f"unsupported comparator: {comparator}")
    return Threshold(
        name=name,
        actual=actual,
        comparator=comparator,
        expected=expected,
        passed=bool(comparisons[comparator]),
        note=note,
    )


def compile_scenario(
    name: str,
    started_at: datetime,
    ended_at: datetime,
    records: Sequence[OperationRecord],
    *,
    invariant_failures: Iterable[str] = (),
    thresholds: Sequence[Threshold] = (),
    details: dict[str, Any] | None = None,
) -> ScenarioResult:
    latencies = [item.latency_ms for item in records]
    errors = Counter(item.error for item in records if item.error)
    failures = tuple(item for item in invariant_failures if item)
    passed = not failures and all(item.passed for item in thresholds)
    duration_seconds = max((ended_at - started_at).total_seconds(), 0.000001)
    return ScenarioResult(
        name=name,
        started_at=started_at.isoformat(),
        ended_at=ended_at.isoformat(),
        duration_seconds=duration_seconds,
        count=len(records),
        ok_count=sum(1 for item in records if item.ok),
        error_count=sum(1 for item in records if not item.ok),
        errors={str(key): int(value) for key, value in sorted(errors.items())},
        throughput_per_second=len(records) / duration_seconds,
        latency=summarize_latencies(latencies),
        invariant_failures=failures,
        thresholds=tuple(thresholds),
        passed=passed,
        details=details or {},
    )


def _now() -> datetime:
    return datetime.now(tz=UTC)


def _git_revision(path: Path) -> str:
    try:
        return subprocess.run(
            ["git", "-C", str(path), "rev-parse", "HEAD"],
            capture_output=True,
            check=True,
            text=True,
        ).stdout.strip()
    except Exception:
        return "unknown"


def _write_concept(
    path: Path, *, concept_id: str, concept_type: str, title: str, body: str
) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    now = datetime.now(tz=UTC).replace(microsecond=0)
    document = ConceptDocument(
        frontmatter=ConceptFrontmatter(
            schema_version=1,
            id=concept_id,
            type=concept_type,
            title=title,
            status=ConceptStatus.ACTIVE,
            description=f"Load concept {title}",
            tags=("load", concept_type),
            aliases=(),
            source_refs=(),
            supersedes=(),
            created_at=now,
            updated_at=now,
            updated_by="load-test",
        ),
        body=body,
    )
    path.write_text(serialize_concept(document), encoding="utf-8")


def build_harness_environment(
    *, concepts: int, seed: int, semantic_enabled: bool = False
) -> HarnessEnvironment:
    temp_root = Path(tempfile.mkdtemp(prefix="memento-load-"))
    state_root = temp_root / "state"
    seed_root = temp_root / "seed"
    rng = random.Random(seed)
    catalog: list[SeedConcept] = []
    instance_count = max(1, concepts // 2)
    project_count = max(1, concepts - instance_count)
    for index in range(instance_count):
        title = f"Instance {index:03d}"
        project_ref = f"/projects/project-{index % project_count:03d}.md"
        path = f"/instances/instance-{index:03d}.md"
        _write_concept(
            seed_root / path.lstrip("/"),
            concept_id=f"instance-{index:03d}-id",
            concept_type="instance",
            title=title,
            body=f"# {title}\n\nLinks to [Project]({project_ref}).\n",
        )
        catalog.append(SeedConcept(path=path, title=title, query=f"Instance {index:03d}"))
    for index in range(project_count):
        title = f"Project {index:03d}"
        instance_ref = f"/instances/instance-{index % instance_count:03d}.md"
        path = f"/projects/project-{index:03d}.md"
        noise = rng.randint(1000, 9999)
        _write_concept(
            seed_root / path.lstrip("/"),
            concept_id=f"project-{index:03d}-id",
            concept_type="project",
            title=title,
            body=f"# {title}\n\nThis is concept {noise}. See [Instance]({instance_ref}).\n",
        )
        catalog.append(SeedConcept(path=path, title=title, query=f"Project {index:03d}"))
    semantic_config: dict[str, Any] = {"enabled": False}
    if semantic_enabled:
        ffi_library = ROOT / "rust/target/release/libmemento_ffi.so"
        sqlite_extension = ROOT / "rust/target/release/libmemento_sqlite_vector.so"
        model_path = ROOT / "models/gte/gte-small.gtemodel"
        if not ffi_library.exists() or not sqlite_extension.exists():
            subprocess.run(
                [
                    "cargo",
                    "build",
                    "--release",
                    "-p",
                    "memento-ffi",
                    "-p",
                    "memento-sqlite-vector",
                ],
                cwd=ROOT / "rust",
                check=True,
            )
        semantic_config = {
            "enabled": True,
            "ffi_library_path": str(ffi_library),
            "sqlite_extension_path": str(sqlite_extension),
            "model_path": str(model_path),
            "model_id": "gte-small-fp32",
            "dimensions": 384,
            "max_batch_size": 16,
            "max_candidates": 200,
            "default_search_mode": "lexical",
        }
    config_path = temp_root / "config.json"
    config_path.write_text(
        json.dumps(
            {
                "schema_version": 2,
                "repository": {"root_path": str(state_root), "bundle_root": "/"},
                "intelligent_tiers": {"semantic_search": semantic_config},
                "authorization": {
                    "principals": {
                        "smith": {
                            "roles": ["reader", "proposer", "curator"],
                            "token_env": "MEMENTO_TOKEN_SMITH",
                            "read_prefixes": ["/instances/", "/projects/"],
                            "write_prefixes": ["/instances/", "/projects/"],
                        },
                        "flint": {
                            "roles": ["reader", "proposer"],
                            "token_env": "MEMENTO_TOKEN_FLINT",
                            "read_prefixes": ["/instances/", "/projects/"],
                            "write_prefixes": ["/projects/"],
                        },
                    }
                },
            },
            indent=2,
            sort_keys=True,
        )
        + "\n",
        encoding="utf-8",
    )
    os.environ.setdefault("MEMENTO_TOKEN_SMITH", "smith-load-token")
    os.environ.setdefault("MEMENTO_TOKEN_FLINT", "flint-load-token")
    runtime = build_runtime(config_path, bootstrap_seed=seed_root)
    return HarnessEnvironment(
        root=temp_root, config_path=config_path, runtime=runtime, concepts=tuple(catalog)
    )


def _run_parallel(
    count: int, workers: int, fn: Callable[[int], OperationRecord]
) -> list[OperationRecord]:
    with concurrent.futures.ThreadPoolExecutor(max_workers=max(1, workers)) as executor:
        return list(executor.map(fn, range(count)))


def run_direct_load(
    env: HarnessEnvironment,
    *,
    workers: int,
    requests: int,
    search_ratio: int = 50,
) -> ScenarioResult:
    started_at = _now()
    pool = ThreadWorkerPool(env, "smith")
    concepts = list(env.concepts)
    total_ratio = max(search_ratio, 0) + max(100 - search_ratio, 0)

    def one(index: int) -> OperationRecord:
        service, context = pool.service_and_context()
        concept = concepts[index % len(concepts)]
        started = time.perf_counter()
        try:
            if (index % total_ratio) < search_ratio:
                payload = service.memory_search(context, query=concept.query, limit=5)
                ok = payload.status == "success" and bool(
                    cast(dict[str, Any], payload.data)["results"]
                )
                error = None if ok else getattr(payload, "error_class", "search_failed")
                return OperationRecord(
                    "search", (time.perf_counter() - started) * 1000.0, ok, error
                )
            payload = service.memory_read(context, id_or_path=concept.path)
            ok = (
                payload.status == "success"
                and cast(dict[str, Any], payload.data)["path"] == concept.path
            )
            error = None if ok else getattr(payload, "error_class", "read_failed")
            return OperationRecord("read", (time.perf_counter() - started) * 1000.0, ok, error)
        except Exception as exc:
            return OperationRecord(
                "direct", (time.perf_counter() - started) * 1000.0, False, type(exc).__name__
            )

    try:
        records = _run_parallel(requests, workers, one)
    finally:
        pool.close()
    ended_at = _now()
    thresholds = (
        build_threshold(
            "error_count",
            sum(1 for item in records if not item.ok),
            "==",
            0,
            "Local direct read/search check.",
        ),
        build_threshold(
            "p99_ms",
            summarize_latencies([item.latency_ms for item in records]).p99_ms,
            "<=",
            2500.0,
            "Local threshold, not a universal SLO.",
        ),
    )
    return compile_scenario(
        "direct_functional_load",
        started_at,
        ended_at,
        records,
        thresholds=thresholds,
        details={"workers": workers, "requests": requests, "concepts": len(concepts)},
    )


def run_semantic_load(env: HarnessEnvironment, *, workers: int, requests: int) -> ScenarioResult:
    started_at = _now()
    pool = ThreadWorkerPool(env, "smith")
    concepts = list(env.concepts)

    def one(index: int) -> OperationRecord:
        service, context = pool.service_and_context()
        concept = concepts[index % len(concepts)]
        started = time.perf_counter()
        try:
            payload = service.memory_search(
                context,
                query=concept.query,
                limit=5,
                search_mode="semantic",
            )
            ok = payload.status == "success"
            warning_count = len(getattr(payload, "warnings", ()))
            return OperationRecord(
                "semantic_search",
                (time.perf_counter() - started) * 1000.0,
                ok,
                None if ok else getattr(payload, "error_class", "semantic_failed"),
                details={"warnings": warning_count},
            )
        except Exception as exc:
            return OperationRecord(
                "semantic_search",
                (time.perf_counter() - started) * 1000.0,
                False,
                type(exc).__name__,
            )

    try:
        records = _run_parallel(requests, workers, one)
    finally:
        pool.close()
    ended_at = _now()
    status = env.runtime.derived_index.semantic_status()
    thresholds = (
        build_threshold(
            "error_count",
            sum(1 for item in records if not item.ok),
            "==",
            0,
            "Scenario should degrade gracefully even without vendored semantic runtime.",
        ),
    )
    return compile_scenario(
        "semantic_query_load",
        started_at,
        ended_at,
        records,
        thresholds=thresholds,
        details={
            "workers": workers,
            "requests": requests,
            "semantic_enabled": status.enabled,
            "semantic_ready": status.ready,
            "warnings": list(status.warnings),
        },
    )


def run_write_contention(env: HarnessEnvironment, *, workers: int) -> ScenarioResult:
    started_at = _now()
    base_revision = env.repo_revision
    target_path = "/projects/project-000.md"
    pool = ThreadWorkerPool(env, "smith")
    winner_markers = [f"contention-worker-{index}" for index in range(workers)]

    def one(index: int) -> OperationRecord:
        service, context = pool.service_and_context()
        marker = winner_markers[index]
        started = time.perf_counter()
        payload = service.memory_patch(
            context,
            path=target_path,
            expected_revision=base_revision,
            idempotency_key=f"contention-{index}-{uuid4()}",
            body=f"# Project 000\n\nWinner {marker}.\n",
        )
        ok = payload.status == "success"
        error = None if ok else getattr(payload, "error_class", "write_failed")
        return OperationRecord("patch", (time.perf_counter() - started) * 1000.0, ok, error)

    try:
        records = _run_parallel(workers, workers, one)
        check_service = env.make_service()
        read_payload = check_service.memory_read(env.context("smith"), id_or_path=target_path)
    finally:
        pool.close()
        if "check_service" in locals():
            env.close_service(check_service)
    ended_at = _now()
    invariant_failures: list[str] = []
    success_count = sum(1 for item in records if item.ok)
    conflict_count = sum(1 for item in records if item.error == "conflict")
    if success_count != 1:
        invariant_failures.append(f"expected exactly one successful write, saw {success_count}")
    if conflict_count != workers - 1:
        invariant_failures.append(
            f"expected {workers - 1} conflicts after single winner, saw {conflict_count}"
        )
    body = ""
    if read_payload.status == "success":
        body = cast(dict[str, Any], read_payload.data)["body"]
        body_lines = {
            line.strip().removeprefix("Winner ").removesuffix(".") for line in body.splitlines()
        }
        winners_in_body = [marker for marker in winner_markers if marker in body_lines]
        if len(winners_in_body) != 1:
            invariant_failures.append(
                f"expected exactly one winner marker in final body, saw {winners_in_body}"
            )
    else:
        invariant_failures.append("unable to read contested concept after writes")
    thresholds = (
        build_threshold(
            "success_count", success_count, "==", 1, "Single compare-and-swap winner expected."
        ),
        build_threshold(
            "conflict_count",
            conflict_count,
            "==",
            workers - 1,
            "Remaining writes should conflict from same base.",
        ),
    )
    return compile_scenario(
        "write_contention",
        started_at,
        ended_at,
        records,
        invariant_failures=invariant_failures,
        thresholds=thresholds,
        details={
            "workers": workers,
            "target_path": target_path,
            "base_revision": base_revision,
            "final_body": body,
        },
    )


def run_idempotent_replay_storm(env: HarnessEnvironment, *, workers: int) -> ScenarioResult:
    started_at = _now()
    base_revision = env.repo_revision
    target_path = "/projects/project-001.md"
    idempotency_key = "replay-storm-key"
    pool = ThreadWorkerPool(env, "smith")

    def one(index: int) -> OperationRecord:
        del index
        service, context = pool.service_and_context()
        started = time.perf_counter()
        details: dict[str, Any] = {}
        for attempt in range(8):
            try:
                payload = service.memory_patch(
                    context,
                    path=target_path,
                    expected_revision=base_revision,
                    idempotency_key=idempotency_key,
                    body="# Project 001\n\nReplay storm body.\n",
                )
                if payload.status == "success":
                    details["operation_id"] = payload.operation_id
                    details["replayed"] = cast(dict[str, Any], payload.data)["replayed"]
                    return OperationRecord(
                        "idempotent_patch",
                        (time.perf_counter() - started) * 1000.0,
                        True,
                        None,
                        details=details,
                    )
                details["last_error_class"] = getattr(payload, "error_class", "replay_failed")
                if attempt < 7 and details["last_error_class"] in {
                    "not_found",
                    "conflict",
                    "validation_error",
                }:
                    time.sleep(0.01)
                    continue
                return OperationRecord(
                    "idempotent_patch",
                    (time.perf_counter() - started) * 1000.0,
                    False,
                    cast(str, details["last_error_class"]),
                    details=details,
                )
            except sqlite3.IntegrityError:
                details["retried_after_integrity_error"] = True
                if attempt == 7:
                    return OperationRecord(
                        "idempotent_patch",
                        (time.perf_counter() - started) * 1000.0,
                        False,
                        "sqlite_integrity_error",
                        details=details,
                    )
                time.sleep(0.01)
        return OperationRecord(
            "idempotent_patch",
            (time.perf_counter() - started) * 1000.0,
            False,
            "retry_exhausted",
            details=details,
        )

    try:
        records = _run_parallel(workers, workers, one)
    finally:
        pool.close()
    ended_at = _now()
    operation_ids = {item.details.get("operation_id") for item in records if item.ok}
    replay_flags = [bool(item.details.get("replayed")) for item in records if item.ok]
    invariant_failures: list[str] = []
    if len(operation_ids) != 1:
        invariant_failures.append(
            f"expected one operation id, saw {sorted(str(item) for item in operation_ids)}"
        )
    if replay_flags.count(False) != 1:
        invariant_failures.append(
            f"expected one non-replayed result, saw {replay_flags.count(False)}"
        )
    if any(not item.ok for item in records):
        invariant_failures.append(
            "all replay-storm callers should eventually observe the same successful result"
        )
    thresholds = (
        build_threshold(
            "distinct_operation_ids",
            len(operation_ids),
            "==",
            1,
            "Idempotent storm should collapse to one recorded operation.",
        ),
        build_threshold(
            "initial_results",
            replay_flags.count(False),
            "==",
            1,
            "Exactly one initial execution expected.",
        ),
        build_threshold(
            "error_count",
            sum(1 for item in records if not item.ok),
            "==",
            0,
            "Every caller should converge on the recorded result after retries.",
        ),
    )
    return compile_scenario(
        "idempotent_replay_storm",
        started_at,
        ended_at,
        records,
        invariant_failures=invariant_failures,
        thresholds=thresholds,
        details={
            "workers": workers,
            "idempotency_key": idempotency_key,
            "target_path": target_path,
        },
    )


def run_proposal_concurrency(env: HarnessEnvironment, *, workers: int) -> ScenarioResult:
    started_at = _now()
    base_revision = env.repo_revision
    pool = ThreadWorkerPool(env, "flint")
    smith_service = env.make_service()
    smith_context = env.context("smith")

    def one(index: int) -> OperationRecord:
        service, context = pool.service_and_context()
        started = time.perf_counter()
        proposal = service.memory_propose(
            context,
            intent=f"Load proposal {index}",
            base_revision=base_revision,
            changes=[
                {
                    "kind": "patch",
                    "path": "/projects/project-000.md",
                    "description": f"Proposal {index}",
                }
            ],
            rationale="load test",
        )
        if proposal.status != "success":
            return OperationRecord(
                "proposal_create",
                (time.perf_counter() - started) * 1000.0,
                False,
                getattr(proposal, "error_class", "proposal_create_failed"),
            )
        proposal_id = cast(dict[str, Any], proposal.data)["proposal"]["proposal_id"]
        listed = service.memory_proposal_list(context)
        fetched = service.memory_proposal_get(context, proposal_id=proposal_id)
        ok = listed.status == "success" and fetched.status == "success"
        error = None if ok else "proposal_readback_failed"
        return OperationRecord(
            "proposal_triplet",
            (time.perf_counter() - started) * 1000.0,
            ok,
            error,
            details={"proposal_id": proposal_id},
        )

    try:
        records = _run_parallel(workers, workers, one)
        curator_list = smith_service.memory_proposal_list(smith_context)
    finally:
        pool.close()
        env.close_service(smith_service)
    ended_at = _now()
    created_ids = [cast(str, item.details["proposal_id"]) for item in records if item.ok]
    invariant_failures: list[str] = []
    if len(set(created_ids)) != len(created_ids):
        invariant_failures.append("proposal ids were not unique")
    visible_count = -1
    if curator_list.status == "success":
        visible_count = len(cast(dict[str, Any], curator_list.data)["proposals"])
        if visible_count < len(created_ids):
            invariant_failures.append(
                f"curator saw only {visible_count} proposals after creating {len(created_ids)}"
            )
    else:
        invariant_failures.append("curator could not list proposals")
    thresholds = (
        build_threshold(
            "error_count",
            sum(1 for item in records if not item.ok),
            "==",
            0,
            "Create/list/read triplets should complete locally.",
        ),
    )
    return compile_scenario(
        "proposal_concurrency",
        started_at,
        ended_at,
        records,
        invariant_failures=invariant_failures,
        thresholds=thresholds,
        details={
            "workers": workers,
            "created_proposals": len(created_ids),
            "curator_visible": visible_count,
        },
    )


def run_backup_restore_drill(env: HarnessEnvironment) -> ScenarioResult:
    started_at = _now()
    started = time.perf_counter()
    records: list[OperationRecord] = []
    backup_dir = env.root / "backup-output"
    manifest = create_backup(env.runtime, backup_dir)
    records.append(OperationRecord("backup", (time.perf_counter() - started) * 1000.0, True))
    env.runtime.close()
    restore_started = time.perf_counter()
    restored = restore_backup(load_service_config(env.config_path), backup_dir)
    records.append(
        OperationRecord("restore", (time.perf_counter() - restore_started) * 1000.0, True)
    )
    env.runtime = build_runtime(env.config_path)
    ended_at = _now()
    invariant_failures: list[str] = []
    if restored["repo_revision"] != manifest.repo_revision:
        invariant_failures.append("restored revision did not match backup manifest")
    if env.repo_revision != manifest.repo_revision:
        invariant_failures.append("runtime repo revision changed after restore drill")
    thresholds = (
        build_threshold(
            "error_count",
            0,
            "==",
            0,
            "Local backup/restore drill should complete without explicit errors.",
        ),
        build_threshold(
            "duration_ms",
            sum(item.latency_ms for item in records),
            "<=",
            30000.0,
            "Local threshold, not a universal SLO.",
        ),
    )
    return compile_scenario(
        "backup_restore_drill",
        started_at,
        ended_at,
        records,
        invariant_failures=invariant_failures,
        thresholds=thresholds,
        details={"backup_dir": str(backup_dir), "manifest": manifest.as_dict()},
    )


def run_http_scenario(
    *,
    base_url: str,
    token: str,
    concurrency: int,
    duration_seconds: float,
    search_ratio: int,
    read_ratio: int,
    status_ratio: int,
    sample_paths: Sequence[str],
    sample_queries: Sequence[str],
) -> ScenarioResult:
    started_at = _now()
    stop_at = time.perf_counter() + duration_seconds
    records_lock = threading.Lock()
    records: list[OperationRecord] = []
    paths = list(sample_paths)
    queries = list(sample_queries)

    def worker(worker_id: int) -> None:
        client = HttpJsonRpcClient(base_url, token)
        client.initialize()
        total = search_ratio + read_ratio + status_ratio
        counter = 0
        while time.perf_counter() < stop_at:
            choice = (worker_id + counter) % max(total, 1)
            started = time.perf_counter()
            try:
                if choice < status_ratio:
                    result = client.call("tools/call", {"name": "memory_status", "arguments": {}})
                    ok = "content" in result or "data" in result
                    record = OperationRecord(
                        "http_status",
                        (time.perf_counter() - started) * 1000.0,
                        ok,
                        None if ok else "unexpected_status_response",
                    )
                elif choice < status_ratio + search_ratio:
                    query = queries[counter % len(queries)]
                    result = client.call(
                        "tools/call",
                        {"name": "memory_search", "arguments": {"query": query, "limit": 5}},
                    )
                    ok = "content" in result or "data" in result
                    record = OperationRecord(
                        "http_search",
                        (time.perf_counter() - started) * 1000.0,
                        ok,
                        None if ok else "unexpected_search_response",
                    )
                else:
                    path = paths[counter % len(paths)]
                    result = client.call(
                        "tools/call", {"name": "memory_read", "arguments": {"id_or_path": path}}
                    )
                    ok = "content" in result or "data" in result
                    record = OperationRecord(
                        "http_read",
                        (time.perf_counter() - started) * 1000.0,
                        ok,
                        None if ok else "unexpected_read_response",
                    )
            except Exception as exc:
                record = OperationRecord(
                    "http", (time.perf_counter() - started) * 1000.0, False, type(exc).__name__
                )
            with records_lock:
                records.append(record)
            counter += 1

    with concurrent.futures.ThreadPoolExecutor(max_workers=max(1, concurrency)) as executor:
        futures = [executor.submit(worker, index) for index in range(concurrency)]
        for future in futures:
            future.result()
    ended_at = _now()
    thresholds = (
        build_threshold(
            "error_count",
            sum(1 for item in records if not item.ok),
            "==",
            0,
            "Local authenticated HTTP smoke/load threshold.",
        ),
    )
    return compile_scenario(
        "http_streamable_jsonrpc",
        started_at,
        ended_at,
        records,
        thresholds=thresholds,
        details={
            "base_url": base_url,
            "concurrency": concurrency,
            "duration_seconds": duration_seconds,
            "mix": {"status": status_ratio, "search": search_ratio, "read": read_ratio},
        },
    )


def build_report(config: HarnessConfig, scenarios: Sequence[ScenarioResult]) -> dict[str, Any]:
    return {
        "generated_at": _now().isoformat(),
        "environment": {
            "python": sys.version,
            "platform": platform.platform(),
            "cwd": str(ROOT),
            "seed": config.seed,
            "profile": config.profile,
        },
        "git_revision": _git_revision(ROOT),
        "scenario_count": len(scenarios),
        "passed": all(item.passed for item in scenarios),
        "scenarios": [item.as_dict() for item in scenarios],
        "notes": [
            "Thresholds in this report are local development checks, not universal service SLOs.",
            "All repository activity uses temporary directories; canonical fixtures are not modified.",
        ],
    }


def parse_args(argv: Sequence[str] | None = None) -> HarnessConfig:
    parser = argparse.ArgumentParser(description="Repository-owned Memento load testing harness")
    parser.add_argument(
        "--profile", choices=("functional", "operational", "check"), default="check"
    )
    parser.add_argument("--concepts", type=int, default=12)
    parser.add_argument("--workers", type=int, default=4)
    parser.add_argument("--requests", type=int, default=24)
    parser.add_argument("--duration-seconds", type=float, default=3.0)
    parser.add_argument("--http-concurrency", type=int, default=4)
    parser.add_argument("--output", type=Path, default=ROOT / "build" / "load-report.json")
    parser.add_argument("--seed", type=int, default=7)
    parser.add_argument("--semantic-enabled", action="store_true")
    parser.add_argument("--include-semantic", action="store_true")
    parser.add_argument("--http-url")
    parser.add_argument("--http-token")
    parser.add_argument("--include-http", action="store_true")
    parser.add_argument("--http-search-ratio", type=int, default=50)
    parser.add_argument("--http-read-ratio", type=int, default=30)
    parser.add_argument("--http-status-ratio", type=int, default=20)
    args = parser.parse_args(argv)
    return HarnessConfig(
        profile=args.profile,
        concepts=args.concepts,
        workers=args.workers,
        requests=args.requests,
        duration_seconds=args.duration_seconds,
        http_concurrency=args.http_concurrency,
        semantic_enabled=bool(args.semantic_enabled),
        include_http=bool(args.include_http),
        include_semantic=bool(args.include_semantic),
        output=args.output,
        seed=args.seed,
        http_url=args.http_url,
        http_token=args.http_token,
        http_search_ratio=args.http_search_ratio,
        http_read_ratio=args.http_read_ratio,
        http_status_ratio=args.http_status_ratio,
    )


def run_profile(config: HarnessConfig) -> dict[str, Any]:
    scenarios: list[ScenarioResult] = []
    with build_harness_environment(
        concepts=config.concepts,
        seed=config.seed,
        semantic_enabled=config.semantic_enabled,
    ) as env:
        scenarios.append(run_direct_load(env, workers=config.workers, requests=config.requests))
        if config.include_semantic:
            scenarios.append(
                run_semantic_load(env, workers=config.workers, requests=config.requests)
            )
        if config.profile in {"operational", "check"}:
            scenarios.append(run_write_contention(env, workers=config.workers))
            scenarios.append(run_idempotent_replay_storm(env, workers=config.workers))
            scenarios.append(run_proposal_concurrency(env, workers=config.workers))
            scenarios.append(run_backup_restore_drill(env))
        if config.include_http:
            if not config.http_url or not config.http_token:
                skipped = compile_scenario(
                    "http_streamable_jsonrpc",
                    _now(),
                    _now(),
                    [],
                    invariant_failures=(
                        "http_url and http_token are required when --include-http is used",
                    ),
                    thresholds=(),
                )
                scenarios.append(skipped)
            else:
                scenarios.append(
                    run_http_scenario(
                        base_url=config.http_url,
                        token=config.http_token,
                        concurrency=config.http_concurrency,
                        duration_seconds=config.duration_seconds,
                        search_ratio=config.http_search_ratio,
                        read_ratio=config.http_read_ratio,
                        status_ratio=config.http_status_ratio,
                        sample_paths=[item.path for item in env.concepts],
                        sample_queries=[item.query for item in env.concepts],
                    )
                )
    report = build_report(config, scenarios)
    config.output.parent.mkdir(parents=True, exist_ok=True)
    config.output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return report


def main(argv: Sequence[str] | None = None) -> int:
    config = parse_args(argv)
    report = run_profile(config)
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0 if bool(report["passed"]) else 1


if __name__ == "__main__":
    raise SystemExit(main())
