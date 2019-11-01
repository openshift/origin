self_dir :=$(dir $(lastword $(MAKEFILE_LIST)))

YQ ?=$(PERMANENT_TMP_GOPATH)/bin/yq
yq_dir :=$(dir $(YQ))


ensure-yq:
ifeq "" "$(wildcard $(YQ))"
	$(info Installing yq into '$(YQ)')
	mkdir -p '$(yq_dir)'
	curl -s -f -L https://github.com/mikefarah/yq/releases/download/2.4.0/yq_$(GOHOSTOS)_$(GOHOSTARCH) -o '$(YQ)'
	chmod +x '$(YQ)';
else
	$(info Using existing yq from "$(YQ)")
endif
.PHONY: ensure-yq

clean-yq:
	$(RM) '$(YQ)'
	if [ -d '$(yq_dir)' ]; then rmdir --ignore-fail-on-non-empty -p '$(yq_dir)'; fi
.PHONY: clean-yq

clean: clean-yq


# We need to be careful to expand all the paths before any include is done
# or self_dir could be modified for the next include by the included file.
# Also doing this at the end of the file allows us to user self_dir before it could be modified.
include $(addprefix $(self_dir), \
	../../lib/golang.mk \
	../../lib/tmp.mk \
)
