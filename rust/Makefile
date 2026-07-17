.PHONY: fmt fmt-check clippy test check
fmt:
	cargo fmt --all
fmt-check:
	cargo fmt --all --check
clippy:
	cargo clippy --workspace --all-targets

test:
	cargo test --workspace
check: fmt-check clippy test
