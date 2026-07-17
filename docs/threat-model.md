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

## Current mitigations

* Strict Pydantic v2 models cover config, principals, envelopes and concept frontmatter.
* `ruamel.yaml` keeps deterministic serialisation under control.
* Reserved-path enforcement happens before filesystem writes.
* Bundle scan and repository audit cover every concept file.
* Authorisation is configured by role and namespace.

## Pending evidence

These controls are implemented as described in code and tests. Production deployment evidence -- especially around reverse proxies, transport context handling and operator hardening -- is still pending.
