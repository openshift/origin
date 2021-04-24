self_dir :=$(dir $(lastword $(MAKEFILE_LIST)))

# Merge yaml patch using mikefarah/yq
# $1 - patch file
# $2 - manifest file
# $3 - output file
define patch-manifest-yq
	$(YQ) m -x '$(2)' '$(1)' > '$(3)'

endef

# Apply yaml-patch using krishicks/yaml-patch
# $1 - patch file
# $2 - manifest file
# $3 - output file
define patch-manifest-yaml-patch
	$(YAML_PATCH) -o '$(1)' < '$(2)' > '$(3)'

endef

profile-yaml-patches = $(sort $(shell find $(1) -type f -name '*.yaml-patch'))
profile-yaml-merge-patches = $(sort $(shell find $(1) -type f -name '*.yaml-merge-patch'))

# Apply profile patches to manifests
# $1 - patch dir
# $2 - manifests dir
define apply-profile-manifest-patches
	$$(foreach p,$$(call profile-yaml-patches,$(1)),$$(call patch-manifest-yaml-patch,$$(p),$$(realpath $(2))/$$(basename $$(notdir $$(p))).yaml,$$(realpath $(2))/$$(basename $$(notdir $$(p)))-$$(notdir $$(realpath $$(dir $$(p)))).yaml))
	$$(foreach p,$$(call profile-yaml-merge-patches,$(1)),$$(call patch-manifest-yq,$$(p),$$(realpath $(2))/$$(basename $$(notdir $$(p))).yaml,$$(realpath $(2))/$$(basename $$(notdir $$(p)))-$$(notdir $$(realpath $$(dir $$(p)))).yaml))
endef

# $1 - target name
# $2 - patch dir
# $3 - manifest dir
define add-profile-manifests-internal

update-profile-manifests-$(1): ensure-yq ensure-yaml-patch
	$(call apply-profile-manifest-patches,$(2),$(3))
.PHONY: update-profile-manifests-$(1)

update-profile-manifests: update-profile-manifests-$(1)
.PHONY: update-profile-manifests

verify-profile-manifests-$(1): VERIFY_PROFILE_MANIFESTS_TMP_DIR:=$$(shell mktemp -d)
verify-profile-manifests-$(1): ensure-yq ensure-yaml-patch
	cp -R $(3)/* $$(VERIFY_PROFILE_MANIFESTS_TMP_DIR)/
	$(call apply-profile-manifest-patches,$(2),$$(VERIFY_PROFILE_MANIFESTS_TMP_DIR))
	diff -Naup $(3) $$(VERIFY_PROFILE_MANIFESTS_TMP_DIR)
.PHONY: verify-profile-manifests-$(1)

verify-profile-manifests: verify-profile-manifests-$(1)
.PHONY: verify-profile-manifests

update-generated: update-profile-manifests
.PHONY: update-generated

update: update-generated
.PHONY: update

verify-generated: verify-profile-manifests
.PHONY: verify-generated

verify: verify-generated
.PHONY: verify

endef


# $1 - target name
# $2 - profile patches dir
# $3 - manifests dir
define add-profile-manifests
$(eval $(call add-profile-manifests-internal,$(1),$(2),$(3)))
endef


# We need to be careful to expand all the paths before any include is done
# or self_dir could be modified for the next include by the included file.
# Also doing this at the end of the file allows us to use self_dir before it could be modified.
include $(addprefix $(self_dir), \
	../../../lib/tmp.mk \
	../yq.mk \
	../yaml-patch.mk \
)
