GO ?= go
ORACLE_PYTHON ?= python3
UMCP_REFERENCE ?=

.PHONY: check test vet format-check build race cross oracle coverage fuzz model-test corpus-test
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
	CGO_ENABLED=0 $(GO) test ./umcp -run='^$$' -fuzz=FuzzJSONRPCValidation -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./umcp -run='^$$' -fuzz=FuzzHTTPRules -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./umcp -run='^$$' -fuzz=FuzzDispatch -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./umcp -run='^$$' -fuzz=FuzzPagination -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./umcp -run='^$$' -fuzz=FuzzStdio -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./umcp -run='^$$' -fuzz=FuzzURLRules -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./umcp -run='^$$' -fuzz=FuzzToolResults -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./umcp -run='^$$' -fuzz=FuzzHTTPPost -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./umcp -run='^$$' -fuzz=FuzzStreamLine -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./umcp -run='^$$' -fuzz=FuzzLegacySSE -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./umcp -run='^$$' -fuzz=FuzzAsyncHTTPParser -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./umcp -run='^$$' -fuzz=FuzzSyncHTTPParser -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./internal/vector -run='^$$' -fuzz=FuzzF32LE -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./gte -run='^$$' -fuzz=FuzzTokenizer -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./gte -run='^$$' -fuzz=FuzzModelParser -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./needle -run='^$$' -fuzz=FuzzModel -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./sentencepiece -run='^$$' -fuzz=FuzzProcessor -fuzztime=10000x -parallel=2 -timeout=120s
	CGO_ENABLED=0 $(GO) test ./embedding -run='^$$' -fuzz=FuzzFrame -fuzztime=10000x -parallel=2 -timeout=120s
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
