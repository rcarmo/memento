# Attribution

## Rust workspace

The Rust implementation under `rust/` includes code derived from and validated against the MIT-licensed `/tmp/go-gte` reference implementation. That attribution applies to:

* `rust/crates/memento-gte`
* `rust/crates/memento-vector`
* `rust/crates/memento-embed`
* `rust/crates/memento-sqlite-vector`
* `rust/crates/memento-ffi`

`memento-ffi` exposes the same Rust embedding and vector functionality through a stable C ABI, and keeps the same attribution chain intact.

## GTE-small model

The repository vendors the FP32 `gte-small.gtemodel` generated from [`thenlper/gte-small`](https://huggingface.co/thenlper/gte-small) through the `rcarmo/go-gte` conversion tooling. The file is `rust/tests/fixtures/gte-small.gtemodel`, is about 128 MB, and has SHA-256 `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`.

Runtime code is MIT licensed. The model artefact follows the upstream model card and repository licensing terms; release manifests must retain its source and digest.

## Pending evidence

Source provenance is documented here. Additional release-time evidence -- such as shipped artefact manifests tying those crates to published binaries -- remains pending.
