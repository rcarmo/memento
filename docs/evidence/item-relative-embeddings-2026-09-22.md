# Item-relative embeddings and persisted graph similarities

The v1.0.6 graph suppressed semantic connections after an unrelated repository update. Item-relative, chunk-only eligibility and persisted semantic scores replace that behaviour in v1.0.7. Canary and production semantics passed, but a legacy metrics count defect required [rollback and a corrective release](release-1.0.7.md).

## Reproduction

The 22 September snapshot had repository/index revision `19b64ec9ad1d52fc64a56153172dfbbf917611d2`, embedding summary revision `1f301b7f6a9829367da503f0c5d3e8499f6a1abe`, 258 visible items, 259 ready embedding rows and one missing row. The graph emitted 257 `embedding_stale` findings and zero semantic edges. Most input had not changed.

Inspection of the previously verified production backup found 259 embedding rows but only one item with ten persisted chunks. The other 258 rows were legacy single vectors. Reporting those rows as ready obscured the remaining conversion work.

The 16 unresolved explicit links are unchanged stored references: twelve `../visual-information-design/SKILL.md`, two `../technical-docs/SKILL.md`, one `../graph-design/SKILL.md`, and one workspace-relative remote-peer reference. They are separate from semantic freshness. No guessed target rewrites were made.

## Changes

- Embedding freshness uses the item's effective title/description/body fingerprint, model ID/checkpoint/dimensions and chunk policy. Repository revisions remain provenance; they do not gate graph or search eligibility.
- Stable-ID renames, metadata-only edits, rebuilds and unrelated commits preserve vectors, timestamps and provenance. Changed inputs invalidate only their item. Multi-item index updates now commit atomically, including links, metrics and embedding cleanup.
- Refresh/search require chunk-capable clients. Search and graph have no legacy-vector fallback. Startup labels legacy rows for regeneration and invalidates incompatible policies without deleting valid chunks. The first-chunk parent mirror is retained only for rollback readers.
- Search ranks the best authorised compatible chunk per item. Graph aggregates equal-weight normalized chunks into one normalized item vector.
- `semantic_graph_items` persists those aggregates and `semantic_graph_pairs` persists compatible pairwise cosine scores. Chunk publication updates the changed item and incident pairs atomically. Startup backfills missing entries with the same item-and-pairs routine; valid entries are untouched.
- Triggers invalidate affected cache entries on changed input/model/status/policy, chunk changes and deletion. Repository revision, timestamp and path changes alone do not invalidate them.
- Web requests read stored scores and metadata only. They apply permission scope and active policy before threshold/top-neighbour selection. Hidden nodes cannot alter visible neighbour rankings. No query or principal response is stored in the cache.
- Metrics distinguish `legacy` rows. Graph diagnostics distinguish legacy conversion from actual stale item input/configuration.

## Verification

Local checks passed:

- `make quality race vuln cross`: exact 100% production statement coverage, race detector, no called-code vulnerabilities, amd64/ARM64 cross-builds.
- `make fuzz model-test performance release-check MEMENTO_VERSION=1.0.7 SOURCE_DATE_EPOCH=0`: fuzz, real GTE/subprocess/Needle tests, allocation budgets and release archives.
- `make ui-test UI_REPEAT=2`: 72 passing browser cases, no skips, plus unit/Gherkin/lifecycle tests.
- SQL fault injection checks cache publication and migration rollback. Restart tests preserve pair scores. Startup tests install a trigger rejecting cache insertion and verify compatible entries are not recomputed.
- `TestSemanticWebLoadsReadCachedScoresOnly` makes all vector columns unavailable, then verifies repeated fresh service instances still return cached semantic connections.
- Ranking tests compare stored-score selection with a vector oracle, including ties, caps and visibility filtering. Item tests cover rename/metadata preservation, input invalidation, failed-update atomicity and deletion cleanup.

The retained Python fixtures are unchanged. Tests explicitly supersede their old repository-relative embedding revisions and partial-update commit behaviour; other captured fields remain compared.

Independent review identified partial update commits and excessive chunk-pair graph work; both were corrected. A later cache review timed out and was not counted as approval. A bounded design review raised pair-backfill completeness; the implementation uses full item-and-pairs publication during startup, and the clarified review found no remaining blocker in that protocol. Exact-commit CI and semantic-cache canary/production qualification passed for v1.0.7. Corrective metrics release and completion of production legacy migration are still required.

Raw evidence is under `/workspace/tmp/memento-diagnostics-20260922/`.
