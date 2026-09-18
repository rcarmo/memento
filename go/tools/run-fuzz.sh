#!/bin/sh
set -eu

GO=${GO:-go}
FUZZ_COUNT=${FUZZ_COUNT:-10000x}
FUZZ_TIMEOUT=${FUZZ_TIMEOUT:-120s}
FUZZ_PARALLEL=${FUZZ_PARALLEL:-2}

find . -name '*_test.go' -not -path './vendor/*' -print | sort |
  while IFS= read -r file; do
    pkg=./$(dirname "${file#./}")
    grep -hE '^func Fuzz[A-Za-z0-9_]+' "$file" |
      sed -E 's/^func (Fuzz[A-Za-z0-9_]+).*/\1/' |
      while IFS= read -r fuzz; do
        printf 'fuzz %s %s\n' "$pkg" "$fuzz"
        CGO_ENABLED=0 "$GO" test "$pkg" -run='^$' -fuzz="^${fuzz}$" \
          -fuzztime="$FUZZ_COUNT" -parallel="$FUZZ_PARALLEL" -timeout="$FUZZ_TIMEOUT"
      done
  done
