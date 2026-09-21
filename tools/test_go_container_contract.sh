#!/usr/bin/env bash
set -euo pipefail

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
IMAGE=${IMAGE:-memento-go:contract}
VERSION=${VERSION:-1.0.2}
ROLLBACK_IMAGE=${ROLLBACK_IMAGE:-ghcr.io/rcarmo/memento@sha256:bebc0a3eaf935a5b4f07c3e060fd8e22a11dacff90cd55532ec04306c30e81bc}
STATE=$(mktemp -d)
ENV_FILE=$(mktemp)
CONTAINER=memento-go-contract-$$
HEADERS=$(mktemp)
BODY=$(mktemp)

cleanup() {
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
    rm -f "$ENV_FILE" "$HEADERS" "$BODY"
    rm -rf "$STATE" 2>/dev/null || sudo rm -rf "$STATE" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

for path in \
    "$ROOT/models/gte/gte-small.gtemodel" \
    "$ROOT/models/needle/memento-router.ndl" \
    "$ROOT/models/needle/needle.model"; do
    test -s "$path"
done

cat >"$ENV_FILE" <<'EOF'
MEMENTO_ADMIN_MASTER_KEY=contract-master
MEMENTO_TOKEN_WORKSPACE=contract-workspace-token
MEMENTO_TOKEN_FLINT=contract-flint-token
MEMENTO_TOKEN_WORK_AGENT=contract-work-token
EOF
chmod 0644 "$ENV_FILE"
chmod 0777 "$STATE"

docker image inspect "$IMAGE" >/dev/null
test "$(docker image inspect "$IMAGE" --format '{{.Config.User}}')" = "65532:65532"
test "$(docker image inspect "$IMAGE" --format '{{json .Config.Entrypoint}}')" = '["/usr/local/bin/memento-go"]'
test "$(docker run --rm "$IMAGE" version)" = "memento-go $VERSION"
if docker run --rm --entrypoint /bin/sh "$IMAGE" -c true >/dev/null 2>&1; then
    echo "Go runtime image unexpectedly contains a shell" >&2
    exit 1
fi
if docker run --rm --entrypoint python "$IMAGE" -V >/dev/null 2>&1; then
    echo "Go runtime image unexpectedly contains Python" >&2
    exit 1
fi
FILES=$(mktemp)
INSPECT_CONTAINER=$(docker create "$IMAGE")
docker export "$INSPECT_CONTAINER" | tar -tf - >"$FILES"
docker rm "$INSPECT_CONTAINER" >/dev/null
for forbidden in usr/bin/git usr/local/bin/memento-serve usr/local/lib/memento/libmemento_ffi.so usr/local/lib/memento/libmemento_needle_ffi.so usr/local/lib/memento/libmemento_sqlite_vector.so; do
    if grep -qx "$forbidden" "$FILES"; then
        echo "Go runtime image unexpectedly contains /${forbidden}" >&2
        exit 1
    fi
done
rm -f "$FILES"

docker run -d --name "$CONTAINER" \
    --read-only \
    --tmpfs /tmp:size=32m,mode=1777 \
    --memory 512m \
    --pids-limit 128 \
    -p 127.0.0.1::8000 \
    -v "$ROOT/deploy/diskstation.config.example.json:/etc/memento/config.json:ro" \
    -v "$ENV_FILE:/run/secrets/memento.env:ro" \
    -v "$STATE:/var/lib/memento" \
    "$IMAGE" \
    --env-file /run/secrets/memento.env \
    --config /etc/memento/config.json serve --http \
    --host 0.0.0.0 --port 8000 --endpoint /mcp >/dev/null

PORT=$(docker port "$CONTAINER" 8000/tcp | awk -F: 'NR==1 {print $NF}')
READY=false
for _ in $(seq 1 180); do
    STATUS=$(docker inspect "$CONTAINER" --format '{{.State.Status}}')
    if test "$STATUS" = exited; then
        docker logs "$CONTAINER" >&2
        exit 1
    fi
    if curl -sS --connect-timeout 1 -o /dev/null "http://127.0.0.1:$PORT/mcp"; then
        READY=true
        break
    fi
    sleep 1
done
test "$READY" = true

UNAUTH=$(curl -sS -o /dev/null -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    --data '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}' \
    "http://127.0.0.1:$PORT/mcp")
test "$UNAUTH" = 401

HTTP_CODE=$(curl -sS -D "$HEADERS" -o "$BODY" -w '%{http_code}' \
    -H 'Authorization: Bearer contract-workspace-token' \
    -H 'Content-Type: application/json' \
    --data '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}' \
    "http://127.0.0.1:$PORT/mcp")
test "$HTTP_CODE" = 200
jq -e --arg version "$VERSION" \
    '.result.serverInfo.name == "memento" and .result.serverInfo.version == $version and .result.protocolVersion == "2025-03-26"' \
    "$BODY" >/dev/null
SESSION=$(awk 'BEGIN{IGNORECASE=1} /^Mcp-Session-Id:/ {gsub("\r", "", $2); print $2}' "$HEADERS")
test -n "$SESSION"

HTTP_CODE=$(curl -sS -o "$BODY" -w '%{http_code}' \
    -H 'Authorization: Bearer contract-workspace-token' \
    -H 'Content-Type: application/json' \
    -H 'Mcp-Protocol-Version: 2025-03-26' \
    -H "Mcp-Session-Id: $SESSION" \
    --data '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"memory_status","arguments":{}}}' \
    "http://127.0.0.1:$PORT/mcp")
test "$HTTP_CODE" = 200
jq -e --arg version "$VERSION" '.result.structuredContent.data.service_version == $version and .result.structuredContent.data.readiness.needle_router.runtime == "go-mmap-subprocess"' "$BODY" >/dev/null

# Exercise a real mapped Needle route. The worker must map the pre-expanded FP32
# sidecar, return the same MCP payload, and exit before the idle memory sample.
HTTP_CODE=$(curl -sS -o "$BODY" -w '%{http_code}' \
    -H 'Authorization: Bearer contract-workspace-token' \
    -H 'Content-Type: application/json' \
    -H 'Mcp-Protocol-Version: 2025-03-26' \
    -H "Mcp-Session-Id: $SESSION" \
    --data '{"jsonrpc":"2.0","id":25,"method":"tools/call","params":{"name":"memory_route","arguments":{"request":"show status","execute":false}}}' \
    "http://127.0.0.1:$PORT/mcp")
test "$HTTP_CODE" = 200
jq -e '.result.structuredContent.status == "success" and .result.structuredContent.data.action.action == "status_field"' "$BODY" >/dev/null
sleep 1
if docker top "$CONTAINER" | grep -q '[m]emento-needle'; then
    echo "short-lived Needle worker remained resident" >&2
    exit 1
fi

# Exercise real pure-Go query embedding through the configured short-lived
# worker. An empty repository still requires model load, tokenization, inference,
# framed response decoding and semantic query execution.
HTTP_CODE=$(curl -sS -o "$BODY" -w '%{http_code}' \
    -H 'Authorization: Bearer contract-workspace-token' \
    -H 'Content-Type: application/json' \
    -H 'Mcp-Protocol-Version: 2025-03-26' \
    -H "Mcp-Session-Id: $SESSION" \
    --data '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"memory_search","arguments":{"query":"semantic contract probe","search_mode":"semantic","limit":3}}}' \
    "http://127.0.0.1:$PORT/mcp")
test "$HTTP_CODE" = 200
jq -e '.result.structuredContent.status == "success" and .result.structuredContent.data.search_mode == "semantic"' "$BODY" >/dev/null
sleep 1
if docker top "$CONTAINER" | grep -q '[m]emento-embed'; then
    echo "short-lived embedding worker remained resident" >&2
    exit 1
fi

GRAPH_CODE=$(curl -sS -o "$BODY" -w '%{http_code}' \
    -H 'Authorization: Bearer contract-workspace-token' \
    "http://127.0.0.1:$PORT/graph")
test "$GRAPH_CODE" = 200
grep -q '<!doctype html>' "$BODY"

docker inspect "$CONTAINER" --format '{{.State.OOMKilled}} {{.RestartCount}}' | grep -q '^false 0$'
DAEMON_PID=$(docker inspect "$CONTAINER" --format '{{.State.Pid}}')
DAEMON_RSS_KIB=$(awk '/^VmRSS:/ {print $2}' "/proc/$DAEMON_PID/status")
MEMORY_BYTES=$(docker stats --no-stream --format '{{.MemUsage}}' "$CONTAINER" | awk '{print $1}')
python3 - "$MEMORY_BYTES" "$DAEMON_RSS_KIB" <<'PY'
import re, sys
value=sys.argv[1]
rss_kib=int(sys.argv[2])
match=re.fullmatch(r'([0-9.]+)([KMG]iB)',value)
if not match:
    raise SystemExit(f'unrecognised memory value: {value}')
scale={'KiB':1/1024,'MiB':1,'GiB':1024}[match.group(2)]
mib=float(match.group(1))*scale
rss_mib=rss_kib/1024
if mib > 500:
    raise SystemExit(f'idle container memory {mib:.1f} MiB exceeds 500 MiB budget')
if rss_mib > 80:
    raise SystemExit(f'idle daemon RSS {rss_mib:.1f} MiB exceeds 80 MiB budget')
print(f'idle container memory: {mib:.1f} MiB; daemon RSS: {rss_mib:.1f} MiB')
PY
docker rm -f "$CONTAINER" >/dev/null

# Prove the accepted on-disk formats remain usable in both directions. The old
# image initializes the volume, v1 opens it without migration, then the old
# image opens it again and observes the same Git/index revisions.
if test -n "$ROLLBACK_IMAGE"; then
    OLD_ENV=(
        -e MEMENTO_ADMIN_MASTER_KEY=contract-master
        -e MEMENTO_TOKEN_SANDBOX_BOOTSTRAP=contract-sandbox
        -e MEMENTO_TOKEN_WORK_AGENT_BOOTSTRAP=contract-work
        -v "$ROOT/examples/config.v1.json:/etc/memento/config.json:ro"
        -v "$STATE:/var/lib/memento"
    )
    docker run --rm "${OLD_ENV[@]}" "$ROLLBACK_IMAGE" --config /etc/memento/config.json status >"$BODY.before"
    docker run --rm "${OLD_ENV[@]}" "$IMAGE" --config /etc/memento/config.json status >"$BODY.go"
    docker run --rm "${OLD_ENV[@]}" "$ROLLBACK_IMAGE" --config /etc/memento/config.json status >"$BODY.after"
    jq -e -n --arg version "$VERSION" \
        --slurpfile before "$BODY.before" --slurpfile go "$BODY.go" --slurpfile after "$BODY.after" \
        '$before[0].repo_revision == $go[0].repo_revision and
         $go[0].repo_revision == $after[0].repo_revision and
         $before[0].index_revision == $go[0].index_revision and
         $go[0].index_revision == $after[0].index_revision and
         $go[0].service_version == $version' >/dev/null
    rm -f "$BODY.before" "$BODY.go" "$BODY.after"
fi

printf 'Go container contract passed: image=%s version=%s\n' "$IMAGE" "$VERSION"
