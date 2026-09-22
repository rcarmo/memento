# Broken-link regression check — 22 September 2026

The 16 currently reported broken-link occurrences predate the item-relative embedding and persisted-cache releases. No new source/target pair or duplicate occurrence was introduced by v1.0.7 or v1.0.8 in the compared snapshots. The literal destinations are missing; the findings are consistent with existing imported references that do not match Memento's repository layout.

## Evidence

The live v1.0.8 overview was untruncated, with matching repository/index revision `22b7497924acee09e217b63ef8c5a06d4e5a2964`. It contained 16 broken occurrences, 14 unique source/target pairs and 13 affected concepts. Two duplicates are separate occurrences in the source Markdown, not duplicated diagnostics.

| Verified snapshot | Total broken links in snapshot | Current occurrences absent from snapshot |
| --- | ---: | ---: |
| 21 September 18:33, before v1.0.5 | 70 | 0 |
| 21 September 23:01, before v1.0.6 | 16 | 0 |
| 22 September 14:59, before v1.0.7 | 16 | 0 |

The snapshot comparison uses source path, raw destination and multiplicity, not transient graph edge IDs. Each source was also inspected in Git, along with its accepted asset manifests. None of the literal destinations exists in the current repository or its retained path history, and none is an included sibling file in the source's asset pack.

The retained Python indexer at `7f29e8b` only looked up concept IDs for absolute destinations. It therefore also classified these non-external relative references as broken. The current Go resolver does resolve paths relative to the source document, but the resulting literal paths still do not exist. Neither `internal/repository/links.go` nor `internal/derived/links.go` changed between `5181eb3` (before v1.0.2) and current `b122578`.

## Findings by destination

| Raw destination | Occurrences | Literal repository destination | Existing related concept / interpretation |
| --- | ---: | --- | --- |
| `../visual-information-design/SKILL.md` | 12 | `/skills/visual-information-design/SKILL.md` | `/skills/diagrams/visual-information-design.md` |
| `../technical-docs/SKILL.md` | 2 | `/work/skills/technical-docs/SKILL.md` | `/skills/writing/technical-docs.md` |
| `../graph-design/SKILL.md` | 1 | `/skills/graph-design/SKILL.md` | `/skills/diagrams/graph-design.md` |
| `../../../piclaw/runtime/skills/builtin/remote-peer/SKILL.md` | 1 | `/piclaw/runtime/skills/builtin/remote-peer/SKILL.md` | Piclaw workspace source reference outside this knowledge repository; do not map it to itself automatically |

The visual-design, graph-design and remote-peer references first appear in the affected concepts at import commit `5a67e3c`, 17 September 07:48 UTC. The visual-design and graph-design target concepts already existed in that import, but their paths differed from the links.

The two technical-docs references first appear in `fd1dcf1`, 25 August 11:16 UTC. The technical-docs concept then lived at `/skills/technical-docs.md`; it later moved to `/skills/writing/technical-docs.md`. The original relative destination matched neither location, so the later move did not break a previously resolving reference.

## Source locations

Line numbers refer to the stored Markdown at backup revision `19b64ec9ad1d52fc64a56153172dfbbf917611d2`; current source/target pairs match that snapshot.

| Source | Raw target | Lines | Occurrences |
| --- | --- | --- | ---: |
| `/work/skills/ado/ado-wiki-governance.md` | `../technical-docs/SKILL.md` | 39, 73 | 2 |
| `/skills/development/web-artifacts-builder.md` | `../visual-information-design/SKILL.md` | 27 | 1 |
| `/skills/documents/pptx.md` | `../visual-information-design/SKILL.md` | 31 | 1 |
| `/skills/documents/pdf.md` | `../visual-information-design/SKILL.md` | 28 | 1 |
| `/skills/piclaw/remote-peer.md` | `../../../piclaw/runtime/skills/builtin/remote-peer/SKILL.md` | 49 | 1 |
| `/skills/piclaw/adaptive-cards-authoring.md` | `../visual-information-design/SKILL.md` | 27 | 1 |
| `/skills/diagrams/excalidraw.md` | `../visual-information-design/SKILL.md` | 26 | 1 |
| `/skills/documents/xlsx.md` | `../visual-information-design/SKILL.md` | 33 | 1 |
| `/skills/operations/token-chart.md` | `../visual-information-design/SKILL.md` | 32 | 1 |
| `/skills/diagrams/drawio-azure-architecture.md` | `../graph-design/SKILL.md` | 31 | 1 |
| `/skills/diagrams/drawio-azure-architecture.md` | `../visual-information-design/SKILL.md` | 30 | 1 |
| `/skills/diagrams/graph-design.md` | `../visual-information-design/SKILL.md` | 30, 420 | 2 |
| `/skills/documents/docx.md` | `../visual-information-design/SKILL.md` | 29 | 1 |
| `/skills/infrastructure/graphite-power-chart.md` | `../visual-information-design/SKILL.md` | 27 | 1 |

## Interpretation and repair boundary

These particular warnings have no evidence of being new runtime regressions. They are longstanding path/reference problems, largely consistent with importing filesystem-oriented skills without translating cross-skill links to concept paths. The remote-peer reference describes an external workspace source and needs an explicit source URL or contextual treatment rather than a guessed concept link.

No concepts, asset packs or resolver rules were changed during this investigation. Canonical links can be repaired through the normal proposal workflow after confirming intent. For skill concepts, the Markdown and root `SKILL.md` must stay aligned; corrections should publish a new asset version rather than mutate an immutable accepted pack. Broad fuzzy resolver aliases would conceal the underlying ambiguity.

Evidence: `/workspace/tmp/memento-link-regressions-20260922/` contains the live overview, extracted links, three historical link lists, per-reference Git provenance, source lines and asset-manifest analysis. The migration observation is separate: at 18:42 UTC it had 11 ready, 248 legacy, one missing, zero errors and no false repository-relative stale findings.
