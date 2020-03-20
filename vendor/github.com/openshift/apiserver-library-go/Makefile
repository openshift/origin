all: build
.PHONY: all

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
	golang.mk \
	targets/openshift/deps.mk \
)

# All the go packages (e.g. for verfy)
GO_PACKAGES :=./...
# Packages to be compiled
GO_BUILD_PACKAGES :=$(GO_PACKAGES)
# Do not auto-expand packages for libraries or it would compile them separately
GO_BUILD_PACKAGES_EXPANDED :=$(GO_BUILD_PACKAGES)

update: update-generated-deep-copies
.PHONY: update

verify: verify-generated-deep-copies
.PHONY: verify

update-generated-deep-copies:
	hack/update-generated-deep-copies.sh
.PHONY: update-generated-deep-copies

verify-generated-deep-copies:
	hack/verify-generated-deep-copies.sh
.PHONY: verify-generated-deep-copies
