# Python uMCP parity

Reference: [`rcarmo/umcp`](https://github.com/rcarmo/umcp) commit `30cce7dfe08c6ee63de235f7d81754ba286dafbb` (runtime-identical to deployed `9c89a708d14ae804e32aa65de10af7c02922617d`).

The module owns 22 source-generated fixture families under `testdata/parity`:

- shared JSON-RPC/version and HTTP/origin/URL rules;
- dispatcher, cancellation, progress, notifications and pagination;
- tool signatures/coercion/schema/results;
- resources/templates/subscriptions, prompts and completion/logging;
- CLI composition;
- stdio, file, TCP, legacy SSE and Streamable HTTP;
- synchronous/asynchronous modes, raw HTTP parsing and raw wire behavior.

`TestDifferentialFixtureManifest` fails if any expected fixture is absent or unreferenced. The full module currently has 100% statement coverage, race tests, fuzz targets and baseline amd64/ARM64 cross-builds. Parent CI also runs all 254 pinned upstream Python tests and the Memento service against Python 3.12, 3.13 and 3.14.

Idiomatic Go APIs use explicit signatures and context rather than reproducing Python reflection/class identity. Observable protocol payloads, validation decisions, ordering, transport framing, session binding, cancellation/progress and errors are the parity boundary.
