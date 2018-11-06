all: build
.PHONY: all

include alpha-build-machinery/make/golang.mk
include alpha-build-machinery/make/targets/openshift/deps.mk
include alpha-build-machinery/make/targets/openshift/bindata.mk

$(call add-bindata,staticpod,./pkg/operator/staticpod/controller/backingresource/manifests/...,bindata,bindata,./pkg/operator/staticpod/controller/backingresource/bindata/bindata.go)

GO_BUILD_PACKAGES :=./pkg/...
