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
OS_OUTPUT_GOPATH ?= 1

export GOFLAGS
export TESTFLAGS
# If set to 1, create an isolated GOPATH inside _output using symlinks to avoid
# other packages being accidentally included. Defaults to on.
export OS_OUTPUT_GOPATH
# May be used to set additional arguments passed to the image build commands for
# mounting secrets specific to a build environment.
export OS_BUILD_IMAGE_ARGS

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

# Build the test binaries.
#
# Example:
#   make build-tests
build-tests:
	hack/build-go.sh test/extended/extended.test
	hack/build-go.sh test/integration/integration.test -tags='integration docker'
.PHONY: build-tests

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
	# build-tests is disabled until we can determine why memory usage is so high
	hack/verify-upstream-commits.sh
	hack/verify-gofmt.sh
	hack/verify-govet.sh
	hack/verify-generated-bootstrap-bindata.sh
	hack/verify-generated-deep-copies.sh
	hack/verify-generated-conversions.sh
	hack/verify-generated-clientsets.sh
	hack/verify-generated-completions.sh
	hack/verify-generated-docs.sh
	hack/verify-cli-conventions.sh
	PROTO_OPTIONAL=1 hack/verify-generated-protobuf.sh
	hack/verify-generated-swagger-descriptions.sh
	hack/verify-generated-swagger-spec.sh
.PHONY: verify

# Update all generated artifacts.
#
# Example:
#   make update
update: build
	hack/update-generated-bootstrap-bindata.sh
	hack/update-generated-deep-copies.sh
	hack/update-generated-conversions.sh
	hack/update-generated-clientsets.sh
	hack/update-generated-completions.sh
	hack/update-generated-docs.sh
	PROTO_OPTIONAL=1 hack/update-generated-protobuf.sh
	hack/update-generated-swagger-descriptions.sh
	hack/update-generated-swagger-spec.sh
.PHONY: update

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
#   make test-end-to-end
test-end-to-end: build
	hack/env hack/verify-generated-protobuf.sh # Test the protobuf serializations when we know Docker is available
	hack/test-end-to-end.sh
.PHONY: test-end-to-end

# Run tools tests.
#
# Example:
#   make test-tools
test-tools:
	hack/test-tools.sh
.PHONY: test-tools

# Run assets tests.
#
# Example:
#   make test-assets
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
	rm -rf $(OUT_DIR)
.PHONY: clean

# Build a release of OpenShift for linux/amd64 and the images that depend on it.
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

# Release the integrated components for OpenShift, origin, logging, and metrics.
# The current tag in the Origin release (the tag that points to HEAD) is used to
# clone and build each component. Components must have a hack/release.sh script
# which must accept env var OS_TAG as the tag to build. Each component should push
# its own images. See hack/release.sh and hack/push-release.sh for an example of
# the appropriate behavior.
#
# Prerequisites:
# * you must be logged into the remote registry with the appropriate
#   credentials to push.
# * all repositories must have a Git tag equal to the current repositories tag of
#   HEAD
#
# TODO: consider making hack/release.sh be a make target (make official-release).
#
# Example:
#   make release-components
release-components: clean
	hack/release-components.sh
.PHONY: release-components

# Build the cross compiled release binaries
#
# Example:
#   make build-cross
build-cross: clean
	hack/build-cross.sh
.PHONY: build-cross

# Install travis dependencies
#
# Example:
#   make install-travis
install-travis:
	hack/install-tools.sh
.PHONY: install-travis

# Build RPMs only for the Linux AMD64 target
#
# Args:
#   BUILD_TESTS: whether or not to build a test RPM, off by default
#
# Example:
#   make build-rpms
build-rpms:
	BUILD_TESTS=$(BUILD_TESTS) OS_ONLY_BUILD_PLATFORMS='linux/amd64' hack/build-rpm-release.sh
.PHONY: build-rpms

# Build RPMs for all architectures
#
# Args:
#   BUILD_TESTS: whether or not to build a test RPM, off by default
#
# Example:
#   make build-rpms-redistributable
build-rpms-redistributable:
	BUILD_TESTS=$(BUILD_TESTS) hack/build-rpm-release.sh
.PHONY: build-rpms-redistributable

# Build a release of OpenShift using tito for linux/amd64 and the images that depend on it.
#
# Args:
#   BUILD_TESTS: whether or not to build a test RPM, off by default
#
# Example:
#   make release-rpms BUILD_TESTS=1
release-rpms: clean build-rpms
	hack/build-images.sh
	hack/extract-release.sh
.PHONY: release

# Vendor the Origin Web Console
#
# Args:
#   GIT_REF:           specifies which branch / tag of the web console to vendor. If set, then any untracked/uncommitted changes
#                      will cause the script to exit with an error. If not set then the current working state of the web console
#                      directory will be used.
#   CONSOLE_REPO_PATH: specifies a directory path to look for the web console repo.  If not set it is assumed to be
#                      a sibling to this repository.
# Example:
#   make vendor-console
vendor-console:
	GIT_REF=$(GIT_REF) CONSOLE_REPO_PATH=$(CONSOLE_REPO_PATH) hack/vendor-console.sh
.PHONY: vendor-console