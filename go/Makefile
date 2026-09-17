GO ?= go
ORACLE_PYTHON ?= python3
UMCP_REFERENCE ?=

.PHONY: check test vet format-check build race cross oracle coverage fuzz
check: format-check vet coverage build
format-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go'))" || (echo 'Run gofmt before checking'; exit 1)
vet:
	CGO_ENABLED=0 $(GO) vet ./...
test:
	CGO_ENABLED=0 $(GO) test ./...
coverage:
	@mkdir -p ../build/go
	CGO_ENABLED=0 $(GO) test -coverpkg=./... -covermode=atomic -coverprofile=../build/go/coverage.out ./...
	$(GO) tool cover -func=../build/go/coverage.out
	@awk 'NR>1 { count[$$1] += $$3 } END { bad=0; for (block in count) if (count[block]==0) { print "UNCOVERED:",block; bad=1 } exit bad }' ../build/go/coverage.out
fuzz:
	CGO_ENABLED=0 $(GO) test ./umcp -run='^$$' -fuzz=FuzzJSONRPCValidation -fuzztime=5s -parallel=2
	CGO_ENABLED=0 $(GO) test ./internal/vector -run='^$$' -fuzz=FuzzF32LE -fuzztime=5s -parallel=2
build:
	CGO_ENABLED=0 $(GO) build -o ../build/go/memento-go ./cmd/memento-go
race:
	# The Go race detector needs cgo in the test toolchain, not the shipped runtime.
	CGO_ENABLED=1 $(GO) test -race ./...
cross:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1 $(GO) build -o ../build/go/memento-go-linux-amd64 ./cmd/memento-go
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -o ../build/go/memento-go-linux-arm64 ./cmd/memento-go
oracle:
	@test -n "$(UMCP_REFERENCE)" || (echo 'Set UMCP_REFERENCE to the pinned uMCP checkout'; exit 1)
	$(ORACLE_PYTHON) oracle/export.py --umcp "$(UMCP_REFERENCE)"
	CARGO_TARGET_DIR=../build/go/oracle-target cargo run --locked --quiet --manifest-path oracle/rust-vector/Cargo.toml > testdata/parity/vectors.json
