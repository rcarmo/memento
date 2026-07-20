# ADR 0011: Embed a gated visual memory debugger

**Status:** accepted
**Date:** 2026-07-20

## Context

Memento already records enough information to explain how shared memory evolves, but inspecting it means moving between Markdown, Git history, `control.sqlite` and `derived.sqlite`. That is tolerable while debugging one concept and hopeless when trying to understand clusters, broken links, oversized assets or the difference between stored links and semantic proximity.

We need a human debugging view rather than another agent tool. It should make the current memory graph legible, show where each piece came from and expose stale or surprising state without becoming a second editor.

## Decision

Memento will serve an optional visual debugger at `/graph` on the same host and port as MCP. `observability.graph_explorer.enabled` controls the entire surface and defaults to `false`. When it is off, the page, static files and API routes return `404`.

When enabled, `/graph` is unauthenticated. This is deliberate: the first deployment is on a trusted LAN and the tool is for development. Titles, paths, tags, topology, provenance and previews may be sensitive, so the page carries a warning and the documentation tells operators to leave it off on an Internet-facing service. MCP authentication and namespace policy do not change.

Memories are the primary nodes. Assets and proposals appear as satellites when a memory is expanded; principals, revisions and operations are filters or overlays. Explicit Markdown links are the only stored relationships. Semantic similarity, common tags, namespace, type and provenance may influence layout, but they use separate edge styles and explanations. An absent embedding says nothing about the health of a Markdown link.

The server supplies stable clusters, coarse coordinates and bounded pages. The browser refines only the visible set, which keeps the 10,000-concept repository limit practical without sending the embedding matrix to the client. Repositories above the direct-render threshold open as aggregates and expand by cluster or neighbourhood.

Three.js renders the WebGL2 scene. Preact handles controls and the inspector. Both libraries are committed at exact versions with checksums and licences. The application ships as browser-native ES modules; Bun handles vendoring, integrity checks and tests, but there is no JavaScript bundle in the release.

The debugger cannot change Markdown, proposals or operation records. It may ask the existing short-lived GTE worker to refresh selected, visible or all embeddings, since those rows are disposable index data. A full refresh requires confirmation.

## What The View Shows

The default node radius reflects Markdown plus retained asset bytes on a logarithmic scale. Other size modes cover Markdown, assets, explicit degree, semantic density, update frequency and age. Colour defaults to memory type and may switch to namespace, creator, age, status, embedding state, proposal state, assets or anomaly class. Outlines and badges carry warnings when colour is already being used for something else.

Selecting a memory opens its path, type, status, tags, timestamps, updater, revision, sizes, explicit links, embedding metadata, proposals, assets and a short sanitised Markdown preview. The inspector also explains the forces affecting its position.

Diagnostics name the rule and measured value. The first set covers orphans, broken links, high degree, isolated groups, revision lag, embedding failures, pending proposals, duplicate content, size outliers, tag drift and namespace outliers. Semantic diagnostics are labelled as derived observations rather than relationship defects.

The current scene can be saved as PNG. A bounded neighbourhood can be exported as SVG, and JSON export contains the filtered nodes, typed edges, coordinates, settings, revisions and diagnostics. None of these formats includes tokens, raw vectors or asset payloads.

## Consequences

The setting is a meaningful trust-boundary switch rather than a cosmetic feature flag. Enabling it exposes debugging metadata to anyone who can reach the service.

Large repositories need an aggregate-first API and level of detail in the browser. The useful target is an overview within five seconds over the LAN, at least 30 fps with 2,000 visible nodes, selection feedback below 100 ms and expansion starting below 250 ms. Current desktop Chromium, Firefox and Safari are supported, along with tablet touch; WebGL2 is required.

Graph responses need coherent repository and index revisions, stable ordering and strict node, edge, preview and export bounds. The implementation must avoid one SQL query or filesystem read per edge. Raw embedding vectors remain inside Memento.

The release must include the browser modules and licence files, pass the existing Python, Rust, packaging and no-AVX checks, and be exercised through Portainer on the DiskStation before the debugger is left enabled on the trusted LAN.

## Deferred

Revision playback and animated diffs will use Git history and operation records after the current-state view is settled. Split comparison between two relationship/force configurations and standalone interactive export are also later work.

## Alternatives

A separate graph container would need another API or direct access to Memento's databases, adding a second deployment and trust boundary. Keeping the view in Memento gives it coherent snapshots and the existing model/index lifecycle.

SVG alone is attractive for documents but not for thousands of moving nodes, depth, touch camera movement and picking. A plain canvas would work, although it would recreate facilities already available in Three.js.

Sending every concept and vector in one response would make the browser responsible for sensitive data and expensive clustering. Aggregate-first responses keep both sides bounded.

An editor embedded in the graph would create another write path and blur its purpose. Changes continue through MCP proposals and curator operations.
