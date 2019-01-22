self_dir :=$(dir $(lastword $(MAKEFILE_LIST)))

define build-package
	$(GO) build $(GO_BUILD_FLAGS) $(1)

endef

# We need to build each package separately so go build creates appropriate binaries
build:
	$(foreach package,$(GO_BUILD_PACKAGES_EXPANDED),$(call build-package,$(package)))
.PHONY: build

clean-binaries:
	$(RM) $(go_build_binaries)

# We need to be careful to expand all the paths before any include is done
# or self_dir could be modified for the next include by the included file.
# Also doing this at the end of the file allows us to user self_dir before it could be modified.
include $(addprefix $(self_dir), \
	../../lib/golang.mk \
)
