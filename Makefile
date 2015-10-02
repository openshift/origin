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

export GOFLAGS
export TESTFLAGS

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

# Build and run unit tests
#
# Args:
#   WHAT: Directory names to test.  All *_test.go files under these
#     directories will be run.  If not specified, "everything" will be tested.
#   TESTS: Same as WHAT.
#   GOFLAGS: Extra flags to pass to 'go' when building.
#   TESTFLAGS: Extra flags that should only be passed to hack/test-go.sh
#
# Example:
#   make check
#   make check WHAT=pkg/build GOFLAGS=-v
check:
	TEST_KUBE=1 hack/test-go.sh $(WHAT) $(TESTS) $(TESTFLAGS)
.PHONY: check

# Verify code is properly organized.
#
# Example:
#   make verify
ifeq ($(SKIP_BUILD), true)
verify:
else
verify: build
endif
	hack/verify-gofmt.sh
	#hack/verify-govet.sh disable until we can verify that the output is sane
	hack/verify-generated-deep-copies.sh
	hack/verify-generated-conversions.sh
	hack/verify-generated-completions.sh
	hack/verify-generated-docs.sh
	hack/verify-generated-swagger-spec.sh
	hack/verify-api-descriptions.sh
.PHONY: verify

# check and verify can't run concurently because of strange concurrent build issues.
check-verify:
	# delegate to another make process that runs serially against the check and verify targets
	$(MAKE) -j1 check verify
.PHONY: check-verify

# Install travis dependencies
#
# Args:
#   TEST_ASSETS: Instead of running tests, test assets only.
ifeq ($(TEST_ASSETS), true)
install-travis:
	hack/install-assets.sh
else
install-travis:
	hack/install-etcd.sh
	hack/install-tools.sh
endif
.PHONY: install-travis

# Run unit and integration tests that don't require Docker.
#
# Args:
#   GOFLAGS: Extra flags to pass to 'go' when building.
#   TESTFLAGS: Extra flags that should only be passed to hack/test-go.sh
#   TEST_ASSETS: Instead of running tests, test assets only.
#
# Example:
#   make check-test
check-test: export KUBE_COVER= -cover -covermode=atomic
check-test: export KUBE_RACE=  -race
ifeq ($(TEST_ASSETS), true)
check-test:
	hack/test-assets.sh
else
check-test: check-verify
	hack/test-cmd.sh
	KUBE_RACE=" " hack/test-integration.sh
endif
.PHONY: check-test

# Build and run the complete test-suite.
#
# Args:
#   GOFLAGS: Extra flags to pass to 'go' when building.
#   TESTFLAGS: Extra flags that should only be passed to hack/test-go.sh
#
# Example:
#   make test
#   make test GOFLAGS=-v
test: export KUBE_COVER= -cover -covermode=atomic
test: export KUBE_RACE=  -race
ifeq ($(SKIP_BUILD), true)
test: check-verify test-int-plus
else
test: build check-verify test-int-plus
endif
.PHONY: test

# Split out of `test`.  This allows `make -j --output-sync=recurse test` to parallelize as expected
test-int-plus: export KUBE_COVER= -cover -covermode=atomic
test-int-plus: export KUBE_RACE=  -race
ifeq ($(SKIP_BUILD), true)
test-int-plus: 
else
test-int-plus: build
endif
test-int-plus:
	hack/test-cmd.sh
	KUBE_RACE=" " hack/test-integration-docker.sh
	hack/test-end-to-end-docker.sh
ifeq ($(EXTENDED),true)
	hack/test-extended.sh
endif
.PHONY: test-int-plus


# Run All-in-one OpenShift server.
#
# Example:
#   make run
OS_OUTPUT_BINPATH=$(shell bash -c 'source hack/common.sh; echo $${OS_OUTPUT_BINPATH}')
PLATFORM=$(shell bash -c 'source hack/common.sh; os::build::host_platform')
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
	hack/build-release.sh
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
