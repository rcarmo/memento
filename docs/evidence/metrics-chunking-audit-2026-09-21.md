# Metrics and chunking audit — 21 September 2026

**Original audit: release blocked.** Eleven new invariant tests failed against the audited working tree. The remediation checkpoint at the end records subsequent fixes and passing tests. The earlier 100% statement-coverage, race, cross-build and parity results did not cover these behaviours.

## Scope and state

- Base commit: `26bef206b675c25a0ff655aba056216c352f8075` (`v1.0.4`).
- Includes uncommitted document chunking, worker cancellation/pacing, search and storage changes.
- Production was last verified on `v1.0.2`. This audit made no production API calls, refreshes, configuration changes or deployments.
- Versions 1.0.3 and 1.0.4 were published earlier but not deployed to production. No additional release, tag, commit or push was made during this audit.
- Review combines source inspection, independent model review and disposable local SQLite/Git/worker reproductions. It is a focused audit of the changed paths and their dependencies, not an exhaustive review of every Memento subsystem.

Source paths and line numbers below refer to this working tree and may move during remediation.

## Confirmed worker and chunking defects

### W1 — New work can be lost while a path is in flight (high)

`internal/derived/semantic_worker.go:61–86, 138–221`

`Enqueue` deduplicates the path but overwrites a shared revision. Completion of the older job removes the sole path entry without checking whether it was re-enqueued. A second full enqueue can also be cleared when an earlier pending-path query returns empty.

Reproductions:

- `TestAuditReenqueueIsNotLost`: enqueue `/a.md` at `r1`, block inference, enqueue the same path at `r2`, release the first job. Observed refresh revisions: `[r1]`; `r2` is lost.
- `TestAuditNewFullRequestNotCleared`: block the first full request's empty-query result, enqueue a new full request, release the old result. Only one pending query runs.

Fix: track per-path request generations and a separate full-refresh generation. Retire only the generation actually processed; never clear a newer full request with an older empty snapshot. Use a consistent lock boundary when capturing a job.

### W2 — Terminal failures can loop indefinitely; revision changes magnify the problem (high)

`internal/derived/semantic_worker.go:152–175, 197–226`

`internal/derived/semantic_chunks.go:81–139, 158–165`

A full refresh polls the first eligible path repeatedly. Planning/publication errors can leave it eligible forever. The retry delay is one millisecond and no terminal failure is recorded for that queue generation.

Reproductions:

- `TestAuditTerminalFullRefreshStops`: 75 failing refresh attempts in an 80 ms sample. This uses a stub to demonstrate retry policy, not a production throughput measurement.
- `TestAuditUnrelatedRevisionDoesNotRejectUnchangedDocument`: advance concept revision metadata without changing the document during inference. Publication returns `document changed during chunk embedding`. The worker can retain the obsolete queued revision and redo inference repeatedly.

Fix: bounded interruptible backoff for transient errors, terminal-failure suppression per request generation, and current revision/content revalidation. An unrelated Git commit should not require repeating unchanged-document inference indefinitely. Genuine document changes must requeue the latest content safely. Retry eligibility and error reporting need explicit contracts.

### W3 — Model and chunk-policy changes do not schedule regeneration (high)

`internal/derived/index.go:242–249`

`internal/derived/semantic_chunks.go:54–58`

Refresh selection checks missing/stale/pending rows and the `chunks-v1:` prefix. It does not compare model revision, model ID, dimensions or `MaxInputChars`. The fingerprint covers text only. Search, however, filters by current model identity, so a model change can hide ready rows without scheduling replacements.

Reproductions:

- `TestAuditModelChangeQueuesReadyChunks`: replace stored model revisions with a predecessor value; refresh eligibility is `[]` for two documents.
- `TestAuditFullHashIncludesChunkPolicy`: embed with a 512-character guard, change it to 4096; refresh eligibility remains `[]`.

Fix: persist and compare an explicit embedding-policy identity covering model identity, tokenisation/chunk algorithm version, token budget, overlap and character guard. Keep that policy distinct from the full-document content fingerprint. Retain rollback-safe storage and avoid automatic production re-embedding when startup refresh is disabled.

### W4 — Persisted inference errors are counted as successful completions (high)

`internal/derived/semantic_chunks.go:171–196`

`internal/derived/semantic_worker.go:197–220`

Publishing an error row returns nil. The worker increments `Completed` and clears its last error. This hides failed documents from worker status and the existing completion metric.

`TestAuditFailedDocumentNotCountedCompleted` observes:

```text
stored=error worker_completed=1 worker_error=<nil>
```

Fix: distinguish successful publication of an error record from successful inference. Report failed attempts separately and preserve useful error state without reintroducing the terminal retry loop. Define whether counters count documents, chunks or batches; the current counter counts successful refresh calls, not individual chunk embeddings.

## Confirmed metrics defects

### M1 — Cancellation does not bound a scrape (high)

`internal/service/live_metrics.go:92–115, 161, 285–305`

The handler imposes no collection deadline or concurrent-scrape bound. Control/derived page queries use `QueryRow`, not `QueryRowContext`. Control metrics also use the application connection pool directly.

`TestAuditMetricsCancellationBoundsControlWait` reserves the only control connection and invokes collection with an already-cancelled context. Collection still waits until the test releases that connection.

Fix: propagate a bounded scrape context through every SQL call; cap concurrent collections. Do not serve stale cached values as fresh successes if caching is introduced. A read-only metrics handle must not execute application startup/migration paths. Cancellation cannot guarantee interruption of all filesystem/kernel I/O, so document that limitation separately.

### M2 — Freshness compares two database values, not Git HEAD (high)

`internal/service/live_metrics.go:169–176`

Both exported revisions come from `index_state`. A Git commit followed by a failed or delayed derived update can leave these equal and old, while metrics reports `index_stale 0`.

`TestAuditMetricsDetectsGitAdvance` creates a new commit/ref in a disposable bare repository without updating SQLite. Metrics exports the old repository/index revisions and `stale=false`.

Fix: compare the index against the canonical repository revision or a reliably maintained canonical-revision snapshot. Represent unavailable/inconsistent observations explicitly. Reading two matching values from the derived database is insufficient.

### M3 — One scrape combines different database snapshots (medium)

`internal/service/live_metrics.go:169–183`

Revision state, page counts, table counts and embedding counts are separate autocommit queries. A writer can commit between them.

`TestAuditMetricsSnapshotConsistency` commits a new document and revision between the state/count reads. The scrape returns the old revision with the new concept count.

Fix: collect related derived SQL data in one short read transaction; release it promptly. File sizes, process metrics and separate databases cannot be one globally atomic snapshot and should be described accordingly.

### M4 — Missing embeddings are absent from backlog gauges (medium)

`internal/service/live_metrics.go:235–263`

The aggregation reads only `concept_embeddings`. New concepts with no embedding row are eligible for the real worker but contribute neither pending nor missing counts.

`TestAuditMetricsCountsMissingEmbeddings` observes one concept and `{error:0 pending:0 ready:0 stale:0}`.

Fix: expose missing rows with a left join and align the eligible-backlog metric with the worker's actual policy/model-aware selection. Keep document backlog and chunk progress separate.

## Release/rollback defects from source inspection

### R1 — Retention does not preserve digest dependencies (high)

`.github/workflows/release.yml:397–416, 442–483`

The cleanup retains five tagged GHCR versions and deletes all untagged versions older than seven days. Multi-architecture indexes reference untagged platform manifests. Their age does not establish that they are unreferenced. The cleanup can select children of retained indexes for deletion.

The same cleanup has no protected set for the current production or rollback digest. `tools/test_go_container_contract.sh:7` pins `v1.0.2` in this same GHCR package. The earlier pipeline failed with `manifest unknown` for its previous `0.5.9` dependency. Replacing that pin repaired the immediate CI failure but did not repair retention.

Fix: preserve explicit live/rollback references and the transitive manifest graph of retained releases, including attestations. Until a tested reachability calculation exists, do not delete untagged manifests solely by age. Test deletion plans offline using synthetic graphs; do not validate by deleting registry objects.

The independent review mistakenly described the rollback digest as potentially external; source inspection confirms it belongs to the same `rcarmo/memento` package, strengthening this finding.

### R2 — Run/artifact retention is detached from retained releases (medium)

`.github/workflows/release.yml:418–439`

The newest five workflow runs are retained independently of release/tag membership or successful completion. Failed/manual/rerun activity can evict evidence for a still-retained release.

Fix: retain runs/artifacts associated with retained/protected releases and treat ad-hoc runs as a separate retention set. Tests must cover five retained releases followed by more than five failed/manual attempts.

## Additional risks and acceptance gaps

- **Index quarantine is too broad.** `internal/derived/index.go:74–105` quarantines most SQLite errors, not only corruption. The new chunk publication path avoids this, but semantic search still uses it. The existing malformed-chunk-schema test reaches quarantine from a read. Narrow corruption classification and test cancellation, constraints, disk-full and schema/programming errors separately.
- **Search recall is capped before scoring.** `internal/derived/semantic_search.go:140–144` selects the first `MaxCandidates` rows by path before computing similarity (default 200). A best-match document beyond that prefix is never scored. This predates chunking but limits the meaning of “full-document retrieval”. The tail fixture demonstrates retrieval within the selected set only.
- **Metrics endpoint is always unauthenticated.** `internal/service/runtime_models_off.go:374–395` routes it before graph handling; `umcp/http.go:261–276` explicitly makes auxiliary routes responsible for their own authentication. It exposes aggregate counts and revision labels even with graph debugging disabled. This is a documented design choice, but no tested scrape-only boundary or opt-in exists. Resolve the deployment/auth policy before exposure.
- **Label churn and escaping need review.** Revision labels create a new series per commit; database-provided embedding statuses are not restricted to a fixed vocabulary. `%q` is Go string quoting, not a general Prometheus label encoder. Normal SHA/version/status strings do not trigger the problem; an unexpected control character can. Use bounded status values and test a real Prometheus parser.
- **Metrics coverage is incomplete.** Go heap gauges are not RSS or container memory, and there is no process CPU metric in this implementation. Database errors, chunk counts/progress and model-worker process activity also need accurate definitions. Use existing container exporters where appropriate instead of relabelling heap memory as RSS.
- **Rollback test has a limited scope.** The actual `v1.0.2` binary passed `status → rebuild-index → status` on disposable state, followed by a new-runtime reopen. This is a models-disabled format check. It does not prove semantic retrieval after rollback, semantic-enabled rebuild/refresh, or downgrade/upgrade policy invalidation.
- **Scale evidence is incomplete.** One actual GTE tail-retrieval fixture and subprocess tests passed. They do not establish NAS throughput, worst-case long-document tokenisation cost, candidate-scale SQL/vector costs, or retrieval quality across the corpus. Per-concept chunk queries, vocabulary reloads and long WordPiece inputs need measurement before optimisation.
- **Worker shutdown is only partly bounded.** New chunk inference shares a cancellable context and progressive batches recheck admission, but the full queue's pending-path lookup still uses `context.Background()` and waits on the index mutex. Test cancellation while blocked there.

## What the independent review did not establish

The review suggested ASCII boundary/lowercasing could violate Unicode safety. The actual GTE tokenizer intentionally uses the same ASCII rules and counts Unicode runes for input limits. Real-vocabulary and fuzz tests cover multibyte input and oversized words. Those suggestions are not confirmed defects by themselves.

It also treated using the application's control DB handle as proof of a write. The metrics code currently issues read PRAGMAs through that handle; no write by that code was proven. Shared-pool contention and missing cancellation were reproduced and are the relevant defects.

## Reproduction commands

The new tests assert desired behaviour and intentionally fail against this audited source. They are not disabled and must become passing regressions during remediation.

```sh
go test ./internal/derived \
  -run '^TestAudit(Reenqueue|Terminal|NewFull|ModelChange|FailedDocument|FullHash|Unrelated)' \
  -count=1 -v

go test ./internal/service -run '^TestAuditMetrics' -count=1 -v

go test -race ./internal/derived ./internal/service \
  -run '^TestAudit(Reenqueue|Terminal|NewFull|ModelChange|FailedDocument|FullHash|Unrelated|Metrics)' \
  -count=1 -v
```

All eleven fail both normally and under race instrumentation, without a race-detector data-race report. These are logical concurrency/state defects.

## Evidence retained

- `internal/derived/audit_repro_test.go`: seven failing invariants.
- `internal/service/metrics_audit_repro_test.go`: four failing invariants.
- `/workspace/tmp/memento-audit-20260921/`: observed test logs, audited tracked diff and untracked-file checksums.
- Prior, narrower passing evidence: `make quality` with 100% coverage; `make race cross`; Go-only `make python-parity`; chunk fuzzing; real-model/subprocess tail tests; actual predecessor format reopen.

The working tree now fails the added regression suite. It must not be described as release-ready or currently green.

## Remediation order and exit gate

1. Fix worker generations, completion/error semantics and bounded retry/cancellation together. The seven worker/identity reproductions must pass without weakening their assertions.
2. Define and persist embedding-policy identity; test model/config changes, semantic-enabled rollback, and incremental/full convergence under concurrent edits.
3. Fix metrics deadline/admission, canonical freshness, consistent SQL snapshots, backlog definitions and access/cardinality policy. All four metrics reproductions must pass.
4. Fix retention with offline graph/pin/run fixtures and verify the proposed deletion set without changing the registry.
5. Measure large-document and candidate-scale behaviour; add real-model retrieval cases beyond the path-prefix candidate cap and negative authorization cases.
6. Repeat independent review, then quality/race/parity/model/fuzz/container/rollback gates against one frozen candidate. Preserve live/rollback artifacts and verified backup before any later approved production test.

No new patch release is proposed by this report.

## Remediation checkpoint — 21 September, after audit

The earlier sections record the failing audited tree. The working candidate now passes all eleven reproductions without removing their assertions. Queue generations preserve newer requests; terminal full-refresh errors stop that run with an error instead of spinning; transient DB errors back off. Policy identity includes the model and chunk settings. Unchanged document publication tolerates unrelated repository advances. Persisted inference errors propagate to worker status and do not increment completion.

Metrics now have serial admission and bounded SQL contexts, canonical Git HEAD comparison before/after collection, a read-only derived snapshot, missing/other embedding gauges, fixed status labels and proper Prometheus escaping. Live revision-string labels were removed. Chunk-aware search scores all authorised candidates before ranking and performs query inference outside the index lock. Both index lock waits and worker inference/queue queries honour cancellation. Legacy adapters retain their exact parity contracts.

Retention no longer deletes any GHCR manifest by age. Protected release refs include `v0.5.9` and `v1.0.2`; release-run deletion is tied to removed release tags, not the newest arbitrary run count. Four offline retention tests pass. The previous cleanup had already deleted the `v1.0.2` index. Registry restore returned HTTP 404 with the available bot credential. The exact NAS image was exported without changing the production container, SHA-256 checked and republished as a protected rollback reference:

- original image config: `sha256:22e4bb5ccca9836f3304201af0e840d6d0e9dd88cdd60fde80cc2cb1a1c37513`;
- preserved archive: `a41fd40d44aca5b4469b824a9e71ea2621f3ac22526d94e95e75bc8397bfccd8`;
- new immutable amd64 manifest: `sha256:07cf50633a25a087242905091ca91755ae6683a88e5e0439e9b0196cb8106ea1`.

The published rollback reference contains the same image/config, not a rebuilt substitute. The container contract now uses that digest and passed the predecessor/current/predecessor state cycle. Separate actual-1.0.2-binary tests passed status/rebuild/status with semantic mode enabled, followed by current-runtime reopening.

Verification: full quality with 100% statement coverage, race, cross-build, vulnerability scan, Python-free parity, all-package fuzz targets, real GTE/Needle parity and allocation budgets, actual GTE chunk retrieval and subprocess inference, and local container/rollback contract passed. The real tail-retrieval fixture scores the answer document 0.9319 versus distractor 0.8022. Idle container daemon RSS was 34.6–36.5 MiB. Independent re-review found no remaining hard blocker in the reviewed worker generation/retry, chunk publication/policy and metrics transaction/freshness paths.

Still required before declaring deployment complete: exact-commit CI, one immutable release, state-copy canary, verified fresh backup, cutover, authenticated disposable lifecycle, restart/convergence and metrics scrape checks. These are execution gates, not claimed by the local results above.
