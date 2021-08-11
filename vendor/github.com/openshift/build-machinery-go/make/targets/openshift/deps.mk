# Use a unique variable name to avoid conflicting with generic
# `self_dir` elsewhere.
_self_dir_openshift_deps :=$(dir $(lastword $(MAKEFILE_LIST)))

deps_gomod_mkfile := $(_self_dir_openshift_deps)/deps-gomod.mk
deps_glide_mkfile := $(_self_dir_openshift_deps)/deps-glide.mk
include $(addprefix $(_self_dir_openshift_deps), \
	../../lib/golang.mk \
)

ifneq "$(GO) list $(GO_MOD_FLAGS) -m" ""
include $(deps_gomod_mkfile)
else
include $(deps_glide_mkfile)
endif
