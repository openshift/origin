# Old-skool build tools.
#
# Targets (see each target for more information):
#   all: Build code.
#   build: Build code.
#   check: Run unit tests.
#   test: Run all tests.
#   run: Run all-in-one server
#   clean: Clean up.

OUT_DIR = _output
OUT_PKG_DIR = Godeps/_workspace/pkg
OS_OUTPUT_GOPATH ?= 1

export GOFLAGS
export TESTFLAGS
export OS_OUTPUT_GOPATH

# Build code.
#
# Args:
#   WHAT: Directory names to build.  If any of these directories has a 'main'
#     package, the build will produce executable files under $(OUT_DIR)/go/bin.
#     If not specified, "everything" will be built.
#   GOFLAGS: Extra flags to pass to 'go' when building.
#   TESTFLAGS: Extra flags that should only be passed to hack/test-go.sh
#
# Example:
#   make
#   make all
#   make all WHAT=cmd/kubelet GOFLAGS=-v
all build:
	hack/build-go.sh $(WHAT)
.PHONY: all build

# Run core verification and all self contained tests.
#
# Example:
#   make check
check: | build verify
	$(MAKE) test-unit test-cmd -o build -o verify
.PHONY: check


# Verify code conventions are properly setup.
#
# Example:
#   make verify
verify: build
	hack/build-go.sh test/extended/extended.test test/extended/networking/extended.test test/integration/integration.test -tags='integration docker'
	hack/verify-upstream-commits.sh
	hack/verify-gofmt.sh
	hack/verify-govet.sh
	hack/verify-generated-deep-copies.sh
	hack/verify-generated-conversions.sh
	hack/verify-generated-completions.sh
	hack/verify-generated-docs.sh
	hack/verify-generated-swagger-spec.sh
	hack/verify-bootstrap-bindata.sh
	hack/verify-generated-swagger-descriptions.sh
.PHONY: verify

# Update all generated artifacts.
#
# Example:
#   make update
update: build
	hack/update-generated-completions.sh
	hack/update-generated-conversions.sh
	hack/update-generated-deep-copies.sh
	hack/update-generated-docs.sh
	hack/update-generated-swagger-descriptions.sh
	hack/update-generated-swagger-spec.sh
.PHONE: update

# Run unit tests.
#
# Args:
#   WHAT: Directory names to test.  All *_test.go files under these
#     directories will be run.  If not specified, "everything" will be tested.
#   TESTS: Same as WHAT.
#   GOFLAGS: Extra flags to pass to 'go' when building.
#   TESTFLAGS: Extra flags that should only be passed to hack/test-go.sh
#
# Example:
#   make test-unit
#   make test-unit WHAT=pkg/build GOFLAGS=-v
test-unit:
	TEST_KUBE=true GOTEST_FLAGS="$(TESTFLAGS)" hack/test-go.sh $(WHAT) $(TESTS)
.PHONY: test-unit

# Run integration tests. Compiles its own tests, cannot be run
# in parallel with any other go compilation.
#
# Example:
#   make test-integration
test-integration:
	KUBE_COVER=" " KUBE_RACE=" " hack/test-integration.sh
.PHONY: test-integration

# Run command tests. Uses whatever binaries are currently built.
#
# Example:
#   make test-cmd
test-cmd: build
	hack/test-cmd.sh
.PHONY: test-cmd

# Run end to end tests. Uses whatever binaries are currently built.
#
# Example:
#   make test-cmd
test-end-to-end: build
	hack/test-end-to-end.sh
.PHONY: test-end-to-end

# Run tools tests.
#
# Example:
#   make test-cmd
test-tools:
	hack/test-tools.sh
.PHONY: test-tools

test-assets:
ifeq ($(TEST_ASSETS),true)
	hack/test-assets.sh
endif
.PHONY: test-assets

# Build and run the complete test-suite.
#
# Example:
#   make test
test: check
	$(MAKE) test-tools test-integration test-assets -o build
	$(MAKE) test-end-to-end -o build
.PHONY: test

# Run All-in-one OpenShift server.
#
# Example:
#   make run
run: export OS_OUTPUT_BINPATH=$(shell bash -c 'source hack/common.sh; echo $${OS_OUTPUT_BINPATH}')
run: export PLATFORM=$(shell bash -c 'source hack/common.sh; os::build::host_platform')
run: build
	$(OS_OUTPUT_BINPATH)/$(PLATFORM)/openshift start
.PHONY: run

# Remove all build artifacts.
#
# Example:
#   make clean
clean:
	rm -rf $(OUT_DIR) $(OUT_PKG_DIR)
.PHONY: clean

# Build an official release of OpenShift, including the official images.
#
# Example:
#   make release
release: clean
	OS_ONLY_BUILD_PLATFORMS="linux/amd64" hack/build-release.sh
	hack/build-images.sh
	hack/extract-release.sh
.PHONY: release

# Build only the release binaries for OpenShift
#
# Example:
#   make release-binaries
release-binaries: clean
	hack/build-release.sh
	hack/extract-release.sh
.PHONY: release-binaries

# Build the cross compiled release binaries
#
# Example:
#   make build-cross
build-cross: clean
	hack/build-cross.sh
.PHONY: build-cross

# Install travis dependencies
#
install-travis:
	hack/install-tools.sh
.PHONY: install-travis

