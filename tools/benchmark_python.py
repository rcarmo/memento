#!/usr/bin/env python3
import json
import sys
import tempfile
import time
from pathlib import Path

from memento.authz import EffectivePolicy
from memento.derived.index import DerivedIndex

root = Path(sys.argv[1])
rebuilds = int(sys.argv[2])
searches = int(sys.argv[3])
samples = []
for _ in range(rebuilds):
    with tempfile.TemporaryDirectory() as temp:
        index = DerivedIndex(Path(temp) / "derived.sqlite")
        start = time.perf_counter_ns()
        index.rebuild(root, repo_revision="benchmark-revision")
        samples.append(time.perf_counter_ns() - start)
        # retain final index outside its TemporaryDirectory only on final loop
        if len(samples) == rebuilds:
            final = tempfile.TemporaryDirectory()
            final_index = DerivedIndex(Path(final.name) / "derived.sqlite")
            final_index.rebuild(root, repo_revision="benchmark-revision")
policy = EffectivePolicy(
    principal="benchmark", roles=("reader",), read_prefixes=("/",), write_prefixes=()
)
for _ in range(10):
    final_index.search(policy=policy, query="shared benchmark", query_syntax="plain", limit=20)
search_samples = []
count = 0
for _ in range(searches):
    start = time.perf_counter_ns()
    page = final_index.search(
        policy=policy, query="shared benchmark", query_syntax="plain", limit=20
    )
    search_samples.append(time.perf_counter_ns() - start)
    count = len(page.results)
print(
    json.dumps(
        {
            "runtime": "python",
            "rebuild_ns": samples,
            "search_ns": search_samples,
            "result_count": count,
        },
        separators=(",", ":"),
    )
)
final.cleanup()
