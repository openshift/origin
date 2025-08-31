all: build
.PHONY: all

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
	golang.mk \
	targets/openshift/bindata.mk \
	targets/openshift/images.mk \
	targets/openshift/deps.mk \
)

IMAGE_REGISTRY :=registry.ci.openshift.org

# This will call a macro called "build-image" which will generate image specific targets based on the parameters:
# $0 - macro name
# $1 - target name
# $2 - image ref
# $3 - Dockerfile path
# $4 - context directory for image build
$(call build-image,openshift-tests,$(IMAGE_REGISTRY)/ocp/4.6:tests,./images/tests/Dockerfile.rhel,.)

# Old-skool build tools.
#
# Targets (see each target for more information):
#   all: Build code.
#   build: Build code.
#   check: Run verify, build, unit tests and cmd tests.
#   test: Run all tests.
#   run: Run all-in-one server
#   clean: Clean up.

# Tests run using `make` are most often run by the CI system, so we are OK to
# assume the user wants jUnit output and will turn it off if they don't.
JUNIT_REPORT ?= true

build-docs:
	hack/generate-docs.sh
.PHONY: build-docs

openshift-tests: GO_BUILD_PACKAGES :=./cmd/openshift-tests
openshift-tests: build
.PHONY: openshift-tests

# run repo-specific checks.
#
# Example:
#   make verify-origin
verify-origin:
	hack/verify-jsonformat.sh
	hack/verify-generated.sh
	hack/verify-tls-ownership.sh
.PHONY: verify-origin
verify: verify-origin

# Update all generated artifacts.
#
# Example:
#   make update
update: update-tls-ownership update-bindata
	hack/update-generated.sh
.PHONY: update

# Update TLS artifacts
#
# Example:
#    make update-tls-ownership
update-tls-ownership:
	hack/update-tls-ownership.sh
.PHONY: update-tls-ownership

# Update external examples
#
# Example:
#	make update-examples
update-examples:
	hack/update-external-examples.sh
	$(MAKE) update-bindata
.PHONY: update-examples

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
test: test-tools
	test/extended/$(SUITE).sh
.PHONY: test

# This will call a macro called "add-bindata" which will generate bindata specific targets based on the parameters:
# $0 - macro name
# $1 - target suffix
# $2 - input dirs
# $3 - prefix
# $4 - pkg
# $5 - output
# It will generate targets {update,verify}-bindata-$(1) logically grouping them in unsuffixed versions of these targets
# and also hooked into {update,verify}-generated for broader integration.
$(call add-bindata,bindata,-ignore ".*\.(go|md)$$$$" examples/db-templates examples/image-streams examples/sample-app examples/quickstarts/... examples/hello-openshift examples/jenkins/... examples/quickstarts/cakephp-mysql.json test/extended/testdata/... e2echart,testextended,testdata,test/extended/testdata/bindata.go)

# Build the node openshift-tests-extension binary
node-tests-ext-build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	go build -o node-tests-ext \
	-ldflags "-X 'main.CommitFromGit=$(shell git rev-parse --short HEAD)' \
	-X 'main.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)' \
	-X 'main.GitTreeState=$(shell if git diff-index --quiet HEAD --; then echo clean; else echo dirty; fi)'" \
	./cmd/node-tests-ext
.PHONY: node-tests-ext-build
