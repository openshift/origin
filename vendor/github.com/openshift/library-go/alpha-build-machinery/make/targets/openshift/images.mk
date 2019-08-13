IMAGE_REGISTRY ?=
IMAGE_ORG ?=openshift
IMAGE_TAG ?=latest


# IMAGE_BUILD_EXTRA_FLAGS lets you add extra flags for imagebuilder
# e.g. to mount secrets and repo information into base image like:
# make images IMAGE_BUILD_EXTRA_FLAGS='-mount ~/projects/origin-repos/4.2/:/etc/yum.repos.d/'
IMAGE_BUILD_EXTRA_FLAGS ?=

# $1 - image name
# $2 - Dockerfile path
# $3 - context
define build-image-internal
image-$(1):
	$(strip imagebuilder --allow-pull $(IMAGE_BUILD_EXTRA_FLAGS) -f $(2) -t $(addsuffix /,$(IMAGE_REGISTRY))$(addsuffix /,$(IMAGE_ORG))$(1)$(addprefix :,$(IMAGE_TAG)) $(3))
.PHONY: image-$(1)

images: image-$(1)
.PHONY: images
endef

define build-image
$(eval $(call build-image-internal,$(1),$(2),$(3)))
endef
