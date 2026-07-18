# GTE-small model

`gte-small.gtemodel` is the FP32 GTE1 model used by Memento's local semantic-search runtime and Rust parity tests.

It was converted from [`thenlper/gte-small`](https://huggingface.co/thenlper/gte-small) with [`rcarmo/go-gte`](https://github.com/rcarmo/go-gte). The expected SHA-256 is:

```text
06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171
```

The file is tracked with Git LFS. Fetch it after cloning:

```bash
git lfs pull
```

Regenerate the model and Rust parity data with:

```bash
rust/tests/scripts/generate_golden.sh
```

That script expects the upstream source model under `/tmp/go-gte/models/gte-small/`. Runtime and model licensing details are in [`docs/attribution.md`](../../docs/attribution.md).
