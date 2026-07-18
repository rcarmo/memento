PYTHON ?= python3
VENV ?= .venv
BIN := $(VENV)/bin

.PHONY: install install-dev lint format format-check typecheck test coverage rust-format-check rust-lint rust-test rust-check check build-wheel install-wheel diff-check load-functional load-operational load-check clean

$(BIN)/python:
	$(PYTHON) -m venv $(VENV)

install: $(BIN)/python
	$(BIN)/python -m pip install -e .

install-dev: $(BIN)/python
	$(BIN)/python -m pip install -e '.[dev]' build

lint:
	$(BIN)/ruff check src tests tools

format:
	$(BIN)/ruff format src tests tools

format-check:
	$(BIN)/ruff format --check src tests tools

typecheck:
	$(BIN)/mypy src tests

test:
	$(BIN)/pytest -q

coverage:
	$(BIN)/pytest --cov=memento --cov-branch --cov-report=term-missing

rust-format-check:
	cd rust && cargo fmt --all --check

rust-lint:
	cd rust && cargo clippy --workspace --all-targets -- -D warnings

rust-test:
	cd rust && cargo test --workspace

rust-check: rust-format-check rust-lint rust-test

check: lint format-check typecheck test rust-check

build-wheel:
	$(BIN)/python -m build --wheel

install-wheel: build-wheel
	$(BIN)/python -m pip install --force-reinstall dist/*.whl

diff-check:
	git diff --exit-code -- . ':(exclude).coverage'

load-functional:
	PYTHONPATH=src $(BIN)/python tools/load_test.py --profile functional --concepts 12 --workers 4 --requests 24 --output build/load-functional.json

load-operational:
	PYTHONPATH=src $(BIN)/python tools/load_test.py --profile operational --concepts 12 --workers 4 --requests 24 --output build/load-operational.json

load-check:
	PYTHONPATH=src $(BIN)/python tools/load_test.py --profile check --concepts 8 --workers 3 --requests 12 --output build/load-check.json

clean:
	rm -rf $(VENV) .pytest_cache .ruff_cache .mypy_cache .coverage htmlcov build dist *.egg-info src/*.egg-info
	find . -type d -name __pycache__ -prune -exec rm -rf {} +
