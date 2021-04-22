ifndef _YAML_PATCH_MK_
_YAML_PATCH_MK_ := defined
self_dir :=$(dir $(lastword $(MAKEFILE_LIST)))

YAML_PATCH ?=$(PERMANENT_TMP_GOPATH)/bin/yaml-patch
yaml_patch_dir :=$(dir $(YAML_PATCH))


ensure-yaml-patch:
ifeq "" "$(wildcard $(YAML_PATCH))"
	$(info Installing yaml-patch into '$(YAML_PATCH)')
	mkdir -p '$(yaml_patch_dir)'
	curl -s -f -L https://github.com/krishicks/yaml-patch/releases/download/v0.0.10/yaml_patch_$(GOHOSTOS) -o '$(YAML_PATCH)'
	chmod +x '$(YAML_PATCH)';
else
	$(info Using existing yaml-patch from "$(YAML_PATCH)")
endif
.PHONY: ensure-yaml-patch

clean-yaml-patch:
	$(RM) '$(YAML_PATCH)'
	if [ -d '$(yaml_patch_dir)' ]; then rmdir --ignore-fail-on-non-empty -p '$(yaml_patch_dir)'; fi
.PHONY: clean-yaml-patch

clean: clean-yaml-patch


# We need to be careful to expand all the paths before any include is done
# or self_dir could be modified for the next include by the included file.
# Also doing this at the end of the file allows us to use self_dir before it could be modified.
include $(addprefix $(self_dir), \
	../../lib/golang.mk \
	../../lib/tmp.mk \
)
endif
