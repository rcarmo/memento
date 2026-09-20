Feature: repository/assets and skills

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Asset migration

    @py-7b1533c71498
    Scenario: Migrate legacy skill pack to concept and generic asset
      Given the controlled domain state and principal described by this behavior
      When the actor performs: migrate legacy skill pack to concept and generic asset
      Then the response and durable state transition match the specified lifecycle

  Rule: Asset pack repository

    @py-4c11fcb34d53
    Scenario: Git stages generic asset as an ordinary blob
      Given the controlled domain state and principal described by this behavior
      When the actor performs: git stages generic asset as an ordinary blob
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-08aea3e5cd13
    Scenario: Immutable asset version
      Given the controlled domain state and principal described by this behavior
      When the actor performs: immutable asset version
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-e59813943efe
    Scenario: Write resolve and retention
      Given the controlled domain state and principal described by this behavior
      When the actor performs: write resolve and retention
      Then the response and durable state transition match the specified lifecycle

  Rule: Asset retrieval

    @py-f16d61b416d7
    Scenario: File digest checks bytes outside requested slice
      Given the controlled domain state and principal described by this behavior
      When the actor performs: file digest checks bytes outside requested slice
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-4a2b121af06b
    Scenario: File reads fail closed on zip manifest disagreement
      Given the controlled domain state and principal described by this behavior
      When the actor performs: file reads fail closed on zip manifest disagreement
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-235911583089
    Scenario: Invalid ranges
      Given the controlled domain state and principal described by this behavior
      When the actor performs: invalid ranges
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-c00694e7e18c
    Scenario: Manifest rejects duplicate and noncanonical entries
      Given the controlled domain state and principal described by this behavior
      When the actor performs: manifest rejects duplicate and noncanonical entries
      Then the request is rejected at the specified boundary and prohibited state is unchanged

  Rule: Skill import

    @py-1171a0a91e4e
    Scenario: Import cli
      Given the controlled domain state and principal described by this behavior
      When the actor performs: import cli
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-5bcaeca1e7d6
    Scenario: Import skill pack fails if destination exists
      Given the controlled domain state and principal described by this behavior
      When the actor performs: import skill pack fails if destination exists
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-1b1215c04e74
    Scenario: Import skill pack leaves no partial directory on validation failure
      Given the controlled domain state and principal described by this behavior
      When the actor performs: import skill pack leaves no partial directory on validation failure
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-2a2f338f60d2
    Scenario: Import skill pack rejects symlinked workspace parents
      Given the controlled domain state and principal described by this behavior
      When the actor performs: import skill pack rejects symlinked workspace parents
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-d180d3ff66b7
    Scenario: Import skill pack writes complete tree non executable
      Given the controlled domain state and principal described by this behavior
      When the actor performs: import skill pack writes complete tree non executable
      Then the response and durable state transition match the specified lifecycle

  Rule: Skill packs

    @py-29ed13f79582
    Scenario: Validate skill pack accepts non native binary payloads
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack accepts non native binary payloads
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-9930a0922467
    Scenario: Validate skill pack accepts valid archive and builds manifest
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack accepts valid archive and builds manifest
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-422d37d7ac28
    Scenario: Validate skill pack rejects duplicate paths
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects duplicate paths
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-e1be6e5b094f
    Scenario: Validate skill pack rejects encrypted entries
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects encrypted entries
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-5cf500afbd66
    Scenario: Validate skill pack rejects excessive compression ratio
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects excessive compression ratio
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-594aa61b8bed
    Scenario: Validate skill pack rejects file larger than limit
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects file larger than limit
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-1736f77d1746
    Scenario: Validate skill pack rejects invalid skill name
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects invalid skill name
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-9ac789451c25
    Scenario: Validate skill pack rejects invalid zip bytes
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects invalid zip bytes
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-971404697cb3
    Scenario: Validate skill pack rejects native binaries
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects native binaries
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-395af1e395f8
    Scenario: Validate skill pack rejects nested archives
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects nested archives
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-d88178b3047f
    Scenario: Validate skill pack rejects non stable semver
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects non stable semver
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-d8810f6ab3c1
    Scenario: Validate skill pack rejects raw zip larger than limit
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects raw zip larger than limit
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-c79249650c73
    Scenario: Validate skill pack rejects special file modes
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects special file modes
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-282ec979a8d2
    Scenario: Validate skill pack rejects symlinks
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects symlinks
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-99ae58c355dc
    Scenario: Validate skill pack rejects too many archive entries
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects too many archive entries
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-868b7d9f4b66
    Scenario: Validate skill pack rejects too many files
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects too many files
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-01803d78cc2b
    Scenario: Validate skill pack rejects total uncompressed larger than limit
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects total uncompressed larger than limit
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-7561308b42c1
    Scenario: Validate skill pack rejects unsafe paths
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack rejects unsafe paths
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-343ade46fd14
    Scenario: Validate skill pack requires exact skill md bytes
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack requires exact skill md bytes
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-2054b452779d
    Scenario: Validate skill pack requires root skill md
      Given the controlled domain state and principal described by this behavior
      When the actor performs: validate skill pack requires root skill md
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Staged assets

    @py-41a709f093e8
    Scenario: Proposal and stage consumption can share one transaction
      Given the controlled domain state and principal described by this behavior
      When the actor performs: proposal and stage consumption can share one transaction
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-bb864db91af6
    Scenario: Staging expiry removes blob
      Given the controlled domain state and principal described by this behavior
      When the actor performs: staging expiry removes blob
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-9b14a9a5ccbc
    Scenario: Staging http auth validation replay and status
      Given the controlled domain state and principal described by this behavior
      When the actor performs: staging http auth validation replay and status
      Then the authorization decision and visible result match the specified principal scope

    @py-f208f113048a
    Scenario: Staging http ticket upload requires no bearer
      Given the controlled domain state and principal described by this behavior
      When the actor performs: staging http ticket upload requires no bearer
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-82bb4884c532
    Scenario: Staging metadata headers are required
      Given the controlled domain state and principal described by this behavior
      When the actor performs: staging metadata headers are required
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-419698579aeb
    Scenario: Staging store is idempotent owned and consumable
      Given the controlled domain state and principal described by this behavior
      When the actor performs: staging store is idempotent owned and consumable
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-3981471230f7
    Scenario: Upload ticket is one time principal bound and reconcilable
      Given the controlled domain state and principal described by this behavior
      When the actor performs: upload ticket is one time principal bound and reconcilable
      Then the bounded result and continuation state match the specified contract
