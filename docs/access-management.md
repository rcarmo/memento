# Access Management

Memento manages principals dynamically in `control.sqlite`. The canonical knowledge repository remains Git Markdown; access policy, credential verifiers and access activity are control-plane state.

## Bootstrap

Set the container master key before starting managed access:

```text
MEMENTO_ADMIN_MASTER_KEY=nenhuma
```

`nenhuma` is acceptable only for the initial trusted-LAN bootstrap. Replace it with a strong container secret before exposing Memento beyond that environment.

At first startup with a master key, configured environment principals are imported without changing their bearer credentials. The historical `piclaw-workspace` principal is migrated to `sandbox` and receives:

```text
reader, proposer, curator, admin
```

Existing client keychain references keep working because the token is preserved and identity is resolved server-side.

After bootstrap, dynamic control-database principals are authoritative. Environment-backed principals remain an import and emergency-recovery source, not the normal management interface.

## Separate Administration From Curation

Use a dedicated principal for onboarding and credential management. Do not give its token to the agents that search or curate memory every day.

| Profile | Roles | Read | Write | Use |
|---|---|---|---|---|
| `*-admin` | `admin`, `reader` | `/` | none | create principals, rotate credentials and change access policy |
| `*-curator` | `reader`, `proposer`, `curator` | the namespaces it manages | the same managed namespaces | review, apply and directly curate content |
| ordinary agent | `reader`, optionally `proposer` | its working namespaces plus shared read-only namespaces | its own namespace when proposing | search, read and propose changes |

Roles are explicit rather than inherited. An administrator needs `reader` for status and ordinary inspection, but does not need `curator` or content write prefixes. A curator should have only the namespaces it manages, such as `/skills/`, `/work/` or `/public/`. Ordinary agents normally read shared namespaces and write only below their own prefix.

Deployments may set `authorization.protected_read_prefixes`, commonly to `/work/`, `/personal/` and `/infrastructure/`. For non-admins, a broad `/` read grant then covers only unprotected namespaces. Add an equal or nested prefix explicitly when that principal should read protected content. The `admin` role bypasses this mask; it does not imply `reader`, `proposer` or `curator` actions. The Principals view warns when a non-admin broad reader needs explicit protected grants.

The web form's **Administrator** preset includes curator roles and shared content writes for operators who deliberately combine both jobs. For a dedicated onboarding principal, remove `proposer` and `curator`, then clear the write-prefix field before creating it.

## Web UI

Open `/admin`. Enter the `sandbox` bearer credential. The browser:

* keeps it only in JavaScript memory for the current tab;
* sends it as `Authorization: Bearer ...` on every `/admin/api/*` request;
* does not use cookies, local storage, session storage, URLs or service workers;
* forgets it on reload, lock or tab close.

The form starts with editable presets:

| Preset | Roles | Read | Write |
|---|---|---|---|
| Shared reader | `reader` | `/skills/`, `/public/` | none |
| Work instance | `reader`, `proposer` | `/work/`, `/skills/`, `/public/` | `/work/` |
| Personal instance | `reader`, `proposer` | `/personal/`, `/skills/`, `/public/` | `/personal/` |
| Infrastructure instance | `reader`, `proposer` | `/infrastructure/`, `/skills/`, `/public/` | `/infrastructure/` |
| Curator | `reader`, `proposer`, `curator` | `/` | shared writable namespaces |
| Administrator | curator roles plus `admin` | `/` | shared writable namespaces |

Every write prefix must be inside a readable prefix. Broad root access and `admin` are visibly called out before creation. When protected namespaces are configured, review any broad-root warning and add only the intended protected read prefixes.

## MCP Tools

Access tools use the existing `/mcp` endpoint and the existing client configuration. They are added to tool discovery only when the authenticated principal has the explicit `admin` role:

```text
access_principal_list
access_principal_create
access_principal_update
access_principal_rename
access_principal_disable
access_principal_enable
access_credential_rotate
access_principal_revoke
access_principal_delete
access_audit_list
```

Non-admin principals cannot discover or invoke these tools. Server-side authorization is always enforced independently of discovery. MCP create and rotate calls require an `idempotency_key`; a replay is rejected because a one-time credential cannot safely be returned twice.

These are direct MCP tools on the normal `/mcp` endpoint. They are added for managed administrators regardless of the configured regular memory-tool surface; they are not operations accepted by `memory_execute`. A client using an MCP proxy can discover and call them through that proxy. The `/admin` page is the browser alternative over `/admin/api/*`.

## Onboard Principals Through A Separate Profile

Create the dedicated administrator with the bootstrap credential or another existing administrator. Save the returned credential immediately: create and rotate responses show it only once.

For Piclaw, store ordinary and administrator credentials under different keychain names. Import them from permission-restricted temporary files rather than putting either token on a command line:

```bash
chmod 600 /path/to/memento.token /path/to/memento-admin.token
piclaw keychain set memento/example/agent \
  --type token --secret-file /path/to/memento.token
piclaw keychain set memento/example/admin \
  --type token --secret-file /path/to/memento-admin.token
rm -f /path/to/memento.token /path/to/memento-admin.token
```

Keep the ordinary Piclaw workspace's `.pi/mcp.json` limited to its ordinary principal:

```json
{
  "mcpServers": {
    "memento": {
      "url": "http://memento.example:18081/mcp",
      "auth": "bearer",
      "bearerTokenKeychain": "memento/example/agent",
      "bearerTokenEnv": "PICLAW_MCP_MEMENTO_TOKEN",
      "lifecycle": "lazy",
      "directTools": false
    }
  }
}
```

Use another Piclaw workspace or operator-only session for administration. Its `.pi/mcp.json` contains only the administrator profile:

```json
{
  "mcpServers": {
    "memento-admin": {
      "url": "http://memento.example:18081/mcp",
      "auth": "bearer",
      "bearerTokenKeychain": "memento/example/admin",
      "bearerTokenEnv": "PICLAW_MCP_MEMENTO_ADMIN_TOKEN",
      "lifecycle": "lazy",
      "directTools": false
    }
  }
}
```

Do not combine these entries in an ordinary agent workspace. Piclaw only decrypts keychain entries referenced by that runtime's configuration, so the administrator token stays out of the ordinary agent's process and tool output. `directTools: false` keeps each server behind Piclaw's MCP proxy; it does not remove Memento's admin-only tools.

Plain Pi uses the same runtime split. Its ordinary workspace has only:

```json
{
  "mcpServers": {
    "memento": {
      "url": "http://memento.example:18081/mcp",
      "auth": "bearer",
      "bearerTokenEnv": "MEMENTO_TOKEN",
      "lifecycle": "lazy",
      "directTools": false
    }
  }
}
```

A separate operator workspace has only:

```json
{
  "mcpServers": {
    "memento-admin": {
      "url": "http://memento.example:18081/mcp",
      "auth": "bearer",
      "bearerTokenEnv": "MEMENTO_ADMIN_TOKEN",
      "lifecycle": "lazy",
      "directTools": false
    }
  }
}
```

Launch the ordinary Pi process with only `MEMENTO_TOKEN` and the operator process with only `MEMENTO_ADMIN_TOKEN`. Supply each through a shell keychain, service manager or secret launcher. Never paste an administrator token into chat, tool arguments, tool output, committed configuration or logs. Delete temporary credential files after keychain import; rotate the credential if its one-time value was exposed. Disable and revoke profiles that are no longer needed, then delete them if their retained audit history is no longer required operationally.

Use the administrator profile only long enough to create, update, rotate, revoke or remove principals. Put each newly issued credential into that principal's own keychain entry, verify its reported roles and namespace visibility, then return to the ordinary or curator profile.

## Credentials

Create and rotate operations return a bearer credential exactly once. Memento stores only an HMAC verifier. The UI provides a copyable Piclaw profile using a keychain reference; it never places the token in `.pi/mcp.json`.

Losing a credential requires rotation. Rotation invalidates the previous credential. Revocation is permanent. Deletion requires the principal to be disabled and revoked first; the tombstoned record and activity remain in the control database.

The final enabled administrator cannot remove, disable or revoke its own admin access.

## Master Key

The master key encrypts a random verifier key. It does not encrypt each bearer token because bearer plaintext is never retained. Credential verification uses HMAC with that verifier key.

Master-key rotation is deliberately not a web or MCP operation. Run an explicit one-shot container command with both keys supplied as secrets:

```bash
MEMENTO_ADMIN_PREVIOUS_MASTER_KEY='old value' \
MEMENTO_ADMIN_MASTER_KEY='new strong value' \
memento --config /etc/memento/config.json rotate-master-key
```

Then start the normal service with only `MEMENTO_ADMIN_MASTER_KEY`. The command re-encrypts the verifier key atomically; principal credentials remain valid.

If managed access records exist and the configured key is wrong, startup fails closed.

## Backup And Recovery

Back up `control.sqlite` together with the Git repository and keep the master key in the container secret manager. A database backup without the key cannot authenticate managed principals. The bootstrap environment principals provide the deliberate emergency recovery path.
