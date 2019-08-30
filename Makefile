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

export GOFLAGS
export TESTFLAGS
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
#   GOGCFLAGS: Additional go compile flags passed to 'go' when building.
#   TESTFLAGS: Extra flags that should only be passed to hack/test-go.sh
#
# Example:
#   make
#   make all
#   make all WHAT=cmd/openshift-tests GOFLAGS=-v
#   make all GOGCFLAGS="-N -l"
all build:
	hack/build-go.sh $(WHAT) $(GOFLAGS)
.PHONY: all build

# Build all binaries.
#
# Example:
#   make build-all
build-all:
	hack/build-go.sh vendor/k8s.io/kubernetes/cmd/hyperkube cmd/openshift-tests
.PHONY: build-all

# Build the test binaries.
#
# Example:
#   make build-tests
build-tests: build-extended-test
.PHONY: build-tests

build-extended-test:
	hack/build-go.sh cmd/openshift-tests
.PHONY: build-extended-test

build-docs:
	hack/generate-docs.sh
.PHONY: build-docs

# Run core verification and all self contained tests.
#
# Example:
#   make check
check: | build verify
	$(MAKE) test-unit -o build -o verify
.PHONY: check


# Verify code conventions are properly setup.
#
# TODO add verifying listers - we can't do it yet because there's an issue with the generated
# expansion file being incorrect.
#
# Example:
#   make verify
verify: build
	# build-tests task has been disabled until we can determine why memory usage is so high
	{ \
	hack/verify-generated-versions.sh ||r=1;\
	hack/verify-gofmt.sh ||r=1;\
	hack/verify-govet.sh ||r=1;\
	hack/verify-imports.sh ||r=1;\
	hack/verify-generated-bindata.sh ||r=1;\
	hack/verify-generated-deep-copies.sh ||r=1;\
	hack/verify-generated-openapi.sh ||r=1;\
	hack/verify-generated-json-codecs.sh ||r=1; \
	hack/verify-generated-swagger-spec.sh ||r=1;\
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

# Update all generated artifacts.
#
# Example:
#   make update
update:
	hack/update-generated-versions.sh
	hack/update-generated-bindata.sh
	hack/update-generated-deep-copies.sh
	hack/update-generated-openapi.sh
	$(MAKE) build
.PHONY: update

# Update all generated artifacts for the API
#
# Example:
#   make update-api
update-api:
	hack/update-generated-deep-copies.sh
	hack/update-generated-openapi.sh
	$(MAKE) build
	hack/update-generated-swagger-spec.sh
.PHONY: update-api

# Update vendored dependencies
#
# Example:
#	make update-deps
update-deps:
	hack/update-deps.sh
.PHONY: update-deps

# Update external examples
#
# Example:
#	make update-examples
update-examples:
	hack/update-external-examples.sh
	hack/update-generated-bindata.sh
.PHONY: update-examples

# Build and run the complete test-suite.
#
# Example:
#   make test
test: test-tools
.PHONY: test

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

# Run tools tests.
#
# Example:
#   make test-tools
test-tools:
	hack/test-tools.sh
.PHONY: test-tools

# Run extended tests.
#
# Args:
#   SUITE: Which Bash entrypoint under test/extended/ to use. Don't include the
#          ending `.sh`. Ex: `core`.
#   FOCUS: Literal string to pass to `--ginkgo.focus=`
# The FOCUS env variable is handled by the respective suite scripts.
#
# Example:
#   make test-extended SUITE=core
#   make test-extended SUITE=conformance FOCUS=pods
#
SUITE ?= conformance
test-extended:
	test/extended/$(SUITE).sh
.PHONY: test-extended

# Remove all build artifacts.
#
# Example:
#   make clean
clean:
	rm -rf $(OUT_DIR)
.PHONY: clean

# Build an official release of OpenShift for all platforms and the images that depend on it.
#
# Example:
#   make release
official-release: build-images build-cross
.PHONY: official-release

# Build a release of OpenShift for linux/amd64 and the images that depend on it.
#
# Example:
#   make release
release: build-images
.PHONY: release

# Build the cross compiled release binaries
#
# Example:
#   make build-cross
build-cross:
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
#
# Example:
#   make build-rpms
build-rpms:
	OS_ONLY_BUILD_PLATFORMS='linux/amd64' hack/build-rpms.sh
.PHONY: build-rpms

# Build RPMs for all architectures
#
# Args:
#
# Example:
#   make build-rpms-redistributable
build-rpms-redistributable:
	hack/build-rpms.sh
.PHONY: build-rpms-redistributable

# Build images from the official RPMs
#
# Args:
#
# Example:
#   make build-images
build-images: build-rpms
	hack/build-images.sh
.PHONY: build-images

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
