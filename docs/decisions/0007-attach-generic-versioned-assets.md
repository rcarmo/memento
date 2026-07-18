# ADR 0007: Attach generic versioned assets to concepts

**Status:** accepted  
**Date:** 2026-07-18

## Decision

A memory concept may carry immutable, versioned ZIP assets. Accepted packs are stored through Git LFS under:

```text
/.assets/<concept-id>/<asset-kind>/<version>.json
/.assets/<concept-id>/<asset-kind>/<version>.zip
```

Binary bytes for pending proposals live in `control.sqlite`; proposal JSON stores the asset ID, digest and manifest. Review and apply use the ordinary proposal lifecycle.

Skills are not a server-side content type. A skill is a normal concept under `/skills/`, tagged `skill`, with an attached `asset_kind="skill"` pack. Its Markdown body must match the ZIP-root `SKILL.md`. Piclaw imports a recalled pack into `.pi/skills/<name>/`; Memento does not install or run it.

## Why

The first implementation gave skills their own proposal table, tools, search and storage layout. That duplicated concept behavior and made other binary attachments impossible without another parallel feature.

A generic asset layer keeps one memory model and one review queue. Storing by concept ID lets a concept move without breaking its assets. Git LFS keeps binary history out of ordinary Git objects while preserving versioned references.

## Consequences

* `memory_propose` accepts `attach_asset_pack` changes alongside concept create or patch changes.
* `memory_asset_get` retrieves the latest stable version or a named version.
* `memory_asset_prune` keeps five versions by default and protects versions referenced by active proposals.
* Pack names and kinds use lowercase words and hyphens; versions use stable semantic versions.
* ZIP validation rejects traversal, links, nested archives, encrypted entries, native executables and oversized payloads.
* Skill-specific MCP wrappers were removed; clients use search/read, ordinary proposals and generic asset tools.
* Schema-5 pending skill proposals and the earlier `/skills/.versions/` repository layout are migrated on upgrade.

## Alternatives considered

* **First-class skill objects:** implemented and removed because they duplicated proposals, curation and search.
* **Store base64 ZIPs in proposal JSON:** rejected because it inflates proposal rows and makes diffs and list responses unwieldy.
* **Store accepted assets by path:** rejected because renaming the concept would leave stale attachment paths.
* **Let Memento extract or install skills:** rejected because importing executable scripts belongs to the client workspace and its local policy.
