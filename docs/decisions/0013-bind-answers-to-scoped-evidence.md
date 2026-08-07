# ADR 0013: Bind answers to scoped evidence

Status: accepted

## Context

`memory_answer` used exact cache, hot context and a bounded deep traversal, but the returned contract only exposed citations. That was enough to reject fabricated IDs at one Git revision, yet it did not explain why each concept was selected, whether a graph neighbour displaced the primary result, or which authorization scope governed retrieval.

A model could also see stale or sensitive material before policy had enough information to reject it. Putting more instructions in the prompt would not fix that boundary: authorization, temporal state and secret handling belong in service code.

A local 23-concept, 15-query corpus showed hybrid top-5 was the useful default. Top-10 recovered missing support at a context cost, while graph expansion helped relational questions and harmed unrelated ones when applied indiscriminately.

## Decision

Keep `memory_answer` as the one public answer operation and attach a versioned `EvidenceSet` to every enabled hot or deep answer.

Question profiling is deterministic and runs before the cache. Explicit secret intent returns `UNKNOWN` before cache lookup, search, graph access, concept reads or model invocation. The profile also identifies current, historical, relational and explicit personal/work namespace intent.

Deep retrieval uses hybrid top-5. It escalates to top-10 only when a lexical sufficiency check cannot find enough query, namespace or temporal support. Semantic unavailability is recorded as a lexical fallback.

Service code filters evidence before the reader:

* ordinary principal authorization applies to search, graph traversal and every fresh concept read;
* explicit personal/work intent narrows evidence to that namespace;
* sensitive tags are rejected;
* current questions reject deprecated, tombstoned, stale, obsolete, historical and conflicting concepts;
* historical questions may retain deprecated concepts;
* outside historical mode, selected superseders remove the concept IDs they replace.

Graph closure runs only for relational questions. It is depth one from the first two primary anchors, cannot exceed `max_concepts`, and never moves graph neighbours ahead of primary evidence. Every graph item carries its anchor and minimal support chain; citations to it include that chain.

Repository bodies remain untrusted data. Evidence items say so explicitly, prompts delimit each body, and final citations must match concepts read at the exact Git revision.

## Consequences

Answers now disclose retrieval strategy, ranks, state, timestamps, source references, supersession, graph provenance and authorization scope. Cache entries created before this contract are ignored and replaced.

The policy is deliberately narrow. It does not reconstruct arbitrary historical Git revisions, infer secrets from model output or add a second public evidence tool. Retrieval can spend one extra top-10 search when top-5 is insufficient, and relational questions can add two bounded graph lookups.

Disabled answers keep the old deterministic `UNKNOWN` payload with `answer_source="disabled"` and no evidence. This preserves the model-free deployment path.
