# Connect Piclaw To Memento

Piclaw bundles `pi-mcp-adapter` and adds keychain-backed bearer-token injection. The Memento token stays encrypted in Piclaw's keychain; the MCP configuration contains only the keychain name and an in-memory environment-variable name.

## Store A Principal Token

Write the token to a temporary file with restrictive permissions, then import it:

```bash
chmod 600 /path/to/memento.token
piclaw keychain set memento/example/reader \
  --type token \
  --secret-file /path/to/memento.token
rm -f /path/to/memento.token
```

Use one keychain entry per Memento principal. Names such as `memento/work/agent` make the trust boundary easier to audit than one shared token.

## Configure The Workspace

Piclaw prefers project-local MCP configuration at `.pi/mcp.json`:

```json
{
  "mcpServers": {
    "memento": {
      "url": "http://memento.example:18081/mcp",
      "auth": "bearer",
      "bearerTokenKeychain": "memento/example/reader",
      "bearerTokenEnv": "PICLAW_MCP_MEMENTO_TOKEN",
      "lifecycle": "lazy",
      "directTools": false
    }
  }
}
```

`bearerTokenKeychain` and `bearerTokenEnv` must appear together. Do not combine them with a literal `bearerToken`, and choose an environment name that is not already set.

At startup Piclaw decrypts the keychain entry, places it in the named environment variable in memory, loads the MCP adapter and removes the value during graceful shutdown. The token is not copied into MCP metadata caches or committed configuration.

Restart Piclaw or start a new session after editing the profile. Reconnecting an existing adapter does not necessarily reload changed keychain references.

## Discover And Verify

Use the MCP panel or proxy tool:

```text
/mcp
/mcp status
/mcp reconnect memento
```

```text
mcp({ server: "memento" })
mcp({ tool: "memento_memory_status", args: "{}" })
```

Status should show the principal corresponding to the stored token. Check its visible concept count and roles before filing shared knowledge.

Keep `directTools: false` unless there is a concrete reason to place selected Memento tools into every agent prompt. The proxy keeps schemas discoverable without consuming the full context window.

## Several Namespace Profiles

A single workspace normally uses one Memento principal. Separate Piclaw workspaces can use different keychain entries against the same server:

```text
memento/example/work-agent
memento/example/personal-agent
memento/example/infrastructure-agent
memento/example/shared-reader
```

Memento applies each principal's namespace prefixes before search ranking, graph traversal and writes. The trusted visual debugger's **View as** selector simulates those policies for diagnosis; it does not replace MCP authorization.

## Troubleshooting

* `bearerTokenEnv ... is already set` -- choose a dedicated environment name or remove the conflicting process environment variable.
* the old principal still appears -- restart Piclaw so keychain hydration runs again.
* `401 Unauthorized` -- confirm the keychain entry contains only the token and that the server still maps that token.
* `403 Forbidden` -- inspect the principal's roles and namespace prefixes rather than replacing the token with a curator token.
* adapter connects by SSE and receives `405` -- ensure the URL ends in the configured Streamable HTTP endpoint, normally `/mcp`.

Piclaw's implementation and tests are in `runtime/src/secure/mcp-keychain.ts` and `runtime/test/secure/mcp-keychain.test.ts` in the Piclaw repository.
