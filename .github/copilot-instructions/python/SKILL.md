---
name: Python project conventions
description: Direct, typed and compact Python design, implementation, packaging and validation rules.
distribution: repository
---

# Python project conventions

Use this skill for all Python changes.

## Aim

Write correct, secure and compact Python. Prefer modules, functions, explicit data and straightforward control flow. Add an abstraction only when current code needs it.

Apply these priorities in order:

1. correctness and security;
2. clear behaviour and failure modes;
3. accurate types;
4. simple control flow;
5. testability and operational evidence;
6. reuse shown by current callers;
7. future extension only when required now.

## Start with the repository

Before editing:

1. read `AGENTS.md` and relevant local skills;
2. inspect existing code and tests;
3. search for current callers and similar code;
4. check `git status`, the active branch and worktrees;
5. use the Makefile for validation.

Do not design from memory when the repository can answer the question.

## Keep the design direct

Start with a focused module, typed functions and explicit values.

Add an abstraction only when it:

- removes duplication across current callers;
- isolates an external boundary;
- enforces an invariant;
- owns a resource lifecycle;
- represents validated boundary data;
- supports more than one implementation that exists now.

Do not add:

- manager, provider, factory, strategy or implementation layers without current need;
- repository/service/controller chains that forward arguments;
- dependency-injection containers for small applications;
- abstract base classes with one implementation;
- plugin systems for hypothetical extensions;
- classes used only as namespaces;
- one-class-per-file structures;
- getters or setters that only expose an attribute;
- compatibility layers unless an identified consumer requires them.

Prefer a module function to a forwarding method. Prefer a data-driven mapping to repeated subclasses. Prefer deletion to deprecation when the code has no external consumer.

## Functions and module size

A function must perform one coherent operation.

Treat 200 lines as a soft maximum. Use a lower limit when a function:

- occupies much of its module;
- mixes configuration, I/O, policy, transformation and rendering;
- repeats stages or result construction;
- hides a reusable pure transformation;
- needs deep nesting to remain readable.

A 120-line function can be too large in a 200-line module. A cohesive 160-line transaction can be acceptable in a large domain module.

Split long functions into typed helpers around real stages such as:

- configuration;
- client and resource setup;
- input construction;
- policy;
- persistence;
- conflict handling;
- result rendering;
- cleanup or rollback.

Do not create wrapper classes or one-line helpers to satisfy a line count. The split must improve names, testing or control flow.

Use early validation and ordinary branches. Avoid modifying caller-owned mutable values. Use keyword-only arguments when adjacent parameters share a type.

## Functional core and imperative shell

Keep policy and transformations pure when practical. Keep MCP/framework calls, model calls, Git, SQLite, clocks, environment variables, subprocesses and filesystem access at explicit boundaries.

The composition root may:

- read configuration;
- create clients;
- connect implementations;
- call the domain operation;
- render the final result.

It should not contain the domain operation itself.

`deploy/` contains deployment artefacts. Reusable application logic and CLI code belong under `src/memento/`. Repository tests live under `tests/`. Developer utilities and experiments belong under `tools/`.

Do not perform I/O in module imports, validators, properties or hidden constructors.

## Data modelling

Use the lightest type that enforces the real boundary.

- Use ordinary values for local calculations.
- Use `@dataclass(frozen=True, slots=True)` for trusted internal values.
- Use `TypedDict` for trusted structured mappings when runtime validation adds no value.
- Use strict Pydantic models for untrusted input, persisted records, API payloads, cache files and model-generated data.
- Use `StrEnum`, `Enum` or `Literal` for closed values.
- Use a small `Protocol` only for current interchangeable implementations.

```python
from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class BlobVersion:
    etag: str
    version_id: str
```

```python
from pydantic import BaseModel, ConfigDict


class SkillManifest(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    name: str
    revision: int
    content_sha256: str
```

Do not use Pydantic for a trusted intermediate projection merely because nearby code uses it. Do not create a model for every private value.

## Types

Type every changed function, method, callback and data field.

Prefer:

- `X | None`;
- built-in generics such as `list[str]`;
- `collections.abc` for accepted interfaces;
- precise return types that carry useful context;
- validation of dynamic SDK or JSON values at the boundary.

Do not use `Any` to avoid design work. Keep casts narrow. A `# type: ignore` must include an error code and a reason.

## Errors

Fail explicitly and preserve causes with `raise ... from exc`.

Catch an error only to:

- translate it;
- retry it;
- compensate or roll back;
- add useful context;
- convert it into a documented result.

Do not catch `Exception` and return `None`, `False` or an empty collection. Distinguish validation, authorisation, missing data, conflict, retryable failure and permanent failure when callers respond differently.

Keep exception hierarchies small. Error text must not expose secrets or sensitive content.

## External operations

- Use context managers for clients and temporary resources.
- Close Azure, HTTP and filesystem resources.
- Bound network calls, subprocesses, retries and polling with timeouts or attempt limits.
- Preserve cancellation in asynchronous code.
- Use asynchronous code only when concurrent I/O provides measured value.
- Do not maintain speculative parallel sync and async APIs.
- Keep retries specific to an operation; do not hide different retry contracts behind one generic helper.

## Configuration, identity and logs

Read environment variables at the composition root and pass typed settings inward. Validate configuration before side effects.

Consume authenticated principals from trusted uMCP request context. Never place credentials in code, fixtures, logs, examples or shell commands.

Log once at the boundary that records the outcome. Persist immutable audit records for security-sensitive or durable mutations. Library code must not print; CLI and operator commands may print deliberate results.

## Shared code

Extract shared code only after finding multiple current callers with the same contract.

Good shared candidates include:

- canonical serialisation;
- stable hashing primitives;
- environment-backed client construction with identical failure rules;
- pure payload builders;
- repeated result construction with identical fields and semantics.

Keep domain rules in their domain functions. Similar syntax does not prove a shared contract.

When renaming or moving code:

1. extract every reference by file;
2. move with `git mv`;
3. update imports, dynamic module strings, packaging, Docker, Make, workflows, tests and docs;
4. repeat text and path searches until the old name is absent;
5. build and inspect the package;
6. install it in a clean environment and run its entry points.

Do not add a compatibility namespace unless a known external consumer requires it.

## Repository layout

Use the layout that exists in this repository:

```text
src/
  memento/
    cli.py       # service CLI entry points
    repository/  # canonical bundle and Git transactions
    control/     # operation and proposal state
    derived/     # rebuildable search and graph state
deploy/          # deployment artefacts
tools/           # developer utilities and experiments
tests/
```

Create milestone packages only when implementation starts. Do not add empty optional-tier module trees as architectural placeholders.

Preserve logical package boundaries with import tests. Do not create repeated source roots to simulate package isolation in one distribution.

## Dependencies and formats

Prefer the standard library when it is clear and maintained. Add a dependency only for a current requirement and pin it under repository policy.

Use established parsers for structured formats. In this repository:

- `markdownify` converts HTML to Markdown;
- `markdown-it-py` parses Markdown structure;
- `python-frontmatter` handles frontmatter;
- `ruamel.yaml` handles round-trip YAML.

Do not edit Markdown structure with regular expressions.

## Packaging

The repository must install through standard Python tooling.

- Define console scripts in `pyproject.toml`.
- Keep imports free of environment-specific side effects.
- Include package data explicitly.
- Build and inspect a wheel after package moves or renames.
- Install the wheel with dependencies in a clean virtual environment.
- Import each package and run each console entry point.
- Use a distinct top-level package name unless the project deliberately accepts collision risk.

## Make workflow

Use the Makefile as the stable interface:

```text
make install
make install-dev
make format
make check
make typecheck
make coverage
```

For Rust work, prefer `make rust-check` from the repository root or `make check` inside `rust/`. Raw `cargo` commands are secondary and should not replace the documented Make workflow.

Update Make targets when source paths or tools change. CI should call Make targets instead of duplicating raw Ruff, pytest or Mypy commands.

## Tests

Test behaviour and invariants through public interfaces.

Include:

- success and failure paths;
- validation and authorisation failures;
- conflict, retry, timeout and rollback behaviour;
- malformed external and model output;
- idempotent replay;
- cache integrity and outage fallback;
- exact persisted and API schemas;
- clean package installation after structural changes.

Prefer parameterised tests, small fakes and in-memory protocol implementations. Avoid deep mocks and tests that only reproduce private call order.

## Completion gate

A Python change is complete when:

- changed code is accurately typed;
- `make format` produces no further changes;
- `make check` passes;
- `make typecheck` passes;
- security and failure paths have tests;
- external operations are bounded;
- no speculative layer or dependency was added;
- package moves pass mechanical reference checks and clean installation;
- documentation matches the implementation;
- `git diff --check` passes.

Static analysis must stay clean on Python 3.12, the lowest supported version. Runtime compatibility evidence covers Python 3.12 through 3.14.

If you need model or corpus fixtures, install and fetch Git LFS objects first:

```bash
git lfs install
git lfs pull
```

## Review checklist

- [ ] Could a module function replace each new class?
- [ ] Does every class own state, validation, lifecycle or substitution?
- [ ] Are trusted internal values dataclasses rather than Pydantic models?
- [ ] Are untrusted and persisted values validated at their boundary?
- [ ] Does each function perform one coherent operation?
- [ ] Are functions over 200 lines justified? Should a lower proportional limit apply?
- [ ] Is I/O confined to explicit boundaries?
- [ ] Are errors typed, useful and safe?
- [ ] Are retries and polling bounded?
- [ ] Is shared code backed by multiple current callers?
- [ ] Are imports, packaging and dynamic references consistent after moves?
- [ ] Do Make, tests, type checks, wheel build and clean install pass?
