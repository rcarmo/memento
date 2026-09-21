Feature: ux/admin access manager

  These scenarios describe the current browser behaviour of the maintained access manager.
  Scenario IDs stay stable. Automation and evidence mappings live in testdata/features/ux/scenario-inventory.json.

  Rule: authentication and credential handling

    @ux-admin-001
    Scenario: Opening the access manager requires an explicit administrator bearer for API actions
      Given the access manager page is reachable
      When a visitor attempts API-backed administration without an administrator bearer
      Then the page stays on the credential prompt or reports that an administrator bearer is required
      And ordinary admin API actions do not proceed

    @ux-admin-002
    Scenario: Admin bearer and one-time credentials stay in volatile tab state and are cleared on lock or close
      Given an operator has authenticated to the access manager
      When the operator locks the page or closes the one-time credential dialog
      Then the page clears the in-memory admin bearer, the rendered content and any revealed one-time credential text
      And stale replies from the previous session do not repopulate the page

    @ux-admin-003
    Scenario: Principals view shows lifecycle state, namespace scope and server warnings
      Given the access manager is open
      When an operator reviews the principals view
      Then each principal shows its name, roles and readable and writable namespace scope
      And lifecycle state and server warnings are surfaced when they apply

    @ux-admin-004
    Scenario: Create view can issue a principal from an editable preset access policy
      Given the access manager is open
      When an operator chooses a preset, edits the requested access and creates a principal
      Then the submitted principal reflects the requested roles and namespace prefixes
      And the create flow returns a one-time credential for the new principal

    @ux-admin-005
    Scenario: Create and rotate actions reveal a one-time credential bundle that can be copied once
      Given an operator has created or rotated a principal credential
      When the one-time credential dialog opens
      Then the page shows the bearer token together with a client configuration bundle
      And the operator can copy that bundle before the secret is cleared on close or Escape

  Rule: principal lifecycle and audit

    @ux-admin-006
    Scenario: Operators can rename, enable, disable, rotate, revoke and delete principals with confirmation
      Given the access manager lists an existing principal
      When an operator confirms a lifecycle action
      Then the principal reflects the requested renamed or enabled state
      And revoked or rotated credentials stop working according to that action's contract

    @ux-admin-007
    Scenario: Activity shows recent access-management events without revealing bearer secrets
      Given access-management actions have occurred
      When an operator opens the activity view
      Then the page lists recent actions with actor, target and timestamp information
      And the activity view does not reveal bearer credential plaintext

    @ux-admin-008
    Scenario: Action failures are visible and stale replies from an earlier session are ignored
      Given an access-management request can fail or outlive the current session
      When an operator triggers a failing action or locks the page before an older reply returns
      Then the current session shows a visible error for the failed action
      And the older reply does not restore content into the newer locked session

    @ux-admin-009
    Scenario: Create access preview shows effective read and write scope and admin risk warnings
      Given an operator is editing the create form
      When the requested roles or namespace prefixes change
      Then the page previews the resulting readable and writable scope
      And the page calls out administrator-level access before creation
