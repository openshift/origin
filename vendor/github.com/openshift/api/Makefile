all: build
.PHONY: all

# Ensure update-scripts are run before crd-gen so updates to Godoc are included in CRDs.
update-codegen-crds: update-scripts

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
	golang.mk \
	targets/openshift/deps.mk \
	targets/openshift/crd-schema-gen.mk \
)

GO_PACKAGES :=$(addsuffix ...,$(addprefix ./,$(filter-out vendor/,$(filter-out hack/,$(wildcard */)))))
GO_BUILD_PACKAGES :=$(GO_PACKAGES)
GO_BUILD_PACKAGES_EXPANDED :=$(GO_BUILD_PACKAGES)
# LDFLAGS are not needed for dummy builds (saving time on calling git commands)
GO_LD_FLAGS:=
CONTROLLER_GEN_VERSION :=v0.7.0

# $1 - target name
# $2 - apis
# $3 - manifests
# $4 - output
$(call add-crd-gen,authorization,./authorization/v1,./authorization/v1,./authorization/v1)
$(call add-crd-gen,apiserver,./apiserver/v1,./apiserver/v1,./apiserver/v1)
$(call add-crd-gen,config,./config/v1,./config/v1,./config/v1)
$(call add-crd-gen,helm,./helm/v1beta1,./helm/v1beta1,./helm/v1beta1)
$(call add-crd-gen,console,./console/v1,./console/v1,./console/v1)
$(call add-crd-gen,console-alpha,./console/v1alpha1,./console/v1alpha1,./console/v1alpha1)
$(call add-crd-gen,imageregistry,./imageregistry/v1,./imageregistry/v1,./imageregistry/v1)
$(call add-crd-gen,operator,./operator/v1,./operator/v1,./operator/v1)
$(call add-crd-gen,operator-alpha,./operator/v1alpha1,./operator/v1alpha1,./operator/v1alpha1)
$(call add-crd-gen,operatoringress,./operatoringress/v1,./operatoringress/v1,./operatoringress/v1)
$(call add-crd-gen,sharedresource,./sharedresource/v1alpha1,./sharedresource/v1alpha1,./sharedresource/v1alpha1)
$(call add-crd-gen,quota,./quota/v1,./quota/v1,./quota/v1)
$(call add-crd-gen,samples,./samples/v1,./samples/v1,./samples/v1)
$(call add-crd-gen,security,./security/v1,./security/v1,./security/v1)
$(call add-crd-gen,securityinternal,./securityinternal/v1,./securityinternal/v1,./securityinternal/v1)
$(call add-crd-gen,cloudnetwork,./cloudnetwork/v1,./cloudnetwork/v1,./cloudnetwork/v1)
$(call add-crd-gen,network,./network/v1,./network/v1,./network/v1)
$(call add-crd-gen,networkoperator,./networkoperator/v1,./networkoperator/v1,./networkoperator/v1)
$(call add-crd-gen,operatorcontrolplane,./operatorcontrolplane/v1alpha1,./operatorcontrolplane/v1alpha1,./operatorcontrolplane/v1alpha1)
$(call add-crd-gen,machine-beta,./machine/v1beta1,./machine/v1beta1,./machine/v1beta1)
$(call add-crd-gen,machine,./machine/v1,./machine/v1,./machine/v1)

RUNTIME ?= podman
RUNTIME_IMAGE_NAME ?= openshift-api-generator

verify-scripts:
	bash -x hack/verify-deepcopy.sh
	bash -x hack/verify-protobuf.sh
	bash -x hack/verify-swagger-docs.sh
	hack/verify-crds.sh
	bash -x hack/verify-types.sh
	hack/verify-crds-version-upgrade.sh
	bash -x hack/verify-compatibility.sh

.PHONY: verify-scripts
verify: verify-scripts verify-codegen-crds

update-scripts:
	hack/update-deepcopy.sh
	hack/update-compatibility.sh
	hack/update-protobuf.sh
	hack/update-swagger-docs.sh
.PHONY: update-scripts

generate-with-container: Dockerfile.build
	$(RUNTIME) build -t $(RUNTIME_IMAGE_NAME) -f Dockerfile.build .
	$(RUNTIME) run -ti --rm -v $(PWD):/go/src/github.com/openshift/api:z -w /go/src/github.com/openshift/api $(RUNTIME_IMAGE_NAME) make update
