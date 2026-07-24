# Connect Pi To Memento

Pi reaches remote MCP servers through [`pi-mcp-adapter`][adapter]. Memento uses Streamable HTTP with a bearer token, so the useful boundary is a short adapter configuration plus an environment variable that Pi inherits at startup.

## Install The Adapter

```bash
pi install npm:pi-mcp-adapter
```

Restart Pi after installation. The adapter contributes a single `mcp` proxy tool instead of placing every Memento schema in the base prompt.

## Keep The Token Outside Configuration

Export a token issued for the intended Memento principal:

```bash
read -rsp "Memento token: " MEMENTO_TOKEN
printf '\n'
export MEMENTO_TOKEN
```

Use your shell keychain, service manager or secret launcher for persistent sessions. Do not commit the token or place it directly in `mcp.json`.

## Configure The Server

Pi reads global MCP configuration from `~/.pi/agent/mcp.json`. A trusted project can override it with `.pi/mcp.json`.

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

`lazy` connects on first use. `directTools: false` keeps Memento behind the compact proxy; this is the better default for a server with many operations and resources.

Start a new Pi session after changing configuration. Discover and call Memento through the adapter:

```text
mcp({})
mcp({ server: "memento" })
mcp({ describe: "memento_memory_status" })
mcp({ tool: "memento_memory_status", args: "{}" })
```

The exact proxied tool prefix depends on the adapter's `toolPrefix` setting. Use discovery rather than assuming names.

## Load The Memento Skill

When Pi runs from the Memento repository, it discovers `.agents/skills/memento/SKILL.md` automatically. To use the skill elsewhere, copy or link that directory into one of Pi's skill roots:

```text
~/.agents/skills/memento/
<project>/.agents/skills/memento/
```

You can force it to load with:

```text
/skill:memento
```

## Check The Connection

`memory_status` should report the expected principal, roles, visible concept count and matching repository/index revisions. A successful TCP connection with the wrong principal is not a successful setup.

Common failures:

* `401 Unauthorized` -- the environment variable is absent or contains the wrong principal token.
* `403 Forbidden` -- the principal is authenticated but the requested role or namespace is not allowed.
* connection reset after configuration changes -- start a fresh Pi session so the adapter rebuilds its transport.
* no tools listed -- verify the project/global config location and inspect `mcp({})` before editing server settings.

[adapter]: https://github.com/nicobailon/pi-mcp-adapter
