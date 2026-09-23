# Memento 1.0.10 graph diagnostics

Memento 1.0.10 replaces the graph's uniform anomaly halos with rule- and severity-aware markers. Production was updated on 23 September 2026; the repository and embedding data remained unchanged.

## Release

- Source commit `a3367562ec7461ee7ac04209d6e91110a17ed548`; [exact-commit CI 35909288835](https://github.com/rcarmo/memento/actions/runs/35909288835) passed, including repeated Chromium/WebKit audits and amd64/ARM64 jobs.
- [Release workflow 35912010829](https://github.com/rcarmo/memento/actions/runs/35912010829) passed runtime, architecture, baseline CPU and publishing jobs. [Release v1.0.10](https://github.com/rcarmo/memento/releases/tag/v1.0.10) includes an SPDX SBOM.
- OCI digest: `sha256:34ba60106ec745c7cc9eee340abbe52a6e345ad409a237ba796af8c29e209857`.
- Local gates passed: `make quality`, `make audit`, `make performance model-test corpus-test`, `make release-check MEMENTO_VERSION=1.0.10 SOURCE_DATE_EPOCH=0`, `make go-container-contract MEMENTO_VERSION=1.0.10`, and `make ui-test UI_REPEAT=2` (18/18 checks in each Chromium and WebKit run).

## Production cutover

Portainer endpoint 18, stack 111, retained the same Compose configuration apart from the image digest. The v1.0.9 Compose and image remained available for rollback. Image pull and stack update requests timed out at the client; neither was repeated without reconciliation. A bounded pull completed with HTTP 200 and the exact released digest. After the update timeout, Portainer's stored Compose and Docker inspection confirmed that the replacement had started from the pinned v1.0.10 image. Its four config/secret/state/model mounts, port, UID/GID, read-only root, security settings and 512 MiB memory limit were retained.

The replacement container `8cdda97e1f5de774fcdc857f70896d6c88eacbc8b566b132a8497e97208b88a6` became healthy. Docker reported `OOMKilled=false`, an empty state error and zero automatic restarts. A successful metrics scrape reported service/index ready, index not stale, 260 ready embeddings, all other embedding states zero and an idle worker. Before and after the cutover, repository, index and embedding revisions matched `761313cec2c6a9e60310ed11d3bb24cca300e323`; graph counts remained 258 visible memories, 522 explicit edges, zero broken edges, 24 orphans and 1,500 semantic edges. The sorted semantic-edge JSON digest was unchanged at `54bbc79eab44034a1ec998342d70b2c0d718a4389b33df24019340f313419ea5` for this capture format. This is a pre/post comparison, not a SQLite integrity check.

The production graph served byte-for-byte copies of the committed `app.js`, `graph-scene.js`, `diagnostics.js` and `app.css`. A Chromium check against the live graph returned HTTP 200 with no page errors: 21 warning markers by default, 24 orphan markers when enabled and 18 informational markers in all-severities mode. The diagnostic panel still lists every finding independent of the canvas marker filter.

Captured metrics, graph snapshots, Compose variants, pull response and browser-check summary: `/workspace/tmp/memento-marker-deploy/`.
