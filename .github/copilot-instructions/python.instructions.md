---
applyTo: "**/*.py,pyproject.toml,Makefile,tests/**/*.py"
---

# Python project instructions

Applies to Python and Python tooling in this repository.

## Required workflow

- Read `AGENTS.md`, `PLAN.md` and the relevant part of `docs/implementation.md`.
- Search `.github/copilot-instructions/` for applicable skills before implementation.
- Follow `.github/copilot-instructions/python/SKILL.md` for every Python change.
- Use Make targets rather than raw pytest, Ruff, type-checker or cargo commands in documentation and CI.
- Update the Make target in the same change when tooling or source paths change.
- Run `make check`, `make typecheck` and `git diff --check` before completion.
- For Rust validation, prefer `make rust-check` from the repository root or `make check` inside `rust/`; treat raw `cargo` commands as secondary.

## Conventions

- Type all new and changed Python code.
- Prefer Ruff for linting and formatting and pytest with branch coverage.
- Keep static analysis clean on Python 3.12; runtime CI covers Python 3.12 through 3.14.
- Install Git LFS and pull LFS objects before using model or corpus fixtures.
- Prefer direct, functional, YAGNI-oriented Python over forwarding layers and speculative abstractions.
- Keep the single package under `src/memento/`, the CLI in that package, tests under `tests/`, deployment artefacts under `deploy/`, and developer utilities under `tools/`.
- Keep Git, SQLite, filesystem, MCP, model and network I/O at explicit boundaries.
- Validate untrusted MCP, persisted, frontmatter and model-generated data with strict models.
- Consume trusted principals from uMCP request context; never accept identity as tool input.
- Keep the deterministic core independent of optional model providers.
- Never embed credentials or sensitive knowledge in code, fixtures, logs or commands.
