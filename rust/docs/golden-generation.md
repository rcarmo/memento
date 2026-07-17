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
