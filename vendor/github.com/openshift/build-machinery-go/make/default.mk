self_dir := $(dir $(lastword $(MAKEFILE_LIST)))

# We extend the default verify/update for Golang

verify: verify-codegen
verify: verify-bindata
.PHONY: verify

update: update-codegen
update: update-bindata
.PHONY: update


# We need to be careful to expand all the paths before any include is done
# or self_dir could be modified for the next include by the included file.
# Also doing this at the end of the file allows us to use self_dir before it could be modified.
include $(addprefix $(self_dir), \
	targets/openshift/deps.mk \
	targets/openshift/images.mk \
	targets/openshift/bindata.mk \
	targets/openshift/codegen.mk \
	golang.mk \
)
