#
# Experimental Makefile for building images by observing their dependencies.
# Each sub directory that contains a Dockerfile is a target. The following
# files alter how those targets are linked:
#
#   .deps: Each line in this file becomes a dependency of the image target
#   .image-dep: The base image the image depends on as a Makefile target
#   .image-name: The canonical name of the image, may reference variables
#                in the Makefile like $(imagerepo) or $(imageprefix)
#
# TODO: handle image does not exist locally but .built exists
#

dockerfile = Dockerfile
image_dirs = $(sort $(dir $(shell find . -type f -name $(dockerfile))))
stamp = ../_output/images/
bin = ../_output/local/bin/linux/amd64/
imagerepo = openshift/
imageprefix = $(imagerepo)origin-
IMAGE_CP = ln -f

imagename =	$(or $(shell cat $(1).image-name 2>/dev/null),$(imageprefix)$(subst /,-,$(subst ./,,$(1:/=))))
imagedep = $(shell cat $(1).image-dep 2>/dev/null)
# if the image dependency has been specified, use its canonical name
imagebuilder = $(if $(4),\
	imagebuilder -from "$(call imagename,$(4)/):$(3)" -t "$(2):$(3)" $(1),\
	imagebuilder -t "$(2):latest" -t "$(2):$(3)" $(1)\
)

all: $(image_dirs:/=)
.PHONY: all

# Tie each top level directory as a target to a timestamp file.
$(foreach dir, $(image_dirs), $(eval  	\
	$(dir:/=): $(stamp)$(dir).built; 			\
))

# Generate one timestamp file for each image directory, include the nested
# dependencies from the directory (if they exist), and run a build.
$(foreach dir, $(image_dirs), $(eval  							  \
	$(stamp)$(dir).built: $(dir)* 										  \
		$(shell cat $(dir).image-dep 2>/dev/null || true)	\
		$(shell cat $(dir).deps 2>/dev/null || true);   	\
		$$(call imagebuilder,$(dir),$(call imagename,$(dir)),other,$(call imagedep,$(dir))) && \
		mkdir -p $(stamp)$(dir) && touch $@							\
))

clean:
	@rm -rf $(stamp)$(WHAT)
.PHONY: clean

release: origin origin/node origin/pod origin/deployer origin/dockerregistry \
	origin/router/haproxy	origin/builder/docker/docker-builder \
	origin/builder/docker/sti-builder
.PHONY: release

#
# Special cases - not automated for now
#
origin/bin/%: $(bin)%
	$(IMAGE_CP) $^ origin/bin/

pod/bin/%: $(bin)%
	$(IMAGE_CP) $^ pod/bin/
