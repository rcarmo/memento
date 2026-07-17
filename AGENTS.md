# Memento agent instructions

Memento is a standalone Python service that gives multiple Piclaw instances shared, durable knowledge over MCP. Read [docs/implementation.md](docs/implementation.md) for the full architecture and [PLAN.md](PLAN.md) for the delivery sequence.

These instructions apply to the whole repository. A more specific `AGENTS.md` overrides them for its subtree.

## Discover instructions first

Before planning or implementation, inspect the repository instructions:

```bash
find .github/copilot-instructions -type f \( -name 'SKILL.md' -o -name '*.instructions.md' \) -print | sort
grep -RniE '^(name:|description:|# Skill:|## Goal)' .github/copilot-instructions
```

Read every applicable file completely. The Python skill and `.github/copilot-instructions/python.instructions.md` are mandatory for Python changes.

## Product boundaries

- Git Markdown is authoritative for knowledge.
- `control.sqlite` is authoritative for operations, idempotency, proposals, leases and scheduler state.
- FTS, graph indexes, caches and signals are derived and rebuildable.
- The daemon is the sole canonical repository writer. One active process holds the write lease.
- Models may answer, classify and propose. Deterministic code owns identity, authorization, paths, validation, hashes, concurrency, writes, audit and completion claims.
- Piclaw conversations, Dream memory, schedules, credentials and keychains remain outside Memento.
- The deterministic service must remain useful with every optional model tier disabled.

## Security invariants

Treat MCP arguments, Markdown, frontmatter, links, retrieved text, model output and proxy headers as untrusted.

- Never accept a principal as a tool argument; use trusted uMCP request context.
- Authorize before search ranking or output to prevent namespace leakage.
- Reject traversal, symlinks, special files, reserved-file writes and paths outside the knowledge root.
- Require expected revisions and durable idempotency for mutations.
- Publish Git changes with compare-and-swap; never force-update canonical history.
- Do not expose unrestricted filesystem, shell, Git administration or network tools to models.
- Never log secrets, tokens, complete concepts or full sensitive prompts by default.
- Bound bodies, diffs, result counts, queues, retries, subprocesses, model steps and timeouts.

## Implementation order

Follow `PLAN.md`. Build the deterministic core before MCP writes or intelligent tiers. Do not add optional-tier modules, provider abstractions or dependencies until their milestone starts; document deferred interfaces instead.

Use `src/memento/` as the single package root. Keep reusable logic in the package, deployment files in `infra/` or their dedicated top-level directories, operator commands in `ops/`, and developer utilities in `tools/`.

## Python and Make workflow

Use the Makefile as the stable local and CI interface:

- `make install` — install the project;
- `make install-dev` — install development tools;
- `make lint` — Ruff checks;
- `make format` — apply formatting;
- `make format-check` — verify formatting;
- `make typecheck` — mandatory static typing;
- `make test` — deterministic tests;
- `make coverage` — branch coverage;
- `make check` — required validation gate.

Type all new and changed Python code. Prefer typed functions, frozen internal dataclasses and strict Pydantic models at untrusted or persisted boundaries. Apply YAGNI and keep side effects at explicit boundaries.

## Testing

Prefer parameterized pytest cases, small fakes and in-memory protocols. Test public behavior and invariants. Add negative coverage for authorization, path containment, malformed content, replay, stale revisions, idempotency conflicts, partial failure and recovery.

Keep unit tests offline and deterministic. Mark integration, live, crash and deployment tests explicitly. Compatibility work must cover Python 3.10–3.12 and Piclaw/uMCP Streamable HTTP.

## Git workflow

- Never rebase; merge or use `git pull --no-rebase`.
- Commit as `Rui Carmo <rui.carmo@gmail.com>`.
- Inspect status, branch and worktrees before writing.
- Do not reset, clean, stash, overwrite or commit another agent's changes.
- Use focused branches and pull requests once a remote and protected `main` workflow exist.
- Do not deploy uncommitted trees or mutable production image tags.

## Documentation

Write concise plain English. Distinguish planned, implemented, deployed and live-verified behavior. Update architecture, security, operations and acceptance evidence with behavioral changes. Do not present roadmap items as current capabilities.

## Completion gate

A change is complete only when applicable instructions were followed, security and failure paths are tested, documentation matches implementation, `make check`, `make typecheck` and `git diff --check` pass, and packaging changes pass wheel build plus clean installation.
