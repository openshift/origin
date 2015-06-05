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
	hack/test-go.sh $(WHAT) $(TESTS) $(TESTFLAGS)
.PHONY: check

# Build and run unit and integration tests that don't require Docker.
#
# Args:
#   GOFLAGS: Extra flags to pass to 'go' when building.
#   TESTFLAGS: Extra flags that should only be passed to hack/test-go.sh
#
# Example:
#   make check-test
check-test: export KUBE_COVER= -cover -covermode=atomic
check-test: export KUBE_RACE=  -race
check-test: build check
check-test:
	hack/verify-gofmt.sh
	hack/test-cmd.sh
	KUBE_RACE=" " hack/test-integration.sh
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
$(info build is being skipped)
test: check
else
test: build check
endif
test:
	hack/test-cmd.sh
	KUBE_RACE=" " hack/test-integration-docker.sh
	hack/test-end-to-end.sh
ifeq ($(EXTENDED),true)
	hack/test-extended.sh
endif
.PHONY: test

# Run All-in-one OpenShift server.
#
# Example:
#   make run
run: build
	$(OUT_DIR)/local/go/bin/openshift start --cors-allowed-origins=.*
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
#   make clean
release: clean
	hack/build-release.sh
	hack/build-images.sh
.PHONY: release
