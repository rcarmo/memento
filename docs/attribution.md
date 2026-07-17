# Attribution

## Rust workspace

The Rust implementation under `rust/` includes code derived from and validated against the MIT-licensed `/tmp/go-gte` reference implementation. This includes:

- `rust/crates/memento-gte`
- `rust/crates/memento-vector`
- `rust/crates/memento-embed`
- `rust/crates/memento-sqlite-vector`
- `rust/crates/memento-ffi`

The `memento-ffi` crate exposes the same Rust embedding and vector functionality through a stable C ABI, while preserving the same attribution chain.
