#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/project-env.sh" || exit 1
set -euo pipefail

GO=${GO:-go}
FUZZ_COUNT=${FUZZ_COUNT:-10000x}
FUZZ_TIMEOUT=${FUZZ_TIMEOUT:-120s}
FUZZ_PARALLEL=${FUZZ_PARALLEL:-2}
PROFILE_SCRIPT=${PROFILE_SCRIPT:-$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/test-profile.sh}

find_args=(. -name '*_test.go' -not -path './vendor/*')
if [[ -d ./umcp ]]; then
  find_args+=(-not -path './umcp/*')
fi

mapfile -t targets < <(
  find "${find_args[@]}" -print | sort |
    while IFS= read -r file; do
      pkg=./$(dirname "${file#./}")
      { grep -hE '^func Fuzz[A-Za-z0-9_]+' "$file" || true; } |
        sed -E 's/^func (Fuzz[A-Za-z0-9_]+).*/\1/' |
        while IFS= read -r fuzz; do
          printf '%s\t%s\n' "$pkg" "$fuzz"
        done
    done | sort -u
)

if ((${#targets[@]} == 0)); then
  echo 'No fuzz targets found.' >&2
  exit 1
fi

status=0
for target in "${targets[@]}"; do
  pkg=${target%%$'\t'*}
  fuzz=${target#*$'\t'}
  printf 'fuzz %s %s\n' "$pkg" "$fuzz"
  CGO_ENABLED=0 "$PROFILE_SCRIPT" "$pkg" -- -run '^$' -fuzz "^${fuzz}$" -fuzztime="$FUZZ_COUNT" -parallel="$FUZZ_PARALLEL" -timeout="$FUZZ_TIMEOUT" || status=1
done

if ((status != 0)); then
  echo 'One or more fuzz targets failed or produced incomplete profiling artifacts.' >&2
fi
exit "$status"
