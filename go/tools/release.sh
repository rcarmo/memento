#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
OUT=${RELEASE_DIR:-$ROOT/../build/go/release}
VERSION=${VERSION:-development}
EPOCH=${SOURCE_DATE_EPOCH:-0}
GO=${GO:-go}
rm -rf "$OUT"
mkdir -p "$OUT"
for ARCH in amd64 arm64; do
  STAGE="$OUT/memento-go-$VERSION-linux-$ARCH"
  mkdir -p "$STAGE"
  if [ "$ARCH" = amd64 ]; then AMD=GOAMD64=v1; else AMD=; fi
  env CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" $AMD "$GO" build -trimpath -buildvcs=false -ldflags="-s -w -X 'main.version=memento-go $VERSION (compatibility baseline: 0.5.9; pure-Go service)'" -o "$STAGE/memento-go" ./cmd/memento-go
  env CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" $AMD "$GO" build -trimpath -buildvcs=false -ldflags="-s -w" -o "$STAGE/memento-embed-go" ./cmd/memento-embed-go
  printf '%s\n' "$VERSION" > "$STAGE/VERSION"
  chmod 0755 "$STAGE/memento-go" "$STAGE/memento-embed-go"
  touch -d "@$EPOCH" "$STAGE"/*
  tar --sort=name --mtime="@$EPOCH" --owner=0 --group=0 --numeric-owner -C "$OUT" -czf "$OUT/memento-go-$VERSION-linux-$ARCH.tar.gz" "$(basename "$STAGE")"
  rm -rf "$STAGE"
done
(cd "$OUT" && sha256sum ./*.tar.gz | LC_ALL=C sort -k2 > SHA256SUMS)
