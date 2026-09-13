# Memento 0.5.1 validation and deployment

## Release identity

* Release: <https://github.com/rcarmo/memento/releases/tag/v0.5.1>
* Commit: `244999de991739000786f2702dac1fda63cf7fcb`
* Published: 2026-09-13 13:12:23 UTC
* OCI index: `sha256:ccd742ecd4cfe073b531cc7d22649d0dbf78ab679814c7fb4d6487ce415f0664`
* CI: <https://github.com/rcarmo/memento/actions/runs/34758823700>
* Release workflow: <https://github.com/rcarmo/memento/actions/runs/34758932472>

The graph navigation and Trash branch was merged into `main` and pushed before release. The exact tagged commit passed CI and the release workflow, including Python 3.12--3.14, wheel installation, Rust checks, amd64/arm64 builds and the non-AVX smoke test. Downloading and hashing the OCI index independently matched the release digest. The `0.5.1`, `0.5`, `0` and `latest` aliases resolved to that digest.

Local validation passed all 354 Python tests after the release-pipeline regression was added. The earlier full check and coverage run passed 353 tests at approximately 85%; the final CI matrix passed 352 tests with two model-dependent skips and approximately 84% coverage. Container smoke tests load the actual packaged models. A clean environment imported the installed 0.5.1 wheel. The browser matrix passed 14 tests with 34 intentional platform skips; detailed interaction checks run in Chromium, with the other projects covering their supported smoke/touch paths.

## Restored model archive

The first CI attempt failed because `model-assets-v1` had been deleted by release retention and its pinned download URL returned 404. The three local runtime files matched their existing manifest hashes. A deterministic USTAR archive was rebuilt from those verified bytes and published under the restored asset release; the manifest now pins archive SHA-256 `9adbc2e95c98259359414f6d41a0d04e01e000412fd0bac5b488fc30821f0d7f`.

A clean download/extract/verify run succeeded. Individual model-file hashes are unchanged. Release cleanup now applies only to versioned application releases, with a regression test protecting that filter. The model-assets release remained present after 0.5.1 cleanup completed.

## DiskStation replacement

Endpoint 18, stack 111 now runs:

* Container: `04eeccffa5c6da771193e1fbab377cc072cfd2e0ebf2fd3b44047156bc8516ee`
* Local amd64 image: `sha256:12fc61fc4a1088f1957c8093b70051f94625d0067b8fe04472432afea05d37d2`
* OCI version/revision: `v0.5.1` / `244999de991739000786f2702dac1fda63cf7fcb`
* Started: 2026-09-13 13:28:53 UTC
* Healthy by: 2026-09-13 13:36:13 UTC
* Restart count: zero

The first replacement changed only the image digest, but post-deployment inspection revealed that Docker assigned a new anonymous `/models` volume. No model-volume mapping had been declared in the old Compose file. A second replacement explicitly reattached the original volume, `141af2b355088351e0d17e9121c6a83b7f9248774e6752fa4711c64c0dfc85cb`, through an external `memento-models-preserved` mapping. This prevents the same drift during future replacements.

Final comparisons confirmed identical source paths, destinations and read/write flags for every original mount, including `/models`. Configuration and secret binds remain read-only, and the state bind remains writable. Environment settings, port bindings, UID/GID `65532:65532`, read-only root, dropped capabilities, no-new-privileges, memory settings and init behaviour were preserved. No configuration helper, destructive volume cleanup or manual embedding refresh was run.

## Live checks

MCP rediscovery returned the same 23 compact/admin tools. Status reports 0.5.1, schema version 2, 186 visible concepts, no proposal backlog and repository/index revision `5224a6d50df9f068bec4bc5e016482f419bb79f6`, unchanged from the pre-deployment check. The content index is not stale. Unauthenticated `/mcp` returned HTTP 401 with a Bearer challenge.

Help advertises the execute-only `trash`, `restore` and `purge` operations and the Trash workflow. Inventory of `/trash/` returned an empty page. An unconfirmed purge request was rejected before mutation with `permanent deletion requires confirm=true; Git history is retained`. Destructive trash/restore/purge tests were limited to disposable local fixtures, including namespace denial, replay, restore collisions, accepted-asset purge and historical-content retention. No production item was moved or deleted.

Read-only inspection of the production derived database found the external-link migration marker, 16 classified external links, and zero HTTP/HTTPS/network-path links still classified as broken.

A headless Chromium visit to the deployed graph loaded 186 nodes, displayed ten namespace labels and reported no page errors. Show Trash, semantic toggling and force-distance adjustment worked; changing distance restarted the running layout. The browser was captured while the simulation was still active, so this check does not claim that the full production graph had settled. Fixture tests verify settling and cancellation. The screenshot and bounded browser results are stored under `docs/evidence/graph/release-0.5.1-*`.

## Existing limitations

The pre-deployment 0.5.0 service already reported 25 of 186 embeddings not ready. That count remains unchanged; startup now reports the embedding revision as `partial`. Needle is loaded and the SQLite vector extension is enabled. Production therefore supplies no semantic edges under the current freshness policy; enabled semantic-edge rendering is covered by local fixtures rather than claimed as a live production result.

Docker still reports `PidsLimit: null`, the existing Synology/Compose enforcement discrepancy. This release does not claim to resolve it. The optional visual debugger remains unauthenticated and restricted operationally to the trusted network; deletion stays on authenticated MCP tools.
