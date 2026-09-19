#!/bin/sh
set -eu

GO=${GO:-go}
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

check_module() {
  module=$1
  cd "$module"
  "$GO" list -f '{{if len .GoFiles}}{{.ImportPath}}|{{.Dir}}{{end}}' ./... |
    while IFS='|' read -r package directory; do
      test -n "$package" || continue
      case "$package" in github.com/rcarmo/memento/go/tools) continue ;; esac
      if ! grep -qsE '^func Fuzz[A-Za-z0-9_]+' "$directory"/*_test.go 2>/dev/null; then
        printf 'package with production Go code has no fuzz target: %s\n' "$package" >&2
        exit 1
      fi
    done
}

check_module "$ROOT"
check_module "$ROOT/umcp"
printf '%s\n' 'every Go runtime package has a fuzz target'
