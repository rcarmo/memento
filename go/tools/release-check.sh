#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
OUT=${RELEASE_DIR:-$ROOT/../build/go/release}
VERSION=${VERSION:-development}
GO=${GO:-go}
cd "$OUT"
sha256sum -c SHA256SUMS
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
if test -n "${NEEDLE_MODEL_PATH:-}"; then
  (cd "$ROOT" && "$GO" run ./cmd/memento-needle-model-go "$NEEDLE_MODEL_PATH" "$TMP/expected.nfp32")
fi
for ARCH in amd64 arm64; do
  ARCHIVE="memento-go-$VERSION-linux-$ARCH.tar.gz"
  EXPECTED="$TMP/expected-$ARCH"
  cat > "$EXPECTED" <<EOF
memento-go-$VERSION-linux-$ARCH/
memento-go-$VERSION-linux-$ARCH/VERSION
memento-go-$VERSION-linux-$ARCH/memento-embed-go
memento-go-$VERSION-linux-$ARCH/memento-go
memento-go-$VERSION-linux-$ARCH/memento-needle-go
memento-go-$VERSION-linux-$ARCH/memento-needle-model-go
memento-go-$VERSION-linux-$ARCH/memento-skill-import-go
EOF
  if test -n "${NEEDLE_MODEL_PATH:-}"; then
    printf '%s\n' "memento-go-$VERSION-linux-$ARCH/memento-router.nfp32" >> "$EXPECTED"
    LC_ALL=C sort -o "$EXPECTED" "$EXPECTED"
  fi
  tar -tzf "$ARCHIVE" > "$TMP/list-$ARCH"
  LC_ALL=C sort -o "$TMP/list-$ARCH" "$TMP/list-$ARCH"
  diff -u "$EXPECTED" "$TMP/list-$ARCH"
  tar -xzf "$ARCHIVE" -C "$TMP"
  for NAME in memento-go memento-embed-go memento-needle-go memento-needle-model-go memento-skill-import-go; do
    BIN="$TMP/memento-go-$VERSION-linux-$ARCH/$NAME"
    file "$BIN" | grep -q 'statically linked'
    if readelf -d "$BIN" 2>/dev/null | grep -q NEEDED; then echo "$BIN has dynamic dependencies" >&2; exit 1; fi
    case "$ARCH" in amd64) file "$BIN" | grep -q 'x86-64';; arm64) file "$BIN" | grep -q 'ARM aarch64';; esac
  done
  if test -n "${NEEDLE_MODEL_PATH:-}"; then
    cmp "$TMP/memento-go-$VERSION-linux-$ARCH/memento-router.nfp32" "$TMP/expected.nfp32"
  fi
done
"$TMP/memento-go-$VERSION-linux-amd64/memento-go" version | grep -F "memento-go $VERSION"
