self_dir :=$(dir $(lastword $(MAKEFILE_LIST)))

deps_gomod_mkfile := $(self_dir)/deps-gomod.mk
deps_glide_mkfile := $(self_dir)/deps-glide.mk
include $(addprefix $(self_dir), \
	../../lib/golang.mk \
)

ifneq "$(GO) list $(GO_MOD_FLAGS) -m" ""
include $(deps_gomod_mkfile)
else
include $(deps_glide_mkfile)
endif
