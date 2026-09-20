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
  env CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" $AMD "$GO" build -trimpath -buildvcs=false -ldflags="-s -w -X 'main.version=$VERSION' -X 'github.com/rcarmo/memento/go/service.BuildVersion=$VERSION'" -o "$STAGE/memento-go" ./cmd/memento-go
  env CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" $AMD "$GO" build -trimpath -buildvcs=false -ldflags="-s -w" -o "$STAGE/memento-embed-go" ./cmd/memento-embed-go
  env CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" $AMD "$GO" build -trimpath -buildvcs=false -ldflags="-s -w" -o "$STAGE/memento-needle-go" ./cmd/memento-needle-go
  env CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" $AMD "$GO" build -trimpath -buildvcs=false -ldflags="-s -w" -o "$STAGE/memento-needle-model-go" ./cmd/memento-needle-model-go
  env CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" $AMD "$GO" build -trimpath -buildvcs=false -ldflags="-s -w" -o "$STAGE/memento-skill-import-go" ./cmd/memento-skill-import-go
  if [ -n "${NEEDLE_MODEL_PATH:-}" ]; then
    "$GO" run ./cmd/memento-needle-model-go "$NEEDLE_MODEL_PATH" "$STAGE/memento-router.nfp32"
  fi
  printf '%s\n' "$VERSION" > "$STAGE/VERSION"
  chmod 0755 "$STAGE/memento-go" "$STAGE/memento-embed-go" "$STAGE/memento-needle-go" "$STAGE/memento-needle-model-go" "$STAGE/memento-skill-import-go"
  touch -d "@$EPOCH" "$STAGE"/*
  tar --sort=name --mtime="@$EPOCH" --owner=0 --group=0 --numeric-owner -C "$OUT" -czf "$OUT/memento-go-$VERSION-linux-$ARCH.tar.gz" "$(basename "$STAGE")"
  rm -rf "$STAGE"
done
(cd "$OUT" && sha256sum ./*.tar.gz | LC_ALL=C sort -k2 > SHA256SUMS)
