all: build
.PHONY: all

# All the go packages (e.g. for verfy)
GO_PACKAGES :=./pkg/... ./cmd/...
# Packages to be compiled
GO_BUILD_PACKAGES :=$(GO_PACKAGES)
# Do not auto-expand packages for libraries or it would compile them separately
GO_BUILD_PACKAGES_EXPANDED :=$(GO_BUILD_PACKAGES)

include $(addprefix alpha-build-machinery/make/, \
	golang.mk \
	targets/openshift/deps.mk \
	targets/openshift/bindata.mk \
)

$(call add-bindata,backingresources,./pkg/operator/staticpod/controller/backingresource/manifests/...,bindata,bindata,./pkg/operator/staticpod/controller/backingresource/bindata/bindata.go)
$(call add-bindata,monitoring,./pkg/operator/staticpod/controller/monitoring/manifests/...,bindata,bindata,./pkg/operator/staticpod/controller/monitoring/bindata/bindata.go)
$(call add-bindata,installer,./pkg/operator/staticpod/controller/installer/manifests/...,bindata,bindata,./pkg/operator/staticpod/controller/installer/bindata/bindata.go)
$(call add-bindata,staticpod,./pkg/operator/staticpod/controller/prune/manifests/...,bindata,bindata,./pkg/operator/staticpod/controller/prune/bindata/bindata.go)
