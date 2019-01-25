SHELL :=/bin/bash
all: verify
.PHONY: all

makefiles :=$(wildcard ./make/*.example.mk)
examples :=$(wildcard ./make/examples/*/Makefile.test)

# $1 - makefile name relative to ./make/ folder
# $2 - target
# $3 - output folder
# We need to change dir to the final makefile directory or relative paths won't match
define update-makefile-log
mkdir -p "$(3)"
$(MAKE) -C "$(dir $(1))" -f "$(notdir $(1))" --no-print-directory --warn-undefined-variables $(2) 2>&1 | tee "$(3)"/"$(notdir $(1))"$(subst ..,.,.$(2).log)

endef


# $1 - makefile name relative to ./make/ folder
# $2 - target
# $3 - output folder
define check-makefile-log
$(call update-makefile-log,$(1),$(2),$(3))
diff -N "$(1)$(subst ..,.,.$(2).log)" "$(3)/$(notdir $(1))$(subst ..,.,.$(2).log)"

endef

update-makefiles:
	$(foreach f,$(makefiles),$(call check-makefile-log,$(f),help,$(dir $(f))))
	$(foreach f,$(examples),$(call check-makefile-log,$(f),,$(dir $(f))))
.PHONY: update-makefiles

verify-makefiles: tmp_dir:=$(shell mktemp -d)
verify-makefiles:
	$(foreach f,$(makefiles),$(call check-makefile-log,$(f),help,$(tmp_dir)/$(dir $(f))))
	$(foreach f,$(examples),$(call check-makefile-log,$(f),,$(tmp_dir)/$(dir $(f))))
.PHONY: verify-makefiles

verify: verify-makefiles
.PHONY: verify

update: update-makefiles
.PHONY: update


include ./make/targets/help.mk
