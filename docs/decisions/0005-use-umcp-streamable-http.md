# ADR 0005: Use uMCP and Streamable HTTP

**Status:** accepted  
**Date:** 2026-07-18
**Amended:** 2026-09-19

> `v1.0.0` keeps the protocol decision and implements the pinned observable uMCP behaviour in the standalone `umcp` module. The external Python package is no longer a runtime dependency.

## Decision

Memento uses the standalone pure-Go `umcp` module as its MCP server and transport core, pinned behaviourally to [`rcarmo/umcp`](https://github.com/rcarmo/umcp). Network clients connect through Streamable HTTP with bearer authentication. Caller identity comes from uMCP's request context, never from tool arguments.

The compatibility baseline pins uMCP `v0.2.2` release commit `9c89a708d14ae804e32aa65de10af7c02922617d`; checked-in fixtures also cover the authorised reference tip `30cce7dfe08c6ee63de235f7d81754ba286dafbb`. Memento keeps stdio and the wider uMCP compatibility work outside its own service logic. The transport reuses HTTP/1.1 connections, binds MCP session IDs to authenticated principals, and delivers subscribed resource notifications over reconnectable GET/SSE streams. Invalid `MCP-Protocol-Version` headers return actionable JSON rather than an empty `400`.

## Why

Memento needs one protocol boundary for Piclaw and other MCP clients, with request-local principals, protocol negotiation, bounded request bodies and remote-safe errors. Implementing those pieces in Memento would duplicate transport work and make authentication easier to get wrong.

Streamable HTTP works across hosts and containers without giving clients filesystem or Git access. The same service can be used by Piclaw, another agent runtime or a small MCP client.

## Consequences

* Every MCP principal has a separate bearer token and namespace policy; managed credentials are verified from control-plane records after bootstrap.
* The server accepts principal identity only from authenticated request context. Admin-only `access_*` tools use the same endpoint and are hidden from non-admin discovery.
* Large asset proposals use a configured 72 MiB HTTP request ceiling; decoded ZIP validation has its own 50 MiB limit.
* Reverse proxies must preserve the Authorization, `Mcp-Session-Id`, `MCP-Protocol-Version` and `Last-Event-ID` headers, permit GET/POST/DELETE/OPTIONS, and allow the configured request size.
* One GET/SSE stream may own a session at a time. Authenticated `DELETE /mcp` releases it immediately; an abruptly closed stream may make replacement GETs return `409` until a notification or periodic keepalive write detects the closure.
* Reconnection starts a fresh stream. Notifications missed while disconnected are not replayed, including when the client sends `Last-Event-ID`.
* Transport upgrades are reviewed against the pinned reference and implemented in `umcp`; the release image contains no external uMCP package.
* The module path is `github.com/rcarmo/memento/umcp`, avoiding the unrelated `umcp` package name on PyPI.

## Alternatives considered

* **Build a Memento-specific HTTP/MCP server:** rejected as duplicate security-sensitive work.
* **Use legacy SSE as the primary transport:** rejected in favour of the current Streamable HTTP protocol.
* **Give agents direct Git or filesystem access:** rejected because it bypasses identity, namespace checks, proposals and operation recovery.
