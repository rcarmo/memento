# Canonical project scratch: resolved by vendored tools/project-env.sh.
# Retained build/profiles is outside disposable BUILD_ROOT and clean.
SHELL := $(abspath ../tools/project-env.sh)
BUILD_ROOT := $(shell bash ../tools/project-env.sh --print-build)
ifeq ($(strip $(BUILD_ROOT)),)
$(error Invalid project temporary root; see tools/project-tmp.sh)
endif
TOOLS_DIR := $(shell bash ../tools/project-env.sh --print-tools)
export BUILD_ROOT TOOLS_DIR
GO ?= go
GOTOOLCHAIN ?= go1.26.6
export GOTOOLCHAIN
STATICCHECK_VERSION ?= v0.6.1
GOVULNCHECK_VERSION ?= v1.1.4
PROFILE_ROOT ?= $(abspath ../build/profiles)
PROFILE_MEM_RATE ?= 524288
STATICCHECK := $(TOOLS_DIR)/staticcheck
GOVULNCHECK := $(TOOLS_DIR)/govulncheck
PROFILE_TEST = GO="$(GO)" PROFILE_ROOT="$(PROFILE_ROOT)" PROFILE_MEM_RATE="$(PROFILE_MEM_RATE)" ../tools/test-profile.sh
.PHONY: check audit tools format-check vet lint vuln test race coverage fuzz cross
check: format-check vet lint test coverage cross
audit: check vuln
tools: $(STATICCHECK) $(GOVULNCHECK)
$(STATICCHECK):
	@mkdir -p $(TOOLS_DIR)
	GOBIN="$(abspath $(TOOLS_DIR))" $(GO) install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
$(GOVULNCHECK):
	@mkdir -p $(TOOLS_DIR)
	GOBIN="$(abspath $(TOOLS_DIR))" $(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
format-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go'))" || (echo 'Run gofmt before checking'; exit 1)
	@$(GO) mod tidy -diff
vet:
	$(GO) vet ./...
lint: $(STATICCHECK)
	$(STATICCHECK) ./...
vuln: $(GOVULNCHECK)
	$(GOVULNCHECK) ./...
test:
	CGO_ENABLED=0 $(PROFILE_TEST) ./...
race:
	CGO_ENABLED=1 $(PROFILE_TEST) ./... -- -race
coverage:
	@mkdir -p "$(BUILD_ROOT)"
	CGO_ENABLED=0 COVERAGE_FILE=$(BUILD_ROOT)/umcp-coverage.out $(PROFILE_TEST) . -- -covermode=atomic
	@awk 'NR>1 { count[$$1] += $$3 } END { bad=0; for (block in count) if (count[block]==0) { print "UNCOVERED:",block; bad=1 } exit bad }' $(BUILD_ROOT)/umcp-coverage.out
fuzz:
	GO="$(GO)" PROFILE_ROOT="$(PROFILE_ROOT)" PROFILE_MEM_RATE="$(PROFILE_MEM_RATE)" FUZZ_COUNT="$${FUZZ_COUNT:-10000x}" FUZZ_TIMEOUT="$${FUZZ_TIMEOUT:-120s}" FUZZ_PARALLEL="$${FUZZ_PARALLEL:-2}" ../tools/run-fuzz.sh
cross:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1 $(GO) test -c -o $(BUILD_ROOT)/umcp-linux-amd64.test .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) test -c -o $(BUILD_ROOT)/umcp-linux-arm64.test .
