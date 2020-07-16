# $1 - required version
# $2 - current version
define is_equal_or_higher_version
$(strip $(filter $(2),$(firstword $(shell set -euo pipefail && printf '%s\n%s' '$(1)' '$(2)' | sort -V -r -b))))
endef

# $1 - program name
# $2 - required version variable name
# $3 - current version string
define require_minimal_version
$(if $($(2)),\
$(if $(strip $(call is_equal_or_higher_version,$($(2)),$(3))),,$(error `$(1)` is required with minimal version "$($(2))", detected version "$(3)". You can override this check by using `make $(2):=`)),\
)
endef
