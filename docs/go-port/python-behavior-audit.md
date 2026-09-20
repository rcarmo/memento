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

`tools/generate-python-parity-features.mjs` regenerates this layout and rejects unassigned or multiply assigned rows. The Go gates reject missing logical directories, missing/duplicate scenario IDs, untagged scenarios, absent Given/When/Then clauses, missing Go test functions, changed source hashes, catalogue drift and access-tool drift.

## Evidence levels

The manifest does not label every Python node as one-to-one differential coverage. The 392 ordinary Memento rows are `mapped_domain_suite`: each maps to existing runnable Go tests and, where already available, source-generated parity fixture families. Four rows are `intentional_replacement`:

* Python package version `0.5.9` becomes the Go release version contract;
* the disabled Python Needle FFI defaults become the configured subprocess/mmap Go boundary;
* Python ctypes Needle lifecycle and cancellation become pure-Go/subprocess lifecycle tests;
* FFI generation output parsing becomes native Go constrained generation and route parsing.

uMCP has stronger direct cross-language evidence: 22 source-generated fixture families in addition to its 254-node test crosswalk.

A mapped domain suite proves that the corresponding Go tests exist and execute under `go test ./...`; it does not by itself prove exact one-to-one assertion equivalence. The generated behavior summary and Gherkin scenario preserve the original Python assertions so reviewers can strengthen individual rows with dedicated fixtures without losing inventory coverage.

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
