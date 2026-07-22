PYTHON ?= python3
VENV ?= .venv
BIN := $(VENV)/bin
MEMENTO_VERSION ?= 0.3.0-rc.17
RELEASE_TAG ?= v$(MEMENTO_VERSION)
PORTAINER_URL ?= https://ops.local:9443
PICLAW ?= piclaw

.PHONY: install install-dev lint format format-check typecheck test coverage graph-check rust-format-check rust-lint rust-test rust-check check build-wheel install-wheel diff-check release-wait deploy-diskstation verify-diskstation release-deploy-diskstation load-functional load-operational load-check clean

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

graph-check:
	bun tools/vendor_graph_libraries.ts --check
	bun build src/memento/graph_debug/static/app.js --target=browser --format=esm --outfile=/tmp/memento-graph-check.js >/dev/null
	rm -f /tmp/memento-graph-check.js

rust-format-check:
	cd rust && cargo fmt --all --check

rust-lint:
	cd rust && cargo clippy --workspace --all-targets -- -D warnings

rust-test:
	cd rust && cargo test --workspace

rust-check: rust-format-check rust-lint rust-test

check: lint format-check typecheck test graph-check rust-check

build-wheel:
	$(BIN)/python -m build --wheel

install-wheel: build-wheel
	$(BIN)/python -m pip install --force-reinstall dist/*.whl

diff-check:
	git diff --exit-code -- . ':(exclude).coverage'

release-wait:
	@PICLAW="$(PICLAW)" $(BIN)/python tools/release_deploy.py wait-release "$(RELEASE_TAG)"

deploy-diskstation:
	@PORTAINER_URL="$(PORTAINER_URL)" PICLAW="$(PICLAW)" \
		$(BIN)/python tools/release_deploy.py deploy "$(MEMENTO_VERSION)"

verify-diskstation:
	$(BIN)/python tools/release_deploy.py verify

release-deploy-diskstation: release-wait deploy-diskstation verify-diskstation

load-functional:
	PYTHONPATH=src $(BIN)/python tools/load_test.py --profile functional --concepts 12 --workers 4 --requests 24 --output build/load-functional.json

load-operational:
	PYTHONPATH=src $(BIN)/python tools/load_test.py --profile operational --concepts 12 --workers 4 --requests 24 --output build/load-operational.json

load-check:
	PYTHONPATH=src $(BIN)/python tools/load_test.py --profile check --concepts 8 --workers 3 --requests 12 --output build/load-check.json

clean:
	rm -rf $(VENV) .pytest_cache .ruff_cache .mypy_cache .coverage htmlcov build dist *.egg-info src/*.egg-info
	find . -type d -name __pycache__ -prune -exec rm -rf {} +
