Feature: proposals/control and assets

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Control plane

    @py-2f2376db6fde
    Scenario: Control db migrations enable wal and v1 tables
      Given the controlled domain state and principal described by this behavior
      When the actor performs: control db migrations enable wal and v1 tables
      Then the response and durable state transition match the specified lifecycle

    @py-e411df339201
    Scenario: Empty repository bootstrap materializes and recovers
      Given the controlled domain state and principal described by this behavior
      When the actor performs: empty repository bootstrap materializes and recovers
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-59d07e7ab2d9
    Scenario: Idempotency replays per principal and rejects payload conflicts
      Given the controlled domain state and principal described by this behavior
      When the actor performs: idempotency replays per principal and rejects payload conflicts
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-c816f6c9b3c8
    Scenario: Materialize ignores persisted repository hooks
      Given the controlled domain state and principal described by this behavior
      When the actor performs: materialize ignores persisted repository hooks
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-36f83caaf941
    Scenario: Path commit timestamp comes from git history
      Given the controlled domain state and principal described by this behavior
      When the actor performs: path commit timestamp comes from git history
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-38c8091c0bd4
    Scenario: Startup recovery classifies publication after crash
      Given the controlled domain state and principal described by this behavior
      When the actor performs: startup recovery classifies publication after crash
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-9f9c9b9e6f55
    Scenario: Startup recovery uses committed diff not worktree scan
      Given the controlled domain state and principal described by this behavior
      When the actor performs: startup recovery uses committed diff not worktree scan
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-c2eb4651d12a
    Scenario: Transaction pipeline commits and materializes current
      Given the controlled domain state and principal described by this behavior
      When the actor performs: transaction pipeline commits and materializes current
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-2079a5320d07
    Scenario: Transaction pipeline rejects stale revision
      Given the controlled domain state and principal described by this behavior
      When the actor performs: transaction pipeline rejects stale revision
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-2d8cc9d860f5
    Scenario: Transaction pipeline retries failed operation with persisted id
      Given the controlled domain state and principal described by this behavior
      When the actor performs: transaction pipeline retries failed operation with persisted id
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-64620d46b08f
    Scenario: Transaction pipeline stages exact paths only
      Given the controlled domain state and principal described by this behavior
      When the actor performs: transaction pipeline stages exact paths only
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-372a5a46e62d
    Scenario: Unpublished crashes never recover as success
      Given the controlled domain state and principal described by this behavior
      When the actor performs: unpublished crashes never recover as success
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-9cee62241a77
    Scenario: Writer lease reports contention
      Given the controlled domain state and principal described by this behavior
      When the actor performs: writer lease reports contention
      Then the response and durable state transition match the specified lifecycle

  Rule: Operations

    @py-de660cc1f26e
    Scenario: Backup rejects destination inside state root
      Given the controlled domain state and principal described by this behavior
      When the actor performs: backup rejects destination inside state root
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-7c21dac2d72b
    Scenario: Backup restore and audit
      Given the controlled domain state and principal described by this behavior
      When the actor performs: backup restore and audit
      Then the response and durable state transition match the specified lifecycle

    @py-e687289248d7
    Scenario: Backup restore rejects manifest revision mismatch
      Given the controlled domain state and principal described by this behavior
      When the actor performs: backup restore rejects manifest revision mismatch
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-239b7ee150e2
    Scenario: Cli prometheus output is clean stdout
      Given the controlled domain state and principal described by this behavior
      When the actor performs: cli prometheus output is clean stdout
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-6e49361e52b6
    Scenario: Cli status and rebuild index
      Given the controlled domain state and principal described by this behavior
      When the actor performs: cli status and rebuild index
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-bbc01a67f402
    Scenario: Compose example uses env file and example env lists required tokens
      Given the controlled domain state and principal described by this behavior
      When the actor performs: compose example uses env file and example env lists required tokens
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-d1cf250c12ae
    Scenario: Config loading and composition root
      Given the controlled domain state and principal described by this behavior
      When the actor performs: config loading and composition root
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-de170a2a1cb7
    Scenario: Diskstation progressive embeddings are persistent and single threaded
      Given the controlled domain state and principal described by this behavior
      When the actor performs: diskstation progressive embeddings are persistent and single threaded
      Then the result and persisted state remain deterministic under concurrent execution

    @py-02679489aa89
    Scenario: Embedding refresh paths exclude asset and runtime changes
      Given the controlled domain state and principal described by this behavior
      When the actor performs: embedding refresh paths exclude asset and runtime changes
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-b78d75b0cb15
    Scenario: Graceful server drain
      Given the controlled domain state and principal described by this behavior
      When the actor performs: graceful server drain
      Then the result and persisted state remain deterministic under concurrent execution

    @py-a887b1154701
    Scenario: Metrics renderer
      Given the controlled domain state and principal described by this behavior
      When the actor performs: metrics renderer
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-a21f3f1c30b3
    Scenario: Restore refuses active writer
      Given the controlled domain state and principal described by this behavior
      When the actor performs: restore refuses active writer
      Then the response and durable state transition match the specified lifecycle

    @py-63692e1a04cc
    Scenario: Restore rejects backup source inside state root
      Given the controlled domain state and principal described by this behavior
      When the actor performs: restore rejects backup source inside state root
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-f91027a16d73
    Scenario: Restore requires mandatory checksums
      Given the controlled domain state and principal described by this behavior
      When the actor performs: restore requires mandatory checksums
      Then the response and durable state transition match the specified lifecycle

    @py-cab450579e96
    Scenario: Runtime close closes sqlite connection
      Given the controlled domain state and principal described by this behavior
      When the actor performs: runtime close closes sqlite connection
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-4f6689ce0bc6
    Scenario: Runtime loads and closes needle router
      Given the controlled domain state and principal described by this behavior
      When the actor performs: runtime loads and closes needle router
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-b0ec8f32b2e4
    Scenario: Runtime server uses writable state log
      Given the controlled domain state and principal described by this behavior
      When the actor performs: runtime server uses writable state log
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-00eb0d9be584
    Scenario: Structured logging redacts secrets
      Given the controlled domain state and principal described by this behavior
      When the actor performs: structured logging redacts secrets
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-519bd9251d22
    Scenario: Systemd timers are disabled by default for exclusive maintenance
      Given the controlled domain state and principal described by this behavior
      When the actor performs: systemd timers are disabled by default for exclusive maintenance
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-429e91866d7f
    Scenario: Systemd units use installed console script
      Given the controlled domain state and principal described by this behavior
      When the actor performs: systemd units use installed console script
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Proposal assets

    @py-d42b91c6870f
    Scenario: Create proposal rolls back when asset insert fails
      Given the controlled domain state and principal described by this behavior
      When the actor performs: create proposal rolls back when asset insert fails
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-08011a339581
    Scenario: Create proposal stores assets atomically
      Given the controlled domain state and principal described by this behavior
      When the actor performs: create proposal stores assets atomically
      Then the response and durable state transition match the specified lifecycle

    @py-061d9341936c
    Scenario: Migrate v5 skill pack proposals into generic assets
      Given the controlled domain state and principal described by this behavior
      When the actor performs: migrate v5 skill pack proposals into generic assets
      Then the response and durable state transition match the specified lifecycle

    @py-af71ea92db02
    Scenario: Migrate v5 skips unfeasible rows when proposal id is taken
      Given the controlled domain state and principal described by this behavior
      When the actor performs: migrate v5 skips unfeasible rows when proposal id is taken
      Then the response and durable state transition match the specified lifecycle
