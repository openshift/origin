include $(addprefix $(dir $(lastword $(MAKEFILE_LIST))), \
	../../lib/golang.mk \
	../../lib/tmp.mk \
)

# NOTE: The release binary specified here needs to be built properly so that
# `--version` works correctly. Just using `go build` will result in it
# reporting `(devel)`. To build for a given platform:
# 	GOOS=xxx GOARCH=yyy go install sigs.k8s.io/controller-tools/cmd/controller-gen@$version
# e.g.
# 	GOOS=darwin GOARCH=amd64 go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.2
#
# If GOOS and GOARCH match your current go env, this will install the binary at
# 	$(go env GOPATH)/bin/controller-gen
# Otherwise (when cross-compiling) it will install the binary at
# 	$(go env GOPATH)/bin/${GOOS}_${GOARCH}/conroller-gen
# e.g.
# 	/home/efried/.gvm/pkgsets/go1.16/global/bin/darwin_amd64/controller-gen
CONTROLLER_GEN_VERSION ?=v0.9.2
CONTROLLER_GEN ?=$(PERMANENT_TMP_GOPATH)/bin/controller-gen-$(CONTROLLER_GEN_VERSION)
ifneq "" "$(wildcard $(CONTROLLER_GEN))"
_controller_gen_installed_version = $(shell $(CONTROLLER_GEN) --version | awk '{print $$2}')
endif
controller_gen_dir :=$(dir $(CONTROLLER_GEN))

ensure-controller-gen:
ifeq "" "$(wildcard $(CONTROLLER_GEN))"
	$(info Installing controller-gen into '$(CONTROLLER_GEN)')
	mkdir -p '$(controller_gen_dir)'
	curl -s -f -L https://github.com/openshift/kubernetes-sigs-controller-tools/releases/download/$(CONTROLLER_GEN_VERSION)/controller-gen-$(GOHOSTOS)-$(GOHOSTARCH) -o '$(CONTROLLER_GEN)'
	chmod +x '$(CONTROLLER_GEN)';
else
	$(info Using existing controller-gen from "$(CONTROLLER_GEN)")
	@[[ "$(_controller_gen_installed_version)" == $(CONTROLLER_GEN_VERSION) ]] || \
	echo "Warning: Installed controller-gen version $(_controller_gen_installed_version) does not match expected version $(CONTROLLER_GEN_VERSION)."
endif
.PHONY: ensure-controller-gen

clean-controller-gen:
	$(RM) $(controller_gen_dir)controller-gen*
	if [ -d '$(controller_gen_dir)' ]; then rmdir --ignore-fail-on-non-empty -p '$(controller_gen_dir)'; fi
.PHONY: clean-controller-gen

clean: clean-controller-gen
