# Visual memory debugger implementation plan

**Status:** in progress
**Decision:** [ADR 0011](decisions/0011-embed-a-gated-visual-memory-debugger.md)

The visual debugger is a built-in `/graph` view for understanding how Memento creates, links and maintains shared memory. The Plan sidebar tracks the active phase; this document holds the API, rendering and release details that should survive the session.

## Scope And Invariants

The global setting is `observability.graph_explorer.enabled`, default `false`. Every graph route returns `404` while it is off. When enabled, the view is unauthenticated and intended for a trusted development network. MCP remains on the same port with its existing authentication.

The graph may read Git knowledge, operation/proposal records and derived indexes. Its only write-like action is rebuilding embeddings through the existing short-lived worker. It does not create, patch, rename, review or apply concepts.

Explicit Markdown links are relationship data. Semantic similarity is a derived overlay. APIs and exports omit bearer tokens, token environment names, raw embedding vectors and asset bodies. Overview responses contain no Markdown body; detail previews are sanitised and bounded.

The browser application uses committed Three.js and Preact ES modules. Bun verifies and refreshes those files and runs browser checks. Release assets remain native modules rather than a committed bundle.

## Boundary And Configuration

`GraphExplorerConfig` contains these bounded settings:

```text
route_prefix                /graph
direct_node_limit           2000
overview_cluster_limit       500
expansion_node_limit        2000
edge_limit                 12000
preview_chars               4000
semantic_neighbours           12
export_node_limit           2000
refresh_max_paths          10000
```

The prefix is an absolute non-root path, has no trailing slash and cannot overlap the MCP endpoint. Enabling the feature emits a startup warning.

The pinned uMCP server exposes a small auxiliary HTTP hook after request framing, header and Origin checks. Memento uses it for graph routes and leaves MCP requests on the established authentication path.

Enabled routes:

```text
GET  /graph
GET  /graph/assets/<file>
GET  /graph/api/v1/status
GET  /graph/api/v1/overview
GET  /graph/api/v1/clusters/<cluster-id>
GET  /graph/api/v1/memories/<concept-id>
GET  /graph/api/v1/neighbourhood/<concept-id>
GET  /graph/api/v1/embeddings/status
POST /graph/api/v1/embeddings/refresh
POST /graph/api/v1/export/json
POST /graph/api/v1/export/svg
```

Static paths are contained below the packaged graph directory. Unsupported methods return `405`; malformed and oversized bodies return bounded errors.

## Backend Graph API

Snapshots combine set-based reads from:

* `derived.sqlite` for concepts, explicit links, graph metrics, embeddings and revision state;
* `control.sqlite` for proposal and operation provenance;
* Markdown files for byte size, updater and the selected concept's preview;
* `.assets` metadata for retained versions and byte totals.

The overview declares repository, index and embedding revisions. Up to 2,000 concepts it returns direct nodes. Larger repositories return namespace/community aggregates, counts, byte totals, anomaly summaries and inter-cluster explicit edges. Cluster and neighbourhood requests return bounded working sets.

A memory node contains its stable ID, bundle path, title, type, status, tags, namespace, timestamps, updater, sizes, explicit degrees, broken-link count, proposal counts, embedding state, cluster ID, coarse coordinates and diagnostic IDs.

Edges carry a type, weight, explanation and a `canonical` flag. Explicit edges include resolution and revision information. Semantic edges include model and embedding revisions but no vector.

The detail endpoint adds a short Markdown preview, inbound and outbound explicit links, semantic neighbours, asset manifests, proposal summaries and layout explanations. Ordering is stable, responses have node/edge caps, and snapshots report revision lag instead of mixing state silently.

### Layout

The server partitions the explicit-link graph by namespace and stable communities, then creates hash-seeded coarse `x`, `y` and `z` coordinates. The same revision and settings produce the same result regardless of SQL/input ordering. Expansion begins at the parent centroid.

Browser force defaults rank explicit links above semantic similarity, namespace, tags, type and provenance. Every force is adjustable. Missing embeddings remove only the semantic force.

### Diagnostics

Each diagnostic has a stable ID, severity, rule, concept IDs, explanation, measured values and thresholds. There is no combined mystery score.

The initial rules cover:

* orphan and broken-link state;
* high degree and isolated namespace/community groups;
* repository/index lag and embedding failure or staleness;
* pending proposal state;
* exact duplicates and compatible-embedding near-duplicates;
* Markdown/asset size outliers;
* tag drift and namespace outliers.

### Embedding Refresh

Selected and visible refreshes accept bounded concept IDs. Full refresh requires `confirm_full=true`. One coordinator coalesces requests into the existing worker, which batches concepts, maps GTE weights, writes derived rows and exits. Status reports queued scope, running/pending state, repository revision and the last error.

## Browser Application

Files under `src/memento/graph_debug/static/` are browser-native modules. `vendor/manifest.json` records versions, sources, licences and SHA-256 digests. `bun tools/vendor_graph_libraries.ts --check` verifies them offline.

Three.js provides:

* instanced memory/cluster meshes;
* curved typed edges and direction cues;
* depth-aware orbit, pan, zoom and focus transitions;
* GPU/raycast picking;
* worker-based refinement of the visible set;
* expansion from a cluster centroid;
* label and edge level of detail;
* mouse, keyboard, trackpad and tablet touch input.

Preact provides:

* text, namespace, type, tag, principal and status filters;
* relationship layer toggles and force controls;
* size and colour selectors;
* anomaly filters;
* the provenance/detail inspector and asset/proposal satellites;
* embedding refresh controls;
* export controls;
* node/edge, fetch, layout, render and frame-pressure timings;
* a persistent warning about unauthenticated access.

Current desktop Chromium, Firefox and Safari are the browser targets. WebGL2 failure produces a clear message. Phone layout is outside this release.

## Exports And Validation

PNG comes from the current WebGL canvas. SVG contains a bounded selected neighbourhood with curved typed edges, labels and legend. JSON contains the filtered graph, positions, settings, revisions and diagnostics. Export tests scan for secrets, raw vectors, executable SVG/HTML and oversized output.

Generated fixtures cover 500, 2,000 and 10,000 concepts, including chains, stars, dense groups, isolates, broken links, proposals, assets, stale embeddings, duplicates and size outliers.

Playwright runs on Chromium, Firefox and WebKit. It covers disabled/enabled routing, overview, selection, camera input, keyboard and tablet touch, filters, layer and force changes, expansion, inspector, diagnostics, refresh and exports. Light and dark screenshots catch visual drift.

Measured fields include response bytes, server query time, first useful paint, layout start/stability, selection feedback, expansion start, p50/p05 frame rate, dropped frames and browser heap. The target is:

```text
overview over LAN       <= 5 s
2,000 visible nodes     >= 30 fps
selection feedback      < 100 ms
expansion begins        < 250 ms
```

Ten expansion/collapse cycles must not show unbounded heap growth.

## Packaging And Release

The wheel, source archive and container include application modules, vendor files, the manifest and licences. Packaging tests load every asset through `importlib.resources`; runtime needs no network access.

Before release:

```text
make check
make coverage
make build-wheel
make install-wheel
make diff-check
```

The release pipeline also runs Python 3.12-3.14, the Rust workspace, Needle's 360-case parity set, GTE parity, browser/vendor tests, amd64/arm64 image builds and the Westmere no-AVX smoke.

The new candidate is pulled through Portainer and applied to stack 111 without moving the previous tag. DiskStation validation checks unauthenticated `/graph`, authenticated MCP, desktop Chromium, tablet touch, response/render timings, RSS, restart behaviour and selected/visible embedding refresh. The graph stays enabled only on the agreed trusted LAN.

## Deferred

Revision playback and animated diffs will derive bounded snapshots from Git history and operation records. Split comparison between two layer/force configurations and standalone interactive export follow later.
