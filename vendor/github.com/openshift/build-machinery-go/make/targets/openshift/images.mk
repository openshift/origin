include $(addprefix $(dir $(lastword $(MAKEFILE_LIST))), \
	imagebuilder.mk \
)

# IMAGE_BUILD_EXTRA_FLAGS lets you add extra flags for imagebuilder
# e.g. to mount secrets and repo information into base image like:
# make images IMAGE_BUILD_EXTRA_FLAGS='-mount ~/projects/origin-repos/4.2/:/etc/yum.repos.d/'
IMAGE_BUILD_DEFAULT_FLAGS ?=--allow-pull
IMAGE_BUILD_EXTRA_FLAGS ?=

# $1 - target name
# $2 - image ref
# $3 - Dockerfile path
# $4 - context
define build-image-internal
image-$(1): ensure-imagebuilder
	$(strip \
		imagebuilder \
		$(IMAGE_BUILD_DEFAULT_FLAGS) \
		-t $(2)
		-f $(3) \
		$(IMAGE_BUILD_EXTRA_FLAGS) \
		$(4) \
	)
.PHONY: image-$(1)

images: image-$(1)
.PHONY: images
endef

define build-image
$(eval $(call build-image-internal,$(1),$(2),$(3),$(4)))
endef
