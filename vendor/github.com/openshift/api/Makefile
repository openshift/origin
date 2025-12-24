all: build
.PHONY: all

update: update-non-codegen update-codegen

RUNTIME ?= podman
RUNTIME_IMAGE_NAME ?= registry.ci.openshift.org/openshift/release:rhel-9-release-golang-1.24-openshift-4.20

EXCLUDE_DIRS := _output/ dependencymagnet/ hack/ third_party/ tls/ tools/ vendor/ tests/
GO_PACKAGES :=$(addsuffix ...,$(addprefix ./,$(filter-out $(EXCLUDE_DIRS), $(wildcard */))))

.PHONY: test-unit
test-unit:
	go test -v $(GO_PACKAGES)

##################################################################################
#
# BEGIN: Update codegen-crds. Defaults to generating updates for all API packages.
#        To run a subset of packages:
#        - Set API_GROUP_VERSIONS to a space separated list of fully qualified <group>/<version>.
#          E.g. API_GROUP_VERSIONS="apps.openshift.io/v1 build.openshift.io/v1" make update-codegen-crds.
#        FeatureSet generation is controlled at the group level by the
#        .codegen.yaml file.
#
##################################################################################

# Ensure update-scripts are run before crd-gen so updates to Godoc are included in CRDs.
# Run update-payload-crds after update-codegen-crds to copy any newly created crds
.PHONY: update-codegen-crds
update-codegen-crds: update-scripts
	hack/update-codegen-crds.sh
	hack/update-payload-crds.sh

#####################
#
# END: Update Codegen
#
#####################

# When not otherwise set, diff/lint against the local master branch
PULL_BASE_SHA ?= master

.PHONY: lint
lint:
	hack/golangci-lint.sh run --new-from-rev=${PULL_BASE_SHA} ${EXTRA_ARGS}

.PHONY: lint-fix
lint-fix: EXTRA_ARGS=--fix
lint-fix: lint

# Ignore the exit code of the fix lint, it will always error as there are unfixed issues
# that cannot be fixed from historic commits.
.PHONY: verify-lint-fix
verify-lint-fix:
	make lint-fix 2>/dev/null || true
	git diff --exit-code

# Verify codegen runs all verifiers in the order they are defined in the root.go file.
# This includes all generators defined in update-codegen, but also the crd-schema-checker and crdify verifiers.
.PHONY: verify-codegen
verify-codegen:
	EXTRA_ARGS=--verify hack/update-codegen.sh

.PHONY: verify-non-codegen
verify-non-codegen:
	bash -x hack/verify-protobuf.sh
	hack/verify-crds.sh
	bash -x hack/verify-types.sh
	bash -x hack/verify-integration-tests.sh
	bash -x hack/verify-group-versions.sh
	bash -x hack/verify-prerelease-lifecycle-gen.sh
	hack/verify-payload-crds.sh
	hack/verify-payload-featuregates.sh

.PHONY: verify-scripts
verify-scripts: verify-non-codegen verify-codegen

.PHONY: verify
verify: verify-scripts lint

.PHONY: verify-codegen-crds
verify-codegen-crds:
	bash -x hack/verify-codegen-crds.sh

.PHONY: verify-crd-schema
verify-crd-schema:
	bash -x hack/verify-crd-schema-checker.sh

.PHONY: verify-crdify
verify-crdify:
	bash -x hack/verify-crdify.sh

.PHONY: verify-feature-promotion
verify-feature-promotion:
	hack/verify-promoted-features-pass-tests.sh

.PHONY: verify-%
verify-%:
	make $*
	git diff --exit-code

################################################################################################
#
# BEGIN: Update scripts. Defaults to generating updates for all API packages.
#        Set API_GROUP_VERSIONS to a space separated list of fully qualified <group>/<version> to limit
#        the scope of the updates. Eg API_GROUP_VERSIONS="apps.openshift.io/v1 build.openshift.io/v1" make update-scripts.
#        Note: Protobuf generation is handled separately, see hack/lib/init.sh.
#
################################################################################################

.PHONY: update-scripts
update-scripts: update-compatibility update-openapi update-deepcopy update-protobuf update-swagger-docs tests-vendor update-prerelease-lifecycle-gen update-payload-featuregates

# Update codegen runs all generators in the order they are defined in the root.go file.
# The per group generators are:[compatibility, deepcopy, swagger-docs, empty-partial-schema, schema-patch, crd-manifest-merge]
# The multi group generators are:[openapi]
.PHONY: update-codegen
update-codegen:
	hack/update-codegen.sh

# Update non-codegen runs all generators that are not part of the codegen utility, or
# are part of it, but are not run by default when invoking codegen without a specific generator.
# E.g. the payload feature gates which is not part of the generator style, but is still a subcommand.
.PHONY: update-non-codegen
update-non-codegen: update-protobuf tests-vendor update-prerelease-lifecycle-gen update-payload-crds update-payload-featuregates

.PHONY: update-compatibility
update-compatibility:
	hack/update-compatibility.sh

.PHONY: update-openapi
update-openapi:
	hack/update-openapi.sh

.PHONY: update-deepcopy
update-deepcopy:
	hack/update-deepcopy.sh

.PHONY: update-protobuf
update-protobuf:
	hack/update-protobuf.sh

.PHONY: update-swagger-docs
update-swagger-docs:
	hack/update-swagger-docs.sh

.PHONY: update-prerelease-lifecycle-gen
update-prerelease-lifecycle-gen:
	hack/update-prerelease-lifecycle-gen.sh

.PHONY: update-payload-crds
update-payload-crds:
	hack/update-payload-crds.sh

.PHONY: update-payload-featuregates
update-payload-featuregates:
	hack/update-payload-featuregates.sh

#####################
#
# END: Update scripts
#
#####################

deps:
	go mod tidy
	go mod vendor
	go mod verify

verify-with-container:
	$(RUNTIME) run -ti --rm -v $(PWD):/go/src/github.com/openshift/api:z -w /go/src/github.com/openshift/api $(RUNTIME_IMAGE_NAME) make verify

generate-with-container:
	$(RUNTIME) run -ti --rm -v $(PWD):/go/src/github.com/openshift/api:z -w /go/src/github.com/openshift/api $(RUNTIME_IMAGE_NAME) make update

.PHONY: integration
integration:
	make -C tests integration

tests-vendor:
	make -C tests vendor

##################################
#
# BEGIN: Build binaries and images
#
##################################

.PHONY: build
build: render write-available-featuresets

render:
	go build --mod=vendor -trimpath github.com/openshift/api/payload-command/cmd/render

write-available-featuresets:
	go build --mod=vendor -trimpath github.com/openshift/api/payload-command/cmd/write-available-featuresets

.PHONY: clean
clean:
	rm -f render write-available-featuresets models-schema
	rm -rf tools/_output

VERSION     ?= $(shell git describe --always --abbrev=7)
MUTABLE_TAG ?= latest
IMAGE       ?= registry.ci.openshift.org/openshift/api

ifeq ($(shell command -v podman > /dev/null 2>&1 ; echo $$? ), 0)
	ENGINE=podman
else ifeq ($(shell command -v docker > /dev/null 2>&1 ; echo $$? ), 0)
	ENGINE=docker
endif

USE_DOCKER ?= 0
ifeq ($(USE_DOCKER), 1)
	ENGINE=docker
endif

.PHONY: images
images:
	$(ENGINE) build -f Dockerfile.rhel8 -t "$(IMAGE):$(VERSION)" -t "$(IMAGE):$(MUTABLE_TAG)" ./

################################
#
# END: Build binaries and images
#
################################
