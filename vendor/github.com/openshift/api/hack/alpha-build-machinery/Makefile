SHELL :=/bin/bash
all: verify
.PHONY: all

makefiles :=$(wildcard ./make/*.example.mk)
examples :=$(wildcard ./make/examples/*/Makefile.test)

# $1 - makefile name relative to ./make/ folder
# $2 - target
# $3 - output folder
# We need to change dir to the final makefile directory or relative paths won't match.
# Dynamic values are replaced with "<redacted_for_diff>" so we can do diff against checkout versions.
# Avoid comparing local paths by stripping the prefix.
# Delete lines referencing temporary files and directories
# Unify make error output between versions
# Ignore old cp errors on centos7
# Ignore different make output with `-k` option
define update-makefile-log
mkdir -p "$(3)"
set -o pipefail; $(MAKE) -j 1 -C "$(dir $(1))" -f "$(notdir $(1))" --no-print-directory --warn-undefined-variables $(2) 2>&1 | \
   sed 's/\.\(buildDate\|versionFromGit\|commitFromGit\|gitTreeState\)="[^"]*" /.\1="<redacted_for_diff>" /g' | \
   sed -E 's~/.*/(github.com/openshift/library-go/alpha-build-machinery/.*)~/\1~g' | \
   sed '/\/tmp\/tmp./d' | \
   sed '/git checkout -b/d' | \
   sed -E 's~^[<> ]*((\+\+\+|\-\-\-) \./(testing/)?manifests/.*.yaml).*~\1~' | \
   sed -E 's/^(make\[2\]: \*\*\* \[).*: (.*\] Error 1)/\1\2/' | \
   grep -v 'are the same file' | \
   grep -E -v -e '^make\[2\]: Target `.*'"'"' not remade because of errors\.$$' | \
   tee "$(3)"/"$(notdir $(1))"$(subst ..,.,.$(2).log)

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
