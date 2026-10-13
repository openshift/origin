include $(addprefix $(dir $(lastword $(MAKEFILE_LIST))), \
	targets/openshift/deps.mk \
	targets/openshift/images.mk \
	targets/openshift/bindata.mk \
	targets/openshift/codegen.mk \
	golang.mk \
)

# We extend the default verify/update for Golang

verify: verify-codegen
verify: verify-bindata
.PHONY: verify

update: update-codegen
update: update-bindata
.PHONY: update
