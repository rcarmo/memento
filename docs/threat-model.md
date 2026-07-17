# Threat model

## Trust boundaries

```text
client tool input (untrusted)
  -> authenticated principal from trusted transport context
  -> deterministic validation and authorization
  -> repository filesystem boundary
  -> canonical markdown bundle
```

## Deterministic-core threats

1. **Path traversal**: reject absolute paths, `..`, symlink components and unsafe targets.
2. **Reserved-file overwrite**: reject direct writes to generated files such as `index.md` and root `log.md`.
3. **Special-file abuse**: reject writes to device files, FIFOs and non-regular existing targets.
4. **Malformed frontmatter**: parse with `python-frontmatter`, validate with strict Pydantic models.
5. **Schema confusion**: reject unknown frontmatter keys and out-of-vocabulary `type` values.
6. **Markdown rewrite corruption**: use `markdown-it-py` token structure instead of regular expressions.
7. **Link integrity drift**: audit broken links and duplicate IDs on every repository scan.
8. **Authorization bypass**: consume principal identity from trusted request context, never from tool arguments.

## Initial mitigations

- strict Pydantic v2 models for config, principals, envelopes and concept frontmatter
- deterministic serialization with `ruamel.yaml`
- reserved-path enforcement before filesystem writes
- bundle scan and repository audit over every concept file
- role and namespace-based authorization configuration
