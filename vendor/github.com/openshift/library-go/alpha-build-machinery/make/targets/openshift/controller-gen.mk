self_dir :=$(dir $(lastword $(MAKEFILE_LIST)))

CONTROLLER_GEN_VERSION ?=v0.2.1
CONTROLLER_GEN_TEMP ?=$(PERMANENT_TMP_GOPATH)/src/sigs.k8s.io/controller-tools
controller_gen_gopath =$(shell realpath -m $(CONTROLLER_GEN_TEMP)/../..)
CONTROLLER_GEN ?=$(CONTROLLER_GEN_TEMP)/controller-gen

ensure-controller-gen:
ifeq "" "$(wildcard $(CONTROLLER_GEN))"
	$(info Installing controller-gen into "$(CONTROLLER_GEN)")
	mkdir -p '$(CONTROLLER_GEN_TEMP)'
	git clone -b '$(CONTROLLER_GEN_VERSION)' --single-branch --depth=1 https://github.com/kubernetes-sigs/controller-tools.git '$(CONTROLLER_GEN_TEMP)'
	@echo '$(CONTROLLER_GEN_TEMP)/../..'
	cd '$(CONTROLLER_GEN_TEMP)' && export GO111MODULE=on GOPATH='$(controller_gen_gopath)' && $(GO) mod vendor 2>/dev/null && $(GO) build -mod=vendor ./cmd/controller-gen
else
	$(info Using existing controller-gen from "$(CONTROLLER_GEN)")
endif
.PHONY: ensure-controller-gen

clean-controller-gen:
	if [ -d '$(controller_gen_gopath)/pkg/mod' ]; then chmod +w -R '$(controller_gen_gopath)/pkg/mod'; fi
	$(RM) -r '$(CONTROLLER_GEN_TEMP)' '$(controller_gen_gopath)/pkg/mod'
	@mkdir -p '$(CONTROLLER_GEN_TEMP)'  # to make sure we can do the next step and to avoid using '/*' wildcard on the line above which could go crazy on wrong substitution
	if [ -d '$(CONTROLLER_GEN_TEMP)' ]; then rmdir --ignore-fail-on-non-empty -p '$(CONTROLLER_GEN_TEMP)'; fi
	@mkdir -p '$(controller_gen_gopath)/pkg/mod'  # to make sure we can do the next step and to avoid using '/*' wildcard on the line above which could go crazy on wrong substitution
	if [ -d '$(controller_gen_gopath)/pkg/mod' ]; then rmdir --ignore-fail-on-non-empty -p '$(controller_gen_gopath)/pkg/mod'; fi
.PHONY: clean-controller-gen

clean: clean-controller-gen


# We need to be careful to expand all the paths before any include is done
# or self_dir could be modified for the next include by the included file.
# Also doing this at the end of the file allows us to user self_dir before it could be modified.
include $(addprefix $(self_dir), \
	../../lib/golang.mk \
	../../lib/tmp.mk \
)
