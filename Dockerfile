FROM rust:1.88-slim-bookworm@sha256:38bc5a86d998772d4aec2348656ed21438d20fcdce2795b56ca434cf21430d89 AS rust-builder

ARG TARGETARCH=amd64

RUN apt-get update \
    && apt-get install -y --no-install-recommends libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build
COPY rust /build/rust
RUN case "$TARGETARCH" in \
        amd64) export RUSTFLAGS="-C target-cpu=x86-64" ;; \
        arm64) export RUSTFLAGS="-C target-cpu=generic" ;; \
        *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; \
    esac \
    && cd rust \
    && cargo build --release -p memento-ffi -p memento-sqlite-vector -p memento-embed -p memento-needle-ffi \
    && cargo build --release -p memento-vector --bin memento-cpu-features

FROM python:3.12-slim-bookworm@sha256:d50fb7611f86d04a3b0471b46d7557818d88983fc3136726336b2a4c657aa30b

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
LABEL org.opencontainers.image.title="Memento" \
    org.opencontainers.image.description="Shared memory service for MCP agents" \
    org.opencontainers.image.source="https://github.com/rcarmo/memento" \
    org.opencontainers.image.version="$VERSION" \
    org.opencontainers.image.revision="$COMMIT" \
    org.opencontainers.image.created="$BUILD_DATE"

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    TMPDIR=/var/lib/memento/tmp

RUN apt-get update \
    && apt-get install -y --no-install-recommends git git-lfs \
    && rm -rf /var/lib/apt/lists/* \
    && addgroup --system --gid 65532 memento \
    && adduser --system --uid 65532 --ingroup memento memento \
    && mkdir -p /app /var/lib/memento/tmp \
    && chown -R memento:memento /app /var/lib/memento

WORKDIR /app
COPY pyproject.toml README.md /app/
COPY src /app/src
COPY tools/container_cpu_smoke.py /usr/local/lib/memento/container_cpu_smoke.py
COPY --from=rust-builder /build/rust/target/release/libmemento_ffi.so /usr/local/lib/memento/
COPY --from=rust-builder /build/rust/target/release/libmemento_sqlite_vector.so /usr/local/lib/memento/
COPY --from=rust-builder /build/rust/target/release/libmemento_needle_ffi.so /usr/local/lib/memento/
COPY --from=rust-builder /build/rust/target/release/memento-embed /usr/local/bin/
COPY --from=rust-builder /build/rust/target/release/memento-cpu-features /usr/local/bin/
COPY models/gte/gte-small.gtemodel /usr/local/share/memento/models/gte-small.gtemodel
COPY models/needle/memento-router.ndl /usr/local/share/memento/models/memento-router.ndl
COPY models/needle/needle.model /usr/local/share/memento/models/needle.model
RUN python -m pip install --upgrade pip \
    && python -m pip install '.[mcp]'

ENV MEMENTO_FFI_LIBRARY=/usr/local/lib/memento/libmemento_ffi.so \
    MEMENTO_SQLITE_VECTOR_EXTENSION=/usr/local/lib/memento/libmemento_sqlite_vector.so \
    MEMENTO_GTE_MODEL=/usr/local/share/memento/models/gte-small.gtemodel \
    MEMENTO_NEEDLE_FFI_LIBRARY=/usr/local/lib/memento/libmemento_needle_ffi.so \
    MEMENTO_NEEDLE_MODEL=/usr/local/share/memento/models/memento-router.ndl \
    MEMENTO_NEEDLE_TOKENIZER=/usr/local/share/memento/models/needle.model

USER 65532:65532
VOLUME ["/var/lib/memento", "/models"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD ["python", "-c", "import socket; socket.create_connection(('127.0.0.1', 8000), 2).close()"]
ENTRYPOINT ["memento-serve"]
CMD ["--help"]
