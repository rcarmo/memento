# Stable MCP output schemas

Issue [#46](https://github.com/rcarmo/memento/issues/46) requests discoverable schemas for stable structured results. The patch adds object-root `outputSchema` metadata to status, help, read, list and search. It preserves existing JSON/text payloads, inputs, authentication, pagination and discovery order.

## Schema boundary

`internal/service/output_schemas.json` stores each schema once. Embedded tool definitions opt in with `outputSchemaRef`; the registry receives its own schema copy. Historical input-dispatch fixtures deliberately omit that new reference when using non-service stub outputs. Real runtime tests enforce the output contract independently.

Each schema describes the complete envelope using disjoint `success` and `error` branches. Required envelope fields and stable data fields are typed, explicit nulls remain valid, and known objects reject unexpected fields. Limits, frontmatter and per-proposal-status maps retain extension points. Domain-error responses validate as errors rather than becoming accidental transport failures. Invalid service output retains the existing redacted transport failure behaviour.

The schemas are additive metadata. Dynamic execute/route and mutation outputs remain schema-less. Consumers may use stable schemas for binding generation and validation; schema-less structured content remains valid MCP.

## Verification

Real storage/auth/runtime contract tests cover all five discovery surfaces, success and ordinary error responses, model-disabled nulls, immutable metadata copies and payload drift. Independent JSON Schema v6 validation checks the same responses in addition to uMCP's intentionally small runtime validator. The full validator is a test-only dependency; it is absent from `go list -deps ./cmd/memento-go`. A bounded 1,000-execution fuzz probe covers schema validation, and the full pre-release gates run separately.

The historical catalog/tool fixtures remain unchanged. Tests compare historical input/annotation/discovery-order metadata independently of the new output-schema metadata. No acceptance threshold or numerical contract was widened.

## Measured registration tuning

Profiling found repeated schema copies during catalog construction: visible tools were registered twice solely to replace discovery. The patch registers callbacks once, then sets the visible discovery order independently. Hidden callbacks remain callable subject to the same authorisation; all surface and input/order contracts pass.

Equivalent `BenchmarkStableOutputCatalogRegistration` on Go1.26.11/i7-12700, three 1-second runs: median9.197ms →6.252ms (-32.0%), about8.37MB →5.20MB allocated per construction (-37.9%),82,409 →48,396 objects (-41.3%). Schemas still receive registry-owned deep copies; no shared mutable metadata was introduced. This removes repeated work rather than weakening validation. Targeted CPU/alloc_space/alloc_objects profiles were inspected; raw captures are removed after comparison.

## Issue triage

Current NAS v1.1.0 compact discovery was live-verified with Needle enabled/loaded: search, read, execute and asset retrieval are advertised; authenticated search/read pass. Closed #41 as already shipped. Current proposal backlog/unresolved counts are both zero; pending-only status semantics, filtering/sorting and visibility-bound pagination were shipped in1.0.11 and remain covered. Closed #42 with that evidence.

#37 requires separate approval and lifecycle/resource validation for optional warm worker reuse/Vulkan; it is not enabled by this patch. #40 requires model training, a new Needle3 architecture/format and independent runtime qualification; it remains a roadmap item. Production GTE/Needle and Gemma default-off policy stay unchanged.
