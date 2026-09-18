GO ?= go
GOTOOLCHAIN ?= go1.26.6
export GOTOOLCHAIN
ORACLE_PYTHON ?= python3
UMCP_REFERENCE ?=
TOOLS_DIR ?= ../build/go/tools
STATICCHECK_VERSION ?= v0.6.1
GOVULNCHECK_VERSION ?= v1.1.4
STATICCHECK := $(TOOLS_DIR)/staticcheck
GOVULNCHECK := $(TOOLS_DIR)/govulncheck

.PHONY: help tools check quality audit-fast audit test vet lint vuln format format-check layout-check build race cross oracle coverage fuzz model-test corpus-test
help:
	@printf '%s\n' 'quality     format-check, layout, vet, staticcheck, tests, coverage and build' 'audit-fast  quality plus vulnerability scan' 'audit       audit-fast plus race, fuzz and cross-build gates' 'tools       install pinned static-analysis tools under ../build/go/tools' 'format      apply gofmt' 'test        run offline pure-Go tests' 'coverage    require zero uncovered Go statements' 'lint        run pinned staticcheck (correctness, style, simplification, unused)' 'vuln        run pinned govulncheck' 'fuzz        discover and run every Go fuzz target' 'race        run all tests with the race detector' 'cross       build Linux amd64 and arm64 commands'
tools: $(STATICCHECK) $(GOVULNCHECK)
$(STATICCHECK):
	@mkdir -p $(TOOLS_DIR)
	GOBIN="$(abspath $(TOOLS_DIR))" $(GO) install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
$(GOVULNCHECK):
	@mkdir -p $(TOOLS_DIR)
	GOBIN="$(abspath $(TOOLS_DIR))" $(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
check: quality
quality: format-check layout-check vet lint test coverage build
audit-fast: quality vuln
audit: audit-fast race fuzz cross
format:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')
format-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go'))" || (echo 'Run gofmt before checking'; exit 1)
layout-check:
	@test -z "$$(find . -maxdepth 1 -name '*.go' -print)" || (echo 'Move root Go files into a package or cmd/<name>'; exit 1)
	@test -z "$$(find cmd -mindepth 1 -maxdepth 1 -type f -print)" || (echo 'Place each command in cmd/<name>/'; exit 1)
	@$(GO) list ./... >/dev/null
	@$(GO) mod tidy -diff
vet:
	CGO_ENABLED=0 $(GO) vet ./...
lint: $(STATICCHECK)
	CGO_ENABLED=0 $(STATICCHECK) ./...
vuln: $(GOVULNCHECK)
	CGO_ENABLED=0 $(GOVULNCHECK) ./...
test:
	CGO_ENABLED=0 $(GO) test ./...
coverage:
	@mkdir -p ../build/go
	CGO_ENABLED=0 $(GO) test -coverpkg=./... -covermode=atomic -coverprofile=../build/go/coverage.out ./...
	$(GO) tool cover -func=../build/go/coverage.out
	@awk 'NR>1 { count[$$1] += $$3 } END { bad=0; for (block in count) if (count[block]==0) { print "UNCOVERED:",block; bad=1 } exit bad }' ../build/go/coverage.out
fuzz:
	GO="$(GO)" FUZZ_COUNT="$${FUZZ_COUNT:-10000x}" FUZZ_TIMEOUT="$${FUZZ_TIMEOUT:-120s}" FUZZ_PARALLEL="$${FUZZ_PARALLEL:-2}" ./tools/run-fuzz.sh

model-test:
	@test -n "$(GTE_MODEL_PATH)" || (echo 'Set GTE_MODEL_PATH to the digest-pinned public GTE1 model'; exit 1)
	CGO_ENABLED=0 GTE_MODEL_PATH="$(GTE_MODEL_PATH)" $(GO) test ./gte -run TestRealGTEModel -count=1 -v
	@test -n "$(NEEDLE_MODEL_PATH)" || (echo 'Set NEEDLE_MODEL_PATH to the digest-pinned NDL1 model'; exit 1)
	@test -n "$(NEEDLE_TOKENIZER_PATH)" || (echo 'Set NEEDLE_TOKENIZER_PATH to the pinned SentencePiece model'; exit 1)
	CGO_ENABLED=0 NEEDLE_MODEL_PATH="$(NEEDLE_MODEL_PATH)" NEEDLE_TOKENIZER_PATH="$(NEEDLE_TOKENIZER_PATH)" $(GO) test ./needle -run 'TestRealNeedle(Model|Tokenizer|Generation)' -count=1 -v -timeout=10m
corpus-test:
	@test -n "$(NEEDLE_MODEL_PATH)" -a -n "$(NEEDLE_TOKENIZER_PATH)" || (echo 'Corpus gate requires pinned Needle model and tokenizer paths'; exit 1)
	CGO_ENABLED=0 NEEDLE_FULL_CORPUS=1 NEEDLE_MODEL_PATH="$(NEEDLE_MODEL_PATH)" NEEDLE_TOKENIZER_PATH="$(NEEDLE_TOKENIZER_PATH)" $(GO) test ./needle -run TestRealNeedleCorpus -count=1 -v -timeout=80m
build:
	CGO_ENABLED=0 $(GO) build -o ../build/go/memento-go ./cmd/memento-go
	CGO_ENABLED=0 $(GO) build -o ../build/go/memento-embed-go ./cmd/memento-embed-go
race:
	# The Go race detector needs cgo in the test toolchain, not the shipped runtime.
	CGO_ENABLED=1 $(GO) test -race ./...
cross:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1 $(GO) build -o ../build/go/memento-go-linux-amd64 ./cmd/memento-go
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -o ../build/go/memento-go-linux-arm64 ./cmd/memento-go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1 $(GO) build -o ../build/go/memento-embed-go-linux-amd64 ./cmd/memento-embed-go
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -o ../build/go/memento-embed-go-linux-arm64 ./cmd/memento-embed-go
oracle:
	@test -n "$(UMCP_REFERENCE)" || (echo 'Set UMCP_REFERENCE to the pinned uMCP checkout'; exit 1)
	$(ORACLE_PYTHON) oracle/export.py --umcp "$(UMCP_REFERENCE)"
	CARGO_TARGET_DIR=../build/go/oracle-target cargo run --locked --quiet --manifest-path oracle/rust-vector/Cargo.toml > testdata/parity/vectors.json
	CARGO_TARGET_DIR=../build/go/oracle-target cargo run --locked --quiet --manifest-path oracle/rust-gte/Cargo.toml -- tokenizer > testdata/parity/gte-tokenizer.json
	CARGO_TARGET_DIR=../build/go/oracle-target cargo run --locked --quiet --manifest-path oracle/rust-gte/Cargo.toml -- inference > testdata/parity/gte-inference.json
	CARGO_TARGET_DIR=../build/go/oracle-target cargo run --locked --quiet --manifest-path oracle/rust-gte/Cargo.toml -- protocol > testdata/parity/embedding-protocol.json
	CARGO_TARGET_DIR=../build/go/oracle-target cargo run --locked --quiet --manifest-path oracle/rust-needle/Cargo.toml -- synthetic-tokenizers > testdata/parity/sentencepiece.json
