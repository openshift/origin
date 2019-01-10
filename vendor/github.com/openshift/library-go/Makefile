all: build
.PHONY: all

include alpha-build-machinery/make/golang.mk
include alpha-build-machinery/make/targets/openshift/deps.mk
include alpha-build-machinery/make/targets/openshift/bindata.mk

$(call add-bindata,backingresources,./pkg/operator/staticpod/controller/backingresource/manifests/...,bindata,bindata,./pkg/operator/staticpod/controller/backingresource/bindata/bindata.go)
$(call add-bindata,monitoring,./pkg/operator/staticpod/controller/monitoring/manifests/...,bindata,bindata,./pkg/operator/staticpod/controller/monitoring/bindata/bindata.go)
$(call add-bindata,installer,./pkg/operator/staticpod/controller/installer/manifests/...,bindata,bindata,./pkg/operator/staticpod/controller/installer/bindata/bindata.go)
$(call add-bindata,staticpod,./pkg/operator/staticpod/controller/prune/manifests/...,bindata,bindata,./pkg/operator/staticpod/controller/prune/bindata/bindata.go)

GO_BUILD_PACKAGES :=./pkg/...
