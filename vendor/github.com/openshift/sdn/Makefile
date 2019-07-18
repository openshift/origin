all: build
.PHONY: all

GO_BUILD_PACKAGES = \
	./cmd/... \
	./vendor/github.com/containernetworking/plugins/plugins/ipam/host-local \
	./vendor/github.com/containernetworking/plugins/plugins/main/loopback

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
$(call build-image,origin-node,./images/sdn/Dockerfile,.)
$(call build-image,origin-sdn-controller,./images/sdn-controller/Dockerfile,.)
