---
applyTo: "**/*.py,pyproject.toml,Makefile,tests/**/*.py"
---

# Python project instructions

Applies to Python and Python tooling in this repository.

## Required workflow

- Read `AGENTS.md`, `PLAN.md` and the relevant part of `docs/implementation.md`.
- Search `.github/copilot-instructions/` for applicable skills before implementation.
- Follow `.github/copilot-instructions/python/SKILL.md` for every Python change.
- Use Make targets rather than raw pytest, Ruff or type-checker commands in documentation and CI.
- Update the Make target in the same change when tooling or source paths change.
- Run `make check`, `make typecheck` and `git diff --check` before completion.

## Conventions

- Type all new and changed Python code.
- Prefer Ruff for linting and formatting and pytest with branch coverage.
- Prefer direct, functional, YAGNI-oriented Python over forwarding layers and speculative abstractions.
- Keep the single package under `src/memento/` and tests under `tests/`.
- Keep Git, SQLite, filesystem, MCP, model and network I/O at explicit boundaries.
- Validate untrusted MCP, persisted, frontmatter and model-generated data with strict models.
- Consume trusted principals from uMCP request context; never accept identity as tool input.
- Keep the deterministic core independent of optional model providers.
- Never embed credentials or sensitive knowledge in code, fixtures, logs or commands.
