# Copyright 2016 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

all: build test verify

# Some env vars that devs might find useful:
#  GOFLAGS      : extra "go build" flags to use - e.g. -v   (for verbose)
#  NO_DOCKER=1  : execute each step natively, not in a Docker container
#  TEST_DIRS=   : only run the unit tests from the specified dirs
#  UNIT_TESTS=  : only run the unit tests matching the specified regexp

# Define some constants
#######################
ROOT           = $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
BINDIR        ?= bin
BUILD_DIR     ?= build
COVERAGE      ?= $(CURDIR)/coverage.html
SC_PKG         = github.com/kubernetes-incubator/service-catalog
TOP_SRC_DIRS   = cmd contrib pkg plugin
SRC_DIRS       = $(shell sh -c "find $(TOP_SRC_DIRS) -name \\*.go \
                   -exec dirname {} \\; | sort | uniq")
TEST_DIRS     ?= $(shell sh -c "find $(TOP_SRC_DIRS) -name \\*_test.go \
                   -exec dirname {} \\; | sort | uniq")
# Either the tag name, e.g. v1.2.3 or the commit hash for untagged commits, e.g. abc123
VERSION       ?= $(shell git describe --always --abbrev=7 --dirty)
# Either the tag name, e.g. v1.2.3 or a combination of the closest tag combined with the commit hash, e.g. v1.2.3-2-gabc123
TAG_VERSION   ?= $(shell git describe --tags --abbrev=7 --dirty)
BUILD_LDFLAGS  = $(shell build/version.sh $(ROOT) $(SC_PKG))
GIT_BRANCH    ?= $(shell git rev-parse --abbrev-ref HEAD)

# Only skip the verification of external href's if we're not on 'master'
SKIP_HTTP=-x
SKIP_COMMENT=" (Skipping external hrefs)"
ifeq ($(GIT_BRANCH),master)
SKIP_HTTP=
SKIP_COMMENT=" (Checking external hrefs)"
endif

# Run stat against /dev/null and check if it has any stdout output.
# If stdout is blank, we are detecting bsd-stat because stat it has
# returned an error to stderr. If not bsd-stat, assume gnu-stat.
ifeq ($(shell stat -c"%U" /dev/null 2> /dev/null),)
STAT           = stat -f '%c %N'
else
STAT           = stat -c '%Y %n'
endif

TYPES_FILES    = $(shell find pkg/apis -name types.go)
GO_VERSION    ?= 1.9

ALL_ARCH=amd64 arm arm64 ppc64le s390x
ALL_CLIENT_PLATFORM=darwin linux windows

PLATFORM ?= linux
# This is the current platform, so that we can build a native client binary by default
CLIENT_PLATFORM?=$(shell uname -s | tr A-Z a-z)
ARCH     ?= amd64

ifeq ($(PLATFORM),windows)
FILE_EXT=.exe
else
FILE_EXT=
endif

# TODO: Consider using busybox instead of debian
BASEIMAGE?=gcr.io/google-containers/debian-base-$(ARCH):0.2

GO_BUILD       = env GOOS=$(PLATFORM) GOARCH=$(ARCH) go build -i $(GOFLAGS) \
                   -ldflags "-X $(SC_PKG)/pkg.VERSION=$(VERSION) $(BUILD_LDFLAGS)"
BASE_PATH      = $(ROOT:/src/github.com/kubernetes-incubator/service-catalog/=)
export GOPATH  = $(BASE_PATH):$(ROOT)/vendor

MUTABLE_TAG                      ?= canary
SERVICE_CATALOG_IMAGE             = $(REGISTRY)service-catalog-$(ARCH):$(VERSION)
SERVICE_CATALOG_MUTABLE_IMAGE     = $(REGISTRY)service-catalog-$(ARCH):$(MUTABLE_TAG)
USER_BROKER_IMAGE                 = $(REGISTRY)user-broker-$(ARCH):$(VERSION)
USER_BROKER_MUTABLE_IMAGE         = $(REGISTRY)user-broker-$(ARCH):$(MUTABLE_TAG)

ifdef UNIT_TESTS
	UNIT_TEST_FLAGS=-run $(UNIT_TESTS) -v
endif

ifdef INT_TESTS
	INT_TEST_FLAGS=--test.run=$(INT_TESTS)
endif

ifdef TEST_LOG_LEVEL
	UNIT_TEST_FLAGS+=-v
	UNIT_TEST_LOG_FLAGS=-args --alsologtostderr --v=$(TEST_LOG_LEVEL)
	INT_TEST_FLAGS+=--alsologtostderr --v=$(TEST_LOG_LEVEL)
endif

ifdef NO_DOCKER
	DOCKER_CMD =
	scBuildImageTarget =
else
	# Mount .pkg as pkg so that we save our cached "go build" output files
	DOCKER_CMD = docker run --security-opt label:disable --rm -v $(PWD):/go/src/$(SC_PKG) \
	  -v $(PWD)/.pkg:/go/pkg --env AZURE_STORAGE_CONNECTION_STRING scbuildimage
	scBuildImageTarget = .scBuildImage
endif

# This section builds the output binaries.
# Some will have dedicated targets to make it easier to type, for example
# "service-catalog" instead of "bin/service-catalog".
#########################################################################
build: .init .generate_files \
	$(BINDIR)/service-catalog \
	$(BINDIR)/user-broker

.PHONY: $(BINDIR)/user-broker
user-broker: $(BINDIR)/user-broker
$(BINDIR)/user-broker: .init contrib/cmd/user-broker \
	  $(shell find contrib/cmd/user-broker -type f) \
	  $(shell find contrib/pkg/broker -type f)
	$(DOCKER_CMD) $(GO_BUILD) -o $@ $(SC_PKG)/contrib/cmd/user-broker

.PHONY: $(BINDIR)/service-catalog
service-catalog: $(BINDIR)/service-catalog
$(BINDIR)/service-catalog: .init .generate_files cmd/service-catalog 
	$(DOCKER_CMD) $(GO_BUILD) -o $@ $(SC_PKG)/cmd/service-catalog

# This section contains the code generation stuff
#################################################
.generate_exes: $(BINDIR)/defaulter-gen \
                $(BINDIR)/deepcopy-gen \
                $(BINDIR)/conversion-gen \
                $(BINDIR)/client-gen \
                $(BINDIR)/lister-gen \
                $(BINDIR)/informer-gen \
                $(BINDIR)/openapi-gen
	touch $@

$(BINDIR)/defaulter-gen: .init
	$(DOCKER_CMD) go build -o $@ $(SC_PKG)/vendor/k8s.io/code-generator/cmd/defaulter-gen

$(BINDIR)/deepcopy-gen: .init
	$(DOCKER_CMD) go build -o $@ $(SC_PKG)/vendor/k8s.io/code-generator/cmd/deepcopy-gen

$(BINDIR)/conversion-gen: .init
	$(DOCKER_CMD) go build -o $@ $(SC_PKG)/vendor/k8s.io/code-generator/cmd/conversion-gen

$(BINDIR)/client-gen: .init
	$(DOCKER_CMD) go build -o $@ $(SC_PKG)/vendor/k8s.io/code-generator/cmd/client-gen

$(BINDIR)/lister-gen: .init
	$(DOCKER_CMD) go build -o $@ $(SC_PKG)/vendor/k8s.io/code-generator/cmd/lister-gen

$(BINDIR)/informer-gen: .init
	$(DOCKER_CMD) go build -o $@ $(SC_PKG)/vendor/k8s.io/code-generator/cmd/informer-gen

$(BINDIR)/openapi-gen: vendor/k8s.io/code-generator/cmd/openapi-gen
	$(DOCKER_CMD) go build -o $@ $(SC_PKG)/$^

.PHONY: $(BINDIR)/e2e.test
$(BINDIR)/e2e.test: .init
	$(DOCKER_CMD) go test -c -o $@ $(SC_PKG)/test/e2e

# Regenerate all files if the gen exes changed or any "types.go" files changed
.generate_files: .init .generate_exes $(TYPES_FILES)
	# generate apiserver deps
	$(DOCKER_CMD) $(BUILD_DIR)/update-apiserver-gen.sh
	# generate all pkg/client contents
	$(DOCKER_CMD) $(BUILD_DIR)/update-client-gen.sh
	touch $@

# Some prereq stuff
###################

.init: $(scBuildImageTarget)
	touch $@

.scBuildImage: build/build-image/Dockerfile
	sed "s/GO_VERSION/$(GO_VERSION)/g" < build/build-image/Dockerfile | \
	  docker build -t scbuildimage -
	touch $@

# Util targets
##############
.PHONY: verify verify-generated verify-client-gen verify-docs
verify: .init .generate_files verify-generated verify-client-gen verify-docs verify-vendor
	@echo Running gofmt:
	@$(DOCKER_CMD) gofmt -l -s $(TOP_TEST_DIRS) $(TOP_SRC_DIRS)>.out 2>&1||true
	@[ ! -s .out ] || \
	  (echo && echo "*** Please 'gofmt' the following:" && \
	  cat .out && echo && rm .out && false)
	@rm .out
	@#
	@echo Running golint and go vet:
	@# Exclude the generated (zz) files for now, as well as defaults.go (it
	@# observes conventions from upstream that will not pass lint checks).
	@$(DOCKER_CMD) sh -c \
	  'for i in $$(find $(TOP_SRC_DIRS) -name *.go \
	    | grep -v ^pkg/kubernetes/ \
	    | grep -v generated \
	    | grep -v ^pkg/client/ \
	    | grep -v v1beta1/defaults.go); \
	  do \
	   golint --set_exit_status $$i || exit 1; \
	  done'
	@#
	$(DOCKER_CMD) go vet $(SC_PKG)/...
	@echo Running repo-infra verify scripts
	@$(DOCKER_CMD) vendor/github.com/kubernetes/repo-infra/verify/verify-boilerplate.sh --rootdir=. | grep -v generated | grep -v .pkg > .out 2>&1 || true
	@[ ! -s .out ] || (cat .out && rm .out && false)
	@rm .out
	@#
	@echo Running errexit checker:
	@$(DOCKER_CMD) build/verify-errexit.sh
	@echo Running tag verification:
	@$(DOCKER_CMD) build/verify-tags.sh

verify-docs: .init
	@echo Running href checker$(SKIP_COMMENT):
	@$(DOCKER_CMD) verify-links.sh -s .pkg -t $(SKIP_HTTP) .

verify-generated: .init .generate_files
	$(DOCKER_CMD) $(BUILD_DIR)/update-apiserver-gen.sh --verify-only

verify-client-gen: .init .generate_files
	$(DOCKER_CMD) $(BUILD_DIR)/verify-client-gen.sh

format: .init
	$(DOCKER_CMD) gofmt -w -s $(TOP_SRC_DIRS)

coverage: .init
	$(DOCKER_CMD) contrib/hack/coverage.sh --html "$(COVERAGE)" \
	  $(addprefix ./,$(TEST_DIRS))

.PHONY: test test-unit test-integration test-e2e
test: .init build test-unit test-integration

# this target checks to see if the go binary is installed on the host
.PHONY: check-go
check-go:
	@if [ -z $$(which go) ]; then \
	  echo "Missing \`go\` binary which is required for development"; \
	  exit 1; \
	fi

# this target uses the host-local go installation to test
.PHONY: test-unit-native
test-unit-native: check-go
	go test $(addprefix ${SC_PKG}/,${TEST_DIRS})

test-unit: .init build
	@echo Running tests:
	$(DOCKER_CMD) go test -race $(UNIT_TEST_FLAGS) \
	  $(addprefix $(SC_PKG)/,$(TEST_DIRS)) $(UNIT_TEST_LOG_FLAGS)

build-integration: .generate_files
	$(DOCKER_CMD) go test -race github.com/kubernetes-incubator/service-catalog/test/integration/... -c

test-integration: .init $(scBuildImageTarget) build build-integration
	# test kubectl
	contrib/hack/setup-kubectl.sh
	contrib/hack/test-apiserver.sh
	# golang integration tests
	$(DOCKER_CMD) test/integration.sh $(INT_TEST_FLAGS)

clean-e2e: .init $(scBuildImageTarget)
	$(DOCKER_CMD) rm -f $(BINDIR)/e2e.test

build-e2e: .generate_files $(BINDIR)/e2e.test

test-e2e: build-e2e
	$(BINDIR)/e2e.test

clean: clean-bin clean-build-image clean-generated clean-coverage

clean-bin: .init $(scBuildImageTarget)
	$(DOCKER_CMD) rm -rf $(BINDIR)
	rm -f .generate_exes

clean-build-image: .init $(scBuildImageTarget)
	$(DOCKER_CMD) rm -rf .pkg
	rm -f .scBuildImage
	docker rmi -f scbuildimage > /dev/null 2>&1 || true

# clean-generated does a `git checkout --` on all generated files and
# directories.  May not work correctly if you have staged some of these files
# or have multiple commits.
clean-generated:
	rm -f .generate_files
	# rollback changes to generated defaults/conversions/deepcopies
	find $(TOP_SRC_DIRS) -name zz_generated* | xargs git checkout --
	# rollback changes to types.generated.go
	find $(TOP_SRC_DIRS) -name types.generated* | xargs git checkout --
	# rollback changes to the generated clientset directories
	find $(TOP_SRC_DIRS) -type d -name *_generated | xargs git checkout --
	# rollback openapi changes
	git checkout -- pkg/openapi/openapi_generated.go

# purge-generated removes generated files from the filesystem.
purge-generated: .init $(scBuildImageTarget)
	find $(TOP_SRC_DIRS) -name zz_generated* -exec $(DOCKER_CMD) rm {} \;
	find $(TOP_SRC_DIRS) -depth -type d -name *_generated \
	  -exec $(DOCKER_CMD) rm -rf {} \;
	$(DOCKER_CMD) rm -f pkg/openapi/openapi_generated.go
	echo 'package v1beta1' > pkg/apis/servicecatalog/v1beta1/types.generated.go
	rm -f .generate_files

clean-coverage:
	rm -f $(COVERAGE)

# Building Docker Images for our executables
############################################
images: user-broker-image service-catalog-image

images-all: $(addprefix arch-image-,$(ALL_ARCH))
arch-image-%:
	$(MAKE) ARCH=$* build
	$(MAKE) ARCH=$* images

define build-and-tag # (service, image, mutable_image, prefix)
	$(eval build_path := "$(4)build/$(1)")
	$(eval tmp_build_path := "$(build_path)/tmp")
	mkdir -p $(tmp_build_path)
	cp $(BINDIR)/$(1) $(tmp_build_path)
	cp $(build_path)/Dockerfile $(tmp_build_path)
	# -i.bak is required for cross-platform compat: https://stackoverflow.com/questions/5694228/sed-in-place-flag-that-works-both-on-mac-bsd-and-linux
	sed -i.bak "s|BASEIMAGE|$(BASEIMAGE)|g" $(tmp_build_path)/Dockerfile
	rm $(tmp_build_path)/Dockerfile.bak
	docker run --rm --privileged multiarch/qemu-user-static:register --reset
	docker build -t $(2) $(tmp_build_path)
	docker tag $(2) $(3)
	rm -rf $(tmp_build_path)
endef

user-broker-image: contrib/build/user-broker/Dockerfile $(BINDIR)/user-broker
	$(call build-and-tag,"user-broker",$(USER_BROKER_IMAGE),$(USER_BROKER_MUTABLE_IMAGE),"contrib/")
ifeq ($(ARCH),amd64)
	docker tag $(USER_BROKER_IMAGE) $(REGISTRY)user-broker:$(VERSION)
	docker tag $(USER_BROKER_MUTABLE_IMAGE) $(REGISTRY)user-broker:$(MUTABLE_TAG)
endif

service-catalog-image: build/service-catalog/Dockerfile $(BINDIR)/service-catalog
	$(call build-and-tag,"service-catalog",$(SERVICE_CATALOG_IMAGE),$(SERVICE_CATALOG_MUTABLE_IMAGE))
ifeq ($(ARCH),amd64)
	docker tag $(SERVICE_CATALOG_IMAGE) $(REGISTRY)service-catalog:$(VERSION)
	docker tag $(SERVICE_CATALOG_MUTABLE_IMAGE) $(REGISTRY)service-catalog:$(MUTABLE_TAG)
endif


# Push our Docker Images to a registry
######################################
push: user-broker-push service-catalog-push

user-broker-push: user-broker-image
	docker push $(USER_BROKER_IMAGE)
	docker push $(USER_BROKER_MUTABLE_IMAGE)
ifeq ($(ARCH),amd64)
	docker push $(REGISTRY)user-broker:$(VERSION)
	docker push $(REGISTRY)user-broker:$(MUTABLE_TAG)
endif

service-catalog-push: service-catalog-image
	docker push $(SERVICE_CATALOG_IMAGE)
	docker push $(SERVICE_CATALOG_MUTABLE_IMAGE)
ifeq ($(ARCH),amd64)
	docker push $(REGISTRY)service-catalog:$(VERSION)
	docker push $(REGISTRY)service-catalog:$(MUTABLE_TAG)
endif


release-push: $(addprefix release-push-,$(ALL_ARCH))
release-push-%:
	$(MAKE) ARCH=$* build
	$(MAKE) ARCH=$* push

# svcat kubectl plugin
############################
.PHONY: $(BINDIR)/svcat/$(TAG_VERSION)/$(PLATFORM)/$(ARCH)/svcat$(FILE_EXT)
svcat:
	# Compile a native binary for local dev/test
	$(MAKE) svcat-for-$(CLIENT_PLATFORM)
	cp $(BINDIR)/svcat/$(TAG_VERSION)/$(CLIENT_PLATFORM)/$(ARCH)/svcat$(FILE_EXT) $(BINDIR)/svcat/

svcat-all: $(addprefix svcat-for-,$(ALL_CLIENT_PLATFORM))

svcat-for-%:
	$(MAKE) PLATFORM=$* VERSION=$(TAG_VERSION) svcat-xbuild

svcat-xbuild: $(BINDIR)/svcat/$(TAG_VERSION)/$(PLATFORM)/$(ARCH)/svcat$(FILE_EXT)
$(BINDIR)/svcat/$(TAG_VERSION)/$(PLATFORM)/$(ARCH)/svcat$(FILE_EXT): .init .generate_files
	$(DOCKER_CMD) $(GO_BUILD) -o $@ $(SC_PKG)/cmd/svcat

svcat-publish: clean-bin svcat-all
	# Download the latest client with https://download.svcat.sh/cli/latest/darwin/amd64/svcat
	# Download an older client with  https://download.svcat.sh/cli/VERSION/darwin/amd64/svcat
	$(DOCKER_CMD) cp -R $(BINDIR)/svcat/$(TAG_VERSION) $(BINDIR)/svcat/$(MUTABLE_TAG)
	# AZURE_STORAGE_CONNECTION_STRING will be used for auth in the following command
	$(DOCKER_CMD) az storage blob upload-batch -d cli -s $(BINDIR)/svcat

# Dependency management via dep (https://golang.github.io/dep)
.PHONY: verify-vendor test-dep
verify-vendor: .init
	# Verify that vendor/ is in sync with Gopkg.lock
	$(DOCKER_CMD) $(BUILD_DIR)/verify-vendor.sh

test-dep: .init
	# Test that a downstream consumer of our client library can use dep
	$(DOCKER_CMD) test/test-dep.sh
