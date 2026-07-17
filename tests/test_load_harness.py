from __future__ import annotations

import importlib.util
import json
import sys
from datetime import UTC, datetime
from pathlib import Path

_LOAD_TEST_PATH = Path(__file__).parents[1] / "tools" / "load_test.py"
_SPEC = importlib.util.spec_from_file_location("memento_load_test", _LOAD_TEST_PATH)
assert _SPEC is not None and _SPEC.loader is not None
_LOAD_TEST = importlib.util.module_from_spec(_SPEC)
sys.modules[_SPEC.name] = _LOAD_TEST
_SPEC.loader.exec_module(_LOAD_TEST)

HarnessConfig = _LOAD_TEST.HarnessConfig
build_report = _LOAD_TEST.build_report
build_threshold = _LOAD_TEST.build_threshold
compile_scenario = _LOAD_TEST.compile_scenario
make_env = _LOAD_TEST.build_harness_environment
percentile = _LOAD_TEST.percentile
run_backup_restore_drill = _LOAD_TEST.run_backup_restore_drill
run_direct_load = _LOAD_TEST.run_direct_load
run_idempotent_replay_storm = _LOAD_TEST.run_idempotent_replay_storm
run_proposal_concurrency = _LOAD_TEST.run_proposal_concurrency
run_write_contention = _LOAD_TEST.run_write_contention
summarize_latencies = _LOAD_TEST.summarize_latencies


def test_percentile_and_latency_summary_are_stable() -> None:
    values = [10.0, 20.0, 30.0, 40.0, 50.0]
    assert percentile(values, 0.0) == 10.0
    assert percentile(values, 0.5) == 30.0
    assert percentile(values, 1.0) == 50.0
    summary = summarize_latencies(values)
    assert summary.p50_ms == 30.0
    assert summary.p95_ms >= 40.0
    assert summary.p99_ms >= summary.p95_ms
    assert summary.max_ms == 50.0


def test_compile_scenario_tracks_thresholds_and_failures() -> None:
    started = ended = datetime.now(UTC)
    scenario = compile_scenario(
        "unit",
        started,
        ended,
        records=[],
        invariant_failures=("boom",),
        thresholds=(build_threshold("errors", 1, "==", 0, "test"),),
    )
    assert scenario.passed is False
    assert scenario.invariant_failures == ("boom",)
    assert scenario.thresholds[0].passed is False
    assert scenario.latency.max_ms == 0.0


def test_direct_load_report_contains_metrics_and_passes(tmp_path: Path) -> None:
    del tmp_path
    with make_env(concepts=8, seed=11) as env:
        scenario = run_direct_load(env, workers=2, requests=8)
    assert scenario.name == "direct_functional_load"
    assert scenario.count == 8
    assert scenario.error_count == 0
    assert scenario.ok_count == 8
    assert scenario.latency.p50_ms >= 0.0
    assert scenario.latency.p99_ms >= scenario.latency.p95_ms
    assert scenario.passed is True


def test_operational_scenarios_enforce_invariants() -> None:
    with make_env(concepts=8, seed=3) as env:
        write_result = run_write_contention(env, workers=4)
        replay_result = run_idempotent_replay_storm(env, workers=4)
        proposal_result = run_proposal_concurrency(env, workers=4)
        backup_result = run_backup_restore_drill(env)
    assert write_result.passed is True
    assert replay_result.passed is True
    assert proposal_result.passed is True
    assert backup_result.passed is True
    assert write_result.details["target_path"].startswith("/projects/")
    assert replay_result.details["idempotency_key"] == "replay-storm-key"
    assert proposal_result.details["created_proposals"] == 4
    assert backup_result.details["manifest"]["repo_revision"]


def test_build_report_serializes_expected_shape(tmp_path: Path) -> None:
    scenario = compile_scenario(
        "shape",
        datetime.now(UTC),
        datetime.now(UTC),
        records=[],
        thresholds=(build_threshold("errors", 0, "==", 0, "ok"),),
    )
    config = HarnessConfig(
        profile="check",
        concepts=4,
        workers=2,
        requests=4,
        duration_seconds=1.0,
        http_concurrency=2,
        semantic_enabled=False,
        include_http=False,
        include_semantic=False,
        output=tmp_path / "report.json",
        seed=1,
    )
    report = build_report(config, [scenario])
    assert report["passed"] is True
    assert report["scenario_count"] == 1
    assert report["git_revision"]
    encoded = json.dumps(report)
    assert "local development checks" in encoded
