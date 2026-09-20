Feature: access/managed access

  The scenarios capture Python Memento behavior at 7f29e8b003557f0105f47ed353b7f65a33619456.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_access.py

    @py-1380ae25e6c2 @python_test_bootstrap_renames_initial_admin_and_authenticates @go_TestAuthorizationReference @go_TestManagedAccessReference
    Scenario: test_bootstrap_renames_initial_admin_and_authenticates
      Given the pinned Python reference fixtures and controlled inputs
      When store(tmp_path); access.bootstrap( authorization(), {"piclaw-workspace": "sandbox-token", "work-agent": "work-token"} ); access.list().
      Then sandbox.roles == ('admin', 'curator', 'proposer', 'reader'); sandbox_auth is not None and sandbox_auth.name == 'sandbox'; work_auth is not None and work_auth.name == 'work-agent'; access.authenticate('wrong') is None.

    @py-7817571ef4e3 @python_test_create_returns_one_time_token_and_validates_scope @go_TestAuthorizationReference @go_TestManagedAccessReference
    Scenario: test_create_returns_one_time_token_and_validates_scope
      Given the pinned Python reference fixtures and controlled inputs
      When store(tmp_path); access.create( actor="sandbox", name="gates", roles=("reader", "proposer"), read_prefixes=("/work/", "/skills/"), write_prefixes=("/work/",…; access.authenticate(token).
      Then principal.name == 'gates'; token starts with 'memento_'; authenticated is not None and authenticated.name == 'gates'; rotated != token. expects access.rotate(actor="sandbox", name="gates", idempotency_key="rotate-gates-1") raises AccessError matching "one-time credential cannot be replayed"; access.create( actor="sandbox", name="bad", roles=("reader",), read_prefixes=("/skills/",), write_prefixes=("/work/",), ) raises AccessError matching "inside a readable".

    @py-78754534ea44 @python_test_lifecycle_and_last_admin_guard @go_TestAuthorizationReference @go_TestManagedAccessReference
    Scenario: test_lifecycle_and_last_admin_guard
      Given the pinned Python reference fixtures and controlled inputs
      When store(tmp_path); access.bootstrap(authorization(), {"piclaw-workspace": "sandbox-token"}); access.create( actor="sandbox", name="second-admin", roles=("reader", "admin"), read_prefixes=("/",), write_prefixes=(), ).
      Then 'admin' in created.roles; disabled.enabled is False; revoked.revoked is True; deleted.deleted is True; access.authenticate('sandbox-token') is None; [event['action'] for event in access.audit()] == ['principal.delete', 'credential.revoke', 'principal.disable', 'principal.create']. expects access.set_enabled(actor="sandbox", name="sandbox", enabled=False) raises AccessError matching "final enabled admin".

    @py-1b4338fe4a5a @python_test_managed_principal_policy_inherits_protected_namespaces @go_TestAuthorizationReference @go_TestManagedAccessReference
    Scenario: test_managed_principal_policy_inherits_protected_namespaces
      Given the pinned Python reference fixtures and controlled inputs
      When resolve_policy(authorization, principal). expects authorize_path(policy, "/personal/rui.md", action="read") raises AuthorizationError.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-81566e51cdcb @python_test_master_key_rotation_preserves_credentials @go_TestAuthorizationReference @go_TestManagedAccessReference
    Scenario: test_master_key_rotation_preserves_credentials
      Given the pinned Python reference fixtures and controlled inputs
      When store(tmp_path); access.bootstrap(authorization(), {"piclaw-workspace": "sandbox-token"}); access.rotate_master_key("nenhuma", "stronger-key").
      Then principal is not None and principal.name == 'sandbox'. expects AccessStore(connection, "nenhuma") raises AccessError matching "invalid".

  Rule: Behavior captured from test_admin_http.py

    @py-7cbcbcbe6d53 @python_test_admin_create_returns_one_time_credential @go_TestAdminHTTPHandleCRUDAndActivity @go_TestAdminHTTPThroughUMCP
    Scenario: test_admin_create_returns_one_time_credential
      Given the pinned Python reference fixtures and controlled inputs
      When handler(tmp_path); request( http, "POST", "/admin/api/principals", "admin-token", { "name": "gates", "roles": ["reader", "proposer"], "read_prefixes": ["/work…; request(http, "GET", "/admin/api/principals", "admin-token").
      Then response.status == 201; payload is not None; payload['principal']['name'] == 'gates'; payload['credential'] starts with 'memento_'; listed.status == 200; list_payload is not None.

    @py-980566bf5992 @python_test_admin_page_and_api_auth @go_TestAdminHTTPHandleCRUDAndActivity @go_TestAdminHTTPThroughUMCP
    Scenario: test_admin_page_and_api_auth
      Given the pinned Python reference fixtures and controlled inputs
      When handler(tmp_path); request(http, "GET", "/admin"); request(http, "GET", "/admin/api/principals", "reader-token").
      Then page.status == 200; b'Memento Access' in page.body; unauthorized.status == 401; allowed.status == 200; payload is not None; any((item['name'] == 'sandbox' for item in payload['principals'])).
