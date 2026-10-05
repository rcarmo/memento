#!/usr/bin/env bash
# Source from helpers, or execute as Make's shell / direct command wrapper.
_memento_env_main() {
  local repo root purpose path oldtmp
  repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
  # Resolver is vendored: generic CI never depends on /workspace tooling.
  source "$repo/tools/project-tmp.sh"
  root=$(project_tmp_resolve memento-go) || return
  project_tmp_init "$root" || return
  export PROJECT_TMP_ROOT="$root"
  export MEMENTO_REPO_ROOT="$repo"
  export GOTOOLCHAIN=${GOTOOLCHAIN:-go1.26.6}
  # Checkouts get distinct output directories; caches are safely tool-shared.
  export BUILD_ROOT="$root/build/$(printf '%s' "$repo" | cksum | cut -d' ' -f1)"
  if [[ -z ${MEMENTO_RUN_ROOT:-} ]]; then
    purpose=${MEMENTO_RUN_PURPOSE:-command}
    project_name_valid "$purpose" || return 1
    project_path_usable "$root/runs/$purpose" || return 1
    mkdir -p "$root/runs/$purpose"
    export MEMENTO_RUN_ROOT=$(mktemp -d "$root/runs/$purpose/run-$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX")
  fi
  local physical_root
  physical_root=$(cd "$root" && pwd -P)
  case "$MEMENTO_RUN_ROOT" in "$root"/runs/*/*|"$physical_root"/runs/*/*) ;; *) echo 'Run root must be project-owned' >&2; return 1;; esac
  project_path_usable "$MEMENTO_RUN_ROOT" || return 1
  export MEMENTO_RUN_ROOT=$(cd "$MEMENTO_RUN_ROOT" && pwd -P)
  export TMPDIR="$MEMENTO_RUN_ROOT/tmp" TMP="$MEMENTO_RUN_ROOT/tmp" TEMP="$MEMENTO_RUN_ROOT/tmp"
  export GOTMPDIR="$MEMENTO_RUN_ROOT/go-tmp"
  export GOCACHE="$root/cache/go-build" GOMODCACHE="$root/cache/go-mod"
  export XDG_CACHE_HOME="$root/cache/xdg" BUN_INSTALL_CACHE_DIR="$root/cache/bun"
  export npm_config_cache="$root/cache/npm" PIP_CACHE_DIR="$root/cache/pip" UV_CACHE_DIR="$root/cache/uv"
  export PLAYWRIGHT_BROWSERS_PATH="$root/cache/playwright" PYTHONPYCACHEPREFIX="$root/cache/python"
  export TOOLS_DIR="$root/cache/go-tools"
  # Retained evidence is deliberately outside cleanable build/scratch.
  export PROFILE_ROOT=${PROFILE_ROOT:-$repo/build/profiles}
  for path in "$BUILD_ROOT" "$TMPDIR" "$GOTMPDIR" "$GOCACHE" "$GOMODCACHE" "$XDG_CACHE_HOME" "$BUN_INSTALL_CACHE_DIR" "$npm_config_cache" "$PIP_CACHE_DIR" "$UV_CACHE_DIR" "$PLAYWRIGHT_BROWSERS_PATH" "$PYTHONPYCACHEPREFIX" "$TOOLS_DIR"; do
    project_path_usable "$path" || { echo "Unsafe project path: $path" >&2; return 1; }
    mkdir -p "$path" || return
  done
  # Repository isolation rejects symlink ancestors. Resolve the owned run's
  # physical path without weakening those checks (/workspace is a host alias).
  export TMPDIR=$(cd "$TMPDIR" && pwd -P)
  export TMP="$TMPDIR" TEMP="$TMPDIR"
  export GOTMPDIR=$(cd "$GOTMPDIR" && pwd -P)
}
_memento_env_main || { return 1 2>/dev/null || exit 1; }
if [[ ${BASH_SOURCE[0]} == "$0" ]]; then
  if [[ ${1:-} == --print-build ]]; then printf '%s\n' "$BUILD_ROOT"
  elif [[ ${1:-} == --print-tools ]]; then printf '%s\n' "$TOOLS_DIR"
  elif [[ ${1:-} == --clean-build ]]; then
    project_path_usable "$BUILD_ROOT" || exit 1
    case "$BUILD_ROOT" in "$PROJECT_TMP_ROOT"/build/*) rm -rf -- "$BUILD_ROOT" ;; *) exit 1;; esac
  elif [[ ${1:-} == -c ]]; then shift; exec bash -c "$*"
  else exec "$@"; fi
fi
