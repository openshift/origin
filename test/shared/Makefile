all: lint
.PHONY: all

# Run unit tests for this module.
#
# The JUNIT_REPORT variables mirror the contract of hack/test-go.sh, which
# delegates to this target when it runs under CI: when JUNIT_REPORT is set,
# output is emitted as test2json JSON, appended to JUNIT_REPORT_OUTPUT, and
# converted to jUnit XML with gotest2junit (built and put on PATH by
# hack/test-go.sh).
GOTEST_FLAGS ?= -race -timeout 240s
JUNIT_REPORT ?=
JUNIT_REPORT_OUTPUT ?= raw_test_output.log
JUNIT_REPORT_FILE ?= unit_shared_report.xml

# bash with pipefail so a failing `go test` fails the recipe even though it
# sits at the head of the jUnit conversion pipeline.
SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -c

test-unit:
	if [ -n "$(JUNIT_REPORT)" ]; then \
		go test -json $(GOTEST_FLAGS) ./... | tee -a "$(JUNIT_REPORT_OUTPUT)" | gotest2junit > "$(JUNIT_REPORT_FILE)"; \
	else \
		go test $(GOTEST_FLAGS) ./...; \
	fi
.PHONY: test-unit

# golangci-lint defaults its cache to ~/.cache/golangci-lint, which is not
# writable in CI containers where HOME=/. Default it next to the Go build
# cache, which is writable wherever `go` itself can run.
GOLANGCI_LINT_CACHE ?= $(shell go env GOCACHE)/golangci-lint
export GOLANGCI_LINT_CACHE

# Run golangci-lint using the tool pinned in go.mod (see the tool directive).
# Vendoring keeps the build hermetic; no network access is required.
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint run ./...
.PHONY: lint

# Refresh go.mod, go.sum and vendor/
# Also called automatically by the same target in the root module
update-gomod:
	go mod tidy
	go mod vendor
.PHONY: update-gomod

# Verify that go.mod, go.sum and vendor/ are exactly what the go toolchain
# produces. Mirrors the org-wide go-verify-deps CI step (tidy, vendor, clean
# tree) rather than the root module's deps.diff flow.
verify-deps:
	go mod tidy
	go mod vendor
	if git status --porcelain -- go.mod go.sum vendor | grep -q .; then \
		echo "ERROR: test/shared dependencies are out of date. Run 'go mod tidy && go mod vendor' in test/shared and commit the results."; \
		git status --short -- go.mod go.sum vendor; \
		exit 1; \
	fi
.PHONY: verify-deps
