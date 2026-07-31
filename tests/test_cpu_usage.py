from __future__ import annotations

from pathlib import Path

from memento.cpu_usage import CpuUsageSampler


def write_stat(path: Path, values: tuple[int, ...]) -> None:
    path.write_text("cpu  " + " ".join(str(value) for value in values) + "\n")


def test_cpu_sampler_reports_busy_percent_over_window(tmp_path: Path) -> None:
    stat = tmp_path / "stat"
    now = [0.0]
    write_stat(stat, (100, 0, 100, 800, 0, 0, 0, 0))
    sampler = CpuUsageSampler(window_seconds=15, stat_path=stat, monotonic=lambda: now[0])
    assert sampler.sample() is None
    now[0] = 10
    write_stat(stat, (120, 0, 120, 860, 0, 0, 0, 0))
    assert sampler.sample() is None
    now[0] = 15
    write_stat(stat, (130, 0, 130, 900, 0, 0, 0, 0))
    # Delta: 60 busy ticks, 160 total ticks.
    assert sampler.sample() == 37.5


def test_cpu_sampler_treats_iowait_as_idle(tmp_path: Path) -> None:
    stat = tmp_path / "stat"
    now = [0.0]
    write_stat(stat, (100, 0, 100, 800, 100, 0, 0, 0))
    sampler = CpuUsageSampler(window_seconds=1, stat_path=stat, monotonic=lambda: now[0])
    assert sampler.sample() is None
    now[0] = 1
    write_stat(stat, (110, 0, 110, 800, 180, 0, 0, 0))
    # 20 busy, 100 total; 80 iowait ticks do not count as busy.
    assert sampler.sample() == 20.0


def test_cpu_sampler_recovers_from_counter_reset(tmp_path: Path) -> None:
    stat = tmp_path / "stat"
    now = [0.0]
    write_stat(stat, (100, 0, 100, 800, 0, 0, 0, 0))
    sampler = CpuUsageSampler(window_seconds=1, stat_path=stat, monotonic=lambda: now[0])
    assert sampler.sample() is None
    now[0] = 1
    write_stat(stat, (1, 0, 1, 8, 0, 0, 0, 0))
    assert sampler.sample() is None
