Feature: repository/concepts and git

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Repository core

    @py-8652ddb7d662
    Scenario: Authorization rejects noncanonical paths
      Given the controlled domain state and principal described by this behavior
      When the actor performs: authorization rejects noncanonical paths
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-abe841ad77b0
    Scenario: Concept body normalization preserves unicode code points
      Given the controlled domain state and principal described by this behavior
      When the actor performs: concept body normalization preserves unicode code points
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-4215c22f8c78
    Scenario: Concept schema validation model
      Given the controlled domain state and principal described by this behavior
      When the actor performs: concept schema validation model
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-7049a31a81c5
    Scenario: Concept serialization is thread safe
      Given the controlled domain state and principal described by this behavior
      When the actor performs: concept serialization is thread safe
      Then the result and persisted state remain deterministic under concurrent execution

    @py-b54fa05fe746
    Scenario: Deterministic index and log generation
      Given the controlled domain state and principal described by this behavior
      When the actor performs: deterministic index and log generation
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-332fb56eeacd
    Scenario: Envelopes are strict
      Given the controlled domain state and principal described by this behavior
      When the actor performs: envelopes are strict
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-5b169f241e0b
    Scenario: Extract links and rewrite rename
      Given the controlled domain state and principal described by this behavior
      When the actor performs: extract links and rewrite rename
      Then the response and durable state transition match the specified lifecycle

    @py-65fc712f270f
    Scenario: Frontmatter parse and deterministic serialize
      Given the controlled domain state and principal described by this behavior
      When the actor performs: frontmatter parse and deterministic serialize
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-8d7d25f29240
    Scenario: Frontmatter rejects unknown keys
      Given the controlled domain state and principal described by this behavior
      When the actor performs: frontmatter rejects unknown keys
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-cd616a5569a7
    Scenario: Generated indexes and log escape markdown titles and authors
      Given the controlled domain state and principal described by this behavior
      When the actor performs: generated indexes and log escape markdown titles and authors
      Then the authorization decision and visible result match the specified principal scope

    @py-6ab123080ad0
    Scenario: Protected namespace prefixes are strict
      Given the controlled domain state and principal described by this behavior
      When the actor performs: protected namespace prefixes are strict
      Then the authorization decision and visible result match the specified principal scope

    @py-7c4209f54ddd
    Scenario: Protected namespaces require explicit read grants
      Given the controlled domain state and principal described by this behavior
      When the actor performs: protected namespaces require explicit read grants
      Then the authorization decision and visible result match the specified principal scope

    @py-055c54e5d303
    Scenario: Rename preserves markdown source and code
      Given the controlled domain state and principal described by this behavior
      When the actor performs: rename preserves markdown source and code
      Then the response and durable state transition match the specified lifecycle

    @py-708a5e3a40b2
    Scenario: Repository audit reports duplicates and broken links
      Given the controlled domain state and principal described by this behavior
      When the actor performs: repository audit reports duplicates and broken links
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-557fce9b171a
    Scenario: Safe path containment rejects traversal symlink special and reserved
      Given the controlled domain state and principal described by this behavior
      When the actor performs: safe path containment rejects traversal symlink special and reserved
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-c9a534c038d3
    Scenario: Strict config and authorization
      Given the controlled domain state and principal described by this behavior
      When the actor performs: strict config and authorization
      Then the authorization decision and visible result match the specified principal scope

    @py-e2211d55a76a
    Scenario: Write path rejects dangling symlink
      Given the controlled domain state and principal described by this behavior
      When the actor performs: write path rejects dangling symlink
      Then the request is rejected at the specified boundary and prohibited state is unchanged

  Rule: Legacy blob migration

    @py-d48f5eb8a065
    Scenario: Migrates pointer and removes legacy filter attributes
      Given the controlled domain state and principal described by this behavior
      When the actor performs: migrates pointer and removes legacy filter attributes
      Then the response and durable state transition match the specified lifecycle

    @py-5a5283758d5b
    Scenario: Rejects cached blob with wrong digest
      Given the controlled domain state and principal described by this behavior
      When the actor performs: rejects cached blob with wrong digest
      Then the request is rejected at the specified boundary and prohibited state is unchanged
