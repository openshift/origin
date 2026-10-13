scripts_dir :=$(shell realpath $(dir $(lastword $(MAKEFILE_LIST)))../../../scripts)

# `make vulncheck` will emit a report similar to:
# 
# [
#   "golang.org/x/net",
#   "v0.5.0",
#   "v0.7.0"
# ]
# [
#   "stdlib",
#   "go1.19.3",
#   "go1.20.1"
# ]
# [
#   "stdlib",
#   "go1.19.3",
#   "go1.19.4"
# ]
# 
# Each stanza lists
# - where the vulnerability exists
# - the version it was found in
# - the version it's fixed in
# 
# If the report contains any entries that are not in stdlib, the check
# will fail (exit nonzero). Otherwise it will succeed -- i.e. the stdlib
# entries are only warnings.
vulncheck:
	bash $(scripts_dir)/vulncheck.sh
.PHONY: vulncheck
