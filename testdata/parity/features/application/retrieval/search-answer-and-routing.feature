Feature: retrieval/search answer and routing

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Evidence

    @py-cb61675abe08
    Scenario: Evidence item requires explicit untrusted marker and provenance
      Given the controlled domain state and principal described by this behavior
      When the actor performs: evidence item requires explicit untrusted marker and provenance
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-0bb33762244a
    Scenario: Evidence sufficiency requires query namespace and temporal support
      Given the controlled domain state and principal described by this behavior
      When the actor performs: evidence sufficiency requires query namespace and temporal support
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-5a2c0ccbd20e
    Scenario: Namespace matching is strict for explicit namespace queries
      Given the controlled domain state and principal described by this behavior
      When the actor performs: namespace matching is strict for explicit namespace queries
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-e3560f44b17b
    Scenario: Profile does not treat my or token usage as secret namespace intent
      Given the controlled domain state and principal described by this behavior
      When the actor performs: profile does not treat my or token usage as secret namespace intent
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-ca82d76b8a09
    Scenario: Profile question classifies policy relevant intent
      Given the controlled domain state and principal described by this behavior
      When the actor performs: profile question classifies policy relevant intent
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-55f9ff129f4b
    Scenario: Query profile is closed to unknown contract fields
      Given the controlled domain state and principal described by this behavior
      When the actor performs: query profile is closed to unknown contract fields
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-557e8f62e311
    Scenario: Sensitive current and historical filters are deterministic
      Given the controlled domain state and principal described by this behavior
      When the actor performs: sensitive current and historical filters are deterministic
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Memory answer

    @py-8da1df0917f9
    Scenario: Adaptive retrieval checks sufficiency after supersession filtering
      Given the controlled domain state and principal described by this behavior
      When the actor performs: adaptive retrieval checks sufficiency after supersession filtering
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-8c357e71d02f
    Scenario: Adaptive retrieval escalates from five to ten only after insufficiency
      Given the controlled domain state and principal described by this behavior
      When the actor performs: adaptive retrieval escalates from five to ten only after insufficiency
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-48565aca7416
    Scenario: Answer cache scope includes protected namespaces
      Given the controlled domain state and principal described by this behavior
      When the actor performs: answer cache scope includes protected namespaces
      Then the authorization decision and visible result match the specified principal scope

    @py-3f1fa08562cf
    Scenario: Current and historical profiles filter conflicts differently
      Given the controlled domain state and principal described by this behavior
      When the actor performs: current and historical profiles filter conflicts differently
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-bd42392155f8
    Scenario: Dream budgets cap oversized candidates and daily proposals
      Given the controlled domain state and principal described by this behavior
      When the actor performs: dream budgets cap oversized candidates and daily proposals
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-6b61639a784b
    Scenario: Dream duplicate window and no overlap
      Given the controlled domain state and principal described by this behavior
      When the actor performs: dream duplicate window and no overlap
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-82aeff30cc6d
    Scenario: Dream no signal means no model call
      Given the controlled domain state and principal described by this behavior
      When the actor performs: dream no signal means no model call
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-9ef5d8905356
    Scenario: Dream propose creates proposal without git mutation
      Given the controlled domain state and principal described by this behavior
      When the actor performs: dream propose creates proposal without git mutation
      Then the response and durable state transition match the specified lifecycle

    @py-1e7f8f12638f
    Scenario: Dream recent activity is bounded by last successful revision
      Given the controlled domain state and principal described by this behavior
      When the actor performs: dream recent activity is bounded by last successful revision
      Then the bounded result and continuation state match the specified contract

    @py-8de2a3beb20f
    Scenario: Dream scanner detects signals dedupes and updates watermark
      Given the controlled domain state and principal described by this behavior
      When the actor performs: dream scanner detects signals dedupes and updates watermark
      Then the response and durable state transition match the specified lifecycle

    @py-265b346ec330
    Scenario: Hot memory unknown falls back to deep and intersecting write invalidates hot answer
      Given the controlled domain state and principal described by this behavior
      When the actor performs: hot memory unknown falls back to deep and intersecting write invalidates hot answer
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-34e715a8d412
    Scenario: Memory answer attaches scoped provenance and filters superseded sensitive items
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory answer attaches scoped provenance and filters superseded sensitive items
      Then the authorization decision and visible result match the specified principal scope

    @py-6b422b3003dd
    Scenario: Memory answer exact cache is revision scoped and scope isolated
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory answer exact cache is revision scoped and scope isolated
      Then the authorization decision and visible result match the specified principal scope

    @py-209f016dbad9
    Scenario: Memory answer honors cancellation before model call
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory answer honors cancellation before model call
      Then cancellation or timeout preserves the specified completion and reconciliation state

    @py-4fb4f66aeaed
    Scenario: Memory answer model fallback does not replay whole agent
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory answer model fallback does not replay whole agent
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-2215bdb5f390
    Scenario: Memory answer rejects invalid citations
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory answer rejects invalid citations
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-d884970ad87d
    Scenario: Memory answer returns deterministic unknown when flags are off
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory answer returns deterministic unknown when flags are off
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-258c6de462ad
    Scenario: Model proposals only store submitted proposals without git mutation
      Given the controlled domain state and principal described by this behavior
      When the actor performs: model proposals only store submitted proposals without git mutation
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-04ff01fe0bdc
    Scenario: Model proposals reject malformed output forbidden namespace and secrets
      Given the controlled domain state and principal described by this behavior
      When the actor performs: model proposals reject malformed output forbidden namespace and secrets
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-f0cc3a704382
    Scenario: Model proposals require search context and store consulted citations
      Given the controlled domain state and principal described by this behavior
      When the actor performs: model proposals require search context and store consulted citations
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-82d604af100b
    Scenario: Model proposals resolve update target from hint
      Given the controlled domain state and principal described by this behavior
      When the actor performs: model proposals resolve update target from hint
      Then the response and durable state transition match the specified lifecycle

    @py-a25c49d5f9a5
    Scenario: Model proposals return disabled when flag is off
      Given the controlled domain state and principal described by this behavior
      When the actor performs: model proposals return disabled when flag is off
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-7a4970ee4bed
    Scenario: Namespace profile isolates personal evidence and marks injection untrusted
      Given the controlled domain state and principal described by this behavior
      When the actor performs: namespace profile isolates personal evidence and marks injection untrusted
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-a026446e6858
    Scenario: Namespace query cannot cross authorization boundary
      Given the controlled domain state and principal described by this behavior
      When the actor performs: namespace query cannot cross authorization boundary
      Then the bounded result and continuation state match the specified contract

    @py-e26e2d7f8ef3
    Scenario: Proposal and dream fallback default disabled
      Given the controlled domain state and principal described by this behavior
      When the actor performs: proposal and dream fallback default disabled
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-8e83a68ea1de
    Scenario: Relational closure preserves primary anchor and completes citation chain
      Given the controlled domain state and principal described by this behavior
      When the actor performs: relational closure preserves primary anchor and completes citation chain
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-2388362de401
    Scenario: Routed model enforces privacy boundary and slot classification
      Given the controlled domain state and principal described by this behavior
      When the actor performs: routed model enforces privacy boundary and slot classification
      Then the bounded result and continuation state match the specified contract

    @py-d08e3a735c6a
    Scenario: Routed model fallback tracks attempts and routes by slot
      Given the controlled domain state and principal described by this behavior
      When the actor performs: routed model fallback tracks attempts and routes by slot
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-d9de9740fa9d
    Scenario: Routed model no fallback on auth validation cancel or 429
      Given the controlled domain state and principal described by this behavior
      When the actor performs: routed model no fallback on auth validation cancel or 429
      Then cancellation or timeout preserves the specified completion and reconciliation state

    @py-981f2380a2d8
    Scenario: Secret intent abstains before cache retrieval reader and model
      Given the controlled domain state and principal described by this behavior
      When the actor performs: secret intent abstains before cache retrieval reader and model
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Router

    @py-d6f6f896048e
    Scenario: Canonical trained shallow tools json is valid
      Given the controlled domain state and principal described by this behavior
      When the actor performs: canonical trained shallow tools json is valid
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-775f93851a92
    Scenario: Defaults are deterministic
      Given the controlled domain state and principal described by this behavior
      When the actor performs: defaults are deterministic
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-d4cca0e49226
    Scenario: Execute plan validation for two step actions
      Given the controlled domain state and principal described by this behavior
      When the actor performs: execute plan validation for two step actions
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-cc7bb8102796
    Scenario: Expand read field
      Given the controlled domain state and principal described by this behavior
      When the actor performs: expand read field
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-222a1a2369c4
    Scenario: Expand search paths
      Given the controlled domain state and principal described by this behavior
      When the actor performs: expand search paths
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-fda201202764
    Scenario: Expand search then graph
      Given the controlled domain state and principal described by this behavior
      When the actor performs: expand search then graph
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-7d4ca0ad1621
    Scenario: Expand search then read
      Given the controlled domain state and principal described by this behavior
      When the actor performs: expand search then read
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-5995da39fd4a
    Scenario: Expand status field
      Given the controlled domain state and principal described by this behavior
      When the actor performs: expand status field
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-271ec608228e
    Scenario: Injection like strings remain data not code
      Given the controlled domain state and principal described by this behavior
      When the actor performs: injection like strings remain data not code
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-cc6d460be0da
    Scenario: Invalid and extra fields are rejected
      Given the controlled domain state and principal described by this behavior
      When the actor performs: invalid and extra fields are rejected
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-78c12376d1df
    Scenario: Parse needle router output normalizes bounded field aliases
      Given the controlled domain state and principal described by this behavior
      When the actor performs: parse needle router output normalizes bounded field aliases
      Then the bounded result and continuation state match the specified contract

    @py-1d8b343d419a
    Scenario: Parse needle router output requires exactly one call
      Given the controlled domain state and principal described by this behavior
      When the actor performs: parse needle router output requires exactly one call
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-6d59e7396c76
    Scenario: Status and read field mappings match sample payload shapes
      Given the controlled domain state and principal described by this behavior
      When the actor performs: status and read field mappings match sample payload shapes
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-1c45239d1e4c
    Scenario: Unknown has no execution
      Given the controlled domain state and principal described by this behavior
      When the actor performs: unknown has no execution
      Then the observable response, ordering, warnings and resulting state match the specified contract
