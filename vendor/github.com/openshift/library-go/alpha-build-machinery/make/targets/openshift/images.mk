IMAGE_REGISTRY ?=
IMAGE_ORG ?=openshift
IMAGE_TAG ?=latest

# $1 - image name
# $2 - Dockerfile path
# $3 - context
define build-image-internal
image-$(1):
	imagebuilder -f $(2) -t $(addsuffix /,$(IMAGE_REGISTRY))$(addsuffix /,$(IMAGE_ORG))$(1)$(addprefix :,$(IMAGE_TAG)) $(3)
.PHONY: image-$(1)

images: image-$(1)
.PHONY: images
endef

define build-image
$(eval $(call build-image-internal,$(1),$(2),$(3)))
endef
