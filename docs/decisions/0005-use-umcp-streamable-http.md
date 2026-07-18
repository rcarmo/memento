# ADR 0005: Use uMCP and Streamable HTTP

**Status:** accepted  
**Date:** 2026-07-18

## Decision

Memento uses [`rcarmo/umcp`](https://github.com/rcarmo/umcp) as its MCP server and transport core. Network clients connect through Streamable HTTP with bearer authentication. Caller identity comes from uMCP's request context, never from tool arguments.

The package pins uMCP commit `691af9f159757d45c180856ec0dfb89da7aa341c`. Memento keeps stdio and the wider uMCP compatibility work outside its own service logic.

## Why

Memento needs one protocol boundary for Piclaw and other MCP clients, with request-local principals, protocol negotiation, bounded request bodies and remote-safe errors. Implementing those pieces in Memento would duplicate transport work and make authentication easier to get wrong.

Streamable HTTP works across hosts and containers without giving clients filesystem or Git access. The same service can be used by Piclaw, another agent runtime or a small MCP client.

## Consequences

* Every MCP principal has a separate bearer token and namespace policy.
* The server accepts principal identity only from authenticated request context.
* Large asset proposals use a configured 72 MiB HTTP request ceiling; decoded ZIP validation has its own 50 MiB limit.
* Reverse proxies must preserve the Authorization header and permit the configured request size.
* Transport upgrades are made in uMCP and pinned deliberately in Memento.
* The release image includes the pinned Git dependency because the `umcp` name on PyPI belongs to another project.

## Alternatives considered

* **Build a Memento-specific HTTP/MCP server:** rejected as duplicate security-sensitive work.
* **Use legacy SSE as the primary transport:** rejected in favour of the current Streamable HTTP protocol.
* **Give agents direct Git or filesystem access:** rejected because it bypasses identity, namespace checks, proposals and operation recovery.
