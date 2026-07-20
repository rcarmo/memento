# Visual memory debugger implementation plan

**Status:** planned
**Decision:** [ADR 0011](decisions/0011-embed-a-gated-visual-memory-debugger.md)
**Target:** built-in `/graph` surface, enabled on the trusted DiskStation LAN
**Toolchain:** Python, browser-native ES modules, vendored Three.js and Preact, Bun housekeeping only

This plan is the delivery ledger for the 2.5D memory graph debugger. It converts the accepted refinement into implementation slices with explicit contracts and gates. The feature is not complete until the release image is deployed through Portainer, the global setting is enabled on DiskStation and the real-browser performance checks pass.

## Outcome

A human opens `/graph` and receives a polished 2.5D overview of current Memento knowledge and management state. The overview explains how memories are grouped, linked, sized, created and maintained. Humans can expand clusters, compare relationship layers, inspect provenance and anomalies, refresh derived embeddings, and export bounded views without acquiring an MCP token or mutating Git/control state.

The feature is disabled by default. When disabled, every graph route returns `404`. When enabled, it is unauthenticated and therefore limited to trusted development networks.

## Non-goals for the MVP

* Editing, creating, patching, renaming, reviewing or applying memories
* Treating semantic similarity as canonical relationship state
* Shipping raw vectors or asset bodies to the browser
* Rendering all 10,000 concepts and all inferred edges simultaneously
* Phone-optimised interaction
* A fallback renderer for browsers without WebGL2
* Standalone interactive HTML export
* Split-screen relationship comparison
* Revision-history playback or animated diffs
* npm application dependencies, JSX compilation or a JavaScript bundler

Revision playback is the first named follow-up feature.

## Phase 0 -- checkpoint current optimisation work

Before graph changes alter the same service/index/runtime files:

- [ ] Complete the interrupted 360-case Needle benchmark for the constrained-vocabulary and allocation changes.
- [ ] Require 360/360 valid tool decisions and scalar/SIMD parity.
- [ ] Run GTE parity and the full Python/Rust gate.
- [ ] Benchmark status/search changes independently and keep them only if they reduce measured latency without weakening corruption recovery.
- [ ] Commit the optimisation patch separately from graph work.

**Exit:** clean working tree and one reviewed optimisation commit.

## Phase 1 -- configuration and same-port HTTP boundary

### Configuration

Add immutable configuration models:

```json
{
  "observability": {
    "graph_explorer": {
      "enabled": false,
      "route_prefix": "/graph",
      "direct_node_limit": 2000,
      "overview_cluster_limit": 500,
      "expansion_node_limit": 2000,
      "edge_limit": 12000,
      "preview_chars": 4000,
      "semantic_neighbours": 12,
      "export_node_limit": 2000,
      "refresh_max_paths": 2000
    }
  }
}
```

Validation rules:

* `route_prefix` is absolute, has no trailing slash and cannot overlap the configured MCP endpoint.
* All bounds have conservative maxima.
* The global setting defaults to disabled.
* Startup emits one structured warning when enabled.

### HTTP integration

Inspect the pinned uMCP HTTP implementation before choosing the adapter:

1. Prefer an official auxiliary-route or request-handler hook if it preserves uMCP framing, Origin handling, shutdown and request limits.
2. Otherwise add one dependency-free asyncio same-port multiplexer that dispatches `/graph...` to the debugger and `/mcp...` to the unchanged uMCP handler.
3. Do not open a second public port.
4. Preserve graceful drain and cancellation.

Required routes when enabled:

```text
GET  /graph
GET  /graph/assets/<vendored-or-app-file>
GET  /graph/api/v1/status
GET  /graph/api/v1/overview
GET  /graph/api/v1/clusters/<cluster-id>
GET  /graph/api/v1/memories/<concept-id>
GET  /graph/api/v1/neighbourhood/<concept-id>
POST /graph/api/v1/embeddings/refresh
POST /graph/api/v1/export/json
POST /graph/api/v1/export/svg
```

PNG export is performed from the browser canvas. Unsupported methods return `405`; unknown routes return `404`. When disabled, known graph routes also return `404`, including static files and APIs.

### Tests

- [ ] Disabled route matrix returns `404` and no identifying body.
- [ ] Enabled UI and API are unauthenticated.
- [ ] MCP without a token remains `401`; enabling the graph changes no MCP behaviour.
- [ ] Prefix collision and malformed path tests.
- [ ] Request body, query, concurrency and response size bounds.
- [ ] Shutdown while graph requests are active.

**Exit:** a gated same-port static test page and status endpoint pass without changing MCP regressions.

## Phase 2 -- graph snapshot and API contracts

Create `src/memento/graph_debug/` with narrowly separated modules:

```text
config/API models     models.py
snapshot queries      snapshot.py
aggregation/layout    layout.py
diagnostics           diagnostics.py
semantic overlay      semantic.py
embedding jobs        refresh.py
HTTP/static handler   http.py
exports                export.py
```

### Overview response

```json
{
  "schema_version": 1,
  "repository_revision": "<sha>",
  "index_revision": "<sha>",
  "embedding_revision": "<sha-or-null>",
  "generated_at": "<utc>",
  "mode": "direct | aggregated",
  "bounds": {"nodes": 2000, "edges": 12000},
  "metrics": {
    "memory_count": 10000,
    "markdown_bytes": 0,
    "asset_bytes": 0,
    "explicit_edges": 0,
    "anomalies": 0
  },
  "nodes": [],
  "edges": [],
  "clusters": [],
  "diagnostics": [],
  "layout": {"seed": "<revision>", "version": "v1"}
}
```

### Memory node

Each direct memory node contains only bounded debugging metadata:

```text
id, path, title, type, status, tags, namespace
created_at, updated_at, updated_by
repository_revision, index_revision
markdown_bytes, asset_bytes, combined_bytes
explicit_in_degree, explicit_out_degree
broken_link_count, orphan
proposal_count, pending_proposal_count
embedding_state, semantic_density
cluster_id, coarse_position{x,y,z}
anomaly_ids[]
```

No body appears in overview responses.

### Typed edges

```text
explicit_outbound
explicit_inbound (only when requested as a view)
semantic_similarity
shared_tag
shared_namespace
shared_type
shared_provenance
asset_attachment
proposal_origin
```

Every edge declares `canonical: true|false`, weight, explanation and revision/model metadata where applicable. Only explicit links are canonical knowledge relationships.

### Detail endpoint

The detail response adds:

* bounded sanitised Markdown preview;
* full explicit inbound/outbound edge lists within limits;
* current semantic neighbours and scores without vectors;
* asset manifests and byte counts without payloads;
* proposal summaries and operation provenance;
* layout-force explanations;
* diagnostic rule details.

### Snapshot strategy

Use read-only, bounded queries:

* `derived.sqlite`: concepts, links, graph metrics, embeddings and index state;
* `control.sqlite`: proposals and operation/provenance summaries;
* current Markdown files: byte size and bounded preview only;
* asset metadata/manifests: retained byte totals and satellite summaries;
* Git: current revision from indexed/runtime state, not one subprocess per node.

Create indexes only where query plans prove they are needed. Avoid N+1 filesystem and SQL access. Snapshot generation must use a coherent repository/index revision and declare staleness rather than silently mixing revisions.

**Tests:** schema validation, query-count ceilings, stale revision behaviour, deterministic ordering, no path escape, bounded previews, no credentials/vectors/assets, 10,000-node response limits.

**Exit:** fixture-backed overview/detail/neighbourhood APIs are deterministic and bounded.

## Phase 3 -- aggregation and deterministic 2.5D layout

### Direct versus aggregated mode

* `memory_count <= direct_node_limit`: return direct nodes.
* Larger repositories: return aggregate nodes grouped initially by namespace, then deterministic graph community inside large namespaces.
* Aggregates include counts, byte totals, type/tag distribution, anomaly counts, explicit inter-cluster edges and deterministic centroids.
* Expansion pages are cursor-based and bounded.

### Community and coarse coordinates

Use deterministic algorithms with no model dependency:

1. Build the authorised explicit-link graph.
2. Partition by namespace and deterministic community detection for oversized groups.
3. Hash repository revision + layout version + stable cluster/node ID for the seed.
4. Place clusters using explicit inter-cluster weights.
5. Assign stable coarse `x,y,z` positions; `z` encodes a bounded debug dimension such as recency or hierarchy, selectable by the client.
6. Send force weights and rest-distance hints, not a browser-specific simulation state.

Semantic proximity modifies only a separately labelled overlay and optional force hints. It never alters stored link metrics.

### Force defaults

```text
explicit links        strongest
semantic similarity   medium, off if unavailable/stale
shared tag            weak
namespace             medium cluster containment
type                   weak
provenance             weak
collision              based on logarithmic node radius
```

All weights are client-adjustable and resettable.

### Determinism tests

- [ ] Same data/revision/settings produce identical clusters and coarse coordinates.
- [ ] Input ordering does not change output.
- [ ] Cluster expansion starts at the parent centroid.
- [ ] Missing embeddings do not change explicit relationships or anomaly results.
- [ ] Layout stays finite for isolates, stars, chains, disconnected graphs and empty repositories.

**Exit:** deterministic 500/2,000/10,000 fixtures and expansion contracts pass.

## Phase 4 -- explainable diagnostics

Implement rules with stable IDs, severity, measured value, threshold and explanation.

### Structural

* `orphan`: zero explicit inbound and outbound links
* `broken_links`: count above zero
* `high_degree`: configurable percentile/absolute threshold
* `isolated_cluster`: no explicit edge to another cluster

### Lifecycle and management

* repository/index revision mismatch
* stale/missing/incompatible embeddings
* pending or failed proposals/operations
* asset manifest or retained-version problems
* old update age threshold

### Content shape

* combined/Markdown/asset size outlier relative to namespace median and MAD
* exact content duplicate by hash
* near-duplicate from current compatible embeddings, clearly labelled derived
* tag drift relative to cluster distribution
* namespace outlier relative to explicit neighbours and metadata

Do not collapse these into one unexplained score. The UI may sort by severity but must show the rule and values.

**Exit:** adversarial fixtures prove no semantic diagnostic is described as a canonical relationship defect.

## Phase 5 -- derived embedding maintenance

Add one coalescing graph-debug refresh coordinator over the existing `SemanticEmbeddingRefreshWorker` and subprocess client.

Scopes:

```text
selected concept
visible concept IDs / expanded neighbourhood
full repository (confirmation token required)
```

Contract:

* one active job per runtime;
* bounded path count for selected/visible jobs;
* full refresh requires current repository revision plus explicit `confirm_full=true`;
* jobs coalesce duplicate paths and expose queued/running/succeeded/failed/cancelled counts;
* process uses short-lived mmap-backed `memento-embed` batches;
* no Git or control-plane mutation;
* cancellation prevents new batches and allows current process cleanup;
* page reload can poll job status.

The endpoint returns `409` for incompatible concurrent full jobs and bounded errors for unavailable semantic configuration.

**Exit:** selected, visible and confirmed-full refresh tests pass, including process exit/RSS and repository revision changes during a job.

## Phase 6 -- vendored browser application

### File layout

```text
src/memento/graph_debug/static/
  index.html
  app.css
  app.js
  api.js
  state.js
  graph-scene.js
  layout-worker.js
  inspector.js
  controls.js
  diagnostics.js
  export.js
  vendor/
    three.module.min.js
    preact.module.js
    preact-hooks.module.js
    LICENSES.md
    manifest.json
```

Do not use JSX. Use `h()` or tagged helpers for Preact components.

### Vendoring

Add a Bun script such as `tools/vendor_graph_libraries.ts` that:

* downloads pinned exact versions from reviewed upstream release artefacts;
* checks SHA-256 before replacing files;
* records source URL, version, licence and digest in `vendor/manifest.json`;
* supports `--check` without network access for CI;
* never runs package lifecycle scripts.

Include licences in attribution and the release image.

### WebGL scene

Use Three.js WebGL2 with:

* instanced node meshes grouped by shape/material;
* logarithmic radius and bounded min/max size;
* quadratic/cubic curved edge geometry with typed style and direction;
* depth-aware camera, fog/contrast and focus transitions;
* GPU or raycast picking with a bounded spatial index;
* deterministic visible-node force refinement in a Web Worker;
* cluster expansion animation from parent centroid;
* label sprites/HTML overlay with strict level of detail;
* touch orbit/pan/pinch and accessible keyboard focus commands;
* light/dark palette from the project diagram palette, transparent where practical.

### Preact UI

Controls include:

* search and namespace/type/tag/principal/status filters;
* layer toggles;
* force sliders and reset;
* size and colour metric selectors;
* anomaly filters/severity threshold;
* timeline placeholder marked as a future feature, not an inactive control;
* selected-node inspector;
* embedding refresh controls/progress;
* export controls;
* performance/debug panel;
* prominent enabled-debug-surface warning.

### Inspector

Render Markdown preview as text/sanitised bounded markup. Never execute embedded HTML, scripts or event attributes. Asset/proposal satellites expand without fetching payload bodies.

**Exit:** local UI operates against deterministic fixtures and the live API without a bundler.

## Phase 7 -- exports

### PNG

Browser exports the current WebGL canvas at bounded resolution with current camera, legend and optional inspector caption. Strip credentials and API state.

### SVG

Server or browser emits a bounded selected-neighbourhood SVG:

* stable coordinates;
* curved typed edges;
* labels and legend;
* selected metric/colour modes;
* no foreignObject or executable content;
* node cap enforced.

### JSON

Export the current filtered graph, coordinates, typed edges, revisions, diagnostics and active settings. Explicitly omit preview bodies by default; include bounded preview only behind a UI checkbox with a warning. Never export raw vectors, tokens or asset payloads.

**Tests:** schema, deterministic output, XSS payloads, maximum sizes and no-secret/no-vector assertions.

**Exit:** PNG, SVG and JSON exports work for direct and expanded aggregated views.

## Phase 8 -- browser and scale validation

### Playwright matrix

Use the workspace Playwright skill/tooling and pin browser versions for:

* Chromium
* Firefox
* WebKit

Test:

* disabled and enabled routes;
* initial overview;
* mouse selection/orbit/pan/zoom;
* keyboard navigation and focus;
* tablet touch pinch/pan/select;
* filters, layers, force controls and reset;
* cluster expansion/collapse;
* inspector and satellites;
* anomaly explanations;
* embedding refresh progress;
* PNG/SVG/JSON export;
* light and dark mode snapshots;
* WebGL2 unsupported message.

### Synthetic fixtures

Generate deterministic repositories/indexes with:

* 500 memories: direct overview
* 2,000 memories: direct-render ceiling
* 10,000 memories: aggregated overview and expansion

Include stars, chains, dense communities, isolates, broken links, assets, proposals, stale embeddings, duplicates and size outliers.

### Performance capture

Machine-readable report fields:

```text
first_response_ms
first_useful_paint_ms
node_count / edge_count
layout_start_ms / layout_stable_ms
selection_feedback_ms
cluster_expansion_start_ms
fps_p50 / fps_p05
dropped_frame_ratio
browser_heap_peak_bytes
response_bytes
server_query_ms
```

Acceptance:

* overview <=5 s over LAN-equivalent conditions;
* >=30 fps at 2,000 visible nodes and bounded edges;
* selection <100 ms;
* expansion starts <250 ms;
* no unbounded heap growth across ten expansion/collapse cycles.

**Exit:** all browsers pass functional tests; Chromium performance fixture meets budgets; Firefox/WebKit stay usable and correct.

## Phase 9 -- packaging, release and DiskStation deployment

### Packaging

- [ ] Include static modules, vendor manifest and licences in wheel/sdist and container.
- [ ] Add offline package tests proving assets are present.
- [ ] Validate OCI labels/version and no network dependency at runtime.
- [ ] Keep feature disabled in default and Internet-facing examples.
- [ ] Add a trusted-LAN DiskStation example with the setting enabled and warning text.

### Release gates

Run:

```text
make check
make coverage
make build-wheel
make install-wheel
make diff-check
```

Then require:

* Python 3.12--3.14 release matrix;
* Rust workspace and 360/360 Needle parity;
* GTE parity;
* amd64/arm64 images;
* Westmere no-AVX GTE/Needle smoke;
* static/vendor integrity check;
* Playwright functional and scale reports.

### DiskStation

1. Publish a new release candidate without moving old tags.
2. Pull by version/digest through Portainer.
3. Update stack 111 while rc.4 remains available for rollback.
4. Enable `observability.graph_explorer.enabled=true` in the trusted-LAN config.
5. Verify `/graph` externally without credentials and MCP still requires bearer auth.
6. Run a real desktop Chromium smoke and tablet-touch smoke.
7. Capture first-paint, expansion, fps, RSS and server latency.
8. Exercise selected and visible embedding refresh; verify GTE worker exits and RAM returns.
9. Restart the container and verify graph/API, repository/index revisions and MCP.
10. Leave the graph enabled only because the deployment is on the agreed trusted LAN.

**Exit:** live DiskStation result meets budgets, remains under the container memory limit, has no OOM/restarts and retains rollback instructions.

## Definition of done

The MVP is complete only when all of these are true:

- [ ] Global flag defaults off and all disabled routes return `404`.
- [ ] Enabled surface is unauthenticated and visibly warns about trusted-network use.
- [ ] MCP authentication and authorisation are unchanged.
- [ ] Overview, aggregation, expansion, selection and deterministic refinement work.
- [ ] Memories are primary nodes; assets/proposals expand as satellites.
- [ ] Explicit and inferred layers are visually and semantically distinct.
- [ ] Size, colour, filters, layers and force controls match the accepted refinement.
- [ ] Provenance, management state, bounded preview and layout explanations appear in the inspector.
- [ ] Structural, lifecycle and content-shape diagnostics are explainable.
- [ ] Selected, visible and confirmed-full embedding refresh works without canonical/control mutation.
- [ ] PNG, bounded SVG and JSON export pass no-secret/no-vector tests.
- [ ] Chromium, Firefox, WebKit and tablet-touch tests pass.
- [ ] 500/2,000/10,000 fixtures pass correctness and response bounds.
- [ ] Five-second overview, 30 fps, 100 ms selection and 250 ms expansion budgets pass.
- [ ] Existing Python/Rust/package/container/no-AVX gates remain green.
- [ ] Release image is published and deployed through Portainer to DiskStation.
- [ ] Real DiskStation browser smoke, RSS, restart and derived-refresh checks pass.
- [ ] Graph remains enabled on the trusted LAN as requested.

## Follow-up roadmap

### Revision playback and animated diffs

Use Git commits plus control-plane operation/proposal records to build bounded snapshots and transitions. Required design work includes snapshot caching, changed-node/edge semantics, deleted-memory tombstones, revision-range limits and animation cancellation. This feature must preserve the same distinction between canonical links and derived semantic overlays.

### Relationship comparison

Add split view and animated interpolation between two force/layer configurations, with shared camera and selected-node identity.

### Standalone interactive export

Package one filtered graph snapshot, vendored runtime modules and redacted metadata into a self-contained directory or archive. Do not make it part of the initial security boundary.
