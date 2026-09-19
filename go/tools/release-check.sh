#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
OUT=${RELEASE_DIR:-$ROOT/../build/go/release}
VERSION=${VERSION:-development}
cd "$OUT"
sha256sum -c SHA256SUMS
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
for ARCH in amd64 arm64; do
  ARCHIVE="memento-go-$VERSION-linux-$ARCH.tar.gz"
  EXPECTED="$TMP/expected-$ARCH"
  cat > "$EXPECTED" <<EOF
memento-go-$VERSION-linux-$ARCH/
memento-go-$VERSION-linux-$ARCH/VERSION
memento-go-$VERSION-linux-$ARCH/memento-embed-go
memento-go-$VERSION-linux-$ARCH/memento-go
EOF
  tar -tzf "$ARCHIVE" > "$TMP/list-$ARCH"
  diff -u "$EXPECTED" "$TMP/list-$ARCH"
  tar -xzf "$ARCHIVE" -C "$TMP"
  for NAME in memento-go memento-embed-go; do
    BIN="$TMP/memento-go-$VERSION-linux-$ARCH/$NAME"
    file "$BIN" | grep -q 'statically linked'
    if readelf -d "$BIN" 2>/dev/null | grep -q NEEDED; then echo "$BIN has dynamic dependencies" >&2; exit 1; fi
    case "$ARCH" in amd64) file "$BIN" | grep -q 'x86-64';; arm64) file "$BIN" | grep -q 'ARM aarch64';; esac
  done
done
"$TMP/memento-go-$VERSION-linux-amd64/memento-go" version | grep -F "memento-go $VERSION"
