SHELL :=/bin/bash
all: verify
.PHONY: all

makefiles :=$(wildcard ./make/*.example.mk)
examples :=$(wildcard ./make/examples/*/Makefile.test)

# $1 - makefile name relative to ./make/ folder
# $2 - target
# We need to change dir to the final makefile directory or relative paths won't match.
# Dynamic values are replaced with "<redacted_for_diff>" so we can do diff against checkout versions.
# Avoid comparing local paths by stripping the prefix.
# Delete lines referencing temporary files and directories
# Unify make error output between versions
# Ignore old cp errors on centos7
# Ignore different make output with `-k` option
define update-makefile-log
set -o pipefail; $(MAKE) -j 1 -C "$(dir $(1))" -f "$(notdir $(1))" --no-print-directory --warn-undefined-variables $(2) > "$(1)$(subst ..,.,.$(2).log.raw)" 2>&1 || (cat "$(1)$(subst ..,.,.$(2).log.raw)" && exit 1)
sed 's/\.\(buildDate\|versionFromGit\|commitFromGit\|gitTreeState\)="[^"]*" /.\1="<redacted_for_diff>" /g' "$(1)$(subst ..,.,.$(2).log.raw)" | \
   sed -E 's~/[^ ]*/(github.com/openshift/build-machinery-go/[^ ]*)~/\1~g' | \
   sed '/\/tmp\/tmp./d' | \
   sed '/git checkout -b/d' | \
   sed -E 's~^[<> ]*((\+\+\+|\-\-\-) \./(testing/)?manifests/.*.yaml).*~\1~' | \
   sed -E 's/^(make\[2\]: \*\*\* \[).*: (.*\] Error 1)/\1\2/' | \
   grep -v 'are the same file' | \
   grep -E -v -e '^make\[2\]: Target `.*'"'"' not remade because of errors\.$$' \
   > "$(1)$(subst ..,.,.$(2).log)"

endef


# $1 - makefile name relative to ./make/ folder
# $2 - target
define check-makefile-log
$(call update-makefile-log,$(1),$(2))
git diff --exit-code

endef

update-makefiles:
	$(foreach f,$(makefiles),$(call check-makefile-log,$(f),help))
	$(foreach f,$(examples),$(call check-makefile-log,$(f),))
.PHONY: update-makefiles

verify-makefiles:
	$(foreach f,$(makefiles),$(call check-makefile-log,$(f),help))
	$(foreach f,$(examples),$(call check-makefile-log,$(f),))
.PHONY: verify-makefiles

verify: verify-makefiles
.PHONY: verify

update: update-makefiles
.PHONY: update


include ./make/targets/help.mk
