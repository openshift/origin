include $(addprefix $(dir $(lastword $(MAKEFILE_LIST))), \
	../../lib/golang.mk \
	../../lib/tmp.mk \
)

.empty-golang-versions-files:
	@rm -f "$(PERMANENT_TMP)/golang-versions" "$(PERMANENT_TMP)/named-golang-versions"
.PHONY: .empty-golang-versions-files

verify-golang-versions:
	@if [ -f "$(PERMANENT_TMP)/golang-versions" ]; then \
		LINES=$$(cat "$(PERMANENT_TMP)/golang-versions" | sort | uniq | wc -l); \
			if [ $${LINES} -gt 1 ]; then \
			echo "Golang version mismatch:"; \
			cat "$(PERMANENT_TMP)/named-golang-versions" | sort | sed 's/^/- /'; \
			false; \
		fi; \
	fi
.PHONY: verify-golang-versions

# $1 - filename (symbolic, used as postfix in Makefile target)
# $2 - golang version
define verify-golang-version-reference-internal
verify-golang-versions-$(1): .empty-golang-versions-files
verify-golang-versions-$(1):
	@mkdir -p "$(PERMANENT_TMP)"
	@echo "$(1): $(2)" >> "$(PERMANENT_TMP)/named-golang-versions"
	@echo "$(2)" >> "$(PERMANENT_TMP)/golang-versions"
.PHONY: verify-golang-versions-$(1)

verify-golang-versions: verify-golang-versions-$(1)
endef

# $1 - filename (symbolic, used as postfix in Makefile target)
# $2 - golang version
define verify-golang-version-reference
$(eval $(call verify-golang-version-reference-internal,$(1),$(2)))
endef

# $1 - Dockerfile filename (symbolic, used as postfix in Makefile target)
define verify-Dockerfile-builder-golang-version
$(call verify-golang-version-reference,$(1),$(shell grep "AS builder" "$(1)" | sed 's/.*golang-\([[:digit:]][[:digit:]]*.[[:digit:]][[:digit:]]*\).*/\1/'))
endef

define verify-go-mod-golang-version
$(call verify-golang-version-reference,go.mod,$(shell grep -e 'go [[:digit:]]*\.[[:digit:]]*' go.mod 2>/dev/null | sed 's/go //'))
endef

define verify-buildroot-golang-version
$(call verify-golang-version-reference,.ci-operator.yaml,$(shell grep -e 'tag: .*golang-[[:digit:]]*\.[[:digit:]]' .ci-operator.yaml 2>/dev/null | sed 's/.*golang-\([[:digit:]][[:digit:]]*.[[:digit:]][[:digit:]]*\).*/\1/'))
endef

# $1 - optional Dockerfile filename (symbolic, used as postfix in Makefile target)
define verify-golang-versions
$(if $(1),$(call verify-Dockerfile-builder-golang-version,$(1))) \
$(if $(wildcard ./.ci-operator.yaml),$(if $(shell grep 'build_root_image:' .ci-operator.yaml 2>/dev/null),$(call verify-buildroot-golang-version))) \
$(if $(wildcard ./go.mod),$(call verify-go-mod-golang-version))
endef
