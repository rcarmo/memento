FROM rust:1-slim AS rust-builder

RUN apt-get update \
    && apt-get install -y --no-install-recommends libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build
COPY rust /build/rust
RUN cd rust && cargo build --release -p memento-ffi -p memento-sqlite-vector -p memento-embed -p memento-needle-ffi

FROM python:3.14-slim

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
COPY --from=rust-builder /build/rust/target/release/libmemento_ffi.so /usr/local/lib/memento/
COPY --from=rust-builder /build/rust/target/release/libmemento_sqlite_vector.so /usr/local/lib/memento/
COPY --from=rust-builder /build/rust/target/release/libmemento_needle_ffi.so /usr/local/lib/memento/
COPY --from=rust-builder /build/rust/target/release/memento-embed /usr/local/bin/
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
ENTRYPOINT ["memento-serve"]
CMD ["--help"]
