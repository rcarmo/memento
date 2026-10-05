#!/usr/bin/env bash
set -u
set -o pipefail

GO=${GO:-go}
PROFILE_ROOT=${PROFILE_ROOT:-build/profiles}
PROFILE_MEM_RATE=${PROFILE_MEM_RATE:-524288}
PROFILE_TIMEOUT=${PROFILE_TIMEOUT:-20m}
patterns=()
flags=()
allow_empty=0
fuzz_mode=0
focused_run=0
timeout_flag=0
list_flags=()

while (($#)); do
  if [[ $1 == -- ]]; then
    shift
    flags=("$@")
    break
  fi
  patterns+=("$1")
  shift
done
((${#patterns[@]})) || patterns=(./...)

for ((i=0; i<${#flags[@]}; i++)); do
  flag=${flags[i]}
  case "$flag" in
    -bench|-bench=*|-fuzz|-fuzz=*) allow_empty=1 ;;
  esac
  case "$flag" in
    -fuzz|-fuzz=*) fuzz_mode=1 ;;
    -run|-run=*) focused_run=1 ;;
    -timeout|-timeout=*) timeout_flag=1 ;;
    -tags|-mod|-modfile|-overlay)
      list_flags+=("$flag")
      if ((i + 1 >= ${#flags[@]})); then
        echo "missing value for $flag" >&2
        exit 2
      fi
      list_flags+=("${flags[i+1]}")
      ((i+=1))
      ;;
    -tags=*|-mod=*|-modfile=*|-overlay=*|-race)
      list_flags+=("$flag")
      ;;
  esac
  case "$flag" in
    -count|-count=*|-c|-cpuprofile|-cpuprofile=*|-coverprofile|-coverprofile=*|-list|-list=*|-memprofile|-memprofile=*|-memprofilerate|-memprofilerate=*|-o|-o=*|-outputdir|-outputdir=*)
      echo "managed by tools/test-profile.sh: $flag" >&2
      exit 2
      ;;
  esac
done

if ((fuzz_mode)); then
  echo 'Fuzz execution blocked: go test profiles only its coordinator, not fuzz workers. Add worker-aware profiling before running this gate.' >&2
  exit 2
fi

mkdir -p "$PROFILE_ROOT" || exit 1
run=$(mktemp -d "$PROFILE_ROOT/run-$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX") || exit 1
run=$(cd "$run" && pwd)
repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
revision=$(git rev-parse HEAD 2>/dev/null || echo unknown)
toolchain=$($GO version 2>&1 || true)
module_file=$($GO env GOMOD 2>/dev/null || true)
module_root=$(dirname "$module_file")
if [[ -z $module_file || $module_file == /dev/null ]]; then
  module_root=$(pwd)
fi

git diff HEAD --binary > "$run/worktree.patch" 2>/dev/null || true
# Preserve new source files too: a HEAD diff alone omits untracked additions.
while IFS= read -r -d '' file; do
  mkdir -p "$run/untracked/$(dirname "$file")"
  cp -- "$repo_root/$file" "$run/untracked/$file" || exit 1
done < <(git -C "$repo_root" ls-files --others --exclude-standard -z)

echo "Profiles and analysis: $run"
printf '%s\n' "$run" > "$PROFILE_ROOT/latest-run.txt"
{
  printf 'revision=%s\n' "$revision"
  printf 'cpu_sampling=Go testing default 100Hz\n'
  printf 'CGO_ENABLED=%s\nGOMAXPROCS=%s\n' "${CGO_ENABLED:-default}" "${GOMAXPROCS:-default}"
  git status --short 2>/dev/null || true
  printf 'repo_root=%s\n' "$repo_root"
  printf 'module_root=%s\n' "$module_root"
  printf 'cwd=%s\n' "$(pwd)"
  printf 'go=%s\n' "$GO"
  printf 'toolchain=%s\n' "$toolchain"
  printf 'goenv_goversion=%s\n' "$($GO env GOVERSION 2>/dev/null || true)"
  printf 'goenv_gotoolchain=%s\n' "$($GO env GOTOOLCHAIN 2>/dev/null || true)"
  printf 'goenv_goos=%s\n' "$($GO env GOOS 2>/dev/null || true)"
  printf 'goenv_goarch=%s\n' "$($GO env GOARCH 2>/dev/null || true)"
  printf 'memprofilerate=%s\n' "$PROFILE_MEM_RATE"
  printf 'package_patterns:'
  printf ' %q' "${patterns[@]}"
  printf '\nflags:'
  printf ' %q' "${flags[@]}"
  printf '\n'
} > "$run/metadata.txt"

if ! "$GO" list "${list_flags[@]}" -f '{{.ImportPath}}|{{if or .TestGoFiles .XTestGoFiles}}tests{{else}}build-only{{end}}' "${patterns[@]}" > "$run/packages.txt" 2> "$run/list.log"; then
  cat "$run/list.log" >&2
  exit 1
fi

printf 'package\ttest_status\tanalysis_status\twarnings\n' > "$run/status.tsv"
if [[ -n ${COVERAGE_FILE:-} ]]; then
  : > "$run/coverage.out"
fi
failed=0

while IFS='|' read -r package kind; do
  [[ -n $package ]] || continue
  out="$run/$package"
  mkdir -p "$out"

  if [[ $kind == build-only ]]; then
    echo "Build-only package $package (no test execution/profiles expected)"
    "$GO" build "${list_flags[@]}" "$package" > "$out/build.log" 2>&1 || failed=1
    continue
  fi
  args=(test "$package" -count=1)
  if ((timeout_flag == 0)); then
    args+=("-timeout=$PROFILE_TIMEOUT")
  fi
  args+=("${flags[@]}" "-cpuprofile=$out/cpu.pprof" "-memprofile=$out/heap.pprof" "-memprofilerate=$PROFILE_MEM_RATE" "-o=$out/test.bin")
  if [[ -n ${COVERAGE_FILE:-} ]]; then
    args+=("-coverprofile=$out/coverage.out")
  fi

  {
    printf 'package=%s\n' "$package"
    printf 'kind=%s\n' "$kind"
    printf 'started_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf 'memprofilerate=%s\n' "$PROFILE_MEM_RATE"
    printf 'command:'
    printf ' %q' "$GO" "${args[@]}"
    printf '\n'
  } > "$out/metadata.txt"
  printf '%q ' "$GO" "${args[@]}" > "$out/command.txt"
  printf '\n' >> "$out/command.txt"

  status=0
  "$GO" "${args[@]}" > "$out/test.log" 2>&1 || status=$?
  cat "$out/test.log"

  if [[ $kind == tests && $allow_empty == 0 ]] && grep -Eq '(^|[[:space:]])(no tests to run|\[no tests to run\])' "$out/test.log"; then
    if ((focused_run)); then
      echo 'No tests matched; refusing a false verification pass.' >&2
      status=1
    fi
  fi

  analysis=0
  warnings=()

  if [[ -s $out/heap.pprof && -s $out/test.bin ]]; then
    "$GO" tool pprof -top -cum -alloc_space "$out/test.bin" "$out/heap.pprof" > "$out/alloc_space.txt" 2>&1 || analysis=1
    "$GO" tool pprof -top -cum -alloc_objects "$out/test.bin" "$out/heap.pprof" > "$out/alloc_objects.txt" 2>&1 || analysis=1
    if grep -Eq 'Total samples = 0|Showing nodes accounting for 0' "$out/alloc_space.txt" "$out/alloc_objects.txt" 2>/dev/null; then
      warnings+=("empty allocation profile sample")
    fi
  else
    printf 'Allocation profile unavailable (build failure, process crash or interrupted run).\n' > "$out/alloc_space.txt"
    cp "$out/alloc_space.txt" "$out/alloc_objects.txt"
    analysis=1
  fi

  if [[ -s $out/cpu.pprof && -s $out/test.bin ]]; then
    "$GO" tool pprof -top -cum "$out/test.bin" "$out/cpu.pprof" > "$out/cpu.txt" 2>&1 || analysis=1
    if grep -Eq 'Total samples = 0|Showing nodes accounting for 0' "$out/cpu.txt" 2>/dev/null; then
      warnings+=("empty cpu profile sample")
    fi
  else
    printf 'CPU profile unavailable (build failure, process crash or interrupted run).\n' > "$out/cpu.txt"
    analysis=1
  fi

  if [[ -n ${COVERAGE_FILE:-} ]]; then
    if [[ -s $out/coverage.out ]]; then
      if [[ ! -s $run/coverage.out ]]; then
        head -1 "$out/coverage.out" > "$run/coverage.out"
      fi
      if [[ $(head -1 "$run/coverage.out") != $(head -1 "$out/coverage.out") ]]; then
        echo 'Incompatible coverage modes across packages' >&2
        analysis=1
      else
        tail -n +2 "$out/coverage.out" >> "$run/coverage.out"
      fi
    else
      printf 'Coverage profile unavailable.\n' > "$out/coverage.txt"
      analysis=1
    fi
  fi

  {
    printf 'Package: %s\n' "$package"
    printf 'Test exit: %s\n' "$status"
    printf 'Analysis exit: %s\n' "$analysis"
    if ((${#warnings[@]})); then
      printf 'Warnings:\n'
      for warning in "${warnings[@]}"; do
        printf ' - %s\n' "$warning"
      done
    fi
    printf '\n=== alloc_space (cumulative) ===\n'
    cat "$out/alloc_space.txt"
    printf '\n=== alloc_objects (cumulative) ===\n'
    cat "$out/alloc_objects.txt"
    printf '\n=== cpu (cumulative) ===\n'
    cat "$out/cpu.txt"
    printf '\nReview cumulative application hotspots separately from test/runtime overhead before changing budgets or widening tolerances.\n'
  } > "$out/analysis.txt"
  cat "$out/analysis.txt"

  printf '%s\t%s\t%s\t' "$package" "$status" "$analysis" >> "$run/status.tsv"
  if ((${#warnings[@]})); then
    printf '%s' "${warnings[*]}" >> "$run/status.tsv"
  fi
  printf '\n' >> "$run/status.tsv"

  if ((status != 0 || analysis != 0)); then
    failed=1
  fi
done < "$run/packages.txt"

if [[ -n ${COVERAGE_FILE:-} ]]; then
  if [[ ! -s $run/coverage.out ]]; then
    printf 'mode: set\n' > "$run/coverage.out"
  fi
  cp "$run/coverage.out" "$COVERAGE_FILE" || failed=1
fi

echo "Result: $failed; profiles: $run"
exit "$failed"
