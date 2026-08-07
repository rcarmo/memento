# ADR 0012: Manage access in the control plane

**Status:** accepted
**Date:** 2026-07-25

## Decision

Memento stores dynamic principals, namespace policy, credential verifiers and access activity in `control.sqlite`. A container-provided master key encrypts a random verifier key; bearer credentials are represented only by HMAC digests and are returned once on creation or rotation.

The existing `/mcp` endpoint exposes `access_*` tools only to authenticated principals with the explicit `admin` role. `/admin` uses the same access service and requires the admin bearer credential on every API request, held only in tab memory.

The bootstrap migration preserves existing environment credentials, renames `piclaw-workspace` to `sandbox`, and grants it `admin`. Static environment principals remain a bootstrap and emergency recovery mechanism.

Master-key rotation is an explicit one-shot container command, never a web or MCP operation.

## Consequences

* Ordinary clients need no second MCP configuration.
* `admin` is separate from `curator`.
* Access policy can change without rebuilding or committing the memory repository.
* The final enabled administrator cannot remove its own access.
* `control.sqlite` and the container master key must be backed up together.
* Credential plaintext is unavailable after the one successful create/rotate response.
