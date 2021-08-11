# We need to include this before we use PERMANENT_TMP_GOPATH
# (indirectly) from ifeq.
include $(addprefix $(dir $(lastword $(MAKEFILE_LIST))), \
	../../lib/tmp.mk \
)

KUSTOMIZE_VERSION ?= 4.1.3
KUSTOMIZE ?= $(PERMANENT_TMP_GOPATH)/bin/kustomize-$(KUSTOMIZE_VERSION)
kustomize_dir := $(dir $(KUSTOMIZE))

ensure-kustomize:
ifeq "" "$(wildcard $(KUSTOMIZE))"
	$(info Installing kustomize into '$(KUSTOMIZE)')
	mkdir -p '$(kustomize_dir)'
	@# install_kustomize.sh lays down the binary as `kustomize`, and will
	@# also fail if a file of that name already exists. Remove it for
	@# backward compatibility (older b-m-gs used the raw file name).
	rm -f $(kustomize_dir)/kustomize
	@# NOTE: Pinning script to a tag rather than `master` for security reasons
	curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/kustomize/v$(KUSTOMIZE_VERSION)/hack/install_kustomize.sh"  | bash -s $(KUSTOMIZE_VERSION) $(kustomize_dir)
	mv $(kustomize_dir)/kustomize $(KUSTOMIZE)
else
	$(info Using existing kustomize from "$(KUSTOMIZE)")
endif
.PHONY: ensure-kustomize

clean-kustomize:
	$(RM) $(kustomize_dir)kustomize*
	if [ -d '$(kustomize_dir)' ]; then rmdir --ignore-fail-on-non-empty -p '$(kustomize_dir)'; fi
.PHONY: clean-kustomize

clean: clean-kustomize
