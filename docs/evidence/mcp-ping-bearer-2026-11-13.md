# MCP ping and Bearer scheme fixes

Issues #43 and #44 are fixed together on `review/mcp-ping-bearer`, based directly on `main` at `c92d3d9`. The separate Gemma branch was first published through `c5bfdb3`; no Gemma inference changes are included here. Production was not deployed or contacted.

## Changes

- Register `ping` in `umcp.NewServer`'s shared protocol dispatcher. It returns an empty object, without invoking a tool or mutating state. Other unknown methods still return `-32601`.
- Share a Bearer parser between MCP identity and admin HTTP authentication. Only the six-character scheme is case-insensitive; a literal space remains required and credentials pass unchanged to the original authenticator. Token case, leading/trailing bytes and whitespace are not normalised.
- Preserve authoritative managed authentication and transport session boundaries. No new anonymous route, token fallback or session-based authentication bypass is introduced.

The immutable Python identity fixture retains its original case-sensitive expectation. One narrowly documented test override records the deliberate RFC 9110 scheme correction without rewriting reference data.

## Verification

Focused and broader tests passed on Linux amd64 with Go 1.27.1 (runtime builds use `CGO_ENABLED=0`; race instrumentation uses CGo only for tests):

- Shared server dispatch and stdio initialize→ping return `{}`; raw large JSON-RPC IDs survive without precision loss. Streamable HTTP notification responses remain empty HTTP 202. Existing shared notification semantics were preserved.
- Models-off runtime HTTP initialization negotiates `2025-03-26`, accepts canonical/lower/upper/mixed-case schemes, sends initialized notification, and answers repeated session-bound ping requests.
- Negative tests preserve missing/wrong credential 401, token-case rejection, wrong session owner 403, unknown session 404, and bad/mismatched protocol version 400.
- Static and real managed-store identity plus actual admin HTTP handler tests cover scheme variants, malformed separators, unsupported schemes, unknown/empty credentials, exact credential-byte preservation and managed-store errors. Disabled/revoked credentials cannot fall back to overlapping static credentials.
- A managed MCP session test disables its authenticated credential after initialization and a successful ping, then verifies the existing session receives 401 for a later ping.
- Focused service race checks, complete nested uMCP race suite, complete root `go test ./... -count=1`, complete nested `umcp` tests, relevant vet checks and whitespace checks passed. The root suite does not automatically traverse the nested module, so it was run explicitly.

A built `cmd/memento-go` was launched with a disposable models-off repository on a loopback ephemeral port. Synthetic reader credentials were supplied only to that child. For `Bearer`, `bearer`, `BEARER` and `bEaReR`, the real HTTP daemon completed initialization→initialized notification→ping with `{}`. Missing credentials and changed token case returned 401; an unknown method still returned `-32601`. The probe stopped the daemon and removed its repository safely. No production image, config, credential or index was used.

Read-only review found no substantive implementation defect. Its suggested managed-session disable/re-authentication check was added and passed. The reviewer did not execute Git diff commands, so final branch/scope checks belong to the main task.

Failed probes were corrected before acceptance: initial ping test calls omitted explicit request context; handler signature initially omitted the RPC-error result; wrong-owner test initially used an invalid synthetic token and expected an incorrect status. A first real-daemon probe rejected the symlink-containing `/workspace` path; the second resolved the project-owned scratch path physically, preserving the existing symlink safety check. No validation/security check was weakened to pass a probe. Ordinary stdio notification behaviour was deliberately not broadened as part of these issues.

## Diagnostic profile conclusions

CPU, alloc_space and alloc_objects captures were inspected for repeated focused tests and the nested dispatcher. The service test workload includes repeated real managed-store creation/token hashing, runtime setup and JSON validation:

- Service CPU: 1.32 s samples; scrypt Salsa work 26.52% flat. Allocation space 606.33 MiB, including scrypt 256 MiB and JSON decoder fetch 132.46 MiB; 3,158,658 sampled objects, dominated by reflection/JSON copies.
- Shared uMCP dispatcher: 60 ms CPU samples over a roughly 63 ms capture, so individual CPU-node rankings have limited resolution. Allocation space 37.54 MiB over 1,000 repeated test cases; JSON decoder creation 9.50 MiB. This is a whole-test measurement, not production ping latency or a per-request allocation budget.

The material outliers are existing secure password hashing in repeated setup and generic JSON/test work, not the new scheme comparison or empty-result handler. No hashing/security work was weakened and no performance improvement is claimed for these correctness fixes. This diagnostic pass does not qualify a release or NAS throughput. Raw profiles, matching binaries, daemon probe source/config/state, temporary fixtures and disposable logs were removed after analysis; only source tests and these concise conclusions remain.
