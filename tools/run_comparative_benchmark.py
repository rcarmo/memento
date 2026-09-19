#!/usr/bin/env python3
import hashlib
import json
import os
import platform
import statistics
import subprocess
import sys
import tempfile
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
count = int(sys.argv[1]) if len(sys.argv) > 1 else 1000
rebuilds = 5
searches = 100


def summary(values):
    values = sorted(v / 1e6 for v in values)
    return {
        "n": len(values),
        "min_ms": round(values[0], 3),
        "p50_ms": round(statistics.median(values), 3),
        "p95_ms": round(values[min(len(values) - 1, int(len(values) * 0.95))], 3),
        "max_ms": round(values[-1], 3),
        "mean_ms": round(statistics.mean(values), 3),
    }


with tempfile.TemporaryDirectory() as temp:
    corpus = Path(temp) / "corpus"
    corpus.mkdir()
    digest = hashlib.sha256()
    for i in range(count):
        nxt = (i + 1) % count
        text = f"---\nid: 'concept-{i:06d}'\ntype: concept\ntitle: Shared benchmark concept {i}\nstatus: active\ntags: [benchmark, group-{i % 10}]\naliases: []\nsource_refs: []\nsupersedes: []\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\nupdated_by: benchmark\n---\nShared benchmark body {i} links to [[/concept-{nxt:06d}.md]].\n"
        raw = text.encode()
        digest.update(raw)
        (corpus / f"concept-{i:06d}.md").write_bytes(raw)
    env = os.environ | {"PYTHONPATH": str(ROOT / "src")}
    go = json.loads(
        subprocess.check_output(
            [
                str(ROOT / "build/go/memento-benchmark-go"),
                str(corpus),
                str(rebuilds),
                str(searches),
            ],
            env=env,
        )
    )
    py = json.loads(
        subprocess.check_output(
            [
                sys.executable,
                str(ROOT / "tools/benchmark_python.py"),
                str(corpus),
                str(rebuilds),
                str(searches),
            ],
            env=env,
        )
    )
    report = {
        "date": time.strftime("%Y-%m-%d"),
        "host": {
            "platform": platform.platform(),
            "machine": platform.machine(),
            "python": platform.python_version(),
            "go": subprocess.check_output(["go", "version"], text=True).strip(),
        },
        "dataset": {
            "concepts": count,
            "sha256": digest.hexdigest(),
            "query": "shared benchmark",
            "rebuilds": rebuilds,
            "searches": searches,
            "warmups": 10,
        },
        "results": {
            x["runtime"]: {
                "rebuild": summary(x["rebuild_ns"]),
                "lexical_search": summary(x["search_ns"]),
                "result_count": x["result_count"],
            }
            for x in (py, go)
        },
    }
    report["relative_go_over_python"] = {
        "rebuild_p50": round(
            report["results"]["go"]["rebuild"]["p50_ms"]
            / report["results"]["python"]["rebuild"]["p50_ms"],
            3,
        ),
        "lexical_search_p50": round(
            report["results"]["go"]["lexical_search"]["p50_ms"]
            / report["results"]["python"]["lexical_search"]["p50_ms"],
            3,
        ),
    }
    out = (
        ROOT / "docs/evidence" / f"go-python-models-off-benchmark-{time.strftime('%Y-%m-%d')}.json"
    )
    out.write_text(json.dumps(report, indent=2) + "\n")
    print(out)
