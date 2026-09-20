FROM golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36 AS go-builder

ARG TARGETARCH=amd64
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
COPY umcp/go.mod umcp/go.sum ./umcp/
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY umcp ./umcp
COPY tools ./tools
COPY models/needle/memento-router.ndl /src/models/needle/memento-router.ndl
RUN case "$TARGETARCH" in \
        amd64) export GOAMD64=v1 ;; \
        arm64) ;; \
        *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; \
    esac \
    && CGO_ENABLED=0 GOOS=linux GOARCH="$TARGETARCH" go build -trimpath -buildvcs=false \
        -ldflags="-s -w -X 'main.version=$VERSION' -X 'github.com/rcarmo/memento/internal/service.BuildVersion=$VERSION'" \
        -o /out/memento-go ./cmd/memento-go \
    && CGO_ENABLED=0 GOOS=linux GOARCH="$TARGETARCH" go build -trimpath -buildvcs=false \
        -ldflags="-s -w" -o /out/memento-embed-go ./cmd/memento-embed-go \
    && CGO_ENABLED=0 GOOS=linux GOARCH="$TARGETARCH" go build -trimpath -buildvcs=false \
        -ldflags="-s -w" -o /out/memento-needle-go ./cmd/memento-needle-go \
    && CGO_ENABLED=0 GOOS=linux GOARCH="$TARGETARCH" go build -trimpath -buildvcs=false \
        -ldflags="-s -w" -o /out/memento-needle-model-go ./cmd/memento-needle-model-go \
    && CGO_ENABLED=0 GOOS=linux GOARCH="$TARGETARCH" go build -trimpath -buildvcs=false \
        -ldflags="-s -w" -o /out/memento-skill-import-go ./cmd/memento-skill-import-go \
    && /out/memento-needle-model-go /src/models/needle/memento-router.ndl /out/memento-router.nfp32 \
    && mkdir -p /out/rootfs/var/lib/memento/tmp /out/rootfs/models

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
LABEL org.opencontainers.image.title="Memento" \
    org.opencontainers.image.description="Pure-Go shared memory service for MCP agents" \
    org.opencontainers.image.source="https://github.com/rcarmo/memento" \
    org.opencontainers.image.version="$VERSION" \
    org.opencontainers.image.revision="$COMMIT" \
    org.opencontainers.image.created="$BUILD_DATE"

COPY --from=go-builder /out/memento-go /usr/local/bin/memento-go
COPY --from=go-builder /out/memento-embed-go /usr/local/bin/memento-embed-go
COPY --from=go-builder /out/memento-embed-go /usr/local/bin/memento-embed
COPY --from=go-builder /out/memento-needle-go /usr/local/bin/memento-needle-go
COPY --from=go-builder /out/memento-needle-model-go /usr/local/bin/memento-needle-model-go
COPY --from=go-builder /out/memento-skill-import-go /usr/local/bin/memento-skill-import-go
COPY --from=go-builder --chown=65532:65532 /out/rootfs/ /
COPY models/gte/gte-small.gtemodel /usr/local/share/memento/models/gte-small.gtemodel
COPY models/needle/memento-router.ndl /usr/local/share/memento/models/memento-router.ndl
COPY --from=go-builder /out/memento-router.nfp32 /usr/local/share/memento/models/memento-router.nfp32
COPY models/needle/needle.model /usr/local/share/memento/models/needle.model

ENV MEMENTO_GTE_MODEL=/usr/local/share/memento/models/gte-small.gtemodel \
    MEMENTO_NEEDLE_MODEL=/usr/local/share/memento/models/memento-router.ndl \
    MEMENTO_NEEDLE_FP32_MODEL=/usr/local/share/memento/models/memento-router.nfp32 \
    MEMENTO_NEEDLE_WORKER=/usr/local/bin/memento-needle-go \
    MEMENTO_NEEDLE_TOKENIZER=/usr/local/share/memento/models/needle.model \
    TMPDIR=/var/lib/memento/tmp

USER 65532:65532
VOLUME ["/var/lib/memento", "/models"]
EXPOSE 8000
HEALTHCHECK --interval=30s --timeout=5s --start-period=5m --retries=3 \
    CMD ["/usr/local/bin/memento-go", "healthcheck", "--address", "127.0.0.1:8000", "--timeout", "2s"]
ENTRYPOINT ["/usr/local/bin/memento-go"]
CMD ["--config", "/etc/memento/config.json", "serve", "--http", "--host", "0.0.0.0", "--port", "8000", "--endpoint", "/mcp"]
