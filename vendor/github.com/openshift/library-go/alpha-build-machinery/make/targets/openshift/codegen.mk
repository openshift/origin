CODEGEN_PKG ?=./vendor/k8s.io/code-generator/
CODEGEN_GENERATORS ?=all
CODEGEN_OUTPUT_BASE ?=../../..
CODEGEN_GO_HEADER_FILE ?=/dev/null

CODEGEN_API_PACKAGE ?=$(error CODEGEN_API_PACKAGE is required)
CODEGEN_GROUPS_VERSION ?=$(error CODEGEN_GROUPS_VERSION is required)
CODEGEN_OUTPUT_PACKAGE ?=$(error CODEGEN_OUTPUT_PACKAGE is required)

define run-codegen
$(CODEGEN_PKG)/generate-groups.sh \
	"$(CODEGEN_GENERATORS)" \
	"$(CODEGEN_OUTPUT_PACKAGE)" \
	"$(CODEGEN_API_PACKAGE)" \
	"$(CODEGEN_GROUPS_VERSION)" \
    --output-base $(CODEGEN_OUTPUT_BASE) \
    --go-header-file $(CODEGEN_GO_HEADER_FILE) \
    $1
endef


verify-codegen:
	$(call run-codegen,--verify-only)
.PHONY: verify-codegen

verify-generated: verify-codegen
.PHONY: verify-generated

verify: verify-generated
.PHONY: verify


update-codegen:
	$(call run-codegen)
.PHONY: update-codegen

update-generated: update-codegen
.PHONY: update-generated

update: update-generated
.PHONY: update
