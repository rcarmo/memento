# Visual memory debugger implementation plan

**Status:** current-state debugger implemented and deployed on the trusted-LAN DiskStation; revision playback deferred
**Decision:** [ADR 0011](decisions/0011-embed-a-gated-visual-memory-debugger.md)
**Current deployment:** Go v1.0.5 on the trusted-LAN DiskStation
**Latest graph-specific acceptance:** [Diagnostics/UI audit and local regression results](evidence/diagnostics-ui-audit-2026-09-21.md) (combined fix awaiting deployment)

The visual debugger is a built-in `/graph` view for understanding how Memento creates, links and maintains shared memory. This document records the implemented API, rendering, validation and release details, then collects the deferred history features at the end.

## Scope And Invariants

The global setting is `observability.graph_explorer.enabled`, default `false`. Every graph route returns `404` while it is off. When enabled, the view is unauthenticated and intended for a trusted development network. MCP remains on the same port with its existing authentication.

The graph may read Git knowledge, operation/proposal records and derived indexes. Its only write-like action is rebuilding embeddings through the existing short-lived worker. It does not create, patch, rename, review or apply concepts.

Explicit Markdown links are relationship data. Semantic similarity is a derived overlay. APIs and exports omit bearer tokens, token environment names, raw embedding vectors and asset bodies. Overview responses contain no Markdown body; detail previews are sanitised and bounded.

The browser application uses committed Three.js and Preact ES modules. Their manifest records pinned source URLs and digests; Go tests verify every embedded asset and licence file. Release assets remain native modules rather than a generated bundle.

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

Browser force defaults rank explicit links above semantic similarity, namespace, tags, type and provenance. Semantic similarity is default-off; its toggle controls both global and selected-node edges above the cosine threshold. Green segmented arcs and selected-node tubes encode similarity. Semantic opacity defaults to 60% and changes only their materials, not explicit links or force strength. Shared namespace, type, tag and source-reference relationships supply bounded, interleaved link chains; provenance equality uses hashed source references rather than exposing their text. Controls with no matching visible relationships are disabled. Aggregate edges retain mean similarity when bounded embeddings are available. Missing embeddings remove only the semantic layer.

Cluster names are projected over the graph. Force controls expose strength, repulsion and preferred distance, reheating a cancellable worker from current positions. The worker yields between batches and stops after sustained low movement rather than a fixed iteration count. Details deduplicate neighbouring references. External URLs are classified separately from broken repository links. Show Trash adds the permission-scoped Trash group; deletion itself stays on authenticated MCP tools.

### Diagnostics

Each diagnostic has a stable ID, severity, rule, concept IDs, explanation, measured values and thresholds. The sidebar names its scope: selected node, selected cluster or current filtered view. Entries are deduplicated by ID and contain node-navigation buttons. Findings spanning a larger snapshot label their clipped target list. Detail, cluster and neighbourhood responses supply fresh scoped diagnostics.

Explicit orphan/degree and broken-link calculations use the permitted explicit link set before display edge limits. Accepted asset links and external URLs are excluded from broken/orphan connectivity; semantic overlays do not establish explicit connectivity. Truncated displays are marked. Failed relationship requests show unavailable data with retry, not empty counts. Successful node detail remains available when only neighbourhood loading fails. Asset versions are grouped by kind and newest semantic version first, before applying the summary limit. Sidebar controls and inspector content are left aligned.

The initial rules cover:

* orphan and broken-link state;
* high degree and isolated namespace/community groups;
* repository/index lag and embedding failure or staleness; transient missing/stale embedding diagnostics remain visible in filters/status but do not draw per-node gold action rings;
* pending proposal state;
* exact duplicates and compatible-embedding near-duplicates;
* Markdown/asset size outliers;
* tag drift and namespace outliers.

### Embedding Refresh

Selected and visible refreshes accept bounded concept IDs. Full refresh requires `confirm_full=true`. The coordinator places these paths at the front of the same progressive worker used for automatic generation; manual requests do not bypass startup, interactive-idle, sampled-CPU or pacing gates. The DiskStation profile processes one path per low-priority single-threaded subprocess. Status reports queued scope, running/pending state, pause reason, current path, completed count, repository revision and the last error.

## Browser Application

Files under `internal/service/graph_static/` are embedded browser-native modules. `vendor/manifest.json` records versions, sources, licences and SHA-256 digests; Go embedding tests verify that every required asset is present and non-empty.

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

The Playwright suite defines Chromium, Firefox, WebKit and tablet projects. Fixture and screenshot cases cover disabled/enabled routing, overview, selection, camera input, keyboard and tablet touch, filters, layer and force changes, expansion, inspector, diagnostics, refresh and exports; browser-specific skips keep unsupported rendering paths explicit. Light and dark screenshots catch visual drift.

Measured fields include response bytes, server query time, first useful paint, layout start/stability, selection feedback, expansion start, p50/p05 frame rate, dropped frames and browser heap. The target is:

```text
overview over LAN       <= 5 s
2,000 visible nodes     >= 30 fps
selection feedback      < 100 ms
expansion begins        < 250 ms
```

Ten expansion/collapse cycles must not show unbounded heap growth.

## Packaging And Release

The static Go binary embeds the application modules, vendor files, manifest and licences; runtime needs no network access for the debugger. Go tests load every embedded asset and verify the checked-in manifest.

Before release:

```text
make quality
make performance
make model-test
make corpus-test
make release-check MEMENTO_VERSION=1.0.0 SOURCE_DATE_EPOCH=0
make go-container-contract MEMENTO_VERSION=1.0.0
```

The release pipeline runs the complete Go audit, Needle's 360-case parity set on ARM64, GTE/Needle real-model and allocation gates, native amd64/arm64 image builds and the Westmere no-AVX smoke. Historical Playwright screenshots and interaction results remain under `docs/evidence/graph/`; current static serving and API contracts are exercised through Go tests, while a target browser pass remains an operator release check.

A tagged release is pulled through Portainer without moving the previous tag. Operator validation checks unauthenticated `/graph`, authenticated MCP, response/render timings, RSS, restart behaviour and selected/visible embedding refresh. The graph stays enabled only on the agreed trusted LAN.

## Deferred

Revision playback and animated diffs will derive bounded snapshots from Git history and operation records. Split comparison between two layer/force configurations and standalone interactive export follow later.

## Relationship to access management

The graph debugger remains a separate trusted-network diagnostic surface. It reads dynamic principal policies for simulation, but simulation is not authorisation. Real principal changes happen only through authenticated `/admin` or admin-only `access_*` calls.
