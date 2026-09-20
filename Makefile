GO ?= go
GOTOOLCHAIN ?= go1.26.6
export GOTOOLCHAIN
TOOLS_DIR ?= build/tools
STATICCHECK_VERSION ?= v0.6.1
GOVULNCHECK_VERSION ?= v1.1.4
STATICCHECK := $(TOOLS_DIR)/staticcheck
GOVULNCHECK := $(TOOLS_DIR)/govulncheck
PREFIX ?= /usr/local
DESTDIR ?=
MEMENTO_VERSION ?= 1.0.0
VERSION ?= $(MEMENTO_VERSION)
SOURCE_DATE_EPOCH ?= 0
GTE_MODEL_PATH ?= $(abspath models/gte/gte-small.gtemodel)
NEEDLE_MODEL_PATH ?= $(abspath models/needle/memento-router.ndl)
NEEDLE_TOKENIZER_PATH ?= $(abspath models/needle/needle.model)

.PHONY: help tools check quality audit-fast audit test vet lint vuln format format-check layout-check fuzz-coverage build race cross release release-check install coverage fuzz performance performance-core model-test simd-test corpus-test go-container-build go-container-contract clean
help:
	@printf '%s\n' 'quality     format-check, layout, vet, staticcheck, tests, coverage and build' 'audit-fast  quality plus vulnerability scan' 'audit       audit-fast plus race, fuzz, model-independent performance and cross-build gates' 'performance run allocation gates including pinned real models' 'performance-core run model-independent allocation gates' 'tools       install pinned static-analysis tools under build/tools' 'format      apply gofmt' 'test        run offline pure-Go tests' 'coverage    require zero uncovered Go statements' 'lint        run pinned staticcheck' 'vuln        run pinned govulncheck' 'fuzz        discover and run every Go fuzz target' 'race        run all tests with the race detector' 'cross       build Linux amd64 and arm64 commands' 'release     build reproducible static amd64/arm64 archives' 'release-check verify release checksums/layout/ELF metadata and smoke test' 'install     install all host binaries under DESTDIR/PREFIX/bin'
tools: $(STATICCHECK) $(GOVULNCHECK)
$(STATICCHECK):
	@mkdir -p $(TOOLS_DIR)
	GOBIN="$(abspath $(TOOLS_DIR))" $(GO) install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
$(GOVULNCHECK):
	@mkdir -p $(TOOLS_DIR)
	GOBIN="$(abspath $(TOOLS_DIR))" $(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
check: quality
quality: format-check layout-check fuzz-coverage vet lint test graph-test python-parity coverage umcp-check build
audit-fast: quality vuln
audit: audit-fast race fuzz performance-core cross
format:
	gofmt -w $$(find cmd internal tools -name '*.go')
format-check:
	@test -z "$$(gofmt -l $$(find cmd internal tools -name '*.go'))" || (echo 'Run gofmt before checking'; exit 1)
layout-check:
	@test -z "$$(find . -maxdepth 1 -name '*.go' -print)" || (echo 'Move root Go files into a package or cmd/<name>'; exit 1)
	@test -z "$$(find cmd -mindepth 1 -maxdepth 1 -type f -print)" || (echo 'Place each command in cmd/<name>/'; exit 1)
	@test ! -d go || (echo 'Legacy go/ tree must not exist'; exit 1)
	@$(GO) list ./... >/dev/null
	@$(GO) mod tidy -diff
fuzz-coverage:
	GO="$(GO)" ./tools/check-fuzz-coverage.sh
vet:
	CGO_ENABLED=0 $(GO) vet ./...
lint: $(STATICCHECK)
	CGO_ENABLED=0 $(STATICCHECK) ./...
vuln: $(GOVULNCHECK)
	CGO_ENABLED=0 $(GOVULNCHECK) ./...
test:
	CGO_ENABLED=0 $(GO) test ./...
.PHONY: mcp-contract graph-test ui-test python-parity
# Native-Go final parity gate. JavaScript/Python tools only regenerate oracle artifacts.
python-parity:
	CGO_ENABLED=0 $(GO) test ./... -count=1
	$(MAKE) -C umcp test
# Install pinned browser tooling with: cd tools/browser && npm ci && npx playwright install chromium
ui-test:
	node tools/browser/audit.mjs
graph-test:
	node --test tools/graph-semantic.test.mjs
# Focused startup/wire contracts plus the standalone transport contract suite.
mcp-contract:
	CGO_ENABLED=0 $(GO) test ./internal/service -run '^TestMCPContract' -count=1
	$(MAKE) -C umcp test
umcp-check:
	$(MAKE) -C umcp check
coverage:
	@mkdir -p build
	CGO_ENABLED=0 $(GO) test -coverpkg="$$( $(GO) list ./... | paste -sd, - )" -covermode=atomic -coverprofile=build/coverage.out ./...
	$(GO) tool cover -func=build/coverage.out
	@awk 'NR>1 && $$2>0 { count[$$1] += $$3 } END { bad=0; for (block in count) if (count[block]==0) { print "UNCOVERED:",block; bad=1 } exit bad }' build/coverage.out
fuzz:
	GO="$(GO)" FUZZ_COUNT="$${FUZZ_COUNT:-10000x}" FUZZ_TIMEOUT="$${FUZZ_TIMEOUT:-120s}" FUZZ_PARALLEL="$${FUZZ_PARALLEL:-2}" ./tools/run-fuzz.sh
	$(MAKE) -C umcp fuzz FUZZ_COUNT="$${FUZZ_COUNT:-10000x}" FUZZ_TIMEOUT="$${FUZZ_TIMEOUT:-120s}" FUZZ_PARALLEL="$${FUZZ_PARALLEL:-2}"
performance-core:
	GTE_MODEL_PATH= NEEDLE_MODEL_PATH= NEEDLE_TOKENIZER_PATH= ./tools/check-performance.sh
performance:
	GTE_MODEL_PATH="$(GTE_MODEL_PATH)" NEEDLE_MODEL_PATH="$(NEEDLE_MODEL_PATH)" NEEDLE_TOKENIZER_PATH="$(NEEDLE_TOKENIZER_PATH)" ./tools/check-performance.sh
model-test:
	@test -n "$(GTE_MODEL_PATH)" || (echo 'Set GTE_MODEL_PATH to the digest-pinned public GTE1 model'; exit 1)
	CGO_ENABLED=0 GTE_MODEL_PATH="$(GTE_MODEL_PATH)" $(GO) test ./internal/gte -run TestRealGTEModel -count=1 -v
	@test -n "$(NEEDLE_MODEL_PATH)" -a -n "$(NEEDLE_TOKENIZER_PATH)" || (echo 'Set NEEDLE_MODEL_PATH and NEEDLE_TOKENIZER_PATH'; exit 1)
	CGO_ENABLED=0 NEEDLE_MODEL_PATH="$(NEEDLE_MODEL_PATH)" NEEDLE_TOKENIZER_PATH="$(NEEDLE_TOKENIZER_PATH)" $(GO) test ./internal/needle -run 'TestRealNeedle(Model|Tokenizer|Generation|MappedGeneration)' -count=1 -v -timeout=10m
simd-test:
	CGO_ENABLED=0 GOAMD64=v1 $(GO) build -o build/memento-simd-check ./cmd/memento-simd-check
	GODEBUG=cpu.avx=off,cpu.avx2=off,cpu.fma=off build/memento-simd-check | grep -E '^arch=amd64 backend=sse2 sse2=true avx2_fma=false '
corpus-test:
	@test -n "$(NEEDLE_MODEL_PATH)" -a -n "$(NEEDLE_TOKENIZER_PATH)" || (echo 'Corpus gate requires pinned Needle model and tokenizer paths'; exit 1)
	CGO_ENABLED=0 NEEDLE_FULL_CORPUS=1 NEEDLE_MODEL_PATH="$(NEEDLE_MODEL_PATH)" NEEDLE_TOKENIZER_PATH="$(NEEDLE_TOKENIZER_PATH)" $(GO) test ./internal/needle -run TestRealNeedleCorpus -count=1 -v -timeout=80m
build:
	@mkdir -p build
	CGO_ENABLED=0 $(GO) build -o build/memento-go ./cmd/memento-go
	CGO_ENABLED=0 $(GO) build -o build/memento-embed-go ./cmd/memento-embed-go
	CGO_ENABLED=0 $(GO) build -o build/memento-needle-go ./cmd/memento-needle-go
	CGO_ENABLED=0 $(GO) build -o build/memento-needle-model-go ./cmd/memento-needle-model-go
	CGO_ENABLED=0 $(GO) build -o build/memento-skill-import-go ./cmd/memento-skill-import-go
race:
	CGO_ENABLED=1 $(GO) test -race ./...
	$(MAKE) -C umcp race
install: build
	install -d "$(DESTDIR)$(PREFIX)/bin"
	for name in memento-go memento-embed-go memento-needle-go memento-needle-model-go memento-skill-import-go; do install -m 0755 "build/$$name" "$(DESTDIR)$(PREFIX)/bin/$$name"; done
release:
	GO="$(GO)" VERSION="$(VERSION)" SOURCE_DATE_EPOCH="$(SOURCE_DATE_EPOCH)" ./tools/release.sh
release-check: release
	GO="$(GO)" VERSION="$(VERSION)" ./tools/release-check.sh
cross: simd-test
	$(MAKE) -C umcp cross
	@mkdir -p build
	@for arch in amd64 arm64; do \
		amd=; test "$$arch" != amd64 || amd=GOAMD64=v1; \
		for name in memento-go memento-embed-go memento-needle-go memento-needle-model-go memento-skill-import-go; do \
			env CGO_ENABLED=0 GOOS=linux GOARCH="$$arch" $$amd $(GO) build -o "build/$$name-linux-$$arch" "./cmd/$$name" || exit; \
		done; \
	done
go-container-build:
	docker build --build-arg VERSION="$(MEMENTO_VERSION)" --build-arg COMMIT="$$(git rev-parse HEAD)" --build-arg BUILD_DATE="$$(date -u +'%Y-%m-%dT%H:%M:%SZ')" -t memento-go:contract .
go-container-contract: go-container-build
	IMAGE=memento-go:contract VERSION="$(MEMENTO_VERSION)" tools/test_go_container_contract.sh
clean:
	rm -rf build
