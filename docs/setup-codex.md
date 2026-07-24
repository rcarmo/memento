# Connect Codex To Memento

Codex CLI and the Codex IDE extension share MCP configuration from `~/.codex/config.toml`. Trusted projects may also use `.codex/config.toml` for project-scoped servers.

Memento is a remote Streamable HTTP server. Keep its bearer token in an environment variable and refer to that variable from Codex configuration.

## Provide The Token

Start Codex from a shell or launcher that already has the principal token:

```bash
read -rsp "Memento token: " MEMENTO_TOKEN
printf '\n'
export MEMENTO_TOKEN
codex
```

For persistent use, load the variable from an operating-system keychain or secret manager. Do not write the token into `config.toml`, shell history or a project repository.

## Configure The Server

Add this to `~/.codex/config.toml` or a trusted project's `.codex/config.toml`:

```toml
[mcp_servers.memento]
url = "http://memento.example:18081/mcp"
bearer_token_env_var = "MEMENTO_TOKEN"
startup_timeout_sec = 20
tool_timeout_sec = 120
```

`bearer_token_env_var` tells Codex to send the variable as an HTTP bearer token. Codex also supports static `http_headers` and environment-backed `env_http_headers`, but the dedicated bearer-token setting is the clearest match for Memento.

The CLI, desktop app and IDE extension use the same MCP configuration. Restart the active Codex process after changing either the file or environment.

## Verify Identity And Scope

List configured MCP servers with the Codex MCP command surface available in your installed version, then call Memento status from a Codex session. The response should show:

* the expected principal name;
* its roles;
* the expected visible-concept count;
* matching repository and index revisions.

If the principal has namespace restrictions, test one visible concept and one deliberately hidden path. Hidden concepts should not appear in search and should behave as unknown when read directly.

## Use The Memento Skill

Codex supports Agent Skills. Copy or link the repository skill into a discovered location such as:

```text
~/.codex/skills/memento/SKILL.md
<project>/.agents/skills/memento/SKILL.md
```

The skill describes the search/read workflow, proposal lifecycle, direct curator writes, namespace behavior, assets and retry reconciliation.

## Troubleshooting

* missing server or tools -- check the active `config.toml` layer and restart Codex.
* `401 Unauthorized` -- confirm `MEMENTO_TOKEN` is present in the Codex process environment and maps to a live principal.
* `403 Forbidden` -- the token is valid but its roles or write/read prefixes do not allow the operation.
* status shows the wrong visible count -- verify that the intended principal token, rather than a broad curator token, was exported.
* long writes or connection resets -- reconcile the repository revision and target path before retrying with the same idempotency key.

The Codex MCP configuration reference is published at [developers.openai.com/codex/mcp](https://developers.openai.com/codex/mcp/).
