# Memento agent instructions

Memento is a standalone Python service that gives multiple Piclaw instances shared, durable knowledge over MCP. Read [docs/implementation.md](docs/implementation.md) for architecture and [PLAN.md](PLAN.md) for delivery status.

These instructions apply to the whole repository. A more specific `AGENTS.md` overrides them for its subtree.

## Discover instructions first

Before planning or implementation, inspect the repository instructions.

```bash
find .github/copilot-instructions -type f \( -name 'SKILL.md' -o -name '*.instructions.md' \) -print | sort
grep -RniE '^(name:|description:|# Skill:|## Goal)' .github/copilot-instructions
```

Read every applicable file completely. Read the Python skill and `.github/copilot-instructions/python.instructions.md` before any Python change.

## Product boundaries

* Treat Git Markdown as the authoritative knowledge store.
* Treat `control.sqlite` as authoritative for operations, idempotency, proposals, leases and scheduler state.
* Treat FTS, graph indexes, caches and signals as derived and rebuildable.
* Keep the daemon as the only canonical repository writer. Exactly one active process holds the write lease.
* Let models answer, classify and propose. Keep identity, authorization, paths, validation, hashes, concurrency, writes, audit and completion claims in deterministic code.
* Keep Piclaw conversations, Dream memory, schedules, credentials and keychains outside Memento.
* Keep the deterministic service useful with every optional model tier disabled.

## Security invariants

Treat MCP arguments, Markdown, frontmatter, links, retrieved text, model output and proxy headers as untrusted.

* Never accept a principal as a tool argument; use trusted uMCP request context.
* Authorise before search ranking or output to prevent namespace leakage.
* Reject traversal, symlinks, special files, reserved-file writes and paths outside the knowledge root.
* Require expected revisions and durable idempotency for mutations.
* Publish Git changes with compare-and-swap; never force-update canonical history.
* Do not expose unrestricted filesystem, shell, Git administration or network tools to models.
* Never log secrets, tokens, complete concepts or full sensitive prompts by default.
* Bound bodies, diffs, result counts, queues, retries, subprocesses, model steps and timeouts.

## Implementation order

Follow `PLAN.md`. Build the deterministic core before MCP writes or intelligent tiers. Do not add optional-tier modules, provider abstractions or dependencies before their milestone starts; document deferred interfaces instead.

Use `src/memento/` as the single package root. Keep reusable logic in the package, deployment files in `infra/` or their dedicated top-level directories, operator commands in `ops/`, and developer utilities in `tools/`.

## Python and Make workflow

Use the Makefile as the stable local and CI interface.

* `make install` -- install the project.
* `make install-dev` -- install development tools.
* `make lint` -- Ruff checks.
* `make format` -- apply formatting.
* `make format-check` -- verify formatting.
* `make typecheck` -- mandatory static typing.
* `make test` -- deterministic tests.
* `make coverage` -- branch coverage.
* `make check` -- required validation gate.

Type all new and changed Python code. Prefer typed functions, frozen internal dataclasses and strict Pydantic models at untrusted or persisted boundaries. Apply YAGNI and keep side effects at explicit boundaries.

## Testing

* Prefer parameterised pytest cases, small fakes and in-memory protocols.
* Test public behaviour and invariants.
* Add negative coverage for authorization, path containment, malformed content, replay, stale revisions, idempotency conflicts, partial failure and recovery.
* Keep unit tests offline and deterministic.
* Mark integration, live, crash and deployment tests explicitly.
* Cover Python 3.12–3.14 and Piclaw/uMCP Streamable HTTP in compatibility work.

## Git workflow

* Never rebase; merge or use `git pull --no-rebase`.
* Commit as `Rui Carmo <rui.carmo@gmail.com>`.
* Inspect status, branch and worktrees before writing.
* Do not reset, clean, stash, overwrite or commit another agent's changes.
* Use focused branches and pull requests once a remote and protected `main` workflow exist.
* Do not deploy uncommitted trees or mutable production image tags.

## Documentation

Write concise plain English. Distinguish planned, implemented, deployed and live-verified behaviour. Update architecture, security, operations and acceptance evidence with behavioural changes. Do not present roadmap items as current capabilities.

## Completion gate

Treat a change as complete only when you followed the applicable instructions, tested security and failure paths, kept documentation aligned with implementation, and passed `make check`, `make typecheck` and `git diff --check`. Packaging changes must also pass wheel build and clean installation.
