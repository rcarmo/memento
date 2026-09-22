#!/usr/bin/env bash
set -euo pipefail
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
OUT=${PERF_OUT:-$ROOT/build/performance.txt}
BUDGETS=$ROOT/tools/performance-budgets.json
mkdir -p "$(dirname "$OUT")"
: >"$OUT"
(
  cd "$ROOT"
  CGO_ENABLED=0 go test -run '^$' -bench 'Benchmark(SemanticBlobCosine384|SearchLexical100)$' -benchmem -benchtime=2s -count=3 ./internal/derived
  CGO_ENABLED=0 go test -run '^$' -bench 'BenchmarkTokenizerVocabularyLookups' -benchmem -count=3 ./internal/needle
  CGO_ENABLED=0 go test -run '^$' -bench 'Benchmark(DotSelected|DotRowsSelected|AXPYRowsSelected)$' -benchmem -count=3 ./internal/simd
  if test -n "${GTE_MODEL_PATH:-}"; then
    CGO_ENABLED=0 GTE_MODEL_PATH="$GTE_MODEL_PATH" go test -run '^$' -bench 'BenchmarkRealGTE(Embed|ColdWorkspace)$' -benchmem -benchtime=2s -count=3 ./internal/gte
  fi
  if test -n "${NEEDLE_MODEL_PATH:-}" -a -n "${NEEDLE_TOKENIZER_PATH:-}"; then
    CGO_ENABLED=0 NEEDLE_MODEL_PATH="$NEEDLE_MODEL_PATH" NEEDLE_TOKENIZER_PATH="$NEEDLE_TOKENIZER_PATH" go test -run '^$' -bench 'BenchmarkRealNeedleGenerate' -benchmem -benchtime=20x -count=3 ./internal/needle
  fi
) | tee "$OUT"
python3 - "$BUDGETS" "$OUT" <<'PY'
import json, re, sys
budgets=json.load(open(sys.argv[1]))['benchmarks']
rows={name:[] for name in budgets}
pattern=re.compile(r'^(Benchmark\S+)-\d+\s+\d+\s+([0-9.]+) ns/op\s+([0-9.]+) B/op\s+([0-9.]+) allocs/op$')
for line in open(sys.argv[2]):
    match=pattern.match(line.strip())
    if match and match.group(1) in rows:
        rows[match.group(1)].append((float(match.group(3)),float(match.group(4))))
errors=[]
for name,budget in budgets.items():
    values=rows[name]
    if len(values)<3:
        if name in {'BenchmarkRealGTEEmbed','BenchmarkRealNeedleGenerate'} and not values:
            continue
        errors.append(f'{name}: expected 3 benchmark samples, found {len(values)}')
        continue
    bytes_median=sorted(v[0] for v in values)[len(values)//2]
    allocs_max=max(v[1] for v in values)
    if bytes_median>budget['max_bytes_per_op']:
        errors.append(f'{name}: median {bytes_median} B/op > {budget["max_bytes_per_op"]}')
    if allocs_max>budget['max_allocs_per_op']:
        errors.append(f'{name}: {allocs_max} allocs/op > {budget["max_allocs_per_op"]}')
if errors:
    print('\n'.join(errors),file=sys.stderr); raise SystemExit(1)
print('performance allocation budgets passed')
PY
