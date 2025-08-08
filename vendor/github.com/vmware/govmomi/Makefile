# © Broadcom. All Rights Reserved.
# The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: Apache-2.0

# If you update this file, please follow
# https://www.thapaliya.com/en/writings/well-documented-makefiles/

# Ensure Make is run with bash shell as some syntax below is bash-specific
SHELL := /usr/bin/env bash

# Print the help/usage when make is executed without any other arguments
.DEFAULT_GOAL := help


## --------------------------------------
## Help
## --------------------------------------

.PHONY: help
help: ## Display usage
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make [target] \033[36m\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)


## --------------------------------------
## Locations and programs
## --------------------------------------

# Directories
BIN_DIR       := bin
TOOLS_DIR     := hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin

# Tooling binaries
GO            ?= $(shell command -v go 2>/dev/null)
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/golangci-lint


## --------------------------------------
## Prerequisites
## --------------------------------------

# Do not proceed unless the go binary is present.
ifeq (,$(strip $(GO)))
$(error The "go" program cannot be found)
endif


## --------------------------------------
## Linting and fixing linter errors
## --------------------------------------

.PHONY: lint
lint: ## Run all the lint targets
	$(MAKE) lint-go-full

GOLANGCI_LINT_FLAGS ?= --fast=true
.PHONY: lint-go
lint-go: $(GOLANGCI_LINT) ## Lint codebase
	$(GOLANGCI_LINT) run -v $(GOLANGCI_LINT_FLAGS)

.PHONY: lint-go-full
lint-go-full: GOLANGCI_LINT_FLAGS = --fast=false --max-same-issues=200
lint-go-full: lint-go ## Run slower linters to detect possible issues

.PHONY: fix
fix: GOLANGCI_LINT_FLAGS = --fast=false --fix
fix: lint-go ## Tries to fix errors reported by lint-go-full target

.PHONY: check
check: lint-go-full
check: 	## Run linters


## --------------------------------------
## Tooling Binaries
## --------------------------------------

TOOLING_BINARIES := $(GOLANGCI_LINT)
tools: $(TOOLING_BINARIES) ## Build tooling binaries
.PHONY: $(TOOLING_BINARIES)
$(TOOLING_BINARIES):
	cd $(TOOLS_DIR); make $(@F)


## --------------------------------------
## Build / Install
## --------------------------------------
.PHONY: install
install: ## Install govc and vcsim
	$(MAKE) -C govc install
	$(MAKE) -C vcsim install


## --------------------------------------
## Generate
## --------------------------------------

GO_MOD_FILES := $(filter-out ./hack/tools/go.mod,$(shell find . -name go.mod))
GO_MOD_OP := tidy

.PHONY: $(GO_MOD_FILES)
$(GO_MOD_FILES):
	go -C $(@D) mod $(GO_MOD_OP)

.PHONY: mod
mod: $(GO_MOD_FILES)
mod: ## Validates the modules

.PHONY: modules-download
mod-download: GO_MOD_OP=download
mod-download: $(GO_MOD_FILES)
mod-download: ## Downloads and caches the modules

.PHONY: doc
doc: install
doc: ## Generates govc USAGE.md
	./govc/usage.sh > ./govc/USAGE.md

.PHONY: generate-types
generate-types: ## Generate the types
	$(MAKE) -C ./gen/ $@


## --------------------------------------
## Tests
## --------------------------------------

# Test options
TEST_COUNT ?= 1
TEST_TIMEOUT ?= 5m
TEST_RACE_HISTORY_SIZE ?= 5
GORACE ?= history_size=$(TEST_RACE_HISTORY_SIZE)

ifeq (-count,$(findstring -count,$(TEST_OPTS)))
$(error Use TEST_COUNT to override this option)
endif

ifeq (-race,$(findstring -race,$(TEST_OPTS)))
$(error The -race flag is enabled by default & cannot be specified in TEST_OPTS)
endif

ifeq (-timeout,$(findstring -timeout,$(TEST_OPTS)))
$(error Use TEST_TIMEOUT to override this option)
endif

.PHONY: go-test
go-test: ## Runs go unit tests with race detector enabled
	GORACE=$(GORACE) CGO_ENABLED=1 $(GO) test \
  -count $(TEST_COUNT) \
  -race \
  -timeout $(TEST_TIMEOUT) \
  -v $(TEST_OPTS) \
  ./...

.PHONY: go-fips140-test
go-fips140-test: ## Test simulator can be used with fips140=only
	GODEBUG=fips140=only $(GO) test ./property

go-test: ## Runs go unit tests with race detector enabled
.PHONY: govc-test
govc-test: install
govc-test: ## Runs govc bats tests
	./govc/test/images/update.sh
	(cd govc/test && ./vendor/github.com/bats-core/bats-core/bin/bats -t .)

.PHONY: test
test: go-test go-fips140-test govc-test	## Runs go-test and govc-test
