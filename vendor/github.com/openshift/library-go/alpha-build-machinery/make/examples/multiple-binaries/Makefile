include $(addprefix ../../, \
	golang.mk \
)

CROSS_BUILD_BINDIR :=_output/bin

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
.PHONY: clean-cross-build

clean: clean-cross-build
