self_dir :=$(dir $(lastword $(MAKEFILE_LIST)))

CRD_SCHEMA_GEN_APIS ?=$(error CRD_SCHEMA_GEN_APIS is required)
CRD_SCHEMA_GEN_MANIFESTS ?=./manifests
CRD_SCHEMA_GEN_OUTPUT ?=./manifests

crd_patches =$(subst $(CRD_SCHEMA_GEN_MANIFESTS),$(CRD_SCHEMA_GEN_OUTPUT),$(wildcard $(CRD_SCHEMA_GEN_MANIFESTS)/*.crd.yaml-merge-patch))

# $1 - crd file
# $2 - patch file
define patch-crd
	$(YQ) m -i '$(1)' '$(2)'

endef

empty :=
update-codegen-crds: ensure-controller-gen ensure-yq
	'$(CONTROLLER_GEN)' \
		schemapatch:manifests="$(CRD_SCHEMA_GEN_MANIFESTS)" \
		paths="$(subst $(empty) ,;,$(CRD_SCHEMA_GEN_APIS))" \
		output:dir="$(CRD_SCHEMA_GEN_OUTPUT)"
	cp -n $(wildcard $(CRD_SCHEMA_GEN_MANIFESTS)/*.crd.yaml-merge-patch) '$(CRD_SCHEMA_GEN_OUTPUT)/' || true  # FIXME: centos
	$(foreach p,$(crd_patches),$(call patch-crd,$(basename $(p)).yaml,$(p)))
.PHONY: update-codegen-crds

update-generated: update-codegen-crds
.PHONY: update-generated

update: update-generated
.PHONY: update

# $1 - manifest (actual) crd
# $2 - temp crd
define diff-crd
	diff -Naup $(1) $(2)

endef

verify-codegen-crds: CRD_SCHEMA_GEN_OUTPUT :=$(shell mktemp -d)
verify-codegen-crds: update-codegen-crds
	$(foreach p,$(wildcard $(CRD_SCHEMA_GEN_MANIFESTS)/*.crd.yaml),$(call diff-crd,$(p),$(subst $(CRD_SCHEMA_GEN_MANIFESTS),$(CRD_SCHEMA_GEN_OUTPUT),$(p))))
.PHONY: verify-codegen-crds

verify-generated: verify-codegen-crds
.PHONY: verify-generated

verify: verify-generated
.PHONY: verify


# We need to be careful to expand all the paths before any include is done
# or self_dir could be modified for the next include by the included file.
# Also doing this at the end of the file allows us to user self_dir before it could be modified.
include $(addprefix $(self_dir), \
	../../lib/golang.mk \
	../../lib/tmp.mk \
	../../targets/openshift/controller-gen.mk \
	../../targets/openshift/yq.mk \
)
