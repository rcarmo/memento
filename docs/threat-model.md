# Threat model

Memento's deterministic core owns identity, authorisation, paths, validation and writes. Everything crossing into it should be treated as untrusted until proven otherwise.

## Trust boundaries

```text
client tool input (untrusted)
  -> authenticated principal from trusted transport context
  -> deterministic validation and authorization
  -> repository filesystem boundary
  -> canonical markdown bundle
```

## Primary threats

* Path traversal: reject absolute paths, `..`, symlink components and unsafe targets.
* Reserved-file overwrite: reject direct writes to generated files such as `index.md` and root `log.md`.
* Special-file abuse: reject writes to device files, FIFOs and non-regular existing targets.
* Malformed frontmatter: parse with `python-frontmatter`, then validate with strict Pydantic models.
* Schema confusion: reject unknown frontmatter keys and out-of-vocabulary `type` values.
* Markdown rewrite corruption: use `markdown-it-py` token structure instead of regular expressions.
* Link integrity drift: audit broken links and duplicate IDs on every repository scan.
* Authorisation bypass: take principal identity from trusted request context, never from tool arguments.
* Answer-tier scope bypass: bind cache, retrieval, graph expansion, concept reads, model context and returned evidence to the same authorization fingerprint.
* Secret disclosure through retrieval: classify explicit secret intent before cache lookup or repository access, and reject sensitive-tagged concepts before the reader.
* Prompt injection in concept bodies: mark repository excerpts as untrusted data, delimit each concept and validate every returned citation against the exact read revision.
* Stale-answer confusion: distinguish current and historical intent, reject conflicting/deprecated evidence for current questions and apply explicit supersession metadata.
* Lease bypass or split-brain writes: require one active writer lease for the local runtime and reject contending operator processes.
* Backup self-destruction: avoid storing recovery sets under `repository.root_path`, because restore replaces that tree.

## Current mitigations

* Strict Pydantic v2 models cover config, principals, envelopes and concept frontmatter.
* Principal bearer tokens resolve through authenticated request context. Bootstrap/recovery tokens come from configured `token_env` variables; managed credentials are checked against HMAC verifiers in `control.sqlite` without retaining bearer plaintext.
* `ruamel.yaml` keeps deterministic serialisation under control.
* Reserved-path enforcement happens before filesystem writes.
* Bundle scan and repository audit cover every concept file.
* Authorisation is configured by role and namespace. Explicit personal/work answer queries narrow that authorized set further instead of broadening it.
* Enabled hot and deep answers expose a bounded `EvidenceSet` with source ranks, state, revision, scope and support chains. Relational graph closure is depth one from no more than two primary anchors.
* Secret-intent answers abstain before cache, retrieval and model calls. Current-answer filters and exact-revision citation validation run in service code, not in prompts.
* Runtime startup acquires the writer lease before serving or running local maintenance commands.
* Backup restore verifies checksums, rejects tar links and path escapes, validates archived `refs/heads/main` against the manifest, then materialises `current/` from the archived bare repository.

## Operator implications

The local CLI is not a parallel administration plane. `status`, `audit`, `rebuild-index` and `backup` all go through runtime startup and therefore need the same exclusive lease as the daemon. For live status inspection, prefer MCP `memory_status` or `memory://status` instead of trying to race the running service.

## Deployment boundary

The trusted-LAN DiskStation deployment exercises bearer-authenticated Streamable HTTP, namespace filtering, dynamic principals, a read-only non-root container, dropped capabilities and persistent state. The graph and admin surfaces are deliberately limited to that network. The supplied reverse-proxy file remains a reference: TLS termination, Internet exposure and an operator-run systemd parity exercise are not production claims.

## Managed-access threats

Dynamic bearer credentials are stored only as HMAC verifiers. The random verifier key is encrypted by the container master key. `/admin` keeps its bearer only in tab memory and sends it on each API request; deployments must use TLS outside a trusted host boundary. Role-filtered discovery is not the authorization boundary: every `access_*` call checks `admin`. Namespace validation prevents writes outside readable prefixes, the final enabled admin cannot remove itself, and credential plaintext is returned once. Master-key loss makes managed authentication unavailable; bootstrap environment principals are the recovery path.
