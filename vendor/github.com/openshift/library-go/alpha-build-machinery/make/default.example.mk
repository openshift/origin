all: build
.PHONY: all

# You can customize go tools depending on the directory layout.
# example:
GO_BUILD_PACKAGES :=./pkg/...
# You can list all the golang related variables by:
#   $ make -n --print-data-base | grep ^GO

# Include the library makefile
include ./default.mk
# All the available targets are listed in <this-file>.help
# or you can list it live by using `make help`

# Codegen module needs setting these required variables
CODEGEN_OUTPUT_PACKAGE :=github.com/openshift/cluster-openshift-apiserver-operator/pkg/generated
CODEGEN_API_PACKAGE :=github.com/openshift/cluster-openshift-apiserver-operator/pkg/apis
CODEGEN_GROUPS_VERSION :=openshiftapiserver:v1alpha1
# You can list all codegen related variables by:
#   $ make -n --print-data-base | grep ^CODEGEN

# This will call a macro called "build-image" which will generate image specific targets based on the parameters:
# $1 - target name
# $2 - image ref
# $3 - Dockerfile path
# $4 - context
# It will generate target "image-$(1)" for builing the image an binding it as a prerequisite to target "images".
$(call build-image,ocp-cli,registry.svc.ci.openshift.org/ocp/4.2:cli,./images/cli/Dockerfile.rhel,.)

# This will call a macro called "add-bindata" which will generate bindata specific targets based on the parameters:
# $0 - macro name
# $1 - target suffix
# $2 - input dirs
# $3 - prefix
# $4 - pkg
# $5 - output
# It will generate targets {update,verify}-bindata-$(1) logically grouping them in unsuffixed versions of these targets
# and also hooked into {update,verify}-generated for broader integration.
$(call add-bindata,v3.11.0,./bindata/v3.11.0/...,bindata,v311_00_assets,pkg/operator/v311_00_assets/bindata.go)

