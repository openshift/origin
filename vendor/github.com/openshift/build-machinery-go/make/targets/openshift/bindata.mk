TMP_GOPATH :=$(shell mktemp -d)


.ensure-go-bindata:
	ln -s $(abspath ./vendor) "$(TMP_GOPATH)/src"
	export GO111MODULE=off && export GOPATH=$(TMP_GOPATH) && export GOBIN=$(TMP_GOPATH)/bin && go install "./vendor/github.com/go-bindata/go-bindata/..."

# $1 - input dirs
# $2 - prefix
# $3 - pkg
# $4 - output
# $5 - output prefix
define run-bindata
	$(TMP_GOPATH)/bin/go-bindata -nocompress -nometadata \
		-prefix "$(2)" \
		-pkg "$(3)" \
		-o "$(5)$(4)" \
		-ignore "OWNERS" \
		$(1) && \
	gofmt -s -w "$(5)$(4)"
endef

# $1 - name
# $2 - input dirs
# $3 - prefix
# $4 - pkg
# $5 - output
define add-bindata-internal
update-bindata-$(1): .ensure-go-bindata
	$(call run-bindata,$(2),$(3),$(4),$(5),)
.PHONY: update-bindata-$(1)

update-bindata: update-bindata-$(1)
.PHONY: update-bindata


verify-bindata-$(1): .ensure-go-bindata
verify-bindata-$(1): TMP_DIR := $$(shell mktemp -d)
verify-bindata-$(1):
	$(call run-bindata,$(2),$(3),$(4),$(5),$$(TMP_DIR)/) && \
	diff -Naup {.,$$(TMP_DIR)}/$(5)
.PHONY: verify-bindata-$(1)

verify-bindata: verify-bindata-$(1)
.PHONY: verify-bindata
endef


update-generated: update-bindata
.PHONY: update-bindata

update: update-generated
.PHONY: update


verify-generated: verify-bindata
.PHONY: verify-bindata

verify: verify-generated
.PHONY: verify


define add-bindata
$(eval $(call add-bindata-internal,$(1),$(2),$(3),$(4),$(5)))
endef
