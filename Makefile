PYTHON ?= python3
VENV ?= .venv
BIN := $(VENV)/bin

.PHONY: install install-dev lint format format-check typecheck test coverage check clean

$(BIN)/python:
	$(PYTHON) -m venv $(VENV)

install: $(BIN)/python
	$(BIN)/python -m pip install -e .

install-dev: $(BIN)/python
	$(BIN)/python -m pip install -e '.[dev]'

lint:
	$(BIN)/ruff check src tests

format:
	$(BIN)/ruff format src tests

format-check:
	$(BIN)/ruff format --check src tests

typecheck:
	$(BIN)/mypy src tests

test:
	$(BIN)/pytest -q

coverage:
	$(BIN)/pytest --cov=memento --cov-branch --cov-report=term-missing

check: lint format-check typecheck test

clean:
	rm -rf $(VENV) .pytest_cache .ruff_cache .mypy_cache .coverage htmlcov build dist *.egg-info src/*.egg-info
	find . -type d -name __pycache__ -prune -exec rm -rf {} +
