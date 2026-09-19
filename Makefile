GO ?= go
MEMENTO_VERSION ?= 1.0.0
SOURCE_DATE_EPOCH ?= 0

.PHONY: check quality audit test performance model-test corpus-test release release-check go-container-build go-container-contract clean

check quality:
	$(MAKE) -C go quality

audit:
	$(MAKE) -C go audit GTE_MODEL_PATH="$(abspath models/gte/gte-small.gtemodel)"

test:
	$(MAKE) -C go test

performance:
	$(MAKE) -C go performance \
		GTE_MODEL_PATH="$(abspath models/gte/gte-small.gtemodel)" \
		NEEDLE_MODEL_PATH="$(abspath models/needle/memento-router.ndl)" \
		NEEDLE_TOKENIZER_PATH="$(abspath models/needle/needle.model)"

model-test:
	$(MAKE) -C go model-test \
		GTE_MODEL_PATH="$(abspath models/gte/gte-small.gtemodel)" \
		NEEDLE_MODEL_PATH="$(abspath models/needle/memento-router.ndl)" \
		NEEDLE_TOKENIZER_PATH="$(abspath models/needle/needle.model)"

corpus-test:
	$(MAKE) -C go corpus-test \
		NEEDLE_MODEL_PATH="$(abspath models/needle/memento-router.ndl)" \
		NEEDLE_TOKENIZER_PATH="$(abspath models/needle/needle.model)"

release:
	$(MAKE) -C go release VERSION="$(MEMENTO_VERSION)" SOURCE_DATE_EPOCH="$(SOURCE_DATE_EPOCH)"

release-check:
	$(MAKE) -C go release-check VERSION="$(MEMENTO_VERSION)" SOURCE_DATE_EPOCH="$(SOURCE_DATE_EPOCH)"

go-container-build:
	docker build --build-arg VERSION="$(MEMENTO_VERSION)" --build-arg COMMIT="$$(git rev-parse HEAD)" --build-arg BUILD_DATE="$$(date -u +'%Y-%m-%dT%H:%M:%SZ')" -t memento-go:contract .

go-container-contract: go-container-build
	IMAGE=memento-go:contract VERSION="$(MEMENTO_VERSION)" tools/test_go_container_contract.sh

clean:
	rm -rf build
	$(MAKE) -C go clean
