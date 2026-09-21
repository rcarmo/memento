# Reproduce the graph and admin tests

The browser tests start an isolated Go service on a loopback port with 120 fixed concepts, a trash concept, synthetic embeddings and disposable administrator credentials. They need no NAS, production token, model download or running Memento instance. Every invocation creates fresh repository/SQLite state and a separate artifact directory.

## Run from a clean checkout

Use Go 1.26.6, Node 22.23.1, npm 10 and Make. Playwright 1.61.0 and its dependencies are pinned in `package-lock.json`; use the browsers downloaded by that version, not a system Chromium override.

```sh
# Install locked npm dependencies and matching Chromium/WebKit builds.
make ui-setup
# On a fresh supported Linux host, also install OS browser libraries:
make ui-setup UI_INSTALL_FLAGS=--with-deps

# Same repeatability gate as CI: both engines, twice, no retries.
make ui-test UI_REPEAT=2
```

The first setup needs network access. OS dependency installation may require sudo. A run reuses installed toolchains/browser binaries but creates all application state from scratch. `make ui-test` defaults to one run of each engine. It first checks Gherkin evidence bindings, runs diagnostics unit tests, and verifies that interruption closes the fixture and records a failed audit.

Focused commands:

```sh
make ux-check
make ui-unit
make ui-lifecycle
make ui-test UI_BROWSER=chromium
make ui-test UI_BROWSER=webkit
```

Chromium and WebKit fail when WebGL2 is unavailable. Firefox can be installed with `make ui-setup UI_BROWSERS=firefox` and run with `make ui-test UI_BROWSER=firefox`; environments without Firefox WebGL2 report graph cases as `not_run`. Firefox fallback/admin checks do not replace the required Chromium/WebKit gates.

## Repeatability and failures

- Each browser case gets a new context with a fixed viewport, locale, UTC timezone and colour scheme.
- The fixture uses explicit IDs, timestamps, paths and synthetic vectors. Rendering and database revision hashes need not be byte-identical; assertions test behaviour and invariant values.
- Delayed responses use request barriers. Refresh polling advances the Playwright clock. Assertions wait for observable state, without fixed browser sleeps or automatic retries.
- Each case has a 90-second deadline; fixture startup and shutdown are bounded.
- SIGINT/SIGTERM close the browser and ask the Go fixture to stop. A process-group termination fallback handles an unresponsive fixture.
- Required browser failures and setup failures return nonzero. A passing later run cannot overwrite evidence from an earlier run.

Artifacts appear under `build/ui-audit/<engine>/run-*/`: `results.json`, `environment.json`, `server.log`, sample screenshots and exports. Failures include a screenshot, page text and a Playwright trace when the context is still available. Environment metadata records Node/browser versions and the dependency-lock digest. CI uploads all runs, including failures.

```sh
cd tools/browser
npx --no-install playwright show-trace ../../build/ui-audit/chromium/run-<id>/failure-<n>-trace.zip
```

The [UX inventory](../../docs/ux-feature-inventory.md) maps 24 Gherkin scenarios to existing evidence. `make ux-check` rejects missing/duplicate scenarios, renamed browser cases, missing Go tests and broken evidence paths. It validates bindings; it does not execute Gherkin steps. Partial and documented-only scenarios remain labelled as coverage gaps.

`live-diagnostics-probe.mjs` is a separate opt-in production observation tool. It is not invoked by the reproducible suite.
