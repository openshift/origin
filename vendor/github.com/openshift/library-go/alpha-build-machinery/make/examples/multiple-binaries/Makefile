include $(addprefix ../../, \
	golang.mk \
	targets/openshift/rpm.mk \
	targets/openshift/crd-schema-gen.mk \
)

# Set crd-schema-gen variables
CRD_SCHEMA_GEN_APIS :=$(addprefix ./pkg/apis/,v1 v1beta1)
CRD_SCHEMA_GEN_VERSION :=v0.2.1

# rpm wants build-id set
GO_LD_EXTRAFLAGS +=-B 0x$$(head -c20 /dev/urandom|od -An -tx1|tr -d ' \n')

OUTPUT_DIR :=_output
CROSS_BUILD_BINDIR :=$(OUTPUT_DIR)/bin
RPM_EXTRAFLAGS :=--quiet --define 'version 2.42.0' --define 'dist .el7' --define 'release 6'

cross-build-darwin-amd64:
	+@GOOS=darwin GOARCH=amd64 $(MAKE) --no-print-directory build GO_BUILD_BINDIR:=$(CROSS_BUILD_BINDIR)/darwin_amd64
.PHONY: cross-build-darwin-amd64

cross-build-windows-amd64:
	+@GOOS=windows GOARCH=amd64 $(MAKE) --no-print-directory build GO_BUILD_BINDIR:=$(CROSS_BUILD_BINDIR)/windows_amd64
.PHONY: cross-build-windows-amd64

cross-build: cross-build-darwin-amd64 cross-build-windows-amd64
.PHONY: cross-build

clean-cross-build:
	$(RM) -r '$(CROSS_BUILD_BINDIR)'
	if [ -d '$(OUTPUT_DIR)' ]; then rmdir --ignore-fail-on-non-empty '$(OUTPUT_DIR)'; fi
.PHONY: clean-cross-build

clean: clean-cross-build
