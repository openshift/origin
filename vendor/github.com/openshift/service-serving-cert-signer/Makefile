# Old-skool build tools.
#
# Targets (see each target for more information):
#   all: Build code.
#   build: Build code.
#   check: Run verify, build, unit tests and cmd tests.
#   test: Run all tests.
#   run: Run all-in-one server
#   clean: Clean up.

OUT_DIR = _output
OS_OUTPUT_GOPATH ?= 1

export GOFLAGS
export TESTFLAGS
# If set to 1, create an isolated GOPATH inside _output using symlinks to avoid
# other packages being accidentally included. Defaults to on.
export OS_OUTPUT_GOPATH
# May be used to set additional arguments passed to the image build commands for
# mounting secrets specific to a build environment.
export OS_BUILD_IMAGE_ARGS

# Tests run using `make` are most often run by the CI system, so we are OK to
# assume the user wants jUnit output and will turn it off if they don't.
JUNIT_REPORT ?= true

# Build code.
#
# Args:
#   WHAT: Directory names to build.  If any of these directories has a 'main'
#     package, the build will produce executable files under $(OUT_DIR)/local/bin.
#     If not specified, "everything" will be built.
#   GOFLAGS: Extra flags to pass to 'go' when building.
#   TESTFLAGS: Extra flags that should only be passed to hack/test-go.sh
#
# Example:
#   make
#   make all
#   make all WHAT=cmd/oc GOFLAGS=-v
all build:
	hack/build-go.sh $(WHAT) $(GOFLAGS)
.PHONY: all build

# Run core verification and all self contained tests.
#
# Example:
#   make check
check: | verify test-unit
.PHONY: check


# Verify code conventions are properly setup.
#
# Example:
#   make verify
verify:
	{ \
	hack/verify-gofmt.sh ||r=1;\
	hack/verify-govet.sh ||r=1;\
	hack/verify-imports.sh ||r=1;\
	hack/verify-generated-bindata.sh ||r=1;\
	exit $$r ;\
	}
.PHONY: verify


# Verify commit comments.
#
# Example:
#   make verify-commits
verify-commits:
	hack/verify-upstream-commits.sh
.PHONY: verify-commits

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
#   make test-unit WHAT=pkg/build TESTFLAGS=-v
test-unit:
	GOTEST_FLAGS="$(TESTFLAGS)" hack/test-go.sh $(WHAT) $(TESTS)
.PHONY: test-unit

# Run e2e tests.
# 
# Args:
#
# Example:
#   make test-e2e
test-e2e:
	test/e2e.sh
.PHONY: test-e2e

# Remove all build artifacts.
#
# Example:
#   make clean
clean:
	rm -rf $(OUT_DIR)
.PHONY: clean

# Build the cross compiled release binaries
#
# Example:
#   make build-cross
build-cross:
	hack/build-cross.sh
.PHONY: build-cross

# Build RPMs only for the Linux AMD64 target
#
# Args:
#
# Example:
#   make build-rpms
build-rpms:
	OS_ONLY_BUILD_PLATFORMS='linux/amd64' hack/build-rpms.sh
.PHONY: build-rpms

# Build images from the official RPMs
# 
# Args:
#
# Example:
#   make build-images
build-images:
	hack/build-images.sh
.PHONY: build-images
