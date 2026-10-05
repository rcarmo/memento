#!/usr/bin/env bash
set -euo pipefail

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
GO=${GO:-go}
OUT=${PERF_OUT:-$ROOT/build/performance.txt}
BUDGETS=$ROOT/tools/performance-budgets.json
PROFILE_SCRIPT=${PROFILE_SCRIPT:-$ROOT/tools/test-profile.sh}
mkdir -p "$(dirname "$OUT")"
: >"$OUT"
exec > >(tee "$OUT") 2>&1

status=0

run_bench() {
  local package=$1
  local bench=$2
  shift 2
  local sample
  for sample in 1 2 3; do
    printf 'benchmark sample %s/3 package=%s bench=%s\n' "$sample" "$package" "$bench"
    CGO_ENABLED=0 "$PROFILE_SCRIPT" "$package" -- -run '^$' -bench "$bench" -benchmem "$@" || status=1
  done
}

cd "$ROOT"
run_bench ./internal/derived 'Benchmark(SemanticBlobCosine384|SearchLexical100)$' -benchtime=2s
run_bench ./internal/needle 'BenchmarkTokenizerVocabularyLookups$'
run_bench ./internal/simd 'Benchmark(DotSelected|DotRowsSelected|AXPYRowsSelected)$'
if [[ -n ${GTE_MODEL_PATH:-} ]]; then
  GTE_MODEL_PATH="$GTE_MODEL_PATH" run_bench ./internal/gte 'BenchmarkRealGTE(Embed|ColdWorkspace)$' -benchtime=2s
fi
if [[ -n ${NEEDLE_MODEL_PATH:-} && -n ${NEEDLE_TOKENIZER_PATH:-} ]]; then
  NEEDLE_MODEL_PATH="$NEEDLE_MODEL_PATH" NEEDLE_TOKENIZER_PATH="$NEEDLE_TOKENIZER_PATH" run_bench ./internal/needle 'BenchmarkRealNeedleGenerate$' -benchtime=20x
fi

python3 - "$BUDGETS" "$OUT" <<'PY' || status=1
import json, re, sys
budgets=json.load(open(sys.argv[1]))['benchmarks']
rows={name:[] for name in budgets}
pattern=re.compile(r'^(Benchmark\S+)-\d+\s+\d+\s+([0-9.]+) ns/op\s+([0-9.]+) B/op\s+([0-9.]+) allocs/op$')
for line in open(sys.argv[2]):
    match=pattern.match(line.strip())
    if match and match.group(1) in rows:
        rows[match.group(1)].append((float(match.group(2)),float(match.group(3)),float(match.group(4))))
errors=[]
for name,budget in budgets.items():
    values=rows[name]
    if len(values)<3:
        if name in {'BenchmarkRealGTEEmbed','BenchmarkRealGTEColdWorkspace','BenchmarkRealNeedleGenerate'} and not values:
            continue
        errors.append(f'{name}: expected 3 benchmark samples, found {len(values)}')
        continue
    bytes_median=sorted(v[1] for v in values)[len(values)//2]
    allocs_max=max(v[2] for v in values)
    if bytes_median>budget['max_bytes_per_op']:
        errors.append(f'{name}: median {bytes_median} B/op > {budget["max_bytes_per_op"]}')
    if allocs_max>budget['max_allocs_per_op']:
        errors.append(f'{name}: {allocs_max} allocs/op > {budget["max_allocs_per_op"]}')
if errors:
    print('\n'.join(errors),file=sys.stderr)
    raise SystemExit(1)
print('performance allocation budgets passed')
PY

exit "$status"
