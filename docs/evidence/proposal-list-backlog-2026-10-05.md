# Proposal backlog and listing correction

Production 1.0.10 reported `proposal_backlog: 11` because it counted ten `needs_rebase` proposals and one `conflicted` proposal. Supported MCP reads on 2026-10-05 returned zero submitted, approved or draft proposals at repository/index revision `b044a6bab7cf3a3642d48660aae00d603c326fe6`. No production proposal was reviewed, rebased, rejected, applied or deleted during this investigation.

The change for [#42](https://github.com/rcarmo/memento/issues/42) separates the counts:

* `proposal_backlog`: caller-visible submitted and approved proposals awaiting review/application.
* `proposal_unresolved`: all caller-visible unresolved states, including draft, needs-rebase and conflicted.
* `proposal_counts`: caller-visible totals by status, with legacy stale normalised to needs-rebase. Absent keys represent zero.

With the observed production records unchanged, the new status would report backlog zero and unresolved eleven. The new code has not been deployed.

## Listing contract

Default listing selects `pending` (submitted/approved) and sorts by `created_at` descending. `status="all"` includes completed history; `unresolved` includes repair/draft work; individual stored states remain selectable. `exclude_statuses` removes selected states, so a review-only queue can exclude `approved`. Applied records are completed history; approved records still await application.

`sort_by` accepts `created_at`, `updated_at` or `proposal_id`; `sort_order` accepts `asc` or `desc`. SQL identifiers are selected from those fixed values, while filter values remain bound parameters. Proposal ID breaks timestamp ties in the same direction. Encrypted cursors bind caller permissions, revision, filters and ordering, and retain the last visible record's ordering tuple even if that anchor is deleted. Old-format cursors must be restarted. Pages are not snapshots across concurrent edits, particularly with mutable updated timestamps.

Hidden candidates are scanned until the page has enough visible records or the query ends. Author and namespace checks remain in force; hidden rows no longer cause empty pages before visible results. Status counts use the existing read-visibility policy; proposal listing retains its existing proposer/review write-grant requirement.

Direct schemas, execute argument validation, reference-aware preflight field lists, embedded execute schemas and catalog descriptions are aligned. Original Python oracle files remain unchanged. Tests adapt only the deliberate contract changes, replay the historical visibility/error/cursor-scope cases, and independently assert new defaults, distinct/tied timestamps, exclusions, cursor invalidation, deleted/updated anchors, HTTP/MCP execute dispatch and status counts.

## Validation performed

All executed Go test commands used the profiling wrapper. `GOTOOLCHAIN=go1.26.6`, CPU profiling at Go's default 100 Hz and allocation sampling at 524,288 bytes were used for the final runs. Each run retains package binaries, CPU/heap profiles, logs, command/flags, toolchain/revision and generated cumulative CPU/`alloc_space`/`alloc_objects` tables under ignored `build/profiles/`. Later wrapper runs also retain tracked diffs and untracked source copies.

| Gate | Result |
| --- | --- |
| Root offline suite | 27 packages passed (`full-final`) |
| Root coverage suite | 27 packages passed; 100.0% production statement coverage (`coverage-pass`) |
| Nested uMCP suite | 2 packages passed (`umcp-final`) |
| Focused control/service/execute race tests | 3 packages passed (`proposal-race`) |
| Final schema/legacy catalog tests | 2 packages passed (`schema-fixed`) |
| Additional real-startup MCP and distinct-timestamp tests | passed (`final-integration`) |
| Final wrapper/source capture check | passed (`wrapper-final`) |
| Formatting, vet, pinned Staticcheck, shell syntax, diff whitespace | passed |

The first Staticcheck attempt encountered a missing shared Go-cache file; retry completed without diagnostics. Earlier regression runs exposed old fixture assumptions, missing schema copies, and test-code compilation mistakes; their logs are retained. Runs that failed to compile have no CPU/heap profiles and are explicitly failed capture attempts. Functional failures that executed produced profiles and were analysed before fixing the failures.

## Profile findings and measured change

The broad service suite spends most allocation volume in JSON decoding and repeated runtime/catalog construction used by tests. The final covered service run allocated about 4,074 MiB cumulatively; JSON decoding accounted for about 2,080 MiB cumulative and runtime construction about 1,843 MiB cumulative. These overlap. SQL execution and filesystem calls dominated cumulative CPU. They describe the full test workload, not a single proposal request or retained service memory.

A separate `BenchmarkProposalListPending` avoids runtime/catalog setup in the timed loop. It seeds 1,000 synthetic proposals (100 submitted, 900 applied) and reads a newest-first 21-row page. The initial profile attributed most allocated bytes to scanning/result construction and most CPU to SQLite execution. Preallocating the bounded result slice removed repeated growth without changing database schema or permission checks.

| Query implementation | Observed ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| New listing before slice preallocation, one run | 289,028 | 31,300 | 646 |
| Preallocated, repeat 1 | 236,592 | 22,480 | 641 |
| Preallocated, repeat 2 | 225,127 | 22,480 | 641 |
| Preallocated, repeat 3 | 227,354 | 22,481 | 641 |

Bytes/op decreased by about 28%; five allocations/op were removed. An intervening preallocated run took 564,381 ns/op under variable host conditions, so these samples do not establish a latency improvement. No performance budget was widened. SQLite row/text conversion still dominates allocation objects and query CPU; repeated catalog decoding remains a separate optimisation candidate.

Profile analysis covered failed and passing runs, including the final focused MCP test: its allocations are dominated by disposable runtime construction and catalog JSON, while the request/query benchmark isolates the list path. Cumulative tables are retained with binaries for deeper inspection.

## Pre-release limits

Fuzz execution currently fails before launching tests: Go's ordinary profile flags capture the coordinator, not fuzz workers. Worker-aware capture must be implemented before `make fuzz`/`make audit` can pass. A refusal check confirmed exit two without creating a run directory or running a fuzz target.

Thirteen short packages in the uninstrumented root suite, twelve in coverage and the uMCP hello example produced empty CPU samples. Their heap profiles were retained, but no CPU-performance conclusions are drawn from them. Changed service/control/execute paths have non-empty representative profiles. Test-spawned helper binaries are not separately profiled; top-level package profiles do not measure those child processes. The full subprocess profiling requirement therefore remains incomplete.

No full race, fuzz campaign, vulnerability scan, model/corpus run, cross-build, container/release check or deployment accompanies this change. The independent review attempts timed out; no delegated review approval is claimed. Passing functional tests and coverage do not complete the new overall pre-release profiling gate.

## Reproduction

Use the profiling-aware Make targets for suites, or the wrapper for a focused package:

```sh
PROFILE_ROOT=build/profiles/proposals \
  tools/test-profile.sh ./internal/service -- -run '^TestProposalList'
PROFILE_ROOT=build/profiles/list-benchmark \
  tools/test-profile.sh ./internal/control -- -run '^$' \
  -bench '^BenchmarkProposalListPending$' -benchmem -benchtime=1s
```

The wrapper uses `-count=1`, produces separate package/run directories, reports empty/missing captures and generates cumulative tables. Review those tables after every run. `PROFILE_MEM_RATE=1` can be selected for allocation-exhaustive focused work, with its overhead reported separately from the standard sampled workload.
