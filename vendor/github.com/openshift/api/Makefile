all: build
.PHONY: all

# Include the library makefile
include $(addprefix ./hack/alpha-build-machinery/make/, \
	golang.mk \
	targets/openshift/deps.mk \
	targets/openshift/crd-schema-gen.mk \
)

GO_PACKAGES :=$(addsuffix ...,$(addprefix ./,$(filter-out vendor/,$(filter-out hack/,$(wildcard */)))))
GO_BUILD_PACKAGES :=$(GO_PACKAGES)
GO_BUILD_PACKAGES_EXPANDED :=$(GO_BUILD_PACKAGES)
# LDFLAGS are not needed for dummy builds (saving time on calling git commands)
GO_LD_FLAGS:=

# $1 - target name
# $2 - apis
# $3 - manifests
# $4 - output
$(call add-crd-gen,authorization,./authorization/v1,./authorization/v1,./authorization/v1)
$(call add-crd-gen,config,./config/v1,./config/v1,./config/v1)
$(call add-crd-gen,console,./console/v1,./console/v1,./console/v1)
$(call add-crd-gen,imageregistry,./imageregistry/v1,./imageregistry/v1,./imageregistry/v1)
$(call add-crd-gen,operator,./operator/v1,./operator/v1,./operator/v1)
$(call add-crd-gen,operator-alpha,./operator/v1alpha1,./operator/v1alpha1,./operator/v1alpha1)
$(call add-crd-gen,quota,./quota/v1,./quota/v1,./quota/v1)
$(call add-crd-gen,samples,./samples/v1,./samples/v1,./samples/v1)
$(call add-crd-gen,security,./security/v1,./security/v1,./security/v1)

RUNTIME ?= podman
RUNTIME_IMAGE_NAME ?= openshift-api-generator

verify-scripts:
	bash -x hack/verify-deepcopy.sh
	bash -x hack/verify-protobuf.sh
	bash -x hack/verify-swagger-docs.sh
	bash -x hack/verify-crds.sh
.PHONY: verify-scripts
verify: verify-scripts verify-codegen-crds

update-scripts:
	hack/update-deepcopy.sh
	hack/update-protobuf.sh
	hack/update-swagger-docs.sh
.PHONY: update-scripts
update: update-scripts update-codegen-crds

generate-with-container: Dockerfile.build
	$(RUNTIME) build -t $(RUNTIME_IMAGE_NAME) -f Dockerfile.build .
	$(RUNTIME) run -ti --rm -v $(PWD):/go/src/github.com/openshift/api:z -w /go/src/github.com/openshift/api $(RUNTIME_IMAGE_NAME) make update-scripts
