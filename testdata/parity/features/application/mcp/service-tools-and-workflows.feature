Feature: mcp/service tools and workflows

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Service mcp

    @py-ef5fe61f75d4
    Scenario: Accepted archive ranges resume and verify digest
      Given the controlled domain state and principal described by this behavior
      When the actor performs: accepted archive ranges resume and verify digest
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-d824fa8e91d1
    Scenario: Accepted asset symlink containment
      Given the controlled domain state and principal described by this behavior
      When the actor performs: accepted asset symlink containment
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-07d799fcd17e
    Scenario: Accepted file chunks match proposal reads
      Given the controlled domain state and principal described by this behavior
      When the actor performs: accepted file chunks match proposal reads
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-b649a725b7cd
    Scenario: Accepted manifest does not read zip and execute preserves entries
      Given the controlled domain state and principal described by this behavior
      When the actor performs: accepted manifest does not read zip and execute preserves entries
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-7c949b0a46ae
    Scenario: Accepted metadata digest mismatch and invalid file range
      Given the controlled domain state and principal described by this behavior
      When the actor performs: accepted metadata digest mismatch and invalid file range
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-b6347816535a
    Scenario: Accepted reads detect corruption and pruned payloads
      Given the controlled domain state and principal described by this behavior
      When the actor performs: accepted reads detect corruption and pruned payloads
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-7defada3eccc
    Scenario: Accepted version pinning trash and pruned versions
      Given the controlled domain state and principal described by this behavior
      When the actor performs: accepted version pinning trash and pruned versions
      Then the response and durable state transition match the specified lifecycle

    @py-f5a69c7ad35f
    Scenario: Accepted views enforce authorisation ranges and safe paths
      Given the controlled domain state and principal described by this behavior
      When the actor performs: accepted views enforce authorisation ranges and safe paths
      Then the authorization decision and visible result match the specified principal scope

    @py-cce57314174e
    Scenario: Archival impact filters hidden and revoked backlinks
      Given the controlled domain state and principal described by this behavior
      When the actor performs: archival impact filters hidden and revoked backlinks
      Then the response and durable state transition match the specified lifecycle

    @py-d11d4f60571a
    Scenario: Archival impact retains asset versions and scopes references
      Given the controlled domain state and principal described by this behavior
      When the actor performs: archival impact retains asset versions and scopes references
      Then the authorization decision and visible result match the specified principal scope

    @py-44f46c291fa1
    Scenario: Archival proposal rejects mixed duplicate and stale requests
      Given the controlled domain state and principal described by this behavior
      When the actor performs: archival proposal rejects mixed duplicate and stale requests
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-0158b4bc1fce
    Scenario: Asset get direct mcp supports manifest view
      Given the controlled domain state and principal described by this behavior
      When the actor performs: asset get direct mcp supports manifest view
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-fabe514fae5c
    Scenario: Asset limits are discoverable
      Given the controlled domain state and principal described by this behavior
      When the actor performs: asset limits are discoverable
      Then the bounded result and continuation state match the specified contract

    @py-ed8dfb0c386d
    Scenario: Asset metadata is execute only with service execute parity
      Given the controlled domain state and principal described by this behavior
      When the actor performs: asset metadata is execute only with service execute parity
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-b4ae0623c2d7
    Scenario: Asset metadata prunes protected paths before parsing and pages batches
      Given the controlled domain state and principal described by this behavior
      When the actor performs: asset metadata prunes protected paths before parsing and pages batches
      Then the bounded result and continuation state match the specified contract

    @py-042d41615fd1
    Scenario: Asset metadata returns generic versions files timestamps and skill parity
      Given the controlled domain state and principal described by this behavior
      When the actor performs: asset metadata returns generic versions files timestamps and skill parity
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-5d055b2783e5
    Scenario: Asset metadata validates scope bounds and persisted metadata
      Given the controlled domain state and principal described by this behavior
      When the actor performs: asset metadata validates scope bounds and persisted metadata
      Then the bounded result and continuation state match the specified contract

    @py-a83350d600e1
    Scenario: Asset pack skill lifecycle uses generic propose review apply and get
      Given the controlled domain state and principal described by this behavior
      When the actor performs: asset pack skill lifecycle uses generic propose review apply and get
      Then the response and durable state transition match the specified lifecycle

    @py-bed0eb54c9e1
    Scenario: Asset pack tool discovery and catalog schemas
      Given the controlled domain state and principal described by this behavior
      When the actor performs: asset pack tool discovery and catalog schemas
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-a7df6d6eb4bd
    Scenario: Audit graph diagnostics are role scoped paginated and stale safe
      Given the controlled domain state and principal described by this behavior
      When the actor performs: audit graph diagnostics are role scoped paginated and stale safe
      Then the authorization decision and visible result match the specified principal scope

    @py-b3a708f2bf63
    Scenario: Audit skips protected content and broken targets
      Given the controlled domain state and principal described by this behavior
      When the actor performs: audit skips protected content and broken targets
      Then the authorization decision and visible result match the specified principal scope

    @py-7b8fe16b592e
    Scenario: Auth visibility and standard envelopes
      Given the controlled domain state and principal described by this behavior
      When the actor performs: auth visibility and standard envelopes
      Then the authorization decision and visible result match the specified principal scope

    @py-0c04076f6f57
    Scenario: Commit operation reconciliation is principal scoped and actionable
      Given the controlled domain state and principal described by this behavior
      When the actor performs: commit operation reconciliation is principal scoped and actionable
      Then the authorization decision and visible result match the specified principal scope

    @py-1d1667f6b959
    Scenario: Compare manifest classifies generic entries and assets
      Given the controlled domain state and principal described by this behavior
      When the actor performs: compare manifest classifies generic entries and assets
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-044c92f8cb9c
    Scenario: Compare manifest enforces scope and bounds before content access
      Given the controlled domain state and principal described by this behavior
      When the actor performs: compare manifest enforces scope and bounds before content access
      Then the bounded result and continuation state match the specified contract

    @py-f036a7fd7333
    Scenario: Concurrent rebase and reject leave proposal rejected
      Given the controlled domain state and principal described by this behavior
      When the actor performs: concurrent rebase and reject leave proposal rejected
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-4b3ecc72a9e5
    Scenario: Conflicted deleted target remains reviewable
      Given the controlled domain state and principal described by this behavior
      When the actor performs: conflicted deleted target remains reviewable
      Then the response and durable state transition match the specified lifecycle

    @py-9f20f634bfa8
    Scenario: Conflicted proposal detailed view survives deleted target
      Given the controlled domain state and principal described by this behavior
      When the actor performs: conflicted proposal detailed view survives deleted target
      Then the response and durable state transition match the specified lifecycle

    @py-c7b59ea7aa15
    Scenario: Control schema upgrade keeps proposal review and assets
      Given the controlled domain state and principal described by this behavior
      When the actor performs: control schema upgrade keeps proposal review and assets
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-518cdb5a3472
    Scenario: Direct mutations warn and preserve generic asset parity
      Given the controlled domain state and principal described by this behavior
      When the actor performs: direct mutations warn and preserve generic asset parity
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-e8d3521ccfdb
    Scenario: Direct rename rewrites inbound links atomically
      Given the controlled domain state and principal described by this behavior
      When the actor performs: direct rename rewrites inbound links atomically
      Then the bounded result and continuation state match the specified contract

    @py-f6fb8189c0d2
    Scenario: Execute applies and replays status only patches
      Given the controlled domain state and principal described by this behavior
      When the actor performs: execute applies and replays status only patches
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-8ad3bf33eb1c
    Scenario: Execute bad projection preserves committed result
      Given the controlled domain state and principal described by this behavior
      When the actor performs: execute bad projection preserves committed result
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-91473d93111d
    Scenario: Execute chains typed asset offsets and digest references
      Given the controlled domain state and principal described by this behavior
      When the actor performs: execute chains typed asset offsets and digest references
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-69da15129e23
    Scenario: Execute deadline without commit remains an error
      Given the controlled domain state and principal described by this behavior
      When the actor performs: execute deadline without commit remains an error
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-aa388d0629f0
    Scenario: Execute default asset chunk fits default budget
      Given the controlled domain state and principal described by this behavior
      When the actor performs: execute default asset chunk fits default budget
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-445dad67cce9
    Scenario: Execute limits auth and error control
      Given the controlled domain state and principal described by this behavior
      When the actor performs: execute limits auth and error control
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-f3d31c7d1d81
    Scenario: Execute rebase preserves control commit on later reference error
      Given the controlled domain state and principal described by this behavior
      When the actor performs: execute rebase preserves control commit on later reference error
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-aa8a2c039bbe
    Scenario: Execute reference schema does not loosen direct asset schema
      Given the controlled domain state and principal described by this behavior
      When the actor performs: execute reference schema does not loosen direct asset schema
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-4ff1245d5d0f
    Scenario: Execute rejects invalid references and multiple commit ops
      Given the controlled domain state and principal described by this behavior
      When the actor performs: execute rejects invalid references and multiple commit ops
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-efcac34f76d8
    Scenario: Execute reports success when deadline expires after commit
      Given the controlled domain state and principal described by this behavior
      When the actor performs: execute reports success when deadline expires after commit
      Then cancellation or timeout preserves the specified completion and reconciliation state

    @py-dc481f58abe3
    Scenario: Execute search read and projection
      Given the controlled domain state and principal described by this behavior
      When the actor performs: execute search read and projection
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-38f0bd623cf9
    Scenario: Execute tool schema and normalization support both argument forms
      Given the controlled domain state and principal described by this behavior
      When the actor performs: execute tool schema and normalization support both argument forms
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-c2d8e1cf7acd
    Scenario: Execute worker does not block event loop
      Given the controlled domain state and principal described by this behavior
      When the actor performs: execute worker does not block event loop
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-0b6681a36f1d
    Scenario: Explicit creates enforce concept invariants
      Given the controlled domain state and principal described by this behavior
      When the actor performs: explicit creates enforce concept invariants
      Then the response and durable state transition match the specified lifecycle

    @py-f3daf49b8473
    Scenario: Generic asset proposal rejects duplicate and rename mix
      Given the controlled domain state and principal described by this behavior
      When the actor performs: generic asset proposal rejects duplicate and rename mix
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-e410a0a750bd
    Scenario: Invalid saved references and limits are compact
      Given the controlled domain state and principal described by this behavior
      When the actor performs: invalid saved references and limits are compact
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-af0eb3f6906a
    Scenario: Legacy stale clean rebase does not renew expiry
      Given the controlled domain state and principal described by this behavior
      When the actor performs: legacy stale clean rebase does not renew expiry
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-2088d641c243
    Scenario: Managed access instructions and tool descriptions
      Given the controlled domain state and principal described by this behavior
      When the actor performs: managed access instructions and tool descriptions
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-08d5630482ae
    Scenario: Mcp asset stage ticket and status bridge
      Given the controlled domain state and principal described by this behavior
      When the actor performs: mcp asset stage ticket and status bridge
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-b0335a8dca2d
    Scenario: Memory graph serializes typed edges and preserves scope
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory graph serializes typed edges and preserves scope
      Then the authorization decision and visible result match the specified principal scope

    @py-df51d56f8f7f
    Scenario: Memory inventory filters protected namespaces before parsing
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory inventory filters protected namespaces before parsing
      Then the authorization decision and visible result match the specified principal scope

    @py-71fd4c793629
    Scenario: Memory inventory is available directly and via execute
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory inventory is available directly and via execute
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-d4945ffb9f77
    Scenario: Memory inventory parses only the bounded page
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory inventory parses only the bounded page
      Then the bounded result and continuation state match the specified contract

    @py-c84e7b9f4cef
    Scenario: Memory inventory returns stable digests assets and pagination
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory inventory returns stable digests assets and pagination
      Then the bounded result and continuation state match the specified contract

    @py-552c058afa37
    Scenario: Memory list includes light frontmatter metadata
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory list includes light frontmatter metadata
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-db18374da9f9
    Scenario: Memory proposal list filters status in control query
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory proposal list filters status in control query
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-28509b3f6159
    Scenario: Memory route direct execute unknown auth and malformed
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory route direct execute unknown auth and malformed
      Then the authorization decision and visible result match the specified principal scope

    @py-ccbebd4c7992
    Scenario: Memory route disabled and server discovery
      Given the controlled domain state and principal described by this behavior
      When the actor performs: memory route disabled and server discovery
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-933d667bd223
    Scenario: Operation lookup rechecks after writer finishes
      Given the controlled domain state and principal described by this behavior
      When the actor performs: operation lookup rechecks after writer finishes
      Then the response and durable state transition match the specified lifecycle

    @py-1ccc89061326
    Scenario: Original proposer rebase preserves identity and reconciles
      Given the controlled domain state and principal described by this behavior
      When the actor performs: original proposer rebase preserves identity and reconciles
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-cf4da8f06ec2
    Scenario: Plan errors do not dump union or arguments
      Given the controlled domain state and principal described by this behavior
      When the actor performs: plan errors do not dump union or arguments
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-2af5d35547a0
    Scenario: Proposal lifecycle self approval stale apply and idempotency
      Given the controlled domain state and principal described by this behavior
      When the actor performs: proposal lifecycle self approval stale apply and idempotency
      Then the response and durable state transition match the specified lifecycle

    @py-84bae50d75d9
    Scenario: Proposal list visibility and expiry
      Given the controlled domain state and principal described by this behavior
      When the actor performs: proposal list visibility and expiry
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-b40d07b7b416
    Scenario: Proposal rebase catalog is execute only
      Given the controlled domain state and principal described by this behavior
      When the actor performs: proposal rebase catalog is execute only
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-225024aa415e
    Scenario: Proposal review keeps role and write scope checks
      Given the controlled domain state and principal described by this behavior
      When the actor performs: proposal review keeps role and write scope checks
      Then the authorization decision and visible result match the specified principal scope

    @py-32f890f6a10c
    Scenario: Proposal summary pagination and bounded asset inspection
      Given the controlled domain state and principal described by this behavior
      When the actor performs: proposal summary pagination and bounded asset inspection
      Then the bounded result and continuation state match the specified contract

    @py-53d48c5b0373
    Scenario: Protected proposals are hidden without explicit read access
      Given the controlled domain state and principal described by this behavior
      When the actor performs: protected proposals are hidden without explicit read access
      Then the authorization decision and visible result match the specified principal scope

    @py-200558c704c6
    Scenario: Pruning versions removes all read views
      Given the controlled domain state and principal described by this behavior
      When the actor performs: pruning versions removes all read views
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-b85bf0910ee4
    Scenario: Purge removes accepted assets and restore collision is safe
      Given the controlled domain state and principal described by this behavior
      When the actor performs: purge removes accepted assets and restore collision is safe
      Then the response and durable state transition match the specified lifecycle

    @py-b9e10d408da6
    Scenario: Rebase control transaction rolls back and can retry
      Given the controlled domain state and principal described by this behavior
      When the actor performs: rebase control transaction rolls back and can retry
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-b08f8b55f946
    Scenario: Rebase keeps asset bytes without upload
      Given the controlled domain state and principal described by this behavior
      When the actor performs: rebase keeps asset bytes without upload
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-fce6b15be514
    Scenario: Rebase permissions expiry and legacy state
      Given the controlled domain state and principal described by this behavior
      When the actor performs: rebase permissions expiry and legacy state
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-4211305a7ad3
    Scenario: Rebase retains skill root body parity
      Given the controlled domain state and principal described by this behavior
      When the actor performs: rebase retains skill root body parity
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-82982893535b
    Scenario: Rebase worker timeout reconciles original key after commit
      Given the controlled domain state and principal described by this behavior
      When the actor performs: rebase worker timeout reconciles original key after commit
      Then cancellation or timeout preserves the specified completion and reconciliation state

    @py-14f4f0f26ede
    Scenario: Rename rejects unauthorised backlink rewrites
      Given the controlled domain state and principal described by this behavior
      When the actor performs: rename rejects unauthorised backlink rewrites
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-d305400c6e5f
    Scenario: Resolved integer types and bounds fail before dispatch
      Given the controlled domain state and principal described by this behavior
      When the actor performs: resolved integer types and bounds fail before dispatch
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-74352392a175
    Scenario: Resolved references cannot widen authorisation
      Given the controlled domain state and principal described by this behavior
      When the actor performs: resolved references cannot widen authorisation
      Then the authorization decision and visible result match the specified principal scope

    @py-da021852fc90
    Scenario: Reviewed archival rejects collision and oversized impact
      Given the controlled domain state and principal described by this behavior
      When the actor performs: reviewed archival rejects collision and oversized impact
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-e00d6cf2fb83
    Scenario: Reviewed archival reports backlinks and replays
      Given the controlled domain state and principal described by this behavior
      When the actor performs: reviewed archival reports backlinks and replays
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-eb268fd71c02
    Scenario: Saved values revalidated in nested arrays and booleans
      Given the controlled domain state and principal described by this behavior
      When the actor performs: saved values revalidated in nested arrays and booleans
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-f62307f2af31
    Scenario: Server rejects duplicate principal names
      Given the controlled domain state and principal described by this behavior
      When the actor performs: server rejects duplicate principal names
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-fc5894882d44
    Scenario: Skill asset proposal rejects noncanonical or mismatched root
      Given the controlled domain state and principal described by this behavior
      When the actor performs: skill asset proposal rejects noncanonical or mismatched root
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-b7ac71a2d989
    Scenario: Staged skill asset is consumed by proposal
      Given the controlled domain state and principal described by this behavior
      When the actor performs: staged skill asset is consumed by proposal
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-ae400645895a
    Scenario: Stale filter uses effective status with bounded query
      Given the controlled domain state and principal described by this behavior
      When the actor performs: stale filter uses effective status with bounded query
      Then the bounded result and continuation state match the specified contract

    @py-fcdf5faf0c11
    Scenario: Stale proposal conflicts and safe subset revision
      Given the controlled domain state and principal described by this behavior
      When the actor performs: stale proposal conflicts and safe subset revision
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-96e4b29042cd
    Scenario: Status reports canonical state when derived index is empty
      Given the controlled domain state and principal described by this behavior
      When the actor performs: status reports canonical state when derived index is empty
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-5aef6eaa1153
    Scenario: Streamable http delivers subscribed notifications after reconnect
      Given the controlled domain state and principal described by this behavior
      When the actor performs: streamable http delivers subscribed notifications after reconnect
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-f66614d9be30
    Scenario: Streamable http explains invalid protocol version
      Given the controlled domain state and principal described by this behavior
      When the actor performs: streamable http explains invalid protocol version
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-7eea25a29054
    Scenario: Streamable http reuses post connection and preserves session
      Given the controlled domain state and principal described by this behavior
      When the actor performs: streamable http reuses post connection and preserves session
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-fe236a638649
    Scenario: Tool discovery surfaces and catalog resources
      Given the controlled domain state and principal described by this behavior
      When the actor performs: tool discovery surfaces and catalog resources
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-20679ee7b9a4
    Scenario: Trash graph view filters original acl
      Given the controlled domain state and principal described by this behavior
      When the actor performs: trash graph view filters original acl
      Then the response and durable state transition match the specified lifecycle

    @py-8667ecc6d7f9
    Scenario: Trash move indexes destination before lexically later source
      Given the controlled domain state and principal described by this behavior
      When the actor performs: trash move indexes destination before lexically later source
      Then the response and durable state transition match the specified lifecycle

    @py-03634d5bcfea
    Scenario: Trash restore purge retains history and original permissions
      Given the controlled domain state and principal described by this behavior
      When the actor performs: trash restore purge retains history and original permissions
      Then the response and durable state transition match the specified lifecycle

    @py-930874159404
    Scenario: Typed reference failure after commit preserves operation
      Given the controlled domain state and principal described by this behavior
      When the actor performs: typed reference failure after commit preserves operation
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-9ba0c7e3143d
    Scenario: Typed reference from eof is rejected without restarting download
      Given the controlled domain state and principal described by this behavior
      When the actor performs: typed reference from eof is rejected without restarting download
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-50e9e93a20f9
    Scenario: Typed references pass direct mcp plan boundary
      Given the controlled domain state and principal described by this behavior
      When the actor performs: typed references pass direct mcp plan boundary
      Then the bounded result and continuation state match the specified contract

    @py-0fb4f5d003c2
    Scenario: Unavailable historical base does not hide queue
      Given the controlled domain state and principal described by this behavior
      When the actor performs: unavailable historical base does not hide queue
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-c5636f092251
    Scenario: Unrelated commit keeps proposal reviewable and audited
      Given the controlled domain state and principal described by this behavior
      When the actor performs: unrelated commit keeps proposal reviewable and audited
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-1706a62736f0
    Scenario: Worker commit reconciliation and shutdown
      Given the controlled domain state and principal described by this behavior
      When the actor performs: worker commit reconciliation and shutdown
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-f514bf70de47
    Scenario: Worker rebase replay and concurrent applies
      Given the controlled domain state and principal described by this behavior
      When the actor performs: worker rebase replay and concurrent applies
      Then the result and persisted state remain deterministic under concurrent execution
