.PHONY: all tools test validate lint .gitvalidation fmt

# Which github repository and branch to use for testing with skopeo
SKOPEO_REPO = projectatomic/skopeo
SKOPEO_BRANCH = master
# Set SUDO=sudo to run container integration tests using sudo.
SUDO =

# when cross compiling _for_ a Darwin or windows host, then we must use openpgp
BUILD_TAGS_WINDOWS_CROSS = containers_image_ostree_stub containers_image_openpgp
BUILD_TAGS_DARWIN_CROSS = containers_image_ostree_stub containers_image_openpgp
# when compiling _on_ a Darwin host, then we can link against gpgme
BUILD_TAGS_DARWIN_NATIVE = containers_image_ostree_stub

ifeq ($(shell uname),Darwin)
PLATFORM_BUILD_TAG = $(BUILD_TAGS_DARWIN_NATIVE)
endif


BUILDTAGS = btrfs_noversion libdm_no_deferred_remove $(PLATFORM_BUILD_TAG)
BUILDFLAGS := -tags "$(BUILDTAGS)"

PACKAGES := $(shell go list $(BUILDFLAGS) ./... | grep -v github.com/containers/image/vendor)
SOURCE_DIRS = $(shell echo $(PACKAGES) | awk 'BEGIN{FS="/"; RS=" "}{print $$4}' | uniq)

# On macOS, (brew install gpgme) installs it within /usr/local, but /usr/local/include is not in the default search path.
# Rather than hard-code this directory, use gpgme-config. Sadly that must be done at the top-level user
# instead of locally in the gpgme subpackage, because cgo supports only pkg-config, not general shell scripts,
# and gpgme does not install a pkg-config file.
# If gpgme is not installed or gpgme-config can’t be found for other reasons, the error is silently ignored
# (and the user will probably find out because the cgo compilation will fail).
GPGME_ENV = CGO_CFLAGS="$(shell gpgme-config --cflags 2>/dev/null)" CGO_LDFLAGS="$(shell gpgme-config --libs 2>/dev/null)"

all: tools test validate .gitvalidation

build: vendor build-internal

build-internal:
	$(GPGME_ENV) go build $(BUILDFLAGS) $(PACKAGES)

cross: vendor
	GOOS=windows $(MAKE) build-internal BUILDTAGS="$(BUILDTAGS) $(BUILD_TAGS_WINDOWS_CROSS)"
	GOOS=darwin $(MAKE) build-internal BUILDTAGS="$(BUILDTAGS) $(BUILD_TAGS_DARWIN_CROSS)"

tools: tools.timestamp

tools.timestamp: Makefile
	@go get -u $(BUILDFLAGS) github.com/golang/lint/golint
	@go get $(BUILDFLAGS) github.com/vbatts/git-validation
	@go get -u github.com/rancher/trash
	@touch tools.timestamp

vendor: tools.timestamp vendor.conf
	@trash
	@touch vendor

clean:
	rm -rf vendor tools.timestamp

test: vendor
	@$(GPGME_ENV) go test $(BUILDFLAGS) -cover $(PACKAGES)

# This is not run as part of (make all), but Travis CI does run this.
# Demonstrating a working version of skopeo (possibly with modified SKOPEO_REPO/SKOPEO_BRANCH, e.g.
#    make test-skopeo SKOPEO_REPO=runcom/skopeo-1 SKOPEO_BRANCH=oci-3 SUDO=sudo
# ) is a requirement before merging; note that Travis will only test
# the master branch of the upstream repo.
test-skopeo:
	@echo === Testing skopeo build
	@export GOPATH=$$(mktemp -d) && \
		skopeo_path=$${GOPATH}/src/github.com/projectatomic/skopeo && \
		vendor_path=$${skopeo_path}/vendor/github.com/containers/image && \
		git clone -b $(SKOPEO_BRANCH) https://github.com/$(SKOPEO_REPO) $${skopeo_path} && \
		rm -rf $${vendor_path} && cp -r . $${vendor_path} && rm -rf $${vendor_path}/vendor && \
		cd $${skopeo_path} && \
		make BUILDTAGS="$(BUILDTAGS)" binary-local test-all-local && \
		$(SUDO) make BUILDTAGS="$(BUILDTAGS)" check && \
		rm -rf $${skopeo_path}

fmt:
	@gofmt -l -s -w $(SOURCE_DIRS)

validate: lint
	@go vet $(PACKAGES)
	@test -z "$$(gofmt -s -l . | grep -ve '^vendor' | tee /dev/stderr)"

lint:
	@out="$$(golint $(PACKAGES))"; \
	if [ -n "$$out" ]; then \
		echo "$$out"; \
		exit 1; \
	fi

EPOCH_TEST_COMMIT ?= e68e0e1110e64f906f9b482e548f17d73e02e6b1

# When this is running in travis, it will only check the travis commit range
.gitvalidation:
	@which git-validation > /dev/null 2>/dev/null || (echo "ERROR: git-validation not found. Consider 'make clean && make tools'" && false)
ifeq ($(TRAVIS),true)
	git-validation -q -run DCO,short-subject,dangling-whitespace
else
	git-validation -q -run DCO,short-subject,dangling-whitespace -range $(EPOCH_TEST_COMMIT)..HEAD
endif
