Feature: models/semantic and embeddings

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Semantic

    @py-387a02dd622f
    Scenario: Asset only revision advances ready embeddings without recomputing
      Given the controlled domain state and principal described by this behavior
      When the actor performs: asset only revision advances ready embeddings without recomputing
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-5a5d05a4ce9c
    Scenario: Ctypes wrapper abi lifecycle info embed batch cancel and errors
      Given the controlled domain state and principal described by this behavior
      When the actor performs: ctypes wrapper abi lifecycle info embed batch cancel and errors
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-e05538a2833b
    Scenario: Ctypes wrapper rejects malformed model headers
      Given the controlled domain state and principal described by this behavior
      When the actor performs: ctypes wrapper rejects malformed model headers
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-aeb0c4ddce71
    Scenario: Embedding text is truncated to configured character limit
      Given the controlled domain state and principal described by this behavior
      When the actor performs: embedding text is truncated to configured character limit
      Then the bounded result and continuation state match the specified contract

    @py-038799913588
    Scenario: Failed sqlite extension load disables further extension loading
      Given the controlled domain state and principal described by this behavior
      When the actor performs: failed sqlite extension load disables further extension loading
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-bfbfdae87c2c
    Scenario: Fake embedder full rebuild incremental update delete and model invalidation
      Given the controlled domain state and principal described by this behavior
      When the actor performs: fake embedder full rebuild incremental update delete and model invalidation
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-be26cd6bef42
    Scenario: Hidden best match cannot change visible semantic or hybrid scores
      Given the controlled domain state and principal described by this behavior
      When the actor performs: hidden best match cannot change visible semantic or hybrid scores
      Then the authorization decision and visible result match the specified principal scope

    @py-88b7e20a237f
    Scenario: Lexical default unchanged and hybrid rrf is deterministic
      Given the controlled domain state and principal described by this behavior
      When the actor performs: lexical default unchanged and hybrid rrf is deterministic
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-04c534241825
    Scenario: Memory search api search mode and status warnings
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory search api search mode and status warnings
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-0eb0cebb5a71
    Scenario: Model info revision differs for same basename with different contents
      Given the controlled domain state and principal described by this behavior
      When the actor performs: model info revision differs for same basename with different contents
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-d339629a4a50
    Scenario: Python ctypes wrapper surfaces vector errors and sqlite vector matches python and rust
      Given the controlled domain state and principal described by this behavior
      When the actor performs: python ctypes wrapper surfaces vector errors and sqlite vector matches python and rust
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-0c1b609b8e45
    Scenario: Semantic search disabled or unavailable falls back to lexical
      Given the controlled domain state and principal described by this behavior
      When the actor performs: semantic search disabled or unavailable falls back to lexical
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-f240e6dd9915
    Scenario: Semantic status requires embedding rows for all concepts
      Given the controlled domain state and principal described by this behavior
      When the actor performs: semantic status requires embedding rows for all concepts
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-794b3b553cf0
    Scenario: Transaction succeeds when embedding update degrades but lexical advances
      Given the controlled domain state and principal described by this behavior
      When the actor performs: transaction succeeds when embedding update degrades but lexical advances
      Then the response and durable state transition match the specified lifecycle

  Rule: Semantic deferred

    @py-7901b01e67d9
    Scenario: Close interrupts database retry wait
      Given the controlled domain state and principal described by this behavior
      When the actor performs: close interrupts database retry wait
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-24ed18c6dc0d
    Scenario: Deferred semantic refresh lags and catches up
      Given the controlled domain state and principal described by this behavior
      When the actor performs: deferred semantic refresh lags and catches up
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-f426cbc7521b
    Scenario: Embedding refresh worker coalesces latest revision and close
      Given the controlled domain state and principal described by this behavior
      When the actor performs: embedding refresh worker coalesces latest revision and close
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-5b056fde050b
    Scenario: Failed priority path does not block progressive queue
      Given the controlled domain state and principal described by this behavior
      When the actor performs: failed priority path does not block progressive queue
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-4ad13b65741b
    Scenario: Model revision change marks persisted embeddings stale
      Given the controlled domain state and principal described by this behavior
      When the actor performs: model revision change marks persisted embeddings stale
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-99a636d90bff
    Scenario: Pause check failure does not drop selected work
      Given the controlled domain state and principal described by this behavior
      When the actor performs: pause check failure does not drop selected work
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-fbda8894567b
    Scenario: Pending paths skip degraded until manual or content change
      Given the controlled domain state and principal described by this behavior
      When the actor performs: pending paths skip degraded until manual or content change
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-2f66b2201bd9
    Scenario: Polling database lock recovers and status never queries sqlite
      Given the controlled domain state and principal described by this behavior
      When the actor performs: polling database lock recovers and status never queries sqlite
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-1880b8223676
    Scenario: Progressive worker pauses for activity load and pacing
      Given the controlled domain state and principal described by this behavior
      When the actor performs: progressive worker pauses for activity load and pacing
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-f33db94de747
    Scenario: Progressive worker resumes pending state and prioritizes manual paths
      Given the controlled domain state and principal described by this behavior
      When the actor performs: progressive worker resumes pending state and prioritizes manual paths
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-22bba7928bf8
    Scenario: Progressive worker waits for cpu sample before processing
      Given the controlled domain state and principal described by this behavior
      When the actor performs: progressive worker waits for cpu sample before processing
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-dc672c3209ac
    Scenario: Rebuild preserves ready embeddings and marks changed content stale
      Given the controlled domain state and principal described by this behavior
      When the actor performs: rebuild preserves ready embeddings and marks changed content stale
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-7b79581af215
    Scenario: Refresh embeddings batches full bundle
      Given the controlled domain state and principal described by this behavior
      When the actor performs: refresh embeddings batches full bundle
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-8859da0183d0
    Scenario: Runtime subprocess worker reports lag then catches up and closes
      Given the controlled domain state and principal described by this behavior
      When the actor performs: runtime subprocess worker reports lag then catches up and closes
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-053549d9bdaa
    Scenario: Status and enqueue remain responsive while polling blocks
      Given the controlled domain state and principal described by this behavior
      When the actor performs: status and enqueue remain responsive while polling blocks
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-0d43f69aa77b
    Scenario: Transient execution lock preserves request
      Given the controlled domain state and principal described by this behavior
      When the actor performs: transient execution lock preserves request
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-e8b106926127
    Scenario: Unexpected loop exit exposes dead worker
      Given the controlled domain state and principal described by this behavior
      When the actor performs: unexpected loop exit exposes dead worker
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Subprocess embeddings

    @py-722fa79ca3a2
    Scenario: Auto backend fallback is bounded and remembered
      Given the controlled domain state and principal described by this behavior
      When the actor performs: auto backend fallback is bounded and remembered
      Then the bounded result and continuation state match the specified contract

    @py-5827d21a5c49
    Scenario: Auto timeout does not exceed deadline and disables next gpu attempt
      Given the controlled domain state and principal described by this behavior
      When the actor performs: auto timeout does not exceed deadline and disables next gpu attempt
      Then cancellation or timeout preserves the specified completion and reconciliation state

    @py-eb9219b297d8
    Scenario: Explicit vulkan failure does not fall back
      Given the controlled domain state and principal described by this behavior
      When the actor performs: explicit vulkan failure does not fall back
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-d30d82402e70
    Scenario: Malformed metadata auto falls back without conversion errors
      Given the controlled domain state and principal described by this behavior
      When the actor performs: malformed metadata auto falls back without conversion errors
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-f0ae4fd6e969
    Scenario: Subprocess embedding client batches and exits
      Given the controlled domain state and principal described by this behavior
      When the actor performs: subprocess embedding client batches and exits
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-08ee38d49afa
    Scenario: Subprocess embedding client cancellation
      Given the controlled domain state and principal described by this behavior
      When the actor performs: subprocess embedding client cancellation
      Then cancellation or timeout preserves the specified completion and reconciliation state

    @py-e01644608815
    Scenario: Subprocess embedding client enforces limits and errors
      Given the controlled domain state and principal described by this behavior
      When the actor performs: subprocess embedding client enforces limits and errors
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-9eb9286cd3bc
    Scenario: Subprocess embedding client uses low priority single thread environment
      Given the controlled domain state and principal described by this behavior
      When the actor performs: subprocess embedding client uses low priority single thread environment
      Then the result and persisted state remain deterministic under concurrent execution

    @py-df1d8c772b68
    Scenario: Vulkan configuration is opt in and requires subprocess
      Given the controlled domain state and principal described by this behavior
      When the actor performs: vulkan configuration is opt in and requires subprocess
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Warm subprocess embeddings

    @py-3c21905a12f0
    Scenario: Default configuration remains cold
      Given the controlled domain state and principal described by this behavior
      When the actor performs: default configuration remains cold
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-0f59da364b42
    Scenario: Environment change recycles worker
      Given the controlled domain state and principal described by this behavior
      When the actor performs: environment change recycles worker
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-00811f639c68
    Scenario: Failed reap is retried not reused
      Given the controlled domain state and principal described by this behavior
      When the actor performs: failed reap is retried not reused
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-a1fcc2c94002
    Scenario: Fallback shares original deadline
      Given the controlled domain state and principal described by this behavior
      When the actor performs: fallback shares original deadline
      Then cancellation or timeout preserves the specified completion and reconciliation state

    @py-81cd0af0d15b
    Scenario: Queued deadline does not discard active worker
      Given the controlled domain state and principal described by this behavior
      When the actor performs: queued deadline does not discard active worker
      Then cancellation or timeout preserves the specified completion and reconciliation state

    @py-3647c9e21dfa
    Scenario: Warm client active request can outlive idle window
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client active request can outlive idle window
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-a0cd53eb2c29
    Scenario: Warm client auto backend falls back to cpu and remembers failure
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client auto backend falls back to cpu and remembers failure
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-2dc9ab47d357
    Scenario: Warm client auto backend falls back when selected type is malformed
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client auto backend falls back when selected type is malformed
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-b2b87c94f2e9
    Scenario: Warm client bad responses drop process and recover
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client bad responses drop process and recover
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-4115d2858113
    Scenario: Warm client cancels active call and recovers
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client cancels active call and recovers
      Then cancellation or timeout preserves the specified completion and reconciliation state

    @py-2d2bbb6c5fe9
    Scenario: Warm client cancels queued call without killing active request
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client cancels queued call without killing active request
      Then cancellation or timeout preserves the specified completion and reconciliation state

    @py-1c75fb7c4a93
    Scenario: Warm client close during active request aborts and reaps worker
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client close during active request aborts and reaps worker
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-97cd7e2fb9ee
    Scenario: Warm client close is idempotent and blocks future use
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client close is idempotent and blocks future use
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-cf672d4af0c4
    Scenario: Warm client drains stderr without deadlock
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client drains stderr without deadlock
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-2804a49541ff
    Scenario: Warm client expires after idle and restarts
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client expires after idle and restarts
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-285ac528e8b5
    Scenario: Warm client explicit vulkan failure does not fall back
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client explicit vulkan failure does not fall back
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-25de89fc5f00
    Scenario: Warm client handles partial pipe reads and writes
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client handles partial pipe reads and writes
      Then the response and durable state transition match the specified lifecycle

    @py-61c2a96f03dd
    Scenario: Warm client missing worker executable fails cleanly
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client missing worker executable fails cleanly
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-8ec77a95caa3
    Scenario: Warm client restarts after request count cap
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client restarts after request count cap
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-c65ab1db00d6
    Scenario: Warm client reuses same process and updates counters
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client reuses same process and updates counters
      Then the response and durable state transition match the specified lifecycle

    @py-f158147f2538
    Scenario: Warm client serialises concurrent calls and returns unique vectors
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client serialises concurrent calls and returns unique vectors
      Then the result and persisted state remain deterministic under concurrent execution

    @py-17f23e1a3383
    Scenario: Warm client times out on hung worker and recovers
      Given the controlled domain state and principal described by this behavior
      When the actor performs: warm client times out on hung worker and recovers
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Warm worker config

    @py-b014b909f53b
    Scenario: Build runtime passes worker idle seconds to subprocess client
      Given the controlled domain state and principal described by this behavior
      When the actor performs: build runtime passes worker idle seconds to subprocess client
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-dc13e65e6fea
    Scenario: Worker idle seconds accepts bounds
      Given the controlled domain state and principal described by this behavior
      When the actor performs: worker idle seconds accepts bounds
      Then the bounded result and continuation state match the specified contract

    @py-83bf9d852e17
    Scenario: Worker idle seconds defaults to zero without changing cpu backend
      Given the controlled domain state and principal described by this behavior
      When the actor performs: worker idle seconds defaults to zero without changing cpu backend
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-08e67d44592e
    Scenario: Worker idle seconds rejects invalid values
      Given the controlled domain state and principal described by this behavior
      When the actor performs: worker idle seconds rejects invalid values
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-d4c518aed500
    Scenario: Worker idle seconds requires subprocess mode when positive
      Given the controlled domain state and principal described by this behavior
      When the actor performs: worker idle seconds requires subprocess mode when positive
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Cpu usage

    @py-35ee2225e458
    Scenario: Cpu sampler recovers from counter reset
      Given the controlled domain state and principal described by this behavior
      When the actor performs: cpu sampler recovers from counter reset
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-76d6ae2b72cb
    Scenario: Cpu sampler reports busy percent over window
      Given the controlled domain state and principal described by this behavior
      When the actor performs: cpu sampler reports busy percent over window
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-e0a85ab78007
    Scenario: Cpu sampler treats iowait as idle
      Given the controlled domain state and principal described by this behavior
      When the actor performs: cpu sampler treats iowait as idle
      Then the observable response, ordering, warnings and resulting state match the specified contract
