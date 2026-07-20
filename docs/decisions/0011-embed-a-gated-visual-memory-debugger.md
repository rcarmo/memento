# ADR 0011: Embed a gated visual memory debugger

**Status:** accepted
**Date:** 2026-07-20

## Decision

Memento includes an optional browser-based visual debugger for human inspection of memory creation, provenance, relationships and derived state. It is served by the Memento process on the same host and port as MCP, under `/graph`, with bounded JSON APIs under `/graph/api/`.

The complete surface is controlled by one global setting, `observability.graph_explorer.enabled`, which defaults to `false`. When disabled, the HTML, JavaScript, styles, assets and data APIs return `404`. When enabled, the surface is intentionally unauthenticated and visible to anyone who can reach Memento. It is a local development and trusted-LAN debugging facility and must not be enabled on an Internet-facing deployment. Enabling it does not change MCP authentication or authorisation.

The initial debugger shows current state plus provenance and management metadata. Memories are primary nodes. Assets and proposals are optional satellite nodes expanded on demand. Principals, revisions and operations begin as filters and overlays rather than permanent nodes.

Explicit Markdown links are canonical relationship state. Semantic proximity, shared tags, namespace, type and provenance are separate computed layout influences with independent controls and distinct visual encoding. Embeddings are derived analytical data, not relationships. Missing or stale embeddings never imply broken links.

The server computes deterministic coarse clusters, communities, positions and bounded layout hints. The browser refines only the visible working set from a deterministic seed. The initial response is aggregated for repositories above the direct-render threshold; clusters and neighbourhoods expand on demand up to the existing 10,000-concept ceiling. Raw embedding vectors never leave the server.

The browser uses vendored Three.js for the WebGL2 scene and vendored Preact for controls, inspectors and diagnostics. Source is browser-native ES modules. Bun is used only for vendoring, integrity verification, housekeeping and tests. The project does not introduce Vite, Webpack, Rollup or another JavaScript build system.

The debugger is read-only for canonical Git knowledge and SQLite control state. It may request selected, visible-neighbourhood or full embedding refresh because embeddings are rebuildable derived state. Full refresh requires explicit confirmation and runs through the existing short-lived batched GTE worker.

## Purpose

The debugger exists to help humans understand and guide Memento's development. It should make these questions answerable without reading Git, SQLite and logs separately:

* How was a memory created or changed, by which principal and operation?
* Which explicit links are present, broken or unexpectedly central?
* Which memories are isolated, duplicated, oversized, stale or outside their expected namespace cluster?
* How do structural links, semantic similarity and metadata groupings differ?
* Which assets and proposals surround a memory, and how much storage do they consume?
* Are repository, index and embedding revisions aligned?
* Why is a selected node positioned, sized, coloured or flagged as it is?

This is observability, not an end-user knowledge browser and not a graph editor.

## Visual model

Memory nodes are rendered in a 2.5D WebGL scene with depth-aware camera movement, curved arcs, instancing, GPU picking and level of detail. The default node radius uses logarithmically scaled combined Markdown and retained-asset bytes. Operators may switch size to Markdown bytes, asset bytes, explicit degree, semantic-neighbour density, update frequency or age.

Colour defaults to memory type and may switch to namespace, creator, age, status, embedding readiness, proposal state, asset presence or anomaly class. Health and anomaly state also use outline, shape or badges so meaning never relies on colour alone.

Positioning combines independently adjustable forces:

* explicit links, strongest by default;
* semantic proximity from current compatible embeddings;
* shared tags;
* namespace;
* memory type;
* provenance.

Stored links and inferred similarity use different edge types, legends and inspector language. The selected-node inspector explains every active force and cluster assignment.

## Inspector and diagnostics

Selecting a memory opens a read-only inspector containing:

* stable ID, path, title, type, status and tags;
* creator/principal, creation/update timestamps and current Git revision;
* Markdown, asset and combined byte counts;
* proposal or operation origin where available;
* explicit inbound and outbound links;
* semantic neighbours with scores, model ID and embedding revision;
* current repository, index and embedding revision state;
* cluster membership and layout-force explanations;
* expandable asset and proposal satellites;
* sanitised bounded Markdown preview;
* copy-path and focus-neighbourhood actions.

The MVP detects and explains structural, lifecycle and content-shape anomalies: orphan nodes, broken links, excessive degree, isolated clusters, stale indexes or embeddings, pending/failed management state, near-duplicates, size outliers, tag drift and namespace outliers. Each diagnostic reports its rule and measured value rather than one opaque anomaly score.

## Progressive loading

The overview endpoint returns bounded aggregate nodes and edges. A repository with no more than the direct-render threshold may return memories directly. Larger repositories return namespace/community aggregates with counts, byte totals, anomaly summaries and deterministic centroids. Expansion endpoints return a bounded cluster page or selected neighbourhood. Semantic neighbours are computed server-side for the requested working set only.

The browser renders no more than approximately 2,000 memory nodes at once. Before frame rate collapses it reduces labels, semantic edges and satellite detail. It never downloads the complete 10,000-node embedding matrix.

## Exports

The MVP exports:

* PNG for the current rendered scene;
* SVG for a bounded selected neighbourhood, preserving labels and curved typed edges;
* JSON containing the current filtered nodes, typed edges, coordinates, active force settings, revisions and diagnostics.

Exports contain no raw vectors, credentials, bearer tokens or full asset payloads. Standalone interactive HTML export is deferred.

## Browser and performance requirements

The supported clients are current desktop Chromium, Firefox and Safari, plus tablet touch. WebGL2 is required. Phone-sized interaction and a non-WebGL renderer are not MVP requirements.

Acceptance budgets are:

* useful aggregated overview within five seconds over LAN;
* at least 30 fps for 2,000 visible memory nodes and bounded edges on an ordinary laptop;
* selection feedback below 100 ms;
* cluster expansion begins below 250 ms and may stream;
* bounded and cancellable server responses;
* visible fetch, layout, render, node/edge and frame-pressure diagnostics.

## Security consequences

The global setting is an explicit trust-boundary switch. Startup logs and the UI warn that enabling it exposes memory metadata without authentication. Titles, paths, tags, topology, provenance, asset sizes and previews may all be sensitive. Documentation forbids enabling it on Internet-facing deployments.

The graph APIs use strict route, method, query and response bounds. They do not expose raw filesystem paths outside bundle paths, SQLite internals, credentials, token environment names, raw vectors or asset bodies. Markdown previews are sanitised and bounded. Canonical/control mutations are unavailable from this surface.

Derived embedding refresh uses a narrow maintenance API with bounded scope, one coalescing job, progress reporting and explicit full-refresh confirmation. It cannot invoke proposal, review, apply, create, patch or rename operations.

## Validation

The feature requires:

* backend tests for the global gate, 404 behaviour, route bounds, aggregation, provenance, diagnostics, semantic-neighbour computation, refresh jobs and exports;
* deterministic layout and community fixtures;
* Playwright tests on Chromium, Firefox and WebKit for mouse, keyboard and tablet-touch workflows;
* light/dark visual snapshots;
* synthetic 500, 2,000 and 10,000-memory fixtures;
* machine-readable fetch/layout/render/frame-budget reports;
* no-secret/no-vector export tests;
* the existing Python, Rust, wheel, container, multi-architecture and no-AVX gates;
* a real DiskStation Portainer deployment and browser smoke while enabled on the trusted LAN.

## Deferred features

Revision playback and animated graph diffs are explicitly planned after the MVP. That feature will derive snapshots from Git history and control-plane events without changing their authority. Split-screen comparison of two relationship/force configurations and standalone interactive HTML export are also deferred.

## Alternatives considered

* **Separate graph container:** rejected because it would duplicate configuration, expose another trust boundary and either require privileged database mounts or a second API anyway.
* **Authenticated graph explorer:** deferred because the agreed MVP is an explicitly enabled trusted-network debugging surface. MCP authentication remains unchanged.
* **SVG-only renderer:** rejected because thousands of animated nodes, curved edges and touch camera movement require GPU-backed rendering.
* **Canvas-only renderer:** rejected because Three.js provides the 2.5D camera, instancing, picking and depth effects needed for the intended polish.
* **Full graph in one response:** rejected because 10,000 concepts plus inferred edges and labels would exceed browser and response budgets.
* **Browser-side raw embeddings:** rejected because vectors are large derived implementation data and unnecessary for rendering.
* **Semantic similarity as a relationship:** rejected because embeddings do not change canonical relationship state.
* **Automatic full embedding refresh on page load:** rejected because opening a debugging UI must not unexpectedly load GTE or start repository-wide work.
* **General editing from the graph:** rejected because the debugger must observe canonical/control state, not become another mutation path.
* **A JavaScript bundler/framework toolchain:** rejected because browser-native modules and vendored libraries meet the requirement with less supply-chain and maintenance overhead.
