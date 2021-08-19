ifndef _YQ_MK_
_YQ_MK_ := defined

include $(addprefix $(dir $(lastword $(MAKEFILE_LIST))), \
	../../lib/golang.mk \
	../../lib/tmp.mk \
)

YQ_VERSION ?=2.4.0
YQ ?=$(PERMANENT_TMP_GOPATH)/bin/yq-$(YQ_VERSION)
yq_dir :=$(dir $(YQ))


ensure-yq:
ifeq "" "$(wildcard $(YQ))"
	$(info Installing yq into '$(YQ)')
	mkdir -p '$(yq_dir)'
	curl -s -f -L https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/yq_$(GOHOSTOS)_$(GOHOSTARCH) -o '$(YQ)'
	chmod +x '$(YQ)';
else
	$(info Using existing yq from "$(YQ)")
endif
.PHONY: ensure-yq

clean-yq:
	$(RM) $(yq_dir)yq*
	if [ -d '$(yq_dir)' ]; then rmdir --ignore-fail-on-non-empty -p '$(yq_dir)'; fi
.PHONY: clean-yq

clean: clean-yq

endif
