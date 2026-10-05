#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/project-env.sh" || exit 1
set -u
set -o pipefail

GO=${GO:-go}
PROFILE_ROOT=${PROFILE_ROOT:-build/profiles}
PROFILE_MEM_RATE=${PROFILE_MEM_RATE:-524288}
PROFILE_TIMEOUT=${PROFILE_TIMEOUT:-20m}
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
hook_template="$script_dir/profilehook_test.go.txt"
patterns=()
flags=()
allow_empty=0
fuzz_mode=0
focused_run=0
timeout_flag=0
list_flags=()

json_escape() {
  local value=${1//\\/\\\\}
  value=${value//\"/\\\"}
  value=${value//$'\n'/\\n}
  value=${value//$'\r'/\\r}
  value=${value//$'\t'/\\t}
  printf '%s' "$value"
}

join_warnings() {
  local IFS='; '
  printf '%s' "$*"
}

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

if [[ ! -f $hook_template ]]; then
  echo "missing hook template: $hook_template" >&2
  exit 1
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
while IFS= read -r -d '' file; do
  mkdir -p "$run/untracked/$(dirname "$file")"
  cp -- "$repo_root/$file" "$run/untracked/$file.snapshot" || exit 1
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
  printf 'profile_hook_template=%s\n' "$hook_template"
  printf 'package_patterns:'
  printf ' %q' "${patterns[@]}"
  printf '\nflags:'
  printf ' %q' "${flags[@]}"
  printf '\n'
} > "$run/metadata.txt"

if ! "$GO" list "${list_flags[@]}" -f '{{.ImportPath}}{{"\t"}}{{.Name}}{{"\t"}}{{.Dir}}{{"\t"}}{{if or .TestGoFiles .XTestGoFiles}}tests{{else}}build-only{{end}}' "${patterns[@]}" > "$run/packages.txt" 2> "$run/list.log"; then
  cat "$run/list.log" >&2
  exit 1
fi

printf 'package\ttest_status\tanalysis_status\twarnings\n' > "$run/status.tsv"
if [[ -n ${COVERAGE_FILE:-} ]]; then
  : > "$run/coverage.out"
fi
failed=0

while IFS=$'\t' read -r package package_name package_dir kind; do
  [[ -n $package ]] || continue
  out="$run/$package"
  mkdir -p "$out"

  if [[ $kind == build-only ]]; then
    echo "Build-only package $package (no test execution/profiles expected)"
    "$GO" build "${list_flags[@]}" "$package" > "$out/build.log" 2>&1 || failed=1
    continue
  fi

  hook_source="$out/profilehook_test.go.txt"
  overlay_file="$out/overlay.json"
  overlay_target="$package_dir/zz_memento_profilehook_test.go"
  profiles_dir="$out/profiles"
  base_cpu="$profiles_dir/placeholder-cpu.pprof"
  base_heap="$profiles_dir/placeholder-heap.pprof"
  mkdir -p "$profiles_dir"

  sed "s/__MEMENTO_TEST_PACKAGE__/$package_name/g" "$hook_template" > "$hook_source" || exit 1
  {
    printf '{\n'
    printf '  "Replace": {\n'
    printf '    "%s": "%s"\n' "$(json_escape "$overlay_target")" "$(json_escape "$hook_source")"
    printf '  }\n'
    printf '}\n'
  } > "$overlay_file"

  args=(test "-overlay=$overlay_file" "$package" -count=1)
  if ((timeout_flag == 0)); then
    args+=("-timeout=$PROFILE_TIMEOUT")
  fi
  args+=("${flags[@]}" "-o=$out/test.bin")
  # cmd/go rejects profiling flags with fuzz; inject inside the test processes.
  if ((fuzz_mode == 0)); then
    args+=("-cpuprofile=$base_cpu" "-memprofile=$base_heap" "-memprofilerate=$PROFILE_MEM_RATE")
  fi
  if [[ -n ${COVERAGE_FILE:-} ]]; then
    args+=("-coverprofile=$out/coverage.out")
  fi

  {
    printf 'package=%s\n' "$package"
    printf 'package_name=%s\n' "$package_name"
    printf 'package_dir=%s\n' "$package_dir"
    printf 'kind=%s\n' "$kind"
    printf 'started_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf 'memprofilerate=%s\n' "$PROFILE_MEM_RATE"
    printf 'profile_dir=%s\n' "$profiles_dir"
    printf 'profile_hook_source=%s\n' "$hook_source"
    printf 'profile_hook_overlay=%s\n' "$overlay_file"
    printf 'command:'
    printf ' %q' "$GO" "${args[@]}"
    printf '\n'
  } > "$out/metadata.txt"
  printf '%q ' "$GO" "${args[@]}" > "$out/command.txt"
  printf '\n' >> "$out/command.txt"

  status=0
  env MEMENTO_TEST_PROFILE_DIR="$profiles_dir" MEMENTO_TEST_MEMPROFILERATE="$PROFILE_MEM_RATE" "$GO" "${args[@]}" > "$out/test.log" 2>&1 || status=$?
  cat "$out/test.log"

  if [[ $kind == tests && $allow_empty == 0 ]] && grep -Eq '(^|[[:space:]])(no tests to run|\[no tests to run\])' "$out/test.log"; then
    if ((focused_run)); then
      echo 'No tests matched; refusing a false verification pass.' >&2
      status=1
    fi
  fi

  analysis=0
  warnings=()
  process_total=0
  worker_total=0
  other_total=0
  printf 'pid\tparent\tkind\tanalysis_status\twarnings\tcpuprofile\tmemprofile\tpath\n' > "$out/processes.tsv"
  : > "$out/process-analysis.txt"

  if [[ -d $profiles_dir ]]; then
    while IFS= read -r proc_dir; do
      [[ -n $proc_dir ]] || continue
      ((process_total+=1))
      proc_meta="$proc_dir/metadata.txt"
      proc_pid=${proc_dir##*/pid-}
      proc_parent=unknown
      proc_kind=process
      proc_path=$out/test.bin
      proc_cpu="$proc_dir/cpu.pprof"
      proc_heap="$proc_dir/heap.pprof"
      proc_rate=$PROFILE_MEM_RATE
      proc_fuzz=
      proc_warnings=()
      proc_analysis=0

      if [[ -f $proc_meta ]]; then
        while IFS= read -r line; do
          [[ $line == *=* ]] || continue
          key=${line%%=*}
          value=${line#*=}
          case "$key" in
            pid) proc_pid=$value ;;
            parent) proc_parent=$value ;;
            kind) proc_kind=$value ;;
            path) proc_path=$value ;;
            cpuprofile) proc_cpu=$value ;;
            memprofile) proc_heap=$value ;;
            memprofilerate) proc_rate=$value ;;
            fuzzworker) proc_fuzz=$value ;;
          esac
        done < "$proc_meta"
      else
        proc_warnings+=("missing metadata")
        proc_analysis=1
      fi

      if [[ $proc_kind == fuzzworker ]]; then
        ((worker_total+=1))
      else
        ((other_total+=1))
      fi

      profile_binary="$out/test.bin"
      if [[ $proc_kind == model-worker ]]; then
        profile_binary="$proc_dir/worker.bin"
        cp -- "$proc_path" "$profile_binary" || proc_analysis=1
      fi
      if [[ -s $proc_heap && -s $profile_binary ]]; then
        "$GO" tool pprof -top -cum -alloc_space "$profile_binary" "$proc_heap" > "$proc_dir/alloc_space.txt" 2>&1 || proc_analysis=1
        "$GO" tool pprof -top -cum -alloc_objects "$profile_binary" "$proc_heap" > "$proc_dir/alloc_objects.txt" 2>&1 || proc_analysis=1
        if grep -Eq 'Total samples = 0|Showing nodes accounting for 0' "$proc_dir/alloc_space.txt" "$proc_dir/alloc_objects.txt" 2>/dev/null; then
          proc_warnings+=("empty allocation profile sample")
        fi
      else
        printf 'Allocation profile unavailable for pid=%s (build failure, process crash, kill or interrupted run).\n' "$proc_pid" > "$proc_dir/alloc_space.txt"
        cp "$proc_dir/alloc_space.txt" "$proc_dir/alloc_objects.txt"
        proc_warnings+=("missing allocation profile")
        proc_analysis=1
      fi

      if [[ -s $proc_cpu && -s $profile_binary ]]; then
        "$GO" tool pprof -top -cum "$profile_binary" "$proc_cpu" > "$proc_dir/cpu.txt" 2>&1 || proc_analysis=1
        if grep -Eq 'Total samples = 0|Showing nodes accounting for 0' "$proc_dir/cpu.txt" 2>/dev/null; then
          proc_warnings+=("empty cpu profile sample")
        fi
      else
        printf 'CPU profile unavailable for pid=%s (build failure, process crash, kill or interrupted run).\n' "$proc_pid" > "$proc_dir/cpu.txt"
        proc_warnings+=("missing cpu profile")
        proc_analysis=1
      fi

      proc_warning_text=
      if ((${#proc_warnings[@]})); then
        proc_warning_text=$(join_warnings "${proc_warnings[@]}")
      fi

      {
        printf 'Process pid=%s parent=%s kind=%s\n' "$proc_pid" "$proc_parent" "$proc_kind"
        if [[ -n $proc_fuzz ]]; then
          printf 'Fuzz worker id: %s\n' "$proc_fuzz"
        fi
        printf 'Executable: %s\n' "$proc_path"
        printf 'CPU profile: %s\n' "$proc_cpu"
        printf 'Heap profile: %s\n' "$proc_heap"
        printf 'Memprofilerate: %s\n' "$proc_rate"
        printf 'Analysis exit: %s\n' "$proc_analysis"
        if ((${#proc_warnings[@]})); then
          printf 'Warnings:\n'
          for warning in "${proc_warnings[@]}"; do
            printf ' - %s\n' "$warning"
          done
        fi
        printf '\n=== alloc_space (cumulative) ===\n'
        cat "$proc_dir/alloc_space.txt"
        printf '\n=== alloc_objects (cumulative) ===\n'
        cat "$proc_dir/alloc_objects.txt"
        printf '\n=== cpu (cumulative) ===\n'
        cat "$proc_dir/cpu.txt"
        printf '\n'
      } > "$proc_dir/analysis.txt"
      cat "$proc_dir/analysis.txt" >> "$out/process-analysis.txt"

      printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
        "$proc_pid" "$proc_parent" "$proc_kind" "$proc_analysis" "$proc_warning_text" "$proc_cpu" "$proc_heap" "$proc_path" >> "$out/processes.tsv"

      if ((proc_analysis != 0)); then
        analysis=1
      fi
    done < <(find "$profiles_dir" -mindepth 1 -maxdepth 1 -type d \( -name 'pid-*' -o -name 'worker-*' \) | LC_ALL=C sort)
  fi

  if ((process_total == 0)); then
    warnings+=("no profiled process directories reported")
    analysis=1
  fi
  if [[ -e $base_cpu || -e $base_heap ]]; then
    warnings+=("placeholder profile path was written; hook rewrite incomplete")
    analysis=1
  fi
  if ((fuzz_mode)); then
    if ((worker_total == 0)); then
      warnings+=("no fuzz worker profiles captured")
      analysis=1
    fi
    if ((other_total == 0)); then
      warnings+=("no coordinator/helper profiles captured")
      analysis=1
    fi
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
    printf 'Profiled processes: %s\n' "$process_total"
    printf 'Fuzz workers: %s\n' "$worker_total"
    printf 'Coordinator/helpers: %s\n' "$other_total"
    if ((${#warnings[@]})); then
      printf 'Warnings:\n'
      for warning in "${warnings[@]}"; do
        printf ' - %s\n' "$warning"
      done
    fi
    printf '\n'
    if [[ -s $out/process-analysis.txt ]]; then
      cat "$out/process-analysis.txt"
    fi
    printf 'Review cumulative application hotspots separately from test/runtime overhead before changing budgets or widening tolerances.\n'
  } > "$out/analysis.txt"
  cat "$out/analysis.txt"

  printf '%s\t%s\t%s\t' "$package" "$status" "$analysis" >> "$run/status.tsv"
  if ((${#warnings[@]})); then
    printf '%s' "$(join_warnings "${warnings[@]}")" >> "$run/status.tsv"
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
