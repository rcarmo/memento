# Golden generation

The Rust workspace keeps parity fixtures with MIT-attributed data generated against `/tmp/go-gte`. This is a fixture-generation path, not part of the normal runtime build.

## Inputs

* `/tmp/go-gte/convert_model.py`
* `/tmp/go-gte/models/gte-small/config.json`
* `/tmp/go-gte/models/gte-small/model.safetensors`
* `/tmp/go-gte/models/gte-small/vocab.txt`

## Outputs

* `rust/tests/fixtures/gte-small.gtemodel`
* `rust/tests/fixtures/go_parity.json`

## Prerequisites

Some tracked fixtures in this repository use Git LFS. Install LFS and pull them before regenerating or auditing goldens:

```bash
git lfs install
git lfs pull
```

## Command

```bash
cd rust
./tests/scripts/generate_golden.sh
```

## What the script does

* Converts the upstream safetensors model into `GTE1` and writes `rust/tests/fixtures/gte-small.gtemodel`.
* Builds a temporary Go program that loads the converted model through `/tmp/go-gte`, tokenises a fixed text set and emits embeddings.
* Writes the resulting parity record to `rust/tests/fixtures/go_parity.json` with source and licence metadata.
* Removes the temporary Go source file on exit.

The script expects `/tmp/go-gte/models/gte-small/model.safetensors` to exist and stops with an error if it does not. It temporarily exposes the Go tokenizer for fixture extraction, generates token and embedding goldens, and leaves the upstream tree otherwise untouched.

## Validation status

Golden generation is intentionally separate from the standard Rust validation path. `make rust-check` at the repository root and `make check` in `rust/` do not run this script, because it depends on external model inputs and a local `/tmp/go-gte` checkout. If the repository later needs routine golden regeneration, add a dedicated target and document its prerequisites rather than making it an implicit part of ordinary checks.
