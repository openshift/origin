include $(addprefix $(dir $(lastword $(MAKEFILE_LIST))), \
	../../lib/golang.mk \
	../../lib/tmp.mk \
)

IMAGEBUILDER_VERSION ?=1.2.1

IMAGEBUILDER ?= $(shell which imagebuilder 2>/dev/null)
ifneq "" "$(IMAGEBUILDER)"
_imagebuilder_installed_version = $(shell $(IMAGEBUILDER) --version)
endif

# NOTE: We would like to
#     go get github.com/openshift/imagebuilder/cmd/imagebuilder@v$(IMAGEBUILDER_VERSION)
# ...but `go get` is too unreliable. So instead we use this to make the
# "you don't have imagebuilder" error useful.
ensure-imagebuilder:
ifeq "" "$(IMAGEBUILDER)"
	$(error imagebuilder not found! Get it with: `go get github.com/openshift/imagebuilder/cmd/imagebuilder@v$(IMAGEBUILDER_VERSION)`)
else
	$(info Using existing imagebuilder from $(IMAGEBUILDER))
	@[[ "$(_imagebuilder_installed_version)" == $(IMAGEBUILDER_VERSION) ]] || \
	echo "Warning: Installed imagebuilder version $(_imagebuilder_installed_version) does not match expected version $(IMAGEBUILDER_VERSION)."
endif
.PHONY: ensure-imagebuilder
