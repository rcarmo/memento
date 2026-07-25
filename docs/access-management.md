# Access Management

Memento manages principals dynamically in `control.sqlite`. The canonical knowledge repository remains Git Markdown; access policy, credential verifiers and access activity are control-plane state.

## Bootstrap

Set the container master key before starting the access-management MVP:

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

Every write prefix must be inside a readable prefix. Broad root access and `admin` are visibly called out before creation.

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
