# Golden generation

The Rust workspace carries MIT-attributed parity generation against `/tmp/go-gte`.

## Inputs

- `/tmp/go-gte/convert_model.py`
- `/tmp/go-gte/models/gte-small/config.json`
- `/tmp/go-gte/models/gte-small/model.safetensors`
- `/tmp/go-gte/models/gte-small/vocab.txt`

## Output

- `rust/tests/fixtures/gte-small.gtemodel`
- `rust/tests/fixtures/go_parity.json`

## Command

```bash
cd rust
./tests/scripts/generate_golden.sh
```

The script converts the upstream safetensors model into `GTE1`, temporarily exposes the Go tokenizer for fixture extraction, generates token and embedding goldens, and restores the upstream file afterwards.
