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

* Concepts are Markdown files in Git, so they can be read and recovered without the service.
* `control.sqlite` records operations, idempotency, proposals, leases and scheduler state.
* FTS, graph indexes, caches and signals can be deleted and rebuilt.
* One daemon holds the writer lease and publishes repository changes.
* Models may answer, classify and draft proposals. Service code resolves identity, checks permissions and paths, validates output and performs writes.
* Piclaw conversations, Dream memory, schedules, credentials and keychains stay outside Memento.
* With every model setting off, search, read and curated writes continue to work.

## Security invariants

Treat MCP arguments, Markdown, frontmatter, links, retrieved text, model output and proxy headers as untrusted.

* Never accept a principal as a tool argument; use trusted uMCP request context.
* Authorise before search ranking or output to prevent namespace leakage.
* Reject traversal, symlinks, special files, reserved-file writes and paths outside the knowledge root.
* Require expected revisions and durable idempotency for mutations.
* Publish Git changes with compare-and-swap; never force-update `main`.
* Do not expose unrestricted filesystem, shell, Git administration or network tools to models.
* Never log secrets, tokens, complete concepts or full sensitive prompts by default.
* Bound bodies, diffs, result counts, queues, retries, subprocesses, model steps and timeouts.

## Implementation order

Follow `PLAN.md`. Finish repository and MCP behaviour before adding optional model tiers. Add provider abstractions and dependencies only when a current milestone needs them.

Use `src/memento/` as the single package root. Keep reusable logic in the package, CLI entry points alongside that package, deployment files in `deploy/`, tests in `tests/`, and developer utilities in `tools/`.

## Python and Make workflow

Use the Makefile as the stable local and CI interface.

* `make install` -- install the project.
* `make install-dev` -- install development tools.
* `make lint` -- Ruff checks.
* `make format` -- apply formatting.
* `make format-check` -- verify formatting.
* `make typecheck` -- mandatory static typing.
* `make test` -- repeatable offline tests.
* `make coverage` -- branch coverage.
* `make check` -- required validation gate.

Prefer Make targets in docs and day-to-day work. Raw `cargo` commands are secondary and mainly useful when working only inside `rust/`.

Type all new and changed Python code. Prefer typed functions, frozen internal dataclasses and strict Pydantic models at untrusted or persisted boundaries. Apply YAGNI and keep side effects at explicit boundaries.

## Testing

* Prefer parameterised pytest cases, small fakes and in-memory protocols.
* Test public behaviour and invariants.
* Add negative coverage for authorization, path containment, malformed content, replay, stale revisions, idempotency conflicts, partial failure and recovery.
* Keep unit tests offline and repeatable.
* Mark integration, live, crash and deployment tests explicitly.
* Keep static analysis clean on the lowest supported Python, 3.12.
* Treat runtime compatibility across Python 3.12-3.14 and Piclaw/uMCP Streamable HTTP as CI and compatibility-work evidence.
* Install Git LFS and pull LFS objects before working with model or corpus fixtures:
  ```bash
  git lfs install
  git lfs pull
  ```

## Git workflow

* Never rebase; merge or use `git pull --no-rebase`.
* Commit as `Rui Carmo <rui.carmo@gmail.com>`.
* Inspect status, branch and worktrees before writing.
* Do not reset, clean, stash, overwrite or commit another agent's changes.
* Use focused branches and pull requests once a remote and protected `main` workflow exist.
* Do not deploy uncommitted trees or mutable production image tags.

## Communication and documentation

Clear writing is part of correctness. A technically accurate change is not finished if its README, ADR or plan makes the behaviour hard to understand.

Before drafting or substantially editing prose, read the workspace `writing-style` skill and its reference. Apply it to READMEs, ADRs, plans, architecture notes, release notes and user-facing explanations.

* Lead with what works and why. Do not narrate commits or the order in which changes happened.
* Use concise plain English, concrete behaviour and specific numbers.
* Give each document one job: READMEs orient, ADRs record decisions, plans sequence work, contracts define exact interfaces and reports hold measurements.
* Link to detailed contracts, tests and reports instead of repeating them in narrative documents.
* Avoid audit-register prose such as "acceptance evidence", "pending operational proof" and repeated implemented/deployed disclaimers. Collect remaining gaps once.
* Avoid slogans and negative parallelism such as "X is authoritative; Y is not" when a concrete description is clearer.
* Do not repeat "canonical", "deterministic", "feature-gated" or boundary assurances through every section. State the rule once, then describe the behaviour that enforces it.
* Use `--` rather than a Unicode em dash, straight quotes, British spelling where natural and `*` for prose bullets.
* Keep headings useful. Do not add "Overview", "Introduction" or "Conclusion".
* Run an anti-pattern sweep before commit and read the result aloud. If it sounds like a compliance packet, generated product brochure or status bot, rewrite it.

Distinguish planned, implemented, deployed and live-verified behaviour where that distinction changes a reader's decision. Do not turn every paragraph into a status label, and do not present roadmap items as current capabilities.

## Completion gate

Treat a change as complete only when you followed the applicable instructions, tested security and failure paths, kept documentation aligned with implementation, and passed `make check`, `make typecheck` and `git diff --check`. Packaging changes must also pass wheel build and clean installation. Golden fixture generation is intentionally outside the standard check path unless the repository grows an explicit target that makes it practical and reliable.
