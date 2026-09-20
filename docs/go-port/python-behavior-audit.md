# Python behavior audit

The reference is `rcarmo/memento@7f29e8b003557f0105f47ed353b7f65a33619456` (`0.5.9`). The separate uMCP reference is `rcarmo/umcp@30cce7dfe08c6ee63de235f7d81754ba286dafbb`.

## Executed references

| Suite | Result | Captured by |
|---|---:|---|
| Memento Python pytest | 490 passed | 396 test nodes from 40 files in `testdata/parity/python-functional-manifest.json` |
| upstream Python uMCP pytest | 254 passed | 254 test nodes from 24 files in `umcp/testdata/python-functional-manifest.json` |
| Go root quality | 100% production statement coverage | `make quality` plus the Python parity manifest gate |
| Go uMCP | 100% statement coverage | `make -C umcp check` plus upstream Python manifest gate |

The expanded Python count is larger than the manifest node count because 33 Memento tests use parameterization. The manifest records an estimated 490 expanded Memento cases and the executed suite confirmed 490.

## Public surface

The [public surface matrix](python-surface-matrix.md) contains 102 rows:

* 44 MCP tools: 34 memory operations and 10 managed-access operations;
* 3 fixed resources, 2 resource templates and 1 prompt;
* 12 MCP protocol methods;
* 31 HTTP endpoint/action rows;
* 9 CLI commands, including the Go-only native healthcheck.

Each row records Python source, request/response/error behavior, Go implementation, real Go test declarations and outcomes for canonical reader, proposer, curator and admin profiles. The profile hierarchy used by the table is reader; reader+proposer; reader+proposer+curator; and all four roles. `access_*` operations require the explicit admin role. `memory_execute` is reader-visible, while every operation inside a plan retains its own role and namespace authorization.

## Gherkin organization

Application features under `testdata/parity/features/application/` are grouped by behavior:

* managed access;
* concepts/Git and assets/skills;
* proposal/control workflows;
* search/answer/routing;
* semantic/embedding and Needle/model transport;
* graph snapshots/diagnostics/UI;
* runtime/configuration/packaging;
* service MCP workflows.

Public-surface features under `testdata/parity/features/surfaces/` group MCP discovery, answer/routing/execute, proposals, assets, mutations, access administration, resources/prompts/completion, protocol lifecycle, HTTP families and CLI operations. Every surface has one success scenario, reader/proposer/curator/admin scenarios and one malformed/failure scenario.

The independent uMCP features under `umcp/testdata/features/` group schema/tools/coercion, resources/prompts/completion, discovery/notifications/progress, context/errors, Streamable HTTP/sessions and transports/integrated servers.

`tools/generate-python-parity-features.mjs` regenerates this layout and rejects unassigned or multiply assigned rows. Gherkin contains only domain behavior and stable row IDs. Python test filenames, Go test names, source paths, helper calls and implementation-specific tags remain in the JSON evidence manifests, not in feature prose. The Go gates reject their reintroduction, as well as missing logical directories, missing/duplicate scenario IDs, untagged scenarios, absent Given/When/Then clauses, missing Go test functions, changed source hashes, catalogue drift and access-tool drift.

## Native-Go final validation

`make python-parity` runs `go test ./...` in the root module and the nested uMCP module. It does not invoke Python, pytest, Node, Bun, npm, npx or uv. A Makefile contract test rejects those commands in the target. The gate was executed successfully with all seven interpreter/package-manager commands replaced by failing shims.

Python and JavaScript are used only to regenerate or review versioned oracle artifacts. Ordinary CI consumes committed JSON/feature fixtures through Go tests.

## Executable validation data

The capture can now drive Go validation rather than only document coverage:

| Validation level | Surface rows | Meaning |
|---|---:|---|
| Exact JSON-RPC replay | 29 | 75 deduplicated Python requests and expected responses replay through `TestPythonCapturedToolReplayCases` |
| Exact MCP protocol fixture | 18 | Pinned uMCP payload/framing/session/resource/prompt fixtures consumed by Go uMCP tests |
| Executable stateful fixture | 54 | Repository/control/index/filesystem fixture family with a Go adapter and post-state/envelope comparator |
| Go extension test | 1 | Native `healthcheck`, which has no Python command equivalent |

`testdata/parity/python-tool-replay-cases.json` contains the 75 exact replay cases for 29 tools. `TestPythonCapturedToolReplayCases` invokes Go for every request and compares complete JSON-RPC responses, normalizing only JSON object order inside text content. `testdata/parity/python-stateful-case-families.json` indexes 1,339 cases across 31 executable stateful families. `testdata/parity/python-structured-surface-cases.json` adds 23 neutral setup/request/expected/state-delta cases for answer, model proposals, Dream, serve, master-key rotation and all 12 admin HTTP routes. Native Go service and CLI tests consume those cases directly. No public surface remains coverage-only or structured-only.

## Evidence levels

The manifest does not label every Python node as one-to-one differential coverage. The 392 ordinary Memento rows are `mapped_domain_suite`: each maps to existing runnable Go tests and, where already available, source-generated parity fixture families. Four rows are `intentional_replacement`:

* Python package version `0.5.9` becomes the Go release version contract;
* the disabled Python Needle FFI defaults become the configured subprocess/mmap Go boundary;
* Python ctypes Needle lifecycle and cancellation become pure-Go/subprocess lifecycle tests;
* FFI generation output parsing becomes native Go constrained generation and route parsing.

uMCP has stronger direct cross-language evidence: 22 source-generated fixture families in addition to its 254-node test crosswalk.

A mapped domain suite proves that the corresponding Go tests exist and execute under `go test ./...`; it does not by itself prove exact one-to-one assertion equivalence. The JSON behavior summary preserves the original Python assertions. Implementation-neutral Gherkin describes the expected domain outcome. Reviewers can add dedicated neutral fixtures without changing the stable row ID or losing inventory coverage.

## Go defects corrected by the detailed audit

The executable second pass found and fixed observable Go divergences rather than merely documenting them:

* `memory_answer` no longer requires a Python-absent reader role; proposer-only authenticated principals remain namespace-scoped and receive the deterministic disabled answer when intelligent answering is off;
* expected answer/model-proposal domain failures use service envelopes instead of generic JSON-RPC internal errors;
* model-proposal authorization precedes disabled-feature disclosure;
* unauthorized absolute target hints remain search hints instead of failing the call immediately;
* proposal context uses Python-compatible normalized FTS5 queries, strict lexical freshness, bounded depth-one eventual graph expansion and source-specific evidence revisions;
* model requests now carry the full Python prompt safety contract and `tool_version` metadata;
* stored model proposals retain consulted concepts, contradictions, reciprocal links and target hints, while refreshing the base revision after model completion;
* citation validation matches Python's ID/path/revision contract rather than imposing an extra title check;
* answer/model-proposal retrieval and model calls no longer hold the repository transaction lock;
* Dream skips model work with no actionable signals, caps actionable inputs, preserves proposal/model-attempt state during failure reconciliation and marks post-generation failures consistently.

Each correction has a native Go regression and retained neutral setup/request/expected/state-delta evidence.

## Reviewed behavior changes

These differences from Python are intentional and tested:

* relative Markdown and fragment-only links resolve from the source concept; protocol-relative links are external;
* accepted asset-pack files are valid non-concept references;
* the obsolete Python/Rust/C ABI runtime is replaced by pure Go and subprocess/mmap model boundaries;
* `healthcheck` is a Go operational command;
* Go typed APIs reject some Python-only runtime coercions and preserve documented deterministic error ordering instead;
* the restored Go admin page adds stale-session and credential-clearing behavior while preserving its HTTP/store contract.

Any further behavior difference requires an explicit manifest status and a tagged Gherkin scenario. The gates reject unknown statuses.

## Production status

Production runs the immutable Python `0.5.9` image digest `sha256:bebc0a3eaf935a5b4f07c3e060fd8e22a11dacff90cd55532ec04306c30e81bc`. It uses the historical Python shell command and socket healthcheck. After rollback it reported 257 concepts, repository/index revision `130cb3e506f08ffb9bf8cc973949bfa02bf064b9`, HTTP 200 for the graph status and HTTP 401 for unauthenticated MCP. The container reached healthy state.
