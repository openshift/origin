self_dir :=$(dir $(lastword $(MAKEFILE_LIST)))

# $1 - crd file
# $2 - patch file
define patch-crd
	$(YQ) m -i -x '$(1)' '$(2)'

endef

empty :=

define diff-file
	diff -Naup '$(1)' '$(2)'

endef

# $1 - apis
# $2 - manifests
# $3 - output
define run-crd-gen
	'$(CONTROLLER_GEN)' \
		schemapatch:manifests="$(2)" \
		paths="$(subst $(empty) ,;,$(1))" \
		output:dir="$(3)"
	$$(foreach p,$$(wildcard $(2)/*.crd.yaml-merge-patch),$$(call patch-crd,$$(subst $(2),$(3),$$(basename $$(p))).yaml,$$(p)))
endef


# $1 - target name
# $2 - apis
# $3 - manifests
# $4 - output
define add-crd-gen-internal

update-codegen-crds-$(1): ensure-controller-gen ensure-yq
	$(call run-crd-gen,$(2),$(3),$(4))
.PHONY: update-codegen-crds-$(1)

update-codegen-crds: update-codegen-crds-$(1)
.PHONY: update-codegen-crds

verify-codegen-crds-$(1): VERIFY_CODEGEN_CRD_TMP_DIR:=$(shell mktemp -d)
verify-codegen-crds-$(1): ensure-controller-gen ensure-yq
	$(call run-crd-gen,$(2),$(3),$$(VERIFY_CODEGEN_CRD_TMP_DIR))
	$$(foreach p,$$(wildcard $(3)/*.crd.yaml),$$(call diff-file,$$(p),$$(subst $(3),$$(VERIFY_CODEGEN_CRD_TMP_DIR),$$(p))))
.PHONY: verify-codegen-crds-$(1)

verify-codegen-crds: verify-codegen-crds-$(1)
.PHONY: verify-codegen-crds

endef


update-generated: update-codegen-crds
.PHONY: update-generated

update: update-generated
.PHONY: update

verify-generated: verify-codegen-crds
.PHONY: verify-generated

verify: verify-generated
.PHONY: verify


define add-crd-gen
$(eval $(call add-crd-gen-internal,$(1),$(2),$(3),$(4)))
endef


# We need to be careful to expand all the paths before any include is done
# or self_dir could be modified for the next include by the included file.
# Also doing this at the end of the file allows us to user self_dir before it could be modified.
include $(addprefix $(self_dir), \
	../../lib/golang.mk \
	../../lib/tmp.mk \
	../../targets/openshift/controller-gen.mk \
	../../targets/openshift/yq.mk \
)
