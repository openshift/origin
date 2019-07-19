all: build
.PHONY: all

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/library-go/alpha-build-machinery/make/, \
	golang.mk \
	targets/openshift/deps.mk \
	targets/openshift/images.mk \
)

# This will call a macro called "build-image" which will generate image specific targets based on the parameters:
# $0 - macro name
# $1 - target suffix
# $2 - Dockerfile path
# $3 - context directory for image build
# It will generate target "image-$(1)" for builing the image an binding it as a prerequisite to target "images".
$(call build-image,origin-$(GO_PACKAGE),./Dockerfile,.)

clean:
	$(RM) ./template-service-broker
.PHONY: clean

GO_TEST_PACKAGES :=./pkg/... ./cmd/...

update:
	hack/update-generated-conversions.sh
	hack/update-generated-deep-copies.sh
	hack/update-generated-defaulters.sh
.PHONY: update

verify:
	hack/verify-generated-conversions.sh
	hack/verify-generated-deep-copies.sh
	hack/verify-generated-defaulters.sh
.PHONY: verify
