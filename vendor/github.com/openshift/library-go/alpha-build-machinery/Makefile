all: verify
.PHONY: all

makefiles :=$(wildcard ./make/*.example.mk)

# $1 - makefile name relative to ./make/ folder
# $2 - output folder
# We need to change dir to the final makefile directory or relative paths won't match
define update-makefile-help
$(MAKE) -C "$(dir $(1))" -f "$(notdir $(1))" --no-print-directory --warn-undefined-variables help 2>&1 | tee "$(2)"/"$(notdir $(1))".help

endef


# $1 - makefile name relative to ./make/ folder
# $2 - output folder
define check-makefile
$(call update-makefile-help,$(1),$(2))
diff -N "$(1).help" "$(2)/$(notdir $(1)).help"

endef

update-makefiles:
	$(foreach f,$(makefiles),$(call check-makefile,$(f),$(dir $(f))))
.PHONY: update-makefiles

verify-makefiles: tmp_dir:=$(shell mktemp -d)
verify-makefiles:
	$(foreach f,$(makefiles),$(call check-makefile,$(f),$(tmp_dir)))
.PHONY: verify-makefiles


verify: verify-makefiles
.PHONY: verify

update: update-makefiles
.PHONY: update


include ./make/targets/help.mk
